package main

import (
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/mcp"
	"kbase/internal/write"
)

// writeCommand is kbase write: one batch whose entries each name their op,
// the command the MCP write tool binds.
func writeCommand() *cobra.Command {
	cmd := valuesCommand(write.Batch, "run one write batch: ordered entries, each naming its op, all written or none",
		batchHelp+"\n\nThe write ops: "+strings.Join(write.Ops(), ", ")+".")
	return mcp.Notes(cmd, `The arguments are the batch: "entry" lists the entries, each naming its op
under "operation" beside that op's keys, with `+argmark.Flag("dry-run")+` and `+argmark.Flag("commit")+`
as its arguments. An op's own help: `+mcp.HelpURIPrefix+`<operation>.`,
		`The batch arrives as YAML (JSON accepted) on stdin, or in the file --values
names; "entry" is a key of that batch, not a flag. Each op also has its own
subcommand, kbase <op>, taking a batch of that op alone, its entries naming no
"operation"; kbase <op> --help is that op's help.`)
}
