## 8. Maintenance subcommands

Every subcommand below is reachable by an external agent harness as an MCP tool: a tool of its own, an operation of the `query` tool, or, for a write op, an entry of the `write` tool's batch (§11). `score` starts a detached child that makes one model call per item and returns at once with the run's identity, so the turn is not held for the run's duration (§11). The rules in this section govern each.

### Write batches

Every write goes through a batch, on every interface. An entry is one item of a batch.

**Shape.** A batch is an ordered list of entries under `entry`. Each entry names its op under `operation` and has that op's keys; a batch may mix ops. `dry-run` and `commit` sit beside `entry` as batch options. An empty batch is valid: it writes no record and rebuilds the derived views ([Writes leave nothing stale](#writes-leave-nothing-stale)), which brings them up to date after an author's edits to leaves.

**Interfaces.** Over MCP, the `write` tool takes a batch (§11). At the command line, `kbase write` takes a mixed batch, each entry naming its op, with `--dry-run` and `--commit`. Each write op also has its own subcommand (`kbase add-edge`, …), which takes a batch of that op alone with the same options; its entries have no `operation`, since the subcommand names the op.

**Order.** Entries apply in the order given. Each entry is checked against the records as the earlier entries left them. kbase never reorders a batch; the caller orders it so that each entry is valid at its position.

**No reference between entries.** No entry names an id that another entry of the same batch mints. Every add mints its node's id, for every kind ([§10](#five-node-types)), and the result gives each minted id beside the entry that minted it. Adding nodes and their edges takes two batches: the first adds the nodes, the second the edges. Between the two the KB is consistent, since a node with no edges is a valid node.

**Two phases.** Before anything applies, every entry's shape is checked: an unknown op, an unknown or missing key, a value's type, and a value outside a fixed set (`relation`, `status`, …). Every shape error is reported at once. Then the entries apply in order, each checked in full against the records as they are at that point, and the batch stops at the first refusal. An entry that would leave the records invalid is refused even if a later entry would repair it; the refusal's remedy names the order that works. No separate end-of-batch check exists. Each refusal item has the keys of [§7](#item), with `entry` the entry's 1-based position in the batch.

