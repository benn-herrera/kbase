package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/write"
)

// editFieldsHelp is the paragraph opening an edit op's shared help: what an
// edit replaces, when it is unchanged, and its refusals. unique is the keys
// unique within the kind, quoted; prose the op's prose keys.
func editFieldsHelp(unique string, prose ...string) string {
	return `A field given replaces its value whole, and a field left out stands; an entry
gives at least one. An entry whose every value, and mark, already stands is
unchanged.
` + unique + ` another node of the kind carries is refused, naming that
node. An "id" no node carries is refused.
` + oneLineHelp(prose...)
}

// editLinksHelp is the paragraph of an edit op's help on how a change keeps
// every link to the node live: relinking is the change that does, and
// ownAnchor the node's own anchor it moves.
func editLinksHelp(relinking, ownAnchor string) string {
	return relinking + ` keeps every link to the node live in the same
write: a citation whose link text is the old title, the naming form, gets the
new title as its text, and any other link text, a quote or a paraphrase,
stands. A change can move three kinds of anchor: the anchor of a heading that
a rewritten naming-form citation sits in, the -1, -2 suffix of any later
heading with the same text, and ` + ownAnchor + `.
Every link into a moved anchor, a citation or not, is re-pointed to its new
anchor; only the link target changes. A title with an unbalanced [ or ], which
would write a naming-form citation no reader sees, is refused at "title",
naming that citation's path:line. An edit writes no other document.`
}

// editResultHelp is what an edit op's help says of its result.
const editResultHelp = `The result gives each entry's "id".`

// hostedEditHelp is the help closing edit-claim's, edit-analysis's and
// edit-experiment's bodies, of kind, whose title alone is unique and whose
// prose keys are prose.
func hostedEditHelp(kind string, prose ...string) string {
	return editFieldsHelp(`A "title"`, prose...) + "\n\n" +
		manuallyScoredHelp("rigor", true, "") + "\n\n" +
		editLinksHelp(`A "title" change`, "the "+kind+"'s own summary entry") + "\n\n" +
		editResultHelp
}

func editClaimCommand() *cobra.Command {
	return writeOpCommand(write.EditClaim,
		"change a claim's title, rigor or rationale",
		`edit-claim changes the record of the claim "id" names.

`+hostedEditHelp("claim", "title", "rationale"))
}
