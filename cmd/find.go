package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	rootCmd.AddCommand(queryCommand(&cobra.Command{
		Use:   "find <query>",
		Short: "the claims whose title or anchor contains a string",
		Long: `find lists, by id, the claims whose title or canonical anchor contains
<query>, case-insensitively: the way from a name or a number to a claim id. An
empty <query> matches every claim.`,
		Args: cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.FindPayload(ix, args[0]))
	}))
}
