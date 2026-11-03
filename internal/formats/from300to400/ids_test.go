package from300to400

import (
	"bytes"
	"maps"
	"path"
	"reflect"
	"testing"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format300"
	"kbase/internal/formats/format400"
)

// TestMintedIDsAreDeterministic: one 3.0.0 tree converts to the same files and
// the same minted ids every time, and a work's or axiom's minted id is derived
// from its 3.0.0 id alone, so a tree holding more works gives it the same id.
func TestMintedIDsAreDeterministic(t *testing.T) {
	files := readTree(t, path.Join(testdata, "citations-3.0.0"))
	first, _, warnings, err := Convert300To400(files)
	if err != nil {
		t.Fatal(err)
	}
	again, _, againWarnings, err := Convert300To400(files)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.EqualFunc(first, again, bytes.Equal) || !reflect.DeepEqual(warnings, againWarnings) {
		t.Error("two conversions of one tree differ")
	}
	ids := map[string]string{}
	for _, w := range warnings {
		if w.List == common.ListIDs {
			ids[w.ID] = w.Text
		}
	}
	if len(ids) == 0 {
		t.Fatal("the conversion lists no minted id")
	}
	for from, to := range ids {
		kind, _ := format300.NodeIDKind(from)
		prefix := format400.WorkPrefix
		if kind == format300.NodeKindAxiom {
			prefix = format400.AxiomPrefix
		}
		alone := map[string]string{}
		mintIDs(prefix, []string{from}, alone, derivedID)
		if alone[from] != to {
			t.Errorf("%s converts to %s in the tree and to %s alone", from, to, alone[from])
		}
		if got, ok := format400.NodeIDKind(to); !ok || got != kind {
			t.Errorf("%s's minted id %s is no %s id", from, to, kind)
		}
	}
}

// TestAxiomHeadingsTakeTheirMarkers: each axiom heading naming an axiom
// record ends in that axiom's marker, and one naming none is left as written
// and reported at its line.
func TestAxiomHeadingsTakeTheirMarkers(t *testing.T) {
	files := common.Files{
		nodesPath:                []byte("- id: \"axm-A1\"\n  title: \"One\"\n"),
		edgesPath:                []byte("[]\n"),
		"kb-root/entry-point.md": []byte("# KB\n"),
		"kb-root/axioms.md":      []byte("### Axiom A1: One\n\nProse.\n\n### Axiom Z9: Orphan\n"),
	}
	got, _, warnings, err := Convert300To400(files)
	if err != nil {
		t.Fatal(err)
	}
	minted := map[string]string{}
	mintIDs(format400.AxiomPrefix, []string{"axm-A1"}, minted, derivedID)
	want := "### Axiom A1: One" + format400.StatementMarker(minted["axm-A1"]) + "\n\nProse.\n\n### Axiom Z9: Orphan\n"
	if string(got["kb-root/axioms.md"]) != want {
		t.Errorf("axioms.md converts to %q, want %q", got["kb-root/axioms.md"], want)
	}
	var reported []int
	for _, w := range warnings {
		if w.Path == "kb-root/axioms.md" {
			reported = append(reported, w.Line)
		}
	}
	if !reflect.DeepEqual(reported, []int{5}) {
		t.Errorf("axioms.md warned at lines %v, want the orphan heading's, 5", reported)
	}
}

// TestACollisionReDerivesInAscendingOrder: where two 3.0.0 ids derive one
// minted id, the lower 3.0.0 id takes it and the higher re-derives; a
// derivation that is itself a 3.0.0 id is re-derived too, so no minted id
// reads as an older one.
func TestACollisionReDerivesInAscendingOrder(t *testing.T) {
	derive := func(prefix, from string, n int) string {
		if n == 0 {
			return prefix + "-aaaaaa"
		}
		return derivedID(prefix, from, n)
	}
	out := map[string]string{}
	mintIDs(format400.WorkPrefix, []string{"wrk-Zeta2020", "wrk-Alpha2020"}, out, derive)
	want := map[string]string{"wrk-Alpha2020": "wrk-aaaaaa", "wrk-Zeta2020": derivedID(format400.WorkPrefix, "wrk-Zeta2020", 1)}
	if !maps.Equal(out, want) {
		t.Errorf("minted %v, want %v", out, want)
	}
	out = map[string]string{}
	mintIDs(format400.WorkPrefix, []string{"wrk-aaaaaa", "wrk-Alpha2020"}, out, derive)
	if out["wrk-Alpha2020"] == "wrk-aaaaaa" || out["wrk-aaaaaa"] == "wrk-aaaaaa" {
		t.Errorf("minted %v, giving a 3.0.0 id as a minted one", out)
	}
}
