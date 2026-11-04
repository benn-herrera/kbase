## 10. Metadata format version

### The stamp

A KB declares its metadata format as `kb-format: <major>.<minor>.<patch>`, semver, in
`kb-root/.index/format.yaml`: one block mapping holding the one key, no `---` fences. Major: a reader
built for the old major misreads the KB (a renamed field, changed semantics, a changed anchor rule).
Minor: additive; an old reader ignores the addition safely. Patch: the bytes a writer produces
changed, their meaning did not. The current version is `4.0.0`.

### Reading the stamp

Before any other read, kbase reads `kb-format` from `kb-root/.index/format.yaml`; where that file
does not exist, from the entry point's frontmatter, the stamp's superseded place, which every KB
older than `3.0.0` uses; where both exist, the file's stamp governs. A `kb-root/` with neither
stamp file nor entry point is refused (`check: kb-root`). A KB stamped in neither place, an
unreadable stamp in either, and a stamp that is not a `<major>.<minor>.<patch>` version are refused
(`check: kb-format`), the refusal naming where it looked. Every producer of a KB writes the stamp
file: the build, `init` and every save (below). A stamp at the current major and minor with another
patch is treated as current and rewritten on the next save.

No conversion leads to a patch, so a migration chain stops at the form its last conversion produces
and the next save supplies the stamp: a `2.0.0` and a `2.0.1` KB both convert through the one
`2.0.x → 3.0.0` conversion (below).

### One version read and written

kbase reads and writes exactly the current version. The superseded forms are `1.0.0`, `2.0.x` and
`3.0.0`; an older stamp is read through migration (below). A newer major or minor, such as `4.1.0` or
`5.0.0`, is refused by every subcommand that reads the KB — `build` on an existing KB,
`update-format`, `verify`, the write ops, the queries, `score` and `export-latex`; `status` and
`cancel` do not load the KB — outcome `refused`, one item `check: kb-format` naming the KB's version
and kbase's, remedy "update kbase". No subcommand writes an older format, and nothing downgrades.

**Migration happens at load, and only `update-format` saves it.** Reading an older KB converts it in
memory, version step by version step: each step takes the KB's files to the next form's files and
reports the repository-relative paths it made obsolete and its warnings. Every command then sees the
current form, and a converted, unsaved KB answers every query as converted. Nothing is written. The
conversion is deterministic: the same files convert to the same result on every load and in
`update-format`, minted ids included. A read-only command (`verify`, the queries, `export-latex`)
writes nothing, and every other write — a write batch, dry run or not, `score`, `build` resuming over
the KB — is refused while the conversion is pending (`check: index`, remedy `kbase update-format`),
so a format change never rides along with another write.

`update-format` (§8) saves the conversion in this order:

1. `.index/edges.yaml`, then `.index/nodes.yaml`, then every other file the conversion produced (each
   document with its frontmatter removed);
2. the summary pages and the sheets;
3. the stamp file, at the version the KB is still at, so that a stamp always exists and always names
   the version the tree is at;
4. removal of the obsolete paths, less any the save itself wrote;
5. the entry point, whose frontmatter the conversion stripped;
6. the stamp file, rewritten at the current version: the stamp is what records that the migration
   completed;
7. the maintained blocks.

The removals are reported under `removed` (§7). A save that stops part-way leaves the old form in
place, stamped at the version it converts from, with some files already current. Where
`.index/nodes.yaml` then exists, both record files are the current form's — no `2.0.x` KB has a
`nodes.yaml`, and it is written second — so the next load takes them as they are, strips any
remaining frontmatter unread and derives nothing from the `2.0.x` registers or frontmatter. Such a
tree is not clean, so `update-format` refuses it (`check: dirty-paths`); the recovery is to restore
`kb-root/` and any moved build record from `HEAD`, then run `update-format` again. On an unmigrated
KB `verify` reports `index-freshness` with remedy `kbase update-format`.

### The KB YAML dialect

kbase writes, and a reader needs, only this subset: block mappings and block lists, lists of flat
mappings, a block list of scalars as a mapping's value, plain or double-quoted scalars, no anchors,
tags, folded or multi-line scalars; flow style only for a list of scalars (`[clm-a, clm-b]`) and an
empty list or mapping (`[]`, `{}`); a scalar is never wrapped. A reader of the dialect, in any
language, is conformant when it reads the record files of the `4.0.0` golden trees
(`internal/formats/testdata/*-4.0.0/`) to the values their decoded-values goldens
(`*-4.0.0.values.jsonl`) hold.

### Recognising frontmatter

