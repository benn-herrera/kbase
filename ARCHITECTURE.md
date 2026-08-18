# Kbase Architecture Design

A single-purpose AI appliance application for the dissection of standard documentation into an AI-friendly knowledge base markdown graph.

This document is the authoritative design reference and the single source of
truth for design mechanisms and tuning constants. Externally observable
behaviors and contracts live in [SPEC.md](SPEC.md).

**Status vocabulary.** Mechanisms here are stated as designed, and a design
this binary already runs is stated in the present tense. A mechanism the
current binary does NOT implement is marked **PLANNED** where it is
described — once, at its own home — so a reader building a picture of the
machine from this document builds the machine that exists. SPEC.md §6 is the
running list of them.

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
   `reference/`** (auto-generated API reference — see the annex convention, SPEC.md §4.3).

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

- **Single self-contained binary, embedded prompts.** No external runtime
  dependencies or shipped data files.

---

## 3. Design principles and decisions

The house thesis: **deterministic code as canonical state holder; LLM in narrow, bounded
judgment roles; the human at the edges.**

- **Cheap simplicity is a core principle, enforced against feature pressure.**
  Config is a provider URL + key. Single model family. No out-of-family machinery in
  the appliance, even for honesty checks — those move to the artifact (see BYOM eval,
  SPEC.md §6, planned). When a proposed addition breaks this, the addition loses.

- **Specified, not autonomous.** A fixed pipeline with a termination condition — not
  an agent free-roaming the corpus. Reproducibility requires same process → same
  output. Terminology guard: in this codebase, "agent" names a
  **Go-side construct** — an ask with an embedded definition, slot-built context,
  and one-shot model calls — never an LLM-driven tool-calling loop. Each pipeline
  step is mechanical where possible; LLM execution is reserved for what cannot
  practically be done any other way.

- **Propose-and-verify at every model seam.** The model never mutates canonical bytes.
  It emits *data with a mechanically checkable post-condition* (a cut list that must
  tile the document; summary-review flags; a taxonomy tree plan). Deterministic code
  validates and executes.

- **Monotone safety: mechanical fallbacks are always valid.** Every model-refined
  artifact has a deterministic fallback that was already acceptable (heuristic cuts,
  un-reviewed summaries flagged as such). Model failure degrades *quality*, never
  *correctness*. Reject-and-retry once, then fall back and log.

- **Refuse-and-split, never truncate.** A batch pipeline needs no eviction; it needs
  loud errors that push oversized units back to the stage that can fix them (the
  tree plan). Silent truncation is how a verbatim-leaf discipline dies unnoticed.

- **Verbatim leaves; summaries route, leaves answer.** Leaves are verbatim slices
  of NFC-normalized custody — the NFC ruling and its rationale live at §4 row 1.
  Otherwise leaves are faithful translations of source — no paraphrase, no
  audience simplification. Summaries exist
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
  agent definitions live in the compiled binary as a code resource.
  `kbase write-agents --out <dir>` dumps copies to local storage for
  adaptation, and refuses rather than overwrite one. The app's own definitions
  cannot be modified — the appliance version fully determines pipeline behavior.

- **Provenance and reproducibility as artifact properties.** A KB's provenance stamp
  is `app version + resolved model IDs + source identity (commit/hash)`. That tuple
  reproduces the artifact: embedded prompts are implied by app version; chunking is
  byte-reproducible (source hash + verified cut list); derived state is regenerable
  and drift-gated.

