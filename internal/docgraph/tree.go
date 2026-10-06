package docgraph

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/kb"
)

// Names the tree carries, as kb_tools' document graph spells them.
const (
	entryPointTitle = "Knowledge Base"
	assetDir        = "assets"
	// referencesTitle heads the leaf citeproc's reference list is lifted
	// into, as kb.OwnProseTitle heads a container's own prose. Neither has a
	// source heading behind it.
	referencesTitle = "References"
)

var (
	// bibliographyRe is citeproc's reference-list Div, keyed on the marker
	// citeproc itself puts on it.
	bibliographyRe = regexp.MustCompile(`^<div\b[^<>]*\bid="refs"[^<>]*\bclass="[^"]*\bcsl-bib-body\b[^"]*"[^<>]*>$`)
	divOpenRe      = regexp.MustCompile(`^<div\b`)
	divCloseRe     = regexp.MustCompile(`^</div>$`)
	divIDRe        = regexp.MustCompile(`(?m)^<div\b[^<>]*\bid="([^"]*)"`)
	// hrefRe is a cross-reference's own href; rewriting it leaves the
	// reader-visible text exactly as pandoc rendered it.
	hrefRe = regexp.MustCompile(`(<a\b[^<>]*?\bhref=")#([^"]*)(")`)
	// embedRe is the element pandoc emits for an external image.
	embedRe = regexp.MustCompile(`<(?:embed|img)\b[^<>]*?\bsrc="([^"]+)"[^<>]*/?>`)
)

// document is one node of the tree.
type document struct {
	title    string
	rank     int
	segment  string
	parent   *document
	children []*document
	path     string
	body     string
	label    string
}

func (d *document) walk() []*document {
	out := []*document{d}
	for _, c := range d.children {
		out = append(out, c.walk()...)
	}
	return out
}

func (d *document) depth() int {
	n := 0
	for p := d.parent; p != nil; p = p.parent {
		n++
	}
	return n
}

// volumeTree is one volume's documents and what placing records against them
// needs.
type volumeTree struct {
	title string
	// volumeDir is the volume root's directory, which its assets are read
	// from.
	volumeDir  string
	index      *document
	ordered    []*document
	references *document
	// holders maps a container's path to the leaf its own prose was lifted
	// into.
	holders map[string]string
	// labelPaths maps a label to the document holding it, and anchors a
	// label to the fragment that lands on it there.
	labelPaths map[string]string
	anchors    map[string]string
	assets     map[string]string
	missing    []string
	// contentTokens are the volume's words as the split must account for
	// them: every title the build supplies rather than reads, once per
	// segment carrying it, then the abstract and the rendering's body.
	contentTokens []string
}

// AlignmentError is the AST's headers and the rendering's headings
// disagreeing: both come from one source, so a difference is a defect.
type AlignmentError struct{ Detail string }

func (e AlignmentError) Error() string {
	return "AST headers and rendered headings disagree: " + e.Detail
}

// DepthError is a tree whose realized depth is not its count of distinct
// heading levels.
type DepthError struct {
	Realized int
	Levels   []int
}

func (e DepthError) Error() string {
	return fmt.Sprintf("tree is %d level(s) deep but the volume uses %d distinct heading levels %v",
		e.Realized, len(e.Levels), e.Levels)
}

type frontmatter struct {
	title, abstract, body string
}

