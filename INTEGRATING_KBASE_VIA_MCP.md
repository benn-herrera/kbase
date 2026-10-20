# Integrating kbase into an agent harness over MCP

For the person wiring kbase into a harness. This document is the map: it says what the server is,
how to stand it up, what a harness sees, and where the rest is written. It does not restate the
contract; where a detail matters, it names the document and section that holds it.

## What you are integrating

kbase is a single Go binary that builds a knowledge base (KB) from a LaTeX document set — or creates
one empty with `kbase init` — and maintains it afterwards: a tree of Markdown documents with a claim
graph over them, held in a `kb-root/` directory inside a git repository. Every operation on a KB is a
subcommand of `kbase`. `kbase mcp` serves those same subcommands to an agent harness over the Model
Context Protocol (MCP), stdio JSON-RPC, one KB per server process. A harness without MCP loses nothing
but convenience: every tool but `rewind-build` is one subcommand with the same arguments and the same
result, and `rewind-build` reports what `status` and the commit trail already state.

Read in this order:

- `README.md` — requirements, quick start, the subcommands, the result document and exit codes,
  and the section "Using kbase from an agent harness (MCP)" with registration examples.
- `SPEC.md` §11 — the MCP contract: tools, results, `build`, resources, protocol, what the server
  never does. §7 is the result document every tool returns; §8 the write ops and their value
  vocabulary; §3 the sheets served as resources.
- `ARCHITECTURE.md` §12 — the pinned constants (the protocol revision, the waits).
- `THESIS.md` — why the KB is shaped as it is, if you need to explain it to someone.

## Setting up

1. **Host requirements** (README "Requirements"): pandoc 3.12 and git are required for builds;
   Graphviz `dot` is optional and draws the claim-graph sheets; Go and `just` build the binary.
   Serving an existing KB for reading needs none of pandoc, git or a provider. Creating a KB with
   `init` needs git and nothing else — no pandoc, no provider.
2. **Build:** `just build` produces `bin/kbase` for the host platform. `just test` runs the unit
   suite; `just test-integration` the hermetic integration suite, which includes the MCP transcript
   recipe (`test-integration-mcp`: fixed request/response transcripts fed through `bin/kbase mcp`
   over real pipes). The build-end leg (`test-integration-mcp-build`: a build whose end the server
   announces over a subscription, the probe sending nothing) needs pandoc, so it runs by name rather
   than inside the hermetic suite. All of them should be green on a fresh clone.
3. **Provider configuration** is read by `build`, `models` and `configure`; only `build` spends
   inference. The server passes its own environment and `--config-dir` to the build it starts (README
   "Configuration"; SPEC §9 for the `KBASE_*` variables and precedence). The `show-config` tool
   reports how the server's configuration resolves — the pool and its faults, the selected entry,
   the endpoint or what is missing — and needs no provider: it takes no action and makes no request
   (SPEC §9.6).
4. **A KB to serve.** Either one already built (a repository with `kb-root/` in it), one you build
   first with `kbase build <volume-root.tex>` from the repository root, or one you create empty with
   `kbase init` and fill through the write ops (SPEC §8). A KB at an older metadata format is
   converted in memory when it is read and stamped by its first write (SPEC §10). Until that save its
   index belongs to the old form, so every query and `render-claim-graph` is refused (`check: index`,
   remedy `kbase refresh`): run `kbase refresh` before serving it.

## Registering the server

The whole registration is one command line:

```
kbase mcp --kb-root <absolute path>
```

`--kb-root` names the KB's `kb-root/` directory, or the repository root that holds it or will hold
it. Without it the KB is found from the server's working directory, which is fine when the harness
launches the server from the repository. Use an absolute path when it does not. Any other path is
refused before the server serves. Global flags `--config-dir`, `--log-level` and `--log-file` apply;
`--log-file` is the place to look when something is off, since stdout carries protocol frames and
nothing else.

README's MCP section shows the entry for two harnesses (Claude Code's `.mcp.json` and opencode's
`opencode.json`). Any harness adds its own equivalent: a server named `kbase`, the command above,
stdio transport. There is no registration automation in this repository, by design; the entry is
the harness's.

## The runtime agents

