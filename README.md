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
kbase configure-providers local --base-url <url>        # writes the provider [local] into providers.toml
<command-that-prints-the-key> | kbase auth add local   # its API key, if it needs one
kbase models --provider local                           # the model ids it offers
kbase configure-models --decision local/<id> --text local/<id> --scoring local/<id>
```

A `[models]` value is a reference: the provider that supplies the endpoint, then the
model id it serves (`local/Qwen3.5-9B`). Both configure commands write into `~/.config/kbase`, or
into the directory `--config-dir` or `$KBASE_CONFIG_DIR` names; `--local` writes
`<repository>/.kbase/` instead. `configure-providers` dials the provider where its key is already
stored, and otherwise prints the steps that follow, so it can run before the key is stored.
`configure-models` dials each provider it names and warns on a model id the catalogue lacks.
`configure-providers` refuses a base URL that carries credentials; store the key with `kbase auth add`. Details
are under [Configuration](#configuration).

Build a KB inside a git repository:

```sh
git init paper-kb && cd paper-kb
kbase build /path/to/paper/main.tex
```

The KB appears at `kb-root/` in the git worktree root, beside `.git`. A fresh `kb-root/` starts at `kb-root/entry-point.md`
and descends through `index.md` files.

To build without spending inference, add `--no-inference`. The build drops the steps that spend
inference, states which ones, and still produces a real KB. No `README.md` is written into it,
because the overview passage is one of the dropped steps.

A KB can also be created empty, with no LaTeX and no build — `git init` for a KB:

```sh
git init notes-kb && cd notes-kb
kbase init
```

`init` writes the skeleton — an `entry-point.md` headed by the repository's name (or `--title`'s
text), `.index/` with the format stamp at `.index/format.yaml`, `AGENTS.md`, `CONVENTIONS.md` and
the `CLAUDE.md` redirect —
derives its summary pages and claim-graph sheet, and commits `kb-root/` as the birth commit. It runs
no `verify` of its own: `kbase verify` over a freshly born KB is green. Creation is birth-once: a
second `init` is refused (`check: kb-root`), and so is `kbase build` over a created KB. What the KB
holds afterwards is authored with the write ops — add a node, edit its fields, add an edge, cite a
node — in write batches. Details are under
[SPEC §8](PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md).

## Building a KB

```
kbase build <volume-root>... [--bibliography FILE]... [--no-inference]
            [--through <stage>] [--state-dir DIR] [--ask-concurrency N]
