package main

import (
	"strconv"

	"github.com/spf13/cobra"

	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func statsCommand() *cobra.Command {
	return queryCommand(&cobra.Command{
		Use:   "stats",
		Short: "the index census, the build-band distribution and the rework dashboard",
		Long: `stats counts the nodes of each kind, the records of each index file and the
claims in each build band, and lists under highest_leverage the ten claims below
` + strconv.FormatFloat(query.DefaultMaxStrength, 'f', -1, 64) + ` with the most dependents and under weakest the ten lowest-strength
claims; ` + "`strength`" + ` gives the full lists. Under "relations" it counts the
edges of each relation, with whether a score run's bearing pass asks about the
relation and, on a scoring relation, the edges at each bearing: ascending,
then the pending ones, then those whose pairing carries none. The value surfaces read the derivation;
the counts and the census are structural, and the bearing counts read the
bearings the index holds.` + pendingHelp,
		Args: cobra.NoArgs,
	}, false, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.StatsPayload(ix))
	})
}
