# SPEC – kbase

Consumer-facing behaviors and outcomes

Must-see:

- Any implementation that satisfies this SPEC is valid
- No project mechanisms or code structure are put forth, that belongs to ARCHITECTURE
- No project-specific process is dictated, that belongs to CONVENTIONS

## 1. What kbase is

kbase is a program that builds, maintains and scores a knowledge base (KB) from LaTeX papers.
`kbase build` reads one or more papers, each given as its top `.tex` file, through pandoc (§2) and
writes one KB: a directory of Markdown files that describes the papers' structure and the argument
they make, as a graph of claims (§10). The build derives its draft from the LaTeX deterministically,
and model calls only refine it (§5). Maintenance subcommands then edit the KB through validated,
all-or-nothing batches of writes, each followed by a rebuild of the derived views (§8), and query,
check and score it. `kbase score` has a model judge each node's rigor and each scoring edge's
bearing, and the strength of every claim and analysis is derived from those values (§10, the
scoring schemes). The subcommands run at a shell or, through `kbase mcp` (§11), from any external
agent harness that speaks MCP. A KB can also be created empty by `init` (§8) and filled by
authoring with those same subcommands, with no LaTeX and no build; LaTeX stays the only input
format of `build`.

- A paper's **top `.tex` file** is the file `00README.json`'s `toplevel` entry names, else the sole
  file containing `\documentclass`.
- A **KB** is a `kb-root/` directory beside a repository's `.git`: the Markdown document tree and
  its metadata layer.
- The **metadata layer** is what kbase writes into a KB: the node and edge records in `.index/`,
  the statement markers in the leaves, the format version in `.index/format.yaml`, and the summary
  pages kbase renders from them (the format of §10).
- The **load-bearing forms** are the Markdown shapes KB readers key on: the up-link line, the
  cross-reference anchor form, the `` ``` math `` fence, the citation span, and the labelled
  blockquote.

Input is LaTeX only; no other input format is in scope.

---

## 2. Given interfaces

| Interface | Observed version | Contract |
|---|---|---|
| pandoc | 3.12 (`pandoc-api-version` 1.23.1.2) | Required on the host, not shipped. |
| Graphviz `dot` | Any version | Optional on the host, found on `PATH`, not shipped. Draws the claim-graph sheets (§3); absent, the command that renders the sheets writes the placeholder at the root only and its result says so (§3, §7). |
| git | 2.56.0 | Required on the host. The recoverable record of a build (§6). |

### pandoc

The accepted range's lower bound is 3.12. Its upper bound is the next major
`pandoc-api-version`, exclusive, where major means the first two components under Haskell's
versioning policy: 1.23.x is accepted, 1.24 is not. Only the owner widens the range. The range and
the `pandoc-api-version` are enforced at preflight; a missing pandoc is refused (`check: pandoc`)
with an error naming pandoc's install page. The build's reader runs pandoc LaTeX to JSON with no
filters, and with `--citeproc` exactly when a bibliography is offered; `export-latex` (§8) is the
one run with a filter.

---

## 3. Compatibility contract

- A KB kbase builds is navigable in Claude Code or opencode through the kbase skill's reading
  route, and maintainable through the same skill's maintenance route by kb-maintainer;
  `kbase score` grades the KB's rigor and bearings.
- Exact: the metadata layer, the load-bearing forms, and the KB's placement (`kb-root/` beside
  `.git`; reading starts at `kb-root/entry-point.md` and descends through `index.md` files).
- Free and untested: presentation — wrap width, bullet characters and the like.
- Byte equality is asserted only where a named reader needs it:

| Byte-equal | Reader that needs it |
|---|---|
| Statement markers, `<!-- kbase-id: <id> -->` attached with no space ([§10](#statements-and-anchors)) | The marker reader; the add ops' and `move-node`'s locate and splice |
| `.index/format.yaml` | The format-version read — the version is the bytes the file contains — and every save, which rewrites it only where its bytes change |
| Summary pages, `kb-summary.md` ([§10](#documents-of-a-kb)) | `verify`'s freshness check (`register-freshness`: each page rendered in memory and compared with the bytes on disk); no person or session edits a page or takes a value from it |
| `.index/nodes.yaml`, `.index/edges.yaml` | The write ops' idempotence: whole files rendered deterministically |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | The precheck of a write batch's rebuild of the derived views and of `update-format` (`check: agents-redirect`, [Stamped documents](#stamped-documents)) |
| The `` ``` math `` fence opener | The inventory's display-maths fence reader |
| The up-link line | The readers that key on the up-link line |
| The claim-graph sheets' DOT, against the DOT goldens | The sheet writer's write-only-where-bytes-change rule: under the same `dot`, a re-render leaves an unchanged sheet's bytes as they are |

### Stamped documents

`AGENTS.md` and `CONVENTIONS.md` under `kb-root/` are the **stamped documents**; with `README.md`
they are the **maintained-block documents**. `README.md` gives the maintenance commands: kbase's
subcommands. The stamped documents point a reader at kbase's tools, never at its records:

- `AGENTS.md` forbids reading or writing anything under `.index/`, save testing that such a path
  exists, and states the rule that a session sets a rigor or a bearing only to a value the user
  gave, both as the MCP server's instructions do, in the same words ([§11](#served-text), items 3
  and 4).
- `CONVENTIONS.md` states that the records and the statement markers are written only by a write
  op, never by hand, and that no agent edits the block kbase maintains in a stamped document.
- A summary page is generated as prose for people and sessions to read. A person or session neither edits one nor takes a value from it, and kbase never reads one back,
  and `verify` reports a page whose bytes differ from the rendering (`register-freshness`).

A **maintained block** is a generated region between marker comments: the lines from
`<!-- vvv-kbase … kbase-vvv -->` to `<!-- ^^^-kbase … kbase-^^^ -->`. Each marker line is
recognised by its two ends alone, so the text between the ends may differ between releases.

- A document containing exactly one opening marker line and one closing marker line, the opening
  first, has its block rewritten: kbase rewrites the span between the two lines and nothing else,
  and the text outside them is the KB's own.
- A document containing any other arrangement of marker lines is left untouched and listed under
  `unfenced` in `update-format`'s result (§8). That is not a refusal.
- A pair with nothing between its lines is valid; the next `update-format` fills it.
- An absent `AGENTS.md` or `CONVENTIONS.md` is written whole, marker lines included. `README.md` is
  written only by a build, which supplies its overview passage outside the block; `init` and
  `update-format` never create one.
- A block is rewritten on every `update-format` run, and when `init` or a build writes the
  document. A document is written only where its bytes change.

`kb-root/CLAUDE.md` is the redirect to `AGENTS.md`: its content is the one line `@AGENTS.md`. `init`
writes it where absent. A write refuses where `CLAUDE.md` exists and is not the redirect (`check:
agents-redirect`); nothing converts it.

### Formats

