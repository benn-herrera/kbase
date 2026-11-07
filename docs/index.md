# kbase documentation

The [README](../README.md) holds what kbase is and how to install it. These pages hold the rest.

- [Building a KB](building.md): `kbase build`, its stages and refusals, resuming and monitoring a
  build, and creating an empty KB.
- [Maintaining a KB](maintaining.md): write ops, queries, scoring, locks, format migration, the
  claim-graph sheets, output and exit codes.
- [MCP](mcp.md): serving kbase to an agent harness: tools, resources and instructions.
- [Configuration](configuration.md): providers, models, credentials, `show-config`, `probe` and the
  global flags.
- [Development](development.md): the `just` recipes, releasing and the integration fixtures.

The project's contract documents are trees under `PROJECT_DOCS/`:
[THESIS](../PROJECT_DOCS/entry-point.md), [SPEC](../PROJECT_DOCS/SPEC/entry-point.md),
[ARCHITECTURE](../PROJECT_DOCS/ARCHITECTURE/entry-point.md) and
[CONVENTIONS](../PROJECT_DOCS/CONVENTIONS/entry-point.md).
