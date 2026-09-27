# ROADMAP – KBase

Ordered intentions, not commitments. This file churns freely; design truth
lives in ARCHITECTURE.md / SPEC.md, per-burst execution detail in `TEMP_*.md`
plans (gitignored, disposed after landing — see CONVENTIONS.md "Plan & Execute
Process"). Finished items are
deleted, not archived — git history is the log.

## Now

- ~~`.mdx` decision~~ ruled 2026-08-09: **ignored** (ARCHITECTURE §4 ingest
  row) — watch what it actually costs; strip-and-scan only on evidence.
- ~~thinking mode: on, off, or per-call?~~ ruled 2026-08-11, landed
  2026-08-12: effort is per-DEFINITION and required by construction
  (ARCHITECTURE §9 row, §12) — boundary refinement declares thinking **off**.
  The A/B that settled it, both runs over the pinned `sync-details.md` at a
  512-token budget: byte-identical cut lists, 4 completion tokens and 2.8s
  with thinking off against 20,924 and 117s with it on, and the reasoning run
  took the only compliance rejection of the two. Each definition added from
  here states its own value; re-measure when the stub prompts are replaced
  with the tuned ones, since that is the change that could move it.

## Next

- **v0.2.0 arc with hybrid MAD sequence (ruled 2026-08-12)**:
  1. Architect reshape design against the ModernCorp exemplar
     (`../ModernCorp/kb-root` + source traced from `ModernCorp.tex`) —
     tree plan schema, output anatomy (index/leaf/entry-point grammar),
     stages 3/5/6/8 design; Benn rules on open points.
     Snapshot prep first: exclusion-copy the exemplar doc tree into
     `test_data/transient/mad-reference/` (drop `.index/`, `session/`,
     `claim-quality.md` wholesale; never edit file contents; manifest
     lists exclusions + in-file elements out of scope), and trace the
     source reference graph from `ModernCorp/ModernCorp.tex` (sole
     entry point), flagging KB content whose source can't be located.
  2. **MAD #1 — design review, shapes-only charter**: evaluate major
     forms (stage composition, tree plan schema, output grammar,
     converter contract); atomic detail stays flexible by declaration.
     Retirement gate: a finding must claim a SHAPE is wrong (wrong
     stage boundary, wrong artifact grammar, missing/superfluous major
     mechanism) or it retires; parameter-level findings get logged as
     build-phase notes, never debated. LaTeX-specific questions graded
     design-level (converter is post-v0.2.0).
  3. Build the arc burst-by-burst, per-burst architect+go-coder
     reviews as usual. **Burst A landed 2026-08-12**: `LaneTask.Produce`
     (O-2) and cross-stage cascade marking (MAD1 F-8) in
     `internal/pipeline`, both written into ARCHITECTURE §12; the
     queued `Agent`/`Call` unexport went with it (evidence recorded
     2026-08-10: `internal/dissect` needs only `CallRunner`).
     **Burst B landed 2026-08-13**: `internal/treeplan`.
     **Bursts C+D landed 2026-08-13**: the mechanical spine —
     `internal/distill` (stage 5: byte derivation, the rebase map, §4.1's
     page grammar), `internal/assemble` (stages 8+9: index/entry-point
     grammar, the §4.7 fixture manifest, the ten verify gates,
     delivery), the `SourceStructureProposal` dev/baseline tree plan, and
     the composing build verb, which puts all of it into a walkable Rojo
     KB with no model in the loop
     (`just test-integration-build-mechanical-rojo`).
     Two findings from it are queued below: link byte offsets on
     `survey.Link`, and the anchor-grammar guess the rebase map makes
     without them.
     **Bursts E+F landed 2026-08-13**: the two no-fallback seams —
     `internal/taxonomy` (stage 3's container descent, composed as a
     fold over one serial lane) and `internal/summarize` (stage 6's
     four level-sliced stages, per-child-kind inputs, JSON summary
     artifacts) — plus the live path through the build verb, which runs
     both against a real provider and delivers a Rojo KB with a
     model-designed tree and real conclusions blocks. Both definitions
     are marked stubs. Two
     findings from the burst: page granularity is capped at a
     document's top-level sections (the landed coverage gate wants
     every surveyed section inside ONE group span, so a section cannot
     be descended into — an oversized one is split mechanically), and
     a container the answer above it placed on a page still spends its
     enumerated call, whose answer is then discarded.
     **Verb surface landed early 2026-08-14, by ruling**: `dev-build`
     became `kbase build` — the delivered verb, which is what the
     integration recipes now drive, because "integration tests test what
     we deliver". `--budget` and `--build-date` are gone from the flag
     set (budgets are §9 constants; the date is `[dev] build_date`), the
     offline shape moved to `[dev] tree_plan = "mechanical"`, and
     `--annex` landed as the §2.8/F-13 declaration seam. The
     burst-H item this belonged to keeps only its remaining half: the
     full-E2E evidence run (resume forensics, telemetry,
     `cached_tokens` slot-order measurement), still post-MAD #2.
  4. **MAD #2 = the standing E2E gate below** (design + implementation
     + generated Rojo KB + exemplar comparison).
