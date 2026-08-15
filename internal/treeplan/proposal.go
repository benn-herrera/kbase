package treeplan

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"kbase/internal/survey"
)

// TreeProposal is the descent's composed answer, before any of §2.7's operators have
// run: the tree the model asked for, in the model's own terms.
//
// It is what the taxonomy stage builds by folding one GroupingAnswer per enumerated
// container (§3.2), and it is deliberately not the artifact. Between TreeProposal and
// TreePlan sit the level-structure operators, the split expansion, the namer
// and every post-condition in §3.3 — which is the whole point: the model
// states grouping semantics, and the verifier states the tree.
type TreeProposal struct {
	// Title and Scope are the corpus's own, for the entry-point node.
	Title string
	Scope string
	// Children are the domains.
	Children []ProposalNode
}

// ProposalNode is one proposed node.
//
// A leaf names the source material it draws from and has no children; an index
// names children and no sources. A leaf naming more than one span is legal
// input and is repaired mechanically (§2.7's dissolution): SplitGroup.Source is
// single-file, so a multi-file leaf is not representable in the artifact.
type ProposalNode struct {
	Title    string
	Scope    string
	Kind     Kind
	Sources  []Span
	Children []ProposalNode
}

// buildNode is the mutable tree the operators rewrite. It is unexported
// because every rewrite between TreeProposal and TreePlan is this package's business:
// a caller that could hold one could hold a tree in a state no post-condition
// has been run over.
type buildNode struct {
	title string
	scope string
	kind  Kind

	sources  []Span
	children []*buildNode

	// origin is the slug of a container this node was re-parented out of by
	// chain collapse. The namer tries it as a prefix when a slug collides,
	// which is §2.7's stated disambiguation.
	origin string

	// The split-expansion fields (§2.4), set on leaves by expand.
	group  string
	part   int
	parts  int
	base   string // the title parts' names derive from, before " (k/n)"
	tokens int    // this part's own token count, for G-2
}

// toBuild converts a TreeProposal into the mutable tree, checking what is decidable
// without looking at the corpus: kinds, shape, and non-empty labels.
func toBuild(p TreeProposal) (*buildNode, error) {
	root := &buildNode{
		title: normalizeLabel(p.Title),
		scope: normalizeLabel(p.Scope),
		kind:  KindEntryPoint,
	}
	if root.title == "" {
		return nil, RejectionError{Subject: "entry-point", Reason: "the corpus has no title"}
	}
	if len(p.Children) == 0 {
		return nil, RejectionError{Subject: "entry-point", Reason: "the corpus has no groups at all"}
	}
	for i := range p.Children {
		c, err := toBuildNode(p.Children[i])
		if err != nil {
			return nil, err
		}
		root.children = append(root.children, c)
	}
	return root, nil
}

func toBuildNode(p ProposalNode) (*buildNode, error) {
	n := &buildNode{
		title: normalizeLabel(p.Title),
		scope: normalizeLabel(p.Scope),
		kind:  p.Kind,
	}
	if n.title == "" {
		return nil, RejectionError{Reason: "a group has no title"}
	}
	if n.scope == "" {
		return nil, RejectionError{Subject: n.title, Reason: "a group has no one-line scope"}
	}
	switch p.Kind {
	case KindLeaf:
		if len(p.Children) > 0 {
			return nil, RejectionError{Subject: n.title, Reason: "a page cannot also hold sections"}
		}
		if len(p.Sources) == 0 {
			return nil, RejectionError{Subject: n.title, Reason: "a page draws on no source material"}
		}
		n.sources = append(n.sources, p.Sources...)
	case KindIndex:
		if len(p.Sources) > 0 {
			return nil, RejectionError{Subject: n.title, Reason: "a section holds pages, not source material"}
		}
		if len(p.Children) == 0 {
			return nil, RejectionError{Subject: n.title, Reason: "a section holds nothing"}
		}
		for i := range p.Children {
			c, err := toBuildNode(p.Children[i])
			if err != nil {
				return nil, err
			}
			n.children = append(n.children, c)
		}
	default:
		return nil, RejectionError{Subject: n.title, Reason: "a group is neither a page nor a section"}
	}
	return n, nil
}

// walk visits every node depth-first, parents before children, carrying the
// level (the entry-point is level 1).
func walk(n *buildNode, level int, fn func(*buildNode, int)) {
	fn(n, level)
	for _, c := range n.children {
		walk(c, level+1, fn)
	}
}

// maxIndexLevel is the deepest level an index-kind node sits at.
func maxIndexLevel(root *buildNode) int {
	max := 0
	walk(root, 1, func(n *buildNode, level int) {
		if n.kind != KindLeaf && level > max {
			max = level
		}
	})
	return max
}

