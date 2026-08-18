package treeplan

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"kbase/internal/dissect"
	"kbase/internal/log"
	"kbase/internal/log/logtest"
	"kbase/internal/survey"
)

// smallDoc is three cheap sections in one file. Cheap, but each one over the
// content floor — see fixture_test.go.
func smallDoc(path, title string) docSpec {
	return docSpec{path: path, title: title, secs: []secSpec{
		{title: "One", paras: 2, words: 35},
		{title: "Two", paras: 2, words: 35},
		{title: "Three", paras: 2, words: 35},
	}}
}

// oneSectionDoc is a whole file that is one small section, for the tests that
// are about tree shape and not about spans.
func oneSectionDoc(path, title string) docSpec {
	return docSpec{path: path, title: title, secs: []secSpec{{title: "Body", paras: 1, words: 70}}}
}

// fragmentDoc is a document whose first section is under the content floor:
// the preamble-sized opener the Rojo corpus delivers as a page holding one
// byte [MAD2: B-6].
func fragmentDoc(path, title string) docSpec {
	return docSpec{path: path, title: title, secs: []secSpec{
		{title: "Opener", paras: 1, words: 4},
		{title: "Body", paras: 2, words: 35},
		{title: "Tail", paras: 2, words: 35},
	}}
}

// §4 row 3, the content floor: a span under the minimum is merged into the
// adjacent group of the same file rather than becoming a page, and the page
// that survives is the one holding the material.
func TestComposeMergesASubFloorSpanIntoItsNeighbour(t *testing.T) {
	for _, tc := range []struct {
		name string
		// tiny is the index of the sub-floor section, and keeps is the section
		// whose page must survive holding it.
		tiny, keeps int
	}{
		{name: "a leading fragment merges forward", tiny: 0, keeps: 1},
		{name: "an interior fragment merges backward", tiny: 2, keeps: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := docSpec{path: "guide.md", title: "Guide", secs: []secSpec{
				{title: "One", paras: 2, words: 35},
				{title: "Two", paras: 2, words: 35},
				{title: "Three", paras: 2, words: 35},
			}}
			doc.secs[tc.tiny] = secSpec{title: doc.secs[tc.tiny].title, paras: 1, words: 4}
			v, art := verifierFor(t, testParams(), doc)

			var leaves []ProposalNode
			for i, sec := range doc.secs {
				leaves = append(leaves, leafFor(art, "guide.md", i, "Page "+sec.title))
			}
			plan := TreeProposal{Title: "Corpus", Scope: "all",
				Children: []ProposalNode{indexNode("Domain", leaves...)}}

			s, err := v.Compose(plan, nil)
			if err != nil {
				t.Fatalf("Compose: %v", err)
			}
			if n := leafCount(s); n != 2 {
				t.Fatalf("leaves = %d, want 2: the fragment is absorbed, not delivered", n)
			}
			// The absorbed page is gone and the neighbour's is the survivor.
			gone := "domain/page-" + strings.ToLower(doc.secs[tc.tiny].title) + ".md"
			if _, ok := s.Node(gone); ok {
				t.Errorf("%s was delivered; the sub-floor span must not name a page", gone)
			}
			host, ok := s.Node("domain/page-" + strings.ToLower(doc.secs[tc.keeps].title) + ".md")
			if !ok {
				t.Fatalf("the absorbing page is missing; paths are %q", paths(s))
			}
			g, ok := s.SplitGroup(host.SplitGroup)
			if !ok {
				t.Fatalf("%s names group %q, which the artifact does not hold", host.Path, host.SplitGroup)
			}
			// It holds both sections' bytes, contiguously.
			f := fileOf(art, "guide.md")
			lo := min(f.Sections[tc.tiny].Start, f.Sections[tc.keeps].Start)
			hi := max(f.Sections[tc.tiny].End, f.Sections[tc.keeps].End)
			if g.Source.Start != lo || g.Source.End != hi {
				t.Errorf("the surviving group spans [%d,%d), want the two sections' [%d,%d)",
					g.Source.Start, g.Source.End, lo, hi)
			}
			assertGroupsMatchTheSplitter(t, v, s)
		})
	}
}

