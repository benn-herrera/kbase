package main

import "github.com/spf13/cobra"

func dropClaimNodeCommand() *cobra.Command {
	return writeOpCommand("drop-claim-node",
		"delete a claim's node record and marker",
		`drop-claim-node deletes a claim node entirely: its node record, its statement
marker in every leaf carrying one, every demoted record naming it, and its own
edges — every founds edge and every claim's supports edge into it, and the
references and refutes edges out of it. The op is refused while another node's
edge record names the claim — a supports edge from it into another node, a
references or refutes edge into it, a lift into it or an experiment's support
of it: run drop-claim-edges and drop-inbound-edges first. The op is refused,
too, while a line under kb-root or a prose field of another record still
mentions the id, each named by path and line or by record file, record and
field; remove the mention first. A refresh follows unless --no-refresh.`)
}
