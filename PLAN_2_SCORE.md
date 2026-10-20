# Phase 2 — `kbase score`

Status: blocked on Phase 1, whose WP1–WP5 are committed checkpoints and whose node-pass rework is
reissued in [`PLAN_1_FORMAT.md`](PLAN_1_FORMAT.md). This phase starts only after that landing is
merged. Landing: one commit series, merged once.
Gates: `just test`, `just edit-gate`, `just test-integration`, `just test-integration-mcp`.

## Preconditions

Phase 1 is merged: five node types (`claim`, `analysis`, `experiment`, `work`, `axiom`),
`rigor`/`bearing`/`strength`, the four relations, `set-rigor` and `set-bearing`, `deps` with
`direction`, and `query.Load`'s refusal of an index it cannot answer all exist. Re-establish each
before editing:

- `rg -n 'NodeKindSupport|support-nodes|SupResults' internal/kb internal/index internal/query internal/write internal/sheet` → no output; the node pass is landed, and a `score` built against `support`, `sup-` or `SupResults` is built against the wrong tree. `internal/migrate` is **not** scoped out: its `"sup"` and `"support-nodes"` literals (`from_1_0_0.go:18`, `:112`, `:150`) are the converter's read of `1.0.0` bytes and stay.
- `grep -n 'func Run' internal/write/ops.go` → `internal/write/ops.go:209`; the in-process write seam
  is `write.Run(op, Options)`, not a shell-out to `./bin/kbase`.
- `grep -n 'CategoryText' internal/config/config.go` → `internal/config/config.go:22`; the scoring
  model is the resolved `Inference.Text`.
- `grep -n 'func Chat' internal/model/chat.go` → one tool-less chat call with retries and an
  observation hook.
- `grep -n 'func Render\|func System' internal/asks/compose.go` → `:103`, `:214`.
- `grep -n 'func Acquire' internal/filelock/filelock.go` → `:23`.
- `grep -n 'func stateDir\|func stateKey' internal/build/store.go` → the state-store resolution this
  phase reuses.

## What the command is

```
kbase score --pass rigor|bearing [--provider NAME] [--limit N] [--dry-run]
            [--resume RUN] [--state-dir DIR] [--timeout DURATION]
```

It walks the KB's scoring set, asks one question per item, validates one value, lands it through
`set-rigor` or `set-bearing`, and records the run in a ledger. It is a CLI verb and is excluded from
MCP.

Result keys after `kb-root`, in order: `migration` (conditional: present only on a call whose load
converted an older KB), `state-dir`, `pass`, `provider`, `provider-rule`, `provider-layer`, `model`,
`run`, `ledger`, `counts`, `written`, `resume`. The four provider keys are absent together under
`--dry-run`. `counts` is `{total, done, malformed, failed, remaining}`. `resume` is present whenever
an item is unattempted or not `done` — the ones `--limit` left, and any `malformed` or `failed` item
— and is absent from a dry run.

Outcomes: `done` when the run finished, a dry run included (its `counts.done` is `0` and it carries
no `resume`: it asked nothing and landed nothing); `bounded` when `--limit` stopped it; `cancelled`
on interruption, with `resume`; `failed` when any item's call or write failed; `refused` on setup
(unknown pass, no provider, an index the load refuses — after the start refresh on a real run, as it
stands under `--dry-run`); `retry` when another score run holds the lock. An older **KB** is not a
refusal: a run is a natural write and so migrates (invariant 9) — the start refresh saves the
conversion, does the converter's cleanup and stamps `2.0.0`, and the run's document carries the
`migration` record. A *stale* index is not a
refusal either: the start refresh repairs it, then the run loads. Only an index the load still
refuses stops it, and under `--dry-run` no refresh runs, so an older KB's index is refused there
with remedy `kbase refresh`.

## Runner shape — the invariants

1. **Provider mode only.** `cmd/score.go` uses `providerOptions.dial` and `model.Chat` with the
   resolved text model. No harness, no session, no tool allowlist, no per-item agent, no Decision
   call. Concurrency is off: one item at a time, in a stable order. The reason is the ledger and the
   write path — both are simpler and auditable sequentially, and a run is minutes, not hours.
2. **One item per call, one quantity per answer.** A call carries one node or one edge. The model
   never returns a pair, and kbase never asks for one. Composition is the walk's.
