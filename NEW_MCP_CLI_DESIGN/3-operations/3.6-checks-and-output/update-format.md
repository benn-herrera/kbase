#### `update-format` (tool + CLI)

This converts a KB to the current format, run alone, so that any problem is purely the
conversion's and restoring `kb-root/` and any moved build record from `HEAD` reverses it. Every run re-derives every derived file
([§1.6](../../1-model/1.6-derived-files.md)) and rewrites the maintained blocks ([§1.7](../../1-model/1.7-maintained-blocks.md)), whether or not the format changed. A conversion
to `4.0.0` ([§1.8](../../1-model/1.8-format-version.md)) does exactly this:
- rewrites every statement marker to `kbase-id` form, attached with no space;
- moves an analysis marker that sits alone on the line directly after a line ending in the marker
  of a claim it lifts to directly after that claim's marker, and removes the emptied line. Nothing
  else moves.
- writes `kb-summary.md` pages;
- adds the node id as the link title of each citation whose anchor resolves to a node's statement,
  and reports each one it cannot resolve without guessing;
- rewrites every link into a heading, citations and ordinary links alike, to the heading's anchor
  under [§1.4](../../1-model/1.4-markers.md)'s slugging, and points a work's citation at its root summary entry ([§1.5](../../1-model/1.5-citations.md)), so the
  converted KB passes `verify`'s dead-link gate;
- converts each bare node id in prose, of every kind, into the naming form ([§1.5](../../1-model/1.5-citations.md)), listing each
  conversion (file, line, id). This happens only in this conversion; a bare id in a KB already at
  `4.0.0` is a `verify` fault for the author to fix.
- mints an id for every work and axiom, derived from its `3.0.0` id alone
  ([§2.6](../../2-conventions/2.6-format-conversion.md)). The old key or label becomes the
  record's `label`. Every place the old id stood now names the new id: edge and demoted record
  ends, record prose fields, the link titles this conversion adds, the bare ids it converts, and
  every `3.0.0` spelling it reads as naming a work or an axiom. It ends each
  `### Axiom <label>: <title>` heading in `axioms.md` with its axiom's marker. It reports each
  heading that names no axiom record, which the conversion cannot pair without guessing;
- rewrites an experiment's `status: pending` as `status: planned`;
- drops the `no-edge` field from claims and analyses. It does not split the text into edges, since
  one string may explain several named nodes; it reports each node's text, for the author to recast
  as `references` edges with `add-edge`;
- drops the `strengthen-by` field from claims, reporting each node's text;
- writes `judged-hash` as all zeros on every scored record, claiming nothing about whether a value
  is current: the first `score` after the conversion finds every value dirty;
- moves each build record standing at the repository root into `kb-root/.build/` with `git mv`.
  The records are kbase's own; only their place changes.

With `commit`, it commits the conversion as one commit, label `kb-write: update-format <from> →
<to>`. Its clean-tree precondition makes that commit exactly the conversion. As with a batch,
`commit` is opt-in.

- **Arguments**: `commit` (bool, optional).
- **Result**: `migration` (where the format changed: `from`, `to`, `note`, `warnings`, the
  unresolved citation links among the warnings), `written`, `removed`, `unfenced` (maintained-block
  documents without the marker pair), `sheet`, `dot`, `commits` (as a write batch's, [§2.2](../../2-conventions/2.2-write-batches.md); `[]`
  without `commit`), and, where the format changed:
  - `ids`, each work's and axiom's `from` (its `3.0.0` id) and `to` (its minted id);
  - `bare-ids`, each conversion's `path`, `line` and `id` (the minted id);
  - `no-edge`, each dropped field's node `id` and `text`;
  - `strengthen-by`, each dropped field's node `id` and `text`.
- **Refusals**:
  - `kb-root/`, or a build record the conversion would move, not clean (`check: dirty-paths`,
    naming the paths): remedy, commit or discard them;
  - a lock;
  - a KB newer than kbase;
  - a load error;
  - a conversion that cannot carry a file's content (`check: conversion`, naming file and line).
