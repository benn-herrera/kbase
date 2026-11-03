#### `add-citation` (write op)

The author writes the cited text in the citing document, then calls this op. kbase finds the cited
node's link target ([§1.5](../../1-model/1.5-citations.md)), computes the relative path and the anchor, and replaces `snippet` on the
named line with the citation link.

- **Keys**:
  - `document` (the citing document, kb-root-relative, required);
  - `line` (required);
  - `snippet` (required): exactly the text on that line that becomes the link text. Unlike the
    marker ops' `snippet`, it is the span that gets replaced, not just a check, so it must occur
    exactly once on the line. A `snippet` occurring more than once there is refused, naming each
    column, since kbase does not guess which span the caller meant;
  - `id` (the cited node, required);
  - `quote` (optional). Without `quote`, `snippet` is a verbatim quote and must appear in the cited
    node's statement; it becomes `["<snippet>"](<path>#<anchor> "<id>")`. With `quote`, `snippet`
    is a paraphrase, and `quote` is the passage paraphrased, which must appear verbatim in the cited
    statement; it becomes `[<snippet>](<path>#<anchor> "<id>")`. Where `snippet` is the cited
    node's id itself, it becomes the naming form, `[<title>](<path>#<anchor> "<id>")`. The author
    writes the id where the reference belongs and converts it. A work or an axiom takes the
    naming form only ([§1.5](../../1-model/1.5-citations.md)).
- A citation written into a heading that moves an anchor — that heading's own, or a later
  same-text heading's `-1`, `-2` suffix — re-points every link whose destination resolves to a
  moved heading, citation or not, in the same write.
- **Result**: `document`, `line`, `id`, `link` (the text written).
- **Refusals**:
  - the line does not contain `snippet`: remedy, re-read the document, with the nearby lines where
    the snippet appears;
  - the quote (`snippet`, or `quote` where given) does not appear verbatim in the cited statement;
  - a quote or a paraphrase of a work or an axiom, which have no statement: remedy, the naming
    form;
  - a hosted node with no marker (a `statement-marker` fault, [§1.4](../../1-model/1.4-markers.md)), which has no link target;
  - an `id` no node carries;
  - `document` not a document of the KB;
  - a link that would not read as one link: an unbalanced `[` or `]` in `snippet` (refused at
    `snippet`) or, for the naming form, in the cited node's title (refused at `id`).
