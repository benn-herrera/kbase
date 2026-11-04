### Queries

Every query answers from the KB's records and statement markers, writes nothing and takes no lock. A KB in an older format answers as converted, with `migration` in its result ([§10](#one-version-read-and-written)); a KB whose load fails is refused ([§10](#the-load-every-fault-refused)). A query's data sits under `results`: a list with `count` for `citations-of`, `find`, `edges` and `trace`, a mapping for `show` and `stats` ([§7](#item)). A list query returns its complete answer ([§7](#bounded-results)). `kbase <query> --help` states its arguments.

### `citations-of`

`citations-of` lists where prose cites a node.

- **Arguments**: `id`, required.
- **Rows**: `path` (the citing document), `line` and `text` (the link text as the document writes it), ordered by path, then line. The answer is complete: `count` and `results`.
- A citation is read by the id in its link title ([§10](#citations)), wherever its link points, outside code, in every document but the summary pages, which the rebuild of the derived views renders.
- An `id` that names no node is refused `check: unknown-id` at `<id>`, `detail` `no node <id>` ([Idempotence](#idempotence)).

### `find`

`find` lists the nodes that match its filters, across every kind; with no filter it lists every node. `find` locates nodes; `show` reads one.

**Arguments**, all optional; a node must match every filter given:
- `--kind`: `claim`, `analysis`, `experiment`, `work` or `axiom`; repeatable, keeping the nodes of any kind named.
- `--title`: text, matched as a case-insensitive substring of the title.
- `--label`: text, matched as a case-insensitive substring of the label, as `--title` is. Only works and axioms have a label.
- `--path`: a kb-root-relative directory, matching every node hosted under it, or a leaf, matching the nodes it hosts. `""` and `.` name `kb-root/`, matching every hosted node. A work, an axiom and a node with no host never match it.
- `--min`: strength at or above. `--max`: strength strictly below.
- `--pending`: the nodes with no strength yet.
- `--min-dependents`: an integer; the nodes with at least that many dependents ([§10](#four-scoring-relations-one-arithmetic)).
- `--sort`: `id` (the default), `strength` or `dependents`. `--order`: `ascending` (the default) or `descending`. `--limit`: allowed only beside an explicit `--sort` ([§7](#bounded-results)).

A node's strength here is its derived strength for a claim or an analysis, and its rigor for a work, an experiment or an axiom. `--min`, `--max` and `--pending` apply to every kind. `--min` and `--max` compare scored strengths only, so a pending node matches neither, and `--min 0` keeps exactly the scored nodes.

**Rows**: `id`, `node_type`, `title`, `label` (null for a claim, an analysis or an experiment), `host` (the host leaf's path; null for a work, an axiom and a node with no statement marker), `strength` (`*pending*` where pending) and `dependents`. The answer is complete, sorted and limited as [§7](#bounded-results) states: `count` and `results`.

- A row's `strength` is the derived strength, so a bearing that changes a claim's strength changes the list. Its `dependents` and `show`'s `premise_count` are structural — counts of edges, which no bearing changes. `strength` is a number at two decimals; the filters and the sort read the full value.
- `--order` turns the named key only: ties under `--sort strength` or `--sort dependents` are broken by ascending strength, then ascending id, in either order.
- A refusal is `check: usage` at the flag: `--kind`, `--sort` and `--order` outside their vocabularies, each with it as `allowed`; `--pending` beside `--min` or `--max`; `--limit` beside no `--sort`, and `--limit 0`.

### `show`

`show` returns the record of one node of any kind.

- **Arguments**: `id`, required.
- **Record**: `node_type`, `id`, `title`, `host`, `host_anchor` and `summary`, then the kind's own fields ([§7](#item)). `host` is `find`'s.
- `host_anchor` is the anchor of the section holding the node's first statement marker in its host ([§10](#statements-and-anchors)). `host` and `host_anchor` are null for a work, an axiom and a node with no statement marker.
- `summary` is the node's summary-page entry, written `<page>#<slug>`; null where the node has none.
- A pending `rigor` or `strength` reads `*pending*`, as in every query; null is only for a value that does not apply.
- An `id` that names no node is refused `check: unknown-id` at `<id>`, `detail` `no node <id>` ([Idempotence](#idempotence)).

### `stats`

`stats` takes no arguments and gives a complete snapshot in one call, its keys as [§7](#item) lists them: one count per node kind; `edges` and `demoted_edges`; `subtree_aggregates`; `rigor_manual`, `rigor_model`, `bearing_manual` and `bearing_model`; `relations`; `strength_bands`, the claims counted by band ([§10](#the-band-ladder)); and the ranked sections `highest_leverage` and `weakest`.

- `subtree_aggregates` is one record per volume ([§10](#documents-of-a-kb)), keyed by the volume's directory in path order, each with `claims` and `experiments`, the counts of those whose statement marker lies in a leaf under it. A KB with no volume has `{}`.
- Each ranked section holds ten rows and previews a `find` question, which its help names: `highest_leverage` is the first ten of `find --kind claim --max 0.65 --min-dependents 1 --sort dependents --order descending`, and `weakest` of `find --kind claim --min 0 --sort strength`.
- `stats` reads derived strengths in `strength_bands`, `highest_leverage` and `weakest`, and is structural in its per-kind counts, `edges`, `demoted_edges` and the edge counts of `relations`.
- `relations` and the `edges` query read the edge records and no derived value: the bearings they report are the values the records hold, which `edit-edge` changes. `relations` reports `walked` per relation: whether a score run's bearing pass makes model calls for that relation's edges — `supports`, `lifts` and `refutes` — where a `founds` bearing is the user's, written by `add-edge` or `edit-edge`.

### `edges`

`edges` lists edge records.

**Arguments**, all optional:
- `--id`: the node. With no node given, every edge is listed.
- `--direction`: `in` (the node is the target), `out` (the node is the source) or `both`, the default. It needs `--id`.
- `--relation`: any of the six relations ([§10](#four-scoring-relations-one-arithmetic)); repeatable.

**Rows**: `source`, `target`, `relation`, `bearing` (as the record holds it; null where the relation has none), `context` (null for none), `manually_scored` (null where the relation has no flag) and `replaced_relation` (null except on a `demoted` row). The flag tells a caller that `score --edge` will need `--rescore-manual`; whether an edge is stale is `score --dry-run`'s answer. The answer is complete, in a fixed order: `count` and `results` ([§7](#bounded-results)).

- Rows order by relation (in the order `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted`), then source, then target. Rows that share relation, source and target — demoted records replacing different relations — order by `replaced_relation`, in the relations' order.
- A `demoted` row's `bearing` and `manually_scored` are the ones its record keeps of the replaced edge ([§10](#the-demoted-relation)).
- A usage refusal is `check: usage` at the flag: `--relation` and `--direction` outside their vocabularies, each with it as `allowed`, an unknown relation once however often it repeats; and `--direction` beside no `--id`.
- An `--id` that names no node is refused `check: unknown-id` at `--id`, `detail` `no node <id>` ([Idempotence](#idempotence)).

### `trace`

`trace` follows the scoring relations transitively from one node. `in` lists everything that bears on the node's strength; `out` lists everything the node's strength reaches. It never follows `references` or `demoted` records. `edges` gives one hop; `trace` gives the full traversal.

**Arguments**:
- `<id>`: required.
- `--direction`: `in` or `out`, required.
- `--sort`: `depth` (the default, reading outward from the node) or `contribution`. `--order`: `ascending` (the default) or `descending`. `--limit`: allowed only beside an explicit `--sort` ([§7](#bounded-results)).

**Rows**: one per edge reached — an `edges` row, then `depth` (its distance from the node), `strength` (the source's strength; for a work, an experiment or an axiom, its rigor) and `contribution`. The answer is complete, sorted and limited as [§7](#bounded-results) states: `count` and `results`.

- An edge reached by more than one path is one row, at its least depth.
- `contribution` is the edge's input to its target's strength as strength propagation computes it. It is `*pending*` where the edge's bearing or its source's strength is pending, and null where the edge takes no part, as a planned experiment's does.
- `--sort contribution` orders every relation together by `contribution`. On a `refutes` edge the contribution is its cap, 1.0 minus the refutation's strength, so a refutation that holds its target down hard sorts low and one that barely limits it sorts high. Ascending puts the inputs that hold their targets lowest first; descending, the ones that leave them highest. Each row names its relation.
- `--order` turns the named key only: ties break by ascending depth, then `edges`' order.
- `strength` and `contribution` read at two decimals, as `find`'s `strength` does; the sort reads the full values.
- A refusal is `check: usage` at the flag: `--direction` missing or outside `in` and `out`, with them as `allowed` (over MCP the schema requires `direction`, so a call missing it is refused as the `query` tool refuses any missing argument, [§11](#tools)); `--sort` and `--order` outside their vocabularies, `--limit` beside no `--sort`, and `--limit 0`, as `find`'s. An `<id>` that names no node is refused `check: unknown-id` at `<id>`, `detail` `no node <id>` ([Idempotence](#idempotence)).

### `export-latex`

```
kbase export-latex [--out DIR] [--volume NAME]...
```

Writes a LaTeX project from the KB's current state into `--out` (default `latex-export/` beside `kb-root/`; a relative `--out` resolves against the project root (§9)). The KB is read under the format-version rules defined in §10; nothing under `kb-root/` is written; a KB whose load fails is refused with the load error. `--volume` is repeatable, each naming a volume ([§10](#documents-of-a-kb)); the named volumes export in entry-point order. A `--volume` that is not a volume is a usage refusal, `allowed` every volume, and so is one naming a volume the entry point's list does not name, which the export does not reach (§7). The out directory may be any directory, the author's own LaTeX project included: kbase owns only its `gen-tex/` subdirectory and writes nothing else there but a missing `preamble.tex`. **Manifest refusal.** A `gen-tex/` that exists holding files without the export manifest `gen-tex/kbase-export.yaml` is refused whole, nothing written — `check: export-dir`, remedy: empty the directory or name another `--out`.

#### The project

`gen-tex/` is kbase's and every export regenerates it: `main.tex`, the whole export; `defaults.yaml`, the pandoc defaults file that produced it, its paths relative to itself; `kbase-export-front.md`, the entry point's prose, and `kbase-export-vol{N}.md`, one per exported volume in order, its documents in navigation-tree order — read in that order, each file's footnotes its own; `kbase-export.latex`, the template; `kbase-export.lua` and `kbase-xref.json`; `assets/`, a copy of every non-Markdown file under each exported volume's directory less the claim-graph sheets (§3), at its `kb-root/`-relative path; `kbase-export-findings.json`, the pandoc filter's findings, which the export merges into the report; the report and the manifest. None of it is for editing. **The preamble is the author's.** `main.tex` loads `preamble.tex` from the out directory, beside `gen-tex/`, after the template's base packages and before its cross-reference packages. Where no `preamble.tex` exists, the export writes a starter — commented, loading nothing, advising the author to add the packages and macros their sources used; where one exists, the export never reads, writes or removes it. Whenever a `preamble.tex` exists, the report contains a line naming it as the author's. A built KB keeps its maths as the sources wrote it, so a macro or package those sources' preambles supplied is the author's to add there; LaTeX names any that is missing. The author may re-run `pandoc -d defaults.yaml` from `gen-tex/` without kbase. Compiling `gen-tex/main.tex` is the author's step; kbase runs no TeX engine.

#### Content

The export reads every document the navigation tree names, `overview.md` included, as ordinary leaf content; `references.md` leaves are read as bibliography data, never as documents. Author labels are namespaced per volume, so a label repeated across volumes stays distinct, and LaTeX numbers what they label. A reference naming a label its volume defines resolves to it; a cleveref-family anchor (`ref+label`, `ref+Label`) renders as one `\cref`/`\Cref` over every label it names, one or several, without the anchor's own text beside it. A link to an exported KB document becomes a hyperlink to that document's heading. Prose is exported as written: a hand-maintained KB's prose mentions of equations and works (`(eq:dv)`, `(Khalil, 2002)`) and its bold-line theorem blocks stay text. The bibliography holds `references.md`'s entries or, where a volume has none, the KB's work records.

**Simplification, not refusal** (the `dot` posture defined in §3): a reference whose label its volume does not define, a link to a document not exported, and a citation with no bibliography entry each become the text they contain; an image whose file is not in the KB becomes its alternative text; a figure or table the export cannot rebuild, or one LaTeX cannot float, is written as far as it can be; and maths the export cannot recover from the page is left out. Each gets a report line in the export report, `gen-tex/kbase-export-report.yaml` (§7's `report` key). Idempotent: a repeat over the tool's own previous export leaves the export byte-identical and reports `unchanged`. The export takes no KB write lock — nothing under `kb-root/` is written, so it reads as the queries do — and it runs no git. Result keys: §7.

### `score`

```
kbase score [--pass rigor|bearing|all] [--node ID] [--edge SOURCE,TARGET,RELATION]
            [--rescore-manual] [--scoring-model MODEL] [--dry-run] [--resume] [--state-dir DIR]
            [--timeout DURATION] [--ask-concurrency N]
```

`score` grades the KB's rigor and bearing values by calling the scoring model: one call per item, one value per call. It computes each value from the faults the reply names, writes it through the edit ops ([Writing the results](#writing-the-results)) and records every attempt in a ledger.

**Scope.**
- By default a run covers every stale item that is not manually scored, in both passes. What makes an item stale is the judged hash ([§10](#provenance-and-the-judged-hash)), so a second run over a KB it has scored makes no model calls.
- The rigor pass covers claims, analyses and experiments. The bearing pass covers `supports`, `lifts` and `refutes` edges. No pass reaches a work's rigor (judged by hand), an axiom's rigor (`1.0` by rule) or a `founds` bearing.

**Arguments**, all optional:
- `--pass`: `rigor`, `bearing` or `all`, the default.
- `--node`: a claim, analysis or experiment id. Scores that node's rigor, stale or not.
- `--edge`: scores that edge's bearing, stale or not. At the command line it names its edge as `<source>,<target>,<relation>` (ids and relations hold no comma); over MCP `edge` is a mapping of the three.
- `--rescore-manual`: a bool. In a full run it also scores every manually scored item of the pass, stale or not; an item that is neither stale nor manually scored is still skipped. It is required to name a manually scored item with `--node` or `--edge` ([Skipped items](#skipped-items)).
- `--dry-run`: a bool; lists what a run would score and writes no KB file ([The closing record](#the-closing-record)).
- `--resume`: a bool. Continues the repository's most recent score run, with the scope it recorded, `rescore_manual` included, keeping every value that run wrote; it takes none of `--pass`, `--node`, `--edge` or `--rescore-manual` beside it ([The ledger and resume](#the-ledger-and-resume)).
- `--scoring-model`: a `<provider>/<model>` reference for this run ([The scoring role](#the-scoring-role)).
- `--ask-concurrency`: under 1 it is refused as `build`'s is (§5).
- `--timeout`: a duration, the deadline for one item's call (default `30s`), counted from the call's start, never while the call waits for its turn.
- `--state-dir`: the state store ([The ledger and resume](#the-ledger-and-resume)).

`--resume` takes neither `--ask-concurrency` nor `--timeout`: each shapes one invocation, not the run. `score` has no `--provider`: a scoring-time endpoint is named by `--scoring-model`, which states the provider and the model in one value, and passing `--provider` is a usage refusal. Every refusal is listed under [Outcomes](#outcomes).

A run makes one call per item and takes minutes. At a terminal `kbase score` blocks until the item loop ends. As an MCP tool ([§11](#tools)) it starts a detached child and returns once the run has started, while `kbase wait` reports its outcome when it ends. A shell caller gets the blocking behaviour a terminal gets.

There is no score batch and no item limit: the scope the caller names is the budget. `kbase cancel` stops a run, `--resume` continues it, and what the run wrote stays.

#### The two passes

`rigor` covers claims, analyses and experiments by id; `bearing` covers `supports`, `lifts` and `refutes` edges by relation in that order, then source, then target. A run over `all`, the default, covers the node set and then the edge set: a text edit changes a node's judged hash and the hash of every edge that touches it ([§10](#provenance-and-the-judged-hash)), so a run scoped to one pass leaves the other stale against that edit. An all-pass run is one run: one run lock, one `run.yaml`, one `items.jsonl`, one `ledger.jsonl` and one `--resume` call, and its `run.yaml` names the set it covers (`all`); a run naming a node records `rigor`, one naming an edge `bearing`. The set is read from the KB as loaded at the run's start, once, and does not change as values are written. A work's rigor is judged with the document in hand and `edit-work` writes it; an axiom's is `1.0` by rule; a `founds` edge's bearing is the reader's, made with the work in hand, and `edit-edge` writes it. The `founds` edge has a bearing like any other — [§10](#four-scoring-relations-one-arithmetic) gives its contribution as `bearing × the source's rigor` — and no pass reaches it.

#### Skipped items

A value whose record is `manually-scored: true` ([§10](#provenance-and-the-judged-hash)) is a person's judgement and stays. `--rescore-manual` reaches it: a full run with the flag also scores every manually scored item of its pass, and a `--node` or `--edge` naming a manually scored item is refused without it. The flag is read from the KB the run loaded at its start. A full run without it reaches a stale manually scored item and skips it: no model call is made for the item and nothing is written for it, and its ledger line records status `skipped`, reason `manually-scored`, with no value, charges or call, and so no capture. It counts under `counts.skipped`, never under `scored` or `failures`, and does not make `resume` present. A value `--rescore-manual` re-judges is written with the flag `false`, so later runs re-judge it too. `run.yaml` records the flag (`rescore_manual: true`), and `--resume` continues the run with it, appending no second `skipped` line for an item; a resume's recomputation of values under the current weights leaves a skipped item's line as it is. A `--dry-run` lists the stale manually scored values it would skip under `skip`, counted under `counts.skipped`.

An item whose node or edge is no longer in the KB — found when the results are written or when a resume reaches it — is skipped too, once, counted the same way: its line records status `skipped`, reason "the node or edge is no longer in the KB; its value is discarded and a resume does not ask it again", and no model call is made or value written for it.

#### One item, one quantity, one call

A call contains one node's text or one edge's — never two subjects, never the other pass's quantity. The item's class determines the instruction as it determines the ids: a claim is asked whether its statement is fixed, consistent and single-sensed, an analysis or an experiment whether its method establishes what it sets out to, and neither is asked the other's question. The item text is limited to the subject: a node's title and statement ([§10](#statements-and-anchors)), or an edge's two statements and its authored context; a summary page is never read. The input guard refuses, before the call, a text containing a scoring token (`- rigor:`, `- strength:`, `(bearing …)`, `(strength …)`) or the template wrapper's own structure (a line opening `## Source:`, `## Target:` or `## Context`, or a run of twelve backticks), naming the document and line, and that item is `failed` with no call made.

#### The answer

A reply is a list of charge lines, each one fault id and the numbered line of the item's own text where the fault shows — one line per occurrence, in the order written — or the word `none`, alone. The call sends the text wrapped at word boundaries to at most 100 columns, its own line breaks kept and nothing rewritten, and writes `N| ` before each line lying inside a section of the prompt input that a charge may cite, numbering from 1: a numbered line is a verbatim slice of that text. A template wrapper label and an edge's `## Context` have no number, and a line with no number is no site.
[RIGOR_AND_BEARING_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/rigor-and-bearing/entry-point.md) states the criteria, the three fixed sets and the citation rule; this section states what kbase does with a reply.

- **The ids are the item's class's.** `rigor` on a claim accepts the statement vocabulary's four, `rigor` on an analysis or an experiment the method vocabulary's six, and `bearing` its five, and nothing else; an id outside that set — the other rigor class's among them, and `flt-pending`, which no table holds — is `malformed`, the refusal naming the id and the call's ids as `allowed`.
- **The citation is the numbered line where the fault shows.** The prompt asks for one number, the line where the fault first shows, `flt-inapplicable: 12`, and never a range. The parse still reads a contiguous range within one section, `flt-transfer: 12-14`; its dash may be a hyphen, an en dash, an em dash or a minus sign, with or without spaces. kbase resolves the field against the rendering the call sent — never a summary page — and records three facts beside the `passage` the lines lie in: `lines`, the answer's own numbers; `citation`, the quoted lines without the gutter; and `offset`, the quote's byte offset in the sent text. A charge naming no line, one naming a line the call does not show, and a range crossing from one section to another or over a gap — a blank line, a label or a heading between its lines — are `malformed`, naming the id and the lines. A numbered line lies in one section, so the `passage` value `both` — a span the two ends carried alike — is not produced; a ledger line recorded with it keeps the meaning it was written with.
- **Nothing is read out of prose.** Blank lines are ignored and any other line that is not a charge line is `malformed`. So is `none` beside any other line, a bare number — a decimal, a percentage, a fraction, a `*pending*` written as a value — since a number alone names no fault id, and an empty reply: silence is never a clean pass.
- **Duplicates and overlaps.** One id on one line set, written twice, is one charge; two line sets of one id that overlap without coinciding is `malformed`, naming both lines; two ids on one line set are both charged, the quote and its offset shared.

**The three parse rules are fixed for as long as the ledger's format version is unchanged.** The citation's floor of at least one valid line number, `none` beside charges refused, and one id over two line sets that overlap without coinciding refused: what changes one is a new version, which `--resume` reads by its own rules. A valid line number is one the call shows, lying in a section a charge may cite. A ledger's meaning is its ids, and a rule that loosened later would re-read lines already written under a new grammar.

#### The value

kbase computes the value from the charge list: the item's class's maximum — `1.00` under each of the three tables — less each id's weight per occurrence under that class's table, floored at zero and exact at two decimals, summed in integer hundredths. `none` is the empty list and reaches the maximum; a ledger line recorded with `flt-pending` gives `*pending*` whatever the table, though no table holds that id. The weights are defined in [RIGOR_AND_BEARING_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/rigor-and-bearing/entry-point.md). No prompt template contains a weight, a maximum or a band: the ids and the criteria are the prompt's, the value is kbase's, and nothing else is an input. A written value is therefore checkable against its own ledger line without calling a model.

#### A refused reply is a failed attempt

A `malformed` reply writes nothing, and the attempt's line records `status: failed` and the reason — the case, and where it names one the id and the lines — and **no text of the reply**. The same holds for a reply cut off at the completion cap, whose charge list may be incomplete, and for a call or a write that failed. The run goes on, and `--resume` retries the item. Nothing is defaulted or supplied by kbase: a link or a node is charged, answered `none`, or left without a model call, and `*pending*` is no model's answer — the edit ops are what write one, as a person's decision.

#### A provider call's failure names what ended it

`provider-refused` — the endpoint answered with a status that is not success: the request was delivered, read and declined, and the reason contains that status and the provider's own words beside it. `transport-failure` — the connection, the name, the certificate or the stream failed, and no answer of the provider's was had. `truncated-empty` — the provider answered, and the answer reached the completion cap before any answer text: the request leaves it no room, and it is not sent again. A reason states the one that happened: the first calls for the request to be looked at, the second for the path to the provider. A `provider-refused` 401 or 403 is the provider refusing the call's key, and its failure item has the remedy that stores one (§9, [A refused key](#a-refused-key)).

#### Writing the results

A value is written through the edit ops with the `manually-scored` flag `false` and the judged hash of the text its call was composed under ([§10](#provenance-and-the-judged-hash)): a rigor answer as an entry of its node kind's op — `edit-claim`, `edit-analysis` or `edit-experiment` — with the node's `id`, a bearing answer as an `edit-edge` entry with the edge's `source`, `target` and `relation`. A value whose item's text was edited after its call was composed is written nowhere: its line is `failed`, reason "its text was edited after it was asked; the value is discarded and a resume asks it again", and a resume scores it again. A value whose node or edge is no longer in the KB is written nowhere either and fails nothing: it is skipped as [Skipped items](#skipped-items) states. The run writes every answered value at once when its item loop ends: one mixed write batch ([§8](#write-batches)), which rebuilds the derived views as every batch does, under one hold of the KB write lock. An entry the batch refuses is that item's `failed`, and the run repeats the batch without it; a write that cannot take the lock fails every item it contained: where another writer held the lock past the wait, the item's line has the lock item's `detail`; any other lock failure records its own error. The run goes on.

#### The scoring role

The calls go to the `scoring` role (§9): `models.scoring`. The role is the KB's declaration of which model judges it, so a run that names none refuses as setup — naming that key, before the KB is opened and before the run's start rebuild — and nothing falls back to the text role. The role's model is the file's `<provider>/<model>` reference or, where the files name none, the one `--scoring-model` names: the flag supplies an unconfigured role and overrides a configured one, its reference is resolved against the pool, and a provider the pool does not hold refuses naming the flag with the pool listed. A run records the model as the flag's where the flag named it. A `--dry-run` resolves no role and refuses nothing for one.

#### One item, one call

Each call is one tool-less chat request on the `scoring` role's endpoint, over the chat protocol that endpoint's entry names (§9.1): `openai` speaks `chat/completions` and `anthropic` speaks the Messages protocol. Every call a run sends to an `openai` endpoint sends the run's id as its `prompt_cache_key`, so a run's calls reach one provider and share one cache entry; the Messages protocol has no such field, so a run whose endpoint speaks it sends no key. A Messages call sets the completion cap [§9.1](#the-chat-protocol) states; no other request is capped. The call sends the scoring role's sampling settings (§9.2) in its protocol's fields (§9.1), and thinking text is never part of a reply on either protocol. A call that does not complete is retried identically, up to three attempts in all, pausing 5 s before the second and 30 s before the third. What fails the item at once, on the one attempt that met it, is a provider's answer with a status that is not retryable — any non-2xx other than 429 and the 5xx class — an answer that reached the completion cap before any answer text, which an identical retry would cut off the same way, and a reply the parse refuses. `--resume` is how a user retries a failed item.

#### The ledger and resume

A run's records are kept under `<state-dir>/score/<run>/`: `run.yaml`, `items.jsonl` (the set the run enumerated, one pending line per item, so a reader counts the whole run), `ledger.jsonl`, `captures.jsonl` and, while the item loop runs, one result file per answered item, which the loop's end merges into the ledger. A real run's id is `run-<id>`; which runs are kept is defined in §6. `run.yaml` holds the run, its KB root, the ledger's format, the set it covers (one pass, or `all`), `rescore_manual` (`true` where the run re-judges manually scored values, absent otherwise), `backend` (`provider`; absent under `--dry-run`, which resolves no role because it makes no model calls), the provider, the model, `ask_sha256` and the start. **It is written once, at the run's creation, and no resume rewrites it**: a resumed or mixed run's header states the first invocation's back end and model, and each ledger line states its own item's pass and what answered it.

**What `--resume` continues.** The run it continues is the state store's newest real run by its `run.yaml` start — a dry run is never one. A resume covers the run's own `items.jsonl`, the set it enumerated, and retries each item that is not `done`, under the pass and `rescore_manual` its `run.yaml` records. A run whose header reads `backend: environment` is resumed on whatever the configuration files now resolve, and its new lines record that.

**When `--resume` is refused**, with `check: usage` at `--resume`, the remedy a fresh `score`, which scores only what is stale:
- no score run exists in the state store;
- the most recent run finished: no item of it is `failed` or unreached;
- a build has started since that run;
- `--pass`, `--node`, `--edge`, `--rescore-manual` or `--dry-run` is given beside it.

`ledger.jsonl` is one JSON object per attempt — the item, its status, the value, the value the KB had before the attempt (`before`, absent where the record held none), the reply, `faults` (the parsed charges: `id`, `passage`, `lines`, `citation`, `offset`, in the order written), `moot` (the charge lines a reply recorded beside a retired `flt-pending`), `backend` (as the header's), the model, the prompt's digest, `remedy` (on a failed attempt whose call the provider refused for its key, the command that stores one, §9; absent otherwise), `note` (beside a `remedy`, the note on how the key reaches the store, §9; absent otherwise), `judged` (the judged hash of the item's text as the call was composed, [§10](#provenance-and-the-judged-hash)) and the attempt's bounds — appended after the item's write and its validation, never before. A line without `remedy` or `judged` is valid under the same ledger format. A resume recomputes values only for a line that records `judged`; a line without it stays as it is. A run writes its captures and then its ledger lines once, in the set's order, when its item loop ends — on completion, on cancellation and on a failure that stops it. `faults` is `[]` for one reply alone, the word `none`; a failed attempt has neither `faults` nor text. The state directory is `build`'s: `$XDG_STATE_HOME/kbase/<key>/`, or `--state-dir`, and one inside `kb-root/` is refused.

`captures.jsonl` is one JSON object per call made — the item, its class (`statement`, `method` or `bearing`), the item text whole with the line each number names, the prompt sent and its digest, the reply the call returned (`""` where the call failed), the error that failed the attempt where one did — the call's, the parse's or the write's — the model, the back end and the call's bounds. Every attempt that makes a call has one, whatever its outcome: a written value, a refused reply, a failed call or a refused write. A failed attempt's reply therefore appears whole there, where its ledger line keeps no text of it. An item the input guard refuses makes no call and has no capture, and a call cancelled in flight leaves no capture and no ledger line. A run's captures are appended before its ledger lines, at the item loop's end, and a resume appends to the same file, and a capture that cannot be written fails the run as a ledger line that cannot be does.

#### `ask_sha256` records the ask set and gates nothing

It is one digest over every instruction the run can send, each rendered for an empty subject and concatenated in a fixed order — a rigor run's statement instruction and then its method one, a bearing run's one, an all-pass run's statement, method and then bearing instruction — so every fixed byte of a run's instructions is certified and nothing that varies with an item is. Each ledger line records its own item's prompt digest beside it, which is what names the variant that judged that item. `--resume` refuses a ledger for another KB root or format. It first writes and records every answer a stopped invocation left in a result file — a value a person has set since is kept — then skips every item the ledger records `done`, and retries the `failed` ones. A run resumed after an update continues and checks no versions: one whose `ask_sha256` differs, or which has none, is run under the running templates, and the header keeps the digest the run began under. Where the running table for a written line's class gives its recorded ids a different value, the resume recomputes it, writes the changed value through the same path a run's results take — before the line records it — and leaves `repriced.yaml` in the run's directory naming each line that changed, the value it held and the value it took. An id that line's class's table no longer holds charges nothing, so a recomputation can raise a value. A recomputation that changes nothing records nothing and writes nothing. A run given no `--resume` is a new run over its own scope; what is stale is the judged hash's to say ([§10](#provenance-and-the-judged-hash)), and the ledger of an earlier run is no evidence about it.

#### The run's locks

A scoring run takes the run lock for its length (§6), in the order a build takes it, before it writes anything and before its start rebuild: a second run over the KB — a build or a scoring run — is `refused` at once, `check: lock`, `path` the holder's state-dir, its `detail` naming the holder's verb and pid, and does not wait for a run that will outlast the wait. The run lock's record names `kind: score` and the run id, which is what `status`, `cancel` and `wait` read ([§6](#git-holds-the-recoverable-record)).

#### Refresh and format

A KB whose load fails ([§10](#the-load-every-fault-refused)) is refused before the run's start rebuild, naming each fault. A KB in an older format is refused before it too, dry run or not, while its conversion is pending (`check: index`, remedy `kbase update-format`, [§10](#one-version-read-and-written)); its document has `migration`. One rebuild of the derived views opens a run that makes model calls, before its items are enumerated, so the derived views are current before the item loop; a dry run writes nothing. The batch that writes the results rebuilds the derived views, so they are current whenever the run exits — finished, failed or cancelled. The start rebuild holds the KB write lock and releases it before the item loop; one that cannot take the lock ends the run there, with the lock's own outcome — `retry` where another writer held it past the wait — and the `lock` item, whose remedy is `re-run kbase score`. A crash inside the write of the results can leave the derived views stale, which `verify` reports and the next run's start rebuild repairs.

#### Outcomes

`done` when the run finished; `cancelled` when SIGINT or SIGTERM stopped it, with no line recorded for an item whose call was in flight or not yet made, and a line for every item answered before the stop; `failed` when an item's call or write failed, or the start rebuild did; `retry` while another run holds the run lock and has not yet recorded itself (§6), or where the start rebuild could not take the KB write lock past the wait; `refused` where the run cannot go on:
- no scoring role in the configuration or the arguments (`key: models.scoring`);
- a `--pass` outside `rigor`, `bearing` and `all`; `--pass` beside `--node` or `--edge`, since a named item's pass follows from its kind; `--node` beside `--edge`; an `--edge` that is not three non-empty comma-separated parts; `--provider`; an `--ask-concurrency` under 1 — each `check: usage`;
- `--node` naming a work or an axiom, and `--edge` naming a `founds`, `references` or `demoted` edge, since no pass scores them (`check: usage`);
- `--node` or `--edge` naming a manually scored item without `--rescore-manual` (`check: usage`);
- a `--node` or `--edge` that names no node or edge (`check: unknown-id`);
- a `--resume` the list under [The ledger and resume](#the-ledger-and-resume) refuses;
- a run already holding the run lock ([The run's locks](#the-runs-locks));
- a pending conversion, and a load error ([Refresh and format](#refresh-and-format));
- a `--state-dir` inside `kb-root/` (`check: state-dir`).

#### The closing record

A `failed` run ends with `failures`. Where the item loop itself failed, it holds one `check: error` item per item the ledger records `failed`, `detail` the item as the document names it and the reason its line recorded — `<node id>: <reason>`, or `<relation> <source> → <target>: <reason>` — in the loop's order, with the `remedy` the line records where it records one and the line's `note`, where it records one, ending the `detail` before what the outcome asks of the caller ([§7](#item)); the reason is the line's reply as a bounded, control-stripped summary, the ledger's line being the one place a reply appears whole; a run the start rebuild stopped ends with one `check: error` item naming the rebuild's own error. A `cancelled` run ends with `cancelled`, whose `item` is the loop's first item having no attempt, and `{}` where every item of the loop has one. A `refused` or `retry` run ends with its `refusals`.

**`resume`** is the call that continues the run — `score` with `--resume` — present wherever an item failed or was not reached; a `skipped` item is no such item. It comes after `counts` and `scored` and before the final record. It is absent from a dry run and from a stop before the item loop: the start rebuild's, the ledger's or the load's. It names no argument of the scope — the run's `run.yaml` records it — and includes `--state-dir`, `--config-dir` wherever the invocation named one — the resumed run must read that directory and no other, or the same run could resolve another reference — `--project-dir` wherever the invocation named one, and `--scoring-model` whenever the invocation did — the call must reach the same model, or it is a different judgement under the same run id.

**`--dry-run`** writes `run.yaml` and `items.jsonl` (one pending line per item of its set — the items it would score, manually scored ones included where `--rescore-manual` reaches them, and the stale manually scored ones it would skip), sends no request to a provider, resolves no role, and writes no KB file. Its document lists `ask`, every item the run would score, and `skip`, every stale manually scored item it would skip, each as `scored` names an item and in the loop's order, after `counts`: `total` the two together, `remaining` the items to score, `skipped` the items to skip, `done` `0`. Its `scored` is `[]`, it has no `resume` and no `roles`, and its outcome is `done`. It writes no charge record and no capture. Its run id is `dry-<id>`, and the runs kept are counted by kind (§6), so a dry run counts only against dry runs. `--resume` with `--dry-run` is a usage refusal naming `--resume`. A dry run on an older KB is refused as a real run is (`check: index`, remedy `kbase update-format`).

Result keys: §7.

### `probe`

```
kbase probe [--model <provider>/<model>] [--decision | --text | --scoring]
            [--prompt TEXT] [--timeout DURATION]
```

One model call, no KB and no state: a diagnostic of a model selection, answered by the selection itself. A probe calls one selection, and its flags name it: `--model` names a `<provider>/<model>` reference, resolved against the pool as a role's reference is; `--decision`, `--text` and `--scoring` each call the selection the configuration resolves for that role — the `models.<role>` reference — and supply no model of their own; and with none of the four given the probe calls the `scoring` role, the selection a scoring run would make, so `kbase probe` with no arguments calls the selection a run would use. The four are mutually exclusive: more than one is a usage refusal naming all four as what may be chosen once, before anything resolves or any request is sent. A selection that does not resolve is `refused`, naming the role's own key — `models.<role>` — or the flag, and what is missing, before any client is opened. The document states which selection answered: the role's name where a role flag or the default named it, and the reference where `--model` did.

A chat selection is one tool-less chat request, sent as a scoring run's calls are, over the chat protocol the provider's entry names (§9.1): no tools, no history, an empty system prompt, and `--prompt` in place of the default one-word check. It sends its role's sampling settings (§9.2) — the `scoring` role's where `--model` names the selection — and the document reports the effective settings, `temperature` and `thinking`. A `--decision` selection is one decision call on the decision API its provider's `decisionApi` names, whatever the entry's `api` (§9.1), with no system message, and its `--prompt` in place of the default predicate. `--timeout` bounds the call (default `30s`); a call that does not complete is retried at once, up to the same three attempts as a scoring run's calls ([One item, one call](#one-item-one-call)), so the deadline is spent on attempts rather than on pauses. The document reports what a ledger cannot — the selection, the provider, the model, the time to the first token, the call's wall time, the finish reason, the reply, and the usage the provider reported — and its outcome follows the call: `done` where it completed, or where a chat answer was cut off at the completion cap before any answer text (`finish-reason` `length`, `reply` empty); `failed` otherwise, with the attempt's own outcome and the provider's words in the failure, and its `remedy` where the provider refused the call's key (§9). It reads no KB and reads or writes no state directory: it prints its document and exits.

`probe` is also a read-only MCP tool taking the same arguments ([§11](#tools)). Each call is a paid model call, and its MCP note is stated in [§11](#served-text).

Result keys: §7.

### `update-format`

```
kbase update-format [--commit]
```

`update-format` converts a KB in an older format to the current one ([§10](#one-version-read-and-written)) and, whether or not the format changed, rebuilds every summary page and claim-graph sheet (§3) and rewrites the maintained blocks ([§3](#stamped-documents)). It runs alone, over a clean `kb-root/` and clean build records the conversion relocates, so that any problem is purely the conversion's and restoring `kb-root/` and any moved build record from `HEAD` reverses it. While a conversion is pending every other write is refused ([§10](#one-version-read-and-written)).

- It writes each file only where its bytes change, and removes a page or a sheet the KB no longer calls for. Without `dot`, the rule defined in §3 applies.
- It repairs nothing, and its result has no `warnings` key of its own: a conversion's warnings are in `migration.warnings`. A bare node id in prose is converted to the naming form only in the conversion of an older format ([§10](#the-300--400-migration)); on a KB already in the current format `update-format` leaves bare ids alone, and each remains a `verify` fault (`bare-id`) for the author to fix.
- The move of the build records is made once the KB loads, before the first write, so a KB whose load fails moves no record. Each moved record is reported twice: its root path, repository-relative, in `removed`, and its new path, kb-root-relative, in `written`.
- `--commit` (`commit`): without it `commits` is `[]` and the `git mv` of each build record is staged. With it, a run that writes or removes anything commits the conversion as one commit, labelled `kb-write: update-format <from> → <to>` (the format before and after, the same version twice where it did not change), holding `kb-root/` and each moved record's root path. Its clean-tree precondition makes that commit exactly the conversion; an `unchanged` run commits nothing. A commit that fails leaves the conversion written, outcome `done`, with `uncommitted` saying why, as a write batch's does ([§8](#write-batches)).
- Where the format changed, the result lists the conversion's minted work and axiom ids, bare-id citations, dropped `no-edge` texts and dropped `strengthen-by` texts, under `ids`, `bare-ids`, `no-edge` and `strengthen-by`. They are listed only on `done` and `unchanged`; on a refusal or a failure their warnings stay in `migration.warnings` and on stderr.

**Refusals**, each before anything is written:
- outside a git worktree, or where git is redirected (`check: worktree`, [§8](#write-batches)), and a repository with no `kb-root/` (`check: kb-root`);
- `kb-root/`, or a build record the conversion would move, with uncommitted changes (`check: dirty-paths`, naming each path; remedy: commit or discard them). The clean check covers `kb-root/` and those records at the repository root, and nothing else outside `kb-root/` ([§5](#output-and-placement));
- a lock: another writer holding the KB write lock past 30 s is `retry`; a build or a scoring run holding the run lock is `refused` at once (`check: lock`, [One write lock](#one-write-lock));
- a KB newer than kbase (`check: kb-format`, remedy "update kbase", [§10](#one-version-read-and-written));
- a load error, naming each fault with its file and record (`check: metadata`), a missing record file included (`check: index`, remedy "restore it from git"); `update-format` repairs none ([§10](#the-load-every-fault-refused));
- a conversion that cannot convert a file's content, naming the file and line (`check: conversion`).

Result keys: §7.

### `verify`

`verify` checks the KB and writes nothing. It takes no arguments and no lock. Its result is `findings`, then `refusals` where the outcome is `refused` ([§7](#item)).

**Faults.** Each fault is an item under `refusals`, with the check that names it:
- `dead-link`: a link under `kb-root/`, anchors and citations included, whose target names no file, or whose fragment names no anchor of the document it reaches.
  - A target resolves against the repository, taken as written; a target leaving the repository is no fault. A URL, a `~` path and a `.tex` target are not file links, and a link inside code is no link.
  - A fragment is live where it names a heading's anchor, or an HTML element's `id` outside code, in the document it reaches. A bare `#fragment` reaches its own document, and an empty fragment names the document's top. A summary page's live anchors are the entries derived for it, never the page as it is now. A fragment into a file outside `kb-root/`, or into one that is not Markdown, is not checked.
- `unknown-id`: an id in a citation's link title, or a node id in prose, that names no node. Prose ids are read outside code, of any kind. A placeholder id — a kind's prefix over `xxxxxx`, such as `clm-xxxxxx` — is never unknown.
- `bare-id`: a node id in prose, of any kind a record has, in every document but the summary pages, outside code, statement markers and link titles and destinations ([§10](#citations)).
- `frontmatter`: a frontmatter block on any document, the entry point included.
- `index-freshness`: a KB in an older format.
- `statement-marker`: a claim, an analysis or an experiment with no statement marker or more than one ([§10](#statements-and-anchors)).
- `axiom-heading`: an axiom whose record and `axioms.md` disagree by one of the three checks of [§10](#axioms). A marked heading is one, at any depth, whose line contains an axiom's marker.
- `register-freshness`: a summary page that does not match what the rebuild of the derived views renders ([§10](#documents-of-a-kb)).

**Remedies.**
- `index-freshness`: `kbase update-format`.
- `register-freshness`: the empty write batch ([§8](#write-batches)), which rebuilds every page; over a KB in an older format, where that batch is refused until the conversion is saved, `kbase update-format`, which rebuilds every page too.
- `bare-id`: a write batch of one `add-citation` entry: the `id`, the `document`, the `line` and the id as `snippet`.
- `frontmatter`: remove the block.
- Any other fault: the user's git work or prose edit, guided by the fault's detail.

**Findings** are not faults: every `demoted` record is one (`check: demoted`, [§10](#the-demoted-relation)). `findings` is present on `done` and on a `refused` outcome that lists faults. A KB whose load fails is refused with the load error and has none.

The build's gate stage runs this same check.

Result keys: §7.
