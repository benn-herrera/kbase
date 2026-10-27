# ACTIVE PLAN REVIEW BURNDOWN – multi-threaded asks (`e45c06e..644fdbc`)

## Status

The prompt engineer has reviewed this file and the architect has revised it. Every ruling is
written in; no item is pending.

- B-01: captures move into the trail directory; `reports/build-*` keeps the newest six.
- B-28: `reports/score-*` keeps the newest sixteen.
- B-26: `MultichoiceItem`; the two `asks` YAML keys become `choice`; the term "letter ask" leaves
  code comments (PK-I) and the documents (PK-N).

## How to use this file

- A dispatch carries three things:
  - this section;
  - `ACTIVE_PLAN.md` "Rules for every package" and "Principles that steer open choices";
  - the one package section it executes.
- **Pending items.** An item may carry the line "PENDING USER RULING — if this line stands, stop
  and report; do not execute." If yours does, stop and report. Execute nothing in that item.
- **Line numbers are as of `644fdbc`.** This replaces the Rules' `aee74c1`.
  - Every location gives a command that re-finds it.
  - If the command finds the text at another line, use that line.
  - If it finds nothing, report that and do not guess.
- **"Design" in the Principles** means `MULTI_THREADED_ASKS.md`. Its §3 is "Invariants".
- **Search with `git grep`, never `grep -r`.** A recursive `grep` also walks gitignored scratch
  trees that hold old copies of the source. Its answers then differ from the ones quoted here, and
  checks written against `git grep` can never pass.
- **Shared Go packages.** Two packages in one wave may edit different files of the same Go package.
  If `just test-pkg` fails in a file outside your scope, report the failure and do not fix it.
  Run `just test-pkg` again once the sibling package has reported.

## Waves and commits

Packages within a wave edit disjoint files and run in parallel. A wave starts when the wave before
it has passed its gate.

| Wave | Packages | Why it waits |
|---|---|---|
| R1 | PK-A ∥ PK-B ∥ PK-C ∥ PK-D ∥ PK-E ∥ PK-K | — (PK-A and PK-K also wait on the B-01 ruling) |
| R2 | PK-F ∥ PK-G ∥ PK-H ∥ PK-L ∥ PK-M | PK-F edits `call.go`, `call_test.go`, `letters.go` and `build.go` after PK-A and PK-B, and it uses PK-C's `ChatResult.Final`. PK-G's tests assume PK-B's B-03 and PK-C's helper move. PK-H edits SPEC and ARCHITECTURE after PK-A, and it describes PK-C's constant and PK-E's refusals. PK-L edits `internal/score/ledger.go` after PK-D. PK-M waits on nothing; its files are disjoint from every other package's, and PK-H lands its doc text (`README.md` on `just dist`, CONVENTIONS "Build and Run"). |
| R3 | PK-I ∥ PK-J ∥ PK-N | Both renames reach files that earlier packages edit. PK-N edits the documents after PK-H. |

- **Rating and seat.** Each item takes its package's rating, model and seat from the table below.
- **Contract text** that an item needs lands in the same commit as the item's code.

**The wave-end gate.** The coordinator runs this after auditing each diff against CONVENTIONS
"Coordinator Policy":

1. `just edit-gate`. On a format finding, run `just fmt`, then `just edit-gate` again.
2. `just test-race`.
3. `just test`.
4. `just test-integration`.
5. `just test-integration-build-arxiv`.
6. `just test-integration-monitor-arxiv`.

**Commits.** There are two commits:

- one at the end of R2, carrying R1 and R2, because PK-H lands the contract text for code that R1
  changes;
- one at the end of R3.

Each commit body names every test truth it moved.

| Package | Scope (files) | Rating → model | Seat | Wave |
|---|---|---|---|---|
| PK-A | `internal/build/{store.go,build.go,storeformat_test.go,trails_test.go,inference_test.go}`, `internal/asks/call/{call.go,call_test.go}`, `SPEC.md`, `ARCHITECTURE.md` | expert → coordinator's model | go-coder | R1 (pending B-01) |
| PK-B | `internal/asks/{letters.go,letters_test.go}`, `internal/claimgraph/{stages.go,unmarked.go,cull.go}` | expert → coordinator's model | go-coder | R1 |
| PK-C | `internal/model/{queue.go,queue_test.go}`, `internal/model/modeltest/{mockllm.go,mockllm_test.go,mockllm_external_test.go}`, `cmd/{score.go,probe.go,askconcurrency.go}` (new), `internal/config/{config.go,template.go,write.go}` | mechanical → `sonnet` | go-coder | R1 |
| PK-D | `internal/score/{run.go,invoke.go,ledger.go,ledger_test.go,status_test.go,run_test.go}` | mechanical → `sonnet` | go-coder | R1 |
| PK-E | `internal/write/` (all files), `internal/score/invoke_test.go` | expert → coordinator's model | go-coder | R1 |
| PK-K | `tools/scripts/{test-integration-build-live-arxiv.sh,test-integration-build-live-fixture.sh,write-evidence.sh}` | mechanical → `sonnet` | shell-dsl-coder | R1 (pending B-01) |
| PK-F | `internal/asks/call/{call.go,call_test.go}`, `internal/asks/letters.go`, `internal/build/{build.go,build_test.go}` | mechanical → `sonnet` | go-coder | R2 |
| PK-G | `cmd/concurrentasks_test.go`, `internal/build/concurrentasks_test.go`, `cmd/askconcurrency_test.go` | expert → coordinator's model | go-coder | R2 |
| PK-H | `SPEC.md`, `ARCHITECTURE.md`, `CONVENTIONS.md`, `README.md` | expert → coordinator's model | tech-writer | R2 |
| PK-L | `internal/score/{ledger.go,ledger_test.go}` | mechanical → `sonnet` | go-coder | R2 |
| PK-M | `justfile` (the `dist` recipe), `tools/scripts/build-dist.sh`, `tools/scripts/test-build-dist.sh`, `tools/scripts/testdata/stubs/git` (new) | expert → coordinator's model | shell-dsl-coder | R2 |
| PK-I | the files B-26 lists | mechanical → `sonnet` | go-coder | R3 (pending B-26) |
| PK-J | `internal/ledger/` → `internal/gitledger/`, its nine importers, `cmd/{execpolicy_test.go,importpolicy_test.go}`, `ARCHITECTURE.md` | mechanical → `sonnet` | go-coder | R3 |
| PK-N | `SPEC.md`, `ARCHITECTURE.md`, `CONVENTIONS.md`, `README.md`, `USER_CONVENTIONS.md` | mechanical → `sonnet` | tech-writer | R3 |

---

## PK-A — call captures and pruning

### B-01 [Critical] — call captures grow without bound once `progress.jsonl` rotates

