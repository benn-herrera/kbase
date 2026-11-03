## 12. Interim releases

**<release version> — the files configure inference; the served surface is smaller.**

- **Every command's help is an MCP resource**, `kbase://help/<command>`: its shared body, then its
  generated argument list, then its MCP note; `--help` is the usage line, the same body, the list
  as the command line takes the arguments (shorthands, repeated flags, a mapping's joined values),
  then the CLI note, which names the global flags ([page §4.2](../../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)). The tool's schema states its arguments, each described, a
  flag's with the default its list states ([§11](../11-the-mcp-server.md)).
  Each is listed as `<command> help`, and the `query` tool's shared help as `query help`.
- **`provider` and `[asks] readerConcurrency` leave `config.toml`.** A file still stating either
  fails strict load naming the key ([§9.3](../9-configuration/9.3-strict-load-contract.md)); delete the line by hand — the configure commands cannot load
  the file to repair it.
- **`configure` is `configure-providers` and `configure-models`**, each a CLI command and a tool
  ([page §3.7](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md)). `configure-providers <provider> --base-url <url>` (`--api`,
  `--decisions-flavor`) writes a `providers.toml` entry, and `--remove` removes one, a provider with
  no entry refused; it dials the provider where its key is stored, and otherwise names the next
  steps. `configure-models --decision/--text/--scoring <provider>/<model>` writes the roles given
  and leaves the rest. `--create`, `--project`, `--provider` and `--model-map` are gone, and with
  them the `detect` refusal. Both write `~/.config/kbase` by default, or the directory `--config-dir`
  or `$KBASE_CONFIG_DIR` names; `--local` writes `<repository>/.kbase` — the repository layer is no
  longer chosen because its directory stands ([§9](../9-configuration/the-configure-commands-write-location.md)). A created `providers.toml` holds the entry
  written alone, the template's example commented out. A provider the stack `configure-models`
  writes into does not hold is now refused with the `configure-providers` call that adds it as
  the remedy.
- **A base URL carries no credentials.** `configure-providers` refuses a `--base-url` carrying
  `user:key@`, and a `providers.toml` entry whose `baseUrl` carries it, or is no absolute `http`
  or `https` URL, is dropped as it loads; neither names any part of the URL. A key is stored with `kbase auth add`. A
  `configure-providers --remove` names the roles and the stored key that still refer to the
  provider it removed.
- **The environment inference mode is gone.** `KBASE_API_BASE_URL`, `KBASE_API_KEY_FILE`,
  `KBASE_DECISION`, `KBASE_TEXT` and `KBASE_SCORING` are not read; `providers.toml` and
  `config.toml` are the one source, and a run isolated from your layers names its directory with
  `--config-dir` or `$KBASE_CONFIG_DIR` ([§9](../9-configuration/index.md)). The role record carries no `back-end`, and
  `show-config` no `mode` or `environment` ([§7](../7-tool-result-contract-and-exit-codes/index.md)).
- **`find` replaces `weak-points` and `strength-below`; `edge-census` is gone.** `find --kind claim`
  lists claims between `--min` and `--max`, sorted by strength or by dependents, or with
  `--pending` the unscored ones ([§8](../8-maintenance-subcommands/find.md)); `edge-census`' counts are `stats`' `relations`, without the
  per-bearing manual and model split ([§7](../7-tool-result-contract-and-exit-codes/index.md)).
- **The superseded write ops and list queries are removed**, with no subcommand, MCP operation or
  `kbase://help/<name>` resource left. Writes: `insert-claim-entry`, `insert-analysis-entry`,
  `insert-experiment-entry`, `insert-work-entry` and `insert-axiom-entry` are `add-claim`,
  `add-analysis`, `add-experiment`, `add-work` and `add-axiom`; `set-rigor` is `edit-claim`,
  `edit-analysis`, `edit-experiment` or `edit-work` with `rigor`, `set-rationale` is `edit-claim`,
  `edit-analysis` or `edit-work` with `rationale`, `set-status` is `edit-experiment` with `status`, and
  `set-bearing` is `edit-edge` with `bearing`; `add-rests-on` and `add-pairs` are `add-edge`;
  `mark-claim-in-leaf` is `move-node`; `drop-claim-edges`, `drop-inbound-edges` and `drop-edge` are
  `remove-edge`; `drop-claim-node` is `remove-node`; `render-citation` is `add-citation`. Queries:
  `deps` is `edges` (`--id`, `--direction`) and `trace`; `cited-by` and `referenced-by` are
  `citations-of`; `strength` is `find` (`--kind claim`, `--min`, `--max`, `--min-dependents`,
  `--sort`, `--order`, `--limit`, `--pending`); `subtree` is `find --path`. No list query pages:
  `--offset`, the result's `offset` and `truncated` and the default cut of 50 are gone, `find`,
  `edges`, `trace` and `citations-of` return the complete answer with `count` beside `results`, and
  `find` and `trace` keep `--limit` only beside an explicit `--sort`. A write batch's entry carries
  no `minted`, `adopted` or `differs`, and the `query` tool has no values operation
  ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md), [§11](../11-the-mcp-server.md)).
