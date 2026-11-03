package index

import (
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// TestFormatStrength: the record's spelling reads back as the walk's own
// value, four decimals at most where that suffices and as many as the value
// needs; the display's is two decimals.
func TestFormatStrength(t *testing.T) {
	// A product one ulp off 0.21: its own decimal spelling carries the noise,
	// and four decimals hold it exactly.
	noisy := 0.3 * 0.7
	for _, tc := range []struct {
		in              *float64
		record, display string
	}{
		{nil, kb.PendingLiteral, kb.PendingLiteral},
		{f(0.7), "0.7", "0.70"},
		{f(0.504), "0.504", "0.50"},
		{f(0.99995), "0.99995", "1.00"},
		{f(0.36288), "0.36288", "0.36"},
		{f(noisy), "0.21", "0.21"},
		{f(0), "0", "0.00"},
		{f(1), "1", "1.00"},
	} {
		if got := FormatStrengthRecord(tc.in); got != tc.record {
			t.Errorf("FormatStrengthRecord(%v) = %q, want %q", tc.in, got, tc.record)
		}
		if tc.in != nil {
			if back, err := strconv.ParseFloat(tc.record, 64); err != nil || math.Abs(back-*tc.in) >= strengthTolerance {
				t.Errorf("the record %q does not read back as %v", tc.record, *tc.in)
			}
		}
		if got := FormatStrengthDisplay(tc.in); got != tc.display {
			t.Errorf("FormatStrengthDisplay(%v) = %q, want %q", tc.in, got, tc.display)
		}
	}
}

func f(v float64) *float64 { return &v }

// strengthOf is the walk's strength over g's records.
func strengthOf(g kb.Graph) Strength { return graphStrength(g, graphEdges(g, GraphKinds(g))) }

// scorable is one supports, lifts or refutes record of a judged bearing.
func scorable(relation, source, target string, bearing float64) kb.ScorableEdge {
	return kb.ScorableEdge{Relation: relation, Source: source, Target: target, Bearing: kb.Bearing{Value: bearing}}
}

func founds(source, target string, bearing kb.Bearing) kb.BearingEdge {
	return kb.BearingEdge{Source: source, Target: target, Bearing: bearing}
}

func claim(id string, rigor *float64) kb.ClaimNode { return kb.ClaimNode{ID: id, Rigor: rigor} }

func TestGraphStrength(t *testing.T) {
	one := kb.Bearing{Value: 1}
	g := kb.Graph{
		Claims: []kb.ClaimNode{
			claim("clm-base01", f(0.9)), claim("clm-bare01", f(0.95)), claim("clm-bare02", f(0.95)),
			claim("clm-weak01", f(0.4)), claim("clm-chain1", f(0.8)), claim("clm-pend01", nil),
			claim("clm-excl01", f(0.9)), claim("clm-far001", f(1)), claim("clm-lift01", f(0.2)),
			claim("clm-low001", f(0.9)), claim("clm-rescu1", nil), claim("clm-cap001", f(0.9)),
			claim("clm-cap002", nil), claim("clm-both01", f(0.9)), claim("clm-half01", f(0.9)),
			claim("clm-axm04", f(0.9)), claim("clm-axm00", f(0.9)), claim("clm-work00", f(0.9)),
			claim("clm-work01", f(0.9)), claim("clm-workpd", f(0.9)), claim("clm-workns", f(0.9)),
			claim("clm-targ01", f(0.8)), claim("clm-refut1", f(0.9)), claim("clm-refr01", f(0.3)),
		},
		Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "I"}},
		Analyses: []kb.AnalysisNode{
			{ID: "ana-lift01", Rigor: f(0.9)}, {ID: "ana-lift02", Rigor: f(1)}, {ID: "ana-ref01", Rigor: f(1)},
			{ID: "ana-both1", Rigor: f(1)}, {ID: "ana-ref02", Rigor: f(1)}, {ID: "ana-lift03", Rigor: f(1)},
			{ID: "ana-ref03", Rigor: f(1)}, {ID: "ana-min01", Rigor: f(0.5)}, {ID: "ana-cap01", Rigor: f(0.9)},
		},
		Experiments: []kb.ExperimentNode{
			{ID: "exp-run001", Status: "run", Rigor: f(1)}, {ID: "exp-pend01", Status: kb.ExperimentPlanned, Rigor: f(1)},
			{ID: "exp-boost1", Status: "run", Rigor: f(0.9)}, {ID: "exp-skept1", Status: "run", Rigor: f(1)},
		},
		Works: []kb.WorkNode{{ID: "wrk-a00000", Label: "a", Rigor: f(0.5)}, {ID: "wrk-b00000", Label: "b"}, {ID: "wrk-c00000", Label: "c", Rigor: f(0.9)}},
		Founds: []kb.BearingEdge{
			founds("wrk-c00000", "clm-base01", one), founds("wrk-a00000", "clm-bare02", kb.Bearing{Value: 0.9}),
			founds("wrk-c00000", "clm-cap001", one), founds("axm-s10000", "clm-axm04", kb.Bearing{Value: 0.4}),
			founds("axm-s10000", "clm-axm00", kb.Bearing{Value: 0}), founds("wrk-a00000", "clm-work00", kb.Bearing{Value: 0}),
			founds("wrk-a00000", "clm-work01", kb.Bearing{Value: 0.3}), founds("wrk-a00000", "clm-workpd", kb.Bearing{Pending: true}),
			founds("wrk-b00000", "clm-workns", one), founds("wrk-c00000", "clm-targ01", one),
			founds("wrk-a00000", "clm-refr01", kb.Bearing{Value: 0.6}),
			founds("axm-s10000", "ana-lift01", one), founds("axm-s10000", "ana-lift02", one), founds("axm-s10000", "ana-ref01", one),
			founds("axm-s10000", "ana-both1", one), founds("axm-s10000", "ana-ref02", one), founds("axm-s10000", "ana-lift03", one),
			founds("axm-s10000", "ana-ref03", one), founds("axm-s10000", "ana-cap01", one),
		},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-base01", "clm-weak01", 1),
			scorable(kb.RelationSupports, "clm-weak01", "clm-chain1", 1),
			scorable(kb.RelationSupports, "clm-base01", "clm-pend01", 1),
			scorable(kb.RelationSupports, "clm-pend01", "clm-excl01", 1),
			scorable(kb.RelationSupports, "clm-excl01", "clm-far001", 1),
			scorable(kb.RelationSupports, "clm-base01", "clm-both01", 0.6667),
			scorable(kb.RelationSupports, "clm-base01", "clm-half01", 0.5),
			scorable(kb.RelationRefutes, "clm-refr01", "clm-targ01", 1),
			scorable(kb.RelationRefutes, "clm-refr01", "clm-refut1", 1),
			scorable(kb.RelationLifts, "ana-lift01", "clm-lift01", 0.98),
			scorable(kb.RelationLifts, "ana-lift02", "clm-low001", 0.8),
			scorable(kb.RelationRefutes, "ana-ref01", "clm-cap001", 0.2),
			scorable(kb.RelationLifts, "ana-both1", "clm-both01", 0.8),
			scorable(kb.RelationRefutes, "ana-ref02", "clm-both01", 0.3),
			scorable(kb.RelationLifts, "ana-lift03", "clm-cap002", 0.9),
			scorable(kb.RelationRefutes, "ana-ref03", "clm-cap002", 0.2),
			scorable(kb.RelationLifts, "exp-run001", "clm-rescu1", 0.7),
			scorable(kb.RelationLifts, "exp-pend01", "clm-pend01", 1),
			scorable(kb.RelationLifts, "exp-boost1", "ana-min01", 1),
			scorable(kb.RelationRefutes, "exp-skept1", "ana-cap01", 0.25),
		},
	}
	strength := strengthOf(g)
	if len(strength.Results) != len(g.Claims) {
		t.Errorf("Results carries %d entries, want one per claim (%d)", len(strength.Results), len(g.Claims))
	}
	if got, want := len(strength.AnalysisResults), len(g.Analyses); got != want {
		t.Errorf("AnalysisResults carries %d entries, want one per analysis (%d)", got, want)
	}
	finals := strength.finals()
	for _, tc := range []struct {
		id   string
		want *float64
	}{
		{"clm-base01", f(0.9)},
		{"clm-bare01", f(0)},
		{"clm-bare02", f(0.45)},
		{"clm-weak01", f(0.4)},
		{"clm-chain1", f(0.4)},
		{"clm-pend01", nil},
		{"clm-excl01", f(0)},
		{"clm-far001", f(0)},
		{"clm-lift01", f(0.882)},
		{"clm-low001", f(0.8)},
		{"clm-rescu1", f(0.7)},
		{"clm-cap001", f(0.8)},
		{"clm-cap002", f(0.8)},
		{"clm-both01", f(0.7)},
		{"clm-half01", f(0.45)},
		{"clm-axm04", f(0.4)},
		{"clm-axm00", f(0)},
		{"clm-work00", f(0)},
		{"clm-work01", f(0.15)},
		{"clm-workpd", f(0)},
		{"clm-workns", f(0)},
		{"clm-targ01", f(0.7)},
		{"clm-refut1", f(0)},
	} {
		got, ok := finals[tc.id]
		if tc.want == nil && ok || tc.want != nil && (!ok || got != *tc.want) {
			t.Errorf("%s: final %v (set %v), want %v", tc.id, got, ok, tc.want)
		}
	}
	for _, tc := range []struct{ id, want string }{
		{"clm-base01", " [= min(0.90, 0.90)]"},
		{"clm-bare01", ""},
		{"clm-bare02", " [= min(0.95, 0.45)]"},
		{"clm-weak01", " [= min(0.40, 0.90)]"},
		{"clm-chain1", " [= min(0.80, 0.40)]"},
		{"clm-excl01", ""},
		{"clm-far001", " [= min(1.00, 0.00)]"},
		{"clm-lift01", " [= max(0.00, 0.88)]"},
		{"clm-low001", " [= max(0.00, 0.80)]"},
		{"clm-rescu1", " [= lift 0.70]"},
		{"clm-cap001", " [= min(min(0.90, 0.90), 0.80)]"},
		{"clm-cap002", " [= min(lift 0.90, 0.80)]"},
		{"clm-both01", " [= min(max(min(0.90, 0.60), 0.80), 0.70)]"},
		{"clm-axm04", " [= min(0.90, 0.40)]"},
		{"clm-axm00", " [= min(0.90, 0.00)]"},
		{"clm-half01", " [= min(0.90, 0.45)]"},
		{"clm-workpd", ""},
		{"clm-workns", ""},
		{"clm-targ01", " [= min(min(0.80, 0.90), 0.70)]"},
		{"clm-refut1", ""},
		{"clm-pend01", ""},
	} {
		if got := RenderStrengthTrace(strength.Results[tc.id]); got != tc.want {
			t.Errorf("%s trace = %q, want %q", tc.id, got, tc.want)
		}
	}
	for _, tc := range []struct {
		id    string
		final *float64
		trace string
	}{{"ana-min01", f(0.9), " [= max(0.50, 0.90)]"}, {"ana-cap01", f(0.75), " [= min(min(0.90, 1.00), 0.75)]"}} {
		r := strength.AnalysisResults[tc.id]
		if r.Final == nil || *r.Final != *tc.final {
			t.Errorf("%s final = %v, want %v", tc.id, r.Final, *tc.final)
		}
		if got := RenderStrengthTrace(r); got != tc.trace {
			t.Errorf("%s trace = %q, want %q", tc.id, got, tc.trace)
		}
	}
}

