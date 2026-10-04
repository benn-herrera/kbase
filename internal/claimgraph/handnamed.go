package claimgraph

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"kbase/internal/kb"
)

// A claim named by hand is a printed name and number with no \ref — "follows
// from Lemma 4.6" — read off the words of a claim's body and offered as an
// edge candidate where it joins a claim. The names are the corpus's own
// display names; the number is any dotted depth or a single capital, a
// lowercase sub-item suffix dropped; a printed list names each item.
//
// The matchers below enumerate the alternatives a backtracking reader tries,
// in the order it tries them, because the item grammar turns on what follows
// a number: "4.6a1" is item 4, not 4.6.

var (
	// printedSpanRE is the printed word and number in bold, anywhere.
	printedSpanRE = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	// citedAfterRE is a mention handed to a citation by its next words.
	citedAfterRE = kb.PyRE(`^\s+(?:of|in)\s+<span\s+class="citation"`)
	// parentheticalRE is what may follow a label's number: a parenthesis.
	parentheticalRE = kb.PyRE(`^\s*\(.*\)\z`)
)

// vocabulary is one corpus's claim names and their plurals, longest first.
type vocabulary struct {
	forms     []string
	canonical map[string]string
}

func newVocabulary(names []string) vocabulary {
	v := vocabulary{canonical: map[string]string{}}
	for _, n := range names {
		plural := n + "s"
		if strings.HasSuffix(n, "y") {
			plural = n[:len(n)-1] + "ies"
		}
		for _, f := range []string{n, plural} {
			if !slices.Contains(v.forms, f) {
				v.forms = append(v.forms, f)
			}
			v.canonical[strings.ToLower(f)] = strings.ToLower(n)
		}
	}
	slices.SortStableFunc(v.forms, func(a, b string) int { return len(b) - len(a) })
	return v
}

func (v vocabulary) name(spelled string) string {
	return v.canonical[strings.ToLower(strings.Join(strings.FieldsFunc(spelled, kb.IsSpace), " "))]
}

// wsRun is the offsets after each successive whitespace character from p:
// out[0] is p.
func wsRun(s string, p int) []int {
	out := []int{p}
	for p < len(s) {
		r, size := utf8.DecodeRuneInString(s[p:])
		if !kb.IsSpace(r) {
			break
		}
		p += size
		out = append(out, p)
	}
	return out
}

// formAt is the end of form spelled at p — its first letter in either case,
// any whitespace run between its words — or -1.
func formAt(s string, p int, form string) int {
	head, size := utf8.DecodeRuneInString(form)
	r, rsize := utf8.DecodeRuneInString(s[p:])
	if p >= len(s) || (r != unicode.ToUpper(head) && r != unicode.ToLower(head)) {
		return -1
	}
	p += rsize
	for i, word := range strings.Split(form[size:], " ") {
		if i > 0 {
			ws := wsRun(s, p)
			if len(ws) < 2 {
				return -1
			}
			p = ws[len(ws)-1]
		}
		if !strings.HasPrefix(s[p:], word) {
			return -1
		}
		p += len(word)
	}
	return p
}

func digitRun(s string, p int) []int {
	out := []int{p}
	for p < len(s) {
		r, size := utf8.DecodeRuneInString(s[p:])
		if !unicode.IsDigit(r) {
			break
		}
		p += size
		out = append(out, p)
	}
	return out
}

// numberEnds is each end of a printed number at p, in the order a
// backtracking reader tries them: a capital, or a digit run longest first,
// then dotted groups greedily.
func numberEnds(s string, p int) []int {
	var bases []int
	if p < len(s) && s[p] >= 'A' && s[p] <= 'Z' {
		bases = []int{p + 1}
	} else if d := digitRun(s, p); len(d) > 1 {
		for k := len(d) - 1; k >= 1; k-- {
			bases = append(bases, d[k])
		}
	}
	var out []int
	var groups func(e int)
	groups = func(e int) {
		if e < len(s) && s[e] == '.' {
			d := digitRun(s, e+1)
			for k := len(d) - 1; k >= 1; k-- {
				groups(d[k])
			}
		}
		out = append(out, e)
	}
	for _, b := range bases {
		groups(b)
	}
	return out
}

func wordAt(s string, p int) bool {
	if p >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[p:])
	return kb.IsWord(r)
}

// itemAt is the printed number at p and the end of its item — the number,
// an optional lowercase suffix, and no word character after.
func itemAt(s string, p int) (string, int, bool) {
	for _, n := range numberEnds(s, p) {
		ends := []int{n}
		if n < len(s) && s[n] >= 'a' && s[n] <= 'z' {
			ends = []int{n + 1, n}
		}
		for _, e := range ends {
			if !wordAt(s, e) {
				return s[p:n], e, true
			}
		}
	}
	return "", 0, false
}