3. **The scoring set comes from the index.**
   - `rigor`: every claim, analysis and experiment node, by id.
   - `bearing`: every `supports`, `founds`, `lifts` and `refutes` edge, ordered by relation
     (`supports`, `founds`, `lifts`, `refutes`), then source, then target.
   - Excluded by construction: works and axioms on the rigor pass — a work's rigor is assigned by
     hand and an axiom's is `1.0` by rule; `references`/`demoted` on the bearing pass.
   The set is enumerated once at run start. It is structural; landing a value does not change it.
4. **Closed, score-free text.** Two unexported renderers assemble the only text a call may see:
   - `rigorText(node)`: the node's title and its statement. A claim's statement is the paragraph or
     block its tier-2 marker sits in, in the leaf that declares it. An analysis's and an experiment's
     is the section at its canonical anchor in its leaf. A claim with no declaring leaf renders its
     title alone. **Never the register's section** — a claim's `CanonicalPath` names the register,
     whose `### Rigor` section carries the authored score and whose `### Strength` block the derived
     one; reading it would either leak them or trip the guard on every claim. Never the rationale.
   - `bearingText(edge)`: the source's and target's statements, the edge's authored `[context]`, and
     the target's derivation passage — the same statement text `rigorText` renders for the target, so
     the judge sees the passage the bearing is about. No premise list, no annotation.
   After assembly, a guard refuses the text if it matches `- rigor:`, `(bearing`, `- strength:` or
   `(strength `, naming the path and line it came from. The guard is cheap and always runs; the call
   is never made on text it rejects, and an item it rejects is `failed` in the ledger with the
   guard's `path:line` as its error — no call, no write, the run continues.
   The renderers refuse out of scope by name: `rigorText` a `wrk-` or `axm-` id; `bearingText`
   `references` and `demoted`.
5. **Validation.** One line: a decimal in `[0,1]` or the pending literal. Anything else is
   `malformed`: no write, the ledger records the reply, the run continues. A transport failure is
   `failed`: no write, the ledger records the error, the run continues.
6. **Landing is the write API's.** A rigor answer becomes `set-rigor {id, rigor}`; a bearing answer
   becomes `set-bearing {id, target, relation, bearing}` — `id` is the source (`write/values.go`'s
   `set-bearing` table). kbase never edits a register or a leaf itself. Every op is called with
   `NoRefresh: true`. A write refusal (a changed KB, a missing token) is recorded as `failed` for
   that item.
7. **The ledger is a run record, not a cache.** Per run: `<state-dir>/score/<run-id>/run.yaml` and
   `ledger.jsonl`. `run.yaml` carries `run`, `kb_root`, `format`, `pass`, `provider`, `model`,
   `started`. One JSON object per item attempt, appended after the item's outcome:
   `run`, `pass`, `kind`, `id`, `source`, `target`, `relation`, `status`, `value`, `reply`, `model`,
   `prompt_sha256`, `started`, `ended`.
   `--resume RUN` reads `run.yaml`, refuses a different KB root, format or pass, and skips every
   item that already has a `done` line, re-asking `malformed` and `failed` ones. A fresh run mints a
   new id and re-asks everything: a recorded value is not evidence about the current one.
   `--dry-run` writes `run.yaml` and `items.jsonl` (one pending line per item), makes no call, writes
   no KB, and cannot be resumed; its outcome is `done` with `counts.done` `0`.
8. **The lock.** `<state-dir>/score/lock` is held for the run (`filelock.Acquire`). A held lock is
   `retry` with `check: lock` and `path` the lock's **parent directory**, `<state-dir>/score`: the
   lock file exists only while it is held, so the item names the directory a reader can inspect. The
   run writes nothing else.
9. **Refresh cadence, and the migration's write.** One `refresh` at run start, before the index is
   loaded and the set enumerated, so the index is current and a `1.0.0` KB is converted and saved
   by it. This is the model's rule, not a plan policy: a run is a natural write, so it migrates
   (invariant 9) — the start refresh saves the conversion, does the converter's cleanup and
   re-derives the index. The `kb.Source`'s `Migration()` is read before the refresh, and `cmd`'s `withMigration`
   puts the record after `kb-root` on the run's document (and the one stderr sentence), as every
   other verb does. One more `refresh` at the end if any write landed — on completion, on `--limit`,
   and on cancellation. A crash leaves a stale index, which `verify` reports and the next run's
   start refresh repairs. Under `--dry-run` neither runs, so a dry run asks nothing and writes
   nothing: it loads the index as it stands and refuses one the load cannot answer, a migrated KB's
   included — that refusal names `kbase refresh`, and a real run on the same KB succeeds.
