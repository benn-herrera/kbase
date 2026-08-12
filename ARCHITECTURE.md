# Kbase Architecture Design

A single-purpose AI appliance application for the dissection of standard documentation into an AI-friendly knowledge base markdown graph.

This document is the authoritative design reference and the single source of
truth for design mechanisms and tuning constants. Externally observable
behaviors and contracts live in [SPEC.md](SPEC.md).

---

## 1. Overview

kbase is a **standalone application/appliance** that converts a human-targeted
documentation corpus — a Markdown doc set or LaTeX source (PDF via a preprocessing
adapter, later) — into an **agent-friendly knowledge base**: a navigable Markdown tree
(entry-point → domain index → subtopic index → leaf) with verbatim leaves, progressive
hierarchical summaries, and mechanically generated bidirectional navigation links.

It is a **doc agent, not a coding agent**: non-REPL, non-interactive, specialized on
this one transformation. It runs a fixed multi-stage pipeline, fans out parallel
workers to accelerate coverage of large corpora, and exits. Runtime requirements are
deliberately minimal: **an API URL and key** pointing at gemma-4-family models (local
or cloud — cloud gemma-4 service is cheap and fast).

**Why it exists (both true at once):**

- **Personal utility, short proof horizon.** First target use: interrogating large tech
  doc sets (e.g. Roblox `creator-docs`) through an agent instead of slogging a tech
  writer's ordering. Codeable and testable to a satisfactory workability answer within
  ~a week — in contrast to Personant, whose living-with validation phase is
  irreducibly months long.
- **Platform reconnaissance.** The appliance is a fast-feedback live-fire trial of the
  same platform bets Personant makes on a slow clock: gemma-4 family tiering,
  narrow-prompt discipline, family-tuned agent definitions with evals,
  propose-and-verify seams. Miscalibrations surface here in days, not in month three
  of living-with.

**Validation targets (in order):**

1. **Rojo v7 docs** (`rojo-rbx/rojo.space`, `docs/`, ~13k tokens) — pipeline shakedown
   on a corpus small enough to check by hand. Note: below the complexity gate where a
   KB pays for itself (the whole set fits in any context window); its role is tooling
   shakedown and control condition, not proof of KB value.
2. **Roblox `creator-docs` prose domains** (`content/en-us/`, CC-BY-4.0, millions of
   tokens) — the real target, decisively past the complexity gate. Start with
   role-relevant domains (`luau`, `scripting`, `projects`, `studio`); **skip
   `reference/`** (auto-generated API reference — see the annex convention, SPEC.md §3).

**Non-goals (deliberate, not deferred):**

- **No claim graph.** This is tech-doc dissection, not paper/proof dissection. No
  claim/experiment/support nodes, no confidence/solidity, no applied-mathematician
  role. What remains from the kb_tools lineage: topography graph, verbatim leaves,
  link integrity, drift-gated derived state.
- **No REPL / interactive UI.** Edge-and-corner cases needing collaborative human
  attention are served by the escape hatch (§3), not by an in-app experience.
- **No business plan.** Evaluated and parked; this is a personal tool with, at most,
  an OSS-release option later. That option is why the open-by-inspection posture (§3)
  is chosen deliberately.

---

## 2. Invariants

These must hold regardless of implementation choices. Violating any invariant is a
blocking defect.

**Build and Deployment**

1. **Single self-contained binary, embedded prompts.** No external runtime
   dependencies or shipped data files.

---

## 3. Design principles and decisions

The house thesis: **deterministic code as canonical state holder; LLM in narrow, bounded
judgment roles; the human at the edges.**

- **Cheap simplicity is a core principle, enforced against feature pressure.**
  Config is a provider URL + key. Single model family. No out-of-family machinery in
  the appliance, even for honesty checks — those move to the artifact (see BYOM eval,
  SPEC.md §5). When a proposed addition breaks this, the addition loses.

- **Specified, not autonomous.** A fixed pipeline with a termination condition — not
  an agent free-roaming the corpus. Reproducibility requires same process → same
  output. Terminology guard (ruled 2026-08-09): in this codebase, "agent" names a
  **Go-side construct** — a role with an embedded definition, slot-built context,
  and one-shot model calls — never an LLM-driven tool-calling loop. Each pipeline
  step is mechanical where possible; LLM execution is reserved for what cannot
  practically be done any other way.

- **Propose-and-verify at every model seam.** The model never mutates canonical bytes.
  It emits *data with a mechanically checkable post-condition* (a cut list that must
  tile the document; summary-review flags; a taxonomy skeleton). Deterministic code
  validates and executes.

- **Monotone safety: mechanical fallbacks are always valid.** Every model-refined
  artifact has a deterministic fallback that was already acceptable (heuristic cuts,
  un-reviewed summaries flagged as such). Model failure degrades *quality*, never
  *correctness*. Reject-and-retry once, then fall back and log.

- **Refuse-and-split, never truncate.** A batch pipeline needs no eviction; it needs
  loud errors that push oversized units back to the stage that can fix them (the
  skeleton). Silent truncation is how a verbatim-leaf discipline dies unnoticed.

- **Verbatim leaves; summaries route, leaves answer.** Leaves are faithful
  translations of source — no paraphrase, no audience simplification. Summaries exist
  only to route navigation; they are **non-load-bearing for truth**. This is stated
  explicitly in every generated KB's entry-point AGENTS.md: agents must answer from
  leaf text, never from index summaries. Consequence: a bad summary misroutes (costs
  tokens, recoverable via up-links) but cannot produce a confidently wrong answer.

- **Source-explicit cross-references only.** Cross-refs come from the source's own
  links and see-alsos. An inferred "related topic" edge is a hallucinated edge with
  better manners. Bidirectional tree-navigation links are purely mechanical (§4).

