package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	rootCmd.AddCommand(queryCommand(&cobra.Command{
		Use:   "gated-on <id>",
		Short: "the claims whose strengthen-by items mention a claim",
		Long:  `gated-on lists, sorted, the claims whose strengthen-by items mention <id>.`,
		Args:  cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.GatedOnPayload(ix, args[0]))
	}))
}
