package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func addAxiomCommand() *cobra.Command {
	return writeOpCommand(write.AddAxiom,
		"record an axiom by label and title, minting its id, and give it its marked heading in axioms.md",
		`add-axiom records an axiom and mints its id, axm-<minted>, given as its
entry's "id" in the result's "entries". A "label" may hold LaTeX. It writes
the record and gives the axiom its heading in kb-root/axioms.md,
### Axiom <label>: <title><!-- kbase-id: <id> -->, whose marker pairs the
heading with the record: where one unmarked heading
### Axiom <label>: <title> stands, it takes the marker; otherwise the heading is
appended, creating the file where it is absent. Its entry's "heading" in the
result's "entries" says which: marked or appended. More than one unmarked
heading of that text is refused (naming their lines; remedy: reword or remove
all but one, then re-run), since kbase does not guess which one the axiom is.
The axiom's prose under the heading is yours to write. Over an axiom of that
"label", or of that "title", already standing, an identical add — the same
values, and its marked heading of that text in place — is unchanged and
reports that axiom's id, heading marked; any other is refused (node exists:
<id>; remedy: drop this entry and use that id wherever you would have named
the new node, or give a label or title no axiom carries); nothing is adopted.
A line break in "label" or "title" is refused, and any other whitespace run
collapses to one space.`)
}
