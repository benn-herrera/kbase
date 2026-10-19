# CONVENTIONS — kbase

The operating contract for an agent driving the `kbase` CLI. `README.md` beside this file is the
human guide, including provider setup. `kbase --help` lists the subcommands; `kbase <command>
--help` is current for each one's arguments and flags.

kbase builds a knowledge base (KB) from LaTeX and maintains it. The KB is `kb-root/` beside the
repository's `.git`; run every subcommand from inside that repository.

## Every invocation

Each subcommand writes one YAML mapping to stdout and nothing else. stderr is for humans; do not
parse it. `--help` and `--version` are outside this contract.

Keys come in this order:

1. `outcome`
2. `kb-root` — the KB's absolute path (absent on `models`, `configure` and `show-config`)
3. the subcommand's own keys
4. by outcome: `refusals` (`refused`, `retry`), `failures` (`failed`), or `cancelled` (`stage`,
   `unit`)

| `outcome` | Exit | What to do |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success |
| `refused` | 1 | Wrong input or KB state; nothing written past the last checkpoint. Correct the input and re-issue |
| `retry` | 2 | Concurrent writer or live lock. Re-issue the identical command later |
| `failed` | 3 | Defect in the tool, its input or the environment. Re-issuing will not fix it; report the `failures` |
| `cancelled` | 4 | Stopped on request; resumable |

