package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/config"
	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/version"
	"kbase/internal/write"
)

const (
	mcpTranscriptDir = "../test_data/fixtures/mcp"
	resultsFixture   = "../test_data/fixtures/results"
	// noResponse is the transcript line standing where a request is owed
	// no response.
	noResponse = "null"
)

// runAsKBaseEnv makes this test binary run as kbase itself, so the build
// tool's child — os.Executable(), this binary under test — is the build the
// shipped binary would start.
const runAsKBaseEnv = "KBASE_TEST_RUN_AS_KBASE"

func TestMain(m *testing.M) {
	if os.Getenv(runAsKBaseEnv) != "" {
		main()
	}
	os.Exit(m.Run())
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
		"XDG_STATE_HOME": filepath.Join(base, "xdg"), "KBASE_CONFIG_DIR": filepath.Join(base, "config"),
	} {
		t.Setenv(k, v)
	}
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

// mcpServer stages the results fixture KB with the transcripts' seed values
// written — two claims alike but for their ids, so a page of them reads the
// same whichever id is minted lower — and returns the tool table and the
// resources over it and the transcripts' placeholders: ${KB_ROOT},
// ${REPO} (the repository holding it), ${STATE_DIR} (an empty state store
// beside it), ${VERSION}, and ${ID1}, ${ID2} for the seeded claims' ids, in
// id order.
func mcpServer(t *testing.T) ([]mcp.Tool, mcp.Resources, *strings.Replacer) {
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
	if err := os.Mkdir(filepath.Join(repo, "kb-root", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code, err := runWriteOp(writeOpOptions{Op: "insert-claim-entry", WorkDir: repo, ValuesFile: filepath.Join(mcpTranscriptDir, "seed.yaml"),
		Create: true, Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	var inserted struct {
		KBRoot string   `yaml:"kb-root"`
		IDs    []string `yaml:"ids"`
	}
	if err != nil || code != 0 || yaml.Unmarshal(out.Bytes(), &inserted) != nil || len(inserted.IDs) != 2 {
		t.Fatalf("staging insert: exit %d, %v\n%s", code, err, out.String())
	}
	kbRoot := filepath.Join(repo, "kb-root")
	tools, err := mcpTools(rootCmd, kbRoot, &bytes.Buffer{}, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(inserted.IDs)
	return tools, sheetResources{kbRoot}, strings.NewReplacer("${KB_ROOT}", jsonText(t, inserted.KBRoot), "${REPO}", jsonText(t, repo),
		"${STATE_DIR}", jsonText(t, filepath.Join(base, "state")), "${VERSION}", jsonText(t, version.Current),
		"${ID1}", inserted.IDs[0], "${ID2}", inserted.IDs[1])
}

func serveLines(t *testing.T, tools []mcp.Tool, resources mcp.Resources, requests []string) []string {
	t.Helper()
	var out bytes.Buffer
	if err := mcp.Serve(strings.NewReader(strings.Join(requests, "\n")+"\n"), &out, version.Current, tools, resources, log.Discard()); err != nil {
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
// the results fixture, and requires exactly the expected lines on stdout.
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
			tools, resources, placeholders := mcpServer(t)
			var requests, want []string
			for i := 0; i < len(lines); i += 2 {
				requests = append(requests, placeholders.Replace(lines[i]))
				if lines[i+1] != noResponse {
					want = append(want, placeholders.Replace(lines[i+1]))
				}
			}
			got := serveLines(t, tools, resources, requests)
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
	tools, resources, _ := mcpServer(t)
	got := serveLines(t, tools, resources, []string{`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`})
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

func TestMCPToolsAreTheCommandTree(t *testing.T) {
	readOnly := []string{"cited-by", "deps", "find", "gated-on", "referenced-by", "render-citation", "show",
		"solidity-below", "stats", "status", "subtree", "verify", "weak-points"}
	var want []string
	for _, c := range rootCmd.Commands() {
		switch binding := c.Annotations[mcpAnnotation]; {
		case binding == mcpBound:
			want = append(want, c.Name())
		case binding == mcpExcluded, c.Name() == "help", c.Name() == "completion":
		default:
			t.Errorf("%s declares no MCP binding: mcpBinding it %s or %s where it is added", c.Name(), mcpBound, mcpExcluded)
		}
	}
	var names, gotReadOnly []string
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
		for key := range s.Properties {
			if rootCmd.PersistentFlags().Lookup(key) != nil {
				t.Errorf("%s: takes the global flag %q", tool.Name, key)
			}
		}
		if cmd, _, err := rootCmd.Find([]string{tool.Name}); err != nil || tool.Description != cmd.Short {
			t.Errorf("%s: description %q is not the command's Short", tool.Name, tool.Description)
		} else if declared, err := declaredPositionals(cmd); err != nil {
			t.Error(err)
		} else {
			for _, p := range declared {
				if !slices.Contains(s.Required, p.name) {
					t.Errorf("%s: positional %q is not required", tool.Name, p.name)
				}
			}
		}
	}
	slices.Sort(want)
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Errorf("tools %v, want the commands declared %s: %v", names, mcpBound, want)
	}
	slices.Sort(gotReadOnly)
	if !slices.Equal(gotReadOnly, readOnly) {
		t.Errorf("read-only tools %v, want %v", gotReadOnly, readOnly)
	}
}

// TestMCPWriteOpSchemas reads each write op's vocabulary off its own
// refusal — every key it allows, every required key it misses — and
// requires the tool's schema to be exactly that, plus create where the op
// takes --create.
func TestMCPWriteOpSchemas(t *testing.T) {
	const unknown = "not-a-values-key"
	seen := 0
	for _, tool := range listTools(t) {
		if _, _, ok := write.OpVocabulary(tool.Name); !ok {
			continue
		}
		seen++
		res := write.Run(tool.Name, write.Options{KBRoot: t.TempDir(), Values: []byte(`{"entry":[{"` + unknown + `":1}]}`)})
		var allowed, required []string
		for _, r := range res.Refusals {
			if r.Key == unknown {
				allowed = r.Allowed
			} else {
				required = append(required, r.Key)
			}
		}
		if write.CreatesRegister(tool.Name) {
			allowed = append(allowed, createArgument)
		}
		var keys []string
		for k := range tool.InputSchema.Properties {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		slices.Sort(allowed)
		slices.Sort(required)
		gotRequired := slices.Sorted(slices.Values(tool.InputSchema.Required))
		if len(allowed) == 0 || !slices.Equal(keys, allowed) {
			t.Errorf("%s: schema keys %v, the op allows %v", tool.Name, keys, allowed)
		}
		if !slices.Equal(gotRequired, required) {
			t.Errorf("%s: schema requires %v, the op requires %v", tool.Name, gotRequired, required)
		}
	}
	if want := len(write.Ops()) + 1; seen != want {
		t.Errorf("%d write-op tools, want every op and render-citation: %d", seen, want)
	}
}

// TestCommandsDeclarePositionals requires every command's usage line to
// name exactly the positionals its argument check accepts.
func TestCommandsDeclarePositionals(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		declared, err := declaredPositionals(c)
		if err != nil {
			t.Error(err)
			continue
		}
		variadic := len(declared) > 0 && declared[len(declared)-1].variadic
		for k := range len(declared) + 3 {
			accepted := c.ValidateArgs(make([]string, k)) == nil
			if want := k == len(declared) || variadic && k > len(declared); accepted != want {
				t.Errorf("%s: %d positionals accepted %v; its usage line %q declares %d", c.Name(), k, accepted, c.Use, len(declared))
			}
		}
	}
}

// serveOne sends one request through the server and decodes its response's
// result into result.
func serveOne(t *testing.T, tools []mcp.Tool, resources mcp.Resources, method string, params map[string]any, result any) {
	t.Helper()
	req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	got := serveLines(t, tools, resources, []string{string(req)})
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
func callTool(t *testing.T, tools []mcp.Tool, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	var result struct {
		Structured map[string]any `json:"structuredContent"`
		IsError    bool           `json:"isError"`
	}
	serveOne(t, tools, sheetResources{}, "tools/call", map[string]any{"name": name, "arguments": args}, &result)
	return result.Structured, result.IsError
}

// TestMCPBuildStartsDetached starts a build through the tool against a
// provider that never answers, so the build stays in flight: the tool
// returns the start document while the build runs on, status reads it
// running under the pid the tool named, and cancel stops it resumably. The
// server's --log-file reaches the build: this test process opens no log, so
// the file exists only if the build opened it.
func TestMCPBuildStartsDetached(t *testing.T) {
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

	repo := filepath.Join(base, "repo")
	gitRepo(t, repo)
	paper, err := os.ReadFile(filepath.Join(mcpTranscriptDir, "paper.tex"))
	if err != nil {
		t.Fatal(err)
	}
	writeValues(t, repo, "paper.tex", string(paper))
	isolateChildBuild(t, base)
	t.Setenv(config.EnvAPIBaseURL, provider.URL)
	t.Setenv(config.EnvModel, "m")
	savedLevel, savedFile := flagLogLevel, flagLogFile
	t.Cleanup(func() { flagLogLevel, flagLogFile = savedLevel, savedFile })
	flagLogLevel, flagLogFile = "debug", filepath.Join(base, "build.log")
	tools, err := mcpTools(rootCmd, repo, &bytes.Buffer{}, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(base, "state")
	monitor := map[string]any{"state-dir": state}

	start, isError := callTool(t, tools, "build", map[string]any{"volume-root": []any{filepath.Join(repo, "paper.tex")}, "state-dir": state})
	t.Cleanup(func() { callTool(t, tools, "cancel", monitor) })
	pid, _ := start["pid"].(float64)
	resume, _ := start["resume"].(string)
	if isError || start["outcome"] != "done" || start["state-dir"] != state || pid <= 0 || !strings.Contains(resume, "--state-dir") {
		t.Fatalf("build: isError %v, %v", isError, start)
	}
	if status, _ := callTool(t, tools, "status", monitor); status["state"] != "running" || status["pid"] != pid {
		t.Errorf("status after the start: %v, want running under pid %v", status, pid)
	}
	if _, err := os.Stat(flagLogFile); err != nil {
		t.Errorf("the server's --log-file did not reach the build: %v", err)
	}
	if cancel, isError := callTool(t, tools, "cancel", monitor); isError || cancel["outcome"] != "done" || cancel["pid"] != pid {
		t.Errorf("cancel: isError %v, %v", isError, cancel)
	}
	if status, _ := callTool(t, tools, "status", monitor); status["state"] != "cancelled" || status["resume"] == nil {
		t.Errorf("status after cancel: %v, want cancelled with a resume command", status)
	}
}

// TestMCPResourcesAreTheSheetsOnDisk lists the staged KB's sheets through the
// server, requires exactly the claim-graph sheets on disk, and reads each
// back byte-identical.
func TestMCPResourcesAreTheSheetsOnDisk(t *testing.T) {
	tools, resources, _ := mcpServer(t)
	kbRoot := resources.(sheetResources).dir
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
		Resources []mcp.Resource `json:"resources"`
	}
	serveOne(t, tools, resources, "resources/list", nil, &listed)
	var got []string
	for _, r := range listed.Resources {
		got = append(got, r.Name)
		if r.URI != "kbase://sheet/"+r.Name || r.MimeType != "image/svg+xml" {
			t.Errorf("listed %+v", r)
		}
		var read struct {
			Contents []struct {
				URI, MimeType, Text string
			} `json:"contents"`
		}
		serveOne(t, tools, resources, "resources/read", map[string]any{"uri": r.URI}, &read)
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

// TestMCPKBRootNamesAKBRoot serves a --kb-root naming a repository root,
// with or without its kb-root/, or that kb-root/ itself, and refuses any
// other path before serving.
func TestMCPKBRootNamesAKBRoot(t *testing.T) {
	repo := stageRepo(t, resultsFixture)
	bare := t.TempDir()
	gitRepo(t, bare)
	if err := os.Mkdir(filepath.Join(bare, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	for _, tc := range []struct {
		name, path string
		serves     bool
	}{
		{"the kb-root", filepath.Join(repo, "kb-root"), true},
		{"the repository root holding it", repo, true},
		{"a directory inside the repository", filepath.Join(repo, "values"), false},
		{"a file inside the kb-root", filepath.Join(repo, "kb-root", "a.md"), false},
		{"a path that does not exist", filepath.Join(repo, "absent"), false},
		{"a repository root with no kb-root yet", bare, true},
		{"a directory inside a repository with no kb-root", filepath.Join(bare, "sub"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd.SetIn(strings.NewReader(""))
			out, code := kbase(t, t.TempDir(), "mcp", "--kb-root", tc.path)
			if tc.serves {
				if code != 0 || out != "" {
					t.Errorf("exit %d, stdout %q; want it served until EOF", code, out)
				}
				return
			}
			var doc struct {
				Outcome  string
				Refusals []map[string]any
			}
			if err := yaml.Unmarshal([]byte(out), &doc); err != nil || code != 1 || doc.Outcome != "refused" || len(doc.Refusals) != 1 ||
				doc.Refusals[0]["check"] != "usage" || doc.Refusals[0]["key"] != "--kb-root" {
				t.Errorf("exit %d: %s; want one usage refusal of --kb-root", code, out)
			}
		})
	}
}
