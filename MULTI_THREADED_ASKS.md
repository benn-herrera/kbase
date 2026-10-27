# Multi-threaded asks: design

This is the handoff a plan is written from. It covers the point where the generic Chat and Decision
endpoints (`internal/model/chat.go`, `internal/model/decision.go`) meet the jobs that use inference.
It is delivered in three phases:

1. a queue;
2. buffer-then-flush at every consumer, still on the sync calls;
3. bounded fan-out through the queue at every consumer (step A), then the sync path retired (step B).

Line numbers are as of commit `aee74c1`. The quoted text is the anchor.

---

## 0. Settled requirements

This document does not reopen them.

1. **API.** The queue is a counting semaphore of N tokens. `(*Queue).Go` waits for a
   token, then starts one goroutine for the item and hands it a `*model.Slot`. A `Slot` is the only
   value that can call chat or decision, and code outside `model` obtains one only from `Go`. §1
   "Shape" states the exact surface.
2. **Only the queue reaches the wire (phase 3, step B).** The raw sync chat loop and every method that
   puts a request on the wire become unexported. A `Slot`'s methods are then the only exported entry
   point to inference, and no importer can make an unbounded call.
3. **N.** N is set in `config.toml`, can be overridden by a flag, and defaults to 2.
4. **What may be written while asks are in flight.** The only permitted write is one file per result,
   unique to that result, in the invocation's state store. The answer cache is the model for this
   rule: content-addressed, one file per call.
   - **Ask batch.** An ask batch is the set of items one consumer launches before it waits for them
     all. §3 inv. 6 lists them.
   - **Write batch.** A write batch is one write op's `entry` list, all-or-nothing (SPEC §8).
   - **Common file.** A common file is any file that more than one result lands in. It is written
     only after its ask batch has drained. Common files include:
     - `ledger.jsonl`, `captures.jsonl`, `items.jsonl` and `progress.jsonl`;
     - the build records and the group records;
     - the KB record files, the leaves and the register pages.
5. **No item writes a file while it holds a token.**
   - An item holds its token across all of its calls: an ask and its re-ask, or the overview's ask
     and its structure re-ask. A re-ask therefore goes out right behind its original, while the
     prompt prefix is still cached.
   - The token is released when the item's last call returns, before the item writes its
     per-result files. Those writes are the tail of the item's own goroutine.
6. **Phase order.** Phase 2 runs on the sync calls, with every test green at its end. Phase 3 moves
   every consumer onto the queue (step A) and then retires the sync path (step B).
7. **What a killed score run keeps.** Each answered item gets one result file in
   `score/<run>/results/`.
   - **What counts as answered.** An item is answered when its call returned. That covers a priced
     answer, a reply the parse refused, a truncated reply, and a call that failed. These are exactly
     the attempts that leave a capture.
   - **What gets no file.** A text the guard refuses makes no call, so it gets no file. Neither does a
     call cancelled in flight, nor a skipped item.
   - **Who writes it.** The item's own goroutine writes the file, after its token is released
     (item 5).
   - **What it holds.** Everything the walk's end needs for that item: its ledger entry, as priced or
     as failed, and its capture.
   - **The walk's end.** Its flush folds every result file into the landing, the captures and the
     ledger, and then removes the files.
   - **The next `--resume`.** It folds whatever a kill left, before it re-prices. The fold works like
     this:
     - It applies the run's `Skip`. A result for an item `Skip` names is not landed, and the fold
       records no `done` or `failed` line for it: a person's value, set since the kill, stands. Its
       capture is still appended, because the call was made. The walk then records the item
       `skipped`, as it records any skipped item.
     - A result whose attempt the ledger already records, with the same item, bounds and prompt
       digest, is removed and not folded again.
   - A kill therefore forfeits only the calls that were in flight.
8. **Monitoring while asks are in flight.**
   - **Build.** `status` pauses for each ask batch.
     - A stage's `units` event is written before its asks.
     - Its unit and fallback events are written in one append after the ask batch: when the stage
       returns, or before it announces its next `units`.
     - Each held event is stamped when it is written, as `progress.emit` stamps every event
       (`internal/build/store.go:200` `e.Time = stamp(p.now())`).
     - `current.units-done` stays 0 for the ask batch's length, and a unit keeps its SPEC §6 meaning.
   - **Score.** `score.Status` overlays the run's result files on the ledger's last statuses.
     - Each result file counts under the status its entry records.
     - The count is exact, because there is one file per answered item and the total comes from
       `items.jsonl`.
     - It costs one directory read per poll.
9. **Prompt-prefix locality.** Asks reach the endpoint in the order they are submitted, so
   consecutive asks share prompt prefixes.
   - **Start order is exact within one consumer's launch loop.** One goroutine calls `Go` for a
     consumer, and each `Go` takes its token before the next `Go` begins. So items take their tokens
     in exactly the order the consumer submits them. Their requests leave in that order, up to
     goroutine scheduling between two consecutive launches. Completion order is arbitrary.
   - **Only one consumer launches at a time per invocation.**
     - A build walks its stages one at a time: `internal/build/build.go:329` `for _, s := range
       remaining {` calls `w.stage(s)` in turn.
     - Within a stage, rows run one at a time: `build.go:514` `for i, r := range run {`.
     - The overview is the last stage: `internal/buildstore/stages.go:39` `{"overview-drafted",
       "overview drafted"},`.
     - A score process runs either a scoring run or a replay, never both: `cmd/score.go:558-562`
       `verb := runScore` … `verb = replay`.
     - A scoring run makes one `score.Invoke` (`cmd/score.go:232`), which walks once. Resume's fold
       and its re-price make no model call.
     - `probe` makes one call.
   - **A group stays together.** `AskGroup` sends a group's first ask alone, waits for it, then
     launches the rest. A stage launches its groups in order, from the goroutine that runs the stage.
     So the items in flight span at most the tail of one group and the head of the next.
   - **Goroutines are bounded.** At most N item goroutines are alive, plus one launch loop per
     consumer. No item goroutine is ever parked waiting for a token.
10. **Per-trail state is kept per build trail, and pruned by count (phase 2).**
    - **Why.** A fresh build mints new claim ids, so its prompts differ from every earlier trail's,
      and an old trail's answers can never hit again. The answers pay off only within one trail: the
      fresh start, every resume, and every `rewind-build` reset on it. The letter-ask group records
      are named by claim ids too, so each fresh build adds a new set.
    - **Where it lives.** A trail's answers and group records live in
      `scratch/trails/<start-commit>/answers/` and `scratch/trails/<start-commit>/asks/`.
      `<start-commit>` is the trail's `start` boundary commit, made before any inference.
    - **What is kept.** At a build's start, the current trail's directory and the newest five others
      stand. Every other trail directory is removed whole. A trail counts once, however many
      invocations it took.
    - **The upgrade.** The flat `scratch/answers/` and `scratch/asks/` an earlier kbase wrote are
      removed on the first build after the upgrade.
    - **Unchanged.** The hit rule and the keys are as they are: content-addressed, with the attempt
      ordinal in the key. Writes follow item 4.
    - **`status`.** `cache-entries` and `cache-bytes` cover the answers of every trail directory.
    - **Mechanism.** §5 "The per-trail directory".
11. **`progress.jsonl` is rotated (phase 2).**
    - **When.** Only in a fresh build, never in a resume. It happens once the build's launch checks
      have passed and it has stages to walk, and only when the live file holds more than
      `progressRotateLines` lines. A build that ends refused or `retry` before that point never
      rotates.
    - **Rotation.**
      - The earlier runs' lines move to `progress-<UTC timestamp>.jsonl`.
      - The current run's lines stay in the live file. They never land in a rotated file.
      - The rotated files are pruned to the newest 15, so 16 files stand at most.
      - Nothing reads a rotated file.
    - **Mechanism, sizing and the readers' safety.** §5 "Progress rotation".

---

## 1. The queue and the slot

**Shape.** This section is the one statement of `Go` and `Slot`. Names not given here are the coder's
choice.

- `NewQueue(n int) *Queue`.
  - It does not check n: the n it receives has already passed the check at config load and at flag
    parse (§2). CONVENTIONS "A check earns its place by what it reads" applies.
  - The `Queue` is a buffered channel of n tokens and nothing else. It starts no goroutine, owns no job
    channel, and has nothing to close.
- `(*Queue).Go(ctx context.Context, wg *sync.WaitGroup, item func(*Slot) (after func())) error`.
  - It blocks until a token is free. A `ctx` that ends first returns `ctx.Err()`, and nothing is
    started.
  - With the token taken, it calls `wg.Add(1)` and starts one goroutine, which:
    1. runs `item(slot)`, which makes every call the item needs through `slot`;
    2. releases the token the moment `item` returns, panic included, so the slot is spent from then
       on;
    3. runs `after`, where non-nil;
    4. calls `wg.Done()`.
  - The release point is the return of `item`. Anything that writes a file goes in `after`
    (§0 item 5).
- `(*Slot).Chat(ctx, c Client, req ChatRequest)`.
  - `ChatRequest` carries the model, the system prompt, the prompt and the backoff.
  - The result carries every attempt in order, the last attempt being the outcome. The capture
    reads the attempts from the result, so no callback is needed.
- `(*Slot).Decision(ctx, c Client, req DecisionRequest)`. The result carries the response.
- `(*Slot).Waited()` is how long `Go` blocked before it took this slot's token.
- **A slot carries one call at a time.** Its methods hold a mutex across the call, so handing the slot
  to a second goroutine cannot put two calls on one token.
- **A slot is good only inside `item`.** A call on a spent slot, on a zero `Slot` that `Go` did not
  issue, or with a nil client, returns an error and does not panic.
- **Deadlines.** A per-call deadline is set inside the item, around the call, with `WithTimeout`. It
  counts from the call's start and never covers the wait for a token, because that wait happens in
  the launch loop before the item exists.

**The consumer.**

- It launches its items in submission order from one goroutine, then waits on its `WaitGroup`.
- Each item writes only its own result cell, the position in the consumer's results that belongs to
  it, and writes its per-result files in `after`.