A document's leading `---` block is frontmatter only when its first non-empty line is key-shaped:
`name:` with a kebab-case name. Such a block that does not parse as a YAML mapping is the
malformed-frontmatter refusal. A leading `---` block whose first line is not key-shaped (a prose
paragraph, a list, the content after a thematic break) is prose, and the document has no
frontmatter. Outside migration, kbase reads frontmatter on the entry point alone, and there for the
superseded stamp alone (Reading the stamp); `verify` reports a frontmatter block on every document,
the entry point included ([§8](#verify)).

### Documents of a KB

A KB lives at `kb-root/` beside the repository's `.git`. It holds kbase's records under
`kb-root/.index/`, the build's records under `kb-root/.build/` (`kb-build-node-pass.yaml`,
`kb-build-classification.yaml`, `kb-build-unmarked.yaml` and `kb-build-cull.yaml`), the sheets (§3),
and Markdown documents of these classes:

- the entry point, `entry-point.md`;
- index documents: every other document whose body names a child on a child-list line, a bulleted
  list item whose content is one or more links to `.md` documents (conventionally `index.md`);
- `axioms.md` (Axioms);
- the maintained-block documents `AGENTS.md`, `CONVENTIONS.md` and `README.md`, and the `CLAUDE.md`
  redirect ([§3](#stamped-documents));
- the summary pages, `kb-summary.md`, generated (below);
- leaves: every other Markdown document under `kb-root/`, except one under `.index/`.

Statement markers are read from leaves alone. The marker on a summary entry's heading is annotation
and hosts nothing. A *volume* is a top-level directory of `kb-root/` holding an `index.md`
([§3](#three-kinds-of-sheet)).

**Summary pages** are generated from the records and the statement markers after every write, and
are never read back. `verify` renders each in memory and compares bytes; a difference is
`register-freshness`, which an empty write batch fixes ([§8](#write-batches)).

- `<volume>/kb-summary.md` exists for each volume that hosts a node. `kb-root/kb-summary.md` exists
  where a node is hosted outside every volume, or a work exists. A page no node calls for is
  removed and reported under `removed`.
- Each page is titled `# KB summary`. Its preamble is one line: "Generated by kbase from the KB's
  records; do not edit."
- A hosted node has its entry on the page of the volume its host lies in. A work has its entry on
  the root page. An axiom has no entry, and neither has a node with no statement marker.
- Each entry is headed `## <title><!-- kbase-id: <id> -->`. Its anchor is the anchor of its heading,
  computed among every heading its page renders (Statements and anchors).
- Entries come in the reading order of their markers, then works by label.
- Every link a page renders is computed from each node's derived page and anchor. A prose link
  `<page>#<anchor>` holds while its node stays on that page with that anchor, and `verify`'s
  `dead-link` check reports one that no longer does ([§8](#verify)).
- What an entry shows is presentation: the node's current state — the leaves that host it, its rigor
  (marked where manually scored), every edge into it with the source's title, strength and bearing,
  the demoted records into it, its rationale and, for a claim or an analysis, a `### Strength`
  section with its derived strength line and trace. An experiment's entry shows its `status`.

### The record files

Each is one YAML document: a block list of block mappings, `[]` when it holds none. A node record's
type is its `id`'s prefix and an edge record's its `relation`; no record has a `kind` field. Every
field its type has is present and any other field is refused; an empty value is written as `""` or
`[]`, never omitted, so no reader takes an absent bearing for the judgement `0`. Strings are
double-quoted, escaping `\`, `"`, every control character below U+0020 but tab, and the characters a
YAML reader would take as a line break or refuse as unprintable, as `\uXXXX`; a string holds no line
break, and a stored prose value has each whitespace run collapsed to one space, its ends trimmed —
an op's `unchanged` test compares the collapsed value. Numbers are plain decimals, the shortest that
parses back as the value, with at least one fractional digit (`1.0`, `0.85`); booleans `true` and
`false`; keys in the type's field order. Nodes are written in `id` byte order and edges by identity —
`relation` in the order `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted`, a
`demoted` record's `replaced-relation` in the same order, then `source`, then `target`. Every
writer re-renders a whole file from the records, and writes it only where its bytes change.

| Node record | Id | Fields, in order |
|---|---|---|
| claim | `clm-<minted>` | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `judged-hash` |
| analysis | `ana-<minted>` | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `judged-hash` |
| experiment | `exp-<minted>` | `id`, `title`, `status`, `rigor`, `manually-scored`, `judged-hash` |
| work | `wrk-<minted>` | `id`, `label`, `title`, `rigor`, `rationale` |
| axiom | `axm-<minted>` | `id`, `label`, `title` |

`title` and `label` follow Five node types. `rigor` is a number in `[0, 1]` or `"*pending*"`.
`manually-scored` is a boolean and `judged-hash` a digest (Provenance and the judged hash).
`rationale` is a string, `""` for none. `status` is `"run"` or `"planned"`: whether the experiment
has been run. No record names a summary page: the page that renders a node is derived from its
host.

| Edge record | Relations | Fields, in order |
|---|---|---|
| unscored | `references` | `relation`, `source`, `target`, `context` |
| demoted | `demoted` | `relation`, `source`, `target`, `context`, `replaced-relation`, `bearing`, `manually-scored` |
| bearing | `founds` | `relation`, `source`, `target`, `context`, `bearing` |
| scorable | `supports`, `lifts`, `refutes` | `relation`, `source`, `target`, `context`, `bearing`, `manually-scored`, `judged-hash` |

`context` is a one-line string, `""` for none, and a query reports it null. `bearing` is a number in
`[0, 1]` or `"*pending*"`. A scoring edge runs push, from the supporter to the supported; a
`references` edge runs from the node that names to the node named. An edge record's identity is
(`relation`, `source`, `target`), but a `demoted` record's is (`demoted`, `replaced-relation`,
`source`, `target`), so an analysis's `lifts` and `refutes` of one claim, both demoted, are two
records. Only one record holds an identity.

### Statements and anchors

A claim, an analysis and an experiment are *hosted*: each has one *statement marker*,
`<!-- kbase-id: <id> -->`, in a leaf. A node's *host* is the document holding its statement. No
record stores the host or the statement: both are derived from where the marker sits each time the
KB loads, so the marker moves with the text when the author edits the leaf. A work and an axiom
have no statement marker and no host.

**Placement.** A marker is placed at the end of a named line, directly after the last non-blank
character of the line, with no space between. A marker that ends a heading line leaves the
heading's anchor unchanged. A marker is never placed on the opening or closing line of a fenced
block (a display-maths fence included). An analysis paired with a claim has its marker directly
after the claim's marker on the same line, nothing between them:
`# … Cure Is Hard<!-- kbase-id: clm-8350hb --><!-- kbase-id: ana-8350hb -->`. The same marker form
ends each axiom heading in `axioms.md` and each summary entry heading; neither is a statement
marker, and an axiom or work id in a marker hosts nothing.

**Naming a line.** Each op that names a line takes `document` (a leaf, relative to `kb-root/`),
`line` (1-based) and `snippet`. The `line` is the line the op acts on. The `snippet` is a short piece
of text that the line, with each whitespace run collapsed to one space, must contain; it verifies the
line, so it need not be unique in the document. A `line` past the end of the document is refused at
`line`. A line that does not contain the snippet is refused at `snippet`, and the refusal names up
to five lines holding it, those nearest the named line, since the document has likely changed since
the caller read it. An op that places a marker also refuses a fenced block's opening or closing line
(a display-maths fence included) at `line`, naming the line. Each op's own refusals are stated with
the op ([§8](#add-kind), [§8](#move-node), [§8](#add-citation)).

**Reading.** A heading's *section* runs from the line after the heading to the next heading as deep
or shallower. A *paragraph* is the run of lines around a marker's line that hold text once the `>`
prefix and markers are removed, all quoted or none, holding no heading line, and in which only the
first line may open a list item; a line of a display-maths fence is in the paragraph whatever its
text.

- A claim's statement is the paragraph its marker sits in. Where the marker ends a heading line, the
  statement is that heading's section.
- An analysis's or an experiment's statement is the section of the nearest heading at or above its
  marker, or the text above the first heading where none precedes it. So a claim marker and an
  analysis marker abutting at the end of a heading line both parse as that heading's section.
- Two layouts that the build's analysis split (§5) writes parse differently:
  - an analysis marker alone on a blank line inside a blockquote: the analysis's statement is the
    whole quoted run, and a claim the analysis lifts, marked in the same blockquote, has as its
    statement the text of its own marker's line before the marker;
  - a paragraph in which a claim's marker is followed by text and then by the marker of an analysis
    that lifts the claim: the claim's statement is the paragraph up to its marker, and the
    analysis's is the whole paragraph.
- Markers and `>` prefixes are removed from every statement.
- No public write op places the blockquote layout; the add ops and `move-node` can place the
  paragraph layout, and a marker placed into either layout parses by it.
- A hosted node with no marker, or with more than one, is a `verify` fault (`statement-marker`). It
  does not stop the KB loading. A node with more than one marker is hosted by the first in path
  order. A node with no marker has no host, no section anchor and no summary entry.

**Anchors.** A heading's anchor is the one GitHub gives it, so every link resolves on GitHub:
github-slugger over the heading's text as GitHub renders it — HTML comments stripped, closing `#`s
dropped, inline markup reduced to its text (a link to its link text, emphasis and code to their
content) — and trimmed. The algorithm: lowercase; drop every character that is not a letter, mark,
number, connector, space or hyphen; turn each space into one hyphen, without collapsing runs
(`A — B` → `a--b`); and suffix a heading repeated in one document `-1`, `-2`. A hosted node's
*section anchor* is the anchor of the nearest heading at or above its marker.

### Citations

A citation is a Markdown link in prose that refers to a node and links to where the node is placed.
It takes no stance: the citing text may agree with the node, dispute it or mock it. Its link title
is the cited node's id. Three forms:

- quote: `["the bound is tight"](leaf.md#bounds "ana-012xyz")`;
- paraphrase: `[the bound holds](leaf.md#bounds "ana-012xyz")`, with no quotation marks;
- naming: `[The Bound Is Tight](leaf.md#bounds "ana-012xyz")`, whose link text is the node's title,
  for naming a node without quoting it.

A quote or a paraphrase cites a statement, so only a hosted node takes those forms. A work or an
axiom is cited in the naming form only. A citation is the only way prose refers to a node: a bare
node id in prose, outside code blocks, markers and citation link titles, is `verify`'s `bare-id`
fault, whose remedy is `add-citation` ([§8](#verify)). A link whose title is absent or is no node id
is an ordinary link. kbase reads every link, a citation's title included, through one link grammar.

The link path is relative to the citing document. The link target is, for a hosted node, its host
leaf and section anchor; for an axiom, the anchor of its marked heading in `axioms.md`; for a work,
its entry on the root summary page. The id lets kbase tell which node a citation means even after a
heading rename or a move. A citation names a node and nothing else.

`add-citation` checks a quote or paraphrase once, when it places it ([§8](#add-citation)). After
that, a citation is checked like every other link: `verify`'s `dead-link` check confirms that the
document exists and the anchor is in it. A quote is never checked again against later text; keeping
the words faithful as the cited text changes is the author's job.

A citation is not an edge. It does not appear in `edges`; `citations-of` lists citations.

A citation's target is not the node's `canonical_path` (below). For a claim the target is its host
leaf and section anchor, while its `canonical_path` is its summary entry.

### `canonical_path` and `canonical_anchor`

Each node has a canonical path and anchor, derived at load, which the claim-graph sheet links to;
no result contains them, `show` and `find` naming `host`, `host_anchor` and `summary` instead
([§7](#item)).

| Node | Canonical path | Canonical anchor |
|---|---|---|
| claim | its summary page | its entry's anchor there |
| work | the root summary page | its entry's anchor there |
| analysis, experiment | the leaf hosting it | the anchor of the heading of the section holding its marker |
| axiom | `axioms.md` | the anchor of its marked heading |

A node with no host has neither.

### Axioms

An axiom is declared by its node record, which `add-axiom` writes (§8). `axioms.md` is prose stating
it, one heading each, `### Axiom <label>: <title><!-- kbase-id: <id> -->`; the marker pairs the
heading with its record, and `axioms.md` is not read for declarations. That marker is not a
statement marker, and the axiom stays unhosted. `verify`'s `axiom-heading` check holds the heading
and the record together by three checks: each axiom record has exactly one heading ending in its
marker; that heading's text is `Axiom ` followed by the record's label, `: ` and its title; and each
marked heading names an axiom record. A fault of these checks is one the author's prose edit clears
([§8](#verify)).

### The load: every fault refused

The load refuses a KB whose records hold any of these faults, `check: metadata`, naming each with its
file and its line or record:

1. a record file that does not parse;
2. an unknown or missing field;
3. a value of the wrong type or outside its domain, a `rigor` above `1.0` included;
4. an id outside its kind's grammar;
5. one node id on two records;
6. a pairing that no relation joins (the relations table below);
7. one edge identity on two records;
8. an edge naming a node that has no record;
9. a cycle among the scoring edges.

Faults 7 to 9 are found only once the record files decode without fault. A record file missing from
a KB stamped at the current version is refused `check: index`, remedy "restore it from git". Faults
7 to 9, which no write op produces, have the remedy "return the record files to a consistent
commit in git, such as the one before a merge, and reapply the other side's changes through write
ops". The remedy of a `rigor` above `1.0` says kbase never writes such a value, so whatever wrote
it is the way to undo it. No op resolves a load error.

### Five node types

`claim`, `analysis`, `experiment`, `work` and `axiom`. `node_type`, `target_kind`, the derived
views, the sheet and the result documents use these five words and no other.

Every id is minted: its kind's three-letter prefix (`clm`, `ana`, `exp`, `wrk`, `axm`), a hyphen
and a minted body of six lowercase letters and digits, so a token names its kind without
translation. To *mint* is to generate a new unique id; every add mints its node's id, for every
kind. No other field is part of an id.

- `title` is one non-empty line. A title is unique within its kind across the KB; nodes of
  different kinds may share a title, as a claim and the analysis split from it often do.
- `label` is one non-empty line. A work's label is its citation key,
  `[A-Za-z0-9](?:[A-Za-z0-9_.:+/-]*[A-Za-z0-9])?`, which opens and closes alphanumeric. An axiom's
  label is any text, LaTeX included. A label is unique within its kind across the KB: a work's among
  works, an axiom's among axioms.
- A label is never an id, and no op takes one where it takes an id.
- The load refuses an empty title or label and a work's label outside the citation-key grammar. The
  write ops refuse a title or a label that another node of the kind holds (`node exists` on an add,
  naming that node); the load does not check uniqueness. The build gives a repeated title the
  counter ` (2)`, ` (3)` and so on, so the KB it writes holds every title unique within its kind.
- A claim, an analysis and an experiment are hosted (Statements and anchors). An axiom is stated in
  `axioms.md` (Axioms). A work is shown by its entry on the root summary page.

Authoring prose may say *invariant*, *framework premise*, *framework*, *proposition*, *evidence*,
*trial*, *cited work* or *reference*: *invariant* and *framework premise* name an axiom,
*proposition* a claim, *evidence* and *trial* an experiment, *cited work* and *reference* a work;
*framework* names the document that declares axioms and is never a node type. The graph has one word
per role; the other words are the author's.

**A claim stands alone; an analysis is made of its citations.** A claim is a proposition about the
subject matter: its premises support it, and it means what it means without them. An analysis is
constituted by the nodes it cites — strip the ids from its sentence and no proposition remains — so
it has at least one premise and concludes in a claim. An analysis citing none is a method with no
inputs but its own, and reads its `rigor` rather than a claim's `0` (Four scoring relations). The
authoring test is that removal: if a proposition survives it is a claim, and if nothing survives it
is an analysis. A claim is the model's atom and needs neither: it may exist with no premises and
nothing resting on it. These are rules for authors, not faults: the loader enforces neither, and a
KB that breaks one is still read.

### Three quantities

`rigor` is a node's local value — a claim's, an analysis's, a work's or an experiment's — a number in
`[0, 1]` or `*pending*`; `bearing` is a scoring edge's scalar, a number in `[0, 1]` or `*pending*`,
and the one token on every scoring edge; `strength` is the derived value, a number in `[0, 1]` or
pending, on a claim and on an analysis, derived and never stored. `*pending*` is the format's one
no-number literal: written for a value nobody has judged, and read wherever a value is absent. It
means unjudged, not low. A work, an experiment and an axiom have no derived value — their `rigor` is
terminal, and an axiom's is `1.0` by rule, which no authored file contains — so each one's strength is
its `rigor`.

### Four scoring relations, one arithmetic

Every scoring edge is stored push: `source` is the supporter, `target` the supported node, and
`target_kind` the target's kind. An edge reads "source *relation* target": "A supports B" means A's
strength bears on B.

| Relation | Source kinds | Target kinds | Contribution |
|---|---|---|---|
| `supports` | claim, experiment | claim, analysis | `bearing × source strength` into the minimum |
| `founds` | work, axiom | claim, analysis | `bearing × the source's rigor` into the minimum; an axiom's rigor is `1.0` by rule |
| `lifts` | analysis, experiment | claim, analysis | `bearing × the lifter's strength`, maxed above the minimum |
| `refutes` | claim, analysis, experiment | claim, analysis | a cap, `1 − bearing × refuter strength`; the smallest cap wins |
| `references` | any kind | any kind | none: it has `context` only, saying why a node names another without bearing on its strength |
| `demoted` | ([The `demoted` relation](#the-demoted-relation)) | | none |

The four *scoring relations* are `supports`, `founds`, `lifts` and `refutes`. A node's *dependents*
are the distinct targets of its outgoing scoring edges.

`supports` carries a premise from the argument itself; `founds` carries support from outside it — a
cited work, or an axiom the argument leans on. An axiom's declaration in `axioms.md` is bookkeeping,
not origin: what it states is an external fact.

`references` and `demoted` are the two unscored relations: `references` has no bearing and no
value, and `demoted` keeps the bearing of the edge it replaced, which strength propagation does not
read. A bearing is required on every scoring edge and illegal on these two. A bearing weights the
contribution and never modifies its source's value. The scoring edges form no cycle, and an edge
always names nodes that exist (The load).

```
premises   = every supports and founds contribution that participates
derivation = min(rigor, every contribution in premises), and where premises is empty the node's
             own rigor for an analysis, 0 for a claim
final      = min(max(derivation, every lift), every cap)
```

`rigor` caps what the premises establish and is no premise itself. A node no participating premise
carries has an empty premise set, and its derivation is what its kind is worth bare: `0` for a claim
— whatever its `rigor` says, judged, pending or absent — so a bare claim reads `0.00` and falls in
`unsubstantiated`, however good its statement; an analysis's own `rigor`, since a method's inputs are
the rules of inference and not a KB node, so a premise-less analysis reads its method's value and the
claim it lifts reads it too. An edge that does not contribute is no supporter, so a claim whose only
edge is excluded reads `0`. Where a premise does participate and `rigor` is pending, the derivation
is unknown and propagates nowhere: a lift still counts — `max(pending, lift)` is the lift — while a
refutation alone leaves it unknown. `lifts` and `refutes` compose around the derivation unchanged: a
claim whose only scoring edge is a lift takes the lift's product, and one whose only scoring edge is
a refutation takes `0`; an analysis composes the same two around its own rigor, so a lift takes
`max(rigor, lift)` and a refutation `min(rigor, cap)`.

Strength propagation uses full precision: nothing rounds a value on its way into a derivation, so a
dependent multiplies its source's exact strength. A summary page — its `- strength:` line and a
premise's `(strength …)` — shows the value in the fewest decimals that parse back as it: four in the
ordinary case, more only where four would lose it, trailing zeros trimmed. A query result gives it as
a number at two decimals, the trace on a `- strength:` line writes it at two, and a `verify` message
quotes the record's form, being about the record. A pending bearing, or a pending source strength,
excludes its edge — no contribution, no lift, no cap — while `0.0` is a judgement and participates,
taking a minimum to zero and a cap to `1.0`. A non-`run` experiment is inert as a lifter or refuter,
and its `supports` edge is excluded like any unjudged premise.

**A value is derived from a charge list, not composed from the interior of the scale.** Each of the
two scoring passes makes a model call for the faults the text commits — the ids
[RIGOR_AND_BEARING_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/rigor-and-bearing/entry-point.md) defines, each with the
line that shows it — and kbase computes the value from the list: the pass's maximum less its ids' weights per
occurrence, floored at zero and exact at two decimals (§8). A written `rigor` is a value in
`[0, 1]` or `*pending*`, and a bearing's the same. `1.00` is the rigor tables' maximum — the largest
a `rigor` pass derives, which reads *no charge*, never *the text displays its reasoning*. An axiom's
`1.00` by rule is that same number and no separate case on the read path: an authored `rigor: 1.0`
is in domain and keeps that value everywhere the system shows it — the summary page's strength line,
the derivation, a query's display and strength propagation. An authored `rigor` above the maximum is
outside the domain, as an infinity (`.inf`) is, and the load refuses it (`check: metadata`), naming
the record file, the record and the field (The load). A bearing's `1.0` is in domain, its table
reserving nothing either. Strength propagation reads the value it is handed, whatever wrote the
number.

### The band ladder

The *band ladder* is the five *rungs* a `strength` is read against, each a threshold bucket with an
inclusive lower bound: `0.85` *ok to build on*, `0.65` *ok to build on, see caveats*, `0.45` *use as
input only*, `0.20` *do not build on, rework needed*, and `0.00` **unsubstantiated**. The bottom
rung states an outcome and not a cause: every strength below `0.20` falls in it from either
direction — a refuter's cap, or a minimum no premise carries — and what it says is that nothing a
claim built on this would inherit substantiates the claim. The cause, where a reader needs it, is
the run's fault record and the derivation trace. One value has one band wherever a band is
shown: a claim's or an analysis's record (`build_status`, `strength_band`), the `stats`
distribution's row, and the sheet's fill and tooltip. A pending strength has no band, and a strength
below `0.00` cannot occur.

### Edge direction and authoring

Every edge is stored as its record says, the scoring edges push: a `supports` or `founds` record
runs from the premise into the claim or analysis it supports, and a `lifts` or `refutes` record from
the lifter or refuter. `add-edge` ([§8](#edge-ops)) writes one edge as its record runs, over the
legal pairings of the relations table. An analysis lifts or refutes and never supplies a premise, so
an `add-edge` naming one as the source of a `supports` or `founds` edge is refused as a pairing no
relation joins, as the load refuses such a record. `edges` presents the edges at a node from the
reader's side, by `--direction`: `in` where the node is the target, `out` where it is the source
([§8](#edges)); `trace` follows them transitively.

### Required bearings

Every scoring edge record has its `bearing` field, and the strict load refuses one that lacks it,
has a value outside `[0, 1] ∪ {*pending*}`, or has it on a `references` record, naming the file, the
line and the field: a missing bearing is never read as `0`, nor as pending.

### Provenance and the judged hash

**The `manually-scored` flag** records whether a person's call wrote a record's rigor or bearing. It
is on claim, analysis and experiment records, and on `supports`, `lifts`, `refutes` and `demoted`
records. A work's rigor, an axiom and a `founds` edge have no flag, since no scoring pass reaches
them.

- A public op writes the flag `true` on a rigor or a bearing the caller supplied, `*pending*`
  included, and `false` on one it writes `*pending*` because the caller left it out. The ops are
  `add-<kind>`, `edit-<kind>`, `add-edge` and `edit-edge` ([§8](#add-kind), [§8](#edit-kind),
  [§8](#edge-ops)).
- An op leaves the flag as it is on a value it does not write. Re-asserting the current value of a
  record marked `false` marks it `true`, outcome `done`.
- A scoring run's results write `false` ([§8](#writing-the-results)), and so does the build
  ([§5](#a-build-records-no-judgement)). No argument sets the flag directly.

**The *judged hash*** is the input hash a value was scored against; a changed hash marks the value
stale.

- It is the field `judged-hash`, the last field of claim, analysis and experiment records and of
  `supports`, `lifts` and `refutes` edge records, required on every one of them: a record without
  it is a load error. It is a SHA-256 digest as 64 lowercase hex digits.
- It covers the item's text plus the digest of the prompt template that judges it. For a node the text
  is its statement (Statements and anchors); for an edge, the statements of its two ends. The model
  is not part of the hash.
- A rigor or a bearing that an add or an edit is given — `*pending*` included — records the judged
  hash of the text as the batch leaves it, and so does every value a scoring run writes, over the
  text its call was composed under. A value set otherwise keeps the hash it has. An add given no
  value writes the never-judged hash.
- The never-judged hash is all zeros. No computed hash matches it, so the item is stale. A demoted
  record has no hash, so an edge restored from one has the never-judged hash.
- An item is *stale* when its current hash differs from the recorded one; a value never judged is
  stale. `score` scores the stale items ([§8](#score)).

### The vocabulary

A node's local value is `rigor` and its derived value `strength`, everywhere they appear: a summary page's
`- strength:` line and `(strength …)` annotation, the derived views, the query keys, the sheet's
bands and `find`'s `strength` sort. The summary page is `kb-summary.md`, placed by host (Documents
of a KB); the statement marker is `<!-- kbase-id: … -->`.

### The `1.0.0 → 2.0.0` migration

The `1.0.0` form is standard YAML frontmatter (`---` fenced) in each document. A leaf's per-node
key groups (`exp-id`, `status`, `strengthens`; `sup-id`, `supports`) are one mapping each in a
top-level `experiment-nodes:` or `support-nodes:` list, a one-node leaf a one-element list; the build
records are YAML (`kb-build-*.yaml`); `.index/*.yaml` are YAML streams, one document per line, each
the record as a JSON object after the `--- ` marker; a writer never splits a record across lines. The
frontmatter is at the top of the file, so the up-link line (§3) is the first line after its closing
`---`.

The `2.0.0` form keeps that serialisation and changes the scoring vocabulary: three quantities
named, five node types and one scalar on every scoring edge, with the field names, relation names
and arithmetic those fields feed. Its patch `2.0.1` changes the bottom band's words on a
derived line, and nothing else.

One converter reads a `1.0.0` KB's files as bytes and writes the `2.0.0` form's, which the
`2.0.x → 3.0.0` converter (below) then takes. A file it does not locate it does not touch, and a
file already malformed under `1.0.0` is passed through as bytes for the `2.0.x` load the next step runs to
refuse by name — the converter never makes invalid `1.0.0` bytes valid. Nor does it drop bytes: a
file holding content the converter cannot convert refuses the conversion, `check: conversion`, naming
the file, the line and the content, so the load writes nothing and the KB remains as it was. It

- renames `- confidence:` on a claim entry, `- quality:` on an analysis entry and a work's
  `- strength:` to `- rigor:`, and `- solidity:` and `(solidity …)` to `- strength:` and
  `(strength …)`;
- renames the premise list `- depends-on:` to `- rests-on:`, an analysis's `supports:` pairs to
  `lifts:` and an experiment's `strengthens:` pairs to `lifts:`, each pair's value becoming its
  bearing;
- splits every claim into its statement and the argument for it: an analysis `ana-<body>` — the
  claim's own id body, or the next free body where the KB already holds that id — takes the claim's
  title, its rigor and its rationale, the claim's premise list moves to it whole, and the claim keeps
  its rigor, its derived line and its rework items and is lifted by that analysis alone, one `lifts`
  pair at bearing `1.0` declared on the leaf that hosts it. The analysis's statement marker is on
  the claim's own marker line, after the claim's, in the leaf the claim's statement is read from; a
  claim its leaf lists and does not mark takes one on the leaf's first heading. An argument stating
  no premise gives its analysis no premise list, which strength propagation reads at the analysis's
  own rigor. A claim no leaf contains is not split: it keeps its premise list and its rationale,
  since the analysis would hold them on behalf of no host — the `2.0.x` load reads no node for it;
- gives every scoring edge its bearing: an old `(applicability X)` becomes `(bearing X)` with its
  value kept, an untokened premise bullet takes `(bearing 1.0)` — work and axiom bullets included
  — and every experiment's mapping gains `rigor: 1.0`;
- rewrites each entry's `### Quality` to `### Rigor` and `### Solidity` to `### Strength`, and
  renames the register file `claim-quality.md` to `claim-rigor.md` with its Tier-2 marker
  `<!-- claim-quality: … -->` to `<!-- claim-rigor: … -->`, and rewrites every Markdown link to it
  to name `claim-rigor.md`, its anchor kept;
- rewrites every authored `work-<citation key>` id to `wrk-<citation key>` where it appears, an
  analysis's `support-nodes:` to `analysis-nodes:`, its `sup-id:` to `ana-id:` and its `sup-` ids to
  `ana-`;
- writes `axioms.md` — one `### Axiom <label>: Title` heading per `### INVARIANT-<label>:` section
  and per `- Axiom N: **…**` bullet of `invariants.md` or a legacy `AGENTS.md`, each axiom bullet's
  body beneath its heading — rewrites every `INVARIANT-<label>` citation in the register to
  `Axiom <label>`, a legacy `Axiom N` citation already being the `2.0.0` form, gives that bullet
  the bearing an axiom premise requires, and moves a legacy bullet's trailing parenthetical into
  `[context]`; labels are preserved and nothing is renumbered;
- splits a bullet naming several scoring targets into one bullet per target; and
- removes both paths the `2.0.0` form does not have, reported under `removed`: `claim-quality.md`,
  and `invariants.md`, whose declarations move into `axioms.md`. Every other file is left byte for
  byte and no index file is converted: the `1.0.0` index files were derived.

**The migration is not value-identical, and no derived-value equality is promised.** Four causes,
and every one of them is a value a reader will see change:

1. **An analysis's lift changes station; an experiment's does not.** Under `1.0.0` an analysis's
   support contribution was maxed into the claim's local value, which then entered the minimum: a
   corroborating analysis could be dragged down by a weak premise. Under `2.0.0` a lift sits above
   the minimum. A supported claim whose lift exceeds its derivation rises to the lift; where the
   derivation is at or above the lift, the value is unchanged. An experiment's pair already sat
   above the minimum, so its station does not change.
2. **`founds` scales.** An old `applicability X` was a gate: any positive value contributed the
   work's whole rigor, and `0` skipped the edge. A `2.0.0` `bearing X` contributes `X × rigor`, so
   every work edge below `1` falls, a `0` drives its claim to zero, and `1` is unchanged.
3. **An untokened or pending work edge stops pending.** A work bullet with no applicability, or an
   explicit pending one, left its claim pending. The untokened bullet now has `1.0` and
   contributes the work's rigor; an explicit `*pending*` token is kept and the edge is excluded.
   In both cases the claim reads a number where it withheld one.
4. **A claim nobody judged reads `0`.** A `1.0.0` claim whose `confidence` is `*pending*` withheld a
   number. The split gives that unjudged value to the claim's analysis, and an analysis whose
   premises are judged reads pending — so the claim's lift from it is excluded, the claim derives
   nothing, and it reads `0.00` and falls in `unsubstantiated`. That is the rule of strength propagation
   and not a defect: an unjudged analysis is no lift, exactly as an unjudged bearing is no
   supporter.

An old applicability was authored as a gate, not a weight, so every non-`1` value needs a human
re-read before it is trusted; the converter keeps the number and does not reinterpret it, and it
invents neither a finding nor a context where a migrated work bullet's context names none.

### The `2.0.x → 3.0.0` migration

The `3.0.0` form moves the graph into records. Every node and every edge is a record in one of two
authored files, `.index/nodes.yaml` and `.index/edges.yaml`; a statement marker in a leaf hosts each
claim, analysis and experiment; the register pages are rendered from the records and never read
back; and no document has frontmatter at all, the stamp being in `.index/format.yaml`. Everything
else — what the queries, the sheets and `score` read — is derived at load and never stored:
strength, derivation strength, build status and band, the premise and citation counts, a scoring
edge's `target_kind`, the citations, subtree aggregates, each document's kind (Documents of a KB),
each node's host, canonical path and anchor. The scoring vocabulary of `2.0.0` — five node types,
three quantities, four scoring relations, one arithmetic — is unchanged.

One converter reads a `2.0.x` KB's files as bytes and writes the `3.0.0` form's: the two record
files from what the `2.0.x` load reads, every document stripped of its frontmatter — the entry
point's `kb-format` with it, the next save writing the stamp file — and a statement marker for each
node. It refuses only what the `2.0.x` load refuses, with that load's own items, and the owner
repairs such a KB as a `2.0.x` KB; otherwise it never refuses, but takes the outcomes below and
warns — one item each, `path`, `line` and `detail` — wherever it drops data or invents it.

- A claim's and a work's register entry becomes its node record — `title` its heading, `rigor`,
  `rationale`, `no-edge`, `strengthen-by` — and which `2.0.x` register held it is dropped silently:
  its page is determined by its host (Documents of a KB), so a node printed in another volume's register moves,
  and a prose link to its old anchor is reported by `verify`'s `dead-link` check. A second register
  entry for one id is dropped with a warning.
- An analysis is its leaf's `analysis-nodes` entry joined with its register entry, titled with the
  heading the register entry is written under — the author's title for it — and where the register
  has no entry for it, with its host leaf's first heading. An analysis entry no leaf hosts, which
  `2.0.x` never read as a node, is dropped with a warning. Its canonical anchor is the section its
  statement marker is in, wherever the marker is (below), so a title and an anchor differing is no
  loss: the anchor names the place the reader lands on. An experiment's `experiment-nodes` entry
  becomes its record, titled with its host's first heading and its `rigor` `*pending*` where it had
  none. A host with no heading titles its node with its file name, with a warning. An axiom is
  read from its heading in `axioms.md`, else from a legacy declaration, as the `2.0.x` load read it.
- Every `rests-on`, `refutes`, `references` bullet, register analysis pair and leaf pair line
  becomes an edge record oriented as the `2.0.x` index row, `context` `""` where it had none. A
  bullet whose token named no node, which `2.0.x` dropped too, is dropped with a warning quoting it.
  An edge authored more than once — a `rests-on` target repeated, an analysis pair in both the
  register and the leaf, an experiment's `supports` pair beside a `rests-on` bullet naming it — is
  one record: equal bearings silently, their contexts joined with `; ` in authored order; different
  bearings keep the higher, a number above `*pending*`, with a warning naming both sites.
- A `demoted` bullet on an entry X naming Y becomes a `demoted` record from Y into X, replacing
  `supports`, its bearing `*pending*` — `2.0.x` kept no bearing when it removed the edge, so none is lost — and its
  origin dropped silently.
- Every `manually-scored` is written `false`.
- Each node's statement marker is placed from the `2.0.x` `claims:` listings and markers. A claim
  marked in a leaf that lists it keeps that marker. A claim listed but marked nowhere it is listed
  gets a marker in the first listing leaf in path order — an equation's on the line inside its
  display-maths fence holding its `\label{…}`, and a claim whose leaf lists no other non-equation
  claim and holds one labelled blockquote on that blockquote's first content line, so its label line
  still parses as one, silently; any other on the leaf's first heading line, with a warning, the
  section that heading opens being its statement. A leaf with no heading gets none of the markers
  its heading would hold, each with a warning. Every other listing leaf becomes a mention, with a
  warning, its `referenced-by` row lost, and every other marker of the claim is removed, with a
  warning. A claim listed nowhere keeps its first marker in path order and loses the others, each
  with a warning; one listed and marked nowhere has no host, `verify`'s `statement-marker` fault. An
  analysis's or an experiment's marker that a `2.0.x` document already contains remains where it is —
  the section it sits in is the node's statement, so one the `1.0.0` migration placed beside the
  claim it argues for keeps that place — and a second marker for the same node is dropped with a
  warning naming the one that remains; a marker in a leaf other than the one declaring the node is
  warned of, naming both paths, and the statement is read from its marker's leaf. Where none exists, one
  is written on a new line directly after the host's first heading.
- Frontmatter is removed from every document, the entry point's with it. `kb-format`, `kind`,
  `claims`, a string `no-claim` and the `subtree-*` keys are dropped silently, markers and the
  values computed at load recording their facts; `experiments`, `path-stable`, a key no `2.0.x` reader knew, an
  `experiment-nodes` or `analysis-nodes` list on a document that is not `kind: leaf` — which
  declared no node to the `2.0.x` load — and a value of the wrong type are dropped with a warning
  naming file, line and key.
- In a `2.0.x` register, text outside every entry is dropped silently, and text inside an entry that
  is neither a field it read nor a derived line is dropped with a warning quoting it. The registers
  are left as bytes: the save renders the pages the KB calls for (Documents of a KB) over them, and
  removes a `2.0.x` register at a page position no page takes, reported under `removed`.
- The derived `.index/edges.yaml` is replaced by the record file, and every other file under
  `.index/` but the two record files and the stamp file — `2.0.x`'s `claims.yaml`, `cites.yaml`,
  `strengthen-by.yaml` and `subtree-aggregates.yaml` — is obsolete, reported under `removed`. The
  stamp file is kept, holding the version the tree is still at, until the save's last
  write replaces it. The stamped `AGENTS.md` and `CONVENTIONS.md` are not read by the conversion:
  `update-format` rewrites the block each contains between its marker lines, and leaves one with no
  such pair untouched, listing it under `unfenced` ([§3](#stamped-documents)).

### The `3.0.0 → 4.0.0` migration

One converter reads a `3.0.0` KB's record files through the `3.0.0` load and writes them in the
`4.0.0` form. A `3.0.0` KB whose record files the `3.0.0` load refuses is refused with that load's
own items, `check: metadata`, one per fault. Every other file is kept as it is. The conversion's
steps:

- **Records.**
  - An experiment's `status: pending` is written `planned`.
  - Each `no-edge` and `strengthen-by` field is dropped, one warning per field naming the node's id
    and quoting its text (a claim's `strengthen-by` items joined with `; `), listed `no-edge` or
    `strengthen-by`.
  - `judged-hash` is written all zeros on every scored record, `manually-scored` kept.
  - Each work's and axiom's id is minted from its `3.0.0` id alone, the same id on every load and in
    `update-format`; its key or label is written as its `label`; every `3.0.0` work or axiom id in
    an edge's end or a prose field is renamed to the minted id; one warning per id is listed `ids`
    with the `3.0.0` id and the minted one. A bare id is read in the `3.0.0` grammar and cited under
    its minted id.
  - Each `### Axiom <label>:` heading of `axioms.md` naming an axiom record ends in that axiom's
    marker; one naming none is reported and left as written.
- **Markers.** Every statement marker in a `kb-root/` document is rewritten in the `kbase-id` form,
  attached with no space. A marker naming two ids is written as two abutting markers. An analysis
  marker alone on the line directly after a line ending in the marker of a claim it lifts is moved
  to directly after that claim's marker, its emptied line removed.
- **Register pages.** The `3.0.0` register pages, `claim-rigor.md` at `kb-root/` and directly inside
  each of its directories, are obsolete and reported under `removed`; the save renders the summary
  pages.
- **Citations**, resolving every anchor under `3.0.0` slugging:
  - Every link into a heading, a citation or an ordinary link, is rewritten to the heading's
    `4.0.0` anchor.
  - The node id is added as the title of each link whose anchor resolves to one node's statement — a
    heading's section holding that node's statement marker, an axiom's heading, or a `3.0.0`
    register entry, whose link is pointed at the node's place — and a work's citation is pointed at
    its entry on the root summary page.
  - Each bare node id in prose, and each link whose text is one, is made the node's citation in the
    naming form, one warning per conversion listed `bare-ids` with its path, line and id.
  - Each link or bare id it cannot resolve to one node without guessing — a section holding several
    nodes' statements, a register anchor several entries share, a heading the anchor names under
    neither slugging or names one heading under `3.0.0`'s slugging and another under `4.0.0`'s, a
    node with no place to point at — takes no id, one warning listed `unresolved-citations`.
  - A heading's `4.0.0` anchor is read after its own bare ids become citations.
  - A link into a removed register page never stays aimed at it: where every entry its anchor names
    is stated in one section it points at that section; otherwise at the summary page replacing the
    register page, without the anchor; and where no summary page replaces it, the link is removed
    and its text remains, a bare id in that text converted as any bare id is.
- **Build records.** Each build record at the repository root (`kb-build-node-pass.yaml`,
  `kb-build-classification.yaml`, `kb-build-unmarked.yaml`, `kb-build-cull.yaml`) is moved into
  `kb-root/.build/` with `git mv` by `update-format`, which refuses while a record it would move has
  uncommitted changes (`check: dirty-paths`, naming it). Its root path is an obsolete path.

### The migration announces itself

A command that reads a KB in an older format writes one sentence to stderr, and each warning on a
line of its own, and includes one `migration` record — `from`, `to`, `note`, `warnings` — after
`kb-root` (§7), on every such call until the `update-format` whose save stamps `4.0.0`. `note` is,
for every migration, "every scored record's judged hash is now the unjudged one: no rigor or bearing
is claimed current, and the next score run finds every one dirty". §7 names the commands. Over MCP
the document is the channel and stderr is the server's log; `status` and `cancel` load no KB and
never include it.

### The `demoted` relation

A `demoted` record is a tombstone for a scoring edge removed to break a cycle; `restore-demoted`
undoes it. The record keeps the edge whole: `relation: demoted`, `source`, `target`, `context`,
`replaced-relation` (`supports`, `lifts` or `refutes`), `bearing` and `manually-scored`. A `founds`
source, a work or an axiom, has no incoming scoring edge, so it closes no cycle. The record runs in
the replaced edge's direction. Its identity is (`demoted`, `replaced-relation`, `source`, `target`),
so more than one `demoted` record from one `source` to one `target` exists only where separate
cycles were removed at different relations between them.

- A `demoted` record is outside strength propagation and the cycle check.
- Only the build writes `demoted` records, in Cycles. No public write op creates one.
- Ops name a `demoted` record by `source`, `target` and `relation: demoted`. That name covers every
  `demoted` record from that source to that target.
- There are three ways to resolve one: `restore-demoted` puts the replaced edge back and deletes the
  record ([§8](#restore-demoted)); `remove-edge` with `relation: demoted` deletes the record and
  leaves the edge removed; `add-edge` of the replaced edge writes the edge with the add's own values
  and deletes the record ([§8](#edge-ops)).
- `verify` lists every `demoted` record under `findings` (§7) with outcome `done`; a finding is not a
  fault. `edges` reports it as a row of its own relation, out of its source, with
  `replaced_relation`; `stats` counts them as `demoted_edges`, after `edges`, which counts every row;
  the sheet draws each as a cut (§3); and its target's summary entry lists it.

### Cycles: cut as the walk finds them

The build breaks the cycles its own edges close, within the stage that writes them, by one
depth-first search; a cycle the records hold at load is a load error (The load). Cycle breaking
removes the back edge the search finds, replacing it with its `demoted` record.

- The search order is the node order of the summary pages, which no minted id decides, so a build's
  removals are reproducible modulo ids ([§5](#determinism)): hosted nodes in the reading order of
  their markers, then every other node by id — a work or an axiom, which no scoring edge enters,
  lies on no cycle. Edges compare by relation in the order `supports`, `founds`, `lifts`,
  `refutes`, then source, then target, each end in that node order.
- The search is depth-first over the `supports`, `founds`, `lifts` and `refutes` records, starting at
  each node and following each node's edges in the search order; `demoted` records are not
  followed.
- Each time the search closes a cycle — an edge reaching a node on the current path, the back edge —
  it removes one edge of that cycle: the first, in the search order, of the cycle's `supports` edges
  into an analysis, where one exists; else the back edge. The removed edge becomes its `demoted`
  record, with a warning naming the edge and the cycle it broke. Nothing else is dropped.
- After each removal the search starts over, until no cycle remains, so the removals depend on the
  edge set and the documents' order alone.

Every public op that adds a scoring edge refuses one that would close a cycle (§8), so only the
build breaks cycles. Restoring a `demoted` edge takes at most two edits: remove or redirect another
edge of its cycle, then restore it with `restore-demoted`, which refuses a restore that still closes
a cycle.

---
