# kbase served surface

The CLI, the MCP server, and the `write` and `query` operations of kbase. Every operation, every
rule a caller meets, and the served text. The size and shape of the always-read text are out of
scope ([§5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/5-presentation-pass.md)); nothing below depends on them.

## 1. The model

A KB lives at `kb-root/` beside the repository's `.git`. It holds kbase's records under
`kb-root/.index/`, the build's records under `kb-root/.build/` ([§2.5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md)), the sheets ([§1.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)), and
Markdown documents of these classes:
- the entry point, `entry-point.md`;
- index documents, `index.md`;
- `axioms.md` ([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md));
- the three maintained-block documents and the `CLAUDE.md` redirect ([§1.7](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md));
- the summary pages, `kb-summary.md` ([§1.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)), derived;
- leaves: every other Markdown document under `kb-root/`.

Statement markers are read from leaves alone. The marker on a summary entry is annotation and hosts
nothing.

Sections:

- [§1.1 Nodes](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)
- [§1.2 Edges and relations](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)
- [§1.3 Demoted edges](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)
- [§1.4 Statement markers](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)
- [§1.5 Citations](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)
- [§1.6 Derived files: summary pages and sheets](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)
- [§1.7 Maintained-block documents](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)
- [§1.8 Format version](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md)
### 1.1 Nodes

A node is one record in `kb-root/.index/nodes.yaml`. Its kind is its id's prefix; no record carries
a kind field.

| Kind | Id | Record fields, in order |
|---|---|---|
| claim | `clm-<minted>` | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `judged-hash` |
| analysis | `ana-<minted>` | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `judged-hash` |
| experiment | `exp-<minted>` | `id`, `title`, `status`, `rigor`, `manually-scored`, `judged-hash` |
| work | `wrk-<minted>` | `id`, `label`, `title`, `rigor`, `rationale` |
| axiom | `axm-<minted>` | `id`, `label`, `title` |

- `title` is one non-empty line, unique within its kind across the KB. Nodes of different kinds
  may share a title, as a claim and the analysis split from it often do.
- `rigor` is a number in `[0, 1]` or `*pending*`. `*pending*` means unjudged, not low. An axiom's
  rigor is `1.0` by rule and is stored nowhere.
- `manually-scored` is a boolean ([§2.9](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)).
- `rationale` is a string, `""` for none. A node the rationale names without its bearing on the
  strength is a `references` edge, whose `context` says why; every kind may reference every kind
  ([§1.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)). What would strengthen a node is a graph question: `trace` with `sort: contribution`,
  `order: ascending`.
- `status` is `run` or `planned`: whether the experiment has been run. "Pending" is kept for one
  meaning only, a value not yet judged (`*pending*`).
- Every node's id is minted: its kind's prefix and a minted body. No other field is part of an id.
- `label` is one non-empty line. A work's label is its citation key,
  `[A-Za-z0-9](?:[A-Za-z0-9_.:+/-]*[A-Za-z0-9])?`, which opens and closes alphanumeric. An axiom's
  label is any text, LaTeX included. A label is unique within its kind across the KB: a work's
  among works, an axiom's among axioms. A label is never an id, and no op takes one where it
  takes an id.
- A claim, an analysis and an experiment are *hosted*: each has one statement marker in a leaf
  ([§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)). A work and an axiom have no statement marker and no host. An axiom is stated in prose in
  `kb-root/axioms.md` under the heading `### Axiom <label>: <title><!-- kbase-id: <id> -->`. The
  marker pairs the heading with its record. It is not a statement marker, and the axiom stays
  unhosted. An axiom's anchor is that heading's anchor, slugged as any heading is
  ([§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)). `verify` checks three things: each axiom record has exactly one
  heading carrying its marker, that heading's text is the record's label and title, and each
  marked heading names an axiom record. A work is shown by its entry on the root
  summary page ([§1.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)).
- `strength` is derived, never stored. A claim and an analysis carry a derived strength. A work's,
  an experiment's and an axiom's strength is its rigor. Strength is read against five bands, each an
  inclusive lower bound: `0.85` *ok to build on*, `0.65` *ok to build on, see caveats*, `0.45`
  *use as input only*, `0.20` *do not build on, rework needed*, `0.00` **unsubstantiated**.
- The rules for authors: a claim stands alone, and an analysis is made of what it cites. kbase does
  not enforce them.
### 1.2 Edges and relations

An edge is one record in `kb-root/.index/edges.yaml`, named by `source`, `target` and `relation`.
An edge reads "source *relation* target": "A supports B" means A's strength bears on B. Every edge
is written and stored in that one direction.

| Relation | Source kinds | Target kinds | Carries | Effect on the target's strength |
|---|---|---|---|---|
| `supports` | claim, experiment | claim, analysis | `context`, `bearing`, `manually-scored`, judged hash | `bearing × source strength`, into the minimum |
| `founds` | work, axiom | claim, analysis | `context`, `bearing` | `bearing × source rigor`, into the minimum |
| `lifts` | analysis, experiment | claim, analysis | `context`, `bearing`, `manually-scored`, judged hash | `bearing × lifter strength`, maxed above the minimum |
| `refutes` | claim, analysis, experiment | claim, analysis | `context`, `bearing`, `manually-scored`, judged hash | caps at `1 − bearing × refuter strength` |
| `references` | any | any | `context` | none |
| `demoted` | ([§1.3](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)) | | | none |

- The four *scoring relations* are `supports`, `founds`, `lifts`, `refutes`. Each needs a `bearing`,
  a number in `[0, 1]` or `*pending*`. `references` and `demoted` carry no strength.
- `context` is a one-line string, `""` for none. Queries report an empty context as null.
- An edge's identity is (`relation`, `source`, `target`). Only one record holds an identity.
- The scoring edges form no cycle.
- An edge always names nodes that exist.
- The derivation: `derivation = min(rigor, every participating supports and founds contribution)`.
  With no participating premise, it is `0` for a claim and the node's own rigor for an analysis.
  Then `final = min(max(derivation, every lift), every cap)`. A pending bearing or a pending source
  strength takes its edge out. An experiment whose status is `planned` takes no part as a lifter or
  refuter, and its `supports` edge is left out.
- A node's *dependents* are the distinct targets of its outgoing scoring edges.
### 1.3 Demoted edges

The build breaks a cycle among the scoring edges by demoting one of its edges. The demoted edge
stays whole in a `demoted` record: `relation: demoted`, `source`, `target`, `context`,
`replaced-relation` (`supports`, `lifts` or `refutes`), `bearing`, `manually-scored`. The record
runs in the replaced edge's direction. Its identity is (`demoted`, `replaced-relation`, `source`,
`target`), so two demoted records can stand between one source and one target, one for each
replaced relation.

- A demoted record is outside the strength walk and the cycle check.
- Only the build writes demoted records. No public op creates one.
- Ops name a demoted record by `source`, `target` and `relation: demoted`. That name covers every
  demoted record from that source to that target.
- There are three ways to resolve one:
  - `restore-demoted` puts the replaced edge back exactly, with its relation, context, bearing and
    mark, and deletes the record. It refuses where the restored edge would close a cycle.
  - `remove-edge` with `relation: demoted` deletes the record and leaves the edge demoted away.
  - `add-edge` of the replaced edge writes the edge with the add's own values and deletes the
    record. It refuses where the edge would close a cycle.
- `verify` lists every demoted record under `findings`. A finding is not a fault.
- `edges` lists demoted records as rows of relation `demoted`, each carrying `replaced_relation`
  ([§3.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md)).
- `remove-node` deletes every demoted record that names the node.
### 1.4 Statement markers

A hosted node's location is its statement marker, `<!-- kbase-id: <id> -->`, in a leaf. No record
stores the location. The host (the leaf) and the statement are derived from where the marker sits
each time the KB is loaded, so the marker moves with the text when the author edits the leaf.

**Placement.** A marker is placed at the end of a named line, directly after the line's last
character, with no space between. A marker that ends a heading line therefore leaves the heading's
anchor slug unchanged. The same marker ends each axiom heading in `axioms.md`
([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)) and each summary entry heading ([§1.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)). Neither is a
statement marker: an axiom or work id in a marker hosts nothing. A marker is never placed on the opening or closing line of a fenced block. An
analysis paired with a claim has its marker directly after the claim's marker on the same line, with
nothing between them:
`# … Cure Is Hard<!-- kbase-id: clm-8350hb --><!-- kbase-id: ana-8350hb -->`.

**Naming a line.** Each op that places a marker takes `document` (a leaf, kb-root-relative), `line`
(1-based), and `snippet`. The `line` places the marker. The `snippet` is a short piece of text that
the line must contain. It only verifies the line, so it need not be unique in the document. The
help recommends a few distinctive words from the line. Where the line does not contain the
snippet, the op refuses and does not guess. The refusal names the nearby lines where the snippet
does appear, if any, since the file has likely shifted since the caller read it.

**Reading.**
- A heading's *section* runs from the line after the heading to the next heading as deep or
  shallower. A *paragraph* is the run of lines around a marker's line that hold text once the `>`
  prefix and markers are removed, all quoted or none, holding no heading line, and in which only the
  first line may open a list item.
- A claim's statement is the paragraph its marker sits in. Where the marker ends a heading line, the
  statement is that heading's section.
- An analysis's or an experiment's statement is the section of the nearest heading at or above its
  marker. So a claim marker and an analysis marker abutting at the end of a heading line both read
  as that heading's section.
- Two layouts the build's analysis split writes read differently:
  - An analysis marker alone on a blank line inside a blockquote. The analysis's statement is the
    whole quoted run. A claim the analysis lifts, marked in the same blockquote, has as its
    statement the text of its own marker's line before the marker.
  - A paragraph in which a claim's marker is followed by text and then by the marker of an analysis
    that lifts the claim. The claim's statement is the paragraph up to its marker; the analysis's
    is the whole paragraph.
- Markers and `>` prefixes are removed from every statement.
- A hosted node with no marker, or with more than one, is a `verify` fault (`statement-marker`). It
  does not stop the KB loading. A node with more than one marker is hosted by the first in path
  order. A node with no marker has no host, no section anchor and no summary entry.

**Anchors.** A heading's anchor slug is the one GitHub gives it, so every link resolves on GitHub:
github-slugger over the heading's text as GitHub renders it — HTML comments stripped, closing `#`s
dropped, inline markup reduced to its text (a link to its link text, emphasis and code to their
content) — trimmed. The algorithm: lowercase;
drop every character that is not a letter, mark, number, connector, space or hyphen; turn each space
into one hyphen, without collapsing runs (`A — B` → `a--b`); and suffix a heading repeated in one
document `-1`, `-2`.

A hosted node's *section anchor* is the anchor of the nearest heading at or above its marker.
### 1.5 Citations

A citation is a Markdown link in prose that refers to a node and links to where the node stands. It
takes no stance: the citing text may agree with the node, dispute it or mock it. Its link title is
the cited node's id:

