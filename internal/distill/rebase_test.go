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

// The rebase fixture is a two-document corpus with a hand-authored link graph.
//
// It is hand-authored rather than driven through the Markdown adapter because
// of the format seam: this package is downstream of the survey artifact and
// its tests should be too. The pinned corpora are surveyed where the surveying
// is done (internal/survey/markdown's corpus test pins their link census); what
// this fixture owns is the REWRITING path, over a link graph shaped to reach
// every landing rule rather than whatever two real corpora happened to write.
type fixture struct {
	plan   treeplan.TreePlan
	art    survey.Artifact
	corpus ingest.Corpus
	intro  []byte
	sync   []byte
}

const (
	// filler carries each span over the content floor the tree plan enforces
	// (dissect.MinTokens): a span under it is merged into its neighbour, and
	// this fixture needs five distinct pages to have any landing rules to
	// test. It holds no brackets and no parentheses, so it adds no link.
	filler = "\nThis paragraph is here to carry the span it sits in over the minimum " +
		"amount of material a delivered page may hold, so that the fixture keeps the " +
		"shape its assertions are about: five spans, five pages, and a file that the " +
		"tree plan splits across more than one of them.\n"

	introPreamble = "Read [the sync page](sync.md), its [details](sync.md#details), " +
		"[the missing one](gone.md) and [the web](https://example.com).\n" + filler + "\n"
	introSection = "# Getting Started\n\nStart here.\n" + filler

	// sync.md carries the same-file anchors: one from its preamble, which the
	// split moves onto another page, and one from inside the section the
	// anchor names, which does not move. Both are the SAME destination text,
	// so the two renderings of it are the whole of what rule 1 has to get
	// right on a split file.
	syncPreamble = "How syncing works: [the details](#details), [sync itself](#sync).\n" + filler + "\n"
	syncBody     = "# Sync\n\nThe body.\n" + filler + "\n"
	syncDetails  = "# Details\n\nThe detail, and [this section](#details) again.\n" + filler
)

