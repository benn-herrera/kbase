package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func editExperimentCommand() *cobra.Command {
	return writeOpCommand(write.EditExperiment,
		"change an experiment's title, status or rigor",
		`edit-experiment changes the record of the experiment "id" names.

`+hostedEditHelp("experiment", "title"))
}
