package index

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// Rows is what Derive yields for a KB: its rows, one set per row type, each in
// its own order, and where its nodes stand.
type Rows struct {
	Nodes []kb.NodeRow
	Edges []kb.EdgeRow
	Cites []kb.CiteRow
	// DocumentKinds is every document's kind, by kb-root-relative path.
	DocumentKinds map[string]string
	// Hosts is the leaf hosting each claim, analysis and experiment that
	// carries a statement marker; HostFaults is each one carrying none or
	// several.
	Hosts      map[string]string
	HostFaults []kb.HostFault
	// HostAnchors is, for each id in Hosts, the anchor of the section its
	// statement marker stands in.
	HostAnchors map[string]string
	// RegisterPages is the kb-root-relative register page each node is
	// rendered on: a hosted claim's, analysis's or experiment's volume page, or
	// the root page where its host lies in no volume, and the root page for
	// every work. An axiom and a node with no host are on none.
	RegisterPages map[string]string
	// RegisterAnchors is the anchor of each node's entry on its register
	// page, by id: an entry for every id RegisterPages places.
	RegisterAnchors map[string]string
	// Resolved is the records as the walk reads them: the KB's records, less
	// each scoring edge DeriveOverCycles cut to break a cycle.
	Resolved kb.Graph
}

// Derive is the rows of the KB src holds, derived from its records and the
// statement markers in its leaves. A load that fails is its result.Refusal,
// an item per fault: record files the strict load refuses, and a cycle among
// the scoring edges, one item per edge the walk would cut to break one.
func Derive(src *kb.Source) (Rows, error) {
	x, err := loadDerivation(src)
	return x.rows, err
}

// DeriveOverCycles is Derive over records whose scoring edges may hold a
// cycle, each cut for the walk: the build's read between the edge batch that
// closes a cycle and the ring row that demotes it. Every other load fault is
// refused as Derive refuses it.
func DeriveOverCycles(src *kb.Source) (Rows, error) {
	x, err := readDerivation(src)
	return x.rows, err
}

// loadDerivation is Derive's reading of the KB src holds.
func loadDerivation(src *kb.Source) (derivation, error) {
	x, err := readDerivation(src)
	if err == nil && len(x.cycles) > 0 {
		return derivation{}, result.Refusal(x.cycles)
	}
	return x, err
}

// readDerivation is DeriveOverCycles' reading of the KB src holds.
func readDerivation(src *kb.Source) (derivation, error) {
	g, err := kb.LoadGraph(src)
	if err != nil {
		return derivation{}, refusal(err)
	}
	docs, err := readDocuments(src)
	if err != nil {
		return derivation{}, refusal(err)
	}
	return deriveGraph(g, docs), nil
}

// documents is what Derive reads off a KB's documents: each document's kind,
// by its path and child list, and its children as TreeLinks reads them, by
// kb-root-relative path, each leaf's text, the volumes, and axioms.md's text,
// "" where it is absent.
type documents struct {
	kinds    map[string]string
	children map[string][]string
	leaves   map[string]string
	volumes  map[string]bool
	axioms   string
}

func readDocuments(src *kb.Source) (documents, error) {
	paths, err := kb.Documents(src)
	if err != nil {
		return documents{}, err
	}
	d := documents{kinds: map[string]string{}, children: map[string][]string{}, leaves: map[string]string{}, volumes: kb.Volumes(paths)}
	for _, rel := range paths {
		text, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			return documents{}, err
		}
		_, _, children := kb.TreeLinks(rel, text)
		kind := kb.DocumentKind(rel, len(kb.ChildList(rel, text)) > 0)
		d.kinds[rel] = kind
		d.children[rel] = children
		if kind == kb.DocumentLeaf {
			d.leaves[rel] = text
		}
	}
	if p := src.KBPath(kb.AxiomsFile); src.IsFile(p) {
		if d.axioms, err = src.ReadText(p); err != nil {
			return documents{}, err
		}
	}
	return d, nil
}

// derivation is one Derive's reading of a KB: its records and documents, every
// edge as the walk reads it, each leaf's first marker line per node id, the
// hosted nodes in their hosted order, each register entry's anchor on its
// page, the strength the walk derived over the resolved records, the rows and
// placements derived so far, and a load error per scoring edge cut to break a
// cycle.
type derivation struct {
	g        kb.Graph
	d        documents
	edges    []kb.Edge
	markers  map[string]map[string]int
	hosted   []string
	anchors  map[string]string
	strength Strength
	rows     Rows
	cycles   []result.Item
}

