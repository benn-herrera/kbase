package main

import "github.com/spf13/cobra"

func dropInboundEdgesCommand() *cobra.Command {
	return writeOpCommand("drop-inbound-edges",
		"remove every other node's edges naming one claim",
		`drop-inbound-edges removes every edge record by which another node names the
claim id: a supports edge from id into another node, a references edge into
id, and every lifts or refutes edge into id and every experiment's supports edge
into it. A claim no other node names is left as it stands. A refresh follows
unless --no-refresh.`)
}