- After the wait, the consumer collates the result cells and writes its common files, exactly as in
  phase 2.
- **Cancel.** Two things happen:
  - A `Go` blocked on a token returns `ctx.Err()`, and the loop stops launching.
  - A launched item's call sees its `ctx` end and returns, with nothing recorded for it.

  An item whose call completed before the cancel writes its files in `after` as usual. That written
  result is the paid inference a resume reuses.
- **Write errors.** An error from a per-result write is recorded in the item's own result cell. The
  consumer handles it as it handles any first error (§3 inv. 11).

**Consumer seams.** `internal/asks` and `internal/score` may not import `model` (ARCHITECTURE §11).
Each therefore declares its own launch seam in its own vocabulary, and the package that holds `model`
fills it.

- **`asks.Launch`** (declared in `internal/asks`).
  - It takes `ctx`, the consumer's `*sync.WaitGroup`, and an item function.
  - The item function receives a `LetterReader` bound to the item's slot, and returns
    `after func(writeErr error)`.
  - `internal/asks/call` fills it. The bound reader holds the capture and answer files of every
    call the item makes. The wrapper's `after` writes those files, then calls the item's `after`
    with the write error.
- **The overview's seam** (declared in `internal/build`, beside `walk.passage`).
  - It takes `ctx`, a `*sync.WaitGroup`, and an item function.
  - The item function receives a passage asker bound to the item's slot, with the shape of today's
    `walk.passage` (`internal/build/build.go:138`). It returns `after func(writeErr error)`.
  - `internal/asks/call` fills it over `Passage`, holding and writing files as `asks.Launch` does.
- **`score.Launch`** (declared in `internal/score`).
  - It takes `ctx`, the consumer's `*sync.WaitGroup`, and an item function.
  - The item function receives an `Ask` bound to the item's slot, and returns `after func()`.
  - `cmd` fills it, with `scoringAsk`'s call built over the slot.
- **Step-A adapters.** Until step A's wiring lands, `internal/asks` and `internal/score` each hold
  one adapter that fills its own seam without `model`:
  - in `asks`, an adapter over a plain `LetterReader`, bounded by `readerConcurrency` as `AskGroup`'s
    semaphore bounds today;
  - in `score`, an adapter over a plain `Ask`, one item at a time.

  Step B deletes both.

**The seal (phase 3, step B).**

- `model.Client`'s `ConsultStream` and `Decision` become unexported, and so do the same methods on
  `OpenAIClient` and `AnthropicClient`. That seals the interface.
- `model.Chat` becomes the unexported `chat`. `Slot.Chat` and `Slot.Decision` are then the only
  exported way to make a call, and only `Go` issues a `Slot`.
- Two exported functions are deleted with their tests. No production code outside `model` calls
  either, and the slot path does not either:
  - **`Consult`.** `grep -rn --include='*.go' '\.Consult(' . | grep -v _test.go` returns one hit,
    `internal/model/modeltest/mockllm.go:198`. That hit is `MockClient.ConsultStream` calling
    `MockClient`'s own `Consult` method, not `Client.Consult`. Every other `.Consult(` call is in a
    test. `MockClient` may keep that method as its own; it stops being part of `model.Client`.
  - **`ConsultDrained`** (`internal/model/client.go:127`).
- `ListModels` stays exported. It reads the catalogue rather than doing inference, and
  `internal/modelclient/modelclient.go:135` calls it.
- **The adapter for test doubles.** A test double outside `model` implements the sealed interface
  through one exported adapter in `model`, whose function fields supply each wire method. A test
  still reaches inference only through a `Slot` from `Go`.
  - This follows the sealing pattern of `testing.TB`'s `private()`: `grep -n 'private()' "$(go env
    GOROOT)/src/testing/testing.go"` answers `1001:	private()`.
- **Why no importer can bypass the bound.** Go's visibility rule settles it. The Go specification,
  "Exported identifiers": an identifier is exported only if its name begins with an upper-case
  letter, and a selector naming an unexported identifier compiles only inside the package that
  declares it.
  - So no package outside `model` can call a real client's wire methods, or take one as a method
    value to fill an adapter field.
  - A type outside `model` that embeds a `model.Client` gets the unexported methods promoted into
    its method set, but its own package still cannot name them.
  - The only exported callers, `Slot.Chat` and `Slot.Decision`, need a `Slot` that `Go` issued and
    has not yet spent.

**Idle connections.** A client keeps at least N idle connections to its host.

- **Today.** `internal/model/transport.go:47` `tr := http.DefaultTransport.(*http.Transport).Clone()`
  leaves `MaxIdleConnsPerHost` unset. So `net/http`'s `DefaultMaxIdleConnsPerHost = 2` applies, and
  with N above 2 every connection past two is closed and redialled. On a remote endpoint that costs a
  TLS handshake.
- **The field.** `model.Endpoint` gains `IdleConns int`.
- **The transport.** It sets `MaxIdleConnsPerHost` to the larger of `IdleConns` and 2, and
  `MaxIdleConns` to the larger of `IdleConns` and the cloned default, 100 (`net/http`'s
  `transport.go:54` `MaxIdleConns: 100,`). So the total never binds before the per-host count.
- **How N gets there.**
  - `modelclient.New` gains the count, and `Registry.Get` puts it on every `model.Endpoint` it builds
    (`internal/modelclient/modelclient.go:69` `client := r.newClient(model.Endpoint{Name:
    ep.Provider, …`).
  - `cmd` passes the invocation's N where it builds a queue, and 0 elsewhere, which leaves the
    defaults.
- **One client per endpoint, one transport per client.** `Registry.Get` returns the client it already
  opened for a key (`modelclient.go:62` `if client, ok := r.opened[key]; ok {`). Each client builds
  its own transport (`internal/model/openai_client.go:45` `http: newHTTPTransport(),`, and
  `anthropic_client.go:63` likewise). A registry lives for one invocation.
- **No import rule moves.** `modelclient` already imports `model`, and `cmd` already imports
  `modelclient` (ARCHITECTURE §11).

---

## 2. The knob: one, not two

`[asks] readerConcurrency` (SPEC §9.2, ARCHITECTURE §12, default 4) bounds one group's letter asks
once the group's first ask has returned. `AskGroup` enforces it with a semaphore:
`internal/asks/letters.go:251` `sem := make(chan struct{}, concurrency)`.

The queue's N replaces it. There is one knob. The old key is superseded in step A, in the package that
stops reading it.

- **Key.** `[asks] concurrency`, default 2. It counts model calls in flight in one invocation, across
  every role. A value under 1 fails the strict load, naming `asks.concurrency`.
  - The created `config.toml` template's `[asks]` block opens at `internal/config/template.go:26`
    `# [asks]`.
  - The key's package inserts these two lines directly after that line:

    ```
    # concurrency = 2        # model calls one build or score has in flight at once,
    #                        # across every role; a whole number, at least 1.
    ```
  - The supersession then deletes the two `readerConcurrency` lines that follow:
    - `template.go:27` `# readerConcurrency = 4  # asks of one group in flight once its first has`
    - `template.go:28` `#                        # returned; a whole number, at least 1.`
- **Flag.** `--ask-concurrency N`, on `build` and `score` only.
  - A value under 1 is a usage refusal that names the flag. The precedent is `--limit`'s refusal,
    `cmd/score.go:128` `return refused(nil, toolresult.Item{Check: checkUsage, Key: "--limit", ...`.
  - Precedence is the flag, then the key, then 2.
  - It is a per-command flag, not a global one. A tool's schema is built from the command's local,
    non-persistent flags: `internal/mcp/mcp.go:223` `cmd.LocalNonPersistentFlags().VisitAll(func(f
    *pflag.Flag) {`.
    - A flag declared on `buildCmd.Flags()` or `scoreCmd.Flags()` is one of those, so it reaches the
      `build` and `score` tool schemas with no table edit.
    - The contract states the same: `ARCHITECTURE.md:1536` and `SPEC.md:1996`.
  - `build`'s resume carries it. `resumeCommand` (`internal/build/build.go:661-679`) carries each of
    `build`'s own flags but `--through` (`cmd/build.go:219-223`: `--bibliography`, `--through`,
    `--state-dir`, `--no-inference`), plus `--config-dir`. It names each one explicitly, so
    `--ask-concurrency` is added there.
  - `score`'s resume does not carry it, because it changes no judgement. This matches `--timeout` and
    `--limit`, which SPEC §8's `resume` paragraph does not list either.
  - The development replay accepts it. `ask-concurrency` is added to `replayFlags`
    (`cmd/score_replay.go:43` `var replayFlags = []string{"replay", "pass", "failed", "limit",`).
  - `probe` takes no flag and runs a queue of one.
- **Where the queue is built.** In `cmd`, in a helper in one of the three files that already import
  `model`: `cmd/build.go`, `cmd/score.go` or `cmd/probe.go`. The replay reaches that helper through
  `cmd/score.go`. The file-level import list therefore does not move:
  `cmd/importpolicy_test.go:463` `"internal/model": {"cmd/build.go", "cmd/score.go", "cmd/probe.go"},`.
- **`readerConcurrency` is superseded (step A),** following the precedent of the `provider` key
  (SPEC §9). This lands in the same package that stops `cmd/build.go:144` reading the key, so no tree
  ever feeds an unchecked value to `AskGroup`'s semaphore.
  - The key loads and merges, and chooses nothing.
  - Its under-1 check is deleted, `internal/config/config.go:142` `if n :=
    cfg.Asks.ReaderConcurrency; n != nil && *n < 1 {`, with its message constant `:62` `const
    keyReaderConcurrency = "asks.readerConcurrency"`.
  - **The stderr line.** A file-mode `build` or `score` whose merged `config.toml` sets the key
    writes this one line to stderr:

    ```
    config.toml states the superseded [asks] readerConcurrency key; kbase does not use it — set asks.concurrency instead
    ```

    It is written by a helper beside `supersededProvider` (`cmd/provider.go:100`). The helper is
    called at exactly two sites, the two where `supersededProvider` is called for a file-mode build or
    score:
    - `cmd/build.go:115`;
    - `cmd/score.go:290`.

    `supersededProvider` has four other callers (`configure`, `models`, `probe` and `show-config`),
    and the new helper is not called beside them.
  - It leaves the schema at the next major, recorded as a ROADMAP line.
  - **Deleted with the read.** `build.Provider.ReaderConcurrency` and `walk.readerConcurrency` go in
    the same step. Their one setter is the block at `cmd/build.go:144-146`, and their one reader is
    `internal/build/build.go:381`.