// TestAPremiseLessAnalysisDerivesItsRigor: a node nothing carries derives what
// its kind is worth bare — an analysis its own rigor, its method being the
// evidence its premises would otherwise supply, and a claim 0, since nothing
// substantiates it. An analysis whose premise participates is unchanged, so one
// resting on an unsubstantiated claim still derives that claim's zero, and a
// claim lifted by a premise-less analysis reads the analysis's strength.
func TestAPremiseLessAnalysisDerivesItsRigor(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{
			{ID: "clm-bare01", Rigor: f(0.8)},
			{ID: "clm-split1", Rigor: f(0.8)},
			{ID: "clm-split9", Rigor: f(0.8)},
		},
		Analyses: []kb.AnalysisNode{
			{ID: "ana-bare01", Rigor: f(0.7)},
			{ID: "ana-zero01", Rigor: f(0.9)},
		},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-bare01", "ana-zero01", 1),
			scorable(kb.RelationLifts, "ana-bare01", "clm-split1", 1),
			scorable(kb.RelationLifts, "ana-zero01", "clm-split9", 1),
		},
	}
	strength := strengthOf(g)
	for _, tc := range []struct {
		name string
		got  *float64
		want *float64
	}{
		{"a claim nothing carries", strength.Results["clm-bare01"].Final, f(0)},
		{"an analysis nothing carries", strength.AnalysisResults["ana-bare01"].Final, f(0.7)},
		{"an analysis resting on an unsubstantiated claim", strength.AnalysisResults["ana-zero01"].Final, f(0)},
		{"a claim lifting off a premise-less analysis", strength.Results["clm-split1"].Final, f(0.7)},
		{"a claim lifting off a zeroed analysis", strength.Results["clm-split9"].Final, f(0)},
	} {
		if tc.got == nil || *tc.got != *tc.want {
			t.Errorf("%s: final %v, want %v", tc.name, tc.got, *tc.want)
		}
	}
}

