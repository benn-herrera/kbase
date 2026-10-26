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
	Nodes        []kb.NodeRow
	Edges        []kb.EdgeRow
	StrengthenBy []kb.StrengthenByRow
	Cites        []kb.CiteRow
	Aggregates   []kb.AggregateRow
	// DocumentKinds is every document's kind, by kb-root-relative path.
	DocumentKinds map[string]string
	// Hosts is the leaf hosting each claim, analysis and experiment that
	// carries a statement marker; HostFaults is each one carrying none or
	// several.
	Hosts      map[string]string
	HostFaults []kb.HostFault
	// RegisterPages is the kb-root-relative register page each node is
	// rendered on: a hosted claim's, analysis's or experiment's volume page, or
	// the root page where its host lies in no volume, and the root page for
	// every work. An axiom and a node with no host are on none.
	RegisterPages map[string]string
	// Resolved is the records with every fault a degraded load meets resolved
	// for the walk, which reads it: an edge with an end no record carries
	// dropped, one record kept of each edge identity, and every cycle among the
	// scoring edges cut. Degraded is a warning per record dropped or edge cut;
	// the load is degraded where it holds any.
	Resolved kb.Graph
	Degraded []result.Item
}

// DegradedItems is a refusal item per fault rows' degraded load resolved:
// check: degraded, the fault's path and detail, remedy RefreshRemedy; nil
// where the load is not degraded.
func (rows Rows) DegradedItems() []result.Item {
	var items []result.Item
	for _, w := range rows.Degraded {
		items = append(items, result.Item{Check: kb.CheckDegraded, Path: w.Path, Remedy: RefreshRemedy, Detail: w.Detail})
	}
	return items
}

// RefuseDegraded is the refusal of a caller that does not run over a degraded
// load of the KB src holds: Derive's refusal or failure, else a result.Refusal
// of DegradedItems where the load is degraded, else nil.
func RefuseDegraded(src *kb.Source) error {
	rows, err := Derive(src)
	if err != nil {
		return err
	}
	if items := rows.DegradedItems(); items != nil {
		return result.Refusal(items)
	}
	return nil
}

// Derive is the rows of the KB src holds, derived from its records and the
// statement markers in its leaves. Record files the strict load refuses are
// its result.Refusal; a premise cycle, an edge naming no node and an edge
// identity authored twice are not refused but resolved for the walk.
func Derive(src *kb.Source) (Rows, error) {
	x, err := loadDerivation(src)
	return x.rows, err
}

