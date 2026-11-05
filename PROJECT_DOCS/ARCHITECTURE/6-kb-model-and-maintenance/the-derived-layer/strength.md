#### Strength

`internal/index/strength.go` is the arithmetic's authority: the four relations, their three stations,
the pending rule and the value's two formats live there. [STRENGTH_SCORING_SCHEME.md](../../../SPEC/scoring/strength/entry-point.md)
states the relations, the formula and its worked examples and is cited, not reproduced.

| Station | Relations | Function |
|---|---|---|
| minimum | `supports`, `founds` | `MinDependencyStrength` |
| maximum | `lifts` | `liftStrength` |
| cap | `refutes` | `capStrength` |

`EdgeContribution` is one edge's value, nil where its bearing or its source's strength is pending. Each
station function reduces it over its edges, omitting an excluded one; `ExperimentTakesPart` excludes
an experiment that is not `run`. `derive` composes one node's result. `graphStrength` fills a
`strengthWalk` from the records and `run` computes each claim into `Strength.Results` and each
analysis into `Strength.AnalysisResults`, in `derivationOrder`, which is Kahn's order: a node after
every node its premises name, from sources to sinks. Nodes a cycle keeps out of that order, and nodes
resting on one, derive nothing; the load refuses such records ([§6](../cycle-breaking.md)).
`RenderStrengthTrace` composes the station text for a trace. The `trace` query (SPEC §8 `trace`)
reports `EdgeContribution` per edge, `internal/query` reading each source's strength off `Derive`'s
node rows and leaving an experiment that is not `run` out as the propagation does.

The propagation computes at full precision, so a dependent multiplies its source's exact strength, and
nothing else rounds a strength. `FormatStrengthRecord` writes the value in the fewest decimals that
read back as it (a summary page's strength line and premise annotation), and a query result contains
the value `FormatStrengthDisplay` formats, where a trace shows it.

