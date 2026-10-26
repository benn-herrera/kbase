package score

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/query"
)

// testGraphRecords is testKB's records: one node of each kind, a work and an
// axiom founding clm-cccccc, and an analysis lifting clm-aaaaaa. edit, where it
// is not nil, changes them before they are written.
func testGraphRecords(edit func(*kb.Graph)) kb.Graph {
	g := kb.Graph{
		Claims: []kb.ClaimNode{
			{ID: "clm-aaaaaa", Title: "Claim A", Rigor: rigor(0.8), Rationale: "r."},
			{ID: "clm-bbbbbb", Title: "Claim B", Rigor: rigor(0.6), Rationale: "r."},
			{ID: "clm-cccccc", Title: "Claim C", Rigor: rigor(0.5), Rationale: "r."},
			{ID: "clm-dddddd", Title: "Claim D", Rigor: rigor(0.4), Rationale: "r."},
			{ID: "clm-eeeeee", Title: "Bearing 0.5 In Words", Rigor: rigor(0.3), Rationale: "r."},
		},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "An Analysis", Rigor: rigor(0.7), Rationale: "r."}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "An Experiment", Status: "run", Rigor: rigor(0.9)}},
		Works:       []kb.WorkNode{{ID: "wrk-www001", Title: "A Cited Work", Rigor: rigor(0.9), Rationale: "w."}},
		Axioms:      []kb.AxiomNode{{ID: "axm-S1", Title: "The design rule"}},
		Founds: []kb.BearingEdge{
			{Source: "wrk-www001", Target: "clm-cccccc", Context: "the work's frame", Bearing: kb.Bearing{Value: 0.5}},
			{Source: "axm-S1", Target: "clm-cccccc", Bearing: kb.Bearing{Value: 1}},
		},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationLifts, Source: "ana-aaaaaa", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.9}}},
	}
	if edit != nil {
		edit(&g)
	}
	return g
}

// testKB is a refreshed KB holding one node of each kind, the claims placed to
// exercise the block extent: one marker mid-section, one in a blockquote, one in
// a paragraph quoting a scoring token, and one under a title that names a
// quantity in prose.
func testKB(t *testing.T) *kb.Source {
	t.Helper()
	return editedTestKB(t, nil)
}

// editedTestKB is testKB with its records changed by edit before they are
// written.
func editedTestKB(t *testing.T, edit func(*kb.Graph)) *kb.Source {
	t.Helper()
	return refreshedKB(t, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n- [ana](ana.md)\n- [exp](exp.md)\n",
		"vol/a.md": "[↑ Vol](index.md)\n\n# A\n\n" +
			"The section's opening prose, which is no claim's statement.\n\n" +
			"The claim's own paragraph, stated mid-section. <!-- claim-rigor: clm-aaaaaa -->\n\n" +
			"More section prose after it.\n\n" +
			"> **Proposition.** The quoted claim. <!-- claim-rigor: clm-bbbbbb -->\n>\n> Its proof follows.\n\n" +
			"A premise of the quoted claim. <!-- claim-rigor: clm-cccccc -->\n\n" +
			"A claim quoting `- rigor: 0.50` from a register. <!-- claim-rigor: clm-dddddd -->\n\n" +
			"A claim whose title names a quantity in prose. <!-- claim-rigor: clm-eeeeee -->\n",
		"vol/ana.md": "[↑ Vol](index.md)\n\n# An Analysis\n\nThe analysis states its own case. <!-- claim-rigor: ana-aaaaaa -->\n",
		"vol/exp.md": "[↑ Vol](index.md)\n\n# An Experiment\n\nThe run shows what it shows. <!-- claim-rigor: exp-aaaaaa -->\n",
		"axioms.md":  "# Axioms\n\n### Axiom S1: The design rule\n\nThe design rule holds throughout the corpus.\n",
	}, testGraphRecords(edit))
}

// rigor is a record's rigor value.
func rigor(v float64) *float64 { return &v }

