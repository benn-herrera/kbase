### Per-interface rendering

**Pattern:** Per-surface presentation layer.

Text that names a tool argument is written once, in a marked form, and each interface formats it as it
takes the argument. Recorded output and help text are read through either interface, so they contain
the neutral form and the reader's interface formats it.

- **The marker.** `internal/argmark` is the single marker by which text names a tool argument (`Flag`,
  `Positional`, `FlagValue` with its value) or contains a kbase call (`Call`, with an arguments mapping
  whose values are JSON values, lists of mappings among them; `Recorded`, a recorded argv). A marker is
  delimited by two private-use code points, with its kind in the byte after the opening one. It imports
  no kbase package, so every producer of text an MCP reader can receive reaches it; `kb`, `config`,
  `result`, `statedir` and `log` use it.
- **Formatting.** `Spelling.Spell` formats each marker as one interface takes it. At the command line
  that is `--name`, `<name>` or `--name value`; over MCP, `name` or `name: value`. A call is formatted by
  the `Spelling.Call` its caller supplies, which `mcp.Spelling(root, overMCP)` builds from the command
  tree: over MCP, the call of the tool the command is exposed as (`toolCall`); at the command line, a
  `kbase` command line, with the batch of a command that reads one piped in on stdin. A call
  with no `Spelling.Call` stays marked. A marker inside other text is backticked where its formatting
  has no delimiter of its own (every one over MCP, a call at the command line); a marker that is the
  whole text stays bare. Text outside markers is unchanged.
- **Writers.** `result.Spelled(w, s)` wraps a writer so that every string `result.Write` puts on it is
  formatted by `s`, and so is any other text written on it, such as what cobra prints from a command's
  summary and flag usages. The root's stdout is such a writer for every subcommand but `mcp`, set in
  `PersistentPreRunE`; the MCP server wraps each tool call's output the same way. A writer already
  `Spelled` keeps its formatting, settled where the call came in (`resultOut`, `cliOut`).
  `result.Underlying(w)` is the writer a `Spelled` one wraps, else `w`: what a verb writing bytes that
  must not be reformatted, as `auth get-piped` writes a key, writes on. A result written on any other
  writer keeps the marked form.
- **`result.Verbatim`.** The KB text a result passes through (a title, a rationale, a context, a leaf's
  line, a conversion list's id and text) is a `Verbatim` and is written unchanged on every
  interface. Only text kbase composes is formatted, because KB text may contain the same code points
  and is the author's. `Items.Error` formats for the command line, since an error's text is read on
  stderr and in logs, and `internal/log` formats every line it writes the same way.
- **Recorded output.** The progress record's items, and a score run's stored result beside its
  ledger, contain the marked form, because each was written on a writer that formats nothing. `status` and
  `cancel` emit them again through `result.Emit` on the writer of the call reading them, so a recorded
  item and the recorded `Resume` argv reach each interface in its format; the argv goes through
  `buildstore.RecordedCommand`'s `argmark.Recorded` call and is read with `build`'s own flags into an
  arguments mapping where it is formatted. `wait` re-emits a score run's stored result through
  `result.Reemit`. The store's layout is unchanged: a marker sits inside the string fields, and
  `Resume` stays an argv.
- **Detached runs.** A run a tool call launched is told its result is read over MCP by `mcp.SurfaceEnv`
  in its environment, set by `Launch.start`. `cmd/main.go` reads it before `Execute` and wraps the
  process's stdout, so the run's own result, which the launch hands back where the run exits first, is
  the tool's.
- **Checks.** `cmd/argspelling_test.go` enforces the layer: no MCP rendering contains command-line
  formatting, no command-line output contains a marker, every marker names an argument, and a refusal's
  argument and a remedy's call are in the form of the interface that receives them.

---

