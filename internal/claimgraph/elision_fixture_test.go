package claimgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
	"kbase/internal/write"
)

// registerQuality is one claim's Quality section as the fixture's register
// carries it.
func registerQuality(id, title string) string {
	return "## " + title + "\n\n<!-- id: " + id + " -->\n\n### Quality\n\n- confidence: *pending*\n- solidity: *pending*\n- rationale: " + title + ".\n\n"
}

// The fixture's claims, keyed by the name the tests use, with the title each
// takes in the register and its leaf's prose.
var (
	elisionIDs    = map[string]string{"a": "clm-aaaaaa", "x": "clm-xaaaaa", "y": "clm-yaaaaa", "b": "clm-bbbbbb", "c": "clm-cccccc", "d": "clm-dddddd"}
	elisionTitles = map[string]string{"a": "A", "x": "X", "y": "Y", "b": "B", "c": "C", "d": "D"}
)

// seedElisionLeaves writes a minimal KB with one register holding every
// claim leaves names: leaves[0] is leaf.md, leaves[i] leaf<i+1>.md, each
// claiming its names in order with each one's marker in its prose. It then
// lands wires through AddBuildEdges, as the build's edge write does.
func seedElisionLeaves(t *testing.T, leaves [][]string, wires values) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	entry := "---\nkb-format: \"1.0.0\"\n---\n# Entry\n\n"
	register := "# Register\n\n"
	files := map[string]string{}
	for i, names := range leaves {
		rel, title := "leaf.md", "Leaf"
		if i > 0 {
			rel, title = fmt.Sprintf("leaf%d.md", i+1), fmt.Sprintf("Leaf %d", i+1)
		}
		entry += "- [" + title + "](" + rel + ")\n"
		ids := make([]string, len(names))
		prose := ""
		for j, n := range names {
			ids[j] = elisionIDs[n]
			prose += "\n" + elisionTitles[n] + " holds here. <!-- claim-quality: " + ids[j] + " -->\n"
			register += registerQuality(ids[j], elisionTitles[n])
		}
		files[rel] = "---\nkind: leaf\nclaims: [" + strings.Join(ids, ", ") + "]\n---\n[↑ Entry](entry-point.md)\n\n# " + title + "\n" + prose
	}
	files[kb.EntryPointFile] = entry
	files[kb.RegisterFile] = register
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, text := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := writer{kbRoot: root, lg: log.Discard()}
	if _, err := w.run("seed", write.AddBuildEdges, wires, false); err != nil {
		t.Fatal(err)
	}
	return root
}

// dependsOn is the fixture's build-edge entry for claim from depending on
// each claim in to.
func dependsOn(from string, to ...string) map[string]any {
	items := make([]map[string]any, len(to))
	for i, n := range to {
		items[i] = map[string]any{"id": elisionIDs[n]}
	}
	return map[string]any{"id": elisionIDs[from], "depends-on": items}
}

// elisionWires is the chain a → x → b with c → a and b → d around it.
func elisionWires() values {
	return values{dependsOn("a", "x"), dependsOn("x", "b"), dependsOn("c", "a"), dependsOn("b", "d")}
}

// TestIssueCullsLeavesNoUnknownID drives the culls through the real leaf
// resolution and write ops over the fixture's variants, then the stage gate:
// no line under kb-root names a culled node.
func TestIssueCullsLeavesNoUnknownID(t *testing.T) {
	cDemotesX := dependsOn("c", "a")
	cDemotesX[kb.RelationDemoted] = []map[string]any{{"id": elisionIDs["x"], "origin": "cited"}}
	for _, tc := range []struct {
		name   string
		leaves [][]string
		wires  values
		culled []string
	}{
		{"culled node alone on its leaf", [][]string{{"a", "b", "c", "d"}, {"x"}}, elisionWires(), []string{"x"}},
		{"two culled nodes share a leaf", [][]string{{"a", "b", "c", "d"}, {"x", "y"}},
			values{dependsOn("a", "x"), dependsOn("x", "y"), dependsOn("y", "b"), dependsOn("c", "a"), dependsOn("b", "d")}, []string{"x", "y"}},
		{"another entry demotes the culled node", [][]string{{"a", "x", "b", "c", "d"}},
			values{dependsOn("a", "x"), dependsOn("x", "b"), cDemotesX, dependsOn("b", "d")}, []string{"x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := seedElisionLeaves(t, tc.leaves, tc.wires)
			ids := make([]string, len(tc.culled))
			for i, n := range tc.culled {
				ids[i] = elisionIDs[n]
			}
			tree, err := readTree(root)
			if err != nil {
				t.Fatal(err)
			}
			run := func(op string, values []byte) write.Result {
				return write.Run(op, write.Options{KBRoot: root, Values: values, NoRefresh: true})
			}
			if _, err := IssueCulls(ids, culledNoClaim, run, leafOf(tree, log.Discard())); err != nil {
				t.Errorf("IssueCulls: %v", err)
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

// TestIssueCullsStopsOnRefusal: the issuer stops at the first outcome that is
// neither done nor unchanged, with the ops before it issued.
func TestIssueCullsStopsOnRefusal(t *testing.T) {
	var ops []string
	run := func(op string, values []byte) write.Result {
		ops = append(ops, op)
		if op == "drop-inbound-edges" {
			return write.Result{Outcome: result.Refused}
		}
		return write.Result{Outcome: result.Done}
	}
	results, err := IssueCulls([]string{"x", "y"}, culledNoClaim, run, func(string) ([]soloClaimLeaf, error) { return nil, nil })
	if err == nil {
		t.Fatal("IssueCulls succeeded over a refused op")
	}
	want := []string{"drop-claim-edges", "drop-inbound-edges"}
	if !slices.Equal(ops, want) {
		t.Errorf("issued %v, want %v (stopped at x)", ops, want)
	}
	if len(results) != 1 {
		t.Errorf("returned %d results, want the 1 before the refusal", len(results))
	}
}
