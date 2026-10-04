package build

import (
	"errors"
	"os"
	"path/filepath"
	"slices"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// seedSpine initialises the claim-graph spine over the document tree:
// .index/, a refresh, and verify green but for frontmatter presence, which
// the document graph writes none of and no stage has stamped yet. What green
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
	written, err := index.Refresh(w.kbRoot, w.opts.Logger)
	slices.Sort(written)
	rec := result.Record{{Key: "index-dir", Value: seeded}, {Key: "refreshed", Value: slices.Compact(written)}}
	var refused index.Refusal
	if errors.As(err, &refused) {
		return rec, refusal(refused.Items)
	}
	if err != nil {
		return rec, err
	}
	found, err := index.Verify(w.kbRoot)
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
