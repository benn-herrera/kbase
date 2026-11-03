### The derived layer

`internal/index/strength.go` is the arithmetic's authority: the four
relations, their three stations — the minimum for `supports` and `founds`, the maximum for `lifts`,
the cap for `refutes` — the pending rule and the value's two spellings live there and nowhere else.
The walk computes at full precision and nothing else rounds a strength: `FormatStrengthRecord`
writes the value in the fewest decimals that read back as it — a register page's strength line and
premise annotation — and a query result carries the value `FormatStrengthDisplay` spells, where a
trace spells it.

| Relation | Source kinds | Target kinds | Station | Contribution |
|---|---|---|---|---|
| `supports` | claim, experiment | claim, analysis | minimum | `bearing × source strength` |
| `founds` | work, axiom | claim, analysis | minimum | `bearing × the source's rigor`; `kb.AxiomRigor` is an axiom's `1.0` by rule |
| `lifts` | analysis, experiment | claim, analysis | maximum | `bearing × the lifter's strength` |
| `refutes` | claim, analysis, experiment | claim, analysis | cap | `1 − bearing × refuter strength`; the smallest cap wins |

A claim of rigor `0.85` with a claim premise at `0.4 × 0.90`, a work at `0.8 × 0.90`, a lift at
`0.7 × 0.90` and a refutation at `0.5 × 0.90` derives `min(0.85, 0.36, 0.72) = 0.36`, the lift takes
the final to `0.63`, and the smallest cap, `0.55`, takes it to `0.55`. `ComputeStrength` walks each
claim into `Strength.Results` and each analysis into `Strength.AnalysisResults`;
`EdgeContribution` is one edge's value in the table's Contribution column, nil where its bearing or
its source's strength is pending. `MinDependencyStrength` (the `[= min(…)]` trace's operand), the
largest lift and the smallest cap each fold it over their station's edges, omitting an excluded
one, and the `trace` query (SPEC §8) reports it per edge, `internal/query` reading each source's
strength off `Derive`'s node rows and leaving a planned experiment's edge out as the walk does;
`RenderStrengthTrace` composes the station the walk took.
[STRENGTH_SCORING_SCHEME.md](../../SPEC/scoring/strength/entry-point.md) states that arithmetic — the relations, the
formula and the worked example — and is cited, not reproduced.

