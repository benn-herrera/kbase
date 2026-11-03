package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func addExperimentCommand() *cobra.Command {
	return writeOpCommand(write.AddExperiment,
		"mint an experiment, write its record and place its statement marker on a named line",
		`add-experiment mints an experiment id, writes the experiment's record and
places its statement marker. The experiment's statement is the section of the
nearest heading at or above the marked line. An experiment carries no
rationale.

`+hostedAddHelp("title"))
}
