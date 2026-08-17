package dissect

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
	"kbase/internal/survey/markdown"
	"kbase/internal/tokens"
)

// The property over real material. The synthetic cases next door are built to
// hit the shapes the heuristic decides between; this one is built to hit the
// shapes nobody chose — a corpus written by technical writers who never heard
// of this splitter.
//
// It drives the Markdown adapter to get there, which is why internal/survey's
// import policy names this package: a property about real corpus sections
// needs a real artifact, and an artifact comes from an adapter. What crosses
// the seam is still only neutral types — section ranges and cut candidates —
// and no parser node reaches this package.

// rojoDocs and omlxDocs are the corpora the justfile pins and fetches
// (prep-test-integration-rojo, prep-test-integration-omlx), at the paths those
// recipes guarantee — oMLX is a monorepo fetched sparse, so its documents are
// the docs/ subdirectory of the clone, not the clone. Each corpus SKIPS
// independently when it is absent rather than fetching anything: a unit-test
// run must not reach the network, and the integration recipe is where fetching
// belongs.
const (
	rojoDocs = "test_data/transient/rojo.space/docs"
	omlxDocs = "test_data/transient/omlx/docs"
)

// pinnedCorpus is one corpus the property runs over: where its documents live,
// the recipe that puts them there, and where its evidence lands. The property
// is written once and swept over this table, so a second corpus is a row here
// and nothing else.
type pinnedCorpus struct {
	name string
	docs string
	// prep is the bare recipe name; the skip message names `just <prep>` so a
	// reader of a skipped run knows which fetch to run.
	prep string
	// evidence is where this corpus leaves its observational evidence, per
	// AGENTS.md's testing rule: the counts below are the result, and a result
	// that exists only in a t.Logf line is a result nobody can check twice. It
	// is under test_data/transient/ because it is test output — generated,
	// gitignored, and rewritten from scratch every run.
	evidence string
}

// corpora is the swept corpora. Rojo is a small hand-written docs site; oMLX is
// an ML runtime's docs/ tree out of a monorepo — different writers, different
// structural habits, the same claims.
func corpora() []pinnedCorpus {
	return []pinnedCorpus{
		{
			name:     "rojo",
			docs:     rojoDocs,
			prep:     "prep-test-integration-rojo",
			evidence: "test_data/transient/dissect-rojo",
		},
		{
			name:     "omlx",
			docs:     omlxDocs,
			prep:     "prep-test-integration-omlx",
			evidence: "test_data/transient/dissect-omlx",
		},
	}
}

// corpusBudgets is the swept operating points: the floor, two that bite on
// real sections, and one over most of them. The property and its evidence read
// the same list, so a point added to the sweep is a point that appears in both.
func corpusBudgets() []int { return []int{MinTokens, 120, 400, 1500} }

// TestSplitOverRealCorpusSections: for every section of every document in each
// pinned corpus, at every budget, Split either produces a list that Verify
// accepts or refuses the span outright. There is no third outcome, and that
// is the whole claim — a heuristic is allowed to choose badly and is not
// allowed to emit something the verifier would reject at the seam.
//
// One subtest per corpus, named for it, so an integration recipe can pin one
// (`-run 'TestSplitOverRealCorpusSections/omlx'`) and so an absent corpus skips
// only its own subtest.
//
// It writes what it saw to the corpus's evidence directory as it goes. The
// pass/fail is the claim; the per-budget tally is the measurement, and the
// measurement is what says whether the property is still finding the shapes it
// was written for — a sweep that quietly stopped starving anything at the floor
// is a sweep whose budgets have drifted off the material.
func TestSplitOverRealCorpusSections(t *testing.T) {
	for _, c := range corpora() {
		t.Run(c.name, func(t *testing.T) { splitOverCorpus(t, c) })
	}
}

// splitOverCorpus is the property itself, over one corpus.
func splitOverCorpus(t *testing.T, c pinnedCorpus) {
	art, corpus := realArtifact(t, c)

	ev := corpusEvidence{Corpus: art.Corpus}
	tally := map[int]*budgetTally{}
	for _, budget := range corpusBudgets() {
		tally[budget] = &budgetTally{Budget: budget}
		ev.Budgets = append(ev.Budgets, tally[budget])
	}

	spans := 0
	for _, f := range art.Files {
		u, ok := corpus.Doc(f.Path)
		if !ok {
			t.Fatalf("no source under custody for %s", f.Path)
		}
		for _, span := range spansOf(f) {
			for _, budget := range corpusBudgets() {
				spans++
				b := tally[budget]
				b.Spans++
				p := params(budget)
				cuts, err := Split(u.Bytes, span, f.Cuts, p)
				if err != nil {
					var starved StarvedRejection
					if !errors.As(err, &starved) {
						t.Fatalf("%s %+v at budget %d: %v", f.Path, span, budget, err)
					}
					b.Starved++
					b.StarvedSpans = append(b.StarvedSpans, starvedSpan{
						File:   f.Path,
						Start:  starved.Span.Start,
						End:    starved.Span.End,
						Tokens: starved.Tokens,
						Budget: starved.Budget,
					})
					continue
				}
				if verr := Verify(u.Bytes, span, f.Cuts, cuts, MoveWindows(u.Bytes, cuts, p), p); verr != nil {
					t.Fatalf("%s %+v at budget %d: Split produced a list its own verifier rejects: %v",
						f.Path, span, budget, verr)
				}
				b.Split++
				b.Parts += len(cuts)
				if len(cuts) > 1 {
					b.MultiPart++
				}
				if len(cuts) > b.MaxParts {
					b.MaxParts = len(cuts)
				}
			}
		}
	}
	// A property test that silently stopped finding subjects would pass
	// forever.
	if spans == 0 {
		t.Fatal("the corpus produced no spans to split")
	}
	ev.Spans = spans

	for _, b := range ev.Budgets {
		t.Logf("budget %d: %d spans, %d split into %d parts (%d multi-part, max %d), %d starved",
			b.Budget, b.Spans, b.Split, b.Parts, b.MultiPart, b.MaxParts, b.Starved)
	}
	writeEvidence(t, evidencePath(t, c), "stats.json", encodeEvidence(t, ev))
}

