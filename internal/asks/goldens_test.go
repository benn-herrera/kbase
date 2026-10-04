package asks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/ledger"
	"kbase/internal/log"
)

// kb_tools' composed-prompt goldens: each test file composes its asks from
// fixture templates and requires the prompt byte for byte. The goldens and
// fixture templates are read from the file at the shelf's recorded commit; the
// inputs are kb_tools' transcribed.
const (
	paragraphGoldens = "kb_tools/tests/test_kb_claimgraph_cinf.py"
	classifyGoldens  = "kb_tools/tests/test_kb_claimgraph_pass2.py"
)

// returnedGolden is what the goldens' re-ask carries back.
const returnedGolden = "  I would say B, probably.\n"

// TestComposedPromptsReplayKbToolsGoldens composes kb_tools' golden paragraph
// and classify asks — first ask and re-ask, each item — from kb_tools' fixture
// templates and requires its golden bytes. It skips where the clone is absent.
func TestComposedPromptsReplayKbToolsGoldens(t *testing.T) {
	clone := filepath.Join("..", "..", ".claude", "adjagent")
	if _, err := os.Stat(filepath.Join(clone, ".git")); err != nil {
		t.Skipf("no adjagent clone at %s: %v", clone, err)
	}
	repo, err := ledger.Open(clone, nil, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	commit := ""
	for _, f := range readProvenance(t).Files {
		if commit != "" && f.Commit != commit {
			t.Fatalf("the shelf is taken at %s and %s; the goldens are read at one commit", commit, f.Commit)
		}
		commit = f.Commit
	}
	source := func(path string) string {
		t.Helper()
		files, err := repo.Files(commit, path)
		if err != nil || len(files) != 1 {
			t.Fatalf("%s at %s: %d files, %v", path, commit, len(files), err)
		}
		for _, b := range files {
			return string(b)
		}
		return ""
	}

	paragraph := source(paragraphGoldens)
	paragraphs := paragraphAsks(fixtureShelf(t, paragraph), "vol/one.md", "# One\n\nS1: It contracts whenever $\\frac{1}{2} k < 1$.\n\nS2: Notation.\n",
		[]ParagraphItem{{"S1", "S1: It contracts whenever $\\frac{1}{2} k < 1$.\n"}, {"S2", "S2: Notation.\n"}})
	replayGoldens(t, paragraphGoldens, paragraph, paragraphs)

	classify := source(classifyGoldens)
	candidates := classifyAsks(fixtureShelf(t, classify),
		Claim{ID: "clm-aaaaaa", Title: "Theorem 1", Document: "vol/a.md", Locator: "**Theorem 1**."},
		"**Theorem 1**. The map has a unique fixed point.\n",
		[]ClassifyItem{{
			Target:    Claim{ID: "clm-bbbbbb", Title: "Lemma 2", Document: "vol/b.md", Locator: "**Lemma 2**."},
			Statement: "**Lemma 2**. The map is a contraction.\n",
			Passages:  []string{"vol/a.md:7: By Lemma 2 the map contracts."},
			Offered:   []string{LetterSupportedBy, LetterInSupportOf, LetterMention},
		}, {
			Target:    Claim{ID: "clm-cccccc", Title: "Equation (3)", Document: "vol/b.md"},
			Statement: "$$ k = \\sup |f'| $$",
			Passages:  []string{"vol/a.md:9: with $k$ as in (3).", "vol/a.md:7: By Lemma 2 the map contracts."},
			Offered:   []string{LetterSupportedBy, LetterMention},
		}})
	replayGoldens(t, classifyGoldens, classify, candidates)
}

// replayGoldens requires each item's first ask and re-ask equal to the
// golden prefix, the item's golden and, on the re-ask, the golden correction.
func replayGoldens(t *testing.T, file, src string, items []LetterItem) {
	t.Helper()
	prefix := pyString(t, pyExpr(t, src, "_GOLDEN_PREFIX"))
	golden := pyTuple(t, pyExpr(t, src, "_GOLDEN_ITEMS"))
	correction := pyString(t, pyExpr(t, src, "_GOLDEN_CORRECTION"))
	if len(golden) != len(items) {
		t.Fatalf("%s: %d golden items for %d asks", file, len(golden), len(items))
	}
	returned := returnedGolden
	for i, it := range items {
		for _, r := range []*string{nil, &returned} {
			want := prefix + golden[i]
			if r != nil {
				want += correction
			}
			want += "\n"
			got, err := it.Compose(r)
			if err != nil {
				t.Fatalf("%s item %d: %v", file, i, err)
			}
			if got != want {
				t.Errorf("%s item %d (re-ask %t):\n got %q\nwant %q", file, i, r != nil, got, want)
			}
		}
	}
}

// fixtureShelf is a test file's _FIXTURE_TEMPLATES by the shelf path each
// key names; a template it does not stand in for is not on it.
func fixtureShelf(t *testing.T, src string) func(string) (string, error) {
	t.Helper()
	byPath := map[string]string{}
	for key, value := range pyDict(t, pyExpr(t, src, "_FIXTURE_TEMPLATES")) {
		var path string
		switch {
		case strings.HasSuffix(key, "LETTER_TEMPLATES[letters.Kind.PARAGRAPH]"):
			path = paragraphTemplate
		case strings.HasSuffix(key, "LETTER_TEMPLATES[letters.Kind.CLASSIFY]"):
			path = classifyTemplate
		case strings.HasSuffix(key, "ALTERNATIVES[ask.LETTER_CORRECTION]"):
			path = fragmentFile("letter-correction")
		case strings.HasPrefix(key, "prompt_templates.ALTERNATIVES[\""):
			path = fragmentFile(strings.TrimSuffix(strings.TrimPrefix(key, "prompt_templates.ALTERNATIVES[\""), "\"]"))
		default:
			t.Fatalf("a fixture template keyed %s names no template this test knows", key)
		}
		byPath[path] = value
	}
	return func(name string) (string, error) {
		text, ok := byPath[name]
		if !ok {
			return "", ComposeError{name, "no fixture template stands in for it"}
		}
		return text, nil
	}
}

// pyToken is one token of a Python expression: a string literal's decoded
// value, or the raw text of anything else, with the bracket depth it sits at.
type pyToken struct {
	str   bool
	text  string
	depth int
}

// pyExpr is the tokens of the expression src assigns to name at its top
// level, through the end of its last line.
func pyExpr(t *testing.T, src, name string) []pyToken {
	t.Helper()
	at := strings.Index(src, "\n"+name+" = ")
	if at < 0 {
		t.Fatalf("no top-level assignment to %s", name)
	}
	s := src[at+len(name)+4:]
	var tokens []pyToken
	depth := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n' && depth == 0:
			return tokens
		case c == ' ' || c == '\t' || c == '\n':
			i++
		case c == '#':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '"' || c == '\'':
			value, n := pyStringLiteral(t, s[i:])
			tokens = append(tokens, pyToken{str: true, text: value, depth: depth})
			i += n
		case strings.IndexByte("([{", c) >= 0:
			tokens = append(tokens, pyToken{text: string(c), depth: depth})
			depth++
			i++
		case strings.IndexByte(")]}", c) >= 0:
			depth--
			tokens = append(tokens, pyToken{text: string(c), depth: depth})
			i++
		case c == ',' || c == ':':
			tokens = append(tokens, pyToken{text: string(c), depth: depth})
			i++
		default:
			j := i
			for j < len(s) && strings.IndexByte(" \t\n#\"'()[]{},:", s[j]) < 0 {
				j++
			}
			tokens = append(tokens, pyToken{text: s[i:j], depth: depth})
			i = j
		}
	}
	return tokens
}

