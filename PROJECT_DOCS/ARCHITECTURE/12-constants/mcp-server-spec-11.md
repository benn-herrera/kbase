### MCP server (SPEC §11)

`internal/mcp` builds the tool table from the command tree its caller
hands it: one Tool per command declaring `kbase.mcp` = `bound` — `mcp`, `wait`, `auth` and
cobra's own `completion` and `help` declare `excluded`, each with its reason — save that the commands
declaring one family (`kbase.mcp.family`, set by `mcp.InFamily`) are one Tool together, in the place
of the first; no tool is listed by hand. A family is an `mcp.Family` its caller hands it (name,
description sentence, shared help), `query` (`toolFamilies`, `cmd/surface.go`); a family a bound
command is named for is that command's, its members no Tool of their own: the write ops declare the
family `write`, and the `write` Tool is `kbase write`'s (`mcp.batchTool`), its schema the batch —
`entry`, a required array of objects, and `dry-run` and `commit`, booleans — its arguments handed whole to
`Calls.WriteOp` as `write.Batch`'s values document. A
command tool's schema is derived from the command's `Use` line positionals and its flags,
`mcp.CommandSchema` skipping a flag annotated `kbase.mcp` = `excluded` (set by `mcp.ExcludeFlag`),
which is no argument of the tool; `score`'s `--replay` and `--failed` are such flags, and an
argument naming one is refused by `internal/mcp/protocol` as outside the schema. A command's
read-only standing is the cobra annotation `kbase.readOnly`, read when the table is built. A command
tool's `required` is its positionals alone, less an optional variadic one, which its `Use` line
spells `[<name>...]`: no bound command refuses for want of a flag, `score`'s `--pass` among them, a
run walking both passes unless it is told otherwise; `wait`'s `--pid`, the one flag a verb requires,
is outside the derivation, `wait` declaring `excluded` ([§11](../11-package-map/index.md)).

A family tool's schema is `operation`, a string enum of its members' names in the tree's order, and
`arguments`, an object; its read-only hint is set where every member declares `kbase.readOnly`.
Each member is built as its own command tool would be, and
the family keeps it by name, unlisted: a call takes the member `operation` names and checks
`arguments` against that member's schema with `protocol.Schema.Faults`, the same check
`internal/mcp/protocol` runs on a tool's arguments, so no argument list is stated twice (a query's
from `mcp.CommandSchema`). A fault is a usage refusal
document, one item per fault naming the argument, the member's arguments as `allowed`, and a
remedy naming `allowed` for the argument names and the member's help resource for what it does;
arguments that fit run the member as its own tool would. A values op's help
lists its entry's keys in vocabulary order, each with the type, required mark and description its
definition in `internal/write/values.go` carries, through `write.OpVocabulary` (`mcp.ArgumentList`),
naming no flag, so it is true on both surfaces. The members' shared help is appended to no member's `Long`. The root command's `Long`
states the CLI's form once each — `queryHelp`, `listQueryHelp` and
`valuesHelp` in `cmd`, the last closing on `batchHelp`, the write batch's contract — and the `query`
family's `Help` the MCP form for `kbase://help/query`, from the same text where the two agree
(`queryReads`, `queryAnswerHelp`, and `listQueryHelp`, one marked text each rendering spells for its
surface, SPEC §7 `key`); the resource lists the members from the annotations
the enum is built from. `batchHelp` is also `kbase write`'s `Long`, so `kbase://help/write` carries
the contract in full, and every write op shares one MCP note and one CLI note (`writeOpCommand`).

`cmd/surface.go` adds every command to the tree, one `AddCommand` per group, its binding and
read-only hint on that line (`readOnlyTool`, `readWriteTool`, `cliOnly`, and for a family tool's
operation `queryOperation` or `writeOperation`, which add `kbase.mcp.family`), and declares beside them
the MCP-only surfaces: the resource families and the detached launches. It names no project for
them: `mcp.Server.Serve` binds the resources to the server's located project.

