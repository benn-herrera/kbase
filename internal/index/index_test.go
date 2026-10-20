package index

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/migrate"
	"kbase/internal/result"
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

func claimEntry(id string, rigor *float64, premises ...kb.Edge) kb.ClaimEntry {
	for i := range premises {
		premises[i].Target, premises[i].TargetKind = id, kb.NodeKindClaim
	}
	return kb.ClaimEntry{ID: id, Rigor: rigor, Premises: premises}
}

// supports is one premise bullet's edge: the premise is the source, the entry
// it is authored on the target.
func supports(source string, bearing float64) kb.Edge {
	return kb.Edge{Source: source, Relation: kb.RelationSupports, Bearing: kb.Bearing{Value: bearing}}
}

func founds(source string, bearing kb.Bearing) kb.Edge {
	return kb.Edge{Source: source, Relation: kb.RelationFounds, Bearing: bearing}
}

func TestComputeStrength(t *testing.T) {
	st := kb.State{
		ClaimEntries: []kb.ClaimEntry{
			claimEntry("clm-base01", f(0.9)),
			claimEntry("clm-weak01", f(0.4), supports("clm-base01", 1)),
			claimEntry("clm-chain1", f(0.8), supports("clm-weak01", 1)),
			claimEntry("clm-pend01", nil),
			claimEntry("clm-excl01", f(0.9), supports("clm-pend01", 1)),
			claimEntry("clm-far001", f(1), supports("clm-excl01", 1)),
			claimEntry("clm-lift01", f(0.2)),
			claimEntry("clm-low001", f(0.9)),
			claimEntry("clm-rescu1", nil),
			claimEntry("clm-cap001", f(0.9)),
			claimEntry("clm-cap002", nil),
			claimEntry("clm-both01", f(0.9), supports("clm-base01", 0.6667)),
			claimEntry("clm-half01", f(0.9), supports("clm-base01", 0.5)),
			claimEntry("clm-axm04", f(0.9), founds("axm-S1", kb.Bearing{Value: 0.4})),
			claimEntry("clm-axm00", f(0.9), founds("axm-S1", kb.Bearing{Value: 0})),
			claimEntry("clm-work00", f(0.9), founds("wrk-a", kb.Bearing{Value: 0})),
			claimEntry("clm-work01", f(0.9), founds("wrk-a", kb.Bearing{Value: 0.3})),
			claimEntry("clm-workpd", f(0.9), founds("wrk-a", kb.Bearing{Pending: true})),
			claimEntry("clm-workns", f(0.9), founds("wrk-b", kb.Bearing{Value: 1})),
			{ID: "clm-targ01", Rigor: f(0.8)},
			{ID: "clm-refr01", Rigor: f(0.3), Refutes: []kb.Edge{{Source: "clm-refr01", Target: "clm-targ01", Relation: kb.RelationRefutes,
				TargetKind: kb.NodeKindClaim, Bearing: kb.Bearing{Value: 1}}}},
		},
		Axioms: []kb.AxiomNode{{NodeType: kb.NodeKindAxiom, ID: "axm-S1", Title: "I", CanonicalPath: "axioms.md", CanonicalAnchor: "axiom-s1-the-rule"}},
		Analyses: []kb.AnalysisNode{
			{ID: "ana-lift01", Rigor: f(0.9), Lifts: []kb.Edge{{Source: "ana-lift01", Target: "clm-lift01", Relation: kb.RelationLifts, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.98}}}},
			{ID: "ana-lift02", Rigor: f(1), Lifts: []kb.Edge{{Source: "ana-lift02", Target: "clm-low001", Relation: kb.RelationLifts, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.8}}}},
			{ID: "ana-ref01", Rigor: f(1), Refutes: []kb.Edge{{Source: "ana-ref01", Target: "clm-cap001", Relation: kb.RelationRefutes, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.2}}}},
			{ID: "ana-both1", Rigor: f(1), Lifts: []kb.Edge{{Source: "ana-both1", Target: "clm-both01", Relation: kb.RelationLifts, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.8}}}},
			{ID: "ana-ref02", Rigor: f(1), Refutes: []kb.Edge{{Source: "ana-ref02", Target: "clm-both01", Relation: kb.RelationRefutes, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.3}}}},
			{ID: "ana-lift03", Rigor: f(1), Lifts: []kb.Edge{{Source: "ana-lift03", Target: "clm-cap002", Relation: kb.RelationLifts, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.9}}}},
			{ID: "ana-ref03", Rigor: f(1), Refutes: []kb.Edge{{Source: "ana-ref03", Target: "clm-cap002", Relation: kb.RelationRefutes, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.2}}}},
			{ID: "ana-min01", Rigor: f(0.5)},
			{ID: "ana-cap01", Rigor: f(0.9)},
		},
		Experiments: []kb.ExperimentNode{
			{ID: "exp-run001", Status: "run", Rigor: f(1), Lifts: []kb.Edge{
				{Source: "exp-run001", Target: "clm-rescu1", Relation: kb.RelationLifts, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.7}}}},
			{ID: "exp-pend01", Status: "pending", Rigor: f(1), Lifts: []kb.Edge{
				{Source: "exp-pend01", Target: "clm-pend01", Relation: kb.RelationLifts, TargetKind: "claim", Bearing: kb.Bearing{Value: 1}}}},
			{ID: "exp-boost1", Status: "run", Rigor: f(0.9), Lifts: []kb.Edge{
				{Source: "exp-boost1", Target: "ana-min01", Relation: kb.RelationLifts, TargetKind: kb.NodeKindAnalysis, Bearing: kb.Bearing{Value: 1}}}},
			{ID: "exp-skept1", Status: "run", Rigor: f(1), Refutes: []kb.Edge{
				{Source: "exp-skept1", Target: "ana-cap01", Relation: kb.RelationRefutes, TargetKind: kb.NodeKindAnalysis, Bearing: kb.Bearing{Value: 0.25}}}},
		},
		Works: []kb.ExternalWork{{ID: "wrk-a", Rigor: f(0.5)}, {ID: "wrk-b"}},
	}
	strength, err := ComputeStrength(st)
	if err != nil {
		t.Fatal(err)
	}
	if len(strength.Results) != len(st.ClaimEntries) {
		t.Errorf("Results carries %d entries, want one per claim (%d)", len(strength.Results), len(st.ClaimEntries))
	}
	if got, want := len(strength.AnalysisResults), len(st.Analyses); got != want {
		t.Errorf("AnalysisResults carries %d entries, want one per analysis (%d)", got, want)
	}
	finals := strength.Finals()
	for _, tc := range []struct {
		id   string
		want *float64
	}{
		{"clm-base01", f(0.9)},
		{"clm-weak01", f(0.4)},
		{"clm-chain1", f(0.4)},
		{"clm-pend01", nil},
		{"clm-excl01", f(0.9)},
		{"clm-far001", f(0.9)},
		{"clm-lift01", f(0.882)},
		{"clm-low001", f(0.9)},
		{"clm-rescu1", f(0.7)},
		{"clm-cap001", f(0.8)},
		{"clm-cap002", f(0.8)},
		{"clm-both01", f(0.7)},
		{"clm-half01", f(0.45)},
		{"clm-axm04", f(0.4)},
		{"clm-axm00", f(0)},
		{"clm-work00", f(0)},
		{"clm-work01", f(0.15)},
		{"clm-workpd", f(0.9)},
		{"clm-workns", f(0.9)},
		{"clm-targ01", f(0.7)},
	} {
		got, ok := finals[tc.id]
		if tc.want == nil && ok || tc.want != nil && (!ok || got != *tc.want) {
			t.Errorf("%s: final %v (set %v), want %v", tc.id, got, ok, tc.want)
		}
	}
	for _, tc := range []struct{ id, want string }{
		{"clm-chain1", " [= min(0.80, 0.40)]"},
		{"clm-lift01", " [= max(0.20, 0.88)]"},
		{"clm-low001", ""},
		{"clm-rescu1", " [= lift 0.70]"},
		{"clm-cap001", " [= min(0.90, 0.80)]"},
		{"clm-cap002", " [= min(lift 0.90, 0.80)]"},
		{"clm-both01", " [= min(max(min(0.90, 0.60), 0.80), 0.70)]"},
		{"clm-axm04", " [= min(0.90, 0.40)]"},
		{"clm-axm00", " [= min(0.90, 0.00)]"},
		{"clm-half01", " [= min(0.90, 0.45)]"},
		{"clm-far001", " [= min(1.00, 0.90)]"},
		{"clm-pend01", ""},
		{"clm-base01", ""},
		{"clm-targ01", " [= min(0.80, 0.70)]"},
	} {
		if got := RenderStrengthTrace(strength.Results[tc.id]); got != tc.want {
			t.Errorf("%s trace = %q, want %q", tc.id, got, tc.want)
		}
	}
	for _, tc := range []struct {
		id    string
		final *float64
		trace string
	}{{"ana-min01", f(0.9), " [= max(0.50, 0.90)]"}, {"ana-cap01", f(0.75), " [= min(0.90, 0.75)]"}} {
		r := strength.AnalysisResults[tc.id]
		if r.Final == nil || *r.Final != *tc.final {
			t.Errorf("%s final = %v, want %v", tc.id, r.Final, *tc.final)
		}
		if got := RenderStrengthTrace(r); got != tc.trace {
			t.Errorf("%s trace = %q, want %q", tc.id, got, tc.trace)
		}
	}
}

