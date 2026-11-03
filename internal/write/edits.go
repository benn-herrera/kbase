package write

import (
	"cmp"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/result"
)

// editOps is each node kind's edit op.
var editOps = map[string]string{
	kb.NodeKindClaim:      EditClaim,
	kb.NodeKindAnalysis:   EditAnalysis,
	kb.NodeKindExperiment: EditExperiment,
	kb.NodeKindWork:       EditWork,
	kb.NodeKindAxiom:      EditAxiom,
}

// isNodeEdit is whether op is a node kind's edit op.
func isNodeEdit(op string) bool {
	for _, edit := range editOps {
		if edit == op {
			return true
		}
	}
	return false
}

// editable is one node record by its id and the fields an edit may change,
// each nil where its kind carries none; manual is the manually-scored mark
// its rigor carries.
type editable struct {
	id                              string
	title, label, rationale, status *string
	rigor                           **float64
	manual                          *bool
}

// nodesOf is every node record of kind as its editable fields, in record
// order.
func (b *recordBatch) nodesOf(kind string) []editable {
	g := &b.g
	var out []editable
	switch kind {
	case kb.NodeKindClaim:
		for i := range g.Claims {
			n := &g.Claims[i]
			out = append(out, editable{id: n.ID, title: &n.Title, rationale: &n.Rationale, rigor: &n.Rigor, manual: &n.ManuallyScored})
		}
	case kb.NodeKindAnalysis:
		for i := range g.Analyses {
			n := &g.Analyses[i]
			out = append(out, editable{id: n.ID, title: &n.Title, rationale: &n.Rationale, rigor: &n.Rigor, manual: &n.ManuallyScored})
		}
	case kb.NodeKindExperiment:
		for i := range g.Experiments {
			n := &g.Experiments[i]
			out = append(out, editable{id: n.ID, title: &n.Title, status: &n.Status, rigor: &n.Rigor, manual: &n.ManuallyScored})
		}
	case kb.NodeKindWork:
		for i := range g.Works {
			n := &g.Works[i]
			out = append(out, editable{id: n.ID, title: &n.Title, label: &n.Label, rationale: &n.Rationale, rigor: &n.Rigor})
		}
	case kb.NodeKindAxiom:
		for i := range g.Axioms {
			n := &g.Axioms[i]
			out = append(out, editable{id: n.ID, title: &n.Title, label: &n.Label})
		}
	}
	return out
}

// titleHolder is the id of the node of kind carrying title, "" where none
// does.
func (b *recordBatch) titleHolder(kind, title string) string {
	for _, n := range b.nodesOf(kind) {
		if *n.title == title {
			return n.id
		}
	}
	return ""
}

// labelHolder is the id of the node of kind carrying label, "" where none
// does.
func (b *recordBatch) labelHolder(kind, label string) string {
	for _, n := range b.nodesOf(kind) {
		if n.label != nil && *n.label == label {
			return n.id
		}
	}
	return ""
}

// editIdentity is the keys naming what an edit changes: a node's id, or an
// edge's source, target and relation.
var editIdentity = []string{editID.name, "source", "target", "relation"}

// editFields is the fields edit op may change, sorted.
func editFields(op string) []string {
	var fields []string
	for _, f := range opFields[op] {
		if !slices.Contains(editIdentity, f.name) {
			fields = append(fields, f.name)
		}
	}
	slices.Sort(fields)
	return fields
}

// noNodeRemedy is how op's entry naming id at key, which no node carries, is
// put right: a removal's node may already be gone, and any other node's add
// mints its id, so it is named only in a later batch.
func noNodeRemedy(op, key, id string) string {
	if removal(op) {
		return kb.NodeLister + ". Where an earlier batch or an earlier entry of this one already removed " + id +
			", nothing is left to do: drop the entry. Otherwise correct " + key + ", then re-run"
	}
	return kb.NodeLister + ". Where this batch adds the node you meant, its id is known only from that add's result: " +
		"move this entry to a later batch, naming that id. Otherwise correct " + key + ", then re-run"
}

