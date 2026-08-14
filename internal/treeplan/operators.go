package treeplan

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"kbase/internal/dissect"
	"kbase/internal/survey"
)

// The three level-structure operators (§2.7). The descent's answer vocabulary
// groups SIBLINGS; it cannot add or remove a tree LEVEL, and three reachable
// inputs need exactly that. All three are repaired here — mechanically, with
// no model call and no new model-facing answer field. The grouping semantics
// the model supplied are preserved; only the level structure is rewritten.
//
// # Order and termination
//
// The operators trade against the same depth budget, so they run in one fixed
// order and not a loop:
//
//	collapse until depth ≤ cap → dissolve cap-level multi-file groups →
//	split expansion → interpose to satisfy G-2/fan-out → re-check both caps
//
// Collapse strictly decreases depth and interposition strictly increases it,
// so an interposition that re-breaches the depth cap is NOT re-collapsed: it
// is a rejection carrying the retry note, the model is asked once to
// regroup flatter, and a second failure fails the unit loudly (I-7). That is
// the one shape kbase genuinely cannot host, and it says so rather than
// looping.
//
// Split expansion sits between dissolution and interposition because it is
// what makes G-2 answerable: an oversized span becomes n sibling leaves, which
// is n entries in the parent's fan-out and n token counts in its digest sum.

// collapse removes index levels until the tree is within the depth cap.
//
// It prefers a single-child chain — an index whose only child is another index
// — because compressing one costs nothing: the level carried no grouping
// decision, only nesting. The outermost container's title and scope survive
// and the innermost's children re-parent to it. Where that is not enough,
// contiguous ancestors are merged the same way, deepest boundary first.
//
// It never merges two source files into one leaf: only index nodes are
// removed, and a leaf keeps its own span, its own title and its own identity
// wherever it lands.
//
// It runs only while the tree is over the cap. An index with one index child
// that fits is a shape the model chose, and collapsing it unasked would also
// collapse the exemplar's single-child subtopic directories, which are a
// budget artifact and correct (§0.4).
//
// Termination: every iteration removes exactly one index node from the tree,
// so the loop cannot run more times than the tree has index nodes, and a tree
// with one index node is at depth 1.
func collapse(root *buildNode, depthCap int) {
	for {
		deepest := maxIndexLevel(root)
		if deepest <= depthCap {
			return
		}
		if n := pickChain(root, deepest); n != nil {
			absorb(n)
			continue
		}
		if n, parent := pickMerge(root, deepest); n != nil {
			mergeInto(n, parent)
			continue
		}
		return // unreachable: over the cap means some index node has a parent
	}
}

// pickChain finds the deepest index node whose only child is an index node and
// whose subtree reaches the deepest level — so absorbing it makes progress
// against the cap rather than flattening some unrelated branch.
func pickChain(root *buildNode, deepest int) *buildNode {
	var best *buildNode
	bestLevel := 0
	walkParent(root, 1, nil, func(n *buildNode, level int, _ *buildNode) {
		if n.kind == KindLeaf || len(n.children) != 1 || n.children[0].kind == KindLeaf {
			return
		}
		if subtreeIndexLevel(n, level) != deepest || level <= bestLevel {
			return
		}
		best, bestLevel = n, level
	})
	return best
}

// pickMerge finds the deepest index node with an index parent: the one whose
// children re-parent upward when no single-child chain is left to compress.
func pickMerge(root *buildNode, deepest int) (*buildNode, *buildNode) {
	var best, bestParent *buildNode
	walkParent(root, 1, nil, func(n *buildNode, level int, parent *buildNode) {
		if n.kind == KindLeaf || parent == nil || level != deepest || best != nil {
			return
		}
		best, bestParent = n, parent
	})
	return best, bestParent
}

// absorb compresses one link of a single-child chain: n takes over its only
// child's children, and the child disappears.
func absorb(n *buildNode) {
	child := n.children[0]
	n.children = child.children
	reparent(n.children, child.title)
}