**All or nothing.** A batch that is refused or fails writes nothing. The rebuild of the derived views is part of the batch, so the records are never written while their derived views are stale. A crash between the writes of two files is not atomic on disk; git recovers that case ([Git holds the recoverable record](#git-holds-the-recoverable-record)).

**Refused before it applies.** A batch is refused, whatever its entries, over:
- a KB whose load fails (`check: metadata`, [§10](#the-load-every-fault-refused));
- a KB in an older format that is not yet converted (`check: index`, remedy `kbase update-format`; [§10](#one-version-read-and-written)); a dry run is refused too;
- a lock ([One write lock](#one-write-lock));
- a `kb-root/CLAUDE.md` that is not the redirect (`check: agents-redirect`, [§3](#stamped-documents)).

**Dry run.** `dry-run: true` does everything but write. Its result is identical to a real run's, refusals and removed edges included, and says `dry-run: true`.

**Result.** The result's keys are the `write` row of [§7](#item). An entry's outcome is `done` or `unchanged`. The batch outcome is `unchanged` where every entry is `unchanged` and the rebuild of the derived views wrote nothing, and `done` otherwise.

#### Values input

A batch is YAML (JSON accepted), on stdin or in the file `--values` names. At the command line, `kbase write`'s entries each name their op under `operation`; an op's own subcommand's entries name none. `kbase <op> --help` describes the op.

Each op's key set is closed and stated with the op below. A refusal names the key and its position in the document. It lists as `allowed` the fixed set the key or value must come from, where there is one; an unknown key lists the op's keys, and a required key left out is refused by name.

**Naming.**
- `id` names a node wherever the node is the subject: `edit-<kind>`, `remove-node`, `move-node`, `add-citation`'s cited node, and the queries `show`, `citations-of`, `edges` and `trace`. `score` names its item `node` or `edge`, since it takes either.
- An edge is named by `source`, `target` and `relation`, everywhere; no edge end is keyed `id`. Every edge row a query returns has the op keys `source`, `target`, `relation`, `bearing` and `context`, so a row's values name the edge for any edge op.
- A node is named by its id only; the id's prefix tells its kind. A `label` or a `title` is a field, and `find` locates a node by either ([§8](#find)).

**Values.**
- A `rigor` or a `bearing` is a number in `[0, 1]` or the string `"*pending*"`, quoted in YAML, where a bare `*` begins an alias. A value outside that domain, a rigor above `1.0` included, is refused at its key, the `detail` naming the domain.
- A prose value (`title`, `rationale`, `context`) containing a line break (CR or LF), U+0008 or U+000C is refused. Any other whitespace run collapses to one space, and the ends are trimmed. The `unchanged` test compares the collapsed value.

#### Idempotence

Every op that names something existing first looks it up, and is refused where the lookup fails, with one error everywhere: `no node <id>` or `no edge <relation> <source> → <target>`. The rule covers every op naming an existing node, edge, demoted record, provider entry or stored key: the edits, `move-node`, the removals, `restore-demoted` (which looks up a demoted record), `add-citation`'s cited node, `show`, `edges`, `trace`, `citations-of`, `score`'s `node` and `edge`, `configure-providers --remove` and `auth remove` ([§8](#show), [§8](#edges), [§8](#trace), [§8](#citations-of), [§8](#score), [§9](#provider-commands)).
- A lookup matches exactly; there is no near-miss matching.
- A removal is a lookup that can fail. Nothing records what was removed, so a repeated removal and a removal of something that never existed cannot be told apart; both are refused.
- The remedy of a refusal points at the query that lists what exists: `find` for nodes, `edges` for edges.

An add looks up first too, and is refused where the item is found (`node exists: <id>` or `edge exists: <relation> <source> → <target>`): create fails if the item exists. Nothing is upserted. Where an edge exists with other values, the remedy is `edit-edge`.

An edit that changes nothing, because every value given is already the current one, is `unchanged`: the item exists, and the caller asked for a state the KB is already in. Each batch entry reports its own outcome, so an `unchanged` entry is never hidden in a `done` batch.

So for `add-<kind>` ([§8](#add-kind)), `edit-<kind>` ([§8](#edit-kind)), the edge writes ([§8](#edge-ops)), `restore-demoted` ([§8](#restore-demoted)), `remove-node` ([§8](#remove-node)), `move-node` ([§8](#move-node)) and `add-citation` ([§8](#add-citation)): a repeated add, removal, restore or citation is refused, and a repeated edit or move is `unchanged`, each leaving `kb-root/` byte-identical. There are two exceptions. `init` creates once: a repeat where a `kb-root/` exists is `refused`, never `unchanged`. `update-format` requires a clean `kb-root/`, so a repeat before its output is committed — by its own `commit` or by hand — is refused `dirty-paths`; over a clean tree it already converted, it reports `unchanged`.

#### Writes leave nothing stale

Every write batch is followed by the rebuild of the derived views (strengths, summary pages and sheets), inside its all-or-nothing; no flag skips it. The rebuild does not repair or convert the KB. The writes of `score` ([§8](#writing-the-results)) and of the build (§5) are followed by it the same way.

#### One write lock

A write batch and `update-format` hold the KB write lock for the whole run: from loading the KB to the result, the rebuild of the derived views included. `init` holds it from before `kb-root/` exists — the lock sits on the directory that contains `kb-root/`, so it is available at creation — through the scaffold, the final rebuild and the initial commit to the result. `score` takes it around writing its results — every value the run produced, after the item loop — and around its rebuild at the start, never across the run ([§8](#writing-the-results)).

A writer that cannot take it within 30 s gets `retry` with `check: lock`, `path` the repository root, and the remedy. A write verb probes for a run holding the run lock ([§6](#git-holds-the-recoverable-record)) before it waits on the write lock and again after taking it: where one runs, the verb is `refused` with the `lock` item at once, taking no wait. A build waits out a write in progress, up to the same 30 s, before it writes (`retry` if it cannot). A scoring run's own writes make no run-lock probe, writing as the run that holds it. `verify` and the queries take no lock. On Windows no advisory lock exists (§6): neither the `retry` nor the run-running refusal is available there, and the operator must keep to one writer at a time.

#### Write ops

The write ops are `add-claim`, `add-analysis`, `add-experiment`, `add-work`, `add-axiom`, `edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work`, `edit-axiom`, `remove-node`, `move-node`, `add-edge`, `edit-edge`, `remove-edge`, `restore-demoted` and `add-citation`. Each is an entry of a batch, as above; each result's keys are defined in [§7](#item).

**`commit`.** `commit: true` (`--commit`) makes git the batch's undo. It is never on by default, and no configuration sets it. Every commit it makes covers `kb-root/` only. A committing batch:
1. applies every entry in memory; on any error it returns the error with nothing written and nothing committed;
2. makes the snapshot commits, before writing, each only where its part of `kb-root/` has uncommitted changes and each alone: first the changes outside `.index/`, labelled `kb-write: kb-root/ changes outside .index/ made before this batch`, then the changes inside it, labelled `kb-write: record changes made before this batch`;
3. writes the batch;
4. commits the batch's changes, inside and outside `.index/`, labelled `kb-write: <ops> <ids>`.

A commit's label is its subject line. A label says where the changes are, not who made them: changes left by an earlier batch run without `commit` go into the commit for wherever they lie.
- `<ops>` is the batch's ops, each once, in entry order, joined by `,`; for an empty batch, the command's (`write` for `kbase write`). `<ids>` is the ids its entries entered or name in their `id`, `source` and `target`, each once, in entry order, joined by `,`.
- A snapshot commit that fails, for example on a hook, stops the batch before it writes: outcome `failed`, nothing written, since writing then would mix the batch into uncommitted changes.
- A batch that writes and removes nothing (`unchanged`) makes no batch commit. A batch commit that fails, for example on a hook, leaves the batch written; the result says so under `uncommitted`, with outcome `done`.
- Reverting the batch commit undoes exactly that batch and never the author's work. `commits` lists each commit made: `commit`, `label` and `paths`, the repository-relative paths it contains, a rename as both sides. A batch without `commit` has no safety net.

**`check: worktree`.** kbase runs git over a KB only where git's worktree root is the repository root containing the `.git` beside `kb-root/`. Outside a git worktree (remedy `git init`), where `GIT_DIR` or `GIT_WORK_TREE` redirects git, or where that `.git` is not one git accepts, so that git reads an enclosing repository, every command that runs git over the KB is refused `check: worktree` before it writes: a committing batch, a dry run included, and `update-format`, `init`, `build`, `status`, `cancel`, `wait` and `build-trail`.

### `init`

```
kbase init [--title TEXT]
```

Creates a KB directly — no LaTeX, no build: the equivalent of `git init` for a KB. Run at any depth in a git worktree; it resolves the repository root and works at `kb-root/` in the git worktree root (placement: §3).

**Refusals come before any write** ([§7](#item)):
- a `--title` containing a line break (CR or LF): `check: usage`, `key` the `title` argument as [§7](#item)'s `key` states it; the title is the entry point's one heading line;
- outside a git worktree: `check: worktree`, `path` the start directory, remedy `git init`, which the tool never runs itself;
- a `kb-root/` that exists in any state, an empty one included: `check: kb-root`, no remedy. `init` creates once, so a repeat where a tree exists is `refused`, never `unchanged`. A fresh `build` over an existing `kb-root/` gives the same refusal ([§5](#starting-over)): one item, `path` the `kb-root/`, the same detail. The result of a refusal that found no `kb-root/` has no `kb-root` key; the refusal items name the paths.

Under the KB write lock, held from before the directory exists ([One write lock](#one-write-lock)), `init` writes:
- `entry-point.md`, with the `--title` text, or the repository's directory name, as its only heading, no children and no frontmatter ([§10](#documents-of-a-kb));
- the format version file, `.index/format.yaml` ([§10](#the-record-files)), and the two record files, `.index/nodes.yaml` and `.index/edges.yaml`, containing no record;
- `AGENTS.md`, `CONVENTIONS.md` and the `CLAUDE.md` redirect, each only where absent. `AGENTS.md` states that this KB is authored, not distilled: its author writes what it contains, a claim at a time.

It then rebuilds the derived views, so they are present when the KB is created: the full sheet is always drawn, the placeholder where `dot` is absent (§3); the volume overview diagram and the per-volume sheets are not drawn, since no volume contains a node. `verify` ([§8](#verify)) over a freshly created KB is green.

Finally it commits `kb-root/` with the plain subject `kb-init: knowledge base created`, never a `kb-build:` subject. So the build's checkpoint commits (§5) are absent, a `build --resume` finds nothing to resume, and a fresh `build` is refused over an initialized KB as `init` is: content joins through §8's ops, never a later build. The commit uses the host's git identity and configuration. Git's own refusal (no identity configured) is `failed`, naming git's words, with the written tree left uncommitted for the operator to commit or remove. Result keys: [§7](#item).

### `restore-demoted`

`restore-demoted` puts back exactly the edge each demoted record from `source` to `target` replaced, with its relation, context, bearing and `manually-scored` flag, and deletes the record ([§10](#the-demoted-relation)).

- **Keys**, all required: `source` and `target` (the ids of the demoted record's ends; each a claim, an analysis or an experiment) and `relation`, which must be `demoted`. Any other relation is refused at `relation`.
- **Result**: `source`, `target`, `relation`, and `restored`, the relations put back, in the records' order.
- **Refusals**:
  - an entry with no demoted record from `source` to `target` is refused "no edge", as the edge writes are ([§8](#edge-ops)): no `check`, `key` `relation`, `detail` `no edge demoted <source> → <target>`. So a repeated restore, and a repeat of an entry within one batch, is refused, `entry` the repeat's number;
  - a restore that would close a cycle: `check: dependency-cycle`, `key` `target`, the `detail` naming the cycle in edge direction. The remedy names removing another edge of the cycle and, to leave the edge out of the graph, `remove-edge` with `relation: demoted`, which leaves no record of it.

### `add-<kind>`

`add-claim`, `add-analysis`, `add-experiment`, `add-work` and `add-axiom` each add one node: they mint its id ([§10](#five-node-types)), write its record, and have no edges. A claim's, an analysis's or an experiment's statement marker is placed at the line `document`, `line` and `snippet` name, as [§10](#statements-and-anchors) states; a work and an axiom have no statement marker. A hosted node always has its marker, so placing it is part of the smallest valid add.

| Key | Type | `add-claim` | `add-analysis` | `add-experiment` | `add-work` | `add-axiom` |
|---|---|---|---|---|---|---|
| `title` | string, one line | required | required | required | required | required |
| `rationale` | string, one line, `""` for none | required | required | — | required | — |
| `status` | `run` or `planned` | — | — | required | — | — |
| `rigor` | number in `[0, 1]` or `*pending*` | optional | optional | optional | optional | — |
| `label` | string, one line | — | — | — | required: the citation key | required: any text |
| `document` | kb-root-relative leaf path | required | required | required | — | — |
| `line` | integer ≥ 1 | required | required | required | — | — |
| `snippet` | string | required | required | required | — | — |

An omitted `rigor` is `*pending*`. The `manually-scored` flag and the `judged-hash` an add writes are defined in [§10](#provenance-and-the-judged-hash); an add given a `rigor` hashes the node's statement as its marker places it.

**Result**, per entry: `id`, the minted id; `add-axiom` adds `heading`, `marked` or `appended`.

**Identity.** A node exists where another node of its kind has its `title` anywhere in the KB; a work also exists where another work has its `label`, and an axiom where another axiom has its `label`. Title and label rules are defined in [§10](#five-node-types). An add of a node that exists is refused "node exists" at `title` or `label`, its `detail` `node exists: <id>`. Entries apply in order, so a batch adding one node twice is refused at the repeat.

**Refusals**, each at the key named:
- a missing `document`, `line` or `snippet`: a shape refusal naming the key;
- `document` that does not exist or is not a leaf of the KB ([§10](#documents-of-a-kb)): at `document`, the remedy naming a leaf;
- `line` past the document's end, or a fenced block's opening or closing line: at `line`;
- `snippet`, where the line does not contain it: at `snippet`, the `detail` naming the nearby lines that do ([§10](#statements-and-anchors));
- a `rigor` or `status` outside its domain; a prose value containing a line break ([Values input](#values-input));
- an `add-work` `label` that is not a citation key ([§10](#five-node-types)): at `label`.

**`add-axiom`** writes the axiom's record and gives it its heading in `axioms.md`, `### Axiom <label>: <title><!-- kbase-id: <id> -->` ([§10](#axioms)). The axiom's prose under the heading is the author's.
- An unmarked heading of its text is a `###` heading whose text, markers removed and trimmed, is `Axiom <label>: <title>`. Where exactly one exists, it marks that heading by ending its line, trailing whitespace dropped, in the axiom's marker (`heading: marked`). Otherwise it appends its marked heading (`heading: appended`), creating `axioms.md` where the file is absent. It never leaves two headings of one text.
- More than one unmarked heading of its text is refused: `key` `label`, `path` `axioms.md`, `line` the first, the `detail` naming their lines, since kbase does not guess which one the axiom is.
- It appends its heading after every other, so it shifts no anchor of `axioms.md`.

An add whose summary entry shifts an existing entry's repeat suffix (`-1`, `-2`) — a title of the same slug rendered before it — updates every link into the shifted entry to its new anchor in the same write, as an edit does ([§8](#edit-kind)).

### `edit-<kind>`

`edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work` and `edit-axiom` change the fields of the node an entry's `id` names. A field given replaces that field's value whole; at least one is required.

| Op | `id` (required) | Fields it may change |
|---|---|---|
| `edit-claim` | `clm-…` | `title`, `rigor`, `rationale` |
| `edit-analysis` | `ana-…` | `title`, `rigor`, `rationale` |
| `edit-experiment` | `exp-…` | `title`, `status`, `rigor` |
| `edit-work` | `wrk-…` | `label`, `title`, `rigor`, `rationale` |
| `edit-axiom` | `axm-…` | `label`, `title` |

Fields take the domains of the adds ([§8](#add-kind)). `label` is checked as `add-work` and `add-axiom` check it.

**Result**: `id`; and on an `edit-work` entry that changes `label`, `rewritten`, each line it rewrote a reference on, `path` and `line`. An edit is `unchanged` where it changes no value, no `manually-scored` flag and no `judged-hash`.

**Rigor.** The `manually-scored` flag and the `judged-hash` an edit writes are defined in [§10](#provenance-and-the-judged-hash). An edit given a `rigor` on a claim, an analysis or an experiment hashes the node's statement as the edit leaves it; an edit giving no `rigor` keeps the record's `judged-hash`.

**Links stay live.** A title change, or an axiom's label change, keeps every link to the node live, in the same write:
- `edit-axiom` rewrites every `axioms.md` heading that ends in its marker, keeping the marker, and appends its heading where none exists, creating `axioms.md` where the file is absent.
- A citation of the node in the naming form ([§10](#citations)) gets the new title as its text. Any other link text is a quote or a paraphrase, the author's words, and stays.
- Where the edit moves an anchor — a hosted node's, a work's or an axiom's heading or summary entry, or another entry's repeat suffix shifting with it — every link into the old anchor is updated to the new one. A link updated is an inline link or a reference definition outside code, in any document but the summary pages, which the edit does not rewrite: the rebuild of the derived views renders them. A citation of the node whose destination resolved to the node's link target before the change points at the target after it, written relative to the citing document. Any other link is unchanged.
- Where a rewritten naming-form citation sits in a heading and moves an anchor — that heading's own, or a later same-text heading's `-1`, `-2` suffix — every link whose destination resolves to a moved heading, citation or not, is updated to its new anchor.

An edit writes no document other than those the updates above reach.

**A work's `label` change** rewrites, in the same write, every reference that has the old key as a key, so that none dangles and the export still finds the work's entry. It reads citation spans, reference-list entries and links into them outside code, a blockquote's prefixes read past, in every document but the summary pages:
- each key token of each citation span's `data-cites` (`<span class="citation" data-cites="…">`, the imported LaTeX citations), and the span's text where it is the key-only rendering, the parenthesised key list;
- each reference-list entry's `id="ref-<key>"`, and each link into `#ref-<key>`.

kbase citations need no rewrite, since they name the node by id and the naming form's text is the title. Prose that mentions the key as words, such as a rationale, is the author's and stays.

**Refusals**:
- no field given, or a field the kind does not have: a shape refusal whose `allowed` is the fields the kind's edit may change, neither `id` nor `operation` among them;
- an `id` of another kind: at `id`, the remedy naming the edit op for that kind;
- an `id` no node has: `check: unknown-id` at `id`, `detail` `no node <id>`;
- a `rigor` or `status` outside its domain; a prose value containing a line break ([Values input](#values-input));
- a `title` or a `label` another node of the kind has: at that key, the `detail` naming that node;
- a work `label` that is not a citation key ([§10](#five-node-types)): at `label`;
- a work `label` a citation span or a reference-list entry already uses as a key, since the rewrite would merge two references into one: at `label`, one item per such line, each naming it by `path` and `line`;
- a `title` under which a naming-form citation of the node would not parse, whole, as one link — an unbalanced `[` or `]` —: at `title`, naming the citing `path:line`.

### Edge ops

`add-edge`, `edit-edge` and `remove-edge` each write one edge; many edges are many entries. An edge is named by `source`, `target` and `relation`, over the relations and legal pairings of [§10](#four-scoring-relations-one-arithmetic); a `demoted` record is defined in [§10](#the-demoted-relation).

| Op | Keys |
|---|---|
| `add-edge` | `source`, `target` (node ids) and `relation` (`supports`, `founds`, `lifts`, `refutes` or `references`), required; `bearing` (a number in `[0, 1]` or `*pending*`; scoring relations only; default `*pending*`) and `context` (a string, one line, `""` for none), optional |
| `edit-edge` | `source`, `target` and `relation`, required; `bearing` and `context`, at least one |
| `remove-edge` | `source`, `target` and `relation` (any of the six, `demoted` included), required |

**Results**: `source`, `target`, `relation` for each.

**`add-edge`**
- Over an edge of that identity the entry is refused "edge exists", whatever its values; `edit-edge` changes them. Over a demoted record of that relation from `source` to `target`, the add writes the edge with its own values and deletes the record.
- The `manually-scored` flag and the `judged-hash` an edge write sets are defined in [§10](#provenance-and-the-judged-hash). A `bearing` given on a `supports`, `lifts` or `refutes` edge hashes the edge's two ends' statements as the batch leaves them.

**`edit-edge`** changes an existing edge's `bearing`, `context`, or both. `unchanged` where it changes no value, no flag and no judged hash. A `bearing` given on a `supports`, `lifts` or `refutes` edge hashes the edge's two ends' statements as `add-edge` does; an edit giving none keeps the record's `judged-hash`.

**`remove-edge`** removes the one edge record the entry names. With `relation: demoted` it removes every demoted record from `source` to `target`, leaving the edge out of the graph. Every other record stays byte for byte as it was.

**Refusals**:
- "edge exists" (`add-edge`) and "no edge" (`edit-edge`, `remove-edge`, where no record of that identity exists) are items with no `check` and `key` `relation`; their `detail` is `edge exists: <relation> <source> → <target>` and `no edge <relation> <source> → <target>`, an "edge exists" remedy naming `edit-edge` and a "no edge" remedy naming `edges`;
- a `source` or `target` no node has: on `add-edge` and `remove-edge`, `check: unknown-id` at that key, `detail` `no node <id>`; on `edit-edge` the entry is refused "no edge";
- a pairing the relation does not join: at `relation`. On `add-edge` its `allowed` is the relations joining the two kinds, in the relations table's order, `references` always among them. `remove-edge` refuses it for a scoring relation only;
- `relation: demoted` on `add-edge` or `edit-edge`: a shape refusal at `relation`, `allowed` the five other relations, its remedy naming `restore-demoted` (and, on `edit-edge`, `remove-edge`);
- a `bearing` on `references`: at `bearing`. An `edit-edge` entry giving neither `bearing` nor `context`: a shape refusal whose `allowed` is `bearing` and `context`. A `bearing` outside its domain: at `bearing`;
- a cycle among the scoring edges that the edge would close: `check: dependency-cycle`, `key` `target`, the `detail` naming the cycle in edge direction; the remedy is to remove another edge of the cycle first.

### `remove-node`

`remove-node` removes a node of any kind and every record naming it: the node record, its statement marker (or an axiom's heading in `axioms.md`), every edge naming it at either end, and every demoted record naming it. kbase keeps no undo: git restores what a removal took, and a dry run previews it.

- **Keys**: `id`, required.
- **Result**: `id`, and `removed-edges`: every edge record removed, each as the full row `edges` answers ([§8](#edges)) — `bearing` and `context` included, so an edge can be added back onto a replacement node without reading `edges` before the removal — ordered by source, target and context, demoted records included.
- An axiom's removal takes every `### Axiom` heading that ends in its marker out of `axioms.md`, and nothing else of that file.
- A removal that shifts another summary entry's or `axioms.md` heading's repeat suffix — a later heading of the same slug taking a lower one — updates every link into the shifted anchor in the same write, as an edit does ([§8](#edit-kind)). A link into the removed node's own entry or heading has no new destination and stays; it is not refused unless it is a citation.

**Refusals**, checked at the point the entry applies:
- an `id` no node has is refused "no node": `check: unknown-id` at `id`, `detail` `no node <id>`; so a repeated removal is refused;
- the id is still mentioned. One item per citing line and per mentioning field, each at `id`:
  - a citation of the node, read outside code in every document but the summary pages, which the rebuild renders, named by `path` and `line`. The remedy is to edit that prose;
  - for a work, a citation span whose `data-cites` names its label where no reference-list entry in any such document has that key, one item per span line, named by `path` and `line`. The work record is that span's only bibliography entry, so without it the export prints the span as plain text. The remedy is to edit that prose;
  - a prose field of a record that survives the removal (`title`, `rationale`, an edge's `context`) that cites the node or contains its id bare, named by its record file as `path` and by record and field in `detail`. The remedy names the `edit-<kind>` of the record's kind or `edit-edge`, an entry earlier in the same batch; for a demoted record's `context`, a `restore-demoted` then `edit-edge`, or a `remove-edge` with `relation: demoted`.

### `move-node`

`move-node` moves a hosted node's statement marker to the end of the line `document`, `line` and `snippet` name ([§10](#statements-and-anchors)), removing it from its previous line, and updates each citation of the node to its new host and section anchor in the same entry.

- **Keys**, all required: `id`, `document`, `line`, `snippet`.
- **Result**: `id`; `unchanged` where the marker already ends the named line.
- **Refusals**: the named line's refusals are those of the add ops ([§8](#add-kind)). A work's or an axiom's `id` is refused at `id`, since it has no statement marker; an `id` no node has is refused as an edit's is ([§8](#edit-kind)); a node with no statement marker is refused `check: statement-marker` at `id`.
- Every other occurrence of the node's marker is taken from its line, and a line it leaves without prose goes with it, so a node with more than one marker is left with one.
- Every citation of the node outside code, in every document but the summary pages, points at its new target after the move, written relative to the citing document, whatever it pointed at before; only the link target changes, its link text stays. A citation an author's own edit breaks, such as by a heading rename, is a broken link, which `verify`'s `dead-link` check reports like any other.

### `add-citation`

`add-citation` turns text the author wrote in a document into a citation of a node ([§10](#citations)): it replaces `snippet` on the named line with a link to the cited node's link target, written relative to the citing document.

- **Keys**:
  - `document` (the citing document, kb-root-relative), `line` and `snippet`, required. `snippet` is the span of the line that becomes the link text; it is replaced, not just checked, so it must occur exactly once on the line, matched as written, outside code, links and statement markers. A `snippet` the line lacks is refused, naming the nearby lines that contain it ([§10](#statements-and-anchors)); one that occurs more than once is refused, naming each column, since kbase does not guess which span the caller meant;
  - `id` (the cited node), required;
  - `quote`, optional.
- **Forms**, by the keys given. Each is a Markdown link whose title is the cited node's id:
  - quote: `quote` absent. `snippet` is a verbatim quote and must appear in the cited node's statement; it is written `["<snippet>"](<path>#<anchor> "<id>")`;
  - paraphrase: `quote` given. `snippet` is the paraphrase, written `[<snippet>](<path>#<anchor> "<id>")`; `quote` is the passage paraphrased and must appear verbatim in the cited statement;
  - naming: `snippet` is the cited node's id itself. It is written `[<title>](<path>#<anchor> "<id>")`, whatever `quote` gives. The author writes the id where the reference belongs and converts it.

  A work or an axiom has no statement, so it takes the naming form only. A quote is compared with the statement as [§10](#statements-and-anchors) reads it, each whitespace run collapsed.
- **Result**: `document`, `line`, `id`, and `link` (the text written).
- A citation written into a heading that moves an anchor — that heading's own, or a later same-text heading's `-1`, `-2` suffix — updates every link whose destination resolves to a moved heading, citation or not, in the same write. The links updated are the inline links and the reference definitions outside code, in every document but the summary pages, which the rebuild renders; only a link's fragment changes.

**Refusals**:
- `document` is any document of the KB, kb-root-relative; a summary page, which kbase renders, is none and is refused at `document`;
- a `line` past the document's end: at `line`. A `snippet` the line does not contain is refused at `snippet`; a citation already in place is no match, so a repeated entry is refused as a line that does not contain the snippet;
- an `id` no node has: `check: unknown-id` at `id`, `detail` `no node <id>`, as an edit's is ([§8](#edit-kind));
- a hosted node with no statement marker, which has no link target: `check: statement-marker` at `id`;
- a quote or a paraphrase of a work or an axiom: at `snippet`, the remedy naming the naming form;
- a quote absent from the cited statement: at `snippet`, or at `quote` where it is given;
- a link that would not parse, whole, as one link to the target — an unbalanced `[` or `]` in the `snippet`, or in the title the naming form writes: at `snippet`, or at `id` for the title.
