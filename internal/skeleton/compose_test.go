package skeleton

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"kbase/internal/dissect"
)

// smallDoc is three cheap sections in one file.
func smallDoc(path, title string) docSpec {
	return docSpec{path: path, title: title, secs: []secSpec{
		{title: "One", paras: 2, words: 20},
		{title: "Two", paras: 2, words: 20},
		{title: "Three", paras: 2, words: 20},
	}}
}

// oneSectionDoc is a whole file that is one small section, for the tests that
// are about tree shape and not about spans.
func oneSectionDoc(path, title string) docSpec {
	return docSpec{path: path, title: title, secs: []secSpec{{title: "Body", paras: 1, words: 20}}}
}

func TestComposeMechanicalTree(t *testing.T) {
	v, art := verifierFor(t, testParams(), smallDoc("guide.md", "Guide"))
	plan := Plan{Title: "The Corpus", Scope: "everything", Children: []PlanNode{
		indexNode("Getting Started",
			leafFor(art, "guide.md", 0, "First Steps"),
			leafFor(art, "guide.md", 1, "Second Steps")),
		leafFor(art, "guide.md", 2, "Reference"),
	}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	want := []string{
		"entry-point.md",
		"getting-started/index.md",
		"getting-started/first-steps.md",
		"getting-started/second-steps.md",
		"reference.md",
	}
	if got := paths(s); !slices.Equal(got, want) {
		t.Fatalf("paths = %q, want %q", got, want)
	}
	if s.Nodes[0].Kind != KindEntryPoint || s.Nodes[0].Title != "The Corpus" {
		t.Fatalf("entry-point = %+v", s.Nodes[0])
	}
	if got, want := s.Nodes[2].Parent, "getting-started/index.md"; got != want {
		t.Fatalf("leaf parent = %q, want %q", got, want)
	}
	if len(s.Groups) != 3 {
		t.Fatalf("groups = %d, want one per leaf span", len(s.Groups))
	}
	for _, g := range s.Groups {
		if g.Parts != 1 {
			t.Errorf("group %s has %d parts; these spans are all under budget", g.ID, g.Parts)
		}
		if g.Budget != testBudgets().LeafTokens {
			t.Errorf("group %s was cut against %d, the artifact declares %d",
				g.ID, g.Budget, testBudgets().LeafTokens)
		}
	}
	// §3.5: for every one-part group, Split over its span returns one section.
	assertGroupsMatchTheSplitter(t, v, s)
}

// §2.4: an over-budget span is split at design time, becomes n sibling leaves
// sharing one group, and the skeleton records the count and no boundary.
func TestComposeSplitsAnOversizedSpan(t *testing.T) {
	big := docSpec{path: "big.md", title: "Big", secs: []secSpec{{title: "Long", paras: 8, words: 60}}}
	v, art := verifierFor(t, testParams(), big)
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{
		leafFor(art, "big.md", 0, "Long Chapter"),
	}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if len(s.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(s.Groups))
	}
	g := s.Groups[0]
	if g.Parts < 2 {
		t.Fatalf("group parts = %d; the fixture is meant to be over budget", g.Parts)
	}
	if n := leafCount(s); n != g.Parts {
		t.Fatalf("%d leaves for a %d-part group", n, g.Parts)
	}
	for k := 1; k <= g.Parts; k++ {
		wantPath := fmt.Sprintf("long-chapter-%d.md", k)
		wantTitle := fmt.Sprintf("Long Chapter (%d/%d)", k, g.Parts)
		n, ok := s.Node(wantPath)
		if !ok {
			t.Fatalf("no node at %s; paths are %q", wantPath, paths(s))
		}
		if n.Title != wantTitle {
			t.Errorf("%s title = %q, want %q", wantPath, n.Title, wantTitle)
		}
		if n.Group != g.ID || n.Part != k {
			t.Errorf("%s names group %q part %d, want %q part %d", wantPath, n.Group, n.Part, g.ID, k)
		}
	}
	// F-2: the span the skeleton states is the WHOLE group's, and no interior
	// boundary appears anywhere in the artifact.
	f := fileOf(art, "big.md")
	if g.Source.Start != 0 || g.Source.End != f.Bytes {
		t.Errorf("group span = %+v, want the whole section [0,%d)", g.Source, f.Bytes)
	}
	assertGroupsMatchTheSplitter(t, v, s)
}