- **Finding and evidence.**
  - `pruneCaptures` counts ended builds in the live progress record alone. It returns early until
    that record holds five of them.
    - `grep -n 'ReadProgress(dir)\|len(starts) < keptBuilds' internal/build/store.go` answers 358
      and 368.
    - Line 368 reads `if len(starts) < keptBuilds {`, and the line after it is `return nil`.
  - When `rotateProgress` rotates, it keeps only the last line of the record, which is this run's
    event.
    - `grep -n 'rotateProgress(dir\|LastIndexByte\|data\[last:\]' internal/build/store.go`
      answers 238, 257 and 262.
    - Line 262 is `atomicfile.Write(live, data[last:], nil)`.
  - The order of a build's start is: the run event at `build.go:215`, `pruneCaptures` at `:219`,
    then rotation at `:335-341`, which runs on fresh builds only (`if !resumed {`).
    - `grep -n 'pruneCaptures(w.stateDir)\|rotateProgress(w.stateDir)' internal/build/build.go`
      answers 219 and 339.
  - So once each of four builds writes more than about 125 lines (500 / 4), the fifth fresh build
    rotates again before it has counted five ended builds. After that, nothing is ever pruned.
  - `scratch/captures/` holds one file per call. `reports/build-*` stops being pruned too. Both
    break the pruning rule in SPEC §6 (`grep -n 'and the call captures' SPEC.md` answers 342).
- **Guidance.** Call captures move into the trail directory and are pruned with their trail. The
  count of ended builds goes.
  1. **Captures in the trail directory.**
     - A `Caller` resolves its captures directory from the current trail directory at every call,
       as it resolves its answers directory today.
     - Replace `New`'s `scratch` and `answers` parameters with one parameter,
       `trail func() string`, which returns the trail directory. Derive `answers/` and
       `captures/` (`asks.CapturesDir`) from it.
     - To find them: `grep -n '^func New\|captures  *string' internal/asks/call/call.go` answers
       46 and 55.
     - `build.go` passes `func() string { return w.trailDir }`. To find the call sites:
       `grep -n 'call.New(' internal/build/build.go` answers 408 and 409. Delete the `scratch` and
       `answers` locals above them.
  2. **`pruneTrails`.** Add `asks.CapturesDir` to the flat directories it removes.
     `grep -n 'asks.AnswersDir, asks.AskRecordsDir' internal/build/store.go` answers 344.
  3. **Rename `pruneCaptures` to `pruneBuildCaptures`.** It prunes `reports/build-*` alone, by
     count.
     - A capture is the pair `build-<UTC stamp>-<random>.stdout.yaml` and `.stderr.log` that share
       a stem. The stamp format sorts by name.
     - Keep the newest `keptBuilds`+1 captures by name, and remove the rest.
     - It reads no progress record.
     - It runs where it runs now.
     - Update the `keptBuilds` comment.
  4. **Tests.** Each test pins one claim.
     - In `internal/build/storeformat_test.go`, find the tests with
       `grep -n 'func TestPruneCaptures' internal/build/storeformat_test.go`. It answers 92 and
       162. Replace them with these two:
       - `TestPruneBuildCapturesKeepsTheNewestSix`. Given eight captured builds and a
         `progress.jsonl` that holds one run event, the prune leaves the newest six pairs. This
         fails today.
       - `TestPruneBuildCapturesBelowTheLimitKeepsAll`. Five captured builds, and no progress
         record at all: all five stand, and the prune returns no error.
     - Delete the replaced test's flat `scratch/answers/` and `scratch/captures/` fixtures (`:118-120`).
     - In `internal/build/trails_test.go`, add `captures/c.capture.jsonl` to each trail and a flat
       `scratch/captures/flat.capture.jsonl` in `TestPruneTrailsKeepsTheCurrentAndTheNewestFive`.
       That test then pins three things:
       - a pruned trail's `captures/` goes with it;
       - a kept trail's `captures/` stands;
       - the flat directory is gone.
     - Point every read of the captures directory at the trail directory:
       - `git grep -n 'CapturesDir' -- internal/build/inference_test.go internal/asks/call/call_test.go`
         answers `inference_test.go:346` and `call_test.go:132, 320, 441`;
       - `call_test.go`'s `newCaller` helper as well.
- **Contract text (this package lands it).**
  - **SPEC §6.** `grep -n 'and the call captures' SPEC.md` answers 342. Replace the sentence that
    opens "At a build's start, the captured output of earlier builds" (lines 341-344) with this
    text:
    > At a build's start, the captured output of builds the MCP server started (`reports/build-*`,
    > §11) is pruned to the newest six by the start each one's name records.
  - **SPEC §6, the trail sentence.** `grep -n "answer cache and letter-ask records are kept" SPEC.md`
    answers 354.
    - "answer cache and letter-ask records are kept" becomes "answer cache, letter-ask records and
      call captures are kept".
    - "the undivided cache and records an earlier kbase kept" becomes "the undivided cache, records
      and call captures an earlier kbase kept".
  - **ARCHITECTURE §8 "Retention".** `grep -n 'Retention' ARCHITECTURE.md` answers 1115.
    - Replace its first sentence, through "is not counted.", with this text:
      > At its start, after the run event, a build keeps the newest `keptBuilds`+1 (§12) captured
      > builds under `reports/` — a capture being the `build-<UTC timestamp>-<random>` pair of
      > `.stdout.yaml` and `.stderr.log`, ordered by name — and removes the rest; it reads no
      > progress record to do so.
    - "holding `answers/` and `asks/`" becomes "holding `answers/`, `asks/` and `captures/`".
    - "with the undivided `scratch/answers/` and `scratch/asks/`" becomes "with the undivided
      `scratch/answers/`, `scratch/asks/` and `scratch/captures/`".
  - **ARCHITECTURE §8, the build order.** `grep -n 'the run event; prune captures' ARCHITECTURE.md`
    answers 1093. "prune captures" becomes "prune the captured builds".
  - **ARCHITECTURE §8 layout.** `grep -n '.captures/. (every' ARCHITECTURE.md` answers 1135.
    Move "`captures/` (every call's request, reply and usage)" out of `scratch/` and into the list
    that `trails/<start-commit>/` holds.
  - **ARCHITECTURE §12.** `grep -n '| Builds kept' ARCHITECTURE.md` answers 1504. The row reads:
    > `keptBuilds` in `internal/build`: 5, the build trails, beside the current one, whose
    > directories — answers, group records and call captures — survive pruning, and the captured
    > builds, beside the newest, that survive it (§8)
- **Checks.**
  - `git grep -n 'scratch/captures' -- SPEC.md ARCHITECTURE.md` answers one line, the
    ARCHITECTURE Retention line.
  - `git grep -n 'ReadProgress' -- internal/build/store.go` prints nothing.
- **Done-line.**
  - `just test-pkg internal/build`;
  - `just test-pkg internal/asks/call`;
  - the checks above.

---

## PK-K — the live recipes' counts

### B-01, scripts part — captures counted under the trail directories