// noNode refuses op's entry naming at key a node id no record carries.
func noNode(op, key, id string) error {
	detail, _ := kb.NoNode(key, id)
	return result.Refusal{{Check: kb.CheckUnknownID, Key: key, Detail: detail, Remedy: noNodeRemedy(op, key, id)}}
}

// refuseFieldless refuses, at shape, each edit entry giving no field to
// change, allowing its op's fields.
func refuseFieldless(entries []entry) error {
	var refusals result.Refusal
	for _, e := range entries {
		fields := editFields(e.Op)
		if !slices.ContainsFunc(fields, e.has) {
			refusals = append(refusals, result.Item{Entry: e.Index, Allowed: fields,
				Detail: "gives no field to change; an edit changes at least one of " + strings.Join(fields, ", ")})
		}
	}
	if len(refusals) > 0 {
		return refusals
	}
	return nil
}

// editNode is the edit of a node of kind: each field its entry gives replaces
// that field of the node its id names. It is refused where the id names a
// node of another kind or no node, and where the title or the label is
// another node's of the kind. A rigor given marks the node manually scored
// where its kind carries the mark, and records the judged hash of its
// statement.
func editNode(kind string) func(*recordBatch, entry) error {
	return func(b *recordBatch, e entry) error {
		id := e.str(editID.name)
		if k, _ := kb.NodeIDKind(id); k != kind {
			return opRefusal(editID.name, fmt.Sprintf("%s is the id of a node of kind %s, and %s edits a node of kind %s", id, k, editOps[kind], kind),
				"restore: edit it with "+editOps[k]+", then re-run")
		}
		nodes := b.nodesOf(kind)
		i := slices.IndexFunc(nodes, func(n editable) bool { return n.id == id })
		if i < 0 {
			return noNode(e.Op, editID.name, id)
		}
		n := nodes[i]
		b.report = entryReport{ID: id}
		title, label := *n.title, ""
		if e.has("title") {
			title = e.str("title")
		}
		if n.label != nil {
			label = *n.label
			if e.has("label") {
				label = e.str("label")
			}
		}
		if title != *n.title || (n.label != nil && label != *n.label) {
			if err := b.rename(kind, n, title, label); err != nil {
				return err
			}
		}
		if e.has("rigor") {
			*n.rigor = e.score("rigor")
			if n.manual != nil {
				*n.manual = b.judged(true)
				b.rigorSet(e, id)
			}
		}
		if e.has("rationale") {
			*n.rationale = e.str("rationale")
		}
		if e.has("status") {
			*n.status = e.str("status")
		}
		return nil
	}
}