// TestWorkAndAxiomFeedTheirRigorAndDeriveNoStrength: a node with no inputs has
// strength = rigor, so a work and an axiom each feed bearing × rigor into a
// dependent's min — a work's rigor judged once by a person, an axiom's 1.0 by
// rule — and neither derives a strength of its own, in the walk or in its row.
func TestWorkAndAxiomFeedTheirRigorAndDeriveNoStrength(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-work01", Title: "W", Rigor: f(0.9)}, {ID: "clm-axm001", Title: "X", Rigor: f(0.9)}},
		Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "S1"}},
		Works:  []kb.WorkNode{{ID: "wrk-a00000", Label: "a", Title: "A", Rigor: f(0.6)}},
		Founds: []kb.BearingEdge{founds("wrk-a00000", "clm-work01", kb.Bearing{Value: 0.5}), founds("axm-s10000", "clm-axm001", kb.Bearing{Value: 0.5})},
	}
	strength := strengthOf(g)
	for _, tc := range []struct {
		id   string
		want float64
	}{
		{"clm-work01", 0.5 * 0.6},
		{"clm-axm001", 0.5 * kb.AxiomRigor},
	} {
		r := strength.Results[tc.id]
		if r.MinDep == nil || *r.MinDep != tc.want || r.Final == nil || *r.Final != tc.want {
			t.Errorf("%s: min %v, final %v, want %v from the source's rigor", tc.id, r.MinDep, r.Final, tc.want)
		}
	}
	rows, err := Derive(graphKB(t, g, nil))
	if err != nil {
		t.Fatal(err)
	}
	finals := strength.finals()
	for id, rigor := range map[string]float64{"wrk-a00000": 0.6, "axm-s10000": kb.AxiomRigor} {
		row := nodeRow(t, rows, id)
		if row.Rigor == nil || *row.Rigor != rigor {
			t.Errorf("%s row rigor = %v, want %v", id, row.Rigor, rigor)
		}
		if row.Strength != nil || row.DerivationStrength != nil {
			t.Errorf("%s row carries a derived strength: %+v", id, row)
		}
		if _, derived := finals[id]; derived {
			t.Errorf("%s has a derived strength", id)
		}
	}
}

