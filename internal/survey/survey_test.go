package survey

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"kbase/internal/ingest"
	"kbase/internal/tokens"
)

// fixtures is the synthetic corpus every structural test draws on. It is one
// corpus rather than one per test so that link resolution has real siblings
// to resolve against, and so the tiling property runs over every shape at
// once — nested and skipped levels, setext, no headings, code fences,
// front matter, CRLF, unicode, and an empty file.
var fixtures = map[string]string{
	"nested.md": `# Guide

Intro paragraph.

### Deep skip

Body of the skipped level.

## Second

More text.
`,
	"setext.md": `Title
=====

Setext intro.

Subtitle
--------

Setext body.
`,
	"flat.md": `Just a paragraph, no headings anywhere.

And another one.
`,
	"empty.md": "",

	"unicode.md": `# 日本語のタイトル

段落テキスト。Ünicode ✓ and a tab	between words.
`,

	"crlf.md": "# CRLF Title\r\n\r\nBody line.\r\n\r\n## CRLF Sub\r\n\r\nMore body.\r\n",

	"fence.md": "# Real\n\n" +
		"```text\n# Not a heading\n## Also not\n```\n\n" +
		"## After the fence\n\nBody.\n",

	"nesting.md": `# Container

> ## Quoted heading
>
> Quoted body.

- ### Listed heading
- item

##

Titleless above.
`,

	"front.md": `---
title: Front Matter Doc
tags: [a, b]
---

# Actual Heading

Body.
`,
	"front-unterminated.md": `---
this block never closes

# Heading After
`,
	"guide/setup.md": `# Setup

Setup body.
`,
	"guide/links.md": `# Links

[sibling](./setup.md) [up](../nested.md) [rooted](/flat.md)
[extensionless](setup) [fragment](../nested.md#second) [anchor](#local)
[external](https://example.com/x) [mail](mailto:a@b.example)
[missing](./gone.md) [escaping](../../outside.md) [empty]()
![logo](../img/logo.png) <https://auto.example/>
`,
}

// surveyFixtures ingests the given files in memory and surveys them. Building
// the corpus through ingest.New rather than off disk keeps the structural
// tests free of the filesystem while still exercising the real path ids.
func surveyFixtures(t *testing.T, files map[string]string) (ingest.Corpus, Artifact) {
	t.Helper()
	units := make([]ingest.Unit, 0, len(files))
	for p, body := range files {
		units = append(units, ingest.Unit{Path: p, Bytes: []byte(body)})
	}
	corpus, err := ingest.New("/corpus", units)
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	art, err := Survey(corpus, tokens.Estimator{})
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	return corpus, art
}