// corpusEvidence is the stats file, and its field order is the file's field
// order. Nothing on this path is a map: an artifact that reorders itself
// between runs cannot be diffed, and a diff is the whole reason to keep it.
type corpusEvidence struct {
	Corpus  survey.Totals  `json:"corpus"`
	Spans   int            `json:"spansTested"`
	Budgets []*budgetTally `json:"budgets"`
}

// budgetTally is one operating point: how many spans were offered, how they
// came out, and which ones the splitter refused outright.
type budgetTally struct {
	Budget int `json:"budget"`
	Spans  int `json:"spans"`
	Split  int `json:"split"`
	// MultiPart is the spans that actually needed cutting — the ones where
	// the greedy walk did any work at all. Parts is the leaves they and the
	// single-part spans became together.
	MultiPart    int           `json:"multiPart"`
	Parts        int           `json:"parts"`
	MaxParts     int           `json:"maxParts"`
	Starved      int           `json:"starved"`
	StarvedSpans []starvedSpan `json:"starvedSpans,omitempty"`
}

// starvedSpan is the material a starved refusal names — which is what a reader
// of this file wants next: WHICH bytes were too big for the budget.
type starvedSpan struct {
	File   string `json:"file"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Tokens int    `json:"tokens"`
	Budget int    `json:"budget"`
}

// encodeEvidence renders the stats with the survey artifact's own JSON
// discipline: HTML escaping off because paths and titles come from documents,
// indented because a human reads this, and one trailing newline because it is
// a text file.
func encodeEvidence(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		t.Fatalf("encode evidence: %v", err)
	}
	return buf.Bytes()
}

// evidencePath is the corpus's evidence directory resolved against the module
// root, since the test binary runs in the package directory.
func evidencePath(t *testing.T, c pinnedCorpus) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), filepath.FromSlash(c.evidence))
}

// writeEvidence fails the test when the evidence cannot be written. A test
// that quietly skipped its own record would leave the same empty directory as
// a test that never ran — which is the failure mode the rule exists to close.
func writeEvidence(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("make the evidence directory: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Logf("evidence: %s (%d bytes)", path, len(data))
}

// spansOf is the file itself plus every section at every depth — the spans a
// taxonomy tree plan will actually hand stage 4.
//
// The recursion is the point rather than thoroughness for its own sake: leaf
// assignments land on sections deep in the heading tree, and those are the
// small ones, where the minimum, the pre-merge and the starved refusal all
// live. Top-level sections alone are the sizes least likely to exercise any of
// it.
func spansOf(f survey.File) []survey.Span {
	spans := []survey.Span{{Start: 0, End: f.Bytes}}
	var walk func(secs []survey.Section)
	walk = func(secs []survey.Section) {
		for _, s := range secs {
			spans = append(spans, survey.Span{Start: s.Start, End: s.End})
			walk(s.Children)
		}
	}
	walk(f.Sections)
	return spans
}

// realArtifact surveys one pinned corpus, or skips.
func realArtifact(t *testing.T, c pinnedCorpus) (survey.Artifact, ingest.Corpus) {
	t.Helper()
	root := moduleRoot(t)
	docs := filepath.Join(root, filepath.FromSlash(c.docs))
	if _, err := os.Stat(docs); err != nil {
		t.Skipf("the pinned %s corpus is not present (%s); run `just %s`", c.name, c.docs, c.prep)
	}
	corpus, err := ingest.Walk(docs, markdown.Extensions(), log.Discard())
	if err != nil {
		t.Fatalf("ingest.Walk: %v", err)
	}
	art, err := markdown.Survey(corpus, tokens.Estimator{}, log.Discard())
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	return art, corpus
}

// moduleRoot walks up to the directory holding go.mod, so the corpus path is
// resolved against the module rather than against whatever directory the test
// binary was started in.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package under test")
		}
		dir = parent
	}
}
