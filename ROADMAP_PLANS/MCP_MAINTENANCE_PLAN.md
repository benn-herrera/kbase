# MCP_MAINTENANCE_PLAN.md — the KB's formal tool surface, over the CLI and MCP

**Status: Phase 1 is under way as `ACTIVE_PLAN.md` tranche 2, under SPEC §11, which supersedes this
document's Results and Protocol subset where they differ (structured content, sheet resources, the
read-only annotation, the exclusion list, `isError` from the exit code); `tree` and `document` are
not in tranche 2. Phase 2 (stitching, topography, region checks) stays here, not started.** This
plan adds the operation catalogue, the module skeleton, the registration entries, the protocol
subset and its tests, the stamped-document changes, the contract changes, and the rows. Phase 1
binds what kbase already does; Phase 2 is new capability, built in kbase first. Open questions are
at the end and are few on purpose.

## What this is for

After a build the KB is canonical and LaTeX is derived (SPEC §1). Maintenance is people and agents
working inside the KB: exploring, deriving, synthesizing, adding and pruning nodes and documents,
and stitching the claim graph as they go. Today that reaches the KB through the `kbase` subcommands
(SPEC §8) and the stamped `AGENTS.md` / `CONVENTIONS.md` (`internal/kbdocs/templates/`). This plan
gives it one formal surface: the `kbase` CLI as the portable baseline, and a stdio MCP server,
`kbase mcp`, exposing the same operations to any harness that speaks MCP. kbase need not know which.

The usage pattern — build, settle in, explore, draft, integrate, repeat — is an example. Nothing
below assumes or enforces that order or any part of it.

## Invariants

Each states what it prevents. These add to ARCHITECTURE §2 (I1–I7) and CONVENTIONS; the rules that
already cover the rest are named at the end of the list.

1. **One implementation per operation (the `cmd/` rule).** Every tool body is one call into a
   subcommand's options-struct function (CONVENTIONS, Module Structure). The tool table is the
   command tree, built by `cmd/mcp.go` and handed to the server; it is never a list typed in
   `internal/mcp`. A test asserts the tool names equal the command tree's subcommands less `mcp`,
   `models` and `configure`. `build`'s body starts the same subcommand detached (below). *Prevents:*
   a second implementation that drifts from the CLI; a tool with no CLI counterpart on a harness
   without MCP.
2. **Refusals are intrinsic only, and the list is closed:** a dangling link the op would write, an
   edge to a missing node, a duplicate id, a register entry without its node, a `depends` cycle, and
   the refusals SPEC §7 and §8 already state. An orphan claim, an unscored claim, a leaf with
   `no-claim:`, a draft with no edges are all valid. A tool never refuses on sequence ("mint before
   wire"), on location ("promote only from `topics/`"), or on completeness. *Prevents:* tools that
   encode the example workflow as a rule.
3. **A refusal explains, and a cycle refusal names the path it would close** — `t → … → s → t`.
   *Prevents:* a refusal the model can act on only by guessing.
4. **Every mutating tool leaves the derived layer fresh and reports what moved.** It runs SPEC §8's
   trailing refresh, and its result carries the nodes whose `solidity` or `build_band` changed, read
   from `internal/index` before and after. A write answering `retry` is re-issued identically up to
   a constant number of times by the tool (I6 makes that safe), not by the model. *Prevents:* an
   obligation — "remember to refresh", "retry three times" — left to a model's tirelessness.
5. **No git outside `internal/ledger` (I1), and no model call reachable from the maintenance
   tools.** `internal/mcp` and the region checks import nothing that reaches `internal/model` or
   `internal/asks/call` (ARCHITECTURE §7; the import-policy test gains the rule). Region checks
   return the build's own rendered asks and the letter meanings; the session's model judges. `build`
   is the one tool that spends inference, exactly as the subcommand does and under the same
   configuration (SPEC §9: flags, then the `KBASE_*` environment, then the configuration files). The
   server never sends the client a `sampling` request: the build's inference stays the build's —
   pinned model, call shape, captures — and the maintenance and navigation tools need none; which
   model a harness's agents run on is the consumer's choice and has no effect on any post-build
   operation. *Prevents:* a second inference configuration; a question text that diverges from the
   build's; a hidden second reader; a build whose decisions depend on whichever model the harness
   happened to hold.
6. **The server's stdout carries JSON-RPC frames and nothing else.** Each tool body runs the
   subcommand's function with its `Stdout` set to a buffer; the buffer, which holds the SPEC §7
   result document, is the tool's result text (I5: the result emitter still writes the document, to
   the buffer). Logging goes through `internal/log` to stderr and `--log-file`. *Prevents:* a stray
   write corrupting the stream.
