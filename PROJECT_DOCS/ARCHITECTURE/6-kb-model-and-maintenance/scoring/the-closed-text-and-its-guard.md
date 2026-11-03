#### The closed text and its guard

`internal/score/text.go`'s two unexported renderers assemble every
character a call may see. `rigorText` renders a node's title and its statement: the statement is
`kb.Statement`'s reading of the node in the leaf hosting it — the one reader `add-citation`'s quote
check reads too — which the run's `graph` holds for every hosted node, read once per leaf by
`graphOf` with the `lifts` edges of the loaded index, markers and `>` prefixes removed, with the
1-based line of the leaf it opens on (`statementLine`: the line nearest the node's marker whose
text, markers and `>` prefixes removed, begins with the statement's first line). A node no leaf
hosts — a work, an axiom, a node with no marker — renders its title alone; a work's record's
`rationale` is never sent. `bearingText` renders the source's and the target's statements by that
same rule, then the edge's authored `context` from the row `Index.Edge(ScoringEdge)` answers. Each
end's label opens a line of its own, and so does the context's, so no numbered line begins with
frame text. `assemble` wraps every passage at word boundaries to `wrapWidth` — 100 columns —
keeping the passage's own line breaks and rewriting nothing, and numbers the lines lying in a
passage with an `end`, from 1 over the whole text; `closedText.numbered` writes `N| ` before a
numbered line, which is the frame's gutter and no part of the carried text, and carries a label
line, a blank separator line and the edge's context bare — a line with no number is no site.
`scoreTokenRE` and `frameRE` scan each assembled passage — no guard pattern spans the blank line
between two — and a hit is `scoreTokenError`, naming the document and line, with the item `failed`
and no call made: the guard refuses, it never strips. The four scoring tokens are half of what it
refuses; `frameRE` adds the frame's own structure — a run of twelve backticks closes the fence the
templates draw around the corpus, and what follows then sits outside the instruction to judge it,
and a line opening `## Source:`, `## Target:` or `## Context` forges the frame's map of the text; a
label a blockquote or an indent displaces from the line's start opens nothing. Neither half is
adaptive to the text: a rendered prompt stays a function of its template, so a text carrying the
frame's structure is refused rather than escaped. An out-of-scope subject is refused by name and
type — a `wrk-` or `axm-` id on `rigorText`, a `references` or `demoted` relation on `bearingText`.