- quote: `["the bound is tight"](leaf.md#bounds "ana-012xyz")`
- paraphrase: `[the bound holds](leaf.md#bounds "ana-012xyz")`, with no quotation marks.
- naming: `[The Bound Is Tight](leaf.md#bounds "ana-012xyz")`, whose link text is the node's
  title, for naming a node without quoting it.

A quote or a paraphrase cites a statement, so only a hosted node takes those forms. A work or an
axiom is cited in the naming form only.

A citation is the only way prose refers to a node. A bare node id in prose, outside code blocks,
markers and citation link titles, is a `verify` fault (`bare-id`), whose remedy is `add-citation`.
The one bulk conversion is the upconversion of a KB from an older format to `4.0.0`: `update-format`
then converts each bare id into the naming form, mechanically, since each id is an exact token
kbase knows, and its result lists each conversion (file, line, id) as a record of what changed. On
a KB already in the current format, `update-format` does not touch bare ids; it repairs no faults.

The path is relative to the citing document. The link target is, for a hosted node, its host leaf
and section anchor ([§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)); for an axiom, the anchor of its marked heading in `axioms.md` ([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)); for a work, its
entry on the root summary page ([§1.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)). The id lets kbase tell which node a citation means even after
a heading rename or a move. A citation names a node and nothing else. Any other link is an ordinary
Markdown link that the author writes.

`add-citation` checks a quote or paraphrase once, when it places it. After that, a citation is
checked like every other link: `verify`'s dead-link gate checks that the document exists and the
anchor is in it. A quote is never checked again against later text. Keeping the words faithful as
the cited text changes is the author's job.

A citation is not an edge. It does not appear in `edges`. `citations-of` lists citations.
### 1.6 Derived files: summary pages and sheets

These files are derived from the records and the markers after every write ([§2.3](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)). They are never
read back.

**Summary pages** (also called the register pages). `<volume>/kb-summary.md` exists for each volume
that hosts a node. A volume is a top-level directory of `kb-root/` holding an `index.md`.
`kb-root/kb-summary.md` exists where a node is hosted outside every volume, or a work exists.
- Each page is titled `# KB summary`. Its preamble is one line: "Generated by kbase from the KB's
  records; do not edit."
- A hosted node has its entry on the page of the volume its host lies in. Works have theirs on the
  root page. An axiom has no entry, and neither has a node with no statement marker ([§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)).
- Each entry is headed `## <title><!-- kbase-id: <id> -->`, so its anchor is the title's slug. The
  marker is annotation only.
- An entry shows the node's whole standing: its rigor (marked where manually scored), every edge
  into it with the source's title, strength and bearing, the demoted records into it, the rationale,
  and for a claim or an analysis its derived strength line and trace.
  Entries come in the reading order of their markers, then works by label.
- Links between entries are computed from each node's derived page and slug.
- A page no node calls for is removed.

**Sheets.** These are drawn claim graphs, as SVG through Graphviz `dot`: `kb-root/claim-graph.svg`
always, and, where two or more volumes hold nodes, `kb-root/claim-graph-digest.svg` and
`<volume>/claim-graph.svg` for each volume holding a node. Without `dot` on PATH, nothing is drawn
and every sheet is left standing. A placeholder `kb-root/claim-graph.svg` is written only where none
exists, and the result says `sheet: not-drawn` with the cause under `dot`. Sheets are drawn for
people. The queries answer anything a sheet shows.
### 1.7 Maintained-block documents

`kb-root/AGENTS.md`, `kb-root/CONVENTIONS.md` and `kb-root/README.md` each hold one *maintained
block*, a block that kbase maintains. The block runs from a line `<!-- vvv-kbase … kbase-vvv -->`
to a line `<!-- ^^^-kbase … kbase-^^^ -->`. Each marker line is recognised by its two ends alone, so the text
between the ends may differ between releases.

- In a document with one ordered pair of marker lines, kbase rewrites the span between them and
  nothing else. The author's text outside the pair survives.
- An absent `AGENTS.md` or `CONVENTIONS.md` is written whole, fences included. `README.md` is
  written only by a build, whose overview passage it carries; `init` and `update-format` never
  create one.
- A document without the pair is left untouched and reported. That is not a refusal.
- A pair with nothing between it is valid. The next `update-format` fills it.
- The blocks are rewritten on every `update-format` run and when `init` or the build writes the
  document.
- `kb-root/CLAUDE.md` is a redirect to `AGENTS.md`. A write refuses where `CLAUDE.md` stands and is
  not the redirect (`check: agents-redirect`).
### 1.8 Format version

`kb-root/.index/format.yaml` holds `kb-format: <major>.<minor>.<patch>`. The current version is
`4.0.0`.

**`4.0.0` is a whole-number format bump: a compatibility break.** Everything below ships in
that one major version. None of them ships as a `3.x` minor or patch, and none is split across
versions. The break runs both ways:
- a kbase release that predates `4.0.0` refuses a `4.0.0` KB as newer than it reads, and nothing
  downgrades;
- a `4.0.0` kbase reads an older KB only through in-memory conversion, and refuses every write
  but `update-format` until the KB is converted ([§2.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)).

`4.0.0` defines:
- the `kbase-id` marker and its no-space attachment;
- `kb-summary.md`;
- the abutting layout for paired analysis markers;
- the node id in each citation's link title, and the naming form;
- the bare-id fault;
- the judged hash (`judged-hash`) on scored records;
- an experiment's `status` vocabulary, `run` | `planned`;
- the build's records under `kb-root/.build/`;
- no `no-edge` field on claims and analyses, and `references` from every kind to every kind;
- no `strengthen-by` field on claims;
- a minted id on every node kind, with a work's citation key and an axiom's label as the record
  field `label`, and the id marker on each axiom heading.

What the conversion to `4.0.0` does is listed under `update-format` ([§3.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)).

Older formats (`1.0.0`, `2.0.x`, `3.0.0`) are read through in-memory conversion ([§2.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)).
## 2. Conventions every op follows

Sections:

- [§2.1 Naming](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.1-naming.md)
- [§2.2 Write batches](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)
- [§2.3 The derivation after every write](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)
- [§2.4 Load errors](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)
- [§2.5 The domain rule](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md)
- [§2.6 Format conversion](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)
- [§2.7 Provider names](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)
- [§2.8 Results and exit codes](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)
- [§2.9 Manually scored, and the judged hash](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)
- [§2.10 Existence](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)
### 2.1 Naming

- `id` names a node wherever the node itself is the subject: `show`, `edit-<kind>`, `remove-node`,
  `move-node`, `citations-of`, `edges`, `trace` and `add-citation`'s cited node. `score` names its
  item `node` or `edge`, since it takes either.
- An edge is named by `source`, `target` and `relation`, everywhere. No edge end is ever keyed `id`.
- Every edge row a query returns uses the op keys `source`, `target`, `relation`, `bearing` and
  `context`, so a row's values name the edge for any edge op. Its other keys (`manually_scored`,
  `replaced_relation`, and `trace`'s `depth`, `strength` and `contribution`), and any op key whose
  value is null, are read-only facts: drop them before copying the rest into `add-edge` or
  `edit-edge`. Copy `bearing` only where the row is manually scored or the relation is `founds`:
  a bearing given marks the edge manually scored ([§2.9](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)), so copying a model-judged one would
  pin it. Into `remove-edge` and `restore-demoted`, copy only `source`, `target` and `relation`.
  Every op's key set is closed.
- The verbs are `add`, `edit` and `remove`. An op is split by node kind exactly when its fields
  differ by kind. The id prefix tells the caller the kind.
- A node is named by its id only. A `label` or a `title` is a field. `find` locates a node by
  either ([§3.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/find.md)).
- The model-configuration vocabulary:
  - a **role** is the slot a model fills: `decision`, `text`, `scoring`;
  - a **provider** is a `providers.toml` table;
  - a **model reference** is `<provider>/<model>`, split at the first `/`.
- An **entry** is one item of a write batch. An item of a summary page is a **summary entry**.
### 2.2 Write batches

Every write goes through a batch. That covers every op of the `write` family, on every surface.

- **Shape.** A batch is an ordered list of entries under `entry`. Each entry names its op under
  `operation` and carries that op's keys. A batch may mix ops. `dry-run` and `commit` sit beside
  `entry` as batch options. An empty batch is valid: it writes no record and runs the derivation
  ([§2.3](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)), which brings the derived files up to date after an author's leaf edits.
- **Surfaces.** Over MCP, the `write` tool takes a batch. At the CLI, `kbase write` takes a mixed
  batch, each entry naming its op, with `--dry-run` and `--commit`. Each op also has its own
  subcommand (`kbase add-edge`, …), which takes a batch of that op alone, with the same options;
  its entries carry no `operation`, since the subcommand names the op. `kbase <op> --help` is that op's help. Both CLI forms read the batch from the file `--values`
  names, or from stdin where `--values` is absent.
- **Order.** Entries apply in the order given. Each entry is checked against the records as the
  earlier entries left them. kbase never reorders a batch. The caller orders it so that each entry
  is valid where it stands.
- **No references between entries.** No entry refers to an id that another entry of the same batch
  mints, and every add mints its node's id, for every kind. Adding nodes and their edges takes two
  batches: the first adds the nodes, and its result
  gives each minted id beside the entry that minted it. The second adds the edges. Between the two
  batches the KB is consistent, since a node with no edges is a valid node.
- **All or nothing.** A batch that fails anywhere writes nothing. The batch runs like this:
  1. Read the whole KB into memory.
  2. Apply every entry in memory.
  3. Compute the derivation ([§2.3](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)).
  4. Write every changed file once. Each file is written temp-then-rename. A crash between two
     files' writes is not atomic on disk; git covers that case.
- **Two error phases.**
  1. *Shape.* Before anything applies, every entry's shape is checked: an unknown op, unknown or
     missing keys, value types, and values outside a closed vocabulary (`relation`, `status`, …).
     Every shape error is reported at once.
  2. *Apply.* Entries apply in order, each checked in full against the records as they stand at
     that point. The batch stops at the first refusal. An entry that would leave the records invalid
     is refused, even if a later entry would repair it. The refusal's remedy names the order that
     works. There is no separate end-of-batch check.
