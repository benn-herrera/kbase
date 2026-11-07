### Dependency attribution (`depends-attributed`)

A captured link has no direction; only this stage's model call gives it one, or rules it a
mention. The candidates are every marked pair, directed as the text marks it, and every link of the
cull record ([§6](../6-build-state-ledger-and-monitoring/index.md)) whose two claims no marked pair joins either way round, directed from its later
claim in reading order to its earlier, an equation node always the target. Each candidate gets one
model call offering *supported by*, *in support of* and *mention*, less *in support of* where
containment directed the pair or its target is an equation node. *Supported by* writes a `supports`
edge from source to target, *in support of* one from target to source, and *mention* a `references`
edge. A candidate whose call yields no readable answer after the retry, and every candidate under
`--no-inference`, takes its deterministic default: *supported by* where containment directed the
pair, *mention* otherwise. The stage writes every edge its answers make, cycles included; last,
cycle breaking ([§10](../10-metadata-format-version/index.md)) demotes one edge of each cycle those edges close — the back edge a depth-first
search finds — each a warning in the stage's report, before the stage's rebuild of the derived
views and verify.

