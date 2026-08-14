package distill

import (
	"fmt"
	"strings"
	"unicode"

	"kbase/internal/survey"
	"kbase/internal/treeplan"
)

// The rebase map (§4.4): the one transformation a leaf body is permitted
// (I-3).
//
// It is a FUNCTION, keyed by (source file, fragment), with the three-rule
// landing selection [MAD1: F-1] states:
//
//  1. a fragment naming a source section lands on the leaf hosting that
//     section's start offset — or, when no leaf holds it, on the index that
//     owns it;
//  2. a whole-file link lands on that file's only leaf, or else on the lowest
//     index whose subtree holds every node drawn from the file;
//  3. a file no node was drawn from does not land: the link stays verbatim and
//     is inventoried, and §9 check 1 exempts it rather than failing on a
//     defect in someone else's corpus.
//
// Targets only. Visible text is byte-identical to source, and no prose is ever
// wrapped in new link syntax — kbase takes the mechanical half of the
// exemplar's linking rule and leaves the semantic half alone.
//
// It is a pure function of (tree plan, cut lists, survey link graph), which is
// exactly the triple §9 check 7 re-derives from — which is why the rebase is
// provable rather than merely applied.

// landing is where one source reference lands in the KB.
type landing struct {
	// node is the tree-plan path of the node the reference lands on.
	node string
	// fragment is the `#anchor` carried through. A leaf body is verbatim, so
	// the source heading the fragment named is present in the leaf that hosts
	// it — the anchor survives the move for free, and dropping it would cost
	// precision for nothing.
	fragment string
}

// rebaseMap resolves a source file's destinations to KB landings.
type rebaseMap struct {
	// byFile maps a source file to the destinations written in it that are
	// rewritable, keyed by the destination's exact source bytes. Keying on the
	// written text is what lets the scanner stay a scanner: it finds a
	// position, this map decides whether the bytes there mean anything.
	byFile map[string]map[string]landing
}

// newRebaseMap builds the map over one job's tree plan, cut lists and survey.
func newRebaseMap(plan treeplan.TreePlan, art survey.Artifact, cuts map[string][]survey.Span) (*rebaseMap, error) {
	hosts, err := leafRanges(plan, cuts)
	if err != nil {
		return nil, err
	}
	whole := wholeFileLandings(plan, hosts)
	sections := sectionIndex(art)

	m := &rebaseMap{byFile: map[string]map[string]landing{}}
	for _, f := range art.Files {
		for _, l := range f.Links {
			if l.Kind != survey.LinkInternal {
				continue
			}
			land, ok := resolve(l, hosts, whole, sections)
			if !ok {
				continue
			}
			if m.byFile[f.Path] == nil {
				m.byFile[f.Path] = map[string]landing{}
			}
			m.byFile[f.Path][l.Target] = land
		}
	}
	return m, nil
}

// resolve applies §4.4's three rules to one surveyed link.
func resolve(l survey.Link, hosts map[string][]hostRange, whole map[string]string,
	sections map[string]map[string]int) (landing, bool) {

	if l.Fragment != "" {
		if offsets, ok := sections[l.Path]; ok {
			if start, ok := offsets[anchorSlug(l.Fragment)]; ok {
				if node, ok := hostOf(hosts[l.Path], start); ok {
					return landing{node: node, fragment: l.Fragment}, true
				}
			}
		}
		// Rule 1 with no section to name, or a section promoted to a
		// container: fall through to the file's own landing, keeping the
		// fragment. The reference stays in the right subtree, which is the
		// whole of what a mechanical map can promise about an anchor grammar
		// no artifact states.
	}
	node, ok := whole[l.Path]
	if !ok {
		return landing{}, false
	}
	return landing{node: node, fragment: l.Fragment}, true
}

// rewrite returns the KB destination one source destination becomes, relative
// to the leaf emitting it.
func (m *rebaseMap) rewrite(sourceFile, leafPath, dest string) (string, bool) {
	land, ok := m.byFile[sourceFile][dest]
	if !ok {
		return "", false
	}
	out := RelPath(leafPath, land.node)
	if land.fragment != "" {
		out += "#" + land.fragment
	}
	return out, true
}

// apply rewrites every rewritable destination in one leaf's body and reports
// the corpus-relative destinations it left alone.
//
// The body is copied rather than edited in place: the custody bytes are
// immutable and the slice handed in is a view of them (dissect.SliceLeaves).
func (m *rebaseMap) apply(sourceFile, leafPath string, body []byte) ([]byte, []string) {
	var (
		out   []byte
		left  []string
		prev  int
		seen  = map[string]bool{}
		spans = Destinations(body)
	)
	for _, d := range spans {
		text := d.Text(body)
		next, ok := m.rewrite(sourceFile, leafPath, text)
		if !ok {
			if !IsExternal(text) && !seen[text] {
				seen[text] = true
				left = append(left, text)
			}
			continue
		}
		out = append(out, body[prev:d.Start]...)
		out = append(out, next...)
		prev = d.End
	}
	if out == nil {
		// Nothing was rewritten: hand back the body unchanged rather than a
		// copy of it, so the common case allocates nothing.
		return body, left
	}
	return append(out, body[prev:]...), left
}