10. **MCP.** `rootCmd.AddCommand(mcpBinding(scoreCmd, mcpExcluded))`. `score` is not read-only. It
    appears in no `tools/list`; `cmd/mcp_test.go` already requires every command to declare a
    binding.
11. **One human-scored exception.** A work's `rigor` is never in the scoring set, never askable
    through either renderer, and never written by a run. It stays the user's, through `set-rigor`:
    a work's standing is judged once, with the document in hand, and never re-posed. kbase cannot
    tell a human caller from a scorer at the write op; the exclusion lives in the set and the
    renderers. A `founds` edge is no exception: its bearing is a bearing subject like any other,
    judged every run.

## What the split gives up, and what it does not promise

**A work's rigor stays human.** A work's rigor is the work's own standing, judged with the document
in hand; it cannot be read from the corpus the claims live in, so neither pass asks for it. It is
assigned once, and `set-rigor` writes it by hand.

**A `founds` bearing does not.** Whether a cited work or an axiom bears on a claim is a judgement
about what that source states, weighed against the claim as it now stands — and the target moves
under a fixed premise, so the bearing is re-asked whenever the pass runs. `founds` is a bearing
subject on the same footing as `supports`, `lifts` and `refutes`, and an axiom's `founds` edge is an
ordinary one of them.

**A bearing is asked only about an edge that already stands.** The pass never creates, removes or
reverses an edge: existence and direction stay the build's (`classify`, `unmarked`). The ask's
criteria say the premise is assumed true and used, and that the question is about the target's
subject.

**What the split newly fails at, and why it is bounded.**

| Failure | Why acceptable |
|---|---|
| A non-sequitur premise stops being a rigor fault | Intended: the premise's bearing multiplies into the derivation, so an off-point premise shows in the claim's strength |
| An unscored premise is excluded, so it reads like an edge that is not there | The edge still stands in `deps` as `*pending*`; only a newly authored edge starts unscored, and the runner validates one value per item, so a pass cannot leave one behind by skipping |
| A `founds` bearing is judged against a target that moves under it | Intended: the ask carries the target's current statement, and each run re-asks, so a fixed premise is re-weighed against the claim as it now stands |
| A bearing answered `1.0` on shared subject matter | One edge is one row to correct; the citing sentence is local evidence |
| One call per edge | Accepted: a run is minutes, and the judgement needs its source and target in view |

**No one-direction guarantee.** A pass moves claims both ways: a lowered bearing lowers the claim, a
lift above the derivation raises it, and a refutation caps it. A bearing the migration wrote at `1.0`
can only be lowered by a judge, but one carried from a pair strength below `1.0` can be raised, and a
refutation adds a cap. A `founds` bearing is asked like the rest and so moves like the rest: a target
that drifted under a fixed work or axiom is re-weighed on the next run.

## Decisions taken

- **The text renderers are internal functions, not verbs.** The only consumer is in this command; a
  public read-only verb would need a second result shape for prose (`query.Page` accepts lists), and
  a public surface with no client is the defect this design already refuses elsewhere. The exclusion
  of out-of-scope subjects survives in the function signatures, which is where it has a caller.
- **The prompts live on `internal/asks`' shelf** — `score-rigor.tmpl.md`, `score-bearing.tmpl.md`,
  and one shared `fragments/score-system.tmpl.md` added to `fragmentSlots`. Provenance is the
  package's existing rule: embedded in the binary, written by kbase, no upstream origin and no
  version pin to drift. The templates carry no model name, no provider and no score vocabulary from
  the sibling pass. `asks.Lint` runs over them in a test; the review gate is the index's.
- **The scoring set has no verb.** It is a function over the loaded index
  (`query.ScoringSet(ix, pass)`), consumed in-process; `--dry-run` is the inspection surface.