7. **Stdlib only; no MCP SDK; protocol version one constant**, `mcp.ProtocolVersion`. The SDK would
   add a dependency CONVENTIONS "Dependency Policy" has not sanctioned. *Prevents:* a dependency the
   distribution does not carry.
8. **Boundary checks at the frame.** Each incoming frame is checked for `jsonrpc == "2.0"`, an `id`
   of string, number or null, a string `method`, and — on `tools/call` — a known tool and arguments
   that are a JSON object carrying the schema's required keys with the right JSON types. The KB is
   the one `kbase mcp --kb-root <path>` was started for; with no flag the subcommand's own
   resolution from the process working directory applies, and a miss is that function's refusal,
   never a crash. Cheap, always on. *Prevents:* a stack trace where the model needed a sentence; a
   server whose KB depends on where a harness happened to start it.
9. **Harness-agnostic server; the specification is the target.** The server never reads a harness
   config file, names no harness, and assumes nothing about its working directory, environment or
   lifecycle beyond what the MCP specification at its current revision states. A live check on a
   particular harness is conformance evidence, never a requirement the design is shaped to.
   *Prevents:* a tracked tool knowing which harness started it; design skew toward one harness's
   habits.

Already covered, so not restated as invariants: the CLI is the baseline and a capability lands as a
subcommand first (the `cmd/` rule, one file per subcommand; a tool is a subcommand's binding, not a
separate surface); the write vocabulary stays single-sourced (CONVENTIONS, "Keep no prose copy of an
op's key vocabulary": a write tool's `inputSchema` comes from `internal/write`'s `opFields` through
one exported function, and the arguments pass through the same checks a values document does); no
pre-pass or prompt from an in-code string (CONVENTIONS, House Rules); drafting location is not an
invariant because no operation here constrains it.

Not adopted: switchable metrics (nothing here is performance-critical; per-call wall time at `debug`
suffices); a new build target (`just edit-gate` and `just test-integration` cover the new tests).

## Results

- **Text.** A tool result's `content` is one `text` item holding the subcommand's SPEC §7 result
  document, unaltered.
- **`isError`** is true exactly when the document's `outcome` is not `done` or `unchanged`.
  `bounded`, `refused`, `retry`, `failed` and `cancelled` are all errors to a harness.
- **Items.** The item shape is SPEC §7's, so a model corrects a refusal from `key`, `entry`,
  `allowed` and `remedy`.
- **Paging.** List queries take `limit` and `offset` as arguments with the semantics SPEC §7 states:
  default `query.DefaultLimit` (ARCHITECTURE §12), `0` means all, `truncated` says when to re-issue
  at `offset + len(results)`.
- **`build`'s result** is as SPEC §7 states for the document the subcommand writes; what the tool
  returns at start is the detached form below.

Tool names are the subcommand names, so the two surfaces share one vocabulary; under Claude Code
they appear as `mcp__kbase__<name>`.

## Operation catalogue

