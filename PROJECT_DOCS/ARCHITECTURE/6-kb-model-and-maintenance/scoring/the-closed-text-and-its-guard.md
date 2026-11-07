#### The closed text and its guard

**Pattern:** Line-numbered context for grounded citation; delimiter-based prompt-injection guard.

`internal/score/text.go`'s two unexported renderers assemble every character a call may see.

`rigorText` renders a node's title and its statement. The title section is read through the node's
canonical path (`src.KBPath(row.CanonicalPath)`). The statement is `kb.Statement`'s reading of the node
in the leaf hosting it, the same reader `add-citation`'s quote check uses. The run's `graph` contains it
for every hosted node, read once per leaf by `graphOf` with the `lifts` edges of the in-memory index,
markers and `>` prefixes removed, with the 1-based line of the leaf it opens on (`statementLine`: the
line nearest the node's marker whose text, markers and `>` prefixes removed, begins with the statement's
first line). A node no leaf hosts (a work, an axiom, a node with no marker) renders its title alone.
`bearingText` renders the source's and the target's sections by that same rule, then the edge's authored
`context` from the row `Index.Edge(ScoringEdge)` answers. Each end's label opens a line of its own, as
does the context's, so no numbered line begins with template-wrapper text.

`assemble` wraps every section of the item text at word boundaries to `wrapWidth` ([§12](../../12-constants/index.md)),
keeping the section's own line breaks and rewriting nothing, and numbers the lines lying in a section
that a charge may cite, from 1 over the whole text. `closedText.numbered` writes `N| ` before a numbered
line; the gutter is the template wrapper's, not part of the text the call sends. A label line, a blank
separator line and the edge's context are written bare: a line with no number is no site.

**Pattern:** Delimiter-based prompt-injection guard. `scoreTokenRE` and `frameRE` scan each assembled
section; no guard pattern spans the blank line between two. A hit is `scoreTokenError`, naming the
document and line, and the item is `failed` with no call made (SPEC §8 One item, one quantity, one call).
The input guard refuses and never strips or escapes, because a rendered prompt stays a function of its
template. `scoreTokenRE` matches the four scoring tokens. `frameRE` matches the template wrapper's own
structure: a run of twelve backticks closes the fence the templates draw around the item text, after
which the text sits outside the instruction to judge it; a line opening `## Source:`, `## Target:` or
`## Context` forges the wrapper's map of the text. A label that a blockquote or an indent displaces from
the line's start opens nothing. The guard
sees only the item text, and a re-ask's correction carries reply text the item text never held, so
`correctionOf` runs `refused` over each quoted reply line before the correction is rendered. A hit drops
the quote slot to `""`, and the capture records `quoted: []`. The correction's values are not guarded:
each is a digit string or an id the parse already read as `flt-[a-z0-9-]+`, and nothing the guard
refuses fits either shape. Nothing is stripped or escaped. A quoted line is never a numbered
line, so no charge can cite it. An out-of-scope subject is refused by name and type: a `wrk-` or `axm-`
id on `rigorText`, a `references` or `demoted` relation on `bearingText`.