// The floor cascades: a neighbour still under the minimum after absorbing a
// fragment is itself a fragment, and the fold continues until nothing is left
// under it.
func TestComposeMergesRepeatedlyUntilNothingIsUnderTheFloor(t *testing.T) {
	doc := docSpec{path: "guide.md", title: "Guide", secs: []secSpec{
		{title: "One", paras: 1, words: 4},
		{title: "Two", paras: 1, words: 4},
		{title: "Three", paras: 1, words: 4},
		{title: "Four", paras: 2, words: 35},
	}}
	v, art := verifierFor(t, testParams(), doc)
	var leaves []ProposalNode
	for i, sec := range doc.secs {
		leaves = append(leaves, leafFor(art, "guide.md", i, "Page "+sec.title))
	}
	plan := TreeProposal{Title: "Corpus", Scope: "all",
		Children: []ProposalNode{indexNode("Domain", leaves...)}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if n := leafCount(s); n != 1 {
		t.Fatalf("leaves = %d, want 1; three fragments and their host are one page", n)
	}
	if len(s.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(s.Groups))
	}
	if got, want := s.Groups[0].Source, (Span{File: "guide.md", Start: 0, End: fileOf(art, "guide.md").Bytes}); got != want {
		t.Errorf("the surviving group spans %+v, want the whole file %+v", got, want)
	}
	assertGroupsMatchTheSplitter(t, v, s)
}

// The floor applies to splits, not to documents (ruled 2026-08-17): a span that
// covers everything its file holds stands as its own leaf whatever its size,
// because the floor's target is a page manufactured out of part of a larger
// document and a whole tiny document behind its own title is not that.
//
// This is the case that used to refuse — a corpus of whole documents under the
// floor was unbuildable, since no document has an adjacent span of another file
// to merge into.
func TestComposeKeepsAFileWholeSoleSpanAsALeaf(t *testing.T) {
	// ~30 tokens: a third of the floor, so nothing here passes by clearing it.
	stub := func(path, title string) docSpec {
		return docSpec{path: path, title: title, secs: []secSpec{{title: "All", paras: 1, words: 22}}}
	}
	v, art := verifierFor(t, testParams(), stub("a.md", "A"), stub("b.md", "B"), smallDoc("guide.md", "Guide"))
	for _, f := range []string{"a.md", "b.md"} {
		span := Span{File: f, Start: 0, End: fileOf(art, f).Bytes}
		src, _ := v.bytes(f)
		if !(dissect.Params{Est: v.p.Est}).UnderMinimum(src,
			survey.Span{Start: span.Start, End: span.End}) {
			t.Fatalf("%s is over the %d-token floor; this fixture no longer tests the exemption",
				f, dissect.MinTokens)
		}
	}
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
		wholeFileLeaf(art, "a.md", "Page A"),
		wholeFileLeaf(art, "b.md", "Page B"),
		leafFor(art, "guide.md", 0, "One"),
		leafFor(art, "guide.md", 1, "Two"),
		leafFor(art, "guide.md", 2, "Three"),
	}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if n := leafCount(s); n != 5 {
		t.Fatalf("leaves = %d, want 5: two tiny documents and three sections of a third", n)
	}
	// Each tiny document owns a page, and that page holds the whole file.
	for _, f := range []string{"a.md", "b.md"} {
		want := Span{File: f, Start: 0, End: fileOf(art, f).Bytes}
		found := false
		for _, g := range s.Groups {
			if g.Source == want {
				found = true
			}
		}
		if !found {
			t.Errorf("no group spans the whole of %s; groups are %+v", f, s.Groups)
		}
	}
	assertGroupsMatchTheSplitter(t, v, s)
}

// splitParentDoc is one file whose three sections the tests below hand to two
// DIFFERENT parent indexes — legal and unremarkable, and the shape the floor's
// parent boundary is about [ARCH F2]. The middle section is the fragment.
func splitParentDoc() docSpec {
	return docSpec{path: "guide.md", title: "Guide", secs: []secSpec{
		{title: "One", paras: 2, words: 35},
		{title: "Two", paras: 1, words: 4},
		{title: "Three", paras: 2, words: 35},
	}}
}

