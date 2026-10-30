package main

import "github.com/spf13/cobra"

func dropClaimNodeCommand() *cobra.Command {
	return writeOpCommand("drop-claim-node",
		"delete a claim: its record, marker and every edge naming it",
		`drop-claim-node deletes a claim node entirely: its node record, its statement
marker in every leaf carrying one, every demoted record naming it, and every
edge record naming it, in one call. The op is refused while a line under
kb-root outside the register pages, or a prose field of another record, still
mentions the id, each named by path and line or by record file, record and
field; remove the mention first. A refresh follows unless --no-refresh.`)
}