- **What stays from the old knob.** `AskGroup` still sends a group's first ask alone, so the server
  caches the shared prefix once. That is ordering, not a bound.
- **What step B removes.**
  - the step-A adapters (§1 "Consumer seams"). The `asks` adapter carries the old bound once
    `AskGroup` takes a launch seam in step A, so `AskGroup` has no `concurrency` parameter left;
  - `asks.DefaultReaderConcurrency`;
  - `claimgraph.Options.ReaderConcurrency` and `claimgraph.Options.Reader`.
    - Step A's stage package already tests `opts.Launch != nil || opts.Reader != nil` wherever
      `Reader` was tested against nil: `internal/claimgraph/stages.go:538`
      `classificationFindings(c, opts.Reader != nil)` in `Depends`, and `classify.go:55`'s `reader`,
      tested at `:84`, `:89` and `:99`.
    - That is so a tree where the build passes `Launch` and no `Reader` still classifies with
      inference.
    - Step B drops the `Reader` half;
  - `score.Invocation.NewAsk`. After step A its only setter, `cmd/score.go:227`, sets the launch
    instead.

  The config field stays, because the key still loads.
- `sheet.renderConcurrency` counts `dot` processes, a different resource. It stays.

---

## 3. Invariants

1. **One queue per invocation.** `build`, `score`, the replay and `probe` (N=1) each build one queue
   in `cmd` from the resolved N.
   - Once step A's wiring lands, every Chat and Decision call the invocation makes goes through a
     `Slot` from that queue.
   - The step-A adapters (§1) bound only paths that wiring has replaced. Step B removes them, and
     from then on nothing else bounds model calls in flight.
2. **A token covers model calls, never a file write** (§0 item 5, §1 "Shape").
   - An item may read the answer cache while it holds its token: one read, before the call it could
     save.
   - `model` touches no file. The import-policy test pins it (`cmd/importpolicy_test.go`).
3. **A per-call deadline counts from the call's start** (§1 "Deadlines").
   - `score --timeout` and `probe --timeout` wrap the call inside the item.
   - The caller's `ctx` cancels both the wait for a token, in `Go`, and a call in flight.
4. **Call semantics do not move.** A slot runs today's retry loop and `Decision` unchanged:
   - `internal/model/chat.go:75` `func Chat(ctx context.Context, c Client, modelID, system, prompt
     string, backoff []time.Duration, observe func(ChatAttempt)) (ChatAttempt, error) {`;
   - `internal/model/decision.go:67` `func (c *OpenAIClient) Decision(ctx context.Context, req
     DecisionRequest) (DecisionResponse, error) {`.

   That covers the attempt count, the backoff, the stop on a non-retryable status, and the
   classification of outcomes. A backoff pause holds its token, and that is accepted. `FirstToken` is
   still measured inside the attempt (`chat.go:162-163`).
5. **Results are position-addressed.** Each answer goes into its item's result cell (an index or an
   identity), never appended in arrival order.
   - Every common file is written in the order the stage or the set already defines.
   - The build-record encoders already sort, in `internal/claimgraph/buildrecords.go`:
     - the classification and unmarked records through `sortedPairs`;
     - the cull record through `slices.SortFunc`.
   - The KB landing and the score files take result-cell order.
   - N therefore changes wall time and nothing else (SPEC §5: the build is reproducible modulo ids).
6. **Ask batches and the write rule (§0, items 4 and 5).**
   - **Build.** A stage's asks form one ask batch, with two exceptions:
     - `claims-culled` runs two ask batches, because pass 2's asks depend on pass 1's answers.
     - The overview is one ask batch of one item. That item holds its token across its ask and its
       structure re-ask, because attempt 2's prompt quotes attempt 1's reply.
   - **Score.** One invocation's walk is one ask batch.
   - **Replay.** Its walk is one ask batch.
   - **Probe.** One item, and it writes no file.
   - **Per-result files** are written in the item's `after`:
     - in the build, `scratch/trails/<start-commit>/answers/<digest>` and
       `scratch/captures/<stem>-<hash>-<attempt>.capture.jsonl`, one of each per call;
     - in a score run, `score/<run>/results/<item>` (§0, item 7).
   - **Common files** are written after an ask batch has drained:
     - in the build, once per stage, after the stage's last ask batch;
     - in score and in the replay, once per invocation.
   - **Logging.** Log lines are diagnostics, not results, so the log file is not a common file. Items
     log through `internal/log`. `model` has no logger.
