package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/kb"
	"kbase/internal/write"
)

func removeEdgeCommand() *cobra.Command {
	return writeOpCommand(write.RemoveEdge,
		"remove one edge record",
		`remove-edge removes the edge record that "source", "target" and "relation"
name. With a "relation" of demoted it removes every demoted record from source
to target, leaving the edge out of the graph, with no record of it. Every
other record stays byte for byte as it was.

Refused, writing nothing: a "source" or "target" no node carries; a
`+proseSeries(kb.ScoringRelations, "or")+` relation that does not join the two
nodes' kinds; and an edge no record holds, no edge. Nothing records what was
removed, so a removal re-issued is refused. edges lists the edge records that
stand, demoted ones included.

The result gives each entry's "source", "target" and "relation".`)
}
