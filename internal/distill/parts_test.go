package distill

import (
	"strings"
	"testing"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
	"kbase/internal/tokens"
	"kbase/internal/treeplan"
)

// The split-group fixture: ONE document cut into three parts, which is what
// the continuation edges and the per-part descriptors are about [MAD2: B-4].
//
// The tree plan is hand-authored rather than composed, because what is under
// test here is what stage 5 does with a plan that has a three-part group in it,
// and the shipped budgets would need a corpus of thousands of words to produce
// one. The cut list is the same artifact stage 4 writes: a tiling of the
// group's span.
const (
	longOpening = "# Long Document\n\nThe opening, which the first part carries with it.\n\n"
	longFirst   = "## First Movement\n\nThe first movement's body.\n\n"
	longSecond  = "## Second Movement\n\nThe second movement's body.\n\n"
	longThird   = "## Third Movement\n\nThe third movement's body.\n"

	longDoc = longOpening + longFirst + longSecond + longThird
)

type splitScene struct {
	plan treeplan.TreePlan
	dist *Distiller
}

func newSplitScene(t *testing.T) splitScene {
	t.Helper()
	corpus, err := ingest.New([]ingest.SourceDoc{{Path: "guide/long.md", Bytes: []byte(longDoc)}})
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	at := func(prefix string) int { return strings.Index(longDoc, prefix) }
	file := survey.File{
		Path: "guide/long.md", SHA256: corpus.Docs[0].SHA256, Bytes: len(longDoc),
		Tokens: tokens.Estimator{}.EstimateBytes([]byte(longDoc)),
		Sections: []survey.Section{{
			Level: 1, Title: "Long Document", Start: 0, End: len(longDoc),
			Children: []survey.Section{
				{Level: 2, Title: "First Movement", Start: at(longFirst), End: at(longSecond)},
				{Level: 2, Title: "Second Movement", Start: at(longSecond), End: at(longThird)},
				{Level: 2, Title: "Third Movement", Start: at(longThird), End: len(longDoc)},
			},
		}},
	}
	art, err := survey.Assemble(corpus, []survey.File{file}, log.Discard())
	if err != nil {
		t.Fatalf("survey.Assemble: %v", err)
	}

	plan := treeplan.TreePlan{
		Schema:     treeplan.SchemaVersion,
		CorpusHash: art.Corpus.ContentHash,
		Nodes: []treeplan.Node{
			{Path: "entry-point.md", Kind: treeplan.KindEntryPoint, Title: "Guide"},
			{Path: "long/index.md", Kind: treeplan.KindIndex, Parent: "entry-point.md",
				Title: "Long Document", Scope: "the long document"},
		},
		Groups: []treeplan.SplitGroup{{
			ID:     "g0001",
			Source: treeplan.Span{File: "guide/long.md", Start: 0, End: len(longDoc)},
			Budget: 400, Parts: 3,
		}},
	}
	for k, name := range []string{"long-document-1.md", "long-document-2.md", "long-document-3.md"} {
		plan.Nodes = append(plan.Nodes, treeplan.Node{
			Path: "long/" + name, Kind: treeplan.KindLeaf, Parent: "long/index.md",
			Title: partTitleFor(k + 1), Scope: "one document about one subject",
			SplitGroup: "g0001", Part: k + 1,
		})
	}
	cuts := map[string][]survey.Span{"g0001": {
		{Start: 0, End: at(longSecond)},
		{Start: at(longSecond), End: at(longThird)},
		{Start: at(longThird), End: len(longDoc)},
	}}
	d, err := New(plan, art, corpus, cuts,
		Provenance{CorpusHash: art.Corpus.ContentHash, BuildDate: "2026-08-13"})
	if err != nil {
		t.Fatalf("distill.New: %v", err)
	}
	return splitScene{plan: plan, dist: d}
}

// partTitleFor spells the part title the tree plan's namer spells (§3.3): the
// base title with the ordinal after it, which is what carries a part's identity
// onto its own page now that the H1 is synthesised from it [MAD2: B-8].
func partTitleFor(k int) string {
	return "Long Document (" + string(rune('0'+k)) + "/3)"
}

