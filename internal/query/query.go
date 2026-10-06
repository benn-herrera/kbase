package query

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/text/cases"

	"kbase/internal/kb"
)

// WeakPoint is a scored claim below the weak-points bar with the count of its
// dependents (DependentsOf).
type WeakPoint struct {
	Claim      kb.NodeRow
	Dependents int
}

// Defaults of the weak-points query.
const (
	DefaultMaxSolidity   = 0.65
	DefaultMinDependents = 1
)

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

// DependsOnEdges is every edge sourced at id, sorted by target, file order
// kept among equal targets.
func (ix *Index) DependsOnEdges(id string) []kb.EdgeRow {
	var out []kb.EdgeRow
	for _, e := range ix.edges {
		if e.Source == id {
			out = append(out, e)
		}
	}
	slices.SortStableFunc(out, func(a, b kb.EdgeRow) int { return strings.Compare(a.Target, b.Target) })
	return out
}

// DependentsOf is the distinct nodes whose solidity id's enters, sorted: the
// sources of the depends and rests-on edges targeting id, and the targets of
// id's own strengthens and supports edges. A references or demoted edge makes
// neither end a dependent.
func (ix *Index) DependentsOf(id string) []string {
	set := map[string]bool{}
	for _, e := range ix.edges {
		switch e.Relation {
		case kb.RelationDepends, kb.RelationRestsOn:
			if e.Target == id {
				set[e.Source] = true
			}
		case kb.RelationStrengthens, kb.RelationSupports:
			if e.Source == id {
				set[e.Target] = true
			}
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

// leafKindRE is the first kind: line anywhere in a document.
var leafKindRE = kb.PyRE(`(?m)^\s*kind:\s*(\S+)`)

// ReferencedBy is every leaf, other than the one first citing id, whose body
// (code, maths and fences blanked) holds an inline link resolving to that
// leaf: kb-root-relative paths, sorted. A document is a leaf by its first
// kind: line; one that cannot be read as text is passed over.
func (ix *Index) ReferencedBy(id string) ([]string, error) {
	cites := ix.CitedBy(id)
	if len(cites) == 0 {
		return []string{}, nil
	}
	origin := kb.ResolvePath(ix.src.KBPath(cites[0].LeafPath))
	files, err := kb.MarkdownFiles(ix.src)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, file := range files {
		if kb.ResolvePath(file) == origin {
			continue
		}
		text, err := ix.src.ReadText(file)
		if err != nil {
			continue
		}
		if m := leafKindRE.FindStringSubmatch(text); m == nil || m[1] != kb.DocumentLeaf {
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
				rel, err := filepath.Rel(ix.src.Root(), file)
				if err != nil {
					return nil, err
				}
				set[filepath.ToSlash(rel)] = true
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

// SolidityBelow is the scored claims whose solidity is under threshold,
// ascending by solidity, then id.
func (ix *Index) SolidityBelow(threshold float64) []kb.NodeRow {
	var out []kb.NodeRow
	for _, c := range ix.claims() {
		if c.Solidity != nil && *c.Solidity < threshold {
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b kb.NodeRow) int {
		return cmp.Or(cmp.Compare(*a.Solidity, *b.Solidity), strings.Compare(a.ID, b.ID))
	})
	return out
}

// WeakPoints is the scored claims under maxSolidity with at least
// minDependents dependents: most dependents first, then lowest solidity,
// then id.
func (ix *Index) WeakPoints(maxSolidity float64, minDependents int) []WeakPoint {
	var out []WeakPoint
	for _, c := range ix.claims() {
		if c.Solidity == nil || *c.Solidity >= maxSolidity {
			continue
		}
		if n := len(ix.DependentsOf(c.ID)); n >= minDependents {
			out = append(out, WeakPoint{Claim: c, Dependents: n})
		}
	}
	slices.SortStableFunc(out, func(a, b WeakPoint) int {
		return cmp.Or(cmp.Compare(b.Dependents, a.Dependents), cmp.Compare(*a.Claim.Solidity, *b.Claim.Solidity),
			strings.Compare(a.Claim.ID, b.Claim.ID))
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
