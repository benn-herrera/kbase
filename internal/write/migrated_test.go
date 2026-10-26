package write

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/result"
)

// unmigratedKB writes a KB in the 2.0.0 form: one claim at rigor 0.5, its
// listing leaf, and the 2.0.0 index's derived edge file.
func unmigratedKB(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	for rel, text := range map[string]string{
		kb.EntryPointFile: "---\nkind: entry-point\nkb-format: \"2.0.0\"\n---\n# KB\n\n- [Leaf](leaf.md)\n",
		"leaf.md":         "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n# Leaf\n\nThe result holds. <!-- claim-rigor: clm-aaaaaa -->\n",
		"claim-rigor.md": "# Claim Rigor\n\n---\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n> **Leaf references:** *(none)*\n\n" +
			"### Rigor\n- rigor: 0.5\n\n### Strength\n- strength: 0.5\n\n---\n\n",
		kb.GraphEdgesFile: "--- {}\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, p, text)
	}
	return root
}

// TestAWriteOnAnUnmigratedKBWritesTheEdgeFileFirst: an op on a KB still in
// the 2.0.0 form that moves only the node records writes both record files,
// the edge file first, and its refresh saves the KB.
func TestAWriteOnAnUnmigratedKBWritesTheEdgeFileFirst(t *testing.T) {
	root := unmigratedKB(t)
	res := Run("set-rigor", Options{KBRoot: root, Values: []byte("entry:\n- id: clm-aaaaaa\n  rigor: 0.75\n"), Logger: log.Discard()})
	if res.Outcome != result.Done || !slices.Equal(res.Written, []string{kb.GraphEdgesFile, kb.GraphNodesFile}) {
		t.Fatalf("set-rigor over 2.0.0: %+v, want done having written %s then %s", res, kb.GraphEdgesFile, kb.GraphNodesFile)
	}
	stampFile, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(kb.FormatFile)))
	if err != nil {
		t.Fatal(err)
	}
	if stamp, err := kb.DecodeFormatStamp(stampFile); err != nil || stamp != kb.FormatVersion {
		t.Errorf("the op's refresh left the stamp at %q (%v), want %s", stamp, err, kb.FormatVersion)
	}
}

// TestAWriteRefusedByItsRefreshWritesNothing: an op that ends with a refresh,
// over a KB whose CLAUDE.md is not the redirect, refuses before it writes, so
// kb-root stands byte-identical, and an unmigrated KB gains no node file.
func TestAWriteRefusedByItsRefreshWritesNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		kb   func(*testing.T) string
	}{
		{"2.0.0", unmigratedKB},
		{"3.0.0", seedKB},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.kb(t)
			writeFile(t, filepath.Join(root, kb.AgentsRedirectFile), "Instructions kept here.\n")
			before := snapshot(t, root)
			res := Run("set-rigor", Options{KBRoot: root, Values: []byte("entry:\n- id: clm-aaaaaa\n  rigor: 0.75\n"), Logger: log.Discard()})
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Check != kb.UnmigratedAgentsCheck {
				t.Fatalf("set-rigor: %+v, want refused with one %s item", res, kb.UnmigratedAgentsCheck)
			}
			if after := snapshot(t, root); !maps.Equal(after, before) {
				t.Errorf("kb-root moved under a refused write:\nbefore %v\nafter  %v", slices.Sorted(maps.Keys(before)), slices.Sorted(maps.Keys(after)))
			}
		})
	}
}

// TestAWriteStoppedBeforeTheNodeFileConvertsAfresh: a write that stopped
// after the edge file, before the node file, leaves a KB whose next load
// reads that edge file as the derived 2.0.0 one it replaced and converts the
// KB from its 2.0.0 sources: an edge only that file holds is not read.
func TestAWriteStoppedBeforeTheNodeFileConvertsAfresh(t *testing.T) {
	root := unmigratedKB(t)
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	g, err := kb.LoadAuthoredGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	g.Founds = append(g.Founds, kb.BearingEdge{Source: "wrk-smith2020", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.5}})
	writeFile(t, src.KBPath(kb.GraphEdgesFile), string(kb.EncodeEdges(g)))
	reopened, err := kbload.Open(root)
	if err != nil {
		t.Fatalf("reopening the KB the write stopped on: %v", err)
	}
	again, err := kb.LoadAuthoredGraph(reopened)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Migrated() || len(again.Founds) != 0 || len(again.Claims) != 1 || *again.Claims[0].Rigor != 0.5 {
		t.Errorf("the reopened KB: migrated %t, founds %+v, claims %+v; want it converted afresh from its 2.0.0 sources",
			reopened.Migrated(), again.Founds, again.Claims)
	}
}
