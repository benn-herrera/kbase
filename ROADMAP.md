# ROADMAP — living task queue

Ordered intentions, not commitments. This file churns freely; design truth
lives in ARCHITECTURE.md / SPEC.md, per-burst execution detail in `TEMP_*.md`
plans (gitignored, disposed after landing — see AGENTS.md "Plan & Execute
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
  4. **MAD #2 = the standing E2E gate below** (design + implementation
     + generated Rojo KB + exemplar comparison).
- Stage 4 remainder (the §5 chain landed 2026-08-10 as `internal/dissect`,
  mock-driven per that burst's R-1; the serial fold landed 2026-08-11 —
  boundaries are adjudicated against current state and the stage writes one
  composed, whole-list-`Verify`d cut list): what is left is **wiring it into a
  real job plan** once the taxonomy tree plan says what the spans are, which is
  also where a single-section span — no boundaries, so no calls and today no
  artifact — gets its answer. Its refinement definition text is a marked stub
  (`dissect.stubDefinition`) for the embedded-definitions item below.

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
- Distillation, hierarchical summaries, review stages; link generation;
  refresh/verify gates (kb_tools port).
- glimmer-30B head-to-head, on the `dev-refine` harness (challenger test
  post-E2E; a tie counts as a win for the incumbent; low-cost-cloud
  availability check first).
- Rojo v7 end-to-end shakedown → creator-docs prose domains. Includes the
  slot-order measurement: validate or reverse the StageRef-before-status swap
  via `cached_tokens` + dev-telemetry prefill timing.
- Provenance receipt writer; chars-per-token calibration feature.
- Author AGENTS.md's TBD sections (Testing, Dependency Policy, Logging) as
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
- PDF preprocessing-adapter slot.
- i18n hardening (ruled 2026-08-13, indefinitely parked; slug half
  RESOLVED same day by verbatim-bytes slugs + NFC custody): remaining
  item is spaceless-script word caps failing open (CJK/Thai — byte/token
  fallback cap needed). Offsets/slicing are rune-safe by construction
  and the whitespace tripwire catches mid-rune cuts as defects. Revisit
  only with global distribution and a team to feed.
