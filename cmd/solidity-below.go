package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func init() {
	rootCmd.AddCommand(queryCommand(&cobra.Command{
		Use:   "solidity-below <threshold>",
		Short: "the scored claims with solidity under a threshold",
		Long: `solidity-below lists the full record of every claim whose solidity is set
and strictly under <threshold>, lowest first; pending claims are left out.`,
		Args: cobra.ExactArgs(1),
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		threshold, err := strconv.ParseFloat(strings.TrimSpace(args[0]), 64)
		if err != nil {
			return nil, []toolresult.Item{{Check: checkUsage, Key: "<threshold>", Detail: fmt.Sprintf("threshold %q is not a number", args[0])}}, nil
		}
		return answered(query.SolidityBelowPayload(ix, threshold))
	}))
}
