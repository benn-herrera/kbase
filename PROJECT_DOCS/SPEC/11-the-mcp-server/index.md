## 11. The MCP server

`kbase mcp` serves kbase to any external agent harness that speaks the Model Context Protocol, over stdio JSON-RPC. It exposes the subcommands: every subcommand it serves is a tool of its own, an operation of the `query` tool, or, for a write op, an entry of the `write` tool's batch, with the same arguments and the same result, so an external agent harness without MCP loses nothing but convenience. No tool is listed without a subcommand behind it.

Contents:

- [§11 Tools](tools.md)
- [§11 Two modes](two-modes.md)
- [§11 Resources](resources.md)
- [§11 Served text](served-text.md)
