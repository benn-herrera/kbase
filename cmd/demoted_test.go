package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// ringPaper is a paper whose two lemmas' proofs each cite the other: the
// build's containment ring cuts both edges.
const ringPaper = "\\documentclass{article}\n\\newtheorem{lemma}{Lemma}\n\\title{A Ring}\n\\begin{document}\n\\maketitle\n" +
	"\\section{Results}\n" +
	"\\begin{lemma}\\label{lem:a}\nEvery widget is a gadget.\n\\end{lemma}\n\\begin{proof}\nBy Lemma~\\ref{lem:b}.\n\\end{proof}\n" +
	"\\begin{lemma}\\label{lem:b}\nEvery gadget is a widget.\n\\end{lemma}\n\\begin{proof}\nBy Lemma~\\ref{lem:a}.\n\\end{proof}\n" +
	"\\end{document}\n"

// demotedRows is every demoted row of the KB's depends-on index.
func demotedRows(t *testing.T, repo string) []kb.EdgeRow {
	t.Helper()
	_, rows, problems, err := kb.ReadIndex[kb.EdgeRow](kb.OnDisk(filepath.Join(repo, kb.KBDir)), nil)
	if err != nil || problems != nil {
		t.Fatal(err, problems)
	}
	var out []kb.EdgeRow
	for _, row := range rows {
		if row.Relation == kb.RelationDemoted {
			out = append(out, row)
		}
	}
	return out
}

// TestDemotedRing builds a paper with a ring without inference and follows
// its cut edges through verify, stats and resolve-demoted.
func TestDemotedRing(t *testing.T) {
	repo, state := buildRepoWith(t, ringPaper)
	run := func(want string, args ...string) resultDoc {
		t.Helper()
		out, code := kbase(t, repo, args...)
		d := checkDocument(t, args[0], out)
		if d.Outcome != want || code != exitCodes[want] {
			t.Fatalf("%q: outcome %s, exit %d; want %s\n%s", args, d.Outcome, code, want, out)
		}
		return d
	}
	run("done", "build", "paper.tex", "--no-inference", "--state-dir", state)

	rows := demotedRows(t, repo)
	if len(rows) != 2 {
		t.Fatalf("demoted rows %v, want the ring's two edges", rows)
	}
	for _, r := range rows {
		if r.Origin == nil || *r.Origin != kb.OriginCited {
			t.Errorf("demoted row %+v, want origin cited: the text marks both references", r)
		}
	}
	a, b := rows[0].Source, rows[0].Target

	verified := run("done", "verify")
	findings := verified.items("findings")
	if len(findings) != 2 || findings[0]["check"] != kb.RelationDemoted || !slices.ContainsFunc(findings, func(f map[string]any) bool {
		return strings.HasPrefix(f["detail"].(string), a+" → "+b+", origin cited")
	}) {
		t.Errorf("verify findings %v, want both demoted edges", findings)
	}
	if stats := run("done", "stats").Values["results"].(map[string]any); stats["demoted_edges"] != 2 {
		t.Errorf("stats demoted_edges = %v, want 2", stats["demoted_edges"])
	}

	register := filepath.Join(repo, kb.KBDir, filepath.FromSlash(findings[0]["path"].(string)))
	text, err := os.ReadFile(register)
	if err != nil {
		t.Fatal(err)
	}
	unmarked := strings.Replace(string(text), " (origin cited)", "", 1)
	if unmarked == string(text) {
		t.Fatalf("no origin annotation in %s:\n%s", register, text)
	}
	writeValues(t, filepath.Dir(register), filepath.Base(register), unmarked)
	run("done", "refresh")
	if refused := run("refused", "verify"); !strings.Contains(refused.items("refusals")[0]["detail"].(string), "carries origin <nil>") {
		t.Errorf("a demoted row with no origin: %v", refused.items("refusals"))
	}
	writeValues(t, filepath.Dir(register), filepath.Base(register), string(text))
	run("done", "refresh")

	values := t.TempDir()
	resolve := func(name, id, target, action string) string {
		return writeValues(t, values, name, "entry:\n- id: "+id+"\n  target: "+target+"\n  action: "+action+"\n")
	}
	restored := run("done", "resolve-demoted", "--values", resolve("restore.yaml", a, b, "restore"))
	if r := restored.items("resolved"); len(r) != 1 || r[0]["action"] != "restored" {
		t.Errorf("restore resolved %v", r)
	}
	run("unchanged", "resolve-demoted", "--values", resolve("restore.yaml", a, b, "restore"))
	cycle := run("refused", "resolve-demoted", "--values", resolve("cycle.yaml", b, a, "restore")).items("refusals")
	if len(cycle) != 1 || cycle[0]["check"] != "dependency-cycle" || !strings.Contains(cycle[0]["detail"].(string), a+" → "+b+" → "+a) {
		t.Errorf("a restore closing the ring: %v", cycle)
	}
	removed := run("done", "resolve-demoted", "--values", resolve("remove.yaml", b, a, "remove"))
	if r := removed.items("resolved"); len(r) != 1 || r[0]["action"] != "removed" {
		t.Errorf("remove resolved %v", r)
	}
	run("unchanged", "resolve-demoted", "--values", resolve("remove.yaml", b, a, "remove"))
	if f := run("done", "verify").items("findings"); len(f) != 0 {
		t.Errorf("findings after both resolutions %v, want none", f)
	}
	if stats := run("done", "stats").Values["results"].(map[string]any); stats["demoted_edges"] != 0 {
		t.Errorf("stats demoted_edges = %v, want 0", stats["demoted_edges"])
	}
}
