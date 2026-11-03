package from300to400

import (
	"fmt"
	"maps"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format200"
	"kbase/internal/formats/format300"
	"kbase/internal/formats/format400"
)

var (
	// unlinkedRE is a destination naming no KB document: a URL scheme, a
	// rooted or a home path.
	unlinkedRE = regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9+.\-]*:|[/~])`)
	// lineSuffixRE is the :line a destination may carry after its path.
	lineSuffixRE = regexp.MustCompile(`:\d+$`)
	// axiomHeadingRE is an axiom's heading in axioms.md as 3.0.0 writes it;
	// group 1 its label.
	axiomHeadingRE = format400.PyRE(`^### Axiom (` + format300.AxiomLabel + `):`)
	// markerIDRE is a hosted node's id as a 4.0.0 statement marker names it.
	markerIDRE = regexp.MustCompile(format400.IDBody(format400.HostedIDKinds...))
	// axiomMarkerRE is an axiom's id as a 4.0.0 marker names it.
	axiomMarkerRE = regexp.MustCompile(format400.IDBody(format400.AxiomPrefix))
)

// heading is one heading of a document: its 0-based line, and its anchor
// under 3.0.0's slugging and under 4.0.0's.
type heading struct {
	line        int
	old, anchor string
}

// relinker is what the citations' step reads of a KB: each document's lines
// in the 4.0.0 marker form, by kb-root-relative path, and its headings; each
// node's title; each hosted node's host and its section's heading line; the
// volumes; each 3.0.0 register page's entries; each work's anchor on the
// root summary page, where no other heading there can take it; each axiom's
// id by its label; and each work's and axiom's minted id by its 3.0.0 id.
type relinker struct {
	docs     map[string][]string
	headings map[string][]heading
	titles   map[string]string
	host     map[string]string
	section  map[string]int
	volumes  map[string]bool
	entries  map[string][]string
	works    map[string]string
	axioms   map[string]string
	minted   map[string]string
}

// convertCitations is out's kb-root documents re-linked and their bare ids
// cited, files holding their 3.0.0 text and out their text with the markers
// converted, and its warnings in path and line order. Every link into a
// heading is re-anchored to the heading's 4.0.0 anchor, the heading its anchor
// names under 3.0.0's slugging; one into a 3.0.0 register page is pointed at
// its entries' place, else at the summary page replacing it, and loses its
// link where no page does. A link resolving to one node's statement takes the
// node's id as its title. Each bare id of a node, a 3.0.0 work's or axiom's
// read as its minted id, is replaced by the node's citation in the naming
// form. Each axiom heading of axioms.md naming an axiom record ends in its
// marker; one naming none is reported. A summary page is kbase's rendering and
// is not read.
func convertCitations(files, out common.Files, minted map[string]string) []common.Warning {
	r := newRelinker(files, out, minted)
	var warnings []common.Warning
	for _, doc := range slices.Sorted(maps.Keys(r.docs)) {
		lines := r.docs[doc]
		view := codeBlanked(lines)
		changed := false
		for n := range lines {
			line, found := r.convertLine(doc, n, lines[n], view[n])
			warnings = append(warnings, found...)
			changed = changed || line != lines[n]
			lines[n] = line
		}
		if changed {
			out[format200.KBDir+"/"+doc] = []byte(strings.Join(lines, "\n"))
		}
	}
	return warnings
}

