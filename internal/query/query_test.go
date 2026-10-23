package query

import (
	"encoding/json"
	"errors"
	"fmt"
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

func claim(id, title string, strength *float64, band string) map[string]any {
	return map[string]any{"node_type": "claim", "id": id, "title": title, "canonical_path": "main/claim-rigor.md",
		"canonical_anchor": strings.ToLower(strings.ReplaceAll(title, " ", "-")), "rigor": strength, "strength": strength,
		"build_status": nil, "build_band": band, "rationale": "", "premise_count": 0, "strengthen_by_count": 0, "citation_count": 1}
}

// edge is one index edge row. bearing is the text the writer emits, or nil
// where the pairing carries none.
func edge(source, target, relation string, targetKind string, bearing any) map[string]any {
	return map[string]any{"source": source, "target": target, "relation": relation, "target_kind": targetKind,
		"source_strength_recorded": nil, "bearing": bearing, "context": nil}
}

func cite(id, leaf string) map[string]any {
	return map[string]any{"claim_id": id, "leaf_path": leaf, "leaf_kind": "leaf", "tier2_marked": false}
}

func f(v float64) *float64 { return &v }

// writeIndex writes kbRoot/.index with the given records per file; files not
// named are written empty.
func writeIndex(t *testing.T, kbRoot string, files map[string][]map[string]any) {
	t.Helper()
	dir := filepath.Join(kbRoot, kb.IndexDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range requiredFiles {
		var b strings.Builder
		for _, rec := range files[name] {
			line, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			b.WriteString(kb.IndexRecordMarker)
			b.Write(line)
			b.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, kb.IndexFileName(name)), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func load(t *testing.T, files map[string][]map[string]any) *Index {
	t.Helper()
	root := t.TempDir()
	writeIndex(t, root, files)
	ix, err := Load(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
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
	ix := load(t, map[string][]map[string]any{"claims": {
		claim("clm-cccccc", "Proposition 4.4 — KPI-Gating as Discrete Pontryagin", f(0.55), "input-only"),
		claim("clm-aaaaaa", "Proposition 4.3 — Institutional Misalignment", f(0.55), "input-only"),
		claim("clm-bbbbbb", "Theorem 4.5 — Positive Invariance of M", f(0.9), "ok-to-build"),
		claim("clm-dddddd", "Lemma 1 — Straße", nil, "unknown"),
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

func leaf(front, body string) string {
	return "---\nkind: leaf\n" + front + "\n---\n[↑ Up](../index.md)\n\n# Leaf\n\n" + body + "\n"
}

func TestReferencedBy(t *testing.T) {
	root := t.TempDir()
	writeIndex(t, root, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "Origin Claim", f(0.5), "input-only")},
		"cites":  {cite("clm-aaaaaa", "main/origin.md")},
	})
	writeFile(t, filepath.Join(root, "main/origin.md"), leaf("claims: [clm-aaaaaa]", "Self link [here](origin.md)."))
	writeFile(t, filepath.Join(root, "main/citing.md"), leaf(`no-claim: "links"`, "See [the origin](origin.md#sec) for it."))
	writeFile(t, filepath.Join(root, "other/deep.md"), leaf(`no-claim: "links"`, "Back [up](../main/origin.md)."))
	writeFile(t, filepath.Join(root, "main/coded.md"), leaf(`no-claim: "code"`, "Only `[x](origin.md)` in code."))
	writeFile(t, filepath.Join(root, "main/index.md"), "---\nkind: index\n---\n\n# Index\n\n[origin](origin.md)\n")
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

func TestStrengthBelowAndWeakPoints(t *testing.T) {
	ix := load(t, map[string][]map[string]any{
		"claims": {
			claim("clm-aaaaaa", "A", f(0.5), "input-only"),
			claim("clm-bbbbbb", "B", f(0.3), "do-not-build"),
			claim("clm-cccccc", "C", f(0.5), "input-only"),
			claim("clm-dddddd", "D", nil, "unknown"),
			claim("clm-eeeeee", "E", f(0.9), "ok-to-build"),
		},
		"edges": {
			edge("clm-aaaaaa", "clm-dddddd", kb.RelationSupports, "claim", "1.0"),
			edge("clm-eeeeee", "clm-aaaaaa", kb.RelationReferences, "claim", nil),
			edge("clm-aaaaaa", "clm-eeeeee", kb.RelationSupports, "claim", "1.0"),
			edge("clm-bbbbbb", "clm-dddddd", kb.RelationSupports, "claim", "1.0"),
			edge("clm-cccccc", "clm-dddddd", kb.RelationSupports, "claim", "1.0"),
		},
	})
	if got, want := ids(ix.StrengthBelow(0.65)), []string{"clm-bbbbbb", "clm-aaaaaa", "clm-cccccc"}; !slices.Equal(got, want) {
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
	ix := load(t, map[string][]map[string]any{
		"claims": {
			claim("clm-aaaaaa", "A", nil, "unknown"),
			claim("clm-bbbbbb", "B", nil, "unknown"),
			claim("clm-cccccc", "C", nil, "unknown"),
			claim("clm-dddddd", "D", nil, "unknown"),
			claim("clm-eeeeee", "E", nil, "unknown"),
			claim("clm-ffffff", "F", nil, "unknown"),
			claim("clm-gggggg", "G", nil, "unknown"),
			claim("clm-hhhhhh", "H", nil, "unknown"),
			{"node_type": "experiment", "id": "exp-aaaaaa", "title": "X", "canonical_path": "p", "canonical_anchor": "x"},
			{"node_type": "analysis", "id": "ana-aaaaaa", "title": "S", "canonical_path": "p", "canonical_anchor": "s"},
		},
		"edges": {
			edge("clm-aaaaaa", "clm-dddddd", kb.RelationSupports, "claim", "1.0"),
			edge("clm-aaaaaa", "clm-eeeeee", kb.RelationSupports, "claim", "1.0"),
			edge("clm-aaaaaa", "clm-dddddd", kb.RelationSupports, "claim", "1.0"),
			edge("clm-aaaaaa", "clm-ffffff", kb.RelationReferences, "claim", nil),
			edge("clm-aaaaaa", "clm-gggggg", kb.RelationDemoted, "claim", nil),
			edge("exp-aaaaaa", "clm-aaaaaa", kb.RelationLifts, "claim", "1.0"),
			edge("ana-aaaaaa", "clm-aaaaaa", kb.RelationLifts, "claim", "1.0"),
			edge("clm-bbbbbb", "clm-aaaaaa", kb.RelationReferences, "claim", nil),
			edge("clm-cccccc", "clm-aaaaaa", kb.RelationDemoted, "claim", nil),
			edge("clm-aaaaaa", "clm-hhhhhh", kb.RelationRefutes, "claim", "1.0"),
		}})
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

func TestDepsRows(t *testing.T) {
	ix := load(t, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "A", nil, "unknown"), claim("clm-bbbbbb", "B", nil, "unknown"),
			claim("clm-cccccc", "C", nil, "unknown"),
			{"node_type": "axiom", "id": "axm-S1", "title": "The design rule", "canonical_path": "axioms.md",
				"canonical_anchor": "axiom-s1-the-design-rule"},
			{"node_type": "axiom", "id": "axm-S2", "title": "The other rule", "canonical_path": "axioms.md",
				"canonical_anchor": "axiom-s2-the-other-rule"},
			{"node_type": "analysis", "id": "ana-aaaaaa", "title": "S", "canonical_path": "p", "canonical_anchor": "s", "rigor": 0.8, "strength": 0.8},
			{"node_type": "work", "id": "wrk-B", "title": "B", "canonical_path": "p", "canonical_anchor": "b"},
			{"node_type": "work", "id": "wrk-C", "title": "C", "canonical_path": "p", "canonical_anchor": "c"},
			{"node_type": "work", "id": "wrk-Z", "title": "Z", "canonical_path": "p", "canonical_anchor": "z"}},
		"edges": {
			edge("wrk-Z", "clm-aaaaaa", kb.RelationFounds, "claim", "*pending*"),
			edge("ana-aaaaaa", "clm-aaaaaa", kb.RelationSupports, "claim", "0.4"),
			edge("clm-aaaaaa", "ana-aaaaaa", kb.RelationSupports, "analysis", "0.6"),
			edge("ana-aaaaaa", "clm-cccccc", kb.RelationLifts, "claim", "0.8"),
			edge("wrk-B", "clm-aaaaaa", kb.RelationFounds, "claim", "0.5"),
			edge("wrk-C", "clm-aaaaaa", kb.RelationFounds, "claim", "0"),
			edge("axm-S1", "clm-aaaaaa", kb.RelationFounds, "claim", "1.0"),
			edge("axm-S2", "clm-aaaaaa", kb.RelationFounds, "claim", "*pending*"),
			edge("clm-aaaaaa", "clm-bbbbbb", kb.RelationReferences, "claim", nil),
		},
	})
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"clm-aaaaaa", []string{
			"supports in ana-aaaaaa=0.4",
			"supports out ana-aaaaaa=0.6",
			"founds in axm-S1=1.0",
			"founds in axm-S2=*pending*",
			"founds in wrk-B=0.5",
			"founds in wrk-C=0",
			"founds in wrk-Z=*pending*",
			"references out clm-bbbbbb=<nil>",
		}},
		{"ana-aaaaaa", []string{"supports in clm-aaaaaa=0.6", "supports out clm-aaaaaa=0.4", "lifts out clm-cccccc=0.8"}},
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
	ix := load(t, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "A", nil, "unknown"), claim("clm-bbbbbb", "B", nil, "unknown"),
			{"node_type": "analysis", "id": "ana-aaaaaa", "title": "S", "canonical_path": "p", "canonical_anchor": "s", "rigor": 0.8, "strength": 0.8},
			{"node_type": "analysis", "id": "ana-bbbbbb", "title": "T", "canonical_path": "p", "canonical_anchor": "t", "rigor": 0.8, "strength": 0.8},
			{"node_type": "experiment", "id": "exp-aaaaaa", "title": "X", "canonical_path": "p", "canonical_anchor": "x"}},
		"edges": {
			edge("clm-aaaaaa", "clm-bbbbbb", kb.RelationRefutes, "claim", "1.0"),
			edge("exp-aaaaaa", "ana-aaaaaa", kb.RelationLifts, "analysis", "0.9"),
			edge("exp-aaaaaa", "ana-bbbbbb", kb.RelationRefutes, "analysis", "0.25"),
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

// TestDepsOrderIgnoresFileOrder: the same edges, written to the index in
// either order, answer the same rows.
func TestDepsOrderIgnoresFileOrder(t *testing.T) {
	first, second := edge("clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, "claim", "0.7"),
		edge("clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, "claim", "0.7")
	first["context"], second["context"] = "first use", "second use"
	back := edge("clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports, "claim", "0.7")
	claims := []map[string]any{claim("clm-aaaaaa", "A", nil, "unknown"), claim("clm-bbbbbb", "B", nil, "unknown")}
	inOrder := load(t, map[string][]map[string]any{"claims": claims, "edges": {first, second, back}})
	reversed := load(t, map[string][]map[string]any{"claims": claims, "edges": {back, second, first}})
	if got, want := DepsPayload(reversed, "clm-aaaaaa", false), DepsPayload(inOrder, "clm-aaaaaa", false); !reflect.DeepEqual(got, want) {
		t.Errorf("deps over the reversed index = %v, want %v", got, want)
	}
}

func TestDepsContext(t *testing.T) {
	withContext := edge("clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, "claim", "0.7")
	withContext["context"] = "used in step 2"
	ix := load(t, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "A", nil, "unknown"), claim("clm-bbbbbb", "B", nil, "unknown"), claim("clm-cccccc", "C", nil, "unknown")},
		"edges":  {withContext, edge("clm-cccccc", "clm-aaaaaa", kb.RelationSupports, "claim", "0.8")},
	})
	want := []result.Record{
		{{Key: "relation", Value: kb.RelationSupports}, {Key: "direction", Value: "in"}, {Key: "target", Value: "clm-bbbbbb"}, {Key: "bearing", Value: "0.7"}, {Key: "context", Value: "used in step 2"}},
		{{Key: "relation", Value: kb.RelationSupports}, {Key: "direction", Value: "in"}, {Key: "target", Value: "clm-cccccc"}, {Key: "bearing", Value: "0.8"}, {Key: "context", Value: nil}},
	}
	if got := DepsPayload(ix, "clm-aaaaaa", false); !reflect.DeepEqual(got, want) {
		t.Errorf("deps = %v, want %v", got, want)
	}
}

func TestShowStrengthenBy(t *testing.T) {
	ix := load(t, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "A", nil, "unknown"), claim("clm-bbbbbb", "B", nil, "unknown"),
			{"node_type": "work", "id": "wrk-A", "title": "W", "canonical_path": "p", "canonical_anchor": "w", "rigor": 0.5}},
		"strengthen-by": {
			{"claim_id": "clm-aaaaaa", "item_idx": 1, "text": "Bound the remainder.", "mentioned_ids": []string{}},
			{"claim_id": "clm-bbbbbb", "item_idx": 0, "text": "Other claim's item.", "mentioned_ids": []string{}},
			{"claim_id": "clm-aaaaaa", "item_idx": 0, "text": "Prove clm-bbbbbb first.", "mentioned_ids": []string{"clm-bbbbbb", "wrk-A"}},
		},
	})
	field := func(rec result.Record, key string) (any, int) {
		for i, f := range rec {
			if f.Key == key {
				return f.Value, i
			}
		}
		return nil, -1
	}
	rec, _ := ShowPayload(ix, "clm-aaaaaa")
	items, at := field(rec, "strengthen_by")
	if _, count := field(rec, "strengthen_by_count"); at != count+1 {
		t.Errorf("strengthen_by at %d, want right after strengthen_by_count at %d", at, count)
	}
	want := []result.Record{
		{{Key: "item_idx", Value: 0}, {Key: "text", Value: "Prove clm-bbbbbb first."}, {Key: "mentioned_ids", Value: []string{"clm-bbbbbb", "wrk-A"}}},
		{{Key: "item_idx", Value: 1}, {Key: "text", Value: "Bound the remainder."}, {Key: "mentioned_ids", Value: []string{}}},
	}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("show strengthen_by = %v, want %v", items, want)
	}
	work, _ := ShowPayload(ix, "wrk-A")
	if _, at := field(work, "strengthen_by"); at != -1 {
		t.Errorf("work record carries strengthen_by: %v", work)
	}
	for _, c := range StrengthBelowPayload(load(t, map[string][]map[string]any{"claims": {claim("clm-aaaaaa", "A", f(0.1), "do-not-build")}}), 1) {
		if _, at := field(c, "strengthen_by"); at != -1 {
			t.Errorf("strength-below record carries strengthen_by: %v", c)
		}
	}
}

func TestSubtreeClaims(t *testing.T) {
	ix := load(t, map[string][]map[string]any{"subtree-aggregates": {
		{"node_path": "entry-point.md", "node_kind": "entry-point", "subtree_claims": []string{"clm-aaaaaa", "clm-bbbbbb"}},
		{"node_path": "vol/index.md", "node_kind": "index", "subtree_claims": []string{"clm-bbbbbb"}},
	}})
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
	ix := load(t, map[string][]map[string]any{"claims": {
		{"node_type": "analysis", "id": "ana-aaaaaa", "title": "S", "canonical_path": "p", "canonical_anchor": "s", "rigor": 0.7, "strength": 0.7},
	}})
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
	exact := 0.2 * 0.9
	ix := load(t, map[string][]map[string]any{"claims": {claim("clm-aaaaaa", "A", f(exact), "do-not-build")}})
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
	ix := load(t, map[string][]map[string]any{"claims": {
		{"node_type": kb.NodeKindAxiom, "id": "axm-S1", "title": "The design rule", "canonical_path": "axioms.md",
			"canonical_anchor": "axiom-s1-the-design-rule", "rigor": kb.AxiomRigor},
		{"node_type": kb.NodeKindWork, "id": "wrk-A", "title": "W", "canonical_path": "p", "canonical_anchor": "w", "rigor": 0.5},
	}})
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
		{"wrk-A", append(idFields(kb.NodeKindWork, "wrk-A", "W", "p", "w"), result.Field{Key: "rigor", Value: 0.5})},
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

func TestStatsCensusPartitionsClaims(t *testing.T) {
	recs := []map[string]any{claim("clm-aaaaaa", "A", nil, "unknown")}
	for _, kind := range kb.NodeKinds[1:] {
		recs = append(recs, map[string]any{"node_type": kind, "id": kind + "-x", "title": "T", "canonical_path": "p",
			"canonical_anchor": "a", "status": "run"})
	}
	ix := load(t, map[string][]map[string]any{"claims": recs})
	census := StatsPayload(ix)[:len(kb.NodeKinds)]
	total := 0
	var keys []string
	for _, field := range census {
		total += field.Value.(int)
		keys = append(keys, field.Key)
	}
	if total != len(recs) {
		t.Errorf("census sums to %d, want %d", total, len(recs))
	}
	if want := []string{"claims", "analyses", "experiments", "axioms", "works"}; !slices.Equal(keys, want) {
		t.Errorf("census keys %q, want %q", keys, want)
	}
}

func TestStatsCountsDemotedEdges(t *testing.T) {
	ix := load(t, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "A", nil, "unknown"), claim("clm-bbbbbb", "B", nil, "unknown"), claim("clm-cccccc", "C", nil, "unknown")},
		"edges": {
			edge("clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, "claim", "1.0"),
			edge("clm-bbbbbb", "clm-aaaaaa", kb.RelationDemoted, "claim", nil),
			edge("clm-aaaaaa", "clm-cccccc", kb.RelationReferences, "claim", nil),
			edge("clm-cccccc", "clm-aaaaaa", kb.RelationDemoted, "claim", nil),
		},
	})
	counts := map[string]any{}
	for _, field := range StatsPayload(ix) {
		counts[field.Key] = field.Value
	}
	if counts["edges"] != 4 || counts["demoted_edges"] != 2 {
		t.Errorf("edges %v, demoted_edges %v; want 4 and 2", counts["edges"], counts["demoted_edges"])
	}
}

// edgeCensusIndex holds every bearing class on supports — a number whose text
// sorts after another's (1e-05 after 0.5), pending, and a pairing that carries
// none — one row each on founds, lifts and references, and none on refutes or
// demoted. Two supports rows share their ends and differ by context.
func edgeCensusIndex(t *testing.T) *Index {
	t.Helper()
	withContext := edge("clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, "claim", "0.5")
	withContext["context"] = "step 2"
	return load(t, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "A", nil, "unknown"), claim("clm-bbbbbb", "B", nil, "unknown"),
			claim("clm-cccccc", "C", nil, "unknown"),
			{"node_type": "analysis", "id": "ana-aaaaaa", "title": "S", "canonical_path": "p", "canonical_anchor": "s", "rigor": 0.8, "strength": 0.8},
			{"node_type": "work", "id": "wrk-B", "title": "B", "canonical_path": "p", "canonical_anchor": "b"}},
		"edges": {
			edge("clm-cccccc", "clm-bbbbbb", kb.RelationSupports, "claim", "0.5"),
			edge("clm-aaaaaa", "clm-bbbbbb", kb.RelationReferences, "claim", nil),
			withContext,
			edge("clm-cccccc", "clm-aaaaaa", kb.RelationSupports, "claim", "1e-05"),
			edge("ana-aaaaaa", "clm-aaaaaa", kb.RelationLifts, "claim", "0.8"),
			edge("clm-aaaaaa", "wrk-B", kb.RelationSupports, "work", nil),
			edge("wrk-B", "clm-aaaaaa", kb.RelationFounds, "claim", "0.5"),
			edge("clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, "claim", "0.5"),
			edge("clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports, "claim", kb.PendingLiteral),
		},
	})
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
		{Key: kb.RelationSupports, Value: result.Record{{Key: "edges", Value: 6}, {Key: "walked", Value: true},
			{Key: "bearings", Value: bearingCounts("1e-05", 1, "0.5", 3, kb.PendingLiteral, 1, nil, 1)}}},
		{Key: kb.RelationFounds, Value: result.Record{{Key: "edges", Value: 1}, {Key: "walked", Value: false},
			{Key: "bearings", Value: bearingCounts("0.5", 1)}}},
		{Key: kb.RelationLifts, Value: result.Record{{Key: "edges", Value: 1}, {Key: "walked", Value: true},
			{Key: "bearings", Value: bearingCounts("0.8", 1)}}},
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
		if want := []string{"source", "relation", "target", "bearing", "context", "walked"}; !slices.Equal(keys, want) {
			t.Fatalf("edges record keys %q, want %q", keys, want)
		}
		out = append(out, fmt.Sprintf("%v %v %v=%v %v %v", r[0].Value, r[1].Value, r[2].Value, r[3].Value, r[4].Value, r[5].Value))
	}
	return out
}

func TestEdges(t *testing.T) {
	ix := edgeCensusIndex(t)
	every := []string{
		"clm-aaaaaa supports clm-bbbbbb=*pending* <nil> true",
		"clm-aaaaaa supports wrk-B=<nil> <nil> true",
		"clm-bbbbbb supports clm-aaaaaa=0.5 <nil> true",
		"clm-bbbbbb supports clm-aaaaaa=0.5 step 2 true",
		"clm-cccccc supports clm-aaaaaa=1e-05 <nil> true",
		"clm-cccccc supports clm-bbbbbb=0.5 <nil> true",
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
		{"founds", []string{kb.RelationFounds}, every[6:7]},
		{"two relations", []string{kb.RelationReferences, kb.RelationLifts}, every[7:]},
		{"a relation with no rows", []string{kb.RelationRefutes}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := edgeRows(t, EdgesPayload(ix, tc.relations)); !slices.Equal(got, tc.want) {
				t.Errorf("edges %v = %q, want %q", tc.relations, got, tc.want)
			}
		})
	}
}

