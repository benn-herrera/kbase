package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	rootCmd.AddCommand(queryCommand(&cobra.Command{
		Use:   "cited-by <id>",
		Short: "the leaves citing a claim",
		Long:  `cited-by lists the citations of <id>, sorted by leaf path.`,
		Args:  cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.CitedByPayload(ix, args[0]))
	}))
}
