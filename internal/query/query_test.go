package query

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

func f(v float64) *float64 { return &v }

// The node records the fixtures are built of. A scored claim rests on
// scoringAxiom at bearing 1.0, so its strength is its rigor.
const scoringAxiom = "axm-S0"

func claim(id, title string) kb.ClaimNode { return kb.ClaimNode{ID: id, Title: title} }

// scored is g with the claim id at rigor (pending where nil), resting on
// scoringAxiom, which g gains where it lacks it.
func scored(g kb.Graph, id string, rigor *float64) kb.Graph {
	if !slices.ContainsFunc(g.Axioms, func(a kb.AxiomNode) bool { return a.ID == scoringAxiom }) {
		g.Axioms = append(g.Axioms, kb.AxiomNode{ID: scoringAxiom, Title: "The scoring rule"})
	}
	for i := range g.Claims {
		if g.Claims[i].ID == id {
			g.Claims[i].Rigor = rigor
		}
	}
	for i := range g.Analyses {
		if g.Analyses[i].ID == id {
			g.Analyses[i].Rigor = rigor
		}
	}
	g.Founds = append(g.Founds, kb.BearingEdge{Source: scoringAxiom, Target: id, Bearing: kb.Bearing{Value: 1}})
	return g
}

func scorable(relation, source, target string, bearing float64) kb.ScorableEdge {
	return kb.ScorableEdge{Relation: relation, Source: source, Target: target, Bearing: kb.Bearing{Value: bearing}}
}

// leafDoc is a leaf of the main volume holding body.
func leafDoc(body string) string { return "[↑ Up](index.md)\n\n# Leaf\n\n" + body }

// writeRecordKB writes a KB of g's records and docs, each at its kb-root
// relative path, and returns its kb-root; an entry point docs does not supply
// is stamped and lists the main volume, whose index lists main/leaf.md.
func writeRecordKB(t *testing.T, g kb.Graph, docs map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	files := map[string]string{
		kb.EntryPointFile: "---\nkb-format: \"" + kb.FormatVersion + "\"\n---\n# KB\n\n- [Main](main/index.md)\n",
		"main/index.md":   "[↑ Up](../entry-point.md)\n\n# Main\n\n- [Leaf](leaf.md)\n",
		kb.GraphNodesFile: string(kb.EncodeNodes(g)),
		kb.GraphEdgesFile: string(kb.EncodeEdges(g)),
		"main/leaf.md":    leafDoc(""),
	}
	maps.Copy(files, docs)
	for rel, text := range files {
		writeFile(t, filepath.Join(root, rel), text)
	}
	return root
}

// load is the index Load answers over a KB of g's records, every claim,
// analysis and experiment but those unhosted names hosted by a statement
// marker in main/leaf.md.
func load(t *testing.T, g kb.Graph, unhosted ...string) *Index {
	t.Helper()
	var body strings.Builder
	host := func(id string) {
		if !slices.Contains(unhosted, id) {
			fmt.Fprintf(&body, "The statement of %s. %s\n\n", id, kb.StatementMarker(id))
		}
	}
	for _, n := range g.Claims {
		host(n.ID)
	}
	for _, n := range g.Analyses {
		host(n.ID)
	}
	for _, n := range g.Experiments {
		host(n.ID)
	}
	return loadKB(t, writeRecordKB(t, g, map[string]string{"main/leaf.md": leafDoc(body.String())}))
}

func loadKB(t *testing.T, root string) *Index {
	t.Helper()
	ix, err := Load(kb.OnDisk(root))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return ix
}

func ids(nodes []kb.NodeRow) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

