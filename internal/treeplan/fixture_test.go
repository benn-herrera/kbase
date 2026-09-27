package treeplan

import (
	"fmt"
	"strings"
	"testing"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
	"kbase/internal/tokens"
)

// The synthetic corpus for this package's tests.
//
// Documents are built out of headed sections of filler paragraphs, and the
// candidate set handed back is the one an adapter would have enumerated for
// them: a heading candidate at every section but the first, and a paragraph
// candidate BETWEEN paragraphs. The gap in that last rule is what makes the
// starved case writable — a section of one long paragraph offers the splitter
// nowhere to cut, which is a real shape (a giant table, one enormous code
// fence) and the one §2.4 step 4 exists for.
//
// Every fixture goes through survey.Assemble, so a fixture whose offsets do
// not tile fails as loudly here as an adapter's would in production. The tests
// are then arguing about tree structure and never about whether the material
// under it is well-formed.
//
// # Sections are sized over the content floor
//
// Every section here is written large enough to clear dissect.MinTokens (64
// tokens, ~256 bytes), because a span under it is not a page: Compose merges it
// into its neighbour. (A document entirely under the floor is the one
// exemption — its sole span stands as a leaf at any size, ruled 2026-08-17 —
// and the test that asserts it says so.) A fixture below the floor therefore
// tests the floor and nothing else, whatever its author meant it to test — the
// floor is an appliance constant and does not scale down with testBudgets the
// way the leaf budget does.

// secSpec is one section of a synthetic document.
//
// subs are its child sections, written one heading level deeper. They are what
// makes a document DESCENDABLE: rule R-C needs a section that is both over the
// leaf budget and has children, and a fixture with no nesting can only ever
// exercise the split path.
type secSpec struct {
	title string
	paras int
	words int
	subs  []secSpec
}

// docSpec is one synthetic document.
//
// pre is the headingless opening, in words: the preamble the survey records
// separately from the heading tree, and the span whose naming convention a
// container's body page reuses (R-2).
//
// meta writes a front-matter block. It is out-of-band: the survey records it
// as a range and NOT as a section, so it sits outside the coverage universe and
// no page may draw on it.
type docSpec struct {
	path  string
	title string
	meta  bool
	pre   int
	secs  []secSpec
}

// buildCorpus turns document specs into the pair every entry point of this
// package takes: the survey artifact and the source under custody.
func buildCorpus(t *testing.T, docs ...docSpec) (survey.Artifact, ingest.Corpus) {
	t.Helper()
	units := make([]ingest.SourceDoc, 0, len(docs))
	files := make([]survey.File, 0, len(docs))
	for _, d := range docs {
		u, f := buildDoc(d)
		units = append(units, u)
		files = append(files, f)
	}
	corpus, err := ingest.New(units)
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	// ingest.New sorts by path; the survey artifact is one file per document
	// in corpus order, so the inventory follows.
	sorted := make([]survey.File, 0, len(files))
	for _, u := range corpus.Docs {
		for _, f := range files {
			if f.Path == u.Path {
				sorted = append(sorted, f)
			}
		}
	}
	art, err := survey.Assemble(corpus, sorted, log.Discard())
	if err != nil {
		t.Fatalf("survey.Assemble: %v", err)
	}
	return art, corpus
}

func buildDoc(d docSpec) (ingest.SourceDoc, survey.File) {
	var sb strings.Builder
	var cands []survey.CutCandidate
	est := tokens.Estimator{}

	var metadata *survey.Span
	if d.meta {
		fmt.Fprintf(&sb, "---\ntitle: %s\n---\n\n", d.title)
		metadata = &survey.Span{Start: 0, End: sb.Len()}
	}

	var preamble *survey.Section
	if d.pre > 0 {
		start := sb.Len()
		sb.WriteString(filler("pre", d.pre))
		sb.WriteString("\n\n")
		preamble = &survey.Section{
			Level: 0, Start: start, End: sb.Len(),
			Tokens: est.Estimate(sb.String()[start:sb.Len()]),
			Gist:   "gist of the opening",
		}
	}

	sections := writeSections(&sb, &cands, d.secs, 1, est)
	src := []byte(sb.String())
	return ingest.SourceDoc{Path: d.path, Bytes: src}, survey.File{
		Path:     d.path,
		Bytes:    len(src),
		Tokens:   est.EstimateBytes(src),
		Title:    d.title,
		Metadata: metadata,
		Preamble: preamble,
		Sections: sections,
		Cuts:     cands,
	}
}