- **Refusal items** carry `check`, `path`, `entry` (the entry's 1-based position in the batch),
  `key`, `line`, `column`, `allowed`, `remedy` and `detail`. A key is left out where it does not
  apply. `detail` is always present.
- **Dry run.** `dry-run: true` does everything except write. Its result is identical to a real
  run's, refusals and removed edges included, and it says `dry-run: true`. It never commits.
- **`commit`.** `commit: true` makes git the batch's undo. It is never on by default, and no
  configuration sets it. Every commit it makes covers `kb-root/` only. A committing batch runs:
  1. Apply every entry in memory. On any error, return it, with nothing written and nothing
     committed.
  2. Pre-flight commits, before writing:
     - where `kb-root/` outside `.index/` has uncommitted changes, commit them alone, label
       `kb-write: kb-root/ changes outside .index/ made before this batch`;
     - where `kb-root/.index/` has uncommitted changes, commit them alone, label
       `kb-write: record changes made before this batch`.

     A commit's *label* is its subject line. A label says where the changes are, not who made
     them: changes left by an earlier batch run without `commit` land in the commit for wherever
     they lie. A pre-flight commit that fails,
     for example on a hook, stops the batch before it writes: outcome `failed`, nothing written,
     since writing then would mix the batch into uncommitted changes.
  3. Write the batch.
  4. Commit the batch's changes, inside and outside `.index/`, label `kb-write: <ops> <ids>`.

  Reverting the batch commit undoes exactly that batch and never the author's work. The result
  lists each commit made and what it took. A batch commit that fails, for example on a hook, leaves
  the batch written; the result says the batch is uncommitted and why, with outcome `done`. A
  committing batch outside a git worktree is refused (`check: worktree`, remedy `git init`). The
  help states that a committing batch commits the whole of `kb-root/`, and that a batch without it
  has no safety net.
- **Locks.** A batch holds the KB write lock from loading the KB to the result. A writer that cannot
  take the lock within 30 s gets `retry` (`check: lock`). While a build or a score run holds the
  operation lock, a batch is refused at once (`check: lock`).
- **Values.** Rigors and bearings are numbers in `[0, 1]` or the string `"*pending*"`, quoted in
  YAML, where a bare `*` begins an alias. A value above `1.0` is
  refused. A prose value (`title`, `rationale`, `context`) holding a line break (CR or LF), U+0008
  or U+000C is refused. Any other whitespace run collapses to one space, and the ends are trimmed.
  The `unchanged` test compares the collapsed value.
- **Result.** The result carries:
  - `outcome`, `kb-root`, `dry-run`;
  - `entries`, one per batch entry in order, each with `entry`, `operation`, `outcome` (`done` or
    `unchanged`) and its op's own keys;
  - `written` and `removed`, as paths;
  - `sheet`, and `dot` where the sheets were not drawn;
  - `commits`, each with `commit`, `label` and `paths`, `[]` without `commit`;
  - `uncommitted`, a detail line, present only where the batch commit failed;
  - on `refused`, `refusals`.

  The batch outcome is `unchanged` where every entry is `unchanged` and the derivation wrote
  nothing, and `done` otherwise.
### 2.3 The derivation after every write

Every write batch is followed by the derivation: strengths, the summary pages and the sheets. No
public flag skips it. It is part of the batch's all-or-nothing: the records and the derived files
are computed in memory and written together, so the records are never written while their derived
files are stale. The derivation does not repair or convert the KB. `score`'s landings and the
build's writes are followed by the derivation the same way.
### 2.4 Load errors

Every command that loads the KB refuses a KB whose load fails, naming each fault with its file and
record (`check: metadata`), with no remedy op. The faults are:
- a record file that does not parse;
- an unknown or missing field;
- a value of the wrong type or outside its domain, a rigor above `1.0` included;
- an id outside its kind's grammar;
- one node id on two records;
- a pairing that no relation joins;
- one edge identity on two records;
- an edge naming a node no record carries;
- a cycle among the scoring edges.

A record file missing from a stamped KB is refused (`check: index`, remedy "restore it from git").
kbase does not resolve these faults in memory. Recovery is the user's work in git, guided by the
load error: return the record files to a consistent commit, for example the one before a merge, and
reapply the other side's changes through write ops. Three of these faults have no supported
producer: one edge identity on two records, an edge naming a missing node, and a cycle. The write
ops refuse all three, and the build demotes its own cycles.
### 2.5 The domain rule

`kb-root/` and everything below it is kbase's domain. kbase checks and commits nothing outside it:
not in a batch's `commit`, and not in `update-format`'s clean check. An output the caller sends
elsewhere, such as `export-latex`'s `--out`, is not kbase state.

The build's records (`kb-build-node-pass.yaml`, `kb-build-classification.yaml`,
`kb-build-unmarked.yaml`, `kb-build-cull.yaml`) live in `kb-root/.build/`, so the build's stage
commits stay within `kb-root/`. They are committed at every stage boundary, so a rewind restores
them with the KB, and they stay after the build finishes.
### 2.6 Format conversion

- A KB stamped in an older format is read through an in-memory conversion. Every read works on it
  and answers as converted, carrying `migration` (`from`, `to`, `note`, `warnings`). Nothing is
  written, and nothing is cached between loads. The conversion is deterministic: the same files
  convert to the same result on every load and in `update-format`, minted ids included.
- While a conversion is pending, every write except `update-format` is refused (`check: index`,
  remedy `kbase update-format`): write batches, `build` over the KB, `score`, and every op that
  writes. A format change never rides along with any other write.
- A KB stamped newer than kbase is refused by every command that loads it (`check: kb-format`,
  remedy "update kbase").
- After upgrading kbase, run `update-format`. It converts the format, re-derives every derived file
  and rewrites the maintained blocks, whether or not the format changed.
### 2.7 Provider names

A provider name is case-insensitive everywhere. Every input naming a provider is folded to
lowercase without complaint:
- `configure-providers`;
- `auth add` and `auth remove`;
- `--provider`;
- the `<provider>` part of a model reference;
- each table name read from `providers.toml`.

Every output shows the lowercase form. Two `providers.toml` tables whose names differ only in case
are a configuration load error. A provider name is ASCII letters, digits, `_` and `-`, not starting
with `-`.
### 2.8 Results and exit codes

Every command writes one YAML document to stdout (over MCP, the same document as text and as
structured content) and nothing else; stderr is for people. The document is:
- `outcome`;
- `kb-root`, on every command that resolves a KB;
- `migration`, on reads of an older-format KB;
- the command's own keys;
- one closing key chosen by the outcome: `refusals` for `refused` and `retry`, `failures` for
  `failed`, `cancelled` for `cancelled`.

Keys are kebab-case. Query data under `results` keeps its own snake_case names (`manually_scored`,
`replaced_relation`). `detail` is one sentence.

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | success (`bounded`: a build stopped at `--through`) |
| `refused` | 1 | wrong input or KB state; nothing written |
| `retry` | 2 | a lock is held; re-issue the identical call later |
| `failed` | 3 | a defect in the tool, its input or the environment |
| `cancelled` | 4 | stopped on request; resumable |

Over MCP, `isError` is set exactly when the exit code is not 0.

**Check values.** A refusal item's `check` names its condition. The values this document uses:

| `check` | Condition |
|---|---|
| `agents-redirect` | `kb-root/CLAUDE.md` stands and is not the redirect ([§1.7](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)) |
| `conversion` | a conversion cannot carry a file's content ([§3.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)) |
| `dependency-cycle` | the entry would close a cycle among the scoring edges |
| `dirty-paths` | uncommitted changes in the paths the command needs clean |
| `index` | a record file missing from a KB, or a write while a conversion is pending ([§2.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)) |
| `kb-format` | a KB stamped newer than kbase |
| `kb-root` | `kb-root/` exists where `init` or a fresh `build` would create it |
| `lock` | the write lock or the operation lock is held, or no run holds the store `cancel` names |
| `metadata` | a load error ([§2.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)) |
| `provider` | a provider or model reference the configuration does not resolve |
| `state-dir` | a state store kbase cannot use: inside `kb-root/`, or written by a newer kbase |
| `symlink` | a KB document that is a symbolic link |
| `usage` | an argument missing, unknown, out of domain, or conflicting with another |
| `error` | a failure item: a defect in the tool, its input or the environment (outcome `failed`) |
| `trail` | the trail records a stage over an unrecorded earlier one |
| `worktree` | outside a git worktree, or a `project-dir` that is not a worktree root, or a repository root whose `.git` git does not read as the worktree root |
| `dead-link`, `unknown-id`, `bare-id`, `frontmatter`, `index-freshness`, `statement-marker`, `axiom-heading`, `register-freshness` | `verify` faults ([§3.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)) |
| `demoted` | a `verify` finding, not a fault ([§1.3](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)) |

**Lists.** A list query (`find`, `edges`, `trace`, `citations-of`) always returns its complete
answer, with `count` and `results`. There is no default cut, no paging and no `offset`. A smaller
answer comes from a narrower question, a filter that means something. `edges` and `citations-of`
return a fixed order, stated with each. `find` and `trace` take `sort` and `limit`. `limit N`, a
single integer, is allowed only beside an explicit `sort`, and returns the first N of that order:
the N weakest, the N strongest. A negative N returns the last N, still in the sort's order, as
Python slicing does. `limit` without `sort` is refused, and so is `limit: 0`.

**Sorting.** `find` and `trace` sort the same way. `sort` names a key, and `order` (`ascending` |
`descending`) sets the direction; every key defaults to `ascending`. Every key is
available both ways, so `limit` can take either end of any ranking. A pending value sorts below
every number, for ordering only; its row still shows `*pending*`. A null, a value that does not
apply to the row, sorts last in either order.
### 2.9 Manually scored, and the judged hash

- `manually-scored` sits on claim, analysis and experiment records and on `supports`, `lifts` and
  `refutes` edges. A work's rigor, an axiom and a `founds` edge carry no mark, since no scoring pass
  reaches them.
- A public op writes the mark `true` on a rigor or a bearing the caller supplied, `*pending*`
  included, and `false` on one it writes `*pending*` because the caller left it out. The ops are
  `add-<kind>`, `edit-<kind>`, `add-edge` and `edit-edge`.
- An op leaves the mark as it stands on a value it does not write. Re-asserting the current value of
  a record marked `false` marks it `true`, outcome `done`.
- A scoring run's landings write `false`. No key or argument sets the mark directly.
- Each scored record carries the hash of what its value was last judged against: the item's text
  plus the ask template's digest. For a node, the text is its statement; for an edge, the
  statements of its two ends. The model is not part of the hash.
  - An item is *dirty* when the current hash differs from the recorded one. A value never judged is
    dirty.
  - A manually set value records its hash too.
  - The field is `judged-hash`, the last field of claim, analysis and experiment records and of
    `supports`, `lifts` and `refutes` edge records. It is required on every one of them; a record
    without it is a load error ([§2.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)). Where never judged it is all zeros, which no computed hash
    matches, so the item is dirty.
### 2.10 Existence

Every op that names something existing first looks it up, and is refused where the lookup fails,
with one error everywhere: "no node `<id>`", or "no edge `<relation>` `<source>` → `<target>`".
This covers every op naming an existing node, edge, demoted record, provider entry or stored key:
among them `show`, `edges`, `trace`, `citations-of`, the edits, `move-node`, the removes,
`restore-demoted` (which looks up a demoted record), `add-citation`'s cited node, `score`'s `node`
and `edge`, `configure-providers --remove` and `auth remove`. A lookup matches exactly; there is no
near-miss matching. A removal is a lookup that can fail, exactly as `show`'s is. Nothing records
what was removed, so a repeated removal and a removal of something that never existed cannot be
told apart; both are refused. The remedy points at the query that lists what does exist.