// separatorEnds is each end of a list separator at p, in the order a
// backtracking reader tries them.
func separatorEnds(s string, p int) []int {
	var out []int
	desc := func(ws []int, from int) []int {
		var o []int
		for k := len(ws) - 1; k >= from; k-- {
			o = append(o, ws[k])
		}
		return o
	}
	words := func(q int, then func(int)) {
		for _, w := range []string{"and", "or"} {
			if strings.HasPrefix(s[q:], w) {
				then(q + len(w))
			}
		}
	}
	for _, q := range desc(wsRun(s, p), 0) {
		if strings.HasPrefix(s[q:], ",") {
			for _, q2 := range desc(wsRun(s, q+1), 0) {
				words(q2, func(q3 int) { out = append(out, desc(wsRun(s, q3), 1)...) })
				out = append(out, q2)
			}
		}
		if ws := wsRun(s, q); len(ws) > 1 {
			words(ws[1], func(q3 int) { out = append(out, desc(wsRun(s, q3), 1)...) })
		}
		for _, tok := range []string{"&", "–", "—", "--"} {
			if strings.HasPrefix(s[q:], tok) {
				out = append(out, desc(wsRun(s, q+len(tok)), 0)...)
			}
		}
	}
	return out
}

// items is the printed list at p: its numbers and its end.
func items(s string, p int) ([]string, int, bool) {
	n, end, ok := itemAt(s, p)
	if !ok {
		return nil, 0, false
	}
	numbers := []string{n}
	for {
		found := false
		for _, q := range separatorEnds(s, end) {
			if n, e, ok := itemAt(s, q); ok {
				numbers, end, found = append(numbers, n), e, true
				break
			}
		}
		if !found {
			return numbers, end, true
		}
	}
}

type mention struct {
	offset  int
	name    string
	numbers []string
}

// blankBytes is s with every byte but a newline a space, offsets kept.
func blankBytes(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}
	return string(b)
}

// mentions is every hand-written mention in text, anchors and citations
// blanked first and a mention its next words hand to a citation skipped.
func (v vocabulary) mentions(page string) []mention {
	text := citationSpanRE.ReplaceAllStringFunc(anchorRenderingRE.ReplaceAllStringFunc(page, blankBytes), blankBytes)
	var out []mention
	for i := 0; i < len(text); {
		_, size := utf8.DecodeRuneInString(text[i:])
		if i > 0 {
			if prev, _ := utf8.DecodeLastRuneInString(text[:i]); kb.IsWord(prev) {
				i += size
				continue
			}
		}
		matched := false
		for _, f := range v.forms {
			e := formAt(text, i, f)
			if e < 0 {
				continue
			}
			ws := wsRun(text, e)
			if len(ws) < 2 {
				continue
			}
			numbers, end, ok := items(text, ws[len(ws)-1])
			if !ok {
				continue
			}
			matched = true
			if !citedAfterRE.MatchString(page[end:]) {
				var unique []string
				for _, n := range numbers {
					if !slices.Contains(unique, n) {
						unique = append(unique, n)
					}
				}
				out = append(out, mention{i, v.name(text[i:e]), unique})
			}
			i = end
			break
		}
		if !matched {
			i += size
		}
	}
	return out
}

// label is text's name and number where the whole of it is one printed label.
func (v vocabulary) label(text string) (string, string, bool) {
	text = kb.Strip(text)
	for _, f := range v.forms {
		e := formAt(text, 0, f)
		if e < 0 {
			continue
		}
		ws := wsRun(text, e)
		if len(ws) < 2 {
			continue
		}
		q := ws[len(ws)-1]
		for _, n := range numberEnds(text, q) {
			suffix := []int{n}
			if n < len(text) && text[n] >= 'a' && text[n] <= 'z' {
				suffix = []int{n + 1, n}
			}
			for _, c := range suffix {
				stop := []int{c}
				if strings.HasPrefix(text[c:], ".") {
					stop = []int{c + 1, c}
				}
				for _, d := range stop {
					if rest := text[d:]; rest == "" || parentheticalRE.MatchString(rest) {
						return v.name(text[:e]), text[q:n], true
					}
				}
			}
		}
	}
	return "", "", false
}

// blockClaimSite is a claim-bearing block's claim, joined by locator.
type blockClaimSite struct {
	node       ClaimNode
	start, end int
}

func blockClaimSites(g *AuthoredGraph, inv *Inventory) []blockClaimSite {
	var out []blockClaimSite
	for _, b := range inv.claimBlocks() {
		if n, ok := claimOf(&b, g.hostedBy(b.Document)); ok {
			out = append(out, blockClaimSite{n, b.Start, b.End})
		}
	}
	return out
}

type nameNumber struct{ name, number string }

