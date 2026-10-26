package main

import "github.com/spf13/cobra"

func markClaimInLeafCommand() *cobra.Command {
	return writeOpCommand("mark-claim-in-leaf",
		"place or move a claim's, analysis's or experiment's marker at a located line of a document",
		`mark-claim-in-leaf places a claim's, analysis's or experiment's statement
marker at the end of the one body line its locator names, taking it from
wherever it stood. The node must have a record and the document must be a leaf
of the KB; a locator that names no line, several, or a fenced block's opening
or closing line is refused. A refresh follows unless --no-refresh.`)
}
