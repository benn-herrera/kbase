package claimgraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
	"kbase/internal/write"
)

// recordRun is an IssueCulls run closure recording each op and its values
// entry, answering done.
func recordRun(ops *[]string, values *[]map[string]any) func(op string, values []byte) write.Result {
	return func(op string, data []byte) write.Result {
		*ops = append(*ops, op)
		var v struct {
			Entry []map[string]any `json:"entry"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			panic(err)
		}
		*values = append(*values, v.Entry[0])
		return write.Result{Outcome: result.Done}
	}
}

// TestIssueCullsRestatesSoloClaimLeaf resolves the cull's leaves through the
// real leafOf over the fixture and records the ops the issuer lands: each leaf
// holding the culled claim alone is restated as no-claim, in path order,
// before the cull's drops; a leaf sharing the claim is not.
func TestIssueCullsRestatesSoloClaimLeaf(t *testing.T) {
	drops := []string{"drop-claim-edges", "drop-inbound-edges", "drop-claim-node"}
	for _, tc := range []struct {
		name     string
		leaves   [][]string
		copyLeaf string // a further leaf, copying leaf2.md's text
		restated []string
	}{
		{"one solo leaf is restated", [][]string{{"a", "b", "c", "d"}, {"x"}}, "", []string{"leaf2.md"}},
		{"a shared leaf is not restated", [][]string{{"a", "x", "b", "c", "d"}}, "", nil},
		{"two solo leaves are both restated in path order", [][]string{{"a", "b", "c", "d"}, {"x"}}, "leaf3.md", []string{"leaf2.md", "leaf3.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := seedElisionLeaves(t, tc.leaves, elisionWires())
			if tc.copyLeaf != "" {
				text, err := kb.OnDisk(root).ReadText(filepath.Join(root, "leaf2.md"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, tc.copyLeaf), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			tree, err := readTree(root)
			if err != nil {
				t.Fatal(err)
			}
			var ops []string
			var values []map[string]any
			results, err := IssueCulls([]string{elisionIDs["x"]}, culledNoClaim, recordRun(&ops, &values), leafOf(tree, log.Discard()))
			if err != nil {
				t.Fatalf("IssueCulls: %v", err)
			}
			var want []string
			for range tc.restated {
				want = append(want, "set-frontmatter")
			}
			want = append(want, drops...)
			if !slices.Equal(ops, want) {
				t.Errorf("issued %v, want %v — the restatements before the drops", ops, want)
			}
			if len(results) != len(want) {
				t.Errorf("returned %d results, want %d", len(results), len(want))
			}
			for i, doc := range tc.restated {
				fm := values[i]
				if fm["document"] != doc || fm["kind"] != "leaf" {
					t.Errorf("restatement %d = %v, want %s restated as a leaf", i, fm, doc)
				}
				if _, ok := fm["claims"]; ok {
					t.Errorf("restatement carries claims: %v", fm)
				}
				if fm["no-claim"] != culledNoClaim {
					t.Errorf("restatement's no-claim = %v, want the reason passed", fm["no-claim"])
				}
			}
		})
	}
}
