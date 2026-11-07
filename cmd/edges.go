package main

import (
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func edgesCommand() *cobra.Command {
	var id string
	var sel query.EdgesSelection
	var cmd *cobra.Command
	cmd = completeListCommand(&cobra.Command{
		Use:   "edges",
		Short: "the edge records, of the named relations or at one node, in a fixed order",
		Long: `edges lists edge records, ordered by relation, source and target. Each row is
the edge's source, target, relation, bearing (as the record holds it; null
where the relation carries none), context (null for none), manually-scored
(null where the relation carries no mark) and replaced-relation (null except
on a demoted row: an edge the build set aside to break a cycle, kept whole in
the replaced edge's direction; it bears on no strength until restore-demoted,
or an add-edge of that edge, puts it back).

` + argmark.Flag("relation") + ` narrows the list to the named relations; it repeats. With no ` + argmark.Flag("id") + `
every edge is listed. ` + argmark.Flag("id") + ` names a node, and ` + argmark.Flag("direction") + ` keeps its edges in
(the node is the target), out (it is the source) or both, the default;
` + argmark.Flag("direction") + ` without ` + argmark.Flag("id") + ` is refused, and so is an ` + argmark.Flag("id") + ` no node carries. A
node's dependents are the targets of its edges with ` + argmark.FlagValue("direction", "out") + ` over
` + proseSeries(kb.ScoringRelations, "and") + `.

A manually scored bearing is rescored only under score's ` + argmark.Flag("rescore-manual") + `;
` + argmark.Call("score", map[string]any{"dry-run": true}) + ` answers whether an edge is dirty.`,
		Args: cobra.NoArgs,
	}, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		sel.ID = nil
		if cmd.Flags().Changed("id") {
			sel.ID = &id
		}
		if !cmd.Flags().Changed("direction") {
			sel.Direction = ""
		}
		rows, usage := query.EdgesPayload(ix, sel)
		return rows, usage, nil
	})
	cmd.Flags().StringArrayVar(&sel.Relations, "relation", nil, "only edges of this relation, one of "+strings.Join(kb.Relations, ", "))
	cmd.Flags().StringVar(&id, "id", "", "only edges at this node")
	cmd.Flags().StringVar(&sel.Direction, "direction", query.DirectionBoth,
		"with "+argmark.Flag("id")+", which of its edges: in (the node is the target), out (it is the source) or both")
	return cmd
}