func TestLoadRefusals(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(kb.OnDisk(root)); !isRefusal(err, len(requiredFiles)) {
		t.Errorf("empty kb-root: %v, want a refusal naming all %d files", err, len(requiredFiles))
	}
	writeIndex(t, root, map[string][]map[string]any{"claims": {{"node_type": "gadget", "id": "x"}}})
	writeFile(t, filepath.Join(root, kb.IndexDir, kb.IndexFileName("cites")), "--- [1]\n\n{\"claim_id\": \"unmarked\"}\n--- not json\n")
	if _, err := Load(kb.OnDisk(root)); !isRefusal(err, 4) {
		t.Errorf("bad lines: %v, want a refusal naming 4", err)
	}
	for _, tc := range []struct {
		name string
		edge map[string]any
	}{
		{"unknown relation", edge("clm-aaaaaa", "clm-bbbbbb", "nope", kb.NodeKindClaim, "0.5")},
		{"bearing as a number", edge("clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports, kb.NodeKindClaim, 0.5)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeIndex(t, root, map[string][]map[string]any{
				"claims": {claim("clm-aaaaaa", "A", f(0.5), "input-only"), claim("clm-bbbbbb", "B", f(0.5), "input-only")},
				"edges":  {tc.edge},
			})
			_, err := Load(kb.OnDisk(root))
			var r result.Refusal
			if !errors.As(err, &r) || len(r) != 1 || r[0].Check != index.CheckIndex || r[0].Remedy != index.RefreshRemedy {
				t.Errorf("Load = %v, want one %s refusal", err, index.CheckIndex)
			}
		})
	}
}

// TestLoadRefusesAnOrphanScoringRow: a scoring row whose source id resolves to
// no node is refused, rather than passing as a row whose pairing carries no
// bearing.
func TestLoadRefusesAnOrphanScoringRow(t *testing.T) {
	root := t.TempDir()
	writeIndex(t, root, map[string][]map[string]any{
		"claims": {claim("clm-aaaaaa", "A", f(0.5), "input-only"), claim("clm-bbbbbb", "B", f(0.5), "input-only")},
		"edges":  {edge("clm-ffffff", "clm-bbbbbb", kb.RelationSupports, kb.NodeKindClaim, "0.5")},
	})
	_, err := Load(kb.OnDisk(root))
	var r result.Refusal
	if !errors.As(err, &r) || len(r) != 1 || r[0].Check != index.CheckIndex || r[0].Key != "clm-ffffff → clm-bbbbbb" ||
		!strings.Contains(r[0].Detail, "source id the index does not hold") {
		t.Fatalf("Load over a scoring row naming no source node = %v, want one %s refusal naming the source", err, index.CheckIndex)
	}
}

func isRefusal(err error, items int) bool {
	var r result.Refusal
	return errors.As(err, &r) && len(r) == items
}