// pyStringLiteral decodes the string literal s opens with — single or triple
// quoted, its backslash escapes resolved, a backslash-newline joining lines —
// and returns its value and length.
func pyStringLiteral(t *testing.T, s string) (string, int) {
	t.Helper()
	quote := s[:1]
	if strings.HasPrefix(s, strings.Repeat(quote, 3)) {
		quote = strings.Repeat(quote, 3)
	}
	var b strings.Builder
	for i := len(quote); i < len(s); i++ {
		if strings.HasPrefix(s[i:], quote) {
			return b.String(), i + len(quote)
		}
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case '\n':
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '\\', '"', '\'':
			b.WriteByte(s[i])
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	t.Fatalf("an unterminated string literal: %.40q", s)
	return "", 0
}

// pyString is an expression of string literals, joined as Python joins
// adjacent ones.
func pyString(t *testing.T, tokens []pyToken) string {
	t.Helper()
	var b strings.Builder
	for _, tk := range tokens {
		if tk.str {
			b.WriteString(tk.text)
		} else if !strings.Contains("()", tk.text) {
			t.Fatalf("%q in an expression of string literals", tk.text)
		}
	}
	return b.String()
}

// pyElements splits a bracketed expression's tokens at its top-level commas.
func pyElements(tokens []pyToken) [][]pyToken {
	var out [][]pyToken
	var cur []pyToken
	for _, tk := range tokens[1 : len(tokens)-1] {
		if tk.depth == 1 && tk.text == "," {
			out = append(out, cur)
			cur = nil
			continue
		}
		cur = append(cur, tk)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// pyTuple is a tuple of string expressions.
func pyTuple(t *testing.T, tokens []pyToken) []string {
	t.Helper()
	var out []string
	for _, e := range pyElements(tokens) {
		out = append(out, pyString(t, e))
	}
	return out
}

// pyDict is a dict of string expressions, keyed by each key's source text.
func pyDict(t *testing.T, tokens []pyToken) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, e := range pyElements(tokens) {
		colon := -1
		for i, tk := range e {
			if tk.depth == 1 && tk.text == ":" {
				colon = i
				break
			}
		}
		if colon < 0 {
			t.Fatalf("a dict entry with no key: %v", e)
		}
		var key strings.Builder
		for _, tk := range e[:colon] {
			if tk.str {
				key.WriteString(`"` + tk.text + `"`)
			} else {
				key.WriteString(tk.text)
			}
		}
		out[key.String()] = pyString(t, e[colon+1:])
	}
	return out
}
