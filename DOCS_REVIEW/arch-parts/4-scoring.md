### Scoring

**Pattern:** Job runner with an append-only event log, resume from the last status per item, optimistic-concurrency commit, projection rebuild.

`kbase score` is defined in SPEC §8 `score`; this section states the mechanism. The packages that run it:

- `internal/score`: the run's entry, scope and stale check, the runner, the item text and its sections,
  the parse, the run's ledger and captures, run retention, the writing of results, the run's status
  reader and the development replay.
- `internal/kb`: the fault vocabulary (`kb.FaultTable`), the three weight tables with their value
  computation (`Price`), and the statement reader `kb.Statement`.
- `internal/asks`: the three templates, the check that each names its class's ids, and the judged hash
  (`asks.JudgedHash`).
- `internal/query`: the scoring set.
- `internal/write`: the edit ops a score run applies, the KB write lock, and the hash every value set
  records.
- `internal/statedir`: the state directory.
- `cmd/score.go`: the verb.

#### The runner

**Pattern:** Bounded worker pool over a job list, results written in one batch at the end.

`score.Runner` owns the item loop and no I/O of its own. It takes the enumerated items; the `Launch`
interface, which hands each item the `Ask` its one model call goes through; the `Write` interface; the
ledger; the model name the ledger lines record; and a logger. Because the item text is composed in this
package, it also takes the `kb.Source` and `query.Index` the renderers read, and the `graph` they
read, built once from them (`graphOf`); `Invoke` passes in the `graph` it built for the scope.

The pass is read once, before the loop, into its classes. Per item, one lookup of the item's class
yields both the instruction its call sends and the table its value is computed under, so the
composer, the parse's fixed set and the value computation all read that one fact. The launch loop
composes each item's prompt and launches the items in the set's order, at most as many at once as the
invocation's `model.Queue` has slots. Each launched item makes its call, parses the reply as the charges
it records against that item text, computes the value, keeps it in its own result cell and writes its
result file.
A text the input guard refuses, a reply the parse refuses, and a failed call or write each append
`failed` with nothing written, and the loop continues.

`prepare` returns the entry and the `call` the item will make: class, item text and prompt, or nil where
the class cannot be derived or the input guard refuses the text. `attempt` returns the entry and whether
the loop's stop interrupted the call. Nothing shared is written until the pool has drained (invariant
I11, §2); the ledger's `flush` then writes the cells' values, appends their captures,
then appends their lines, in the set's order. No reason for writing at the loop's end rather than per
item is written beyond that invariant. `Counts` comes from the ledger's last status per item, not from this
invocation's attempts, so a resumed run reports the whole run. A cancelled context stops the loop and
records nothing for an item without an answer; an interruption outranks a failure in the run's outcome.

#### The ask is one seam

**Pattern:** Dependency injection.

`Ask` takes the rendered prompt whole and returns a `score.Reply`: the reply's text and whether the
endpoint truncated it at the completion cap. The runner has no view of a provider. `cmd/importpolicy_test.go`
enforces that `internal/score` never imports `model` or `build`: `internal/score` imports `argmark`,
`asks`, `atomicfile`, `buildstore`, `kb`, `log`, `query`, `result` and `write`. A truncated reply is the
runner's to judge, not the `Ask`'s: `put` fails its item as it fails a call that returned an error,
whatever the text would parse to.

`scoringLaunch` in `cmd/score.go` fills `Launch`. It builds the invocation's one `model.Queue` and
returns a `Launch` that starts each item on a slot of it. The item's `Ask` makes one `Slot.Chat` call on
the scoring role's endpoint with an empty system prompt, applies `--timeout` from the call's start
(never over the wait for a slot), and reports `model.FinishLength` as the truncation. The `modelclient`
registry opens the client at the first call, so a run refused at its scope or holding no item connects
to nothing. A provider's refusal of the call's key is a `modelclient.KeyRefusedError`; `scoringLaunch`
turns it into a `score.RemedyError`, which the failed attempt's ledger line records. Retry and backoff
are the chat client's (§7); `scoringLaunch` supplies the pauses (`scoreBackoff`),
and the attempt count and pauses are in §12. The development replay takes its `Launch`
from the same function, so a replayed call carries the run's endpoint, timeout and truncation rule.

Each score template splices the `score-system` fragment into its own text, so the instruction is carried
once, in the user turn.

#### The prompt shelf

