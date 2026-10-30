package query

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/text/cases"

	"kbase/internal/kb"
)

// ClaimStrength is a claim with the count of its dependents (DependentsOf).
type ClaimStrength struct {
	Claim      kb.NodeRow
	Dependents int
}

// The bar and the dependents floor of stats' highest-leverage weak points.
const (
	DefaultMaxStrength   = 0.65
	DefaultMinDependents = 1
)

// The orders of the strength query.
const (
	SortStrength   = "strength"
	SortDependents = "dependents"
)

// StrengthSorts is the strength query's orders.
var StrengthSorts = []string{SortStrength, SortDependents}

// StrengthSelection selects claims by strength: the scored claims at or above
// Min and strictly below Max, a nil bound not bounding, or under Pending the
// claims with no strength; either way those with at least MinDependents
// dependents. Sort is one of StrengthSorts.
type StrengthSelection struct {
	Min, Max      *float64
	MinDependents int
	Pending       bool
	Sort          string
}

// claims is the claim nodes, sorted by id.
func (ix *Index) claims() []kb.NodeRow {
	var out []kb.NodeRow
	for _, n := range ix.nodes {
		if n.NodeType == kb.NodeKindClaim {
			out = append(out, n)
		}
	}
	return out
}

// sortedSet is set's members, sorted.
func sortedSet(set map[string]bool) []string {
	out := []string{}
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// DependentsOf is the distinct nodes id's strength enters, sorted: the targets
// of id's outgoing scoring edges — the nodes that rest on a claim or analysis,
// and the claim or analysis a lift raises or a refutation caps. A references or
// demoted edge makes neither end a dependent.
func (ix *Index) DependentsOf(id string) []string {
	set := map[string]bool{}
	for _, e := range ix.edges {
		if e.Source == id && kb.Scoring(e.Relation) {
			set[e.Target] = true
		}
	}
	return sortedSet(set)
}

// GatedOn is the claims whose strengthen-by items mention id, sorted.
func (ix *Index) GatedOn(id string) []string {
	set := map[string]bool{}
	for _, item := range ix.strengthenBy {
		if slices.Contains(item.MentionedIDs, id) {
			set[item.ClaimID] = true
		}
	}
	return sortedSet(set)
}

// CitedBy is the citations of id, sorted by leaf path.
func (ix *Index) CitedBy(id string) []kb.CiteRow {
	var out []kb.CiteRow
	for _, c := range ix.cites {
		if c.ClaimID == id {
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b kb.CiteRow) int { return strings.Compare(a.LeafPath, b.LeafPath) })
	return out
}

// Find is the claims whose title or canonical anchor contains query, compared
// case-folded; an empty query matches every claim.
func (ix *Index) Find(query string) []kb.NodeRow {
	fold := cases.Fold()
	needle := fold.String(query)
	var out []kb.NodeRow
	for _, c := range ix.claims() {
		if strings.Contains(fold.String(c.Title), needle) || strings.Contains(fold.String(c.CanonicalAnchor), needle) {
			out = append(out, c)
		}
	}
	return out
}

// ReferencedBy is every leaf, other than the one first citing id, whose body
// (code, maths and fences blanked) holds an inline link resolving to that
// leaf: kb-root-relative paths, sorted. A document is a leaf by its kind as
// the load derives it; one that cannot be read as text is passed over.
func (ix *Index) ReferencedBy(id string) ([]string, error) {
	cites := ix.CitedBy(id)
	if len(cites) == 0 {
		return []string{}, nil
	}
	origin := kb.ResolvePath(ix.src.KBPath(cites[0].LeafPath))
	set := map[string]bool{}
	for rel, kind := range ix.documentKinds {
		file := ix.src.KBPath(rel)
		if kind != kb.DocumentLeaf || kb.ResolvePath(file) == origin {
			continue
		}
		text, err := ix.src.ReadText(file)
		if err != nil {
			continue
		}
		for _, match := range kb.LinkRE.FindAllStringSubmatch(kb.StripCode(text), -1) {
			target := kb.StripTarget(match[1])
			if target == "" {
				continue
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(file), target)
			}
			if kb.ResolvePath(target) == origin {
				set[rel] = true
				break
			}
		}
	}
	return sortedSet(set), nil
}

// SubtreeClaims is the claims under an index node: "" or "." names the first
// entry point, anything else a node path or, failing that, the directory of
// one. Where a path repeats, its last aggregate answers.
func (ix *Index) SubtreeClaims(nodePath string) []string {
	if nodePath == "" || nodePath == "." {
		for _, a := range ix.aggregates {
			if a.NodeKind == kb.DocumentEntryPoint {
				return append([]string{}, a.SubtreeClaims...)
			}
		}
		return []string{}
	}
	byPath := map[string]kb.AggregateRow{}
	for _, a := range ix.aggregates {
		byPath[a.NodePath] = a
	}
	a, ok := byPath[nodePath]
	if !ok {
		a, ok = byPath[strings.TrimRight(nodePath, "/")+"/"+kb.IndexFile]
	}
	if !ok {
		return []string{}
	}
	return append([]string{}, a.SubtreeClaims...)
}

// Strengths is the claims sel selects. SortStrength orders them by ascending
// strength, then id; SortDependents by most dependents, then lowest strength,
// then id. A pending claim has no strength to order by, so among pending
// claims strength drops out of either order.
func (ix *Index) Strengths(sel StrengthSelection) []ClaimStrength {
	var out []ClaimStrength
	for _, c := range ix.claims() {
		if sel.Pending != (c.Strength == nil) {
			continue
		}
		if c.Strength != nil && (sel.Min != nil && *c.Strength < *sel.Min || sel.Max != nil && *c.Strength >= *sel.Max) {
			continue
		}
		if n := len(ix.DependentsOf(c.ID)); n >= sel.MinDependents {
			out = append(out, ClaimStrength{Claim: c, Dependents: n})
		}
	}
	strength := func(c ClaimStrength) float64 {
		if c.Claim.Strength == nil {
			return 0
		}
		return *c.Claim.Strength
	}
	slices.SortStableFunc(out, func(a, b ClaimStrength) int {
		byStrength := cmp.Compare(strength(a), strength(b))
		if sel.Sort == SortDependents {
			return cmp.Or(cmp.Compare(b.Dependents, a.Dependents), byStrength, strings.Compare(a.Claim.ID, b.Claim.ID))
		}
		return cmp.Or(byStrength, strings.Compare(a.Claim.ID, b.Claim.ID))
	})
	return out
}

// Node is the node carrying id, of any kind, or false. Where ids repeat, the
// last in load order answers.
func (ix *Index) Node(id string) (kb.NodeRow, bool) {
	var found kb.NodeRow
	ok := false
	for _, n := range ix.nodes {
		if n.ID == id {
			found, ok = n, true
		}
	}
	return found, ok
}
