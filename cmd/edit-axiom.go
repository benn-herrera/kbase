package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func editAxiomCommand() *cobra.Command {
	return writeOpCommand(write.EditAxiom,
		"change an axiom's label or title, rewriting its heading in axioms.md",
		`edit-axiom changes the label or the title of the axiom "id" names. Either
change rewrites the axiom's heading,
### Axiom <label>: <title><!-- kbase-id: <id> -->, in kb-root/axioms.md,
keeping its marker and appending the heading where none stands.

`+editFieldsHelp(`A "title" or a "label"`, "label", "title")+`

`+editLinksHelp(`A "title" or "label" change`, "the axiom's own heading in axioms.md")+`

`+editResultHelp)
}
