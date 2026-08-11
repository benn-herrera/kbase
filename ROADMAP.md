# ROADMAP — living task queue

Ordered intentions, not commitments. This file churns freely; design truth
lives in ARCHITECTURE.md / SPEC.md, per-burst execution detail in `TEMP_*.md`
plans (gitignored, disposed after landing — see AGENTS.md "Plan & Execute
Process"). Finished items are
deleted, not archived — git history is the log.

## Now

- ~~`.mdx` decision~~ ruled 2026-08-09: **ignored** (ARCHITECTURE §4 ingest
  row) — watch what it actually costs; strip-and-scan only on evidence.

## Next

- Stage 4 remainder (the §5 chain itself landed 2026-08-10 as
  `internal/dissect`, mock-driven per that burst's R-1): wire it into a real
  job plan once the taxonomy skeleton says what the spans are, and fold the
  adjudicated per-boundary offsets back into one cut list for stage 5. Its
  refinement definition text is a marked stub (`dissect.stubDefinition`) for
  the embedded-definitions item below.
- Export watch resolved (2026-08-10): `internal/dissect`, the first
  out-of-package consumer, needs `CallRunner` (`NewCallRunner` returns it,
  `NewCoordinator` takes it) and never touches `Agent` or `Call` — both are
  still coordinator-internal, so unexporting them is now evidence-backed.
  Left for whoever is next in `internal/pipeline`; it is a rename, not a
  design question.

## Later (build order exploits determinism-first)
- **MAD review gate (ruled 2026-08-09): after the first end-to-end run
  that turns the Rojo docs into a KB of the intended shape, before any
  creator-docs scaling work.** Pre-E2E adversarial review is speculation,
  which is not where model review earns its cost; the per-burst
  architect/go-coder passes carry review until then. The E2E milestone's
  deliverables include the evidence artifacts the MAD review interrogates:
  the generated KB itself, a full run's results, the stepwise
  recovery/resume forensics (including from a deliberately interrupted
  run), and the telemetry/metrics data (`cached_tokens`, timing — which
  also settles the slot-order measurement).
- Embedded prompt/agent definitions + the family-tuning eval harness.
  Derive role knowledge from `.claude/agents/kb-*.md` as referents (process
  phasing, leaf-fidelity rules, review adversarialism) — not ports: kbase
  definitions are gemma-4-narrow, CRITICAL-sectioned, embedded, evaled.
- Distillation, hierarchical summaries, review stages; link generation;
  refresh/verify gates (kb_tools port).
- Rojo v7 end-to-end shakedown → creator-docs prose domains. Includes the
  slot-order measurement: validate or reverse the RefA-before-status swap
  via `cached_tokens` + dev-telemetry prefill timing.
- Provenance receipt writer; chars-per-token calibration feature.
- Author AGENTS.md's TBD sections (Testing, Dependency Policy, Logging) as
  their subjects accumulate enough reality to document.

## Parked

- Community KB share layer — design captured in SHARE_DESIGN.md; post-v1.
- MCP serve mode — hold until evidence agents fumble the CLI.
- LaTeX ingest adapter; PDF preprocessing-adapter slot.