func TestFind(t *testing.T) {
	ix := load(t, kb.Graph{Claims: []kb.ClaimNode{
		claim("clm-cccccc", "Proposition 4.4 — KPI-Gating as Discrete Pontryagin"),
		claim("clm-aaaaaa", "Proposition 4.3 — Institutional Misalignment"),
		claim("clm-bbbbbb", "Theorem 4.5 — Positive Invariance of M"),
		claim("clm-dddddd", "Lemma 1 — Straße"),
	}})
	for _, tc := range []struct {
		name, query string
		want        []string
	}{
		{"by number", "4.3", []string{"clm-aaaaaa"}},
		{"case-insensitive", "pontryagin", []string{"clm-cccccc"}},
		{"shared substring, id order", "Proposition", []string{"clm-aaaaaa", "clm-cccccc"}},
		{"full case folding", "STRASSE", []string{"clm-dddddd"}},
		{"empty matches all", "", []string{"clm-aaaaaa", "clm-bbbbbb", "clm-cccccc", "clm-dddddd"}},
		{"no match", "no-such-claim", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ids(ix.Find(tc.query)); !slices.Equal(got, tc.want) {
				t.Errorf("Find(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func leaf(body string) string {
	return "[↑ Up](../index.md)\n\n# Leaf\n\n" + body + "\n"
}

// TestReferencedBy: the leaves linking to the leaf whose marker first names
// the claim, a leaf's body link not making it an index; an index's child list
// is not a reference.
func TestReferencedBy(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, kb.EntryPointFile), "---\nkb-format: \""+kb.FormatVersion+"\"\n---\n# KB\n\n- [Main](main/index.md)\n- [Other](other/index.md)\n")
	writeFile(t, filepath.Join(root, "main/index.md"), "[↑ Up](../entry-point.md)\n\n# Main\n\n- [Origin](origin.md)\n- [Citing](citing.md)\n- [Coded](coded.md)\n")
	writeFile(t, filepath.Join(root, "other/index.md"), "[↑ Up](../entry-point.md)\n\n# Other\n\n- [Deep](deep.md)\n")
	writeFile(t, filepath.Join(root, "main/origin.md"), leaf("Origin claim. <!-- claim-rigor: clm-aaaaaa --> Self link [here](origin.md)."))
	writeFile(t, filepath.Join(root, "main/citing.md"), leaf("See [the origin](origin.md#sec) for it."))
	writeFile(t, filepath.Join(root, "other/deep.md"), leaf("Back [up](../main/origin.md)."))
	writeFile(t, filepath.Join(root, "main/coded.md"), leaf("Only `[x](origin.md)` in code."))
	g := kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Origin Claim", Rigor: f(0.5)}}}
	writeFile(t, filepath.Join(root, kb.GraphNodesFile), string(kb.EncodeNodes(g)))
	writeFile(t, filepath.Join(root, kb.GraphEdgesFile), string(kb.EncodeEdges(g)))
	ix, err := Load(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ix.ReferencedBy("clm-aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"main/citing.md", "other/deep.md"}; !slices.Equal(got, want) {
		t.Errorf("ReferencedBy = %v, want %v", got, want)
	}
	if got, _ := ix.ReferencedBy("clm-zzzzzz"); len(got) != 0 {
		t.Errorf("unknown claim: %v, want none", got)
	}
}

// TestStrengthBelowAndWeakPoints: E rests on A, so its strength is A's 0.5
// whatever its own rigor; it falls below the threshold and, nothing resting
// on it, is no weak point.
func TestStrengthBelowAndWeakPoints(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C"),
			claim("clm-dddddd", "D"), claim("clm-eeeeee", "E")},
		References: []kb.UnscoredEdge{{Source: "clm-eeeeee", Target: "clm-aaaaaa"}},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-aaaaaa", "clm-dddddd", 1),
			scorable(kb.RelationSupports, "clm-aaaaaa", "clm-eeeeee", 1),
			scorable(kb.RelationSupports, "clm-bbbbbb", "clm-dddddd", 1),
			scorable(kb.RelationSupports, "clm-cccccc", "clm-dddddd", 1),
		},
	}
	for id, rigor := range map[string]*float64{"clm-aaaaaa": f(0.5), "clm-bbbbbb": f(0.3), "clm-cccccc": f(0.5), "clm-dddddd": nil, "clm-eeeeee": f(0.9)} {
		g = scored(g, id, rigor)
	}
	ix := load(t, g)
	if got, want := ids(ix.StrengthBelow(0.65)), []string{"clm-bbbbbb", "clm-aaaaaa", "clm-cccccc", "clm-eeeeee"}; !slices.Equal(got, want) {
		t.Errorf("StrengthBelow(0.65) = %v, want %v", got, want)
	}
	var got []string
	for _, wp := range ix.WeakPoints(DefaultMaxStrength, DefaultMinDependents) {
		got = append(got, wp.Claim.ID)
	}
	if want := []string{"clm-aaaaaa", "clm-bbbbbb", "clm-cccccc"}; !slices.Equal(got, want) {
		t.Errorf("WeakPoints = %v, want %v (most distinct dependents, then lowest strength, then id)", got, want)
	}
	if wp := ix.WeakPoints(DefaultMaxStrength, 2); len(wp) != 1 || wp[0].Dependents != 2 {
		t.Errorf("WeakPoints(min 2) = %+v, want clm-aaaaaa with 2", wp)
	}
}

