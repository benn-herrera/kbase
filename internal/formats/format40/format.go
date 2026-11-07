// Package format40 is the 4.0 metadata format's one definition: what a
// 4.0 KB's bytes are — its stamp, the record files' field names and record
// shapes, the judged hash, the id grammar, the node kinds and relations, the
// encoder and the decoder over the record files' bytes, and the leaf forms a
// reader keys on — the statement marker and where one goes, the label line,
// an equation's title. It imports no kbase package, and restates what it
// shares with an earlier format. It is the live format: it changes, patch
// bumps included, and owns the full version a save writes.
package format40

// Version is the full version a save writes.
const Version = "4.0.1"

// FormatKey is the stamp's one key: in .index/format.yaml, and in the
// entry point's frontmatter of a KB a migration has not stamped there yet.
const FormatKey = "kb-format"

// IndexDir is the index's directory inside kb-root.
const IndexDir = ".index"

// IndexFileExt is the extension of every file under .index/.
const IndexFileExt = ".yaml"

// The KB's two authored record files, kb-root-relative: every node in one,
// every edge in the other.
const (
	NodesFile = IndexDir + "/nodes" + IndexFileExt
	EdgesFile = IndexDir + "/edges" + IndexFileExt
)