7. **Stopping.**
   - **Build.** On a stopping error or a cancel:
     - the launch loop stops, and the stage's in-flight calls are cancelled;
     - the ask batch drains;
     - the stage writes no common file.

     Resume restores to the boundary and re-runs the stage (SPEC §6). The answer files already
     written stand, so the resumed stage asks nothing it already holds.
   - **Score.**
     - On every orderly exit (done, bounded, cancelled, failed), the walk drains and then flushes.
     - The flush runs on a context the cancel did not end. It is one landing and two appends.
     - It runs before the end refresh, which a cancelled run already performs before it releases
       the holder lock (SPEC §8: "One more `refresh` follows the last write — on completion, on
       `--limit` and on cancellation").
     - `kbase cancel` waits up to `cancelWait` for the holder lock: `internal/build/monitor.go:14`
       `const cancelWait = 20 * time.Second`.
     - Where the run is still flushing when that wait runs out, `cancel` answers `retry`
       (`StopHolder`, in the same file) and the run completes its flush regardless. No correctness
       rests on the flush's duration.
     - A kill leaves result files. The next `--resume` folds them (§0, item 7).
8. **Separation.**
   - `model` knows nothing of files, roles or consumers.
   - `asks/call` writes only its per-result files, in `after` (§1 "Shape").
   - `claimgraph`, `asks` and `score` know nothing of N or of `model`. They see only their launch
     seams, and the `LetterReader` and `Ask` those seams hand an item.
   - `claimgraph` still never reaches `model` (the import-policy test).
9. **The score landing.**
   - A write batch is all-or-nothing (SPEC §8, `**Batches** are all-or-nothing.`).
   - A refused write batch writes nothing, and its refusal names each offending entry (`entry`, SPEC
     §7). The run fails those items and re-issues the write batch without them, until it lands or is
     empty.
   - A write batch that fails, as opposed to being refused, fails every item it carried.
   - A landing that cannot take the KB write lock fails every item it carried. Each such item's line
     carries `write.HeldLockItem`'s detail.
   - The lock is uncontended in practice. The run holds the operation lock, and a write verb refuses
     while that is held (SPEC §8 "One write lock").
   - The re-price lands its moved values through the same landing function, and keeps its own rule:
     a refused entry fails the resume, with the ledger as it stood
     (`internal/score/ledger.go:992` `if err := w(a.asItem(), value); err != nil {`).
10. **Shared state.**
    - Item goroutines write only their own result cells.
    - Progress events, fallbacks, totals and record accumulation run on the consumer's goroutine,
      after the ask batch, reading the result cells. Together with §0 item 8, this also serializes the
      progress emitter (`internal/build/store.go:199` `func (p *progress) emit(e buildstore.Event)
      error {`) and its unit counter (`internal/build/rows.go:309` `done++`) without a lock.
    - Every input an item reads is built before it is launched. `References`' `bodies` memo
      (`internal/claimgraph/unmarked.go:213` `bodies := map[string]string{}`) is a map written on
      first use, so a group's items are composed in the launch loop.
    - The score runner composes each item's prompt in its launch loop, before it launches the item.
    - `just test-race` is the gate.
11. **Errors.**
    - An ask batch reports its first error by arrival. That error cancels the rest of the ask batch.
    - A sibling's cancellation error is never reported as the cause.
    - Today `decide` wraps any error, a cancelled `ctx` included, as `asks.IncompleteError`
      (`internal/asks/call/call.go:160` `return nil, asks.IncompleteError{`). The overview call
      returns `ctx.Err()` instead (`call.go:407` `case ctx.Err() != nil:`).
    - `AskGroup` picks the first error by index: `internal/asks/letters.go:272` `for _, err := range
      errs {`. With items running at once, that can report a cancelled sibling as an inference
      failure.
    - The fix: `decide` returns `ctx.Err()` when its `ctx` has ended, and the consumer keeps the first
      error that arrives.
12. **Retire only after the replacement has done the job.** Phase 3 step B lands only after step A's
    evidence has landed, in an earlier wave (§5).

**Standing invariants.**

- **Logging.** `model` does not log, and the queue takes no logger. Failures return as errors to
  callers that already log through `internal/log`.
- **Boundary checks.** N is user input. It is checked at config load and at flag parse. A slot refuses
  a nil client, a spent slot and a zero slot with an error, never a panic.
- **Instrumentation.** A slot reports its token wait (`Waited`).
  - A build capture records that wait next to `duration-ms`, which stays the call's own run time.
  - A score capture's `started` and `ended` bound the `Ask` call inside the item, and so exclude the
    wait.
  - `probe` measures `wall-ms` inside its item, around the call, so it includes no token wait.
  - Nothing new is switchable. Captures are already always on.
- **Build.** The existing `just` targets cover this work. No new target is needed.

**Failure modes these invariants close:**

| Likely wrong turn | Closed by |
|---|---|
| A queue per stage or per role | 1 |
| A semaphore or pool kept beside the queue after step B | 1, §2 |
| An item writes a file while holding its token | 2, §0 item 5 |
| A `WithTimeout` wrapped around `Go`, timing items out for their wait | 3, §1 |
| Records or ledger in completion order | 5 |
| A common file written while asks are in flight | 6 |
| One refused landing entry fails the whole score landing | 9 |
| Groups launched from per-group goroutines, breaking start order | §0 item 9 |
| A data race on progress or a lazily filled map | 10 |
| A cancelled sibling reported as the stage's cause | 11 |
| A slot shared across goroutines, putting two calls on one token | §1 |

**Deliberately unspecified:**

- how `Go` recovers the token on a panic;
- the result types' field names, and the seam types' exact names;
- the file name of a stage's group-record file, and of a score result file.

---

## 4. Module skeleton

| Module | Owns | Does not |
|---|---|---|
| `internal/model` | `Queue` (the tokens and `Go`) and `Slot` (`Chat`, `Decision`, `Waited`). The retry loop, which becomes the unexported `chat` in step B. The sealed wire methods and the test-double adapter. `Endpoint.IdleConns` and the transport it sets. | Read config, log, know roles, touch files, or start a goroutine but an item's |
| `internal/modelclient` | Passing the invocation's idle-connection count onto every `model.Endpoint` it builds | Build the queue |
| `internal/config` | The `[asks] concurrency` key: its schema, merge, under-1 refusal and template lines. `readerConcurrency` superseded. | Build the queue |
| `cmd` (`build.go`, `score.go`, `probe.go`; the replay through `score.go`) | Resolving N (the flag, then the key, then 2); building the queue; giving the registry its idle-connection count; filling `score.Launch`; handing the queue to `build.Provider`; probe's one item | Launch a consumer's items |
| `internal/asks/call` | Filling `asks.Launch` and the overview's seam over the queue. Captures, and the answer cache in the trail directory the build names, held while an item calls and written in its `after`. | Bound concurrency, choose the cache's directory, or write a common file |
| `internal/asks` | The `asks.Launch` seam and its step-A adapter. `AskGroup`: the first ask launched alone and waited for, then the rest launched in order, its `GroupRecord` collated once the consumer has waited. One function that writes a stage's group records as one file. | Import `model`, own a goroutine, or hold a semaphore (from step B on) |
| `internal/claimgraph` | One launch loop per stage, groups launched in order, results by result cell, and one write of each record per stage | Import `model` or know N |
| `internal/score` | The `score.Launch` seam and its step-A adapter. `Runner.Run` and `Replay.Walk` launching in set order, results by result cell, the per-item result file, the landing's write batches, the flush, and folding results on `--resume`. | Import `model` or know N |
| `internal/build` | Holding a stage's unit and fallback events until the ask batch has drained (§0, item 8). The overview's seam and its one item. The queue, passed through on `Provider`. The per-trail directory, its key and its pruning (§0, item 10). Rotating `progress.jsonl` (§0, item 11). | Own N |

The dependency direction is unchanged: `cmd` → {`build`, `score`, `modelclient`} → `model`, and
`claimgraph` → `asks` (the seam only).

---

## 5. Phases and caller inventory

### Phase 1: the queue

Add `Queue` and `Slot` (§1) beside the unchanged `model.Chat` and `Client.Decision`.

- No consumer changes, and no key or flag lands yet. The key and the flag land in step A, with the
  queue's first consumers.
- The import-policy test gains the rule that `internal/model` imports no kbase package and not `os`.
- Tests go in `internal/model`, run with `just test-race`:
  - never more than N items alive at once;
  - items take their tokens in the order `Go` was called;
  - a `ctx` that ends while `Go` waits returns its error and starts nothing;
  - a `ctx` that ends during a call ends the call;
  - the token is released before `after` runs: with N=1, the next item starts while the previous
    one's `after` is still running;
  - a spent slot and a zero slot refuse a call;
  - one slot never carries two calls at once;
  - `Waited` reports the wait;
  - N=1 with many items completes.

Inventory: none. This phase adds code and calls nothing that exists today.

### Phase 2: per-result files during asks, common files after (sync calls kept)

Re-run the inventory with:

```
grep -rn --include='*.go' -E 'WriteNodePass\(|WriteUnmarked\(|WriteClassification\(|WriteCull\(|w\.landLeaf\(|writeGroupRecord\(|IssueCulls\(|issue\(w\.raw|\.Append\(it|\.capture\(it|appendLine\(&comparisons|landValue\(|atomicfile\.Write\(answer|os\.Create\(capturePath|w\.passage\(' internal cmd | grep -v _test.go | grep -v 'func '
```

The table covers inference consumers only. `internal/claimgraph/stages.go:350-359` (`Declared`) and
`stages.go:442` (`Equations`) write without inference and are out of scope.

| Site | Today | Phase 2 |
|---|---|---|
| `internal/asks/call/call.go` `decide` and `ask` | The capture is opened before the call (`:107` and `:377`, `f, err := os.Create(capturePath)`) and filled during it; the chat attempts come through the observe callback (`:393`) | Compose the capture in memory. Write the capture, then the answer, once each, from the goroutine that made the call, after the call returns. These are per-result files. The callback appends to a slice and touches no file. |
| `internal/asks/letters.go` `AskGroup` (`:305` `if err := writeGroupRecord(recordDir, rec); err != nil {`) | Writes its group record per group | Writes nothing. Each stage writes the group records it collected as one file, through one `asks` function, into the directory the stage is given. |
| `internal/claimgraph/stages.go` `Discovered` (`:183` and `:197` `WriteNodePass`, `:191` `w.landLeaf`) | Per leaf: `WriteNodePass` twice, then `landLeaf` (one insert write batch per leaf) | Hold each leaf's verdicts and claims. After the ask batch, make one insert write batch across all leaves, its entries in sorted leaf order. Then one `WriteNodePass`, whose bytes equal what today's sequence leaves last. Fallbacks follow on the stage goroutine. Equivalence is in the notes below the table. |
| `internal/claimgraph/unmarked.go` `References`/`askPlanned` (`:210`, `:288` `WriteUnmarked`) | The plan is written before the asks, then `WriteUnmarked` runs per source group | One `WriteUnmarked`, carrying the plan and the pairs, after the ask batch. The coverage re-read (`:225` `landed, _, err := buildrecords.ReadUnmarked(`) follows it. |
| `internal/claimgraph/classify.go` `classify` (`:79` `return WriteClassification(repoRoot, rec)`, called per group at `:132`) | `save()` per source group, `--no-inference` included | One `save()` after the loop |
| `internal/claimgraph/cull.go` `IssueCulls` (`:337`; `internal/claimgraph/elision.go:15`) | Three write ops per culled claim | Three write batches, one per op, ids in plan order. Equivalence is in the notes below the table. |
| `internal/claimgraph/cull.go` `WriteCull` (`:346`) | One write | Unchanged. It already follows the last ask batch. |
| `internal/claimgraph/stages.go` `ringRow` (`:571` `issue(w.raw, write.DemoteClaimEdge, …)`) | One `demote-claim-edge` per cut | One write batch over the cuts, in cut order. `index.BreakCycles` computes every cut on one graph before any cut is issued (`:567` `_, cuts, items := index.BreakCycles(g, hosted)`). |
| `internal/claimgraph/stages.go` `writeEdges` (`:539`) | One `add-build-edges` write batch | Unchanged |
| `internal/build/rows.go` `overview` (`:410`, the write of `overview.md`) | One write after its asks | Unchanged. Its calls' capture and answer follow the `call.go` row. |
| `internal/build/rows.go` `unitProgress` (`:301` `func (w *walk) unitProgress() claimgraph.Progress {`) | One event appended per unit and per fallback | §0, item 8. The `units` event is written before the ask batch. Unit and fallback events are held, and written in one append when the stage returns or before it announces its next `units`. |
| `internal/score/run.go` `Runner.Run` (`:183` `r.Ledger.capture`, `:188` `r.Ledger.Append`) and `internal/score/invoke.go` `landValue` (`:248`) | Per item: land under its own lock hold, then capture, then the ledger line | Per item: ask, parse and price into the result cell, then write the item's result file (§0, item 7). After the walk: one landing (inv. 9), then the captures, then the ledger lines, each in one append in walk order, skipped lines included. Then the run's result files are removed. |
| `internal/score/ledger.go` `Resume` (`:250`) and `reprice` (`:960`) | Re-prices per moved line through the `Write` seam | First fold the result files a killed invocation left (§0, item 7). Then re-price through the landing's write batches. |
| `internal/score/replay.go` `Replay.Walk` (`:317`, `:320`, `:327`; `!kbasedist`) | Per item: capture, ledger line, comparison | Held in memory. On every return after the walk asked anything (done, cancelled or failed), each of the three files gets one append, in the replay's item order. There is no result file, because a replay has no resume. A replay directory that the prune removed fails the open, as the end-of-walk check does today (`:343`). |
| The per-trail directory: `internal/asks/call/call.go` `New` (`:51`), `internal/build/build.go` `launch` and `run`, `internal/build/rows.go` `claimGraph` (`:286`), `internal/build/store.go` beside `pruneCaptures` (`:240`), `internal/buildstore/store.go` `answerCache` (`:227`) | One flat `scratch/answers/` and one flat `scratch/asks/`, never pruned | §0 item 10; the mechanism below |
| Progress rotation: `internal/build/build.go` `run` (after `:314`) and `internal/build/store.go` beside `openProgress` (`:191`) | One `progress.jsonl`, appended forever | §0 item 11; the mechanism below |

**Why one insert write batch across leaves equals the per-leaf sequence.**

- **Asks.** A leaf's asks read the tree read once at the stage's start, not the leaves landed before
  it: `stages.go:153` `leaf := readLeaf(t.Documents[p], &inv)`.
- **Adoption.** Inside one write batch, a minted claim joins the write batch's graph
  (`internal/write/inserts.go:447` `b.g.Claims = append(b.g.Claims, want)`). Its marker updates the
  write batch's hosts (`inserts.go:328` `b.hosts[id] = subject`). Adoption by host
  (`inserts.go:371` `adoptByHost`) reads both, so a later entry adopts exactly what it would after
  the earlier leaf's own write.
- **Failure.** A refusal on any leaf fails the stage in both forms, and the resume restores the
  stage's paths.

**Why three per-op write batches for the cull refuse no more than the sequence.**

- `drop-claim-edges` and `drop-inbound-edges` check only that their id is a node
  (`internal/write/edges.go:135` and `:156` `b.nodeKind("id", id)`). Every culled node still stands
  when those two write batches run.
- `dropClaimNodes` (`internal/write/marks.go:114`) drops every entry, then refuses the mentions
  that remain once all the drops are composed: `:125` `return refuseRemainingMentions(b, entries)`.
- So the write batches refuse a subset of what the sequence refuses. They differ in one case: a
  culled claim named only in another culled claim's prose. The sequence refuses it; the write batch
  accepts it, because the naming record is dropped in the same write batch.

**The per-trail directory (§0 item 10).**

- **Key.** `<start-commit>` is the commit of the newest entry in the walk's trail whose stage is
  `start`.
  - The walk reads the trail at `internal/build/build.go:289` `trail, err := w.repo.Trail()`, newest
    first.
  - It extends the trail as each stage records, at `:499` `w.trail = append([]ledger.Entry{{Stage:
    s.ID, Commit: commit, Body: body}}, w.trail...)`.
  - `start` is the first stage (`internal/buildstore/stages.go:29` `{"start", "build started"},`), so
    every inference row runs after the key exists. A test in `internal/buildstore` pins that order:
    every `Step` in `buildstore.Steps` (`internal/buildstore/stages.go:84`) whose `CallsAModel()`
    (`:77`) is true belongs to a stage after `start` in `buildstore.Stages`.
- **Where the walk holds it.** The walk keeps the current trail's directory in a field. It sets the
  field on the stage goroutine in two places: when it reads the trail, and when `start` records.
  - Both happen before any item goroutine of a later stage is launched, so every read happens after
    the write.
  - A walk with no `start` entry has not reached an inference row, so nothing reads the field
    unset.
- **The answers directory.** The callers are built in `walk.launch` (`build.go:379-380`), before a
  fresh build records `start`. So `call.New` takes the answers directory as a function, which
  returns the field joined with `answers`.
  - The `Caller` calls that function at every cache read and every cache write, and keeps no copy.
- **The group records.** They go to the field joined with `asks`. `claimGraph` passes that path at
  each stage, where `rows.go:286` passes `filepath.Join(w.stateDir, buildstore.ScratchDir,
  asks.AskRecordsDir)` today. That covers the once-per-stage group-records file of the phase-2 row
  above.
- **Retention.**
  - **When.** Once a build has passed its launch checks (`build.go:314` `if err :=
    w.launch(remaining); err != nil {`), where stages remain to walk, and before `w.prepare`.
  - **What it does.**
    - It lists `scratch/trails/` alone.
    - It keeps the current trail's directory and the newest `keptBuilds` other trail directories
      (`internal/build/store.go:233` `const keptBuilds = 5`), and removes every other trail directory
      whole.
    - Where the current trail's directory does not stand, it creates it. That happens on a resumed
      trail that recorded `start` but wrote no entry yet, and on the first resume after the upgrade,
      whose flat entries the prune removes.
    - It then sets that directory's modification time to now, with `os.Chtimes`.
    - It removes `scratch/answers/` and `scratch/asks/` whole, where they stand.
  - **Newest.** "Newest" is by the trail directory's modification time. A trail directory's own
    modification time changes only when an entry directly in it changes: `atomicfile.Write` creates
    its temp file in the target's own directory (`internal/atomicfile/atomicfile.go`, `replace`:
    `createTemp(filepath.Dir(target), …)`), which is `answers/` or `asks/`, not the trail directory.
    So the `os.Chtimes` above is what marks a trail as last built on.
  - **Fresh builds.** A fresh build has no current trail at that point, and keeps the newest five.
    Its trail directory is created when its first entry is written, which sets that directory's
    modification time.
