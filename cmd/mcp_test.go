package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/gitledger"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/mcp/protocol"
	toolresult "kbase/internal/result"
	"kbase/internal/version"
	"kbase/internal/write"
)

const (
	mcpTranscriptDir = "../test-data/fixtures/mcp"
	resultsFixture   = "../test-data/fixtures/results"
	// noResponse is the transcript line standing where a request is owed
	// no response.
	noResponse = "null"
)

// runAsKBaseEnv makes this test binary run as kbase itself, so the build
// tool's child — os.Executable(), this binary under test — is the build the
// shipped binary would start.
const runAsKBaseEnv = "KBASE_TEST_RUN_AS_KBASE"

// scratchStateHome is the XDG_STATE_HOME TestMain points the package's tests
// at, so no test reaches the user's state base or credentials store.
var scratchStateHome string

func TestMain(m *testing.M) {
	if os.Getenv(runAsKBaseEnv) != "" {
		main()
	}
	os.Exit(runWithScratchStateHome(m))
}

// runWithScratchStateHome runs the tests with XDG_STATE_HOME on a scratch
// directory, removed after them.
func runWithScratchStateHome(m *testing.M) int {
	dir, err := os.MkdirTemp("", "kbase-cmd-state-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "scratch state home:", err)
		return 1
	}
	defer os.RemoveAll(dir)
	if err := os.Setenv("XDG_STATE_HOME", dir); err != nil {
		fmt.Fprintln(os.Stderr, "scratch state home:", err)
		return 1
	}
	scratchStateHome = dir
	return m.Run()
}

// isolateChildBuild points everything a build started from this test reads
// or writes outside its repository — git's configuration and identity, the
// state home, kbase's configuration directory — into base, and makes this
// binary run as kbase when the build tool starts it.
func isolateChildBuild(t *testing.T, base string) {
	t.Helper()
	for k, v := range map[string]string{
		runAsKBaseEnv:     "1",
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@invalid", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@invalid",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": filepath.Join(base, "gitconfig"),
		"XDG_STATE_HOME": filepath.Join(base, "xdg"), "KBASE_CONFIG_DIR": childConfigDir(base),
	} {
		t.Setenv(k, v)
	}
}

// childConfigDir is the configuration directory isolateChildBuild names for
// base: the one directory a kbase started from the test reads.
func childConfigDir(base string) string { return filepath.Join(base, "config") }

// childStubConfig writes, into the configuration directory isolateChildBuild
// names for base, a pool of one entry, stub, on url — speaking OpenAI chat, as
// the test's stub endpoints stream it — and a config.toml naming model m on it
// for each of roles.
func childStubConfig(t *testing.T, base, url string, roles ...config.Role) {
	t.Helper()
	dir := childConfigDir(base)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeValues(t, dir, config.ProvidersFileName, "[stub]\nbaseUrl = \""+url+"\"\napi = \"openai\"\n")
	models := "[models]\n"
	for _, role := range roles {
		models += string(role) + " = \"stub/m\"\n"
	}
	writeValues(t, dir, config.ConfigFileName, models)
}

