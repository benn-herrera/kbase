package docgraph

import (
	"cmp"
	"fmt"
	"html"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Finding statuses: a FACT describes state and never gates.
const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
	StatusFact = "FACT"
)

// The build's checks, named as kb_tools' report names them.
const (
	nameAST       = "A-ast-to-markdown"
	nameTree      = "B-markdown-to-tree"
	nameMath      = "math-survives"
	nameAnchor    = "anchor-lands"
	nameAsset     = "image-asset"
	nameLinks     = "verify-md-links"
	nameReachable = "validate-build/4-reachability"
)

// Finding is one line of a build's report.
type Finding struct {
	Status, Check, Detail string
}

func (f Finding) String() string { return f.Status + " " + f.Check + " " + f.Detail }

// excerptWords is how much of a missed run a finding quotes.
const excerptWords = 14

// checkASTAgainstMarkdown is check A: every word run of every leaf block and
// of every content-bearing metadata value reaches the whole-volume rendering,
// in order. A run, not a word set: an abstract restates the body's vocabulary
// and not its phrasing.
func checkASTAgainstMarkdown(stem string, doc node, markdown string) []Finding {
	rendered := markdownTokens(markdown)
	var findings []Finding
	blocks := leafBlocks(doc["blocks"])
	for _, b := range blocks {
		for _, run := range textRuns(b) {
			if !holds(rendered, run) {
				findings = append(findings, Finding{StatusFail, nameAST, fmt.Sprintf("%s block %s lost: %s", stem, str(b["t"]), excerpt(run))})
			}
		}
	}
	meta, _ := doc["meta"].(node)
	entries := metaEntries(meta)
	var keys []string
	for _, e := range entries {
		if !slices.Contains(keys, e.key) {
			keys = append(keys, e.key)
		}
		for _, run := range e.runs {
			if !holds(rendered, run) {
				findings = append(findings, Finding{StatusFail, nameAST, fmt.Sprintf("%s meta.%s lost: %s", stem, e.path, excerpt(run))})
			}
		}
	}
	if len(findings) > 0 {
		return findings
	}
	slices.Sort(keys)
	return []Finding{{StatusPass, nameAST, fmt.Sprintf("%s %d blocks and %d meta entries (%s) all reach the rendering",
		stem, len(blocks), len(entries), strings.Join(keys, ", "))}}
}

// holds is whether run occurs in rendered as a contiguous subsequence.
func holds(rendered, run []string) bool {
	if len(run) == 0 {
		return true
	}
	for start := 0; start+len(run) <= len(rendered); start++ {
		if rendered[start] == run[0] && slices.Equal(rendered[start:start+len(run)], run) {
			return true
		}
	}
	return false
}

func excerpt(run []string) string {
	shown := strings.Join(run[:min(len(run), excerptWords)], " ")
	more := ""
	if len(run) > excerptWords {
		more = "..."
	}
	return fmt.Sprintf("%q%s (%d words)", shown, more, len(run))
}

// counter is a multiset of words.
type counter map[string]int

func countOf(tokens []string) counter {
	c := counter{}
	for _, t := range tokens {
		c[t]++
	}
	return c
}

// minus is what c holds beyond d.
func (c counter) minus(d counter) counter {
	out := counter{}
	for k, n := range c {
		if n > d[k] {
			out[k] = n - d[k]
		}
	}
	return out
}

func (c counter) total() int {
	n := 0
	for _, v := range c {
		n += v
	}
	return n
}

// checkMarkdownAgainstTree is check B: the volume's words partition across
// the segments the split cut, both ways, and each document's body keeps every
// word of its segment. Navigation lines are not body, so a heading in its
// parent's child list is not a duplicate.
func checkMarkdownAgainstTree(stem string, t *volumeTree) []Finding {
	volume := countOf(t.contentTokens)
	assigned := counter{}
	documents := t.index.walk()
	for _, n := range documents {
		for _, tok := range markdownTokens(n.segment) {
			assigned[tok]++
		}
	}
	var findings []Finding
	findings = append(findings, wordDifference(stem, "dropped by the split", volume.minus(assigned))...)
	findings = append(findings, wordDifference(stem, "duplicated by the split", assigned.minus(volume))...)
	for _, n := range documents {
		lost := countOf(markdownTokens(n.segment)).minus(countOf(markdownTokens(n.body)))
		findings = append(findings, wordDifference(stem+" "+n.path, "lost by a per-document transform", lost)...)
	}
	if len(findings) > 0 {
		return findings
	}
	return []Finding{{StatusPass, nameTree, fmt.Sprintf("%s %d words partition across %d documents", stem, volume.total(), len(documents))}}
}

func wordDifference(subject, what string, diff counter) []Finding {
	if len(diff) == 0 {
		return nil
	}
	words := make([]string, 0, len(diff))
	for w := range diff {
		words = append(words, w)
	}
	slices.SortFunc(words, func(a, b string) int { return cmp.Or(cmp.Compare(diff[b], diff[a]), strings.Compare(a, b)) })
	shown := make([]string, 0, excerptWords)
	for _, w := range words[:min(len(words), excerptWords)] {
		shown = append(shown, fmt.Sprintf("%q×%d", w, diff[w]))
	}
	return []Finding{{StatusFail, nameTree, fmt.Sprintf("%s: %d word(s) %s, e.g. %s", subject, diff.total(), what, strings.Join(shown, ", "))}}
}

