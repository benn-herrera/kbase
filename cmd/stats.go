package main

import (
	"strconv"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func statsCommand() *cobra.Command {
	bar := strconv.FormatFloat(query.DefaultMaxStrength, 'f', -1, 64)
	ranked := strconv.Itoa(query.StatsRankLimit)
	return queryCommand(&cobra.Command{
		Use:   "stats",
		Short: "a snapshot of the KB in one call: node, edge and relation counts, manual and model value counts, strength bands, and the highest-leverage and weakest claims",
		Long: `stats gives a snapshot of the KB in one call:
- claims, analyses, experiments, axioms, works: one count per node kind;
- edges, demoted_edges;
- subtree_aggregates: the claims and experiments stated under each volume;
- rigor_manual, rigor_model: the rigors marked manually scored, and those not;
  bearing_manual, bearing_model: the bearings likewise;
- relations: per relation, its edge count, whether a score run's bearing pass
  asks about it, and on a scoring relation the edges at each bearing
  (ascending, then pending, then those whose pairing carries none);
- strength_bands: the claims in each strength band, then the pending ones;
- highest_leverage: the ` + ranked + ` claims below ` + bar + ` with the most dependents;
- weakest: the ` + ranked + ` lowest-strength claims.
find's strength filters give the full lists:
` + argmark.Call("find", findArgs(query.HighestLeverage())) + ` for highest_leverage,
` + argmark.Call("find", findArgs(query.Weakest())) + ` for weakest.`,
		Args: cobra.NoArgs,
	}, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		return answered(query.StatsPayload(ix))
	})
}

// findArgs is find's arguments asking the question sel states — the kinds,
// bounds, dependents floor, sort and order stats' sections set — its limit
// left out: the full list a section previews.
func findArgs(sel query.FindSelection) map[string]any {
	args := map[string]any{}
	if len(sel.Kinds) > 0 {
		args["kind"] = sel.Kinds
	}
	for name, value := range map[string]*float64{"min": sel.Min, "max": sel.Max} {
		if value != nil {
			args[name] = *value
		}
	}
	if sel.MinDependents != 0 {
		args["min-dependents"] = sel.MinDependents
	}
	if sel.Sort != nil {
		args["sort"] = *sel.Sort
	}
	if sel.Order != query.OrderAscending {
		args["order"] = sel.Order
	}
	return args
}
