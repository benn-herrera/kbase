# Maintaining a KB

How to change, query, score and verify a living KB, how its claim-graph sheets are drawn, and how results and exit codes read. Back to the [README](../README.md).

A KB created empty by `kbase init` ([Building a KB](building.md#creating-an-empty-kb)) and one distilled by `kbase build` are maintained the same
way — the subcommands below operate on any living `kb-root/`. Over a created KB `kbase build`
stays refused forever (`check: kb-root`; `init`'s birth commit is a plain subject, not a
`kb-build:` stage); content joins through the write ops.

After a build, the commands operate on the living KB:
- the write ops, each an entry of a write batch: `add-claim`, `add-analysis`, `add-experiment`,
  `add-work` and `add-axiom` add one node each; `edit-claim`, `edit-analysis`, `edit-experiment`,
  `edit-work` and `edit-axiom` change a node's fields; `remove-node` and `move-node` remove a node
  or move its statement marker; `add-edge`, `edit-edge`, `remove-edge` and `restore-demoted` write
  edges; `add-citation` turns written text into a citation of a node;
- the queries, which read and write nothing: `find`, `show`, `edges`, `trace`, `citations-of` and
  `stats`;
- `verify`, which checks the KB; `update-format`, which converts and re-derives it; and
  `export-latex`, which writes a LaTeX project from the KB through pandoc;
- `score`, which asks the configured scoring model for the faults each item's text commits, one item
  at a time, and prices them into the KB's rigor and bearing values.

Run `kbase --help` for the list and `kbase <command> --help` for each one. The contract for these subcommands is
[SPEC §8](../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md).

- **Write batches.** `kbase write` takes an ordered batch as YAML (JSON accepted) on stdin or in
  the file `--values` names: entries under `entry`, each naming its op under `operation`, with
  `dry-run` and `commit` beside them. Each op's own subcommand takes a batch of that op alone, its
  entries naming no operation. Every entry's shape is checked first; the entries then apply in
  order, each against the records the earlier ones left, and the batch stops at the first refusal.
  A batch is all or nothing, its derived summary pages and sheets included. `--dry-run` reports the
  result and writes nothing; an empty batch (`printf 'entry: []\n' | kbase write`) re-derives the
  summary pages and sheets after leaf edits.

  ```yaml
  entry:
    - operation: add-claim
      title: Bounded sequences have convergent subsequences
      rationale: Bolzano–Weierstrass, as stated in the section's opening paragraph
      document: paper/section-2.md
      line: 41
      snippet: bounded sequence
  ```

  An add mints its node's id, known only from the result, so a node and the edges that name it go
  in two batches. An edit whose values already stand reports `unchanged`; a re-issued add or
  removal is refused, because the node exists or no longer does. `--commit` makes git the batch's
  undo: it commits the whole of `kb-root/` before and after the write, so reverting the batch's
  commit undoes exactly the batch. Without it nothing is committed, and only commits of your own
  undo the batch.
- **Scores.** A node's own value is `rigor`, a scoring edge's is `bearing`, and a claim's or an
  analysis's derived value is `strength`. The edit ops record a judgement: `edit-claim`,
  `edit-analysis`, `edit-experiment` and `edit-work` write a node's `rigor`, and `edit-edge` an
  existing scoring edge's `bearing`; only `add-edge` creates an edge. A value given by hand,
  `*pending*` included, marks the node or edge manually scored, which `score` leaves alone unless
  run with `--rescore-manual`; leave a value out of an add and `score` judges it. A claim or
  analysis with no `rigor` reads `*pending*` — unscored, not low. Its derivation pends with it
  unless a lift stands over it: a node corroborated and never judged publishes the lift's product,
  while a refutation alone leaves its strength unknown. A `bearing` of `*pending*` leaves its edge
  out of the derivation until it is judged.
- **Removal.** `remove-edge` removes one edge record, or with `relation: demoted` every demoted
  record from `source` to `target`. `remove-node` removes a node of any kind with every record
  naming it, every edge at either end included; it refuses while a citation of the node, or a
  surviving record's title, rationale or edge context, still names it. Run it with `--dry-run`
  first to see the edges it takes.
- `kbase verify` checks links, records, statement markers and summary pages under `kb-root/`; a
  node id standing bare in prose is a `bare-id` fault, and a citation is a Markdown link whose title
  is the cited node's id (`add-citation` writes one).
- The list queries `find`, `edges`, `trace` and `citations-of` return their whole answer, with
  `count` beside `results`; none pages. `find` and `trace` take `--limit` only beside an explicit
  `--sort`, and a negative `--limit` keeps the last N. `find` narrows by `--kind`, `--title`,
  `--label`, `--path`, `--min`, `--max`, `--pending` and `--min-dependents`; `--min 0` leaves out
  pending nodes.
- `show` returns a node's record: its `host`, `host_anchor` and `summary` place, and its `rigor` —
  `strength` too on a claim and an analysis. `edges` lists edges, at one node with `--id` and
  `--direction in|out|both`; each row carries `source`, `target`, `relation`, `bearing`, `context`,
  `manually_scored` and `replaced_relation`. `trace <id> --direction in|out` walks the scoring edges
  transitively, each row adding its `depth`, source `strength` and `contribution`.
- A write batch and `update-format` commit only when given `--commit`. Where the `.git` beside `kb-root/`
  is not the one git reads as the worktree root, every command that runs git over the KB is refused
  (`check: worktree`).

**Scoring.** `kbase score` asks the configured scoring model — `models.scoring` — one item at a
time, and asks it for the
**faults the text commits**, not for a number: a list of charge lines, each a fault id and the span
of the item's own text that shows it, or `none`. kbase prices the list with the
pass's table. When the walk ends — done, cancelled or failed — the answered values land in one
write through the edit ops, with the manually-scored mark false. The `rigor` pass covers every
claim, analysis and experiment; the `bearing` pass every `supports`, `lifts` and `refutes` edge.
A full run asks only what is dirty: an item whose text, or whose ask template, changed since its
value was judged, and any value never judged. A second run over a KB it scored asks nothing. A value
a person set is manually scored, and a full run skips it; `--rescore-manual` re-judges those too,
replacing the person's value. `--pass rigor|bearing|all` (default `all`) scopes a run to one set,
and `--node <id>` or `--edge <source>,<target>,<relation>` scores one item, dirty or not; neither
takes `--pass`, and there is no item limit — the scope is the budget. Two values are never asked,
both because the document they rest on lies outside the corpus: a work's rigor, and a `founds`
edge's bearing. Judge each by hand, with the document in hand, through `edit-work` and `edit-edge`;
it stays pending until you give one. The two ask templates,
the ids and the weights are stated once, in
[RIGOR_AND_BEARING_SCORING_SCHEME.md](../PROJECT_DOCS/SPEC/scoring/rigor-and-bearing/entry-point.md); the ledger is where a
value can be checked against its charges.

Every attempt is recorded under `<state-dir>/score/<run>/`: `run.yaml` names the run, the set it walks
(one pass, or `all`), what answered (`backend`: `provider`), the model and the digest
of the ask its judgements answer; `ledger.jsonl` holds a line per attempt with its own item's pass, the
faults it recorded, the reply where it parsed, the
value it prices to, the model and the prompt's digest. `--resume` continues the repository's most
recent score run with the scope its `run.yaml` records, without re-asking what it already landed, and
takes no scope argument beside it; it is refused where that run finished or a build has started since,
and a fresh `score` then asks only what is dirty. A value lands only against the text it was asked
about: one whose item was edited after its call was composed lands nothing and a resume asks it again,
and one whose node or edge has left the KB is `skipped`. Because a
value is a projection of the line's ids, a resume **re-prices** a run when the table has moved since:
it lands the values that moved through the same write path and records them in `repriced.yaml`, with
no model asked. A resume under a changed ask — a run older than kbase's current instructions —
continues under the current ask: the header keeps the digest the run began under, and nothing refuses
and nothing asks you to choose. The result carries the `resume` call whenever an item failed or was
not reached. A failed run lists what failed under `failures` — an item and the reason its ledger
line recorded, or a refresh's own error — and a cancelled one names the item it stopped before. The
reason names what ended the item's call: `provider-refused` where the endpoint answered with a status
that is not success, its status and its own words beside it, and `transport-failure` where the
connection, the name, the certificate or the stream failed and no answer was had.
`--dry-run` lists under `ask` what a run would ask and under `skip` the dirty manually scored values
it would skip; it asks nothing, resolves no role and writes no KB. A run refreshes the derived files
once at its start, holding the KB write lock; the derivation follows the landing, so the derived
files are current whenever the run exits. A KB in an older format is refused, dry run or not
(`check: index`, remedy `kbase update-format`).

`--scoring-model` names the model for one run: a `<provider>/<model>` reference, which supplies the
role where `models.scoring` names none and overrides it where it does. Every ask is one chat call
on that endpoint, re-issued identically up to the transport attempt count while it does not complete,
pausing 5 s then 30 s between attempts — as long as the item's deadline covers the pause: a pause it
cannot cover is not waited out, and the item fails at once naming the attempts it made. A status that
is not retryable — any non-2xx other than 429
or a 5xx — fails the item at once. Every ask of a run carries the run's id as its
`prompt_cache_key`, so a run's asks reach one provider and share one cache entry.
A run asks a model for minutes: at a terminal `kbase score` blocks until the walk ends, and as an
[MCP](mcp.md#using-kbase-from-an-agent-harness-mcp) tool it starts a detached child and returns with
the run's identity, `kbase wait` reporting the outcome. `--ask-concurrency` and `--timeout` (one
item's call, default 30 s) bound the asks.

**Locks.** A write batch and `update-format` hold the KB write lock for the whole run.
A writer that cannot take it within 30 s gets `retry`. While a build or a score run holds the
operation lock, these are refused at once (`check: lock`). A build waits out a write in progress for
up to 30 s. `kbase score` takes the write lock around its start refresh and its one landing, never
across the run. A resume restores every path under `kb-root/` to the last boundary, so edits made
while a build is interrupted are lost unless carried in git. `verify`, `export-latex` and the
queries take no lock. Locks are advisory and Unix only.

**Demoted edges.** When the build breaks a dependency cycle, it cuts one edge of the cycle and
records it as a `demoted` edge that keeps the edge it replaced whole, relation and bearing
included. `verify` lists each under `findings` with outcome `done`, `stats` counts them as
`demoted_edges`, and the sheets draw them as cuts. `restore-demoted` puts back the edge a demoted
edge replaced and is refused (`check: dependency-cycle`) where that would close a cycle;
`remove-edge` with `relation: demoted` deletes the demoted record instead, leaving the cut. Each
names it as its `edges` row does. `add-edge` is refused the same way for a scoring edge that closes
a cycle.

**Format migration.** A KB's format is `4.0.0`, stamped `kb-format` in the one-key file
`.index/format.yaml`, and its documents carry no frontmatter at all: its nodes and edges are kbase's
records in `.index/nodes.yaml` and `.index/edges.yaml`, and its `kb-summary.md` summary pages are
generated from them. A statement marker is `<!-- kbase-id: <id> -->`. A KB in an older form —
`3.0.0`, `2.0.x` or `1.0.0` — is converted in memory when it is loaded and answers every read as
converted; the result document says so in a `migration` record, whose `warnings` list what the
conversion dropped or changed, and stderr says the same. While the conversion is unsaved, every
write but `update-format` — a write batch, `score`, and `build` resuming over the KB — is refused
(`check: index`, remedy `kbase update-format`). `update-format` saves it, and refuses a `kb-root/`
with uncommitted changes (`check: dirty-paths`, naming each path); `--commit` commits the
conversion as one commit. Whether or not the format changed, every `update-format` re-derives the
summary pages and sheets and rewrites the block kbase maintains in `AGENTS.md`, `CONVENTIONS.md` and
`README.md`, between its `vvv-kbase` and `^^^-kbase` marker lines. A document without that pair is
left as it stands and listed under `unfenced`. A KB stamped with a newer major or minor is refused
(`check: kb-format`; update kbase), and so is a conversion that cannot carry a file's content,
naming the file and line (`check: conversion`). A missing record file is refused (`check: index`;
restore it from git), and any other load fault is refused (`check: metadata`), naming each fault
with its file and record; `update-format` repairs none, and recovery is in git. A run that cannot take the KB write
lock in 30 s answers `retry`, and one started while a build or a score run holds the operation lock
is refused (`check: lock`). `status` and `cancel` do not load the KB.

## Claim-graph sheets

Every write batch and `update-format` draw the claim graph as SVG: `kb-root/claim-graph.svg`, the whole
KB with every volume as a cluster; and, where two or more volumes hold nodes,
`kb-root/claim-graph-digest.svg`, one box per volume with the edges between volumes counted, and
`kb-root/<volume>/claim-graph.svg` for each volume (its claims plus every claim one edge away).
Every arrow runs from the supporter to the node it supports: a claim or an experiment `supports` a
claim or an analysis; a work or an axiom `founds` one; an analysis or an experiment `lifts` one; and
a claim, an analysis or an experiment `refutes` one. Each relation has one stroke of its own, and
the cut a cycle-breaking demotion made is drawn dotted. Nodes are styled by kind and filled by
strength band, a pending one grey, and every node links to its place: a claim's or a work's entry on
its summary page, an
analysis's or an experiment's section in the leaf that hosts it, an axiom's heading in `axioms.md`.

The sheets need Graphviz's `dot` on `PATH`, any version. Without it, existing sheets are left as
they are, the write writes a placeholder `kb-root/claim-graph.svg` only where none exists and
reports `sheet: not-drawn`. Sheets are rewritten only when their content
changes. A digest or volume sheet the KB no longer calls for is removed by the next render and
reported under `removed`. The drawing is specified in [SPEC §3](../PROJECT_DOCS/SPEC/3-compatibility-contract/index.md).

## Output and exit codes

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
