package write

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/argmark"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// listEdge is one edge a list of the build's edge write authors on its node:
// the values key listing it, its relation, the other end, whether it runs into
// the node, its context, its bearing, and whether the caller supplied the
// bearing.
type listEdge struct {
	key, relation, other, context string
	inbound                       bool
	bearing                       kb.Bearing
	judged                        bool
}

// ends is the edge's source and target on node.
func (x listEdge) ends(node string) (source, target string) {
	if x.inbound {
		return x.other, node
	}
	return node, x.other
}

// listEdges is the edges e lists under its rests-on and references keys on a
// node of nodeKind, in list order: a premise founds the node where its source
// may found one, and supports it otherwise; a references entry is the node's
// reference to it.
func listEdges(e entry, nodeKind string) []listEdge {
	var out []listEdge
	for _, v := range e.restsOn(kb.ListRestsOn) {
		sourceKind, _ := kb.NodeIDKind(v.ID)
		relation := kb.RelationSupports
		if kb.BearingLegal(kb.RelationFounds, sourceKind, nodeKind) {
			relation = kb.RelationFounds
		}
		out = append(out, listEdge{key: kb.ListRestsOn, relation: relation, other: v.ID, context: collapse(v.Context),
			inbound: true, bearing: bearingOf(v.Bearing), judged: v.Judged})
	}
	for _, v := range e.restsOn(kb.RelationReferences) {
		out = append(out, listEdge{key: kb.RelationReferences, relation: kb.RelationReferences, other: v.ID, context: collapse(v.Context)})
	}
	return out
}

// resolveEnds refuses an edge whose other end names no node record.
func (b *recordBatch) resolveEnds(edges []listEdge) error {
	kinds := index.GraphKinds(b.g)
	for _, x := range edges {
		if _, ok := kinds[x.other]; !ok {
			return result.Refusal{{Check: kb.CheckUnknownID, Key: x.key + ".id=" + x.other,
				Remedy: fmt.Sprintf("create %s's node with the add op for its kind, or correct %s.id, then re-run", x.other, x.key),
				Detail: "names no node record. Every id-valued field must name a node that already exists at write time"}}
		}
	}
	return nil
}

// mint is a fresh id of prefix, clear of every node record the batch holds.
func (b *recordBatch) mint(prefix string) string {
	kinds := index.GraphKinds(b.g)
	id := kb.MintID(prefix, func(id string) bool {
		_, taken := kinds[id]
		return taken
	})
	b.report = entryReport{ID: id}
	return id
}

// judged is the manually-scored flag of a rigor or a bearing an op writes:
// true where the values supplied it and a caller's values they are.
func (b *recordBatch) judged(supplied bool) bool { return supplied && b.provenance == ProvenanceCaller }

// holds is whether an edge record of relation joins source to target.
func (b *recordBatch) holds(relation, source, target string) bool {
	switch relation {
	case kb.RelationReferences:
		return slices.ContainsFunc(b.g.References, func(r kb.UnscoredEdge) bool { return r.Source == source && r.Target == target })
	case kb.RelationFounds:
		return slices.ContainsFunc(b.g.Founds, func(r kb.BearingEdge) bool { return r.Source == source && r.Target == target })
	}
	return slices.ContainsFunc(b.g.Scorable, func(r kb.ScorableEdge) bool {
		return r.Relation == relation && r.Source == source && r.Target == target
	})
}

// closesCycle is the cycle a scoring edge source → target would close among
// the batch's scoring edges, in edge direction: source → target → … → source.
// It is nil where the edge closes none, in which case the edge joins the
// graph later checks read. That graph is kept across the batch's entries, so
// an op taking a scoring edge out of the records clears b.depends.
func (b *recordBatch) closesCycle(source, target string) []string {
	if b.depends == nil {
		b.depends = index.GraphDependsGraph(b.g)
	}
	if path := b.depends.Path(source, target); path != nil {
		cycle := append(path, source)
		slices.Reverse(cycle)
		return cycle
	}
	b.depends.Add(target, source)
	return nil
}