func TestDependentsOf(t *testing.T) {
	demoted := func(source, target string) kb.DemotedEdge {
		return kb.DemotedEdge{Source: source, Target: target, ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Pending: true}}
	}
	ix := load(t, kb.Graph{
		Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C"), claim("clm-dddddd", "D"),
			claim("clm-eeeeee", "E"), claim("clm-ffffff", "F"), claim("clm-gggggg", "G"), claim("clm-hhhhhh", "H")},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "X", Status: kb.ExperimentRun}},
		References:  []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-ffffff"}, {Source: "clm-bbbbbb", Target: "clm-aaaaaa"}},
		Demoted:     []kb.DemotedEdge{demoted("clm-aaaaaa", "clm-gggggg"), demoted("clm-cccccc", "clm-aaaaaa")},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-aaaaaa", "clm-dddddd", 1),
			scorable(kb.RelationSupports, "clm-aaaaaa", "clm-eeeeee", 1),
			scorable(kb.RelationRefutes, "clm-aaaaaa", "clm-dddddd", 1),
			scorable(kb.RelationLifts, "exp-aaaaaa", "clm-aaaaaa", 1),
			scorable(kb.RelationLifts, "ana-aaaaaa", "clm-aaaaaa", 1),
			scorable(kb.RelationRefutes, "clm-aaaaaa", "clm-hhhhhh", 1),
		},
	})
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"clm-aaaaaa", []string{"clm-dddddd", "clm-eeeeee", "clm-hhhhhh"}},
		{"exp-aaaaaa", []string{"clm-aaaaaa"}},
		{"ana-aaaaaa", []string{"clm-aaaaaa"}},
		{"clm-bbbbbb", []string{}},
		{"clm-cccccc", []string{}},
		{"clm-ffffff", []string{}},
		{"clm-gggggg", []string{}},
	} {
		if got := ix.DependentsOf(tc.id); !slices.Equal(got, tc.want) {
			t.Errorf("DependentsOf(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// depsRows is id's deps rows as "relation direction target=bearing".
func depsRows(ix *Index, id string) []string {
	var out []string
	for _, rec := range DepsPayload(ix, id, false).([]result.Record) {
		out = append(out, fmt.Sprint(rec[0].Value, " ", rec[1].Value, " ", rec[2].Value, "=", rec[3].Value))
	}
	return out
}

// TestDepsRows: an analysis supplies no premise, so it is a supports edge's
// target alone.
func TestDepsRows(t *testing.T) {
	founds := func(source string, bearing kb.Bearing) kb.BearingEdge {
		return kb.BearingEdge{Source: source, Target: "clm-aaaaaa", Bearing: bearing}
	}
	ix := load(t, kb.Graph{
		Claims:   []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C")},
		Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S", Rigor: f(0.8)}},
		Axioms:   []kb.AxiomNode{{ID: "axm-S1", Title: "The design rule"}, {ID: "axm-S2", Title: "The other rule"}},
		Works:    []kb.WorkNode{{ID: "wrk-B", Title: "B"}, {ID: "wrk-C", Title: "C"}, {ID: "wrk-Z", Title: "Z"}},
		Founds: []kb.BearingEdge{founds("wrk-Z", kb.Bearing{Pending: true}), founds("wrk-B", kb.Bearing{Value: 0.5}),
			founds("wrk-C", kb.Bearing{}), founds("axm-S1", kb.Bearing{Value: 1}), founds("axm-S2", kb.Bearing{Pending: true})},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-aaaaaa", "ana-aaaaaa", 0.6),
			scorable(kb.RelationLifts, "ana-aaaaaa", "clm-cccccc", 0.8),
		},
		References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb"}},
	})
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"clm-aaaaaa", []string{
			"supports out ana-aaaaaa=0.6",
			"founds in axm-S1=1.0",
			"founds in axm-S2=*pending*",
			"founds in wrk-B=0.5",
			"founds in wrk-C=0.0",
			"founds in wrk-Z=*pending*",
			"references out clm-bbbbbb=<nil>",
		}},
		{"ana-aaaaaa", []string{"supports in clm-aaaaaa=0.6", "lifts out clm-cccccc=0.8"}},
	} {
		if got := depsRows(ix, tc.id); !slices.Equal(got, tc.want) {
			t.Errorf("deps %s = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestDepsRowsForARefutingClaimAndALiftedAnalysis: a claim's refutation reaches
// deps out of the refuter, and an experiment's lift or refutation reaches deps
// into the analysis it lands on.
func TestDepsRowsForARefutingClaimAndALiftedAnalysis(t *testing.T) {
	ix := load(t, kb.Graph{
		Claims:      []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S", Rigor: f(0.8)}, {ID: "ana-bbbbbb", Title: "T", Rigor: f(0.8)}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "X", Status: kb.ExperimentRun}},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationRefutes, "clm-aaaaaa", "clm-bbbbbb", 1),
			scorable(kb.RelationLifts, "exp-aaaaaa", "ana-aaaaaa", 0.9),
			scorable(kb.RelationRefutes, "exp-aaaaaa", "ana-bbbbbb", 0.25),
		},
	})
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"clm-aaaaaa", []string{"refutes out clm-bbbbbb=1.0"}},
		{"clm-bbbbbb", []string{"refutes in clm-aaaaaa=1.0"}},
		{"ana-aaaaaa", []string{"lifts in exp-aaaaaa=0.9"}},
		{"ana-bbbbbb", []string{"refutes in exp-aaaaaa=0.25"}},
	} {
		if got := depsRows(ix, tc.id); !slices.Equal(got, tc.want) {
			t.Errorf("deps %s = %v, want %v", tc.id, got, tc.want)
		}
	}
	if got, want := ix.DependentsOf("clm-aaaaaa"), []string{"clm-bbbbbb"}; !slices.Equal(got, want) {
		t.Errorf("DependentsOf(clm-aaaaaa) = %v, want %v", got, want)
	}
	if got, want := ix.DependentsOf("exp-aaaaaa"), []string{"ana-aaaaaa", "ana-bbbbbb"}; !slices.Equal(got, want) {
		t.Errorf("DependentsOf(exp-aaaaaa) = %v, want %v", got, want)
	}
}

// TestDepsOrderIgnoresFileOrder: the same edge records, the edge file holding
// them in either order, answer the same rows.
func TestDepsOrderIgnoresFileOrder(t *testing.T) {
	edges := []kb.ScorableEdge{
		scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", 0.7),
		scorable(kb.RelationSupports, "clm-cccccc", "clm-aaaaaa", 0.7),
		scorable(kb.RelationSupports, "clm-aaaaaa", "clm-dddddd", 0.7),
	}
	edges[0].Context, edges[1].Context = "first use", "second use"
	g := kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C"), claim("clm-dddddd", "D")}}
	edgeFile := func(order []kb.ScorableEdge) string {
		var b strings.Builder
		for _, e := range order {
			b.Write(kb.EncodeEdges(kb.Graph{Scorable: []kb.ScorableEdge{e}}))
		}
		return b.String()
	}
	backward := slices.Clone(edges)
	slices.Reverse(backward)
	inOrder := loadKB(t, writeRecordKB(t, g, map[string]string{kb.GraphEdgesFile: edgeFile(edges)}))
	reversed := loadKB(t, writeRecordKB(t, g, map[string]string{kb.GraphEdgesFile: edgeFile(backward)}))
	if got, want := DepsPayload(reversed, "clm-aaaaaa", false), DepsPayload(inOrder, "clm-aaaaaa", false); !reflect.DeepEqual(got, want) {
		t.Errorf("deps over the reversed edge file = %v, want %v", got, want)
	}
}

func TestDepsContext(t *testing.T) {
	withContext := scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", 0.7)
	withContext.Context = "used in step 2"
	ix := load(t, kb.Graph{
		Claims:   []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C")},
		Scorable: []kb.ScorableEdge{withContext, scorable(kb.RelationSupports, "clm-cccccc", "clm-aaaaaa", 0.8)},
	})
	want := []result.Record{
		{{Key: "relation", Value: kb.RelationSupports}, {Key: "direction", Value: "in"}, {Key: "target", Value: "clm-bbbbbb"}, {Key: "bearing", Value: "0.7"},
			{Key: manuallyScoredKey, Value: false}, {Key: "context", Value: "used in step 2"}},
		{{Key: "relation", Value: kb.RelationSupports}, {Key: "direction", Value: "in"}, {Key: "target", Value: "clm-cccccc"}, {Key: "bearing", Value: "0.8"},
			{Key: manuallyScoredKey, Value: false}, {Key: "context", Value: nil}},
	}
	if got := DepsPayload(ix, "clm-aaaaaa", false); !reflect.DeepEqual(got, want) {
		t.Errorf("deps = %v, want %v", got, want)
	}
}

// TestShowStrengthenBy: an item's mentioned ids are the claims its text names.
func TestShowStrengthenBy(t *testing.T) {
	a, b := claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")
	a.StrengthenBy = []string{"Prove clm-bbbbbb first, after wrk-A.", "Bound the remainder."}
	b.StrengthenBy = []string{"Other claim's item."}
	ix := load(t, kb.Graph{Claims: []kb.ClaimNode{a, b}, Works: []kb.WorkNode{{ID: "wrk-A", Title: "W", Rigor: f(0.5)}}})
	rec, _ := ShowPayload(ix, "clm-aaaaaa")
	items, at := fieldAt(rec, "strengthen_by")
	if _, count := fieldAt(rec, "strengthen_by_count"); at != count+1 {
		t.Errorf("strengthen_by at %d, want right after strengthen_by_count at %d", at, count)
	}
	want := []result.Record{
		{{Key: "item_idx", Value: 0}, {Key: "text", Value: "Prove clm-bbbbbb first, after wrk-A."}, {Key: "mentioned_ids", Value: []string{"clm-bbbbbb"}}},
		{{Key: "item_idx", Value: 1}, {Key: "text", Value: "Bound the remainder."}, {Key: "mentioned_ids", Value: []string{}}},
	}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("show strengthen_by = %v, want %v", items, want)
	}
	work, _ := ShowPayload(ix, "wrk-A")
	if _, at := fieldAt(work, "strengthen_by"); at != -1 {
		t.Errorf("work record carries strengthen_by: %v", work)
	}
	for _, c := range StrengthBelowPayload(load(t, scored(kb.Graph{Claims: []kb.ClaimNode{a}}, a.ID, f(0.1))), 1) {
		if _, at := fieldAt(c, "strengthen_by"); at != -1 {
			t.Errorf("strength-below record carries strengthen_by: %v", c)
		}
	}
}

