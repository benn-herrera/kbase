package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

func addCitationCommand() *cobra.Command {
	return writeOpCommand(write.AddCitation,
		"replace a span of a named line with a citation of a node",
		`add-citation turns text you have written into a citation of the node "id"
names. The text is the span "document", "line" and "snippet" name, outside
code, links and statement markers. kbase links it to the node's place — a
claim's, an analysis's or an experiment's leaf and section anchor, an axiom's
heading in axioms.md, a work's entry on the root summary page — written
relative to the citing document, with the id as its link title.

The form follows the keys. To cite a node by name, write its id where the
reference belongs and give the id as the snippet: it becomes the naming form,
the node's title as the link text. A work or an axiom has no statement and
takes the naming form only. To quote a node's statement, the snippet is a
verbatim span of it, given without "quote", and becomes
["<snippet>"](<path>#<anchor> "<id>"). To paraphrase it, the snippet is your
paraphrase and "quote" holds the passage it paraphrases, verbatim from the
statement: [<snippet>](<path>#<anchor> "<id>"). The quote is checked once,
now; later edits to the cited text are yours to keep faithful. Where the link
sits in a heading and moves an anchor, as the naming form's title can, every
link into a moved heading, a citation or not, is re-pointed in the same write.

Refused, writing nothing: a line past the document's end, or one that does
not contain the snippet, naming the nearby lines that do — re-read the
document, it has likely changed; a snippet occurring more than once on the
line, naming each column; a quote that does not appear verbatim in the cited
statement; a snippet with an unbalanced [ or ], which would write a link no
reader sees, or a naming form whose title has one, refused at "id"; a quote or
a paraphrase of a work or an axiom; a node with no statement marker, which has
no place to link to; an "id" no node carries, no node; and a document that is
not one of the KB's.

The result gives each entry's "document", "line", "id" and "link", the text
written.`)
}
