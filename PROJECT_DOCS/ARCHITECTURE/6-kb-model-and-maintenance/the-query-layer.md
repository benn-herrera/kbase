### The query layer

`query.EdgesPayload` is the one incidence adapter: it selects the index's edge rows by relation and,
where `EdgesSelection` names a node, by `Direction` (`in` the rows whose target it is, `out` those whose
source it is, else both), and presents each as `query.EdgeRowRecord` — `source`, `target`, `relation`,
`bearing` (the index's number-or-literal text, null only where the pairing carries none), `context`,
`manually_scored` and `replaced_relation` — in the fixed relation order, then by source and target
(SPEC §8). Its answer is the whole selection: no list query slices an answer, and `find` and `trace`
cut only where a `--limit` stands beside a `--sort`. No query verb returns scoring text: the index
carries no statement body, so no passage ever becomes a query answer.
