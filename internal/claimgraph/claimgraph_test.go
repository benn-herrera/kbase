package claimgraph

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/records"
)

// seedKB writes a KB at the current format — the entry point entry, the
// format stamp, every document of docs and g's records — and returns its
// kb-root.
func seedKB(t *testing.T, entry string, docs map[string]string, g kb.Graph) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	files := map[string][]byte{kb.EntryPointFile: []byte(entry), kb.FormatFile: kb.EncodeFormatStamp(kb.FormatVersion),
		kb.GraphNodesFile: kb.EncodeNodes(g), kb.GraphEdgesFile: kb.EncodeEdges(g)}
	for rel, text := range docs {
		files[rel] = []byte(text)
	}
	for rel, data := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// claimRecords is a claim record per id of titles, titled as it gives, its
// rigor pending, in id order.
func claimRecords(titles map[string]string) []kb.ClaimNode {
	var out []kb.ClaimNode
	for _, id := range slices.Sorted(maps.Keys(titles)) {
		out = append(out, kb.ClaimNode{ID: id, Title: titles[id], Rationale: titles[id] + "."})
	}
	return out
}

// loadRecords is the records root's files hold.
func loadRecords(t *testing.T, root string) kb.Graph {
	t.Helper()
	g, err := kb.LoadGraph(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestLandLeafMarksEveryClaim: a leaf's one prose claim and its equation are
// each marked in the call that mints them — the prose claim on its locator's
// line, the equation at the end of the line inside its fence that carries its
// label, the fence still closing on its own line — and landing them again
// adopts both and writes nothing.
func TestLandLeafMarksEveryClaim(t *testing.T) {
	const leaf = "v/a.md"
	root := seedKB(t, "# Entry\n\n- [V](v/index.md)\n", map[string]string{
		"v/index.md": "[↑ Entry](../entry-point.md)\n\n# V\n\n- [A](a.md)\n",
		leaf:         "[↑ V](index.md)\n\n# A\n\nThe bound holds.\n\n``` math\nx = 1 \\label{eq:e}\n```\n",
	}, kb.Graph{})
	w := writer{kbRoot: root, lg: log.Discard()}
	claims := []newClaim{
		{title: "The bound holds", rationale: "Prose.", locator: "The bound holds."},
		{title: equationTitle("eq:e", "A"), rationale: "An equation.", locator: equationLocator("eq:e")},
	}
	ids, err := w.landLeaf(leaf, claims)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("ids = %v, want two claims minted", ids)
	}
	text, err := kb.OnDisk(root).ReadText(filepath.Join(root, leaf))
	if err != nil {
		t.Fatal(err)
	}
	lines := kb.Lines(text)
	at := kb.MarkerLines(text)
	if got := lines[at[ids[0]]]; !strings.HasPrefix(got, "The bound holds.") {
		t.Errorf("the prose claim's marker is on %q, want its locator's line", got)
	}
	if got := lines[at[ids[1]]]; !strings.HasPrefix(got, `x = 1 \label{eq:e}`) {
		t.Errorf("the equation's marker is on %q, want the fence line carrying its label", got)
	}
	fences := kb.Fences(lines)
	if len(fences) != 1 || fences[0].End != len(lines) || at[ids[1]] <= fences[0].Start {
		t.Errorf("fences = %+v over %q, want the one fence closing on the last line with the equation's marker inside", fences, lines)
	}

	again, err := w.landLeaf(leaf, claims)
	if err != nil {
		t.Fatal(err)
	}
	after, err := kb.OnDisk(root).ReadText(filepath.Join(root, leaf))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(again, ids) || after != text {
		t.Errorf("landing again took %v and rewrote the leaf: want %v adopted and the leaf as it stood", again, ids)
	}
}

// TestCleanlinessRefusesNodeRecords: the double-run guard passes a tree whose
// node records file is empty and refuses one holding a record.
func TestCleanlinessRefusesNodeRecords(t *testing.T) {
	docs := map[string]string{"v/index.md": "[↑ Entry](../entry-point.md)\n\n# V\n\n- [A](a.md)\n", "v/a.md": "[↑ V](index.md)\n\n# A\n\nThe bound holds.\n"}
	for _, tc := range []struct {
		name    string
		g       kb.Graph
		refused bool
	}{
		{"no node records", kb.Graph{}, false},
		{"a node record", kb.Graph{Claims: claimRecords(map[string]string{"clm-aaaaaa": "A"})}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := readTree(seedKB(t, "# Entry\n\n- [V](v/index.md)\n", docs, tc.g))
			if err != nil {
				t.Fatal(err)
			}
			err = checkCleanliness(tree)
			var s *stop
			if refused := errors.As(err, &s) && s.check == "point-14"; refused != tc.refused || (err != nil && s == nil) {
				t.Errorf("checkCleanliness = %v, want refused %t", err, tc.refused)
			}
		})
	}
}

