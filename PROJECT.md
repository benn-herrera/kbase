# kbase — the KB-ifier appliance

**Status:** design capture from working discussions (2026-08-05/06). Nothing built. This
document is the single source for decisions made so far; ARCHITECTURE.md and SPEC.md
will be broken out when there is enough solid material to populate them properly.

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
   `reference/`** (auto-generated API reference — see "annex" convention, §4).

**Non-goals (deliberate, not deferred):**

- **No claim graph.** This is tech-doc dissection, not paper/proof dissection. No
  claim/experiment/support nodes, no confidence/solidity, no applied-mathematician
  role. What remains from the kb_tools lineage: topography graph, verbatim leaves,
  link integrity, drift-gated derived state.
- **No REPL / interactive UI.** Edge-and-corner cases needing collaborative human
  attention are served by the escape hatch (§2), not by an in-app experience.
- **No business plan.** Evaluated and parked (see the insurance-gap-analysis strategy
  record); this is a personal tool with, at most, an OSS-release option later. That
  option is why the open-by-inspection posture (§2) is chosen deliberately. License:
  MIT (decided 2026-08-06). Deeper history:
  `../insurance-gap-analysis/kb-personant-tech-monetization.md` (predates the parking
  decisions, which live in that project's agent memory). Related market fact
  (verified 2026-08-06): Roblox ships a first-party AI-docs surface (llms.txt,
  per-page `.md` endpoints, Studio MCP) — retrieval tools; kbase's value claim is
  the curated-topography/docent experience, not retrieval.

---

## 2. Design principles and decisions

The house thesis, third incarnation (after Personant and the contract-analysis
design): **deterministic code as canonical state holder; LLM in narrow, bounded
judgment roles; the human at the edges.**

- **Cheap simplicity is a core principle, enforced against feature pressure.**
  Config is a provider URL + key. Single model family. No out-of-family machinery in
  the appliance, even for honesty checks — those move to the artifact (see BYOM eval,
  §4). When a proposed addition breaks this, the addition loses.

- **Specified, not autonomous.** A fixed pipeline with a termination condition — not
  an agent free-roaming the corpus. Reproducibility requires same process → same
  output.

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
  better manners. Bidirectional tree-navigation links are purely mechanical (§3).

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

## 3. Architecture

### Pipeline (fixed stage order; map-reduce shape)

| # | Stage | Actor | Notes |
|---|---|---|---|
| 1 | **Ingest** | deterministic adapters | Markdown-set and LaTeX adapters; PDF is a *preprocessing-adapter slot* (docling/marker-class external tools), never a native capability — PDF extraction is a tar pit, fenced off. |
| 2 | **Survey** | deterministic (mostly) | Per-file structural inventory: heading tree, section token sizes, link graph, first-paragraph gists. For Markdown this is nearly all mechanical. Produces the compact artifact the taxonomy stage consumes. If the survey itself would overflow, roll up per-domain surveys first (survey-of-surveys). |
| 3 | **Taxonomy design** | gemma-4-31B | Consumes the survey, never raw source. Serial (needs whole-corpus view). Emits the document skeleton: hierarchy, leaf assignments, acceptance criteria. Must guarantee no leaf's source span exceeds the per-call budget (sizes are in the survey) — oversized sections are split at design time, not discovered mid-distillation. |
| 4 | **Dissection** | mechanical + 26B-A4B + mechanical | See "boundary refinement" below. Output: verified cut list → deterministic dissector slices the immutable source by byte offsets. |
| 5 | **Distillation** | gemma-4-26B-A4B, parallel fan-out | Verbatim source→Markdown leaf translation. ~90% of tokens, translation-shaped. Parallel-safe by file ownership: one domain/leaf per worker. Worker context: exact source byte-range + taxonomy position + style contract + neighbor gists. |
| 6 | **Hierarchical summaries** | gemma-4-31B, bottom-up | Each index level consumes its children's *summaries*, never bodies — bounded by fan-out × summary cap, both controlled by the taxonomy. Summaries are the navigation surface; quality binds hardest here, hence the heavy tier. |
| 7 | **Review** | gemma-4-26B-A4B | Structured rubric, not "is this good?" (see §4). Output is **flags for regeneration** (by the 31B), never edits — a weaker model's fingerprints stay off the best text. |
| 8 | **Link generation** | deterministic | Bidirectional tree-nav links (up-links, index children) emitted from the skeleton by template. Dead links impossible by construction; the link checker demotes to regression tripwire. |
| 9 | **Refresh / verify gates** | deterministic (kb_tools lineage) | Derived-state regeneration + integrity gates; idempotent; drift-gated. |

### Boundary refinement (stage 4 detail)

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

### Context management

Careful, not complex — no stage ever legitimately needs the corpus in a window:

- One **per-call context builder** with named component budgets; component sizes known
  from ingest (the survey collects token counts).
- **Ceiling ~180K** (gemma-4 window is nominally ~250K; 180–200K is the reliable
  zone). **Target 60–80K per call** — attention quality, especially on small models,
  degrades well before the window fills, and decomposition's whole point is that
  nobody runs near the ceiling.
- Overflow behavior: **refuse-and-split** (loud), pushed back to the skeleton. Never
  truncate, never evict.

### Model tier mapping

| Tier | Stages | Why |
|---|---|---|
| gemma-4-31B (dense) | taxonomy design; hierarchical summaries; regeneration on review flags; low-confidence boundary escalation | serial, judgment-heavy, small token share; navigation-surface quality binds here |
| gemma-4-26B-A4B (MoE) | cut-list refinement; distillation volume; review | parallel, translation/checklist-shaped, ~90% of tokens |

Per-stage model config is supported from day one (it's just API refs) — keeps a
"borrow a frontier model for taxonomy only" escape hatch open if 31B hierarchies
disappoint. Default remains pure gemma-4.

### Code ancestry (decided)

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
  AI-driven steps are the clearly delineated pipeline stages (§3).

---

## 4. Implementation specifics decided so far

**Configuration / UX**

- Required config: provider API URL + key.
- **Model auto-detection:** on configure, call `/v1/models` and auto-select gemma-4
  models from the list (max-convenience default). Matching must be tolerant of
  provider naming variance (`google/gemma-4-31b-it`, `gemma4:31b-a4b`, …).
- **Fail loud and list** on ambiguity or no-match: "no gemma-4 family detected; found
  these; use `--model-map` to assign." Never silent best-guess — a wrong tier mapping
  is exactly the silent-platform failure mode.
- Manual explicit config (`--model-map`, per-stage overrides) always allowed.
- Resolved model IDs (auto or manual) are stamped into the provenance receipt.

**Tooling**

- Task recipes via **`just`** (decided 2026-08-06), not make. Rationale: pure
  task-runner use — Go owns build incrementality, so make's timestamp graph goes
  unused; just drops make's error-prone syntax traps (tab recipes, `.PHONY`, `$$`
  escaping). Available everywhere via standard package managers; dependency cost
  judged negligible. Two-gate discipline (cheap edit gate per change, full suite at
  checkpoints) carries over from the sibling-repo Makefile pattern.

**Prompts / agent definitions**

- Embedded in the compiled binary as code resources; the app's operative copies are
  immutable.
- CLI dump command writes copies to local storage, each stamped with: app version, a
  header stating "this is a copy for adaptation; the app does not read this file," and
  the adaptation-guidance note (including the recommendation to have a stronger model
  write a version suited to its capabilities).

**Generated-KB contract**

- Entry-point AGENTS.md states, minimally: **summaries route; leaves answer** (answer
  from leaf text, never index summaries); the annex convention for any raw-lookup
  territories; pointer to `.agents/`.
- `.agents/` directory ships in every KB: docent definition, maintainer definition,
  adaptation note, routing-eval question set.
- **Annex convention** for machine-shaped material (e.g. creator-docs `reference/`):
  such territories are *not* distilled into leaves. The entry point documents the
  lookup convention instead — path grammar, file format, one worked example ("engine
  API classes at `reference/engine/classes/<ClassName>.yaml`; grep there directly").
  A deterministic lookup shim is built only if usage shows agents repeatedly fumbling
  the raw structure — signpost first, machinery on evidence.

**Summary review (stage 7 rubric)**

- Two failure directions, treated differently:
  - *Hallucination direction* (summary claims X, children don't support it): tractable
    for a small model — decompose the summary into atomic claims, entail each against
    the children's text. Local, well-posed checks.
  - *Omission direction* (load-bearing child point missing): **not** easier than
    generation — noticing absence requires knowing what mattered. Converted to a
    checklist: the reviewer receives the children's own gists and must mark each as
    represented or deliberately excluded.
- Framing: **find the discrepancy**, never approve/rate (small-model judges
  rubber-stamp under approval framing).
- Output: flags → 31B regenerates. The reviewer never edits.
- Residual risk acknowledged: summary validation has no complete mitigation. The
  risk posture is containment (summaries can waste tokens, never terminally lie —
  routes/answers split) + routing eval + n=1 spot-checks in personal use.

**Routing eval**

- Generated Q→leaf pairs ("what question does this leaf answer?" — the mad-libs
  strategy borrowed from Personant's testing harness), then verify that
  summaries-only navigation reaches the right leaf.
- Ships with the artifact in `.agents/` — **BYOM eval for a BYOM artifact**. The
  appliance itself stays single-family; consumers who want cross-family honesty run
  the eval with the model they brought. gemma-4 consumer results are a conservative
  lower bound (weak-navigator-succeeds is evidence stronger navigators will).
- Known bias, accepted: gemma-generated questions may skew toward gemma-natural
  phrasing; pairs derive from leaf content so skew is mild. Optionally salt with
  frontier-generated questions (outside the appliance).

**Token counting (decided 2026-08-06)**

- Operative counter is a **calibrated chars-per-token heuristic** (single
  gemma-tokenizer-specific constant). No cgo SentencePiece binding; no hard
  dependency on a `/tokenize` endpoint — tokenize is not part of the
  OpenAI-compatible core (vLLM/llama.cpp expose it; Ollama and most cloud hosts
  don't), so requiring it would narrow provider support against the
  cheap-simplicity principle.
- Rationale: every consumer of token counts (survey section sizes, taxonomy leaf
  budgets, per-call context budgets) is a threshold check with 2–3× headroom
  (60–80K target vs 180K ceiling). Refuse-and-split makes correctness independent
  of estimator error — an undercount surfaces as one loud re-split, an efficiency
  blip. Exact counting buys nothing the margins can distinguish from perfect.
- Calibration: if a tokenize endpoint is detected at configure time, one
  calibration call checks/refines the ratio; thereafter the constant is refined
  from the `usage` token counts providers return on every real pipeline call —
  free ground truth as a byproduct. Calibration source is stamped into the
  provenance receipt.

**Numbers (initial constants, to be calibrated)**

| Constant | Value | Note |
|---|---|---|
| Context ceiling | ~180K tokens | reliable zone of gemma-4's ~250K window |
| Per-call target | 60–80K tokens | quality/cost operating point; nobody runs near ceiling |
| Boundary overlap | percentage of neighbor sections | exact % TBD at calibration |
| Min section size | > overlap size | pre-merged mechanically before refinement |
| Retry policy | 1 retry, then mechanical fallback + log | monotone safety |
| Chars-per-token | TBD at calibration | gemma-4-specific constant; heuristic counter (see "Token counting"), usage-refined |

**Provenance receipt (per KB)**

- App version (⇒ embedded prompt set), resolved model IDs per stage, source identity
  (commit hash / content hash), cut lists implied reproducible from source hash.
- Every KB page carries build provenance (`built from upstream <version/commit> @
  <date>`) — staleness is displayed, not solved; refresh/currency is explicitly a
  post-market-fit concern, out of v1 scope.