// IsExternal reports whether a destination points off the corpus — a scheme, a
// protocol-relative host, or a bare fragment. Those are nobody's problem here:
// §9 check 1 is about files the KB delivers.
func IsExternal(dest string) bool {
	d := strings.TrimSpace(dest)
	if d == "" || strings.HasPrefix(d, "#") || strings.HasPrefix(d, "//") {
		return true
	}
	if i := strings.Index(d, "://"); i > 0 {
		return true
	}
	// `mailto:`, `tel:` and friends: a scheme is letters then a colon, before
	// any slash — which a relative path can never be, since `.` and `/` come
	// first in every one this corpus can produce.
	for i, r := range d {
		switch {
		case r == ':':
			return i > 0
		case r == '/' || r == '.' || r == '#' || r == '?':
			return false
		case !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '+' && r != '-':
			return false
		}
	}
	return false
}

// hostRange is one leaf's byte range in one source file.
type hostRange struct {
	span survey.Span
	node string
}

// leafRanges is every leaf's own byte range, per source file, in tree-plan
// order.
//
// This is [MAD1: F-2]'s two-artifact boundary read in the one direction stage
// 5 needs it: the tree plan says which bytes a GROUP holds, and for a group of
// more than one part the cut list says where the interior boundaries fall. A
// missing cut list for a split group is structural incoherence — the stage
// that composed it is upstream and proven — so it is a refusal and not a unit
// to redo.
func leafRanges(plan treeplan.TreePlan, cuts map[string][]survey.Span) (map[string][]hostRange, error) {
	out := map[string][]hostRange{}
	for _, n := range plan.Nodes {
		if n.Kind != treeplan.KindLeaf {
			continue
		}
		g, ok := plan.SplitGroup(n.SplitGroup)
		if !ok {
			return nil, fmt.Errorf("distill: %s names group %q, which the tree plan does not hold", n.Path, n.SplitGroup)
		}
		span, err := partSpan(g, n.Part, cuts[g.ID])
		if err != nil {
			return nil, err
		}
		out[g.Source.File] = append(out[g.Source.File], hostRange{span: span, node: n.Path})
	}
	return out, nil
}

// partSpan is the byte range one leaf draws (§5, [MAD1: F-2]).
func partSpan(g treeplan.SplitGroup, part int, cut []survey.Span) (survey.Span, error) {
	if g.Parts == 1 {
		return survey.Span{Start: g.Source.Start, End: g.Source.End}, nil
	}
	if len(cut) != g.Parts {
		return survey.Span{}, fmt.Errorf(
			"distill: group %s claims %d parts and its cut list holds %d sections", g.ID, g.Parts, len(cut))
	}
	if part < 1 || part > g.Parts {
		return survey.Span{}, fmt.Errorf("distill: group %s has no part %d", g.ID, part)
	}
	return cut[part-1], nil
}

// hostOf is the leaf holding one source offset.
func hostOf(ranges []hostRange, offset int) (string, bool) {
	for _, r := range ranges {
		if offset >= r.span.Start && offset < r.span.End {
			return r.node, true
		}
	}
	return "", false
}

// wholeFileLandings is rule 2: a file's single leaf, or the lowest index whose
// subtree holds every node drawn from it.
func wholeFileLandings(plan treeplan.TreePlan, hosts map[string][]hostRange) map[string]string {
	parent := make(map[string]string, len(plan.Nodes))
	for _, n := range plan.Nodes {
		parent[n.Path] = n.Parent
	}
	out := make(map[string]string, len(hosts))
	for file, ranges := range hosts {
		if len(ranges) == 1 {
			out[file] = ranges[0].node
			continue
		}
		common := ancestry(parent, ranges[0].node)
		for _, r := range ranges[1:] {
			common = commonPrefix(common, ancestry(parent, r.node))
		}
		if len(common) > 0 {
			out[file] = common[len(common)-1]
		}
	}
	return out
}

// ancestry is a node's chain from the entry-point down to its own parent —
// the containers whose subtrees hold it.
func ancestry(parent map[string]string, node string) []string {
	var up []string
	for p := parent[node]; p != ""; p = parent[p] {
		up = append(up, p)
	}
	// Root first, so a common prefix is a common set of enclosing containers.
	for i, j := 0, len(up)-1; i < j; i, j = i+1, j-1 {
		up[i], up[j] = up[j], up[i]
	}
	return up
}

func commonPrefix(a, b []string) []string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// sectionIndex maps each file's heading anchors to the section's start offset.
func sectionIndex(art survey.Artifact) map[string]map[string]int {
	out := make(map[string]map[string]int, len(art.Files))
	for _, f := range art.Files {
		anchors := map[string]int{}
		var rec func([]survey.Section)
		rec = func(secs []survey.Section) {
			for _, s := range secs {
				if a := anchorSlug(s.Title); a != "" {
					// First writer wins: duplicate headings in one document
					// get `-1`, `-2` suffixes from a site generator, and the
					// bare anchor names the first.
					if _, dup := anchors[a]; !dup {
						anchors[a] = s.Start
					}
				}
				rec(s.Children)
			}
		}
		rec(f.Sections)
		out[f.Path] = anchors
	}
	return out
}

// anchorSlug is the heading→anchor convention a Markdown site generator
// applies: lowercase, drop everything that is not a letter, digit, space or
// hyphen, and hyphenate the spaces.
//
// It is BEST EFFORT and says so: the anchor grammar belongs to whatever
// renders the source site, no artifact states it, and kbase has no way to
// learn it. A fragment this fails to match falls through to rule 2 rather than
// to a dead link, so being wrong costs a hop of precision and never a broken
// reference — which is the trade that makes a guess admissible here at all.
func anchorSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