// rename gives node n of kind title and label, each where it differs,
// refused where another node of the kind carries either, and, for a work,
// where a citation span or a reference-list entry already uses label as a
// key. It keeps every link into n live in the same write: an axiom's heading
// in axioms.md is rewritten, its marker kept; each citation in the naming
// form, its link text the old title, takes the new one; a work's every use
// of its old label as a key takes the new one (rekey); each citation of n
// pointing where the rename moves n's link target from is re-pointed to where
// it moves it; and every link pointing at a heading or a summary entry the
// rename moved is re-pointed to its new anchor.
func (b *recordBatch) rename(kind string, n editable, title, label string) error {
	retitled, relabelled := title != *n.title, n.label != nil && label != *n.label
	if retitled {
		if held := b.titleHolder(kind, title); held != "" {
			return opRefusal("title", fmt.Sprintf("%s carries the title %q, and a title is unique within its kind", held, title),
				"restore: give a title no other node of kind "+kind+" carries, then re-run")
		}
	}
	if relabelled {
		if held := b.labelHolder(kind, label); held != "" {
			return opRefusal("label", fmt.Sprintf("%s carries the label %q, and a label is unique within its kind", held, label),
				"restore: give a label no other node of kind "+kind+" carries, then re-run")
		}
		if kind == kb.NodeKindWork {
			if err := b.refuseKeyInUse(n.id, label); err != nil {
				return err
			}
		}
	}
	before := b.leafTexts()
	was, err := b.anchored()
	if err != nil {
		return err
	}
	oldTitle, oldLabel := *n.title, ""
	*n.title = title
	if n.label != nil {
		oldLabel, *n.label = *n.label, label
	}
	if kind == kb.NodeKindAxiom {
		if err := b.rewriteAxiomHeading(n.id); err != nil {
			return err
		}
	}
	if retitled {
		if err := b.renameCitations(n.id, oldTitle, title); err != nil {
			return err
		}
	}
	if relabelled && kind == kb.NodeKindWork {
		if b.report.Rewritten, err = b.rekey(oldLabel, label); err != nil {
			return err
		}
	}
	now, err := b.anchored()
	if err != nil {
		return err
	}
	moved := b.movedHeadings(before)
	moved.summaries(was.summary, now.summary)
	from, to := was.target[n.id], now.target[n.id]
	return b.repoint(func(rel string, l kb.TextLink) string {
		if t := pointsAt(rel, l.Destination); l.Title == n.id && t == from && t != to {
			return retargeted(rel, l.Destination, to)
		}
		return moved.dest(rel, l.Destination)
	})
}

// rewriteAxiomHeading rewrites each heading of axioms.md carrying the axiom
// id's marker to carry the axiom's label and title, its marker kept,
// appending one where none stands.
func (b *recordBatch) rewriteAxiomHeading(id string) error {
	l, err := b.axiomsDocument()
	if err != nil {
		return err
	}
	n := record(b.g.Axioms, func(n kb.AxiomNode) bool { return n.ID == id })
	headings := slices.DeleteFunc(kb.AxiomHeadings(l.text), func(h kb.AxiomHeading) bool { return h.ID != id })
	if len(headings) == 0 {
		appendAxiomHeading(l, n.Label, n.Title, id)
	}
	for _, h := range headings {
		l.text = spliceLines(l.text, h.Line-1, h.Line, []string{kb.AxiomHeadingLine(n.Label, n.Title, id)})
	}
	return nil
}