func newFixture(t *testing.T) fixture {
	t.Helper()
	intro := []byte(introPreamble + introSection)
	sync := []byte(syncPreamble + syncBody + syncDetails)

	corpus, err := ingest.New([]ingest.SourceDoc{
		{Path: "guide/intro.md", Bytes: intro},
		{Path: "guide/sync.md", Bytes: sync},
	})
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	est := tokens.Estimator{}
	files := []survey.File{
		{
			Path: "guide/intro.md", SHA256: corpus.Docs[0].SHA256, Bytes: len(intro), Tokens: est.EstimateBytes(intro),
			Preamble: &survey.Section{Level: 0, Start: 0, End: len(introPreamble)},
			Sections: []survey.Section{{
				Level: 1, Title: "Getting Started", Start: len(introPreamble), End: len(intro),
			}},
			Links: []survey.Link{
				{Kind: survey.LinkInternal, Target: "sync.md", Path: "guide/sync.md"},
				{Kind: survey.LinkInternal, Target: "sync.md#details", Path: "guide/sync.md", Fragment: "details"},
				{Kind: survey.LinkUnresolved, Target: "gone.md"},
				{Kind: survey.LinkExternal, Target: "https://example.com"},
			},
		},
		{
			Path: "guide/sync.md", SHA256: corpus.Docs[1].SHA256, Bytes: len(sync), Tokens: est.EstimateBytes(sync),
			Preamble: &survey.Section{Level: 0, Start: 0, End: len(syncPreamble)},
			Sections: []survey.Section{
				{Level: 1, Title: "Sync", Start: len(syncPreamble), End: len(syncPreamble) + len(syncBody)},
				{Level: 1, Title: "Details", Start: len(syncPreamble) + len(syncBody), End: len(sync)},
			},
			Links: []survey.Link{
				{Kind: survey.LinkAnchor, Target: "#details", Fragment: "details"},
				{Kind: survey.LinkAnchor, Target: "#sync", Fragment: "sync"},
			},
		},
	}
	art, err := survey.Assemble(corpus, files, log.Discard())
	if err != nil {
		t.Fatalf("survey.Assemble: %v", err)
	}
	proposal, err := treeplan.SourceStructureProposal(art, "Guide", "the guide corpus", nil)
	if err != nil {
		t.Fatalf("SourceStructureProposal: %v", err)
	}
	params := treeplan.DefaultParams()
	v, err := treeplan.NewVerifier(art, corpus, params, log.Discard())
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	plan, err := v.Compose(proposal, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	return fixture{plan: plan, art: art, corpus: corpus, intro: intro, sync: sync}
}

func (f fixture) distiller(t *testing.T) *Distiller {
	t.Helper()
	d, err := New(f.plan, f.art, f.corpus, nil,
		Provenance{CorpusHash: f.art.Corpus.ContentHash, BuildDate: "2026-08-13"})
	if err != nil {
		t.Fatalf("distill.New: %v", err)
	}
	return d
}

// nodeAt is the leaf drawing on one source file at one offset — how a test
// names a page without hard-coding a generated slug.
func (f fixture) nodeAt(t *testing.T, file string, offset int) treeplan.Node {
	t.Helper()
	for _, n := range f.plan.Nodes {
		if n.Kind != treeplan.KindLeaf {
			continue
		}
		g, _ := f.plan.SplitGroup(n.SplitGroup)
		if g.Source.File == file && offset >= g.Source.Start && offset < g.Source.End {
			return n
		}
	}
	t.Fatalf("no page draws on %s at %d", file, offset)
	return treeplan.Node{}
}

// domainOf is the index node one file's pages all sit under.
func (f fixture) domainOf(t *testing.T, file string) treeplan.Node {
	t.Helper()
	leaf := f.nodeAt(t, file, 0)
	for _, n := range f.plan.Nodes {
		if n.Path == leaf.Parent {
			return n
		}
	}
	t.Fatalf("no parent for %s", leaf.Path)
	return treeplan.Node{}
}

// The three landing rules of §4.5, over one rendered page.
func TestRebaseLandings(t *testing.T) {
	f := newFixture(t)
	d := f.distiller(t)

	page := f.nodeAt(t, "guide/intro.md", 0)
	got, err := d.Render(page)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	text := string(got.Data)

	// Rule 2 — a whole-file link to a file that became three pages lands on
	// the lowest index whose subtree holds them all.
	domain := f.domainOf(t, "guide/sync.md")
	wantWhole := "(" + RelPath(page.Path, domain.Path) + ")"
	if !strings.Contains(text, wantWhole) {
		t.Errorf("the whole-file link did not land on %s; want %q in:\n%s", domain.Path, wantWhole, text)
	}

	// Rule 1 — a fragment naming a source section lands on the page hosting
	// that section's start, and carries the fragment with it: the body is
	// verbatim, so the heading the anchor named is in that page.
	details := f.nodeAt(t, "guide/sync.md", len(syncPreamble)+len(syncBody))
	wantFrag := "(" + RelPath(page.Path, details.Path) + "#details)"
	if !strings.Contains(text, wantFrag) {
		t.Errorf("the fragment link did not land on %s; want %q in:\n%s", details.Path, wantFrag, text)
	}

	// Rule 3 — a target no node was drawn from stays verbatim and is
	// inventoried.
	if !strings.Contains(text, "(gone.md)") {
		t.Errorf("the unresolved link was rewritten; it must stay verbatim:\n%s", text)
	}
	if want := []string{"gone.md"}; !equalStrings(got.Unresolved, want) {
		t.Errorf("Unresolved = %q, want %q", got.Unresolved, want)
	}

	// Off-corpus targets are nobody's business here.
	if !strings.Contains(text, "(https://example.com)") {
		t.Errorf("the external link was touched:\n%s", text)
	}

	// Visible text is never rewritten — only targets (§4.5).
	for _, s := range []string{"[the sync page]", "[the details]", "[the missing one]", "[the web]"} {
		if strings.Contains(introPreamble, s) && !strings.Contains(text, s) {
			t.Errorf("the visible text %q did not survive the rebase:\n%s", s, text)
		}
	}
}

// A file that yields exactly one page lands on that page, not on its index —
// rule 2's first clause, which costs a hop of precision if it is dropped.
func TestRebaseSingleLeafFileLandsOnThePage(t *testing.T) {
	f := newFixture(t)
	// The rebase map is built over the plan, so this asks the map directly
	// rather than round-tripping through a body that has no such link.
	m, err := newRebaseMap(f.plan, f.art, nil)
	if err != nil {
		t.Fatalf("newRebaseMap: %v", err)
	}
	whole := wholeFileLandings(f.plan, mustRanges(t, f.plan))
	pages := 0
	for _, n := range f.plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			g, _ := f.plan.SplitGroup(n.SplitGroup)
			if g.Source.File == "guide/intro.md" {
				pages++
			}
		}
	}
	if pages != 2 {
		t.Fatalf("the fixture's intro yields %d pages; the rule under test needs the multi-page case", pages)
	}
	if got := whole["guide/intro.md"]; !strings.HasSuffix(got, "/index.md") {
		t.Errorf("a two-page file landed on %q, want its index", got)
	}
	if _, ok := m.byFile["guide/intro.md"]["sync.md"]; !ok {
		t.Error("the map holds no landing for a resolved internal target")
	}
}

