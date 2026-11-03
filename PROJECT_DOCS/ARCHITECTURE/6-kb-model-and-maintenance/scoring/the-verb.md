#### The verb

`runScore` is the body, and `cmd/score.go` keeps the role, the ask, the locks, the start refresh
and the resume call: `scoreUsage`'s checks of the scope arguments together and of `--edge`'s form;
the KB root; the state directory; for `--resume`, `score.ResumeTarget` and the pass and
`rescore_manual` its header records; the scoring role resolved before the KB is opened —
`scoringRole`, through `config.Resolve`, with `--scoring-model` supplying or overriding the model,
and skipped under `--dry-run` — so a setup refusal writes nothing and mints no run; `openKB` and the
migration record; `score.RunID`; the run's locks through `build.Hold`, before any record of the run
is written; one refresh through `holdWriting`, skipped under `--dry-run`, which releases the write
lock before the walk; then the launch, `scoringLaunch`, whose client the registry opens at the
first item launched, so a run refused at its scope or holding no item dials nothing, and where a
key-file refusal ends the run; then `score.Invoke`, given that launch; `roles` where the report's
`Asked` says the walk was handed it, then `run`, `ledger`, `counts`, a dry run's `ask` and `skip`,
and `scored`; `resume` (`scoreResume`) where an item failed or was not reached, ahead of the record
the outcome closes on; and last, the report's `failures` on `failed`, or its `cancelled` on
`cancelled`. The landings derive inside their batch, so no refresh follows the walk. The closing
records and `resume`'s reach are SPEC §8's. Registration is `readWriteTool(scoreCmd)` in
`cmd/surface.go` (SPEC §11): the command is bound and appears in `tools/list`, but the MCP server
starts it as a detached child (`mcp.ScoreLaunch`) — as it starts `build` — returning the run's
identity and a `wait` command instead of holding the turn for the walk's duration, a run still
starting after 5 s as `state: starting`. `--edge` is declared a mapping flag (`mcp.MappingFlag`):
over MCP an object of `source`, `target` and `relation`, joined into the `--edge` the child
takes.
