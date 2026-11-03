## 11. The MCP server

`kbase mcp` serves kbase to any external agent harness that speaks the Model Context Protocol, over
stdio JSON-RPC. It is the subcommands, bound: every subcommand it serves is a tool of its own, an
operation of the `query` family tool, or a write op an entry of the `write` tool's batch, with the
same arguments and the same result, so an external
agent harness without MCP loses nothing but convenience. No tool is listed without a subcommand
behind it.

- **Tools.** Every subcommand is served, less `completion`, `help`, `mcp` itself and
  `wait` — a tool call would hold the turn for the build's or score run's duration — and less
  `auth` and its subcommands — a key arrives on stdin, which no tool call has, and no tool accepts
  a key ([page §4.4](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md)). `probe` is a tool; its MCP note makes each call one the user directs ([page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)). The queries are the
  operations of one tool, `query`; the `write` tool is `kbase write`'s, whose batch's entries name
  the write ops ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)), and a write op has no tool of its own; every
  other subcommand is one tool, named as the subcommand. Each operation stays a subcommand, unchanged
  on the command line. A command tool's input schema is derived from its command's arguments and flags (the global flags
  excluded — the server carries its own — but for `project-dir`, below), and a duration flag's
  argument is the Go duration string the command line spells (`30s`). A flag is optional in the
  schema but where its command declares it required, as `trace`'s `direction` is. Each of a command tool's
  properties carries a description: a positional's the one its command declares, a flag's its
  usage, followed by the default `--help` prints for the flag where it prints one and, for a
  duration flag, that its argument is a Go duration. A family tool's schema is
  `operation`, required, a string enum of its operations' names, and `arguments`, an object holding
  what the operation takes: its positionals and flags by name, derived as a command
  tool's schema is. `operation`'s description names where each operation's help
  is, and `arguments`' names that help's argument list. The `write` tool's schema is the batch: `entry`, required, an
  array of objects, each naming its op under `operation` beside that op's keys, and `dry-run` and
  `commit`, booleans; the entries' shape is the batch's own check ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)), every fault an item naming its
  `entry`. The
  family tool checks `arguments` against its operation's own schema: an unknown key, a missing
  required key or a value of the wrong type is a refusal document, `refused`, one item per fault,
  `check: usage`, `key` the argument's name, `allowed` every argument the operation takes and
  `remedy` naming `allowed` for the argument names and the operation's help resource for what it
  does. A values entry's keys carry no types there, so
  a wrong value for a values key is the op's own refusal ([§8](8-maintenance-subcommands/index.md)), which names the key and, where the
  value comes from a closed set, lists that set as `allowed`. An `operation` outside the enum, or an
  `arguments` that is not an object, fails the tool's
  schema. Every tool's schema, a family's included, carries one
  more property, `project-dir`: the project the call runs in, the git worktree root that holds
  `kb-root/` or will. The schema is rendered per mode (below): located, the property is
  optional, and a call that omits it runs in the server's located project; not located, it is listed
  in `required`, and its description says only what it names. A call runs in the project it names where it names one, else in
  the server's located project; a call naming an empty one where none is located is `refused`, `check: usage`, `key:
  project-dir`. A `project-dir` that is no git worktree root is `refused`, `check: worktree`, `key:
  project-dir`, `path` the absolute path; a relative one resolves against the server's working
  directory. A refusal over MCP carries no `line` or `column` unless it names a `path`, where they locate it
  in that document, and
  `entry` as the entry's position in the batch; its `key` keeps the CLI spelling ([§7](7-tool-result-contract-and-exit-codes/index.md)), or,
  in a family tool's argument check, is the schema property name. Arguments that fail the tool's
  schema (a missing required key, a wrong type, an unknown key) are JSON-RPC `-32602` errors, not
  refusal documents; a family tool's `arguments` are checked as above instead. A command declares
  itself read-only in its definition, and the tool carries that as its read-only annotation, a
  family tool where every operation declares it; `query` — every query among
  its operations — `verify`, `status`, `build-trail`, `show-config`, `models` and `probe` are the read-only
  set. The `query` tool's description is one sentence, then where its operations' shared help
  is, `kbase://help/query`, and where one operation's is, `kbase://help/<operation>`. A command
  tool's description, `write`'s included, is its command's one-line summary, then its MCP note, then
  where its full help is, `kbase://help/<command>` ([page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)); `write`'s MCP note carries the
  batch contract that page lists. The table is one tool per family, one per other bound command but
  the write ops, and none listed by hand; a test derives the set from the commands' declarations
  and asserts it for equality, so a tool listed by hand fails as a missing command does.
