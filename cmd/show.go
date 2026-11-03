package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/mcp"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

func showCommand() *cobra.Command {
	return queryCommand(mcp.DescribePositional(&cobra.Command{
		Use:   "show <id>",
		Short: "the full record of one node of any kind",
		Long: `show returns the record of the node carrying ` + argmark.Positional("id") + `, whatever its kind: its
host (the leaf stating it, as find's host), host_anchor (the anchor of the
section stating it), each null for a work, an axiom or a node with no
statement marker, and summary (its entry on a summary page, as
<page>#<anchor>; null where it has none), then its kind's own fields, which for
a work or an axiom begin with label (a work's is its citation key). An id no
node carries is refused.`,
		Args: cobra.ExactArgs(1),
	}, "id", "the id of the node to show"), func(ix *query.Index, args []string) (any, []toolresult.Item, error) {
		record, ok := query.ShowPayload(ix, args[0])
		if !ok {
			return nil, []toolresult.Item{query.NoNode(argmark.Positional("id"), args[0])}, nil
		}
		return answered(record)
	})
}
