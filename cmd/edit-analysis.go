package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func editAnalysisCommand() *cobra.Command {
	return writeOpCommand(write.EditAnalysis,
		"change an analysis's title, rigor or rationale",
		`edit-analysis changes the record of the analysis "id" names.

`+hostedEditHelp("analysis", "title", "rationale"))
}
