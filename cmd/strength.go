package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func strengthCommand() *cobra.Command {
	var minStrength, maxStrength float64
	var sel query.StrengthSelection
	var cmd *cobra.Command
	cmd = queryCommand(&cobra.Command{
		Use:   "strength",
		Short: "claims in a strength range with their dependent counts, or the unscored ones",
		Long: `strength lists the full record of every scored claim whose strength is at least
--min and strictly below --max, with its dependents' count: a dependent is a
node the claim's strength enters — a distinct target of its outgoing scoring
edges.
With neither bound it lists every scored claim. --min-dependents keeps only
the claims with at least that many dependents.

--sort strength, the default, orders by ascending strength, then id; --sort
dependents by most dependents, then lowest strength, then id — the shaky,
load-bearing claims first when --max is low.

--pending lists the claims with no strength yet instead, each with a null
strength; it refuses beside --min or --max. Under it --sort strength orders by
id, and --sort dependents by most dependents, then id. --min-dependents still
applies.` + pendingHelp,
		Args: cobra.NoArgs,
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		sel.Min, sel.Max = nil, nil
		if cmd.Flags().Changed("min") {
			sel.Min = &minStrength
		}
		if cmd.Flags().Changed("max") {
			sel.Max = &maxStrength
		}
		rows, usage := query.StrengthPayload(ix, sel)
		return rows, usage, nil
	})
	cmd.Flags().Float64Var(&minStrength, "min", 0, "only claims with strength at or above this")
	cmd.Flags().Float64Var(&maxStrength, "max", 0, "only claims with strength strictly below this")
	cmd.Flags().IntVar(&sel.MinDependents, "min-dependents", 0, "only claims with at least this many dependents")
	cmd.Flags().StringVar(&sel.Sort, "sort", query.SortStrength, "the order: strength or dependents")
	cmd.Flags().BoolVar(&sel.Pending, "pending", false, "list the claims with no strength instead")
	return cmd
}
