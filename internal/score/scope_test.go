package score

import (
	"os"
	"slices"
	"strings"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/kb"
	"kbase/internal/query"
	"kbase/internal/result"
)

// dirtyItems is every item of every pass the KB at root holds dirty, as
// ItemName names them, in walk order.
func dirtyItems(t *testing.T, root string) []string {
	t.Helper()
	src := kb.OnDisk(root)
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	g, err := graphOf(src, ix)
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := recordedHashes(src)
	if err != nil {
		t.Fatal(err)
	}
	items, err := scopeOf(ix, g, recorded, Invocation{Pass: query.PassAll})
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, it := range items {
		names = append(names, ItemName(it))
	}
	return names
}

// rewriteLeaf replaces old with new in the leaf rel of the KB at root, failing
// t where the leaf does not hold old.
func rewriteLeaf(t *testing.T, root, rel, old, new string) {
	t.Helper()
	path := kb.OnDisk(root).KBPath(rel)
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), old) {
		t.Fatalf("%s holds no %q", rel, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(text), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTheDirtySet: a KB whose values were never judged — every judged hash all
// zeros — is dirty whole; once a run has landed every value nothing is dirty;
// an edit elsewhere in a leaf than a statement leaves nothing dirty; an edit
// to a node's statement makes that node dirty and every edge touching it, and
// nothing else; and hashes recorded under another ask template's digest make
// every item dirty.
func TestTheDirtySet(t *testing.T) {
	root, state := invokeKB(t), t.TempDir()
	every := slices.Concat(invokeRigorItems, invokeBearingItems)
	if got := dirtyItems(t, root); !slices.Equal(got, every) {
		t.Fatalf("the never-judged KB holds %q dirty, want every item %q", got, every)
	}

	ask, _ := replying(t, slices.Repeat([]string{"none"}, len(every))...)
	if rep := invoked(t, invocation(t, root, state, query.PassAll, false, ask)); rep.Outcome != result.Done {
		t.Fatalf("the run: outcome %s", rep.Outcome)
	}
	if got := dirtyItems(t, root); len(got) != 0 {
		t.Fatalf("after the run landed every value, %q are dirty, want none", got)
	}

	rewriteLeaf(t, root, "vol/a.md", "# The First Result\n\n", "# The First Result\n\nThe section's own prose, no claim's statement.\n\n")
	if got := dirtyItems(t, root); len(got) != 0 {
		t.Errorf("an edit outside every statement made %q dirty, want none", got)
	}

	rewriteLeaf(t, root, "vol/a.md", "Every exchange conserves stock.", "Every exchange conserves its stock.")
	if got, want := dirtyItems(t, root), []string{"clm-aaaaaa", "supports clm-aaaaaa → clm-bbbbbb", "refutes ana-aaaaaa → clm-aaaaaa"}; !slices.Equal(got, want) {
		t.Errorf("an edit to clm-aaaaaa's statement made %q dirty, want the claim and the edges touching it: %q", got, want)
	}

	// Every record's hash as another template would record it over the
	// statements as they now stand.
	src := kb.OnDisk(root)
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	g, err := graphOf(src, ix)
	if err != nil {
		t.Fatal(err)
	}
	records, err := kb.LoadGraph(src)
	if err != nil {
		t.Fatal(err)
	}
	other := strings.Repeat("f", 64)
	for i, n := range records.Claims {
		records.Claims[i].JudgedHash = asks.JudgedHash(other, g.statements[n.ID])
	}
	for i, n := range records.Analyses {
		records.Analyses[i].JudgedHash = asks.JudgedHash(other, g.statements[n.ID])
	}
	for i, n := range records.Experiments {
		records.Experiments[i].JudgedHash = asks.JudgedHash(other, g.statements[n.ID])
	}
	for i, e := range records.Scorable {
		records.Scorable[i].JudgedHash = asks.JudgedHash(other, g.statements[e.Source], g.statements[e.Target])
	}
	for rel, data := range map[string][]byte{kb.GraphNodesFile: kb.EncodeNodes(records), kb.GraphEdgesFile: kb.EncodeEdges(records)} {
		if err := os.WriteFile(src.KBPath(rel), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := dirtyItems(t, root); !slices.Equal(got, every) {
		t.Errorf("hashes recorded under another template's digest leave %q dirty, want every item %q", got, every)
	}
}