// refuseKeyInUse refuses work id's relabelling to label where a citation
// span or a reference-list entry already uses label as a key: the rewrite
// would merge two references into one. Each such line is named.
func (b *recordBatch) refuseKeyInUse(id, label string) error {
	var refusals result.Refusal
	err := b.eachDocument(func(rel string, l *leafEdit, _ bool) error {
		last := -1
		for _, r := range kb.CitationKeyRefs(l.text) {
			if r.Key != label || (r.Use != kb.KeyInSpan && r.Use != kb.KeyInEntry) || r.Line == last {
				continue
			}
			last = r.Line
			refusals = append(refusals, result.Item{Key: "label", Path: rel, Line: r.Line + 1,
				Detail: fmt.Sprintf("%s:%d already uses %s as a citation key, so relabelling %s to it would merge two references into one", rel, r.Line+1, label, id),
				Remedy: "give a label no citation span or reference-list entry uses as a key, then re-run"})
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(refusals) > 0 {
		return refusals
	}
	return nil
}

// rekey gives every use of the citation key from as a key
// (kb.CitationKeyRefs) the key to, in every document but the summary pages,
// and is every line it rewrote, by path then line, [] where none.
func (b *recordBatch) rekey(from, to string) ([]Rewritten, error) {
	out := []Rewritten{}
	err := b.rewriteDocuments(func(rel, text string) (string, error) {
		lines := kb.SplitLines(text)
		rewritten := slices.Clone(lines)
		refs := kb.CitationKeyRefs(text)
		for i := len(refs) - 1; i >= 0; i-- {
			if r := refs[i]; r.Key == from {
				rewritten[r.Line] = rewritten[r.Line][:r.Start] + to + rewritten[r.Line][r.End:]
			}
		}
		for n := range lines {
			if rewritten[n] != lines[n] {
				out = append(out, Rewritten{Path: rel, Line: n + 1})
			}
		}
		return spliceChanged(text, lines, rewritten), nil
	})
	return out, err
}

// citeTarget is where a citation links: a kb-root-relative document and an
// anchor in it.
type citeTarget struct{ path, anchor string }

// derived is the derivation's rows over the records and documents as the
// batch has left them.
func (b *recordBatch) derived() (index.Rows, error) {
	src, err := kbload.Open(b.root)
	if err != nil {
		return index.Rows{}, err
	}
	for _, f := range recordFiles(b.g) {
		src.Put(src.KBPath(f.rel), f.data)
	}
	for _, l := range b.leaves {
		src.Put(l.target, []byte(l.text))
	}
	return index.Derive(src)
}

// targetOf is node id's link target over the records and documents as the
// batch has left them: a hosted node's host and the anchor of its marker's
// section, and an axiom's or a work's as the derivation places it.
func (b *recordBatch) targetOf(id string) (citeTarget, error) {
	if kind, _ := kb.NodeIDKind(id); kind != kb.NodeKindAxiom && kind != kb.NodeKindWork {
		host, ok := b.hosts[id]
		if !ok {
			return citeTarget{}, nil
		}
		l, err := b.leafAt(host)
		if err != nil || l == nil {
			return citeTarget{}, err
		}
		return citeTarget{host, index.SectionSlug(l.text, kb.MarkerLines(l.text)[id])}, nil
	}
	rows, err := b.derived()
	if err != nil {
		return citeTarget{}, err
	}
	for _, r := range rows.Nodes {
		if r.ID == id {
			return citeTarget{r.CanonicalPath, r.CanonicalAnchor}, nil
		}
	}
	return citeTarget{}, nil
}

// anchors is where the derivation places nodes, by id: target, each work's
// and axiom's link target, and summary, each node's summary entry.
type anchors struct{ target, summary map[string]citeTarget }

// anchored is the anchors over the records and documents as the batch has
// left them.
func (b *recordBatch) anchored() (anchors, error) {
	rows, err := b.derived()
	if err != nil {
		return anchors{}, err
	}
	out := anchors{target: map[string]citeTarget{}, summary: map[string]citeTarget{}}
	for _, r := range rows.Nodes {
		if r.NodeType == kb.NodeKindWork || r.NodeType == kb.NodeKindAxiom {
			out.target[r.ID] = citeTarget{r.CanonicalPath, r.CanonicalAnchor}
		}
	}
	for id, page := range rows.RegisterPages {
		out.summary[id] = citeTarget{page, rows.RegisterAnchors[id]}
	}
	return out, nil
}

// repeatSuffixRE is the repeat suffixes the slugger appends to a slug.
var repeatSuffixRE = regexp.MustCompile(`(?:-[0-9]+)+$`)

// summaryMayShift is whether adding or removing a summary entry titled title,
// for node id, can move another entry's anchor. The slugger suffixes a repeated
// slug, so only a heading whose slug, its repeat suffixes stripped, equals
// another's can move: the entry's own title, another node's, or a page's fixed
// rigor and strength headings, which every entry carries. A false answer
// spares an add or a removal the two derivations anchored costs; a build adds
// many nodes per batch, and a derivation per add would make it quadratic.
func (b *recordBatch) summaryMayShift(id, title string) bool {
	base := func(heading string) string { return repeatSuffixRE.ReplaceAllString(kb.Slugify(heading), "") }
	fixed := []string{base(kb.SummaryRigorHeading), base(kb.SummaryStrengthHeading)}
	if slices.Contains(fixed, base(title)) {
		return true
	}
	for _, kind := range []string{kb.NodeKindClaim, kb.NodeKindAnalysis, kb.NodeKindExperiment, kb.NodeKindWork} {
		for _, n := range b.nodesOf(kind) {
			if t := base(*n.title); n.id != id && (t == base(title) || slices.Contains(fixed, t)) {
				return true
			}
		}
	}
	return false
}

// keepSummaryLinks applies change, an add or a removal of the summary entry
// of node id titled title, then re-points every link into another entry
// whose anchor change moved (summaryMayShift). A link into a removed entry
// has no new destination and stands.
func (b *recordBatch) keepSummaryLinks(id, title string, change func() error) error {
	if !b.summaryMayShift(id, title) {
		return change()
	}
	was, err := b.anchored()
	if err != nil {
		return err
	}
	if err := change(); err != nil {
		return err
	}
	now, err := b.anchored()
	if err != nil {
		return err
	}
	moved := movedAnchors{}
	moved.summaries(was.summary, now.summary)
	if len(moved) == 0 {
		return nil
	}
	return b.repoint(func(rel string, l kb.TextLink) string { return moved.dest(rel, l.Destination) })
}

// eachDocument calls visit with every document citations are read in
// (kb.CitingDocuments) and every document an earlier entry created, in path
// order: its kb-root-relative path, its text as the batch holds it, and
// whether the batch already edits it.
func (b *recordBatch) eachDocument(visit func(rel string, l *leafEdit, opened bool) error) error {
	docs, err := kb.CitingDocuments(b.src)
	if err != nil {
		return err
	}
	for rel, l := range b.leaves {
		if l.disk == nil && !slices.Contains(docs, rel) {
			docs = append(docs, rel)
		}
	}
	slices.Sort(docs)
	for _, rel := range docs {
		l, opened := b.leaves[rel]
		if !opened {
			if l, err = b.open(rel); err != nil {
				return err
			}
		}
		if err := visit(rel, l, opened); err != nil {
			return err
		}
	}
	return nil
}

// rewriteDocuments gives every document of eachDocument the text rewrite
// returns for it, editing each it changes.
func (b *recordBatch) rewriteDocuments(rewrite func(rel, text string) (string, error)) error {
	return b.eachDocument(func(rel string, l *leafEdit, opened bool) error {
		text, err := rewrite(rel, l.text)
		if err != nil || text == l.text {
			return err
		}
		if !opened {
			if _, err := b.subject(rel); err != nil {
				return err
			}
			b.edits(rel, l)
		}
		l.text = text
		return nil
	})
}

// rewriteLinks is text with each line holding one of links, text's links in
// text order, as rewrite returns it, given the line as rewritten so far and
// each of its links, the last first, so that every link's offsets still hold.
func rewriteLinks(text string, links []kb.TextLink, rewrite func(line string, l kb.TextLink) (string, error)) (string, error) {
	lines := kb.SplitLines(text)
	rewritten := slices.Clone(lines)
	for i := len(links) - 1; i >= 0; i-- {
		line, err := rewrite(rewritten[links[i].Line], links[i])
		if err != nil {
			return "", err
		}
		rewritten[links[i].Line] = line
	}
	return spliceChanged(text, lines, rewritten), nil
}

// spliceChanged is text, whose lines are lines, with each line that rewritten
// changes replaced by rewritten's, every other line keeping its bytes.
func spliceChanged(text string, lines, rewritten []string) string {
	for n, line := range rewritten {
		if line != lines[n] {
			text = spliceLines(text, n, n+1, []string{line})
		}
	}
	return text
}

// renameCitations gives each citation of id in the naming form — its link
// text, collapsed, old — the link text title, in every document but the
// summary pages. It is refused at title where a rewritten link would not
// read, whole, as one link to its destination.
func (b *recordBatch) renameCitations(id, old, title string) error {
	return b.rewriteDocuments(func(rel, text string) (string, error) {
		return rewriteLinks(text, kb.TextCitations(text), func(line string, c kb.TextLink) (string, error) {
			if c.Title != id || collapse(c.Text) != old {
				return line, nil
			}
			link := "[" + title + line[c.Start+1+len(c.Text):c.End]
			if !readsAsOneLink(link, c.Destination) {
				return "", opRefusal("title", fmt.Sprintf("composes %s at %s:%d, which the toolchain's link reader does not see "+
					"as one link to %s — an unbalanced '[' or ']' in the title hides the citation from every gate that reads links",
					link, rel, c.Line+1, c.Destination), "restore: give a title whose brackets balance, then re-run")
			}
			return line[:c.Start] + link + line[c.End:], nil
		})
	})
}

// repoint gives each link outside code, a citation or not, an inline link
// (kb.TextLinks) or a reference definition (kb.TextLinkDefinitions), in every
// document but the summary pages, the destination dest returns for it, given
// the document's kb-root-relative path; dest returns the link's own
// destination to leave it standing.
func (b *recordBatch) repoint(dest func(rel string, l kb.TextLink) string) error {
	return b.rewriteDocuments(func(rel, text string) (string, error) {
		links := append(kb.TextLinks(text), kb.TextLinkDefinitions(text)...)
		slices.SortFunc(links, func(a, b kb.TextLink) int { return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Start, b.Start)) })
		return rewriteLinks(text, links, func(line string, l kb.TextLink) (string, error) {
			return line[:l.DestStart] + dest(rel, l) + line[l.DestEnd:], nil
		})
	})
}

// retargeted is the destination dest, written in the document at rel, pointed
// at to instead, keeping its <…> wrapping.
func retargeted(rel, dest string, to citeTarget) string {
	repointed := index.RelPosix(to.path, path.Dir(rel)) + "#" + to.anchor
	if strings.HasPrefix(dest, "<") {
		repointed = "<" + repointed + ">"
	}
	return repointed
}

// leafTexts is the text of each document the batch edits, by kb-root-relative
// path.
func (b *recordBatch) leafTexts() map[string]string {
	texts := make(map[string]string, len(b.leaves))
	for rel, l := range b.leaves {
		texts[rel] = l.text
	}
	return texts
}

// movedAnchors maps each heading an entry moved, as the target its anchor
// gave before the entry, to its anchor after it.
type movedAnchors map[citeTarget]string

// movedHeadings is every heading of a document the batch edits whose anchor
// differs from the one it had when the batch's texts were before, a document
// absent from before standing as the batch read it. It compares headings by
// line, so it holds for an entry that rewrites lines in place.
func (b *recordBatch) movedHeadings(before map[string]string) movedAnchors {
	moved := movedAnchors{}
	for rel, l := range b.leaves {
		was, ok := before[rel]
		if !ok {
			was = l.read
		}
		if was == l.text {
			continue
		}
		now := kb.HeadingAnchors(l.text)
		for line, anchor := range kb.HeadingAnchors(was) {
			if a, ok := now[line]; ok && a != anchor {
				moved[citeTarget{rel, anchor}] = a
			}
		}
	}
	return moved
}

// summaries adds to m each summary entry whose anchor moved from was to now,
// both by node id.
func (m movedAnchors) summaries(was, now map[string]citeTarget) {
	for id, t := range was {
		if a := now[id]; a.path == t.path && a.anchor != t.anchor {
			m[t] = a.anchor
		}
	}
}

// dest is the destination dest, written in the document at rel, with its
// fragment the moved anchor where it points at a moved heading, its path and
// its <…> wrapping kept.
func (m movedAnchors) dest(rel, dest string) string {
	at := strings.Index(dest, "#")
	anchor, ok := m[pointsAt(rel, dest)]
	if at < 0 || !ok {
		return dest
	}
	if strings.HasPrefix(dest, "<") {
		anchor += ">"
	}
	return dest[:at+1] + anchor
}

// pointsAt is where the link destination dest, written in the document at
// rel, points.
func pointsAt(rel, dest string) citeTarget {
	_, anchor, _ := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(dest, "<"), ">"), "#")
	target := citeTarget{path: rel, anchor: anchor}
	if p := kb.StripTarget(dest); p != "" {
		target.path = kb.ResolveLink(rel, p)
	}
	return target
}
