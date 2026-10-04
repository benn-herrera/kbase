package index

import (
	"path"
	"strings"

	"kbase/internal/kb"
)

// LeafReferences is, for every node a leaf hosts or references, the sorted
// leaf paths that do: a claim's citing leaves, an experiment's home and its
// referencing leaves, a support's home.
func LeafReferences(st kb.State) map[string][]string {
	refs := map[string]map[string]bool{}
	add := func(id, leaf string) {
		if refs[id] == nil {
			refs[id] = map[string]bool{}
		}
		refs[id][leaf] = true
	}
	for _, leaf := range st.Leaves {
		for _, c := range leaf.Claims {
			add(c, leaf.Path)
		}
		for _, x := range leaf.ExperimentRefs {
			add(x, leaf.Path)
		}
	}
	for _, x := range st.Experiments {
		add(x.ID, x.CanonicalPath)
	}
	for _, s := range st.Supports {
		add(s.ID, s.CanonicalPath)
	}
	out := map[string][]string{}
	for id, set := range refs {
		out[id] = sortedSet(set)
	}
	return out
}

// RenderLeafReferences is a register entry's footer: each citing leaf as a
// link relative to the register's directory, named by its stem.
func RenderLeafReferences(registerPath string, leaves []string) string {
	if len(leaves) == 0 {
		return kb.LeafReferencesPendingFooter
	}
	dir := path.Dir(registerPath)
	links := make([]string, len(leaves))
	for i, leaf := range leaves {
		rel := leaf
		if dir != "." && dir != "" {
			rel = RelPosix(leaf, dir)
		}
		target := rel
		if !strings.HasPrefix(rel, "../") && !strings.HasPrefix(rel, "/") {
			target = "./" + rel
		}
		links[i] = "[" + stem(path.Base(rel)) + "](" + target + ")"
	}
	return kb.LeafReferencesPrefix + " " + strings.Join(links, ", ") + "."
}

// RelPosix is posixpath.relpath for two slash paths relative to one root.
func RelPosix(target, base string) string {
	segments := func(p string) []string {
		if p = path.Clean(p); p == "." {
			return nil
		}
		return strings.Split(p, "/")
	}
	t, b := segments(target), segments(base)
	common := 0
	for common < len(t) && common < len(b) && t[common] == b[common] {
		common++
	}
	var parts []string
	for range b[common:] {
		parts = append(parts, "..")
	}
	parts = append(parts, t[common:]...)
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

// stem is a file name without its last suffix, as PurePath.stem has it.
func stem(name string) string {
	if i := strings.LastIndex(name, "."); i > 0 && i < len(name)-1 {
		return name[:i]
	}
	return name
}
