package claimgraph

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
)

func refusePoint(point int, format string, args ...any) *stop {
	return stopf(fmt.Sprintf("point-%d", point), format, args...)
}

// structuralChecks are ordered by what each check's subject depends on: links
// first, because every relation after them is derived from links and a dead
// link is a phantom child.
var structuralChecks = []func(*Tree) error{checkMarkdownLinks, checkAnchors, checkEntryPoint, checkUplinks, checkPathShape, checkSpine}

// conformanceGate is kb_claimgraph stage A for the declared pass: every
// structural guarantee, then point 14's cleanliness, which is the double-run
// guard — a tree carrying any metadata artifact is not one the document graph
// just wrote, and minting over it would mint a second set of ids.
func conformanceGate(t *Tree) error {
	for _, check := range append(slices.Clone(structuralChecks), checkCleanliness) {
		if err := check(t); err != nil {
			return err
		}
	}
	return nil
}

// passTwoGate is the later stages' entry condition: the structural checks.
func passTwoGate(t *Tree) error {
	for _, check := range structuralChecks {
		if err := check(t); err != nil {
			return err
		}
	}
	return nil
}

// checkEntryPoint is point 1: the root lists every volume, and nothing else.
func checkEntryPoint(t *Tree) error {
	root := kb.EntryPointFile
	if _, ok := t.Documents[root]; !ok {
		return refusePoint(1, "no %s at the KB root", root)
	}
	volumes := map[string]bool{}
	for _, p := range t.Paths {
		if strings.Count(p, "/") == 1 && strings.HasSuffix(p, "/"+kb.IndexFile) {
			volumes[p] = true
		}
	}
	listed := map[string]bool{}
	for _, c := range t.Children[root] {
		listed[c] = true
	}
	var unlisted, extra []string
	for v := range volumes {
		if !listed[v] {
			unlisted = append(unlisted, v)
		}
	}
	for l := range listed {
		if !volumes[l] {
			extra = append(extra, l)
		}
	}
	if len(unlisted) > 0 || len(extra) > 0 {
		slices.Sort(unlisted)
		slices.Sort(extra)
		return refusePoint(1, "%s's link set is not the depth-1 volume-index set: unlisted %q, listed but not a volume index %q", root, unlisted, extra)
	}
	return nil
}

// checkUplinks is points 3 and 4: the first line after the frontmatter is an
// up-link, and the parent names the document back.
func checkUplinks(t *Tree) error {
	for _, p := range t.Paths {
		if p == kb.EntryPointFile {
			continue
		}
		parent, ok := t.Parents[p]
		if !ok {
			return refusePoint(3, "%s does not open, below its frontmatter, with an up-link carrying %q", p, kb.UplinkMarker)
		}
		if _, ok := t.Documents[parent]; !ok {
			return refusePoint(3, "%s's up-link names %s, which is not a document of this tree", p, parent)
		}
		if !slices.Contains(t.Children[parent], p) {
			return refusePoint(4, "%s up-links to %s, whose child list does not name it", p, parent)
		}
	}
	return nil
}

// checkPathShape is point 2: descendants decide the filename, both ways.
func checkPathShape(t *Tree) error {
	for _, p := range t.Paths {
		if p == kb.EntryPointFile {
			continue
		}
		atIndex := path.Base(p) == kb.IndexFile
		children := t.Children[p]
		if len(children) > 0 && !atIndex {
			return refusePoint(2, "%s lists %d children but does not sit at <dir>/%s", p, len(children), kb.IndexFile)
		}
		if atIndex && len(children) == 0 {
			return refusePoint(2, "%s sits at an index path but lists no children", p)
		}
	}
	return nil
}

// checkSpine is point 5: the down-link spine is total, acyclic, and inverts
// the up-link relation exactly.
func checkSpine(t *Tree) error {
	root := kb.EntryPointFile
	seen := map[string]bool{}
	type item struct {
		p     string
		trail []string
	}
	stack := []item{{root, []string{root}}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[it.p] {
			return refusePoint(5, "the down-link spine is not acyclic: %s", strings.Join(it.trail, " -> "))
		}
		seen[it.p] = true
		for _, c := range t.Children[it.p] {
			stack = append(stack, item{c, append(slices.Clone(it.trail), c)})
		}
	}
	var unreached []string
	for _, p := range t.Paths {
		if !seen[p] {
			unreached = append(unreached, p)
		}
	}
	if len(unreached) > 0 {
		return refusePoint(5, "%d document(s) unreachable from %s by down-links: %q", len(unreached), root, unreached[:min(5, len(unreached))])
	}
	for _, p := range t.Paths {
		for _, c := range t.Children[p] {
			if t.Parents[c] != p {
				return refusePoint(5, "%s lists %s as a child, but %s's up-link does not name %s", p, c, c, p)
			}
		}
	}
	return nil
}

// checkMarkdownLinks is point 7 for the link form the dead-link gate sees.
func checkMarkdownLinks(t *Tree) error {
	dead, err := kb.DeadLinks(builtTree(t.Root))
	if err != nil {
		return err
	}
	if len(dead) > 0 {
		d := dead[0]
		return refusePoint(7, "%d dead markdown link(s), first at %s:%d %s %q", len(dead), d.File, d.Line, d.Kind, d.Target)
	}
	return nil
}

// checkAnchors is point 7 for the rewritten cross-references the dead-link
// gate cannot see. A bare fragment is point 7's legitimate unresolved
// outcome. Read over marker-stripped text: a marker appended to a wrapped
// anchor's first line would otherwise leave it unmatched and unchecked.
func checkAnchors(t *Tree) error {
	for _, p := range t.Paths {
		for _, m := range anchorRE.FindAllStringSubmatch(unquote(stripMarkers(t.Documents[p].Text)), -1) {
			target, _, _ := strings.Cut(m[1], "#")
			if target == "" {
				continue
			}
			if _, ok := t.Documents[kb.ResolveLink(p, target)]; !ok {
				return refusePoint(7, "%s: cross-reference anchor %q lands on no document", p, m[1])
			}
		}
	}
	return nil
}

// checkCleanliness is point 14 over the node records and every Markdown file
// the tree holds.
func checkCleanliness(t *Tree) error {
	const rebuild = "This is not a tree the front end just wrote: this stage mints ids, so a second run over its own output would mint a second set and double the graph. Rebuild the tree from the corpus and run once"
	src := builtTree(t.Root)
	g, err := kb.LoadGraph(src)
	if err != nil {
		return err
	}
	if len(index.GraphKinds(g)) > 0 {
		return refusePoint(14, "%s already holds node records. %s", kb.GraphNodesFile, rebuild)
	}
	var files []string
	err = filepath.WalkDir(t.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != t.Root && kb.ExcludeDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	slices.Sort(files)
	for _, f := range files {
		text, err := src.ReadText(f)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(t.Root, f)
		if err != nil {
			return err
		}
		fm, err := kb.ParseFrontmatter(text)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.ToSlash(rel), err)
		}
		for key := range fm {
			return refusePoint(14, "%s already carries frontmatter (%s). %s", filepath.ToSlash(rel), key, rebuild)
		}
		if strings.Contains(text, kb.StatementMarkerOpener) {
			return refusePoint(14, "%s already carries %q. %s", filepath.ToSlash(rel), kb.StatementMarkerOpener, rebuild)
		}
	}
	return nil
}