func TestReadDisplayLine(t *testing.T) {
	for _, tc := range []struct {
		name, display, title, span string
		ok                         bool
	}{
		{"optional argument", "**Theorem 2** (The governance bifurcation). Every x.", "The governance bifurcation", "**Theorem 2** (The governance bifurcation).", true},
		{"nested parentheses", "**Theorem 1** (A (nested) title) holds.", "A (nested) title", "**Theorem 1** (A (nested) title)", true},
		{"untitled", "**Lemma 3**. Let x be given.", "Lemma 3", "**Lemma 3**.", true},
		{"unbalanced argument falls back to the printed name", "**Lemma 3** (open. Let x.", "Lemma 3", "**Lemma 3**", true},
		{"no printed name", "Let x be given.", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			title, span, ok := readDisplayLine(tc.display)
			if title != tc.title || span != tc.span || ok != tc.ok {
				t.Errorf("readDisplayLine = (%q, %q, %t), want (%q, %q, %t)", title, span, ok, tc.title, tc.span, tc.ok)
			}
		})
	}
}

func TestDistinct(t *testing.T) {
	a, b := "**Theorem**. Every x is y.", "**Theorem**. Every z is w."
	title, span, ok := distinct("Theorem", "**Theorem**.", a, []string{b})
	if !ok || span != "**Theorem**. Every x" || title != "Theorem. Every x" {
		t.Errorf("distinct = (%q, %q, %t), want the span grown to the first word that differs", title, span, ok)
	}
	if _, _, ok := distinct("Theorem", "**Theorem**.", a, []string{a}); ok {
		t.Error("a display line running inside another to its last word must yield no span")
	}
}

func TestNamesNoPremise(t *testing.T) {
	for word, want := range map[string]bool{
		"Section": true, "Sections": true, "Fig.": true, "§": true, "Remarks": true, "Definition": true,
		"Proof": false, "Theorem": false, "by": false, "": false,
	} {
		if got := namesNoPremise(word); got != want {
			t.Errorf("namesNoPremise(%q) = %t, want %t", word, got, want)
		}
	}
}

func TestMentions(t *testing.T) {
	v := newVocabulary([]string{"Lemma", "Theorem", "Corollary", "Main Theorem"})
	for _, tc := range []struct {
		text string
		want []mention
	}{
		{"follows from Lemma 4.6 and Theorems 2 and 3.", []mention{{13, "lemma", []string{"4.6"}}, {27, "theorem", []string{"2", "3"}}}},
		{"by Lemmas 2, 3, and 5", []mention{{3, "lemma", []string{"2", "3", "5"}}}},
		{"Corollaries 1–2 hold", []mention{{0, "corollary", []string{"1", "2"}}}},
		{"the Main Theorem B", []mention{{4, "main theorem", []string{"B"}}}},
		{"Lemma 4.6a holds", []mention{{0, "lemma", []string{"4.6"}}}},
		{"Lemma 4.6a1 holds", []mention{{0, "lemma", []string{"4"}}}},
		{"subLemma 3", nil},
		{"Lemma x", nil},
		{`Theorem 3 of <span class="citation" data-cites="k">K</span>`, nil},
		{`Lemma <a href="a.md#l" data-reference-type="ref" data-reference="l">3</a>`, nil},
	} {
		got := v.mentions(tc.text)
		if len(got) != len(tc.want) {
			t.Errorf("mentions(%q) = %+v, want %+v", tc.text, got, tc.want)
			continue
		}
		for i := range got {
			if got[i].offset != tc.want[i].offset || got[i].name != tc.want[i].name || !slices.Equal(got[i].numbers, tc.want[i].numbers) {
				t.Errorf("mentions(%q)[%d] = %+v, want %+v", tc.text, i, got[i], tc.want[i])
			}
		}
	}
}

