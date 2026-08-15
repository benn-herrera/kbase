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
// It is hand-authored rather than driven through the Markdown adapter for two
// reasons, and the second is the one that matters. The first is the format
// seam: this package is downstream of the survey artifact and its tests should
// be too. The second is that the PINNED corpus resolves zero internal links —
// Rojo's own docs use site-root-relative destinations that do not match its
// file layout, so every cross-document link there is `LinkUnresolved` and the
// rebase map lands nothing. A corpus test over Rojo therefore proves the
// inventory path and nothing about the rewriting path; this fixture proves the
// rewriting path, and the two together are the coverage.
type fixture struct {
	plan   treeplan.TreePlan
	art    survey.Artifact
	corpus ingest.Corpus
	intro  []byte
	sync   []byte
}

const (
	introPreamble = "Read [the sync page](sync.md), its [details](sync.md#details), " +
		"[the missing one](gone.md) and [the web](https://example.com).\n\n"
	introSection = "# Getting Started\n\nStart here.\n"

	syncPreamble = "How syncing works.\n\n"
	syncBody     = "# Sync\n\nThe body.\n\n"
	syncDetails  = "# Details\n\nThe detail.\n"
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
	v, err := treeplan.NewVerifier(art, corpus, params)
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

// The three landing rules of §4.4, over one rendered page.
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

	// Visible text is never rewritten — only targets (§4.4).
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
