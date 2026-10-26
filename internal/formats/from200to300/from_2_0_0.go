// Package from200to300 converts a 2.0.x KB's files to the 3.0.0 form. The node
// and edge records are written from what the 2.0.x load reads; every
// document's frontmatter is removed, the entry point's kb-format stamp with
// it — the next save writes the stamp in the stamp file; each node
// is left one statement marker where the 2.0.x listings and markers place it
// (placeMarkers); the 2.0.x index files are obsolete, edges.yaml replaced by the record file and the
// stamp file kept, which a resumed save carries at the version the tree is still at; and each
// thing the conversion drops that the 3.0.0 form has no place for is a
// warning. A KB the 2.0.x load refuses is refused with that load's error.
package from200to300

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format200"
	"kbase/internal/formats/format300"
)

// The 3.0.0 record files, repository-relative.
var (
	nodesPath = path.Join(format200.KBDir, format300.NodesFile)
	edgesPath = path.Join(format200.KBDir, format300.EdgesFile)
)

// noEdgeKey is the register field a 2.0.x entry's no-edge reason is written
// under; the 2.0.x register reader does not read it.
const noEdgeKey = "no-edge"

// relations is each 2.0.x relation as the 3.0.0 form names it.
var relations = map[string]string{
	format200.RelationSupports:   format300.RelationSupports,
	format200.RelationFounds:     format300.RelationFounds,
	format200.RelationLifts:      format300.RelationLifts,
	format200.RelationRefutes:    format300.RelationRefutes,
	format200.RelationReferences: format300.RelationReferences,
	format200.RelationDemoted:    format300.RelationDemoted,
}

var markerRE = regexp.MustCompile(`^<!--\s*id:\s*(\S+?)\s*-->$`)

// site is where a 2.0.x value was authored: a repository-relative file and a
// 1-based line.
type site struct {
	path string
	line int
}

func (s site) String() string { return fmt.Sprintf("%s:%d", s.path, s.line) }

// conversion collects one conversion's warnings.
type conversion struct {
	warnings []common.Warning
}

func (c *conversion) warn(at site, format string, args ...any) {
	c.warnings = append(c.warnings, common.Warning{Path: at.path, Line: at.line, Detail: fmt.Sprintf(format, args...)})
}

// kbPath is a kb-root-relative path as a repository-relative one.
func kbPath(rel string) string { return path.Join(format200.KBDir, rel) }