// TestWorkAndAxiomFeedTheirRigorAndDeriveNoStrength: a node with no inputs has
// strength = rigor, so a work and an axiom each feed bearing × rigor into a
// dependent's min — a work's rigor judged once by a person, an axiom's 1.0 by
// rule — and neither derives a strength of its own, in the walk or in its row.
func TestWorkAndAxiomFeedTheirRigorAndDeriveNoStrength(t *testing.T) {
	st := kb.State{
		ClaimEntries: []kb.ClaimEntry{
			claimEntry("clm-work01", f(0.9), founds("wrk-a", kb.Bearing{Value: 0.5})),
			claimEntry("clm-axm001", f(0.9), founds("axm-S1", kb.Bearing{Value: 0.5})),
		},
		Axioms: []kb.AxiomNode{{NodeType: kb.NodeKindAxiom, ID: "axm-S1", Title: "S1", CanonicalPath: "axioms.md", CanonicalAnchor: "axiom-s1"}},
		Works:  []kb.ExternalWork{{ID: "wrk-a", Title: "A", CanonicalPath: "claim-rigor.md", CanonicalAnchor: "a", Rigor: f(0.6)}},
	}
	strength, err := ComputeStrength(st)
	if err != nil {
		t.Fatal(err)
	}
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
	rows := map[string]kb.NodeRow{}
	for _, row := range buildClaims(st, strength) {
		rows[row.ID] = row
	}
	finals := strength.Finals()
	for id, rigor := range map[string]float64{"wrk-a": 0.6, "axm-S1": kb.AxiomRigor} {
		row := rows[id]
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
	st := kb.State{ClaimEntries: []kb.ClaimEntry{
		claimEntry("clm-aaaaaa", f(0.7)),
		claimEntry("clm-bbbbbb", f(0.9), supports("clm-aaaaaa", 0.72)),
		claimEntry("clm-cccccc", f(0.9), supports("clm-bbbbbb", 0.72)),
	}}
	strength, err := ComputeStrength(st)
	if err != nil {
		t.Fatal(err)
	}
	first := 0.72 * 0.7
	second := 0.72 * first
	finals := strength.Finals()
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

// TestClaimRefuterAndPairsIntoAnalysesReachTheRecords walks a KB whose claim
// refutes a claim and whose run experiments lift and refute analyses, and reads
// the numbers and the edge rows back off what the walk publishes.
func TestClaimRefuterAndPairsIntoAnalysesReachTheRecords(t *testing.T) {
	root := writeKB(t, map[string]string{
		"entry-point.md": "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n- [ana](ana.md)\n- [exp](exp.md)\n",
		"vol/a.md":       "---\nkind: leaf\nclaims: [clm-targ01, clm-refr01]\n---\n[↑ Vol](index.md)\n\n# A\n",
		"vol/ana.md": "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n  - ana-id: ana-bbbbbb\n---\n" +
			"[↑ Vol](index.md)\n\n# An\n",
		"vol/exp.md": "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n    rigor: 0.9\n" +
			"    lifts:\n      - ana-aaaaaa: 1.0\n  - exp-id: exp-bbbbbb\n    status: run\n    rigor: 1.0\n" +
			"    refutes:\n      - ana-bbbbbb: 0.25\n---\n[↑ Vol](index.md)\n\n# Ex\n",
		"vol/claim-rigor.md": "# Register\n\n" +
			"## Claim T\n<!-- id: clm-targ01 -->\n\n### Rigor\n- rigor: 0.80\n- strength: *pending*\n- rationale: r.\n\n" +
			"## Claim R\n<!-- id: clm-refr01 -->\n\n### Rigor\n- rigor: 0.30\n- strength: *pending*\n- rationale: r.\n" +
			"- refutes:\n  - clm-targ01: 1.0\n\n" +
			"## Ana A\n<!-- id: ana-aaaaaa -->\n\n### Rigor\n- rigor: 0.50\n- strength: *pending*\n- rationale: r.\n\n" +
			"## Ana B\n<!-- id: ana-bbbbbb -->\n\n### Rigor\n- rigor: 0.90\n- strength: *pending*\n- rationale: r.\n",
	})
	st, err := kb.Discover(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	strength, err := ComputeStrength(st)
	if err != nil {
		t.Fatal(err)
	}
	finals := strength.Finals()
	for id, r := range strength.AnalysisResults {
		if r.Final != nil {
			finals[id] = *r.Final
		}
	}
	for _, tc := range []struct {
		id   string
		want float64
	}{
		{"clm-targ01", 0.7},
		{"ana-aaaaaa", 0.9},
		{"ana-bbbbbb", 0.75},
	} {
		got, ok := finals[tc.id]
		if !ok || got != tc.want {
			t.Errorf("%s final = %v (set %v), want %v", tc.id, got, ok, tc.want)
		}
	}
	edges := map[string]bool{}
	for _, e := range buildEdges(st) {
		edges[e.Source+" "+e.Target+" "+e.Relation+" "+e.TargetKind+" "+fmt.Sprint(e.Bearing)] = true
	}
	for _, want := range []string{
		"clm-refr01 clm-targ01 refutes claim 1.0",
		"exp-aaaaaa ana-aaaaaa lifts analysis 1.0",
		"exp-bbbbbb ana-bbbbbb refutes analysis 0.25",
	} {
		if !edges[want] {
			t.Errorf("edges.yaml lacks the row %q; it carries %v", want, edges)
		}
	}
	if got := AuthoredDependsGraph(st)["clm-targ01"]; !slices.Contains(got, "clm-refr01") {
		t.Errorf("the acyclicity graph draws no edge from clm-targ01 to its refuter; it holds %v", got)
	}
}

// TestComputeStrengthRefusesARefutationOfAPremise: a claim refuting a claim it
// rests on is a cycle in the derivation — the refuter's value draws on the
// target it caps.
func TestComputeStrengthRefusesARefutationOfAPremise(t *testing.T) {
	st := kb.State{ClaimEntries: []kb.ClaimEntry{
		claimEntry("clm-aaaaaa", f(0.5)),
		{ID: "clm-bbbbbb", Rigor: f(0.5), Premises: []kb.Edge{supports("clm-aaaaaa", 1)},
			Refutes: []kb.Edge{{Source: "clm-bbbbbb", Target: "clm-aaaaaa",
				Relation: kb.RelationRefutes, TargetKind: kb.NodeKindClaim, Bearing: kb.Bearing{Value: 0.5}}}},
	}}
	_, err := ComputeStrength(st)
	var cycle CycleError
	if !errors.As(err, &cycle) || !slices.Equal(cycle.Members, []string{"clm-aaaaaa", "clm-bbbbbb"}) {
		t.Errorf("err = %v, want a cycle over the target and its refuter", err)
	}
}

func TestComputeStrengthRefusesACycleAmongUnscoredClaims(t *testing.T) {
	st := kb.State{ClaimEntries: []kb.ClaimEntry{
		claimEntry("clm-aaaaaa", nil, supports("clm-bbbbbb", 1)),
		claimEntry("clm-bbbbbb", nil, supports("clm-aaaaaa", 1)),
		claimEntry("clm-cccccc", f(0.5)),
	}}
	_, err := ComputeStrength(st)
	var cycle CycleError
	if !errors.As(err, &cycle) || !slices.Equal(cycle.Members, []string{"clm-aaaaaa", "clm-bbbbbb"}) {
		t.Errorf("err = %v", err)
	}
}

// TestComputeStrengthRefusesALiftOfAPremise: an analysis that lifts a claim it
// rests on is a cycle, whichever list each edge was authored in.
func TestComputeStrengthRefusesALiftOfAPremise(t *testing.T) {
	st := kb.State{
		ClaimEntries: []kb.ClaimEntry{claimEntry("clm-aaaaaa", f(0.5))},
		Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Rigor: f(0.9),
			Premises: []kb.Edge{supports("clm-aaaaaa", 1)},
			Lifts:    []kb.Edge{{Source: "ana-aaaaaa", Target: "clm-aaaaaa", Relation: kb.RelationLifts, TargetKind: "claim", Bearing: kb.Bearing{Value: 1}}}}},
	}
	_, err := ComputeStrength(st)
	var cycle CycleError
	if !errors.As(err, &cycle) || !slices.Equal(cycle.Members, []string{"ana-aaaaaa", "clm-aaaaaa"}) {
		t.Errorf("err = %v, want a cycle over the claim and its analysis", err)
	}
}

// escaped is the JSON escape of the code point hex names.
func escaped(hex string) string { return `\` + "u" + hex }

func TestSerialize(t *testing.T) {
	recs := []record{{{Key: "s", Value: "a\"b\\c\n\t\x01é"}, {Key: "n"}, {Key: "f", Value: 0.5}, {Key: "i", Value: 3}, {Key: "b", Value: true},
		{Key: "l", Value: []string{"x", "y"}}, {Key: "e", Value: []string{}}},
		{{Key: "breaks", Value: "ls" + string(rune(0x2028)) + "ps" + string(rune(0x2029)) + "nel\u0085del\u007fbom" + string(rune(0xfeff)) +
			"fffe" + string(rune(0xfffe)) + "ffff" + string(rune(0xffff))}}}
	want := `--- {"s": "a\"b\\c\n\t\u0001é", "n": null, "f": 0.5, "i": 3, "b": true, "l": ["x", "y"], "e": []}` + "\n" +
		`--- {"breaks": "ls` + escaped("2028") + "ps" + escaped("2029") + "nel" + escaped("0085") + "del" + escaped("007f") + "bom" +
		escaped("feff") + "fffe" + escaped("fffe") + "ffff" + escaped("ffff") + `"}` + "\n"
	if got := Serialize(recs); got != want {
		t.Errorf("Serialize =\n%s\nwant\n%s", got, want)
	}
	if got := Serialize(nil); got != "" {
		t.Errorf("Serialize(nil) = %q", got)
	}
}

// TestSerializeAsTheMigrationWrites: an index the 0.9.0 → 1.0.0 migration
// converts reads back byte for byte as refresh writes it, so a migrated KB's
// index is the writer's own.
func TestSerializeAsTheMigrationWrites(t *testing.T) {
	recs := []record{
		{{Key: "node_type", Value: "claim"}, {Key: "id", Value: "clm-aaaaaa"}, {Key: "title", Value: "A — \"quoted\" <b> & ü x"},
			{Key: "rigor", Value: 0.5}, {Key: "strength"}, {Key: "n", Value: 2}},
		{{Key: "claim_id", Value: "clm-aaaaaa"}, {Key: "mentioned_ids", Value: []string{"exp-bbbbbb"}}, {Key: "tier2_marked", Value: false}, {Key: "text", Value: "tab\tff\fbs\bc1\u0085"}},
	}
	stream := Serialize(recs)
	var jsonl strings.Builder
	for _, line := range strings.SplitAfter(stream, "\n") {
		record, _ := strings.CutPrefix(line, kb.IndexRecordMarker)
		jsonl.WriteString(record)
	}
	out, _, err := migrate.Chain(kb.UnstampedFormatVersion, kb.FormatVersion, migrate.Files{"kb-root/.index/claims.jsonl": []byte(jsonl.String())})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out["kb-root/.index/claims.yaml"]); got != stream {
		t.Errorf("the migration writes\n%s\nrefresh writes\n%s", got, stream)
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

// TestCoverageRefusalNamesTheBullet: an edge whose source no authored node
// stands up is refused with the register, the line and the id it names, so the
// one who wrote the bullet can find it.
func TestCoverageRefusalNamesTheBullet(t *testing.T) {
	register := "# Register\n\n## A\n<!-- id: clm-aaaaaa -->\n\n### Rigor\n- rigor: 0.8\n" +
		"- rests-on:\n  - wrk-ghost — Missing (bearing 0.5)\n\n### Strength\n- strength: *pending*\n"
	root := writeKB(t, map[string]string{
		"entry-point.md": "---\nkind: entry-point\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md":       "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ Vol](index.md)\n\n# A\n",
		kb.RegisterFile:  register,
	})
	st, err := kb.Discover(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildRecords(st)
	var cov CoverageError
	if !errors.As(err, &cov) {
		t.Fatalf("BuildRecords = %v, want a CoverageError", err)
	}
	if cov.Key != "wrk-ghost" {
		t.Errorf("coverage = %+v, want the id it names", cov)
	}
	line := strings.Count(register[:strings.Index(register, "wrk-ghost")], "\n") + 1
	var items result.Refusal
	if mapped := refusal(kb.OnDisk(root), err); !errors.As(mapped, &items) || len(items) != 1 {
		t.Fatalf("refusal(err) = %#v, want one item", mapped)
	}
	if it := items[0]; it.Check != checkMetadata || it.Key != "wrk-ghost" || it.Path != kb.RegisterFile || it.Line != line {
		t.Errorf("item = %+v, want check %s for wrk-ghost at %s:%d", it, checkMetadata, kb.RegisterFile, line)
	}
}

const fmKind = "---\nkind: %s\n---\n\n"

func withKind(kind, body string) string { return strings.Replace(fmKind, "%s", kind, 1) + body }

func TestCitationFindings(t *testing.T) {
	target := "# Target\n\n## The Section\n\nThe load-bearing clause lives here, stated plainly.\n"
	register := "## Entry\n<!-- id: clm-aa1111 -->\n\nRationale prose naming clm-bb2222 from the other domain.\n"
	nodes := `--- {"id": "clm-aa1111", "canonical_path": "alpha/one.md"}` + "\n" + `--- {"id": "clm-bb2222", "canonical_path": "beta/two.md"}` + "\n"
	long := strings.Repeat("x", excerptMaxChars+1)
	bound := strings.Repeat("y", excerptMaxChars)
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"plain invariant numbering", map[string]string{"index.md": withKind("index", "This follows per design-doc Invariant 3.\n")}, []string{checkChannel}},
		{"invariant id", map[string]string{"index.md": withKind("index", "See INVARIANT-S2 for the numbering.\n")}, []string{checkChannel}},
		{"bare claim id", map[string]string{"index.md": withKind("index", "The claim clm-aa1111 supports this.\n")}, []string{checkChannel}},
		{"clean prose", map[string]string{"index.md": withKind("index", "Nothing citation-shaped here at all.\n")}, nil},
		{"fenced example", map[string]string{"index.md": withKind("index", "How:\n\n```\nSee INVARIANT-S2 and clm-aa1111, per Invariant 11.\n```\n")}, nil},
		{"sanctioned channels", map[string]string{
			"claim-rigor.md": "## A Claim\n<!-- id: clm-aa1111 -->\n\n- rigor: 0.75\n- rests-on:\n  - INVARIANT-S2 (axiom scaffold)\n  - Axiom S1 (bearing 0.8)\n  - clm-bb2222 — a real dependency (strength 0.80) (bearing 1.0)\n- strength: 0.75 (ok to build on, see caveats)\n",
			"leaf.md":        withKind("leaf", "<!-- claim-rigor: clm-aa1111 -->\n\nProse.\n"),
		}, nil},
		{"axiom declaration sections", map[string]string{
			"axioms.md": "# Axioms\n\n### Axiom S1: First rule\n\nHolds with Axiom S2 and INVARIANT-S3 around it.\n",
		}, nil},
		{"axiom token outside its declaration", map[string]string{
			"axioms.md": "# Axioms\n\n## Preamble\n\nThis corpus relies on Axiom S2 throughout.\n\n### Axiom S2: Axiom numbering\n\nBody.\n",
		}, []string{checkChannel}},
		{"bare axiom token in prose", map[string]string{"index.md": withKind("index", "See Axiom S1 for the design rule.\n")}, []string{checkChannel}},
		{"legacy declaration in AGENTS.md", map[string]string{
			"AGENTS.md": "# Agents\n\n### INVARIANT-S1: First rule\n\nHolds jointly with INVARIANT-S2.\n\n### INVARIANT-S2: Axiom numbering\n\nBody.\n",
		}, nil},
		{"legacy axiom bullet in AGENTS.md", map[string]string{
			"AGENTS.md": "# Agents\n\n- Axiom 1: **One** — a bullet declaration.\n\nOther instructions.\n",
		}, nil},
		{"axiom token in AGENTS.md prose", map[string]string{
			"AGENTS.md": "# Agents\n\nFollow Axiom S2 when in doubt.\n\n### INVARIANT-S1: First rule\n\nBody.\n",
		}, []string{checkChannel}},
		{"leaf body exempt", map[string]string{
			"leaf.md": withKind("leaf", "The source says INVARIANT-S2 and clm-aa1111, per Invariant 4.\n"),
		}, nil},
		{"declaration sections may name siblings", map[string]string{
			"invariants.md": "# Invariants\n\n### INVARIANT-S1: First rule\n\nHolds jointly with INVARIANT-S2, which numbers the axioms.\n\n### INVARIANT-S2: Axiom numbering\n\n- Axiom 1: **One** — a bullet.\n",
		}, nil},
		{"invariant prose outside a declaration", map[string]string{
			"invariants.md": "# Invariants\n\n## Preamble\n\nThis corpus relies on INVARIANT-S2 throughout.\n\n### INVARIANT-S2: Axiom numbering\n\nBody.\n",
		}, []string{checkChannel}},
		{"well-formed authority citation", map[string]string{
			"index.md": withKind("index", `As established in ["The load-bearing clause lives here"](target.md#the-section).`+"\n"), "target.md": target,
		}, nil},
		{"unresolvable target", map[string]string{"index.md": withKind("index", `See ["the clause"](nowhere.md#x).`+"\n")}, []string{checkReferent}},
		{"missing anchor", map[string]string{
			"index.md": withKind("index", `See ["The load-bearing clause lives here"](target.md#no-such).`+"\n"), "target.md": target,
		}, []string{checkReferent}},
		{"absent excerpt", map[string]string{
			"index.md": withKind("index", `See ["a clause never written there"](target.md#the-section).`+"\n"), "target.md": target,
		}, []string{checkExcerpt}},
		{"whitespace-normalized excerpt", map[string]string{
			"index.md": withKind("index", `See ["The load-bearing   clause    lives here"](target.md#the-section).`+"\n"), "target.md": target,
		}, nil},
		{"excerpt without anchor", map[string]string{
			"index.md": withKind("index", `See ["The load-bearing clause lives here"](target.md).`+"\n"), "target.md": target,
		}, []string{checkExcerpt}},
		{"bloated excerpt", map[string]string{
			"index.md": withKind("index", `See ["`+long+`"](target.md#the-section).`+"\n"), "target.md": "# T\n\n## The Section\n\n" + long + "\n",
		}, []string{checkExcerpt}},
		{"excerpt at the bound", map[string]string{
			"index.md": withKind("index", `See ["`+bound+`"](target.md#the-section).`+"\n"), "target.md": "# T\n\n## The Section\n\n" + bound + "\n",
		}, nil},
		{"blank excerpt", map[string]string{
			"index.md": withKind("index", `See [" "](target.md#the-section) for the rule.`+"\n"), "target.md": target,
		}, []string{checkExcerpt}},
		{"excerpt only inside a fence at the target", map[string]string{
			"index.md":  withKind("index", `Authority: ["the moon is made of cheese"](target.md#the-section).`+"\n"),
			"target.md": "# Target\n\n## The Section\n\nThe real clause.\n\n```\nthe moon is made of cheese\n```\n",
		}, []string{checkExcerpt}},
		{"anchor only as a fenced heading", map[string]string{
			"index.md":  withKind("index", `Authority: ["invented authority text"](target.md#fake-section).`+"\n"),
			"target.md": "# Target\n\n```markdown\n## Fake Section\n\ninvented authority text\n```\n",
		}, []string{checkReferent}},
		{"typographic quotes", map[string]string{
			"index.md": withKind("index", "Authority: [“a clause that is not in the target”](target.md#the-section).\n"), "target.md": target,
		}, []string{checkExcerpt}},
		{"bracket inside the link text", map[string]string{
			"index.md": withKind("index", `Authority: ["the rule [see note] applies"](does-not-exist.md#nope).`+"\n"),
		}, []string{checkReferent}},
		{"non-durable target", map[string]string{
			"domain/sub/index.md": withKind("index", `See ["the clause"](../../../elsewhere/notes.md#x).`+"\n"),
		}, []string{checkDurable}},
		{"external link", map[string]string{"index.md": withKind("index", "See [the spec](https://example.invalid/spec).\n")}, nil},
		{"foreign-domain reference without an edge", map[string]string{
			"alpha/claim-rigor.md": register, ".index/claims.yaml": nodes, ".index/edges.yaml": "",
		}, []string{checkChannel, checkEdge}},
		{"foreign-domain reference with an edge", map[string]string{
			"alpha/claim-rigor.md": register, ".index/claims.yaml": nodes,
			".index/edges.yaml": `--- {"source": "clm-bb2222", "target": "clm-aa1111"}` + "\n",
		}, []string{checkChannel}},
		{"no-edge exemption", map[string]string{
			"alpha/claim-rigor.md": "## Entry\n<!-- id: clm-aa1111 -->\n\n- no-edge: cited as contrast\n\nRationale prose naming clm-bb2222 from the other domain.\n",
			".index/claims.yaml":   nodes,
			".index/edges.yaml":    "",
		}, []string{checkChannel}},
		{".index out of scope", map[string]string{".index/SCHEMA.md": "# Schema\n\n1. Invariant 1: the index is derived.\n"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := CitationFindings(kb.OnDisk(writeKB(t, tc.files)))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, fd := range findings {
				got = append(got, fd.Check)
			}
			slices.Sort(got)
			got = slices.Compact(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("checks = %q, want %q; findings %v", got, tc.want, findings)
			}
		})
	}
}
