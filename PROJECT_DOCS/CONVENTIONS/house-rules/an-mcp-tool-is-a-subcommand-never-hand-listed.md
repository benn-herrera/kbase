### An MCP tool is a subcommand, never hand-listed

Single source of truth (I9): the tool table derives from the command tree, and a command's help is
four texts on its own `cobra.Command`. The mechanism is in
[ARCHITECTURE, Tool table](../../ARCHITECTURE/12-constants/mcp-server-spec-11/tool-table.md) and
[Help texts](../../ARCHITECTURE/12-constants/mcp-server-spec-11/help-texts.md). Pitfalls:

- A tool listed by hand, where a subcommand exposed like any other serves, as `build-trail` is;
  `TestMCPToolsAreTheCommandTree` (`cmd/mcp_test.go`) fails it.
- A schema typed beside the command's flags, or an argument list typed into a family tool.
- A description copied into a tool, typed into a schema beside the argument's brief
  (`mcp.Brief`), or a brief restating the full description.
- Help text written outside the four texts: into a resource, a description, the instructions, or a
  duplicate of the body for one interface. `TestHelpRenderingsAreTheirParts`
  (`cmd/helpresources_test.go`) holds each rendering to exactly its parts.
- CLI input shape (stdin, `--values`, a dashed flag) in a resource only MCP reads.
- What a family's operations share, appended to a member's `Long`.
