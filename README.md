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
  sheets](docs/maintaining.md#claim-graph-sheets); without it they are not drawn.
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

```bash
brew tap benn-herrera/tap
brew install kbase
```

The formula builds kbase from the tagged release's source and puts it on `PATH`, with the tools it
needs; see [benn-herrera/tap](https://github.com/benn-herrera/homebrew-tap)

### Windows

No binary is distributed for Windows. Build it from source.

### From source

`just build` writes `bin/kbase` for the host platform; [Development](docs/development.md) lists
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
into the directory `--config-dir` or `$KBASE_CONFIG_DIR` names; `--layer repository` writes
`<repository>/.kbase/` instead. `configure-providers` dials the provider where its key is already
stored, and otherwise prints the steps that follow, so it can run before the key is stored.
`configure-models` dials each provider it names and warns on a model id the catalogue lacks.
`configure-providers` refuses a base URL that carries credentials; store the key with `kbase auth add`. Details
are under [Configuration](docs/configuration.md).

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

A KB can also be created empty, with no LaTeX and no build: see
[Creating an empty KB](docs/building.md#creating-an-empty-kb).

## Recovering a stored key

`kbase auth get-piped <provider>` writes the key stored for `<provider>` to stdout, which must
be a pipe or a file: stdout that is a terminal (or another character device, such as
`/dev/null`) is refused before the key is read. Pipe it to where you keep keys, such as
`kbase auth get-piped local | pbcopy`; a refusal goes into that pipe instead. The refusal
guards only stdout pointed straight at a terminal: `| cat` still shows the key. A redirect to a
file leaves the key on disk in plain text, readable as the shell's umask allows: create the file
owner-only first. The command is left out of kbase's help and MCP tools. Its contract is in
[SPEC: Provider commands](PROJECT_DOCS/SPEC/9-configuration/provider-commands.md).

## More documentation

- [Building a KB](docs/building.md): `kbase build`, its stages and refusals, resuming and monitoring a
  build, creating an empty KB.
- [Maintaining a KB](docs/maintaining.md): write ops, queries, scoring, locks, format migration, the
  claim-graph sheets, output and exit codes.
- [MCP](docs/mcp.md): using kbase from an agent harness.
- [Configuration](docs/configuration.md): providers, models, credentials, `show-config`, `probe` and
  the global flags.
- [Development](docs/development.md): the `just` recipes, releasing and the integration fixtures.
- [docs index](docs/index.md), and the project's contract documents under `PROJECT_DOCS/`:
  [THESIS](PROJECT_DOCS/entry-point.md) (why the build has its shape),
  [SPEC](PROJECT_DOCS/SPEC/entry-point.md) (what any compliant implementation must do),
  [ARCHITECTURE](PROJECT_DOCS/ARCHITECTURE/entry-point.md) (how this implementation meets the spec) and
  [CONVENTIONS](PROJECT_DOCS/CONVENTIONS/entry-point.md) (house rules and practices).

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