// The parent boundary, preference half: a touching neighbour under the SAME
// parent wins over a touching neighbour under a different one, even where the
// cross-parent one is the backward neighbour the unscoped rule took first.
//
// Byte adjacency is equal on both sides here — the file's sections tile — so
// the only thing choosing between them is the parent, which is the whole point:
// merging backward would re-home the fragment under an index that never
// promised it.
func TestComposePrefersASameParentNeighbourOverACrossParentOne(t *testing.T) {
	doc := splitParentDoc()
	lg := &logtest.Capture{}
	art, corpus := buildCorpus(t, doc)
	v, err := NewVerifier(art, corpus, testParams(), lg)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	// One under A; the fragment and Three under B. Backward is cross-parent,
	// forward is same-parent.
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
		indexNode("Domain A", leafFor(art, "guide.md", 0, "Page One")),
		indexNode("Domain B",
			leafFor(art, "guide.md", 1, "Page Two"),
			leafFor(art, "guide.md", 2, "Page Three")),
	}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if n := leafCount(s); n != 2 {
		t.Fatalf("leaves = %d, want 2: the fragment is absorbed, not delivered; paths are %q", n, paths(s))
	}
	if _, ok := s.Node("domain-b/page-two.md"); ok {
		t.Errorf("the sub-floor span was delivered as a page of its own")
	}
	f := fileOf(art, "guide.md")
	// The fragment went FORWARD, into its own parent's page.
	host := groupOf(t, s, "domain-b/page-three.md")
	if want := (Span{File: "guide.md", Start: f.Sections[1].Start, End: f.Sections[2].End}); host.Source != want {
		t.Errorf("the same-parent host spans %+v, want the fragment's bytes and its own %+v", host.Source, want)
	}
	// And the cross-parent neighbour was left exactly as planned, which is what
	// the backward-first rule would have broken.
	across := groupOf(t, s, "domain-a/page-one.md")
	if want := (Span{File: "guide.md", Start: f.Sections[0].Start, End: f.Sections[0].End}); across.Source != want {
		t.Errorf("the page under the other parent spans %+v, want its own section %+v", across.Source, want)
	}
	if s.CrossParentMerges != 0 {
		t.Errorf("crossParentMerges = %d over a merge that stayed under one parent", s.CrossParentMerges)
	}
	if n := lg.Count("warn", "file", "guide.md"); n != 0 {
		t.Errorf("%d cross-parent warnings for a merge that crossed nothing", n)
	}
	assertGroupsMatchTheSplitter(t, v, s)
}

// The parent boundary, fallback half: where the ONLY touching neighbours sit
// under another parent the merge is taken anyway — refusing would make a
// buildable corpus unbuildable — and it is warned and counted, because no gate
// can see it afterwards. The index the fragment was the sole child of is then
// pruned, which is the other half of the same event.
func TestComposeMergesAcrossParentsAsALastResortAndCountsIt(t *testing.T) {
	doc := splitParentDoc()
	lg := &logtest.Capture{}
	art, corpus := buildCorpus(t, doc)
	v, err := NewVerifier(art, corpus, testParams(), lg)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	// The fragment is alone under B; both its touching neighbours are under A.
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
		indexNode("Domain A",
			leafFor(art, "guide.md", 0, "Page One"),
			leafFor(art, "guide.md", 2, "Page Three")),
		indexNode("Domain B", leafFor(art, "guide.md", 1, "Page Two")),
	}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if s.CrossParentMerges != 1 {
		t.Errorf("crossParentMerges = %d, want the one merge that crossed a parent", s.CrossParentMerges)
	}
	if !lg.Has(t, "warn", "file", "guide.md") {
		t.Errorf("the cross-parent merge was not warned; records are %+v", lg.Snapshot())
	}
	// The backward neighbour absorbed it, as the unscoped order always did:
	// with no same-parent candidate the direction rule decides again.
	f := fileOf(art, "guide.md")
	host := groupOf(t, s, "domain-a/page-one.md")
	if want := (Span{File: "guide.md", Start: f.Sections[0].Start, End: f.Sections[1].End}); host.Source != want {
		t.Errorf("the host spans %+v, want the fragment's bytes and its own %+v", host.Source, want)
	}
	// The prune: an index whose every child was absorbed under some other
	// parent routes nowhere and is removed, with its scope line.
	for _, p := range paths(s) {
		if strings.HasPrefix(p, "domain-b/") {
			t.Errorf("%s survived; the index the fragment was the sole child of holds nothing", p)
		}
	}
	if n := leafCount(s); n != 2 {
		t.Fatalf("leaves = %d, want 2; paths are %q", n, paths(s))
	}
	assertGroupsMatchTheSplitter(t, v, s)
}

