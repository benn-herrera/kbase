# ACTIVE PLAN — scoring issues

**Status:** active, in outline. Points to address are gathered here first; each becomes a
dispatchable package under CONVENTIONS' "Plan dispatchability" once it is understood.

## Objective

Sort out the scoring issues: what the rigor and bearing asks answer, against what the scheme
(`RIGOR_AND_BEARING_SCORING_SCHEME.md`) says they should.

## Points to address

1. **The bearing ask answered a lone `flt-no-engagement` on edges whose source supplies what the
   context claims.** Landed: that id is retired. Bearing now prices how much of the source's own
   argument does work for the target, one charge per part that does none, and every charge costs the
   same `0.10` whatever its reason, so zero takes ten findings rather than one verdict. Open: the
   next capture's count of links whose scale top (`none`) is never reached, and a re-tune of the
   `0.10` against the corpus's average chunk size.
2. **Target coverage is unpriced.** Bearing says how much of the source points at the target, not
   how much of the target the source reaches; `flt-partial` used to price the second quantity and is
   retired. A link that touches one clause of a five-part theorem prices high. The scheme's R&D
   item: a measure of target coverage, and whether it belongs in the bearing value or beside it.
3. **A part's dings are absolute, not a fraction.** Fifty on-target parts and two off read as two and
   two. A second ask quantifying the argument's volume is the only fix considered and is not
   reliable enough to build; the bias is stated in the scheme and the table until one lands. Both
   rigor tables are uniform on the same reasoning — fifteen hundredths a finding, so one reads 0.85
   and seven reach zero — and no table holds a charge that prices a text or a link at nothing alone.
4. **An ask cannot carry its own call parameters: thinking, temperature and the completion cap.**
   All three are fixed below the ask: thinking is disabled on both surfaces (`model.Chat`,
   `encodeAnthropicRequest`), temperature is fixed, and only a Messages request is capped, by
   `maxTokensFor`'s characters-per-token estimate. An ask, or the configuration behind its role,
   should be able to set each, and each surface should send the value in its own protocol's field.
5. **A scoring call that ends at the completion cap or the deadline is captured with `reply` `""`
   and no finish reason or usage**, so a capture cannot tell a reasoning-exhausted reply from an
   empty one. The `score.Ask` seam now reports a truncated reply to the runner, which fails the
   item; the capture still records neither the reply nor why it ended.
6. **No test drives a scoring landing against a held KB write lock.** A landing denied the lock is
   that item's `failed`, covered by code reading alone: `write.LockWait` is a fixed constant, so no
   test can hold the lock without paying the full wait. A wait seam for `holdKB` would cover every
   writer, not only scoring.
7. **`score.NewestRun` reads only `run.yaml`'s `started`**, so a `run.yaml` with another malformed
   field counts toward `status`'s newest run where it was once skipped.

### The rubric's open questions

The scheme's `## Not settled` (`RIGOR_AND_BEARING_SCORING_SCHEME.md`) is the full statement of each;
this names only what would move.

8. **The bottom cut's height.** The last rung takes every strength under `0.20` and reads
   `unsubstantiated`; where the cut belongs is open, and moving it moves published strengths.
9. **Whether a bare assertion becomes a criterion.** Today it charges none of the six and keeps
   `1.00`; adding a seventh `rigor` id and a weight is a revision of the scheme, not an edit.
10. **Whether the fault ids freeze.** A rename moves the ledger's recorded ids and the templates'
    text with them, and so moves `ask_sha256`; what a re-derivation does with a recorded id the
    running table lacks stays open until the vocabulary is frozen or a rule is written for it.
11. **The ceiling and the axiom's rule value are one number** (0.8.0): a pass derives `1.00` and
    `set-rigor` takes it, so the reserve is no longer an open question of discipline against
    enforcement.
12. **What `counts` reports for a run re-priced mid-run.** The run records the re-price in
    `repriced.yaml`, and no result key states it.

### Adjacent: the model, the sheet, the index load, the hit list

13. **May a work refute a claim directly?** `2.0.0` said no: a work carries no authored role list,
    so a work's counter-evidence reaches a claim as an analysis the work founds, and that analysis
    refutes the claim. The move, written for `2.0.0`, was a `refutes:` list on the work's register
    entry; the arithmetic is already defined — a cap at `1 − bearing × the work's rigor` — so the
    change is the authored edge and its parse, not the walk. Under `3.0.0` the authored edge is a
    record in `edges.yaml`.
14. **Does the sheet want a bearing axis?** The sheet draws the derived value alone: a bearing is an
    edge judgement, and a second visual axis over it is a ruling of its own. The move is a stroke
    attribute per bearing drawn from the column every scoring row already carries, with the DOT
    goldens regenerated.
15. **The index load does not refuse a text bearing that is neither a number nor `*pending*`**;
    `stats`' `relations` ranks such text with pending.
16. **The uncertainty pool and hit list, parked:** the declared categories yield nothing useful,
    and the hit list returns when a scoring pass has run on the fixture and a ranking signal puts
    the author's premises near the top (`ROADMAP_PLANS/UNCERTAINTY_POOL_AND_HIT_LIST.md`; the closed
    plan's record is `ACTIVE_PLAN.md` at commit `34b956e`). Never an all-pairs check.

## Post-sprint review

Once the plan's last landing is committed, one pass reviews the changes its packages landed. The
reviewers are an architect and a go-coder. Before they start they read THESIS.md, SPEC.md,
ARCHITECTURE.md and CONVENTIONS.md, and they do not read this plan. They review the changes against
those standards and for fitness for their purpose, and their brief includes the tests.

- **Review**: findings against the four documents.
- **Burndown list**: findings that stand go into a burndown dispatch doc, which gets prompt-engineer
  review and revision as usual.
- **Remediation burndown**: dispatch the burndown list.
