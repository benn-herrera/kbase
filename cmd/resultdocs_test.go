package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// kbase runs one invocation of the real root command from dir, as the binary
// would, and returns its stdout and exit code. The invoked subcommand's flags
// are put back to their defaults afterwards, as a new process would find them.
func kbase(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	t.Chdir(dir)
	stdout, _, err := executeRoot(t, args...)
	code := exitStatus(err, stdout, &bytes.Buffer{})
	if sub, _, ferr := rootCmd.Find(args); ferr == nil {
		if err := mcp.ResetFlags(sub); err != nil {
			t.Fatal(err)
		}
	}
	return stdout.String(), code
}

func writeValues(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestMaintenanceResultsParse runs every maintenance subcommand over a small
// KB and reads each result against its subcommand's documented keys.
func TestMaintenanceResultsParse(t *testing.T) {
	repo := maintenanceRepo(t)
	values := t.TempDir()
	run := func(want string, args ...string) parsedResult {
		t.Helper()
		out, code := kbase(t, repo, args...)
		d := checkResult(t, args[0], out)
		if d.Outcome != want || code != toolresult.ExitCode(want) {
			t.Errorf("%q: outcome %s, exit %d; want %s\n%s", args, d.Outcome, code, want, out)
		}
		return d
	}

	run("done", "verify")
	commitKB(t, repo)
	run("done", "update-format")
	run("refused", "update-format")
	commitKB(t, repo)
	run("unchanged", "update-format")
	run("done", "verify")

	writeRepo(t, repo, map[string]string{"kb-root/a.md": "[↑ KB](entry-point.md)\n\n# A\n\nA result is shown.\n\nAnother result is shown too.\n"})
	add := writeValues(t, values, "add.yaml", "entry:\n"+
		"- title: A Result\n  rigor: 0.5\n  rationale: Shown.\n  document: a.md\n  line: 5\n  snippet: A result is shown\n"+
		"- title: Another Result\n  rigor: 0.4\n  rationale: Shown too.\n  document: a.md\n  line: 7\n  snippet: Another result is shown too\n")
	first := run("done", "add-claim", "--values", add)
	ids := first.entryIDs()
	for _, e := range first.items("entries") {
		if e["outcome"] != toolresult.Done || e["operation"] != "add-claim" {
			t.Errorf("add entry %v, want a done add-claim", e)
		}
	}
	if len(ids) != 2 || first.Values["sheet"] == nil {
		t.Fatalf("add: %v", first.Values)
	}
	if again := run("unchanged", "add-claim", "--values", add); !slices.Equal(again.entryIDs(), ids) {
		t.Errorf("identical re-issued add: ids %v, want %v", again.entryIDs(), ids)
	}
	differing := run("refused", "add-claim", "--values", writeValues(t, values, "differing.yaml", "entry:\n"+
		"- title: A Result\n  rigor: 0.25\n  rationale: Shown.\n  document: a.md\n  line: 5\n  snippet: A result is shown\n"))
	if items := differing.items("refusals"); len(items) != 1 || items[0]["entry"] != 1 || items[0]["detail"] != "node exists: "+ids[0] {
		t.Errorf("differing re-issued add: refusals %v, want entry 1, node exists: %s", differing.Values["refusals"], ids[0])
	}
	dry := run("done", "edit-claim", "--dry-run", "--values", writeValues(t, values, "dry.yaml", "entry:\n- id: "+ids[0]+"\n  rigor: 0.25\n"))
	if dry.Values["dry-run"] != true || len(anyStrings(dry.Values["written"])) == 0 {
		t.Errorf("dry run: %v, want dry-run true and what it would write", dry.Values)
	}
	mixed := run("done", "write", "--values", writeValues(t, values, "mixed.yaml", "entry:\n"+
		"- operation: edit-claim\n  id: "+ids[0]+"\n  rigor: 0.25\n- operation: add-edge\n  source: "+ids[0]+"\n  target: "+ids[1]+"\n  relation: supports\n"))
	if e := mixed.items("entries"); len(e) != 2 || e[0]["operation"] != "edit-claim" || e[1]["operation"] != "add-edge" {
		t.Errorf("kbase write: entries %v, want edit-claim then add-edge", mixed.Values["entries"])
	}
	named := run("refused", "edit-claim", "--values", writeValues(t, values, "named.yaml", "entry:\n- operation: edit-claim\n  id: "+ids[0]+"\n  rigor: 0.5\n"))
	if items := named.items("refusals"); len(items) != 1 || items[0]["key"] != "operation" {
		t.Errorf("edit-claim over an entry naming its op: %v, want operation refused", items)
	}
	bad := writeValues(t, values, "bad.yaml", "entry:\n- id: "+ids[0]+"\n  rigour: 0.5\n")
	refusal := run("refused", "edit-claim", "--values", bad)
	if items := refusal.items("refusals"); len(items) != 1 || items[0]["key"] != "rigour" || items[0]["entry"] != 1 {
		t.Errorf("edit-claim refusal: %v", items)
	}
	absent := writeValues(t, values, "absent.yaml", "entry:\n- document: a.md\n  line: 1\n  snippet: "+ids[0]+"\n  id: "+ids[0]+"\n")
	run("refused", "add-citation", "--values", absent)

	for _, args := range [][]string{
		{"find"}, {"show", ids[0]}, {"stats"}, {"edges"}, {"edges", "--relation", kb.RelationFounds}, {"edges", "--id", ids[0], "--direction", "out"},
		{"citations-of", ids[0]}, {"trace", ids[0], "--direction", "in"}, {"trace", ids[0], "--direction", "out", "--sort", "contribution", "--limit", "-1"},
	} {
		run("done", args...)
	}
	run("refused", "show", "clm-zzzzzz")
	if items := run("refused", "trace", "clm-zzzzzz", "--direction", "in").items("refusals"); len(items) != 1 || items[0]["detail"] != "no node clm-zzzzzz" {
		t.Errorf("trace clm-zzzzzz refusal: %v, want no node", items)
	}
	if items := run("refused", "citations-of", "clm-zzzzzz").items("refusals"); len(items) != 1 || items[0]["detail"] != "no node clm-zzzzzz" {
		t.Errorf("citations-of clm-zzzzzz refusal: %v, want no node", items)
	}
	run("refused", "find", "--max", "half")
	if items := run("refused", "find", "--pending", "--max", "0.5").items("refusals"); len(items) != 1 || items[0]["check"] != toolresult.CheckUsage ||
		items[0]["key"] != "--pending" {
		t.Errorf("find --pending --max 0.5 refusal: %v, want one usage item naming --pending", items)
	}
	unknown := run("refused", "edges", "--relation", "nope", "--relation", "nope")
	if items := unknown.items("refusals"); len(items) != 1 || items[0]["check"] != toolresult.CheckUsage || items[0]["key"] != "--relation" ||
		!slices.Equal(anyStrings(items[0]["allowed"]), kb.Relations) || !strings.Contains(items[0]["detail"].(string), "nope") {
		t.Errorf("edges --relation nope --relation nope refusal: %v, want one item", items)
	}
	if items := run("refused", "edges", "--relation", "").items("refusals"); len(items) != 1 || items[0]["key"] != "--relation" {
		t.Errorf("edges --relation \"\" refusal: %v", items)
	}
	claims := run("done", "find", "--kind", "claim", "--sort", "id")
	if claims.Values["count"] != 2 || len(claims.Values["results"].([]any)) != 2 {
		t.Errorf("find --kind claim = %v, want both claims", claims.Values)
	}
	for _, limit := range []string{"1", "-1"} {
		got := run("done", "find", "--kind", "claim", "--sort", "id", "--limit", limit)
		want := claims.Values["results"].([]any)[:1]
		if limit == "-1" {
			want = claims.Values["results"].([]any)[1:]
		}
		if got.Values["count"] != 1 || !reflect.DeepEqual(got.Values["results"], want) {
			t.Errorf("find --kind claim --sort id --limit %s = %v, want %v", limit, got.Values, want)
		}
	}
	for _, args := range [][]string{{"find", "--limit", "1"}, {"find", "--sort", "id", "--limit", "0"}} {
		if items := run("refused", args...).items("refusals"); len(items) != 1 || items[0]["key"] != "--limit" {
			t.Errorf("%q refusal: %v, want one item naming --limit", args, items)
		}
	}
	// stats' help names the find call giving the full list behind each ranked
	// section, as each surface renders it; run as rendered, each call answers
	// a list whose head is the section.
	stats := run("done", "stats").Values["results"].(map[string]any)
	resultIDs := func(rows any) []any {
		var out []any
		for _, r := range rows.([]any) {
			out = append(out, r.(map[string]any)["id"])
		}
		return out
	}
	sectionLimits := map[string]int{"highest-leverage": *query.HighestLeverage().Limit, "weakest": *query.Weakest().Limit}
	requireHead := func(surface, section string, full []any) {
		t.Helper()
		if got := resultIDs(stats[section]); !slices.Equal(got, full[:min(len(full), sectionLimits[section])]) {
			t.Errorf("%s: stats %s = %v, want the head of its find call's answer %v", surface, section, got, full)
		}
	}
	cliHelp, _ := kbase(t, repo, "stats", "--help")
	cliCalls := regexp.MustCompile("`kbase (find(?: --[a-z-]+ [^ ,`\n]+)+)` for ([a-z-]+)").FindAllStringSubmatch(cliHelp, -1)
	srv := servedServer(rootCmd, mcp.LocatedAt(realPath(t, repo)), &bytes.Buffer{}, log.Discard())
	mcpCalls := regexp.MustCompile("`query (\\{\"operation\":\"find\",\"arguments\":\\{[^{}]*\\}\\})` for ([a-z-]+)").
		FindAllStringSubmatch(readResource(t, srv, mcp.HelpURIPrefix+"stats"), -1)
	if len(cliCalls) != len(sectionLimits) || len(mcpCalls) != len(sectionLimits) {
		t.Fatalf("stats' help renders %q at the command line and %q over MCP, want one find call per ranked section", cliCalls, mcpCalls)
	}
	for _, c := range cliCalls {
		requireHead("cli", c[2], resultIDs(run("done", strings.Fields(c[1])...).Values["results"]))
	}
	for _, c := range mcpCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(c[1]), &args); err != nil {
			t.Fatal(err)
		}
		doc, isError := callTool(t, srv, queryFamily, args)
		if isError {
			t.Fatalf("mcp: %s answers %v", c[1], doc)
		}
		requireHead("mcp", c[2], resultIDs(doc["results"]))
	}

	outside := t.TempDir()
	out, code := kbase(t, outside, "update-format")
	if d := checkResult(t, "update-format", out); d.Outcome != "refused" || code != 1 {
		t.Errorf("update-format outside a worktree: %s", out)
	}
}

