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
3. `migration` — `from`, `to`, `note`, on the result of a verb that read the KB on an older metadata
   format, until a save stamps it current; absent otherwise
4. the subcommand's own keys
5. by outcome, and last: `refusals` (`refused`, `retry`), `failures` (`failed`), or `cancelled`
   (`stage` and `unit` on a build, `item` on `score`)

| `outcome` | Exit | What to do |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success |
| `refused` | 1 | Wrong input or KB state; nothing written past the last checkpoint. Correct the input and re-issue |
| `retry` | 2 | Concurrent writer or live lock. Re-issue the identical command later |
| `failed` | 3 | Defect in the tool, its input or the environment. Re-issuing will not fix it; report the `failures` |
| `cancelled` | 4 | Stopped on request; resumable |

The same fact always uses the same key: `written` (files this call wrote; `[]` when none), `removed`
(files this call removed; `[]` when none), `ids` (node ids, in entry order), `count` (total before
any limit), `resume` (a shell command that continues the work), `state-dir` (the build state
store's absolute path).

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
- Result keys: `state-dir`, `roles` (one record per role the build resolved — `role`, `back-end`,
  `provider`, `provider-rule`, `provider-layer`, `model` — absent under `--no-inference`), `through`,
  `no-inference`, `resumed`, `restored`, `stages` (each `stage`, `commit`, `dropped`, `report`), and
  `resume`. `through` and `stages` are absent from a result whose invocation walked no stage;
  `resume` is absent unless an identical re-issue continues
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

**Write ops** — `insert-claim-entry`, `insert-analysis-entry`, `insert-experiment-entry`,
`insert-work-entry`, `set-bearing`, `set-rigor`, `set-rationale`,
`add-rests-on`, `set-frontmatter`, `mark-claim-in-leaf`, and the three removal ops
`drop-claim-edges`, `drop-inbound-edges` and `drop-claim-node`, beside `resolve-demoted`:

- Values are one YAML (or JSON) document on stdin or in the file `--values` names. Its one key,
  `entry`, lists the entries; the batch is written whole or not at all.
- Each op's key set is closed. A key outside it is refused, with `allowed` listing the keys the op
  takes.
- Every op is idempotent: re-issuing the same values leaves `kb-root/` byte-identical and reports
  `unchanged`. An insert whose register already holds an entry with the same title (a work: the same
  key) adopts it rather than writing a second; `adopted` lists `entry`, `id`, and `differs` — the
  supplied keys whose values the existing entry does not carry.
- `insert-claim-entry`, `insert-analysis-entry` and `insert-work-entry` create a missing register
  only under `--create`.
- Each op ends with a refresh unless given `--no-refresh`; after a run of `--no-refresh` writes, run
  `kbase refresh`.
- Result keys: `ids` (inserts only), `minted`, `adopted`, `written`, `refreshed` (null under
  `--no-refresh`), `removed` (the trailing refresh's; `[]` under `--no-refresh`).
- `resolve-demoted` takes `entry` items of `id`, `target` and `action`: `remove` deletes the demoted
  edge, and `restore` rewrites it as a `supports` premise, refused (`check: dependency-cycle`) where
  that would close a cycle.

**`render-citation`** takes values the same way and writes nothing; `citations` holds one sanctioned
citation string per entry.

**Queries** read `kb-root/.index/` and write nothing; `results` keeps the query's own key names.
`show <id>` and `stats` return a mapping. The list queries — `deps`, `gated-on`, `cited-by`, `find`,
`referenced-by`, `strength-below`, `subtree`, `weak-points` — return at most `--limit` results
(default 50; `0` for all) from `--offset`, with `count`, `offset` and `truncated`. While `truncated`
is true, re-issue with `--offset` set to `offset` plus the number of `results` returned.

**Scores.** A node's own value is `rigor`, a scoring edge's is `bearing`, and a claim's or an
analysis's derived value is `strength`; `set-rigor` and `set-bearing` are the two ops that write a
judgement. `set-bearing` replaces an existing token and never adds one — every scoring edge already
carries one, and a missing one is a refused load. A claim, analysis, work or experiment carrying no
`rigor` reads `*pending*` in every result: unscored, not low. Its `strength` pends with it unless a
lift stands over it, in which case the lift's product is the published number. A result carries
`strength` as a number rounded to two decimals, while the register's record carries the value itself;
the arithmetic always reads the value.

**`score`** runs the scoring pass — a CLI verb, not an MCP tool: `kbase score --pass rigor|bearing`;
`--help` lists its flags. One item per call, landed through `set-rigor` or `set-bearing` under the KB
write lock. The call asks for the faults the text commits — one charge line per occurrence, each a
fault id and a span of the item's own text, or `none`, or `flt-pending` — and kbase prices the list
into the value: the scheme's criteria, its two id sets and its weights are
[RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md)'s. `rigor` covers every
claim, analysis and experiment, `bearing` every `supports`, `founds`, `lifts` and `refutes` edge; a
work's rigor is never asked — set it by hand.

- The model is the `scoring` role: `models.scoring` in `config.toml`, or `KBASE_SCORING` under the
  environment's endpoint. A run whose mode names none is refused before the KB is opened, naming that
  key; nothing falls back to the text model. `--scoring-model` overrides the role for one run — a
  `<provider>/<model>` reference in file mode, a bare id in environment mode, the harness's id under
  `--harness` — and never substitutes for one.
- A reply the parse refuses, a call cut off at the completion cap, and a failed call or landing each
  write nothing for that item: the attempt is recorded `failed` with the reason and no text, and the
  run goes on.
- Records stand under `<state-dir>/score/<run>/`; `--resume` skips what landed and a fresh run asks
  everything again. Each of the two refreshes takes the KB write lock briefly, and so does each
  landing. A resume whose landed values the running table now prices differently **re-prices** them —
  through the same write path, no model asked — and records the move in `repriced.yaml`; a resume
  under a changed ask continues under the current ask, and neither refuses.
- `--harness NAME` runs one process per item from the profile of that name in `harnesses.toml`
  instead of asking an endpoint: no endpoint is chosen, the mode is not consulted, and a repository
  profile says so once on stderr. `--harness opencode` is not a profile: it runs the opencode v2 back
  end kbase composes itself — no tools, no instructions of its own, its own model naming
  `<provider>/<model>` — and needs no `harnesses.toml` entry. `--dry-run` lists the set, asks nothing
  and cannot be resumed.
- A run holds `<state-dir>/score`, so a second is `retry` (`check: lock`).

Keys: `state-dir`, `pass`, `roles` (one record, `scoring`; absent under `--dry-run`), `run`,
`ledger`, `counts` (`total`, `done`, `malformed`, `failed`, `remaining`), `scored`, `resume` (wherever
an item is not `done`, ahead of the closing record, carrying the flags, the back end and — in
environment mode — the variables it read), and last — `failures` on `failed` (`check: error` items:
each failed item and its reason, or the error that stopped the run), `cancelled` (`item`, the one it
stopped before) on `cancelled`, `refusals` on `refused` and `retry`.

**`refresh`** rewrites derived metadata, `.index/*.yaml` and the `*.svg` claim graph diagrams.
**`verify`** writes nothing; its findings are `refusals` items, and one a refresh clears carries
`remedy: kbase refresh`. **`render-claim-graph`** reports `sheet` as `placeholder` or `drawn`; it
writes only the sheets whose bytes change, so a hand edit to a drawn one is overwritten on the next
render and never reported.

## Providers

Only a `build` with an inference-spending stage left to walk, `score` (a dry run and a `--harness`
run excepted), `models` and `configure` use a provider. A build that needs one and has none is
refused before its first stage; `--no-inference` needs none. A `--harness` run dials no endpoint, but
`models.scoring` — or `KBASE_SCORING` — must still name a model, even where the profile's command
carries no `{model}`.

Configuration is read from two layers: `<repository>/.kbase/` over `~/.config/kbase`, a repository
pool entry or harness profile replacing a same-named global one whole and a `config.toml` value
overriding the corresponding field alone. A directory `--config-dir` names, else
`$KBASE_CONFIG_DIR`, is instead the only layer read. Outside a worktree only the global layer is
read, and nothing is refused for that. Three files are read from each layer: `providers.toml`,
`config.toml` and `harnesses.toml`.

- An entry's keys are `baseUrl` (required), `apiKeyFile` (the only way an entry supplies a key),
  `type` (`inference` where unstated), `api` (`openai` where unstated) and `decisionsFlavor` (the
  decision surface, `systemone` where silent).
- `config.toml`'s `[models]` holds `decision`, `text` and `scoring`, and each value is a
  **reference**: a `providers.toml` entry name, then the model id that entry serves, split at the
  first `/` — `local/Qwen3.5-9B`, `openrouter/anthropic/claude`. A bare model id there is refused,
  naming `models.<role>`, the form and the `kbase configure` command that rewrites it; the
  top-level `provider` key is superseded, loads, and chooses nothing (a file-mode run says so once on
  stderr).
- The provider is `--provider`, else the sole `providers.toml` entry — a rule for `models` and
  `configure`, which dial an entry to list its catalogue; it is not a rule for a role. With several
  and none chosen, the refusal names `key: --provider` with every entry name as `allowed`. A
  `--provider` the pool does not hold refuses with `path` the `providers.toml` of the layer kbase
  writes configuration into — the file an entry belongs in — and `remedy: kbase configure --create`;
  a provider that faulted out of the pool carries the fault's own reason instead.
- `kbase models` lists a provider's model ids. `kbase configure` assigns the `decision`, `text` and
  `scoring` roles; a category it is not given is refused naming `models.<category>` (`check: detect`)
  with the provider's model ids as `allowed` — re-issue with `--model-map
  decision=<entry>/<id>,text=<entry>/<id>,scoring=<entry>/<id>` — except `scoring`, where the target
  file already holds one. No other category name is accepted: an unknown one is a usage refusal.
- **The environment is a second mode, not an override.** `KBASE_API_BASE_URL` plus
  `KBASE_DECISION`, `KBASE_TEXT` and `KBASE_SCORING` — bare model ids — configure inference with no
  configuration directory at all, and a role variable beside a `[models]` value is refused with one
  item per side rather than merged. A role variable without the base URL, or the base URL beside
  file-supplied models, refuses the same way. `KBASE_API_KEY_FILE` replaces the key of whichever
  endpoint is in force and is not a mode. A `--harness` run consults neither mode.
- `harnesses.toml` holds the harness profiles `--harness` names: one top-level table per profile,
  with `command` (an argv array) as its only key. `configure --create` seeds `claude` live and
  `opencode` commented with its limitation; a profile is added by editing the file, never by a
  command. A profile the repository supplied is announced once on stderr, and where it replaces a
  same-named global one whole the line names that layer too (SPEC §9.3).
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
  what named each, the pool with each entry's layer and any fault, `config.toml`'s merged `[models]`
  values, the harness profiles and their layers, the `mode`, the selection, and one `roles` record
  per role naming what answers it — and changes nothing: no KB read, no file written, no request
  made. Each resolved field names the layer that supplied it, and a gap is a diagnostic in a `done`
  result, not a refusal.
- The verbs that read configuration load all three files strictly before doing anything else (README
  "Configuration"); an unknown key fails the load of the layer holding it, no lower layer is read in
  its place, and `configure` cannot repair a file that does not load.
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
