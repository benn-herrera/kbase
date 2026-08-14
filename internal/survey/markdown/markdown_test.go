package markdown

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmtext "github.com/yuin/goldmark/text"

	"kbase/internal/ingest"
	"kbase/internal/log/logtest"
	"kbase/internal/survey"
	"kbase/internal/text"
	"kbase/internal/tokens"
)

// fixtures is the synthetic corpus every structural test draws on. It is one
// corpus rather than one per test so that link resolution has real siblings
// to resolve against, and so every shape runs through survey.Assemble's
// tiling check at once — nested and skipped levels, setext, no headings, code
// fences, front matter, CRLF, unicode, and an empty file.
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
description: What this document is for
tags: [a, b]
---

# Actual Heading

Body.
`,
	"front-unterminated.md": `---
this block never closes

# Heading After
`,
	// A document that opens with a thematic break and uses `---` as a
	// section rule: delimited exactly like front matter, and content.
	"front-thematic.md": `---

An opening rule, then prose a delimiter-only scan would swallow whole.

---

# Survives The Scan

Body.
`,
	// A leading blank line means the delimiter is not the first line, so
	// there is no block — CommonMark reads what follows as a setext heading,
	// and the survey reports the document it was given.
	"front-blank-first.md": "\n---\ntitle: Not Front Matter\n---\n\nBody.\n",

	// A BOM ahead of the delimiter is the misparse this branch exists to
	// prevent, wearing a hat: without skipping it, line 1 is not a thematic
	// break and the whole YAML block becomes a setext H2 title.
	"front-bom.md": "\uFEFF---\ntitle: BOM Doc\n---\n\n# After The BOM\n\nBody.\n",
	"plain-bom.md": "\uFEFF# Heading Under A BOM\n\nBody.\n",
	"front-empty.md": `---
---

# Empty Block

Body.
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
func surveyFixtures(t *testing.T, files map[string]string) (ingest.Corpus, survey.Artifact) {
	t.Helper()
	corpus, art, _ := surveyFixturesLogged(t, files)
	return corpus, art
}

// surveyFixturesLogged is surveyFixtures for the cases that assert on what
// the survey recorded as well as on what it produced.
func surveyFixturesLogged(t *testing.T, files map[string]string) (ingest.Corpus, survey.Artifact, *logtest.Capture) {
	t.Helper()
	units := make([]ingest.SourceDoc, 0, len(files))
	for p, body := range files {
		units = append(units, ingest.SourceDoc{Path: p, Bytes: []byte(body)})
	}
	corpus, err := ingest.New(units)
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	lg := &logtest.Capture{}
	art, err := Survey(corpus, tokens.Estimator{}, lg)
	if err != nil {
		t.Fatalf("Survey: %v", err)
	}
	return corpus, art, lg
}