// newRelinker reads files' and out's kb-root documents and out's node records.
func newRelinker(files, out common.Files, minted map[string]string) *relinker {
	r := &relinker{docs: map[string][]string{}, headings: map[string][]heading{}, titles: map[string]string{},
		host: map[string]string{}, section: map[string]int{}, entries: map[string][]string{}, works: map[string]string{},
		axioms: map[string]string{}, minted: minted}
	var g format400.Graph
	if data, ok := out[nodesPath]; ok {
		format400.DecodeNodes(data, format400.NodesFile, &g)
	}
	for p, data := range out {
		rel, inKB := strings.CutPrefix(p, format200.KBDir+"/")
		if inKB && strings.HasSuffix(rel, ".md") && !pageAt(rel, format400.SummaryFile) {
			r.docs[rel] = strings.Split(string(data), "\n")
			r.headings[rel] = docHeadings(strings.Split(string(files[p]), "\n"), r.docs[rel])
		}
	}
	for _, doc := range slices.Sorted(maps.Keys(r.docs)) {
		for n, line := range r.docs[doc] {
			for _, m := range format400.StatementMarkerRE.FindAllStringSubmatch(line, -1) {
				for _, id := range markerIDRE.FindAllString(m[1], -1) {
					if _, seen := r.host[id]; !seen {
						r.host[id], r.section[id] = doc, sectionLine(r.headings[doc], n)
					}
				}
			}
		}
	}
	r.volumes = format400.Volumes(slices.Collect(maps.Keys(r.docs)))
	var rootHeadings []string
	add := func(id, title string, derived bool) {
		r.titles[id] = title
		if _, hosted := r.host[id]; !hosted {
			return
		}
		page := format400.SummaryPage(r.host[id], r.volumes)
		register := registerReplacedBy(page)
		r.entries[register] = append(r.entries[register], id)
		if page == format400.SummaryFile {
			rootHeadings = append(rootHeadings, format400.SummaryEntryHeadings(title, derived)...)
		}
	}
	for _, n := range g.Claims {
		add(n.ID, n.Title, true)
	}
	for _, n := range g.Analyses {
		add(n.ID, n.Title, true)
	}
	for _, n := range g.Experiments {
		add(n.ID, n.Title, false)
	}
	for _, n := range g.Axioms {
		r.titles[n.ID] = n.Title
		r.axioms[n.Label] = n.ID
	}
	for _, n := range g.Works {
		r.titles[n.ID] = n.Title
		r.entries[format200.RegisterFile] = append(r.entries[format200.RegisterFile], n.ID)
	}
	for _, w := range g.Works {
		others := append([]string{format400.SummaryTitle}, rootHeadings...)
		for _, o := range g.Works {
			if o.ID != w.ID {
				others = append(others, format400.SummaryEntryHeadings(o.Title, false)...)
			}
		}
		if anchor := format400.Slugify(w.Title); !takenBy(anchor, others) {
			r.works[w.ID] = anchor
		}
	}
	r.anchorHeadings()
	return r
}

// anchorHeadings anchors every heading under 4.0.0's slugging as it reads
// converted, its bare ids cited. The text a heading renders does not depend
// on any anchor, since a citation renders as its link text, so converting it
// before the anchors stand reads the same text as the conversion writes.
func (r *relinker) anchorHeadings() {
	anchors := map[string][]string{}
	for doc, hs := range r.headings {
		lines, view := r.docs[doc], codeBlanked(r.docs[doc])
		var s format400.Slugger
		for _, h := range hs {
			converted, _ := r.convertLine(doc, h.line, lines[h.line], view[h.line])
			anchors[doc] = append(anchors[doc], s.Slug(format200.FirstHeading(converted)))
		}
	}
	for doc, as := range anchors {
		for i, a := range as {
			r.headings[doc][i].anchor = a
		}
	}
}

// takenBy is whether a heading among others, in any order before the one
// anchor is the slug of, can take anchor: one slugging to it, or enough
// repeats of one slugging to its stem that a suffix reaches it.
func takenBy(anchor string, others []string) bool {
	count := map[string]int{}
	for _, o := range others {
		count[format400.Slugify(o)]++
	}
	if count[anchor] > 0 {
		return true
	}
	if i := strings.LastIndex(anchor, "-"); i >= 0 {
		n, err := strconv.Atoi(anchor[i+1:])
		return err == nil && n >= 1 && count[anchor[:i]] > n
	}
	return false
}

// docHeadings is every heading of a document, by its lines in the 4.0.0
// marker form, each anchored under 3.0.0's slugging in old, its 3.0.0 lines:
// the marker conversion moves no heading, so the nth heading of each is the
// same heading. The 4.0.0 anchors are anchorHeadings'.
func docHeadings(old, lines []string) []heading {
	oldAt := headingLines(old)
	var out []heading
	for i, n := range headingLines(lines) {
		h := heading{line: n}
		if i < len(oldAt) {
			h.old = format200.Slugify(format200.FirstHeading(old[oldAt[i]]))
		}
		out = append(out, h)
	}
	return out
}

// codeBlanked is lines with every fenced block's lines empty and every inline
// code and maths span blanked.
func codeBlanked(lines []string) []string {
	return strings.Split(format400.BlankInlineCode(strings.Join(blankFences(lines), "\n")), "\n")
}

// headingLines is the 0-based line of every heading of lines outside a fence.
func headingLines(lines []string) []int {
	var out []int
	for n, line := range blankFences(lines) {
		if format200.FirstHeading(line) != "" {
			out = append(out, n)
		}
	}
	return out
}

