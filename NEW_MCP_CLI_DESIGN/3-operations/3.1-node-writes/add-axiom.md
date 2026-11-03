#### `add-axiom` (write op)

This records an axiom and mints its id, `axm-<minted>`. It writes the record and gives the axiom
its heading in `axioms.md`, `### Axiom <label>: <title><!-- kbase-id: <id> -->`
([§1.1](../../1-model/1.1-nodes.md)). Where one unmarked heading of exactly that text stands, it
gets the marker. Otherwise the heading is appended. The add never leaves two headings of one
text: a second heading would take the `-1` anchor, and the author's prose and any links would
stay with the first. A record without its marked heading is a `verify` fault, so the heading is
part of the smallest valid add. The axiom's prose under the heading is the author's. `edit-axiom`
rewrites the heading on a label or title change. `remove-node` removes the heading with the
record and leaves the prose under it to the author.

- **Keys**: `label` (one line, any text, required), `title` (required).
- **Result**: `id`, minted; `heading`, `marked` or `appended`.
- **Refusals**: an axiom of that label or that title that already stands ([§2.10](../../2-conventions/2.10-existence.md)); more than one
  unmarked heading of that text in `axioms.md`, naming their lines, since kbase does not guess
  which one the axiom is; a prose value holding a line break ([§2.2](../../2-conventions/2.2-write-batches.md), Values).