kbase build --resume
```

- **Bibliographies.** By default every `.bib` beside the volume root is used, in sorted order.
  `--bibliography` replaces that set. An unreadable bibliography is reported and the build continues
  without it.
- **`--through <stage>`** stops after the named stage, by id or by display name; the ids are listed below. A bounded run exits 0
  with outcome `bounded`.
- **`--ask-concurrency N`** sets how many model calls are in flight at once, across every role
  (default: `[asks] concurrency` in `config.toml`, else 4).
- **Refusals.** The build refuses outside a git worktree (run `git init`), a fresh build wherever
  `kb-root/` exists, in any state (`check: kb-root`), over dirty paths it owns, on unparseable
  source, on an include it cannot load, when a stage that calls a model has no role configured, and
  while another build runs on the repository (`check: lock`). Each refusal names what it found.
- **Determinism.** The same inputs and pandoc version give a byte-identical document tree; the claim
  graph is identical except for the node ids, which are minted fresh on every build and are unique
  within the KB.
- **No judgement.** Every scoring edge a build writes carries `*pending*` as its bearing, and no
  stage writes a `rigor` or any other number into a score. A claim a build adds an edge to reads
  unchanged until that edge is judged, by `kbase score` or by `edit-edge` with a `bearing`.

Stages, in order: `start`, `document-graph`, `spine-seed`, `claims-declared`, `claims-discovered`,
`equations-minted`, `references-found`, `claims-culled`, `depends-attributed`, `analyses-split`,
`phase-3a`, `overview-drafted`. What each does is in [ARCHITECTURE §4](PROJECT_DOCS/ARCHITECTURE/4-stage-table/index.md).

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

A fresh build is refused wherever `kb-root/` exists, so a different input set is a fresh build
reached only by starting over: remove `kb-root/`, or reset to `started-at` (below). So is a full
build after a `--no-inference` one: `kbase build --resume` over the finished trail is `unchanged`,
and the dropped steps run only in a build that starts over from `started-at` and runs without
`--no-inference`.

`kbase build-trail` (also an [MCP](#using-kbase-from-an-agent-harness-mcp) tool) lists the trail,
read-only: each boundary oldest first with its commit and the command that rewinds to it, and
`started-at`, the clean, ready-to-build state before the build, with the command that starts over.
A boundary's command is `git reset --keep <commit>` followed by the recorded `kbase build --resume`,
carrying the flags the build's last run was launched with, the bound excepted; `started-at`'s is the
reset alone, after which a build is fresh and names its own volume roots. kbase runs none of them;
run them as listed. `git reset --keep` refuses rather than discard uncommitted changes in the files
it touches, and `dirty` lists the worktree's changed paths. `kbase status` lists each stage done or
not done, and names `build-trail` as where the commits are.

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

The store is versioned: `<state-dir>/format` is `1`, and `build`, `status`, `wait`, `cancel`,
`build-trail` and the MCP `build` tool refuse a newer store (`check: state-dir`). Each build
keeps the newest six captured outputs of builds the MCP server started (`reports/build-*`), and the
answer cache, multichoice-ask records and call captures of the current build trail and the five before
it. `status` reports `cache-entries` and `cache-bytes`. Deleting the store is safe but
forfeits the cache.

The operation lock is not in the store: it is `kbase-operation.lock` in the repository's git
directory, so one run — a build or a scoring run, never both — runs per repository whatever
`--state-dir` each names.

The build records are tracked in `kb-root/.build/`, as YAML: `kb-build-node-pass.yaml`,
`kb-build-unmarked.yaml`, `kb-build-classification.yaml` and `kb-build-cull.yaml`.

## Maintaining a KB

A KB created empty by `kbase init` and one distilled by `kbase build` are maintained the same
way — the subcommands below operate on any living `kb-root/`. Over a created KB `kbase build`
stays refused forever (`check: kb-root`; `init`'s birth commit is a plain subject, not a
`kb-build:` stage); content joins through the write ops.

After a build, the commands operate on the living KB:
- the write ops, each an entry of a write batch: `add-claim`, `add-analysis`, `add-experiment`,
  `add-work` and `add-axiom` add one node each; `edit-claim`, `edit-analysis`, `edit-experiment`,
  `edit-work` and `edit-axiom` change a node's fields; `remove-node` and `move-node` remove a node
  or move its statement marker; `add-edge`, `edit-edge`, `remove-edge` and `restore-demoted` write
  edges; `add-citation` turns written text into a citation of a node;
- the queries, which read and write nothing: `find`, `show`, `edges`, `trace`, `citations-of` and
  `stats`;
- `verify`, which checks the KB; `update-format`, which converts and re-derives it; and
  `export-latex`, which writes a LaTeX project from the KB through pandoc;
- `score`, which asks the configured scoring model for the faults each item's text commits, one item
  at a time, and prices them into the KB's rigor and bearing values.

Run `kbase --help` for the list and `kbase <command> --help` for each one.

- **Write batches.** `kbase write` takes an ordered batch as YAML (JSON accepted) on stdin or in
  the file `--values` names: entries under `entry`, each naming its op under `operation`, with
  `dry-run` and `commit` beside them. Each op's own subcommand takes a batch of that op alone, its
  entries naming no operation. Every entry's shape is checked first; the entries then apply in
  order, each against the records the earlier ones left, and the batch stops at the first refusal.
  A batch is all or nothing, its derived summary pages and sheets included. `--dry-run` reports the
  result and writes nothing; an empty batch (`printf 'entry: []\n' | kbase write`) re-derives the
  summary pages and sheets after leaf edits.

  ```yaml
  entry:
    - operation: add-claim
      title: Bounded sequences have convergent subsequences
      rationale: Bolzano–Weierstrass, as stated in the section's opening paragraph
      document: paper/section-2.md
      line: 41
      snippet: bounded sequence
  ```

  An add mints its node's id, known only from the result, so a node and the edges that name it go
  in two batches. An edit whose values already stand reports `unchanged`; a re-issued add or
  removal is refused, because the node exists or no longer does. `--commit` makes git the batch's
  undo: it commits the whole of `kb-root/` before and after the write, so reverting the batch's
  commit undoes exactly the batch. Without it nothing is committed, and only commits of your own
  undo the batch.
- **Scores.** A node's own value is `rigor`, a scoring edge's is `bearing`, and a claim's or an
  analysis's derived value is `strength`. The edit ops record a judgement: `edit-claim`,
  `edit-analysis`, `edit-experiment` and `edit-work` write a node's `rigor`, and `edit-edge` an
  existing scoring edge's `bearing`; only `add-edge` creates an edge. A value given by hand,
  `*pending*` included, marks the node or edge manually scored, which `score` leaves alone unless
  run with `--rescore-manual`; leave a value out of an add and `score` judges it. A claim or
  analysis with no `rigor` reads `*pending*` — unscored, not low. Its derivation pends with it
  unless a lift stands over it: a node corroborated and never judged publishes the lift's product,
  while a refutation alone leaves its strength unknown. A `bearing` of `*pending*` leaves its edge
  out of the derivation until it is judged.
- **Removal.** `remove-edge` removes one edge record, or with `relation: demoted` every demoted
  record from `source` to `target`. `remove-node` removes a node of any kind with every record
  naming it, every edge at either end included; it refuses while a citation of the node, or a
  surviving record's title, rationale or edge context, still names it. Run it with `--dry-run`
  first to see the edges it takes.
- `kbase verify` checks links, records, statement markers and summary pages under `kb-root/`; a
  node id standing bare in prose is a `bare-id` fault, and a citation is a Markdown link whose title
  is the cited node's id (`add-citation` writes one).
- The list queries `find`, `edges`, `trace` and `citations-of` return their whole answer, with
  `count` beside `results`; none pages. `find` and `trace` take `--limit` only beside an explicit
  `--sort`, and a negative `--limit` keeps the last N. `find` narrows by `--kind`, `--title`,
  `--label`, `--path`, `--min`, `--max`, `--pending` and `--min-dependents`; `--min 0` leaves out
  pending nodes.
- `show` returns a node's record: its `host`, `host_anchor` and `summary` place, and its `rigor` —
  `strength` too on a claim and an analysis. `edges` lists edges, at one node with `--id` and
  `--direction in|out|both`; each row carries `source`, `target`, `relation`, `bearing`, `context`,
  `manually_scored` and `replaced_relation`. `trace <id> --direction in|out` walks the scoring edges
  transitively, each row adding its `depth`, source `strength` and `contribution`.
- A write batch and `update-format` commit only when given `--commit`. Where the `.git` beside `kb-root/`
  is not the one git reads as the worktree root, every command that runs git over the KB is refused
  (`check: worktree`).

**Scoring.** `kbase score` asks the configured scoring model — `models.scoring` — one item at a
time, and asks it for the
**faults the text commits**, not for a number: a list of charge lines, each a fault id and the span
of the item's own text that shows it, or `none`. kbase prices the list with the
pass's table. When the walk ends — done, cancelled or failed — the answered values land in one
write through the edit ops, with the manually-scored mark false. The `rigor` pass covers every
claim, analysis and experiment; the `bearing` pass every `supports`, `lifts` and `refutes` edge.
A full run asks only what is dirty: an item whose text, or whose ask template, changed since its
value was judged, and any value never judged. A second run over a KB it scored asks nothing. A value
a person set is manually scored, and a full run skips it; `--rescore-manual` re-judges those too,
replacing the person's value. `--pass rigor|bearing|all` (default `all`) scopes a run to one set,
and `--node <id>` or `--edge <source>,<target>,<relation>` scores one item, dirty or not; neither
takes `--pass`, and there is no item limit — the scope is the budget. Two values are never asked,
both because the document they rest on lies outside the corpus: a work's rigor, and a `founds`
edge's bearing. Judge each by hand, with the document in hand, through `edit-work` and `edit-edge`;
it stays pending until you give one. The two ask templates,
the ids and the weights are stated once, in
[RIGOR_AND_BEARING_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/rigor-and-bearing/entry-point.md); the ledger is where a
value can be checked against its charges.

Every attempt is recorded under `<state-dir>/score/<run>/`: `run.yaml` names the run, the set it walks
(one pass, or `all`), what answered (`backend`: `provider`), the model and the digest
of the ask its judgements answer; `ledger.jsonl` holds a line per attempt with its own item's pass, the
faults it recorded, the reply where it parsed, the
value it prices to, the model and the prompt's digest. `--resume` continues the repository's most
recent score run with the scope its `run.yaml` records, without re-asking what it already landed, and
takes no scope argument beside it; it is refused where that run finished or a build has started since,
and a fresh `score` then asks only what is dirty. A value lands only against the text it was asked
about: one whose item was edited after its call was composed lands nothing and a resume asks it again,
and one whose node or edge has left the KB is `skipped`. Because a
value is a projection of the line's ids, a resume **re-prices** a run when the table has moved since:
it lands the values that moved through the same write path and records them in `repriced.yaml`, with
no model asked. A resume under a changed ask — a run older than kbase's current instructions —
continues under the current ask: the header keeps the digest the run began under, and nothing refuses
and nothing asks you to choose. The result carries the `resume` call whenever an item failed or was
not reached. A failed run lists what failed under `failures` — an item and the reason its ledger
line recorded, or a refresh's own error — and a cancelled one names the item it stopped before. The
reason names what ended the item's call: `provider-refused` where the endpoint answered with a status
that is not success, its status and its own words beside it, and `transport-failure` where the
connection, the name, the certificate or the stream failed and no answer was had.
`--dry-run` lists under `ask` what a run would ask and under `skip` the dirty manually scored values
it would skip; it asks nothing, resolves no role and writes no KB. A run refreshes the derived files
once at its start, holding the KB write lock; the derivation follows the landing, so the derived
files are current whenever the run exits. A KB in an older format is refused, dry run or not
(`check: index`, remedy `kbase update-format`).

`--scoring-model` names the model for one run: a `<provider>/<model>` reference, which supplies the
role where `models.scoring` names none and overrides it where it does. Every ask is one chat call
on that endpoint, re-issued identically up to the transport attempt count while it does not complete,
pausing 5 s then 30 s between attempts — as long as the item's deadline covers the pause: a pause it
cannot cover is not waited out, and the item fails at once naming the attempts it made. A status that
is not retryable — any non-2xx other than 429
or a 5xx — fails the item at once. Every ask of a run carries the run's id as its
`prompt_cache_key`, so a run's asks reach one provider and share one cache entry.
A run asks a model for minutes: at a terminal `kbase score` blocks until the walk ends, and as an
[MCP](#using-kbase-from-an-agent-harness-mcp) tool it starts a detached child and returns with
the run's identity, `kbase wait` reporting the outcome. `--ask-concurrency` and `--timeout` (one
item's call, default 30 s) bound the asks.

**Locks.** A write batch and `update-format` hold the KB write lock for the whole run.
A writer that cannot take it within 30 s gets `retry`. While a build or a score run holds the
operation lock, these are refused at once (`check: lock`). A build waits out a write in progress for
up to 30 s. `kbase score` takes the write lock around its start refresh and its one landing, never
across the run. A resume restores every path under `kb-root/` to the last boundary, so edits made
while a build is interrupted are lost unless carried in git. `verify`, `export-latex` and the
queries take no lock. Locks are advisory and Unix only.

**Demoted edges.** When the build breaks a dependency cycle, it cuts one edge of the cycle and
records it as a `demoted` edge that keeps the edge it replaced whole, relation and bearing
included. `verify` lists each under `findings` with outcome `done`, `stats` counts them as
`demoted_edges`, and the sheets draw them as cuts. `restore-demoted` puts back the edge a demoted
edge replaced and is refused (`check: dependency-cycle`) where that would close a cycle;
`remove-edge` with `relation: demoted` deletes the demoted record instead, leaving the cut. Each
names it as its `edges` row does. `add-edge` is refused the same way for a scoring edge that closes
a cycle.

**Format migration.** A KB's format is `4.0.0`, stamped `kb-format` in the one-key file
`.index/format.yaml`, and its documents carry no frontmatter at all: its nodes and edges are kbase's
records in `.index/nodes.yaml` and `.index/edges.yaml`, and its `kb-summary.md` summary pages are
generated from them. A statement marker is `<!-- kbase-id: <id> -->`. A KB in an older form —
`3.0.0`, `2.0.x` or `1.0.0` — is converted in memory when it is loaded and answers every read as
converted; the result document says so in a `migration` record, whose `warnings` list what the
conversion dropped or changed, and stderr says the same. While the conversion is unsaved, every
write but `update-format` — a write batch, `score`, and `build` resuming over the KB — is refused
(`check: index`, remedy `kbase update-format`). `update-format` saves it, and refuses a `kb-root/`
with uncommitted changes (`check: dirty-paths`, naming each path); `--commit` commits the
conversion as one commit. Whether or not the format changed, every `update-format` re-derives the
summary pages and sheets and rewrites the block kbase maintains in `AGENTS.md`, `CONVENTIONS.md` and
`README.md`, between its `vvv-kbase` and `^^^-kbase` marker lines. A document without that pair is
left as it stands and listed under `unfenced`. A KB stamped with a newer major or minor is refused
(`check: kb-format`; update kbase), and so is a conversion that cannot carry a file's content,
naming the file and line (`check: conversion`). A missing record file is refused (`check: index`;
restore it from git), and any other load fault is refused (`check: metadata`), naming each fault
with its file and record; `update-format` repairs none, and recovery is in git. A run that cannot take the KB write
lock in 30 s answers `retry`, and one started while a build or a score run holds the operation lock
is refused (`check: lock`). `status` and `cancel` do not load the KB.

### Claim-graph sheets

Every write batch and `update-format` draw the claim graph as SVG: `kb-root/claim-graph.svg`, the whole
KB with every volume as a cluster; and, where two or more volumes hold nodes,
`kb-root/claim-graph-digest.svg`, one box per volume with the edges between volumes counted, and
`kb-root/<volume>/claim-graph.svg` for each volume (its claims plus every claim one edge away).
Every arrow runs from the supporter to the node it supports: a claim or an experiment `supports` a
claim or an analysis; a work or an axiom `founds` one; an analysis or an experiment `lifts` one; and
a claim, an analysis or an experiment `refutes` one. Each relation has one stroke of its own, and
the cut a cycle-breaking demotion made is drawn dotted. Nodes are styled by kind and filled by
strength band, a pending one grey, and every node links to its place: a claim's or a work's entry on
its summary page, an
analysis's or an experiment's section in the leaf that hosts it, an axiom's heading in `axioms.md`.

The sheets need Graphviz's `dot` on `PATH`, any version. Without it, existing sheets are left as
they are, the write writes a placeholder `kb-root/claim-graph.svg` only where none exists and
reports `sheet: not-drawn`. Sheets are rewritten only when their content
changes. A digest or volume sheet the KB no longer calls for is removed by the next render and
reported under `removed`. The drawing is specified in [SPEC §3](PROJECT_DOCS/SPEC/3-compatibility-contract/index.md).

### Using kbase from an agent harness (MCP)

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
  the build is over and prints its `status` document; follow it with `status` and stop it with
  `cancel`. Run `wait` in the background and your harness wakes the session when it exits, with the
  outcome in hand. A build still starting at the bound is reported `done` with `state: starting`,
  `state-dir`, `pid` and `wait`; it keeps running, and `status` follows it. The build's output is
  captured under the state store's `reports/`.
- **`score` is detached.** It starts a scoring run as its own process and returns once the run's
  ledger and pid stand or it has exited, waiting at most 5 s. On a started run it returns `run`,
  `ledger`, `pid` and `wait`, the exact background command that returns when the run is over and
  prints the run's document — the counts by ledger status, the items whose values landed and the
  ones that failed with their reasons. Run `wait` in the background and your harness wakes the
  session when it exits. Follow the run with `status`, stop it with `cancel`, and continue it by
  calling `score` again with `resume` true and no scope argument. A run still starting at the
  bound is reported as a build is, and keeps running. The run's output is captured under the state
  store's `reports/`. A score run's `edge` argument is a mapping of `source`, `target` and
  `relation`.
- **Resources.** `kbase://help/<command>` is each command's help — its body, its argument list and
  its MCP note — one per tool and per op of `write` and `query`, with `kbase://help/write` and
  `kbase://help/query` for what each family shares. The claim-graph sheets are resources,
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
repository layer, and configuration still works: `show-config`, `models`, `configure-providers` and
`configure-models` run in any directory. The verbs that read configuration load all three files strictly (SPEC §9 names them): an
unknown key fails that file's load and that run, naming the key and the file, and no lower layer is
read in its place.

