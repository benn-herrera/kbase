#### The verb

**Pattern:** Command orchestration; detached background job.

`runScore` calls `scoreRun`, the body. `cmd/score.go` keeps the role, the `Ask`, the locks, the start
rebuild and the resume call, in this order:

1. `scoreUsage`'s checks of the scope arguments together and of `--edge`'s form, then the
   `--ask-concurrency` check.
2. The KB root and the state directory; for `--resume`, `score.ResumeTarget` and the pass and
   `rescore_manual` its header records.
3. The scoring role, resolved before the KB is opened: `scoringRole`, through `config.Resolve`, with
   `--scoring-model` supplying or overriding the model. It is skipped under `--dry-run`, so a setup
   refusal writes nothing and mints no run.
4. `openKB` and the `migration` record; the load-error refusal; the pending-conversion refusal
   (`index.PendingConversion`).
5. `score.RunID`, then the run's locks through `build.Hold` ([§8](../../8-ledger-state-store-and-monitoring/locks.md)), before any record of the run
   is written.
6. One rebuild of the derived views through `holdWriting`, skipped under `--dry-run`, which releases the
   KB write lock before the loop.
7. The `Launch`, `scoringLaunch`, then `score.Invoke` given that `Launch`.

The result is assembled in this order: `roles` where the report's `Asked` says the loop was
handed the `Launch`, then `run`, `ledger`, `counts`, a dry run's `ask` and `skip`, and `scored`;
`resume` (`scoreResume`) where an item failed or was not reached, ahead of the final record; and last
the report's `failures` on `failed`, or its `cancelled` on `cancelled`. The batch that writes the
results rebuilds the derived views, so no rebuild follows the loop. `persistScoreResult` writes the
stored result beside the ledger (`score.WriteStoredResult`) before the locks are released, so `wait` reads
the result the verb emitted. The records and `resume`'s reach: SPEC §8 Outcomes and The closing record.

Registration is `readWriteTool(scoreCmd)` in `cmd/surface.go`: the command is exposed as an MCP tool,
and the server starts it as a detached child (`mcp.ScoreLaunch`), as it starts `build`, returning the
run's identity and a `wait` command instead of holding the turn for the loop's duration. The tool
table and the detached child are [§12](../../12-constants/mcp-server-spec-11/index.md); the start wait is `launchStarting`
([§12](../../12-constants/index.md)). `--edge` is declared a mapping flag (`mcp.MappingFlag`): over MCP an object of
`source`, `target` and `relation`, joined into the `--edge` the child takes.