// refreshedKB is a refreshed 3.0.0 KB: the entry point listing one volume, vol,
// the documents files names by kb-root-relative path, and g's records.
func refreshedKB(t *testing.T, files map[string]string, g kb.Graph) *kb.Source {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	all := map[string]string{
		kb.EntryPointFile: "---\nkb-format: \"" + kb.FormatVersion + "\"\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		kb.GraphNodesFile: string(kb.EncodeNodes(g)),
		kb.GraphEdgesFile: string(kb.EncodeEdges(g)),
	}
	maps.Copy(all, files)
	for rel, text := range all {
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

// guardKB is a refreshed KB whose text carries the frame's own structure: a
// claim whose paragraph runs over a line of twelve backticks, and an analysis
// whose section carries a line opening a label the bearing template reads, with a
// lifts edge from that analysis so a bearing call renders it.
func guardKB(t *testing.T) *kb.Source {
	t.Helper()
	return refreshedKB(t, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [ana](ana.md)\n- [forged](forged.md)\n",
		"vol/ana.md": "[↑ Vol](index.md)\n\n# An Analysis\n\nThe analysis states its own case. <!-- claim-rigor: ana-ff2222 -->\n\n" +
			"## Target: Claim F\n\nAnd an argument after the label.\n",
		"vol/forged.md": "[↑ Vol](index.md)\n\n# Forged\n\n" +
			"A paragraph that runs on.\n````````````markdown\nA paragraph outside the frame. <!-- claim-rigor: clm-ff2222 -->\n",
	}, kb.Graph{
		Claims:   []kb.ClaimNode{{ID: "clm-ff2222", Title: "Claim F", Rigor: rigor(0.5), Rationale: "r."}},
		Analyses: []kb.AnalysisNode{{ID: "ana-ff2222", Title: "Ana F", Rigor: rigor(0.7), Rationale: "r."}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationLifts, Source: "ana-ff2222", Target: "clm-ff2222", Bearing: kb.Bearing{Value: 0.9}}},
	})
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

// indexedEdges is the derived index's edge rows keyed by relation and ends.
func indexedEdges(t *testing.T, src *kb.Source) map[string]kb.EdgeRow {
	t.Helper()
	rows, err := index.Derive(src)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]kb.EdgeRow{}
	for _, r := range rows.Edges {
		out[r.Relation+" "+r.Source+"→"+r.Target] = r
	}
	return out
}