- **`resolve-demoted` is `restore-demoted`, and takes no `action`.** It restores the edge a
  `demoted` record replaced, named as the record's `edges` row names it — `source`, `target` and
  `relation: demoted`; `remove-edge` removes the record instead. Its result carries no `resolved` key
  ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`render-claim-graph` is gone**: `update-format` draws the sheets and reports `sheet` and `dot` ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **The queries are one MCP tool.** `query` runs one read-only query: `operation` names it
  from an enum and `arguments` carries what its own tool took; a bad argument is a usage refusal
  naming it and the allowed set. Every subcommand stands on the command line, and what the queries
  share is stated once, in `kbase --help` and in `kbase://help/query` ([§11](../11-the-mcp-server.md)).
- **Every write is a batch.** `kbase write` and the MCP `write` tool take an ordered batch whose
  entries each name their op under `operation`; each write op's subcommand takes a batch of that op
  alone. A batch checks every entry's shape first, then applies the entries in order and stops at
  the first refusal; nothing is written unless all of it is, the summary pages and sheets included.
  `dry-run` (`--dry-run`) reports a batch's result and writes nothing, an empty batch re-derives the
  summary pages and sheets, and `--no-refresh` is gone. A write's result carries `dry-run`,
  `entries` — each entry's op and outcome, an add's id among them, in place of `ids` — `written`, `removed`, `sheet` and `commits`, and no `refreshed` ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md),
  [§11](../11-the-mcp-server.md)).
- **A write batch, and `update-format`, can commit `kb-root/`.** `commit: true` (`--commit`) makes
  git the batch's undo: before writing, the batch commits `kb-root/`'s uncommitted changes outside
  `.index/`, then those inside it, each alone, and after writing commits the batch itself, labelled
  `kb-write: <ops> <ids>`, so reverting that commit undoes exactly the batch. A failed pre-flight
  commit writes nothing; a failed batch commit leaves the batch written, `done`, with
  `uncommitted`. `update-format --commit` commits the conversion as one commit. `commits` lists
  each commit's `commit`, `label` and `paths`; committing is never the default. Where the `.git`
  beside `kb-root/` is not the one git reads as the worktree root, every command that runs git over
  the KB — a committing batch, dry run included, `update-format`, `init`, `build`, `status`,
  `cancel` and `wait` — is refused `check: worktree` before it writes ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **A `rigor` above `1.00` in the node records is a load error.** The load refuses it as it
  refuses `.inf`, naming the record and the field ([§10](../10-metadata-format-version/index.md)); no write op writes one.
- **KB format `4.0.0`'s records.** Each scored record carries its `judged-hash`, an experiment's
  `status` is `run` or `planned`, and no record carries `no-edge` or `strengthen-by`; a `3.0.0` KB
  converts at load, a warning quoting each dropped text ([§10](../10-metadata-format-version/index.md)). `gated-on` is gone, and `show` and
  `stats` carry no strengthen-by keys ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **KB format `4.0.0`'s markers, anchors and summary pages.** A statement marker is
  `<!-- kbase-id: <id> -->`, attached with no space; a heading's anchor follows GitHub's slugger, a
  repeat suffixed `-1`, `-2`, so a canonical anchor can differ from the `3.0.0` one; the register
  pages are `kb-summary.md`, titled `# KB summary`, each entry's heading carrying its node's marker.
  A `3.0.0` KB's markers convert at load and its `claim-rigor.md` pages are removed by
  `update-format` ([§10](../10-metadata-format-version/index.md)).
