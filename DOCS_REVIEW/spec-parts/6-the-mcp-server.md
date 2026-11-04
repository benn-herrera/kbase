## 11. The MCP server

`kbase mcp` serves kbase to any external agent harness that speaks the Model Context Protocol, over stdio JSON-RPC. It exposes the subcommands: every subcommand it serves is a tool of its own, an operation of the `query` tool, or, for a write op, an entry of the `write` tool's batch, with the same arguments and the same result, so an external agent harness without MCP loses nothing but convenience. No tool is listed without a subcommand behind it.

### Tools

- **The tools** are `write`, `query`, `build`, `score`, `status`, `cancel`, `build-trail`, `verify`, `update-format`, `export-latex`, `init`, `configure-providers`, `configure-models`, `models`, `show-config` and `probe`. Every tool but `query` and `write` is one subcommand, named as the subcommand.
  - `query` is a **family tool**: one MCP tool with an `operation` argument that names one of several subcommands, each an operation of the tool. Its operations are the queries `citations-of`, `edges`, `find`, `show`, `stats` and `trace`.
  - `write` runs a batch whose entries name the write ops ([§8 Write batches](#write-batches)); a write op has no tool of its own.
  - Each operation stays a subcommand, unchanged on the command line.
- **CLI-only commands.** Every other subcommand is served. These are not, and no tool exposes them:
  - `auth` and its subcommands: a key arrives on stdin, which no tool call has, and no tool accepts a key;
  - `wait`: a tool call would hold the turn for the build's or score run's duration, so a session polls `status` instead;
  - `mcp`, the server itself;
  - `help` and `completion`, which belong to the CLI framework.

  The root help and `mcp`'s help name `auth`, `wait` and `mcp` as the CLI-only commands ([Served text](#served-text)).
- **A command tool's input schema** has one property per positional and per flag of its command, named as the command line names them without the dashes. The global flags are excluded, since the server has its own, except `project-dir` ([Two modes](#two-modes)). A duration flag's argument is the Go duration string the command line takes (`30s`). A flag is optional in the schema unless its command declares it required, as `trace`'s `direction` is. Each property has a description: a positional's is the one its command declares; a flag's is its usage, followed by the default `--help` prints for the flag where it prints one and, for a duration flag, by the note that its argument is a Go duration.
- **The `query` tool's schema** has `operation`, required, a string enum of its operations' names, and `arguments`, an object holding what the operation takes: its positionals and flags by name, derived as a command tool's schema is. `operation`'s description names where each operation's help is, and `arguments`' names that help's argument list. The tool checks `arguments` against the operation's own schema: an unknown key, a missing required key or a value of the wrong type is a refusal document, `refused`, one item per fault, `check: usage`, `key` the argument's name, `allowed` every argument the operation takes, and `remedy` naming `allowed` for the argument names and the operation's help resource for what the operation does. An `operation` outside the enum, or an `arguments` that is not an object, fails the tool's schema.
- **The `write` tool's schema** is the batch: `entry`, required, an array of objects, each naming its op under `operation` beside that op's keys; and `dry-run` and `commit`, booleans. The schema gives an entry's keys no types. The batch checks the entries' shape ([§8 Write batches](#write-batches)), every fault an item naming its `entry`, and a wrong value for a key is the op's own refusal (§8), which names the key and, where the value comes from a fixed set, lists that set as `allowed`.
- **Every tool's schema** has one more property, `project-dir`, the project the call runs in ([Two modes](#two-modes)).
- **Results.** A tool's result is the §7 document ([§7 Tool results](#tool-results)), as the result's text and as structured content. `isError` is set exactly when the document's exit code (§7) is not 0, so a refusal reaches the model as an error with its items and remedy, while `bounded` and a completed `cancel` do not. A refusal's items are defined in [§7 Item](#item), `line` and `column` included. Arguments that fail the tool's schema (a missing required key, a wrong type, an unknown key) are JSON-RPC `-32602` errors, not refusal documents; a `query` tool's `arguments` are checked as above instead. A list query answers completely, or is sliced by `limit` as [§7 Bounded results](#bounded-results) states.
- **Read-only annotation.** The read-only tools have the read-only annotation: `query`, `verify`, `status`, `build-trail`, `show-config`, `models` and `probe`. A family tool is read-only where every operation is.
- **Descriptions.** A tool's description, and the help it points to, are defined in [Served text](#served-text).
- **`build` and `score` are detached.** A call starts `kbase build` or `kbase score` as a process of its own, from the root of the call's project, passing `--project-dir` where the call named one, in its own process group, with no stdin, configured as the server is (the environment, `--config-dir`, `--log-level` and `--log-file`, §9). The run's stdout and stderr are captured as files in the state store's `reports/` (§6); the capture files are not removed. A relative `state-dir` argument resolves against the repository root; the server's own `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status` would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that refusal and starts no process. A run starting during a write waits that write out before its first stage (`build`) or its first rebuild of the derived views (`score`), and is visible to `status` while it waits.

  The tool waits at most 5 s, until the run has started or has exited, and returns by what happened first:
  - *started*: outcome `done`.
    - `build` has entered a stage, as its progress record shows. The result has `kb-root`, `state-dir`, `pid`, `resume` and `wait`; `resume` is absent where the record's command does not parse as a `kbase build` invocation (§6).
    - `score` has its ledger and pid. The result has `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait`.
  - *exited*: the run's own document, `isError` set by its outcome (§7).
  - *still starting at the bound*: outcome `done` with `state: starting`, `state-dir`, `pid` and `wait`. The run keeps running, and `status` follows it.

  `wait` is the shell command that runs the server's own executable as `wait [--project-dir <root>] --state-dir <store> --pid <pid>`, with `--project-dir` where the call named one, shell-joined as `status` joins its `resume` (§6). It returns when the run is over and prints that store's `status` document ([§6 `kbase wait`](#kbase-wait)), so a session runs it in the background and is woken when it exits. `status` and `cancel` follow the start.

  `score`'s `edge` argument is a mapping of `source`, `target` and `relation`, each a required string and no other key allowed, which the run receives as `--edge <source>,<target>,<relation>`. A value holding a comma is refused at `edge`, `check: usage`, and starts no process.
- **`build-trail`** is a read-only tool; its result, refusals and commands are defined in [§6 `kbase build-trail`](#kbase-build-trail).
- **What the server never does.** It sends no sampling request and offers no prompts. No inference runs from any tool but `build`, `score` and `probe`, and each tool's inference is its own: the build's decision and text calls, the score run's scoring calls, the one call `probe` makes. `models` reads the selected provider's model catalogue; `configure-providers` reads the catalogue of the entry it writes where a key is stored for it; `configure-models` checks each reference it writes against its provider's catalogue. None of them makes a model call. The server writes nothing but JSON-RPC frames to stdout; diagnostics go to stderr and `--log-file`. It loads no configuration itself: the verbs it exposes load their own on each call (§9.3).
- **Protocol.** The server implements one MCP revision, `2025-11-25`. An `initialize` naming another revision is answered with the server's, for the client to decide. Calls are answered one at a time, in order. EOF on stdin ends the server.

### Two modes

The server has a default project or it does not. With a default project, a call may leave out `project-dir`. Without one, every call names its project.

- **With a default project.** `kbase mcp` found one, searching upward from its working directory for the repository root. `project-dir` is optional in every tool's schema, and a call that omits it runs in the default project.
- **Without a default project.** `kbase mcp --project-dir-required`, or a server whose search finds no repository. `project-dir` is in the `required` list of every tool's schema, its description says only what it names, and the instructions say how a session gets the path. A server that finds no repository serves exactly as one given `--project-dir-required` does, instructions included; the flag forces this mode from inside a repository. A server with no default project lists no sheet ([Resources](#resources)).
- **Startup never fails for want of a project.**
- **No project flag.** The global `--project-dir` given to `mcp` is a usage refusal (§7) before it serves.
- **`project-dir`** is the absolute path of the project's git worktree root, the directory holding `.git`, where `kb-root/` is or will be; it is never `kb-root/` itself. A call runs in the project it names where it names one, else in the default project. A call naming an empty one where there is no default project is `refused`, `check: usage`, `key: project-dir`. A `project-dir` that is no git worktree root is `refused`, `check: worktree`, `key: project-dir`, `path` the absolute path; a relative one resolves against the server's working directory.
- **Registration** is done by the external agent harness: one server entry, `kbase mcp` (with a default project, run from inside the project) or `kbase mcp --project-dir-required` (without one, for a harness that serves many projects from one place). README gives entries for two external agent harnesses as examples.

### Resources

| URI | Type | Listed | Contents |
|---|---|---|---|
| `kbase://sheet/<kb-root-relative path>` | `image/svg+xml` | with a default project whose KB opens | each claim-graph sheet ([§3](#claim-graphsvg)) present on disk, the placeholder included |
| `kbase://model-hints` | `text/markdown` | always | guidance on choosing a model for each role, in the role vocabulary (§9.2), the same rows a created `config.toml` has |
| `kbase://help/<command>` | `text/plain` | always | one per command exposed as an MCP tool, each write op and query operation included: shared body, argument list and MCP note ([Served text](#served-text)) |
| `kbase://help/query` | `text/plain` | always | the `query` tool's operations, listed from the commands it serves, then what they share |
| `kbase://help/write` | `text/plain` | always | `kbase write`'s own help, which has the batch contract in full |

- `resources/read` of a sheet returns the SVG as text. A KB that exists but will not open lists no sheet: the list is answered without one, and that KB's unopenability is reported by `status` and `verify` when they are asked, not as a list fault.
- A help resource is named `<command> help` (`query help` for the `query` tool's). The `query` tool's shared help states, as MCP's arguments name them, how a query reads the KB and answers; `kbase --help` states it as the command line names the arguments, and no operation's own help states it.
- A help resource's argument list is generated from the arguments the command's tool takes: a write op's is an entry's keys, `kbase write`'s the batch's. The command's `--help` has the same list, formatted as the command line formats it.
- Model guidance and help are KB-independent text: listed whether or not a KB is found or opens, and the same in every project.
- No resource is subscribable. `initialize` advertises the `resources` capability with no `subscribe` among the ones it has. Nothing announces a run's end, so a session polls `status`, or runs in the background the `wait` command `build` and `score` return. `resources/subscribe` and `resources/unsubscribe` are methods the server does not answer (`-32601`).
- An unknown URI is JSON-RPC `-32002` (resource not found); a missing `uri` is `-32602`.
- Nothing else is served, a summary page included: the KB's documents are the external agent harness's own files.

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

Items 3 and 4 are the text the stamped `AGENTS.md` also contains ([§3 Stamped documents](#stamped-documents)), byte for byte. Item 5 is the text `kbase --help` also contains, byte for byte.

**Help.** Each command has a one-line summary, one shared help body, an optional MCP note and an optional CLI note. The shared body is the same text on both interfaces: an argument has one name, which the command line writes `--name` (a positional `<name>`) and MCP takes bare, and a kbase call is written as the interface in use writes it, as a `kbase` command line or as the tool's call. A note states only what differs because the environment differs; no help text is written separately for each interface. Each text is rendered in three places:

| Rendering | Contents | Read |
|---|---|---|
| MCP tool description | the summary, the MCP note, and where the full help is | always (`tools/list`) |
| `kbase://help/<command>` | the shared body, the argument list and the MCP note | on demand |
| `kbase <command> --help` | the usage line, the shared body, the argument list, the subcommand list where the command has subcommands, the CLI note, and for a subcommand a line naming the global flags | on demand |

- The argument list is part of the help on both interfaces. It lists each argument's name, type and whether it is required or repeatable, with its description, which states the default where `--help` prints one. Each interface writes an argument as it takes it: the name as the body writes it; a repeatable argument as a list over MCP; a mapping as its CLI string at the command line; a shorthand only at the command line.
- The two help renderings differ only by their notes, that formatting, and the usage line (`kbase <command> [flags]`), which only the command line has. The descriptions are the same text.
- A query operation's argument list is its own, not the `query` tool's.
- A tool description and its schema have what every call of that tool needs. The help has the full rules, refusals and examples.
- A command tool's description, `write`'s included, is its summary, then its MCP note, then where its full help is, `kbase://help/<command>`. The `query` tool's description is one sentence, then where its operations' shared help is, `kbase://help/query`, and where one operation's is, `kbase://help/<operation>`.

**MCP notes.** The content each note must have:
- `write`: the batch contract. "A batch applies its entries in the order given, each checked against the records as the earlier entries left them; kbase never reorders a batch." Shape errors are all reported at once; then the batch stops at the first refusal and writes nothing. `dry-run` and `commit`. Where the shared help is (`kbase://help/write`) and where each op's help is (`kbase://help/<operation>`). `kbase://help/write` and the root `kbase --help` have the batch contract in full ([§8 Write batches](#write-batches)).
- `build` and `score`: the call returns once the run has started, as a detached process that keeps running without the server; the result's `pid`, `state-dir` and `wait`; `status` follows the run and `cancel` stops it.
- `probe`: "Each call is a paid model call: run it only when the user directs it, one call per direction, never on your own initiative or to retry."
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
- A help that tells a caller to copy an edge row from a result into `add-edge` or `edit-edge` gives this advice. A row's `manually_scored`, `replaced_relation`, and `trace`'s `depth`, `strength` and `contribution`, and any op key whose value is null, are read-only facts: drop them before copying the rest. Copy `bearing` only where the row is manually scored or the relation is `founds`: a bearing given marks the edge manually scored ([§10 Provenance and the judged hash](#provenance-and-the-judged-hash)), so copying a model-judged one would pin it. Into `remove-edge` and `restore-demoted`, copy only `source`, `target` and `relation`.
- The help of an op that takes a `rationale` says that what would strengthen a node is a graph question, answered by `trace` with `sort: contribution` and `order: ascending`.
- No help names a command or resource that this SPEC does not define.