// groupOf is the split group one delivered node draws from.
func groupOf(t *testing.T, s TreePlan, nodePath string) SplitGroup {
	t.Helper()
	n, ok := s.Node(nodePath)
	if !ok {
		t.Fatalf("%s was not delivered; paths are %q", nodePath, paths(s))
	}
	g, ok := s.SplitGroup(n.SplitGroup)
	if !ok {
		t.Fatalf("%s names group %q, which the artifact does not hold", nodePath, n.SplitGroup)
	}
	return g
}

// A span with a neighbour it does not TOUCH is not mergeable: closing the gap
// would deliver bytes no group planned, and would paper over the coverage hole
// §3.3's tiling check exists to refuse.
func TestComposeWillNotMergeAcrossAGap(t *testing.T) {
	doc := docSpec{path: "guide.md", title: "Guide", secs: []secSpec{
		{title: "One", paras: 1, words: 4},
		{title: "Two", paras: 2, words: 35},
		{title: "Three", paras: 2, words: 35},
	}}
	v, art := verifierFor(t, testParams(), doc)
	f := fileOf(art, "guide.md")
	// The fragment, and a page that starts one byte after it ends.
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
		{Title: "Fragment", Scope: "s", Kind: KindLeaf,
			Sources: []Span{{File: "guide.md", Start: 0, End: f.Sections[0].End}}},
		{Title: "Rest", Scope: "s", Kind: KindLeaf,
			Sources: []Span{{File: "guide.md", Start: f.Sections[1].Start + 1, End: f.Bytes}}},
	}}

	_, err := v.Compose(plan, nil)
	var defect DefectError
	if !errors.As(err, &defect) {
		t.Fatalf("want the floor's refusal over a gap, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "no adjacent material") {
		t.Fatalf("the refusal is not the floor's: %v", err)
	}
}

