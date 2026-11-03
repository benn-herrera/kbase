package main

import (
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/mcp"
	"kbase/internal/write"
)

// writeCommand is kbase write: one batch whose entries each name their op,
// the command the MCP write tool binds.
func writeCommand() *cobra.Command {
	cmd := valuesCommand(write.Batch, "run one write batch: ordered entries, each naming its op, all written or none",
		batchHelp+"\n\nThe write ops: "+strings.Join(write.Ops(), ", ")+".")
	return mcp.Notes(cmd, `The arguments are the batch: "entry" lists the entries, each naming its op
under "operation" beside that op's keys; dry-run: true does everything but
write, and commit: true commits the batch. A batch applies its entries in the
order given, each checked against the records as the earlier entries left
them; kbase never reorders a batch. Shape errors are all reported at once;
then the batch stops at the first refusal and writes nothing. An op's own
help: kbase://help/<operation>.`,
		`The batch arrives as YAML (JSON accepted) on stdin, or in the file --values
names; "entry" is a key of that document, not a flag. Each op also has its own
subcommand, kbase <op>, taking a batch of that op alone, its entries naming no
"operation"; kbase <op> --help is that op's help.`)
}
