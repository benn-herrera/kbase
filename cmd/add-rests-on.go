package main

import "github.com/spf13/cobra"

func addRestsOnCommand() *cobra.Command {
	return writeOpCommand("add-rests-on",
		"add supports or founds edges into a node, or references edges from it",
		`add-rests-on adds edge records on one claim or analysis: each rests-on premise
a supports edge into it from a clm- or exp- id, or a founds edge from a wrk- or
axm- id, and each references entry a references edge out of it, which only a
claim takes. A rests-on premise defaults to a pending bearing when the values
name none, an edge the records already hold keeps its record as it stands —
set-bearing re-scores it — and an edge that would close a cycle is refused. A
refresh follows unless --no-refresh.`)
}
