# ACTIVE PLAN

**Scope.** The `2.0.0` format (Phase 1) and the scoring loop (Phase 2) are landed. The next landing
is **Phase 4, the inference asking rework**: how kbase names a model, resolves it to an endpoint, and
what it asks it through. Phase 3 — the two external hand-offs that consume the format — stands
beside it, owned elsewhere and landing nothing here.

Phase 4 is planned in two steps. The **design is its first work item**: it settles the endpoint
resolution, the client lifecycle, the per-role model references, the `scoring` role, the two
scoring-time flags and the harness back end, and names the work packages afterward. Nothing in Phase 4
is coded before that design is written and gets its prompt-engineer review — a design is not the
subject of an intersprint review. The intersprint review follows this rework's landing and covers the
scoring driver sprint as a whole.

## Plan documents

| Phase | Document | Lands | Depends on | Status |
|---|---|---|---|---|
| 1 | — | one commit series, merged once | — | **landed**, one squashed commit `07bf4c6` on `main` |
| 2 | — | one commit series, merged once | Phase 1 | **landed** on `kb-score`, WP1–WP8 and the contract fixes, plus the `test-data/` rename; `just checkpoint` green. **The merge waits on the intersprint review**, which follows Phase 4's landing and covers the scoring driver sprint. |
| 3 | [`PLAN_3_TOOLING.md`](PLAN_3_TOOLING.md) | nothing here; two external repositories | Phases 1, 2 | ready: both phases are landed; external owners |
| 4 | [`PLAN_4_INFERENCE.md`](PLAN_4_INFERENCE.md) | one commit series, merged once | Phase 2 landed | **landed** on `kb-score`: `WP1` resolution and the registry, `WP2` the scoring role and the flags, `WP3` the harness back end, `WP4` the rigour-and-bearing rubric — charge lines priced by kbase ([scheme](RIGOR_AND_BEARING_SCORING_SCHEME.md), [design](RIGOR_AND_BEARING_SCORING_DESIGN.md)) — and `WP5` the contract text; **the merge waits on the intersprint review** over the scoring driver sprint |

`PLAN_2_SCORE.md` is retired with Phase 2 landed; its open issues are carried in this document, and
git holds the record of what it said. Phase 1's plan was retired the same way.

[`STRENGTH_SCORING_SCHEME.md`](STRENGTH_SCORING_SCHEME.md) states the scoring model — the three
quantities, the four relations and the walk's arithmetic. The format is [SPEC.md](SPEC.md) §10, the
configuration surface §9, and [ARCHITECTURE.md](ARCHITECTURE.md) is how this implementation carries
them.

## Phase 4 — outcomes

1. **Resolution is not connection.** Which endpoint a verb uses is a fact about the KB's
   configuration, decided without I/O and refused by name before anything is asked; opening a client
   is a separate step. One verb may hold several clients at once — a build whose decision and text
   roles sit on different providers — and closes them together at its exit.
2. **Per-role model references.** A model is named `provider/model` in the configuration, and the
   global `provider` goes: each role carries its own provider, model, rule and layer, so the result
   document reports those per role rather than for the verb. Existing configurations that name a
   provider globally get a stated compatibility path, not a silent reinterpretation.
3. **A `scoring` role.** `[models] scoring` names the model the scoring asks go to. Absent, it is a
   refusal — never a fallback to `text` — in provider mode and in harness mode alike.
4. **Two scoring-time flags.** `--scoring-model` chooses the model at scoring time; `--harness`
   chooses the back end the asks go to. They compose: `--scoring-model` names the model to use in
   either back end, a pool reference under provider mode and the harness's own model id under a
   harness.
5. **The harness back end.** The scoring runner already reaches a model through one seam. A harness
   implementation of that seam is the second back end, and the design states what the ledger records
   when a harness answers, how one item is kept from carrying into the next, whether a harness failure
   is an item's or the run's, and whether the model a document names is the one requested or the one
   the harness reports using.
6. **The contract moves with it.** SPEC §9 for the configuration surface, §7 and §8 for the result
   keys and the scoring flags, and `show-config`, `configure`, README, USER_CONVENTIONS and the MCP
   integration guide for the reader's side.

## Carried forward from Phase 2

Open issues the scoring landing leaves, none of them blocking its merge:

- **A `bounded` run that also failed an item** reports `bounded` and carries no `failures` record;
  the reasons stand in `counts.failed` and the ledger only, because `bounded` outranks `failed`.
- **The landing's write lock is unproven under contention.** A landing denied the KB write lock is
  that item's `failed`, and the repo has no cheap way to hold that lock without its fixed wait, so
  the behaviour is covered by code reading alone. A wait seam for `holdKB` would close it for every
  writer, not just scoring.
- **A stop in the end refresh** reports the refresh's outcome over the walk's, so a run whose walk
  was cancelled and whose closing refresh was retried reads `retry`. Which of the two a two-half
  failure reports is a contract question, not a code one.

- **A tool-less agent for the ask**, supplied by adjagent and usable from either harness through its
  own agent configuration, so a harness profile can run an agent that cannot read the KB instead of
  one that is merely instructed not to. Phase 3's territory; unowned.

## Order and gates

Phase 4's order is design, then its prompt-engineer review, then work packages, then the contract
text in the same landing. The design is reviewed before any code, because it decides the shape of a
surface the verbs that load configuration read.

Per landing: `just test`, `just edit-gate`, and the integration targets the plan names. The Phase 2
landing adds one leg to `_test-integration-results-fixture` — the `score --dry-run` invocation, run by
the public target through the `run <verb-for-keys> <outcome> <args…>` helper — and no
network-dependent target.

**Seats.** The contract documents (`SPEC.md`, `ARCHITECTURE.md`, `README.md`, `USER_CONVENTIONS.md`,
`INTEGRATING_KBASE_VIA_MCP.md`) are dual-audience and belong to the tech-writer seat; the coder
supplies the built behaviour and the acceptance evidence, and the writer lands the prose in the same
landing. Model-facing text — the two scoring prompt templates, and any the harness back end adds —
gets prompt-engineer review before its first real run and before merge. The Phase 4 design is the
architect's.

## What this plan does not do

- It does not change the scoring model: no fifth relation, no bearing on the sheet, no new derived
  quantity.
- It does not let a pass read the other pass's answer, and it does not make a scoring run's judgement
  a verdict.
- It does not take up Phase 3's hand-offs, and it does not merge Phase 2 ahead of the review.
