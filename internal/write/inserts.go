package write

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// insertEdge is one scoring edge an insert's values author on its node: the
// values key listing it, its relation, the other end, whether it runs into the
// node, its context, its bearing, and whether the caller supplied the bearing.
type insertEdge struct {
	key, relation, other, context string
	inbound                       bool
	bearing                       kb.Bearing
	judged                        bool
}

// ends is the edge's source and target on node.
func (x insertEdge) ends(node string) (source, target string) {
	if x.inbound {
		return x.other, node
	}
	return node, x.other
}

// insertLists is each edge list an insert takes: whether its edges run into
// the node, and the relations they carry.
var insertLists = map[string]struct {
	inbound   bool
	relations []string
}{
	kb.ListRestsOn: {true, []string{kb.RelationSupports, kb.RelationFounds}},
	kb.SupportsKey: {false, []string{kb.RelationSupports}},
	kb.LiftsKey:    {false, []string{kb.RelationLifts}},
	kb.RefutesKey:  {false, []string{kb.RelationRefutes}},
}

// listEdges is the edges e lists under each of keys on a node of nodeKind, in
// list order: a premise founds the node where its source may found one, and
// supports it otherwise; a references entry is the node's reference to it.
func listEdges(e entry, nodeKind string, keys ...string) []insertEdge {
	var out []insertEdge
	for _, key := range keys {
		switch key {
		case kb.ListRestsOn:
			for _, v := range e.restsOn(key) {
				sourceKind, _ := kb.NodeIDKind(v.ID)
				relation := kb.RelationSupports
				if kb.BearingLegal(kb.RelationFounds, sourceKind, nodeKind) {
					relation = kb.RelationFounds
				}
				out = append(out, insertEdge{key: key, relation: relation, other: v.ID, context: collapse(v.Context),
					inbound: true, bearing: bearingOf(v.Bearing), judged: v.Judged})
			}
			continue
		case kb.RelationReferences:
			for _, v := range e.restsOn(key) {
				out = append(out, insertEdge{key: key, relation: kb.RelationReferences, other: v.ID, context: collapse(v.Context)})
			}
			continue
		}
		for _, p := range e.pairs(key) {
			out = append(out, insertEdge{key: key, relation: insertLists[key].relations[0], other: p.ID, context: collapse(p.Context),
				bearing: bearingOf(p.Score), judged: p.Judged})
		}
	}
	return out
}

// resolveEnds refuses an edge whose other end names no node record.
func (b *recordBatch) resolveEnds(edges []insertEdge) error {
	kinds := index.GraphKinds(b.g)
	for _, x := range edges {
		if _, ok := kinds[x.other]; !ok {
			return result.Refusal{{Check: kb.CheckUnknownID, Key: x.key + ".id=" + x.other,
				Remedy: fmt.Sprintf("create %s's node with the insert op for its kind, or correct %s.id, then re-run", x.other, x.key),
				Detail: "names no node record. Every id-valued field must name a node that already exists at write time"}}
		}
	}
	return nil
}

// heldEdges is node's records of the edges the list under key carries.
func (b *recordBatch) heldEdges(node, key string) []insertEdge {
	list := insertLists[key]
	var out []insertEdge
	for _, r := range b.g.Edges() {
		if !kb.Scoring(r.Relation) || !slices.Contains(list.relations, r.Relation) {
			continue
		}
		switch {
		case list.inbound && r.Target == node:
			out = append(out, insertEdge{key: key, relation: r.Relation, other: r.Source, context: r.Context, inbound: true, bearing: *r.Bearing})
		case !list.inbound && r.Source == node:
			out = append(out, insertEdge{key: key, relation: r.Relation, other: r.Target, context: r.Context, bearing: *r.Bearing})
		}
	}
	return out
}