The same fact always uses the same key: `written` (files this call wrote; `[]` when none), `ids`
(node ids, in entry order), `count` (total before any limit), `resume` (a shell command that
continues the work), `state-dir` (the build state store's absolute path).

### Correcting a refusal

Every refusal, failure and `verify` finding is an item: a mapping with these keys, in this order,
each omitted where it does not apply. `detail` is always present, with at least one of `check`,
`path`, `key`.

| Key | Holds |
|---|---|
| `check` | The check or refusal class, e.g. `usage`, `dirty-paths`, `include`, `metadata-key`, `lock`, `pandoc`, `unknown-id`, or a `verify` check name |
| `path` | The file the item is about |
| `entry` | 1-based index into the values document's `entry` list |
| `key` | The values key, flag or positional argument at fault, spelled as `--help` spells it |
| `line`, `column` | 1-based position in the values document or in `path` |
| `allowed` | The closed set the value must come from |
| `remedy` | The kbase command that clears the item, where one exists |
| `detail` | One sentence |

A refusal lists every offending item, so correct all of them before re-issuing. Act on the fields,
not on `detail`: locate the fault by `entry`, `key` and `line`/`column`; replace a value with one
from `allowed`; run `remedy` where given. `check: usage` means the command line itself is wrong —
read `kbase <command> --help`.

## Building

```
kbase build <volume-root>... [--bibliography FILE]... [--no-inference]
            [--through <stage>] [--state-dir DIR]
```

- `<volume-root>` is a paper's top `.tex` file: the one `00README.json`'s `toplevel` names, else the
  sole file containing `\documentclass`. Input is LaTeX only.
- The KB is written only at `<git root>/kb-root/`. `build` refuses outside a git worktree, against a
  populated `kb-root/` with no `kb-build:` commit trail, and over dirty paths it owns.
- `--no-inference` drops the rows that spend inference, walks every other stage, and still produces
  a real KB, minus its `README.md`. Each entry of the result's `stages` names its `dropped` rows.
- `--through <stage>` stops after that stage (id or display name) with outcome `bounded`, exit 0.
- `--bibliography` replaces the default set: every `.bib` beside the volume root.
- `--state-dir` overrides the state store, `$XDG_STATE_HOME/kbase/<key>/`. Pass the same value to
  every `build`, `status` and `cancel` for that KB.
- Result keys: `state-dir`, `through`, `no-inference`, `resumed`, `restored`, `stages` (each
  `stage`, `commit`, `dropped`, `report`), and `resume`. `through` and `stages` are absent from a
  result whose invocation walked no stage; `resume` is absent unless an identical re-issue continues
  the work: present only where the run recorded a boundary, so of the outcomes `cancelled`, `failed`
  and `refused`, only a run that walked a stage supplies one. A `bounded` result
  carries no `resume`: the run is finished at the bound, so re-issuing its command is not what
  continues it. Resuming past the bound is a reset to an earlier boundary and a re-issue — see
  *Monitoring, cancelling and resuming a build*.
  Per-stage findings are in the `report` file, not stdout.

## Monitoring, cancelling and resuming a build

`build` runs in the foreground until it ends; run it in the background and poll `kbase status`.

- `status` reports `state` (`none`, `running`, `cancelled`, `interrupted`, `failed`, `bounded`,
  `finished`), `pid`, `started`/`updated`/`ended`, `stages` with their commits, `current` (`stage`,
  `units-done`, `units-total`), `recent-refusals` and `recent-fallbacks` (the newest 10 items each),
  `cache-entries` and `cache-bytes` (the answer cache's entry count and size), and `resume`, which is
  absent for `finished`, `running` and `bounded`. `interrupted` is a build that died without writing
  an end event: no build writes it, and it names the stage in flight, the pid and the command that
  resumes it.
- `cancel` stops the running build; the build exits `cancelled`, losing only the unit in flight.
- To continue a cancelled or failed build, or one `status` reports `interrupted`, run the `resume`
  command exactly as given; a `bounded` build hands out none. The trail document below prints that
  command anyway, and says why.
- Position comes from the `kb-build:` commit trail: each stage boundary is a commit, and an
  interrupted stage's uncommitted work is reset to the last stage commit and the stage re-run.
  To resume from an earlier boundary, reset the repository to it and re-issue the build with the
  newest boundary's inputs — the same volume roots and `--bibliography` files, in the same order;
  other inputs are refused (`check: inputs`). Over MCP, the `rewind-build` tool and the
  `kbase://build-trail` resource report the trail, and for a `target` (a stage id or display name)
  the exact `git reset --hard` and re-issue commands, with `discards` counting the boundaries newer
  than it. Running the reset is yours: the tool changes nothing, and a dirty worktree is reported in
  `dirty` rather than refused. A `bounded` run's recorded command is printed there even though
  `status` withholds it, because after a reset to an earlier boundary there are stages left to walk
  — the case that suppression excludes.

## Maintaining a KB

Maintenance never requires git.

**Write ops** — `insert-claim-entry`, `insert-support-entry`, `insert-experiment-entry`,
`insert-work-entry`, `set-work-strength`, `set-applicability`, `set-rigor`, `set-rationale`,
`add-depends-on`, `set-frontmatter`, `mark-claim-in-leaf`, `set-on-point-fraction`:

- Values are one YAML (or JSON) document on stdin or in the file `--values` names. Its one key,
  `entry`, lists the entries; the batch is written whole or not at all.
- Each op's key set is closed. A key outside it is refused, with `allowed` listing the keys the op
  takes.
- Every op is idempotent: re-issuing the same values leaves `kb-root/` byte-identical and reports
  `unchanged`. An insert whose register already holds an entry with the same title (a work: the same
  key) adopts it rather than writing a second; `adopted` lists `entry`, `id`, and `differs` — the
  supplied keys whose values the existing entry does not carry.
- `insert-claim-entry`, `insert-support-entry` and `insert-work-entry` create a missing register
  only under `--create`.
- Each op ends with a refresh unless given `--no-refresh`; after a run of `--no-refresh` writes, run
  `kbase refresh`.
- Result keys: `ids` (inserts only), `minted`, `adopted`, `written`, `refreshed` (null under
  `--no-refresh`).

**`render-citation`** takes values the same way and writes nothing; `citations` holds one sanctioned
citation string per entry.

**Queries** read `kb-root/.index/` and write nothing; `results` keeps the query's own key names.
`show <id>` and `stats` return a mapping. The list queries — `deps`, `gated-on`, `cited-by`, `find`,
`referenced-by`, `solidity-below`, `subtree`, `weak-points` — return at most `--limit` results
(default 50; `0` for all) from `--offset`, with `count`, `offset` and `truncated`. While `truncated`
is true, re-issue with `--offset` set to `offset` plus the number of `results` returned.

**`refresh`** rewrites derived metadata, `.index/*.yaml` and the `*.svg` claim graph diagrams.
**`verify`** writes nothing; its findings are `refusals` items, and one a refresh clears carries
`remedy: kbase refresh`. **`render-claim-graph`** reports `sheet` as `placeholder` or `drawn`; it
never overwrites a drawn sheet.

## Providers

Only a `build` with an inference-spending stage left to walk, `models` and `configure` use a
provider. A build that needs one and has none is refused before its first stage; `--no-inference`
needs none.

Configuration is read from two layers: `<repository>/.kbase/` over `~/.config/kbase`, a repository
pool entry replacing a same-named global one whole and a `config.toml` value overriding the
corresponding field alone. A directory `--config-dir` names, else `$KBASE_CONFIG_DIR`, is instead
the only layer read. Outside a worktree only the global layer is read, and nothing is refused for
that.

- The provider is `--provider`, else `config.toml`'s `provider`, else the sole `providers.toml`
  entry. With several and none chosen, the refusal names `key: --provider` with every entry name as
  `allowed`. A `--provider` the pool does not hold refuses with `path` the `providers.toml` of the
  layer kbase writes configuration into — the file an entry belongs in — and
  `remedy: kbase configure --create`; a provider that faulted out of the pool carries the fault's
  own reason instead.
- `kbase models` lists a provider's model ids. `kbase configure` assigns the `decision` and `text`
  categories; a category it is not given is refused naming `models.decision` or `models.text` with
  the provider's model ids as `allowed` — re-issue with `--model-map decision=<id>,text=<id>`. No
  other category name is accepted: an unknown one is a usage refusal.
- The guidance for choosing those ids is in the files themselves: a `config.toml` kbase creates
  states beside `[models]` what each category wants and example ids, and an MCP client reads the
  same rows at `kbase://model-hints`. The ids are examples, not requirements — match the stated
  capability — and no provider surface is bound to a model class.
- `kbase configure --create` writes an annotated `providers.toml` where none stands, overwriting no
  other content; `config.toml` is created where absent and edited in place otherwise. Without
  `--provider` it makes no request and says the ids went unchecked. `--project` writes
  `<repository>/.kbase/` whether or not that directory exists, is refused outside a worktree, and
  beside `--config-dir` is a usage refusal naming `--project`.
- `kbase show-config` reports how this invocation's configuration resolves — the layers read and
  what named each, the pool with each entry's layer and any fault, `config.toml`'s merged choices,
  the selection and the endpoint — and changes nothing: no KB read, no file written, no request
  made. Each resolved field names the layer that supplied it.
- Every subcommand loads both configuration files strictly first; an unknown key fails the load of
  the layer holding it, no lower layer is read in its place, and `configure` cannot repair a file
  that does not load.
- Reference an API key with `apiKeyFile`, written with a leading `~` where the file lives under the
  home directory (`~/keys/lab.txt`): a relative path resolves beside `providers.toml`, and `~user`
  is not expanded. The same rule holds for `--config-dir`, `$KBASE_CONFIG_DIR` and
  `$KBASE_API_KEY_FILE`. Never echo key material.
- An entry's `decisionsFlavor` says which decision surface its server speaks: `systemone` (the
  default) or `decisions`. It names a wire shape, not a model — the decision category's id stays in
  `config.toml` — and chat is `chat/completions` either way. Any other value drops the entry the way
  a wrong `type` or `api` does.
- `kbase show-config` states each entry's effective `decisions-flavor`, so an entry silent on it
  reads `systemone`.
