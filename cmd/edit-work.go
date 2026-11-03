package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func editWorkCommand() *cobra.Command {
	return writeOpCommand(write.EditWork,
		"change a work's label, title, rigor or rationale",
		`edit-work changes the record of the work "id" names.

A "label" change rewrites the old key wherever it is used as a key: each key
of a citation span's data-cites, and the span's text where it is the key-only
rendering (the parenthesised key list); each reference-list entry's
id="ref-<key>"; each link into #ref-<key>. Prose naming the key as words
stands. A "label" that a citation span or a reference-list entry already uses
as a key is refused, naming each path:line (the rewrite would merge two
references into one; remedy: give a label no span or entry uses).

`+editFieldsHelp(`A "title" or a "label"`, "title", "rationale")+`

`+editLinksHelp(`A "title" change`, "the work's own entry on the root summary page")+`

`+editResultHelp+`
The result also gives "rewritten" for a label change: the path and line of
each line rewritten.`)
}
