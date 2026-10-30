package claimgraph

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
	"kbase/internal/write"
)

// The fixture's claims, keyed by the name the tests use, with the title each
// takes in its record and its leaf's prose.
var (
	elisionIDs    = map[string]string{"a": "clm-aaaaaa", "x": "clm-xaaaaa", "y": "clm-yaaaaa", "b": "clm-bbbbbb", "c": "clm-cccccc", "d": "clm-dddddd"}
	elisionTitles = map[string]string{"a": "A", "x": "X", "y": "Y", "b": "B", "c": "C", "d": "D"}
)

// seedElisionLeaves writes a minimal KB holding a claim record for every
// claim leaves names, and demoted's records: leaves[0] is leaf.md, leaves[i]
// leaf<i+1>.md, each marking its names in order in its prose. It then lands
// wires through AddBuildEdges, as the build's edge write does.
func seedElisionLeaves(t *testing.T, leaves [][]string, wires values, demoted ...kb.DemotedEdge) string {
	t.Helper()
	entry := "# Entry\n\n"
	titles := map[string]string{}
	docs := map[string]string{}
	for i, names := range leaves {
		rel, title := "leaf.md", "Leaf"
		if i > 0 {
			rel, title = fmt.Sprintf("leaf%d.md", i+1), fmt.Sprintf("Leaf %d", i+1)
		}
		entry += "- [" + title + "](" + rel + ")\n"
		prose := ""
		for _, n := range names {
			prose += "\n" + elisionTitles[n] + " holds here. <!-- claim-rigor: " + elisionIDs[n] + " -->\n"
			titles[elisionIDs[n]] = elisionTitles[n]
		}
		docs[rel] = "[↑ Entry](entry-point.md)\n\n# " + title + "\n" + prose
	}
	root := seedKB(t, entry, docs, kb.Graph{Claims: claimRecords(titles), Demoted: demoted})
	w := writer{kbRoot: root, lg: log.Discard()}
	if _, err := w.run("seed", write.AddBuildEdges, wires); err != nil {
		t.Fatal(err)
	}
	return root
}

// restsOn is the fixture's build-edge entry for claim from resting on each
// claim in to.
func dependsOn(from string, to ...string) map[string]any {
	items := make([]map[string]any, len(to))
	for i, n := range to {
		items[i] = map[string]any{"id": elisionIDs[n], "bearing": kb.PendingLiteral}
	}
	return map[string]any{"id": elisionIDs[from], kb.ListRestsOn: items}
}

// elisionWires is the chain a → x → b with c → a and b → d around it.
func elisionWires() values {
	return values{dependsOn("a", "x"), dependsOn("x", "b"), dependsOn("c", "a"), dependsOn("b", "d")}
}

// TestIssueCullsLeavesNoUnknownID drives the culls through the real write ops
// over the fixture's variants, then the stage gate: no line under kb-root
// names a culled node, and no culled node's record or marker stands.
func TestIssueCullsLeavesNoUnknownID(t *testing.T) {
	xDemotedIntoC := kb.DemotedEdge{Source: elisionIDs["x"], Target: elisionIDs["c"], ReplacedRelation: kb.RelationSupports,
		Bearing: kb.Bearing{Pending: true}}
	for _, tc := range []struct {
		name    string
		leaves  [][]string
		wires   values
		demoted []kb.DemotedEdge
		culled  []string
	}{
		{"culled node alone on its leaf", [][]string{{"a", "b", "c", "d"}, {"x"}}, elisionWires(), nil, []string{"x"}},
		{"two culled nodes share a leaf", [][]string{{"a", "b", "c", "d"}, {"x", "y"}},
			values{dependsOn("a", "x"), dependsOn("x", "y"), dependsOn("y", "b"), dependsOn("c", "a"), dependsOn("b", "d")}, nil, []string{"x", "y"}},
		{"a demoted record names the culled node", [][]string{{"a", "x", "b", "c", "d"}},
			values{dependsOn("a", "x"), dependsOn("x", "b"), dependsOn("c", "a"), dependsOn("b", "d")}, []kb.DemotedEdge{xDemotedIntoC}, []string{"x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := seedElisionLeaves(t, tc.leaves, tc.wires, tc.demoted...)
			ids := make([]string, len(tc.culled))
			for i, n := range tc.culled {
				ids[i] = elisionIDs[n]
			}
			run := func(op string, values []byte) write.Result {
				return write.Run(op, write.Options{KBRoot: root, Values: values, NoRefresh: true, Provenance: write.ProvenanceMachine})
			}
			if _, err := IssueCulls(ids, run); err != nil {
				t.Errorf("IssueCulls: %v", err)
			}
			tree, err := readTree(root)
			if err != nil {
				t.Fatal(err)
			}
			g := loadRecords(t, root)
			for _, id := range ids {
				if slices.ContainsFunc(g.Claims, func(n kb.ClaimNode) bool { return n.ID == id }) {
					t.Errorf("%s's record stands after its cull", id)
				}
				for _, p := range tree.Paths {
					if _, marked := kb.MarkerLines(tree.Documents[p].Text)[id]; marked {
						t.Errorf("%s still marks %s after its cull", p, id)
					}
				}
			}
			findings := gate(root, log.Discard())
			if !slices.ContainsFunc(findings, func(f Finding) bool { return f.Check == "verify" }) {
				t.Fatalf("the gate stopped before verify: %+v", findings)
			}
			for _, f := range findings {
				for _, fl := range f.Fields {
					items, _ := fl.Value.([]result.Item)
					for _, it := range items {
						if it.Check == "unknown id" {
							t.Errorf("gate: %s", it.Detail)
						}
					}
				}
			}
		})
	}
}

