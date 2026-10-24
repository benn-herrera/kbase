package format_2_0_0

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"kbase/internal/migrate/common"
)

// MalformedError is authored metadata no reader can take: refresh and verify
// stop on it.
type MalformedError struct{ Msg string }

func (e MalformedError) Error() string { return e.Msg }

func malformed(format string, args ...any) error {
	return MalformedError{Msg: fmt.Sprintf(format, args...)}
}

var (
	tier2InlineRE    = PyRE(`(?s)<!--\s*claim-rigor:\s*(.*?)\s*-->`)
	pairTargetIDRE   = PyRE(`^` + IDBody("clm", "ana") + `$`)
	headingLineRE    = PyRE(`^(#{1,6})\s+(.*)$`)
	expIDFullRE      = PyRE(`^` + IDBody("exp") + `$`)
	anaIDFullRE      = PyRE(`^` + IDBody("ana") + `$`)
	axiomHeading     = PyRE(`^### Axiom (` + AxiomLabel + `):\s*(.+)$`)
	invariantHeading = PyRE(`^### (INVARIANT-[A-Z]+[0-9]+):\s*(.+)$`)
	axiomBulletRE    = PyRE(`^- Axiom (\d+): \*\*(.+?)\*\*`)
)

// LeafRecord is a kind: leaf document's claim-graph metadata.
type LeafRecord struct {
	Path, Kind     string
	Claims         []string
	Tier2Marked    map[string]bool
	NoClaimReason  string
	ExperimentRefs []string
}

// IndexRecord is a kind: index or entry-point document's declared aggregates.
type IndexRecord struct {
	Path, Kind                 string
	DeclaredSubtreeClaims      []string
	DeclaredSubtreeExperiments []string
}

// The frontmatter keys a leaf's node declaration carries besides its id and
// status: the node's local value, and the pair lists an analysis or experiment
// authors on the source.
const (
	RigorKey   = "rigor"
	LiftsKey   = "lifts"
	RefutesKey = "refutes"
)

// ExperimentNode is a physical experiment: terminal, contributing to claims.
type ExperimentNode struct {
	ID, Title, CanonicalPath, CanonicalAnchor, Status string
	Rigor                                             *float64
	Supports, Lifts, Refutes                          []Edge
}

// AnalysisNode is one analysis: its hosting leaf's fan-out joined
// with its register entry's local value and premises.
type AnalysisNode struct {
	ID, Title, CanonicalPath, CanonicalAnchor string
	Rigor                                     *float64
	Premises                                  []Edge
	Lifts, Refutes                            []Edge
	Rationale                                 string
	Strength                                  *float64
	StrengthTrace                             string
}

// AxiomNode is an axiom a declaration document states.
type AxiomNode struct {
	NodeType, ID, Title, CanonicalPath, CanonicalAnchor string
}

// MarkerLines is, for every claim id a tier-2 marker in text names, the
// 0-based line the first such marker opens on.
func MarkerLines(text string) map[string]int {
	out := map[string]int{}
	for _, m := range tier2InlineRE.FindAllStringSubmatchIndex(text, -1) {
		line := strings.Count(text[:m[0]], "\n")
		for _, cid := range ClaimIDs(text[m[2]:m[3]]) {
			if _, seen := out[cid]; !seen {
				out[cid] = line
			}
		}
	}
	return out
}

// documentFrontmatter is the document's frontmatter, its malformation named
// with the document.
func documentFrontmatter(text, rel string) (Frontmatter, error) {
	fm, err := ParseFrontmatter(text)
	if err != nil {
		return nil, malformed("%s: %v", rel, err)
	}
	return fm, nil
}

