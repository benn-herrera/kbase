package kb

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRebaseInlineLinks(t *testing.T) {
	for _, tc := range []struct {
		name, text, from, to, want string
	}{
		{"up a level, anchor kept", "see [x](../b/c.md#s)", "v/sub", "v", "see [x](b/c.md#s)"},
		{"down a level", "[x](c.md)", "v", "v/sub", "[x](../c.md)"},
		{"same directory", "[x](c.md)", "v", "v/", "[x](c.md)"},
		{"root to volume", "[x](v/c.md)", "", "v", "[x](c.md)"},
		{"angle destination", "[x](<a b.md>)", "v/sub", "v", "[x](<sub/a b.md>)"},
		{"linked image moves both", "[![](i.png)](i.png)", "v/sub", "v", "[![](sub/i.png)](sub/i.png)"},
		{"urls, rooted paths and anchors stay", "[a](https://x.org) [b](/r.md) [c](#f) [d](~/h.md)", "v/sub", "v", "[a](https://x.org) [b](/r.md) [c](#f) [d](~/h.md)"},
		{"code span is not a link", "`[x](c.md)` [y](c.md)", "v/sub", "v", "`[x](c.md)` [y](sub/c.md)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RebaseInlineLinks(tc.text, tc.from, tc.to); got != tc.want {
				t.Errorf("RebaseInlineLinks = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBlankFencedLines(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"plain fence", "a\n```\n[x](y)\n```\nb", []string{"a", "", "", "", "b"}},
		{"quoted fence closing on an emphasis tail", "> ``` math\n> [T](z)\n> ```*\n[l](m)", []string{"", "", "", "[l](m)"}},
		{"shorter run does not close", "````\n```\n[x](y)\n````\nb", []string{"", "", "", "", "b"}},
		{"other character does not close", "```\n~~~\n```\nb", []string{"", "", "", "b"}},
		{"info string does not close", "```\n```python\n```\nb", []string{"", "", "", "b"}},
		{"quoted fence closes with its blockquote", "> ```\n> x\nafter", []string{"", "", "after"}},
		{"fence in a list item", "1.  item\n\n    ```\n    [x](y)\n    ```\n[l](m)", []string{"1.  item", "", "", "", "", "[l](m)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := BlankFencedLines(tc.text); !slices.Equal(got, tc.want) {
				t.Errorf("BlankFencedLines = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStripCode(t *testing.T) {
	text := "see `[a](b)` and $`\\bigl[a\\bigr](\n\\xi)`$ then [c](d)"
	got := StripCode(text)
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("line count changed: %q", got)
	}
	var targets []string
	for _, m := range LinkRE.FindAllStringSubmatch(got, -1) {
		targets = append(targets, m[1])
	}
	if !slices.Equal(targets, []string{"d"}) {
		t.Errorf("links left after StripCode = %q, want only d (from %q)", targets, got)
	}
}

func TestStripTarget(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"<a file.md>", "a file.md"},
		{"x.md#frag", "x.md"},
		{"path/file.md:42", "path/file.md"},
		{"a%20file.md", "a file.md"},
		{"a%23b.md", "a#b.md"},
		{"#only", ""},
	} {
		if got := StripTarget(tc.in); got != tc.want {
			t.Errorf("StripTarget(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLinkPatterns(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{"[x](a.md)", []string{"a.md"}},
		{`["the rule [see note] applies"](t.md)`, []string{"t.md"}},
		{"[x](<a b.md>)", []string{"<a b.md>"}},
		{"[![](img.png)](img.png)", []string{"img.png"}},
		{"[label]: ref.md", []string{"ref.md"}},
		{`[label]: ref.md "title"`, []string{"ref.md"}},
		{"[^1]: a footnote", nil},
		{"[architect]: which minimal stage subset builds first.", nil},
	} {
		if got := rawTargets(tc.line); !slices.Equal(got, tc.want) {
			t.Errorf("rawTargets(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestDeadLinks(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("vol/index.md", strings.Join([]string{
		"[↑ up](../entry-point.md)",
		"[ok](leaf.md#frag)",
		"[case](Leaf.md)",
		"[gone](missing.md)",
		"[out](../../outside.md)",
		"`[code](nowhere.md)`",
		"[web](https://example.org/x.md) [tex](paper.tex) [anchor](#here) [home](~/x.md)",
	}, "\n"))
	write("vol/leaf.md", "[↑ v](index.md)")
	write("entry-point.md", "- [v](vol/index.md)")
	write(".index/skipped.md", "[x](nowhere.md)")
	write("vol/x.tmpl.md", "[x](nowhere.md)")

	dead, err := DeadLinks(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range dead {
		got = append(got, d.String())
	}
	want := []string{
		`vol/index.md:3 broken intra "Leaf.md"`,
		`vol/index.md:4 broken intra "missing.md"`,
		`vol/index.md:5 broken inter "../../outside.md"`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("DeadLinks =\n%q\nwant\n%q", got, want)
	}
}