The tool's `inputSchema` is derived: a read or control tool's from its command's flags and declared
positional arguments (global flags excluded; the server carries its own), a write tool's from
`opFields` (a values document's one key, `entry`, is the argument). `--no-refresh` is not offered
through a tool (invariant 4).

### Phase 1 — read, navigate, control

Binds what exists. About 18 tools.

| Tool | Subcommand | Arguments (sketch) | Returns |
|---|---|---|---|
| `deps`, `gated-on`, `cited-by`, `find`, `referenced-by`, `solidity-below`, `subtree`, `weak-points` | the SPEC §8 queries, as `kb_cmd --json` names them | the query's positionals; `limit`, `offset` | `count`, `offset`, `truncated`, `results` |
| `show`, `stats` | the same | `show`: `{id}`; `stats`: `{}` | `results`: the mapping |
| `verify` | `verify` | `{}` | outcome `done`, or `refused` with the findings as items |
| `refresh` | `refresh` | `{}` | `written`; exposed for after a hand edit — every write tool runs it already |
| `render-claim-graph` | `render-claim-graph` | `{}` | `written`, `sheet` |
| `status`, `cancel` | `status`, `cancel` | `{state-dir?}` | SPEC §6's snapshot; `cancel`'s `state-dir`, `pid`, `resume` |
| `build` | `build` | the subcommand's flags and volume root | detached start, below |
| `tree` | new subcommand | `{path?, depth?}` | the topography under `path`: each document's path, kind, title, children |
| `document` | new subcommand | `{path}` | kind, title, parent, children, frontmatter fields, hosted ids, the absolute path for the harness's own read tool; no body text |

`tree` and `document` read the document tree and frontmatter through `internal/kb`'s readers
(`kb.Documents`, `kb.TreeLinks`, `kb.ParseFrontmatter`) and the leaf record; neither touches
`.index/` and neither refuses anything but an unknown path (`check: unknown-id` is for ids; the path
case is `check: usage`, `key: path`).

**`build` over MCP is detached.** The tool starts `kbase build` with the supplied arguments as a
detached process and returns at once with `state-dir` and the `resume` command; the harness follows
it through `status` and stops it through `cancel`. The tool waits only until the child has taken the
run lock or exited, bounded by a constant in ARCHITECTURE §12; a child that exits first — a refusal
at start (SPEC §5) — has its result document returned as the tool's result, so a refused start is an
error and not a silent launch. The child's stdout and stderr are captured to a state-store path
ARCHITECTURE §8's layout names. The alternative, CLI-only `build`, keeps a harness's session out of
an hours-long run at the cost of a harness that cannot start one; the plan chooses detached.

### Phase 2 — stitching (new capability, built in kbase first)

Existing write ops and `render-citation`, bound as-is. Schema: `opFields[<op>]`; effect and refusals
as SPEC §8 and the cited "The Write API's Contract" state them. Each tool call is one `entry` list,
as a values document is, all written or none. The values file stays the batch channel on the CLI.

| Tool | Effect on the claim graph | Effect on the topography |
|---|---|---|
| `insert-claim-entry`, `insert-support-entry`, `insert-experiment-entry`, `insert-work-entry` | mints or adopts the node and its register entry; `ids`, `minted`, `adopted` | none — hosting is `set-frontmatter`'s |
| `add-depends-on` | adds `depends-on` / `references` edges; **gains the cycle refusal** (below) | none |
| `set-frontmatter` | hosts ids in a leaf; declares `supports` / `strengthens` ends | writes the block; `kind` is the caller's |
| `mark-claim-in-leaf` | places a Tier-2 marker | none |
| `set-rigor`, `set-on-point-fraction`, `set-work-strength`, `set-applicability` | the scores; the cascade is the refresh every write runs, reported as what moved | none |
| `set-rationale` | prose on an entry | none |
| `render-citation` | none (read) | none; `citations` |

**The cycle refusal (`add-depends-on`).** Today a cycle is caught only after the write, by `verify`
through `internal/index`'s `CycleError`, which carries the cycle's members, not a path. The gap is a
path finder, `dependency_path(entries, start, end)`, in `internal/index` beside solidity, walking
the authored `depends-on` graph (`rests-on` excluded: a work emits no edge). The op calls it for
each new target and refuses with an item `check: dependency-cycle`, `key: depends-on`, its `entry`
position, and a `detail` reading `<id> depends-on <t> would close a cycle: <t> → … → <id> → <t>;
drop the edge, or record it under references`. No `remedy`: no command clears it. A cycle forced in
by editing a register by hand stays `verify`'s business.

New write ops, each with a row in `opFields` and a subcommand file under `cmd/`:

- **`remove-edge`** — `{id, target, class: "depends-on" | "references"}`. Removes one bullet from
  the entry's named list. Refuses: the entry does not hold that edge (names what it holds).
  Topography: none. `supports` / `strengthens` ends are removed by restating the hosting leaf's
  block through `set-frontmatter`.
- **`retire-node`** — `{id}`. Removes a register entry; for a claim, also its Tier-2 marker and its
  membership in the hosting leaf's `claims:`, every other frontmatter attribute carried forward
  (CONVENTIONS, "A frontmatter writer carries forward every attribute it does not own"); for an
  experiment or support, the leaf's declaration block likewise. Refuses: any entry's `depends-on`,
  `references`, `supports` or `strengthens` names the node (lists the edges; `remove-edge` first);
  the id does not resolve. Topography: none. An orphan this leaves — a leaf now hosting nothing — is
  valid and reported, not refused.
- **`place-document`** — `{destination, title, kind: "leaf" | "index", body, no_claim?: string}`.
  Creates a tree document from supplied text: the up-link to the parent (`<dir>/index.md`, or
  `entry-point.md` for a volume directory), the frontmatter block (`kind`, and for a leaf `no-claim:
  <no_claim>`; claims are hosted later by `set-frontmatter`), and the child link in the parent
  index, appended last. A `kind: index` destination `<dir>/index.md` creates the directory and links
  it from its own parent. Refuses: destination exists; parent index missing; a link in `body` that
  resolves to nothing (dangling, named); destination under `kb.ExcludeDirs` or outside `kb-root/`.
  Claim graph: none.
- **`promote-document`** — `{source, destination, title, kind, no_claim?}`. `place-document` with
  the body read from `source`, a `.md` under a directory in `kb.ExcludeDirs`; the source is deleted
  after the placed copy is proven. Same refusals, plus: `source` not under an excluded directory, or
  absent. The two share one placement function; the op differs only in where the body comes from and
  in consuming the source.
- **`move-document`** — `{source, destination}`. Renames a tree document: rewrites its up-link,
  moves its child link from the old parent index to the new one (appended last), rebases its body's
  relative links (`kb.RebaseInlineLinks`), and rewrites the leaf link in every register entry the
  document hosts. For an index, the whole directory moves and every descendant's links are rebased.
  Refuses: source not in the tree; destination exists; new parent index missing; a link elsewhere in
  the tree that resolves to the old path (named — the caller edits the citing documents first).
  Derived fields (`Leaf references:` footers, `subtree-claims`, `.index/`) are the refresh's.
- **`prune-document`** — `{path}`. Deletes a tree document and its child link in the parent index.
  Refuses: it hosts any node (names the ids; `retire-node` first or `set-frontmatter` to re-host
  elsewhere); it is an index with children; another document links to it (named). Claim graph: none,
  by construction of the refusals.

The four topography ops read and write link lines in leaves, which ARCHITECTURE I2 does not admit
for maintenance ops as it is worded (see Risks).

### Phase 2 — region checks

All three are mechanical, read the tree the way the build does (`internal/claimgraph`'s stage
machinery, which this reuses), make no model call, and never mint. They are read-only subcommands
with outcome `done` and a `results` key. `scope` is a kb-root-relative path: a directory (prefix) or
one document; absent means the whole KB.

- **`region-unminted`** — `{scope?}`. For each leaf in scope: the paragraphs the node pass would ask
  about (its opening-words minimum and obligated-paragraph rule applied), less paragraphs already
  carrying a claim marker and less lines inside claim-bearing blocks. Returns, per leaf, the group
  prefix of the rendered `paragraph.tmpl.md` ask **once** and per paragraph the item tail — the
  split `asks.GroupRecord` records a build's asks with — with the letter meanings, the paragraph's
  locator (what `mark-claim-in-leaf` takes), and, where the tracked node-pass record
  `kb-build-node-pass.json` holds one, the build's recorded verdict for that paragraph as
  information. Claim graph and topography: untouched.
- **`region-unmarked`** — `{scope?, k?}`. The `references-found` stage's shortlist (ARCHITECTURE §4)
  over the authored graph, with its pair set widened by every edge the registers already carry
  (`depends-on` and `references`) so an authored pair is never re-proposed; sources filtered to
  scope; `k` defaults to the stage's constant. Returns per pair the rendered ask (prefix once per
  source, tails per target) with the letter meanings, and the two ids `add-depends-on` takes.
  Untouched graphs.
