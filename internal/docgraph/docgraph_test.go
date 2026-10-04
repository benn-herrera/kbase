package docgraph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/records"
)

func TestSlug(t *testing.T) {
	for _, tc := range []struct{ title, want string }{
		{"Introduction", "introduction"},
		{"  Proof of Theorem 3.1  ", "proof-of-theorem-3-1"},
		{"", "untitled"},
		{"∑∫", "untitled"},
		{"Alexandrov-type rigidity for minimal capillary surfaces in a ball", "alexandrov-type-rigidity-for-minimal-capillary"},
		{strings.Repeat("a", 60), strings.Repeat("a", 48)},
	} {
		if got := slug(tc.title); got != tc.want {
			t.Errorf("slug(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestRelative(t *testing.T) {
	for _, tc := range []struct{ source, target, want string }{
		{"vol/index.md", "entry-point.md", "../entry-point.md"},
		{"vol/a/index.md", "vol/index.md", "../index.md"},
		{"vol/x.md", "vol/x.md", "x.md"},
		{"vol/a/b.md", "vol/c/d/e.md", "../c/d/e.md"},
		{"vol/index.md", "vol/assets/fig.png", "assets/fig.png"},
	} {
		if got := relative(tc.source, tc.target); got != tc.want {
			t.Errorf("relative(%q, %q) = %q, want %q", tc.source, tc.target, got, tc.want)
		}
	}
}

func TestDistinct(t *testing.T) {
	taken := map[string]bool{}
	got := []string{distinct("intro", taken, 1), distinct("intro", taken, 2), distinct("index", taken, 3), distinct("tools", taken, 4)}
	want := []string{"intro", "intro-2", "index-3", "tools-4"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("distinct #%d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLiftDisplayMath(t *testing.T) {
	str := func(s string) node { return node{"t": "Str", "c": s} }
	space := node{"t": "Space"}
	display := node{"t": "Math", "c": []any{node{"t": "DisplayMath"}, "x"}}
	got, ok := liftDisplayMath(node{"t": "Emph", "c": []any{str("a"), space, display, space, str("b")}})
	if !ok || len(got) != 3 {
		t.Fatalf("lifted = %v, want Emph, Math, Emph", got)
	}
	for i, want := range []string{"Emph", "Math", "Emph"} {
		if _, typ := asNode(got[i]); typ != want {
			t.Errorf("lifted[%d] is %s, want %s", i, typ, want)
		}
	}
	if _, ok := liftDisplayMath(node{"t": "Emph", "c": []any{str("a")}}); ok {
		t.Error("an Emph holding no display maths must be left alone")
	}
}

func TestSplitBibliography(t *testing.T) {
	body := "text\n\n<div id=\"refs\" class=\"references csl-bib-body hanging-indent\">\n<div id=\"ref-a\" class=\"csl-entry\">\nA.\n</div>\n</div>\n\n[^1]: note"
	content, bib, err := splitBibliography(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(bib, `<div id="refs"`) || !strings.HasSuffix(bib, "</div>\n</div>") {
		t.Errorf("bibliography = %q", bib)
	}
	if content != "text\n\n\n[^1]: note" {
		t.Errorf("content = %q", content)
	}
	if _, _, err := splitBibliography("<div id=\"refs\" class=\"csl-bib-body\">\n<div>"); err == nil {
		t.Error("an unclosed bibliography Div must fail")
	}
}

func TestControlRefusalMatchers(t *testing.T) {
	root := "main.tex"
	for _, tc := range []struct {
		name     string
		check    func(string, []string) []string
		refusals []string
		ok       bool
	}{
		{"unparseable named", unparseableRefusal, []string{"main.tex: unparseable source: Error at line 3"}, true},
		{"unparseable without the reader's error", unparseableRefusal, []string{"main.tex: unparseable source: "}, false},
		{"unparseable, other paper", unparseableRefusal, []string{"other.tex: unparseable source: x"}, false},
		{"unloadable, each named", unloadableRefusal, []string{
			"main.tex: could not load include sec1.tex, named at line 12 column 1",
			"main.tex: could not load include sec2.tex, named at line 13 column 1",
		}, true},
		{"unloadable without the line", unloadableRefusal, []string{"main.tex: could not load include sec1.tex"}, false},
		{"unloadable, none", unloadableRefusal, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.check(root, tc.refusals); (len(got) == 0) != tc.ok {
				t.Errorf("differences = %q, want ok=%v", got, tc.ok)
			}
		})
	}
}

// TestHandledNodeTypes: the set the census reads holds exactly the type*
// constants every branch names its node type through.
func TestHandledNodeTypes(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "ast.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, d := range f.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if !strings.HasPrefix(name.Name, "type") || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok {
					continue
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				declared[v] = true
			}
		}
	}
	for v := range declared {
		if !handledNodeTypes[v] {
			t.Errorf("%s is declared but not in handledNodeTypes", v)
		}
	}
	for v := range handledNodeTypes {
		if !declared[v] {
			t.Errorf("%s is in handledNodeTypes but declared by no type constant", v)
		}
	}
}

// TestCitationStates: each key of a citation is resolved, unanswered (citeproc
// marked it "key?") or key-only (no bibliography stood behind it).
func TestCitationStates(t *testing.T) {
	str := func(s string) node { return node{"t": typeStr, "c": s} }
	rendered := []any{str("(Author"), node{"t": typeSpace}, str("2000;"), node{"t": typeSpace},
		node{"t": typeStrong, "c": []any{str("missing?")}}, str(")")}
	for _, tc := range []struct {
		name     string
		keysOnly bool
		want     []string
	}{
		{"against a bibliography", false, []string{records.StateResolved, records.StateUnanswered}},
		{"no bibliography", true, []string{records.StateKeyOnly, records.StateKeyOnly}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &volumeReader{keysOnly: tc.keysOnly}
			got := c.states([]string{"found", "missing"}, rendered)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("states = %v, want %v", got, tc.want)
			}
		})
	}
}
