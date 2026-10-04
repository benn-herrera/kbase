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
	citationLinkRE   = regexp.MustCompile(`\[(` + linkText + `)\]\([` + kb.PyWhitespace + `]*([^)` + kb.PyWhitespace + `]+)[` + kb.PyWhitespace + `]*\)`)
	excerptRE        = kb.PyRE(`(?s)^\s*(?:"(.*)"|“(.*)”|‘(.*)’)\s*$`)
	plainInvariantRE = regexp.MustCompile(`Invariant [0-9]+`)
	invariantIDRE    = regexp.MustCompile(`INVARIANT-[A-Z]+[0-9]+`)
	declarationRE    = kb.PyRE(`^###\s+INVARIANT-[A-Z]+[0-9]+\s*:`)
	headingRE        = kb.PyRE(`^(#{1,6})\s`)
	tier2MarkerRE    = kb.PyRE(`(?s)<!--\s*(?:claim-quality|id):.*?-->`)
	entryMarkerRE    = kb.PyRE(`<!--\s*id:\s*(` + kb.IDBody() + `)`)
	fieldBulletRE    = kb.PyRE(`^-\s+[a-z][a-z-]*:`)
	fieldContRE      = kb.PyRE(`^\s+\S`)
	noEdgeRE         = kb.PyRE(`^\s*-\s+no-edge:\s*(\S.*)$`)
	// citationFrontmatterRE is the block as the citation gate bounds it: the
	// opener word-bounded, the body up to the first closer.
	citationFrontmatterRE = kb.PyRE(`(?s)<!--\s*kb-frontmatter(?:-->|[^\p{L}\p{N}_].*?-->)`)
	leafKindRE            = kb.PyRE(`(?m)^kind:\s*(\S+)`)
	schemeRE              = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`)
)

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
// keeps its frontmatter and Tier-2 markers only, every other document is read
// whole; code is blanked either way.
func inScopeText(text string) string {
	text = kb.StripCodeSplitLines(text)
	fm := citationFrontmatterRE.FindStringIndex(text)
	kind := ""
	if fm != nil {
		if m := leafKindRE.FindStringSubmatch(text[fm[0]:fm[1]]); m != nil {
			kind = m[1]
		}
	}
	if kind != kb.DocumentLeaf {
		return text
	}
	keep := tier2MarkerRE.FindAllStringIndex(text, -1)
	keep = append([][]int{fm}, keep...)
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
// frontmatter, markers, citation links, register field bullets and, in the
// framework source, each INVARIANT section's own body.
func proseOnly(text string, declarations bool) string {
	for _, re := range []*regexp.Regexp{citationFrontmatterRE, tier2MarkerRE, citationLinkRE} {
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

// Citation-gate checks, as kb_tools names them.
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

func channelFindings(rel, text string, framework bool) []Finding {
	var out []Finding
	for n, line := range strings.Split(proseOnly(text, framework), "\n") {
		for _, rule := range []struct {
			re     *regexp.Regexp
			remedy string
		}{
			{plainInvariantRE, "plain-numbered invariant reference in prose"},
			{invariantIDRE, "invariant id in prose outside its declaration section"},
			{nodeIDPattern, "bare claim/experiment/support id in prose"},
		} {
			for _, tok := range kb.WordBounded(rule.re, line) {
				out = append(out, citation(checkChannel, rel, n+1, "%q: %s", tok, rule.remedy))
			}
		}
	}
	return out
}

var nodeIDPattern = regexp.MustCompile(kb.IDBody())

// resolveLenient is the absolute path p names with every symlink in its
// existing prefix resolved, as a non-strict resolve has it.
func resolveLenient(p string) string {
	p = filepath.Clean(p)
	var tail []string
	for {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(append([]string{r}, tail...)...)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Join(append([]string{p}, tail...)...)
		}
		tail = append([]string{filepath.Base(p)}, tail...)
		p = parent
	}
}

func citationLinkFindings(rel, text, kbRoot, source string) ([]Finding, error) {
	root := resolveLenient(kbRoot)
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
			resolved := resolveLenient(p)
			if !kb.Within(root, resolved) {
				out = append(out, citation(checkDurable, rel, n+1, "citation target %q resolves outside %s/; cite a durable KB path", target, kb.KBDir))
				continue
			}
			if !kb.IsFile(resolved) {
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
			cited, err := kb.ReadText(resolved)
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

func foreignEdgeFindings(rel, text string, domains map[string]string, edges map[[2]string]bool) []Finding {
	var out []Finding
	entry, exempt := "", false
	for n, line := range strings.Split(text, "\n") {
		if m := entryMarkerRE.FindStringSubmatch(line); m != nil {
			entry, exempt = m[1], false
			continue
		}
		if entry == "" {
			continue
		}
		if noEdgeRE.MatchString(line) {
			exempt = true
			continue
		}
		own, ok := domains[entry]
		if !ok || exempt {
			continue
		}
		for _, ref := range kb.NodeIDs(line) {
			if ref == entry {
				continue
			}
			other, ok := domains[ref]
			if !ok || other == own || edges[[2]string{entry, ref}] {
				continue
			}
			out = append(out, citation(checkEdge, rel, n+1, "%s (domain %q) references foreign-domain %s (domain %q) with no depends-on edge", entry, own, ref, other))
		}
	}
	return out
}

// indexMaps is the node domains and the (source, target) edges the index on
// disk records.
func indexMaps(indexDir string) (map[string]string, map[[2]string]bool, error) {
	domains := map[string]string{}
	edges := map[[2]string]bool{}
	read := func(name string, each func(map[string]any)) error {
		p := filepath.Join(indexDir, name)
		if !kb.IsFile(p) {
			return nil
		}
		text, err := kb.ReadText(p)
		if err != nil {
			return err
		}
		for _, raw := range kb.SplitLines(text) {
			if kb.Strip(raw) == "" {
				continue
			}
			if v, ok := parseJSON(raw); ok {
				if rec, ok := v.(map[string]any); ok {
					each(rec)
				}
			}
		}
		return nil
	}
	err := read("claims.jsonl", func(rec map[string]any) {
		id, okID := rec["id"].(string)
		p, okPath := rec["canonical_path"].(string)
		if okID && okPath {
			domains[id] = domainOf(p)
		}
	})
	if err != nil {
		return nil, nil, err
	}
	err = read("depends-on.jsonl", func(rec map[string]any) {
		s, okS := rec["source"].(string)
		t, okT := rec["target"].(string)
		if okS && okT {
			edges[[2]string{s, t}] = true
		}
	})
	return domains, edges, err
}

// CitationFindings is the citation-grammar gate over every authored document
// under kbRoot: ids and invariant names only in sanctioned channels, every
// citation link resolving inside kb-root to a file, every quoted excerpt
// short and present at its anchor, and every foreign-domain id in a register
// entry backed by a depends-on edge. Leaf bodies are out of scope.
func CitationFindings(kbRoot string) ([]Finding, error) {
	root, err := filepath.Abs(kbRoot)
	if err != nil {
		return nil, err
	}
	framework := kb.FrameworkSource(root)
	var frameworkInfo fs.FileInfo
	if framework != "" {
		if frameworkInfo, err = os.Stat(framework); err != nil {
			return nil, err
		}
	}
	domains, edges, err := indexMaps(filepath.Join(root, kb.IndexDir))
	if err != nil {
		return nil, err
	}
	files, err := kb.MarkdownFiles(root)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, file := range files {
		raw, err := kb.ReadText(file)
		if err != nil {
			return nil, err
		}
		text := inScopeText(raw)
		relPath, err := filepath.Rel(root, file)
		if err != nil {
			return nil, err
		}
		rel := filepath.ToSlash(relPath)
		isFramework := false
		if frameworkInfo != nil {
			info, err := os.Stat(file)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
			isFramework = err == nil && os.SameFile(info, frameworkInfo)
		}
		out = append(out, channelFindings(rel, text, isFramework)...)
		links, err := citationLinkFindings(rel, text, root, file)
		if err != nil {
			return nil, err
		}
		out = append(out, links...)
		if filepath.Base(file) == kb.RegisterFile {
			out = append(out, foreignEdgeFindings(rel, text, domains, edges)...)
		}
	}
	return out, nil
}
