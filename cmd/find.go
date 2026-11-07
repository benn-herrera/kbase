package main

import (
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func findCommand() *cobra.Command {
	var minStrength, maxStrength float64
	var path, sort string
	var limit int
	var sel query.FindSelection
	var cmd *cobra.Command
	cmd = completeListCommand(&cobra.Command{
		Use:   "find",
		Short: "the nodes of any kind matching filters on kind, title, label, path, strength and dependents",
		Long: `find lists the nodes matching every filter given, across every kind; with no
filter it lists every node. find locates nodes; show reads one. Each row is the
node's id, kind, title, label (a work's citation key or an axiom's label;
null for a claim, an analysis or an experiment), host (the leaf stating it; null
for a work, an axiom or a node with no statement marker), strength and
dependents (the count of nodes its strength enters). A claim's or an
analysis's strength is derived; a work's, an experiment's or an axiom's is its
rigor.

` + argmark.Flag("kind") + ` repeats, keeping the nodes of any kind it names. ` + argmark.Flag("label") + `
matches only works and axioms, the kinds that carry a label. ` + argmark.Flag("path") + ` matches
a node's host, so a node whose host is null never matches it. ` + argmark.Flag("min") + ` and ` + argmark.Flag("max") + `
compare scored strengths only: a pending node matches neither, so ` + argmark.FlagValue("min", "0") + `
keeps exactly the scored nodes, and ` + argmark.FlagValue("sort", "strength") + ` without it ranks the
pending ones first. ` + argmark.Flag("pending") + ` is refused beside ` + argmark.Flag("min") + ` or ` + argmark.Flag("max") + `.

Sort ties break by ascending strength, then ascending id, in either ` + argmark.Flag("order") + `.`,
		Args: cobra.NoArgs,
	}, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		sel.Path, sel.Min, sel.Max, sel.Sort, sel.Limit = nil, nil, nil, nil, nil
		if cmd.Flags().Changed("path") {
			sel.Path = &path
		}
		if cmd.Flags().Changed("min") {
			sel.Min = &minStrength
		}
		if cmd.Flags().Changed("max") {
			sel.Max = &maxStrength
		}
		if cmd.Flags().Changed("sort") {
			sel.Sort = &sort
		}
		if cmd.Flags().Changed("limit") {
			sel.Limit = &limit
		}
		rows, usage := query.FindPayload(ix, sel)
		return rows, usage, nil
	})
	cmd.Flags().StringArrayVar(&sel.Kinds, "kind", nil, "only nodes of this kind, one of "+strings.Join(kb.NodeKinds, ", "))
	cmd.Flags().StringVar(&sel.Title, "title", "", "only nodes whose title contains this, case-insensitively")
	cmd.Flags().StringVar(&sel.Label, "label", "", "only works and axioms whose label contains this, case-insensitively")
	cmd.Flags().StringVar(&path, "path", "", `only nodes whose host is this leaf or lies under this kb-root-relative directory, "" or . for kb-root; never a node with a null host`)
	cmd.Flags().Float64Var(&minStrength, "min", 0, "only nodes scored at or above this")
	cmd.Flags().Float64Var(&maxStrength, "max", 0, "only nodes scored strictly below this")
	cmd.Flags().BoolVar(&sel.Pending, "pending", false, "only nodes with no strength yet")
	cmd.Flags().IntVar(&sel.MinDependents, "min-dependents", 0, "only nodes with at least this many dependents")
	cmd.Flags().StringVar(&sort, "sort", query.SortID, "the key to sort by: "+strings.Join(query.FindSorts, ", "))
	cmd.Flags().StringVar(&sel.Order, "order", query.OrderAscending, sortOrderUsage)
	cmd.Flags().IntVar(&limit, "limit", 0, sortLimitUsage)
	return cmd
}
