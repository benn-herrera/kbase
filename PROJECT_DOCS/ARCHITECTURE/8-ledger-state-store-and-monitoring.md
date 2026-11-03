## 8. Ledger, state store and monitoring

`internal/gitledger` is the only package that execs git (I1); it execs the host's git, which SPEC §2
already requires, so the user's configuration, hooks and attributes apply, and no git library is a
dependency. `internal/build` owns the walk, the state store's writers, resume, `--no-inference`,
`--through`, progress and cancel; `internal/buildstore` owns the stage vocabulary, the store's
reading, status and the trail's reading. Which packages and `cmd` files import `gitledger` is
[§11's dependency rules](11-package-map/dependency-rules.md).

- **Fresh versus resume** is read from the commit trail alone (SPEC §6); nothing in the state store
  is authoritative for position.
- **Inputs.** `build.Run`, once it has found the repository, makes a fresh build's volume roots and
  `--bibliography` files absolute against the repository root and puts them in path order before
  anything reads them: the launch checks, `walk.body`, the document graph and the records'
  regeneration. Every boundary records them, and `start` also records `started-at`, the HEAD before
  it — on a branch with no commit, an empty `kb-start:` commit made for it; the line keys and their
  readers are `internal/buildstore`'s. `--resume` takes the inputs from the newest trail entry in
  the order recorded, never re-sorted. `gitledger.Repo.Trail` stops at `Repo.Removal`, the newest
  commit off the trail that took `kb-root/` out of the tree, so a trail a committed removal ends is
  read by nothing. A resume reads the newest build's trail (`buildstore.CurrentTrail`: back to the
  newest `start` boundary), so a fresh build over a removed `kb-root/` starts a trail of its own; a
  fresh build reads none. `run` refuses, as soon as it reads the trail, a `--resume` with nothing to
  resume, `kb-root/` absent among it (SPEC §5), and `prepare` a fresh build over any `kb-root/`
  that exists, with `buildstore.KBRootExists`, the refusal `init` gives. A fresh build's dirt check
  drops the paths under the absent `kb-root/`, which its `start` boundary commits. `walk.result`
  hands out no `resume` where `kb-root/` does not exist when the run ends.
- **The document graph's tree** is written by `docgraph.Build`, then every leaf rewritten with each
  link pandoc wraps around a cross-reference unwrapped to that cross-reference
  (`build.unwrapTree`), in the build and in its records' regeneration alike.
- **Resume over an interrupted stage.** A stage writes its build records and its `kb-root/` work once
  its asks have all returned, so a stop during the asks leaves those paths as the boundary left them,
  and a kill during that write leaves them partial; either way it is uncommitted. A resume restores those paths to the last stage commit and
  re-runs the stage whole; the answer cache (`scratch/trails/<start-commit>/answers/`) is what carries the paid-for
  inference across, so the re-run asks nothing it already holds; it mints fresh ids, and nothing
  outside the restored paths referenced the old ones. A classify prompt names claim ids, but those
  were minted in stages already committed and the restore reaches back only to the last boundary, so
  a resumed run's prompts are identical and the cache answers them; a fresh build's are not, and it
  pays again. Each build record an inference stage writes is written once, after its asks; resume reads
  none of them for position. The progress record accounts for the dirt an interrupted
  stage leaves; the
  dirty-path refusal (SPEC §5) applies to dirt it cannot account for. The resume command is recorded as the argv
  `kbase build --resume` with every flag the launch carried but the bound, `--through`, and the inputs,
  and reaches a document as the build call each surface spells (below);
  `--config-dir` is written absolute: resuming past the bound is what resuming is for. It is emitted only where an identical re-issue continues the work: a `bounded` run is
  handed none, being finished at the bound — re-issuing its command is not what continues it;
  `build-trail` is what a reader uses to reset and resume. A refusal that
  recorded no boundary leaves nothing to continue. It is also emitted only where the
  recorded invocation reads as a `kbase build` invocation (SPEC §6): the state store is a directory
  the caller names, so a record naming any other program is repository-supplied text and no document
  renders it for a harness to run.
- **Records missing on resume.** The docgraph records live in the state store. A resume that finds
  them absent regenerates them by re-running the document-graph stage into scratch and requires its
  tree byte-identical to the committed `kb-root/` (SPEC §5, determinism); a difference is `failed`,
  naming the first differing path.