// §2.4 step 4: a span the splitter cannot cut is a rejection carrying the
// corrective note, not a truncation and not a defect.
func TestComposeRefusesAnUncuttableSpan(t *testing.T) {
	wall := docSpec{path: "wall.md", title: "Wall", secs: []secSpec{{title: "Solid", paras: 1, words: 400}}}
	v, art := verifierFor(t, testParams(), wall)
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{
		leafFor(art, "wall.md", 0, "Solid Wall"),
	}}

	_, err := v.Compose(plan, nil)
	if err == nil {
		t.Fatal("an uncuttable span composed cleanly")
	}
	var starvedRej StarvedRejection
	if !errors.As(err, &starvedRej) {
		t.Fatalf("want a StarvedRejection, got %T: %v", err, err)
	}
	// It wraps the dissector's own refusal, so a caller can reach the span.
	var se dissect.StarvedError
	if !errors.As(err, &se) {
		t.Fatalf("the rejection does not carry dissect.StarvedError: %v", err)
	}
	r, ok := AsRejection(err)
	if !ok {
		t.Fatal("a starved span is the model's to fix, so it is a Rejection")
	}
	assertNoteIsPromptable(t, r)
	if !strings.Contains(r.Note(), fmt.Sprint(se.Tokens)) {
		t.Errorf("note %q does not carry the token count §2.4 specifies", r.Note())
	}
}

// §2.7 operator 1: a chain of single-child index levels compresses until the
// tree is inside the depth cap, and every leaf survives it.
func TestComposeCollapsesASingleChildChain(t *testing.T) {
	v, art := verifierFor(t, testParams(), oneSectionDoc("d.md", "D"))
	leaf := wholeFileLeaf(art, "d.md", "The Page")
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{
		indexNode("L2", indexNode("L3", indexNode("L4", indexNode("L5", leaf)))),
	}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	assertWithinCaps(t, s, testBudgets())
	if n := leafCount(s); n != 1 {
		t.Fatalf("leaves = %d, want the one the plan had", n)
	}
	// The outermost container's title survives; the innermost's children
	// re-parent to it.
	if _, ok := s.Node("l2/index.md"); !ok {
		t.Fatalf("the outermost container did not survive: %q", paths(s))
	}
}

// §2.7 operator 1, second clause: where no single-child chain is left,
// contiguous ancestors merge deepest-boundary-first until the cap is met.
func TestComposeMergesAncestorsWhenChainsAreNotEnough(t *testing.T) {
	docs := []docSpec{}
	for i := 1; i <= 5; i++ {
		docs = append(docs, oneSectionDoc(fmt.Sprintf("d%d.md", i), fmt.Sprintf("D%d", i)))
	}
	v, art := verifierFor(t, testParams(), docs...)

	// Every index holds one index and one leaf, so nothing is a single-child
	// chain and the merge path is the only way down to the cap.
	node := indexNode("L6", wholeFileLeaf(art, "d5.md", "Page Five"))
	for i, title := range []string{"L5", "L4", "L3", "L2"} {
		node = indexNode(title, node, wholeFileLeaf(art, fmt.Sprintf("d%d.md", 4-i), fmt.Sprintf("Page %d", 4-i)))
	}
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{node}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	assertWithinCaps(t, s, testBudgets())
	if n := leafCount(s); n != 5 {
		t.Fatalf("leaves = %d, want 5; collapse must never lose a page", n)
	}
}

