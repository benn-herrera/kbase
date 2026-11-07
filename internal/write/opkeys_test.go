package write

import (
	"maps"
	"slices"
	"testing"
)

// specOpKey is one op's values-entry keys as its SPEC §8 leaf states them,
// each with whether an entry requires it. A key the leaf states as "at least
// one of" is optional here: no single one is required.
type specOpKey struct {
	leaf string
	keys map[string]bool
}

// documentedOpKeys is transcribed from the SPEC §8 leaves, one row per op the
// surfaces offer. A new op needs its row; a changed leaf needs its row changed.
var documentedOpKeys = map[string]specOpKey{
	AddClaim: {"add-kind.md", map[string]bool{
		"title": true, "rationale": true, "rigor": false, "document": true, "line": true, "snippet": true}},
	AddAnalysis: {"add-kind.md", map[string]bool{
		"title": true, "rationale": true, "rigor": false, "document": true, "line": true, "snippet": true}},
	AddExperiment: {"add-kind.md", map[string]bool{
		"title": true, "status": true, "rigor": false, "document": true, "line": true, "snippet": true}},
	AddWork: {"add-kind.md", map[string]bool{
		"title": true, "rationale": true, "rigor": false, "label": true}},
	AddAxiom: {"add-kind.md", map[string]bool{
		"title": true, "label": true}},
	EditClaim: {"edit-kind.md", map[string]bool{
		"id": true, "title": false, "rigor": false, "rationale": false}},
	EditAnalysis: {"edit-kind.md", map[string]bool{
		"id": true, "title": false, "rigor": false, "rationale": false}},
	EditExperiment: {"edit-kind.md", map[string]bool{
		"id": true, "title": false, "status": false, "rigor": false}},
	EditWork: {"edit-kind.md", map[string]bool{
		"id": true, "label": false, "title": false, "rigor": false, "rationale": false}},
	EditAxiom: {"edit-kind.md", map[string]bool{
		"id": true, "label": false, "title": false}},
	AddEdge: {"edge-ops.md", map[string]bool{
		"source": true, "target": true, "relation": true, "bearing": false, "context": false}},
	EditEdge: {"edge-ops.md", map[string]bool{
		"source": true, "target": true, "relation": true, "bearing": false, "context": false}},
	RemoveEdge: {"edge-ops.md", map[string]bool{
		"source": true, "target": true, "relation": true}},
	MoveNode: {"move-node.md", map[string]bool{
		"id": true, "document": true, "line": true, "snippet": true}},
	RemoveNode: {"remove-node.md", map[string]bool{
		"id": true}},
	AddCitation: {"add-citation.md", map[string]bool{
		"document": true, "line": true, "snippet": true, "id": true, "quote": false}},
	RestoreDemoted: {"restore-demoted.md", map[string]bool{
		"source": true, "target": true, "relation": true}},
}

// TestOpKeysMatchSpec compares every caller-issued op's key vocabulary with
// its SPEC §8 row, names and required-ness both ways, and every row with an
// op the surfaces offer.
func TestOpKeysMatchSpec(t *testing.T) {
	ops := Ops()
	for _, op := range ops {
		row, ok := documentedOpKeys[op]
		if !ok {
			t.Errorf("%s: no row in documentedOpKeys; transcribe its keys from its SPEC §8 leaf", op)
			continue
		}
		fields, ok := OpVocabulary(op)
		if !ok {
			t.Errorf("%s: Ops lists it and OpVocabulary knows no vocabulary for it", op)
			continue
		}
		got := map[string]bool{}
		for _, f := range fields {
			got[f.Name] = f.Required
		}
		for _, key := range slices.Sorted(maps.Keys(row.keys)) {
			required, has := got[key]
			switch {
			case !has:
				t.Errorf("%s: SPEC §8 %s states key %q, which the vocabulary lacks", op, row.leaf, key)
			case required != row.keys[key]:
				t.Errorf("%s: key %q required = %v, SPEC §8 %s states %v", op, key, required, row.leaf, row.keys[key])
			}
		}
		for _, key := range slices.Sorted(maps.Keys(got)) {
			if _, stated := row.keys[key]; !stated {
				t.Errorf("%s: key %q is in the vocabulary and SPEC §8 %s states none", op, key, row.leaf)
			}
		}
	}
	for _, op := range slices.Sorted(maps.Keys(documentedOpKeys)) {
		if !slices.Contains(ops, op) {
			t.Errorf("%s: documentedOpKeys has a row and Ops offers no such op", op)
		}
	}
}
