# README – KBase

kbase is a Go binary that builds a knowledge base (KB) from LaTeX and maintains it afterward. Give
it a paper's top `.tex` file and it writes a navigable Markdown tree with a claim graph, built by a
mechanical pass and refined by inference.

The KB is the one [kb_tools](#relation-to-kb_tools) builds: either toolchain can maintain a KB the
other built, both at metadata format `1.0.0`.

## Requirements and Supported Platforms

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

## Quick Start Guide

Build the binary (`bin/kbase`, host platform only), then put it on `PATH` or run it as
`./bin/kbase`. The examples below assume `PATH`.

```sh
just build
```

Configure a provider. Skip this step for `--no-inference` builds.

```sh
mkdir -p ~/.config/kbase
cat > ~/.config/kbase/providers.toml <<'EOF'
[local]
baseUrl = "http://localhost:8000/v1"
apiKeyFile = "local.key"
EOF
kbase configure --model-map heavy=<model-id>,light=<model-id>
```

`kbase models` lists the model ids the provider offers. Details are under
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

## Building a KB

```
kbase build <volume-root>... [--bibliography FILE]... [--charter FILE] [--no-inference]
            [--through <stage>] [--state-dir DIR]
```

- **Bibliographies.** By default every `.bib` beside the volume root is used, in sorted order.
  `--bibliography` replaces that set. An unreadable bibliography is reported and the build continues
  without it.
- **`--through <stage>`** stops after the named stage, by id or display name. A bounded run exits 0
  with outcome `bounded`.
- **`--charter FILE`** records a statement of the build's scope as `kb-build-charter.md`.
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

Stages, in order: `start`, `document-graph`, `spine-seed`, `claims-declared`, `claims-discovered`,
`equations-minted`, `references-found`, `depends-attributed`, `phase-3a`, `overview-drafted`. What
each does is in [ARCHITECTURE.md](ARCHITECTURE.md) §4.

### Resuming and monitoring

Each stage boundary is a commit in your repository, with subject `kb-build: <stage-id> | <display
name>`. Run `kbase build` again with the same arguments and it resumes from the last boundary on the
commit trail. After an interrupted stage, it restores the paths the build owns to the last stage
commit and re-runs that stage. To go back further, reset to an earlier stage commit and resume from
there.

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

The store is versioned: `<state-dir>/format` is `1`, and `build`, `status`, `cancel` and the MCP
`build` tool refuse a newer store (`check: state-dir`). Each build prunes `reports/build-*` and
`scratch/captures/` older than the fifth-latest ended build, and keeps the answer cache. `status`
reports `cache-entries` and `cache-bytes`. Deleting the store is safe but forfeits the cache.

The run lock is not in the store: it is `kbase-build.lock` in the repository's git directory, so one
build runs per repository whatever `--state-dir` each names.

## Maintaining a KB

After a build, subcommands operate on the living KB: write ops that insert entries and set fields,
`render-citation`, queries over the claim graph, `refresh` and `verify`, and `render-claim-graph`.
The queries are `find`, `show`, `deps`, `gated-on`, `cited-by`, `referenced-by`, `solidity-below`,
`subtree`, `weak-points` and `stats`. Run `kbase --help` for the list and `kbase <command> --help`
for each one.

- Write ops take values as YAML (JSON accepted) on stdin or `--values`. Re-issuing an op leaves
  `kb-root/` byte-identical, and the second call reports `unchanged`.
- A write op ends with a `refresh` unless given `--no-refresh`. `kbase verify` then checks freshness
  and links; the citation grammar is checked inside the build.
- List queries return `--limit` results (default 50; `0` means all) from `--offset`, and report
  `count` and `truncated`. When `truncated` is true, re-issue with `--offset` at the offset plus the
  results returned.
- `show` on a claim lists its `strengthen_by` items (`item_idx`, `text`, `mentioned_ids`); `deps`
  carries each edge's `context`.
- Maintenance never requires git.

**Locks.** A write op, `refresh` and `render-claim-graph` hold the KB write lock for the whole run.
A writer that cannot take it within 30 s gets `retry`. While a build runs, these are refused at once
(`check: lock`). A build waits out a write in progress for up to 30 s. A resume restores every path
under `kb-root/` to the last boundary, so edits made while a build is interrupted are lost unless
carried in git. `render-citation`, `verify` and the queries take no lock. Locks are advisory and
Unix only.

**Demoted edges.** When the build breaks a dependency cycle, the edge it cuts is recorded as a
`demoted` relation carrying its origin, `cited` or `inferred`, instead of becoming a plain
reference. `verify` lists each under `findings` with outcome `done`, `stats` counts them as
`demoted_edges`, and the sheets draw them as cuts. `resolve-demoted` takes `entry` items of `id`,
`target` and `action`: `remove` deletes the edge, `restore` rewrites it as a dependency and is
refused (`check: dependency-cycle`) where that would close a cycle.

**Format migration.** Read-only commands write nothing, even over an older KB. A write op's
`--no-refresh` on an unmigrated KB is refused (`check: kb-format`, remedy `kbase refresh`), and
`verify` reports it stale with the same remedy. The migrating `refresh` lists obsolete paths under
`removed`. A KB stamped with a newer major or minor is refused (`check: kb-format`; update kbase).
`status` and `cancel` do not load the KB.

### Claim-graph sheets

`refresh` and `render-claim-graph` draw the claim graph as SVG: `kb-root/claim-graph.svg`, the whole
KB with every volume as a cluster; and, where two or more volumes hold nodes,
`kb-root/claim-graph-digest.svg`, one box per volume with the edges between volumes counted, and
`kb-root/<volume>/claim-graph.svg` for each volume (its claims plus every claim one edge away).
Edges are styled by how the dependency was found, nodes by kind and solidity, and every node links
to its register entry.

The sheets need Graphviz's `dot` on `PATH`, any version. Without it, existing sheets are left as
they are, `refresh` writes a placeholder `kb-root/claim-graph.svg` only where none exists, and
`render-claim-graph` reports `sheet: placeholder`. Sheets are rewritten only when their content
changes. A digest or volume sheet the KB no longer calls for is removed by the next render and
reported under `removed`. The drawing is specified in [SPEC.md](SPEC.md) §3.

### Using kbase from an agent harness (MCP)

`kbase mcp` serves one KB to any agent harness that speaks the Model Context Protocol, over stdio.
Every tool is one subcommand, with the same arguments and the same result.

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

- **Tools.** The subcommands: the write ops, `render-citation`, the queries (including `show` and
  `stats`), `refresh`, `verify`, `render-claim-graph`, `status`, `cancel` and `build`. `completion`,
  `help`, `models`, `configure` and `mcp` are not offered. A write-op tool takes one entry and
  offers no `--no-refresh`.
- **`build` is detached.** It starts the build as its own process and returns once the build has
  entered a stage or has exited, waiting at most 40 s. On entering a stage it returns the
  `state-dir`, `pid` and `resume` command; follow it with `status` and stop it with `cancel`. At the
  bound the result is `failed`, naming the state directory and saying the build is still starting;
  the build keeps running, and `status` reports it once it begins. The build's output is captured
  under the state store's `reports/`.
- **Resources.** The claim-graph sheets are resources, `kbase://sheet/<path under kb-root>`, read as
  SVG.
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

Two TOML files in `--config-dir`, else `$KBASE_CONFIG_DIR`, else `~/.config/kbase`. Both are loaded
strictly: an unknown key fails the whole file, and the message names the key.

- **`providers.toml`** is the endpoint pool. Each top-level table is a provider you name, with
  `baseUrl` (required) and `apiKeyFile` (relative to the file) or `apiKeyUnsafe`. kbase warns if a
  key file is group- or world-readable; `chmod 600` it.
- **`config.toml`** holds the choices: `provider`, and `[models]` `heavy` and `light`. `kbase
  configure` writes it and preserves every other byte of an existing file.

The environment variables `KBASE_API_BASE_URL`, `KBASE_MODEL` and `KBASE_API_KEY_FILE` each override
the one corresponding field; `KBASE_MODEL` names both tiers. Precedence is flags, then `KBASE_*`
environment, then the files, and the environment alone suffices with no config directory.

Provider selection is `--provider`, else `config.toml`'s `provider`, else the sole `providers.toml`
entry. In a build, the claim graph's letter asks go to the light model and the overview passage to
the heavy one.

Global flags on every subcommand: `--config-dir`, `--log-level debug|info|warn|error` (default
`warn`), `--log-file`. The root command also takes `-v`/`--version`. `kbase mcp` without `--kb-root`
finds the KB from the working directory, as every subcommand does.

## Relation to kb_tools

kbase builds exactly what kb_tools builds, as Go, with its own provider management. A KB from either
is navigable by the kb-docent agent and maintainable by kb-maintainer, and each toolchain's checks
run green over the other's output.

Differences you will notice:

- The claim-graph sheets are drawn through Graphviz; kb_tools is adopting the same drawing in place
  of its own layout.
- Write ops take YAML on stdin or `--values`, not a TOML file, and results are YAML documents with
  the exit codes above.
- The inference endpoint comes from `providers.toml` or the `KBASE_API_BASE_URL`, `KBASE_MODEL` and
  `KBASE_API_KEY_FILE` variables, not kb_tools' `API_BASE_URL`, `MODEL`, `API_KEY_FILE` and
  `ALLOW_HTTP`; a cleartext key over non-loopback `http://` is a one-line warning, not gated.
- List queries page (`--limit`, `--offset`), where kb_tools returns every result.
- Build state lives outside the worktree; the three build records at the repository root are YAML:
  `kb-build-node-pass.yaml`, `kb-build-unmarked.yaml`, `kb-build-classification.yaml`.
- The KB's metadata is format `1.0.0` (YAML frontmatter, YAML records, `.index/*.yaml`), stamped
  `kb-format` in the entry point's frontmatter, in both toolchains. kbase reads a `0.9.0` KB, built
  before either adopted it, by converting it in memory, and its next `refresh` or write op rewrites
  it in the current form. A KB stamped with a newer version is refused.
- Re-issuing an insert adopts the existing entry instead of minting a second one.
- The dead-link gate covers `kb-root/`, not the whole repository.
- The KB's stamped `AGENTS.md`, `CONVENTIONS.md` and `README.md` give the maintenance commands for
  both toolchains.

The full list is [SPEC.md](SPEC.md) §4.

## Development

```sh
just             # list recipes
just edit-gate   # cheap gate: run after every change
just checkpoint  # full suite: run at checkpoints
just test        # unit tests (VERBOSE=1 for per-test output)
just dist        # checkpoint, cross-build every target, stage the user distro tarball
```

`just dist` stages the distribution tarball. No built binaries are committed; `bin/` and `dist/` are
gitignored. The tarball ships this README and [USER_CONVENTIONS.md](USER_CONVENTIONS.md) as
`CONVENTIONS.md`, the operating contract for an agent driving the CLI.

Integration recipes run `./bin/kbase` itself and keep their evidence under `test_data/transient/`.
Recipes that need pandoc, a live provider or the kb_tools reference are excluded from `just
test-integration` and say so in `just --list`.

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