- **Artifact self-description.** Each generated KB ships the entry-point AGENTS.md
  contract: any agent that picks up the artifact finds its operating manual inside.
  The agent DEFINITIONS are not in there (ruled 2026-08-15) — they are samples for
  the user's own tooling rather than KB content, so they are written on demand by
  `kbase write-agents` and the contract carries a one-sentence pointer to it.
  A definition inside the tree would be a file the user adapts and the next build
  overwrites.

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
| 1 | **Ingest** | deterministic | Markdown corpora, walked and taken into immutable byte custody, through a **Unicode NFC pre-pass** (§9): custody bytes are NFC bytes, so every offset, slice, hash, title, slug and comparison downstream works on one spelling of every character instead of the two Unicode allows. It is the only transform custody permits, it happens before any offset exists, and it is recorded — the unit carries the upload digest beside the custody one, and invalid UTF-8 is passed through untouched. The pass covers **ids as well as bytes**: `SourceDoc.Path` is the NFC spelling of the path, so a document has one identity however the filesystem spelled its name, and every comparison downstream is NFC-against-NFC byte equality with no re-normalization at any use site to remember or forget. The rationale is the split between the two jobs a path does: *following a reference* needs one spelling on both sides, while *naming the file a human has to open* needs the disk's own — so `SourceDoc.UploadPath` keeps the spelling as read, exactly as `UploadSHA256` keeps the bytes as read. Ids are normalized before the sort, because path order is the corpus's canonical order and the thing ordered has to be the id. **Formats that are not Markdown are converted before kbase sees them, never inside it** — LaTeX is a separate `tex→md` converter module (see below), and PDF is a *preprocessing-adapter slot* (docling/marker-class external tools), never a native capability: PDF extraction is a tar pit, fenced off. MDX is **a document, read as-is** (ruled 2026-08-18, `markdown.ExtX`). It was ignored on the ground that its content is build-time-produced and so breaks source-hash provenance; that is true of the DYNAMIC half only — the JSX components and the `import` statements — and the prose beside them is ordinary Markdown sitting in the source bytes like any other. What the appliance ignores is therefore the dynamic content, deliberately, rather than the file: a JSX tag and an `import` line already parse as CommonMark (a paragraph, or an HTML block), so they survive into a leaf as inert text, which is verbatim custody doing exactly what it promises. **No strip pass** — that would be a second transform over custody bytes beside the NFC pre-pass, and it stays unbuilt until a corpus shows it costs something real. The ruling's occasion was the measurement the exclusion list was built to make possible: Rojo writes three of its eleven documents in MDX, so ignoring the extension delivered a knowledge base missing a quarter of the corpus with nothing but a run-record line to say so. **Every path the walk does not take is recorded** — `ingest.Exclusion`, one entry per skip with the class of its reason, logged at `info` and carried into the run record (SPEC §3.8) — because the ignored format class above, the dot-directories and the broken links are otherwise indistinguishable from documents that were never there, which makes "nothing was silently dropped" a claim about what the walk chose to look at [MAD2: B-3]. Disclosure is the run record only (ruled 2026-08-17): the denominator is what an operator needs, and the delivered tree is the documentation set's rather than a report on the walk. |
| 2 | **Survey** | deterministic (mostly) | Per-file structural inventory: heading tree, section token sizes, link graph, first-paragraph gists, and the legal cut candidates stage 4 clamps the model to (§5). For Markdown this is nearly all mechanical. Produces the compact artifact the taxonomy stage consumes. If the survey itself would overflow, roll up per-domain surveys first (survey-of-surveys). **Link resolution reads two address spaces** (SPEC §4.5): the destination against the linking file's directory, and — a documentation site serves `page.md` at `page/` — against that page's own URL directory. A doc corpus is written in one of the two and kbase would otherwise read it in the other, which is not a spelling nicety: source cross-references are the design's ONLY lateral-navigation channel (§3), so a resolver whose address space is wrong for the corpus class delivers a knowledge base with none. Escaping the corpus root disqualifies one attempt, never the destination. The resolution outcome is counted in the corpus roll-up and carried into the run record, because "this corpus has no cross-references" and "this build resolved none of them" are the same artifact without it. |
| 3 | **Taxonomy design** | gemma-4-31B | Consumes the survey, never raw source. Serial (needs whole-corpus view). Emits the document tree plan: hierarchy, leaf assignments, acceptance criteria. Must guarantee no leaf's source span exceeds the per-call budget (sizes are in the survey) — oversized sections are split at design time, not discovered mid-distillation. No-fallback seam: a container that fails twice fails the unit, poisons the artifact and refuses the job. **Sections ARE containers, where the size question forces it** (rule R-C, ruled 2026-08-17; `treeplan.Skeleton`). A surveyed section becomes a container — an index over its own body and its child sections — exactly when its subtree exceeds the leaf budget AND it has children; otherwise it stays one candidate, one span, one page. That is not "more granularity": it is the split that has to happen anyway, made semantic. A section over the budget is going to be cut regardless, and the author's own headings are a better cut than the splitter's heuristic one — which is also why the rule needs no constant of its own and why its call count scales with corpus SIZE rather than with how many headings someone typed (siblings' subtrees are disjoint, so one level holds at most `corpusTokens / leafTokens` descents). A section over the budget with NO children still splits mechanically, which is correct: there is nothing to descend into. **The body span carries the container's own heading line** (`body(S) = [S.Start, firstChild.Start)`), deliberately: starting after the heading would drop those bytes from every delivered page and open a second carve-out beside `metadata`. The body page is named by the file-preamble convention ("Introduction to …", R-2), so the tree has one grammar for "this container's own material" wherever it appears. **Coverage is stated over a boundary set, not over containment**: for each non-annexed file the group spans exactly tile the material the survey found, and every span endpoint is a section start the survey drew (`treeplan.checkTiling`). That is strictly stronger than the containment predicate it replaced — it forbids the same mid-section cut and names gaps directly instead of catching them by side-effect — while permitting what containment could not: a container's body, a run of sibling subsections, or the container's whole subtree on one page. The rule is stated exactly ONCE, in `Skeleton`; the taxonomy stage's question set, the `[dev]` mechanical proposal and the floor all read it, and the coverage gate holds no descent rule at all. **A section too SMALL is the same question at the other end**, and it is answered the same way: composition merges a span under the §9 minimum into the adjacent group of its own file (`treeplan.Verifier.floor`, the operator between dissolution and split expansion) rather than naming a page over it, and refuses — naming every one — where such a span has no adjacent span to merge into. **The merge is scoped to the parent index** (ruled 2026-08-17, ARCH F2): file byte order does not respect the tree, so a touching neighbour under the SAME parent is preferred over one under a different parent in either direction, ahead of the backward-first rule. Where only a cross-parent neighbour touches, the merge is taken — refusing would make an ordinary corpus unbuildable over a defect it does not have — but it is logged at `warn` and counted into `TreePlan.CrossParentMerges` (SPEC §3.3, §3.8), because no gate can observe it afterwards: coverage is asked per file and per section, and gate 5 compares the delivered set against the plan that already holds the repair. The index a cross-parent merge empties is pruned with its scope line, which is the same collapse dissolution makes: the tree keeps only what the artifact can represent. Coverage is exactly-once, so the bytes must sit on SOME page and only the stage that assigns spans can decide that a page should not exist; the alternative is a routing surface promising an overview and delivering one byte [MAD2: B-6]. **The floor applies to SPLITS, not to documents** (ruled 2026-08-17): a span covering its file's whole coverable content is exempt and stands as its own leaf at any size (`treeplan.Verifier.wholeFile`, asked inside `underFloor` so the operator and the re-check read one predicate). The floor's target is a page manufactured out of part of a larger document; an author's whole tiny document behind its own title is an honest page, not a promising-scope-empty-room — and the alternative was a corpus of small whole documents that could not be built at all. Merging several tiny FILES into one page is a separate feature, deliberately not implemented: it would have to title a page after no document. The floor is stated over group spans and carries to every part of a split group, because the splitter pre-merges below-minimum sections. Implemented: `internal/taxonomy` — a container descent composed as a fold over one serial lane; call-time input and no-call rules are §12's. |
| 4 | **Dissection** | mechanical + 26B-A4B + mechanical | See "boundary refinement" (§5). The model never emits raw offsets: it chooses among adapter-enumerated legal cut positions, so structurally invalid cuts are unrepresentable. Output: verified cut list → deterministic dissector slices the immutable source by byte offsets. |
| 5 | **Distillation** | mechanical | **Leaves are mechanical, in every format** (ruled 2026-08-09, widened 2026-08-12): the leaf body is the verified byte slice plus mechanically generated wrapping — the model never writes leaf text, so leaf-fidelity deviation is impossible rather than checked. The "26B-A4B translation for non-Markdown formats" variant is **dead**: conversion happens outside kbase and is deterministic (row 1), so there is no format at which a model writes a leaf. The historical "~90% of tokens" estimate applied to that translation and goes with it. Implemented: `internal/distill` (byte derivation, the rebase map at SPEC §4.5, the page grammar, and the post-cut per-part descriptor stage 8 renders into a split family's index bullets — the cut positions do not exist when stage 3 writes the group's one shared scope, so the distinguishing half of that bullet can only be derived here [MAD2: B-4]); the stage has no `AskSpec` and spends nothing. |
| 6 | **Hierarchical summaries** | gemma-4-31B, bottom-up | **Sibling leaves are always summarised as a group, in isolation** (ruled 2026-08-17): one call over a node's direct pages, which is the call an index whose children are all leaves already makes. Where the node holds nothing else that call's answer IS its summary. Where it also holds index children, the group's answer is an **ephemeral leaf-group card** — a temp-work artifact, never delivered — and the node's own call then reads only summary-class units: the card, plus each index child's capped framing+conclusions. So `summary(node) = blend(group(direct leaves), summaries(index children))`, and a page body never shares a call with a capped summary. What this replaces was the cell's own contradiction and a real defect: reading leaf children as bodies beside capped index children made a heavy root-level page outweigh three whole domains ~2:1 in the entry-point's call, on two unrelated corpora, as a function of corpus layout rather than of anything a definition could say [MAD2: B-5]. Three consequences are RULED and stated rather than repaired: inside the group call a copious leaf outweighs a terse sibling (size is signal among things of one kind); a node with one direct leaf makes a group of one; and parity at the parent is per SHELF, not per page — one card speaks for N pages beside one summary per index child. Summaries are the navigation surface; quality binds hardest here, hence the heavy tier. No-fallback seam (O-1), with cascade-failure carrying a failed level upward — and a mixed node's summary is downstream of its own card, so a card that never verifies cascades into it. Implemented: `internal/summarize` — TWO stages per level, deepest first (the level's cards, then its summaries, because §12 forbids a unit naming an upstream from its own stage); the artifact is JSON, never rendered Markdown, so a regenerated summary cannot half-rewrite a delivered page, and the card is named `<node>.leaves.json` where a summary is `<node>.json`, so no render path can reach one. The scheme's own price is disclosed rather than inferred: `leafGroupCards` in the run record's live block is one per mixed node, on the same argument that put `taxonomyCalls` there — a scheme costing up to one heavy call per mixed node must not leave a live run unable to say how many it paid (ARCH F8). |
| 7 | **Review** | gemma-4-26B-A4B | **PLANNED** — no review stage runs in `kbase build` today (§6). Structured rubric, not "is this good?" (see §6). Output is **flags for regeneration** (by the 31B), never edits — a weaker model's fingerprints stay off the best text. |
| 8 | **Link generation** | deterministic | Bidirectional tree-nav links (up-links, index children, and the continuation edges between the parts of one split group) emitted from the tree plan by template — a part's siblings are a projection of `groups[].parts` exactly as its parent is a projection of the parent edge, which is why they are mechanical navigation and not the inferred cross-reference §3 forbids [MAD2: B-4]. Dead NAV links are impossible by construction — they are projections of the tree plan — and the link checker demotes to a regression tripwire for them. A source cross-reference is a different claim: it is resolved (row 2) and rebased at stage 5 onto the node that now holds its target, and what remains — a destination naming a document the corpus does not hold — is delivered verbatim, exempted by guarantee 1, and COUNTED (SPEC §4.5 rule 3, §3.8). The exemption is what keeps someone else's broken corpus from refusing a delivery; the count is what keeps it from being invisible. Pages are COPIED from the stage-5 artifact byte-for-byte; rebasing already happened. Implemented: `internal/assemble` (index/entry-point grammar, the closed fixture manifest — `AGENTS.md`, `README.md`, `CLAUDE.md`); the up-link, relative-path and provenance-footer renderers are `internal/distill`'s, called from here. |
| 9 | **Refresh / verify gates** | deterministic (kb_tools lineage) | Derived-state regeneration + integrity gates; idempotent; drift-gated. Any failure refuses delivery and names the node; there is no deliver-with-warnings mode. Implemented: `assemble.Verify` — ten gates, each scoped to the file class it applies to, run over the assembled tree in the store before any byte reaches `<out>`. |

**Format seam.** The survey artifact is the source-format independence
boundary. Format-specific code lives only in an adapter package
(`internal/survey/markdown`); the adapter produces the neutral artifact, and
every stage downstream consumes artifact fields and byte offsets into the
custody bytes — never a parser node. The neutral package holds the artifact
types, the corpus roll-up, and the tiling and custody checks it runs over every
adapter's output; the ingest walk is neutral too, taking the adapter's
document-extension set as a parameter. The boundary is enforced by an
import-policy test that parses every file in the module
(`internal/survey/importpolicy_test.go`): a parser library outside its adapter
fails the build, naming the file, the import, and the rule. No adapter
*interface* exists yet — with one implementation there is nothing to compare
against, so the abstraction is deferred to a second adapter, where it can be
derived rather than guessed.

**LaTeX is a converter, not an adapter** (ruled 2026-08-12). The earlier plan
was a native `internal/survey/latex` adapter; it is withdrawn. LaTeX support
becomes a SEPARATE module — its own `go.mod`, its own repository life — that
reads a tree of `.tex` and writes a tree of `.md`, and knows nothing about
kbase. kbase then ingests that output through the ordinary Markdown front door,
which means the whole pipeline downstream of ingest stays single-format and the
converter can be used, tested and replaced on its own.

Its contract, so it produces a corpus this pipeline can survey:

- **Mirror the source hierarchy.** `src/ch1/intro.tex` → `out/ch1/intro.md`.
  The tex↔md path mapping is mechanical, so provenance is readable without a
  manifest and re-running the converter over a changed source touches one file.
- **Ordered include-graph links at the `\input` sites.** An `\input` is a
  structural edge; emitting it as a link at the position it occurred keeps the
  document's reading order in the output, which is what the survey's link
  graph and the taxonomy stage read.
- **Two passes, shared context.** Pass one collects the preamble macro table
  and the label map; pass two converts with both in hand. A one-pass converter
  cannot resolve a `\ref` to a label it has not reached.
- **Math passes through verbatim**, in `$`/`$$` delimiters, with
  parameterless macros expanded inside it. Rewriting math is the failure mode
  that silently changes meaning, and the KB's consumers read TeX math fine.
- **Never silently drop.** Anything the converter does not understand is
  emitted verbatim and flagged in a manifest. A converter that quietly omits a
  construct produces a corpus whose leaves are wrong in a way no downstream
  check can see.
- **pandoc is a dev-time differential oracle only** — something to diff
  against while building confidence, never a shipped dependency: the appliance
  is one static binary with no external toolchain.

---

## 5. Boundary refinement (stage 4 detail)

Implemented in `internal/dissect` (format-neutral: it reads the survey
artifact's neutral types and the custody bytes, never a parser).

1. **Mechanical splitter** (`dissect.Split`) proposes cuts at heuristic section
   breaks → offset+length list (`[]survey.Span`). It prefers the strongest
   structure available before the budget (`survey.CutKind.Rank`: heading, then
   fence, then paragraph) and fills toward the budget among equals. Pre-merges
   below-minimum fragments so the model only adjudicates real boundaries. A
   span whose candidates cannot cut it under the budget is refused by name
   (`dissect.StarvedRejection`) and goes back to the stage that sized it —
   refuse-and-split, never truncate. **Its output always passes verification**,
   which is a property test over both synthetic adversarial spans and every
   section of the pinned corpus.
2. **Model refinement pass** (26B-A4B, `dissect.Refiner`): a serial FOLD — one
   call per boundary in ONE serial lane per span, so a worker walks them in
   order and each window load overwrites the previous (O(1) context, O(n)
   calls). The
   scan judges each boundary against the cut list **as it stands now**: an
   accepted choice updates the working list, and the next boundary's window,
   menu and verification all derive from that updated list. The fold's output
   is therefore ONE artifact — the composed cut list, whole-list-`Verify`d
   before it is written and carried by the LAST task of its lane; every
   earlier boundary's call is `pipeline.LaneTask.Contributes` and produces nothing at
   all, which is what makes the fold's resume stage-granular (§12). The STAGE
   is `dissect.StagePlan`: one lane per split group, so a job's folds fan out
   across workers while each stays strictly in order, and one ask spec routing
   every response back to the fold that owns its unit. It is registered by the
   composing verb (`cmd/build.go`), which threads the definition's declared
   effort and retry (`dissect.Effort`/`dissect.Retry`, beside the definition
   text in `internal/dissect/definition.go`) to both the folds and the stage;
   a fold built with a declaration the stage does not ask with is refused
   where the lanes resolve. Each
   boundary's overlap window (`dissect.MoveWindow`, from
   `dissect.MoveWindows`) renders into the Content slot and its numbered candidate
   menu into call reference buffer, capped at `dissect.menuCap` entries around the
   incumbent (`dissect.menuSide` — 3 — candidates on each side, the cut itself
   marked `(current)`; a short side contributes what it has and lends nothing to
   the other), so confirming the boundary — the most
   common correct answer at a fallback-backed seam — is a choice the model can
   deliberately make.
   A boundary's question does not exist until its predecessor is adjudicated,
   so the per-call input is BUILT when the worker reaches the unit
   (`pipeline.InputBuilder`, §12) rather than when the stage was described.
   The model writes no text and **is shown no raw byte offset anywhere**: not
   in the menu, not in the status lines, not in stage reference buffer, and not in
   the retry note a retry carries (`dissect.RejectionError.Note`, the
   model-facing rendering, against `Error`'s operator-facing one). It answers
   with a menu NUMBER and the answer must BE that number —
   `dissect.parseChoice` rejects a number embedded in a sentence, because the
   prompt necessarily contains other numbers (menu positions, "boundary 3 of
   12") and a mis-mapped choice is a legal menu entry that then verifies. A
   visible retry is cheaper than a silently wrong-but-valid cut. Candidates are
   enumerated at survey time by the format adapter and carried in the artifact
   as `survey.CutCandidate{Offset, Kind}` (schema `kbase.survey/2`), so
   refinement's whole input is reproducible from stamped artifacts. Authority is clamped twice: to the overlap window ∩ the menu, and
   to the enumerated set — bisecting a heading is unrepresentable, not merely
   detectable. The composed cut list is stamped with the stage's **parameter
   digest** (span + budget + declared effort + the mechanical cut list): the
   list is computed in-process rather than read from an upstream artifact, so
   without it a re-plan's `cuts/cutlist.txt` and this one's are
   indistinguishable to the resume scan (§12). The digest is taken over the
   MECHANICAL list, which the fold never touches — it identifies the questions
   the stage asked, and the working list is the answers. The effort is in it
   because §3 claims the provenance tuple REPRODUCES the artifact, and a list
   adjudicated with thinking on is a different answer to a different asking.
   The stage refuses a fold built with a declaration it does not itself ask
   with (`dissect.StagePlan`), so the digest can never describe an asking the
   calls did not make. Optional per-boundary confidence
   emission, with low-confidence boundaries escalating (31B look or human),
   is **PLANNED**: the answer must BE a menu number (`parseChoice`, above),
   so a boundary carries no confidence today and nothing escalates.
   The fold's single-owner discipline is asserted in two halves, because it has
   two seams: `Refiner.claim` refuses a second goroutine, and an ORDINAL
   (`Refiner.next`) refuses a boundary out of turn — which is what covers the
   BUILD path, where the working list is read in the worker goroutine before
   any exclusivity check on the write path could fire. The ordinal advances
   only on a terminal outcome, so the one informed retry re-asks the same
   boundary. A `Refiner` is one run's: composing its list spends it, and a
   second fold over the same instance is refused rather than silently
   restarting from a half-folded working list.
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
   the fallback) and `dissect.OffsetDefectError` (never retried — it reaches
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
4. **Dissector** (`dissect.SliceLeaves`, deterministic) slices the immutable source
   by verified offsets. Source is never sliced-and-retyped; the leaves are
   views into the custody bytes and the cut list is a derived overlay
   (content-anchored spans — same discipline as the contract-analysis design).

---

## 6. Summary review (stage 7 detail)

**PLANNED.** Neither the review pass nor the flag-driven regeneration it
feeds is part of `kbase build` today: the stage chain the verb composes runs
straight from stage 6 to stage 8, and a delivered summary is never reviewed.
The design below stands as written.

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
- Overflow behavior: **refuse-and-split** (loud), pushed back to the tree plan. Never
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
**presumes stage reference buffer is flushed at section/stage transitions** as the table
says — that policy is the orchestrator's to enforce (phase-op table), and a buffer that
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
- Slot 8 additionally carries the per-call acceptance criteria the taxonomy tree plan
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
2. **Phase-op table (orchestrator).** The orchestrator holds a phase enum
   (job-setup → stage-setup → call-loop → section-transition → …) and an
   allowed-operations table **as data**: every context-affecting operation
   (rebuild a stage context, flush stage reference buffer, …) checks the table, and a
   disallowed operation is a loud defect. This closes the gap types cannot reach:
   constructing a *fresh* stage context with different bytes mid-task is
   type-legal but a churn bug — the risk is identity across instances, not
   mutation of one.
3. **Churn tripwire (per call, always on).** The builder emits per-slot byte
   hashes with every built call; slots the current phase declares stable must
   hash identically to the previous call, and a mismatch is a loud refusal —
   the drift-gate discipline applied to prompt bytes. This is the production
   form of the prefix-stability property; cheap (a few string hashes per call).

Single source of truth: the phase-op table that gates operations also derives the
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
- Calibration is **PLANNED**: today the estimator is the fixed provisional
  constant of §9 and nothing refines it. As designed — if a tokenize endpoint
  is detected at configure time, one calibration call checks/refines the
  ratio; thereafter the constant is refined from the `usage` token counts
  providers return on every real pipeline call, free ground truth as a
  byproduct — and the calibration source is stamped into the provenance
  receipt (SPEC.md §4.6).

---

## 9. Constants (single source; initial values, to be calibrated)

| Constant | Value | Note |
|---|---|---|
| Context ceiling | ~180K tokens | reliable zone of gemma-4's ~250K window |
| Per-call target | 60–80K tokens | quality/cost operating point; nobody runs near ceiling |
| Boundary overlap | 20% of each neighbor, capped at 1K tokens/side | `dissect.overlapFraction`/`overlapCapTokens`; the window the model sees AND the clamp bound. Exact % still TBD at calibration |
| Min section size | 64 tokens | `dissect.MinTokens`; pre-merged mechanically before refinement, and the same floor a delivered page must clear — §4 row 3's content floor reads this constant and `dissect.Params.UnderMinimum`, so the two seams cannot disagree about what is too small. "> overlap into a minimum section" holds by construction, since the overlap is a fraction under 100%. TBD at calibration |
| Boundary menu cap | 7 entries | `dissect.menuCap`; 7±2 is the honest ceiling for a choice a small tier reasons over, and the cap is what makes this seam's prompt bounded by construction (§12). Mechanism: §5 |
| Retry policy | per definition; defaults 2 attempts / 12-word note, then mechanical fallback + log | `pipeline.RetryPolicy`, declared with `pipeline.DeclareRetry` beside the effort at the registration site; the runner reads it and decides none of it. Full story: §12 |
| Effort declaration | per definition, two values (first attempt / retry). Boundary refinement **off/off**; taxonomy design and summaries **off/on** | `model.RequestEffort`, positional at the registration site; gated by `pipeline.AskSpec.validate`. Full story: §12 |
| Chars-per-token | 4.0 (provisional) | gemma-4-specific constant (`tokens.DefaultCharsPerToken`); heuristic counter (§8), calibrated then usage-refined |
| Response token cap | 16K (provisional) | per-call MaxTokens default (`model.DefaultMaxTokens`); revisit at calibration |
| Stream idle timeout | 2 min | max gap between stream reads, SSE keepalives count (`model.streamIdleTimeout`) |
| Response-header timeout | 2 min | handshake guard on the transport (`model.responseHeaderTimeout`) |
| Error-body echo cap | 8 KB | non-2xx response echo bound (`model.errorBodyLimit`) |
| Gist word cap | 40 words | survey routing hints and titles (`survey.WordCap`, post-seam export); word-capped, never mid-word |
| Leaf token budget | 4000 (provisional) | `treeplan.defaultLeafTokens`; guarantee G-1 — the span budget the stage-3 verifier splits toward, and what bounds stage 4's and stage 5's per-call input. Carried in the artifact (`TreePlan.budgets`), so a consumer reads the number the tree was verified against and never this constant |
| Summary input budget | 60K tokens (provisional) | `treeplan.defaultSummaryInputTokens`; guarantee G-2 — the whole input one stage-6 call may read. A node has up to TWO calls (§4 row 6), so the bound is stated over each and the node costs the larger: its direct leaves' bodies summed (the group call), against one summary cap per index child plus one more for the leaf-group card where the node has direct leaves (its own call). Provable from the tree plan before stage 6 runs; `treeplan.shelves` is the accumulator, filled by the interposition operator and by the composed-artifact post-condition |
| Summary cap | 400 tokens (provisional) | `treeplan.defaultSummaryTokens`; what one SHELF costs its parent — an index child's own summary, or the one card standing for every page directly under the node — and the cap stage 6's verifier holds both to |
| Entry-point ceiling | 3000 tokens (provisional) | `treeplan.defaultEntryPointTokens`; enforced by refusal at stage 9, where the rendered bytes exist to measure |
| Summary level ceiling | 16 levels | `summarize.levelCeiling`; how many KB levels stage 6 declares stages for. **Not a cap on the tree** (R-3, ruled 2026-08-17): there is no depth budget, nothing refuses a deep corpus, and the tree plan carries no `depthCap` — the tree is as deep as the nesting the source GRAPH requires, and a heading tree is one witness of that graph rather than the definition of it, so a flat single file with an internal link-list index can legitimately be six levels. This number exists only because the stage LIST is fixed at job setup while the tree plan is written by stage 3 in the same job: there is no artifact to count levels off when the chain is described. A level with no nodes resolves to zero lanes, so declaring more levels than a corpus has costs two empty stages each and nothing else, which is what makes a generous ceiling the cheap side of the trade. It is generous against the deepest thing anything can present — the entry-point, Markdown's own six heading levels, and the folder and grouping levels a model may interpose above them — and a tree past it REFUSES rather than losing its deepest summaries silently (`Summarizer.resolve`). The depth a build reached is measured and reported in the run record (`maxDepth`, `treeplan.TreePlan.MaxDepth`), never adjudicated |
| Fan-out cap | 12 children (provisional) | `treeplan.defaultFanOutCap`; a group of one is legal. Breaches are repaired by index interposition, not truncated |
| Taxonomy candidate cap | 120 entries (provisional) | `treeplan.defaultCandidateCap`; one container call's candidate list. A container with more direct children is mechanically pre-batched, so a breach reaching the verifier is our arithmetic and not a runtime condition |
| Slug cap | 8 words, 64 bytes | `treeplan.slugWordCap`/`slugByteCap`; a slug keeps the title's own bytes (math symbols, accents, CJK survive), filtered only by `treeplan.slugHostile` (path separators, the Windows-illegal set, `#`, `%`, `.`, parentheses, whitespace/dash runs collapsed to one hyphen, ASCII-only lowercasing). Byte cut lands on a word boundary. Safe under the NFC pre-pass (§4 row 1) |
| NFC pre-pass | Unicode NFC, at ingest — **content bytes and path ids alike** | `ingest.New`/`ingest.NormalizePath`, via `golang.org/x/text/unicode/norm` (the sanctioned dependency this row exists for; policy: AGENTS.md). Full story: §4 row 1 |
| Wire retries | 3 attempts, 500ms base doubling | 429 retried, other 4xx not; ctx cancel never (`pipeline.wireAttempts`/`wireBackoffBase`) |
| Model attempts | 2 (default) | initial + one informed retry (`pipeline.defaultModelAttempts`). A DEFAULT, not the policy: the count is `RetryPolicy.Attempts`, per definition, and every definition ships this value today |
| Retry-note bound | 12 words + 4-word prefix (default) | the retry's failure-reason note (`pipeline.defaultRetryNoteWords`); per definition as `RetryPolicy.NoteWords`, same reading as the row above |
| Worker pool default | 4 | serial-lane workers per stage (`pipeline.DefaultWorkers`) |
| Job lock filename | `job.lock` | `pipeline.LockFileName`; O_EXCL, refuse on contention, never auto-broken; lives inside `temp-work/` with everything else transient |
| Artifact stamp | `<artifact>.stamp.json`, schema 1 | `pipeline.StampSuffix` sidecar; schema mismatch ⇒ Invalid |
| Temporary-work directory | `<out>/temp-work/` | `pipeline.TempWorkDirName`; the store's root, kbase-created, swept and torn down only inside it (§12) |
| File/directory creation modes | 0666 / 0777 | `pipeline.CreateFileMode`/`CreateDirMode`, exported (also used below `pipeline` in the import graph, e.g. `config`). Umask-respecting: kbase is a documentation tool, not a keystore, so everything it creates asks for the permissive mode and lets the user's umask decide — passed to `OpenFile`/`MkdirAll`/`WriteFile` only, never `Chmod` (which ignores umask). The provider key-file permission warning is separate and untouched: those are the user's own files |
| Max corpus bytes | 256 MB (provisional) | in-memory ingest ceiling (`ingest.maxCorpusBytes`); loud refusal, no override flag |
| CRITICAL section cap | 100 words total (provisional) | whole slot-8 trailer (`prompt.criticalWordCap`); the budget the two reserved shares below are cut from, never itself enforced |
| — authored `## CRITICAL` share | 60 words (derived) | `prompt.authoredCriticalCap` = cap − criteria share; enforced by the dev-time definition test and repeated by the builder for user-adapted copies |
| — injected criteria share | 40 words (provisional) | `prompt.maxCriteriaWords`; the builder's per-call acceptance criteria. Reserving it is what makes a dev-time pass guarantee a runtime pass (§7) |

### 9.1 `[dev]` switches

Switches that serve kbase's own development live in `config.toml`'s `[dev]`
table and **never** on a verb. A verb's flag set is what we deliver, and
`kbase build` is the delivered article: a development switch in it is a
promise to a user that we did not mean to make, and an integration test that
drives one is not testing what we ship.

That placement is a concession to pragmatism, not a free pass. Each switch
must name the **upstream cause** that makes it necessary — a property of the
design that leaves no other way to get at the behaviour — in the `config.toml`
template comment and here. A switch that cannot name one is a flag looking for
a home, and the answer is no.

| Switch | Upstream cause | Code |
|---|---|---|
| `telemetry` | inference timing is a provider extension, off the normal path and useless in a user's output; it is a second measurement channel for prompt-shape questions (§7) | `config.DevConfig.Telemetry`; consumed in `pipeline.runner` at **info**, so `--log-level info` is needed to see it |
| `keep_temp_work` | the asymmetric teardown rule (§12) deletes the scratch tree of a run that SUCCEEDED, which is exactly the run whose intermediates someone occasionally wants | `config.DevConfig.KeepTempWork`; also a verb flag (`--keep-temp-work`), which is the user's legitimate need for the same thing — the flag turns it on and cannot turn it off |
| `build_date` | provenance receipts are DATED BY DESIGN (SPEC §4.6), so two otherwise identical runs either side of midnight UTC deliver different bytes — and byte-determinism cannot be tested against a clock. Dropping the date would throw away a fact the page is meant to carry | `config.DevConfig.BuildDate`, `cmd.resolveBuildDate`; a malformed pin refuses rather than landing in every footer |
| `tree_plan = "mechanical"` | taxonomy design is a RULED no-fallback seam (§12, O-1), so the pipeline has no mechanical mode of its own and a hermetic end-to-end run of the delivered binary has no entrance. This is that entrance: `treeplan.SourceStructureProposal` instead of the model stage, and no provider dialed at all — so no summaries either | `config.TreePlanMechanical`, `config.DevConfig.MechanicalTreePlan`; recorded in `run.json` as `"treePlan": "mechanical (dev)"`, because a tree nobody designed must say so |

---

## 10. Model tier mapping

| Tier | Stages | Why |
|---|---|---|
| gemma-4-31B (dense) | taxonomy design; hierarchical summaries; regeneration on review flags (**PLANNED**, §6); low-confidence boundary escalation (**PLANNED**, §5) | serial, judgment-heavy, small token share; navigation-surface quality binds here |
| gemma-4-26B-A4B (MoE) | cut-list refinement; review (**PLANNED**, §6) | parallel, checklist-shaped |

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

## 12. Execution and resume (orchestration design)

The orchestration layer (`internal/pipeline`) runs the fixed pipeline. "Agent"
here is the §3 Go-side construct: an ask = embedded definition + tier +
verifier (+ mechanical fallback where one exists), executing one-shot calls.

### Phase-op table (single source)

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

A worker owns a **domain** and processes its leaves serially (one serial lane);
parent↔child channels only, no peer↔peer. Stability is enforced per §7's
three layers, concretized: all workers of a stage share ONE immutable
StageContext, so the stage-constant slots are identical across workers by
construction; each worker keeps its own previous-call hashes and frontier
(per-worker tripwire); a cheap per-call assert checks every call's
stage-constant slot hashes against canonical values — slots 2–3 against the
ones captured at stage setup, and **slot 1 against the job's**, rendered
once at job setup from the plan's single job frame. The slot-1 half is
what makes §7's job-constant claim enforced rather than merely intended: a
worker's previous-call hashes reset at every lane start, so its first call
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
  tree plan, never retried at the runner.
- **Every seam's inputs are bounded by construction** (ruled 2026-08-11).
  Each thing a stage renders into a prompt is capped at its source, or the
  source refuses and splits: the refinement window is a fraction capped per
  side, its menu is capped in entries (§9), and stage 7's gist checklists get
  the same treatment when they arrive. The consequence is the point —
  once every input to a call is bounded by a named constant, an over-budget
  build is not a runtime condition to be resilient to. It is a **defect
  class**: our own sizing arithmetic is wrong, and the only way to learn that
  is the hard failure. So `ErrOverBudget` keeps its current routing even at a
  fallback-backed seam that has a valid fallback in hand — a graceful fallback
  there would silently paper over the one thing the constant table exists to
  make impossible. Bounding at the source is the work; the hard fail is what
  makes the bounding checkable.
- The frozen-prompt assertion (tripwire) checks OUR stability contract; a
  violation is a kbase defect: worker abort, never retry or fallback.
- Transport failures (timeout, 5xx, dropped stream) say nothing about output
  validity: bounded backoff retries, separately from model-attempt policy.
- Mechanical validation is the propose-and-verify seam concrete: the
  response is text claiming to be data; a deterministic verifier parses and
  checks the stage's post-condition (tiling, budgets, schema). Typed
  artifacts out; raw model text never escapes the runner. The verifier is
  given the UNIT as well as the response, because a post-condition can be
  per-unit — stage 4 checks a cut against that boundary's own candidate menu
  and clamp window — and the lookup table it selects from is built when the
  stage's work is described, so the AskSpec stays stage-constant and shared.
  A verifier that concludes the failure is OURS rather than the model's says
  so by wrapping `ErrVerifierDefect` (§5's tripwire is the first case): that
  is neither retried nor fallen back, since both remedies trust the
  derivation just indicted — the worker aborts, like a frozen-prompt
  violation.
- **What a verifier may do, and what the runner guarantees in exchange.** A
  verifier's VERDICT must be a function of (unit, response, the stage's state
  as of the call). It MAY fold its result into the stage's own state — stage
  4's fold is the exemplar, and its accumulation IS the design (§5) — but it
  may never write the store: the artifact reaches disk through the runner and
  the encoder, once, on the attempt that won. The guarantees that makes safe
  are stated where they are depended on (`pipeline.Verifier`,
  `pipeline.MechanicalFallback`) rather than discovered: `Verify` runs once per model
  attempt and never over a stored response, never concurrently within one
  stage's lane, and a rejected attempt leaves the stage state untouched so
  the informed retry re-asks the same question; `MechanicalFallback` runs at most once
  per unit and only after that unit's attempts are exhausted.
- The informed retry always carries the mechanical failure reason — a blind
  identical resend hopes temperature fixes it, which is not design (ruled).
  How many attempts there are, how much of the reason the note carries, and
  **how hard the retry asks** are the definition's (`pipeline.RetryPolicy`):
  the runner reads them and decides none of them, for the
  same reason it has never decided the effort. Escalating on the RETRY rather
  than the first attempt is the shape — the first pass is what the definition
  thinks the question costs, and the retry is the only attempt carrying new
  information, so reasoning there is reasoning over a stated fact rather than
  over the question cold. The retry logs what it asked at and whether that was
  an escalation.
- A call whose answer would be discarded is **not made**. A builder reports the
  call unneeded (`pipeline.InputBuilder`) and the worker makes none: no prompt,
  no tokens, no verification, and therefore no failure of any kind. See the
  call-time-inputs block below for the rule and its one restriction.
- Seam resolution: **fallback-backed seams** (model improves an already-valid
  mechanical fallback: cut refinement, review) fall back to the fallback,
  logged, marked as fallen back. **No-fallback seams** (taxonomy,
  summaries, format translation) have no fallback by definition of why
  inference was chosen: the unit fails loudly, sibling units complete, and
  the job REFUSES EMISSION at assembly — "sorry, something rotted" beats
  "here's your invalid crap" (ruled). Resumable rerun redoes only failures.
- Accounting: per-call usage aggregation, failed units included — a unit
  that burned two model attempts and produced nothing is exactly the one
  a cost figure must not omit; `cached_tokens` logged as prefix-cache ground
  truth; dev-telemetry emission (config-gated, off by default) as structured
  records through the logging seam. The telemetry records emit at **info**
  while the default log level is warn, so `[dev] telemetry = true` needs
  `--log-level info` alongside it to show anything. It stays at info rather
  than being promoted: a diagnostic that pollutes the default channel is one
  everybody learns to ignore.

**Seams, asks, and declared effort** (ruled 2026-08-11). A *seam* is the
mechanical→inference junction, and its only classification is what happens
when inference fails: a fallback-backed seam has a valid mechanical fallback to
stand on, an essential one does not. That says nothing about how hard the
model should work, so effort — thinking today, temperature next, one
`model.RequestEffort` value — attaches to the **definition**: one value per exact
ask, declared where the definition is registered and threaded from there to
the request. Not to the seam, and not to the stage. Today each seam happens
to pose exactly one ask, so "per definition" and "per seam" would name the
same values — which is precisely why the attachment is fixed by construction
now (`model.RequestEffort` is positional on `dissect.NewRefiner` and
`model.DefaultRequest`, and required on `dissect.CutJob`, `taxonomy.Design`
and `summarize.Job`) rather than left to a terminology that a second ask at
one seam would silently break. Each definition's own declaration lives beside
its text — `dissect.Effort`/`Retry`, `taxonomy.Effort`/`Retry`,
`summarize.Effort`/`Retry` — and the composing verb threads it to the stage it
registers. Both `chat_template_kwargs` keys are always
sent, false included: an omitted key leaves the served chat template's default
deciding, which is not a declaration.

The **retry policy** moved to the definition on the same argument (ruled
2026-08-14). One size fitted every ask only for as long as every ask retried
the same way, and escalation breaks that: "ask cheaply, and reason about it only
once the cheap pass has produced something the verifier can name as wrong" is a
statement about one exact ask. So `pipeline.RetryPolicy` — the retry-attempt
effort, the attempt count, the note's word budget — is declared beside the
effort, is required on `AskSpec` under the same gate, and joins the stage's
parameter digest. Two declared efforts per ask, then, and which one goes on the
wire is which attempt is being made.

The DECLARED BIT is gated in exactly one place: `AskSpec.validate`, which refuses
an ask whose effort was never declared before any agent exists. Positional
parameters make the value *stated*, not *declared* —
`model.RequestEffort{}` passed directly compiles and reads as a deliberate "no
thinking" — and the AskSpec is the one hop every production request passes
through, so a second runtime check in `DefaultRequest` would guard a path that
cannot ship. Every `Request` this appliance builds is a definition's ask; the
catalogue probe (`Client.ListModels`) builds none.

### Chain-stamped artifacts and resume

Resume is **structural, not temporal** — no journal, no cursor, no place to
lose. Three properties: (1) every output unit is written atomically
(temp+rename+fsync) — mid-write kill states do not exist; (2) every stage
artifact carries a stamp: app version + input hashes (source identity +
upstream artifact hashes) + output hash — validity is decidable by
inspection with no knowledge of how the prior run died; (3) the worklist is
stateless — resume scans outputs and re-derives it.

Stage artifacts form a dependency chain (survey → tree plan → cuts → leaves →
summaries → links). The chain is **described lazily, one stage at a time**:
a stage says what units it owes when the walk REACHES it, not at job setup.
That is not an optimization — it is forced. The tree plan stage 3 emits is
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
tree plan a previous planning epoch left behind — but only a *proven* one:
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
tokens; structural incoherence refuses the whole resume and names the remedy
for its case — `--fresh` where the contradiction is the job directory's own
contents (a stamp from a rearranged tree, a directory where an artifact
belongs), and no remedy at all where the store contradicts the chain
DESCRIPTION (a unit consuming an artifact no stage produces and no stamp
covers), since an empty job directory would not hold that artifact either.
This is the kb_tools drift-gate discipline applied to the pipeline's own
execution.

Resume is an **optimization, never load-bearing**: `kbase build --fresh`
ignores all prior outputs unconditionally and is always sufficient over
anything a previous run left behind. The mechanism is a DISCARD, not a scan
that declines to read: the job directory's prior contents are deleted under
the lock, before the scan, which is what makes the sentence true of the
wreckage a rebuild would not otherwise touch. The scan that follows is the
ordinary one and reports what it finds, so the run's reused count is a
MEASUREMENT of the discard rather than a property the mode asserts about
itself — a fresh scan that read no verdicts would report zero reused whether
or not the deletion happened, which is a number nothing can be concluded from. It overrides the store's prior
state and nothing else — a `--out` that already holds a delivered knowledge
base refuses in both modes (SPEC.md §3.1). And at no tier does any path emit
unverified material — assembly-time verification re-checks everything
regardless of provenance (two independent nets).

**Call-time task inputs.** A task's prompt input may be a value or a builder
(`pipeline.InputBuilder`); a builder runs when the worker REACHES the unit.
Every stage whose questions are known up front passes `ConstInput` and is
unaffected in every respect — the frozen-prompt assertion, the per-slot budget
refusal and the churn tripwire all judge the built input exactly as they judged
a stored one. It exists for one shape: a stage whose later questions depend on
its own earlier answers. Stage 4's fold is that shape (§5), and a serial domain
lane already guarantees the ordering the deferral needs, so the extension is
the deferral and nothing else.

A builder may also report that the task needs **no call at all**. A stage
that decides its own questions can decide that one of them
stopped being a question, and stage 3 is the case: its container calls are
enumerated from the source tree before the first of them runs, and a container
whose parent's answer put its material on a page is absorbed, so its answer
would be thrown away (§3.2). *A call whose answer is discarded must not be able
to fail the build*, and the cheapest guarantee of that is that it does not
exist — no prompt, no tokens, no response to verify, no transport to exhaust.
The skip is counted (`JobResult.Skipped`) rather than silent: "nine questions
enumerated, six asked" is a fact about the run. It is legal only on a
`Contributes` task and refused otherwise (`SkippedArtifactError`): a task that
owes an artifact and makes no call leaves the stage a unit short that nothing
wrote and nothing reported, indistinguishable on the next scan from an
interruption. Stage 3 therefore still spends ONE call on an absorbed container
in the worst case — the last, whose call is what writes the composed plan — and
that answer is dropped before anything parses it, so it cannot be rejected
either.

**Two task modes: a call, or a producer.** Four stages of
the pipeline infer nothing — ingest/survey, distillation, assembly, verify —
and running them outside the coordinator would forfeit resume, stamps, the
sweep and the single worklist for exactly the stages that produce the
deliverable. So a task either asks a model (`LaneTask.Input`) or derives its
artifact in process (`LaneTask.Produce`), exactly one of the two, and the worker
calls the producer where it would have called the runner. Everything else is
identical: the unit description, the input set resolved and hashed BEFORE the
derivation runs, the encoder, the atomic write, the stamp, the resume scan, the
sweep, and the failure inventory — a producer's failure is an inventoried unit
failure with no fallback and no retry, since the same inputs derive the same
failure. A producer touches no prompt machinery, so a mechanical stage declares
no AskSpec at all and carries its own encoder instead. The modes do not MIX within
a stage: everything stage-scoped here is declared per stage — one AskSpec, one
shared StageContext, one seam, one tier, one encoder — so a half-mechanical
stage has no honest answer for what its seam is, and the description is refused.
The rejected alternatives were a null AskSpec with a fake verifier, which lies to
the seam classification, and leaving the deterministic stages outside the
orchestration entirely.

**Multi-call artifacts and stage-granular resume.** A task may be marked
`Contributes`: it makes its call, its response is verified, and it writes nothing
— its result is the stage's own state. The stage's units are what it WRITES, so
those calls are invisible to the resume scan. Stage 4's fold uses this: *n*
boundary calls produce one composed cut list, carried by the last task of the
lane.

That makes an interrupted fold redone **whole**, and the alternative is why.
Per-boundary artifacts would each be individually provable and individually
reusable — but boundary *i+1* was adjudicated against boundary *i*'s ACCEPTED
position, a dependency no stamp records, so a resume that reused *i+1* while
redoing *i* would compose an answer to a question nobody asked. Honest
alternatives were a chained per-boundary parameter digest or stage granularity;
the ruling took granularity (2026-08-11), because resume is an optimization and
this is the shape where the proof machinery costs more than the work it saves.
A kill mid-fold therefore leaves nothing at all on disk, which is exactly why
there is nothing to salvage.

**A failed call poisons the lane it is in** (ruled 2026-08-12). The artifact
exists exactly when EVERY call of its lane succeeded: one inventoried failure
among the `Contributes` tasks feeding it and the producing task writes nothing,
the remaining calls of that artifact are not spent, and the artifact is
reported as the cascade of the failure beside it. This is the same discipline
as the mid-kill case and needs no new stamp machinery — which is the whole
argument for it. The alternative was to write the artifact anyway and rely on
the run's own `JobResult` to refuse: but the stamp would be perfectly valid
(nothing in a stamp knows a call failed), so the next run would verdict it
Valid, drop the lane, and the failure would have existed in one run's report
and nowhere on disk. A build refusal at a boundary (`ErrOverBudget`, the one
failure the bounded-inputs principle exists to make loud) is the reachable
case, and a rerun would have laundered it.

What is built instead of salvage is the measurement: the fold
logs each boundary's outcome (accepted/rejected/fallback, move distance in
tokens, the window's size) and, at the composed write, the whole stage's
adjudicated-token cost — which IS what a redo re-spends, since granularity is
the stage. If that number ever justifies finer salvage, it will have said so
first.

**And a unit whose in-chain upstream failed THIS RUN is cascade-failed**
(ruled 2026-08-12). It is the poisoning discipline applied across a chain edge,
and it is stated because neither neighbouring rule reaches the case: poisoning
is intra-lane, while "an unstamped upstream is structural incoherence" is
scoped by its own justification — nothing this chain runs would ever produce it
— which is false for an artifact whose producing unit failed a moment ago.
Applying the incoherence rule anyway would refuse the whole resume with
`--fresh` guidance and discard every proven artifact in the job over one failed
unit. So the dependent is marked cascade-failed instead: not attempted, no call
spent, nothing written, inventoried beside its cause under its own failure kind
and naming the ROOT of the chain rather than its immediate predecessor. It
propagates transitively, since a cascade-failed unit is itself a failure the
next stage reads, and a dependent lane's remaining calls are dropped with it.
The marking happens where the stage's lanes are filtered, not at the unit: a
fold's calls all precede the task that writes what they compose, and a unit that
failed this run may still have a PREVIOUS run's artifact on disk, which would
resolve happily and buy a call spent deriving from superseded bytes. It is
RUN-SCOPED bookkeeping — no new stamp, no persisted state, no change to the
scan; next run the cause and its cascade are both simply Absent and both are
redone, which is why the extension needs no artifact machinery at all.

### Hardening (the Murphy set)

Single-writer lockfile per job dir (refuse on contention). Resume verdict
forensics logged (reused/redone counts, per-redo reasons).

**After a hard kill** (SIGKILL, power loss) `temp-work/job.lock` is left behind, and
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

**Temporary work** (ruled 2026-08-11, implemented 2026-08-12). Everything
transient a run produces — stage artifacts, their stamps, `job.lock`, the
residue of a killed write — lives under `<out>/temp-work/`, and **nothing
transient lives anywhere else**. Not in a system temporary directory either: an
interrupted run's intermediates are what its resume reads, and a location the
OS may clear between runs would make resume a coin flip. Only DELIVERED
artifacts live in the output hierarchy proper.

**Two path classes, and no third** (ruled 2026-08-15). The appliance writes
exactly two kinds of file: DELIVERED — class A by grammar, class B by the
fixture manifest (SPEC.md §4.7), permanent, and classified without exception — and
TEMP-WORK — scratch, under one teardown rule. **There are precisely zero
special-case write-and-delete-when-done files of any kind; that is what
`temp-work/` exists for.** Nothing is written outside `temp-work/` and later
removed, and nothing in the delivered tree is exempt from classification. A
file that has neither a consumer of the knowledge base nor a stage that reads
it back is a development record, and a development record belongs in the dev
mirror: `run.json` sits at the temp-work ROOT — the mirror of the delivered
tree's own root — and is torn down with everything else there, so a run without
the keep switch keeps no record (ruled acceptable; the switch is how you keep
one). `delivery.json` sits beside it under the same rule, and the rule is what
decided its home: a list of what a run delivered is read by the next run over
that `--out` and by nothing else, which makes it scratch rather than an
artifact of the knowledge base, so it lives in temp-work and dies with it. The
rule forbids the class, not the instances that prompted it: any new file
wanting a lifecycle of its own is a design error, and the answer is always one
of the two classes.

The directory `temp-work/` **is** the store's root (`pipeline.OpenTempWork`,
`pipeline.TempWorkDirName`), and it MIRRORS the output tree: scratch for
`<out>/ch1/sec2/` sits at `<out>/temp-work/ch1/sec2/`, created on demand by the
write that needs it, so one store-relative path names the scratch copy and its
delivered counterpart. Delivery is a copy of the store's own bytes out to the
mirrored path — the delivered file and the one the stamp proves are the same
bytes.

**Delivery refuses a populated `--out`** (ruled 2026-08-17). The copy above
overwrites whatever it lands on and sweeps nothing, so a rerun into a delivered
tree would leave the previous plan's pages standing beside the new plan's — a
knowledge base whose gates all passed over a tree nobody delivered. The fix is
a refusal at job setup rather than a delivery-time reconciliation: `--out`
holding anything but `temp-work/` refuses, names every entry, writes nothing
(`cmd.checkOutIsClear`, the `write-agents` refusal one directory up), and the
remedy is deleting the directory. Iterative update of a delivered knowledge
base is not a feature of this appliance [MAD2: B-7]. The exception is the run
`--out` is already in the middle of, and the predicate is structural rather
than heuristic (`cmd.interruptedJob`): `temp-work/` survives exactly the runs
that did not finish, and `run.json` inside it is written only once a delivery
completed — so a job frame without a record IS an interrupted build, whose
half-copied delivery is its own to overwrite, while temp-work with one is a
finished build and a second run over it a rerun.

The predicate reads a **job frame** rather than a directory, and that is the
third clause: a directory named `temp-work` proves nothing, because anyone can
create one, and `mkdir <out>/temp-work` would otherwise be a one-command
disarming of the whole refusal. So the residue has to carry something a run of
this binary wrote — `job.lock`, `delivery.json`, or a `.stamp.json` sidecar
(`cmd.jobFrame`). A run that got no further than creating the directory
delivered nothing either, so its `--out` is empty and the question never
arises.

**The delivery manifest** (`delivery.json`, ruled 2026-08-17) is what the
exception needs to be honest, and it is one artifact answering three questions
that have no other answer. A run writes it at the temp-work root — the exact
paths it is about to deliver — BEFORE it copies the first of them, so the run
that dies mid-copy leaves the full list rather than the prefix it managed.
Then:

- **The delivery may already be done.** Between the last copied byte and
  `run.json` sit reads that can fail, and a kill lands there like anywhere
  else; the result is a complete knowledge base that every later run reads as
  an interrupted job and overwrites, the refusal never firing again. So the
  next run verifies every manifest path byte-for-byte against the store
  artifact it just proved, and a whole match means the delivery finished and
  only the record was lost: write the record, copy nothing. A partial match is
  an interrupted delivery — there is no threshold at which most of it counts.
- **The residue may belong to a different plan.** The resumed run need not
  compute the same tree: title, annexes, corpus and app version all enter the
  parameter digest, and `--fresh` guarantees the re-plan. Delivery writes only
  what the new plan names, so without a sweep the old plan's pages stand in the
  tree beside the new plan's — gates all green over a tree nobody delivered,
  which is the state this refusal exists to prevent. The sweep deletes
  EXACTLY the manifest's paths, and the manifest is the previous attempt's own
  testimony, so what kbase may delete inside `--out` is bounded by what kbase
  wrote there. An operator's file can never be on that list.
- **And the result is checked, not assumed.** After delivering, `<out>` minus
  `temp-work/` must equal the delivered set exactly, or the run fails loudly
  (`cmd.buildJob.checkOutHoldsExactly`). It lives at the delivery step and not
  among the ten gates: those run store-side by design and cannot see `--out` at
  all, so the step that moves the bytes is the one that can prove where they
  landed. Residue from any cause — a sweep that failed, a file dropped in
  mid-run, an orphan no manifest accounted for — refuses rather than being
  deleted, since deleting what kbase did not write is the one thing the sweep's
  safety argument forbids.

That rooting is what makes the sweep safe, structurally rather than by check:
it can only delete inside a directory kbase itself created one level below
whatever the operator named, so `--out notes` cannot reach `notes/`. (`sweep`
additionally refuses a root not named `temp-work` — one string comparison, a
tripwire behind the structure.)

Teardown is asymmetric and that is one rule rather than a cleanup step with an
exception: a run that SUCCEEDED and delivered removes the tree; a failed or
interrupted run keeps it, resume-compatible, which is the same "litter around a
broken job is evidence" reading the sweep already takes. A keep switch overrides
the deletion for the run that succeeded and should not have: `[dev]
keep_temp_work` in config.toml, and a CLI flag (`--keep-temp-work`) that turns
it on for one run. The flag only ever turns keeping ON — the sole reason to
insist on deletion is disk, whose remedy is one `rm -r`, while the reason to
keep is evidence that cannot be recovered once it is gone.

**Sweep.** A completed run removes every file under the store root the chain
does not account for — a killed write's temp residue, and artifacts of a
prior run whose plan named different paths (internally consistent, so no
verdict would ever catch them). It runs at the END of a run that described
its whole chain, which under lazy description is the only moment the
accounted-for set is complete: a job-setup sweep would delete the later
stages' reusable artifacts before their stages had resolved.

Crashpoint hooks at phase transitions and store writes; the resume test
harness kills at every registered point and asserts byte-identical final
output vs an uninterrupted run — including a kill at a stage boundary whose
*next* stage derives its unit paths from the artifact the killed stage
wrote.
