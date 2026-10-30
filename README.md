# README – KBase

kbase is a Go binary that builds a knowledge base (KB) from LaTeX and provides tools to maintain it
afterward. Given a set of `.tex` entry point files, it writes a navigable Markdown tree plus
metadata describing a graph structure of the claims forming the argument within. The build process
starts with a mechanical pass and is refined by an inference pass that is tractable on local 
hardware.

![Volume Graph](readme-assets/kb-volume-graph.png)
![Claim Graph](readme-assets/kb-claim-graph.png)

## Developer
<https://keyoxide.org/wkd/benn@bennherrera.dev>

## Requirements and Supported Platforms

### Requirements
- **pandoc 3.12.** Required on the host, not shipped. Any pandoc whose `pandoc-api-version` is
  1.23.x is accepted; a missing or out-of-range pandoc is refused at preflight.
- **git 2.56.0.** Required on the host. A build records its progress as commits in your repository.
- **Graphviz `dot`**, optional, any version, on `PATH`. Draws the [claim-graph
  sheets](#claim-graph-sheets); without it they are not drawn.
- **A provider configuration**, for builds that spend inference and for scoring runs. A build run with
  `--no-inference` needs none, and so does a `kbase score --dry-run`. A scoring run asks the
  `scoring` role's endpoint, so `models.scoring` must name a model: the role is what declares which
  model judges the KB.
- **Go**, to build kbase from source. `just` runs the project's recipes.

Input is LaTeX only. A volume root is the paper's top `.tex` file: the one `00README.json`'s
`toplevel` entry names, else the sole file containing `\documentclass`.

### Platforms
- macOS (run and dev; installed through the Homebrew tap)
- Linux (installed through the Homebrew tap; dev experience untested, your mileage may vary)
- Windows (no distributed binary: build from source; dev experience untested, your mileage will absolutely vary)

## Installation

### Homebrew (macOS and Linux)

```sh
brew tap benn-herrera/kbase
brew install kbase
```

The formula builds kbase from the tagged release's source and puts it on `PATH`, with the tools it
needs; the [tap's README](https://github.com/benn-herrera/homebrew-kbase) lists them.

### Windows

No binary is distributed for Windows. Build it from source.

### From source

`just build` writes `bin/kbase` for the host platform; the [Development](#development) section lists
the recipes.

## Quick Start Guide

Put `kbase` on `PATH` — installed as above, or built with `just build` and run as `./bin/kbase`. The
examples below assume `PATH`.

Configure a provider. Skip this step for `--no-inference` builds.

```sh
kbase configure --create                    # writes providers.toml and config.toml, offline
$EDITOR ~/.config/kbase/providers.toml      # point [local] at your endpoint
kbase models                                # the model ids it offers
kbase configure --model-map decision=local/<id>,text=local/<id>,scoring=local/<id>
```

A `[models]` value is a reference: the `providers.toml` entry that supplies the endpoint, then the
model id it serves (`local/Qwen3.5-9B`). `configure --create` writes into `~/.config/kbase`, or into
`<repository>/.kbase/` where that directory already stands; add `--project` to name the repository's
`.kbase/` whether or not it exists. Without `--provider` it makes no request, so the ids it writes
are unchecked — `kbase models` and the last command above check them. Details are under
[Configuration](#configuration).

Build a KB inside a git repository:

```sh
git init paper-kb && cd paper-kb
kbase build /path/to/paper/main.tex
```

The KB appears at `kb-root/` beside `.git`. A fresh `kb-root/` starts at `kb-root/entry-point.md`
and descends through `index.md` files.

To build without spending inference, add `--no-inference`. The build drops the rows that spend
inference, states which ones, and still produces a real KB. No `README.md` is written into it,
because the overview passage is one of the dropped rows.

A KB can also be created empty, with no LaTeX and no build — `git init` for a KB:

```sh
git init notes-kb && cd notes-kb
kbase init
```

`init` writes the skeleton — an `entry-point.md` headed by the repository's name (or `--title`'s
text), `.index/` with the format stamp at `.index/format.yaml`, `AGENTS.md`, `CONVENTIONS.md` and
the `CLAUDE.md` redirect —
refreshes it, and commits `kb-root/` as the birth commit. It runs no `verify` of its own: `kbase
verify` over a freshly born KB is green. Creation is birth-once: a second `init` is refused, and
`kbase build` stays refused over a created KB. What the KB holds afterwards is authored with the
maintenance subcommands — insert an entry, set a field, mark a claim, refresh, verify — a claim at a
time. Details are under [SPEC §8](SPEC.md).

## Building a KB

```
kbase build <volume-root>... [--bibliography FILE]... [--no-inference]
            [--through <stage>] [--state-dir DIR]
```

- **Bibliographies.** By default every `.bib` beside the volume root is used, in sorted order.
  `--bibliography` replaces that set. An unreadable bibliography is reported and the build continues
  without it.
- **`--through <stage>`** stops after the named stage, by id or display name. A bounded run exits 0
  with outcome `bounded`.
- **Refusals.** The build refuses outside a git worktree (run `git init`), against a populated
  `kb-root/` with no `kb-build:` commit trail, over dirty paths it owns, on unparseable source, on
  an include it cannot load, when a stage that calls a model has no provider configured, and while
  another build runs on the repository (`check: lock`). A build over a non-empty commit trail given
  other volume roots or `--bibliography` files, or the same ones in another order, is refused with
  `check: inputs`, one item per differing input, before anything is restored or written. A finished
  build re-run with other inputs is refused, not reported `unchanged`. Each refusal names what it
  found.
- **Determinism.** The same inputs and pandoc version give a byte-identical document tree; the claim
  graph is identical except for the node ids, which are minted fresh on every build and are unique
  within the KB.
- **No judgement.** Every scoring edge a build writes carries `*pending*` as its bearing, and no
  stage writes a `rigor` or any other number into a score. A claim a build adds an edge to reads
  unchanged until you judge that edge with `set-bearing`.

Stages, in order: `start`, `document-graph`, `spine-seed`, `claims-declared`, `claims-discovered`,
`equations-minted`, `references-found`, `claims-culled`, `depends-attributed`, `analyses-split`,
`phase-3a`, `overview-drafted`. What each does is in [ARCHITECTURE.md](ARCHITECTURE.md) §4.

`claims-culled` asks, for each pair of linked claims with a prose end, whether one adds nothing to
the other; a prose claim that adds nothing is removed, and its links pass to the claim it was culled
into. It adds no edge: `depends-attributed` asks every surviving link its direction. Under
`--no-inference` it is walked without asks. It records its verdicts and the surviving links in
`kb-build-cull.yaml`.

`analyses-split` splits each claim whose text runs past the line it is titled by. The claim keeps
that line. A new analysis carries the rest: the claim's own block or paragraph, or, where that holds
nothing more, the proof that establishes it. The claim's premises and cited works move onto the
analysis, and the analysis lifts the claim, its bearing left for you to judge. It asks no model.

### Resuming and monitoring

Each stage boundary is a commit in your repository, with subject `kb-build: <stage-id> | <display
name>`. `kbase build <volume-root>...` starts a build; `kbase build --resume` continues it from the
last boundary on the commit trail, and takes no volume roots or `--bibliography`. After an
interrupted stage, it restores the paths the build owns to the last stage commit and re-runs that
stage.

The build owns its inputs. A relative volume root or `--bibliography` resolves against the project
root, and the build orders each list by full path, whatever order you gave it in, so the order you
give does not change the result. Every boundary's body ends with a `volume-root:` line per volume
root, then a `bibliography:` line per `--bibliography` file, each absolute and in that order; the
`start` boundary also records `started-at:`, the commit the build began from. On a branch with no
commit, the build first makes an empty `kb-start:` commit to start from. A resume continues with
the inputs the newest boundary records.

A build given volume roots over a trail is refused (`check: inputs`). The remedy names both routes:
continue with `kbase build --resume`, or start over from `started-at` — `rewind-build`'s
`started-at` target, or `git reset --hard <started-at>`, with the hash written out. A different
input set is a fresh build, reached only by starting over. So is a full build after a
`--no-inference` one: `kbase build --resume` over the finished trail is `unchanged`, and the dropped
rows run only in a build that starts over from `started-at` without `--no-inference`.

To go back to an earlier boundary, reset the repository to it and run `kbase build --resume`.
`kbase status` names each stage's commit; over [MCP](#using-kbase-from-an-agent-harness-mcp) the
`rewind-build` tool and the `kbase://build-trail` resource report the whole trail, and `started-at`,
the clean, ready-to-build state before the build. For a boundary you name they print both commands
— the `git reset --hard` and the recorded resume — ready to run; for the `started-at` target, the
reset alone, after which a build is fresh and names its own volume roots. The reset is yours: the
tool reports it and changes nothing. It discards every boundary newer than the one you reset to,
which the document counts as `discards`. A dirty worktree does not stop it; `dirty` lists what the
reset would lose.

`build` runs in the foreground (through the [MCP server](#using-kbase-from-an-agent-harness-mcp) it
is detached). From the repository, in a second shell:

```sh
kbase status   # state, pid, stage commits, progress, the resume command
kbase cancel   # stop the build; costs only the calls in flight
```

State that does not belong in the repository lives in `$XDG_STATE_HOME/kbase/<key>/`, or in
`--state-dir`: the pid and holder lock that `status` and `cancel` read, `progress.jsonl`, per-call
evidence, the answer cache, and the captured output of a build the MCP server started. It holds
paid-for inference and survives `git clean`.

The store is versioned: `<state-dir>/format` is `1`, and `build`, `status`, `wait`, `cancel`, the
MCP `build` tool and the MCP `rewind-build` tool refuse a newer store (`check: state-dir`). Each build
keeps the newest six captured outputs of builds the MCP server started (`reports/build-*`), and the
answer cache, multichoice-ask records and call captures of the current build trail and the five before
it. `status` reports `cache-entries` and `cache-bytes`. Deleting the store is safe but
forfeits the cache.

The operation lock is not in the store: it is `kbase-operation.lock` in the repository's git
directory, so one run — a build or a scoring run, never both — runs per repository whatever
`--state-dir` each names.

The build records are tracked at the repository root, as YAML: `kb-build-node-pass.yaml`,
`kb-build-unmarked.yaml`, `kb-build-classification.yaml` and `kb-build-cull.yaml`.

## Maintaining a KB

A KB created empty by `kbase init` and one distilled by `kbase build` are maintained the same
way — the subcommands below operate on any living `kb-root/`. Over a created KB `kbase build`
stays refused forever (SPEC §5's fresh-vs-resume reads the `kb-build:` trail, and `init`'s birth
commit is a plain subject); content joins through the write ops, a claim at a time.

After a build, subcommands operate on the living KB: write ops that insert entries and set fields,
`render-citation`, queries over the claim graph, `refresh` and `verify`, and `export-latex`, which writes a LaTeX project from the KB through pandoc; `score` asks the configured
scoring model for the faults each item's text commits, one item at a time, and prices them into the
KB's rigor and bearing values.
The queries are `find`, `show`, `deps`, `gated-on`, `cited-by`, `referenced-by`, `strength`,
`subtree`, `stats` and `edges`. Run `kbase --help` for the list and
`kbase <command> --help` for each one.

- Write ops take values as YAML (JSON accepted) on stdin or `--values`. Re-issuing an op leaves
  `kb-root/` byte-identical, and the second call reports `unchanged`. An insert adopts an existing
  node of the same title and host instead of writing a second one: one hosted in the `document` the
  insert names, else one of that title no leaf hosts yet. A claim's, an analysis's or an
  experiment's host is the leaf its statement marker stands in, and which `claim-rigor.md` register
  page shows it follows from that host.
- **Scores.** A node's own value is `rigor`, a scoring edge's is `bearing`, and a claim's or an
  analysis's derived value is `strength`. Two ops record a judgement: `set-rigor` writes a node's
  `rigor` and `set-bearing` a scoring edge's `bearing`; `set-bearing` replaces an existing edge
  record's bearing and never creates an edge. A claim or analysis with no
  `rigor` reads `*pending*` — unscored, not low. Its derivation pends with it unless a lift stands
  over it: a node corroborated and never judged publishes the lift's product, while a refutation
  alone leaves its strength unknown. A `bearing` of `*pending*` leaves its edge out of the
  derivation until it is judged.
- Three write ops are kbase's own. `drop-claim-edges` removes a claim's `rests-on` or `references`
  list. `drop-inbound-edges` removes every edge that names it. `drop-claim-node` deletes the node
  with every edge naming it, in one call, and refuses while any other line still names it.
- A write op ends with a `refresh` unless given `--no-refresh`. `kbase verify` then checks freshness
  and links under `kb-root/`; the citation grammar is checked inside the build.
- List queries return `--limit` results (default 50; `0` means all) from `--offset`, and report
  `count` and `truncated`. When `truncated` is true, re-issue with `--offset` at the offset plus the
  results returned.
- `show` on a claim lists its `strengthen_by` items (`item_idx`, `text`, `mentioned_ids`), and a
  node's record carries its `rigor` — `strength` too on a claim and an analysis; `deps` reports each
  edge as `relation`, `direction` (`in` where the other end supports the node, `out` where it
  supports the other end), `target`, `bearing` and `context`.
- Maintenance never requires git.

**Scoring.** `kbase score` asks the configured scoring model — `models.scoring` — one item at a
time, and asks it for the
**faults the text commits**, not for a number: a list of charge lines, each a fault id and the span
of the item's own text that shows it, or `none`. kbase prices the list with the
pass's table and writes the result through `set-rigor` or `set-bearing`. The `rigor` pass covers
every claim, analysis and experiment; the `bearing` pass every `supports`, `lifts` and `refutes` edge.
A run walks both, the node set and then the edge set, because one text edit moves a node's rigor and
the bearing of every edge that touches it: `--pass rigor` or `--pass bearing` scopes a run to one set,
for one edited subtree or a pass that is failing. Two values are never asked, both because the
document they rest on lies outside the corpus: a work's rigor, and a `founds` edge's bearing. Judge
each by hand, with the document in hand, through `set-rigor` and `set-bearing`. The two ask templates,
the ids and the weights are stated once, in
[RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md); the ledger is where a
value can be checked against its charges.

Every attempt is recorded under `<state-dir>/score/<run>/`: `run.yaml` names the run, the set it walks
(one pass, or `both`), what answered (`backend`: `provider`), the model and the digest
of the ask its judgements answer; `ledger.jsonl` holds a line per attempt with its own item's pass, the
faults it recorded, the reply where it parsed, the
value it prices to, the model and the prompt's digest. `--resume <run>` continues a run without
re-asking what it already landed, while a fresh run asks everything again. Because a
value is a projection of the line's ids, a resume **re-prices** a run when the table has moved since:
it lands the values that moved through the same write path and records them in `repriced.yaml`, with
no model asked. A resume under a changed ask — a run older than kbase's current instructions —
continues under the current ask: the header keeps the digest the run began under, and nothing refuses
and nothing asks you to choose. `--limit N` stops after N items, and the result carries the `resume`
command — its flags — whenever an item is still not `done`. A failed run lists what failed under `failures` — an item and the reason its ledger
line recorded, or a refresh's own error — and a cancelled one names the item it stopped before. The
reason names what ended the item's call: `provider-refused` where the endpoint answered with a status
that is not success, its status and its own words beside it, and `transport-failure` where the
connection, the name, the certificate or the stream failed and no answer was had. A
`bounded` run carries no `failures`: its failures are counted in `counts.failed` and reasoned in the
ledger.
`--dry-run` lists the set, asks nothing and writes no charge record. A run refreshes once at its
start — on an older KB that is the write that converts and saves it — and once after its last value
or the values a resume re-priced, each refresh holding the KB write lock; a dry run does neither.

`--scoring-model` names the model for one run: a `<provider>/<model>` reference, which supplies the
role where `models.scoring` names none and overrides it where it does. Every ask is one chat call
on that endpoint, re-issued identically up to the transport attempt count while it does not complete,
pausing 5 s then 30 s between attempts — as long as the item's deadline covers the pause: a pause it
cannot cover is not waited out, and the item fails at once naming the attempts it made. A status that
is not retryable — any non-2xx other than 429
or a 5xx — fails the item at once. Every ask of a run carries the run's id as its
`prompt_cache_key`, so a run's asks reach one provider and share one cache entry.
A run asks a model for minutes: at a terminal `kbase score` blocks until the walk ends, and as an
[MCP](#using-kbase-from-an-agent-harness-mcp) tool it starts a detached child and returns at once with
the run's identity, `kbase wait` reporting the outcome.

**Locks.** A write op and `refresh` hold the KB write lock for the whole run.
A writer that cannot take it within 30 s gets `retry`. While a build runs, these are refused at once
(`check: lock`). A build waits out a write in progress for up to 30 s. `kbase score` takes it around
its one landing and each of its two refreshes, never across the run. A resume restores every path
under `kb-root/` to the last boundary, so edits made while a build is interrupted are lost unless
carried in git. `render-citation`, `verify` and the queries take no lock. Locks are advisory and
Unix only.

**Demoted edges.** When the build — or `refresh`, repairing what a merge left — breaks a dependency
cycle, it cuts one edge of the cycle and records it as a `demoted` edge that keeps the edge it
replaced whole, relation and bearing included. `verify` lists each under `findings` with outcome
`done`, `stats` counts them as `demoted_edges`, and the sheets draw them as cuts. `resolve-demoted`
takes `entry` items of `id`, `target` and `action`: `remove` deletes the demoted edge, `restore`
puts back the edge it replaced and is refused (`check: dependency-cycle`) where that would close a
cycle. A write op that adds a scoring edge closing a cycle is refused the same way.

**Format migration.** A KB's metadata format is `3.0.0`, stamped `kb-format` in the one-key file
`.index/format.yaml`, and its documents carry no frontmatter at all: its nodes and edges are kbase's
records in `.index/nodes.yaml` and `.index/edges.yaml`, and its `claim-rigor.md` register pages are
generated from them. A KB in an older form, `2.0.x` or `1.0.0`, is converted in memory when it is
loaded and
answers every command as converted; the result document says so in a `migration` record, whose
`warnings` list what the conversion dropped or changed, and stderr says the same. The next write —
`refresh`, a write op's trailing refresh or a scoring run's start — saves the conversion: it writes
the records, strips every other document's frontmatter, writes `AGENTS.md` and `CONVENTIONS.md`
whole from the current templates, renders the register pages, removes the files the form no longer
has (listed under `removed`), strips the entry point's frontmatter too and writes `3.0.0` to the
stamp file last. Every converted rigor and bearing is marked model-scored, so a default `score` run
re-judges it. Read-only commands write nothing. A write op's `--no-refresh` on an unmigrated KB is refused
(`check: index`, remedy `kbase refresh`), and `verify` reports it stale with the same remedy. A KB
stamped with a newer major or minor is refused (`check: kb-format`; update kbase). `status` and
`cancel` do not load the KB.

### Claim-graph sheets

`refresh` draws the claim graph as SVG: `kb-root/claim-graph.svg`, the whole
KB with every volume as a cluster; and, where two or more volumes hold nodes,
`kb-root/claim-graph-digest.svg`, one box per volume with the edges between volumes counted, and
`kb-root/<volume>/claim-graph.svg` for each volume (its claims plus every claim one edge away).
Every arrow runs from the supporter to the node it supports: a claim or an experiment `supports` a
claim or an analysis; a work or an axiom `founds` one; and a claim, an analysis or an experiment
`lifts` or `refutes` one. Each relation has one stroke of its own, and the cut a cycle-breaking
demotion made is drawn dotted. Nodes are styled by kind and filled by strength band, a pending one
grey, and every node links to its place: a claim's or a work's entry on its register page, an
analysis's or an experiment's section in the leaf that hosts it, an axiom's heading in `axioms.md`.

The sheets need Graphviz's `dot` on `PATH`, any version. Without it, existing sheets are left as
they are, `refresh` writes a placeholder `kb-root/claim-graph.svg` only where none exists and
reports `sheet: placeholder`. Sheets are rewritten only when their content
changes. A digest or volume sheet the KB no longer calls for is removed by the next render and
reported under `removed`. The drawing is specified in [SPEC.md](SPEC.md) §3.

### Using kbase from an agent harness (MCP)

`kbase mcp` serves kbase to any agent harness that speaks the Model Context Protocol, over stdio.
Every subcommand it serves is a tool of its own or an operation of the `query` or `write` tool, with
the same arguments and the same result; `rewind-build`, which reports the build's commit trail, has
no subcommand behind it.

The server entry is command `kbase` with one of two argument lists. A harness that runs kbase from
inside the project uses TUI mode, where the server finds its project from the working directory:

```json
["mcp"]
```

A harness that serves many projects from one place, with no project working directory, uses GUI
mode, where every tool call names its project with `project-dir`:

```json
["mcp", "--project-dir-required"]
```

`kbase mcp` takes no `--project-dir`; given one, it is refused before serving.

Two examples. Claude Code, in `.mcp.json`:

```json
{"mcpServers": {"kbase": {"command": "kbase", "args": ["mcp"]}}}
```

opencode, in `opencode.json`:

```json
{"mcp": {"kbase": {"type": "local", "command": ["kbase", "mcp"], "enabled": true}}}
```

These are examples; any MCP-capable harness adds its own equivalent entry.

- **Tools.** `query` runs the queries (including `show` and `stats`) and `render-citation`, and
  `write` runs the write ops: `operation` names the subcommand and `arguments` carries what it
  takes — a query's positionals and flags by name, or one values entry — and a bad argument is a
  usage refusal naming it and every argument the operation takes. Every other subcommand is a tool
  of its own: `init`, `show-config`, `models`, `configure`, `refresh`, `verify`, `export-latex`,
  `status`, `cancel`, `build` and `score`. `completion`, `help`, `mcp`, `probe` and `auth` are not
  offered, and neither is `wait`: a held call would spend the turn the build or score run takes, so the
  `build` and `score` tools hand you their command to run in the background instead. `models`
  reads a provider's model catalogue and `configure` writes `config.toml`; neither performs
  inference — `build` and `score` are the tools that do. A write op takes one entry and offers
  no `--no-refresh`.
  `rewind-build` is the exception to one tool per subcommand: it lists the build's boundaries,
  oldest first, and for a `target` you name — a stage id or display name — returns the
  `git reset --hard` and the recorded re-issue command, with `discards` counting the boundaries the
  reset removes. It is read-only; running the reset is yours.
- **Every tool takes `project-dir`:** the absolute path of the git worktree root of the project the
  call runs in. In TUI mode it is optional: a call that omits it runs in the server's own project,
  and on a server with no project of its own it is refused, naming `project-dir`. In GUI mode it
  is required. A path that is not a git
  worktree root is refused. Where a `build` or `score` call names it, the `wait` and `resume`
  commands it hands back carry `--project-dir`.
- **`build` is detached.** It starts the build as its own process and returns once the build has
  entered a stage or has exited, waiting at most 40 s. On entering a stage it returns the
  `state-dir`, `pid`, `resume` command and `wait`, the exact background command that returns when
  the build is over and prints its `status` document; follow it with `status` and stop it with
  `cancel`. Run `wait` in the background and your harness wakes the session when it exits, with the
  outcome in hand. At the bound the result is `failed`, naming the state directory and saying the
  build is still starting; the build keeps running, and `status` reports it once it begins. The
  build's output is captured under the state store's `reports/`. A client that subscribes to
  `kbase://build` needs no `wait`: the server sends one notification when the build stops
  progressing.
- **`score` is detached.** It starts a scoring run as its own process and returns once the run's
  ledger and pid stand or it has exited, waiting at most 40 s. On a started run it returns `run`,
  `ledger`, `pid` and `wait`, the exact background command that returns when the run is over and
  prints the run's document — the counts by ledger status, the items whose values landed and the
  ones that failed with their reasons. Run `wait` in the background and your harness wakes the
  session when it exits. Follow the run with `status`, stop it with `cancel`, and continue it by
  calling `score` again with `--resume <run>` and the same arguments. At the bound the result is
  `failed`, naming the state directory, and the run keeps running. The run's output is captured
  under the state store's `reports/`.
- **Resources.** The claim-graph sheets are resources, `kbase://sheet/<path under kb-root>`, read as
  SVG. `kbase://model-hints` is the model guidance as markdown — the same rows a created
  `config.toml` states — and needs no KB. `kbase://build-trail` is the `rewind-build` document as
  YAML, with no target named, for the repository the KB sits in; it is listed whether or not the
  repository holds a boundary. `kbase://build` is the `status` document for the store the `build`
  tool last launched, and the one resource a client may subscribe to: once it is subscribed the
  server sends `notifications/resources/updated` with that URI when the build ends — finished,
  bounded, cancelled, failed or interrupted — and the client reads the document for the detail.
- **Requirements are unchanged.** Sheets still need `dot`, and builds still need pandoc and a
  provider configuration, on the host that runs `kbase mcp`.

### Output and exit codes

Every subcommand but `mcp` writes one YAML document to stdout; stderr is for humans. (`mcp`'s stdout
carries JSON-RPC frames; `--help` and `--version` are human-facing.) The `outcome` key decides the
exit code.

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success |
| `refused` (including verify faults and usage errors) | 1 | Wrong input or KB state |
| `retry` | 2 | A writer that could not take the KB write lock within 30 s, or a build starting while another holds the state store; re-issue identically later. A running build makes writers `refused` with `check: lock`, not `retry` |
| `failed` | 3 | Defect in the tool, its input or the environment |
| `cancelled` | 4 | Stopped on request; resumable |

## Configuration

Configuration is read from two layers: `<repository>/.kbase/` over `~/.config/kbase`. A directory
`--config-dir` names, else `$KBASE_CONFIG_DIR`, is instead the one directory read, so an explicit
choice isolates a run from a repository's own `.kbase/`. Outside a git worktree there is no
repository layer, and configuration still works: `show-config`, `models` and `configure` run in any
directory. The verbs that read configuration load all three files strictly (SPEC §9 names them): an
unknown key fails that file's load and that run, naming the key and the file, and no lower layer is
read in its place.

- **`providers.toml`** is the endpoint pool, merged by entry name: a repository entry of the same
  name replaces the global entry whole, so a repository never inherits the global entry's key file
  for an endpoint it chose. Each top-level table is a provider you name — one or more ASCII letters, digits, `_` or `-`, not beginning with `-`, and names differing only by case are refused — with `baseUrl` (required).
  Give it its key with `kbase auth add <name>` (below): a stored key belongs to the name, whatever
  layer states the entry. An entry may instead name a key file with `apiKeyFile` — for CI and
  mounted secrets — and a relative path resolves beside the file; a stored key outranks it, and
  `kbase show-config` prints the command that moves such a key into the store. A leading
  `~` in `apiKeyFile`, `--config-dir` or `$KBASE_CONFIG_DIR` expands from your
  home directory; `~user` does not. kbase warns if a key file it reads is group- or world-readable;
  `chmod 600` it. An entry's `api` names the chat protocol its server speaks — `anthropic` (the
  default, the Anthropic Messages protocol) or `openai` (the OpenAI-compatible `chat/completions`
  surface), which an entry selects with `api = "openai"`. Use Messages for any role whose thinking
  is on: it caps the thinking a reply may spend, where the OpenAI-compatible protocol only hints at
  an effort the server may exceed without bound. The `decision` role always asks the OpenAI-shaped
  decision surface on its entry's base URL, whatever `api` says, so an `anthropic` entry serves it
  only where its server also speaks that surface. An entry's `decisionsFlavor` names which decision surface its server speaks —
  `systemone` (the default) or `decisions` — and changes nothing about chat, which `api` decides, and
  nothing about the model, which is still `config.toml`'s `models.decision`.
- **`config.toml`** holds the choices: `[models]` `decision`, `text` and `scoring`, each a
  `<provider>/<model>` reference — the entry that supplies the endpoint, then the model id it serves,
  split at the first `/` (a model id may contain one, a provider name may not). A bare model id there
  is refused, naming the key and the `kbase configure` command that rewrites it. A created file also
  states, beside `[models]`, what each role wants and example ids to choose from — the same guidance
  an MCP client reads at `kbase://model-hints`. The layers merge `config.toml` field by field: a
  value the repository states wins, and one it leaves out keeps the global value. `kbase configure`
  writes it and preserves every other byte of an existing file. A `config.toml` stating a top-level
  `provider` key or `[asks] readerConcurrency` fails to load, naming the key; delete the line.
- **`kbase configure --create`** writes an annotated `providers.toml` where none stands — nothing
  else creates it — and overwrites no other content. `config.toml` is created where absent by a run
  that resolves a provider, or by `--create`
  with no `--provider`, and edited in place otherwise. It writes the layer a flag or
  `$KBASE_CONFIG_DIR` names, else `<repository>/.kbase/` where that directory
  already stands, else `~/.config/kbase`; `--project` names the repository's `.kbase/` whether or not
  it exists, and is refused outside a worktree. Without `--provider` it makes no request and says the
  model ids went unchecked; with one it dials, warns on an id the catalogue lacks, and a refusal — or
  a failed dial or listing — writes nothing. A dialled run requires all three roles, except a
  `scoring` value the target file already holds, which it leaves standing.

**The files and the store are the whole configuration.** The endpoint and the model each role uses
come from `providers.toml` and `config.toml` alone, and its key from the store, else the entry's key
file; no environment variable supplies any of them. To keep a run apart from your own layers, name
its directory with `--config-dir` or `$KBASE_CONFIG_DIR`; that leaves your store in use, which
`$XDG_STATE_HOME` moves.

Provider selection — `--provider`, else the sole `providers.toml` entry — belongs to `models` and
`configure` alone: they select an entry to dial, which is not a role. A `--provider` the pool does not
hold refuses against the `providers.toml` of the layer `configure` writes into — the file an entry
belongs in — and offers `kbase configure --create` unless that provider faulted out of the pool,
where the fault's own reason stands instead. A pool with no usable entry refuses against that same
file. In a build, the claim graph's multichoice asks go to the `decision` role and the overview passage to
`text`; in a scoring run, the asks go to `scoring`.

`kbase show-config` reports how all of this resolves, and changes nothing: no KB read, no file
written, no request made. It states the layers read and what named each; the pool, with each entry's
layer, key kind (`store`, `file` or `none`, never the key), decision surface and any fault; `config.toml`'s merged `[models]` values; the
selected entry and rule; and one record per role, naming the endpoint, the layer that supplied
each field, the model asked, or what is missing. Gaps are diagnostics in a `done` result, not
refusals, so one call shows them all.

`kbase probe [--model <provider>/<model>] [--decision | --text | --scoring] [--prompt TEXT]` asks
one model one question and reports what came back: the selection asked, the provider, the model, the
time to the first token, the call's wall time, the finish reason, the reply, and the usage the
provider reported. A probe asks one selection: `--decision`, `--text` and `--scoring` each ask the
selection the configuration resolves for that role — `models.<role>` in the file — and `--model`
names a reference to reach instead, resolved against the same pool. The four are exclusive; with
none of them the probe asks the `scoring` role — the selection a run would make — so
`kbase probe` with no arguments is the heartbeat of a configuration. The document names the selection
that answered. It reads no KB and reads or writes no state directory: one ask, one result document.

`kbase auth add <provider>` is how you give an entry its key. It stores a provider's API key, read from stdin and nowhere else, one
encrypted file per `providers.toml` entry under `$XDG_STATE_HOME/kbase/auth/`
(`~/.local/state/kbase/auth/` by default). Pipe the key in from your key manager or clipboard —
`<command-that-prints-the-key> | kbase auth add <provider>` — so it never stands on a command line
or in a terminal; a terminal on stdin, empty input, or a name no `providers.toml` entry carries is
refused. The encryption keeps the key out of
anything that reads the file; it is no protection against your own OS account.
`kbase auth remove <provider>` clears a stored key and `kbase auth list` names the providers that
have one. These verbs are CLI-only: no MCP tool takes a key.

Global flags on every subcommand: `--config-dir`, `--log-level debug|info|warn|error` (default
`warn`), `--log-file`, `--project-dir` (the git worktree root that holds `kb-root/`; the search for
the repository starts there instead of the working directory). A relative path a subcommand takes
resolves against the project root; `--config-dir` and `--log-file` resolve against the working
directory. The root command also takes `-v`/`--version`. `kbase mcp` finds its project from the
working directory, as every subcommand does, unless run in GUI mode (`--project-dir-required`); it
refuses `--project-dir`.

## Development

```sh
just             # list recipes
just edit-gate   # cheap gate: run after every change
just checkpoint  # full suite: run at checkpoints
just test        # unit tests (VERBOSE=1 for per-test output)
just tag         # signed tag v<version> of a vetted tree; local only
just tap-update  # after a pushed release: the tap formula's url and sha256, committed in the tap
```

`just tag` refuses while tracked files have uncommitted changes, then creates a signed annotated
tag `v<version>` locally, the version being what the binary reports, and verifies its signature; it
runs no gates, so vet the tree first, and it pushes nothing. No built binaries are committed;
`bin/` is gitignored. An agent driving kbase reads its operating contract from kbase itself:
`kbase --help` and `kbase <command> --help` on the command line; over MCP, the server's
instructions, the tools, and each tool's or operation's help at `kbase://help/<name>`.

Integration recipes run `./bin/kbase` itself, except the live-fixture recipes, which run the
fixture's own copy (below), and keep their evidence under `test-data/transient/`.
A recipe excluded from `just test-integration` says so, and what it needs, in `just --list`.

`just prep-test-integration-fixture <dir>` stages a fixture repository. It resets the repository to
its `cleared` tag, installs the agent set for both harnesses, and copies `bin/kbase` to
`<dir>/.kbase/kbase`; it also copies `tools/harness.sh` to `<dir>/.kbase/harness.sh`. The fixture's root `.gitignore` at `cleared` must ignore `.kbase/`, which also
keeps its test configuration out of its history; staging refuses one that does not, before it
changes anything. The live
fixture recipes run that copy. A harness session over a staged fixture finds kbase on `PATH`:
start it from the fixture as `PATH=$(pwd)/.kbase:${PATH} claude` (or `opencode`), or
as `.kbase/harness.sh <harness>`, which does that and shows the `kbase` it resolves. No recipe
starts one.

## Contract documents

- [THESIS.md](THESIS.md): why the build has its shape.
- [SPEC.md](SPEC.md): what any compliant implementation must do.
- [ARCHITECTURE.md](ARCHITECTURE.md): how this implementation meets the spec.
- [CONVENTIONS.md](CONVENTIONS.md): house rules and practices.

## License

[MIT](LICENSE).

## Third Party Acknowledgements

Directly consumed Go modules (versions in `go.mod`):

| Library | Owner | License | Use |
|---|---|---|---|
| [BurntSushi/toml](https://github.com/BurntSushi/toml) | TOML authors | MIT | Strict decoding of `providers.toml` and `config.toml` |
| [spf13/cobra](https://github.com/spf13/cobra) | spf13 | Apache-2.0 | Command-line structure, flags and help |
| [spf13/pflag](https://github.com/spf13/pflag) | spf13 | BSD-3-Clause | Flag parsing behind the MCP server's tool schemas and its flag reset |
| [go.yaml.in/yaml/v3](https://go.yaml.in/yaml/v3) | YAML organization | MIT and Apache-2.0 | YAML values input, tool results and build records |
| [golang.org/x/text](https://pkg.go.dev/golang.org/x/text) | The Go Authors | BSD-3-Clause | Unicode NFC normalization in ingest; case folding in queries |

pandoc (John MacFarlane and contributors) and Graphviz `dot` (Graphviz authors) are run as external
programs and are not bundled.