// addEdges appends edges on node to the records, refusing one naming an edge
// an earlier one of edges names with another context or bearing — the same
// edge twice is one record. An edge a record already carries is left as it
// stands. An edge a demoted record of its relation stands for is written with
// these values, and that record deleted.
func (b *recordBatch) addEdges(e entry, node string, edges []listEdge) error {
	added := map[string]listEdge{}
	for _, x := range edges {
		source, target := x.ends(node)
		name := index.EdgeName(x.relation, source, target)
		if first, twice := added[name]; twice {
			if first.context != x.context || first.bearing != x.bearing {
				return result.Refusal{{Key: x.key, Entry: e.Index, Remedy: "name the edge once, then re-run",
					Detail: fmt.Sprintf("names %s twice, with another context or bearing; an edge is one record", name)}}
			}
			continue
		}
		added[name] = x
		if b.holds(x.relation, source, target) {
			continue
		}
		if x.relation == kb.RelationReferences {
			b.g.References = append(b.g.References, kb.UnscoredEdge{Source: source, Target: target, Context: x.context})
			continue
		}
		if x.relation == kb.RelationFounds {
			b.g.Founds = append(b.g.Founds, kb.BearingEdge{Source: source, Target: target, Context: x.context, Bearing: x.bearing})
			continue
		}
		b.g.Demoted = slices.DeleteFunc(b.g.Demoted, kb.DemotedMatch(x.relation, source, target))
		b.g.Scorable = append(b.g.Scorable, kb.ScorableEdge{Relation: x.relation, Source: source, Target: target,
			Context: x.context, Bearing: x.bearing, ManuallyScored: b.judged(x.judged)})
		if x.judged {
			b.valueSet(scoredRecord{relation: x.relation, source: source, target: target}, x.bearing.Pending)
		}
	}
	return nil
}

