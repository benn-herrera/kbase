package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func restoreDemotedCommand() *cobra.Command {
	return writeOpCommand(write.RestoreDemoted,
		"restore the edge a demoted record replaced",
		`restore-demoted puts back exactly the edge each demoted record from source to
target replaced — the build demotes a scoring edge to break a cycle — with its
relation, context, bearing and manually-scored mark, and deletes the record.
Name the records as their edges row names them: "source", "target" and a
"relation" of demoted. Every demoted record from source to target is restored,
one per relation the build demoted there.

Refused, writing nothing: an entry no demoted record from source to target
answers, no edge, so a restore re-issued is refused; and a restore that would
close a cycle (check: dependency-cycle), the detail naming the cycle: remove
another edge of it first. To leave the edge out of the graph, remove-edge with
a "relation" of demoted removes the record; to put the edge back with new
values, add-edge writes it and deletes the record.

The result gives each entry's "source", "target" and "relation", and
"restored", the relations put back.

`+edgeRowCopyHelp)
}