func deriveGraph(g kb.Graph, d documents) derivation {
	kinds := GraphKinds(g)
	x := derivation{g: g, d: d, edges: graphEdges(g, kinds), markers: leafMarkers(d.leaves)}
	r := &x.rows
	r.DocumentKinds = maps.Clone(d.kinds)
	r.Hosts, r.HostFaults = kb.Hosts(g, d.leaves)
	x.hosted = hostedOrder(d.children, r.Hosts, leafMarkerOffsets(d.leaves))
	r.RegisterPages = map[string]string{}
	for id, host := range r.Hosts {
		r.RegisterPages[id] = kb.SummaryPage(host, d.volumes)
	}
	for _, n := range g.Works {
		r.RegisterPages[n.ID] = kb.RegisterFile
	}
	r.HostAnchors = map[string]string{}
	for id, host := range r.Hosts {
		r.HostAnchors[id] = SectionSlug(d.leaves[host], x.markers[host][id])
	}
	x.anchors = x.pageAnchors()
	r.RegisterAnchors = x.anchors
	r.Edges = edgeRows(x.edges, kinds)
	r.Cites = graphCites(kinds, x.markers)
	var cuts []kb.ScorableEdge
	r.Resolved, cuts, _ = BreakCycles(g, x.hosted)
	for _, e := range cuts {
		x.cycles = append(x.cycles, result.Item{Check: kb.CheckMetadata, Path: kb.GraphEdgesFile, Remedy: kb.ConsistentRecordsRemedy,
			Detail: fmt.Sprintf("the edge %s closes a cycle among the scoring edges", EdgeName(e.Relation, e.Source, e.Target))})
	}
	x.strength = graphStrength(r.Resolved, graphEdges(r.Resolved, kinds))
	r.Nodes = x.nodes(x.strength)
	return x
}

// leafMarkers is each leaf's first marker line per node id, by leaf path.
func leafMarkers(leaves map[string]string) map[string]map[string]int {
	return perLeaf(leaves, kb.MarkerLines)
}

// leafMarkerOffsets is each leaf's first marker offset per node id, by leaf
// path.
func leafMarkerOffsets(leaves map[string]string) map[string]map[string]int {
	return perLeaf(leaves, kb.MarkerOffsets)
}

func perLeaf(leaves map[string]string, read func(string) map[string]int) map[string]map[string]int {
	out := map[string]map[string]int{}
	for p, text := range leaves {
		out[p] = read(text)
	}
	return out
}

// GraphKinds is every node of g's kind, by id.
func GraphKinds(g kb.Graph) map[string]string {
	out := map[string]string{}
	for _, n := range g.Claims {
		out[n.ID] = kb.NodeKindClaim
	}
	for _, n := range g.Analyses {
		out[n.ID] = kb.NodeKindAnalysis
	}
	for _, n := range g.Experiments {
		out[n.ID] = kb.NodeKindExperiment
	}
	for _, n := range g.Works {
		out[n.ID] = kb.NodeKindWork
	}
	for _, n := range g.Axioms {
		out[n.ID] = kb.NodeKindAxiom
	}
	return out
}

// graphEdges is every edge record of g as the walk and the rows read it: an
// empty context none, a target's kind read off kinds, a bearing on a founds,
// scorable or demoted record alone (a demoted record keeps the replaced
// edge's, which no walk reads), and the manually-scored flag where the
// record's type carries one.
func graphEdges(g kb.Graph, kinds map[string]string) []kb.Edge {
	var out []kb.Edge
	for _, r := range g.Edges() {
		e := kb.Edge{Source: r.Source, Target: r.Target, Relation: r.Relation, TargetKind: kinds[r.Target], ReplacedRelation: r.ReplacedRelation}
		if kb.Scoring(r.Relation) || r.Relation == kb.RelationDemoted {
			e.Bearing = *r.Bearing
		}
		if kb.BearingWalk(r.Relation) || r.Relation == kb.RelationDemoted {
			e.ManuallyScored = &r.ManuallyScored
		}
		if r.Context != "" {
			e.Context = &r.Context
		}
		out = append(out, e)
	}
	return out
}

