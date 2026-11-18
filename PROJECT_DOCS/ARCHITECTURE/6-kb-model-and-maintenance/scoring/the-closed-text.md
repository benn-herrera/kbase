#### The closed text

**Pattern:** Line-numbered context for grounded citation.

`internal/score/text.go`'s two unexported renderers assemble the item text a call carries.

`rigorText` renders a node's title and its statement. The statement is `kb.Statement`'s reading of the node
in the leaf hosting it, the same reader `add-citation`'s quote check uses. The run's `graph` contains it
for every hosted node, read once per leaf by `graphOf` with the `lifts` edges of the in-memory index,
markers and `>` prefixes removed. A node no leaf hosts (a work, an axiom, a node with no marker) renders
its title alone. `bearingText` renders the source's and the target's sections by that same rule, then
the edge's authored `context` from the row `Index.Edge(ScoringEdge)` answers. Each end's label opens a
line of its own, as does the context's.

`assemble` wraps every section of the item text at word boundaries to `wrapWidth`
([§12](../../12-constants/index.md)), keeping the section's own line breaks and
rewriting nothing, and numbers the lines lying in a section that a charge may cite, from 1 over the
whole text. `closedText.numbered` writes `N| ` before a numbered line; the gutter is the template
wrapper's, not part of the text the call sends. A label line, a blank separator line and the edge's
context are written bare: a line with no number is no site. An out-of-scope subject is refused by name
and type: a `wrk-` or `axm-` id on `rigorText`, a `references` or `demoted` relation on `bearingText`.
