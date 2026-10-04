package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	rootCmd.AddCommand(queryCommand(&cobra.Command{
		Use:   "stats",
		Short: "the index census, the build-band distribution and the rework dashboard",
		Long: `stats counts the nodes of each kind and the records of each index file,
the claims in each build band, and lists the ten highest-leverage weak points
and the ten lowest-solidity claims.`,
		Args: cobra.NoArgs,
	}, false, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.StatsPayload(ix))
	}))
}