// §2.7 operator 3: a group over more than one source file can be neither a
// leaf nor (at the cap) an index, so its files become sibling leaves.
func TestComposeDissolvesAMultiFileGroup(t *testing.T) {
	// One document names itself and one does not, so both halves of the
	// naming fallback are exercised: the survey's title, then the base name.
	v, art := verifierFor(t, testParams(),
		oneSectionDoc("a.md", "Alpha"), oneSectionDoc("notes/beta-doc.md", ""))
	multi := PlanNode{
		Title: "Both", Scope: "two files at once", Kind: KindLeaf,
		Sources: []Span{
			{File: "a.md", Start: 0, End: fileOf(art, "a.md").Bytes},
			{File: "notes/beta-doc.md", Start: 0, End: fileOf(art, "notes/beta-doc.md").Bytes},
		},
	}
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{multi}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if n := leafCount(s); n != 2 {
		t.Fatalf("leaves = %d, want one per source file", n)
	}
	for _, want := range []string{"alpha.md", "beta-doc.md"} {
		if _, ok := s.Node(want); !ok {
			t.Errorf("no node at %s; paths are %q", want, paths(s))
		}
	}
	for _, g := range s.Groups {
		if g.Source.File == "" {
			t.Errorf("group %s has no file; a group span is single-file", g.ID)
		}
	}
}

// §2.7 operator 2: an index over the fan-out cap has its children partitioned
// into ordered batches under synthetic indexes, mechanically titled.
func TestComposeInterposesOverTheFanOutCap(t *testing.T) {
	docs := []docSpec{}
	var children []PlanNode
	for i := 1; i <= 6; i++ {
		docs = append(docs, oneSectionDoc(fmt.Sprintf("d%d.md", i), fmt.Sprintf("D%d", i)))
	}
	v, art := verifierFor(t, testParams(), docs...)
	for i := 1; i <= 6; i++ {
		children = append(children, wholeFileLeaf(art, fmt.Sprintf("d%d.md", i), fmt.Sprintf("Page %d", i)))
	}
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{indexNode("Domain", children...)}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	assertWithinCaps(t, s, testBudgets())
	if n := leafCount(s); n != 6 {
		t.Fatalf("leaves = %d, want 6; interposition moves pages, never drops them", n)
	}
	for k, want := range []string{"Domain (1/2)", "Domain (2/2)"} {
		path := fmt.Sprintf("domain/domain-%d-2/index.md", k+1)
		n, ok := s.Node(path)
		if !ok {
			t.Fatalf("no synthetic index at %s; paths are %q", path, paths(s))
		}
		if n.Title != want {
			t.Errorf("%s title = %q, want %q", path, n.Title, want)
		}
	}
}

// §2.7's stated refusal: an interposition that re-breaches the depth cap is
// not re-collapsed. It says so, once, with a corrective note.
func TestComposeRefusesAnInterpositionThatNeedsAFifthLevel(t *testing.T) {
	docs := []docSpec{}
	for i := 1; i <= 6; i++ {
		docs = append(docs, oneSectionDoc(fmt.Sprintf("d%d.md", i), fmt.Sprintf("D%d", i)))
	}
	v, art := verifierFor(t, testParams(), docs...)
	var leaves []PlanNode
	for i := 1; i <= 6; i++ {
		leaves = append(leaves, wholeFileLeaf(art, fmt.Sprintf("d%d.md", i), fmt.Sprintf("Page %d", i)))
	}
	// The breaching index already sits at the deepest permitted level.
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{
		indexNode("L2", indexNode("L3", indexNode("L4", leaves...))),
	}}

	_, err := v.Compose(plan, nil)
	if err == nil {
		t.Fatal("a tree needing a fifth index level composed cleanly")
	}
	r, ok := AsRejection(err)
	if !ok {
		t.Fatalf("want a rejection the model can answer, got %T: %v", err, err)
	}
	assertNoteIsPromptable(t, r)
}

// G-2: an index whose children cost more than one summary call can read is
// repaired by interposition, and the guarantee is provable from the artifact.
func TestComposeInterposesOverTheSummaryInputBudget(t *testing.T) {
	// Four leaves, each near the leaf budget: within the fan-out cap, past
	// the summary input budget.
	p := testParams()
	p.Budgets.SummaryInputTokens = 300
	docs := []docSpec{}
	for i := 1; i <= 4; i++ {
		docs = append(docs, docSpec{path: fmt.Sprintf("d%d.md", i), title: fmt.Sprintf("D%d", i),
			secs: []secSpec{{title: "Body", paras: 2, words: 60}}})
	}
	v, art := verifierFor(t, p, docs...)
	var leaves []PlanNode
	for i := 1; i <= 4; i++ {
		leaves = append(leaves, wholeFileLeaf(art, fmt.Sprintf("d%d.md", i), fmt.Sprintf("Page %d", i)))
	}
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{indexNode("Domain", leaves...)}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if got := indexCount(s); got < 3 {
		t.Fatalf("%d index nodes; the budget breach should have interposed at least one", got)
	}
	assertWithinCaps(t, s, p.Budgets)
}

