# ROADMAP – kbase

Future intent, not commitment. Design truth lives in [SPEC.md](SPEC.md) and
[ARCHITECTURE.md](ARCHITECTURE.md); the plan under way is `ACTIVE_PLAN.md`. Finished items are
deleted, not archived.

## Now

- The uncertainty pool and hit list, parked: the declared categories yield nothing useful and the
  hit list returns when a scoring pass has run on the fixture and a ranking signal puts the
  author's premises near the top (`ROADMAP_PLANS/UNCERTAINTY_POOL_AND_HIT_LIST.md`; the closed
  plan's record is `ACTIVE_PLAN.md` at commit `34b956e`). Never an all-pairs check.
- **No test drives a scoring landing against a held KB write lock.** A landing denied the lock is
  that item's `failed`, covered by code reading alone: `write.LockWait` is a fixed constant, so no
  test can hold the lock without paying the full wait. A wait seam for `holdKB` would cover every
  writer, not only scoring.
- **A scoring call that ends at the completion cap or the deadline is captured with `reply` `""`
  and no finish reason or usage.** The `score.Ask` seam returns only the reply text or an error, so
  a capture cannot tell a reasoning-exhausted reply from an empty one.
- **`score.NewestRun` reads only `run.yaml`'s `started`**, so a `run.yaml` with another malformed
  field counts toward `status`'s newest run where it was once skipped.
- **CONVENTIONS describes a command flag annotation `kbase.required`; no code sets it.**
- **AGENTS.md routes mechanical dispatches to `reaper/Qwen3.8-Flash-Next`, which the Claude Code
  agent tool cannot select.**

## The command surface

- **`cmd/` surfaces `internal/` capabilities and implements none.** A verb file is its flags, an
  options struct, one call into `internal/`, and the result it emits. The server's capabilities in
  `cmd/mcp.go` (detached launches, resources, subscriptions), scoring-run orchestration in
  `cmd/score.go`, configuration logic in `cmd/configure.go` and the client registry in
  `cmd/provider.go` move to their packages; tool derivation from cobra stays.
- **One registration file states the whole surface.** Each verb file exports a constructor; one file
  registers every verb, one `AddCommand` per family (queries, write ops, KB maintenance, runs,
  configuration, server), each wrapped `readOnlyTool`, `readWriteTool` or `cliOnly`, so the binding
  and the read-only hint live on the registration line and nowhere else. A test refuses a command
  reaching the tree without them. MCP-only surfaces (the rewind tool, the resource families, the
  detached launches) are listed in the same file.
- **Policy: a verb that reads or writes KB content or metadata is a tool.** Only process control
  and environment diagnostics are `cliOnly` (`mcp`, `wait`, `probe`). It lands in ARCHITECTURE
  beside I9.

## Reader fidelity

- Display names from `\input`-ed preambles.
- Resolved numbering: theorem, equation and section numbers as the author's counter scheme produces
  them, where within-section numbering and `\numberwithin` make pandoc's counter disagree.
- The `-latex_macros` reading: pandoc's LaTeX reader with macro expansion disabled (`-f
  latex-latex_macros`), which may preserve author environment names.

## KB format changes
The `2.0` model is in force, at `2.0.1` (the patch is the bottom band's words): five node types —
`claim`, `analysis`, `experiment`, `work` and `axiom`; four scoring relations — `supports`, `founds`,
`lifts` and `refutes` — beside the two unscored ones, `references` and `demoted`; and three
quantities — `rigor`, a node's local value, `bearing`, a scoring edge's scalar, and `strength`, the
derived value.

- `3.0.0`: the derived index as **one file** instead of `.index/`'s five tables, each record
  carrying its table's name. A new table becomes a record type rather than a new file, so a feature
  does not grow the file set and needs no migration for it; the save commits one rename instead of
  five, and the tables no longer have a partial state between them. The line-per-record shape makes
  the merge mechanical; the cost is that every refresh rewrites the file and one malformed line
  takes out every consumer at once.

## Configuration

- **Remove the inert `provider` key from `config.toml` at the next major.** It loads and chooses
  nothing today, and a file-mode run reports it superseded (SPEC §9). It leaves the schema once the
  reader-facing examples, `USER_CONVENTIONS.md` and adjagent's `references/scoring.md` have moved off
  it; until then strict load keeps modelling the key so the file a user has stays loadable and
  `kbase configure` can rewrite it.
- **Let an ask carry its own common call parameters: thinking, temperature and the completion
  cap.** Today all three are fixed below the ask: thinking is disabled on both surfaces
  (`model.Chat`, `encodeAnthropicRequest`), temperature is fixed, and only a Messages request is
  capped, by `maxTokensFor`'s characters-per-token estimate. An ask, or the configuration behind
  its role, should be able to set each, and each surface should send the value in its own
  protocol's field.

## Scoring rubric

Carried from the rubric's landing; the scheme's `## Not settled` is the full statement of each, and
this names only what would move.

- **The bottom cut's height.** The last rung takes every strength under `0.20` and reads
  `unsubstantiated`; where the cut belongs is open, and moving it moves published strengths.
- **Whether a bare assertion becomes a criterion.** Today it charges none of the six and keeps
  `1.00`; adding a seventh `rigor` id and a weight is a revision of the scheme, not an edit.
- **Whether the twelve ids freeze.** A rename moves the ledger's recorded ids and the templates' text
  with them, and so moves `ask_sha256`; what a re-derivation does with a recorded id the running
  table lacks stays open until the vocabulary is frozen or a rule is written for it.
- **The ceiling and the axiom's rule value are one number** (0.8.0): a pass derives `1.00` and
  `set-rigor` takes it, so the reserve is no longer an open question of discipline against
  enforcement.
- **What `counts` reports for a run re-priced mid-run.** The run records the re-price in
  `repriced.yaml`, and no result key states it.

## Deferred from `2.0.0`
Two questions the model answers no to. Each names the move that would settle it.

- **May a work refute a claim directly?** `2.0.0` says no: a work carries no authored role list, so a
  work's counter-evidence reaches a claim as an analysis the work founds, and that analysis refutes
  the claim. The move is a `refutes:` list on the work's register entry; the arithmetic is already
  defined — a cap at `1 − bearing × the work's rigor` — so the change is the list and its parse, not
  the walk.
- **Does the sheet want a bearing axis?** `2.0.0` draws the derived value alone: a bearing is an edge
  judgement, and a second visual axis over it is a ruling of its own. The move is a stroke attribute
  per bearing drawn from the index column every scoring row already carries, with the DOT goldens
  regenerated.

## After kbase is proven in use
- The README's argument-graph images are snapshots of another project's graph. They are replaced by
  `render-claim-graph` output from kbase's own KB once a KB argues the scoring mechanism, so the
  pictures become kbase's own graph rather than an illustration of its capability.
- Records-native leaves with a Go writer, replacing the pandoc writer route.
- The next major bump of the metadata format, `3.0.0` (SPEC §10): jointly established units — a
  strongly connected component a person promotes into one node carrying the strength its members
  share, the DAG constraint over units — once the `demoted` relation has been lived with.
- A complete LaTeX reader in place of pandoc's: the `polytexnical` project (`../polytexnical`),
  taken up only after kbase has shown the pipeline produces a claim graph worth auditing.