**Novel.** The fault criteria the templates carry realize THESIS item 2 (local judgments from a closed
list of faults), defined by the rigor and bearing scoring scheme and SPEC §8 The answer.

**Pattern:** Prompt templates with a lint test.

Three templates, one per class: `score-rigor-statement.tmpl.md`, `score-rigor-method.tmpl.md` and
`score-bearing.tmpl.md`. Each has one `dyn.` slot, which the runner fills with the assembled item text,
and splices four shared fragments (`fragmentSlots`): `fragments/score-system.tmpl.md`, the system
prompt; `score-disposition`, the class-neutral standard every score template judges by, which names
no id; and `score-answer-occurrences` and `score-answer-close`, the answer section's text, which all
three state alike so no vocabulary can drift from the parse or from its siblings. `asks.ScoreRigorStatement`,
`asks.ScoreRigorMethod` and `asks.ScoreBearing` render them through `asks.Render`; `asks.System`
serves the system fragments alone (`systemSlots`), never an answer fragment as a prompt. The templates
are embedded in the binary, written by kbase, with no upstream origin and no version pin.

The templates carry the criteria and the line form. They name no id the class does not accept, and no
figure or word a model could read as a judgement: no decimal in `[0,1]`, no percentage, no weight, no
maximum, no band's word. This is a deliberate blinding: the weights are `internal/kb`'s, the single
source of truth for them. A quantity of the subject's own, an example's rate or threshold,
is the claim's and stays.

Tests in `internal/asks` assert that each template declares one `dyn.` slot, fills it with the text the
runner composed, splices the shared fragments, names exactly its class's ids (`flt-[a-z0-9-]+` tokens
against `kb.RigorStatement.IDs()`, `kb.RigorMethod.IDs()` or `kb.BearingFaults.IDs()`), carries none of
those judgement forms, and keeps the instruction fixed around a subject of any size. `asks.Lint`
checks every file on the shelf for a delimiter that does not parse as a slot, and for any phrase its
caller prohibits; the tests prohibit none.

#### The closed text and its guard

**Pattern:** Line-numbered context for grounded citation; delimiter-based prompt-injection guard.

`internal/score/text.go`'s two unexported renderers assemble every character a call may see.

`rigorText` renders a node's title and its statement. The title section is read through the node's
canonical path (`src.KBPath(row.CanonicalPath)`). The statement is `kb.Statement`'s reading of the node
in the leaf hosting it, the same reader `add-citation`'s quote check uses. The run's `graph` contains it
for every hosted node, read once per leaf by `graphOf` with the `lifts` edges of the in-memory index,
markers and `>` prefixes removed, with the 1-based line of the leaf it opens on (`statementLine`: the
line nearest the node's marker whose text, markers and `>` prefixes removed, begins with the statement's
first line). A node no leaf hosts (a work, an axiom, a node with no marker) renders its title alone.
`bearingText` renders the source's and the target's sections by that same rule, then the edge's authored
`context` from the row `Index.Edge(ScoringEdge)` answers. Each end's label opens a line of its own, as
does the context's, so no numbered line begins with template-wrapper text.

`assemble` wraps every section of the item text at word boundaries to `wrapWidth` (§12),
keeping the section's own line breaks and rewriting nothing, and numbers the lines lying in a section
that a charge may cite, from 1 over the whole text. `closedText.numbered` writes `N| ` before a numbered
line; the gutter is the template wrapper's, not part of the text the call sends. A label line, a blank
separator line and the edge's context are written bare: a line with no number is no site.

**Pattern:** Delimiter-based prompt-injection guard. `scoreTokenRE` and `frameRE` scan each assembled
section; no guard pattern spans the blank line between two. A hit is `scoreTokenError`, naming the
document and line, and the item is `failed` with no call made (SPEC §8 One item, one quantity, one call).
The input guard refuses and never strips or escapes, because a rendered prompt stays a function of its
template. `scoreTokenRE` matches the four scoring tokens. `frameRE` matches the template wrapper's own
structure: a run of twelve backticks closes the fence the templates draw around the item text, after
which the text sits outside the instruction to judge it; a line opening `## Source:`, `## Target:` or
`## Context` forges the wrapper's map of the text. A label that a blockquote or an indent displaces from
the line's start opens nothing. An out-of-scope subject is refused by name and type: a `wrk-` or `axm-`
id on `rigorText`, a `references` or `demoted` relation on `bearingText`.

