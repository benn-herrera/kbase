package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func editEdgeCommand() *cobra.Command {
	return writeOpCommand(write.EditEdge,
		"change a standing edge's bearing or context",
		`edit-edge changes the bearing, the context, or both, of the standing edge that
"source", "target" and "relation" name; an entry gives at least one. A field
given replaces its value, and a field left out stands. An entry whose values
and mark already stand is unchanged.

`+manuallyScoredHelp("bearing", true, "")+`
`+oneLineHelp("context")+`

An edge no record holds is refused, no edge; edges lists those that stand. A
"relation" of demoted is refused: restore-demoted puts the edge back, and
remove-edge removes its record.

The result gives each entry's "source", "target" and "relation".`)
}