// fileOf returns the surveyed file at path.
func fileOf(t *testing.T, art survey.Artifact, path string) survey.File {
	t.Helper()
	for _, f := range art.Files {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("no surveyed file at %q", path)
	return survey.File{}
}

// outline renders a file's heading tree as indented "level title" lines — the
// compact form the table assertions compare against.
func outline(f survey.File) []string {
	var out []string
	var walk func(secs []survey.Section, depth int)
	walk = func(secs []survey.Section, depth int) {
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
	}, {
		// The false positive the mapping gate kills: delimited like front
		// matter, but its contents are prose, so the heading BELOW the
		// second rule still has to be a section.
		name: "a delimited prose block is content, not front matter",
		path: "front-thematic.md",
		want: []string{"1 Survives The Scan"},
	}, {
		name: "front matter behind a BOM is still detected",
		path: "front-bom.md",
		want: []string{"1 After The BOM"},
	}, {
		name: "an empty block is front matter carrying nothing",
		path: "front-empty.md",
		want: []string{"1 Empty Block"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := outline(fileOf(t, art, tc.path))
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("outline:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestSectionRangesCarryTheirHeading: a section starts at its heading LINE,
// not at the heading text, so a leaf sliced out by offset arrives titled.
func TestSectionRangesCarryTheirHeading(t *testing.T) {
	corpus, art := surveyFixtures(t, fixtures)
	u, _ := corpus.Doc("nested.md")
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

// TestPreamble: content before the first heading is a section in its own
// right, and a file without headings is all preamble.
func TestPreamble(t *testing.T) {
	corpus, art := surveyFixtures(t, fixtures)

	flat := fileOf(t, art, "flat.md")
	u, _ := corpus.Doc("flat.md")
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
// a routing hint never reads as a complete sentence it is not. The rule
// itself is text.CapWords at the artifact's cap; this checks the adapter puts
// gists through it.
func TestGistWordCap(t *testing.T) {
	f := surveyOne(t, "long.md", "# Long\n\n"+manyWords()+"\n")

	got := f.Sections[0].Gist
	if !strings.HasSuffix(got, text.CapEllipsis) {
		t.Errorf("capped gist %q must be marked as cut", got)
	}
	trimmed := strings.TrimSuffix(got, text.CapEllipsis)
	if n := len(strings.Fields(trimmed)); n != survey.WordCap {
		t.Errorf("capped gist holds %d words, want %d", n, survey.WordCap)
	}
	if !strings.HasSuffix(trimmed, fmt.Sprintf("word%d", survey.WordCap-1)) {
		t.Errorf("gist %q was cut mid-word", got)
	}
}

// TestTitleWordCap: a title is capped by the same rule a gist is. The case
// that makes this non-theoretical is setext — a setext heading's title is the
// ENTIRE paragraph above the underline, so a long opening paragraph would
// otherwise buy a title of the same length in an artifact whose whole design
// constraint is compactness.
func TestTitleWordCap(t *testing.T) {
	f := surveyOne(t, "setext-long.md", manyWords()+"\n---\n\nBody.\n")

	if len(f.Sections) != 1 {
		t.Fatalf("sections = %+v, want the one setext heading", f.Sections)
	}
	title := f.Sections[0].Title
	if !strings.HasSuffix(title, text.CapEllipsis) {
		t.Errorf("capped title %q must be marked as cut", title)
	}
	if n := len(strings.Fields(strings.TrimSuffix(title, text.CapEllipsis))); n != survey.WordCap {
		t.Errorf("capped title holds %d words, want %d", n, survey.WordCap)
	}
}

// manyWords is a paragraph twice as long as the cap, with each word carrying
// its own index so a test can say where the cut landed.
func manyWords() string {
	words := make([]string, 0, survey.WordCap*2)
	for i := range survey.WordCap * 2 {
		words = append(words, fmt.Sprintf("word%d", i))
	}
	return strings.Join(words, " ")
}

// TestGistFlattensInlineMarkup: a gist is plain text — emphasis, code spans,
// link labels, and wrapped lines all render as the words a reader sees.
func TestGistFlattensInlineMarkup(t *testing.T) {
	f := surveyOne(t, "inline.md", "# Inline\n\nUse **bold**, `code`, and [a link](x.md)\nwrapped across lines.\n")
	if want := "Use bold, code, and a link wrapped across lines."; f.Sections[0].Gist != want {
		t.Errorf("gist = %q, want %q", f.Sections[0].Gist, want)
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
		u, _ := corpus.Doc(f.Path)
		if f.Tokens != est.Estimate(string(u.Bytes)) {
			t.Errorf("%s: file tokens %d disagree with its bytes", f.Path, f.Tokens)
		}
		var check func(secs []survey.Section)
		check = func(secs []survey.Section) {
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

// surveyOne surveys a single-document corpus — the helper for cases that need
// their own source rather than a shared fixture.
func surveyOne(t *testing.T, path, body string) survey.File {
	t.Helper()
	_, art := surveyFixtures(t, map[string]string{path: body})
	return fileOf(t, art, path)
}

// TestExtensions: the adapter owns what counts as a Markdown document, and
// hands ingest a set it cannot rewrite.
func TestExtensions(t *testing.T) {
	got := Extensions()
	if len(got) != 1 || got[0] != Ext {
		t.Fatalf("Extensions() = %v, want [%s]", got, Ext)
	}
	got[0] = ".tex"
	if again := Extensions(); again[0] != Ext {
		t.Errorf("Extensions() returned a shared slice: a caller's edit changed it to %v", again)
	}
}

// TestFrontMatterWouldMisparse pins the reason the front-matter branch
// exists. Plain CommonMark turns a YAML header into a setext H2 titled with
// the whole block, because the closing `---` underlines the `key: value`
// paragraph above it. If a future goldmark stops doing this, the mechanical
// skip becomes dead weight and this test says so.
func TestFrontMatterWouldMisparse(t *testing.T) {
	src := []byte(fixtures["front.md"])
	doc := goldmark.DefaultParser().Parse(gmtext.NewReader(src))

	var fabricated []string
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		if !ok || h.Lines().Len() == 0 {
			continue
		}
		if h.Lines().At(0).Start < len("---\ntitle: Front Matter Doc") {
			fabricated = append(fabricated, text.CapWords(plainText(h, src), survey.WordCap))
		}
	}
	if len(fabricated) == 0 {
		t.Error("goldmark no longer misparses YAML front matter — reconsider the mechanical skip")
	}
}