- **Guidance.**
  - Count `*.capture.jsonl` under the `scratch/trails` directory of the state store each line
    already names. `git grep -n 'scratch/captures' -- tools/scripts` answers:
    - `test-integration-build-live-arxiv.sh:111` (`${state}`);
    - `test-integration-build-live-arxiv.sh:140`, twice (`${root}/${dir}/env-state`);
    - `test-integration-build-live-arxiv.sh:189` (`${layered}/state`);
    - `test-integration-build-live-fixture.sh:174` (`${state}`);
    - `write-evidence.sh:45`.
  - Each count's root stays the one it has. Only `/scratch/captures` becomes `/scratch/trails`.
  - In `write-evidence.sh:45`, "scratch/captures/, scratch/trails/" becomes
    "scratch/trails/ (per trail: answers/, asks/, captures/)".
- **Check.** `git grep -n 'scratch/captures' -- tools/scripts` prints nothing.

### B-02 [Warning] — the live recipes pass when no letter-ask answer was cached

This item does not wait on the ruling. It ships with this package.

- **Finding and evidence.**
  - Both recipes count cached answers as `*.txt`. `git grep -n "'\*.txt'" -- tools/scripts`
    answers `live-arxiv.sh:112` and `live-fixture.sh:175`.
  - Letter-ask answers are written as `.json`. `grep -n 'key+".json"' internal/asks/call/call.go`
    answers 166: `answer := filepath.Join(c.answers(), key+".json")`.
  - Only the overview's answers are `.txt`. `grep -n '".txt"' internal/asks/call/call.go`
    answers 443.
- **Guidance.**
  - Count `*.json` (letter answers) and `*.txt` (overview answers) separately under
    `${state}/scratch/trails`.
  - In `comparison.yaml`, record them as `letter-answers` and `overview-answers`, in place of
    `answers`.
  - Both scripts require `letter-answers` > 0. The arxiv script, which already checks the README
    passage, also requires `overview-answers` > 0.
  - Update each check's detail string to match.
- **Done-line.**
  - Run `bash -n tools/scripts/<name>.sh` on each changed script, as CONVENTIONS "Testing"
    requires.
  - No `just --dry-run` check applies: these edits change no variable a recipe passes.

---

## PK-B — the group's first ask; stage wall time

### B-03 [Warning] — a one-item group's first ask holds up every later group

- **Evidence.** `sed -n 273,282p internal/asks/letters.go` shows:
  - `wg := &b.wg`;
  - `if i == 0 { wg = &alone }`, the item's launch, then `alone.Wait()`, for every group whatever
    its length.
- **Finding.** A group of one has no later item that could reuse its prefix. The wait only stops
  the next group from launching.
- **Guidance.**
  - Route item 0 to `alone` only when `len(items) > 1`.
  - Update `AskGroup`'s doc comment to match.
  - Add this test to `letters_test.go`. It fails today:
    - a batch of two one-item groups;
    - the first item blocks until the second item has started;
    - the test fails on a 2 s timeout.
- **Done-line.** `just test-pkg internal/asks`.

### B-04 [Note] — no stage report records a stage's elapsed time

- **Evidence.**
  - `askTotals` reports `wall-seconds` as `t.WallSeconds`, the sum of per-call wall time.
    `sed -n 96,106p internal/claimgraph/stages.go` shows this.
  - `git grep -n 'stage-wall\|elapsed' -- internal/claimgraph internal/asks` prints nothing
    stage-level.
- **Location.**
  - `askTotals`, and its callers: `git grep -n 'askTotals(' -- internal/claimgraph` answers
    `stages.go:232, 670`, `unmarked.go:329` and `cull.go:360, 361`.
  - `asks.Batch`: `grep -n 'func NewBatch' internal/asks/letters.go` answers 241.
- **Guidance.**
  - `asks.Batch` records its elapsed time, from `NewBatch` to the return of `Wait`.
  - Each stage passes that time to `askTotals`, which adds `stage-wall-seconds` (one decimal)
    right after `wall-seconds`. The cull reports each of its two batches.
  - Add no test: the figure is a measurement.
- **Done-line.** `just test-pkg internal/asks`; `just test-pkg internal/claimgraph`.

---

## PK-C — model, mock and the ask-concurrency default

### B-05 [Warning] — `Slot.Chat` drops the attempt the call ended on, and two callers re-derive it

- **Evidence.**
  - `grep -n '_, err := chat(' internal/model/queue.go` answers 121.
  - `git grep -n 'completed on its last attempt' -- cmd` answers `probe.go:156` and `score.go:383`.
    Each is followed by `res.Attempts[len(res.Attempts)-1]`.
- **Guidance.**
  - Add `Final ChatAttempt` to `ChatResult`, filled from `chat`'s first return value. Its doc
    comment says:
    - it is the attempt the call ended on;
    - on a call that returned no error, it is the last of `Attempts`;
    - where the context ended the call, it is the interrupted attempt, which `Attempts` does not
      hold (`internal/model/chat.go:105-106` returns before `observe`).
  - Replace the two re-derivations with `res.Final`, and delete both comments.
  - **Leave `internal/model/chat_test.go` alone.** It has `lastAttempt` at :82 and 11 call sites:
    128, 161, 207, 243, 265, 343, 411, 429, 499, 530 and 590. They pin `Attempts`, the observed
    list. On a context-ended call, `Final` is a different value.
- **Done-line.** `just test-pkg internal/model`; `just test-pkg cmd`.

### B-06 [Note] — `errSlotNoClient` refuses a state nothing produces

- **Evidence.**
  - `Registry.Get` returns `nil, err` or a client that is not nil (`sed -n 61,77p internal/modelclient/modelclient.go`).
  - Its two callers return on that error:
    - `cmd/build.go:157-160` (`client, err := reg.Get(ep); if err != nil { return nil, "", err }`);
    - `cmd/score.go:360-363` (`scoringLaunch`).
  - The rule is CONVENTIONS "Name the producer before you write the refusal".
- **Location.**
  - `grep -n 'errSlotNoClient' internal/model/queue.go` answers 74 and 90.
  - `grep -n 'nil client' internal/model/queue_test.go` answers 270, 299 and 309.
- **Guidance.**
  - Delete `errSlotNoClient` and its `case`. `refusal` no longer takes the client.
  - Delete the `"/nil client"` subtest.
  - Take "or with a nil client" out of `TestSlotRefusesOutsideItsItem`'s doc comment.
- **Done-line.** `just test-pkg internal/model`.

### B-07 [Note] — the ask-concurrency default and its helpers live in `score.go`, and the template restates the default

- **Evidence.**
  - `git grep -n 'askConcurrency(\|givenAskConcurrency\|registerAskConcurrencyFlag' -- cmd` shows
    callers in `cmd/build.go` and `cmd/score_replay.go`.
  - The default is restated at `internal/config/template.go:27`, `# concurrency = 2`.
- **Guidance.**
  - Declare `DefaultAskConcurrency = 2` in `internal/config/config.go`, beside `keyConcurrency`
    (`grep -n 'keyConcurrency = ' internal/config/config.go` answers 66).
  - Render the template's `2` through a `%d` slot, filled from that constant at `write.go:121`.
  - Move these from `cmd/score.go:88-126` into a new `cmd/askconcurrency.go`:
    - the three functions `registerAskConcurrencyFlag`, `givenAskConcurrency` and
      `askConcurrency`;
    - the const `askConcurrencyFlag`.
  - Delete `defaultAskConcurrency`. Its two uses read `config.DefaultAskConcurrency`.
  - PK-H lands the ARCHITECTURE §12 row.