// subject is document's kb-root-relative path, refused where it lies outside
// kb-root.
func (b *recordBatch) subject(document string) (string, error) {
	target, err := containedIn(b.root, document, "document")
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(b.root, target)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// leaf is the batch's leaf named by document, opened on first use; a document
// that does not exist or is no leaf of the KB is refused.
func (b *recordBatch) leaf(document string) (string, *leafEdit, error) {
	subject, err := b.subject(document)
	if err != nil {
		return "", nil, err
	}
	if !b.src.IsFile(filepath.Join(b.root, filepath.FromSlash(subject))) {
		return "", nil, opRefusal("document", fmt.Sprintf("%q does not exist. A marker is placed beside the prose it "+
			"anchors, and this API never composes a document body", subject),
			"restore: correct document, or author the document first, then re-run")
	}
	l, err := b.leafAt(subject)
	if err != nil {
		return "", nil, err
	}
	if l == nil {
		return "", nil, opRefusal("document", fmt.Sprintf("%q is no leaf of the KB, and only a leaf's statement marker "+
			"hosts a node", subject),
			"restore: name the leaf the node's statement stands in, then re-run")
	}
	return subject, l, nil
}

// knownDocuments is every document of the KB, kb-root-relative, read once.
func (b *recordBatch) knownDocuments() ([]string, error) {
	if b.documents == nil {
		var err error
		if b.documents, err = kb.Documents(b.src); err != nil {
			return nil, err
		}
	}
	return b.documents, nil
}

// leafAt is the batch's leaf at the kb-root-relative subject, opened on first
// use; nil where subject is no leaf of the KB, the batch editing it or not —
// a title edit rewrites citations in documents of every kind.
func (b *recordBatch) leafAt(subject string) (*leafEdit, error) {
	documents, err := b.knownDocuments()
	if err != nil || !slices.Contains(documents, subject) {
		return nil, err
	}
	l, opened := b.leaves[subject]
	if !opened {
		if l, err = b.open(subject); err != nil {
			return nil, err
		}
	}
	if kb.DocumentKind(subject, len(kb.ChildList(subject, l.read)) > 0) != kb.DocumentLeaf {
		return nil, nil
	}
	if !opened {
		b.edits(subject, l)
	}
	return l, nil
}

// open is the document at the kb-root-relative subject as the batch reads it,
// "" where it stands absent.
func (b *recordBatch) open(subject string) (*leafEdit, error) {
	target := filepath.Join(b.root, filepath.FromSlash(subject))
	disk, err := onDisk(b.src, target)
	if err != nil {
		return nil, err
	}
	if disk == nil {
		return &leafEdit{target: target}, nil
	}
	text, err := readExact(b.src, target, subject)
	if err != nil {
		return nil, err
	}
	return &leafEdit{target: target, disk: disk, read: text, text: text}, nil
}

// edits records that the batch edits the document at subject.
func (b *recordBatch) edits(subject string, l *leafEdit) {
	b.leaves[subject] = l
	b.leafOrder = append(b.leafOrder, subject)
}

// marked records that the batch placed id's marker in the leaf at subject,
// which now hosts it.
func (b *recordBatch) marked(subject string, l *leafEdit, id string) {
	l.marked = append(l.marked, id)
	b.hosts[id] = subject
}

// markAtLine appends a node's marker to the 1-based line of the leaf document
// names.
func (b *recordBatch) markAtLine(document string, line int, id string) error {
	subject, l, err := b.leaf(document)
	if err != nil {
		return err
	}
	l.text = appendMarker(l.text, line-1, kb.StatementMarker(id))
	b.marked(subject, l, id)
	return nil
}

// moveMarker takes id's marker off the line it stands on in the leaf document
// names and puts it back after the first end bytes of the 1-based line, as
// that line stands with the marker off; a line of 0 means the marker's own
// line. The caller passes an end that lies before every other marker on the
// target line.
func (b *recordBatch) moveMarker(document, id string, line, end int) error {
	subject, l, err := b.leaf(document)
	if err != nil {
		return err
	}
	at := kb.MarkerLines(l.text)[id]
	marker := kb.StatementMarker(id)
	l.text = spliceLines(l.text, at, at+1, []string{unmarkLine(kb.SplitLines(l.text)[at], marker)})
	if line > 0 {
		at = line - 1
	}
	target := kb.SplitLines(l.text)[at]
	l.text = spliceLines(l.text, at, at+1, []string{target[:end] + marker + target[end:]})
	b.marked(subject, l, id)
	return nil
}

// nearbyLimit is how many of the lines holding a snippet an add's refusal
// names: those nearest the line the entry named.
const nearbyLimit = 5

// nodeExists refuses an add of a node that stands, naming it.
func nodeExists(key, id string) error {
	return result.Refusal{{Key: key, Detail: "node exists: " + id,
		Remedy: fmt.Sprintf("%s stands with this %s: where it is the node you meant, drop this entry and use %s wherever you would have "+
			"named the node it adds (%s reads it); where you meant a different node, give a %s no node of its kind carries; then re-run",
			id, key, id, argmark.Call("show", map[string]any{"id": id}), key)}}
}

// checkNamedLine refuses the line an add's document, line and snippet name
// where it lies past the document's end, opens or closes a fenced block, or
// does not contain the snippet; that last refusal names the nearby lines that
// do, since the document has likely moved since the caller read it.
func (b *recordBatch) checkNamedLine(e entry) error {
	line, snippet := e.count("line"), e.str("snippet")
	subject, l, err := b.leaf(e.str("document"))
	if err != nil {
		return err
	}
	lines := kb.SplitLines(l.text)
	if line > len(lines) {
		return lineOutOfRange(subject, line, len(lines))
	}
	at := line - 1
	for _, f := range kb.Fences(lines) {
		if at == f.Start || f.Closed && at == f.End-1 {
			return opRefusal("line", fmt.Sprintf("line %d of %s opens or closes a fenced block, and no marker is placed there", line, subject),
				"restore: name another line, then re-run")
		}
	}
	if strings.Contains(collapse(lines[at]), snippet) {
		return nil
	}
	return snippetAbsent(subject, line, len(lines), snippet, "", func(i int) bool { return strings.Contains(collapse(lines[i]), snippet) })
}

// lineOutOfRange refuses, at line, a 1-based line past the end of subject, a
// document of count lines.
func lineOutOfRange(subject string, line, count int) error {
	return opRefusal("line", fmt.Sprintf("line %d is past the end of %s, which has %d lines", line, subject, count),
		"restore: re-read the document and name a line in it, then re-run")
}

// noStatementMarker refuses, at id, a hosted node whose statement carries no
// marker; consequence says what the missing marker leaves undone.
func noStatementMarker(id, consequence string) error {
	return result.Refusal{{Check: index.CheckStatementMarker, Key: editID.name,
		Remedy: "write its marker, " + kb.StatementMarker(id) + ", at the end of its statement's line, then re-run",
		Detail: id + " carries no statement marker, so " + consequence}}
}

// snippetAbsent refuses, at snippet, the 1-based line of subject, a document
// of count lines, as not holding snippet where it stands, naming the nearby
// lines that do by holds, over 0-based lines; qualifier follows the snippet in
// the detail. The document has likely moved since the caller read it.
func snippetAbsent(subject string, line, count int, snippet, qualifier string, holds func(i int) bool) error {
	var holding []int
	for i := range count {
		if holds(i) {
			holding = append(holding, i+1)
		}
	}
	slices.SortStableFunc(holding, func(x, y int) int { return cmp.Compare(max(x-line, line-x), max(y-line, line-y)) })
	holding = holding[:min(len(holding), nearbyLimit)]
	slices.Sort(holding)
	where := "it appears on no line of " + subject
	if len(holding) > 0 {
		where = "it appears on line " + joinInts(holding)
		if len(holding) > 1 {
			where = "it appears on lines " + joinInts(holding)
		}
	}
	return opRefusal("snippet", fmt.Sprintf("line %d of %s does not contain %q%s; %s. The document has likely changed since it was read",
		line, subject, snippet, qualifier, where), "restore: re-read the document and name the line holding the snippet, then re-run")
}

// joinInts is ns written as a comma-separated list.
func joinInts(ns []int) string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = strconv.Itoa(n)
	}
	return strings.Join(out, ", ")
}

