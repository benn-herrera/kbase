package index

import (
	"errors"
	"maps"
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
// anchored at the axioms.md heading carrying its marker — after an unmarked
// heading of the same text, so at its -1 suffix — and an axiom with no marked
// heading has no anchor. Each node a page renders has its entry's anchor
// there, an analysis's and an experiment's from its own title.
func TestDerivePlacesEveryKind(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "In a volume"}, {ID: "clm-bbbbbb", Title: "In a directory no volume is"},
			{ID: "clm-cccccc", Title: "At the root"}, {ID: "clm-dddddd", Title: "Unmarked"}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "A method"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "A run", Status: "run"}, {ID: "exp-bbbbbb", Title: "Unmarked", Status: kb.ExperimentPlanned}},
		Works:       []kb.WorkNode{{ID: "wrk-k00000", Label: "key2020", Title: "Key, A. 2020. *Book*."}},
		Axioms:      []kb.AxiomNode{{ID: "axm-a10000", Label: "A1", Title: "A rule"}, {ID: "axm-a20000", Label: "A2", Title: "No heading"}},
	}
	rows, err := Derive(graphKB(t, g, map[string]string{
		kb.AxiomsFile: "# Axioms\n\n### Axiom A1: A rule\n\nAn unmarked heading of the same text.\n\n" +
			kb.AxiomHeadingLine("A1", "A rule", "axm-a10000") + "\n",
		kb.EntryPointFile: "# KB\n\n- [v](v/index.md)\n",
		"v/index.md":      "[↑ KB](../entry-point.md)\n\n# V\n\n- [leaf](leaf.md)\n",
		"v/leaf.md": "[↑ V](index.md)\n\n# Leaf\n\nA claim. <!-- kbase-id: clm-aaaaaa -->\n\n" +
			"## Method\n<!-- kbase-id: ana-aaaaaa -->\n\n### Run\n\n```\n# not a heading\n```\n<!-- kbase-id: exp-aaaaaa -->\n",
		"loose/x.md": "# X\n\nSays so. <!-- kbase-id: clm-bbbbbb -->\n",
		"top.md":     "# Top\n\nSays so. <!-- kbase-id: clm-cccccc -->\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, path, anchor, page, entry string }{
		{"clm-aaaaaa", "v/kb-summary.md", "in-a-volume", "v/kb-summary.md", "in-a-volume"},
		{"ana-aaaaaa", "v/leaf.md", "method", "v/kb-summary.md", "a-method"},
		{"exp-aaaaaa", "v/leaf.md", "run", "v/kb-summary.md", "a-run"},
		{"clm-bbbbbb", "kb-summary.md", "in-a-directory-no-volume-is", "kb-summary.md", "in-a-directory-no-volume-is"},
		{"clm-cccccc", "kb-summary.md", "at-the-root", "kb-summary.md", "at-the-root"},
		{"clm-dddddd", "", "", "", ""},
		{"exp-bbbbbb", "", "", "", ""},
		{"wrk-k00000", "kb-summary.md", "key-a-2020-book", "kb-summary.md", "key-a-2020-book"},
		{"axm-a10000", "axioms.md", "axiom-a1-a-rule-1", "", ""},
		{"axm-a20000", "axioms.md", "", "", ""},
	} {
		r := nodeRow(t, rows, tc.id)
		if r.CanonicalPath != tc.path || r.CanonicalAnchor != tc.anchor {
			t.Errorf("%s: canonical %q#%q, want %q#%q", tc.id, r.CanonicalPath, r.CanonicalAnchor, tc.path, tc.anchor)
		}
		if page, ok := rows.RegisterPages[tc.id]; page != tc.page || ok != (tc.page != "") {
			t.Errorf("%s: register page %q (placed %t), want %q", tc.id, page, ok, tc.page)
		}
		if entry, ok := rows.RegisterAnchors[tc.id]; entry != tc.entry || ok != (tc.entry != "") {
			t.Errorf("%s: register entry anchor %q (placed %t), want %q", tc.id, entry, ok, tc.entry)
		}
	}
	wantHostAnchors := map[string]string{"clm-aaaaaa": "leaf", "ana-aaaaaa": "method", "exp-aaaaaa": "run", "clm-bbbbbb": "x", "clm-cccccc": "top"}
	if !maps.Equal(rows.HostAnchors, wantHostAnchors) {
		t.Errorf("host anchors %v, want %v", rows.HostAnchors, wantHostAnchors)
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
	for _, tc := range []struct{ id, path, anchor, strength string }{
		{"clm-aaa001", "alpha/kb-summary.md", "theorem-1", "0.90"},
		{"clm-aaa002", "alpha/kb-summary.md", "equation-eqflow--flow-dynamics", "0.95"},
		{"clm-aaa003", "alpha/kb-summary.md", "the-gradient-nabla-f-is-bounded-when-a--b--c", "0.60"},
		{"clm-aaa004", "alpha/kb-summary.md",
			"a-deliberately-long-prose-claim-title-that-runs-well-past-the-three-wrapped-lines-a-sheet-label-allows-before-it-is-cut", "0.40"},
		{"clm-aaa005", "", "", "0.00"},
		{"clm-aaa006", "", "", "0.00"},
		{"clm-bbb001", "", "", "0.88"},
		{"clm-bbb002", "beta/kb-summary.md", "the-governance-bifurcation", "0.66"},
		{"clm-bbb003", "", "", kb.PendingLiteral},
		{"ana-aaa007", "alpha/analysis.md", "a-supporting-lemma", "0.80"},
		{"exp-aaa008", "alpha/experiments/flow.md", "flow-simulation", "0.80"},
		// The 3.0.0 axm-1, axm-S1 and wrk-smith2020 as the conversion mints them.
		{"axm-kbrt32", "axioms.md", "axiom-1-agents-act-on-what-they-observe", "1.00"},
		{"axm-7b65tg", "axioms.md", "axiom-s1-conservation-of-stock", "1.00"},
		{"wrk-omcjwo", "kb-summary.md", "smith-a-2020-markets-fixture-press", "0.90"},
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
}

// TestDeriveRefusesTheIntegrityFaults: one edge identity on two records, an
// edge naming a node no record carries, and a cycle among the scoring edges
// are each a load error — check metadata, the edge file, a detail naming the
// record and the remedy in git — and nothing else is refused.
func TestDeriveRefusesTheIntegrityFaults(t *testing.T) {
	one := kb.Bearing{Value: 1}
	claims := []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A"}, {ID: "clm-bbbbbb", Title: "B"}}
	ab := kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: one}
	for _, tc := range []struct {
		name   string
		g      kb.Graph
		detail string
	}{
		{"one identity on two records",
			kb.Graph{Claims: claims, Scorable: []kb.ScorableEdge{ab, {Relation: ab.Relation, Source: ab.Source, Target: ab.Target, Context: "again", Bearing: one}}},
			"the edge supports clm-aaaaaa → clm-bbbbbb is on more than one record"},
		{"one demoted identity on two records",
			kb.Graph{Claims: claims, Demoted: []kb.DemotedEdge{
				{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports, Bearing: one},
				{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Pending: true}}}},
			"the edge demoted supports clm-aaaaaa → clm-bbbbbb is on more than one record"},
		{"an edge naming no node",
			kb.Graph{Claims: claims, Founds: []kb.BearingEdge{{Source: "wrk-g00000", Target: "clm-aaaaaa", Bearing: one}}},
			"the edge founds wrk-g00000 → clm-aaaaaa names wrk-g00000, which no node record carries"},
		{"an edge naming two missing nodes",
			kb.Graph{Claims: claims, References: []kb.UnscoredEdge{{Source: "clm-yyyyyy", Target: "clm-zzzzzz"}}},
			"the edge references clm-yyyyyy → clm-zzzzzz names clm-yyyyyy and clm-zzzzzz, which no node record carries"},
		{"a cycle among the scoring edges",
			kb.Graph{Claims: claims, Scorable: []kb.ScorableEdge{ab, {Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: one}}},
			"closes a cycle among the scoring edges"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Derive(graphKB(t, tc.g, map[string]string{kb.EntryPointFile: "# KB\n"}))
			var refused result.Refusal
			if !errors.As(err, &refused) || len(refused) != 1 {
				t.Fatalf("Derive = %v, want one refusal item", err)
			}
			if it := refused[0]; it.Check != kb.CheckMetadata || it.Path != kb.GraphEdgesFile || it.Remedy != kb.ConsistentRecordsRemedy ||
				!strings.Contains(it.Detail, tc.detail) {
				t.Errorf("refusal %+v, want check %s, path %s, remedy %q and a detail holding %q",
					it, kb.CheckMetadata, kb.GraphEdgesFile, kb.ConsistentRecordsRemedy, tc.detail)
			}
		})
	}
	// A demoted record beside the scorable record it replaced is two
	// identities, and no fault.
	g := kb.Graph{Claims: claims, Scorable: []kb.ScorableEdge{ab},
		Demoted: []kb.DemotedEdge{{Source: ab.Source, Target: ab.Target, ReplacedRelation: kb.RelationSupports, Bearing: one}}}
	if _, err := Derive(graphKB(t, g, map[string]string{kb.EntryPointFile: "# KB\n"})); err != nil {
		t.Errorf("Derive over a scorable and a demoted record of one pair = %v, want rows", err)
	}
}

