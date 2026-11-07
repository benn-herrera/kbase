package kb

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/formats/format40"
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

// TestTextCitations: a link whose title is a node id is a citation, with its
// 0-based line and offsets on the raw line; an ordinary link, a link titled
// with no id, and a citation in inline code or a fence are not.
func TestTextCitations(t *testing.T) {
	cite := `[cited](a.md#s "clm-aaaaaa")`
	line := `See [ordinary](a.md) and [titled](a.md "a title") then ` + cite + "."
	text := "# Doc\n\n" + line + "\nInline `" + cite + "` here.\n\n```\n" + cite + "\n```\n"
	start := strings.Index(line, cite)
	dest := start + len("[cited](")
	want := []TextCitation{{Line: 2, Link: format40.Link{Start: start, End: start + len(cite), DestStart: dest, DestEnd: dest + len("a.md#s"),
		Text: "cited", Destination: "a.md#s", Title: "clm-aaaaaa"}}}
	if got := TextCitations(text); !slices.Equal(got, want) {
		t.Errorf("TextCitations = %+v, want %+v", got, want)
	}
}

// TestTextLinkDefinitions: a reference definition, angle-wrapped or titled,
// is read with its destination's offsets on the raw line; a footnote
// definition, an inline link and a definition inside a fence are not.
func TestTextLinkDefinitions(t *testing.T) {
	text := "# Doc\n\n[a]: x.md#s\n   [b]: <y z.md> \"t\"\n[^1]: a note\nSee [c](x.md).\n\n```\n[d]: w.md\n```\n"
	var got []string
	for _, l := range TextLinkDefinitions(text) {
		got = append(got, fmt.Sprintf("%d %s", l.Line, SplitLines(text)[l.Line][l.DestStart:l.DestEnd]))
	}
	if want := []string{"2 x.md#s", "3 <y z.md>"}; !slices.Equal(got, want) {
		t.Errorf("TextLinkDefinitions = %q, want %q", got, want)
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

func TestStripCodeSpanPairing(t *testing.T) {
	sp := strings.Repeat
	for _, tc := range []struct{ name, in, want string }{
		{"single backticks", "a `x` b", "a " + sp(" ", 3) + " b"},
		{"a shorter run inside is content", "a `` c`d `` b", "a " + sp(" ", 9) + " b"},
		{"a longer run inside is content", "`` ```* `` then `\\bigl[b\\bigr](\\xi)`", sp(" ", 10) + " then " + sp(" ", 20)},
		{"an unclosed opener is literal", "it`s odd [a](b.md)", "it`s odd [a](b.md)"},
		{"a span closes on its own line", "`` x ``\n[a](b.md)", sp(" ", 7) + "\n[a](b.md)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripCode(tc.in); got != tc.want {
				t.Errorf("StripCode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
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
		{`[the bound](a.md#bounds "ana-aaaaaa") and ["it holds"](b.md 'a title')`, []string{"a.md#bounds", "b.md"}},
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

	dead, err := DeadLinks(OnDisk(root))
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

// TestDeadLinksInChecksARegisterPageFragment: a link into a register page
// holds where its fragment is one of the slugs derived for that page, and is
// broken fragment where it is not — a page no longer called for holding none —
// while a link with no fragment and a page whose file is gone are judged by
// the file check alone; a fragment into any other document holds where it is
// one of that document's anchors.
func TestDeadLinksInChecksARegisterPageFragment(t *testing.T) {
	root := t.TempDir()
	for rel, text := range map[string]string{
		"a/claim-rigor.md": "", "b/claim-rigor.md": "", "d/claim-rigor.md": "", "a/leaf.md": "# Leaf\n",
		"a/prose.md": strings.Join([]string{
			"[held](claim-rigor.md#theorem-1)",
			"[moved](claim-rigor.md#lemma-2)",
			"[encoded](<../b/claim-rigor.md#lemma%2D2>)",
			"[not on that page](../b/claim-rigor.md#theorem-1)",
			"[bare](claim-rigor.md)",
			"[other file](leaf.md#nowhere)",
			"[gone page](../c/claim-rigor.md#x)",
			"[uncalled page](../d/claim-rigor.md#theorem-1)",
			"[other file's heading](leaf.md#leaf)",
		}, "\n"),
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pages := map[string][]string{"a/claim-rigor.md": {"theorem-1"}, "b/claim-rigor.md": {"lemma-2"}, "c/claim-rigor.md": nil, "d/claim-rigor.md": nil}
	dead, err := DeadLinksIn(OnDisk(root), root, pages)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range dead {
		got = append(got, d.String())
	}
	want := []string{
		`a/prose.md:2 broken fragment "claim-rigor.md#lemma-2"`,
		`a/prose.md:4 broken fragment "../b/claim-rigor.md#theorem-1"`,
		`a/prose.md:6 broken fragment "leaf.md#nowhere"`,
		`a/prose.md:7 broken intra "../c/claim-rigor.md#x"`,
		`a/prose.md:8 broken fragment "../d/claim-rigor.md#theorem-1"`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("DeadLinksIn =\n%q\nwant\n%q", got, want)
	}
}
