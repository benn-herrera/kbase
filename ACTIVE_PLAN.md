# ACTIVE PLAN

**Scope.** Phases 1, 2 and 4 are landed and merged: the `2.0.0` format, the scoring loop, and the
inference asking rework with the rigour-and-bearing rubric. **Phase 3** — the two external hand-offs
that consume the format — is what remains, owned elsewhere and landing nothing here.

The scoring driver sprint merged to `main` as one squashed commit; the branches are kept locally, so
no history was lost. Its plans are retired and git holds them. `RIGOR_AND_BEARING_SCORING_SCHEME.md`
remains the normative page on its subject: the fault vocabulary, the table, the answer and the ledger.

## Plan documents

| Phase | Document | Lands | Depends on | Status |
|---|---|---|---|---|
| 1 | — | one commit series, merged once | — | **merged**, one squashed commit `07bf4c6` |
| 2 | — | one commit series, merged once | Phase 1 | **merged** in `65e9b78`: the scoring loop, WP1–WP8, the contract fixes and the `test-data/` rename |
| 3 | [`PLAN_3_TOOLING.md`](PLAN_3_TOOLING.md) | nothing here; two external repositories | Phases 1, 2, 4 | ready: the consuming formats are landed; external owners |
| 4 | — | one commit series, merged once | Phase 2 | **merged** in `65e9b78`: resolution and the registry, the `scoring` role and its flags, the rubric's charge-line answer, and the contract text |

`PLAN_1_FORMAT.md`, `PLAN_2_SCORE.md`, `PLAN_4_INFERENCE.md`, `RIGOR_AND_BEARING_SCORING_DESIGN.md`
and `REVIEW_BURNDOWN.md` are retired. What the rubric's design left open is the scheme's
`## Not settled`; what the landings left open is carried below.

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
   refusal — never a fallback to `text` — in either mode.
4. **A scoring-time flag.** `--scoring-model` chooses the model at scoring time: a pool reference
   under file mode, the environment's bare id under the environment's endpoint.
5. **One ask path.** The runner reaches its model through one seam, and the provider fills it: one
   chat call per item on the `scoring` role's endpoint.
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
landing. Model-facing text — the two scoring prompt templates — gets prompt-engineer review before
its first real run and before merge. The Phase 4 design is the architect's.

## What this plan does not do

- It does not change the scoring model: no fifth relation, no bearing on the sheet, no new derived
  quantity.
- It does not let a pass read the other pass's answer, and it does not make a scoring run's judgement
  a verdict.
- It does not take up Phase 3's hand-offs, and it does not merge Phase 2 ahead of the review.

## Post-sprint review

Once the sprint's last landing is committed, one pass reviews the changes since the branch was cut
rather than the individual changes. The reviewers are an architect and a coder; before they start
they read THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md, and they do not read this plan.
Their subject is the changes against those standards and whether the result is fit for its
purpose — not a rehash of how it was implemented.

The section has three parts. **Review**: findings against the four documents — where the landed work
departs from a stated requirement, a stated architecture, or a house rule, and where it is right but
poorly suited to what it is for. **Remediation**: what each finding makes necessary, landed in a
follow-up commit or declined with the reason recorded. **Burndown**: what remains, one fact to an
item, on the list below. Each change has already had its own seat's review; this pass is for what
fell between landings.

### Post-sprint cleanup burndown

- `model.MockClient` is test support sitting in `internal/model`'s production API, beside the two wire
  clients the package doc introduces; the conventional home for a fake that other packages' tests
  import is a sibling package named for the job (`internal/model/modeltest`, as `net/http/httptest`
  is to `net/http`).