// TestIssueCullsDropsAClaimNamedOnlyByAnotherCull: the one case where the
// one batch accepts what culling claim by claim refuses — y named only in x's
// rationale, y issued first — lands, both records gone.
func TestIssueCullsDropsAClaimNamedOnlyByAnotherCull(t *testing.T) {
	seed := func() (string, func(op string, values []byte) write.Result) {
		root := seedElisionLeaves(t, [][]string{{"a", "b", "c", "d"}, {"x", "y"}},
			values{dependsOn("a", "x"), dependsOn("x", "y"), dependsOn("y", "b"), dependsOn("c", "a"), dependsOn("b", "d")})
		w := writer{kbRoot: root, lg: log.Discard()}
		if _, err := w.run("seed", "set-rationale", values{{"id": elisionIDs["x"], "rationale": "Follows from " + elisionIDs["y"] + "."}}); err != nil {
			t.Fatal(err)
		}
		return root, w.raw
	}
	cull := []string{elisionIDs["y"], elisionIDs["x"]}

	_, run := seed()
	if _, err := IssueCulls(cull[:1], run); err == nil {
		t.Fatal("culling y alone landed while x's rationale names it; the case this test pins does not arise")
	}
	root, run := seed()
	if _, err := IssueCulls(cull, run); err != nil {
		t.Fatalf("IssueCulls: %v", err)
	}
	g := loadRecords(t, root)
	for _, id := range cull {
		if slices.ContainsFunc(g.Claims, func(n kb.ClaimNode) bool { return n.ID == id }) {
			t.Errorf("%s's record stands after its cull", id)
		}
	}
}

// TestIssueCullsStopsOnRefusal: the culls are one drop-claim-node batch over
// every culled id in plan order, and an outcome neither done nor unchanged is
// the issuer's error, returned with that outcome.
func TestIssueCullsStopsOnRefusal(t *testing.T) {
	var ops []string
	run := func(op string, values []byte) write.Result {
		var batch struct {
			Entry []struct{ ID string }
		}
		if err := json.Unmarshal(values, &batch); err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(batch.Entry))
		for i, e := range batch.Entry {
			ids[i] = e.ID
		}
		ops = append(ops, op+" "+strings.Join(ids, ","))
		return write.Result{Outcome: result.Refused}
	}
	res, err := IssueCulls([]string{"y", "x"}, run)
	if err == nil {
		t.Fatal("IssueCulls succeeded over a refused op")
	}
	want := []string{"drop-claim-node y,x"}
	if !slices.Equal(ops, want) {
		t.Errorf("issued %q, want %q", ops, want)
	}
	if res.Outcome != result.Refused {
		t.Errorf("returned outcome %s, want the refusal", res.Outcome)
	}
}
