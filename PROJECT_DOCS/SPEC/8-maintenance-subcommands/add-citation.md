### `add-citation`

`add-citation` turns text the author wrote in a document into a citation of a node ([§10](../10-metadata-format-version/citations.md)): it replaces `snippet` on the named line with a link to the cited node's link target, written relative to the citing document.

- **Keys**:
  - `document` (the citing document, kb-root-relative), `line` and `snippet`, required. `snippet` is the span of the line that becomes the link text; it is replaced, not just checked, so it must occur exactly once on the line, matched as written, outside code, links and statement markers. A `snippet` the line lacks is refused, naming the nearby lines that contain it ([§10](../10-metadata-format-version/statements-and-anchors.md)); one that occurs more than once is refused, naming each column, since kbase does not guess which span the caller meant;
  - `id` (the cited node), required;
  - `quote`, optional.
- **Forms**, by the keys given. Each is a Markdown link whose title is the cited node's id:
  - quote: `quote` absent. `snippet` is a verbatim quote and must appear in the cited node's statement; it is written `["<snippet>"](<path>#<anchor> "<id>")`;
  - paraphrase: `quote` given. `snippet` is the paraphrase, written `[<snippet>](<path>#<anchor> "<id>")`; `quote` is the passage paraphrased and must appear verbatim in the cited statement;
  - naming: `snippet` is the cited node's id itself. It is written `[<title>](<path>#<anchor> "<id>")`, whatever `quote` gives. The author writes the id where the reference belongs and converts it.

  A work or an axiom has no statement, so it takes the naming form only. A quote is compared with the statement as [§10](../10-metadata-format-version/statements-and-anchors.md) reads it, each whitespace run collapsed.
- **Result**: `document`, `line`, `id`, and `link` (the text written).
- A citation written into a heading that moves an anchor — that heading's own, or a later same-text heading's `-1`, `-2` suffix — updates every link whose destination resolves to a moved heading, citation or not, in the same write. The links updated are the inline links and the reference definitions outside code, in every document but the summary pages, which the rebuild renders; only a link's fragment changes.

**Refusals**:
- `document` is any document of the KB, kb-root-relative; a summary page, which kbase renders, is none and is refused at `document`;
- a `line` past the document's end: at `line`. A `snippet` the line does not contain is refused at `snippet`. An identical repeat — the named line already holds, in place of the snippet, exactly the link this entry would write — is `unchanged` ([§8](write-batches/idempotence.md)); a citation of the snippet's text in another form or to another node is no match, and is refused as a line that does not contain the snippet;
- an `id` no node has: `check: unknown-id` at `id`, `detail` `no node <id>`, as an edit's is ([§8](edit-kind.md));
- a hosted node with no statement marker, which has no link target: `check: statement-marker` at `id`;
- a quote or a paraphrase of a work or an axiom: at `snippet`, the remedy naming the naming form;
- a quote absent from the cited statement: at `snippet`, or at `quote` where it is given;
- a link that would not parse, whole, as one link to the target — an unbalanced `[` or `]` in the `snippet`, or in the title the naming form writes: at `snippet`, or at `id` for the title.