- **Two locks, both `internal/filelock` advisory locks, and both a run's — a build's or a scoring
  run's.** The *operation lock* is `<git dir>/kbase-operation.lock`, recording the state-dir; a run
  takes it after it opens and holds it for the whole run, and removes it on release. Because it is
  per repository, a second run through any `--state-dir` is `refused` (`check: lock`, `path` the
  state-dir, no remedy) with a detail naming the holder — "a build (pid 1) is running; its state-dir
  is …" or "a score run (pid 1) is running; …", both read from the store the lock names. A write op
  or `update-format` that finds it held is refused with the same item; a write verb
  probes for it before waiting on the write lock and again after taking it, and a scoring run's own
  landings and refreshes probe for it not at all: they write as the run that holds it. An empty
  lock file is the window between acquiring the lock and writing the state-dir: it reads as
  "starting", so a second run gets `retry` and a writer proceeds. A state-dir written before its
  store's holder record is the other window, and reads as "a run is starting". The build's order
  is: the operation lock; the state store's holder lock; its first progress event; and only
  then the wait for a write in progress, by taking and releasing the KB write lock ([§6](6-kb-model-and-maintenance/index.md); the same
  wait, `retry` if it cannot). `status` therefore sees a waiting build, and a build that gives up
  waiting ends `retry`. The full order of a build is: refuse a newer store at open; the operation lock;
  the holder lock; ready the store (upgrade, stamp); the run event; prune the captured builds; wait out
  writers. Its own writes and its seed and gate refreshes take no write lock, so it cannot refuse
  itself. A scoring run takes the same two locks, in the same order, through `build.Hold`: the
  operation lock first, then the holder lock, before it writes the first record of its run or its
  start refresh. The *holder lock* is in the state store, its record carrying the holder's pid, its
  start, the verb that owns the run (`Kind`) and, for a scoring run, the run id; `status` reads
  liveness from it and the run it names, `wait` reads whether the run it waits for still holds it,
  and `cancel` signals its holder. Given no `--state-dir`, the monitors — `status`, `cancel`, `wait`,
  `build-trail` — resolve their store through `buildstore.MonitorOptions.Open`, which reads the
  store the operation lock names while its file stands and names one, else the default; the MCP
  launch keeps the default. A clean exit removes the file, cancel included, so a standing lock
  file is not by itself evidence of a live holder. A record naming no known `kind` is a store another
  version wrote: `buildstore.ReadHolder` refuses it (`check: state-dir`, `path` the record), and
  nothing reads it as a build's. A second builder on the same state-dir is `refused`; where only
  the holder lock is held the result is `retry`. User-facing text calls it "holds the state store";
  "operation lock" names only `<git dir>/kbase-operation.lock`. On Windows, where no advisory lock
  exists, `status` cannot see a running run.
