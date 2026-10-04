package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	var maxSolidity float64
	var minDependents int
	cmd := queryCommand(&cobra.Command{
		Use:   "weak-points",
		Short: "the shaky, load-bearing claims: the rework targets with most leverage",
		Long: `weak-points lists every claim whose solidity is set and under --max-solidity
and that at least --min-dependents nodes hold an edge to: most dependents first,
then lowest solidity. Pending claims are left out.`,
		Args: cobra.NoArgs,
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.WeakPointsPayload(ix, maxSolidity, minDependents))
	})
	cmd.Flags().Float64Var(&maxSolidity, "max-solidity", query.DefaultMaxSolidity, "only claims with solidity strictly below this count as shaky")
	cmd.Flags().IntVar(&minDependents, "min-dependents", query.DefaultMinDependents, "only claims with at least this many dependents count")
	rootCmd.AddCommand(cmd)
}
