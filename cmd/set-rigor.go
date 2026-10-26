package main

import "github.com/spf13/cobra"

func setRigorCommand() *cobra.Command {
	return writeOpCommand("set-rigor",
		"rewrite a node's local rigor",
		`set-rigor rewrites one claim's, analysis's, experiment's or external work's
rigor in its node record, marking a claim's, analysis's or experiment's rigor
manually-scored. A value is a number in [0, 1] or *pending*. A refresh follows
unless --no-refresh.`)
}
