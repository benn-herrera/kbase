package survey

import (
	"strings"
	"testing"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/log/logtest"
)

// twoDocuments is the corpus the Assemble cases run against: two documents of
// known length, so a per-file inventory can be written by hand and every
// total is checkable by eye. Nothing here parses anything — Assemble's job is
// arithmetic and refusal, both of which are format-independent.
func twoDocuments(t *testing.T) ingest.Corpus {
	t.Helper()
	c, err := ingest.New([]ingest.SourceDoc{
		{Path: "a.md", Bytes: []byte(strings.Repeat("a", 10))},
		{Path: "b.md", Bytes: []byte(strings.Repeat("b", 20))},
	})
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	return c
}

// inventory is a well-formed pair of files for twoDocuments: one headingless,
// one with a nested heading tree, together covering every link kind.
func inventory() []File {
	return []File{{
		Path:     "a.md",
		Bytes:    10,
		Tokens:   3,
		Preamble: &Section{Start: 0, End: 10},
		Links: []Link{
			{Kind: LinkInternal, Target: "b.md", Path: "b.md"},
			{Kind: LinkUnresolved, Target: "gone.md"},
		},
	}, {
		Path:   "b.md",
		Bytes:  20,
		Tokens: 5,
		Sections: []Section{{Level: 1, Start: 0, End: 20, Children: []Section{
			{Level: 2, Start: 5, End: 12},
			{Level: 2, Start: 12, End: 20},
		}}},
		Links: []Link{
			{Kind: LinkExternal, Target: "https://example.com"},
			{Kind: LinkAnchor, Target: "#here", Fragment: "here"},
			{Kind: LinkUnresolved, Target: "../outside.md"},
		},
	}}
}

// TestAssembleTotals: the roll-up is the sum of what the per-file entries
// say, the provenance is the corpus's own content hash, and the corpus's
// unresolved-link budget is announced rather than buried in the artifact.
func TestAssembleTotals(t *testing.T) {
	corpus := twoDocuments(t)
	lg := &logtest.Capture{}

	art, err := Assemble(corpus, inventory(), lg)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	if art.Schema != SchemaVersion {
		t.Errorf("schema = %q, want %q", art.Schema, SchemaVersion)
	}
	if art.Corpus.ContentHash != corpus.ContentHash {
		t.Error("the artifact must carry the corpus content hash as its provenance")
	}
	want := Totals{
		ContentHash: corpus.ContentHash,
		Files:       2,
		Bytes:       30,
		Tokens:      8,
		// a.md's preamble, b.md's heading, and its two subheadings.
		Sections: 4,
		Links:    LinkTotals{Internal: 1, Unresolved: 2, External: 1, Anchor: 1},
	}
	if art.Corpus != want {
		t.Errorf("totals = %+v, want %+v", art.Corpus, want)
	}
	if !lg.Has(t, "warn", "count", 2) {
		t.Error("the corpus's unresolved-link total must be warned about; stage 8 has to answer for it")
	}
}

// TestAssembleNoUnresolvedIsQuiet: the warning is a budget report, not a
// ritual — a clean corpus produces none.
func TestAssembleNoUnresolvedIsQuiet(t *testing.T) {
	corpus := twoDocuments(t)
	files := inventory()
	files[0].Links = files[0].Links[:1]
	files[1].Links = files[1].Links[:2]

	lg := &logtest.Capture{}
	if _, err := Assemble(corpus, files, lg); err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	for _, r := range lg.Snapshot() {
		if r.Level == "warn" {
			t.Errorf("a corpus with no unresolved links must warn about nothing; got %+v", r)
		}
	}
}

// TestAssembleRefusesBadInventory is the custody cross-check: the artifact
// describes bytes somebody else is holding, so every file has to name a
// document the corpus actually has, at the length the corpus actually holds,
// with ranges that cover it. An adapter that drops, reorders or miscounts a
// document fails here rather than several stages later, where the symptom is
// a leaf quietly missing its source.
func TestAssembleRefusesBadInventory(t *testing.T) {
	corpus := twoDocuments(t)

	for _, tc := range []struct {
		name  string
		files func() []File
		want  string
	}{{
		name:  "a document was dropped",
		files: func() []File { return inventory()[:1] },
		want:  "inventory holds 1 files, the corpus holds 2",
	}, {
		name: "files arrive out of corpus order",
		files: func() []File {
			f := inventory()
			return []File{f[1], f[0]}
		},
		want: `inventory 0 is "b.md", the corpus holds "a.md"`,
	}, {
		name: "a file names a document the corpus does not hold",
		files: func() []File {
			f := inventory()
			f[1].Path = "elsewhere.md"
			return f
		},
		want: `the corpus holds "b.md"`,
	}, {
		name: "the reported length disagrees with custody",
		files: func() []File {
			f := inventory()
			f[0].Bytes = 9
			return f
		},
		want: "a.md: inventory reports 9 bytes, custody holds 10",
	}, {
		name: "the ranges do not cover the document",
		files: func() []File {
			f := inventory()
			secs := f[1].Sections
			secs[0].End, secs[0].Children[1].End = 18, 18
			return f
		},
		want: "b.md: sections cover 18 bytes, file is 20",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			art, err := Assemble(corpus, tc.files(), log.Discard())
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
			if len(art.Files) != 0 {
				t.Errorf("a refused inventory must not yield a partial artifact; got %+v", art)
			}
		})
	}
}
