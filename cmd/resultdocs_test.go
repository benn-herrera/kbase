package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/latex/pandoc"
	"kbase/internal/mcp"
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
	run := func(want string, args ...string) resultDoc {
		t.Helper()
		out, code := kbase(t, repo, args...)
		d := checkDocument(t, args[0], out)
		if d.Outcome != want || code != toolresult.ExitCode(want) {
			t.Errorf("%q: outcome %s, exit %d; want %s\n%s", args, d.Outcome, code, want, out)
		}
		return d
	}

	run("done", "verify")
	run("done", "refresh")
	run("unchanged", "refresh")
	run("done", "verify")
	run("unchanged", "render-claim-graph")

	insert := writeValues(t, values, "insert.yaml", "entry:\n- title: A Result\n  rigor: 0.5\n  rationale: Shown.\n"+
		"- title: Another Result\n  rigor: 0.4\n  rationale: Shown too.\n")
	first := run("done", "insert-claim-entry", "--values", insert)
	ids := anyStrings(first.Values["ids"])
	if len(ids) != 2 || !slices.Equal(anyStrings(first.Values["minted"]), ids) || first.Values["refreshed"] == nil {
		t.Fatalf("insert: %v", first.Values)
	}
	differ := writeValues(t, values, "differ.yaml", "entry:\n- title: A Result\n  rigor: 0.9\n  rationale: Shown.\n")
	again := run("unchanged", "insert-claim-entry", "--no-refresh", "--values", differ)
	adopted, _ := again.Values["adopted"].([]any)
	if a, _ := adopted[0].(map[string]any); len(adopted) != 1 || a["entry"] != 1 || a["id"] != ids[0] || !slices.Equal(anyStrings(a["differs"]), []string{"rigor"}) {
		t.Errorf("re-issued insert: adopted %v", again.Values["adopted"])
	}
	if refreshed, ok := again.Values["refreshed"]; !ok || refreshed != nil {
		t.Errorf("refreshed under --no-refresh = %v, want null", again.Values["refreshed"])
	}
	bad := writeValues(t, values, "bad.yaml", "entry:\n- id: "+ids[0]+"\n  rigour: 0.5\n")
	refusal := run("refused", "set-rigor", "--values", bad)
	if items := refusal.items("refusals"); len(items) != 2 || items[0]["key"] != "rigour" || items[0]["entry"] != 1 {
		t.Errorf("set-rigor refusal: %v", items)
	}
	citation := writeValues(t, values, "cite.yaml", "entry:\n- citing-document: a.md\n  cited-document: nowhere.md\n  anchor: a\n  excerpt: words\n")
	run("refused", "render-citation", "--values", citation)

	for _, args := range [][]string{
		{"deps", ids[0]}, {"deps", ids[0], "-i"}, {"gated-on", ids[0]}, {"cited-by", ids[0]}, {"find", ""},
		{"referenced-by", ids[0]}, {"strength-below", "--max-strength", "0.5"}, {"subtree", ""}, {"weak-points"},
		{"show", ids[0]}, {"stats"}, {"edge-census"}, {"edges"}, {"edges", "--relation", kb.RelationFounds},
	} {
		run("done", args...)
	}
	run("refused", "show", "clm-zzzzzz")
	run("refused", "strength-below", "--max-strength", "half")
	unknown := run("refused", "edges", "--relation", "nope", "--relation", "nope")
	if items := unknown.items("refusals"); len(items) != 1 || items[0]["check"] != checkUsage || items[0]["key"] != "--relation" ||
		!slices.Equal(anyStrings(items[0]["allowed"]), kb.Relations) || !strings.Contains(items[0]["detail"].(string), "nope") {
		t.Errorf("edges --relation nope --relation nope refusal: %v, want one item", items)
	}
	if items := run("refused", "edges", "--relation", "").items("refusals"); len(items) != 1 || items[0]["key"] != "--relation" {
		t.Errorf("edges --relation \"\" refusal: %v", items)
	}
	page := run("done", "find", "", "--limit", "1", "--offset", "1")
	if page.Values["count"] != 2 || page.Values["offset"] != 1 || page.Values["truncated"] != false || len(page.Values["results"].([]any)) != 1 {
		t.Errorf("find --limit 1 --offset 1 = %v, want the second of two, nothing past it", page.Values)
	}
	if head := run("done", "find", "", "--limit", "1"); head.Values["truncated"] != true {
		t.Errorf("find --limit 1 = %v, want truncated", head.Values)
	}
	run("refused", "find", "", "--limit", "-1")

	outside := t.TempDir()
	out, code := kbase(t, outside, "refresh")
	if d := checkDocument(t, "refresh", out); d.Outcome != "refused" || code != 1 {
		t.Errorf("refresh outside a worktree: %s", out)
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
	run := func(want string, args ...string) resultDoc {
		t.Helper()
		out, code := kbase(t, repo, args...)
		d := checkDocument(t, args[0], out)
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
	done := run("done", "build", "paper.tex", "--no-inference", "--state-dir", state)
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