- **`providers.toml`** declares the providers, merged by name: a repository's table for a provider
  replaces the global table of the same name whole, so a repository never inherits the global
  table's key file for an endpoint it chose. Each top-level table is a provider you name — one or more ASCII letters,
  digits, `_` or `-`, not beginning with `-` — with `baseUrl` (required). A name is case-insensitive
  and folded to lowercase everywhere it is read, `--provider`, a model reference's provider part and
  `auth` included, and every output names a provider so: a repository's table overrides the global
  one of the same name in any case, while two tables of one file differing only by case are
  refused.
  Give it its key with `kbase auth add <name>` (below): a stored key belongs to the name, whatever
  layer states the provider. A provider's table may instead name a key file with `apiKeyFile` — for CI and
  mounted secrets — and a relative path resolves beside the file; a stored key outranks it, and
  `kbase show-config` prints the command that moves such a key into the store. A leading
  `~` in `apiKeyFile`, `--config-dir` or `$KBASE_CONFIG_DIR` expands from your
  home directory; `~user` does not. kbase warns if a key file it reads is group- or world-readable;
  `chmod 600` it. A provider's `api` names the chat protocol its server speaks — `anthropic` (the
  default, the Anthropic Messages protocol) or `openai` (the OpenAI-compatible `chat/completions`
  surface), which a provider selects with `api = "openai"`. Use Messages for any role whose thinking
  is on: it caps the thinking a reply may spend, where the OpenAI-compatible protocol only hints at
  an effort the server may exceed without bound. The `decision` role always asks the OpenAI-shaped
  decision surface on its provider's base URL, whatever `api` says, so an `anthropic` provider serves it
  only where its server also speaks that surface. A provider's `decisionsFlavor` names which decision surface its server speaks —
  `systemone` (the default) or `decisions` — and changes nothing about chat, which `api` decides, and
  nothing about the model, which is still `config.toml`'s `models.decision`.
