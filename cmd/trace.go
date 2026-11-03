package main

import (
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/mcp"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func traceCommand() *cobra.Command {
	var sort string
	var limit int
	var sel query.TraceSelection
	var cmd *cobra.Command
	cmd = completeListCommand(&cobra.Command{
		Use:   "trace <id>",
		Short: "every scoring edge reached from a node, walking in or out, with each edge's contribution",
		Long: `trace follows the scoring relations (` + proseSeries(kb.ScoringRelations, "and") + `)
transitively from the node ` + argmark.Positional("id") + `: ` + argmark.FlagValue("direction", "in") + ` gives every edge bearing on
the node's strength, ` + argmark.FlagValue("direction", "out") + ` every edge its strength reaches. It never walks
` + proseSeries([]string{kb.RelationReferences, kb.RelationDemoted}, "or") + ` records, and it is always the full walk; edges gives
one hop.

Each row is an edges row, then depth (its distance from the node, 1 for the
node's own edges), strength (the source's: a claim's or an analysis's derived
strength; a work's, an experiment's or an axiom's rigor) and contribution (the
edge's input to its target's strength as the derivation computes it: bearing
times the source's strength, or on refutes the cap 1 minus that; null where
the edge takes no part, as a planned experiment's; otherwise *pending* where
the bearing or the source's strength is pending).

` + argmark.FlagValue("sort", "depth") + `, the default, reads outward from the node. ` + argmark.FlagValue("sort", "contribution") + `
orders every relation together by contribution — on refutes the cap, so the
strongest refutation sorts lowest and the weakest highest: ascending puts the
inputs that hold their targets lowest first, descending the ones that leave
them highest. A *pending* contribution sorts below every number and a null one
last in either ` + argmark.Flag("order") + `, so take those that leave their targets highest with
` + argmark.FlagValue("order", "descending") + `, not a negative ` + argmark.Flag("limit") + `.
` + argmark.Flag("order") + ` reverses the key only: ties break by ascending depth, then edges' order.
An id no node carries is refused.`,
		Args: cobra.ExactArgs(1),
	}, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		sel.ID, sel.Sort, sel.Limit = args[0], nil, nil
		if cmd.Flags().Changed("sort") {
			sel.Sort = &sort
		}
		if cmd.Flags().Changed("limit") {
			sel.Limit = &limit
		}
		rows, usage := query.TracePayload(ix, sel)
		return rows, usage, nil
	})
	cmd.Flags().StringVar(&sel.Direction, "direction", "",
		"in (what bears on the node's strength) or out (what its strength reaches)")
	mcp.RequireFlag(cmd.Flags().Lookup("direction"))
	cmd.Flags().StringVar(&sort, "sort", query.SortDepth, "the key to sort by: "+strings.Join(query.TraceSorts, ", "))
	cmd.Flags().StringVar(&sel.Order, "order", query.OrderAscending, sortOrderUsage)
	cmd.Flags().IntVar(&limit, "limit", 0, sortLimitUsage)
	return mcp.DescribePositional(cmd, "id", "the id of the node the walk starts from")
}
