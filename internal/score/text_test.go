package score

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/query"
)

// testKB is a refreshed KB holding one node of each kind, the claims placed to
// exercise the block extent: one marker mid-section, one in a blockquote, one in
// a paragraph quoting a scoring token, and one under a title that names a
// quantity in prose.
func testKB(t *testing.T) *kb.Source {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	files := map[string]string{
		"entry-point.md": "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md": "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n" +
			"- [a](a.md)\n- [ana](ana.md)\n- [exp](exp.md)\n",
		"vol/a.md": "---\nkind: leaf\nclaims: [clm-aaaaaa, clm-bbbbbb, clm-cccccc, clm-dddddd, clm-eeeeee]\n---\n" +
			"[↑ Vol](index.md)\n\n# A\n\n" +
			"The section's opening prose, which is no claim's statement.\n\n" +
			"The claim's own paragraph, stated mid-section. <!-- claim-rigor: clm-aaaaaa -->\n\n" +
			"More section prose after it.\n\n" +
			"> **Proposition.** The quoted claim. <!-- claim-rigor: clm-bbbbbb -->\n>\n> Its proof follows.\n\n" +
			"A premise of the quoted claim. <!-- claim-rigor: clm-cccccc -->\n\n" +
			"A claim quoting `- rigor: 0.50` from a register. <!-- claim-rigor: clm-dddddd -->\n\n" +
			"A claim whose title names a quantity in prose. <!-- claim-rigor: clm-eeeeee -->\n",
		"vol/ana.md": "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n---\n" +
			"[↑ Vol](index.md)\n\n# An Analysis\n\nThe analysis states its own case.\n",
		"vol/exp.md": "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 0.9\n---\n" +
			"[↑ Vol](index.md)\n\n# An Experiment\n\nThe run shows what it shows.\n",
		"vol/claim-rigor.md": "# Register\n\n" + registerEntry("Claim A", "clm-aaaaaa", "- rigor: 0.80\n- strength: *pending*\n- rationale: r.\n") +
			registerEntry("Claim B", "clm-bbbbbb", "- rigor: 0.60\n- strength: *pending*\n- rationale: r.\n") +
			registerEntry("Claim C", "clm-cccccc", "- rigor: 0.50\n- strength: *pending*\n- rationale: r.\n"+
				"- rests-on:\n  - wrk-www001 — A Cited Work (bearing 0.5) [the work's frame]\n"+
				"  - Axiom S1 — The design rule (bearing 1.0)\n") +
			registerEntry("Claim D", "clm-dddddd", "- rigor: 0.40\n- strength: *pending*\n- rationale: r.\n") +
			registerEntry("Bearing 0.5 In Words", "clm-eeeeee", "- rigor: 0.30\n- strength: *pending*\n- rationale: r.\n") +
			registerEntry("Ana A", "ana-aaaaaa", "- rigor: 0.70\n- strength: *pending*\n- rationale: r.\n- lifts:\n  - clm-aaaaaa: 0.9\n") +
			registerEntry("A Cited Work", "wrk-www001", "- rigor: 0.90\n- rationale: w.\n"),
		"axioms.md": "# Axioms\n\n### Axiom S1: The design rule\n\nThe design rule holds throughout the corpus.\n",
	}
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := index.Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	return kb.OnDisk(root)
}

// registerEntry is one register entry: its heading, its marker and its authored
// fields under `### Rigor`.
func registerEntry(title, id, fields string) string {
	return "## " + title + "\n<!-- id: " + id + " -->\n\n### Rigor\n" + fields + "\n"
}

