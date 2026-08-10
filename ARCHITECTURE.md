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
| 2 | **Survey** | deterministic (mostly) | Per-file structural inventory: heading tree, section token sizes, link graph, first-paragraph gists. For Markdown this is nearly all mechanical. Produces the compact artifact the taxonomy stage consumes. If the survey itself would overflow, roll up per-domain surveys first (survey-of-surveys). |
| 3 | **Taxonomy design** | gemma-4-31B | Consumes the survey, never raw source. Serial (needs whole-corpus view). Emits the document skeleton: hierarchy, leaf assignments, acceptance criteria. Must guarantee no leaf's source span exceeds the per-call budget (sizes are in the survey) — oversized sections are split at design time, not discovered mid-distillation. |
| 4 | **Dissection** | mechanical + 26B-A4B + mechanical | See "boundary refinement" (§5). Output: verified cut list → deterministic dissector slices the immutable source by byte offsets. |
| 5 | **Distillation** | gemma-4-26B-A4B, parallel fan-out | Verbatim source→Markdown leaf translation. ~90% of tokens, translation-shaped. Parallel-safe by file ownership: one domain/leaf per worker. Worker context: exact source byte-range + taxonomy position + style contract + neighbor gists. |
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

1. **Mechanical splitter** proposes cuts at heuristic section breaks → offset+length
   list. Pre-merges below-minimum fragments so the model only adjudicates real
   boundaries.
2. **Model refinement pass** (26B-A4B): serial scan, each split loaded into the end of
   context with a percentage under/overlap of neighbors; each load overwrites the
   previous (O(1) context, O(n) calls). The model's output is **only** a modified
   offset+length list — it writes no text. Its authority is **clamped to the overlap
   window**: it may move a proposed cut within the visible neighborhood, not invent or
   delete sections. Optional per-boundary confidence emission; low-confidence
   boundaries escalate (31B look or human).
3. **Verification** (deterministic, all cheap): exact tiling — monotonic offsets, no
   gaps/overlaps, sum = document length; every deviation within clamp; minimum section
   size respected. Pass → dissect. Fail → retry once → fall back to mechanical cuts
   and log. The mechanical list is always valid: refinement can only improve or be
   discarded.
4. **Dissector** (deterministic) slices the immutable source by verified offsets.
   Source is never sliced-and-retyped; the cut list is a derived overlay
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
| Boundary overlap | percentage of neighbor sections | exact % TBD at calibration |
| Min section size | > overlap size | pre-merged mechanically before refinement |
| Retry policy | 1 retry, then mechanical fallback + log | monotone safety |
| Chars-per-token | 4.0 (provisional) | gemma-4-specific constant (`tokens.DefaultCharsPerToken`); heuristic counter (§8), calibrated then usage-refined |
| Response token cap | 16K (provisional) | per-call MaxTokens default (`model.DefaultMaxTokens`); revisit at calibration |
| Stream idle timeout | 2 min | max gap between stream reads, SSE keepalives count (`model.streamIdleTimeout`) |
| Response-header timeout | 2 min | handshake guard on the transport (`model.responseHeaderTimeout`) |
| Error-body echo cap | 8 KB | non-2xx response echo bound (`model.errorBodyLimit`) |
| Gist word cap | 40 words | survey routing hints and titles (`survey.gistWordCap`); word-capped, never mid-word |
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
