# Phase 2 — `kbase score`

Status: blocked on Phase 1. Landing: one commit series, merged once.
Gates: `just test`, `just edit-gate`, `just test-integration`, `just test-integration-mcp`.

## Preconditions

Phase 1 is merged: `rigor`/`bearing`/`strength`, the four relations, `set-rigor` and `set-bearing`,
`deps` with `direction`, and `query.Load`'s refusal of an index it cannot answer all exist.
Re-establish each before editing:

- `grep -n 'func Run' internal/write/ops.go` → `internal/write/ops.go:191`; the in-process write seam
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
            [--resume RUN] [--state-dir DIR] [--timeout D]
```

It walks the KB's scoring set, asks one question per item, validates one value, lands it through
`set-rigor` or `set-bearing`, and records the run in a ledger. It is a CLI verb and is excluded from
MCP.

Result keys after `kb-root`, in order: `state-dir`, `pass`, `provider`, `provider-rule`,
`provider-layer`, `model`, `run`, `ledger`, `counts`, `written`, `resume`. The four provider keys
are absent together under `--dry-run`. `counts` is `{total, done, malformed, failed, remaining}`.
`resume` is present whenever any item is not `done` — the ones `--limit` left unattempted, and any
`malformed` or `failed` item.

Outcomes: `done` when the run finished; `bounded` when `--limit` stopped it; `cancelled` on
interruption, with `resume`; `failed` when any item's call or write failed; `refused` on setup
(unknown pass, no provider, index or KB the load refuses, KB not `2.0.0`); `retry` when another
score run holds the lock. A *stale* index is not a refusal: the run refreshes at start (invariant 9),
then loads; only an index the load still refuses stops it.

## Runner shape — the invariants

1. **Provider mode only.** `cmd/score.go` uses `providerOptions.dial` and `model.Chat` with the
   resolved text model. No harness, no session, no tool allowlist, no per-item agent, no Decision
   call. Concurrency is off: one item at a time, in a stable order. The reason is the ledger and the
   write path — both are simpler and auditable sequentially, and a run is minutes, not hours.
2. **One item per call, one quantity per answer.** A call carries one node or one edge. The model
   never returns a pair, and kbase never asks for one. Composition is the walk's.
3. **The scoring set comes from the index.**
   - `rigor`: every claim, support and experiment node, by id.
   - `bearing`: every `supports`, `lifts` and `refutes` edge, ordered by relation
     (`supports`, `lifts`, `refutes`), then source, then target.
   - Excluded by construction: works (no rigor), `founds` edges (no bearing), framework `supports`
     edges (no bearing), `references` and `demoted`.
   The set is enumerated once at run start. It is structural; landing a value does not change it.
4. **Closed, score-free text.** Two unexported renderers assemble the only text a call may see:
   - `rigorText(node)`: the node's title and its statement. A claim's statement is the paragraph or
     block its tier-2 marker sits in, in the leaf that declares it. A support's and an experiment's
     is the section at its canonical anchor in its leaf. A claim with no declaring leaf renders its
     title alone. **Never the register's section** — a claim's `CanonicalPath` names the register,
     whose quality block carries the old scores; reading it would either leak them or trip the
     guard on every claim. Never the rationale.
   - `bearingText(edge)`: the source's and target's statements, the edge's authored context, and the
     target's premise list as ids and titles only, with no annotations.
   After assembly, a guard refuses the text if it matches `- rigor:`, `(bearing`, `- strength:` or
   `(strength `, naming the path and line it came from. The guard is cheap and always runs; the call
   is never made on text it rejects.
   The renderers refuse out of scope by name: `rigorText` a `wrk-` id; `bearingText` `founds`,
   `references` and `demoted`.
5. **Validation.** One line: a decimal in `[0,1]` or the pending literal. Anything else is
   `malformed`: no write, the ledger records the reply, the run continues. A transport failure is
   `failed`: no write, the ledger records the error, the run continues.
6. **Landing is the write API's.** A rigor answer becomes `set-rigor {id, rigor}`; a bearing answer
   becomes `set-bearing {source, target, relation, bearing}`. kbase never edits a register or a leaf
   itself. Every op is called with `NoRefresh: true`. A write refusal (a changed KB, a missing
   token) is recorded as `failed` for that item.
7. **The ledger is a run record, not a cache.** Per run: `<state-dir>/score/<run-id>/run.yaml` and
   `ledger.jsonl`. `run.yaml` carries `run`, `kb_root`, `format`, `pass`, `provider`, `model`,
   `started`. One JSON object per item attempt, appended after the item's outcome:
   `run`, `pass`, `kind`, `id`, `source`, `target`, `relation`, `status`, `value`, `reply`, `model`,
   `prompt_sha256`, `started`, `ended`.
   `--resume RUN` reads `run.yaml`, refuses a different KB root, format or pass, and skips every
   item that already has a `done` line, re-asking `malformed` and `failed` ones. A fresh run mints a
   new id and re-asks everything: a recorded value is not evidence about the current one.
   `--dry-run` writes `run.yaml` and `items.jsonl` (one pending line per item), makes no call, writes
   no KB, and cannot be resumed.
8. **The lock.** `<state-dir>/score/lock` is held for the run (`filelock.Acquire`). A held lock is
   `retry` with `check: lock`, `path` the lock file; the run writes nothing else.
9. **Refresh cadence.** One `refresh` at run start, so the index the set came from is current, and
   one at the end if any write landed — on completion, on `--limit`, and on cancellation. A crash
   leaves a stale index, which `verify` reports and the next run's start refresh repairs. Under
   `--dry-run` neither runs: the dry run loads the index as it stands and refuses one the load
   cannot answer, because it writes no KB file.
10. **MCP.** `rootCmd.AddCommand(mcpBinding(scoreCmd, mcpExcluded))`. `score` is not read-only. It
    appears in no `tools/list`; `cmd/mcp_test.go` already requires every command to declare a
    binding.
11. **Two human-scored exceptions.** A work's `rigor` and a `founds` edge's `bearing` are never in
    the scoring set, never askable through either renderer, and never written by a run. They remain
    the user's, through `set-rigor` and `set-bearing`. kbase cannot tell a human caller from a scorer
    at the write op; the exclusion lives in the set and the renderers.

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
  convention.
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
paragraph or block; its register section is never opened. A support's and an experiment's is
`kb.AnchorSection` over the leaf its `CanonicalPath` names. Reads go through `kb.Source` (the loader
monopoly), so a migrated KB reads in its current form. The score guard is one regex over the
assembled text; a hit is an error naming `path:line`, never a silent strip. Out-of-scope subjects
return a typed refusal.

**Facts the step rests on.**

- `grep -n 'func AnchorSection\|func MarkerLines' internal/kb/leaf.go` → `:359`, `:83`; the section
  reader and the marker-line map both exist.
- `grep -n 'func (s \*Source) ReadText' internal/kb/source.go` → the only document read.
- `grep -n 'CanonicalPath' internal/kb/indexstream.go` → `:56`; every node row names its document —
  the leaf for a support or experiment, the register for a claim. The renderer opens the former and
  never the latter.

**Acceptance.** A claim, support and experiment render a non-empty text; a work id is refused;
`founds`, `references` and `demoted` are refused; no rendered text contains `- rigor:`, `(bearing`,
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
error. Works, `founds`, framework `supports`, `references` and `demoted` are absent by construction.

**Acceptance.** A fixture with one of every-kind yields exactly the claim, support and experiment for
`rigor`, and the claim/experiment `supports`, support/experiment `lifts` and `refutes` for `bearing`;
a work and a `founds` edge are absent from both. The order is stable across reloads.

### WP4 — the state directory and the ledger

**Scope.** `internal/statedir/statedir.go` (new, moved from `internal/build/store.go`),
`internal/build/store.go`, `internal/score/ledger.go` (new), `internal/score/ledger_test.go` (new).

**Build.** Move the override/XDG/key logic and the "not inside kb-root" refusal into
`internal/statedir`; `build` calls it and keeps its own result text. `score` calls it the same way.
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

**Build.** Flags: `--pass` (required, `rigor|bearing`), `--provider` (shared), `--timeout`
(per call), `--limit`, `--dry-run`, `--resume`, `--state-dir`. `scoreOptions` embeds
`providerOptions`, so `NewClient` is the seam `cmd/score_test.go` uses. Registration:
`rootCmd.AddCommand(mcpBinding(scoreCmd, mcpExcluded))`. The command opens the KB through `openKB`
(so a migrated KB reads in its current form and a `2.1.0`/`3.0.0` KB refuses), mints `run`, takes
the lock, refreshes once, then loads and enumerates the index. A missing or unreadable index is
refused with `check: index`, remedy `kbase refresh`; *staleness* is what the start refresh repairs,
so it is never a refusal, and a KB it would half-migrate is refused before any write.
Under `--dry-run` it does not dial and omits the four provider keys; otherwise it calls
`providerOptions.dial`, takes `r.Inference.Text` as the model, and fills `provider`, `provider-rule`,
`provider-layer`, `model`. It opens the ledger, runs the loop, refreshes once more if anything was
written (including on `--limit` and cancellation), and emits the document. `cmd/resultkeys_test.go`
gains `"score"` in `documentedKeys` and `"score.counts"` in `documentedSubKeys`.

**Facts the step rests on.**

- `grep -n 'func loadVerbContext\|func openKB' cmd/*.go` → the shared preamble every KB verb uses.
- `grep -n 'func (o providerOptions) dial' cmd/provider.go` → the one provider-resolution path.
- `grep -n 'mcpExcluded' cmd/wait.go` → the precedent for a bound-but-excluded command.
- `grep -n 'documentedKeys\|documentedSubKeys' cmd/resultkeys_test.go` → `:19`, `:39`; `score` is
  absent today, and the integration leg that runs it would fail the key check without these entries.

**Acceptance.** `kbase score --help` names the two passes and the two human exceptions. A `--pass
other` is `check: usage`. A held lock is `retry`. `tools/list` does not name `score`. The result
document passes `TestResultDocument` in both dry and real modes.

### WP7 — fixtures, the smoke test, the binary leg

**Scope.** `cmd/testdata/score-kb/` (new), `justfile`'s `_test-integration-results-fixture`.

**Build.** The fixture is a `2.0.0` KB with two claims, one support, one experiment, one work, one
`founds` edge, one `supports` edge, one `lifts` edge and one `refutes` edge — enough that each pass
has a non-empty set and each exclusion has a witness. `TestScoreRunLandsAndResumes` copies it into a
temp repo with a bare `.git`, runs `index.Refresh`, then `runScore` with a scripted
`model.MockClient` seeded with one answer per item:

1. The first run writes `done` for every item; a re-read through `query.Load` and
   `query.ShowPayload(ix, id)` shows each landed value; the work's `rigor` and the `founds` bearing
   are unchanged.
2. A second call with `--resume <run>` and a client that errors on any call reports `done` and issues
   zero calls.
3. A malformed script lands no value for that item and the run's `counts.malformed` is 1.

The recipe gains one leg after the insert: `run done score --dry-run --pass rigor --state-dir
"${root}/${out}/state"`. It proves the binary enumerates, writes the ledger's item list, needs no
provider, and emits the documented keys.

**Acceptance.** The smoke test passes with no network; the recipe leg is `done`; the ledger directory
contains `run.yaml`, `ledger.jsonl` and `items.jsonl` where each applies.

### WP8 — contract text

**Scope.** `SPEC.md`, `ARCHITECTURE.md`, `README.md`, `INTEGRATING_KBASE_VIA_MCP.md`,
`USER_CONVENTIONS.md`, `ROADMAP.md`.

**Build.**

- **SPEC §3**: "kb-claim-scorer grades derivations" becomes "`kbase score` grades the KB's rigor and
  bearings".
- **SPEC §7**: the `score` row in the per-subcommand table, in the order WP6 emits; the shared
  vocabulary already carries `migration` from Phase 1. The `counts` record's keys are documented as
  a sub-record.
- **SPEC §8**: a `score` entry — what the two passes are, one item per call, the two human-scored
  exceptions, the ledger and resume, the model category, and that it runs the binary rather than the
  server.
- **SPEC §11**: the exclusion list gains `score`; the read-only set is unchanged.
- **ARCHITECTURE**: a scoring section — the runner, the provider seam, the prompt shelf, the closed
  text and its guard, the scoring set, the ledger and the state directory, the one-op landing, and
  `mcpExcluded`. It cites SPEC §8 rather than restating the pass rules.
- **README**: the command list and a scoring paragraph. **USER_CONVENTIONS**: the command list.
- **INTEGRATING_KBASE_VIA_MCP.md**: the installed set's scoring sentence becomes "scoring runs
  `kbase score`; the former scorer seat is retired by the adjagent hand-off", and the scoring route
  is named as a CLI verb, not a tool.
- **ROADMAP.md**: no new item. The uncertainty-pool item's precondition ("a scoring pass has run on
  the fixture") is now reachable; leave the item as it stands.

**Acceptance.** Every edited sentence holds against `./bin/kbase score --help` and one dry run. No
document claims a scoring tool on the MCP surface.

## Contract deltas

Phase 2 owns SPEC §3's scoring sentence, §7's `score` row, §8's `score` entry and §11's exclusion
list; ARCHITECTURE's scoring section; README; INTEGRATING; USER_CONVENTIONS. It touches no format
statement: Phase 1's §10 is already the truth about `rigor` and `bearing`.

## Acceptance observations

Each is an observation; the falsifier is what a reviewer runs to disprove it. A work package's
`**Acceptance.**` block is its done-check; these are the landing's.

1. **Both passes land values.** On the fixture, a rigor run writes one value per claim, support and
   experiment; a bearing run writes one per `supports`, `lifts` and `refutes` edge; `verify` returns
   `done` after each. *Falsified by:* a skipped node kind, a work rigor written, a `founds` bearing
   written, or a refusal after the write.
2. **A work and a `founds` edge are outside both passes.** They never appear in the set, and the
   renderers refuse them by name. *Falsified by:* either appearing in `items.jsonl`, or a renderer
   answering one.
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
10. **The dry run is inert.** It calls no provider, writes no KB file, and still emits the set and the
    documented keys. *Falsified by:* a network request, a KB write, or a `written` that is not `[]`.
11. **The binary leg holds.** `score --dry-run` on the results fixture is `done` and parses against
    its documented keys. *Falsified by:* a key-check failure or a provider requirement.

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
7. **The run lock is taken before enumeration**, so a second run cannot enumerate a set the first is
   about to change.
8. **`bounded` is not `done`.** A `--limit` run must carry `resume`; a `done` with items unattempted
   is a lie the skill will repeat to the user.