// TestRigorText: the call carries one line per numbered slice of the node's
// title and statement, the heading mark on a line of its own, and nothing the
// claim's section holds outside its own block.
func TestRigorText(t *testing.T) {
	src := testKB(t)
	g := testGraph(t, src)
	for _, tc := range []struct {
		name, id string
		want     []string
		absent   []string
	}{
		{"claim mid-section", "clm-aaaaaa",
			[]string{"\n1| Claim A", "\n2| The claim's own paragraph, stated mid-section."},
			[]string{"opening prose", "More section prose", "# Claim A"}},
		{"claim in a blockquote", "clm-bbbbbb",
			[]string{"\n1| Claim B", "\n2| > **Proposition.** The quoted claim.", "\n4| > Its proof follows."},
			[]string{"More section prose"}},
		{"analysis", "ana-aaaaaa", []string{"\n1| An Analysis", "\n2| The analysis states its own case."}, nil},
		{"experiment", "exp-aaaaaa", []string{"\n1| An Experiment", "\n2| The run shows what it shows."}, nil},
		{"title naming a quantity", "clm-eeeeee", []string{"\n1| Bearing 0.5 In Words", "\n2| A claim whose title names a quantity in prose."}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rigorText(src, g, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(got.text) == "" {
				t.Fatal("rendered no text")
			}
			numbered := got.numbered()
			t.Logf("rigorText(%s):\n%s", tc.id, numbered)
			for _, want := range tc.want {
				if !strings.Contains("\n"+numbered, want) {
					t.Errorf("the rendering lacks a line opening %q:\n%s", want, numbered)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(numbered, absent) {
					t.Errorf("the rendering carries %q:\n%s", absent, numbered)
				}
			}
			if tok := scoreTokenRE.FindString(got.text); tok != "" {
				t.Errorf("text carries the token %q:\n%s", tok, got.text)
			}
			assertNumbered(t, got)
		})
	}
}

// TestClaimMarkerOnAHeading: a claim whose marker ends a heading's line hosts
// the section that heading opens — the body under it to the next heading as
// deep or shallower, its subsections included — and never the heading's own
// text; a heading whose section holds no body gives the claim no statement, so
// its text is its title alone.
func TestClaimMarkerOnAHeading(t *testing.T) {
	src := refreshedKB(t, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md": "[↑ Vol](index.md)\n\n# A\n\n" +
			"## The Claimed Section <!-- claim-rigor: clm-aaaaaa -->\n\n" +
			"The section's opening paragraph.\n\n" +
			"``` math\n# not a heading\n\\dot x = 0\n```\n\n" +
			"A second paragraph of the same section.\n\n" +
			"### A Subsection\n\n" +
			"The subsection's prose, still the claim's.\n\n" +
			"## The Next Section\n\n" +
			"Prose the claim does not own.\n\n" +
			"## Empty <!-- claim-rigor: clm-bbbbbb -->\n\n" +
			"## After The Empty One\n\n" +
			"More prose nobody owns.\n",
	}, testGraphRecords(func(g *kb.Graph) {
		g.Claims = []kb.ClaimNode{
			{ID: "clm-aaaaaa", Title: "The Claimed Section", Rigor: rigor(0.8), Rationale: "r."},
			{ID: "clm-bbbbbb", Title: "Empty", Rigor: rigor(0.6), Rationale: "r."},
		}
		g.Scorable = nil
	}))
	g := testGraph(t, src)

	passages, err := claimPassages(src, g, g.nodes["clm-aaaaaa"])
	if err != nil {
		t.Fatal(err)
	}
	want := "The section's opening paragraph.\n\n``` math\n# not a heading\n\\dot x = 0\n```\n\n" +
		"A second paragraph of the same section.\n\n### A Subsection\n\nThe subsection's prose, still the claim's."
	if len(passages) != 1 || passages[0].text != want {
		t.Fatalf("claimPassages = %+v, want the section under the heading", passages)
	}
	if at, want := passages[0].line, lineIn(t, src, "vol/a.md", "The section's opening paragraph."); at != want {
		t.Errorf("the statement opens on line %d, want %d", at, want)
	}

	got, err := rigorText(src, g, "clm-aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	numbered := got.numbered()
	t.Logf("rigorText(clm-aaaaaa):\n%s", numbered)
	for _, absent := range []string{"Prose the claim does not own", "After The Empty One", "More prose nobody owns"} {
		if strings.Contains(numbered, absent) {
			t.Errorf("the rendering carries %q:\n%s", absent, numbered)
		}
	}
	assertNumbered(t, got)

	empty, err := rigorText(src, g, "clm-bbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(empty.lines, func(l line) bool { return l.end == passageStatement }) {
		t.Errorf("the claim under an empty heading carries a statement:\n%s", empty.numbered())
	}
	if strings.Contains(empty.text, "More prose nobody owns") {
		t.Errorf("the claim under an empty heading carries the section below it:\n%s", empty.numbered())
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

// TestBearingText: each end's label opens a line of its own, each end's title and
// statement are the lines under it, and the edge's note is carried with no line
// either charge may cite.
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
			[]string{"\n" + sourceLabel + "\n", "\n1| The design rule",
				"\n2| The design rule holds throughout the corpus.",
				"\n" + targetLabel + "\n", "\n3| Claim C", "\n4| A premise of the quoted claim."},
			[]string{"- rigor:", contextLabel}},
		{"a work's founds edge", kb.RelationFounds + " wrk-www001→clm-cccccc",
			[]string{"\n" + sourceLabel + "\n", "\n1| A Cited Work", "\n" + targetLabel + "\n",
				"\n2| Claim C", "\n3| A premise of the quoted claim.",
				"\n" + contextLabel + "\n", "\nthe work's frame"},
			[]string{"- rigor:", "- rationale:", "| the work's frame"}},
		{"an analysis's lifts edge", kb.RelationLifts + " ana-aaaaaa→clm-aaaaaa",
			[]string{"\n1| An Analysis", "\n2| The analysis states its own case.",
				"\n3| Claim A", "\n4| The claim's own paragraph, stated mid-section."},
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
			numbered := got.numbered()
			t.Logf("bearingText(%s):\n%s", tc.key, numbered)
			for _, want := range tc.want {
				if !strings.Contains("\n"+numbered, want) {
					t.Errorf("the rendering lacks %q:\n%s", want, numbered)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(numbered, absent) {
					t.Errorf("the rendering carries %q:\n%s", absent, numbered)
				}
			}
			if tok := scoreTokenRE.FindString(got.text); tok != "" {
				t.Errorf("text carries the token %q:\n%s", tok, got.text)
			}
			// The context is carried and is no citation's home, so every
			// numbered line is one of an end's and none is the context's.
			assertNumbered(t, got)
			for _, l := range got.lines {
				if l.no > 0 && l.end != passageSource && l.end != passageTarget {
					t.Errorf("the numbered line %d is recorded as %q, want one of an end's", l.no, l.end)
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
	if got.text != "" || len(got.lines) != 0 {
		t.Errorf("a refused item returned text %q and lines %v", got.text, got.lines)
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
	if got.text != "" || len(got.lines) != 0 {
		t.Errorf("a refused item returned text %q and lines %v", got.text, got.lines)
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
	if got.text != "" || len(got.lines) != 0 {
		t.Errorf("a refused item returned text %q and lines %v", got.text, got.lines)
	}
}

// assertNumbered is what a closed text owes: every line lies in the text, in its
// order and behind the break before it; every numbered line is a slice of a
// passage a charge may cite, holds non-space content, and the numbers run from 1
// with no gap.
func assertNumbered(t *testing.T, ct closedText) {
	t.Helper()
	want, last := 1, 0
	for _, l := range ct.lines {
		if l.from < last || l.to < l.from || l.to > len(ct.text) {
			t.Fatalf("the line %+v does not lie in the text after %d, and the text is %d bytes", l, last, len(ct.text))
		}
		if l.from > 0 && ct.text[l.from-1] != '\n' {
			t.Errorf("the line %+v does not open after a break", l)
		}
		if strings.Contains(ct.text[l.from:l.to], "\n") {
			t.Errorf("the line %+v runs over a break: %q", l, ct.text[l.from:l.to])
		}
		last = l.to
		if l.no == 0 {
			continue
		}
		if l.no != want {
			t.Errorf("a numbered line carries %d, want %d: the numbers run from 1 with no gap", l.no, want)
		}
		want++
		if l.end == "" {
			t.Errorf("the numbered line %d lies in no citable passage: %q", l.no, ct.text[l.from:l.to])
		}
		if strings.TrimSpace(ct.text[l.from:l.to]) == "" {
			t.Errorf("the numbered line %d holds no content: %q", l.no, ct.text[l.from:l.to])
		}
	}
	if want == 1 {
		t.Error("the text numbers no line")
	}
}

// splitGutter is a rendered line's number and the text behind the gutter, ok false
// where the line carries no number.
func splitGutter(line string) (no int, text string, ok bool) {
	digits, text, found := strings.Cut(line, "| ")
	if !found {
		return 0, line, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, line, false
	}
	return n, text, true
}

// TestNumberedRendering: the gutter is the frame's, and it reaches no label and no
// note. Every rendered line is a label, a blank line or the edge's note carried
// bare, or a numbered line whose number follows the one before it and whose text
// behind the gutter is the numbered line's own slice of the carried text.
func TestNumberedRendering(t *testing.T) {
	src := testKB(t)
	e, ok := indexedEdges(t, src)[kb.RelationFounds+" wrk-www001→clm-cccccc"]
	if !ok {
		t.Fatal("the index holds no founds edge from wrk-www001 to clm-cccccc")
	}
	ct, err := bearingText(src, testGraph(t, src), e)
	if err != nil {
		t.Fatal(err)
	}
	numbered := ct.numbered()
	t.Logf("bearingText(wrk-www001→clm-cccccc):\n%s", numbered)
	var citable []string
	for _, l := range ct.lines {
		if l.no > 0 {
			citable = append(citable, ct.text[l.from:l.to])
		}
	}
	var sawLabel, sawNote bool
	next := 1
	for i, rendered := range strings.Split(numbered, "\n") {
		switch {
		case rendered == sourceLabel, rendered == targetLabel, rendered == contextLabel:
			sawLabel = true
			continue
		case rendered == *e.Context:
			sawNote = true
			continue
		case rendered == "":
			// A blank line between two passages is carried bare.
			continue
		}
		no, text, ok := splitGutter(rendered)
		if !ok {
			t.Errorf("line %d of the rendering is no numbered line, label, note or blank line: %q", i+1, rendered)
			continue
		}
		if no != next {
			t.Errorf("line %d of the rendering is numbered %d, want %d", i+1, no, next)
		}
		next = no + 1
		if want := citable[no-1]; text != want {
			t.Errorf("the line numbered %d renders %q, and the text's numbered line is %q", no, text, want)
		}
	}
	if !sawLabel || !sawNote {
		t.Errorf("the rendering holds no label (%t) or no note (%t):\n%s", sawLabel, sawNote, numbered)
	}
	if want := len(citable) + 1; next != want {
		t.Errorf("the rendering numbers %d lines, and the text's numbered lines are %d", next-1, want-1)
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

// TestBreakPoints: a break falls at a space, and never at one a formula holds. A
// dollar opens a span only where a matching run closes it in the same line, so a
// price costs no break, and an escaped dollar opens nothing.
func TestBreakPoints(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		want       []int
	}{
		{"plain words", "a b c", []int{1, 3}},
		{"a formula holds its spaces", "$a b$ c", []int{5}},
		{"display math too", "$$a b$$ c", []int{7}},
		{"a lone dollar opens no span", "costs $5 and more", []int{5, 8, 12}},
		{"an escaped dollar opens none", `a \$x b $c d$`, []int{1, 5, 7}},
		{"no space at all", "unbroken", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := breakPoints(tc.line)
			if !slices.Equal(got, tc.want) {
				t.Errorf("breakPoints(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

// TestWrapLine: wrapping breaks at the last space a line fits behind and inserts
// nothing else, so the pieces joined by newlines are the line. A word wider than
// the line stands whole, and a formula's own spaces are no break.
func TestWrapLine(t *testing.T) {
	formula := "$" + strings.Repeat("f", 46) + " " + strings.Repeat("g", 46) + "$"
	for _, tc := range []struct {
		name, line string
		want       []string
	}{
		{"a line inside the width", "the map is continuous", []string{"the map is continuous"}},
		{"a break at the last space that fits",
			strings.Repeat("a", 60) + " " + strings.Repeat("b", 60),
			[]string{strings.Repeat("a", 60) + " ", strings.Repeat("b", 60)}},
		{"a word wider than the width stands whole",
			strings.Repeat("a", 120) + " b",
			[]string{strings.Repeat("a", 120) + " ", "b"}},
		{"a formula keeps its own space",
			"lead " + formula + " tail",
			[]string{"lead ", formula + " tail"}},
		{"nothing at all", "", []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapLine(tc.line)
			if !slices.Equal(got, tc.want) {
				t.Errorf("wrapLine(%q) = %q, want %q", tc.line, got, tc.want)
			}
			if joined := strings.Join(got, "\n"); strings.ReplaceAll(joined, "\n", "") != strings.ReplaceAll(tc.line, "\n", "") {
				t.Errorf("wrapLine(%q) joined = %q: wrapping rewrote the line", tc.line, joined)
			}
		})
	}
}

// TestLookResolvesLines: a charge's field names lines of the carried text, and the
// index resolves them to the passage they lie in, the quote behind the gutter and
// the numbers as the ledger spells them. A range crosses no passage and no gap,
// every dash the form admits is read alike, and a field naming no line the call
// shows is refused with the case named.
func TestLookResolvesLines(t *testing.T) {
	ct, err := assemble([]passage{
		{end: passageSource, text: "The map is continuous.\nEvery chart commutes."},
		{end: passageTarget, text: "A bound holds."},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertNumbered(t, ct)
	for _, tc := range []struct {
		name, field string
		want        cite
		why         []string
	}{
		{name: "one line", field: "1",
			want: cite{end: passageSource, lines: "1", citation: "The map is continuous.", offset: 0, lo: 1, hi: 1}},
		{name: "a line of the target", field: "3",
			want: cite{end: passageTarget, lines: "3", citation: "A bound holds.", offset: len(ct.text) - len("A bound holds."), lo: 3, hi: 3}},
		{name: "a range in one passage", field: "1-2",
			want: cite{end: passageSource, lines: "1-2", citation: "The map is continuous.\nEvery chart commutes.", offset: 0, lo: 1, hi: 2}},
		{name: "the range's hyphens", field: "1–2", want: cite{lines: "1-2", lo: 1, hi: 2}},
		{name: "the range's em dash", field: "1—2", want: cite{lines: "1-2", lo: 1, hi: 2}},
		{name: "the range's minus sign", field: "1−2", want: cite{lines: "1-2", lo: 1, hi: 2}},
		{name: "spaces around the dash", field: "1 - 2", want: cite{lines: "1-2", lo: 1, hi: 2}},
		{name: "one space after the dash", field: "1- 2", want: cite{lines: "1-2", lo: 1, hi: 2}},
		{name: "a range across the two passages", field: "2-3",
			why: []string{"crosses from one passage to another", "2-3"}},
		{name: "a range across the blank line between them", field: "1-3",
			why: []string{"crosses from one passage to another", "1-3"}},
		{name: "a number the call does not show", field: "4",
			why: []string{"line 4 is not one the call shows"}},
		{name: "no line zero", field: "0", why: []string{"line 0 is not one the call shows"}},
		{name: "a span of text", field: "the map",
			why: []string{`field "the map" names no line`, "one line number or a range N-M"}},
		{name: "a range backwards", field: "2-1", why: []string{"runs backwards"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := ct.look(tc.field)
			if len(tc.why) > 0 {
				if why == "" {
					t.Fatalf("look(%q) = %+v, want a refusal", tc.field, got)
				}
				for _, want := range tc.why {
					if !strings.Contains(why, want) {
						t.Errorf("the refusal %q does not name %q", why, want)
					}
				}
				return
			}
			if why != "" {
				t.Fatalf("look(%q): %s", tc.field, why)
			}
			if tc.want.lines != "" && (got.lines != tc.want.lines || got.lo != tc.want.lo || got.hi != tc.want.hi) {
				t.Errorf("look(%q) names %+v, want %+v", tc.field, got, tc.want)
			}
			if tc.want.end != "" && got != tc.want {
				t.Errorf("look(%q) = %+v, want %+v", tc.field, got, tc.want)
			}
			if !strings.HasPrefix(ct.text[got.offset:], got.citation) {
				t.Errorf("look(%q) records the offset %d and the quote %q, which is not the text there", tc.field, got.offset, got.citation)
			}
		})
	}
}

// TestLookRefusesAGap: a blank line and a heading each end a range, so the lines
// either side of one are two sites and no one charge. A heading a range names as
// its own last line is not between its lines.
func TestLookRefusesAGap(t *testing.T) {
	ct, err := assemble([]passage{
		{end: passageStatement, text: "one\n\ntwo\n# Next\nthree"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertNumbered(t, ct)
	if got, why := ct.look("4"); why != "" {
		t.Errorf("look(4): %s", why)
	} else if got.citation != "three" {
		t.Errorf("look(4) quotes %q, want the last line", got.citation)
	}
	if got, why := ct.look("2-3"); why != "" {
		t.Errorf("look(2-3): %s", why)
	} else if got.citation != "two\n# Next" {
		t.Errorf("look(2-3) quotes %q, want the two lines it names", got.citation)
	}
	for _, tc := range []struct{ field, crosses string }{
		{"1-2", "crosses a gap"},
		{"2-4", "crosses a gap"},
	} {
		if _, why := ct.look(tc.field); !strings.Contains(why, tc.crosses) {
			t.Errorf("look(%q) = %q, want a refusal naming what it crosses", tc.field, why)
		}
	}
}