- **`region-status`** — `{scope?}`. Information, never errors: claims with no edge in either
  direction over `query.Index`'s dependency edges (a new degree-zero accessor), entries at
  `*pending*` (rigor, applicability, strength), leaves carrying `no-claim:`, documents without
  frontmatter, and — if `kb-build-node-pass.json` stands — leaves the build left unread.

## Module skeleton

```
cmd/
  mcp.go                      the subcommand: builds the tool table from the command tree, runs
                              mcp.Serve on stdin/stdout. Imports internal/mcp.
  tree.go, document.go        Phase 1 navigation subcommands over an options-struct function.
  remove-edge.go, retire-node.go, place-document.go, promote-document.go, move-document.go,
  prune-document.go           Phase 2 write ops, through writeOpCommand.
  region-unminted.go, region-unmarked.go, region-status.go
                              Phase 2 region checks.
internal/mcp                  framing, dispatch, the Tool type (name, description, inputSchema,
                              call), inputSchema derivation from a command's flags and declared
                              positionals, the frame checks, stdout capture. Imports internal/result
                              and internal/log only. Never a KB package, never `model`.
internal/write                new ops and their opFields rows; one exported function returning a
                              write op's schema, read by cmd when it builds the table; the up-link
                              and child-link composers are read from internal/kb (below).
internal/kb                   the up-link line and index child-link bullet composers, lifted out of
                              the one place the build spells them (internal/docgraph/tree.go) so
                              docgraph and write share them; the navigation readers behind
                              tree and document.
internal/index                dependency_path beside solidity.
internal/query                a degree-zero accessor on Index.
internal/claimgraph           the region checks, over the stage machinery already here.
```

