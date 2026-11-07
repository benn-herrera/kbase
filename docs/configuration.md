# Configuration

How kbase finds its providers, models and credentials, and the commands that write and inspect them. Back to the [README](../README.md).

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
  only where its server also speaks that surface. A provider's `decisionApi` names which decision API its server speaks —
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
  `providers.toml`, creating the file where none stands, with `--api` and `--decision-api`
  setting its chat protocol and decision API; `--base-url` must be an absolute http or https URL
  carrying no credentials. `--remove` removes the provider's table instead, and a provider with no
  table is refused. Where the provider has a key it dials the provider and gives its catalogue as `models`;
  otherwise `next` lists the steps that follow (`kbase auth add`, then `kbase models --provider
  <name>`). **`kbase configure-models --decision|--text|--scoring <provider>/<model>`** assigns
  the roles given, at least one, and leaves the rest as they are. Each provider it names must be
  defined by the `providers.toml` of the layer it writes into or, under `--layer repository`, the global one.
  Each is dialled once; a model missing from its catalogue is a warning, an unlistable catalogue fails the command,
  and a refusal writes nothing. Both, like `models`, list each configuration fault of the layers
  they read under the result's `warnings`, and `configure-models` lists each catalogue miss there
  too; each also goes to stderr, so an MCP caller sees them as a CLI user does. Six verbs report
  configuration faults under `warnings`: `build`, `configure-models`, `configure-providers`, `models`,
  `probe` and `score` (over MCP, the launch result of `build` and `score` carries the configuration's
  faults as read at launch; the run's own result carries the run's own). Both configure commands
  preserve every other byte of an existing file. They write
  `~/.config/kbase`, or the directory `--config-dir` or `$KBASE_CONFIG_DIR` names;
  `--layer repository` writes `<repository>/.kbase/` (refused outside a worktree) and `--layer global`
  writes `~/.config/kbase` even where the variable is set. `--layer` with `--config-dir` is refused.

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

`kbase probe [--model <provider>/<model> | --role decision|text|scoring] [--prompt TEXT]` asks
one model one question and reports what came back: the selection asked, the provider, the model, the
time to the first token, the call's wall time, the finish reason, the reply, and the usage the
provider reported. A probe asks one selection: `--role` names a role, `decision`, `text` or `scoring`, and asks the
selection the configuration resolves for it — `models.<role>` in the file — and `--model` names a
reference to reach instead, resolved against the same providers. The two are exclusive; with neither
the probe asks the `scoring` role — the selection a run would make — so
`kbase probe` with no arguments is the heartbeat of a configuration. The result names the selection
that answered. It reads no KB and reads or writes no state directory: one ask, one result.
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