// graphCites is one row per claim and leaf holding a statement marker naming
// it, by claim and leaf.
func graphCites(kinds map[string]string, markers map[string]map[string]int) []kb.CiteRow {
	var out []kb.CiteRow
	for leaf, lines := range markers {
		for id := range lines {
			if kinds[id] == kb.NodeKindClaim {
				out = append(out, kb.CiteRow{ClaimID: id, LeafPath: leaf})
			}
		}
	}
	slices.SortFunc(out, func(a, b kb.CiteRow) int {
		return cmp.Or(strings.Compare(a.ClaimID, b.ClaimID), strings.Compare(a.LeafPath, b.LeafPath))
	})
	return out
}

// SectionSlug is the anchor of the heading of the section holding the 0-based
// line of text: the nearest heading at or above it outside code, the heading
// kb.AnchorSection finds by that anchor; "" where no heading stands above it.
func SectionSlug(text string, line int) string {
	anchors := kb.HeadingAnchors(text)
	for i := line; i >= 0; i-- {
		if a, ok := anchors[i]; ok {
			return a
		}
	}
	return ""
}

// nodes is every node of x's records as a row, by kind and id: a claim's
// derived strength, band and counts — its supports and founds edges in, its
// cites — a work's and an axiom's label, and every node's canonical path and
// anchor. A claim's path is its register page and an analysis's or an experiment's its
// host, each "" with no host, as is its anchor.
func (x derivation) nodes(strength Strength) []kb.NodeRow {
	premises, citations := map[string]int{}, map[string]int{}
	for _, e := range x.edges {
		if e.Relation == kb.RelationSupports || e.Relation == kb.RelationFounds {
			premises[e.Target]++
		}
	}
	for _, c := range x.rows.Cites {
		citations[c.ClaimID]++
	}
	hosts := x.rows.Hosts
	// section is a hosted analysis's or experiment's host and the slug of its
	// marker's section.
	section := func(id string) (string, string) {
		host, ok := hosts[id]
		if !ok {
			return "", ""
		}
		return host, x.rows.HostAnchors[id]
	}
	g := x.g
	var out []kb.NodeRow
	for _, n := range g.Claims {
		r := strength.Results[n.ID]
		row := kb.NodeRow{NodeType: kb.NodeKindClaim, ID: n.ID, Title: n.Title,
			Rigor: n.Rigor, DerivationStrength: r.Derivation, Strength: r.Final,
			BuildStatus: BuildStatusPhrase(r.Final), StrengthBand: StrengthBand(r.Final),
			Rationale: n.Rationale, PremiseCount: premises[n.ID], CitationCount: citations[n.ID]}
		if _, hosted := hosts[n.ID]; hosted {
			row.CanonicalPath, row.CanonicalAnchor = x.rows.RegisterPages[n.ID], x.anchors[n.ID]
		}
		out = append(out, row)
	}
	for _, n := range g.Experiments {
		path, anchor := section(n.ID)
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindExperiment, ID: n.ID, Title: n.Title,
			CanonicalPath: path, CanonicalAnchor: anchor, Status: n.Status, Rigor: n.Rigor})
	}
	for _, n := range g.Analyses {
		path, anchor := section(n.ID)
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindAnalysis, ID: n.ID, Title: n.Title,
			CanonicalPath: path, CanonicalAnchor: anchor, Rigor: n.Rigor, Strength: strength.AnalysisResults[n.ID].Final})
	}
	for _, n := range g.Axioms {
		anchor, _ := kb.AxiomAnchor(x.d.axioms, n.ID)
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindAxiom, ID: n.ID, Title: n.Title, Label: n.Label, CanonicalPath: kb.AxiomsFile,
			CanonicalAnchor: anchor, Rigor: ptr(kb.AxiomRigor)})
	}
	for _, n := range g.Works {
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindWork, ID: n.ID, Title: n.Title, Label: n.Label, CanonicalPath: kb.RegisterFile,
			CanonicalAnchor: x.anchors[n.ID], Rigor: n.Rigor})
	}
	slices.SortStableFunc(out, func(a, b kb.NodeRow) int {
		return cmp.Or(strings.Compare(a.NodeType, b.NodeType), strings.Compare(a.ID, b.ID))
	})
	return out
}

// graphItems is every fault of a refused strict load as a result item.
func graphItems(err *kb.GraphLoadError) []result.Item {
	items := make([]result.Item, len(err.Faults))
	for i, f := range err.Faults {
		items[i] = result.Item{Check: f.Check, Key: f.Key, Path: f.Path, Line: f.Line, Allowed: f.Allowed, Remedy: f.Remedy, Detail: f.Detail}
	}
	return items
}