// Every part of a split group is rendered with the edges to its siblings, and
// with the title it was routed by — the two halves of "a reader who lands
// mid-series can tell where they are and keep going".
func TestRenderSplitPartNavigation(t *testing.T) {
	sc := newSplitScene(t)
	tests := []struct {
		part int
		want []string
	}{
		{1, []string{
			"[↑ long/index.md](index.md)",
			"[→ long/long-document-2.md](long-document-2.md)"}},
		{2, []string{
			"[↑ long/index.md](index.md)",
			"[← long/long-document-1.md](long-document-1.md)",
			"[→ long/long-document-3.md](long-document-3.md)"}},
		{3, []string{
			"[↑ long/index.md](index.md)",
			"[← long/long-document-2.md](long-document-2.md)"}},
	}
	for _, tt := range tests {
		t.Run(partTitleFor(tt.part), func(t *testing.T) {
			n := sc.node(t, tt.part)
			l, err := sc.dist.Render(n)
			if err != nil {
				t.Fatalf("Render %s: %v", n.Path, err)
			}
			_, body, ok := ParseFrontmatter(l.Data)
			if !ok {
				t.Fatalf("%s does not open with its frontmatter block", n.Path)
			}
			var got []string
			for _, link := range NavBlock(body) {
				got = append(got, navPrefix+link.Marker+" "+link.Label+"]("+link.Target+")")
			}
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("the navigation block is:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
			// The part states the identity it was routed by, which the old
			// suppression rule discarded whenever the slice opened on a source
			// heading [MAD2: B-8].
			if want := "# " + n.Title + "\n"; !strings.Contains(string(l.Data), want) {
				t.Errorf("%s carries no %q:\n%s", n.Path, strings.TrimSpace(want), l.Data)
			}
		})
	}
}

// The per-part descriptors: what each part turns out to hold, from the headings
// inside its own byte range — the post-cut half stage 3 could not have written,
// since the boundaries did not exist yet.
func TestDescriptorsNameEachPartsHeadings(t *testing.T) {
	sc := newSplitScene(t)
	got, err := sc.dist.Descriptors()
	if err != nil {
		t.Fatalf("Descriptors: %v", err)
	}
	want := map[string]string{
		"long/long-document-1.md": "Long Document → First Movement",
		"long/long-document-2.md": "Second Movement",
		"long/long-document-3.md": "Third Movement",
	}
	if len(got) != len(want) {
		t.Fatalf("Descriptors = %v, want one per part", got)
	}
	for path, w := range want {
		if got[path] != w {
			t.Errorf("%s is described as %q, want %q", path, got[path], w)
		}
	}
	// Sibling bullets are choosable, which is the property the descriptors
	// exist for: the shared scope is on every part, so it cannot be what
	// distinguishes them.
	seen := map[string]bool{}
	for _, d := range got {
		if seen[d] {
			t.Errorf("two parts carry the identical descriptor %q", d)
		}
		seen[d] = true
	}
}

// A part whose cut fell in open prose describes nothing rather than borrowing
// a heading it does not deliver.
func TestDescriptorsSkipAHeadinglessPart(t *testing.T) {
	sc := newSplitScene(t)
	// Move the second boundary back into the middle of the second movement's
	// body, so part 3 opens in prose and holds no heading at all.
	cuts := sc.dist.cuts["g0001"]
	mid := strings.Index(longDoc, "The third movement's body.")
	sc.dist.cuts["g0001"] = []survey.Span{cuts[0], {Start: cuts[1].Start, End: mid}, {Start: mid, End: len(longDoc)}}

	got, err := sc.dist.Descriptors()
	if err != nil {
		t.Fatalf("Descriptors: %v", err)
	}
	if _, ok := got["long/long-document-3.md"]; ok {
		t.Errorf("a part delivering no heading was described as %q", got["long/long-document-3.md"])
	}
	if got["long/long-document-2.md"] != "Second Movement → Third Movement" {
		t.Errorf("part 2 is described as %q, want both headings it now delivers",
			got["long/long-document-2.md"])
	}
}

// Distinctness is the property the descriptors EXIST for (SPEC §4.3): every
// part of one group shares one scope, so two bullets whose descriptors also
// coincide differ in nothing but the path. A source that repeats its heading
// titles across a cut boundary produces exactly that, and the tied parts fall
// back to their ordinals [GO M-6].
//
// The fallback is per collision: part 1's headings already tell it apart, so
// it keeps them.
func TestDescriptorsFallBackToOrdinalsWhenHeadingsTie(t *testing.T) {
	sc := newSplitScene(t)
	at := func(prefix string) int { return strings.Index(longDoc, prefix) }
	// One heading title repeated across both later cuts — `## Notes` twice, the
	// shape a manual with a per-chapter notes section has.
	sc.dist.headings["guide/long.md"] = []heading{
		{start: 0, title: "Long Document"},
		{start: at(longFirst), title: "Notes"},
		{start: at(longSecond), title: "Notes"},
		{start: at(longThird), title: "Notes"},
	}

	got, err := sc.dist.Descriptors()
	if err != nil {
		t.Fatalf("Descriptors: %v", err)
	}
	want := map[string]string{
		"long/long-document-1.md": "Long Document → Notes",
		"long/long-document-2.md": "2 of 3",
		"long/long-document-3.md": "3 of 3",
	}
	for path, w := range want {
		if got[path] != w {
			t.Errorf("%s is described as %q, want %q", path, got[path], w)
		}
	}
	seen := map[string]bool{}
	for path, d := range got {
		if seen[d] {
			t.Errorf("%s repeats the descriptor %q a sibling already carries", path, d)
		}
		seen[d] = true
	}
}

// The descriptor is the first channel putting arbitrary CORPUS bytes onto a
// page kbase composes, and an index page is not verbatim source. A heading
// carrying link syntax must arrive as text, or the bullet gains a destination
// the tree does not deliver and guarantee 1 refuses a delivery nobody can
// repair [GO M-7].
func TestDescriptorsNeutralizeCorpusMarkup(t *testing.T) {
	sc := newSplitScene(t)
	at := func(prefix string) int { return strings.Index(longDoc, prefix) }
	// What plainText yields for a heading written ``## `](x)` form`` — the code
	// span's text, verbatim, brackets and all.
	sc.dist.headings["guide/long.md"] = []heading{
		{start: at(longSecond), title: "](x) form"},
		{start: at(longThird), title: "*emphatic* <b>"},
	}

	got, err := sc.dist.Descriptors()
	if err != nil {
		t.Fatalf("Descriptors: %v", err)
	}
	for path, want := range map[string]string{
		"long/long-document-2.md": `\]\(x\) form`,
		"long/long-document-3.md": `\*emphatic\* \<b\>`,
	} {
		if got[path] != want {
			t.Errorf("%s is described as %q, want %q", path, got[path], want)
		}
	}

	// The property, stated where it bites: a down-link bullet carrying the
	// descriptor has exactly ONE destination — its own link to the part.
	for path, d := range got {
		bullet := []byte("- [A part](" + path + ") — the shared scope (this part: " + d + ")\n")
		ds := Destinations(bullet)
		if len(ds) != 1 || ds[0].Text(bullet) != path {
			var found []string
			for _, x := range ds {
				found = append(found, x.Text(bullet))
			}
			t.Errorf("the bullet for %s carries destinations %q, want only its own link:\n%s",
				path, found, bullet)
		}
	}
}

func (s splitScene) node(t *testing.T, part int) treeplan.Node {
	t.Helper()
	for _, n := range s.plan.Nodes {
		if n.Kind == treeplan.KindLeaf && n.Part == part {
			return n
		}
	}
	t.Fatalf("the fixture has no part %d", part)
	return treeplan.Node{}
}
