// Package sheet renders the claim-graph sheet from the derived index. Its
// body is a placeholder: "NYI" over a digest of the index it was rendered
// from.
package sheet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// placeholderMark is the text a placeholder sheet, and no drawn one, carries.
const placeholderMark = ">NYI</text>"

// Render is the sheet for the index in indexDir: the first 12 hex digits of
// the SHA-256 of its *.jsonl files concatenated in sorted path order.
func Render(indexDir string) ([]byte, error) {
	entries, err := os.ReadDir(indexDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	h := sha256.New()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(indexDir, name))
		if err != nil {
			return nil, err
		}
		h.Write(data)
	}
	digest := hex.EncodeToString(h.Sum(nil))[:12]
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" width="240" height="80" viewBox="0 0 240 80">
<text x="120" y="36" text-anchor="middle" font-family="sans-serif" font-size="24">NYI</text>
<text x="120" y="60" text-anchor="middle" font-family="monospace" font-size="12">index sha256:%s</text>
</svg>
`, digest), nil
}

// IsOwn is whether a sheet is this package's placeholder rather than a
// drawn one.
func IsOwn(svg []byte) bool { return bytes.Contains(svg, []byte(placeholderMark)) }
