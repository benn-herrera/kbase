# SPEC – kbase

The contract: what any compliant implementation of kbase must do. Consumer-facing outcomes only.
[ARCHITECTURE.md](ARCHITECTURE.md) states how this implementation meets it; [THESIS.md](THESIS.md)
states why the build has its shape.

Every statement here is decided; where a point was once left open, its ruling is written in place.

---

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
  pages `refresh` renders from them (the format of §10).
- The **load-bearing forms** are the Markdown shapes KB readers key on: the up-link line, the
  cross-reference anchor form, the `` ``` math `` fence, the citation span, and the labelled
  blockquote.

Input is LaTeX only; no other input format is in scope.

---

## 2. Given interfaces

| Interface | Observed version | Contract |
|---|---|---|
| pandoc | 3.12 (`pandoc-api-version` 1.23.1.2) | Required on the host, not shipped. |
| Graphviz `dot` | Any version | Optional on the host, found on `PATH`, not shipped. Draws the claim-graph sheets (§3); absent, `refresh` writes the placeholder at the root only and its result says so (§3, §7). |
| git | 2.56.0 | Required on the host. The recoverable record of a build (§6). |

**pandoc.** The accepted range's lower bound is 3.12. Its upper bound is the next major
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
| Statement markers | The marker reader; `mark-claim-in-leaf`'s locate and splice |
| `.index/format.yaml` | `kbload`'s stamp read — the version is the bytes the file holds — and every save, which rewrites it only where its bytes change |
| Register pages | `verify`'s freshness check (`register freshness`: each page rendered in memory and compared with the bytes on disk) |
| `.index/nodes.yaml`, `.index/edges.yaml` | The write ops' idempotence: whole files rendered deterministically |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | Refresh's precheck of the stamped documents (`check: agents-redirect`) |
| The `` ``` math `` fence opener | The inventory's display-maths fence reader |
| The up-link line | `kb.UplinkMarker` and the readers that key on it |
| The claim-graph sheets' DOT, against the DOT goldens (`internal/sheet/testdata/`) | The sheet writer's write-only-where-bytes-change rule: under the same `dot`, `refresh` leaves an unchanged sheet's bytes as they are |

**Stamped documents.** `AGENTS.md`, `CONVENTIONS.md` and `README.md` give the maintenance commands:
kbase's subcommands. They point a reader at kbase's tools, never at its records: `AGENTS.md`
forbids reading `.index/`, the claim-graph sheets and any `.dot` file, and states the rule that a
session sets a rigor or a bearing only to a value the user gave, as the MCP server's instructions do
(§11). A register page is the one exception: `refresh` generates it as prose for people and
sessions to read, and nobody takes a value from it or edits it. `CONVENTIONS.md` states that the
records and the statement markers are written only by a write op, never by hand. `AGENTS.md` and
`CONVENTIONS.md` are kbase's: a migration save writes both whole from the current templates (§10).

**Formats.** A new KB is written in the current metadata format, `3.0.0` (§10). A KB in a
superseded form, `2.0.x` or `1.0.0` (§10), is read through migration.

**Theorem numbering** is whatever pandoc prints.

### `claim-graph.svg`

A picture of the claim graph, drawn through Graphviz `dot` (§2). `refresh` renders every sheet on
each run and write one only where its bytes change. The SVG's bytes are
Graphviz's; no check reads them for their content — the refresh test writes one DOT twice through
the same `dot` and asserts identical bytes, the write-only-where-bytes-change rule's basis — and no
other command's behaviour depends on them (ARCHITECTURE §6).

**Three kinds of sheet.**

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

**What is drawn.** The four scoring relations — `supports`, `founds`, `lifts` and `refutes` — each in
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
`kb-root/claim-graph.svg` does not exist `refresh` writes a placeholder there — an SVG naming the
missing tool and, beneath it, `index sha256:` followed by the first 12 hex digits of the SHA-256 of
the `.index/` files — the two record files — concatenated in sorted path order, deterministic.
`refresh` says so in one line on stderr, and its result names the lack (§7).

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

**Output and placement.** The KB is written only at `<git root>/kb-root/`. Build state is never
written into `kb-root/`.

**Fresh versus resume** is derived from the `kb-build:` commit trail (§6), never configured.
`kbase build <volume-root>...` starts a build over an empty trail; `kbase build --resume` continues
the trail. A build against a populated `kb-root/` with no `kb-build:` commit trail is refused.
`build` refuses outside a git worktree, naming `git init`. It refuses if the kbase-owned paths it
commits are dirty and the dirt is not an interrupted stage's own uncommitted work (§6), and only
then.

**Inputs.** A relative volume root or `--bibliography` resolves against the project root. A build
orders its volume roots, and its `--bibliography` files where given, by full path as text, whatever
order they were given in; every stage reads them in that order and the trail records it (§6).
`--resume` takes no inputs: it continues with the volume roots and `--bibliography` files the
trail's newest boundary records, in the order recorded. Before it resolves a state store, `build`
refuses, `check: usage`, a `--resume` given a volume root (`key` `<volume-root>`) or a
`--bibliography` (`key` `--bibliography`), one item per argument kind, and a build given neither
volume roots nor `--resume` (`key` `<volume-root>`).

**Starting over.** A trail's start-over commit is the `started-at` its `start` boundary records
(§6); for a trail that records none, the parent of its `start` boundary; for one whose `start` is a
root commit, none. A remedy writes the commit out as its hash, shown here as `<started-at>`.

A build given volume roots over a non-empty trail is refused before anything is restored or
written: one item, `check: inputs`, `key` `<volume-root>`, `path` the repository root, `detail`
naming the inputs the trail records, remedy "continue with `kbase build --resume`, or start over:
`rewind-build`'s `started-at` target, or `git reset --hard <started-at>`" — without the
`rewind-build` clause where the trail records no `started-at`, and with "rebuild in a fresh clone"
in place of the start-over clause where there is no start-over commit. A different input set is a
fresh build, reached only by starting over; kbase has no command that rewinds. A finished build
re-run with volume roots is refused so; `kbase build --resume` over a finished trail is
`unchanged`.

`--resume` needs a coherent trail. Over an empty trail it is refused: `check: trail`, `key`
`--resume`, `path` the repository root, remedy `kbase build <volume-root>...`. Over a trail whose
newest boundary records no inputs it is refused: `check: inputs`, `key` `--resume`, `path` the
repository root, remedy "start over: `git reset --hard <started-at>`", or "rebuild in a fresh
clone" where there is no start-over commit. A trail that records a stage while leaving an earlier
stage in walk order unrecorded was written by a build walking another stage order, and a build over
it is refused before anything is restored or written: one item, `check: trail`, `path` the
repository root, `detail` naming the recorded stage and the unrecorded one before it, remedy "start
over from a commit before the trail".

**Determinism.** The same inputs and pandoc version give a byte-identical document graph: the tree
`document-graph` writes and the records. The claim-graph stages mint fresh node ids on every build —
unique within the KB, now and going forward — so a build is reproducible modulo
ids from `claims-declared` on, and no consumer may read anything into an id's value. The order in
which volume roots and `--bibliography` files are given does not affect the result: the same set
gives the same document graph, and the same KB modulo ids.

**`--no-inference`** is first-class. It drops the rows that spend inference, still walks and records
every stage, and states the rows it dropped. The run is a real KB built without those rows, not a
stopped walk. A trail a `--no-inference` build finished is a finished trail: `kbase build --resume`
over it is `unchanged`, and its dropped rows never run on it. A build that wants them starts over
from the trail's `started-at` (`rewind-build`'s `started-at` target, or `git reset --hard
<started-at>`) and is a fresh build without `--no-inference`. Which rows drop follows the classification on the row (ARCHITECTURE §4), never a stage
id. A row that spends inference in part is not dropped: its asks are skipped and the rest of the row
still runs, as for `depends-attributed`.

**`--through <stage>`** bounds the run at the named stage. A bounded run is not a failed one:
outcome `bounded`, exit 0 (§7).

**Inference preflight.** A build with a stage left to walk that calls a model refuses (§7,
`refused`) before the first stage it walks when no provider is configured (§9), naming the
configuration it lacks; its `detail` names `kbase show-config` as the diagnostic that shows how the
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

**Bibliographies.** With no `--bibliography`, the build passes every `.bib` in any volume root's
directory, in sorted path order, one set offered to every volume; `--bibliography` given replaces
that set, read in path order (above). **Degradation, not refusal.** An unreadable bibliography: the build continues without it
and reports the file.

**Under `--no-inference`** no `README.md` is written: the overview call is the dropped row, and the
README is assembled around its passage.

**Monitoring.** `build` runs in the foreground; an external agent harness backgrounds it. `kbase
status` and `kbase cancel` (§6) observe and stop it.

**What a build produces**: Markdown leaves; the index, up-link and cross-reference structure;
load-bearing forms; the claim graph's node and edge records, among them an analysis for each claim
whose bounds hold text beyond its statement proper; the register pages. The build's stages and what
each does are in ARCHITECTURE, "Stage table"; how each computes its product is ARCHITECTURE's.

**A build marks every claim.** Every claim a build mints is hosted in the call that mints it: its
statement marker (§10) stands in its leaf, a leaf's only claim and an equation included — an
equation's at the end of the line inside its display-maths fence that carries its `\label{…}`,
never on the fence's closing line; a block claim's on its locator's line. The analysis split
(below) moves a prose claim's marker to just after the sentence its title is drawn from, on the line
that sentence ends on. It hosts each analysis it mints by a marker on the first blank line of the
blockquote that is its statement, or at the end of the last line of its claim's paragraph (§10).
Where either line closes a fenced block, the marker goes at the end of the line before it.

**A build records no judgement.** Every scoring edge a build writes carries `*pending*` as its
bearing, and no stage writes a `rigor` or any other number into a score; every value a build writes
is `manually-scored: false` (§8). The judgement is a person's, through `set-rigor` and `set-bearing`,
or a scoring run's (§8). A claim a build attaches an edge to reads unchanged until that edge is
judged.

**Co-linearity cull (`claims-culled`).** The stage walks between `references-found` and
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

**Dependency attribution (`depends-attributed`).** A captured link carries no direction; only this
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

**Analysis split (`analyses-split`).** The stage walks between `depends-attributed` and
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

**Records and inference.** The reader and placement facts the claim graph needs travel as records in
the build state store, not in `kb-root/` (ARCHITECTURE, "Records"). Each inference pass is one
closed-set decision per call under its embedded template (ARCHITECTURE, "Inference seams"); no stage
exits on a model's opinion. The multichoice asks are each one `POST` to the decision surface the selected
entry names (§9.1) carrying a closed-set question (`choice` or `noul`) and answered by probabilities
over that set — no reply text is parsed; the overview call remains one tool-less chat request
answered from its prompt alone.

---

## 6. Build state, ledger and monitoring

**Git holds the recoverable record.** Each stage boundary is a commit in the user's repository, with
subject `kb-build: <stage-id> | <display name>` and a structured body. Every boundary's body ends
with the build's inputs: a `volume-root: <path>` line per volume root, then a `bibliography: <path>`
line per `--bibliography`, each in the order the build reads them (§5), each path absolute. The
`start` boundary's body also records `started-at: <commit>`, the commit the build began from: the
clean, ready-to-build state a fresh build over other inputs starts again from. On a branch with no
commit, a fresh build first makes one empty commit, subject `kb-start: the repository before the
build` — outside `kb-build:`, so no part of the trail — after every refusal it can give and before
its `start` stage, and records that commit as `started-at`. A commit carries `kb-root/` and the tracked build records, at the repository root:
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
op or `refresh` attempted while a build or a scoring run runs is refused (§8).
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
`cancel`, `wait` and `rewind-build` read the store the operation lock names while its file stands
and names one, else the default store; a new run never adopts it. A second run is `refused` with
`check: lock`, `path` the running run's state-dir, and no remedy, its `detail` naming the holder's verb and pid: "a
build (pid 4321) is running; its state-dir is …", or "a score run (pid 4321) is running; …". Where
the operation lock is held but names no state-dir yet, the second run gets `retry`; where it names
one whose holder record is not yet written, the refusal says a run is starting. Every lock is
advisory and exists on Unix only; on Windows `status` cannot see a running run and no lock-based
refusal or `retry` is given.

**The state store is versioned.** `<state-dir>/format` holds one integer, currently `1`, written
when a build first holds the store; a store with no `format` file is version 1. `build`, `status`,
`wait`, `cancel`, the MCP `build` tool and `rewind-build` (§11) refuse a store newer than this kbase:
`check: state-dir`, `key: --state-dir`, `path` the store, remedy "update kbase, or use another
--state-dir". A build converts an older store in place before it uses it; the conversion is
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
directory stands — a `--resume` naming a run no longer standing among them — removes nothing. A
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

Returns a YAML document with: state (`none`, `running`, `cancelled`, `interrupted`, `failed`,
`bounded`, `finished`); pid; timestamps; each stage recorded or not, with its commit; the current
stage's units done and total; recent refusals and fallbacks; the `--no-inference` flag; and the
resume command, absent where no identical re-issue continues the work (§7) and absent where the
recorded command does not read as a `kbase build` invocation — at least two elements, `kbase` then
`build`. A run whose record fails that supplies none, as a run that recorded none does. `state` and
`resume` are read from the newest run the build stands at: a refusal or `retry` that recorded no
boundary wrote nothing, so the run before it is the one reported. `interrupted`
is the one state the build never writes: it is derived from a run event with no end event and a
holder that is not live.
Progress is persisted to a file so a late or reconnecting monitor catches up. An inference stage's
units and fallbacks are recorded once its asks have all returned, so while they are in flight
`current` names the stage and its total with none done.

**A score run's status.** Where the store's holder record names a score run, `status` reports that run
in its own shape instead of the build's: `state-dir`; `state` (`running` while the record holds the
store, else `interrupted` where the run left no result document, or the outcome
its result document records — `done`, `bounded`, `cancelled`, `failed`, `refused`
or `retry`); `pid`;
`started`; `run`; `pass`; `counts` (`total`, `done`, `failed`, `skipped`, `remaining`, read from the
run's `items.jsonl`, the status `ledger.jsonl` last records for each, and, while the run walks, each
answered item's result file); `scored` (the items whose
value landed); `failures` (each failed item and the reason its line recorded); and `resume` (the command
continuing the run, where an item is not `done`). It is the run's own records re-read, not a second
document: `wait` prints the result document the run wrote beside its ledger. A store no live run
holds, and whose record names no score run, reports its newest run, whichever verb it was: the later
of the newest score run's start, read from its `run.yaml`, and the newest build invocation's, read
from the progress record — each the record the run already wrote. A run directory whose `run.yaml` is
absent or unreadable left no record of when it started and is skipped. Where the store records
neither run, or the two starts cannot be compared — a missing or unparsable stamp on either side — it
reports the build as above.

### `kbase cancel`

