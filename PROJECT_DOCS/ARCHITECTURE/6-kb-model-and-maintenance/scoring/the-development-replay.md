#### The development replay

**Pattern:** Offline prompt-regression harness.

`kbase score --replay RUN` repeats a run's captured model calls under the binary's embedded templates
and reports what the runtime would have done, so a change to a scoring template is `edit → just build →
replay` over the items that failed. SPEC states nothing of it. Its code and tests are
`cmd/score_replay.go` and `internal/score/replay.go` with their test files, each carrying
`//go:build kbasedev`. `cmd/score.go` reaches it only through the package-level `scoreReplay` hook, which
the tagged file's `init` sets and which is nil in the default build. That `init` also declares `--replay`,
`--failed` and `--limit` (a run takes no limit, so `--limit` is the replay's alone), each annotated
`kbase.mcp` = `excluded` through `mcp.ExcludeFlag` ([§12](../../12-constants/mcp-server-spec-11/index.md)), so dev and release builds
serve one tool table.

The default build is the release shape and excludes the replay's code. Dev builds pass `-tags kbasedev`,
as `just build`, `just test` and `just test-race` do. What the replay reads is not dev-only: a run's
ledger and captures are written by the untagged `internal/score/ledger.go`, so a replay in a dev build
reads the records a released kbase wrote. `edit-gate` vets both build tags, and `test-dist-no-replay`
runs `TestDistBuildHasNoReplay` (`cmd/dist_test.go`) on the untagged build, which asserts the three flags
absent and the hook nil, and the same of the build's development replay ([§8](../../8-ledger-state-store-and-monitoring/index.md)).

`scoreReplayRun` takes `--pass`, `--failed`, `--limit`, `--scoring-model`, `--state-dir`, `--timeout` and
`--ask-concurrency` beside `--replay`, and the global `--config-dir`. Any other score flag with it is a
usage refusal naming `--replay`; `--failed` or `--limit` without `--replay` is one naming it; and a
`--replay` value outside `^(run|dry)-<id>$` (`replayRunRE`: the alphabet and length `kb.MintID` mints)
is one naming `--replay`. The replay opens no KB and takes no KB write lock or run lock; a run in
progress may be replayed. With `--state-dir` it needs no KB root and works from any directory. Without
it, it finds the store through the kb-root at or above the working directory and is refused, `check:
usage`, `key: --state-dir`, where there is none. Both development replays, this one and the build's,
resolve their store through `replayStateDir` in `cmd/replay.go`, beside `replayTakes`, which names the
flags a replay takes in its refusal. A relative `--state-dir` resolves against the project root where
the working directory has one, and against the working directory where it has none.

`score.OpenReplay` selects every item with a capture, in the order of the run's `items.jsonl`, each by
its last capture. `--pass rigor` or `--pass bearing` narrows the selection to that pass's calls, refused
where the run did not run it; `--pass all` or no flag takes the run's whole list. `--failed` selects only
the items the run's ledger last records failed, and `--limit N` takes the first N. A run with no
`captures.jsonl` (a dry run, one pruned, one that asked nothing) is `refused`, `check: usage`,
`key: --replay`. The scoring role is resolved by `scoringRole` and the `Launch` built by `scoringLaunch`,
the functions `runScore` uses.

`Replay.Walk` renders each captured item text through `class.render`, which uses the binary's embedded
template for the capture's class, sends the prompt through the `Ask` its `Launch` hands the item, and
reads the reply with `call.attempt`, the runner's own parse against the captured item text under the
class's table, then computes the value. It writes no result to the KB, and a per-item failure is counted,
not the replay's outcome. While it runs it logs its progress through the verb's logger, at info level,
as the message `replay progress` with the keys `done`, `failed`, `remaining` and `total`: at most once
per `ReplayProgressInterval` ([§12](../../12-constants/index.md)) and once more when the loop ends. The counter and the
one goroutine that logs it are `buildstore.ReplayProgress`, which both development replays use. An item
only increments the counter as its call returns (an item whose status is `done` counts done, any other
answered item failed, an interrupted one neither), and no item and no progress line writes a file
(I11, [§2](../../2-invariants.md)).

The replay writes only under `score/<run>/replays/<replay-id>/`, each of its three files appended once,
when the loop ends, the id minted `replay-<id>` as a run's is:

- `replay.yaml`: `replay`, `run`, `backend`, `provider`, `model`, `pass`, `ask_sha256` (computed as
  `Create` computes it for the replay's pass) and `started`.
- `ledger.jsonl`: one line per item asked, in the run ledger's shape, its `run` the replay id.
- `captures.jsonl`: the replay's own captures, in the captures' shape, `run` the replay id.
- `comparison.jsonl`: per item, `item` (`ItemName`), `before` (the value the run's line recorded before
  its call), `original` and `replay` (each `status`, `value` and `faults`, the fault ids in order, `[]`
  for `none`) and `prompt_changed` (the replayed prompt's hash differs from the captured
  `prompt_sha256`).

Setup computes the hash and `replay.yaml` before creating anything, and creates `replays/` and the
replay's directory with `os.Mkdir`, never `MkdirAll`: a run the prune removed fails the replay rather
than being recreated without a `run.yaml`, which no prune would ever remove. After writing
`replay.yaml`, `Walk` keeps itself and the newest `KeptRuns-1` other replays of the run's `replays/` by
`replay.yaml`'s `started`, through `buildstore.PruneByStart`, which the run prune also uses, and after
the loop a replay whose directory no longer exists ends `failed` naming it. An item whose run records no
ledger line compares against an `original` with status `""` and is not counted in `changed`.

The result document is one SPEC §7 YAML document with the keys `outcome`, `state-dir`, `run`, `replay`,
`dir` (the replay directory, absolute), `counts` (`total`, `done`, `failed`, `skipped`) and `changed` (an
integer: the items whose status or value differs from the original's), then the final record the outcome
names. The outcome is `done` when every selected item was asked, `cancelled` when the context stopped the
loop (its `cancelled` record naming the first item it did not ask), and `failed` for a fault of the replay
itself, an unreadable capture file or a store write.

