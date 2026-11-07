### The build's development replay

**Pattern:** Offline eval / record-replay regression harness.

`build.OpenReplay` and `Replay.Walk` (`internal/build/replay.go`, tagged `//go:build kbasedev`, as the scoring replay is,
[§6](../6-kb-model-and-maintenance/scoring/the-development-replay.md)) repeat one build's recorded questions under the
binary's embedded templates and question files, from the inputs the build directory records
([§2](../2-invariants.md), I10), so a prompt, a question file or a model can be refined against one build
without re-running it. The build is the one a hex prefix of its start commit names, or, given none,
the directory with the newest modification time, listed by `listTrails`, the listing `pruneTrails`
orders by.

- **Selection.** `OpenReplay` reads each kind's `asks/<kind>.yaml` (`asks.ReadGroupRecords`) and the
  overview captures (`call.ReadOverviewCaptures`) and selects in this order: the multichoice kinds in
  `asks.Kinds` order, groups in file order and items in record order, then the overview calls by
  attempt. The selection can be narrowed to one kind or `overview`, to the items recorded `failed` or
  `defaulted` and the overview calls `failed` or `rejected`, and to the first N. `OpenReplay` refuses
  (`check: usage`, `key: --replay`):
  - a store with no build directory, and a prefix matching none or more than one build (`path` the
    store's `scratch/trails`);
  - a build recording no group record and no overview capture, and an empty selection (`path` the
    build directory);
  - a selected record or capture with no inputs (`path` that file).
- **Calls.** The replay takes no lock. `asks.RecordedItems` rebuilds the items and `asks.NewReplayWave`
  puts them to the `decision` role; each overview call is its own item on the `text` role, through
  `call.New` pointed at the replay's directory, every call on the invocation's queue.
  `MultichoiceItem.Compose` and `asks.OverviewPrompt` remain the only composition sites. A call that
  never completes is a `failed` row and the replay continues. Progress is logged as the scoring replay
  logs it, through `buildstore.ReplayProgress`, with `total` the selected rows. `countedLaunch` wraps
  the launch handed to `asks.NewReplayWave`: it wraps each item's `MultichoiceReader` and counts the
  item by its last call, `done` where it returned no error, `failed` where it returned an
  `asks.IncompleteError`, neither where its error stopped the wave. Each overview call is counted
  the same way, so the last line's `done` and `failed` are `ReplayResult`'s.
- **Output.** It writes only under `scratch/trails/<start-commit>/replays/<replay-id>/`, the id minted
  `replay-<id>`: `replay.yaml` (`replay`, `trail`, `decision_model`, `text_model`, `started`); its own
  `answers/` and `captures/`, in the build directory's shapes; `asks/<kind>.yaml`, its own group
  records; and `comparison.jsonl`, one row per replayed item or overview call, with the keys `kind`, `group`,
  `item`, `attempt`, `original`, `replay` and `prompt_changed`:
  - an overview row has `kind` `overview`, `group` `""` and `item` `overview`, and it alone has
    `attempt`;
  - `original` and `replay` each hold `outcome`, `choice` and `confidence`; a multichoice item's are
    as its record holds them. An overview call's `outcome` is `failed` where its capture's last
    attempt did not complete, else the overview stage's judgement of its reply, `answered` or
    `rejected`, and its `choice` and `confidence` are null;
  - `prompt_changed` is true where the re-rendered prompt's digest differs from the recorded
    `prompt-sha256`. It never writes the build directory's `answers/`, `asks/`
  or `captures/`, nor `kb-root/`.
- **Directory handling.** Setup creates `replays/`, the replay's directory and its subdirectories with
  `os.Mkdir`, never `MkdirAll`, so a build directory the prune removed fails setup rather than being
  recreated, and restores the build directory's modification time, which is the current-build marker and
  the prune's order. After writing `replay.yaml`, the replay keeps itself and the newest `KeptRuns`-1
  other replays of its build's `replays/` by `started` (`buildstore.PruneByStart`), which removes only
  entries of `replays/`. Once the item loop drains, a replay whose `replay.yaml` no longer exists,
  because a prune removed it, removes its directory, and its `replays/` and the build directory where
  they contain nothing else, and ends `failed`; otherwise it writes its group records and
  `comparison.jsonl`, on every exit after it made any call.
- **Outcome.** `ReplayResult` counts the selected rows (`total`), those replayed to an outcome other than
  `failed` (`done`), those `failed`, and as `changed` the rows whose outcome or choice differs from the
  original's. The outcome is `done` when every selected row was put to a model, `cancelled` when the
  context stopped the replay, and `failed` for a fault of the replay itself: a store write, a compose
  error or the replay directory removed during the replay.
- **Verb.** `kbase build --replay [<start-commit>]` is in `cmd/build_replay.go`, reached from
  `cmd/build.go` only through the `buildReplay` hook that file's `init` sets. The tag, the hook,
  `mcp.ExcludeFlag` and `test-dist-no-replay` are as for the scoring replay ([§6](../6-kb-model-and-maintenance/scoring/the-verb.md)). `--replay`,
  `--kind`, `--failed` and `--limit` are excluded from MCP. Beside `--replay` it takes `--state-dir`,
  `--ask-concurrency` and `--config-dir`; any other of build's own flags is refused, naming `--replay`, as
  are `--kind`, `--failed` or `--limit` without it, more than one positional or one not matching
  `^[0-9a-f]+$`, a `--kind` outside the five (`paragraph`, `unmarked`, `colinear`, `classify`,
  `overview`) and a negative `--limit`. Models and sampling come from the build's own `Provider` function
  (`buildModels`, `cmd/build.go`). The result has verb `build` and `outcome`, `state-dir`,
  `trail` (the start commit), `replay`, `replay-dir` (the replay directory, absolute), `counts` and `changed`, then the final record
  its outcome names; the `cancelled` record has the keys `kind`, `group` and `item` of the first row
  not asked.

---