Signals the store's holder, a build or a scoring run as the holder record names it. The calls
in flight are abandoned with nothing written for them; the event is logged, the lock is released, and the run
process exits `cancelled`. Cancel costs only the calls in flight: an answer already returned stands — in a
build's answer cache, and in a score run's ledger; the run is resumable, and a
cancelled score run's `resume` is the command read from its result document. A store no run holds is
`refused`: `check: lock`, `path` the store's holder lock, detail "no run holds the state store".

### `kbase wait`

`kbase wait --state-dir <store> --pid <pid>` returns when the run `<pid>` is no longer progressing:
the store stands at another run, or at that run with a state it stopped at. It then prints the
`status` document for that store — the same outcome and fields — so a command run in the background
carries the build's terminal state when it exits. `--pid` is required and must be positive; absent
or not positive is a usage refusal. `--state-dir` defaults as `status`'s. It takes no lock, and
reads no commit trail until that final document.

---

## 7. Tool-result contract and exit codes

**Tool results.** Every subcommand, `models` and `configure` included, writes one YAML document to
stdout and nothing else. stderr is for humans. Keys are in fixed order; strings are quoted on emit;
a refusal enumerates every offending item. The `outcome` key is the contract. `--help` and
`--version` are human-facing and outside it; `mcp`'s stdout carries JSON-RPC frames (§11), though a
usage error on its own command line, before it serves, still prints a YAML refusal.

**Shared vocabulary.** Every result document is one YAML mapping, with keys in this order:

1. `outcome`.
2. `kb-root`: the absolute path. Present on every subcommand that resolves a KB, which is all of
   them except `models`, `configure`, `show-config`, `probe` and the `auth` subcommands; `init` omits it from results written
   while no `kb-root/` stands (§8).
3. `migration`: a conditional record — `from`, `to`, `note`, `warnings` (one item per thing the
   conversion dropped or changed that the converted files do not show: `path`, `line` where it
   names one, `detail`; `[]` when none) — present on the result of every verb that reads a KB
   through the loader on an older format version, and absent otherwise (§10):
   `refresh`, `verify`, `render-citation`, `export-latex`, the queries, `score`
   and every write op. It follows `kb-root` where the result carries both.
4. The subcommand's own keys, in the order its table gives.
5. Exactly one of these, chosen by `outcome`, and it closes the document: `refusals` for `refused`
   and `retry`; `failures` for `failed`; `cancelled` for `cancelled` — a mapping of `stage` and
   `unit` on a build, and of `item` on a `score` run (§8). `done`, `unchanged` and `bounded` close
   on no such key.

