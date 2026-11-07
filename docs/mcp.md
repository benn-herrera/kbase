# Using kbase from an agent harness (MCP)

How to serve kbase to an agent harness over the Model Context Protocol, and what the server offers. Back to the [README](../README.md).

`kbase mcp` serves kbase to any agent harness that speaks the Model Context Protocol, over stdio.
Every subcommand it serves is a tool of its own or an operation of the `query` or `write` tool, with
the same arguments and the same result. Three commands are CLI-only and served by no tool: `auth`
(a key never travels through MCP), `wait` (an MCP call cannot block; a session polls `status`) and
`mcp` itself.

The server entry is command `kbase` with one of two argument lists. A harness that runs kbase from
inside the project gets a located server: it finds its project by searching upward from its working
directory for the repository root.

```json
["mcp"]
```

A harness that serves many projects from one place, with no project working directory, gets a
server that is not located: every tool call names its project with `project-dir`. A server whose
search finds no repository serves the same way, and the flag forces it from inside a repository:

```json
["mcp", "--project-dir-required"]
```

Startup never fails for want of a project. `kbase mcp` takes no `--project-dir`; given one, it is
refused before serving.

Two examples. Claude Code, in `.mcp.json`:

```json
{"mcpServers": {"kbase": {"command": "kbase", "args": ["mcp"]}}}
```

opencode, in `opencode.json`:

```json
{"mcp": {"kbase": {"type": "local", "command": ["kbase", "mcp"], "enabled": true}}}
```

These are examples; any MCP-capable harness adds its own equivalent entry.

- **Tools.** Sixteen. `query` runs the queries (`find`, `show`, `edges`, `trace`, `citations-of` and
  `stats`) and `write` runs write batches: for `query`, `operation` names the query and `arguments`
  carries what it takes, its positionals and flags by name; for `write`, each entry of the batch
  names its op under `operation`. A bad argument is a usage refusal naming it and every argument the
  operation takes. Every other served subcommand is a tool of its own: `build`, `score`, `status`,
  `cancel`, `build-trail`, `verify`, `update-format`, `export-latex`, `init`, `configure-providers`,
  `configure-models`, `models`, `show-config` and `probe`. `completion`, `help`, `auth`, `wait` and
  `mcp` are not offered: a held `wait` would spend the turn the build or score run takes, so the
  `build` and `score` tools hand you their command to run in the background instead. `query`,
  `status`, `build-trail`, `verify`, `models`, `show-config` and `probe` are read-only. Only
  `build`, `score` and `probe` run inference; each `probe` call is a paid model call, which a
  session runs only when the user directs it. `build-trail` lists the build's boundaries, each with
  the command that rewinds to it; it is read-only, and running a reset is yours.
- **Every tool takes `project-dir`:** the absolute path of the project's git worktree root, the
  directory holding `.git`, never `kb-root/` itself. On a located server it is optional: a call that
  omits it runs in the server's own project. On a server that is not located it is required in
  every tool's schema, and the instructions tell the session to get the path from the user, by
  asking or by proposing one to confirm. A relative path resolves against the server's working
  directory; a path that is not a git worktree root is refused (`check: worktree`). Where a `build`
  or `score` call names it, the `wait` and `resume` commands it hands back carry `--project-dir`.
- **`build` is detached.** It starts the build as its own process and returns once the build has
  entered a stage or has exited, waiting at most 5 s. On entering a stage it returns the
  `state-dir`, `pid`, `resume` command and `wait`, the exact background command that returns when
  the build is over and prints its `status` result; follow it with `status` and stop it with
  `cancel`. Run `wait` in the background and your harness wakes the session when it exits, with the
  outcome in hand. A build still starting at the bound is reported `done` with `state: starting`,
  `state-dir`, `pid` and `wait`; it keeps running, and `status` follows it. The build's output is
  captured under the state store's `reports/`.
- **`score` is detached.** It starts a scoring run as its own process and returns once the run's
  ledger and pid stand or it has exited, waiting at most 5 s. On a started run it returns `run`,
  `ledger`, `pid` and `wait`, the exact background command that returns when the run is over and
  prints the run's result — the counts by ledger status, the items whose values landed and the
  ones that failed with their reasons. Run `wait` in the background and your harness wakes the
  session when it exits. Follow the run with `status`, stop it with `cancel`, and continue it by
  calling `score` again with `resume` true and no scope argument. A run still starting at the
  bound is reported as a build is, and keeps running. The run's output is captured under the state
  store's `reports/`. A score run's `edge` argument is a mapping of `source`, `target` and
  `relation`.
- **Resources.** `kbase://help/<command>` is each command's help — its body, its argument list and
  its MCP note — one per tool and per op of `write` and `query`, with `kbase://help/write` for what the batch is and
  `kbase://help/query`, which lists each operation with its summary, for what the family shares. The claim-graph sheets are resources,
  `kbase://sheet/<path under kb-root>`, read as SVG; a located server lists them, where its KB
  opens. `kbase://model-hints` is the model guidance as markdown — the same rows a created
  `config.toml` states — and needs no KB. No resource is subscribable: a session polls `status`, or
  runs the `wait` command `build` and `score` return.
- **Instructions.** `initialize` carries instructions that tell a session where the project is, to
  use the `kbase` skill, never to read or write under `kb-root/.index/`, that rigor and bearing
  values are the user's to give (`score` judges claim, analysis and experiment rigor and `supports`,
  `lifts` and `refutes` bearings; a work's rigor and a `founds` bearing stay pending until the user
  gives one), how an edge reads (`kbase --help` states it too), to pass `commit` on every write
  batch, that the sheets are drawn for people, and to run a key command kbase prints as printed.
- **Requirements are unchanged.** Sheets still need `dot`, and builds still need pandoc and a
  provider configuration, on the host that runs `kbase mcp`.
