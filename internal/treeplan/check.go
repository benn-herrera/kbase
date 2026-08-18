package treeplan

import (
	"fmt"
	"strings"

	"kbase/internal/survey"
)

// Check runs §3.3's composed post-conditions over a whole tree plan.
//
// It is the sole statement of what a valid tree plan is. Compose runs it over
// its own output, and stage 9 runs it over an artifact it did not build; both
// therefore mean the same thing by "valid", which is what makes a resumed job
// and a fresh one comparable at all.
//
// The order is chosen so a failure is diagnostic rather than merely true:
// identity, then tree shape, then the caps, then the leaf/group accounting,
// then the budget guarantees, then coverage. A tree whose parent edges are
// wrong makes nonsense of every count taken over it, so shape is asked first.
//
// Failures divide the same way everything else here does. A rejection is
// something the model's grouping could have caused; a defect is something only
// this package could have caused — a duplicate path when the namer guarantees
// uniqueness, a group named by the wrong number of leaves, a part count that
// disagrees with the splitter that produced it (§3.4).
func (v *Verifier) Check(s TreePlan) error {
	if s.Schema != SchemaVersion {
		return DefectError{Reason: fmt.Sprintf("artifact declares schema %q, this build writes %q",
			s.Schema, SchemaVersion)}
	}
	if s.CorpusHash != v.art.Corpus.ContentHash {
		return DefectError{Reason: fmt.Sprintf("artifact plans corpus %s, this verifier holds %s",
			s.CorpusHash, v.art.Corpus.ContentHash)}
	}
	if err := s.Budgets.Validate(); err != nil {
		return err
	}
	if err := checkAnnexes(s.Annexes, v.art); err != nil {
		return err
	}

	if err := checkTree(s); err != nil {
		return err
	}
	if err := checkCaps(s); err != nil {
		return err
	}
	parts, err := v.checkGroups(s)
	if err != nil {
		return err
	}
	if err := v.checkDigest(s, parts); err != nil {
		return err
	}
	return v.checkCoverage(s)
}

// checkTree proves the node list is a tree: one entry-point, unique paths,
// every parent present and already seen, and every path in its parent's own
// directory under the name grammar.
//
// It reads no depth: a tree is as deep as the source made it (R-3).
func checkTree(s TreePlan) error {
	if len(s.Nodes) == 0 {
		return DefectError{Reason: "the artifact holds no nodes"}
	}
	if s.Nodes[0].Kind != KindEntryPoint || s.Nodes[0].Path != entryPointPath || s.Nodes[0].Parent != "" {
		return DefectError{Subject: s.Nodes[0].Path,
			Reason: "the first node is not the parentless entry-point; nodes run depth-first, parents first"}
	}

	kinds := map[string]Kind{entryPointPath: KindEntryPoint}
	for i, n := range s.Nodes {
		if i == 0 {
			continue
		}
		if _, dup := kinds[n.Path]; dup {
			return DefectError{Subject: n.Path, Reason: "two nodes share a path"}
		}
		if n.Kind == KindEntryPoint {
			return DefectError{Subject: n.Path, Reason: "a second entry-point"}
		}
		if n.Kind != KindIndex && n.Kind != KindLeaf {
			return DefectError{Subject: n.Path, Reason: fmt.Sprintf("unknown node kind %q", n.Kind)}
		}
		parent, ok := kinds[n.Parent]
		if !ok {
			return DefectError{Subject: n.Path, Reason: fmt.Sprintf(
				"names parent %q, which is not a node already listed", n.Parent)}
		}
		if parent == KindLeaf {
			return DefectError{Subject: n.Path, Reason: "its parent is a page, which holds no children"}
		}
		if err := checkPathGrammar(n); err != nil {
			return err
		}
		if normalizeLabel(n.Title) == "" {
			return RejectionError{Subject: n.Path, Reason: "a node has no title"}
		}
		kinds[n.Path] = n.Kind
	}
	return nil
}

