package main

import "github.com/spf13/cobra"

func insertExperimentEntryCommand() *cobra.Command {
	return writeOpCommand("insert-experiment-entry",
		"mint an experiment id, write its node record and mark it in its hosting document",
		`insert-experiment-entry mints an exp- id, writes the experiment's node record
and edges in one act, places its statement marker on a new line directly after
the given document's first heading, and reports the id. An experiment of the
same title and status is adopted where its rigor and its supports, lifts and
refutes lists equal those the values supply — a key or a pair's bearing left
out is not compared: one hosted in the document, else one with no marker, which
is then marked there. Otherwise a new experiment is minted. The document must
be a leaf of the KB with a heading. A refresh follows unless --no-refresh.`)
}