- **Results.** The [§7](7-tool-result-contract-and-exit-codes/index.md) document, as the result's text and as structured content. `isError` is set
  exactly when the document's exit code ([§7](7-tool-result-contract-and-exit-codes/index.md)) is not 0, so a refusal reaches the model as an error
  carrying its items and remedy while `bounded` and a completed `cancel` do not. No list query pages:
  `citations-of`, `edges`, `find` and `trace` answer completely ([§7](7-tool-result-contract-and-exit-codes/bounded-results.md), [§8](8-maintenance-subcommands/citations-of.md), [§8](8-maintenance-subcommands/edges.md), [§8](8-maintenance-subcommands/find.md), [§8](8-maintenance-subcommands/trace.md)), and `find` and `trace`
  take `limit` only beside `sort`.
- **Two modes**, located and not located, as [page §4.3](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md) defines them: `kbase mcp` started inside
  a repository is located; one whose search finds no repository, or `kbase mcp
  --project-dir-required`, is not, and the two serve alike, instructions included. The server takes
  no project flag: the global `--project-dir` given to `mcp` is a usage refusal ([§7](7-tool-result-contract-and-exit-codes/index.md)) before it
  serves.
- **`build` is detached.** The tool starts `kbase build` as its own process, from the root of the
  call's project, passing `--project-dir` where the call named one, in its own process group, with
  no stdin, configured as the server is (the environment,
  `--config-dir`, `--log-level` and `--log-file`, [§9](9-configuration/index.md)). The build's stdout and stderr are captured as
  files in the state store's `reports/` ([§6](6-build-state-ledger-and-monitoring/index.md)). The tool waits until the build's progress record shows
  it has entered a stage, or until it exits, for at most 5 s, and returns by what happened first
  ([page §3.5 `build`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)); a build starting during a write waits that write out before its first
  stage, and is visible to `status` while it waits:
  - *entered a stage* — outcome `done` with `kb-root`, `state-dir`, `pid`, `resume` and `wait`, the
    `resume` absent where the record's command does not read as a `kbase build` invocation ([§6](6-build-state-ledger-and-monitoring/index.md)).
    `wait` is the exact shell command `kbase wait [--project-dir <root>] --state-dir <store> --pid
    <pid>`, carrying `--project-dir` where the call named one, shell-joined as `status` joins its
    `resume` ([§6](6-build-state-ledger-and-monitoring/index.md)): it returns when the run is over and prints that store's
    `status` document, so a session runs it in the background and is woken when it exits. `status`
    and `cancel` follow it;
  - *exited* — the build's own document, `isError` set by its outcome ([§7](7-tool-result-contract-and-exit-codes/index.md));
  - *still starting at the bound* — outcome `done` with `state: starting`, `state-dir`, `pid` and
    `wait`; the build keeps running, and `status` follows it.

  A relative `state-dir` argument resolves against the repository root; the server's own
  `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status`
  would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that
  refusal and starts no process. The capture files are not removed.
- **`score` is detached.** The tool starts `kbase score` as its own process, from the root of the
  call's project, passing `--project-dir` where the call named one, in its own process group, with
  no stdin, configured as the server is (the environment,
  `--config-dir`, `--log-level` and `--log-file`, [§9](9-configuration/index.md)). The run's stdout and stderr are captured as
  files in the state store's `reports/` ([§6](6-build-state-ledger-and-monitoring/index.md)). Its `edge` argument is a mapping of `source`,
  `target` and `relation`, each a required string and no other key allowed, which the run receives
  as `--edge <source>,<target>,<relation>`; a value holding a comma is refused at `edge`, `check:
  usage`, and starts no process. The tool waits until the run's ledger and pid stand, or until it
  exits, for at most 5 s, and returns by what happened first ([page §3.5 `score`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)); a run
  starting during a write waits that write out before its first refresh, and is visible to
  `status` while it waits:
  - *started* — outcome `done` with `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait`.
    `wait` is the exact shell command `kbase wait [--project-dir <root>] --state-dir <store> --pid
    <pid>`, carrying `--project-dir` where the call named one, shell-joined as `status` joins its
    `resume` ([§6](6-build-state-ledger-and-monitoring/index.md)): it returns when the run is over and prints that run's
    document, so a session runs it in the background and is woken when it exits. `status` and `cancel` follow it;
  - *exited* — the run's own document, `isError` set by its outcome ([§7](7-tool-result-contract-and-exit-codes/index.md));
  - *still starting at the bound* — outcome `done` with `state: starting`, `state-dir`, `pid` and
    `wait`; the run keeps running, and `status` follows it.

  A relative `state-dir` argument resolves against the repository root; the server's own
  `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status`
  would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that
  refusal and starts no process. The capture files are not removed.
