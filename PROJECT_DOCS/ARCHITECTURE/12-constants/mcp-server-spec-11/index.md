### MCP server (SPEC §11)

**Pattern:** MCP server generated from the command tree; JSON-RPC over stdio; detached child plus
status polling.

`internal/mcp` builds the server from the command tree its caller hands it; `internal/mcp/protocol`
holds the wire. Nothing is listed by hand.

Contents:

- [§12 Tool table](tool-table.md)
- [§12 Help texts](help-texts.md)
- [§12 A call's project](a-call-s-project.md)
- [§12 Wire and resources](wire-and-resources.md)
- [§12 Detached build and score](detached-build-and-score.md)