#### The parse resolves the numbered lines

**Pattern:** Structured-output validation against a fixed grammar.

`assemble` returns the item text and one `line` per line of it: the number the call writes, the section
its bytes lie in, and its byte range. A charge's `passage`, `lines` and `offset` are facts about that
rendering. A `rigor` call's two sections are a node's `title` and `statement`; a `bearing` call's are the
`source`'s and the `target`'s. `score.ParseAnswer` (`internal/score/answer.go`) reads a reply against
those lines and the item's class's `kb.FaultTable`. A charge line's field is one number or a range
`N-M`, which `closedText.look` resolves; a range that crosses sections, a blank line, a label or a
heading is `malformed`. The recorded `citation` is the cited lines as the call sent them, without the
gutter, joined by newlines, and `offset` is that quote's byte offset in the item text. The edge's
`## Context` has no number and is a site for no charge, so the `passage` value `both` is never produced.
Grammar, duplicates and overlaps: SPEC §8 The answer.

The three weight tables are `internal/kb`'s: `kb.RigorStatement`, `kb.RigorMethod` and
`kb.BearingFaults`, each a `kb.FaultTable` of ids and per-occurrence weights in integer hundredths, with
`IDs`, `Accepts` and `Price`. The parse yields `Answer{Faults, Moot, Value}`: the charges, `Moot` (charge
lines recorded beside a `flt-pending`, which no table holds; `ParseAnswer` produces none, and a recorded
line may carry them), and the `Price` of the ids. `Price` of a recorded line carrying `flt-pending` is
`*pending*`. The value is defined in SPEC §8 The value. The ids a line records are the whole input of the
value, which makes a written value checkable and a run recomputable without a model.

The grammar is fixed for as long as the ledger's format version is unchanged: a rule loosened later
would re-read lines already written under a new grammar (SPEC §8 The answer).

#### The scoring set

**Pattern:** Input-hash staleness, as in make.

`query.ScoringSet(ix, pass)` reads the in-memory index in-package and returns `[]query.Item`, each
holding a `ScoringNode` or a `ScoringEdge` and never both, in the loop's order. `query.PassRigor`,
`query.PassBearing` and `query.PassAll` are the three values the set and the `--pass` flag share
(`query.Passes`); `PassAll` is the node set and then the edge set, each in its own order. An item
contains identity only, so an edge's row comes from `Index.Edge` and no caller builds a `kb.EdgeRow` by
hand. The set has no verb: it is consumed in-process, and `--dry-run` is the inspection surface.
Membership: SPEC §8 The two passes.

`internal/score/scope.go` narrows the set to a run's scope. `scopeOf` returns the one named item, or
else each item of the pass that is stale or, under `--rescore-manual`, manually scored. `namedNode` and
`namedEdge` refuse an id or an edge that does not exist or that no pass asks; `namedItem` refuses a
manually scored item without `--rescore-manual`. `dirty` compares the hash each record carries
(`recordedHashes`, read once through `kb.LoadGraph`) with `judgedHash`, the hash of what the item would
be judged against now: `asks.NodeJudgedHash` over a node's statement, `asks.EdgeJudgedHash` over its two
ends' statements, each the one the run's `graph` read through `kb.Statement`. The judged hash is
defined in SPEC §10 Provenance and the judged hash. `asks.JudgedHash` hashes the digest of the
template the item is judged under, then each statement behind its length; the model is no part of it.
`RigorAsk` is the single mapping from node kind to template, which the hash and the call both read.

A resume takes no scope: its set is the run's `items.jsonl` (`readItems`). `ResumeTarget` picks the run
a `--resume` continues, the newest `run-` directory by its header's start (`newestRun`, filtered to
`RunKindReal`, so a `dry-` run is never one). It refuses where none exists, where `ReadRun` counts the
run finished, and where a build invocation in the store's progress record (`buildstore.ReadState`)
started after it.

**Pattern:** Filtered work set. `Runner.Skip` is `score.ManualSkip(ix, rescore)`, built from the run's
`--rescore-manual` and each item's `manually-scored` flag (`query.Index.ManuallyScored`). It says which
items the loop records `skipped` (status `score.StatusSkipped`, reason `manually-scored`; no call, no
capture). `score.Resume` takes the same predicate, so `Ledger.reprice` leaves a skipped item's written
line as it is. The loop also records `skipped`, once, an item that is not in the in-memory index it
loaded (`inKB`), and `attempt.fail` does the same for a write's `errItemGone`, each with that error's text as
the reason (SPEC §8 Skipped items).

