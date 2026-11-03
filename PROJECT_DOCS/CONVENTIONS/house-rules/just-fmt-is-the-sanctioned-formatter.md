### `just fmt` is the sanctioned formatter

Running it to clear a gofmt alignment finding is not
an edit outside the tools.
- **The edit and write tools govern text changes, not filesystem operations.** A move is `git mv`, a
  deletion `git rm`, and staging either is part of the landing's set; the landing commits once.
- **An MCP tool is a subcommand's binding** (I9). The trap: a tool listed by hand, a description
  copied into a tool, or a schema typed beside the command's flags. Every command reaches the tree
  in `cmd/surface.go`, one `AddCommand` per group, wrapped `readOnlyTool`, `readWriteTool`,
  `queryOperation`, `writeOperation` or `cliOnly`, which set the cobra annotation `kbase.mcp` =
  `bound` or `excluded` and the read-only hint, the two operation wrappers adding
  `kbase.mcp.family` (`mcp.InFamily`): the command is an operation of that family's tool, not a
  tool of its own; `cliOnly` takes the command's reason as a required argument and records it as the
  annotation `kbase.mcp.reason`, and the tool-table test fails an exclusion whose reason is empty;
  cobra's own `help` and `completion` are declared `cliOnly` there too. A test fails an
  `AddCommand` outside that file. A command's subcommands are added in `cmd/surface.go` too; the
  binding is declared on the root's child alone, since the tool table reads no deeper. The table binds only `bound` commands, and the test derives its
  expectation from the annotations, never from a list — one tool per family named by a member's
  `kbase.mcp.family`, its `operation` enum those members, one per other bound command — failing a
  command that declares neither. A family operation's arguments are checked against its own derived
  schema (`mcp.CommandSchema`, `write.OpVocabulary`); the trap is an argument list typed into the
  family tool. What a family's operations share is never appended to a member's `Long`: the root
  help states the CLI's form and `kbase://help/<family>` the MCP form, from one text where they
  agree; the trap is CLI input shape (stdin, `--values`, a dashed flag) in a resource only MCP reads.
  No tool is listed without a command behind it: the tool-table test (`cmd/mcp_test.go`) pins the
  set to the `bound` commands alone, so a tool listed by hand fails it; the trap is reaching for a
  hand-built tool where a subcommand bound like any other serves, as `build-trail` is. A flag annotated `kbase.mcp` = `excluded`
  (`mcp.ExcludeFlag`) is not part of its command's tool schema: `mcp.CommandSchema` skips it.
  A command's help is four texts on its own `cobra.Command`: `Short` the summary, `Long` the shared
  body, and the annotations `kbase.help.mcp` and `kbase.help.cli` (`mcp.MCPNoteAnnotation`,
  `mcp.CLINoteAnnotation`) the notes, each stating only what differs on its surface
  ([page §4.2](../../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)). `internal/mcp` composes the three renderings from them — the tool
  description, `kbase://help/<command>` (`mcp.HelpResources`) and `--help` (`mcp.CLIHelp`, the
  root's help function) — and `TestHelpRenderingsAreTheirParts` holds each to exactly its parts;
  the trap is help text written anywhere else: into a resource, a description, the instructions,
  or a second copy of the body for one surface. Protocol tests are transcript fixtures under `test-data/fixtures/mcp/`, never
  strings asserted in code.
- **Dev-only diagnostics are compiled out of the shipped binary.** A diagnostic for developing kbase
  itself — today `score --replay` and `build --replay` — lives in files carrying `//go:build kbasedev`,
  code and tests alike; the untagged build is what users receive and it carries none of that code. What
  those diagnostics read is not dev-only: every build writes its trail, ledger and captures whatever
  built it. A developer opts in with `-tags kbasedev` (`just build`). The untagged code reaches it only
  through a hook its tagged file sets, which is nil without the tag. Its flags carry the flag-level
  `kbase.mcp` = `excluded` annotation, so dev and release builds serve one MCP tool table. SPEC states
  nothing of it; ARCHITECTURE does. The trap: an untagged reference to tagged code, which now breaks
  the default build — the one everyone runs — where under the old polarity it broke only the rare
  dist-tagged build. That is why `edit-gate` vets both polarities, and why `test-dist-no-replay`, in
  the hermetic omnibus, runs `TestDistBuildHasNoReplay`
  (`cmd/dist_test.go`, tagged `!kbasedev`) on the untagged build.
- **A command resolves its starting directory with `invocationDir(cmd)`; no `os.Getwd` in `cmd`
  outside that function.** The trap: `os.Getwd()` anywhere a verb's logic runs ignores the project
  an MCP call or `--project-dir` names. `cmd/invocationdir_test.go` enforces it.
- **Never enumerate what you will accept in a field kbase does not fill.** pandoc fills the
  reference macro type and Div classes; an author fills metadata keys. Read such a field whole and
  branch only on the values you must act on, passing everything else downstream under its own name.
  A table that must stay closed counts and reports its residue. A pattern or branch that enumerates
  accepted values leaves an unanticipated one no trace: a reference whose type the pattern did not
  list is read as no reference. An answer to one of kbase's own asks is not this case — that format
  is ours, and its parser stays strict.
