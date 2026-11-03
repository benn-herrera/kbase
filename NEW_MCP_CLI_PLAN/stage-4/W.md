### W — served text (expert; `README.md` by tech-writer)

**Design:**
- [§3 Operations](../../NEW_MCP_CLI_DESIGN/3-operations/index.md) — the commands and tools the served text names
- [§4 Served text](../../NEW_MCP_CLI_DESIGN/4-served-text/index.md), [§4.1](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md), [§4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md), [§4.3](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md), [§4.4](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md), [§4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md) — instructions, help, server modes, CLI-only commands, resources

Relies on stage 2: the four help homes (Rule 12).

The work is two dispatches.
- A coder dispatch takes the scope below, all but `README.md`.
- A tech-writer dispatch takes `README.md`, after the coder's commit has landed. `README.md` is user
  documentation, read by people and agents. The coder dispatch does not edit it.
  - It is swept for every name the four stages moved. Their record is [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md)'s
    `<release version>` bullets ([SPEC.md:2402](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md) on, up to the first released section), which every
    package added to (Rule 6), and [page §3](../../NEW_MCP_CLI_DESIGN/3-operations/index.md)–[§4](../../NEW_MCP_CLI_DESIGN/4-served-text/index.md).
  - It is checked against [page §4](../../NEW_MCP_CLI_DESIGN/4-served-text/index.md)'s served text.

**Scope:**
- `internal/mcp/instructions.go`, `internal/mcp/instructions.txt`;
- `internal/kbdocs/templates/*`, `internal/kbdocs/kbdocs.go` (`SessionRule`, `IndexRule`);
- `internal/mcp/server.go` (`Located`, `ProjectRequired`), `internal/mcp/mcp.go`
  (`projectDirDescription`, `inProject`);
- `cmd/mcp.go`, `cmd/main.go` (the root help);
- the help homes (Rule 12) of every command whose [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) text this package writes;
- `test-data/fixtures/mcp/initialize.jsonl`, `gui-project-dir.jsonl`, `help.jsonl`;
- `README.md` (tech-writer dispatch only).

**Facts acted on:**
- [Page §4.1](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md): the eight instruction items, with their quoted wording.
- [Page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md): the notes by command and the shared bodies by command.
  - Each command's own package wrote its [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) body and notes (Rule 12). This package writes
    those that no earlier package wrote. Those an earlier package wrote are checked against the
    page and not rewritten; a mismatch is reported in the Commit body.
- [Page §4.3](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md): two modes, and a server that locates nothing serves as `--project-dir-required`.
- [Page §4.4](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md): the CLI-only commands, named in the root help and `mcp`'s help.
- [Page §4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md): the resource table.
- Today:
  - the instructions are `instructions.txt` ("The kbase skill carries the workflow.", `:1`) plus
    `IndexRule` and `SessionRule` (`internal/mcp/instructions.go:34–49`);
  - a server that locates none serves `rules` with no project-dir sentence (`:39–41`), the third
    mode that [page §4.3](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md) ends.
- `git grep -l 'ServedInstructions\|ProjectRequired()\|LocatedAt\|kbdocs\.SessionRule\|kbdocs\.IndexRule' -- '*.go'` →
  - `cmd/{init_test,mcp,mcp_test,score_tool_test}.go`;
  - `internal/mcp/{instructions,server}.go`.

**Contract updates:**
- [SPEC §11](../../PROJECT_DOCS/SPEC/11-the-mcp-server.md) and [§3 "Stamped documents"](../../PROJECT_DOCS/SPEC/3-compatibility-contract/stamped-documents.md), citing [page §4](../../NEW_MCP_CLI_DESIGN/4-served-text/index.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.
- [ARCHITECTURE §3](../../PROJECT_DOCS/ARCHITECTURE/3-principles.md)'s "The served surface is the operating contract", wherever it names a layer the
  page moves.

**Tests:**
- The transcripts pin the instructions in each mode.
- A server under no repository lists `project-dir` as required.
- The root help carries the edge-direction sentence from the same stored string as the
  instructions.

**Done-line:** coder dispatch, after Rule 13's two: `just test-integration`. Tech-writer dispatch:
`just edit-gate`.
