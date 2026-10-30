package main

import "github.com/spf13/cobra"

func setBearingCommand() *cobra.Command {
	return writeOpCommand("set-bearing",
		"rewrite one scoring edge's bearing",
		`set-bearing rewrites the bearing on one scoring edge's record — a supports or
founds premise, a lift or a refutation — and, on any but a founds edge, marks
it manually-scored. A value is a number in [0, 1] or *pending*. An edge with no
record, an edge the build demoted, a relation that is not a scoring relation, a
pairing that carries no bearing and a value outside the domain are each
refused: the op re-scores an edge, it does not create one; add-rests-on and
add-pairs add one. A refresh follows unless --no-refresh.`)
}
