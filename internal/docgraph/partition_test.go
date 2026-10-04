package docgraph

import (
	"errors"
	"slices"
	"testing"

	"kbase/internal/records"
)

func TestMarkdownTokens(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"indented code is verbatim", "para\n\n    a_b c\n\ntext", []string{"para", "a_b", "c", "text"}},
		{"a list item's second paragraph is prose", "1.  item\n\n    second_par", []string{"1", "item", "secondpar"}},
		{"code inside a list item", "1.  item\n\n        a_b", []string{"1", "item", "a_b"}},
		{"a fence is verbatim, its markers carry nothing", "x\n``` math\nx_1 < y\n```\n*z*", []string{"x", "x_1", "<", "y", "z"}},
		{"a quoted fence", "> ``` math\n> a_b\n> ```", []string{"a_b"}},
		{"markup undone outside verbatim", "*rate*-limited <span>w</span> R&amp;D [t](u.md)", []string{"rate-limited", "w", "R&D", "t"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := markdownTokens(tc.text); !slices.Equal(got, tc.want) {
				t.Errorf("markdownTokens = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFencedBlocks(t *testing.T) {
	got := fencedBlocks("> ``` math\n> a\n>  b\n> ```*\n\n1.  item\n\n    ``` math\n    x\n      y\n    ```\n~~~\nt\n~~~")
	want := []fenced{{"math", "a\n b"}, {"math", "x\n  y"}, {"", "t"}}
	if !slices.Equal(got, want) {
		t.Errorf("fencedBlocks = %q, want %q", got, want)
	}
}

func TestHolds(t *testing.T) {
	rendered := []string{"a", "b", "c", "a", "d"}
	for _, tc := range []struct {
		run  []string
		want bool
	}{
		{nil, true},
		{[]string{"a", "d"}, true},
		{[]string{"b", "c", "a"}, true},
		{[]string{"a", "c"}, false},
		{[]string{"d", "e"}, false},
	} {
		if got := holds(rendered, tc.run); got != tc.want {
			t.Errorf("holds(%q) = %v, want %v", tc.run, got, tc.want)
		}
	}
}

func TestMathIn(t *testing.T) {
	body := "> text $`x_1`$ and\n> ``` math\n> \\det J > 0\n> ```\n<figcaption><math><annotation encoding=\"application/x-tex\">1&lt;t&#39;</annotation></math></figcaption>"
	got := mathIn(body)
	want := []string{"x_1", "\\det J > 0", "1<t'"}
	if !slices.Equal(got, want) {
		t.Errorf("mathIn = %q, want %q", got, want)
	}
}

// TestQuotedLabelLines: a bold-only paragraph at a one-level quote's margin
// is a block to kb_tools' reader; it hosts what follows it in its quote only
// where no readable block encloses it, and nowhere deeper is it read.
func TestQuotedLabelLines(t *testing.T) {
	str := func(s string) node { return node{"t": typeStr, "c": s} }
	space := node{"t": typeSpace}
	bold := func(words ...string) node {
		var c []any
		for i, w := range words {
			if i > 0 {
				c = append(c, space)
			}
			c = append(c, str(w))
		}
		return node{"t": typePara, "c": []any{node{"t": typeStrong, "c": c}}}
	}
	ref := node{"t": typePara, "c": []any{node{"t": typeLink, "c": []any{
		newAttr("", nil, [][2]string{{"reference-type", "ref"}, {"reference", "x"}}), []any{str("1")}, []any{"#x", ""}}}}}
	div := func(class string, content ...any) node {
		return node{"t": typeDiv, "c": []any{newAttr("", []string{class}, nil), content}}
	}
	quote := func(content ...any) node { return node{"t": typeBlockQuote, "c": content} }
	bare := func(p node) node {
		return node{"t": typePara, "c": []any{node{"t": typeSpan, "c": []any{newAttr("", nil, nil), list(p["c"])}}}}
	}
	doc := node{"meta": node{}, "blocks": []any{
		quote(bold("Step", "One"), ref),
		div("mdframed", bare(bold("Opening", "theme")), ref),
		quote(quote(bold("Too", "Deep"))),
		quote(bold("Not", "2")),
	}}
	_, drafts, err := readVolume(doc, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	type got struct {
		kind, name string
		within     int
	}
	var all []got
	for _, r := range drafts {
		all = append(all, got{r.Kind, r.Name, r.Within})
	}
	want := []got{
		{records.KindBlock, "Step One", 0}, {records.KindReference, "", 1},
		{records.KindBlock, "mdframed", 0}, {records.KindBlock, "Opening theme", 3}, {records.KindReference, "", 3},
	}
	if !slices.Equal(all, want) {
		t.Errorf("records = %+v, want %+v", all, want)
	}
}

// TestParseFrontmatter: each value is read as text whatever the emitter's
// quoting, and only the keys the block carries are classified.
func TestParseFrontmatter(t *testing.T) {
	for _, tc := range []struct {
		name, block, title, abstract string
		refused                      bool
	}{
		{"plain", "title: Plain Title", "Plain Title", "", false},
		{"double-quoted with escapes", `title: "Mary: An \"Account\"é"`, `Mary: An "Account"é`, "", false},
		{"single-quoted", `title: 'It''s: here'`, "It's: here", "", false},
		{"not one scalar", "title: \"Part one: x\"\n  Part two", `"Part one: x" Part two`, "", false},
		{"block scalar abstract", "title: T\nabstract: |\n  First line.\n\n  Second $`x`$.", "T", "First line.\nSecond $`x`$.", false},
		{"apparatus", "author:\n- A\n- B\ndate: 2026", "", "", false},
		{"unclassified key", "title: T\ndedication: To X", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fm, err := parseFrontmatter("---\n" + tc.block + "\n---\n\nbody\n")
			if tc.refused {
				var mk MetadataKeyError
				if !errors.As(err, &mk) {
					t.Fatalf("err = %v, want a MetadataKeyError", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if fm.title != tc.title || fm.abstract != tc.abstract || fm.body != "body" {
				t.Errorf("got title %q abstract %q body %q, want %q %q %q", fm.title, fm.abstract, fm.body, tc.title, tc.abstract, "body")
			}
		})
	}
}

func TestParentIndex(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"entry-point.md", ""},
		{"vol/index.md", "entry-point.md"},
		{"vol/leaf.md", "vol/index.md"},
		{"vol/sec/index.md", "vol/index.md"},
		{"vol/sec/leaf.md", "vol/sec/index.md"},
	} {
		if got := parentIndex(tc.path); got != tc.want {
			t.Errorf("parentIndex(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestReachability(t *testing.T) {
	good := map[string]string{
		"entry-point.md": "- [V](vol/index.md)",
		"vol/index.md":   "[↑ KB](../entry-point.md)\n\n- [L](leaf.md)",
		"vol/leaf.md":    "[↑ V](index.md)",
	}
	if got := reachability(good); len(got) != 1 || got[0].Status != StatusPass {
		t.Errorf("a well-formed tree: %v", got)
	}
	bad := map[string]string{
		"entry-point.md": "- [V](vol/index.md)",
		"vol/index.md":   "[↑ KB](../entry-point.md)",
		"vol/leaf.md":    "[↑ KB](../entry-point.md)",
		"vol/orphan.md":  "no up-link",
	}
	var details []string
	for _, f := range reachability(bad) {
		if f.Status != StatusFail {
			t.Errorf("unexpected %v", f)
		}
		details = append(details, f.Detail)
	}
	want := []string{
		`vol/leaf.md unreachable — no down-link chain from entry-point.md reaches it`,
		`vol/leaf.md up-link misparented — its "↑" link to "../entry-point.md" resolves to entry-point.md, but its parent is vol/index.md`,
		`vol/orphan.md no up-link — a non-root document carrying no "↑" link, so nothing walks up from it`,
		`vol/orphan.md unreachable — no down-link chain from entry-point.md reaches it`,
	}
	if !slices.Equal(details, want) {
		t.Errorf("findings =\n%q\nwant\n%q", details, want)
	}
}