// blankFences is lines with every fenced block's lines empty and every line's
// carriage return dropped.
func blankFences(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimSuffix(line, "\r")
	}
	for _, f := range format200.Fences(slices.Clone(out)) {
		for i := f.Start; i < f.End; i++ {
			out[i] = ""
		}
	}
	return out
}

// sectionLine is the line of the nearest heading at or above line, -1 where
// none stands above it.
func sectionLine(hs []heading, line int) int {
	at := -1
	for _, h := range hs {
		if h.line <= line {
			at = h.line
		}
	}
	return at
}

// pageAt is whether rel is a page named name at kb-root or directly inside
// one of its directories.
func pageAt(rel, name string) bool {
	return path.Base(rel) == name && strings.Count(rel, "/") <= 1
}

// target is node id's citation target: the kb-root-relative document and
// the anchor in it; ok is false where the node has none — a hosted kind with
// no marker, an axiom with no heading in axioms.md, a work whose anchor on the
// root summary page another heading there can take.
func (r *relinker) target(id string) (doc, anchor string, ok bool) {
	kind, _ := format400.NodeIDKind(id)
	switch kind {
	case format400.NodeKindWork:
		anchor, ok = r.works[id]
		return format400.SummaryFile, anchor, ok
	case format400.NodeKindAxiom:
		for _, h := range r.headings[format200.AxiomsFile] {
			if r.headingAxiom(h.line) == id {
				return format200.AxiomsFile, h.anchor, true
			}
		}
		return "", "", false
	}
	doc, ok = r.host[id]
	if !ok {
		return "", "", false
	}
	for _, h := range r.headings[doc] {
		if h.line == r.section[id] {
			anchor = h.anchor
		}
	}
	return doc, anchor, true
}

// destination is the link from document from to anchor in document doc.
func destination(from, doc, anchor string) string {
	dest := relPath(doc, path.Dir(from))
	if anchor != "" {
		dest += "#" + anchor
	}
	return dest
}