// addHosted adds a claim, an analysis or an experiment: refused where held, the
// id of the node of its kind carrying its title, is not "", or where its named
// line is refused; otherwise it mints an id of prefix, writes the record
// through add, and places the statement marker at the end of the named line,
// re-pointing every link into a summary entry the new one shifts
// (keepSummaryLinks).
func (b *recordBatch) addHosted(e entry, prefix, held string, add func(id string)) error {
	if held != "" {
		return nodeExists("title", held)
	}
	if err := b.checkNamedLine(e); err != nil {
		return err
	}
	return b.keepSummaryLinks("", e.str("title"), func() error {
		id := b.mint(prefix)
		add(id)
		b.rigorSet(e, id)
		return b.markAtLine(e.str("document"), e.count("line"), id)
	})
}

func addClaim(b *recordBatch, e entry) error {
	return b.addHosted(e, kb.ClaimPrefix, b.titleHolder(kb.NodeKindClaim, e.str("title")), func(id string) {
		b.g.Claims = append(b.g.Claims, kb.ClaimNode{ID: id, Title: e.str("title"), Rigor: e.score("rigor"),
			ManuallyScored: b.judged(e.has("rigor")), Rationale: e.str("rationale")})
	})
}

func addAnalysis(b *recordBatch, e entry) error {
	return b.addHosted(e, kb.AnalysisPrefix, b.titleHolder(kb.NodeKindAnalysis, e.str("title")), func(id string) {
		b.g.Analyses = append(b.g.Analyses, kb.AnalysisNode{ID: id, Title: e.str("title"), Rigor: e.score("rigor"),
			ManuallyScored: b.judged(e.has("rigor")), Rationale: e.str("rationale")})
	})
}

func addExperiment(b *recordBatch, e entry) error {
	return b.addHosted(e, kb.ExperimentPrefix, b.titleHolder(kb.NodeKindExperiment, e.str("title")), func(id string) {
		b.g.Experiments = append(b.g.Experiments, kb.ExperimentNode{ID: id, Title: e.str("title"), Status: e.str("status"),
			Rigor: e.score("rigor"), ManuallyScored: b.judged(e.has("rigor"))})
	})
}

