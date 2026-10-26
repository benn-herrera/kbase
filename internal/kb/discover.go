package kb

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// comparePathParts orders slash paths the way Python orders Paths: segment
// by segment.
func comparePathParts(a, b string) int {
	return slices.Compare(strings.Split(a, "/"), strings.Split(b, "/"))
}

// SymlinkError is a KB document that is a symbolic link: a load refuses it
// rather than reading through it.
type SymlinkError struct{ Path string }

func (e *SymlinkError) Error() string {
	return fmt.Sprintf("%s is a symbolic link, and a KB document is read only where it stands", e.Path)
}

// Files is every regular file of the KB whose name match accepts and none of
// whose directories below kb-root is excluded, as kb-root-relative slash
// paths in Python's Path order. A file match accepts that is a symbolic link
// is refused rather than read through.
func Files(src *Source, match func(name string) bool) ([]string, error) {
	root := src.Root()
	var out []string
	err := src.walk(root, func(name string) bool { return ExcludeDirs[name] }, func(p string) error {
		if !match(filepath.Base(p)) || !src.IsFile(p) {
			return nil
		}
		if isSymlink(p) {
			return &SymlinkError{Path: p}
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	slices.SortFunc(out, comparePathParts)
	return out, err
}

// Documents is every document of the KB — every .md file outside the
// excluded directories and names — as kb-root-relative slash paths.
func Documents(src *Source) ([]string, error) {
	return Files(src, func(name string) bool {
		return strings.HasSuffix(name, ".md") && !ExcludeNames[name]
	})
}
