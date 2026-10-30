package claimgraph

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/records"
)

// marked is a line's statement marker for id, with the space before it.
func marked(id string) string { return " " + kb.StatementMarker(id) }

// treeFiles is every file under dir, by slash path relative to it.
func treeFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// buildRecordFiles is the bytes of every build record under repo, "" where one
// does not stand.
func buildRecordFiles(t *testing.T, repo string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range buildrecords.RecordFiles {
		b, err := os.ReadFile(filepath.Join(repo, f))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		out[f] = string(b)
	}
	return out
}

// leafLines is the leaf at rel under root, line by line.
func leafLines(t *testing.T, root, rel string) []string {
	t.Helper()
	return kb.SplitLines(treeFiles(t, root)[rel])
}

// splitFinding is the stage's one finding's fields, by key.
func splitFinding(t *testing.T, report Report) map[string]any {
	t.Helper()
	for _, f := range report.Findings {
		if f.Check == "analyses-split" {
			out := map[string]any{}
			for _, fd := range f.Fields {
				out[fd.Key] = fd.Value
			}
			return out
		}
	}
	t.Fatalf("no analyses-split finding among %+v", report.Findings)
	return nil
}

// TestSplitHostsOnTheBlocksBlankLine: theorem D's block holds two items below
// its display line, so D splits. Its analysis is titled as D, rigor pending and
// not manually scored, marked on the block's first blank line and lifting D at
// pending; D's marker stays, and no build record moves.
func TestSplitHostsOnTheBlocksBlankLine(t *testing.T) {
	const a, b, c, d = "clm-aaaaaa", "clm-bbbbbb", "clm-cccccc", "clm-dddddd"
	root, repo, opts := seedTheoremLeaf(t,
		map[string]string{a: "Alpha holds here", b: "Beta holds here", c: "Gamma holds here", d: "Theorem 1"},
		"Alpha holds here."+marked(a)+"\n\nBeta holds here."+marked(b)+"\n\nGamma holds here."+marked(c)+"\n\n"+
			"> **Theorem**\n>\n> **Theorem 1.** Delta holds here."+marked(d)+"\n>\n> 1.  First by convexity.\n>\n> 2.  Second by convexity.\n")
	leafBefore := leafLines(t, root, cullLeaf)
	recordsBefore := buildRecordFiles(t, repo)

	report, err := Split(opts)
	requireStagePassed(t, report, err)

	g := loadRecords(t, root)
	if len(g.Analyses) != 1 {
		t.Fatalf("analyses %+v, want one", g.Analyses)
	}
	ana := g.Analyses[0]
	if ana.Title != "Theorem 1" || ana.Rigor != nil || ana.ManuallyScored {
		t.Errorf("analysis %+v, want D's title, rigor pending, not manually scored", ana)
	}
	leaf := leafLines(t, root, cullLeaf)
	blank := slices.Index(leafBefore, ">")
	display := slices.IndexFunc(leafBefore, func(l string) bool { return strings.HasSuffix(l, marked(d)) })
	if leaf[blank] != ">"+marked(ana.ID) || leaf[display] != leafBefore[display] {
		t.Errorf("blank line %q and display line %q, want the analysis on the first and D's line unchanged", leaf[blank], leaf[display])
	}
	if lift := (kb.ScorableEdge{Relation: kb.RelationLifts, Source: ana.ID, Target: d, Bearing: kb.Bearing{Pending: true}}); !slices.Contains(g.Scorable, lift) {
		t.Errorf("edges %+v, want %+v", g.Scorable, lift)
	}
	if after := buildRecordFiles(t, repo); !maps.Equal(after, recordsBefore) {
		t.Error("the stage moved a build record")
	}
}