func TestLabel(t *testing.T) {
	v := newVocabulary([]string{"Lemma", "Theorem"})
	for _, tc := range []struct {
		text, name, number string
		ok                 bool
	}{
		{"Theorem 2", "theorem", "2", true},
		{" Lemma 4.6a. ", "lemma", "4.6", true},
		{"Theorem 1 (Main)", "theorem", "1", true},
		{"theorem A.1", "theorem", "A.1", true},
		{"Theorem 1 states", "", "", false},
		{"Proposition 1", "", "", false},
	} {
		name, number, ok := v.label(tc.text)
		if name != tc.name || number != tc.number || ok != tc.ok {
			t.Errorf("label(%q) = (%q, %q, %t), want (%q, %q, %t)", tc.text, name, number, ok, tc.name, tc.number, tc.ok)
		}
	}
}

func TestPrecedingWord(t *testing.T) {
	for text, want := range map[string]string{"follows by Lemma ": "Lemma", "see (Theorem ": "Theorem", "": "", "  ": ""} {
		if got := precedingWord(text, len(text)); got != want {
			t.Errorf("precedingWord(%q) = %q, want %q", text, got, want)
		}
	}
	for gap, want := range map[string]bool{" and ": true, ", ": true, ", and ": true, "–": true, " or ": true, " then ": false, "": false} {
		if got := listGapRE.MatchString(gap); got != want {
			t.Errorf("list gap %q = %t, want %t", gap, got, want)
		}
	}
}

func TestRecordInventory(t *testing.T) {
	recs := []records.Record{
		{Kind: records.KindReference, Order: 7, Document: "v/b.md", Type: "ref", Labels: "thm:a", Href: "a.md#thm:a", Target: "v/a.md"},
		{Kind: records.KindBlock, Order: 1, Document: "v/a.md", Name: "Theorem", Identifier: "thm:a"},
		{Kind: records.KindCitation, Order: 2, Document: "v/a.md", Within: 1, Keys: []string{"k1", "k2"}, States: []string{records.StateResolved, records.StateUnanswered}},
		{Kind: records.KindBlock, Order: 3, Document: "v/a.md", Name: "Proof"},
		{Kind: records.KindReference, Order: 4, Document: "v/a.md", Within: 3, Opening: true, Type: "ref", Labels: "thm:a", Href: "#thm:a"},
		{Kind: records.KindReference, Order: 5, Document: "v/a.md", Within: 3, Type: "ref+label", Labels: "x, y", Href: "#x,y"},
		{Kind: records.KindFence, Order: 6, Document: "v/a.md"},
	}
	inv := recordInventory(recs)
	if len(inv.Blocks) != 2 || len(inv.Fences) != 1 || len(inv.Citations) != 2 || len(inv.Proofs) != 1 {
		t.Fatalf("inventory = %+v", inv)
	}
	var labels, hosts []string
	for _, a := range inv.Anchors {
		labels = append(labels, a.Document+" "+a.Label)
		hosts = append(hosts, a.HostingEnvironment)
	}
	if want := []string{"v/a.md thm:a", "v/a.md x", "v/a.md y", "v/b.md thm:a"}; !slices.Equal(labels, want) {
		t.Errorf("anchors = %q, want one per label in scan order: %q", labels, want)
	}
	if want := []string{"Proof", "Proof", "Proof", ""}; !slices.Equal(hosts, want) {
		t.Errorf("hosting environments = %q, want %q", hosts, want)
	}
	if want := [][2]string{{"#thm:a", "thm:a"}}; !slices.Equal(inv.Proofs[0].Head, want) {
		t.Errorf("proof head = %q, want the opening run's reference alone: %q", inv.Proofs[0].Head, want)
	}
	if inv.Citations[1].Key != "k2" || inv.Citations[1].State != records.StateUnanswered {
		t.Errorf("citations = %+v, want one per key carrying its own state", inv.Citations)
	}
	if inv.Fences[0].Labels == nil {
		t.Error("a fence with no label carries an empty list, not none")
	}
}