- ~~Stage 4 remainder~~ **resolved 2026-08-14**: the fold is wired into
  `kbase build` and `kbase dev-refine` is deleted with it (Cruft II — the
  same antipattern the dev-build elimination ruled on). A live build's cuts
  stage is `dissect.StagePlan`, one lane per split group, and the light tier
  is now required for a live build exactly as the heavy one is; under
  `[dev] tree_plan = "mechanical"` the stage stays `dissect.Split`'s output.
  A group that fits one page has no boundary and costs no call, which the
  run record states as `boundariesAdjudicated: 0`. The refinement definition
  text remains a marked stub (`dissect.stubDefinition`) for the
  embedded-definitions item below.

## Later (build order exploits determinism-first)
- **MAD #2 gate (re-ruled 2026-08-13): fires on the complete v0.1
  artifact** — a tree with model-grouped taxonomy AND summaries that
  should in theory be usable/navigable (stub-definition prose quality is
  explicitly post-v0.1 tuning territory; the charter says so, so
  reviewers judge shape and navigability, not prose). Velocity path
  (ruled 2026-08-13): C+D combined (mechanical spine → walkable Rojo KB
  with empty summaries), then E+F combined (taxonomy + summaries, live,
  stub definitions) → v0.1 → MAD #2. Stage-7 review/regen, definition
  tuning, and the full-E2E evidence run (resume forensics, telemetry,
  `cached_tokens` slot-order measurement) come AFTER, informed by the
  review. Per-burst reviews collapse into one batched review of the
  C–F delta once the KB is real.
- Embedded prompt/agent definitions + the family-tuning eval harness.
  Derive role knowledge from `.claude/agents/kb-*.md` as referents (process
  phasing, leaf-fidelity rules, review adversarialism) — not ports: kbase
  definitions are gemma-4-narrow, CRITICAL-sectioned, embedded, evaled.
  Three rules carried from the dissection review (2026-08-11), each about
  wording rather than mechanism, which is why they wait for this burst:
  - **Say it once per channel.** The refinement stub states "answer with one
    number and nothing else" four times: the `## CRITICAL` section, the
    trailer's automatic re-render of it (§7's dual render, by design), the
    task definition, and a per-call acceptance criterion. The fourth is not
    free — acceptance criteria share a word-capped reserved share with the
    machine-generated retry note, so a redundant criterion competes with
    the retry feedback that has to fit beside it.
  - **Echo verification.** Have the answer be the number *plus* the entry text
    it names, and let the verifier check the two agree. That kills silent
    mis-mapping at the source rather than at the parser — worth an eval
    against the strict-number rule, which is the cheaper form of the same
    protection.
  - **See more than you may touch.** The display window may be allowed to grow
    beyond the clamp: the model judges a boundary better with more context
    than it is allowed to move within. The clamp is unchanged — this is about
    what is shown, not what may be chosen.
- Hierarchical summaries and review stages (distillation, link generation and
  the verify gates landed with bursts C+D).
- **Link byte offsets on `survey.Link`** (finding, bursts C+D). §4.5 asks
  stage 5 to rewrite link targets inside a page's bytes; the survey artifact
  carries each destination as written, its resolution and its fragment, but no
  offsets, and it collapses repeats of one target to a single entry. So
  `internal/distill` ships its own destination SCANNER (`distill/scan.go`) —
  two hard-coded Markdown facts (`](…)` and `[label]: …`), fenced blocks
  skipped, and no interpretation of what it finds. The adapter already knows
  the offsets (goldmark hands them over). Emitting them is a `kbase.survey/3`
  change, after which that file is deleted and `Destinations` becomes a read of
  the artifact. Not taken unilaterally in C+D because a schema bump is a shape
  decision.
- **The anchor grammar the rebase map guesses** (finding, bursts C+D). §4.5
  rule 1 lands a `#fragment` on the page hosting the section it names, and
  matching a fragment to a section needs the heading→anchor convention of
  whatever renders the source site — which no artifact states and kbase cannot
  learn. `distill.anchorSlug` implements the common one (lowercase, strip,
  hyphenate) and a miss falls through to rule 2, so being wrong costs a hop of
  precision and never a broken link. Revisit if a corpus shows the fallthrough
  is routing badly.
- **Definitions-extraction burst (ruled 2026-08-15)**: the shipped agent
  definitions leave the delivered KB. They are samples for the USER's agent
  tooling, not KB content — so they become an on-demand verb
  (`kbase write-agents --out <dir>`, name ruled 2026-08-16) with overwrite
  REFUSAL: if any target file exists, refuse and list every conflict, write
  nothing; no `--force` — the user deletes what they mean to replace.
  ~~`routing-eval.json` leaves delivery with them~~ pulled forward, landing
  2026-08-15 with the run.json→temp-work move: it is a dev-grade testing
  artifact (empty until stage 7.4; consumer is the eval harness), and the
  "user validates adapted agents" story that argued for shipping it needs
  shipping-grade data — a commitment explicitly not yet made. Its future
  home is build evidence beside the run record, decided when the eval
  stage lands. Class-B manifest shrinks to CONVENTIONS.md, README.md, CLAUDE.md;
  the `.agents/` directory, its check-4 exemption, and the entry-point/
  CONVENTIONS.md pointer text go with it. Prefer landing BEFORE the batched
  C–F review / MAD #2 so reviewers judge the lean manifest, not the
  superseded one.
