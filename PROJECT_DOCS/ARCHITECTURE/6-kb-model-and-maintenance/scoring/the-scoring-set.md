#### The scoring set

**Pattern:** Input-hash staleness, as in make.

`query.ScoringSet(ix, pass)` reads the in-memory index in-package and returns `[]query.Item`, each
holding a `ScoringNode` or a `ScoringEdge` and never both, in the loop's order. `query.PassRigor`,
`query.PassBearing` and `query.PassAll` are the three values the set and the `--pass` flag share
(`query.Passes`); `PassAll` is the node set and then the edge set, each in its own order. An item
contains identity only, so an edge's row comes from `Index.Edge` and no caller builds a `kb.EdgeRow` by
hand. The set has no verb: it is consumed in-process, and `--dry-run` is the inspection surface.
Membership: SPEC §8 The two passes.

`internal/score/scope.go` narrows the set to a run's scope. `scopeOf` returns the one named item, or
else each item of the pass that is stale or, under `--rescore-manual`, manually scored. `namedNode` and
`namedEdge` refuse an id or an edge that does not exist or that no pass asks; `namedItem` refuses a
manually scored item without `--rescore-manual`. `dirty` compares the hash each record carries
(`recordedHashes`, read once through `kb.LoadGraph`) with `judgedHash`, the hash of what the item would
be judged against now: `asks.NodeJudgedHash` over a node's statement, `asks.EdgeJudgedHash` over its two
ends' statements, each the one the run's `graph` read through `kb.Statement`. The judged hash is
defined in SPEC §10 Provenance and the judged hash. `asks.JudgedHash` hashes the digest of the
template the item is judged under, then each statement behind its length; the model is no part of it.
`RigorAsk` is the single mapping from node kind to template, which the hash and the call both read.

A resume takes no scope: its set is the run's `items.jsonl` (`readItems`). `ResumeTarget` picks the run
a `--resume` continues, the newest `run-` directory by its header's start (`newestRun`, filtered to
`RunKindReal`, so a `dry-` run is never one). It refuses where none exists, where `ReadRun` counts the
run finished, and where a build invocation in the store's progress record (`buildstore.ReadState`)
started after it.

**Pattern:** Filtered work set. `Runner.Skip` is `score.ManualSkip(ix, rescore)`, built from the run's
`--rescore-manual` and each item's `manually-scored` flag (`query.Index.ManuallyScored`). It says which
items the loop records `skipped` (status `score.StatusSkipped`, reason `manually-scored`; no call, no
capture). `score.Resume` takes the same predicate, so `Ledger.reprice` leaves a skipped item's written
line as it is. The loop also records `skipped`, once, an item that is not in the in-memory index it
loaded (`inKB`), and `attempt.fail` does the same for a write's `errItemGone`, each with that error's text as
the reason (SPEC §8 Skipped items).

