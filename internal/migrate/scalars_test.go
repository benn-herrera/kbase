package migrate_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/migrate"
)

// awkwardStrings are scalars a writer's quoting rule can get wrong: YAML 1.1
// words, number and date look-alikes, indicators, quotes, backslashes, line
// breaks, whitespace at either end and non-ASCII text.
var awkwardStrings = []string{
	"", "y", "Y", "n", "yes", "Yes", "NO", "on", "Off", "true", "False", "null", "Null", "~",
	"0", "1", "-1", "1.5", "1e5", "0x1F", "0o17", "1_000", ".inf", "-.Inf", ".nan", "2024-01-01", "12:30",
	"1abc", "9lives", "a'b", `a"b`, `back\slash`, "tab\there", "line\nbreak", "crlf\r\nx", "trailing ", "  leading",
	"#hash", "a #b", "a: b", "- dash", "[x]", "{y}", "*star", "&anchor", "!tag", "%pct", "@at", "`tick", "|pipe",
	">gt", "?q", "é", "日本語", "emoji \U0001F600", "line\xe2\x80\xa8separator", "next\xc2\x85line", "\v\f",
	"clm-abc123", "kb-root/a.md", "a/b_c.d-e", "kind", "*pending*",
}

var awkwardNumbers = []string{"0", "1", "-1", "100", "1.5", "0.0", "-0.5", "1e-05", "2.5E+3"}

// goldenScalars is every string and number scalar the 1.0.0 goldens hold:
// the documents' frontmatter, the index streams and the build records.
func goldenScalars(t *testing.T) (strs, nums []string) {
	t.Helper()
	err := filepath.WalkDir(filepath.Join("testdata", "1.0.0"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		text := string(data)
		if strings.HasSuffix(p, ".md") {
			m := kb.FindFrontmatter(text)
			if m == nil {
				return nil
			}
			text = text[m[2]:m[3]]
		}
		dec := yaml.NewDecoder(strings.NewReader(text))
		for {
			var doc yaml.Node
			if err := dec.Decode(&doc); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return err
			}
			var walk func(n *yaml.Node)
			walk = func(n *yaml.Node) {
				if n.Kind == yaml.ScalarNode {
					switch n.ShortTag() {
					case "!!str":
						strs = append(strs, n.Value)
					case "!!int", "!!float":
						nums = append(nums, n.Value)
					}
				}
				for _, c := range n.Content {
					walk(c)
				}
			}
			walk(&doc)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(strs) == 0 || len(nums) == 0 {
		t.Fatalf("the goldens gave %d strings and %d numbers", len(strs), len(nums))
	}
	return strs, nums
}

func migrateRendering(t *testing.T, key, value *yaml.Node) string {
	t.Helper()
	var b bytes.Buffer
	if err := migrate.EncodeYAML(&b, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{key, value}}); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func kbRendering(t *testing.T, key string, value *yaml.Node) string {
	t.Helper()
	lines, err := kb.RenderFrontmatterField(key, value)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n") + "\n"
}

// TestConverterFindsFrontmatterAsKBDoes: the converter takes a document as
// already in YAML frontmatter exactly where internal/kb finds frontmatter,
// over the goldens and documents opening with prose between rules.
func TestConverterFindsFrontmatterAsKBDoes(t *testing.T) {
	docs := []string{
		"# T\n", "---\n---\n", "---\n\n---\n", "---\nkind: leaf\n---\n", "---\r\nkind: leaf\r\n---\r\n", "---\n\nkind: leaf\n---\n",
		"---\nkey: [unclosed\n---\n", "---\nSome notes: with: colons\n---\n", "---\nPlain prose.\n---\n", "---\n- a list\n---\n",
		"---\n  indented: x\n---\n", "---\nKind: leaf\n---\n", "---\nkb-format: \"1.0.0\"\n---\n", "---\nunclosed: true\n", "--- a rule\n",
	}
	err := filepath.WalkDir(filepath.Join("testdata", "1.0.0"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		data, err := os.ReadFile(p)
		docs = append(docs, string(data))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
		_, _, got := migrate.YAMLFrontmatter(doc)
		if want := kb.FindFrontmatter(doc) != nil; got != want {
			t.Errorf("%q: migrate finds frontmatter %t, kb %t", doc, got, want)
		}
	}
}

// TestConverterWritesScalarsAsKBDoes: the 0.9.0 converter's copies of the
// 1.0.0 writer's scalar rules render, split and strip every golden and
// awkward value exactly as internal/kb's, since migrate may not import kb.
func TestConverterWritesScalarsAsKBDoes(t *testing.T) {
	strs, nums := goldenScalars(t)
	strs = append(strs, awkwardStrings...)
	nums = append(nums, awkwardNumbers...)
	for _, s := range strs {
		if got, want := migrateRendering(t, migrate.StringNode("k"), migrate.StringNode(s)), kbRendering(t, "k", kb.FrontmatterString(s)); got != want {
			t.Errorf("value %q: migrate writes %q, kb %q", s, got, want)
		}
		if got, want := migrateRendering(t, migrate.StringNode(s), migrate.StringNode("v")), kbRendering(t, s, kb.FrontmatterString("v")); got != want {
			t.Errorf("key %q: migrate writes %q, kb %q", s, got, want)
		}
		if !slices.Equal(migrate.SplitLines(s), kb.SplitLines(s)) || migrate.Strip(s) != kb.Strip(s) || migrate.RStrip(s) != kb.RStrip(s) {
			t.Errorf("%q: migrate splits or strips it other than kb", s)
		}
	}
	for _, n := range nums {
		if got, want := migrate.NumberTag(n), kb.FrontmatterNumber(n).Tag; got != want {
			t.Errorf("number %q: migrate tags it %s, kb %s", n, got, want)
		}
		pair := func(s string) string {
			m := migrateRendering(t, migrate.StringNode("k"), &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{
				{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{migrate.StringNode(s), {Kind: yaml.ScalarNode, Tag: migrate.NumberTag(n), Value: n}}}}})
			k := kbRendering(t, "k", kb.FrontmatterList([]*yaml.Node{kb.FrontmatterMapping(kb.FrontmatterString(s), kb.FrontmatterNumber(n))}))
			if m != k {
				return m + " / " + k
			}
			return ""
		}
		if diff := pair("clm-abc123"); diff != "" {
			t.Errorf("a nested pair scoring %s: migrate / kb: %s", n, diff)
		}
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if migrate.IsSpace(r) != kb.IsSpace(r) {
			t.Errorf("IsSpace(%U): migrate %t, kb %t", r, migrate.IsSpace(r), kb.IsSpace(r))
		}
	}
}