#### Bearing predicates

**Novel.** The two predicates separate which edges carry a bearing from which a score run asks about,
which realizes THESIS item 1 (the four scoring relations) as defined in SPEC §10 Four scoring relations,
one arithmetic.

**Pattern:** Filtered work set.

Two predicates in `internal/kb` answer whether an edge carries a bearing, and they are not the same
question. `kb.Scoring`, over the four relations in `kb.ScoringRelations`, is whether an edge carries a
bearing at all. The readers of a stored bearing use it: the write package's relation checks, the
derived views and strength propagation in `internal/index`, `internal/query`'s trace and payloads, and
`bearingText`. `kb.BearingWalk`, over the three in `kb.BearingWalkRelations`, is whether a score run
asks for that bearing. Its readers are `query.ScoringSet`'s bearing pass, `namedEdge`, the
`manually-scored` flag on edge rows (`internal/index/derive.go`) and its description in the write
package, and `stats`' `walked` key. A `founds` edge carries a bearing and is never an item, its source
lying outside the corpus, so `edit-edge` alone writes it.

#### The ledger and the state directory

**Pattern:** Append-only job log with resume.

`score.Ledger` opens `<state-dir>/score/<run>/`, writes `run.yaml` through `atomicfile.Write`, and
appends the loop's lines in one write, with `O_APPEND` and `Sync`, when the loop ends. It reads back
the last entry per item, with its status and the evidence behind it, so `--resume` skips what was written
and a failed run names each failed item's reason. A line is appended after the item's write and
validation, never before: after a crash between the two the item gets a new model call and an
idempotent write, where the reverse order would skip a value that was never written. `Create` never reuses a run directory: the run id is minted
past any that exists, and a directory that appears under it fails the call.

