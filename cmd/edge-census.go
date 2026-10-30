package main

import (
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/kb"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func edgeCensusCommand() *cobra.Command {
	scoring := kb.ScoringRelations
	return queryCommand(&cobra.Command{
		Use:   "edge-census",
		Short: "the edge count per relation, and per bearing on the scoring relations",
		Long: `edge-census counts the index's edges of each relation, says whether a score
run's bearing pass asks about that relation, and for each scoring relation —
` + strings.Join(scoring[:len(scoring)-1], ", ") + " and " + scoring[len(scoring)-1] + ` — counts the edges at each bearing,
ascending, then the pending ones, then those whose pairing carries none. The
edge counts are structural; the bearing counts read the bearings the index
holds; neither reads the derivation.` + pendingHelp,
		Args: cobra.NoArgs,
	}, false, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.EdgeCensusPayload(ix))
	})
}