- **Done-line.** `just test-pkg internal/config`; `just test-pkg cmd`.

### B-08 [Note] — `queue_test.go`: one duplicate test, and two that depend on timing

- **Evidence.**
  - `TestQueueOneTokenManyItems` (`:455`) runs 500 items on one token. An item that never runs
    also shortens `TestQueueTokensInLaunchOrder`'s recorded order (`:148`).
  - `TestSlotWaitedReportsTheWait` (`:350`) fails on `waited < hold/2` with `hold` at 50 ms
    (`:367`).
  - `TestQueueGoEndedContext` (`:179`), case "ended while Go waits", calls
    `time.AfterFunc(20*time.Millisecond, cancel)` (`:199`). If that fires before `Go` blocks, the
    case passes through `Go`'s pre-check instead.
- **Guidance.**
  - Delete `TestQueueOneTokenManyItems`.
  - `TestSlotWaitedReportsTheWait`:
    - record `releasedAt := time.Now()` in the `AfterFunc` callback, before `close(release)`;
    - assert `waited >= releasedAt.Sub(before) - time.Millisecond && waited <= blocked`;
    - drop the `hold/2` floor.
  - `TestQueueGoEndedContext`:
    - delete the "ended while Go waits" case;
    - flatten what remains, removing the table, the `tokenHeld` field and both `tokenHeld`
      branches;
    - the test then cancels, calls `Go`, and asserts `context.Canceled` and that the item never
      started;
    - say exactly that in its doc comment.
- **Done-line.** `just test-pkg internal/model`.

### B-09 [Note] — `modeltest.MockClient.Consult` is exported for one in-package caller

- **Evidence.**
  - `git grep -n '\.Consult(' -- '*.go'` answers `mockllm.go:203` (inside `ConsultStream`) and
    `mockllm_external_test.go`. Nothing else calls it.
- **Guidance.**
  - Rename the method `Consult` to `consult`.
  - Move every case of `mockllm_external_test.go` that calls it into `mockllm_test.go`
    (`package modeltest`). Keep each case's claim.
  - In `mockllm.go`'s comments, change every mention of the method `Consult` to `consult`.
  - Leave the names that contain "Consult" as they are: `ConsultStream`,
    `NewScriptedMockPerConsult` and `perConsult`.
- **Done-line.** `just test-pkg internal/model`.

---

## PK-D — score run tidy-ups

### B-10 [Note] — `prepare` stamps `Started`, and `attempt` overwrites it on every launched item

- **Evidence.**
  - `grep -n 'Started: Stamp()\|e.Started = Stamp()' internal/score/run.go` answers 285 and 308.
  - Only `prepare`'s `refused` path keeps the first stamp.
- **Guidance.**
  - Drop `Started: Stamp()` from the entry literal at 285.
  - In `refused`, set `e.Started, e.Ended = Stamp(), Stamp()`.
- **Done-line.** `just test-pkg internal/score`.

### B-11 [Note] — `landBatch` loops forever on a `Refused` result that names no item

- **Evidence.**
  - `refusedEntries` starts `named := map[int][]string{}` and returns it, empty and not nil,
    when `refusals` is empty (`sed -n 343-352p internal/score/invoke.go`).
  - `landBatch` tests `named == nil` (`grep -n 'named == nil' internal/score/invoke.go` answers
    312 and 318).
  - So `rest` equals `batch`, and the loop re-issues the batch with the KB write lock held.
  - Nothing produces an empty refusal today. `internal/write` returns its refusals only when
    there are some.
- **Guidance.**
  - Test `len(named) == 0` at both sites.
  - Update `refusedEntries`' doc comment.
- **Done-line.** `just test-pkg internal/score`.

### B-12 [Note] — `Ledger.Append` has no production caller, and its doc describes the retired per-item path

- **Evidence.**
  - `git grep -n '\.Append(' -- internal cmd` answers only `ledger_test.go` and `status_test.go`.
    Both are `package score`.
- **Guidance.**
  - Delete `Append` (`grep -n 'func (l \*Ledger) Append' internal/score/ledger.go` answers 331).
  - Add to `ledger_test.go`:
    `func appendLine(l *Ledger, it query.Item, e Entry) error { return l.record([]attempt{l.line(it, e)}) }`.
  - Every `l.Append(it, e)` in both test files becomes `appendLine(l, it, e)`.
- **Done-line.** `just test-pkg internal/score`.

### B-13 [Note] — a test's name no longer says what it asserts

- **Guidance.** Rename `TestRunStopsAtAnItemBoundaryOnCancel` (`internal/score/run_test.go:463`) to
  `TestRunCancelRecordsWhatWasAnswered`. Its doc comment already states that claim.
- **Done-line.** `just test-pkg internal/score`.

---

## PK-E — write refusals name their entries

### B-14 [Warning] — record-op refusals carry no `entry`, and stop at the first offending entry

- **Evidence.**
  - `editRecords` returns on the first entry whose apply fails: `sed -n 160,164p internal/write/records.go`,
    `for _, e := range entries { if err := edit.apply(b, e); err != nil { return outcome{}, b, err } }`.
  - None of the refusals that apply raises sets `Entry`:
    - `noRecord` (`:244-247`);
    - `setBearing`'s demoted refusal (`:329-330`);
    - its illegal-relation refusal, through `opRefusal` (`ops.go:187-188`).
  - SPEC §7 "Item" says: "A refusal lists every offending item", and `entry` is the "1-based index
    into the values' `entry` list".
  - SPEC §11 says: "A refusal over MCP carries `entry` as `1`".
- **Location.**
  - `editRecords`;
  - every other apply loop: `git grep -n 'range entries' -- internal/write`.
- **Guidance.**
  - In each apply loop:
    - collect each entry's `result.Refusal`, and set `Entry: e.Index` on any item whose `Entry`
      is 0;
    - continue to the next entry;
    - after the loop, return everything collected as one refusal.
  - An error that is not a `result.Refusal` still returns at once.
  - A refused batch still writes nothing.
  - Move each `internal/write` test truth that pinned an entry-less or single-item refusal, and
    report each one.
  - Add this test, which fails today: a `set-bearing` batch whose entries 1 and 3 name no edge
    record is refused with two items, entries 1 and 3.
  - `TestLandValuesFailsARefusedEntryAndLandsTheRest` (`internal/score/invoke_test.go:349`) has a
    case, "a refusal naming none", which uses an unknown node. That node now names its entry.
    - Name a producer of a refusal that still names no entry: `git grep -n 'result.Refusal{{\|result.Item{' -- internal/write`,
      then keep the hits that set no `Entry`.
    - If such a producer is reachable from a landing, make it the case.
    - If none is, delete the case and report that.
    - Keep `landBatch`'s per-value path either way.
- **Contract text.** PK-H lands it.
- **Done-line.** `just test-pkg internal/write`; `just test-pkg internal/score`.

---

## PK-F — the asks call path

### B-15 [Warning] — a call cancelled in flight still writes its capture