// printedClaims is every claim a printed name and number joins: a block by
// its display line's printed span, a claim with no block by a title that is
// that label and nothing more.
func printedClaims(g *AuthoredGraph, sites []blockClaimSite, v vocabulary) map[nameNumber][]ClaimNode {
	joined := map[nameNumber][]ClaimNode{}
	inBlocks := map[string]bool{}
	for _, s := range sites {
		inBlocks[s.node.ID] = true
		if m := printedNameRE.FindStringSubmatch(s.node.Locator); m != nil {
			if name, number, ok := v.label(m[1]); ok {
				k := nameNumber{name, number}
				joined[k] = append(joined[k], s.node)
			}
		}
	}
	for _, n := range g.Nodes {
		if n.Equation == "" && !inBlocks[n.ID] {
			if name, number, ok := v.label(n.Title); ok {
				k := nameNumber{name, number}
				joined[k] = append(joined[k], n)
			}
		}
	}
	return joined
}

// claimBody is a block or prose claim's body: its block's extent, or the
// paragraph its marker sits in, markers off and unquoted, line for line, with
// the document line it begins on. An equation node has neither.
type claimBody struct {
	node    ClaimNode
	first   int
	text    string
	inBlock bool
}

func claimBodies(t *Tree, g *AuthoredGraph, inv *Inventory) []claimBody {
	lines := map[string][]string{}
	of := func(doc string) []string {
		if lines[doc] == nil {
			lines[doc] = pageLines(t, doc)
		}
		return lines[doc]
	}
	var out []claimBody
	inBlocks := map[string]bool{}
	for _, s := range blockClaimSites(g, inv) {
		inBlocks[s.node.ID] = true
		l := of(s.node.Document)
		out = append(out, claimBody{s.node, s.start, strings.Join(l[min(s.start, len(l)):min(s.end, len(l))], "\n"), true})
	}
	inProse := map[string][]ClaimNode{}
	var docs []string
	for _, n := range g.Nodes {
		if n.Equation == "" && !inBlocks[n.ID] {
			if inProse[n.Document] == nil {
				docs = append(docs, n.Document)
			}
			inProse[n.Document] = append(inProse[n.Document], n)
		}
	}
	slices.Sort(docs)
	for _, doc := range docs {
		leaf := readLeaf(t.Documents[doc], inv)
		l := of(doc)
		for _, p := range leaf.render.paragraphs {
			numbers := slices.Sorted(slices.Values(p.lines))
			for _, n := range inProse[doc] {
				if !leaf.marks(p, n.ID) {
					continue
				}
				text := make([]string, 0, len(numbers))
				for _, k := range numbers {
					if k < len(l) {
						text = append(text, l[k])
					}
				}
				out = append(out, claimBody{n, numbers[0], strings.Join(text, "\n"), false})
			}
		}
	}
	return out
}

// handNamed is one candidate a hand-written name joins, and where the first
// mention producing it sits.
type handNamed struct {
	source, target string
	document       string
	line           int
}

// harvestHandNamed is every candidate a hand-written name joins in a claim's
// body, one per pair, in pair order. A block's own display label is blanked,
// since corpora print two blocks under one number often enough that reading
// it would join each to its twin. Where a label joins claims in several
// volumes the mention's own volume's are preferred.
func harvestHandNamed(t *Tree, g *AuthoredGraph, inv *Inventory) []handNamed {
	var names []string
	for _, b := range inv.claimBlocks() {
		if !slices.Contains(names, b.Environment) {
			names = append(names, b.Environment)
		}
	}
	if len(names) == 0 {
		return nil
	}
	v := newVocabulary(names)
	printed := printedClaims(g, blockClaimSites(g, inv), v)
	found := map[pair]handNamed{}
	for _, b := range claimBodies(t, g, inv) {
		body := b.text
		if b.inBlock {
			first, _, _ := strings.Cut(body, "\n")
			if from := len(first) + 1; from <= len(body) {
				if m := printedSpanRE.FindStringIndex(body[from:]); m != nil {
					body = body[:from+m[0]] + blankBytes(body[from+m[0]:from+m[1]]) + body[from+m[1]:]
				}
			}
		}
		doc := b.node.Document
		volume := t.Documents[doc].Domain()
		for _, m := range v.mentions(body) {
			for _, number := range m.numbers {
				joined := printed[nameNumber{m.name, number}]
				var local []ClaimNode
				for _, n := range joined {
					if t.Documents[n.Document].Domain() == volume {
						local = append(local, n)
					}
				}
				if len(local) == 0 {
					local = joined
				}
				for _, target := range local {
					p := pair{b.node.ID, target.ID}
					if _, seen := found[p]; !seen && target.ID != b.node.ID {
						found[p] = handNamed{p.source, p.target, doc, b.first + strings.Count(body[:m.offset], "\n")}
					}
				}
			}
		}
	}
	out := make([]handNamed, 0, len(found))
	for _, h := range found {
		out = append(out, h)
	}
	slices.SortFunc(out, func(a, b handNamed) int { return comparePairs(pair{a.source, a.target}, pair{b.source, b.target}) })
	return out
}
