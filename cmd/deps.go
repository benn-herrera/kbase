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
target, for a rests-on edge its applicability, and its context (null where
the edge records none). With -i it lists instead
the ids of <id>'s dependents, the nodes whose solidity <id>'s enters: the
sources of depends and rests-on edges to <id> and, for an experiment or
support, the targets of its strengthens and supports edges. A references or
demoted edge makes neither end a dependent.`,
		Args: cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.DepsPayload(ix, args[0], inverse))
	})
	cmd.Flags().BoolVarP(&inverse, "inverse", "i", false, "list the ids depending on <id> instead")
	rootCmd.AddCommand(cmd)
}