An add looks up first too, and is refused where the thing is found: "node exists: `<id>`", or
"edge exists". Where an edge exists with other values, the remedy is `edit-edge`. Nothing is
adopted.

An edit whose value already stands reports `unchanged`. The thing exists, and the caller asked for
a state the KB is already in. Each batch entry reports its own outcome ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)), so an `unchanged`
entry is never hidden in a `done` batch.
## 3. Operations

The surfaces:
- **write op**: an entry of a write batch. Over MCP, an op of the `write` tool. At the CLI, its own
  subcommand, or an entry of a `kbase write` batch ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)).
- **query op**: over MCP, an op of the `query` tool, taking `operation` and `arguments`. At the
  CLI, its own subcommand.
- **tool + CLI**: an MCP tool of its own and a CLI subcommand, with the same arguments.
- **CLI only**.

Every MCP tool also takes `project-dir` ([§4.3](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md)). Every CLI command takes the global flags
`--project-dir`, `--config-dir`, `--log-file` and `--log-level` (debug|info|warn|error, default
warn). Each op's help is described in [§4.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md).

| Family | Ops | Count |
|---|---|---|
| Node writes | `add-claim`, `add-analysis`, `add-experiment`, `add-work`, `add-axiom`, `edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work`, `edit-axiom`, `remove-node`, `move-node` | 12 |
| Edge writes | `add-edge`, `edit-edge`, `remove-edge`, `restore-demoted` | 4 |
| Citations | `add-citation`, `citations-of` | 2 |
| Queries | `find`, `show`, `edges`, `trace`, `stats` | 5 |
| Runs and run control | `build`, `score`, `status`, `cancel`, `wait`, `build-trail` | 6 |
| Checks and output | `verify`, `update-format`, `export-latex` | 3 |
| Setup and configuration | `init`, `configure-providers`, `configure-models`, `models`, `show-config`, `probe`, `auth add`, `auth list`, `auth remove` | 9 |
| Server | `mcp` | 1 |
| **Total** | | **42** |

MCP tools (16): `write`, `query`, `build`, `score`, `status`, `cancel`, `build-trail`, `verify`,
`update-format`, `export-latex`, `init`, `configure-providers`, `configure-models`, `models`,
`show-config`, `probe`.

The read-only tools are `query`, `status`, `build-trail`, `verify`, `models`, `show-config` and
`probe`. Each carries the read-only annotation.

Groups:

- [§3.1 Node writes](#31-node-writes)
- [§3.2 Edge writes](#32-edge-writes)
- [§3.3 Citations](#33-citations)
- [§3.4 Queries](#34-queries)
- [§3.5 Runs and run control](#35-runs-and-run-control)
- [§3.6 Checks and output](#36-checks-and-output)
- [§3.7 Setup and configuration](#37-setup-and-configuration)
- [§3.8 The server](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.8-server/index.md)
### 3.1 Node writes

Every add, edit and remove is a write op, and every one follows [§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) and [§2.3](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md).

**Existing nodes.** An add of a node that already stands is refused, naming its id ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)). The
identity an add checks is as follows.
- A node exists where a node of that kind carries that title anywhere in the KB ([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)).
- A work also exists where another work carries that `label`, and an axiom where another axiom
  carries that `label`.
- Entries apply in order ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)), so a batch adding one node twice is refused at the repeat, "node
  exists" ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).

Ops:

- [`add-claim`, `add-analysis`, `add-experiment` (write ops)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-claim-add-analysis-add-experiment.md)
- [`add-work` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-work.md)
- [`add-axiom` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-axiom.md)
- [`edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work`, `edit-axiom` (write ops)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/edit-claim-edit-analysis-edit-experiment-edit-work-edit-axiom.md)
- [`remove-node` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/remove-node.md)
- [`move-node` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/move-node.md)
#### `add-claim`, `add-analysis`, `add-experiment` (write ops)

These mint an id, write the node record, and place its statement marker at the named line ([§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)).
They carry no edges.

| Key | Type | `add-claim` | `add-analysis` | `add-experiment` |
|---|---|---|---|---|
| `title` | string, one line | required | required | required |
| `rationale` | string | required | required | — |
| `status` | `run` \| `planned` | — | — | required |
| `rigor` | number in [0,1] \| `*pending*` | optional (default `*pending*`) | optional | optional |
| `document` | kb-root-relative leaf path | required | required | required |
| `line` | integer ≥ 1 | required | required | required |
| `snippet` | string | required | required | required |

A hosted node always has its marker: a node with no marker is a `verify` fault, so placing it is
part of the smallest valid add.

- **Result** (per entry): `id`, minted.
- **Refusals**:
  - a missing `document`, `line` or `snippet`: shape, naming the missing key;
  - `document` not a leaf of the KB: remedy, name a leaf;
  - `line` past the document's end;
  - the line does not contain `snippet`: remedy, re-read the document, with the nearby lines where
    the snippet appears;
  - `line` is a fenced block's opening or closing line: remedy, name another line;
  - `rigor` outside `[0, 1] ∪ {*pending*}`: `allowed` the range;
  - `status` outside `run`, `planned`: `allowed`;
  - a prose value holding a line break ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md), Values);
  - a pending conversion ([§2.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)), a lock ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)), and a `CLAUDE.md` that is not the redirect ([§1.7](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)).
#### `add-work` (write op)

This records an external work and mints its id, `wrk-<minted>`. Its citation key is its `label`
([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)). The work is shown on the root summary page.

- **Keys**: `label` (the citation key, required), `title` (required), `rationale` (required),
  `rigor` (optional; default `*pending*`; no manual mark).
- **Result**: `id`, minted.
- **Refusals**: a `label` that is not a citation key (it must open and close alphanumeric,
  [§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)); a work of that label or that title that already stands ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)); `rigor` out of
  domain.
#### `add-axiom` (write op)

This records an axiom and mints its id, `axm-<minted>`. It writes the record and gives the axiom
its heading in `axioms.md`, `### Axiom <label>: <title><!-- kbase-id: <id> -->`
([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)). Where one unmarked heading of exactly that text stands, it
gets the marker. Otherwise the heading is appended. The add never leaves two headings of one
text: a second heading would take the `-1` anchor, and the author's prose and any links would
stay with the first. A record without its marked heading is a `verify` fault, so the heading is
part of the smallest valid add. The axiom's prose under the heading is the author's. `edit-axiom`
rewrites the heading on a label or title change. `remove-node` removes the heading with the
record and leaves the prose under it to the author.

- **Keys**: `label` (one line, any text, required), `title` (required).
- **Result**: `id`, minted; `heading`, `marked` or `appended`.
- **Refusals**: an axiom of that label or that title that already stands ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)); more than one
  unmarked heading of that text in `axioms.md`, naming their lines, since kbase does not guess
  which one the axiom is; a prose value holding a line break ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md), Values).
#### `edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work`, `edit-axiom` (write ops)

These change the record fields given. A field given replaces that field's value whole. At least
one field is required. A title change, or an axiom's label change, also keeps every link to the
node live, in the same write:
- `edit-axiom` rewrites the axiom's heading in `axioms.md`, keeping its marker (see `add-axiom`);
- where the node's anchor moves (an axiom's heading, a work's summary entry, a hosted node's
  summary entry,
  [§1.5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)), every link into the old anchor is re-pointed: each
  citation of the node (found by the id in its link title, as `citations-of` finds them) and
  each ordinary link into that heading. Only the link target changes, as `move-node` re-points;
- a citation whose link text is the old title, the naming form, gets the new title as its text.
  Any other link text is a quote or a paraphrase, the author's words, and stays.
- where a rewritten naming-form citation sits in a heading and moves an anchor — that heading's
  own, or a later same-text heading's `-1`, `-2` suffix — every link whose destination resolves
  to a moved heading, citation or not, is re-pointed to its new anchor.

An edit writes no other document.

| Op | `id` | Fields it may change |
|---|---|---|
| `edit-claim` | `clm-…`, required | `title`, `rigor`, `rationale` |
| `edit-analysis` | `ana-…`, required | `title`, `rigor`, `rationale` |
| `edit-experiment` | `exp-…`, required | `title`, `status`, `rigor` |
| `edit-work` | `wrk-…`, required | `label`, `title`, `rigor`, `rationale` |
| `edit-axiom` | `axm-…`, required | `label`, `title` |

- Setting `rigor` on a claim, an analysis or an experiment marks it manually scored ([§2.9](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)).
- A work's label change rewrites, in the same write, every reference that carries the old key
  as a key, so that no reference dangles and the export still finds the work's entry:
  - each key token of each citation span's `data-cites` (`<span class="citation"
    data-cites="…">`, the imported LaTeX citations), and the span's text where it is the
    key-only rendering, the parenthesised key list;
  - each reference-list entry's `id="ref-<key>"`, and each link into `#ref-<key>`.

  kbase citations need no rewrite, since they name the node by id and the naming form's text is
  the title. Prose that mentions the key as words, such as a rationale, is the author's and stays.
- **Result** (an `edit-work` that changes `label`): `rewritten`, each rewritten reference's `path`
  and `line`.
- **Result**: `id`; `unchanged` where every value given already stands (and no mark changes).
- **Refusals**:
  - an `id` no node carries;
  - an `id` of another kind: remedy, the op for its kind, named from the prefix;
  - no field given: shape;
  - a field the kind does not carry: shape, `allowed` the kind's fields;
  - a value out of domain;
  - a `title` or a `label` another node of the same kind carries ([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)), naming that node's id;
  - a work `label` that is not a citation key ([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md));
  - a work `label` already used as a citation key in a `data-cites` span or a reference-list entry,
    naming `path:line`, since the rewrite would merge two references into one;
  - a `title` with an unbalanced `[` or `]` where a naming-form citation of the node would carry it,
    so that it would not read as one link, naming the citing `path:line`.
#### `remove-node` (write op)

This removes a node and every record naming it: the node record, its statement marker, every edge
naming it at either end, and every demoted record naming it. It takes `id` only, for every kind. No
undo is built; git restores what a removal took, and a dry run ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)) previews it.

- **Keys**: `id` (required).
- **Result**: `id`; `removed-edges`, every edge record removed, each named by `source`, `target`
  and `relation`, as a full edge row (`bearing` and `context` included), so an edge can be re-added
  onto a replacement node without reading `edges` before the removal. A node that does not stand
  is refused ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).
- **Refusals**: while the id is still mentioned, at the point the entry applies, by:
  - a citation of the node, found by the id in its link title (as `citations-of` finds it), named
    `path:line`. Remedy: edit that prose.
  - for a work, a `data-cites` span carrying its label where no reference-list entry has that key,
    named `path:line`. The work record is that span's only bibliography entry, so without it the
    export prints the span as plain text. Remedy: edit that prose.
  - a prose field of a record that survives the removal (`title`, `rationale`, an edge's
    `context`) holding a citation of the node or its bare id, named by record and field. Remedy:
    an `edit-<kind>` or `edit-edge` entry earlier in the same batch; for a demoted record's
    `context`, `restore-demoted` then `edit-edge`, or `remove-edge` on the demoted record.
