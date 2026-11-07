#### Removal and move

`remove-node` reads the rows of the edges it removes from `derived` (`index.Derive` over the batch's
records and documents, which `targetOf` shares), drops them through `dropEdges`, and takes the record
and the marker out through `dropNode` inside `keepSummaryLinks`, or an axiom's headings out of
`axioms.md` through `dropAxiomHeadings`, which updates references to every later heading whose repeat
suffix the removal lowered. It then refuses, through `refuseMentions`, a mention that outlives the
node:

- a citation in any document but the summary pages (`eachDocument`, shared with `renameCitations`,
  `rekey` and `repoint`);
- the node's id standing bare in any such document, one per line holding it (`kb.TextBareIDs`, the
  reader `verify` and `cite-bare-ids` share);
- for a work, a citation span with its label as the key where no reference-list entry has that key;
- a surviving record's prose field (`graphProse`, `Mentions`).

`move-node` checks the named line through `checkNamedLine`, places the marker with `appendMarker`,
takes every other occurrence out (`unmarkEverywhere`, keeping the named line), and updates every
citation, through `repoint`, to `targetOf`: a hosted node's host and its marker's section anchor
(`index.SectionSlug`, the slug the rebuild of the derived views uses).

A document the batch edits is a leaf as `index.Derive` reads it, by its child list (`leafAt`), so a
citation added to a leaf leaves it a leaf.