// TestDependentUsesItsSourcesExactStrength: a premise multiplies the value its
// source published, so 0.70 × 0.72 = 0.504 enters a dependent's min as 0.504
// and not as the 0.5 a two-decimal walk would have published.
func TestDependentUsesItsSourcesExactStrength(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{claim("clm-aaaaaa", f(0.7)), claim("clm-bbbbbb", f(0.9)), claim("clm-cccccc", f(0.9))},
		Works:  []kb.WorkNode{{ID: "wrk-a00000", Label: "a", Rigor: f(0.7)}},
		Founds: []kb.BearingEdge{founds("wrk-a00000", "clm-aaaaaa", kb.Bearing{Value: 1})},
		Scorable: []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-aaaaaa", "clm-bbbbbb", 0.72),
			scorable(kb.RelationSupports, "clm-bbbbbb", "clm-cccccc", 0.72)},
	}
	strength := strengthOf(g)
	first := 0.72 * 0.7
	second := 0.72 * first
	finals := strength.finals()
	for _, tc := range []struct {
		id     string
		want   float64
		record string
	}{
		{"clm-aaaaaa", 0.7, "0.7"},
		{"clm-bbbbbb", first, "0.504"},
		{"clm-cccccc", second, "0.36288"},
	} {
		got, ok := finals[tc.id]
		if !ok {
			t.Fatalf("%s published no strength", tc.id)
		}
		if got != tc.want {
			t.Errorf("%s strength = %v, want %v", tc.id, got, tc.want)
		}
		if spelled := FormatStrengthRecord(&got); spelled != tc.record {
			t.Errorf("%s's record spells %q, want %q", tc.id, spelled, tc.record)
		}
	}
}