- **Evidence.**
  - `grep -n 'captureErr := cl.holdIn' internal/asks/call/call.go` answers 217 (`decide`) and 483
    (`ask`).
  - Each is followed by `switch { case ctx.Err() != nil: ... return ..., ctx.Err()`. So the
    capture is held before the cancel is seen, and `goHolding`'s after writes it
    (`sed -n 107,121p`).
  - SPEC §6 `kbase cancel` says: "The calls in flight are abandoned with nothing written for
    them".
- **Guidance.**
  - In both functions, directly before `holdIn`, add:
    `if err := ctx.Err(); err != nil { return <zero>, err }`.
    - In `decide`, move the comment of the existing ctx case ("The call was stopped, not
      failed…") onto it.
  - Delete the `case ctx.Err() != nil` arm of each later `switch`. Nothing between the early
    return and the `switch` can be cancelled.
  - In `TestDecideReturnsItsContextsEnd` (`call_test.go:382`), assert that after `wg.Wait()`
    `filepath.Glob(filepath.Join(<the trail directory the test's Caller is given>, asks.CapturesDir, "*"))`
    finds nothing. Since B-01, captures live in the trail directory.
- **Done-line.** `just test-pkg internal/asks`.

### B-16 [Note] — the overview launch's function type is spelled out in full at three sites

- **Location.**
  - `grep -n 'type overviewLaunch' internal/build/build.go` answers 133.
  - `grep -n 'func (c \*Caller) PassageLaunch' internal/asks/call/call.go` answers 89 (the result
    type) and 90 (the inner literal).
  - `internal/build/build_test.go:1294` (`inlineOverview`).
- **Guidance.**
  - Declare
    `type PassageLaunch func(ctx context.Context, wg *sync.WaitGroup, item func(passage func(ctx context.Context, excerpts string, rejected []string, attempt int) (string, error)) (after func(writeErr error))) error`
    in `internal/asks/letters.go`, directly after `Launch`. Its doc comment is adapted from
    `overviewLaunch`'s.
  - `Caller.PassageLaunch` returns `asks.PassageLaunch`.
  - `walk.passage` and `inlineOverview` use `asks.PassageLaunch`.
  - Delete `overviewLaunch`.
- **Done-line.** `just test-pkg internal/asks`; `just test-pkg internal/build`.

### B-17 [Note] — `ask` tracks the last attempt by hand

- **Evidence.**
  - `grep -n 'var got model.ChatAttempt\|got = a\|got.Error' internal/asks/call/call.go` answers
    471, 481 and 488.
  - `got` is read only on the `ErrChatIncomplete` path. There, `chat` returns the last observed
    attempt (`chat.go:118-121`), so it equals `res.Final` (B-05).
- **Guidance.** Read `res.Final.Error` at 488, then delete `got` and its assignment.
- **Done-line.** `just test-pkg internal/asks`.

### B-18 [Warning] — `TestACaptureRecordsItsTokenWait` fails without a defect when the scheduler is slow

- **Evidence.**
  - The sleeper goroutine starts before the second `launch`
    (`sed -n 428,437p internal/asks/call/call_test.go`).
  - The test then requires S2's wait to be at least `hold - hold/5`, which is 80 ms
    (`grep -n 'floor :=' internal/asks/call/call_test.go`).
  - A deschedule of more than 20 ms between the two fails the test with no defect present.
- **Guidance.**
  - Take `before := time.Now()` directly before the second `launch`.
  - Inside the sleeper, take `releasedAt := time.Now()` directly before `close(release)`.
  - Assert S2's wait is at least `(releasedAt.Sub(before) - time.Millisecond).Milliseconds()`, and
    that S1's wait is less than S2's.
  - Drop `floor`.
- **Done-line.** `just test-pkg internal/asks`.

---

## PK-G — concurrency evidence tests

### B-19 [Warning] — the in-flight tests need the scheduler to reach `peak == 2` within a 20 ms hold, and one comment claims more than its test exercises

- **Evidence.**
  - `internal/build/concurrentasks_test.go:155` reads
    `newWire(func(string) time.Duration { return 20 * time.Millisecond })`, and `:157` reads
    `if w.peak != 2`.
  - `cmd/concurrentasks_test.go:113` and `:115` have the same pattern.
  - The build test's comment says "across both roles" (`:152`). The overview runs in its own
    stage, after every letter ask has drained.
- **Guidance.**
  - Give each provider double a gate in place of the sleep:
    - the first call waits until a second call has entered, or until 5 s pass;
    - every later call passes straight through.
  - Assert that `peak` reached 2 and never went above it.
  - The build test's comment becomes "at most N letter asks in flight".
- **Done-line.** `just test-pkg internal/build`; `just test-pkg cmd`.

### B-20 [Note] — two cmd tests duplicate unit tests, and the build N=1/N=4 test never checks that it scrambled anything

- **Evidence.**
  - `cmd/concurrentasks_test.go:97` scrambles by
    `time.Duration(len(prompt)%4) * 5 * time.Millisecond`. Nothing checks that the order changed.
  - `TestRunRecordsTheSameWhateverOrderItsItemsComplete` (`internal/score/run_test.go:563`) forces
    a full reversal and compares the same two files.
  - `TestAScoreRunAsksInSubmissionOrder` (`:122`) is covered by two tests together:
    `TestRunLaunchesItsItemsInSetOrder` (`run_test.go:601`) and `TestQueueTokensInLaunchOrder`
    (`internal/model/queue_test.go:148`). `scoringLaunch` calls `queue.Go` directly
    (`cmd/score.go:375`).
  - `TestABuildAtOneAndFourLandsTheSameKB` (`internal/build/concurrentasks_test.go:141`) is the one
    check of order independence across all stages. The claimgraph test covers `Discovered` alone.
    The build test uses the same length-based scramble (`:142`), unchecked.
- **Guidance.**
  - Delete both cmd tests. Their survivors are named above.
  - Delete any helper they leave unused. `TestAScoreRunHoldsNCallsInFlight` stays.
  - In the build test:
    - assert from `w.events` at N=4 that at least one ask returned out of the order the asks
      arrived;
    - on failure, report "nothing arrived out of order";
    - if the scramble cannot guarantee that, make the double's hold reorder deterministically.
- **Done-line.** `just test-pkg internal/build`; `just test-pkg cmd`.

### B-21 [Warning] — contract behaviour that no test exercises

- **Evidence.**
  - SPEC §8 (`:835`) says "`score`'s does not" carry `--ask-concurrency` on resume. But
    `git grep -n 'ask-concurrency' -- cmd/score_test.go` prints nothing.
  - CONVENTIONS "Testing" (`:477-479`) says `build` refuses `--ask-concurrency` under 1 "before it
    resolves and creates its state store, and those refusals carry no `state-dir`".
    `TestBuildAndScoreRefuseAnAskConcurrencyUnderOne` (`cmd/askconcurrency_test.go:56`) asserts
    neither.
  - SPEC §9.2 says a file-mode `build` or `score` names the superseded key in one stderr line.
    Only `score` is tested (`:82`).
- **Guidance.** All three changes go in `cmd/askconcurrency_test.go`.
  - Assert that `scoreResume` with `AskConcurrency` set returns no `--ask-concurrency`.
  - In the build subtest:
    - set `StateDir` to an absent path under `t.TempDir()`;
    - assert the document has no `state-dir` key;
    - assert the path still does not exist.
  - Make the superseded-key test a table over both verbs. For `build`, call
    `buildProvider(...)(reg)` with a file-mode `Config` that states the key, and count the line
    on its stderr.
- **Done-line.** `just test-pkg cmd`.

---

## PK-H — contract text

Find each anchor with its command, and land the text as given. Run no Go target. The done-line is
each check below. No runner target covers prose; the wave-end gate runs `just edit-gate`.

### B-22 [Warning] — ARCHITECTURE describes score wiring that no longer exists

- **Evidence.**
  - `cmd/score.go:276` builds `inv.Launch` with `scoringLaunch`.
  - `scoringLaunch` calls `reg.Get(ep)` first (`:360`), before `score.Invoke` (`:283`).
  - `score.Runner` has `Launch Launch` (`internal/score/run.go:128`).
  - `prepare` returns `(Entry, *call)` (`:284`).
  - `attempt` returns `(_ Entry, interrupted bool)` (`:307`).
  - `git grep -n 'scoringAsk' -- '*.go'` prints nothing.
- **Text.**
  - **§4.** `grep -n 'with a nil letter reader' ARCHITECTURE.md` answers 85. "with a nil letter
    reader" becomes "with a nil `Launch` (`claimgraph.Options.Launch`), which is what not asking
    is".
  - **§6, the runner paragraph.** `grep -n '.Ask. seam, a .Write. seam' ARCHITECTURE.md` answers
    555 ("an" ends line 554).
    - "an `Ask` seam" becomes "a `Launch` seam, which hands each item the `Ask` its one call goes
      through".
    - The clause from "`attempt` returns, beside the entry, the `call` it made" through "nil where
      it made none —" becomes: "`prepare` returns, beside the entry, the `call` the item will
      make — class, closed text and prompt, nil where the class cannot be derived or the guard
      refuses the text — and `attempt` returns the entry and whether the walk's stop interrupted
      the call;".
  - **§6, "The ask is one seam."** `grep -n 'The runner.s .Ask. takes' ARCHITECTURE.md` answers 571.
    - "The runner's `Ask`" becomes "Each item's `Ask`".
    - Replace the text from "`scoringAsk` in `cmd/score.go` fills the seam" through "the
      truncation the runner judges." with:
      > `scoringLaunch` in `cmd/score.go` fills the `Launch` seam: it opens the scoring role's
      > client through the registry — so a key-file refusal comes after the start refresh and
      > before `score.Invoke`'s index load — builds the invocation's one `model.Queue`, and hands
      > each item an `Ask` that makes one `Slot.Chat` call on the role's endpoint with an empty
      > system prompt, the item's `--timeout` applied from the call's start, reporting
      > `model.FinishLength` as the truncation; the runner learns nothing of what answered. The
      > development replay takes its launch from the same function, so a replayed call carries
      > the run's endpoint, timeout and the truncation the runner judges.
  - **§6, "The run's entry."** `grep -n 'or the .Ask. from the constructor' ARCHITECTURE.md`
    answers 790.
    - "or the `Ask` from the constructor the verb supplies" becomes "or the walk over the launch
      the verb built (`Invocation.Launch`)".
    - "whether the `Ask` was opened (`Asked`)" becomes "whether the invocation reached the route
      its calls take — not a dry run, so `Launch` was handed to the walk (`Asked`)".
  - **§6, "The verb."** `grep -n 'given the registry and .scoringAsk.' ARCHITECTURE.md` answers 808.
    "then `score.Invoke`, given the registry and `scoringAsk` as its `Ask` constructor; `roles`
    where the report says the ask was opened" becomes "then the launch, `scoringLaunch`, where a
    key-file refusal ends the run; then `score.Invoke`, given that launch; `roles` where the
    report's `Asked` says the walk was handed it".
  - **§6, the replay.** `grep -n 'built by .scoringAsk.' ARCHITECTURE.md` answers 843.
    - "the `Ask` built by `scoringAsk`" becomes "the launch built by `scoringLaunch`".
    - At 846, "sends it through `Ask`" becomes "sends it through the `Ask` its launch hands the
      item".
  - **§11.** `grep -n 'below its .Ask. seam' ARCHITECTURE.md` answers 1458. "below its `Ask` seam"
    becomes "below its `Launch` seam".
- **Check.** `grep -n 'scoringAsk\|constructor the verb\|.Ask. was opened\|nil letter reader' ARCHITECTURE.md`
  prints nothing.

### B-23 [Warning] — the import-policy test enforces `internal/model` rules that ARCHITECTURE §11 does not state

- **Evidence.**
  - `cmd/importpolicy_test.go:74-75` reads `from: []string{"internal/model"}, allowed:
    []string{"internal/model"}`.
  - `TestModelImportsNoOS` is at `:501`.
  - `grep -n 'model. imports no\|model imports' ARCHITECTURE.md` prints nothing.
  - ARCHITECTURE §11 calls its rules "the whole rule".
- **Text.** `grep -n '^- Only .asks/call., .build., .modelclient. and three .cmd. files import .model.' ARCHITECTURE.md`
  answers 1439. Insert this bullet directly before it:
  > - `internal/model` imports no kbase package and no `os`: it touches no file and knows no role
  >   or consumer (`cmd/importpolicy_test.go`, `TestModelImportsNoOS` for `os`).
- **Check.** `grep -n 'imports no kbase package and no .os.' ARCHITECTURE.md` answers one line.

### B-24 [Note] — SPEC states the `--ask-concurrency` sentence twice, and gives a misleading reason

- **Evidence.**
  - `grep -n 'since it changes no judgement' SPEC.md` answers 158 and 835. The sentence is
    identical at both.
  - `scoreResume` (`cmd/score.go:476-495`) carries `--pass`, `--resume`, `--state-dir`,
    `--config-dir`, `--scoring-model` and `--rescore-manual`. It carries neither `--timeout` nor
    `--limit`, which change no judgement either.
- **Text.**
  - **§5.** At the first hit (the sentence opens at 157), the sentence becomes:
    > A value of `--ask-concurrency` under 1 is a usage refusal naming `--ask-concurrency`, and
    > `build`'s `resume` carries it.
  - **§8.** At the second hit (the sentence opens at 834), it becomes:
    > `--ask-concurrency` under 1 is refused as `build`'s is (§5); `score`'s `resume` does not
    > carry it, nor `--timeout` or `--limit`: each shapes one invocation, not the run.
- **Check.** `grep -c 'since it changes no judgement' SPEC.md` answers 0.

### B-25 [Note] — a plan package that changes an interface misses the files that build a ported double elsewhere

- **Evidence.**
  - `cmd/score_replay_test.go:147` builds `cancelOnSecond`, which `cmd/score_test.go:175`
    defines.
  - `git grep -l 'cancelOnSecond' -- '*.go'` answers three files.
- **Text.** In CONVENTIONS "### Plan dispatchability" (`grep -n '### Plan dispatchability' CONVENTIONS.md`
  answers 370), add this after the bullet "Carries the command and its answer…":
  > - A package that changes a type, a signature or a test double names in its scope every file
  >   that references it — `git grep -l '<Name>' -- '*.go'`, test files included — and carries
  >   that command and its answer
- **Check.** `grep -n "git grep -l '<Name>'" CONVENTIONS.md` answers one line.

### README text for B-01

- **Evidence.** `grep -n 'scratch/captures/. older than' README.md` answers 227: "Each build prunes
  `reports/build-*` and `scratch/captures/` older than the fifth-latest ended build, and keeps the
  answer cache of the current build trail and the five before it."
- **Text.** That sentence becomes:
  > Each build keeps the newest six captured outputs of builds the MCP server started
  > (`reports/build-*`), and the answer cache, letter-ask records and call captures of the current
  > build trail and the five before it.
- **Check.** `grep -c 'scratch/captures' README.md` answers 0.

### Contract text for B-28

- **SPEC §6.** The sentence beginning "At a scoring run's start, once its own run directory stands"
  (`grep -n "At a scoring run's start" SPEC.md` answers 344). Append this after its last clause,
  "captures included.":
  > The captured output of scoring runs the MCP server started (`reports/score-*`, §11) is pruned
  > at the same point to the newest 16 by the start each one's name records.
- **ARCHITECTURE §8 "Retention".** After "the rest whole (§6, Scoring)." (`grep -n 'the rest whole (§6, Scoring)' ARCHITECTURE.md`
  answers 1130), add:
  > It keeps the newest `KeptRuns` (§12) captured runs under `reports/score-*`, by name, and
  > removes the rest.
- **Check.** `grep -n 'reports/score-' SPEC.md ARCHITECTURE.md` answers both lines.

### Contract text for B-07 and B-14

- **B-07.** `grep -n '| Ask concurrency' ARCHITECTURE.md` answers 1495. "default 2" becomes
  "default `config.DefaultAskConcurrency`, 2".
- **B-14.** `grep -n 'the missing-record, illegal-relation and demoted refusals' ARCHITECTURE.md`
  answers 765. "where it names none (the missing-record, illegal-relation and demoted
  refusals)," becomes "where it names none,".