The server gives a harness the tools; the agents that know how to use them come from a separate
repository, [adjagent](https://github.com/benn-herrera/adjagent), which renders a whole agent set
from templates and installs it into a project. The KB part of that set is one skill and two
subagents:

- **skill:** `kbase` — the KB as one of the session's capabilities rather than a command to
  remember. It locates the KB and routes to a reference for reading, maintenance or building.
- **subagents:** `kb-maintainer` (the write side: inserts entries, sets fields, runs the
  refresh/verify loop) and `kb-claim-scorer` (grades derivations; writes nothing).

**Acquiring them.** Clone adjagent anywhere; it needs Python 3.11 or later. From its root:

```sh
just install <your project> [--harness=NAME]
```

That renders the set and writes the agent definitions and the skill under the harness's project
directory (`.claude/` for Claude Code, `.opencode/` for opencode; `--subdir=` overrides). The
installed tree is an artifact: each file carries a banner naming its template and a body hash, and
re-running the install overwrites it. Do not hand-edit installed copies for anything you mean to
keep; change the template and reinstall, or keep your own fork of the templates.

**A harness adjagent does not know yet** is described to it by one file,
`templates/harness/<name>.toml`: the project and user directories, the name of the agents file,
the frontmatter keys the harness expects on a definition, and the harness's names for the generic
tools (read, edit, bash, …). Copy `claude.toml` or `opencode.toml` and change the values; the same
install command then renders for your harness.

**Modifying them for the server.** As installed, the definitions reach the KB through a command
line. With the server registered, those passages become tool calls, and re-pointing them is
adjagent's work, not yours: the text sits in its templates and in the skill's references, and the
change arrives with the same install command. Nothing in kbase's repository is edited for this. Once
the definitions go through the server, nothing invokes a command line, which also guarantees nothing
falls back to one. The exact tool names and their schemas are SPEC §11 and the
server's own `tools/list`.

**Checking the result** is the in-harness check below: a reading session whose first lookup goes
through `find`, `show` and `deps`, with no command line invoked and no index file read by hand.

## What the harness sees

- **Tools.** One per subcommand, named as the subcommand: `init`, the write ops (`insert-claim-entry`,
  `insert-analysis-entry`, `set-rigor`, `set-bearing`, `add-rests-on`, `resolve-demoted`, …),
  `render-citation`, the queries (`find`, `show`, `deps`, `gated-on`, `cited-by`, `referenced-by`,
  `strength-below`, `subtree`, `weak-points`, `stats`), `show-config`, `models`, `configure`,
  `refresh`, `verify`, `render-claim-graph`, `export-latex`, `status`, `cancel` and `build`. Not
  offered: `completion`, `help`, `mcp` — and `wait`, which a tool call would hold for the build's
  duration; the `build` tool returns its command instead. Each tool's input schema is derived from
  its command's arguments and flags; a write op's schema is one entry with the op's closed set of
  value keys and no others. The queries, `verify`, `status`, `render-citation`, `show`,
  `show-config`, `models` and `rewind-build` carry the read-only annotation. `models` reads the
  selected provider's model catalogue. `configure` writes `config.toml`, dialling the provider for
  that catalogue except on its `--create` path with no `--provider`, which dials nothing. Neither
  performs inference, which stays with `build`.
- **`rewind-build` is the one listed tool.** No subcommand stands behind it: its schema is written by
  hand (`target` and `state-dir`, both optional), and a refusal from it is keyed by the schema
  property name rather than a flag spelling. It lists the build's stage boundaries, oldest first,
  with each one's stage, commit and subject, `head` and the worktree's `dirty` paths; for a `target`
  — a stage id or display name resolved as `--through` resolves one — it names that boundary with
  `discards`, the count of boundaries newer than it, and `commands` carries the `git reset --hard`
  and the recorded re-issue, for you to run. It changes nothing. The table test requires it to be the
  only listed tool, by equality.
- **Results.** Every tool returns the result document of SPEC §7 twice: as the text content (YAML)
  and as `structuredContent`. The document is §7's shared vocabulary — `outcome`, `kb-root`, then
  the subcommand's own keys — and a `migration` record (`from`, `to`, `note`) stands after `kb-root`
  on every result from a KB read on an older format, until a save stamps it current; `stats` counts
  one per node kind in `results` — `claims`, `analyses`, `experiments`, `axioms`, `works`. `isError`
  is set exactly when the document's exit code is not 0, so a refusal reaches the model as an error
  that still carries its items and remedy. Arguments that fail the schema are JSON-RPC `-32602`
  errors, not result documents. List queries page with `limit` and `offset`.
- **Resources.** The claim-graph sheets on disk, as `image/svg+xml` resources with URI
  `kbase://sheet/<path under kb-root>`; `resources/read` returns the SVG as text. Beside them
  `kbase://model-hints` is the `[models]` guidance as markdown — the same rows a created
  `config.toml` carries — and needs no KB. The third is `kbase://build-trail`, the `rewind-build`
  document as YAML — no target named, for the repository the KB sits in: the tool's own text, byte
  for byte, listed whether or not the repository holds a boundary. The fourth is `kbase://build`,
  the `status` document for the store the `build` tool last launched from this server — the served
  repository's default store where it launched none — listed always, answering `state: none` before
  any build. It is the one resource a client may subscribe to (`resources/subscribe`,
  `resources/unsubscribe`), and once subscribed the server sends
  `notifications/resources/updated` with that URI when the build stops progressing — finished,
  bounded, cancelled, failed or interrupted — so a session learns its build ended without asking.
  The frame carries the URI alone; read the document for the detail. A run already over when you
  subscribe is announced at once, after the subscribe response. Nothing else is served: the KB's
  Markdown documents are ordinary files the harness reads itself.
- **`build` is detached.** The tool starts the build as its own process and returns once the build
  has entered a stage or has exited, waiting at most 40 s; at the bound the result is `failed`,
  naming the state directory, and the build keeps running. Its result carries `wait`, the exact
  command to run in the background — `<kbase> wait --state-dir <store> --pid <pid>` — which returns
  when the run is over and prints that store's `status` document: run it in the background and your
  harness wakes the session when it exits, the outcome already in its output. Follow the build with
  `status`, stop it with `cancel`, resume it by calling `build` again with the same arguments. A
  client that subscribes to `kbase://build` needs no `wait`: the server sends the notification
  instead. The build's output is captured under the state store's `reports/`. Its stage boundaries
  are commits in the user's repository — say so before starting one.
- **Protocol.** The server implements one MCP revision, pinned as a constant (ARCHITECTURE §12,
  currently `2025-11-25`); an `initialize` naming another revision is answered with the server's
  for the client to decide. Calls are answered one at a time, in order. EOF on stdin ends the
  server. The server sends no sampling request, offers no prompts, and reads no harness
  configuration.

## Checking the integration

Two levels, both cheap.

1. **Pipe level, no harness.** Feed the server a handshake and a tool call on stdin and read the
   responses: an `initialize` request, the `notifications/initialized` notification, `tools/list`,
   then `tools/call` with `{"name": "stats", "arguments": {}}`, and `resources/list`. Each
   request gets one JSON-RPC response line; `tools/list` should name every bound tool (SPEC §11) and
   the one listed tool, `rewind-build`, and `stats` should return `outcome: done` in its structured
   content. The transcript fixtures under `test_data/fixtures/mcp/` are worked examples of the exact
   frames.
2. **In the harness.** Register the server, open a session in the repository, confirm the harness
   lists `kbase` as connected with its tools, and ask for something that needs a lookup ("how strong
   is <a named result> and what does it rest on"). A pass is the session answering through `find`,
   `show` and `deps` alone, with no file read of the index. On Claude Code this negotiated
   `2025-11-25` and the session answered through five tool calls; the harness's own MCP log records
   the negotiated revision if you need it.

## When something is off

- The result's `refusals` items say what was wrong and, where there is one, the `remedy`; the exit
  table in README maps outcomes to meanings (`retry` means a writer held the KB lock, re-issue
  later; a running build makes writers `refused`).
- A JSON-RPC error rather than a result means the request itself was malformed or failed the
  tool's schema.
- `--log-file <path>` on the server command captures diagnostics across the session; stderr has
  the same lines.
- A build that does not appear in `status` within the bound is still starting; `status` reports it
  once it has begun.

## What is deliberately out of scope

The server integrates one KB per process; serve two KBs with two entries. It does not install itself
into any harness. Its resources are the claim-graph sheets, the `[models]` guidance and the build
trail, each covered above; the KB's own documents are not resources — they are ordinary files the
harness reads itself. Nor does any tool name a configuration directory: `--config-dir` is the
server's flag, so the layers the server resolved are what `show-config` reports and `models` selects
its entry from, and `configure` writes into one of them — or into the served KB's repository layer,
when `--project` names it. `show-config` changes nothing (SPEC §9.6).
