# Integrating kbase into an agent harness over MCP

For the person wiring kbase into a harness. This document is the map: it says what the server is,
how to stand it up, what a harness sees, and where the rest is written. It does not restate the
contract; where a detail matters, it names the document and section that holds it.

## What you are integrating

kbase is a single Go binary that builds a knowledge base (KB) from a LaTeX document set and
maintains it afterwards: a tree of Markdown documents with a claim graph over them, held in a
`kb-root/` directory inside a git repository. Every operation on a KB is a subcommand of `kbase`.
`kbase mcp` serves those same subcommands to an agent harness over the Model Context Protocol
(MCP), stdio JSON-RPC, one KB per server process. A harness without MCP loses nothing but
convenience: every tool is one subcommand with the same arguments and the same result.

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
   Serving an existing KB for reading needs none of pandoc, git or a provider.
2. **Build:** `just build` produces `bin/kbase` for the host platform. `just test` runs the unit
   suite; `just test-integration` the hermetic integration suite, which includes the MCP transcript
   recipe (`test-integration-mcp`: fixed request/response transcripts fed through `bin/kbase mcp`
   over real pipes). Both should be green on a fresh clone.
3. **Provider configuration** is needed only by `build`, the one tool that spends inference. The
   server passes its own environment and `--config-dir` to the build it starts (README
   "Configuration"; SPEC §9 for the `KBASE_*` variables and precedence).
4. **A KB to serve.** Either one already built (a repository with `kb-root/` in it) or one you
   build first with `kbase build <volume-root.tex>` from the repository root. A KB built by the
   Python reference implementation (kb_tools) at an older metadata format is read as is and migrated
   on its first write (SPEC §10).

## Registering the server

The whole registration is one command line:

```
kbase mcp --kb-root <absolute path>
```

`--kb-root` names the `kb-root/` directory or the repository root holding it. Without it the KB is
found from the server's working directory, which is fine when the harness launches the server from
the repository. Use an absolute path when it does not. Any other path is refused before the server
serves. Global flags `--config-dir`, `--log-level` and `--log-file` apply; `--log-file` is the
place to look when something is off, since stdout carries protocol frames and nothing else.

README's MCP section shows the entry for two harnesses (Claude Code's `.mcp.json` and opencode's
`opencode.json`). Any harness adds its own equivalent: a server named `kbase`, the command above,
stdio transport. There is no registration automation in this repository, by design; the entry is
the harness's.

## What the harness sees

- **Tools.** One per subcommand, named as the subcommand: the write ops (`insert-claim-entry`,
  `set-rigor`, `add-depends-on`, `resolve-demoted`, …), `render-citation`, the queries (`find`,
  `show`, `deps`, `gated-on`, `cited-by`, `referenced-by`, `solidity-below`, `subtree`,
  `weak-points`, `stats`), `refresh`, `verify`, `render-claim-graph`, `status`, `cancel` and
  `build`. Not offered: `completion`, `help`, `models`, `configure`, `mcp`. Each tool's input schema
  is derived from its command's arguments and flags; a write op's schema is one entry with the
  op's closed set of value keys and no others. The queries, `verify`, `status`, `render-citation`
  and `show` carry the read-only annotation.
- **Results.** Every tool returns the result document of SPEC §7 twice: as the text content (YAML)
  and as `structuredContent`. `isError` is set exactly when the document's exit code is not 0, so a
  refusal reaches the model as an error that still carries its items and remedy. Arguments that
  fail the schema are JSON-RPC `-32602` errors, not result documents. List queries page with
  `limit` and `offset`.
- **Resources.** The claim-graph sheets on disk, as `image/svg+xml` resources with URI
  `kbase://sheet/<path under kb-root>`; `resources/read` returns the SVG as text. Nothing else is
  served: the KB's Markdown documents are ordinary files the harness reads itself.
- **`build` is detached.** The tool starts the build as its own process and returns once the build
  has entered a stage or has exited, waiting at most 40 s; at the bound the result is `failed`,
  naming the state directory, and the build keeps running. Follow it with `status`, stop it with
  `cancel`, resume it by calling `build` again with the same arguments. The build's output is
  captured under the state store's `reports/`. Its stage boundaries are commits in the user's
  repository — say so before starting one.
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
   request gets one JSON-RPC response line; `tools/list` should name 30 tools and `stats` should
   return `outcome: done` in its structured content. The transcript fixtures under
   `test_data/fixtures/mcp/` are worked examples of the exact frames.
2. **In the harness.** Register the server, open a session in the repository, confirm the harness
   lists `kbase` as connected with its tools, and ask for something that needs a lookup ("how solid
   is <a named result> and what does it rest on"). A pass is the session answering through `find`,
   `show` and `deps` alone, with no file read of the index. On Claude Code this negotiated
   `2025-11-25` and the docent answered through five tool calls; the harness's own MCP log records
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

The server integrates one KB per process; serve two KBs with two entries. It does not install
itself into any harness, does not expose the KB's documents as resources, and does not expose
provider management (`models`, `configure`), which stay on the command line for the person who
operates the host.