// holdsList is whether node's records hold exactly the edges of edges listed
// under key, in any order, whoever judged their bearings; an edge whose bearing
// the values left out matches its record at any bearing.
func (b *recordBatch) holdsList(node, key string, edges []insertEdge) bool {
	name := func(x insertEdge) string { return strings.Join([]string{x.relation, x.other, x.context}, "\x00") }
	held := map[string]kb.Bearing{}
	for _, x := range b.heldEdges(node, key) {
		held[name(x)] = x.bearing
	}
	listed := map[string]bool{}
	for _, x := range edges {
		if x.key != key {
			continue
		}
		bearing, ok := held[name(x)]
		if !ok || x.judged && kb.BearingText(bearing) != kb.BearingText(x.bearing) {
			return false
		}
		listed[name(x)] = true
	}
	return len(listed) == len(held)
}

// valueMatch is whether a node carries the value a values key supplies.
type valueMatch struct {
	key   string
	equal bool
}

// differing is the keys of matches e supplies whose value the node does not
// carry, in order.
func differing(e entry, matches []valueMatch) []string {
	var out []string
	for _, m := range matches {
		if e.has(m.key) && !m.equal {
			out = append(out, m.key)
		}
	}
	return out
}

// collapsedList is items each collapsed, nil where there are none.
func collapsedList(items []string) []string {
	var out []string
	for _, it := range items {
		out = append(out, collapse(it))
	}
	return out
}

// adopt records an insert entry that landed on a node already there, with the
// supplied values that node does not carry: an insert does not update.
func (b *recordBatch) adopt(e entry, id string, differs []string) {
	b.ids = append(b.ids, id)
	b.adopted = append(b.adopted, Adoption{Entry: e.Index, ID: id, Differs: append([]string{}, differs...)})
}

// mint is a fresh id of prefix, clear of every node record the batch holds.
func (b *recordBatch) mint(prefix string) string {
	kinds := index.GraphKinds(b.g)
	id := kb.MintID(prefix, func(id string) bool {
		_, taken := kinds[id]
		return taken
	})
	b.ids, b.minted = append(b.ids, id), append(b.minted, id)
	return id
}

// judged is the manually-scored flag of a rigor or a bearing an insert
// writes: true where the values supplied it and a caller's values they are.
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

// closesCycle is the path source → … → target that a scoring edge source →
// target would close among the batch's scoring edges, nil where it closes
// none, in which case the edge joins the graph later checks read.
func (b *recordBatch) closesCycle(source, target string) []string {
	if b.depends == nil {
		b.depends = index.GraphDependsGraph(b.g)
	}
	if path := b.depends.Path(source, target); path != nil {
		return path
	}
	b.depends.Add(target, source)
	return nil
}