// TestClaimRefuterAndPairsIntoAnalysesReachTheRecords derives a KB whose claim
// refutes a claim and whose run experiments lift and refute analyses, and reads
// the numbers and the edge rows back off what the derivation publishes. The
// refuting experiment authors 1.0, the rigor tables' maximum, so its 0.25
// bearing caps the analysis at 1 − 0.25 × 1.0.
func TestClaimRefuterAndPairsIntoAnalysesReachTheRecords(t *testing.T) {
	one := kb.Bearing{Value: 1}
	g := kb.Graph{
		Claims:   []kb.ClaimNode{{ID: "clm-targ01", Title: "Claim T", Rigor: f(0.8)}, {ID: "clm-refr01", Title: "Claim R", Rigor: f(0.3)}},
		Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "Ana A", Rigor: f(0.5)}, {ID: "ana-bbbbbb", Title: "Ana B", Rigor: f(0.9)}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Ex A", Status: "run", Rigor: f(0.9)},
			{ID: "exp-bbbbbb", Title: "Ex B", Status: "run", Rigor: f(1)}},
		Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "The rule"}},
		Founds: []kb.BearingEdge{founds("axm-s10000", "clm-targ01", one), founds("axm-s10000", "clm-refr01", one), founds("axm-s10000", "ana-bbbbbb", one)},
		Scorable: []kb.ScorableEdge{scorable(kb.RelationRefutes, "clm-refr01", "clm-targ01", 1),
			scorable(kb.RelationLifts, "exp-aaaaaa", "ana-aaaaaa", 1), scorable(kb.RelationRefutes, "exp-bbbbbb", "ana-bbbbbb", 0.25)},
	}
	rows, err := Derive(graphKB(t, g, nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		want float64
	}{
		{"clm-targ01", 0.7},
		{"ana-aaaaaa", 0.9},
		{"ana-bbbbbb", 0.75},
	} {
		if got := nodeRow(t, rows, tc.id).Strength; got == nil || *got != tc.want {
			t.Errorf("%s strength = %v, want %v", tc.id, got, tc.want)
		}
	}
	edges := map[string]bool{}
	for _, e := range rows.Edges {
		edges[e.Source+" "+e.Target+" "+e.Relation+" "+e.TargetKind+" "+fmt.Sprint(e.Bearing)] = true
	}
	for _, want := range []string{
		"clm-refr01 clm-targ01 refutes claim 1.0",
		"exp-aaaaaa ana-aaaaaa lifts analysis 1.0",
		"exp-bbbbbb ana-bbbbbb refutes analysis 0.25",
	} {
		if !edges[want] {
			t.Errorf("the edge rows lack %q; they carry %v", want, edges)
		}
	}
}

// TestALoadDerivesPastAPremiseCycle: a walk meeting a cycle refuses nothing;
// the claim off the cycle derives, and the cycle's members derive nothing.
func TestALoadDerivesPastAPremiseCycle(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{claim("clm-aaaaaa", nil), claim("clm-bbbbbb", nil), claim("clm-cccccc", f(0.5))},
		Scorable: []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", 1),
			scorable(kb.RelationSupports, "clm-aaaaaa", "clm-bbbbbb", 1)},
	}
	strength := strengthOf(g)
	if v, ok := strength.finals()["clm-cccccc"]; !ok || v != 0 {
		t.Errorf("clm-cccccc final %v (derived %t), want 0", v, ok)
	}
	for _, id := range []string{"clm-aaaaaa", "clm-bbbbbb"} {
		if _, ok := strength.Results[id]; ok {
			t.Errorf("%s derived a result on the cycle", id)
		}
	}
}

func TestRenderLeafReferences(t *testing.T) {
	for _, tc := range []struct {
		register string
		leaves   []string
		want     string
	}{
		{"claim-rigor.md", []string{"common/a.md", "b.md"}, "> **Leaf references:** [a](./common/a.md), [b](./b.md)."},
		{"vol/claim-rigor.md", []string{"vol/x/a.md", "common/b.md"}, "> **Leaf references:** [a](./x/a.md), [b](../common/b.md)."},
		{"vol/claim-rigor.md", nil, kb.LeafReferencesPendingFooter},
	} {
		if got := RenderLeafReferences(tc.register, tc.leaves); got != tc.want {
			t.Errorf("RenderLeafReferences(%q, %q) = %q, want %q", tc.register, tc.leaves, got, tc.want)
		}
	}
}

// writeKB writes files under a fresh kb-root and returns it.
func writeKB(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
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
	return root
}

