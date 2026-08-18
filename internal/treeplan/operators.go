package treeplan

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"kbase/internal/dissect"
	"kbase/internal/survey"
)

// The two level-structure operators (§2.7), plus the content floor. The
// descent's answer vocabulary groups SIBLINGS; it cannot add or remove a tree
// LEVEL, and the reachable inputs that need exactly that are repaired here —
// mechanically, with no model call and no new model-facing answer field. The
// grouping semantics the model supplied are preserved; only the level structure
// is rewritten.
//
// # Order and termination
//
// The operators run in one fixed order and not a loop:
//
//	dissolve multi-file groups → merge sub-floor spans → split expansion →
//	interpose to satisfy G-2/fan-out → re-check fan-out and G-2
//
// The floor sits after dissolution because it reads one span per leaf, and
// before expansion because it changes which bytes a group holds — and
// therefore how many parts the splitter makes of it. It runs before
// interposition because it changes fan-out, and interposition is what fan-out
// is measured for.
//
// There is no collapse operator, and no depth cap for one to serve (R-3, ruled
// 2026-08-17). Tree depth follows the nesting the source graph requires: the
// deepest index level is exactly the level a descended section creates, so an
// operator whose job was to delete it would silently undo the structure the
// source asked for. Depth is measured and reported, never capped.
//
// Split expansion sits between dissolution and interposition because it is
// what makes G-2 answerable: an oversized span becomes n sibling leaves, which
// is n entries in the parent's fan-out and n token counts in its digest sum.

// walkParent is walk with the parent carried alongside.
func walkParent(n *buildNode, level int, parent *buildNode, fn func(*buildNode, int, *buildNode)) {
	fn(n, level, parent)
	for _, c := range n.children {
		walkParent(c, level+1, n, fn)
	}
}

// dissolve is the multi-file operator: a group holding more than one source
// span cannot be one leaf, because SplitGroup.Source is a single span in a
// single file. Its spans become sibling leaves in its place.
//
// It is applied at every level, because the impossibility is the same one
// everywhere: the artifact cannot represent the node either way, and dissolving
// in place is the repair that changes the least. It never fires on a
// well-formed descent, since a container with several files is enumerated as a
// container.
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

