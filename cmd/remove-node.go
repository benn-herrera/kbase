package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/write"
)

func removeNodeCommand() *cobra.Command {
	return writeOpCommand(write.RemoveNode,
		"remove a node and every record naming it, its edges included",
		`remove-node removes the node "id" names and every record naming it: its
node record, its statement marker or an axiom's heading in axioms.md, every
demoted record naming it, and every edge naming it at either end. Every edge
naming the node goes with it, whatever its bearing: run the batch with
`+argmark.Flag("dry-run")+` first to see which. No undo is built; git restores
what a removal took.

Refused, writing nothing: an "id" no node carries, no node, so a removal
re-issued is refused; find lists the nodes that stand. Refused too, at the
point the entry applies, while a mention of the node would outlive it: a
citation of it, or its id
`+bareIDHelp("")+`, each named by path and
line, whose prose you edit first; for a work, a citation span whose
data-cites carries its label where no reference-list entry has that key
(named by path and line; remedy: edit that prose), since without the work
record the export prints the span as plain text; or a title, a rationale or
an edge's context of a record that survives, named by record and field, which
an edit-<kind> or edit-edge entry earlier in the same batch rewrites.

The result gives each entry's "id" and "removed-edges": every edge record
removed, as an edges row, whose keys edges' help names. To re-add an edge onto
a replacement node, copy its row into add-edge with the replacement's id at
that end.

`+edgeRowCopyHelp)
}

// edgeRowCopyHelp is the paragraph of every help telling a caller to copy an
// edge row from a result into another op's entry.
var edgeRowCopyHelp = `Copying an edge row into add-edge or edit-edge: its "manually-scored",
"replaced-relation" (except that from a ` + kb.RelationDemoted + ` row, "replaced-relation" goes
in as "relation"), trace's "depth", "strength" and "contribution", and any key
whose value is null are read-only facts; drop them before copying the rest.
Copy "bearing" only where the row is manually scored or the relation is
` + kb.RelationFounds + `: a bearing given marks the edge manually scored, so copying a
model-judged one would pin it. Into remove-edge and restore-demoted, copy only
"source", "target" and "relation".`