- **The ledger lives under the build state store's directory.** `internal/statedir` is extracted from
  `internal/build/store.go` (`stateDir`, `stateKey`) and used by both; a scoring run and a build
  then agree on what "the state directory for this KB" means, and neither invents a second
  convention. It is a code move only: `storeformat.go`'s `checkStateDir` and the `--state-dir`
  refusal (`check: state-dir`, `key: --state-dir`, remedy naming a store kbase can read) move with
  the resolution and their result text is unchanged; `internal/statedir` returns the same error and
  the caller wraps it as it does today.
- **The smoke test is a verb-logic test with a mock, plus one binary leg.** `cmd/score_test.go`
  drives `runScore` over a fixture with `providerOptions.NewClient` returning a scripted
  `model.MockClient`: it lands values, then `--resume` asks zero items. The binary leg is
  `score --dry-run` in `test-integration-results-fixture`, checked by `TestResultDocument`. There is
  no network-dependent integration target.

## Work packages

### WP1 — the prompt templates

**Scope.** `internal/asks/prompt-templates/score-rigor.tmpl.md`,
`internal/asks/prompt-templates/score-bearing.tmpl.md`,
`internal/asks/prompt-templates/fragments/score-system.tmpl.md`, `internal/asks/compose.go`
(`fragmentSlots`), `internal/asks/score.go` (new), `internal/asks/score_test.go` (new).

**Build.** Each template composes from `dyn.` slots the caller supplies and splices the shared system
fragment. The rigor template states: judge this node's own rigor, not its premises; answer one line,
a number in `[0,1]` or `*pending*`; nothing else. The bearing template states: the premise is
assumed true and used; judge how much of the source bears on the target's subject; answer one line,
a number or `*pending*`. Neither names the other quantity. `ScoreRigor`/`ScoreBearing` render them
through `asks.Render` and return a `ComposeError` for a missing slot. A test asserts both templates
compose with the slots the runner supplies and that `Lint` finds no unfilled slot.

**Acceptance.** Both render; a missing slot is a `ComposeError`; no template line names a rigor,
bearing or strength value.

### WP2 — the closed text

**Scope.** `internal/score/text.go` (new), `internal/score/text_test.go` (new); a
`kb.MarkerParagraph`-shaped helper in `internal/kb` only if the paragraph extent cannot be had from
`kb.AnchorSection`/`kb.MarkerLines`.

**Build.** `rigorText(src *kb.Source, n query.ScoringNode) (string, error)` and
`bearingText(src *kb.Source, e query.ScoringEdge) (string, error)` as in the runner shape. A claim's
statement is found in the leaves that declare it (`LeafRecord.Claims`), at the tier-2 marker's
paragraph or block; its register section is never opened. `kb.MarkerLines` gives the marker's line
number only, so the extent rule is explicit: from the marker's line, the run of non-blank lines
containing it — a `>` blockquote's whole block where the marker sits in one — clipped at the next
line that opens a new heading. An analysis's and an experiment's statement is `kb.AnchorSection`
over the leaf its `CanonicalPath` names; an axiom's `CanonicalPath` and anchor are its declaration's
in `axioms.md`, so the same reader gives the statement a `founds` edge carries as its source.
`bearingText`'s target passage is the same statement text, not the premise list. Its source passage
is the source's own statement on the same rule. Reads go through `kb.Source` (the loader monopoly),
so a migrated KB reads in its current form. The score guard is one regex over the assembled text; a
hit is an error naming `path:line`, never a silent strip, and the item is `failed` with no call made.
Out-of-scope subjects return a typed refusal.

**Facts the step rests on.**

- `grep -n 'func AnchorSection\|func MarkerLines' internal/kb/leaf.go` → `:371`, `:87`; the section
  reader and the marker-line map both exist.
- `grep -n 'func (s \*Source) ReadText' internal/kb/source.go` → `:140`, the only document read.
- `grep -n 'CanonicalPath' internal/kb/indexstream.go` → `:55`; every node row names its document —
  the leaf for an analysis or experiment, the register for a claim, `axioms.md` for an axiom. The
  renderer opens the leaf and the axiom source, and never the register.

**Acceptance.** A claim, analysis and experiment render a non-empty text, and an axiom renders its
declaration's body as a `founds` source; a work or axiom id is refused by `rigorText`, and
`references` and `demoted` are refused; no rendered text contains `- rigor:`, `(bearing`,
`- strength:` or `(strength `. A title containing the bare words "bearing 0.5" is not a token and is
not removed: it renders and is sent, which is correct — the guard rejects tokens, not the topic. A
claim whose tier-2 marker sits mid-section renders that paragraph alone, not the section.