- **`config.toml`** holds the choices: `[models]` `decision`, `text` and `scoring`, the roles, each a
  `<provider>/<model>` reference — the provider that supplies the endpoint, then the model id it serves,
  split at the first `/` (a model id may contain one, a provider name may not). A bare model id there
  is refused, naming the key and the form. `[asks] concurrency` (default 4) is how many model calls a
  build or score run has in flight at once. A created file also
  states, beside `[models]`, what each role wants and example ids to choose from — the same guidance
  an MCP client reads at `kbase://model-hints`. The layers merge `config.toml` field by field: a
  value the repository states wins, and one it leaves out keeps the global value. A `config.toml`
  stating a top-level `provider` key or `[asks] readerConcurrency` fails to load, naming the key;
  delete the line by hand — the configure commands cannot load the file to repair it.
- **`kbase configure-providers <provider> --base-url <url>`** writes a provider into
  `providers.toml`, creating the file where none stands, with `--api` and `--decisions-flavor`
  setting its chat protocol and decision surface; `--base-url` must be an absolute http or https URL
  carrying no credentials. `--remove` removes the provider's table instead, and a provider with no
  table is refused. Where the provider has a key it dials the provider and gives its catalogue as `models`;
  otherwise `next` lists the steps that follow (`kbase auth add`, then `kbase models --provider
  <name>`). **`kbase configure-models --decision|--text|--scoring <provider>/<model>`** assigns
  the roles given, at least one, and leaves the rest as they are. Each provider it names must be
  defined by the `providers.toml` of the layer it writes into or, under `--local`, the global one.
  Each is dialled once; a model missing from its catalogue is a warning, an unlistable catalogue fails the command,
  and a refusal writes nothing. Both, like `models`, list each configuration fault of the layers
  they read under the result's `warnings`, and `configure-models` lists each catalogue miss there
  too; each also goes to stderr, so an MCP caller sees them as a CLI user does. Both
  preserve every other byte of an existing file. They write
  `~/.config/kbase`, or the directory `--config-dir` or `$KBASE_CONFIG_DIR` names; `--local` writes
  `<repository>/.kbase/` (refused outside a worktree) and `--global` writes `~/.config/kbase` even
  where the variable is set. `--global` with `--local`, or either with `--config-dir`, is refused.