A call's project is applied in one place. `mcp.Tools` wraps every tool in
`inProject`: the wrapper adds the `project-dir` property to the schema, and on each call
`callProject` resolves the project — the `project-dir` the call names, which `buildstore.ProjectRoot`
requires to be a git worktree root (the same check `--project-dir` gets on the command line),
else the located project — removes `project-dir` from the arguments, and hands the inner tool the
project. No tool closes over a directory: a command tool sets it as the call's invocation
directory, a write op passes it to `Calls.WriteOp`, and a launch starts its child there. The located project is `mcp.Located`, whose zero value is the
explicit none: `cmd/mcp.go` sets it from `kb.RepositoryRoot` of the invocation directory only
where that finds a repository, so no reader ever passes `""` to a path function that would read it as
the working directory. With none, a call naming no project answers the usage refusal keyed
`project-dir`.

Every tool has a command behind it (I9): `build-trail`, the build's commit trail, is
`cmd/build-trail.go`, a command tool like `status` (`buildstore.BuildTrail`), so its schema, key
order and refusal keys are derived as any command's are.
`TestMCPToolsAreTheCommandTree` derives its expectation from the annotations alone — a family's name
for its members, a command's own otherwise — and asserts the tool set for equality, so a tool listed
by hand fails; `TestBuildTrailIsOneDocumentOnBothSurfaces` runs `build-trail` at the command line
and through the server and requires the two documents to agree, each surface spelling the build
call in each boundary's command as it takes it.

The `write` tool's arguments are the batch (`batchTool`), run through `runWriteOp` as
`write.Batch`'s values document with `ArgumentEntry` set: each refusal keeps its `entry`, and one
naming no `path` drops `line` and `column`, which would point into a document the caller never
wrote, while one naming a document keeps its line there; a refusal's `key` is the values key, which no
surface spells differently.
Every other tool runs the command's RunE with its flags reset and its output captured to a buffer. The
buffer is the result text; decoded once more, the YAML document is the `structuredContent`; the
document's outcome gives `isError` (`result.IsError`). A RunE starts from `invocationDir(cmd)` —
the call's project over MCP, else `--project-dir`, else the working directory — never
`os.Getwd()`.

`internal/mcp/protocol` holds newline-delimited JSON-RPC over stdio, `initialize`, `ping`,
`tools/list` and `tools/call`, the `Tool`, `Schema` and `Property` types, and the frame checks
(`jsonrpc`, `id`, `method`, known tool, object arguments). Arguments failing the schema (missing,
wrong type, a string outside its `enum`, unknown key) are JSON-RPC `-32602` errors, not refusal
documents; `Schema.Faults` is that check as data, which a family tool runs on its `arguments`. `Serve` takes the
server version, the instructions `initialize` reports, the tool table and the resources from its
caller, and imports `log` only. The instructions are `mcp.ServedInstructions`, spelled over MCP:
its opening, which varies by mode — the located project's state where it located one, the
`project-dir` lines where it located none — and the sentence naming the skill, then the `.index/`
rule, `kbdocs.IndexRule`, and the session rule, `kbdocs.SessionRule`, which the stamped
`AGENTS.md` carries through its `index-rule` and `session-rule` slots, then `mcp.EdgeDirection`,
which the root's `Long` carries too, then the paragraphs only the instructions carry
(`instructions.go`): the text a session reads. `resources/list` and `resources/read` serve the resources `internal/mcp` supplies: the
sheets, under `kbase://sheet/<kb-root-relative path>`; the model guidance, at
`kbase://model-hints`, which is `internal/config`'s rows rendered as markdown and needs no KB;
and each family's help, at `kbase://help/<family>`, then each bound command's, at
`kbase://help/<command>` (`mcp.HelpResources`), the latter the command's `Long` and its MCP note
(`kbase.help.mcp`), composed in `internal/mcp/help.go`; neither needs a KB.
An unknown URI is `-32002`, a missing `uri` `-32602`; `initialize` advertises `resources` with no
`subscribe`, and the protocol answers no subscribe method (`TestNoSubscription`).

The server runs in one of two modes, chosen in `cmd/mcp.go` before serving (SPEC §11). Located,
`kbase mcp` inside a repository, the located project is `kb.RepositoryRoot` of `invocationDir`.
Where that finds none, or under `--project-dir-required`, it is `mcp.ProjectRequired()`, the zero
`mcp.Located`: none, so that `inProject` lists `project-dir` in each schema's `required` with
`projectDirMeaning` alone as its description, where a located server gives
`projectDirDescription`, that text followed by the sentences on the server's own project. The `callProject`
refusal stays behind that schema for a call whose `project-dir` is empty. The global
`--project-dir` given to `mcp` is a usage refusal keyed `--project-dir` before serving; startup
never fails for want of a project.

