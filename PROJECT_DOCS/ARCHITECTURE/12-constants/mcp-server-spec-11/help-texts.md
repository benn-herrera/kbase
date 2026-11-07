#### Help texts

**Pattern:** Self-describing API.

Each command stores four help texts: `Short`, `Long`, an MCP note (`kbase.help.mcp`) and a CLI note
(`kbase.help.cli`). `internal/mcp/help.go` composes three renderings from them, each formatting its
markers for its interface through `mcp.Spelling` ([§9](../../9-result-emitter/per-interface-rendering.md)):

- the tool description (`Short`, the MCP note where `mcp.NoteEveryCall` marks the command, where
  the full help is), read on `tools/list`, whose schema describes each argument by its brief
  (`mcp.Brief`, [Tool table](tool-table.md));
- the help resource `kbase://help/<command>` (`Long`, the argument list, the MCP note);
- `CLIHelp`, the `--help` text the root's help function prints (the usage line, `Long`, the argument
  list, the CLI note `CLINote`, which is closed by the flags excluded from MCP and, below the root, a
  line naming the global flags).

`mcp.NoteEveryCall` marks one command, `probe`. A tool description carries the MCP note only where
the call's own result cannot deliver it in time: `probe`'s paid-call warning must precede the call,
while `build`'s and `score`'s detached-run facts arrive in their result (`pid`, `wait`).
`TestHelpRenderingsAreTheirParts` expects the note in `probe`'s description alone, so with every
command given a note a mark on any other command, or none on `probe`, fails it.

`ArgumentList` builds the argument list from the definitions a tool's schema reads: a command's
positionals and flags (`commandArguments`), a values op's `write.OpVocabulary`, the batch's
`batchSchema`, and the root's persistent and own flags, each interface typing a list and a mapping
its own way. `TestHelpRenderingsAreTheirParts` (`cmd/helpresources_test.go`) asserts, for every
`bound` command and with and without notes, that each rendering is exactly its parts. The
`measure-mcp-surface` target (`tools/scripts/measure-mcp-surface.sh`, with `TestMeasureMCPSurface`)
measures the always-read size over the release build into `surface.yaml`: the `tools/list` result
from a server under no repository (`tools-list-chars`) and from one with a default project
(`tools-list-chars-located`), the input schemas and the instructions; then, under `per-tool`, each
tool's share of the first. It is a measurement and gates nothing.

The root command's `Long` states the command line's form once each: `queryHelp`, `listQueryHelp` and
`valuesHelp` in `cmd`, the last closing on `batchHelp`, the write batch's contract. The `query`
family's `Help` is the MCP form for `kbase://help/query`, from the same text where the two agree
(`queryReads`, `queryAnswerHelp` and `listQueryHelp`, one marked text that each rendering formats for
its interface); the resource lists the members from the annotations the enum is built from, each
with its `Short` (`familyHelp`). `batchHelp` is also `kbase write`'s `Long`, so `kbase://help/write`
has the contract in full; `write`'s MCP note states only that the arguments are the batch and where
each op's help is. Every write op shares one MCP note and one CLI note (`writeOpCommand`). The
members' shared help is appended to no member's `Long`.

`mcp.ServedInstructions` is what `initialize` reports (`instructions.go`): its opening, which
differs by mode (the default project's state where there is one, the `project-dir` lines where
there is none) and the sentence naming the skill; then `kbdocs.IndexRule` and `kbdocs.SessionRule`,
which the stamped `AGENTS.md` contains in its `index-rule` and `session-rule` slots; then
`mcp.EdgeDirection`, which the root's `Long` contains too; then the paragraphs only the instructions
have.