- **KB format `4.0.0`'s citations.** A citation is a Markdown link whose title is the cited node's
  id, checked as every link is; a bare node id in prose is a `verify` fault, `bare-id`, and the
  build's citation-grammar check is gone. `add-citation` writes the id of the node it cites as the
  title. A `3.0.0` KB's links
  convert at load: each into a heading re-anchored, each resolving to one node titled with its id, a
  work's pointed at its root summary entry, each bare id made its citation, and each it cannot
  resolve reported ([§8](../8-maintenance-subcommands/index.md), [§10](../10-metadata-format-version/index.md)).
- **A KB's load no longer degrades.** One edge identity on two records, an edge naming a node no
  record carries and a cycle among the scoring edges are load errors, `check: metadata`, refused by
  every command that loads the KB, `verify` and `update-format` included; recovery is in git, returning the
  record files to a consistent commit and reapplying the other side's changes through write ops.
  `update-format` repairs nothing and its result carries no `warnings`; `check: degraded` is gone
  ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§10](../10-metadata-format-version/index.md)).
- **`update-format` replaces `refresh`.** It converts a KB in an older format, re-derives every
  register page and sheet and rewrites the maintained blocks, whether or not the format changed,
  and refuses a `kb-root/` with uncommitted changes (`check: dirty-paths`, naming each path). Its
  result adds `unfenced` and `commits` and, where the format changed,
  lists the conversion's minted work and axiom ids, bare-id citations, dropped `no-edge` texts and
  dropped `strengthen-by` texts under `ids`, `bare-ids`, `no-edge` and `strengthen-by`. `verify`'s
  `index-freshness` fault names `kbase update-format` as its remedy ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **A conversion no longer rides on a write.** While a KB in an older format is unconverted, every
  write but `update-format` — a write batch, `score`, dry run included,
  and `build` resuming over it — is refused, `check: index`, remedy `kbase update-format`; reads keep
  answering as converted ([§10](../10-metadata-format-version/index.md)).
- **`AGENTS.md`, `CONVENTIONS.md` and `README.md` hold a maintained block.** kbase rewrites only the
  text between the block's `vvv-kbase` and `^^^-kbase` marker lines; text outside it is the KB's own.
  A document without the pair is left untouched and listed under `update-format`'s `unfenced` — a
  KB stamped before this release holds none until its stamped documents are rewritten by hand or
  removed for `update-format` to write whole ([§3](../3-compatibility-contract/index.md)).