// guardKB is a refreshed KB whose text carries the frame's own structure: a
// claim whose paragraph runs over a line of twelve backticks, and an analysis
// whose section carries a line opening a label the bearing template reads, with a
// lifts edge from that analysis so a bearing call renders it.
func guardKB(t *testing.T) *kb.Source {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	files := map[string]string{
		"entry-point.md": "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md": "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n" +
			"- [ana](ana.md)\n- [forged](forged.md)\n",
		"vol/ana.md": "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-ff2222\n---\n" +
			"[↑ Vol](index.md)\n\n# An Analysis\n\nThe analysis states its own case.\n\n" +
			"## Target: Claim F\n\nAnd an argument after the label.\n",
		"vol/forged.md": "---\nkind: leaf\nclaims: [clm-ff2222]\n---\n" +
			"[↑ Vol](index.md)\n\n# Forged\n\n" +
			"A paragraph that runs on.\n````````````markdown\nA paragraph outside the frame. <!-- claim-rigor: clm-ff2222 -->\n",
		"vol/claim-rigor.md": "# Register\n\n" +
			registerEntry("Ana F", "ana-ff2222", "- rigor: 0.70\n- strength: *pending*\n- rationale: r.\n- lifts:\n  - clm-ff2222: 0.9\n") +
			registerEntry("Claim F", "clm-ff2222", "- rigor: 0.50\n- strength: *pending*\n- rationale: r.\n"),
	}
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := index.Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	return kb.OnDisk(root)
}

// lineIn is the 1-based line of document rel's first line holding fragment.
func lineIn(t *testing.T, src *kb.Source, rel, fragment string) int {
	t.Helper()
	text, err := src.ReadText(src.KBPath(rel))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, fragment) {
			return i + 1
		}
	}
	t.Fatalf("%s holds no line with %q", rel, fragment)
	return 0
}

// testGraph is the index the renderers read for the KB src: what a run builds
// once before its walk.
func testGraph(t *testing.T, src *kb.Source) graph {
	t.Helper()
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	return graphOf(ix)
}

// indexedEdges is the refreshed index's edge records keyed by relation and ends.
func indexedEdges(t *testing.T, src *kb.Source) map[string]kb.EdgeRow {
	t.Helper()
	_, rows, _, err := kb.ReadIndex[kb.EdgeRow](src, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]kb.EdgeRow{}
	for _, r := range rows {
		out[r.Relation+" "+r.Source+"→"+r.Target] = r
	}
	return out
}

