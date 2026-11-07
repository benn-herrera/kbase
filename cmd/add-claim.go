package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

// hostedAddHelp is the help add-claim, add-analysis and add-experiment share:
// the identity an add checks, what a given rigor's mark does, and where the
// marker goes. prose is the op's own prose keys, each held to one line.
func hostedAddHelp(prose ...string) string {
	return `An add of a "title" a node of its kind already carries is unchanged where
it is identical — the same values, and that node's marker already where the
add places it — and reports that node's id. Any other is refused (node exists:
<id>; remedy: drop this entry and use that id wherever you would have named
the new node, or give a title no node of its kind carries); nothing is
adopted.
` + oneLineHelp(prose...) + `

` + manuallyScoredHelp("rigor", false, "") + `

The statement marker goes at the end of the line "document", "line" and
"snippet" name, directly after its last character, so a heading's anchor is
unchanged. The snippet need not be unique. A line that does not contain the
snippet is refused, naming the nearby lines that do — the document has likely
changed since you read it, so re-read it. A line past the document's end is
refused, and so is a fenced block's opening or closing line.

The result gives each entry's minted "id". The node carries no edges: a later
batch naming the id writes them.`
}

func addClaimCommand() *cobra.Command {
	return writeOpCommand(write.AddClaim,
		"mint a claim, write its record and place its statement marker on a named line",
		`add-claim mints a claim id, writes the claim's record and places its
statement marker. The claim's statement is the paragraph the marked line is
in; on a heading line, it is that heading's section.

`+hostedAddHelp("title", "rationale"))
}