- **No store-format change.** `buildstore.StoreFormat` stays 1. An older kbase on such a store reads
  and writes its own flat directories, and the next build here removes them.
- **`status`.** `answerCache` (`internal/buildstore/store.go:227`) counts the regular files of every
  `scratch/trails/*/answers/`. It counts no file of the old flat `scratch/answers/`, because the next
  build removes that.

**Progress rotation (§0 item 11).**

- **Sizing.** `wc -l test-data/transient/test-integration-build-live-arxiv/2609.10318v1/state/progress.jsonl`
  answers `85`. That is one full build with inference of the live recipe's paper, and the largest
  single-build record on this host. A `--no-inference` build of each arXiv paper writes 32 lines
  (`test-data/transient/test-integration-build-arxiv/*/state/progress.jsonl`). Five such builds with
  inference make about 425 lines, so `progressRotateLines` is 500. A larger paper's units raise its
  count, and rotation then comes after fewer builds. That only rotates sooner; it loses no state any
  reader needs (below).
- **Where.** In `w.run`, directly after the per-trail prune, at the same point:
  - after the launch checks (`internal/build/build.go:314` `if err := w.launch(remaining); err !=
    nil {`);
  - where stages remain to walk;
  - where the trail records no `start` (`:312` `resumed := recorded[buildstore.StageStart] != ""` is
    false);
  - before `w.prepare`.

  A build that ends refused, `retry`, `unchanged` or `bounded` before that point never rotates.
- **What the live file holds there.** Exactly one line of this invocation: its run event.
  - `Run` writes that event at `build.go:199` `if err := w.progress.emit(buildstore.Event{Event:
    buildstore.EventRun, …`.
  - Every other event is written after the launch checks, or at the run's end. The emit sites are
    `build.go:199` (run), `:211` (end), `:500` (recorded), `:511` (stage) and `:529` (unit), and
    `internal/build/rows.go:306`, `:310` and `:313`, inside a stage.
  - Between `:199` and `:314`, `Run` calls `pruneCaptures`, `awaitWriters` and `w.run`'s trail
    read, refusal checks and `launch`. None of them emits.
- **Mechanism.** Where the live file holds more than `progressRotateLines` lines:
  1. Close the progress record's open file.
  2. Read the live file, and split off its last line, this invocation's run event.
  3. Write every line before it to `progress-<stamp>.jsonl`, through `atomicfile.Write`.
  4. Write the run event alone as `progress.jsonl`, through `atomicfile.Write`, which replaces the
     live file in one rename.
  5. Reopen the live file for appending, as `openProgress` opens it, into the same `*progress`,
     replacing its `f` (`internal/build/store.go:186-189` `type progress struct { f *os.File; now
     func() time.Time }`).
     - It must not assign a new value to `w.progress`. `Run`'s `defer w.progress.close()`
       (`build.go:198`) bound its receiver when the defer ran, so a new value would leave the
       reopened file unclosed.
  6. Prune the rotated files to the newest `keptProgressFiles` (15), by name.

  This order has three consequences:
  - The live file always stands whole.
  - The current run's lines never land in a rotated file.
  - A kill between steps 3 and 4 leaves the earlier lines in both files. That is harmless, because
    nothing reads a rotated file.
- **Names.** `atomicfile`'s temp files are named `.<target>.…` (`internal/atomicfile/atomicfile.go:27`
  `func tempPrefix(target string) string { return "." + filepath.Base(target) + "." }`), so the prune,
  which lists `progress-*.jsonl`, never sees one.
- **The stamp.** `<stamp>` is the walk's clock, `opts.now()`, in UTC, in Go layout
  `20060102T150405.000000000Z`. Its nanoseconds are fixed-width, so names sort by time, and two
  rotations cannot share a name.
- **Failures.** A failure of any step returns from `w.run` as its error, so the build ends `failed`.
  - A failure in steps 2–4 still runs step 5 before returning, so `Run`'s end event (`build.go:211`)
    lands in the live file.
  - Where step 5 itself fails, the end event is lost, and `Run` logs that (`:212`
    `opts.Logger.Error("the progress record lost the build's end", …`).
- **Nothing reads a rotated file.** `internal/buildstore/store.go:141` `f, err :=
  os.Open(filepath.Join(dir, ProgressFile))` is the one read of the record.
- **The readers stay safe with fewer builds in the live file.** Each prunes less or reports less,
  never more:
  - **Capture retention.** `pruneCaptures` returns before it removes anything while the live file
    holds fewer than `keptBuilds` ended builds: `internal/build/store.go:251` `if len(starts) <
    keptBuilds {` … `return nil`.
  - **`status`, `wait` and the MCP watcher.** All three read `readStore`
    (`internal/buildstore/status.go:129` `func readStore(dir string) (storeReading, error) {`).
    - The rotating build holds the store, so the state is `running`, whatever the record holds
      (`status.go:157` `case r.live:`).
    - The live file holds this run's run event from step 4 on, so the store's newest invocation is
      this build, before and after the rotation.
    - Only a build that passed its launch checks rotates, so no refused build leaves a store whose
      earlier runs were rotated away.
  - **The dirty-path check on resume** (`internal/build/store.go:218` `uncommittedIn`). It reads only
    the newest invocation that entered a stage. A rotation happens only for a fresh build, and a fresh
    build's earlier invocations recorded no `start`. Before `start` records, nothing writes a
    kbase-owned path, so no dirt a rotated record would have accounted for exists.

Phase 2 changes no call site of `model`. A test that pins a per-unit write moves with the contract,
and its commit states the truth it now tracks (CONVENTIONS "Coordinator Policy"). Done-line:
`just test`, `just test-race`, `just test-integration`.

### Phase 3: bounded fan-out through the queue, then retire the sync path

Re-run the inventory with:

```
grep -rn --include='*.go' -E 'model\.Chat\(|\.Decision\((ctx|callCtx)|\.ConsultStream\(|\.Consult\(|ConsultDrained\(' cmd internal | grep -v _test.go | grep -v '^internal/model/'
grep -rn --include='*.go' -E 'asks\.AskGroup\(|^func AskGroup|^func \(r \*Runner\) Run\(|^func \(rp \*Replay\) Walk\(' internal | grep -v _test.go
```

The answers are as of this design's base commit. Phase 2 moves some of these lines.

Call sites:

- `cmd/probe.go:114` (`Decision`) and `:124` (`Chat`);
- `cmd/score.go:321`, in `scoringAsk` (`Chat`). It serves both the run and the replay
  (`cmd/score_replay.go:153`).
- `internal/asks/call/call.go:131`, in `decide` (`Decision`);
- `internal/asks/call/call.go:393`, in `ask` (`Chat`). It serves the overview through `Passage`
  (`internal/build/build.go:382`).

Launch loops:

- the `AskGroup` call in each stage: `stages.go:247` (in `askLeaf`, `:242`), `unmarked.go:268`,
  `classify.go:109` and `cull.go:399`;
- `internal/asks/letters.go:235` `AskGroup`;
- `internal/score/run.go:134` `Runner.Run`;
- `internal/score/replay.go:262` `Replay.Walk`;
- the overview, `internal/build/rows.go:366` `func overview(w *walk) (result.Record, error) {`.

Test-side users of the retiring path, found with
`grep -rln --include='*_test.go' 'model\.Chat(\|\.Decision(\|ChatAttempts\|ErrChatIncomplete\|ConsultDrained\|\.Consult(' .`:

- `internal/model/{chat,decision,drain}_test.go`, which are external `model_test` files;
- `internal/model/{anthropic_client,openai_client}_test.go`, which are in-package and switch to the
  unexported names;
- `internal/model/modeltest/mockllm_external_test.go`, which follows `MockClient` onto the adapter;
- `cmd/score_test.go` and `internal/build/inference_test.go`.

Client doubles outside `model`, found with `grep -rln --include='*.go' 'func (.*) ConsultStream(' .`:

- `internal/model/modeltest/mockllm.go`
- `cmd/provider_test.go`
- `cmd/score_test.go`
- `internal/asks/call/call_test.go`
- `internal/build/inference_test.go`

**Step A: consumers move onto the queue.**

- **Key, flag and supersession.** `[asks] concurrency` and `--ask-concurrency` land, as does the
  replay's `replayFlags` entry. `readerConcurrency` is superseded (§2).
- **The idle-connection count** reaches the transport (§1 "Idle connections").
- **The seams.**
  - `asks.Launch` and `score.Launch` land, each with its step-A adapter (§1 "Consumer seams").
  - `cmd` builds the queue, fills `score.Launch`, and hands the queue to `build.Provider`.
  - `internal/build` declares the overview's seam, and `internal/asks/call` fills it and
    `asks.Launch` over the queue.
- **`call.Caller`.** Inside an item, it calls `slot.Decision` and `slot.Chat`. It holds each call's
  capture and answer in memory, and writes them in `after`. Each capture records the attempts the
  result carries, and the slot's wait.
- **`scoringAsk` and `probe`.**
  - Each builds its call inside an item, over the item's slot, with `--timeout` wrapped around the
    call there.
  - Probe launches one item on a queue of one, and measures `wall-ms` inside it.
  - The probe's document is unchanged:
    - its fields come from `probeAnswerFields` (`cmd/probe.go:273`), fed by the last attempt's
      `ChatAttempt`, which the slot's result carries unchanged;
    - `first-token-ms` is still measured inside the attempt (`chat.go:162-163`).
  - `TestProbeReportsTheCall` (`cmd/probe_test.go:152`) pins the keys.
- **Stages.** Each stage runs one launch loop, on the goroutine that runs the stage, and launches its
  groups in order:
  - `Discovered`, through `askLeaf`, by leaf;
  - `askPlanned` and `classify` by source;
  - `askCull` by group within each pass, with pass 2 starting only after pass 1 has drained.

  Each group's items are composed in the loop (inv. 10).
- **`AskGroup`.** It launches a group's first ask, waits for that item, then launches the rest into
  the stage's `WaitGroup` and returns. The group's record is collated after the stage waits. Its
  re-ask runs inside the same item. `AskGroup` keeps the first error by arrival (inv. 11).
- **Overview.** One item holds its token across both attempts, and parses attempt 1's reply between
  them. `overview.md` is written after the wait.
- **Build's resume.** `resumeCommand` gains `--ask-concurrency N` where the invocation named the flag,
  carried on `build.Options` like `ConfigDir` (§2).
- **`Runner.Run` and `Replay.Walk`.** Each decides `Status` and `Skip`, honours `Limit`, and composes
  every prompt in its launch loop. It launches the items it will ask, in set order.
- **Evidence (done-line).** `just test`, `just test-race` and `just test-integration` are green, with
  every consumer on the queue:
  - **The bound (AC 2).** It holds.
  - **Submission order (AC 12).** It holds.
  - **N-independence (AC 4).** It holds, shown by a test at N=1 and N=4 with deterministic doubles:
    - **Build.** In `internal/build`, following the precedent of `inference_test.go`. It compares
      through `claimgraph.CompareModuloIDs` (`internal/claimgraph/modulo_ids.go:43`) over
      `BuiltBy` (`:24`). `ComparedKB` (`:19`) holds `NodePass`, `Classification` and `Unmarked`
      but no cull record path, so the comparison gains the cull record.
    - The six existing callers of `sameModuloIDs` must stay green with it:
      `internal/build/build_test.go:307`, `:661`, `:700` and `:717`, and
      `internal/build/inference_test.go:358` and `:569`. Each compares two builds of one input, so
      SPEC §5's determinism modulo ids covers the cull record too.
    - **Score.** `ledger.jsonl` and `captures.jsonl`, compared modulo stamps and durations.

**Step B: retire. Precondition: step A's evidence, landed in an earlier wave.**

- Unexport `model.Chat` as `chat`, which `Slot.Chat` calls.
- Unexport `Client.Decision` and `Client.ConsultStream`, and the clients' same methods, as `decision`
  and `consultStream`. This seals `Client` (§1).
- Delete `Client.Consult` and `ConsultDrained` with their tests. The slot path calls neither.
- Port the tests:
  - `chat_test.go`'s and `decision_test.go`'s cases drive a `Slot` from a queue of one, so they keep
    their retry and backoff claims without creating an in-package and external test pair.
  - Cases in `drain_test.go` that pin only `Consult` or `ConsultDrained` are deleted.
  - Cases whose claim the stream path still makes are ported the same way.
- **Move the five doubles onto the `model` adapter (§1).**
  - Once `model.Client` is sealed, a `*modeltest.MockClient` is no longer a `model.Client`. Every site
    that hands one where a `model.Client` is taken therefore moves with it.
  - That includes the `modelclient.NewClientFunc` factories in tests
    (`internal/modelclient/modelclient.go:22` `type NewClientFunc func(model.Endpoint) model.Client`).
  - Find them with `grep -rln --include='*_test.go' 'modeltest' .`, which answers:
    - `cmd/build_env_test.go`, `cmd/build_test.go`, `cmd/configure_refs_test.go`,
      `cmd/configure_test.go`, `cmd/models_test.go`, `cmd/provider_test.go`,
      `cmd/score_record_test.go`, `cmd/score_scoring_test.go` and `cmd/score_test.go`;
    - `internal/build/inference_test.go`;
    - `internal/model/chat_test.go`, `internal/model/decision_test.go` and
      `internal/model/drain_test.go`;
    - `internal/model/modeltest/mockllm_external_test.go` and
      `internal/model/modeltest/mockllm_test.go`;
    - `internal/modelclient/modelclient_test.go`.
- Delete what §2 "What step B removes" lists.
- Move two fixture lines in `tools/scripts/test-integration-config.sh`, lines 613 and 615, from
  `readerConcurrency = 2` to `concurrency = 2`.

---

## 6. Acceptance criteria

1. **The seal.** No package outside `internal/model` can put a request on the wire except through a
   `Slot` that `(*Queue).Go` issued and has not yet spent. The compiler enforces this: the wire
   methods and the chat loop are unexported, and `Consult` and `ConsultDrained` no longer exist.
2. **The bound.** At no moment does an invocation have more than N model calls in flight, across all
   its roles, for any N ≥ 1.
3. **N.** N comes from `--ask-concurrency`, else `[asks] concurrency`, else 2.
   - A value under 1 is refused, naming the flag or the key.
   - A `config.toml` holding `readerConcurrency` loads, changes nothing, and draws the one stderr line
     §2 gives.
4. **N-independence.** Against a deterministic provider, the following are identical for N=1 and N=4:
   - a build's tracked records and `kb-root/`, modulo minted ids;
   - a score run's ledger and captures, modulo stamps and durations.
5. **Common files.**
   - While any ask is in flight, no common file (§0, item 4) changes.
   - Each inference stage writes each of its build records once.
   - A score invocation takes the KB write lock once for its landing, and writes `captures.jsonl` and
     `ledger.jsonl` once each.
6. **No file under a token.** No item writes a file while it holds a token, and the queue starts no
   goroutine but an item's.
7. **No timeout for waiting.** With N=1 and items queued behind a slow call, no score item fails for
   time spent waiting for a token.
8. **Resume after a stop.**
   - A build cancelled or killed mid-stage and then resumed asks nothing it holds an answer file for.
   - A score run killed mid-walk and then resumed asks nothing whose result file was written (§0,
     item 7).
   - A cancelled score run records a line for every item answered before the stop.
9. **Probe.** `probe`'s result document is unchanged in keys and meaning.
10. **Tool schemas.** The MCP `build` and `score` tool schemas carry `ask-concurrency`. The tool table
    is otherwise unchanged.
11. **Gates.** `just test`, `just test-race` and `just test-integration` pass at the end of every
    phase. In phase 2 they pass on the sync calls.
12. **Submission order.**
    - Within one consumer, items take their tokens in the order the consumer launches them.
    - A group's first ask completes before the rest of that group starts.
13. **Idle connections.** A client the registry builds for an invocation whose N is above 2 keeps at
    least N idle connections per host.
14. **Per-trail state is bounded by trail.**
    - After a build's start, `scratch/trails/` holds at most the current trail's directory plus five
      others.
    - The current trail's answers and group records survive every resume on that trail.
15. **The progress record is bounded.**
    - A fresh build whose live `progress.jsonl` exceeds `progressRotateLines` starts a new one.
    - At most 15 rotated records stand.
    - A resume never rotates.

---

