## 14. Limits

**Pattern:** Known-limitations section.

Where the format and this implementation stop. A limit whose mechanism already states it is not
repeated here: the zero-value default is defined in [§11](../11-package-map/the-zero-value-hazard.md), and the in-memory
index's lack of statement text in [§6](../6-kb-model-and-maintenance/the-query-layer.md).

**A value records whose call wrote it and what it was judged against, and nothing more.**
`manually-scored` says whether a person's call or a machine's wrote a rigor or a bearing, and
`judged-hash` what it was last judged against (SPEC §10 Provenance and the judged hash): no author,
time or reason. A work's rigor and a `founds` bearing, which no run writes, have neither. A
`*pending*` the caller wrote and one written because it left the value out differ only in that flag.
A value converted from `2.0.x` is marked model-scored whoever judged it, and nothing records that it
was converted.

**A `founds` bearing on a work names its finding in prose.** The `context` on the record says which
theorem, table or result the bearing judges, and no id, range or hash joins the two, so no check can
prove the judgement was about that finding. An axiom's `founds` edge needs no such prose: its source
is the axiom's own id.

Contents:

- [§14 A conversion records no history](a-conversion-records-no-history.md)
- [§14 Merged metadata is refused, not merged](merged-metadata-is-refused-not-merged.md)
- [§14 No format sits between two majors](no-format-sits-between-two-majors.md)
- [§14 No refresh moves a judgement](no-refresh-moves-a-judgement.md)
