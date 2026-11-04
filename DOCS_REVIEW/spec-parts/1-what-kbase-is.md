## 1. What kbase is

kbase is a Go program that builds, maintains and scores a knowledge base (KB) from LaTeX papers.
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
  the statement markers in the leaves, the format stamp in `.index/format.yaml`, and the summary
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
| `.index/format.yaml` | `kbload`'s stamp read — the version is the bytes the file holds — and every save, which rewrites it only where its bytes change |
| Summary pages, `kb-summary.md` ([§10](#documents-of-a-kb)) | `verify`'s freshness check (`register-freshness`: each page rendered in memory and compared with the bytes on disk) |
| `.index/nodes.yaml`, `.index/edges.yaml` | The write ops' idempotence: whole files rendered deterministically |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | The precheck of a write batch's rebuild of the derived views and of `update-format` (`check: agents-redirect`, [Stamped documents](#stamped-documents)) |
| The `` ``` math `` fence opener | The inventory's display-maths fence reader |
| The up-link line | `kb.UplinkMarker` and the readers that key on it |
| The claim-graph sheets' DOT, against the DOT goldens (`internal/sheet/testdata/`) | The sheet writer's write-only-where-bytes-change rule: under the same `dot`, a re-render leaves an unchanged sheet's bytes as they are |

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
- A summary page is generated as prose for people and sessions to read. kbase never reads one back,
  and `verify` reports a page whose bytes differ from the rendering (`register-freshness`).

A **maintained block** is a generated region between marker comments: the lines from
`<!-- vvv-kbase … kbase-vvv -->` to `<!-- ^^^-kbase … kbase-^^^ -->`. Each marker line is
recognised by its two ends alone, so the text between the ends may differ between releases.

- A document holding exactly one opening marker line and one closing marker line, the opening
  first, has its block rewritten: kbase rewrites the span between the two lines and nothing else,
  and the text outside them is the KB's own.
- A document holding any other arrangement of marker lines is left untouched and listed under
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

A **volume** is a top-level directory of `kb-root/` holding an `index.md`; in a built KB it is one
paper's directory.

- `kb-root/claim-graph.svg` is the full sheet of the whole KB, always drawn: every volume a cluster
  holding its attached nodes. The sheet's title, and the label of the cluster of nodes no volume
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

The overview diagram and the per-volume sheets exist only where two or more volumes hold nodes; the
nodes no volume hosts are not a volume, and a volume holding no node has no sheet and no
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
refused ([§5 Starting over](#starting-over)). The KB a build writes holds every title unique within
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
- A fresh build where `kb-root/` exists in any state, an empty or scaffold-only one and one holding
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
  remedy `kbase update-format`. Over a KB stamped newer than kbase: `check: kb-format`
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
wants them starts over ([§5 Starting over](#starting-over)) and is a fresh build without
`--no-inference`. A step that makes model calls in part is not dropped: its model calls are skipped
and the rest of the step still runs, as for `depends-attributed`.

**`--through <stage>`** bounds the run at the named stage, given by id or display name. A bounded
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
  that is neither content nor apparatus (ARCHITECTURE §5.6). A key pandoc's writer drops — an empty
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

**What a build produces**: Markdown leaves; the index, up-link and cross-reference structure;
load-bearing forms; the claim graph's node and edge records, among them an analysis for each claim
whose bounds hold text beyond its statement proper; the summary pages. The build's stages and what
each does are in ARCHITECTURE, "Stage table"; how each computes its product is ARCHITECTURE's.

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

A claim whose bounds hold text beyond its statement proper is split into the claim, which narrows
to its statement proper, and an analysis. An equation node is never split, and neither is a claim
whose bounds are its statement proper alone. The analysis's statement is the claim's own block or
paragraph where that holds text beyond the statement proper. Otherwise it is the first proof, in
reading order, that establishes the claim. Another proof establishing it is no part of this
analysis's statement; its text is a statement only where it is the first proof of some other claim
it establishes. A claim whose analysis's statement would be a blockquote holding no blank line is
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
not in `kb-root/` (ARCHITECTURE, "Records"). Each inference pass is one fixed-set decision per
model call under its embedded template (ARCHITECTURE, "Inference seams"), and the calls are made as
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

**The state store holds what does not belong in the repository** — the run record, which `status`
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
`--state-dir`). The store holds cached model answers and survives `git clean`.

**The run lock** is the file `kbase-operation.lock` in the repository's git directory, recording
the state dir, and the lock file in the state store beside the run record. A repository carries one
run at a time — a build or a scoring run, never both — whatever `--state-dir` each names. A clean
exit removes the lock files, cancel included, so a file in the store is never evidence of a live
run. Given no `--state-dir`, `status`, `cancel`, `wait` and `build-trail` read the store the run
lock names while its file exists and names one, else the default store; a new run never adopts it.
A second run is `refused` with `check: lock`, `path` the running run's state dir, and no remedy,
its `detail` naming the running run's verb and pid: "a build (pid 4321) is running; its state-dir
is …", or "a score run (pid 4321) is running; …". Where the run lock is held but names no state
dir yet, the second run gets `retry`; where it names one whose run record is not yet written, the
refusal says a run is starting. Every lock is advisory and exists on Unix only; on Windows `status`
cannot see a running run and no lock-based refusal or `retry` is given.

A maintenance operation over an existing KB that runs no git command — the queries, `verify`,
`export-latex` and a write batch without `commit` — works without a git repository. The commands
that run git are refused `check: worktree` outside a git worktree; they are listed at
[Write ops](#write-ops).

### The state store is versioned

`<state-dir>/format` holds one integer, currently `1`, written
when a build first takes the run lock; a store with no `format` file is version 1. `build`,
`status`, `wait`, `cancel`, the MCP `build` tool and `build-trail` refuse a store newer than this
kbase: `check: state-dir`, `key` the `state-dir` argument, `path` the store, remedy "update kbase,
or use another `state-dir`", the argument in both spelled as [§7](#item)'s `key` states. A build
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
([§5 Starting over](#starting-over)). Where `state` is `none` because `kb-root/` is absent or was
removed, no stage is done, and `current`, `pid`, `started`, `updated`, `ended` and `resume` are
null. `recent-refusals`, `recent-fallbacks`, `cache-entries` and `cache-bytes` still read the state
store: they record earlier invocations, not a current build.

A build's `resume` is null where no identical re-issue continues the work (§7) and where the
recorded command does not read as a `kbase build` invocation — at least two elements, `kbase` then
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
lock`, `path` the store's lock file, detail "no run holds the state store".

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
argument is `state-dir`; it takes no target, since every boundary and `started-at` carries its own
command. The checkpoint commits it reads are the newest build's ([§5 Starting over](#starting-over)).

Its result has these keys after `kb-root`, in this order:

- `state-dir`;
- `head`: the commit `HEAD` names, null on a branch with no commit;
- `boundaries`, oldest first, each with `stage`, `commit`, `subject` and `command`;
- `started-at`: `commit` and `command`, null where the checkpoint commits record none;
- `dirty`: `false`, or the worktree's changed paths, the first 20 named and the rest counted as
  `+N more`.

A `command` is a list of the command lines to run in order. A boundary's is `git reset --keep
<commit>` then the build resumed with the flags its last run was launched with, `--through`
excepted, which each surface spells as it takes it: `kbase build --resume` and those flags at the
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
`--version` are human-facing and outside it; `mcp`'s stdout carries JSON-RPC frames (§11), though a
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
| `key` | The setting the item is about: a values key, spelled as its configuration file spells it, or a flag or positional argument, spelled as the surface the call came through takes it: `--name` at the command line, `name` over MCP; a positional `<name>` at the command line, as its usage line spells it; an item refusing a value given for a named thing may carry that value as given, as the auth verbs carry the provider name |
| `line`, `column` | 1-based position in the values document or in `path`. A refusal over MCP that names a document (`path`) keeps its `line` and `column` there; one naming no `path` has neither |
| `allowed` | The fixed set the value must come from |
| `remedy` | The command that clears the item, where one exists (e.g. `kbase update-format`, `git init`); on a configuration or provider refusal it names `kbase show-config`, after the command that clears the item where one exists; a word in angle brackets is a placeholder the one running it fills in, as `<command-that-prints-the-key>` ([§9](#a-refused-key)); a `kbase` command exposed as an MCP tool — in a remedy, its prose and a result field such as `resume` alike — is, over MCP, that tool's call: the tool's name, then its arguments as compact JSON, keys sorted, carrying only the arguments that tool's schema has (a query op's `query {"operation":"<op>","arguments":{…}}`, `update-format {}`); at the command line it is `kbase <command>`, help included, its positionals, then each flag in name order, a true bool bare, a placeholder for a repeated positional `<name>...`; a command no tool exposes (`auth`, `wait`, `mcp`) is a command line on both surfaces. A command or argument that is a field's whole text is written bare; inside other text it is set off in backticks over MCP, and at the command line a command alone. An argument with its value reads `name: value` over MCP and `--name value` at the command line. The remedy of the refusal of an id no node has (`check: unknown-id`) reads "list the nodes that stand with `kbase find`, correct `<id>`, then re-run", `kbase find` written as the surface writes a command. A remedy naming a write batch reads at the command line as its values document piped in, `printf 'entry: []\n' | kbase write`, and over MCP as the `write` tool's call, its entries included |
| `detail` | One sentence; under `retry` and `failed` it ends with what the outcome asks of the caller: `re-issue the identical call later`, or `re-issuing the identical call will not fix it` |

A refusal lists every offending item. A usage error is one item with `check: usage`. A
provider-selection refusal's `key` is the `provider` argument, spelled as `key` states, and lists
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
| `export-dir` | `export-latex`'s output directory holds a `gen-tex/` that kbase did not write |
| `include` | an `\input` or `\include` the build cannot load, or that resolves outside the top `.tex` file's directory ([§5](#inference-preflight)) |
| `index` | a record file missing from a KB, or a write while a conversion is pending ([§10](#one-version-read-and-written)) |
| `inference` | the build's overview call did not yield a usable passage (outcome `failed`) |
| `kb-format` | a KB stamped newer than kbase |
| `kb-root` | `kb-root/` exists where `init` or a fresh `build` would create it; a `--resume` with nothing to resume while `kb-root/` exists; a `kb-root/` with neither stamp file nor entry point |
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
| `worktree` | outside a git worktree, or a `project-dir` that is not a worktree root, or a repository root whose `.git` git does not read as the worktree root |
| `write` | a failure inside a write batch (outcome `failed`) |
| a stage id | a build stage's own checks found its products disagreeing (outcome `failed`); `path` the stage's report |

**Per-subcommand keys**, following `kb-root`:

| Subcommand | Keys |
|---|---|
| `build` | `state-dir`; `roles` (one record per role the build resolved, in role order — `decision`, then `text` — in the role record's shape below; absent where it resolved none, as under `--no-inference`); `through` (absent where this invocation ran no stage); `no-inference`; `resumed` (bool); `restored` (paths reset to the last boundary); `stages` (list of `stage`, `commit` — null if unrecorded — `dropped` (step ids), `report` (path to the stage's full findings in the state store); absent where this invocation ran no stage); `resume` (the call that continues the build, spelled as `remedy` spells a command: at the command line `kbase build` with `--resume` and the flags the invocation was given that a resume still needs, all in name order — `--no-inference` where given, `--ask-concurrency` where given, `--state-dir`, `--config-dir` and `--project-dir` where given, `--config-dir` written absolute; over MCP the `build` tool's call with `resume: true` and the same arguments but `config-dir`, which no tool takes — never the inputs or `--through`; absent where an identical re-issue continues nothing: `done`, `unchanged`, `bounded`, `retry`, a `failed` or `cancelled` run that ran no stage, and a `refused` run that recorded no boundary) |
| `status` | `state-dir`; `state` (the build `state` vocabulary of [§6](#kbase-status), `interrupted` included); `pid`; `started`; `updated`; `ended`; `no-inference`; `stages` (a mapping: `done`, `not-done`, `rewind`); `current` (`stage`, `units-done`, `units-total`); `recent-refusals`, `recent-fallbacks` (items, the newest 10 of each); `cache-entries`, `cache-bytes` (integers: the answer cache's entry count and total size in bytes, §6); `resume` (null for `finished`, `running` and `bounded`; read from the run §6 names, spelled as `build`'s for the surface reading it; `recent-refusals` items are spelled so too, an item an earlier kbase recorded as written) |
| `cancel` | `state-dir`; `pid`; `resume` (spelled as `build`'s; absent where the record's command does not read as a `kbase build` invocation, §6) |
| `build-trail` | `state-dir`; `head`; `boundaries` (oldest first: `stage`, `commit`, `subject`, `command`); `started-at` (`commit`, `command`; null where none is recorded); `dirty` (`false`, or the changed paths, the first 20 named and the rest counted) ([§6](#kbase-build-trail)) |
| `init` | `written` (every path this call wrote — the scaffold, the two empty record files, the stamped documents and the output of the rebuild of the derived views, merged, deduplicated, path-sorted); `removed` (as `update-format`'s; `[]` on create) |
| `update-format` | `written` (includes every summary page, sheet and maintained-block document written); `removed` (files the KB no longer calls for — a summary page no node is rendered on, a sheet the volume-count rule drops, a file a format migration made obsolete (§10) — kb-root-relative inside `kb-root/`, repository-relative beside it; `[]` when none); `unfenced`; `sheet` (`drawn` where the sheets were rendered this call, else `not-drawn`: nothing was drawn, whether or not a sheet exists); `dot` (`"not found on PATH"`, present only when `sheet` is `not-drawn`); `commits` (as a write batch's); `uncommitted` (as a write batch's); and, where the format changed, `ids` (`from`, `to`: each work's and axiom's `3.0.0` id and the id minted in its place), `bare-ids` (`path`, `line`, `id`), `no-edge` and `strengthen-by` (`id`, `text`). `unfenced` onward are present only on `done` and `unchanged` |
| `verify` | `findings` (items the KB's structure has that are not faults — every `demoted` record (§10), as an item with `check: demoted`, `path` the summary page of the record's `target`, omitted where that target has no host, and `detail` the edge and the relation it replaced; `[]` when none); its faults are `refusals` items, each with the remedy [§8](#verify) names for it where one op clears it |
| `write` and every write op | `dry-run`; `entries` (one per batch entry, in order: `entry`, `operation`, `outcome` — `done` or `unchanged` — and on an add's entry ([§8](#add-kind)) `id` (the node it minted) alone but on an `add-axiom` entry, which adds `heading` (`marked` or `appended`), on an edit's entry ([§8](#edit-kind)) `id` (the node it edited) alone but on an `edit-work` entry that changes `label`, which adds `rewritten` (each line it rewrote a reference on, `path` and `line`, ordered by path, then line; `[]` where none), on a `move-node` entry ([§8](#move-node)) `id` (the node it moved) alone, on a `remove-node` entry ([§8](#remove-node)) `id` (the node it removed) and `removed-edges` (each edge record it removed, as its `edges` row), on an `add-citation` entry ([§8](#add-citation)) `document` and `line` (where it wrote the citation), `id` (the node cited) and `link` (the text written), on an edge write's and a `restore-demoted` entry ([§8](#edge-ops), [§8](#restore-demoted)) `source`, `target` and `relation`, the edge it names, and on a `restore-demoted` entry then `restored` (the relations it put back); `[]` where the batch did not apply); `written` (kb-root-relative paths the batch wrote, or under `dry-run` would write: records, leaves and derived views); `removed` (the rebuild of the derived views', as `update-format`'s); `sheet` and `dot` as `update-format`'s, where the rebuild ran; `commits` (each commit made: `commit`, `label`, `paths`; `[]` without `commit`); `uncommitted` (a detail line, present only where the batch commit failed) |
| `citations-of` | `count`; `results` (every citation of the node, complete, unsliced: `path`, `line`, `text` ([§8](#citations-of))) |
| `find` | `count`; `results` (every node the filters keep, complete, in the sort's order, cut only by `limit`: `id`, `node_type`, `title`, `label`, `host`, `strength`, `dependents` ([§8](#find))) |
| `edges` | `count`; `results` (every edge record the arguments keep, complete, in its fixed order: `source`, `target`, `relation`, `bearing` (as the index writes it: a number in `[0, 1]` or `*pending*` written as text, null only where the pairing has none), `context` (null where the edge has none), `manually_scored` (the record's flag ([§10](#provenance-and-the-judged-hash)), null where its record has none) and `replaced_relation` ([§8](#edges))) |
| `trace` | `count`; `results` (one row per edge the traversal reaches, complete, in the sort's order, cut only by `limit`: an `edges` record, then `depth`, `strength` and `contribution` ([§8](#trace))) |
| `show`, `stats` | `results` (the query's mapping; a node record has `node_type`, `id`, `title`, `host`, `host_anchor` and `summary` ([§8](#show)), then the keys its kind has: `rigor` and `strength` on a claim, with `build_status`, `strength_band`, `rationale` and `premise_count` after them; `rigor`, `strength`, `strength_band` and `rationale` on an analysis; `status` and `rigor` on an experiment; `label`, `rigor` and `rationale` on a work; `label` and `rigor` on an axiom, its `rigor` `1.0` by rule; `show` adds `manually_scored` ([§10](#provenance-and-the-judged-hash)) after `rigor` on a claim, an analysis and an experiment; `strength` is a number rounded to two decimals for reading, `rigor` the number the node holds; `stats`' mapping has one count per node kind — `claims`, `analyses`, `experiments`, `axioms`, `works` — then `edges`, `demoted_edges` and `subtree_aggregates` (per volume, `claims` and `experiments` ([§8](#stats))), then `rigor_manual`, `rigor_model`, `bearing_manual` and `bearing_model` (the records whose type has `manually-scored`, `*pending*` values included, counted by the flag), then `relations`, then `strength_bands` and its two ranked sections, whose records have `strength`; `relations` has one record per relation in this order — `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted` — each `edges`, the relation's row count, `walked` (bool), then on a scoring relation `bearings`: one record per distinct bearing its rows have, `bearing`, as the index writes it, and `edges`; numbers ascending, then `*pending*`, then null, `[]` where the relation has no rows) |
| `models` | `provider`; `provider-rule`, `provider-layer` (the rule that named the provider — `flag` or `sole-provider` — and the layer that supplied it; this verb always selects one); `models` (ids, sorted ascending); `warnings` (items: `path` the file or directory the configuration fault is against, `detail`; `[]` when none; present whatever the outcome) |
| `show-config` | `layers` (one record per directory this invocation read, in precedence order: `path`, `source` (`explicit`, `environment`, `repository` or `global`), `exists`, `providers-exists`, `config-exists` — the directory's presence and each of its two files'); `providers` (one record per entry, in name order: `name`, `base-url`, `decision-api` (the decision API it speaks, `systemone` where the entry is silent), `key` (`store`, `file` or `none`), `key-file`, `upgrade` (for a `file` key, `cat <key-path> | kbase auth add <provider>`, the resolved path single-quoted where a shell would split or expand it), `layer` (the layer that supplied the entry), `usable`, `fault`; a provider a dropping fault removed has `name`, `layer`, `usable`, `fault` only); `config` (the merged choices: `models` (`decision`, `text`, `scoring` — each the reference `config.toml` states, null where unset); `sampling` (`text`, `scoring` — each `temperature` and `thinking` as `config.toml` states them, null where unset)); `selection` (`provider` and `rule` — `flag` or `sole-provider` — or `detail` where no provider was named); `roles` (one record per role in role order, in the role record's shape below, each with `base-url`, `sources` (`base-url`, `model`, `key`, and for a chat role `temperature`, `thinking` — the layer that supplied that field, one of `layers`' four names, `store` for a key the store supplied, null where the field is unset), a chat role's `temperature` and `thinking` (null where unset: kbase's default, §9.2) and `detail` where the role does not resolve); `diagnostics` (items: every fault, `path` the `providers.toml` that has it, the stored key file that does not open, with `remedy` the refused-key remedy ([§9](#a-refused-key)) where it is truncated or does not decrypt, or the store's directory where it exists and does not list; an entry dropped because the key file `providers.toml` names cannot be read has the same remedy (§9.4); a warning per entry whose stored key leaves its `apiKeyFile` unread, `path` the `providers.toml` stating it; and the resolution failure where a role does not resolve) |
| `configure-providers` | `provider` (lowercase); `layer` (`global`, `repository`, `explicit` or `environment`); `written` (`[]` with outcome `unchanged`); `models` (ids, sorted ascending, where it sent a request to the provider); `next` (a list of strings, where no key is stored or that request failed: [§9](#provider-commands), "Provider commands"); `warnings` (items: `path` the file or directory the configuration fault is against, `detail`; `[]` when none) |
| `configure-models` | `models` (`decision`, `text`, `scoring`, as the written `config.toml`'s `[models]` holds them, null for a role it does not set); `written` (`[]` with outcome `unchanged`); `warnings` (items: a configuration fault has `path` and `detail`; a model missing from its provider's catalogue has `key` the role and `detail`; `[]` when none) |
| `probe` | `provider`; `provider-rule`, `provider-layer` (as `models`'s); `selection` (the selection that answered: `decision`, `text` or `scoring` where a role flag or the default named it, else the `<provider>/<model>` reference `--model` named); `model` (the id asked); `temperature`, `thinking` (chat selections only: the effective settings — `temperature` a number; `thinking` `off`, `low`, `medium` or `high`); `first-token-ms`, `wall-ms` (integers: request start to the first non-empty content or reasoning delta — zero where none arrived — and the call's end-to-end time); `finish-reason` (null where the stream reported none); `reply` (the reply's text on a chat endpoint, and on a decision endpoint the answer's probability, chosen letter or type); `usage` (a record: `reported` — whether the provider sent a usage block at all, so a zero count is a measurement only where it is true — then `prompt-tokens`, `completion-tokens`, `reasoning-tokens`, `cached-tokens`) |
| `auth add`, `auth remove` | `provider` (the name given, folded to lowercase; never the key) |
| `auth list` | `providers` (the names with a stored key, in name order; `[]` when none) |
| `export-latex` | `written` (absolute paths; outside `kb-root/`); `volumes` (the volume directories exported, entry-point order); `report` (the path of the export report written under the out dir — one YAML list, every simplification and finding line the run produced, each naming the form, the source path and the reason) |
| `score` | `migration` (conditional, the shared vocabulary's; it comes before `state-dir`, on the refusal of a run over an older KB, which `update-format` converts first); `state-dir`; `pass` (the set it runs: one pass, or `all`; a named node's `rigor`, a named edge's `bearing`); `roles` (one record, `scoring`, in the role record's shape below; absent under `--dry-run`); `run` (the run's id); `ledger` (the run's directory, absolute); `counts` (a record: `total`, `done`, `failed`, `skipped`, `remaining` — the items by the status the ledger last records for each, so a resumed run reports the whole run); `ask` and `skip` (a dry run's alone: the items a run would ask and the stale manually scored items it would skip, named as `scored` names them, in the item loop's order ([§8](#the-closing-record))); `scored` (the items whose value was written, in the item loop's order — a node's id, or an edge's `<relation> <source> → <target>`); `resume` (the call that continues the run — `score` with `--resume`, spelled as `build`'s, over MCP the `score` tool's call — keeping `--state-dir`, `--project-dir` where given, `--scoring-model` where given, and `--config-dir` written absolute, and no argument of the scope; present wherever an item failed or was not reached — a `skipped` item does not make it present — and it comes before the record that ends the document; absent from a dry run, and from a stop before the item loop) |

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
| `model` | the model id this invocation asked for; null where no source named one |
| `base-url`, `sources` | `show-config` alone: the endpoint, and the layer that supplied each field |
| `temperature`, `thinking` | `show-config` alone, chat roles: the role's sampling settings, null where unset (§9.2) |
| `detail` | `show-config` alone: why the role does not resolve, where it does not |

`provider`, `provider-rule` and `provider-layer` appear together on every role that resolved: a pool
entry supplies every endpoint. `provider-layer`'s `environment` is the directory `$KBASE_CONFIG_DIR`
names (§9).

**The model is the one requested, never one observed.** `model` is the id this invocation asked for,
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

`stats`' ranked sections stay at 10, and the `bearings` of its `relations` hold
one record per distinct bearing value. No other subcommand's result grows with the KB; `build`'s
per-stage detail goes to the `report` file, not stdout.

**Exit codes** derive from `outcome`:

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success, including a `--through`-bounded run |
| `refused` (including verify faults and usage errors) | 1 | Wrong input or KB state; nothing written past the last checkpoint |
| `retry` | 2 | Another writer holds the KB write lock past the wait (`check: lock`, `path` the repository root), or a run holds the run lock but has not yet recorded its state dir (§6); re-issue identically later. A run that holds the run lock and has recorded its state dir is not `retry`: it is `refused` with the `lock` item (§6) |
| `failed` | 3 | Defect in the tool, its input or the environment; re-issuing will not fix it |
| `cancelled` | 4 | Stopped on request; resumable |
