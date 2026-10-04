package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	rootCmd.AddCommand(queryCommand(&cobra.Command{
		Use:   "referenced-by <id>",
		Short: "the leaves whose body links to a claim's originating leaf",
		Long: `referenced-by scans every leaf body under kb-root/ and lists, sorted, the
leaves holding a link to the leaf that originates <id> — the first, by path,
citing it — that leaf itself excepted.`,
		Args: cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		leaves, err := query.ReferencedByPayload(ix, args[0])
		return leaves, nil, err
	}))
}