// sectionNodes is every node whose statement stands in the section of the
// heading at line of doc: the hosted nodes marked there, and the axiom whose
// heading it is.
func (r *relinker) sectionNodes(doc string, line int) []string {
	var out []string
	for id, host := range r.host {
		if host == doc && r.section[id] == line {
			out = append(out, id)
		}
	}
	if doc == format200.AxiomsFile {
		if id := r.headingAxiom(line); id != "" {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// headingAxiom is the id of the axiom record whose label the axiom heading at
// line of axioms.md names, "" where the line is no axiom heading or names no
// record.
func (r *relinker) headingAxiom(line int) string {
	if m := axiomHeadingRE.FindStringSubmatch(r.docs[format200.AxiomsFile][line]); m != nil {
		return r.axioms[m[1]]
	}
	return ""
}

// relink is link l of document doc re-pointed: its new destination, "" where
// it stays as written; the node it cites, "" where none; where it cannot be
// resolved to one node without guessing, why; and whether the link goes,
// its text standing alone, because the page it points into goes with no page
// in its place.
func (r *relinker) relink(doc string, l format400.Link) (dest, id, unresolved string, unlink bool) {
	raw := l.Destination
	angled := strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">")
	if angled {
		raw = raw[1 : len(raw)-1]
	}
	if raw == "" || unlinkedRE.MatchString(raw) {
		return "", "", "", false
	}
	rawPath, fragment, hasFragment := strings.Cut(raw, "#")
	decoded := lineSuffixRE.ReplaceAllString(rawPath, "")
	if d, err := url.PathUnescape(decoded); err == nil {
		decoded = d
	}
	target := doc
	if decoded != "" {
		target = path.Join(path.Dir(doc), decoded)
	}
	if f, err := url.PathUnescape(fragment); err == nil {
		fragment = f
	}
	wrap := func(d string) string {
		if angled {
			return "<" + d + ">"
		}
		return d
	}
	switch _, isDoc := r.docs[target]; {
	case strings.HasPrefix(target, "../") || target == "..":
		return "", "", "", false
	case pageAt(target, format200.RegisterFile):
		var named []string
		for _, e := range r.entries[target] {
			if hasFragment && format200.Slugify(r.titles[e]) == fragment {
				named = append(named, e)
			}
		}
		if at, anchor, ok := r.placeOf(named); ok && len(named) == 1 {
			return wrap(destination(doc, at, anchor)), named[0], "", false
		} else if ok {
			return wrap(destination(doc, at, anchor)), "", fmt.Sprintf("names %d entries of the 3.0.0 register page %s, which is removed, all stated in the section of %s at #%s, so the node it cites cannot be told; it is pointed at that section and no id added",
				len(named), target, at, anchor), false
		}
		page, replaced := r.summaryReplacing(target)
		switch {
		case !replaced:
			return "", "", fmt.Sprintf("points into the 3.0.0 register page %s, which is removed, and no summary page replaces it; its link text stands without the link", target), true
		case !hasFragment:
			return wrap(destination(doc, page, "")), "", "", false
		default:
			return wrap(destination(doc, page, "")), "", fmt.Sprintf("names %d entries of the 3.0.0 register page %s, which is removed, and no one place holds them; it is pointed at the summary page replacing it, %s, without its anchor",
				len(named), target, page), false
		}
	case !isDoc || !hasFragment:
		return "", "", "", false
	}
	h, why := r.headingAt(target, fragment)
	if why != "" {
		return "", "", why, false
	}
	dest = wrap(rawPath + "#" + h.anchor)
	switch nodes := r.sectionNodes(target, h.line); len(nodes) {
	case 0:
		return dest, "", "", false
	case 1:
		return dest, nodes[0], "", false
	default:
		return dest, "", fmt.Sprintf("points into the section of %s holding the statements of %s, so the node it cites cannot be told; its anchor is rewritten and no id added",
			target, strings.Join(nodes, ", ")), false
	}
}

// placeOf is the one citation target every node of ids has; ok is false
// where ids is empty, a node has none, or two nodes' differ.
func (r *relinker) placeOf(ids []string) (doc, anchor string, ok bool) {
	for i, id := range ids {
		at, a, has := r.target(id)
		if !has || i > 0 && (at != doc || a != anchor) {
			return "", "", false
		}
		doc, anchor = at, a
	}
	return doc, anchor, len(ids) > 0
}

// summaryReplacing is the summary page that replaces the 3.0.0 register page
// register: the one its entries render on, its volume's where it stands in a
// volume, else the root's. ok is false where no node renders there, so the
// save writes no page.
func (r *relinker) summaryReplacing(register string) (page string, ok bool) {
	page = format400.SummaryPage(register, r.volumes)
	return page, len(r.entries[registerReplacedBy(page)]) > 0
}

// registerReplacedBy is the kb-root-relative 3.0.0 register page standing
// where the summary page page stands.
func registerReplacedBy(page string) string {
	return strings.TrimSuffix(page, format400.SummaryFile) + format200.RegisterFile
}

// headingAt is the heading of doc fragment names: the first whose 3.0.0
// anchor it is, else the one whose 4.0.0 anchor it already is; and why none
// is taken, "" where one is: it names none, or its 3.0.0 and its 4.0.0
// readings name two different headings.
func (r *relinker) headingAt(doc, fragment string) (heading, string) {
	hs := r.headings[doc]
	byOld := slices.IndexFunc(hs, func(h heading) bool { return h.old == fragment })
	byAnchor := slices.IndexFunc(hs, func(h heading) bool { return h.anchor == fragment })
	switch {
	case byOld >= 0 && byAnchor >= 0 && byOld != byAnchor:
		return heading{}, fmt.Sprintf("names the heading of %s at line %d under 3.0.0's slugging and the one at line %d under 4.0.0's, so the heading it means cannot be told; it is left as written",
			doc, hs[byOld].line+1, hs[byAnchor].line+1)
	case byOld >= 0:
		return hs[byOld], ""
	case byAnchor >= 0:
		return hs[byAnchor], ""
	}
	return heading{}, fmt.Sprintf("names no heading of %s; it is left as written", doc)
}

// edit replaces line[start:end] with text.
type edit struct {
	start, end int
	text       string
}

// convertLine is line n of doc with its links re-pointed and its bare ids
// cited, and what it reports; view is the line with its code blanked.
func (r *relinker) convertLine(doc string, n int, line, view string) (string, []common.Warning) {
	var edits []edit
	var warnings []common.Warning
	warn := func(list, id, detail string) {
		warnings = append(warnings, common.Warning{Path: format200.KBDir + "/" + doc, Line: n + 1, List: list, ID: id,
			Detail: fmt.Sprintf("%s:%d: %s", doc, n+1, detail)})
	}
	// cite is node id's citation in the naming form, read back as one link
	// citing it; ok false where the node has no place or its title breaks the
	// link.
	cite := func(id string) (string, bool) {
		at, anchor, ok := r.target(id)
		if !ok {
			return "", false
		}
		c := format400.Citation(r.titles[id], destination(doc, at, anchor), id)
		back := format400.Links(c)
		if len(back) != 1 || back[0].Start != 0 || back[0].End != len(c) || back[0].Title != id {
			return "", false
		}
		return c, true
	}
	links := format400.Links(view)
	// byID is every link whose whole text is the id of a node it does not cite
	// another node by: the naming form of that node, by its id.
	byID := map[int]bool{}
	// unlinked is every link that goes, its text standing alone and read as
	// prose.
	unlinked := map[int]bool{}
	for i, l := range links {
		text := strings.TrimSpace(l.Text)
		named := renamed(r.minted, text)
		if cited, cites := l.CitedID(); r.titles[named] != "" && (!cites || cited == named) {
			byID[i] = true
			if c, ok := cite(named); ok {
				edits = append(edits, edit{l.Start, l.End, c})
				warn(common.ListBareIDs, named, fmt.Sprintf("the link %q naming %s by its id is its citation in the naming form, %s", line[l.Start:l.End], text, c))
			} else {
				warn(common.ListUnresolvedCitations, named, fmt.Sprintf("the link %q names %s by its id, and %s has no place a citation points at; it is left as written", line[l.Start:l.End], text, text))
			}
			continue
		}
		dest, id, why, unlink := r.relink(doc, l)
		written := line[l.Start:l.End]
		if why != "" {
			warn(common.ListUnresolvedCitations, id, fmt.Sprintf("the link %q %s", written, why))
		}
		if unlink {
			unlinked[i] = true
			edits = append(edits, edit{l.Start, l.Start + 1, ""}, edit{l.Start + 1 + len(l.Text), l.End, ""})
			continue
		}
		if dest == "" {
			dest = l.Destination
		}
		if _, cites := l.CitedID(); id != "" && l.Title != "" && !cites {
			warn(common.ListUnresolvedCitations, id, fmt.Sprintf("the link %q cites %s and carries a title of its own, so the id is not added; its anchor is rewritten", written, id))
		} else if id != "" && l.Title == "" {
			dest += ` "` + id + `"`
		}
		if dest != l.Destination {
			edits = append(edits, edit{l.DestStart, l.DestEnd, dest})
		}
	}
	for _, tok := range format400.BareTokens(view, oldIDTokenRE) {
		id := renamed(r.minted, tok.ID)
		if r.titles[id] == "" {
			continue
		}
		if i := slices.IndexFunc(links, func(l format400.Link) bool { return l.Start <= tok.Start && tok.End <= l.End }); i >= 0 && !unlinked[i] {
			if !byID[i] {
				warn(common.ListUnresolvedCitations, id, fmt.Sprintf("%s stands inside a link's text, where its citation cannot nest; it is left as written", tok.ID))
			}
			continue
		}
		c, ok := cite(id)
		if !ok {
			warn(common.ListUnresolvedCitations, id, fmt.Sprintf("the bare id %s names a node with no place a citation points at; it is left as written", tok.ID))
			continue
		}
		edits = append(edits, edit{tok.Start, tok.End, c})
		warn(common.ListBareIDs, id, fmt.Sprintf("the bare id %s is its citation in the naming form, %s", tok.ID, c))
	}
	if doc == format200.AxiomsFile {
		if m := axiomHeadingRE.FindStringSubmatch(view); m != nil && !axiomMarkerRE.MatchString(line) {
			if id := r.axioms[m[1]]; id != "" {
				end := len(strings.TrimRight(line, " \t\r"))
				edits = append(edits, edit{end, end, format400.StatementMarker(id)})
			} else {
				warn("", "", fmt.Sprintf("the axiom heading of label %s names no axiom record, so no marker pairs it with one; it is left as written", m[1]))
			}
		}
	}
	slices.SortFunc(edits, func(a, b edit) int { return b.start - a.start })
	for _, e := range edits {
		line = line[:e.start] + e.text + line[e.end:]
	}
	return line, warnings
}

// relPath is the slash path of target relative to the directory dir, both
// kb-root-relative.
func relPath(target, dir string) string {
	split := func(p string) []string {
		var out []string
		for _, s := range strings.Split(p, "/") {
			if s != "" && s != "." {
				out = append(out, s)
			}
		}
		return out
	}
	t, d := split(target), split(dir)
	common := 0
	for common < len(t) && common < len(d) && t[common] == d[common] {
		common++
	}
	parts := slices.Repeat([]string{".."}, len(d)-common)
	return strings.Join(append(parts, t[common:]...), "/")
}
