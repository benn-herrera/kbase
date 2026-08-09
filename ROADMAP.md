# ROADMAP — living task queue

Ordered intentions, not commitments. This file churns freely; design truth
lives in ARCHITECTURE.md / SPEC.md, per-burst execution detail in `TEMP_*.md`
plans (gitignored, disposed after landing — see AGENTS.md "Plan & Execute
Process"). Finished items are
deleted, not archived — git history is the log.

## Now

- **Small follow-ups from the foundation review's confirmation pass**: a
  cmd-level test of the --log-level/--log-file wiring path (Execute-based);
  resolve `prompt.CriticalWordCap` naming vs the enforced authored cap;
  stream-reader micro-items (arm idle timer at first Next, route decode
  errors through terminalErr); `expectedFrontier`'s hand-maintained slot
  list. All small; one agent, one burst.

## Next
- **Tasking/orchestrator layer** — the agent framework, where "agent" is a
  **Go-side construct** (ruled 2026-08-09): a role = embedded definition +
  slot-built context + one-shot LLM calls; never an LLM-driven tool-calling
  loop. Each pipeline step is mechanical where possible, LLM execution
  reserved for what can't practically be done any other way. Pieces: agent
  lifecycle; parent↔child communication only (no peer↔peer — matches the
  map-reduce shape); coordinator that spawns sub-agents for tasks; file
  read (full or offset+length) and file write as **orchestrator-side
  deterministic capabilities** feeding the content buffer / landing
  verified outputs — never LLM-callable tools. Plus phase enum +
  allowed-operations matrix (ARCHITECTURE §7 layers 1–2), feeding the
  builder's `CheckStability`; buffer flush policy execution. Carries from
  the foundation review:
  decide per-worker vs global stability frontier under stage-5 fan-out;
  the phase matrix must become the single source the prefix/stability tests
  derive frontiers from; RefA flush-at-section-transition must land here
  (precondition for the slot-order swap staying safe); dev-telemetry
  emission (`[dev] telemetry` switch exists; wire `TelemetryLogDetail`
  through internal/log at the first pipeline call site).

## Later (build order exploits determinism-first)

- Markdown ingest adapter + survey stage (no model needed — testable against a
  Rojo docs clone immediately). Parser: goldmark (CommonMark, maintained,
  AST segments carry byte offsets into source — exactly what cut lists and
  the heading-tree survey need).
- Mechanical splitter → boundary refinement → verification → dissector
  (ARCHITECTURE §5). **MAD review here.**
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
