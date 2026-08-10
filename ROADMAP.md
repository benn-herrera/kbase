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

- Mechanical splitter → boundary refinement → verification → dissector
  (ARCHITECTURE §5). **Carries the R-6 verifier primitives**, which the
  orchestrator burst did not land: §5's verification paragraph is design
  with no code behind it, and the cut stage needs cut-list tiling, the
  clamp, candidate-set membership and the whitespace-adjacency tripwire
  before it can verify anything. (`internal/survey/tiling.go` is survey's
  own unexported check over a different subject and is not this.)
  Also reconcile here (ruled 2026-08-10, leave-and-watch): `Agent`/`Call`/
  `CallRunner` stay exported though only the coordinator constructs them —
  unexport or keep based on this burst's actual consumer set.

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