- **ModernCorp rebuild-and-compare (ruled 2026-08-17)**: convert the
  exemplar's LaTeX source (../ModernCorp, entry ModernCorp.tex) to
  Markdown via an external tool as a ONE-OFF bridge (pandoc-class;
  consistent with the parked converter module's "dev-time differential
  oracle" role — the shipped tex→md module stays parked), build a KB from
  the converted tree with kbase, and compare against the exemplar
  (mad-reference snapshot) — which was built by the nondeterministic
  agent-set prototype, so the comparison is appliance vs agent-swarm:
  the win condition is comparable quality WITHOUT the run-to-run
  variance ("eliminate the box-of-chocolates factor"), plus the gates,
  provenance and run record the agent set never had. Ground truth for
  conversion loss: what the agent-built KB preserved from the same
  LaTeX. Path details to figure out when
  taken: conversion recipe (prep-recipe shape, tool-guarded), macro
  coverage losses measured not assumed, corpus root + entry doc,
  comparison methodology (MAD-style vs walk-notes). Prerequisite:
  descent (a LaTeX tome is the deep-nesting shape). Natural slot: beside
  or after the temporal.io shakedown.
- glimmer-30B head-to-head, on the eval harness the embedded-definitions item
  above builds (challenger test post-E2E; a tie counts as a win for the
  incumbent; low-cost-cloud availability check first). It has no harness of
  its own any more: `dev-refine` carried the A/B switch and went with the
  verb, and a `[dev]` key to bring it back would need an upstream cause the
  eval harness is the answer to.
- Rojo v7 end-to-end shakedown → creator-docs prose domains. Includes the
  slot-order measurement: validate or reverse the StageRef-before-status swap
  via `cached_tokens` + dev-telemetry prefill timing.
- **Docset title inference** (queued 2026-08-15). `--title` and the
  directory-name fallback are the floor, not the answer: most docsets carry
  something that states the whole set's title — a root README/index H1, a
  site-config file, a cover page. Design a sourcing chain that reads the
  corpus's own signals before falling back, with `--title` staying as the
  override. Rojo is the instructive counterexample: per-section frontmatter
  `Title` fields but no cover sheet for the set — so the chain must know
  when it has NO whole-set signal and fall back honestly rather than promote
  a section title. Ruled 2026-08-15: resolving LATE is fine — after the
  system holds the full inventory and anatomy of the file set, where the
  signals are all in hand and the title's render surface is smallest. Note site-config files (mkdocs.yml etc.) are outside the
  ingested extension set; reading one for a title is a deliberate ingest
  question, not a free read.
- Provenance receipt writer; chars-per-token calibration feature.
- Author CONVENTIONS.md's TBD sections (Testing, Dependency Policy, Logging) as
  their subjects accumulate enough reality to document.

## Parked

- Community KB share layer — design captured in SHARE_DESIGN.md; post-v1.
- MCP serve mode — hold until evidence agents fumble the CLI.
- **`tex→md` converter module** (reshaped 2026-08-12, ARCHITECTURE §4): a
  SEPARATE module with its own `go.mod` — filesystem in, filesystem out, no
  knowledge of kbase — whose Markdown output kbase ingests through the normal
  front door. It replaces the withdrawn native `internal/survey/latex`
  adapter, and it takes the "26B translation for non-Markdown formats"
  distillation variant with it: conversion is deterministic, so leaves stay
  mechanical in every format. Contract in §4; pandoc is a dev-time
  differential oracle, never a shipped dependency.
- PDF preprocessing-adapter slot. **Early probe path (noted 2026-08-14):**
  oMLX serves MarkItDown as a model (already visible in reaper's catalog),
  so PDF→markdown is reachable through the provider API we speak — a
  cheap evaluation of whether PDF corpora yield worthwhile KBs before any
  docling-class integration. Shape when taken: small dev-convert verb
  through the existing client (no ad-hoc scripts, key hygiene standing),
  conversion stamped (PDF hash + converting model id), determinism
  verified empirically (the wrapper, not MarkItDown itself, is the
  unknown), output tree fed to `kbase build`. Extraction-grade caveat:
  a negative result indicts the extractor, not the corpus class.
- i18n hardening (ruled 2026-08-13, indefinitely parked; slug half
  RESOLVED same day by verbatim-bytes slugs + NFC custody): remaining
  item is spaceless-script word caps failing open (CJK/Thai — byte/token
  fallback cap needed). Offsets/slicing are rune-safe by construction
  and the whitespace tripwire catches mid-rune cuts as defects. Revisit
  only with global distribution and a team to feed.