// ParseLeaf is a leaf's record, or nil where the document has no frontmatter
// or is not kind: leaf.
func ParseLeaf(text, rel string) (*LeafRecord, error) {
	fm, err := documentFrontmatter(text, rel)
	if err != nil {
		return nil, err
	}
	if len(fm) == 0 || fm.Kind() != DocumentLeaf {
		return nil, nil
	}
	claims, err := fm.ListOrEmpty("claims")
	if err != nil {
		return nil, malformed("%s: claims: %v", rel, err)
	}
	leaf := &LeafRecord{Path: rel, Kind: DocumentLeaf, Claims: claims, Tier2Marked: map[string]bool{}}
	if v, ok := fm.Get("no-claim"); ok && !v.IsList && !v.IsBool {
		leaf.NoClaimReason = v.Str
	}
	refs, err := fm.ListOrEmpty("experiments")
	if err != nil {
		return nil, malformed("%s: experiments: %v", rel, err)
	}
	for _, r := range refs {
		if strings.HasPrefix(r, "exp-") {
			leaf.ExperimentRefs = append(leaf.ExperimentRefs, r)
		}
	}
	for _, m := range tier2InlineRE.FindAllStringSubmatch(StripFrontmatter(text), -1) {
		for _, cid := range ClaimIDs(m[1]) {
			if containsString(claims, cid) {
				leaf.Tier2Marked[cid] = true
			}
		}
	}
	return leaf, nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// FirstHeading is the text of a document's first heading at any level.
func FirstHeading(text string) string {
	for _, line := range SplitLines(text) {
		if m := headingLineRE.FindStringSubmatch(line); m != nil {
			return Strip(m[2])
		}
	}
	return ""
}

// declarations is a leaf's node declaration list under key — nil where the
// document is no kind: leaf or declares none — refusing an entry that lacks
// its id key.
func declarations(fm Frontmatter, key, idKey, rel string) ([]FrontmatterEntry, error) {
	if fm.Kind() != DocumentLeaf {
		return nil, nil
	}
	v := fm[key]
	if !v.IsList && v.Truthy() || len(v.List) > 0 {
		return nil, malformed("%s: %s is not a list of mappings", rel, key)
	}
	for _, node := range v.Entries {
		if _, ok := node.Frontmatter[idKey]; !ok {
			return nil, malformed("%s: an entry of %s carries no %s", rel, key, idKey)
		}
	}
	return v.Entries, nil
}

// pairTargetKind is a scoring pair target's node kind, read off an id
// pairTargetIDRE matched.
func pairTargetKind(id string) string {
	if strings.HasPrefix(id, "ana-") {
		return NodeKindAnalysis
	}
	return NodeKindClaim
}

// pairValueText is a pair entry's value as a fault names it: the token as
// written, or the shape a value no bearing could be read from has.
func pairValueText(v Value) string {
	switch {
	case v.IsList:
		return "a list"
	case v.IsBool:
		return "a boolean"
	}
	return strconv.Quote(v.Str)
}

// scorePairs is a node's score list under key: each entry one claim or analysis
// id and its bearing, in list order. An entry that names no one node id, whose
// value is no bearing, or whose pairing BearingLegal rejects is a fault at its
// own line and yields no edge.
func scorePairs(node Frontmatter, key, relation, sourceKind, rel, id string, faults *[]AnnotationFault) []Edge {
	var out []Edge
	allowed := "[0, 1] or " + PendingLiteral
	for _, entry := range node[key].Entries {
		target, score, single := scorePair(entry)
		switch {
		case !single:
			*faults = append(*faults, AnnotationFault{Field: key, Path: rel, Line: entry.Line,
				Remedy: "write one - <clm- or ana-id>: <bearing> line per pair",
				Detail: "an entry of " + key + " on " + id + " is not one node id and its bearing"})
			continue
		case !pairTargetIDRE.MatchString(target):
			*faults = append(*faults, AnnotationFault{Field: key, Path: rel, Line: entry.Line,
				Remedy: "name the claim or analysis the pair bears on",
				Detail: key + " on " + id + " names " + strconv.Quote(target) + ", which is no claim or analysis id"})
			continue
		}
		targetKind := pairTargetKind(target)
		if !BearingLegal(relation, sourceKind, targetKind) {
			*faults = append(*faults, AnnotationFault{Field: key, Path: rel, Line: entry.Line,
				Remedy: "lift or refute from an analysis, supports from an experiment",
				Detail: relation + " from " + sourceKind + " to " + targetKind + " is no legal pairing, so " + id + " bears on no " + target})
			continue
		}
		bearing, ok := BearingFromValue(score.Str)
		if score.IsList || score.IsBool || !ok {
			*faults = append(*faults, AnnotationFault{Field: key, Path: rel, Line: entry.Line, Allowed: allowed,
				Remedy: "write a number in [0, 1] or " + PendingLiteral,
				Detail: key + " for " + target + " is " + pairValueText(score) + " — must be in " + allowed})
			continue
		}
		out = append(out, Edge{Source: id, Target: target, Relation: relation, TargetKind: targetKind,
			Bearing: bearing})
	}
	return out
}

// frontmatterFloat is s as a value, nil where s names none: a missing key, the
// pending literal, or text that is no number.
func frontmatterFloat(s string) *float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return &v
	}
	return nil
}

