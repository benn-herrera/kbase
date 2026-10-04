# README – KBase

kbase is a Go binary that builds a knowledge base (KB) from LaTeX and maintains it afterward. Give it a paper's top `.tex` file and it writes a navigable Markdown tree with a claim graph, built by a mechanical pass and refined by inference.

The KB is the one [kb_tools](#relation-to-kb_tools) builds: either toolchain can maintain a KB the other built.

## Requirements and Supported Platforms

- **pandoc 3.12.** Required on the host, not shipped. Any pandoc whose `pandoc-api-version` is 1.23.x is accepted; a missing or out-of-range pandoc is refused at preflight.
- **git 2.56.0.** Required on the host. A build records its progress as commits in your repository.
- **A provider configuration**, for builds that spend inference. A build run with `--no-inference` needs none.
- **Go**, to build kbase from source. `just` runs the project's recipes.

Input is LaTeX only. A volume root is the paper's top `.tex` file: the one `00README.json`'s `toplevel` entry names, else the sole file containing `\documentclass`.

## Quick Start Guide

Build the binary (`bin/kbase`, host platform only):

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

`kbase models` lists the model ids the provider offers. Details are under [Configuration](#configuration).

Build a KB inside a git repository:

```sh
git init paper-kb && cd paper-kb
kbase build /path/to/paper/main.tex
```

The KB appears at `kb-root/` beside `.git`. A fresh `kb-root/` starts at `kb-root/entry-point.md` and descends through `index.md` files.

To build without spending inference, add `--no-inference`. The build drops the rows that spend inference, states which ones, and still produces a real KB. No `README.md` is written into it, because the overview passage is one of the dropped rows.

## Building a KB

```
kbase build <volume-root> [--bibliography FILE]... [--charter FILE] [--no-inference]
            [--through <stage>] [--state-dir DIR]
```

- **Bibliographies.** By default every `.bib` beside the volume root is used, in sorted order. `--bibliography` replaces that set. An unreadable bibliography is reported and the build continues without it.
- **`--through <stage>`** stops after the named stage, by id or display name. A bounded run exits 0 with outcome `bounded`.
- **`--charter FILE`** records a statement of the build's scope as `kb-build-charter.md`.
- **Refusals.** The build refuses outside a git worktree (run `git init`), against a populated `kb-root/` with no `kb-build:` commit trail, over dirty paths it owns, on unparseable source, on an include it cannot load, and when a stage that calls a model has no provider configured. Each refusal names what it found.
- **Determinism.** The same inputs and pandoc version give a byte-identical document tree; the claim graph is identical except for the node ids, which are minted fresh on every build and are unique within the KB.

Stages, in order: `start`, `document-graph`, `spine-seed`, `claims-declared`, `claims-discovered`, `equations-minted`, `depends-attributed`, `phase-3a`, `overview-drafted`. What each does is in [ARCHITECTURE.md](ARCHITECTURE.md) §4.

### Resuming and monitoring

Each stage boundary is a commit in your repository, with subject `kb-build: <stage-id> | <display name>`. Run `kbase build` again with the same arguments and it resumes from the last boundary on the commit trail. After an interrupted stage, it restores the paths the build owns to the last stage commit and re-runs that stage. To go back further, reset to an earlier stage commit and resume from there.

`build` runs in the foreground. From the repository, in a second shell:

```sh
kbase status   # state, pid, stage commits, progress, the resume command
kbase cancel   # stop the build; costs only the unit in flight
```

State that does not belong in the repository (run lock, `progress.jsonl`, per-call evidence, the answer cache) lives in `$XDG_STATE_HOME/kbase/<key>/`, or in `--state-dir`. It holds paid-for inference and survives `git clean`.

## Maintaining a KB

After a build, subcommands operate on the living KB: write ops that insert entries and set fields, queries over the claim graph, `refresh` and `verify`, and `render-claim-graph`. Run `kbase --help` for the list and `kbase <command> --help` for each one.

- Write ops take values as YAML (JSON accepted) on stdin or `--values`. Re-issuing an op leaves `kb-root/` byte-identical, and the second call reports `unchanged`.
- A write op ends with a `refresh` unless given `--no-refresh`. `kbase verify` then checks freshness, links and citations.
- Maintenance never requires git.

### Output and exit codes

Every subcommand writes one YAML document to stdout; stderr is for humans. The `outcome` key decides the exit code.

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success |
| `refused` (including verify findings and usage errors) | 1 | Wrong input or KB state |
| `retry` | 2 | Concurrent writer or live lock; re-issue identically later |
| `failed` | 3 | Defect in the tool, its input or the environment |
| `cancelled` | 4 | Stopped on request; resumable |

## Configuration

Two TOML files in `--config-dir`, else `$KBASE_CONFIG_DIR`, else `~/.config/kbase`. Both are loaded strictly: an unknown key fails the whole file, and the message names the key.

- **`providers.toml`** is the endpoint pool. Each top-level table is a provider you name, with `baseUrl` (required) and `apiKeyFile` (relative to the file) or `apiKeyUnsafe`. kbase warns if a key file is group- or world-readable; `chmod 600` it.
- **`config.toml`** holds the choices: `provider`, and `[models]` `heavy` and `light`. `kbase configure` writes it and preserves every other byte of an existing file.

Provider selection is `--provider`, else `config.toml`'s `provider`, else the sole `providers.toml` entry. In a build, the claim graph's letter asks go to the light model and the overview passage to the heavy one.

Global flags on every subcommand: `--config-dir`, `--log-level debug|info|warn|error` (default `warn`), `--log-file`.

## Relation to kb_tools

kbase builds exactly what kb_tools builds, as Go, with its own provider management. A KB from either is navigable by the kb-docent agent and maintainable by kb-maintainer, and each toolchain's checks run green over the other's output. One exception: kb_tools' sheet freshness check on a kbase-written `claim-graph.svg` is red until kb_tools' `refresh` runs.

Differences you will notice:

- `claim-graph.svg` is a placeholder showing "NYI" and a digest of the index, not the drawn sheet.
- Write ops take YAML on stdin or `--values`, not a TOML file, and results are YAML documents with the exit codes above.
- The inference endpoint comes from `providers.toml`, not the `API_BASE_URL`, `MODEL`, `API_KEY_FILE` and `ALLOW_HTTP` environment variables.
- Build state lives outside the worktree, and the node-pass and classification records are YAML (`kb-build-node-pass.yaml`, `kb-build-classification.yaml`).
- Re-issuing an insert adopts the existing entry instead of minting a second one.
- The dead-link gate covers `kb-root/`, not the whole repository.
- The KB's stamped `AGENTS.md`, `CONVENTIONS.md` and `README.md` give the maintenance commands for both toolchains.

The full list is [SPEC.md](SPEC.md) §4.

## Development

```sh
just             # list recipes
just edit-gate   # cheap gate: run after every change
just checkpoint  # full suite: run at checkpoints
just test        # unit tests (VERBOSE=1 for per-test output)
just dist        # checkpoint, cross-build every target, stage the user distro tarball
```

`just dist` stages the distribution tarball. No built binaries are committed; `bin/` and `dist/` are gitignored. The tarball ships this README and [USER_CONVENTIONS.md](USER_CONVENTIONS.md) as `CONVENTIONS.md`, the operating contract for an agent driving the CLI.

Integration recipes run `./bin/kbase` itself and keep their evidence under `test_data/transient/`. Recipes that need pandoc, a live provider or the kb_tools reference are excluded from `just test-integration` and say so in `just --list`.

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

pandoc (John MacFarlane and contributors) is run as an external program and is not bundled.
