### Stamped documents

`README.md` gives the maintenance commands: kbase's subcommands. The stamped documents point a reader at kbase's tools, never at its records:
`AGENTS.md` forbids reading or writing anything under `.index/`, save testing that such a path
exists, and states the rule that a session sets a rigor or a bearing only to a value the user gave,
both as the MCP server's instructions do, in the same words ([page §4.1](../../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md) items 3 and 4, [§11](../11-the-mcp-server.md)). A register page is the one exception: kbase generates it as prose for people and
sessions to read, and nobody takes a value from it or edits it. `CONVENTIONS.md` states that the
records and the statement markers are written only by a write op, never by hand, and that no agent
edits the block kbase maintains in a stamped document. `AGENTS.md`, `CONVENTIONS.md` and
`README.md` are maintained-block documents ([page §1.7](../../../NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)): `update-format` rewrites each one's block from the
current templates and leaves the text outside it ([§8](../8-maintenance-subcommands/index.md)).
