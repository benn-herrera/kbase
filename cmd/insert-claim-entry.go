package main

import "github.com/spf13/cobra"

func insertClaimEntryCommand() *cobra.Command {
	return writeOpCommand("insert-claim-entry",
		"mint a claim id, write its node record and edges and, given a document and locator, mark it there",
		`insert-claim-entry mints a clm- id, writes the claim's node record and edges in
one act, and reports the id. Given document and locator together, it places
the claim's statement marker at the located line; a claim with no marker is a
verify fault until mark-claim-in-leaf places one. A claim with the same title
is adopted rather than written twice: one hosted in the given document, else
one with no marker, which is then marked there; with no document, only an
unmarked claim is adopted. A refresh follows unless --no-refresh.`)
}