// floor is the content floor (ARCHITECTURE §4 row 3): a leaf span holding less
// material than dissect's §9 minimum is MERGED into an adjacent group of the
// same file rather than becoming a page of its own.
//
// The floor is dissect.MinTokens and not a number of this package's own. The
// stage below already refuses to leave a section that small standing — a
// fragment nobody would have cut on purpose — and a page nobody would have
// routed to on purpose is the same judgement one seam earlier. A tree plan is
// where it can be repaired: coverage is exactly-once, so the bytes have to sit
// on SOME page, and the only mechanism that can decide the page should not
// exist is the one that assigns the spans.
//
// # What a legal merge is
//
// The neighbour is the span immediately before this one in the FILE's own byte
// order, and only when the two touch. Two conditions, and both are
// load-bearing:
//
//   - Immediate. Merging across an intervening span would swallow a third
//     leaf's bytes, and each source byte belongs to one leaf.
//   - Touching. Merging over a gap would deliver bytes no group planned, and
//     would silently close a coverage hole that §3.3's tiling check exists to
//     refuse. A merge may only re-home material, never adopt some.
//
// A fragment with no touching predecessor — a file's first span, or one after
// a gap — merges FORWARD into its successor instead, which is
// dissect.premerge's rule at the seam below it and for the same reason. The
// absorbed node is the small one either way, so the surviving
// page keeps the title and scope of the material that dominates it — which is
// what makes the empty "Introduction to <file>" preamble page disappear rather
// than grow a body under a name nobody meant.
//
// # The parent boundary
//
// A touching neighbour UNDER THE SAME PARENT is preferred over a touching
// neighbour under a different one, in both directions, and that preference
// beats the backward-first order above [ARCH F2]. The merge is keyed on file
// byte order, and file byte order does not respect the tree: where the descent
// placed two adjacent top-level sections of one file under different subtopic
// indexes — legal and unremarkable — an unscoped merge re-homes a fragment
// under an index that never promised it, and can empty the index it left.
//
// A cross-parent merge is still TAKEN where it is the only legal one, because
// refusing would make a buildable corpus unbuildable over a defect the corpus
// does not have. It is logged at warn and counted into
// TreePlan.CrossParentMerges (SPEC §3.3, §3.8) so the effect is measured rather
// than invisible: no gate observes it — coverage is per-file and per-section,
// and gate 5 compares the delivered set against the plan that already contains
// the repair.
//
// Merging cascades: a neighbour that is still under the floor after absorbing a
// fragment is itself a fragment, and is folded on the next pass.
//
// # The sole-span exemption
//
// The floor applies to SPLITS, not to documents (ruled 2026-08-17): a span that
// covers everything its file holds is exempt and stands as its own leaf at any
// size. The floor's target is a page manufactured out of part of a larger
// document — a fragment nobody would have cut on purpose — and an author's
// whole tiny document behind its own title is not that: it is an honest page,
// and the routing surface promises exactly what it delivers. So the exemption
// lives in underFloor, where both this operator and Check's re-ask read it.
//
// Merging several tiny FILES into one leaf is a different feature (it would
// have to name the merged page something no document is called) and is not
// implemented.
//
// # When there is no legal merge
//
// A span under the floor, not covering its whole file, and with no touching
// neighbour in that file is a fragment no regrouping reaches — the spans of a
// file are what the survey found — so this is a DefectError and not a
// Rejection: retrying re-asks a question that was never asked wrong. It names
// every such span at once, because an operator fixing a corpus wants the list
// and not the first entry ten times.
func (v *Verifier) floor(root *buildNode) (int, error) {
	byFile := map[string][]*buildNode{}
	parent := map[*buildNode]*buildNode{}
	walkParent(root, 1, nil, func(n *buildNode, _ int, p *buildNode) {
		parent[n] = p
		if n.kind == KindLeaf && len(n.sources) == 1 {
			byFile[n.sources[0].File] = append(byFile[n.sources[0].File], n)
		}
	})

	dead := map[*buildNode]bool{}
	var stranded []Span
	crossed := 0
	// The survey's own file order, so the refusal reads the same way on every
	// run without a sort of ours.
	for _, f := range v.art.Files {
		leaves := byFile[f.Path]
		if len(leaves) == 0 {
			continue
		}
		sort.Slice(leaves, func(i, j int) bool {
			return leaves[i].sources[0].Start < leaves[j].sources[0].Start
		})
		s, n := v.mergeFragments(leaves, parent, dead)
		stranded = append(stranded, s...)
		crossed += n
	}
	if len(stranded) > 0 {
		return 0, underFloorDefect(stranded)
	}
	prune(root, dead)
	return crossed, nil
}