// checkPathGrammar proves a node's path is the one the namer would have given
// it: inside its parent's directory, and shaped by its kind.
func checkPathGrammar(n Node) error {
	dir := dirOf(n.Parent)
	rest, ok := strings.CutPrefix(n.Path, dir)
	if !ok || rest == "" {
		return DefectError{Subject: n.Path, Reason: fmt.Sprintf("is not inside its parent's directory %q", dir)}
	}
	switch n.Kind {
	case KindLeaf:
		name, ok := strings.CutSuffix(rest, mdExt)
		if !ok || name == "" || strings.Contains(name, "/") || rest == indexFileName {
			return DefectError{Subject: n.Path, Reason: "a page is `{slug}.md` beside its parent index"}
		}
	case KindIndex:
		name, ok := strings.CutSuffix(rest, "/"+indexFileName)
		if !ok || name == "" || strings.Contains(name, "/") {
			return DefectError{Subject: n.Path, Reason: "a section is `{slug}/index.md` below its parent"}
		}
	}
	return nil
}

// checkCaps is §3.3's fan-out conditions over the composed tree, after every
// §2.7 operator has run.
//
// There is no depth condition (R-3, ruled 2026-08-17). Tree depth follows the
// nesting the source graph requires; it is measured and reported, never capped
// and never repaired, because a repair that removes the deepest index level
// removes exactly the level a descended section just created.
func checkCaps(s TreePlan) error {
	fanOut := map[string]int{}
	for _, n := range s.Nodes {
		if n.Parent != "" {
			fanOut[n.Parent]++
		}
	}
	for _, n := range s.Nodes {
		if fanOut[n.Path] > s.Budgets.FanOutCap {
			return RejectionError{Subject: n.Path, Reason: "too many entries under one heading"}
		}
		if n.Kind != KindLeaf && fanOut[n.Path] == 0 {
			return RejectionError{Subject: n.Path, Reason: "a section holds nothing"}
		}
	}
	return nil
}

// checkGroups proves the leaf↔group accounting and G-1, and returns each
// leaf's own byte range.
//
// G-1 is stated as the splitter's own answer rather than as a second token
// comparison: for every group, re-running dissect.Split over SplitGroup.Source at
// SplitGroup.Budget returns exactly SplitGroup.Parts sections. That is what "no leaf's
// span exceeds the per-call budget" means operationally — the mechanical
// fallback stage 4 refines and stage 5 slices IS this list — and stating it
// twice, once as arithmetic and once as the splitter, would be two rules with
// nothing keeping them in agreement.
//
// It is also the only statement the artifact can make about parts, because the
// artifact deliberately holds no interior boundary (I-1, F-2).
//
// The content floor is asked here too, of the same spans: a group carries at
// least dissect.MinTokens of material. Compose's floor operator guarantees it
// by merging, so this is what makes "produced by Compose" and "passes Check"
// one predicate over the floor as well — and what stage 9 re-asks of an
// artifact it did not build.
func (v *Verifier) checkGroups(s TreePlan) (map[string]survey.Span, error) {
	byID := make(map[string]SplitGroup, len(s.Groups))
	for _, g := range s.Groups {
		if _, dup := byID[g.ID]; dup {
			return nil, DefectError{Subject: g.ID, Reason: "two groups share an id"}
		}
		if g.Parts < 1 {
			return nil, DefectError{Subject: g.ID, Reason: fmt.Sprintf("claims %d parts", g.Parts)}
		}
		if g.Budget != s.Budgets.LeafTokens {
			return nil, DefectError{Subject: g.ID, Reason: fmt.Sprintf(
				"was cut against a %d-token budget, the artifact declares %d", g.Budget, s.Budgets.LeafTokens)}
		}
		byID[g.ID] = g
	}

	// Every group's part ranges, from the splitter itself.
	cuts := make(map[string][]survey.Span, len(s.Groups))
	for _, g := range s.Groups {
		got, err := v.split(g.Source)
		if err != nil {
			return nil, starved(g.ID, g.Source, err)
		}
		if len(got) != g.Parts {
			return nil, DefectError{Subject: g.ID, Reason: fmt.Sprintf(
				"claims %d parts and the splitter makes %d of the same span at the same budget",
				g.Parts, len(got))}
		}
		// The content floor, stated over the group's span (the floor operator
		// merges spans, so the span is what it repairs). It carries to every
		// PART of the group for free: the splitter pre-merges below-minimum
		// sections, so a span at or over the floor cannot yield a part under
		// it, and refinement holds the same minimum over the moved boundaries.
		if v.underFloor(g.Source) {
			return nil, underFloorDefect([]Span{g.Source})
		}
		cuts[g.ID] = got
	}

	// Every group named by exactly Parts leaves, parts 1..n, no repeats.
	seen := map[string][]bool{}
	ranges := make(map[string]survey.Span, len(s.Nodes))
	for _, n := range s.Nodes {
		if n.Kind != KindLeaf {
			if n.SplitGroup != "" || n.Part != 0 {
				return nil, DefectError{Subject: n.Path, Reason: "only a page names a group"}
			}
			continue
		}
		g, ok := byID[n.SplitGroup]
		if !ok {
			return nil, DefectError{Subject: n.Path, Reason: fmt.Sprintf(
				"names group %q, which the artifact does not hold", n.SplitGroup)}
		}
		if n.Part < 1 || n.Part > g.Parts {
			return nil, DefectError{Subject: n.Path, Reason: fmt.Sprintf(
				"is part %d of a %d-part group", n.Part, g.Parts)}
		}
		if seen[g.ID] == nil {
			seen[g.ID] = make([]bool, g.Parts)
		}
		if seen[g.ID][n.Part-1] {
			return nil, DefectError{Subject: n.Path, Reason: "two pages are the same part of one group"}
		}
		seen[g.ID][n.Part-1] = true
		ranges[n.Path] = cuts[g.ID][n.Part-1]
	}
	for id, got := range seen {
		for k, ok := range got {
			if !ok {
				return nil, DefectError{Subject: id, Reason: fmt.Sprintf("part %d is on no page", k+1)}
			}
		}
	}
	for _, g := range s.Groups {
		if seen[g.ID] == nil {
			return nil, DefectError{Subject: g.ID, Reason: "no page draws on this group"}
		}
	}
	return ranges, nil
}

