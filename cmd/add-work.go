package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func addWorkCommand() *cobra.Command {
	return writeOpCommand(write.AddWork,
		"record an external work by its citation key, minting its id",
		`add-work records an external work and mints its id, wrk-<minted>, given in
the result's "id". The work is shown on the root summary page. A work of that
"label", or of that "title", already standing is refused (node exists: <id>;
remedy: drop this entry and use that id wherever you would have named the new
node, or give a label or title no work carries); nothing is adopted. A line
break in "title" or "rationale" is refused, and any other whitespace run
collapses to one space.`)
}
