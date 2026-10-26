package main

import "github.com/spf13/cobra"

func setRationaleCommand() *cobra.Command {
	return writeOpCommand("set-rationale",
		"rewrite a node's rationale",
		`set-rationale rewrites one claim's, analysis's or external work's rationale
in its node record. A refresh follows unless --no-refresh.`)
}