// twoVolumeKB is a KB of g's records and two volumes, alpha hosting
// clm-aa1111 and beta hosting clm-bb2222.
func twoVolumeKB(g kb.Graph) map[string]string {
	files := graphFiles(g)
	maps.Copy(files, map[string]string{
		kb.EntryPointFile: "# KB\n\n- [alpha](alpha/index.md)\n- [beta](beta/index.md)\n",
		"alpha/index.md":  "[↑ KB](../entry-point.md)\n\n# Alpha\n\n- [one](one.md)\n",
		"alpha/one.md":    "[↑ Alpha](index.md)\n\n# One\n\nOne. <!-- kbase-id: clm-aa1111 -->\n",
		"beta/index.md":   "[↑ KB](../entry-point.md)\n\n# Beta\n\n- [two](two.md)\n",
		"beta/two.md":     "[↑ Beta](index.md)\n\n# Two\n\nTwo. <!-- kbase-id: clm-bb2222 -->\n",
	})
	return files
}

// TestBareIDFindings: the id of a node of any kind the records carry,
// standing in prose, is a bare-id fault, link text being prose; one inside a
// fence, an inline code span, a statement marker, a citation's link title or a
// link's destination is not, nor a placeholder id, nor an id no record
// carries, nor one on a register page.
func TestBareIDFindings(t *testing.T) {
	g := kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aa1111", Title: "One"}},
		Works: []kb.WorkNode{{ID: "wrk-s00000", Label: "smith2020", Title: "Smith 2020"}}, Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "S1"}}}
	volumes := twoVolumeKB(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aa1111", Title: "One"}, {ID: "clm-bb2222", Title: "Two"}}})
	volumes["alpha/"+kb.RegisterFile] = "## One" + kb.StatementMarker("clm-aa1111") + "\n\n- rests-on:\n  - clm-bb2222 — Two\n"
	ep := kb.EntryPointFile
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"bare ids of every kind", map[string]string{ep: "The claim clm-aa1111 rests on wrk-s00000 and axm-s10000.\n"},
			[]string{"entry-point.md:1 clm-aa1111", "entry-point.md:1 wrk-s00000", "entry-point.md:1 axm-s10000"}},
		{"an id as a link's text", map[string]string{ep: "See [clm-aa1111](leaf.md).\n"}, []string{"entry-point.md:1 clm-aa1111"}},
		{"a fenced block", map[string]string{ep: "How:\n\n```\nclm-aa1111\n```\n"}, nil},
		{"an inline code span", map[string]string{ep: "Write `clm-aa1111` as a citation.\n"}, nil},
		{"a statement marker", map[string]string{"leaf.md": "# Leaf\n\nProse." + kb.StatementMarker("clm-aa1111") + "\n"}, nil},
		{"a citation's link title", map[string]string{ep: `See [One](leaf.md#leaf "clm-aa1111").` + "\n"}, nil},
		{"a link's destination", map[string]string{ep: "See [One](leaf.md#one-clm-aa1111).\n"}, nil},
		{"a placeholder id", map[string]string{ep: "An id such as clm-xxxxxx.\n"}, nil},
		{"an id no record carries", map[string]string{ep: "See clm-zzzzzz, wrk-zzzzzz and axm-zzzzzz.\n"}, nil},
		{"an id inside a word", map[string]string{ep: "preclm-aa1111 is no id.\n"}, nil},
		{"a register page", volumes, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := graphFiles(g)
			maps.Copy(files, tc.files)
			findings, err := BareIDFindings(kb.OnDisk(writeKB(t, files)))
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != len(tc.want) {
				t.Fatalf("findings %v, want one per %q", findings, tc.want)
			}
			for i, fd := range findings {
				site, id, _ := strings.Cut(tc.want[i], " ")
				if fd.Check != CheckBareID || fmt.Sprintf("%s:%d", fd.Path, fd.Line) != site || !strings.Contains(fd.Detail, id) {
					t.Errorf("finding %+v, want a %s fault at %s naming %s", fd, CheckBareID, site, id)
				}
			}
		})
	}
}

// TestVerifyRefusesABareUnknownIDOnce: a claim id no record carries standing
// in prose is one fault, unknown-id, and no bare-id.
func TestVerifyRefusesABareUnknownIDOnce(t *testing.T) {
	files := graphFiles(kb.Graph{})
	files[kb.EntryPointFile] = "See clm-zzzzzz.\n"
	findings, err := Verify(kb.OnDisk(writeKB(t, files)))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range findings {
		if f.Path == kb.EntryPointFile && f.Line == 1 {
			got = append(got, f.Check)
		}
	}
	if want := []string{kb.CheckUnknownID}; !slices.Equal(got, want) {
		t.Errorf("verify's faults at %s:1 = %q, want %q", kb.EntryPointFile, got, want)
	}
}