### WP3 — the scoring set

**Scope.** `internal/query/scoring.go` (new), `internal/query/scoring_test.go` (new).

**Build.** `ScoringNode{Kind, ID}` and `ScoringEdge{Source, Target, Relation}`, and one item type
carrying either: `type Item struct { Node *ScoringNode; Edge *ScoringEdge }` — exactly one is
non-nil, so the runner asks by the field it has rather than re-reading the pass. `ScoringSet(ix
*Index, pass string) ([]Item, error)` over the loaded index, in the order named above. It reads
`ix.nodes` and `ix.edges` in-package, so the index's fields stay private. An unknown pass is an
error. Works and axioms are absent from the rigor set; `references` and `demoted` from the bearing
set. An axiom's `founds` edge is present.

**Acceptance.** A fixture with one of every kind yields exactly the claim, analysis and experiment
for `rigor`, and the claim/experiment `supports`, work/axiom `founds`, analysis/experiment `lifts`
and `refutes` for `bearing`; a work and an axiom are absent from the rigor set, and both `founds`
edges are present in the bearing set. The order is stable across reloads.

### WP4 — the state directory and the ledger

**Scope.** `internal/statedir/statedir.go` (new, moved from `internal/build/store.go`),
`internal/build/store.go`, `internal/score/ledger.go` (new), `internal/score/ledger_test.go` (new).

**Build.** Move the override/XDG/key logic and the "not inside kb-root" refusal into
`internal/statedir`, with `storeformat.go`'s `checkStateDir` (`:43`) and the store-version check
(`storeVersion`, `:29`) that the `--state-dir` refusal names. The move is mechanical: `build` calls
`statedir` and keeps its own result text; `internal/statedir` returns the resolution error and the
caller wraps it into the `state-dir` item it wraps today, so a refusal's `check: state-dir`,
`key: --state-dir` and remedy text do not change. `score` calls it the same way.
`Ledger` opens `<dir>/score/<run-id>/`, writes `run.yaml` with `atomicfile.Write`, appends each JSON
line with `O_APPEND` and `Sync`, and reads back the last status per item for `--resume`. A ledger
line is written **after** the item's write and validation, never before: a crash between the write
and the append re-asks an item whose write is idempotent, which is safe; the reverse would skip a
value that never landed.

**Acceptance.** Two runs with different KB roots get different directories; a `--state-dir` inside
`kb-root` is refused; a ledger re-reads the last status per item; a truncated final line is ignored
and the item re-asked.

### WP5 — the runner

**Scope.** `internal/score/run.go` (new), `internal/score/answer.go` (new),
`internal/score/run_test.go` (new).

**Build.** The runner owns the loop and no I/O of its own. It takes the enumerated items, an `Ask`
seam, a `Write` seam, the ledger, `Limit`, `DryRun` and a logger. Per item: ask → `ParseAnswer` →
on malformed append `malformed`, continue → on ask error append `failed`, continue → write → append
`done`, accumulating `written`. After the loop it reports `Counts` and `Remaining`, both computed
from the ledger's last line per item rather than this invocation's attempts, so a resumed run reports
the whole run; the caller refreshes and composes the result. `ParseAnswer` accepts one trimmed line: a decimal in `[0,1]` or
the pending literal; it rejects a trailing sentence, two numbers, a percent sign and an empty reply.
The abort rule is the context: a cancelled `ctx` stops before the next ask, appends nothing for the
item in flight, and returns `cancelled` with the ledger as it stands.

**Acceptance.** A scripted sequence of good, malformed, failed and good answers yields
`done`=2, `malformed`=1, `failed`=1, `remaining`=2, and the two good values reach the write seam. A
cancelled context stops at an item boundary. `--limit 1` over three items yields `bounded`,
`done`=1, `remaining`=2.

### WP6 — the command

**Scope.** `cmd/score.go` (new), `cmd/score_test.go` (new), `cmd/resultkeys_test.go`.