// §3.3 check 6: a chapter left out of the tree is caught even though every
// path, parent and cap is fine.
func TestComposeRefusesUncoveredMaterial(t *testing.T) {
	v, art := verifierFor(t, testParams(), smallDoc("guide.md", "Guide"))
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{
		leafFor(art, "guide.md", 0, "Only The First"),
	}}

	_, err := v.Compose(plan, nil)
	if err == nil {
		t.Fatal("two thirds of the corpus went missing and the skeleton verified")
	}
	r, ok := AsRejection(err)
	if !ok {
		t.Fatalf("want a rejection, got %T: %v", err, err)
	}
	if !strings.Contains(r.Note(), "no page") {
		t.Fatalf("note %q does not say material was left out", r.Note())
	}
}

// §2.8: a declared annex exempts its prefix from coverage, and a prefix the
// survey never saw is a loud configuration refusal rather than an exclusion of
// nothing.
func TestComposeAnnexes(t *testing.T) {
	v, art := verifierFor(t, testParams(),
		oneSectionDoc("guide.md", "Guide"), oneSectionDoc("ref/api.md", "API"))
	plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{
		wholeFileLeaf(art, "guide.md", "The Guide"),
	}}

	if _, err := v.Compose(plan, nil); err == nil {
		t.Fatal("the reference material is in no tree and no annex; that must refuse")
	}

	s, err := v.Compose(plan, []Annex{{Prefix: "ref", Convention: "ref/<name>.md"}})
	if err != nil {
		t.Fatalf("Compose with the annex declared: %v", err)
	}
	if len(s.Annexes) != 1 || s.Annexes[0].Prefix != "ref" {
		t.Fatalf("annexes = %+v", s.Annexes)
	}

	if _, err := v.Compose(plan, []Annex{{Prefix: "nowhere"}}); err == nil {
		t.Fatal("an annex naming no document must refuse at setup")
	}
	if _, err := v.Compose(plan, []Annex{{Prefix: "ref"}, {Prefix: "ref/api.md"}}); err == nil {
		t.Fatal("nested annexes must refuse")
	}
}