- **Check.** `grep -c 'illegal-relation and demoted refusals' ARCHITECTURE.md` answers 0.

---

## PK-L — scoring runs' captured output

### B-28 [Warning] — `reports/score-*` is never pruned

- **Evidence.**
  - The MCP server writes each score run's output under `reports/` with prefix `score-`:
    - `internal/mcp/launch.go:75` (`capturePrefix: score.CapturePrefix`);
    - `:153` (`os.CreateTemp(reports, l.capturePrefix+…)`).
  - `git grep -n 'CapturePrefix' -- internal cmd` shows the build's prune reading only
    `buildstore.CapturePrefix` (`internal/build/store.go:376`). Nothing reads
    `score.CapturePrefix` to prune it.
- **Guidance.**
  - Where `PruneRuns` runs (`grep -n 'func PruneRuns' internal/score/ledger.go` answers 771), also
    prune `<state-dir>/reports/score-*`:
    - a capture is the `.stdout.yaml`/`.stderr.log` pair sharing a stem;
    - keep the newest `KeptRuns` by name and remove the rest.
  - Add `TestPruneRunsKeepsTheNewestSixteenScoreCaptures` to `ledger_test.go`: of 18 captured runs,
    the newest 16 pairs stand. This fails today.
  - PK-H lands the contract text.