// TestDeriveOverCyclesBreaksACycleForTheWalk: over the build's records, a
// supports cycle is cut by BreakCycles, the cut kept as a demoted record, and
// both claims derive a strength.
func TestDeriveOverCyclesBreaksACycleForTheWalk(t *testing.T) {
	one := kb.Bearing{Value: 1}
	back := kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Context: "back", Bearing: kb.Bearing{Value: 0.5}}
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A", Rigor: f(0.9)}, {ID: "clm-bbbbbb", Title: "B", Rigor: f(1)}},
		Works:  []kb.WorkNode{{ID: "wrk-k00000", Label: "key2020", Title: "W", Rigor: f(0.8)}},
		Founds: []kb.BearingEdge{{Source: "wrk-k00000", Target: "clm-aaaaaa", Bearing: one}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: one},
			back},
	}
	rows, err := DeriveOverCycles(graphKB(t, g, map[string]string{kb.EntryPointFile: "# KB\n"}))
	if err != nil {
		t.Fatal(err)
	}
	wantDemoted := []kb.DemotedEdge{{Source: back.Source, Target: back.Target, Context: back.Context,
		ReplacedRelation: kb.RelationSupports, Bearing: back.Bearing}}
	if r := rows.Resolved; !slices.Equal(r.Scorable, g.Scorable[:1]) || !slices.Equal(r.Demoted, wantDemoted) {
		t.Errorf("resolved %+v and %+v, want %+v and %+v", r.Scorable, r.Demoted, g.Scorable[:1], wantDemoted)
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
	rows, err := DeriveOverCycles(graphKB(t, g, map[string]string{
		kb.EntryPointFile: "# KB\n\n- [Leaf](leaf.md)\n",
		"leaf.md":         "[↑ KB](entry-point.md)\n\n# Leaf\n\nB. <!-- kbase-id: clm-bbbbbb -->\n\nA. <!-- kbase-id: clm-aaaaaa -->\n",
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