**The files and the store are the whole configuration.** The endpoint and the model each role uses
come from `providers.toml` and `config.toml` alone, and its key from the store, else the provider's key
file; no environment variable supplies any of them. To keep a run apart from your own layers, name
its directory with `--config-dir` or `$KBASE_CONFIG_DIR`; that leaves your store in use, which
`$XDG_STATE_HOME` moves.

Provider selection — `--provider`, else the sole provider in `providers.toml` — belongs to the catalogue
verbs, `models` and `show-config`, alone: they select a provider to dial or report, which is not a
role. A provider name that no layer declares, there or in a role's reference, is refused against the
`providers.toml` the configure commands write into — the file a provider belongs in — with
`kbase configure-providers <name> --base-url <url>` as the remedy, unless that provider's table failed
to load, where the fault's own reason stands instead. In a build, the claim graph's multichoice asks go to the `decision` role and the overview passage to
`text`; in a scoring run, the asks go to `scoring`.

`kbase show-config` reports how all of this resolves, and changes nothing: no KB read, no file
written, no request made. It states the layers read and what named each; every provider, with its
layer, key kind (`store`, `file` or `none`, never the key), decision surface and any fault; `config.toml`'s merged `[models]` values; the
selected provider and rule; and one record per role, naming the endpoint, the layer that supplied
each field, the model asked, or what is missing. Gaps are diagnostics in a `done` result, not
refusals, so one call shows them all.

