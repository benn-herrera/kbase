package markdown

import (
	"strings"
	"testing"

	"kbase/internal/survey"
)

// cutsOf surveys one document and returns its candidates. It goes through the
// shared surveyOne helper, so survey.Assemble's neutral checks run over every
// list these cases produce — a candidate the artifact would refuse is a
// failure here as well as there.
func cutsOf(t *testing.T, body string) []survey.CutCandidate {
	t.Helper()
	return surveyOne(t, "doc.md", body).Cuts
}

// fenceDoc carries every shape the enumeration has to get right in one
// document: front matter (so every offset is rebased), headings, top-level
// paragraphs, a fenced block whose interior looks like Markdown, a fence
// inside a list item, and a heading inside a blockquote.
const fenceDoc = "---\ntitle: Cuts\n---\n\n" +
	"# Guide\n\nIntro paragraph.\n\n" +
	"```lua\n-- # not a heading\nlocal x = 1\n\nstill code\n```\n\n" +
	"After the fence.\n\n" +
	"- item\n\n  ```lua\n  nested = true\n  ```\n\n" +
	"> # quoted heading\n>\n> quoted body\n\n" +
	"## Second\n\nTail.\n"

// TestCutsAreLineStartsAndWhitespaceAdjacent is the property the neutral
// tripwire rests on: the adapter never produces an offset with printable
// bytes on both sides. Assemble enforces it too; asserting it here says WHY
// it holds — every candidate is the first byte of a line.
func TestCutsAreLineStartsAndWhitespaceAdjacent(t *testing.T) {
	src := []byte(fenceDoc)
	cuts := cutsOf(t, fenceDoc)
	if len(cuts) == 0 {
		t.Fatal("the document has block boundaries; the adapter enumerated none")
	}
	for _, c := range cuts {
		if !survey.WhitespaceAdjacent(src, c.Offset) {
			t.Errorf("candidate %+v is not whitespace-adjacent: %q", c, src[c.Offset-1:c.Offset+1])
		}
		if src[c.Offset-1] != '\n' {
			t.Errorf("candidate %+v is not the first byte of a line", c)
		}
		if !c.Kind.Known() {
			t.Errorf("candidate %+v has a kind the artifact does not know", c)
		}
	}
}

// TestCutsExcludeFenceInteriors: a fence is enumerated by its edges, and the
// bytes between them are not cut positions however much they look like
// Markdown. This is the check that makes "the model cannot bisect a code
// sample" a fact about the candidate set rather than an intention.
func TestCutsExcludeFenceInteriors(t *testing.T) {
	src := fenceDoc
	cuts := cutsOf(t, src)

	open := strings.Index(src, "```lua")
	closeLine := strings.Index(src, "```\n\nAfter")
	after := closeLine + len("```\n")
	if open < 0 || closeLine < 0 {
		t.Fatalf("the fixture no longer holds the fenced block it is testing")
	}
	var sawOpen, sawAfter bool
	for _, c := range cuts {
		switch {
		case c.Offset == open:
			sawOpen = true
		case c.Offset == after:
			sawAfter = true
		case c.Offset > open && c.Offset < after:
			t.Errorf("candidate %+v is inside the fenced block [%d,%d): %q",
				c, open, after, src[c.Offset:min(c.Offset+16, len(src))])
		}
	}
	if !sawOpen || !sawAfter {
		t.Errorf("the fence's edges must be candidates; open=%v after=%v", sawOpen, sawAfter)
	}
	// The nested fence's interior is not a candidate either — it is inside a
	// list item, so the block contributes nothing at all.
	nested := strings.Index(src, "  nested = true")
	for _, c := range cuts {
		if c.Offset > nested-len("  ```lua\n") && c.Offset <= nested {
			t.Errorf("candidate %+v cuts into a list item's fenced block", c)
		}
	}
}

