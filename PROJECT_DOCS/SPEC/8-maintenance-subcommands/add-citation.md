### `add-citation`

`add-citation` is [page §3.3](../../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/add-citation.md)'s citation write: its keys, its three forms, its result's `document`,
`line`, `id` and `link`, its refusals, and its re-pointing, in the same write, of every link,
citation or not, into a heading whose anchor the citation moves. The link goes to the cited node's target and is written
relative to the citing document ([page §1.5](../../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)), and a quote is checked against the cited node's statement
as [page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) reads it. What this release adds to the page and holds:

- `document` is any document of the KB, kb-root-relative; a summary page, which kbase renders, is
  none and is refused at `document`.
- `snippet` is matched on the line as written, outside code, links and statement markers, so a
  citation already in place is no match, and a re-issued entry is refused as a line that does not
  contain the snippet. A line past the document's end is refused at `line`.
- An `id` no node carries is refused as an edit's is ([§8](edit-kind.md)): `check: unknown-id` at
  `id`, `detail` `no node <id>`. A hosted node with no statement marker is refused
  `check: statement-marker` at `id`. A quote or a paraphrase of a work or an axiom is refused at
  `snippet`, and a quote absent from the statement at `snippet`, or at `quote` where it is given.
- Where `snippet` is the cited id, the naming form is written whatever `quote` gives.
- A link that would not read, whole, as one link to the target — an unbalanced `[` or `]` in the
  `snippet`, or in the title the naming form writes — is refused at `snippet`, or at `id` for the
  title.
- The links re-pointed are the inline links and the reference definitions outside code, in every
  document but the summary pages, which the derivation renders; only a link's fragment changes.
