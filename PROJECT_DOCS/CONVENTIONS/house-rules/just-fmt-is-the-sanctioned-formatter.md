### `just fmt` is the sanctioned formatter

Running it to clear a gofmt alignment finding is not
an edit outside the tools.
- **The edit and write tools govern text changes, not filesystem operations.** A move is `git mv`, a
  deletion `git rm`, and staging either is part of the change's commit; the change is committed once.
- **An MCP tool is a subcommand exposed as a tool** (I9). Single source of truth: the tool table derives from the command tree. Pitfall: a tool listed by hand, a description
  copied into a tool, or a schema typed beside the command's flags. Every command reaches the tree
  in `cmd/surface.go`, one `AddCommand` per group, wrapped `readOnlyTool`, `readWriteTool`,
  `queryOperation`, `writeOperation` or `cliOnly`, which set the cobra annotation `kbase.mcp` =
  `bound` or `excluded` and the read-only hint, the two operation wrappers adding
  `kbase.mcp.family` (`mcp.InFamily`): the command is an operation of that family's tool, not a
  tool of its own; `cliOnly` takes the command's reason as a required argument and records it as the
  annotation `kbase.mcp.reason`, and the tool-table test fails an exclusion whose reason is empty;
  cobra's own `help` and `completion` are declared `cliOnly` there too. A test fails an
  `AddCommand` outside that file. A command's subcommands are added in `cmd/surface.go` too; the
  exposure is declared on the root's child alone, since the tool table reads no deeper. The table exposes only `bound` commands, and the test derives its
  expectation from the annotations, never from a list — one tool per family named by a member's
  `kbase.mcp.family`, its `operation` enum those members, one per other bound command — failing a
  command that declares neither. A family operation's arguments are checked against its own derived
  schema (`mcp.CommandSchema`, `write.OpVocabulary`); Pitfall: an argument list typed into the
  family tool. What a family's operations share is never appended to a member's `Long`: the root
  help states the CLI's form and `kbase://help/<family>` the MCP form, from one text where they
  agree; Pitfall: CLI input shape (stdin, `--values`, a dashed flag) in a resource only MCP reads.
  No tool is listed without a command behind it: the tool-table test (`cmd/mcp_test.go`) pins the
  set to the `bound` commands alone, so a tool listed by hand fails it; Pitfall: reaching for a
  hand-built tool where a subcommand exposed like any other serves, as `build-trail` is. A flag annotated `kbase.mcp` = `excluded`
  (`mcp.ExcludeFlag`) is not part of its command's tool schema: `mcp.CommandSchema` skips it.
  A command's help is four texts on its own `cobra.Command`: `Short` the summary, `Long` the shared
  body, and the annotations `kbase.help.mcp` and `kbase.help.cli` (`mcp.MCPNoteAnnotation`,
  `mcp.CLINoteAnnotation`) the notes, each stating only what differs on its interface
  (SPEC §11 Served text). `internal/mcp` composes the three renderings from them — the tool
  description, `kbase://help/<command>` (`mcp.HelpResources`) and `--help` (`mcp.CLIHelp`, the
  root's help function) — and `TestHelpRenderingsAreTheirParts` holds each to exactly its parts;
  Pitfall: help text written anywhere else: into a resource, a description, the instructions,
  or a duplicate of the body for one interface. Protocol tests are transcript fixtures under `test-data/fixtures/mcp/`, never
  strings asserted in code.
- **Dev-only diagnostics are compiled out of the shipped binary.** Build-tag gating: a diagnostic for developing kbase
  itself — today `score --replay` and `build --replay` — lives in files with `//go:build kbasedev`,
  code and tests alike; the untagged build is what users receive and it contains none of that code. What
  those diagnostics read is not dev-only: every build writes its trail, ledger and captures whatever
  built it. A developer opts in with `-tags kbasedev` (`just build`). The untagged code reaches it only
  through a hook its tagged file sets, which is nil without the tag. Its flags have the flag-level
  `kbase.mcp` = `excluded` annotation, so dev and release builds serve one MCP tool table. SPEC states
  nothing of it; ARCHITECTURE does. Pitfall: an untagged reference to tagged code, which breaks
  the default build — the one everyone runs. That is why `edit-gate` vets both tag settings, and why `test-dist-no-replay`, in
  the hermetic aggregate target, runs `TestDistBuildHasNoReplay`
  (`cmd/dist_test.go`, tagged `!kbasedev`) on the untagged build.
- **A command resolves its starting directory with `invocationDir(cmd)`; no `os.Getwd` in `cmd`
  outside that function.** Single point of access: `os.Getwd()` is called in one function. Pitfall: `os.Getwd()` anywhere a verb's logic runs ignores the project
  an MCP call or `--project-dir` names. `cmd/invocationdir_test.go` enforces it.
- **Never enumerate what you will accept in a field kbase does not fill.** Tolerant reader: pandoc fills the
  reference macro type and Div classes; an author fills metadata keys. Read such a field whole and
  branch only on the values you must act on, passing everything else downstream under its own name.
  A table that must stay closed counts and reports its residue. A pattern or branch that enumerates
  accepted values leaves an unanticipated one no trace: a reference whose type the pattern did not
  list is read as no reference. An answer to one of kbase's own model calls is not this case — that format
  is ours, and its parser stays strict.