// mergeInto removes n and re-parents its children to parent, which keeps its
// own title and scope.
func mergeInto(n, parent *buildNode) {
	out := make([]*buildNode, 0, len(parent.children)+len(n.children)-1)
	for _, c := range parent.children {
		if c == n {
			out = append(out, n.children...)
			continue
		}
		out = append(out, c)
	}
	parent.children = out
	reparent(n.children, n.title)
}

// reparent records which container a set of nodes came out of, so the namer
// can disambiguate a colliding slug by prefixing it (§2.7).
func reparent(nodes []*buildNode, origin string) {
	slug := Slug(origin)
	for _, c := range nodes {
		c.origin = slug
	}
}

// subtreeIndexLevel is the deepest level an index node sits at within n's
// subtree, with n itself at level.
func subtreeIndexLevel(n *buildNode, level int) int {
	max := 0
	walk(n, level, func(c *buildNode, l int) {
		if c.kind != KindLeaf && l > max {
			max = l
		}
	})
	return max
}

// walkParent is walk with the parent carried alongside.
func walkParent(n *buildNode, level int, parent *buildNode, fn func(*buildNode, int, *buildNode)) {
	fn(n, level, parent)
	for _, c := range n.children {
		walkParent(c, level+1, n, fn)
	}
}

// dissolve is §2.7's third operator: a group holding more than one source span
// cannot be one leaf, because SplitGroup.Source is a single span in a single file.
// Its spans become sibling leaves in its place.
//
// The design names this at the deepest permitted index level, where the pincer
// bites — a group with ≥2 source files can be neither a leaf (multi-file) nor
// an index (no level left). It is applied at every level here because the
// impossibility is the same one everywhere: the artifact cannot represent the
// node either way, and dissolving in place is the repair that changes the
// least. Below the cap it never fires on a well-formed descent, since a
// container with several files is enumerated as a container.
//
// Each resulting leaf is named after its source file — the survey's title for
// it, or its base name — and keeps the dissolved group's scope line. Where two
// resulting leaves draw on the same file they share a title, and the namer
// gives them distinct paths.
func (v *Verifier) dissolve(root *buildNode) {
	walk(root, 1, func(n *buildNode, _ int) {
		if n.kind == KindLeaf {
			return
		}
		out := make([]*buildNode, 0, len(n.children))
		for _, c := range n.children {
			if c.kind != KindLeaf || len(c.sources) <= 1 {
				out = append(out, c)
				continue
			}
			for _, s := range c.sources {
				out = append(out, &buildNode{
					title:   v.fileLabel(s.File),
					scope:   c.scope,
					kind:    KindLeaf,
					sources: []Span{s},
					origin:  Slug(c.title),
				})
			}
		}
		n.children = out
	})
}

// fileLabel is what a dissolved group's leaf is titled: the survey's title for
// the file, or its base name when the document does not name itself.
func (v *Verifier) fileLabel(file string) string {
	if f, ok := v.file(file); ok && f.Title != "" {
		return normalizeLabel(f.Title)
	}
	base := path.Base(file)
	return normalizeLabel(strings.TrimSuffix(base, path.Ext(base)))
}

