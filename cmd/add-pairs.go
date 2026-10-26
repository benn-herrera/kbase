package main

import "github.com/spf13/cobra"

func addPairsCommand() *cobra.Command {
	return writeOpCommand("add-pairs",
		"add scoring edges from an existing claim, analysis or experiment",
		`add-pairs adds scoring edges from one existing claim, analysis or experiment
to the claims and analyses its pair lists name; a list the node's kind does not
author is refused. A bearing defaults to pending where the values name none, an
edge the records already hold keeps its record as it stands, bearing included —
set-bearing re-scores it — and an edge that would close a cycle is refused. A
refresh follows unless --no-refresh.`)
}
