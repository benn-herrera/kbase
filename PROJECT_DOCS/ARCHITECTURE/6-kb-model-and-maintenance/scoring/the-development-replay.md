#### The development replay

`kbase score --replay RUN` re-asks a run's captured subjects under the
binary's embedded templates and reports what the runtime would have done, so a change to a scoring
template is `edit → just build → replay` over the items that failed. SPEC states nothing of it.
Its code and tests are `cmd/score_replay.go` and `internal/score/replay.go` with their test files,
each carrying `//go:build kbasedev`. `cmd/score.go` reaches it only through the package-level
`scoreReplay` hook, which the tagged file's `init` sets and which is nil in the default build; that
`init` also declares `--replay`, `--failed` and `--limit` — a run takes no limit, so `--limit` is
the replay's alone — each annotated `kbase.mcp` = `excluded` through
`mcp.ExcludeFlag` ([§12](../../12-constants/index.md)), so dev and release builds serve one tool table. The default build is the
release shape and excludes the replay's code; dev builds pass `-tags kbasedev`, as `just build`,
`just test` and `just test-race` do. What it reads is not dev-only: a run's ledger and captures are
written by `internal/score/ledger.go`, untagged, so a replay compiled into a dev build reads the
records a released kbase wrote. `edit-gate` vets both polarities, and `test-dist-no-replay` runs
`TestDistBuildHasNoReplay` (`cmd/dist_test.go`) on the untagged build, which asserts the three flags
absent and the hook nil, and the same of the build's development replay ([§8](../../8-ledger-state-store-and-monitoring.md)).

`scoreReplayRun` takes `--pass`, `--failed`, `--limit`, `--scoring-model`, `--state-dir`, `--timeout`,
`--ask-concurrency` and `--config-dir` beside `--replay`. Any other score flag with it is a usage refusal naming `--replay`,
`--failed` or `--limit` without `--replay` one naming it, and a `--replay` value outside
`^(run|dry)-<id>$` (`replayRunRE`: the alphabet and length `kb.MintID` mints) one naming `--replay`. It opens no KB and
takes no KB, operation or holder lock; a run in progress may be replayed. With `--state-dir` it needs
no KB root and works from any directory; without it, it finds the store through the kb-root at or
above the working directory and is refused, `check: usage`, `key: --state-dir`, where there is none.
Both development replays, this one and the build's ([§8](../../8-ledger-state-store-and-monitoring.md)), resolve their store through
`replayStateDir` in `cmd/replay.go`, beside `replayTakes`, which names the flags a replay takes in
its refusal. A relative `--state-dir` resolves against the project root where the working directory
has one, and against the working directory where it has none. `score.OpenReplay` selects
every item with a capture, in the order of the run's `items.jsonl`, each by its last capture;
`--pass rigor` or `--pass bearing` narrows the selection to that pass's calls — refused where the run
did not run it — and `--pass all` or no flag takes the run's whole list; `--failed` selects only
those the run's ledger last records failed, and `--limit N` takes
the first N. A run with no `captures.jsonl` — a dry run, one pruned, one that asked nothing — is
`refused`, `check: usage`, `key: --replay`. The scoring role is resolved by `scoringRole` and the
launch built by `scoringLaunch`, the functions `runScore` uses.

`Replay.Walk` renders each captured closed text through `class.render` — the binary's embedded
template for the capture's class — sends it through the `Ask` its launch hands the item, and reads the reply with `call.put`, the
runner's own parse against the captured closed text under the class's table, then prices it. It lands
nothing, and a per-item failure is counted, not the replay's outcome. While it walks it logs its
progress through the verb's logger, at info level, as the message `replay progress` with the keys
`done`, `failed`, `remaining` and `total`: at most once per `ReplayProgressInterval` ([§12](../../12-constants/index.md)) and once more
when the walk ends, where `done` and `failed` are the result's counts. The counter and the one
goroutine that logs it are `buildstore.ReplayProgress`, which both development replays use; an item
only bumps the counter as its call returns — here an item whose status is `done` counts done, any
other answered item failed, and an interrupted one neither — and no item, and no progress line, writes a file
(I11). It writes only under
`score/<run>/replays/<replay-id>/`, each of its three files appended once, when the walk ends, the id minted `replay-<id>` as a run's is:

- `replay.yaml`: `replay`, `run`, `backend`, `provider`, `model`, `ask_sha256` (computed as `Create`
  computes it for the run's pass) and `started`.
- `ledger.jsonl`: one line per item asked, in the run ledger's shape, its `run` the replay id.
- `captures.jsonl`: the replay's own captures, in the captures' shape, `run` the replay id.
- `comparison.jsonl`: per item, `item` (`ItemName`), `before` (the value the run's line recorded
  before its ask), `original` and `replay` (each `status`, `value`
  and `faults`, the fault ids in order, `[]` for `none`) and `prompt_changed` (the replayed prompt's
  digest differs from the captured `prompt_sha256`).

Setup computes the digest and `replay.yaml` before creating anything, and creates `replays/` and the
replay's directory with `os.Mkdir`, never `MkdirAll`: a run the prune removed fails the replay rather
than being recreated without a `run.yaml`, which no prune would ever remove. After writing
`replay.yaml`, `Walk` keeps itself and the newest `KeptRuns-1` other replays of the run's `replays/`
by `replay.yaml`'s `started`, through `buildstore.PruneByStart`, which the run prune also uses, and after the walk a
replay whose directory no longer stands ends `failed` naming it. An item whose run records no ledger
line compares against an `original` with status `""` and is not counted in `changed`.

The result document is one SPEC §7 YAML document with the keys `outcome`, `state-dir`,
`run`, `replay`, `dir` (the replay directory, absolute), `counts` (`total`, `done`, `failed`) and
`changed` (an integer: the items whose status or value differs from the original's), then the closing
record the outcome names. The outcome is `done` when every selected item was asked, `cancelled` when
the context stopped the walk — its `cancelled` record naming the first item it did not ask — and
`failed` for a fault of the replay itself, an unreadable capture file or a store write.

---