- **Done-line.** `just test-pkg internal/score`.

---

## PK-M — `just dist --tag`

### B-29 [Note] — `just dist` tags the release it built

- **Requirement.** `just dist` takes an optional `--tag`. Given it, once the dist build has completed
  successfully — every stage of `build-dist.sh`'s `main`, signing included — it runs
  `git tag v<major>.<minor>.<patch>`, the version the build already resolved. Without it, nothing
  changes. The tag is lightweight and local: nothing is pushed.
- **Facts.**
  - The recipe today: `just --show dist` answers
    `dist:` / `@BIN_DIR="{{ BIN_DIR }}" DIST_DIR="{{ DIST_DIR }}" tools/scripts/build-dist.sh`.
  - The version: `tools/scripts/build-dist.sh` `resolve_release_names` sets
    `ver="v$(go run ./cmd --version)"` and refuses anything not `v<major>.<minor>.<patch>`.
  - `main` runs `set_defaults build_targets resolve_release_names stage_release sign_staged
    make_tarball sign_tarball write_installer sign_installer report`.
  - `git tag` answers `v0.5.0`, `v0.6.0`, `v0.8.0`: a tag for the current version may already exist.
  - `just --version` answers `just 1.59.0`.
  - `test-dist` runs `tools/scripts/test-build-dist.sh`, which drives `build-dist.sh` against stub
    `go`, `gpg`, `curl` and `uname`.
- **Guidance.**
  - CONVENTIONS "A `justfile` recipe whose body is a shebang script…" governs the recipe: the recipe
    passes the script its input as an environment variable, `TAG`. `TAG` is optional: the script
    reads it as `${TAG:-}`, as it reads `BIN_DIR` and `DIST_DIR`, and only `TAG=1` tags; every
    existing caller (`just dist` without the option, `dist-rc`, the `build_release` calls in
    `test-build-dist.sh`) sets none and behaves as today. Spell the recipe's option in the form
    `just 1.59.0` supports for a long flag; if it supports none, use a recipe parameter and say so in
    the report.
  - Move `resolve_release_names` ahead of `build_targets` in `main`: it needs only
    `go run ./cmd --version`, and the move makes the version refusal fire before any build too.
  - With `TAG=1`, straight after `resolve_release_names` and before `build_targets`, refuse in the
    script's existing refusal style ("dist: check: usage: …; nothing was staged or signed."):
    - where the tag already exists;
    - where tracked files have uncommitted changes, since the tag would name a commit that did not
      build the release.
  - The tag step runs last, after `report`, and only on success (`set -euo pipefail` already stops
    the script on a failed stage).
  - The recipe's `[doc(…)]` names the option.
  - **Test isolation.** `build-dist.sh` always runs from this repository's root (it `cd`s to the
    root its own path names), so isolation is a stub `git` at `tools/scripts/testdata/stubs/git`,
    beside the existing stubs, which records its calls and answers the tag-exists and
    uncommitted-changes queries as each case sets. `test-build-dist.sh` adds cases for:
    - `TAG=1`: the stub records `tag v<version>` after the build's last stage;
    - `TAG=1` with the tag existing, or with uncommitted changes: refused, nothing built (the stub
      `go` records no build);
    - no `TAG`: the stub records no tag call.

    No case may run the real `git` against this repository.
- **Done-line.** `bash -n` on both scripts; `just --dry-run dist` and the dry run with the option,
  each rendered line setting every variable `build-dist.sh` requires; `just test-dist` (needs
  `expect`; run by the coordinator at the wave gate).