- **State store version.** `<state-dir>/format` is one integer, `buildstore.StoreFormat` ([§12](12-constants/index.md)),
  written when a build first holds the store; an absent file reads as 1. `build`, `status`, `wait`,
  `cancel` and the MCP build tool read it at open, and a store newer than `buildstore.StoreFormat`
  is refused (`check: state-dir`, `key` the marked `state-dir` flag, `path` the store, remedy "update
  kbase, or use another" and the same flag; SPEC §7 `key`). An older store is converted in place by the build, in "ready the store",
  through the chain `storeUpgrades` in `internal/build`: one step per version, each stamping
  `format` when it completes, so an interrupted upgrade resumes at the step it stopped at; a gap in
  the chain is an error. The chain is independent of the KB's `formats.Chain` (SPEC §10).
- **Retention.** At its start, after the run event, a build keeps the newest `keptBuilds`+1 ([§12](12-constants/index.md))
  captured builds under `reports/` — a capture being the `build-<UTC timestamp>-<random>` pair of
  `.stdout.yaml` and `.stderr.log`, ordered by name — and removes the rest; it reads no progress
  record to do so. Once its launch checks pass and before its first stage, a build prunes `scratch/trails/`,
  one directory per build trail named by the commit of the newest `start` entry of the trail the walk
  stands on, holding `answers/`, `asks/`, `captures/` and `replays/`: it keeps the current trail's
  directory — on a resume created if absent and stamped with the current time; a fresh build's
  directory is created by its first ask, so its time is when it last gained a subdirectory — and
  the newest `keptBuilds` ([§12](12-constants/index.md)) others by modification time, and removes the rest
  whole, with the undivided `scratch/answers/`, `scratch/asks/` and `scratch/captures/` an earlier kbase wrote. At the
  same point, a fresh build whose `progress.jsonl` holds more than `progressRotateLines` ([§12](12-constants/index.md)) lines
  moves every line before its own run event to `progress-<UTC timestamp>.jsonl`, leaves its run
  event alone in the record, and keeps the newest `keptProgressFiles` ([§12](12-constants/index.md)) such files; nothing
  reads them. The stage reports are never pruned, and `status` and `cancel` remove nothing.
  `status` computes `cache-entries` and `cache-bytes` over every trail's `answers/`. A
  scoring run prunes `score/` once its run's ledger has opened: of its kind (`run-` or `dry-`) it
  keeps the run it names and the newest `KeptRuns-1` ([§12](12-constants/index.md)) others by `run.yaml`'s start, and removes
  the rest whole ([§6](6-kb-model-and-maintenance/index.md), Scoring). It keeps the newest `KeptRuns` ([§12](12-constants/index.md)) captured runs under
  `reports/score-*`, by name, and removes the rest.
- **State store layout:** `format`, the holder lock, `progress.jsonl` and up to `keptProgressFiles` rotated
  `progress-<UTC timestamp>.jsonl` files, `records/` (the docgraph
  records and pre-pass census), `reports/<stage-id>.yaml` (each stage's row records and findings,
  named by the result's `stages[].report`; also the captures of a build the MCP server starts,
  `build-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`), `scratch/` —
  `trails/<start-commit>/`, one per build trail, holding `answers/`
  (the answer cache, content-addressed by model, system prompt and prompt and the call's sampling
  settings where they are not kbase's default, written through
  `atomicfile`; an ask's attempt ordinal is part of the key so a re-ask reaches the model),
  `asks/` (the multichoice-ask group records, one file per stage, `<kind>.yaml`, one YAML document per
  group: the group's shared template inputs once, each item's own inputs, its `prompt-sha256`
  (`asks.AskDigest` of the state and the question), its outcome (`answered`, `re-asked`,
  `defaulted`, `failed`) and its calls; written by a stage whose batch stopped as well as by one that
  finished), `captures/` (every call's request, reply and usage) and `replays/<replay-id>/` (a
  development replay of the trail, below), `regenerate/` (the records regeneration on
  resume), `score/<run>/` (a scoring run's `run.yaml`, `items.jsonl`, `ledger.jsonl` and
  `captures.jsonl`, `results/` — one file per answered item, `<walk index>.json`, until the walk's
  end or the next resume folds it — and a development replay's `replays/<replay-id>/`; [§6](6-kb-model-and-maintenance/index.md), Scoring).
- **The build's development replay.** `build.OpenReplay` and `Replay.Walk`
  (`internal/build/replay.go`, tagged `//go:build kbasedev` as [§6](6-kb-model-and-maintenance/index.md)'s scoring replay is) re-ask one
  trail's recorded asks under the binary's embedded templates and question files, from the inputs
  the trail records (I10), so a prompt, a question file or a model can be refined against one build
  without re-running it. The trail is the one a hex prefix of its start commit names, or, given
  none, the trail directory with the newest modification time, listed by `listTrails`, the listing
  `pruneTrails` orders by. `OpenReplay` reads each kind's `asks/<kind>.yaml`
  (`asks.ReadGroupRecords`) and the overview captures (`call.ReadOverviewCaptures`) and selects in
  walk order — the multichoice kinds in `asks.Kinds` order, groups in file order and items in
  record order, then the overview calls by ask attempt — narrowed to one kind or to `overview`, to
  the items recorded `failed` or `defaulted` and the overview calls `failed` or `rejected`, and to
  the first N. It refuses, `check: usage`, `key: --replay`, a store with no trail directory and a
  prefix matching none or more than one (`path` `scratch/trails`), a trail recording no group
  record and no overview capture (`path` the trail), an empty selection (`path` the trail), and a selected record or capture carrying no
  inputs (`path` that file). The replay's support code is likewise tagged `//go:build kbasedev`,
  in `internal/asks/replay.go` and `internal/asks/call/overview_captures.go`. The replay takes no
  lock. Its items are rebuilt by
  `asks.RecordedItems` and asked as one `asks.NewReplayBatch` on the decision role, then each
  overview call as its own item on the text role, through `call.New` pointed at the replay's
  directory, every call on the invocation's queue; `MultichoiceItem.Compose` and
  `asks.OverviewPrompt` stay the only composition sites, the build's own. A call that never
  completes is a `failed` row and the walk carries on. It logs its progress as [§6](6-kb-model-and-maintenance/index.md)'s scoring
  replay does, through `buildstore.ReplayProgress` and the verb's logger, its `total` the selected
  rows: the launch handed to `asks.NewReplayBatch` wraps each item's `MultichoiceReader`, and an
  item counts by its last call — `done` where it returned no error, `failed` where it returned an
  `asks.IncompleteError`, neither where its error stopped the batch — and each overview call
  likewise, so the last line's `done` and `failed` are `ReplayResult`'s. It writes only under
  `trails/<start-commit>/replays/<replay-id>/`, the id minted `replay-<id>`: `replay.yaml`
  (`replay`, `trail`, `decision_model`, `text_model`, `started`); its own `answers/` and
  `captures/`, in the trail's shapes; `asks/<kind>.yaml`, its own group records; and
  `comparison.jsonl`, one row per replayed item or overview call — `kind`, `group`, `item` (an
  overview row's `group` `""` and `item` `overview`), `attempt` (an overview row's alone),
  `original` and `replay` (each `outcome`, `choice` and `confidence`: a multichoice item's as its
  record holds them; an overview call's outcome `failed` where its capture's last attempt did not
  complete, else the overview stage's judgement of its reply, `answered` or `rejected`, and its
  choice and confidence null) and `prompt_changed` (the re-rendered prompt's digest differs from
  the recorded `prompt-sha256`). It never writes the trail's `answers/`, `asks/` or `captures/`,
  nor `kb-root/`. Setup creates `replays/`, the replay's directory and its subdirectories with
  `os.Mkdir`, never `MkdirAll`, so a trail the prune removed fails setup rather than standing
  again, and restores the trail directory's modification time, which is the current-trail marker
  and the prune's order. After writing `replay.yaml`, it keeps itself and the newest `KeptRuns-1`
  ([§12](12-constants/index.md)) other replays of its trail's `replays/` by `replay.yaml`'s `started`, through
  `buildstore.PruneByStart`; pruning removes only entries of `replays/`, so the trail directory's
  time does not move. Once the walk drains, a replay whose `replay.yaml` no longer stands — a
  prune removed it, and its in-flight calls' own writes may have recreated parts of it — removes
  its directory — and its trail's `replays/` and the trail directory where they hold nothing
  else — and ends `failed`; otherwise it writes its group records and `comparison.jsonl`, on
  every exit after it asked anything. `ReplayResult` counts the selected rows (`total`), those
  replayed to an outcome other than `failed` (`done`) and those `failed`, and as `changed` the rows
  whose outcome or choice differs from the original's. Its outcome is `done` when every selected
  row was asked, `cancelled` when the context stopped the walk — naming the first row it did not
  ask — and `failed` for a fault of the replay itself: a store write, a compose error, or the
  replay directory removed under the walk.

  The verb is `kbase build --replay [<start-commit>]`, in `cmd/build_replay.go`, reached from
  `cmd/build.go` only through the `buildReplay` hook that file's `init` sets; the tag, the hook
  pattern, `mcp.ExcludeFlag` and `test-dist-no-replay` are as for score's replay ([§6](6-kb-model-and-maintenance/index.md)). The `init`
  declares `--replay` (a boolean), `--kind`, `--failed` and `--limit`, each excluded from MCP.
  Beside `--replay` it takes `--kind` (`paragraph`, `unmarked`, `colinear`, `classify` or
  `overview`), `--failed`, `--limit N`, `--state-dir`, `--ask-concurrency` and `--config-dir`; the
  store resolves as [§6](6-kb-model-and-maintenance/index.md) states for both replays. Each refusal is `check: usage`: `--kind`,
  `--failed` or `--limit` without `--replay`, naming that flag; any other of build's own flags
  beside `--replay`, read from `LocalNonPersistentFlags`, naming `--replay`; more than one
  positional, or one not matching `^[0-9a-f]+$`, naming `--replay`; a `--kind` outside the five,
  naming `--kind`; a negative `--limit`, naming `--limit`; and the store's and `OpenReplay`'s
  refusals above. Models and sampling come from configuration through the build's own Provider
  seam and queue (`buildModels`, `cmd/build.go`): the decision role answers the multichoice asks
  and the text role the overview calls. The result document, verb `build`, carries `outcome`,
  `state-dir`, `trail` (the start commit), `replay`, `dir` (the replay directory, absolute),
  `counts` (`total`, `done`, `failed`) and `changed`, then the closing record its outcome names, a
  `cancelled` record holding the `kind`, `group` and `item` of the first row not asked.
- **Progress units** are leaves for the node pass, source groups for `references-found`, the ask
  groups of each of its two passes for `claims-culled` (each pass announcing its own total), ask
  groups for classification; a defaulted item is a `fallback` event. A stage's `units` event is written
  before its asks and its unit and `fallback` events in one append after them, so `status` moves
  between ask batches; every build record is written once per stage, after its asks.
- **Trail order.** `run` refuses a trail recording a stage past one it leaves unrecorded in
  `Stages` order (SPEC §5, `check: trail`), right after reading which stages it records and before
  the launch checks and any restore: walking the gap would run a stage over products of stages
  meant to follow it.
- **A stale `.git/index.lock`** left by a kill during a boundary commit is a refusal naming the
  lock.
- **`--state-dir`** defaults to the [§12](12-constants/index.md) key under `$XDG_STATE_HOME/kbase/`.
- **Under `--no-inference`** the overview call is the dropped row and no `README.md` is written.

- **Ledger.** Commit subject, tracked records and pathspec scoping are SPEC §6; resume position is
  read from the trail alone. The
  four tracked build records — node-pass, unmarked, classification and cull — are
  `claimgraph`'s to read and write, as YAML `kb-build-*.yaml`
  (SPEC §6, [§10](10-compatibility-harness.md)). `build` commits them.
- **State store.** Contents and path are SPEC §6; the key form is [§12](12-constants/index.md); the layout is above.
- **Status, progress, cancel, wait.** `status` reads the holder lock and its record,
  `progress.jsonl` and the commit trail and returns the YAML snapshot in SPEC §6; a record naming
  `score` is reported in the score run's own shape, read from the run's ledger and result document;
  that reading is `internal/score`'s (`score.Status`), which the verb renders. A build's `stages`
  lists the stages done and not done in the newest build's trail and names `build-trail` for their
  commits; `buildstore.BuildTrail` reads the same trail, each boundary's command its
  `git reset --keep` then the build resumed with the flags its last run was launched with: the
  resume argv the run the store stands at recorded, through `RecordedCommand` as `status` reads it
  (the marked `build` call with `resume` alone where no run recorded one that reads as a kbase
  build invocation), and `started-at`'s the reset alone.
  Where no build run holds the store and `kb-root/` is absent, or the trail above a removal of it is
  empty, both read no trail and `status` no run (`buildstore.noBuild`). `score.Status` also counts the run's result files while it walks. Where no live run holds the store and its record names no score run,
  `status` reports the store's newest run, comparing the start the newest run directory's `run.yaml`
  records (`score.NewestRun`) with the newest build invocation's in `progress.jsonl`
  (`buildstore.ReadState`), and reports the build where neither stands or the two do not order.
  `cancel` signals the holder the record names, a build's or a scoring run's, and waits for the
  holder lock to go through the one `build.StopHolder` — `cmd` choosing only which run's resume the
  document carries; the holder abandons
  the calls in flight with nothing written for them, logs the event, releases the locks and exits
  `cancelled`. `wait` polls `buildstore.ReadState` through `buildstore.WaitState` — the store
  alone — until the run it names is superseded or has ended, then renders
  `status`'s document through the same
  `buildstore.Status`: it takes no lock and reads no commit trail until that final document. A score
  run's pid is waited for through `score.RunByPID` and the store's holder record instead
  (`score.WaitRun`), so `wait` returns the run's own result document.
- **A recorded document is spelled when it is emitted again** (SPEC §7 `key`, `remedy`). The progress
  record's items and a score run's result document beside its ledger hold the marked form
  (`internal/argmark`): each was written on a writer that spells nothing. `status` and `cancel`
  emit them again through `result.Emit` on the writer of the call reading
  them, so a recorded item and the recorded `Resume` argv — `buildstore.RecordedCommand`'s
  `argmark.Recorded` call, read with `build`'s own flags into an arguments mapping where it is spelled —
  reach each surface in its spelling, and `wait` re-emits a score run's document through
  `result.Reemit`. The store's layout is unchanged: a marker sits inside the string fields, and
  `Resume` stays an argv. A launched run is told its document is read over MCP by
  `mcp.SurfaceEnv` in its environment, set by `Launch.start`; `cmd/main.go` reads it before
  `Execute` and spells the process's stdout for MCP, so the run's own document, which the launch
  hands back where the run exits first, is the tool's.

