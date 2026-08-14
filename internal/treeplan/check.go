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
// wrong produces a nonsense depth number, so depth is not asked first.
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

	levels, err := checkTree(s)
	if err != nil {
		return err
	}
	if err := checkCaps(s, levels); err != nil {
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
// directory under the name grammar. It returns each node's level.
func checkTree(s TreePlan) (map[string]int, error) {
	if len(s.Nodes) == 0 {
		return nil, DefectError{Reason: "the artifact holds no nodes"}
	}
	if s.Nodes[0].Kind != KindEntryPoint || s.Nodes[0].Path != entryPointPath || s.Nodes[0].Parent != "" {
		return nil, DefectError{Subject: s.Nodes[0].Path,
			Reason: "the first node is not the parentless entry-point; nodes run depth-first, parents first"}
	}

	levels := map[string]int{entryPointPath: 1}
	kinds := map[string]Kind{entryPointPath: KindEntryPoint}
	for i, n := range s.Nodes {
		if i == 0 {
			continue
		}
		if _, dup := levels[n.Path]; dup {
			return nil, DefectError{Subject: n.Path, Reason: "two nodes share a path"}
		}
		if n.Kind == KindEntryPoint {
			return nil, DefectError{Subject: n.Path, Reason: "a second entry-point"}
		}
		if n.Kind != KindIndex && n.Kind != KindLeaf {
			return nil, DefectError{Subject: n.Path, Reason: fmt.Sprintf("unknown node kind %q", n.Kind)}
		}
		pl, ok := levels[n.Parent]
		if !ok {
			return nil, DefectError{Subject: n.Path, Reason: fmt.Sprintf(
				"names parent %q, which is not a node already listed", n.Parent)}
		}
		if kinds[n.Parent] == KindLeaf {
			return nil, DefectError{Subject: n.Path, Reason: "its parent is a page, which holds no children"}
		}
		if err := checkPathGrammar(n); err != nil {
			return nil, err
		}
		if normalizeLabel(n.Title) == "" {
			return nil, RejectionError{Subject: n.Path, Reason: "a node has no title"}
		}
		levels[n.Path] = pl + 1
		kinds[n.Path] = n.Kind
	}
	return levels, nil
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

// checkCaps is §3.3's depth and fan-out conditions over the composed tree,
// after every §2.7 operator has run.
func checkCaps(s TreePlan, levels map[string]int) error {
	fanOut := map[string]int{}
	for _, n := range s.Nodes {
		if n.Kind != KindLeaf && levels[n.Path] > s.Budgets.DepthCap {
			return RejectionError{Subject: n.Path,
				Reason: "a section sits below the last level the tree has"}
		}
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

// checkDigest is G-2 over the composed tree: for every index, the sum of what
// one summary call would read — leaf children as bodies, index children at
// summaryTokens each (§6.3, per child KIND at any level) — is within
// summaryInputTokens.
//
// This is the guarantee that makes stage 6's per-call input bounded before
// stage 6 exists, which is exactly what §11.7 asks for: provable from the
// tree plan without running the stage.
func (v *Verifier) checkDigest(s TreePlan, parts map[string]survey.Span) error {
	sums := map[string]int{}
	for _, n := range s.Nodes {
		if n.Parent == "" {
			continue
		}
		if n.Kind == KindLeaf {
			r := parts[n.Path]
			sums[n.Parent] += v.tokensOf(groupFile(s, n.SplitGroup), r)
			continue
		}
		sums[n.Parent] += s.Budgets.SummaryTokens
	}
	for _, n := range s.Nodes {
		if n.Kind == KindLeaf {
			continue
		}
		if sums[n.Path] > s.Budgets.SummaryInputTokens {
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

// checkCoverage is §3.3's tiling condition: group spans are disjoint, and
// unioned with the declared annexes they cover every surveyed section.
//
// A chapter silently missing from the KB is caught here even when every link
// resolves — which is the failure this check exists for, and the reason the
// survey's own tiling property is inherited rather than re-derived.
//
// Coverage is stated over SECTIONS and not over bytes: a file's metadata block
// is surveyed as an out-of-band range rather than as a section, and a KB that
// does not distil YAML front matter has not dropped a chapter.
func (v *Verifier) checkCoverage(s TreePlan) error {
	byFile := spansByFile(s.Groups)

	// Groups first, in artifact order, so a failure is reported the same way
	// on every run: a map's iteration order is not an order.
	for _, g := range s.Groups {
		if _, ok := v.file(g.Source.File); !ok {
			return DefectError{Subject: g.ID, Reason: fmt.Sprintf(
				"draws on %s, which the survey does not describe", g.Source.File)}
		}
		if a, ok := annexedBy(g.Source.File, s.Annexes); ok {
			return DefectError{Subject: g.ID, Reason: fmt.Sprintf(
				"draws on %s, which the annex %q excludes", g.Source.File, a)}
		}
	}

	for _, f := range v.art.Files {
		if _, annexed := annexedBy(f.Path, s.Annexes); annexed {
			continue
		}
		spans := byFile[f.Path]
		prev := 0
		for i, r := range spans {
			if r.Start < 0 || r.End <= r.Start || r.End > f.Bytes {
				return DefectError{Subject: f.Path, Reason: fmt.Sprintf(
					"span [%d,%d) is not a range of the %d-byte file", r.Start, r.End, f.Bytes)}
			}
			if i > 0 && r.Start < prev {
				return DefectError{Subject: f.Path, Reason: fmt.Sprintf(
					"two group spans overlap at %d; each source byte belongs to one leaf", r.Start)}
			}
			prev = r.End
		}
		for _, sec := range sectionsOf(f) {
			if !covered(spans, sec) {
				return RejectionError{Subject: fmt.Sprintf("%s [%d,%d)", f.Path, sec.Start, sec.End),
					Reason: "some source material is on no page and in no annex"}
			}
		}
	}
	return nil
}

// covered reports whether a section falls inside one span. The spans are
// disjoint by the time this is asked, so "inside one" and "inside exactly one"
// are the same question.
func covered(spans []survey.Span, sec survey.Span) bool {
	for _, s := range spans {
		if sec.Start >= s.Start && sec.End <= s.End {
			return true
		}
	}
	return false
}