`Resume` refuses a `run.yaml` naming another KB root or ledger format. It takes the run's pass from the
header. It drops a tail torn after its last newline, so the item the line names gets a new model call, then
merges the result files a stopped invocation left (`fold`): each value is written through the run's
`Write` interface, recorded, and the file removed; one whose attempt the ledger already records is
removed alone. Then it recomputes values ([the re-price](#the-re-price)).

`items.jsonl` holds the set the run enumerated, one pending line per item, written when the run opens
its ledger, so status and a waiter count the whole run rather than the attempts made so far; it is all a
dry run writes beside `run.yaml`. The files, the line's keys and the optional `remedy`, `note` and
`judged` under one ledger format are SPEC §8 The ledger and resume. Two mechanisms behind them:

- A line contains `faults` (the parsed charges, in the order written), `moot` and `before`, the value
  its item had in the in-memory index the run read before the attempt wrote. `faults` is `[]` for the
  reply `none` alone, so an empty list always holds the model's positive statement that it found no
  fault. A failed attempt contains neither charge list and no reply text: its `reply` is the reason.
  `remedy` is the one the call's failure named, and `note` goes with it; a failure item appends the note
  after the bounded reason, so the bound never cuts it. Reply, remedy and note pass through
  `argmark.Spelling` ([§9](#per-interface-rendering)), so no line carries a marker.
- `backend` and `pass` sit on every line, because a resumed or mixed run's lines must say what answered
  each item and which class each judgement came from. `Create` records `ask_sha256` from the header's
  `Pass` (`askDigest`), as it records the format: one hash over every instruction the run can send, each
  rendered for an empty subject, concatenated in the order statement, method, bearing. It is recorded at
  one site, so no caller can open a run without the prompts its judgements answer, and a render that fails
  is a defect of the build, not a run written with no hash. `Resume` reads the header's hash and compares
  nothing against it (SPEC §8 `ask_sha256` records the ask set and gates nothing).

The state directory is `internal/statedir.Dir`'s: the override, else `$XDG_STATE_HOME/kbase/<key>`, and
never inside `kb-root/`. It is shared with the build so a run and a build agree on the state directory
for a KB; the build store's own format record is `internal/buildstore`'s (§8).
The run takes the run lock through `build.Hold` ([§8](#locks)); the lock's record names `kind: score` and
the run id.

#### The captures

**Pattern:** Request/response capture for replay.

The loop's end appends every capture to `captures.jsonl` in one write, beside the ledger, through
`appendLines`, with the `O_APPEND` and `Sync` the ledger's own lines take. A capture is written for each
attempt that returns a `call`. `attempt` returns none where the class cannot be derived or the input
guard refuses the text, and a call the context cancelled returns nothing, so none of the three leaves a
capture. Captures are written before the lines, and a capture that cannot be written ends the run
`failed`, as a ledger line that cannot be written does.

A capture's keys, in order: `run`, `pass`, the item's identity as the ledger line formats it (`identity`:
`kind` and `id`, or `source`, `target` and `relation`), `class` (`statement`, `method` or `bearing`),
`text` and `lines` (the item text whole, each line `{no, end, from, to}` as `closedText` holds it),
`prompt` (the bytes sent), `prompt_sha256`, `reply` (what the `Ask` returned; `""` where the call
errored or the reply was truncated), `error` (the call's, the parse's or the write's error text, absent
on a successful write), `model`, `backend`, `started` and `ended`. The ledger's failed line carries the
reason and no reply text; the capture is where the reply is kept (SPEC §8 The ledger and resume).

`Resume` removes a torn last line from `captures.jsonl`, as from `ledger.jsonl`, through `trimTorn`, the
single torn-tail trim, before anything appends, so a crash mid-capture never joins two captures into one malformed line.

A capture is self-sufficient for a replay. `readLastCaptures` (in the replay's tagged `replay.go`; the
raw read is `readComplete` in the untagged `ledger.go`) returns, per item, its last capture with the item
text rebuilt. It reads the file to its last complete line, so a run still appending is readable, and
refuses a line that names no class or whose line ranges fall outside its text. **The fidelity rule:**
`class.render` is the single site that renders an item text into a prompt, shared by `compose` and the
replay, so rendering a capture's item text through an unchanged template reproduces its `prompt_sha256`
byte for byte. `TestCapturesReRenderToTheirDigest` asserts it over captures the runner wrote.

#### Run retention

**Pattern:** Keep-last-N retention.

`KeptRuns` (§12) runs of each kind remain under `score/`. A real run's id is minted
`run-<id>` and a dry run's `dry-<id>` (`kb.MintID`'s prefix, `RunKindReal` and `RunKindDry`). A
directory's kind is its name's prefix (`runKind`), so the two kinds are counted apart.
`score.PruneRuns(dir, run)` keeps the run the invocation names, created or resumed, and the newest
`KeptRuns-1` other directories of its kind. It orders them by the `started` each `run.yaml` records,
equal starts by name with the later name newer, and removes the rest of that kind whole with
`os.RemoveAll`, captures and replays with them, through `buildstore.PruneByStart`. It also keeps the
newest `KeptRuns` score-run captures under the state store's `reports/` (`buildstore.PruneCaptures`,
prefix `score-`). `buildstore.DirStarts` is the single per-directory read of that start and
`buildstore.CompareStarts` the single ordering, both shared with `NewestRun`, so `status`'s newest run
and the prune's agree. A directory whose `run.yaml` is absent or unreadable yields no start and is kept.

`score.Invoke` calls the prune once the run's ledger has opened (after `Create` on the dry path, after
`Create` or `Resume` on the real one), so the run it keeps exists, and an invocation refused before then
removes nothing. It runs under the run lock, and a prune error ends the invocation `failed`. The one
writer under `score/` the run lock does not exclude is a replay, which takes no lock: a prune may remove
the run a replay is reading, and the replay then fails rather than writing into a removed directory
([the development replay](#the-development-replay)). `ResumeTarget` filters runs by kind, so a `dry-`
run is never a resume target; `status`, `wait` and `RunByPID` find a run by its directory or pid,
whatever the prefix.

#### Writing the results

**Pattern:** Optimistic concurrency with partial-batch retry.

`internal/score`'s `landValues` writes a run's answered values in one write batch, under one hold of the
KB write lock taken through `write.LockKB` ([§8](#locks); wait: `write.LockWait`, §12).
It is the single path to the KB: the loop's end, a resume's merge of stopped results and a resume's
recomputation all reach it through the `Write` interface (SPEC §8 Writing the results).

Under the lock and before any batch, `landValues` opens the KB afresh through the interface the verb
supplies (`Invocation.Open`, `kbload.Open` in `cmd/score.go`) and reads it through `query.Load` and
`graphOf` (`graphAt`). Each value is then checked against the KB it finds:

- A KB that does not open or load fails every value with its error.
- A value whose node or edge is not in the KB fails with `errItemGone`, which `attempt.fail` records as
  status `skipped`, not `failed`, so `ReadRun` counts it skipped and `ResumeTarget` does not resume
  for it. A resume's loop records an item of its run that is not in the in-memory index it loaded the
  same way, once (`Runner.Run`, `inKB`), and makes no call for it.
- A value whose `Landing.Judged` (the judged hash `prepare` recorded as its call was composed, carried
  on the result file and the ledger line as `judged`) differs from `judgedHash` over the text the KB now
  holds fails with `errTextChanged`: a statement edited by hand while no lock kept the edit out. A
  resume sends it again.

Only the remaining values go into the batch, so the hash the edit op records is always the hash of the
text the value judged. They form one mixed write batch (`write.Batch`) through `write.Run`, an entry per
value in the order given. `opOf` names the edit op: `edit-claim`, `edit-analysis` or `edit-experiment` by
the node's kind with `id` and `rigor`, or `edit-edge` with `source`, `target`, `relation` and `bearing`.
The batch is applied under `Provenance: write.ProvenanceMachine`, so a written value has
`manually-scored: false`, and it rebuilds the derived views inside the batch, so the records and the
derived views are written together. A `done` or `unchanged` batch has written its values.

A refused batch writes nothing (SPEC §8 Write batches). Where the refusal names its entries, those items
fail and the batch is retried without them, until it is written or empty. Where it names none, the
batch's values are retried one at a time under the same hold, each refusal then failing its own item;
this covers a refusal the reopen did not foresee, a hand edit between the reopen and the batch, or a
check the write makes that a load does not. A failed value's error is its details joined and formatted
for the command line (`valueError`), so no ledger line carries a marker. A batch that fails, as against
being refused, fails every item it contained. A lock that cannot be taken fails every item too: one held
past the wait fails each with the detail of `write.HeldLockItem`, the item a verb answers `retry` with,
and any other lock failure fails each with the lock's own error. The run continues. `landValues` returns
one error per value, nil where it was written. It checks for no run lock: it writes as the run holding
that lock, so no run can be holding the KB out.

Each value the batch sets records its judged hash in `internal/write` (`judged.go`). An entry that sets
a rigor or a bearing names the record through `valueSet`: a caller's value, `*pending*` included, or a
run's number. A run's `*pending*`, which the build writes for `score` to judge, is no judgement and names
none. Once the entry has applied, `hashValuesSet` gives each named record `asks.NodeJudgedHash` or
`asks.EdgeJudgedHash` over the statements the batch leaves, read through `kb.Statement` from the host
leaf, so a node an add marks in the same entry is hashed over the statement its marker gives it.

#### The re-price

**Pattern:** Projection rebuild from stored evidence.

`Resume` takes the same `Write` interface a run's writing is opened with, and `reprice` recomputes every
written line's value from the ids its line records, under the table the line's own class holds. A run
written under one rigor table is recomputed under the class table. Only a line that records `judged`
and its charges is recomputed; a line without them says nothing of what text its value judged and is left
as it is, as is a line for an item `skip` names.

A value the running table gives differently is written first through the batch above, under the KB write
lock, and only then does the line record it, so no line claims a value the KB did not take. A value the
write discards (`errTextChanged` or `errItemGone`) rewrites its line with that reason through
`attempt.fail`, as a loop's discarded write does: `failed` for a changed item, which the loop that
follows sends again, and `skipped` for a removed one, which nothing sends again. Any other refused write
fails the resume with the ledger as it stood; the next resume recomputes the same change, the write being
idempotent. Each changed line is listed in `repriced.yaml` in the run's directory, written only where a
value was written: the run, the time, and per line the item, the value it held and the value it took.
`run.yaml` is written once, so `repriced.yaml` is where a recomputation is recorded. `Ledger.Repriced()`
reports how many values changed. No model is asked: the ids are the whole input of the value, and the
model never saw a weight. Behaviour: SPEC §8 `ask_sha256` records the ask set and gates nothing.

#### The run's entry

**Pattern:** Command orchestration.

`score.Invoke` runs one invocation over a KB its caller has opened, rebuilt and locked. It runs
`query.Load`, builds the run's `graph`, and determines the set: the scope (the scoring set), or a resume's
own `items.jsonl` under the pass and `rescore_manual` its `run.yaml` records. Then either the dry run's
records and the listing of what it would ask and skip (`Report.Ask`, `Report.Skip`), or the loop over the
`Launch` the verb built (`Invocation.Launch`): the ledger created or resumed, the prune, the pid,
`items.jsonl`, and the loop, whose values are written by the run's one batch when the loop ends.

It returns a `score.Report`:

- the outcome, and `Asked`: whether the invocation reached the route its calls take (not a dry run, so
  `Launch` was handed to the loop);
- the run and its ledger directory once the ledger exists with the pid and `items.jsonl` beside it, and
  on a stop only where the loop stopped it;
- the counts and the items scored;
- on `failed`, one `check: error` item per item the ledger records failed, its detail the item's name
  and its line's reason; on `cancelled`, the loop's first item with no attempt, `{}` where every one has
  one;
- `Unfinished`: whether an item failed or was not reached.

`score.RunID` creates the runs directory and mints the id (`run-<id>`, or `dry-<id>` under `--dry-run`)
or takes the one `score.ResumeTarget` found, before the locks, because the run lock's record names the
run.

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
5. `score.RunID`, then the run's locks through `build.Hold` ([§8](#locks)), before any record of the run
   is written.
6. One rebuild of the derived views through `holdWriting`, skipped under `--dry-run`, which releases the
   KB write lock before the loop.
7. The `Launch`, `scoringLaunch`, then `score.Invoke` given that `Launch`.

The result document is assembled in this order: `roles` where the report's `Asked` says the loop was
handed the `Launch`, then `run`, `ledger`, `counts`, a dry run's `ask` and `skip`, and `scored`;
`resume` (`scoreResume`) where an item failed or was not reached, ahead of the final record; and last
the report's `failures` on `failed`, or its `cancelled` on `cancelled`. The batch that writes the
results rebuilds the derived views, so no rebuild follows the loop. `persistScoreResult` writes the
document beside the ledger (`score.WriteResultDocument`) before the locks are released, so `wait` reads
the document the verb emitted. The records and `resume`'s reach: SPEC §8 Outcomes and The closing record.

Registration is `readWriteTool(scoreCmd)` in `cmd/surface.go`: the command is exposed as an MCP tool,
and the server starts it as a detached child (`mcp.ScoreLaunch`), as it starts `build`, returning the
run's identity and a `wait` command instead of holding the turn for the loop's duration. The tool
table and the detached child are [§12](#mcp-server-spec-11); the start wait is `launchStarting`
(§12). `--edge` is declared a mapping flag (`mcp.MappingFlag`): over MCP an object of
`source`, `target` and `relation`, joined into the `--edge` the child takes.

#### The development replay

**Pattern:** Offline prompt-regression harness.

`kbase score --replay RUN` repeats a run's captured model calls under the binary's embedded templates
and reports what the runtime would have done, so a change to a scoring template is `edit → just build →
replay` over the items that failed. SPEC states nothing of it. Its code and tests are
`cmd/score_replay.go` and `internal/score/replay.go` with their test files, each carrying
`//go:build kbasedev`. `cmd/score.go` reaches it only through the package-level `scoreReplay` hook, which
the tagged file's `init` sets and which is nil in the default build. That `init` also declares `--replay`,
`--failed` and `--limit` (a run takes no limit, so `--limit` is the replay's alone), each annotated
`kbase.mcp` = `excluded` through `mcp.ExcludeFlag` ([§12](#mcp-server-spec-11)), so dev and release builds
serve one tool table.

The default build is the release shape and excludes the replay's code. Dev builds pass `-tags kbasedev`,
as `just build`, `just test` and `just test-race` do. What the replay reads is not dev-only: a run's
ledger and captures are written by the untagged `internal/score/ledger.go`, so a replay in a dev build
reads the records a released kbase wrote. `edit-gate` vets both build tags, and `test-dist-no-replay`
runs `TestDistBuildHasNoReplay` (`cmd/dist_test.go`) on the untagged build, which asserts the three flags
absent and the hook nil, and the same of the build's development replay (§8).

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
per `ReplayProgressInterval` (§12) and once more when the loop ends. The counter and the
one goroutine that logs it are `buildstore.ReplayProgress`, which both development replays use. An item
only increments the counter as its call returns (an item whose status is `done` counts done, any other
answered item failed, an interrupted one neither), and no item and no progress line writes a file
(I11, §2).

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
