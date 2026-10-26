package write

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestMarkMovesTheOneMarker: mark-claim-in-leaf places a claim's, an
// analysis's or an experiment's marker on the located line, taking it from the
// leaf that held it — a line the marker alone stood on going with it — so the
// node is hosted once, where it was placed; a re-issue is unchanged and
// byte-identical, and a move within one leaf leaves one marker there.
func TestMarkMovesTheOneMarker(t *testing.T) {
	for _, id := range []string{"clm-aaaaaa", "ana-aaaaaa", "exp-aaaaaa"} {
		t.Run(id, func(t *testing.T) {
			root := insertKB(t, insertGraph())
			marker := kb.StatementMarker(id)
			held := snapshot(t, root)[hostedLeaf]
			// A claim's marker ends a prose line; an analysis's and an
			// experiment's stands alone below its section's heading.
			want := strings.Replace(held, marker+"\n", "", 1)
			if strings.HasPrefix(id, "clm-") {
				want = strings.Replace(held, " "+marker+"\n", "\n", 1)
			}
			values := "entry:\n- document: " + looseLeaf + "\n  id: " + id + "\n  locator: " + looseLine + "\n"
			res := runRecordOp(t, root, "mark-claim-in-leaf", values, ProvenanceCaller)
			if res.Outcome != result.Done {
				t.Fatalf("mark-claim-in-leaf: %+v", res)
			}
			files := snapshot(t, root)
			if files[hostedLeaf] != want {
				t.Errorf("%s reads\n%s\nwant the marker gone and nothing else\n%s", hostedLeaf, files[hostedLeaf], want)
			}
			if !strings.Contains(files[looseLeaf], looseLine+". "+marker+"\n") {
				t.Errorf("%s does not carry the marker on the located line:\n%s", looseLeaf, files[looseLeaf])
			}
			if _, host := placedNode(t, root, id); host != looseLeaf {
				t.Errorf("%s is hosted at %q, want %q", id, host, looseLeaf)
			}
			if again := runRecordOp(t, root, "mark-claim-in-leaf", values, ProvenanceCaller); again.Outcome != result.Unchanged || !maps.Equal(files, snapshot(t, root)) {
				t.Errorf("again: %+v, want unchanged, byte-identical", again)
			}
			within := "entry:\n- document: " + looseLeaf + "\n  id: " + id + "\n  locator: The other paragraph\n"
			if res := runRecordOp(t, root, "mark-claim-in-leaf", within, ProvenanceCaller); res.Outcome != result.Done {
				t.Fatalf("a move within the leaf: %+v", res)
			}
			if leaf := snapshot(t, root)[looseLeaf]; strings.Count(leaf, marker) != 1 || !strings.Contains(leaf, "The other paragraph. "+marker+"\n") {
				t.Errorf("a move within the leaf left other than one marker, on the located line:\n%s", leaf)
			}
		})
	}
}

// TestMarkRefusesAFenceLine: a locator naming a fenced block's opening or
// closing line — a display-maths fence's or a code fence's — is refused at the
// locator, nothing written, while a prose line of the same leaf takes the
// marker.
func TestMarkRefusesAFenceLine(t *testing.T) {
	root := insertKB(t, insertGraph())
	leaf := "# B leaf\n\nThe display below.\n\n``` math\nx = y\n```\n\nAfter the display.\n\n```python\nprint(1)\n````\n"
	if err := os.WriteFile(filepath.Join(root, looseLeaf), []byte(leaf), 0o644); err != nil {
		t.Fatal(err)
	}
	mark := func(locator string) Result {
		return runRecordOp(t, root, "mark-claim-in-leaf", "entry:\n- document: "+looseLeaf+"\n  id: clm-bbbbbb\n  locator: \""+locator+"\"\n", ProvenanceCaller)
	}
	before := snapshot(t, root)
	for _, locator := range []string{"``` math", "```python", "````"} {
		t.Run(locator, func(t *testing.T) {
			if res := mark(locator); res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "locator" {
				t.Errorf("mark-claim-in-leaf: %+v, want one refusal on locator", res)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused mark-claim-in-leaf wrote the KB")
			}
		})
	}
	if res := mark("After the display"); res.Outcome != result.Done {
		t.Errorf("a prose line of the same leaf: %+v, want done", res)
	}
}

// TestMarkRefusals: a node no record carries, a document that does not exist
// and a locator the document does not hold are each refused, nothing written.
func TestMarkRefusals(t *testing.T) {
	root := insertKB(t, insertGraph())
	before := snapshot(t, root)
	for _, c := range []struct{ name, values, key string }{
		{"no record", "entry:\n- document: " + looseLeaf + "\n  id: clm-cccccc\n  locator: " + looseLine + "\n", "id=clm-cccccc"},
		{"no document", "entry:\n- document: c.md\n  id: clm-aaaaaa\n  locator: " + looseLine + "\n", "document"},
		{"no such line", "entry:\n- document: " + looseLeaf + "\n  id: clm-aaaaaa\n  locator: words the leaf never says\n", "locator"},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := runRecordOp(t, root, "mark-claim-in-leaf", c.values, ProvenanceCaller)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != c.key {
				t.Errorf("mark-claim-in-leaf: %+v, want one refusal on %s", res, c.key)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused mark-claim-in-leaf wrote the KB")
			}
		})
	}
}
