# ACTIVE PLAN – multi-threaded asks

## Status

**Dispatchable.** The plan has had its prompt-engineer review and the architect's revision on it.

## Design of record

[`MULTI_THREADED_ASKS.md`](MULTI_THREADED_ASKS.md) is the design this plan executes:

- §0 is settled.
- §1 states the queue, the slot, the consumer seams and the idle-connection fix.
- §3 holds the invariants every package keeps.
- §5 holds the phase inventories and the mechanisms of the per-trail directory and progress rotation.
- §6 holds the acceptance criteria (AC 1–15) the sprint exits on.
- §7 holds the replacement text for every contract change.

Three rules govern how the plan and the design relate:

- **The design governs.** Where a row of this plan and the design disagree, the work implements the
  design, and the checkpoint commit reports the plan line as moved.
- **A package does not edit the design.** A defect found in it goes to the architect.
- **The plan does not restate design text.** A package names the design sections and ACs that carry
  its text, and the dispatch carries those.

## Package status

The coordinator updates this table as packages finish.

| Package | Status |
|---|---|
| P1 | done (C1) |
| P2.1 | done (W2) |
| P2.2 | done (W3) |
| P2.3 | done (W2) |
| P2.4 | done (W2) |
| P2.5 | done (W3) |
| P2.6 | done (W3) |
| P2.7 | done (W4) |
| P2.8 | done (C2) |
| P2.9 | done (W4) |
| D2 | done (C2) |
| A1 | done (W6) |
| A2 | done (W7) |
| A3 | done (W6) |
| A4 | done (W6) |
| A5 | done (C3) |
| A6 | done (C3) |
| DA | done (C3) |
| B1 | done (C4) |
| B2 | done (C4) |
| B3 | done (C4) |
| DB | done (C4) |

## Outcome

- Every model call an invocation makes goes through a `model.Slot`. Only the invocation's one
  `model.Queue` issues a slot, from `Go`, and only while one of its N tokens is free.
- Within one consumer, items take their tokens in the order they were submitted.
- An item holds its token across all of its calls, and releases it before it writes a file.
- While any ask is in flight, the only files written are per-result files. Every common file is
  written after its ask batch drains. Design §0 item 4 defines "ask batch" and "common file".
- Each build trail's answers and group records live in one directory per trail, and the build keeps
  the current trail's and five others.
- `progress.jsonl` is rotated at a fresh build's start once it outgrows about five builds.
- The sync call path is unexported, so the compiler holds every consumer to a slot.

## Principles that steer open choices

- Design §3 binds every package. Names it leaves open are the coder's (§3 "Deliberately
  unspecified").
