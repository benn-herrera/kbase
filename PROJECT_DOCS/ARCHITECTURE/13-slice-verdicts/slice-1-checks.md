### Slice-1 checks

identical tree path set and child order; equal per-leaf word streams under the
markdown tokeniser; the Document-Tree conformance passes with matching counts
of up-link lines, three-attribute anchors, math fences, `data-cites` citation spans and labelled
blockquotes with identifiers; equal inventory; records carry every reader and placement fact of that
inventory; byte-identical `kb-root/` and records across two runs. All pass: the writer route is
pandoc's. Any fails: execution stops and the owner rules (fix, amend the check, or a Go writer).

**Slice-2** applies the same checks to four more papers, and requires two negative controls to
refuse as SPEC §5 specifies (naming the paper and pandoc's error; naming each unloaded file and the
line that named it). **Slice-3** records a census, with no pass threshold.

A check that fails for a reason whose fix needs a third pre-pass, a filter, or a change to any of I1
to I7 is escalated to the owner.

---

