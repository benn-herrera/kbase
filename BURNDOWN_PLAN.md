# Burndown plan — the probe, the score status, and drift

Status: reviewed; dispatch follows. Anchors at `9933501`.

## Why

One landing closes the findings below and one probe bug from a handoff record. Nothing is committed
between the items.

## Work items

### Coder

**1. The score run's result-document write error is discarded.** `cmd/score_status.go:33` calls
`_ = score.WriteResultDocument(...)` (`:22` is the function's signature); the deferred sites are
`cmd/score.go:238` and `:323`, neither of which can return an error. The error goes to the run's
logger (`opts.Logger`). `wait`'s fallback for a run that left no document,
`interruptedScoreDocument`, stays, and its killed-run test (`cmd/score_tool_test.go:216`) stays green.
Accepted when no `_ =` remains on that call, a test makes the run directory unwritable and asserts the
logged line, and the killed-run test still passes.

**2. No test pins the charge-line form.** `internal/asks/score_test.go` lost `lineFormRE`'s check; the
form lives once, in `internal/asks/prompt-templates/fragments/score-system.tmpl.md`. A structural
assertion pins it: the id placeholder and colon from `internal/score/answer.go:49,88`, and the number
or range from `internal/score/text.go:474-477`. It pins no wording and names no sample id — the id-set
assertion at `score_test.go:120` already holds that. Accepted when deleting the form's sentence fails
the test and restoring it passes, and the probe is recorded in the commit message or transient
evidence, not reported alone.

**3. `probe` has no delivered-binary leg.** Add a `probe` invocation through `./bin/kbase` to
`test-integration-results-fixture`, against a stub, and parse its document against SPEC §7's `probe`
keys (`SPEC.md:455`) and their order (`cmd/resultkeys_test.go:35`). The stub is a Go process: the
recipe is git-only, python3 is excluded from the omnibus (`justfile:1149-1153`), and the recipe's only
endpoint today is the closed port at `justfile:1217`. Accepted when the recipe runs green, its
`[doc(...)]` string and header comment (`justfile:1150-1156`) enumerate the new leg, and the leg's
document is under the transient evidence directory.

**4. `probe --decision` asks a decision model on the chat path.** One outcome: the probe's decision
selection asks the surface the entry declares. An `api: anthropic` entry meets its existing `Decision`
refusal (`internal/model/anthropic_client.go:192`) instead of a chat request; an entry with
`decisionsFlavor` reaches that surface. The anchors: the unconditional `model.Chat` at
`cmd/probe.go:100`, the role flags at `cmd/probe.go:169`, `Flavor` at `internal/config/resolve.go:146`,
and the surfaces in `internal/model/decision.go`. The document keeps SPEC §7's keys: a blocking
surface reports `first-token-ms` 0 and `finish-reason` null, which the row already allows. Accepted
when a stub that answers only on the decision surface returns a document, the same stub fails the test
if `Chat` is taken, and the anthropic entry returns the existing refusal rather than the chat 400. The
record is `test-data/transient/empirical-scoring-run-results/ASK_FAILURES_05.md`.

**5. `model.MockClient` sits in the production API.** The unit is `internal/model/mockllm.go` —
including the unexported `splitForChunks` and its test — plus `ErrMockExhausted` in
`internal/model/client.go:392-394`. Both move to `internal/model/modeltest`. The four in-package tests
that use the mock (`chat_test.go`, `decision_test.go`, `drain_test.go`, `mockllm_test.go`) become
`package model_test`; the `splitForChunks` case in `mockllm_test.go` moves with the function. Sweep
the production comment naming the type at `cmd/provider.go:26-28` and every `cmd/` test import.
Accepted when `go doc internal/model` names no test support, and `go vet ./...` and the suite are
green.

**6. `cmd/mcp.go:195` says every flag is optional.** The comment states the fact and stops: the
positionals required, then the command's flags, all optional. It claims no rule the derivation does
not implement. Accepted when the comment matches `commandSchema`.

**7. `just agents` fails obscurely without a sibling clone.** `justfile:147-148` already states the
tandem assumption, so the gap is the failure: `AGENTS_DIR := "../adjagent"` yields a file-not-found.
The recipe fails with a message naming `AGENTS_DIR` and the path it looked for. Accepted by running
`just agents` with the clone absent and reading the message.

### Tech-writer

**8. SPEC still names an internal harness.** `SPEC.md:752` reads "A harness that runs `kbase score` at
a shell gets the blocking behaviour a terminal gets"; the internal harness is gone from the branch.
The external uses are `:16`, `:216`, `:502`, `:1649`, `:1651`, `:1778`, `:1798` and `:1799`. Every use
names the external surface; `:752` says what a shell caller gets. Accepted when the scoring paragraph
reads correctly alone and every `harness` in SPEC names the external surface.

**9. The scoring scheme's superseded note reads as if `0.95` were stale.**
`RIGOR_AND_BEARING_SCORING_SCHEME.md:229-230` says "the tables and values this page states elsewhere
carry the current ceiling", while the statement table at `:270` shows `0.95` for one
`flt-underspecified`, and the method table's unreachable list at `:292` also shows `0.95`. Accepted
when no sentence claims those tables carry the earlier ceiling, every `0.95` outside a table reads as
the earlier figure the note names, and no value changes.

**10. The required-flag sentences disagree.** `ARCHITECTURE.md:1245` says no flag is required;
`CONVENTIONS.md:149` says a flag its verb refuses to run without declares `kbase.required`. The
architecture sentence states today's bound tools and adds the rule for a flag that becomes required;
`CONVENTIONS.md:149` is scoped to a bound command's flag; `wait`'s `--pid` is outside both because
`wait` declares `excluded` from MCP (`ARCHITECTURE.md:1240-1242`, whose excluded list also gains
`probe`, which declares `excluded`). Item 6's comment carries the fact only. Accepted when the two
sentences can be read together without either being false, and the comment contradicts neither.

**11. `ARCHITECTURE.md:32` (I9) says no tool but `build` performs inference.** `score` is bound as a
detached tool (`cmd/score.go:703`) and its run asks the scoring model. The invariant says what its own
next clause implies: no tool performs inference in the call, and `build` and `score` are the detached
subcommands. Accepted when I9 reads true against the bound tool table.

**12. `ARCHITECTURE.md:24` (I1)'s parenthetical names only the build.** The self-exec owner is
`cmd/mcp.go`, which now starts a detached build and a detached score run. The parenthetical names
both; I1's assertable content is unchanged. Accepted when the parenthetical matches what `cmd/mcp.go`
execs.

**13. `INTEGRATING_KBASE_VIA_MCP.md:130-132` says inference stays with `build`.** `score` performs
inference in its detached run. The sentence names both. Accepted when no sentence in the guide says
inference is `build`'s alone.

**14. `ARCHITECTURE.md:729-732` still describes the probe as one chat call.** It says "One
`model.Chat` call" and "`probeEndpoint` → `clients.get` → `model.Chat`". A decision selection now calls
`Decision` on the surface its entry declares; the chat selections keep `model.Chat`. Accepted when
the description covers both paths and the document's zero/null fields.

**15. `SPEC.md:455`'s `probe` row does not say what `reply` carries on a decision surface.** It is the
reply text on a chat surface; on a decision surface it is the answer's probability, chosen letter or
type. Accepted when the row states that, without naming a key the probe does not emit.

**16. `ARCHITECTURE.md:713-715`'s call-path list gives the probe one chat ask.** A `--decision` ask is a
`model.Decision` call; the list names the probe's chat ask and adds the decision path. Accepted when
the list accounts for both.

**17. `SPEC.md:1002-1007` says the probe's call is one chat request.** A decision selection is one
decision call on the declared surface, and its default check is a predicate rather than the one-word
check. Accepted when the paragraph covers both paths and keeps the timeout, re-issue, document and
state facts as they stand.

**18. `SPEC.md:1809` says `kbase probe` reaches an anthropic endpoint.** Its chat selections do; a
`--decision` selection refuses, as the letter asks do. Accepted when the 0.8.0 bullet excepts both.

**19. `ARCHITECTURE.md:737` says every model call is a chat request.** The decision calls are not; the
bullet that follows describes the chat protocol's two implementations. Accepted when the sentence
names chat calls, and the decision surfaces stay where they are described.

## Order and seats

One coder dispatch takes items 1–7; one tech-writer dispatch takes 8–19. They work in one tree on
disjoint files. Both dispatches are expert. Nothing is committed until every item lands, then one
commit carries the set; the gate runs once, before that commit.

The landing stages the files these items touch plus this plan. `CONVENTIONS.md` carries a pending
post-sprint rule that predates the plan; item 10's scope word lands in the same file, so the landing
stages it with that rule. `ACTIVE_PLAN.md` is not staged; the `MockClient` bullet at `:105` retires in
whatever commit carries that document.

## Gate

`just checkpoint`; `just test-integration-results-fixture` with the new `probe` leg; the unit test
item 4 adds, which replays the record against its stub. Evidence under `test-data/transient/`.

## Out of scope

- No change to `withMigration`; its order is correct as merged.
- No `kbase.required` code path: no bound tool requires a flag, so item 10 adds no mechanism.
- No return of the per-template charge-line example; item 2 replaces it with a structural check.
- No live-provider leg and nothing needing credentials.

## Post-sprint review

Once the landing is merged, an architect and a coder review the merged result against THESIS.md,
SPEC.md, ARCHITECTURE.md and CONVENTIONS.md, without this plan. Their subject is the changes against
those standards and whether the result is fit for its purpose — not how it was implemented — and their
brief includes reviewing the tests.

- **Review** — findings against the four documents, and tests that assert a document's claim rather
  than an implementation detail.
- **Remediation** — each finding lands in a follow-up commit or is declined with the reason recorded.
- **Burndown** — what remains, one fact per item, below.

### Remediation

Landed in the follow-up commit:

- `tools/probestub` joins the formatter's roots: `GOSRCDIRS` names `tools`, so `just fmt` and
  `fmt-check` reach the stub.
- `CONVENTIONS.md` §Testing states the helper rule the `model`/`model_test` split forced: an
  unexported name does not cross the package boundary, and a test-support package importing `p`
  cannot be imported by `p`'s in-package tests; the pair keeps one copy per package and states why.
- `SPEC.md` §12's 0.8.0 bullet says a Messages endpoint asked for a decision fails, not refuses: the
  verb reports `failed` with the client's error.
- `cmd/probe.go`'s decision-answer comment no longer claims a keyed lookup the response does not have.
- `ARCHITECTURE.md`'s model-importer inventory names `internal/model/modeltest`.
- The results-fixture recipe's dependency sentence names git and the Go toolchain, not git alone.

Declined:

- **The charge-line prose test.** `TestScoreSystemStatesTheChargeLineForm` and its `chargeLineRE` are
  removed. "No test asserts a prompt's prose" governs; the fragment's form is carried by the parser's
  refusal and the prompt-engineer review. Residual: no test fails when the form sentence is dropped.
- **The four fixtures' second home.** `testAPIKey`, `newTestEndpoint`, `chatRequest` and `statusServer`
  stay one copy per package. A shared home needs a test-support package importing `model`, which
  `model`'s in-package tests cannot import without an import cycle; the Testing rule states it.
- **`usage.reported` as `PromptTokens > 0`.** The convention's read for a surface that sends no usage
  block; an explicit field would touch five sites for one return. Note only.

### Burndown

- The probe's decision request contents are unasserted: both stubs discard the body, so a regression
  sending the chat default passes. One assertion over the request and the stub's captured body.
- `waitScore`'s two branches and its `interruptedScoreDocument` fallback are untested.
- `probeDecisionReply`'s letter and type branches are untested; only a predicate figure is.
- `internal/model/mockllm_test.go` still tests `modeltest.MockClient` from `internal/model`'s test
  binary; a `modeltest_test` package would exercise the same API without importing production `model`
  beside it.
- No test fails when the system fragment's charge-line form sentence is dropped.
- A move has no stated form for a dispatch: `internal/model/mockllm.go` was removed with
  `command rm`, because `git rm` stages and the tool set carries no delete verb.
- `ARCHITECTURE.md:1088`'s `kept:` parenthetical names `model.Chat` for the overview call and
  `model.Decision` for the letter asks only; scoring's asks and the probe's two calls are absent.
- The probe's routing is stated at `ARCHITECTURE.md:715-716` and again at `:731-732`; one should cite
  the other.
