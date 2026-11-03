#### The scoring set

`query.ScoringSet(ix, pass)` reads the loaded index in-package and returns
`[]query.Item`, each carrying a `ScoringNode` or a `ScoringEdge` and never both, in the walk's order;
`query.PassRigor`, `query.PassBearing` and `query.PassAll` are the three spellings the set and the
`--pass` flag share (`query.Passes`), `PassAll` the node set and then the edge set, each in its own
order. An item carries identity only, so an edge's row comes from `Index.Edge` and no caller builds
a `kb.EdgeRow` by hand. The set has no verb: it is consumed in-process, and `--dry-run` is the
inspection surface. Which nodes and edges are in it, and which are excluded, is SPEC §8's.

`internal/score/scope.go` narrows it to a run's scope. `scopeOf` is the one named item —
`namedNode` and `namedEdge` refusing an id or an edge that does not stand, one no pass asks, and,
through `namedItem`, a manually scored item without `--rescore-manual` — else each item of the
pass that `dirty` says is dirty or, under `--rescore-manual`, is manually scored. `dirty` compares
the hash each record carries (`recordedHashes`, read once through `kb.LoadGraph`) with
`judgedHash`, the hash of what the item would be judged against now: `asks.NodeJudgedHash` over a
node's statement, `asks.EdgeJudgedHash` over its two ends', each statement the one the run's
`graph` read through `kb.Statement`. A resume takes no scope: its set is the run's `items.jsonl`
(`readItems`). `ResumeTarget` picks the run a `--resume` continues — the newest `run-` directory by
its header's start (`newestRun`) — and refuses where none stands, where `ReadRun` counts that run
finished, and where a build invocation in the store's progress record (`buildstore.ReadState`)
started after it.

The skip comes from the KB the run loaded too: `Runner.Skip`, `score.ManualSkip(ix, rescore)` over
the run's `--rescore-manual` and each item's `manually-scored` flag (`query.Index.ManuallyScored`),
says which items the walk records `skipped` — status `score.StatusSkipped`, reason
`manually-scored`, no call and no capture — and `score.Resume` takes the same predicate, so
`Ledger.reprice` leaves a skipped item's landed line as it stands. The walk also records `skipped`,
once, an item that is no longer in the index it loaded (`inKB`), and `attempt.fail` records a
landing's `errItemGone` value the same way, each with that error's text as the reason.

Two predicates in `internal/kb` answer whether an edge carries a bearing, and they are not the same
question. `kb.Scoring` — the four relations in `kb.ScoringRelations` — is whether an edge carries a
bearing at all, and every reader of a stored bearing uses it: the values' relation check, the strength
walk, the records, `edges`, a node's payload, the index's load and the bearing text. `kb.BearingWalk` —
the three in `kb.BearingWalkRelations` — is whether a score run asks for that bearing, and the bearing
pass is its only reader: a `founds` edge carries a bearing and is never an item, its source lying
outside the corpus, so `edit-edge` alone writes it.
