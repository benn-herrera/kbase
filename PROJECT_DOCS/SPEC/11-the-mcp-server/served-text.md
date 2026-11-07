### Served text

Served text is what a session reads without calling a tool: the instructions, the tool descriptions, the help resources and `kbase --help`. Where a text below is quoted, a compliant server serves that wording; every other item states content.

**Instructions.** `initialize` always has `instructions`, in each mode. They contain what spans the whole session or several tools, and stay short. They have eight items, in this order, and every tool they name is one the server serves:
1. **Where the project is.** With a default project: the KB's absolute path, or the repository's where `kb-root/` is yet to be built, saying the KB can be built. Without one: the project path is the absolute path of the project's git worktree root, the directory holding `.git`, where `kb-root/` is or will be; the session gets it from the user, by asking or by proposing one for the user to confirm, and uses it with `project-dir` on every call that session.
2. The `kbase` skill contains the workflow.
3. **The `.index/` rule.** `kb-root/.index/` has kbase's own records: never read or write anything under it. Testing that a path there exists (`test -d`, a stat) is allowed; listing, globbing or opening anything under it is reading it.
4. **The session rule.** Rigor and bearing values are judgements, and a session does not make them. Set a rigor or a bearing, its current value included, only to a value the user has given for that node or edge, in their own words or quoted in the task. When adding a node or an edge without one, leave the value out: kbase records it as pending. `score` judges the rigor of claims, analyses and experiments and the bearing of `supports`, `lifts` and `refutes` edges. A work's rigor and a `founds` bearing stay pending until the user gives one.
5. **Edge direction**: “An edge reads "source *relation* target", so "A supports B" means A's strength bears on B.”
6. **Commit**: "Pass `commit` on every write batch: it is the only way to make the batch undoable as one unit. It commits all of `kb-root/`."
7. **Sheets.** The claim-graph sheets are drawn for people. Offer them for display, as files in `kb-root/` or, where this server lists them, as `kbase://sheet/` resources. Never read one for information; the queries answer what a sheet shows.
8. **Keys.** A command kbase prints for handling a key, such as `show-config`'s `upgrade`, is run as printed. Never compose one from a key file's path, and never ask for a key or read a key file into the conversation.

Items 3 and 4 are the text the stamped `AGENTS.md` also contains ([§3](../3-compatibility-contract/stamped-documents.md)), byte for byte. Item 5 is the text `kbase --help` also contains, byte for byte.

**Help.** Each command has a one-line summary, one shared help body, an optional MCP note and an optional CLI note. The shared body is the same text on both interfaces: an argument has one name, which the command line writes `--name` (a positional `<name>`) and MCP takes bare, and a kbase call is written as the interface in use writes it, as a `kbase` command line or as the tool's call. A note states only what differs because the environment differs; no help text is written separately for each interface. Each text is rendered in three places:

| Rendering | Contents | Read |
|---|---|---|
| MCP tool description | the summary, where the full help is, and the MCP note only where the call's own result cannot deliver it in time | always (`tools/list`) |
| `kbase://help/<command>` | the shared body, the argument list and the MCP note | on demand |
| `kbase <command> --help` | the usage line, the shared body, the argument list, the subcommand list where the command has subcommands, the CLI note, and for a subcommand a line naming the global flags | on demand |

- The argument list is part of the help on both interfaces. It lists each argument's name, type and whether it is required or repeatable, with its description, which states the default where `--help` prints one. Each interface writes an argument as it takes it: the name as the body writes it; a repeatable argument as a list over MCP; a mapping as its CLI string at the command line; a shorthand only at the command line.
- The two help renderings differ only by their notes, that formatting, and the usage line (`kbase <command> [flags]`), which only the command line has. The descriptions are the same text.
- A query operation's argument list is its own, not the `query` tool's.
- A tool description and its schema have what every call of that tool needs. The help has the full rules, refusals and examples.
- A command tool's description, `write`'s included, is its summary, then its MCP note only where the call's own result cannot deliver it in time, then where its full help is, `kbase://help/<command>`. The `query` tool's description is one sentence, then where its operations' shared help is, `kbase://help/query`, which lists each operation with its summary, and where one operation's is, `kbase://help/<operation>`.

**MCP notes.** The content each note must have:
- `write`: that the arguments are the batch, `entry` listing the entries with `dry-run` and `commit` as its arguments, and where each op's help is (`kbase://help/<operation>`). The batch contract is the shared body's alone, in `kbase://help/write` and the root `kbase --help` ([§8](../8-maintenance-subcommands/write-batches/index.md)).
- `build` and `score`: the call returns once the run has started, as a detached process that keeps running without the server; the result's `pid`, `state-dir` and `wait`, and that its `warnings` is the configuration as read at launch, the run's own result carrying the run's own; `status` follows the run and `cancel` stops it.
- `probe`: "Each call is a paid model call: run it only when the user directs it, one call per direction, never on your own initiative or to retry."
- `update-format`: "Running it is the user's decision: run it only when the user directs it, a refusal's remedy included."
- a write op: its arguments are batch entries.
- any command whose MCP arguments differ from its CLI flags in a way the argument list's formatting does not show: the difference.

**CLI notes.** The content each note must have:
- `build` and `score`: they run in the foreground; SIGINT or SIGTERM (`kbase cancel`) stops them resumably.
- a write op: the batch arrives as YAML (JSON accepted) on stdin or in the file `--values` names; `--dry-run` and `--commit`.

**Obligations on help.**
- The root help and `mcp`'s help name the three CLI-only commands, `auth`, `wait` and `mcp`. The root help's overview names `score`, `wait`, `build-trail`, `init`, `export-latex` and `probe` among the commands.
- `write`'s help states that a committing batch commits the whole of `kb-root/`, and that a batch without `commit` has no safety net.
- `status` and `cancel` speak of any run, detached or in the foreground, naming `build` and `score` as its kinds.
- `remove-node`'s help says plainly that it removes every edge naming the node, and points to the dry run.
- `stats`' help points to `find`'s strength filters.
- `find`'s help says, in one line, that `min: 0` leaves out pending nodes.
- `update-format`'s help says that running it is the user's decision: a session asks the user before running it, a refusal's `kbase update-format` remedy included.
- The help of an op that takes a placement `snippet` (`add-<kind>`, `move-node`) recommends quoting a few distinctive words from the line, not the whole line.
- A help that tells a caller to copy an edge row from a result into `add-edge` or `edit-edge` gives this advice. A row's `manually-scored`, `replaced-relation`, and `trace`'s `depth`, `strength` and `contribution`, and any op key whose value is null, are read-only facts: drop them before copying the rest, except that from a `demoted` row, `replaced-relation` goes in as `relation`. Copy `bearing` only where the row is manually scored or the relation is `founds`: a bearing given marks the edge manually scored ([§10](../10-metadata-format-version/provenance-and-the-judged-hash.md)), so copying a model-judged one would pin it. Into `remove-edge` and `restore-demoted`, copy only `source`, `target` and `relation`.
- The `rationale` argument of a claim's or an analysis's add or edit says that what would strengthen the node is not its rationale but a graph question, answered by `trace` with `direction: in`, `sort: contribution` and `order: ascending`. A work's `rationale` says nothing of it: no scoring edge ends at a work.
- No help names a command or resource that this SPEC does not define.