// The plan's own shape is checked before anything reads the corpus, and the
// failures are the model's.
func TestComposeRefusesMalformedPlans(t *testing.T) {
	v, art := verifierFor(t, testParams(), smallDoc("guide.md", "Guide"))
	leaf := leafFor(art, "guide.md", 0, "Page")

	for _, tc := range []struct {
		name string
		plan Plan
	}{
		{"no corpus title", Plan{Scope: "all", Children: []PlanNode{leaf}}},
		{"no domains", Plan{Title: "Corpus", Scope: "all"}},
		{"an untitled group", Plan{Title: "Corpus", Scope: "all",
			Children: []PlanNode{{Scope: "s", Kind: KindLeaf, Sources: leaf.Sources}}}},
		{"a group with no scope", Plan{Title: "Corpus", Scope: "all",
			Children: []PlanNode{{Title: "T", Kind: KindLeaf, Sources: leaf.Sources}}}},
		{"a page with children", Plan{Title: "Corpus", Scope: "all",
			Children: []PlanNode{{Title: "T", Scope: "s", Kind: KindLeaf, Sources: leaf.Sources,
				Children: []PlanNode{leaf}}}}},
		{"a page over nothing", Plan{Title: "Corpus", Scope: "all",
			Children: []PlanNode{{Title: "T", Scope: "s", Kind: KindLeaf}}}},
		{"an empty section", Plan{Title: "Corpus", Scope: "all",
			Children: []PlanNode{{Title: "T", Scope: "s", Kind: KindIndex}}}},
		{"a section over source material", Plan{Title: "Corpus", Scope: "all",
			Children: []PlanNode{{Title: "T", Scope: "s", Kind: KindIndex, Sources: leaf.Sources,
				Children: []PlanNode{leaf}}}}},
		{"an unknown kind", Plan{Title: "Corpus", Scope: "all",
			Children: []PlanNode{{Title: "T", Scope: "s", Kind: "annex", Sources: leaf.Sources}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := v.Compose(tc.plan, nil)
			if err == nil {
				t.Fatal("composed cleanly")
			}
			if _, ok := AsRejection(err); !ok {
				t.Fatalf("want a rejection, got %T: %v", err, err)
			}
		})
	}
}

// A span the corpus does not have is OUR defect: no model emitted an offset.
func TestComposeTreatsBadSpansAsDefects(t *testing.T) {
	v, _ := verifierFor(t, testParams(), smallDoc("guide.md", "Guide"))
	for _, tc := range []struct {
		name string
		span Span
	}{
		{"unknown file", Span{File: "nope.md", Start: 0, End: 4}},
		{"past the end", Span{File: "guide.md", Start: 0, End: 1 << 20}},
		{"empty", Span{File: "guide.md", Start: 4, End: 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := Plan{Title: "Corpus", Scope: "all", Children: []PlanNode{
				{Title: "T", Scope: "s", Kind: KindLeaf, Sources: []Span{tc.span}}}}
			_, err := v.Compose(plan, nil)
			var defect DefectError
			if !errors.As(err, &defect) {
				t.Fatalf("want a DefectError, got %T: %v", err, err)
			}
		})
	}
}

// A verifier is refused rather than built when its inputs disagree about which
// bytes are being planned.
func TestNewVerifierRefusesMismatchedInputs(t *testing.T) {
	art, corpus := buildCorpus(t, smallDoc("guide.md", "Guide"))
	other, _ := buildCorpus(t, smallDoc("elsewhere.md", "Elsewhere"))

	if _, err := NewVerifier(other, corpus, testParams()); err == nil {
		t.Error("a survey of a different corpus was accepted")
	}
	if _, err := NewVerifier(art, corpus, Params{}); err == nil {
		t.Error("a zero budget set was accepted; every cap would then pass")
	}
	bad := testParams()
	bad.Budgets.LeafTokens = bad.Budgets.SummaryInputTokens + 1
	if _, err := NewVerifier(art, corpus, bad); err == nil {
		t.Error("a leaf budget no summary call could read was accepted")
	}
}

// assertGroupsMatchTheSplitter is §3.5's acceptance criterion read directly:
// re-running the mechanical splitter over a group's span at its budget yields
// exactly the part count the skeleton claims.
func assertGroupsMatchTheSplitter(t *testing.T, v *Verifier, s Skeleton) {
	t.Helper()
	for _, g := range s.Groups {
		cuts, err := v.split(g.Source)
		if err != nil {
			t.Fatalf("group %s: %v", g.ID, err)
		}
		if len(cuts) != g.Parts {
			t.Errorf("group %s claims %d parts, the splitter makes %d", g.ID, g.Parts, len(cuts))
		}
	}
}

// assertWithinCaps checks the structural caps over the artifact the way a
// consumer would: from the parent chain, not from anything the verifier
// remembers.
func assertWithinCaps(t *testing.T, s Skeleton, b Budgets) {
	t.Helper()
	levels := levelsOf(s)
	fanOut := map[string]int{}
	for _, n := range s.Nodes {
		if n.Kind != KindLeaf && levels[n.Path] > b.DepthCap {
			t.Errorf("%s is an index at level %d, over the %d-level cap", n.Path, levels[n.Path], b.DepthCap)
		}
		if n.Parent != "" {
			fanOut[n.Parent]++
		}
	}
	for path, n := range fanOut {
		if n > b.FanOutCap {
			t.Errorf("%s has %d children, over the %d cap", path, n, b.FanOutCap)
		}
	}
}

func indexCount(s Skeleton) int {
	n := 0
	for _, node := range s.Nodes {
		if node.Kind == KindIndex {
			n++
		}
	}
	return n
}