func mustRanges(t *testing.T, plan treeplan.TreePlan) map[string][]hostRange {
	t.Helper()
	r, err := leafRanges(plan, nil)
	if err != nil {
		t.Fatalf("leafRanges: %v", err)
	}
	return r
}

// Rule 1 over a BARE fragment, which is the case a split strands: the anchor
// names a heading in the file it is written in, and the cut may have put that
// heading on another page.
//
// One destination, two pages, two correct renderings — which is why the map is
// a function of (source file, destination) read per emitting leaf and not a
// text substitution: the page hosting the heading keeps the source's own bare
// fragment, and every other page gets a path to it.
func TestRebaseSameFileFragments(t *testing.T) {
	f := newFixture(t)
	d := f.distiller(t)

	preamble := f.nodeAt(t, "guide/sync.md", 0)
	details := f.nodeAt(t, "guide/sync.md", len(syncPreamble)+len(syncBody))
	body := f.nodeAt(t, "guide/sync.md", len(syncPreamble))
	if preamble.Path == details.Path {
		t.Fatalf("the fixture must split sync.md; the preamble and Details share %s", preamble.Path)
	}

	from, err := d.Render(preamble)
	if err != nil {
		t.Fatalf("Render %s: %v", preamble.Path, err)
	}
	for _, want := range []string{
		"(" + RelPath(preamble.Path, details.Path) + "#details)",
		"(" + RelPath(preamble.Path, body.Path) + "#sync)",
	} {
		if !strings.Contains(string(from.Data), want) {
			t.Errorf("the moved anchor did not land; want %q in:\n%s", want, from.Data)
		}
	}
	if len(from.Unresolved) != 0 {
		t.Errorf("Unresolved = %q; a same-file anchor that landed is not left behind", from.Unresolved)
	}

	on, err := d.Render(details)
	if err != nil {
		t.Fatalf("Render %s: %v", details.Path, err)
	}
	if !strings.Contains(string(on.Data), "(#details)") {
		t.Errorf("an anchor landing on its own page must keep the source's bare fragment:\n%s", on.Data)
	}
}

