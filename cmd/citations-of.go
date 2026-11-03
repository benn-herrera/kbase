package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/mcp"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// citationsOfCommand is the citations-of query, its answer complete, count
// beside it.
func citationsOfCommand() *cobra.Command {
	return completeListCommand(mcp.DescribePositional(&cobra.Command{
		Use:   "citations-of <id>",
		Short: "every citation of a node in prose, by the id in its link title",
		Long: `citations-of lists every citation of the node ` + argmark.Positional("id") + ` names, found by the id in
its link title wherever the link points, outside code, in every document but
the summary pages: one row per citation, its path (the citing document),
line and text (the link text as written), ordered by path, then line. The
answer is complete, count beside it. An id no node carries is refused, no
node.`,
		Args: cobra.ExactArgs(1),
	}, "id", "the id of the node whose citing links to list"), func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		rows, ok, err := query.CitationsOfPayload(ix, args[0])
		if !ok {
			return nil, []toolresult.Item{query.NoNode(argmark.Positional("id"), args[0])}, nil
		}
		return rows, nil, err
	})
}
