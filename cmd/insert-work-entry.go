package main

import "github.com/spf13/cobra"

func insertWorkEntryCommand() *cobra.Command {
	return writeOpCommand("insert-work-entry",
		"record an external work by its citation key",
		`insert-work-entry records an external work by its citation key: it writes the
work's wrk- node record and reports the id. Works are rendered on the root
register page. A work with the same key is adopted rather than written twice,
and a batch naming one new key twice is refused. A refresh follows unless
--no-refresh.`)
}
