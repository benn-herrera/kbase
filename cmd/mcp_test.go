package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"go.yaml.in/yaml/v3"

	"kbase/internal/build"
	"kbase/internal/config"
	"kbase/internal/kb"
	"kbase/internal/latex/pandoc"
	"kbase/internal/ledger"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
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

// testServer is a server a test drives: the tool table, the resources, the
// subscription, and the instructions the server reports at initialize.
type testServer struct {
	tools        []mcp.Tool
	resources    mcp.Resources
	subscription *mcp.Subscription
	instructions string
}

// mcpServer stages the results fixture KB with the transcripts' seed values
// written — two claims alike but for their ids, so a page of them reads the
// same whichever id is minted lower — and returns the server over it and the
// transcripts' placeholders: ${KB_ROOT}, ${REPO} (the repository holding it),
// ${STATE_DIR} (an empty state store beside it), ${VERSION}, and ${ID1},
// ${ID2} for the seeded claims' ids, in id order.
func mcpServer(t *testing.T) (testServer, *strings.Replacer) {
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
	launch := &buildLaunch{served: kbRoot}
	tools, err := mcpTools(rootCmd, kbRoot, &bytes.Buffer{}, log.Discard(), launch)
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := servedInstructions(kbRoot)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(inserted.IDs)
	return testServer{tools: tools, resources: servedResources(kbRoot, launch), instructions: instructions,
			subscription: buildSubscription(launch, log.Discard())},
		strings.NewReplacer("${KB_ROOT}", jsonText(t, inserted.KBRoot), "${REPO}", jsonText(t, repo),
			"${STATE_DIR}", jsonText(t, filepath.Join(base, "state")), "${VERSION}", jsonText(t, version.Current),
			"${ID1}", inserted.IDs[0], "${ID2}", inserted.IDs[1])
}