- **The build records live in `kb-root/.build/`.** A build writes and commits
  `kb-build-node-pass.yaml`, `kb-build-classification.yaml`, `kb-build-unmarked.yaml` and
  `kb-build-cull.yaml` there, so its stage commits stay within `kb-root/`. Converting a `3.0.0` KB
  moves each record standing at the repository root into `kb-root/.build/` with `git mv`, and
  `update-format` refuses while a record it would move has uncommitted changes (`check:
  dirty-paths`, naming it) ([§5](../5-build-behaviour/index.md), [§6](../6-build-state-ledger-and-monitoring/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`add-claim`, `add-analysis`, `add-experiment`, `add-work` and `add-axiom` add one node each.**
  Each writes its record with no edges and mints its node's id, for every kind, so no later entry of
  the same batch names it ([page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md));
  a claim's, an analysis's or an experiment's statement marker goes at the end of the line
  `document`, `line` and `snippet` name, with no space, and a line not containing the snippet is refused, naming the nearby lines that do.
  `add-work` takes the work's citation key as its `label`, and `add-axiom` a `label` of any one line
  of text; a work's or an axiom's label is unique within its kind. An add of a title its kind
  already holds — or of a label its kind already holds — is refused, `node exists`, naming that
  node; nothing is adopted, so a re-issued add is refused. `add-axiom` marks the one unmarked
  heading of its text in `axioms.md` or appends its marked heading, reporting which under
  `heading`, and refuses more than one unmarked heading of its text, naming their lines. An add
  whose summary entry shifts a standing entry's repeat suffix re-points every link into that entry
  ([page `add-axiom`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-axiom.md), [§8](../8-maintenance-subcommands/index.md)).
- **Every node id is minted, and a work's or an axiom's label is a record field.** A work record
  carries `label`, its citation key, and an axiom record `label`, any one line, after `id`; each
  axiom heading in `axioms.md` ends in its axiom's marker ([page §1.1](../../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)). Converting a `3.0.0` KB mints
  each work's and axiom's id from its `3.0.0` id alone, the old key or label its `label`, renames
  every place the old id stood, marks each axiom heading naming a record and reports each naming
  none; `update-format`'s result lists each conversion under `ids` (`from`, `to`)
  ([page `update-format`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)).
- **`edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work` and `edit-axiom` change a node's
  fields.** Each replaces the fields its entry gives — a title, a rigor, a rationale or a status,
  and a work's or an axiom's label, as its kind carries — of the node its `id` names, reporting the
  `id`; an entry whose values already stand, its judged hash among them, is `unchanged`, and a
  rigor given marks the node manually scored. A title or a label another node of
  the kind carries is refused, naming it, and an id of another kind is refused, naming the edit op
  for its kind. A title change gives each citation of the node in the naming form the new title;
  a title or a label change re-points every link, a citation, an ordinary link or a reference
  definition, into a heading or summary entry whose anchor it moves, and `edit-axiom` rewrites its
  marked `axioms.md` heading; a title under which a naming-form citation would not read as one link, an
  unbalanced `[` or `]`, is refused, naming the citing `path:line`. A work's label change rewrites
  every reference using the old key as a key — each `data-cites` key, the key-only rendering, the
  reference-list entry's `id="ref-<key>"` and each link into `#ref-<key>` — and a label a span or
  an entry already uses as a key is refused, naming its `path:line`; its result entry lists each
  line it rewrote under `rewritten` ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`add-edge`, `edit-edge` and `remove-edge` write one edge each,** named by `source`, `target` and
  `relation` and reported by them. `add-edge` writes any relation but `demoted` between the kinds it
  joins — a pairing it does not join is refused, `allowed` the relations that do — with a `bearing`
  on the scoring relations, `*pending*` where left out, and a `context`; an edge that stands is
  refused, `edge exists` at `relation`, and one closing a cycle is refused naming it; over a demoted record of the
  edge it writes the add's values and deletes the record. `edit-edge` changes a standing edge's
  `bearing` or `context`, a bearing given marking it manually scored, `unchanged` where the values
  and the judged hash stand. `remove-edge` removes one record, or with `relation: demoted` every demoted record from
  `source` to `target`; an edge no record holds is refused, `no edge` at `relation`, so a re-issued add or removal
  is refused ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`restore-demoted` refuses what it cannot restore and reports what it restored.** An entry no
  demoted record from `source` to `target` answers is refused, `no edge` at `relation` — a re-issued restore, an
  edge standing with no demoted record and a batch's repeat of an entry among them, each formerly
  `unchanged` or a shape refusal — and its result entry carries `restored`, the relations put back
  ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`remove-node` removes a node of any kind with every record naming it.** It takes `id` alone and
  removes the node's record, its statement marker or an axiom's `axioms.md` heading, every demoted
  record naming it and every edge naming it at either end, its result entry listing those edges
  under `removed-edges` as `edges` rows; a dry run previews them. A citation of the node, named
  `path:line`, a work's label carried by a citation span no reference-list entry has the key of,
  named `path:line`, or its mention in a surviving record's title, rationale or edge context
  refuses the removal, an `edit-<kind>` or `edit-edge` entry earlier in the same batch clearing the
  latter. A removal that shifts another summary entry's or `axioms.md` heading's repeat suffix
  re-points every link into it; an `id` no node carries is refused, `no node`, so a re-issued removal is refused ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`move-node` moves a claim's, an analysis's or an experiment's statement marker.** It places the
  marker at the end of the line `document`, `line` and `snippet` name, refused as an add refuses
  that line, takes it from wherever else it stood, and re-points every citation of the node to its
  new host and section without touching the citing words; a move to where the marker stands is
  `unchanged`, and a work, an axiom and a node with no marker are refused ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`add-citation` turns written text into a citation of a node.** It replaces the one span of the
  line `document`, `line` and `snippet` name with a link to the node's leaf and section anchor —
  an axiom's heading, a work's root summary entry — written relative to the document, the id its
  link title: a quote checked verbatim against the node's statement, a paraphrase whose `quote` is,
  or, where `snippet` is the id, the naming form, the one form a work or an axiom takes. A snippet
  the line lacks or holds twice, a quote absent from the statement, a node with no marker and an
  unbalanced `[` or `]` are refused; a
  citation written into a heading re-points every link, a citation or not, into a heading whose
  anchor it moves; its
  result entry carries `document`, `line`, `id` and `link` ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`citations-of` lists where prose cites a node,** found by the id in each link title, as `path`,
  `line` and `text` ordered by path, then line; its answer is complete, `count` and `results` with
  no paging, and an id no node carries is refused, `no node` ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/index.md)).