func TestSubtreeClaims(t *testing.T) {
	ix := loadKB(t, writeRecordKB(t, kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")}}, map[string]string{
		kb.EntryPointFile: "---\nkb-format: \"" + kb.FormatVersion + "\"\n---\n# KB\n\n- [Top](top.md)\n- [Vol](vol/index.md)\n",
		"top.md":          "[↑ Up](entry-point.md)\n\n# Top\n\nA. " + kb.StatementMarker("clm-aaaaaa") + "\n",
		"vol/index.md":    "[↑ Up](../entry-point.md)\n\n# Vol\n\n- [Leaf](leaf.md)\n",
		"vol/leaf.md":     "[↑ Up](index.md)\n\n# Leaf\n\nB. " + kb.StatementMarker("clm-bbbbbb") + "\n",
	}))
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"", []string{"clm-aaaaaa", "clm-bbbbbb"}},
		{".", []string{"clm-aaaaaa", "clm-bbbbbb"}},
		{"vol/index.md", []string{"clm-bbbbbb"}},
		{"vol", []string{"clm-bbbbbb"}},
		{"vol//", []string{"clm-bbbbbb"}},
		{"nowhere", []string{}},
	} {
		if got := ix.SubtreeClaims(tc.path); !slices.Equal(got, tc.want) {
			t.Errorf("SubtreeClaims(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestNodeFieldsTotalOverKinds(t *testing.T) {
	for _, kind := range kb.NodeKinds {
		if nodeFields[kind] == nil {
			t.Errorf("no field list for node kind %q", kind)
		}
	}
	if len(nodeFields) != len(kb.NodeKinds) {
		t.Errorf("nodeFields has %d kinds, kb.NodeKinds %d", len(nodeFields), len(kb.NodeKinds))
	}
}

// TestAnalysisBandAndRecord: an analysis's build band is derived from its
// strength as the index loads, and show's record carries its kind, rigor and
// strength.
func TestAnalysisBandAndRecord(t *testing.T) {
	ix := load(t, scored(kb.Graph{Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S"}}}, "ana-aaaaaa", f(0.7)))
	n, ok := ix.Node("ana-aaaaaa")
	if !ok || n.BuildBand != "ok-with-caveats" {
		t.Errorf("analysis node = %+v, %v; want build_band ok-with-caveats", n, ok)
	}
	rec, ok := ShowPayload(ix, "ana-aaaaaa")
	if !ok {
		t.Fatal("show ana-aaaaaa: no record")
	}
	want := map[string]any{"node_type": kb.NodeKindAnalysis, "id": "ana-aaaaaa", "rigor": 0.7, "strength": 0.7, "build_band": "ok-with-caveats"}
	got := map[string]any{}
	for _, f := range rec {
		got[f.Key] = f.Value
	}
	for key, v := range want {
		if !reflect.DeepEqual(got[key], v) {
			t.Errorf("show %s = %v, want %v", key, got[key], v)
		}
	}
}

// TestShowDisplaysAStrength: show's strength field is the display rounding of
// a value the walk computed, while the index row keeps its exact value.
func TestShowDisplaysAStrength(t *testing.T) {
	bearing, premise := 0.2, 0.9
	exact := bearing * premise
	g := kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")},
		Scorable: []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", bearing)}}
	ix := load(t, scored(scored(g, "clm-bbbbbb", f(premise)), "clm-aaaaaa", f(1)))
	rec, ok := ShowPayload(ix, "clm-aaaaaa")
	if !ok {
		t.Fatal("show clm-aaaaaa: no record")
	}
	var got any
	for _, f := range rec {
		if f.Key == "strength" {
			got = f.Value
		}
	}
	if got != 0.18 {
		t.Errorf("show strength = %v, want the display rounding 0.18", got)
	}
	if n, ok := ix.Node("clm-aaaaaa"); !ok || n.Strength == nil || *n.Strength != exact {
		t.Errorf("the index row's strength = %v, want the exact %v", n.Strength, exact)
	}
}

// TestShowAnAxiomCarriesItsRigor: an axiom's record carries its rigor, the
// row's 1.0 by rule, and nothing named strength, and a work's record is
// unchanged.
func TestShowAnAxiomCarriesItsRigor(t *testing.T) {
	ix := load(t, kb.Graph{Axioms: []kb.AxiomNode{{ID: "axm-S1", Title: "The design rule"}},
		Works: []kb.WorkNode{{ID: "wrk-A", Title: "W", Rigor: f(0.5)}}})
	idFields := func(kind, id, title, path, anchor string) result.Record {
		return result.Record{{Key: "node_type", Value: kind}, {Key: "id", Value: id}, {Key: "title", Value: title},
			{Key: "canonical_path", Value: path}, {Key: "canonical_anchor", Value: anchor}}
	}
	for _, tc := range []struct {
		id   string
		want result.Record
	}{
		{"axm-S1", append(idFields(kb.NodeKindAxiom, "axm-S1", "The design rule", "axioms.md", "axiom-s1-the-design-rule"),
			result.Field{Key: "rigor", Value: kb.AxiomRigor})},
		{"wrk-A", append(idFields(kb.NodeKindWork, "wrk-A", "W", kb.RegisterFile, "w"), result.Field{Key: "rigor", Value: 0.5})},
	} {
		got, ok := ShowPayload(ix, tc.id)
		if !ok {
			t.Fatalf("show %s: no record", tc.id)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("show %s = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestAnUnhostedNodeHasNoCanonicalPathOrAnchor: every surface carrying a node
// record — show and strength-below — writes an unhosted node's canonical path
// and anchor as null, and a hosted node's as its row holds them.
func TestAnUnhostedNodeHasNoCanonicalPathOrAnchor(t *testing.T) {
	g := kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "X", Status: kb.ExperimentRun}}}
	ix := load(t, scored(scored(scored(g, "clm-aaaaaa", f(0.3)), "clm-bbbbbb", f(0.3)), "ana-aaaaaa", f(0.7)),
		"clm-aaaaaa", "ana-aaaaaa", "exp-aaaaaa")
	canonical := func(rec result.Record) [2]any {
		var out [2]any
		for _, field := range rec {
			switch field.Key {
			case "canonical_path":
				out[0] = field.Value
			case "canonical_anchor":
				out[1] = field.Value
			}
		}
		return out
	}
	hosted := [2]any{"main/claim-rigor.md", "b"}
	for _, tc := range []struct {
		id   string
		want [2]any
	}{{"clm-aaaaaa", [2]any{}}, {"clm-bbbbbb", hosted}, {"ana-aaaaaa", [2]any{}}, {"exp-aaaaaa", [2]any{}}} {
		rec, ok := ShowPayload(ix, tc.id)
		if !ok {
			t.Fatalf("show %s: no record", tc.id)
		}
		if got := canonical(rec); got != tc.want {
			t.Errorf("show %s canonical path and anchor = %#v, want %#v", tc.id, got, tc.want)
		}
	}
	got := map[string][2]any{}
	for _, rec := range StrengthBelowPayload(ix, 0.5) {
		got[rec[1].Value.(string)] = canonical(rec)
	}
	if want := map[string][2]any{"clm-aaaaaa": {}, "clm-bbbbbb": hosted}; !maps.Equal(got, want) {
		t.Errorf("strength-below canonical paths and anchors = %#v, want %#v", got, want)
	}
}

func TestStatsCensusPartitionsClaims(t *testing.T) {
	g := kb.Graph{Claims: []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B")},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "X", Status: kb.ExperimentRun}},
		Works:       []kb.WorkNode{{ID: "wrk-A", Title: "W"}},
		Axioms:      []kb.AxiomNode{{ID: "axm-S1", Title: "T"}}}
	ix := load(t, g)
	census := StatsPayload(ix)[:len(kb.NodeKinds)]
	total := 0
	var keys []string
	for _, field := range census {
		total += field.Value.(int)
		keys = append(keys, field.Key)
	}
	if want := len(index.GraphKinds(g)); total != want {
		t.Errorf("census sums to %d, want %d", total, want)
	}
	if want := []string{"claims", "analyses", "experiments", "axioms", "works"}; !slices.Equal(keys, want) {
		t.Errorf("census keys %q, want %q", keys, want)
	}
}

func TestStatsCountsDemotedEdges(t *testing.T) {
	demoted := func(source, relation string) kb.DemotedEdge {
		return kb.DemotedEdge{Source: source, Target: "clm-aaaaaa", ReplacedRelation: relation, Bearing: kb.Bearing{Pending: true}}
	}
	ix := load(t, kb.Graph{
		Claims:     []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C")},
		Scorable:   []kb.ScorableEdge{scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", 1)},
		Demoted:    []kb.DemotedEdge{demoted("clm-bbbbbb", kb.RelationRefutes), demoted("clm-cccccc", kb.RelationSupports)},
		References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-cccccc"}},
	})
	counts := map[string]any{}
	for _, field := range StatsPayload(ix) {
		counts[field.Key] = field.Value
	}
	if counts["edges"] != 4 || counts["demoted_edges"] != 2 {
		t.Errorf("edges %v, demoted_edges %v; want 4 and 2", counts["edges"], counts["demoted_edges"])
	}
}

// edgeCensusIndex holds both bearing classes on supports — a number, one whose
// text sorts after another's (1e-05 after 0.5), and pending — one row each on
// founds, lifts and references, and none on refutes or demoted. One supports
// row carries a context.
func edgeCensusIndex(t *testing.T) *Index {
	t.Helper()
	withContext := scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", 0.5)
	withContext.Context = "step 2"
	pending := scorable(kb.RelationSupports, "clm-dddddd", "clm-aaaaaa", 0)
	pending.Bearing = kb.Bearing{Pending: true}
	return load(t, kb.Graph{
		Claims:   []kb.ClaimNode{claim("clm-aaaaaa", "A"), claim("clm-bbbbbb", "B"), claim("clm-cccccc", "C"), claim("clm-dddddd", "D")},
		Analyses: []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "S", Rigor: f(0.8)}},
		Works:    []kb.WorkNode{{ID: "wrk-B", Title: "B"}},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-cccccc", "clm-bbbbbb", 0.5),
			withContext,
			scorable(kb.RelationSupports, "clm-cccccc", "clm-aaaaaa", 1e-05),
			scorable(kb.RelationLifts, "ana-aaaaaa", "clm-aaaaaa", 0.8),
			pending,
		},
		Founds:     []kb.BearingEdge{{Source: "wrk-B", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.5}}},
		References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb"}},
	})
}

// manualCounts is a bearings record whose rows carry their manual and model
// counts, four values per row.
func manualCounts(rows ...any) []result.Record {
	out := []result.Record{}
	for i := 0; i < len(rows); i += 4 {
		out = append(out, result.Record{{Key: "bearing", Value: rows[i]}, {Key: "edges", Value: rows[i+1]},
			{Key: "manual", Value: rows[i+2]}, {Key: "model", Value: rows[i+3]}})
	}
	return out
}

func bearingCounts(pairs ...any) []result.Record {
	out := []result.Record{}
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, result.Record{{Key: "bearing", Value: pairs[i]}, {Key: "edges", Value: pairs[i+1]}})
	}
	return out
}

func TestEdgeCensus(t *testing.T) {
	got := EdgeCensusPayload(edgeCensusIndex(t))
	want := result.Record{
		{Key: kb.RelationSupports, Value: result.Record{{Key: "edges", Value: 4}, {Key: "walked", Value: true},
			{Key: "bearings", Value: manualCounts("1e-05", 1, 0, 1, "0.5", 2, 0, 2, kb.PendingLiteral, 1, 0, 1)}}},
		{Key: kb.RelationFounds, Value: result.Record{{Key: "edges", Value: 1}, {Key: "walked", Value: false},
			{Key: "bearings", Value: bearingCounts("0.5", 1)}}},
		{Key: kb.RelationLifts, Value: result.Record{{Key: "edges", Value: 1}, {Key: "walked", Value: true},
			{Key: "bearings", Value: manualCounts("0.8", 1, 0, 1)}}},
		{Key: kb.RelationRefutes, Value: result.Record{{Key: "edges", Value: 0}, {Key: "walked", Value: true},
			{Key: "bearings", Value: []result.Record{}}}},
		{Key: kb.RelationReferences, Value: result.Record{{Key: "edges", Value: 1}, {Key: "walked", Value: false}}},
		{Key: kb.RelationDemoted, Value: result.Record{{Key: "edges", Value: 0}, {Key: "walked", Value: false}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("edge-census = %v, want %v", got, want)
	}
}

// edgeRows is an edges answer as "source relation target=bearing context
// walked" lines.
func edgeRows(t *testing.T, records []result.Record) []string {
	t.Helper()
	var out []string
	for _, r := range records {
		var keys []string
		for _, f := range r {
			keys = append(keys, f.Key)
		}
		if want := []string{"source", "relation", "target", "bearing", manuallyScoredKey, "context", "walked"}; !slices.Equal(keys, want) {
			t.Fatalf("edges record keys %q, want %q", keys, want)
		}
		out = append(out, fmt.Sprintf("%v %v %v=%v %v %v", r[0].Value, r[1].Value, r[2].Value, r[3].Value, r[5].Value, r[6].Value))
	}
	return out
}

func TestEdges(t *testing.T) {
	ix := edgeCensusIndex(t)
	every := []string{
		"clm-bbbbbb supports clm-aaaaaa=0.5 step 2 true",
		"clm-cccccc supports clm-aaaaaa=1e-05 <nil> true",
		"clm-cccccc supports clm-bbbbbb=0.5 <nil> true",
		"clm-dddddd supports clm-aaaaaa=*pending* <nil> true",
		"wrk-B founds clm-aaaaaa=0.5 <nil> false",
		"ana-aaaaaa lifts clm-aaaaaa=0.8 <nil> true",
		"clm-aaaaaa references clm-bbbbbb=<nil> <nil> false",
	}
	for _, tc := range []struct {
		name      string
		relations []string
		want      []string
	}{
		{"every relation", nil, every},
		{"founds", []string{kb.RelationFounds}, every[4:5]},
		{"two relations", []string{kb.RelationReferences, kb.RelationLifts}, every[5:]},
		{"a relation with no rows", []string{kb.RelationRefutes}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := edgeRows(t, EdgesPayload(ix, tc.relations)); !slices.Equal(got, tc.want) {
				t.Errorf("edges %v = %q, want %q", tc.relations, got, tc.want)
			}
		})
	}
}

// sheetFixture is the 3.0.0 KB the sheet goldens are drawn from.
var sheetFixture = filepath.Join("..", "sheet", "testdata", "adjagent-sheet", "kb-root")

// editedSheetKB is a copy of the sheet fixture whose records edit has changed,
// written through kb's record encoders.
func editedSheetKB(t *testing.T, edit func(*kb.Graph)) *kb.Source {
	t.Helper()
	root := filepath.Join(t.TempDir(), "kb-root")
	if err := os.CopyFS(root, os.DirFS(sheetFixture)); err != nil {
		t.Fatal(err)
	}
	src := kb.OnDisk(root)
	g, err := kb.LoadAuthoredGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	edit(&g)
	for rel, data := range map[string][]byte{kb.GraphNodesFile: kb.EncodeNodes(g), kb.GraphEdgesFile: kb.EncodeEdges(g)} {
		if err := os.WriteFile(src.KBPath(rel), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return kb.OnDisk(root)
}

// TestLoadAnswersFromTheRecords: Load holds every node the KB's records hold.
func TestLoadAnswersFromTheRecords(t *testing.T) {
	src := kb.OnDisk(sheetFixture)
	g, err := kb.LoadGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Sorted(maps.Keys(index.GraphKinds(g)))
	ix, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := ids(ix.Nodes()); len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("Load holds %v, want the records' %v", got, want)
	}
}

// TestLoadAnswersAConvertedKB: a KB converted at load and not yet saved
// answers, holding the nodes it holds read as current.
func TestLoadAnswersAConvertedKB(t *testing.T) {
	current, err := Load(kb.OnDisk(sheetFixture))
	if err != nil {
		t.Fatal(err)
	}
	converted, err := Load(kb.MigratedSource(sheetFixture, "2.0.1", nil, nil))
	if err != nil {
		t.Fatalf("Load of a converted, unsaved KB: %v", err)
	}
	if got, want := ids(converted.Nodes()), ids(current.Nodes()); !slices.Equal(got, want) {
		t.Errorf("converted KB holds %v, want %v", got, want)
	}
}

// TestLoadRefusesADegradedKB: a KB whose load is degraded — here a premise
// cycle the walk cuts — is refused, one check: degraded item per fault, each
// with remedy kbase refresh.
func TestLoadRefusesADegradedKB(t *testing.T) {
	src := editedSheetKB(t, func(g *kb.Graph) {
		g.Scorable = append(g.Scorable, kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-aaa003", Target: "clm-aaa002",
			Bearing: kb.Bearing{Value: 1}})
	})
	_, err := Load(src)
	var r result.Refusal
	if !errors.As(err, &r) || len(r) == 0 {
		t.Fatalf("Load over a cycle = %v, want a refusal", err)
	}
	for _, item := range r {
		if item.Check != kb.CheckDegraded || item.Remedy != index.RefreshRemedy || item.Path != kb.GraphEdgesFile || item.Detail == "" {
			t.Errorf("refusal item %+v, want check %s, path %s, remedy %s and a detail", item, kb.CheckDegraded, kb.GraphEdgesFile, index.RefreshRemedy)
		}
	}
}

// manualSheetKB is the sheet fixture with clm-aaa001, exp-aaa008 and the
// supports edge clm-aaa001 → clm-aaa003 manually scored, and a second demoted
// record of clm-aaa004 → clm-aaa001, replacing a refutation, manually scored:
// a pair two demoted records show alike.
func manualSheetKB(t *testing.T) *Index {
	t.Helper()
	src := editedSheetKB(t, func(g *kb.Graph) {
		for i := range g.Claims {
			g.Claims[i].ManuallyScored = g.Claims[i].ID == "clm-aaa001"
		}
		for i := range g.Experiments {
			g.Experiments[i].ManuallyScored = g.Experiments[i].ID == "exp-aaa008"
		}
		for i, e := range g.Scorable {
			g.Scorable[i].ManuallyScored = e.Relation == kb.RelationSupports && e.Source == "clm-aaa001" && e.Target == "clm-aaa003"
		}
		g.Demoted = append(g.Demoted, kb.DemotedEdge{Source: "clm-aaa004", Target: "clm-aaa001", ReplacedRelation: kb.RelationRefutes,
			Bearing: kb.Bearing{Value: 0.5}, ManuallyScored: true})
	})
	ix, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// fieldAt is the value of rec's field key and its position, -1 where rec has
// none.
func fieldAt(rec result.Record, key string) (any, int) {
	for i, f := range rec {
		if f.Key == key {
			return f.Value, i
		}
	}
	return nil, -1
}

// TestShowCarriesManuallyScored: a claim's, an analysis's and an experiment's
// record carry manually_scored right after rigor; a work's and an axiom's
// carry none.
func TestShowCarriesManuallyScored(t *testing.T) {
	ix := manualSheetKB(t)
	for _, tc := range []struct {
		id   string
		want any
	}{
		{"clm-aaa001", true},
		{"clm-aaa002", false},
		{"ana-aaa007", false},
		{"exp-aaa008", true},
		{"wrk-smith2020", nil},
		{"axm-S1", nil},
	} {
		rec, ok := ShowPayload(ix, tc.id)
		if !ok {
			t.Fatalf("show %s: no record", tc.id)
		}
		got, at := fieldAt(rec, manuallyScoredKey)
		_, rigor := fieldAt(rec, "rigor")
		switch {
		case tc.want == nil && at != -1:
			t.Errorf("show %s carries manually_scored: %v", tc.id, rec)
		case tc.want != nil && (got != tc.want || at != rigor+1):
			t.Errorf("show %s: manually_scored %v at %d, want %v right after rigor at %d", tc.id, got, at, tc.want, rigor)
		}
	}
}

// TestDepsAndEdgesCarryManuallyScored: an edge record carries its record's
// manually-scored value after its bearing, null on a relation whose records
// carry none, and two demoted rows showing one pair carry their two records'
// values.
func TestDepsAndEdgesCarryManuallyScored(t *testing.T) {
	ix := manualSheetKB(t)
	var deps []string
	for _, rec := range DepsPayload(ix, "clm-aaa004", false).([]result.Record) {
		if rec[4].Key != manuallyScoredKey {
			t.Fatalf("deps record %v: field 5 is %s, want %s", rec, rec[4].Key, manuallyScoredKey)
		}
		deps = append(deps, fmt.Sprint(rec[0].Value, " ", rec[1].Value, " ", rec[2].Value, " ", rec[4].Value))
	}
	if want := []string{
		"lifts in ana-aaa007 false",
		"references out clm-bbb003 <nil>",
		"demoted out clm-aaa001 false",
		"demoted out clm-aaa001 true",
		"demoted out clm-bbb001 false",
	}; !slices.Equal(deps, want) {
		t.Errorf("deps clm-aaa004 = %q, want %q", deps, want)
	}
	manual := map[string]any{}
	for _, rec := range EdgesPayload(ix, []string{kb.RelationSupports, kb.RelationFounds}) {
		if rec[4].Key != manuallyScoredKey {
			t.Fatalf("edges record %v: field 5 is %s, want %s", rec, rec[4].Key, manuallyScoredKey)
		}
		manual[fmt.Sprint(rec[0].Value, " ", rec[1].Value, " ", rec[2].Value)] = rec[4].Value
	}
	for name, want := range map[string]any{
		"clm-aaa001 supports clm-aaa003": true,
		"clm-aaa001 supports clm-bbb001": false,
		"axm-S1 founds clm-aaa001":       nil,
	} {
		if got, ok := manual[name]; !ok || got != want {
			t.Errorf("edges %s: manually_scored %v (present %v), want %v", name, got, ok, want)
		}
	}
}

// TestStatsCountsManualAndModelScores: the fixture's flagged records are its
// nine claims, one analysis and one experiment, and its seven scorable and
// three demoted edges; two of each are manually scored.
func TestStatsCountsManualAndModelScores(t *testing.T) {
	stats := StatsPayload(manualSheetKB(t))
	_, at := fieldAt(stats, "subtree_aggregates")
	var got []string
	for _, f := range stats[at+1 : at+5] {
		got = append(got, fmt.Sprint(f.Key, "=", f.Value))
	}
	if want := []string{"rigor_manual=2", "rigor_model=9", "bearing_manual=2", "bearing_model=8"}; !slices.Equal(got, want) {
		t.Errorf("stats after subtree_aggregates = %q, want %q", got, want)
	}
}

// TestEdgeCensusCountsManualAndModelBearings: a bearings record on supports,
// lifts and refutes counts its manual and model rows after edges; founds'
// carries no such count.
func TestEdgeCensusCountsManualAndModelBearings(t *testing.T) {
	census := EdgeCensusPayload(manualSheetKB(t))
	bearings := func(relation string) []result.Record {
		rec, _ := fieldAt(census, relation)
		b, _ := fieldAt(rec.(result.Record), "bearings")
		return b.([]result.Record)
	}
	for relation, want := range map[string][]result.Record{
		kb.RelationSupports: manualCounts("1.0", 4, 1, 3),
		kb.RelationLifts:    manualCounts("0.5", 1, 0, 1, "0.8", 1, 0, 1),
		kb.RelationRefutes:  manualCounts("0.5", 1, 0, 1),
		kb.RelationFounds:   bearingCounts("0.5", 1, "0.75", 1, "0.8", 1, "1.0", 3),
	} {
		if got := bearings(relation); !reflect.DeepEqual(got, want) {
			t.Errorf("edge-census %s bearings = %v, want %v", relation, got, want)
		}
	}
}