// buildRepo is a fresh repository holding a one-section paper, git's
// identity and configuration fixed; it skips where pandoc is unusable.
func buildRepo(t *testing.T) (repo, state string) {
	t.Helper()
	return buildRepoWith(t, "\\documentclass{article}\n\\newtheorem{lemma}{Lemma}\n\\title{A Paper}\n\\begin{document}\n\\maketitle\n"+
		"\\section{Results}\n\\begin{lemma}\\label{lem:a}\nEvery widget is a gadget.\n\\end{lemma}\n\\end{document}\n")
}

// buildRepoWith is buildRepo holding paper as paper.tex.
func buildRepoWith(t *testing.T, paper string) (repo, state string) {
	t.Helper()
	if _, err := pandoc.Preflight(context.Background()); err != nil {
		t.Skipf("pandoc is not usable here: %v", err)
	}
	base := t.TempDir()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@invalid", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@invalid",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": filepath.Join(base, "gitconfig"), "XDG_STATE_HOME": filepath.Join(base, "xdg"),
	} {
		t.Setenv(k, v)
	}
	repo, state = filepath.Join(base, "repo"), filepath.Join(base, "state")
	for _, d := range []string{".git/objects", ".git/refs/heads"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeValues(t, filepath.Join(repo, ".git"), "HEAD", "ref: refs/heads/main\n")
	writeValues(t, repo, "paper.tex", paper)
	return repo, state
}

