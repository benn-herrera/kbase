package kb

import "testing"

// TestChildListEdges: the child-list rule takes a bulleted list item that is
// only .md links — each with an optional quoted title — and rejects prose
// before the links, a non-.md target, a non-item line, a wrapped item, and a
// bare sentence.
func TestChildListEdges(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want []string
	}{
		{"plain item", "- [A](a.md) — a leaf", []string{"a.md"}},
		{"bare item", "- [A](a.md)", []string{"a.md"}},
		{"two links", "* [A](a.md) [B](b.md)", []string{"a.md", "b.md"}},
		{"indented", "  - [A](a.md)", []string{"a.md"}},
		{"prose before link", "The wedge ([§2](p3-friction.md)) is distinct.", nil},
		{"non-md target", "- [A](a.tex)", nil},
		{"not a list item", "[A](a.md)", nil},
		{"no links", "- a bare item", nil},
		{"prose after links", "- [A](a.md) and then some prose", nil},
		{"wrapped item", "- [A Long Wrapped Child Title Goes Here\n  and finishes](v1-leaf.md) — a leaf", nil},
		{"indented code", "    - [A](a.md) — indented code", nil},
		{"tabbed into code", "\t- [A](a.md) — a tab reaches column four", nil},
		{"quoted title", `- [§1 The Leaf](v1-leaf.md "The leaf")`, []string{"v1-leaf.md"}},
		{"single-quoted title", "- [§1 The Leaf](v1-leaf.md 'The leaf')", []string{"v1-leaf.md"}},
		{"angled destination with title", `- [§1](<v1-leaf.md> "The leaf")`, []string{"v1-leaf.md"}},
		{"description carrying a link", "- [§2 The Wedge](p3-friction.md) — the wedge ([§4](p4-nested.md))", []string{"p3-friction.md"}},
		{"description then a link", "- [A](a.md) — see [B](b.md) for context", []string{"a.md"}},
		{"emphasis-wrapped link", "- **[§2 The Wedge](p3-friction.md)** — the wedge", []string{"p3-friction.md"}},
		{"emphasis-wrapped link, italic", "- *[§2 The Wedge](p3-friction.md)* — the wedge", []string{"p3-friction.md"}},
		{
			// Emphasis after the link with no separator is prose in the tail
			// position, exactly as un-emphasised prose there is; the wrapper
			// is relaxed before the link, not in a separator-less tail.
			"emphasis after the link, no separator", "- [§2 The Wedge](p3-friction.md) *the wedge*", nil,
		},
		{"image is not a child", "- ![fig](p3-friction.md) — a leaf", nil},
		{"second link carries a title", `- [A](a.md) [B](b.md "T")`, []string{"a.md", "b.md"}},
		{"negative-number description", "- [A](a.md) -1.5% growth", []string{"a.md"}},
		{"bare hyphen description", "- [A](a.md) - see below", []string{"a.md"}},
		{"inline maths in the title", "- [Extension $`\\#`$ 1](extension-1-uncertainty.md)", []string{"extension-1-uncertainty.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ChildList("dir/index.md", tc.line+"\n")
			if len(got) != len(tc.want) {
				t.Fatalf("ChildList = %q; want %q", got, tc.want)
			}
			for i := range got {
				want := "dir/" + tc.want[i]
				if got[i] != want {
					t.Errorf("ChildList[%d] = %q; want %q", i, got[i], want)
				}
			}
		})
	}
}

// TestChildListBlanksFences: a child-list-shaped line inside a fenced block is
// a code example, not the tree's. The shape is the one the corpus's own
// CONVENTIONS.md carries.
func TestChildListBlanksFences(t *testing.T) {
	text := "---\nkind: index\n---\n[↑ A KB](../entry-point.md)\n\n" +
		"# First Volume\n\nWrite child entries like this:\n\n" +
		"```\n- [Child title](child-file.md) — one-line description\n```\n\n" +
		"- [Real](real.md) — a real leaf\n"
	if got := ChildList("vol1/index.md", text); len(got) != 1 || got[0] != "vol1/real.md" {
		t.Errorf("ChildList = %q; want only the real leaf", got)
	}
}

// TestIsMarker: the write API's two in-body markers are markers whatever
// whitespace Python's \s admits around their content; any other comment is not.
func TestIsMarker(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"<!-- claim-rigor: clm-aa1111 -->", true},
		{"<!-- id: clm-aa1111 -->", true},
		{"  <!-- id: clm-aa1111 -->\n", true},
		{"<!-- claim-rigor: clm-aa1111 -->", true},
		{"<!--　id: clm-aa1111 -->", true},
		{"<!-- keep me -->", false},
		{"<!-- identity: x -->", false},
		{"text <!-- id: clm-aa1111 -->", false},
	} {
		if got := IsMarker(tc.text); got != tc.want {
			t.Errorf("IsMarker(%q) = %t; want %t", tc.text, got, tc.want)
		}
	}
}