// The writer's three spellings of maths: an inline code span, a math fence,
// and a caption's MathML annotation, whose LaTeX is XML text.
var (
	inlineMathRe    = regexp.MustCompile("(?s)\\$`(.*?)`\\$")
	annotatedMathRe = regexp.MustCompile(`(?s)<annotation encoding="application/x-tex">(.*?)</annotation>`)
)

const displayMathInfo = "math"

// checkMath is the maths-survival count: every Math element of the blocks
// and the content metadata reaches some document's body, by literal content
// with whitespace runs closed up.
func checkMath(stem string, doc node, t *volumeTree) []Finding {
	source := counter{}
	for _, m := range mathTexts(doc) {
		source[normalizedMath(m)]++
	}
	placed := counter{}
	for _, n := range t.index.walk() {
		for _, m := range mathIn(n.body) {
			placed[normalizedMath(m)]++
		}
	}
	missing := source.minus(placed)
	if len(missing) == 0 {
		return []Finding{{StatusPass, nameMath, fmt.Sprintf("%s %d maths elements all reach a document", stem, source.total())}}
	}
	texts := make([]string, 0, len(missing))
	for m := range missing {
		texts = append(texts, m)
	}
	slices.Sort(texts)
	var shown []string
	for _, m := range texts[:min(len(texts), 3)] {
		r := []rune(m)
		shown = append(shown, fmt.Sprintf("%q", string(r[:min(len(r), 80)])))
	}
	return []Finding{{StatusFail, nameMath, fmt.Sprintf("%s: %d maths element(s) reach no document, e.g. %s", stem, missing.total(), strings.Join(shown, "; "))}}
}

func mathIn(body string) []string {
	plain := unquoted(body)
	var out []string
	for _, m := range inlineMathRe.FindAllStringSubmatch(plain, -1) {
		out = append(out, m[1])
	}
	for _, f := range fencedBlocks(body) {
		if f.info == displayMathInfo {
			out = append(out, f.content)
		}
	}
	for _, m := range annotatedMathRe.FindAllStringSubmatch(plain, -1) {
		out = append(out, html.UnescapeString(m[1]))
	}
	return out
}

func normalizedMath(text string) string { return strings.Join(strings.Fields(text), " ") }

// referenceRe is a written cross-reference: pandoc names the label in
// data-reference as well as in the href, so where the link should land can be
// read back off the page rather than taken from the map that wrote it.
var referenceRe = regexp.MustCompile(`(?s)<a\b[^<>]*?\bhref="([^"]*)"[^<>]*?\bdata-reference="([^"]*)"`)

// checkAnchors is the anchor-landing check: every rewritten link lands on the
// document holding its label, and no label the map holds was left a bare
// fragment. Labels outside the map are stated, not gated.
func checkAnchors(stem string, t *volumeTree) []Finding {
	var findings []Finding
	rewritten := 0
	raw := map[string]bool{}
	for _, n := range t.index.walk() {
		for _, m := range referenceRe.FindAllStringSubmatch(n.body, -1) {
			destination, label := m[1], m[2]
			expected, inMap := t.labelPaths[label]
			if strings.HasPrefix(destination, "#") {
				if inMap {
					findings = append(findings, Finding{StatusFail, nameAnchor, fmt.Sprintf("%s %s: %q is in the label-to-node map (%s) but its link was left unrewritten", stem, n.path, label, expected)})
				} else {
					raw[label] = true
				}
				continue
			}
			rewritten++
			file, _, _ := strings.Cut(destination, "#")
			landed := path.Clean(path.Join(path.Dir(n.path), file))
			if !inMap || landed != expected {
				findings = append(findings, Finding{StatusFail, nameAnchor, fmt.Sprintf("%s %s: anchor for %q resolves to %s but the label sits in %q", stem, n.path, label, landed, expected)})
			}
		}
	}
	gated := len(findings) > 0
	if len(raw) > 0 {
		labels := make([]string, 0, len(raw))
		for l := range raw {
			labels = append(labels, l)
		}
		slices.Sort(labels)
		findings = append(findings, Finding{StatusFact, nameAnchor, fmt.Sprintf("%s: %d label(s) outside the map render as the raw label a reader can see — %s",
			stem, len(labels), strings.Join(labels[:min(len(labels), 6)], ", "))})
	}
	if gated {
		return findings
	}
	return append(findings, Finding{StatusPass, nameAnchor, fmt.Sprintf("%s %d rewritten anchors land on the node that held the label", stem, rewritten)})
}

// checkAssets is the image-asset report: every embedded image copied, and
// every one not on disk named. A missing image does not gate; the tree carries
// pandoc's caption and no link to it.
func checkAssets(stem string, t *volumeTree) []Finding {
	missing := slices.Sorted(slices.Values(t.missing))
	findings := make([]Finding, 0, len(missing)+1)
	for _, source := range missing {
		findings = append(findings, Finding{StatusFact, nameAsset, fmt.Sprintf("%s: %q is embedded by the source but is not on disk beside it — no image was copied and no self-linking form was constructed for it", stem, source)})
	}
	return append(findings, Finding{StatusPass, nameAsset, fmt.Sprintf("%s %d image asset(s) copied into the tree", stem, len(t.assets))})
}
