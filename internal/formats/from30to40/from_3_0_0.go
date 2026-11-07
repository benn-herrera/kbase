// Package from30to40 converts a 3.0.0 KB's files to the 4.0.0 form. The node
// and edge records are read strictly by format30 and written by format40:
// an experiment's status pending becomes planned, every scored record's judged
// hash is the unjudged one, and a claim's or an analysis's no-edge field and a
// claim's strengthen-by field are dropped, each one a warning carrying the
// node's id and the field's text. Every statement marker in a kb-root
// document is rewritten in the kbase-id form, attached with no space, an
// analysis marker alone below a line ending in the marker of a claim it lifts
// moving onto that line (convertMarkers). The register pages, claim-rigor.md
// at kb-root and directly inside each of its directories, are obsolete: the
// save renders the summary pages in their place. Every link into a heading is
// re-anchored under 4.0.0's slugging, a link resolving to one node's statement
// takes the node's id as its title, and a bare id becomes its node's citation
// (convertCitations). Each build record standing at the repository root moves,
// bytes unchanged, into kb-root/.build/, its root path obsolete. Every other
// file is carried through as it stands, the stamp file at the version the tree
// is still at. A record file the 3.0.0 load refuses is refused with that
// load's faults.
package from30to40

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format20"
	"kbase/internal/formats/format30"
	"kbase/internal/formats/format40"
)

// The record files, repository-relative.
var (
	nodesPath = path.Join(format20.KBDir, format30.NodesFile)
	edgesPath = path.Join(format20.KBDir, format30.EdgesFile)
)

// buildRecords are the build records' names: 3.0.0 keeps each at the
// repository root, 4.0.0 in buildDir.
var buildRecords = []string{"kb-build-node-pass.yaml", "kb-build-classification.yaml", "kb-build-unmarked.yaml", "kb-build-cull.yaml"}

// buildDir is the repository-relative directory 4.0.0 keeps the build
// records in.
var buildDir = path.Join(format20.KBDir, ".build")

// Convert is files in the 4.0.0 form, the register pages and the
// moved build records' root paths as the obsolete paths, sorted, and the
// conversion's warnings: the records' in node
// id order, then the citations' in path and line order. A
// record file that already decodes as 4.0.0 — a save of this conversion
// stopped after writing it, which a read loads until the tree is restored — is taken as it stands; one that is absent stays
// absent, for the 4.0.0 load to refuse. The markers' one move reads the lifts
// of whichever form each record file stands in.
func Convert(files common.Files) (common.Files, []string, []common.Warning, error) {
	var old format30.Graph
	var current format40.Graph
	var faults []format30.GraphFault
	converted := map[string]bool{}
	for _, file := range []struct {
		at, rel string
		decode  func([]byte, string, *format30.Graph) []format30.GraphFault
		current func([]byte, string, *format40.Graph) []format40.GraphFault
	}{
		{nodesPath, format30.NodesFile, format30.DecodeNodes, format40.DecodeNodes},
		{edgesPath, format30.EdgesFile, format30.DecodeEdges, format40.DecodeEdges},
	} {
		data, ok := files[file.at]
		if !ok {
			converted[file.at] = true
			continue
		}
		var g format40.Graph
		if len(file.current(data, file.rel, &g)) == 0 {
			current = mergeGraphs(current, g)
			converted[file.at] = true
			continue
		}
		faults = append(faults, file.decode(data, file.rel, &old)...)
	}
	if len(faults) > 0 {
		return nil, nil, nil, &format30.GraphLoadError{Faults: faults}
	}
	minted := mintedIDs(old, current)
	g, warnings := convertGraph(old, minted)
	out := maps.Clone(files)
	if !converted[nodesPath] {
		out[nodesPath] = format40.EncodeNodes(g)
	}
	if !converted[edgesPath] {
		out[edgesPath] = format40.EncodeEdges(g)
	} else if edges, renamed := renameEdgeEnds(current, minted); renamed {
		// A 3.0.0 edge file whose work and axiom ids fit the minted grammar
		// decodes as 4.0.0, its ends still naming the 3.0.0 ids.
		out[edgesPath] = format40.EncodeEdges(edges)
	}
	lifts := map[lift]bool{}
	for _, e := range slices.Concat(g.Scorable, current.Scorable) {
		if e.Relation == format40.RelationLifts && strings.HasPrefix(e.Target, format40.ClaimPrefix+"-") {
			lifts[lift{e.Source, e.Target}] = true
		}
	}
	var obsolete []string
	for p, data := range files {
		rel, inKB := strings.CutPrefix(p, format20.KBDir+"/")
		switch {
		case !inKB || !strings.HasSuffix(rel, ".md"):
		case pageAt(rel, format20.RegisterFile):
			delete(out, p)
			obsolete = append(obsolete, p)
		default:
			out[p] = []byte(convertMarkers(string(data), lifts))
		}
	}
	for _, name := range buildRecords {
		if data, ok := files[name]; ok {
			delete(out, name)
			out[path.Join(buildDir, name)] = data
			obsolete = append(obsolete, name)
		}
	}
	slices.Sort(obsolete)
	return out, obsolete, append(warnings, convertCitations(files, out, minted)...), nil
}

// statuses is each 3.0.0 experiment status as 4.0.0 spells it.
var statuses = map[string]string{
	format30.ExperimentRun:     format40.ExperimentRun,
	format30.ExperimentPending: format40.ExperimentPlanned,
}

