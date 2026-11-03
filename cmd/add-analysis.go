package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func addAnalysisCommand() *cobra.Command {
	return writeOpCommand(write.AddAnalysis,
		"mint an analysis, write its record and place its statement marker on a named line",
		`add-analysis mints an analysis id, writes the analysis's record and places
its statement marker. The analysis's statement is the section of the nearest
heading at or above the marked line. Naming the line a claim's marker ends
places the analysis's marker directly after the claim's, with nothing between
them, the layout of an analysis paired with that claim. The pairing writes no
edge between them.

`+hostedAddHelp("title", "rationale"))
}