// Convert200To300 is files in the 3.0.0 form, the 2.0.x index files that form
// no longer has, and the conversion's warnings in file and line order. Where
// the node records stand, a save of this conversion stopped after writing both
// record files: they are taken as they stand, every document's frontmatter is
// removed unread, nothing is derived from the 2.0.x registers or frontmatter,
// and there are no warnings.
func Convert200To300(files common.Files) (common.Files, []string, []common.Warning, error) {
	if _, ok := files[nodesPath]; ok {
		out, obsolete, err := resumed(files)
		return out, obsolete, nil, err
	}
	c := &conversion{}
	st, err := format200.Discover(files, func(rel string, line int, detail string) {
		c.warn(site{kbPath(rel), line}, "%s", detail)
	})
	if err != nil {
		return nil, nil, nil, err
	}
	regs, err := c.readRegisters(files)
	if err != nil {
		return nil, nil, nil, err
	}
	docs, err := c.readDocuments(files)
	if err != nil {
		return nil, nil, nil, err
	}
	g := c.graph(st, regs, docs.declared)

	out := maps.Clone(files)
	maps.Copy(out, docs.stripped)
	if err := c.placeMarkers(files, out, st, g, docs); err != nil {
		return nil, nil, nil, err
	}
	obsolete := dropIndexFiles(out)
	out[nodesPath] = format300.EncodeNodes(g)
	out[edgesPath] = format300.EncodeEdges(g)
	slices.SortStableFunc(c.warnings, func(a, b common.Warning) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	return out, obsolete, c.warnings, nil
}

// resumed is files whose 3.0.0 records stand, with every document's
// frontmatter removed, and the 2.0.x index files obsolete.
func resumed(files common.Files) (common.Files, []string, error) {
	out := maps.Clone(files)
	rels, err := format200.Documents(files)
	if err != nil {
		return nil, nil, err
	}
	for _, rel := range rels {
		p := kbPath(rel)
		text := string(files[p])
		if format200.FindFrontmatter(text) == nil {
			continue
		}
		out[p] = withoutFrontmatter(text)
	}
	return out, dropIndexFiles(out), nil
}

// dropIndexFiles removes from files every index file but the two record
// files and the stamp file, and is their paths, sorted. The stamp file stands
// on disk at the version the tree is still at while its save runs, so a
// resumed conversion carries it through and the save's last write replaces it.
func dropIndexFiles(files common.Files) []string {
	var obsolete []string
	indexDir := path.Join(format200.KBDir, format200.IndexDir)
	stamp := path.Join(format200.KBDir, format300.FormatFile)
	for p := range files {
		if path.Dir(p) == indexDir && p != nodesPath && p != edgesPath && p != stamp {
			obsolete = append(obsolete, p)
			delete(files, p)
		}
	}
	slices.Sort(obsolete)
	return obsolete
}

// withoutFrontmatter is text with its frontmatter block removed.
func withoutFrontmatter(text string) []byte {
	return []byte(text[format200.FrontmatterEnd(text):])
}

// registerFacts is what the converter reads from the 2.0.x registers beside
// the 2.0.x load: every bound entry's heading by id, in the inventory's order —
// registers in Registers order, entries in document order — each entry's
// no-edge reason, and each analysis entry's register half.
type registerFacts struct {
	keyed    map[string][]site
	noEdge   map[string]string
	analyses []analysisEntry
}

// nth is the nth of an id's sites, the last where fewer stand: the site of the
// nth record a reader returned for that id.
func nth(sites []site, n int) site {
	if len(sites) == 0 {
		return site{}
	}
	return sites[min(n, len(sites)-1)]
}

// warnRepeats warns of every site of an id past its first, which the 2.0.x
// inventory keeps; format names the id and the kept site's file.
func (c *conversion) warnRepeats(sites map[string][]site, format string) {
	for id, at := range sites {
		for _, again := range at[1:] {
			c.warn(again, format, id, at[0].path)
		}
	}
}

// analysisEntry is one register's half of an analysis and the heading it sits
// under.
type analysisEntry struct {
	id    string
	at    site
	entry format200.AnalysisEntry
}

func (c *conversion) readRegisters(files common.Files) (registerFacts, error) {
	facts := registerFacts{keyed: map[string][]site{}, noEdge: map[string]string{}}
	regs, err := format200.Registers(files)
	if err != nil {
		return facts, err
	}
	for _, rel := range regs {
		p := kbPath(rel)
		text := format200.TranslateNewlines(string(files[p]))
		headings := map[string]site{}
		for _, e := range format200.LocateRegisterEntries(text) {
			if !e.Bound() {
				continue
			}
			at := site{p, e.HeadingLine + 1}
			headings[e.NodeID] = at
			facts.keyed[e.NodeID] = append(facts.keyed[e.NodeID], at)
		}
		c.scanRegister(p, text, facts.noEdge)
		order, halves, err := format200.ParseAnalysisEntries(text, rel, nil, func(string, int, string) {})
		if err != nil {
			return facts, err
		}
		for _, id := range order {
			facts.analyses = append(facts.analyses, analysisEntry{id: id, at: headings[id], entry: halves[id]})
		}
	}
	c.warnRepeats(facts.keyed, "a second register entry for %s is dropped; the entry in %s is kept")
	return facts, nil
}

// registerFieldKeys is every "- <key>:" line a 2.0.x register entry carries.
var registerFieldKeys = append(slices.Clone(format200.RigorFieldKeys), format200.LiftsKey, noEdgeKey)

// fieldKey is the register field a stripped line opens, ok false where it
// opens none.
func fieldKey(s string) (string, bool) {
	for _, k := range registerFieldKeys {
		if strings.HasPrefix(s, "- "+k+":") {
			return k, true
		}
	}
	return "", false
}

// scanRegister warns of each line inside a register entry that is no field,
// footer, derived line or structure, recording each entry's no-edge reason in
// noEdge. An entry is a "## " section holding an id marker; a line outside
// every entry is dropped silently.
func (c *conversion) scanRegister(p, text string, noEdge map[string]string) {
	raw := format200.SplitLines(text)
	lines := format200.SplitLines(format200.StripCodeFences(text))
	marked := map[int]bool{}
	for _, e := range format200.LocateRegisterEntries(text) {
		if e.Bound() {
			marked[e.HeadingLine] = true
		}
	}
	inEntry, rationale, entry := false, false, ""
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			inEntry, rationale, entry = marked[i], false, ""
			continue
		}
		if !inEntry {
			continue
		}
		s := format200.Strip(line)
		if m := markerRE.FindStringSubmatch(s); m != nil {
			entry, rationale = m[1], false
			continue
		}
		if k, ok := fieldKey(s); ok {
			rationale = k == "rationale"
			if _, reason, _ := strings.Cut(s, ":"); k == noEdgeKey && entry != "" && noEdge[entry] == "" {
				noEdge[entry] = format200.Strip(reason)
			}
			continue
		}
		switch {
		case s == "":
			rationale = false
			if format200.Strip(raw[i]) == "" {
				continue
			}
		case s == "---" || s == "### Rigor" || s == "### Strength" || strings.HasPrefix(s, format200.LeafReferencesPrefix):
			rationale = false
			continue
		case strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || rationale:
			continue
		}
		c.warn(site{p, i + 1}, "register text dropped: %q", format200.Strip(raw[i]))
	}
}

