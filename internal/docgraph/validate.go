package docgraph

import (
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/kb"
)

// validateBuild is kb_tools' post-build validation over the tree as it lies
// on disk: every document leaf discovery sees is reachable down from the entry
// point and up-linked to its own parent index.
func validateBuild(kbRoot string) ([]Finding, error) {
	documents, err := treeDocuments(kbRoot)
	if err != nil {
		return nil, err
	}
	return reachability(documents), nil
}

// treeDocuments is every document under kbRoot leaf discovery reads, by its
// kb-root-relative path.
func treeDocuments(kbRoot string) (map[string]string, error) {
	files, err := kb.MarkdownFiles(kbRoot)
	if err != nil {
		return nil, err
	}
	documents := map[string]string{}
	for _, f := range files {
		if kb.ExcludeNames[filepath.Base(f)] {
			continue
		}
		rel, err := filepath.Rel(kbRoot, f)
		if err != nil {
			return nil, err
		}
		text, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		documents[filepath.ToSlash(rel)] = string(text)
	}
	return documents, nil
}

// parentIndex is the document path hangs from, "" for the entry point.
func parentIndex(p string) string {
	if p == kb.EntryPointFile {
		return ""
	}
	dir := path.Dir(p)
	if path.Base(p) == kb.IndexFile {
		dir = path.Dir(dir)
	}
	if dir == "." {
		return kb.EntryPointFile
	}
	return dir + "/" + kb.IndexFile
}

// reachability walks up from every document by its up-links, which must land
// on its own parent index, and down from the entry point by every other link.
func reachability(documents map[string]string) []Finding {
	links := map[string][]kb.DeclaredLink{}
	for source, text := range documents {
		links[source] = kb.DeclaredLinks(text)
	}
	var findings []Finding
	fail := func(detail string) { findings = append(findings, Finding{StatusFail, nameReachable, detail}) }
	for _, source := range slices.Sorted(maps.Keys(documents)) {
		if source == kb.EntryPointFile {
			continue
		}
		var ups []kb.DeclaredLink
		for _, l := range links[source] {
			if l.Up {
				ups = append(ups, l)
			}
		}
		if len(ups) == 0 {
			fail(fmt.Sprintf("%s no up-link — a non-root document carrying no %q link, so nothing walks up from it", source, kb.UplinkMarker))
			continue
		}
		expected := parentIndex(source)
		for _, l := range ups {
			target := kb.ResolveDeclared(source, l.Target)
			if _, ok := documents[target]; !ok {
				fail(fmt.Sprintf("%s up-link dangles — its %q link to %q resolves to no authored document", source, kb.UplinkMarker, l.Target))
			} else if target != expected {
				fail(fmt.Sprintf("%s up-link misparented — its %q link to %q resolves to %s, but its parent is %s", source, kb.UplinkMarker, l.Target, target, expected))
			}
		}
	}
	if _, ok := documents[kb.EntryPointFile]; !ok {
		fail(kb.EntryPointFile + " absent — the tree has no root to walk down from")
	} else {
		reached := map[string]bool{kb.EntryPointFile: true}
		queue := []string{kb.EntryPointFile}
		for len(queue) > 0 {
			source := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			for _, l := range links[source] {
				target := kb.ResolveDeclared(source, l.Target)
				if _, ok := documents[target]; ok && !l.Up && !reached[target] {
					reached[target] = true
					queue = append(queue, target)
				}
			}
		}
		for _, p := range slices.Sorted(maps.Keys(documents)) {
			if !reached[p] {
				fail(fmt.Sprintf("%s unreachable — no down-link chain from %s reaches it", p, kb.EntryPointFile))
			}
		}
	}
	if len(findings) > 0 {
		slices.SortStableFunc(findings, func(a, b Finding) int { return strings.Compare(a.Detail, b.Detail) })
		return findings
	}
	return []Finding{{StatusPass, nameReachable, fmt.Sprintf("%d documents, every one up-linked to its parent index and reachable from %s", len(documents), kb.EntryPointFile)}}
}