- **`show` refuses an unknown id as every op does,** `no node <id>` with its remedy ([§8](../8-maintenance-subcommands/show.md)).
- **A refusal over MCP that names a document keeps its `line` and `column` there;** one naming no
  `path` still carries neither ([§11](../11-the-mcp-server.md)).
- **`find` finds nodes of every kind by filters, and returns its whole answer.** Its `<query>`
  argument is gone: `--kind` (repeatable), `--title` (a case-insensitive substring of the title
  alone, no longer the anchor), `--label` (a case-insensitive substring of a work's or an axiom's
  label), `--path`, `--min`, `--max`, `--pending` and `--min-dependents` narrow it; a row is `id`,
  `kind`, `title`, `label` (null for a claim, an analysis or an experiment), `host`, `strength`
  (`*pending*` where pending) and `dependents`, with no `build_band`. `--sort id|strength|dependents` and `--order
  ascending|descending` order it, and `--limit N` keeps the first N, or for a negative N the last
  N, allowed only beside `--sort` and never 0; `--offset` and the result's `offset` and `truncated`
  are gone ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/find.md)).
- **`show` names where a node is stated and summarised.** Its record carries `host`, `host_anchor`
  and `summary` (`<page>#<anchor>`) in place of `canonical_path` and `canonical_anchor`, a claim's
  carries no `citation_count`, an analysis's and a work's carry their `rationale`, and a work's and
  an axiom's their `label` ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/show.md)).
- **`stats` carries no `citation_edges`, and `subtree_aggregates` is per volume:** each volume's
  `claims` and `experiments`, in place of one count of index pages. Its help names the `find`
  question behind each ranked section ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/stats.md)).