#### `move-node` (write op)

This moves a hosted node's statement marker to the end of the named line ([§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)), taking it from
where it stood. It is one op for every kind that has a marker. In the same entry it re-points every
citation of the node (found by the node id in each link title, [§1.5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)) to the node's new path and
anchor, changing only the link target, never the citing words. The node's citations are links
kbase computed from its host, so re-pointing them is part of the smallest consistent move. A
citation an author's own edit breaks, such as by a heading rename, is a broken link, which
`verify`'s dead-link check reports like any other.

- **Keys**: `id` (required), `document`, `line`, `snippet` (all required).
- **Result**: `id`; `unchanged` where the marker already ends that line.
- **Refusals**: a work or an axiom (no statement marker); a node with no marker to move (a
  `statement-marker` fault, [§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)); an `id` no node carries; and the placement refusals of the add
  ops.
### 3.2 Edge writes

Ops:

- [`add-edge` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/add-edge.md)
- [`edit-edge` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/edit-edge.md)
- [`remove-edge` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/remove-edge.md)
- [`restore-demoted` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/restore-demoted.md)
#### `add-edge` (write op)

This adds one edge. Many edges are many entries.

- **Keys**:
  - `source` (node id, required);
  - `target` (node id, required);
  - `relation` (`supports` | `founds` | `lifts` | `refutes` | `references`, required);
  - `bearing` (number in [0,1] | `*pending*`, optional; scoring relations only; default
    `*pending*`);
  - `context` (string, optional).
- Over a standing edge of that identity, the entry is refused, "edge exists", whatever its values.
  Changing them is `edit-edge`'s job ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).
- Over a demoted record of that relation from source to target, the add writes the edge with its own
  values and deletes the record.
- **Result**: `source`, `target`, `relation`.
- **Refusals**:
  - a `source` or `target` no node carries;
  - a pairing the relation does not join ([§1.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)), with `allowed` the relations that join these kinds;
  - `relation: demoted`: remedy `restore-demoted`;
  - a `bearing` on `references`;
  - a bearing out of domain;
  - an edge that would close a cycle (`check: dependency-cycle`, the detail naming the cycle in edge
    direction): remedy, remove another edge of the cycle first.
#### `edit-edge` (write op)

This changes a standing edge's `bearing`, its `context`, or both.

- **Keys**: `source`, `target`, `relation` (required); `bearing`, `context` (at least one).
- A bearing given on `supports`, `lifts` or `refutes` marks the edge manually scored ([§2.9](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)).
- **Result**: `source`, `target`, `relation`; `unchanged` where the values already stand.
- **Refusals**:
  - no edge of that identity ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md));
  - `relation: demoted`: remedy `restore-demoted`, or `remove-edge`;
  - a `bearing` on `references`;
  - a bearing out of domain;
  - neither field given: shape.
#### `remove-edge` (write op)

This removes the edge record that the entry names. With `relation: demoted`, it removes every
demoted record from source to target, leaving the edge demoted away. Every other record stays byte
for byte as it was.

- **Keys**: `source`, `target`, `relation` (any of the six; required).
- **Result**: `source`, `target`, `relation`.
- **Refusals**: no such record ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)); a `source` or `target` no node carries; a scoring relation
  that does not join the two nodes' kinds.
#### `restore-demoted` (write op)

This puts back exactly the edge each demoted record from source to target replaced, with its
relation, context, bearing and mark, and deletes the record.

- **Keys**: `source`, `target`, `relation` (must be `demoted`; all required).
- **Result**: `source`, `target`, `relation`, `restored` (the relations put back). No demoted
  record from source to target is refused ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).
- **Refusals**:
  - any relation other than `demoted`;
  - a restore that would close a cycle (`check: dependency-cycle`, naming the cycle): remedy,
    remove another edge of the cycle first.
### 3.3 Citations

Ops:

- [`add-citation` (write op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/add-citation.md)
- [`citations-of` (query op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/citations-of.md)
#### `add-citation` (write op)

The author writes the cited text in the citing document, then calls this op. kbase finds the cited
node's link target ([§1.5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)), computes the relative path and the anchor, and replaces `snippet` on the
named line with the citation link.

- **Keys**:
  - `document` (the citing document, kb-root-relative, required);
  - `line` (required);
  - `snippet` (required): exactly the text on that line that becomes the link text. Unlike the
    marker ops' `snippet`, it is the span that gets replaced, not just a check, so it must occur
    exactly once on the line. A `snippet` occurring more than once there is refused, naming each
    column, since kbase does not guess which span the caller meant;
  - `id` (the cited node, required);
  - `quote` (optional). Without `quote`, `snippet` is a verbatim quote and must appear in the cited
    node's statement; it becomes `["<snippet>"](<path>#<anchor> "<id>")`. With `quote`, `snippet`
    is a paraphrase, and `quote` is the passage paraphrased, which must appear verbatim in the cited
    statement; it becomes `[<snippet>](<path>#<anchor> "<id>")`. Where `snippet` is the cited
    node's id itself, it becomes the naming form, `[<title>](<path>#<anchor> "<id>")`. The author
    writes the id where the reference belongs and converts it. A work or an axiom takes the
    naming form only ([§1.5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)).
- A citation written into a heading that moves an anchor — that heading's own, or a later
  same-text heading's `-1`, `-2` suffix — re-points every link whose destination resolves to a
  moved heading, citation or not, in the same write.
- **Result**: `document`, `line`, `id`, `link` (the text written).
- **Refusals**:
  - the line does not contain `snippet`: remedy, re-read the document, with the nearby lines where
    the snippet appears;
  - the quote (`snippet`, or `quote` where given) does not appear verbatim in the cited statement;
  - a quote or a paraphrase of a work or an axiom, which have no statement: remedy, the naming
    form;
  - a hosted node with no marker (a `statement-marker` fault, [§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)), which has no link target;
  - an `id` no node carries;
  - `document` not a document of the KB;
  - a link that would not read as one link: an unbalanced `[` or `]` in `snippet` (refused at
    `snippet`) or, for the naming form, in the cited node's title (refused at `id`).
#### `citations-of` (query op)

This lists where prose cites a node, ordered by path, then line. It reads each citation link by
the node id in its link title.

- **Arguments**: `id` (required).
- **Results** rows: `path` (the citing document), `line`, `text` (the link text as written). `move-node` keeps the node's own citations current ([§3.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/move-node.md)). The only check on a
  citation after it is placed is that its link resolves (`verify`'s dead-link check).
- **Refusals**: an `id` no node carries.
### 3.4 Queries

Every query reads the KB and writes nothing. It takes no lock. An older-format KB answers as
converted ([§2.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)), and a load error refuses it ([§2.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)). Every query's data sits under `results`: a
list with `count` for the list queries, a mapping for `show` and `stats`.

Ops:

- [`find` (query op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/find.md)
- [`show` (query op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/show.md)
- [`edges` (query op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md)
- [`trace` (query op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/trace.md)
- [`stats` (query op)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/stats.md)
#### `find` (query op)

This lists the nodes matching its filters, across every kind. With no filter it lists every node.
`find` locates nodes; `show` reads one.

- **Arguments**, all optional:
  - `kind` (`claim` | `analysis` | `experiment` | `work` | `axiom`, repeatable);
  - `title` (text, matched case-insensitively as a substring);
  - `label` (text, matched case-insensitively as a substring, as `title` is; only works and axioms
    carry one);
  - `path`: a kb-root-relative directory, matching every node hosted under it, or a leaf, matching
    the nodes it hosts. `""` and `.` name `kb-root/`, matching every hosted node. Works and axioms
    never match it.
  - `min` (strength at or above);
  - `max` (strength strictly below);
  - `pending` (nodes with no strength yet; refused beside `min` or `max`);
  - `min-dependents` (integer);
  - `sort`: `id` (default), `strength` or `dependents`, ties broken by strength, then id; `order`
    ([§2.8](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md));
  - `limit` (only beside an explicit `sort`, [§2.8](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)).
- `strength` is [§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)'s for every kind: derived for a claim or an analysis, the rigor for a work, an
  experiment or an axiom. `min`, `max` and `pending` apply to every kind. `min` and `max` compare
  scored strengths only, so a pending node never matches either, and `min: 0` keeps exactly the
  scored nodes.
- **Results** rows: `id`, `node_type`, `title`, `label` (null for hosted kinds), `host` (the host leaf's path; null for works, axioms and
  a node with no statement marker), `strength` (as above, `*pending*` where pending), and
  `dependents` ([§1.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)).
  `find` sorts and filters by strength and dependents, so its rows show both; that also makes `find` the full list behind
  `stats`' `highest_leverage`.
- **Refusals**: `pending` beside `min` or `max` (usage); a value outside a vocabulary.
#### `show` (query op)

This returns the full record of one node, of any kind.

- **Arguments**: `id` (required).
- **Results**, a mapping:
  - `node_type`, `id`, `title`;
  - `host`: the host leaf's path, the same value as `find`'s `host`, and `host_anchor`, the
    section anchor where the node is stated ([§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)). Both null for works, axioms and a node with no
    statement marker. One key holds one shape on every surface.
  - `summary`: its `kb-summary.md` entry as `<page>#<slug>`. Null where it has none.
  - then the kind's own fields:
    - claim: `rigor`, `manually_scored`, `strength`, `build_status`, `strength_band`, `rationale`,
      `premise_count`;
    - analysis: `rigor`, `manually_scored`, `strength`, `strength_band`, `rationale`;
    - experiment: `status`, `rigor`, `manually_scored`;
    - work: `label`, `rigor`, `rationale`;
    - axiom: `label`, `rigor` (`1.0`).
- `strength` is rounded to two decimals; `rigor` is the value the record holds.
- **Refusals**: an `id` no node carries.
#### `edges` (query op)

This lists edge records, ordered by relation, source and target.

- **Arguments**, all optional:
  - `id`, the node, with `direction` (`in` = the node is the target, `out` = it is the source,
    `both`, the default). The node is the query's subject, so `id` names it ([§2.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.1-naming.md)); `source` and `target`
    name the ends in the rows;
  - `relation` (repeatable; any of the six).

  With no node given, every edge is listed.
- **Results** rows: `source`, `target`, `relation`, `bearing` (as the record holds it; null where the
  relation carries none), `context` (null for none), `manually_scored` (null where the relation
  carries no mark), and `replaced_relation` (null except on a `demoted` row). The mark tells a
  caller that `score --edge` will need `--rescore-manual`; whether an edge is dirty is
  `score --dry-run`'s answer.
- A node's dependents ([§1.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)) are the targets of `edges` with that node, `direction: out` and the
  four scoring relations.
- **Refusals**: `direction` without a node (usage); an unknown
  relation.
#### `trace` (query op)

This follows the scoring relations transitively from one node. `in` gives everything that bears on
the node's strength; `out` gives everything the node's strength reaches. It never walks
`references` or `demoted` records.

- **Arguments**: `id` (required); `direction` (`in` | `out`, required); `sort` (`depth`, the
  default, reading outward from the node; or `contribution`); `order` ([§2.8](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)); `limit` (only beside
  an explicit `sort`, [§2.8](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)). `sort: contribution` orders by each input's effect on its target's
  strength, the `contribution` as computed: on `refutes` that is the cap, 1.0 minus the
  refutation's strength, so a refutation that holds its target down hard sorts low and one that
  barely limits it sorts high. Ascending puts the inputs that hold their targets lowest first;
  descending the ones that leave them highest. `trace` is always the full transitive walk; `edges` gives one hop. `contribution`
  is the value the scoring code computes as the edge's input to its target's expression,
  reusing that code, never a restated formula. Every relation sorts together in one ordering;
  each row names its relation.
- **Results**: one row per edge reached, in `edges` row form plus `depth` (its distance from the
  node), the source's `strength` ([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md); for a work, an experiment or an axiom, its rigor) and
  `contribution` (`*pending*` where the edge's bearing or source strength is pending; null where
  the edge takes no part, as a planned experiment's), so the ordering shows. Every row copies into
  an edge op as [§2.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.1-naming.md) states.
- **Refusals**: an id no node carries; `limit` without `sort`.
#### `stats` (query op)

This gives a complete snapshot in one call:
- one count per node kind (`claims`, `analyses`, `experiments`, `axioms`, `works`);
- `edges` and `demoted_edges`;
- `subtree_aggregates`, per volume;
- `rigor_manual`, `rigor_model`, `bearing_manual`, `bearing_model`;
- `relations`: per relation, its row count, whether a score run's bearing pass asks about it, and,
  on a scoring relation, the edges at each bearing;
- `strength_bands`;
- `highest_leverage`: the ten claims below `0.65` with the most dependents;
- `weakest`: the ten lowest-strength claims.

Its help points to `find`'s strength filters for the full lists that the two ten-item lists
preview.

- **Arguments**: none.
### 3.5 Runs and run control

Over MCP, `build` and `score` start a detached child process and return once the run has started.
The run carries on without the server. At the CLI they run in the foreground, and SIGINT or SIGTERM
stops them resumably.

One repository carries one run at a time, a build or a score run, under the operation lock in the
repository's git directory. A second run is refused (`check: lock`), naming the holder's verb and
pid.

The state store defaults to `$XDG_STATE_HOME/kbase/<key>` (`~/.local/state/kbase/<key>` where the
variable is unset). `<key>` is the first 16 hex digits of the SHA-256 of the absolute,
symlink-resolved `kb-root/` path. `build` and `score` use that default unless `--state-dir` names
another. `status`, `cancel`, `wait` and `build-trail` read the store that the operation lock names
while a run holds it, and otherwise the default.

Ops:

- [`build` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)
- [`score` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)
- [`status` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/status.md)
- [`cancel` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/cancel.md)
- [`wait` (CLI only)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/wait.md)
- [`build-trail` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md)
#### `build` (tool + CLI)

This builds a KB from LaTeX volume roots into `kb-root/`. A volume root is a paper's top `.tex`
file: the file `00README.json`'s toplevel entry names, else the sole file containing
`\documentclass`. Each stage boundary is a commit, label `kb-build: <stage> | <display>`, scoped
to the paths the build owns. The start boundary records `started-at`, the commit the build began
from. The *trail* is the `kb-build:` commits on the current branch. Resetting the branch to
`started-at` discards it. The KB a build writes holds every title unique within its kind ([§1.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)),
as any write does.

- **Arguments**:
  - `volume-root` (paths, repeatable; relative paths resolve against the project root);
  - `bibliography` (`.bib` paths, repeatable; default every `.bib` beside any volume root);
  - `no-inference` (bool);
  - `through` (a stage id or display name);
  - `resume` (bool);
  - `state-dir`;
  - `ask-concurrency` (integer ≥ 1).

  A fresh build takes volume roots. `resume` takes none and continues with the inputs the trail
  records.
- **Result**: `state-dir`, `roles`, `through`, `no-inference`, `resumed`, `restored`, `stages`
  (`stage`, `commit`, `dropped`, `report`), `resume`.
- **MCP result**: once the build has entered a stage, `done` with `kb-root`, `state-dir`, `pid`,
  `resume` and `wait` (the `kbase wait` command line). Where the build exits first, the build's own
  document. Where it is still starting after 5 s, `done` with `state: starting`, `state-dir`, `pid`
  and `wait`; the build keeps running, and `status` follows it.
- **Refusals**:
  - a fresh build where `kb-root/` exists in any state, a skeleton-only one and one holding an
    interrupted build's records included (`check: kb-root`; the same refusal as `init`). This is
    the one refusal of a fresh build over existing ground;
  - `resume` with nothing to resume while `kb-root/` stands: remedy, remove `kb-root/` to start
    over. kbase does not inspect half-built state.
  - `resume` where `kb-root/` does not exist, whatever the trail records (usage): remedy, a fresh
    build;
  - `resume` given volume roots or a bibliography (usage);
  - neither volume roots nor `resume` (usage);
  - dirty paths the build owns (`check: dirty-paths`);
  - outside a git worktree: remedy `git init`;
  - a run holding the operation lock;
  - a pending conversion of an existing KB;
  - a missing decision or text role without `no-inference`.
#### `score` (tool + CLI)

This asks the scoring model for rigor and bearing values, one item per call. It prices the faults
each reply names and lands each value through the edit ops, with the mark `false`.

- **Scope**:
  - by default, every dirty item that is not manually scored, in both passes;
  - the rigor pass covers claims, analyses and experiments;
  - the bearing pass covers `supports`, `lifts` and `refutes` edges;
  - no pass reaches a work's rigor (judged by hand), an axiom (`1.0` by rule) or a `founds` bearing.
- **Arguments**:
  - `pass` (`rigor` | `bearing` | `all`, default `all`);
  - `node` (an id: score that node's rigor, dirty or not);
  - `edge`: score that edge's bearing, dirty or not. Over MCP a mapping `{source, target,
    relation}`; at the CLI `--edge <source>,<target>,<relation>` (ids and relations hold no
    comma);
  - `rescore-manual` (bool). In a full run it also asks every manually scored item in the pass;
    clean unmarked items are still skipped. It is required with `node` or `edge` on a manually
    scored item.
  - `dry-run` (bool);
  - `resume` (bool: continue the repository's most recent score run, with the scope it recorded,
    keeping every value it landed; it takes none of `pass`, `node`, `edge`, `rescore-manual`
    beside it);
  - `scoring-model` (a model reference for this run);
  - `ask-concurrency`;
  - `timeout` (duration, default `30s`);
  - `state-dir`.
- **Dry run**: lists every node and edge a run would ask, manually scored ones included where
  `rescore-manual` reaches them, with the total ask count. It also lists the dirty manually scored
  values, which the run skips. It writes no KB and cannot be resumed.
- **Result**: `state-dir`, `pass`, `roles`, `run`, `ledger`, `counts` (`total`, `done`, `failed`,
  `skipped`, `remaining`), `scored`, `resume` (where an item failed or was not reached).
- **MCP result**: once the ledger and pid stand, `done` with `kb-root`, `state-dir`, `pid`, `run`,
  `ledger` and `wait`. Where the run exits first, its own document. Where it is still starting
  after 5 s, `done` with `state: starting`, `state-dir`, `pid` and `wait`; `status` follows it.
- **Refusals**:
  - no scoring role in the configuration or the arguments (`key: models.scoring`);
  - `node` on a work or an axiom;
  - `resume` where the most recent score run finished, or where a newer run of any kind has
    started since it: remedy, a fresh `score`, which asks only what is dirty;
  - `resume` beside `pass`, `node`, `edge` or `rescore-manual` (usage);
  - `edge` on a `founds`, `references` or `demoted` edge;
  - `pass` with `node` or `edge`: a named item's pass follows from its kind;
  - `node` or `edge` on a manually scored item without `rescore-manual`;
  - an id or edge that does not stand;
  - a `resume` naming no standing run;
  - a run holding the operation lock;
  - a pending conversion;
  - a load error.
- There is no score batch and no item limit. The scope the caller names is the budget. `cancel`
  stops a run, `resume` continues it, and what landed stays.
#### `status` (tool + CLI)

This reports the current or last run: build or score, whichever is newest.

- **Arguments**: `state-dir`.
- **Build result**:
  - `state-dir`;
  - `state` (`none`, `running`, `cancelled`, `interrupted`, `failed`, `bounded`, `finished`);
    `none` where no build stands: `kb-root/` is absent, or was removed with no build since;
  - `pid`, `started`, `updated`, `ended`, `no-inference`;
  - `stages`, each stage done or not done, without commits, and one line saying that each completed
    stage's commit, with the command to rewind to it, is in `kbase build-trail`;
  - `current` (`stage`, `units-done`, `units-total`);
  - `recent-refusals`, `recent-fallbacks`;
  - `cache-entries`, `cache-bytes`;
  - `resume`.
- **Score result**:
  - `state-dir`;
  - `state` (`running`, `interrupted`, or the run's outcome: `done`, `cancelled`, `failed`,
    `refused`, `retry`);
  - `pid`, `started`, `run`, `pass`, `counts`, `scored`, `failures`, `resume`.
- **Refusals**: outside a git worktree; a `state-dir` inside `kb-root/`; a state store newer than
  kbase.
#### `cancel` (tool + CLI)

This signals the run that holds the state store, of any kind, and waits for it to let go. The item
in flight is abandoned with nothing written for it, and the run exits `cancelled`. It continues only
when asked to: `kbase build --resume` or `kbase score --resume`.

- **Arguments**: `state-dir`.
- **Result**: `state-dir`, `pid`, `resume`.
- **Refusals**: no run holds the store (`check: lock`).
#### `wait` (CLI only)

This returns when the run with the given pid, of any kind, is no longer progressing, then prints
that store's `status` document. It takes no lock.

- **Arguments**: `--pid` (required, positive), `--state-dir`.
- **Refusals**: a missing or non-positive `--pid` (usage).
#### `build-trail` (tool + CLI)

This lists the build's history, read-only. Each stage boundary comes with its commit and the exact
command that rewinds to it. `started-at` comes with the command that starts over. It runs none of
them. A boundary's command is the reset followed by `kbase build --resume` with the flags the
build's last run was launched with; the start-over command
is the reset alone. The reset is `git reset --keep <commit>`, which refuses rather than discarding
uncommitted changes in files the reset touches. It takes no target argument: every boundary and
`started-at` carries its own command.

- **Arguments**: `state-dir`.
- **Result**: `state-dir`, `head`, `boundaries` (oldest first: `stage`, `commit`, `subject`,
  `command`), `started-at` (`commit`, `command`; null where unrecorded), `dirty` (false, or the
  changed paths, the first 20 named and the rest counted).
- **Refusals**: outside a worktree; a state store `status` would refuse; a trail that records a
  stage over an unrecorded earlier one (`check: trail`).
### 3.6 Checks and output

Ops:

- [`verify` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)
- [`update-format` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)
- [`export-latex` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md)
#### `verify` (tool + CLI)

This checks the KB and writes nothing.

- **Faults** (outcome `refused`, each an item under `refusals`):
  - the dead-link gate over every link under `kb-root/`, anchors included, citations included
    (`dead-link`): a fragment is live where it names a heading's anchor or an HTML element's
    `id` in the document it reaches, or, on a summary page, an entry's anchor; and an id in a
    citation's link title, or a node id in prose, that no node carries (`unknown-id`);
  - a bare node id in prose, outside code blocks, markers and citation link titles (`bare-id`;
    remedy: `add-citation` with the id as `snippet`, [§1.5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md));
  - a frontmatter block on any document (`frontmatter`);
  - a KB in an older format (`index-freshness`);
  - a hosted node with no statement marker or more than one (`statement-marker`);
  - an axiom record with no marked heading in `axioms.md` or with more than one, a marked heading
    naming no axiom record, or a marked heading whose text is not its record's label and title
    (`axiom-heading`);
  - a summary page (a register page, [§1.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)) that does not match what the derivation renders
    (`register-freshness`).
- **Remedies**:
  - `kbase update-format` for a KB in an older format;
  - an empty write batch ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)) for a summary page that does not match the derivation: it
    re-derives every page;
  - for any other fault, the user's git work or prose edit, guided by the fault's detail.
- **Findings** (not faults): every demoted record (`check: demoted`).
- **Arguments**: none.
- **Result**: `findings`, then `refusals` where refused.
#### `update-format` (tool + CLI)

This converts a KB to the current format, run alone, so that any problem is purely the
conversion's and restoring `kb-root/` and any moved build record from `HEAD` reverses it. Every run re-derives every derived file
([§1.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)) and rewrites the maintained blocks ([§1.7](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)), whether or not the format changed. A conversion
to `4.0.0` ([§1.8](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md)) does exactly this:
- rewrites every statement marker to `kbase-id` form, attached with no space;
- moves an analysis marker that sits alone on the line directly after a line ending in the marker
  of a claim it lifts to directly after that claim's marker, and removes the emptied line. Nothing
  else moves.
- writes `kb-summary.md` pages;
- adds the node id as the link title of each citation whose anchor resolves to a node's statement,
  and reports each one it cannot resolve without guessing;
- rewrites every link into a heading, citations and ordinary links alike, to the heading's anchor
  under [§1.4](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)'s slugging, and points a work's citation at its root summary entry ([§1.5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)), so the
  converted KB passes `verify`'s dead-link gate;
- converts each bare node id in prose, of every kind, into the naming form ([§1.5](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)), listing each
  conversion (file, line, id). This happens only in this conversion; a bare id in a KB already at
  `4.0.0` is a `verify` fault for the author to fix.
- mints an id for every work and axiom, derived from its `3.0.0` id alone
  ([§2.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)). The old key or label becomes the
  record's `label`. Every place the old id stood now names the new id: edge and demoted record
  ends, record prose fields, the link titles this conversion adds, the bare ids it converts, and
  every `3.0.0` spelling it reads as naming a work or an axiom. It ends each
  `### Axiom <label>: <title>` heading in `axioms.md` with its axiom's marker. It reports each
  heading that names no axiom record, which the conversion cannot pair without guessing;
- rewrites an experiment's `status: pending` as `status: planned`;
- drops the `no-edge` field from claims and analyses. It does not split the text into edges, since
  one string may explain several named nodes; it reports each node's text, for the author to recast
  as `references` edges with `add-edge`;
- drops the `strengthen-by` field from claims, reporting each node's text;
- writes `judged-hash` as all zeros on every scored record, claiming nothing about whether a value
  is current: the first `score` after the conversion finds every value dirty;
- moves each build record standing at the repository root into `kb-root/.build/` with `git mv`.
  The records are kbase's own; only their place changes.

With `commit`, it commits the conversion as one commit, label `kb-write: update-format <from> →
<to>`. Its clean-tree precondition makes that commit exactly the conversion. As with a batch,
`commit` is opt-in.

- **Arguments**: `commit` (bool, optional).
- **Result**: `migration` (where the format changed: `from`, `to`, `note`, `warnings`, the
  unresolved citation links among the warnings), `written`, `removed`, `unfenced` (maintained-block
  documents without the marker pair), `sheet`, `dot`, `commits` (as a write batch's, [§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md); `[]`
  without `commit`), and, where the format changed:
  - `ids`, each work's and axiom's `from` (its `3.0.0` id) and `to` (its minted id);
  - `bare-ids`, each conversion's `path`, `line` and `id` (the minted id);
  - `no-edge`, each dropped field's node `id` and `text`;
  - `strengthen-by`, each dropped field's node `id` and `text`.
- **Refusals**:
  - `kb-root/`, or a build record the conversion would move, not clean (`check: dirty-paths`,
    naming the paths): remedy, commit or discard them;
  - a lock;
  - a KB newer than kbase;
  - a load error;
  - a conversion that cannot carry a file's content (`check: conversion`, naming file and line).
#### `export-latex` (tool + CLI)

This writes a LaTeX project from the KB through one pandoc run, synchronously.
- It writes into `--out` (default `latex-export/` beside `kb-root/`). kbase owns `gen-tex/` there
  and regenerates it on every export. It takes no lock, runs no git, and writes nothing under
  `kb-root/`.
- Where `--out` holds no `preamble.tex`, a commented starter is written. One that stands is never
  touched.
- The report lists every form the export simplified, and why.
- Compiling `main.tex` is the author's step.
- A re-issue over an unchanged KB writes nothing and reports `unchanged`.

- **Arguments**: `out`, `volume` (repeatable; default every volume).
- **Result**: `written` (absolute), `volumes`, `report`.
- **Refusals**:
  - a `volume` that is not a volume ([§1.6](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md));
  - a `gen-tex/` holding files and no export manifest;
  - pandoc missing or failing;
  - a load error.
### 3.7 Setup and configuration

The configuration lives in two places:
- `~/.config/kbase`, **global** (the default);
- `<repository>/.kbase`, **local**.

Each holds `providers.toml` (providers: `baseUrl`, `apiKeyFile`, `type`, `api`, `decisionsFlavor`)
and `config.toml` (`[models]` `decision`, `text`, `scoring`; `[asks]`; `[sampling.text]`,
`[sampling.scoring]`). `--config-dir` or `$KBASE_CONFIG_DIR` names the one directory read instead.

No op, CLI or MCP, takes a key file's path. A provider's `apiKeyFile` in `providers.toml` is read,
and no op writes it.

Ops:

- [`init` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/init.md)
- [`configure-providers` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-providers.md)
- [`configure-models` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-models.md)
- [`models` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/models.md)
- [`show-config` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/show-config.md)
- [`probe` (tool + CLI)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/probe.md)
- [`auth add`, `auth list`, `auth remove` (CLI only)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/auth-add-auth-list-auth-remove.md)
#### `init` (tool + CLI)

This creates an empty KB at `kb-root/`: no LaTeX, no build. It writes:
- `entry-point.md`, with the title as its only heading;
- `.index/format.yaml` and the two empty record files;
- `AGENTS.md`, `CONVENTIONS.md` (each with its maintained block) and the `CLAUDE.md` redirect;
- the derived skeleton (the sheet, or its placeholder where `dot` is absent).

It then commits `kb-root/` as `kb-init: knowledge base created`.

- **Arguments**: `title` (default: the repository's directory name).
- **Result**: `written`, `removed`.
- **Refusals**:
  - `kb-root/` exists, in any state (`check: kb-root`, no remedy): you cannot create what already
    exists;
  - outside a git worktree: remedy `git init`;
  - a title holding a line break ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md), Values);
  - git refusing the commit (`failed`, git's words, the tree left uncommitted).
#### `configure-providers` (tool + CLI)

This adds or removes a provider in `providers.toml`, creating the file where it is absent.
- Keys never pass through it.
- Where the provider's key is stored, it dials the provider and reports its catalogue, which vets
  the connection.
- Where no key is stored, it writes the provider and names the next steps: `kbase auth add <provider>`,
  then `kbase models`.

- **Arguments**: `configure-providers <provider> --base-url <url>`, with `--api` and
  `--decisions-flavor` optional, writes the provider's table, creating it or setting its fields;
  fields already in place report `unchanged`. `configure-providers <provider> --remove` removes it,
  and a provider with no table in the file is refused ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)). `--global` (default) | `--local`, mutually
  exclusive, each refused beside `--config-dir`.
- A dial that fails leaves the provider written, outcome `done`, with a diagnostic naming the
  failure and the next step. The provider is what was asked for; the dial only vets it.
- **Result**: `provider`, `layer`, `written`, `models` (where dialled), `next` (the steps that
  follow, where any do: no key stored, a failed dial or reload, or a removal that leaves a role or a
  stored key naming the provider), `warnings` (one item per configuration fault in the files the
  call writes into or reads beneath, as `models` reports them: `path` the file or directory,
  `detail` what is wrong; empty where none; on every outcome from the write on — a refusal or a
  write that fails carries none; `[]` also where the configuration does not reload after the
  write, which `next` reports). The written provider's own fault is both a `warnings` item and,
  with its remedy, a `next` step.
- **Refusals**: an invalid provider name; `--local` outside a git worktree; `--global` or `--local`
  beside `--config-dir`.
#### `configure-models` (tool + CLI)

This assigns a model reference to each role in `config.toml`, creating the file where it is absent.
It checks each reference against its provider's catalogue. It rewrites the `[models]` keys and keeps
every other byte. A model missing from its catalogue is a warning carried in `warnings`, not a
failure, since catalogues rotate.

- **Arguments**: `--decision`, `--text`, `--scoring`, each taking a `<provider>/<model>`, at least
  one given. The roles given are checked and written, and the rest stand as they are.
  `--global` | `--local` as above, each refused beside `--config-dir`; `--timeout`.
- **Result**: `models` (`decision`, `text`, `scoring`), `written`, `warnings` (one item per
  configuration fault in the layers read, as `models` reports them, `path` the file or directory,
  `detail` what is wrong; then one per catalogue miss, `key` the role's argument, `detail` the
  reference; `[]` where none; on every outcome once the configuration loads — a location refusal
  or a load failure carries none).
- **Refusals**: a value that is not a model reference; a provider with no entry; the location flags
  as above.
#### `models` (tool + CLI)

This lists the model ids a provider's catalogue offers, sorted.

- **Arguments**: `provider` (default: the only provider), `timeout`.
- **Result**: `provider`, `provider-rule`, `provider-layer`, `models`, `warnings` (one item per
  configuration fault, `path` the file or directory it is against, `[]` where none, on every
  outcome).
- **Refusals**: several providers and none chosen (`key: --provider`, `allowed` the names).
#### `show-config` (tool + CLI)

This reports how the configuration resolves, and changes nothing: the layers read, every provider
with the layer it came from, its protocol, the kind of key it has (`store`, `file`, `none`) and any
fault, the merged choices, the selection, each role's resolution, and diagnostics.
- Keys never appear.
- For a provider whose key comes from a file, `upgrade` is the command that moves the key into the
  store, and it is run as printed ([§4.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md)).
- It reads no KB and makes no network request.
- An unresolved role is a diagnostic, not a refusal.

- **Arguments**: `provider`.
- **Result**: `layers`, `providers` (each provider whose key comes from a file carries `upgrade`),
  `config`, `selection`, `roles`, `diagnostics`.
#### `probe` (tool + CLI)

This asks one model one question and reports what came back. Each call is a paid model call.

- **Arguments**: one of `decision`, `text`, `scoring` (default) or `model` (a reference); `prompt`;
  `timeout`.
- **Result**: `provider`, `provider-rule`, `provider-layer`, `selection`, `model`, `temperature`,
  `thinking`, `first-token-ms`, `wall-ms`, `finish-reason`, `reply`, `usage`.
- **Refusals**: more than one selector (usage); a selection that does not resolve, naming the
  key, flag or setting at fault.
#### `auth add`, `auth list`, `auth remove` (CLI only)

These manage stored API keys, one encrypted file per provider under `$XDG_STATE_HOME/kbase/auth/`.
The encryption is obfuscation, not a boundary. A key arrives on stdin alone, piped; it is never an
argument and never typed at a prompt.

- `auth add <provider>`: stores the piped key. Refused: a provider no `providers.toml` table names (any case); stdin
  that is a terminal; empty input; input over 64 KiB; whitespace or a control character inside the
  key. Result: `provider`, never the key.
- `auth list`: `providers`, lowercase, in name order.
- `auth remove <provider>`: `provider`. A provider with no stored key is refused ([§2.10](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).
### 3.8 The server

Ops:

- [`mcp` (CLI only)](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/3-operations/3.8-server/mcp.md)
#### `mcp` (CLI only)

This serves kbase over the Model Context Protocol: newline-delimited JSON-RPC on stdin and stdout.
stdout carries protocol frames only; diagnostics go to stderr and `--log-file`. EOF ends the server.

- **Arguments**: `--project-dir-required`. The global `--project-dir` is refused.
- The modes are in [§4.3](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md). The server sends no sampling request and offers no prompts. No inference
  runs from any tool but `build`, `score` and `probe`.
## 4. Served text

Sections:

- [§4.1 The instructions](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md)
- [§4.2 Tool descriptions, help resources and `--help`](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)
- [§4.3 Server modes and `project-dir`](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md)
- [§4.4 CLI-only commands](#44-cli-only-commands)
- [§4.5 Served resources](#45-served-resources)
### 4.1 The instructions

`initialize` always carries `instructions`. They hold what spans the whole session or several
tools, and stay short. Quoted wording below is the served text; every other item states the content
the served text carries.

1. **Where the project is.** Located mode: the KB's absolute path, or the repository's where
   `kb-root/` is yet to be built, saying the KB can be built. Not-located mode: the project path is
   the absolute path of the project's git worktree root, the directory holding `.git`, where
   `kb-root/` is or will be; the session gets it from the user, by asking or by proposing one for
   the user to confirm, and uses it with `project-dir` on every call that session.
2. The `kbase` skill carries the workflow.
3. `kb-root/.index/` holds kbase's own records: never read or write anything under it. Testing that
   a path there exists (`test -d`, a stat) is allowed; listing, globbing or opening anything under
   it is reading it.
4. Rigor and bearing values are judgements, and a session does not make them. Set a rigor or a
   bearing, its current value included, only to a value the user has given for that node or edge,
   in their own words or quoted in the task. When adding a node or an edge without one, leave the
   value out: kbase records it as pending. `score` judges claim, analysis and experiment rigor and
   `supports`, `lifts` and `refutes` bearings. A work's rigor and a `founds` bearing stay pending
   until the user gives one.
5. **Edge direction**: an edge reads "source *relation* target", so "A supports B" means A's
   strength bears on B. This one stored sentence is also rendered in `kbase --help`, so the CLI
   carries it too.
6. **Commit**: "Pass `commit` on every write batch: it is the only way to make the batch undoable
   as one unit. It commits all of `kb-root/`."
7. **Sheets**: the claim-graph sheets are drawn for people. Offer them for display, as files in
   `kb-root/` or, where this server lists them, as `kbase://sheet/` resources. Never read one for information; the queries answer
   what a sheet shows.
8. **Keys**: a command kbase prints for handling a key, such as `show-config`'s `upgrade`, is run as
   printed. Never compose one from a key file's path, and never ask for a key or read a key file
   into the conversation.
### 4.2 Tool descriptions, help resources and `--help`

Each command stores four texts:
- a one-line summary;
- one shared help body, the same text on both surfaces: an argument has one name, which the
  command line decorates as `--name` (a positional as `<name>`) and MCP takes bare; a kbase call
  is written as that surface makes one — a `kbase` command line, or the binding tool's call;
- an optional MCP note;
- an optional CLI note.

A note states only what differs because the environment differs. No help text is written
separately for each surface. The renderings:

| Rendering | Text | Read |
|---|---|---|
| MCP tool description | summary + MCP note + where the full help is | always (`tools/list`) |
| `kbase://help/<command>` | shared body + argument list + MCP note | on demand |
| `kbase <command> --help` | usage line + shared body + argument list + CLI note + a line naming the global flags (a subcommand's) | on demand |

A test asserts that each rendering is exactly its parts, so the surfaces cannot drift. The argument
list is part of the help on both surfaces: it is generated from the command's argument definitions
— each argument's name, type, default, and whether it is required or repeatable, with its
description — and each surface spells an argument as it takes one: the name as the body spells
it, a repeatable argument as a list over MCP, a mapping as its CLI string at the command line, a
shorthand only at the command line. The two help renderings differ only by their notes, that
spelling, and the usage line, the command line's synopsis (`kbase <command> [flags]`), which only
the command line has; the descriptions are the same text. A query operation's argument list is
its own, not the `query` tool's. A tool
description and its schema carry what every call of that tool needs. The help carries the full
rules, refusals and examples. `measure-mcp-surface` tracks the size of the always-read text.

- **`write` tool description.** One sentence, then the batch contract:
  - "A batch applies its entries in the order given, each checked against the records as the
    earlier entries left them; kbase never reorders a batch."
  - Shape errors are all reported at once; then the batch stops at the first refusal and writes
    nothing.
  - `dry-run` and `commit`.
  - Where the shared help is (`kbase://help/write`), and where each op's help is
    (`kbase://help/<operation>`).

  `kbase://help/write` and the CLI root help carry the batch contract in full ([§2.2](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)).
- **`query` tool description.** One sentence, then where the shared help is (`kbase://help/query`:
  how a query reads, sorts and limits) and where each op's help is.
- **MCP notes**:
  - `build` and `score`: the call returns once the run has started, as a detached process that
    carries on without the server; the result's `pid`, `state-dir` and `wait`; `status` follows the
    run and `cancel` stops it.
  - `probe`: "Each call is a paid model call: run it only when the user directs it, one call per
    direction, never on your own initiative or to retry."
  - the write ops: arguments are batch entries.
  - each command whose MCP arguments differ from its CLI flags in a way the argument list's
    spelling does not show states the difference.
- **CLI notes**:
  - `build` and `score`: run in the foreground; SIGINT or SIGTERM (`kbase cancel`) stops them
    resumably.
  - the write ops: the batch arrives as YAML (JSON accepted) on stdin or in the file `--values`
    names; `--dry-run` and `--commit`.
- **Shared bodies, by command**:
  - `status` and `cancel` speak of any run, detached or in the foreground, naming `build` and
    `score` as its kinds.
  - `remove-node`'s help says plainly that it removes every edge naming the node, and points to the
    dry run.
  - `stats`' help points to `find`'s strength filters.
  - `find`'s help says, in one line, that `min: 0` leaves out pending nodes.
  - No help names a command or resource outside this document.
### 4.3 Server modes and `project-dir`

- **Located.** The server found one project, searching upward from its working directory for the
  repository root. A call may leave out `project-dir` and runs there. `project-dir` is optional in
  every schema.
- **Not located.** `kbase mcp --project-dir-required`, or a server whose search found no repository.
  Every call names its project. `project-dir` is in every schema's `required` list, and the
  instructions say how to get the path ([§4.1](.claude-temp/retired-design/NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md), item 1). A server that locates no project serves
  exactly as one given `--project-dir-required`, instructions included. The flag forces this mode
  from inside a repository.
- `project-dir` is the absolute path of the project's git worktree root, the directory holding
  `.git`, where `kb-root/` is or will be. It is never `kb-root/` itself. A relative path resolves
  against the server's working directory. A path that is not a worktree root is refused
  (`check: worktree`, `key: project-dir`).
- Startup never fails for want of a project.
### 4.4 CLI-only commands

`auth` (a key never travels through MCP), `wait` (an MCP call cannot block; a session polls
`status`) and `mcp` (the server itself). The root help and `mcp`'s help name these three. Every
other command is an MCP tool or an op of `write` or `query`. Every tool other than `query` is a CLI
command, and each query op is one.
(`completion` and `help` are the CLI framework's own.)
### 4.5 Served resources

| URI | Type | Listed | Holds |
|---|---|---|---|
| `kbase://help/<command>` | `text/plain` | always | one per command a tool binds, each write and query op included: shared body + argument list + MCP note |
| `kbase://help/write`, `kbase://help/query` | `text/plain` | always | the family's ops and what they share |
| `kbase://sheet/<kb-root-relative path>` | `image/svg+xml` | located mode, a KB that opens | each sheet present on disk, the placeholder included |
| `kbase://model-hints` | `text/markdown` | always | guidance on choosing a model for each role, in the role vocabulary |

No resource is subscribable, and the server declares no `subscribe` capability. Nothing announces
a run's end: a session polls `status`, and a shell-capable harness runs, in the background, the
`wait` command that `build` and `score` return. An unknown URI is `-32002`; a missing `uri` is
`-32602`. The KB's
documents, the summary pages included, are not served: they are the harness's own files.
## 5. Out of scope: the presentation pass

A separate pass (ACTIVE_DESIGN.md, "Presentation") settles: how `project-dir`'s meaning is stated
once rather than in every tool's schema; how strictly the `write` schema types each op's entry
against its always-read size; and the per-tool floor at 16 tools. Nothing above depends on it.