The `build` tool (`internal/mcp/launch.go`, the one self-exec owner under I1, file-level, enforced
by the exec-policy test) re-executes the kbase binary as `kbase build …`. The child runs from the
root of the call's project, with the server's environment, in its own process group (detach flags
per platform in `internal/mcp/detach_unix.go` and `internal/mcp/detach_windows.go`), stdin unset,
the server's `--config-dir`, `--log-level` and `--log-file` passed through, and `--project-dir`
where the call named one, so the child's recorded `resume` carries it. Every path the child is
given is absolute, because its working directory is not the server's: a relative `state-dir`
argument is resolved against the repository root, while the server's `--config-dir` and
`--log-file` stay relative to the server's working directory. Its stdout and stderr are files in
the state store's `reports/`, `build-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`; they
are not cleaned up. The tool waits on the child's progress record and returns when the record shows
a stage entered or the child exits, bounded by `launchStarting` (5 s, [§12](index.md); the child takes
the operation and holder locks and emits its first progress event before it waits out a write,
[§8](../8-ledger-state-store-and-monitoring.md)):

| Child, within 5 s | Result |
|---|---|
| Entered a stage | `done` with `kb-root`, `state-dir`, `pid`, `resume` and `wait` (`resume` only where the record's command reads as a `kbase build` invocation, SPEC §6; `wait` the run's background command, `buildstore.ShellJoin` of the executable with `wait [--project-dir <root>] --state-dir <store> --pid <pid>`, `--project-dir` where the call named one) |
| Exited first | The child's own document; `isError` from its outcome |
| Neither | `done` with `state: starting`, `state-dir`, `pid` and `wait`; `status` follows the run |

Before starting, the tool runs `status`'s preconditions: no worktree, or a `--state-dir` inside
`kb-root/`, returns the refusal `status` gives and starts no process.

The `score` tool re-executes the kbase binary as `kbase score …` by the same detachment: the child
runs from the root of the call's project, in its own process group, with the server's environment,
`--config-dir`, `--log-level` and `--log-file`, `--project-dir` where the call named one, and its stdout and stderr are files in the state
store's `reports/`, `score-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`, which `score.PruneRuns` keeps the newest `KeptRuns` of ([§8](../8-ledger-state-store-and-monitoring.md)). It waits on the
run's records and returns when the run's ledger and pid stand (the run's directory written, its pid
file reading back) or the child exits, bounded by the same `launchStarting`. The
`edge` flag is declared a mapping (`mcp.MappingFlag`), so its schema property is an object of the
required strings `source`, `target` and `relation`, closed to other keys, and `CommandLine` joins
the mapping into the child's `--edge <source>,<target>,<relation>`, as `mcp.Spelling` does in a
call it renders; a value holding a comma is refused at `edge` (`commaInMapping`) and starts
nothing:

| Child, within 5 s | Result |
|---|---|
| Ledger and pid stand | `done` with `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait` (`wait` the run's background command, `buildstore.ShellJoin` of the executable with `wait [--project-dir <root>] --state-dir <store> --pid <pid>`, `--project-dir` where the call named one) |
| Exited first | The child's own document; `isError` from its outcome |
| Neither | `done` with `state: starting`, `state-dir`, `pid` and `wait`; `status` follows the run |

A score run's result document is written beside its ledger before the run lets go of the store's
holder lock, so `wait` reads the same document the run reported rather than racing the lock's release.
`wait` locates the run by the pid the launch named — `score.RunByPID` reads the run's `pid` file —
and reports the run's outcome; a run whose process ended without writing a result reports the
failure its ledger supports. `status` and `cancel` read the state store's holder record (the
pid, the run, the start stamp and the kind `score`), so a run in flight is visible under the pid the
tool returned.

Nothing announces a run's end: no resource is subscribable and the server writes no frame a
request does not owe, so a session follows a launched run by calling `status`, or by running the
`wait` command the launch returned (SPEC §11). `test-integration-mcp-build`
(`TestMCPBuildFollowedByStatus`) drives a real server to a build's end by polling `status`.

---