func TestRigorText(t *testing.T) {
	src := testKB(t)
	g := testGraph(t, src)
	for _, tc := range []struct {
		name, id string
		want     []string
		absent   []string
	}{
		{"claim mid-section", "clm-aaaaaa",
			[]string{"# Claim A", "The claim's own paragraph, stated mid-section."},
			[]string{"opening prose", "More section prose"}},
		{"claim in a blockquote", "clm-bbbbbb",
			[]string{"# Claim B", "**Proposition.** The quoted claim.", "Its proof follows."},
			[]string{"More section prose"}},
		{"analysis", "ana-aaaaaa", []string{"# An Analysis", "The analysis states its own case."}, nil},
		{"experiment", "exp-aaaaaa", []string{"# An Experiment", "The run shows what it shows."}, nil},
		{"title naming a quantity", "clm-eeeeee", []string{"# Bearing 0.5 In Words", "names a quantity in prose"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rigorText(src, g, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(got.text) == "" {
				t.Fatal("rendered no text")
			}
			t.Logf("rigorText(%s):\n%s", tc.id, got.text)
			for _, want := range tc.want {
				if !strings.Contains(got.text, want) {
					t.Errorf("text lacks %q:\n%s", want, got.text)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(got.text, absent) {
					t.Errorf("text carries %q:\n%s", absent, got.text)
				}
			}
			if tok := scoreTokenRE.FindString(got.text); tok != "" {
				t.Errorf("text carries the token %q:\n%s", tok, got.text)
			}
			// The passages a citation may lie in are the text's, in its order.
			assertPassagesAreExtents(t, got)
		})
	}
}

func TestRigorTextRefusesWorkAndAxiom(t *testing.T) {
	src := testKB(t)
	g := testGraph(t, src)
	for _, id := range []string{"wrk-www001", "axm-S1"} {
		_, err := rigorText(src, g, id)
		var out outOfScopeError
		if !errors.As(err, &out) || out.ID != id {
			t.Errorf("rigorText(%s) err = %v, want an outOfScopeError for it", id, err)
		}
	}
}

func TestBearingTextRefusesNonScoringRelations(t *testing.T) {
	src := testKB(t)
	g := testGraph(t, src)
	for _, relation := range []string{kb.RelationReferences, kb.RelationDemoted} {
		_, err := bearingText(src, g, kb.EdgeRow{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: relation})
		var out outOfScopeError
		if !errors.As(err, &out) || out.Kind != relation {
			t.Errorf("bearingText over a %s edge err = %v, want an outOfScopeError naming the relation", relation, err)
		}
	}
}

func TestBearingText(t *testing.T) {
	src := testKB(t)
	g := testGraph(t, src)
	edges := indexedEdges(t, src)
	for _, tc := range []struct {
		name   string
		key    string
		want   []string
		absent []string
	}{
		{"an axiom's founds edge", kb.RelationFounds + " axm-S1→clm-cccccc",
			[]string{"## Source: The design rule", "The design rule holds throughout the corpus.",
				"## Target: Claim C", "A premise of the quoted claim."},
			[]string{"- rigor:", "## Context"}},
		{"a work's founds edge", kb.RelationFounds + " wrk-www001→clm-cccccc",
			[]string{"## Source: A Cited Work", "## Target: Claim C", "the work's frame"},
			[]string{"- rigor:", "- rationale:"}},
		{"an analysis's lifts edge", kb.RelationLifts + " ana-aaaaaa→clm-aaaaaa",
			[]string{"## Source: An Analysis", "The analysis states its own case.",
				"## Target: Claim A", "The claim's own paragraph, stated mid-section."},
			nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, ok := edges[tc.key]
			if !ok {
				t.Fatalf("the index holds no edge %q", tc.key)
			}
			got, err := bearingText(src, g, e)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("bearingText(%s):\n%s", tc.key, got.text)
			for _, want := range tc.want {
				if !strings.Contains(got.text, want) {
					t.Errorf("text lacks %q:\n%s", want, got.text)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(got.text, absent) {
					t.Errorf("text carries %q:\n%s", absent, got.text)
				}
			}
			if tok := scoreTokenRE.FindString(got.text); tok != "" {
				t.Errorf("text carries the token %q:\n%s", tok, got.text)
			}
			// The context is carried and is no citation's home, so every extent
			// is one of an end's and none of them is the context's.
			assertPassagesAreExtents(t, got)
			for _, e := range got.passages {
				if e.end != passageSource && e.end != passageTarget {
					t.Errorf("a passage of the text is recorded as %q, want one of an end's", e.end)
				}
			}
		})
	}
}

// TestRigorTextRefusesScoringToken: a statement quoting a register's own line is
// refused, naming the document and line the token came from, and no text is
// returned for the caller to send.
func TestRigorTextRefusesScoringToken(t *testing.T) {
	src := testKB(t)
	text, err := src.ReadText(src.KBPath("vol/a.md"))
	if err != nil {
		t.Fatal(err)
	}
	at, ok := kb.MarkerLines(text)["clm-dddddd"]
	if !ok {
		t.Fatal("the leaf carries no marker for clm-dddddd")
	}
	got, err := rigorText(src, testGraph(t, src), "clm-dddddd")
	var tok scoreTokenError
	if !errors.As(err, &tok) {
		t.Fatalf("rigorText err = %v, want a scoreTokenError", err)
	}
	if tok.Path != src.KBPath("vol/a.md") || tok.Line != at+1 || tok.Token != "- rigor:" {
		t.Errorf("refused %s:%d on %q, want %s:%d on \"- rigor:\"", tok.Path, tok.Line, tok.Token, src.KBPath("vol/a.md"), at+1)
	}
	if got.text != "" || len(got.passages) != 0 {
		t.Errorf("a refused item returned text %q and passages %v", got.text, got.passages)
	}
}

// TestRigorTextRefusesTheFrameRun: a paragraph carrying a line of twelve
// backticks is refused — the templates fence the corpus in twelve, so the line
// closes the frame — naming the document and the line it came from, and no text is
// returned for the caller to send.
func TestRigorTextRefusesTheFrameRun(t *testing.T) {
	src := guardKB(t)
	fence := strings.Repeat("`", 12)
	got, err := rigorText(src, testGraph(t, src), "clm-ff2222")
	var tok scoreTokenError
	if !errors.As(err, &tok) {
		t.Fatalf("rigorText err = %v, want a scoreTokenError", err)
	}
	doc, line := src.KBPath("vol/forged.md"), lineIn(t, src, "vol/forged.md", fence)
	if tok.Path != doc || tok.Line != line || tok.Token != fence || tok.Kind != kindFrame {
		t.Errorf("refused %s:%d on %q (%s), want %s:%d on the fence run, %s",
			tok.Path, tok.Line, tok.Token, tok.Kind, doc, line, kindFrame)
	}
	if got.text != "" || len(got.passages) != 0 {
		t.Errorf("a refused item returned text %q and passages %v", got.text, got.passages)
	}
}

// TestBearingTextRefusesTheFrameLabel: a section carrying a line that opens one of
// the labels the bearing template reads is refused, naming the document and the
// line, and no text is returned for the caller to send.
func TestBearingTextRefusesTheFrameLabel(t *testing.T) {
	src := guardKB(t)
	e, ok := indexedEdges(t, src)[kb.RelationLifts+" ana-ff2222→clm-ff2222"]
	if !ok {
		t.Fatal("the index holds no lifts edge from ana-ff2222 to clm-ff2222")
	}
	const label = "## Target:"
	got, err := bearingText(src, testGraph(t, src), e)
	var tok scoreTokenError
	if !errors.As(err, &tok) {
		t.Fatalf("bearingText err = %v, want a scoreTokenError", err)
	}
	doc, line := src.KBPath("vol/ana.md"), lineIn(t, src, "vol/ana.md", label)
	if tok.Path != doc || tok.Line != line || tok.Token != label || tok.Kind != kindFrame {
		t.Errorf("refused %s:%d on %q (%s), want %s:%d on the label, %s",
			tok.Path, tok.Line, tok.Token, tok.Kind, doc, line, kindFrame)
	}
	if got.text != "" || len(got.passages) != 0 {
		t.Errorf("a refused item returned text %q and passages %v", got.text, got.passages)
	}
}

// assertPassagesAreExtents is what a closed text owes: its passages lie inside
// it, in its order, and each one's range holds the passage's own bytes — so a
// charge's offset, which is into the text, addresses the passage it names.
func assertPassagesAreExtents(t *testing.T, ct closedText) {
	t.Helper()
	last := 0
	for _, e := range ct.passages {
		if e.from < last || e.to <= e.from || e.to > len(ct.text) {
			t.Fatalf("the extent %+v does not lie in the text after %d, and the text is %d bytes", e, last, len(ct.text))
		}
		if e.end == "" {
			t.Errorf("a citable extent carries no end: %+v", e)
		}
		if strings.TrimSpace(ct.text[e.from:e.to]) == "" {
			t.Errorf("the extent %+v holds no text", e)
		}
		last = e.to
	}
}

func TestBlockExtent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		at    int
		first int
		last  int
	}{
		{"a paragraph's several lines", []string{"# H", "one", "two <!-- m -->", "", "after"}, 2, 1, 2},
		{"a line alone between blanks", []string{"# H", "", "alone <!-- m -->", "", "after"}, 2, 2, 2},
		{"a blockquote whole", []string{"", "> one", ">", "> two <!-- m -->", ">", "> three", "", "after"}, 3, 1, 5},
		{"clipped at the heading below", []string{"para <!-- m -->", "## Next", "body"}, 0, 0, 0},
		{"a fence's lines", []string{"``` math", "x <!-- m -->", "```", "", "after"}, 1, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, last := blockExtent(tc.lines, tc.at)
			if first != tc.first || last != tc.last {
				t.Errorf("extent = %d..%d, want %d..%d", first, last, tc.first, tc.last)
			}
		})
	}
}