// checkDigest is G-2 over the composed tree: for every index, the largest input
// any stage-6 call over its children reads (shelves.calls — the leaves as one
// group, the sections as capped summaries) is within summaryInputTokens.
//
// This is the guarantee that makes stage 6's per-call input bounded before
// stage 6 exists, which is exactly what §11.7 asks for: provable from the
// tree plan without running the stage.
func (v *Verifier) checkDigest(s TreePlan, parts map[string]survey.Span) error {
	sums := map[string]shelves{}
	for _, n := range s.Nodes {
		if n.Parent == "" {
			continue
		}
		sh := sums[n.Parent]
		if n.Kind == KindLeaf {
			sh.addPage(v.tokensOf(groupFile(s, n.SplitGroup), parts[n.Path]))
		} else {
			sh.addSection(s.Budgets.SummaryTokens)
		}
		sums[n.Parent] = sh
	}
	for _, n := range s.Nodes {
		if n.Kind == KindLeaf {
			continue
		}
		if sums[n.Path].calls(s.Budgets.SummaryTokens) > s.Budgets.SummaryInputTokens {
			return RejectionError{Subject: n.Path,
				Reason: "too much material to digest under one heading"}
		}
	}
	return nil
}

// groupFile is the corpus file a group draws from.
func groupFile(s TreePlan, id string) string {
	g, ok := s.SplitGroup(id)
	if !ok {
		return ""
	}
	return g.Source.File
}

// checkCoverage is §3.3's tiling condition: for every non-annexed file the
// group spans exactly TILE the material the survey found, and every span
// endpoint is a boundary the survey drew.
//
// A chapter silently missing from the KB is caught here even when every link
// resolves — which is the failure this check exists for, and the reason the
// survey's own tiling property is inherited rather than re-derived.
//
// Coverage is stated over the boundary set and not over containment. The old
// predicate asked, of every surveyed section at every depth, "does ONE group
// span contain it" — which was an outright prohibition on descent: a container
// whose children became sibling pages is contained by no single span. The
// boundary set asks instead that the spans leave no gap, take no byte outside
// the material, and start and end only where the survey says a section starts.
//
// The two agree wherever the old one was right. Every tiling containment
// accepted this one accepts, and everything containment refused for the right
// reason it still refuses: a page cutting into the middle of a section has an
// endpoint that is no section start, and the section straddling it is exactly
// what made containment fail. What changes is the reason it refused DESCENT —
// a container covered by its own subtree — which was never a defect in the
// tree, and that gaps, which containment caught only by side-effect, are now
// named directly.
//
// It holds no descent rule and knows nothing of Skeleton, which is the point:
// the rule is stated once, where it is decided, and the gate accepts any tiling
// the source's own structure can produce.
//
// Coverage is stated over the surveyed SECTIONS and not over bytes: a file's
// metadata block is surveyed as an out-of-band range rather than as a section,
// and a KB that does not distil YAML front matter has not dropped a chapter.
func (v *Verifier) checkCoverage(s TreePlan) error {
	byFile := spansByFile(s.Groups)

	// Groups first, in artifact order, so a failure is reported the same way
	// on every run: a map's iteration order is not an order.
	for _, g := range s.Groups {
		if _, ok := v.file(g.Source.File); !ok {
			return DefectError{Subject: g.ID, Reason: fmt.Sprintf(
				"draws on %s, which the survey does not describe", g.Source.File)}
		}
		if a, ok := AnnexedBy(g.Source.File, s.Annexes); ok {
			return DefectError{Subject: g.ID, Reason: fmt.Sprintf(
				"draws on %s, which the annex %q excludes", g.Source.File, a)}
		}
	}

	for _, f := range v.art.Files {
		if _, annexed := AnnexedBy(f.Path, s.Annexes); annexed {
			continue
		}
		if err := checkTiling(f, byFile[f.Path]); err != nil {
			return err
		}
	}
	return nil
}