- **Model-family-as-platform, one family: gemma-4.** Agent/prompt definitions are
  tuned to gemma-4 tiers and validated by evals against known corpora. Two disciplines
  by failure mode:
  - *Automated pipeline*: family-pinned, embedded, evaled. Infrastructure prompts
    drift silently across families; they get one tuned family and a test suite.
  - *Interactive edge-case repair*: freely user-adapted (BYOM) because a human is in
    the loop — the user is the eval.

- **Escape hatch over babyfood UI.** For the weird 5%, the documented process is:
  "use your favorite agent app; here are the agent definitions the internal AI uses."
  Docs recommend asking a stronger model to write a modified copy suited to its
  capabilities. This leverages the user's ever-improving harness (someone else's R&D
  budget) instead of hand-milling an interactive experience at tremendous developer
  expense — used 5% of the time and perpetually inferior to the tools users already
  live in.

- **Immutable embedded prompts; write-modified-copy only.** The primary system
  agent definitions live in the compiled binary as a code resource. A CLI option dumps
  copies to local storage for adaptation. The app's own definitions cannot be
  modified — the appliance version fully determines pipeline behavior.

- **Provenance and reproducibility as artifact properties.** A KB's provenance stamp
  is `app version + resolved model IDs + source identity (commit/hash)`. That tuple
  reproduces the artifact: embedded prompts are implied by app version; chunking is
  byte-reproducible (source hash + verified cut list); derived state is regenerable
  and drift-gated.

- **Artifact self-description.** Each generated KB ships a `.agents/` directory:
  docent (navigate me) and maintainer (repair me) definitions, the adaptation-guidance
  note, the routing-eval question set, and the entry-point AGENTS.md contract. Any
  agent that picks up the artifact finds its own operating manual inside.

- **BYOM consumption.** The KB is *created* with gemma-4 but consumed by whatever
  model the user brings. This is safe because the KB is **content, not prompts** —
  plain descriptive text and links targeting general language comprehension, the
  surface where model families converge. The only steering surface is the AGENTS.md
  contract, kept to simple family-portable imperatives.

- **Validation-latency as a design value.** Prefer mechanisms whose correctness is
  checkable immediately and mechanically (tiling checks, link integrity, routing
  evals) over those requiring longitudinal observation.

---

## 4. Pipeline (fixed stage order; map-reduce shape)

| # | Stage | Actor | Notes |
|---|---|---|---|
| 1 | **Ingest** | deterministic adapters | Markdown-set and LaTeX adapters; PDF is a *preprocessing-adapter slot* (docling/marker-class external tools), never a native capability — PDF extraction is a tar pit, fenced off. MDX is **ignored** (ruled 2026-08-09): its content is build-time-produced, not in the source bytes, so it breaks source-hash provenance; revisit as strip-and-scan only if a corpus that matters shows real cost. |
| 2 | **Survey** | deterministic (mostly) | Per-file structural inventory: heading tree, section token sizes, link graph, first-paragraph gists, and the legal cut candidates stage 4 clamps the model to (§5). For Markdown this is nearly all mechanical. Produces the compact artifact the taxonomy stage consumes. If the survey itself would overflow, roll up per-domain surveys first (survey-of-surveys). |
| 3 | **Taxonomy design** | gemma-4-31B | Consumes the survey, never raw source. Serial (needs whole-corpus view). Emits the document skeleton: hierarchy, leaf assignments, acceptance criteria. Must guarantee no leaf's source span exceeds the per-call budget (sizes are in the survey) — oversized sections are split at design time, not discovered mid-distillation. |
| 4 | **Dissection** | mechanical + 26B-A4B + mechanical | See "boundary refinement" (§5). The model never emits raw offsets: it chooses among adapter-enumerated legal cut positions (ruled 2026-08-09), so structurally invalid cuts are unrepresentable. Output: verified cut list → deterministic dissector slices the immutable source by byte offsets. |
| 5 | **Distillation** | mechanical (Markdown); 26B-A4B translation for non-Markdown formats | **Markdown leaves are mechanical** (ruled 2026-08-09): the leaf body is the verified byte slice plus mechanically generated wrapping — the model never writes leaf text, so leaf-fidelity deviation is impossible rather than checked. Model translation exists only where a format adapter requires it (LaTeX→Markdown, later); there it is parallel fan-out, one domain per worker. The historical "~90% of tokens" estimate applied to translation and largely evaporates on the Markdown path. |
| 6 | **Hierarchical summaries** | gemma-4-31B, bottom-up | Each index level consumes its children's *summaries*, never bodies — bounded by fan-out × summary cap, both controlled by the taxonomy. Summaries are the navigation surface; quality binds hardest here, hence the heavy tier. |
| 7 | **Review** | gemma-4-26B-A4B | Structured rubric, not "is this good?" (see §6). Output is **flags for regeneration** (by the 31B), never edits — a weaker model's fingerprints stay off the best text. |
| 8 | **Link generation** | deterministic | Bidirectional tree-nav links (up-links, index children) emitted from the skeleton by template. Dead links impossible by construction; the link checker demotes to regression tripwire. |
| 9 | **Refresh / verify gates** | deterministic (kb_tools lineage) | Derived-state regeneration + integrity gates; idempotent; drift-gated. |

**Format seam.** The survey artifact is the source-format independence
boundary. Format-specific code lives only in an adapter package
(`internal/survey/markdown` today, `internal/survey/latex` later); the adapter
produces the neutral artifact, and every stage downstream consumes artifact
fields and byte offsets into the custody bytes — never a parser node. The
neutral package holds the artifact types, the corpus roll-up, and the tiling
and custody checks it runs over every adapter's output; the ingest walk is
neutral too, taking the adapter's document-extension set as a parameter. The
boundary is enforced by an import-policy test that parses every file in the
module (`internal/survey/importpolicy_test.go`): a parser library outside its
adapter fails the build, naming the file, the import, and the rule. Adding a
format is adding a row. No adapter *interface* exists yet — with one
implementation there is nothing to compare against, so the abstraction is
deferred to the second adapter, where it can be derived rather than guessed
(ruled 2026-08-09).

