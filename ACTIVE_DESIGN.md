# Design: open issues on the operation surface

The surface's target shape is `NEW_MCP_CLI_DESIGN/`, and the work landing it is `NEW_MCP_CLI_PLAN/`.
What remains open here is outside both.

## Presentation (part two, after the surface is settled)

- The always-read cost today is `tools/list` at 14,689 chars, of which input schemas are 11,535,
  plus the instructions at 620.
- About 5.9K of the schema total is one paragraph: `project-dir`'s ~420-char description,
  repeated in all 14 tools. Candidate: state its meaning once in the instructions, and give each
  tool's property a one-line description.
- Each tool carries that per-tool floor, so fewer tools cost less.
- Tension: exact typed entry schemas per write op (the node ops split by kind) would grow the `write` tool's
  always-read schema, which is ~1.0K today and loose. The choice is between strict keys and types
  in the schema with prose in help, fully typed, or loose.
