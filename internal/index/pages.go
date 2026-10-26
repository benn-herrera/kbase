package index

import (
	"bytes"
	"cmp"
	"maps"
	"path"
	"slices"
	"strings"

	"kbase/internal/kb"
)

// CheckRegisterFreshness is the fault class of a register page standing other
// than refresh renders it.
const CheckRegisterFreshness = "register freshness"

// registerPageHead opens every register page.
const registerPageHead = "# Claim register\n\n`kbase refresh` generates this file from the KB's records; it is never edited by hand.\n"

// The edge lists an entry carries beside rests-on, references and demoted,
// which kb's constants name.
const (
	listLiftedBy  = "lifted-by"
	listRefutedBy = "refuted-by"
)

// renderRegisterPages is every register page the KB src holds calls for, by
// kb-root-relative path: each rendered from the records and the statement
// markers in the leaves, never from a page standing. A KB state Derive refuses
// is its refusal.
func renderRegisterPages(src *kb.Source) (map[string][]byte, error) {
	x, err := loadDerivation(src)
	if err != nil {
		return nil, err
	}
	return x.registerPages(), nil
}

// RegisterFreshness is a refresh-fixable finding per register page — kb-root's
// and each volume's — that does not stand as renderRegisterPages renders it: a
// page called for that is missing or whose bytes differ, and a page standing
// that no node is rendered on. A KB state Derive refuses is its refusal.
func RegisterFreshness(src *kb.Source) ([]Finding, error) {
	x, err := loadDerivation(src)
	if err != nil {
		return nil, err
	}
	pages := x.registerPages()
	var out []Finding
	for _, rel := range x.pagePositions() {
		want, called := pages[rel]
		have, err := src.ReadFile(src.KBPath(rel))
		if err != nil && !kb.ErrNotExist(err) {
			return nil, err
		}
		var detail string
		switch standing := err == nil; {
		case called && !standing:
			detail = rel + " is missing"
		case !called && standing:
			detail = rel + " stands, and no node is rendered on it"
		case called && !bytes.Equal(have, want):
			detail = rel + " is not as refresh renders it"
		default:
			continue
		}
		out = append(out, Finding{Check: CheckRegisterFreshness, Path: rel, Detail: detail, RefreshFixable: true})
	}
	return out, nil
}

// RegisterPagePositions is every kb-root-relative path a register page can
// stand at, sorted: the files refresh renders or removes.
func RegisterPagePositions(src *kb.Source) ([]string, error) {
	d, err := readDocuments(src)
	if err != nil {
		return nil, refusal(err)
	}
	return derivation{d: d}.pagePositions(), nil
}

// pagePositions is every path a register page can stand at, sorted: kb-root's
// and each volume's.
func (x derivation) pagePositions() []string {
	out := []string{kb.RegisterFile}
	for dir := range x.d.volumes {
		out = append(out, volumePage(dir))
	}
	slices.Sort(out)
	return out
}

// pageSlugs is every page position and the slugs of the entries rendered on
// it, none where no node is.
func (x derivation) pageSlugs() map[string][]string {
	out := map[string][]string{}
	for _, p := range x.pagePositions() {
		out[p] = nil
	}
	titles := pageNodes(x.g)
	for page, ids := range x.pageEntries() {
		for _, id := range ids {
			out[page] = append(out[page], kb.Slugify(titles[id].title))
		}
	}
	return out
}

// HostedOrder is the nodes of g that src's documents host, in the order the
// register pages render them: the order BreakCycles walks them in.
func HostedOrder(src *kb.Source, g kb.Graph) ([]string, error) {
	d, err := readDocuments(src)
	if err != nil {
		return nil, refusal(err)
	}
	hosts, _ := kb.Hosts(g, d.leaves)
	return hostedOrder(d.children, hosts, leafMarkers(d.leaves)), nil
}

// hostedOrder is the ids hosts places, in reading order of their marker: the
// host's place in the spine walk over children, then the marker's line in
// markers, ids at one line by id. A host the walk does not reach follows every
// host it does, by path.
func hostedOrder(children map[string][]string, hosts map[string]string, markers map[string]map[string]int) []string {
	place := map[string]int{}
	for i, p := range kb.ReadingOrder(children) {
		place[p] = i
	}
	at := func(leaf string) int {
		if i, ok := place[leaf]; ok {
			return i
		}
		return len(place)
	}
	ids := slices.Collect(maps.Keys(hosts))
	slices.SortFunc(ids, func(a, b string) int {
		ha, hb := hosts[a], hosts[b]
		return cmp.Or(cmp.Compare(at(ha), at(hb)), strings.Compare(ha, hb),
			cmp.Compare(markers[ha][a], markers[hb][b]), strings.Compare(a, b))
	})
	return ids
}