// fileOf returns the surveyed file at path.
func fileOf(t *testing.T, art Artifact, path string) File {
	t.Helper()
	for _, f := range art.Files {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("no surveyed file at %q", path)
	return File{}
}

// outline renders a file's heading tree as indented "level title" lines — the
// compact form the table assertions compare against.
func outline(f File) []string {
	var out []string
	var walk func(secs []Section, depth int)
	walk = func(secs []Section, depth int) {
		for _, s := range secs {
			out = append(out, fmt.Sprintf("%s%d %s", strings.Repeat("  ", depth), s.Level, s.Title))
			walk(s.Children, depth+1)
		}
	}
	walk(f.Sections, 0)
	return out
}

// TestHeadingTree covers what becomes a section and what does not, across the
// heading shapes a real corpus contains.
func TestHeadingTree(t *testing.T) {
	_, art := surveyFixtures(t, fixtures)

	for _, tc := range []struct {
		name, path string
		want       []string
	}{{
		name: "skipped level nests as written",
		path: "nested.md",
		want: []string{"1 Guide", "  3 Deep skip", "  2 Second"},
	}, {
		name: "setext headings are headings",
		path: "setext.md",
		want: []string{"1 Title", "  2 Subtitle"},
	}, {
		name: "no headings at all",
		path: "flat.md",
		want: nil,
	}, {
		name: "empty file",
		path: "empty.md",
		want: nil,
	}, {
		// The characteristic false positive: `#` lines inside a fence are
		// content, and a survey that read them as headings would cut leaves
		// in the middle of code samples.
		name: "hashes inside a code fence are not headings",
		path: "fence.md",
		want: []string{"1 Real", "  2 After the fence"},
	}, {
		// Only document-level headings are boundaries, and a titleless
		// heading is not one either — see scan's documentation.
		name: "quoted, listed, and titleless headings are not boundaries",
		path: "nesting.md",
		want: []string{"1 Container"},
	}, {
		name: "unicode title survives intact",
		path: "unicode.md",
		want: []string{"1 日本語のタイトル"},
	}, {
		name: "CRLF line endings",
		path: "crlf.md",
		want: []string{"1 CRLF Title", "  2 CRLF Sub"},
	}, {
		name: "front matter does not fabricate a heading",
		path: "front.md",
		want: []string{"1 Actual Heading"},
	}, {
		name: "unterminated front matter is just content",
		path: "front-unterminated.md",
		want: []string{"1 Heading After"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := outline(fileOf(t, art, tc.path))
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("outline:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestTilingProperty is the property the whole offset discipline rests on:
// front matter, preamble, and every section's own body, concatenated in
// document order, reproduce the source byte for byte. It reconstructs the
// file from the artifact's offsets alone, so it fails on a gap, an overlap,
// or a misplaced boundary independently of verifyTiling's own arithmetic.
func TestTilingProperty(t *testing.T) {
	corpus, art := surveyFixtures(t, fixtures)
	for _, f := range art.Files {
		t.Run(f.Path, func(t *testing.T) {
			u, ok := corpus.Unit(f.Path)
			if !ok {
				t.Fatalf("no source unit for %q", f.Path)
			}
			var b bytes.Buffer
			if r := f.FrontMatter; r != nil {
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
func writeOwnBodies(b *bytes.Buffer, src []byte, s Section) {
	own := s.End
	if len(s.Children) > 0 {
		own = s.Children[0].Start
	}
	b.Write(src[s.Start:own])
	for _, c := range s.Children {
		writeOwnBodies(b, src, c)
	}
}

// TestSectionRangesCarryTheirHeading: a section starts at its heading LINE,
// not at the heading text, so a leaf sliced out by offset arrives titled.
func TestSectionRangesCarryTheirHeading(t *testing.T) {
	corpus, art := surveyFixtures(t, fixtures)
	u, _ := corpus.Unit("nested.md")
	f := fileOf(t, art, "nested.md")

	top := f.Sections[0]
	if got := string(u.Bytes[top.Start:top.End]); !strings.HasPrefix(got, "# Guide\n") {
		t.Errorf("section range starts %q, want it to open with its own heading line", got[:min(20, len(got))])
	}
	sub := top.Children[1]
	if got := string(u.Bytes[sub.Start:sub.End]); got != "## Second\n\nMore text.\n" {
		t.Errorf("subsection range = %q", got)
	}
}

// TestSectionTokensMatchTheirRange recomputes each section's estimate from
// the artifact's own offsets. It checks the OFFSETS, not the arithmetic: a
// section whose range drifted would report a token count for bytes it does
// not contain, and the taxonomy stage would budget a leaf against the wrong
// span.
func TestSectionTokensMatchTheirRange(t *testing.T) {
	corpus, art := surveyFixtures(t, fixtures)
	var est tokens.Estimator
	for _, f := range art.Files {
		u, _ := corpus.Unit(f.Path)
		if f.Tokens != est.Estimate(string(u.Bytes)) {
			t.Errorf("%s: file tokens %d disagree with its bytes", f.Path, f.Tokens)
		}
		var check func(secs []Section)
		check = func(secs []Section) {
			for _, s := range secs {
				if want := est.Estimate(string(u.Bytes[s.Start:s.End])); s.Tokens != want {
					t.Errorf("%s: section %q tokens = %d, want %d for [%d,%d)", f.Path, s.Title, s.Tokens, want, s.Start, s.End)
				}
				check(s.Children)
			}
		}
		check(f.Sections)
	}
}

// TestFrontMatter: the block is recorded as a byte range and excluded from
// the parse, and the bytes before the first heading remain accounted for.
func TestFrontMatter(t *testing.T) {
	corpus, art := surveyFixtures(t, fixtures)
	u, _ := corpus.Unit("front.md")
	f := fileOf(t, art, "front.md")

	if f.FrontMatter == nil {
		t.Fatal("front matter block was not detected")
	}
	got := string(u.Bytes[f.FrontMatter.Start:f.FrontMatter.End])
	want := "---\ntitle: Front Matter Doc\ntags: [a, b]\n---\n"
	if got != want {
		t.Errorf("front matter range holds %q, want %q", got, want)
	}
	if f.Preamble == nil || f.Preamble.Start != f.FrontMatter.End {
		t.Errorf("preamble must resume where the front matter ends; got %+v", f.Preamble)
	}
	if plain := fileOf(t, art, "front-unterminated.md"); plain.FrontMatter != nil {
		t.Errorf("an unterminated block is not front matter; got %+v", plain.FrontMatter)
	}
}

// TestFrontMatterWouldMisparse pins the reason the front-matter branch
// exists. Plain CommonMark turns a YAML header into a setext H2 titled with
// the whole block, because the closing `---` underlines the `key: value`
// paragraph above it. If a future goldmark stops doing this, the mechanical
// skip becomes dead weight and this test says so.
func TestFrontMatterWouldMisparse(t *testing.T) {
	src := []byte(fixtures["front.md"])
	doc := goldmark.DefaultParser().Parse(text.NewReader(src))

	var fabricated []string
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		if !ok || h.Lines().Len() == 0 {
			continue
		}
		if h.Lines().At(0).Start < len("---\ntitle: Front Matter Doc") {
			fabricated = append(fabricated, collapse(plainText(h, src)))
		}
	}
	if len(fabricated) == 0 {
		t.Error("goldmark no longer misparses YAML front matter — reconsider the mechanical skip")
	}
}

// TestPreamble: content before the first heading is a section in its own
// right, and a file without headings is all preamble.
func TestPreamble(t *testing.T) {
	corpus, art := surveyFixtures(t, fixtures)

	flat := fileOf(t, art, "flat.md")
	u, _ := corpus.Unit("flat.md")
	if flat.Preamble == nil {
		t.Fatal("a headingless file must still report its content")
	}
	if flat.Preamble.Start != 0 || flat.Preamble.End != len(u.Bytes) {
		t.Errorf("preamble = [%d,%d), want the whole file [0,%d)", flat.Preamble.Start, flat.Preamble.End, len(u.Bytes))
	}
	if flat.Preamble.Level != 0 {
		t.Errorf("preamble level = %d, want 0 (headingless)", flat.Preamble.Level)
	}
	if nested := fileOf(t, art, "nested.md"); nested.Preamble != nil {
		t.Errorf("a file opening with a heading has no preamble; got %+v", nested.Preamble)
	}
	if empty := fileOf(t, art, "empty.md"); empty.Preamble != nil || empty.Sections != nil {
		t.Errorf("empty file must produce no ranges; got %+v", empty)
	}
}

// TestGists: a gist is the leading paragraph of the thing it describes — the
// file's, or the section's own body, never a child's.
func TestGists(t *testing.T) {
	_, art := surveyFixtures(t, fixtures)

	f := fileOf(t, art, "nested.md")
	if want := "Intro paragraph."; f.Gist != want {
		t.Errorf("file gist = %q, want %q", f.Gist, want)
	}
	if want := "Intro paragraph."; f.Sections[0].Gist != want {
		t.Errorf("section gist = %q, want %q", f.Sections[0].Gist, want)
	}
	if want := "Body of the skipped level."; f.Sections[0].Children[0].Gist != want {
		t.Errorf("child gist = %q, want %q", f.Sections[0].Children[0].Gist, want)
	}

	// An index-only heading borrows nothing: setext.md's H1 owns a
	// paragraph, but a heading followed straight by a subheading must not
	// adopt the subheading's text.
	index := surveyOne(t, "idx.md", "# Index\n## Child\n\nChild text.\n")
	if got := index.Sections[0].Gist; got != "" {
		t.Errorf("index-only section gist = %q, want empty", got)
	}
	if got := index.Sections[0].Children[0].Gist; got != "Child text." {
		t.Errorf("child gist = %q, want %q", got, "Child text.")
	}
}

// TestGistWordCap: a long paragraph is cut at a word boundary and marked, so
// a routing hint never reads as a complete sentence it is not.
func TestGistWordCap(t *testing.T) {
	words := make([]string, 0, GistWordCap*2)
	for i := range GistWordCap * 2 {
		words = append(words, fmt.Sprintf("word%d", i))
	}
	long := strings.Join(words, " ")
	f := surveyOne(t, "long.md", "# Long\n\n"+long+"\n")

	got := f.Sections[0].Gist
	if !strings.HasSuffix(got, gistEllipsis) {
		t.Errorf("capped gist %q must be marked as cut", got)
	}
	trimmed := strings.TrimSuffix(got, gistEllipsis)
	if n := len(strings.Fields(trimmed)); n != GistWordCap {
		t.Errorf("capped gist holds %d words, want %d", n, GistWordCap)
	}
	if !strings.HasSuffix(trimmed, fmt.Sprintf("word%d", GistWordCap-1)) {
		t.Errorf("gist %q was cut mid-word", got)
	}
}

// TestGistFlattensInlineMarkup: a gist is plain text — emphasis, code spans,
// link labels, and wrapped lines all render as the words a reader sees.
func TestGistFlattensInlineMarkup(t *testing.T) {
	f := surveyOne(t, "inline.md", "# Inline\n\nUse **bold**, `code`, and [a link](x.md)\nwrapped across lines.\n")
	if want := "Use bold, code, and a link wrapped across lines."; f.Sections[0].Gist != want {
		t.Errorf("gist = %q, want %q", f.Sections[0].Gist, want)
	}
}

// TestDeterminism: the artifact is the pipeline's reproducible input to the
// taxonomy stage, so the same corpus must serialize to identical bytes —
// including when the units arrive in a different order — and must contain no
// machine-local path.
func TestDeterminism(t *testing.T) {
	const root = "/somewhere/local/corpus"
	units := []ingest.Unit{}
	for p, body := range fixtures {
		units = append(units, ingest.Unit{Path: p, Bytes: []byte(body)})
	}
	reversed := make([]ingest.Unit, len(units))
	for i, u := range units {
		reversed[len(units)-1-i] = u
	}

	render := func(us []ingest.Unit) string {
		t.Helper()
		corpus, err := ingest.New(root, us)
		if err != nil {
			t.Fatalf("ingest.New: %v", err)
		}
		art, err := Survey(corpus, tokens.Estimator{})
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
	if strings.Contains(first, root) {
		t.Error("the artifact leaked the corpus root path")
	}
	if !strings.Contains(first, `"schema": "`+SchemaVersion+`"`) {
		t.Error("the artifact must carry its schema version")
	}
}

// TestCorpusTotals: the roll-up is the sum of what the per-file entries say.
func TestCorpusTotals(t *testing.T) {
	corpus, art := surveyFixtures(t, fixtures)

	if art.Corpus.ContentHash != corpus.ContentHash {
		t.Error("the artifact must carry the corpus content hash as its provenance")
	}
	if art.Corpus.Files != len(fixtures) || len(art.Files) != len(fixtures) {
		t.Errorf("files = %d/%d, want %d", art.Corpus.Files, len(art.Files), len(fixtures))
	}
	var bytesTotal, tokensTotal, sections int
	for _, f := range art.Files {
		bytesTotal += f.Bytes
		tokensTotal += f.Tokens
		if f.Preamble != nil {
			sections++
		}
		sections += countSections(f.Sections)
	}
	if art.Corpus.Bytes != bytesTotal || art.Corpus.Tokens != tokensTotal || art.Corpus.Sections != sections {
		t.Errorf("totals %+v disagree with the per-file entries (bytes %d tokens %d sections %d)",
			art.Corpus, bytesTotal, tokensTotal, sections)
	}
}

// surveyOne surveys a single-document corpus — the helper for cases that need
// their own source rather than a shared fixture.
func surveyOne(t *testing.T, path, body string) File {
	t.Helper()
	_, art := surveyFixtures(t, map[string]string{path: body})
	return fileOf(t, art, path)
}