// expand is §2.4: every leaf span is run through dissect.Split at the leaf
// budget, and a span that needs n parts becomes n sibling leaves sharing one
// SplitGroup.
//
// Split runs over EVERY span, not only the ones an estimate says are over
// budget. It is the same answer — Split's loop does nothing to a span already
// under budget — and it makes the artifact's own acceptance criterion true by
// construction rather than by a second arithmetic path: for every one-part
// group, Split over its span returns a one-section list; for every multi-part
// group, exactly SplitGroup.Parts sections (§3.5).
//
// The parts' names and titles come from the group's title and the part's
// ordinal, and from nothing else. Stage 4 will MOVE the boundaries inside the
// group, and a name derived from where a boundary landed would falsify the
// tree plan the moment refinement ran (O-8).
func (v *Verifier) expand(root *buildNode) ([]SplitGroup, error) {
	groups := make([]SplitGroup, 0, 16)
	var err error
	walk(root, 1, func(n *buildNode, _ int) {
		if err != nil || n.kind == KindLeaf {
			return
		}
		out := make([]*buildNode, 0, len(n.children))
		for _, c := range n.children {
			if c.kind != KindLeaf {
				out = append(out, c)
				continue
			}
			g, parts, perr := v.splitLeaf(c, len(groups)+1)
			if perr != nil {
				err = perr
				return
			}
			groups = append(groups, g)
			out = append(out, parts...)
		}
		n.children = out
	})
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// splitLeaf turns one leaf node into its SplitGroup and the leaves that name it.
func (v *Verifier) splitLeaf(n *buildNode, ordinal int) (SplitGroup, []*buildNode, error) {
	if len(n.sources) != 1 {
		return SplitGroup{}, nil, DefectError{Subject: n.title, Reason: fmt.Sprintf(
			"holds %d source spans at split time; dissolution leaves exactly one", len(n.sources))}
	}
	span := n.sources[0]
	cuts, err := v.split(span)
	if err != nil {
		return SplitGroup{}, nil, starved(n.title, span, err)
	}

	g := SplitGroup{
		ID:     groupID(ordinal),
		Source: span,
		Budget: v.p.Budgets.LeafTokens,
		Parts:  len(cuts),
	}
	if len(cuts) == 1 {
		n.group, n.part, n.parts, n.base = g.ID, 1, 1, n.title
		n.tokens = v.tokensOf(span.File, cuts[0])
		return g, []*buildNode{n}, nil
	}

	parts := make([]*buildNode, 0, len(cuts))
	for k, r := range cuts {
		parts = append(parts, &buildNode{
			title:   partTitle(n.title, k+1, len(cuts)),
			scope:   n.scope,
			kind:    KindLeaf,
			sources: []Span{{File: span.File, Start: r.Start, End: r.End}},
			origin:  n.origin,
			group:   g.ID,
			part:    k + 1,
			parts:   len(cuts),
			base:    n.title,
			tokens:  v.tokensOf(span.File, r),
		})
	}
	return g, parts, nil
}

// groupID is a group's name in the artifact and its directory under `cuts/`,
// so it is a safe path segment by construction.
//
// It is an ordinal rather than a content-derived identity on purpose. The
// tree plan is one whole-corpus artifact: any change to the corpus redoes it
// and invalidates every cut list downstream regardless, so a "stable" id would
// buy nothing and cost a second naming grammar.
func groupID(ordinal int) string { return fmt.Sprintf("g%04d", ordinal) }

// starved wraps a Split refusal as the rejection §2.4 step 4 describes, and
// leaves anything else alone — a bad span is our defect, not the model's.
func starved(subject string, span Span, err error) error {
	var se dissect.StarvedRejection
	if errors.As(err, &se) {
		return StarvedRejection{Subject: subject, File: span.File, Starved: se}
	}
	return DefectError{Subject: subject, Reason: err.Error()}
}

// interpose is §2.7's second operator: an index violating G-2 or the fan-out
// cap has its children partitioned into m ordered batches in tree plan node
// order, each batch parented to a synthetic index one level down.
//
// The batching is greedy and order-preserving: children stay in the order the
// model grouped them, and a batch closes when the next child would breach
// either bound. Budgets.Validate guarantees every batch takes at least one
// child, which is what makes this terminate.
//
// Synthetic titles and scopes are mechanical — the same
// name-independent-of-content discipline as O-8. The alternative, re-asking the
// model to regroup, was rejected because the model already answered this
// question once and a second ask spends a heavy call to relabel a mechanical
// partition. If synthetic titles read badly in the E2E run, that is the
// evidence that flips it.
func (v *Verifier) interpose(root *buildNode) error {
	var indexes []*buildNode
	walk(root, 1, func(n *buildNode, _ int) {
		if n.kind != KindLeaf {
			indexes = append(indexes, n)
		}
	})
	for _, n := range indexes {
		if !v.breaches(n) {
			continue
		}
		batches := v.batch(n.children)
		if len(batches) < 2 {
			// One batch means the breach is not a partitioning problem: a
			// single child too large to digest, which Budgets.Validate makes
			// unreachable.
			return DefectError{Subject: n.title, Reason: "breaches its own budget as one batch"}
		}
		if len(batches) > v.p.Budgets.FanOutCap ||
			len(batches)*v.p.Budgets.SummaryTokens > v.p.Budgets.SummaryInputTokens {
			return RejectionError{Subject: n.title,
				Reason: "too much under one heading; split it into separate sections"}
		}
		out := make([]*buildNode, 0, len(batches))
		for k, b := range batches {
			out = append(out, &buildNode{
				title:    partTitle(n.title, k+1, len(batches)),
				scope:    n.scope,
				kind:     KindIndex,
				children: b,
				origin:   Slug(n.title),
			})
		}
		n.children = out
	}
	return nil
}

// breaches reports whether an index is over the fan-out cap or over G-2.
func (v *Verifier) breaches(n *buildNode) bool {
	return len(n.children) > v.p.Budgets.FanOutCap || v.digestSum(n) > v.p.Budgets.SummaryInputTokens
}

// digestSum is G-2's left-hand side: what one summary call over this index
// would read. The rule is per child KIND, not per tree level (§6.3): a leaf
// child is read as a body, an index child as its own capped summary, and a
// mixed index reads both.
func (v *Verifier) digestSum(n *buildNode) int {
	sum := 0
	for _, c := range n.children {
		sum += v.digest(c)
	}
	return sum
}

func (v *Verifier) digest(n *buildNode) int {
	if n.kind == KindLeaf {
		return n.tokens
	}
	return v.p.Budgets.SummaryTokens
}

// batch is the greedy order-preserving partition interposition uses.
func (v *Verifier) batch(children []*buildNode) [][]*buildNode {
	var out [][]*buildNode
	var cur []*buildNode
	cost := 0
	for _, c := range children {
		d := v.digest(c)
		if len(cur) > 0 && (len(cur)+1 > v.p.Budgets.FanOutCap || cost+d > v.p.Budgets.SummaryInputTokens) {
			out = append(out, cur)
			cur, cost = nil, 0
		}
		cur = append(cur, c)
		cost += d
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// recheck is the last line of §2.7's fixed order: both caps, once, after every
// operator has run. A breach here is a rejection and not another rewrite —
// interposition strictly increases depth and collapse strictly decreases it,
// so repairing this one would re-open the one it just closed.
func (v *Verifier) recheck(root *buildNode) error {
	if got := maxIndexLevel(root); got > v.p.Budgets.DepthCap {
		return RejectionError{Subject: fmt.Sprintf("%d index levels", got),
			Reason: "this needs one more level than the tree has; group flatter"}
	}
	var err error
	walk(root, 1, func(n *buildNode, _ int) {
		if err != nil || n.kind == KindLeaf {
			return
		}
		if len(n.children) > v.p.Budgets.FanOutCap {
			err = RejectionError{Subject: n.title, Reason: "too many entries under one heading"}
			return
		}
		if v.digestSum(n) > v.p.Budgets.SummaryInputTokens {
			err = RejectionError{Subject: n.title, Reason: "too much material to digest under one heading"}
		}
	})
	return err
}

// tokensOf estimates one range of one file, through the threaded estimator.
func (v *Verifier) tokensOf(file string, r survey.Span) int {
	src, ok := v.bytes(file)
	if !ok {
		return 0
	}
	return v.p.Est.EstimateBytes(src[r.Start:r.End])
}
