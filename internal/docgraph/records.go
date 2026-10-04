package docgraph

import (
	"cmp"
	"slices"
	"strings"

	"kbase/internal/records"
)

// draft is a record as the walk reads it, before the tree places it: the
// section it was read in, where on the page the writer sets it, and a
// reference's URL as the reader wrote it.
type draft struct {
	records.Record
	section int
	site    site
	url     string
}

// site is where on the page the writer puts a fact the reader placed in a
// section.
type site int

const (
	siteBody site = iota
	siteHeading
	siteNote
	siteBibliography
)

// The writer's escaping of an attribute value: a Link's attributes go
// through pandoc's HTML writer, which escapes the apostrophe too; a Span's or a
// Div's through its Markdown writer, which does not.
var (
	linkAttr  = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	blockAttr = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
)

// The citeproc reference list and its entries, as the reader marks them.
const (
	bibliographyID    = "refs"
	bibliographyClass = "csl-bib-body"
	workIDPrefix      = "ref-"
)

// volumeReader walks the reader's AST once, in document order: it builds the
// outline from the blocks and records each reader fact against the section
// the outline has reached. It reads each node as the transform will hand it to
// the writer: an author's block is a blockquote, whose headings are its
// content, and the title page is gone.
type volumeReader struct {
	names    map[string]string
	keysOnly bool
	o        outline
	// outlining is set while the blocks are walked: the abstract's facts are
	// recorded, but it declares no heading and no label.
	outlining bool
	err       error
	records   []draft
	within    []int
	site      site
	// opening counts the references still to come in the current block's
	// opening emphasis run, which is the first thing its content walks.
	opening int
}

func readVolume(doc node, names map[string]string, keysOnly bool) (outline, []draft, error) {
	c := &volumeReader{names: names, keysOnly: keysOnly, o: outline{
		labelSection: map[string]int{}, headerLabels: map[string]bool{}, anchoredLabels: map[string]bool{},
	}}
	if meta, ok := doc["meta"].(node); ok {
		c.walk(meta["abstract"], false, noMargin)
	}
	c.outlining = true
	c.walk(doc["blocks"], false, pageMargin)
	return c.o, c.records, c.err
}

// margin is where a block's first line sits on the page: at the page's own
// margin, at the margin of a blockquote one level deep, or neither. kb_tools'
// reader reads a label line only at a one-level quote's margin, and the
// transform's labelled blockquote puts its own there only from the page's.
type margin int

const (
	noMargin margin = iota
	pageMargin
	quoteMargin
)

// inside is the margin a blockquote's content sits at when the blockquote
// sits at m.
func (m margin) inside() margin {
	if m == pageMargin {
		return quoteMargin
	}
	return noMargin
}

func (c *volumeReader) section() int { return len(c.o.headers) - 1 }

// declare places a label in the current section.
func (c *volumeReader) declare(label string) {
	if c.outlining && label != "" {
		c.o.labelSection[label] = c.section()
	}
}

func (c *volumeReader) add(r records.Record) int {
	return c.addDraft(draft{Record: r})
}

func (c *volumeReader) addDraft(d draft) int {
	d.Order = len(c.records) + 1
	d.section = c.section()
	d.site = c.site
	if len(c.within) > 0 {
		d.Within = c.within[len(c.within)-1]
	}
	c.records = append(c.records, d)
	return d.Order
}

// at walks v with the site set to s.
func (c *volumeReader) at(s site, v any, quoted bool) {
	saved := c.site
	c.site = s
	c.walk(v, quoted, noMargin)
	c.site = saved
}

// quote walks a blockquote's content. A block whose label line is read inside
// it, with no readable block around it, hosts what follows it to the quote's
// end: kb_tools hosts a fact in the first block whose extent covers it, and
// every extent in one quote ends where the quote does.
func (c *volumeReader) quote(content any, m margin) {
	saved := len(c.within)
	c.walk(content, true, m)
	c.within = c.within[:saved]
}