func TestComposeMechanicalTree(t *testing.T) {
	v, art := verifierFor(t, testParams(), smallDoc("guide.md", "Guide"))
	plan := TreeProposal{Title: "The Corpus", Scope: "everything", Children: []ProposalNode{
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
// sharing one group, and the tree plan records the count and no boundary.
func TestComposeSplitsAnOversizedSpan(t *testing.T) {
	big := docSpec{path: "big.md", title: "Big", secs: []secSpec{{title: "Long", paras: 8, words: 60}}}
	v, art := verifierFor(t, testParams(), big)
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
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
		if n.SplitGroup != g.ID || n.Part != k {
			t.Errorf("%s names group %q part %d, want %q part %d", wantPath, n.SplitGroup, n.Part, g.ID, k)
		}
	}
	// F-2: the span the tree plan states is the WHOLE group's, and no interior
	// boundary appears anywhere in the artifact.
	f := fileOf(art, "big.md")
	if g.Source.Start != 0 || g.Source.End != f.Bytes {
		t.Errorf("group span = %+v, want the whole section [0,%d)", g.Source, f.Bytes)
	}
	assertGroupsMatchTheSplitter(t, v, s)
}

// §2.4 step 4: a span the splitter cannot cut is a rejection carrying the
// retry note, not a truncation and not a defect.
func TestComposeRefusesAnUncuttableSpan(t *testing.T) {
	wall := docSpec{path: "wall.md", title: "Wall", secs: []secSpec{{title: "Solid", paras: 1, words: 400}}}
	v, art := verifierFor(t, testParams(), wall)
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
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
	var se dissect.StarvedRejection
	if !errors.As(err, &se) {
		t.Fatalf("the rejection does not carry dissect.StarvedRejection: %v", err)
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
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
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
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{node}}

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
	multi := ProposalNode{
		Title: "Both", Scope: "two files at once", Kind: KindLeaf,
		Sources: []Span{
			{File: "a.md", Start: 0, End: fileOf(art, "a.md").Bytes},
			{File: "notes/beta-doc.md", Start: 0, End: fileOf(art, "notes/beta-doc.md").Bytes},
		},
	}
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{multi}}

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
	var children []ProposalNode
	for i := 1; i <= 6; i++ {
		docs = append(docs, oneSectionDoc(fmt.Sprintf("d%d.md", i), fmt.Sprintf("D%d", i)))
	}
	v, art := verifierFor(t, testParams(), docs...)
	for i := 1; i <= 6; i++ {
		children = append(children, wholeFileLeaf(art, fmt.Sprintf("d%d.md", i), fmt.Sprintf("Page %d", i)))
	}
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{indexNode("Domain", children...)}}

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
// not re-collapsed. It says so, once, with a retry note.
func TestComposeRefusesAnInterpositionThatNeedsAFifthLevel(t *testing.T) {
	docs := []docSpec{}
	for i := 1; i <= 6; i++ {
		docs = append(docs, oneSectionDoc(fmt.Sprintf("d%d.md", i), fmt.Sprintf("D%d", i)))
	}
	v, art := verifierFor(t, testParams(), docs...)
	var leaves []ProposalNode
	for i := 1; i <= 6; i++ {
		leaves = append(leaves, wholeFileLeaf(art, fmt.Sprintf("d%d.md", i), fmt.Sprintf("Page %d", i)))
	}
	// The breaching index already sits at the deepest permitted level.
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
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
	var leaves []ProposalNode
	for i := 1; i <= 4; i++ {
		leaves = append(leaves, wholeFileLeaf(art, fmt.Sprintf("d%d.md", i), fmt.Sprintf("Page %d", i)))
	}
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{indexNode("Domain", leaves...)}}

	s, err := v.Compose(plan, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if got := indexCount(s); got < 3 {
		t.Fatalf("%d index nodes; the budget breach should have interposed at least one", got)
	}
	assertWithinCaps(t, s, p.Budgets)
}

// G-2's cost rule, stated over the accumulator both its sites fill: a node's
// direct leaves are one call and its own summary is another, so the node costs
// the LARGER of the two rather than the sum of everything under it.
//
// The fourth case is the one the old per-child sum got wrong in the other
// direction: where the pages are light, the shelf still costs a full summary cap
// at the parent, because what the parent reads is the card and not the bodies.
func TestSummaryCallCostIsPerShelf(t *testing.T) {
	const summaryCap = 400
	for _, tc := range []struct {
		name string
		sh   shelves
		want int
	}{
		{"no children at all", shelves{}, 0},
		{"leaves only: the group call IS the summary call", shelves{pages: 1000, leaves: true}, 1000},
		{"sections only", shelves{sections: 2 * summaryCap}, 2 * summaryCap},
		{"mixed, light pages: the shelf costs the cap, not the bodies",
			shelves{pages: 10, leaves: true, sections: 2 * summaryCap}, 3 * summaryCap},
		{"mixed, heavy pages: the group call is the larger of the two",
			shelves{pages: 5000, leaves: true, sections: 2 * summaryCap}, 5000},
		{"a zero-token page is still a shelf",
			shelves{leaves: true, sections: summaryCap}, 2 * summaryCap},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sh.calls(summaryCap); got != tc.want {
				t.Errorf("calls = %d, want %d", got, tc.want)
			}
		})
	}
}

// §3.3 check 6: a chapter left out of the tree is caught even though every
// path, parent and cap is fine.
func TestComposeRefusesUncoveredMaterial(t *testing.T) {
	v, art := verifierFor(t, testParams(), smallDoc("guide.md", "Guide"))
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
		leafFor(art, "guide.md", 0, "Only The First"),
	}}

	_, err := v.Compose(plan, nil)
	if err == nil {
		t.Fatal("two thirds of the corpus went missing and the tree plan verified")
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
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
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

// §2.8: a declared prefix carries no convention text, so the verifier writes
// one from the survey — the path grammar the prefix's own files exhibit, the
// dominant extension, and one worked example. A caller that supplied its own
// keeps it (TestComposeAnnexes above).
func TestComposeAuthorsAnnexConventions(t *testing.T) {
	v, art := verifierFor(t, testParams(),
		oneSectionDoc("guide.md", "Guide"),
		oneSectionDoc("ref/api.md", "API"),
		oneSectionDoc("ref/net/tcp.md", "TCP"))
	plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
		wholeFileLeaf(art, "guide.md", "The Guide"),
	}}

	s, err := v.Compose(plan, []Annex{{Prefix: "ref"}})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	// One `<dir>/` for the deepest file under the prefix, `.md` as the
	// dominant extension, and the lexicographically first file as the
	// example — so the line reads as a rule plus an instance of it.
	if want := "ref/<dir>/<name>.md, e.g. ref/api.md"; s.Annexes[0].Convention != want {
		t.Errorf("convention = %q, want %q", s.Annexes[0].Convention, want)
	}
}

