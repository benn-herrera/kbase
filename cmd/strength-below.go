package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func strengthBelowCommand() *cobra.Command {
	var maxStrength float64
	cmd := queryCommand(&cobra.Command{
		Use:   "strength-below",
		Short: "the scored claims with strength under a threshold",
		Long: `strength-below lists the full record of every claim whose strength is set and
strictly under --max-strength, lowest first; pending claims are left out.`,
		Args: cobra.NoArgs,
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.StrengthBelowPayload(ix, maxStrength))
	})
	cmd.Flags().Float64Var(&maxStrength, "max-strength", query.DefaultMaxStrength, "only claims with strength strictly below this are listed")
	return cmd
}
