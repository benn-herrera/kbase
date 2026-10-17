package export

import (
	"path"
	"strings"

	"kbase/internal/kb"
)

// Strip removes a document's navigation forms and keeps every other line in
// the line unit it was read in: the frontmatter block, the up-link line, and —
// for an index — each child-list line, all as kb's child-list reading finds
// them, so Strip drops exactly the lines the walk descends. srcPath is the
// document's kb-root-relative path, which says whether it is an index. Every
// other drop — markers, rewritten links, mathematics — belongs to the content
// transform, not here.
//
// text is loader-normalized: kb.Source.ReadText already translated \r\n and \r
// to \n, and a direct caller owes the same. The line unit is kb.SplitLines'
// break set, so a body carrying U+2028 or U+0085 spells the line break it
// holds; the result is those lines rejoined with \n, so a final terminator is
// not carried.
//
// The entry point is an index for this purpose: its volume-list lines are
// child-list lines and go, while its front-matter blurb and its authored prose
// sections were never part of the block and stay.
func Strip(srcPath, text string) string {
	lines, uplink, children := kb.ChildListLines(srcPath, text)
	index := path.Base(srcPath) == kb.IndexFile || srcPath == kb.EntryPointFile
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == uplink || index && children[i] != nil {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