- Use the standard library only. `golang.org/x/sync` is not sanctioned (CONVENTIONS "Dependency
  Policy"), and `grep -n 'golang.org/x/sync' go.mod go.sum` prints nothing.
- Choose the boring mechanism: a buffered channel of tokens, and a `sync.WaitGroup`.
- Add a test only where it pins a stated invariant or AC, or the defect a change closes.
- A test that pins a per-unit write moves with the contract, and the package's report states the
  truth it now tracks (CONVENTIONS "Coordinator Policy").

## Rules for every package

Every dispatch carries this section and "Principles that steer open choices", beside the package's
own row.

- **Pre-task reading.**
  - THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md;
  - the seat's own definition;
  - the design sections and ACs the package names.
- **Scope.** A package edits only the files and symbols its scope names. A change it needs anywhere
  else is reported, not made.
- **Line numbers.** Line numbers are as of commit `aee74c1`, and the quoted text is the anchor.
  Re-run each command a package carries. A moved line is expected. A quoted line that no longer
  exists is reported, not guessed.
- **Model.**
  - A mechanical package runs at `model: sonnet`.
  - An expert package runs on the coordinator's model (AGENTS.md "Dispatch planning").
- **Gates.**
  - A package's done-line is `just test-pkg <pkg>` for each package it touches, plus the checks its
    row names.
  - It runs no whole-repository target: not `just edit-gate`, `just fmt`, `just test`,
    `just test-race`, or any integration recipe. Those are the wave-end gate's.
  - It invokes no toolchain directly: no `go test`, `go vet` or `gofmt`.
- **Report.** The package's report names:
  - every test it added, moved or deleted, and the claim each one pins;
  - every symbol its row tells it to report.
- **Shell.**
  - Call `command rm`, `command cp`, `command mv` and `command ls`, never the bare names.
  - Scratch goes in `.claude-temp/`, never `/tmp`.
  - Never run a git command that rewrites the working tree.

## Waves, gates and checkpoints

Packages in one wave run in parallel, because their files are disjoint. A wave starts when every
package it depends on has finished.

**The wave-end gate.** It is the coordinator's, not a package's. At each wave's end, after auditing
each package's diff against CONVENTIONS "Coordinator Policy", the coordinator runs these in order,
all green:

1. `just edit-gate`; on a format finding, `just fmt`, then `just edit-gate` again;
2. `just test-race`;
3. `just test`;
4. `just test-integration`;
5. the extra recipes the wave names.

**Checkpoints.** The coordinator commits a checkpoint only at the four waves that close a phase or a
step: C1–C4. This plan's rule is that each checkpoint carries the contract-document changes its
code makes true.

- C2, C3 and C4 each carry their doc package.
- C1 needs none: phase 1 adds code nothing calls, and invalidates no contract text.
- At every other wave, the gate runs and nothing is committed.
- A checkpoint commit's body names every fixture or test truth it moved and what that now answers,
  and any plan row the design overrode.

| Wave | Packages | Serialized because | Wave-end extras | Commits |
|---|---|---|---|---|
| W1 | P1 | — | — | C1 (phase 1) |
| W2 | P2.1 ∥ P2.3 ∥ P2.4 | — | — | — |
| W3 | P2.2 ∥ P2.5 ∥ P2.6 | P2.2 edits `internal/asks/call/call_test.go` after P2.1; P2.5 and P2.6 use the symbols P2.4 reports | — | — |
| W4 | P2.7 ∥ P2.9 | P2.7 edits `call.go`, `call_test.go`, `internal/build/rows.go` and `internal/build/store.go` after P2.1, P2.2 and P2.3 | — | — |
| W5 | P2.8 ∥ D2 | P2.8 edits `internal/build/build.go` and `store.go` after P2.7; D2 describes all of phase 2's code | `just test-integration-build-arxiv`, `just test-integration-monitor-arxiv` | C2 (phase 2, on sync calls) |
| W6 | A1 ∥ A3 ∥ A4 | needs C2 | — | — |
| W7 | A2 | fills the seams A3 and A4 declare, and needs A1's key | — | — |
| W8 | A5 ∥ A6 ∥ DA | A5 needs every consumer on the queue; A6 edits the `cmd` registry lines A2 rewrote; DA describes A1–A6, the `Endpoint` field's name being the design's | `just test-integration-mcp-build` | C3 (step A) |
| W9 | B1 ∥ B3 | **needs C3, the commit holding A5's evidence** | `just test-integration-config` (also in the omnibus; named here because B3 moves its fixture) | — |
| W10 | B2 | edits `internal/asks/call/call_test.go` after B1 | — | — |
| W11 | DB | it describes step B's code | `just test-integration-build-arxiv`, `just test-integration-monitor-arxiv` | C4 (step B) |

**The pandoc recipes.**

- `pandoc --version` on this host answers `pandoc 3.12.1`.
- Each id in `BUILD_ARXIV_IDS` and `MONITOR_ARXIV_ID` already has a directory under
  `test-data/transient/arxiv/`, so their prep step makes no fetch.
- They cover `--no-inference` builds, kill and resume, and cancel and resume, over the stage code that
  phases 2 and 3 move.

---

## Phase 1 — the queue

### P1 — `model.Queue` and `model.Slot`

- **Rating.** Expert. **Seat.** go-coder. **Wave.** W1, alone.
- **Scope.**
  - new `internal/model/queue.go`, holding `Queue`, `NewQueue`, `Go` and `Slot`;
  - new `internal/model/queue_test.go`;
  - in `cmd/importpolicy_test.go`, a rule for `internal/model`.
  - Nothing else: no consumer, no key, no flag.
- **Design.** §0 items 1, 5 and 9. §1 "Shape". §3 inv. 2, 3 and 4, and "Standing invariants". §5
  "Phase 1". AC 6.
- **Facts.**
  - Nothing of the kind exists yet: `grep -rn 'type Slot\|type Queue\|func NewQueue'
    internal/model` prints nothing.
  - `Slot.Chat` runs today's loop unchanged: `internal/model/chat.go:75` `func Chat(ctx
    context.Context, c Client, modelID, system, prompt string, backoff []time.Duration, observe
    func(ChatAttempt)) (ChatAttempt, error) {`. It collects the attempts with `observe`, which is
    `model`'s own code.
  - `Slot.Decision` calls the interface method, declared at `internal/model/client.go:35`
    `Decision(ctx context.Context, req DecisionRequest) (DecisionResponse, error)`.
  - The import-policy test has no `internal/model` rule today. Its rules take this shape, at
    `cmd/importpolicy_test.go:70-72`: `from: []string{"internal/ledger"}`, `allowed:
    []string{"internal/log"}`, `why: "ledger imports log and nothing else of kbase"`.
  - Its module walk is `cmd/modulescan_test.go:37` `func moduleGoFiles(t *testing.T, fset
    *token.FileSet) []moduleGoFile {`, with `:98` `func importedAs(f *ast.File) map[string]string {`.
  - `internal/model` imports no kbase package and not `os` today. `go list -f '{{join .Imports
    "\n"}}' ./internal/model | grep -E '^kbase/|^os$'` printed nothing at `aee74c1`.
- **Work.**
  - Design §1 "Shape", as stated there.
  - In `cmd/importpolicy_test.go`:
    - a rule that `internal/model` imports no kbase package;
    - an assertion, over the module walk, that no production file of `internal/model` imports `os`.
- **Tests.**
  - One test per claim in design §5 "Phase 1", nine in all.
  - The import-policy rule and assertion above.
- **Done-line.** `just test-pkg internal/model`, `just test-pkg cmd`.

---

## Phase 2 — per-result files during asks, common files after (sync calls kept)

No package in this phase changes a call site of `model`.

### P2.1 — a call's capture and answer, written once after the call

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W2.
- **Scope.**
  - in `internal/asks/call/call.go`, `decide`, `ask` and the capture records they write;
  - `internal/asks/call/call_test.go`.
- **Design.** §5, the phase-2 table's `decide` and `ask` row. §3 inv. 6.
- **Facts.** `grep -n 'os.Create(capturePath)\|atomicfile.Write(answer' internal/asks/call/call.go`
  answers:
  - `107: f, err := os.Create(capturePath)`
  - `168: if err := atomicfile.Write(answer, mustMarshal(a), nil); err != nil {`
  - `377: f, err := os.Create(capturePath)`
  - `422: if err := atomicfile.Write(answer, []byte(got.Reply), nil); err != nil {`

  The chat attempts reach the open capture through the callback that opens at `:393` `got, err :=
  model.Chat(ctx, c.client, modelID, system, prompt, backoff, func(a model.ChatAttempt) {`. The
  writer's signature is `internal/atomicfile/atomicfile.go:37` `func Write(path string, data []byte,
  beforeRename func(keep func())) error {`.
- **Work.**
  - Compose each capture in memory: the same JSON lines, in the same order, encoded the same way
    (`SetEscapeHTML(false)`).
  - After the call returns, write the capture through `atomicfile.Write`, then the answer.
  - The callback appends to a slice and touches no file.
  - Every return path that writes a capture today writes one still.
  - Capture names, answer-cache keys and error texts do not change.
- **Test.** While the double's call is running, neither the call's capture nor its answer exists.
  After the call returns, both do.
- **Done-line.** `just test-pkg internal/asks/call`.

### P2.2 — claim-graph stages write their records and `kb-root/` once, after their asks

- **Rating.** Expert. **Seat.** go-coder. **Wave.** W3, after P2.1.
- **Scope.**
  - in `internal/asks/letters.go`, `AskGroup`, `writeGroupRecord`, `GroupRecordPath`, and one new
    function that writes a stage's group records as one file;
  - `internal/asks/letters_test.go`;
  - the six `asks.AskGroup(` calls in `internal/asks/call/call_test.go` (`:66`, `:134`, `:150`,
    `:168`, `:198`, `:219`);
  - in `internal/claimgraph/stages.go`, `Discovered`, `askLeaf` and `ringRow`;
  - in `internal/claimgraph/unmarked.go`, `References` and `askPlanned`;
  - in `internal/claimgraph/classify.go`, `classify`;
  - in `internal/claimgraph/cull.go`, `Culled` and `askCull`;
  - in `internal/claimgraph/elision.go`, `IssueCulls`;
  - in `internal/claimgraph/write.go`, a multi-leaf insert beside `landLeaf`;
  - the tests of `internal/asks` and `internal/claimgraph`.
  - Not `internal/build`. `claimgraph.Options` keeps its field names.
- **Design.**
  - §5, the phase-2 table rows for `letters.go`, `Discovered`, `References`/`askPlanned`,
    `classify`, `IssueCulls`, `WriteCull` and `ringRow`;
  - §5, the two equivalence notes under that table;
  - §3 inv. 5 and 6;
  - AC 5.
- **Facts.**
  - The sites in scope. `grep -rn --include='*.go' -E
    'WriteNodePass\(|WriteUnmarked\(|WriteClassification\(|WriteCull\(|w\.landLeaf\(|writeGroupRecord\(|IssueCulls\(|issue\(w\.raw'
    internal | grep -v _test.go | grep -v 'func '` answers:
    - `internal/claimgraph/unmarked.go:210` and `:288`;
    - `internal/claimgraph/classify.go:79`;
    - `internal/claimgraph/stages.go:183`, `:191`, `:197`, `:350`, `:353`, `:356`, `:359`, `:442`
      and `:571`;
    - `internal/claimgraph/cull.go:337` and `:346`;
    - `internal/asks/letters.go:305`.

    `stages.go:350-359` (`Declared`) and `:442` (`Equations`) ask nothing and stay.
  - Nothing in production reads a group record. `grep -rn 'GroupRecordPath' --include='*.go' .`
    answers:
    - `internal/asks/letters.go:313` (the definition) and `:330` (its use);
    - `internal/asks/letters_test.go:97` and `:149`.
  - The single insert write batch matches the per-leaf sequence. Design §5 settles it from
    `internal/write/inserts.go:328`, `:371` and `:447`, and `internal/claimgraph/stages.go:153`.
  - The three cull write batches refuse a subset of what the sequence refuses, with the one case
    design §5 names. That rests on `internal/write/marks.go:114` and `:125`, and
    `internal/write/edges.go:135` and `:156`.
  - `ringRow` computes every cut first: `stages.go:567` `_, cuts, items := index.BreakCycles(g,
    hosted)` precedes the issue loop, `:570` `for i, d := range cuts {`.
  - The record encoders sort, so one write per stage gives today's bytes. In
    `internal/claimgraph/buildrecords.go`:
    - the classification and unmarked records go through `sortedPairs`;
    - the cull record's entries, culls and links go through `slices.SortFunc`.
- **Work.**
  - The rows of the table, as the design states them.
  - `Discovered`'s one `WriteNodePass` writes the bytes today's per-leaf sequence leaves last: every
    leaf `landed`, with its outcome, verdicts and claims.
  - A stage writes its group records once, after its last ask batch, into the `AskRecords`
    directory it is given.
  - Progress calls stay where they are. `internal/build` holds their events (P2.3).
- **Tests.**
  - **AC 5, for the build stages.** At every ask, a letter-reader double finds each build record and
    both `kb-root/.index/` record files byte-identical to their bytes before the stage.
  - **Equivalence.** Over two leaves where the second's claim takes the first's title, the stage's
    `kb-root/` and node-pass record match the per-leaf sequence's, modulo ids.
- **Done-line.** `just test-pkg internal/claimgraph`, `just test-pkg internal/asks/...`.

### P2.3 — build progress held per ask batch

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W2.
- **Scope.**
  - in `internal/build/rows.go`, `unitProgress` and `claimGraph`;
  - in `internal/build/store.go`, an append of several events beside `emit`;
  - the tests of `internal/build`.
- **Design.** §0 item 8 ("Build"). §5, the `unitProgress` row.
- **Facts.**
  - `internal/build/rows.go:301` `func (w *walk) unitProgress() claimgraph.Progress {`. Its `Unit`
    closure counts at `:309` `done++` and emits at once.
  - `internal/build/store.go:199` `func (p *progress) emit(e buildstore.Event) error {` stamps the
    event (`:200` `e.Time = stamp(p.now())`), writes one line and `Sync`s it.
  - Out of scope: `internal/build/build.go:529` `if err := w.progress.emit(buildstore.Event{Event:
    buildstore.EventUnit, Stage: s.ID, Unit: r.ID, …` emits a row's event after the row returns,
    when no ask is in flight.
- **Work.**
  - `Units` flushes any held events, then writes its own.
  - `Unit` increments `done` exactly as today and holds its event, carrying that `done`. The numbering
    is therefore unchanged.
  - `Fallback` holds its event.
  - The held events are written in one append and one `Sync` in two cases:
    - when the stage function returns, on every outcome;
    - before the next `Units`.
  - Each event is stamped when it is written, by the same rule `emit` follows.
  - **Errors.** Where the stage returned an error and the flush also fails, the stage's error is
    returned and the flush's is logged at error level through the walk's logger. Where only the flush
    fails, its error is returned.
- **Test.** While a stage's asks run, `progress.jsonl` holds the stage's `units` event and none of
  its `unit` or `fallback` events. Afterwards it holds all of them, in order, with `done` running 1…n.
- **Done-line.** `just test-pkg internal/build`.

### P2.4 — score: answers into result cells, result files, one landing, one flush, the fold

- **Rating.** Expert. **Seat.** go-coder. **Wave.** W2.
- **Scope.**
  - in `internal/score/run.go`, `Runner.Run`, `attempt` and the `Write` seam;
  - in `internal/score/invoke.go`, `Invoke`, and `landValue`, which becomes the landing over write
    batches;
  - in `internal/score/ledger.go`:
    - the multi-line append (many JSON lines, one write and one `Sync`);
    - the result file's write, read and fold;
    - `Resume`;
    - `reprice`, which goes through the landing;
  - the tests of `internal/score`;
  - any `cmd/*_test.go` that pins the per-item truth this moves.
  - Not `replay.go`, which is P2.5. Not `status.go` or `ReadRun`, which are P2.6.
- **Design.** §0 item 7. §3 inv. 5, 6, 7 ("Score") and 9. §5, the `Runner.Run`/`landValue` and
  `Resume`/`reprice` rows. AC 5 and AC 8.
- **Facts.**
  - Today, per item:
    - `internal/score/run.go:177` `e, c, interrupted := r.attempt(ctx, it, facts, g)`;
    - then `:183` `if err := r.Ledger.capture(it, e, *c); err != nil {`;
    - then `:188` `if err := r.Ledger.Append(it, e); err != nil {`.

    Skipped lines are appended at `:170`.
  - `landValue` (`internal/score/invoke.go:248`) takes the KB write lock per item: `:264` `release,
    err := write.LockKB(root)`. It is wired at `:149` `land := func(it query.Item, value
    kb.Bearing) error { return landValue(`.
  - `reprice` lands each moved value through the seam: `internal/score/ledger.go:992` `if err :=
    w(a.asItem(), value); err != nil {`.
  - A refusal names its entry. SPEC §7: "| `entry` | 1-based index into the values' `entry` list |".
  - Other packages' tests stage runs through `score.Create` and `Ledger.Append`:
    `cmd/score_status_test.go:80` and `:92`, `internal/mcp/resources_test.go:77`, and
    `internal/score/status_test.go:38`. Keep both exported, with their current signatures.
  - The result directory is fixed by design §0 item 7: `score/<run>/results/`.
  - Design §3 inv. 7 settles the flush against `kbase cancel`'s wait: no correctness rests on the
    flush's duration.
- **Work.**
  - The design's rows, in the order §5 gives.
  - The flush runs on every orderly exit (done, bounded, cancelled and failed), on a context the
    cancel did not end.
  - The skipped lines join the flush, in walk order.
- **Report.** Name the symbols later packages build on:
  - the multi-line append method;
  - the result-file reader;
  - the landing function D2 names in ARCHITECTURE §6 "Provenance".
- **Tests.** Each pins an AC or an invariant:
  - **AC 5.** Inside the `Ask` double, `ledger.jsonl`, `captures.jsonl` and both KB record files
    keep their bytes from before the walk. After the walk, the lines stand in walk order.
  - **Inv. 9.** A write batch with one refused entry fails that item and lands the rest.
  - **AC 8, second clause.** On `--resume`, the result files a stopped run left fold: they land, are
    recorded and are removed, with no ask for them.
    - A result whose attempt the ledger already records is removed alone.
    - A result `Skip` names lands nothing, records no `done` or `failed` line, and appends its
      capture. The walk records it `skipped`.
  - **AC 8, third clause.** A cancel mid-walk records a line for every answered item, and none for
    the call in flight.
- **Done-line.** `just test-pkg internal/score`, plus `just test-pkg cmd` where it edited a `cmd`
  test.

### P2.5 — the replay's three files, appended once

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W3, after P2.4.
- **Scope.**
  - in `internal/score/replay.go`, `Replay.Walk`;
  - `internal/score/replay_test.go`;
  - `cmd/score_replay_test.go`, only where it pins a per-item append.
- **Inputs from P2.4.** The coordinator puts the multi-line append method's name, from P2.4's
  report, into the dispatch.
- **Design.** §5, the `Replay.Walk` row. AC 5.
- **Facts.**
  - `internal/score/replay.go:317` `if err := l.capture(it, e, *c); err != nil {`, `:320` `if err :=
    l.Append(it, e); err != nil {` and `:327` `if err := l.appendLine(&comparisons, comparisonFile,
    cmp); err != nil {` append a capture, a ledger line and a comparison per item.
  - The file carries `//go:build !kbasedist`. The wave-end `just edit-gate` vets it in its first
    step, `go vet ./...`, which builds without the tag. Its second step, `go vet -tags kbasedist
    ./...`, checks the build the file is left out of (`justfile:141-143`).
- **Work.**
  - Hold the three files' records in memory.
  - On every return after the walk asked anything (done, cancelled or failed), append each file once,
    through P2.4's multi-line append, in the replay's item order.
  - An append that fails makes the outcome `failed`, as a store write does today.
  - Write no result file.
- **Tests.**
  - **AC 5.** Inside the `Ask` double, the replay directory's three files keep their bytes from
    before the walk. After the walk, each holds one line per asked item, in order.
  - **Cancel.** A cancel mid-walk leaves one line in each file per item asked before the stop.
- **Done-line.** `just test-pkg internal/score`, plus `just test-pkg cmd` where it edited
  `cmd/score_replay_test.go`.

### P2.6 — a score run's status overlays its result files

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W3, after P2.4.
- **Scope.**
  - in `internal/score/ledger.go`, `ReadRun` and `RunStatus` only;
  - `internal/score/status.go`;
  - `internal/score/status_test.go`.
- **Inputs from P2.4.** The coordinator puts the result-file reader's name, from P2.4's report, into
  the dispatch.
- **Design.** §0 items 7 and 8 ("Score").
- **Facts.**
  - `internal/score/status.go:77` `func runDocument(stateDir string, h buildstore.Holder, live bool)
    (string, []result.Field) {` and `interruptedDocument` (`:183`) count through `ReadRun`.
  - `ReadRun` is `internal/score/ledger.go:619` `func ReadRun(dir, run string) (RunStatus, error) {`.
    It reads `items.jsonl` and `ledger.jsonl` alone.
- **Work.**
  - `ReadRun` reads the run's result files, under `score/<run>/results/`, through P2.4's reader.
  - Each result file counts under the status its entry records, overriding the ledger's last status
    for that item.
- **Test.** A run with result files and no flush reports them in `counts`, `scored` and `failures`.
- **Done-line.** `just test-pkg internal/score`.

### P2.7 — the per-trail directory: answers and group records, pruned by count

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W4, after P2.1, P2.2 and P2.3. Parallel with
  P2.9.
- **Scope.**
  - in `internal/asks/call/call.go`, `New` and `Caller`: the answers directory becomes a function;
  - in `internal/asks/call/call_test.go`, the `New(` calls and the answer `Glob`s;
  - in `internal/build/build.go`, a `walk` field holding the current trail's directory, set where
    the trail is read and where `start` records; `launch` (passing the function); and `run` (calling
    the prune);
  - in `internal/build/rows.go`, the `AskRecords:` path `claimGraph` passes;
  - in `internal/build/store.go`, a prune of `scratch/trails/` beside `pruneCaptures`;
  - in `internal/buildstore/store.go`, `answerCache`;
  - `internal/buildstore/store_test.go`, and a new test in `internal/buildstore` for stage order;
  - the tests of `internal/build`.
- **Design.** §0 item 10. §5, the per-trail row and "The per-trail directory". AC 14.
- **Facts.**
  - The cache directory is fixed at construction today: `internal/asks/call/call.go:51` `func
    New(client model.Client, scratch string, lg log.Logger) *Caller {` sets `:55` `answers:
    filepath.Join(scratch, asks.AnswersDir),`.
  - `internal/asks/call/call_test.go` is `package call`. Its `New(` calls are at `:65`, `:133`,
    `:149`, `:167`, `:197` and `:218`. Its answer `Glob`s read the flat directory at `:108`, `:182`
    and `:224`: `filepath.Glob(filepath.Join(scratch, asks.AnswersDir, …))`.
  - The build constructs its two callers in `walk.launch`, before a fresh build records `start`:
    `internal/build/build.go:379` `letters := call.New(p.LettersClient, filepath.Join(w.stateDir,
    buildstore.ScratchDir), w.opts.Logger)` and `:380` `overview := call.New(p.OverviewClient, …`.
  - The walk's trail is set at `build.go:293` `w.trail = trail` and extended at `:499` `w.trail =
    append([]ledger.Entry{{Stage: s.ID, Commit: commit, Body: body}}, w.trail...)`. A
    `ledger.Entry` carries `Stage, Commit, Subject, Body string` (`internal/ledger/ledger.go:62-63`).
  - The group records' directory is passed at `internal/build/rows.go:286` `AskRecords:
    filepath.Join(w.stateDir, buildstore.ScratchDir, asks.AskRecordsDir)`, with `asks.AskRecordsDir`
    = `"asks"` (`internal/asks/letters.go:42`).
  - `start` is the first stage: `internal/buildstore/stages.go:29` `{"start", "build started"},`.
  - The launch checks end at `build.go:314` `if err := w.launch(remaining); err != nil {`.
  - `internal/build/store.go:233` `const keptBuilds = 5`, and `:240` `func pruneCaptures(dir string)
    error {`.
  - `internal/buildstore/store.go:227` `func answerCache(dir string) (entries, size int, err error) {`
    counts the regular files of the flat `scratch/answers/`. `TestAnswerCacheCountsTheCache`
    (`internal/buildstore/store_test.go:17-26`) pins that: flat files count, directories do not.
- **Work.** Design §5 "The per-trail directory", as stated. Its rules close this package's choices:
  - The `Caller` calls the directory function at every cache read and every cache write, and keeps no
    copy.
  - The function returns the `walk` field joined with `answers`. The field is set on the stage
    goroutine before any later stage launches an item.
  - No branch handles a trail with no `start` entry. The new `internal/buildstore` test pins that
    every `Step` in `buildstore.Steps` (`internal/buildstore/stages.go:84`) whose `CallsAModel()`
    (`:77`) is true belongs to a stage after `start` in `buildstore.Stages`. So no ask reads the
    field unset.
  - `answerCache` counts the regular files of every `scratch/trails/*/answers/`, and no file of a
    flat `scratch/answers/`. `TestAnswerCacheCountsTheCache` moves to that truth.
  - The prune does three things:
    - where the current trail's directory does not stand, it creates it;
    - it then sets that directory's modification time with `os.Chtimes`;
    - it removes `scratch/answers/` and `scratch/asks/` whole, where they stand.
  - Expected cost: a trail resumed for the first time after the upgrade asks its calls again, because
    its flat answers are removed. `test-integration-build-live-fixture-resume`'s first run after the
    upgrade is such a resume.
- **Tests (AC 14).**
  - **Retention.** A store holds eight trail directories, each with `answers/` and `asks/` entries
    and staggered modification times, the current trail's the oldest, plus a flat `scratch/answers/`
    and a flat `scratch/asks/`. After the prune, the current trail's directory and the five newest
    others stand whole, and both flat directories are gone.
  - **A missing current directory.** The prune over a store with no directory for the current trail
    creates it, and its modification time is the prune's.
  - **Survival.** `TestInterruptedAskingStageResumesFromTheAnswerCache`
    (`internal/build/inference_test.go:532`) still passes. It is extended to assert three things:
    - the interrupted run's answers stand under `scratch/trails/<start-commit>/answers/`, with the
      commit read from the repository's trail;
    - its group records stand under `scratch/trails/<start-commit>/asks/`;
    - the resume asks none of the answered calls again.
  - **Status.** `answerCache` counts entries across two trail directories.
  - **Stage order.** The `internal/buildstore` test above.
- **Done-line.** `just test-pkg internal/build`, `just test-pkg internal/buildstore`, `just test-pkg
  internal/asks/call`.

### P2.8 — `progress.jsonl` rotation

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W5, after P2.7. Parallel with D2.
- **Scope.**
  - in `internal/build/build.go`, `run`, at the point P2.7's prune runs;
  - in `internal/build/store.go`, the rotation, its two constants, and the prune of rotated files,
    beside `openProgress`;
  - the tests of `internal/build`.
  - `Run` is unchanged. The trail is still read in `run`, after the run event.
- **Design.** §0 item 11. §5, the progress-rotation row and "Progress rotation". AC 15.
- **Facts.**
  - The rotation runs directly after the per-trail prune, at the same point: after
    `internal/build/build.go:314` `if err :=
    w.launch(remaining); err != nil {`, where stages remain, before `w.prepare`. A fresh build is one
    whose `:312` `resumed := recorded[buildstore.StageStart] != ""` is false.
  - By then the live file holds exactly one line of this invocation, its run event.
    - `:199` `if err := w.progress.emit(buildstore.Event{Event: buildstore.EventRun, …` writes it.
    - `grep -n 'progress.emit(' internal/build/*.go | grep -v _test.go` answers eight lines:
      `build.go:199`, `:211`, `:500`, `:511` and `:529`, and `rows.go:306`, `:310` and `:313`.
      Every one but `:199` runs after `:314` or at the run's end.
  - `openProgress` is `internal/build/store.go:191` `func openProgress(dir string, now func()
    time.Time) (*progress, error) {`, and `:211` `func (p *progress) close() error { return
    p.f.Close() }`.
  - `atomicfile`'s temp files are named `.<target>.…` (`internal/atomicfile/atomicfile.go:27`), so a
    prune listing `progress-*.jsonl` never sees one.
  - The sizing is in design §5 "Progress rotation": `wc -l
    test-data/transient/test-integration-build-live-arxiv/2609.10318v1/state/progress.jsonl` answers
    `85`, so `progressRotateLines` is 500. `keptProgressFiles` is 15.
  - The one reader of the record is `internal/buildstore/store.go:141` `f, err :=
    os.Open(filepath.Join(dir, ProgressFile))`. Design §5 states why every reader stays safe.
- **Work.** Design §5 "Progress rotation", steps 1 to 6, as stated. Its rules close this package's
  choices:
  - **The line test.** The live file holds more than `progressRotateLines` lines, its run event
    included.
  - **The split.** The live file's last line is this invocation's run event. Everything before it
    goes to the rotated file, and that line alone stays as `progress.jsonl`. Both are written through
    `atomicfile.Write`.
  - **The stamp.** `opts.now()` in UTC, in Go layout `20060102T150405.000000000Z`. Fixed-width
    nanoseconds make the names sort by time and never collide.
  - **The prune.** By name, keeping the newest `keptProgressFiles`.
  - **The reopen.** Step 5 reopens into the same `*progress`, replacing its `f`
    (`internal/build/store.go:186-189`). It never assigns a new `w.progress`, because `Run`'s
    `defer w.progress.close()` (`build.go:198`) bound its receiver when the defer ran.
  - **Failures.** A failure of any step returns from `run` as its error, and the build ends
    `failed`.
    - A failure in steps 2–4 still runs step 5 before returning, so the end event (`build.go:211`)
      lands.
    - Where step 5 itself fails, the end event is lost and logged (`:212`).
  - **Order.** The rotation runs directly after P2.7's per-trail prune, at the same point.
- **New behaviour.** Rotation happens only once a fresh build has passed its launch checks and has
  stages to walk. A build that ends refused, `retry`, `unchanged` or `bounded` before that point
  leaves the record as it was.
- **Tests (AC 15).**
  - A fresh build over a store whose `progress.jsonl` exceeds `progressRotateLines`:
    - leaves a live file whose first line is its own run event;
    - leaves a `progress-<stamp>.jsonl` holding every earlier line, and no line of its own.
  - A resume over the same store rotates nothing.
  - A fresh build refused at its launch checks (a volume root that is not a file) rotates nothing.
  - A fresh build under the threshold rotates nothing.
  - With 16 rotated files standing, a rotation leaves the newest 15.
- **Done-line.** `just test-pkg internal/build`.

### P2.9 — the live recipes count the per-trail directories

- **Rating.** Mechanical. **Seat.** shell-dsl-coder. **Wave.** W4, parallel with P2.7.
- **Scope.**
  - `tools/scripts/test-integration-build-live-arxiv.sh`, lines 112, 113 and 115;
  - `tools/scripts/test-integration-build-live-fixture.sh`, lines 175, 176 and 186;
  - `tools/scripts/write-evidence.sh`, line 45's text.
- **Design.** §0 item 10.
- **Facts.**
  - Both scripts count with a recursive `find`:
    - `test-integration-build-live-arxiv.sh:53` `count() { find "$1" -type f -name "$2" 2>/dev/null |
      wc -l | tr -d ' '; }`;
    - `test-integration-build-live-fixture.sh:132`, likewise.
  - They root the counts at the flat directories: `answers="$(count "${state}/scratch/answers"
    '*.txt')"` and `ask_records="$(count "${state}/scratch/asks" '*.yaml')"`.
  - Under `scratch/trails/` those roots stand no more, so both counts would read 0. The two patterns
    are distinct (`*.txt` answers, `*.yaml` group records), so both can root at `scratch/trails`.
  - `write-evidence.sh:45` names `scratch/answers/, scratch/asks/` in its evidence listing.
- **Work.**
  - Root both counts in each script at `"${state}/scratch/trails"`, keeping each pattern.
  - Group records are one file per stage after P2.2, `<kind>.yaml`, holding one YAML document per
    group. So `ask_records` now counts stage files, and its `-gt 0` check still holds.
    - In each script's check message, change "group records" to "group-record files"
      (`test-integration-build-live-arxiv.sh:115`, `test-integration-build-live-fixture.sh:186`).
    - The `ask-records:` key the scripts write keeps its name.
  - In `write-evidence.sh:45`, name `scratch/trails/` in place of `scratch/answers/, scratch/asks/`.
- **Unchanged.** The answer count matches `*.txt` alone, so it misses the decision answers, which are
  `.json`. That is today's behaviour, and this package leaves it.
- **Done-line.** These recipes are LIVE, so CONVENTIONS "Testing" sets the check. `config` and `dir`
  are positional parameters (`justfile:175` `test-integration-build-live-arxiv
  config=LIVE_CONFIG_DIR:` and `:185` `test-integration-build-live-fixture dir config=LIVE_CONFIG_DIR
  *roots:`), so each value is passed bare.
  - `bash -n` on each of the three scripts;
  - `just --dry-run test-integration-build-live-arxiv test-data/fixtures/config`, whose rendered line
    sets every variable the script requires;
  - `just --dry-run test-integration-build-live-fixture .claude-temp/live-fixture-dry
    test-data/fixtures/config`, likewise;
  - `grep -n 'scratch/answers\|scratch/asks' tools/scripts/*.sh` prints nothing.

### D2 — phase 2's contract and user docs

- **Rating.** Expert. **Seat.** tech-writer. **Wave.** W5, parallel with P2.8.
- **Scope.** `SPEC.md`, `ARCHITECTURE.md`, `README.md` and `USER_CONVENTIONS.md`.
- **Inputs from P2.4.** The coordinator puts the landing function's name, from P2.4's report, into
  the dispatch.
- **Design.** In §7:
  - every entry labelled "phase 2" under SPEC.md and ARCHITECTURE.md;
  - the phase-2 half of each entry that has two;
  - the whole "README.md and USER_CONVENTIONS.md" section.
- **Facts.**
  - Many quoted phrases wrap across lines in the target documents. Find each one by grepping a short
    fragment from the phrase's first line. For example:
    - `grep -n 'The in-flight' SPEC.md`, which answers line 391, where "The in-flight call is
      abandoned" begins;
    - `grep -n 'appends each attempt' ARCHITECTURE.md`, which answers line 672;
    - `grep -n 'are all-or-nothing' SPEC.md`, which answers line 571.
  - Three entries give a direction instead of text, and the writer writes their prose:
    - ARCHITECTURE §6 "The one-op landing";
    - ARCHITECTURE §6 "Provenance";
    - the `results/` sub-bullet of ARCHITECTURE §8 layout.
- **Work.** Land each entry's given text, and write the three directed entries' prose.
- **Checks.**
  - `grep -n 'in-flight unit\|each landing\|unit in flight' SPEC.md ARCHITECTURE.md README.md
    USER_CONVENTIONS.md` prints nothing. At `aee74c1` it answered six lines: `ARCHITECTURE.md:396`,
    `SPEC.md:393`, `SPEC.md:560`, `USER_CONVENTIONS.md:101`, `README.md:217` and `README.md:336`.
  - `grep -n 'I11' ARCHITECTURE.md` answers the new invariant's row.
  - `grep -n 'never pruned' ARCHITECTURE.md` answers only the stage reports' sentence.
  - `grep -n 'The answer cache and the stage\|composed prompts' SPEC.md ARCHITECTURE.md` prints
    nothing.
  - `grep -n 'scratch/answers/' SPEC.md ARCHITECTURE.md` prints only the §8 Retention sentence that
    names the undivided directories an earlier kbase wrote.
- **Done-line.** The checks hold. Committing C2 is the coordinator's job.

---

## Phase 3, step A — consumers move onto the queue

### A1 — the `[asks] concurrency` key

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W6.
- **Scope.**
  - in `internal/config/config.go`, `AsksConfig` and the new key's constant and under-1 check;
  - in `internal/config/layer.go`, `takeConfig`;
  - in `internal/config/template.go`, two lines inserted after `# [asks]`;
  - the config tests these touch.
  - `readerConcurrency`, its constant `keyReaderConcurrency`, its under-1 check and its template lines
    all stay. A2 supersedes the key, because A2 removes `cmd/build.go:144`'s read of it.
- **Design.** §2 "Key", with its two template lines word for word. AC 3, its first clause.
- **Facts.**
  - The old key is the precedent to follow:
    - its field, `internal/config/config.go:57` ``ReaderConcurrency *int `toml:"readerConcurrency"` ``;
    - its key constant, `:62` `const keyReaderConcurrency = "asks.readerConcurrency"`;
    - its under-1 refusal, `:142` `if n := cfg.Asks.ReaderConcurrency; n != nil && *n < 1 {`;
    - its merge by presence, `internal/config/layer.go:174` `if cfg.Asks.ReaderConcurrency == nil {`.
  - The template's block opens at `internal/config/template.go:26` `# [asks]`.
- **Work.**
  - Add `concurrency` beside the old key, with the same strict load and the same merge by presence.
  - A value under 1 fails the load, naming `asks.concurrency`.
  - Insert the design's two template lines directly after `# [asks]`.
- **Test.** A `concurrency` value under 1 is refused, naming `asks.concurrency`. An absent key leaves
  the field unset, and a set one merges by presence.
- **Done-line.** `just test-pkg internal/config`.

### A3 — `asks.Launch`, and claim-graph stages launching groups in order

- **Rating.** Expert. **Seat.** go-coder. **Wave.** W6.
- **Scope.**
  - in `internal/asks/letters.go`:
    - the `Launch` seam type;
    - its step-A adapter over a plain `LetterReader`, bounded by a concurrency as `AskGroup`'s
      semaphore bounds today;
    - `AskGroup` over a `Launch`: the first ask launched alone and waited for, then the rest launched,
      the record collated after the consumer waits, and no `concurrency` parameter left;
    - the first error by arrival;
  - the six `asks.AskGroup(` calls in `internal/asks/call/call_test.go`;
  - in `internal/claimgraph/stages.go`, `Depends` (its `opts.Reader != nil` at `:538`), `Options`
    (a `Launch` field beside `Reader`), `Discovered`
    and `askLeaf` (`:242`, which holds the `AskGroup` call at `:247`);
  - in `internal/claimgraph/unmarked.go`, `References` and `askPlanned`;
  - in `internal/claimgraph/classify.go`, `classify`;
  - in `internal/claimgraph/cull.go`, `askCull`;
  - the tests of `internal/asks` and `internal/claimgraph`.
  - Not `internal/build`. Until A2 sets `Options.Launch`, a stage builds the adapter from
    `Options.Reader` and `Options.ReaderConcurrency`.
- **Design.** §0 item 9. §1 "Consumer seams". §3 inv. 5, 8, 10 and 11. §5 "Step A": "Stages" and
  "`AskGroup`". AC 12.
- **Facts.**
  - The launch loops: `grep -rn --include='*.go' -E 'asks\.AskGroup\(|^func AskGroup' internal |
    grep -v _test.go` answers:
    - `internal/claimgraph/unmarked.go:268`;
    - `internal/claimgraph/classify.go:109`;
    - `internal/claimgraph/stages.go:247`;
    - `internal/claimgraph/cull.go:399`;
    - `internal/asks/letters.go:235`.

    P2.2 moved these lines. The `asks.AskGroup(` text is the anchor.
  - Today's group semaphore, which the adapter keeps until B2: `internal/asks/letters.go:251` `sem :=
    make(chan struct{}, concurrency)`.
  - `AskGroup` reports the first error by index. Its anchor is `for _, err := range errs {`.
  - `References` fills a map lazily: `bodies := map[string]string{}`. Each group's items are
    therefore composed in the launch loop (inv. 10).
  - The progress hooks, per §0 item 8:
    - `units(` is called before the launch loop: `opts.Progress.units(`, `progress.units(` in
      `classify.go`;
    - `unit(` and `fallback(` are called after the wait, on the goroutine that runs the stage,
      reading the result cells: `opts.Progress.unit(`, `opts.Progress.fallback(`, and `progress.` in
      `classify.go`.
  - `internal/asks` may not import `model`: ARCHITECTURE §11, "Only `asks/call`, `build`,
    `modelclient` and three `cmd` files import `model`".
  - Two places decide "inference or not" by testing `Reader` against nil:
    - `internal/claimgraph/stages.go:538`, in `Depends`: `findings = append(findings,
      classificationFindings(c, opts.Reader != nil)...)`;
    - `internal/claimgraph/classify.go:55` `reader, progress := opts.Reader, opts.Progress`, whose
      `reader` is tested at `:84` `!held(e, ok, reader != nil)`, `:89` `if reader != nil {` and `:99`
      `if reader == nil {`.
- **Work.**
  - Each place that tests `Reader` against nil tests `opts.Launch != nil || opts.Reader != nil`
    instead. So the tree A2 leaves, where the build passes `Launch` and no `Reader`, still classifies
    with inference. B2 drops the `Reader` half.
  - Each stage runs one launch loop, on the goroutine that runs the stage, and launches its groups in
    order: `Discovered` through `askLeaf` by leaf, `askPlanned` and `classify` by source, and
    `askCull` by group. Cull pass 2 starts after pass 1 drains.
  - Each item's re-ask runs inside the same item.
  - Each result goes to its result cell, never in arrival order.
  - `claimgraph` and `asks` learn nothing of N or of `model`.
- **Report.** Name the `Launch` type, the adapter, and `AskGroup`'s new signature. A2 fills the seam,
  and B2 deletes the adapter.
- **Tests.**
  - **AC 12, unit level.** With a recording `Launch` fake, a stage's items are launched in submission
    order, and each group's first item completes before the group's second is launched.
  - **Inv. 5.** With a fake that completes items out of order, a stage's build records and `kb-root/`
    equal its sequential run's, modulo ids.
  - **Inv. 11.** A failing item's error is reported, not a cancelled sibling's.
- **Done-line.** `just test-pkg internal/claimgraph`, `just test-pkg internal/asks/...`.

### A4 — `score.Launch`, and the score walk and replay launching in set order

- **Rating.** Expert. **Seat.** go-coder. **Wave.** W6.
- **Scope.**
  - in `internal/score/run.go`:
    - the `Launch` seam type, and its step-A adapter over a plain `Ask`, one item at a time;
    - the `Runner` field that carries the launch;
    - `Runner.Run`, and `attempt` (`:219` `func (r *Runner) attempt(ctx context.Context, it
      query.Item, facts passFacts, g graph) (e Entry, c *call, interrupted bool) {`);
  - in `internal/score/invoke.go`, an `Invocation` field carrying the launch, beside `NewAsk`, and
    `Invoke`, which builds the adapter from `NewAsk` where that field is unset;
  - in `internal/score/replay.go`, `Replay.Walk`;
  - in `cmd/score_replay.go`, the one `rp.Walk(` call, which passes the adapter;
  - the tests of `internal/score`.
- **Design.** §0 items 7 and 9. §1 "Consumer seams". §3 inv. 5, 7 ("Score") and 10. §5 "Step A":
  "`Runner.Run` and `Replay.Walk`". AC 12.
- **Facts.**
  - The drivers are `func (r *Runner) Run(ctx context.Context) (Result, error) {` and `func (rp
    *Replay) Walk(ctx context.Context, ask Ask, h ReplayHeader) (ReplayResult, error) {`.
  - The one call site in `cmd`: `cmd/score_replay.go:158` `res, err := rp.Walk(ctx, ask,
    score.ReplayHeader{`.
  - The run opens its `Ask` through `internal/score/invoke.go:67` `NewAsk func() (Ask, error)`, which
    `cmd/score.go:227` fills. A2 sets the launch field there instead, and B2 deletes `NewAsk`.
  - `Limit` cuts the set before the loop: anchor `bounded := r.Limit > 0 && r.Limit < len(items)`.
  - `Status` and `Skip` are decided per item inside the loop: anchors `status :=
    r.Ledger.Status(it)` and `if r.Skip != nil && r.Skip(it) {`. P2.4 reshaped that loop. Read it as
    landed.
  - `internal/score` imports no `model` (ARCHITECTURE §11).
- **Work.**
  - In the walk's launch loop, decide `Status`, `Skip` and `Limit`, and compose every prompt.
  - Launch the asks in set order. Each item stamps its entry's bounds around its own `Ask` call,
    writes its result cell, and writes its result file in `after`.
  - The flush stays P2.4's, in walk order.
- **Report.** Name the `Launch` type, the adapter, the `Runner` field and the `Invocation` field. A2
  fills them, and B2 deletes the adapter.
- **Tests.**
  - **Inv. 5.** Against a fake `Launch` that completes items out of order, the ledger and the captures
    equal the sequential run's, modulo stamps and durations.
  - **AC 12, unit level.** A recording fake sees items launched in set order.
- **Done-line.** `just test-pkg internal/score`, `just test-pkg cmd`.

### A2 — every consumer on the queue, and `readerConcurrency` superseded

- **Rating.** Expert. **Seat.** go-coder. **Wave.** W7.
- **Scope.**
  - in `cmd/build.go`, `cmd/score.go`, `cmd/probe.go` and `cmd/score_replay.go`:
    - the flag on `build` and `score`;
    - resolving N;
    - the queue helper;
    - filling `score.Launch`, set on the `Invocation` at `cmd/score.go:227` and passed at the replay's
      `rp.Walk(`;
    - the probe's one item;
    - `replayFlags`;
    - removing the block that reads `p.Config.Asks.ReaderConcurrency`;
  - in `cmd/provider.go`, the superseded-key helper, and its two calls;
  - in `internal/config/config.go`, deleting `readerConcurrency`'s under-1 check and its constant
    `keyReaderConcurrency`;
  - in `internal/config/template.go`, deleting the two `readerConcurrency` lines;
  - `internal/config/config_test.go`'s `TestLoadConfigReaderConcurrency`, moved to the new truth;
  - in `internal/build/build.go`:
    - `Provider` and `Options`, deleting `Provider.ReaderConcurrency`;
    - `walk`'s `letters`, `passage` and `readerConcurrency` fields (declared at `:136-138`, assigned
      at `:381-382`), deleting `readerConcurrency`. With the `cmd/build.go:144-146` block gone,
      nothing sets it or `Provider.ReaderConcurrency` (design §2, "Deleted with the read");
    - the overview's seam type, declared beside `walk.passage`;
    - `launch` and `resumeCommand`;
  - in `internal/build/rows.go`:
    - `claimGraph`: at `:286`, the `Reader:` and `ReaderConcurrency:` fields are removed and
      `Launch:` is set in their place. A3's `opts.Launch != nil || opts.Reader != nil` keeps the
      classification asking.
    - `overview`: one item.
  - in `internal/asks/call/call.go`:
    - filling `asks.Launch` and the overview's seam over the queue;
    - the per-item reader that holds its files until `after`;
    - the capture's wait;
    - `decide`'s error;
  - the tests of those packages;
  - `cmd/mcp_test.go`, for the AC 10 assertion only.
- **Inputs from A3 and A4.** The coordinator puts the seam, adapter, field and signature names from
  A3's and A4's reports into the dispatch.
- **Design.**
  - §0 items 5 and 9;
  - §1 "Shape", "The consumer" and "Consumer seams";
  - §2 "Flag", "Where the queue is built", and "`readerConcurrency` is superseded (step A)";
  - §3 inv. 1, 2, 3 and 11, and "Instrumentation";
  - §5 "Step A": "The seams", "`call.Caller`", "`scoringAsk` and `probe`", "Overview" and "Build's
    resume";
  - AC 3, 6, 7, 9 and 10.
- **Facts.**
  - The model call sites outside `model`: `grep -rn --include='*.go' -E
    'model\.Chat\(|\.Decision\((ctx|callCtx)|\.ConsultStream\(|\.Consult\(|ConsultDrained\(' cmd
    internal | grep -v _test.go | grep -v '^internal/model/'` answers:
    - `cmd/probe.go:114` and `:124`;
    - `cmd/score.go:321`;
    - `internal/asks/call/call.go:131` and `:393`.

    P2.1 moved the `call.go` lines. Its anchors are `got, err := c.client.Decision(ctx,
    model.DecisionRequest{` and `got, err := model.Chat(ctx, c.client`. After this package, each
    call is made through a `Slot`.
  - Two calls are wrapped in `WithTimeout` before they run: `cmd/score.go:319` and `cmd/probe.go:100`,
    each `callCtx, cancel := context.WithTimeout(ctx, timeout)`. Each wrap moves inside the item,
    around the slot's call (inv. 3). Probe's `wall-ms` clock (`cmd/probe.go:104` `started :=
    time.Now()`) moves inside the item with it.
  - The queue helper lives in one of three files: `cmd/importpolicy_test.go:463`
    `"internal/model": {"cmd/build.go", "cmd/score.go", "cmd/probe.go"},`.
  - `cmd/score_replay.go:43` `var replayFlags = []string{"replay", "pass", "failed", "limit",
    "scoring-model", "state-dir", "timeout"}` gains `"ask-concurrency"`.
  - `internal/build/build.go:661` `func (w *walk) resumeCommand() []string {` names each flag
    explicitly. Add `--ask-concurrency N` there, where the invocation named the flag.
  - The `walk` fields A2 replaces:
    - `internal/build/build.go:136` `letters asks.LetterReader`;
    - `:137` `readerConcurrency int`;
    - `:138` `passage func(ctx context.Context, excerpts string, rejected []string, attempt int)
      (string, error)`;
    - assigned at `:381` `w.letters, w.readerConcurrency = letters.Letters(p.Letters),
      p.ReaderConcurrency` and `:382` `w.passage = func(…`.
  - `rows.go:286` reads `letters` and `readerConcurrency` today: `Reader: w.letters,
    ReaderConcurrency: w.readerConcurrency,`.
  - The overview's two attempts loop in `internal/build/rows.go:381` `for attempt := 1; attempt <=
    overviewAsks; attempt++ {`. They become one item, and `overview.md` (`:410`) is written after the
    wait.
  - `decide` wraps a cancelled `ctx` as an incomplete call. Its anchor is `return nil,
    asks.IncompleteError{Subject: fmt.Sprintf("%s-%s-%s", q.Kind, q.Group, q.Item)`. The fix is in
    design §3 inv. 11.
  - The flag's refusal follows `--limit`'s: `cmd/score.go:128` `return refused(nil,
    toolresult.Item{Check: checkUsage, Key: "--limit", Detail: "--limit counts items and cannot be
    negative"})`.
  - The superseded-key line goes beside `cmd/build.go:115` `supersededProvider(stderr, p.Config)`
    and `cmd/score.go:290` `supersededProvider(opts.Stderr, opts.Config)`, and nowhere else.
  - `cmd/build.go:144` `if n := p.Config.Asks.ReaderConcurrency; n != nil {` goes in this package.
    Removing that read and the under-1 check together means no tree feeds an unchecked value to
    `AskGroup`'s semaphore.
  - The flag reaches the `build` and `score` tool schemas with no table edit. They are built from
    `internal/mcp/mcp.go:223` `cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {`, and a
    flag declared on `buildCmd.Flags()` or `scoreCmd.Flags()` is local and non-persistent.
  - The probe's document comes from `cmd/probe.go:273` `func probeAnswerFields(…`, fed by the last
    attempt's `ChatAttempt`. `TestProbeReportsTheCall` (`cmd/probe_test.go:152`) pins its keys.
- **Work.**
  - The design's step-A items in this scope.
  - Every consumer launches through the invocation's one queue. Probe launches one item on a queue of
    one.
  - The flag's precedence is the flag, then the key, then 2.
- **Report.** Name the variable or field that carries N to each `modelclient.New` call in the four
  `cmd` files. A6 wires it.
- **Tests.**
  - **AC 3.**
    - The flag, then the key, then 2; the flag's refusal under 1.
    - A file-mode `score` whose `config.toml` sets `readerConcurrency` writes the design's stderr line
      exactly once, and its document matches the run without the key.
    - `readerConcurrency = 0` loads, with no refusal.
  - **AC 6.** Through `asks/call`'s seam: during an item's calls, none of its captures or answers
    exists; they exist once `after` has run.
  - **AC 7.** At N=1 with items queued behind a slow call, and `--timeout` shorter than the total
    wait but longer than one call, no item fails.
  - **AC 10.** The `build` and `score` tool schemas carry `ask-concurrency`, and the tool set is
    unchanged.
  - **Inv. 11 and Instrumentation.** A capture records the slot's wait, and `decide` returns
    `ctx.Err()` on a cancelled `ctx`.
  - **Build's resume.** The resume command carries `--ask-concurrency` where the launch named it.
- **Done-line.** `just test-pkg cmd`, `just test-pkg internal/build`, `just test-pkg
  internal/asks/call`, `just test-pkg internal/config`.

### A5 — evidence: the bound, submission order, and outputs that do not depend on N

- **Rating.** Expert. **Seat.** go-coder. **Wave.** W8.
- **Scope.**
  - a new test file in `internal/build`;
  - a new test file in `cmd`, which calls no `modelclient.New` (A6 changes its signature in the same
    wave) and reaches the registry only through `scoreIn`;
  - in `internal/claimgraph/modulo_ids.go`, `ComparedKB`, `BuiltBy` and `CompareModuloIDs`, which
    gain the cull record;
  - a new `internal/claimgraph/modulo_ids_test.go`.
- **Design.** §0 item 9. §5 "Step A", "Evidence". AC 2, AC 4 and AC 12.
- **Facts.**
  - `internal/claimgraph/modulo_ids.go:19-21` `type ComparedKB struct { Name, KBRoot, NodePass,
    Classification, Unmarked string }` carries no cull record.
    - The comparer is `:43` `func CompareModuloIDs(a, b ComparedKB) ([]ModuloIDsCheck, error) {`.
    - The cull record's file is `internal/buildrecords/buildrecords.go:20` `CullFile =
      "kb-build-cull.yaml"`.
  - `CompareModuloIDs` is tested today only through `internal/build/build_test.go:219` `func
    sameModuloIDs(t *testing.T, repo, want string) {`. `internal/claimgraph/modulo_ids_test.go` does
    not exist; this package creates it.
  - `sameModuloIDs` has six callers, which must stay green once the cull record is compared:
    `internal/build/build_test.go:307`, `:661`, `:700` and `:717`, and
    `internal/build/inference_test.go:358` and `:569`. Each compares two builds of one input. A red
    one is reported as a finding, not loosened.
  - The build-test precedent is `internal/build/inference_test.go:174` `func (f fixture)
    asking(client model.Client) Options {`, which builds a `Provider` over one double.
  - The score harness lives in `cmd`, because `internal/score` cannot import `model`:
    - `runScore` is `cmd/score.go:99` `func runScore(opts scoreOptions) (int, error) {`;
    - its test harness is `cmd/score_test.go:111` `func scoreIn(t *testing.T, repo, state string,
      opts scoreOptions, client func() model.Client) scoreResult {`, which calls it at `:143`.
- **Work.** Write these tests. Change no behaviour beyond `CompareModuloIDs` gaining the cull record.
  - **Build, AC 4.** At N=1 and N=4, the tracked records and `kb-root/` agree modulo ids, the cull
    record included.
  - **Score, AC 4.** At N=1 and N=4, `ledger.jsonl` and `captures.jsonl` agree modulo stamps and
    durations.
  - **The bound, AC 2.** For a build and for a score run at N=2, each over more than N items, a
    counting double sees at most N calls in flight, and sees N reached.
  - **Submission order, AC 12.** At N=1, a recording double sees a build stage's requests and a score
    walk's requests in submission order. Each group's first request completes before the group's
    second arrives.
  - **The cull-record comparison.** A unit test in the new `modulo_ids_test.go`: two KBs differing
    only in the cull record's ids agree, and two differing in a cull disagree.
- **Report.** Name each evidence test. Step B's precondition cites them.
- **Done-line.** `just test-pkg internal/build`, `just test-pkg cmd`, `just test-pkg
  internal/claimgraph`.

### A6 — the transport keeps N idle connections per host

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W8.
- **Scope.**
  - in `internal/model/client.go`, `Endpoint.IdleConns`;
  - in `internal/model/transport.go`, `newHTTPTransport`, which takes the count;
  - in `internal/model/openai_client.go`, `NewOpenAIClient`, and in
    `internal/model/anthropic_client.go`, `NewAnthropicClient`, each passing it;
  - in `internal/modelclient/modelclient.go`, `New` gaining the count, and `Registry.Get` setting it
    on the endpoint;
  - every `modelclient.New(` call: the seven in `cmd` and the two in `cmd` tests;
  - the tests of `internal/model` and `internal/modelclient`;
  - one new test in `cmd`.
- **Inputs from A2.** The coordinator puts the name of the value carrying N, from A2's report, into
  the dispatch.
- **Design.** §1 "Idle connections". AC 13.
- **Facts.**
  - `internal/model/transport.go:47` `tr := http.DefaultTransport.(*http.Transport).Clone()` leaves
    `MaxIdleConnsPerHost` unset. `grep -n 'DefaultMaxIdleConnsPerHost' "$(go env
    GOROOT)/src/net/http/transport.go"` answers `62:const DefaultMaxIdleConnsPerHost = 2`. The
    cloned `MaxIdleConns` is that file's `:54` `MaxIdleConns: 100,`.
  - Both clients build their transport with `newHTTPTransport()`: `openai_client.go:45` `http:
    newHTTPTransport(),` and `anthropic_client.go:63` `return &AnthropicClient{endpoint: e, http:
    newHTTPTransport()}`.
  - `internal/modelclient/modelclient.go:44` `func New(seam NewClientFunc, lg log.Logger,
    promptCacheKey string) *Registry {`. `Registry.Get` builds the endpoint at `:69` `client :=
    r.newClient(model.Endpoint{Name: ep.Provider, …`.
  - `grep -rn 'modelclient.New(' --include='*.go' .` answers:
    - `cmd/models.go:51`, `cmd/configure.go:84` and `cmd/show-config.go:68`, which pass 0;
    - `cmd/build.go:50`, `cmd/probe.go:89`, `cmd/score.go:224` and `cmd/score_replay.go:151`, which
      pass N (1 for probe);
    - `cmd/build_env_test.go:43` and `cmd/build_test.go:153`, which pass 0.
- **Work.**
  - `MaxIdleConnsPerHost` is the larger of `IdleConns` and 2.
  - `MaxIdleConns` is the larger of `IdleConns` and 100.
- **Tests (AC 13).**
  - A client built from an endpoint with `IdleConns` 4 has a transport whose `MaxIdleConnsPerHost`
    is 4. With 0, it is 2.
  - The registry puts its count on every endpoint it hands its seam.
  - A `score` run at `--ask-concurrency 4` builds its registry with 4, observed through the
    `NewClient` seam.
- **Done-line.** `just test-pkg internal/model`, `just test-pkg internal/modelclient`,
  `just test-pkg cmd`.

### DA — step A's contract docs and ROADMAP

- **Rating.** Expert. **Seat.** tech-writer. **Wave.** W8.
- **Scope.** `SPEC.md`, `ARCHITECTURE.md`, `CONVENTIONS.md` and `ROADMAP.md`.
- **Design.** In §7:
  - every entry labelled "phase 3, step A" under SPEC.md, ARCHITECTURE.md and CONVENTIONS.md;
  - the step-A half of each entry that has two;
  - "ROADMAP.md (phase 3, step A)".

  The `Endpoint` field's name, `IdleConns`, is design §1's.
- **Facts.**
  - Find each quoted phrase by grepping a short fragment from its first line. For example:
    - `grep -n 'Items are walked one' ARCHITECTURE.md` (line 562);
    - `grep -n 'go through$' CONVENTIONS.md` (line 62), needed because "go through `model.Chat`"
      wraps after "through";
    - `grep -n 'Every chat call is one' ARCHITECTURE.md` (line 894).
  - The two ROADMAP "Next" items to delete: `grep -n 'Use goroutines to parallelize\|Same for most
    of the build asks' ROADMAP.md` answers lines 11 and 12.
- **Work.**
  - Land the given text.
  - Restate ARCHITECTURE §7's call path, and the probe's path, as the design directs.
- **Checks.**
  - `grep -n 'model\.Chat\|model\.Decision' ARCHITECTURE.md CONVENTIONS.md` answers only the §11
    `kept:` lines, which DB replaces.
  - `grep -n 'readerConcurrency' SPEC.md` answers only the superseded block line and sentence in
    §9.2.
  - `grep -n 'Reader concurrency' ARCHITECTURE.md` prints nothing.
- **Done-line.** The checks hold. Committing C3 is the coordinator's job.

---

## Phase 3, step B — retire the sync path

**Precondition for every package in this step.**

- C3 is committed.
- C3's commit holds A5's evidence tests, as A5's report names them, green at W8's wave-end gate.

That is the evidence that the queue does the job before the old path goes (design §3 inv. 12).

### B1 — seal `model.Client`

- **Rating.** Expert. **Seat.** go-coder. **Wave.** W9.
- **Scope.**
  - **Production:**
    - `internal/model/chat.go`, `client.go`, `decision.go`, `openai_client.go`, `anthropic_client.go`
      and `queue.go`;
    - a new file in `internal/model` for the test-double adapter;
    - `internal/model/modeltest/mockllm.go`.
  - **Tests:**
    - `cmd/build_env_test.go`, `cmd/build_test.go`, `cmd/configure_refs_test.go`,
      `cmd/configure_test.go`, `cmd/models_test.go`, `cmd/provider_test.go`,
      `cmd/score_record_test.go`, `cmd/score_scoring_test.go` and `cmd/score_test.go`;
    - `internal/asks/call/call_test.go` and `internal/build/inference_test.go`;
    - `internal/model/chat_test.go`, `decision_test.go`, `drain_test.go`, `openai_client_test.go`,
      `anthropic_client_test.go` and `queue_test.go`;
    - `internal/model/modeltest/mockllm_external_test.go` and `mockllm_test.go`;
    - `internal/modelclient/modelclient_test.go`.
  - No production file outside `internal/model`.
- **Design.** §1 "The seal". §5 "Step B", its first five bullets. AC 1.
- **Facts.**
  - The test users of the retiring path: `grep -rln --include='*_test.go'
    'model\.Chat(\|\.Decision(\|ChatAttempts\|ErrChatIncomplete\|ConsultDrained\|\.Consult(' .`
    answers eight files, every one in the scope above.
  - The doubles: `grep -rln --include='*.go' 'func (.*) ConsultStream(' .` answers the two real
    clients in `internal/model` plus five doubles, every one in the scope above.
  - The sites that hand a `*modeltest.MockClient` where a `model.Client` is taken: `grep -rln
    --include='*_test.go' 'modeltest' .` answers sixteen files, every one in the scope above.
  - Tests reach those sites through `internal/modelclient/modelclient.go:22` `type NewClientFunc
    func(model.Endpoint) model.Client`.
  - `internal/model/modeltest/mockllm.go:198` `resp, err := m.Consult(ctx, req)` calls `MockClient`'s
    own method, not `Client.Consult`.
  - `ListModels` stays exported: `internal/modelclient/modelclient.go:135` `infos, err :=
    client.ListModels(ctx)`.
  - Design §1 settles, from the Go specification's "Exported identifiers", that no importer can bypass
    a slot.
- **Work.** The design's step-B bullets on the seal, the deletions, the ported tests (driving a `Slot`
  from a queue of one) and the doubles.
- **Done-line.** `just test-pkg internal/model/...`, `just test-pkg internal/modelclient`,
  `just test-pkg cmd`, `just test-pkg internal/build`, `just test-pkg internal/asks/call`.

### B2 — remove the step-A adapters and the old knob's code

- **Rating.** Mechanical. **Seat.** go-coder. **Wave.** W10.
- **Scope.**
  - in `internal/asks/letters.go`, the step-A adapter and `DefaultReaderConcurrency`;
  - in `internal/claimgraph/stages.go`, `Options.Reader` and `Options.ReaderConcurrency`, and their
    uses in `stages.go`, `unmarked.go`, `classify.go` and `cull.go`;
  - in `internal/score/run.go`, the step-A adapter;
  - in `internal/score/invoke.go`:
    - `Invocation.NewAsk`, and the branch that builds the adapter from it;
    - `Report.Asked`, whose comment and setter move to the launch field A4 added:
      - **Setter.** `Asked` is set true once `Invoke` has called that field and it returned no
        error. Today that is `:143` `rep := Report{Asked: true}`, which follows `:139` `ask, err :=
        inv.NewAsk()`.
      - **Comment.** "Asked is whether the invocation opened its launch: `<field>` was called and
        returned no error." `<field>` is the name from A4's report.
  - the tests these touch: `internal/asks/call/call_test.go`, `internal/score/invoke_test.go`, and
    the tests of `internal/asks` and `internal/claimgraph`.
  - No `cmd` file and no `internal/build` file:
    - A2 moved `cmd/score.go:227` off `NewAsk`;
    - A2 removed `cmd/build.go:144-146`, `Provider.ReaderConcurrency` and `walk.readerConcurrency`.
  - The config field stays, because the key still loads.
- **Inputs from A3 and A4.** The coordinator puts into the dispatch:
  - the adapter names, from A3's and A4's reports;
  - the `Invocation` launch field's name, from A4's report, which the `Asked` comment names.
- **Design.** §2 "What step B removes".
- **Facts.**
  - `grep -rn 'NewAsk' --include='*.go' .` answered, at `aee74c1`:
    - `cmd/score.go:227`, which A2 rewrites;
    - `internal/score/invoke.go:64`, `:67`, `:75` and `:139`;
    - `internal/score/invoke_test.go:100`, `:115` and `:131`.

    After this package it prints nothing. The `invoke_test.go` cases move to a test-local `Launch`
    fake.
  - A3 made two places test `opts.Launch != nil || opts.Reader != nil`. This package drops the
    `Reader` half of each, leaving `opts.Launch != nil`:
    - `internal/claimgraph/stages.go`, in `Depends`, the `classificationFindings(` call;
    - `internal/claimgraph/classify.go`'s test of whether to ask.

    Under `--no-inference`, `Launch` is nil, as `Reader` is today.
  - Every reference to the old knob: `grep -rln --include='*.go' 'ReaderConcurrency' .` answered, at
    `aee74c1`:
    - `cmd/build.go`;
    - `internal/asks/letters.go`;
    - `internal/build/build.go` and `internal/build/rows.go`;
    - `internal/claimgraph/classify.go`, `cull.go`, `stages.go` and `unmarked.go`;
    - `internal/config/config.go` and `layer.go`;
    - the config tests.
- **Work.** Delete the listed code. `AskGroup` still sends a group's first ask alone.
- **Checks.**
  - `grep -rln --include='*.go' 'ReaderConcurrency' .` answers only files under `internal/config`.
  - `grep -rn 'NewAsk' --include='*.go' .` prints nothing.
- **Done-line.** The checks hold, plus `just test-pkg internal/asks/...`, `just test-pkg
  internal/claimgraph`, `just test-pkg internal/score`.

### B3 — the configuration fixture moves off `readerConcurrency`

- **Rating.** Mechanical. **Seat.** shell-dsl-coder. **Wave.** W9.
- **Scope.** `tools/scripts/test-integration-config.sh`, lines 613 and 615 only.
- **Design.** §5 "Step B", its last bullet.
- **Facts.** `grep -n 'readerConcurrency' tools/scripts/*.sh` answers:
  - `test-integration-config.sh:613: '' '[asks]' 'readerConcurrency = 2' > "${refsdir}/config.toml"`
  - `test-integration-config.sh:615: 'scoring = "local/clef"' '' '[asks]' 'readerConcurrency = 2' > "${out}/refs-expected.toml"`

  `concurrency` is a loaded key since A1, in C3.
- **Work.** Both lines change from `readerConcurrency = 2` to `concurrency = 2`.
- **Done-line.**
  - `bash -n tools/scripts/test-integration-config.sh`, as CONVENTIONS "Testing" directs after a
    script changes.
  - `grep -n 'readerConcurrency' tools/scripts/*.sh` prints nothing.
  - The recipe itself runs at W9's wave-end gate.

### DB — step B's contract docs

- **Rating.** Mechanical. **Seat.** tech-writer. **Wave.** W11.
- **Scope.** `ARCHITECTURE.md` and `CONVENTIONS.md`.
- **Design.** In §7, every entry labelled "phase 3, step B", and the step-B half of each entry that has
  two. The CONVENTIONS house rule goes in "## House Rules", directly after the bullet beginning
  "**No stage exits on a model's opinion.**" (`grep -n "No stage exits on a model" CONVENTIONS.md`
  answers line 230).
- **Facts.**
  - Each entry gives its text.
  - Find each quoted phrase by grepping a short fragment from its first line.
- **Work.** Land the given text.
- **Checks.**
  - `grep -n 'model\.Chat\|model\.Decision' ARCHITECTURE.md CONVENTIONS.md` prints nothing. At
    `aee74c1` it answered twelve lines: `ARCHITECTURE.md` 573, 869, 884, 885, 893, 979, 1273, 1274,
    1316 and 1317, and `CONVENTIONS.md` 63 and 69. DA and DB together replace every one.
  - `grep -n '^| I12' ARCHITECTURE.md` answers the new invariant's row.
- **Done-line.** The checks hold. Committing C4 is the coordinator's job.

---

## Post-sprint review

This section starts after C4. It holds the review, revision and burndown steps.

1. **Review.**
   - **Reviewers.** Two reviewers work independently:
     - an architect;
     - a coder, at the go-coder seat.
   - **Pre-task reading.** THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md. Neither reviewer
     gets this plan or the design: the sprint's outcomes now stand in the contract documents, and
     the review judges the changes against them.
   - **Brief.** The coordinator gives each reviewer the commit range: the commit C1 builds on,
     through C4. The question: do the changes meet the contract documents' standards, and are they
     fit for purpose?
   - **The tests are in scope.** The reviewers report:
     - duplicate coverage;
     - tests no logic error could fail;
     - stated invariants or acceptance behaviour no test exercises;
     - integration recipes that pass without exercising the behaviour they name.
2. **Burndown.**
   - The architect merges both reviews into one burndown, `ACTIVE_PLAN_REVIEW_BURNDOWN.md`, at the
     repository root. Each item carries a severity, the finding, its location and guidance.
   - That file instructs agents, so the prompt engineer reviews it before its first use, and the
     architect revises it on those findings.
   - **Pre-seeded item.** Besides the reviewers' findings, the burndown carries this rename:
     - `internal/asks/letters.go` becomes `internal/asks/multichoice.go`, and its test file follows;
     - `LetterQuestion` becomes `MultichoiceQuestion`, `LetterReader` becomes `MultichoiceReader`,
       and `LetterItem` becomes `MultiChoiceItem`;
     - the `Letter` member (`CallRecord`, `ItemRecord`) becomes `Choice`.
     - The rename covers every reference to those names across the module, tests included.
   - **Pre-seeded item.** The git seam `internal/ledger` becomes `internal/gitledger`, so it no longer
     shares a name with a scoring run's ledger (`internal/score/ledger.go`, which stays). The rename
     covers every importer and test, the exec-policy and import-policy tests' owner entries, and the
     ARCHITECTURE and CONVENTIONS text naming the package.
3. **Revision.**
   - Burndown items are dispatched under this plan's rules: each with a rating, a seat, and a
     done-line in runner targets.
   - Each fix pass ends with the wave-end gate and W11's extra recipes before it is committed.