// pageEntries is the ids rendered on each page, in page order: the hosted
// nodes in their hosted order, then the works by id.
func (x derivation) pageEntries() map[string][]string {
	var works []string
	for _, n := range x.g.Works {
		works = append(works, n.ID)
	}
	slices.Sort(works)
	out := map[string][]string{}
	for _, id := range slices.Concat(x.hosted, works) {
		page := x.rows.RegisterPages[id]
		out[page] = append(out[page], id)
	}
	return out
}

// pageNode is what an entry shows of one node record.
type pageNode struct {
	kind, title, status, rationale, noEdge string
	rigor                                  *float64
	manual                                 bool
	strengthenBy                           []string
}

// derived is whether the node carries a derived strength.
func (n pageNode) derived() bool {
	return n.kind == kb.NodeKindClaim || n.kind == kb.NodeKindAnalysis
}

// pageNodes is every node record of g as an entry shows it, by id.
func pageNodes(g kb.Graph) map[string]pageNode {
	out := map[string]pageNode{}
	for _, n := range g.Claims {
		out[n.ID] = pageNode{kind: kb.NodeKindClaim, title: n.Title, rationale: n.Rationale, noEdge: n.NoEdge,
			rigor: n.Rigor, manual: n.ManuallyScored, strengthenBy: n.StrengthenBy}
	}
	for _, n := range g.Analyses {
		out[n.ID] = pageNode{kind: kb.NodeKindAnalysis, title: n.Title, rationale: n.Rationale, noEdge: n.NoEdge,
			rigor: n.Rigor, manual: n.ManuallyScored}
	}
	for _, n := range g.Experiments {
		out[n.ID] = pageNode{kind: kb.NodeKindExperiment, title: n.Title, status: n.Status, rigor: n.Rigor, manual: n.ManuallyScored}
	}
	for _, n := range g.Works {
		out[n.ID] = pageNode{kind: kb.NodeKindWork, title: n.Title, rationale: n.Rationale, rigor: n.Rigor}
	}
	for _, n := range g.Axioms {
		out[n.ID] = pageNode{kind: kb.NodeKindAxiom, title: n.Title}
	}
	return out
}

// pageEdge is one edge as an entry lists it: the node at its other end, the relation a demoted edge replaced, its bearing where it
// carries one, its context, and whether it is a scoring edge, whose other
// end's derived strength the bullet shows.
type pageEdge struct {
	other, replaced, context string
	bearing                  *kb.Bearing
	scoring                  bool
}

// pageRenderer is what every entry of one derivation is rendered from: the
// node records, each node's lists of edges by list name, and the leaves whose
// statement markers name each node.
type pageRenderer struct {
	x      derivation
	nodes  map[string]pageNode
	lists  map[string]map[string][]pageEdge
	leaves map[string][]string
}

// registerPages is every page the derivation calls for, rendered.
func (x derivation) registerPages() map[string][]byte {
	p := newPageRenderer(x)
	out := map[string][]byte{}
	for page, ids := range x.pageEntries() {
		var b strings.Builder
		b.WriteString(registerPageHead)
		for _, id := range ids {
			b.WriteString("\n" + strings.Join(p.entry(page, id), "\n") + "\n")
		}
		out[page] = []byte(b.String())
	}
	return out
}

// newPageRenderer reads the resolved records' edges into the lists of the
// entry each appears on — every scoring and demoted edge its target's, every
// references edge its source's — each list in identity order.
func newPageRenderer(x derivation) pageRenderer {
	p := pageRenderer{x: x, nodes: pageNodes(x.g), lists: map[string]map[string][]pageEdge{}, leaves: map[string][]string{}}
	add := func(id, list string, e pageEdge) {
		if p.lists[id] == nil {
			p.lists[id] = map[string][]pageEdge{}
		}
		p.lists[id][list] = append(p.lists[id][list], e)
	}
	for _, e := range x.rows.Resolved.Edges() {
		edge := pageEdge{other: e.Source, replaced: e.ReplacedRelation, context: e.Context, bearing: e.Bearing,
			scoring: kb.Scoring(e.Relation)}
		switch e.Relation {
		case kb.RelationSupports, kb.RelationFounds:
			add(e.Target, kb.ListRestsOn, edge)
		case kb.RelationLifts:
			add(e.Target, listLiftedBy, edge)
		case kb.RelationRefutes:
			add(e.Target, listRefutedBy, edge)
		case kb.RelationDemoted:
			add(e.Target, kb.RelationDemoted, edge)
		case kb.RelationReferences:
			edge.other = e.Target
			add(e.Source, kb.RelationReferences, edge)
		}
	}
	for _, leaf := range slices.Sorted(maps.Keys(x.markers)) {
		for id := range x.markers[leaf] {
			p.leaves[id] = append(p.leaves[id], leaf)
		}
	}
	return p
}

