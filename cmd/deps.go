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
		Short: "a node's incident edges, or with -i the nodes depending on it",
		Long: `deps lists every edge incident at <id> — each scoring edge in either
direction and <id>'s own outgoing references and demoted edges — as its
relation, its direction (in where <id> is the supported end, out where it is
the supporter), the other end, its bearing (null where the pairing carries
none) and its context (null where the edge records none), in relation order and
then by the other end. With -i it lists instead the ids of <id>'s dependents,
the nodes <id>'s strength enters: the targets of its outgoing scoring edges. A
references or demoted edge makes neither end a dependent.`,
		Args: cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.DepsPayload(ix, args[0], inverse))
	})
	cmd.Flags().BoolVarP(&inverse, "inverse", "i", false, "list the ids depending on <id> instead")
	rootCmd.AddCommand(cmd)
}