func serveLines(t *testing.T, srv testServer, requests []string) []string {
	t.Helper()
	var out bytes.Buffer
	if err := mcp.Serve(strings.NewReader(strings.Join(requests, "\n")+"\n"), &out, version.Current, srv.instructions, srv.tools, srv.resources, srv.subscription, log.Discard()); err != nil {
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
			srv, placeholders := mcpServer(t)
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

// listedToolName is the one tool the table lists by hand, beside the tools its
// bound commands derive: the build trail, which no subcommand stands behind.
const listedToolName = "rewind-build"

func TestMCPToolsAreTheCommandTree(t *testing.T) {
	// want is every bound command plus the one listed tool, and the same set
	// is the read-only expectation. The listed tool is a deliberate exception,
	// so the equality below fails a second listed tool as it fails a missing
	// command.
	want := []string{listedToolName}
	readOnly := []string{listedToolName}
	for _, c := range rootCmd.Commands() {
		switch binding := c.Annotations[mcpAnnotation]; {
		case binding == mcpBound:
			want = append(want, c.Name())
		case binding == mcpExcluded, c.Name() == "help", c.Name() == "completion":
		default:
			t.Errorf("%s declares no MCP binding: mcpBinding it %s or %s where it is added", c.Name(), mcpBound, mcpExcluded)
		}
		if c.Annotations[readOnlyAnnotation] == "true" {
			readOnly = append(readOnly, c.Name())
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
		if tool.Name == listedToolName {
			// No command stands behind this one, so its description is its
			// own and must carry what the missing command Short would not:
			// that it is read-only.
			if !strings.Contains(tool.Description, "read-only") {
				t.Errorf("%s: description %q does not state that it is read-only", tool.Name, tool.Description)
			}
			if keys := slices.Sorted(maps.Keys(s.Properties)); !slices.Equal(keys, []string{stateDirFlag, "target"}) {
				t.Errorf("%s: takes %v, want %s and target", tool.Name, keys, stateDirFlag)
			}
			continue
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
	slices.Sort(readOnly)
	if !slices.Equal(gotReadOnly, readOnly) {
		t.Errorf("read-only tools %v, want the commands declaring %s: %v", gotReadOnly, readOnlyAnnotation, readOnly)
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
func serveOne(t *testing.T, srv testServer, method string, params map[string]any, result any) {
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
func callTool(t *testing.T, srv testServer, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	_, structured, isError := callToolResult(t, srv, name, args)
	return structured, isError
}

// callToolResult is one tools/call through the server: the result's text, its
// structured content and whether it is an error.
func callToolResult(t *testing.T, srv testServer, name string, args map[string]any) (string, map[string]any, bool) {
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
	t.Setenv(config.EnvDecision, "m")
	t.Setenv(config.EnvText, "m")
	savedLevel, savedFile := flagLogLevel, flagLogFile
	t.Cleanup(func() { flagLogLevel, flagLogFile = savedLevel, savedFile })
	flagLogLevel, flagLogFile = "debug", filepath.Join(base, "build.log")
	launch := &buildLaunch{served: repo}
	tools, err := mcpTools(rootCmd, repo, &bytes.Buffer{}, log.Discard(), launch)
	if err != nil {
		t.Fatal(err)
	}
	srv := testServer{tools: tools}
	state := filepath.Join(base, "state")
	monitor := map[string]any{"state-dir": state}

	start, isError := callTool(t, srv, "build", map[string]any{"volume-root": []any{filepath.Join(repo, "paper.tex")}, "state-dir": state})
	t.Cleanup(func() { callTool(t, srv, "cancel", monitor) })
	pid, _ := start["pid"].(float64)
	resume, _ := start["resume"].(string)
	if isError || start["outcome"] != "done" || start["state-dir"] != state || pid <= 0 || !strings.Contains(resume, "--state-dir") {
		t.Fatalf("build: isError %v, %v", isError, start)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wantWait := build.ShellJoin([]string{exe, "wait", "--state-dir", state, "--pid", strconv.Itoa(int(pid))})
	if wait, _ := start["wait"].(string); wait != wantWait {
		t.Errorf("build: wait = %q, want the exact command %q", wait, wantWait)
	}
	if status, _ := callTool(t, srv, "status", monitor); status["state"] != "running" || status["pid"] != pid {
		t.Errorf("status after the start: %v, want running under pid %v", status, pid)
	}
	if _, err := os.Stat(flagLogFile); err != nil {
		t.Errorf("the server's --log-file did not reach the build: %v", err)
	}
	if cancel, isError := callTool(t, srv, "cancel", monitor); isError || cancel["outcome"] != "done" || cancel["pid"] != pid {
		t.Errorf("cancel: isError %v, %v", isError, cancel)
	}
	if status, _ := callTool(t, srv, "status", monitor); status["state"] != "cancelled" || status["resume"] == nil {
		t.Errorf("status after cancel: %v, want cancelled with a resume command", status)
	}
	// The wait the tool handed out is the background command a session without
	// a subscription runs: the run it names is over, so it returns at once with
	// that store's status document.
	out, code := kbase(t, repo, "wait", "--state-dir", state, "--pid", strconv.Itoa(int(pid)))
	if doc := checkDocument(t, "wait", out); doc.Outcome != toolresult.Done || code != 0 ||
		doc.Values["state"] != "cancelled" || doc.Values["pid"] != int(pid) {
		t.Errorf("wait: outcome %s, exit %d, %v\n%s", doc.Outcome, code, doc.Values, out)
	}
}

// TestMCPResourcesAreTheSheetsOnDisk lists the staged KB's sheets through the
// server, requires exactly the claim-graph sheets on disk, and reads each
// back byte-identical.
func TestMCPResourcesAreTheSheetsOnDisk(t *testing.T) {
	srv, _ := mcpServer(t)
	// The server offers the sheets and the model guidance; this test is the
	// sheets, which the guidance's own test leaves alone.
	kbRoot := srv.resources.(resourceFamilies)[0].source.(sheetResources).dir
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
	serveOne(t, srv, "resources/list", nil, &listed)
	var got []string
	for _, r := range listed.Resources {
		if !strings.HasPrefix(r.URI, sheetURIPrefix) {
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

// TestMCPModelHintsResource: the model guidance is one resource beside the
// sheets — same list, same read — read as markdown, and needing no KB. A KB
// the sheet family refuses leaves the guidance and the trail listed, and the
// guidance readable.
func TestMCPModelHintsResource(t *testing.T) {
	srv, _ := mcpServer(t)
	sheets := srv.resources.(resourceFamilies)[0].source.(sheetResources)

	var listed struct {
		Resources []mcp.Resource `json:"resources"`
	}
	serveOne(t, srv, "resources/list", nil, &listed)
	onDisk, err := sheets.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Resources) != len(onDisk)+3 {
		t.Fatalf("resources/list is %+v, want the %d sheets, the guidance, the trail and the build state", listed.Resources, len(onDisk))
	}
	if !slices.Equal(listed.Resources[:len(onDisk)], onDisk) {
		t.Errorf("the sheets list as %+v, want %+v", listed.Resources[:len(onDisk)], onDisk)
	}
	if got := listed.Resources[len(onDisk)]; got.URI != modelHintsURI || got.Name != "Model hints" || got.MimeType != "text/markdown" {
		t.Errorf("the guidance lists as %+v", got)
	}
	if got := listed.Resources[len(onDisk)+1]; got.URI != buildTrailURI || got.Name != "Build trail" || got.MimeType != "application/yaml" {
		t.Errorf("the build trail lists as %+v", got)
	}
	if got := listed.Resources[len(onDisk)+2]; got.URI != buildURI || got.Name != "Build state" || got.MimeType != "application/yaml" {
		t.Errorf("the build state lists as %+v", got)
	}

	var read struct {
		Contents []struct {
			URI, MimeType, Text string
		} `json:"contents"`
	}
	serveOne(t, srv, "resources/read", map[string]any{"uri": modelHintsURI}, &read)
	if len(read.Contents) != 1 || read.Contents[0].URI != modelHintsURI ||
		read.Contents[0].MimeType != "text/markdown" || read.Contents[0].Text != config.ModelHintsMarkdown() {
		t.Errorf("%s reads back %+v", modelHintsURI, read.Contents)
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

	// What a list holds where it lists no sheet: the guidance, then the trail,
	// which reports the repository it cannot read, then the build's state.
	want := []mcp.Resource{{URI: modelHintsURI, Name: "Model hints", MimeType: "text/markdown"},
		{URI: buildTrailURI, Name: "Build trail", MimeType: "application/yaml"},
		{URI: buildURI, Name: "Build state", MimeType: "application/yaml"}}

	// A KB that exists but will not open: the sheet family refuses it, so that
	// family lists nothing, while the composite lists around it rather than
	// failing the list.
	broken := t.TempDir()
	gitRepo(t, broken)
	if err := os.Mkdir(filepath.Join(broken, "kb-root"), 0o755); err != nil {
		t.Fatal(err)
	}
	if sheets, err := (sheetResources{broken}).List(); err == nil || sheets != nil {
		t.Errorf("a KB that will not open: the sheet family listed %v, %v; want its refusal", sheets, err)
	}
	apart := servedResources(broken, &buildLaunch{served: broken})
	if together, err := apart.List(); err != nil || !slices.Equal(together, want) {
		t.Errorf("a KB that will not open: the composite listed %v, %v; want %v", together, err, want)
	}

	// The guidance is still readable there: what it states needs no KB.
	var guidance struct {
		Contents []struct {
			URI, MimeType, Text string
		} `json:"contents"`
	}
	serveOne(t, testServer{resources: apart}, "resources/read", map[string]any{"uri": modelHintsURI}, &guidance)
	if len(guidance.Contents) != 1 || guidance.Contents[0].Text != config.ModelHintsMarkdown() {
		t.Errorf("with a KB that will not open, %s reads back %+v", modelHintsURI, guidance.Contents)
	}

	// A directory holding no KB lists no sheet and raises no fault: the
	// guidance, and the trail, which reports the repository it cannot find,
	// are what it lists.
	nowhere := t.TempDir()
	if together, err := servedResources(nowhere, &buildLaunch{served: nowhere}).List(); err != nil || !slices.Equal(together, want) {
		t.Errorf("outside a KB the composite answered %v, %v; want %v", together, err, want)
	}
}

// stubResources answers List with what a test supplies and reads nothing.
type stubResources struct {
	listed []mcp.Resource
	err    error
}

func (s stubResources) List() ([]mcp.Resource, error) { return s.listed, s.err }

func (s stubResources) Read(mcp.Resource) (string, error) { return "", nil }

// TestResourceFamiliesListSkipsOnlyRefusals: a family refusing what it lists
// from contributes no entries and leaves the rest listing, while any other
// error is the list's and stops it.
func TestResourceFamiliesListSkipsOnlyRefusals(t *testing.T) {
	hints := []mcp.Resource{{URI: modelHintsURI}}
	refusing := stubResources{err: toolresult.Refusal{{Check: checkKBRoot, Path: "kb-root"}}}
	families := resourceFamilies{{source: refusing}, {source: stubResources{listed: hints}}}
	if listed, err := families.List(); err != nil || !slices.Equal(listed, hints) {
		t.Errorf("a refusing family: listed %v, %v; want %v", listed, err, hints)
	}
	failing := stubResources{err: errors.New("the KB's index is unreadable")}
	families = resourceFamilies{{source: stubResources{listed: hints}}, {source: failing}}
	if listed, err := families.List(); !errors.Is(err, failing.err) || listed != nil {
		t.Errorf("a failing family: listed %v, %v; want its error and no list", listed, err)
	}
}

// trailRepo stages the results fixture's KB in a fresh repository and records
// two of the build's boundaries there through the ledger, as a build records
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
	// The paths a build owns in a KB: kb-root/ and the values records. The
	// boundaries commit them, so the worktree is clean when the tool reads it.
	led, err := ledger.Open(repo, []string{kb.KBDir, "values"}, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	var commit string
	for _, id := range []string{"start", "document-graph"} {
		s, ok := build.ResolveStage(id)
		if !ok {
			t.Fatalf("%q names no stage", id)
		}
		if commit, err = led.Record(s.ID, s.Display, ""); err != nil {
			t.Fatal(err)
		}
	}
	return repo, commit
}

// TestMCPRewindBuildIsTheBuildTrail: the rewind-build tool and the
// kbase://build-trail resource answer with the same document, the tool as a
// result an agent calls and the resource as discovery listed beside the sheets
// and the model guidance. A target names a boundary, an unknown one is a usage
// refusal, and a dirty tree is reported rather than refused.
func TestMCPRewindBuildIsTheBuildTrail(t *testing.T) {
	repo, last := trailRepo(t)
	launch := &buildLaunch{served: repo}
	tools, err := mcpTools(rootCmd, repo, &bytes.Buffer{}, log.Discard(), launch)
	if err != nil {
		t.Fatal(err)
	}
	srv := testServer{tools: tools, resources: servedResources(repo, launch)}

	doc, structured, isError := callToolResult(t, srv, listedToolName, map[string]any{})
	if isError || structured["outcome"] != "done" || structured["head"] != last {
		t.Fatalf("%s: isError %v, %v", listedToolName, isError, structured)
	}
	if dirty, ok := structured["dirty"].(bool); !ok || dirty {
		t.Errorf("a clean worktree reports dirty %v, want false", structured["dirty"])
	}
	boundaries, _ := structured["boundaries"].([]any)
	if len(boundaries) != 2 {
		t.Errorf("the tool reports %d boundaries, want the 2 recorded", len(boundaries))
	}

	var read struct {
		Contents []struct {
			URI, MimeType, Text string
		} `json:"contents"`
	}
	serveOne(t, srv, "resources/read", map[string]any{"uri": buildTrailURI}, &read)
	if len(read.Contents) != 1 || read.Contents[0].URI != buildTrailURI ||
		read.Contents[0].MimeType != "application/yaml" || read.Contents[0].Text != doc {
		t.Errorf("%s reads back %+v, want the tool's document byte for byte", buildTrailURI, read.Contents)
	}

	var listed struct {
		Resources []mcp.Resource `json:"resources"`
	}
	serveOne(t, srv, "resources/list", nil, &listed)
	if len(listed.Resources) != 3 || listed.Resources[1].URI != buildTrailURI {
		t.Fatalf("resources/list is %+v, want the guidance, the build trail and the build state", listed.Resources)
	}
	if trail := listed.Resources[1]; trail.Name != "Build trail" || trail.MimeType != "application/yaml" {
		t.Errorf("the build trail lists as %+v", trail)
	}

	// A target names a boundary: the reset is that boundary's commit, and the
	// discards are the boundaries newer than it.
	_, targeted, isError := callToolResult(t, srv, listedToolName, map[string]any{"target": "start"})
	target, _ := targeted["target"].(map[string]any)
	commit, _ := target["commit"].(string)
	commands, _ := targeted["commands"].([]any)
	if isError || targeted["outcome"] != "done" || target["stage"] != "start" || target["discards"] != float64(1) ||
		len(commands) == 0 || commands[0] != "git reset --hard "+commit {
		t.Errorf("%s target start: isError %v, %v", listedToolName, isError, targeted)
	}

	// An unknown target is this tool's own usage refusal, keyed by the schema
	// property it was given.
	_, refusedDoc, isError := callToolResult(t, srv, listedToolName, map[string]any{"target": "not-a-stage"})
	items, _ := refusedDoc["refusals"].([]any)
	if !isError || refusedDoc["outcome"] != "refused" || len(items) != 1 {
		t.Fatalf("%s target not-a-stage: isError %v, %v", listedToolName, isError, refusedDoc)
	}
	if item, _ := items[0].(map[string]any); item["check"] != "usage" || item["key"] != "target" {
		t.Errorf("%s refuses an unknown target as %v, want a usage refusal of target", listedToolName, item)
	}

	// Dirt is a warning, never a refusal: the document names what the reset
	// would lose.
	writeValues(t, repo, "notes.md", "uncommitted\n")
	_, dirtyDoc, isError := callToolResult(t, srv, listedToolName, map[string]any{})
	paths, _ := dirtyDoc["dirty"].([]any)
	if isError || dirtyDoc["outcome"] != "done" || !slices.Contains(paths, any("notes.md")) {
		t.Errorf("a dirty tree: isError %v, %v", isError, dirtyDoc)
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

// TestMCPInitializeNamesWhatItServes drives initialize over the three states
// the served directory can be in. Every answer is held to the same properties:
// the form its state chooses, naming an absolute path, and no tool name in it
// but show-config — build occurring only in the phrase saying the KB can be
// built. The transcripts pin the serving state's bytes; the other two states
// are asserted here.
func TestMCPInitializeNamesWhatItServes(t *testing.T) {
	bound := map[string]bool{}
	for _, tool := range listTools(t) {
		bound[tool.Name] = true
	}
	// Both names the strings use and the invariant exempts must be tools the
	// table really holds, or the check below proves nothing.
	if !bound["show-config"] || !bound["build"] {
		t.Fatalf("the tool table holds neither build nor show-config: %v", bound)
	}
	request := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n"

	repo := stageRepo(t, resultsFixture)
	kbRoot := filepath.Join(repo, "kb-root")
	unbuilt := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(unbuilt, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepo(t, unbuilt)

	t.Cleanup(func() { rootCmd.SetIn(nil) })
	for _, tc := range []struct {
		name, wantPrefix string
		args             []string
		wantPath         string
		builds           int
	}{
		{name: "a repository holding kb-root/", args: []string{"mcp", "--kb-root", repo},
			wantPrefix: "kbase serves the knowledge base at ", wantPath: kbRoot},
		{name: "the kb-root/ itself", args: []string{"mcp", "--kb-root", kbRoot},
			wantPrefix: "kbase serves the knowledge base at ", wantPath: kbRoot},
		{name: "a repository whose kb-root/ is yet to be made", args: []string{"mcp", "--kb-root", unbuilt},
			wantPrefix: "kbase is registered for ", wantPath: unbuilt, builds: 1},
		{name: "no repository", args: []string{"mcp"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd.SetIn(strings.NewReader(request))
			out, code := kbase(t, t.TempDir(), tc.args...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, out)
			}
			var resp struct {
				Result struct {
					Instructions *string `json:"instructions"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
				t.Fatalf("%s: %v", out, err)
			}
			if tc.wantPath == "" {
				if resp.Result.Instructions != nil {
					t.Errorf("instructions %q where no repository holds the directory, want none", *resp.Result.Instructions)
				}
				return
			}
			if resp.Result.Instructions == nil {
				t.Fatalf("no instructions, want the form beginning %q", tc.wantPrefix+tc.wantPath)
			}
			text := *resp.Result.Instructions
			if !filepath.IsAbs(tc.wantPath) {
				t.Fatalf("the path this case expects, %q, is not absolute", tc.wantPath)
			}
			if !strings.HasPrefix(text, tc.wantPrefix+tc.wantPath) {
				t.Errorf("instructions %q, want the form beginning %q", text, tc.wantPrefix+tc.wantPath)
			}
			if strings.Contains(text, "`") {
				t.Errorf("instructions %q carry a backtick", text)
			}
			builds := 0
			for _, token := range instructionsTokens(text) {
				if token == "build" {
					builds++
					continue
				}
				if bound[token] && token != "show-config" {
					t.Errorf("instructions %q name the tool %q", text, token)
				}
			}
			if builds != tc.builds {
				t.Errorf("instructions %q say build %d times, want %d", text, builds, tc.builds)
			}
			if builds == 1 && strings.Count(text, "can build its knowledge base") != 1 {
				t.Errorf("instructions %q say build outside the phrase about building the KB", text)
			}
		})
	}
}

// instructionsTokens splits text on every character that is neither a letter,
// a digit nor a hyphen, so show-config is one token and no tool name is found
// inside another.
func instructionsTokens(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
}
