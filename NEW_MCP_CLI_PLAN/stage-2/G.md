### G — help: one shared body plus notes (expert)

**Design:**
- [§3 Operations](../../NEW_MCP_CLI_DESIGN/3-operations/index.md) — the surfaces each command has, and where each op's help is
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and their
  renderings

**Scope:**
- `internal/mcp/help.go` (`HelpResources`, `commandHelp`, `familyHelp`);
- `internal/mcp/mcp.go` (`Tools`: the tool description; the annotation constants beside
  `BindingAnnotation`, `:43`);
- every `cmd` production file that builds a command: `git grep -l 'cobra.Command' -- 'cmd/*.go' ':!*_test.go'`
  → 52 files at `68021ba`. A command's help is set in its own file or passed to a builder:
  - `writeOpCommand(op, short, long)` (`cmd/write_op.go:159`), which closes `Long` with
    `writeOpLong` and `entryKeysHelp` (`:149`, `:129`), for the 18 write ops and `render-citation`;
  - `maintenanceCommand` (`cmd/refresh.go:239` at `68021ba`; in `cmd/update-format.go` since stage
    1), `monitorCommand` (`cmd/status.go:13`) and `queryCommand` (`cmd/query.go:129`);
  - a `Long:` field in the remaining files.
- `cmd/helpresources_test.go`;
- `tools/scripts/test-integration-mcp.sh` (the help comparison).

**Facts acted on:**
- [Page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md):
  - four stored texts per command: summary, shared body, MCP note, CLI note;
  - three renderings;
  - "A test asserts that each rendering is exactly its parts".
- Today:
  - the help resource is cobra's `--help` text (`commandHelp`, `internal/mcp/help.go:74–81`);
  - the tool description is `cmd.Short + ". Full help: " + helpURI(...)`
    (`internal/mcp/mcp.go:218`);
  - `TestHelpResourcesAreTheBinarysHelp` (`cmd/helpresources_test.go:22`) compares the resource with
    `--help`, run by `tools/scripts/test-integration-mcp.sh:136–164`.
- `git grep -l 'commandHelp\|HelpResources' -- '*.go'` → `cmd/helpresources_test.go`,
  `cmd/mcp_test.go`, `cmd/surface.go`, `internal/mcp/help.go`.
- The four homes, on each command's own `cobra.Command`:
  - the summary is `Short`;
  - the shared body is `Long`. A write op's body is `writeOpLong`'s composition, its `entryKeysHelp`
    line included, since that line "names no flag, so it holds on the command line and over MCP
    alike" (`entryKeysHelp`'s doc, `cmd/write_op.go:127–128`);
  - the MCP note and the CLI note are two annotations, `kbase.help.mcp` and `kbase.help.cli`,
    declared as constants in `internal/mcp/mcp.go` beside `BindingAnnotation` (`:43`).

  A later package finds them by `git grep -n '"kbase.help.mcp"\|"kbase.help.cli"' -- internal/mcp`.
- cobra's generated blocks. No command sets a help or usage template:
  `git grep -n 'SetHelpTemplate\|SetUsageTemplate\|SetHelpFunc\|SetUsageFunc' -- '*.go'` → nothing.
  So `--help` prints `Long` and then cobra's default `Usage:`, `Flags:` and `Global Flags:` blocks,
  as `just measure-mcp-surface`'s capture of every `--help` shows
  (`run/served/cli-help/kbase.<command>.txt` under `test-data/transient/measure-mcp-surface/`).
  - Those blocks are the CLI's statement of a command's arguments. [SPEC.md:817](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/queries.md) at `68021ba`:
    "`kbase <query> --help` states its arguments"; [SPEC.md:532](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/item.md) spells a refusal's `key` "as
    `--help` spells it". MCP's counterpart is the tool's schema, which [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) keeps beside the
    description ("A tool description and its schema carry what every call of that tool needs").
  - So `kbase <command> --help` prints the shared body, then the CLI note, then cobra's generated
    usage block (`cmd.UsageString()`), which is generated from the flags and is none of the four
    stored texts.
  - `kbase://help/<command>` is the shared body and the MCP note, with no usage block.
  - The parts test asserts each rendering against exactly that composition.
- Today's texts move unchanged: `Short` stays the summary, `Long` the shared body, and both notes
  start empty. This package moves only the mechanism and the texts' homes. Each later package that
  adds or changes a command writes that command's [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) body and notes (Rule 12).
- The help resource loses cobra's usage block, so `resources.jsonl`'s help reads move (Rule 4), and
  [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md)'s bullet "Every command's help is an MCP resource, … the text its `--help` prints"
  ([SPEC.md:2404](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md)) is corrected (Rule 6).

**Contract updates:**
- [SPEC §11](../../PROJECT_DOCS/SPEC/11-the-mcp-server.md): the help resource is the body plus the MCP note, citing [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): the corrected bullet.
- ARCHITECTURE: the §11 `internal/mcp` entry ("cobra's own help rendering, `--help`'s text").
- [CONVENTIONS "House Rules"](../../PROJECT_DOCS/CONVENTIONS/house-rules/index.md): the help trap at [CONVENTIONS.md:181–184](../../PROJECT_DOCS/CONVENTIONS/house-rules/just-fmt-is-the-sanctioned-formatter.md) ("rendered by cobra's help
  function from the command itself, so it is what `--help` prints").

**Tests:**
- The assertion that each of the three renderings is exactly its parts, for every bound command,
  the `--help` rendering's usage block taken from `cmd.UsageString()`.
- The help comparison in `test-integration-mcp` checks that the resource and `--help` carry the
  same shared body, each followed by its own note.

**Done-line** (after Rule 13's two): `just test-integration`.