// documents is the documents whose frontmatter the conversion removes, by
// repository-relative path; every site a leaf declares each node at, in the
// 2.0.x load's order — documents in Documents order, declarations in list
// order; and the site of each document's claims key, by path.
type documents struct {
	stripped common.Files
	declared map[string][]site
	listed   map[string]site
}

func (c *conversion) readDocuments(files common.Files) (documents, error) {
	docs := documents{stripped: common.Files{}, declared: map[string][]site{}, listed: map[string]site{}}
	rels, err := format200.Documents(files)
	if err != nil {
		return docs, err
	}
	for _, rel := range rels {
		p := kbPath(rel)
		text := string(files[p])
		m := format200.FindFrontmatter(text)
		if m == nil {
			continue
		}
		fm, err := format200.ParseFrontmatter(text)
		if err != nil {
			return docs, err
		}
		var root yaml.Node
		if err := yaml.Unmarshal([]byte(text[m[2]:m[3]]), &root); err != nil {
			return docs, err
		}
		for _, key := range topLevelKeys(&root) {
			at := site{p, key.Line + 1}
			v := fm[key.Value]
			switch {
			// The entry point's stamp moves to the stamp file at the next
			// save, so its dropping is not a loss worth a warning.
			case key.Value == format300.FormatKey:
			case key.Value == format200.ExperimentNodesKey || key.Value == format200.AnalysisNodesKey:
				idKey := format200.ExpIDKey
				if key.Value == format200.AnalysisNodesKey {
					idKey = format200.AnaIDKey
				}
				if fm.Kind() != format200.DocumentLeaf {
					c.warn(at, "frontmatter key %q on a document that is not kind: leaf declares no node to the 2.0.x load and is dropped", key.Value)
					continue
				}
				for _, d := range v.Entries {
					if id := d.Str(idKey); id != "" {
						docs.declared[id] = append(docs.declared[id], site{p, d.Line})
					}
				}
			case key.Value == "kind" || key.Value == "no-claim":
				if v.IsList || v.IsBool {
					c.warn(at, "frontmatter key %q holds a value of the wrong type and is dropped", key.Value)
				}
			case key.Value == "claims" || strings.HasPrefix(key.Value, "subtree-"):
				if key.Value == "claims" {
					docs.listed[p] = at
				}
				if v.IsBool || !v.IsList && v.Str != "" {
					c.warn(at, "frontmatter key %q holds a value of the wrong type and is dropped", key.Value)
				}
			default:
				c.warn(at, "frontmatter key %q dropped", key.Value)
			}
		}
		docs.stripped[p] = withoutFrontmatter(text)
	}
	c.warnRepeats(docs.declared, "a second declaration of %s is dropped; the declaration in %s is kept")
	return docs, nil
}

