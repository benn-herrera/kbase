package build

import (
	"os"
	"path/filepath"
	"slices"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/result"
)

// seedSpine initialises the claim-graph spine over the document tree:
// .index/, a refresh — whose save stamps the entry point with the format
// version — and verify green but for frontmatter presence, which the
// document graph writes none of and no stage has written yet. What green
// means here is that the spine is installed over the tree that is there.
func seedSpine(w *walk) (result.Record, error) {
	indexDir := filepath.Join(w.kbRoot, kb.IndexDir)
	seeded := "present"
	if info, err := os.Stat(indexDir); err != nil || !info.IsDir() {
		if err := os.MkdirAll(indexDir, 0o755); err != nil {
			return nil, err
		}
		seeded = "created"
	}
	src, err := kbload.Open(w.kbRoot)
	if err != nil {
		return nil, err
	}
	written, err := index.Refresh(src, w.opts.Logger)
	slices.Sort(written)
	rec := result.Record{{Key: "index-dir", Value: seeded}, {Key: "refreshed", Value: slices.Compact(written)}}
	if err != nil {
		return rec, err
	}
	found, err := index.BuildVerify(src)
	if err != nil {
		return rec, err
	}
	var findings []index.Finding
	for _, f := range found {
		if f.Check != index.CheckFrontmatterPresence {
			findings = append(findings, f)
		}
	}
	items := index.Items(findings)
	rec = append(rec, result.Field{Key: "verify", Value: items})
	if len(items) > 0 {
		return rec, failure(items)
	}
	return rec, nil
}
