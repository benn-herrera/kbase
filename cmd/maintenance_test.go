package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/log"
)

func maintenanceRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": "<!-- kb-frontmatter\nkind: entry-point\n-->\n\n# KB\n\n- [a](a.md)\n",
		"kb-root/a.md":           "[↑ KB](entry-point.md)\n\n<!-- kb-frontmatter\nkind: leaf\nno-claim: \"prose\"\n-->\n\n# A\n",
	}
	for rel, text := range files {
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func runMaintenance(t *testing.T, run func(maintenanceOptions) (int, error), dir string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code, err := run(maintenanceOptions{WorkDir: dir, Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	if err != nil {
		t.Fatal(err)
	}
	return code, out.String()
}

// TestVerifyReportsALegacyAxiomDeclaration: a KB that declares its axioms in
// axioms.md and still carries a declaration in AGENTS.md reports each leftover
// declaration under findings, and verify stays done.
func TestVerifyReportsALegacyAxiomDeclaration(t *testing.T) {
	repo := maintenanceRepo(t)
	root := filepath.Join(repo, "kb-root")
	extra := map[string]string{
		"axioms.md": "# Axioms\n\n### Axiom S1: The design rule\n\nEvery edge carries a bearing.\n",
		"AGENTS.md": "# Notes\n\n- Axiom 1: **Every node carries a bearing.**\n",
	}
	for rel, text := range extra {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code, out := runMaintenance(t, runRefresh, root); code != 0 {
		t.Fatalf("refresh: exit %d\n%s", code, out)
	}
	code, out := runMaintenance(t, runVerify, repo)
	if code != 0 || !strings.Contains(out, `outcome: "done"`) {
		t.Fatalf("verify: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, `check: "axiom-source"`) || !strings.Contains(out, `path: "AGENTS.md"`) ||
		!strings.Contains(out, `detail: "Axiom 1"`) {
		t.Errorf("verify did not report the leftover declaration:\n%s", out)
	}
}

// TestVerifyRefusesAMalformedBullet: a hand-authored bullet the reader refuses
// is a refused verify, one item per fault under the metadata class at the
// bullet's file and line, naming the field, the values it allows and the
// remedy — not a failure.
func TestVerifyRefusesAMalformedBullet(t *testing.T) {
	repo := maintenanceRepo(t)
	values := t.TempDir()
	run := func(want string, args ...string) resultDoc {
		t.Helper()
		out, code := kbase(t, repo, args...)
		d := checkDocument(t, args[0], out)
		if d.Outcome != want || code != exitCodes[want] {
			t.Fatalf("%q: outcome %s, exit %d; want %s\n%s", args, d.Outcome, code, want, out)
		}
		return d
	}
	run("done", "refresh")
	claim := anyStrings(run("done", "insert-claim-entry", "--create", "--values",
		writeValues(t, values, "claim.yaml", "entry:\n- register: claim-rigor.md\n  title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")).Values["ids"])[0]
	analysis := anyStrings(run("done", "insert-analysis-entry", "--values",
		writeValues(t, values, "analysis.yaml", "entry:\n- register: claim-rigor.md\n  title: An Analysis\n  rigor: 0.9\n  rationale: Lifts the result.\n")).Values["ids"])[0]
	run("done", "set-frontmatter", "--values", writeValues(t, values, "declare.yaml", "entry:\n- document: a.md\n  kind: leaf\n"+
		"  claims: ["+claim+"]\n  analysis-node:\n  - ana-id: "+analysis+"\n    lifts:\n    - id: "+claim+"\n      bearing: 0.5\n"))

	leaf := filepath.Join(repo, "kb-root", "a.md")
	text, err := os.ReadFile(leaf)
	if err != nil {
		t.Fatal(err)
	}
	pair := "- " + claim + ": 0.5"
	faulted := strings.Replace(string(text), pair, "- "+claim+": 2", 1)
	if faulted == string(text) {
		t.Fatalf("no pair bearing in a.md:\n%s", text)
	}
	if err := os.WriteFile(leaf, []byte(faulted), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := kbase(t, repo, "verify")
	if code != exitCodes["refused"] {
		t.Fatalf("verify: exit %d, want refused\n%s", code, out)
	}
	d := checkDocument(t, "verify", out)
	items := d.items("refusals")
	if d.Outcome != "refused" || len(items) != 1 {
		t.Fatalf("outcome %q with %d refusals, want refused with one:\n%s", d.Outcome, len(items), out)
	}
	it := items[0]
	wantLine := 1 + strings.Count(faulted[:strings.Index(faulted, "- "+claim+": 2")], "\n")
	if it["check"] != "metadata" || it["path"] != "a.md" || it["line"] != wantLine || it["key"] != "lifts" {
		t.Errorf("item = %v, want check metadata at a.md:%d key lifts", it, wantLine)
	}
	if allowed, _ := it["allowed"].([]any); len(allowed) != 1 || allowed[0] != "[0, 1] or *pending*" {
		t.Errorf("allowed = %v, want the bearing domain alone", it["allowed"])
	}
	if it["remedy"] != "write a number in [0, 1] or *pending*" {
		t.Errorf("remedy = %v, want the bearing remedy", it["remedy"])
	}
}

func TestRefreshAndVerifyVerbs(t *testing.T) {
	repo := maintenanceRepo(t)
	sub := filepath.Join(repo, "kb-root")

	if code, out := runMaintenance(t, runVerify, repo); code != 1 || !strings.Contains(out, `outcome: "refused"`) || !strings.Contains(out, "claims.yaml is missing") ||
		!strings.Contains(out, "metadata format is 0.9.0") {
		t.Errorf("verify before refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runRefresh, sub); code != 0 || !strings.Contains(out, `outcome: "done"`) || !strings.Contains(out, `- ".index/claims.yaml"`) {
		t.Errorf("first refresh: exit %d\n%s", code, out)
	}
	if b, _ := os.ReadFile(filepath.Join(sub, "entry-point.md")); !strings.HasPrefix(string(b), "---\nkind: entry-point\n") || !strings.Contains(string(b), "kb-format: \"2.0.0\"\n---\n") {
		t.Errorf("the first refresh left the entry point unstamped:\n%s", b)
	}
	if code, out := runMaintenance(t, runRefresh, repo); code != 0 || !strings.Contains(out, `outcome: "unchanged"`) {
		t.Errorf("second refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runVerify, repo); code != 0 || !strings.Contains(out, `outcome: "done"`) {
		t.Errorf("verify after refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runVerify, t.TempDir()); code != 1 || !strings.Contains(out, "not inside a git worktree") {
		t.Errorf("verify outside a worktree: exit %d\n%s", code, out)
	}
}