---

## PK-I — rename the letter-ask vocabulary to multichoice

### B-26 [Note] — rename

- **The item type** is `MultichoiceItem`, matching `MultichoiceQuestion` and `MultichoiceReader`.
- **The YAML key** of the two `asks` record fields becomes `yaml:"choice"`
  (`git grep -n 'yaml:"letter"' -- '*.go'` answers `internal/asks/letters.go:135` and `:144`, plus
  `internal/buildrecords/buildrecords.go:66`, which stays). No production code reads group records
  back (`git grep -n 'GroupRecord' -- '*.go'` shows only test decoders), and no fixture carries the
  key, so only the test decoders move.
- **Code comments** that say "letter ask(s)" or "letter-ask" say "multichoice ask(s)" instead:
  `git grep -n -i 'letter.ask' -- '*.go'` lists them. A comment describing the offered letters as
  the answer's alphabet stays. PK-N does the same in the documents.

- **Guidance.**
  - Run `git mv internal/asks/letters.go internal/asks/multichoice.go`, and
    `git mv internal/asks/letters_test.go internal/asks/multichoice_test.go`.
  - Rename these:
    - `LetterQuestion` → `MultichoiceQuestion`;
    - `LetterReader` → `MultichoiceReader`;
    - `LetterItem` → `MultichoiceItem`;
    - the Go field `Letter` of `asks.CallRecord` and `asks.ItemRecord` → `Choice`. This covers
      every reference, keyed `Letter:` literals included.
  - The two `asks` YAML tags become `yaml:"choice"`.
  - **Files.**
    - `git grep -lwE 'LetterQuestion|LetterReader|LetterItem' -- '*.go'` answers 11 files:
      `internal/asks/{call/call.go,call/call_test.go,kinds.go,letters.go,letters_test.go}` and
      `internal/claimgraph/{commonfiles_test.go,cull_stage_test.go,launch_test.go,modulo_ids_test.go,nodepass_test.go,unmarked_test.go}`.
    - `git grep -lE '\.Letter([^A-Za-z0-9_]|$)' -- '*.go'` answers:
      `internal/asks/{call/call_test.go,letters_test.go}`, `internal/build/inference_test.go`, and
      `internal/claimgraph/{classify.go,cull.go,depends_stage_test.go,livecounts_test.go,stages.go,unmarked.go,unmarked_test.go}`.
    - The keyed `Letter:` literals of the two `asks` types are at `internal/asks/letters.go:342`
      and `:345`.
  - **Leave these alone:**
    - `buildrecords`' own `Letter` field (`internal/buildrecords/buildrecords.go:66`) and its keyed
      literals, such as `buildrecords.CandidateEntry{… Letter: item.Letter …}`. The key there stays
      `Letter`; the read `item.Letter` becomes `item.Choice`.
    - The `asks.Letter*` constants.
    - `MULTI_THREADED_ASKS.md`.
  - The contract documents name none of these symbols:
    `grep -n 'LetterQuestion\|LetterReader\|LetterItem\|letters\.go' SPEC.md ARCHITECTURE.md CONVENTIONS.md`
    prints nothing.
- **Check.** `git grep -nwE 'LetterQuestion|LetterReader|LetterItem' -- '*.go'` prints nothing;
  `git grep -n 'yaml:"letter"' -- internal/asks` prints nothing.
- **Done-line.**
  - `just test-pkg internal/asks`;
  - `just test-pkg internal/claimgraph`;
  - `just test-pkg internal/build`;
  - the check.

---

## PK-N — the documents say "multichoice ask"

### B-30 [Note] — the term "letter ask" leaves the documents

- **Requirement.** The documents follow PK-I's rename: "letter ask(s)" and "letter-ask" become
  "multichoice ask(s)" and "multichoice-ask". A sentence describing the offered letters as the
  answer's alphabet keeps the word "letter" there.
- **Sites.** `git grep -n -i 'letter.ask' -- SPEC.md ARCHITECTURE.md CONVENTIONS.md README.md USER_CONVENTIONS.md`
  answers 6, 15, 1, 1 and 1 lines respectively at HEAD (counts may move after PK-H).
- Leave `MULTI_THREADED_ASKS.md` and `ACTIVE_PLAN*.md` alone.
- **Check.** The same command prints nothing.
- **Done-line.** The check.

### B-31 [Note] — review the tests of touched code

- `CONVENTIONS.md:368`, "Their review brief includes reviewing the tests", becomes "Their review
  brief includes reviewing the tests of all touched code".
- **Done-line.** `grep -c 'tests of all touched code' CONVENTIONS.md` answers 1.

---

## PK-J — rename the git seam to `gitledger`

### B-27 [Note] — rename

- **Guidance.**
  - Run `git mv internal/ledger internal/gitledger`. Change the package clause to
    `gitledger`, and change each qualifier `ledger.` to `gitledger.` in every importer.
  - Importers: `git grep -l '"kbase/internal/ledger"' -- '*.go'` answers nine files:
    - `cmd/{mcp_test.go,init_test.go,init.go}`;
    - `internal/build/{build.go,build_test.go}`;
    - `internal/buildstore/{status_test.go,rewind.go,stages.go,status.go}`.
  - Owner entries: `cmd/execpolicy_test.go:25`, and `cmd/importpolicy_test.go:70, 87, 88, 469`.
  - `internal/score/ledger.go` and the result key `ledger` stay as they are.
  - **ARCHITECTURE.** The package is named at:
    - 403: "and `ledger`" becomes "and `gitledger`";
    - 537: "`ledger.Commit`" becomes "`gitledger.Commit`", and "the plain-subject half of
      `ledger`" becomes "the plain-subject half of `gitledger`";
    - 1038 and 1474: `internal/ledger` becomes `internal/gitledger`;
    - 1043 and 1462: "`ledger`", the package, becomes "`gitledger`". At 1462 that is both
      mentions;
    - 1271: "(the ledger's owned paths derive from them)" becomes "(the owned paths
      `gitledger.Open` is given derive from them)". `buildstore.OwnedPaths` is
      `append([]string{kb.KBDir}, buildrecords.RecordFiles...)` (`internal/buildstore/stages.go:50`),
      handed to `ledger.Open` (`status.go:44`);
    - 1284: the package-map name becomes `internal/gitledger`, and its column stays aligned.

    Every other `ledger` hit in `grep -n 'ledger' ARCHITECTURE.md` is a scoring run's ledger or
    the result key. Leave those.
  - CONVENTIONS names the package nowhere: `git grep -n 'internal/ledger' -- CONVENTIONS.md` prints
    nothing.
- **Check.** `git grep -n 'internal/ledger' -- '*.go' ARCHITECTURE.md CONVENTIONS.md` prints
  nothing. At `644fdbc` it answers 3 ARCHITECTURE lines and the Go lines listed above.
- **Done-line.**
  - `just test-pkg internal/gitledger`;
  - `just test-pkg internal/build`;
  - `just test-pkg internal/buildstore`;
  - `just test-pkg cmd`;
  - the check.