// topLevelKeys is a parsed frontmatter block's keys in document order.
func topLevelKeys(root *yaml.Node) []*yaml.Node {
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	m := root.Content[0]
	var keys []*yaml.Node
	for i := 0; i+1 < len(m.Content); i += 2 {
		keys = append(keys, m.Content[i])
	}
	return keys
}

// graph is the 3.0.0 records of what the 2.0.x load read, every
// manually-scored false.
//
// An id keyed by more than one register entry, or declared more than once by
// host leaves, is one record, from the first: the entry the 2.0.x inventory
// keeps. Every entry's edges stand, each at its own site.
func (c *conversion) graph(st format200.State, regs registerFacts, declared map[string][]site) format300.Graph {
	var g format300.Graph
	edges := edgeSet{byKey: map[string]int{}, c: c}
	seen := map[string]int{}
	// occurrence is how many records of id came before this one.
	occurrence := func(id string) int {
		n := seen[id]
		seen[id]++
		return n
	}
	for _, e := range st.ClaimEntries {
		n := occurrence(e.ID)
		at := nth(regs.keyed[e.ID], n)
		if n == 0 {
			var strengthenBy []string
			for _, item := range e.StrengthenBy {
				strengthenBy = append(strengthenBy, item.Text)
			}
			g.Claims = append(g.Claims, format300.ClaimNode{ID: e.ID, Title: e.Title, Rigor: e.Rigor,
				Rationale: e.Rationale, NoEdge: regs.noEdge[e.ID], StrengthenBy: strengthenBy})
		}
		edges.addAll(at, e.Premises, e.Refutes, e.References)
		for _, d := range e.Demoted {
			// A demoted bullet on X naming Y records the supports edge Y → X it replaced.
			edges.add(authoredEdge{relation: format300.RelationDemoted, replaced: format300.RelationSupports,
				source: d.Target, target: d.Source, context: contextOf(d), bearing: format300.Bearing{Pending: true}, at: at})
		}
	}
	for _, w := range st.Works {
		if occurrence(w.ID) == 0 {
			g.Works = append(g.Works, format300.WorkNode{ID: w.ID, Title: w.Title, Rigor: w.Rigor, Rationale: w.Rationale})
		}
	}
	for _, a := range st.Axioms {
		g.Axioms = append(g.Axioms, format300.AxiomNode{ID: a.ID, Title: a.Title})
	}
	// The 2.0.x load joined each hosted analysis with the last register half
	// of its id; the record takes the first, the entry the inventory keeps.
	kept := map[string]analysisEntry{}
	for _, r := range regs.analyses {
		if _, ok := kept[r.id]; !ok {
			kept[r.id] = r
		}
	}
	titles := map[string]string{}
	for _, a := range st.Analyses {
		n := occurrence(a.ID)
		if n == 0 {
			half := kept[a.ID].entry
			titles[a.ID] = c.analysisTitle(a.ID, half.Title, a.Title, nth(declared[a.ID], 0))
			g.Analyses = append(g.Analyses, format300.AnalysisNode{ID: a.ID, Title: titles[a.ID], Rigor: format200.AuthoredRigor(half.Rigor),
				Rationale: half.Rationale, NoEdge: regs.noEdge[a.ID]})
		}
		edges.addAll(nth(regs.keyed[a.ID], 0), a.Premises)
		// The 2.0.x load joined the leaf's pairs with the register's, the leaf's
		// winning a shared target; both sites enter here so a differing bearing
		// is kept by the identity rule and warned of.
		edges.addAll(nth(declared[a.ID], n), a.Lifts, a.Refutes)
	}
	for _, r := range regs.analyses {
		if _, hosted := titles[r.id]; !hosted {
			c.warn(r.at, "analysis %s is hosted by no leaf's %s and is dropped", r.id, format200.AnalysisNodesKey)
			continue
		}
		edges.addAll(r.at, r.entry.Lifts, r.entry.Refutes)
	}
	for _, x := range st.Experiments {
		n := occurrence(x.ID)
		if n == 0 {
			g.Experiments = append(g.Experiments, format300.ExperimentNode{ID: x.ID, Title: c.hostTitle(x.ID, x.Title, nth(declared[x.ID], 0)),
				Status: x.Status, Rigor: x.Rigor})
		}
		edges.addAll(nth(declared[x.ID], n), x.Supports, x.Lifts, x.Refutes)
	}
	for _, e := range edges.edges {
		switch e.relation {
		case format300.RelationReferences:
			g.References = append(g.References, format300.UnscoredEdge{Source: e.source, Target: e.target, Context: e.context})
		case format300.RelationDemoted:
			g.Demoted = append(g.Demoted, format300.DemotedEdge{Source: e.source, Target: e.target, Context: e.context,
				ReplacedRelation: e.replaced, Bearing: e.bearing})
		case format300.RelationFounds:
			g.Founds = append(g.Founds, format300.BearingEdge{Source: e.source, Target: e.target, Context: e.context, Bearing: e.bearing})
		default:
			g.Scorable = append(g.Scorable, format300.ScorableEdge{Relation: e.relation, Source: e.source, Target: e.target,
				Context: e.context, Bearing: e.bearing})
		}
	}
	return g
}

