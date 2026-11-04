# SPEC – kbase

Consumer-facing behaviors and outcomes

Must-see:

- Any implementation that satisfies this SPEC is valid
- No project mechanisms or code structure are put forth, that belongs to ARCHITECTURE
- No project-specific process is dictated, that belongs to CONVENTIONS

## 1. What kbase is

kbase is a Go binary that builds and maintains a knowledge base (KB) from LaTeX. One or more volume
roots go in, one KB comes out — its entry point listing every volume — and a set of maintenance
subcommands then operates on the living KB, at a shell or, through `kbase mcp` (§11), from any
external agent harness that speaks MCP. A KB can also be created empty by `init` (§8) and filled by
authoring with those same subcommands — no volume, no LaTeX, no build: LaTeX stays the only input
format of `build`, and `init` is the zero case of the tree, not another input.

- A **volume root** is the top `.tex` file of a paper: the file `00README.json`'s `toplevel` entry
  names, else the sole file containing `\documentclass`.
- A **KB** is a `kb-root/` directory beside a repository's `.git`: the Markdown document tree and
  its metadata layer.
- The **metadata layer** is what kbase writes into a KB: the node and edge records in `.index/`,
  the statement markers in the leaves, the format stamp in `.index/format.yaml`, and the register
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
the `pandoc-api-version` are enforced at preflight; a missing pandoc is refused with an error naming
pandoc's install page. The build's reader runs pandoc LaTeX to JSON with no filters, and with
`--citeproc` exactly when a bibliography is offered; `export-latex` (§8) is the one run with a
filter.

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
| Statement markers, `<!-- kbase-id: <id> -->` attached with no space ([page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)) | The marker reader; The add ops' and `move-node`'s locate and splice |
| `.index/format.yaml` | `kbload`'s stamp read — the version is the bytes the file holds — and every save, which rewrites it only where its bytes change |
| Register pages, `kb-summary.md` ([page §1.6](NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)) | `verify`'s freshness check (`register-freshness`: each page rendered in memory and compared with the bytes on disk) |
| `.index/nodes.yaml`, `.index/edges.yaml` | The write ops' idempotence: whole files rendered deterministically |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | The precheck of a write batch's derivation and of `update-format` (`check: agents-redirect`, [page §1.7](NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)) |
| The `` ``` math `` fence opener | The inventory's display-maths fence reader |
| The up-link line | `kb.UplinkMarker` and the readers that key on it |
| The claim-graph sheets' DOT, against the DOT goldens (`internal/sheet/testdata/`) | The sheet writer's write-only-where-bytes-change rule: under the same `dot`, a re-render leaves an unchanged sheet's bytes as they are |

### Stamped documents

`README.md` gives the maintenance commands: kbase's subcommands. The stamped documents point a reader at kbase's tools, never at its records:
`AGENTS.md` forbids reading or writing anything under `.index/`, save testing that such a path
exists, and states the rule that a session sets a rigor or a bearing only to a value the user gave,
both as the MCP server's instructions do, in the same words ([page §4.1](NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md) items 3 and 4, §11). A register page is the one exception: kbase generates it as prose for people and
sessions to read, and nobody takes a value from it or edits it. `CONVENTIONS.md` states that the
records and the statement markers are written only by a write op, never by hand, and that no agent
edits the block kbase maintains in a stamped document. `AGENTS.md`, `CONVENTIONS.md` and
`README.md` are maintained-block documents ([page §1.7](NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)): `update-format` rewrites each one's block from the
current templates and leaves the text outside it (§8).
### Formats

A new KB is written in the current metadata format, `4.0.0`
([page §1.8](NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md), §10). A KB in a superseded form,
`3.0.0`, `2.0.x` or `1.0.0` (§10), is read through migration.

**Theorem numbering** is whatever pandoc prints.

### `claim-graph.svg`

A picture of the claim graph, drawn through Graphviz `dot` (§2). `update-format`, `init` and a write batch's
derivation render every sheet on each run and write one only where its bytes change. The SVG's bytes are
Graphviz's; no check reads them for their content — the sheet writer's test writes one DOT twice through
the same `dot` and asserts identical bytes, the write-only-where-bytes-change rule's basis — and no
other command's behaviour depends on them (ARCHITECTURE §6).

#### Three kinds of sheet

- `kb-root/claim-graph.svg` is the full sheet of the whole KB, always drawn: every volume a cluster
  holding its attached nodes. The sheet's title, and the label of the cluster of nodes no volume
  hosts, is the entry point's first heading, verbatim; `kb-root` where it has none.
  The KB title is the graph's label across the top. A volume's cluster is titled from its `index.md`
  first heading, falling back to the directory name, with the subtitle "N nodes · M unattached".
- `kb-root/claim-graph-digest.svg` is the volume digest: one box per volume listing its node counts
  per kind on one line, its unattached count and its within-volume edge counts, and the cross-volume
  edges as bundles whose counts cover every edge class; each box links to the volume's index and, on
  a second line reading `claim graph →`, to its sheet. The full sheet links to the digest (`volume
  digest →`) and the digest to the full sheet (`← full claim graph`).
- `kb-root/<volume>/claim-graph.svg`, one per volume, a top-level directory holding an `index.md`:
  the volume's claims plus every claim one edge away, drawing only edges with an end in the volume.

The digest and the per-volume sheets exist only where two or more volumes hold nodes; the nodes no
volume hosts are not a volume, and a volume holding no node has no sheet and no digest
box. A one-volume KB has the full sheet alone. A digest or volume sheet the KB no longer calls for
is removed by the next render, and reported (§7); a file named `claim-graph.svg` directly inside a
top-level directory of `kb-root/` is by definition such a sheet.

#### What is drawn

The four scoring relations — `supports`, `founds`, `lifts` and `refutes` — each in
a stroke of its own, every stroke pointing in the push direction, from the supporter to the node it
supports. `references` edges are not drawn; a `demoted` edge (§10) is drawn as a cut. A node with
no drawn edge is not drawn as a node; it is listed in a table titled "unattached (N)", one row per
node with a link and a tooltip. Of the drawn `supports` edges, only the transitive reduction
appears; it checks an edge only against the edges still kept.

- *Edges*: each relation is drawn in one style of its own — `supports` black solid, and `founds`,
  `lifts` and `refutes` each a stroke beside it — whatever the build knew of how an edge was found:
  the sheet reads no build record. A **cut**, a `demoted` record, is drawn dotted red in the
  direction of the edge it replaced; its tooltip calls it `cut`, the digest counts cuts beside the
  other classes (`1 cut`) and draws a bundle of them, and its legend row reads "cut from supports:
  part of a circle".
- *Nodes by kind*, seven, each with its own mark: a labelled block (bold frame), an equation, prose,
  an analysis (dashed rounded), an experiment (component shape), an axiom (grey, bold frame), a work
  (note). An edge end that no record carries is drawn as a ghost box, dashed red, labelled with the
  id. Claims and analyses are filled by strength band, `*pending*` in grey. A node's tooltip reads
  `<id> [<kind>, <band>] <title>`, the band omitted for a node that has none, every backslash in it
  shown as written (`$\Gamma$` does not become the graph's name).
- *Links*: every node links to its canonical place (§10) — a claim's or a work's entry on its
  register page, an analysis's or an experiment's section in the leaf hosting it, an axiom's heading
  in `axioms.md` — and a claim with no host links nowhere; a digest box links to the volume's index
  and sheet.
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

A value of `--ask-concurrency` under 1 is a usage refusal naming `--ask-concurrency`, and `build`'s
`resume` carries it.

### Output and placement

The KB is written only at `kb-root/` in the git worktree root. The build records live in `kb-root/.build/`
([page §2.5](NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md)), so every path the build commits is under `kb-root/`. The state store (§6) is
never written into `kb-root/`.

**Fresh versus resume** is derived from the `kb-build:` commit trail (§6), never configured.
`kbase build <volume-root>...` starts a fresh build where no `kb-root/` stands; `kbase build
--resume` continues the trail. A fresh build where `kb-root/` exists, in any state, is refused
([§5 Starting over](#starting-over)). The KB a build writes holds every title unique within its kind ([page §3.5
`build`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)), and its writes go through the write ops, each followed by the derivation ([page §2.3](NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)):
the derived files are current at every stage commit from the spine seed on. A link pandoc wraps
around a cross-reference, its destination the bare LaTeX label, is unwrapped to the
cross-reference inside it, which links the target document's heading.
`build` refuses outside a git worktree, naming `git init`. It refuses if the kbase-owned paths it
commits are dirty and the dirt is not an interrupted stage's own uncommitted work (§6), and only
then.

### Inputs

A relative volume root or `--bibliography` resolves against the project root. A build
orders its volume roots, and its `--bibliography` files where given, by full path as text, whatever
order they were given in; every stage reads them in that order and the trail records it (§6).
`--resume` takes no inputs: it continues with the volume roots and `--bibliography` files the
trail's newest boundary records, in the order recorded. Before it resolves a state store, `build`
refuses, `check: usage`, a `--resume` given a volume root (`key` `<volume-root>`) or a
`--bibliography` (`key` `--bibliography`), one item per argument kind, and a build given neither
volume roots nor `--resume` (`key` `<volume-root>`).

### Starting over

`build`'s arguments, results and refusals, the fresh build over an existing `kb-root/` and a
`resume` with nothing to resume among them, are [page §3.5 `build`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)'s; a fresh build over any `kb-root/`
is refused with `init`'s refusal ([page §3.7 `init`](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/init.md)), and the way to start over is to remove
`kb-root/`. `build-trail` ([page §3.5 `build-trail`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md)) lists the commands that rewind: a boundary's is
`git reset --keep <commit>` then the build resumed with the flags its last run was launched with,
the bound excepted, and `started-at`'s the reset alone; kbase runs none of them.

What the page leaves to this document:
- A fresh build over a trail a removed `kb-root/` left behind starts a trail of its own: a resume
  continues the newest build's trail, read back to its `start` boundary, and `status` and
  `build-trail` read that trail alone.
- `status`, `build-trail` and a resume read no trail a later removal of `kb-root/` ends: the trail
  they read lies above the newest commit that removed `kb-root/` and is no `kb-build:` boundary.
  With `kb-root/` absent, or the trail so ended, and no build run holding the state store,
  `status` reports `none` with no stage done and no `current`, and `build-trail` no boundary and
  `started-at` null; a live run is reported as it stands. A build that ends with `kb-root/` absent
  hands out no `resume` in its result. A reset to before the removal restores `kb-root/` and the
  old trail with it.
- A `resume` with nothing to resume — an empty trail, one whose newest boundary records no
  inputs, or a `kb-root/` that does not exist — is one item, `key` `--resume`, `path` the
  repository root: while `kb-root/` stands, `check: kb-root`, remedy "remove kb-root/, then
  run a fresh build, to start over"; otherwise `check: usage`, `detail` "kb-root/ does not
  exist: there is no build to resume", remedy `kbase build <volume-root>...`, whatever the trail
  records.
- A fresh build where `kb-root/` is absent counts no deleted path under it as dirt, committed or
  not, and its first boundary commit records the removal; any other dirty path it owns is refused
  (`check: dirty-paths`), remedy "commit the change or restore the path to the last commit, then
  re-run".
- A trail that records a stage while leaving an earlier stage in walk order unrecorded was written
  by a build walking another stage order, and a build over it is refused before anything is
  restored or written: one item, `check: trail`, `path` the repository root, `detail` naming the
  recorded stage and the unrecorded one before it, remedy "start over from a commit before the
  trail". `build-trail` refuses it the same way.
- `kbase build --resume` over a finished trail is `unchanged`.
### Determinism

The same inputs and pandoc version give a byte-identical document graph: the tree
`document-graph` writes and the records. The claim-graph stages mint fresh node ids on every build —
unique within the KB, now and going forward — so a build is reproducible modulo
ids from `claims-declared` on, and no consumer may read anything into an id's value. The order in
which volume roots and `--bibliography` files are given does not affect the result: the same set
gives the same document graph, and the same KB modulo ids.

**`--no-inference`** is first-class. It drops the steps that spend inference, still walks and records
every stage, and states the steps it dropped. The run is a real KB built without those steps, not a
stopped walk. A trail a `--no-inference` build finished is a finished trail: `kbase build --resume`
over it is `unchanged`, and its dropped steps never run on it. A build that wants them starts over
([§5 Starting over](#starting-over)) and is a fresh build without `--no-inference`. Which steps drop follows the classification on the step (ARCHITECTURE §4), never a stage
id. A step that spends inference in part is not dropped: its asks are skipped and the rest of the step
still runs, as for `depends-attributed`.

**`--through <stage>`** bounds the run at the named stage. A bounded run is not a failed one:
outcome `bounded`, exit 0 (§7).

### Inference preflight

A build with a stage left to walk that calls a model refuses (§7,
`refused`) before the first stage it walks when no provider is configured (§9), naming the
configuration it lacks; its `remedy` names `kbase show-config` as the diagnostic that shows how the
configuration resolved (§9.6). A build spending no inference needs none. Every model call a build
makes is one tool-less chat request answered from its prompt alone — except the claim-graph multichoice
asks, which are each one `POST` to the decision surface the selected entry names (§9.1) carrying a
closed-set question (a `choice` or `noul` question over the offered letters) and answered by
probabilities over that set, not by reply text (§9).

**Refusals** (outcome `refused`):

- Unparseable source: names the paper and the reader error.
- Unloadable include: names each file and the line that named it.
- Include outside the volume: `\input` and `\include` resolve only inside the volume root's
  directory, the one pandoc runs in; one that resolves outside it — by an absolute path, by `..`,
  through a symlink, or below a first-level include — is refused, naming the include and the file
  that named it. A target that resolves inside and names no file stays the reader's own warning.
- An unclassified metadata key: a key the rendering's metadata block carries that is neither content
  nor apparatus (ARCHITECTURE §5.6). A key pandoc's writer drops — an empty entry — reaches no block
  and is not refused.

### Bibliographies

With no `--bibliography`, the build passes every `.bib` in any volume root's
directory, in sorted path order, one set offered to every volume; `--bibliography` given replaces
that set, read in path order (above). **Degradation, not refusal.** An unreadable bibliography: the build continues without it
and reports the file.

**Under `--no-inference`** no `README.md` is written: the overview call is the dropped row, and the
README is assembled around its passage.

### Monitoring

`build` runs in the foreground; an external agent harness backgrounds it. `kbase
status` and `kbase cancel` (§6) observe and stop it.

**What a build produces**: Markdown leaves; the index, up-link and cross-reference structure;
load-bearing forms; the claim graph's node and edge records, among them an analysis for each claim
whose bounds hold text beyond its statement proper; the register pages. The build's stages and what
each does are in ARCHITECTURE, "Stage table"; how each computes its product is ARCHITECTURE's.

### A build marks every claim

Every claim a build mints is hosted in the call that mints it: its
statement marker (§10) stands in its leaf, a leaf's only claim and an equation included — an
equation's at the end of the line inside its display-maths fence that carries its `\label{…}`,
never on the fence's closing line; a block claim's on its locator's line. The analysis split
(below) moves a prose claim's marker to just after the sentence its title is drawn from, on the line
that sentence ends on. It hosts each analysis it mints by a marker on the first blank line of the
blockquote that is its statement, or at the end of the last line of its claim's paragraph (§10).
Where either line closes a fenced block, the marker goes at the end of the line before it.

### A build records no judgement

Every scoring edge a build writes carries `*pending*` as its
bearing, and no stage writes a `rigor` or any other number into a score; every value a build writes
is `manually-scored: false` ([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)). The judgement is a person's, through `edit-<kind>` with `rigor` and `edit-edge` with `bearing`,
or a scoring run's (§8). A claim a build attaches an edge to reads unchanged until that edge is
judged.

### Co-linearity cull (`claims-culled`)

The stage walks between `references-found` and
`depends-attributed`, and removes prose claims that state no step a linked claim does not. Its
candidates are the links — every marked pair, every pair the unmarked-reference ask answered yes in
either direction, and every adjacency link, each unordered — with at least one end a prose claim.
Each candidate gets one co-linearity ask, shown both claims in reading order and offered three
answers: the second adds nothing to the first, the first adds nothing to the second, or each is a
step of its own. An answer naming a claim as adding nothing — answered, or
answered after a re-ask — culls it into the other, except that a block claim or an equation node is
never culled: where the answer names one, the prose claim of the pair is culled into it instead.
One whose ask yields no readable answer after the re-ask is recorded `defaulted` and both claims
keep. A claim several answers cull goes into the earliest of their partners in reading order;
following those to a claim culled into nothing gives its root. A claim whose root is not the claim
it was culled into is asked once more against its root and kept unless that answer names it as
adding nothing; no claim is asked a third time. A culled claim is removed — its outgoing and
inbound edges, then its node record and its statement marker — and no edge is added. The stage asks no
direction: every inferred link that survives, one with a culled end re-pointed onto that end's root,
goes undirected to the cull record (§6) for dependency attribution. Under `--no-inference` the stage
is still walked with its row dropped: no ask is made and the cull record stands as the declared
pass wrote it.

### Dependency attribution (`depends-attributed`)

A captured link carries no direction; only this
stage's ask gives it one, or rules it a mention. The candidates are every marked pair, directed as
the text marks it, and every link of the cull record (§6) whose two claims no marked pair joins
either way round, directed from its later claim in reading order to its earlier, an equation node
always the target. Each candidate is asked once, offered *supported by*, *in support of* and
*mention*, less *in support of* where containment directed the pair or its target is an equation
node. *Supported by* writes a `supports` edge from source to target, *in support of* one from target
to source, and *mention* a `references` edge. A candidate whose ask yields no readable answer after
the re-ask, and every candidate under `--no-inference`, takes its draft: *supported by* where
containment directed the pair, *mention* otherwise. The stage writes every edge its answers make,
cycles included; last, the cycle walk (§10) demotes one edge of each cycle those edges close, each
cut a warning in the stage's report, before the stage's refresh and verify.

### Analysis split (`analyses-split`)

The stage walks between `depends-attributed` and
`phase-3a`. It asks no model, so it runs in full under `--no-inference`.

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
`supports` and `founds` edge into the claim moves onto the analysis. So does every `demoted` record
into the claim that replaced a `supports` edge, each with its source, context, bearing and
`manually-scored` unchanged. One `lifts` edge runs from the analysis to the claim, bearing
`*pending*`, `manually-scored: false`. Every path through a split claim before the stage runs
through its analysis after it, so the stage closes no cycle and cuts nothing.

### Records and inference

The reader and placement facts the claim graph needs travel as records in
the build state store, not in `kb-root/` (ARCHITECTURE, "Records"). Each inference pass is one
closed-set decision per call under its embedded template (ARCHITECTURE, "Inference seams"); no stage
exits on a model's opinion. The multichoice asks are each one `POST` to the decision surface the selected
entry names (§9.1) carrying a closed-set question (`choice` or `noul`) and answered by probabilities
over that set — no reply text is parsed; the overview call remains one tool-less chat request
answered from its prompt alone.

---

## 6. Build state, ledger and monitoring

### Git holds the recoverable record

Each stage boundary is a commit in the user's repository, with
subject `kb-build: <stage-id> | <display name>` and a structured body. Every boundary's body ends
with the build's inputs: a `volume-root: <path>` line per volume root, then a `bibliography: <path>`
line per `--bibliography`, each in the order the build reads them (§5), each path absolute. The
`start` boundary's body also records `started-at: <commit>`, the commit the build began from: the
clean, ready-to-build state a fresh build over other inputs starts again from. On a branch with no
commit, a fresh build first makes one empty commit, subject `kb-start: the repository before the
build` — outside `kb-build:`, so no part of the trail — after every refusal it can give and before
its `start` stage, and records that commit as `started-at`. A commit carries `kb-root/`, the tracked build records in its `.build/` included ([page §2.5](NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md)):
the node-pass record `kb-build-node-pass.yaml`, the unmarked record
`kb-build-unmarked.yaml`, the classification record `kb-build-classification.yaml` and the cull
record `kb-build-cull.yaml`, YAML at `1.0.0` (§10). The cull record holds `about`; `entries`, one
per co-linearity ask, sorted by `pass`, `a`, `b`, each with `a` and `b` (the two claim ids, `a`
before `b` in id order), `pass` (1 or 2), `verdict` (`a-adds-nothing`, `b-adds-nothing` or
`distinct`) and `outcome` (`answered`, `re-asked` or `defaulted`); `culls`, sorted by `id`, each
with `id` (a culled claim) and `into` (the claim it was culled into); and `links`, sorted by `a`,
`b`, every inferred link that survived the cull, each with `a` and `b` (id-sorted, carrying no
direction) and `harvest` (`unmarked`, `adjacent` or `culled`). The declared pass writes it with
every list empty. Resume position is read from the commit trail. A user may reset to a stage
commit and resume from there. Per-leaf work inside an inference stage
stays uncommitted until the stage boundary; a resume over an interrupted stage restores the
kbase-owned paths to the last stage commit and re-runs the stage. **A build in flight owns
`kb-root/`:** a resume restores every path under it to the last boundary, whatever was done to it
since, and nothing vets what it finds there. An edit made while a build is interrupted is the
editor's to carry in git, by waiting for the build or by reverting the commit that reset it; a write
op or `update-format` attempted while a build or a scoring run runs is refused (§8).
Commits are scoped by pathspec to kbase-owned paths.

**The state store holds what does not belong in the repository** — the holder lock and the record
the holder writes, which `status` and `cancel` read: the holder's pid, its start, the verb that owns
the run (`build` or `score`) and, for a scoring run, its run id — `progress.jsonl`, per-call
evidence, the docgraph records (§5), the scoring runs under `score/` (each with its ledger and
`captures.jsonl`, every call's closed text, prompt and reply, failures included; §8), a
per-unit answer cache keyed by input hash, and the captured output of a build the MCP server starts
(§11). Nothing in it is authoritative for position. A clean exit removes the holder lock's file,
cancel included, so a file standing in the store is never evidence of a live holder. The kind is
required of the record: one naming neither verb is a store another version wrote, `refused` with
`check: state-dir`, `path` the record, and never read as a build's. The store
lives at `$XDG_STATE_HOME/kbase/<key>/`, where `<key>` is the first 16 hex digits of the SHA-256 of
the absolute, symlink-resolved `kb-root/` path, overridable with `--state-dir`; it holds paid-for
inference and survives `git clean`. **The operation lock is in the repository's git directory**,
`kbase-operation.lock`, recording the state-dir: a repository carries one run at a time — a build or
a scoring run, never both — whatever `--state-dir` each names. Given no `--state-dir`, `status`,
`cancel`, `wait` and `build-trail` read the store the operation lock names while its file stands
and names one, else the default store; a new run never adopts it. A second run is `refused` with
`check: lock`, `path` the running run's state-dir, and no remedy, its `detail` naming the holder's verb and pid: "a
build (pid 4321) is running; its state-dir is …", or "a score run (pid 4321) is running; …". Where
the operation lock is held but names no state-dir yet, the second run gets `retry`; where it names
one whose holder record is not yet written, the refusal says a run is starting. Every lock is
advisory and exists on Unix only; on Windows `status` cannot see a running run and no lock-based
refusal or `retry` is given.

### The state store is versioned

`<state-dir>/format` holds one integer, currently `1`, written
when a build first holds the store; a store with no `format` file is version 1. `build`, `status`,
`wait`, `cancel`, the MCP `build` tool and `build-trail` refuse a store newer than this kbase:
`check: state-dir`, `key` the `state-dir` argument, `path` the store, remedy "update kbase, or use another
`state-dir`", the argument in both spelled as [§7](#item)'s `key` states. A build converts an older store in place before it uses it; the conversion is
stepwise and resumes if interrupted. The store's version is independent of the KB format version
(§10).

**The state store is pruned only by a build or a scoring run.** At a build's start, the captured
output of builds the MCP server started (`reports/build-*`, §11) is pruned to the newest six by the
start each one's name records. At a scoring run's start, once its own run directory stands,
the run directories under `score/` are pruned by kind: a real run's id is `run-<id>` and a dry run's
`dry-<id>` (§8), a directory's kind is its name's prefix, and the invocation keeps, of its own kind,
16 runs — the run it names, created or resumed, and the newest 15 others by the start their
`run.yaml` records, equal starts ordered by name with the later name newer — and removes the rest of
that kind whole, captures included. The captured output of scoring runs the MCP server started
(`reports/score-*`, §11) is pruned at the same point to the newest 16 by the start each one's name
records. A dry run never removes a real run, nor a real run a dry one,
and a dry run an earlier kbase minted as `run-…` counts as real. A directory whose `run.yaml` is
absent or unreadable has no start to order by and is left. An invocation refused before its run
directory stands — a `--resume` finding no run to continue among them — removes nothing. A
resumed run's `captures.jsonl` loses a torn last line before anything appends to it, as its ledger
does. A build trail's answer cache, multichoice-ask records and call captures are kept in one
directory per trail, named by the trail's `start` boundary commit. At a build's start, before its first stage,
the current trail's directory and the newest five others stand, and the rest are removed whole,
along with the undivided cache, records and call captures an earlier kbase kept. At the same point, a fresh build
whose progress record exceeds 500 lines moves the earlier runs' lines to a file under a timestamped
name, keeping its own in the record; the 15 newest such files are kept. The stage reports are
kept. `status` and `cancel`
remove nothing.
Deleting the store is always safe, but it forfeits the answer cache, whose entries were paid for in
inference; `status` reports its size.

**Maintenance operations over a standing KB never require git; `init`, creation, is the verb that requires the worktree (§8).**

### `kbase status`

`status`' argument, its build result and its score result, and its refusals are [page §3.5
`status`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/status.md)'s: it reports the current or last run, a build or a score run, whichever is newest.
A build's `stages` is a mapping: `done` and `not-done`, the stage ids each way in walk order, and
`rewind`, the one line saying that each completed stage's commit, with the command that rewinds
to it, is in `kbase build-trail` ([page §3.5 `build-trail`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md)). The stages read are the newest build's
trail ([§5 Starting over](#starting-over)). Where `kb-root/` is absent, or the newest change to
`kb-root/` removed it and no stage boundary follows, and no build run holds the state store,
`state` is `none`, no stage is done, and `current`, `pid`, `started`, `updated`, `ended` and
`resume` are null. `recent-refusals`, `recent-fallbacks`, `cache-entries` and `cache-bytes` still
read the state store: they record earlier invocations, not a standing build.

What the page leaves to this document: a build's `resume` is absent where no identical re-issue
continues the work (§7) and absent where the recorded command does not read as a `kbase build`
invocation — at least two elements, `kbase` then `build`. A run whose record fails that supplies
none, as a run that recorded none does. `state` and `resume` are read from the newest run the build
stands at: a refusal or `retry` that recorded no boundary wrote nothing, so the run before it is the
one reported. `interrupted` is the one state the build never writes: it is derived from a run event
with no end event and a holder that is not live.
Progress is persisted to a file so a late or reconnecting monitor catches up. An inference stage's
units and fallbacks are recorded once its asks have all returned, so while they are in flight
`current` names the stage and its total with none done.

#### A score run's status

Where the store's holder record names a score run, `status` reports that run
in its own shape instead of the build's: `state-dir`; `state` (`running` while the record holds the
store, else `interrupted` where the run left no result document, or the outcome
its result document records — `done`, `cancelled`, `failed`, `refused` or `retry`; a run written
under an earlier release may record `bounded`); `pid`;
`started`; `run`; `pass`; `counts` (`total`, `done`, `failed`, `skipped`, `remaining`, read from the
run's `items.jsonl`, the status `ledger.jsonl` last records for each, and, while the run walks, each
answered item's result file); `scored` (the items whose
value landed); `failures` (each failed item and the reason its line recorded, with the `remedy` the
line records where it records one, §8); and `resume` (the call
continuing the run, `score` with `--resume`, as its result document carries it, where the run
stopped short of `done`). It is the run's own records re-read, not a second
document: `wait` prints the result document the run wrote beside its ledger. A store no live run
holds, and whose record names no score run, reports its newest run, whichever verb it was: the later
of the newest score run's start, read from its `run.yaml`, and the newest build invocation's, read
from the progress record — each the record the run already wrote. A run directory whose `run.yaml` is
absent or unreadable left no record of when it started and is skipped. Where the store records
neither run, or the two starts cannot be compared — a missing or unparsable stamp on either side — it
reports the build as above.

### `kbase cancel`

`cancel`'s argument, result and refusal are [page §3.5 `cancel`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/cancel.md)'s: it signals the run that
holds the state store, of either kind — a build or a scoring run, as the holder record names it —
and the run continues only when asked to, by `kbase build --resume` or `kbase score --resume`.

What the page leaves to this document: the event is logged, the lock is released, and the run
process exits `cancelled`. Cancel costs only the calls in flight: an answer already returned stands
— in a build's answer cache, and in a score run's ledger; a cancelled score run's `resume` is the
call read from its result document. A store no run holds is `refused`: `check: lock`, `path` the
store's holder lock, detail "no run holds the state store".
### `kbase wait`

`wait`'s arguments and refusal are [page §3.5 `wait`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/wait.md)'s: it returns when the run the pid names,
of either kind, is no longer progressing — the store stands at another run, or at that run with a
state it stopped at — then prints that store's `status` document, so a command run in the
background carries the run's terminal state when it exits. `--state-dir` defaults as `status`'s.
It takes no lock, and reads no commit trail until that final document.
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
   them except `models`, `configure-providers`, `configure-models`, `show-config`, `probe` and the `auth` subcommands; `init` omits it from results written
   while no `kb-root/` stands (§8).
3. `migration`: a conditional record — `from`, `to`, `note`, `warnings` (one item per thing the
   conversion dropped or changed that the converted files do not show: `path`, `line` where it
   names one, `detail`; `[]` when none) — present on the result of every verb that reads a KB
   through the loader on an older format version, and absent otherwise (§10):
   `update-format`, `verify`, `export-latex`, the queries, `score`
   and every write op — on `score` and a write op, whose run is refused while the conversion is
   pending ([page §2.6](NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)), beside that refusal. `update-format`'s lists only the warnings
   its own keys do not ([§7](#item)'s per-subcommand table). It follows `kb-root` where the result carries both.
4. The subcommand's own keys, in the order its table gives.
5. Exactly one of these, chosen by `outcome`, and it closes the document: `refusals` for `refused`
   and `retry`; `failures` for `failed`; `cancelled` for `cancelled` — a mapping of `stage` and
   `unit` on a build, and of `item` on a `score` run (§8). `done`, `unchanged` and `bounded` close
   on no such key.

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
| `check` | The named check or refusal class: a verify check name, `error`, `write`, `usage`, `provider`, `dirty-paths`, `include`, `metadata-key`, `metadata` (a load error, [page §2.4](NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)), `lock`, `state-dir`, `inputs`, `trail`, `pandoc`, `unknown-id`, `kb-format`, `kb-root`, `worktree`, `export-dir`, `index`, `symlink`, `conversion`, `demoted`, `dependency-cycle` |
| `path` | The file the item is about |
| `entry` | 1-based index into the values' `entry` list |
| `key` | The setting the item is about: a values key, spelled as its configuration file spells it, or a flag or positional argument, spelled as the surface the call came through takes it: `--name` at the command line, `name` over MCP; a positional `<name>` at the command line, as its usage line spells it ([page §4.2](NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)); an item refusing a value given for a named thing may carry that value as given, as the auth verbs carry the provider name |
| `line`, `column` | 1-based position in the values document or in `path` |
| `allowed` | The closed set the value must come from |
| `remedy` | The command that clears the item, where one exists (e.g. `kbase update-format`, `git init`); on a configuration or provider refusal it names `kbase show-config`, after the command that clears the item where one exists; a word in angle brackets is a placeholder the one running it fills in, as `<command-that-prints-the-key>` (§9, "A refused key"); a `kbase` command a tool binds — in a remedy, its prose and a result field such as `resume` alike — is, over MCP, that tool's call: the tool's name, then its arguments as compact JSON, keys sorted, carrying only the arguments that tool's schema has (a query op's `query {"operation":"<op>","arguments":{…}}`, `update-format {}`); at the command line it is `kbase <command>`, help included, its positionals, then each flag in name order, a true bool bare, a placeholder for a repeated positional `<name>...`; a command no tool binds (`auth`, `wait`, `mcp`) is a command line on both surfaces. A command or argument that is a field's whole text stands bare; inside other text it is set off in backticks over MCP, and at the command line a command alone. An argument with its value reads `name: value` over MCP and `--name value` at the command line |
| `detail` | One sentence; under `retry` and `failed` it ends with what the outcome asks of the caller: `re-issue the identical call later`, or `re-issuing the identical call will not fix it` |

A refusal lists every offending item. A usage error is one item with `check: usage`. A
provider-selection refusal's `key` is the `provider` argument, spelled as `key` states, and lists every entry name as `allowed`.

**Per-subcommand keys**, following `kb-root`:

| Subcommand | Keys |
|---|---|
| `build` | `state-dir`; `roles` (one record per role the build resolved, in role order — `decision`, then `text` — in the role record's shape below; absent where it resolved none, as under `--no-inference`); `through` (absent where this invocation walked no stage); `no-inference`; `resumed` (bool); `restored` (paths reset to the last boundary); `stages` (list of `stage`, `commit` — null if unrecorded — `dropped` (row ids), `report` (path to the stage's full findings in the state store); absent where this invocation walked no stage); `resume` (the call that continues the build, spelled as `remedy` spells a command: at the command line `kbase build` with `--resume` and the flags the invocation carried that a resume still needs, all in name order — `--no-inference` where given, `--ask-concurrency` where given, `--state-dir`, `--config-dir` and `--project-dir` where given, `--config-dir` written absolute; over MCP the `build` tool's call with `resume: true` and the same arguments but `config-dir`, which no tool takes — never the inputs or `--through`; absent where an identical re-issue continues nothing: `done`, `unchanged`, `bounded`, `retry`, a `failed` or `cancelled` run that walked no stage, and a `refused` run that recorded no boundary) |
| `status` | `state-dir`; `state` (§6's vocabulary, `interrupted` included); `pid`; `started`; `updated`; `ended`; `no-inference`; `stages` (list of `stage`, `commit`); `current` (`stage`, `units-done`, `units-total`); `recent-refusals`, `recent-fallbacks` (items, the newest 10 of each); `cache-entries`, `cache-bytes` (integers: the answer cache's entry count and total size in bytes, §6); `resume` (absent for `finished`, `running` and `bounded`; read from the run §6 names, spelled as `build`'s for the surface reading it; `recent-refusals` items are spelled so too, an item an earlier kbase recorded as written) |
| `cancel` | `state-dir`; `pid`; `resume` (spelled as `build`'s; absent where the record's command does not read as a `kbase build` invocation, §6) |
| `init` | `written` (every path this call wrote — the spine, the two empty record files, the stamped readiness documents and the refresh's output, merged, deduplicated, path-sorted); `removed` (as `update-format`'s; `[]` at birth) |
| `update-format` | [Page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)'s keys, in its order: `written` (includes every register page, sheet and maintained-block document written); `removed` (files the KB no longer calls for — a register page no node is rendered on, a sheet the volume-count rule drops, a file a format migration made obsolete (§10) — kb-root-relative inside `kb-root/`, repository-relative beside it; `[]` when none); `unfenced`; `sheet` (`drawn` where the sheets were rendered this call, else `not-drawn`: nothing was drawn, whether or not a sheet stands); `dot` (`"not found on PATH"`, present only when `sheet` is `not-drawn`); `commits` (as a write batch's); `uncommitted` (as a write batch's); and, where the format changed, `ids` (`from`, `to`: each work's and axiom's `3.0.0` id and the id minted in its place), `bare-ids` (`path`, `line`, `id`), `no-edge` and `strengthen-by` (`id`, `text`). `unfenced` onward are present only on `done` and `unchanged` |
| `verify` | `findings` (items the KB's structure carries that are not faults — every `demoted` record (§10), as an item with `check: demoted`, `path` the register page of the record's `target`, omitted where that target has no host, and `detail` the edge and the relation it replaced; `[]` when none); its faults are `refusals` items, each carrying the remedy [page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md) names for it where one op clears it |
| `write` and every write op | [Page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s keys, in its order: `dry-run`; `entries` (one per batch entry, in order: `entry`, `operation`, `outcome` — `done` or `unchanged` — and on an add's entry ([§8](#add-kind)) `id` (the node it minted) alone but on an `add-axiom` entry, which adds `heading` (`marked` or `appended`), on an edit's entry ([§8](#edit-kind)) `id` (the node it edited) alone but on an `edit-work` entry that changes `label`, which adds `rewritten` (each line it rewrote a reference on, `path` and `line`, ordered by path, then line; `[]` where none), on a `move-node` entry ([§8](#move-node)) `id` (the node it moved) alone, on a `remove-node` entry ([§8](#remove-node)) `id` (the node it removed) and `removed-edges` (each edge record it removed, as its `edges` row), on an `add-citation` entry ([§8](#add-citation)) `document` and `line` (where it wrote the citation), `id` (the node cited) and `link` (the text written),; on an edge write's and a `restore-demoted` entry ([§8](#edge-ops), [§8](#restore-demoted)) `source`, `target` and `relation`, the edge it names, and on a `restore-demoted` entry then `restored` (the relations it put back); `[]` where the batch did not apply); `written` (kb-root-relative paths the batch wrote, or under `dry-run` would write: records, leaves and derived files); `removed` (the derivation's, as `update-format`'s); `sheet` and `dot` as `update-format`'s, where the derivation ran; `commits` (each commit made: `commit`, `label`, `paths`; `[]` without `commit`); `uncommitted` (a detail line, present only where the batch commit failed) |
| `citations-of` | `count`; `results` (every citation of the node, complete, unsliced: `path`, `line`, `text` ([§8](#citations-of))) |
| `find` | `count`; `results` (every node the filters keep, complete, in the sort's order, cut only by `limit`: `id`, `node_type`, `title`, `label`, `host`, `strength`, `dependents` ([page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/find.md))) |
| `edges` | `count`; `results` (every edge record the arguments keep, complete, in its fixed order: `source`, `target`, `relation`, `bearing` (as the index writes it: a number in `[0, 1]` or `*pending*` spelled as text, null only where the pairing carries none), `context` (null where the edge has none), `manually_scored` (the record's flag (§8), null where its record carries none) and `replaced_relation` ([page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md))) |
| `trace` | `count`; `results` (one row per edge the walk reaches, complete, in the sort's order, cut only by `limit`: an `edges` record, then `depth`, `strength` and `contribution` ([page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/trace.md))) |
| `show`, `stats` | `results` (the query's mapping; a node record carries `node_type`, `id`, `title`, `host`, `host_anchor` and `summary` ([page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/show.md)), then the keys its kind carries: `rigor` and `strength` on a claim, with `build_status`, `strength_band`, `rationale` and `premise_count` after them; `rigor`, `strength`, `strength_band` and `rationale` on an analysis; `status` and `rigor` on an experiment; `label`, `rigor` and `rationale` on a work; `label` and `rigor` on an axiom, its `rigor` `1.0` by rule; `show` adds `manually_scored` (§8) after `rigor` on a claim, an analysis and an experiment; `strength` is a number rounded to two decimals for reading, `rigor` the number the node holds; `stats`' mapping carries one count per node kind — `claims`, `analyses`, `experiments`, `axioms`, `works` — then `edges`, `demoted_edges` and `subtree_aggregates` (per volume, `claims` and `experiments` ([§8](#stats))), then `rigor_manual`, `rigor_model`, `bearing_manual` and `bearing_model` (the records whose type carries `manually-scored`, `*pending*` values included, counted by the flag), then `relations`, then `strength_bands` and its two ranked sections, whose records carry `strength`; `relations` carries one record per relation in this order — `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted` — each `edges`, the relation's row count, `walked` (bool), then on a scoring relation `bearings`: one record per distinct bearing its rows carry, `bearing`, as the index writes it, and `edges`; numbers ascending, then `*pending*`, then null, `[]` where the relation has no rows) |
| `models` | `provider`; `provider-rule`, `provider-layer` (the rule that named the provider — `flag` or `sole-provider` — and the layer that supplied it; this verb always selects one); `models` (ids, sorted ascending) |
| `show-config` | `layers` (one record per directory this invocation read, in precedence order: `path`, `source` (`explicit`, `environment`, `repository` or `global`), `exists`, `providers-exists`, `config-exists` — the directory's presence and each of its two files'); `providers` (one record per entry, in name order: `name`, `base-url`, `decisions-flavor` (the surface it speaks, `systemone` where the entry is silent), `key` (`store`, `file` or `none`), `key-file`, `upgrade` (for a `file` key, `cat <key-path> | kbase auth add <provider>`, the resolved path single-quoted where a shell would split or expand it), `layer` (the layer that supplied the entry), `usable`, `fault`; a provider a dropping fault removed has `name`, `layer`, `usable`, `fault` only); `config` (the merged choices: `models` (`decision`, `text`, `scoring` — each the reference `config.toml` states, null where unset); `sampling` (`text`, `scoring` — each `temperature` and `thinking` as `config.toml` states them, null where unset)); `selection` (`provider` and `rule` — `flag` or `sole-provider` — or `detail` where no provider was named); `roles` (one record per role in role order, in the role record's shape below, each with `base-url`, `sources` (`base-url`, `model`, `key`, and for a chat role `temperature`, `thinking` — the layer that supplied that field, one of `layers`' four names, `store` for a key the store supplied, null where the field is unset), a chat role's `temperature` and `thinking` (null where unset: kbase's default, §9.2) and `detail` where the role does not resolve); `diagnostics` (items: every fault, `path` the `providers.toml` that carries it, the stored key file that does not open, with `remedy` the refused-key remedy (§9, "A refused key") where it is truncated or does not decrypt, or the store's directory where it stands and does not list; an entry dropped because the key file `providers.toml` names cannot be read carries the same remedy (§9.4); a warning per entry whose stored key leaves its `apiKeyFile` unread, `path` the `providers.toml` stating it; and the resolution failure where a role does not resolve) |
| `configure-providers` | as [page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-providers.md) states, in this order: `provider` (lowercase); `layer` (`global`, `repository`, `explicit` or `environment`); `written` (`[]` with outcome `unchanged`); `models` (ids, sorted ascending, where dialled); `next` (a list of strings, where no key is stored or the dial failed: [§9](#provider-commands), "Provider commands") |
| `configure-models` | as [page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-models.md) states: `models` (`decision`, `text`, `scoring`, as the written `config.toml`'s `[models]` holds them, null for a role it does not set); `written` (`[]` with outcome `unchanged`) |
| `probe` | `provider`; `provider-rule`, `provider-layer` (as `models`'s); `selection` (the selection that answered: `decision`, `text` or `scoring` where a role flag or the default named it, else the `<provider>/<model>` reference `--model` named); `model` (the id asked); `temperature`, `thinking` (chat selections only: the effective settings — `temperature` a number; `thinking` `off`, `low`, `medium` or `high`); `first-token-ms`, `wall-ms` (integers: request start to the first non-empty content or reasoning delta — zero where none arrived — and the call's end-to-end time); `finish-reason` (null where the stream reported none); `reply` (the reply's text on a chat surface, and on a decision surface the answer's probability, chosen letter or type); `usage` (a record: `reported` — whether the provider sent a usage block at all, so a zero count is a measurement only where it is true — then `prompt-tokens`, `completion-tokens`, `reasoning-tokens`, `cached-tokens`) |
| `auth add`, `auth remove` | `provider` (the name given, folded to lowercase; never the key) |
| `auth list` | `providers` (the names with a stored key, in name order; `[]` when none) |
| `export-latex` | `written` (absolute paths; outside `kb-root/`); `volumes` (the volume directories exported, entry-point order); `report` (the path of the export report written under the out dir — one YAML list, every simplification and finding line the run produced, each naming the form, the source path and the reason) |
| `score` | `migration` (conditional, the shared vocabulary's; it stands before `state-dir`, on the refusal of a run over an older KB, which `update-format` converts first); `state-dir`; `pass` (the set it walks: one pass, or `all`; a named node's `rigor`, a named edge's `bearing`); `roles` (one record, `scoring`, in the role record's shape below; absent under `--dry-run`); `run` (the run's id); `ledger` (the run's directory, absolute); `counts` (a record: `total`, `done`, `failed`, `skipped`, `remaining` — the items by the status the ledger last records for each, so a resumed run reports the whole run); `ask` and `skip` (a dry run's alone: the items a run would ask and the dirty manually scored items it would skip, named as `scored` names them, in the walk's order ([§8](#the-closing-record))); `scored` (the items whose value landed, in the walk's order — a node's id, or an edge's `<relation> <source> → <target>`); `resume` (the call that continues the run — `score` with `--resume`, spelled as `build`'s, over MCP the `score` tool's call — carrying `--state-dir`, `--project-dir` where given, `--scoring-model` where given, and `--config-dir` written absolute, and no argument of the scope; present wherever an item failed or was not reached — a `skipped` item does not make it present — and it stands before the record the outcome closes on; absent from a dry run, and from a stop before the walk) |

A `status` read over a score run reports §6's score shape in place of the build keys above.

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

`provider`, `provider-rule` and `provider-layer` stand together on every role that resolved: a pool
entry supplies every endpoint. `provider-layer`'s `environment` is the directory `$KBASE_CONFIG_DIR`
names (§9).

**The model is the one requested, never one observed.** `model` is the id this invocation asked for,
and no result document or ledger line states one a server ran.

### Bounded results

`find`, `edges`, `trace` and `citations-of` return their complete answers ([page §2.8](NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)), `find`'s and
`trace`'s `--limit N` allowed only beside `--sort`. No list query pages. `stats`' ranked sections stay at 10, and the `bearings` of its `relations` hold
one record per distinct bearing value. No other subcommand's result grows with the KB; `build`'s
per-stage detail goes to the `report` file, not stdout.

**Exit codes** derive from `outcome`:

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success, including a `--through`-bounded run |
| `refused` (including verify faults and usage errors) | 1 | Wrong input or KB state; nothing written past the last checkpoint |
| `retry` | 2 | Another writer holds the KB write lock past the wait (`check: lock`, `path` the repository root), or another run holds the state store (its holder lock, §6); re-issue identically later. A run holding the operation lock is not `retry`: it is `refused` with the `lock` item (§6) |
| `failed` | 3 | Defect in the tool, its input or the environment; re-issuing will not fix it |
| `cancelled` | 4 | Stopped on request; resumable |

---

## 8. Maintenance subcommands

Every subcommand below is reachable by an external agent harness as a builtin tool — its own, as
an operation of the `query` tool, or, a write op, as an entry of the `write` tool's batch (§11) — `probe` excepted — a
tool would let an agent poll a model, which no KB operation asks for (§11); `score` starts a detached
child that asks one model call per item and returns at once with the run's identity, so the turn is
not held for the walk's duration (§11). The rules in this section govern each.

### Values input

Every write is a batch, read and checked as [page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) states: YAML (JSON accepted) on stdin or in the
file `--values` names, `kbase write`'s entries each naming its op under `operation` and an op's own
subcommand's naming none. The per-op key vocabulary is closed, and stated once, in kbase's write package; it is not
copied into prose. A refusal names the key and its position, and lists as `allowed` the closed set
the key or value must come from, where there is one; `kbase <op> --help` describes the op. One edge
has one name across the ops: an op naming one edge takes it as the edge record's `source`, `target`
and `relation`, an `edges` row's keys, and an op over one existing node takes its `id`. A prose
value containing U+0008 or U+000C is refused.
### Idempotence

A batch's outcome, and each entry's, is [page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)'s. [Page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)'s
existence rule — an add of what stands refused, a removal of what does not refused, an edit to a
value that stands `unchanged`, and nothing adopted — is the rule of every write op: `add-<kind>`
([§8](#add-kind)), `edit-<kind>` ([§8](#edit-kind)), the edge writes ([§8](#edge-ops)),
`restore-demoted` ([§8](#restore-demoted)), `remove-node` ([§8](#remove-node)), `move-node` ([§8](#move-node)) and `add-citation` ([§8](#add-citation)). A
re-issued add, removal, restore or citation is refused and a re-issued edit or move is `unchanged`, each leaving `kb-root/` byte-identical. There are
two exceptions. `init`'s creation is birth-once, and a re-issue where a `kb-root/` stands
is `refused`, never `unchanged`. `update-format` requires a clean `kb-root/`, so a re-issue
before its output is committed — by its own `commit` or by hand — is refused `dirty-paths`; over
a clean tree it already converted, it reports `unchanged`.
### Writes leave nothing stale

Every write batch is followed by the derivation, inside its all-or-nothing ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)); no
flag skips it. A batch refuses, before it writes anything, a `kb-root/CLAUDE.md` that is not the
redirect (`check: agents-redirect`, §3), which the derivation would refuse.
### One write lock

A write batch and `update-format` hold the KB write lock for the whole run: from loading
the KB to the result, the batch's derivation and `update-format`'s re-derivation included. `init` holds it from before `kb-root/` exists — the lock sits on the directory `kb-root/` stands in, so it
is available at birth — through the skeleton, the trailing refresh and the birth commit to the
result. `score` takes it around its one landing — every value the walk produced, after the walk — and
around its start refresh, never across the run (§8). A writer that cannot take it within 30 s gets `retry` with `check: lock`, `path` the
repository root, and the remedy. A write verb probes for a run holding the operation lock (§6) before
it waits on the write lock and again after taking it: where one runs it is `refused` with the `lock`
item at once, taking no wait. A build waits out a write in progress, up to the same 30 s, before it
writes (`retry` if it cannot); its own writes take no lock. A scoring run's landings and refreshes
make no operation-lock probe, writing as the run that holds it. `verify` and the queries take
no lock. On Windows no
advisory lock exists (§6): neither the `retry` nor the run-running refusal is available there, and
one writer at a time is the operator's to keep.

**Batches** are all-or-nothing, their derivation included ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)).

### `init`

```
kbase init [--title TEXT]
```

Creates a KB directly — no LaTeX, no build: the equivalent of `git init` for a KB. Run at any depth
in a git worktree; it resolves the repository root and works at `kb-root/` in the git worktree root (§1's
placement). **Refusals come before any write** (§7 item shape): a `--title` holding a line break
(CR or LF) — `check: usage`, `key` the `title` argument, spelled as [§7](#item)'s `key` states: the title is the entry point's one heading line;
outside a git worktree — `check: worktree`, `path` the start directory, remedy `git init`, which
the tool never runs itself; and
against a `kb-root/` that exists in any state, an empty one included — `check: kb-root`, no remedy:
creation is birth-once, a re-issue where a tree stands is `refused`, never `unchanged`. That refusal
is the one a fresh `build` gives over an existing `kb-root/` ([page §3.7 `init`](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/init.md), [page §3.5
`build`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)): one item, `path` the `kb-root/`, the same detail. The result of
a refusal that found no `kb-root/` carries no `kb-root` key; the refusal items name the paths.

Then, under the KB write lock from before the directory exists (§8): it writes `entry-point.md` —
with the `--title` text, or the repository's directory name, as the only heading, no children and
no frontmatter (§10) — the format stamp file, `.index/format.yaml` (§10), and the two record files,
`.index/nodes.yaml` and `.index/edges.yaml`, holding no record; stamps the readiness documents, `AGENTS.md`,
`CONVENTIONS.md` and the `CLAUDE.md` redirect, each only where absent, `AGENTS.md` pinned that this
KB is authored, not distilled — its author writes what it holds, a claim at a time; then
refreshes, so the derived metadata layer is present at birth
(§3's full sheet always, the placeholder where `dot` is absent, the digest and volume sheets
not drawn: no volume holds a node). Every input to the skeleton is kbase's own, so `init` runs no
`verify` of its own; `verify` (§8) over a freshly born KB is green. Finally the birth commit: `kb-root/` committed with the plain subject
`kb-init: knowledge base created` — never a `kb-build:` subject, so the trail of §5 stays empty,
a `build --resume` finds nothing to resume, and a fresh `build` is refused over an initialized KB as
`init` is: content joins
§8's ops, never a later build. The commit rides the host's git identity and configuration; git's own
refusal (no identity configured) is a `failed` naming git's words, the written tree standing
uncommitted for the operator to commit or remove. Result keys: §7.

### Write ops

`kbase write` runs one batch whose entries each name their op, and each op below runs as a batch of
that op alone ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)). Each result's keys are §7's.

A batch's `commit` option (`--commit` at the command line) is [page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s "`commit`": the
pre-flight commits and their labels, the batch commit labelled `kb-write: <ops> <ids>`, each commit
covering `kb-root/` only, and `check: worktree`, remedy `git init`, outside a git worktree. What
this release adds to the page and holds:

- `<ops>` is the batch's ops, each once, in entry order, joined by `,`; for an empty batch, the
  command's (`write` for `kbase write`); `<ids>` is the ids its entries entered or name in their `id`, `source` and
  `target`, each once, in entry order, joined by `,`.
- A pre-flight commit is made only where its part of `kb-root/` has uncommitted changes, and a
  batch that writes and removes nothing (`unchanged`) makes no batch commit.
- Each commit's `paths` are the repository-relative paths it holds, a rename as both sides.
### `restore-demoted`

`restore-demoted` is [page §3.2](NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/restore-demoted.md)'s restore of [page §1.3](NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)'s demoted records: its keys, its result's
`restored`, and its refusals. An entry no demoted record from `source` to `target` answers is refused
"no edge" ([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)), so a re-issued restore, and a repeat of an entry within one batch, is
refused, `entry` the repeat's number. The derivation follows it as it follows every batch
([page §2.3](NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)). What this release adds to the page and holds:

- The "no edge" item is as the edge writes' ([§8](#edge-ops)): no `check`, `key` `relation`, `detail`
  `no edge demoted <source> → <target>`.
- The cycle refusal names `target` as its key, and its remedy names, beside removing another edge of
  the cycle, `remove-edge` with `relation: demoted`, which leaves the edge out of
  the graph, with no record of it.
- `restored` lists the replaced relations in the records' order.
### `add-<kind>`

`add-claim`, `add-analysis`, `add-experiment`, `add-work` and `add-axiom` are [page §3.1](NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md)'s node
adds: each op's keys, defaults, result and refusals, and the identity an add checks — a title its
kind holds anywhere in the KB, a label a work or an axiom of its kind holds — refused "node exists"
([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)). A hosted node's marker is placed by `document`, `line` and `snippet` as [page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)
states; the mark an add writes is [page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)'s, and `title` and `rationale` take [page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s
"Values" check. What this release adds to the page and holds:

- A refusal names its key: `title` or `label` for "node exists" (its `detail` is
  `node exists: <id>`), `line` for a line past the document's end or a fenced block's opening or
  closing line, and `snippet` for a line that does not contain it. The snippet is compared with each
  whitespace run collapsed to one space, and the `snippet` refusal's `detail` names up to five lines
  holding it, those nearest the named line.
- An add given a `rigor`, `*pending*` included, records the judged hash of the node's statement
  as its marker places it ([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)); one given none writes the never-judged `judged-hash`, all
  zeros.
- `add-axiom` reads an unmarked heading of its text as a `###` heading whose text, markers removed
  and trimmed, is `Axiom <label>: <title>`; it marks that heading by ending its line, trailing
  whitespace dropped, in the axiom's marker, and creates `axioms.md` where the file is absent. Its
  refusal of more than one such heading is keyed `label`, its `path` `axioms.md` and its `line` the
  first.