A new KB is written in the current metadata format, `4.0.0` ([§10](#the-stamp)). A KB in a
superseded form, `3.0.0`, `2.0.x` or `1.0.0` (§10), is read through migration.

**Theorem numbering** is whatever pandoc prints.

### `claim-graph.svg`

A picture of the claim graph, drawn through Graphviz `dot` (§2). Each such diagram is a **sheet**.
`update-format`, `init` and a write batch's rebuild of the derived views render every sheet on each
run and write one only where its bytes change. The SVG's bytes are Graphviz's; no check reads them
for their content, and no other command's behaviour depends on them.

#### Three kinds of sheet

A **volume** is a top-level directory of `kb-root/` containing an `index.md`; in a built KB it is one
paper's directory.

- `kb-root/claim-graph.svg` is the full sheet of the whole KB, always drawn: every volume a cluster
  containing its attached nodes. The sheet's title, and the label of the cluster of nodes no volume
  hosts, is the entry point's first heading, verbatim; `kb-root` where it has none.
  The KB title is the graph's label across the top. A volume's cluster is titled from its `index.md`
  first heading, falling back to the directory name, with the subtitle "N nodes · M unattached".
- `kb-root/claim-graph-digest.svg` is the volume overview diagram: one box per volume listing its
  node counts per kind on one line, its unattached count and its within-volume edge counts, and the
  cross-volume edges as bundles whose counts cover every edge class; each box links to the volume's
  index and, on a second line reading `claim graph →`, to its sheet. The full sheet links to the
  overview diagram (`volume digest →`) and the overview diagram to the full sheet (`← full claim
  graph`).
- `kb-root/<volume>/claim-graph.svg`, one per volume: the volume's claims plus every claim one edge
  away, drawing only edges with an end in the volume.

The overview diagram and the per-volume sheets exist only where two or more volumes contain nodes; the
nodes no volume hosts are not a volume, and a volume containing no node has no sheet and no
overview-diagram box. A one-volume KB has the full sheet alone. An overview diagram or volume sheet
the KB no longer calls for is removed by the next render, and reported (§7); a file named
`claim-graph.svg` directly inside a top-level directory of `kb-root/` is by definition such a
sheet.

#### What is drawn

The four scoring relations — `supports`, `founds`, `lifts` and `refutes` — each in
a stroke of its own, every stroke pointing in the push direction, from the supporter to the node it
supports. `references` edges are not drawn; a `demoted` record (§10) is drawn dotted red, as the
edge it replaced. A node with no drawn edge is not drawn as a node; it is listed in a table titled
"unattached (N)", one row per node with a link and a tooltip. Of the drawn `supports` edges, only
the transitive reduction appears; it checks an edge only against the edges still kept.

- *Edges*: each relation is drawn in one style of its own — `supports` black solid, and `founds`,
  `lifts` and `refutes` each a stroke beside it — whatever the build knew of how an edge was found:
  the sheet reads no build record. A `demoted` record is drawn dotted red in the direction of the
  edge it replaced; its tooltip calls it `cut`, the overview diagram counts such records beside the
  other classes (`1 cut`) and draws a bundle of them, and its legend row reads "cut from supports:
  part of a circle".
- *Nodes*, seven shapes: a claim's three — a labelled block (bold frame), an equation and prose —
  and an analysis (dashed rounded), an experiment (component shape), an axiom (grey, bold frame)
  and a work (note). An edge end that no record has is drawn as a ghost box, dashed red, labelled
  with the id. Claims and analyses are filled by strength band, `*pending*` in grey. A node's
  tooltip reads `<id> [<kind>, <band>] <title>`, the band omitted for a node that has none, every
  backslash in it shown as written (`$\Gamma$` does not become the graph's name).
- *Links*: every node links to its canonical place ([§10](#canonical_path-and-canonical_anchor)) —
  a claim's or a work's entry on its summary page, an analysis's or an experiment's section in the
  leaf hosting it, an axiom's heading in `axioms.md` — and a claim with no host links nowhere; an
  overview-diagram box links to the volume's index and sheet.
- A legend. The SVG root is `width="100%"` with a `viewBox`, so it scales to the viewer.

**Without `dot` on `PATH`**, no sheet is rendered: an existing sheet is left as it is, and where
`kb-root/claim-graph.svg` does not exist the render writes a placeholder there — an SVG naming the
missing tool and, beneath it, `index sha256:` followed by the first 12 hex digits of the SHA-256 of
the `.index/` files — the two record files — concatenated in sorted path order, deterministic.
The command says so in one line on stderr, and its result names the lack (§7).

---

## 5. Build behaviour

```
kbase build <volume-root>... [--bibliography FILE]... [--no-inference]
            [--through <stage>] [--state-dir DIR] [--ask-concurrency N]
kbase build --resume [--no-inference] [--through <stage>] [--state-dir DIR]
            [--ask-concurrency N]
```

A value of `--ask-concurrency` under 1 is a usage refusal naming `--ask-concurrency`, and the
`resume` call a build returns includes `--ask-concurrency` where it was given.

### Output and placement

The KB is written only at `kb-root/` in the git worktree root. `kb-root/` and everything below it
is kbase's domain: kbase checks and commits nothing outside it, not in a build, not in a write
batch's `commit` ([§8](#write-ops)), and not in `update-format`'s clean check, which also checks
the build records at the repository root that it would move ([§8](#update-format)). An output the
caller sends elsewhere, such as `export-latex`'s `--out`, is not kbase state. The state store (§6)
is never written into `kb-root/`.

The build records — `kb-build-node-pass.yaml`, `kb-build-classification.yaml`,
`kb-build-unmarked.yaml` and `kb-build-cull.yaml` — live in `kb-root/.build/`, so every path the
build commits is under `kb-root/`. They are committed at every stage boundary, so a rewind restores
them with the KB, and they stay after the build finishes.

**Fresh versus resume** is derived from the `kb-build:` checkpoint commits (§6), never configured.
`kbase build <volume-root>...` starts a fresh build where no `kb-root/` exists; `kbase build
--resume` continues the checkpoint commits. A fresh build where `kb-root/` exists, in any state, is
refused ([§5](#starting-over)). The KB a build writes keeps every title unique within
its kind ([§10](#five-node-types)), and its writes go through the write ops, each followed by the
rebuild of the derived views ([§8](#writes-leave-nothing-stale)): the derived views are current at
every checkpoint commit from the `spine-seed` stage on. A link pandoc wraps around a
cross-reference, its destination the bare LaTeX label, is unwrapped to the cross-reference inside
it, which links the target document's heading.

`build` refuses outside a git worktree, naming `git init`. It refuses if `kb-root/` is dirty and the
dirt is not an interrupted stage's own uncommitted work (§6), and only then.

### Inputs

A relative top `.tex` file path or `--bibliography` path resolves against the project root. A build
orders its top `.tex` files, and its `--bibliography` files where given, by full path as text,
whatever order they were given in; every stage reads them in that order and the checkpoint commits
record it (§6). `--resume` takes no inputs: it continues with the top `.tex` files and
`--bibliography` files the newest boundary records, in the order recorded. Before it resolves a
state store, `build` refuses, `check: usage`, a `--resume` given a top `.tex` file (`key`
`<volume-root>`) or a `--bibliography` (`key` `--bibliography`), one item per argument kind, and a
build given neither top `.tex` files nor `--resume` (`key` `<volume-root>`).

### Starting over

To start over, remove `kb-root/`, then run a fresh build. kbase does not inspect half-built state.
`build-trail` ([§6](#kbase-build-trail)) lists the commands that rewind to a checkpoint commit;
kbase runs none of them.

**`build`'s refusals.** Each is outcome `refused`:

- Outside a git worktree: `check: worktree`, remedy `git init`.
- A fresh build where `kb-root/` exists in any state, an empty or scaffold-only one and one containing
  an interrupted build's records included: `check: kb-root`, `path` the `kb-root/`, no remedy. It
  is the refusal of `init` over an existing `kb-root/` ([§8](#init)), and the one refusal of a
  fresh build over existing ground.
- A `--resume` with nothing to resume — an empty set of checkpoint commits, one whose newest
  boundary records no inputs, or a `kb-root/` that does not exist — is one item, `key` `--resume`,
  `path` the repository root. While `kb-root/` exists: `check: kb-root`, remedy "remove kb-root/,
  then run a fresh build, to start over". Otherwise: `check: usage`, `detail` "kb-root/ does not
  exist: there is no build to resume", remedy `kbase build <volume-root>...`, whatever the
  checkpoint commits record.
- A `--resume` given a top `.tex` file or a `--bibliography`, and a build given neither top `.tex`
  files nor `--resume`: `check: usage` ([Inputs](#inputs)).
- A `--ask-concurrency` under 1: `check: usage`. A `--through` that is neither a stage id nor a
  stage's display name: `check: usage`, `key` `--through`, `allowed` the stage ids.
- A dirty `kb-root/` that is not an interrupted stage's own uncommitted work: `check: dirty-paths`,
  one item per path, remedy "commit the change or restore the path to the last commit, then
  re-run".
- Checkpoint commits that record a stage while leaving an earlier stage unrecorded were written by
  a build running another stage order, and a build over them is refused before anything is restored
  or written: one item, `check: trail`, `path` the repository root, `detail` naming the recorded
  stage and the unrecorded one before it, remedy "start over from a commit before the trail".
- A run holding the run lock: `check: lock` ([§6](#git-holds-the-recoverable-record)).
- A state store newer than this kbase, or whose run record names neither verb: `check: state-dir`
  ([§6](#the-state-store-is-versioned)). A `--state-dir` inside `kb-root/`: `check: usage`, `key`
  `--state-dir`.
- A resume over an existing KB in an older format whose conversion is pending: `check: index`,
  remedy `kbase update-format`. Over a KB whose format version is newer than kbase: `check: kb-format`
  ([§10](#one-version-read-and-written)).
- No provider configured for a build that has a model-calling stage left to run, which includes a
  missing `decision` or `text` role without `--no-inference` (`check: provider`,
  [Inference preflight](#inference-preflight)).
- Source refusals before the first stage: an unparseable paper (`check: source`), an include the
  build cannot load or that resolves outside the top `.tex` file's directory (`check: include`), an
  unclassified metadata key (`check: metadata-key`) and a pandoc that is missing or out of range
  (`check: pandoc`) ([Inference preflight](#inference-preflight)).

A few readings of the checkpoint commits follow from these rules:

- A fresh build over checkpoint commits a removed `kb-root/` left behind starts a set of its own: a
  resume continues the newest build's checkpoint commits, read back to its `start` boundary, and
  `status` and `build-trail` read those alone.
- `status`, `build-trail` and a resume read no checkpoint commits that a later removal of
  `kb-root/` ends: the commits they read lie above the newest commit that removed `kb-root/` and is
  no `kb-build:` boundary. With `kb-root/` absent, or the commits so ended, and no build run holding
  the run lock, `status` reports `none` with no stage done and no `current`, and `build-trail` no
  boundary and `started-at` null; a live run is reported as it is. A build that ends with
  `kb-root/` absent hands out no `resume` in its result. A reset to before the removal restores
  `kb-root/` and the old checkpoint commits with it.
- A fresh build where `kb-root/` is absent counts no deleted path under it as dirt, committed or
  not, and its first boundary commit records the removal; any other dirty path under `kb-root/` is
  refused (`check: dirty-paths`).
- `kbase build --resume` over a finished build's checkpoint commits is `unchanged`.

### Determinism

The same inputs and pandoc version give a byte-identical document graph: the tree
`document-graph` writes and the records. The claim-graph stages mint fresh node ids on every build —
unique within the KB, now and going forward — so a build is reproducible modulo
ids from `claims-declared` on, and no consumer may read anything into an id's value. The order in
which top `.tex` files and `--bibliography` files are given does not affect the result: the same
set gives the same document graph, and the same KB modulo ids.

**`--no-inference`** is first-class. It drops the steps that make model calls, still runs and
records every stage, and states the steps it dropped. The run is a real KB built without those
steps, not a stopped run. A build that `--no-inference` finished is a finished build:
`kbase build --resume` over it is `unchanged`, and its dropped steps never run on it. A build that
wants them starts over ([§5](#starting-over)) and is a fresh build without
`--no-inference`. A step that makes model calls in part is not dropped: its model calls are skipped
and the rest of the step still runs, as for `depends-attributed`.

**`--through <stage>`** stops the run at the named stage, given by id or display name. A bounded
run is not a failed one: outcome `bounded`, exit 0 (§7).

### Inference preflight

A build with a stage left to run that makes model calls refuses (§7, `refused`) before the first
stage it runs when no provider is configured (§9), naming the configuration it lacks
(`check: provider`); its `remedy` names `kbase show-config` as the diagnostic that shows how the
configuration resolved (§9.6). A build making no model calls needs no provider. Every model call a
build makes is one tool-less chat request answered from its prompt alone — except the claim-graph
multichoice model calls, which are each one `POST` to the decision endpoint the selected entry
names (§9.1) containing a fixed-set question (a `choice` or `noul` question over the offered letters)
and answered by probabilities over that set, not by reply text (§9).

**Refusals** (outcome `refused`):

- Unparseable source (`check: source`): names the paper and the reader error.
- Unloadable include (`check: include`): names each file and the line that named it.
- Include outside the volume (`check: include`): `\input` and `\include` resolve only inside the
  top `.tex` file's directory, the one pandoc runs in; one that resolves outside it — by an
  absolute path, by `..`, through a symlink, or below a first-level include — is refused, naming
  the include and the file that named it. A target that resolves inside and names no file remains
  the reader's own warning.
- An unclassified metadata key (`check: metadata-key`): a key the rendering's metadata block has
  that is neither content nor apparatus. A key pandoc's writer drops — an empty
  entry — reaches no block and is not refused.

### Bibliographies

With no `--bibliography`, the build passes every `.bib` in any top `.tex` file's directory, in
sorted path order, one set offered to every volume; `--bibliography` given replaces that set, read
in path order (above). **Degradation, not refusal.** An unreadable bibliography: the build
continues without it and reports the file.

**Under `--no-inference`** no `README.md` is written: the overview call is a dropped step, and the
README is assembled around its passage.

### Monitoring

At the command line `build` runs in the foreground, and SIGINT or SIGTERM stops it resumably; over
MCP the `build` tool starts it detached (§11). `kbase status`, `kbase cancel` and `kbase wait` (§6)
observe, stop and await it.

**What a build produces**: Markdown leaves; the index pages, up-link and cross-reference structure;
load-bearing forms; the claim graph's node and edge records, among them an analysis for each claim
whose bounds contain text beyond its statement proper; the summary pages.

### A build marks every claim

Every claim a build mints is hosted in the call that mints it: its
statement marker (§10) is in its leaf, a leaf's only claim and an equation included — an
equation's at the end of the line inside its display-maths fence that has its `\label{…}`,
never on the fence's closing line; a block claim's on its locator's line. The analysis split
(below) moves a prose claim's marker to just after the sentence its title is drawn from, on the line
that sentence ends on. It hosts each analysis it mints by a marker on the first blank line of the
blockquote that is its statement, or at the end of the last line of its claim's paragraph (§10).
Where either line closes a fenced block, the marker goes at the end of the line before it.

### A build records no judgement

Every scoring edge a build writes has `*pending*` as its
bearing, and no stage writes a `rigor` or any other number into a score; every value a build writes
is `manually-scored: false` ([§10](#provenance-and-the-judged-hash)). The judgement is a person's,
through `edit-<kind>` with `rigor` and `edit-edge` with `bearing`, or a scoring run's (§8). A claim
a build attaches an edge to reads unchanged until that edge is judged.

### Co-linearity cull (`claims-culled`)

To **cull** a claim is to merge it into another claim; the survivor of a chain of merges is the
claim's root. The stage runs between `references-found` and `depends-attributed`, and culls prose
claims that state no step a linked claim does not. Its candidates are the links — every marked
pair, every pair the unmarked-reference model call answered yes in either direction, and every
adjacency link, each unordered — with at least one end a prose claim. Each candidate gets one
co-linearity model call, which shows both claims in reading order and offers three answers: the
second adds nothing to the first, the first adds nothing to the second, or each is a step of its
own. An answer naming a claim as adding nothing — answered, or answered after a retry — culls it
into the other, except that a block claim or an equation node is never culled: where the answer
names one, the prose claim of the pair is culled into it instead. A candidate whose call yields no
readable answer after the retry is recorded `defaulted` and both claims are kept. A claim several
answers cull goes into the earliest of their partners in reading order; following those merges to
a claim culled into nothing gives its root. A claim whose root is not the claim it was culled into
gets one more model call against its root, and is kept unless that answer names it as adding
nothing; no claim gets a third call. A culled claim is removed — its outgoing and inbound edges,
then its node record and its statement marker — and no edge is added. The stage decides no direction:
every inferred link that survives, one with a culled end updated to refer to that end's root, goes
undirected to the cull record (§6) for dependency attribution. Under `--no-inference` the stage
still runs with its model-calling steps dropped: no call is made and the cull record is as the
declared pass wrote it.

### Dependency attribution (`depends-attributed`)

A captured link has no direction; only this stage's model call gives it one, or rules it a
mention. The candidates are every marked pair, directed as the text marks it, and every link of the
cull record (§6) whose two claims no marked pair joins either way round, directed from its later
claim in reading order to its earlier, an equation node always the target. Each candidate gets one
model call offering *supported by*, *in support of* and *mention*, less *in support of* where
containment directed the pair or its target is an equation node. *Supported by* writes a `supports`
edge from source to target, *in support of* one from target to source, and *mention* a `references`
edge. A candidate whose call yields no readable answer after the retry, and every candidate under
`--no-inference`, takes its deterministic default: *supported by* where containment directed the
pair, *mention* otherwise. The stage writes every edge its answers make, cycles included; last,
cycle breaking (§10) demotes one edge of each cycle those edges close — the back edge a depth-first
search finds — each a warning in the stage's report, before the stage's rebuild of the derived
views and verify.

### Analysis split (`analyses-split`)

The stage runs between `depends-attributed` and `phase-3a`. It makes no model call, so it runs in
full under `--no-inference`.

A claim's bounds are its own block or paragraph and the proof blocks that establish it. A proof
establishes the claims its opening names or, where its opening names none, the claim whose block it
follows. A claim's statement proper is the line the build titles it by: a block claim's display
line, or a prose claim's paragraph up to and including the sentence its title is drawn from.

A claim whose bounds contain text beyond its statement proper is split into the claim, which narrows
to its statement proper, and an analysis. An equation node is never split, and neither is a claim
whose bounds are its statement proper alone. The analysis's statement is the claim's own block or
paragraph where that contains text beyond the statement proper. Otherwise it is the first proof, in
reading order, that establishes the claim. Another proof establishing it is no part of this
analysis's statement; its text is a statement only where it is the first proof of some other claim
it establishes. A claim whose analysis's statement would be a blockquote containing no blank line is
left unsplit, there being no line to mark the analysis on; the stage's report counts each such
claim as `unsplit-no-blank-line`.

The claim keeps its record, every edge out of it and every `references` edge into it. The analysis
is minted with the claim's title, rigor `*pending*` and a rationale stating the split. Every
`supports` and `founds` edge into the claim is relocated onto the analysis. So does every `demoted` record
into the claim that replaced a `supports` edge, each with its source, context, bearing and
`manually-scored` unchanged. One `lifts` edge runs from the analysis to the claim, bearing
`*pending*`, `manually-scored: false`. Every path through a split claim before the stage runs
through its analysis after it, so the stage closes no cycle and removes no edge to break one.

### Records and inference

The reader and placement facts the claim graph needs are held as records in the build state store,
not in `kb-root/`. Each inference pass is one fixed-set decision per
model call under its embedded template, and the calls are made as
[Inference preflight](#inference-preflight) states. A stage's exit never depends on a model's
opinion.

---

## 6. Build state, ledger and monitoring

### Git holds the recoverable record

A build's **checkpoint commits** are its `kb-build:` commits on the current branch, one per stage
boundary, in the user's repository. Each has subject `kb-build: <stage-id> | <display name>` and a
structured body. Every boundary's body ends with the build's inputs: a `volume-root: <path>` line
per top `.tex` file, then a `bibliography: <path>` line per `--bibliography`, each in the order the
build reads them (§5), each path absolute. The `start` boundary's body also records `started-at:
<commit>`, the commit the build began from: the clean, ready-to-build state a fresh build over
other inputs starts again from. Resetting the branch to `started-at` discards the checkpoint
commits. On a branch with no commit, a fresh build first makes one empty commit, subject
`kb-start: the repository before the build` — outside `kb-build:`, so not a checkpoint commit —
after every refusal it can give and before its `start` stage, and records that commit as
`started-at`. A commit includes `kb-root/`, the build records in its `.build/` included
([§5](#output-and-placement)): the node-pass record `kb-build-node-pass.yaml`, the unmarked record
`kb-build-unmarked.yaml`, the classification record `kb-build-classification.yaml` and the cull
record `kb-build-cull.yaml`, YAML at `1.0.0` (§10). The cull record has `about`; `entries`, one per
co-linearity model call, sorted by `pass`, `a`, `b`, each with `a` and `b` (the two claim ids, `a`
before `b` in id order), `pass` (1 or 2), `verdict` (`a-adds-nothing`, `b-adds-nothing` or
`distinct`) and `outcome` (`answered`, `re-asked` or `defaulted`); `culls`, sorted by `id`, each
with `id` (a culled claim) and `into` (the claim it was culled into); and `links`, sorted by `a`,
`b`, every inferred link that survived the cull, each with `a` and `b` (id-sorted, with no
direction) and `harvest` (`unmarked`, `adjacent` or `culled`). The declared pass writes it with
every list empty. Resume position is read from the checkpoint commits. A user may reset to a stage
commit and resume from there. Per-leaf work inside an inference stage
stays uncommitted until the stage boundary; a resume over an interrupted stage restores `kb-root/`
to the last stage commit and re-runs the stage. **A build in flight owns
`kb-root/`:** a resume restores every path under it to the last boundary, whatever was done to it
since, and nothing vets what it finds there. An edit made while a build is interrupted is the
editor's to preserve in git, by waiting for the build or by reverting the commit that reset it; a write
op or `update-format` attempted while a build or a scoring run runs is refused (§8).
Commits are scoped by pathspec to `kb-root/`.

**The state store contains what does not belong in the repository** — the run record, which `status`
and `cancel` read: the run's pid, its start, the verb that owns it (`build` or `score`) and, for a
scoring run, its run id — `progress.jsonl`, per-call evidence, the document-graph records (§5), the
scoring runs under `score/` (each with its ledger and `captures.jsonl`, every call's item text,
prompt and reply, failures included; §8), a per-unit answer cache keyed by input hash, and the
captured output of a build the MCP server starts (§11). Nothing in it is authoritative for
position. The verb is required of the run record: one naming neither verb is a store another
version wrote, `refused` with `check: state-dir`, `path` the record, and never read as a build's.
The store is at `$XDG_STATE_HOME/kbase/<key>/` (`~/.local/state/kbase/<key>/` where the variable
is unset or not an absolute path), where `<key>` is the first 16 hex digits of the SHA-256 of the
absolute, symlink-resolved `kb-root/` path, overridable with `--state-dir`; a relative `--state-dir`
resolves against the repository root, and one inside `kb-root/` is refused (`check: usage`, `key`
`--state-dir`). The store contains cached model answers and survives `git clean`.

**The run lock** is the file `kbase-operation.lock` in the repository's git directory, recording
the state dir, and the lock file in the state store beside the run record. A repository has one
run at a time — a build or a scoring run, never both — whatever `--state-dir` each names. A clean
exit removes the lock files, cancel included, so a file in the store is never evidence of a live
run. Given no `--state-dir`, `status`, `cancel`, `wait` and `build-trail` read the store the run
lock names while its file exists and names one, else the default store; a new run never reuses it.
A second run is `refused` with `check: lock`, `path` the running run's state dir, and no remedy,
its `detail` naming the running run's verb and pid: "a build (pid 4321) is running; its state-dir
is …", or "a score run (pid 4321) is running; …". Where the run lock is held but names no state
dir yet, the second run gets `retry`; where it names one whose run record is not yet written, the
refusal says a run is starting. A run whose `--state-dir` store another repository's run holds gets `retry` (`check: lock`, `path` the store's lock file, `detail` naming the other run's verb, pid and start). `cancel`'s `retry` cases are at [§6](#kbase-cancel). Every lock is advisory and exists on Unix only; on Windows `status`
cannot see a running run and no lock-based refusal or `retry` is given.

A maintenance operation over an existing KB that runs no git command — the queries, `verify`,
`export-latex` and a write batch without `commit` — works without a git repository. The commands
that run git are refused `check: worktree` outside a git worktree; they are listed at
[Write ops](#write-ops).

### The state store is versioned

`<state-dir>/format` contains one integer, currently `1`, written
when a build first takes the run lock; a store with no `format` file is version 1. `build`,
`status`, `wait`, `cancel`, the MCP `build` tool and `build-trail` refuse a store newer than this
kbase: `check: state-dir`, `key` the `state-dir` argument, `path` the store, remedy "update kbase,
or use another `state-dir`", the argument in both written as [§7](#item)'s `key` states. A build
converts an older store in place before it uses it; the conversion is stepwise and resumes if
interrupted. The store's version is independent of the KB format version (§10).

**The state store is pruned only by a build or a scoring run.** At a build's start, the captured
output of builds the MCP server started (`reports/build-*`, §11) is pruned to the newest six by the
start each one's name records. At a scoring run's start, once its own run directory exists,
the run directories under `score/` are pruned by kind: a real run's id is `run-<id>` and a dry run's
`dry-<id>` (§8), a directory's kind is its name's prefix, and the invocation keeps, of its own kind,
16 runs — the run it names, created or resumed, and the newest 15 others by the start their
`run.yaml` records, equal starts ordered by name with the later name newer — and removes the rest of
that kind whole, captures included. The captured output of scoring runs the MCP server started
(`reports/score-*`, §11) is pruned at the same point to the newest 16 by the start each one's name
records. A dry run never removes a real run, nor a real run a dry one,
and a dry run an earlier kbase minted as `run-…` counts as real. A directory whose `run.yaml` is
absent or unreadable has no start to order by and is left. An invocation refused before its run
directory exists — a `--resume` finding no run to continue among them — removes nothing. A
resumed run's `captures.jsonl` loses a torn last line before anything appends to it, as its ledger
does. A build's answer cache, multichoice call records and call captures are kept in one
directory per build, named by the build's `start` boundary commit. At a build's start, before its
first stage, the current build's directory and the newest five others are kept, and the rest are
removed whole, along with the undivided cache, records and call captures an earlier kbase kept. At
the same point, a fresh build whose progress record exceeds 500 lines moves the earlier runs' lines
to a file under a timestamped name, keeping its own in the record; the 15 newest such files are
kept. The stage reports are kept. `status` and `cancel`
remove nothing.
Deleting the store is always safe, but it forfeits the answer cache, whose entries are cached model
answers; `status` reports its size.

### `kbase status`

`status` reports the current or last run, a build or a score run, whichever is newest. Its one
argument is `state-dir`; its keys are in [§7](#item). It is refused outside a git worktree
(`check: worktree`), with a `--state-dir` inside `kb-root/` (`check: usage`, `key` `--state-dir`)
and over a store newer than this kbase (`check: state-dir`, [above](#the-state-store-is-versioned)).

A build's `state` is one of:

- `none`: no run is recorded in the store; or `kb-root/` is absent, or the newest change to it
  removed it and no stage boundary follows, and no run holds the run lock;
- `running`: a run holds the run lock;
- `finished`: the newest run ended `done` or `unchanged`;
- `bounded`: it ended `bounded`;
- `cancelled`: it ended `cancelled`;
- `failed`: it ended `failed`, `refused` or `retry`;
- `interrupted`: it recorded a start but no end, and no run holds the run lock. It is the one state
  the build never writes.

A build's `stages` is a mapping: `done` and `not-done`, the stage ids each way in stage order, and
`rewind`, the one line saying that each completed stage's commit, with the command that rewinds
to it, is in `kbase build-trail`. The stages read are the newest build's checkpoint commits
([§5](#starting-over)). Where `state` is `none` because `kb-root/` is absent or was
removed, no stage is done, and `current`, `pid`, `started`, `updated`, `ended` and `resume` are
null. `recent-refusals`, `recent-fallbacks`, `cache-entries` and `cache-bytes` still read the state
store: they record earlier invocations, not a current build.

A build's `resume` is null where no identical retry continues the work (§7) and where the
recorded command does not parse as a `kbase build` invocation — at least two elements, `kbase` then
`build`. A run whose record fails that supplies none, as a run that recorded none does. `state` and
`resume` come from the build's newest run that counts: a refusal or `retry` that recorded no
boundary wrote nothing, so the run before it is the one reported.
Progress is persisted to a file so a late or reconnecting monitor catches up. An inference stage's
units and fallbacks are recorded once its model calls have all returned, so while they are in
flight `current` names the stage and its total with none done.

#### A score run's status

Where the store's run record names a score run, `status` reports that run
in its own shape instead of the build's: `state-dir`; `state` (`running` while the run lock is
held, else `interrupted` where the run left no result document, or the outcome
its result document records — `done`, `cancelled`, `failed`, `refused` or `retry`); `pid`;
`started`; `run`; `pass`; `counts` (`total`, `done`, `failed`, `skipped`, `remaining`, read from the
run's `items.jsonl`, the status `ledger.jsonl` last records for each, and, while the run is in its
item loop, each answered item's result file); `scored` (the items whose
value was written); `failures` (each failed item and the reason its line recorded, with the `remedy`
the line records where it records one, §8); and `resume` (the call
continuing the run, `score` with `--resume`, as its result document has it, where the run
stopped short of `done`). It is the run's own records re-read, not a second
document: `wait` prints the result document the run wrote beside its ledger. A store with no live
run, and whose run record names no score run, reports its newest run, whichever verb it was: the
later of the newest score run's start, read from its `run.yaml`, and the newest build invocation's,
read from the progress record — each the record the run already wrote. A run directory whose
`run.yaml` is absent or unreadable left no record of when it started and is skipped. Where the
store records neither run, or the two starts cannot be compared — a missing or unparsable stamp on
either side — it reports the build as above.

### `kbase cancel`

`cancel` signals the run that holds the run lock, of either kind — a build or a scoring run, as the
run record names it — and waits for it to let go. Its one argument is `state-dir`; its keys are in
[§7](#item). The event is logged, the lock is released, and the run process exits `cancelled`. The
run continues only when asked to, by `kbase build --resume` or `kbase score --resume`. Cancel costs
only the calls in flight, whose item is abandoned with nothing written for it: an answer already
returned is kept — in a build's answer cache, and in a score run's ledger; a cancelled score run's
`resume` is the call read from its result document. A store with no run is `refused`: `check:
lock`, `path` the store's lock file, detail "no run holds the state store". A run that holds the store but has not yet recorded itself, or that does not let go of it within 20 s of the signal, is `retry` (`check: lock`, `path` the store's lock file).

### `kbase wait`

`wait` returns when the run the pid names, of either kind, is no longer progressing — the store is
at another run, or at that run with a state it stopped at — then prints the run's document: the
store's `status` document for a build, and the result document the run wrote for a scoring run, so
a command run in the background has the run's terminal state when it exits. Its arguments are
`--pid`, required and positive, and `--state-dir`, which defaults as `status`'s. A missing or
non-positive `--pid` is refused: `check: usage`, `key` `--pid`. `wait` is a command-line command
only (§11). It takes no lock, and reads no checkpoint commits until that final document.

### `kbase build-trail`

`build-trail` lists the build's history, read-only. It runs none of the commands it lists. Its one
argument is `state-dir`; it takes no target, since every boundary and `started-at` has its own
command. The checkpoint commits it reads are the newest build's ([§5](#starting-over)).

Its result has these keys after `kb-root`, in this order:

- `state-dir`;
- `head`: the commit `HEAD` names, null on a branch with no commit;
- `boundaries`, oldest first, each with `stage`, `commit`, `subject` and `command`;
- `started-at`: `commit` and `command`, null where the checkpoint commits record none;
- `dirty`: `false`, or the worktree's changed paths, the first 20 named and the rest counted as
  `+N more`.

A `command` is a list of the command lines to run in order. A boundary's is `git reset --keep
<commit>` then the build resumed with the flags its last run was launched with, `--through`
excepted, each written as the interface takes it: `kbase build --resume` and those flags at the
command line, the `build` tool's call over MCP. `started-at`'s is the reset alone. `git reset
--keep` refuses rather than discarding uncommitted changes in the files the reset touches.

`build-trail` is refused outside a git worktree (`check: worktree`), over a state store `status`
would refuse (`check: state-dir`, or `check: usage` for a `--state-dir` inside `kb-root/`), and over
checkpoint commits that record a stage over an unrecorded earlier one (`check: trail`, as
[Starting over](#starting-over) states). `failed` (3) is a git failure.

---

## 7. Tool-result contract and exit codes

### Tool results

Every subcommand, `models` and the configure commands included, writes one YAML document to
stdout and nothing else. stderr is for humans. Keys are in fixed order; strings are quoted on emit;
a refusal enumerates every offending item. The `outcome` key is the contract. `--help` and
`--version` are human-facing and outside it; `mcp`'s stdout transports JSON-RPC frames (§11), though a
usage error on its own command line, before it serves, still prints a YAML refusal.

### Shared vocabulary

Every result document is one YAML mapping, with keys in this order:

1. `outcome`.
2. `kb-root`: the absolute path. Present on every subcommand that resolves a KB, which is all of
   them except `models`, `configure-providers`, `configure-models`, `show-config`, `probe` and the
   `auth` subcommands; `init` omits it from results written while no `kb-root/` exists (§8).
3. `migration`: a conditional record — `from`, `to`, `note`, `warnings` (one item per thing the
   conversion dropped or changed that the converted files do not show: `path`, `line` where it
   names one, `detail`; `[]` when none) — present on the result of every verb that reads a KB
   through the loader on an older format version, and absent otherwise
   ([§10](#one-version-read-and-written)): `update-format`, `verify`, `export-latex`, the queries,
   `score` and every write op — on `score` and a write op, whose run is refused while the
   conversion is pending, beside that refusal. `update-format`'s lists only the warnings its own
   keys do not (the per-subcommand keys below). It follows `kb-root` where the result has both.
4. The subcommand's own keys, in the order its row gives.
5. Exactly one of these, chosen by `outcome`, and it ends the document: `refusals` for `refused`
   and `retry`; `failures` for `failed`; `cancelled` for `cancelled` — a mapping of `stage` and
   `unit` on a build, and of `item` on a `score` run (§8). `done`, `unchanged` and `bounded` end
   with no such key.

Keys are kebab-case. The one exception is query data under `results`, which keeps the query data's
own names verbatim (`node_type`, `host_anchor`). The same fact always uses the same key:
`written` (files this call wrote, a path named once however many times the call wrote it — a
converted file a phase writes and a later phase rewrites is still one entry; kb-root-relative inside
`kb-root/`, absolute outside it; `[]` when nothing was written), `id` (one node's id),
`count` (a total before any limit), `resume` (a shell command that continues the work), `state-dir`
(the state store's absolute path).

### Item

Every refusal, failure and verify finding is one mapping with these keys in this order, a
key omitted when it does not apply; `detail` is always present, with at least one of `check`, `path`
or `key`:

| Key | Holds |
|---|---|
| `check` | The named check or refusal class: one of the values below |
| `path` | The file the item is about |
| `entry` | 1-based index into the values' `entry` list |
| `key` | The setting the item is about: a values key, written as its configuration file writes it, or a flag or positional argument, written as the interface the call came through takes it: `--name` at the command line, `name` over MCP; a positional `<name>` at the command line, as its usage line writes it; an item refusing a value given for a named thing may contain that value as given, as the auth verbs' items contain the provider name |
| `line`, `column` | 1-based position in the values document or in `path`. A refusal over MCP that names a document (`path`) keeps its `line` and `column` there; one naming no `path` has neither |
| `allowed` | The fixed set the value must come from |
| `remedy` | The command that clears the item, where one exists (e.g. `kbase update-format`, `git init`); on a configuration or provider refusal it names `kbase show-config`, after the command that clears the item where one exists; a word in angle brackets is a placeholder the one running it fills in, as `<command-that-prints-the-key>` ([§9](#a-refused-key)); a `kbase` command exposed as an MCP tool — in a remedy, its prose and a result field such as `resume` alike — is, over MCP, that tool's call: the tool's name, then its arguments as compact JSON, keys sorted, containing only the arguments that tool's schema has (a query op's `query {"operation":"<op>","arguments":{…}}`, `update-format {}`); at the command line it is `kbase <command>`, help included, its positionals, then each flag in name order, a true bool bare, a placeholder for a repeated positional `<name>...`; a command no tool exposes (`auth`, `wait`, `mcp`) is a command line on both interfaces. A command or argument that is a field's whole text is written bare; inside other text it is set off in backticks over MCP, and at the command line a command alone. An argument with its value reads `name: value` over MCP and `--name value` at the command line. The remedy of the refusal of an id no node has (`check: unknown-id`) reads "list the nodes that stand with `kbase find`, correct `<id>`, then re-run", `kbase find` written as the interface writes a command. A remedy naming a write batch reads at the command line as its values document piped in, `printf 'entry: []\n' | kbase write`, and over MCP as the `write` tool's call, its entries included |
| `detail` | One sentence; under `retry` and `failed` it ends with what the outcome asks of the caller: `re-issue the identical call later`, or `re-issuing the identical call will not fix it` |

A refusal lists every offending item. A usage error is one item with `check: usage`. A
provider-selection refusal's `key` is the `provider` argument, written as `key` states, and lists
every entry name as `allowed`.

**Check values.** An item's `check` names its condition:

| `check` | Condition |
|---|---|
| `agents-redirect` | `kb-root/CLAUDE.md` exists and is not the redirect ([§3](#stamped-documents)) |
| `axiom-heading`, `bare-id`, `dead-link`, `frontmatter`, `index-freshness`, `register-freshness`, `statement-marker`, `unknown-id` | `verify` faults ([§8](#verify)). `unknown-id` is also the refusal of an id that no node record has, and `statement-marker` that of an op that needs a hosted node's statement marker where it has none |
| `conversion` | a conversion cannot preserve a file's content (`update-format`, [§8](#update-format)) |
| `demoted` | a `verify` finding, not a fault ([§10](#the-demoted-relation)) |
| `dependency-cycle` | the entry would close a cycle among the scoring edges |
| `dirty-paths` | uncommitted changes in the paths the command needs clean |
| `error` | a failure item: a defect in the tool, its input or the environment (outcome `failed`) |
| `export-dir` | `export-latex`'s output directory contains a `gen-tex/` that kbase did not write |
| `include` | an `\input` or `\include` the build cannot load, or that resolves outside the top `.tex` file's directory ([§5](#inference-preflight)) |
| `index` | a record file missing from a KB, or a write while a conversion is pending ([§10](#one-version-read-and-written)) |
| `inference` | the build's overview call did not yield a usable passage (outcome `failed`) |
| `kb-format` | a KB whose format version is newer than kbase |
| `kb-root` | `kb-root/` exists where `init` or a fresh `build` would create it; a `--resume` with nothing to resume while `kb-root/` exists; a `kb-root/` with neither format-version file nor entry point |
| `lock` | the KB write lock or the run lock is held, or no run holds the store `cancel` names |
| `metadata` | a load error ([§10](#the-load-every-fault-refused)) |
| `metadata-key` | a metadata key in a paper's rendering that is neither content nor apparatus ([§5](#inference-preflight)) |
| `pandoc` | pandoc missing or outside the accepted range (§2) |
| `provider` | a provider or model reference the configuration does not resolve |
| `records` | the build's document-graph records are missing from the state store and the tree regenerated to replace them differs from the `document-graph` boundary's (outcome `failed`) |
| `source` | a paper the reader cannot parse ([§5](#inference-preflight)) |
| `state-dir` | a state store written by a newer kbase, or whose run record names neither verb ([§6](#the-state-store-is-versioned)) |
| `symlink` | a KB document that is a symbolic link |
| `trail` | the checkpoint commits record a stage over an unrecorded earlier one |
| `usage` | an argument missing, unknown, out of domain, or conflicting with another; a `--state-dir` inside `kb-root/` |
| `worktree` | outside a git worktree, or a `project-dir` that is not a worktree root, or a repository root whose `.git` git does not parse as the worktree root |
| `write` | a failure inside a write batch (outcome `failed`) |
| a stage id | a build stage's own checks found its products disagreeing (outcome `failed`); `path` the stage's report |

**Per-subcommand keys**, following `kb-root`:

| Subcommand | Keys |
|---|---|
| `build` | `state-dir`; `roles` (one record per role the build resolved, in role order — `decision`, then `text` — in the role record's shape below; absent where it resolved none, as under `--no-inference`); `through` (absent where this invocation ran no stage); `no-inference`; `resumed` (bool); `restored` (paths reset to the last boundary); `stages` (list of `stage`, `commit` — null if unrecorded — `dropped` (step ids), `report` (path to the stage's full findings in the state store); absent where this invocation ran no stage); `resume` (the call that continues the build, written as `remedy` writes a command: at the command line `kbase build` with `--resume` and the flags the invocation was given that a resume still needs, all in name order — `--no-inference` where given, `--ask-concurrency` where given, `--state-dir`, `--config-dir` and `--project-dir` where given, `--config-dir` written absolute; over MCP the `build` tool's call with `resume: true` and the same arguments but `config-dir`, which no tool takes — never the inputs or `--through`; absent where an identical retry continues nothing: `done`, `unchanged`, `bounded`, `retry`, a `failed` or `cancelled` run that ran no stage, and a `refused` run that recorded no boundary) |
| `status` | `state-dir`; `state` (the build `state` vocabulary of [§6](#kbase-status), `interrupted` included); `pid`; `started`; `updated`; `ended`; `no-inference`; `stages` (a mapping: `done`, `not-done`, `rewind`); `current` (`stage`, `units-done`, `units-total`); `recent-refusals`, `recent-fallbacks` (items, the newest 10 of each); `cache-entries`, `cache-bytes` (integers: the answer cache's entry count and total size in bytes, §6); `resume` (null for `finished`, `running` and `bounded`; read from the run §6 names, written as `build`'s for the interface reading it; `recent-refusals` items are written so too, an item an earlier kbase recorded as written) |
| `cancel` | `state-dir`; `pid`; `resume` (written as `build`'s; absent where the record's command does not parse as a `kbase build` invocation, §6; for a cancelled scoring run, the `resume` its own result document holds, null where it holds none, [§6](#kbase-cancel)) |
| `build-trail` | `state-dir`; `head`; `boundaries` (oldest first: `stage`, `commit`, `subject`, `command`); `started-at` (`commit`, `command`; null where none is recorded); `dirty` (`false`, or the changed paths, the first 20 named and the rest counted) ([§6](#kbase-build-trail)) |
| `init` | `written` (every path this call wrote — the scaffold, the two empty record files, the stamped documents and the output of the rebuild of the derived views, merged, deduplicated, path-sorted); `removed` (as `update-format`'s; `[]` on create) |
| `update-format` | `written` (includes every summary page, sheet and maintained-block document written); `removed` (files the KB no longer calls for — a summary page no node is rendered on, a sheet the volume-count rule drops, a file a format migration made obsolete (§10) — kb-root-relative inside `kb-root/`, repository-relative beside it; `[]` when none); `unfenced`; `sheet` (`drawn` where the sheets were rendered this call, else `not-drawn`: nothing was drawn, whether or not a sheet exists); `dot` (`"not found on PATH"`, present only when `sheet` is `not-drawn`); `commits` (as a write batch's); `uncommitted` (as a write batch's); and, where the format changed, `ids` (`from`, `to`: each work's and axiom's `3.0.0` id and the id minted in its place), `bare-ids` (`path`, `line`, `id`), `no-edge` and `strengthen-by` (`id`, `text`). `unfenced` onward are present only on `done` and `unchanged` |
| `verify` | `findings` (items the KB's structure has that are not faults — every `demoted` record (§10), as an item with `check: demoted`, `path` the summary page of the record's `target`, omitted where that target has no host, and `detail` the edge and the relation it replaced; `[]` when none); its faults are `refusals` items, each with the remedy [§8](#verify) names for it where one op clears it |
| `write` and every write op | `dry-run`; `entries` (one per batch entry, in order: `entry`, `operation`, `outcome` — `done` or `unchanged` — and on an add's entry ([§8](#add-kind)) `id` (the node it minted) alone but on an `add-axiom` entry, which adds `heading` (`marked` or `appended`), on an edit's entry ([§8](#edit-kind)) `id` (the node it edited) alone but on an `edit-work` entry that changes `label`, which adds `rewritten` (each line it rewrote a reference on, `path` and `line`, ordered by path, then line; `[]` where none), on a `move-node` entry ([§8](#move-node)) `id` (the node it moved) alone, on a `remove-node` entry ([§8](#remove-node)) `id` (the node it removed) and `removed-edges` (each edge record it removed, as its `edges` row), on an `add-citation` entry ([§8](#add-citation)) `document` and `line` (where it wrote the citation), `id` (the node cited) and `link` (the text written), on an edge write's and a `restore-demoted` entry ([§8](#edge-ops), [§8](#restore-demoted)) `source`, `target` and `relation`, the edge it names, and on a `restore-demoted` entry then `restored` (the relations it put back); `[]` where the batch did not apply); `written` (kb-root-relative paths the batch wrote, or under `dry-run` would write: records, leaves and derived views); `removed` (the rebuild of the derived views', as `update-format`'s); `sheet` and `dot` as `update-format`'s, where the rebuild ran; `commits` (each commit made: `commit`, `label`, `paths`; `[]` without `commit`); `uncommitted` (a detail line, present only where the batch commit failed) |
| `citations-of` | `count`; `results` (every citation of the node, complete, unsliced: `path`, `line`, `text` ([§8](#citations-of))) |
| `find` | `count`; `results` (every node the filters keep, complete, in the sort's order, cut only by `limit`: `id`, `node_type`, `title`, `label`, `host`, `strength`, `dependents` ([§8](#find))) |
| `edges` | `count`; `results` (every edge record the arguments keep, complete, in its fixed order: `source`, `target`, `relation`, `bearing` (as the record files write it: a number in `[0, 1]` or `*pending*` written as text, null only where the pairing has none), `context` (null where the edge has none), `manually_scored` (the record's flag ([§10](#provenance-and-the-judged-hash)), null where its record has none) and `replaced_relation` ([§8](#edges))) |
| `trace` | `count`; `results` (one row per edge the traversal reaches, complete, in the sort's order, cut only by `limit`: an `edges` record, then `depth`, `strength` and `contribution` ([§8](#trace))) |
| `show`, `stats` | `results` (the query's mapping; a node record has `node_type`, `id`, `title`, `host`, `host_anchor` and `summary` ([§8](#show)), then the keys its kind has: `rigor` and `strength` on a claim, with `build_status`, `strength_band`, `rationale` and `premise_count` after them; `rigor`, `strength`, `strength_band` and `rationale` on an analysis; `status` and `rigor` on an experiment; `label`, `rigor` and `rationale` on a work; `label` and `rigor` on an axiom, its `rigor` `1.0` by rule; `show` adds `manually_scored` ([§10](#provenance-and-the-judged-hash)) after `rigor` on a claim, an analysis and an experiment; `strength` is a number rounded to two decimals for reading, `rigor` the number the node holds; `stats`' mapping has one count per node kind — `claims`, `analyses`, `experiments`, `axioms`, `works` — then `edges`, `demoted_edges` and `subtree_aggregates` (per volume, `claims` and `experiments` ([§8](#stats))), then `rigor_manual`, `rigor_model`, `bearing_manual` and `bearing_model` (the records whose type has `manually-scored`, `*pending*` values included, counted by the flag), then `relations`, then `strength_bands` and its two ranked sections, whose records have `strength`; `relations` has one record per relation in this order — `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted` — each `edges`, the relation's row count, `walked` (bool), then on a scoring relation `bearings`: one record per distinct bearing its rows have, `bearing`, as the record files write it, and `edges`; numbers ascending, then `*pending*`, then null, `[]` where the relation has no rows) |
| `models` | `provider`; `provider-rule`, `provider-layer` (the rule that named the provider — `flag` or `sole-provider` — and the layer that supplied it; this verb always selects one); `models` (ids, sorted ascending); `warnings` (items: `path` the file or directory the configuration fault is against, `detail`; `[]` when none; present whatever the outcome) |
| `show-config` | `layers` (one record per directory this invocation read, in precedence order: `path`, `source` (`explicit`, `environment`, `repository` or `global`), `exists`, `providers-exists`, `config-exists` — the directory's presence and each of its two files'); `providers` (one record per entry, in name order: `name`, `base-url`, `api` (the chat protocol it speaks, `anthropic` where the entry is silent), `decision-api` (the decision API it speaks, `systemone` where the entry is silent), `key` (`store`, `file` or `none`), `key-file`, `upgrade` (for a `file` key, `cat <key-path> | kbase auth add <provider>`, the resolved path single-quoted where a shell would split or expand it), `layer` (the layer that supplied the entry), `usable`, `fault`; a provider a dropping fault removed has `name`, `layer`, `usable`, `fault` only); `config` (the merged choices: `models` (`decision`, `text`, `scoring` — each the reference `config.toml` states, null where unset); `sampling` (`text`, `scoring` — each `temperature` and `thinking` as `config.toml` states them, null where unset)); `selection` (`provider` and `rule` — `flag` or `sole-provider` — or `detail` where no provider was named); `roles` (one record per role in role order, in the role record's shape below, each with `base-url`, `sources` (`base-url`, `model`, `key`, and for a chat role `temperature`, `thinking` — the layer that supplied that field, one of `layers`' four names, `store` for a key the store supplied, null where the field is unset), a chat role's `temperature` and `thinking` (null where unset: kbase's default, §9.2) and `detail` where the role does not resolve); `diagnostics` (items: every fault, `path` the `providers.toml` that has it, the stored key file that does not open, with `remedy` the refused-key remedy ([§9](#a-refused-key)) where it is truncated or does not decrypt, or the store's directory where it exists and does not list; an entry dropped because the key file `providers.toml` names cannot be read has the same remedy (§9.4); a warning per entry whose stored key leaves its `apiKeyFile` unread, `path` the `providers.toml` stating it; and the resolution failure where a role does not resolve) |
| `configure-providers` | `provider` (lowercase); `layer` (`global`, `repository`, `explicit` or `environment`); `written` (`[]` with outcome `unchanged`); `models` (ids, sorted ascending, where it sent a request to the provider); `next` (a list of strings, where no key is stored or that request failed: [§9](#provider-commands), "Provider commands"); `warnings` (items: `path` the file or directory the configuration fault is against, `detail`; `[]` when none) |
| `configure-models` | `models` (`decision`, `text`, `scoring`, as the written `config.toml`'s `[models]` contains them, null for a role it does not set); `written` (`[]` with outcome `unchanged`); `warnings` (items: a configuration fault has `path` and `detail`; a model missing from its provider's catalogue has `key` the role and `detail`; `[]` when none) |
| `probe` | `provider`; `provider-rule`, `provider-layer` (as `models`'s); `selection` (the selection that answered: `decision`, `text` or `scoring` where a role flag or the default named it, else the `<provider>/<model>` reference `--model` named); `model` (the id requested); `temperature`, `thinking` (chat selections only: the effective settings — `temperature` a number; `thinking` `off`, `low`, `medium` or `high`); `first-token-ms`, `wall-ms` (integers: request start to the first non-empty content or reasoning delta — zero where none arrived — and the call's end-to-end time); `finish-reason` (null where the stream reported none); `reply` (the reply's text on a chat endpoint, and on a decision endpoint the answer's probability, chosen letter or type); `usage` (a record: `reported` — whether the provider sent a usage block at all, so a zero count is a measurement only where it is true — then `prompt-tokens`, `completion-tokens`, `reasoning-tokens`, `cached-tokens`) |
| `auth add`, `auth remove` | `provider` (the name given, folded to lowercase; never the key) |
| `auth list` | `providers` (the names with a stored key, in name order; `[]` when none) |
| `export-latex` | `written` (absolute paths; outside `kb-root/`); `volumes` (the volume directories exported, entry-point order); `report` (the path of the export report written under the out dir — one YAML list, every simplification and finding line the run produced, each naming the form, the source path and the reason) |
| `score` | `migration` (conditional, the shared vocabulary's; it comes before `state-dir`, on the refusal of a run over an older KB, which `update-format` converts first); `state-dir`; `pass` (the set it runs: one pass, or `all`; a named node's `rigor`, a named edge's `bearing`); `roles` (one record, `scoring`, in the role record's shape below; absent under `--dry-run`); `run` (the run's id); `ledger` (the run's directory, absolute); `counts` (a record: `total`, `done`, `failed`, `skipped`, `remaining` — the items by the status the ledger last records for each, so a resumed run reports the whole run); `ask` and `skip` (a dry run's alone: the items a run would make model calls for and the stale manually scored items it would skip, named as `scored` names them, in the item loop's order ([§8](#the-closing-record))); `scored` (the items whose value was written, in the item loop's order — a node's id, or an edge's `<relation> <source> → <target>`); `resume` (the call that continues the run — `score` with `--resume`, written as `build`'s, over MCP the `score` tool's call — keeping `--state-dir`, `--project-dir` where given, `--scoring-model` where given, and `--config-dir` written absolute, and no argument of the scope; present wherever an item failed or was not reached — a `skipped` item does not make it present — and it comes before the record that ends the document; absent from a dry run, and from a stop before the item loop) |

A `status` read over a score run reports the score shape of [§6](#a-score-runs-status) in place of
the build keys above.

### The role record

A verb whose endpoints are per role states them in `roles`, one record per role
it resolved, in role order — `decision`, `text`, `scoring` — with these keys in this order, each
omitted where it does not apply:

| Key | Holds |
|---|---|
| `role` | `decision`, `text` or `scoring` |
| `provider` | the provider's name |
| `provider-rule` | the rule that named it: `flag` (a flag overrode the reference) or `config` (the reference named it) |
| `provider-layer` | the layer that supplied the provider: `explicit`, `environment`, `repository` or `global` |
| `model` | the model id this invocation requested; null where no source named one |
| `base-url`, `sources` | `show-config` alone: the endpoint, and the layer that supplied each field |
| `temperature`, `thinking` | `show-config` alone, chat roles: the role's sampling settings, null where unset (§9.2) |
| `detail` | `show-config` alone: why the role does not resolve, where it does not |

`provider`, `provider-rule` and `provider-layer` appear together on every role that resolved: a pool
entry supplies every endpoint. `provider-layer`'s `environment` is the directory `$KBASE_CONFIG_DIR`
names (§9).

**The model is the one requested, never one observed.** `model` is the id this invocation requested,
and no result document or ledger line states one a server ran.

### Bounded results

`find`, `edges`, `trace` and `citations-of` return their complete answers, with `count` and
`results`: there is no default cut, no paging and no `offset`. A smaller answer comes from a
narrower question, a filter that means something. `edges` and `citations-of` return a fixed order,
stated with each. `find` and `trace` take `sort` and `limit`.

- `limit N`, a single integer, is allowed only beside an explicit `sort`, and returns the first N of
  that order: the N weakest, the N strongest. A negative N returns the last N, still in the sort's
  order, as a Python slice does. `limit` without `sort` is refused (`check: usage`), and so is
  `limit: 0`.
- `sort` names a key and `order` (`ascending` | `descending`) sets the direction; every key
  defaults to `ascending`, and every key is available both ways, so `limit` can take either end of
  any ranking. A pending value sorts below every number, for ordering only; its row still shows
  `*pending*`. A null, a value that does not apply to the row, sorts last in either order. Each
  query states its own sort keys.

`stats`' ranked sections stay at 10, and the `bearings` of its `relations` contain
one record per distinct bearing value. No other subcommand's result grows with the KB; `build`'s
per-stage detail goes to the `report` file, not stdout.

**Exit codes** derive from `outcome`:

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success, including a `--through`-bounded run |
| `refused` (including verify faults and usage errors) | 1 | Wrong input or KB state; nothing written past the last checkpoint |
| `retry` | 2 | `check: lock` where another writer holds the KB write lock past the wait (`path` the repository root); a run holds the run lock but has not yet recorded its state dir; another repository's run holds a state store shared through `--state-dir`; or `cancel` finds the run not yet recorded, or the run does not let go within the wait (§6); retry identically later. A run that holds the run lock and has recorded its state dir is not `retry`: it is `refused` with the `lock` item (§6) |
| `failed` | 3 | Defect in the tool, its input or the environment; retrying will not fix it |
| `cancelled` | 4 | Stopped on request; resumable |

## 8. Maintenance subcommands

Every subcommand below is reachable by an external agent harness as an MCP tool: a tool of its own, an operation of the `query` tool, or, for a write op, an entry of the `write` tool's batch (§11). `score` starts a detached child that makes one model call per item and returns once the run has started, with the run's identity, so the turn is not held for the run's duration ([§11](#tools) states the wait). The rules in this section govern each.

### Write batches

Every write goes through a batch, on every interface. An entry is one item of a batch.

**Shape.** A batch is an ordered list of entries under `entry`. Each entry names its op under `operation` and has that op's keys; a batch may mix ops. `dry-run` and `commit` sit beside `entry` as batch options. An empty batch is valid: it writes no record and rebuilds the derived views ([Writes leave nothing stale](#writes-leave-nothing-stale)), which brings them up to date after an author's edits to leaves.

**Interfaces.** Over MCP, the `write` tool takes a batch (§11). At the command line, `kbase write` takes a mixed batch, each entry naming its op, with `--dry-run` and `--commit`. Each write op also has its own subcommand (`kbase add-edge`, …), which takes a batch of that op alone with the same options; its entries have no `operation`, since the subcommand names the op.

**Order.** Entries apply in the order given. Each entry is checked against the records as the earlier entries left them. kbase never reorders a batch; the caller orders it so that each entry is valid at its position.

**No reference between entries.** No entry names an id that another entry of the same batch mints. Every add mints its node's id, for every kind ([§10](#five-node-types)), and the result gives each minted id beside the entry that minted it. Adding nodes and their edges takes two batches: the first adds the nodes, the second the edges. Between the two the KB is consistent, since a node with no edges is a valid node.

**Two phases.** Before anything applies, every entry's shape is checked: an unknown op, an unknown or missing key, a value's type, and a value outside a fixed set (`relation`, `status`, …). Every shape error is reported at once. Then the entries apply in order, each checked in full against the records as they are at that point, and the batch stops at the first refusal. An entry that would leave the records invalid is refused even if a later entry would repair it; the refusal's remedy names the order that works. No separate end-of-batch check exists. Each refusal item has the keys of [§7](#item), with `entry` the entry's 1-based position in the batch.

**All or nothing.** A batch that is refused or fails writes nothing. The rebuild of the derived views is part of the batch, so the records are never written while their derived views are stale. Each file a batch writes is replaced atomically — written whole, then renamed into place — so a crash leaves each file either old or new. A crash between the writes of two files is not atomic across files; git recovers that case ([Git holds the recoverable record](#git-holds-the-recoverable-record)).

**Refused before it applies.** A batch is refused, whatever its entries, over:
- a KB whose load fails (`check: metadata`, [§10](#the-load-every-fault-refused));
- a KB in an older format that is not yet converted (`check: index`, remedy `kbase update-format`; [§10](#one-version-read-and-written)); a dry run is refused too;
- a lock ([One write lock](#one-write-lock));
- a `kb-root/CLAUDE.md` that is not the redirect (`check: agents-redirect`, [§3](#stamped-documents)).

**Dry run.** `dry-run: true` does everything but write. Its result is identical to a real run's, refusals and removed edges included, and says `dry-run: true`. A dry run never commits: with `commit` given it makes no commit, and its result's `commits` is empty.

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
- A removal is a lookup that can fail. Nothing records what was removed, so a repeated removal and a removal of something that never existed cannot be told apart; both are refused. A restore is refused on repeat for the same reason: it consumes the demoted record, and the edge it leaves is indistinguishable from one added directly.
- The remedy of a refusal points at the query that lists what exists: `find` for nodes, `edges` for edges.

An add looks up first too. Where the item is not found, it is created. Where it is found:
- an identical add — one whose application would change no byte of `kb-root/`: the same values, the same flag and judged hash, and a node's statement marker already where the add places it — is `unchanged`, and reports the existing item as an add would have;
- any other add is refused (`node exists: <id>` or `edge exists: <relation> <source> → <target>`). Nothing is upserted: `edit-<kind>` and `edit-edge` change an existing item's values.

An edit that changes nothing, because every value given is already the current one, is `unchanged`: the item exists, and the caller asked for a state the KB is already in. Each batch entry reports its own outcome, so an `unchanged` entry is never hidden in a `done` batch.

So for `add-<kind>` ([§8](#add-kind)), `edit-<kind>` ([§8](#edit-kind)), the edge writes ([§8](#edge-ops)), `restore-demoted` ([§8](#restore-demoted)), `remove-node` ([§8](#remove-node)), `move-node` ([§8](#move-node)) and `add-citation` ([§8](#add-citation)): a repeated removal or restore is refused, and a repeated add, edit, move or citation is `unchanged`, each leaving `kb-root/` byte-identical. There are two exceptions. `init` creates once: a repeat where a `kb-root/` exists is `refused`, never `unchanged`. `update-format` requires a clean `kb-root/`, so a repeat before its output is committed — by its own `commit` or by hand — is refused `dirty-paths`; over a clean tree it already converted, it reports `unchanged`.

#### Writes leave nothing stale

Every write batch is followed by the rebuild of the derived views (strengths, summary pages and sheets), inside its all-or-nothing; no flag skips it. The rebuild does not repair or convert the KB. The writes of `score` ([§8](#writing-the-results)) and of the build (§5) are followed by it the same way.

#### One write lock

A write batch and `update-format` hold the KB write lock for the whole run: from loading the KB to the result, the rebuild of the derived views included. `init` holds it from before `kb-root/` exists — the lock sits on the directory that contains `kb-root/`, so it is available at creation — through the scaffold, the final rebuild and the initial commit to the result. `score` takes it around writing its results — every value the run produced, after the item loop — and around its rebuild at the start, never across the run ([§8](#writing-the-results)).

A writer that cannot take it within 30 s gets `retry` with `check: lock`, `path` the repository root, and the remedy. A write verb probes for a run holding the run lock ([§6](#git-holds-the-recoverable-record)) before it waits on the write lock and again after taking it: where one runs, the verb is `refused` with the `lock` item at once, taking no wait. A build waits out a write in progress, up to the same 30 s, before it writes (`retry` if it cannot); its own writes take no KB write lock. A scoring run's own writes make no run-lock probe, writing as the run that holds it. `verify` and the queries take no lock. On Windows no advisory lock exists (§6): neither the `retry` nor the run-running refusal is available there, and the operator must keep to one writer at a time.

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

**Identity.** A node exists where another node of its kind has its `title` anywhere in the KB; a work also exists where another work has its `label`, and an axiom where another axiom has its `label`. Title and label rules are defined in [§10](#five-node-types). An add of a node that exists is `unchanged` where it is identical ([§8](#idempotence)), its `id` the existing node's; otherwise it is refused "node exists" at `title` or `label`, its `detail` `node exists: <id>`. Entries apply in order, so a batch adding one node twice is `unchanged` at an identical repeat and refused at a differing one.

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
- Over an edge of that identity the entry is `unchanged` where its `bearing` and `context`, as the add would write them, are the edge's own, and the edge's flag and judged hash are those the add would write ([§8](#idempotence)); otherwise it is refused "edge exists", and `edit-edge` changes the values. Over a demoted record of that relation from `source` to `target`, the add writes the edge with its own values and deletes the record.
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
- a `line` past the document's end: at `line`. A `snippet` the line does not contain is refused at `snippet`. An identical repeat — the named line already holds, in place of the snippet, exactly the link this entry would write — is `unchanged` ([§8](#idempotence)); a citation of the snippet's text in another form or to another node is no match, and is refused as a line that does not contain the snippet;
- an `id` no node has: `check: unknown-id` at `id`, `detail` `no node <id>`, as an edit's is ([§8](#edit-kind));
- a hosted node with no statement marker, which has no link target: `check: statement-marker` at `id`;
- a quote or a paraphrase of a work or an axiom: at `snippet`, the remedy naming the naming form;
- a quote absent from the cited statement: at `snippet`, or at `quote` where it is given;
- a link that would not parse, whole, as one link to the target — an unbalanced `[` or `]` in the `snippet`, or in the title the naming form writes: at `snippet`, or at `id` for the title.

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

Writes a LaTeX project from the KB's current state into `--out` (default `latex-export/` beside `kb-root/`; a relative `--out` resolves against the project root (§9)). The KB is read under the format-version rules defined in §10; nothing under `kb-root/` is written; a KB whose load fails is refused with the load error. `--volume` is repeatable, each naming a volume ([§10](#documents-of-a-kb)); the named volumes export in entry-point order, and with no `--volume` every volume exports. A missing or out-of-range pandoc (§2) is refused `check: pandoc` before any write; any other pandoc error is `failed`. A `--volume` that is not a volume is a usage refusal, `allowed` every volume, and so is one naming a volume the entry point's list does not name, which the export does not reach (§7). The out directory may be any directory, the author's own LaTeX project included: kbase owns only its `gen-tex/` subdirectory and writes nothing else there but a missing `preamble.tex`. **Manifest refusal.** A `gen-tex/` that exists holding files without the export manifest `gen-tex/kbase-export.yaml` is refused whole, nothing written — `check: export-dir`, remedy: empty the directory or name another `--out`.

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

Each call is one tool-less chat request on the `scoring` role's endpoint, over the chat protocol that endpoint's entry names (§9.1): `openai` speaks `chat/completions` and `anthropic` speaks the Messages protocol. Every call a run sends to an `openai` endpoint sends the run's id as its `prompt_cache_key`, so a run's calls reach one provider and share one cache entry; the Messages protocol has no such field, so a run whose endpoint speaks it sends no key. A Messages call sets the completion cap §9.1 states; no other request is capped. The call sends the scoring role's sampling settings (§9.2) in its protocol's fields (§9.1), and thinking text is never part of a reply on either protocol. A call that does not complete is retried identically, up to three attempts in all, pausing 5 s before the second and 30 s before the third. What fails the item at once, on the one attempt that met it, is a provider's answer with a status that is not retryable — any non-2xx other than 429 and the 5xx class — an answer that reached the completion cap before any answer text, which an identical retry would cut off the same way, and a reply the parse refuses. `--resume` is how a user retries a failed item.

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

`done` when the run finished; `cancelled` when SIGINT or SIGTERM stopped it, with no line recorded for an item whose call was in flight or not yet made, and a line for every item answered before the stop; `failed` when an item's call or write failed, or the start rebuild did; `retry` (`check: lock`, [§6](#git-holds-the-recoverable-record)) while another run holds the run lock and has not yet recorded itself, while another repository's run holds a state store shared through `--state-dir`, or where the start rebuild could not take the KB write lock past the wait; `refused` where the run cannot go on:
- no scoring role in the configuration or the arguments (`key: models.scoring`);
- a `--pass` outside `rigor`, `bearing` and `all`; `--pass` beside `--node` or `--edge`, since a named item's pass follows from its kind; `--node` beside `--edge`; an `--edge` that is not three non-empty comma-separated parts; `--provider`; an `--ask-concurrency` under 1 — each `check: usage`;
- `--node` naming a work or an axiom, and `--edge` naming a `founds`, `references` or `demoted` edge, since no pass scores them (`check: usage`);
- `--node` or `--edge` naming a manually scored item without `--rescore-manual` (`check: usage`);
- a `--node` or `--edge` that names no node or edge (`check: unknown-id`);
- a `--resume` the list under [The ledger and resume](#the-ledger-and-resume) refuses;
- a run already holding the run lock ([The run's locks](#the-runs-locks));
- a pending conversion, and a load error ([Refresh and format](#refresh-and-format));
- a `--state-dir` inside `kb-root/` (`check: usage`, [§6](#git-holds-the-recoverable-record)).

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

## 9. Configuration

Two TOML files, `providers.toml` (§9.1, the endpoint pool) and `config.toml` (§9.2, the choices), are read under the strict-load contract (§9.3) from one or two layers. `<repository>/.kbase/` is read over `~/.config/kbase`. A directory `--config-dir` names, else `$KBASE_CONFIG_DIR`, is instead the one directory read, so an explicit choice isolates a run from a repository's own `.kbase/`. An invocation outside a worktree has no repository layer. `show-config`, `models`, `configure-providers` and `configure-models` run in any directory. `configure-providers` and `configure-models` write where [The configure commands' write location](#the-configure-commands-write-location) states.

API keys are in neither file. The credentials store (`kbase auth add`) contains a key under a provider's name, and an entry may name a key file instead (§9.1).

### Merging

The two files merge by two rules.

- `providers.toml` merges by provider name, and a provider's table is replaced whole: a repository table of the same name supersedes the global one entirely, never field by field, so a repository cannot inherit the global table's `apiKeyFile` for an endpoint it chose. A stored key belongs to the provider name, whatever layer supplies the table. Provider names compare case-insensitively (§9.1), so a repository table overrides the global table of the same name in any case. A table that a fault drops (§9.1) supersedes nothing: the lower layer's table of that name remains in the pool, and the fault is reported. A fault naming a provider that a higher layer supersedes is not reported: the pool's provider is the higher one, and a diagnostic about a value nobody reads is not a fact about this run.
- `config.toml` merges field by field: a value a higher layer states wins, and one it leaves out keeps the lower layer's value.

A layer that fails the strict load refuses the whole load, naming its file (§9.3): no lower layer is read, so a broken repository file never silently falls through to the global one.

### A leading `~` expands; nothing else does

In the `--config-dir` and `$KBASE_CONFIG_DIR` values, and in an entry's `apiKeyFile`, `~` alone is the home directory and `~` followed by a path separator is a path beneath it. `~user` is not supported, a `~` anywhere else is literal, and where the home lookup fails or its answer is not an absolute path, the string remains exactly as written.

### Global flags

Every subcommand accepts:

- `--config-dir DIR`: the one configuration directory read (the introduction above). The flag outranks `$KBASE_CONFIG_DIR`, and a blank value of either counts as unset.
- `--log-level debug|info|warn|error` (default `warn`).
- `--log-file PATH`: copies diagnostics to a file; console output is never redirected away.
- `--project-dir PATH`: the project root, the git worktree root that contains `kb-root/` or will. The subcommand's search for the repository starts there instead of the working directory. A path that is not a git worktree root is refused: `check: worktree`, `key` the `project-dir` argument formatted as the `key` rule in [§7](#item) states (an MCP call's `project-dir` is refused the same way), `path` the absolute path.
- `-h`/`--help`.

The root command also accepts `-v`/`--version`, which prints exactly the version string and a newline.

A relative path a subcommand takes (a volume root, `--bibliography`, `--state-dir`, `export-latex --out`, `--values`) resolves against the project root. `--config-dir` and `--log-file` resolve against the working directory, whatever `--project-dir` names, and a printed `resume` writes `--config-dir` absolute.

### Provider commands

`models`, `configure-providers`, `configure-models` and `auth` read and write the configuration. `show-config` is defined in §9.6 and `probe` in [§8](#probe). Each writes its result under [§7](#item). The inputs and outputs of all of them name a provider in lowercase (§9.1).

**`kbase models [--provider NAME] [--timeout DURATION]`** lists the selected provider's model identifiers, sorted ascending, as `models`. The provider is selected as [Provider selection](#provider-selection-is-the-catalogue-verbs-alone) states. `--timeout` bounds one request (default 30 s). `models` reports each configuration fault of the configuration it read as an item of `warnings` and as a line on stderr: `path` the file or directory the fault is against, `detail` what is wrong. A fault is a `providers.toml` entry dropped from the pool, a key file that is readable beyond its owner (§9.4) or cannot be read, a stored key file that does not open, or a store directory that exists and does not list ([The files are the one source of inference](#the-files-are-the-one-source-of-inference)). `warnings` is `[]` where there are none, and is present on every outcome, refusals included. A provider that refuses the call's key is a failure with the remedy [A refused key](#a-refused-key) states.

**`kbase configure-providers <provider> --base-url URL [--api anthropic|openai] [--decision-api systemone|decisions]`** writes a provider's table in `providers.toml`, creating the file where it is absent: it creates the table or sets the fields given. A run whose fields already have the values given writes nothing and is `unchanged`. **`kbase configure-providers <provider> --remove`** removes the provider's table. `--global` or `--local` chooses the layer written ([The configure commands' write location](#the-configure-commands-write-location)). Keys never pass through the command, and no command takes a key file's path: an entry's `apiKeyFile` is read from `providers.toml` and written by no command. Its result is `provider`, `layer`, `written`, `models`, `next` and `warnings`.

- Refusals, `check: usage`:
  - a write with no `--base-url`;
  - a `--base-url` containing credentials (`user:key@` or `key@`), with the `kbase auth add` pipe as the remedy and a detail that ends as a refused key's does ([A refused key](#a-refused-key)); this is checked before any other fault of the URL;
  - a `--base-url` that is no absolute `http` or `https` URL;
  - an `--api` other than `anthropic` or `openai`, or a `--decision-api` other than `systemone` or `decisions`, each with those values as `allowed`;
  - `--remove` beside an entry field;
  - a provider name that breaks the provider-name rule (§9.1), keyed `<provider>`.

  No refusal of `--base-url` names any part of it.
- `--remove` of a provider whose `providers.toml` has no entry is refused: `check: provider`, `path` that `providers.toml`, detail ``no provider `<name>` in <path>``, and `kbase show-config` as the remedy, which lists the entries that exist ([Idempotence](#idempotence)).
- Where the written entry has a key, stored or read from the file its `apiKeyFile` names, `configure-providers` sends a request to the provider under a 30 s deadline and gives the catalogue, sorted ascending, as `models`; this vets the connection. A failed request leaves the provider written, with the outcome `done`.
- `next` lists the steps that follow, as commands formatted as the `remedy` rule in [§7](#item) states. It is absent where there are none.
  - With no key, `next` is two commands: the `kbase auth add` pipe with the provider's name filled in, then `kbase models --provider <name>`.
  - Where the request fails, `next` is one entry naming the failure, then the `configure-providers` call for a wrong URL, the `kbase auth add` pipe for a wrong key, and `kbase models --provider <name>`.
  - Where the stack written into does not reload, `next` is one entry naming that and `kbase show-config`. Where the written entry does not load, it is one entry naming the fault's reason and `kbase show-config`. The written entry's own fault is both an item of `warnings` and, with its remedy, a step of `next`.
  - After `--remove`, where neither the layer written nor a layer beneath it still contains the provider, `next` has one entry per role whose reference names it, giving the role, the reference and the `configure-models` call that replaces it, including the removal's location flag; then, where a key file is stored under the name, one entry giving `kbase auth remove <name>`, saying where the file does not open.
- It reads only the stack it writes into (the stack is defined under [The configure commands' write location](#the-configure-commands-write-location)): a layer outside that stack that fails the strict load does not stop it. `warnings` lists every fault of the stack, as `models` reports them, on every outcome from its write or removal on; a refusal, or a write that fails, has none. `warnings` is `[]` where the stack does not reload after the write.

**`kbase configure-models [--decision REF] [--text REF] [--scoring REF] [--timeout DURATION]`** assigns a model reference (`<provider>/<model>`, §9.2) to each role given in `config.toml`, creating the file where it is absent. At least one role is given: a run naming none is a usage refusal. The roles not given are unchanged. `--global` or `--local` chooses the layer written ([The configure commands' write location](#the-configure-commands-write-location)). Its result is `models`, `written` and `warnings`.

- It sends one request to each provider named, `--timeout` (default 30 s) bounding each, and checks each reference against that provider's catalogue. A catalogue that cannot be listed is `failed`, with nothing written. A model missing from its catalogue is a warning, not a failure, since catalogues rotate: an item of `warnings` with `key` the role's flag and `detail` naming the role, the model and the provider, and a line on stderr.
- A reference's provider part is case-folded to lowercase and written so (§9.1).
- It refuses, before any request, a value that is no reference, else a provider that the stack written into does not hold. Each item is keyed by its role's flag, and every offending role is listed at once. A provider refusal has `path` the written layer's `providers.toml`, and as its remedy the `configure-providers <provider> --base-url <url>` call including the location flag this call was given, then `kbase show-config`; a provider that a fault dropped from the stack keeps the fault's reason ([Provider selection](#provider-selection-is-the-catalogue-verbs-alone)). A refusal writes nothing.
- `models` is the three roles the written file contains, null for a role it does not set.
- It reads only the stack it writes into: a layer outside it that fails the strict load does not stop it, and a stack that fails the strict load is `failed`, with no `warnings`. Once the stack loads, `warnings` lists every fault of the stack, as `models` reports them, then one item per catalogue miss, on every outcome; the list is `[]` where there are none.

**`kbase auth add <provider>`** stores the API key piped on its stdin for `<provider>`, a `providers.toml` entry name, replacing any key stored for it. The store is one file per provider, `$XDG_STATE_HOME/kbase/auth/<provider>-key.enc` (`~/.local/state` where the variable is unset or not an absolute path), named for the provider in lowercase, encrypted under a key derived for the OS user (obfuscation against a reader of the file, not a boundary against that user), and readable by its owner alone. `auth` is CLI-only: no MCP tool accepts a key ([§11](#served-text)).

- `add` reads the configuration as the provider commands do, and a configuration that does not load refuses it as it does for them. A name, case-folded to lowercase, that no `providers.toml` entry has, in the pool or dropped from it by a fault, is refused: `check: usage`, `key` the folded name, detail ``no provider `<name>` in providers.toml (names match in any case)``, `remedy: kbase show-config`.
- The key arrives on stdin and no other way. Each of the following is refused, `check: usage`, naming no part of the input, with the piped form (the provider's name filled in) as its `remedy`: stdin that is a terminal; input of more than 65536 bytes; input that is empty once surrounding whitespace is trimmed; a key that, so trimmed, still contains a whitespace or control character. The name refusals come before stdin is read, and a refusal writes nothing.
- A name that breaks the provider-name rule (§9.1) is a usage refusal, `key` the name as given and the detail the rule, never the name; its `remedy` is `kbase show-config` on `add` and `kbase auth list` on `remove`. A refusal of the argument count names no argument given.

**`kbase auth remove <provider>`** clears the stored key. Where none is stored it is refused ([Idempotence](#idempotence)): `check: usage`, `key` the folded name, detail ``no key stored for provider `<name>` ``, `remedy: kbase auth list`. **`kbase auth list`** names the providers with a stored key, in lowercase, as `providers`. `remove` and `list` do not read the configuration, so a key file whose entry is gone is still removed. No result names a stored key.

### A refused key

Nothing checks a key before a call. A provider that answers a call 401 or 403 has refused the call's key, a missing key and a wrong one alike, and the call fails as any provider refusal does ([§8](#a-provider-calls-failure-names-what-ended-it), `provider-refused`). Its failure item has `remedy` `<command-that-prints-the-key> | kbase auth add <provider>`, `<provider>` the provider the call sent its request to. The user runs the remedy, or an agent runs it with the command the user names or the `upgrade` that `show-config` gives. The item's `detail` ends, before the outcome's ask, by saying so, and that no one asks for the key, so it never enters a conversation.

The call failures that have it are a build's (the `inference-failed` stop of a claim-graph stage and the overview's `inference` failure), a scoring run's `failures` (from the attempt's ledger line, [§8](#the-ledger-and-resume)), `probe`'s failure, and the failure of `models` and `configure-models` to list a catalogue. A call ended by any other status has no such remedy. Neither does `configure-providers`' request, whose failure is its `next` ([Provider commands](#provider-commands)).

### Provider selection is the catalogue verbs' alone

`models` selects a provider to send a request to, not a role: a role is never chosen this way, and `config.toml` names no provider. The rule is `--provider`, else the sole provider. `models` refuses (`check: provider`) where it can select none: zero usable providers, two or more with none selected, or a `--provider` the pool does not hold.

- An empty pool, and a name the pool does not hold (in `--provider` or in a role's reference), point at the `providers.toml` that `configure-providers` writes into with no location flag ([The configure commands' write location](#the-configure-commands-write-location)), the file a provider's table belongs in. `configure-models`' value points instead at the `providers.toml` of the layer it writes, checked against that layer's stack ([Provider commands](#provider-commands)).
- A name the pool does not hold also has `key` the argument or `models.<role>` that named it, `allowed` the pool's provider names, a `detail` naming the table to add there (`configure-models`' says instead that its flag names a provider no `providers.toml` this call reads has), and a `remedy` naming `kbase configure-providers <name> --base-url <url>` (`configure-models`' including its location flag), then `kbase show-config`.
- Where that provider was dropped from the pool by a fault, the fault's own reason is the `detail`, with `kbase show-config` alone as the remedy.
- Two or more providers with none chosen list every provider name as `allowed`, with `key` the `provider` argument.

The argument, and each command a remedy names, are formatted as the `key` and `remedy` rules in [§7](#item) state. `show-config` reports any of these in its document and still exits 0.

### The configure commands' write location

`configure-providers` and `configure-models` write one layer's directory:

- With neither `--global` nor `--local`, a command writes the directory `--config-dir` or `$KBASE_CONFIG_DIR` names, whether or not it exists, the flag before the variable; else `~/.config/kbase`, even where a repository's `.kbase/` exists. `--global` writes `~/.config/kbase` and `--local` writes `<repository>/.kbase`; an explicit `--global` or `--local` outranks the variable.
- `--global` and `--local` together are a usage refusal keyed `--local`. Either beside `--config-dir` is a usage refusal keyed by that flag. `--local` outside a worktree is refused with `check: worktree`, `path` the directory the search started from, and `git init` as the remedy.
- `configure-providers`' `layer` names the layer written: `global`, `repository` (`--local`), `explicit` or `environment`.
- The stack a command writes into is the layer it writes and the layers beneath it: the global layer is beneath the repository layer, and nothing is beneath any other layer. The command reads that stack only.
- `configure-providers` creates `providers.toml` where absent, containing its comment header, an example entry commented out, and the entry written, and no other entry. `configure-models` creates `config.toml` where absent from its commented template: comments that explain the file and guide the `[models]` ids (the guidance `kbase://model-hints` serves, [§11](#resources)), an empty value for each role it is not given (which parses as unset), and `[asks]` and `[sampling]` commented out. Each otherwise edits the file under §9.5. A refusal writes nothing, and so does a `configure-models` catalogue failure.

### The files are the one source of inference

The endpoint and the model each role uses come from `providers.toml` and `config.toml` alone, and so do the settings a role's calls are made with: `[asks]` and `[sampling]` (§9.2). An entry's key is the one the store contains under its name, whatever layer supplies the entry; else the one in the file its `apiKeyFile` names, which an entry with a stored key never reads; else none. Every verb that reads the configuration reads the store.

- A stored key file that does not open is treated as no stored key (the entry's `apiKeyFile`, else none) and is reported by its path and what is wrong with it, never its content: as a diagnostic in `show-config` (§9.6), as an item of `warnings` and a line on stderr in `models`, `configure-models` and `configure-providers`, and as a warning line on stderr in `build`, `score` and `probe`. A stored key file that no entry's name matches is read by nothing and is not reported.
- A store directory that exists and does not list yields no stored key for any entry, and is reported the same way, by its path and what went wrong. It does not stop the configuration loading.
- No environment variable supplies an endpoint, a model or a key. `$XDG_STATE_HOME` chooses where the store is, never a key. A run isolated from the user's layers names its directory with `--config-dir` or `$KBASE_CONFIG_DIR`; that does not isolate it from the user's store, but `$XDG_STATE_HOME` does.
- The inference preflight ([§5](#inference-preflight)) names the lacking setting as the refusal's `key`: `models.decision`, `models.text` or `models.scoring`. `show-config` reports that refusal as a diagnostic rather than stopping (§9.6).

There is no plaintext-transport switch. When a key is configured and the base URL is `http://` to a host other than loopback, `build`, `score` and `probe` write one line on stderr saying the key travels in cleartext, and proceed (as §9.4's permission warning does).

### 9.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses. A provider name is one or more ASCII letters, digits, `_` or `-`, not beginning with `-` (`^[A-Za-z0-9_][A-Za-z0-9_-]*$`). A table whose name breaks the rule fails the file's load, naming the file and the entry.

**A provider name is case-insensitive everywhere.** Every input naming a provider is case-folded to lowercase without complaint: each table name read from `providers.toml`, `--provider`, the provider part of a model reference, the provider that `configure-providers` or `configure-models` is given, and the provider that `auth add` or `auth remove` is given. Every output names a provider in lowercase. Two tables of one file whose names differ only by case refuse the load, naming both names as written and the file.

An entry:

```toml
[<name>]
baseUrl = "..."        # required
apiKeyFile = "..."     # path to a key file, resolved relative to this file's
                        # own directory if relative; an entry's only key field
type = "inference"     # optional; the only accepted value, also the default
api = "anthropic"      # optional; "anthropic" (also the default) or "openai" —
                       # the chat protocol this server speaks
decisionApi = "systemone"  # optional; "systemone" (also the default) or
                               # "decisions" — the decision API this
                               # server speaks
```

A key inside an entry that this list does not name fails the file's load (§9.3). An entry that fails validation is **dropped** with a warning naming the entry and the reason (never key material), and the rest of the pool still loads. An entry fails validation when:

- its `baseUrl` is empty;
- its `baseUrl` contains credentials (`user:key@`), or is no absolute `http` or `https` URL with a host (the reason names no part of the URL);
- `type`, `api` or `decisionApi` is set to anything but its accepted values.

A failure to read the key file faults just that entry the same way. An absent `providers.toml` is an empty pool, not a load error.

#### The chat protocol

`api` names the wire protocol chat uses, chosen per server; it has two values. `anthropic`, what an entry silent on `api` gets, is the Anthropic Messages API, `POST <baseUrl>/messages`. `openai` is the OpenAI-compatible `chat/completions` API. The two differ in more than a path. Over the OpenAI-compatible protocol a thinking level is an effort hint that the server may exceed without bound. A Messages request has `budget_tokens` and a `max_tokens` ceiling, and the ceiling bounds thinking even where a server overruns the budget.

Every chat call and catalogue listing (`models`, `configure-providers` and `configure-models`, `probe`'s chat selections, a build's overview call, a scoring run's calls) uses its entry's `api`. The `decision` role's calls never do (below).

A Messages request:

- Requires a `max_tokens` cap. The allowance for the answer is `max(256, ceil(characters / 4))` over the characters of the request's messages and system text. A role whose `thinking` is not `off` (§9.2) adds its thinking budget: 1,024 tokens for `low`, 4,096 for `medium`, 16,384 for `high`, sent as `thinking: {type: enabled, budget_tokens: <budget>}`. Thinking `off` is sent as `thinking: {type: disabled}`; the field's absence would leave it at the server's default.
- Takes the system prompt as a top-level `system` string rather than a turn, so a system message is lifted out of `messages`.
- Sends `temperature`, the role's, 0 where unset, whatever the thinking level.
- Sends no `prompt_cache_key`, `chat_template_kwargs`, `reasoning` or `reasoning_effort` field.

This mapping targets servers that implement the Messages protocol: oMLX, LiteLLM-style proxies, OpenRouter. Anthropic's own endpoint rejects a temperature beside thinking, and its newer models may reject `budget_tokens`, `temperature` or `thinking: {type: disabled}`. A Messages entry's catalogue is `GET <baseUrl>/models`, whose `created_at` is RFC 3339.

An `openai` request contains the role's `temperature`, 0 where unset. Thinking `off` sends `chat_template_kwargs: {enable_thinking: false}` and `reasoning: {enabled: false}`. A level sends it three ways at once, since a server reads whichever it speaks: `reasoning_effort: <level>`, `reasoning: {effort: <level>}` and `chat_template_kwargs: {enable_thinking: true}`.

`decisionApi` is unaffected by `api`, and both decision APIs are OpenAI-shaped: the `decision` role's multichoice questions always go to the decision API's endpoint on its entry's base URL, authenticated as an `openai` entry is, whatever the entry's `api`. A server that speaks only Messages has no such endpoint, and the decision call fails with that server's HTTP status.

#### The decision API

`decisionApi` names which of two decision APIs the server speaks; each has its own endpoint. It changes nothing about chat, which `api` alone decides, and it names no model id: the decision role's model id stays in `config.toml` (§9.2). An entry silent on it speaks `systemone`.

- `systemone`: `POST <baseUrl>/systemone`, body `model`, `state` and `questions`, an object keyed by each question's own key. The response is an `answers` object keyed the same way.
- `decisions`: `POST <baseUrl>/decisions`, body `model`, `input` (the same composed state) and `questions`, an array of `type`, `name`, `instructions` and `choices`. A `noul` question is asked as a `predicate`. A `choice` question's criteria are its `choices`, each option a `value` with its guidance as the `description`, sorted by value. The response is an `answers` array, each entry having the `name` it answers: a `predicate` (a probability, parsed as P(true)), a `choice` (its choice, confidence and per-value probabilities), a `score`, or a `refusal` (a declined answer, with no value, which leaves the question unanswered).

An answer naming a question the request did not ask is refused under either shape. A question left unanswered is retried once, then takes its deterministic default ([§5](#co-linearity-cull-claims-culled)).

### 9.2 `config.toml` — the choices

```toml
[models]
decision = "<entry>/<id>"      # the claim graph's multichoice questions: a providers.toml
                               # entry name, then the model that entry's
                               # decision API serves
text = "<entry>/<id>"          # the overview passage
scoring = "<entry>/<id>"       # kbase score's model calls; required for score

[asks]
concurrency = 4                # optional; model calls one build or score has in flight at once, across every role

[sampling.text]                # optional; the overview passage's sampling
temperature = 0.0              # optional; a number
thinking = "off"               # optional; off | low | medium | high
[sampling.scoring]             # optional; kbase score's sampling, the same two keys
```

**A `[models]` value is a reference**, `<provider>/<model>`, split at the **first** `/`: a provider name is the name of a `providers.toml` table and contains none, while a model id may (`openrouter/anthropic/…`). Either part empty, or no `/` at all, is not a reference and is refused at resolution, naming `models.<role>` and the form. Nothing is normalised but surrounding whitespace and the provider part, case-folded to lowercase (§9.1): a model id keeps the provider's formatting, not kbase's.

A role left unset resolves to "not configured": no defaulting, no guessing, and no role falls back to another.

`concurrency` is how many model calls one invocation of `build` or `score` has in flight at once, across every role. Absent, it is 4. `--ask-concurrency` overrides it for that invocation, and a flag value under 1 is a usage refusal keyed by the flag. A `concurrency` under 1 fails the load, naming `asks.concurrency`. Calls start in the order the invocation submits them.

#### Sampling is per chat role

`[sampling.text]` and `[sampling.scoring]` each take `temperature` and `thinking`, both optional. A key left unset is kbase's default, temperature 0 and thinking off, never the provider's. They merge field by field like every `config.toml` value, each keeping the layer that supplied it. There is no `[sampling.decision]`: the decision role's fixed-set probabilities take no sampling, and a `sampling.decision` key fails the strict load as unmodeled (§9.3). A `thinking` value other than the four fails the load, naming `sampling.<role>.thinking` and the four. Resolving a role refuses, naming `sampling.<role>.temperature`, a temperature outside the range its entry's chat protocol accepts: 0 to 2 for `openai`, 0 to 1 for `anthropic`.

### 9.3 Strict-load contract

Both files are decoded strictly: any key or table the schemas above do not model fails the **whole file's** load. The message names the file path and every offending key by its full dotted path (for example `models.typo_key`), as "unknown key" or "unknown keys" when more than one. A malformed-TOML file fails with a parser-detail message naming the file and line. Only key **names** appear in either message, never values: a file may still contain a secret a hand edit left behind, and the refusal that names its key is exactly where that value would be echoed.

A verb that reads the configuration and meets a file that fails the strict load is refused before it does anything else: one `check: usage` item whose `detail` is the message. Neither configure command repairs a file that fails the strict load; the remedy is a hand edit.

A layer that supplies a provider supplies its `apiKeyFile` path too, and kbase confines no key file's location; and a stored key goes to the endpoint of whichever table has its name, in whatever layer. Trusting a cloned configuration means trusting the key file it names, and the endpoint it would send a stored key to, as much as the endpoint it names.

### 9.4 Key-file permission warning

When a `providers.toml` entry sets `apiKeyFile`, the store contains no key under its name, and the host is not Windows, kbase checks the resolved key file's permission bits. If they grant group-read or other-read, it reports a warning (as a fault, [Provider commands](#provider-commands)) naming the key file by its path and as the one `providers.toml` names, never by the field that states it, with the offending mode and recommending `chmod 600`. The entry stays usable. A key file that cannot be read (§9.1) is named the same way, with the refused-key remedy ([A refused key](#a-refused-key)). This is the only file-permission behaviour kbase applies to a file it did not create.

### 9.5 The configure commands' targeted update (byte-preservation contract)

`configure-providers` and `configure-models` never re-serialize the file they write; each edits the existing bytes in place, keeping every byte it was not asked to change.

- An absent or blank file gets its template ([The configure commands' write location](#the-configure-commands-write-location)), then the edit below.
- `configure-models` changes **only** the `[models]` keys of the roles given. `configure-providers` changes **only** the keys it is given (`baseUrl`, and `api` and `decisionApi` where given) in the table its provider names, matched whatever its case; an entry's other keys, `apiKeyFile` included, are unchanged.
- A key already set to the value is not rewritten, so a run that changes nothing writes nothing and is `unchanged`. Every other byte (comments, unrelated keys and tables, spacing, key order) is preserved exactly. A rewritten line's own trailing `# comment` is kept; its column position is not guaranteed.
- A missing key is added after the last key of its table; a missing table is appended at the end of the file.
- `configure-providers --remove` removes the provider's table: its header, through its last key and the lines directly below that key with no blank line between, and one blank line that would otherwise be left doubled. A comment set off by a blank line before the next table is left for that table.
- Before writing, kbase decodes both the original and the proposed file as TOML. Unless the only difference between the two decoded structures is exactly the values it intended to change, the write fails (`failed`) and the file is untouched. A file that is not valid TOML fails the same way.
- The write is atomic. A file that is a symlink has its target rewritten; the symlink remains.
- Any failure along this path leaves the file completely unwritten.

### 9.6 `kbase show-config`

`kbase show-config [--provider NAME]` reports how this invocation's configuration resolves, and changes nothing: it reads no KB, writes no file and makes no network request. `--provider` names the provider the selection reports. Its result keys are the `show-config` row of [§7](#item). A configuration that fails the strict load is refused before the verb runs (§9.3). Otherwise the outcome is `done` whatever the report finds: a fault in the pool and an unresolved role are diagnostics, not refusals, so one call shows every gap at once.

The report states:

- the layers this invocation read, in precedence order, each with its path, the source that named it (`explicit` for a flag, `environment` for `$KBASE_CONFIG_DIR`, `repository` or `global`), and whether the directory and each of its two files exist;
- `config.toml`'s merged `[models]` values, null where a role is unset, and its `[sampling]` values, null where unset;
- every provider in name order, with the layer that supplied it and the chat protocol and decision API it speaks, each default filled in (so a provider silent on `decisionApi` is reported as `systemone`), including a provider that a dropping fault removed (unusable, with the fault's reason);
- the provider the selection names and the rule that named it, or why none was named;
- one record per role: the endpoint it resolves to, the layer that supplied each field, a chat role's sampling settings, or why it does not resolve.

The sources vocabulary is the layers': `repository` or `global` for a value a file supplies, `explicit` or `environment` for one read from the one directory a flag or `$KBASE_CONFIG_DIR` named.

Its diagnostics are, in order:

- one `provider` item per fault, with `path` the `providers.toml` that has the fault (which need not be the file a selection or `configure-providers` names). For a stored key file that does not open, `path` is that file, `detail` is what is wrong with it and, where it is truncated or does not decrypt, `remedy` is the refused-key remedy ([A refused key](#a-refused-key)) with the provider's name filled in. For a store directory that exists and does not list, `path` is that directory and `detail` is what went wrong, with no `remedy`;
- one `provider` item per provider whose stored key leaves the `apiKeyFile` it states unread, `path` the `providers.toml` that states it and `remedy` removing that line from the file, or `kbase auth remove <provider>` to use the file;
- one item per role that does not resolve, its `path` naming the file the setting belongs in: the write target's `providers.toml` or `config.toml`.

The upgrade (below) and the unread-`apiKeyFile` warning appear in this report only: no other verb's output has them.

#### Names and paths, never secrets

A key is reported by the kind it is (`store`, `file` or `none`) and a key file by the path its entry states, never by its content. A `file` key's entry has `upgrade`, the command that relocates the key into the store, with its values filled in: `cat <key-path> | kbase auth add <provider>`. The stored key then outranks the file. An agent runs the command as printed ([§11](#served-text)). `kbase auth add` runs from a shell with the key piped on stdin; no MCP tool accepts a key (§11).

#### It describes the process that ran it

The layers, the environment and the selection are this invocation's, so a call through the MCP server (§11) reports the configuration of the server process. `--config-dir` and `$KBASE_CONFIG_DIR` are set where the server is launched; no tool takes either.

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
to the values their decoded-values goldens hold.

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

- the entry point, `entry-point.md`, which lists every volume;
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
- What an entry shows is presentation: the node's current state — the leaves that host it, then
  its authored values under `### Rigor`: an experiment's `status`, the rigor (marked where manually
  scored), the edge lists and the rationale where the node has them. Only a claim's and an analysis's
  entry has edge lists: the `supports` and `founds` edges, the `lifts` edges, the `refutes` edges and
  the demoted records into the node, and the `references` edges out of it, each with the other end's
  title, its strength where the edge is a scoring edge and the other end has one, and the bearing.
  A claim's or an analysis's entry then has a `### Strength` section with its derived strength line
  and trace; no other kind's entry does.

### The record files

Each is one YAML document: a block list of block mappings, `[]` when it holds none. A node record's
type is its `id`'s prefix and an edge record's its `relation`; no record has a `kind` field. Every
field its type has is present and any other field is refused; an empty value is written as `""` or
`[]`, never omitted, so no reader takes an absent bearing for the judgement `0`. Strings are
double-quoted, escaping `\`, `"`, every control character below U+0020 but tab, and the characters a
YAML reader would take as a line break or refuse as unprintable, as `\uXXXX`; a string holds no line
break, and a stored prose value has each whitespace run collapsed to one space, its ends trimmed —
an op's `unchanged` test compares the collapsed value. Numbers are written in the shortest form that
parses back as the value (Python's float repr): a plain decimal with at least one fractional digit
(`1.0`, `0.85`), except below 1e-4, where it is exponent form (`5e-05`); booleans `true` and
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
`line`, the remedy to re-read the document and name a line in it. A line that does not contain the
snippet is refused at `snippet`, and the refusal names up to five lines holding it, those nearest the
named line, since the document has likely changed since the caller read it; the remedy is to re-read
the document and name the line holding the snippet. An op that places a marker also refuses a fenced
block's opening or closing line (a display-maths fence included) at `line`, naming the line, the
remedy to name another line. Each op's own refusals are stated with
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
  write ops refuse a title or a label that another node of the kind holds (`node exists` on an add
  that is not identical to that node, naming it); the load does not check uniqueness. The build gives a repeated title the
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
removes the first `supports` edge into an analysis on the cycle where there is one, else the back edge the search finds, replacing it with its `demoted` record.

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

## 11. The MCP server

`kbase mcp` serves kbase to any external agent harness that speaks the Model Context Protocol, over stdio JSON-RPC. It exposes the subcommands: every subcommand it serves is a tool of its own, an operation of the `query` tool, or, for a write op, an entry of the `write` tool's batch, with the same arguments and the same result, so an external agent harness without MCP loses nothing but convenience. No tool is listed without a subcommand behind it.

### Tools

- **The tools** are `write`, `query`, `build`, `score`, `status`, `cancel`, `build-trail`, `verify`, `update-format`, `export-latex`, `init`, `configure-providers`, `configure-models`, `models`, `show-config` and `probe`. Every tool but `query` and `write` is one subcommand, named as the subcommand.
  - `query` is a **family tool**: one MCP tool with an `operation` argument that names one of several subcommands, each an operation of the tool. Its operations are the queries `citations-of`, `edges`, `find`, `show`, `stats` and `trace`.
  - `write` runs a batch whose entries name the write ops ([§8](#write-batches)); a write op has no tool of its own.
  - Each operation stays a subcommand, unchanged on the command line.
- **CLI-only commands.** Every other subcommand is served. These are not, and no tool exposes them:
  - `auth` and its subcommands: a key arrives on stdin, which no tool call has, and no tool accepts a key;
  - `wait`: a tool call would hold the turn for the build's or score run's duration, so a session polls `status` instead;
  - `mcp`, the server itself;
  - `help` and `completion`, which belong to the CLI framework.

  The root help and `mcp`'s help name `auth`, `wait` and `mcp` as the CLI-only commands ([Served text](#served-text)).
- **A command tool's input schema** has one property per positional and per flag of its command, named as the command line names them without the dashes. The global flags are excluded, since the server has its own, except `project-dir` ([Two modes](#two-modes)). A duration flag's argument is the Go duration string the command line takes (`30s`). A flag is optional in the schema unless its command declares it required, as `trace`'s `direction` is. Each property has a description: a positional's is the one its command declares; a flag's is its usage, followed by the default `--help` prints for the flag where it prints one and, for a duration flag, by the note that its argument is a Go duration.
- **The `query` tool's schema** has `operation`, required, a string enum of its operations' names, and `arguments`, an object holding what the operation takes: its positionals and flags by name, derived as a command tool's schema is. `operation`'s description names where each operation's help is, and `arguments`' names that help's argument list. The tool checks `arguments` against the operation's own schema: an unknown key, a missing required key or a value of the wrong type is a refusal document, `refused`, one item per fault, `check: usage`, `key` the argument's name, `allowed` every argument the operation takes, and `remedy` naming `allowed` for the argument names and the operation's help resource for what the operation does. An `operation` outside the enum, or an `arguments` that is not an object, fails the tool's schema.
- **The `write` tool's schema** is the batch: `entry`, required, an array of objects, each naming its op under `operation` beside that op's keys; and `dry-run` and `commit`, booleans. The schema gives an entry's keys no types. The batch checks the entries' shape ([§8](#write-batches)), every fault an item naming its `entry`, and a wrong value for a key is the op's own refusal (§8), which names the key and, where the value comes from a fixed set, lists that set as `allowed`.
- **Every tool's schema** has one more property, `project-dir`, the project the call runs in ([Two modes](#two-modes)).
- **Results.** A tool's result is the §7 document ([§7](#tool-results)), as the result's text and as structured content. `isError` is set exactly when the document's exit code (§7) is not 0, so a refusal reaches the model as an error with its items and remedy, while `bounded` and a completed `cancel` do not. A refusal's items are defined in [§7](#item), `line` and `column` included. Arguments that fail the tool's schema (a missing required key, a wrong type, an unknown key) are JSON-RPC `-32602` errors, not refusal documents; a `query` tool's `arguments` are checked as above instead. A list query answers completely, or is sliced by `limit` as [§7](#bounded-results) states.
- **Read-only annotation.** The read-only tools have the read-only annotation: `query`, `verify`, `status`, `build-trail`, `show-config`, `models` and `probe`. A family tool is read-only where every operation is.
- **Descriptions.** A tool's description, and the help it points to, are defined in [Served text](#served-text).
- **`build` and `score` are detached.** A call starts `kbase build` or `kbase score` as a process of its own, from the root of the call's project, passing `--project-dir` where the call named one, in its own process group, with no stdin, configured as the server is (the environment, `--config-dir`, `--log-level` and `--log-file`, §9). The run's stdout and stderr are captured as files in the state store's `reports/` (§6); the capture files are pruned as §6 states. A relative `state-dir` argument resolves against the repository root; the server's own `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status` would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that refusal and starts no process. A run starting during a write waits that write out before its first stage (`build`) or its first rebuild of the derived views (`score`), and is visible to `status` while it waits.

  The tool waits at most 5 s, until the run has started or has exited, and returns by what happened first:
  - *started*: outcome `done`.
    - `build` has entered a stage, as its progress record shows. The result has `kb-root`, `state-dir`, `pid`, `resume` and `wait`; `resume` is absent where the record's command does not parse as a `kbase build` invocation (§6).
    - `score` has its ledger and pid. The result has `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait`.
  - *exited*: the run's own document, `isError` set by its outcome (§7).
  - *still starting at the bound*: outcome `done` with `state: starting`, `state-dir`, `pid` and `wait`. The run keeps running, and `status` follows it.

  `wait` is the shell command that runs the server's own executable as `wait [--project-dir <root>] --state-dir <store> --pid <pid>`, with `--project-dir` where the call named one, shell-joined as `status` joins its `resume` (§6). It returns when the run is over and prints the run's document: for a build, the store's `status` document; for a scoring run, the run's own result document ([§6](#kbase-wait)), so a session runs it in the background and is woken when it exits. `status` and `cancel` follow the start.

  `score`'s `edge` argument is a mapping of `source`, `target` and `relation`, each a required string and no other key allowed, which the run receives as `--edge <source>,<target>,<relation>`. A value holding a comma is refused at `edge`, `check: usage`, and starts no process.
- **`build-trail`** is a read-only tool; its result, refusals and commands are defined in [§6](#kbase-build-trail).
- **What the server never does.** It sends no sampling request and offers no prompts. No inference runs from any tool but `build`, `score` and `probe`, and each tool's inference is its own: the build's decision and text calls, the score run's scoring calls, the one call `probe` makes. `models` reads the selected provider's model catalogue; `configure-providers` reads the catalogue of the entry it writes where a key is stored for it; `configure-models` checks each reference it writes against its provider's catalogue. None of them makes a model call. The server writes nothing but JSON-RPC frames to stdout; diagnostics go to stderr and `--log-file`. It loads no configuration itself: the verbs it exposes load their own on each call (§9.3).
- **Protocol.** The server implements one MCP revision, `2025-11-25`. An `initialize` naming another revision is answered with the server's, for the client to decide. Calls are answered one at a time, in order. EOF on stdin ends the server.

### Two modes

The server has a default project or it does not. With a default project, a call may leave out `project-dir`. Without one, every call names its project.

- **With a default project.** `kbase mcp` found one, searching upward from its working directory for the repository root. `project-dir` is optional in every tool's schema, and a call that omits it runs in the default project.
- **Without a default project.** `kbase mcp --project-dir-required`, or a server whose search finds no repository. `project-dir` is in the `required` list of every tool's schema, its description says only what it names, and the instructions say how a session gets the path. A server that finds no repository serves exactly as one given `--project-dir-required` does, instructions included; the flag forces this mode from inside a repository. A server with no default project lists no sheet ([Resources](#resources)).
- **Startup never fails for want of a project.**
- **No project flag.** The global `--project-dir` given to `mcp` is a usage refusal (§7) before it serves.
- **`project-dir`** is the absolute path of the project's git worktree root, the directory holding `.git`, where `kb-root/` is or will be; it is never `kb-root/` itself. A call runs in the project it names where it names one, else in the default project. A call naming an empty one where there is no default project is `refused`, `check: usage`, `key: project-dir`. A `project-dir` that is no git worktree root is `refused`, `check: worktree`, `key: project-dir`, `path` the absolute path; a relative one resolves against the server's working directory.
- **Registration** is done by the external agent harness: one server entry, `kbase mcp` (with a default project, run from inside the project) or `kbase mcp --project-dir-required` (without one, for a harness that serves many projects from one place). README gives entries for two external agent harnesses as examples.

### Resources

| URI | Type | Listed | Contents |
|---|---|---|---|
| `kbase://sheet/<kb-root-relative path>` | `image/svg+xml` | with a default project whose KB opens | each claim-graph sheet ([§3](#claim-graphsvg)) present on disk, the placeholder included |
| `kbase://model-hints` | `text/markdown` | always | guidance on choosing a model for each role, in the role vocabulary (§9.2), the same rows a created `config.toml` has |
| `kbase://help/<command>` | `text/plain` | always | one per command exposed as an MCP tool, each write op and query operation included: shared body, argument list and MCP note ([Served text](#served-text)) |
| `kbase://help/query` | `text/plain` | always | the `query` tool's operations, listed from the commands it serves, then what they share |
| `kbase://help/write` | `text/plain` | always | `kbase write`'s own help, which has the batch contract in full |

- `resources/read` of a sheet returns the SVG as text. A KB that exists but will not open lists no sheet: the list is answered without one, and that KB's unopenability is reported by `status` and `verify` when they are asked, not as a list fault.
- A help resource is named `<command> help` (`query help` for the `query` tool's). The `query` tool's shared help states, as MCP's arguments name them, how a query reads the KB and answers; `kbase --help` states it as the command line names the arguments, and no operation's own help states it.
- A help resource's argument list is generated from the arguments the command's tool takes: a write op's is an entry's keys, `kbase write`'s the batch's. The command's `--help` has the same list, formatted as the command line formats it.
- Model guidance and help are KB-independent text: listed whether or not a KB is found or opens, and the same in every project.
- No resource is subscribable. `initialize` advertises the `resources` capability with no `subscribe` among the ones it has. Nothing announces a run's end, so a session polls `status`, or runs in the background the `wait` command `build` and `score` return. `resources/subscribe` and `resources/unsubscribe` are methods the server does not answer (`-32601`).
- An unknown URI is JSON-RPC `-32002` (resource not found); a missing `uri` is `-32602`.
- Nothing else is served, a summary page included: the KB's documents are the external agent harness's own files.

### Served text

Served text is what a session reads without calling a tool: the instructions, the tool descriptions, the help resources and `kbase --help`. Where a text below is quoted, a compliant server serves that wording; every other item states content.

**Instructions.** `initialize` always has `instructions`, in each mode. They contain what spans the whole session or several tools, and stay short. They have eight items, in this order, and every tool they name is one the server serves:
1. **Where the project is.** With a default project: the KB's absolute path, or the repository's where `kb-root/` is yet to be built, saying the KB can be built. Without one: the project path is the absolute path of the project's git worktree root, the directory holding `.git`, where `kb-root/` is or will be; the session gets it from the user, by asking or by proposing one for the user to confirm, and uses it with `project-dir` on every call that session.
2. The `kbase` skill contains the workflow.
3. **The `.index/` rule.** `kb-root/.index/` has kbase's own records: never read or write anything under it. Testing that a path there exists (`test -d`, a stat) is allowed; listing, globbing or opening anything under it is reading it.
4. **The session rule.** Rigor and bearing values are judgements, and a session does not make them. Set a rigor or a bearing, its current value included, only to a value the user has given for that node or edge, in their own words or quoted in the task. When adding a node or an edge without one, leave the value out: kbase records it as pending. `score` judges the rigor of claims, analyses and experiments and the bearing of `supports`, `lifts` and `refutes` edges. A work's rigor and a `founds` bearing stay pending until the user gives one.
5. **Edge direction**: “An edge reads "source *relation* target", so "A supports B" means A's strength bears on B.”
6. **Commit**: "Pass `commit` on every write batch: it is the only way to make the batch undoable as one unit. It commits all of `kb-root/`."
7. **Sheets.** The claim-graph sheets are drawn for people. Offer them for display, as files in `kb-root/` or, where this server lists them, as `kbase://sheet/` resources. Never read one for information; the queries answer what a sheet shows.
8. **Keys.** A command kbase prints for handling a key, such as `show-config`'s `upgrade`, is run as printed. Never compose one from a key file's path, and never ask for a key or read a key file into the conversation.

Items 3 and 4 are the text the stamped `AGENTS.md` also contains ([§3](#stamped-documents)), byte for byte. Item 5 is the text `kbase --help` also contains, byte for byte.

**Help.** Each command has a one-line summary, one shared help body, an optional MCP note and an optional CLI note. The shared body is the same text on both interfaces: an argument has one name, which the command line writes `--name` (a positional `<name>`) and MCP takes bare, and a kbase call is written as the interface in use writes it, as a `kbase` command line or as the tool's call. A note states only what differs because the environment differs; no help text is written separately for each interface. Each text is rendered in three places:

| Rendering | Contents | Read |
|---|---|---|
| MCP tool description | the summary, the MCP note, and where the full help is | always (`tools/list`) |
| `kbase://help/<command>` | the shared body, the argument list and the MCP note | on demand |
| `kbase <command> --help` | the usage line, the shared body, the argument list, the subcommand list where the command has subcommands, the CLI note, and for a subcommand a line naming the global flags | on demand |

- The argument list is part of the help on both interfaces. It lists each argument's name, type and whether it is required or repeatable, with its description, which states the default where `--help` prints one. Each interface writes an argument as it takes it: the name as the body writes it; a repeatable argument as a list over MCP; a mapping as its CLI string at the command line; a shorthand only at the command line.
- The two help renderings differ only by their notes, that formatting, and the usage line (`kbase <command> [flags]`), which only the command line has. The descriptions are the same text.
- A query operation's argument list is its own, not the `query` tool's.
- A tool description and its schema have what every call of that tool needs. The help has the full rules, refusals and examples.
- A command tool's description, `write`'s included, is its summary, then its MCP note, then where its full help is, `kbase://help/<command>`. The `query` tool's description is one sentence, then where its operations' shared help is, `kbase://help/query`, and where one operation's is, `kbase://help/<operation>`.

**MCP notes.** The content each note must have:
- `write`: the batch contract. "A batch applies its entries in the order given, each checked against the records as the earlier entries left them; kbase never reorders a batch." Shape errors are all reported at once; then the batch stops at the first refusal and writes nothing. `dry-run` and `commit`. Where the shared help is (`kbase://help/write`) and where each op's help is (`kbase://help/<operation>`). `kbase://help/write` and the root `kbase --help` have the batch contract in full ([§8](#write-batches)).
- `build` and `score`: the call returns once the run has started, as a detached process that keeps running without the server; the result's `pid`, `state-dir` and `wait`; `status` follows the run and `cancel` stops it.
- `probe`: "Each call is a paid model call: run it only when the user directs it, one call per direction, never on your own initiative or to retry."
- a write op: its arguments are batch entries.
- any command whose MCP arguments differ from its CLI flags in a way the argument list's formatting does not show: the difference.

**CLI notes.** The content each note must have:
- `build` and `score`: they run in the foreground; SIGINT or SIGTERM (`kbase cancel`) stops them resumably.
- a write op: the batch arrives as YAML (JSON accepted) on stdin or in the file `--values` names; `--dry-run` and `--commit`.

**Obligations on help.**
- The root help and `mcp`'s help name the three CLI-only commands, `auth`, `wait` and `mcp`. The root help's overview names `score`, `wait`, `build-trail`, `init`, `export-latex` and `probe` among the commands.
- `write`'s help states that a committing batch commits the whole of `kb-root/`, and that a batch without `commit` has no safety net.
- `status` and `cancel` speak of any run, detached or in the foreground, naming `build` and `score` as its kinds.
- `remove-node`'s help says plainly that it removes every edge naming the node, and points to the dry run.
- `stats`' help points to `find`'s strength filters.
- `find`'s help says, in one line, that `min: 0` leaves out pending nodes.
- `update-format`'s help says to run it after upgrading kbase.
- The help of an op that takes a `snippet` recommends quoting a few distinctive words from the line, not the whole line.
- A help that tells a caller to copy an edge row from a result into `add-edge` or `edit-edge` gives this advice. A row's `manually_scored`, `replaced_relation`, and `trace`'s `depth`, `strength` and `contribution`, and any op key whose value is null, are read-only facts: drop them before copying the rest. Copy `bearing` only where the row is manually scored or the relation is `founds`: a bearing given marks the edge manually scored ([§10](#provenance-and-the-judged-hash)), so copying a model-judged one would pin it. Into `remove-edge` and `restore-demoted`, copy only `source`, `target` and `relation`.
- The help of an op that takes a `rationale` says that what would strengthen a node is a graph question, answered by `trace` with `sort: contribution` and `order: ascending`.
- No help names a command or resource that this SPEC does not define.

