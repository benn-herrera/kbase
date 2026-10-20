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
- **A provider configuration**, for builds that spend inference. A build run with `--no-inference`
  needs none.
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
kbase configure --model-map decision=<model-id>,text=<model-id>
```

`configure --create` writes into `~/.config/kbase`, or into `<repository>/.kbase/` where that
directory already stands; add `--project` to name the repository's `.kbase/` whether or not it
exists. Without `--provider` it makes no request, so the ids it writes are unchecked — `kbase
models` and the last command above check them. Details are under [Configuration](#configuration).

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

The run lock is not in the store: it is `kbase-build.lock` in the repository's git directory, so one
build runs per repository whatever `--state-dir` each names.

The build records are tracked at the repository root, as YAML: `kb-build-node-pass.yaml`,
`kb-build-unmarked.yaml`, `kb-build-classification.yaml` and `kb-build-cull.yaml`.

## Maintaining a KB

A KB created empty by `kbase init` and one distilled by `kbase build` are maintained the same
way — the subcommands below operate on any living `kb-root/`. Over a created KB `kbase build`
stays refused forever (SPEC §5's fresh-vs-resume reads the `kb-build:` trail, and `init`'s birth
commit is a plain subject); content joins through the write ops, a claim at a time.

After a build, subcommands operate on the living KB: write ops that insert entries and set fields,
`render-citation`, queries over the claim graph, `refresh` and `verify`, `render-claim-graph`, and
`export-latex`, which writes a LaTeX project from the KB through pandoc.
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

**Locks.** A write op, `refresh` and `render-claim-graph` hold the KB write lock for the whole run.
A writer that cannot take it within 30 s gets `retry`. While a build runs, these are refused at once
(`check: lock`). A build waits out a write in progress for up to 30 s. A resume restores every path
under `kb-root/` to the last boundary, so edits made while a build is interrupted are lost unless
carried in git. `render-citation`, `verify` and the queries take no lock. Locks are advisory and
Unix only.

**Demoted edges.** When the build breaks a dependency cycle, the `supports` edge it cuts is recorded
as a `demoted` relation carrying its origin, `cited` or `inferred`, instead of becoming a plain
reference. `verify` lists each under `findings` with outcome `done`, `stats` counts them as
`demoted_edges`, and the sheets draw them as cuts. `resolve-demoted` takes `entry` items of `id`,
`target` and `action`: `remove` deletes the edge, `restore` rewrites it as a `supports` edge and is
refused (`check: dependency-cycle`) where that would close a cycle.

**Format migration.** A KB's metadata format is `2.0.0`, stamped `kb-format` in the entry point's
frontmatter. A KB in either older form, `1.0.0` or `0.9.0`, is converted in memory when it is
loaded, and the next write stamps `2.0.0`; the result document says so in a `migration` record, and
one line on stderr says what the conversion moved. Derived values are not carried across unchanged:
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
  `export-latex`, `status`, `cancel` and `build`. `completion`, `help` and `mcp` are not offered,
  and neither is `wait`: a held call would spend the turn the build takes, so the `build` tool hands
  you its command to run in the background instead. `models` reads a
  provider's model catalogue and `configure` writes `config.toml`; neither performs inference —
  `build` is the one tool that does. A write-op tool takes one entry and offers no `--no-refresh`.
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
directory. Both files are loaded strictly: an unknown key fails that file's load and the run, naming
the key and the file, and no lower layer is read in its place.

- **`providers.toml`** is the endpoint pool, merged by entry name: a repository entry of the same
  name replaces the global entry whole, so a repository never inherits a credential for an endpoint
  it chose. Each top-level table is a provider you name, with `baseUrl` (required) and `apiKeyFile`
  — an entry supplies its key no other way, and a relative path resolves beside the file. A leading
  `~` in `apiKeyFile`, `--config-dir`, `$KBASE_CONFIG_DIR` or `$KBASE_API_KEY_FILE` expands from your
  home directory; `~user` does not. kbase warns if a key file is group- or world-readable;
  `chmod 600` it. An entry's `decisionsFlavor` names which decision surface its server speaks —
  `systemone` (the default) or `decisions` — and changes nothing else: chat stays
  `chat/completions`, and the decision model is still `config.toml`'s `models.decision`.
- **`config.toml`** holds the choices: `provider`, and `[models]` `decision` and `text`. A created
  one also states, beside `[models]`, what each category wants and example ids to choose from — the
  same guidance an MCP client reads at `kbase://model-hints`. The layers merge these field by
  field: a value the repository states wins, and one it leaves out keeps the global value.
  `kbase configure` writes it and preserves every other byte of an existing file.
- **`kbase configure --create`** writes an annotated `providers.toml` where none stands — nothing
  else creates that file — and overwrites no other content. `config.toml` is created where absent on
  any run and edited in place otherwise. It writes the layer a flag or `$KBASE_CONFIG_DIR` names,
  else `<repository>/.kbase/` where that directory already stands, else `~/.config/kbase`;
  `--project` names the repository's `.kbase/` whether or not it exists, and is refused outside a
  worktree. Without `--provider` it makes no request and says the model ids went unchecked; with one
  it dials, warns on an id the catalogue lacks, and a refusal — or a failed dial or listing —
  writes nothing.

The environment variables `KBASE_API_BASE_URL`, `KBASE_DECISION`, `KBASE_TEXT` and
`KBASE_API_KEY_FILE` each override the one corresponding field. The environment overrides the files
and nothing overrides the environment; no flag sets one of those fields, and the environment alone
suffices with no config directory.

Provider selection is `--provider`, else `config.toml`'s `provider`, else the sole `providers.toml`
entry. A `--provider` the pool does not hold refuses against the `providers.toml` of the layer
`configure` writes into — the file an entry belongs in — and offers `kbase configure --create`
unless that provider faulted out of the pool, where the fault's own reason stands instead. A pool
with no usable entry refuses against that same file. In a build, the claim graph's letter asks go to
the decision model and the overview passage to the text one.

`kbase show-config` reports how all of this resolves — the layers read and what named each, the pool
with each entry's layer, key kind, decision surface and any fault, `config.toml`'s merged choices, the
inference variables this process sets, the selected entry and rule, and the endpoint inference
resolves to (naming the layer that supplied each field) or what is missing — and changes nothing: no
KB read, no file written, no request made. Gaps are diagnostics in a `done` result, not refusals, so
one call shows them all.

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

Integration recipes run `./bin/kbase` itself and keep their evidence under `test_data/transient/`.
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
| [go.yaml.in/yaml/v3](https://go.yaml.in/yaml/v3) | YAML organization | MIT and Apache-2.0 | YAML values input, tool results and build records |
| [golang.org/x/text](https://pkg.go.dev/golang.org/x/text) | The Go Authors | BSD-3-Clause | Unicode NFC normalization in ingest; case folding in queries |

pandoc (John MacFarlane and contributors) and Graphviz `dot` (Graphviz authors) are run as external
programs and are not bundled.