// parseExperimentLeaf is ParseExperimentLeaf, recording each pair line's fault
// in faults.
func parseExperimentLeaf(text, rel string, faults *[]AnnotationFault) ([]ExperimentNode, error) {
	fm, err := documentFrontmatter(text, rel)
	if err != nil {
		return nil, err
	}
	decls, err := declarations(fm, ExperimentNodesKey, ExpIDKey, rel)
	if err != nil || len(decls) == 0 {
		return nil, err
	}
	if v, ok := fm["experiments"]; ok && (v.IsList || v.IsBool || v.Str != "") {
		return nil, malformed("%s: experiment-hosting leaf carries experiments: — an owning experiment leaf must not also reference other experiments", rel)
	}
	heading := FirstHeading(StripFrontmatter(text))
	var nodes []ExperimentNode
	for _, d := range decls {
		id := d.Str(ExpIDKey)
		if !expIDFullRE.MatchString(id) {
			return nil, malformed("%s: experiment-hosting leaf has malformed exp-id %q", rel, id)
		}
		status, ok := d.Frontmatter[StatusKey]
		if !ok || status.IsList || status.IsBool || (status.Str != "run" && status.Str != "pending") {
			return nil, malformed("%s: experiment %s has invalid status (expected 'run' or 'pending')", rel, id)
		}
		nodes = append(nodes, ExperimentNode{ID: id, Title: heading, CanonicalPath: rel, CanonicalAnchor: Slugify(heading),
			Status: status.Str, Rigor: frontmatterFloat(d.Str(RigorKey)),
			Supports: scorePairs(d.Frontmatter, SupportsKey, RelationSupports, NodeKindExperiment, rel, id, faults),
			Lifts:    scorePairs(d.Frontmatter, LiftsKey, RelationLifts, NodeKindExperiment, rel, id, faults),
			Refutes:  scorePairs(d.Frontmatter, RefutesKey, RelationRefutes, NodeKindExperiment, rel, id, faults)})
	}
	return nodes, nil
}

// parseAnalysisLeaf is ParseAnalysisLeaf, recording each pair line's fault in
// faults.
func parseAnalysisLeaf(text, rel string, faults *[]AnnotationFault) ([]AnalysisNode, error) {
	fm, err := documentFrontmatter(text, rel)
	if err != nil {
		return nil, err
	}
	decls, err := declarations(fm, AnalysisNodesKey, AnaIDKey, rel)
	if err != nil || len(decls) == 0 {
		return nil, err
	}
	heading := FirstHeading(StripFrontmatter(text))
	var nodes []AnalysisNode
	for _, d := range decls {
		id := d.Str(AnaIDKey)
		if !anaIDFullRE.MatchString(id) {
			return nil, malformed("%s: analysis-hosting leaf has malformed ana-id %q", rel, id)
		}
		// An analysis lifts and refutes; a supports list it declares names
		// pairings the table refuses, so it is read for its faults alone.
		scorePairs(d.Frontmatter, SupportsKey, RelationSupports, NodeKindAnalysis, rel, id, faults)
		nodes = append(nodes, AnalysisNode{ID: id, Title: heading, CanonicalPath: rel, CanonicalAnchor: Slugify(heading),
			Lifts:   scorePairs(d.Frontmatter, LiftsKey, RelationLifts, NodeKindAnalysis, rel, id, faults),
			Refutes: scorePairs(d.Frontmatter, RefutesKey, RelationRefutes, NodeKindAnalysis, rel, id, faults)})
	}
	return nodes, nil
}

