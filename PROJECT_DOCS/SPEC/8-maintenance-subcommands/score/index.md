### `score`

```
kbase score [--pass rigor|bearing|all] [--node ID] [--edge SOURCE,TARGET,RELATION]
            [--rescore-manual] [--scoring-model MODEL] [--dry-run] [--resume] [--state-dir DIR]
            [--timeout DURATION] [--ask-concurrency N]
```

Its scope — every dirty item that is not manually scored, by default — its arguments, its dry run,
its result, its MCP result and its refusals are [page §3.5 `score`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s; what makes an item dirty,
the judged hash, is [page §2.9](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)'s. At the command line `--edge` names its edge as
`<source>,<target>,<relation>`; over MCP `edge` is a mapping of the three. `--node` and `--edge`
on one invocation together, and an `--edge` that is not three non-empty comma-separated parts, are
usage refusals. `--ask-concurrency` under 1 is refused as `build`'s is ([§5](../../5-build-behaviour/index.md)); `score`'s `resume`
does not carry it, nor `--timeout`: each shapes one invocation, not the run.

Grades the KB's rigor and bearing values by asking the scoring model: one call per item, one value per
call, each value landed through the edit ops and recorded in a ledger. `--timeout` is the deadline for one item's
call (default `30s`), counted from the call's start, never while the call waits for its turn. A run asks one call
per item and takes minutes: at a terminal `kbase score` blocks until the walk ends, and as an MCP tool
([§11](../../11-the-mcp-server.md)) it starts a detached child and returns once the run has started, while `kbase wait`
reports its outcome when it ends. A shell caller gets the blocking behaviour a terminal gets.
There is no `--provider`: a scoring-time endpoint is named by `--scoring-model`, which states the
provider and the model in one value, and passing `--provider` is a usage refusal.

Contents:

- [§8 The two passes](the-two-passes.md)
- [§8 Skipped items](skipped-items.md)
- [§8 One item, one quantity, one call](one-item-one-quantity-one-call.md)
- [§8 The answer](the-answer.md)
- [§8 The value](the-value.md)
- [§8 A refused reply is a failed attempt](a-refused-reply-is-a-failed-attempt.md)
- [§8 A provider call's failure names what ended it](a-provider-call-s-failure-names-what-ended-it.md)
- [§8 Landing](landing.md)
- [§8 The scoring role](the-scoring-role.md)
- [§8 One item, one call](one-item-one-call.md)
- [§8 The ledger and resume](the-ledger-and-resume.md)
- [§8 `ask_sha256` records the ask set and gates nothing](ask-sha256-records-the-ask-set-and-gates-nothing.md)
- [§8 The run's locks](the-run-s-locks.md)
- [§8 Refresh and format](refresh-and-format.md)
- [§8 Outcomes](outcomes.md)
- [§8 The closing record](the-closing-record.md)
