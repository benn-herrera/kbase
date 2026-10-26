package index

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/result"
)

// graphKB is an in-memory KB holding g's record files and docs, each by
// kb-root-relative path.
func graphKB(t *testing.T, g kb.Graph, docs map[string]string) *kb.Source {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		kb.KBDir + "/" + kb.GraphNodesFile: kb.EncodeNodes(g),
		kb.KBDir + "/" + kb.GraphEdgesFile: kb.EncodeEdges(g),
	}
	for rel, text := range docs {
		files[kb.KBDir+"/"+rel] = []byte(text)
	}
	return kb.MigratedSource(root, kb.FormatVersion, files, nil)
}

// nodeRow is rows' node row of id.
func nodeRow(t *testing.T, rows Rows, id string) kb.NodeRow {
	t.Helper()
	i := slices.IndexFunc(rows.Nodes, func(r kb.NodeRow) bool { return r.ID == id })
	if i < 0 {
		t.Fatalf("no node row for %s", id)
	}
	return rows.Nodes[i]
}

// TestDerivePlacesEveryKind: a claim hosted in a volume is on its volume's
// register page, one hosted outside every volume — in a directory holding no
// index.md, or at kb-root — on the root page, and an unmarked one on none, with
// no canonical path or anchor; an analysis's and an experiment's path is the
// host leaf and their anchor the slug of the heading above the marker, a heading
// inside code not counting; a work is on the root page, and an axiom on none,
// anchored at its axioms.md heading.
func TestDerivePlacesEveryKind(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "In a volume"}, {ID: "clm-bbbbbb", Title: "In a directory no volume is"},
			{ID: "clm-cccccc", Title: "At the root"}, {ID: "clm-dddddd", Title: "Unmarked"}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "A method"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "A run", Status: "run"}, {ID: "exp-bbbbbb", Title: "Unmarked", Status: "pending"}},
		Works:       []kb.WorkNode{{ID: "wrk-key2020", Title: "Key, A. 2020. *Book*."}},
		Axioms:      []kb.AxiomNode{{ID: "axm-A1", Title: "A rule"}},
	}
	rows, err := Derive(graphKB(t, g, map[string]string{
		kb.EntryPointFile: "# KB\n\n- [v](v/index.md)\n",
		"v/index.md":      "[↑ KB](../entry-point.md)\n\n# V\n\n- [leaf](leaf.md)\n",
		"v/leaf.md": "[↑ V](index.md)\n\n# Leaf\n\nA claim. <!-- claim-rigor: clm-aaaaaa -->\n\n" +
			"## Method\n<!-- claim-rigor: ana-aaaaaa -->\n\n### Run\n\n```\n# not a heading\n```\n<!-- claim-rigor: exp-aaaaaa -->\n",
		"loose/x.md": "# X\n\nSays so. <!-- claim-rigor: clm-bbbbbb -->\n",
		"top.md":     "# Top\n\nSays so. <!-- claim-rigor: clm-cccccc -->\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, path, anchor, page string }{
		{"clm-aaaaaa", "v/claim-rigor.md", "in-a-volume", "v/claim-rigor.md"},
		{"ana-aaaaaa", "v/leaf.md", "method", "v/claim-rigor.md"},
		{"exp-aaaaaa", "v/leaf.md", "run", "v/claim-rigor.md"},
		{"clm-bbbbbb", "claim-rigor.md", "in-a-directory-no-volume-is", "claim-rigor.md"},
		{"clm-cccccc", "claim-rigor.md", "at-the-root", "claim-rigor.md"},
		{"clm-dddddd", "", "", ""},
		{"exp-bbbbbb", "", "", ""},
		{"wrk-key2020", "claim-rigor.md", "key-a-2020-book", "claim-rigor.md"},
		{"axm-A1", "axioms.md", "axiom-a1-a-rule", ""},
	} {
		r := nodeRow(t, rows, tc.id)
		if r.CanonicalPath != tc.path || r.CanonicalAnchor != tc.anchor {
			t.Errorf("%s: canonical %q#%q, want %q#%q", tc.id, r.CanonicalPath, r.CanonicalAnchor, tc.path, tc.anchor)
		}
		if page, ok := rows.RegisterPages[tc.id]; page != tc.page || ok != (tc.page != "") {
			t.Errorf("%s: register page %q (placed %t), want %q", tc.id, page, ok, tc.page)
		}
	}
}

// TestDeriveOverTheSheetFixture: the converted sheet fixture derives the
// strengths its README works out, each node's canonical path and anchor — the
// claims no leaf marks having none — a cite per marked claim, and the entry
// point's subtree, its volumes' index.md files listing no child and so being
// leaves.
func TestDeriveOverTheSheetFixture(t *testing.T) {
	src, err := kbload.Open(sheetFixtureKB)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Derive(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Nodes) != sheetFixtureNodes || len(rows.Edges) != sheetFixtureEdges {
		t.Errorf("%d nodes and %d edges, want %d and %d", len(rows.Nodes), len(rows.Edges), sheetFixtureNodes, sheetFixtureEdges)
	}
	if len(rows.Degraded) != 0 {
		t.Errorf("the fixture loads degraded: %+v", rows.Degraded)
	}
	for _, tc := range []struct{ id, path, anchor, strength string }{
		{"clm-aaa001", "alpha/claim-rigor.md", "theorem-1", "0.90"},
		{"clm-aaa002", "alpha/claim-rigor.md", "equation-eqflow-flow-dynamics", "0.95"},
		{"clm-aaa003", "alpha/claim-rigor.md", "the-gradient-nabla-f-is-bounded-when-a-b-c", "0.60"},
		{"clm-aaa004", "alpha/claim-rigor.md",
			"a-deliberately-long-prose-claim-title-that-runs-well-past-the-three-wrapped-lines-a-sheet-label-allows-before-it-is-cut", "0.40"},
		{"clm-aaa005", "", "", "0.00"},
		{"clm-aaa006", "", "", "0.00"},
		{"clm-bbb001", "", "", "0.88"},
		{"clm-bbb002", "beta/claim-rigor.md", "the-governance-bifurcation", "0.66"},
		{"clm-bbb003", "", "", kb.PendingLiteral},
		{"ana-aaa007", "alpha/analysis.md", "a-supporting-lemma", "0.80"},
		{"exp-aaa008", "alpha/experiments/flow.md", "flow-simulation", "0.80"},
		{"axm-1", "axioms.md", "axiom-1-agents-act-on-what-they-observe", "1.00"},
		{"axm-S1", "axioms.md", "axiom-s1-conservation-of-stock", "1.00"},
		{"wrk-smith2020", "claim-rigor.md", "smith-a-2020-markets-fixture-press", "0.90"},
	} {
		r := nodeRow(t, rows, tc.id)
		strength := r.Strength
		if r.NodeType != kb.NodeKindClaim && r.NodeType != kb.NodeKindAnalysis {
			strength = r.Rigor
		}
		if r.CanonicalPath != tc.path || r.CanonicalAnchor != tc.anchor || FormatStrengthDisplay(strength) != tc.strength {
			t.Errorf("%s: %q#%q at %s, want %q#%q at %s", tc.id, r.CanonicalPath, r.CanonicalAnchor, FormatStrengthDisplay(strength),
				tc.path, tc.anchor, tc.strength)
		}
	}
	var cited []string
	for _, c := range rows.Cites {
		cited = append(cited, c.ClaimID+" "+c.LeafPath)
	}
	if want := []string{"clm-aaa001 alpha/results.md", "clm-aaa002 alpha/results.md", "clm-aaa003 alpha/results.md",
		"clm-aaa004 alpha/results.md", "clm-bbb002 beta/governance.md"}; !slices.Equal(cited, want) {
		t.Errorf("cites %q, want %q", cited, want)
	}
	want := []kb.AggregateRow{{NodePath: kb.EntryPointFile, NodeKind: kb.DocumentEntryPoint,
		SubtreeClaims:      []string{"clm-aaa001", "clm-aaa002", "clm-aaa003", "clm-aaa004", "clm-bbb002"},
		SubtreeExperiments: []string{"exp-aaa008"}}}
	if len(rows.Aggregates) != 1 || rows.Aggregates[0].NodePath != want[0].NodePath || rows.Aggregates[0].NodeKind != want[0].NodeKind ||
		!slices.Equal(rows.Aggregates[0].SubtreeClaims, want[0].SubtreeClaims) ||
		!slices.Equal(rows.Aggregates[0].SubtreeExperiments, want[0].SubtreeExperiments) {
		t.Errorf("aggregates %+v, want %+v", rows.Aggregates, want)
	}
}

// TestDeriveRefusesNeitherACycleNorADanglingEdge: a supports cycle and an edge
// whose source no record carries leave Derive answering, every edge a row.
func TestDeriveRefusesNeitherACycleNorADanglingEdge(t *testing.T) {
	one := kb.Bearing{Value: 1}
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A"}, {ID: "clm-bbbbbb", Title: "B"}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: one},
			{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: one}},
		Founds: []kb.BearingEdge{{Source: "wrk-gone", Target: "clm-aaaaaa", Bearing: one}},
	}
	rows, err := Derive(graphKB(t, g, map[string]string{kb.EntryPointFile: "# KB\n"}))
	if err != nil {
		t.Fatalf("Derive = %v, want rows", err)
	}
	if len(rows.Edges) != 3 {
		t.Errorf("%d edge rows, want 3", len(rows.Edges))
	}
}