// walk reads v in document order. quoted is inside a blockquote, where a
// heading is the block's content; m is where a block's first line sits.
func (c *volumeReader) walk(v any, quoted bool, m margin) {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			c.walk(e, quoted, m)
		}
	case node:
		t, _ := x["t"].(string)
		args := list(x["c"])
		if identified[t] {
			id, classes, _ := attr(args[0])
			if _, titlepage := blockLabel(classes, c.names); t == typeDiv && titlepage {
				return
			}
			c.declare(id)
			if c.outlining && id != "" && t != typeTable {
				c.o.anchoredLabels[id] = true
			}
		}
		switch t {
		case typeHeader:
			id, _, _ := attr(args[1])
			if quoted || !c.outlining {
				c.declare(id)
				c.walk(args[2], quoted, noMargin)
				return
			}
			c.o.headers = append(c.o.headers, header{level: atoi(args[0]), identifier: id, node: x})
			if id != "" {
				c.o.headerLabels[id] = true
			}
			c.declare(id)
			c.at(siteHeading, args[2], quoted)
			return
		case typeBlockQuote:
			c.quote(args, m.inside())
			return
		case typePara, typePlain:
			if name, id, ok := pageLabel(args); ok && m == quoteMargin {
				order := c.add(records.Record{Kind: records.KindBlock, Name: name, Identifier: blockAttr.Replace(id)})
				if len(c.within) == 0 {
					c.within = append(c.within, order)
				}
			}
		case typeRawBlock:
			if c.outlining && c.err == nil {
				c.err = RawBlockError{Format: str(args[0]), Text: str(args[1])}
			}
			return
		case typeNote:
			// The writer sets a note's text after the body, outside any block.
			saved := c.within
			c.within = nil
			c.at(siteNote, args, quoted)
			c.within = saved
			return
		case typeDiv:
			id, classes, _ := attr(args[0])
			name, _ := blockLabel(classes, c.names)
			switch {
			case name != "":
				c.block(id, name, list(args[1]), m)
				return
			case id == bibliographyID && slices.Contains(classes, bibliographyClass):
				c.at(siteBibliography, args[1], quoted)
				return
			case strings.HasPrefix(id, workIDPrefix):
				c.add(records.Record{Kind: records.KindWork, Key: blockAttr.Replace(strings.TrimPrefix(id, workIDPrefix))})
			}
		case typeMath:
			labels := []string{}
			for _, m := range mathLabel.FindAllStringSubmatch(str(args[1]), -1) {
				labels = append(labels, m[1])
				c.declare(m[1])
			}
			if isDisplayMath(x) {
				c.add(records.Record{Kind: records.KindFence, EquationLabels: labels})
			}
		case typeLink:
			_, _, kv := attr(args[0])
			if typ, ok := kv["reference-type"]; ok {
				r := draft{Record: records.Record{Kind: records.KindReference, Type: linkAttr.Replace(typ), Labels: linkAttr.Replace(kv["reference"])},
					url: linkAttr.Replace(str(list(args[2])[0]))}
				if c.opening > 0 {
					r.Opening = true
					c.opening--
				}
				c.addDraft(r)
			}
		case typeCite:
			keys := citeKeys(x)
			written := make([]string, len(keys))
			for i, k := range keys {
				written[i] = blockAttr.Replace(k)
			}
			c.add(records.Record{Kind: records.KindCitation, Keys: written, States: c.states(keys, args[1])})
		}
		// A Div that is not the author's keeps its content at its margin.
		if t != typeDiv {
			m = noMargin
		}
		for _, k := range sortedKeys(x) {
			c.walk(x[k], quoted, m)
		}
	}
}

// pageLabel is the name of a paragraph the page writes as a label line: one
// bold run of words, alone, inside at most one span carrying an identifier.
// A span carrying no attribute at all is written as its bare content.
func pageLabel(inlines []any) (name, id string, ok bool) {
	if len(inlines) != 1 {
		return "", "", false
	}
	n, t := asNode(inlines[0])
	for t == typeSpan {
		c := list(n["c"])
		sid, classes, kv := attr(c[0])
		inner := list(c[1])
		if (sid != "" && id != "") || len(classes) > 0 || len(kv) > 0 || len(inner) != 1 {
			return "", "", false
		}
		if sid != "" {
			id = sid
		}
		n, t = asNode(inner[0])
	}
	if t != typeStrong {
		return "", "", false
	}
	var b strings.Builder
	for _, v := range list(n["c"]) {
		w, wt := asNode(v)
		switch {
		case wt == typeStr:
			b.WriteString(str(w["c"]))
		case wt == typeSpace || wt == typeSoftBreak:
			b.WriteString(" ")
		default:
			return "", "", false
		}
	}
	return b.String(), id, labelNameRe.MatchString(b.String())
}