// parseIndex is a kind: index or entry-point document's record, or nil.
func parseIndex(text, rel string) (*IndexRecord, error) {
	fm, err := documentFrontmatter(text, rel)
	if err != nil {
		return nil, err
	}
	if len(fm) == 0 {
		return nil, nil
	}
	kind := fm.Kind()
	if kind != DocumentIndex && kind != DocumentEntryPoint {
		return nil, nil
	}
	claims, err := fm.ListOrEmpty("subtree-claims")
	if err != nil {
		return nil, malformed("%s: subtree-claims: %v", rel, err)
	}
	exps, err := fm.ListOrEmpty("subtree-experiments")
	if err != nil {
		return nil, malformed("%s: subtree-experiments: %v", rel, err)
	}
	rec := &IndexRecord{Path: rel, Kind: kind, DeclaredSubtreeClaims: claims}
	for _, e := range exps {
		if strings.HasPrefix(e, "exp-") {
			rec.DeclaredSubtreeExperiments = append(rec.DeclaredSubtreeExperiments, e)
		}
	}
	return rec, nil
}

// AxiomSource is the file a KB's axioms are declared in — axioms.md, else the
// legacy invariants.md, else the legacy AGENTS.md — or "" where none is a
// file.
func AxiomSource(src common.Files) string {
	for _, name := range []string{AxiomsFile, InvariantsFile, AgentsFile} {
		if p := path.Join(KBDir, name); hasFile(src, p) {
			return p
		}
	}
	return ""
}

// ParseAxioms is every axiom the KB declares: axioms.md's declarations, or a
// legacy document's INVARIANT headings and then its Axiom bullets.
func ParseAxioms(src common.Files) ([]AxiomNode, error) {
	source := AxiomSource(src)
	if source == "" {
		return nil, nil
	}
	text, err := readText(src, source)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(source)
	if name == AxiomsFile {
		return parseAxiomsDocument(text, name), nil
	}
	return parseLegacyAxioms(text, name), nil
}

// parseAxiomsDocument is every "### Axiom <label>: Title" declaration of
// axioms.md, anchored at its own heading's slug.
func parseAxiomsDocument(text, name string) []AxiomNode {
	var nodes []AxiomNode
	for _, line := range SplitLines(text) {
		if m := axiomHeading.FindStringSubmatch(line); m != nil {
			nodes = append(nodes, AxiomNode{NodeKindAxiom, AxiomPrefix + m[1], Strip(m[2]), name, Slugify(Strip(line[4:]))})
		}
	}
	return nodes
}

// parseLegacyAxioms is every INVARIANT heading and then every Axiom bullet of
// a document axioms.md does not replace. A bullet anchors at INVARIANT-S2's
// heading, and at "" where the document has none.
func parseLegacyAxioms(text, name string) []AxiomNode {
	lines := SplitLines(text)
	var nodes []AxiomNode
	numbering := ""
	for _, line := range lines {
		if m := invariantHeading.FindStringSubmatch(line); m != nil {
			anchor := Slugify(Strip(line[4:]))
			nodes = append(nodes, AxiomNode{NodeKindAxiom, AxiomPrefix + strings.TrimPrefix(m[1], "INVARIANT-"), Strip(m[2]), name, anchor})
			if m[1] == "INVARIANT-S2" {
				numbering = anchor
			}
		}
	}
	for _, line := range lines {
		if m := axiomBulletRE.FindStringSubmatch(line); m != nil {
			nodes = append(nodes, AxiomNode{NodeKindAxiom, AxiomPrefix + m[1], Strip(m[2]), name, numbering})
		}
	}
	return nodes
}
