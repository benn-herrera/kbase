### Item

Every refusal, failure and verify finding is one mapping with these keys in this order, a
key omitted when it does not apply; `detail` is always present, with at least one of `check`, `path`
or `key`:

| Key | Holds |
|---|---|
| `check` | The named check or refusal class: one of the values below |
| `path` | The file the item is about |
| `entry` | 1-based index into the batch's `entry` list |
| `key` | The setting the item is about: a values key, written as its configuration file writes it, or a flag or positional argument, written as the interface the call came through takes it: `--name` at the command line, `name` over MCP; a positional `<name>` at the command line, as its usage line writes it; an item refusing a value given for a named thing may contain that value as given, as the auth verbs' items contain the provider name |
| `line`, `column` | 1-based position in the batch or in `path`. A refusal over MCP that names a document (`path`) keeps its `line` and `column` there; one naming no `path` has neither |
| `allowed` | The fixed set the value must come from |
| `remedy` | The command that clears the item, where one exists (e.g. `kbase update-format`, `git init`); on a configuration or provider refusal it names `kbase show-config`, after the command that clears the item where one exists. How a remedy is written follows the table |
| `detail` | One sentence; under `retry` and `failed` it ends with what the outcome asks of the caller: `re-issue the identical call later`, or `re-issuing the identical call will not fix it` |

A refusal lists every offending item. A usage error is one item with `check: usage`. A
provider-selection refusal's `key` is the `provider` argument, written as `key` states, and lists
every entry name as `allowed`.

**How a `remedy` is written:**

- a word in angle brackets is a placeholder the one running it fills in, as `<command-that-prints-the-key>` ([§9](../9-configuration/a-refused-key.md));
- a `kbase` command exposed as an MCP tool — in a remedy, its prose and a result field such as `resume` alike — is:
  - over MCP, that tool's call: the tool's name, then its arguments as compact JSON, keys sorted, containing only the arguments that tool's schema has (a query op's `query {"operation":"<op>","arguments":{…}}`, `update-format {}`);
  - at the command line it is `kbase <command>`, help included, its positionals, then each flag in name order, a true bool bare, a placeholder for a repeated positional `<name>...`;
- a command no tool exposes (`auth`, `wait`, `mcp`) is a command line on both interfaces;
- a command or argument that is a field's whole text is written bare; inside other text it is set off in backticks over MCP, and at the command line a command alone;
- an argument with its value reads `name: value` over MCP and `--name value` at the command line;
- the remedy of the refusal of an id no node has (`check: unknown-id`) reads "list the nodes that stand with `kbase find`, correct `<id>`, then re-run", `kbase find` written as the interface writes a command;
- a remedy naming a write batch reads at the command line as its batch piped in, `printf 'entry: []\n' | kbase write`, and over MCP as the `write` tool's call, its entries included.

**Check values.** An item's `check` names its condition:

| `check` | Condition |
|---|---|
| `agents-redirect` | `kb-root/CLAUDE.md` exists and is not the redirect ([§3](../3-compatibility-contract/stamped-documents.md)) |
| `axiom-heading`, `bare-id`, `dead-link`, `frontmatter`, `index-freshness`, `register-freshness`, `statement-marker`, `unknown-id` | `verify` faults ([§8](../8-maintenance-subcommands/verify.md)). `unknown-id` is also the refusal of an id that no node record has, and `statement-marker` that of an op that needs a hosted node's statement marker where it has none |
| `conversion` | a conversion cannot preserve a file's content (`update-format`, [§8](../8-maintenance-subcommands/update-format.md)) |
| `demoted` | a `verify` finding, not a fault ([§10](../10-metadata-format-version/the-demoted-relation.md)) |
| `dependency-cycle` | the entry would close a cycle among the scoring edges |
| `dirty-paths` | uncommitted changes in the paths the command needs clean |
| `error` | a failure item: a defect in the tool, its input or the environment (outcome `failed`) |
| `export-dir` | `export-latex`'s output directory contains a `gen-tex/` that kbase did not write |
| `include` | an `\input` or `\include` the build cannot load, or that resolves outside the top `.tex` file's directory ([§5](../5-build-behaviour/inference-preflight.md)) |
| `index` | a record file missing from a KB, or a write while a conversion is pending ([§10](../10-metadata-format-version/one-version-read-and-written.md)) |
| `inference` | the build's overview call did not yield a usable passage (outcome `failed`) |
| `kb-format` | a KB whose format version is newer than kbase |
| `kb-root` | `kb-root/` exists where `init` or a fresh `build` would create it; a `--resume` with nothing to resume while `kb-root/` exists; a `kb-root/` with neither format-version file nor entry point |
| `lock` | the KB write lock or the run lock is held, or no run holds the store `cancel` names |
| `metadata` | a load error ([§10](../10-metadata-format-version/the-load-every-fault-refused.md)) |
| `metadata-key` | a metadata key in a paper's rendering that is neither content nor apparatus ([§5](../5-build-behaviour/inference-preflight.md)) |
| `pandoc` | pandoc missing or outside the accepted range ([§2](../2-given-interfaces/index.md)) |
| `provider` | a provider or model reference the configuration does not resolve |
| `records` | the build's document-graph records are missing from the state store and the tree regenerated to replace them differs from the `document-graph` boundary's (outcome `failed`) |
| `source` | a paper the reader cannot parse ([§5](../5-build-behaviour/inference-preflight.md)) |
| `state-dir` | a state store written by a newer kbase, or whose run record names neither verb ([§6](../6-build-state-ledger-and-monitoring/the-state-store-is-versioned.md)) |
| `symlink` | a KB document that is a symbolic link |
| `trail` | the checkpoint commits record a stage over an unrecorded earlier one |
| `usage` | an argument missing, unknown, out of domain, or conflicting with another; a `--state-dir` inside `kb-root/` |
| `worktree` | outside a git worktree, or a `project-dir` that is not a worktree root, or a repository root whose `.git` git does not parse as the worktree root |
| `write` | a failure inside a write batch (outcome `failed`) |
| a stage id | a build stage's own checks found its products disagreeing (outcome `failed`); `path` the stage's report |