// entry is node id's entry on page, as lines: its heading and id marker; but
// for a work, the leaf-references line; its rigor section — an experiment's
// status and rigor, a work's rigor and rationale, and a claim's or an
// analysis's rigor, edge lists, rationale, strengthen-by and no-edge — and a
// claim's or an analysis's strength section.
func (p pageRenderer) entry(page, id string) []string {
	n := p.nodes[id]
	lines := []string{"## " + n.title, kb.RenderIDMarker(id), ""}
	if n.kind != kb.NodeKindWork {
		lines = append(lines, RenderLeafReferences(page, p.leaves[id]), "")
	}
	lines = append(lines, "### Rigor")
	if n.kind == kb.NodeKindExperiment {
		lines = append(lines, "- "+kb.StatusKey+": "+n.status)
	}
	rigor := "- " + kb.RigorKey + ": " + rigorText(n.rigor)
	if n.manual {
		rigor += " (manual)"
	}
	lines = append(lines, rigor)
	if n.derived() {
		for _, list := range []string{kb.ListRestsOn, listLiftedBy, listRefutedBy, kb.RelationReferences, kb.RelationDemoted} {
			lines = append(lines, p.edgeList(page, list, p.lists[id][list])...)
		}
	}
	if n.rationale != "" {
		lines = append(lines, "- rationale: "+n.rationale)
	}
	if len(n.strengthenBy) > 0 {
		lines = append(lines, "- strengthen-by:")
		for _, item := range n.strengthenBy {
			lines = append(lines, "  - "+item)
		}
	}
	if n.noEdge != "" {
		lines = append(lines, "- no-edge: "+n.noEdge)
	}
	if n.derived() {
		r := p.result(id)
		lines = append(lines, "", "### Strength", strengthLine(r.Final, RenderStrengthTrace(r)))
	}
	return lines
}

// result is a claim's or an analysis's derived strength.
func (p pageRenderer) result(id string) Result {
	if r, ok := p.x.strength.Results[id]; ok {
		return r
	}
	return p.x.strength.AnalysisResults[id]
}

// edgeList is the list named list of edges as lines on page: its header and
// one bullet per edge — the other end's id, its title linked to its entry
// where it has one, a scoring edge's other end's derived strength where it
// carries one, a demoted edge's replaced relation, the bearing, the context.
// No edge, no lines.
func (p pageRenderer) edgeList(page, list string, edges []pageEdge) []string {
	if len(edges) == 0 {
		return nil
	}
	lines := []string{"- " + list + ":"}
	for _, e := range edges {
		line := "  - " + e.other + " — " + p.titleLink(page, e.other)
		if e.scoring && p.nodes[e.other].derived() {
			line += " " + kb.RenderStrengthAnnotation(FormatStrengthRecord(p.result(e.other).Final))
		}
		if e.replaced != "" {
			line += " (demoted from " + e.replaced + ")"
		}
		if e.bearing != nil {
			line += " " + kb.RenderBearingAnnotation(*e.bearing)
		}
		if e.context != "" {
			line += " [" + e.context + "]"
		}
		lines = append(lines, line)
	}
	return lines
}

// titleLink is node id's title, linked from page to its entry where the node is
// rendered on a page.
func (p pageRenderer) titleLink(page, id string) string {
	title := p.nodes[id].title
	to, ok := p.x.rows.RegisterPages[id]
	if !ok {
		return title
	}
	href := to
	if dir := path.Dir(page); dir != "." {
		href = RelPosix(to, dir)
	}
	return "[" + title + "](" + href + "#" + kb.Slugify(title) + ")"
}

// rigorText is an authored rigor as a bullet writes it: its value, or the
// pending literal where it has none.
func rigorText(v *float64) string {
	b := kb.Bearing{Pending: v == nil}
	if v != nil {
		b.Value = *v
	}
	return kb.BearingText(b)
}