**Build.** Flags: `--pass` (required, `rigor|bearing`), `--provider` and `--timeout` (the shared
`registerProviderFlags`, `cmd/provider.go:370`: a duration, default `30s` — `defaultListTimeout`,
`:24` — applied per call here), `--limit`, `--dry-run`, `--resume`, `--state-dir`. `scoreOptions`
embeds `providerOptions`, so `NewClient` is the seam `cmd/score_test.go` uses. Registration:
`rootCmd.AddCommand(mcpBinding(scoreCmd, mcpExcluded))`. **The command body is one function**,
`runScore(opts scoreOptions) (int, error)` in `cmd/score.go`; the `scoreCmd` action calls it, and
`cmd/score_test.go` and the smoke test drive it directly — every later reference to `runScore` in
this document means that function, and the migration assertions go through it. It opens the KB
through `openKB` (so a migrated KB reads in its current form and a `2.1.0`/`3.0.0` KB refuses),
reads `Migration()` off that source for the document, mints `run`, takes the lock, refreshes once —
this is the write that converts and saves a `1.0.0` KB — then loads and enumerates the index. A
missing or unreadable index is refused with `check: index`, remedy `kbase refresh`; *staleness* is
what the start refresh repairs, so it is never a refusal. Under `--dry-run` the branch order is:
open, read `Migration()`, mint `run`, take the lock, **skip the refresh**, load the index (refuse
one the load cannot answer), enumerate, write the ledger's item list, emit — so it dials nothing
and omits the four provider keys; otherwise it calls `providerOptions.dial`, takes
`r.Inference.Text` as the model, and fills `provider`, `provider-rule`, `provider-layer`, `model`.
It opens the ledger, runs the loop,
refreshes once more if anything was written (including on `--limit` and cancellation), and emits the
document, `withMigration(fields, src, "score", opts.Stderr)` after `kb-root`. `cmd/resultkeys_test.go`
gains `"score"` in `documentedKeys` and `"score.counts"` in `documentedSubKeys`.

**Facts the step rests on.**

- `grep -n 'func loadVerbContext\|func openKB' cmd/*.go` → the shared preamble every KB verb uses.
- `grep -n 'func (o providerOptions) dial' cmd/provider.go` → the one provider-resolution path.
- `grep -n 'mcpExcluded' cmd/wait.go` → the precedent for a bound-but-excluded command.
- `grep -n 'documentedKeys\|documentedSubKeys' cmd/resultkeys_test.go` → `:19`, `:39`; `score` is
  absent today, and the integration leg that runs it would fail the key check without these entries.
- `grep -n 'func withMigration' cmd/refresh.go` → `:94`; the one helper that attaches the record
  after `kb-root`. `grep -n 'Saved()' internal/index/refresh.go` → `:446`, the refresh clearing
  `migrated` so the same `Source` loads an index after it writes the conversion.
- `grep -n 'func registerProviderFlags' cmd/provider.go` → `:370`: `--provider` and `--timeout`
  arrive together; `defaultListTimeout` is `:24`.

**Acceptance.** `kbase score --help` names the two passes and the one human-scored value. A `--pass
other` is `check: usage`. A held lock is `retry` with `path` `<state-dir>/score`. `tools/list` does
not name `score`. The result document passes `TestResultDocument` in both dry and real modes. On a
`1.0.0` KB a real run is `done`, the KB is `2.0.0` afterwards, and the document carries one
`migration` record after `kb-root`; a dry run on the same KB before it is `refused` with remedy
`kbase refresh`, and the runs after it carry no `migration`.

### WP7 — fixtures, the smoke test, the binary leg

**Scope.** `cmd/testdata/score-kb/` (new), `justfile`'s `_test-integration-results-fixture`.

**Build.** The fixture is a `2.0.0` KB with two claims, one analysis, one experiment, one work, one
axiom, one `supports` edge from a claim, one `founds` edge from the work, one `founds` edge from the
axiom, one `lifts` edge and one `refutes` edge — enough that each pass has a non-empty set, each
exclusion has a witness, and the axiom `founds` edge's membership in the bearing set is falsifiable.
`TestScoreRunLandsAndResumes` copies it into a temp repo with a bare `.git`, runs `index.Refresh`,
then `runScore` with a scripted `model.MockClient` seeded with one answer per item:

1. The first run writes `done` for every item; a re-read through `query.Load` and
   `query.ShowPayload(ix, id)` shows each landed value, both `founds` bearings included; the work's
   `rigor` is unchanged: a work has no item.
