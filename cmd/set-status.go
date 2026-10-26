package main

import "github.com/spf13/cobra"

func setStatusCommand() *cobra.Command {
	return writeOpCommand("set-status",
		"set an experiment's status: run or pending",
		`set-status rewrites one experiment's status in its node record and nothing
else. A refresh follows unless --no-refresh.`)
}
