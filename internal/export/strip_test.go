package export

import (
	"strings"
	"testing"

	"kbase/internal/kb"
)

// TestStripNavigation: the strip removes exactly the navigation forms — the
// frontmatter block, the up-link line and an index's child-list lines — and
// keeps every other line, including an index's prose that carries links.
func TestStripNavigation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		text    string
		keep    []string
		dropped []string
	}{
		{
			name: "index",
			path: "vol1/index.md",
			text: `[↑ A KB](../entry-point.md)

# First Volume

Prose carrying a link to the first leaf ([§1](v1-leaf.md)) and more prose.

## Derivations

- [§1 The Leaf](v1-leaf.md) — a leaf
- [§2 The Nested Index](nested/index.md)
`,
			keep:    []string{"# First Volume", "Prose carrying a link to the first leaf ([§1](v1-leaf.md)) and more prose.", "## Derivations"},
			dropped: []string{"[↑ A KB](../entry-point.md)", "[§1 The Leaf](v1-leaf.md)", "[§2 The Nested Index](nested/index.md)"},
		},
		{
			name: "entry point",
			path: "entry-point.md",
			text: `---
kb-format: "3.0.0"
---

# A KB

A blurb about the KB.

## Volumes

- [First](vol1/index.md) — the first volume
- [Second](vol2/index.md) — the second volume
`,
			keep:    []string{"# A KB", "A blurb about the KB.", "## Volumes"},
			dropped: []string{"kb-format", "[First](vol1/index.md)", "[Second](vol2/index.md)"},
		},
		{
			name: "leaf keeps a link list that is body",
			path: "vol1/v1-leaf.md",
			text: `[↑ First Volume](index.md)

# §1 The Leaf

Body with a [link](../vol2/v2-leaf.md).

- [A body link](../vol2/v2-leaf.md) — the author's own list
`,
			keep:    []string{"# §1 The Leaf", "Body with a [link](../vol2/v2-leaf.md).", "- [A body link](../vol2/v2-leaf.md) — the author's own list"},
			dropped: []string{"[↑ First Volume](index.md)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Strip(tc.path, tc.text)
			for _, want := range tc.keep {
				if !strings.Contains(got, want) {
					t.Errorf("stripped output lost %q\n--- got ---\n%s", want, got)
				}
			}
			for _, gone := range tc.dropped {
				if strings.Contains(got, gone) {
					t.Errorf("stripped output kept %q\n--- got ---\n%s", gone, got)
				}
			}
		})
	}
}

// TestStripFrontmatterIsTheHouseRule: the strip reads the frontmatter through
// kb's reader, so a document opening with a thematic break or an unclosed fence
// keeps its body rather than exporting empty.
func TestStripFrontmatterIsTheHouseRule(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		text string
		keep []string
	}{
		{
			name: "thematic break opener",
			path: "vol1/index.md",
			text: "---\n# V\n\nprose\n\n---\n\nmore prose\n",
			keep: []string{"# V", "prose", "more prose"},
		},
		{
			name: "unclosed fence",
			path: "entry-point.md",
			text: "---\nkb-format: \"3.0.0\"\n\n# T\n\nBody.\n",
			keep: []string{"kb-format: \"3.0.0\"", "# T", "Body."},
		},
		{
			name: "blank line after the fence",
			path: "entry-point.md",
			text: "---\nkb-format: \"3.0.0\"\n---\n\n# KB\n\nbody\n",
			keep: []string{"# KB", "body"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Strip(tc.path, tc.text)
			for _, want := range tc.keep {
				if !strings.Contains(got, want) {
					t.Errorf("stripped output lost %q\n--- got ---\n%q", want, got)
				}
			}
		})
	}
}

// TestStripUplinkAfterBlank: the up-link line is the body's first line carrying
// text, so a blank line above the up-link does not save the navigation link;
// prose in that position is kept.
func TestStripUplinkAfterBlank(t *testing.T) {
	atTop := Strip("vol1/leaf.md", "[↑ Vol](index.md)\n\nbody\n")
	if strings.Contains(atTop, "[↑ Vol](index.md)") {
		t.Errorf("the up-link survived on the first line:\n%q", atTop)
	}
	afterBlank := Strip("vol1/leaf.md", "\n[↑ Vol](index.md)\n\nbody\n")
	if strings.Contains(afterBlank, "[↑ Vol](index.md)") {
		t.Errorf("the up-link survived a blank line above it:\n%q", afterBlank)
	}
	afterTwoBlanks := Strip("vol1/leaf.md", "\n\n[↑ Vol](index.md)\n\nbody\n")
	if strings.Contains(afterTwoBlanks, "[↑ Vol](index.md)") {
		t.Errorf("the up-link survived two blank lines above it:\n%q", afterTwoBlanks)
	}
	prose := Strip("vol1/leaf.md", "\nSome prose without the marker.\n\nbody\n")
	if !strings.Contains(prose, "Some prose without the marker.") {
		t.Errorf("prose in the body's first line was dropped:\n%q", prose)
	}
	unmarked := Strip("vol1/leaf.md", "\n[Some link](index.md)\n\nbody\n")
	if !strings.Contains(unmarked, "[Some link](index.md)") {
		t.Errorf("a link line carrying no marker was dropped:\n%q", unmarked)
	}
}

// TestStripKeepsProseAroundLinks: an index's prose line that carries links and
// an authored prose section both survive the strip; only the child-list lines
// go.
func TestStripKeepsProseAroundLinks(t *testing.T) {
	text := `[↑ A KB](../entry-point.md)

## Key Results

The price wedge ([§2](p3-friction.md)) and the selector ([§4](p3-selection.md)) are distinct legs.

- [§2](p3-friction.md) — the wedge
- [§4](p3-selection.md) — the selector
`
	got := Strip("part3/index.md", text)
	if !strings.Contains(got, "The price wedge ([§2](p3-friction.md)) and the selector ([§4](p3-selection.md)) are distinct legs.") {
		t.Errorf("prose around links was stripped:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "- [§2]") || strings.HasPrefix(line, "- [§4]") {
			t.Errorf("child-list line survived: %q", line)
		}
	}
}

// TestStripAgreesWithChildList: the walk descends exactly the lines Strip drops
// over a body carrying a fenced child-list example and a real child list.
func TestStripAgreesWithChildList(t *testing.T) {
	text := "[↑ A KB](../entry-point.md)\n\n# First Volume\n\n" +
		"```\n- [Example](child-file.md) — a fenced example\n```\n\n" +
		"- [Real](real.md) — a real child\n- [Other](other/index.md) — a nested index\n"
	targets := kb.ChildList("vol1/index.md", text)
	if len(targets) != 2 {
		t.Fatalf("ChildList = %q; want the two real children", targets)
	}
	stripped := Strip("vol1/index.md", text)
	for _, target := range []string{"[Real](real.md)", "[Other](other/index.md)"} {
		if strings.Contains(stripped, target) {
			t.Errorf("%q was descended but kept in the body", target)
		}
	}
	if !strings.Contains(stripped, "[Example](child-file.md)") {
		t.Errorf("the fenced example was dropped from the body:\n%s", stripped)
	}
}