// convertGraph is old's records in the 4.0.0 form, each judged hash the
// unjudged one, each work's and axiom's id its minted one and its 3.0.0 key or
// label its label, every 3.0.0 work or axiom id in an edge's end or a prose
// field renamed to its minted id; and a warning per id minted and per no-edge
// and strengthen-by field it drops.
func convertGraph(old format30.Graph, minted map[string]string) (format40.Graph, []common.Warning) {
	var g format40.Graph
	warnings := mintedWarnings(old, minted)
	id := func(id string) string { return renamed(minted, id) }
	prose := func(s string) string { return renameIDs(minted, s) }
	dropped := func(list, id, text string) {
		text = prose(text)
		warnings = append(warnings, common.Warning{Path: nodesPath, List: list, ID: id, Text: text,
			Detail: fmt.Sprintf("%s's %s field is dropped, which 4.0.0 records do not carry: %q", id, list, text)})
	}
	for _, n := range old.Claims {
		g.Claims = append(g.Claims, format40.ClaimNode{ID: n.ID, Title: prose(n.Title), Rigor: n.Rigor,
			ManuallyScored: n.ManuallyScored, Rationale: prose(n.Rationale)})
		if n.NoEdge != "" {
			dropped(common.ListNoEdge, n.ID, n.NoEdge)
		}
		if len(n.StrengthenBy) > 0 {
			dropped(common.ListStrengthenBy, n.ID, strings.Join(n.StrengthenBy, "; "))
		}
	}
	for _, n := range old.Analyses {
		g.Analyses = append(g.Analyses, format40.AnalysisNode{ID: n.ID, Title: prose(n.Title), Rigor: n.Rigor,
			ManuallyScored: n.ManuallyScored, Rationale: prose(n.Rationale)})
		if n.NoEdge != "" {
			dropped(common.ListNoEdge, n.ID, n.NoEdge)
		}
	}
	for _, n := range old.Experiments {
		g.Experiments = append(g.Experiments, format40.ExperimentNode{ID: n.ID, Title: prose(n.Title), Status: statuses[n.Status],
			Rigor: n.Rigor, ManuallyScored: n.ManuallyScored})
	}
	for _, n := range old.Works {
		g.Works = append(g.Works, format40.WorkNode{ID: id(n.ID), Label: strings.TrimPrefix(n.ID, format30.WorkPrefix+"-"),
			Title: prose(n.Title), Rigor: n.Rigor, Rationale: prose(n.Rationale)})
	}
	for _, n := range old.Axioms {
		g.Axioms = append(g.Axioms, format40.AxiomNode{ID: id(n.ID), Label: strings.TrimPrefix(n.ID, format30.AxiomPrefix+"-"),
			Title: prose(n.Title)})
	}
	for _, e := range old.References {
		g.References = append(g.References, format40.UnscoredEdge{Source: id(e.Source), Target: id(e.Target), Context: prose(e.Context)})
	}
	for _, e := range old.Demoted {
		g.Demoted = append(g.Demoted, format40.DemotedEdge{Source: id(e.Source), Target: id(e.Target), Context: prose(e.Context),
			ReplacedRelation: e.ReplacedRelation, Bearing: bearing(e.Bearing), ManuallyScored: e.ManuallyScored})
	}
	for _, e := range old.Founds {
		g.Founds = append(g.Founds, format40.BearingEdge{Source: id(e.Source), Target: id(e.Target), Context: prose(e.Context),
			Bearing: bearing(e.Bearing)})
	}
	for _, e := range old.Scorable {
		g.Scorable = append(g.Scorable, format40.ScorableEdge{Relation: e.Relation, Source: id(e.Source), Target: id(e.Target),
			Context: prose(e.Context), Bearing: bearing(e.Bearing), ManuallyScored: e.ManuallyScored})
	}
	slices.SortStableFunc(warnings, func(a, b common.Warning) int { return strings.Compare(a.ID, b.ID) })
	return g, warnings
}

// mergeGraphs is a's records followed by b's.
func mergeGraphs(a, b format40.Graph) format40.Graph {
	return format40.Graph{Claims: slices.Concat(a.Claims, b.Claims), Analyses: slices.Concat(a.Analyses, b.Analyses),
		Experiments: slices.Concat(a.Experiments, b.Experiments), Works: slices.Concat(a.Works, b.Works),
		Axioms: slices.Concat(a.Axioms, b.Axioms), References: slices.Concat(a.References, b.References),
		Demoted: slices.Concat(a.Demoted, b.Demoted), Founds: slices.Concat(a.Founds, b.Founds),
		Scorable: slices.Concat(a.Scorable, b.Scorable)}
}

// renameEdgeEnds is g's edge records with every end naming a 3.0.0 work or
// axiom id renamed to its minted id, and whether any end was.
func renameEdgeEnds(g format40.Graph, minted map[string]string) (format40.Graph, bool) {
	changed := false
	id := func(end *string) {
		if to, ok := minted[*end]; ok {
			*end, changed = to, true
		}
	}
	out := format40.Graph{References: slices.Clone(g.References), Demoted: slices.Clone(g.Demoted),
		Founds: slices.Clone(g.Founds), Scorable: slices.Clone(g.Scorable)}
	for i := range out.References {
		id(&out.References[i].Source)
		id(&out.References[i].Target)
	}
	for i := range out.Demoted {
		id(&out.Demoted[i].Source)
		id(&out.Demoted[i].Target)
	}
	for i := range out.Founds {
		id(&out.Founds[i].Source)
		id(&out.Founds[i].Target)
	}
	for i := range out.Scorable {
		id(&out.Scorable[i].Source)
		id(&out.Scorable[i].Target)
	}
	return out, changed
}

func bearing(b format30.Bearing) format40.Bearing {
	return format40.Bearing{Value: b.Value, Pending: b.Pending}
}
