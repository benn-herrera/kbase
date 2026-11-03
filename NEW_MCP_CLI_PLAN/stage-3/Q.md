### Q — `score` (expert)

**Design:**
- [§2.3 The derivation after every write](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md) — landings are followed by it
- [§2.9 Manually scored, and the judged hash](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md) — the mark and the dirty check
- [§3.5 Runs and run control](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/index.md) — detaching, the operation lock, the state store
- [§3.5 `score`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — MCP and CLI notes: `score`'s notes

Relies on stage 1: `judged-hash` on every scored record, all zeros from the conversion. Relies on
stage 2: the edit ops, through which landings go (`edit-claim`, `edit-analysis`, `edit-experiment`
and `edit-edge`, each in its own `cmd/<op>.go`).

**Scope:**
- `cmd/score.go` (`scoreOptions`, flags);
- `cmd/score_status.go`;
- `internal/score/` (`invoke.go`, `run.go` `Runner`, `ManualSkip`, `RescoreManual`; `status.go`);
- `internal/query/scoring.go` (`ScoringSet`, `PassBoth`);
- the judged hash's one function, in its chosen home;
- `internal/write/` (the manual sets record the hash; `Options.NoRefresh`);
- `internal/mcp/launch.go` (`ScoreLaunch`; `state: starting` after 5 s);
- the scripts that run `score` with a flag this package removes (Facts).

**Facts acted on:**
- A node's statement has one reader, page §1.4's, exported by package N (stage 2) for
  `add-citation`'s quote check (`git grep -n 'func Statement' -- internal/kb internal/index`).
  `score`'s own `statementPassages`/`claimPassages` (`internal/score/text.go`) read a claim's block
  instead; the judged hash and the ask read the statement through N's reader, and score's copy goes.
- [Page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md) `score`:
  - scope by dirtiness;
  - `pass` is `rigor` | `bearing` | `all`;
  - `node` and `edge` (over MCP a mapping; at the CLI `--edge <source>,<target>,<relation>`);
  - `rescore-manual` and `resume` are bools;
  - the dry-run listing;
  - no limit: "The scope the caller names is the budget";
  - `pass` beside `node` or `edge` is refused;
  - the result, the MCP result and the refusals.
- [Page §2.9](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md): the judged hash; dirty when it differs; all zeros never matches.
- Today:
  - `--limit` exists (`cmd/score.go:582`);
  - `--resume` takes a run id (`:586`);
  - `--rescore-manual` takes a pass (`:587`, `score.RescoreNone`);
  - `PassBoth = "both"` (`internal/query/scoring.go:17`).
- Landings go through the edit ops, with the mark `false` ([page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)). Today they go through
  `set-rigor` and `set-bearing` (`internal/score/invoke.go:251–252`, `:316`).
- The landings' derivation cadence is the coder's choice. The derived files are current whenever
  the run exits, finished, failed or cancelled (see "Decisions the record settles"). Today a
  landing writes with `NoRefresh: true` (`internal/score/invoke.go:316`), the run refreshes at its
  start (`cmd/score.go:216–222`) and after its last write (`:262–275`). If the scoring run stops
  setting `write.Options.NoRefresh`, the build is the field's last caller, and package R removes
  the field.
- A landing today is two writes: `landBatch` (`internal/score/invoke.go`) runs `set-rigor`, then
  `set-bearing`, each its own `write.Run`. A mixed `write.Batch` (stage 2) lands both, with their
  derivation, as one write.
- The [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) notes this package writes for `score` (Rule 12): the MCP note says the call returns
  once the run has started, as a detached process, with `pid`, `state-dir` and `wait`, and that
  `status` follows it and `cancel` stops it; the CLI note says it runs in the foreground and
  SIGINT or SIGTERM (`kbase cancel`) stops it resumably.
- `git grep -l 'scoreOptions' -- '*.go'` →
  - `cmd/{askconcurrency_test,concurrentasks_test,idleconns_test,provider_test,refusedkey_test}.go`;
  - `cmd/{score,score_record_test,score_replay,score_replay_test,score_scoring_test,score_status_test,score_test}.go`.
- `git grep -l 'ScoringSet\|ManualSkip\|RescoreNone' -- '*.go'` →
  - `cmd/score.go`;
  - `internal/query/{scoring,scoring_test}.go`;
  - `internal/score/{invoke,invoke_test,replay_test,run,run_test}.go`.
- The scripts: `git grep -n -- '--limit' -- tools/scripts justfile` →
  - `tools/scripts/test-integration-results-fixture.sh`: `:119`, `:131` and `:142` run
    `score --pass rigor --limit 1`, each expecting `bounded`, their comments (`:117`, `:129`,
    `:135`) saying the limit bounds the run to one item. `:186` and `:187` are `find`'s
    (package O).
  - `tools/scripts/test-integration-build-live-arxiv.sh` (LIVE): `:287` runs
    `score --pass rigor --limit 1`, and its evidence line `:315` says "one item asked".
  - `tools/scripts/test-integration-parity-adjagent-sheet.sh`, whose `--limit` is the queries'
    (packages O and S), not `score`'s.
- What those legs leave true: each still asks exactly one item. The bound is [page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s `node`:
  each runs `score --node <a claim id the leg's KB holds>` with no `--pass`, since `pass` beside
  `node` is refused. Each leg's expected outcome becomes the one a run whose one item failed ends
  with, and its comment says `node` bounds it. The live leg's evidence line keeps "one item asked"
  and names `--node`.
- The development replay (`cmd/score_replay.go`, tag `kbasedev`) keeps its own `--limit`
  (`scoreReplayFlags`, `:41`). The page governs the shipped surface, and the replay does not ship
  (`just test-dist-no-replay`).

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): `### score` citing [page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md) and [§2.9](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md).
- [SPEC §6](../../PROJECT_DOCS/SPEC/6-build-state-ledger-and-monitoring/index.md): score status.
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.
- [ARCHITECTURE §6 "Scoring"](../../PROJECT_DOCS/ARCHITECTURE/6-kb-model-and-maintenance/scoring/index.md) and [§11](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/index.md): the hash function's home.

**Tests:**
- The dirty set over a KB:
  - an edit to a node's statement makes it dirty;
  - an edit elsewhere in the leaf does not;
  - a new template digest makes every item dirty;
  - all zeros are dirty.
- `--node` on a work is refused.
- `--edge` on `founds` is refused.
- `--pass` beside `--node` is refused.
- A manually scored item without `--rescore-manual` is refused.
- `--resume` after a newer run is refused.
- The dry run lists the dirty manual values.
- A manual `edit-claim` records the current hash.
- A run cancelled after a landing leaves derived files that match the records.

**Done-line** (after Rule 13's two): `just test-integration`,
`just --dry-run test-integration-build-live-arxiv`.
