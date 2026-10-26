package format200

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"unicode/utf8"

	"kbase/internal/formats/common"
)

func decodeText(p string, data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("%s: not UTF-8", p)
	}
	return TranslateNewlines(string(data)), nil
}

// walkFiles visits every path src holds below dir, less those under a
// directory below dir that skip refuses, in the order filepath.WalkDir visits a
// tree of those files.
func walkFiles(src common.Files, dir string, skip func(name string) bool, visit func(p string) error) error {
	var paths []string
	for p := range src {
		rel, ok := strings.CutPrefix(p, dir+"/")
		if !ok {
			continue
		}
		parts := strings.Split(rel, "/")
		if !slices.ContainsFunc(parts[:len(parts)-1], skip) {
			paths = append(paths, p)
		}
	}
	slices.SortFunc(paths, comparePathParts)
	for _, p := range paths {
		if err := visit(p); err != nil {
			return err
		}
	}
	return nil
}

// hasFile is whether src holds p; every path it holds is a regular file.
func hasFile(src common.Files, p string) bool {
	_, ok := src[p]
	return ok
}

// readText is the text of the bytes src holds at p, decoded as a read of the
// file from disk decodes it; a path src does not hold is a missing file.
func readText(src common.Files, p string) (string, error) {
	data, ok := src[p]
	if !ok {
		return "", &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}
	return decodeText(p, data)
}
