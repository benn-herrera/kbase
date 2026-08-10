// The artifact's two load-bearing properties — its ranges tile the source
// under custody, and the same corpus serializes to the same bytes — are
// properties of the ARTIFACT, so they are tested here rather than in whichever
// adapter happens to exist. Checking them means producing a real artifact,
// which means driving a real adapter, so this is an external test package:
// internal/survey itself must never import an adapter (it would be an import
// cycle, and importpolicy_test.go allows the import here for test files only).
//
// The corpus below is deliberately small and shape-driven — one document per
// tiling shape. The exhaustive parsing fixtures live with the adapter, whose
// job the parsing is.
package survey_test

import (
	"bytes"
	"strings"
	"testing"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
	"kbase/internal/survey/markdown"
	"kbase/internal/tokens"
)

var shapes = map[string]string{
	// A metadata block, then a preamble, then a heading tree: all three
	// range kinds in one file.
	"front.md": "---\ntitle: Front Matter Doc\ntags: [a, b]\n---\n\nOpening prose.\n\n# Heading\n\nBody.\n",
	// Nested and skipped levels: children must tile their parent exactly.
	"nested.md": "# Guide\n\nIntro.\n\n### Deep skip\n\nSkipped body.\n\n## Second\n\nMore.\n",
	// No headings at all: the whole file is preamble.
	"flat.md": "Just a paragraph.\n\nAnd another.\n",
	// No bytes at all: no ranges, and the tiling still has to add up.
	"empty.md": "",
	// CRLF and a missing trailing newline: the offsets are byte offsets, and
	// nothing normalizes the custody bytes on the way past.
	"crlf.md": "# CRLF\r\n\r\nBody.\r\n\r\n## Sub\r\n\r\nMore body without a final newline",
	// A BOM is folded into the metadata range rather than trimmed, so byte 0
	// is still covered.
	"bom.md": "\uFEFF---\ntitle: BOM Doc\n---\n\n# After The BOM\n\nBody.\n",
	// Links exist so the artifact's link ordering is on the determinism path.
	"guide/links.md": "# Links\n\n[up](../nested.md) [missing](./gone.md) [ext](https://example.com/x)\n![img](../img/logo.png)\n",
}

// surveyShapes ingests the corpus in memory and surveys it with the Markdown
// adapter.
func surveyShapes(t *testing.T, files map[string]string) (ingest.Corpus, survey.Artifact) {
	t.Helper()
	units := make([]ingest.Unit, 0, len(files))
	for p, body := range files {
		units = append(units, ingest.Unit{Path: p, Bytes: []byte(body)})
	}
	corpus, err := ingest.New(units)
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	art, err := markdown.Survey(corpus, tokens.Estimator{}, log.Discard())
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	return corpus, art
}

// TestTilingProperty is the property the whole offset discipline rests on:
// the metadata range, the preamble, and every section's own body,
// concatenated in document order, reproduce the source byte for byte. It
// reconstructs each file from the artifact's offsets alone, so it fails on a
// gap, an overlap, or a misplaced boundary independently of the arithmetic in
// the production tiling check.
func TestTilingProperty(t *testing.T) {
	corpus, art := surveyShapes(t, shapes)
	for _, f := range art.Files {
		t.Run(f.Path, func(t *testing.T) {
			u, ok := corpus.Unit(f.Path)
			if !ok {
				t.Fatalf("no source unit for %q", f.Path)
			}
			var b bytes.Buffer
			if r := f.Metadata; r != nil {
				b.Write(u.Bytes[r.Start:r.End])
			}
			if p := f.Preamble; p != nil {
				b.Write(u.Bytes[p.Start:p.End])
			}
			for _, s := range f.Sections {
				writeOwnBodies(&b, u.Bytes, s)
			}
			if !bytes.Equal(b.Bytes(), u.Bytes) {
				t.Errorf("sections do not tile %s:\n got %q\nwant %q", f.Path, b.String(), u.Bytes)
			}
		})
	}
}

// writeOwnBodies appends a section's own body (heading line through its first
// child) followed by its children's, recursively.
func writeOwnBodies(b *bytes.Buffer, src []byte, s survey.Section) {
	own := s.End
	if len(s.Children) > 0 {
		own = s.Children[0].Start
	}
	b.Write(src[s.Start:own])
	for _, c := range s.Children {
		writeOwnBodies(b, src, c)
	}
}

// TestShapesCoverTheRangeKinds guards the corpus above: if it stops carrying
// a metadata block, a preamble, or a nested tree, the tiling property quietly
// stops being tested over the shapes that can break it.
func TestShapesCoverTheRangeKinds(t *testing.T) {
	_, art := surveyShapes(t, shapes)

	var metadata, preamble, nested bool
	for _, f := range art.Files {
		metadata = metadata || f.Metadata != nil
		preamble = preamble || f.Preamble != nil
		for _, s := range f.Sections {
			nested = nested || len(s.Children) > 0
		}
	}
	if !metadata || !preamble || !nested {
		t.Errorf("the shape corpus must exercise every range kind; metadata=%v preamble=%v nested=%v",
			metadata, preamble, nested)
	}
}

// TestDeterminism: the artifact is the pipeline's reproducible input to the
// taxonomy stage, so the same corpus must serialize to identical bytes,
// including when the units arrive in a different order.
//
// Metadata labels are on the corpus deliberately: they are strings on the
// marshal path, and a map-backed collection introduced there would break
// reproducibility exactly as a map on the section path would. (The
// machine-path check — that no absolute path leaks in — lives in
// cmd/survey_test.go, over a real temporary directory; ingest.Corpus carries
// no root for this test to leak.)
func TestDeterminism(t *testing.T) {
	units := make([]ingest.Unit, 0, len(shapes))
	for p, body := range shapes {
		units = append(units, ingest.Unit{Path: p, Bytes: []byte(body)})
	}
	reversed := make([]ingest.Unit, len(units))
	for i, u := range units {
		reversed[len(units)-1-i] = u
	}

	render := func(us []ingest.Unit) string {
		t.Helper()
		corpus, err := ingest.New(us)
		if err != nil {
			t.Fatalf("ingest.New: %v", err)
		}
		art, err := markdown.Survey(corpus, tokens.Estimator{}, log.Discard())
		if err != nil {
			t.Fatalf("Survey: %v", err)
		}
		var b bytes.Buffer
		if err := art.WriteJSON(&b); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
		return b.String()
	}

	first := render(units)
	if second := render(units); second != first {
		t.Error("two surveys of the same corpus produced different JSON")
	}
	if shuffled := render(reversed); shuffled != first {
		t.Error("input order changed the artifact")
	}
	if !strings.Contains(first, `"schema": "`+survey.SchemaVersion+`"`) {
		t.Error("the artifact must carry its schema version")
	}
	// Guards the coverage above: if the corpus stops carrying metadata
	// labels, this test silently stops exercising them.
	for _, want := range []string{`"title": "Front Matter Doc"`, `"tags"`, `"metadata"`} {
		if !strings.Contains(first, want) {
			t.Errorf("the determinism corpus must exercise metadata fields; %s is absent", want)
		}
	}
}