// checkSources refuses source material the corpus does not have, or that a
// declared annex has already claimed.
func (v *Verifier) checkSources(root *buildNode, annexes []Annex) error {
	var err error
	walk(root, 1, func(n *buildNode, _ int) {
		if err != nil || n.kind != KindLeaf {
			return
		}
		for _, s := range n.sources {
			f, ok := v.file(s.File)
			if !ok {
				err = DefectError{Subject: n.title, Reason: fmt.Sprintf(
					"draws on %s, which the survey does not describe", s.File)}
				return
			}
			if s.Start < 0 || s.End <= s.Start || s.End > f.Bytes {
				err = DefectError{Subject: n.title, Reason: fmt.Sprintf(
					"span [%d,%d) is not a range of the %d-byte file %s", s.Start, s.End, f.Bytes, s.File)}
				return
			}
			if a, ok := AnnexedBy(s.File, annexes); ok {
				err = DefectError{Subject: n.title, Reason: fmt.Sprintf(
					"draws on %s, which the annex %q excludes from distillation", s.File, a)}
				return
			}
		}
	})
	return err
}

// AnnexedBy reports the declared annex prefix that claims a corpus path.
//
// A prefix claims the directory it names and everything beneath it. It is a
// path-segment comparison rather than a string prefix: `guide` must not claim
// `guidebook.md`.
//
// It is exported because the stage that ENUMERATES the corpus asks the same
// question before this one does — §3.2's descent does not enumerate annexed
// material at all — and two spellings of "does this prefix claim this file"
// is one rule with two places to drift.
func AnnexedBy(file string, annexes []Annex) (string, bool) {
	for _, a := range annexes {
		if file == a.Prefix || strings.HasPrefix(file, a.Prefix+"/") {
			return a.Prefix, true
		}
	}
	return "", false
}

// annexConvention authors an annex's convention line mechanically from the
// survey (§2.8): the path grammar its own files exhibit, the dominant file
// extension under the prefix, and the lexicographically first file as the one
// worked example SPEC §3 requires.
//
// It is authored HERE, and not carried in from the flag that declared the
// prefix, because the text has to be re-derivable: stage 9 re-checks a tree
// plan it did not build, and a hand-written string is the one field it could
// not derive again. A caller that supplies its own Convention keeps it —
// §2.8's deferred `--annex <prefix>=<convention.md>` form enters there and
// nowhere else.
//
// The prefix is known to claim at least one surveyed file (checkAnnexes runs
// first), so the empty return is unreachable through Compose; it is what an
// unvalidated prefix would get rather than a panic.
func annexConvention(art survey.Artifact, prefix string) string {
	var (
		example string
		depth   int
		exts    = map[string]int{}
	)
	for _, f := range art.Files {
		if _, ok := AnnexedBy(f.Path, []Annex{{Prefix: prefix}}); !ok {
			continue
		}
		rel := strings.TrimPrefix(f.Path, prefix+"/")
		if d := strings.Count(rel, "/"); d > depth {
			depth = d
		}
		exts[path.Ext(rel)]++
		if example == "" || f.Path < example {
			example = f.Path
		}
	}
	if example == "" {
		return ""
	}
	return prefix + "/" + strings.Repeat("<dir>/", depth) + "<name>" + dominantExt(exts) +
		", e.g. " + example
}

// dominantExt is the most common extension in the tally, ties broken
// lexicographically. The sort is not cosmetic: a map walk would make the
// composed artifact depend on iteration order, and a tree plan that differs
// between two runs over one corpus is the one thing it may never be.
func dominantExt(tally map[string]int) string {
	best, bestN := "", -1
	for _, ext := range slices.Sorted(maps.Keys(tally)) {
		if n := tally[ext]; n > bestN {
			best, bestN = ext, n
		}
	}
	return best
}

// checkAnnexes validates the declared prefixes against the survey.
//
// This is a configuration fault rather than a model failure or a kbase defect,
// so it is a plain error: `--annex` naming a directory the survey never saw is
// a loud refusal at job setup, never a silently-ignored exclusion (§2.8).
func checkAnnexes(annexes []Annex, art survey.Artifact) error {
	for i, a := range annexes {
		if a.Prefix == "" {
			return fmt.Errorf("treeplan: annex %d declares an empty prefix", i+1)
		}
		if a.Prefix != path.Clean(a.Prefix) || strings.HasPrefix(a.Prefix, "/") {
			return fmt.Errorf("treeplan: annex %q is not a clean corpus-relative prefix", a.Prefix)
		}
		hit := false
		for _, f := range art.Files {
			if f.Path == a.Prefix || strings.HasPrefix(f.Path, a.Prefix+"/") {
				hit = true
				break
			}
		}
		if !hit {
			return fmt.Errorf("treeplan: annex %q names no surveyed document; "+
				"an annex that excludes nothing is a flag that did not do what it said", a.Prefix)
		}
		for j, b := range annexes {
			if i == j {
				continue
			}
			if a.Prefix == b.Prefix || strings.HasPrefix(a.Prefix, b.Prefix+"/") {
				return fmt.Errorf("treeplan: annexes %q and %q nest; declare the outer one only", b.Prefix, a.Prefix)
			}
		}
	}
	return nil
}
