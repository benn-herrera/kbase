package main

import (
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	toolresult "kbase/internal/result"
)

// ringPaper is a paper whose two lemmas' proofs each cite the other: the
// build's cycle breaking cuts one of the ring's two edges.
const ringPaper = "\\documentclass{article}\n\\newtheorem{lemma}{Lemma}\n\\title{A Ring}\n\\begin{document}\n\\maketitle\n" +
	"\\section{Results}\n" +
	"\\begin{lemma}\\label{lem:a}\nEvery widget is a gadget.\n\\end{lemma}\n\\begin{proof}\nBy Lemma~\\ref{lem:b}.\n\\end{proof}\n" +
	"\\begin{lemma}\\label{lem:b}\nEvery gadget is a widget.\n\\end{lemma}\n\\begin{proof}\nBy Lemma~\\ref{lem:a}.\n\\end{proof}\n" +
	"\\end{document}\n"

// graphRecords is the KB's records, as the loader reads them.
func graphRecords(t *testing.T, repo string) kb.Graph {
	t.Helper()
	src, err := kbload.Open(filepath.Join(repo, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	g, err := kb.LoadGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestDemotedRing builds a paper with a ring without inference and follows
// its cut edge, which the analysis split moved onto the analysis of the lemma
// it entered, through verify, stats and resolve-demoted: restoring it would
// close the ring again, through both lemmas' analyses, and removing it
// resolves it.
func TestDemotedRing(t *testing.T) {
	repo, state := buildRepoWith(t, ringPaper)
	run := func(want string, args ...string) resultDoc {
		t.Helper()
		out, code := kbase(t, repo, args...)
		d := checkDocument(t, args[0], out)
		if d.Outcome != want || code != toolresult.ExitCode(want) {
			t.Fatalf("%q: outcome %s, exit %d; want %s\n%s", args, d.Outcome, code, want, out)
		}
		return d
	}
	run("done", "build", "paper.tex", "--no-inference", "--state-dir", state)

	g := graphRecords(t, repo)
	cuts := g.Demoted
	if len(cuts) != 1 || cuts[0].ReplacedRelation != kb.RelationSupports {
		t.Fatalf("demoted records %+v, want one supports edge cut from the ring", cuts)
	}
	a, b := cuts[0].Source, cuts[0].Target

	verified := run("done", "verify")
	findings := verified.items("findings")
	if len(findings) != 1 || findings[0]["check"] != kb.RelationDemoted ||
		!strings.HasPrefix(findings[0]["detail"].(string), a+" → "+b+", demoted from "+kb.RelationSupports) {
		t.Errorf("verify findings %v, want the demoted edge %s → %s", findings, a, b)
	}
	if stats := run("done", "stats").Values["results"].(map[string]any); stats["demoted_edges"] != 1 {
		t.Errorf("stats demoted_edges = %v, want 1", stats["demoted_edges"])
	}

	// resolve-demoted names a cut by the node it went into, id, and the node it
	// came from, target.
	values := t.TempDir()
	resolve := func(name, action string) string {
		return writeValues(t, values, name, "entry:\n- id: "+b+"\n  target: "+a+"\n  action: "+action+"\n")
	}
	lifted := map[string]string{}
	for _, e := range g.Scorable {
		if e.Relation == kb.RelationLifts {
			lifted[e.Source] = e.Target
		}
	}
	analysisOf := map[string]string{}
	for ana, claim := range lifted {
		analysisOf[claim] = ana
	}
	ring := a + " → " + analysisOf[a] + " → " + lifted[b] + " → " + b + " → " + a
	cycle := run("refused", "resolve-demoted", "--values", resolve("restore.yaml", "restore")).items("refusals")
	if len(lifted) != 2 || len(cycle) != 1 || cycle[0]["check"] != "dependency-cycle" || !strings.Contains(cycle[0]["detail"].(string), ring) {
		t.Errorf("a restore closing the ring: %v, want the cycle %s", cycle, ring)
	}
	removed := run("done", "resolve-demoted", "--values", resolve("remove.yaml", "remove"))
	if r := removed.items("resolved"); len(r) != 1 || r[0]["action"] != "removed" {
		t.Errorf("remove resolved %v", r)
	}
	run("unchanged", "resolve-demoted", "--values", resolve("remove.yaml", "remove"))
	if f := run("done", "verify").items("findings"); len(f) != 0 {
		t.Errorf("findings after both resolutions %v, want none", f)
	}
	if stats := run("done", "stats").Values["results"].(map[string]any); stats["demoted_edges"] != 0 {
		t.Errorf("stats demoted_edges = %v, want 0", stats["demoted_edges"])
	}
}
