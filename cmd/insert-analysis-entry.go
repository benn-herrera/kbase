package main

import "github.com/spf13/cobra"

func insertAnalysisEntryCommand() *cobra.Command {
	return writeOpCommand("insert-analysis-entry",
		"mint an analysis id, write its node record and edges and, given a document, mark it there",
		`insert-analysis-entry mints an ana- id, writes the analysis's node record and
edges in one act, and reports the id. Given a document, it places the
analysis's statement marker on a new line directly after that document's first
heading; an analysis with no marker is a verify fault until one is placed. An
analysis with the same title is adopted rather than written twice: one hosted
in the given document, else one with no marker, which is then marked there;
with no document, only an unmarked analysis is adopted. A refresh follows
unless --no-refresh.`)
}