// writeSections writes one level of sections and recurses into their children,
// returning the heading tree with the offsets it just wrote.
//
// A section's range runs from its own heading line to the end of its last
// child, which is survey.Section's contract and what makes the ranges tile —
// survey.Assemble refuses the fixture otherwise. Its token count covers the
// whole subtree, for the same reason: that is the number rule R-C is stated in.
func writeSections(sb *strings.Builder, cands *[]survey.CutCandidate, specs []secSpec,
	level int, est tokens.Estimator) []survey.Section {

	out := make([]survey.Section, 0, len(specs))
	for _, s := range specs {
		start := sb.Len()
		if start > 0 {
			// Offset zero is not a cut position: a cut there would produce an
			// empty first part.
			*cands = append(*cands, survey.CutCandidate{Offset: start, Kind: survey.CutHeading})
		}
		fmt.Fprintf(sb, "%s %s\n\n", strings.Repeat("#", level), s.title)
		for p := range s.paras {
			if p > 0 {
				*cands = append(*cands, survey.CutCandidate{Offset: sb.Len(), Kind: survey.CutParagraph})
			}
			sb.WriteString(filler(fmt.Sprintf("%s%d", strings.ToLower(s.title[:1]), p), s.words))
			sb.WriteString("\n\n")
		}
		kids := writeSections(sb, cands, s.subs, level+1, est)
		out = append(out, survey.Section{
			Level:    level,
			Title:    s.title,
			Start:    start,
			End:      sb.Len(),
			Tokens:   est.Estimate(sb.String()[start:sb.Len()]),
			Gist:     "gist of " + s.title,
			Children: kids,
		})
	}
	return out
}

// filler is n space-separated words, tagged so a failure message says which
// paragraph the bytes came from.
func filler(tag string, n int) string {
	parts := make([]string, 0, n)
	for i := range n {
		parts = append(parts, fmt.Sprintf("%s%02d", tag, i%100))
	}
	return strings.Join(parts, " ")
}

// testBudgets is a small operating point: the same arithmetic as §9's, three
// orders of magnitude down, so a fixture that exercises a budget is a few
// hundred bytes rather than a megabyte.
func testBudgets() Budgets {
	return Budgets{
		LeafTokens:         200,
		SummaryInputTokens: 2000,
		SummaryTokens:      50,
		EntryPointTokens:   300,
		FanOutCap:          4,
		CandidateCap:       10,
	}
}

func testParams() Params { return Params{Budgets: testBudgets()} }

// verifierFor is the verifier under test over a synthetic corpus.
func verifierFor(t *testing.T, p Params, docs ...docSpec) (*Verifier, survey.Artifact) {
	t.Helper()
	art, corpus := buildCorpus(t, docs...)
	v, err := NewVerifier(art, corpus, p, log.Discard())
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v, art
}

// leafFor is a plan leaf over one surveyed section.
func leafFor(art survey.Artifact, file string, section int, title string) ProposalNode {
	f := fileOf(art, file)
	s := f.Sections[section]
	return ProposalNode{
		Title:   title,
		Scope:   "scope of " + title,
		Kind:    KindLeaf,
		Sources: []Span{{File: file, Start: s.Start, End: s.End}},
	}
}

// wholeFileLeaf is a plan leaf over one whole document.
func wholeFileLeaf(art survey.Artifact, file, title string) ProposalNode {
	f := fileOf(art, file)
	return ProposalNode{
		Title:   title,
		Scope:   "scope of " + title,
		Kind:    KindLeaf,
		Sources: []Span{{File: file, Start: 0, End: f.Bytes}},
	}
}

// indexNode is a plan index over children.
func indexNode(title string, children ...ProposalNode) ProposalNode {
	return ProposalNode{Title: title, Scope: "scope of " + title, Kind: KindIndex, Children: children}
}

func fileOf(art survey.Artifact, path string) survey.File {
	for _, f := range art.Files {
		if f.Path == path {
			return f
		}
	}
	panic("no such file in the fixture: " + path)
}

// paths is every node path of a tree plan, in artifact order.
func paths(s TreePlan) []string {
	out := make([]string, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		out = append(out, n.Path)
	}
	return out
}

// levelsOf is each node's level, computed from the parent chain the way every
// consumer of the artifact has to.
func levelsOf(s TreePlan) map[string]int {
	levels := map[string]int{}
	for _, n := range s.Nodes {
		if n.Parent == "" {
			levels[n.Path] = 1
			continue
		}
		levels[n.Path] = levels[n.Parent] + 1
	}
	return levels
}

func maxLevel(s TreePlan) int {
	max := 0
	for _, l := range levelsOf(s) {
		if l > max {
			max = l
		}
	}
	return max
}

func leafCount(s TreePlan) int {
	n := 0
	for _, node := range s.Nodes {
		if node.Kind == KindLeaf {
			n++
		}
	}
	return n
}