// TestDeriveKeepsOneRecordOfAnEdgeIdentity: of the records sharing an
// identity the walk reads the one of the higher bearing, a number above
// pending, and the first on a tie; of a demoted record's the replaced edge's
// bearing decides, and its replaced relation is part of its identity; of
// references, the first. Each record left out is one warning.
func TestDeriveKeepsOneRecordOfAnEdgeIdentity(t *testing.T) {
	b := func(v float64) kb.Bearing { return kb.Bearing{Value: v} }
	pending := kb.Bearing{Pending: true}
	g := kb.Graph{
		Claims:   []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f(1)}, {ID: "clm-bbbbbb", Title: "B", Rigor: f(1)}},
		Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "M"}},
		Works:    []kb.WorkNode{{ID: "wrk-key2020", Title: "W", Rigor: f(1)}},
		Founds: []kb.BearingEdge{{Source: "wrk-key2020", Target: "clm-aaaaaa", Bearing: pending},
			{Source: "wrk-key2020", Target: "clm-aaaaaa", Bearing: b(0.3)}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "first", Bearing: b(0.5)},
			{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "second", Bearing: b(0.5)},
			{Relation: kb.RelationLifts, Source: "ana-aaaaaa", Target: "clm-bbbbbb", Bearing: b(0.4)},
			{Relation: kb.RelationLifts, Source: "ana-aaaaaa", Target: "clm-bbbbbb", Bearing: b(0.8)},
		},
		Demoted: []kb.DemotedEdge{
			{Source: "ana-aaaaaa", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationLifts, Bearing: pending},
			{Source: "ana-aaaaaa", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationLifts, Bearing: b(0.2)},
			{Source: "ana-aaaaaa", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationRefutes, Bearing: b(0.1)},
		},
		References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "one"},
			{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "two"}},
	}
	rows, err := Derive(graphKB(t, g, map[string]string{kb.EntryPointFile: "# KB\n"}))
	if err != nil {
		t.Fatal(err)
	}
	want := kb.Graph{
		Founds:     g.Founds[1:],
		Scorable:   []kb.ScorableEdge{g.Scorable[0], g.Scorable[3]},
		Demoted:    g.Demoted[1:],
		References: g.References[:1],
	}
	r := rows.Resolved
	if !slices.Equal(r.Founds, want.Founds) || !slices.Equal(r.Scorable, want.Scorable) ||
		!slices.Equal(r.Demoted, want.Demoted) || !slices.Equal(r.References, want.References) {
		t.Errorf("resolved edges %+v %+v %+v %+v, want %+v %+v %+v %+v", r.Founds, r.Scorable, r.Demoted, r.References,
			want.Founds, want.Scorable, want.Demoted, want.References)
	}
	if len(rows.Degraded) != 5 {
		t.Errorf("warnings %+v, want one per record left out, 5", rows.Degraded)
	}
	for id, want := range map[string]string{"clm-aaaaaa": "0.30", "clm-bbbbbb": "0.15"} {
		if got := FormatStrengthDisplay(nodeRow(t, rows, id).Strength); got != want {
			t.Errorf("%s strength %s, want %s from the records kept", id, got, want)
		}
	}
}