2. A second call with `--resume <run>` and a client that errors on any call reports `done` and issues
   zero calls.
3. A malformed script lands no value for that item and the run's `counts.malformed` is 1.
4. `internal/migrate/testdata/register-1.0.0`'s `kb-root` — a real `1.0.0` KB — copied in place of
   the fixture's and scored through `runScore`: it lands a value, is `2.0.0` afterwards, and its
   document carries one `migration` record after `kb-root`; a second run on it carries none.

The recipe gains one leg after the insert: `run score done score --dry-run --pass rigor --state-dir
"${root}/${out}/state"` — `run`'s signature is `run <verb-for-keys> <outcome> <args…>`
(`justfile:1083`). It proves the binary enumerates, writes the ledger's item list, needs no provider,
and emits the documented keys.

**Acceptance.** The smoke test passes with no network; the recipe leg is `done`; the ledger directory
contains `run.yaml`, `ledger.jsonl` and `items.jsonl` where each applies; the `1.0.0` case migrates
and saves.

### WP8 — contract text

**Scope.** `SPEC.md`, `ARCHITECTURE.md`, `README.md`, `INTEGRATING_KBASE_VIA_MCP.md`,
`USER_CONVENTIONS.md`.

**Build.**

- **SPEC §3**: "kb-claim-scorer grades derivations" becomes "`kbase score` grades the KB's rigor and
  bearings".
- **SPEC §7**: the `score` row in the per-subcommand table, in the order WP6 emits — `migration`
  first among the conditional keys, since a scoring run is the call that saves a converted KB; the
  shared vocabulary already carries `migration` from Phase 1. The `counts` record's keys are
  documented as a sub-record, and `resume`'s absence from a dry run is stated.
- **SPEC §8**: a `score` entry — what the two passes are, one item per call, the one human-scored
  value, the ledger and resume, the model category, and that it runs the binary rather than the
  server.
- **SPEC §11**: the exclusion list gains `score`; the read-only set is unchanged.
- **ARCHITECTURE**: a scoring section — the runner, the provider seam, the prompt shelf, the closed
  text and its guard, the scoring set, the ledger and the state directory, the one-op landing, and
  `mcpExcluded`. It cites SPEC §8 rather than restating the pass rules.
- **README**: the command list and a scoring paragraph. **USER_CONVENTIONS**: the command list.
- **INTEGRATING_KBASE_VIA_MCP.md**: the installed set's scoring sentence becomes "scoring runs
  `kbase score`; the former scorer seat is retired by the adjagent hand-off", and the scoring route
  is named as a CLI verb, not a tool.
- **ROADMAP.md** is Phase 1's (the bearing item, `intervolume`, the next major). Phase 2 adds no
  item there: the uncertainty-pool item's precondition ("a scoring pass has run on the fixture") is
  reachable once this phase lands, and the item stands as written.

**Acceptance.** Every edited sentence holds against `./bin/kbase score --help` and one dry run. No
document claims a scoring tool on the MCP surface.

## Contract deltas

Phase 2 owns SPEC §3's scoring sentence, §7's `score` row, §8's `score` entry and §11's exclusion
list; ARCHITECTURE's scoring section; README; INTEGRATING; USER_CONVENTIONS. It touches no format
statement: Phase 1's §10 is already the truth about `rigor` and `bearing`.

## Acceptance observations

Each is an observation; the falsifier is what a reviewer runs to disprove it. A work package's
`**Acceptance.**` block is its done-check; these are the landing's.

1. **Both passes land values.** On the fixture, a rigor run writes one value per claim, analysis and
   experiment; a bearing run writes one per `supports`, `founds`, `lifts` and `refutes` edge;
   `verify` returns `done` after each. *Falsified by:* a skipped node kind, a work rigor written, or a
   refusal after the write.
2. **A work is outside both passes; every scoring edge is a bearing subject.** A work's rigor never
   appears in the set and `rigorText` refuses a `wrk-` id; an axiom is absent from the rigor set too,
   and the bearing set carries every `supports`, `founds`, `lifts` and `refutes` edge.
   *Falsified by:* a work item in `items.jsonl`, a renderer answering a work, or a `founds` edge
   missing from the bearing set.
