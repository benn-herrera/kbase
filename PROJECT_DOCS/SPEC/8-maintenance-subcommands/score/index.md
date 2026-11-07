### `score`

```
kbase score [--pass rigor|bearing|all] [--node ID] [--edge SOURCE,TARGET,RELATION]
            [--rescore-manual] [--scoring-model MODEL] [--dry-run] [--resume] [--state-dir DIR]
            [--timeout DURATION] [--ask-concurrency N]
```

`score` grades the KB's rigor and bearing values by calling the scoring model: one instruction per item, one value per item, and a refused reply asked again once with a correction ([A refused reply: one re-ask, then failed](a-refused-reply-one-re-ask-then-failed.md)). It computes each value from the faults the reply names, writes it through the edit ops ([Writing the results](writing-the-results.md)) and records every attempt in a ledger.

**Scope.**
- By default a run covers every stale item that is not manually scored, in both passes. What makes an item stale is the judged hash ([§10](../../10-metadata-format-version/provenance-and-the-judged-hash.md)), so a second run over a KB it has scored makes no model calls.
- The rigor pass covers claims, analyses and experiments. The bearing pass covers `supports`, `lifts` and `refutes` edges. No pass reaches a work's rigor (judged by hand), an axiom's rigor (`1.0` by rule) or a `founds` bearing.

**Arguments**, all optional:
- `--pass`: `rigor`, `bearing` or `all`, the default.
- `--node`: a claim, analysis or experiment id. Scores that node's rigor, stale or not.
- `--edge`: scores that edge's bearing, stale or not. At the command line it names its edge as `<source>,<target>,<relation>` (ids and relations hold no comma); over MCP `edge` is a mapping of the three.
- `--rescore-manual`: a bool. In a full run it also scores every manually scored item of the pass, stale or not; an item that is neither stale nor manually scored is still skipped. It is required to name a manually scored item with `--node` or `--edge` ([Skipped items](skipped-items.md)).
- `--dry-run`: a bool; lists what a run would score and writes no KB file ([The closing record](the-closing-record.md)).
- `--resume`: a bool. Continues the repository's most recent score run, with the scope it recorded, `rescore_manual` included, keeping every value that run wrote; it takes none of `--pass`, `--node`, `--edge` or `--rescore-manual` beside it ([The ledger and resume](the-ledger-and-resume.md)).
- `--scoring-model`: a `<provider>/<model>` reference for this run ([The scoring role](the-scoring-role.md)).
- `--ask-concurrency`: under 1 it is refused as `build`'s is ([§5](../../5-build-behaviour/index.md)).
- `--timeout`: a duration, the deadline for each scoring call (default `30s`), counted from the call's start, never while the call waits for its turn.
- `--state-dir`: the state store ([The ledger and resume](the-ledger-and-resume.md)).

`--resume` takes neither `--ask-concurrency` nor `--timeout`: each shapes one invocation, not the run. `score` has no `--provider`: a scoring-time endpoint is named by `--scoring-model`, which states the provider and the model in one value, and passing `--provider` is a usage refusal. Every refusal is listed under [Outcomes](outcomes.md).

A run makes one call per item, two for an item whose first reply is refused, and takes minutes. At a terminal `kbase score` blocks until the item loop ends. As an MCP tool ([§11](../../11-the-mcp-server/tools.md)) it starts a detached child and returns once the run has started, while `kbase wait` reports its outcome when it ends. A shell caller gets the blocking behaviour a terminal gets.

There is no score batch and no item limit: the scope the caller names is the budget. `kbase cancel` stops a run, `--resume` continues it, and what the run wrote stays.

Contents:

- [§8 The two passes](the-two-passes.md)
- [§8 Skipped items](skipped-items.md)
- [§8 One item, one quantity, one call](one-item-one-quantity-one-call.md)
- [§8 The answer](the-answer.md)
- [§8 The value](the-value.md)
- [§8 A refused reply: one re-ask, then failed](a-refused-reply-one-re-ask-then-failed.md)
- [§8 A provider call's failure names what ended it](a-provider-call-s-failure-names-what-ended-it.md)
- [§8 Writing the results](writing-the-results.md)
- [§8 The scoring role](the-scoring-role.md)
- [§8 One call per ask](one-call-per-ask.md)
- [§8 The ledger and resume](the-ledger-and-resume.md)
- [§8 `ask_sha256` records the ask set and gates nothing](ask-sha256-records-the-ask-set-and-gates-nothing.md)
- [§8 The run's locks](the-run-s-locks.md)
- [§8 Refresh and format](refresh-and-format.md)
- [§8 Outcomes](outcomes.md)
- [§8 The closing record](the-closing-record.md)
