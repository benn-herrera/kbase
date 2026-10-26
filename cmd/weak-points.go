package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func weakPointsCommand() *cobra.Command {
	var maxStrength float64
	var minDependents int
	cmd := queryCommand(&cobra.Command{
		Use:   "weak-points",
		Short: "the shaky, load-bearing claims: the rework targets with most leverage",
		Long: `weak-points lists every claim whose strength is set and under --max-strength
and that has at least --min-dependents dependents: most dependents first, then
lowest strength. A dependent is a node the claim's strength enters — the target
of its outgoing scoring edge. Pending claims are left out.`,
		Args: cobra.NoArgs,
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.WeakPointsPayload(ix, maxStrength, minDependents))
	})
	cmd.Flags().Float64Var(&maxStrength, "max-strength", query.DefaultMaxStrength, "only claims with strength strictly below this count as shaky")
	cmd.Flags().IntVar(&minDependents, "min-dependents", query.DefaultMinDependents, "only claims with at least this many dependents count")
	return cmd
}
