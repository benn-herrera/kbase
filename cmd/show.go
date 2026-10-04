package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	rootCmd.AddCommand(queryCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "the full record of one node of any kind",
		Long: `show returns the record of the node carrying <id>, whatever its kind, each
kind with its own fields. An id no node carries is refused.`,
		Args: cobra.ExactArgs(1),
	}, false, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		record, ok := query.ShowPayload(ix, args[0])
		if !ok {
			return nil, []toolresult.Item{query.UnknownNode(args[0])}, nil
		}
		return answered(record)
	}))
}
