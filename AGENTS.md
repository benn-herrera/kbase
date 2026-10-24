# AGENTS – KBase

## Coding
- When accepting dispatched-agent work, follow CONVENTIONS.md `## Coordinator Policy`: audit the
  diff, not the report, before commit.

## Dispatch planning
- Every plan rates each dispatch **mechanical** or **expert**, and the rating decides the model: a
  mechanical dispatch runs on a stepped-down model, an expert one inherits the coordinator's
  model. Rate what the dispatch must decide, not how much it types: a package whose work is
  settled by the plan's own text — a rename, a transcription, a test that mirrors a stated rule —
  is mechanical; one that must choose between defensible options, diagnose behaviour, or write
  model-facing prose is expert.
- for opencode sessions:
  - dispatch on `reaper/Qwen3.8-Flash-Next`
  - A mechanical dispatch runs on local inference, so the global rule applies while it runs: one
    dispatched agent at a time, no fan-out. Since the coordinator is *not* local inference in our
    current configuration, the limit on coordinator activity does not apply.
- for claude code sessions:
  - use the normal model step-down rule