## 7. Contract changes

Each change lands in the checkpoint whose code makes it true. The text below states §0 items 7–11.

**Finding the old text.** Many quoted phrases wrap across lines in the document they come from. Find
each one by grepping a short fragment from the phrase's first line, never the whole phrase.

**Invariant numbers.** Phase 2 adds ARCHITECTURE I11, and step B adds I12. So the table reads I10,
I11 between the two checkpoints, and I10, I11, I12 at the end.

**The consequence to state plainly.** A crash or kill forfeits what had not yet reached a
per-result file:

- the calls in flight;
- every common file the ask batch had not written.

In a build, the resume re-runs the stage from its boundary. The answer files carry the paid
inference across, so only the calls in flight are paid for again. In score, the next `--resume` folds
the result files, so only the calls in flight are asked again.

**SPEC.md**

- **§9.2 `[asks]` block and its prose (phase 3, step A).**
  - Block: replace the `readerConcurrency = 4` line with two lines:
    - `concurrency = 2  # optional; model calls one build or score has in flight at once, across
      every role`;
    - `readerConcurrency  # SUPERSEDED — loads, and chooses nothing; removed at the next major`.
  - Prose: replace the `readerConcurrency` sentences (from "`readerConcurrency` is how many" through
    "naming `asks.readerConcurrency`.") with "`concurrency` is how many model calls one invocation of
    `build` or `score` has in flight at once, across every role; absent, it is 2, and
    `--ask-concurrency` overrides it for that invocation. A value under 1 fails the load, naming
    `asks.concurrency`. Calls start in the order the invocation submits them. `readerConcurrency` is
    superseded: it loads and chooses nothing, a file-mode `build` or `score` says so in one line on
    stderr naming `asks.concurrency`, and it leaves the schema at the next major
    ([ROADMAP.md](ROADMAP.md))."
- **§5 and §8 synopses (phase 3, step A).** Add `[--ask-concurrency N]` to `build` and `score`, with
  one sentence: "a value under 1 is a usage refusal naming `--ask-concurrency`; `build`'s `resume`
  carries it, `score`'s does not, since it changes no judgement."
- **§6 `kbase status` (phase 2).** After "Progress is persisted to a file so a late or reconnecting
  monitor catches up.", add: "An inference stage's units and fallbacks are recorded once its asks
  have all returned, so while they are in flight `current` names the stage and its total with none
  done."
- **§6, a score run's status (phase 2).** For "read from the run's `items.jsonl` and the status
  `ledger.jsonl` last records for each", use "read from the run's `items.jsonl`, the status
  `ledger.jsonl` last records for each, and, while the run walks, each answered item's result file".
- **§6 `kbase cancel` (phase 2).**
  - Old: "The in-flight call is abandoned with nothing written for it" and "Cancel costs only the
    in-flight unit or item".
  - New: "The calls in flight are abandoned with nothing written for them" and "Cancel costs only
    the calls in flight: an answer already returned stands — in a build's answer cache, and in a
    score run's ledger".
- **§6, "The state store is pruned only by a build or a scoring run" (phase 2).** For "The answer
  cache and the stage reports are kept." (`grep -n 'The answer cache and the stage' SPEC.md` answers
  line 351), use: "A build trail's answer cache and letter-ask records are kept in one directory per
  trail, named by the trail's `start` boundary commit. At a build's start, before its first stage,
  the current trail's directory and the newest five others stand, and the rest are removed whole,
  along with the undivided cache and records an earlier kbase kept. At the same point, a fresh build
  whose progress record exceeds 500 lines moves the earlier runs' lines to a file under a timestamped
  name, keeping its own in the record; the 15 newest such files are kept. The stage reports are
  kept."
- **§8 "One write lock" (phase 2).**
  - Old: "`score` takes it around each landing".
  - New: "`score` takes it around its one landing — every value the walk produced, after the walk".
- **§8 score, "Landing" (phase 2).**
  - Old: "Each op runs with `--no-refresh`, under the KB write lock taken and released for that one
    landing. A write refusal is that item's `failed`, and so is a landing that cannot take the lock".
  - New: "The run lands every answered value at once when its walk ends: one `set-rigor` batch and one
    `set-bearing` batch, each `--no-refresh`, under one hold of the KB write lock. An entry a batch
    refuses is that item's `failed`, and the batch is re-issued without it; a landing that cannot take
    the lock fails every item it carried".
  - The lock sentence that follows stands. SPEC's "batch" here is the write batch of SPEC §8's
    `**Batches** are all-or-nothing.`
- **§8 score, `--timeout` (phase 3, step A).** After "the deadline for one item's call", add "counted
  from the call's start, never while the call waits for its turn".
- **§8 score, "The ledger and resume" (phase 2).**
  - In the records list, add after `captures.jsonl`: "and, while a walk runs, one result file per
    answered item, which the walk's end folds into the ledger".
  - After "appended after the item's write and its validation, never before", add: "A run writes its
    captures and then its ledger lines once, in the set's order, when its walk ends — on completion,
    on `--limit`, on cancellation and on a failure that stops it."
  - For "`--resume RUN` continues a run: it refuses a ledger for another KB root, format or pass,
    skips every item the ledger records `done`", use "… pass, first lands and records every answer a
    stopped invocation left in a result file — a value a person has set since being left as it
    stands — then skips every item the ledger records `done`".
- **§8 score, `captures.jsonl` (phase 2).** For "A capture is appended before the item's ledger
  line, a resume appends to the same file", use "A run's captures are appended before its ledger
  lines, at the walk's end, and a resume appends to the same file".
- **§8 score, "Outcomes" (phase 2).**
  - Old: "stopped it at an item boundary, with no line recorded for the item in flight and the ledger
    standing as it was".
  - New: "stopped it, with no line recorded for an item whose call was in flight or not yet made, and
    a line for every item answered before the stop".
- **§8 score, "The closing record" (phase 3, step A).** For "the walk's first item carrying no
  attempt, the one it stopped before", use "the walk's first item carrying no attempt".
- **§7 `score` keys (phase 2).** For `scored`, "in the order it landed them", use "in the walk's
  order".

**ARCHITECTURE.md**

- **§2, new I11 (phase 2).** "While an ask batch's asks are in flight, the only file written is one
  per result, unique to it, in the state store — a build's answer and capture per call, a score run's
  result per item — written by the goroutine that made the call after it returns; every file more
  than one result lands in is written after the ask batch has drained. An ask batch is a build stage's
  asks (each cull pass and the overview its own), a score walk or a replay walk."
- **§2, I11 (phase 3, step A).** For "written by the goroutine that made the call after it returns",
  use "written by the item that made the call, after it has released its token".
- **§2, new I12 (phase 3, step B).** "Every model call an invocation makes — a letter ask, the
  overview call, a scoring ask, a probe — is made through a `model.Slot`, which only the invocation's
  one `model.Queue` issues, from `Go`, and only while one of its N tokens is free; nothing else bounds
  model calls in flight. An item holds its token across all of its calls and releases it before it
  writes a file. Within one consumer, items take their tokens in the order the consumer launches
  them. The methods that put a request on the wire are unexported, so the compiler holds every other
  package to a slot."
- **§2 I10 (phase 2).**
  - Old: "Every ask a build or a scoring run makes leaves".
  - New: "Every ask whose answer a build or a scoring run recorded leaves".
- **§6 Write API row (phase 2).** For "a scoring run takes it around each landing, from
  `internal/score`", use "a scoring run takes it around its one landing, from `internal/score`".
- **§6 "Provenance" (phase 2).** For "`internal/score`'s landing (`landValue`, a landing and a
  re-price alike)", name the landing function the coder lands, then "a landing and a re-price alike".
- **§6 Scoring, "The runner" (phase 2, then phase 3, step A).**
  - Phase 2:
    - For "prices them, writes and appends `done`", use "prices them into the item's result cell and
      writes its result file; when the walk ends the run lands the result cells' values and appends
      their captures and lines".
    - For "`Run` hands it to `Ledger.capture` before `Ledger.Append`", use "the walk's end writes
      every capture before every line".
    - For "A cancelled context stops before the next ask and records nothing for the item in flight",
      use "A cancelled context stops the walk and records nothing for an item without an answer".
  - Phase 3, step A:
    - Old: "Items are walked one at a time, in the set's order, with no concurrency."
    - New: "Every item's prompt is composed in the walk's launch loop, which launches the items in
      the set's order through the run's `Launch` seam, at most N at once; each answer is held in its
      item's result cell and written in the set's order when the walk ends."
- **§6 "The ask is one seam" (phase 3, step A).**
  - Old: "fills the seam with `model.Chat`".
  - New: "fills the seam, per item, with its `Slot`'s `Chat`, the item's `--timeout` applied from the
    call's start".
- **§6 "The ledger and the state directory" (phase 2).**
  - Old: "appends each attempt as one JSON line with `O_APPEND` and `Sync`".
  - New: "appends the walk's lines in one write, with `O_APPEND` and `Sync`, when the walk ends".
  - After "drops a tail torn after its last newline so the item it names is asked again", add: "then
    folds the result files a stopped invocation left — landed through the run's landing, recorded,
    removed; one whose attempt the ledger already records is removed alone — before it re-prices".
- **§6 "The captures" (phase 2).**
  - For "`Ledger.capture` appends one line to `captures.jsonl`", use "the walk's end appends every
    capture to `captures.jsonl` in one write".
  - For "`Run` writes the capture before the item's ledger line", use "the captures are written before
    the lines".
- **§6 "The one-op landing" (phase 2).** Restate it in these terms:
  - one write batch per op, under one lock hold;
  - refused entries failed, and the write batch re-issued without them (inv. 9);
  - the verb's single end refresh stands.
- **§6 "The re-price" (phase 2).** For "landed through that seam first — `set-rigor`/`set-bearing`,
  under the KB lock as a landing holds it", use "landed first through the landing's write batches,
  under the KB lock as a landing holds it, any refused entry failing the resume".
- **§6 "The run's entry" (phase 2).** For "each value landed by `landValue`", use "the values landed
  by the run's one landing when the walk ends".