- **`edges` lists one node's edges, and returns its whole answer.** `--id` with `--direction
  in|out|both` (`both` by default) keeps the edges at a node, an `--id` no node carries refused,
  `no node`; a row is `source`, `target`,
  `relation`, `bearing`, `context`, `manually_scored` and `replaced_relation` — the relation a
  `demoted` record replaced, null on every other row — with no `walked`, and `remove-node`'s
  `removed-edges` rows follow it; `--limit`, `--offset` and the result's `offset` and `truncated`
  are gone ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/edges.md)).
- **`trace` follows the scoring edges from a node, transitively.** `trace <id> --direction in|out`
  lists every `supports`, `founds`, `lifts` and `refutes` edge that bears on the node's strength or
  that its strength reaches, each an `edges` row with its `depth`, its source's `strength` and its
  `contribution`, the edge's input to its target's strength as the derivation computes it.
  `--sort depth|contribution`, `--order` and `--limit` order and cut it as `find`'s do ([§7](../7-tool-result-contract-and-exit-codes/index.md), [§8](../8-maintenance-subcommands/trace.md)).
- **Help, schemas and results name an argument as the surface takes it**: `--name` at the command
  line, `name` over MCP, set off in backticks inside prose; over MCP a kbase command line in a
  result or remedy is the tool call. A command line's flags come in name order, so `resume` reads
  `kbase build --no-inference --resume …`; a "no node" remedy reads "list the nodes that stand with
  `kbase find`, correct `<id>`, then re-run" ([§7](../7-tool-result-contract-and-exit-codes/item.md)).
  A remedy naming a write batch reads at the command line as its values document piped in,
  `printf 'entry: []\n' | kbase write`, and over MCP as the `write` tool's call, its entries
  included.
- **A value set records the hash of what it was judged against.** A rigor or a bearing an add or an
  edit is given — `*pending*` included — and every value `score` lands record the record's
  `judged-hash` over the node's statement, or an edge's two ends' statements, with the ask
  template's digest; a value set otherwise keeps it ([§8](../8-maintenance-subcommands/edit-kind.md)).
- **`score` asks only what is dirty.** A run walks each item whose judged hash differs from the
  one its record carries, a never-judged value included; a second run over a KB it scored asks
  nothing. `--pass` is `rigor`, `bearing` or `all` (was `both`); `--node <id>` and `--edge
  <source>,<target>,<relation>` (over MCP a mapping) score one item, dirty or not, and are refused
  beside `--pass`, on a value no pass asks, and on a manually scored item without
  `--rescore-manual`, which is now a bool: given, a full run also re-judges every manually scored
  value of its pass. `--limit` is gone — the scope is the budget — and so is the `bounded` outcome
  ([§8](../8-maintenance-subcommands/score/index.md)).
- **`score` lands a value only against the text it was asked about.** A value whose item's text
  was edited after its call was composed — a statement edited by hand between a stop and its
  `--resume` — lands nothing: its ledger line is `failed`, reason "its text was edited after it was
  asked; the value is discarded and a resume asks it again", and the resume asks it again. A value
  whose node or edge is no longer in the KB lands nothing and is `skipped`, reason "the node or edge
  is no longer in the KB; its value is discarded and a resume does not ask it again": it is no
  failure, and leaves nothing to resume. Each ledger line records `judged`,
  the hash its call was composed under ([§8](../8-maintenance-subcommands/score/landing.md)).
- **`score --resume` is a bool.** It continues the repository's most recent score run with the
  scope its `run.yaml` records, `rescore_manual` among it, and takes no scope argument beside it;
  it is refused where that run finished or a build has started since, remedy a fresh `score`. A
  result's `resume` is that call, naming no run id and no scope ([§7](../7-tool-result-contract-and-exit-codes/item.md), [§8](../8-maintenance-subcommands/score/the-closing-record.md)).
- **`score --dry-run` lists what a run would ask.** Its document carries `ask`, the items a run
  would ask, and `skip`, the dirty manually scored values it would skip, counted under
  `counts.remaining` and `counts.skipped` ([§7](../7-tool-result-contract-and-exit-codes/item.md)).
- **`score` lands through the edit ops.** Its values land as one mixed batch of `edit-claim`,
  `edit-analysis`, `edit-experiment` and `edit-edge` entries with the mark `false`, followed by
  the derivation, so the derived files are current whenever the run exits; the run no longer
  refreshes after its last write, and a node's text it asks about is the statement `add-citation`
  quotes, its markers and blockquote prefixes removed ([§8](../8-maintenance-subcommands/score/landing.md)).
- **The `score` tool reports a run still starting after 5 s** as `done` with `state: starting`,
  `state-dir`, `pid` and `wait`, for `status` to follow, in place of a failure after 40 s ([§11](../11-the-mcp-server.md)).
- **The `build` tool reports a run still starting after 5 s** the same way, in place of a failure
  after 40 s ([§11](../11-the-mcp-server.md)).
- **A fresh `build` is refused wherever `kb-root/` exists**, a spine-only or skeleton one included,
  with `init`'s refusal: `check: kb-root`, no remedy. A fresh build over a trail whose `kb-root/`
  was removed starts a trail of its own, so removing `kb-root/` is how to start over; a
  `--resume` with nothing to resume while `kb-root/` stands names that remedy, `check: kb-root`.
  `status`, `build-trail` and a resume read no trail a later removal of `kb-root/` ends; with
  `kb-root/` absent, `status` reports `none` and `build-trail` no boundary ([§5](../5-build-behaviour/starting-over.md), [§8](../8-maintenance-subcommands/init.md)).
- **`build-trail` replaces the `rewind-build` tool and the `kbase://build-trail` resource.** It is
  a subcommand and a tool, takes no target, and gives each boundary its own command — `git reset
  --keep <commit>` then the build resumed with the flags its last run was launched with, the bound
  excepted — and `started-at` the reset alone; `git reset
  --keep` refuses rather than discarding uncommitted changes ([§11](../11-the-mcp-server.md)).
- **`status` lists each stage done or not done, without commits**: `stages` holds `done`,
  `not-done` and `rewind`, the line naming `build-trail` as where each commit is ([§6](../6-build-state-ledger-and-monitoring/kbase-status/index.md)).
- **No resource is subscribable.** `kbase://build` and its end-of-run notification are gone, and
  `initialize` declares no `subscribe`: a session polls `status`, or runs the `wait` command `build`
  and `score` return ([§11](../11-the-mcp-server.md)).
- **The KB a build writes holds every title unique within its kind**, a repeated title taking a
  counter, ` (2)` on; the build's writes go through `add-claim`, `add-work` and `remove-node`, each
  followed by the derivation, and a link wrapped around a cross-reference is unwrapped to the
  cross-reference ([§5](../5-build-behaviour/output-and-placement.md)).
