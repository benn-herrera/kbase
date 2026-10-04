package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	var inverse bool
	cmd := queryCommand(&cobra.Command{
		Use:   "deps <id>",
		Short: "a node's dependency edges, or with -i the nodes depending on it",
		Long: `deps lists the edges sourced at <id>, sorted by target: each one's relation,
target and, for a rests-on edge, its applicability. With -i it lists instead
the ids of every node holding an edge, of any relation, to <id>.`,
		Args: cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.DepsPayload(ix, args[0], inverse))
	})
	cmd.Flags().BoolVarP(&inverse, "inverse", "i", false, "list the ids depending on <id> instead")
	rootCmd.AddCommand(cmd)
}
