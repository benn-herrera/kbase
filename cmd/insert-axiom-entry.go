package main

import "github.com/spf13/cobra"

func insertAxiomEntryCommand() *cobra.Command {
	return writeOpCommand("insert-axiom-entry",
		"record an axiom by label and title",
		`insert-axiom-entry records an axiom by its label: it writes the axiom's
axm- node record and reports the id. An axiom with the same label is adopted
rather than written twice, and a batch naming one new label twice is refused.
A refresh follows unless --no-refresh.`)
}