Keys are kebab-case. The one exception is query data under `results`, which keeps the query data's
own names verbatim (`strengthen_by`, `mentioned_ids`). The same fact always uses the same key:
`written` (files this call wrote, a path named once however many times the call wrote it — a
converted file a phase writes and a later phase rewrites is still one entry; kb-root-relative inside
`kb-root/`, absolute outside it; `[]` when nothing was written), `ids` (node ids, in entry order),
`count` (a total before any limit), `resume` (a shell command that continues the work), `state-dir`
(the state store's absolute path).

**Item.** Every refusal, failure and verify finding is one mapping with these keys in this order, a
key omitted when it does not apply; `detail` is always present, with at least one of `check`, `path`
or `key`:

| Key | Holds |
|---|---|
| `check` | The named check or refusal class: a verify check name, `error`, `write`, `usage`, `provider`, `detect`, `dirty-paths`, `include`, `metadata-key`, `metadata`, `lock`, `state-dir`, `inputs`, `trail`, `pandoc`, `unknown-id`, `kb-format`, `kb-root`, `worktree`, `export-dir`, `index`, `degraded`, `symlink`, `conversion`, `demoted`, `dependency-cycle` |
| `path` | The file the item is about |
| `entry` | 1-based index into the values' `entry` list |
| `key` | The setting the item is about: a values key, spelled as its configuration file spells it, or a flag or positional argument, spelled as `--help` spells it; an item refusing a value given for a named thing may carry that value as given, as the auth verbs carry the provider name |
| `line`, `column` | 1-based position in the values document or in `path` |
| `allowed` | The closed set the value must come from |
| `remedy` | The command that clears the item, where one exists (e.g. `kbase refresh`, `git init`) |
| `detail` | One sentence |

A refusal lists every offending item. A usage error is one item with `check: usage`. A
provider-selection refusal names `key: --provider` and lists every entry name as `allowed`; a
category `configure` is not given names `key: models.<category>` (`models.decision`,
`models.text` or `models.scoring`, `check: detect`) and lists the provider's model ids as `allowed`.

**Per-subcommand keys**, following `kb-root`:

| Subcommand | Keys |
|---|---|
| `build` | `state-dir`; `roles` (one record per role the build resolved, in role order — `decision`, then `text` — in the role record's shape below; absent where it resolved none, as under `--no-inference`); `through` (absent where this invocation walked no stage); `no-inference`; `resumed` (bool); `restored` (paths reset to the last boundary); `stages` (list of `stage`, `commit` — null if unrecorded — `dropped` (row ids), `report` (path to the stage's full findings in the state store); absent where this invocation walked no stage); `resume` (`kbase build --resume` and the flags the invocation carried that a resume still needs — `--no-inference` where given, `--ask-concurrency` where given, `--state-dir`, `--config-dir` and `--project-dir` where given, `--config-dir` written absolute — never the inputs or `--through`; absent where an identical re-issue continues nothing: `done`, `unchanged`, `bounded`, `retry`, a `failed` or `cancelled` run that walked no stage, and a `refused` run that recorded no boundary) |
| `status` | `state-dir`; `state` (§6's vocabulary, `interrupted` included); `pid`; `started`; `updated`; `ended`; `no-inference`; `stages` (list of `stage`, `commit`); `current` (`stage`, `units-done`, `units-total`); `recent-refusals`, `recent-fallbacks` (items, the newest 10 of each); `cache-entries`, `cache-bytes` (integers: the answer cache's entry count and total size in bytes, §6); `resume` (absent for `finished`, `running` and `bounded`; read from the run §6 names) |
| `cancel` | `state-dir`; `pid`; `resume` (absent where the record's command does not read as a `kbase build` invocation, §6) |
| `init` | `written` (every path this call wrote — the spine, the two empty record files, the stamped readiness documents and the refresh's output, merged, deduplicated, path-sorted); `removed` (as `refresh`'s; `[]` at birth) |
| `refresh` | `written` (includes every register page and sheet written); `removed` (files the KB no longer calls for — a register page no node is rendered on, a sheet the volume-count rule drops, a file a format migration made obsolete (§10) — kb-root-relative inside `kb-root/`, repository-relative beside it; `[]` when none); `sheet` (`drawn` where the sheets were rendered this call, else `placeholder`: nothing was drawn, whether or not a sheet stands; present only on `done` and `unchanged`); `dot` (`"not found on PATH"`, present only when `sheet` is `placeholder`); `warnings` (one item per record the repair of a degraded load dropped and per edge it cut (§10), `path` the record file and `detail` the record and why; `[]` when none) |
| `verify` | `findings` (items the KB's structure carries that are not faults — every `demoted` record (§10), as an item with `check: demoted`, `path` the register page of the record's `target`, omitted where that target has no host, and `detail` the edge and the relation it replaced; `[]` when none); its faults are `refusals` items, a refresh-fixable item carrying `remedy: kbase refresh` |
| every write op | `ids` (inserts only; minted or adopted, in entry order); `minted` (the subset this call drew); `adopted` (list of `entry`, `id`, `differs` — values keys whose supplied value the existing entry does not carry); `written` (paths this call wrote); `refreshed` (paths; null under `--no-refresh`); `removed` (the trailing refresh's, as `refresh`'s; `[]` under `--no-refresh`) |
| `resolve-demoted` | as every write op; `resolved` (list of `source`, `target`, `action`: `removed` or `restored`) |
| `render-citation` | `citations` (one per entry, in entry order) |
| `deps`, `gated-on`, `cited-by`, `find`, `referenced-by`, `strength`, `subtree`, `edges` | `count`; `offset`; `truncated` (bool); `results` (the query's list, in its own key names, sliced; `strength`'s records are a claim's node record (as `show`'s below, without `manually_scored` and `strengthen_by`) then `dependents`, the count of nodes its strength enters — `strength` null on a pending claim; `deps`' edge records, not `-i`'s ids, are `relation`, `direction`, `target`, `bearing`, `manually_scored`, `context` — the bearing as the index writes it, a number in `[0, 1]` or `*pending*` spelled as text, and null only where the pairing carries none; `manually_scored` the record's flag (§8), null where its record carries none; the context null where the edge has none; `edges`' records are `source`, `relation`, `target`, `bearing` (as `deps`' bearing), `manually_scored` (as `deps`'), `context` (as `deps`') and `walked` (bool)) |
| `show`, `stats` | `results` (the query's mapping; a node record carries `node_type`, `id`, `title`, `canonical_path` and `canonical_anchor`, then the keys its kind carries: `rigor` and `strength` on a claim, with `build_status`, `build_band`, `rationale`, `premise_count`, `strengthen_by_count` and `citation_count` after them; `rigor`, `strength` and `build_band` on an analysis; `status` and `rigor` on an experiment; `rigor` on a work; `rigor` on an axiom, its `1.0` by rule; `show` adds `manually_scored` (§8) after `rigor` on a claim, an analysis and an experiment; `strength` is a number rounded to two decimals for reading, `rigor` the number the node holds; `stats`' mapping carries one count per node kind — `claims`, `analyses`, `experiments`, `axioms`, `works` — then `edges`, `demoted_edges`, `strengthen_by_items`, `citation_edges` and `subtree_aggregates`, then `rigor_manual`, `rigor_model`, `bearing_manual` and `bearing_model` (the records whose type carries `manually-scored`, `*pending*` values included, counted by the flag), then `relations`, then `strength_bands` and its two ranked sections, whose records carry `strength`; `relations` carries one record per relation in this order — `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted` — each `edges`, the relation's row count, `walked` (bool), then on a scoring relation `bearings`: one record per distinct bearing its rows carry, `bearing`, as the index writes it, and `edges`; numbers ascending, then `*pending*`, then null, `[]` where the relation has no rows; `show`'s claim record adds `strengthen_by` after `strengthen_by_count`: the claim's strengthen-by items in `item_idx` order, each `item_idx`, `text`, `mentioned_ids` (`[]` when none)) |
| `models` | `provider`; `provider-rule`, `provider-layer` (the rule that named the entry — `flag` or `sole-entry` — and the layer that supplied it; this verb always selects one); `models` (ids, sorted ascending) |
| `show-config` | `layers` (one record per directory this invocation read, in precedence order: `path`, `source` (`explicit`, `environment`, `repository` or `global`), `exists`, `providersExists`, `configExists` — the directory's presence and each of its two files'); `providers` (one record per entry, in name order: `name`, `base-url`, `decisions-flavor` (the surface it speaks, `systemone` where the entry is silent), `key` (`file` or `none`), `key-file`, `layer` (the layer that supplied the entry), `usable`, `fault`; a provider a dropping fault removed has `name`, `layer`, `usable`, `fault` only); `config` (the merged choices: `models` (`decision`, `text`, `scoring` — each the reference `config.toml` states, null where unset); `sampling` (`text`, `scoring` — each `temperature` and `thinking` as `config.toml` states them, null where unset)); `selection` (`provider` and `rule` — `flag` or `sole-entry` — or `detail` where no entry was named); `roles` (one record per role in role order, in the role record's shape below, each with `base-url`, `sources` (`base-url`, `model`, `key`, and for a chat role `temperature`, `thinking` — the layer that supplied that field, one of `layers`' four names, null where the field is unset), a chat role's `temperature` and `thinking` (null where unset: kbase's default, §9.2) and `detail` where the role does not resolve); `diagnostics` (items: every fault, `path` the `providers.toml` that carries it, and the resolution failure where a role does not resolve) |
| `configure` | `provider` (the entry configured, null where the configuration names none); `provider-rule`, `provider-layer` (as `models`'s; absent where no provider resolved, as under `--create` with no `--provider`); `models` (`decision`, `text`, `scoring`, as `config.toml`'s `[models]` spells them, null for a category left unset); `written` (the files this call wrote — `providers.toml`, then `config.toml`, in the order they were written; `[]` with outcome `unchanged`) |
| `probe` | `provider`; `provider-rule`, `provider-layer` (as `models`'s); `selection` (the selection that answered: `decision`, `text` or `scoring` where a role flag or the default named it, else the `<provider>/<model>` reference `--model` named); `model` (the id asked); `temperature`, `thinking` (chat selections only: the effective settings — `temperature` a number; `thinking` `off`, `low`, `medium` or `high`); `first-token-ms`, `wall-ms` (integers: request start to the first non-empty content or reasoning delta — zero where none arrived — and the call's end-to-end time); `finish-reason` (null where the stream reported none); `reply` (the reply's text on a chat surface, and on a decision surface the answer's probability, chosen letter or type); `usage` (a record: `reported` — whether the provider sent a usage block at all, so a zero count is a measurement only where it is true — then `prompt-tokens`, `completion-tokens`, `reasoning-tokens`, `cached-tokens`) |
| `auth add`, `auth remove` | `provider` (the name given; never the key) |
| `auth list` | `providers` (the names with a stored key, in name order; `[]` when none) |
| `export-latex` | `written` (absolute paths; outside `kb-root/`); `volumes` (the volume directories exported, entry-point order); `report` (the path of the export report written under the out dir — one YAML list, every degradation and finding line the run produced, each naming the form, the source path and the reason) |
| `score` | `migration` (conditional, the shared vocabulary's; it stands before `state-dir`, and a run that starts on an older KB is the call whose start refresh saves the conversion); `state-dir`; `pass` (the set it walks: one pass, or `both`); `roles` (one record, `scoring`, in the role record's shape below; absent under `--dry-run`); `run` (the run's id); `ledger` (the run's directory, absolute); `counts` (a record: `total`, `done`, `failed`, `skipped`, `remaining` — the items by the status the ledger last records for each, so a resumed run reports the whole run); `scored` (the items whose value landed, in the walk's order — a node's id, or an edge's `<relation> <source> → <target>`); `resume` (the command that continues the run, carrying `--project-dir` where given, and `--config-dir` written absolute; present wherever an item failed or was not reached — a `skipped` item does not make it present — and it stands before the record the outcome closes on; absent from a dry run, and from a stop before the walk) |

A `status` read over a score run reports §6's score shape in place of the build keys above.

**The role record.** A verb whose endpoints are per role states them in `roles`, one record per role
it resolved, in role order — `decision`, `text`, `scoring` — with these keys in this order, each
omitted where it does not apply:

| Key | Holds |
|---|---|
| `role` | `decision`, `text` or `scoring` |
| `provider` | the pool entry's name |
| `provider-rule` | the rule that named it: `flag` (a flag overrode the reference) or `config` (the reference named it) |
| `provider-layer` | the layer that supplied the entry: `explicit`, `environment`, `repository` or `global` |
| `model` | the model id this invocation asked for; null where no source named one |
| `base-url`, `sources` | `show-config` alone: the endpoint, and the layer that supplied each field |
| `temperature`, `thinking` | `show-config` alone, chat roles: the role's sampling settings, null where unset (§9.2) |
| `detail` | `show-config` alone: why the role does not resolve, where it does not |

`provider`, `provider-rule` and `provider-layer` stand together on every role that resolved: a pool
entry supplies every endpoint. `provider-layer`'s `environment` is the directory `$KBASE_CONFIG_DIR`
names (§9).

**The model is the one requested, never one observed.** `model` is the id this invocation asked for,
and no result document or ledger line states one a server ran.

**Bounded results.** List queries take `--limit N` (default in ARCHITECTURE §12; `0` means all) and
`--offset N`; where `truncated` is true the caller re-issues the query with `--offset` set to
`offset + len(results)`. `stats`' ranked sections stay at 10, and the `bearings` of its `relations` hold
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

Every subcommand below is reachable by an external agent harness as a builtin tool — its own, or as
an operation of the `query` or `write` tool (§11) — `probe` excepted — a
tool would let an agent poll a model, which no KB operation asks for (§11); `score` starts a detached
child that asks one model call per item and returns at once with the run's identity, so the turn is
not held for the walk's duration (§11). The rules in this section govern each.

**Values input.** YAML (JSON accepted) on stdin or `--values`. The per-op key vocabulary is closed,
and stated once, in kbase's write package; it is not copied into prose. A refusal names the key, its
position, and the allowed set, and `kbase <op> --help` describes the op. A prose value containing
U+0008 or U+000C is refused.

**Idempotence.** Every mutating op is idempotent under re-issue: the same values issued twice leave
`kb-root/` byte-identical to issuing them once, and the second call reports `unchanged`. An insert
of a claim, an analysis or an experiment adopts by title and host: given a `document`, it adopts a
node of that title hosted there, else one of that title with no statement marker, which it then
marks there; given no `document`, it adopts only a node of that title with no marker; otherwise it
mints. No insert adopts by title alone, and an experiment is adopted only where its status, and
the rigor and the `supports`, `lifts` and `refutes` lists the values supply, also equal the supplied
ones; a rigor, a list or a pair's bearing left out is not compared. A work is adopted by its
citation key, an axiom by its label; a batch naming twice a key or label the KB does not yet hold is
refused at the repeat. An adoption that writes nothing is `unchanged`; one that marks an unhosted node is `done`. The one
exception is `init`: creation is birth-once (below), and a re-issue where a `kb-root/` stands
is `refused`, never `unchanged`.

**Writes leave nothing stale.** A mutating op ends with a refresh unless `--no-refresh` is given;
one that ends with a refresh refuses, before it writes anything, a `kb-root/CLAUDE.md` that is not
the redirect (`check: agents-redirect`, §3), which that refresh would refuse.

**One write lock.** A write op and `refresh` hold the KB write lock for the whole run: from loading
the KB through the trailing refresh and the sheets to the result. `init` holds it from before `kb-root/` exists — the lock sits on the directory `kb-root/` stands in, so it
is available at birth — through the skeleton, the trailing refresh and the birth commit to the
result. `score` takes it around its one landing — every value the walk produced, after the walk — and
around each of its two refreshes, never across the run (§8). A writer that cannot take it within 30 s gets `retry` with `check: lock`, `path` the
repository root, and the remedy. A write verb probes for a run holding the operation lock (§6) before
it waits on the write lock and again after taking it: where one runs it is `refused` with the `lock`
item at once, taking no wait. A build waits out a write in progress, up to the same 30 s, before it
writes (`retry` if it cannot); its own writes take no lock. A scoring run's landings and refreshes
make no operation-lock probe, writing as the run that holds it. `render-citation`, `verify` and the queries take
no lock. On Windows no
advisory lock exists (§6): neither the `retry` nor the run-running refusal is available there, and
one writer at a time is the operator's to keep.

**Batches** are all-or-nothing.

### `init`

```
kbase init [--title TEXT]
```

Creates a KB directly — no LaTeX, no build: the equivalent of `git init` for a KB. Run at any depth
in a git worktree; it resolves the repository root and works at `<git root>/kb-root/` (§1's
placement). **Refusals come before any write** (§7 item shape): a `--title` holding a line break
(CR or LF) — `check: usage`, `key: --title`: the title is the entry point's one heading line;
outside a git worktree — `check: worktree`, `path` the start directory, remedy `git init`, which
the tool never runs itself; and
against a `kb-root/` that exists in any state, an empty one included — `check: kb-root`, no remedy:
creation is birth-once, a re-issue where a tree stands is `refused`, never `unchanged`. The result of
a refusal that found no `kb-root/` carries no `kb-root` key; the refusal items name the paths.

Then, under the KB write lock from before the directory exists (§8): it writes `entry-point.md` —
with the `--title` text, or the repository's directory name, as the only heading, no children and
no frontmatter (§10) — the format stamp file, `.index/format.yaml` (§10), and the two record files,
`.index/nodes.yaml` and `.index/edges.yaml`, holding no record; stamps the readiness documents, `AGENTS.md`,
`CONVENTIONS.md` and the `CLAUDE.md` redirect, each only where absent, `AGENTS.md` pinned that this
KB is authored, not distilled — what it holds its author writes
into it, a claim at a time; then refreshes, so the derived metadata layer is present at birth
(§3's full sheet always, the placeholder where `dot` is absent, the digest and volume sheets
not drawn: no volume holds a node). Every input to the skeleton is kbase's own, so `init` runs no
`verify` of its own; `verify` (§8) over a freshly born KB is green. Finally the birth commit: `kb-root/` committed with the plain subject
`kb-init: knowledge base created` — never a `kb-build:` subject, so the trail of §5 stays empty and
`build`'s populated-`kb-root/` refusal (§5) stands over an initialized KB forever: content joins
§8's ops, never a later build. The commit rides the host's git identity and configuration; git's own
refusal (no identity configured) is a `failed` naming git's words, the written tree standing
uncommitted for the operator to commit or remove. Result keys: §7.

### Write ops

Each op's result keys are §7's.

### `resolve-demoted`

The op over a cut (§10). Values:
`entry` items of `id` (the node the replaced edge went into, the record's `target`), `target` (the
other end, the record's `source`) — each a claim, an analysis or an experiment — and `action`:
`remove` or `restore`. `remove` deletes the `demoted` record, leaving the edge cut; `restore` writes
the edge it replaced exactly — its relation, ends, context, bearing and `manually-scored` — refused
(`check: dependency-cycle`, `entry` the item's number, `detail` naming the cycle the restored edge
would close) where that would close a cycle, and refused where the pair has neither a `demoted`
record nor the replaced edge standing. A batch naming the same (`id`, `target`) pair twice is
refused whatever the actions: the item's `entry` is the repeat's number, its `key` is `target`, and
its `detail` names the first entry. Idempotent: a re-issue is `unchanged` — a pair with no `demoted`
record and `remove`, or with the replaced edge standing and `restore`. Runs the trailing refresh as
every write op does.

### `insert-claim-entry`
### `insert-analysis-entry`
### `insert-experiment-entry`
### `insert-work-entry`
### `insert-axiom-entry`
### `set-bearing`
### `set-rigor`
### `set-rationale`
### `set-status`
### `add-rests-on`
### `add-pairs`
### `mark-claim-in-leaf`

Each writes records and statement markers (§10), and none names a register: which register page
shows a node follows from its host. An insert writes a node's record and its edges in one act;
`insert-claim-entry` given `document` and `locator` together places the claim's marker at the
located line in the same call, and given one without the other is refused, naming the missing key;
`insert-analysis-entry` given a `document`, and `insert-experiment-entry`, which requires one, place
the marker on a new line directly after that document's first heading. `mark-claim-in-leaf` places
or moves a claim's, an analysis's or an experiment's one marker, appending it to the line the
locator names. `insert-axiom-entry` records an axiom by `label` and `title`.

The values take the shapes §10 gives them: a node's `rigor`; `rests-on` entries of `{id, context,
bearing}`; pair entries of `{id, bearing}` in a claim's `refutes` and, at insert, in an analysis's
`lifts` and `refutes` and an experiment's `supports`, `lifts` and `refutes`; and `add-pairs`'
entries of `{id, bearing, context}`, from an existing claim (`refutes`), analysis (`lifts`,
`refutes`) or experiment (`supports`, `lifts`, `refutes`). `set-rigor` takes `id` and `rigor`;
`set-bearing` takes `id` (the source), `target`, `relation` and `bearing`; `set-status` takes an
experiment's `id` and its `status`, `run` or `pending`; `add-rests-on` takes `id` and the `rests-on`
and `references` lists it adds. Each op's closed key set is `kbase <op> --help`'s, as for every op.

A written `rigor` is a number in `[0, 1]` or `*pending*`. `1.00` is the rigor tables' maximum — the
whole of the rigor scale, and the value a charge list with no charge takes (§10) — so every op taking
a node's own rigor — the inserts and `set-rigor` — refuses a value above it, naming the key and the
range. A bearing's writes take the same top, its table reserving nothing either.

`set-bearing` **replaces an existing edge record's bearing and never creates an edge**: an edge with
no record is refused, and so is a `demoted` record, which `resolve-demoted` restores. It refuses a
relation outside the four, a pairing whose bearing is illegal (`references` and `demoted`, and every
source and target kind the four do not join), and a value outside `[0, 1] ∪ {*pending*}`. A node's
`rigor` is optional on every insert, and a bearing on every `rests-on` entry and every pair entry, at
insert or through `add-pairs`: a value the caller leaves out is written `*pending*`, so attaching a
node or an edge moves no derived value until it is judged. Every public op that adds a scoring edge
refuses one that would close a cycle (`check: dependency-cycle`, naming the path); an edge the
records already hold keeps its record as it stands, bearing included.

**Manually scored.** A claim's, an analysis's and an experiment's record, and every `supports`,
`lifts` and `refutes` edge record, carries `manually-scored` (§10). A public write op — at the
command line or over MCP — writes it `true` on a rigor or a bearing its caller supplied, `*pending*`
included, and `false` on one it writes `*pending*` because the caller left it out; the build and a
scoring run's landings and re-prices write `false`; an op leaves the flag as it stands on a value it
does not write, and an insert that adopts writes no value. `set-rigor` or `set-bearing` re-asserting
the current value of a record marked `false` marks it `true`, outcome `done`. A `demoted` record
keeps the replaced edge's bearing and flag unchanged, through the cut and through its restoration.
No values key, flag or tool argument sets the flag.

### `drop-claim-edges`

Removes a claim's own edge records of one class — its premises (every `supports` and `founds` edge
into it, an experiment's support included) or its references (the `references` edges out of it) —
the `edge-class` named or, where none is, both; a claim holding no edge of the class stands. The
`demoted` class is refused: demoted records are `resolve-demoted`'s to remove. Idempotent: a
re-issue is `unchanged`.

### `drop-inbound-edges`

Removes every edge record by which another node names the claim: a `supports` edge from it into
another node, a `references` edge into it, and every `lifts` or `refutes` edge and every
experiment's `supports` edge into it. A claim no other node names stands. Idempotent: a re-issue is
`unchanged`.

### `drop-claim-node`

Deletes a claim node entirely, in one call: its node record, its statement marker in every leaf
carrying one, every `demoted` record naming it, and every edge record naming it — its own edges
(every `founds` edge and every claim's `supports` edge into it, and the `references` and `refutes`
edges out of it) and every edge by which another node names it (those `drop-inbound-edges`
removes). No prior call is needed. Refused, writing nothing, where, once those removals are
composed, the node is still named by a line under `kb-root/` outside the register pages, each such
line named as `path:line`, or by a prose field of another record — a title, `rationale`, `no-edge`,
a `strengthen-by` item, an edge's `context` — each named by its record file, its record and its
field. Idempotent: a re-issue removes whatever of the node still stands (`done`) and is `unchanged`
where nothing does.

### `render-citation`

Reads the cited document and writes nothing; returns the sanctioned authority-citation string — one
Markdown link whose text quotes the excerpt and whose target is the cited document at its anchor,
spelled relative to the citing document: `["<excerpt>"](<path>#<anchor>)`.

### Queries

Each query derives its answer from the KB's records and statement markers and writes nothing;
`kbase <query> --help` states its arguments, and §7 gives its result keys. A KB converted at load
and not yet saved answers as converted, its result carrying `migration`; a degraded KB is refused
(§10). `cited-by`, `referenced-by` and a claim's `citation_count` read a claim's citations from the
leaves its statement markers stand in.

### `deps`
### `gated-on`
### `cited-by`
### `find`
### `referenced-by`
### `strength`
### `subtree`
### `show`
### `stats`
### `edges`

`strength` lists claims by the derived value. `--min` (inclusive) and `--max` (strictly below)
bound it, and with neither it lists every scored claim; `--min-dependents` (default 0) keeps the
claims with at least that many dependents. `--sort strength`, the default, orders by ascending
strength, then id; `--sort dependents` by most dependents, then lowest strength, then id.
`--pending` lists the claims with no strength instead, and is refused (usage, naming `--pending`)
beside `--min` or `--max`; under it `--sort strength` orders by id and `--sort dependents` by most
dependents, then id. Its rows read each claim's `strength`, so a bearing that moves a claim moves
the list. Its `dependents` count and `show`'s `premise_count` are structural — counts of edges,
which no bearing changes.

`stats` reads the derivation in `strength_bands`, `highest_leverage` and `weakest`, and is
structural in its per-kind counts, `edges`, `demoted_edges`, `strengthen_by_items` and the edge
counts of `relations`.

`stats`' `relations` and `edges` read the index's edge rows and no derivation: the bearings they
report are the values the rows hold, which `set-bearing` moves. `relations` reports `walked` per
relation and `edges` on each row: whether a score run's bearing pass asks about that relation's
edges — `supports`, `lifts` and `refutes` — where a `founds` bearing is the user's, written by
`set-bearing`.

### `export-latex`

```
kbase export-latex [--out DIR] [--volume NAME]...
```

Writes a LaTeX project from the KB's current state into `--out` (default `latex-export/` beside
`kb-root/`; a relative `--out` resolves against the project root (§9)). Every KB file is read
through the one loader (§10's stamp rules apply); nothing under `kb-root/` is written. `--volume`
is repeatable, each naming a volume directory under `kb-root/`;
the named volumes export in entry-point order, and a `--volume` naming no volume in the entry
point's list is a usage refusal (§7). The out directory may be any directory, the author's own
LaTeX project included: kbase owns only its `gen-tex/` subdirectory and writes nothing else there
but a missing `preamble.tex`. **Manifest refusal.** A `gen-tex/` that exists holding files without
the export manifest `gen-tex/kbase-export.yaml` is refused whole, nothing written — `check:
export-dir`, remedy: empty the directory or name another `--out`.

**The project.** `gen-tex/` is kbase's and every export regenerates it: `main.tex`, the whole
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

**Content.** The export walks every document the index child-list spine names, `overview.md`
included, as ordinary leaf content; `references.md` leaves are read as bibliography data, never
walked as documents. Author labels are namespaced per volume, so a label repeated across volumes
stays distinct, and LaTeX numbers what they label. A reference naming a label its volume defines
resolves to it; a cleveref-family anchor (`ref+label`, `ref+Label`) renders as one `\cref`/`\Cref`
over every label it names, one or several, without the anchor's own text beside it. A link to an
exported KB document becomes a hyperlink to that document's heading. Prose is exported as written:
a hand-maintained KB's prose mentions of equations and works (`(eq:dv)`, `(Khalil, 2002)`) and its
bold-line theorem blocks stay text. The bibliography holds `references.md`'s entries or, where a
volume has none, the KB's work records.

**Degradation, not refusal** (§3's `dot` posture): a reference whose label its volume does not
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
kbase score [--pass rigor|bearing|both] [--limit N] [--scoring-model MODEL] [--dry-run]
            [--resume RUN] [--rescore-manual none|rigor|bearing|both] [--state-dir DIR]
            [--timeout DURATION] [--ask-concurrency N]
```

`--ask-concurrency` under 1 is refused as `build`'s is (§5); `score`'s `resume` does not carry it,
nor `--timeout` or `--limit`: each shapes one invocation, not the run.

Grades the KB's rigor and bearing values by asking the scoring model: one call per item, one value per
call, each value landed through the write API and recorded in a ledger. `--pass` names the set to walk
— `rigor`, `bearing`, or `both`, which is the default — and `--timeout` is the deadline for one item's
call (default `30s`), counted from the call's start, never while the call waits for its turn. A run asks one call
per item and takes minutes: at a terminal `kbase score` blocks until the walk ends, and as an MCP tool
(§11) it starts a detached child and returns at once with the run's identity, while `kbase wait`
reports its outcome when it ends. A shell caller gets the blocking behaviour a terminal gets.
There is no `--provider`: a scoring-time endpoint is named by `--scoring-model`, which states the
provider and the model in one value, and passing `--provider` is a usage refusal.

**The two passes.** `rigor` walks every claim, analysis and experiment, by id — a node's own value.
`bearing` walks every `supports`, `lifts` and `refutes` edge, by relation in that order, then source,
then target. A run walks both, the node set and then the edge set, unless `--pass` names one: a text
edit moves a node's rigor and the bearing of every edge that touches it, so a run scoped to one pass
leaves the other stale against that edit, and scoping is for the case with a reason — one edited
subtree, or isolating a pass that is failing. A both-pass run is one run: one lock, one
`run.yaml`, one `items.jsonl`, one `ledger.jsonl` and one `resume` command, and its
`run.yaml` names the set it walks (`both`). The set is read from the index once, at the run's start,
and does not move as values land; `--limit N` bounds the walk at N items of the set, and without it a
run walks every one. **A work's rigor is the one value no pass asks and no run writes:** a
work's standing is judged with the document in hand, and `set-rigor` is what writes it. An axiom is
on neither pass either, its rigor being `1.0` by rule. **A `founds` edge's bearing is the second such
value:** the work or axiom it rests on lies outside the corpus, so a model shown only the KB has the
work's title and nothing to judge the link by, and the bearing is the reader's, made with the work in
hand. `set-bearing` is what writes it, as `set-rigor` writes a work's rigor. The edge carries a bearing
like any other — the values table prices it `bearing × the source's rigor` — and no pass reaches it.

**A person's value is skipped.** A value whose record is `manually-scored: true` (§8) is a person's
judgement standing, and the walk skips it — reading the flag from the KB the run loaded at its
start — unless `--rescore-manual` names its pass: `rigor`, `bearing` or `both`; `none`, the default,
re-judges none, and any other value is a usage refusal. A skipped item is asked nothing and lands
nothing: its ledger line records status `skipped`, reason `manually-scored`, with no value, charges
or call, and so no capture. It counts under `counts.skipped`, never under `scored` or `failures`,
and does not make `resume` present. A value `--rescore-manual` re-judges lands `false`, so later
runs re-judge it too. The flag holds for one invocation and is not in `run.yaml`: `resume` repeats
it where the run was started with it, a `--resume` asks a skipped item only where its own
`--rescore-manual` names that item's pass, appending no second `skipped` line for an item, and a
re-price leaves a skipped item's landed line as it is. A `--dry-run` evaluates nothing, so its
`skipped` is `0`.

**One item, one quantity, one call.** A call carries one node's text or one edge's — never two
subjects, never the other pass's quantity. The item's class gates the instruction as it gates the ids:
a claim is asked whether its statement is fixed, consistent and single-sensed, an analysis or an
experiment whether its method establishes what it sets out to, and neither is asked the other's
question. The text is closed to the subject: a node's title and statement, or an edge's two
statements, its authored context and the target's passage; a register page is never read. A text
carrying a scoring token (`- rigor:`, `- strength:`, `(bearing …)`, `(strength …)`) is refused
before the call, naming the document and line, and that item is `failed` with no call made.

**The answer.** A reply is a list of charge lines, each one fault id and the numbered line of the item's
own text where the fault shows — one line per occurrence, in the order written — or the word `none`,
alone. The call carries the text
wrapped at word boundaries to at most 100 columns, its own line breaks kept and nothing rewritten, and
writes `N| ` before each line lying inside a passage a charge may cite, numbering from 1: a numbered
line is a verbatim slice of that text. A frame label and an edge's `## Context` carry no number, and a
line with no number is no site.
[RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md) states the criteria, the
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

**The value.** kbase prices the charge list: the item's class's maximum — `1.00` under each of the
three tables — less each id's weight per occurrence under that class's table, floored at zero and
exact at two decimals, summed in integer hundredths. `none` is the empty list and
reaches the maximum; a ledger line recorded with `flt-pending` prices `*pending*` whatever the table,
though no table holds that id now. The tables are `internal/kb`'s
and no template carries a weight, a maximum or a band: the ids and the criteria are the ask's, the
price is kbase's, and nothing else is an input. A landing is therefore checkable against its own
ledger line without asking a model.

**A refused reply is a failed attempt.** A `malformed` reply writes nothing, and the attempt's line
records `status: failed` and the reason — the case, and where it names one the id and the lines — and
**no text of the reply**. The same holds for a reply cut off at the completion cap, whose charge list
may be incomplete, and for a call or a write that failed. The run goes on, and `--resume` asks the
item again. Nothing is defaulted or supplied by kbase: a link or a node is charged, answered `none`, or
left unasked, and `*pending*` is no model's answer — `set-bearing` and `set-rigor` are what write one,
as a person's decision.

**A provider call's failure names what ended it.** `provider-refused` — the endpoint answered
with a status that is not success: the request was carried, read and declined, and the reason
carries that status and the provider's own words beside it. `transport-failure` — the
connection, the name, the certificate or the stream failed, and no answer of the provider's was
had. `truncated-empty` — the provider answered, and the answer reached the completion cap before
any answer text: the request leaves it no room, and it is not sent again. A reason states the one that happened: the first calls for the request to be looked at, the
second for the path to the provider.

**Landing.** A rigor answer becomes `set-rigor` with the node's id; a bearing answer becomes
`set-bearing` with the source as `id`, the edge's `target` and `relation`, and the bearing. The run
lands every answered value at once when its walk ends: one `set-rigor` batch and one `set-bearing`
batch, each `--no-refresh`, under one hold of the KB write lock. An entry a batch refuses is that
item's `failed`, and the batch is re-issued without it; a landing that cannot take the lock fails
every item it carried: where another writer held the lock past the wait, the item's line
carries the lock item's `detail`; any other lock failure records its own error. The
run goes on.

**The scoring role.** The calls go to the `scoring` role (§9): `models.scoring`. The role is the KB's
declaration of which model judges it, so a run that names none refuses as setup — naming that key,
before the KB is opened and before the run's start refresh — and nothing falls back to the text role.
The role's model is the file's `<provider>/<model>` reference or, where the files name none, the one
`--scoring-model` names: the flag supplies an unconfigured role and overrides a configured one, its
reference is resolved against the pool, and a provider the pool does not hold refuses naming the flag
with the pool listed. A run records the model as the flag's where the flag named it. A `--dry-run`
resolves no role and refuses nothing for one.

**One item, one call.** The ask is one tool-less chat call on the `scoring` role's endpoint, over the
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

**The ledger and resume.** A run's records stand under `<state-dir>/score/<run>/`: `run.yaml`,
`items.jsonl` (the set the run enumerated, one pending line per item, so a reader counts the whole
run), `ledger.jsonl`, `captures.jsonl` and, while a walk runs, one result file per answered item, which
the walk's end folds into the ledger. A real run's id is `run-<id>`; the runs kept are §6's.
`run.yaml` holds the run, its KB root, the ledger's format, the set it walks
(one pass, or `both`), `backend` (`provider`, which a header written under an earlier release may
read `environment`; absent under `--dry-run`, which resolves no role because it asks nothing), the
provider, the model, `ask_sha256` and the start. **It is
written once, at the run's creation, and no resume rewrites it**: a resumed or mixed run's header
states the first invocation's back end and model, and each ledger line states its own item's pass and
what answered it.

`ledger.jsonl` is one JSON object per attempt — the item, its status, the value, the value the KB
carried before the attempt (`before`, absent where the record held none), the reply, `faults`
(the parsed charges: `id`, `passage`, `lines`, `citation`, `offset`, in the order written), `moot` (the charge
lines a reply recorded beside a retired `flt-pending`), `backend` (as the header's; a line written
under an earlier release may read `environment`), the model, the prompt's digest and the attempt's bounds —
appended after the item's write and its validation, never before. A run writes its captures and then
its ledger lines once, in the set's order, when its walk ends — on completion, on `--limit`, on
cancellation and on a failure that stops it. `faults` is `[]` for one reply
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

**`ask_sha256` records the ask set and gates nothing.** It is one digest over every instruction the
run can send, each rendered for an empty subject and concatenated in a fixed order — a rigor run's
statement instruction and then its method one, a bearing run's one, a both-pass run's statement, method
and then bearing instruction — so every fixed byte of a run's
instructions is certified and nothing that varies with an item is. Each ledger line carries its own
item's prompt digest beside it, which is what names the variant that judged that item. A header
written before the rigor pair carries one render's digest and is read as the one ask that run began
under. `--resume RUN` continues a run: it refuses a ledger for another KB root, format or pass, first lands and records every answer a
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
is a new run that asks everything: a value already in the KB is not evidence about the one this run
would judge.

**The run's locks.** A scoring run takes the KB's operation lock and the state store's holder lock
for its length, in the order a build takes them, before it writes anything and before its start
refresh: a second run over the KB — a build or a scoring run — is `refused` at once, `check: lock`,
`path` the holder's state-dir, its `detail` naming the holder's verb and pid (§6), never held through
a wait the first run will outlast. Its holder record names `kind: score` and the run id, which is
what `status`, `cancel` and `wait` read (§6).

**Refresh and format.** Both refreshes hold the KB write lock for the refresh and release it before
the run continues. A degraded KB (§10) is refused before that refresh, naming each fault: the
refresh is the remedy, and a run would judge a graph the load resolved rather than the one the
records hold. One `refresh` opens a run, before the index is read, so the set comes from a
current index; on an older KB it is the natural write that converts and stamps the KB (§10), so a
scoring run is the call that saves a conversion, and its document carries `migration`. A KB still
unmigrated after it is refused before the first item (`check: index`, remedy `kbase refresh`): a
landing runs with `--no-refresh`, which an unmigrated KB refuses (§10). One more
`refresh` follows the last write — on completion, on `--limit` and on cancellation. A refresh that
cannot take the lock ends the run there, with the lock's own outcome — `retry` where another writer
held it past the wait — and the `lock` item, whose remedy is `re-run kbase score`. A crash after a
write leaves a stale index, which `verify` reports and the next run's start refresh repairs.

**Outcomes.** `done` when the run finished; `bounded` when `--limit` stopped it, and a run that both
stopped at its limit and failed an item is `bounded`; `cancelled` when SIGINT or SIGTERM stopped it,
with no line recorded for an item whose call was in flight or not yet made, and a line for every
item answered before the stop;
`failed` when an item's call or write failed, or a refresh did; `retry` while another run holds
the state store, or where a refresh could not take the KB write lock past the wait; `refused`
where the run cannot go on — setup (a pass outside the three, a negative `--limit`, a `--state-dir`
inside `kb-root/`, a `--resume` naming no real run kbase minted, no scoring model configured, an
index the load cannot answer, a
degraded KB, a KB still unmigrated after the start refresh), a run already holding the KB's
operation lock (§6), or a KB that no longer opens when the end refresh re-reads it.

**The closing record.** A `failed` run closes on `failures`. Where the walk itself failed, it holds one
`check: error` item per item the ledger records `failed`, `detail` the item as the document names it
and the reason its line recorded — `<node id>: <reason>`, or `<relation> <source> → <target>:
<reason>` — in the walk's order; the reason is the line's reply as a bounded, control-stripped
summary, the ledger's line being the one place a reply stands whole; a run a refresh stopped closes
on one `check: error` item naming the refresh's own error. A `cancelled` run closes on `cancelled`,
whose `item` is the walk's first item carrying no attempt, and `{}` where
every item of the walk carries one. A `refused` or `retry` run closes on its `refusals`. A `bounded`
run that failed an item carries no `failures`: the failure is counted in `counts.failed` and reasoned
in the ledger alone.

**`resume`** carries the command that continues the run, present wherever an item failed or was not
reached — one `--limit` left, one `failed`; a `skipped` item is no such item — and it stands after
`counts` and `scored` and before the closing record, so a stop at the end refresh carries it
whenever such an item remains. It carries `--rescore-manual` wherever the run was started with it
naming a pass. It is absent
from a dry run and from a stop before the walk: the start refresh's, the dial's, the
ledger's or the index load's. It carries `--pass` wherever the invocation named one, and names none
where it did not: a run that walked both by default is continued the same way, its header being what
says which set it walks. It carries `--config-dir` wherever the invocation named one — the
resumed run must read that directory and no other, or the same run id would resolve another
reference — and it carries `--scoring-model` whenever the invocation did — the command must reach the
same model, or it is a different judgement under the same run id. A run started under an earlier
release whose header reads `backend: environment` is resumed on whatever the files now resolve, and
its new lines record that.

**`--dry-run`** lists the set and asks nothing. It writes `run.yaml` and `items.jsonl` (one pending
line per item), dials no provider, resolves no role, and writes no KB file:
its `counts.done` is `0`, its `scored` is `[]`, it carries no `resume` and no `roles`, and its outcome
is `done`. It writes no charge record and no capture. Its run id is `dry-<id>`, and the runs kept
are counted by kind (§6), so a dry run counts only against dry runs. It cannot
be resumed — `--resume` with `--dry-run` is a usage refusal naming `--resume`, and `--resume` naming
a `dry-` id is refused as no real run's id. Because the refresh
that would save a conversion is a write, a dry run on an older KB saves none and so is refused by
the run itself, as a KB still unmigrated after its start refresh is (`check: index`, remedy
`kbase refresh`), and a real run on the same KB then migrates and saves.

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
selection a scoring run would make, so `kbase probe` with no arguments is the heartbeat of a
configuration. The four are mutually exclusive: more than one is a usage refusal naming all four as
what may be chosen once, before anything resolves or dials. A selection that does not resolve is
`refused`, naming the role's own key — `models.<role>` — or the flag, and what is missing, before
any client is opened. The document states which selection answered: the role's name where a role flag
or the default named it, and the reference where `--model` did.

A chat selection is one tool-less chat request through the registry a scoring run's asks take, so
an endpoint is reached over the wire shape its pool entry names: no tools, no history, an empty
system prompt, and `--prompt` in place of the default one-word check. It carries its role's sampling
settings (§9.2) — the `scoring` role's where `--model` names the selection — and the document
reports the effective settings, `temperature` and `thinking`. A `--decision` selection is
one decision call on the surface its entry's `decisionsFlavor` names, with no system message, and
its `--prompt` in place of the default predicate. `--timeout` bounds the call (default `30s`); a
call that does not complete is re-issued at once up to the attempt count, so that deadline is spent
on attempts rather than on pauses (§12). The document reports what a ledger cannot — the selection,
the provider, the model, the time to the first token, the call's wall time, the finish reason, the
reply, and the usage the provider reported — and its outcome follows the call: `done` where it
completed, or where a chat answer was cut off at the completion cap before any answer text
(`finish-reason` `length`, `reply` empty); `failed` otherwise, with the attempt's own outcome and the
provider's words in the failure. It reads no KB and reads or writes no state directory: it prints its document and exits,
and no MCP tool binds it (§11).

Result keys: §7.

### `refresh`

Renders every register page (§10) from the records and the statement markers and writes every
claim-graph sheet (§3) on each run, each only where its bytes change, removing a page or a sheet the
KB no longer calls for. **`refresh` writes no authored file but a migration save and a degraded
repair** (§10): on a degraded load it first writes the records as the walk resolves them and reports
every record it dropped and every edge it cut under `warnings`, and on a KB converted at load it
saves the conversion. Without `dot`, §3's rule. Result keys: §7.

### `verify`

The standard check a running KB owes: the dead-link and unknown-id gate over the documents under
`kb-root/`, link targets resolved against the repository and the ids against the node records — a
link into a register page also has its fragment checked against the slugs of the entries derived
for that page — then the metadata gate: a frontmatter block on any document, the entry point
included (`frontmatter`, remedy "remove the block: kbase reads no frontmatter here"); a KB loaded at an older
format (`index freshness`, refresh-fixable); a claim, an analysis or an experiment carrying no
statement marker or more than one (`statement marker`, not refresh-fixable); an axiom record and
`axioms.md`'s headings that disagree (`axiom heading`, not refresh-fixable); each fault a degraded
load resolved (`degraded`, refresh-fixable); and each register page that does not stand as `refresh`
renders it (`register freshness`, refresh-fixable). Faults are outcome `refused`; a `demoted` record
(§10) is a finding, listed under `findings` with outcome `done`, and `findings` is present on both
`done` and `refused`. The citation grammar is the build-time check: the build's gate stage runs it
after the standard check, and `verify` does not. Result keys: §7.

### `status`, `cancel`

See §6.

---

## 9. Configuration

Two TOML files — `providers.toml` (§9.1, the endpoint pool) and `config.toml` (§9.2, the choices) —
are read under the strict-load contract (§9.3), in one or two layers.
`<repository>/.kbase/` is read over `~/.config/kbase`; a directory `--config-dir` names, else
`$KBASE_CONFIG_DIR`, is instead the one directory read, so an explicit choice isolates a run from a
repository's own `.kbase/`. An invocation outside a worktree has no repository layer:
`show-config`, `models` and `configure` run in any directory.

**Merging.** The two files merge by two rules. `providers.toml` merges by entry
name and an entry is replaced whole: a repository entry of the same name supersedes the global one
entirely, never field by field, so a repository cannot inherit a credential for an endpoint it chose.
An override names its entry exactly: entries whose names differ only by case — two in one file, or
one in each of two layers, whether pooled or dropped by a fault — refuse the load, naming both
spellings and the file stating each; across layers its detail names the fix: spell the higher entry
as the lower one is, or rename it. A fault naming a pool entry a higher layer superseded is
not reported: the pool's entry is the higher one, and a diagnostic about a value nobody reads is not a
fact about this run. `config.toml` merges field by field: a value a higher layer states wins, and one
it leaves out keeps the lower layer's value. A layer that fails the strict load refuses the whole
load, naming its file: no lower layer is read, so a broken repository file never silently falls
through to the global one.

**A leading `~` expands; nothing else does.** In the `--config-dir` and `$KBASE_CONFIG_DIR` values,
and in an entry's `apiKeyFile`, `~` alone is the home directory and `~`
followed by a path separator is a path beneath it. `~user` is not supported, a `~` anywhere else is
literal, and where the home lookup fails or its answer is not absolute, the string stays exactly as
written.

**Global flags.** Every subcommand accepts `--config-dir DIR`, `--log-level debug|info|warn|error`
(default `warn`), `--log-file PATH` (tees diagnostics to a file; console output is never redirected
away) and `-h`/`--help`; the root command also accepts `-v`/`--version`, which prints exactly the
version string and a newline. A directory `--config-dir` or `$KBASE_CONFIG_DIR` names is the only
one read, the flag before the variable; otherwise configuration is read from the two layers above.
`--project-dir PATH` (the project root: the git worktree root that holds `kb-root/` or will; the
subcommand's search for the repository starts there instead of the working directory; a path that
is no git worktree root is refused, `check: worktree`, `key: --project-dir`, `path` the absolute
path)

A relative path a subcommand takes — a volume root, `--bibliography`, `--state-dir`, `export-latex
--out`, `--values` — resolves against the project root; `--config-dir` and `--log-file` resolve
against the working directory, whatever `--project-dir` names, and a printed `resume` writes
`--config-dir` absolute.

**Provider commands.** `kbase models [--provider NAME] [--timeout DURATION]` lists the selected
provider's model identifiers, sorted ascending, in its result document (§7). `kbase configure
[--create] [--project] [--provider NAME] [--model-map CATEGORY=<entry>/<id>]... [--timeout
DURATION]` assigns a model to each category from `--model-map` alone — the categories are `decision`,
`text` and `scoring`, and a value is the `<provider>/<model>` reference §9.2 states. The dialled path requires every
category: a category the flag does not name is refused naming `models.<category>`, `check: detect`,
with the provider's model ids as `allowed` (§7) — except `scoring` where the target file already
states one, which is left standing, so `configure --provider P` over a complete file writes no
scoring value and is `unchanged` where it changes nothing else. A file `configure` writes is never
left making `kbase score` refuse. `kbase show-config [--provider NAME]` reports how the configuration
resolves and changes nothing (§9.6). `kbase probe [--model REF] [--decision | --text | --scoring]
[--prompt TEXT]` asks one selection — a role flag's role, else the `scoring` role, or the reference
`--model` names — one question and reports the call, reading no KB and writing no state (§8). Each
writes its result under §7.

`kbase auth add <provider>` stores the API key piped on its stdin for `<provider>`, a
`providers.toml` entry name, replacing any key stored for it: one file per provider,
`$XDG_STATE_HOME/kbase/auth/<provider>-key.enc` (`~/.local/state` where the variable is unset),
the file named for `<provider>` in lowercase, each file encrypted under an encryption key derived
for the OS user — obfuscation against a reader of the file, not a boundary against that user — and
readable by its owner alone. `add` reads the configuration as the provider verbs do, and a
configuration that does not load refuses as it does for them; a name no `providers.toml` entry
carries exactly, case included — in the pool or dropped from it by a fault — is refused,
`check: usage`, `key` the name as given, `remedy: kbase show-config`. The key arrives on stdin and
no other way: stdin that is a terminal is refused, as is input that is empty once surrounding
whitespace is trimmed, each `check: usage` with the piped form, the provider's name filled in, as
its `remedy`. A refusal writes nothing, and the name refusals come before stdin is read. `kbase auth remove <provider>` clears the stored key, `unchanged` where none was stored, and
`kbase auth list` names the providers with a stored key, in lowercase; neither reads the
configuration, so a key file whose entry is gone is still removed. A name that breaks the
provider-name rule (§9.1) is a usage refusal, `key` the name as given and the detail the rule,
never the name; its `remedy` is `kbase show-config` on `add`, `kbase auth list` on `remove`. A
refusal of the argument count names no argument given. No result names a key.

**Provider selection is the catalogue verbs' alone.** `models` and `configure` select a pool entry to
dial — not a role: a role is never chosen this way, and `config.toml` names no entry. The rule is
`--provider`, else the sole `providers.toml` entry. Where no pool entry is named — zero usable providers, two or more with none
selected, or a `--provider` the pool does not hold — `models` and `configure` refuse. An empty pool,
and a name the pool does not hold, point at the `providers.toml` of the layer `configure` writes into,
the file an entry belongs in; the name the pool does not hold also carries `key: --provider`, and the
remedy `kbase configure --create` unless that provider faulted out of the pool, where the fault's own
reason stands instead. Two or more entries with none chosen list every entry name as `allowed` with
`key: --provider`. `show-config` reports any of these in its document and still exits 0.

**`configure`'s write target.** `configure` writes `config.toml` — and, under `--create`, the files
it creates — into the directory a flag or variable named, whether or not it exists: the
`--config-dir` value, else `$KBASE_CONFIG_DIR`, the flag before the variable. `--project` names
`<repository>/.kbase/` instead, whether or not it exists, and outranks the variable; `--config-dir`
with `--project` is a usage refusal naming `--project`, the two naming different directories; and
`--project` outside a worktree is refused with `check: worktree`, `path` the directory the search
started from, and `git init` as the remedy. Otherwise `configure` writes the repository layer where
its directory already stands, and the global layer otherwise.

**`configure --create`** writes the target layer's `providers.toml` where no file
stands, from its annotated template — no other command creates it — and overwrites no file's
other content. `config.toml` is created where absent by a run that resolves a provider, and
by `--create` with no `--provider` — a run over a pool that resolves none refuses before writing
anything (§9, "Provider selection") — and otherwise edited under §9.5, which keeps every byte
`configure` was not asked to change. Without `--provider` it makes no request, and one line on stderr
says the model ids went unchecked; with `--provider` the ordinary path runs — select, dial, list the
catalogue, warn on an id the catalogue does not offer. A refusal writes nothing, and so does a dial or
catalogue failure.

**The files are the one source of inference.** The endpoint and the model each role asks come from
`providers.toml` and `config.toml` alone, and so does how a role is asked — `[asks]` and `[sampling]`
(§9.2). No environment variable supplies an endpoint, a model or a key. A run isolated from the
user's layers names its directory with `--config-dir` or `$KBASE_CONFIG_DIR`. The inference
preflight (§5) names each lacking setting as `baseUrl` or `models.<role>` — `models.decision`,
`models.text`, `models.scoring` — and `show-config` reports that refusal as a diagnostic rather than
stopping (§9.6).

There is no plaintext-transport switch: when a key is configured and the base URL is `http://` to a
host other than loopback, one line on stderr says the key travels in cleartext, and the build
proceeds (as §9.4's permission warning does).

### 9.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses within the provider-name rule:
each matches `^[A-Za-z0-9_][A-Za-z0-9_-]*$`, and an entry whose name does not fails the file's
load, naming the file and the entry; names that differ only by case are refused (§9, "Merging"). An
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

No other key is recognized inside an entry. An entry that fails validation (empty `baseUrl`, or
`type`/`api`/`decisionsFlavor` set to anything but its accepted values) is **dropped** with a
warning naming the entry and the reason (never key material); the rest of the pool still loads. A
key-file read failure faults just that entry the same way. An absent `providers.toml` is an empty
pool, not a load error.

**The chat protocol.** `api` names the wire protocol chat uses, chosen per server; it is a two-value
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

**The decision surface.** `decisionsFlavor` names the wire shape the decision call uses, chosen per
server: it changes nothing about chat, which `api` alone decides, and it binds
no model id — the decision category's id stays in `config.toml` (§9.2). An entry silent on it speaks
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
`models.<role>` and the form. Nothing is normalised but whitespace: a model id is the
provider's spelling, not kbase's.

A category left unset resolves to "not configured" — no defaulting, no guessing, and no role falls
back to another. `concurrency` is how many model calls one invocation of `build` or `score` has in
flight at once, across every role; absent, it is 4, and `--ask-concurrency` overrides it for that
invocation. A value under 1 fails the load, naming `asks.concurrency`. Calls start in the order the
invocation submits them. There is no `[dev]` table, top-level `provider` key or `[asks]
readerConcurrency`: a file stating any of them fails strict load (§9.3) naming the key, and the
remedy is a hand edit. A
`[models]` table spelling `heavy`/`light` — the superseded category names — fails strict load as
unmodeled keys (§9.3); the remedy is a hand edit.

**Sampling is per chat role.** `[sampling.text]` and `[sampling.scoring]` each take
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
`models.typo_key` — a superseded category name such as `models.heavy` fails the same way),
pluralizing "unknown key"/"unknown keys" when more than one. A malformed-TOML file fails with a
parser-detail message naming the file and line. Only key **names** ever appear in either message,
never values: a file may still hold a secret a hand edit left behind — a field no longer modeled, a
stray value — and the refusal that names its key is exactly where that value would be echoed.
`kbase configure` cannot repair a config file it cannot load: the verbs that read configuration load
it before doing anything else. A layer that supplies a pool entry supplies its `apiKeyFile` path too,
and kbase confines no key file's location: trusting a cloned configuration means trusting the key
file it names as much as the endpoint it names.

### 9.4 Key-file permission warning

When a `providers.toml` entry sets `apiKeyFile` and the host is not Windows, kbase stats the
resolved key-file path. If its permission bits grant group-read or other-read, it prints a warning
naming the path and the offending mode and recommending `chmod 600`. The entry stays usable. This is
the only file-permission behaviour kbase applies to a file it did not create.

### 9.5 `configure`'s targeted update (byte-preservation contract)

`kbase configure` never re-serializes `config.toml`; it edits the existing bytes in place.

- An absent or blank file gets a full commented template, its `models.decision`, `models.text` and
  `models.scoring` written with the values the run resolves and blank for the user to fill where the
  run names none.
- Otherwise **only** the three keys it writes are touched: the `[models]` table's `decision`, `text`
  and `scoring`. Every other
  byte — comments, unrelated keys and tables, spacing, key order — is preserved exactly. A rewritten
  line's own trailing `# comment` is kept; its column position is not guaranteed.
- Missing `decision`/`text`/`scoring` keys are appended inside (or as a new) `[models]` table. A
  category the run is not given and the file does not hold stays absent: the offline path writes
  what it was told and refuses nothing, and the dialled path is where an unassigned category is a
  refusal (§9, "Provider commands").
- Before writing, kbase decodes both the original and the proposed file as TOML and refuses the
  write, leaving the file untouched, unless the only difference between the two decoded structures
  is exactly the values it intended to change.
- The write is atomic (temp file in the same directory, then rename). A `config.toml` that is a
  symlink has its target rewritten; the symlink stays.
- Any failure along this path leaves the file completely unwritten.

### 9.6 `kbase show-config`

`kbase show-config [--provider NAME]` reports how this invocation's configuration resolves, and
changes nothing: it reads no KB, writes no file and makes no network request. A configuration that
fails strict load is refused before the verb runs, as for every verb (§9.3). Otherwise the outcome is
`done` whatever the report finds: a fault in the pool and an unresolved role are diagnostics, not
refusals, so one call shows every gap at once. It makes no request and opens no client.

The report states the layers this invocation read, in precedence order, each with its path, the
source that named it — `explicit` (a flag), `environment` (`$KBASE_CONFIG_DIR`), `repository` or
`global` — and whether the directory and each of its three files stand; `config.toml`'s merged
`[models]` values, null where a category is unset, and its `[sampling]` values, null where unset; every pool entry in name order with the layer
that supplied it and the decision surface it speaks — its default filled in, so an entry silent on
the flavour reads `systemone` — including a provider a dropping fault removed (unusable, with the
fault's reason); the pool entry the selection names and the rule that named it, or
why no entry was named; one record per role — the endpoint it resolves to, the layer that supplied
each field, a chat role's sampling settings, or why it does not resolve.

The sources vocabulary is `layers`': `repository` or `global` for a value a file holds, `explicit` or
`environment` for one read from the one directory a flag or `$KBASE_CONFIG_DIR` named. Its
diagnostics are one `provider` item per fault, each carrying `path` the `providers.toml` that carries
the fault — which need not be the file a selection or `configure` names — and one item per role that
does not resolve, its `path` naming the file the setting belongs in: the write target's `providers.toml` or
`config.toml`.

**Names and paths, never secrets.** A key is reported by the kind it is — from a file, or none —
and a key file by the path its entry states, never by its content.

**It describes the process that ran it.** The layers, the environment and the selection are this
invocation's, so a call through the MCP server (§11) reports the configuration of the server
process.

---

## 10. Metadata format version

**The stamp.** A KB declares its metadata format as `kb-format: <major>.<minor>.<patch>`, semver,
in `kb-root/.index/format.yaml`: one block mapping carrying the one key, no `---` fences. The entry
point's frontmatter is the stamp's superseded place, standing in every KB older than `3.0.0` and
read wherever it still stands. Two forms are
superseded: `1.0.0`, which `2.0.0` replaced, and `2.0.x`, which `3.0.0` replaces. Major: a reader
built for the old major misreads the KB (a renamed field,
changed semantics, a changed anchor rule). Minor: additive; an old reader ignores the addition
safely. Patch: the bytes a writer produces changed, their meaning did not.

**Reading the stamp.** Before any other read, kbase reads `kb-format` from
`kb-root/.index/format.yaml`; where that file does not stand, from the entry point's frontmatter,
the superseded place — and where both stand, the file's stamp is the KB's. A KB with neither stamp
file nor entry point is refused (`check: kb-root`) as it is today; a KB stamped in neither place,
and an unreadable stamp in either, are refused (`check: kb-format`), the refusal naming where it
looked. Every producer of a KB stamps the file: the build's spine, `init` at birth, and every save
(below). A stamp at the current major and minor with another patch is read as current and
rewritten on the next save.

The current version is `3.0.0`. No converter leads to a patch, so a migration chain stops at the
form its last converter produces and the next save supplies the stamp: a `2.0.0` and a `2.0.1` KB
both convert through the one `2.0.x → 3.0.0` converter (below), `2.0.1`'s patch having changed only
the words of the band ladder's bottom rung on a derived line.

**One version read and written.** kbase reads and writes exactly the current version (ARCHITECTURE
§12). An older stamp is read through migration (below). A newer major or minor is refused by every
subcommand that reads the KB — `build` on an existing KB, `refresh`, `verify`, the write ops, the
queries, `score`, `render-citation` and `export-latex`; `status` and `cancel` do
not load the KB — outcome `refused`, one
item `check: kb-format` naming the KB's version and kbase's, remedy "update kbase". No subcommand
writes an older format, and nothing downgrades.

**Migration happens at load and lands on the next write.** Reading an older KB converts it — the
KB's files as bytes in, the current form's files as bytes out, plus the set of repository-relative
paths the conversion made obsolete and the conversion's warnings — version step by version step, in
memory; every command then sees the current form, and a converted, unsaved KB answers every query
as converted. A read-only command (`verify`, the queries, `render-citation`, `export-latex`) writes
nothing. The next natural write — `refresh`, a
write op's trailing refresh, or a scoring run's start refresh — saves the conversion, in this
order: `.index/edges.yaml`, then `.index/nodes.yaml`, then every other file the conversion produced
(each document with its frontmatter removed); then the stamped `AGENTS.md` and `CONVENTIONS.md`,
each written whole from the current templates whatever it held; then the register pages and the
sheets; then the stamp file is written at the version the KB is still at, so that a stamp always
stands and always names the version the tree is at; then the obsolete paths are removed, less any
the save itself wrote; then the entry point, whose frontmatter the conversion stripped; and last
the stamp file is rewritten at the current version — the stamp is the mark that the migration
completed. The removals are reported under `removed` (§7). Every read of a KB file goes through one
loader, `internal/kbload`, which serves the migrated bytes to every command on an older KB
(ARCHITECTURE §11). A write op with `--no-refresh` on an unmigrated KB is refused (`check: index`,
remedy `kbase refresh`): no save mixes two formats. A save that stops part-way leaves the old form
standing, stamped at the version it converts from, with some files already current. Where
`.index/nodes.yaml` then stands, both record files are the current form's —
no `2.0.x` KB carries a `nodes.yaml`, and it is written second — so the next load takes them as they
stand, strips any remaining frontmatter unread and derives nothing from the `2.0.x` registers or
frontmatter; and the next save completes the migration. On an unmigrated KB `verify` reports
`index freshness` with remedy `kbase refresh`, which is the migration.

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
derivation strength, build status and band, the premise, strengthen-by and citation counts, a
scoring edge's `target_kind`, the strengthen-by rows and their `mentioned_ids`, the citations,
subtree aggregates, each document's kind (the entry point is the entry point, a document whose child
list names a child an index, any other document a leaf), each node's host, canonical path and anchor,
and the degraded state (below). The scoring vocabulary of `2.0.0` — five node types, three
quantities, four scoring relations, one arithmetic — stands unchanged, and the rest of this section
describes `3.0.0`; `1.0.0` and `2.0.x` are the forms it supersedes, and their names appear only in
the migrations below.

**The KB YAML dialect.** kbase writes, and a reader needs, only this subset: block mappings and
block lists, lists of flat mappings, a block list of scalars as a mapping's value, plain or
double-quoted scalars, no anchors, tags, folded or multi-line scalars; flow style only for a list of
scalars (`[clm-a, clm-b]`) and an empty list or mapping (`[]`, `{}`); a scalar is never wrapped. A
reader of the dialect, in any language, is conformant when it reads the record files of the `3.0.0`
golden trees to the values their decoded-values goldens hold (ARCHITECTURE §12).

**Recognising frontmatter.** A document's leading `---` block is frontmatter only when its first
non-empty line is key-shaped: `name:` with a kebab-case name. Such a block that does not parse as a
YAML mapping is the malformed-frontmatter refusal. A leading `---` block whose first line is not
key-shaped (a prose paragraph, a list, the content after a thematic break) is prose, and the
document has no frontmatter. kbase reads frontmatter on the entry point alone, and there for the
superseded stamp alone (§10); `verify` reports a
frontmatter block on every document, the entry point included (§8).

**The record files.** Each is one YAML document: a block list of block mappings, `[]` when it holds
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

| Node record | Fields, in order |
|---|---|
| claim (`clm-`) | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `no-edge`, `strengthen-by` |
| analysis (`ana-`) | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `no-edge` |
| experiment (`exp-`) | `id`, `title`, `status`, `rigor`, `manually-scored` |
| work (`wrk-`) | `id`, `title`, `rigor`, `rationale` |
| axiom (`axm-`) | `id`, `title` |

A `title` is a non-empty line; titles may repeat, two entries of one register page sharing a title
sharing its slug. `rigor` is a number in `[0, 1]` or `"*pending*"`. `manually-scored` is a boolean
(§8). `rationale` and `no-edge` are strings, `""` for none; `strengthen-by` a list of strings, `[]`
for none, in block form otherwise; `status` `"run"` or `"pending"`. No record names a register: the
page that renders a node is derived from its host.

| Edge record | Relations | Fields, in order |
|---|---|---|
| unscored | `references` | `relation`, `source`, `target`, `context` |
| demoted | `demoted` | `relation`, `source`, `target`, `context`, `replaced-relation`, `bearing`, `manually-scored` |
| bearing | `founds` | `relation`, `source`, `target`, `context`, `bearing` |
| scorable | `supports`, `lifts`, `refutes` | `relation`, `source`, `target`, `context`, `bearing`, `manually-scored` |

`context` is `""` for none, and a query reports it null. `bearing` is a number in `[0, 1]` or
`"*pending*"`. A scoring edge runs push, from the supporter to the supported; a `references` edge
runs from the node that names to the node named. An edge record's identity is (`relation`,
`source`, `target`), but a `demoted` record's is (`demoted`, `replaced-relation`, `source`,
`target`), so an analysis's `lifts` and `refutes` of one claim, both demoted, are two records.

**Statement markers.** A claim, an analysis and an experiment is hosted by one statement marker,
`<!-- claim-rigor: <id> -->`, in a leaf; its host is the leaf holding it, derived at load, and no
record stores hosting. A node with no marker has no host, and one with several takes the first in
path order for the walk; both are `verify`'s `statement marker` fault, never refresh-fixable, and
neither degrades the load. A claim's statement is the block its marker sits in, or, where the marker
ends a heading's line, the section that heading opens, from the line after it to the next heading as
deep or shallower; an analysis's or an experiment's is the section the marker sits in, from the
nearest heading above it to the next heading as deep or shallower. Two marker forms read otherwise.
The build's analysis split (§5) places both, through its build-only `split-claim`. In the first, an
analysis's marker stands on a blank line of a blockquote, a line holding only the `>` prefix and
markers. The analysis's statement is that blockquote, its whole quoted run. A claim that analysis
lifts, its marker in the same blockquote, has for its statement the text of its marker's line before
the marker; any other claim marked in that blockquote has that blockquote for its statement, as
above. The second form stands in a paragraph: the run of lines around a marker's line that hold text
once the `>` prefix and markers are removed, all quoted or none, holding no heading line, and in
which only the first line may open a list item. In it, a claim's marker is followed by text other
than markers, and after that text stands the marker of an analysis that lifts the claim. The claim's
statement is the paragraph up to its marker. The analysis's statement is the whole paragraph.
Markers and `>` prefixes are removed from both. A paragraph of one line is the form's simplest case.
No public write op places the first form; `mark-claim-in-leaf`, which places a marker at the end of
a line, can place the second, and a marker placed into either form, by an op or by hand, reads by
it. A claim and an analysis whose markers stand side by side with no text between them, as the
format migration writes them, are in neither form. The inserts place an analysis's or an
experiment's marker on a new line directly after the host's first heading; `mark-claim-in-leaf`
places or moves any node's marker at the end of the line its locator names (§8). A marker is never
on a fenced block's opening or closing line (a display-maths fence included), so a locator naming
either line is refused, naming the locator.

**Register pages.** A claim, an analysis and an experiment is rendered on the register page of the
volume its host lies in, `<volume>/claim-rigor.md`, a volume being a top-level directory of
`kb-root/` holding an `index.md` (§3); a host outside every volume puts it on the root page,
`claim-rigor.md`. Works are rendered on the root page; an axiom on none, `axioms.md` stating it; a
node with no host on none. A page exists for each volume hosting a node, and at the root where a node
is hosted outside every volume or a work exists; a page no longer called for is removed by `refresh`
and reported under `removed`. Each entry is headed `## <title>`, so its anchor is the title's slug,
and every link a page renders — a premise's title to its entry — is computed from each node's
derived page and slug, so it holds within a page and across pages; a prose link `<page>#<slug>`
holds while its node stays on that page with that title, and the dead-link gate reports one that no
longer does (§8). A page is never read back: `verify` renders it in memory and compares bytes, and a
difference is `register freshness`, which `refresh` fixes. People and sessions read a page as prose,
take no value from it, and never edit it. Entries come in the reading order of their markers — the
spine walk, then the marker's line — then works by id. What an entry shows is presentation,
untested beyond the anchors and the freshness compare: the title, the id, the leaves citing it, the
`### Rigor` block — `rigor`, marked manual where `manually-scored` is true, then `rests-on` (every
`supports` and `founds` edge into it, with the source's title, derived strength and bearing),
`lifted-by`, `refuted-by`, `references`, `demoted` (each demoted edge into it, with the relation it
replaced and its bearing), `rationale`, `strengthen-by` and `no-edge` — and on a claim or an
analysis the `### Strength` block, its derived line and trace; an experiment's `status` and
`rigor`; a work's `rigor` and `rationale`. Each scoring and demoted edge appears on its target's
entry.

**`canonical_path` and `canonical_anchor`.** A claim's are its register page and its title's slug;
a work's the root register page and its title's slug; an analysis's and an experiment's the leaf
hosting it and the slug of the heading of the section holding its marker; an axiom's `axioms.md` and
the slug of `Axiom <label>: <title>`. A node with no host has neither, each null.

**Axioms.** An axiom is declared by its node record, which `insert-axiom-entry` writes (§8);
`axioms.md` is prose stating it, one `### Axiom <label>: <title>` heading each, and is not read for
declarations. `verify`'s `axiom heading` check holds the two together, not refresh-fixable: each
axiom record has its heading in `axioms.md`, and each such heading names an axiom record of that
label and title.

**The load: strict on syntax, lenient on integrity.** The load refuses, naming file and line
(`check: metadata`), a record file that does not parse, an unknown or missing field, a value of the
wrong type or outside its domain, an id outside its kind's grammar, a node `id` twice, and a pairing
no relation joins (the relations table below); an absent record file in a stamped `3.0.0` KB is
refused `check: index`, remedy "restore it from git". Three faults are resolved instead, and the load
completes **degraded** — a state computed on every load and never stored:

| Fault | Resolved for the walk, and in `refresh`'s repair |
|---|---|
| An edge identity on two or more records | the record with the higher bearing — a number above `*pending*`, the first in file order on a tie; for a `demoted` record the replaced edge's bearing; for `references` the first in file order |
| An edge naming a node no record carries | dropped |
| A cycle among the scoring edges | cut by the cycle walk (below); nothing is dropped |

While degraded, `verify` runs and reports every fault as a `degraded` item, remedy `kbase refresh`;
every query, `score`, `export-latex`, a `build` resuming a trail and every
public write op is refused, `check: degraded`, naming the faults, remedy `kbase refresh`. `refresh`
writes the records as the walk resolved them and reports each record dropped and each edge cut under
its `warnings` (§7). Merging metadata across branches is unsupported; `kbase refresh` repairs what a
merge leaves.

**Five node types.** `claim`, `analysis`, `experiment`, `work` and `axiom`. `node_type`,
`target_kind`, the index, the sheet and the result documents use these five words and no other. Ids
carry the kind's three-letter prefix — `clm-`, `ana-`, `exp-` and `wrk-` before a minted or authored
body, and `axm-<label>` for an axiom, whose label is written as authored
(`[A-Za-z0-9][A-Za-z0-9._-]*`) — so a token names its kind without translation. An axiom is
declared by its record and stated in `axioms.md` (above). Authoring prose may
say *invariant*, *framework premise*, *framework*, *proposition*, *evidence*, *trial*, *cited work*
or *reference*: *invariant* and *framework premise* name an axiom, *proposition* a claim, *evidence*
and *trial* an experiment, *cited work* and *reference* a work; *framework* names the document that
declares axioms and is never a node type. The graph has one word per role; the other words are the
author's.

**Three quantities.** `rigor` is a node's local value — a claim's, an analysis's, a work's or an
experiment's — a number in `[0, 1]` or `*pending*`; `bearing` is a scoring edge's scalar, a number in
`[0, 1]` or `*pending*`, and the one token on every scoring edge; `strength` is the derived value, a
number in `[0, 1]` or pending, on a claim and on an analysis. `*pending*` is the format's one
no-number literal: written for a value nobody has judged, and read wherever a value is absent. A work,
an experiment and an axiom have no derived value — their `rigor` is terminal, and an axiom's is `1.0`
by rule, which no authored file carries — so each one's strength is its `rigor`.

**Four scoring relations, one arithmetic.** Every scoring edge is stored push: `Source` is the
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
[RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md) defines, each with the
line that shows it — and kbase prices the list: the pass's maximum less its ids' weights per
occurrence, floored at zero and exact at two decimals (§8). A written `rigor` is a value in
`[0, 1]` or `*pending*`, and a bearing's the same; what changed is who chooses a number. A pass
derives it, `1.00` is the rigor tables' maximum — the largest a `rigor` pass derives, which reads *no
charge among the six*, never *the text displays its reasoning*. An axiom's `1.00` by rule is that
same number and no separate case on the read path: an authored `rigor: 1.0` — a converted
experiment, which the `1.0.0` migration gives `rigor: 1.0` — is in domain and reads as itself
everywhere the system shows it — the strength line a refresh derives from it, the index's
derivation, a query's display and the walk. An authored finite `rigor` above the maximum is read as
the maximum: a hand-edited `1.20` reads as `1.00`, and the record file keeps its text, while an
infinity (`.inf`) is outside the domain and refused — `refresh` writes
no authored file but a migration save and a degraded repair, and the repair writes a rigor it found
as it found it. The clamp is a reading rule, not a finding: `verify` reports nothing for it. A
bearing's `1.0` stands, its table reserving nothing either. The walk itself is unchanged and reads
the value it is handed, whatever wrote the number.

**The band ladder.** A `strength` is read against five rungs, each an inclusive lower bound: `0.85`
*ok to build on*, `0.65` *ok to build on, see caveats*, `0.45` *use as input only*, `0.20` *do not
build on, rework needed*, and `0.00` **unsubstantiated**. The bottom rung states an outcome and not a
cause: every strength below `0.20` lands there from either direction — a refuter's cap, or a minimum
no premise carries — and what it says is that nothing a claim built on this would inherit
substantiates the claim. The cause, where a reader needs it, is the run's fault record and the
derivation trace. One value carries one band wherever a band is shown: a claim's or an analysis's
record (`build_status`, `build_band`), the `stats` distribution's row, and the sheet's fill and
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

**Edge direction and authoring.** Every edge is stored as its record says, the scoring edges push:
a `supports` or `founds` record runs from the premise into the claim or analysis it supports, and a
`lifts` or `refutes` record from the lifter or refuter. The write ops author them from either side
(§8): `add-rests-on` and an insert's `rests-on` list name a claim's or an analysis's premises — a
claim or an experiment yields a `supports` edge into it, a work or an axiom a `founds` edge — and its
`references`; an insert's or `add-pairs`' pair lists name the targets of a claim's refutations, an
analysis's lifts and refutations, and an experiment's supports, lifts and refutations. An analysis
lifts or refutes and never supplies a premise, so a `rests-on` entry naming one is refused as a
pairing no relation joins, as the load refuses such a record. `deps` presents an incident edge from
the reader's side — `relation`, `direction` (`in` where the other end supports `id`, `out` where
`id` supports it), `target`, `bearing`, `manually_scored`, `context` — and the inverse query is
unchanged in name and meaning: the nodes a node's strength enters.

**Required bearings.** Every scoring edge record carries its `bearing` field, and the strict load
refuses one that lacks it, carries a value outside `[0, 1] ∪ {*pending*}`, or carries it on a
`references` record (above), naming the file, the line and the field: a missing bearing is never
read as `0`, nor as pending.

**The vocabulary.** `confidence`, `quality` and a work's `strength` were three names for one local
value; all three are `rigor`. `solidity` was the derived value; it is `strength` on every surface —
a register page's `- strength:` line and `(strength …)` annotation, the derived rows, the query
keys, the sheet's bands and the `strength` query. The register page is
`claim-rigor.md`, placed by host (above); an entry's authored values stand under `### Rigor` and its
derived line and trace under `### Strength`; the statement marker is `<!-- claim-rigor: … -->`. A
claim's and an analysis's entry carries both headings; a work's carries `### Rigor` alone, having no
derived value.

**The `1.0.0 → 2.0.0` migration.** One converter reads a `1.0.0` KB's files as bytes and writes the
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

**The `2.0.x → 3.0.0` migration.** One converter reads a `2.0.x` KB's files as bytes and writes the
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
  warning; one listed and marked nowhere has no host, `verify`'s `statement marker` fault. An
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
  write replaces it. The stamped `AGENTS.md` and `CONVENTIONS.md` are not read: the save writes
  both whole from the current templates, a legacy `AGENTS.md`'s declarations going with its old
  text.

**The migration announces itself.** A verb that reads a KB through the loader on an older version
writes one sentence to stderr, and each warning on a line of its own, and carries one `migration`
record — `from`, `to`, `note`, `warnings` — after `kb-root` (§7), on every such call until the call
whose save stamps `3.0.0`. `note` is, for every migration, "every rigor and bearing that score judges
is now marked model-scored, so a default score run re-judges it; to keep one, re-assert its current
value with set-rigor or set-bearing, only where the user asks". §7 names the verbs. Over MCP the
document is the channel and stderr is the server's log; `status` and `cancel` load no KB and never
carry it. A `3.1.0` and a `4.0.0` KB are refused `check: kb-format`.

**The `demoted` relation**, added at `1.0.0`. An edge class only the build's cycle walk and
`refresh`'s repair of a degraded load write, and only the build's analysis split (§5) moves: a
scoring edge one of them cuts to break a cycle is replaced by a `demoted` record that keeps it whole
— its `source`, `target` and `context`, and as `replaced-relation`, `bearing` and `manually-scored`
the replaced edge's relation and values — in the replaced edge's direction. `replaced-relation` is
`supports`, `lifts` or `refutes`: a `founds` source, a work or an axiom, has no incoming scoring
edge, so it closes no cycle. A `demoted` record stands outside the premise walk and the acyclicity
check exactly as `references` does and carries no strength. The build writes cuts through two
build-only write ops, `add-build-edges` and `demote-claim-edge`, and moves one onto the analysis it
splits from a claim through a third, `split-claim`. None is a subcommand or among the public ops,
so the public write surface cannot create one. `demote-claim-edge` replaces each standing scoring
edge of a pair with its `demoted` record; where that record already stands it removes the edge
standing beside it, where only `demoted` records stand it leaves the pair as it stands, and a pair
with neither is refused. `resolve-demoted` (§8) removes a `demoted` record or restores the edge it
replaced exactly, refusing a restore that would close a cycle (`check: dependency-cycle`, the entry
number and the path) and a restore where neither a `demoted` record nor the replaced edge stands.
`verify` lists every `demoted` record under `findings` (§7) with outcome `done`; `deps` reports it
as a row of its own relation, out of its source; `stats` counts them as `demoted_edges`, after
`edges`, which counts every row; the sheet draws each as a cut (§3); and its target's register entry
lists it.

**Cycles: cut as the walk finds them.** One walk serves the build's ring row, the degraded load's
resolution and `refresh`'s repair, so all three cut the same edges.

- The walk order is the node order of the register pages, which no minted id decides, so a build's
  cuts are reproducible modulo ids (§5): hosted nodes in the reading order of their markers, then
  every other node by id — a work's and an axiom's id being its key or label, not minted. Edges
  compare by relation in the relations table's order, then source, then target, each end in that
  node order.
- The walk is depth-first over the `supports`, `founds`, `lifts` and `refutes` records, starting at
  each node and following each node's edges in the walk order; `demoted` records are not walked.
- Each time the walk closes a cycle — an edge reaching a node on the current path — it cuts one edge
  of that cycle: the first, in the walk order, of the cycle's `supports` edges into an analysis,
  where one stands; else the edge the walk is on. The cut edge becomes its `demoted` record, with a
  warning naming the edge and the cycle it broke; where a `demoted` record of its identity already
  stands, the cut edge goes and that record stands alone, as `demote-claim-edge` leaves it. Nothing
  else is dropped.
- After each cut the walk starts over, until no cycle remains, so the cuts depend on the edge set and
  the documents' order alone.

Every public op that adds a scoring edge refuses one that would close a cycle (§8), so outside the
build only a merge reaches the walk. Repairing a cycle takes at most two edits: remove or redirect
another edge of the cycle, then restore the demoted edge with `resolve-demoted`, which refuses a
restore that still closes a cycle.

---

## 11. The MCP server

`kbase mcp` serves kbase to any external agent harness that speaks the Model Context Protocol, over
stdio JSON-RPC. It is the subcommands, bound: every subcommand it serves is a tool of its own or an
operation of one of two family tools, with the same arguments and the same result, so an external
agent harness without MCP loses nothing but convenience. The exception is `rewind-build`, the one
listed tool — it has no subcommand behind it, reports position that `status` and the commit trail
already state, and changes nothing.

- **Tools.** Every subcommand is served, less `completion`, `help`, `mcp` itself and
  `wait` — a tool call would hold the turn for the build's or score run's duration — and less
  `probe`, a diagnostic whose tool would let an agent poll a model at will, and less `auth` and
  its subcommands — a key arrives on stdin, which no tool call has, and no tool accepts a key; plus
  `rewind-build`, which no subcommand stands behind. The queries and `render-citation` are the
  operations of one tool, `query`, and the write ops the operations of another, `write`; every
  other subcommand is one tool, named as the subcommand. Each operation stays a subcommand, unchanged
  on the command line. A command tool's input schema is derived from its command's arguments and flags (the global flags
  excluded — the server carries its own — but for `project-dir`, below), and a duration flag's
  argument is the Go duration string the command line spells (`30s`). A family tool's schema is
  `operation`, required, a string enum of its operations' names, and `arguments`, an object holding
  what the operation takes: for a query, its positionals and flags by name, derived as a command
  tool's schema is; for `render-citation` and a write op, one values entry, the op's closed
  vocabulary (§8) as its keys. A call writes that one entry, and `--no-refresh` is not offered. The
  family tool checks `arguments` against its operation's own schema: an unknown key, a missing
  required key or a value of the wrong type is a refusal document, `refused`, one item per fault,
  `check: usage`, `key` the argument's name, `allowed` every argument the operation takes and
  `remedy` naming `allowed` for the argument names and the operation's help resource for what it
  does and, for a write op or `render-citation`, its keys. A values entry's keys carry no types there, so
  a wrong value for a values key is the op's own refusal (§8), which names the key and the allowed
  set. An `operation` outside the enum, or an `arguments` that is not an object, fails the tool's
  schema. Every tool's schema, a family's included, carries one
  more property, `project-dir`: the project the call runs in, the git worktree root that holds
  `kb-root/` or will. The schema is rendered per mode (below): in TUI mode the property is
  optional, and a call that omits it runs in the server's located project; in GUI mode it is listed
  in `required`, and its description says only what it names. A call runs in the project it names where it names one, else in
  the server's located project; a call with neither is `refused`, `check: usage`, `key:
  project-dir`. A `project-dir` that is no git worktree root is `refused`, `check: worktree`, `key:
  project-dir`, `path` the absolute path; a relative one resolves against the server's working
  directory. A refusal over MCP carries `entry` as `1` and no
  `line` or `column`; its `key` keeps the CLI spelling (§7), or, where no command stands behind the
  tool, is the schema property name (`target` on `rewind-build`), as it is in a family tool's
  argument check. Arguments that fail the tool's
  schema (a missing required key, a wrong type, an unknown key) are JSON-RPC `-32602` errors, not
  refusal documents; a family tool's `arguments` are checked as above instead. A command declares
  itself read-only in its definition, and the tool carries that as its read-only annotation, a
  family tool where every operation declares it; `query` — every query and `render-citation` among
  its operations — `verify`, `status`, `show-config` and `models` are the read-only set, and
  `rewind-build` completes it, its annotation hand-set with its description because no command
  declares it. A family tool's description is one sentence, then where its operations' shared help
  is, `kbase://help/query` or `kbase://help/write`, and where one operation's is,
  `kbase://help/<operation>`. The table is one tool per family, one per other bound command, and
  that one listed tool, and a test derives the set from the commands' declarations and asserts it
  for equality, so a second listed tool fails as a missing command does.
- **Results.** The §7 document, as the result's text and as structured content. `isError` is set
  exactly when the document's exit code (§7) is not 0, so a refusal reaches the model as an error
  carrying its items and remedy while `bounded` and a completed `cancel` do not. A list query takes
  `limit` and `offset` among its `arguments`, with §7's semantics.
- **Two modes.** In TUI mode, `kbase mcp`, for a harness that runs kbase from inside the project,
  the server locates its project by searching upward from its working directory for the repository
  root, and a tool call naming no `project-dir` runs there; a server whose search finds no
  repository has no located project. In GUI mode, `kbase mcp --project-dir-required`, for a harness
  that serves many projects from one place, the server locates none: every call must name
  `project-dir`. The server takes no project flag: the global `--project-dir` given to `mcp` is a
  usage refusal (§7) before it serves. Startup never fails for want of a project.
- **`build` is detached.** The tool starts `kbase build` as its own process, from the root of the
  call's project, passing `--project-dir` where the call named one, in its own process group, with
  no stdin, configured as the server is (the environment,
  `--config-dir`, `--log-level` and `--log-file`, §9). The build's stdout and stderr are captured as
  files in the state store's `reports/` (§6). The tool waits until the build's progress record shows
  it has entered a stage, or until it exits, for at most 40 s, and returns by what happened first.
  The bound is the KB write-lock wait (30 s, §8) plus 10 s, because a build starting during a write
  waits that write out before its first stage; the build is visible to `status` while it waits:
  - *entered a stage* — outcome `done` with `kb-root`, `state-dir`, `pid`, `resume` and `wait`, the
    `resume` absent where the record's command does not read as a `kbase build` invocation (§6).
    `wait` is the exact shell command `kbase wait [--project-dir <root>] --state-dir <store> --pid
    <pid>`, carrying `--project-dir` where the call named one, shell-joined as `status` joins its
    `resume` (§6): it returns when the run is over and prints that store's
    `status` document, so a session whose client does not subscribe to `kbase://build` runs it in
    the background and is woken when it exits. `status` and `cancel` follow it;
  - *exited* — the build's own document, `isError` set by its outcome (§7);
  - *still starting at the bound* — outcome `failed`, with an item naming the state directory and
    saying the build is still starting; the build keeps running, and `status` reports it once it has
    begun.

  A relative `state-dir` argument resolves against the repository root; the server's own
  `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status`
  would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that
  refusal and starts no process. The capture files are not removed.
- **`score` is detached.** The tool starts `kbase score` as its own process, from the root of the
  call's project, passing `--project-dir` where the call named one, in its own process group, with
  no stdin, configured as the server is (the environment,
  `--config-dir`, `--log-level` and `--log-file`, §9). The run's stdout and stderr are captured as
  files in the state store's `reports/` (§6). The tool waits until the run's ledger and pid stand,
  or until it exits, for at most 40 s, and returns by what happened first. The bound is the KB
  write-lock wait (30 s, §8) plus 10 s, because a run starting during a write waits that write out
  before its first refresh; the run is visible to `status` while it waits:
  - *started* — outcome `done` with `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait`.
    `wait` is the exact shell command `kbase wait [--project-dir <root>] --state-dir <store> --pid
    <pid>`, carrying `--project-dir` where the call named one, shell-joined as `status` joins its
    `resume` (§6): it returns when the run is over and prints that run's
    document, so a session whose client does not subscribe to a run's progress runs it in the
    background and is woken when it exits. `status` and `cancel` follow it;
  - *exited* — the run's own document, `isError` set by its outcome (§7);
  - *still starting at the bound* — outcome `failed`, with an item naming the state directory and
    saying the run is still starting; the run keeps running, and `status` reports it once it has
    begun.

  A relative `state-dir` argument resolves against the repository root; the server's own
  `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status`
  would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that
  refusal and starts no process. The capture files are not removed.
- **`rewind-build` reports the trail.** The build's boundaries are commits (§6); the tool lists them
  itself, oldest first, each with `stage`, `commit` and `subject`, and names `head`, the commit the
  repository stands at. `dirty` is `false` on a clean worktree, else the changed paths of the whole
  worktree — not only the paths a build owns — the first 20 named and the rest counted in a final
  `+N more`; it is a warning of what a reset would lose, never a refusal. The document's keys come in
  this order: `outcome`, `kb-root`, `state-dir`, `head`, `boundaries`, `started-at`, `dirty`,
  `commands`, then `target` where one was named, then `resume-note` where the document has a
  sentence to add. That
  order is the document's own, because no flag table exists to derive it from, and both arguments —
  `target` and `state-dir` — are optional.
  `commands` is `git reset --hard <commit>` followed by the run's recorded command, shell-joined as
  `status` joins its `resume` (§6); a recorded command is rendered only where it reads as a
  `kbase build` invocation — at least two elements, `kbase` then `build` — so a record that fails
  that leaves `commands` the reset line alone, and `resume-note` says the record's command could not
  be confirmed as a kbase invocation. With no `target` the reset line names no commit. A `target` is a
  stage id or display name, resolved as `--through` resolves one, and names that stage's boundary in
  the trail, with `discards` counting the boundaries newer than it. `started-at` is the commit the
  trail's `start` boundary records (§6), with `label` "the clean, ready-to-build state before the
  build", and null where the trail records none. A `target` of `started-at` names it: `commands` is
  the reset line alone, since a build after that reset is fresh and names its own volume roots;
  `discards` counts every boundary; and `allowed` lists `started-at` where it is recorded. A
  `bounded` run's recorded
  command is printed here although `status` withholds it, because after a reset to an earlier
  boundary there are stages left to walk — the case that suppression excludes — and `resume-note`
  says so. Outcomes: `done` (0); `refused` (1) for a `target` no boundary carries — `check: usage`,
  `key: target`, `allowed` the boundary ids — for a directory outside a worktree, for a state store
  `status` would refuse, or for a trail recording a stage over an unrecorded earlier one, which
  carries `build`'s own refusal (`check: trail`, §6); `failed` (3) for a git failure. It is read-only:
  the reset and the re-issue are the caller's to run.
- **Resources.** The claim-graph sheets (§3) present on disk, the placeholder included, are listed
  by `resources/list` as `image/svg+xml` resources with URI `kbase://sheet/<kb-root-relative path>`;
  `resources/read` returns the SVG as text. A KB that exists but will not open lists no sheet: the
  list is answered without one, and that KB's unopenability is `status`' and `verify`'s to report
  when they are asked, not a list fault. Beside them `kbase://model-hints` is listed as one
  `text/markdown` resource holding the guidance on choosing the `[models]` ids (§9.2) — the same
  rows a created `config.toml` carries — and it is KB-independent text: it is listed whether a KB
  is found and opens, is found and will not open, or is not found at all, and what it states does
  not vary with one. A third resource, `kbase://build-trail`, is one `application/yaml` document:
  the `rewind-build` document for the repository the KB sits in, no `target` named, returned byte
  for byte as the tool returns it. It is KB-scoped where the guidance is not: it is listed whether
  or not the repository holds a boundary, and a repository it cannot read is reported in that
  document, not as a list fault. A fourth resource, `kbase://build`, is one `application/yaml`
  document: the document the `status` tool returns (§6), for the state store the `build` tool last
  launched from this server — the store that launch resolved, so a relative `state-dir` argument is
  the repository root's, read in that launch's project — and for the default store of the server's
  located project where this server launched none. It is listed always, as the trail is: a read
  before any build answers that document with `state: none`, and the server's located project,
  where it is no worktree, answers `status`'s refusal document. It does not fail. A server with no located project
  lists no sheet, and its `kbase://build-trail`, and its `kbase://build` until it launches a build,
  answer the refusal a call naming no project gets (`check: usage`, `key: project-dir`).
  `resources/subscribe` and `resources/unsubscribe` take and drop a subscription to it, held for
  the connection's life. `kbase://build` is the only URI a client may subscribe to:
  another URI the server lists is `-32602` and one it does not list is `-32002`, and a missing `uri`
  is `-32602`; a second subscribe to one already held changes nothing, and an unsubscribe of one not
  held is answered and changes nothing. While it is subscribed, the server sends
  `notifications/resources/updated` carrying that URI, once for each run of a build that is no
  longer progressing — `finished`, `bounded`, `cancelled`, `failed` and `interrupted`, the last
  because a build whose process died with no recorded end is exactly a session waiting for nothing —
  and nothing for `none` or `running`. A run is announced once, and a resumed build is a new run
  that is announced once; a run already over when the client subscribes is announced then, after the
  response that took the subscription, so the client holds the subscription before the frame
  arrives. The frame carries the URI alone: the document is read, never carried, so no frame goes
  stale. Delivery and queueing are the client's, not the server's: the server writes the frame once,
  when the run ends, and nothing here makes it promise the session is at a turn when it arrives. An
  unknown URI is JSON-RPC `-32002` (resource not found); a missing `uri` is `-32602`.
  `initialize` advertises the `resources` capability, its `subscribe` among the ones it carries.
  Each family tool has its help listed as one `text/plain` resource, `kbase://help/query` and
  `kbase://help/write`: its operations, listed from the commands it serves, then what they share,
  stated there as MCP's arguments spell it, in `kbase --help` as the command line spells it, and in
  no operation's own help — for `query`, how a query reads the KB and answers, how a list query
  pages, and how `render-citation` takes its arguments; for `write`, how a write op takes its
  arguments, one entry, and that over MCP the op's values and no-refresh flags do not apply and a
  refresh always follows. Each command a tool binds, an operation's included,
  then has its help listed as one `text/plain` resource with URI
  `kbase://help/<command>`, the command's name in place of `<command>`: exactly the text
  `kbase <command> --help` prints. These are KB-independent text, as the model guidance is: listed
  whether or not a KB is found or opens, and the same in every project. `rewind-build`, which no
  command stands behind, has none.
  Nothing else is served, a register page included: the KB's documents are the external agent
  harness's own files.
- **What the server never does.** It sends no sampling request and offers no prompts; no inference
  runs from any tool but `build` and `score`, and each tool's inference is its own — the build's
  decision and text calls, the score run's scoring calls. `models` reads the
  selected provider's model catalogue; `configure` reads that catalogue before it writes the model
  map, refusing a category `--model-map` leaves unassigned — `scoring` alone excepted where the
  target file already states one — and naming the catalogue as the ids to choose from, except under
  `--create` with no `--provider`, where it dials nothing and says the ids are unchecked. Neither
  asks a model anything. It writes nothing but JSON-RPC frames to stdout; diagnostics go to stderr
  and `--log-file`. It loads no configuration itself — the verbs it exposes load their own on each
  call (§9.3). The `build` and `score` children it starts inherit the server's environment and
  `--config-dir`.
- **Protocol.** One MCP revision, the one the server implements, pinned as one constant
  (ARCHITECTURE §12); an initialise naming another revision is answered with the server's for the
  client to decide; calls are answered one at a time, in order; EOF on stdin ends the server.
- **Instructions.** `initialize` always carries `instructions`, in both modes and whether or not the
  server located a project: what the server serves, and the rules every session keeps. Where it
  located a project, they open with where it is: over a repository whose `kb-root/` stands, the
  KB's path; over one whose `kb-root/` is yet to be made, the repository's, saying the KB can be
  built; the path is absolute. In GUI mode that opening is one line instead: name the project with
  `project-dir` on each tool call. A TUI-mode server that located no project opens with neither.
  The `kbase` skill is named as the workflow, `show-config` as the tool reporting
  configuration, and `kbase://help/<name>` as a tool's or an operation's full help. It states what two outcomes (§7)
  ask of the caller: a `retry` call is re-issued unchanged later, and a `failed` call is not
  re-issued, since re-issuing will not fix it. It forbids a session to read or
  write kbase's records and derived files — anything
  under `kb-root/.index/`, a claim-graph sheet, a `.dot` file — save testing that such a path
  exists; it names the `query` tool's operations as what reports what the records hold, the `write`
  tool's as what changes them, and `refresh` as what writes the sheets. A `claim-rigor.md`
  register page is the exception: `refresh` generates it to be read as prose, and a session takes no
  value from it and never edits it. Its last paragraph is the session rule `AGENTS.md` also states
  (§3): a session sets a rigor or a bearing, its current value included, only to a value the user
  gave for that node or edge, in their own words or quoted in its task; it leaves a value out when
  the user gave none, so kbase records it pending for `score` to judge; a value it writes is
  recorded as the user's, which `score` skips unless `--rescore-manual` names its pass; and `score`
  is how values are judged or re-judged. Reading a document's prose stays open, and so does reading a
  sheet as an MCP resource. Every tool it names is one the server serves. The text is an embedded
  file of the binary.
- **Registration** is the external agent harness's: one server entry, `kbase mcp` (TUI mode) or
  `kbase mcp --project-dir-required` (GUI mode). README gives entries for two external agent
  harnesses as examples.

---

## 12. Interim releases

**<release version> — the files configure inference; the served surface is smaller.**

- **Every command's help is an MCP resource**, `kbase://help/<command>`, the text its `--help` prints (§11).
- **`provider` and `[asks] readerConcurrency` leave `config.toml`.** A file still stating either
  fails strict load naming the key (§9.3); delete the line by hand — `kbase configure` cannot load
  the file to repair it.
- **The environment inference mode is gone.** `KBASE_API_BASE_URL`, `KBASE_API_KEY_FILE`,
  `KBASE_DECISION`, `KBASE_TEXT` and `KBASE_SCORING` are not read; `providers.toml` and
  `config.toml` are the one source, and a run isolated from your layers names its directory with
  `--config-dir` or `$KBASE_CONFIG_DIR` (§9). The role record carries no `back-end`, and
  `show-config` no `mode` or `environment` (§7).
- **`strength` replaces `weak-points` and `strength-below`; `edge-census` is gone.** `strength`
  lists claims between `--min` and `--max`, sorted by strength or by dependents, or with
  `--pending` the unscored ones (§8); `edge-census`' counts are `stats`' `relations`, without the
  per-bearing manual and model split (§7).
- **`drop-claim-node` deletes a claim in one call.** It removes every edge naming the claim with the
  node, so `drop-claim-edges` and `drop-inbound-edges` are no longer needed first; a mention in prose
  still refuses the drop (§8).
- **`render-claim-graph` is gone**: `refresh` draws the sheets and reports `sheet` and `dot` (§7, §8).
- **The queries and the write ops are one MCP tool each.** `query` runs a query or
  `render-citation`, `write` a write op: `operation` names it from an enum and `arguments` carries
  what its own tool took; a bad argument is a usage refusal naming it and the allowed set. Every
  subcommand stands on the command line, and what the queries and the write ops share is stated
  once, in `kbase --help` and in `kbase://help/query` and `kbase://help/write` (§11).

**0.9.0 — Messages is the default chat protocol.** A `providers.toml` entry silent on `api` speaks
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

**0.8.0 — the rigor ceiling is `1.00`.** The rigor tables' maximum, the most a write carries, and
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

**0.8.0 — a scoring charge cites line numbers.** A call numbers the lines of the text it carries, and
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