// TestDeriveDropsADanglingEdge: an edge record of any relation with an end no
// node record carries is dropped from the walk, with a warning naming the
// missing ids.
func TestDeriveDropsADanglingEdge(t *testing.T) {
	one := kb.Bearing{Value: 1}
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f(1)}},
		Works:  []kb.WorkNode{{ID: "wrk-key2020", Title: "W", Rigor: f(0.5)}},
		Founds: []kb.BearingEdge{{Source: "wrk-key2020", Target: "clm-aaaaaa", Bearing: one},
			{Source: "wrk-gone", Target: "clm-aaaaaa", Bearing: one}},
		Scorable:   []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-zzzzzz", Bearing: one}},
		References: []kb.UnscoredEdge{{Source: "clm-zzzzzz", Target: "clm-yyyyyy"}},
		Demoted:    []kb.DemotedEdge{{Source: "clm-yyyyyy", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationSupports, Bearing: one}},
	}
	rows, err := Derive(graphKB(t, g, map[string]string{kb.EntryPointFile: "# KB\n"}))
	if err != nil {
		t.Fatal(err)
	}
	r := rows.Resolved
	if !slices.Equal(r.Founds, g.Founds[:1]) || len(r.Scorable)+len(r.References)+len(r.Demoted) != 0 {
		t.Errorf("resolved edges %+v %+v %+v %+v, want the one founds record naming two nodes", r.Founds, r.Scorable, r.References, r.Demoted)
	}
	var missing []string
	for _, w := range rows.Degraded {
		missing = append(missing, w.Detail)
	}
	for _, want := range []string{"wrk-gone", "clm-zzzzzz", "clm-zzzzzz or clm-yyyyyy", "clm-yyyyyy"} {
		if !slices.ContainsFunc(missing, func(d string) bool { return strings.HasSuffix(d, "no record carries "+want) }) {
			t.Errorf("no warning names %s as missing; the warnings are %q", want, missing)
		}
	}
	if len(rows.Degraded) != 4 {
		t.Errorf("%d warnings, want one per record dropped, 4", len(rows.Degraded))
	}
	if got := FormatStrengthDisplay(nodeRow(t, rows, "clm-aaaaaa").Strength); got != "0.50" {
		t.Errorf("clm-aaaaaa strength %s, want 0.50", got)
	}
}