Dependency direction follows ARCHITECTURE §11: `mcp` is imported by `cmd` alone, and `cmd` reaches
`write`, `index`, `query` and `claimgraph` as it does today. The tool table is injected into `mcp`,
so `mcp` imports no module that reaches `model`. `write` imports `kb` for the composers; `docgraph`
imports them from `kb` and is retired from its own spelling only after its tree is byte-identical on
the tracked fixtures.

What does **not** belong: no tool list in `internal/mcp`; no cobra in `internal/mcp`; no harness
name anywhere in it; no JSON-RPC outside it; no `print` to stdout in a subcommand's function that is
not its declared result (the server's buffer would carry it into the document).

## Registration

Registration is not kbase's. The harness boundary is the protocol: `kbase mcp` is a command a
harness starts, and nothing in kbase writes a harness file. README carries, as examples for two
harnesses, the one config entry a user adds by hand, named `kbase`, each naming the `kbase mcp`
command with the KB's location explicit:

- Claude Code — `.mcp.json` at the project root, under `mcpServers`: `{"mcpServers": {"kbase":
  {"command": "kbase", "args": ["mcp", "--kb-root", "<absolute path to kb-root>"]}}}`
- opencode — `opencode.json` at the project root, under `mcp`: `{"mcp": {"kbase": {"type": "local",
  "command": ["kbase", "mcp", "--kb-root", "<absolute path to kb-root>"], "enabled": true}}}`

The explicit `--kb-root` is what makes the entry independent of the working directory a harness
chooses; omitting it falls back to resolution from the working directory, for a harness known to
start the server in the project root. `command` is `kbase` where the binary is on the harness's
`PATH`, else its absolute path. Any other harness that speaks MCP adds the equivalent entry in its
own format; the two above are examples, not a supported set.

## Protocol subset

Newline-delimited JSON-RPC 2.0 over stdio: one object per line, UTF-8, no embedded newline. The
server is single-threaded and answers in order. EOF on stdin ends it with exit 0.

- `initialize` → `{protocolVersion, capabilities: {tools: {listChanged: false}}, serverInfo: {name:
  "kbase", version}}`, the version from `internal/version`. If the client's `protocolVersion` equals
  `mcp.ProtocolVersion` it is echoed; otherwise the server answers with `mcp.ProtocolVersion` and
  the client decides. The constant's value is whatever both harnesses negotiate in the live check —
  the current MCP revision when this lands — and is pinned there, not here.
- `notifications/initialized`, and every `notifications/*` → no response, logged at `debug`.
- `ping` → `{}`.
- `tools/list` → every tool with `name`, `description`, `inputSchema` (`type: object`, properties,
  required); no cursor.
- `tools/call` → `{content: [{type: "text", text}], isError}`, per Results. A refusal or failure
  inside a tool body is an `isError` result; protocol faults are JSON-RPC errors: `-32700` parse (id
  null), `-32600` not a request object or a batch array, `-32601` unknown method, `-32602` unknown
  tool or arguments failing the frame check (invariant 8), `-32603` an error escaping the dispatcher
  itself (logged).
- No server-to-client requests; no `roots`, `sampling`, `resources`, `prompts`. `structuredContent`
  not emitted.

### Test plan

Transcript fixtures, `test_data/fixtures/mcp/*.jsonl`: one transcript per case, alternating request
lines and the expected response line (or a marker for "no response"). Two harnesses run them:

- **Unit, in-memory streams.** Protocol cases run in `internal/mcp` through `Serve(in, out, table)`
  with a stub table; cases that need a KB run in `cmd/`, over a temporary repository the way
  `cmd/maintenance_test.go` stages one. Cases: initialize with the pinned version; initialize with
  another; `notifications/initialized` yields no line; `ping`; `tools/list` names equal the command
  tree less `mcp`, `models`, `configure`; every `inputSchema` is an object schema; `tools/call
  stats`; `tools/call find` paged; an unknown tool is `-32602`; a non-JSON line is `-32700` with
  `id: null`; a batch array is `-32600`; an unknown method is `-32601`; a tool whose function writes
  to its `Stdout` leaves exactly one frame on the stream; a refusal is `isError: true` with the item
  keys intact. Phase 2 adds: an insert returns `ids` and `verify` is `done` afterwards;
  `add-depends-on` closing a cycle is `isError` naming the path; a staged `retry` is absorbed; each
  topography refusal; `region-unminted` over a fixture lists exactly the node pass's asked
  paragraphs minus the marked ones.
- **Process boundary.** Recipe `test-integration-mcp` runs the Phase 1 transcripts through
  `./bin/kbase mcp` over real pipes against the tracked fixture KB under
  `test_data/fixtures/results/kb-root/`, staged the way `test-integration-results-fixture` stages
  it, and compares each response line to the fixture's. It needs git only, so it joins the hermetic
  `test-integration` omnibus; it preserves its log and artifacts where CONVENTIONS, "Testing"
  requires.

One-time live check per harness, run by the owner in an interactive session (it needs the harness):
add the README entry to a scratch project, confirm the harness lists `kbase` as connected, call
`stats` and `tree`, read the negotiated `protocolVersion` off the harness's MCP log, and confirm the
explicit `--kb-root` was honoured whatever the server's working directory was. Evidence — the
version string, the harness release, the `tools/list` the harness showed — goes in the landing
commit message; `mcp.ProtocolVersion` and the two README entries are what it fixes. Repeat on a
harness release that moves MCP.

## Stamped KB documents

The stamped templates under `internal/kbdocs/templates/` (`AGENTS.tmpl.md`, `CONVENTIONS.tmpl.md`)
say how a KB is maintained: they give the maintenance commands for both toolchains (SPEC §4). They
gain the tool surface: the maintenance operations are reachable as the CLI and, where the harness
has it, as the `kbase` MCP server exposing the same operations; stitching is available to the
session on offer or direction; orphan and pending are valid states. Neither document states or
implies a required order. These are model-facing text and get prompt-engineer review before first
use (CONVENTIONS, House Rules). Already-built KBs keep their stamped copies, since `phase-3a` stamps
only if absent (ARCHITECTURE §4): a risk, below.

## Contract-document changes

Routed per the document set: behaviour to SPEC, mechanism to ARCHITECTURE, traps to CONVENTIONS.
Named here; written in the rows.

- **SPEC.md** — a new "Tool surface" statement: the CLI is the baseline; `kbase mcp` exposes the
  same operations over stdio JSON-RPC to any harness; the closed refusal list (invariant 2) and what
  is not refused (orphans, pending, drafts anywhere); every mutating operation leaves the derived
  layer fresh and reports what moved; no maintenance operation touches git or a model; `build` over
  MCP is detached. §1 consumers gain MCP harnesses. §7: `mcp`'s stdout carries frames, an exception
  beside `--help` and `--version`; write ops gain the key reporting what moved; `tree`, `document`
  and the region checks gain rows. §8: subsections for `remove-edge`, `retire-node`,
  `place-document`, `promote-document`, `move-document`, `prune-document`, `region-unminted`,
  `region-unmarked`, `region-status`, `tree`, `document`, and the cycle refusal at `add-depends-on`.
  §4: a divergence row for each new op, the cycle refusal and the tool surface — an unlisted
  difference in maintenance-op behaviour or the tool interface is a defect.
- **ARCHITECTURE.md** — §11: `internal/mcp` in the package map and its dependency rule (imported by
  `cmd` alone; reaches no `model`), `internal/kb`'s composers, and the new `index` and `query`
  members; a short "MCP server" section with the protocol subset and the stdout rule; §8's state
  layout names the detached build's capture path; §12 gains the retry count and the start-wait
  bound; I2 re-worded if the owner accepts the Risks entry on it.
- **CONVENTIONS.md** — the traps that survive translation: the server's stdout carries frames only;
  a workflow refusal is a defect (a tool refuses only the closed list); the tool table is the
  command tree — never list a tool by hand; protocol tests are transcript fixtures under
  `test_data/fixtures/mcp/`, not asserted strings in code.
- **README.md** — the two registration entries and the `test-integration-mcp` recipe.

## Rows

Done-whens are observable. Every coder row includes the contract-document edits its change
invalidates, and ends green on `just edit-gate`. Prompt-engineer rows produce model-facing text.

### Phase 1 — read, navigate, control

| # | Owner | Files | Done when |
|---|---|---|---|
| 1.1 | go-coder | `internal/mcp` (new), `cmd/mcp.go`, `cmd/importpolicy_test.go`, `test_data/fixtures/mcp/*.jsonl`, SPEC, ARCHITECTURE, CONVENTIONS, README | `kbase mcp --kb-root <path>` serves that KB from any working directory, and without the flag resolves as the subcommands do; the in-memory transcript cases for Phase 1 pass; a test asserts the tool names equal the command tree less `mcp`, `models`, `configure`; `importpolicy_test.go` asserts `mcp` is imported by `cmd` alone and reaches no `model`; every `inputSchema` is derived from flags and declared positionals, and a test asserts each command with positionals declares them; a tool that writes to its `Stdout` leaves one frame; `--log-level` and `--log-file` are honoured through `internal/log` |
| 1.2 | go-coder | `cmd/tree.go`, `cmd/document.go`, `internal/kb` navigation readers, `cmd/resultkeys_test.go`, SPEC §7–§8 | `kbase tree` and `kbase document` print the results described above over the tracked fixture KB; `document` on an index lists its children in index order; an unknown path is a `check: usage` refusal; the result-key tests cover both |
| 1.3 | go-coder | `cmd/mcp.go`, `cmd/build.go` (the detached start only), `internal/build` state-store layout, ARCHITECTURE §8, §12 | the `build` tool returns with `state-dir` and `resume` while the child runs; `status` through the tool reads `running`, `cancel` through the tool stops it with the build resumable; a start the build refuses returns the refusal document with `isError: true`; the child's capture lands at the path ARCHITECTURE §8 names |
| 1.4 | shell-dsl-coder | `justfile` | `just test-integration-mcp` passes through `./bin/kbase mcp` over pipes on the staged fixture KB, each response line equal to its transcript's; the recipe is in the `test-integration` omnibus and its `[doc]` says it is hermetic |
| 1.5 | prompt-engineer | `internal/kbdocs/templates/AGENTS.tmpl.md`, `CONVENTIONS.tmpl.md` | the stamped text names the CLI and the MCP tools for reading and navigating, says nothing yet about writing, and implies no order; the stamp tests in `internal/kbdocs` pass |
| 1.6 | owner | — | the live check above, both harnesses; `mcp.ProtocolVersion` and the two README entries confirmed or corrected in one commit, evidence in its message |

### Phase 2 — stitching, topography, region checks

| # | Owner | Files | Done when |
|---|---|---|---|
| 2.1 | go-coder | `internal/write` (schema function), `cmd/mcp.go`, `internal/mcp`, `cmd/resultkeys_test.go`, SPEC §7 | the schema function covers every `opFields` row and a test asserts its `required` equals the row's required fields and every property is typed; every existing write op is a tool; a write result carries what moved and a staged `retry` is re-issued and absorbed; `--no-refresh` is not an argument; every existing write test passes unchanged |
| 2.2 | go-coder | `internal/index`, `internal/write` ops, SPEC §4, §8 | `dependency_path` has unit tests over a synthetic cycle and an acyclic graph; an `add-depends-on` values document closing a cycle exits 1 with the path in the item's `detail`; `verify`'s cycle check is untouched |
| 2.3 | go-coder | `internal/write` ops and `opFields`, `cmd/remove-edge.go`, `cmd/retire-node.go`, SPEC §4, §8 | both ops are tools; each refusal above has a test; after each op over the fixture KB `verify` is `done` with the refresh run; `retire-node` carries forward every frontmatter attribute it does not own |
| 2.4 | go-coder | `internal/kb` (composers), `internal/docgraph/tree.go`, `internal/write`, `cmd/place-document.go`, `promote-document.go`, `move-document.go`, `prune-document.go`, SPEC, ARCHITECTURE | the composers exist in `internal/kb` and `docgraph` reads them, its tree byte-identical to before on the tracked fixtures and the importpolicy test green; the four ops are tools; each refusal has a test; after each op `verify` is `done`; `promote-document`'s source is gone only after the placed copy is proven; `move-document` of a claim-hosting leaf leaves its register link resolving |
| 2.5 | go-coder | `internal/claimgraph` region checks, `internal/query` degree-zero accessor, `cmd/region-*.go`, SPEC §8, ARCHITECTURE | over the fixture KB `region-unminted` lists exactly the node pass's asked paragraphs minus the marked ones, each with a rendered ask whose prefix plus tail equals the build's composed ask for it; `region-unmarked` equals the `references-found` shortlist minus authored edges, filtered by scope; `region-status` counts agree with `stats` where they overlap; the importpolicy test asserts no path to `model`; the first step records which of the three checks the leaf pages alone support (Risks) |
| 2.6 | go-coder | `internal/mcp`, `cmd/mcp.go`, `test_data/fixtures/mcp/*.jsonl` | the Phase 2 transcripts pass; the tool-name test holds over the full command tree; `refresh` and the three region tools are listed |
| 2.7 | shell-dsl-coder | `justfile` | `test-integration-mcp` also drives a write transcript (insert, `add-depends-on` cycle refusal, `place-document`, `prune-document`) and ends with `./bin/kbase verify` outcome `done` |
| 2.8 | prompt-engineer | `internal/kbdocs/templates/AGENTS.tmpl.md`, `CONVENTIONS.tmpl.md` | the stamped text reflects the Stamped KB documents section and the maintainer seat as the open question below resolves it; both templates stamp; the stamp tests pass |

### Acceptance, the review loop's exit

- Every tool has a CLI counterpart and the two give the same document on the fixture KB.
- A harness configured with the README entry lists `kbase` connected in both harnesses.
- Minting with no edges, scoring an orphan, and placing a draft leaf hosting nothing each leave
  `verify` outcome `done`.
- A `depends-on` cycle is refused at the write naming its path; nothing else about edge order or
  document order is refused.
- No package but `internal/ledger` execs git; `internal/mcp` and the region checks reach no `model`.
- The server survives a tool function that writes to its `Stdout`: one frame per request on the
  fixture transcripts.

## Risks

- **I2 as worded.** ARCHITECTURE I2 says maintenance ops read only the metadata layer, and that the
  inventory's reader and placement facts come from records, never from leaf markup. The topography
  ops read and rewrite link lines in leaves, and the region checks need reader facts that live in a
  build's state store, which a KB built by kb_tools or a clone elsewhere does not have. Row 2.5
  establishes what the pages alone support; if either need stands, I2 is amended in the same change,
  a ruling for the owner before the row lands.
- **Tool-list cost.** About 40 tools at ~200–400 tokens each is ~10K tokens of definitions in every
  session. Descriptions are one sentence and schemas the vocabulary; if it binds, fold the narrow
  read tools (`gated-on`, `cited-by`, `subtree`) first, not the refusal text.
- **Protocol version moves with harness releases.** Pinned constant plus the live check, repeated on
  a release that changes MCP; a harness refusing the pin is loud (no tools), not quiet.
- **Harness trust prompts.** Claude Code asks the user to approve a project `.mcp.json` the first
  time; opencode's behaviour is confirmed in the live check. Neither is this tool's to suppress.
- **Working directory.** With `--kb-root` given, none. Without it the KB is resolved from the
  server's working directory, and a harness that starts the server elsewhere fails every call with
  the KB-root refusal naming the probe; README's entries carry the flag so the fallback is never
  relied on.
- **Full refresh per write.** Affordable at today's scale; the delta read loads the index twice.
- **Region-check output size.** A leaf's ask embeds the whole labelled body; prefix-once bounds it
  to one body per leaf or per source. A whole-KB `region-unmarked` is still large; the stamped text
  names a directory as the default scope.
- **Already-built KBs keep the old stamped text** in their `AGENTS.md` / `CONVENTIONS.md`. The
  remedy is the user's: delete the file and let the next `phase-3a` re-stamp, or edit it. A re-stamp
  op is not in this plan.
- **`move-document` leaves citers to the caller.** Refusing when another document links to the old
  path is the minimum; rewriting citers is a follow-up if the refusal proves frequent.
- **Parallel writers.** The server and a dispatched agent may write the same register;
  `internal/filelock`'s lock and the `retry` outcome cover it, and the tool absorbs the retry.
- **Shared composers.** Moving the up-link and child-link composers into `internal/kb` touches the
  build; row 2.4's byte-identical comparison on the fixtures gates it. Fallback: leave the build's
  spelling where it is and accept two composers with a test asserting they agree.

## Open questions for the owner

1. **`build` over MCP.** The plan makes it detached (Operation catalogue, Phase 1). If that is not
   accepted, the alternative is CLI-only: `build` leaves the tool table, `status` and `cancel` stay,
   and the tool-name test's exclusion list gains `build`.
2. **The maintainer seat.** With the session holding the full tool surface, the maintainer's
   remaining job is dispatched batch landing with file-ownership parallelism. Whether that seat
   stays is adjagent's to decide; the stamped templates name it, so the answer fixes their text (row
   2.8). The plan assumes it stays.