// TestSplitProseClaim: each prose claim's marker ends its paragraph's first
// line, where the build places it. P1's one-line paragraph holds a lead-in, the
// sentence its title is drawn from and one more, so its marker moves to just
// after that sentence and the analysis's marker ends the line; P2's paragraph
// is that sentence alone and stays as it stands. P3's sentence ends its second
// line, before more text, and P4's ends its first, before a second line: each
// claim's marker moves to just after its sentence, and its analysis's marker
// ends the paragraph's last line.
func TestSplitProseClaim(t *testing.T) {
	const p1, p2, p3, p4, d = "clm-aaaaaa", "clm-bbbbbb", "clm-cccccc", "clm-eeeeee", "clm-dddddd"
	const lead, opening, more = "Recall the setup.", "The first opening sentence holds here.", "More follows from it."
	const alone = "The second opening sentence holds alone."
	const third, fourth = "The third opening sentence holds here.", "The fourth opening sentence holds here."
	root, _, opts := seedTheoremLeaf(t,
		map[string]string{p1: opening, p2: alone, p3: third, p4: fourth, d: "Theorem 1"},
		lead+" "+opening+" "+more+marked(p1)+"\n\n"+alone+marked(p2)+"\n\n"+
			"Lead in."+marked(p3)+"\n"+third+" More follows.\nAnd a third line.\n\n"+
			fourth+marked(p4)+"\nMore follows on the next line.\n\n"+
			"> **Theorem**\n>\n> **Theorem 1.** Delta holds here."+marked(d)+"\n")

	report, err := Split(opts)
	requireStagePassed(t, report, err)

	g := loadRecords(t, root)
	if len(g.Analyses) != 3 {
		t.Fatalf("analyses %+v, want P1's, P3's and P4's", g.Analyses)
	}
	lifting := map[string]string{}
	for _, e := range g.Scorable {
		if e.Relation == kb.RelationLifts {
			lifting[e.Target] = e.Source
		}
	}
	leaf := leafLines(t, root, cullLeaf)
	for _, tc := range []struct {
		claim string
		want  func(ana string) []string
	}{
		{p1, func(ana string) []string {
			return []string{lead + " " + opening + marked(p1) + " " + more + marked(ana)}
		}},
		{p3, func(ana string) []string {
			return []string{"Lead in.", third + marked(p3) + " More follows.", "And a third line." + marked(ana)}
		}},
		{p4, func(ana string) []string {
			return []string{fourth + marked(p4), "More follows on the next line." + marked(ana)}
		}},
	} {
		ana, ok := lifting[tc.claim]
		if !ok {
			t.Errorf("no analysis lifts %s: %+v", tc.claim, g.Scorable)
			continue
		}
		want := tc.want(ana)
		if at := slices.Index(leaf, want[0]); at < 0 || !slices.Equal(leaf[at:min(at+len(want), len(leaf))], want) {
			t.Errorf("leaf %q, want %s's lines %q", leaf, tc.claim, want)
		}
	}
	if want := alone + marked(p2); !slices.Contains(leaf, want) {
		t.Errorf("leaf %q, want P2's line unchanged: %q", leaf, want)
	}
}

// TestSplitHostsAtAProof: over two volumes, D is display-only with a proof
// below it, D3 has an item below its display line and a proof, and leaf 2's
// proof names D and D2, display-only with no proof of its own. D's analysis
// stands at its own proof, the first in reading order; D3's in its own block;
// D2's at leaf 2's proof, its title's link rebased to volume 2's register
// page. D3's proof hosts nothing.
func TestSplitHostsAtAProof(t *testing.T) {
	const d, d2, d3 = "clm-dddddd", "clm-eeeeee", "clm-ffffff"
	const leaf1, leaf2 = "v1/leaf.md", "v2/leaf.md"
	const linked = "Bound by [the data](data.csv)"
	proof := "> **proof**\n>\n> *Proof.* Obvious.\n"
	root := seedKB(t, "# Entry\n\n- [V1](v1/index.md)\n- [V2](v2/index.md)\n", map[string]string{
		"v1/index.md": "[↑ Entry](../entry-point.md)\n\n# V1\n\n- [Leaf](leaf.md)\n",
		"v2/index.md": "[↑ Entry](../entry-point.md)\n\n# V2\n\n- [Leaf](leaf.md)\n",
		"v1/data.csv": "x\n",
		leaf1: "[↑ V1](index.md)\n\n# Leaf\n\n" +
			"> <span id=\"thm:d\">**Theorem**</span>\n>\n> **Theorem 1.** Delta holds here." + marked(d) + "\n\n" + proof + "\n" +
			"> <span id=\"thm:d3\">**Theorem**</span>\n>\n> **Theorem 2.** Delta three holds here." + marked(d3) + "\n>\n> 1.  An item.\n\n" + proof + "\n" +
			"> <span id=\"thm:d2\">**Theorem**</span>\n>\n> **Theorem 3** (" + linked + "). Delta two holds here." + marked(d2) + "\n",
		leaf2: "[↑ V2](index.md)\n\n# Leaf\n\n> **proof**\n>\n> *Proof of " +
			`<a href="../v1/leaf.md#thm:d" data-reference-type="ref" data-reference="thm:d">1</a> and ` +
			`<a href="../v1/leaf.md#thm:d2" data-reference-type="ref" data-reference="thm:d2">3</a>.* Both follow.` + "\n",
	}, kb.Graph{Claims: claimRecords(map[string]string{d: "Theorem 1", d2: linked, d3: "Theorem 2"})})
	repo := filepath.Dir(root)
	block := func(order int, doc, name, identifier string) records.Record {
		return records.Record{Kind: records.KindBlock, Order: order, Document: doc, Name: name, Identifier: identifier}
	}
	opening := func(order int, label string) records.Record {
		return records.Record{Kind: records.KindReference, Order: order, Document: leaf2, Within: 6, Opening: true, Type: "ref",
			Labels: label, Href: "../v1/leaf.md#" + label, Target: leaf1}
	}
	recs := []records.Record{
		block(1, leaf1, "Theorem", "thm:d"), block(2, leaf1, "proof", ""),
		block(3, leaf1, "Theorem", "thm:d3"), block(4, leaf1, "proof", ""),
		block(5, leaf1, "Theorem", "thm:d2"),
		block(6, leaf2, "proof", ""), opening(7, "thm:d"), opening(8, "thm:d2"),
	}
	recordsPath := filepath.Join(t.TempDir(), "docgraph.jsonl")
	var lines []byte
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(append(lines, line...), '\n')
	}
	if err := os.WriteFile(recordsPath, lines, 0o644); err != nil {
		t.Fatal(err)
	}
	before := map[string][]string{leaf1: leafLines(t, root, leaf1), leaf2: leafLines(t, root, leaf2)}

	report, err := Split(Options{KBRoot: root, RepoRoot: repo, Records: recordsPath, Logger: log.Discard()})
	requireStagePassed(t, report, err)

	g := loadRecords(t, root)
	if len(g.Analyses) != 3 {
		t.Fatalf("analyses %+v, want three", g.Analyses)
	}
	lifting := map[string]kb.AnalysisNode{}
	for _, e := range g.Scorable {
		if e.Relation == kb.RelationLifts {
			for _, n := range g.Analyses {
				if n.ID == e.Source {
					lifting[e.Target] = n
				}
			}
		}
	}
	// blankAfter is the 0-based line of doc's first blank quoted line below
	// the first line holding from, as the leaf stood before the stage.
	blankAfter := func(doc, from string) int {
		at := slices.IndexFunc(before[doc], func(l string) bool { return strings.Contains(l, from) })
		return at + slices.Index(before[doc][at:], ">")
	}
	for _, tc := range []struct {
		claim, doc string
		line       int
	}{
		{d, leaf1, blankAfter(leaf1, "**Theorem 1.**")},
		{d3, leaf1, blankAfter(leaf1, `id="thm:d3"`)},
		{d2, leaf2, blankAfter(leaf2, "**proof**")},
	} {
		n, ok := lifting[tc.claim]
		if !ok {
			t.Errorf("no analysis lifts %s", tc.claim)
			continue
		}
		if got := leafLines(t, root, tc.doc)[tc.line]; got != ">"+marked(n.ID) {
			t.Errorf("%s:%d = %q, want %s's analysis marked alone there", tc.doc, tc.line+1, got, tc.claim)
		}
	}
	if want := "Bound by [the data](../v1/data.csv)"; lifting[d2].Title != want {
		t.Errorf("D2's analysis title %q, want %q", lifting[d2].Title, want)
	}
	got := splitFinding(t, report)
	for key, want := range map[string]int{"analyses": 3, "in-own-text": 1, "at-a-proof": 2, "proofs-bound": 3, "proofs-in-no-statement": 1} {
		if got[key] != want {
			t.Errorf("finding %s = %v, want %d", key, got[key], want)
		}
	}
}

