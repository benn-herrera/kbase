#### Tool table

**Pattern:** Generated API surface from a command registry.

`mcp.Tools` builds one tool per command declaring `kbase.mcp` = `bound` (`mcp.Bind`); `mcp.Exclude`
declares `excluded` with a reason. `mcp`, `wait`, `auth` and cobra's own `completion` and `help`
declare `excluded`. The table reads the root's commands only (`boundCommands`), so a parent's
`excluded` keeps its subcommands out. `cmd/surface.go` adds every command to the tree, one `AddCommand` per group, its
declaration on that line (`readOnlyTool`, `readWriteTool`, `cliOnly`; for a family tool's operation
`queryOperation` or `writeOperation`, which add `kbase.mcp.family`). It also declares the MCP-only
interfaces: the resource families and the detached launches. It names no project for them:
`mcp.Server.Serve` binds the resources to the server's default project.

- **Read-only.** A command's read-only standing is the cobra annotation `kbase.readOnly`
  (`mcp.ReadOnly`), read when the table is built. A family tool is read-only where every member
  declares it.
- **Families.** Commands declaring one family (`kbase.mcp.family`, set by `mcp.InFamily`) are one
  tool together, in the place of the first. A family is an `mcp.Family` its caller hands it (name,
  description sentence, shared help): `query` (`toolFamilies`, `cmd/surface.go`). A family a `bound`
  command is named for is that command's, its members having no tool of their own: the write ops
  declare the family `write`, and the `write` tool is `kbase write`'s (`mcp.batchTool`), its schema
  the batch (`entry`, a required array of objects; `dry-run` and `commit`, booleans), its arguments
  handed whole to `Calls.WriteOp` as `write.Batch`'s values document. `runWriteOp` runs the batch
  with `ArgumentEntry` set: each refusal keeps its `entry`, and one naming no `path` drops `line` and
  `column`, which would point into a document the caller never wrote, while one naming a document
  keeps its line there. A refusal's `key` is the values key, which no interface formats differently.
- **Command tool schema.** `mcp.CommandSchema` derives it from the `Use` line's positionals and the
  command's flags, skipping a flag annotated `kbase.mcp` = `excluded` (`mcp.ExcludeFlag`), which is no
  argument of the tool: `score`'s `--replay` and `--failed` are such flags, and `internal/mcp/protocol`
  refuses an argument naming one as outside the schema. `required` is the positionals, less an
  optional variadic one that the `Use` line writes `[<name>...]`, plus each flag declared with
  `mcp.RequireFlag`, which is `trace`'s `direction` alone. `direction` has no default because `in`
  (what bears on the node) and `out` (what the node reaches) are opposite questions, equally likely,
  with nothing to break the tie. A transitive walk over both answers both, but doubles the output on a
  guess at what was asked; requiring the caller to name the question is the plain interface. `edges`,
  one hop, can list both with each row naming its end. `wait`'s `--pid`, the one flag a verb
  requires without that declaration, is outside the derivation: `wait` declares `excluded`. No other
  bound command refuses for want of a flag, `score`'s `--pass` among them, a run walking both passes
  unless it is told otherwise.
- **Family tool schema.** `operation` is a string enum of the members' names in the tree's order, and
  `arguments` an object. Each member is built as its own command tool would be, and the family keeps
  it by name, unlisted. A call takes the member `operation` names and checks `arguments` against that
  member's schema with `protocol.Schema.Faults`, the same check `internal/mcp/protocol` runs on a
  tool's arguments, so no argument list is stated twice. A fault is a usage refusal document, one item
  per fault naming the argument, the member's arguments as `allowed`, and a remedy naming `allowed`
  for the argument names and the member's help resource for what it does. Arguments that fit run the
  member as its own tool would. A values op's argument list reads `write.OpVocabulary` (the entry's
  keys in vocabulary order, each with the type, required mark and description its definition in
  `internal/write/values.go` carries), naming no flag, so it is true on both interfaces.
- **Results.** Every tool but `write` runs the command's `RunE` with its flags reset and its output
  captured to a buffer. The buffer is the result text; decoded once more, the YAML document is the
  `structuredContent`; the document's outcome gives `isError` (`result.IsError`). A `RunE` starts from
  `invocationDir(cmd)`, never `os.Getwd()`: the project the call names over MCP, else `--project-dir`
  (which must be a git worktree root), else the working directory (`cmd/mcp.go`). Each tool's result is formatted for MCP and checked for an
  item keyed `--name` or `<name>` of its own arguments (`warnCLIKeys`, a warning only).
- **The tool set is asserted.** `TestMCPToolsAreTheCommandTree` derives its expectation from the
  annotations alone (a family's name for its members, a command's own otherwise) and asserts the tool
  set for equality, so a tool listed by hand fails. `build-trail`, the build's checkpoint-commit
  history, is `cmd/build-trail.go`, a command tool like `status` (`buildstore.BuildTrail`), so its
  schema, key order and refusal keys are derived as any command's are;
  `TestBuildTrailIsOneDocumentOnBothSurfaces` runs it at the command line and through the server and
  requires the two documents to agree. Every tool has a command behind it (I9).
- **Protocol revision.** `protocol.ProtocolVersion` is the one constant; an `initialize` naming
  another revision is answered with the server's, for the client to decide.

