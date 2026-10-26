package from200to300

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format200"
	"kbase/internal/formats/format300"
)

// statementMarker is one statement marker of a converted document: its byte
// extent there, the 2.0.x site it opens on, and the node ids it names.
type statementMarker struct {
	start, end int
	at         site
	ids        []string
}

// markedDocument is one converted document's statement markers in text order
// and, for each, the ids the conversion takes out of it.
type markedDocument struct {
	path    string
	markers []statementMarker
	drop    []map[string]bool
}

// markerRef is one marker naming an id: its document's index in Documents
// order and its own in that document.
type markerRef struct{ doc, marker int }

// hostedNode is an analysis or an experiment and the site its host leaf
// declares it at.
type hostedNode struct {
	id string
	at site
}

// placeMarkers leaves each node of g at most one statement marker in out.
//
// A claim's marker is the first one its first marking listing leaf holds, in
// path order; where no listing leaf marks it, a marker inserted in its first
// listing leaf; where no leaf lists it, the first marker naming it. An
// analysis's or an experiment's marker standing anywhere is the node's host and
// stays where it is; where none stands, one is inserted on a new line after the
// host's first heading. A marker for an analysis or an experiment that stands
// in a leaf other than the one declaring the node is warned of, naming both
// paths, and its marker's leaf is the statement's home. Every other marker
// naming a node, and every marker naming none, is removed with a warning, as is
// each listing leaf not holding the claim's marker, which becomes a mention.
func (c *conversion) placeMarkers(files, out common.Files, st format200.State, g format300.Graph, docs documents) error {
	rels, err := format200.Documents(files)
	if err != nil {
		return err
	}
	marked := make([]markedDocument, len(rels))
	byID := map[string][]markerRef{}
	for d, rel := range rels {
		p := kbPath(rel)
		marked[d] = readMarkers(p, string(files[p]), string(out[p]))
		for m, mk := range marked[d].markers {
			for _, id := range mk.ids {
				byID[id] = append(byID[id], markerRef{d, m})
			}
		}
	}
	remove := func(r markerRef, id, format string, args ...any) {
		m := &marked[r.doc]
		m.drop[r.marker][id] = true
		c.warn(m.markers[r.marker].at, format, args...)
	}

	titles, known := map[string]string{}, map[string]bool{}
	for _, n := range g.Claims {
		titles[n.ID], known[n.ID] = n.Title, true
	}
	listedBy := map[string][]string{}
	listing := map[string][]string{}
	for _, l := range st.Leaves {
		p := kbPath(l.Path)
		for _, id := range unique(l.Claims) {
			if !known[id] {
				c.warn(docs.listed[p], "claims lists %s, which names no claim; no marker is written for it", id)
				continue
			}
			listedBy[id] = append(listedBy[id], p)
			listing[p] = append(listing[p], id)
		}
	}

	inserted := map[string][]string{}
	for _, n := range g.Claims {
		refs := byID[n.ID]
		keep, host := -1, ""
		for _, p := range listedBy[n.ID] {
			if i := slices.IndexFunc(refs, func(r markerRef) bool { return marked[r.doc].path == p }); i >= 0 {
				keep, host = i, p
				break
			}
		}
		switch {
		case host == "" && len(listedBy[n.ID]) > 0:
			host = listedBy[n.ID][0]
			inserted[host] = append(inserted[host], n.ID)
		case host == "" && len(refs) > 0:
			keep, host = 0, marked[refs[0].doc].path
		}
		for _, p := range listedBy[n.ID] {
			if p != host {
				c.warn(docs.listed[p], "claims lists %s, whose marker is in %s; this listing becomes a mention and its referenced-by row is lost", n.ID, host)
			}
		}
		for i, r := range refs {
			if i != keep {
				remove(r, n.ID, "a marker for %s is removed; its marker is in %s", n.ID, host)
			}
		}
	}

	hosts := map[string][]hostedNode{}
	addHosted := func(id string) {
		at := nth(docs.declared[id], 0)
		known[id] = true
		// A marker already standing is the node's host and stays where it is:
		// it puts the node's statement in the section the marker sits in, which
		// is the leaf's own reading of it — the marker a 1.0.0 conversion
		// places beside the claim it argues for is one. A marker standing in a
		// leaf other than the one declaring the node is warned of: the marker's
		// leaf is still the statement's home, but the declaring leaf's entry is
		// a second home the 2.0.x load cannot resolve. Each marker past the
		// first is dropped, as a repeated claim marker is.
		refs := byID[id]
		if len(refs) == 0 {
			hosts[at.path] = append(hosts[at.path], hostedNode{id, at})
			return
		}
		if markerPath := marked[refs[0].doc].path; at.path != markerPath {
			c.warn(at, "%s is declared in %s, whose marker is in %s; the marker's leaf is its statement's home", id, at.path, markerPath)
		}
		for _, r := range refs[1:] {
			remove(r, id, "a second marker for %s is removed; its marker stands in %s", id, marked[refs[0].doc].path)
		}
	}
	for _, a := range g.Analyses {
		addHosted(a.ID)
	}
	for _, x := range g.Experiments {
		addHosted(x.ID)
	}
	for _, hs := range hosts {
		slices.SortStableFunc(hs, func(a, b hostedNode) int { return cmp.Compare(a.at.line, b.at.line) })
	}

	for d, md := range marked {
		for m, mk := range md.markers {
			for _, id := range mk.ids {
				if !known[id] {
					remove(markerRef{d, m}, id, "a marker for %s, which names no node, is removed", id)
				}
			}
		}
	}

	for _, md := range marked {
		text := removeMarkers(string(out[md.path]), md)
		text = c.insertMarkers(text, md.path, docs.listed[md.path], listing[md.path], inserted[md.path], hosts[md.path], titles)
		out[md.path] = []byte(text)
	}
	return nil
}