// checkTiling is the boundary-set condition over one file's group spans, which
// arrive sorted by start (spansByFile).
//
// The three claims, in the order a failure is most usefully reported: every
// span lies inside the coverage universe, the spans tile it with no gap and no
// overlap, and every endpoint is a member of the boundary set.
func checkTiling(f survey.File, spans []survey.Span) error {
	secs := sectionsOf(f)
	if len(secs) == 0 {
		// A document the survey found no material in. It has nothing to cover,
		// and a group drawing on it is drawing on bytes nothing described.
		if len(spans) > 0 {
			return DefectError{Subject: f.Path,
				Reason: "a group draws on a file the survey found no material in"}
		}
		return nil
	}
	lo, hi := coverageUniverse(secs)
	if len(spans) == 0 {
		return RejectionError{Subject: fmt.Sprintf("%s [%d,%d)", f.Path, lo, hi),
			Reason: "some source material is on no page and in no annex"}
	}

	bounds := make(map[int]bool, len(secs)+1)
	bounds[lo], bounds[hi] = true, true
	for _, sec := range secs {
		bounds[sec.Start] = true
	}

	// The tiling first, whole: what is on no page is the claim guarantee 6 is
	// about, and a boundary in the wrong place is a diagnosis of second
	// interest to someone whose chapter went missing.
	prev := lo
	for _, r := range spans {
		if r.Start < 0 || r.End <= r.Start || r.End > f.Bytes {
			return DefectError{Subject: f.Path, Reason: fmt.Sprintf(
				"span [%d,%d) is not a range of the %d-byte file", r.Start, r.End, f.Bytes)}
		}
		if r.Start < lo || r.End > hi {
			return DefectError{Subject: f.Path, Reason: fmt.Sprintf(
				"span [%d,%d) reaches outside the material the survey describes, [%d,%d)",
				r.Start, r.End, lo, hi)}
		}
		if r.Start < prev {
			return DefectError{Subject: f.Path, Reason: fmt.Sprintf(
				"two group spans overlap at %d; each source byte belongs to one leaf", r.Start)}
		}
		if r.Start > prev {
			return RejectionError{Subject: fmt.Sprintf("%s [%d,%d)", f.Path, prev, r.Start),
				Reason: "some source material is on no page and in no annex"}
		}
		prev = r.End
	}
	if prev != hi {
		return RejectionError{Subject: fmt.Sprintf("%s [%d,%d)", f.Path, prev, hi),
			Reason: "some source material is on no page and in no annex"}
	}

	// Then the boundary set: the spans tile, and every edge they tile along is
	// an edge the survey drew. This is what forbids a page cutting into the
	// middle of a section — the whole of what the old containment predicate
	// forbade — while permitting a container's body, a run of sibling
	// subsections, or the container's whole subtree on one page.
	for _, r := range spans {
		if !bounds[r.Start] || !bounds[r.End] {
			return RejectionError{Subject: fmt.Sprintf("%s [%d,%d)", f.Path, r.Start, r.End),
				Reason: "a page begins or ends inside a section, not at one"}
		}
	}
	return nil
}

// coverageUniverse is the half-open range a file's pages must tile: from the
// first byte of its material to the last. It is derived from the surveyed
// sections rather than from the file size, which is what keeps an out-of-band
// metadata block out of the tree without a rule of its own.
func coverageUniverse(secs []survey.Span) (lo, hi int) {
	lo, hi = secs[0].Start, secs[0].End
	for _, s := range secs {
		lo = min(lo, s.Start)
		hi = max(hi, s.End)
	}
	return lo, hi
}
