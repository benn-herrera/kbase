#### `verify` (tool + CLI)

This checks the KB and writes nothing.

- **Faults** (outcome `refused`, each an item under `refusals`):
  - the dead-link gate over every link under `kb-root/`, anchors included, citations included
    (`dead-link`): a fragment is live where it names a heading's anchor or an HTML element's
    `id` in the document it reaches, or, on a summary page, an entry's anchor; and an id in a
    citation's link title, or a node id in prose, that no node carries (`unknown-id`);
  - a bare node id in prose, outside code blocks, markers and citation link titles (`bare-id`;
    remedy: `add-citation` with the id as `snippet`, [§1.5](../../1-model/1.5-citations.md));
  - a frontmatter block on any document (`frontmatter`);
  - a KB in an older format (`index-freshness`);
  - a hosted node with no statement marker or more than one (`statement-marker`);
  - an axiom record with no marked heading in `axioms.md` or with more than one, a marked heading
    naming no axiom record, or a marked heading whose text is not its record's label and title
    (`axiom-heading`);
  - a summary page (a register page, [§1.6](../../1-model/1.6-derived-files.md)) that does not match what the derivation renders
    (`register-freshness`).
- **Remedies**:
  - `kbase update-format` for a KB in an older format;
  - an empty write batch ([§2.2](../../2-conventions/2.2-write-batches.md)) for a summary page that does not match the derivation: it
    re-derives every page;
  - for any other fault, the user's git work or prose edit, guided by the fault's detail.
- **Findings** (not faults): every demoted record (`check: demoted`).
- **Arguments**: none.
- **Result**: `findings`, then `refusals` where refused.