- **`verify`'s checks are named in kebab-case**: `dead-link`, `unknown-id`, `index-freshness`,
  `statement-marker`, `axiom-heading` and `register-freshness`, beside `bare-id`, `frontmatter` and
  the `demoted` finding; `add-citation`'s and `move-node`'s refusal of a node with no statement
  marker is `check: statement-marker` ([page §2.8](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md), [§8](../8-maintenance-subcommands/verify.md)).
- **`verify` checks every link's fragment.** A fragment into a leaf, `axioms.md` or any other
  Markdown document under `kb-root/`, a bare `#fragment` into its own document included, is a
  `dead-link` fault where it names none of that document's heading anchors or element ids, as a
  fragment into a summary page already was where it names no entry; the build's gate stage runs the
  same check ([page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md), [§8](../8-maintenance-subcommands/verify.md)).
- **A stale summary page's remedy is the empty write batch,** `entry: []`, which re-derives every
  page, in place of `kbase update-format`, which stays its remedy over a KB in an older format,
  where the batch is refused; a `bare-id` fault's remedy is a write batch whose `add-citation`
  entry cites the node, the id as `snippet` ([page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md), [§8](../8-maintenance-subcommands/verify.md)).
- **`export-latex --volume` names a volume, a top-level directory of `kb-root/` holding an
  `index.md`.** One that is not a volume is a usage refusal whose `allowed` lists every volume; one
  naming a volume the entry point does not list is refused as before. The report lists every form
  the export simplified ([page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md), [§8](../8-maintenance-subcommands/export-latex/index.md)).
- **A provider name is case-insensitive.** `--provider`, a model reference's provider part,
  `auth add`, `auth remove` and each `providers.toml` table name are folded to lowercase, and every
  output names a provider so. A repository entry now overrides the global one of the same name in
  any case, where a case-only difference across layers used to refuse the load; two tables of one
  file differing only by case still refuse it ([page §2.7](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md), [§9](../9-configuration/merging.md)).
- **`auth remove` of a provider with no stored key is refused**, its remedy `kbase auth list`, where
  it was `unchanged` ([page §3.7](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/auth-add-auth-list-auth-remove.md), [§9](../9-configuration/provider-commands.md)).
- **`probe` is a read-only tool**, taking the command's arguments; its MCP note makes each call, a
  paid model call, one the user directs ([page §3.7](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/probe.md), [§8](../8-maintenance-subcommands/probe.md)).
- **`show-config`'s layer keys are kebab-case:** `providers-exists` and `config-exists`, where they
  were `providersExists` and `configExists` ([§7](../7-tool-result-contract-and-exit-codes/item.md)).
- **The `[models]` slots are named roles everywhere**, `kbase://model-hints` included, where the
  guidance said categories ([page §2.1](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.1-naming.md)).
- **The instructions carry eight items**: where the project is, the `kbase` skill, the `.index/`
  rule, the session rule — which now says what a score run judges and that a work's rigor and a
  `founds` bearing stay pending until the user gives one — how an edge reads, which `kbase --help`
  also states, `commit` on every write batch, the sheets as pictures for people, and a key command
  run as kbase prints it ([page §4.1](../../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md), [§11](../11-the-mcp-server.md)). `AGENTS.md`'s maintained block carries the new
  session rule once `update-format` rewrites it.
- **A server that locates no project serves as `--project-dir-required` does**: `project-dir` is
  required in every schema, and the instructions say how a session gets the path, where such a
  server used to leave `project-dir` optional and refuse a call omitting it ([page §4.3](../../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md), [§11](../11-the-mcp-server.md)).
- **`kbase --help` and `kbase mcp --help` name the CLI-only commands**, `auth`, `wait` and `mcp`
  ([page §4.4](../../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md)); the root help's overview names the commands as they now stand, `score`,
  `wait`, `build-trail`, `init`, `export-latex` and `probe` among them.

Contents:

- [§0.9.0 — Messages is the default chat protocol](0.9.0-messages-is-the-default-chat-protocol.md)
- [§0.8.0 — the rigor ceiling is `1.00`](0.8.0-the-rigor-ceiling-is-1-00.md)
- [§0.8.0 — a scoring charge cites line numbers](0.8.0-a-scoring-charge-cites-line-numbers.md)
