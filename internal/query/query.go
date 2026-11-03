package query

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"golang.org/x/text/cases"

	"kbase/internal/kb"
)

// The bar and the dependents floor of stats' highest-leverage weak points.
const (
	DefaultMaxStrength   = 0.65
	DefaultMinDependents = 1
)

// Two of find's sort keys.
const (
	SortStrength   = "strength"
	SortDependents = "dependents"
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

// dependentCounts is, for every id, the count of the distinct nodes its
// strength enters: the targets of its outgoing scoring edges — the nodes that
// rest on a claim or analysis, and the claim or analysis a lift raises or a
// refutation caps. A references or demoted edge makes neither end a dependent.
func (ix *Index) dependentCounts() map[string]int {
	type pair struct{ source, target string }
	seen := map[pair]bool{}
	counts := map[string]int{}
	for _, e := range ix.edges {
		if p := (pair{e.Source, e.Target}); kb.Scoring(e.Relation) && !seen[p] {
			seen[p] = true
			counts[e.Source]++
		}
	}
	return counts
}

// Citation is one citation of a node in prose: the kb-root-relative path of
// the citing document, its 1-based line, and the link text as written.
type Citation struct {
	Path string
	Line int
	Text string
}

// CitationsOf is every citation of id, read by the id in its link title,
// outside code in every document but the summary pages, which the derivation
// renders; ordered by path, then line.
func (ix *Index) CitationsOf(id string) ([]Citation, error) {
	docs, err := kb.CitingDocuments(ix.src)
	if err != nil {
		return nil, err
	}
	var out []Citation
	for _, rel := range docs {
		text, err := ix.src.ReadText(ix.src.KBPath(rel))
		if err != nil {
			return nil, err
		}
		lines := kb.SplitLines(text)
		for _, c := range kb.TextCitations(text) {
			if c.Title == id {
				out = append(out, Citation{Path: rel, Line: c.Line + 1, Text: lines[c.Line][c.Start+1 : c.Start+1+len(c.Text)]})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b Citation) int { return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line)) })
	return out, nil
}

// SortID is find's default sort key.
const SortID = "id"

// FindSorts is the keys find sorts by.
var FindSorts = []string{SortID, SortStrength, SortDependents}

// The directions a sort takes.
const (
	OrderAscending  = "ascending"
	OrderDescending = "descending"
)

// Orders is the directions a sort takes.
var Orders = []string{OrderAscending, OrderDescending}

// FindSelection is find's question. Every filter set narrows it: Kinds to
// the nodes of those kinds, Title to those whose title contains it
// case-folded, Label likewise to the works and axioms whose label does, Path
// to those hosted under it (kb-root for "" or "."), Min and
// Max to the scored nodes at or above Min and strictly below Max, Pending to
// the nodes with no strength, MinDependents to those with at least that many
// dependents. Sort is nil where none is given, ordering by id; Order is one of
// Orders; Limit is nil where none is given.
type FindSelection struct {
	Kinds         []string
	Title         string
	Label         string
	Path          *string
	Min, Max      *float64
	Pending       bool
	MinDependents int
	Sort          *string
	Order         string
	Limit         *int
}

// FoundNode is one node find answers: its row, its host leaf ("" where it has
// none), its strength (nil where pending) and its dependents' count.
type FoundNode struct {
	Node       kb.NodeRow
	Host       string
	Strength   *float64
	Dependents int
}

// nodeStrength is a node's strength by its kind: the derived value of a claim
// or an analysis, the rigor of every other kind; nil where pending.
func nodeStrength(n kb.NodeRow) *float64 {
	if n.NodeType == kb.NodeKindClaim || n.NodeType == kb.NodeKindAnalysis {
		return n.Strength
	}
	return n.Rigor
}

// hostedUnder is whether a node hosted at host ("" for none) lies under the
// kb-root-relative dir: a directory holding it, or the leaf itself; "" and
// "." name kb-root.
func hostedUnder(host, dir string) bool {
	dir = path.Clean(dir)
	return host != "" && (dir == "." || host == dir || strings.HasPrefix(host, dir+"/"))
}

// Find is the nodes sel selects, in its sort's order, cut to its limit. It
// assumes sel's vocabulary values are valid (FindUsage).
func (ix *Index) Find(sel FindSelection) []FoundNode {
	fold := cases.Fold()
	needle, label := fold.String(sel.Title), fold.String(sel.Label)
	dependents := ix.dependentCounts()
	var out []FoundNode
	for _, n := range ix.nodes {
		found := FoundNode{Node: n, Host: ix.hosts[n.ID], Strength: nodeStrength(n), Dependents: dependents[n.ID]}
		s := found.Strength
		switch {
		case len(sel.Kinds) > 0 && !slices.Contains(sel.Kinds, n.NodeType),
			!strings.Contains(fold.String(n.Title), needle),
			!strings.Contains(fold.String(n.Label), label),
			sel.Path != nil && !hostedUnder(found.Host, *sel.Path),
			sel.Pending && s != nil,
			(sel.Min != nil || sel.Max != nil) && s == nil,
			sel.Min != nil && *s < *sel.Min,
			sel.Max != nil && *s >= *sel.Max,
			found.Dependents < sel.MinDependents:
			continue
		}
		out = append(out, found)
	}
	key := SortID
	if sel.Sort != nil {
		key = *sel.Sort
	}
	descending := sel.Order == OrderDescending
	slices.SortStableFunc(out, func(a, b FoundNode) int {
		byStrength := compareRanks(strengthRank(a.Strength), strengthRank(b.Strength), false)
		byID := strings.Compare(a.Node.ID, b.Node.ID)
		switch key {
		case SortStrength:
			return cmp.Or(compareRanks(strengthRank(a.Strength), strengthRank(b.Strength), descending), byID)
		case SortDependents:
			return cmp.Or(compareRanks(rank{value: float64(a.Dependents)}, rank{value: float64(b.Dependents)}, descending), byStrength, byID)
		}
		if descending {
			return -byID
		}
		return byID
	})
	if sel.Limit != nil {
		out = limited(out, *sel.Limit)
	}
	return out
}

// rank is one row's value of a sort key: a number, pending, or null where the
// key does not apply to the row.
type rank struct {
	value         float64
	pending, null bool
}

// strengthRank is a strength as a sort reads it, nil being pending.
func strengthRank(v *float64) rank {
	if v == nil {
		return rank{pending: true}
	}
	return rank{value: *v}
}

// compareRanks orders a and b ascending, a pending value below every number,
// or descending where descending; a null follows every other value in either
// direction.
func compareRanks(a, b rank, descending bool) int {
	if a.null || b.null {
		return cmp.Compare(boolRank(a.null), boolRank(b.null))
	}
	c := cmp.Or(cmp.Compare(boolRank(!a.pending), boolRank(!b.pending)), cmp.Compare(a.value, b.value))
	if descending {
		return -c
	}
	return c
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// limited is the first n of s, or for a negative n the last -n, in s's order.
func limited[T any](s []T, n int) []T {
	if n < 0 {
		return s[max(len(s)+n, 0):]
	}
	return s[:min(n, len(s))]
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
