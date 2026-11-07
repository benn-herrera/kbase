#### Edits and renames

An edit (`edit-<kind>`) replaces the fields its entry gives, through `nodesOf`, which presents each
kind's records as their editable fields. Title uniqueness is judged by `titleHolder` and label
uniqueness by `labelHolder`, as in an add.

**Pattern:** Referential-integrity maintenance (rename refactoring). A title or label change
(`rename`) keeps every link into the node valid in the same batch. It reads each work's and axiom's
link target and every node's summary entry before and after the change, from `index.Derive` over a
source holding the batch's records and documents as earlier entries left them (`anchored`, one rebuild
each side). It then rewrites every document but the summary pages (`eachDocument`: `kb.CitingDocuments`
and each document an earlier entry created), each document it changes a `leafEdit`.

- `edit-axiom` rewrites its marked heading in `axioms.md` (`rewriteAxiomHeading`).
- A citation in the naming form takes the new title through `renameCitations`, which refuses at
  `title`, naming the citing `path:line`, a rewritten link that `readsAsOneLink` does not see whole.
- A work's new label replaces every use of the old key as a key (`rekey`, over
  `kb.CitationKeyRefs`), after `refuseKeyInUse` refuses a label that a citation span or a
  reference-list entry already uses.
- These rewrites change lines in place, so `movedHeadings` finds each heading whose anchor changed by
  comparing `kb.HeadingAnchors` line by line, before and after, and `movedAnchors.summaries` adds each
  summary entry whose anchor changed. `repoint` then gives every link outside code, citation or not,
  inline link or reference definition, one destination computed from the one it had before: the
  node's new target where a citation of it pointed at the old, otherwise the changed heading's or
  entry's new anchor, the fragment alone changing. The map is read once per link, so no link is
  updated twice and a shifted repeat suffix (`on-x-1` to `on-x`) is never chained.