**Per-subcommand keys**, following `kb-root`:

| Subcommand | Keys |
|---|---|
| `build` | `state-dir`; `roles` (one record per role the build resolved, in role order — `decision`, then `text` — in the role record's shape below; absent where it resolved none, as under `--no-inference`); `through` (absent where this invocation ran no stage); `no-inference`; `resumed` (bool); `reset` (paths reset to the last boundary); `stages` (list of `stage`, `commit` — null if unrecorded — `dropped` (step ids), `report` (path to the stage's full findings in the state store); absent where this invocation ran no stage); `resume` (the call that continues the build, written as `remedy` writes a command: at the command line `kbase build` with `--resume` and the flags the invocation was given that a resume still needs, all in name order — `--no-inference` where given, `--ask-concurrency` where given, `--state-dir`, `--config-dir` and `--project-dir` where given, `--config-dir` written absolute; over MCP the `build` tool's call with `resume: true` and the same arguments but `config-dir`, which no tool takes — never the inputs or `--through`; absent where an identical retry continues nothing: `done`, `unchanged`, `bounded`, `retry`, a `failed` or `cancelled` run that ran no stage, and a `refused` run that recorded no boundary); `warnings` (as `models`'s: a configuration fault's `path` and `detail`; `[]` when none; present on every outcome the verb reports itself; a refusal before the verb runs, a flag error or a failed configuration load, carries none) |
| `status` | `state-dir`; `state` (the build `state` vocabulary of [§6](../6-build-state-ledger-and-monitoring/kbase-status/index.md), `interrupted` included); `pid`; `started`; `updated`; `ended`; `no-inference`; `stages` (a mapping: `done`, `not-done`, `rewind`); `current` (`stage`, `done`, `total`: the current stage's units done and its unit total); `recent-refusals`, `recent-fallbacks` (items, the newest 10 of each); `cache-entries`, `cache-bytes` (integers: the answer cache's entry count and total size in bytes, [§6](../6-build-state-ledger-and-monitoring/index.md)); `resume` (null for `finished`, `running` and `bounded`; read from the run [§6](../6-build-state-ledger-and-monitoring/index.md) names, written as `build`'s for the interface reading it; `recent-refusals` items are written so too, an item an earlier kbase recorded as written) |
| `cancel` | `state-dir`; `pid`; `resume` (written as `build`'s; absent where the record's command does not parse as a `kbase build` invocation, [§6](../6-build-state-ledger-and-monitoring/index.md); for a cancelled scoring run, the `resume` its own stored result holds, null where it holds none, [§6](../6-build-state-ledger-and-monitoring/kbase-cancel.md)) |
| `build-trail` | `state-dir`; `head`; `boundaries` (oldest first: `stage`, `commit`, `subject`, `command`); `started-at` (`commit`, `command`; null where none is recorded); `dirty` (`false`, or the changed paths, the first 20 named and the rest counted) ([§6](../6-build-state-ledger-and-monitoring/kbase-build-trail.md)) |
| `init` | `written` (every path this call wrote — the scaffold, the two empty record files, the stamped documents and the output of the rebuild of the derived views, merged, deduplicated, path-sorted); `removed` (as `update-format`'s; `[]` on create) |
| `update-format` | `written` (includes every summary page, sheet and maintained-block document written); `removed` (files the KB no longer calls for — a summary page no node is rendered on, a sheet the volume-count rule drops, a file a format migration made obsolete ([§10](../10-metadata-format-version/index.md)) — kb-root-relative inside `kb-root/`, repository-relative beside it; `[]` when none); `unfenced`; `sheet` (`drawn` where the sheets were rendered this call, else `not-drawn`: nothing was drawn, whether or not a sheet exists); `dot` (`"not found on PATH"`, present only when `sheet` is `not-drawn`); `commits` (as a write batch's); `uncommitted` (as a write batch's); `ids` (where the format changed: `from`, `to`, each work's and axiom's `3.0.0` id and the id minted in its place); `bare-ids` (`path`, `line`, `id`: every bare id the run cited, the conversion's included; `[]` when none); `no-edge` and `strengthen-by` (where the format changed: `id`, `text`). `unfenced` onward are present only on `done` and `unchanged` |
| `verify` | `findings` (items the KB's structure has that are not faults — every `demoted` record ([§10](../10-metadata-format-version/index.md)), as an item with `check: demoted`, `path` the summary page of the record's `target`, omitted where that target has no host, and `detail` the edge and the relation it replaced; `[]` when none); its faults are `refusals` items, each with the remedy [§8](../8-maintenance-subcommands/verify.md) names for it where one op clears it |
| `write` and every write op | `dry-run`; `entries` (one per batch entry, in order: `entry`, `operation`, `outcome` — `done` or `unchanged` — and then the keys of its kind of entry, listed after the table; `[]` where the batch did not apply); `written` (kb-root-relative paths the batch wrote, or under `dry-run` would write: records, leaves and derived views); `removed` (the rebuild of the derived views', as `update-format`'s); `sheet` and `dot` as `update-format`'s, where the rebuild ran; `commits` (each commit made: `commit`, `label`, `paths`; `[]` without `commit`); `uncommitted` (a detail line, present only where the batch commit failed) |
| `citations-of` | `count`; `results` (every citation of the node, complete, unsliced: `path`, `line`, `text` ([§8](../8-maintenance-subcommands/citations-of.md))) |
| `find` | `count`; `results` (every node the filters keep, complete, in the sort's order, cut only by `limit`: `id`, `kind`, `title`, `label`, `host`, `strength`, `dependents` ([§8](../8-maintenance-subcommands/find.md))) |
| `edges` | `count`; `results` (every edge record the arguments keep, complete, in its fixed order: `source`, `target`, `relation`, `bearing` (as the record files write it: a number in `[0, 1]` or `*pending*` written as text, null only where the pairing has none), `context` (null where the edge has none), `manually-scored` (the record's flag ([§10](../10-metadata-format-version/provenance-and-the-judged-hash.md)), null where its record has none) and `replaced-relation` ([§8](../8-maintenance-subcommands/edges.md))) |
| `trace` | `count`; `results` (one row per edge the traversal reaches, complete, in the sort's order, cut only by `limit`: an `edges` record, then `depth`, `strength` and `contribution` ([§8](../8-maintenance-subcommands/trace.md))) |
| `show`, `stats` | `results` (the query's mapping: a `show` record or `stats`' mapping, each listed after the table) |
| `models` | `provider`; `provider-rule`, `provider-layer` (the rule that named the provider — `flag` or `sole-provider` — and the layer that supplied it; this verb always selects one); `models` (ids, sorted ascending); `warnings` (items: `path` the file or directory the configuration fault is against, `detail`; `[]` when none; present on every outcome the verb reports itself; a refusal before the verb runs, a flag error or a failed configuration load, carries none) |
| `show-config` | `layers` (one record per directory this invocation read, in precedence order: `path`, `layer` (`explicit`, `environment`, `repository` or `global`), `exists`, `providers-exists`, `config-exists` — the directory's presence and each of its two files'); `providers` (one record per entry, in name order: `name`, `base-url`, `api` (the chat protocol it speaks, `anthropic` where the entry is silent), `decision-api` (the decision API it speaks, `systemone` where the entry is silent), `key-source` (`store`, `file` or `none`), `key-file`, `upgrade` (where `key-source` is `file`, `cat <key-path> | kbase auth add <provider>`, the resolved path single-quoted where a shell would split or expand it), `layer` (the layer that supplied the entry), `usable`, `fault`; a provider a dropping fault removed has `name`, `layer`, `usable`, `fault` only); `config` (the merged choices: `models` (`decision`, `text`, `scoring` — each the reference `config.toml` states, null where unset); `sampling` (`text`, `scoring` — each `temperature` and `thinking` as `config.toml` states them, null where unset)); `selection` (`provider` and `provider-rule` — `flag` or `sole-provider` — or `detail` where no provider was named); `roles` (one record per role in role order, in the role record's shape below, each with `base-url`, `sources` (`base-url`, `model`, `key`, and for a chat role `temperature`, `thinking` — the layer that supplied that field, one of `layers`' four names, `store` for a key the store supplied, null where the field is unset), a chat role's `temperature` and `thinking` (null where unset: kbase's default, [§9.2](../9-configuration/9.2-config-toml-the-choices/index.md)) and `detail` where the role does not resolve); `diagnostics` (items: every fault, `path` the `providers.toml` that has it, the stored key file that does not open, with `remedy` the refused-key remedy ([§9](../9-configuration/a-refused-key.md)) where it is truncated or does not decrypt, or the store's directory where it exists and does not list; an entry dropped because the key file `providers.toml` names cannot be read has the same remedy ([§9.4](../9-configuration/9.4-key-file-permission-warning.md)); a warning per entry whose stored key leaves its `apiKeyFile` unread, `path` the `providers.toml` stating it; and the resolution failure where a role does not resolve) |
| `configure-providers` | `provider` (lowercase); `layer` (`global`, `repository`, `explicit` or `environment`); `written` (`[]` with outcome `unchanged`); `models` (ids, sorted ascending, where it sent a request to the provider); `next` (a list of strings, where no key is stored or that request failed: [§9](../9-configuration/provider-commands.md), "Provider commands"); `warnings` (items: `path` the file or directory the configuration fault is against, `detail`; `[]` when none) |
| `configure-models` | `models` (`decision`, `text`, `scoring`, as the written `config.toml`'s `[models]` contains them, null for a role it does not set); `written` (`[]` with outcome `unchanged`); `warnings` (items: a configuration fault has `path` and `detail`; a model missing from its provider's catalogue has `key` the role and `detail`; `[]` when none) |
| `probe` | `provider`; `provider-rule`, `provider-layer` (as `models`'s); `probed` (the selection that answered: `decision`, `text` or `scoring` where `--role` or the default named it, else the `<provider>/<model>` reference `--model` named); `model` (the id requested); `temperature`, `thinking` (chat selections only: the effective settings — `temperature` a number; `thinking` `off`, `low`, `medium` or `high`); `first-token-ms`, `wall-ms` (integers: request start to the first non-empty content or reasoning delta — zero where none arrived — and the call's end-to-end time); `finish-reason` (null where the stream reported none); `reply` (the reply's text on a chat endpoint, and on a decision endpoint the answer's probability, chosen letter or type); `usage` (a record: `reported` — whether the provider sent a usage block at all, so a zero count is a measurement only where it is true — then `prompt-tokens`, `completion-tokens`, `reasoning-tokens`, `cached-tokens`); `warnings` (as `models`'s: a configuration fault's `path` and `detail`; `[]` when none; present on every outcome the verb reports itself; a refusal before the verb runs, a flag error or a failed configuration load, carries none) |
| `auth add`, `auth remove` | `provider` (the name given, folded to lowercase; never the key) |
| `auth list` | `providers` (the names with a stored key, in name order; `[]` when none) |
| `export-latex` | `written` (absolute paths; outside `kb-root/`); `volumes` (the volume directories exported, entry-point order); `report` (the path of the export report written under the out dir — one YAML list, every simplification and finding line the run produced, each naming the form, the source path and the reason) |
| `score` | `migration` (conditional, the shared vocabulary's; it comes before `state-dir`, on the refusal of a run over an older KB, which `update-format` converts first); `state-dir`; `pass` (the set it runs: one pass, or `all`; a named node's `rigor`, a named edge's `bearing`); `roles` (one record, `scoring`, in the role record's shape below; absent under `--dry-run`); `run` (the run's id); `ledger` (the run's directory, absolute); `counts` (a record: `total`, `done`, `failed`, `skipped`, `remaining` — the items by the status the ledger last records for each, so a resumed run reports the whole run); `ask` and `skip` (a dry run's alone: the items a run would make model calls for and the stale manually scored items it would skip, named as `scored` names them, in the item loop's order ([§8](../8-maintenance-subcommands/score/the-closing-record.md))); `scored` (the items whose value was written, in the item loop's order — a node's id, or an edge's `<relation> <source> → <target>`); `resume` (the call that continues the run — `score` with `--resume`, written as `build`'s, over MCP the `score` tool's call — keeping `--state-dir`, `--project-dir` where given, `--scoring-model` where given, and `--config-dir` written absolute, and no argument of the scope; present wherever an item failed or was not reached — a `skipped` item does not make it present — and it comes before the record that ends the result; absent from a dry run, and from a stop before the item loop); `warnings` (as `models`'s: a configuration fault's `path` and `detail`; `[]` when none; present on every outcome the verb reports itself; a refusal before the verb runs, a flag error or a failed configuration load, carries none) |

A `status` read over a score run reports the score shape of [§6](../6-build-state-ledger-and-monitoring/kbase-status/a-score-run-s-status.md) in place of
the build keys above.

**The keys of an `entries` item** in a `write` and every write op result, after `entry`, `operation` and `outcome`:

- on an add's entry ([§8](../8-maintenance-subcommands/add-kind.md)) `id` (the node it minted) alone;
  - but on an `add-axiom` entry, which adds `heading` (`marked` or `appended`);
- on an edit's entry ([§8](../8-maintenance-subcommands/edit-kind.md)) `id` (the node it edited) alone;
  - but on an `edit-work` entry that changes `label`, which adds `rewritten` (each line it rewrote a reference on, `path` and `line`, ordered by path, then line; `[]` where none);
- on a `move-node` entry ([§8](../8-maintenance-subcommands/move-node.md)) `id` (the node it moved) alone;
- on a `remove-node` entry ([§8](../8-maintenance-subcommands/remove-node.md)) `id` (the node it removed) and `removed-edges` (each edge record it removed, as its `edges` row);
- on an `add-citation` entry ([§8](../8-maintenance-subcommands/add-citation.md)) `document` and `line` (where it wrote the citation), `id` (the node cited) and `link` (the text written);
- on an edge write's and a `restore-demoted` entry ([§8](../8-maintenance-subcommands/edge-ops.md), [§8](../8-maintenance-subcommands/restore-demoted.md)) `source`, `target` and `relation`, the edge it names;
  - and on a `restore-demoted` entry then `restored` (the relations it put back).

**The `results` of `show` and `stats`:**

- a `show` record has `kind`, `id`, `title`, `host`, `host-anchor` and `summary` ([§8](../8-maintenance-subcommands/show.md)), then the keys its kind has:
  - `rigor` and `strength` on a claim, with `build-status`, `strength-band`, `rationale` and `premises` (the count of premise edges) after them;
  - `rigor`, `strength`, `strength-band` and `rationale` on an analysis;
  - `status` and `rigor` on an experiment;
  - `label`, `rigor` and `rationale` on a work;
  - `label` and `rigor` on an axiom, its `rigor` `1.0` by rule;
  - `show` adds `manually-scored` ([§10](../10-metadata-format-version/provenance-and-the-judged-hash.md)) after `rigor` on a claim, an analysis and an experiment;
  - `strength` is a number rounded to two decimals for reading, `rigor` the number the node holds;
- `stats`' mapping has one count per node kind — `claims`, `analyses`, `experiments`, `axioms`, `works` — then:
  - `edges`, `demoted-edges` and `subtree-aggregates` (per volume, `claims` and `experiments` ([§8](../8-maintenance-subcommands/stats.md)));
  - `rigor-manual`, `rigor-model`, `bearing-manual` and `bearing-model` (the records whose type has `manually-scored`, `*pending*` values included, counted by the flag);
  - `relations`;
  - `strength-bands` (one count per band, keyed by the band's slug in [§10](../10-metadata-format-version/the-band-ladder.md)'s ladder order, then `unknown`, the pending claims);
  - `highest-leverage`, whose records have `id`, `strength`, `strength-band`, `dependents` and `title`, and `weakest`, whose records have `id`, `strength` and `title`;
- `relations` has one record per relation in this order — `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted` — each:
  - `edges`, the relation's row count;
  - `walked` (bool);
  - then on a scoring relation `bearings`: one record per distinct bearing its rows have, `bearing`, as the record files write it, and `edges`; numbers ascending, then `*pending*`, then null, `[]` where the relation has no rows.