// The plan's own shape is checked before anything reads the corpus, and the
// failures are the model's.
func TestComposeRefusesMalformedPlans(t *testing.T) {
	v, art := verifierFor(t, testParams(), smallDoc("guide.md", "Guide"))
	leaf := leafFor(art, "guide.md", 0, "Page")

	for _, tc := range []struct {
		name string
		plan TreeProposal
	}{
		{"no corpus title", TreeProposal{Scope: "all", Children: []ProposalNode{leaf}}},
		{"no domains", TreeProposal{Title: "Corpus", Scope: "all"}},
		{"an untitled group", TreeProposal{Title: "Corpus", Scope: "all",
			Children: []ProposalNode{{Scope: "s", Kind: KindLeaf, Sources: leaf.Sources}}}},
		{"a group with no scope", TreeProposal{Title: "Corpus", Scope: "all",
			Children: []ProposalNode{{Title: "T", Kind: KindLeaf, Sources: leaf.Sources}}}},
		{"a page with children", TreeProposal{Title: "Corpus", Scope: "all",
			Children: []ProposalNode{{Title: "T", Scope: "s", Kind: KindLeaf, Sources: leaf.Sources,
				Children: []ProposalNode{leaf}}}}},
		{"a page over nothing", TreeProposal{Title: "Corpus", Scope: "all",
			Children: []ProposalNode{{Title: "T", Scope: "s", Kind: KindLeaf}}}},
		{"an empty section", TreeProposal{Title: "Corpus", Scope: "all",
			Children: []ProposalNode{{Title: "T", Scope: "s", Kind: KindIndex}}}},
		{"a section over source material", TreeProposal{Title: "Corpus", Scope: "all",
			Children: []ProposalNode{{Title: "T", Scope: "s", Kind: KindIndex, Sources: leaf.Sources,
				Children: []ProposalNode{leaf}}}}},
		{"an unknown kind", TreeProposal{Title: "Corpus", Scope: "all",
			Children: []ProposalNode{{Title: "T", Scope: "s", Kind: "annex", Sources: leaf.Sources}}}},
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
			plan := TreeProposal{Title: "Corpus", Scope: "all", Children: []ProposalNode{
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

	if _, err := NewVerifier(other, corpus, testParams(), log.Discard()); err == nil {
		t.Error("a survey of a different corpus was accepted")
	}
	if _, err := NewVerifier(art, corpus, Params{}, log.Discard()); err == nil {
		t.Error("a zero budget set was accepted; every cap would then pass")
	}
	bad := testParams()
	bad.Budgets.LeafTokens = bad.Budgets.SummaryInputTokens + 1
	if _, err := NewVerifier(art, corpus, bad, log.Discard()); err == nil {
		t.Error("a leaf budget no summary call could read was accepted")
	}
}

// assertGroupsMatchTheSplitter is §3.5's acceptance criterion read directly:
// re-running the mechanical splitter over a group's span at its budget yields
// exactly the part count the tree plan claims.
//
// It also reads the content floor where it is finally DELIVERED — on the
// parts, one per page — rather than on the group span Check states it over.
// The two agree because the splitter pre-merges below-minimum sections, so a
// span over the floor cannot yield a part under it; this is that implication
// exercised over real bytes instead of argued in a comment [MAD2: B-6].
func assertGroupsMatchTheSplitter(t *testing.T, v *Verifier, s TreePlan) {
	t.Helper()
	for _, g := range s.Groups {
		cuts, err := v.split(g.Source)
		if err != nil {
			t.Fatalf("group %s: %v", g.ID, err)
		}
		if len(cuts) != g.Parts {
			t.Errorf("group %s claims %d parts, the splitter makes %d", g.ID, g.Parts, len(cuts))
		}
		for k, c := range cuts {
			if v.underFloor(Span{File: g.Source.File, Start: c.Start, End: c.End}) {
				t.Errorf("group %s part %d of %d is under the content floor; no delivered page may be",
					g.ID, k+1, len(cuts))
			}
		}
	}
}

// assertWithinCaps checks the structural caps over the artifact the way a
// consumer would: from the parent chain, not from anything the verifier
// remembers.
func assertWithinCaps(t *testing.T, s TreePlan, b Budgets) {
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

func indexCount(s TreePlan) int {
	n := 0
	for _, node := range s.Nodes {
		if node.Kind == KindIndex {
			n++
		}
	}
	return n
}