// B-10's class: a site generator disambiguates repeated headings with `-N`,
// and the map has to read that suffix as an occurrence ordinal — while a
// heading whose own title ends in a number keeps its exact slug.
func TestSectionStartSuffixedDuplicates(t *testing.T) {
	anchors := map[string][]int{
		"install": {10, 200, 300},
		"step-2":  {400},
	}
	for _, tc := range []struct {
		name     string
		fragment string
		want     int
		found    bool
	}{
		{"bare slug names the first occurrence", "install", 10, true},
		{"-1 names the second", "install-1", 200, true},
		{"-2 names the third", "install-2", 300, true},
		{"past the last occurrence does not land", "install-3", 0, false},
		{"an exact slug wins over reading its tail as an ordinal", "step-2", 400, true},
		{"a leading zero is not the generator's suffix", "install-01", 0, false},
		{"a non-numeric tail is part of the slug", "install-next", 0, false},
		{"nothing of that name at all", "gone", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := sectionStart(anchors, tc.fragment)
			if ok != tc.found || got != tc.want {
				t.Errorf("sectionStart(%q) = %d, %v; want %d, %v", tc.fragment, got, ok, tc.want, tc.found)
			}
		})
	}
}

// sectionIndex records every occurrence of a repeated heading, in document
// order — the input the ordinal above is read against.
func TestSectionIndexKeepsEveryOccurrence(t *testing.T) {
	art := survey.Artifact{Files: []survey.File{{
		Path: "dup.md",
		Sections: []survey.Section{
			{Level: 1, Title: "Install", Start: 0, End: 50, Children: []survey.Section{
				{Level: 2, Title: "Install", Start: 20, End: 50},
			}},
			{Level: 1, Title: "Install", Start: 50, End: 90},
		},
	}}}
	got := sectionIndex(art)["dup.md"]["install"]
	if want := []int{0, 20, 50}; !equalInts(got, want) {
		t.Errorf("occurrences = %v, want %v in document order", got, want)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAnchorSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Details", "details"},
		{"Getting Started", "getting-started"},
		{"Rojo 7: What's New?", "rojo-7-whats-new"},
		{"  Spaced  Out  ", "spaced--out"},
		{"---", ""},
		{"Under_scores", "under-scores"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := anchorSlug(tt.in); got != tt.want {
				t.Errorf("anchorSlug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Every page's body must be the source bytes it names, verbatim apart from the
// rebase — the property stage 9 check 7 exists to keep true.
func TestRenderIsVerbatimApartFromTargets(t *testing.T) {
	f := newFixture(t)
	d := f.distiller(t)
	for _, n := range d.Leaves() {
		got, err := d.Render(n)
		if err != nil {
			t.Fatalf("Render %s: %v", n.Path, err)
		}
		g, _ := f.plan.SplitGroup(n.SplitGroup)
		doc, _ := f.corpus.Doc(g.Source.File)
		body := string(doc.Bytes[g.Source.Start:g.Source.End])
		// Everything outside a link destination is byte-identical, so every
		// non-link line of the source appears in the page unchanged.
		for _, line := range strings.Split(body, "\n") {
			if strings.TrimSpace(line) == "" || strings.Contains(line, "](") {
				continue
			}
			if !strings.Contains(string(got.Data), line) {
				t.Errorf("%s dropped the source line %q", n.Path, line)
			}
		}
	}
}

func TestRenderRefusesNonLeaf(t *testing.T) {
	f := newFixture(t)
	d := f.distiller(t)
	for _, n := range f.plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			continue
		}
		if _, err := d.Render(n); err == nil {
			t.Errorf("Render(%s) accepted a %s", n.Path, n.Kind)
		}
	}
}

func TestNewRefusesMismatchedCorpus(t *testing.T) {
	f := newFixture(t)
	bad := f.plan
	bad.CorpusHash = "not the corpus"
	if _, err := New(bad, f.art, f.corpus, nil,
		Provenance{CorpusHash: "x", BuildDate: "2026-08-13"}); err == nil {
		t.Fatal("a tree plan for another corpus was accepted")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
