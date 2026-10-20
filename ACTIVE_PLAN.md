# ACTIVE PLAN

**Scope.** Three phases on the `2.0.0` metadata format. Phase 1 lands the format itself: three
quantities (`rigor`, `bearing`, `strength`), four scoring relations, one walk, one converter. Phase 2
lands `kbase score`, the scoring loop kbase now owns. Phase 3 is the two external hand-offs that
consume them — the reference toolchain's `2.0.0` reader and the adjagent skill/scorer retirement —
and lands nothing in this repository.

## Plan documents

| Phase | Document | Lands | Depends on | Status |
|---|---|---|---|---|
| 1 | [`PLAN_1_FORMAT.md`](PLAN_1_FORMAT.md) | one commit series, merged once | — | reissued for the node-pass rework — WP1–WP5 committed, the rework open |
| 2 | [`PLAN_2_SCORE.md`](PLAN_2_SCORE.md) | one commit series, merged once | Phase 1 | blocked on Phase 1 |
| 3 | [`PLAN_3_TOOLING.md`](PLAN_3_TOOLING.md) | nothing here; two external repositories | Phases 1, 2 | blocked; external owners |

The landing branch is `kb-2.0.0-format`. Its checkpoints are WP1 at `d2c49dc`, WP2–WP4 at `7e0cfee`,
WP5 at `81300cf`, the document checkpoint at `d4479dc`, the node-pass design at `8cd1ae4`, the
vocabulary design at `012ba17` and the design of record at `d103548`; the reissued plans stand on the
branch above them. The head moves as work lands and is not recorded here — read it from the branch.
WP1–WP5 are committed checkpoints that implement the model before the node pass: they are reworked
inside the same landing, not accepted as finished, and Phase 1's work packages say for each what is
landed and what the rework adds. Coding resumes on the reissued Phase 1 and continues the one
landing, not a second series.

[`STRENGTH_SCORING_SCHEME.md`](STRENGTH_SCORING_SCHEME.md) states the scoring model — the three
quantities, the four relations and the walk's arithmetic. The format is [SPEC.md](SPEC.md) §10, and
[ARCHITECTURE.md](ARCHITECTURE.md) is how this implementation carries it.

`ROADMAP_PLANS/BEARING_FIELD.md` is retired: its content is folded into Phase 1 and the file was
removed with this checkpoint. There is one plan of record per phase.

## Outcomes

1. **A `2.0.0` KB.** Five node types — `claim`, `analysis`, `experiment`, `work`, `axiom` — and
   four scoring relations. `rigor` is a node's local value, `bearing` a scoring edge's scalar,
   `strength` the derived value. `supports` and `founds` multiply into the min, `lifts` max above
   it, `refutes` cap from above; every scoring edge carries a bearing, an axiom's strength is `1.0`.
   Scoring edges are stored push; the register stays claim-centric under `- rests-on:`. Ids carry a
   three-letter prefix — `clm-`, `ana-`, `exp-`, `wrk-`, `axm-<label>` — and the register is
   `claim-rigor.md`, its authored section `### Rigor` and its derived block `### Strength`. A
   **valid** `1.0.0` KB opens through one converter and is re-derived on its first `refresh`; a file
   already malformed under `1.0.0` is refused by the load and is not the converter's to fix.
2. **A scoring run kbase owns.** `kbase score --pass rigor|bearing` enumerates the scoring set from
   the index, renders closed score-free text, calls the configured provider one item per call,
   validates one value, lands it through `set-rigor`/`set-bearing`, and ledgers the run. It is a CLI
   verb, excluded from MCP. Its start refresh is the natural write that saves an older KB in
   `2.0.0` — a `1.0.0` KB is migrated and saved by `score`, and the run's document carries the
   `migration` record. External works are human-authored and outside both passes.
3. **The consumers.** The reference toolchain reads `2.0.0` and the compat harness compares again;
   adjagent retires `kb-claim-scorer` and routes scoring to `kbase score`.