// mergeFragments folds one file's sub-floor leaves into their neighbours,
// marking each absorbed node dead. It returns the spans it could not place and
// how many of the merges it made crossed a parent index boundary.
//
// The candidate order is the whole of the parent-boundary rule (see floor): a
// touching neighbour under the same parent in either direction, and only then a
// touching neighbour under a different one.
//
// Termination: every iteration removes one entry from the working list, either
// by merging it away or by stranding it.
func (v *Verifier) mergeFragments(leaves []*buildNode, parent map[*buildNode]*buildNode,
	dead map[*buildNode]bool) ([]Span, int) {

	var stranded []Span
	crossed := 0
	for {
		i := -1
		for j, n := range leaves {
			if v.underFloor(n.sources[0]) {
				i = j
				break
			}
		}
		if i < 0 {
			return stranded, crossed
		}
		frag := leaves[i]
		span := frag.sources[0]
		back, fwd := -1, -1
		if i > 0 && leaves[i-1].sources[0].End == span.Start {
			back = i - 1
		}
		if i+1 < len(leaves) && leaves[i+1].sources[0].Start == span.End {
			fwd = i + 1
		}
		host := -1
		for _, c := range []int{back, fwd} {
			if c >= 0 && parent[leaves[c]] == parent[frag] {
				host = c
				break
			}
		}
		if host < 0 {
			for _, c := range []int{back, fwd} {
				if c >= 0 {
					host = c
					break
				}
			}
			if host >= 0 {
				crossed++
				v.lg.Warn("content floor: a sub-floor span merged across a parent index boundary",
					"file", span.File, "start", span.Start, "end", span.End,
					"page", frag.title, "from", parentTitle(parent[frag]),
					"into", leaves[host].title, "under", parentTitle(parent[leaves[host]]))
			}
		}
		switch {
		case host < 0:
			stranded = append(stranded, span)
		case host < i:
			leaves[host].sources[0].End = span.End
			dead[frag] = true
		default:
			leaves[host].sources[0].Start = span.Start
			dead[frag] = true
		}
		leaves = append(leaves[:i], leaves[i+1:]...)
	}
}

// parentTitle names a node's parent for the warn above, and says so when there
// is none rather than logging an empty string.
func parentTitle(n *buildNode) string {
	if n == nil {
		return "(the entry-point)"
	}
	return n.title
}

// underFloor reports whether a span holds less material than a delivered page
// may carry, through dissect's own predicate and constant: the tree-plan seam
// and the cut-list seam ask one question, spelled once.
//
// A span this verifier cannot measure — a file it holds no bytes for, a range
// outside them — is not under the floor. Those are defects with their own
// names (checkSources, split), and answering "too small" here would report the
// wrong one.
//
// Neither is a span that covers its whole file: that is the sole-span
// exemption, and it is asked here so that the operator, Check's re-ask and
// anything later reading the floor read one predicate (see floor).
func (v *Verifier) underFloor(span Span) bool {
	src, ok := v.bytes(span.File)
	if !ok || span.Start < 0 || span.End <= span.Start || span.End > len(src) {
		return false
	}
	if v.wholeFile(span) {
		return false
	}
	return dissect.Params{Est: v.p.Est}.UnderMinimum(src, survey.Span{Start: span.Start, End: span.End})
}

// wholeFile reports whether span covers everything the survey says its file
// holds to be covered — the coverage universe, `preamble` + `sections`, which
// excludes a front-matter block (SPEC §3.2).
//
// Coverage is exactly-once, so a span that covers that universe is necessarily
// its file's SOLE span, which is why the exemption can be asked of one span
// without consulting the others.
func (v *Verifier) wholeFile(span Span) bool {
	f, ok := v.file(span.File)
	if !ok {
		return false
	}
	secs := sectionsOf(f)
	if len(secs) == 0 {
		return false
	}
	lo, hi := coverageUniverse(secs)
	return span.Start == lo && span.End == hi
}

// underFloorDefect is the floor's loud refusal: every span it could not place,
// named, in one message.
func underFloorDefect(spans []Span) error {
	named := make([]string, 0, len(spans))
	for _, s := range spans {
		named = append(named, fmt.Sprintf("%s [%d,%d)", s.File, s.Start, s.End))
	}
	return DefectError{Reason: fmt.Sprintf(
		"the corpus holds material no page can carry: %s — each is under the %d-token minimum, "+
			"with no adjacent material in the same file to merge it into; a span covering its whole "+
			"file is exempt from the floor, so each of these is a fragment whose file has material "+
			"the plan left on no page",
		strings.Join(named, ", "), dissect.MinTokens)}
}