- **§6 "The development replay" (phase 2, then phase 3, step A).**
  - Phase 2. After "It writes only under `score/<run>/replays/<replay-id>/`", add: "each of its three
    files appended once, when the walk ends".
  - Phase 3, step A. In "`scoreReplayRun` takes `--pass`, `--failed`, …", add `--ask-concurrency`.
- **§7, the `internal/asks` bullet (phase 2).** In "the caller (system fragment, per-call captures
  and the answer cache under the state store's `scratch/`)", add "each written once, by the goroutine
  that made the call, after it returns, the answers into the current build trail's directory".
- **§7, the `internal/asks` bullet (phase 3, step A).** Replace "by the goroutine that made the call,
  after it returns" with "once the item that made the call has released its token".
- **§7 bullets 1 and 2, and the letter-ask bullet (phase 3, step A).**
  - Restate the call path as a `model.Slot`'s `Chat` and `Decision`, issued by the invocation's
    `model.Queue`.
  - Rewrite `probe`'s path (`probeEndpoint` → `Registry.Get` → `model.Decision` …) as one item on a
    queue of one.
  - In the bullet beginning "A letter ask is one `POST` to the entry's decision surface", replace
    "(`model.Decision`;" with "(a `Slot`'s `Decision`;".
  - In the bullet beginning "Every chat call is one tool-less chat request" (`grep -n 'Every chat call
    is one' ARCHITECTURE.md` answers line 894), after its first sentence, add: "The transport keeps
    at least N idle connections per host, N being the invocation's concurrency, so its connections
    are reused rather than redialled."
  - Check: once step B's entries have landed, `grep -n 'model\.Chat\|model\.Decision'
    ARCHITECTURE.md CONVENTIONS.md` prints nothing.
- **§8 "Resume over an interrupted stage" (phase 2).**
  - Old: "A cancelled or killed build leaves the stage's per-leaf work uncommitted in kbase-owned
    paths."
  - New: "A stage writes its build records and its `kb-root/` work once its asks have all returned,
    so a stop during the asks leaves those paths as the boundary left them, and a kill during that
    write leaves them partial; either way it is uncommitted."
  - Old: "The node-pass and classification records are still written per leaf and per group, but
    resume does not read them for position."
  - New: "Each build record an inference stage writes is written once, after its asks; resume reads
    none of them for position."
  - In "the answer cache (`scratch/answers/`) is what carries the paid-for inference across", replace
    `scratch/answers/` with `scratch/trails/<start-commit>/answers/` (`grep -n 'the answer cache
    (`scratch' ARCHITECTURE.md` answers line 1037).
- **§8 "Progress units" (phase 2).**
  - Old: "so `status` moves during an inference stage; a defaulted item is a `fallback` event. The
    node-pass, unmarked and classification records are written per leaf or per group; the cull record
    once".
  - New: "a stage's `units` event is written before its asks and its unit and `fallback` events in
    one append after them, so `status` moves between ask batches; every build record is written once
    per stage, after its asks".
- **§8 "Retention" (phase 2).**
  - Old: "`scratch/answers/` and the stage reports are never pruned, and `status` and `cancel` remove
    nothing. `status` computes `cache-entries` and `cache-bytes` from `scratch/answers/`." (`grep -n
    'and the stage reports are never pruned' ARCHITECTURE.md` answers line 1099.)
  - New: "Once its launch checks pass and before its first stage, a build prunes `scratch/trails/`,
    one directory per build trail named by the commit of the newest `start` entry of the trail the walk
    stands on, holding `answers/` and `asks/`: it keeps the current trail's directory, marked with the
    current time, and the newest `keptBuilds` (§12) others by modification time, and removes the rest
    whole, with the undivided `scratch/answers/` and `scratch/asks/` an earlier kbase wrote. At the
    same point, a fresh build whose `progress.jsonl` holds more than `progressRotateLines` (§12) lines
    moves every line before its own run event to `progress-<UTC timestamp>.jsonl`, leaves its run
    event alone in the record, and keeps the newest `keptProgressFiles` (§12) such files; nothing
    reads them. The stage reports are never pruned, and `status` and `cancel` remove nothing.
    `status` computes `cache-entries` and `cache-bytes` over every trail's `answers/`."
- **§8 layout (phase 2).** In "`scratch/` — `captures/` (…), `answers/` (…), `asks/` (composed
  prompts), …", replace the `answers/` and `asks/` entries with: "`trails/<start-commit>/`, one per
  build trail, holding `answers/` (the answer cache, content-addressed by model, system prompt and
  prompt, written through `atomicfile`; an ask's attempt ordinal is part of the key so a re-ask
  reaches the model) and `asks/` (the letter-ask group records, one file per stage)". After
  `progress.jsonl`, add "and up to `keptProgressFiles` rotated `progress-<UTC timestamp>.jsonl`
  files". `grep -n 'composed prompts' ARCHITECTURE.md` answers line 1110.
  - `score/<run>/` adds `results/`: one file per answered item, until the walk's end or the next
    resume folds it.
- **§8 "Status, progress, cancel, wait" (phase 2).**
  - For "the holder abandons the in-flight call with nothing written for it", use "the holder abandons
    the calls in flight with nothing written for them".
  - Add a sentence after the paragraph's account of how `score.Status` reads a run: "`score.Status`
    also counts the run's result files while it walks."
- **§11 package map (phase 3, step A).** In the `internal/asks/call` line, replace "through
  model.Decision" and "through model.Chat" (`ARCHITECTURE.md:1273-1274`) with "through a Slot's
  Decision" and "through a Slot's Chat".
- **§11 package map (phase 3, step B).** In `kept: … model (…)` (`ARCHITECTURE.md:1316-1317`), replace
  the `model.Chat`/`model.Decision` wording with "model (the `Queue` and its `Slot`, the only exported
  entry to inference)".
- **§12 Constants.**
  - Phase 2. For "the ended builds whose captured output and call captures survive pruning (§8)", in
    the "Builds kept" row, use "the ended builds whose captured output and call captures survive
    pruning, and the build trails, beside the current one, whose directories survive it (§8)".
  - Phase 2. Add the row: "Progress rotation | `progressRotateLines` in `internal/build`: 500, the
    lines a fresh build's `progress.jsonl` may hold before it is rotated, about five builds;
    `keptProgressFiles`: 15, the rotated files kept (§8)".
  - Phase 3, step A. Replace the "Reader concurrency" row with: "Ask concurrency | `[asks]
    concurrency` in `config.toml` (SPEC §9.2), `--ask-concurrency` on `build` and `score`, default 2;
    the model calls one invocation has in flight, and the idle connections per host its clients keep;
    `probe` runs a queue of one."

**CONVENTIONS.md**

- **The `internal/model` bullet (phase 3, step A).**
  - "go through `model.Chat`" becomes "go through a `Slot`'s `Chat`".
  - "go through `model.Decision`" becomes "go through a `Slot`'s `Decision`".
  - `Endpoint{Name, BaseURL, APIKey, Protocol, Decisions, PromptCacheKey}` becomes
    `Endpoint{Name, BaseURL, APIKey, Protocol, Decisions, PromptCacheKey, IdleConns}`.
- **New house rule (phase 3, step B).** In "## House Rules", directly after the bullet beginning
  "**No stage exits on a model's opinion.**" (`grep -n "No stage exits on a model" CONVENTIONS.md`
  answers line 230), add: "**A model call is made through a `Slot` from the invocation's queue,
  inside the item `Go` launches; the item writes files only in its `after`, once its token is
  released.** The trap: a goroutine pool or semaphore beside the queue, which re-introduces a second
  bound; a `WithTimeout` around `Go`, which times an item out for its wait; groups launched from
  per-group goroutines, which loses submission order; or a common file — a ledger, a record, a
  progress line — written from an item while its ask batch is in flight (ARCHITECTURE I11)."

**README.md and USER_CONVENTIONS.md (phase 2)**

These are user-facing docs, and four lines in them state behaviour that phase 2 changes.
`grep -n -i 'in flight\|each landing' README.md USER_CONVENTIONS.md` answers four lines, and
`USER_CONVENTIONS.md:99` ("the stage in flight") stays as it is. The four that change:

- `README.md:217`: replace "stop the build; costs only the unit in flight" with "stop the build;
  costs only the calls in flight".
- `README.md:336`: replace "`kbase score` takes it around each landing and each of its two
  refreshes" with "`kbase score` takes it around its one landing and each of its two refreshes".
- `USER_CONVENTIONS.md:101`: replace "losing only the unit in flight" with "losing only the calls in
  flight".
- `README.md:227-228`: replace "and keeps the answer cache" with "and keeps the answer cache of the
  current build trail and the five before it". The phrase breaks after "keeps": `grep -n 'and
  keeps$' README.md` answers line 227.

**ROADMAP.md (phase 3, step A)**

- Add a line beside the `provider` item: "Remove the inert `readerConcurrency` key from
  `config.toml`'s `[asks]` at the next major."
- Delete the two "Next" items this work finishes: the one beginning "Use goroutines to parallelize
  scoring asks" and the one beginning "Same for most of the build asks". ROADMAP.md's header states
  the rule: "Finished items are deleted".

---

## 8. Mechanisms reused

- **No bounded-concurrency helper to reuse.**
  `grep -rn --include='*.go' -E 'sync\.WaitGroup|make\(chan struct\{\}, |errgroup|semaphore' . | grep -v _test.go`
  prints four lines, two at each of two sites:
  - `internal/asks/letters.go:251-252`, `AskGroup`'s semaphore and its `WaitGroup`. Step B removes
    the semaphore.
  - `internal/sheet/sheet.go:48-49`, whose `slots := make(chan struct{}, renderConcurrency)` bounds
    `dot` processes. It stays.
- **Standard library only.** `grep -n 'golang.org/x/sync' go.mod go.sum` prints nothing, and the
  module is not sanctioned (CONVENTIONS "Dependency Policy").
- **Per-result writes go through `atomicfile.Write`** (`internal/atomicfile/atomicfile.go:37`), the
  one atomic writer (ARCHITECTURE §11).
