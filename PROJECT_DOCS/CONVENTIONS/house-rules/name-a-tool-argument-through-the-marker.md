### Name a tool argument through the marker

Text an MCP reader can receive names a tool argument through the marker (`internal/argmark`), and
a kbase command line in it is a call held as an arguments mapping; a refusal's key is built by the
constructor. Never wrap a marker in backticks or quotes at the source. The mechanism is in
[ARCHITECTURE §9, Per-interface rendering](../../ARCHITECTURE/9-result-emitter/per-interface-rendering.md).
- **Never write `--name`, `<name>` or `kbase <command>` literally for an argument or a command a tool
  takes.** A literal stays literal over MCP. It stays where no MCP rendering reads the text — a CLI
  note, the root's `Long`, a `cliOnly` command, the `kbasedev` replays — and for a flag no tool's
  schema carries, the server's own `--config-dir`, `--log-level` and `--log-file` among them.
- **Never infer the marker from a shape.** `<id>` in `run-<id>` or `<provider>/<model>` is a
  placeholder and stays as written; only the author knows which angle brackets name an argument.
- **A test compares an item against the marked form,** built with the constructor; a test reading
  an emitted CLI result compares against `--name`. `TestNoCLISpellingInMCPRenderings`,
  `TestMarkedArgumentsRenderOnBothSurfaces` and `TestEveryMarkerNamesAnArgument`
  (`cmd/argspelling_test.go`) fail on a help text that misses or misnames one; the server logs a
  warning for a result item keyed `--name` or `<name>` of its tool's own arguments.
