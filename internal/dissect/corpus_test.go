package dissect

import (
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

// rojoDocs is the corpus the justfile pins and fetches
// (prep-test-integration-rojo). This test SKIPS when it is absent rather than
// fetching anything: a unit-test run must not reach the network, and the
// integration recipe is where fetching belongs.
const rojoDocs = "test_data/transient/rojo.space/docs"

// TestSplitOverRealCorpusSections: for every section of every document in the
// pinned corpus, at every budget, Split either produces a list that Verify
// accepts or refuses the span outright. There is no third outcome, and that
// is the whole claim — a heuristic is allowed to choose badly and is not
// allowed to emit something the verifier would reject at the seam.
func TestSplitOverRealCorpusSections(t *testing.T) {
	art, corpus := realArtifact(t)

	spans := 0
	for _, f := range art.Files {
		u, ok := corpus.Unit(f.Path)
		if !ok {
			t.Fatalf("no source under custody for %s", f.Path)
		}
		for _, span := range spansOf(f) {
			for _, budget := range []int{minTokens, 120, 400, 1500} {
				spans++
				cuts, err := Split(u.Bytes, span, f.Cuts, budget)
				if err != nil {
					var starved StarvedError
					if !asStarved(err, &starved) {
						t.Fatalf("%s %+v at budget %d: %v", f.Path, span, budget, err)
					}
					continue
				}
				if verr := Verify(u.Bytes, span, f.Cuts, cuts, Windows(u.Bytes, cuts)); verr != nil {
					t.Fatalf("%s %+v at budget %d: Split produced a list its own verifier rejects: %v",
						f.Path, span, budget, verr)
				}
			}
		}
	}
	// A property test that silently stopped finding subjects would pass
	// forever.
	if spans == 0 {
		t.Fatal("the corpus produced no spans to split")
	}
}

// spansOf is the file itself plus each of its top-level sections — the spans
// a taxonomy skeleton will actually hand stage 4, at both the sizes it hands
// them.
func spansOf(f survey.File) []survey.Range {
	spans := []survey.Range{{Start: 0, End: f.Bytes}}
	for _, s := range f.Sections {
		spans = append(spans, survey.Range{Start: s.Start, End: s.End})
	}
	return spans
}

// asStarved is errors.As with the one target this file cares about, named so
// the assertion above reads as the claim it makes.
func asStarved(err error, target *StarvedError) bool {
	s, ok := err.(StarvedError)
	if ok {
		*target = s
	}
	return ok
}

// realArtifact surveys the pinned corpus, or skips.
func realArtifact(t *testing.T) (survey.Artifact, ingest.Corpus) {
	t.Helper()
	root := moduleRoot(t)
	docs := filepath.Join(root, filepath.FromSlash(rojoDocs))
	if _, err := os.Stat(docs); err != nil {
		t.Skipf("the pinned corpus is not present (%s); run `just prep-test-integration-rojo`", rojoDocs)
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
