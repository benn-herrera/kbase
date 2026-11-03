package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func moveNodeCommand() *cobra.Command {
	return writeOpCommand(write.MoveNode,
		"move a node's statement marker to a named line, re-pointing its citations",
		`move-node moves the statement marker of the node "id" names to the end of
the line "document", "line" and "snippet" name, taking it from wherever it
stood. In the same write it re-points every citation of the node, found by the
id in its link title, to the node's new document and section anchor, changing
only the link target, never the citing words. A move to the line the marker
already ends is unchanged.

The snippet need not be unique. A line that does not contain the snippet is
refused, naming the nearby lines that do — the document has likely changed
since you read it, so re-read it. A line past the document's end is refused,
and so is a fenced block's opening or closing line.

Refused too: a work or an axiom, which has no statement marker; an "id" no
node carries, no node; and a node with no marker to move.

The result gives each entry's "id".`)
}