// block records an author's block where kb_tools' reader reads its label
// line; one it cannot read is no block to it, and the facts inside host to
// the enclosing block it does read.
func (c *volumeReader) block(id, name string, content []any, m margin) {
	if !labelLineRead(name, m == pageMargin) {
		c.quote(content, m.inside())
		return
	}
	r := records.Record{Kind: records.KindBlock, Name: name, Identifier: blockAttr.Replace(id)}
	opening := 0
	if len(content) > 0 {
		if first, t := asNode(content[0]); t == typePara || t == typePlain {
			opening = openingReferences(list(first["c"]))
		}
	}
	order := c.add(r)
	c.within = append(c.within, order)
	c.opening = opening
	c.quote(content, m.inside())
	c.within = c.within[:len(c.within)-1]
}

func (c *volumeReader) states(keys []string, rendered any) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		switch {
		case c.keysOnly:
			out[i] = records.StateKeyOnly
		case containsStr(rendered, k+"?"):
			out[i] = records.StateUnanswered
		default:
			out[i] = records.StateResolved
		}
	}
	return out
}

func containsStr(v any, want string) bool {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if containsStr(e, want) {
				return true
			}
		}
	case node:
		if t, _ := x["t"].(string); t == typeStr {
			return str(x["c"]) == want
		}
		return containsStr(x["c"], want)
	}
	return false
}

// openingReferences counts the references inside a block's leading emphasis
// run, as "Proof of Theorem 1." carries them. A note's text is set elsewhere,
// so a reference inside one is not the run's.
func openingReferences(inlines []any) int {
	for _, v := range inlines {
		n, t := asNode(v)
		if whitespace[t] {
			continue
		}
		if t != typeEmph {
			return 0
		}
		count := 0
		var descend func(any)
		descend = func(v any) {
			switch x := v.(type) {
			case []any:
				for _, e := range x {
					descend(e)
				}
			case node:
				switch t, _ := x["t"].(string); t {
				case typeNote:
					return
				case typeLink:
					if _, _, kv := attr(list(x["c"])[0]); kv["reference-type"] != "" {
						count++
					}
				}
				descend(x["c"])
			}
		}
		descend(n["c"])
		return count
	}
	return 0
}

// place sets each record's document now the tree exists, adds a reference or
// citation record for each copy of a heading's title the navigation lines
// carry, and orders the records as the tree is read.
func place(drafts []draft, t *volumeTree, headers int) []records.Record {
	type placed struct {
		records.Record
		slot, child int
	}
	// Within a document: the up-link line, the body, the notes, the child list.
	const (
		slotUplink = iota
		slotBody
		slotNotes
		slotChildren
	)
	var all []placed
	for _, d := range drafts {
		r := d.Record
		slot := slotBody
		switch d.site {
		case siteHeading:
			r.Document = t.sectionPath(d.section)
		case siteNote:
			r.Document = t.holder(t.sectionPath(headers - 1))
			slot = slotNotes
		case siteBibliography:
			if t.references != nil {
				r.Document = t.references.path
				break
			}
			fallthrough
		default:
			r.Document = t.holder(t.sectionPath(d.section))
		}
		if r.Kind == records.KindReference {
			r.Href, r.Target = d.url, ""
			if label, ok := strings.CutPrefix(d.url, "#"); ok {
				r.Href, r.Target = t.href(r.Document, label)
			}
		}
		all = append(all, placed{Record: r, slot: slot})
		if d.site != siteHeading || (r.Kind != records.KindReference && r.Kind != records.KindCitation) {
			continue
		}
		// The navigation lines copy a title as the rendering spelled it, so a
		// copied href is the reader's own.
		heading := t.ordered[d.section]
		copied := r
		copied.Within, copied.Href, copied.Target = 0, d.url, ""
		if r.Kind != records.KindReference {
			copied.Href = ""
		}
		for _, child := range heading.children {
			copied.Document = child.path
			all = append(all, placed{Record: copied, slot: slotUplink})
		}
		copied.Document = heading.parent.path
		all = append(all, placed{Record: copied, slot: slotChildren, child: slices.Index(heading.parent.children, heading)})
	}
	slices.SortStableFunc(all, func(a, b placed) int {
		return cmp.Or(strings.Compare(a.Document, b.Document), cmp.Compare(a.slot, b.slot), cmp.Compare(a.child, b.child))
	})
	renumbered := map[int]int{}
	out := make([]records.Record, len(all))
	for i, p := range all {
		if p.Kind == records.KindBlock {
			renumbered[p.Order] = i + 1
		}
		out[i] = p.Record
	}
	for i := range out {
		out[i].Order = i + 1
		out[i].Within = renumbered[out[i].Within]
	}
	return out
}

// WriteRecords writes the report's records and pre-pass census into the
// state directory.
func WriteRecords(stateDir string, r Report) error {
	return records.Write(stateDir, r.Records, r.Censuses)
}