`kbase probe [--model <provider>/<model>] [--decision | --text | --scoring] [--prompt TEXT]` asks
one model one question and reports what came back: the selection asked, the provider, the model, the
time to the first token, the call's wall time, the finish reason, the reply, and the usage the
provider reported. A probe asks one selection: `--decision`, `--text` and `--scoring` each ask the
selection the configuration resolves for that role — `models.<role>` in the file — and `--model`
names a reference to reach instead, resolved against the same providers. The four are exclusive; with
none of them the probe asks the `scoring` role — the selection a run would make — so
`kbase probe` with no arguments is the heartbeat of a configuration. The document names the selection
that answered. It reads no KB and reads or writes no state directory: one ask, one result document.
Over MCP it is a read-only tool, and each call is a paid model call.

`kbase auth add <provider>` is how you give a provider its key. It stores a provider's API key, read from stdin and nowhere else, one
encrypted file per provider under `$XDG_STATE_HOME/kbase/auth/`
(`~/.local/state/kbase/auth/` where the variable is unset or not an absolute path). Pipe the key in from your key manager or clipboard —
`<command-that-prints-the-key> | kbase auth add <provider>` — so it never stands on a command line
or in a terminal; a terminal on stdin, empty input, input over 64 KiB, a key with whitespace or a
control character inside it, or a name no `providers.toml` provider carries is refused. The encryption keeps the key out of
anything that reads the file; it is no protection against your own OS account.
`kbase auth remove <provider>` clears a stored key and is refused, with `kbase auth list` as the
remedy, where none is stored; `kbase auth list` names the providers that have one. These verbs are
CLI-only: no MCP tool takes a key.

