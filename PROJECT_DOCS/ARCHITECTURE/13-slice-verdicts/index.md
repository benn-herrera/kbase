## 13. Slice verdicts

**Pattern:** Spike/prototype decision record.

The reader's design was settled by a slice: `kbase build --through document-graph` over six corpus
papers and a count report of all fifty, judged by comparing kbase's tree against the reference tree
for the same paper, with kbase's own readers. The hypotheses and their verdicts:

| Hypothesis | Confirmed by | Refuted by | Verdict |
|---|---|---|---|
| No filter layer is needed | The conformance and inventory checks pass on kbase's tree; per-leaf word streams match; every citation state the reference trees exhibit appears in kbase's | A load-bearing form or word no JSON transform can produce | **Confirmed** on the six Slice-1 and Slice-2 papers; 49 of 50 corpus papers build |
| Records suffice for the inventory | Every reader and placement fact of the inventory ([§5.5](../5-reader-and-transforms/5.5-records.md)) is carried by kbase's records, item by item in scan order; every record joins exactly one document | A reader or placement fact that exists only in rendered bytes | **Confirmed**, with the visibility rule of [§5.5](../5-reader-and-transforms/5.5-records.md): a block record exists only where the page has a label line the reader reads |
| The preceding text run suffices | Withdrawn: the word before a reference is defined over the written page, so it is a page-defined reading ([§5.5](../5-reader-and-transforms/5.5-records.md)), not a record | — | Withdrawn |
| pandoc's writer suffices | Every Slice-1 check passes under it | A load-bearing form it cannot emit | **Confirmed**; the writer route is pandoc's gfm writer with `--wrap=none` ([§5.4](../5-reader-and-transforms/5.4-writer-route.md)) |

Contents:

- [§13 Slice-1 checks](slice-1-checks.md)