// addEdges appends edges on node to the records, refusing one naming an edge
// an earlier one of edges names with another context or bearing — the same
// edge twice is one record — and, where refuseCycles, a scoring edge that
// would close a cycle. An edge a record already carries is left as it stands.
func (b *recordBatch) addEdges(e entry, node string, edges []insertEdge, refuseCycles bool) error {
	added := map[string]insertEdge{}
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
		if refuseCycles {
			if path := b.closesCycle(source, target); path != nil {
				return result.Refusal{{Check: checkDependencyCycle, Entry: e.Index, Key: x.key,
					Remedy: "drop the edge from the values, or remove an edge on the named path first, then re-run",
					Detail: fmt.Sprintf("%s would close the cycle %s", name, strings.Join(append(path, source), " → "))}}
			}
		}
		if x.relation == kb.RelationFounds {
			b.g.Founds = append(b.g.Founds, kb.BearingEdge{Source: source, Target: target, Context: x.context, Bearing: x.bearing})
			continue
		}
		b.g.Scorable = append(b.g.Scorable, kb.ScorableEdge{Relation: x.relation, Source: source, Target: target,
			Context: x.context, Bearing: x.bearing, ManuallyScored: b.judged(x.judged)})
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
// use; nil where subject is no leaf of the KB.
func (b *recordBatch) leafAt(subject string) (*leafEdit, error) {
	if l, ok := b.leaves[subject]; ok {
		return l, nil
	}
	documents, err := b.knownDocuments()
	if err != nil || !slices.Contains(documents, subject) {
		return nil, err
	}
	target := filepath.Join(b.root, filepath.FromSlash(subject))
	disk, err := onDisk(b.src, target)
	if err != nil {
		return nil, err
	}
	text, err := readExact(b.src, target, subject)
	if err != nil {
		return nil, err
	}
	if _, _, children := kb.TreeLinks(subject, text); kb.DocumentKind(subject, len(children) > 0) != kb.DocumentLeaf {
		return nil, nil
	}
	l := &leafEdit{target: target, disk: disk, read: text, text: text}
	b.leaves[subject] = l
	b.leafOrder = append(b.leafOrder, subject)
	return l, nil
}

// marked records that the batch placed id's marker in the leaf at subject,
// which now hosts it.
func (b *recordBatch) marked(subject string, l *leafEdit, id string) {
	l.marked = append(l.marked, id)
	b.hosts[id] = subject
}

// markAtLocator appends a claim's marker to the line locator names in the leaf
// document names.
func (b *recordBatch) markAtLocator(document, locator, id string) error {
	subject, l, err := b.leaf(document)
	if err != nil {
		return err
	}
	at, err := locateExcerpt(l.text, locator)
	if err != nil {
		return err
	}
	l.text = appendMarker(l.text, at, kb.StatementMarker(id))
	b.marked(subject, l, id)
	return nil
}

// markAfterHeading places an analysis's or an experiment's marker on a new
// line directly after the first heading of the leaf document names, so the
// node's statement is that heading's section.
func (b *recordBatch) markAfterHeading(document, id string) error {
	subject, l, err := b.leaf(document)
	if err != nil {
		return err
	}
	at := slices.IndexFunc(kb.SplitLines(kb.StripCodeSplitLines(l.text)), func(line string) bool { return kb.FirstHeading(line) != "" })
	if at < 0 {
		return opRefusal("document", fmt.Sprintf("%q has no heading, and an analysis's or an experiment's marker goes "+
			"on the line after its host's first heading, its statement being that heading's section", subject),
			"restore: give the document its heading, or name the leaf the node's statement stands in, then re-run")
	}
	l.text = spliceLines(l.text, at+1, at+1, []string{kb.StatementMarker(id)})
	b.marked(subject, l, id)
	return nil
}

// adoptByHost lands e on the node of candidates the host rule picks — given a
// document, one hosted there, else one with no host, which mark then marks
// there; given none, one with no host — taking the first no supplied value
// differs from, else the first. It returns "" where none is picked, for the
// caller to mint.
func (b *recordBatch) adoptByHost(e entry, candidates []string, differs map[string][]string, mark func(id string) error) (string, error) {
	pick := func(host string) string {
		first := ""
		for _, id := range candidates {
			if b.hosts[id] != host {
				continue
			}
			if len(differs[id]) == 0 {
				return id
			}
			if first == "" {
				first = id
			}
		}
		return first
	}
	if e.has("document") {
		subject, err := b.subject(e.str("document"))
		if err != nil {
			return "", err
		}
		if id := pick(subject); id != "" {
			b.adopt(e, id, differs[id])
			return id, nil
		}
	}
	id := pick("")
	if id == "" {
		return "", nil
	}
	if e.has("document") {
		if err := mark(id); err != nil {
			return "", err
		}
	}
	b.adopt(e, id, differs[id])
	return id, nil
}

// insertClaimNode adopts or mints a claim. Its marker is placed given a
// document and a locator together; one without the other is refused, naming
// the missing key.
func insertClaimNode(b *recordBatch, e entry) error {
	if e.has("document") != e.has("locator") {
		given, missing := "document", "locator"
		if e.has("locator") {
			given, missing = missing, given
		}
		return result.Refusal{{Key: missing, Entry: e.Index, Remedy: fmt.Sprintf("supply %s beside %s, or neither, then re-run", missing, given),
			Detail: fmt.Sprintf("is missing beside %s: a claim's marker goes on the line its locator names in its document, "+
				"so the two are given together", given)}}
	}
	keys := []string{kb.ListRestsOn, kb.RefutesKey}
	edges := listEdges(e, kb.NodeKindClaim, keys...)
	if err := b.resolveEnds(edges); err != nil {
		return err
	}
	want := kb.ClaimNode{Title: collapse(e.str("title")), Rigor: e.score("rigor"), ManuallyScored: b.judged(e.has("rigor")),
		Rationale: collapse(e.str("rationale")), NoEdge: collapse(e.str("no-edge")), StrengthenBy: collapsedList(e.list("strengthen-by"))}
	var candidates []string
	differs := map[string][]string{}
	for _, n := range b.g.Claims {
		if n.Title != want.Title {
			continue
		}
		candidates = append(candidates, n.ID)
		differs[n.ID] = differing(e, []valueMatch{{"rigor", scoreEqual(want.Rigor, n.Rigor)},
			{"rationale", want.Rationale == n.Rationale}, {kb.ListRestsOn, b.holdsList(n.ID, kb.ListRestsOn, edges)},
			{"no-edge", want.NoEdge == n.NoEdge}, {"strengthen-by", slices.Equal(want.StrengthenBy, n.StrengthenBy)},
			{kb.RefutesKey, b.holdsList(n.ID, kb.RefutesKey, edges)}})
	}
	mark := func(id string) error { return b.markAtLocator(e.str("document"), e.str("locator"), id) }
	if id, err := b.adoptByHost(e, candidates, differs, mark); id != "" || err != nil {
		return err
	}
	want.ID = b.mint(kb.ClaimPrefix)
	b.g.Claims = append(b.g.Claims, want)
	if err := b.addEdges(e, want.ID, edges, true); err != nil {
		return err
	}
	if e.has("document") {
		return mark(want.ID)
	}
	return nil
}

// insertAnalysisNode adopts or mints an analysis, its marker placed where a
// document is given.
func insertAnalysisNode(b *recordBatch, e entry) error {
	edges := listEdges(e, kb.NodeKindAnalysis, kb.ListRestsOn, kb.LiftsKey, kb.RefutesKey)
	if err := b.resolveEnds(edges); err != nil {
		return err
	}
	want := kb.AnalysisNode{Title: collapse(e.str("title")), Rigor: e.score("rigor"), ManuallyScored: b.judged(e.has("rigor")),
		Rationale: collapse(e.str("rationale")), NoEdge: collapse(e.str("no-edge"))}
	var candidates []string
	differs := map[string][]string{}
	for _, n := range b.g.Analyses {
		if n.Title != want.Title {
			continue
		}
		candidates = append(candidates, n.ID)
		differs[n.ID] = differing(e, []valueMatch{{"rigor", scoreEqual(want.Rigor, n.Rigor)},
			{"rationale", want.Rationale == n.Rationale}, {kb.ListRestsOn, b.holdsList(n.ID, kb.ListRestsOn, edges)},
			{"no-edge", want.NoEdge == n.NoEdge}, {kb.LiftsKey, b.holdsList(n.ID, kb.LiftsKey, edges)},
			{kb.RefutesKey, b.holdsList(n.ID, kb.RefutesKey, edges)}})
	}
	mark := func(id string) error { return b.markAfterHeading(e.str("document"), id) }
	if id, err := b.adoptByHost(e, candidates, differs, mark); id != "" || err != nil {
		return err
	}
	want.ID = b.mint(kb.AnalysisPrefix)
	b.g.Analyses = append(b.g.Analyses, want)
	if err := b.addEdges(e, want.ID, edges, true); err != nil {
		return err
	}
	if e.has("document") {
		return mark(want.ID)
	}
	return nil
}

// insertExperimentNode adopts an experiment of the title and status whose
// rigor and pair lists equal the supplied ones, a key left out not compared,
// or mints one, and places its marker in the document.
func insertExperimentNode(b *recordBatch, e entry) error {
	keys := []string{kb.SupportsKey, kb.LiftsKey, kb.RefutesKey}
	edges := listEdges(e, kb.NodeKindExperiment, keys...)
	if err := b.resolveEnds(edges); err != nil {
		return err
	}
	want := kb.ExperimentNode{Title: collapse(e.str("title")), Status: e.str("status"), Rigor: e.score("rigor"),
		ManuallyScored: b.judged(e.has("rigor"))}
	var candidates []string
	for _, n := range b.g.Experiments {
		matches := []valueMatch{{"rigor", scoreEqual(n.Rigor, want.Rigor)}}
		for _, key := range keys {
			matches = append(matches, valueMatch{key, b.holdsList(n.ID, key, edges)})
		}
		if n.Title == want.Title && n.Status == want.Status && len(differing(e, matches)) == 0 {
			candidates = append(candidates, n.ID)
		}
	}
	mark := func(id string) error { return b.markAfterHeading(e.str("document"), id) }
	if id, err := b.adoptByHost(e, candidates, nil, mark); id != "" || err != nil {
		return err
	}
	want.ID = b.mint(kb.ExperimentPrefix)
	b.g.Experiments = append(b.g.Experiments, want)
	if err := b.addEdges(e, want.ID, edges, true); err != nil {
		return err
	}
	return mark(want.ID)
}

// refuseReentry refuses an entry naming, by its values key, the node an
// earlier entry of the batch entered: the id is derived from the key, so a
// second entry would be a second node for one key.
func (b *recordBatch) refuseReentry(e entry, key, id string) error {
	first, twice := b.created[id]
	if !twice {
		return nil
	}
	return result.Refusal{{Key: key, Entry: e.Index, Remedy: "drop one of the two entries, then re-run",
		Detail: fmt.Sprintf("names %s, which entry %d of this batch enters; a second entry would be a second node for one %s",
			id, first, key)}}
}

// enterKeyed records that e entered the node id.
func (b *recordBatch) enterKeyed(e entry, id string) {
	b.created[id] = e.Index
	b.ids = append(b.ids, id)
}

// insertWorkNode adopts the work its citation key names, reporting the
// supplied values it does not carry, or enters it.
func insertWorkNode(b *recordBatch, e entry) error {
	want := kb.WorkNode{ID: kb.WorkPrefix + "-" + e.str("key"), Title: collapse(e.str("title")), Rigor: e.score("rigor"),
		Rationale: collapse(e.str("rationale"))}
	if err := b.refuseReentry(e, "key", want.ID); err != nil {
		return err
	}
	if n := record(b.g.Works, func(n kb.WorkNode) bool { return n.ID == want.ID }); n != nil {
		b.adopt(e, n.ID, differing(e, []valueMatch{{"title", want.Title == n.Title}, {"rigor", scoreEqual(want.Rigor, n.Rigor)},
			{"rationale", want.Rationale == n.Rationale}}))
		return nil
	}
	b.enterKeyed(e, want.ID)
	b.g.Works = append(b.g.Works, want)
	return nil
}

// insertAxiomNode adopts the axiom its label names, reporting a differing
// title, or enters it.
func insertAxiomNode(b *recordBatch, e entry) error {
	want := kb.AxiomNode{ID: kb.AxiomPrefix + "-" + e.str("label"), Title: collapse(e.str("title"))}
	if err := b.refuseReentry(e, "label", want.ID); err != nil {
		return err
	}
	if n := record(b.g.Axioms, func(n kb.AxiomNode) bool { return n.ID == want.ID }); n != nil {
		b.adopt(e, n.ID, differing(e, []valueMatch{{"title", want.Title == n.Title}}))
		return nil
	}
	b.enterKeyed(e, want.ID)
	b.g.Axioms = append(b.g.Axioms, want)
	return nil
}