3. **One item, one quantity, one call.** Each call's prompt names one node or one edge; each reply
   parses to one value; no call's text carries the other pass's quantity or an existing score.
   *Falsified by:* a two-subject prompt, a pair in a reply, or a score token in any assembled text.
4. **A malformed answer writes nothing and does not stop the run.** It is `malformed` in the ledger,
   counted, and the next item proceeds. *Falsified by:* a defaulted value, a halted run, or a
   ledger line without the reply.
5. **Resume continues a run and does not cache.** `--resume` skips `done` items and re-asks
   `malformed`/`failed`; a fresh run re-asks everything. *Falsified by:* a second fresh run issuing
   zero calls, or a resume issuing a call for a `done` item.
6. **The ledger is complete and inspectable.** Every attempt has one line with the model, the prompt
   digest and the reply; a resumed run's counts cover the whole run, not just the resumed part.
   *Falsified by:* a missing item line, a count that resets on resume, or a line written before its
   write.
7. **The run leaves the index current.** A completed, bounded or cancelled run refreshes once after
   its last write; a subsequent query answers without a warning. *Falsified by:* a stale index after
   a clean exit, or a per-item refresh that rewrites the sheet on every item.
8. **`score` is not on the MCP surface.** `tools/list` omits it; the read-only set is unchanged.
   *Falsified by:* a bound tool, or a read-only annotation.
9. **Two concurrent runs do not interleave.** A second run is `retry` naming the lock. *Falsified by:*
   two ledgers under one run id, or two runs writing the same edge concurrently.
10. **The dry run is inert.** It is `done`, its `counts.done` is `0`, it carries no `resume`, it calls
    no provider, writes no KB file, and still emits the set and the documented keys. *Falsified by:* a
    network request, a KB write, a `written` that is not `[]`, `counts.done` above `0`, or a `resume`
    on a dry-run document.
11. **The binary leg holds.** `score --dry-run` on the results fixture is `done` and parses against
    its documented keys. *Falsified by:* a key-check failure or a provider requirement.
12. **A scoring run is the write that saves a converted KB.** On a `1.0.0` KB, a real run is `done`,
    the KB is `2.0.0` afterwards, and the document carries exactly one `migration` record after
    `kb-root`; the next run carries none. *Falsified by:* a `refused` outcome for a `1.0.0` KB, a KB
    still at `1.0.0` after a `done` run, a missing or duplicated record, or a record on the second
    run.

## Gates

`just test`; `just edit-gate`; `just test-integration` (with the new leg); `just test-integration-mcp`.
No target reaches a network. The prompt-review gate is `ACTIVE_PLAN.md`'s.

## Ordering traps

Each rule stated in a work package is not repeated here. These are the ones whose cost is paid by
the *order* the work is done in, or that a coder must hold across packages.

1. **Phase 1 first.** `score` cannot compile against `Fraction`, `depends-on` or `strength`-as-local.
2. **Enumerate once.** Values move as items land; the set is structural and must not be re-read
   mid-run, or a landed bearing changes what the next item's text sees.
3. **Do not skip an edge whose source is unscored.** A pending source strength excludes the edge from
   the walk, but the bearing is still a judgement to record; write it.
4. **`--no-refresh` per op; one refresh at the start and one at the end if anything landed.** A
   per-item refresh rewrites the whole KB and the sheet per item; a missing final refresh leaves the
   index stale after a clean exit.
5. **The experiment write is `set-rigor` with an `exp-` id**, and it lands in the host leaf's
   mapping, not a register.
6. **The text guard runs before the call.** Assembling and then "cleaning" a score out of the text is
   the failure; the guard refuses and names the source line.
7. **The run lock is taken before enumeration**, and the start refresh before it: a second run cannot
   enumerate a set the first is about to change, and the refresh is the write that converts and saves
   an older KB, so the index the set is read from is always the current one.
8. **`bounded` is not `done`.** A `--limit` run must carry `resume`; a `done` with items unattempted
   is a lie the skill will repeat to the user. A dry run is the exception the other way: it is `done`
   with `counts.done` `0` and no `resume`, because it attempted nothing by design.
9. **A dry run on an older KB is refused, and that is correct.** The refresh that would convert it is
   a write, and the dry run writes nothing; the refusal names `kbase refresh`, and the next real run
   migrates and saves.
