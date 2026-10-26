package index

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"kbase/internal/kb"
)

// excerptMaxChars bounds an authority citation's quoted excerpt.
const excerptMaxChars = 240

const linkText = `(?:[^\[\]]|\[[^\[\]]*\])*`

var (
	citationLinkRE  = regexp.MustCompile(`\[(` + linkText + `)\]\([` + kb.PyWhitespace + `]*([^)` + kb.PyWhitespace + `]+)[` + kb.PyWhitespace + `]*\)`)
	excerptRE       = kb.PyRE(`(?s)^\s*(?:"(.*)"|“(.*)”|‘(.*)’)\s*$`)
	invariantIDRE   = regexp.MustCompile(`INVARIANT-[A-Z]+[0-9]+`)
	plainAxiomRE    = regexp.MustCompile(`Axiom ` + kb.AxiomLabel)
	declarationRE   = kb.PyRE(`^###\s+Axiom\s+` + kb.AxiomLabel + `\s*:`)
	headingRE       = kb.PyRE(`^(#{1,6})\s`)
	markerCommentRE = kb.PyRE(`(?s)<!--\s*claim-rigor:.*?-->`)
	fieldBulletRE   = kb.PyRE(`^-\s+[a-z][a-z-]*:`)
	fieldContRE     = kb.PyRE(`^\s+\S`)
	schemeRE        = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`)
)

// frontmatterSpans is the document's frontmatter block as one span, or none.
func frontmatterSpans(text string) [][]int {
	if m := kb.FindFrontmatter(text); m != nil {
		return [][]int{m[:2]}
	}
	return nil
}

// blankSpans replaces every character but newlines in each span with a space.
func blankSpans(text string, spans [][]int) string {
	for i := len(spans) - 1; i >= 0; i-- {
		s := spans[i]
		text = text[:s[0]] + blankRunes(text[s[0]:s[1]]) + text[s[1]:]
	}
	return text
}

func blankRunes(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		return ' '
	}, s)
}

// inScopeText is the part of a document the citation checks read: a leaf
// keeps its frontmatter and statement markers only, every other document is read
// whole; code is blanked either way.
func inScopeText(text string, leaf bool) string {
	text = kb.StripCodeSplitLines(text)
	if !leaf {
		return text
	}
	fm := frontmatterSpans(text)
	keep := append(fm, markerCommentRE.FindAllStringIndex(text, -1)...)
	var b strings.Builder
	for i, r := range text {
		if r == '\n' || slices.ContainsFunc(keep, func(s []int) bool { return s[0] <= i && i < s[1] }) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// proseOnly is text with every sanctioned reference channel blanked: the
// frontmatter, markers, citation links, register field bullets and, in a
// declaration document, each declaration's own body.
func proseOnly(text string, declarations bool) string {
	text = blankSpans(text, frontmatterSpans(text))
	for _, re := range []*regexp.Regexp{markerCommentRE, citationLinkRE} {
		text = blankSpans(text, re.FindAllStringIndex(text, -1))
	}
	lines := strings.Split(text, "\n")
	inDeclaration, inField := false, false
	for i, line := range lines {
		if declarations {
			if declarationRE.MatchString(line) {
				inDeclaration = true
			} else if h := headingRE.FindStringSubmatch(line); h != nil && len(h[1]) <= 3 {
				inDeclaration = false
			}
		}
		if fieldBulletRE.MatchString(line) {
			inField = true
		} else if inField && !fieldContRE.MatchString(line) {
			inField = false
		}
		if inDeclaration || inField {
			lines[i] = strings.Repeat(" ", utf8.RuneCountInString(line))
		}
	}
	return strings.Join(lines, "\n")
}

// The citation-gate checks.
const (
	checkChannel  = "citation channel"
	checkReferent = "citation referent"
	checkExcerpt  = "citation excerpt"
	checkDurable  = "citation durable"
	checkEdge     = "citation edge"
)

func citation(check, rel string, line int, format string, args ...any) Finding {
	return Finding{Check: check, Path: rel, Line: line, Detail: fmt.Sprintf("%s:%d: %s", rel, line, fmt.Sprintf(format, args...))}
}

func channelFindings(rel, text string, declarations bool) []Finding {
	var out []Finding
	for n, line := range strings.Split(proseOnly(text, declarations), "\n") {
		for _, rule := range []struct {
			re     *regexp.Regexp
			remedy string
		}{
			{invariantIDRE, "invariant id in prose outside its declaration section"},
			{plainAxiomRE, "axiom token in prose outside its declaration section"},
			{nodeIDPattern, "bare claim/experiment/analysis id in prose"},
		} {
			for _, tok := range kb.WordBounded(rule.re, line) {
				out = append(out, citation(checkChannel, rel, n+1, "%q: %s", tok, rule.remedy))
			}
		}
	}
	return out
}

var nodeIDPattern = regexp.MustCompile(kb.IDBody())

func citationLinkFindings(src *kb.Source, rel, text, source string) ([]Finding, error) {
	root := kb.ResolvePath(src.Root())
	var out []Finding
	for n, line := range strings.Split(text, "\n") {
		for _, m := range citationLinkRE.FindAllStringSubmatch(line, -1) {
			linkTextValue, target := m[1], m[2]
			if schemeRE.MatchString(target) || strings.HasPrefix(target, "#") {
				continue
			}
			rawPath, anchor, _ := strings.Cut(target, "#")
			p := filepath.FromSlash(kb.StripTarget(rawPath))
			if !filepath.IsAbs(p) {
				p = filepath.Join(filepath.Dir(source), p)
			}
			resolved := kb.ResolvePath(p)
			if !kb.Within(root, resolved) {
				out = append(out, citation(checkDurable, rel, n+1, "citation target %q resolves outside %s/; cite a durable KB path", target, kb.KBDir))
				continue
			}
			if !src.IsFile(resolved) {
				out = append(out, citation(checkReferent, rel, n+1, "citation target %q does not resolve to a file", target))
				continue
			}
			em := excerptRE.FindStringSubmatchIndex(linkTextValue)
			if em == nil {
				continue
			}
			excerpt := ""
			for g := 1; g < len(em)/2; g++ {
				if em[2*g] >= 0 {
					excerpt = linkTextValue[em[2*g]:em[2*g+1]]
					break
				}
			}
			if kb.NormalizeSpace(excerpt) == "" {
				out = append(out, citation(checkExcerpt, rel, n+1, "citation of %q carries an empty quoted excerpt", target))
				continue
			}
			if strings.Contains(excerpt, "\n") || utf8.RuneCountInString(excerpt) > excerptMaxChars {
				out = append(out, citation(checkExcerpt, rel, n+1, "citation excerpt is %d chars; at most %d on one line", utf8.RuneCountInString(excerpt), excerptMaxChars))
				continue
			}
			if anchor == "" {
				out = append(out, citation(checkExcerpt, rel, n+1, "citation of %q carries a quoted excerpt but no #anchor", target))
				continue
			}
			cited, err := src.ReadText(resolved)
			if err != nil {
				return nil, err
			}
			body, ok := kb.AnchorSection(cited, anchor)
			switch {
			case !ok:
				out = append(out, citation(checkReferent, rel, n+1, "citation anchor %q not found in %s", "#"+anchor, rawPath))
			case !strings.Contains(kb.NormalizeSpace(body), kb.NormalizeSpace(excerpt)):
				out = append(out, citation(checkExcerpt, rel, n+1, "quoted excerpt does not appear at %q", target))
			}
		}
	}
	return out, nil
}

// domainOf is a node's top-level directory under kb-root, "" at the root.
func domainOf(canonicalPath string) string {
	parts := strings.Split(path.Clean(canonicalPath), "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

// foreignEdgeFindings is a finding per id a claim's, an analysis's or a work's
// rationale or strengthen-by item names in another domain than the node's own,
// where no edge has the named node as its source and the node as its target. A
// node whose record carries a no-edge reason is exempt, and a node with no
// canonical path has no domain.
func foreignEdgeFindings(x derivation) []Finding {
	domains := map[string]string{}
	for _, r := range x.rows.Nodes {
		if r.CanonicalPath != "" {
			domains[r.ID] = domainOf(r.CanonicalPath)
		}
	}
	edges := map[[2]string]bool{}
	for _, e := range x.edges {
		edges[[2]string{e.Target, e.Source}] = true
	}
	type prose struct {
		id, noEdge string
		texts      []string
	}
	var nodes []prose
	for _, n := range x.g.Claims {
		nodes = append(nodes, prose{n.ID, n.NoEdge, slices.Concat([]string{n.Rationale}, n.StrengthenBy)})
	}
	for _, n := range x.g.Analyses {
		nodes = append(nodes, prose{n.ID, n.NoEdge, []string{n.Rationale}})
	}
	for _, n := range x.g.Works {
		nodes = append(nodes, prose{id: n.ID, texts: []string{n.Rationale}})
	}
	var out []Finding
	for _, n := range nodes {
		own, ok := domains[n.id]
		if !ok || n.noEdge != "" {
			continue
		}
		for _, text := range n.texts {
			for _, ref := range kb.NodeIDs(text) {
				other, ok := domains[ref]
				if ref == n.id || !ok || other == own || edges[[2]string{n.id, ref}] {
					continue
				}
				out = append(out, Finding{Check: checkEdge, Path: kb.GraphNodesFile,
					Detail: fmt.Sprintf("%s: %s (domain %q) references foreign-domain %s (domain %q) with no premise edge", kb.GraphNodesFile, n.id, own, ref, other)})
			}
		}
	}
	return out
}

// CitationFindings is the citation-grammar gate over every authored document
// of the KB: ids and axiom tokens only in sanctioned channels, every citation
// link resolving inside kb-root to a file, every quoted excerpt short and
// present at its anchor, and every foreign-domain id a node record's prose
// names backed by a premise edge. Leaf bodies are out of scope, and register
// pages, which refresh renders from the records, are out of the channel check.
// A KB state Derive refuses is its refusal.
func CitationFindings(src *kb.Source) ([]Finding, error) {
	root := src.Root()
	x, err := loadDerivation(src)
	if err != nil {
		return nil, err
	}
	declarations, err := declarationDocument(src)
	if err != nil {
		return nil, err
	}
	pages := map[string]bool{}
	for _, p := range x.pagePositions() {
		pages[p] = true
	}
	files, err := kb.MarkdownFiles(src)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, file := range files {
		raw, err := src.ReadText(file)
		if err != nil {
			return nil, err
		}
		relPath, err := filepath.Rel(root, file)
		if err != nil {
			return nil, err
		}
		rel := filepath.ToSlash(relPath)
		text := inScopeText(raw, x.d.kinds[rel] == kb.DocumentLeaf)
		info, err := os.Stat(file)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if !pages[rel] {
			sanctioned := err == nil && declarations != nil && os.SameFile(info, declarations)
			out = append(out, channelFindings(rel, text, sanctioned)...)
		}
		links, err := citationLinkFindings(src, rel, text, file)
		if err != nil {
			return nil, err
		}
		out = append(out, links...)
	}
	return append(out, foreignEdgeFindings(x)...), nil
}

// declarationDocument is the axioms.md a KB declares its axioms in, whose
// declarations the citation gate sanctions; nil where the KB has none.
func declarationDocument(src *kb.Source) (fs.FileInfo, error) {
	info, err := os.Stat(src.KBPath(kb.AxiomsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return info, err
}