// TestCutsCoverTheStructuralKinds: heading lines, fence edges and top-level
// paragraph starts are all enumerated, and nested blocks are not. A splitter
// that prefers headings needs them to be labelled as such.
func TestCutsCoverTheStructuralKinds(t *testing.T) {
	src := fenceDoc
	byOffset := map[int]survey.CutKind{}
	kinds := map[survey.CutKind]int{}
	for _, c := range cutsOf(t, src) {
		byOffset[c.Offset] = c.Kind
		kinds[c.Kind]++
	}
	for _, tc := range []struct {
		what string
		at   string
		want survey.CutKind
	}{
		{"a heading line", "# Guide", survey.CutHeading},
		{"a later heading", "## Second", survey.CutHeading},
		{"a fence's opening line", "```lua\n-- #", survey.CutFence},
		{"a top-level paragraph", "After the fence.", survey.CutParagraph},
	} {
		at := strings.Index(src, tc.at)
		if at < 0 {
			t.Fatalf("the fixture no longer holds %s", tc.what)
		}
		if got, ok := byOffset[at]; !ok || got != tc.want {
			t.Errorf("%s at %d is %q (present=%v), want %q", tc.what, at, got, ok, tc.want)
		}
	}
	// A heading inside a blockquote is that quote's content, not a boundary.
	if at := strings.Index(src, "> # quoted heading"); byOffset[at+2] != "" {
		t.Errorf("a heading inside a blockquote must not be a candidate; got %q", byOffset[at+2])
	}
	for _, k := range []survey.CutKind{survey.CutHeading, survey.CutFence, survey.CutParagraph} {
		if kinds[k] == 0 {
			t.Errorf("the fixture must exercise %q candidates; it produced none", k)
		}
	}
}

// TestCutsAreSortedUniqueAndInterior: the artifact's ordering promise, and
// the collapse rule that goes with it — a fence's closing edge and the
// paragraph that follows it can be the same byte, and the survivor is the
// stronger kind.
func TestCutsAreSortedUniqueAndInterior(t *testing.T) {
	// No blank line after the fence, so its closing edge IS the paragraph's
	// first byte.
	const doc = "# T\n\n```\ncode\n```\nStraight into prose.\n"
	cuts := cutsOf(t, doc)

	prev := -1
	for _, c := range cuts {
		if c.Offset <= prev {
			t.Fatalf("candidates are not strictly ascending: %+v after %d", c, prev)
		}
		if c.Offset <= 0 || c.Offset >= len(doc) {
			t.Errorf("candidate %+v is not interior to the file", c)
		}
		prev = c.Offset
	}
	at := strings.Index(doc, "Straight into prose.")
	found := survey.CutKind("")
	for _, c := range cuts {
		if c.Offset == at {
			found = c.Kind
		}
	}
	if found != survey.CutFence {
		t.Errorf("the shared offset is %q, want the stronger %q", found, survey.CutFence)
	}
}

// TestCutsRebaseOverFrontMatter: every artifact offset is measured in the
// original file, and the parser sees a metadata-stripped view of it. A
// candidate that skipped the rebase would still look plausible — it would
// just point at the wrong line — so the check is that it addresses the same
// text in the source.
//
// It scans for the heading candidate NEAREST the real offset and asserts what
// that one addresses, rather than filtering to candidates already AT it: the
// filtered form could only ever fail by finding nothing, which catches a
// rebase dropped entirely and not one that is off by a few bytes.
func TestCutsRebaseOverFrontMatter(t *testing.T) {
	at := strings.Index(fenceDoc, "# Guide")
	nearest, best := -1, 0
	for _, c := range cutsOf(t, fenceDoc) {
		if c.Kind != survey.CutHeading {
			continue
		}
		if d := max(c.Offset-at, at-c.Offset); nearest < 0 || d < best {
			nearest, best = c.Offset, d
		}
	}
	if nearest < 0 {
		t.Fatalf("no heading candidate at all; the first heading is at offset %d", at)
	}
	if !strings.HasPrefix(fenceDoc[nearest:], "# Guide") {
		t.Fatalf("the heading candidate nearest offset %d is at %d, which addresses %q — the offsets were "+
			"not rebased over the front matter", at, nearest, firstLine(fenceDoc[nearest:]))
	}
}

// firstLine is a candidate's own line, for a failure message that shows what
// the offset actually points at.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// TestCutsOnADocumentWithNoBlocks: an empty or single-block document has no
// interior boundary, and the empty list is the honest answer rather than a
// candidate at one of the file's own ends.
func TestCutsOnADocumentWithNoBlocks(t *testing.T) {
	for _, doc := range []string{"", "Just one paragraph.\n", "# Only a heading\n"} {
		if cuts := cutsOf(t, doc); len(cuts) != 0 {
			t.Errorf("%q produced %+v, want no candidates", doc, cuts)
		}
	}
}