// loadDerivation is Derive's reading of the KB src holds.
func loadDerivation(src *kb.Source) (derivation, error) {
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
// kb-root-relative path, each leaf's text, and the volumes.
type documents struct {
	kinds    map[string]string
	children map[string][]string
	leaves   map[string]string
	volumes  map[string]bool
}

func readDocuments(src *kb.Source) (documents, error) {
	paths, err := kb.Documents(src)
	if err != nil {
		return documents{}, err
	}
	d := documents{kinds: map[string]string{}, children: map[string][]string{}, leaves: map[string]string{}, volumes: Volumes(paths)}
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
	return d, nil
}

// derivation is one Derive's reading of a KB: its records and documents, every
// edge as the walk reads it, each leaf's first marker line per node id, the
// hosted nodes in their hosted order, the strength the walk derived over the
// resolved records, and the rows and placements derived so far.
type derivation struct {
	g        kb.Graph
	d        documents
	edges    []kb.Edge
	markers  map[string]map[string]int
	hosted   []string
	strength Strength
	rows     Rows
}

func deriveGraph(g kb.Graph, d documents) derivation {
	kinds := GraphKinds(g)
	x := derivation{g: g, d: d, edges: graphEdges(g, kinds), markers: leafMarkers(d.leaves)}
	r := &x.rows
	r.DocumentKinds = maps.Clone(d.kinds)
	r.Hosts, r.HostFaults = kb.Hosts(g, d.leaves)
	x.hosted = hostedOrder(d.children, r.Hosts, x.markers)
	r.RegisterPages = map[string]string{}
	for id, host := range r.Hosts {
		r.RegisterPages[id] = RegisterPage(host, d.volumes)
	}
	for _, n := range g.Works {
		r.RegisterPages[n.ID] = kb.RegisterFile
	}
	r.Edges = edgeRows(x.edges, kinds)
	r.Cites = graphCites(kinds, x.markers)
	r.StrengthenBy = graphStrengthenBy(g)
	r.Resolved, r.Degraded = resolveGraph(g, kinds, x.hosted)
	x.strength = graphStrength(r.Resolved, graphEdges(r.Resolved, kinds))
	r.Nodes = x.nodes(x.strength)
	r.Aggregates = x.aggregates()
	return x
}

// leafMarkers is each leaf's first marker line per node id, by leaf path.
func leafMarkers(leaves map[string]string) map[string]map[string]int {
	out := map[string]map[string]int{}
	for p, text := range leaves {
		out[p] = kb.MarkerLines(text)
	}
	return out
}

// resolveGraph is g with each edge record an end of which no record carries
// dropped, then of each edge identity's records the one with the highest
// bearing kept — a number above pending, the first on a tie; for references
// the first — then every cycle among the scoring edges broken by BreakCycles
// walking the hosted nodes in hosted's order; and a warning per record dropped
// or edge cut. kinds is every node of g's kind, by id.
func resolveGraph(g kb.Graph, kinds map[string]string, hosted []string) (kb.Graph, []result.Item) {
	var warnings []result.Item
	warn := func(format string, args ...any) {
		warnings = append(warnings, result.Item{Path: kb.GraphEdgesFile, Detail: fmt.Sprintf(format, args...)})
	}
	out := g
	out.References = resolveRecords(g.References, func(e kb.UnscoredEdge) edgeFacts {
		return edgeFacts{EdgeName(kb.RelationReferences, e.Source, e.Target), e.Source, e.Target, nil}
	}, kinds, warn)
	out.Demoted = resolveRecords(g.Demoted, func(e kb.DemotedEdge) edgeFacts {
		return edgeFacts{EdgeName(kb.RelationDemoted+" "+e.ReplacedRelation, e.Source, e.Target), e.Source, e.Target, &e.Bearing}
	}, kinds, warn)
	out.Founds = resolveRecords(g.Founds, func(e kb.BearingEdge) edgeFacts {
		return edgeFacts{EdgeName(kb.RelationFounds, e.Source, e.Target), e.Source, e.Target, &e.Bearing}
	}, kinds, warn)
	out.Scorable = resolveRecords(g.Scorable, func(e kb.ScorableEdge) edgeFacts {
		return edgeFacts{EdgeName(e.Relation, e.Source, e.Target), e.Source, e.Target, &e.Bearing}
	}, kinds, warn)
	out, _, cuts := BreakCycles(out, hosted)
	return out, append(warnings, cuts...)
}

// edgeFacts is what the resolution reads off one edge record: the name that is
// its identity, its ends, and its bearing, nil on a references record.
type edgeFacts struct {
	name, source, target string
	bearing              *kb.Bearing
}

// resolveRecords is records less each one naming a node kinds lacks, and less
// each one whose identity a kept record already holds at a bearing no lower,
// in file order; a later record of a higher bearing takes the kept one's place.
// warn is called once per record dropped.
func resolveRecords[E any](records []E, facts func(E) edgeFacts, kinds map[string]string, warn func(string, ...any)) []E {
	var kept []E
	at := map[string]int{}
	for _, e := range records {
		f := facts(e)
		var missing []string
		for _, id := range []string{f.source, f.target} {
			if _, ok := kinds[id]; !ok && !slices.Contains(missing, id) {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			warn("%s is dropped: no record carries %s", f.name, strings.Join(missing, " or "))
			continue
		}
		i, twice := at[f.name]
		if !twice {
			at[f.name] = len(kept)
			kept = append(kept, e)
			continue
		}
		standing := facts(kept[i])
		switch {
		case f.bearing == nil:
			warn("%s is authored more than once: a later record is dropped and the first stands", f.name)
		case bearingAbove(*f.bearing, *standing.bearing):
			warn("%s is authored more than once: the record of bearing %s is dropped and the one of bearing %s stands",
				f.name, kb.BearingText(*standing.bearing), kb.BearingText(*f.bearing))
			kept[i] = e
		default:
			warn("%s is authored more than once: the record of bearing %s is dropped and the one of bearing %s stands",
				f.name, kb.BearingText(*f.bearing), kb.BearingText(*standing.bearing))
		}
	}
	return kept
}

// bearingAbove is whether a is higher than b: a number above pending, and the
// larger of two numbers.
func bearingAbove(a, b kb.Bearing) bool {
	if a.Pending {
		return false
	}
	return b.Pending || a.Value > b.Value
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
// empty context none, a target's kind read off kinds, a bearing on a founds or
// scorable record alone, and the manually-scored flag where the record's type
// carries one.
func graphEdges(g kb.Graph, kinds map[string]string) []kb.Edge {
	var out []kb.Edge
	for _, r := range g.Edges() {
		e := kb.Edge{Source: r.Source, Target: r.Target, Relation: r.Relation, TargetKind: kinds[r.Target]}
		if kb.Scoring(r.Relation) {
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

// graphStrengthenBy is every claim's strengthen-by items, by claim and item
// index: an item's text with its whitespace normalised, and every claim id it
// names that g holds, sorted. An empty item yields no row.
func graphStrengthenBy(g kb.Graph) []kb.StrengthenByRow {
	claims := map[string]bool{}
	for _, n := range g.Claims {
		claims[n.ID] = true
	}
	var out []kb.StrengthenByRow
	for _, n := range g.Claims {
		for idx, item := range n.StrengthenBy {
			text := kb.NormalizeSpace(item)
			if text == "" {
				continue
			}
			ids := kb.ClaimIDs(text)
			slices.Sort(ids)
			mentioned := []string{}
			for _, id := range slices.Compact(ids) {
				if claims[id] {
					mentioned = append(mentioned, id)
				}
			}
			out = append(out, kb.StrengthenByRow{ClaimID: n.ID, ItemIdx: idx, Text: text, MentionedIDs: mentioned})
		}
	}
	slices.SortStableFunc(out, func(a, b kb.StrengthenByRow) int {
		return cmp.Or(strings.Compare(a.ClaimID, b.ClaimID), cmp.Compare(a.ItemIdx, b.ItemIdx))
	})
	return out
}

// Volumes is the volumes among paths, kb-root-relative documents: the
// top-level directories holding an index.md.
func Volumes(paths []string) map[string]bool {
	volumes := map[string]bool{}
	for _, rel := range paths {
		if dir, rest, nested := strings.Cut(rel, "/"); nested && rest == kb.IndexFile {
			volumes[dir] = true
		}
	}
	return volumes
}

// RegisterPage is the register page a node hosted at host is rendered on: the
// page of the volume host lies in, or the root page where it lies in none.
func RegisterPage(host string, volumes map[string]bool) string {
	if dir, _, nested := strings.Cut(host, "/"); nested && volumes[dir] {
		return volumePage(dir)
	}
	return kb.RegisterFile
}

// volumePage is the register page of the volume at dir.
func volumePage(dir string) string { return dir + "/" + kb.RegisterFile }

// sectionSlug is the slug of the heading of the section holding line of text:
// the nearest heading at or above it outside code, the heading kb.AnchorSection
// finds by that slug; "" where no heading stands above it.
func sectionSlug(text string, line int) string {
	lines := strings.Split(kb.StripCodeSplitLines(text), "\n")
	for i := min(line, len(lines)-1); i >= 0; i-- {
		if h := kb.FirstHeading(lines[i]); h != "" {
			return kb.Slugify(h)
		}
	}
	return ""
}

// nodes is every node of x's records as a row, by kind and id: a claim's
// derived strength, band and counts — its supports and founds edges in, its
// strengthen-by rows, its cites — and every node's canonical path and anchor.
// A claim's path is its register page and an analysis's or an experiment's its
// host, each "" with no host, as is its anchor.
func (x derivation) nodes(strength Strength) []kb.NodeRow {
	premises, citations, items := map[string]int{}, map[string]int{}, map[string]int{}
	for _, e := range x.edges {
		if e.Relation == kb.RelationSupports || e.Relation == kb.RelationFounds {
			premises[e.Target]++
		}
	}
	for _, c := range x.rows.Cites {
		citations[c.ClaimID]++
	}
	for _, r := range x.rows.StrengthenBy {
		items[r.ClaimID]++
	}
	hosts := x.rows.Hosts
	// section is a hosted analysis's or experiment's host and the slug of its
	// marker's section.
	section := func(id string) (string, string) {
		host, ok := hosts[id]
		if !ok {
			return "", ""
		}
		return host, sectionSlug(x.d.leaves[host], x.markers[host][id])
	}
	g := x.g
	var out []kb.NodeRow
	for _, n := range g.Claims {
		r := strength.Results[n.ID]
		row := kb.NodeRow{NodeType: kb.NodeKindClaim, ID: n.ID, Title: n.Title,
			Rigor: n.Rigor, DerivationStrength: r.Derivation, Strength: r.Final,
			BuildStatus: BuildStatusPhrase(r.Final), BuildBand: BuildBand(r.Final),
			Rationale: n.Rationale, PremiseCount: premises[n.ID],
			StrengthenByCount: items[n.ID], CitationCount: citations[n.ID]}
		if _, hosted := hosts[n.ID]; hosted {
			row.CanonicalPath, row.CanonicalAnchor = x.rows.RegisterPages[n.ID], kb.Slugify(n.Title)
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
		label := strings.TrimPrefix(n.ID, kb.AxiomPrefix+"-")
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindAxiom, ID: n.ID, Title: n.Title, CanonicalPath: kb.AxiomsFile,
			CanonicalAnchor: kb.Slugify("Axiom " + label + ": " + n.Title), Rigor: ptr(kb.AxiomRigor)})
	}
	for _, n := range g.Works {
		out = append(out, kb.NodeRow{NodeType: kb.NodeKindWork, ID: n.ID, Title: n.Title, CanonicalPath: kb.RegisterFile,
			CanonicalAnchor: kb.Slugify(n.Title), Rigor: n.Rigor})
	}
	slices.SortStableFunc(out, func(a, b kb.NodeRow) int {
		return cmp.Or(strings.Compare(a.NodeType, b.NodeType), strings.Compare(a.ID, b.ID))
	})
	return out
}

// aggregates is every index and entry-point document's subtree, by path: the
// claims a leaf beneath its directory cites and the experiments a leaf there
// hosts, the entry point's spanning the KB.
func (x derivation) aggregates() []kb.AggregateRow {
	cites, hosts := x.rows.Cites, x.rows.Hosts
	var out []kb.AggregateRow
	for _, p := range slices.Sorted(maps.Keys(x.d.kinds)) {
		kind := x.d.kinds[p]
		if kind != kb.DocumentIndex && kind != kb.DocumentEntryPoint {
			continue
		}
		ep := kind == kb.DocumentEntryPoint
		claims, exps := map[string]bool{}, map[string]bool{}
		for _, c := range cites {
			if ep || isUnder(c.LeafPath, p) {
				claims[c.ClaimID] = true
			}
		}
		for _, n := range x.g.Experiments {
			if host, ok := hosts[n.ID]; ok && (ep || isUnder(host, p)) {
				exps[n.ID] = true
			}
		}
		out = append(out, kb.AggregateRow{NodePath: p, NodeKind: kind, SubtreeClaims: sortedSet(claims), SubtreeExperiments: sortedSet(exps)})
	}
	return out
}

// graphItems is every fault of a refused strict load as a result item.
func graphItems(err *kb.GraphLoadError) []result.Item {
	items := make([]result.Item, len(err.Faults))
	for i, f := range err.Faults {
		item := result.Item{Check: f.Check, Key: f.Key, Path: f.Path, Line: f.Line, Remedy: f.Remedy, Detail: f.Detail}
		if f.Allowed != "" {
			item.Allowed = []string{f.Allowed}
		}
		items[i] = item
	}
	return items
}