// TestDeriveBreaksACycleForTheWalk: a supports cycle is cut by BreakCycles,
// the cut kept as a demoted record, and both claims derive a strength.
func TestDeriveBreaksACycleForTheWalk(t *testing.T) {
	one := kb.Bearing{Value: 1}
	back := kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Context: "back", Bearing: kb.Bearing{Value: 0.5}}
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f(0.9)}, {ID: "clm-bbbbbb", Title: "B", Rigor: f(1)}},
		Works:  []kb.WorkNode{{ID: "wrk-key2020", Title: "W", Rigor: f(0.8)}},
		Founds: []kb.BearingEdge{{Source: "wrk-key2020", Target: "clm-aaaaaa", Bearing: one}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: one},
			back},
	}
	rows, err := Derive(graphKB(t, g, map[string]string{kb.EntryPointFile: "# KB\n"}))
	if err != nil {
		t.Fatal(err)
	}
	wantDemoted := []kb.DemotedEdge{{Source: back.Source, Target: back.Target, Context: back.Context,
		ReplacedRelation: kb.RelationSupports, Bearing: back.Bearing}}
	if r := rows.Resolved; !slices.Equal(r.Scorable, g.Scorable[:1]) || !slices.Equal(r.Demoted, wantDemoted) {
		t.Errorf("resolved %+v and %+v, want %+v and %+v", r.Scorable, r.Demoted, g.Scorable[:1], wantDemoted)
	}
	if len(rows.Degraded) != 1 {
		t.Errorf("warnings %+v, want the one cut", rows.Degraded)
	}
	for _, id := range []string{"clm-aaaaaa", "clm-bbbbbb"} {
		if got := FormatStrengthDisplay(nodeRow(t, rows, id).Strength); got != "0.80" {
			t.Errorf("%s strength %s, want 0.80", id, got)
		}
	}
}

// TestDeriveCutsInTheLeafsOrder: over the cycle clm-aaaaaa ⇄ clm-bbbbbb, whose
// leaf marks clm-bbbbbb first, the walk starts at clm-bbbbbb and cuts the edge
// closing the cycle back into it, clm-aaaaaa → clm-bbbbbb, where id order would
// cut the other.
func TestDeriveCutsInTheLeafsOrder(t *testing.T) {
	one := kb.Bearing{Value: 1}
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A"}, {ID: "clm-bbbbbb", Title: "B"}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: one},
			{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: one}},
	}
	rows, err := Derive(graphKB(t, g, map[string]string{
		kb.EntryPointFile: "# KB\n\n- [Leaf](leaf.md)\n",
		"leaf.md":         "[↑ KB](entry-point.md)\n\n# Leaf\n\nB. <!-- claim-rigor: clm-bbbbbb -->\n\nA. <!-- claim-rigor: clm-aaaaaa -->\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := []kb.DemotedEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports, Bearing: one}}
	if r := rows.Resolved; !slices.Equal(r.Demoted, want) || !slices.Equal(r.Scorable, g.Scorable[1:]) {
		t.Errorf("resolved %+v and %+v, want %+v and %+v", r.Scorable, r.Demoted, g.Scorable[1:], want)
	}
}

// TestDeriveRefusesWhatTheLoadRefuses: a KB missing its node file is the
// strict load's refusal, naming the file.
func TestDeriveRefusesWhatTheLoadRefuses(t *testing.T) {
	root := filepath.Join(t.TempDir(), kb.KBDir)
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Derive(kb.MigratedSource(root, kb.FormatVersion, map[string][]byte{
		kb.KBDir + "/" + kb.EntryPointFile: []byte("# KB\n"),
		kb.KBDir + "/" + kb.GraphEdgesFile: kb.EncodeEdges(kb.Graph{}),
	}, nil))
	var refused result.Refusal
	if !errors.As(err, &refused) || len(refused) != 1 || refused[0].Check != kb.CheckIndex || refused[0].Path != kb.GraphNodesFile {
		t.Errorf("Derive = %v, want the index refusal naming %s", err, kb.GraphNodesFile)
	}
}