// gitRepo makes repo a repository as git init makes one, without starting
// a process.
func gitRepo(t *testing.T, repo string) {
	t.Helper()
	for _, d := range []string{".git/objects", ".git/refs/heads"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeValues(t, filepath.Join(repo, ".git"), "HEAD", "ref: refs/heads/main\n")
}

// mcpServer stages the results fixture KB as
// tools/scripts/test-integration-mcp.sh stages it: the results fixture's
// values/statements.md appended to a.md, the transcripts' two seeded claims
// placed on its lines — hosted alike, differing in title — then a work and an
// analysis, then one edge of each relation a public op writes, each batch
// naming only ids an earlier batch's result gave. It returns the server over
// the KB — committed and converted by update-format before the first write —
// and the transcripts' placeholders: ${KB_ROOT}, ${REPO} (the repository
// holding it), ${STATE_DIR} (an empty state store beside it), ${VERSION},
// ${ID1}, ${ID2} for the seeded claims' ids, in id order, ${TITLE1},
// ${TITLE2} for their titles, ${ID3} for the analysis's id and ${WORK1} for
// the work's.
func mcpServer(t *testing.T) (mcp.Server, *strings.Replacer) {
	t.Helper()
	repo, err := filepath.EvalSymlinks(stageRepo(t, resultsFixture))
	if err != nil {
		t.Fatal(err)
	}
	gitRepo(t, repo)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	isolateChildBuild(t, base)
	updateFormatKB(t, repo)
	if err := os.Mkdir(filepath.Join(repo, "kb-root", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	values := filepath.Join(resultsFixture, "values")
	leaf, err := os.ReadFile(filepath.Join(repo, "kb-root", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	statements, err := os.ReadFile(filepath.Join(values, "statements.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeValues(t, filepath.Join(repo, "kb-root"), "a.md", string(leaf)+string(statements))
	seed := filepath.Join(mcpTranscriptDir, "seed.yaml")
	none := strings.NewReplacer()
	kbRoot, claims := stagingOp(t, repo, "add-claim", seed, none)
	titles := seedTitles(t, seed)
	if len(claims) != 2 || len(titles) != 2 {
		t.Fatalf("staging add-claim: ids %v, titles %v, want two of each", claims, titles)
	}
	if claims[0] > claims[1] {
		claims[0], claims[1] = claims[1], claims[0]
		titles[0], titles[1] = titles[1], titles[0]
	}
	_, work := stagingOp(t, repo, "add-work", filepath.Join(values, "add-work.yaml"), none)
	_, analysis := stagingOp(t, repo, "add-analysis", filepath.Join(values, "add-analysis.yaml"), none)
	if len(work) != 1 || len(analysis) != 1 {
		t.Fatalf("staging add-work, add-analysis: ids %v, %v, want one each", work, analysis)
	}
	ids := strings.NewReplacer("${ID1}", claims[0], "${ID2}", claims[1], "${ID3}", analysis[0], "${WORK1}", work[0])
	stagingOp(t, repo, "add-edge", filepath.Join(values, "add-edge.yaml"), ids)
	srv := servedServer(rootCmd, mcp.LocatedAt(repo), &bytes.Buffer{}, log.Discard())
	srv.Instructions = mcp.ServedInstructions(srv.Located)
	return srv,
		strings.NewReplacer("${KB_ROOT}", jsonText(t, kbRoot), "${REPO}", jsonText(t, repo),
			"${STATE_DIR}", jsonText(t, filepath.Join(base, "state")), "${VERSION}", jsonText(t, version.Current),
			"${ID1}", claims[0], "${ID2}", claims[1], "${ID3}", analysis[0], "${WORK1}", work[0],
			"${TITLE1}", jsonText(t, titles[0]), "${TITLE2}", jsonText(t, titles[1]))
}

// seedTitles is the title of each entry of the values file, in entry order.
func seedTitles(t *testing.T, valuesFile string) []string {
	t.Helper()
	b, err := os.ReadFile(valuesFile)
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Entry []struct {
			Title string `yaml:"title"`
		} `yaml:"entry"`
	}
	if err := yaml.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, e := range v.Entry {
		titles = append(titles, e.Title)
	}
	return titles
}

// stagingOp runs the write op from repo over the values file with ids'
// placeholders replaced, and returns the kb-root and the ids its result
// reports, in entry order.
func stagingOp(t *testing.T, repo, op, valuesFile string, ids *strings.Replacer) (string, []string) {
	t.Helper()
	values, err := os.ReadFile(valuesFile)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code, err := runWriteOp(writeOpOptions{Op: op, WorkDir: repo, Stdin: strings.NewReader(ids.Replace(string(values))),
		Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	var res struct {
		KBRoot  string `yaml:"kb-root"`
		Entries []struct {
			ID string `yaml:"id"`
		} `yaml:"entries"`
	}
	if err != nil || code != 0 || yaml.Unmarshal(out.Bytes(), &res) != nil {
		t.Fatalf("staging %s: exit %d, %v\n%s", op, code, err, out.String())
	}
	var reported []string
	for _, e := range res.Entries {
		reported = append(reported, e.ID)
	}
	return res.KBRoot, reported
}

func serveLines(t *testing.T, srv mcp.Server, requests []string) []string {
	t.Helper()
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(strings.Join(requests, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	var lines []string
	sc := bufio.NewScanner(&out)
	sc.Buffer(nil, 1<<24)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func jsonText(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Trim(string(b), `"`)
}

// TestMCPTranscripts runs every transcript — request lines each followed by
// its expected response line, or by noResponse — through the server over
// the results fixture, and requires exactly the expected lines on stdout. A
// transcript named gui-* is served as kbase mcp --project-dir-required
// serves: a server that locates no project, so each call names the fixture.
// The server is served with no Graphviz dot on PATH, so a write's result pins
// the sheets not drawn whether or not the host has dot.
func TestMCPTranscripts(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(mcpTranscriptDir, "*.jsonl"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no transcripts under %s (%v)", mcpTranscriptDir, err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			if len(lines)%2 != 0 {
				t.Fatalf("%d lines: a transcript pairs each request with its response", len(lines))
			}
			srv, placeholders := mcpServer(t)
			t.Setenv("PATH", gitOnlyPath(t))
			if strings.HasPrefix(filepath.Base(file), "gui-") {
				srv = servedServer(rootCmd, mcp.ProjectRequired(), &bytes.Buffer{}, log.Discard())
				srv.Instructions = mcp.ServedInstructions(srv.Located)
			}
			var requests, want []string
			for i := 0; i < len(lines); i += 2 {
				requests = append(requests, placeholders.Replace(lines[i]))
				if lines[i+1] != noResponse {
					want = append(want, placeholders.Replace(lines[i+1]))
				}
			}
			got := serveLines(t, srv, requests)
			for i := range max(len(got), len(want)) {
				var g, w string
				if i < len(got) {
					g = got[i]
				}
				if i < len(want) {
					w = want[i]
				}
				if g != w {
					t.Errorf("response %d:\n got: %s\nwant: %s", i+1, g, w)
				}
			}
		})
	}
}

type listedSchema struct {
	Type                 string                    `json:"type"`
	Properties           map[string]map[string]any `json:"properties"`
	Required             []string                  `json:"required"`
	AdditionalProperties *bool                     `json:"additionalProperties"`
}

type listedTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema listedSchema    `json:"inputSchema"`
	Annotations map[string]bool `json:"annotations"`
}

func listTools(t *testing.T) []listedTool {
	t.Helper()
	srv, _ := mcpServer(t)
	return toolsListedBy(t, srv)
}

// toolsListedBy is what srv's tools/list answers.
func toolsListedBy(t *testing.T, srv mcp.Server) []listedTool {
	t.Helper()
	got := serveLines(t, srv, []string{`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`})
	if len(got) != 1 {
		t.Fatalf("tools/list: %d lines", len(got))
	}
	var resp struct {
		Result struct {
			Tools []listedTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(got[0]), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Result.Tools
}

// asOperation is the call of the family tool whose operation op is: the tool's
// name, and its arguments, operation and, where given, arguments.
func asOperation(t *testing.T, op string, arguments map[string]any) (string, map[string]any) {
	t.Helper()
	cmd, _, err := rootCmd.Find([]string{op})
	if err != nil || cmd.Annotations[mcp.FamilyAnnotation] == "" {
		t.Fatalf("%s is no family tool's operation (%v)", op, err)
	}
	args := map[string]any{"operation": op}
	if arguments != nil {
		args["arguments"] = arguments
	}
	return cmd.Annotations[mcp.FamilyAnnotation], args
}

// callOperation is one call of op through its family tool, as callTool.
func callOperation(t *testing.T, srv mcp.Server, op string, arguments map[string]any) (map[string]any, bool) {
	t.Helper()
	name, args := asOperation(t, op, arguments)
	return callTool(t, srv, name, args)
}

// unknownArgument is an argument no operation takes.
const unknownArgument = "not-an-argument"

// operationArguments is what the family tool's argument check refuses in a call
// of op given unknownArgument alone: the arguments op takes, as the item
// refusing unknownArgument lists them, and the required ones it misses.
func operationArguments(t *testing.T, srv mcp.Server, op string) (allowed, missing []string) {
	t.Helper()
	doc, isError := callOperation(t, srv, op, map[string]any{unknownArgument: 1})
	items, _ := doc["refusals"].([]any)
	if !isError || doc["outcome"] != toolresult.Refused || len(items) == 0 {
		t.Fatalf("%s given %s: isError %v, %v; want refused", op, unknownArgument, isError, doc)
	}
	refusedUnknown := false
	for _, it := range items {
		item, _ := it.(map[string]any)
		if item["check"] != toolresult.CheckUsage {
			t.Errorf("%s: refused %v, want a usage refusal", op, item)
		}
		key, _ := item["key"].(string)
		if key != unknownArgument {
			missing = append(missing, key)
			continue
		}
		// An operation that takes no argument lists none: allowed is absent.
		refusedUnknown = true
		listed, _ := item["allowed"].([]any)
		for _, a := range listed {
			name, _ := a.(string)
			allowed = append(allowed, name)
		}
	}
	if !refusedUnknown {
		t.Fatalf("%s: no item refuses %s: %v", op, unknownArgument, items)
	}
	return allowed, missing
}

func TestMCPToolsAreTheCommandTree(t *testing.T) {
	// want is every family and every bound command no family claims, and
	// readOnly the read-only expectation of the same set. No tool is listed by
	// hand, so the equality below fails one as it fails a missing command.
	var want, readOnly []string
	// operations is each family's members in the tree's order, and writes
	// whether one of them may write.
	operations := map[string][]string{}
	writes := map[string]bool{}
	// The tree is walked as Execute leaves it, so the commands cobra adds of
	// its own are held to the same declaration as the rest.
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	for _, c := range rootCmd.Commands() {
		declaredReadOnly := c.Annotations[mcp.ReadOnlyAnnotation] == "true"
		switch c.Annotations[mcp.BindingAnnotation] {
		case mcp.Bound:
			if family := c.Annotations[mcp.FamilyAnnotation]; family != "" {
				// A family a bound command is named for is that command's: its
				// tool serves the members, which have none of their own.
				if slices.ContainsFunc(rootCmd.Commands(), func(head *cobra.Command) bool {
					return head.Name() == family && head.Annotations[mcp.FamilyAnnotation] == "" && head.Annotations[mcp.BindingAnnotation] == mcp.Bound
				}) {
					continue
				}
				if operations[family] == nil {
					want = append(want, family)
				}
				operations[family] = append(operations[family], c.Name())
				writes[family] = writes[family] || !declaredReadOnly
				continue
			}
			want = append(want, c.Name())
			if declaredReadOnly {
				readOnly = append(readOnly, c.Name())
			}
		case mcp.Excluded:
			if strings.TrimSpace(c.Annotations[mcp.ExclusionReasonAnnotation]) == "" {
				t.Errorf("%s is excluded with no reason: give cliOnly its reason where %s adds it", c.Name(), registrationFile)
			}
		default:
			t.Errorf("%s declares no MCP binding: wrap it readOnlyTool, readWriteTool, queryOperation, writeOperation or cliOnly where %s adds it", c.Name(), registrationFile)
		}
	}
	for family := range operations {
		if !writes[family] {
			readOnly = append(readOnly, family)
		}
	}
	var names, gotReadOnly []string
	// projectDir is the project-dir property the first tool carries, which
	// every other tool must carry alike.
	var projectDir map[string]any
	for _, tool := range listTools(t) {
		names = append(names, tool.Name)
		if tool.Annotations["readOnlyHint"] {
			gotReadOnly = append(gotReadOnly, tool.Name)
		}
		s := tool.InputSchema
		if s.Type != "object" || s.Properties == nil || s.Required == nil || s.AdditionalProperties == nil || *s.AdditionalProperties {
			t.Errorf("%s: not a closed object schema with properties and required: %+v", tool.Name, s)
		}
		for _, key := range s.Required {
			if _, ok := s.Properties[key]; !ok {
				t.Errorf("%s: required %q is no property", tool.Name, key)
			}
		}
		// project-dir is the one global flag a tool takes: the project the
		// call runs in, optional on every tool, described alike.
		for key := range s.Properties {
			if key != "project-dir" && rootCmd.PersistentFlags().Lookup(key) != nil {
				t.Errorf("%s: takes the global flag %q", tool.Name, key)
			}
		}
		property, ok := s.Properties["project-dir"]
		description, _ := property["description"].(string)
		switch {
		case !ok || property["type"] != "string" || description == "":
			t.Errorf("%s: project-dir is %v, want a described string property", tool.Name, property)
		case slices.Contains(s.Required, "project-dir"):
			t.Errorf("%s: requires project-dir", tool.Name)
		case projectDir == nil:
			projectDir = property
		case !maps.Equal(property, projectDir):
			t.Errorf("%s: project-dir is %v, unlike the other tools' %v", tool.Name, property, projectDir)
		}
		if ops, ok := operations[tool.Name]; ok {
			// A family's description is its own sentence and two pointers: to
			// the family's help resource and to each operation's.
			for _, uri := range []string{mcp.HelpURIPrefix + tool.Name + ".", mcp.HelpURIPrefix + "<operation>"} {
				if !strings.Contains(tool.Description, uri) {
					t.Errorf("%s: description %q does not point at %s", tool.Name, tool.Description, uri)
				}
			}
			if keys := slices.Sorted(maps.Keys(s.Properties)); !slices.Equal(keys, []string{"arguments", "operation", "project-dir"}) {
				t.Errorf("%s: takes %v, want arguments, operation and project-dir", tool.Name, keys)
			}
			var enum []string
			listed, _ := s.Properties["operation"]["enum"].([]any)
			for _, op := range listed {
				name, _ := op.(string)
				enum = append(enum, name)
			}
			if s.Properties["operation"]["type"] != "string" || !slices.Equal(enum, ops) || !slices.Contains(s.Required, "operation") {
				t.Errorf("%s: operation is %v, required %v; want a required string enum of its members %v",
					tool.Name, s.Properties["operation"], s.Required, ops)
			}
			if s.Properties["arguments"]["type"] != "object" || slices.Contains(s.Required, "arguments") {
				t.Errorf("%s: arguments is %v, required %v; want an optional object", tool.Name, s.Properties["arguments"], s.Required)
			}
			continue
		}
		if tool.Name == write.Batch {
			// The write tool's arguments are the batch: its entries, dry-run
			// and commit.
			if keys := slices.Sorted(maps.Keys(s.Properties)); !slices.Equal(keys, []string{"commit", "dry-run", "entry", "project-dir"}) ||
				s.Properties["entry"]["type"] != "array" || s.Properties["dry-run"]["type"] != "boolean" || s.Properties["commit"]["type"] != "boolean" ||
				!slices.Equal(s.Required, []string{"entry"}) {
				t.Errorf("%s: takes %v, required %v; want the batch: entry (a required array), dry-run and commit (booleans) and project-dir", tool.Name, s.Properties, s.Required)
			}
			continue
		}
		if cmd, _, err := rootCmd.Find([]string{tool.Name}); err != nil {
			t.Error(err)
		} else if declared, err := mcp.DeclaredPositionals(cmd); err != nil {
			t.Error(err)
		} else {
			for _, p := range declared {
				if slices.Contains(s.Required, p.Name) == p.Optional {
					t.Errorf("%s: positional %q required %v, want %v", tool.Name, p.Name, !p.Optional, p.Optional)
				}
			}
		}
	}
	slices.Sort(want)
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Errorf("tools %v, want the commands declared %s: %v", names, mcp.Bound, want)
	}
	slices.Sort(gotReadOnly)
	slices.Sort(readOnly)
	if !slices.Equal(gotReadOnly, readOnly) {
		t.Errorf("read-only tools %v, want the commands declaring %s: %v", gotReadOnly, mcp.ReadOnlyAnnotation, readOnly)
	}
}

// TestMCPRequiredFlags: a tool's schema requires its positionals, less an
// optional variadic one, and no flag — score's --pass among them, a run walking
// every pass unless it is told otherwise — and every flag still stands as an
// optional property carrying its brief as its description; its usage and
// default are the help's argument list's. A query's required
// positional is an operation's argument, which refusal.jsonl's show exchange
// holds to the same rule.
func TestMCPRequiredFlags(t *testing.T) {
	tools := map[string]listedTool{}
	for _, tool := range listTools(t) {
		tools[tool.Name] = tool
	}
	required := func(t *testing.T, tool string) []string {
		t.Helper()
		listed, ok := tools[tool]
		if !ok {
			t.Fatalf("no %s tool", tool)
		}
		return slices.Sorted(slices.Values(listed.InputSchema.Required))
	}
	for _, c := range []struct {
		name, tool string
		want       []string
	}{
		{"flags alone, none of them required", "score", nil},
		{"flags and an optional variadic positional", "build", nil},
		{"flags and no positional", "status", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := required(t, c.tool); !slices.Equal(got, c.want) {
				t.Errorf("%s: required %v, want %v", c.tool, got, c.want)
			}
		})
	}
	t.Run("--pass is not required", func(t *testing.T) {
		if slices.Contains(tools["score"].InputSchema.Required, "pass") {
			t.Error("score: --pass is required, which its verb does not refuse a run without")
		}
	})
	t.Run("edge is a mapping over MCP", func(t *testing.T) {
		property := tools["score"].InputSchema.Properties["edge"]
		keys, _ := property["properties"].(map[string]any)
		required, _ := property["required"].([]any)
		if property["type"] != "object" || len(keys) != 3 || !slices.Equal(required, []any{"source", "target", "relation"}) ||
			property["additionalProperties"] != false {
			t.Errorf("score: edge property %v, want an object of exactly source, target and relation, each required", property)
		}
		for _, key := range []string{"source", "target", "relation"} {
			if k, _ := keys[key].(map[string]any); k["type"] != "string" {
				t.Errorf("score: edge's %s is %v, want a string", key, keys[key])
			}
		}
	})
	t.Run("every flag not excluded from MCP derives, beside project-dir", func(t *testing.T) {
		properties := tools["score"].InputSchema.Properties
		for _, name := range []string{"replay", "failed"} {
			if _, ok := properties[name]; ok {
				t.Errorf("score: the development flag --%s is a property of the tool", name)
			}
		}
		flags := []string{"project-dir"}
		scoreCmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" || mcp.ExcludedFlag(f) {
				return
			}
			flags = append(flags, f.Name)
			brief := mcp.BriefOf(scoreCmd, f.Name)
			if got, _ := properties[f.Name]["description"].(string); brief == "" || got != mcp.Spelling(rootCmd, true).Spell(brief) {
				t.Errorf("score: --%s description %q, want its brief %q", f.Name, got, brief)
			}
		})
		slices.Sort(flags)
		if got := slices.Sorted(maps.Keys(properties)); !slices.Equal(got, flags) {
			t.Errorf("score: properties %v, want its flags %v", got, flags)
		}
	})
}

// TestMCPBuildAndScoreTakeAskConcurrency: --ask-concurrency, declared on each
// verb's own flags, reaches the build and score tool schemas as an optional
// integer, with no table naming it.
func TestMCPBuildAndScoreTakeAskConcurrency(t *testing.T) {
	tools := map[string]listedTool{}
	for _, tool := range listTools(t) {
		tools[tool.Name] = tool
	}
	for _, name := range []string{"build", "score"} {
		s := tools[name].InputSchema
		if property, ok := s.Properties[askConcurrencyFlag]; !ok || property["type"] != "integer" {
			t.Errorf("%s: %s property %v, want an integer", name, askConcurrencyFlag, s.Properties[askConcurrencyFlag])
		}
		if slices.Contains(s.Required, askConcurrencyFlag) {
			t.Errorf("%s: %s is required", name, askConcurrencyFlag)
		}
	}
}

// TestSharedHelpIsStatedOnce: what the queries share is stated in no command's
// own help: the root command's help states the CLI's form once each, and the
// query tool's help resource, which lists its operations from the tree, the
// MCP form and none of the CLI's. How the CLI takes a write batch is stated in
// the root's help alone, and the write batch's contract in kbase write's help
// and the root's alone.
func TestSharedHelpIsStatedOnce(t *testing.T) {
	srv, _ := mcpServer(t)
	// listQueryHelp and queryReads are marked texts, which each surface spells.
	cliList, mcpList := mcp.Spelling(rootCmd, false).Spell(listQueryHelp), mcp.Spelling(rootCmd, true).Spell(listQueryHelp)
	cliReads, mcpReads := mcp.Spelling(rootCmd, false).Spell(queryReads), mcp.Spelling(rootCmd, true).Spell(queryReads)
	if !strings.Contains(cliList, "--limit") || strings.Contains(mcpList, "--") {
		t.Errorf("the list help spells its arguments\n%s\nat the command line and\n%s\nover MCP", cliList, mcpList)
	}
	if printed, _ := kbase(t, realPath(t, t.TempDir()), "--help"); !strings.Contains(printed, cliList) {
		t.Errorf("kbase --help does not state the list help as the command line spells it:\n%s", printed)
	}
	cliAnswer, mcpAnswer := queryAnswerHelp(" on stdout"), queryAnswerHelp("")
	cli := []string{queryHelp, listQueryHelp, valuesHelp, batchHelp}
	queryShared := []string{cliReads, mcpReads, cliAnswer, mcpAnswer, cliList, mcpList}
	for _, text := range cli {
		if strings.Count(rootCmd.Long, text) != 1 {
			t.Errorf("kbase --help states %q %d times, want once", text, strings.Count(rootCmd.Long, text))
		}
	}
	for _, c := range rootCmd.Commands() {
		if c.Annotations[mcp.BindingAnnotation] != mcp.Bound {
			continue
		}
		help := readResource(t, srv, mcp.HelpURIPrefix+c.Name())
		for _, text := range queryShared {
			if strings.Contains(help, text) {
				t.Errorf("%s's own help states %q, which the query help states", c.Name(), text)
			}
		}
		if strings.Contains(help, valuesHelp) {
			t.Errorf("%s's own help states how the CLI takes a write batch, which the root's states", c.Name())
		}
		if want := c.Name() == write.Batch; strings.Contains(help, mcpHelpSpelled(batchHelp)) != want {
			t.Errorf("%s's own help states the batch contract %v, want %v", c.Name(), !want, want)
		}
	}
	for family, want := range map[string][]string{queryFamily: {mcpReads, mcpAnswer, mcpList}} {
		help := readResource(t, srv, mcp.HelpURIPrefix+family)
		for _, text := range queryShared {
			if strings.Contains(help, text) != slices.Contains(want, text) {
				t.Errorf("%s's help states %q %v, want %v", family, text, strings.Contains(help, text), slices.Contains(want, text))
			}
		}
		if strings.Contains(help, "--") || strings.Contains(help, "stdin") || strings.Contains(help, "stdout") {
			t.Errorf("%s's help carries the CLI's form:\n%s", family, help)
		}
		for _, c := range rootCmd.Commands() {
			if c.Annotations[mcp.FamilyAnnotation] == family && !strings.Contains(help, c.Name()) {
				t.Errorf("%s's help does not list its operation %s", family, c.Name())
			}
		}
	}
}

// TestValuesOpHelpNamesItsKeys: a values operation's full help names every key
// of its entry, so the help its argument refusal points at holds its keys.
func TestValuesOpHelpNamesItsKeys(t *testing.T) {
	srv, _ := mcpServer(t)
	seen := 0
	for _, c := range rootCmd.Commands() {
		fields, ok := write.OpVocabulary(c.Name())
		if !ok {
			continue
		}
		seen++
		words := instructionsTokens(readResource(t, srv, mcp.HelpURIPrefix+c.Name()))
		for _, f := range fields {
			if !slices.Contains(words, f.Name) {
				t.Errorf("%s's help does not name its key %s", c.Name(), f.Name)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no command is a values operation")
	}
}

// readResource is the text resources/read answers for uri.
func readResource(t *testing.T, srv mcp.Server, uri string) string {
	t.Helper()
	var read struct {
		Contents []struct{ Text string } `json:"contents"`
	}
	serveOne(t, srv, "resources/read", map[string]any{"uri": uri}, &read)
	if len(read.Contents) != 1 {
		t.Fatalf("%s reads %+v", uri, read.Contents)
	}
	return read.Contents[0].Text
}

// TestNoSurfaceNamesTheProvenanceOrCreate: who supplied a write's values is
// write.Options.Provenance, set by kbase's own callers and never by a caller of
// the surface, so no command flag, tool property or operation argument names
// it; and no write op offers --create, as a flag or as its operation's create
// argument.
func TestNoSurfaceNamesTheProvenanceOrCreate(t *testing.T) {
	const provenance, create = "provenance", "create"
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, fs := range []*pflag.FlagSet{c.Flags(), c.PersistentFlags()} {
			fs.VisitAll(func(f *pflag.Flag) {
				if strings.Contains(strings.ToLower(f.Name), provenance) {
					t.Errorf("%s: flag --%s names the provenance", c.CommandPath(), f.Name)
				}
				if _, op := write.OpVocabulary(c.Name()); op && f.Name == create {
					t.Errorf("%s: the write op offers --%s", c.CommandPath(), f.Name)
				}
			})
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	srv, _ := mcpServer(t)
	for _, tool := range toolsListedBy(t, srv) {
		for p := range tool.InputSchema.Properties {
			if strings.Contains(strings.ToLower(p), provenance) {
				t.Errorf("%s: tool property %s names the provenance", tool.Name, p)
			}
		}
		// A family tool's operations take the arguments their own schemas
		// derive, which its argument check lists.
		operations, _ := tool.InputSchema.Properties["operation"]["enum"].([]any)
		for _, o := range operations {
			op, _ := o.(string)
			arguments, _ := operationArguments(t, srv, op)
			for _, a := range arguments {
				if strings.Contains(strings.ToLower(a), provenance) {
					t.Errorf("%s: %s's argument %s names the provenance", tool.Name, op, a)
				}
				if _, isOp := write.OpVocabulary(op); isOp && a == create {
					t.Errorf("%s: the write op %s takes %s", tool.Name, op, a)
				}
			}
		}
	}
}

// TestCommandsDeclarePositionals requires every command's usage line to
// name exactly the positionals its argument check accepts.
func TestCommandsDeclarePositionals(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		declared, err := mcp.DeclaredPositionals(c)
		if err != nil {
			t.Error(err)
			continue
		}
		variadic := len(declared) > 0 && declared[len(declared)-1].Variadic
		optional := variadic && declared[len(declared)-1].Optional
		for k := range len(declared) + 3 {
			accepted := c.ValidateArgs(make([]string, k)) == nil
			want := k == len(declared) || variadic && k > len(declared) || optional && k == len(declared)-1
			if accepted != want {
				t.Errorf("%s: %d positionals accepted %v; its usage line %q declares %d", c.Name(), k, accepted, c.Use, len(declared))
			}
		}
	}
}

// serveOne sends one request through the server and decodes its response's
// result into result.
func serveOne(t *testing.T, srv mcp.Server, method string, params map[string]any, result any) {
	t.Helper()
	req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	got := serveLines(t, srv, []string{string(req)})
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if len(got) != 1 || json.Unmarshal([]byte(got[0]), &resp) != nil || resp.Error != nil || json.Unmarshal(resp.Result, result) != nil {
		t.Fatalf("%s %v: %q", method, params, got)
	}
}

// callTool is one tools/call through the server: its structured content and
// whether it is an error.
func callTool(t *testing.T, srv mcp.Server, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	_, structured, isError := callToolResult(t, srv, name, args)
	return structured, isError
}

// callToolResult is one tools/call through the server: the result's text, its
// structured content and whether it is an error.
func callToolResult(t *testing.T, srv mcp.Server, name string, args map[string]any) (string, map[string]any, bool) {
	t.Helper()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Structured map[string]any `json:"structuredContent"`
		IsError    bool           `json:"isError"`
	}
	serveOne(t, srv, "tools/call", map[string]any{"name": name, "arguments": args}, &result)
	var text string
	if len(result.Content) > 0 && result.Content[0].Type == "text" {
		text = result.Content[0].Text
	}
	return text, result.Structured, result.IsError
}

// startTool is one tools/call of a tool that starts a detached run, as
// callTool, a done result's text checked as the launch's result.
func startTool(t *testing.T, srv mcp.Server, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	text, structured, isError := callToolResult(t, srv, name, args)
	if structured["outcome"] == toolresult.Done {
		checkResult(t, "launch", text)
	}
	return structured, isError
}

// stalledBuildRepos stages repositories named by names under one isolated
// base, each holding the transcripts' paper, for builds against a provider
// that never answers, so a build started there stays in flight. It returns the
// base and the repositories, in names' order.
func stalledBuildRepos(t *testing.T, names ...string) (string, []string) {
	t.Helper()
	if _, err := pandoc.Preflight(context.Background()); err != nil {
		t.Skipf("pandoc is not usable here: %v", err)
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(provider.Close)
	t.Cleanup(func() { close(release) })
	paper, err := os.ReadFile(filepath.Join(mcpTranscriptDir, "paper.tex"))
	if err != nil {
		t.Fatal(err)
	}
	var repos []string
	for _, name := range names {
		repo := filepath.Join(base, name)
		gitRepo(t, repo)
		writeValues(t, repo, "paper.tex", string(paper))
		repos = append(repos, repo)
	}
	isolateChildBuild(t, base)
	childStubConfig(t, base, provider.URL, config.RoleDecision, config.RoleText)
	return base, repos
}

// TestMCPBuildStartsDetached starts a build through the tool against a
// provider that never answers, so the build stays in flight: the tool
// returns the start result while the build runs on, status reads it
// running under the pid the tool named, and cancel stops it resumably. The
// server's --log-file reaches the build: this test process opens no log, so
// the file exists only if the build opened it. A call naming no project-dir
// hands out wait and resume lines without one; a call naming another project
// builds there, and both lines carry --project-dir.
func TestMCPBuildStartsDetached(t *testing.T) {
	base, repos := stalledBuildRepos(t, "repo", "other")
	repo, other := repos[0], repos[1]
	savedLevel, savedFile := flagLogLevel, flagLogFile
	t.Cleanup(func() { flagLogLevel, flagLogFile = savedLevel, savedFile })
	flagLogLevel, flagLogFile = "debug", filepath.Join(base, "build.log")
	srv := servedServer(rootCmd, mcp.LocatedAt(repo), &bytes.Buffer{}, log.Discard())
	state := filepath.Join(base, "state")
	monitor := map[string]any{"state-dir": state}

	start, isError := startTool(t, srv, "build", map[string]any{"volume-root": []any{filepath.Join(repo, "paper.tex")}, "state-dir": state})
	t.Cleanup(func() { callTool(t, srv, "cancel", monitor) })
	pid, _ := start["pid"].(float64)
	resume, _ := start["resume"].(string)
	// Over MCP the resume is the build tool's call.
	if isError || start["outcome"] != "done" || start["state-dir"] != state || pid <= 0 || !strings.HasPrefix(resume, "build {") ||
		!strings.Contains(resume, `"state-dir":`) || strings.Contains(resume, `"project-dir"`) {
		t.Fatalf("build: isError %v, %v", isError, start)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wantWait := buildstore.ShellJoin([]string{exe, "wait", "--state-dir", state, "--pid", strconv.Itoa(int(pid))})
	if wait, _ := start["wait"].(string); wait != wantWait {
		t.Errorf("build: wait = %q, want the exact command %q", wait, wantWait)
	}

	otherState := filepath.Join(base, "other-state")
	otherMonitor := map[string]any{"state-dir": otherState, "project-dir": other}
	elsewhere, isError := startTool(t, srv, "build", map[string]any{"project-dir": other,
		"volume-root": []any{filepath.Join(other, "paper.tex")}, "state-dir": otherState})
	t.Cleanup(func() { callTool(t, srv, "cancel", otherMonitor) })
	otherPID, _ := elsewhere["pid"].(float64)
	otherResume, _ := elsewhere["resume"].(string)
	if isError || elsewhere["outcome"] != "done" || elsewhere["kb-root"] != filepath.Join(other, "kb-root") || otherPID <= 0 ||
		!strings.Contains(otherResume, `"project-dir":"`+other+`"`) {
		t.Fatalf("build naming %s: isError %v, %v", other, isError, elsewhere)
	}
	wantWait = buildstore.ShellJoin([]string{exe, "wait", "--project-dir", other, "--state-dir", otherState, "--pid", strconv.Itoa(int(otherPID))})
	if wait, _ := elsewhere["wait"].(string); wait != wantWait {
		t.Errorf("build naming %s: wait = %q, want the exact command %q", other, wait, wantWait)
	}
	if cancel, isError := callTool(t, srv, "cancel", otherMonitor); isError || cancel["pid"] != otherPID {
		t.Errorf("cancel naming %s: isError %v, %v", other, isError, cancel)
	}
	if status, _ := callTool(t, srv, "status", monitor); status["state"] != "running" || status["pid"] != pid {
		t.Errorf("status after the start: %v, want running under pid %v", status, pid)
	}
	if _, err := os.Stat(flagLogFile); err != nil {
		t.Errorf("the server's --log-file did not reach the build: %v", err)
	}
	// The build is cancelled once its document-graph boundary stands: one
	// cancelled before it writes kb-root/ leaves no KB, so nothing to resume.
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(50 * time.Millisecond) {
		status, _ := callTool(t, srv, "status", monitor)
		stages, _ := status["stages"].(map[string]any)
		if done, _ := stages["done"].([]any); slices.Contains(done, any(buildstore.StageDocumentGraph)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status a minute after the start: %v, want %s done", status, buildstore.StageDocumentGraph)
		}
	}
	if cancel, isError := callTool(t, srv, "cancel", monitor); isError || cancel["outcome"] != "done" || cancel["pid"] != pid {
		t.Errorf("cancel: isError %v, %v", isError, cancel)
	}
	if status, _ := callTool(t, srv, "status", monitor); status["state"] != "cancelled" || status["resume"] == nil {
		t.Errorf("status after cancel: %v, want cancelled with a resume command", status)
	}
	// The wait the tool handed out is the background command a session without
	// a subscription runs: the run it names is over, so it returns at once with
	// that store's status result.
	out, code := kbase(t, repo, "wait", "--state-dir", state, "--pid", strconv.Itoa(int(pid)))
	if doc := checkResult(t, "wait", out); doc.Outcome != toolresult.Done || code != 0 ||
		doc.Values["state"] != "cancelled" || doc.Values["pid"] != int(pid) {
		t.Errorf("wait: outcome %s, exit %d, %v\n%s", doc.Outcome, code, doc.Values, out)
	}
}

// TestMCPBuildReportsARunStillStarting: a build that has not entered a stage
// 5 s after the call — waiting on the KB write lock another writer holds — is
// reported done, state starting, with its state-dir, pid and the wait command,
// for status to follow. It enters no stage, so it needs no pandoc.
func TestMCPBuildReportsARunStillStarting(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(base, "repo")
	gitRepo(t, repo)
	writeValues(t, repo, "paper.tex", "\\documentclass{article}\n\\begin{document}\nA paper.\n\\end{document}\n")
	isolateChildBuild(t, base)
	srv := servedServer(rootCmd, mcp.LocatedAt(repo), &bytes.Buffer{}, log.Discard())
	state := filepath.Join(base, "state")
	release, err := write.LockKB(filepath.Join(repo, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	t.Cleanup(func() { callTool(t, srv, "cancel", map[string]any{"state-dir": state}) })

	start, isError := startTool(t, srv, "build", map[string]any{"volume-root": []any{filepath.Join(repo, "paper.tex")},
		"no-inference": true, "state-dir": state})
	pid, _ := start["pid"].(float64)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wantWait := buildstore.ShellJoin([]string{exe, "wait", "--state-dir", state, "--pid", strconv.Itoa(int(pid))})
	if isError || start["outcome"] != toolresult.Done || start["kb-root"] != filepath.Join(repo, "kb-root") || start["state"] != "starting" ||
		start["state-dir"] != state || pid <= 0 || start["wait"] != wantWait {
		t.Fatalf("build start: isError %v, %v; want done, the kb-root, state starting, the state-dir, the pid and %q", isError, start, wantWait)
	}
	if keys := slices.Sorted(maps.Keys(start)); !slices.Equal(keys, []string{"kb-root", "outcome", "pid", "state", "state-dir", "wait", "warnings"}) {
		t.Errorf("a build still starting reports %q, want kb-root, outcome, state, state-dir, pid, wait and warnings alone", keys)
	}
}

// TestMCPResourcesAreTheSheetsOnDisk lists the staged KB's sheets through the
// server, requires exactly the claim-graph sheets on disk, and reads each
// back byte-identical.
func TestMCPResourcesAreTheSheetsOnDisk(t *testing.T) {
	srv, _ := mcpServer(t)
	// The server offers the sheets and the model guidance; this test is the
	// sheets, which the guidance's own test leaves alone.
	repo, _ := srv.Located.Root()
	kbRoot := filepath.Join(repo, "kb-root")
	var want []string
	for _, pattern := range []string{"*.svg", "*/claim-graph.svg"} {
		matches, err := filepath.Glob(filepath.Join(kbRoot, pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			rel, err := filepath.Rel(kbRoot, m)
			if err != nil {
				t.Fatal(err)
			}
			want = append(want, filepath.ToSlash(rel))
		}
	}
	if len(want) == 0 {
		t.Fatal("the staged KB's refresh left no sheet on disk")
	}
	slices.Sort(want)
	var listed struct {
		Resources []protocol.Resource `json:"resources"`
	}
	serveOne(t, srv, "resources/list", nil, &listed)
	var got []string
	for _, r := range listed.Resources {
		if !strings.HasPrefix(r.URI, mcp.SheetURIPrefix) {
			continue
		}
		got = append(got, r.Name)
		if r.URI != "kbase://sheet/"+r.Name || r.MimeType != "image/svg+xml" {
			t.Errorf("listed %+v", r)
		}
		var read struct {
			Contents []struct {
				URI, MimeType, Text string
			} `json:"contents"`
		}
		serveOne(t, srv, "resources/read", map[string]any{"uri": r.URI}, &read)
		disk, err := os.ReadFile(filepath.Join(kbRoot, filepath.FromSlash(r.Name)))
		if err != nil {
			t.Fatal(err)
		}
		if len(read.Contents) != 1 || read.Contents[0].URI != r.URI || read.Contents[0].Text != string(disk) {
			t.Errorf("%s read back differs from the file on disk", r.URI)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("listed %v, the sheets on disk are %v", got, want)
	}
}

// requireHelpResources requires listed to be the help resources, each
// text/plain and named "<name> help": one per family tool,
// kbase://help/<family>, then one per bound command, in the command tree's
// order, its kbase://help/<command>.
func requireHelpResources(t *testing.T, listed []protocol.Resource) {
	t.Helper()
	var want []protocol.Resource
	for _, f := range toolFamilies() {
		want = append(want, protocol.Resource{URI: mcp.HelpURIPrefix + f.Name, Name: f.Name + " help", MimeType: "text/plain"})
	}
	for _, c := range rootCmd.Commands() {
		if c.Annotations[mcp.BindingAnnotation] == mcp.Bound {
			want = append(want, protocol.Resource{URI: mcp.HelpURIPrefix + c.Name(), Name: c.Name() + " help", MimeType: "text/plain"})
		}
	}
	if len(want) == 0 || !slices.Equal(listed, want) {
		t.Errorf("after the guidance, resources/list is %+v, want each bound command's help %+v", listed, want)
	}
}

// TestMCPModelHintsResource: the model guidance is one resource beside the
// sheets — same list, same read — read as markdown, and needing no KB. A KB
// the sheet family refuses leaves the guidance listed and readable.
func TestMCPModelHintsResource(t *testing.T) {
	srv, _ := mcpServer(t)

	var listed struct {
		Resources []protocol.Resource `json:"resources"`
	}
	serveOne(t, srv, "resources/list", nil, &listed)
	// The sheets listed, wherever they stand in the list; which sheets they
	// are is TestMCPResourcesAreTheSheetsOnDisk's.
	var onDisk []protocol.Resource
	for _, r := range listed.Resources {
		if strings.HasPrefix(r.URI, mcp.SheetURIPrefix) {
			onDisk = append(onDisk, r)
		}
	}
	if len(listed.Resources) < len(onDisk)+1 {
		t.Fatalf("resources/list is %+v, want the %d sheets and the guidance", listed.Resources, len(onDisk))
	}
	requireHelpResources(t, listed.Resources[len(onDisk)+1:])
	if !slices.Equal(listed.Resources[:len(onDisk)], onDisk) {
		t.Errorf("the sheets list as %+v, want %+v", listed.Resources[:len(onDisk)], onDisk)
	}
	if got := listed.Resources[len(onDisk)]; got.URI != mcp.ModelHintsURI || got.Name != "Model hints" || got.MimeType != "text/markdown" {
		t.Errorf("the guidance lists as %+v", got)
	}

	var read struct {
		Contents []struct {
			URI, MimeType, Text string
		} `json:"contents"`
	}
	serveOne(t, srv, "resources/read", map[string]any{"uri": mcp.ModelHintsURI}, &read)
	if len(read.Contents) != 1 || read.Contents[0].URI != mcp.ModelHintsURI ||
		read.Contents[0].MimeType != "text/markdown" || read.Contents[0].Text != config.ModelHintsMarkdown() {
		t.Errorf("%s reads back %+v", mcp.ModelHintsURI, read.Contents)
	}

	var resp struct {
		Error struct {
			Code int
		} `json:"error"`
	}
	lines := serveLines(t, srv, []string{`{"jsonrpc":"2.0","id":9,"method":"resources/read","params":{"uri":"kbase://absent"}}`})
	if len(lines) != 1 || json.Unmarshal([]byte(lines[0]), &resp) != nil || resp.Error.Code != -32002 {
		t.Errorf("an unlisted URI answered %q, want -32002", lines)
	}

	// What a list holds where it lists no sheet: the guidance, then each bound
	// command's help.
	want := []protocol.Resource{{URI: mcp.ModelHintsURI, Name: "Model hints", MimeType: "text/markdown"}}
	// listedBy is what a server's resources/list answers before the help
	// resources; a fault in the list is an error response, which fails the
	// test.
	listedBy := func(srv mcp.Server) []protocol.Resource {
		var listed struct {
			Resources []protocol.Resource `json:"resources"`
		}
		serveOne(t, srv, "resources/list", nil, &listed)
		if len(listed.Resources) < len(want) {
			return listed.Resources
		}
		return listed.Resources[:len(want)]
	}

	// A KB that exists but will not open: the sheet family refuses it, so that
	// family lists nothing, while the composite lists around it rather than
	// failing the list.
	broken := t.TempDir()
	gitRepo(t, broken)
	if err := os.Mkdir(filepath.Join(broken, "kb-root"), 0o755); err != nil {
		t.Fatal(err)
	}
	var refusal toolresult.Refusal
	if src, err := servedKB(broken); !errors.As(err, &refusal) || src != nil {
		t.Errorf("a KB that will not open: the sheet family opened %v, %v; want its refusal", src, err)
	}
	apart := servedServer(rootCmd, mcp.LocatedAt(broken), &bytes.Buffer{}, log.Discard())
	if together := listedBy(apart); !slices.Equal(together, want) {
		t.Errorf("a KB that will not open: the composite listed %v; want %v", together, want)
	}

	// The guidance is still readable there: what it states needs no KB.
	var guidance struct {
		Contents []struct {
			URI, MimeType, Text string
		} `json:"contents"`
	}
	serveOne(t, apart, "resources/read", map[string]any{"uri": mcp.ModelHintsURI}, &guidance)
	if len(guidance.Contents) != 1 || guidance.Contents[0].Text != config.ModelHintsMarkdown() {
		t.Errorf("with a KB that will not open, %s reads back %+v", mcp.ModelHintsURI, guidance.Contents)
	}

	// A server with no located project lists no sheet and raises no fault: the
	// guidance is what it lists before the help.
	nowhere := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
	if together := listedBy(nowhere); !slices.Equal(together, want) {
		t.Errorf("with no located project the composite answered %v; want %v", together, want)
	}
}

// requireNoProjectRefusal requires doc to be the refusal of a call on a server
// with no located project that names none.
func requireNoProjectRefusal(t *testing.T, what, doc string) {
	t.Helper()
	var d struct {
		Outcome  string
		Refusals []map[string]any
	}
	if err := yaml.Unmarshal([]byte(doc), &d); err != nil || d.Outcome != toolresult.Refused || len(d.Refusals) != 1 {
		t.Fatalf("%s: %v\n%s; want one refusal", what, err, doc)
	}
	item := d.Refusals[0]
	if item["check"] != toolresult.CheckUsage || item["key"] != "project-dir" ||
		item["detail"] != "this server has no project of its own and the call names none" ||
		item["remedy"] != "name the project with project-dir: the absolute path of its git worktree root" {
		t.Errorf("%s: refused %v, want the usage refusal naming project-dir", what, item)
	}
}

// trailRepo stages the results fixture's KB in a fresh repository, converted
// by update-format so a write lands there, and records
// two of the build's boundaries there through gitledger, as a build records
// them — no build runs, so the test needs no pandoc — and returns the
// repository and the last boundary's commit.
func trailRepo(t *testing.T) (string, string) {
	t.Helper()
	repo, err := filepath.EvalSymlinks(stageRepo(t, resultsFixture))
	if err != nil {
		t.Fatal(err)
	}
	gitRepo(t, repo)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	isolateChildBuild(t, base)
	updateFormatKB(t, repo)
	// The paths a build owns in a KB: kb-root/ and the values records. The
	// boundaries commit them, so the worktree is clean when the tool reads it.
	led, err := gitledger.Open(repo, []string{kb.KBDir, "values"}, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	var commit string
	for _, id := range []string{"start", "document-graph"} {
		s, ok := buildstore.ResolveStage(id)
		if !ok {
			t.Fatalf("%q names no stage", id)
		}
		if commit, err = led.Record(s.ID, s.Display, ""); err != nil {
			t.Fatal(err)
		}
	}
	return repo, commit
}

// TestBuildTrailIsOneResultOnBothSurfaces: build-trail at the command line
// and over MCP gives the same result — its keys, the head, each
// boundary's stage, commit and subject, started-at and dirty — each boundary's
// command the reset, git reset --keep <commit>, then the build resumed, which
// each surface spells as it takes it. A dirty tree is reported rather than
// refused.
func TestBuildTrailIsOneResultOnBothSurfaces(t *testing.T) {
	repo, last := trailRepo(t)
	srv := servedServer(rootCmd, mcp.LocatedAt(repo), &bytes.Buffer{}, log.Discard())

	_, overMCP, isError := callToolResult(t, srv, "build-trail", map[string]any{})
	out, code := kbase(t, repo, "build-trail")
	atCLI := checkResult(t, "build-trail", out)
	if isError || overMCP["outcome"] != toolresult.Done || overMCP["head"] != last || code != 0 || atCLI.Outcome != toolresult.Done {
		t.Fatalf("build-trail: over MCP isError %v, %v; at the CLI exit %d\n%s", isError, overMCP, code, out)
	}
	if mcpKeys := slices.Sorted(maps.Keys(overMCP)); !slices.Equal(mcpKeys, slices.Sorted(slices.Values(atCLI.Keys))) {
		t.Errorf("over MCP the keys are %q, at the CLI %q", mcpKeys, atCLI.Keys)
	}
	for _, key := range []string{"kb-root", "state-dir", "head", "started-at", "dirty"} {
		if fmt.Sprint(overMCP[key]) != fmt.Sprint(atCLI.Values[key]) {
			t.Errorf("%s: over MCP %v, at the CLI %v", key, overMCP[key], atCLI.Values[key])
		}
	}
	mcpBoundaries, _ := overMCP["boundaries"].([]any)
	cliBoundaries := atCLI.items("boundaries")
	if len(mcpBoundaries) != 2 || len(cliBoundaries) != 2 {
		t.Fatalf("boundaries: over MCP %v, at the CLI %v; want the 2 recorded", mcpBoundaries, cliBoundaries)
	}
	for i, b := range mcpBoundaries {
		m, _ := b.(map[string]any)
		c := cliBoundaries[i]
		for _, key := range []string{"stage", "commit", "subject"} {
			if m[key] != c[key] {
				t.Errorf("boundary %d %s: over MCP %v, at the CLI %v", i, key, m[key], c[key])
			}
		}
		reset := "git reset --keep " + fmt.Sprint(m["commit"])
		if got := fmt.Sprint(m["command"]); got != fmt.Sprint([]any{reset, `build {"resume":true}`}) {
			t.Errorf("boundary %d command over MCP = %s, want the reset then the build tool's resume call", i, got)
		}
		if got := fmt.Sprint(c["command"]); got != fmt.Sprint([]any{reset, "kbase build --resume"}) {
			t.Errorf("boundary %d command at the CLI = %s, want the reset then kbase build --resume", i, got)
		}
	}

	// Dirt is a warning, never a refusal: the result names what a reset
	// would touch.
	writeValues(t, repo, "notes.md", "uncommitted\n")
	_, dirtyDoc, isError := callToolResult(t, srv, "build-trail", map[string]any{})
	paths, _ := dirtyDoc["dirty"].([]any)
	if isError || dirtyDoc["outcome"] != "done" || !slices.Contains(paths, any("notes.md")) {
		t.Errorf("a dirty tree: isError %v, %v", isError, dirtyDoc)
	}
}

// TestMCPCallRunsInItsProject: a call runs in the project its project-dir
// names, else in the server's located project, and a project-dir that is no
// git worktree root is refused as no worktree; a server that locates none
// requires project-dir, which TestMCPProjectRequiredLocatesNoProject holds it
// to. Each
// tool kind is held to it: a command and a write op.
func TestMCPCallRunsInItsProject(t *testing.T) {
	srvA, _ := mcpServer(t)
	a, _ := srvA.Located.Root()
	b, last := trailRepo(t)
	none := servedServer(rootCmd, mcp.ProjectRequired(), &bytes.Buffer{}, log.Discard())

	for _, tc := range []struct {
		name   string
		srv    mcp.Server
		args   map[string]any
		kbRoot string
	}{
		{"a call naming another project runs there", srvA, map[string]any{"project-dir": b}, filepath.Join(b, "kb-root")},
		{"a call naming none runs in the located project", srvA, map[string]any{}, filepath.Join(a, "kb-root")},
		{"a server with none runs a call where it names", none, map[string]any{"project-dir": a}, filepath.Join(a, "kb-root")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name, args := asOperation(t, "stats", nil)
			maps.Copy(args, tc.args)
			if stats, isError := callTool(t, tc.srv, name, args); isError || stats["kb-root"] != tc.kbRoot {
				t.Errorf("stats: isError %v, %v; want kb-root %s", isError, stats, tc.kbRoot)
			}
		})
	}

	t.Run("a project-dir that is no worktree root is refused", func(t *testing.T) {
		sub := filepath.Join(b, "kb-root")
		name, args := asOperation(t, "stats", nil)
		args["project-dir"] = sub
		stats, isError := callTool(t, srvA, name, args)
		items, _ := stats["refusals"].([]any)
		if !isError || stats["outcome"] != toolresult.Refused || len(items) != 1 {
			t.Fatalf("stats naming %s: isError %v, %v; want one refusal", sub, isError, stats)
		}
		if item, _ := items[0].(map[string]any); item["check"] != checkWorktree || item["key"] != "project-dir" || item["path"] != sub {
			t.Errorf("stats naming %s refused %v, want the worktree refusal of project-dir", sub, item)
		}
	})

	t.Run("a write batch naming another project writes there", func(t *testing.T) {
		added, isError := callTool(t, srvA, write.Batch, map[string]any{"project-dir": b, "entry": []any{map[string]any{
			"operation": "add-claim", "title": "B's result", "rigor": 0.5, "rationale": "Shown.", "document": "a.md", "line": 4, "snippet": "A"}}})
		var ids []any
		if entries, _ := added["entries"].([]any); len(entries) == 1 {
			e, _ := entries[0].(map[string]any)
			ids = append(ids, e["id"])
		}
		if isError || added["outcome"] != toolresult.Done || added["kb-root"] != filepath.Join(b, "kb-root") || len(ids) != 1 {
			t.Fatalf("add-claim: isError %v, %v", isError, added)
		}
		for _, tc := range []struct {
			project string
			args    map[string]any
			found   bool
		}{{"B", map[string]any{"project-dir": b}, true}, {"A", map[string]any{}, false}} {
			name, args := asOperation(t, "show", map[string]any{"id": ids[0]})
			maps.Copy(args, tc.args)
			if shown, isError := callTool(t, srvA, name, args); isError == tc.found {
				t.Errorf("show %v in %s: isError %v, %v; want it found %v", ids[0], tc.project, isError, shown, tc.found)
			}
		}
	})

	t.Run("build-trail naming another project reports its trail", func(t *testing.T) {
		trail, isError := callTool(t, srvA, "build-trail", map[string]any{"project-dir": b})
		if isError || trail["outcome"] != toolresult.Done || trail["head"] != last || trail["kb-root"] != filepath.Join(b, "kb-root") {
			t.Errorf("build-trail naming %s: isError %v, %v; want its trail at %s", b, isError, trail, last)
		}
	})
}

// TestMCPRefusesProjectDir: kbase mcp given the global --project-dir is a
// usage refusal of that flag before it serves, whatever the path names.
func TestMCPRefusesProjectDir(t *testing.T) {
	resetProjectDir(t)
	repo := realPath(t, t.TempDir())
	gitRepo(t, repo)
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	rootCmd.SetIn(strings.NewReader(""))
	out, code := kbase(t, t.TempDir(), "--project-dir", repo, "mcp")
	d := parseResult(t, out)
	items := d.items(toolresult.RefusalsKey)
	if d.Outcome != toolresult.Refused || code != 1 || len(items) != 1 {
		t.Fatalf("outcome %s, exit %d, %d items; want refused, 1, one item\n%s", d.Outcome, code, len(items), out)
	}
	item := items[0]
	if item["check"] != toolresult.CheckUsage || item["key"] != "--project-dir" ||
		item["detail"] != "kbase mcp takes no project; each tool call names its own with project-dir" ||
		item["remedy"] != "run `kbase mcp` from inside the project, or `kbase mcp --project-dir-required` where the harness has no project working directory" {
		t.Errorf("the refusal item = %v", item)
	}
}

// TestMCPProjectRequiredLocatesNoProject: kbase mcp --project-dir-required
// started inside a repository whose KB has sheets locates no project there. A
// call omitting project-dir fails the schema that requires it, naming it; one
// naming the empty project reaches the no-project refusal behind the schema;
// and no sheet is listed.
func TestMCPProjectRequiredLocatesNoProject(t *testing.T) {
	srv, _ := mcpServer(t)
	repo, _ := srv.Located.Root()
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"operation":"stats"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"query","arguments":{"operation":"stats","project-dir":""}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/list"}`,
	}
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	rootCmd.SetIn(strings.NewReader(strings.Join(requests, "\n") + "\n"))
	out, code := kbase(t, repo, "mcp", "--project-dir-required")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	type response struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	responses := map[int]response{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var resp struct {
			ID int `json:"id"`
			response
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response %q: %v", line, err)
		}
		responses[resp.ID] = resp.response
	}
	if len(responses) != len(requests) {
		t.Fatalf("%d responses for %d requests:\n%s", len(responses), len(requests), out)
	}

	if e := responses[1].Error; e == nil || e.Code != -32602 || !strings.Contains(e.Message, `"project-dir"`) {
		t.Errorf("stats omitting project-dir answered %+v; want -32602 naming project-dir", responses[1])
	}

	var call struct {
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
	}
	if err := json.Unmarshal(responses[2].Result, &call); err != nil || len(call.Content) != 1 || !call.IsError {
		t.Fatalf("stats naming the empty project: %+v; want one error result", responses[2])
	}
	requireNoProjectRefusal(t, "stats", call.Content[0].Text)

	var listed struct {
		Resources []protocol.Resource `json:"resources"`
	}
	if err := json.Unmarshal(responses[3].Result, &listed); err != nil || len(listed.Resources) == 0 {
		t.Fatalf("resources/list: %+v", responses[3])
	}
	for _, r := range listed.Resources {
		if strings.HasPrefix(r.URI, mcp.SheetURIPrefix) {
			t.Errorf("resources/list lists the sheet %s of the directory the server started in", r.URI)
		}
	}
}

// TestMCPProjectDirSchemaPerMode: every tool's project-dir is optional on a
// server that may locate a project of its own, and its description there
// carries a clause about that project after what the property names. On a
// server whose every call names its project the property is required and
// its description is what it names alone. The rest of the schema is alike.
func TestMCPProjectDirSchemaPerMode(t *testing.T) {
	located, _ := mcpServer(t)
	optional := map[string]listedTool{}
	for _, tool := range toolsListedBy(t, located) {
		optional[tool.Name] = tool
	}
	requiring := toolsListedBy(t, servedServer(rootCmd, mcp.ProjectRequired(), &bytes.Buffer{}, log.Discard()))
	if len(requiring) != len(optional) {
		t.Fatalf("%d tools where the project is required, %d where it is optional", len(requiring), len(optional))
	}
	for _, req := range requiring {
		opt, ok := optional[req.Name]
		if !ok {
			t.Errorf("%s is listed only where the project is required", req.Name)
			continue
		}
		if !slices.Contains(req.InputSchema.Required, "project-dir") {
			t.Errorf("%s: required %v lacks project-dir where every call names its project", req.Name, req.InputSchema.Required)
		}
		if slices.Contains(opt.InputSchema.Required, "project-dir") {
			t.Errorf("%s: required %v holds project-dir where the server may locate its own", req.Name, opt.InputSchema.Required)
		}
		others := slices.DeleteFunc(slices.Clone(req.InputSchema.Required), func(k string) bool { return k == "project-dir" })
		if !slices.Equal(others, opt.InputSchema.Required) {
			t.Errorf("%s: required %v beside project-dir, %v where it is optional", req.Name, others, opt.InputSchema.Required)
		}
		meaning, _ := req.InputSchema.Properties["project-dir"]["description"].(string)
		described, _ := opt.InputSchema.Properties["project-dir"]["description"].(string)
		if meaning == "" || !strings.HasPrefix(described, meaning) || len(described) <= len(meaning) {
			t.Errorf("%s: project-dir described %q where required, %q where optional; want the optional one to be the required one followed by its own clause",
				req.Name, meaning, described)
		}
	}
}

// TestMCPInitializeNamesWhatItServes drives initialize and tools/list through
// kbase mcp over the states a server can start in. Every answer carries the
// instructions, holding the .index/ rule, the session rule and the
// edge-direction sentence in that order, and what opens them is the mode's:
// located, the KB's absolute path or the repository's; not located, a line
// naming project-dir and no path. A server under no repository serves exactly
// as one given --project-dir-required: the same instructions, and project-dir
// required in every tool's schema. No unserved command's name is in an
// answer, build occurs only in the phrase its state chooses, and no marker is
// left unspelled. The transcripts pin each mode's bytes.
func TestMCPInitializeNamesWhatItServes(t *testing.T) {
	// What the server serves: its tools, and each family tool's operations.
	bound := map[string]bool{}
	for _, tool := range listTools(t) {
		bound[tool.Name] = true
		operations, _ := tool.InputSchema.Properties["operation"]["enum"].([]any)
		for _, op := range operations {
			name, _ := op.(string)
			bound[name] = true
		}
	}
	// The instructions may name only what the server serves: a command the
	// tree holds but the tool table does not is refused by name.
	unserved := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		if !bound[c.Name()] {
			unserved[c.Name()] = true
		}
	}
	// "build" is counted separately below, so it must be a served tool, and
	// the table must be non-empty, or the check proves nothing.
	if !bound["build"] || len(unserved) == 0 {
		t.Fatalf("the tool table lacks build, or the tree holds no unserved command: bound %v unserved %v", bound, unserved)
	}
	cobraOwn := cobraCommands()
	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"

	repo := realPath(t, stageRepo(t, resultsFixture))
	kbRoot := filepath.Join(repo, "kb-root")
	unbuilt := filepath.Join(realPath(t, t.TempDir()), "repo")
	if err := os.MkdirAll(unbuilt, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepo(t, unbuilt)
	nowhere := realPath(t, t.TempDir())

	t.Cleanup(func() { rootCmd.SetIn(nil) })
	// serve is what initialize and tools/list answer from a server started
	// in dir.
	serve := func(t *testing.T, dir string, args ...string) (string, []listedTool) {
		t.Helper()
		rootCmd.SetIn(strings.NewReader(requests))
		out, code := kbase(t, dir, args...)
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if code != 0 || len(lines) != 2 {
			t.Fatalf("exit %d, %d lines: %s", code, len(lines), out)
		}
		var initialized struct {
			Result struct {
				Instructions *string `json:"instructions"`
			} `json:"result"`
		}
		var listed struct {
			Result struct {
				Tools []listedTool `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(lines[0]), &initialized); err != nil {
			t.Fatalf("%s: %v", lines[0], err)
		}
		if err := json.Unmarshal([]byte(lines[1]), &listed); err != nil || len(listed.Result.Tools) == 0 {
			t.Fatalf("%s: %v", lines[1], err)
		}
		if initialized.Result.Instructions == nil {
			t.Fatal("no instructions")
		}
		return *initialized.Result.Instructions, listed.Result.Tools
	}
	// notLocated is what a server given --project-dir-required answers.
	notLocated, _ := serve(t, repo, "mcp", "--project-dir-required")

	for _, tc := range []struct {
		name, dir string
		args      []string
		// wantOpening is what a located server's opening begins with; empty
		// where the server locates no project.
		wantOpening string
		// buildPhrase is the one phrase build may occur in, if any.
		buildPhrase string
	}{
		{name: "a repository holding kb-root/", dir: repo, args: []string{"mcp"},
			wantOpening: "kbase serves the knowledge base at " + kbRoot},
		{name: "a repository whose kb-root/ is yet to be made", dir: unbuilt, args: []string{"mcp"},
			wantOpening: "kbase serves the repository at " + unbuilt, buildPhrase: "can build its knowledge base"},
		{name: "no repository", dir: nowhere, args: []string{"mcp"}},
		{name: "project-dir required, inside a repository", dir: repo, args: []string{"mcp", "--project-dir-required"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, tools := serve(t, tc.dir, tc.args...)
			at := 0
			for _, part := range []string{kbdocs.IndexRule(), kbdocs.SessionRule(), mcp.EdgeDirection} {
				i := strings.Index(text[at:], part)
				if i < 0 {
					t.Errorf("instructions %q lack %q after byte %d", text, part, at)
					break
				}
				at += i + len(part)
			}
			if tc.wantOpening != "" {
				if !strings.HasPrefix(text, tc.wantOpening) {
					t.Errorf("instructions %q, want them opening %q", text, tc.wantOpening)
				}
			} else if text != notLocated || !slices.Contains(instructionsTokens(text), "project-dir") ||
				strings.Contains(text, repo) || strings.Contains(text, tc.dir) {
				t.Errorf("instructions %q, want --project-dir-required's %q, naming project-dir and no path", text, notLocated)
			}
			for _, tool := range tools {
				if required := slices.Contains(tool.InputSchema.Required, "project-dir"); required != (tc.wantOpening == "") {
					t.Errorf("%s: project-dir required %v, want it required exactly where the server locates no project", tool.Name, required)
				}
			}
			if argmark.Marked(text) {
				t.Errorf("instructions %q keep a marker", text)
			}
			// A kbase:// URI names a resource, not a command, though its
			// path reads as one (kbase://help/<command>).
			prose := strings.Join(slices.DeleteFunc(strings.Fields(text), func(word string) bool {
				return strings.HasPrefix(word, "kbase://")
			}), " ")
			builds := 0
			for _, token := range instructionsTokens(prose) {
				if token == "build" {
					builds++
					continue
				}
				// A word cobra names a command of its own by (help) is the
				// word, never that CLI-only command.
				if cobraOwn[token] {
					continue
				}
				if unserved[token] {
					t.Errorf("instructions %q name the command %q, which the server does not serve", text, token)
				}
			}
			wantBuilds := 0
			if tc.buildPhrase != "" {
				wantBuilds = 1
			}
			if builds != wantBuilds {
				t.Errorf("instructions %q say build %d times, want %d", text, builds, wantBuilds)
			}
			if builds == 1 && strings.Count(text, tc.buildPhrase) != 1 {
				t.Errorf("instructions %q say build outside the phrase %q", text, tc.buildPhrase)
			}
		})
	}
}

// cobraCommands is the names of the commands cobra adds to a tree of its own —
// the ones cmd/surface.go declares cliOnly after adding them — read off a tree
// holding one command of ours, which cobra requires before it adds any.
func cobraCommands() map[string]bool {
	const ours = "ours"
	tree := &cobra.Command{Use: "tree"}
	tree.AddCommand(&cobra.Command{Use: ours})
	tree.InitDefaultHelpCmd()
	tree.InitDefaultCompletionCmd()
	names := map[string]bool{}
	for _, c := range tree.Commands() {
		if c.Name() != ours {
			names[c.Name()] = true
		}
	}
	return names
}

// instructionsTokens splits text on every character that is neither a letter,
// a digit nor a hyphen, so show-config is one token and no tool name is found
// inside another.
func instructionsTokens(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
}