- An add whose summary entry shifts a standing entry's repeat suffix (`-1`, `-2`) — a title of
  the same slug rendered before it — re-points every link into the shifted entry to its new anchor
  in the same write, as an edit does ([§8](#edit-kind)). `add-axiom` appends its heading after
  every other, so it shifts no anchor of `axioms.md`.
### `edit-<kind>`

`edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work` and `edit-axiom` are [page §3.1](NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/edit-claim-edit-analysis-edit-experiment-edit-work-edit-axiom.md)'s
node edits: the fields each kind's edit may change, at least one given, a title change or an
axiom's label change keeping every link to the node live in the same write ([page §1.5](NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)), a work's
label change rewriting every reference that carries the old key as a key, the result's `id` and
that label change's `rewritten`, and the refusals; `label` is checked as `add-work` and `add-axiom` check it ([§8](#add-kind)). An edit
whose every value already stands is `unchanged` ([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)); the mark a `rigor` writes is
[page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)'s, and `title` and `rationale` take [page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s "Values" check. What this release adds to the
page and holds:

- An entry giving no field, and a field the kind does not carry, are each a shape refusal whose
  `allowed` is the fields the kind's edit may change, neither `id` nor `operation` among them.
- An `id` of another kind is refused at `id`, its remedy naming the edit op for that kind; an `id`
  no node carries is refused `check: unknown-id` at `id`, `detail` `no node <id>`; a `title`
  or a `label` another node of the kind carries is refused at that key, its `detail` naming that
  node; a work `label` a citation span or a reference-list entry already uses as a key is refused
  at `label`, one item per such line, each naming it by `path` and `line`.
- An edit giving a `rigor` — `*pending*` included — on a claim, an analysis or an experiment
  records the judged hash of the node's statement as the edit leaves it ([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)); a scoring run's
  landing, which sets a number, records it the same way. An edit giving no `rigor` keeps the
  record's `judged-hash` as it stands.
- A link re-pointed is an inline link or a reference definition outside code, in any document but
  the summary pages, which the edit does not rewrite: the derivation renders them. Where the edit
  moves a summary entry's anchor — the node's own, or another entry's repeat suffix shifting with
  it — every link into that entry is re-pointed too. A citation of the node whose destination resolves to the node's
  link target before the change is pointed at the target after it, written relative to the citing
  document. Any other link stands.
- A work's label change reads citation spans, reference-list entries and links into them outside
  code, a blockquote's prefixes read past, in every document but the summary pages.
- A title under which a naming-form citation of the node would not read, whole, as one link — an
  unbalanced `[` or `]` — is refused at `title`, naming the citing `path:line`.
- `edit-axiom` rewrites every `axioms.md` heading carrying its marker, and appends its heading
  where none stands, creating `axioms.md` where the file is absent.
### Edge ops

`add-edge`, `edit-edge` and `remove-edge` are [page §3.2](NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/index.md)'s edge writes: each op's keys,
result and refusals, over [page §1.2](NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)'s relations and legal pairings and [page §1.3](NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)'s demoted records.
An add of an edge that stands is refused "edge exists", and a removal or an edit of one no record
holds is refused "no edge" ([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)); the mark a bearing writes is [page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)'s, and
`context` takes [page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s "Values" check. What this release adds to the page and holds:

- "edge exists" and "no edge" items carry no `check` and `key` `relation`
  ([§7](#item)); their `detail` is
  `edge exists: <relation> <source> → <target>` and `no edge <relation> <source> → <target>`, an
  "edge exists" remedy naming `edit-edge` and a "no edge" remedy naming `edges`.
- A `source` or `target` no node carries is refused `check: unknown-id` at that key, `detail`
  `no node <id>`, as an edit's `id` is ([§8](#edit-kind)).
- A pairing the relation does not join is refused at `relation`; on `add-edge` its `allowed` is the
  relations joining the two kinds, in the relations table's order, `references` always among them.
- `relation: demoted` on `add-edge` or `edit-edge` is a shape refusal at `relation`, `allowed` the
  five other relations, its remedy naming `restore-demoted` (and, on `edit-edge`, `remove-edge`).
- A bearing on `references` is refused at `bearing`. An `edit-edge` entry giving neither `bearing`
  nor `context` is a shape refusal whose `allowed` is `bearing` and `context`.
- A cycle refusal names `target` as its key. A `bearing` given on a `supports`, `lifts` or `refutes`
  edge, `*pending*` included, records the judged hash of its two ends' statements as the batch
  leaves them ([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)); an added edge given none writes the never-judged one, all zeros, and
  an edit giving none keeps the record's `judged-hash` as it stands.
### `remove-node`

`remove-node` is [page §3.1](NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/remove-node.md)'s node removal: its key, its result's `id` and `removed-edges`,
and its refusals, checked at the point the entry applies ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)). An `id` no node carries
is refused "no node" ([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)), so a re-issued removal is refused. What this release adds to
the page and holds:

- The "no node" item is as an edit's ([§8](#edit-kind)): `check: unknown-id` at `id`, `detail`
  `no node <id>`.
- An axiom's removal takes every `### Axiom` heading carrying its marker out of `axioms.md`, and
  nothing else of that file.
- A removal that shifts another summary entry's or `axioms.md` heading's repeat suffix — a later
  heading of the same slug taking a lower one — re-points every link into the shifted anchor in the
  same write, as an edit does ([§8](#edit-kind)). A link into the removed node's own entry or
  heading has no new destination and stands: it is not refused unless it is a citation.
- Each `removed-edges` row is the row `edges` answers for that record ([page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md)), ordered
  by source, target and context, demoted records included.
- A mention refusal is one item per citing line and per mentioning field, each at `id`. A citation is read outside code, in every
  document but the summary pages, which the derivation renders; it is named by `path` and `line`. A
  prose field mentions the node where it cites it or holds its id bare, and is named by its record
  file as `path` and by record and field in `detail`; its remedy names the `edit-<kind>` of the
  record's kind or `edit-edge`, and for a demoted record's `context` a `restore-demoted` then
  `edit-edge`, or a `remove-edge` with `relation: demoted`.
- A work's span refusal reads each citation span's `data-cites` outside code, in every document
  but the summary pages, one item per span line at `id`, named by `path` and `line`; a
  reference-list entry of the label's key in any such document lets the removal through.
### `move-node`

`move-node` is [page §3.1](NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/move-node.md)'s marker move: its keys, its result's `id`, `unchanged` where the
marker already ends the named line, and its refusals, the named line's being the add ops' ([§8](#add-kind)).
The marker goes where [page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) places it, and each citation of the node is re-pointed to its
new host and section anchor ([page §1.5](NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)). What this release adds to the page and holds:

- A work's or an axiom's `id` is refused at `id`; an `id` no node carries is refused as an edit's
  is ([§8](#edit-kind)); a node with no statement marker is refused `check: statement-marker` at
  `id`.
- Every other occurrence of the node's marker is taken from its line, and a line it leaves without
  prose goes with it, so a node holding more than one marker is left with one.
- Every citation of the node outside code, in every document but the summary pages, points at its
  new target after the move, written relative to the citing document, whatever it pointed at
  before; its link text stands.
### `add-citation`

`add-citation` is [page §3.3](NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/add-citation.md)'s citation write: its keys, its three forms, its result's `document`,
`line`, `id` and `link`, its refusals, and its re-pointing, in the same write, of every link,
citation or not, into a heading whose anchor the citation moves. The link goes to the cited node's target and is written
relative to the citing document ([page §1.5](NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)), and a quote is checked against the cited node's statement
as [page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) reads it. What this release adds to the page and holds:

- `document` is any document of the KB, kb-root-relative; a summary page, which kbase renders, is
  none and is refused at `document`.
- `snippet` is matched on the line as written, outside code, links and statement markers, so a
  citation already in place is no match, and a re-issued entry is refused as a line that does not
  contain the snippet. A line past the document's end is refused at `line`.
- An `id` no node carries is refused as an edit's is ([§8](#edit-kind)): `check: unknown-id` at
  `id`, `detail` `no node <id>`. A hosted node with no statement marker is refused
  `check: statement-marker` at `id`. A quote or a paraphrase of a work or an axiom is refused at
  `snippet`, and a quote absent from the statement at `snippet`, or at `quote` where it is given.
- Where `snippet` is the cited id, the naming form is written whatever `quote` gives.
- A link that would not read, whole, as one link to the target — an unbalanced `[` or `]` in the
  `snippet`, or in the title the naming form writes — is refused at `snippet`, or at `id` for the
  title.
- The links re-pointed are the inline links and the reference definitions outside code, in every
  document but the summary pages, which the derivation renders; only a link's fragment changes.
### Queries

Each query derives its answer from the KB's records and statement markers and writes nothing;
`kbase <query> --help` states its arguments, and §7 gives its result keys. A KB converted at load
and not yet saved answers as converted, its result carrying `migration`; a KB whose load fails is
refused ([page §2.4](NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).
### `citations-of`

`citations-of` is [page §3.3](NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/citations-of.md)'s query: its `id`, its rows `path`, `line` and `text` ordered by
path, then line, and its refusal. Its answer is complete ([page §2.8](NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)): `count` and `results`. What this release adds to the page and holds:

- A citation is read by the id in its link title, wherever its link points, outside code, in every
  document but the summary pages, which the derivation renders; `text` is its link text as the
  document writes it.
- An `id` no node carries is refused `check: unknown-id` at `<id>`, `detail` `no node <id>`.
### `find`

`find` is [page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/find.md)'s query: its filters over every kind, `label` among them, its rows `id`,
`node_type`, `title`, `label`, `host`, `strength` and `dependents`, and its refusals. Its answer is complete, sorted and limited as
[page §2.8](NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s "Lists" and "Sorting" state: `count` and `results`. What this
release adds to the page and holds:

- A row's `strength` reads the derivation, so a bearing that moves a claim moves the list. Its
  `dependents` and `show`'s `premise_count` are structural — counts of edges, which no bearing
  changes.
- `--order` turns the named key only: ties under `--sort strength` or `--sort dependents` are
  broken by ascending strength, then ascending id, in either order.
- A refusal is `check: usage` at the flag: `--kind`, `--sort` and `--order` outside their
  vocabularies, each with it as `allowed`; `--pending` beside `--min` or `--max`; `--limit` beside no
  `--sort`, and `--limit 0`.
### `show`

`show` is [page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/show.md)'s query: its record of one node of any kind — `host`, `host_anchor` and
`summary`, then its kind's own fields — and its refusal. What this release adds to the page and
holds:

- `host_anchor` is the anchor of the section holding the node's first statement marker in its host.
- A pending `rigor` or `strength` reads `*pending*`, as in every query; null is only for a value
  that does not apply.
- An `<id>` no node carries is refused `check: unknown-id` at `<id>`, `detail` `no node <id>` ([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).
### `stats`

`stats` is [page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/stats.md)'s query, its keys as [§7](#item) lists them. Its help names the `find` question each
ranked section previews: `highest_leverage` is the first ten of `find --kind claim --max 0.65
--min-dependents 1 --sort dependents --order descending`, and `weakest` of `find --kind claim
--min 0 --sort strength`. What this release adds to the page and holds: `subtree_aggregates` is
one record per volume, keyed by the volume's directory in path order, each `claims` and
`experiments`, the counts of those whose statement marker stands in a leaf under it; a KB with no
volume has `{}`.

`stats` reads the derivation in `strength_bands`, `highest_leverage` and `weakest`, and is
structural in its per-kind counts, `edges`, `demoted_edges` and the edge counts of `relations`.

`stats`' `relations` and the `edges` query read the index's edge rows and no derivation: the
bearings they report are the values the rows hold, which `edit-edge` moves. `relations` reports
`walked` per relation: whether a score run's bearing pass asks about that relation's edges —
`supports`, `lifts` and `refutes` — where a `founds` bearing is the user's, written by
`add-edge` or `edit-edge`.
### `edges`

`edges` is [page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md)'s query: its node `--id` with `--direction`, its repeatable `--relation`,
its rows and their fixed order, and its refusals. Its answer is complete as [page §2.8](NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s "Lists"
states: `count` and `results`. What this release adds to the page and holds:

- Rows that share relation, source and target — demoted records replacing different relations —
  order by `replaced_relation`, in the relations' order.
- A `demoted` row's `bearing` and `manually_scored` are the ones its record keeps of the replaced
  edge ([page §1.3](NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)).
- A usage refusal is `check: usage` at the flag: `--relation` and `--direction` outside their
  vocabularies, each with it as `allowed`, an unknown relation once however often it repeats; and
  `--direction` beside no `--id`.
- An `--id` no node carries is refused `check: unknown-id` at `--id`, `detail` `no node <id>`
  ([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).
### `trace`

`trace` is [page §3.4](NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/trace.md)'s query: its required `<id>` and `--direction`, its full walk over the
scoring relations, its rows — an `edges` row, then `depth`, `strength` and `contribution` — and its
refusals. Its answer is complete, sorted and limited as [page §2.8](NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s "Lists" and "Sorting" state:
`count` and `results`. What this release adds to the page and holds:

- An edge reached by more than one path is one row, at its least depth.
- `--order` turns the named key only: ties break by ascending depth, then `edges`' order.
- `strength` and `contribution` read at two decimals, as `find`'s `strength` does; the sort reads
  the full values.
- A refusal is `check: usage` at the flag: `--direction` missing or outside `in` and `out`, with
  them as `allowed` (over MCP the schema requires `direction`, so a call missing it is refused
  as the `query` tool refuses any missing argument, §11); `--sort` and `--order` outside their vocabularies, `--limit` beside no
  `--sort`, and `--limit 0`, as `find`'s. An `<id>` no node carries is refused `check: unknown-id`
  at `<id>`, `detail` `no node <id>` ([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).
### `export-latex`

```
kbase export-latex [--out DIR] [--volume NAME]...
```

Writes a LaTeX project from the KB's current state into `--out` (default `latex-export/` beside
`kb-root/`; a relative `--out` resolves against the project root (§9)). Every KB file is read
through the one loader (§10's stamp rules apply); nothing under `kb-root/` is written; a KB whose
load fails is refused with the load error ([page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md)). `--volume`
is repeatable, each naming a volume as [page §1.6](NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md) defines one;
the named volumes export in entry-point order. A `--volume` that is not a volume is a usage
refusal, `allowed` every volume, and so is one naming a volume the entry point's list does not
name, which the export does not reach (§7). The out directory may be any directory, the author's own
LaTeX project included: kbase owns only its `gen-tex/` subdirectory and writes nothing else there
but a missing `preamble.tex`. **Manifest refusal.** A `gen-tex/` that exists holding files without
the export manifest `gen-tex/kbase-export.yaml` is refused whole, nothing written — `check:
export-dir`, remedy: empty the directory or name another `--out`.

#### The project

`gen-tex/` is kbase's and every export regenerates it: `main.tex`, the whole
export; `defaults.yaml`, the pandoc defaults file that produced it, its paths relative to itself;
`kbase-export-front.md`, the entry point's prose, and `kbase-export-vol{N}.md`, one per exported
volume in order, its documents in walk order — read in that order, each file's footnotes its own;
`kbase-export.latex`, the template;
`kbase-export.lua` and `kbase-xref.json`; `assets/`, a copy of every non-Markdown file under each
exported volume's directory less the claim-graph sheets (§3), at its `kb-root/`-relative path;
`kbase-export-findings.json`, the
filter's findings, which the export merges into the report; the report and the manifest. None of it
is for editing. **The preamble is the author's.** `main.tex` loads `preamble.tex` from the out
directory, beside `gen-tex/`, after the template's base packages and before its cross-reference
packages. Where no `preamble.tex` stands, the export writes a starter — commented, loading nothing,
advising the author to add the packages and macros their sources used; where one stands, the
export never reads, writes or removes it. Whenever a `preamble.tex` exists, the report carries a
line naming it as the author's. A built KB keeps its maths as the
sources wrote it, so a macro or package those sources' preambles supplied is the author's to add
there; LaTeX names any that is missing. The author may re-run `pandoc -d defaults.yaml` from
`gen-tex/` without kbase. Compiling `gen-tex/main.tex` is the author's step; kbase runs no TeX
engine. How the project is produced is ARCHITECTURE §5.7.

#### Content

The export walks every document the index child-list spine names, `overview.md`
included, as ordinary leaf content; `references.md` leaves are read as bibliography data, never
walked as documents. Author labels are namespaced per volume, so a label repeated across volumes
stays distinct, and LaTeX numbers what they label. A reference naming a label its volume defines
resolves to it; a cleveref-family anchor (`ref+label`, `ref+Label`) renders as one `\cref`/`\Cref`
over every label it names, one or several, without the anchor's own text beside it. A link to an
exported KB document becomes a hyperlink to that document's heading. Prose is exported as written:
a hand-maintained KB's prose mentions of equations and works (`(eq:dv)`, `(Khalil, 2002)`) and its
bold-line theorem blocks stay text. The bibliography holds `references.md`'s entries or, where a
volume has none, the KB's work records.

**Simplification, not refusal** (§3's `dot` posture; [page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md)): a reference whose label its volume does not
define, a link to a document not exported, and a citation with no bibliography entry each become
the text they carry; an image whose file is not in the KB becomes its alternative text; a figure or
table the export cannot rebuild, or one LaTeX cannot float, is written as far as it can be; and
maths the export cannot recover from the page is left out. Each carries a report line in the
export report, `gen-tex/kbase-export-report.yaml` (§7's `report` key). Idempotent: a re-issue over the tool's own previous export leaves the
export byte-identical and reports `unchanged`. The export takes no KB write lock — nothing under
`kb-root/` is written, so it reads as the queries do — and it runs no git. Result
keys: §7.

### `score`

```
kbase score [--pass rigor|bearing|all] [--node ID] [--edge SOURCE,TARGET,RELATION]
            [--rescore-manual] [--scoring-model MODEL] [--dry-run] [--resume] [--state-dir DIR]
            [--timeout DURATION] [--ask-concurrency N]
```

Its scope — every dirty item that is not manually scored, by default — its arguments, its dry run,
its result, its MCP result and its refusals are [page §3.5 `score`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s; what makes an item dirty,
the judged hash, is [page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)'s. At the command line `--edge` names its edge as
`<source>,<target>,<relation>`; over MCP `edge` is a mapping of the three. `--node` and `--edge`
on one invocation together, and an `--edge` that is not three non-empty comma-separated parts, are
usage refusals. `--ask-concurrency` under 1 is refused as `build`'s is (§5); `score`'s `resume`
does not carry it, nor `--timeout`: each shapes one invocation, not the run.

Grades the KB's rigor and bearing values by asking the scoring model: one call per item, one value per
call, each value landed through the edit ops and recorded in a ledger. `--timeout` is the deadline for one item's
call (default `30s`), counted from the call's start, never while the call waits for its turn. A run asks one call
per item and takes minutes: at a terminal `kbase score` blocks until the walk ends, and as an MCP tool
(§11) it starts a detached child and returns once the run has started, while `kbase wait`
reports its outcome when it ends. A shell caller gets the blocking behaviour a terminal gets.
There is no `--provider`: a scoring-time endpoint is named by `--scoring-model`, which states the
provider and the model in one value, and passing `--provider` is a usage refusal.

#### The two passes

The passes, what neither reaches and the scope a run walks are [page §3.5 `score`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s. `rigor` walks
claims, analyses and experiments by id; `bearing` walks `supports`, `lifts` and `refutes` edges by
relation in that order, then source, then target. A run over `all`, the default, walks the node set
and then the edge set: a text edit moves a node's judged hash and the hash of every edge that
touches it ([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)), so a run scoped to one pass leaves the other dirty against that edit. An
all-pass run is one run: one lock, one `run.yaml`, one `items.jsonl`, one `ledger.jsonl` and one
`resume` call, and its `run.yaml` names the set it walks (`all`); a run naming a node records
`rigor`, one naming an edge `bearing`. The set is read from the index once, at the run's start, and
does not move as values land. A work's rigor is judged with the document in hand and `edit-work`
writes it; an axiom's is `1.0` by rule; a `founds` edge's bearing is the reader's, made with the
work in hand, and `edit-edge` writes it. The `founds` edge carries a bearing like any other — the
values table prices it `bearing × the source's rigor` — and no pass reaches it.
#### Skipped items

A value whose record is `manually-scored: true` ([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)) is a person's judgement standing.
What `--rescore-manual` reaches is [page §3.5 `score`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s; the mark is read from the KB the run loaded
at its start. A full run without it walks a dirty manually scored item and skips it: the item is
asked nothing and lands nothing, its ledger line records status `skipped`, reason
`manually-scored`, with no value, charges or call, and so no capture. It counts under
`counts.skipped`, never under `scored` or `failures`, and does not make `resume` present. A value
`--rescore-manual` re-judges lands `false`, so later runs re-judge it too. `run.yaml` records the
flag (`rescore_manual: true`), and `--resume` continues the run with it, appending no second
`skipped` line for an item; a re-price leaves a skipped item's landed line as it is. A `--dry-run`
lists the dirty manually scored values it would skip under `skip`, counted under
`counts.skipped`.

An item whose node or edge is no longer in the KB is skipped too, once, counted the same way: its
line records status `skipped`, reason "the node or edge is no longer in the KB; its value is
discarded and a resume does not ask it again", and nothing is asked of it or landed on it.
#### One item, one quantity, one call

A call carries one node's text or one edge's — never two
subjects, never the other pass's quantity. The item's class gates the instruction as it gates the ids:
a claim is asked whether its statement is fixed, consistent and single-sensed, an analysis or an
experiment whether its method establishes what it sets out to, and neither is asked the other's
question. The text is closed to the subject: a node's title and statement, or an edge's two
statements, its authored context and the target's passage; a register page is never read. A text
carrying a scoring token (`- rigor:`, `- strength:`, `(bearing …)`, `(strength …)`) is refused
before the call, naming the document and line, and that item is `failed` with no call made.

#### The answer

A reply is a list of charge lines, each one fault id and the numbered line of the item's
own text where the fault shows — one line per occurrence, in the order written — or the word `none`,
alone. The call carries the text
wrapped at word boundaries to at most 100 columns, its own line breaks kept and nothing rewritten, and
writes `N| ` before each line lying inside a passage a charge may cite, numbering from 1: a numbered
line is a verbatim slice of that text. A frame label and an edge's `## Context` carry no number, and a
line with no number is no site.
[RIGOR_AND_BEARING_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/rigor-and-bearing/entry-point.md) states the criteria, the
three closed sets and the citation rule; this section states what kbase does with a reply.

- **The ids are the item's class's.** `rigor` on a claim accepts the statement vocabulary's four,
  `rigor` on an analysis or an experiment the method vocabulary's six, and `bearing` its five, and
  nothing else; an id outside that set — the other rigor class's among them, and `flt-pending`, which
  no table holds — is `malformed`, the refusal naming the id and the call's ids as `allowed`.
- **The citation is the numbered line where the fault shows.** The ask asks for one number, the line
  where the fault first shows, `flt-inapplicable: 12`, and never a range. The parse still reads a
  contiguous range within one passage, `flt-transfer: 12-14`; its dash may be a hyphen, an en dash, an
  em dash or a minus sign, with or without spaces. kbase resolves the
  field against the rendering the call carried — never a register page — and records three facts beside
  the `passage` the lines lie in: `lines`, the answer's own numbers; `citation`, the quoted lines
  without the gutter; and `offset`, the quote's byte offset in the carried text. A charge naming no
  line, one naming a line the call does not show, and a range crossing from one passage to another or
  over a gap — a blank line, a label or a heading between its lines — are `malformed`, naming the id
  and the lines. A numbered line lies in one passage, so the `both` value — a span the two ends
  carried alike — is no longer produced; a ledger line already recorded with it keeps the meaning it
  was written with.
- **Nothing is read out of prose.** Blank lines are ignored and any other line that is not a charge
  line is `malformed`. So is `none` beside any other line, a bare number — a decimal, a percentage, a
  fraction, a `*pending*` written as a value — since a number standing alone names no fault id, and an
  empty reply: silence is never a clean pass.
- **Duplicates and overlaps.** One id on one line set, written twice, is one charge; two line sets of
  one id that overlap without coinciding is `malformed`, naming both lines; two ids on one line set
  are both charged, the quote and its offset shared.

**The three parse rules are frozen for the life of the format version.** The citation's floor of at
least one valid line number, `none` beside charges refused, and one id over two line sets that
overlap without coinciding refused —
fixed while the ledger's format version stands, and what moves one is a new version, which `--resume`
reads by its own rules. A valid line number is one the call shows, lying in a passage a charge may
cite. A ledger's meaning is its ids, and a rule that loosens later would re-read lines already written
under a new grammar. This change moves no version: `citation` is still a span of the carried text, so
a line written before it reads as it did, and a line recording `flt-pending` still prices `*pending*`.

#### The value

kbase prices the charge list: the item's class's maximum — `1.00` under each of the
three tables — less each id's weight per occurrence under that class's table, floored at zero and
exact at two decimals, summed in integer hundredths. `none` is the empty list and
reaches the maximum; a ledger line recorded with `flt-pending` prices `*pending*` whatever the table,
though no table holds that id now. The tables are `internal/kb`'s
and no template carries a weight, a maximum or a band: the ids and the criteria are the ask's, the
price is kbase's, and nothing else is an input. A landing is therefore checkable against its own
ledger line without asking a model.

#### A refused reply is a failed attempt

A `malformed` reply writes nothing, and the attempt's line
records `status: failed` and the reason — the case, and where it names one the id and the lines — and
**no text of the reply**. The same holds for a reply cut off at the completion cap, whose charge list
may be incomplete, and for a call or a write that failed. The run goes on, and `--resume` asks the
item again. Nothing is defaulted or supplied by kbase: a link or a node is charged, answered `none`, or
left unasked, and `*pending*` is no model's answer — the edit ops are what write one, as a
person's decision.

#### A provider call's failure names what ended it

`provider-refused` — the endpoint answered
with a status that is not success: the request was carried, read and declined, and the reason
carries that status and the provider's own words beside it. `transport-failure` — the
connection, the name, the certificate or the stream failed, and no answer of the provider's was
had. `truncated-empty` — the provider answered, and the answer reached the completion cap before
any answer text: the request leaves it no room, and it is not sent again. A reason states the one that happened: the first calls for the request to be looked at, the
second for the path to the provider. A `provider-refused` 401 or 403 is the provider refusing the
call's key, and its failure item carries the remedy that stores one (§9, "A refused key").

#### Landing

A value lands through the edit ops with the mark `false` ([page §3.5 `score`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)): a rigor answer as an
entry of its node kind's op — `edit-claim`, `edit-analysis` or `edit-experiment` — with the node's
`id`, a bearing answer as an `edit-edge` entry with the edge's `source`, `target` and `relation`,
and each records the judged hash of the text it was judged against ([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)). A value whose
item's text was edited after its call was composed lands nothing: its line is `failed`, reason
"its text was edited after it was asked; the value is discarded and a resume asks it again", and a
resume asks it again. A value whose node or edge is no longer in the KB lands nothing either, and
fails nothing: its line is `skipped`, reason "the node or edge is no longer in the KB; its value is
discarded and a resume does not ask it again", so it counts under `counts.skipped`, never under `failures`, and does not make the run
resumable. A resume that finds an item of its run removed records it the same way, once, and asks
nothing of it. The run lands
every answered value at once when its walk ends: one mixed write batch, followed by the
derivation as every batch is ([page §2.3](NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)), under one hold of the KB write lock. An entry the
batch refuses is that item's `failed`, and the batch is re-issued without it; a landing that
cannot take the lock fails every item it carried: where another writer held the lock past the
wait, the item's line carries the lock item's `detail`; any other lock failure records its own
error. The run goes on.
#### The scoring role

The calls go to the `scoring` role (§9): `models.scoring`. The role is the KB's
declaration of which model judges it, so a run that names none refuses as setup — naming that key,
before the KB is opened and before the run's start refresh — and nothing falls back to the text role.
The role's model is the file's `<provider>/<model>` reference or, where the files name none, the one
`--scoring-model` names: the flag supplies an unconfigured role and overrides a configured one, its
reference is resolved against the pool, and a provider the pool does not hold refuses naming the flag
with the pool listed. A run records the model as the flag's where the flag named it. A `--dry-run`
resolves no role and refuses nothing for one.

#### One item, one call

The ask is one tool-less chat call on the `scoring` role's endpoint, over the
chat protocol that endpoint's entry names (§9.1): `openai` speaks `chat/completions` and `anthropic`
speaks the Messages protocol. Every ask a run sends to an `openai` endpoint carries the run's id as
its `prompt_cache_key`, so a run's asks reach one provider and share one cache entry; the Messages
protocol has no such field, so a run whose endpoint speaks it sends no key. Messages requires a
completion cap, and no other request is capped: the answer's allowance is `max(256, ceil(characters / 4))` over the
characters of the ask's messages and system text — four characters per token because the ask is plain
prose and no LaTeX is ever sent. The call carries the scoring role's sampling settings (§9.2) in its
protocol's fields (§9.1), and thinking text is never part of a reply on either protocol. A call
that does not complete is re-issued identically up to the transport attempt count, pausing 5 s then
30 s between attempts (ARCHITECTURE §12). What fails the item at once, on the one attempt that met
it, is a provider's answer carrying a status that is not retryable — any non-2xx other than 429 and
the 5xx class — an answer that reached the completion cap before any answer text, which an
identical re-send would cut off the same way, and a reply the parse refuses. `--resume` is how a user asks a failed item again.

#### The ledger and resume

A run's records stand under `<state-dir>/score/<run>/`: `run.yaml`,
`items.jsonl` (the set the run enumerated, one pending line per item, so a reader counts the whole
run), `ledger.jsonl`, `captures.jsonl` and, while a walk runs, one result file per answered item, which
the walk's end folds into the ledger. A real run's id is `run-<id>`; the runs kept are §6's.
`run.yaml` holds the run, its KB root, the ledger's format, the set it walks
(one pass, or `all`), `rescore_manual` (`true` where the run re-judges manually scored values,
absent otherwise), `backend` (`provider`, which a header written under an earlier release may
read `environment`; absent under `--dry-run`, which resolves no role because it asks nothing), the
provider, the model, `ask_sha256` and the start. **It is
written once, at the run's creation, and no resume rewrites it**: a resumed or mixed run's header
states the first invocation's back end and model, and each ledger line states its own item's pass and
what answered it.

What `--resume` continues and when it is refused are [page §3.5 `score`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s. The run it continues
is the store's newest real run by its `run.yaml` start — a dry run is never one; that run
finished where no item of it is `failed` or unreached; and a build has started since it where the
store's progress record names a build invocation that started later. A resume walks the run's own
`items.jsonl`, the set it enumerated, and asks again each item that is not `done`, under the pass
and `rescore_manual` its `run.yaml` records.

`ledger.jsonl` is one JSON object per attempt — the item, its status, the value, the value the KB
carried before the attempt (`before`, absent where the record held none), the reply, `faults`
(the parsed charges: `id`, `passage`, `lines`, `citation`, `offset`, in the order written), `moot` (the charge
lines a reply recorded beside a retired `flt-pending`), `backend` (as the header's; a line written
under an earlier release may read `environment`), the model, the prompt's digest, `remedy` (on a
failed attempt whose call the provider refused for its key, the command that stores one, §9;
absent otherwise, and absent from every line written before the field, which reads under the same
ledger format), `note` (beside a `remedy`, the note on how the key reaches the store, §9; absent
otherwise), `judged` (the judged hash of the item's text as the call was composed, [page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md);
absent from every line written before the field) and the attempt's bounds —
appended after the item's write and its validation, never before. A resume re-prices only a line
that records `judged`; one written before the field stands as it is. A run writes its captures and then
its ledger lines once, in the set's order, when its walk ends — on completion, on cancellation and
on a failure that stops it. `faults` is `[]` for one reply
alone, the word `none`; a failed attempt carries neither `faults` nor text. The state directory is
`build`'s:
`$XDG_STATE_HOME/kbase/<key>/`, or `--state-dir`, and one inside `kb-root/` is refused.

`captures.jsonl` is one JSON object per call made — the item, its class (`statement`, `method` or
`bearing`), the closed text whole with the line each number names, the prompt sent and its digest,
the reply the call returned (`""` where the call failed), the error that failed the attempt where
one did — the call's, the parse's or the write's — the model, the back end and the call's bounds.
Every attempt that makes a call has one, whatever its outcome: a landing, a refused reply, a failed
call or a refused write. A failed attempt's reply therefore stands whole there, where its ledger
line keeps no text of it. An item the guard refuses makes no call and has no capture, and a call
cancelled in flight leaves no capture and no ledger line. A run's captures are appended before its
ledger lines, at the walk's end, and a resume appends to the same file, and a capture that cannot be written fails the run
as a ledger line that cannot be does.

#### `ask_sha256` records the ask set and gates nothing

It is one digest over every instruction the
run can send, each rendered for an empty subject and concatenated in a fixed order — a rigor run's
statement instruction and then its method one, a bearing run's one, an all-pass run's statement, method
and then bearing instruction — so every fixed byte of a run's
instructions is certified and nothing that varies with an item is. Each ledger line carries its own
item's prompt digest beside it, which is what names the variant that judged that item. A header
written before the rigor pair carries one render's digest and is read as the one ask that run began
under. `--resume` continues a run: it refuses a ledger for another KB root or format, first lands and records every answer a
stopped invocation left in a result file — a value a person has set since being left as it
stands — then skips every item the ledger records `done`, and re-asks the `failed` ones. A run resumed after an update
continues and asks nothing about versions: one whose `ask_sha256` differs, or
which carries none as every run written before the field existed does, is asked under the running
template's ask, and the header keeps the digest the run began under. Where the running table for a
landed line's class prices its recorded ids differently, the resume re-derives it, lands the moved
value through the same write path a landing uses — before the line records it — and leaves
`repriced.yaml` in the run's directory naming each line that moved, the value it held and the value it
took. An id that line's class's table no longer holds charges nothing, so a re-price can move a value
up. A re-derivation that moves nothing records nothing and lands nothing. A run given no `--resume`
is a new run over its own scope; what is dirty is the judged hash's to say ([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)), and the
ledger of an earlier run is no evidence about it.

#### The run's locks

A scoring run takes the KB's operation lock and the state store's holder lock
for its length, in the order a build takes them, before it writes anything and before its start
refresh: a second run over the KB — a build or a scoring run — is `refused` at once, `check: lock`,
`path` the holder's state-dir, its `detail` naming the holder's verb and pid (§6), never held through
a wait the first run will outlast. Its holder record names `kind: score` and the run id, which is
what `status`, `cancel` and `wait` read (§6).

#### Refresh and format

A KB whose load fails ([page §2.4](NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)) is refused before the run's start refresh, naming each fault. A KB
in an older format is refused before it too, dry run or not, while its conversion is pending
(`check: index`, remedy `kbase update-format`, [page §2.6](NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)); its document carries `migration`. One
refresh opens a run that asks, before the index is read, so the derived files stand current before
the walk; a dry run writes nothing. Each landing is followed by the derivation inside its batch
([page §2.3](NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)), so the derived files are current whenever the run exits — finished, failed or
cancelled. The refresh holds the KB write lock and releases it before the walk; one that cannot
take the lock ends the run there, with the lock's own outcome — `retry` where another writer held
it past the wait — and the `lock` item, whose remedy is `re-run kbase score`. A crash inside a
landing's write can leave a stale index, which `verify` reports and the next run's start refresh
repairs.
#### Outcomes

`done` when the run finished; `cancelled` when SIGINT or SIGTERM stopped it, with no line recorded
for an item whose call was in flight or not yet made, and a line for every item answered before
the stop; `failed` when an item's call or write failed, or the start refresh did; `retry` while
another run holds the state store, or where the start refresh could not take the KB write lock past
the wait; `refused` where the run cannot go on — [page §3.5 `score`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s refusals, and besides them
a `--state-dir` inside `kb-root/`, `--node` beside `--edge`, an `--edge` that names no edge, and an
index the load cannot answer.
#### The closing record

A `failed` run closes on `failures`. Where the walk itself failed, it holds one
`check: error` item per item the ledger records `failed`, `detail` the item as the document names it
and the reason its line recorded — `<node id>: <reason>`, or `<relation> <source> → <target>:
<reason>` — in the walk's order, with the `remedy` the line records where it records one and the
line's `note`, where it records one, ending the `detail` before the outcome's ask (§7); the reason is the line's reply as a
bounded, control-stripped summary, the ledger's line being the one place a reply stands whole; a run the start refresh stopped closes
on one `check: error` item naming the refresh's own error. A `cancelled` run closes on `cancelled`,
whose `item` is the walk's first item carrying no attempt, and `{}` where
every item of the walk carries one. A `refused` or `retry` run closes on its `refusals`.

**`resume`** carries the call that continues the run — `score` with `--resume` — present wherever
an item failed or was not reached; a `skipped` item is no such item. It stands after `counts` and
`scored` and before the closing record. It is absent from a dry run and from a stop before the
walk: the start refresh's, the ledger's or the index load's. It names no argument of the scope —
the run's `run.yaml` records it — and carries `--state-dir`, `--config-dir` wherever the
invocation named one — the resumed run must read that directory and no other, or the same run
could resolve another reference — `--project-dir` wherever the invocation named one, and
`--scoring-model` whenever the invocation did — the call must reach the same model, or it is a
different judgement under the same run id. A run started under an earlier release whose header
reads `backend: environment` is resumed on whatever the files now resolve, and its new lines
record that.

**`--dry-run`** is [page §3.5 `score`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s. It writes `run.yaml` and `items.jsonl` (one pending line
per item of its set — the items it would ask and the dirty manually scored ones it would skip),
dials no provider, resolves no role, and writes no KB file. Its document lists `ask`, every item
the run would ask, and `skip`, every dirty manually scored item it would skip, each as `scored`
names an item and in the walk's order, after `counts`: `total` the two together, `remaining` the
items to ask, `skipped` the items to skip, `done` `0`. Its `scored` is `[]`, it carries no `resume`
and no `roles`, and its outcome is `done`. It writes no charge record and no capture. Its run id
is `dry-<id>`, and the runs kept are counted by kind (§6), so a dry run counts only against dry
runs. `--resume` with `--dry-run` is a usage refusal naming `--resume`. A dry run on an older KB is
refused as a real run is (`check: index`, remedy `kbase update-format`).

Result keys: §7.
### `probe`

```
kbase probe [--model <provider>/<model>] [--decision | --text | --scoring]
            [--prompt TEXT] [--timeout DURATION]
```

One ask, no KB and no state: a diagnostic of a model selection, answered by the selection itself.
A probe asks one selection, and its flags name it: `--model` names a `<provider>/<model>` reference,
resolved against the pool as a role's reference is; `--decision`, `--text` and `--scoring` each ask
the selection the configuration resolves for that role — the `models.<role>` reference — and supply
no model of their own; and with none of the four given the probe asks the `scoring` role, the
selection a scoring run would make, so `kbase probe` with no arguments asks the selection a run
would use. The four are mutually exclusive: more than one is a usage refusal naming all four as
what may be chosen once, before anything resolves or dials. A selection that does not resolve is
`refused`, naming the role's own key — `models.<role>` — or the flag, and what is missing, before
any client is opened. The document states which selection answered: the role's name where a role flag
or the default named it, and the reference where `--model` did.

A chat selection is one tool-less chat request through the registry a scoring run's asks take, so
an endpoint is reached over the wire shape its provider names: no tools, no history, an empty
system prompt, and `--prompt` in place of the default one-word check. It carries its role's sampling
settings (§9.2) — the `scoring` role's where `--model` names the selection — and the document
reports the effective settings, `temperature` and `thinking`. A `--decision` selection is
one decision call on the surface its provider's `decisionsFlavor` names, with no system message, and
its `--prompt` in place of the default predicate. `--timeout` bounds the call (default `30s`); a
call that does not complete is re-issued at once up to the attempt count, so that deadline is spent
on attempts rather than on pauses (§12). The document reports what a ledger cannot — the selection,
the provider, the model, the time to the first token, the call's wall time, the finish reason, the
reply, and the usage the provider reported — and its outcome follows the call: `done` where it
completed, or where a chat answer was cut off at the completion cap before any answer text
(`finish-reason` `length`, `reply` empty); `failed` otherwise, with the attempt's own outcome and the
provider's words in the failure, and its `remedy` where the provider refused the call's key (§9). It reads no KB and reads or writes no state directory: it prints its document and exits.

`probe` is also a read-only MCP tool taking the same arguments ([page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/probe.md); §11). Each call is
a paid model call, and its MCP note is the one [page §4.2](NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) quotes.

Result keys: §7.

### `update-format`

[Page §3.6 `update-format`](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) is the op: run alone over a clean `kb-root/` and clean build records the
conversion moves (`check: dirty-paths`, naming each path), it converts a KB in an older format (§10) and, whether or not the
format changed, re-derives every register page and claim-graph sheet (§3) and rewrites the
maintained blocks ([page §1.7](NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)). While a conversion is pending every other write is refused
([page §2.6](NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)).

What this release adds to the page and holds:

- It writes each file only where its bytes change, and removes a page or a sheet the KB no longer
  calls for. Without `dot`, §3's rule.
- A KB whose load fails is refused before anything is written, and `update-format` repairs no load
  error ([page §2.4](NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).
- Without `commit` (`--commit`) `commits` is `[]` and the `git mv` of each build record into
  `kb-root/.build/` stands staged. With it, a run that writes or removes anything makes [page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)'s
  one commit, labelled `kb-write: update-format <from> → <to>` (the format before and after, the
  same version twice where it did not change), holding `kb-root/` and each moved record's root
  path; an `unchanged` run commits nothing. A commit that fails leaves the conversion written,
  outcome `done`, with `uncommitted` saying why, as a write batch's does ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)).
- The move is made once the KB loads, before the first write, so a KB whose load fails moves no
  record. Each moved record is reported twice: its root path, repository-relative, in `removed`,
  and its new path, kb-root-relative, in `written`.
- `ids`, `bare-ids`, `no-edge` and `strengthen-by` are listed only on `done` and `unchanged`. On a
  refusal or a failure, their warnings stay in `migration.warnings` and on stderr.

Result keys: §7.
### `verify`

[Page §3.6 `verify`](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md) is the contract: its
faults and their check names, their remedies, the `demoted` findings and the result. What this
release adds to the page and holds:

- The dead-link gate resolves each link target against the repository; a target leaving the
  repository is no fault. Which fragments are live is the page's rule; a bare `#fragment` reaches
  its own document, an element's `id` counts outside code only, a summary page's entry anchors are
  those derived for it, never the page as it stands, and an empty fragment names the document's
  top. A fragment into a file outside `kb-root/`, or into one that is not Markdown, is not checked.
- A summary page's `register-freshness` remedy is the page's empty write batch, except over a KB in an
  older format, where that batch is refused until the conversion is saved, it is
  `kbase update-format`, which re-derives every page too.
- `unknown-id`'s prose ids are read outside code, of any kind.
- `bare-id` reads every document but the register pages, outside code, statement markers and link
  titles and destinations. Its remedy is a write batch of one `add-citation` entry: the `id`, the
  `document`, the `line` and the id as `snippet`.
- `frontmatter`'s remedy is to remove the block; the entry point is checked as every document is.
- `statement-marker` covers a claim, an analysis or an experiment. An `axiom-heading` marked heading
  is one at any depth whose line carries an axiom's marker.
- A KB whose load fails is refused with the load error ([page §2.4](NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).
- `findings` is present on `done` and on `refused`.
- The build's gate stage runs this same check.

Result keys: §7.
### `status`, `cancel`

See §6.

---

## 9. Configuration

Two TOML files — `providers.toml` (§9.1, the endpoint pool) and `config.toml` (§9.2, the choices) —
are read under the strict-load contract (§9.3), in one or two layers.
`<repository>/.kbase/` is read over `~/.config/kbase`; a directory `--config-dir` names, else
`$KBASE_CONFIG_DIR`, is instead the one directory read, so an explicit choice isolates a run from a
repository's own `.kbase/`. An invocation outside a worktree has no repository layer:
`show-config`, `models`, `configure-providers` and `configure-models` run in any directory;
`configure-providers` and `configure-models` write where [page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md) and "The configure commands'
write location" below state.

### Merging

The two files merge by two rules. `providers.toml` merges by provider
name and a provider's table is replaced whole: a repository table of the same name supersedes the global one
entirely, never field by field, so a repository cannot inherit the global table's `apiKeyFile` for an
endpoint it chose; a stored key belongs to the provider name, whatever layer supplies the table.
A provider's name is folded to lowercase as it is read ([page §2.7](NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)), so a repository table
overrides the global table of the same name in any case; two tables of one file whose names differ
only by case refuse the load, naming both spellings and the file. A fault naming a provider a higher layer superseded is
not reported: the pool's provider is the higher one, and a diagnostic about a value nobody reads is not a
fact about this run. `config.toml` merges field by field: a value a higher layer states wins, and one
it leaves out keeps the lower layer's value. A layer that fails the strict load refuses the whole
load, naming its file: no lower layer is read, so a broken repository file never silently falls
through to the global one.

### A leading `~` expands; nothing else does

In the `--config-dir` and `$KBASE_CONFIG_DIR` values,
and in an entry's `apiKeyFile`, `~` alone is the home directory and `~`
followed by a path separator is a path beneath it. `~user` is not supported, a `~` anywhere else is
literal, and where the home lookup fails or its answer is not absolute, the string stays exactly as
written.

### Global flags

Every subcommand accepts `--config-dir DIR`, `--log-level debug|info|warn|error`
(default `warn`), `--log-file PATH` (tees diagnostics to a file; console output is never redirected
away) and `-h`/`--help`; the root command also accepts `-v`/`--version`, which prints exactly the
version string and a newline. A directory `--config-dir` or `$KBASE_CONFIG_DIR` names is the only
one read, the flag before the variable; otherwise configuration is read from the two layers above.
`--project-dir PATH` (the project root: the git worktree root that holds `kb-root/` or will; the
subcommand's search for the repository starts there instead of the working directory; a path that
is no git worktree root is refused, `check: worktree`, `key` the `project-dir` argument, spelled as
[§7](#item)'s `key` states — an MCP call's `project-dir` refused so too — `path` the absolute
path)

A relative path a subcommand takes — a volume root, `--bibliography`, `--state-dir`, `export-latex
--out`, `--values` — resolves against the project root; `--config-dir` and `--log-file` resolve
against the working directory, whatever `--project-dir` names, and a printed `resume` writes
`--config-dir` absolute.

### Provider commands

`kbase models [--provider NAME] [--timeout DURATION]` lists the selected
provider's model identifiers, sorted ascending, in its result document (§7; [page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/models.md)).
Every input naming a provider — `--provider`, a model reference's provider part, `auth add` and
`auth remove` — is folded to lowercase, and every output names it so ([page §2.7](NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)).
`kbase configure-providers` and `kbase configure-models` write `providers.toml` and `config.toml` as
[page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-providers.md) and its [`configure-models`](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-models.md) leaf state, where they write as "The configure commands' write
location" states, and under §9.5. What the page leaves to this implementation:

- `configure-providers` refuses, `check: usage`: a write with no `--base-url`; a `--base-url`
  carrying credentials (`user:key@` or `key@`), the `kbase auth add` pipe as its remedy,
  its detail ending as a refused key's does ([§9](#a-refused-key), "A refused key"), checked
  before any other fault of it; a `--base-url` that is no absolute `http` or `https` URL; no
  refusal of `--base-url` names any part of it. Also refused: an `--api` other than `anthropic` or `openai`, or a
  `--decisions-flavor` other than `systemone` or `decisions`, each with those values as `allowed`;
  and `--remove` beside an entry field. A provider name breaking the provider-name rule (§9.1) is
  refused keyed `<provider>`. `--remove` of a provider the file holds no entry for is refused
  `check: provider`, `path` that `providers.toml`, detail ``no provider `<name>` in <path>``, and
  `kbase show-config` as the remedy.
- `configure-providers` dials where the written entry has a key — stored, or read from the file
  its `apiKeyFile` names — under a 30 s deadline, and gives the catalogue, sorted ascending, as
  `models`. With no key, `next` is two commands: the `kbase auth add` pipe, the provider's name
  filled in, then `kbase models --provider <name>`. Where the dial fails, `next` is one entry
  naming the failure, then the `configure-providers` call for a wrong URL, the `kbase auth add`
  pipe for a wrong key, and `kbase models --provider <name>`; where the stack written
  into does not reload, one entry naming that and `kbase show-config`; where the written entry
  does not load, one entry naming its fault's reason and `kbase show-config`.
- `configure-providers --remove` gives `next` as page `configure-providers` "Result" states, where
  neither the layer written nor a layer beneath it still holds the provider: one entry per role
  whose reference names it, giving the role, the reference and the `configure-models` call,
  carrying the removal's location flag, that replaces it; then, where a key file is stored under
  the name, one entry giving `kbase auth remove <name>`, saying where the file does not open. With
  nothing to list there is no `next`.
- `configure-providers` and `configure-models` read only the stack they write into: a layer
  outside it that fails the strict load does not stop them. Each reports every fault of that
  stack as an item of `warnings`, as `kbase models` reports them — `configure-providers` on every
  outcome from its write or removal on, `configure-models` on every outcome once the stack loads.
- `configure-models` folds its provider part to lowercase ([page §2.7](NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)) and writes it so. It refuses a value that is no reference and a provider
  the written layer's stack does not hold — the layer written and the layers beneath it: the
  global layer beneath the repository one, nothing beneath any other — each keyed by its role's
  flag, every one at once, before any dial. The latter's `path` is the written layer's
  `providers.toml`, its remedy the `configure-providers <provider> --base-url <url>` call carrying
  the location flag the call carried, then `kbase show-config`; a provider dropped from that stack
  by a fault keeps the fault's reason, as "Provider selection" below states. A run naming no role
  is a usage refusal. A catalogue that
  cannot be listed is `failed`, nothing written; a model missing from its catalogue is an item
  of `warnings`, its `key` the role's flag, and a line on stderr. `models` is the three roles the
  written file holds, null where it holds none.

`kbase show-config [--provider NAME]` reports how the configuration
resolves and changes nothing (§9.6). `kbase probe [--model REF] [--decision | --text | --scoring]
[--prompt TEXT] [--timeout DURATION]`, also the `probe` tool, asks one selection — a role flag's
role, else the `scoring` role, or the reference `--model` names — one question and reports the
call, reading no KB and writing no state ([§8](#probe)). Each writes its result under §7.

`kbase auth add <provider>` stores the API key piped on its stdin for `<provider>`, a
`providers.toml` entry name, replacing any key stored for it: one file per provider,
`$XDG_STATE_HOME/kbase/auth/<provider>-key.enc` (`~/.local/state` where the variable is unset or not an absolute path),
the file named for `<provider>` in lowercase, each file encrypted under an encryption key derived
for the OS user — obfuscation against a reader of the file, not a boundary against that user — and
readable by its owner alone. `add` reads the configuration as the provider verbs do, and a
configuration that does not load refuses as it does for them; a name, folded to lowercase, that no
`providers.toml` entry carries — in the pool or dropped from it by a fault — is refused,
`check: usage`, `key` the folded name, detail ``no provider `<name>` in providers.toml (names match in any case)``, `remedy: kbase show-config`. The key arrives on stdin and
no other way: stdin that is a terminal is refused, as is input of more than 65536 bytes, input that
is empty once surrounding whitespace is trimmed, and a key that, so trimmed, still holds a
whitespace or control character, each `check: usage`, naming no part of the input, with the piped
form, the provider's name filled in, as its `remedy`. A refusal writes nothing, and the name refusals come before stdin is read. `kbase auth remove <provider>` clears the stored key; where none is stored it is refused
([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)), `check: usage`, `key` the folded name, detail ``no key stored for provider `<name>` ``,
`remedy: kbase auth list`. `kbase auth list` names the providers with a stored key, in lowercase; neither reads the
configuration, so a key file whose entry is gone is still removed. A name that breaks the
provider-name rule (§9.1) is a usage refusal, `key` the name as given and the detail the rule,
never the name; its `remedy` is `kbase show-config` on `add`, `kbase auth list` on `remove`. A
refusal of the argument count names no argument given. No result names a stored key.

### A refused key

Nothing checks a key before a call. A provider that answers a call 401 or 403
has refused the call's key — a missing key and a wrong one alike — and the call fails as any
provider refusal does (§8, `provider-refused`). Its failure item carries `remedy`
`<command-that-prints-the-key> | kbase auth add <provider>`, `<provider>` the provider the call
dialled: the user runs it, or an agent runs it with the command the user names or `show-config`'s
`upgrade` gives; its `detail` ends, before the outcome's ask, by saying so, and that no one asks
for the key, so it never enters a conversation. The call
failures that carry it are a build's — the `inference-failed` stop of a claim-graph stage and the
overview's `inference` failure — a scoring run's `failures` (from the attempt's ledger line, §8),
`probe`'s failure, and `models`' and `configure-models`' failure to list a catalogue. A call ended by any
other status carries no such remedy, and neither does `configure-providers`' dial, whose failure is
its `next` ([§9](#provider-commands), "Provider commands").

### Provider selection is the catalogue verbs' alone

`models` selects a provider to
dial — not a role: a role is never chosen this way, and `config.toml` names no provider. The rule is
`--provider`, else the sole provider. Where no provider is named — zero usable providers, two or more with none
selected, or a `--provider` the pool does not hold — `models` refuses. An empty pool,
and a name the pool does not hold — here, or in a role's reference — point at the
`providers.toml` `configure-providers` writes into with no location flag ([§9](#the-configure-commands-write-location), "The
configure commands' write location"), the file a provider's table belongs in; `configure-models`' value
points instead at the `providers.toml` of the layer it writes, checked against that layer's stack
([Provider commands](#provider-commands)). The name the pool does not hold
also carries `key` the argument or `models.<role>` that named it, a detail naming the table to add
there — `configure-models`' says instead that its flag names a provider no `providers.toml`
this call reads holds — and a remedy naming `kbase configure-providers <name> --base-url <url>`, `configure-models`'
carrying its location flag, then `kbase show-config`. Where that provider faulted out of the pool, the fault's own reason stands
instead, with `kbase show-config` alone as the remedy. Two or more providers with none chosen list every provider name as `allowed` with
`key` the `provider` argument. The argument, and each command a remedy names, are spelled as
[§7](#item)'s `key` and `remedy` state. `show-config` reports any of these in its document and still exits 0.

### The configure commands' write location

`configure-providers` and `configure-models` write where [page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-providers.md) states: `--global`, the default,
or `--local`, each refused beside `--config-dir`. What the page leaves open:

- With neither flag, a command writes the directory `--config-dir` or `$KBASE_CONFIG_DIR` names,
  whether or not it exists, the flag before the variable; else `~/.config/kbase`, even where a
  repository's `.kbase/` stands. An explicit `--global` or `--local` outranks the variable.
- `--global` and `--local` together are a usage refusal keyed `--local`; either beside
  `--config-dir` is a usage refusal keyed by that flag. `--local` outside a worktree is refused with
  `check: worktree`, `path` the directory the search started from, and `git init` as the remedy.
- `configure-providers`' `layer` names the layer written: `global`, `repository` (`--local`),
  `explicit` or `environment`.
- `configure-providers` creates `providers.toml` where absent holding its comment header, an example
  entry commented out, and the entry written, and no other entry. `configure-models` creates
  `config.toml` where absent from its commented template. Each otherwise edits the file under
  §9.5. A refusal writes nothing, and so does a `configure-models` catalogue failure.
### The files are the one source of inference

The endpoint and the model each role asks come from
`providers.toml` and `config.toml` alone, and so does how a role is asked — `[asks]` and `[sampling]`
(§9.2). An entry's key is the one the store (`kbase auth add`, above) holds under its name, whatever
layer supplies the entry; else the one in the file its `apiKeyFile` names, which an entry with a
stored key never reads; else none. Every verb that reads the configuration reads the store. A stored
key file that does not open is treated as no stored key — the entry's `apiKeyFile`, else none — and
is reported by its path and what is wrong with it, never its content: a diagnostic in `show-config`
(§9.6), an item of `warnings` and a line on stderr in `models`, `configure-models` and `configure-providers`, and a warning line
on stderr for every other verb. A store directory that stands and does
not list holds no stored key for any entry, and is reported the same way by its path and what went
wrong; it does not stop the configuration loading. No environment variable supplies an
endpoint, a model or a key; `$XDG_STATE_HOME` chooses where the store is, never a key. A run isolated
from the user's layers names its directory with `--config-dir` or `$KBASE_CONFIG_DIR`, which does
not isolate it from the user's store: `$XDG_STATE_HOME` does. The inference
preflight (§5) names each lacking setting as `baseUrl` or `models.<role>` — `models.decision`,
`models.text`, `models.scoring` — and `show-config` reports that refusal as a diagnostic rather than
stopping (§9.6).

There is no plaintext-transport switch: when a key is configured and the base URL is `http://` to a
host other than loopback, one line on stderr says the key travels in cleartext, and the build
proceeds (as §9.4's permission warning does).

### 9.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses within the provider-name rule:
each matches `^[A-Za-z0-9_][A-Za-z0-9_-]*$`, and an entry whose name does not fails the file's
load, naming the file and the entry. A name is case-insensitive and read in lowercase, and two names
of one file that differ only by case are refused ([page §2.7](NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md); §9, "Merging"). An
entry:

```toml
[<name>]
baseUrl = "..."        # required
apiKeyFile = "..."     # path to a key file, resolved relative to this file's
                        # own directory if relative; an entry's only key field
type = "inference"     # optional; the only accepted value, also the default
api = "anthropic"      # optional; "anthropic" (also the default) or "openai" —
                       # the chat protocol this server speaks
decisionsFlavor = "systemone"  # optional; "systemone" (also the default) or
                               # "decisions" — the decision surface this
                               # server speaks
```

No other key is recognized inside an entry. An entry that fails validation (empty `baseUrl`; a
`baseUrl` carrying credentials (`user:key@`), or one that is no absolute `http` or `https` URL
with a host, its reason naming no part of the URL; or `type`/`api`/`decisionsFlavor` set to
anything but its accepted values) is **dropped** with a
warning naming the entry and the reason (never key material); the rest of the pool still loads. A
key-file read failure faults just that entry the same way. An absent `providers.toml` is an empty
pool, not a load error.

#### The chat protocol

`api` names the wire protocol chat uses, chosen per server; it is a two-value
field. `anthropic` is the Anthropic Messages surface, `POST <baseUrl>/messages` — what an entry
silent on `api` gets — and `openai` is the OpenAI-compatible `chat/completions` surface. The two
differ in more than a path. Over the OpenAI-compatible protocol a thinking level is an effort hint
that the server may exceed without bound; a Messages request carries `budget_tokens` and a
`max_tokens` ceiling, and the ceiling bounds thinking even where a server overruns the budget.
Messages requires a `max_tokens` cap: kbase's allowance for the answer is
`max(256, ceil(characters / 4))` over the ask's messages and system text (§8), and a role
whose `thinking` is not `off` (§9.2) adds its thinking budget — 1,024 tokens for `low`, 4,096
for `medium`, 16,384 for `high` — sent as `thinking: {type: enabled, budget_tokens: <budget>}`;
thinking `off` is sent as `thinking: {type: disabled}`, the field's absence leaving it at the
server's default. Messages takes the system prompt as a top-level `system` string rather
than a turn, so a system message is lifted out of `messages`. `temperature` is the role's, 0
where unset, and is sent whatever the thinking level. Anthropic's own endpoint rejects a
temperature beside thinking. It has no
`prompt_cache_key`, `chat_template_kwargs`, `reasoning` or `reasoning_effort` field, and
none is sent. This mapping targets servers that implement the Messages protocol — oMLX,
LiteLLM-style proxies, OpenRouter; Anthropic's own newer models may reject `budget_tokens`,
`temperature` or `thinking: {type: disabled}`. An `openai` request carries the role's
`temperature`, 0 where unset; thinking `off` sends
`chat_template_kwargs: {enable_thinking: false}` and `reasoning: {enabled: false}`, and a
level sends it three ways at once — `reasoning_effort: <level>`,
`reasoning: {effort: <level>}` and `chat_template_kwargs: {enable_thinking: true}` — since a
server reads whichever of them it speaks. A Messages entry's
catalogue is `GET <baseUrl>/models`, whose `created_at` is RFC 3339. `decisionsFlavor` is unaffected
by `api`, and the decision surfaces are OpenAI-shaped: the `decision` role's multichoice asks
always use the OpenAI-shaped decision surface on its entry's base URL, authenticated as an `openai`
entry is, whatever the entry's `api`. A server that speaks only Messages has no such surface, and the
decision call fails with that server's HTTP status.

#### The decision surface

`decisionsFlavor` names the wire shape the decision call uses, chosen per
server: it changes nothing about chat, which `api` alone decides, and it binds
no model id — the decision role's model id stays in `config.toml` (§9.2). An entry silent on it speaks
`systemone`, the shape kbase's multichoice asks were written against. `systemone`: `POST
<baseUrl>/systemone`, body `model`, `state` and `questions` — an object keyed by each question's own
key — answered by an `answers` object keyed the same way. `decisions`: `POST <baseUrl>/decisions`,
body `model`, `input` (the same composed state) and `questions` — an array of `type`, `name`,
`instructions` and `choices`, where a `noul` question is asked as a `predicate` and a `choice`
question's criteria are its `choices`, each option a `value` with its guidance as the `description`,
sorted by value — answered by an `answers` array, each entry carrying the `name` it answers: a
`predicate` (a probability, read as P(true)), a `choice` (its choice, confidence and per-value
probabilities), a `score`, or a `refusal` (a declined answer, carrying no value, which leaves the
question unanswered). An answer naming a question the request did not ask is refused under either
shape; a question left unanswered is the seam's own re-ask-then-default path.

### 9.2 `config.toml` — the choices

```toml
[models]
decision = "<entry>/<id>"      # the claim graph's multichoice asks: a providers.toml
                               # entry name, then the model that entry's
                               # decision surface serves
text = "<entry>/<id>"          # the overview passage
scoring = "<entry>/<id>"       # kbase score's asks; required for score

[asks]
concurrency = 4                # optional; model calls one build or score has in flight at once, across every role

[sampling.text]                # optional; the overview passage's sampling
temperature = 0.0              # optional; a number
thinking = "off"               # optional; off | low | medium | high
[sampling.scoring]             # optional; kbase score's asks' sampling, the same two keys
```

**A `[models]` value is a reference**, `<provider>/<model>`, split at the **first** `/`: a provider
name is a `providers.toml` table's and carries none, while a model id may (`openrouter/anthropic/…`).
Either part empty, or no `/` at all, is not a reference and is refused at resolution, naming
`models.<role>` and the form. Nothing is normalised but whitespace and the provider part, folded to
lowercase ([page §2.7](NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)): a model id is the provider's spelling, not kbase's.

A role left unset resolves to "not configured" — no defaulting, no guessing, and no role falls
back to another. `concurrency` is how many model calls one invocation of `build` or `score` has in
flight at once, across every role; absent, it is 4, and `--ask-concurrency` overrides it for that
invocation. A value under 1 fails the load, naming `asks.concurrency`. Calls start in the order the
invocation submits them. There is no `[dev]` table, top-level `provider` key or `[asks]
readerConcurrency`: a file stating any of them fails strict load (§9.3) naming the key, and the
remedy is a hand edit. A
`[models]` table spelling `heavy`/`light` — the superseded role names — fails strict load as
unmodeled keys (§9.3); the remedy is a hand edit.

#### Sampling is per chat role

`[sampling.text]` and `[sampling.scoring]` each take
`temperature` and `thinking`, both optional. A key left unset is kbase's default —
temperature 0 and thinking off — never the provider's. They merge field by field like every
`config.toml` value, each keeping the layer that supplied it. There is no
`[sampling.decision]`: the decision role's closed-set probabilities take no sampling, and a
`sampling.decision` key fails strict load as unmodeled (§9.3). A `thinking` value other than
the four fails the load, naming `sampling.<role>.thinking` and the four. Resolving a role
refuses, naming `sampling.<role>.temperature`, a temperature outside the range its entry's
chat protocol accepts — 0 to 2 for `openai`, 0 to 1 for `anthropic`.

### 9.3 Strict-load contract

Both files are decoded strictly: any key or table not modeled by the schemas above fails the **whole
file's** load. The message names the file path and every offending key by its full dotted path (e.g.
`models.typo_key` — a superseded role name such as `models.heavy` fails the same way),
pluralizing "unknown key"/"unknown keys" when more than one. A malformed-TOML file fails with a
parser-detail message naming the file and line. Only key **names** ever appear in either message,
never values: a file may still hold a secret a hand edit left behind — a field no longer modeled, a
stray value — and the refusal that names its key is exactly where that value would be echoed.
Neither configure command can repair a configuration file it cannot load: the verbs that read configuration load
it before doing anything else. A layer that supplies a provider supplies its `apiKeyFile` path too,
and kbase confines no key file's location; and a stored key goes to the endpoint of whichever table
carries its name, in whatever layer. Trusting a cloned configuration means trusting the key file it
names, and the endpoint it would send a stored key to, as much as the endpoint it names.

### 9.4 Key-file permission warning

When a `providers.toml` entry sets `apiKeyFile`, the store holds no key under its name, and the host
is not Windows, kbase stats the resolved key-file path. If its permission bits grant group-read or other-read, it prints a warning
naming the key file by its path and as the one `providers.toml` names, never by the field that
states it, with the offending mode and recommending `chmod 600`. The entry stays usable. A key file
that cannot be read (§9.1) is named the same way, with the refused-key remedy (§9, "A refused key").
This is the only file-permission behaviour kbase applies to a file it did not create.

### 9.5 The configure commands' targeted update (byte-preservation contract)

`configure-providers` and `configure-models` never re-serialize the file they write; each edits
the existing bytes in place, keeping every byte it was not asked to change ([page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-models.md)).

- An absent or blank file gets its template ([§9](#the-configure-commands-write-location), "The configure commands' write location"), then
  the edit below.
- `configure-models` touches **only** the `[models]` keys of the roles given. `configure-providers`
  touches **only** the keys it is given — `baseUrl`, and `api` and `decisionsFlavor` where given — in
  the table its provider names, matched whatever its case; an entry's other keys, `apiKeyFile`
  included, stand. A key already holding the value is not rewritten, so a run that changes nothing
  writes nothing and is `unchanged`. Every other byte — comments, unrelated keys and tables,
  spacing, key order — is preserved exactly. A rewritten line's own trailing `# comment` is kept;
  its column position is not guaranteed.
- A missing key is added after the last key of its table; a missing table is appended at the end of
  the file.
- `configure-providers --remove` removes the provider's table: its header, through its last key and the
  lines directly below that key with no blank line between, and one blank line that would otherwise
  be left doubled. A comment set off by a blank line before the next table is left for that table.
- Before writing, kbase decodes both the original and the proposed file as TOML and refuses the
  write, leaving the file untouched, unless the only difference between the two decoded structures
  is exactly the values it intended to change.
- The write is atomic (temp file in the same directory, then rename). A file that is a symlink has
  its target rewritten; the symlink stays.
- Any failure along this path leaves the file completely unwritten.
### 9.6 `kbase show-config`

`kbase show-config [--provider NAME]` reports how this invocation's configuration resolves, and
changes nothing: it reads no KB, writes no file and makes no network request. A configuration that
fails strict load is refused before the verb runs, as for every verb (§9.3). Otherwise the outcome is
`done` whatever the report finds: a fault in the pool and an unresolved role are diagnostics, not
refusals, so one call shows every gap at once. It makes no request and opens no client.

The report states the layers this invocation read, in precedence order, each with its path, the
source that named it — `explicit` (a flag), `environment` (`$KBASE_CONFIG_DIR`), `repository` or
`global` — and whether the directory and each of its two files stand; `config.toml`'s merged
`[models]` values, null where a role is unset, and its `[sampling]` values, null where unset; every provider in name order with the layer
that supplied it and the decision surface it speaks — its default filled in, so a provider silent on
the flavour reads `systemone` — including a provider a dropping fault removed (unusable, with the
fault's reason); the provider the selection names and the rule that named it, or
why none was named; one record per role — the endpoint it resolves to, the layer that supplied
each field, a chat role's sampling settings, or why it does not resolve. Its arguments, result and
the `upgrade` a provider with a file key carries are [page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/show-config.md)'s; every provider it names is named
in lowercase ([page §2.7](NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)).

The sources vocabulary is `layers`': `repository` or `global` for a value a file holds, `explicit` or
`environment` for one read from the one directory a flag or `$KBASE_CONFIG_DIR` named. Its
diagnostics are one `provider` item per fault, each carrying `path` the `providers.toml` that carries
the fault — which need not be the file a selection or `configure-providers` names — or, for a stored key file
that does not open, that file, its `detail` what is wrong with it and, where it is truncated or
does not decrypt, `remedy` the refused-key remedy (§9, "A refused key"), the provider's name filled
in — or, for a store directory that stands and does not list, that directory and its `detail` what
went wrong, with no `remedy`; then one `provider` item per provider whose stored key leaves the
`apiKeyFile` it states unread, `path` the `providers.toml` that states it and `remedy` removing
that line from the file, or `kbase auth remove <provider>` to use the file; then one item per role
that does not resolve, its `path` naming the file the setting belongs in: the
write target's `providers.toml` or `config.toml`. The upgrade and the unread-`apiKeyFile` warning are
this report's alone: no other verb's output carries them.

#### Names and paths, never secrets

A key is reported by the kind it is — `store`, `file` or
`none` — and a key file by the path its entry states, never by its content. A `file` key's entry
carries its `upgrade`, the command that moves the key into the store with its values filled in,
`cat <key-path> | kbase auth add <provider>`; the stored key then outranks the file.
`kbase auth add` runs from a shell with the key piped on stdin; no MCP tool accepts a key (§11).

#### It describes the process that ran it

The layers, the environment and the selection are this
invocation's, so a call through the MCP server (§11) reports the configuration of the server
process.

---

## 10. Metadata format version

### The stamp

A KB declares its metadata format as `kb-format: <major>.<minor>.<patch>`, semver,
in `kb-root/.index/format.yaml`: one block mapping carrying the one key, no `---` fences. The entry
point's frontmatter is the stamp's superseded place, standing in every KB older than `3.0.0` and
read wherever it still stands. Three forms are
superseded: `1.0.0`, which `2.0.0` replaced, `2.0.x`, which `3.0.0` replaced, and `3.0.0`, which
`4.0.0` replaces. Major: a reader
built for the old major misreads the KB (a renamed field,
changed semantics, a changed anchor rule). Minor: additive; an old reader ignores the addition
safely. Patch: the bytes a writer produces changed, their meaning did not.

### Reading the stamp

Before any other read, kbase reads `kb-format` from
`kb-root/.index/format.yaml`; where that file does not stand, from the entry point's frontmatter,
the superseded place — and where both stand, the file's stamp is the KB's. A KB with neither stamp
file nor entry point is refused (`check: kb-root`) as it is today; a KB stamped in neither place,
and an unreadable stamp in either, are refused (`check: kb-format`), the refusal naming where it
looked. Every producer of a KB stamps the file: the build's spine, `init` at birth, and every save
(below). A stamp at the current major and minor with another patch is read as current and
rewritten on the next save.

The current version is `4.0.0` ([page §1.8](NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md)).
No converter leads to a patch, so a migration chain stops at the
form its last converter produces and the next save supplies the stamp: a `2.0.0` and a `2.0.1` KB
both convert through the one `2.0.x → 3.0.0` converter (below), `2.0.1`'s patch having changed only
the words of the band ladder's bottom rung on a derived line.

### One version read and written

kbase reads and writes exactly the current version (ARCHITECTURE
§12). An older stamp is read through migration (below). A newer major or minor is refused by every
subcommand that reads the KB — `build` on an existing KB, `update-format`, `verify`, the write ops, the
queries, `score` and `export-latex`; `status` and `cancel` do
not load the KB — outcome `refused`, one
item `check: kb-format` naming the KB's version and kbase's, remedy "update kbase". No subcommand
writes an older format, and nothing downgrades.

**Migration happens at load, and only `update-format` saves it**
([page §2.6](NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)). Reading an older KB converts it — the
KB's files as bytes in, the current form's files as bytes out, plus the set of repository-relative
paths the conversion made obsolete and the conversion's warnings — version step by version step, in
memory; every command then sees the current form, and a converted, unsaved KB answers every query
as converted. A read-only command (`verify`, the queries, `export-latex`) writes
nothing, and every other write — a write batch, dry run or not, `score`, `build`
resuming over the KB — is refused while the conversion is pending (`check: index`, remedy
`kbase update-format`). `update-format` (§8) saves the conversion, in this
order: `.index/edges.yaml`, then `.index/nodes.yaml`, then every other file the conversion produced
(each document with its frontmatter removed); then the register pages and the
sheets; then the stamp file is written at the version the KB is still at, so that a stamp always
stands and always names the version the tree is at; then the obsolete paths are removed, less any
the save itself wrote; then the entry point, whose frontmatter the conversion stripped; and last
the stamp file is rewritten at the current version — the stamp is the mark that the migration
completed — after which it rewrites the maintained blocks. The removals are reported under `removed` (§7). Every read of a KB file goes through one
loader, `internal/kbload`, which serves the migrated bytes to every command on an older KB
(ARCHITECTURE §11). A save that stops part-way leaves the old form
standing, stamped at the version it converts from, with some files already current. Where
`.index/nodes.yaml` then stands, both record files are the current form's —
no `2.0.x` KB carries a `nodes.yaml`, and it is written second — so the next load takes them as they
stand, strips any remaining frontmatter unread and derives nothing from the `2.0.x` registers or
frontmatter. Such a tree is not clean, so `update-format` refuses it (`check: dirty-paths`); the recovery
is to restore `kb-root/` and any moved build record from `HEAD`, then run `update-format` again
([page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)). On an unmigrated KB `verify` reports
`index-freshness` with remedy `kbase update-format`.

**`1.0.0`**, the first major bump, is one change of serialisation: standard YAML frontmatter (`---`
fenced, the same keys and values), with one change of shape — a leaf's
repeated per-node key groups (`exp-id`, `status`, `strengthens`; `sup-id`, `supports`) become one
mapping each in a top-level `experiment-nodes:` or `support-nodes:` list, keys and order kept, a
one-node leaf a one-element list; the build records as YAML (`kb-build-*.yaml`); `.index/*.yaml` as
YAML streams, one document per line, each the record as a JSON object after the `--- ` marker; a
writer never folded a record across lines. The YAML frontmatter stood at the top of the file, so the
up-link line (§3) was the first line after its closing `---`.

**`2.0.0`**, the second major bump, is the scoring vocabulary: three quantities named, five node
types in place of six, and one scalar on every scoring edge. It kept the serialisation above and
changed field names, relation names and the arithmetic those fields feed. Its patch `2.0.1` changed
the bottom band's words on a derived line, and nothing else.

**`3.0.0`**, the third major bump, moves the graph into records. Every node and every edge is a
record in one of two authored files, `.index/nodes.yaml` and `.index/edges.yaml`; a statement marker
in a leaf hosts each claim, analysis and experiment; the register pages are rendered from the
records and never read back; and no document carries frontmatter at all, the stamp standing in
`.index/format.yaml`. Everything else — the index the queries, the sheets and `score`
read — is derived at load and never stored: strength,
derivation strength, build status and band, the premise and citation counts, a scoring edge's
`target_kind`, the citations, subtree aggregates, each document's kind (the entry point is the entry
point, a document whose child list names a child an index, any other document a leaf), each node's
host, canonical path and anchor. The scoring vocabulary of `2.0.0` —
five node types, three quantities, four scoring relations, one arithmetic — stands unchanged.

**`4.0.0`**, the fourth major bump, is what
[page §1.8](NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md) defines. Of it, kbase now writes the
records' half: each scored record's `judged-hash`
([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)), an
experiment's `status` `run` or `planned`, no `no-edge` field on a claim or an analysis and no
`strengthen-by` field on a claim ([page §1.1](NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)), and a
`references` edge from any kind to any kind
([page §1.2](NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)); its markers, anchors and
summary pages ([page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md),
[§1.6](NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)); and its citations
([page §1.5](NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)). The rest of this section
describes `4.0.0`; `1.0.0`, `2.0.x` and `3.0.0` are the forms it supersedes, and their names appear
only in the migrations below.

### The KB YAML dialect

kbase writes, and a reader needs, only this subset: block mappings and
block lists, lists of flat mappings, a block list of scalars as a mapping's value, plain or
double-quoted scalars, no anchors, tags, folded or multi-line scalars; flow style only for a list of
scalars (`[clm-a, clm-b]`) and an empty list or mapping (`[]`, `{}`); a scalar is never wrapped. A
reader of the dialect, in any language, is conformant when it reads the record files of the `4.0.0`
golden trees to the values their decoded-values goldens hold (ARCHITECTURE §12).

### Recognising frontmatter

A document's leading `---` block is frontmatter only when its first
non-empty line is key-shaped: `name:` with a kebab-case name. Such a block that does not parse as a
YAML mapping is the malformed-frontmatter refusal. A leading `---` block whose first line is not
key-shaped (a prose paragraph, a list, the content after a thematic break) is prose, and the
document has no frontmatter. kbase reads frontmatter on the entry point alone, and there for the
superseded stamp alone (§10); `verify` reports a
frontmatter block on every document, the entry point included (§8).

### The record files

Each is one YAML document: a block list of block mappings, `[]` when it holds
none. A node record's type is its `id`'s prefix and an edge record's its `relation`; no record
carries a `kind`. Every field its type has is present and any other field is refused; an empty
value is spelled `""` or `[]`, never omitted, so no reader takes an absent bearing for the judgement
`0`. Strings are double-quoted, escaping `\`, `"`, every control character below U+0020 but tab, and
the characters a YAML reader would take as a line break or refuse as unprintable, as `\uXXXX`; a
string holds no line break, and a prose value collapses each whitespace run to one space, its ends
trimmed — adoption and an op's re-assertion test compare the collapsed value.
Numbers are plain, in the spelling a rigor is written in; booleans `true` and `false`; keys in the
type's field order. Nodes are written in `id` byte order and edges by identity — relation in the
order of the relations table below, a `demoted` record's `replaced-relation` in the same order, then
`source`, then `target`. Every writer re-renders a whole file from the records, and writes it only
where its bytes change.

A node record's fields, in order, are its kind's in
[page §1.1](NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)'s record-fields table.

A `title` is a non-empty line ([page §1.1](NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)); two entries
of one register page sharing a title take the anchors GitHub's slugger gives repeated headings
([page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) "Anchors"). `rigor` is a number in `[0, 1]` or `"*pending*"`. `manually-scored` is a boolean
(§8). `rationale` is a string, `""` for none; `status` `"run"` or `"planned"`. `judged-hash` is a
SHA-256 digest as 64 lowercase hex digits, all zeros on a value never judged
([page §2.9](NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)). No record
names a register: the page that renders a node is derived from its host.

| Edge record | Relations | Fields, in order |
|---|---|---|
| unscored | `references` | `relation`, `source`, `target`, `context` |
| demoted | `demoted` | `relation`, `source`, `target`, `context`, `replaced-relation`, `bearing`, `manually-scored` |
| bearing | `founds` | `relation`, `source`, `target`, `context`, `bearing` |
| scorable | `supports`, `lifts`, `refutes` | `relation`, `source`, `target`, `context`, `bearing`, `manually-scored`, `judged-hash` |

`context` is `""` for none, and a query reports it null. `bearing` is a number in `[0, 1]` or
`"*pending*"`. A scoring edge runs push, from the supporter to the supported; a `references` edge
runs from the node that names to the node named. An edge record's identity is (`relation`,
`source`, `target`), but a `demoted` record's is (`demoted`, `replaced-relation`, `source`,
`target`), so an analysis's `lifts` and `refutes` of one claim, both demoted, are two records.

**Statement markers** are [page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)'s: the marker
`<!-- kbase-id: <id> -->`, attached with no space after its line's last character, a paired
analysis's abutting its claim's; the reading of each node's statement, the two layouts the build's
analysis split writes among them; the `statement-marker` fault; and the anchors. A host is derived
at load, and no record stores hosting. The build's analysis split (§5) places both of the split's
layouts, through its build-only `split-claim`. No public write op places the blockquote layout;
the add ops and `move-node`, which place a marker at the end of the line `document`, `line` and
`snippet` name, can place the paragraph layout, and a marker placed into either layout reads by
it ([§8](#add-kind), [§8](#move-node)). A `line` naming a
fenced block's opening or closing line (a display-maths fence included) is refused, naming the
line.

**Citations** are [page §1.5](NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)'s: the quote,
paraphrase and naming forms, each a Markdown link whose title is the cited node's id; the link
target per kind; and a bare node id in prose, `verify`'s `bare-id` fault (§8). A link whose title is
absent or is no node id is an ordinary link. kbase reads every link, a citation's title included,
through one link grammar.

**Register pages** are [page §1.6](NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)'s summary
pages, `kb-summary.md` at kb-root and in each volume (§3): which page a node's entry stands on, the
title, the one-line preamble, each entry's `## <title><!-- kbase-id: <id> -->` heading, what an entry
shows, the entry order and a page's removal. An entry's anchor is its heading's among every heading
its page renders ([page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) "Anchors"), and every link
a page renders is computed from each node's derived page and anchor. A page no longer called for is
reported under `removed`. A prose link `<page>#<anchor>`
holds while its node stays on that page with that anchor, and the dead-link gate reports one that no
longer does (§8). A page is never read back: `verify` renders it in memory and compares bytes, and a
difference is `register-freshness`, which an empty write batch fixes ([page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)). Beyond the anchors and the freshness
compare, what an entry shows is presentation and untested: today an entry carries the leaves citing
it, a `### Rigor` section and, on a claim or an analysis, a `### Strength` section.

### `canonical_path` and `canonical_anchor`

Each node's derived row carries a canonical path and anchor, which the claim-graph sheet links to
and the scoring run reads a node's text through; no result carries them, `show` and `find` naming
`host`, `host_anchor` and `summary` instead ([§7](#item)). A claim's are its register page and its entry's anchor
there; a work's the root register page and its entry's anchor; an analysis's and an experiment's
the leaf hosting it and the anchor of the heading of the section holding its marker; an axiom's
`axioms.md` and the slug of `Axiom <label>: <title>`. Anchors are
[page §1.4](NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)'s. A node with no host has neither.
### Axioms

An axiom is declared by its node record, which `add-axiom` writes (§8);
`axioms.md` is prose stating it, one `### Axiom <label>: <title><!-- kbase-id: <id> -->` heading
each, the marker pairing heading and record ([page §1.1](NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)), and is not read for declarations. `verify`'s
`axiom-heading` check holds the two together by the three checks page §1.1 lists, a fault the
author's prose edit clears ([page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)).

### The load: every fault refused

The load refuses a KB whose records hold any fault
[page §2.4](NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md) lists, `check: metadata`,
naming each with its file and its line or record; the pairings a relation joins are the relations
table below. An absent record file in a KB stamped at the current version is refused
`check: index`, remedy "restore it from git". The three faults no write op produces — one edge
identity on two records, an edge naming a node no record carries, a cycle among the scoring edges —
carry the remedy "return the record files to a consistent commit in git, such as the one before a
merge, and reapply the other side's changes through write ops". No op resolves a load error.
### Five node types

`claim`, `analysis`, `experiment`, `work` and `axiom`. `node_type`,
`target_kind`, the index, the sheet and the result documents use these five words and no other.
Every id is minted, its kind's three-letter prefix over a minted body, and a work's citation key and
an axiom's label are its record's `label` field ([page §1.1](NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)), so a token names its kind without
translation. An axiom is declared by its record and stated in `axioms.md` (above), under the
heading its marker pairs with the record. Authoring prose may
say *invariant*, *framework premise*, *framework*, *proposition*, *evidence*, *trial*, *cited work*
or *reference*: *invariant* and *framework premise* name an axiom, *proposition* a claim, *evidence*
and *trial* an experiment, *cited work* and *reference* a work; *framework* names the document that
declares axioms and is never a node type. The graph has one word per role; the other words are the
author's.

### Three quantities

`rigor` is a node's local value — a claim's, an analysis's, a work's or an
experiment's — a number in `[0, 1]` or `*pending*`; `bearing` is a scoring edge's scalar, a number in
`[0, 1]` or `*pending*`, and the one token on every scoring edge; `strength` is the derived value, a
number in `[0, 1]` or pending, on a claim and on an analysis. `*pending*` is the format's one
no-number literal: written for a value nobody has judged, and read wherever a value is absent. A work,
an experiment and an axiom have no derived value — their `rigor` is terminal, and an axiom's is `1.0`
by rule, which no authored file carries — so each one's strength is its `rigor`.

### Four scoring relations, one arithmetic

Every scoring edge is stored push: `Source` is the
supporter, `Target` the supported node, and `target_kind` the target's kind.

| Relation | Source kinds | Target kinds | Contribution |
|---|---|---|---|
| `supports` | claim, experiment | claim, analysis | `bearing × source strength` into the minimum |
| `founds` | work, axiom | claim, analysis | `bearing × the source's rigor` into the minimum; an axiom's rigor is `1.0` by rule |
| `lifts` | analysis, experiment | claim, analysis | `bearing × the lifter's strength`, maxed above the minimum |
| `refutes` | claim, analysis, experiment | claim, analysis | a cap, `1 − bearing × refuter strength`; the smallest cap wins |

`supports` carries a premise from the argument itself; `founds` carries support from outside it — a
cited work, or an axiom the argument leans on. An axiom's declaration in `axioms.md` is bookkeeping,
not origin: what it states is an external fact.

`references` and `demoted` are the two unscored relations: `references` carries no bearing and no
value, and `demoted` keeps the bearing of the edge it replaced, which no walk reads. A bearing is
required on every scoring edge and illegal on these two. A bearing weights the contribution and
never modifies its source's value.

```
premises   = every supports and founds contribution that participates
derivation = min(rigor, every contribution in premises), and where premises is empty the node's
             own rigor for an analysis, 0 for a claim
final      = min(max(derivation, every lift), every cap)
```

`rigor` caps what the premises establish and is no premise itself. A node no participating premise
carries has an empty premise set, and its derivation is what its kind is worth bare: `0` for a claim
— whatever its `rigor` says, judged, pending or absent — so a bare claim reads `0.00` and lands
`unsubstantiated`, however good its statement; an analysis's own `rigor`, since a method's inputs are
the rules of inference and not a KB node, so a premise-less analysis reads its method's value and the
claim it lifts reads it too. An edge that does not contribute is no supporter, so a claim whose only
edge is excluded reads `0`. Where a premise does participate and `rigor` is pending, the derivation is
unknown and propagates nowhere: a lift stands over it — `max(pending, lift)` is the lift — while a
refutation alone leaves it unknown. `lifts` and `refutes` compose around the derivation unchanged: a
claim whose only scoring edge is a lift takes the lift's product, and one whose only scoring edge is
a refutation takes `0`; an analysis composes the same two around its own rigor, so a lift takes
`max(rigor, lift)` and a refutation `min(rigor, cap)`.

The walk carries full precision: nothing rounds a value on its way into a derivation, so a dependent
multiplies its source's exact strength. A register page — its `- strength:` line and a premise's
`(strength …)` — carries the value in the fewest decimals that read back as it: four in the ordinary
case, more only where four would lose it, trailing zeros trimmed. A query result carries it as a
number at two decimals, the trace on a `- strength:` line spells it at two, and a `verify` message
quotes the record's spelling, being about the record. A pending bearing, or a pending source strength,
excludes its edge — no contribution, no lift, no cap — while `0.0` is a judgement and participates,
taking a minimum to zero and a cap to `1.0`. A non-`run` experiment is inert as a lifter or refuter,
and its `supports` edge is excluded like any unjudged premise.

**A value is derived from a charge list, not composed from the interior of the scale.** Each of the
two scoring passes asks a model for the faults the text commits — the ids
[RIGOR_AND_BEARING_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/rigor-and-bearing/entry-point.md) defines, each with the
line that shows it — and kbase prices the list: the pass's maximum less its ids' weights per
occurrence, floored at zero and exact at two decimals (§8). A written `rigor` is a value in
`[0, 1]` or `*pending*`, and a bearing's the same; what changed is who chooses a number. A pass
derives it, `1.00` is the rigor tables' maximum — the largest a `rigor` pass derives, which reads *no
charge among the six*, never *the text displays its reasoning*. An axiom's `1.00` by rule is that
same number and no separate case on the read path: an authored `rigor: 1.0` — a converted
experiment, which the `1.0.0` migration gives `rigor: 1.0` — is in domain and reads as itself
everywhere the system shows it — the strength line a refresh derives from it, the index's
derivation, a query's display and the walk. An authored `rigor` above the maximum is outside the
domain, as an infinity (`.inf`) is: the load refuses it (`check: metadata`), naming the record file,
the record and the field; its remedy says kbase never writes such a value, so whatever wrote it is
the way to undo it. A bearing's `1.0` stands, its table reserving nothing either. The walk itself is
unchanged and reads the value it is handed, whatever wrote the number.

### The band ladder

A `strength` is read against five rungs, each an inclusive lower bound: `0.85`
*ok to build on*, `0.65` *ok to build on, see caveats*, `0.45` *use as input only*, `0.20` *do not
build on, rework needed*, and `0.00` **unsubstantiated**. The bottom rung states an outcome and not a
cause: every strength below `0.20` lands there from either direction — a refuter's cap, or a minimum
no premise carries — and what it says is that nothing a claim built on this would inherit
substantiates the claim. The cause, where a reader needs it, is the run's fault record and the
derivation trace. One value carries one band wherever a band is shown: a claim's or an analysis's
record (`build_status`, `strength_band`), the `stats` distribution's row, and the sheet's fill and
tooltip. A pending strength has no band, and a strength below `0.00` cannot occur.

**A claim stands alone; an analysis is made of its citations.** A claim is a proposition about the
subject matter: its premises support it, and it means what it means without them. An analysis is
constituted by the nodes it cites — strip the ids from its sentence and no proposition remains — so
it has at least one premise and concludes in a claim. An analysis citing none is a method with no
inputs but its own, and reads its `rigor` rather than a claim's `0` (above). The authoring test is
that removal: if a proposition survives it is a claim, and if nothing survives it is an analysis. A
claim is the model's atom and needs neither: it may stand with no premises and nothing resting on it.
These are rules for authors, not faults: the loader enforces neither, and a KB that breaks one is
still read.

### Edge direction and authoring

Every edge is stored as its record says, the scoring edges push:
a `supports` or `founds` record runs from the premise into the claim or analysis it supports, and a
`lifts` or `refutes` record from the lifter or refuter. `add-edge` ([§8](#edge-ops)) writes one edge as its record runs, over
[page §1.2](NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)'s legal pairings. An analysis lifts or refutes and never
supplies a premise, so an `add-edge` naming one as the source of a `supports` or `founds` edge is
refused as a pairing no relation joins, as the load refuses such a record. `edges` presents the edges at a node
from the reader's side, by `--direction`: `in` where the node is the target, `out` where it is the
source ([§8](#edges)); `trace` follows them transitively.

### Required bearings

Every scoring edge record carries its `bearing` field, and the strict load
refuses one that lacks it, carries a value outside `[0, 1] ∪ {*pending*}`, or carries it on a
`references` record (above), naming the file, the line and the field: a missing bearing is never
read as `0`, nor as pending.

### The vocabulary

`confidence`, `quality` and a work's `strength` were three names for one local
value; all three are `rigor`. `solidity` was the derived value; it is `strength` on every surface —
a register page's `- strength:` line and `(strength …)` annotation, the derived rows, the query
keys, the sheet's bands and `find`'s `strength` sort. The register page is
`kb-summary.md`, placed by host (above); an entry's authored values stand under `### Rigor` and its
derived line and trace under `### Strength`; the statement marker is `<!-- kbase-id: … -->`. A
claim's and an analysis's entry carries both headings; a work's carries `### Rigor` alone, having no
derived value.

### The `1.0.0 → 2.0.0` migration

One converter reads a `1.0.0` KB's files as bytes and writes the
`2.0.0` form's, which the `2.0.x → 3.0.0` converter (below) then takes. A file it does not locate it
does not touch, and a file already malformed under `1.0.0` is carried as bytes for the `2.0.x` load
the next step runs to refuse by name — the converter never makes invalid `1.0.0` bytes valid. Nor
does it drop bytes: a file holding content the converter cannot carry refuses the conversion,
`check: conversion`, naming the file, the line and the content, so the load writes nothing and the
KB stands as it was. It

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
  pair at bearing `1.0` declared on the leaf that hosts it. The analysis's statement marker stands on
  the claim's own marker line, after the claim's, in the leaf the claim's statement is read from; a
  claim its leaf lists and does not mark takes one on the leaf's first heading. An argument stating
  no premise gives its analysis no premise list, which the walk reads at the analysis's own rigor. A
  claim no leaf carries is not split: it keeps its premise list and its rationale, since the analysis
  would hold them on behalf of no host — the `2.0.x` load reads no node for it;
- gives every scoring edge its bearing: an old `(applicability X)` becomes `(bearing X)` with its
  value carried, an untokened premise bullet takes `(bearing 1.0)` — work and axiom bullets included
  — and every experiment's mapping gains `rigor: 1.0`;
- rewrites each entry's `### Quality` to `### Rigor` and `### Solidity` to `### Strength`, and
  renames the register file `claim-quality.md` to `claim-rigor.md` with its Tier-2 marker
  `<!-- claim-quality: … -->` to `<!-- claim-rigor: … -->`, and rewrites every Markdown link to it
  to name `claim-rigor.md`, its anchor kept;
- rewrites every authored `work-<citation key>` id to `wrk-<citation key>` where it appears, an
  analysis's `support-nodes:` to `analysis-nodes:`, its `sup-id:` to `ana-id:` and its `sup-` ids to
  `ana-`;
- writes `axioms.md` — one `### Axiom <label>: Title` heading per `### INVARIANT-<label>:` section and
  per `- Axiom N: **…**` bullet of `invariants.md` or a legacy `AGENTS.md`, each axiom bullet's body
  beneath its heading — rewrites every `INVARIANT-<label>` citation in the register to
  `Axiom <label>`, a legacy `Axiom N` citation already being the `2.0.0` spelling, gives that bullet
  the bearing an axiom premise now requires, and moves a legacy bullet's trailing parenthetical into
  `[context]`; labels are preserved and nothing renumbered;
- splits a bullet naming several scoring targets into one bullet per target; and
- removes both paths the form no longer has, reported under `removed`: `claim-quality.md`, and
  `invariants.md`, whose declarations have moved into `axioms.md`. Every other file is left byte for
  byte and no index is converted: the `1.0.0` index was derived.

**The migration is not value-identical, and no derived-value equality is promised.** Four causes,
and every one of them is a value a reader will see move:

1. **An analysis's lift moves station; an experiment's does not.** Under `1.0.0` an analysis's
   support contribution was maxed into the claim's local value, which then entered the minimum: a
   corroborating analysis could be dragged down by a weak premise. Under `2.0.0` a lift stands above
   the minimum. A supported claim whose lift exceeds its derivation rises to the lift; where the
   derivation is at or above the lift, the value is unchanged. An experiment's pair already stood
   above the minimum, so its station does not move.
2. **`founds` scales.** An old `applicability X` was a gate: any positive value contributed the
   work's whole rigor, and `0` skipped the edge. A `2.0.0` `bearing X` contributes `X × rigor`, so
   every work edge below `1` falls, a `0` drives its claim to zero, and `1` is unchanged.
3. **An untokened or pending work edge stops pending.** A work bullet with no applicability, or an
   explicit pending one, left its claim pending. The untokened bullet now carries `1.0` and
   contributes the work's rigor; an explicit `*pending*` token is carried and the edge is excluded.
   In both cases the claim reads a number where it withheld one.
4. **A claim nobody judged reads `0`.** A `1.0.0` claim whose `confidence` is `*pending*` withheld a
   number. The split gives that unjudged value to the claim's analysis, and an analysis whose
   premises are judged reads pending — so the claim's lift from it is excluded, the claim derives
   nothing, and it reads `0.00` and lands `unsubstantiated`. That is the walk's rule and not a
   defect: an unjudged analysis is no lift, exactly as an unjudged bearing is no supporter.

An old applicability was authored as a gate, not a weight, so every non-`1` value needs a human
re-read before it is trusted; the converter carries the number and does not reinterpret it, and it
invents neither a finding nor a context where a migrated work bullet's context names none.

### The `2.0.x → 3.0.0` migration

One converter reads a `2.0.x` KB's files as bytes and writes the
`3.0.0` form's: the two record files from what the `2.0.x` load reads, every document stripped of
its frontmatter — the entry point's `kb-format` with it, the next save writing the stamp file — and
a statement marker for each node. It refuses only
what the `2.0.x` load refuses, with that load's own items, and the owner repairs such a KB as a
`2.0.x` KB; otherwise it never refuses, but takes the outcomes below and warns — one item each,
`path`, `line` and `detail` — wherever it drops data or invents it.

- A claim's and a work's register entry becomes its node record — `title` its heading, `rigor`,
  `rationale`, `no-edge`, `strengthen-by` — and which `2.0.x` register held it is dropped silently:
  its page is now its host's (above), so a node printed in another volume's register moves, and a
  prose link to its old anchor is reported by the dead-link gate. A second register entry for one
  id is dropped with a warning.
- An analysis is its leaf's `analysis-nodes` entry joined with its register entry, titled with the
  heading the register entry is written under — the author's title for it — and where the register
  carries no entry for it, with its host leaf's first heading. An analysis entry no leaf hosts, which
  `2.0.x` never read as a node, is dropped with a warning. Its canonical anchor is the section its
  statement marker stands in, wherever the marker is (below), so a title and an anchor differing is
  no loss: the anchor names the place the reader lands on. An
  experiment's `experiment-nodes` entry becomes its record, titled with its host's first heading and
  its `rigor` `*pending*` where it had none. A host with no heading titles its node with its file
  name, with a warning. An axiom is `axioms.md`'s heading, else a legacy declaration, as the `2.0.x`
  load read it.
- Every `rests-on`, `refutes`, `references` bullet, register analysis pair and leaf pair line
  becomes an edge record oriented as the `2.0.x` index row, `context` `""` where it had none. A
  bullet whose token named no node, which `2.0.x` dropped too, is dropped with a warning quoting it.
  An edge authored more than once — a `rests-on` target repeated, an analysis pair in both the
  register and the leaf, an experiment's `supports` pair beside a `rests-on` bullet naming it — is
  one record: equal bearings silently, their contexts joined with `; ` in authored order; different
  bearings keep the higher, a number above `*pending*`, with a warning naming both sites.
- A `demoted` bullet on an entry X naming Y becomes a `demoted` record from Y into X, replacing
  `supports`, its bearing `*pending*` — `2.0.x`'s cut kept no bearing, so none is lost — and its
  origin dropped silently.
- Every `manually-scored` is written `false`.
- Each node's statement marker is placed from the `2.0.x` `claims:` listings and markers. A claim
  marked in a leaf that lists it keeps that marker. A claim listed but marked nowhere it is listed
  gets a marker in the first listing leaf in path order — an equation's on the line inside its
  display-maths fence carrying its `\label{…}`, and a claim whose leaf lists no other non-equation
  claim and holds one labelled blockquote on that blockquote's first content line, so its label line
  still reads as one, silently; any other on the leaf's first heading line, with a warning, the
  section that heading opens being its statement. A leaf with no heading gets none of the markers
  its heading would carry, each with a warning. Every other listing leaf becomes a mention,
  with a warning, its `referenced-by` row lost, and every other marker of the claim is removed, with
  a warning. A claim listed nowhere keeps its first marker in path order and loses the others, each with a
  warning; one listed and marked nowhere has no host, `verify`'s `statement-marker` fault. An
  analysis's or an experiment's marker that a `2.0.x` document already carries stands where it is —
  the section it sits in is the node's statement, so one the `1.0.0` migration placed beside the claim
  it argues for keeps that place — and a second marker for the same node is dropped with a warning
  naming the one that stands; a marker standing in a leaf other than the one declaring the node is
  warned of, naming both paths, and its marker's leaf is the statement's home. Where none stands, one
  is written on a new line directly after the host's first heading.
- Frontmatter is removed from every document, the entry point's with it. `kb-format`, `kind`,
  `claims`, a string `no-claim` and the `subtree-*`
  keys are dropped silently, markers and the derivation carrying their facts; `experiments`,
  `path-stable`, a key no `2.0.x` reader knew, an `experiment-nodes` or `analysis-nodes` list on a
  document that is not `kind: leaf` — which declared no node to the `2.0.x` load — and a value of
  the wrong type are dropped with a warning naming file, line and key.
- In a `2.0.x` register, text outside every entry is dropped silently, and text inside an entry that
  is neither a field it read nor a derived line is dropped with a warning quoting it. The registers
  are left as bytes: the save renders the pages the KB calls for (above) over them, and removes a `2.0.x`
  register at a page position no page takes, reported under `removed`.
- The derived `.index/edges.yaml` is replaced by the record file, and every other file under
  `.index/` but the two record files and the stamp file — `2.0.x`'s `claims.yaml`, `cites.yaml`,
  `strengthen-by.yaml` and `subtree-aggregates.yaml` — is obsolete, reported under `removed`. The
  stamp file is carried through, holding the version the tree is still at, until the save's last
  write replaces it. The stamped `AGENTS.md` and `CONVENTIONS.md` are not read by the conversion:
  `update-format` rewrites the block each holds between its marker lines, and leaves one holding no
  such pair untouched, listing it under `unfenced`
  ([page §1.7](NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)).

### The `3.0.0 → 4.0.0` migration

One converter reads a `3.0.0` KB's record files through the
`3.0.0` load and writes them in the `4.0.0` form, doing the records' steps of
[page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)'s list: an
experiment's `status: pending` written `planned`; each `no-edge` and `strengthen-by` field dropped,
one warning per field naming the node's id and quoting its text (a claim's `strengthen-by` items
joined with `; `); `judged-hash` written all zeros on every scored record, `manually-scored`
kept; and each work's and axiom's id minted from its `3.0.0` id alone, the same id on every load
and in `update-format`, its key or label written as its `label` and every `3.0.0` work or axiom id
in an edge's end or a prose field renamed to the minted id, one warning per id listed `ids` with
the `3.0.0` id and the minted one. A bare id is read in the `3.0.0` grammar and cited under its
minted id, and each `### Axiom <label>:` heading of `axioms.md` naming an axiom record ends in that
axiom's marker, one naming none reported and left as written. It does the markers' steps of that list: every statement marker in a kb-root document
rewritten in the `kbase-id` form, attached with no space, a marker naming two ids written as two
abutting markers, and an analysis marker standing alone on the line directly after a line ending in
the marker of a claim it lifts moved to directly after that claim's marker, its emptied line removed.
The `3.0.0` register pages, `claim-rigor.md` at kb-root and directly inside each of its
directories, are obsolete and reported under `removed`; the save renders the summary pages. It does
the citations' steps of that list, resolving every anchor under `3.0.0` slugging: every link into a
heading, a citation or an ordinary link, rewritten to the heading's `4.0.0` anchor; the node id
added as the title of each link whose anchor resolves to one node's statement — a heading's section
holding that node's statement marker, an axiom's heading, or a `3.0.0` register entry, whose link
is pointed at the node's place — and a work's citation pointed at its entry on the root summary
page; and each bare node id in prose, and each link whose text is one, made the node's citation in
the naming form, one warning per conversion listed `bare-ids` with its path, line and id. Each link
or bare id it cannot resolve to one node without guessing — a section holding several nodes'
statements, a register anchor several entries share, a heading the anchor names under neither
slugging or names one heading under `3.0.0`'s slugging and another under `4.0.0`'s, a node with no
place to point at — takes no id, one warning listed `unresolved-citations`. A heading's `4.0.0`
anchor is read after its own bare ids become citations. A link into a removed register page never
stays aimed at it: where every entry its anchor names is stated in one section it points at that
section; otherwise at the summary page replacing the register page, without the anchor; and where
no summary page replaces it, the link is removed and its text stands, a bare id in that text
converted as any bare id is. Every other file is carried
as it stands. A `3.0.0` KB whose record files the `3.0.0` load refuses is
refused with that load's own items, `check: metadata`, one per fault.

### The migration announces itself

A verb that reads a KB through the loader on an older version
writes one sentence to stderr, and each warning on a line of its own, and carries one `migration`
record — `from`, `to`, `note`, `warnings` — after `kb-root` (§7), on every such call until the
`update-format` whose save stamps `4.0.0`. `note` is, for every migration, "every scored record's judged hash is now
the unjudged one: no rigor or bearing is claimed current, and the next score run finds every one
dirty". §7 names the verbs. Over MCP the
document is the channel and stderr is the server's log; `status` and `cancel` load no KB and never
carry it. A `4.1.0` and a `5.0.0` KB are refused `check: kb-format`.

**The `demoted` relation**, added at `1.0.0`. An edge class only the build's cycle walk
writes, and only the build's analysis split (§5) moves: a
scoring edge the walk cuts to break a cycle is replaced by [page §1.3](NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)'s `demoted` record, which
keeps it whole in its direction and stands outside the strength walk and the cycle check.
`replaced-relation` is `supports`, `lifts` or `refutes`: a `founds` source, a work or an axiom, has
no incoming scoring edge, so it closes no cycle. The build writes cuts through two
build-only write ops, `add-build-edges` and `demote-claim-edge`, and moves one onto the analysis it
splits from a claim through a third, `split-claim`. None is a subcommand or among the public ops,
so the public write surface cannot create one. `demote-claim-edge` replaces one standing scoring
edge, named by its `source`, `target` and `relation`, with its `demoted` record; where only that
record stands it leaves it as it stands, and an edge with neither is refused. Each cut is one
record, so more than one `demoted` record from one `source` to one `target` stands only where
separate cycles were cut at different relations between them. A record is resolved in one of
[page §1.3](NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)'s three ways — `restore-demoted`, `remove-edge` with `relation: demoted`, or `add-edge`
of the replaced edge ([§8](#edge-ops)); the second leaves the edge cut. `verify` lists every `demoted` record under `findings`
(§7) with outcome `done`; `edges` reports it as a row of its own relation, out of its source;
`stats` counts them as `demoted_edges`, after `edges`, which counts every row; the sheet draws each
as a cut (§3); and its target's register entry lists it.

### Cycles: cut as the walk finds them

The build's ring row cuts the cycles the build's own edges close, within the stage that writes them,
by one walk; a cycle the records hold at load is a load error
([page §2.4](NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).

- The walk order is the node order of the register pages, which no minted id decides, so a build's
  cuts are reproducible modulo ids (§5): hosted nodes in the reading order of their markers, then
  every other node by id — a work or an axiom, which no scoring edge enters, so it lies on no
  cycle. Edges
  compare by relation in the relations table's order, then source, then target, each end in that
  node order.
- The walk is depth-first over the `supports`, `founds`, `lifts` and `refutes` records, starting at
  each node and following each node's edges in the walk order; `demoted` records are not walked.
- Each time the walk closes a cycle — an edge reaching a node on the current path — it cuts one edge
  of that cycle: the first, in the walk order, of the cycle's `supports` edges into an analysis,
  where one stands; else the edge the walk is on. The cut edge becomes its `demoted` record, with a
  warning naming the edge and the cycle it broke. Nothing else is dropped.
- After each cut the walk starts over, until no cycle remains, so the cuts depend on the edge set and
  the documents' order alone.

Every public op that adds a scoring edge refuses one that would close a cycle (§8), so only the
build reaches the walk. Restoring a demoted edge takes at most two edits: remove or redirect another
edge of its cycle, then restore it with `restore-demoted`, which refuses a restore that still closes
a cycle.

---

## 11. The MCP server

`kbase mcp` serves kbase to any external agent harness that speaks the Model Context Protocol, over
stdio JSON-RPC. It is the subcommands, bound: every subcommand it serves is a tool of its own, an
operation of the `query` family tool, or a write op an entry of the `write` tool's batch, with the
same arguments and the same result, so an external
agent harness without MCP loses nothing but convenience. No tool is listed without a subcommand
behind it.

- **Tools.** Every subcommand is served, less `completion`, `help`, `mcp` itself and
  `wait` — a tool call would hold the turn for the build's or score run's duration — and less
  `auth` and its subcommands — a key arrives on stdin, which no tool call has, and no tool accepts
  a key ([page §4.4](NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md)). `probe` is a tool; its MCP note makes each call one the user directs ([page §4.2](NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)). The queries are the
  operations of one tool, `query`; the `write` tool is `kbase write`'s, whose batch's entries name
  the write ops ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)), and a write op has no tool of its own; every
  other subcommand is one tool, named as the subcommand. Each operation stays a subcommand, unchanged
  on the command line. A command tool's input schema is derived from its command's arguments and flags (the global flags
  excluded — the server carries its own — but for `project-dir`, below), and a duration flag's
  argument is the Go duration string the command line spells (`30s`). A flag is optional in the
  schema but where its command declares it required, as `trace`'s `direction` is. Each of a command tool's
  properties carries a description: a positional's the one its command declares, a flag's its
  usage, followed by the default `--help` prints for the flag where it prints one and, for a
  duration flag, that its argument is a Go duration. A family tool's schema is
  `operation`, required, a string enum of its operations' names, and `arguments`, an object holding
  what the operation takes: its positionals and flags by name, derived as a command
  tool's schema is. `operation`'s description names where each operation's help
  is, and `arguments`' names that help's argument list. The `write` tool's schema is the batch: `entry`, required, an
  array of objects, each naming its op under `operation` beside that op's keys, and `dry-run` and
  `commit`, booleans; the entries' shape is the batch's own check ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)), every fault an item naming its
  `entry`. The
  family tool checks `arguments` against its operation's own schema: an unknown key, a missing
  required key or a value of the wrong type is a refusal document, `refused`, one item per fault,
  `check: usage`, `key` the argument's name, `allowed` every argument the operation takes and
  `remedy` naming `allowed` for the argument names and the operation's help resource for what it
  does. A values entry's keys carry no types there, so
  a wrong value for a values key is the op's own refusal (§8), which names the key and, where the
  value comes from a closed set, lists that set as `allowed`. An `operation` outside the enum, or an
  `arguments` that is not an object, fails the tool's
  schema. Every tool's schema, a family's included, carries one
  more property, `project-dir`: the project the call runs in, the git worktree root that holds
  `kb-root/` or will. The schema is rendered per mode (below): located, the property is
  optional, and a call that omits it runs in the server's located project; not located, it is listed
  in `required`, and its description says only what it names. A call runs in the project it names where it names one, else in
  the server's located project; a call naming an empty one where none is located is `refused`, `check: usage`, `key:
  project-dir`. A `project-dir` that is no git worktree root is `refused`, `check: worktree`, `key:
  project-dir`, `path` the absolute path; a relative one resolves against the server's working
  directory. A refusal over MCP carries no `line` or `column` unless it names a `path`, where they locate it
  in that document, and
  `entry` as the entry's position in the batch; its `key` keeps the CLI spelling (§7), or,
  in a family tool's argument check, is the schema property name. Arguments that fail the tool's
  schema (a missing required key, a wrong type, an unknown key) are JSON-RPC `-32602` errors, not
  refusal documents; a family tool's `arguments` are checked as above instead. A command declares
  itself read-only in its definition, and the tool carries that as its read-only annotation, a
  family tool where every operation declares it; `query` — every query among
  its operations — `verify`, `status`, `build-trail`, `show-config`, `models` and `probe` are the read-only
  set. The `query` tool's description is one sentence, then where its operations' shared help
  is, `kbase://help/query`, and where one operation's is, `kbase://help/<operation>`. A command
  tool's description, `write`'s included, is its command's one-line summary, then its MCP note, then
  where its full help is, `kbase://help/<command>` ([page §4.2](NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)); `write`'s MCP note carries the
  batch contract that page lists. The table is one tool per family, one per other bound command but
  the write ops, and none listed by hand; a test derives the set from the commands' declarations
  and asserts it for equality, so a tool listed by hand fails as a missing command does.
- **Results.** The §7 document, as the result's text and as structured content. `isError` is set
  exactly when the document's exit code (§7) is not 0, so a refusal reaches the model as an error
  carrying its items and remedy while `bounded` and a completed `cancel` do not. No list query pages:
  `citations-of`, `edges`, `find` and `trace` answer completely ([§7](#bounded-results), [§8](#citations-of), [§8](#edges), [§8](#find), [§8](#trace)), and `find` and `trace`
  take `limit` only beside `sort`.
- **Two modes**, located and not located, as [page §4.3](NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md) defines them: `kbase mcp` started inside
  a repository is located; one whose search finds no repository, or `kbase mcp
  --project-dir-required`, is not, and the two serve alike, instructions included. The server takes
  no project flag: the global `--project-dir` given to `mcp` is a usage refusal (§7) before it
  serves.
- **`build` is detached.** The tool starts `kbase build` as its own process, from the root of the
  call's project, passing `--project-dir` where the call named one, in its own process group, with
  no stdin, configured as the server is (the environment,
  `--config-dir`, `--log-level` and `--log-file`, §9). The build's stdout and stderr are captured as
  files in the state store's `reports/` (§6). The tool waits until the build's progress record shows
  it has entered a stage, or until it exits, for at most 5 s, and returns by what happened first
  ([page §3.5 `build`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)); a build starting during a write waits that write out before its first
  stage, and is visible to `status` while it waits:
  - *entered a stage* — outcome `done` with `kb-root`, `state-dir`, `pid`, `resume` and `wait`, the
    `resume` absent where the record's command does not read as a `kbase build` invocation (§6).
    `wait` is the exact shell command `kbase wait [--project-dir <root>] --state-dir <store> --pid
    <pid>`, carrying `--project-dir` where the call named one, shell-joined as `status` joins its
    `resume` (§6): it returns when the run is over and prints that store's
    `status` document, so a session runs it in the background and is woken when it exits. `status`
    and `cancel` follow it;
  - *exited* — the build's own document, `isError` set by its outcome (§7);
  - *still starting at the bound* — outcome `done` with `state: starting`, `state-dir`, `pid` and
    `wait`; the build keeps running, and `status` follows it.

  A relative `state-dir` argument resolves against the repository root; the server's own
  `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status`
  would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that
  refusal and starts no process. The capture files are not removed.
- **`score` is detached.** The tool starts `kbase score` as its own process, from the root of the
  call's project, passing `--project-dir` where the call named one, in its own process group, with
  no stdin, configured as the server is (the environment,
  `--config-dir`, `--log-level` and `--log-file`, §9). The run's stdout and stderr are captured as
  files in the state store's `reports/` (§6). Its `edge` argument is a mapping of `source`,
  `target` and `relation`, each a required string and no other key allowed, which the run receives
  as `--edge <source>,<target>,<relation>`; a value holding a comma is refused at `edge`, `check:
  usage`, and starts no process. The tool waits until the run's ledger and pid stand, or until it
  exits, for at most 5 s, and returns by what happened first ([page §3.5 `score`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)); a run
  starting during a write waits that write out before its first refresh, and is visible to
  `status` while it waits:
  - *started* — outcome `done` with `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait`.
    `wait` is the exact shell command `kbase wait [--project-dir <root>] --state-dir <store> --pid
    <pid>`, carrying `--project-dir` where the call named one, shell-joined as `status` joins its
    `resume` (§6): it returns when the run is over and prints that run's
    document, so a session runs it in the background and is woken when it exits. `status` and `cancel` follow it;
  - *exited* — the run's own document, `isError` set by its outcome (§7);
  - *still starting at the bound* — outcome `done` with `state: starting`, `state-dir`, `pid` and
    `wait`; the run keeps running, and `status` follows it.

  A relative `state-dir` argument resolves against the repository root; the server's own
  `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status`
  would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that
  refusal and starts no process. The capture files are not removed.
- **`build-trail` reports the trail.** It is a subcommand and its tool, read-only; its argument,
  result and refusals are [page §3.5 `build-trail`](NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md)'s. Each `command` is a list of the
  command lines it runs in order: a boundary's, `git reset --keep <commit>` then the build resumed
  with the flags its last run was launched with, the bound excepted, which each surface spells as it
  takes it — `kbase build --resume` and those flags at the command line, the `build` tool's call
  over MCP — and `started-at`'s the reset alone. The trail is the newest
  build's ([§5 Starting over](#starting-over)). `failed` (3) is a git failure.
- **Resources.** The claim-graph sheets (§3) present on disk, the placeholder included, are listed
  by `resources/list` as `image/svg+xml` resources with URI `kbase://sheet/<kb-root-relative path>`;
  `resources/read` returns the SVG as text. A KB that exists but will not open lists no sheet: the
  list is answered without one, and that KB's unopenability is `status`' and `verify`'s to report
  when they are asked, not a list fault. Beside them `kbase://model-hints` is listed as one
  `text/markdown` resource holding the guidance on choosing the `[models]` ids (§9.2) — the same
  rows a created `config.toml` carries — and it is KB-independent text: it is listed whether a KB
  is found and opens, is found and will not open, or is not found at all, and what it states does
  not vary with one. A server with no located project lists no sheet. No resource is
  subscribable, and `initialize` advertises the `resources` capability with no `subscribe` among
  the ones it carries ([page §4.5](NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md)): nothing announces a run's end, so a session polls `status`,
  or runs in the background the `wait` command `build` and `score` return; `resources/subscribe`
  and `resources/unsubscribe` are methods the server does not answer (`-32601`). An unknown URI is
  JSON-RPC `-32002` (resource not found); a missing `uri` is `-32602`.
  The `query` tool has its help listed as one `text/plain` resource, `kbase://help/query`, named
  `query help`: its
  operations, listed from the commands it serves, then what they share, stated there as MCP's
  arguments spell it, in `kbase --help` as the command line spells it, and in no operation's own
  help — how a query reads the KB and answers. `kbase://help/write` is `kbase write`'s own help, below, which carries the
  batch contract in full, as `kbase --help` does ([page §4.2](NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)). Each command a tool binds, an
  operation's and a write op's included,
  then has its help listed as one `text/plain` resource with URI
  `kbase://help/<command>`, the command's name in place of `<command>`, named `<command> help`: the command's shared help
  body, its argument list and its MCP note, the rendering [page §4.2](NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) states. The argument list is
  generated from the arguments its tool takes — a write op's is an
  entry's keys, `kbase write`'s the batch's — and its `--help` holds the same list, spelled as the
  command line spells it. These are KB-independent text, as the model guidance is: listed
  whether or not a KB is found or opens, and the same in every project.
  Nothing else is served, a register page included: the KB's documents are the external agent
  harness's own files.
- **What the server never does.** It sends no sampling request and offers no prompts; no inference
  runs from any tool but `build` and `score`, and each tool's inference is its own — the build's
  decision and text calls, the score run's scoring calls. `models` reads the
  selected provider's model catalogue; `configure-providers` reads the catalogue of the entry it
  writes where a key is stored for it, and `configure-models` checks each reference it writes
  against its provider's catalogue ([page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md)). None of them asks a model anything. It writes nothing but JSON-RPC frames to stdout; diagnostics go to stderr
  and `--log-file`. It loads no configuration itself — the verbs it exposes load their own on each
  call (§9.3). The `build` and `score` children it starts inherit the server's environment and
  `--config-dir`.
- **Protocol.** One MCP revision, the one the server implements, pinned as one constant
  (ARCHITECTURE §12); an initialise naming another revision is answered with the server's for the
  client to decide; calls are answered one at a time, in order; EOF on stdin ends the server.
- **Instructions.** `initialize` always carries `instructions` in each mode: the eight items of
  [page §4.1](NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md), in its order and with its quoted wording. The `.index/` rule and the session rule are
  the text the stamped `AGENTS.md` also carries ([§3](#stamped-documents)), and the edge-direction sentence the text
  `kbase --help` also carries. Every tool they name is one the server serves.
- **Registration** is the external agent harness's: one server entry, `kbase mcp` (located, run
  from inside the project) or `kbase mcp --project-dir-required` (not located, for a harness that
  serves many projects from one place). README gives entries for two external agent harnesses as
  examples.

---

## 12. Interim releases

**<release version> — the files configure inference; the served surface is smaller.**

- **Every command's help is an MCP resource**, `kbase://help/<command>`: its shared body, then its
  generated argument list, then its MCP note; `--help` is the usage line, the same body, the list
  as the command line takes the arguments (shorthands, repeated flags, a mapping's joined values),
  then the CLI note, which names the global flags ([page §4.2](NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)). The tool's schema states its arguments, each described, a
  flag's with the default its list states (§11).
  Each is listed as `<command> help`, and the `query` tool's shared help as `query help`.
- **`provider` and `[asks] readerConcurrency` leave `config.toml`.** A file still stating either
  fails strict load naming the key (§9.3); delete the line by hand — the configure commands cannot load
  the file to repair it.
- **`configure` is `configure-providers` and `configure-models`**, each a CLI command and a tool
  ([page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md)). `configure-providers <provider> --base-url <url>` (`--api`,
  `--decisions-flavor`) writes a `providers.toml` entry, and `--remove` removes one, a provider with
  no entry refused; it dials the provider where its key is stored, and otherwise names the next
  steps. `configure-models --decision/--text/--scoring <provider>/<model>` writes the roles given
  and leaves the rest. `--create`, `--project`, `--provider` and `--model-map` are gone, and with
  them the `detect` refusal. Both write `~/.config/kbase` by default, or the directory `--config-dir`
  or `$KBASE_CONFIG_DIR` names; `--local` writes `<repository>/.kbase` — the repository layer is no
  longer chosen because its directory stands ([§9](#the-configure-commands-write-location)). A created `providers.toml` holds the entry
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
  `--config-dir` or `$KBASE_CONFIG_DIR` (§9). The role record carries no `back-end`, and
  `show-config` no `mode` or `environment` (§7).
- **`find` replaces `weak-points` and `strength-below`; `edge-census` is gone.** `find --kind claim`
  lists claims between `--min` and `--max`, sorted by strength or by dependents, or with
  `--pending` the unscored ones ([§8](#find)); `edge-census`' counts are `stats`' `relations`, without the
  per-bearing manual and model split (§7).
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
  (§7, §8, §11).
- **`resolve-demoted` is `restore-demoted`, and takes no `action`.** It restores the edge a
  `demoted` record replaced, named as the record's `edges` row names it — `source`, `target` and
  `relation: demoted`; `remove-edge` removes the record instead. Its result carries no `resolved` key
  (§7, §8).
- **`render-claim-graph` is gone**: `update-format` draws the sheets and reports `sheet` and `dot` (§7, §8).
- **The queries are one MCP tool.** `query` runs one read-only query: `operation` names it
  from an enum and `arguments` carries what its own tool took; a bad argument is a usage refusal
  naming it and the allowed set. Every subcommand stands on the command line, and what the queries
  share is stated once, in `kbase --help` and in `kbase://help/query` (§11).
- **Every write is a batch.** `kbase write` and the MCP `write` tool take an ordered batch whose
  entries each name their op under `operation`; each write op's subcommand takes a batch of that op
  alone. A batch checks every entry's shape first, then applies the entries in order and stops at
  the first refusal; nothing is written unless all of it is, the summary pages and sheets included.
  `dry-run` (`--dry-run`) reports a batch's result and writes nothing, an empty batch re-derives the
  summary pages and sheets, and `--no-refresh` is gone. A write's result carries `dry-run`,
  `entries` — each entry's op and outcome, an add's id among them, in place of `ids` — `written`, `removed`, `sheet` and `commits`, and no `refreshed` (§7, §8,
  §11).
- **A write batch, and `update-format`, can commit `kb-root/`.** `commit: true` (`--commit`) makes
  git the batch's undo: before writing, the batch commits `kb-root/`'s uncommitted changes outside
  `.index/`, then those inside it, each alone, and after writing commits the batch itself, labelled
  `kb-write: <ops> <ids>`, so reverting that commit undoes exactly the batch. A failed pre-flight
  commit writes nothing; a failed batch commit leaves the batch written, `done`, with
  `uncommitted`. `update-format --commit` commits the conversion as one commit. `commits` lists
  each commit's `commit`, `label` and `paths`; committing is never the default. Where the `.git`
  beside `kb-root/` is not the one git reads as the worktree root, every command that runs git over
  the KB — a committing batch, dry run included, `update-format`, `init`, `build`, `status`,
  `cancel` and `wait` — is refused `check: worktree` before it writes (§7, §8).
- **A `rigor` above `1.00` in the node records is a load error.** The load refuses it as it
  refuses `.inf`, naming the record and the field (§10); no write op writes one.
- **KB format `4.0.0`'s records.** Each scored record carries its `judged-hash`, an experiment's
  `status` is `run` or `planned`, and no record carries `no-edge` or `strengthen-by`; a `3.0.0` KB
  converts at load, a warning quoting each dropped text (§10). `gated-on` is gone, and `show` and
  `stats` carry no strengthen-by keys (§7, §8).
- **KB format `4.0.0`'s markers, anchors and summary pages.** A statement marker is
  `<!-- kbase-id: <id> -->`, attached with no space; a heading's anchor follows GitHub's slugger, a
  repeat suffixed `-1`, `-2`, so a canonical anchor can differ from the `3.0.0` one; the register
  pages are `kb-summary.md`, titled `# KB summary`, each entry's heading carrying its node's marker.
  A `3.0.0` KB's markers convert at load and its `claim-rigor.md` pages are removed by
  `update-format` (§10).
- **KB format `4.0.0`'s citations.** A citation is a Markdown link whose title is the cited node's
  id, checked as every link is; a bare node id in prose is a `verify` fault, `bare-id`, and the
  build's citation-grammar check is gone. `add-citation` writes the id of the node it cites as the
  title. A `3.0.0` KB's links
  convert at load: each into a heading re-anchored, each resolving to one node titled with its id, a
  work's pointed at its root summary entry, each bare id made its citation, and each it cannot
  resolve reported (§8, §10).
- **A KB's load no longer degrades.** One edge identity on two records, an edge naming a node no
  record carries and a cycle among the scoring edges are load errors, `check: metadata`, refused by
  every command that loads the KB, `verify` and `update-format` included; recovery is in git, returning the
  record files to a consistent commit and reapplying the other side's changes through write ops.
  `update-format` repairs nothing and its result carries no `warnings`; `check: degraded` is gone
  (§7, §10).
- **`update-format` replaces `refresh`.** It converts a KB in an older format, re-derives every
  register page and sheet and rewrites the maintained blocks, whether or not the format changed,
  and refuses a `kb-root/` with uncommitted changes (`check: dirty-paths`, naming each path). Its
  result adds `unfenced` and `commits` and, where the format changed,
  lists the conversion's minted work and axiom ids, bare-id citations, dropped `no-edge` texts and
  dropped `strengthen-by` texts under `ids`, `bare-ids`, `no-edge` and `strengthen-by`. `verify`'s
  `index-freshness` fault names `kbase update-format` as its remedy (§7, §8).
- **A conversion no longer rides on a write.** While a KB in an older format is unconverted, every
  write but `update-format` — a write batch, `score`, dry run included,
  and `build` resuming over it — is refused, `check: index`, remedy `kbase update-format`; reads keep
  answering as converted (§10).
- **`AGENTS.md`, `CONVENTIONS.md` and `README.md` hold a maintained block.** kbase rewrites only the
  text between the block's `vvv-kbase` and `^^^-kbase` marker lines; text outside it is the KB's own.
  A document without the pair is left untouched and listed under `update-format`'s `unfenced` — a
  KB stamped before this release holds none until its stamped documents are rewritten by hand or
  removed for `update-format` to write whole (§3).
- **The build records live in `kb-root/.build/`.** A build writes and commits
  `kb-build-node-pass.yaml`, `kb-build-classification.yaml`, `kb-build-unmarked.yaml` and
  `kb-build-cull.yaml` there, so its stage commits stay within `kb-root/`. Converting a `3.0.0` KB
  moves each record standing at the repository root into `kb-root/.build/` with `git mv`, and
  `update-format` refuses while a record it would move has uncommitted changes (`check:
  dirty-paths`, naming it) (§5, §6, §8).
- **`add-claim`, `add-analysis`, `add-experiment`, `add-work` and `add-axiom` add one node each.**
  Each writes its record with no edges and mints its node's id, for every kind, so no later entry of
  the same batch names it ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md));
  a claim's, an analysis's or an experiment's statement marker goes at the end of the line
  `document`, `line` and `snippet` name, with no space, and a line not containing the snippet is refused, naming the nearby lines that do.
  `add-work` takes the work's citation key as its `label`, and `add-axiom` a `label` of any one line
  of text; a work's or an axiom's label is unique within its kind. An add of a title its kind
  already holds — or of a label its kind already holds — is refused, `node exists`, naming that
  node; nothing is adopted, so a re-issued add is refused. `add-axiom` marks the one unmarked
  heading of its text in `axioms.md` or appends its marked heading, reporting which under
  `heading`, and refuses more than one unmarked heading of its text, naming their lines. An add
  whose summary entry shifts a standing entry's repeat suffix re-points every link into that entry
  ([page `add-axiom`](NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-axiom.md), §8).
- **Every node id is minted, and a work's or an axiom's label is a record field.** A work record
  carries `label`, its citation key, and an axiom record `label`, any one line, after `id`; each
  axiom heading in `axioms.md` ends in its axiom's marker ([page §1.1](NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)). Converting a `3.0.0` KB mints
  each work's and axiom's id from its `3.0.0` id alone, the old key or label its `label`, renames
  every place the old id stood, marks each axiom heading naming a record and reports each naming
  none; `update-format`'s result lists each conversion under `ids` (`from`, `to`)
  ([page `update-format`](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)).
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
  line it rewrote under `rewritten` (§7, §8).
- **`add-edge`, `edit-edge` and `remove-edge` write one edge each,** named by `source`, `target` and
  `relation` and reported by them. `add-edge` writes any relation but `demoted` between the kinds it
  joins — a pairing it does not join is refused, `allowed` the relations that do — with a `bearing`
  on the scoring relations, `*pending*` where left out, and a `context`; an edge that stands is
  refused, `edge exists` at `relation`, and one closing a cycle is refused naming it; over a demoted record of the
  edge it writes the add's values and deletes the record. `edit-edge` changes a standing edge's
  `bearing` or `context`, a bearing given marking it manually scored, `unchanged` where the values
  and the judged hash stand. `remove-edge` removes one record, or with `relation: demoted` every demoted record from
  `source` to `target`; an edge no record holds is refused, `no edge` at `relation`, so a re-issued add or removal
  is refused (§7, §8).
- **`restore-demoted` refuses what it cannot restore and reports what it restored.** An entry no
  demoted record from `source` to `target` answers is refused, `no edge` at `relation` — a re-issued restore, an
  edge standing with no demoted record and a batch's repeat of an entry among them, each formerly
  `unchanged` or a shape refusal — and its result entry carries `restored`, the relations put back
  (§7, §8).
- **`remove-node` removes a node of any kind with every record naming it.** It takes `id` alone and
  removes the node's record, its statement marker or an axiom's `axioms.md` heading, every demoted
  record naming it and every edge naming it at either end, its result entry listing those edges
  under `removed-edges` as `edges` rows; a dry run previews them. A citation of the node, named
  `path:line`, a work's label carried by a citation span no reference-list entry has the key of,
  named `path:line`, or its mention in a surviving record's title, rationale or edge context
  refuses the removal, an `edit-<kind>` or `edit-edge` entry earlier in the same batch clearing the
  latter. A removal that shifts another summary entry's or `axioms.md` heading's repeat suffix
  re-points every link into it; an `id` no node carries is refused, `no node`, so a re-issued removal is refused (§7, §8).
- **`move-node` moves a claim's, an analysis's or an experiment's statement marker.** It places the
  marker at the end of the line `document`, `line` and `snippet` name, refused as an add refuses
  that line, takes it from wherever else it stood, and re-points every citation of the node to its
  new host and section without touching the citing words; a move to where the marker stands is
  `unchanged`, and a work, an axiom and a node with no marker are refused (§7, §8).
- **`add-citation` turns written text into a citation of a node.** It replaces the one span of the
  line `document`, `line` and `snippet` name with a link to the node's leaf and section anchor —
  an axiom's heading, a work's root summary entry — written relative to the document, the id its
  link title: a quote checked verbatim against the node's statement, a paraphrase whose `quote` is,
  or, where `snippet` is the id, the naming form, the one form a work or an axiom takes. A snippet
  the line lacks or holds twice, a quote absent from the statement, a node with no marker and an
  unbalanced `[` or `]` are refused; a
  citation written into a heading re-points every link, a citation or not, into a heading whose
  anchor it moves; its
  result entry carries `document`, `line`, `id` and `link` (§7, §8).
- **`citations-of` lists where prose cites a node,** found by the id in each link title, as `path`,
  `line` and `text` ordered by path, then line; its answer is complete, `count` and `results` with
  no paging, and an id no node carries is refused, `no node` (§7, §8).
- **`show` refuses an unknown id as every op does,** `no node <id>` with its remedy ([§8](#show)).
- **A refusal over MCP that names a document keeps its `line` and `column` there;** one naming no
  `path` still carries neither (§11).
- **`find` finds nodes of every kind by filters, and returns its whole answer.** Its `<query>`
  argument is gone: `--kind` (repeatable), `--title` (a case-insensitive substring of the title
  alone, no longer the anchor), `--label` (a case-insensitive substring of a work's or an axiom's
  label), `--path`, `--min`, `--max`, `--pending` and `--min-dependents` narrow it; a row is `id`,
  `kind`, `title`, `label` (null for a claim, an analysis or an experiment), `host`, `strength`
  (`*pending*` where pending) and `dependents`, with no `build_band`. `--sort id|strength|dependents` and `--order
  ascending|descending` order it, and `--limit N` keeps the first N, or for a negative N the last
  N, allowed only beside `--sort` and never 0; `--offset` and the result's `offset` and `truncated`
  are gone (§7, [§8](#find)).
- **`show` names where a node is stated and summarised.** Its record carries `host`, `host_anchor`
  and `summary` (`<page>#<anchor>`) in place of `canonical_path` and `canonical_anchor`, a claim's
  carries no `citation_count`, an analysis's and a work's carry their `rationale`, and a work's and
  an axiom's their `label` (§7, [§8](#show)).
- **`stats` carries no `citation_edges`, and `subtree_aggregates` is per volume:** each volume's
  `claims` and `experiments`, in place of one count of index pages. Its help names the `find`
  question behind each ranked section (§7, [§8](#stats)).
- **`edges` lists one node's edges, and returns its whole answer.** `--id` with `--direction
  in|out|both` (`both` by default) keeps the edges at a node, an `--id` no node carries refused,
  `no node`; a row is `source`, `target`,
  `relation`, `bearing`, `context`, `manually_scored` and `replaced_relation` — the relation a
  `demoted` record replaced, null on every other row — with no `walked`, and `remove-node`'s
  `removed-edges` rows follow it; `--limit`, `--offset` and the result's `offset` and `truncated`
  are gone (§7, [§8](#edges)).
- **`trace` follows the scoring edges from a node, transitively.** `trace <id> --direction in|out`
  lists every `supports`, `founds`, `lifts` and `refutes` edge that bears on the node's strength or
  that its strength reaches, each an `edges` row with its `depth`, its source's `strength` and its
  `contribution`, the edge's input to its target's strength as the derivation computes it.
  `--sort depth|contribution`, `--order` and `--limit` order and cut it as `find`'s do (§7, [§8](#trace)).
- **Help, schemas and results name an argument as the surface takes it**: `--name` at the command
  line, `name` over MCP, set off in backticks inside prose; over MCP a kbase command line in a
  result or remedy is the tool call. A command line's flags come in name order, so `resume` reads
  `kbase build --no-inference --resume …`; a "no node" remedy reads "list the nodes that stand with
  `kbase find`, correct `<id>`, then re-run" ([§7](#item)).
  A remedy naming a write batch reads at the command line as its values document piped in,
  `printf 'entry: []\n' | kbase write`, and over MCP as the `write` tool's call, its entries
  included.
- **A value set records the hash of what it was judged against.** A rigor or a bearing an add or an
  edit is given — `*pending*` included — and every value `score` lands record the record's
  `judged-hash` over the node's statement, or an edge's two ends' statements, with the ask
  template's digest; a value set otherwise keeps it ([§8](#edit-kind)).
- **`score` asks only what is dirty.** A run walks each item whose judged hash differs from the
  one its record carries, a never-judged value included; a second run over a KB it scored asks
  nothing. `--pass` is `rigor`, `bearing` or `all` (was `both`); `--node <id>` and `--edge
  <source>,<target>,<relation>` (over MCP a mapping) score one item, dirty or not, and are refused
  beside `--pass`, on a value no pass asks, and on a manually scored item without
  `--rescore-manual`, which is now a bool: given, a full run also re-judges every manually scored
  value of its pass. `--limit` is gone — the scope is the budget — and so is the `bounded` outcome
  ([§8](#score)).
- **`score` lands a value only against the text it was asked about.** A value whose item's text
  was edited after its call was composed — a statement edited by hand between a stop and its
  `--resume` — lands nothing: its ledger line is `failed`, reason "its text was edited after it was
  asked; the value is discarded and a resume asks it again", and the resume asks it again. A value
  whose node or edge is no longer in the KB lands nothing and is `skipped`, reason "the node or edge
  is no longer in the KB; its value is discarded and a resume does not ask it again": it is no
  failure, and leaves nothing to resume. Each ledger line records `judged`,
  the hash its call was composed under ([§8](#landing)).
- **`score --resume` is a bool.** It continues the repository's most recent score run with the
  scope its `run.yaml` records, `rescore_manual` among it, and takes no scope argument beside it;
  it is refused where that run finished or a build has started since, remedy a fresh `score`. A
  result's `resume` is that call, naming no run id and no scope ([§7](#item), [§8](#the-closing-record)).
- **`score --dry-run` lists what a run would ask.** Its document carries `ask`, the items a run
  would ask, and `skip`, the dirty manually scored values it would skip, counted under
  `counts.remaining` and `counts.skipped` ([§7](#item)).
- **`score` lands through the edit ops.** Its values land as one mixed batch of `edit-claim`,
  `edit-analysis`, `edit-experiment` and `edit-edge` entries with the mark `false`, followed by
  the derivation, so the derived files are current whenever the run exits; the run no longer
  refreshes after its last write, and a node's text it asks about is the statement `add-citation`
  quotes, its markers and blockquote prefixes removed ([§8](#landing)).
- **The `score` tool reports a run still starting after 5 s** as `done` with `state: starting`,
  `state-dir`, `pid` and `wait`, for `status` to follow, in place of a failure after 40 s (§11).
- **The `build` tool reports a run still starting after 5 s** the same way, in place of a failure
  after 40 s (§11).
- **A fresh `build` is refused wherever `kb-root/` exists**, a spine-only or skeleton one included,
  with `init`'s refusal: `check: kb-root`, no remedy. A fresh build over a trail whose `kb-root/`
  was removed starts a trail of its own, so removing `kb-root/` is how to start over; a
  `--resume` with nothing to resume while `kb-root/` stands names that remedy, `check: kb-root`.
  `status`, `build-trail` and a resume read no trail a later removal of `kb-root/` ends; with
  `kb-root/` absent, `status` reports `none` and `build-trail` no boundary ([§5](#starting-over), [§8](#init)).
- **`build-trail` replaces the `rewind-build` tool and the `kbase://build-trail` resource.** It is
  a subcommand and a tool, takes no target, and gives each boundary its own command — `git reset
  --keep <commit>` then the build resumed with the flags its last run was launched with, the bound
  excepted — and `started-at` the reset alone; `git reset
  --keep` refuses rather than discarding uncommitted changes (§11).
- **`status` lists each stage done or not done, without commits**: `stages` holds `done`,
  `not-done` and `rewind`, the line naming `build-trail` as where each commit is ([§6](#kbase-status)).
- **No resource is subscribable.** `kbase://build` and its end-of-run notification are gone, and
  `initialize` declares no `subscribe`: a session polls `status`, or runs the `wait` command `build`
  and `score` return (§11).
- **The KB a build writes holds every title unique within its kind**, a repeated title taking a
  counter, ` (2)` on; the build's writes go through `add-claim`, `add-work` and `remove-node`, each
  followed by the derivation, and a link wrapped around a cross-reference is unwrapped to the
  cross-reference ([§5](#output-and-placement)).
- **`verify`'s checks are named in kebab-case**: `dead-link`, `unknown-id`, `index-freshness`,
  `statement-marker`, `axiom-heading` and `register-freshness`, beside `bare-id`, `frontmatter` and
  the `demoted` finding; `add-citation`'s and `move-node`'s refusal of a node with no statement
  marker is `check: statement-marker` ([page §2.8](NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md), [§8](#verify)).
- **`verify` checks every link's fragment.** A fragment into a leaf, `axioms.md` or any other
  Markdown document under `kb-root/`, a bare `#fragment` into its own document included, is a
  `dead-link` fault where it names none of that document's heading anchors or element ids, as a
  fragment into a summary page already was where it names no entry; the build's gate stage runs the
  same check ([page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md), [§8](#verify)).
- **A stale summary page's remedy is the empty write batch,** `entry: []`, which re-derives every
  page, in place of `kbase update-format`, which stays its remedy over a KB in an older format,
  where the batch is refused; a `bare-id` fault's remedy is a write batch whose `add-citation`
  entry cites the node, the id as `snippet` ([page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md), [§8](#verify)).
- **`export-latex --volume` names a volume, a top-level directory of `kb-root/` holding an
  `index.md`.** One that is not a volume is a usage refusal whose `allowed` lists every volume; one
  naming a volume the entry point does not list is refused as before. The report lists every form
  the export simplified ([page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/export-latex.md), [§8](#export-latex)).
- **A provider name is case-insensitive.** `--provider`, a model reference's provider part,
  `auth add`, `auth remove` and each `providers.toml` table name are folded to lowercase, and every
  output names a provider so. A repository entry now overrides the global one of the same name in
  any case, where a case-only difference across layers used to refuse the load; two tables of one
  file differing only by case still refuse it ([page §2.7](NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md), [§9](#merging)).
- **`auth remove` of a provider with no stored key is refused**, its remedy `kbase auth list`, where
  it was `unchanged` ([page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/auth-add-auth-list-auth-remove.md), [§9](#provider-commands)).
- **`probe` is a read-only tool**, taking the command's arguments; its MCP note makes each call, a
  paid model call, one the user directs ([page §3.7](NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/probe.md), [§8](#probe)).
- **`show-config`'s layer keys are kebab-case:** `providers-exists` and `config-exists`, where they
  were `providersExists` and `configExists` ([§7](#item)).
- **The `[models]` slots are named roles everywhere**, `kbase://model-hints` included, where the
  guidance said categories ([page §2.1](NEW_MCP_CLI_DESIGN/2-conventions/2.1-naming.md)).
- **The instructions carry eight items**: where the project is, the `kbase` skill, the `.index/`
  rule, the session rule — which now says what a score run judges and that a work's rigor and a
  `founds` bearing stay pending until the user gives one — how an edge reads, which `kbase --help`
  also states, `commit` on every write batch, the sheets as pictures for people, and a key command
  run as kbase prints it ([page §4.1](NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md), §11). `AGENTS.md`'s maintained block carries the new
  session rule once `update-format` rewrites it.
- **A server that locates no project serves as `--project-dir-required` does**: `project-dir` is
  required in every schema, and the instructions say how a session gets the path, where such a
  server used to leave `project-dir` optional and refuse a call omitting it ([page §4.3](NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md), §11).
- **`kbase --help` and `kbase mcp --help` name the CLI-only commands**, `auth`, `wait` and `mcp`
  ([page §4.4](NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md)); the root help's overview names the commands as they now stand, `score`,
  `wait`, `build-trail`, `init`, `export-latex` and `probe` among them.

### 0.9.0 — Messages is the default chat protocol

A `providers.toml` entry silent on `api` speaks
the Anthropic Messages protocol; `api = "openai"` selects the OpenAI-compatible protocol, and an
entry that relied on the old default states it (§9.1).

**0.8.0 — KB format `3.0.0`: the graph as records.** A KB's nodes and edges are records in two
authored files, `.index/nodes.yaml` and `.index/edges.yaml`, and everything else is derived at load
(§10). A `2.0.x` or `1.0.0` KB converts at load, answers every command as converted, and is saved by
the next natural write, which writes `AGENTS.md` and `CONVENTIONS.md` whole from the current
templates; the `migration` record gains `warnings` (§7, §10).

- **Statement markers host every claim, analysis and experiment**, one each; the build marks every
  claim it mints, and `mark-claim-in-leaf` places or moves any node's marker (§5, §8, §10).
- **Register pages are rendered, never read.** A node's page is its host's volume's; `refresh`
  renders the pages and `verify` checks them byte for byte, and a prose link into one has its
  fragment checked (§8, §10).
- **A person's value stands.** A rigor or a bearing a caller supplies is marked `manually-scored`,
  and `score` skips it unless `--rescore-manual` names its pass (§8).
- **Inserts adopt by title and host**; the write ops' `--create` and `set-frontmatter` are gone, and
  `insert-axiom-entry`, `set-status` and `add-pairs` are new (§8).
- **Cycles are cut one edge at a time, by one walk** the build, the load and `refresh` share; a
  `demoted` record keeps the edge it replaced whole and carries no origin, and the sheet draws one
  style per relation (§3, §10).
- **The load is strict on syntax and lenient on integrity**: a duplicate edge, a dangling edge and a
  cycle — what a merge leaves — degrade the load rather than refuse it, and `refresh` repairs them,
  reporting each under `warnings` (§8, §10).

**0.8.0 — a second chat protocol: the Anthropic Messages API.** A `providers.toml` entry may state
`api = "anthropic"` to reach an Anthropic Messages endpoint directly, with `api = "openai"` (the
default) behaving as before (§9.1). `models`, `configure`, `kbase probe`'s chat selections, a build's
overview call and a scoring run's asks all reach it; the `decision` role's multichoice asks and
`kbase probe --decision` do not, because the decision surfaces are OpenAI-shaped, and a Messages
endpoint asked for either fails naming the provider and the role (§8). Messages requires a
completion cap, sent as `max(256, ceil(characters / 4))` over the ask's text, and has no
prompt-cache key, so a scoring run against such an endpoint sends none (§8).

### 0.8.0 — the rigor ceiling is `1.00`

The rigor tables' maximum, the most a write carries, and
an axiom's rule value are one number: a rigor pass's maximum is `1.00`, so a text charged with
nothing prices at `1.00` (§8), and an authored `rigor: 1.0` is in domain and reads as itself (§10).
No charge means *this leaves nothing*: both rigor vocabularies price every finding at fifteen
hundredths of the maximum and the bearing table every finding at a tenth, so nothing one charge
says can price a text or a link at nothing. Zero takes seven rigor findings, or ten bearing ones.

**0.8.0 — `score` as a tool, the provider as its one back end, and one operation lock.**

- **A scoring run reaches its model through the provider.** Every ask is one chat call on the
  `scoring` role's endpoint, in either mode (§8).
- **`score` is a builtin tool as well as a CLI verb.** A tool call starts the run detached and returns
  its pid, state store and ledger; `status`, `cancel` and `wait` cover a scoring run as they cover a
  build, and a run at a terminal blocks as before (§11).
- **One operation lock.** `kbase-operation.lock` in the repository's git directory is held by a build
  or by a scoring run, so the two never write the same KB at once. A write op or `refresh` is refused
  while either holds it, and the refusal names which run and its pid (§6).

### 0.8.0 — a scoring charge cites line numbers

A call numbers the lines of the text it carries, and
a charge names the line where the fault shows instead of reproducing a span of it: selecting a
numbered line is a capability a model has, and reproducing a span byte for byte is not (§8).

- **The call numbers its lines.** Each passage is wrapped at word boundaries to at most 100 columns,
  its own line breaks kept and nothing rewritten, so a numbered line is a verbatim slice of the
  passage it came from and one paragraph still charges once per occurrence. `N| ` before a numbered
  line is the frame's gutter and no part of the text. The frame's labels and an edge's `## Context`
  carry no number, so the note is no site a charge can cite.
- **A charge names one line or a contiguous range within one passage**: `flt-inapplicable: 12`,
  `flt-transfer: 12-14`. A charge naming no line, or a line the call does not show, fails the attempt
  as a malformed reply does, and so does a range crossing a passage or a gap.
- **The ledger records the quote and the numbers.** `faults` keeps `citation` a span of the carried
  text, now quoted by kbase, and gains `lines`, the answer's own numbers. The `both` passage value is
  no longer produced — a numbered line lies in one passage — while a line already written with it
  keeps the meaning it was written with, so the format version does not move.

# Scoring schemes