// analysisTitle is a hosted analysis's title: the heading its 2.0.x register
// entry is written under, which is the author's title for it, and where the
// register writes none, its host's first heading — else the host's file name,
// warned of at the declaration at.
func (c *conversion) analysisTitle(id, registerHeading, hostHeading string, at site) string {
	if registerHeading != "" {
		return registerHeading
	}
	return c.hostTitle(id, hostHeading, at)
}

// hostTitle is a hosted node's title: heading, its host's first heading as the
// 2.0.x load read it, or where the host has none the host's file name less its
// extension, warned of at the declaration at.
func (c *conversion) hostTitle(id, heading string, at site) string {
	if heading != "" {
		return heading
	}
	title := strings.TrimSuffix(path.Base(at.path), path.Ext(at.path))
	c.warn(at, "%s's host has no heading, so it is titled %q, its host's file name", id, title)
	return title
}

func contextOf(e format200.Edge) string {
	if e.Context == nil {
		return ""
	}
	return *e.Context
}

// authoredEdge is one 2.0.x authoring of an edge and the site it was authored
// at.
type authoredEdge struct {
	relation, replaced, source, target, context string
	bearing                                     format300.Bearing
	at                                          site
}

// edgeSet is the edges authored so far, one per identity, in the order each
// identity was first authored.
type edgeSet struct {
	edges []authoredEdge
	byKey map[string]int
	c     *conversion
}

func (s *edgeSet) addAll(at site, lists ...[]format200.Edge) {
	for _, list := range lists {
		for _, e := range list {
			s.add(authoredEdge{relation: relations[e.Relation], source: e.Source, target: e.Target,
				context: contextOf(e), bearing: format300.Bearing(e.Bearing), at: at})
		}
	}
}

// add records one authoring. A second authoring of an identity with an equal
// bearing joins its context to the first's; with a different bearing the
// higher is kept, a number above pending, and the two sites are warned of.
func (s *edgeSet) add(e authoredEdge) {
	key := strings.Join(format300.EdgeKey(e.relation, e.replaced, e.source, e.target), "\x00")
	i, seen := s.byKey[key]
	if !seen {
		s.byKey[key] = len(s.edges)
		s.edges = append(s.edges, e)
		return
	}
	held := &s.edges[i]
	if held.bearing == e.bearing {
		held.context = joinContexts(held.context, e.context)
		return
	}
	kept := *held
	if above(e.bearing, held.bearing) {
		kept = e
	}
	s.c.warn(held.at, "%s %s → %s is authored with bearing %s at %s and %s at %s; %s is kept",
		e.relation, e.source, e.target, bearingText(held.bearing), held.at, bearingText(e.bearing), e.at, bearingText(kept.bearing))
	*held = kept
}

func joinContexts(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "; " + b
}

// above is whether bearing a is higher than b: a number is above pending.
func above(a, b format300.Bearing) bool {
	if a.Pending || b.Pending {
		return !a.Pending
	}
	return a.Value > b.Value
}

func bearingText(b format300.Bearing) string {
	if b.Pending {
		return format300.PendingLiteral
	}
	return format300.PyFloatRepr(b.Value)
}