## Decisions at kickoff

These are the kickoff decisions; each is argued where it is carried.

- **One landing for the format.** Vocabulary, walk, parse, index, query, writer, converter and
  fixtures cannot go green in pieces. Phase 1 is one commit series merged once; no checkpoint
  passes until its last work package is in.
- **Graph words are functions; prose keeps the synonyms.** Five node types and four relations use
  one word each; `invariant` and `framework` are prose, mapped by a contract legend. An axiom is a
  node type with strength `1.0`, not a bearingless special case.
- **Scoring is a kbase verb, not a seat and not an MCP tool.** Provider mode only — no harness, no
  per-item session, no tool allowlist. One item per call, one quantity per answer; kbase composes
  through the walk.
- **The two text renderers are internal.** `rigorText`/`bearingText` are unexported functions of the
  score engine (argued in `PLAN_2_SCORE.md`, Decisions taken), and their signatures are where an
  out-of-scope subject is refused. A public verb is a separate change if an external harness ever
  needs one.
- **The scoring prompts live on `internal/asks`' shelf**, embedded and kbase-authored — the
  package's existing provenance rule. They are model-facing and get prompt-engineer review before
  first use. Their names and shape are `PLAN_2_SCORE.md`'s.
- **The scoring set is enumerated in-process from the index.** No verb, no separate query language.
  A `--dry-run` writes the per-run ledger's item list and no KB change.
- **The ledger is a run record, not a cache**, outside the repository under the build state store's
  directory. A fresh run re-asks every item; `--resume <run>` continues one. Its shape is
  `PLAN_2_SCORE.md`'s.
- **Two subjects are never scored by a pass.** A work's `rigor` and a `founds` edge's `bearing` are
  human-authored through `set-rigor`/`set-bearing`; neither the scoring set nor a text renderer can
  name them.
- **No new dependency.** The provider client, prompt composer, state directory, lock, logger and
  atomic write all exist.

## Order and gates

Phase 1 first: Phase 2 consumes `rigor`, `bearing`, the `lifts`/`refutes` relations and `deps`'
`direction`. Phase 3 consumes both and does not gate either landing in this repository — the compat
harness is flag-gated until the reference toolchain reads `2.0.0`, so CI stays green while it is
blocked.

Per phase: `just test`, `just edit-gate`, and the integration targets named in the phase document.
Phase 1 runs `just test-integration` and `just test-integration-mcp`, whose legs move with the
renames. Phase 2 adds one leg to the private `_test-integration-results-fixture` recipe — the
`score --dry-run` invocation, run by the public `test-integration-results-fixture` through the same
`run <verb-for-keys> <outcome> <args…>` helper — and no network-dependent target.

**Seats.** The contract documents (`SPEC.md`, `ARCHITECTURE.md`, `README.md`, `USER_CONVENTIONS.md`)
are dual-audience and belong to the tech-writer seat; the phase's coder supplies the built behaviour
and the acceptance evidence, and the writer lands the prose in the same landing. The two scoring
prompt templates are model-facing and get prompt-engineer review before their first real run and
before merge.

## What this plan does not do

- It does not promise derived-value equality across the migration. Lifts move station, `founds`
  scales, and untokened work edges stop pending; the installed base is re-derived and re-scored.
- It does not draw a bearing on the sheet, add an "unscored" query, add a pending count, aliases for
  retired names, or a fifth relation.
- It does not install a scoring agent, and it does not let a pass read the other pass's answer.
- It does not make the reference toolchain or adjagent updates here; Phase 3 names their owners and
  their acceptance.

## Deferred chores

- **Rename `test_data/` to `test-data/`.** Every other top-level directory is lower-kebab-case; this
  one is not. The name is spelled in the `justfile`, the transient-path constants in `internal/*`,
  the tests that build paths into it, and the project documents.