// prune drops the leaves the floor absorbed, and any index left holding
// nothing by their departure.
//
// An index whose every child was absorbed into a page under some other parent
// routes nowhere, and Check refuses one ("a section holds nothing"). Removing
// it is the same repair dissolution makes: the tree keeps only what the
// artifact can represent. The root is never removed — an entry-point with
// no children is a corpus with no material, and Check is where that is said.
func prune(n *buildNode, dead map[*buildNode]bool) {
	out := make([]*buildNode, 0, len(n.children))
	for _, c := range n.children {
		if dead[c] {
			continue
		}
		if c.kind != KindLeaf {
			prune(c, dead)
			if len(c.children) == 0 {
				continue
			}
		}
		out = append(out, c)
	}
	n.children = out
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

// shelves is what one index's children cost stage 6, accumulated by kind: the
// pages' own bodies on one side, one summary cap per index child on the other.
// It is the accumulator both G-2 sites fill — the operator here, from the working
// tree, and the post-condition in check.go, from the composed artifact — so the
// rule below has one statement and two feeders.
type shelves struct {
	// pages is the direct leaves' bodies, summed; leaves says whether there was
	// one at all, which a zero-token page would otherwise hide.
	pages  int
	leaves bool
	// sections is one summary cap per index child.
	sections int
}

func (sh *shelves) addPage(tokens int) {
	sh.pages += tokens
	sh.leaves = true
}

func (sh *shelves) addSection(summaryCap int) { sh.sections += summaryCap }

// calls is G-2's left-hand side: the largest input any stage-6 call over these
// children reads.
//
// There are up to TWO calls (§4 row 6). A node's direct leaves are always
// summarised as a group in isolation, so their bodies are one call's whole input.
// Where the node holds index children too, that group's card — capped like any
// other summary — is what the node's own call reads in the pages' place, beside
// one capped summary per index child. So the two calls are bounded separately and
// the node is bounded by the larger of them: a page body never shares a call with
// a subsection's summary, which is what the per-shelf cost below encodes.
//
// Where there are no index children the group call IS the node's summary call,
// and its bodies are the whole cost.
func (sh shelves) calls(summaryCap int) int {
	if sh.sections == 0 {
		return sh.pages
	}
	own := sh.sections
	if sh.leaves {
		own += summaryCap
	}
	return max(sh.pages, own)
}

// digestSum is G-2's left-hand side for one working node.
func (v *Verifier) digestSum(n *buildNode) int {
	var sh shelves
	for _, c := range n.children {
		v.digest(&sh, c)
	}
	return sh.calls(v.p.Budgets.SummaryTokens)
}

func (v *Verifier) digest(sh *shelves, n *buildNode) {
	if n.kind == KindLeaf {
		sh.addPage(n.tokens)
		return
	}
	sh.addSection(v.p.Budgets.SummaryTokens)
}

// batch is the greedy order-preserving partition interposition uses. The cost it
// accumulates is the same shelves arithmetic digestSum states — each batch
// becomes one index node, so the bound a batch is closed against is the bound
// that node will be checked against.
func (v *Verifier) batch(children []*buildNode) [][]*buildNode {
	var out [][]*buildNode
	var cur []*buildNode
	var sh shelves
	for _, c := range children {
		next := sh
		v.digest(&next, c)
		if len(cur) > 0 && (len(cur)+1 > v.p.Budgets.FanOutCap ||
			next.calls(v.p.Budgets.SummaryTokens) > v.p.Budgets.SummaryInputTokens) {
			out = append(out, cur)
			cur, next = nil, shelves{}
			v.digest(&next, c)
		}
		cur = append(cur, c)
		sh = next
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// recheck is the last line of §2.7's fixed order: the fan-out cap and G-2,
// once, after every operator has run. A breach here is a rejection and not
// another rewrite — the operator that would repair it is the one that caused
// it.
//
// Depth is not re-checked, because nothing caps it (R-3): interposition adds
// levels freely, and the levels it adds are the price of a fan-out the source
// itself presented.
func (v *Verifier) recheck(root *buildNode) error {
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