// TestSplitLeavesStatementOnlyClaims: a display-only theorem with no proof, a
// one-sentence prose claim and an equation node are none of them split; nor is
// a theorem whose block holds more but no blank line to mark an analysis on,
// which the finding counts. The stage leaves kb-root/ byte-identical.
func TestSplitLeavesStatementOnlyClaims(t *testing.T) {
	const p, d, d2, e = "clm-aaaaaa", "clm-dddddd", "clm-ffffff", "clm-eeeeee"
	root, _, opts := seedTheoremLeaf(t,
		map[string]string{p: "The only opening sentence holds alone.", d: "Theorem 1", d2: "Theorem 2", e: "Equation (`eq:one`) — Leaf"},
		"The only opening sentence holds alone."+marked(p)+"\n\n"+
			"> **Theorem**\n>\n> **Theorem 1.** Delta holds here."+marked(d)+"\n\n``` math\nx = 1 \\label{eq:one}"+marked(e)+"\n```\n\n"+
			"> **Theorem**\n> A preamble line.\n> **Theorem 2.** Epsilon holds here."+marked(d2)+"\n> A closing line.\n",
		records.Record{Kind: records.KindFence, Order: 2, Document: cullLeaf, EquationLabels: []string{"eq:one"}},
		records.Record{Kind: records.KindBlock, Order: 3, Document: cullLeaf, Name: "Theorem"})
	if f := gate(root, log.Discard()); slices.ContainsFunc(f, func(f Finding) bool { return f.Status == StatusFail }) {
		t.Fatalf("the seed does not pass the gate: %+v", f)
	}
	before := treeFiles(t, root)

	report, err := Split(opts)
	requireStagePassed(t, report, err)

	if after := treeFiles(t, root); !maps.Equal(after, before) {
		t.Error("the stage changed kb-root/ over claims with nothing beyond their statements")
	}
	if got := splitFinding(t, report); got["analyses"] != 0 || got["unsplit-no-blank-line"] != 1 {
		t.Errorf("finding %+v, want no analysis and Theorem 2 counted unsplit for want of a blank line", got)
	}
}