- **`build-trail` reports the trail.** It is a subcommand and its tool, read-only; its argument,
  result and refusals are [page §3.5 `build-trail`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md)'s. Each `command` is a list of the
  command lines it runs in order: a boundary's, `git reset --keep <commit>` then the build resumed
  with the flags its last run was launched with, the bound excepted, which each surface spells as it
  takes it — `kbase build --resume` and those flags at the command line, the `build` tool's call
  over MCP — and `started-at`'s the reset alone. The trail is the newest
  build's ([§5 Starting over](5-build-behaviour/starting-over.md)). `failed` (3) is a git failure.
- **Resources.** The claim-graph sheets ([§3](3-compatibility-contract/index.md)) present on disk, the placeholder included, are listed
  by `resources/list` as `image/svg+xml` resources with URI `kbase://sheet/<kb-root-relative path>`;
  `resources/read` returns the SVG as text. A KB that exists but will not open lists no sheet: the
  list is answered without one, and that KB's unopenability is `status`' and `verify`'s to report
  when they are asked, not a list fault. Beside them `kbase://model-hints` is listed as one
  `text/markdown` resource holding the guidance on choosing the `[models]` ids ([§9.2](9-configuration/9.2-config-toml-the-choices/index.md)) — the same
  rows a created `config.toml` carries — and it is KB-independent text: it is listed whether a KB
  is found and opens, is found and will not open, or is not found at all, and what it states does
  not vary with one. A server with no located project lists no sheet. No resource is
  subscribable, and `initialize` advertises the `resources` capability with no `subscribe` among
  the ones it carries ([page §4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md)): nothing announces a run's end, so a session polls `status`,
  or runs in the background the `wait` command `build` and `score` return; `resources/subscribe`
  and `resources/unsubscribe` are methods the server does not answer (`-32601`). An unknown URI is
  JSON-RPC `-32002` (resource not found); a missing `uri` is `-32602`.
  The `query` tool has its help listed as one `text/plain` resource, `kbase://help/query`, named
  `query help`: its
  operations, listed from the commands it serves, then what they share, stated there as MCP's
  arguments spell it, in `kbase --help` as the command line spells it, and in no operation's own
  help — how a query reads the KB and answers. `kbase://help/write` is `kbase write`'s own help, below, which carries the
  batch contract in full, as `kbase --help` does ([page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)). Each command a tool binds, an
  operation's and a write op's included,
  then has its help listed as one `text/plain` resource with URI
  `kbase://help/<command>`, the command's name in place of `<command>`, named `<command> help`: the command's shared help
  body, its argument list and its MCP note, the rendering [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) states. The argument list is
  generated from the arguments its tool takes — a write op's is an
  entry's keys, `kbase write`'s the batch's — and its `--help` holds the same list, spelled as the
  command line spells it. These are KB-independent text, as the model guidance is: listed
  whether or not a KB is found or opens, and the same in every project.
  Nothing else is served, a register page included: the KB's documents are the external agent
  harness's own files.
- **What the server never does.** It sends no sampling request and offers no prompts; no inference
  runs from any tool but `build` and `score`, and each tool's inference is its own — the build's
  decision and text calls, the score run's scoring calls. `models` reads the
  selected provider's model catalogue; `configure-providers` reads the catalogue of the entry it
  writes where a key is stored for it, and `configure-models` checks each reference it writes
  against its provider's catalogue ([page §3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md)). None of them asks a model anything. It writes nothing but JSON-RPC frames to stdout; diagnostics go to stderr
  and `--log-file`. It loads no configuration itself — the verbs it exposes load their own on each
  call ([§9.3](9-configuration/9.3-strict-load-contract.md)). The `build` and `score` children it starts inherit the server's environment and
  `--config-dir`.
- **Protocol.** One MCP revision, the one the server implements, pinned as one constant
  (ARCHITECTURE §12); an initialise naming another revision is answered with the server's for the
  client to decide; calls are answered one at a time, in order; EOF on stdin ends the server.
- **Instructions.** `initialize` always carries `instructions` in each mode: the eight items of
  [page §4.1](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md), in its order and with its quoted wording. The `.index/` rule and the session rule are
  the text the stamped `AGENTS.md` also carries ([§3](3-compatibility-contract/stamped-documents.md)), and the edge-direction sentence the text
  `kbase --help` also carries. Every tool they name is one the server serves.
- **Registration** is the external agent harness's: one server entry, `kbase mcp` (located, run
  from inside the project) or `kbase mcp --project-dir-required` (not located, for a harness that
  serves many projects from one place). README gives entries for two external agent harnesses as
  examples.

---

