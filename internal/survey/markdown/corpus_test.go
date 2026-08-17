package markdown

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
	"kbase/internal/tokens"
)

// The resolver over real material. The synthetic corpus next door is built to
// hit the shapes resolution decides between; this one is built to hit the
// spelling conventions nobody chose — cross-references written by technical
// writers against a rendered site, in the two address spaces resolveTarget has
// to read.
//
// The census it pins is the measurement MAD #2's B-1 says nothing took: a
// corpus with a working link graph surveyed to `internal: 0` and no gate, log
// line or artifact said so. A pinned census is what makes that loud.

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

// pinnedCorpus is one corpus this property runs over: where its documents
// live, the recipe that puts them there, the link census the corpus at its
// pinned commit produces, and where its evidence lands.
type pinnedCorpus struct {
	name string
	docs string
	// prep is the bare recipe name; the skip message names `just <prep>` so a
	// reader of a skipped run knows which fetch to run.
	prep string
	// links is the whole census, not just the two numbers this work package
	// moved: the four counts partition every destination in the corpus, so
	// pinning all four is what makes a destination that changed CLASS —
	// rather than merely resolving — a failure here instead of a silent
	// re-classification.
	links survey.LinkTotals
	// resolved is every internal edge the corpus is expected to yield, as
	// `<file> -> <target>` => resolved document. Counts say how many; this
	// says which, which is the half a count cannot state.
	resolved map[string]string
	// evidence is where this corpus leaves its observational evidence, per
	// AGENTS.md's testing rule: a measurement worth taking is worth being able
	// to look at afterwards. It is under test_data/transient/ because it is
	// test output — generated, gitignored, and rewritten from scratch every run.
	evidence string
}

// corpora is the swept corpora. Rojo is a Docusaurus site whose cross-file
// references are written in URL space; oMLX is a monorepo docs/ tree whose one
// internal reference is written in file space. Between them they exercise both
// address spaces and the refusal that has to survive both.
func corpora() []pinnedCorpus {
	return []pinnedCorpus{
		{
			name:  "rojo",
			docs:  rojoDocs,
			prep:  "prep-test-integration-rojo",
			links: survey.LinkTotals{Internal: 4, Unresolved: 1, External: 31, Anchor: 50},
			resolved: map[string]string{
				// URL space: `project-format.md` serves at `project-format/`,
				// so `../properties` names the sibling document. In file space
				// these four leave the corpus root, which is why refusing an
				// escape before the second attempt made the whole set dead.
				"project-format.md -> ../properties":                         "properties.md",
				"project-format.md -> ../properties#bool":                    "properties.md",
				"project-format.md -> ../properties#property-type-support":   "properties.md",
				"properties.md -> ../project-format#instance-property-value": "project-format.md",
			},
			evidence: "test_data/transient/links-rojo",
		},
		{
			name:  "omlx",
			docs:  omlxDocs,
			prep:  "prep-test-integration-omlx",
			links: survey.LinkTotals{Internal: 1, Unresolved: 2, External: 11, Anchor: 0},
			resolved: map[string]string{
				// File space, and it must STAY file space: the URL-space
				// attempt runs second and may not re-point an edge the first
				// attempt already answered.
				"distributed-cluster.md -> heterogeneous-cluster.md": "heterogeneous-cluster.md",
			},
			evidence: "test_data/transient/links-omlx",
		},
	}
}

// TestLinkResolutionOverRealCorpus pins each pinned corpus's link census and
// every internal edge in it.
//
// The two unresolved sets it leaves behind are as load-bearing as the resolved
// ones and are recorded in the evidence: Rojo's remaining miss is an `.mdx`
// document ingest never took custody of, and oMLX's two are a file above the
// corpus root and a Python script. None of the three is a resolver failure,
// and a run that "fixed" them would be inventing targets.
//
// One subtest per corpus, named for it, so an integration recipe can pin one
// (`-run 'TestLinkResolutionOverRealCorpus/rojo'`) and so an absent corpus
// skips only its own subtest.
func TestLinkResolutionOverRealCorpus(t *testing.T) {
	for _, c := range corpora() {
		t.Run(c.name, func(t *testing.T) { linksOverCorpus(t, c) })
	}
}

func linksOverCorpus(t *testing.T, c pinnedCorpus) {
	art := realArtifact(t, c)

	if art.Corpus.Links != c.links {
		t.Errorf("link census = %+v, want %+v", art.Corpus.Links, c.links)
	}

	ev := linkEvidence{Corpus: art.Corpus.ContentHash, Links: art.Corpus.Links}
	got := map[string]string{}
	for _, f := range art.Files {
		for _, l := range f.Links {
			switch l.Kind {
			case survey.LinkInternal:
				got[f.Path+" -> "+l.Target] = l.Path
				ev.Internal = append(ev.Internal, edge{From: f.Path, Target: l.Target, Path: l.Path})
			case survey.LinkUnresolved:
				ev.Unresolved = append(ev.Unresolved, edge{From: f.Path, Target: l.Target})
			}
		}
	}
	for k, want := range c.resolved {
		if got[k] != want {
			t.Errorf("%s resolved to %q, want %q", k, got[k], want)
		}
	}
	for k, path := range got {
		if _, expected := c.resolved[k]; !expected {
			t.Errorf("%s resolved to %q, which this corpus is not pinned to produce", k, path)
		}
	}
	for _, e := range ev.Unresolved {
		t.Logf("%s: unresolved %q", e.From, e.Target)
	}
	writeEvidence(t, evidencePath(t, c), "links.json", encodeEvidence(t, ev))
}

// linkEvidence is the census with the edges behind it — the file a reader
// opens to ask which references a corpus actually has, rather than how many.
// Nothing on this path is a map: an artifact that reorders itself between runs
// cannot be diffed, and the artifact's own link order is already deterministic.
type linkEvidence struct {
	Corpus     string            `json:"corpusHash"`
	Links      survey.LinkTotals `json:"links"`
	Internal   []edge            `json:"internal"`
	Unresolved []edge            `json:"unresolved"`
}

// edge is one outbound reference: where it was written, what it said, and the
// document it named. Path is empty on an unresolved one, which is the whole
// difference between the two lists.
type edge struct {
	From   string `json:"from"`
	Target string `json:"target"`
	Path   string `json:"path,omitempty"`
}

// encodeEvidence renders the evidence with the survey artifact's own JSON
// discipline: HTML escaping off because paths and targets come from documents,
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

// realArtifact surveys one pinned corpus, or skips.
func realArtifact(t *testing.T, c pinnedCorpus) survey.Artifact {
	t.Helper()
	docs := filepath.Join(moduleRoot(t), filepath.FromSlash(c.docs))
	if _, err := os.Stat(docs); err != nil {
		t.Skipf("the pinned %s corpus is not present (%s); run `just %s`", c.name, c.docs, c.prep)
	}
	corpus, err := ingest.Walk(docs, Extensions(), log.Discard())
	if err != nil {
		t.Fatalf("ingest.Walk: %v", err)
	}
	art, err := Survey(corpus, tokens.Estimator{}, log.Discard())
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	return art
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
