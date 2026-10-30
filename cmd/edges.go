package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/kb"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func edgesCommand() *cobra.Command {
	var relations []string
	cmd := queryCommand(&cobra.Command{
		Use:   "edges",
		Short: "the edges of the named relations, or of every relation, with their bearings",
		Long: `edges lists the index's edge rows — source, relation, target, bearing,
context, and whether a score run's bearing pass asks about the edge — ordered
by relation, source and target. --relation narrows the list to the named
relations; it repeats.` + pendingHelp,
		Args: cobra.NoArgs,
	}, true, func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		var unknown []string
		var usage []toolresult.Item
		for _, r := range relations {
			if !slices.Contains(kb.Relations, r) && !slices.Contains(unknown, r) {
				unknown = append(unknown, r)
				usage = append(usage, toolresult.Item{Check: toolresult.CheckUsage, Key: "--relation", Allowed: kb.Relations,
					Detail: fmt.Sprintf("--relation %q names no relation", r)})
			}
		}
		if usage != nil {
			return nil, usage, nil
		}
		return answered(query.EdgesPayload(ix, relations))
	})
	cmd.Flags().StringArrayVar(&relations, "relation", nil, "only edges of this relation, one of "+strings.Join(kb.Relations, ", ")+"; repeatable")
	return cmd
}
