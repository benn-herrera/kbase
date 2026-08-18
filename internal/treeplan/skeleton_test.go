package treeplan

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"kbase/internal/survey"
)

// Rule R-C, stated as a table over documents the survey could hand us: a
// section descends exactly when its subtree is over the leaf budget AND it has
// children, at every depth, with no cap on how far that goes.
//
// The budget here is testBudgets().LeafTokens (200), so "big" and "small" below
// are written either side of it.
func TestSkeletonDescentRule(t *testing.T) {
	// small is comfortably under the leaf budget and over the content floor;
	// big is over the budget on its own.
	small := func(title string, subs ...secSpec) secSpec {
		return secSpec{title: title, paras: 2, words: 35, subs: subs}
	}
	big := func(title string, subs ...secSpec) secSpec {
		return secSpec{title: title, paras: 4, words: 60, subs: subs}
	}

	for _, tc := range []struct {
		name string
		doc  docSpec
		want []string
	}{{
		name: "sections that fit are pages, exactly as before descent",
		doc:  docSpec{path: "flat.md", title: "Flat", secs: []secSpec{small("One"), small("Two")}},
		want: []string{"One", "Two"},
	}, {
		// The oMLX shape: one document, one heading level below it, and a
		// subtree far past what one page may carry.
		name: "an over-budget section with children becomes a container",
		doc: docSpec{path: "one.md", title: "One", secs: []secSpec{
			big("Cluster", small("Workers"), small("Scheduler")),
		}},
		want: []string{
			"Cluster",
			"  Introduction to Cluster",
			"  Workers",
			"  Scheduler",
		},
	}, {
		// Nothing to descend into: this is the split path's honest domain.
		name: "an over-budget section with no children stays one span",
		doc:  docSpec{path: "wall.md", title: "Wall", secs: []secSpec{big("Solid")}},
		want: []string{"Solid"},
	}, {
		// The churn guard the rule gets for free: a section that fits on one
		// page IS one page, however many headings its author typed.
		name: "a section that fits is one page however many children it has",
		doc: docSpec{path: "small.md", title: "Small", secs: []secSpec{
			{title: "Compact", paras: 1, words: 20, subs: []secSpec{
				{title: "A", paras: 1, words: 20}, {title: "B", paras: 1, words: 20}}},
		}},
		want: []string{"Compact"},
	}, {
		name: "descent recurses: a child over budget descends in turn",
		doc: docSpec{path: "deep.md", title: "Deep", secs: []secSpec{
			big("Top", big("Middle", small("Leaf A"), small("Leaf B")), small("Aside")),
		}},
		want: []string{
			"Top",
			"  Introduction to Top",
			"  Middle",
			"    Introduction to Middle",
			"    Leaf A",
			"    Leaf B",
			"  Aside",
		},
	}, {
		// R-3: no cap. The chain descends as far as the source nests, and the
		// skeleton is where that is decided.
		name: "no depth cap: descent follows the source all the way down",
		doc: docSpec{path: "chain.md", title: "Chain", secs: []secSpec{
			big("L1", big("L2", big("L3", small("L4a"), small("L4b")))),
		}},
		want: []string{
			"L1",
			"  Introduction to L1",
			"  L2",
			"    Introduction to L2",
			"    L3",
			"      Introduction to L3",
			"      L4a",
			"      L4b",
		},
	}, {
		// R-2: the preamble's convention and the body page's are one
		// convention, which is why they read identically.
		name: "the preamble and a container's body are named the same way",
		doc: docSpec{path: "pre.md", title: "Preface", pre: 40, secs: []secSpec{
			big("Cluster", small("Workers"), small("Scheduler")),
		}},
		want: []string{
			"Introduction to Preface",
			"Cluster",
			"  Introduction to Cluster",
			"  Workers",
			"  Scheduler",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			_, art := verifierFor(t, testParams(), tc.doc)
			f := fileOf(art, tc.doc.path)
			skel := Skeleton(f, testBudgets())

			if got := skelShape(skel); !slices.Equal(got, tc.want) {
				t.Fatalf("skeleton =\n%s\nwant\n%s",
					strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
			assertSkeletonTiles(t, f, skel)
		})
	}
}

// The body span carries the container's own heading line: the alternative
// drops those bytes from every delivered page and opens a second carve-out in
// the verbatim-custody story beside `metadata`.
func TestSkeletonBodyCarriesItsHeadingLine(t *testing.T) {
	doc := docSpec{path: "one.md", title: "One", secs: []secSpec{
		{title: "Cluster", paras: 4, words: 60, subs: []secSpec{
			{title: "Workers", paras: 2, words: 35}}},
	}}
	v, art := verifierFor(t, testParams(), doc)
	f := fileOf(art, "one.md")
	skel := Skeleton(f, testBudgets())

	if len(skel) != 1 || len(skel[0].Children) == 0 {
		t.Fatalf("the fixture did not descend: %q", skelShape(skel))
	}
	body := skel[0].Children[0]
	if body.Span.Start != f.Sections[0].Start {
		t.Errorf("body starts at %d, want the section's own first byte %d",
			body.Span.Start, f.Sections[0].Start)
	}
	src, _ := v.bytes("one.md")
	if head := string(src[body.Span.Start:]); !strings.HasPrefix(head, "# Cluster\n") {
		t.Errorf("the body page does not open with its container's heading line: %.20q", head)
	}
	if body.Span.End != f.Sections[0].Children[0].Start {
		t.Errorf("body ends at %d, want the first child's start %d",
			body.Span.End, f.Sections[0].Children[0].Start)
	}
}

// The skeleton is a function of the survey and the budgets and of nothing else
// — no bytes, no clock, no model — so the same corpus enumerates the same
// containers on every run. That is what lets the taxonomy stage (which holds
// only the survey) and this package's own composition agree about what exists.
func TestSkeletonIsDeterministicAndBudgetDriven(t *testing.T) {
	doc := docSpec{path: "one.md", title: "One", secs: []secSpec{
		{title: "Cluster", paras: 4, words: 60, subs: []secSpec{
			{title: "Workers", paras: 2, words: 35}, {title: "Scheduler", paras: 2, words: 35}}},
	}}
	_, art := verifierFor(t, testParams(), doc)
	f := fileOf(art, "one.md")

	first := skelShape(Skeleton(f, testBudgets()))
	if again := skelShape(Skeleton(f, testBudgets())); !slices.Equal(first, again) {
		t.Fatalf("two calls, two skeletons:\n%q\n%q", first, again)
	}
	if len(first) == 1 {
		t.Fatalf("the fixture does not descend at the test budget: %q", first)
	}

	// Raise the budget past the subtree and the same section is one page:
	// the rule is the leaf budget, and nothing else.
	b := testBudgets()
	b.LeafTokens = f.Sections[0].Tokens + 1
	if got := skelShape(Skeleton(f, b)); !slices.Equal(got, []string{"Cluster"}) {
		t.Fatalf("over the budget the section still descended: %q", got)
	}
}

// skelShape renders a skeleton as indented titles: the shape a test argues
// about, without the offsets that would make every case a table of magic
// numbers. assertSkeletonTiles asserts the offsets.
func skelShape(nodes []SkelNode) []string {
	var out []string
	var rec func(ns []SkelNode, depth int)
	rec = func(ns []SkelNode, depth int) {
		for _, n := range ns {
			out = append(out, strings.Repeat("  ", depth)+n.Title)
			rec(n.Children, depth+1)
		}
	}
	rec(nodes, 0)
	return out
}

// assertSkeletonTiles is the property every consumer leans on: a container's
// children tile its own span exactly, in byte order, with no gap and no
// overlap. It is what makes the coverage gate's boundary set satisfiable by
// descending into a container OR by paging the whole of it.
func assertSkeletonTiles(t *testing.T, f survey.File, nodes []SkelNode) {
	t.Helper()
	// The top level tiles the file's own coverage universe.
	lo, hi := coverageUniverse(sectionsOf(f))
	assertTiles(t, f.Path, survey.Span{Start: lo, End: hi}, nodes)
}

func assertTiles(t *testing.T, file string, span survey.Span, children []SkelNode) {
	t.Helper()
	if len(children) == 0 {
		return
	}
	at := span.Start
	for _, c := range children {
		if c.Span.Start != at {
			t.Fatalf("%s: %q starts at %d, want %d — the children of a container tile it",
				file, c.Title, c.Span.Start, at)
		}
		if c.Span.End <= c.Span.Start {
			t.Fatalf("%s: %q spans nothing", file, c.Title)
		}
		assertTiles(t, file, c.Span, c.Children)
		at = c.Span.End
	}
	if at != span.End {
		t.Fatalf("%s: children of [%d,%d) end at %d", file, span.Start, span.End, at)
	}
}

// The proposal every hermetic recipe runs is built FROM the skeleton, so the
// mechanical path descends identically to the one the taxonomy stage will be
// handed. Anything less and the offline oMLX recipes prove nothing about the
// feature.
func TestSourceStructureProposalDescendsWithTheSkeleton(t *testing.T) {
	// Sized so nothing but descent happens: every page is over the content
	// floor and under the leaf budget, and the container is over the budget
	// only as a subtree. The two operators that would otherwise also fire —
	// the floor and the splitter — have their own tests.
	doc := docSpec{path: "one.md", title: "One", pre: 80, secs: []secSpec{
		{title: "Cluster", paras: 2, words: 45, subs: []secSpec{
			{title: "Workers", paras: 2, words: 50}, {title: "Scheduler", paras: 2, words: 50}}},
		{title: "Aside", paras: 2, words: 35},
	}}
	v, art := verifierFor(t, testParams(), doc)

	p, err := SourceStructureProposal(art, testBudgets(), "Corpus", "all", nil)
	if err != nil {
		t.Fatalf("SourceStructureProposal: %v", err)
	}
	if got := proposalShape(p.Children); !slices.Equal(got, []string{
		"index One",
		"  page Introduction to One",
		"  index Cluster",
		"    page Introduction to Cluster",
		"    page Workers",
		"    page Scheduler",
		"  page Aside",
	}) {
		t.Fatalf("proposal shape:\n%s", strings.Join(got, "\n"))
	}

	// And it composes: a descended document is an ordinary tree plan, and the
	// coverage gate accepts a container covered by its subtree.
	s, err := v.Compose(p, nil)
	if err != nil {
		t.Fatalf("Compose over a descended proposal: %v", err)
	}
	for _, want := range []string{
		"one/index.md",
		"one/introduction-to-one.md",
		"one/cluster/index.md",
		"one/cluster/introduction-to-cluster.md",
		"one/cluster/workers.md",
		"one/aside.md",
	} {
		if _, ok := s.Node(want); !ok {
			t.Errorf("no node at %s; paths are %q", want, paths(s))
		}
	}
}

// proposalShape renders a proposal the way skelShape renders a skeleton, with
// the kind in front because that is the half the proposal adds.
func proposalShape(nodes []ProposalNode) []string {
	var out []string
	var rec func(ns []ProposalNode, depth int)
	rec = func(ns []ProposalNode, depth int) {
		for _, n := range ns {
			kind := "page"
			if n.Kind == KindIndex {
				kind = "index"
			}
			out = append(out, fmt.Sprintf("%s%s %s", strings.Repeat("  ", depth), kind, n.Title))
			rec(n.Children, depth+1)
		}
	}
	rec(nodes, 0)
	return out
}
