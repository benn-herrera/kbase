package build

import (
	"os"
	"path/filepath"
	"slices"

	"kbase/internal/atomicfile"
	"kbase/internal/buildstore"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/result"
)

// seedSpine initialises the claim-graph spine over the document tree:
// .index/, a format stamp where the tree carries none, each record file it
// lacks written holding no record, a refresh — which migrates a superseded
// tree, its save stamping the KB — and verify green. The stamp is seeded
// before the Open because the document-graph stage rewrites the entry point
// without a frontmatter stamp; where one stands, the loader reads it and the
// migration's save writes the file. What green means is that the spine is
// installed over the tree that is there. A record file that stands is the
// refresh's to re-render.
func seedSpine(w *walk) (result.Record, error) {
	indexDir := filepath.Join(w.kbRoot, kb.IndexDir)
	seeded := "present"
	if info, err := os.Stat(indexDir); err != nil || !info.IsDir() {
		if err := os.MkdirAll(indexDir, 0o755); err != nil {
			return nil, err
		}
		seeded = "created"
	}
	if !kbload.HasFormatStamp(w.kbRoot) {
		p := filepath.Join(w.kbRoot, kb.FormatFile)
		if err := atomicfile.Write(p, kb.EncodeFormatStamp(kb.FormatVersion), nil); err != nil {
			return nil, err
		}
	}
	src, err := kbload.Open(w.kbRoot)
	if err != nil {
		return nil, err
	}
	for _, file := range []struct {
		rel  string
		data []byte
	}{{kb.GraphNodesFile, kb.EncodeNodes(kb.Graph{})}, {kb.GraphEdgesFile, kb.EncodeEdges(kb.Graph{})}} {
		p := src.KBPath(file.rel)
		if src.IsFile(p) {
			continue
		}
		if err := atomicfile.Write(p, file.data, nil); err != nil {
			return nil, err
		}
		src.Put(p, file.data)
	}
	written, err := w.refresh(src)
	slices.Sort(written)
	rec := result.Record{{Key: "index-dir", Value: seeded}, {Key: "refreshed", Value: slices.Compact(written)}}
	if err != nil {
		return rec, err
	}
	found, err := index.BuildVerify(src)
	if err != nil {
		return rec, err
	}
	items := index.Items(found)
	rec = append(rec, result.Field{Key: "verify", Value: items})
	if len(items) > 0 {
		return rec, buildstore.Failure(items)
	}
	return rec, nil
}