Global flags on every subcommand: `--config-dir`, `--log-level debug|info|warn|error` (default
`warn`), `--log-file`, `--project-dir` (the git worktree root that holds `kb-root/`; the search for
the repository starts there instead of the working directory). A relative path a subcommand takes
resolves against the project root; `--config-dir` and `--log-file` resolve against the working
directory. The root command also takes `-v`/`--version`. `kbase mcp` finds its project from the
working directory, as every subcommand does, unless run not located (`--project-dir-required`); it
refuses `--project-dir`.

## Development

```sh
just             # list recipes
just edit-gate   # cheap gate: run after every change
just checkpoint  # full suite: run at checkpoints
just test        # unit tests (VERBOSE=1 for per-test output)
just tag         # signed tag v<version> of a vetted tree; local only
just tap-update  # after a pushed release: the tap formula's url and sha256, committed in the tap; go.mod's go line raised if outside Go's supported window, uncommitted
just tap-local   # local homebrew install of release candidate
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

- [THESIS](PROJECT_DOCS/entry-point.md): why the build has its shape.
- [SPEC](PROJECT_DOCS/SPEC/entry-point.md): what any compliant implementation must do.
- [ARCHITECTURE](PROJECT_DOCS/ARCHITECTURE/entry-point.md): how this implementation meets the spec.
- [CONVENTIONS](PROJECT_DOCS/CONVENTIONS/entry-point.md): house rules and practices.

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