---

## 5. Boundary refinement (stage 4 detail)

Implemented in `internal/dissect` (format-neutral: it reads the survey
artifact's neutral types and the custody bytes, never a parser).

1. **Mechanical splitter** (`dissect.Split`) proposes cuts at heuristic section
   breaks → offset+length list (`[]survey.Range`). It prefers the strongest
   structure available before the budget (`survey.CutKind.Rank`: heading, then
   fence, then paragraph) and fills toward the budget among equals. Pre-merges
   below-minimum fragments so the model only adjudicates real boundaries. A
   span whose candidates cannot cut it under the budget is refused by name
   (`dissect.StarvedError`) and goes back to the stage that sized it —
   refuse-and-split, never truncate. **Its output always passes verification**,
   which is a property test over both synthetic adversarial spans and every
   section of the pinned corpus.
2. **Model refinement pass** (26B-A4B, `dissect.Refiner`): a serial FOLD — one
   call per boundary in ONE domain stream, so a worker walks them in order and
   each window load overwrites the previous (O(1) context, O(n) calls). The
   scan judges each boundary against the cut list **as it stands now**: an
   accepted choice updates the working list, and the next boundary's window,
   menu and verification all derive from that updated list. The stage's output
   is therefore ONE artifact — the composed cut list, whole-list-`Verify`d
   before it is written — and the per-boundary calls produce none
   (`pipeline.Task.CallOnly`), which is what makes the fold's resume
   stage-granular (§12). Each boundary's overlap window (`dissect.Window`, from
   `dissect.Windows`) renders into the Content slot and its numbered candidate
   menu into reference buffer B, capped at `dissect.menuCap` entries around the
   incumbent, which is marked `(current)` so confirming the boundary — the most
   common correct answer at a refinement seam — is a choice the model can
   deliberately make.
   A boundary's question does not exist until its predecessor is adjudicated,
   so the per-call input is BUILT when the worker reaches the unit
   (`pipeline.InputBuilder`, §12) rather than when the stage was described.
   The model writes no text and **is shown no raw byte offset anywhere**: not
   in the menu, not in the status lines, not in reference buffer A, and not in
   the corrective note a retry carries (`dissect.RejectionError.Note`, the
   model-facing rendering, against `Error`'s operator-facing one). It answers
   with a menu NUMBER and the answer must BE that number —
   `dissect.parseChoice` rejects a number embedded in a sentence, because the
   prompt necessarily contains other numbers (menu positions, "boundary 3 of
   12") and a mis-mapped choice is a legal menu entry that then verifies. A
   visible retry is cheaper than a silently wrong-but-valid cut. Candidates are
   enumerated at survey time by the format adapter and carried in the artifact
   as `survey.CutCandidate{Offset, Kind}` (ruled 2026-08-10, schema
   `kbase.survey/2`), so refinement's whole input is reproducible from stamped
   artifacts. Authority is clamped twice: to the overlap window ∩ the menu, and
   to the enumerated set — bisecting a heading is unrepresentable, not merely
   detectable. The composed cut list is stamped with the stage's **parameter
   digest** (span + budget + the mechanical cut list): the list is computed
   in-process rather than read from an upstream artifact, so without it a
   re-plan's `cuts/cutlist.txt` and this one's are indistinguishable to the
   resume scan (§12). The digest is taken over the MECHANICAL list, which the
   fold never touches — it identifies the questions the stage asked, and the
   working list is the answers. Optional per-boundary confidence emission; low-confidence
   boundaries escalate (31B look or human).
3. **Verification** (`dissect.Verify` — deterministic, all cheap,
   format-neutral): exact tiling — monotonic offsets, no gaps/overlaps, sum =
   document length; every deviation within clamp; candidate-set membership;
   minimum section size respected; and the **whitespace-adjacency tripwire**
   (`survey.WhitespaceAdjacent`, the same function that admitted the candidate
   in the first place): at every cut, at least one adjacent byte must be
   whitespace. Both-sides-non-whitespace cannot result from model misjudgment
   under candidate constraint — it means OUR offset pipeline is broken
   (rebasing drift, wrong buffer), so it is a loud-abort defect, not a retry.
   Orthogonal nets: the membership check inspects structure, the tripwire
   inspects raw bytes; a bug must thread both, and membership is checked FIRST
   so only an offset the adapter really enumerated can reach the tripwire. The
   two outcomes are distinct types: `dissect.RejectionError` (retry once, then
   the baseline) and `dissect.OffsetDefectError` (never retried — it reaches
   the runner wrapped in `pipeline.ErrVerifierDefect`, which aborts the
   worker). The split between them is exhaustive **by construction, not by
   enumeration**: `RejectionError` is the only class a model's answer can be
   responsible for, so the seam wraps everything else — the tripwire, a window
   count that does not match the boundaries, a span that is not a range of the
   source, any plain error a later check adds — as our defect. The two
   mistakes do not cost the same: a defect routed to the model burns a retry
   and then emits a degraded unit from a broken derivation, while a model
   failure routed to the defect path stops the job loudly. The minimum applies
   to CHOICES: a one-section list is not refused for the size of a span nobody
   chose. An empty span is refused before any of this (`dissect.checkSpan`) —
   a span with no bytes is not a span to cut, and the refusal names the caller
   rather than the one-section list it would otherwise have produced. Pass →
   dissect. The mechanical list is always valid — `dissect.NewRefiner` verifies
   it before the stage runs, since it is what every failure falls back to.

   **Verification against frozen neighbours does not compose — which is why
   the scan is a fold** (ruled 2026-08-11). Judge each boundary against the
   ORIGINAL positions of its neighbours and the list the stage stands on is the
   composition of *n* independent moves. Tiling survives that (windows cannot
   overlap: each reaches `overlapFraction` and 2 × 0.2 < 1), and so do
   membership and the tripwire. The **minimum does not**: a 70-token section
   whose left boundary moves 6 tokens in and whose right boundary then moves 6
   tokens back composes to 58, under the floor, with each move having verified
   on its own. Judged against CURRENT state the second move is simply refused,
   and the minimum is enforced exactly: the slack is real, finite, and
   allocated first-adjudicated-first-served — no worst-case pre-rationing, no
   freedom halved. A refused move is an ordinary rejection (retry once with the
   reason, then the boundary stands where it is), so the cost of running out of
   slack is quality, never correctness. Before the composed list is written it
   goes through `Verify` once more, whole. That check is unreachable by
   construction — every accepted move verified the same list — so its failure
   is a `pipeline.ErrVerifierDefect` and not a rejection: nothing the model
   answered could produce it. It stays because the composing step is exactly
   where this would otherwise be discovered rather than remembered.
4. **Dissector** (`dissect.Dissect`, deterministic) slices the immutable source
   by verified offsets. Source is never sliced-and-retyped; the leaves are
   views into the custody bytes and the cut list is a derived overlay
   (content-anchored spans — same discipline as the contract-analysis design).

---

## 6. Summary review (stage 7 detail)

- Two failure directions, treated by different methods:
  - *Hallucination direction* (summary claims X, children don't support it): tractable
    for a small model — decompose the summary into atomic claims, entail each against
    the children's text. Local, well-posed checks.
  - *Omission direction* (load-bearing child point missing): **not** easier than
    generation — noticing absence requires knowing what mattered. Converted to a
    checklist: the reviewer receives the children's own gists and must mark each as
    represented or deliberately excluded.
- Framing: **find the discrepancy**, never approve/rate (small-model judges
  rubber-stamp under approval framing).
- Residual risk acknowledged: summary validation has no complete mitigation. The
  risk posture is containment (summaries can waste tokens, never terminally lie —
  routes/answers split) + routing eval + n=1 spot-checks in personal use.

---

## 7. Context management

Careful, not complex — no stage ever legitimately needs the corpus in a window:

- One **per-call context builder** with named component budgets; component sizes known
  from ingest (the survey collects token counts).
- **Ceiling and per-call target** per the constants table (§9): the ceiling sits in
  the reliable zone of gemma-4's window; the target keeps every call far below it —
  attention quality, especially on small models, degrades well before the window
  fills, and decomposition's whole point is that nobody runs near the ceiling.
- Overflow behavior: **refuse-and-split** (loud), pushed back to the skeleton. Never
  truncate, never evict.

### Per-call slot layout (stability-ordered)

The builder assembles every call from an ordered slot stack, most stable content
first:

| # | Slot | Stability | Content |
|---|---|---|---|
| 1 | System frame | job-constant | process framing + job-stable specifics, generated **once at job start** from an embedded deterministic template; universal static text first, job-interpolated lines last. Churn prevented by construction. |
| 2 | Agent definition | stage-constant | the stage's embedded definition, including its `## CRITICAL` section |
| 3 | Task definition | task-constant | what this agent is working on; formatted-in specifics chosen for stability |
| 4 | Reference buffer A | multi-call | orchestrator-curated material stable across several calls (e.g. cross-file listings); flushed on stage/section transitions |
| 5 | Task status | per-call | checklist/status lines ordered most→least stable, so the fastest-churning line (e.g. current file) is last |
| 6 | Content buffer | per-call | the source span / child summaries / prior output under work |
| 7 | Reference buffer B | per-call | orchestrator-curated transient material specific to the current content |
| 8 | Reminder trailer | per-call | `REMINDER:`-wrapped render of the `## CRITICAL` block + per-call acceptance criteria (see below) |

The ordering is simultaneously:

- **Cache-optimal.** Prefix caching pays for byte-identical leading tokens, and any
  mutation invalidates everything after it — so descending stability maximizes the
  reusable prefix across a stage's call fan-out. The same principle applies *within*
  slot 5: when only its last line changes, the cache holds through everything above.
  This is why the multi-call buffer outranks the per-call status block: a status line
  churning every call must not re-prefill a cross-file listing, and under stage-5
  fan-out the buffer is shared across workers while the status block is per-worker.
- **Attention-safe.** Long-context degradation is U-shaped: primacy and recency
  positions are well-attended (attention sinks + causal exposure at the front,
  positional locality at the back); the middle sags. Slots 1–5 sit in the primacy
  zone, the trailer in the recency zone; only the middle buffers hold at-risk
  positions, and nothing load-bearing lives there without a trailer restatement. The
  status block pays for slot 4's position by sitting a buffer deeper — acceptable
  because status lines are orienting, not load-bearing: the per-call facts that bind
  (the span, the acceptance criteria) live in slot 6 and are restated in the trailer.

The 4/5 ordering is **provisional**, pending the measurement §7 already names as
ground truth: run the shakedown corpus both ways and read `cached_tokens` (plus
per-call telemetry) rather than reasoning further about attention curves. It also
**presumes reference buffer A is flushed at section/stage transitions** as the table
says — that policy is the orchestrator's to enforce (phase matrix), and a buffer that
churned per call would move the frontier *up*, making the ordering a loss rather than
a win.

All slot content is deterministic from embedded templates + job inputs + task
state — the model never writes into its own context (buffers are
orchestrator-curated), and the provenance tuple reproduces exact prompt bytes.
Gemma's chat template has no true system role, so the stack renders through one
deterministic template into a single user turn; slot boundaries are a builder
concept, not wire messages.

### CRITICAL / REMINDER mechanism

- An agent definition may carry a `## CRITICAL` section: the few constraints whose
  violation is the stage's characteristic failure (e.g. verbatim-no-summarization
  for distillation). The section is named for its *content*, so the label is true at
  every position it renders.
- The builder renders it **twice**: as-authored in slot 2 (primacy) and
  `REMINDER:`-wrapped as slot 8 (recency) — stated up front, repeated last, the two
  well-attended ends. Dual-render is a builder flag so the eval harness can A/B it
  per stage/tier.
- Slot 8 additionally carries the per-call acceptance criteria the taxonomy skeleton
  already emits (tiling ranges, budgets) — data injection by the builder, never a
  model judgment call, under its own reserved share of the cap.
- The trailer is **hard-capped** (§9) and the cap is enforced loudly, never by
  truncation: a unit test over the embedded definitions fails the build at dev time,
  and the builder refuses an over-cap trailer at runtime (which also covers
  user-adapted definition copies). The cap is **split into reserved shares** —
  authored section and injected criteria — and each half is enforced against its own
  share, never against the sum. That is what makes the dev-time gate a *sufficient*
  condition for the runtime one: a definition that passes cannot then be pushed over
  by conforming criteria, which would be an unfixable failure (the definition ships
  embedded and immutable, and splitting content does not shrink a trailer). The two
  overruns are distinct error types, so a caller can tell the failure it caused from
  the one it inherited.
- Why it exists: retrieval survives distance far better than *sustained adherence*
  during long generations — constraint drift late in a long output is the
  characteristic small-tier failure, and it compounds across thousands of leaf calls
  into review flags and regeneration cost. At well under 1% of a target call, the
  restatement is insurance priced at noise.

### Phase-gated churn enforcement

Slot stability is enforced at three layers, each covering what the previous one
cannot reach:

1. **Type system (builder).** Slots 1–3 are constructor-set on an immutable
   per-stage context; slot 8 is derived and never directly settable. Mutating a
   stable slot mid-loop is not a runtime error — it does not compile.
2. **Phase matrix (orchestrator).** The orchestrator holds a phase enum
   (job-setup → stage-setup → call-loop → section-transition → …) and an
   allowed-operations matrix **as data**: every context-affecting operation
   (rebuild a stage context, flush reference buffer A, …) checks the matrix, and a
   disallowed operation is a loud defect. This closes the gap types cannot reach:
   constructing a *fresh* stage context with different bytes mid-task is
   type-legal but a churn bug — the risk is identity across instances, not
   mutation of one.
3. **Churn tripwire (per call, always on).** The builder emits per-slot byte
   hashes with every built call; slots the current phase declares stable must
   hash identically to the previous call, and a mismatch is a loud refusal —
   the drift-gate discipline applied to prompt bytes. This is the production
   form of the prefix-stability property; cheap (a few string hashes per call).

Single source of truth: the phase matrix that gates operations also derives the
expected-stability frontier asserted by the builder's sequence tests — tests and
runtime enforce the same table. End-to-end verification signal: providers'
`cached_tokens` usage accounting gives wire-level ground truth that churn
prevention is holding against a real prefix cache.

---

## 8. Token counting

- Operative counter is a **calibrated chars-per-token heuristic** (single
  gemma-tokenizer-specific constant). No cgo SentencePiece binding; no hard
  dependency on a `/tokenize` endpoint — tokenize is not part of the
  OpenAI-compatible core (vLLM/llama.cpp expose it; Ollama and most cloud hosts
  don't), so requiring it would narrow provider support against the
  cheap-simplicity principle (§3).
- Rationale: every consumer of token counts (survey section sizes, taxonomy leaf
  budgets, per-call context budgets) is a threshold check with 2–3× headroom
  between per-call target and ceiling (§9). Refuse-and-split makes correctness
  independent of estimator error — an undercount surfaces as one loud re-split, an
  efficiency blip. Exact counting buys nothing the margins can distinguish from
  perfect.
- Calibration: if a tokenize endpoint is detected at configure time, one
  calibration call checks/refines the ratio; thereafter the constant is refined
  from the `usage` token counts providers return on every real pipeline call —
  free ground truth as a byproduct. Calibration source is stamped into the
  provenance receipt (SPEC.md §7).

---

## 9. Constants (single source; initial values, to be calibrated)

| Constant | Value | Note |
|---|---|---|
| Context ceiling | ~180K tokens | reliable zone of gemma-4's ~250K window |
| Per-call target | 60–80K tokens | quality/cost operating point; nobody runs near ceiling |
| Boundary overlap | 20% of each neighbor, capped at 1K tokens/side | `dissect.overlapFraction`/`overlapCapTokens`; the window the model sees AND the clamp bound. Exact % still TBD at calibration |
| Min section size | 64 tokens | `dissect.minTokens`; pre-merged mechanically before refinement. "> overlap into a minimum section" holds by construction, since the overlap is a fraction under 100%. TBD at calibration |
| Boundary menu cap | 7 entries | `dissect.menuCap`: `dissect.menuSide` (3) candidates before the mechanical cut, the cut itself marked `(current)`, 3 after; a short side contributes what it has and lends nothing to the other. 7±2 is the honest ceiling for a choice a small tier reasons over, and the cap is what makes this seam's prompt bounded by construction (§12) |
| Retry policy | 1 retry, then mechanical fallback + log | monotone safety |
| Effort declaration | per definition; boundary refinement: thinking **off** | `model.Effort`, positional at the registration site (`cmd.devRefineEffort` → `dissect.NewRefiner`) and on `model.DefaultRequest`; `pipeline.Role` refuses an undeclared one. Both `chat_template_kwargs` keys are always sent, false included (§12). Non-pipeline calls (catalogue probes) use `model.UtilityEffort`, thinking off. Refinement's value is measured, not assumed: the 2026-08-12 A/B produced a byte-identical cut list for 4 completion tokens against 20,924 (ROADMAP) |
| Chars-per-token | 4.0 (provisional) | gemma-4-specific constant (`tokens.DefaultCharsPerToken`); heuristic counter (§8), calibrated then usage-refined |
| Response token cap | 16K (provisional) | per-call MaxTokens default (`model.DefaultMaxTokens`); revisit at calibration |
| Stream idle timeout | 2 min | max gap between stream reads, SSE keepalives count (`model.streamIdleTimeout`) |
| Response-header timeout | 2 min | handshake guard on the transport (`model.responseHeaderTimeout`) |
| Error-body echo cap | 8 KB | non-2xx response echo bound (`model.errorBodyLimit`) |
| Gist word cap | 40 words | survey routing hints and titles (`survey.WordCap`, post-seam export); word-capped, never mid-word |
| Transport retries | 3 attempts, 500ms base doubling | 429 retried, other 4xx not; ctx cancel never (`pipeline.transportAttempts`/`transportBackoffBase`) |
| Semantic attempts | 2 | initial + one informed retry (`pipeline.semanticAttempts`) |
| Corrective-note bound | 12 words + 4-word prefix | the retry's failure-reason note (`pipeline.correctiveNoteWords`) |
| Worker pool default | 4 | domain-stream workers per stage (`pipeline.DefaultWorkers`) |
| Job lock filename | `job.lock` | `pipeline.LockFileName`; O_EXCL, refuse on contention, never auto-broken |
| Artifact stamp | `<artifact>.stamp.json`, schema 1 | `pipeline.StampSuffix` sidecar; schema mismatch ⇒ Invalid |
| Job-dir modes | 0600 / 0700 | `pipeline.artifactFileMode`/`artifactDirMode`; matches log + key-file posture |
| Max corpus bytes | 256 MB (provisional) | in-memory ingest ceiling (`ingest.maxCorpusBytes`); loud refusal, no override flag |
| CRITICAL section cap | 100 words total (provisional) | whole slot-8 trailer (`prompt.criticalWordCap`); the budget the two reserved shares below are cut from, never itself enforced |
| — authored `## CRITICAL` share | 60 words (derived) | `prompt.authoredCriticalCap` = cap − criteria share; enforced by the dev-time definition test and repeated by the builder for user-adapted copies |
| — injected criteria share | 40 words (provisional) | `prompt.maxCriteriaWords`; the builder's per-call acceptance criteria. Reserving it is what makes a dev-time pass guarantee a runtime pass (§7) |

---

## 10. Model tier mapping

| Tier | Stages | Why |
|---|---|---|
| gemma-4-31B (dense) | taxonomy design; hierarchical summaries; regeneration on review flags; low-confidence boundary escalation | serial, judgment-heavy, small token share; navigation-surface quality binds here |
| gemma-4-26B-A4B (MoE) | cut-list refinement; distillation volume; review | parallel, translation/checklist-shaped, ~90% of tokens |

Per-stage model config is supported from day one (it's just API refs) — keeps a
"borrow a frontier model for taxonomy only" escape hatch open if 31B hierarchies
disappoint. Default remains pure gemma-4.

---

## 11. Code ancestry (decided)

- **True ancestor: `kb_tools`** (stdlib-only Python deterministic spine — refresh,
  verify, link checking, id/schema discipline). Already the right shape.
- **Personant contributes philosophy, not modules.** Its mass is all in being a
  resident organism (spine, recall, working-set eviction, turn loop, crash recovery,
  autogit custody); the appliance is a compiler run — starts, transforms, verifies,
  exits. Concepts that carry: per-call context composition/budgets, model tiering by
  stage, single-source constants, drift-gated derived state. All reimplementable in
  less code than the imports would cost. Even the provider client is ~200 fresh lines,
  not a runtime's abstraction.
- **Decided (2026-08-06): pure Go, single static binary.** Language stance is "why
  not Go?" — and the appliance permits **no ad-hoc Python execution**. The kb_tools
  gates (tiling, link integrity, id/schema) are small and port cleanly; no shell-out,
  no Python runtime dependency. Deterministic pieces are internal Go functionality;
  AI-driven steps are the clearly delineated pipeline stages (§4).

---

## 12. Execution and resume (orchestration design, ruled 2026-08-09)

The orchestration layer (`internal/pipeline`) runs the fixed pipeline. "Agent"
here is the §3 Go-side construct: a role = embedded definition + tier +
verifier (+ mechanical baseline where one exists), executing one-shot calls.

### Phase matrix (single source)

Worker execution moves through phases: `JobSetup → StageSetup → CallLoop ⇄
SectionTransition → StageTeardown`. One data table maps each phase to (a) its
allowed context-affecting operations and (b) its declared stability frontier
(a `prompt.Slot`). Consumers — all of them, by design: runtime op-gating (a
disallowed op is a loud defect), the per-call churn tripwire, the prompt
package's prefix/stability tests (which derive expected frontiers here,
closing §7's single-source claim), and crashpoint registration. `FlushRefA`
is legal only in `SectionTransition` — the enforcement that makes the §7
slot ordering safe.

### Workers and frontiers

A worker owns a **domain** and processes its leaves serially (domain-stream);
parent↔child channels only, no peer↔peer. Stability is enforced per §7's
three layers, concretized: all workers of a stage share ONE immutable
StageContext, so the stage-constant slots are identical across workers by
construction; each worker keeps its own previous-call hashes and frontier
(per-worker tripwire); a cheap per-call assert checks every call's
stage-constant slot hashes against canonical values — slots 2–3 against the
ones captured at stage setup, and **slot 1 against the job's**, rendered
once at job setup from the plan's single system frame. The slot-1 half is
what makes §7's job-constant claim enforced rather than merely intended: a
worker's previous-call hashes reset at every stream start, so its first call
of every stage claims nothing and the per-worker tripwire can never compare
slot 1 across a stage boundary. One job-level frame makes divergence
unrepresentable; the canonical catches what construction cannot see — a
builder change bleeding a per-call field into slots 1–3, which would render
identically at setup and differently on the wire.

### Call runner protocol

Every model call passes through one runner: **build → frozen-prompt
assertion → transport → mechanical validation → one informed retry → seam
resolution**.

- Build refusals (`ErrOverBudget`) are refuse-and-split — pushed back to the
  skeleton, never retried at the runner.
- **Every seam's inputs are bounded by construction** (ruled 2026-08-11).
  Each thing a stage renders into a prompt is capped at its source, or the
  source refuses and splits: the refinement window is a fraction capped per
  side, its menu is capped in entries (§9), and stage 7's gist checklists get
  the same treatment when they arrive. The consequence is the point —
  once every input to a call is bounded by a named constant, an over-budget
  build is not a runtime condition to be resilient to. It is a **defect
  class**: our own sizing arithmetic is wrong, and the only way to learn that
  is the hard failure. So `ErrOverBudget` keeps its current routing even at a
  refinement seam that has a valid baseline in hand — a graceful fallback
  there would silently paper over the one thing the constant table exists to
  make impossible. Bounding at the source is the work; the hard fail is what
  makes the bounding checkable.
- The frozen-prompt assertion (tripwire) checks OUR stability contract; a
  violation is a kbase defect: worker abort, never retry or fallback.
- Transport failures (timeout, 5xx, dropped stream) say nothing about output
  validity: bounded backoff retries, separately from semantic policy.
- Mechanical validation is the propose-and-verify seam concrete: the
  response is text claiming to be data; a deterministic verifier parses and
  checks the stage's post-condition (tiling, budgets, schema). Typed
  artifacts out; raw model text never escapes the runner. The verifier is
  given the UNIT as well as the response, because a post-condition can be
  per-unit — stage 4 checks a cut against that boundary's own candidate menu
  and clamp window — and the lookup table it selects from is built when the
  stage's work is described, so the Role stays stage-constant and shared.
  A verifier that concludes the failure is OURS rather than the model's says
  so by wrapping `ErrVerifierDefect` (§5's tripwire is the first case): that
  is neither retried nor fallen back, since both remedies trust the
  derivation just indicted — the worker aborts, like a frozen-prompt
  violation.
- The single semantic retry always carries the mechanical failure reason —
  a blind identical resend hopes temperature fixes it, which is not design
  (ruled).
- Seam resolution: **refinement seams** (model improves an already-valid
  mechanical baseline: cut refinement, review) fall back to the baseline,
  logged, marked degraded. **Essential-inference seams** (taxonomy,
  summaries, format translation) have no fallback by definition of why
  inference was chosen: the unit fails loudly, sibling units complete, and
  the job REFUSES EMISSION at assembly — "sorry, something rotted" beats
  "here's your invalid crap" (ruled). Resumable rerun redoes only failures.
- Accounting: per-call usage aggregation, failed units included — a unit
  that burned two semantic attempts and produced nothing is exactly the one
  a cost figure must not omit; `cached_tokens` logged as prefix-cache ground
  truth; dev-telemetry emission (config-gated, off by default) as structured
  records through the logging seam. The telemetry records emit at **info**
  while the default log level is warn, so `[dev] telemetry = true` needs
  `--log-level info` alongside it to show anything. It stays at info rather
  than being promoted: a diagnostic that pollutes the default channel is one
  everybody learns to ignore.

**Seams, asks, and declared effort** (ruled 2026-08-11). A *seam* is the
mechanical→inference junction, and its only classification is what happens
when inference fails: a refinement seam has a valid mechanical baseline to
stand on, an essential one does not. That says nothing about how hard the
model should work, so effort — thinking today, temperature next, one
`model.Effort` value — attaches to the **definition**: one value per exact
ask, declared where the definition is registered and threaded from there to
the request. Not to the seam, and not to the stage. Today each seam happens
to pose exactly one ask, so "per definition" and "per seam" would name the
same values — which is precisely why the attachment is fixed by construction
now (`model.Effort` is positional on both `dissect.NewRefiner` and
`model.DefaultRequest`, and `pipeline.Role` refuses an undeclared value)
rather than left to a terminology that a second ask at one seam would
silently break. Both `chat_template_kwargs` keys are always sent, false
included: an omitted key leaves the served chat template's default deciding,
which is not a declaration. Calls that are not a definition's ask —
catalogue probes and the like — say so with `model.UtilityEffort`.

### Chain-stamped artifacts and resume

Resume is **structural, not temporal** — no journal, no cursor, no place to
lose. Three properties: (1) every output unit is written atomically
(temp+rename+fsync) — mid-write kill states do not exist; (2) every stage
artifact carries a stamp: app version + input hashes (source identity +
upstream artifact hashes) + output hash — validity is decidable by
inspection with no knowledge of how the prior run died; (3) the worklist is
stateless — resume scans outputs and re-derives it.

Stage artifacts form a dependency chain (survey → skeleton → cuts → leaves →
summaries → links). The chain is **described lazily, one stage at a time**:
a stage says what units it owes when the walk REACHES it, not at job setup.
That is not an optimization — it is forced. The skeleton stage 3 emits is
what defines stage 4's leaf units, and stage 4's verified cut list defines
stage 5's, so no job-setup description of those stages could exist. A
resolver therefore reads artifacts that earlier stages have already been
proven to hold, and its failure stops the job rather than resolving to an
empty stage (an empty stage and a complete one are indistinguishable). Two
consequences: a description defect in a later stage is refused when that
stage is described rather than at job setup, and a job that stops before its
chain is fully described is never emit-ready, because it never learned what
the rest of the chain owed. A unit's declared upstreams may name an artifact
**this chain does not produce** — a re-plan legitimately consumes the
skeleton a previous planning epoch left behind — but only a *proven* one:
the store is asked for its stamp, and an unstamped upstream is structural
incoherence (nothing this chain runs would ever produce it), not a unit to
redo. A unit may never name a sibling from its own stage: a stage's units
run concurrently, so "earlier in the same stage" is not an ordering.

Resume finds the **deepest provably-valid prefix**: first broken link in the
chain is the restart point; everything downstream is invalid by definition
(its inputs changed), everything upstream stands on proof. A stage past the
boundary is simply never described — it is downstream of the frontier by
definition, which is exactly what makes lazy description safe. Within the first incomplete stage, units are individually verdicted
`Valid | Absent | Invalid` — reuse requires affirmative proof; ANY doubt
(unparseable stamp, hash mismatch, version skew) is redone, priced in
tokens; structural incoherence refuses the whole resume with `--fresh`
guidance. This is the kb_tools drift-gate discipline applied to the
pipeline's own execution.

Resume is an **optimization, never load-bearing**: `--fresh` ignores all
prior outputs unconditionally and is always sufficient. And at no tier does
any path emit unverified material — assembly-time verification re-checks
everything regardless of provenance (two independent nets).

**Call-time task inputs.** A task's prompt input may be a value or a builder
(`pipeline.InputBuilder`); a builder runs when the worker REACHES the unit.
Every stage whose questions are known up front passes `ConstInput` and is
unaffected in every respect — the frozen-prompt assertion, the per-slot budget
refusal and the churn tripwire all judge the built input exactly as they judged
a stored one. It exists for one shape: a stage whose later questions depend on
its own earlier answers. Stage 4's fold is that shape (§5), and a serial domain
stream already guarantees the ordering the deferral needs, so the extension is
the deferral and nothing else.

**Multi-call artifacts and stage-granular resume.** A task may be marked
`CallOnly`: it makes its call, its response is verified, and it writes nothing
— its result is the stage's own state. The stage's units are what it WRITES, so
those calls are invisible to the resume scan. Stage 4's fold uses this: *n*
boundary calls produce one composed cut list, carried by the last task of the
stream.

That makes an interrupted fold redone **whole**, and the alternative is why.
Per-boundary artifacts would each be individually provable and individually
reusable — but boundary *i+1* was adjudicated against boundary *i*'s ACCEPTED
position, a dependency no stamp records, so a resume that reused *i+1* while
redoing *i* would compose an answer to a question nobody asked. Honest
alternatives were a chained per-boundary parameter digest or stage granularity;
the ruling took granularity (2026-08-11), because resume is an optimization and
this is the shape where the proof machinery costs more than the work it saves.
A kill mid-fold therefore leaves nothing at all on disk, which is exactly why
there is nothing to salvage. What is built instead is the measurement: the fold
logs each boundary's outcome (accepted/rejected/fallback, move distance in
tokens, the window's size) and, at the composed write, the whole stage's
adjudicated-token cost — which IS what a redo re-spends, since granularity is
the stage. If that number ever justifies finer salvage, it will have said so
first.

### Hardening (the Murphy set)

Single-writer lockfile per job dir (refuse on contention). Resume verdict
forensics logged (reused/redone counts, per-redo reasons).

**After a hard kill** (SIGKILL, power loss) `job.lock` is left behind, and
the lock is never broken automatically — pids mean nothing across hosts and
containers, and an age threshold races exactly the long stage it exists for.
So BOTH modes refuse until a human deletes that one file, which qualifies
"`--fresh` is always sufficient": delete `job.lock`, then resume or
`--fresh` both work. The refusal names the file.

**Durability.** Writers temp+rename+fsync the file; directory entries are
not fsynced, and that hole is safe rather than merely acknowledged. The
artifact-before-stamp ordering means every partially-durable outcome lands
on redo: lose the artifact's entry and a lone stamp verdicts Invalid, lose
the stamp's and a lone artifact verdicts Invalid, lose both and the unit is
Absent. No ordering of losses yields a stamp proving bytes that are not
there, so no partial outcome can produce a false Valid. Cloud-synced output
directories (Dropbox-class) break rename/inode assumptions and are
documented unsupported.

**Sweep.** A completed run removes every file under the job dir the chain
does not account for — a killed write's temp residue, and artifacts of a
prior run whose plan named different paths (internally consistent, so no
verdict would ever catch them). It runs at the END of a run that described
its whole chain, which under lazy description is the only moment the
accounted-for set is complete: a job-setup sweep would delete the later
stages' reusable artifacts before their stages had resolved.

**Temporary work** (ruled 2026-08-11). A real job's intermediates live under
`<kb-output>/temp-work/` and **never** in a system temporary directory: an
interrupted run's intermediates are what its resume reads, and a location the
OS may clear between runs would make resume a coin flip. They are deleted on
SUCCESSFUL completion **only** — a failed or interrupted run keeps them, which
is one rule rather than a cleanup step with an exception, and it is the same
"litter around a broken job is evidence" reading the sweep already takes. A
keep switch overrides the deletion (`config.toml`, plus a CLI flag that beats
it) for the run that succeeded and should not have. The policy is recorded
here now and implemented with the job-plan wiring; `kbase dev-refine` predates
it and has no temp work at all — its `--out` is explicit, required, and
everything it writes is kept as the smoke run's evidence.

Crashpoint hooks at phase transitions and store writes; the resume test
harness kills at every registered point and asserts byte-identical final
output vs an uninterrupted run — including a kill at a stage boundary whose
*next* stage derives its unit paths from the artifact the killed stage
wrote.
