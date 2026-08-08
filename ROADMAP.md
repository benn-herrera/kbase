# ROADMAP — living task queue

Ordered intentions, not commitments. This file churns freely; design truth
lives in ARCHITECTURE.md / SPEC.md, per-burst execution detail in `TEMP_*.md`
plans (gitignored-by-habit, disposed after landing). Finished items are
deleted, not archived — git history is the log.

## Now

- **Context-management builder** (`internal/tokens`, `internal/prompt`) — plan
  complete in TEMP_PLAN_CONTEXT_MANAGEMENT.md, awaiting dispatch. Includes
  `{{name}}` interpolation, CRITICAL/REMINDER mechanism, churn-tripwire
  primitives.

## Next

- **Adversarial code review of the foundations** (architect + go-coder, with a
  security-lens pass over key handling). Deliberately lighter than MAD at this
  stage; MAD is reserved for the deterministic pipeline spine when it lands.
- **Tasking/orchestrator layer** — phase enum + allowed-operations matrix
  (ARCHITECTURE §7 layers 1–2), feeding the builder's `CheckStability`;
  buffer flush policy execution.

## Later (build order exploits determinism-first)

- Markdown ingest adapter + survey stage (no model needed — testable against a
  Rojo docs clone immediately).
- Mechanical splitter → boundary refinement → verification → dissector
  (ARCHITECTURE §5). **MAD review here.**
- Embedded prompt/agent definitions + the family-tuning eval harness.
- Distillation, hierarchical summaries, review stages; link generation;
  refresh/verify gates (kb_tools port).
- Rojo v7 end-to-end shakedown → creator-docs prose domains.
- Provenance receipt writer; chars-per-token calibration feature.
- Author AGENTS.md's TBD sections (Testing, Dependency Policy, Logging) as
  their subjects accumulate enough reality to document.

## Parked

- Community KB share layer — design captured in SHARE_DESIGN.md; post-v1.
- MCP serve mode — hold until evidence agents fumble the CLI.
- LaTeX ingest adapter; PDF preprocessing-adapter slot.
