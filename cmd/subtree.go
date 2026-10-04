package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	rootCmd.AddCommand(queryCommand(&cobra.Command{
		Use:   "subtree <path>",
		Short: "the claim ids under an index node",
		Long: `subtree lists the claims under the index node at <path>, kb-root-relative:
the node's file or its directory; "" or "." for the entry point.`,
		Args: cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.SubtreePayload(ix, args[0]))
	}))
}
