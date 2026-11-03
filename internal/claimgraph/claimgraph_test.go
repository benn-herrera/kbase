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

// TestTheBuildsWritesKeepTitlesUniqueWithinAKind: a claim read twice in one
// leaf is one claim, the second entry taking the first's id; the same title
// read in another leaf is a claim of its own, its title made unique with a
// counter; and two works of one title are two works under two titles.
func TestTheBuildsWritesKeepTitlesUniqueWithinAKind(t *testing.T) {
	root := seedKB(t, "# Entry\n\n- [V](v/index.md)\n", map[string]string{
		"v/index.md": "[↑ Entry](../entry-point.md)\n\n# V\n\n- [A](a.md)\n- [B](b.md)\n",
		"v/a.md":     "[↑ V](index.md)\n\n# A\n\nThe bound holds.\n\nThe bound holds, again.\n",
		"v/b.md":     "[↑ V](index.md)\n\n# B\n\nThe bound holds here too.\n",
	}, kb.Graph{})
	w := writer{kbRoot: root, lg: log.Discard()}
	ids, err := w.landLeaves([]entry{
		{title: "The bound", rationale: "Prose.", document: "v/a.md", locator: "The bound holds."},
		{title: "The bound", rationale: "Prose.", document: "v/a.md", locator: "The bound holds, again."},
		{title: "The bound", rationale: "Prose.", document: "v/b.md", locator: "The bound holds here too."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != ids[1] || ids[2] == ids[0] {
		t.Fatalf("ids = %v, want the first leaf's two entries one claim and the second leaf's another", ids)
	}
	minted, _, err := w.writeWorks([]CitedWork{{Key: "k1", Title: "On bounds", Named: true}, {Key: "k2", Title: "On bounds", Named: true}})
	if err != nil {
		t.Fatal(err)
	}
	g := loadRecords(t, root)
	var claims, works []string
	for _, n := range g.Claims {
		claims = append(claims, n.Title)
	}
	for _, n := range g.Works {
		works = append(works, n.Title)
		if minted[n.Label] != n.ID {
			t.Errorf("work %s, label %s: writeWorks gave %q for its key", n.ID, n.Label, minted[n.Label])
		}
	}
	if len(minted) != 2 {
		t.Errorf("writeWorks gave ids %v, want one per key", minted)
	}
	slices.Sort(claims)
	slices.Sort(works)
	if want := []string{"The bound", "The bound (2)"}; !slices.Equal(claims, want) {
		t.Errorf("claim titles %q, want %q", claims, want)
	}
	if want := []string{"On bounds", "On bounds (2)"}; !slices.Equal(works, want) {
		t.Errorf("work titles %q, want %q", works, want)
	}
}

// TestACitedWorksEdgeTakesItsMintedID: the plan's works land through add-work,
// each citation key its label, and the edge pass names each work by the id
// add-work minted it: the claim resting on the cited work is founded by that
// work's record.
func TestACitedWorksEdgeTakesItsMintedID(t *testing.T) {
	root := seedKB(t, "# Entry\n\n- [V](v/index.md)\n", map[string]string{
		"v/index.md": "[↑ Entry](../entry-point.md)\n\n# V\n\n- [A](a.md)\n",
		"v/a.md":     "[↑ V](index.md)\n\n# A\n\nThe bound holds.\n",
	}, kb.Graph{})
	w := writer{kbRoot: root, lg: log.Discard()}
	if _, err := w.writePlan(plan{
		entries: []entry{{title: "The bound", rationale: "Prose.", document: "v/a.md", locator: "The bound holds."}},
		works:   []CitedWork{{Key: "Smith2020", Title: "On bounds", Named: true}},
		restsOn: []restsOn{{entry: 0, workKey: "Smith2020"}},
	}); err != nil {
		t.Fatal(err)
	}
	g := loadRecords(t, root)
	if len(g.Works) != 1 || len(g.Claims) != 1 || g.Works[0].Label != "Smith2020" {
		t.Fatalf("works %+v, claims %+v, want one work labelled Smith2020 and one claim", g.Works, g.Claims)
	}
	if want := (kb.BearingEdge{Source: g.Works[0].ID, Target: g.Claims[0].ID, Bearing: kb.Bearing{Pending: true}}); !slices.Contains(g.Founds, want) {
		t.Errorf("founds %+v, want %+v", g.Founds, want)
	}
}

// TestACounteredBlockTitleKeepsItsBlockJoin: "Theorem 1" in two papers lands
// as two claims, the second titled with a counter, and each still joins its
// own block — its display as its locator and its block's identifier — the
// countered one by the display line its marker stands on, never by its title.
func TestACounteredBlockTitleKeepsItsBlockJoin(t *testing.T) {
	theorem := func(holds string) string {
		return "[↑ Entry](../entry-point.md)\n\n# Results\n\n> **Theorem**\n>\n> **Theorem 1.** " + holds + "\n"
	}
	root := seedKB(t, "# Entry\n\n- [One](one/index.md)\n- [Two](two/index.md)\n", map[string]string{
		"one/index.md": theorem("Alpha holds."),
		"two/index.md": theorem("Beta holds."),
	}, kb.Graph{})
	recs := []records.Record{
		{Kind: records.KindBlock, Order: 1, Document: "one/index.md", Name: "Theorem", Identifier: "thm:one"},
		{Kind: records.KindBlock, Order: 2, Document: "two/index.md", Name: "Theorem", Identifier: "thm:two"},
	}
	tree, err := readTree(root)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := scanInventory(tree, recs)
	if err != nil {
		t.Fatal(err)
	}
	p := assemblePlan(tree, &inv, blockClaims(&inv))
	if len(p.entries) != 2 {
		t.Fatalf("plan entries %+v, want the two theorems", p.entries)
	}
	w := writer{kbRoot: root, lg: log.Discard()}
	ids, err := w.addClaims("pass-1", p.entries)
	if err != nil {
		t.Fatal(err)
	}
	after, err := readTree(root)
	if err != nil {
		t.Fatal(err)
	}
	inv2, err := scanInventory(after, recs)
	if err != nil {
		t.Fatal(err)
	}
	g, err := readGraph(after, &inv2)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for i, b := range inv2.claimBlocks() {
		n, ok := g.node(ids[i])
		titles = append(titles, n.Title)
		if !ok || n.Locator != b.Display || n.Identifier != b.Identifier {
			t.Errorf("%s (%q) joins locator %q, identifier %q; want its block's %q, %q", ids[i], n.Title, n.Locator, n.Identifier, b.Display, b.Identifier)
		}
		if c, ok := claimOf(&b, g.hostedBy(b.Document)); !ok || c.ID != ids[i] {
			t.Errorf("block %s joins %+v, want %s", b.Identifier, c, ids[i])
		}
	}
	if !slices.Equal(titles, []string{"Theorem 1", "Theorem 1 (2)"}) {
		t.Errorf("titles %q, want the second countered", titles)
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