// addWork mints a work's id and records it, refused where a work of that
// label or that title stands, re-pointing every link into a summary entry the
// new one shifts (keepSummaryLinks).
func addWork(b *recordBatch, e entry) error {
	n := kb.WorkNode{Label: e.str("label"), Title: e.str("title"), Rigor: e.score("rigor"), Rationale: e.str("rationale")}
	if held := b.labelHolder(kb.NodeKindWork, n.Label); held != "" {
		return nodeExists("label", held)
	}
	if held := b.titleHolder(kb.NodeKindWork, n.Title); held != "" {
		return nodeExists("title", held)
	}
	return b.keepSummaryLinks("", n.Title, func() error {
		n.ID = b.mint(kb.WorkPrefix)
		b.g.Works = append(b.g.Works, n)
		return nil
	})
}

// The heading an add-axiom reports: an unmarked heading of its text took its
// marker, or its heading was appended.
const (
	axiomHeadingMarked   = "marked"
	axiomHeadingAppended = "appended"
)

// addAxiom mints an axiom's id and records it, refused where an axiom of that
// label or that title stands, and gives it its marked heading in axioms.md:
// the one unmarked heading of its text takes its marker, and where none
// stands the heading is appended. More than one unmarked heading of its text
// is refused, naming their lines.
func addAxiom(b *recordBatch, e entry) error {
	n := kb.AxiomNode{Label: e.str("label"), Title: e.str("title")}
	if held := b.labelHolder(kb.NodeKindAxiom, n.Label); held != "" {
		return nodeExists("label", held)
	}
	if held := b.titleHolder(kb.NodeKindAxiom, n.Title); held != "" {
		return nodeExists("title", held)
	}
	l, err := b.axiomsDocument()
	if err != nil {
		return err
	}
	text := kb.AxiomHeadingText(n.Label, n.Title)
	var unmarked []int
	for _, h := range kb.AxiomHeadings(l.text) {
		if h.ID == "" && h.Text == text {
			unmarked = append(unmarked, h.Line)
		}
	}
	if len(unmarked) > 1 {
		return result.Refusal{{Key: "label", Path: kb.AxiomsFile, Line: unmarked[0],
			Remedy: fmt.Sprintf("reword or remove all but one of the headings %q in %s, on lines %s, then re-run", text, kb.AxiomsFile, joinInts(unmarked)),
			Detail: fmt.Sprintf("%s holds %d unmarked headings %q, on lines %s, and kbase does not guess which one the axiom is",
				kb.AxiomsFile, len(unmarked), text, joinInts(unmarked))}}
	}
	n.ID = b.mint(kb.AxiomPrefix)
	b.g.Axioms = append(b.g.Axioms, n)
	if len(unmarked) == 1 {
		at := unmarked[0] - 1
		line := kb.SplitLines(l.text)[at]
		l.text = spliceLines(l.text, at, at+1, []string{strings.TrimRight(line, " \t\r") + kb.StatementMarker(n.ID)})
		b.report.Heading = axiomHeadingMarked
		return nil
	}
	appendAxiomHeading(l, n.Label, n.Title, n.ID)
	b.report.Heading = axiomHeadingAppended
	return nil
}

// axiomsDocument is axioms.md as the batch edits it, opened on first use, ""
// where it stands absent.
func (b *recordBatch) axiomsDocument() (*leafEdit, error) {
	if l, ok := b.leaves[kb.AxiomsFile]; ok {
		return l, nil
	}
	if _, err := b.subject(kb.AxiomsFile); err != nil {
		return nil, err
	}
	l, err := b.open(kb.AxiomsFile)
	if err != nil {
		return nil, err
	}
	b.edits(kb.AxiomsFile, l)
	return l, nil
}

// appendAxiomHeading appends the marked heading of the axiom id of label and
// title to axioms.md's text l, after a blank line where its last line is not
// blank.
func appendAxiomHeading(l *leafEdit, label, title, id string) {
	lines := kb.SplitLines(l.text)
	heading := []string{kb.AxiomHeadingLine(label, title, id)}
	if len(lines) > 0 && kb.Strip(lines[len(lines)-1]) != "" {
		heading = append([]string{""}, heading...)
	}
	l.text = spliceLines(l.text, len(lines), len(lines), heading)
}