// TestBuildResultsParse runs build, status and cancel and reads each result
// against its subcommand's documented keys.
func TestBuildResultsParse(t *testing.T) {
	repo, state := buildRepo(t)
	run := func(want string, args ...string) parsedResult {
		t.Helper()
		out, code := kbase(t, repo, args...)
		d := checkResult(t, args[0], out)
		if d.Outcome != want || code != toolresult.ExitCode(want) {
			t.Errorf("%q: outcome %s, exit %d; want %s\n%s", args, d.Outcome, code, want, out)
		}
		return d
	}
	run("refused", "build", "paper.tex", "--through", "phase-5", "--state-dir", state)
	if before := run("done", "status", "--state-dir", state); before.Values["state"] != "none" {
		t.Errorf("status before any build = %v", before.Values)
	}
	bounded := run("bounded", "build", "paper.tex", "--no-inference", "--through", "spine-seed", "--state-dir", state)
	if stages, _ := bounded.Values["stages"].([]any); len(stages) != 3 {
		t.Errorf("stages = %v, want start through spine-seed", bounded.Values["stages"])
	}
	if _, ok := bounded.Values["resume"]; ok {
		t.Errorf("a bounded build carries resume: %v", bounded.Values)
	}
	if bounded.Values["through"] != "spine-seed" {
		t.Errorf("through = %v, want spine-seed", bounded.Values["through"])
	}
	status := run("done", "status", "--state-dir", state)
	if status.Values["state"] != "bounded" {
		t.Errorf("status = %v", status.Values)
	}
	run("refused", "cancel", "--state-dir", state)
	done := run("done", "build", "--resume", "--no-inference", "--state-dir", state)
	if _, ok := done.Values["resume"]; ok {
		t.Errorf("a finished build carries resume: %v", done.Values)
	}
	stages, ok := done.Values["stages"].([]any)
	if !ok {
		t.Fatalf("the finished build carries no stages: %v", done.Values)
	}
	for _, s := range stages {
		report, _ := s.(map[string]any)["report"].(string)
		if b, err := os.ReadFile(report); err != nil || !bytes.HasPrefix(b, []byte("stage: ")) {
			t.Errorf("stage report %s: %v", report, err)
		}
	}
}
