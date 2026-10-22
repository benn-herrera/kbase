# README – KBase

kbase is a Go binary that builds a knowledge base (KB) from LaTeX and provides tools to maintain it
afterward. Given a set of `.tex` entry point files, it writes a navigable Markdown tree plus
metadata describing a graph structure of the claims forming the argument within. The build process
starts with a mechanical pass and is refined by an inference pass that is tractable on local 
hardware.

![Volume Graph](readme-assets/kb-volume-graph.png)
![Claim Graph](readme-assets/kb-claim-graph.png)

## Requirements and Supported Platforms

### Requirements
- **pandoc 3.12.** Required on the host, not shipped. Any pandoc whose `pandoc-api-version` is
  1.23.x is accepted; a missing or out-of-range pandoc is refused at preflight.
- **git 2.56.0.** Required on the host. A build records its progress as commits in your repository.
- **Graphviz `dot`**, optional, any version, on `PATH`. Draws the [claim-graph
  sheets](#claim-graph-sheets); without it they are not drawn.
- **A provider configuration**, for builds that spend inference and for scoring runs. A build run with
  `--no-inference` needs none, and so does a `kbase score --dry-run`. A `kbase score --harness` run
  dials no endpoint, but `models.scoring` — or `KBASE_SCORING` — must still name a model: the role is
  what declares which model judges the KB.
- **opencode v2**, for `--harness opencode` alone: the installed binary, authenticated for the model
  the scoring role names. kbase passes that model to opencode as `--model` and its value, unaltered,
  and reads opencode's configuration from nowhere but an empty directory it creates for the run.
- **Go**, to build kbase from source. `just` runs the project's recipes.

Input is LaTeX only. A volume root is the paper's top `.tex` file: the one `00README.json`'s
`toplevel` entry names, else the sole file containing `\documentclass`.

### Platforms
- macOS (run and dev)
- Linux (binary can be built from macOS, dev experience untested, your mileage may vary)
- Windows (binary can be built from macOS, dev experience untested, your mileage will absolutely vary)

## Installation

Each release is published on the GitHub releases page as four artifacts: the archive
`kbase-v<version>.tgz`, its detached signature, the installer `install-kbase-v<version>.sh`, and the
installer's own detached signature. The archive holds one binary per platform, each again with its
own signature, beside this README, `CONVENTIONS.md` and `LICENSE`.

### With the installer

Download the installer and its signature into an empty directory and run it there:

```sh
base=https://github.com/benn-herrera/kbase/releases/download/v<version>
curl -LO "$base/install-kbase-v<version>.sh" -LO "$base/install-kbase-v<version>.sh.asc"
bash install-kbase-v<version>.sh
```

The installer reads the platform from `uname -s` and `uname -m` and refuses any but macOS arm64,
Linux amd64, and Windows amd64 under Git Bash or MSYS, naming both values it read. It needs `gpg` on
`PATH`. It then:

1. Locates the signing key (`benn@bennherrera.dev`) through WKD, downloads the installer's own
   `.asc` where it is missing, and reads that signature for the key that made it.
2. Looks that key up in your keyring. It refuses where the signature names no key, where the key is
   not in the keyring, and where gpg lists no fingerprint or user ID for it, naming the key and where
   it is published.
3. Where the key is in the keyring but not ultimately trusted, prints the user ID, the fingerprint,
   the key URL and the policy URL, and asks:

   ```
   Choice [y/o/N]:
   ```

   `y` records ultimate ownertrust for the fingerprint, so later installs with this key ask nothing.
   `o` records nothing and verifies this run, leaving the keyring untouched and gpg flagging each
   signature as untrusted. Any other answer, an empty one included, installs nothing. With no
   terminal on stdin — the way `curl … | bash` runs it — the installer refuses instead of asking and
   prints the two commands that establish the trust by hand.
4. Verifies its own signature against the script it is running, downloads the archive and its
   signature into the working directory if they are not already there, and verifies them.
5. Extracts `kbase-v<version>/` where it does not already stand, verifies each of the three binaries
   against its `.asc`, and copies the platform's binary to `kbase` (`kbase.exe` on Windows) inside
   that directory, clearing macOS's quarantine attribute from it.

A failed `gpg --verify` stops the script where it stands: it never reaches the extraction after a
failed archive signature, and never reaches the copy after a failed binary signature. A signature
made by a key outside your keyring fails the same way, at whichever file it signs. Nothing is
installed outside the directory you ran the installer in.

### Verifying by hand

Every signature is detached: gpg verifies it, and the installer is only doing that for you. To check
the archive and unpack it yourself, in place of steps 4 and 5:

```sh
gpg --verify kbase-v<version>.tgz.asc kbase-v<version>.tgz
tar xzf kbase-v<version>.tgz
gpg --verify kbase-v<version>/kbase-darwin-arm64.asc kbase-v<version>/kbase-darwin-arm64
```

The binaries in the archive are `kbase-darwin-arm64`, `kbase-linux-amd64` and
`kbase-windows-amd64.exe`, each with an `.asc` beside it. Copy the one for your platform to `kbase`
(or `kbase.exe`), and put it on `PATH`.

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

`init` writes the skeleton — a stamped `entry-point.md` headed by the repository's name (or
`--title`'s text), `.index/`, `AGENTS.md`, `CONVENTIONS.md` and the `CLAUDE.md` redirect —
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
`equations-minted`, `references-found`, `claims-culled`, `depends-attributed`, `phase-3a`,
`overview-drafted`. What each does is in [ARCHITECTURE.md](ARCHITECTURE.md) §4.

`claims-culled` asks, for each pair of linked claims with a prose end, whether one adds nothing to
the other; a prose claim that adds nothing is removed, and its links pass to the claim it was culled
into. It adds no edge: `depends-attributed` asks every surviving link its direction. Under
`--no-inference` it is walked without asks. It records its verdicts and the surviving links in
`kb-build-cull.yaml`.

### Resuming and monitoring

Each stage boundary is a commit in your repository, with subject `kb-build: <stage-id> | <display
name>`. Run `kbase build` again with the same arguments and it resumes from the last boundary on the
commit trail. After an interrupted stage, it restores the paths the build owns to the last stage
commit and re-runs that stage.

To go back further, reset the repository to an earlier boundary and re-issue the build from there.
`kbase status` names each stage's commit; over [MCP](#using-kbase-from-an-agent-harness-mcp) the
`rewind-build` tool and the `kbase://build-trail` resource report the whole trail and, for a
boundary you name, print both commands — the `git reset --hard` and the recorded re-issue — ready to
run. The reset is yours: the tool reports it and changes nothing. It discards every boundary newer
than the one you reset to, which the document counts as `discards`. A dirty worktree does not stop
it; `dirty` lists what the reset would lose.

A resume must be given the inputs the trail's newest boundary records: the commit body ends with a
`volume-root:` line per volume root, then a `bibliography:` line per `--bibliography` file, in the
order given. A boundary written before inputs were recorded is not compared. Otherwise the resume is
refused (`check: inputs`); resume with the recorded inputs, or start over from a commit before the
trail.

`build` runs in the foreground (through the [MCP server](#using-kbase-from-an-agent-harness-mcp) it
is detached). From the repository, in a second shell:

```sh
kbase status   # state, pid, stage commits, progress, the resume command
kbase cancel   # stop the build; costs only the unit in flight
```

State that does not belong in the repository lives in `$XDG_STATE_HOME/kbase/<key>/`, or in
`--state-dir`: the pid and holder lock that `status` and `cancel` read, `progress.jsonl`, per-call
evidence, the answer cache, and the captured output of a build the MCP server started. It holds
paid-for inference and survives `git clean`.

The store is versioned: `<state-dir>/format` is `1`, and `build`, `status`, `wait`, `cancel`, the
MCP `build` tool and the MCP `rewind-build` tool refuse a newer store (`check: state-dir`). Each build
prunes `reports/build-*` and `scratch/captures/` older than the fifth-latest ended build, and keeps
the answer cache. `status` reports `cache-entries` and `cache-bytes`. Deleting the store is safe but
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
`render-citation`, queries over the claim graph, `refresh` and `verify`, `render-claim-graph`, and
`export-latex`, which writes a LaTeX project from the KB through pandoc; `score` asks the configured
scoring model for the faults each item's text commits, one item at a time, and prices them into the
KB's rigor and bearing values.
The queries are `find`, `show`, `deps`, `gated-on`, `cited-by`, `referenced-by`, `strength-below`,
`subtree`, `weak-points` and `stats`. Run `kbase --help` for the list and `kbase <command> --help`
for each one.

- Write ops take values as YAML (JSON accepted) on stdin or `--values`. Re-issuing an op leaves
  `kb-root/` byte-identical, and the second call reports `unchanged`. An insert adopts an existing
  entry of the same register and title instead of writing a second one.
- **Scores.** A node's own value is `rigor`, a scoring edge's is `bearing`, and a claim's or an
  analysis's derived value is `strength`. Two ops record a judgement: `set-rigor` writes a node's
  `rigor` and `set-bearing` a scoring edge's `bearing`; `set-bearing` replaces an existing token and
  never adds one, because every scoring edge already carries one. A claim or analysis with no
  `rigor` reads `*pending*` — unscored, not low. Its derivation pends with it unless a lift stands
  over it: a node corroborated and never judged publishes the lift's product, while a refutation
  alone leaves its strength unknown. A `bearing` of `*pending*` leaves its edge out of the
  derivation until it is judged.
- Three write ops are kbase's own. `drop-claim-edges` removes a claim's `rests-on` or `references`
  list. `drop-inbound-edges` removes every edge that names it. `drop-claim-node` deletes the node
  itself, and refuses while an edge or any other line still names it, so run the two edge ops first.
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

**Scoring.** `kbase score --pass rigor|bearing` asks the configured scoring model — `models.scoring`,
or `KBASE_SCORING` under the environment's endpoint — one item at a time, and asks it for the
**faults the text commits**, not for a number: a list of charge lines, each a fault id and the span
of the item's own text that shows it, or `none`, or `flt-pending`. kbase prices the list with the
pass's table and writes the result through `set-rigor` or `set-bearing`. The `rigor` pass covers
every claim, analysis and experiment; the `bearing` pass every `supports`, `founds`, `lifts` and
`refutes` edge. A work's rigor is not asked: it is the one value you judge by hand, with the document
in hand, through `set-rigor`. The two ask templates, the ids and the weights are stated once, in
[RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md); the ledger is where a
value can be checked against its charges.

Every attempt is recorded under `<state-dir>/score/<run>/`: `run.yaml` names the run, the back end,
the model and the digest of the ask its judgements answer; `ledger.jsonl` holds a line per attempt
with the faults it recorded, the reply where it parsed, the value it prices to, the model and the
prompt's digest; and under `--harness` one `harness/` working directory. `--resume <run>` continues a
run without re-asking what it already landed, while a fresh run asks everything again. Because a
value is a projection of the line's ids, a resume **re-prices** a run when the table has moved since:
it lands the values that moved through the same write path and records them in `repriced.yaml`, with
no model asked. A resume under a changed ask — a run older than kbase's current instructions —
continues under the current ask: the header keeps the digest the run began under, and nothing refuses
and nothing asks you to choose. `--limit N` stops after N items, and the result carries the `resume`
command — flags, back end and, under the environment mode, the variables it read — whenever an item is
still not `done`. A failed run lists what failed under `failures` — an item and the reason its ledger
line recorded, or a refresh's own error — and a cancelled one names the item it stopped before. The
reason names what ended the item's call: `provider-refused` where the endpoint answered with a status
that is not success, its status and its own words beside it, and `transport-failure` where the
connection, the name, the certificate or the stream failed and no answer was had. A
`bounded` run carries no `failures`: its failures are counted in `counts.failed` and reasoned in the
ledger.
`--dry-run` lists the set, asks nothing and writes no charge record. A run refreshes once at its
start — on an older KB that is the write that converts and saves it — and once after its last value
or the values a resume re-priced, each refresh holding the KB write lock; a dry run does neither.

`--scoring-model` names the model for one run: a `<provider>/<model>` reference in file mode, which
supplies the role where `models.scoring` names none and overrides it where it does; a bare id under
the environment's endpoint, where the role's variable must name one too; the role's value whole under
`--harness`, which under `--harness opencode` is handed to opencode as it stands, so a model opencode
does not recognise fails that item, in opencode's own words, and the run goes on.
`--harness opencode` runs one process per item instead of asking an endpoint: an installed opencode
v2, with an agent kbase defines for that process alone — no tools, no instructions of its own, its
configuration sources cut back to an empty directory under the run's own records — the prompt on its
stdin and its stdout the reply. Nothing is written to your opencode configuration. It is the only
value the flag takes: kbase runs no command a configuration file names, and any other name is
refused before the KB is opened.
A run asks a model for minutes, so it is a command line: nothing is offered for it over MCP.

**Locks.** A write op, `refresh` and `render-claim-graph` hold the KB write lock for the whole run.
A writer that cannot take it within 30 s gets `retry`. While a build runs, these are refused at once
(`check: lock`). A build waits out a write in progress for up to 30 s. `kbase score` takes it around
each landing and each of its two refreshes, never across the run. A resume restores every path
under `kb-root/` to the last boundary, so edits made while a build is interrupted are lost unless
carried in git. `render-citation`, `verify` and the queries take no lock. Locks are advisory and
Unix only.

**Demoted edges.** When the build breaks a dependency cycle, the `supports` edge it cuts is recorded
as a `demoted` relation carrying its origin, `cited` or `inferred`, instead of becoming a plain
reference. `verify` lists each under `findings` with outcome `done`, `stats` counts them as
`demoted_edges`, and the sheets draw them as cuts. `resolve-demoted` takes `entry` items of `id`,
`target` and `action`: `remove` deletes the edge, `restore` rewrites it as a `supports` edge and is
refused (`check: dependency-cycle`) where that would close a cycle.

**Format migration.** A KB's metadata format is `2.0.1`, stamped `kb-format` in the entry point's
frontmatter. A KB in either older form, `1.0.0` or `0.9.0`, is converted in memory when it is
loaded, and the next write stamps the current version; the result document says so in a `migration`
record, and one line on stderr says what the conversion moved. A `2.0.0` KB is read as current — its
patch moved only the bottom band's words, `unsubstantiated` — and its derived `- strength:` lines and
index re-render on the next `refresh`, which stamps `2.0.1`. Derived values are not carried across unchanged:
a lifted claim may read higher and a work edge below `1` lower. Read-only commands write nothing,
even over an older KB, but one that reads the derived index is refused until a save has rewritten it
— the queries and `render-claim-graph` answer `check: index` with remedy `kbase refresh`. A write
op's `--no-refresh` on an unmigrated KB is refused the same way (`check: index`, remedy `kbase
refresh`), and `verify` reports it stale with the same remedy. The migrating `refresh`
lists obsolete paths under `removed`. A KB stamped with a newer major or minor is refused
(`check: kb-format`; update kbase). `status` and `cancel` do not load the KB.

### Claim-graph sheets

`refresh` and `render-claim-graph` draw the claim graph as SVG: `kb-root/claim-graph.svg`, the whole
KB with every volume as a cluster; and, where two or more volumes hold nodes,
`kb-root/claim-graph-digest.svg`, one box per volume with the edges between volumes counted, and
`kb-root/<volume>/claim-graph.svg` for each volume (its claims plus every claim one edge away).
Every arrow runs from the supporter to the node it supports: a claim or an experiment `supports` a
claim or an analysis; a work or an axiom `founds` one; and a claim, an analysis or an experiment
`lifts` or `refutes` one. Each relation has a stroke of its own, and the cut a cycle-breaking
demotion made is drawn dotted. Nodes are styled by kind and filled by strength band, a pending one
grey, and every node links to its register entry.

The sheets need Graphviz's `dot` on `PATH`, any version. Without it, existing sheets are left as
they are, `refresh` writes a placeholder `kb-root/claim-graph.svg` only where none exists, and
`render-claim-graph` reports `sheet: placeholder`. Sheets are rewritten only when their content
changes. A digest or volume sheet the KB no longer calls for is removed by the next render and
reported under `removed`. The drawing is specified in [SPEC.md](SPEC.md) §3.

### Using kbase from an agent harness (MCP)

`kbase mcp` serves one KB to any agent harness that speaks the Model Context Protocol, over stdio.
Every tool is one subcommand, with the same arguments and the same result — except `rewind-build`,
which reports the build's commit trail and has no subcommand behind it. Wiring it into a harness is
covered in [INTEGRATING_KBASE_VIA_MCP.md](INTEGRATING_KBASE_VIA_MCP.md).

The server entry is the same for every harness: command `kbase`, arguments:

```json
["mcp", "--kb-root", "<absolute path>"]
```

`--kb-root` is a kb-root directory, or a repository root (holding `.git`) with or without
`kb-root/`; any other path is refused before serving.

Two examples. Claude Code, in `.mcp.json`:

```json
{"mcpServers": {"kbase": {"command": "kbase", "args": ["mcp", "--kb-root", "<abs>"]}}}
```

opencode, in `opencode.json`:

```json
{"mcp": {"kbase": {"type": "local", "command": ["kbase", "mcp", "--kb-root", "<abs>"], "enabled": true}}}
```

These are examples; any MCP-capable harness adds its own equivalent entry.

- **Tools.** The subcommands: `init`, the write ops, `render-citation`, the queries (including `show`
  and `stats`), `show-config`, `models`, `configure`, `refresh`, `verify`, `render-claim-graph`,
  `export-latex`, `status`, `cancel`, `build` and `score`. `completion`, `help` and `mcp` are not
  offered, and neither is `wait`: a held call would spend the turn the build or score run takes, so
  the `build` and `score` tools hand you their command to run in the background instead. `models`
  reads a provider's model catalogue and `configure` writes `config.toml`; neither performs
  inference — `build` and `score` are the tools that do. A write-op tool takes one entry and offers
  no `--no-refresh`.
  `rewind-build` is the exception to one tool per subcommand: it lists the build's boundaries,
  oldest first, and for a `target` you name — a stage id or display name — returns the
  `git reset --hard` and the recorded re-issue command, with `discards` counting the boundaries the
  reset removes. It is read-only; running the reset is yours.
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
  name replaces the global entry whole, so a repository never inherits a credential for an endpoint
  it chose. Each top-level table is a provider you name, with `baseUrl` (required) and `apiKeyFile`
  — an entry supplies its key no other way, and a relative path resolves beside the file. A leading
  `~` in `apiKeyFile`, `--config-dir`, `$KBASE_CONFIG_DIR` or `$KBASE_API_KEY_FILE` expands from your
  home directory; `~user` does not. kbase warns if a key file is group- or world-readable;
  `chmod 600` it. An entry's `decisionsFlavor` names which decision surface its server speaks —
  `systemone` (the default) or `decisions` — and changes nothing else: chat stays
  `chat/completions`, and the decision model is still `config.toml`'s `models.decision`.
- **`config.toml`** holds the choices: `[models]` `decision`, `text` and `scoring`, each a
  `<provider>/<model>` reference — the entry that supplies the endpoint, then the model id it serves,
  split at the first `/` (a model id may contain one, a provider name may not). A bare model id there
  is refused, naming the key and the `kbase configure` command that rewrites it. A created file also
  states, beside `[models]`, what each role wants and example ids to choose from — the same guidance
  an MCP client reads at `kbase://model-hints`. The layers merge `config.toml` field by field: a
  value the repository states wins, and one it leaves out keeps the global value. `kbase configure`
  writes it and preserves every other byte of an existing file. Its top-level `provider` key is
  **superseded**: it still loads and nothing chooses by it, and a file-mode run says so once on
  stderr; `kbase configure` removes it.
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

**Two modes, never merged.** Inference is supplied by the files or by the environment, and by one of
them only. The environment is `KBASE_API_BASE_URL` plus the role variables `KBASE_DECISION`,
`KBASE_TEXT` and `KBASE_SCORING` — bare model ids of that endpoint — and it needs no configuration
directory at all. A role variable beside a `[models]` value is refused, one item per side, and so is a
role variable without a base URL or a base URL beside file-supplied models: the environment is the
whole configuration or nothing. `KBASE_API_KEY_FILE` is not a mode — it replaces the key of whichever
endpoint is in force, and is optional. A harness consults neither mode: it chooses no endpoint.

Provider selection — `--provider`, else the sole `providers.toml` entry — belongs to `models` and
`configure` alone: they select an entry to dial, which is not a role. A `--provider` the pool does not
hold refuses against the `providers.toml` of the layer `configure` writes into — the file an entry
belongs in — and offers `kbase configure --create` unless that provider faulted out of the pool,
where the fault's own reason stands instead. A pool with no usable entry refuses against that same
file. In a build, the claim graph's letter asks go to the `decision` role and the overview passage to
`text`; in a scoring run, the asks go to `scoring`.

`kbase show-config` reports how all of this resolves, and changes nothing: no KB read, no file
written, no request made. It states the layers read and what named each; the pool, with each entry's
layer, key kind, decision surface and any fault; `config.toml`'s merged `[models]` values; the
inference variables this process sets; the mode;
the selected entry and rule; and one record per role, naming the endpoint, the layer that supplied
each field, the model asked, or what is missing. Gaps are diagnostics in a `done` result, not
refusals, so one call shows them all.

Global flags on every subcommand: `--config-dir`, `--log-level debug|info|warn|error` (default
`warn`), `--log-file`. The root command also takes `-v`/`--version`. `kbase mcp` without `--kb-root`
finds the KB from the working directory, as every subcommand does.

## Development

```sh
just             # list recipes
just edit-gate   # cheap gate: run after every change
just checkpoint  # full suite: run at checkpoints
just test        # unit tests (VERBOSE=1 for per-test output)
just dist        # cross-build every target and stage the signed distro; run checkpoint first
```

`just dist` writes four artifacts to `dist/`: the tarball `kbase-v<version>.tgz`, its detached
signature `kbase-v<version>.tgz.asc`, the installer `install-kbase-v<version>.sh` and its signature
`install-kbase-v<version>.sh.asc`. It runs no checkpoint of its own: a cross-build is what users
receive, so the gate is a precondition of the release, not a step the recipe runs. No built binaries
are committed; `bin/` and `dist/` are gitignored. The tarball unpacks to `kbase-v<version>/` holding
this README, [USER_CONVENTIONS.md](USER_CONVENTIONS.md) as `CONVENTIONS.md` — the operating contract
for an agent driving the CLI — the LICENSE, and one binary per platform, each with its own `.asc`.
[Installation](#installation) is what a user does with them.

Integration recipes run `./bin/kbase` itself and keep their evidence under `test-data/transient/`.
Recipes that need pandoc or a live provider are excluded from `just test-integration` and say so in
`just --list`.

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