// unique is ids without repeats, in first-seen order.
func unique(ids []string) []string {
	var out []string
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// readMarkers is the statement markers of converted, the document at p whose
// 2.0.x bytes are original: the conversion removed only whole lines above its
// body, so a marker's 2.0.x line is its converted line plus their number.
func readMarkers(p, original, converted string) markedDocument {
	md := markedDocument{path: p}
	removed := len(format200.SplitLinesKeepEnds(original)) - len(format200.SplitLinesKeepEnds(converted))
	var starts []int
	off := 0
	for _, l := range format200.SplitLinesKeepEnds(converted) {
		starts = append(starts, off)
		off += len(l)
	}
	for _, m := range format300.StatementMarkerRE.FindAllStringSubmatchIndex(converted, -1) {
		ids := unique(format200.NodeIDs(converted[m[2]:m[3]]))
		if len(ids) == 0 {
			continue
		}
		line := sort.SearchInts(starts, m[0]+1)
		md.markers = append(md.markers, statementMarker{start: m[0], end: m[1], at: site{p, line + removed}, ids: ids})
		md.drop = append(md.drop, map[string]bool{})
	}
	return md
}

// removeMarkers is text without the ids md drops: a marker keeping some of
// its ids is rewritten as one marker for each, one keeping none is taken from
// its line with the space before it, and a line it leaves blank goes with it.
func removeMarkers(text string, md markedDocument) string {
	for i := len(md.markers) - 1; i >= 0; i-- {
		mk, drop := md.markers[i], md.drop[i]
		if len(drop) == 0 {
			continue
		}
		var kept []string
		for _, id := range mk.ids {
			if !drop[id] {
				kept = append(kept, format300.StatementMarker(id))
			}
		}
		if len(kept) > 0 {
			text = text[:mk.start] + strings.Join(kept, " ") + text[mk.end:]
			continue
		}
		from, to := lineSpan(text, mk.start, mk.end)
		if format200.Strip(text[from:mk.start]+text[mk.end:to]) == "" {
			text = text[:from] + text[to:]
			continue
		}
		start := mk.start
		if start > from && text[start-1] == ' ' {
			start--
		}
		text = text[:start] + text[mk.end:]
	}
	return text
}

// lineSpan is the extent of the lines text[start:end] touches, the last one's
// line break included.
func lineSpan(text string, start, end int) (from, to int) {
	from, off := -1, 0
	for _, l := range format200.SplitLinesKeepEnds(text) {
		next := off + len(l)
		if from < 0 && start < next {
			from = off
		}
		if end <= next {
			return from, next
		}
		off = next
	}
	return from, len(text)
}

// insertMarkers is text, the leaf at p, with a marker for each of claims and
// hosts. An equation claim's goes on the line inside its display-maths fence
// carrying its label; the claim of a leaf listing one non-equation claim and
// holding one label line, on that block's first content line, so the label
// line still reads as one; any other claim's on the first
// heading, with a warning at listedAt. Each hosted node's goes on a new line
// after the first heading, in declaration order. A leaf with no heading gets
// none of the markers the heading would carry, each with a warning.
func (c *conversion) insertMarkers(text, p string, listedAt site, listing, claims []string, hosts []hostedNode, titles map[string]string) string {
	if len(claims) == 0 && len(hosts) == 0 {
		return text
	}
	lines := format200.SplitLines(text)
	scrubbed := format200.SplitLines(format200.StripCodeFences(text))
	heading := slices.IndexFunc(scrubbed, func(l string) bool { return format200.FirstHeading(l) != "" })
	var labelLines []int
	for i, l := range scrubbed {
		if _, ok := format300.LabelLine(l); ok {
			labelLines = append(labelLines, i)
		}
	}
	statements := 0
	for _, id := range listing {
		if _, eq := format300.EquationLabel(titles[id]); !eq {
			statements++
		}
	}

	appended := map[int][]string{}
	for _, id := range claims {
		at := -1
		if label, eq := format300.EquationLabel(titles[id]); eq {
			at = equationLine(lines, label)
		} else if statements == 1 && len(labelLines) == 1 {
			at = blockContentLine(scrubbed, labelLines[0])
		}
		if at < 0 {
			if heading < 0 {
				c.warn(listedAt, "%s gets no marker: %s has no heading to carry it", id, p)
				continue
			}
			at = heading
			c.warn(listedAt, "%s's marker is appended to the first heading; the section under it is its statement", id)
		}
		appended[at] = append(appended[at], format300.StatementMarker(id))
	}
	var after []string
	for _, h := range hosts {
		if heading < 0 {
			c.warn(h.at, "%s gets no marker: %s has no heading to place it after", h.id, p)
			continue
		}
		after = append(after, format300.StatementMarker(h.id))
	}

	var b strings.Builder
	for i, full := range format200.SplitLinesKeepEnds(text) {
		line, end := lines[i], full[len(lines[i]):]
		for _, m := range appended[i] {
			line = format300.AppendMarker(line, m)
		}
		b.WriteString(line)
		if i == heading {
			nl := cmp.Or(end, "\n")
			for _, m := range after {
				b.WriteString(nl + m)
			}
		}
		b.WriteString(end)
	}
	return b.String()
}

// blockContentLine is the index of the first line after the label line at
// label, inside the same blockquote, carrying text beyond its quote prefix;
// -1 where the block has none.
func blockContentLine(lines []string, label int) int {
	for i := label + 1; i < len(lines); i++ {
		prefix := format200.BlockquotePrefix.FindString(lines[i])
		if prefix == "" {
			break
		}
		if format200.Strip(lines[i][len(prefix):]) != "" {
			return i
		}
	}
	return -1
}

// equationLine is the index of the first line inside a display-maths fence of
// lines carrying label's locator, -1 where none does. The fence's opening line
// is not inside it, and its closing line can carry no locator.
func equationLine(lines []string, label string) int {
	for _, f := range format200.Fences(lines) {
		if f.Info != format300.DisplayMathInfo {
			continue
		}
		for i := f.Start + 1; i < f.End; i++ {
			if strings.Contains(lines[i], format300.EquationLocator(label)) {
				return i
			}
		}
	}
	return -1
}