// frontmatterKeyRe is a top-level key of the metadata block.
var frontmatterKeyRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*):(.*)$`)

// parseFrontmatter splits the metadata block -s puts at the head of the
// rendering off the body, reads each key's value as text, and classifies the
// keys it carries: an entry the writer dropped is no key here. Every value is
// read as kb_tools reads it, line by line rather than as typed YAML, so a title
// that would not parse back as one string is still its text.
func parseFrontmatter(markdown string) (frontmatter, error) {
	lines := kb.Lines(markdown)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{body: markdown}, nil
	}
	closing := -1
	for n := 1; n < len(lines); n++ {
		if strings.TrimSpace(lines[n]) == "---" {
			closing = n
			break
		}
	}
	if closing < 0 {
		return frontmatter{body: markdown}, nil
	}
	values := map[string]string{}
	key := ""
	var collected []string
	for _, line := range lines[1:closing] {
		if m := frontmatterKeyRe.FindStringSubmatch(line); m != nil {
			if key != "" {
				values[key] = unquoteYAML(flattenYAML(collected))
			}
			key, collected = m[1], []string{m[2]}
		} else if key != "" {
			collected = append(collected, line)
		}
	}
	if key != "" {
		values[key] = unquoteYAML(flattenYAML(collected))
	}
	var unknown []string
	for k := range values {
		if !contentKeys[k] && !apparatusKeys[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return frontmatter{}, MetadataKeyError{Keys: unknown}
	}
	return frontmatter{
		title:    strings.Join(strings.Fields(values["title"]), " "),
		abstract: values["abstract"],
		body:     strings.TrimLeft(strings.Join(lines[closing+1:], "\n"), "\n"),
	}, nil
}

// flattenYAML is one entry's value as text: a block scalar's header dropped,
// every line trimmed, list dashes and trailing backslashes off, empty lines
// gone.
func flattenYAML(collected []string) string {
	first := strings.TrimSpace(collected[0])
	switch first {
	case "|", ">", "|-", ">-":
		first = ""
	}
	parts := []string{first}
	for _, line := range collected[1:] {
		parts = append(parts, strings.TrimRight(strings.TrimPrefix(strings.TrimSpace(line), "- "), `\`))
	}
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n")
}

// yamlEscapes are the escapes an emitter writes in a double-quoted scalar; any
// other keeps its backslash.
var (
	yamlEscapes = map[string]string{
		`\`: `\`, `"`: `"`, "/": "/", "n": "\n", "t": "\t", "r": "\r", "b": "\b", "f": "\f", "0": "\x00", " ": " ",
	}
	yamlEscapeRe = regexp.MustCompile(`(?s)\\(u[0-9a-fA-F]{4}|x[0-9a-fA-F]{2}|.)`)
)

// unquoteYAML is a scalar as its own text, whichever quoting the emitter
// chose: double-quoted with escapes, single-quoted with ” for a quote, or
// plain. A value whose ends disagree is left as it stands.
func unquoteYAML(v string) string {
	if len(v) < 2 || v[0] != v[len(v)-1] {
		return v
	}
	switch v[0] {
	case '"':
		return yamlEscapeRe.ReplaceAllStringFunc(v[1:len(v)-1], func(m string) string {
			esc := m[1:]
			if (esc[0] == 'u' || esc[0] == 'x') && len(esc) > 1 {
				if n, err := strconv.ParseUint(esc[1:], 16, 32); err == nil {
					return string(rune(n))
				}
			}
			if r, ok := yamlEscapes[esc]; ok {
				return r
			}
			return m
		})
	case '\'':
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	return v
}

// splitBibliography lifts citeproc's reference list out of the body: citeproc
// appends it at the end, where a cut at headings would leave it inside
// whatever section came last.
func splitBibliography(body string) (string, string, error) {
	lines := kb.Lines(body)
	start := slices.IndexFunc(lines, bibliographyRe.MatchString)
	if start < 0 {
		return body, "", nil
	}
	depth := 0
	for end := start; end < len(lines); end++ {
		switch {
		case divOpenRe.MatchString(lines[end]):
			depth++
		case divCloseRe.MatchString(lines[end]):
			depth--
			if depth == 0 {
				rest := append(slices.Clone(lines[:start]), lines[end+1:]...)
				return strings.Join(rest, "\n"), strings.Join(lines[start:end+1], "\n"), nil
			}
		}
	}
	return "", "", fmt.Errorf("the bibliography Div opens at line %d of the rendering and never closes", start+1)
}

// buildTree cuts the whole-volume rendering at its headings. The AST supplies
// each node's level and label, the rendering its content; nothing is
// re-rendered from an AST slice, so numbering and citations stay computed
// across the document.
func buildTree(stem, markdown string, o outline, volumeDir string) (*volumeTree, error) {
	fm, err := parseFrontmatter(markdown)
	if err != nil {
		return nil, err
	}
	content, bibliography, err := splitBibliography(fm.body)
	if err != nil {
		return nil, err
	}
	lines := kb.Lines(content)
	found := headings(lines)
	if err := assertAligned(o.headers, found); err != nil {
		return nil, err
	}

	title := fm.title
	if title == "" {
		title = stem
	}
	var levels []int
	for _, h := range o.headers {
		if !slices.Contains(levels, h.level) {
			levels = append(levels, h.level)
		}
	}
	slices.Sort(levels)
	rank := map[int]int{}
	for i, l := range levels {
		rank[l] = i + 1
	}

	preambleEnd := len(lines)
	if len(found) > 0 {
		preambleEnd = found[0].start
	}
	var head []string
	for _, part := range []string{"# " + title, strings.TrimSpace(fm.abstract), strings.TrimSpace(strings.Join(lines[:preambleEnd], "\n"))} {
		if part != "" {
			head = append(head, part)
		}
	}
	index := &document{title: title, segment: strings.Join(head, "\n\n")}
	stack := []*document{index}
	t := &volumeTree{title: title, volumeDir: volumeDir, index: index}
	for i, h := range o.headers {
		hd := found[i]
		stop := len(lines)
		if i+1 < len(found) {
			stop = found[i+1].start
		}
		r := rank[h.level]
		stack = stack[:min(r, len(stack))]
		parent := stack[len(stack)-1]
		n := &document{
			title:   hd.text,
			rank:    r,
			segment: strings.Join(append([]string{"# " + hd.text}, lines[hd.end:stop]...), "\n"),
			parent:  parent,
			label:   h.identifier,
		}
		parent.children = append(parent.children, n)
		stack = append(stack, n)
		t.ordered = append(t.ordered, n)
	}

	if bibliography != "" {
		t.references = &document{title: referencesTitle, rank: 1, segment: "# " + referencesTitle + "\n\n" + bibliography, parent: index}
		index.children = append(index.children, t.references)
	}
	ownProse := liftOwnProse(index)
	supplied := []string{title}
	if bibliography != "" {
		supplied = append(supplied, referencesTitle)
	}
	for range ownProse {
		supplied = append(supplied, kb.OwnProseTitle)
	}
	for _, s := range append(supplied, fm.abstract, fm.body) {
		t.contentTokens = append(t.contentTokens, markdownTokens(s)...)
	}

	directory := slug(plain(title))
	index.path = directory + "/" + kb.IndexFile
	placeChildren(index, directory)

	realized := 0
	for _, n := range t.ordered {
		realized = max(realized, n.depth())
	}
	if realized != len(levels) {
		return nil, DepthError{Realized: realized, Levels: levels}
	}

	// A heading's label stays with the heading, which stays with its
	// container; every other label a section declares before its first
	// subsection sits in the prose and follows it to the lifted leaf.
	t.holders = map[string]string{}
	for _, leaf := range ownProse {
		t.holders[leaf.parent.path] = leaf.path
	}
	labelPaths := map[string]string{}
	t.labelPaths = labelPaths
	for label, section := range o.labelSection {
		declaredIn := t.sectionPath(section)
		if o.headerLabels[label] {
			labelPaths[label] = declaredIn
		} else {
			labelPaths[label] = t.holder(declaredIn)
		}
	}
	if t.references != nil {
		for _, m := range divIDRe.FindAllStringSubmatch(bibliography, -1) {
			labelPaths[m[1]] = t.references.path
		}
	}
	anchors := map[string]string{}
	t.anchors = anchors
	for label := range o.anchoredLabels {
		anchors[label] = label
	}
	for _, n := range t.ordered {
		if n.label != "" {
			anchors[n.label] = githubAnchor(n.title)
		}
	}
	t.assets, t.missing = collectAssets(index, volumeDir)
	for _, n := range index.walk() {
		n.body = t.rewriteBody(n, directory)
	}
	return t, nil
}

// sectionPath is the document a section's own heading landed in; -1 is the
// material before the first heading.
func (t *volumeTree) sectionPath(section int) string {
	if section < 0 {
		return t.index.path
	}
	return t.ordered[section].path
}

// holder is where the prose of the document at p landed.
func (t *volumeTree) holder(p string) string {
	if h, ok := t.holders[p]; ok {
		return h
	}
	return p
}

func assertAligned(headers []header, found []heading) error {
	if len(headers) != len(found) {
		return AlignmentError{fmt.Sprintf("the AST holds %d headers and the rendering %d headings", len(headers), len(found))}
	}
	for i, h := range headers {
		if h.level != found[i].level {
			return AlignmentError{fmt.Sprintf("header %d is level %d in the AST and %d in the rendering", i, h.level, found[i].level)}
		}
		if ast, rendered := h.tokens(), inlineMarkdownTokens(found[i].text); !slices.Equal(ast, rendered) {
			return AlignmentError{fmt.Sprintf("header %d reads %q in the AST and %q in the rendering", i, ast, rendered)}
		}
	}
	return nil
}

// liftOwnProse gives the prose below every container's heading to a leaf of
// its own, first among its children; the container keeps its heading.
func liftOwnProse(n *document) []*document {
	var lifted []*document
	for _, c := range n.children {
		lifted = append(lifted, liftOwnProse(c)...)
	}
	if len(n.children) == 0 {
		return lifted
	}
	heading, prose, _ := strings.Cut(n.segment, "\n")
	if strings.TrimSpace(prose) == "" {
		return lifted
	}
	leaf := &document{title: kb.OwnProseTitle, rank: n.rank + 1, segment: "# " + kb.OwnProseTitle + "\n" + prose, parent: n}
	n.children = append([]*document{leaf}, n.children...)
	n.segment = heading
	return append(lifted, leaf)
}

func placeChildren(parent *document, directory string) {
	taken := map[string]bool{}
	for i, c := range parent.children {
		segment := distinct(slug(plain(c.title)), taken, i+1)
		if len(c.children) > 0 {
			c.path = directory + "/" + segment + "/" + kb.IndexFile
			placeChildren(c, directory+"/"+segment)
		} else {
			c.path = directory + "/" + segment + ".md"
		}
	}
}

// distinct is the first segment neither reserved nor taken in this
// directory; the ordinal disambiguates first, since it says which came first.
func distinct(base string, taken map[string]bool, ordinal int) string {
	candidate := base
	for n := 1; reserved(candidate) || taken[candidate]; n++ {
		if n == 1 {
			candidate = fmt.Sprintf("%s-%d", base, ordinal)
		} else {
			candidate = fmt.Sprintf("%s-%d-%d", base, ordinal, n)
		}
	}
	taken[candidate] = true
	return candidate
}

func reserved(c string) bool {
	return c == strings.TrimSuffix(kb.IndexFile, ".md") || kb.ExcludeDirs[c] || kb.ExcludeNames[c] || kb.ExcludeNames[c+".md"]
}

// collectAssets maps every external image the volume embeds to its name in
// the tree; an image not on disk is recorded rather than linked.
func collectAssets(index *document, volumeDir string) (map[string]string, []string) {
	assets := map[string]string{}
	var missing []string
	taken := map[string]bool{}
	for _, n := range index.walk() {
		for _, m := range embedRe.FindAllStringSubmatch(n.segment, -1) {
			source := m[1]
			if _, ok := assets[source]; ok || slices.Contains(missing, source) {
				continue
			}
			if info, err := os.Stat(filepath.Join(volumeDir, filepath.FromSlash(source))); err != nil || !info.Mode().IsRegular() {
				missing = append(missing, source)
				continue
			}
			name := path.Base(source)
			ext := path.Ext(source)
			for taken[name] {
				name = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(path.Base(source), ext), len(taken), ext)
			}
			taken[name] = true
			assets[source] = name
		}
	}
	return assets, missing
}

// href is the href a link to #label is written with in the document at
// source, and the document it names: the document holding the label, or none
// where the tree holds no such label and the link stays a bare fragment.
func (t *volumeTree) href(source, label string) (string, string) {
	target, ok := t.labelPaths[label]
	if !ok {
		return "#" + label, ""
	}
	fragment := ""
	if a, ok := t.anchors[label]; ok {
		fragment = "#" + a
	}
	return relative(source, target) + fragment, target
}

// rewriteBody resolves each cross-reference's href to the document holding
// its label, and constructs a self-linking image beside each embed. Both only
// add; a label the map does not hold is left as pandoc wrote it.
func (t *volumeTree) rewriteBody(n *document, directory string) string {
	body := hrefRe.ReplaceAllStringFunc(n.segment, func(m string) string {
		g := hrefRe.FindStringSubmatch(m)
		href, _ := t.href(n.path, g[2])
		return g[1] + href + g[3]
	})
	return embedRe.ReplaceAllStringFunc(body, func(m string) string {
		name, ok := t.assets[embedRe.FindStringSubmatch(m)[1]]
		if !ok {
			return m
		}
		target := relative(n.path, directory+"/"+assetDir+"/"+name)
		return m + "\n\n[![](" + target + ")](" + target + ")"
	})
}

// relative is target as a link from the document at source spells it.
func relative(source, target string) string {
	from := strings.Split(path.Dir(source), "/")
	to := strings.Split(target, "/")
	if from[0] == "." {
		from = nil
	}
	common := 0
	for common < len(from) && common < len(to)-1 && from[common] == to[common] {
		common++
	}
	parts := slices.Repeat([]string{".."}, len(from)-common)
	return strings.Join(append(parts, to[common:]...), "/")
}

// render is the document as it lands on disk: up-link, body, child list.
func render(n *document) string {
	parentPath, parentTitle := kb.EntryPointFile, entryPointTitle
	if n.parent != nil {
		parentPath, parentTitle = n.parent.path, n.parent.title
	}
	parts := []string{fmt.Sprintf("[%s %s](%s)", kb.UplinkMarker, parentTitle, relative(n.path, parentPath))}
	if body := strings.TrimSpace(n.body); body != "" {
		parts = append(parts, body)
	}
	if len(n.children) > 0 {
		items := make([]string, len(n.children))
		for i, c := range n.children {
			items[i] = fmt.Sprintf("- [%s](%s)", c.title, relative(n.path, c.path))
		}
		parts = append(parts, strings.Join(items, "\n"))
	}
	return strings.Join(parts, "\n\n") + "\n"
}

// writeTrees writes each volume's tree and the entry point listing every
// volume, in order.
func writeTrees(trees []*volumeTree, kbRoot string) error {
	listing := make([]string, len(trees))
	for i, t := range trees {
		if err := writeTree(t, kbRoot); err != nil {
			return err
		}
		listing[i] = fmt.Sprintf("- [%s](%s)", t.title, t.index.path)
	}
	entry := fmt.Sprintf("# %s\n\n%s\n", entryPointTitle, strings.Join(listing, "\n"))
	return writeFile(filepath.Join(kbRoot, kb.EntryPointFile), []byte(entry))
}

// writeTree writes the volume's documents and its copied assets.
func writeTree(t *volumeTree, kbRoot string) error {
	for _, n := range t.index.walk() {
		if err := writeFile(filepath.Join(kbRoot, filepath.FromSlash(n.path)), []byte(render(n))); err != nil {
			return err
		}
	}
	sources := make([]string, 0, len(t.assets))
	for s := range t.assets {
		sources = append(sources, s)
	}
	slices.Sort(sources)
	for _, s := range sources {
		data, err := os.ReadFile(filepath.Join(t.volumeDir, filepath.FromSlash(s)))
		if err != nil {
			return fmt.Errorf("copying asset %s: %w", s, err)
		}
		dest := filepath.Join(kbRoot, filepath.FromSlash(path.Dir(t.index.path)), assetDir, t.assets[s])
		if err := writeFile(dest, data); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", p, err)
	}
	return nil
}
