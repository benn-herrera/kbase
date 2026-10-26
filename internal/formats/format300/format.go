// Package format300 is the 3.0.0 metadata format's one definition: what a
// 3.0.0 KB's bytes are — its stamp, the record files' field names and record
// shapes, the id grammar, the node kinds and relations, the encoder and the
// decoder over the record files' bytes, and the leaf forms a reader keys on —
// the statement marker and where one goes, the label line, an equation's
// title. It imports no kbase package. It may
// change until a kbase release ships 3.0.0, and is frozen after.
package format300

const Version300 = "3.0.0"

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
