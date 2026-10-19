# SPEC – kbase

The contract: what any compliant implementation of kbase must do. Consumer-facing outcomes only.
[ARCHITECTURE.md](ARCHITECTURE.md) states how this implementation meets it; [THESIS.md](THESIS.md)
states why the build has its shape.

Every statement here is decided; where a point was once left open, its ruling is written in place.

---

## 1. What kbase is

kbase is a Go binary that builds and maintains a knowledge base (KB) from LaTeX. One or more volume
roots go in, one KB comes out — its entry point listing every volume — and a set of maintenance
subcommands then operates on the living KB, at a shell or, through `kbase mcp` (§11), from any agent
harness that speaks MCP. A KB can also be created empty by `init` (§8) and filled by authoring with
those same subcommands — no volume, no LaTeX, no build: LaTeX stays the only input format of
`build`, and `init` is the zero case of the tree, not another input.

- A **volume root** is the top `.tex` file of a paper: the file `00README.json`'s `toplevel` entry
  names, else the sole file containing `\documentclass`.
- A **KB** is a `kb-root/` directory beside a repository's `.git`: the Markdown document tree, its
  metadata layer, and `.index/`.
- The **metadata layer** is what the write API renders into a KB: the YAML frontmatter,
  registers, Tier-2 markers, `.index/*.yaml` (the format of §10).
- The **load-bearing forms** are the Markdown shapes KB readers key on: the up-link line, the
  cross-reference anchor form, the `` ``` math `` fence, the citation span, and the labelled
  blockquote.

Input is LaTeX only; no other input format is in scope.

---

## 2. Given interfaces

| Interface | Observed version | Contract |
|---|---|---|
| pandoc | 3.12 (`pandoc-api-version` 1.23.1.2) | Required on the host, not shipped. |
| Graphviz `dot` | Any version | Optional on the host, found on `PATH`, not shipped. Draws the claim-graph sheets (§3); absent, `refresh` writes the placeholder at the root only and `render-claim-graph` says so (§3, §7). |
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
  kb-claim-scorer grades derivations.
- Exact: the metadata layer, the load-bearing forms, and the KB's placement (`kb-root/` beside
  `.git`; reading starts at `kb-root/entry-point.md` and descends through `index.md` files).
- Free and untested: presentation — wrap width, bullet characters and the like.
- Byte equality is asserted only where a named reader needs it:

| Byte-equal | Reader that needs it |
|---|---|
| Metadata rendering (register entries, frontmatter, markers, derived-field placeholders) | The metadata reader; refresh's placeholder recognition; the write ops' locate and splice |
| `.index/*.yaml` and derived fields | Refresh's freshness gate (dry-run refresh diffed against disk) |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | Refresh's precheck of the stamped documents (`check: agents-redirect`) |
| The `` ``` math `` fence opener | The inventory's display-maths fence reader |
| The up-link line | `kb.UplinkMarker` and the readers that key on it |
| The claim-graph sheets' DOT, against the DOT goldens (`internal/sheet/testdata/`) | The sheet writer's write-only-where-bytes-change rule: under the same `dot`, `refresh` and `render-claim-graph` leave an unchanged sheet's bytes as they are |

**Stamped documents.** `AGENTS.md`, `CONVENTIONS.md` and `README.md` give the maintenance commands:
kbase's subcommands.

**Formats.** A new KB is written in the current metadata format, `1.0.0` (§10). A KB in the
superseded `0.9.0` form (§10) is read through migration.

**Theorem numbering** is whatever pandoc prints.

### `claim-graph.svg`

A picture of the claim graph, drawn through Graphviz `dot` (§2). `refresh` and `render-claim-graph`
render every sheet on each run and write one only where its bytes change. The SVG's bytes are
Graphviz's; no check reads them for their content — the refresh test writes one DOT twice through
the same `dot` and asserts identical bytes, the write-only-where-bytes-change rule's basis — and no
other command's behaviour depends on them (ARCHITECTURE §6).

**Three kinds of sheet.**

- `kb-root/claim-graph.svg` is the full sheet of the whole KB, always drawn: every volume a cluster
  holding its attached nodes. The sheet's title, and the label of the cluster of claims registered
  at `kb-root/` level, is the entry point's first heading, verbatim; `kb-root` where it has none.
  The KB title is the graph's label across the top. A volume's cluster is titled from its `index.md`
  first heading, falling back to the directory name, with the subtitle "N nodes · M unattached".
- `kb-root/claim-graph-digest.svg` is the volume digest: one box per volume listing its node counts
  per kind on one line, its unattached count and its within-volume edge counts, and the cross-volume
  edges as bundles whose counts cover every edge class; each box links to the volume's index and, on
  a second line reading `claim graph →`, to its sheet. The full sheet links to the digest (`volume
  digest →`) and the digest to the full sheet (`← full claim graph`).
- `kb-root/<volume>/claim-graph.svg`, one per volume, a top-level directory holding an `index.md`:
  the volume's claims plus every claim one edge away, drawing only edges with an end in the volume.

The digest and the per-volume sheets exist only where two or more volumes hold nodes; the
`kb-root/`-level claims are not a volume, and a volume holding no node has no sheet and no digest
box. A one-volume KB has the full sheet alone. A digest or volume sheet the KB no longer calls for
is removed by the next render, and reported (§7); a file named `claim-graph.svg` directly inside a
top-level directory of `kb-root/` is by definition such a sheet.

**What is drawn.** Premise relations, plus the `strengthens`, `supports` and `rests-on` relations.
Other `references` edges are not drawn, except those the build's cycle breaking cut from `depends`,
which are the index's `demoted` rows (§10), drawn as cuts. A KB whose cuts are plain `references`
rows draws no cuts. A claim with no drawn edge is not
drawn as a node; it is listed in a table titled "unattached (N)", one row per claim with a link and
a tooltip. Of the drawn `depends` edges, only the transitive reduction appears; it checks an edge
only against the edges still kept. Of `depends` edges sharing a source and target, one is drawn
whatever their contexts.

- *Edges*: `depends` edges are **cited**, black solid, where the paper's text marks the dependency,
  or **inferred**, blue dashed, where the build found it with no mark. An edge source → target was
  found with no mark where the unmarked record answers (source, target) yes in that direction, or
  where the cull record's `links` hold its pair either way round (§6). Of the build records beside
  `kb-root/` the sheet reads only the unmarked record and the cull record's links, for that
  distinction; an absent record finds nothing. `strengthens`, `supports` and `rests-on` edges are
  each drawn in a style of their own, beside them. A **cut**, read from the index's `demoted` rows,
  is drawn dotted red, heavier where its origin is `cited` (or absent) than where it is `inferred`;
  its tooltip names the origin (`cut, cited`, `cut, inferred`), and the digest counts cuts by origin
  (`1 cited cut`) and draws a bundle of cuts at the cited weight. Both origins share one legend row,
  "cut from depends: part of a circle (cited / inferred)".
- *Nodes by kind*, eight, each with its own mark: a labelled block (bold frame), an equation, prose,
  a support (dashed rounded), an experiment (component shape), an invariant and an axiom (grey, bold
  frame), a work (note). An edge end that no record carries is drawn as a ghost box, dashed red,
  labelled with the id. Claims and supports are filled by solidity band, `*pending*` in grey. A
  node's tooltip reads `<id> [<kind>, <band>] <title>`, every backslash in it shown as written
  (`$\Gamma$` does not become the graph's name).
- *Links*: every node links to its register entry; a digest box links to the volume's index and
  sheet.
- A legend. The SVG root is `width="100%"` with a `viewBox`, so it scales to the viewer.

**Without `dot` on `PATH`**, no sheet is rendered: an existing sheet is left as it is, and where
`kb-root/claim-graph.svg` does not exist `refresh` writes a placeholder there — an SVG naming the
missing tool and, beneath it, `index sha256:` followed by the first 12 hex digits of the SHA-256 of
the `.index/*.yaml` files concatenated in sorted path order, deterministic. `refresh` says so in one
line on stderr; `render-claim-graph`'s result names the lack (§7).

---

## 5. Build behaviour

```
kbase build <volume-root>... [--bibliography FILE]... [--no-inference]
            [--through <stage>] [--state-dir DIR]
```

**Output and placement.** The KB is written only at `<git root>/kb-root/`. Build state is never
written into `kb-root/`.

**Fresh versus resume** is derived from the `kb-build:` commit trail (§6), never configured. A build
against a populated `kb-root/` with no `kb-build:` commit trail is refused. `build` refuses outside
a git worktree, naming `git init`. It refuses if the kbase-owned paths it commits are dirty and the
dirt is not an interrupted stage's own uncommitted work (§6), and only then. A build over a
non-empty trail is given the inputs its newest boundary records (§6) — the same volume roots and the
same `--bibliography` files, each in the same order — or it is refused before anything is restored
or written: one item per differing input, `check: inputs`, `key` `<volume-root>` or
`--bibliography`, `path` the repository root, `detail` naming what the trail records and what the
run was given (the paths added and removed, or the same paths in another order), remedy "resume with
the inputs the trail records, or start over from a commit before the trail". A finished build re-run
with other inputs is refused so, not `unchanged`. A newest boundary that records no inputs, written
before boundaries carried them, is not compared. A trail that records a stage while leaving an
earlier stage in walk order unrecorded was written by a build walking another stage order, and a
build over it is refused before anything is restored or written: one item, `check: trail`, `path`
the repository root, `detail` naming the recorded stage and the unrecorded one before it, remedy
"start over from a commit before the trail".

**Determinism.** The same inputs and pandoc version give a byte-identical document graph: the tree
`document-graph` writes and the records. The claim-graph stages mint fresh node ids on every build —
unique within the KB, now and going forward — so a build is reproducible modulo
ids from `claims-declared` on, and no consumer may read anything into an id's value.

**`--no-inference`** is first-class. It drops the rows that spend inference, still walks and records
every stage, and states the rows it dropped. The run is a real KB built without those rows, not a
stopped walk. Which rows drop follows the classification on the row (ARCHITECTURE §4), never a stage
id. A row that spends inference in part is not dropped: its asks are skipped and the rest of the row
still runs, as for `depends-attributed`.

**`--through <stage>`** bounds the run at the named stage. A bounded run is not a failed one:
outcome `bounded`, exit 0 (§7).

**Inference preflight.** A build with a stage left to walk that calls a model refuses (§7,
`refused`) before the first stage it walks when no provider is configured (§9), naming the
configuration it lacks; its `detail` names `kbase show-config` as the diagnostic that shows how the
configuration resolved (§9.6). A build spending no inference needs none. Every model call a build
makes is one tool-less chat request answered from its prompt alone — except the claim-graph letter
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
that set. **Degradation, not refusal.** An unreadable bibliography: the build continues without it
and reports the file.

**Under `--no-inference`** no `README.md` is written: the overview call is the dropped row, and the
README is assembled around its passage.

**Monitoring.** `build` runs in the foreground; an agent harness backgrounds it. `kbase status` and
`kbase cancel` (§6) observe and stop it.

**What a build produces**: Markdown leaves; the index, up-link and cross-reference structure;
load-bearing forms; claim-graph registers and nodes; the derived index. The build's stages and what
each does are in ARCHITECTURE, "Stage table"; how each computes its product is ARCHITECTURE's.

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
inbound edges, then its register entry, its marker and its id in every leaf's `claims`, a leaf whose
only claim it was restated no-claim with a fixed reason — and no edge is added. The stage asks no
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
node. *Supported by* writes a `depends` edge from source to target, *in support of* one from target
to source, and *mention* a `references` edge. A candidate whose ask yields no readable answer after
the re-ask, and every candidate under `--no-inference`, takes its draft: *supported by* where
containment directed the pair, *mention* otherwise. The stage writes every edge its answers make,
cycles included; last, every `depends` edge lying on a cycle is demoted (§10), before the stage's
refresh and verify.

**Records and inference.** The reader and placement facts the claim graph needs travel as records in
the build state store, not in `kb-root/` (ARCHITECTURE, "Records"). Each inference pass is one
closed-set decision per call under its embedded template (ARCHITECTURE, "Inference seams"); no stage
exits on a model's opinion. The letter asks are each one `POST` to the decision surface the selected
entry names (§9.1) carrying a closed-set question (`choice` or `noul`) and answered by probabilities
over that set — no reply text is parsed; the overview call remains one tool-less chat request
answered from its prompt alone.

---

## 6. Build state, ledger and monitoring

**Git holds the recoverable record.** Each stage boundary is a commit in the user's repository, with
subject `kb-build: <stage-id> | <display name>` and a structured body. Every boundary's body ends
with the build's inputs: a `volume-root: <path>` line per volume root, then a `bibliography: <path>`
line per `--bibliography`, each in the order given, the path repository-relative with forward
slashes. A commit carries `kb-root/` and the tracked build records, at the repository root:
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
op, `refresh` or `render-claim-graph` attempted while a build runs is refused (§8). Commits are
scoped by pathspec to kbase-owned paths.

**The state store holds what does not belong in the repository** — the pid and the holder lock
`status` and `cancel` read, `progress.jsonl`, per-call evidence, the docgraph records (§5), a
per-unit answer cache keyed by input hash, and the captured output of a build the MCP server starts
(§11). Nothing in it is authoritative for position. A clean exit removes the holder lock's file,
cancel included, so a file standing in the store is never evidence of a live holder. The store
lives at `$XDG_STATE_HOME/kbase/<key>/`, where `<key>` is the first 16 hex digits of the SHA-256 of
the absolute, symlink-resolved `kb-root/` path, overridable with `--state-dir`; it holds paid-for
inference and survives `git clean`. **The run lock is in the repository's git directory**,
`kbase-build.lock`, recording the state-dir: one build runs on a repository at a time, whatever
`--state-dir` each names. A second build is `refused` with `check: lock`, `path` the running build's
state-dir, and no remedy; where the run lock is held but names no state-dir yet (the build is
starting), the second build gets `retry`. Every lock is advisory and exists on Unix only; on Windows
`status` cannot see a running build and no lock-based refusal or `retry` is given.

**The state store is versioned.** `<state-dir>/format` holds one integer, currently `1`, written
when a build first holds the store; a store with no `format` file is version 1. `build`, `status`,
`wait`, `cancel`, the MCP `build` tool and `rewind-build` (§11) refuse a store newer than this kbase:
`check: state-dir`, `key: --state-dir`, `path` the store, remedy "update kbase, or use another
--state-dir". A build converts an older store in place before it uses it; the conversion is
stepwise and resumes if interrupted. The store's version is independent of the KB format version
(§10).

**The state store is pruned only by a build.** At a build's start, the captured output of earlier
builds (`reports/build-*`) and the call captures (`scratch/captures/`) last modified before the
start of the fifth-latest build that ended are removed; a build that ended `refused` or `retry` does
not count. The answer cache and the stage reports are kept. `status` and `cancel` remove nothing.
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
Progress is persisted to a file so a late or reconnecting monitor catches up.

### `kbase cancel`

Signals the lock holder. The in-flight call is abandoned with nothing written for it; the event is
logged, the lock is released, and the build process exits `cancelled`. Cancel costs only the
in-flight unit; the build is resumable.

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
   them except `models`, `configure` and `show-config`; `init` omits it from results written while
   no `kb-root/` stands (§8).
3. The subcommand's own keys, in the order its table gives.
4. Exactly one of these, chosen by `outcome`: `refusals` for `refused` and `retry`; `failures` for
   `failed`; `cancelled` (a mapping of `stage` and `unit`) for `cancelled`.

Keys are kebab-case. The one exception is query data under `results`, which keeps the query data's
own names verbatim (`strengthen_by`, `mentioned_ids`). The same fact always uses the same key:
`written` (files this call wrote; kb-root-relative inside `kb-root/`, absolute outside it; `[]` when
nothing was written), `ids` (node ids, in entry order), `count` (a total before any limit), `resume`
(a shell command that continues the work), `state-dir` (the state store's absolute path).

**Item.** Every refusal, failure and verify finding is one mapping with these keys in this order, a
key omitted when it does not apply; `detail` is always present, with at least one of `check`, `path`
or `key`:

| Key | Holds |
|---|---|
| `check` | The named check or refusal class: a verify check name, `usage`, `provider`, `dirty-paths`, `include`, `metadata-key`, `lock`, `state-dir`, `inputs`, `trail`, `pandoc`, `unknown-id`, `kb-format`, `kb-root`, `worktree`, `export-dir`, `demoted`, `dependency-cycle` |
| `path` | The file the item is about |
| `entry` | 1-based index into the values' `entry` list |
| `key` | The setting the item is about: a values key, spelled as its configuration file spells it, or a flag or positional argument, spelled as `--help` spells it |
| `line`, `column` | 1-based position in the values document or in `path` |
| `allowed` | The closed set the value must come from |
| `remedy` | The command that clears the item, where one exists (e.g. `kbase refresh`, `git init`) |
| `detail` | One sentence |

A refusal lists every offending item. A usage error is one item with `check: usage`. A
provider-selection refusal names `key: --provider` and lists every entry name as `allowed`; a
category `configure` is not given names `key: models.<category>` (`models.decision` or
`models.text`) and lists the provider's model ids as `allowed`.

**Per-subcommand keys**, following `kb-root`:

| Subcommand | Keys |
|---|---|
| `build` | `state-dir`; `provider`, `provider-rule`, `provider-layer` (the endpoint's provenance: the pool entry it came from, the rule that named it — `flag`, `config` or `sole-entry` — and the layer that supplied the entry — `explicit`, `environment`, `repository` or `global`; the three are absent together where no pool entry supplied the endpoint: `--no-inference`, a build that resolved no provider, or a base URL the environment supplies); `through` (absent where this invocation walked no stage); `no-inference`; `resumed` (bool); `restored` (paths reset to the last boundary); `stages` (list of `stage`, `commit` — null if unrecorded — `dropped` (row ids), `report` (path to the stage's full findings in the state store); absent where this invocation walked no stage); `resume` (absent where an identical re-issue continues nothing: `done`, `unchanged`, `bounded`, `retry`, a `failed` or `cancelled` run that walked no stage, and a `refused` run that recorded no boundary) |
| `status` | `state-dir`; `state` (§6's vocabulary, `interrupted` included); `pid`; `started`; `updated`; `ended`; `no-inference`; `stages` (list of `stage`, `commit`); `current` (`stage`, `units-done`, `units-total`); `recent-refusals`, `recent-fallbacks` (items, the newest 10 of each); `cache-entries`, `cache-bytes` (integers: the answer cache's entry count and total size in bytes, §6); `resume` (absent for `finished`, `running` and `bounded`; read from the run §6 names) |
| `cancel` | `state-dir`; `pid`; `resume` (absent where the record's command does not read as a `kbase build` invocation, §6) |
| `init` | `written` (every path this call wrote — the spine, the stamped readiness documents and the refresh's output, merged, deduplicated, path-sorted); `removed` (as `refresh`'s; `[]` at birth) |
| `refresh` | `written` (includes every sheet written); `removed` (files the KB no longer calls for — a sheet the volume-count rule drops, a file a format migration made obsolete (§10) — kb-root-relative inside `kb-root/`, repository-relative beside it; `[]` when none) |
| `verify` | `findings` (items the KB's structure carries that are not faults — every `demoted` edge (§10), as items with `check: demoted`, `path` the register, `detail` the edge and its origin; `[]` when none); its faults are `refusals` items, a refresh-fixable item carrying `remedy: kbase refresh` |
| every write op | `ids` (inserts only; minted or adopted, in entry order); `minted` (the subset this call drew); `adopted` (list of `entry`, `id`, `differs` — values keys whose supplied value the existing entry does not carry); `written` (paths this call wrote); `refreshed` (paths; null under `--no-refresh`); `removed` (the trailing refresh's, as `refresh`'s; `[]` under `--no-refresh`) |
| `resolve-demoted` | as every write op; `resolved` (list of `source`, `target`, `action`: `removed` or `restored`) |
| `render-citation` | `citations` (one per entry, in entry order) |
| `deps`, `gated-on`, `cited-by`, `find`, `referenced-by`, `solidity-below`, `subtree`, `weak-points` | `count`; `offset`; `truncated` (bool); `results` (the query's list, in its own key names, sliced; `deps`' edge records, not `-i`'s ids, add `context` after `applicability`: the edge's context string, null where it has none) |
| `show`, `stats` | `results` (the query's mapping; `show`'s claim record adds `strengthen_by` after `strengthen_by_count`: the claim's strengthen-by items in `item_idx` order, each `item_idx`, `text`, `mentioned_ids` (`[]` when none)) |
| `render-claim-graph` | `written` (the sheets whose bytes changed); `removed` (as `refresh`'s); `sheet` (`drawn` where the sheets were rendered this call, else `placeholder`: nothing was drawn, whether or not a sheet stands); `dot` (`"not found on PATH"`, present only when `sheet` is `placeholder`) |
| `models` | `provider`; `provider-rule`, `provider-layer` (the rule that named the entry — `flag`, `config` or `sole-entry` — and the layer that supplied it, as `build`'s; this verb always resolves one); `models` (ids, sorted ascending) |
| `show-config` | `layers` (one record per directory this invocation read, in precedence order: `path`, `source` (`explicit`, `environment`, `repository` or `global`), `exists`, `providersExists`, `configExists` — the directory's presence and each file's); `providers` (one record per entry, in name order: `name`, `base-url`, `decisions-flavor` (the surface it speaks, `systemone` where the entry is silent), `key` (`file` or `none`), `key-file`, `layer` (the layer that supplied the entry), `usable`, `fault`; a provider a dropping fault removed has `name`, `layer`, `usable`, `fault` only); `config` (the merged choices: `provider`, `models` (`decision`, `text`, null where unset)); `environment` (the inference variable names this process sets, sorted); `selection` (`provider` and `rule` — `flag`, `config` or `sole-entry` — or `detail` where no entry was named); `inference` (`name`, `base-url`, `decision`, `text`, `sources` (one record: `base-url`, `decision`, `text`, `key` — the layer that supplied that field, one of `layers`' four names, null where the field is unset); null where it does not resolve); `diagnostics` (items: every fault, `path` the `providers.toml` that carries it, and the resolution failure where inference does not resolve) |
| `configure` | `provider` (the entry configured, null where the configuration names none); `provider-rule`, `provider-layer` (as `models`'s; absent where no provider resolved, as under `--create` with no `--provider`); `models` (`decision`, `text`, as `config.toml`'s `[models]` spells them, null for a category left unset); `written` (the files this call wrote, `providers.toml` before `config.toml` where `--create` created both; `[]` with outcome `unchanged`) |
| `export-latex` | `written` (absolute paths; outside `kb-root/`); `volumes` (the volume directories exported, entry-point order); `report` (the path of the export report written under the out dir — one YAML list, every degradation and finding line the run produced, each naming the form, the source path and the reason) |

**Bounded results.** List queries take `--limit N` (default in ARCHITECTURE §12; `0` means all) and
`--offset N`; where `truncated` is true the caller re-issues the query with `--offset` set to
`offset + len(results)`. `stats`' ranked sections stay at 10. No other subcommand's result
grows with the KB; `build`'s per-stage detail goes to the `report` file, not stdout.

**Exit codes** derive from `outcome`:

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success, including a `--through`-bounded run |
| `refused` (including verify faults and usage errors) | 1 | Wrong input or KB state; nothing written past the last checkpoint |
| `retry` | 2 | Another writer holds the KB write lock past the wait (`check: lock`, `path` the repository root), or another build holds the state store (its holder lock, §6); re-issue identically later. A running build is not `retry`: it is `refused` with the `lock` item (§6) |
| `failed` | 3 | Defect in the tool, its input or the environment; re-issuing will not fix it |
| `cancelled` | 4 | Stopped on request; resumable |

---

## 8. Maintenance subcommands

Every subcommand below is a builtin tool an agent harness can call. The rules in this section govern
each.

**Values input.** YAML (JSON accepted) on stdin or `--values`. The per-op key vocabulary is closed,
and stated once, in kbase's write package; it is not copied into prose. A refusal names the key, its
position, and the allowed set, and `kbase <op> --help` describes the op. A prose value containing
U+0008 or U+000C is refused.

**Idempotence.** Every mutating op is idempotent under re-issue: the same values issued twice leave
`kb-root/` byte-identical to issuing them once, and the second call reports `unchanged`. An insert
adopts an existing entry of the same register and title (an insert's values name no host; the host
is assigned later through `set-frontmatter` and the marker); a work is adopted by its key. The one
exception is `init`: creation is birth-once (below), and a re-issue where a `kb-root/` stands
is `refused`, never `unchanged`.

**Writes leave nothing stale.** A mutating op ends with a refresh unless `--no-refresh` is given.

**One write lock.** A write op, `refresh` and `render-claim-graph` hold the KB write lock for the
whole run: from loading the KB through the trailing refresh and the sheets to the result. `init`
holds it from before `kb-root/` exists — the lock sits on the directory `kb-root/` stands in, so it
is available at birth — through the skeleton, the trailing refresh and the birth commit to the
result. A writer that cannot take it within 30 s gets `retry` with `check: lock`, `path` the
repository root, and the remedy. A write verb probes for a running build (§6) before it waits on the
write lock and again after taking it: where a build runs it is `refused` with the `lock` item at
once, taking no wait. A build waits out a write in progress, up to the same 30 s, before it writes
(`retry` if it cannot); its own writes take no lock. `render-citation`, `verify` and the queries take
no lock. On Windows no
advisory lock exists (§6): neither the `retry` nor the build-running refusal is available there, and
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
frontmatter `kind: entry-point`, stamped (§10), with the `--title` text, or the repository's
directory name, as the only heading and no children — and `kb-root/.index/`, empty; stamps the
readiness documents, `AGENTS.md`, `CONVENTIONS.md` and the `CLAUDE.md` redirect, each only where
absent, `AGENTS.md` pinned that this KB is authored, not distilled — what it holds its author writes
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
`entry` items of `id` (the source claim), `target`, and `action`: `remove` or `restore`. `remove`
deletes the `demoted` row; `restore` rewrites it as a `depends` edge, refused (`check:
dependency-cycle`, `entry` the item's number, `detail` naming the path `target → … → id → target`)
where that would close a cycle, and refused where the pair has neither a `demoted` nor a `depends`
edge. A batch naming the same (`id`, `target`) pair twice is refused whatever the actions: the
item's `entry` is the repeat's number, its `key` is `target`, and its `detail` names the first
entry. Idempotent: a re-issue is `unchanged` — a pair with no `demoted` row and `remove`, or already
a `depends` edge and `restore`. Runs the trailing refresh as every write op does.

### `insert-claim-entry`
### `insert-support-entry`
### `insert-experiment-entry`
### `insert-work-entry`
### `set-work-strength`
### `set-applicability`
### `set-rigor`
### `set-rationale`
### `add-depends-on`
### `set-frontmatter`
### `mark-claim-in-leaf`
### `set-on-point-fraction`

An insert creates a missing register only under `--create`.

### `drop-claim-edges`

Removes a claim entry's outgoing `depends-on` or
`references` bullets, the class named or, where none is, both — each bullet with its continuation
lines — and a list's header where neither bullet nor other text is left; an absent list stands. The
`demoted` class is refused: demoted rows are `resolve-demoted`'s to remove. Idempotent: a re-issue
is `unchanged`.

### `drop-inbound-edges`

Removes, from every register entry holding one, each `depends-on` or `references`
bullet naming the claim, with its continuation lines, and a list's header where neither bullet nor
other text is left; an entry naming no such bullet stands. Idempotent: a re-issue is `unchanged`.

### `drop-claim-node`

Deletes a claim node entirely: its register entry block, its claim-quality marker
from every leaf carrying it, its id from every leaf claiming it, and every other entry's demoted
row naming it. Refused, writing nothing, where any entry's depends-on or references still names the
node — run `drop-claim-edges` and `drop-inbound-edges` first; where the call would empty a leaf's
claims list, the nodes one call drops from a leaf counted together, that being a no-claim decision
`set-frontmatter` makes; and where, once those removals are composed, any line under `kb-root/`
still names the node, each such line named as `path:line` — refresh's own roll-ups excepted, the
trailing refresh rewriting them. Idempotent: a re-issue removes whatever of the node still stands
(`done`) and is `unchanged` where nothing does.

### `render-citation`

Reads the cited document and writes nothing; returns the sanctioned authority-citation string — one
Markdown link whose text quotes the excerpt and whose target is the cited document at its anchor,
spelled relative to the citing document: `["<excerpt>"](<path>#<anchor>)`.

### Queries

Each query reads `kb-root/.index/` and writes nothing; `kbase <query> --help` states its arguments,
and §7 gives its result keys.

### `deps`
### `gated-on`
### `cited-by`
### `find`
### `referenced-by`
### `solidity-below`
### `subtree`
### `show`
### `weak-points`
### `stats`

### `render-claim-graph`

Renders every claim-graph sheet (§3) from the index and the build records, writing those whose bytes
change. Without `dot`, §3's rule, and the result names
the lack. Result keys: §7.

### `export-latex`

```
kbase export-latex [--out DIR] [--volume NAME]...
```

Writes a LaTeX project from the KB's current state into `--out` (default `latex-export/` beside
`kb-root/`; a relative `--out` resolves against the working directory). Every KB file is read through the one loader (§10's stamp rules apply); nothing under
`kb-root/` is written. `--volume` is repeatable, each naming a volume directory under `kb-root/`;
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
volume has none, the register's works.

**Degradation, not refusal** (§3's `dot` posture): a reference whose label its volume does not
define, a link to a document not exported, and a citation with no bibliography entry each become
the text they carry; an image whose file is not in the KB becomes its alternative text; a figure or
table the export cannot rebuild, or one LaTeX cannot float, is written as far as it can be; and
maths the export cannot recover from the page is left out. Each carries a report line in the
export report, `gen-tex/kbase-export-report.yaml` (§7's `report` key). Idempotent: a re-issue over the tool's own previous export leaves the
export byte-identical and reports `unchanged`. The export takes no KB write lock — nothing under
`kb-root/` is written, so it reads as the queries do — and it runs no git. Result
keys: §7.

### `refresh`

Derives the metadata layer's derived fields and `.index/*.yaml` and writes every claim-graph sheet
(§3) on each run. Without `dot`, §3's rule. Result
keys: §7.

### `verify`

The standard check a running KB owes: the dead-link and
unknown-id gate over the documents under `kb-root/`, link targets resolved against the repository,
then the metadata gate. Faults are outcome `refused`; a `demoted` edge (§10) is a finding, listed
under `findings` with outcome `done`, and `findings` is present on both `done` and `refused`. A
`demoted` row whose origin is missing or not `cited` or `inferred` is a fault, refused under
`referential integrity`. The citation grammar is the build-time
check: the build's gate stage runs it after the standard check, and `verify` does not. Result keys:
§7.

### `status`, `cancel`

See §6.

---

## 9. Configuration

Two TOML files — `providers.toml` (§9.1, the endpoint pool) and `config.toml` (§9.2, the
choices) — read under the strict-load contract (§9.3), in one or two layers.
`<repository>/.kbase/` is read over `~/.config/kbase`; a directory `--config-dir` names, else
`$KBASE_CONFIG_DIR`, is instead the one directory read, so an explicit choice isolates a run from a
repository's own `.kbase/`. An invocation outside a worktree has no repository layer:
`show-config`, `models` and `configure` run in any directory.

**Merging.** The two files merge by different rules. `providers.toml` merges by entry name and an
entry is replaced whole: a repository entry of the same name supersedes the global one entirely,
never field by field, so a repository cannot inherit a credential for an endpoint it chose. A fault
naming an entry a higher layer superseded is not reported: the pool's entry is the higher one, and a
diagnostic about a value nobody reads is not a fact about this run.
`config.toml` merges field by field: a value a higher layer states wins, and one it leaves out keeps
the lower layer's value — the rule the environment's variables follow below. A layer that fails the
strict load refuses the whole load, naming its file: no lower layer is read, so a broken repository
file never silently falls through to the global one.

**A leading `~` expands; nothing else does.** In the `--config-dir` and `$KBASE_CONFIG_DIR` values,
in an entry's `apiKeyFile`, and in `$KBASE_API_KEY_FILE`, `~` alone is the home directory and `~`
followed by a path separator is a path beneath it. `~user` is not supported, a `~` anywhere else is
literal, and where the home lookup fails or its answer is not absolute, the string stays exactly as
written.

**Global flags.** Every subcommand accepts `--config-dir DIR`, `--log-level debug|info|warn|error`
(default `warn`), `--log-file PATH` (tees diagnostics to a file; console output is never redirected
away) and `-h`/`--help`; the root command also accepts `-v`/`--version`, which prints exactly the
version string and a newline. A directory `--config-dir` or `$KBASE_CONFIG_DIR` names is the only
one read, the flag before the variable; otherwise configuration is read from the two layers above.

**Provider commands.** `kbase models [--provider NAME] [--timeout DURATION]` lists the selected
provider's model identifiers, sorted ascending, in its result document (§7). `kbase configure
[--create] [--project] [--provider NAME] [--model-map CATEGORY=ID]... [--timeout DURATION]` assigns
a model id to each category from `--model-map` alone — the categories are `decision` and `text`; a
category the flag does not name is refused naming `models.<category>` with the provider's model ids
as `allowed` (§7) — and writes `config.toml` under §9.5. `kbase show-config [--provider NAME]`
reports how the configuration resolves and changes nothing (§9.6). Each writes its result under §7.
Provider selection is shared by all three: `--provider`, else `config.toml`'s `provider`, else the
sole `providers.toml` entry. Where no pool entry is named — zero usable providers, two or more with
none selected, or a `--provider` the pool does not hold — `models` and `configure` refuse. An empty
pool, and a name the pool does not hold, point at the `providers.toml` of the layer `configure`
writes into, the file an entry belongs in; the name the pool does not hold also carries `key:
--provider`, and the remedy `kbase configure --create` unless that provider faulted out of the pool,
where the fault's own reason stands instead. Two or more entries with none chosen list every entry
name as `allowed` with `key: --provider`. `show-config` reports any of these in its document and
still exits 0.

**`configure`'s write target.** `configure` writes `config.toml` — and, under `--create`, the files
it creates — into the directory a flag or variable named, whether or not it exists: the
`--config-dir` value, else `$KBASE_CONFIG_DIR`, the flag before the variable. `--project` names
`<repository>/.kbase/` instead, whether or not it exists, and outranks the variable; `--config-dir`
with `--project` is a usage refusal naming `--project`, the two naming different directories; and
`--project` outside a worktree is refused with `check: worktree`, `path` the directory the search
started from, and `git init` as the remedy. Otherwise `configure` writes the repository layer where
its directory already stands, and the global layer otherwise.

**`configure --create`** writes the target layer's `providers.toml` where no file stands, from the
annotated template — no other command creates that file — and overwrites no file's other content.
`config.toml` is created where absent on any `configure` run, and otherwise edited under §9.5, which
keeps every byte `configure` was not asked to change. Without `--provider` it makes no request, and
one line on stderr says the model ids went unchecked; with `--provider` the ordinary path runs —
select, dial, list the catalogue, warn on an id the catalogue does not offer. A refusal writes
nothing, and so does a dial or catalogue failure.

**The environment.** A subcommand that spends inference — and `show-config`, which reports the
names it reads — also reads `KBASE_API_BASE_URL`, `KBASE_DECISION`, `KBASE_TEXT` and
`KBASE_API_KEY_FILE`, so an integrator configures a build from the invoking environment with no
configuration directory at all. The environment overrides the configuration files: a variable given
overrides the corresponding field and nothing else, and no flag sets one of those fields. A flag
chooses the configuration directory or the pool entry, and §9.6's report names the rule behind each.
`KBASE_DECISION` and `KBASE_TEXT` name the decision and text categories where the configuration does
not. The inference preflight (§5) names each lacking setting by the key its configuration file
spells it with — `baseUrl`, `models.decision`, `models.text` — except where the lack is a variable's
doing, as `KBASE_API_KEY_FILE` naming an unreadable file is, and then by that variable. There is no
plaintext-transport switch: when a key is configured and the base URL is `http://` to a host other
than loopback, one line on stderr says the key travels in cleartext, and the build proceeds (as
§9.4's permission warning does).

### 9.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses:

```toml
[<name>]
baseUrl = "..."        # required
apiKeyFile = "..."     # path to a key file, resolved relative to this file's
                        # own directory if relative; an entry's only key field
type = "inference"     # optional; the only accepted value, also the default
api = "openai"         # optional; the only accepted value, also the default
decisionsFlavor = "systemone"  # optional; "systemone" (also the default) or
                               # "decisions" — the decision surface this
                               # server speaks
```

No other key is recognized inside an entry. An entry that fails validation (empty `baseUrl`, or
`type`/`api`/`decisionsFlavor` set to anything but its accepted value) is **dropped** with a warning
naming the entry and the reason (never key material); the rest of the pool still loads. A key-file
read failure faults just that entry the same way. An absent `providers.toml` is an empty pool, not a
load error.

**The decision surface.** `decisionsFlavor` names the wire shape the decision call uses, chosen per
server: it changes nothing about chat, which is `chat/completions` under either shape, and it binds
no model id — the decision category's id stays in `config.toml` (§9.2). An entry silent on it speaks
`systemone`, the shape kbase's letter asks were written against. `systemone`: `POST
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
provider = "..."             # active providers.toml entry name

[models]
decision = "..."               # model id for the decision category (a model the
                               # entry's decision surface serves)
text = "..."                   # model id for the text category

[asks]
readerConcurrency = 4          # optional; asks of one group in flight once its first has returned
```

A category left unset resolves to "not configured" — no defaulting, no guessing. `readerConcurrency`
is how many of one group's letter asks a build has in flight once the group's first ask has returned;
absent, it is 4. A value under 1 fails the load, naming `asks.readerConcurrency`. There is no `[dev]`
table; an old `[dev]` key fails strict load
(§9.3). A `[models]` table spelling `heavy`/`light` — the superseded category names — fails strict
load as unmodeled keys (§9.3); the remedy is a hand edit.

### 9.3 Strict-load contract

Both files are decoded strictly: any key or table not modeled by the schemas above fails the **whole
file's** load. The message names the file path and every offending key by its full dotted path (e.g.
`models.typo_key` — a superseded category name such as `models.heavy` fails the same way),
pluralizing "unknown key"/"unknown keys" when more than one. A malformed-TOML file fails with a
parser-detail message naming the file and line. Only key **names** ever appear in either message,
never values: a file may still hold a secret a hand edit left behind — a field no longer modeled, a
stray value — and the refusal that names its key is exactly where that value would be echoed.
`kbase configure` cannot repair a config file it cannot load: every subcommand loads configuration
before doing anything else.

### 9.4 Key-file permission warning

When a `providers.toml` entry sets `apiKeyFile` and the host is not Windows, kbase stats the
resolved key-file path. If its permission bits grant group-read or other-read, it prints a warning
naming the path and the offending mode and recommending `chmod 600`. The entry stays usable. This is
the only file-permission behaviour kbase applies to a file it did not create.

### 9.5 `configure`'s targeted update (byte-preservation contract)

`kbase configure` never re-serializes `config.toml`; it edits the existing bytes in place.

- An absent or blank file gets a full commented template, with `provider`, `models.decision` and
  `models.text` filled in.
- Otherwise **only** the top-level `provider` key and the `[models]` table's `decision`/`text` keys
  are rewritten. Every other byte — comments, unrelated keys and tables, spacing, key order — is
  preserved exactly. A rewritten line's own trailing `# comment` is kept; its column position is not
  guaranteed.
- A missing `provider` key is inserted just after the leading comment block; missing
  `decision`/`text` keys are appended inside (or as a new) `[models]` table.
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
`done` whatever the report finds: a fault in the pool and an unresolved inference are diagnostics,
not refusals, so one call shows every gap at once.

The report states the layers this invocation read, in precedence order, each with its path, the
source that named it — `explicit` (a flag), `environment` (`$KBASE_CONFIG_DIR`), `repository` or
`global` — and whether the directory and each of its two files stand; `config.toml`'s merged
`provider` and both category ids, null where a category is unset; every pool entry in name order
with the layer that supplied it and the decision surface it speaks — its default filled in, so an
entry silent on the flavour reads `systemone` — including a provider a dropping fault removed
(unusable, with the fault's reason); the inference variable names this process sets (§9, "The
environment"); the pool entry the selection names and the rule that named it, or why no entry was
named; and the endpoint and category ids inference resolves to, or null with a `provider` item saying
what the configuration lacks. Each resolved field names the layer that supplied it, in the same four names `layers` uses:
`repository` or `global` for a value a file holds, `explicit` or `environment` for one read from the
one directory a flag or `$KBASE_CONFIG_DIR` named, and `environment` for one a `KBASE_*` variable
supplied. Its diagnostics are one `provider` item per fault, each carrying `path` the
`providers.toml` that carries the fault — which need not be the file a selection or `configure`
names — and the resolution's own failure where inference does not resolve, its `path` naming the
file the setting belongs in: the write target's `providers.toml` or `config.toml`.

**Names and paths, never secrets.** A key is reported by the kind it is — from a file, or none —
and a key file by the path its entry states, never by its content; an inference variable by its
name, never its value.

**It describes the process that ran it.** The layers, the environment and the selection are this
invocation's, so a call through the MCP server (§11) reports the configuration of the server
process.

---

## 10. Metadata format version

**The stamp.** A KB declares its metadata format as `kb-format: <major>.<minor>.<patch>` in the
entry point's frontmatter, semver. The superseded formats — the
comment-block frontmatter, the JSON build records, the JSONL index — are `0.9.0`, and a KB with no
stamp is `0.9.0`. Major: a reader built for the old major misreads the KB (a renamed field, changed
semantics, a changed anchor rule). Minor: additive; an old reader ignores the addition safely.
Patch: the bytes a writer produces changed, their meaning did not.

**Reading the stamp.** Before any other read, kbase reads `kb-format` alone from
`kb-root/entry-point.md`, recognising either frontmatter form; a KB with no entry point is refused
(`check: kb-root`) as it is today. Every producer of a KB stamps it: the build's spine, `init`'s
entry point at birth, and every save (below). A stamp at the current major and minor with another patch is read as current and
rewritten on the next save.

**One version read and written.** kbase reads and writes exactly the current version (ARCHITECTURE
§12). An older stamp is read through migration (below). A newer major or minor is refused by every
subcommand that reads the KB — `build` on an existing KB, `refresh`, `verify`, the write ops, the
queries and `render-claim-graph`; `status` and `cancel` do not load the KB — outcome `refused`, one
item `check: kb-format` naming the KB's version and kbase's, remedy "update kbase". No subcommand
writes an older format, and nothing downgrades.

**Migration happens at load and lands on the next write.** Reading an older KB converts it — the
KB's files as bytes in, the current form's files as bytes out, plus the set of repository-relative
paths the conversion made obsolete — version step by version step, in memory; every command then
sees the current form. A read-only command (`verify`, the queries) writes nothing. The next natural
write — `refresh`, or a write op's trailing refresh — writes every file the format covers in the
current form, the authored frontmatter of every document included, then removes the obsolete paths
less any the save itself wrote, then stamps the entry point last — the stamp is the mark that the
migration completed — reporting the removals under `removed` (§7). Every read of a KB file goes
through one loader, `internal/kbload`, which serves the migrated bytes to every command on an older
KB (ARCHITECTURE §11); `verify` reports an unmigrated KB stale from its stamp. A write op with
`--no-refresh` on an unmigrated KB is refused (`check: kb-format`, remedy `kbase refresh`): no save
mixes two formats. A save that fails before the stamp leaves the old form standing with some files
already current; the next load reads each file in the form it finds, and the next save completes the
migration. On an unmigrated KB `verify` reports the index stale with remedy `kbase refresh`, which
is the migration.

**`1.0.0`**, the first major bump, is one change of serialisation: standard YAML frontmatter (`---`
fenced, the same keys and values) in place of the comment block, with one change of shape — a leaf's
repeated per-node key groups (`exp-id`, `status`, `strengthens`; `sup-id`, `supports`) become one
mapping each in a top-level `experiment-nodes:` or `support-nodes:` list, keys and order kept, a
one-node leaf a one-element list; the build records as YAML (`kb-build-*.yaml`); `.index/*.yaml` as
YAML streams, one document per line, each the record as a JSON object after the `--- ` marker — a
valid YAML flow mapping that a reader holding only a JSON parser reads by stripping the marker — in
place of `.jsonl`; a writer never folds a record across lines. The YAML frontmatter stands at the
top of the file, so the up-link line (§3) is the first line after its closing `---`. **Recognising
frontmatter.** A document's leading `---` block is frontmatter only when its first non-empty line is
key-shaped: `name:` with a kebab-case name. Such a block that does not parse as a YAML mapping is
the malformed-frontmatter refusal. A leading `---` block whose first line is not key-shaped (a prose
paragraph, a list, the content after a thematic break) is prose, and the document has no
frontmatter. **The KB YAML dialect.** kbase writes, and a reader needs, only this subset: block
mappings and block lists, lists of flat mappings, plain or double-quoted scalars, no anchors, tags,
folded or multi-line scalars; flow style only for a list of scalars (`[clm-a, clm-b]`), an empty
list or mapping (`[]`, `{}`), and the JSON object of an index line; a scalar is never wrapped. A
reader of the dialect, in any language, is conformant when it reads the `1.0.0` golden pairs
(ARCHITECTURE §10) to the same values. The `0.9.0 → 1.0.0` migration converts the record and index
files under their new names and leaves nothing of the old form behind. Elsewhere this document
describes the `1.0.0` form; `.jsonl` and the comment block are `0.9.0`, the superseded form, named
only here.

**The `demoted` relation**, added at `1.0.0`. A claim-graph edge class only the build writes: the
build's cycle breaking records the `depends` edge it removes as `demoted` with its origin — `cited`
where the text marked the dependency, `inferred` where the build found it (§3's words) — in place of
turning it into `references`. The cuts are the dependency-attribution stage's ring row — every
`depends` edge on a cycle once attribution has written its edges. The origin is `inferred` where
the build found the cut edge with no mark, by §3's rule, else `cited`. It stands outside the
premise walk and the acyclicity check exactly as `references` does and carries no solidity. In the
register an entry carries a `- demoted:` list after `references:`, its items `- <target> — <title>
(origin cited)` or `(origin inferred)`, with an optional context, in the annotation style of the
other lists. In the index the row has `relation: "demoted"`, every field a `references` row
carries, then `origin` (`cited`, `inferred`, or null where the bullet has none). The build writes
cuts through two build-only write ops, `add-build-edges` and `demote-claim-edge`, neither a
subcommand nor among the public ops, so the public write surface cannot create one; `verify`
refuses a `demoted` row carrying no valid origin, so a KB's demoted edges are the build's and a
person's resolutions only remove them. `demote-claim-edge` turns an entry's existing `depends` edge
into a `demoted` row of the same entry carrying the origin the build computed for the pair; where
that row already stands it removes a `depends` edge still standing beside it, and otherwise leaves
the pair as it stands; it refuses a row standing with the other origin, naming both, and a pair
with neither a `depends` edge nor a `demoted` row. One write op, `resolve-demoted` (§8), removes
one or restores it to `depends`, refusing a restore that would close a cycle
(`check: dependency-cycle`, the entry number and the path) and a restore where neither a
`demoted` nor a `depends` edge exists. `verify` lists every `demoted` edge under `findings` (§7)
with outcome `done`; the queries report it under its relation wherever they report `references`
(`show`, `deps`, `referenced-by`); the sheet draws it as a cut, its origin in the cut's weight and
tooltip (§3); `stats` counts them as `demoted_edges`, after `depends_on_edges`, which still counts
every row.

---

## 11. The MCP server

`kbase mcp --kb-root <path>` serves one KB to any agent harness that speaks the Model Context
Protocol, over stdio JSON-RPC. It is the subcommands, bound: every tool but one is one subcommand,
with the same arguments and the same result, so a harness without MCP loses nothing but convenience.
The exception is `rewind-build`, the one listed tool — it has no subcommand behind it, reports
position that `status` and the commit trail already state, and changes nothing.

- **Tools.** One per subcommand, named as the subcommand, less `completion`, `help`, `mcp` itself
  and `wait` — a tool call would hold the turn for the build's duration — plus `rewind-build`, which
  no subcommand stands behind. A tool's input schema is derived
  from its command's arguments and flags (the global flags excluded; the server carries its own),
  and a duration flag's argument is the Go duration string the command line spells (`30s`). A write
  op's schema is one entry: the op's closed vocabulary (§8) as properties, no others allowed, plus
  `create` where the CLI offers `--create`; a call writes that one entry, and `--no-refresh` is not
  offered. A refusal over MCP carries `entry` as `1` and no `line` or `column`; its `key` keeps the
  CLI spelling (§7), or, where no command stands behind the tool, is the schema property name
  (`target` on `rewind-build`). Arguments that fail the tool's schema (a missing required key, a
  wrong type, an unknown key) are JSON-RPC `-32602` errors, not refusal documents. A command
  declares itself read-only in its definition, and the tool carries that as its read-only
  annotation; the queries, `verify`, `status`, `render-citation`, `show`, `show-config` and
  `models` are the read-only set, and `rewind-build` completes it, its annotation hand-set with its
  description because no command declares it. The table is the bound commands plus that one listed
  tool, and a test asserts the set for equality, so a second listed tool fails as a missing command
  does.
- **Results.** The §7 document, as the result's text and as structured content. `isError` is set
  exactly when the document's exit code (§7) is not 0, so a refusal reaches the model as an error
  carrying its items and remedy while `bounded` and a completed `cancel` do not. List queries take
  `limit` and `offset` with §7's semantics.
- **`--kb-root`** names a kb-root directory, or a repository root (a directory holding `.git`) with
  or without a `kb-root/` in it. Any other path is a usage refusal (§7) before the server serves.
- **`build` is detached.** The tool starts `kbase build` as its own process, from the repository
  root, in its own process group, with no stdin, configured as the server is (the environment,
  `--config-dir`, `--log-level` and `--log-file`, §9). The build's stdout and stderr are captured as
  files in the state store's `reports/` (§6). The tool waits until the build's progress record shows
  it has entered a stage, or until it exits, for at most 40 s, and returns by what happened first.
  The bound is the KB write-lock wait (30 s, §8) plus 10 s, because a build starting during a write
  waits that write out before its first stage; the build is visible to `status` while it waits:
  - *entered a stage* — outcome `done` with `kb-root`, `state-dir`, `pid`, `resume` and `wait`, the
    `resume` absent where the record's command does not read as a `kbase build` invocation (§6).
    `wait` is the exact shell command `kbase wait --state-dir <store> --pid <pid>`, shell-joined as
    `status` joins its `resume` (§6): it returns when the run is over and prints that store's
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
- **`rewind-build` reports the trail.** The build's boundaries are commits (§6); the tool lists them
  itself, oldest first, each with `stage`, `commit` and `subject`, and names `head`, the commit the
  repository stands at. `dirty` is `false` on a clean worktree, else the changed paths of the whole
  worktree — not only the paths a build owns — the first 20 named and the rest counted in a final
  `+N more`; it is a warning of what a reset would lose, never a refusal. The document's keys come in
  this order: `outcome`, `kb-root`, `state-dir`, `head`, `boundaries`, `dirty`, `commands`, then
  `target` where one was named, then `resume-note` where the document has a sentence to add. That
  order is the document's own, because no flag table exists to derive it from, and both arguments —
  `target` and `state-dir` — are optional.
  `commands` is `git reset --hard <commit>` followed by the run's recorded command, shell-joined as
  `status` joins its `resume` (§6); a recorded command is rendered only where it reads as a
  `kbase build` invocation — at least two elements, `kbase` then `build` — so a record that fails
  that leaves `commands` the reset line alone, and `resume-note` says the record's command could not
  be confirmed as a kbase invocation. With no `target` the reset line names no commit. A `target` is a
  stage id or display name, resolved as `--through` resolves one, and names that stage's boundary in
  the trail, with `discards` counting the boundaries newer than it. A `bounded` run's recorded
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
  the repository root's — and for the served repository's default store where this server launched
  none. It is listed always, as the trail is: a read before any build answers that document with
  `state: none`, and a served directory that is no worktree answers `status`'s refusal document. It
  does not fail. `resources/subscribe` and `resources/unsubscribe` take and drop a subscription to
  it, held for the connection's life. `kbase://build` is the only URI a client may subscribe to:
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
  Nothing else is served: the KB's documents are the harness's own files.
- **What the server never does.** It sends no sampling request and offers no prompts; no inference
  runs from any tool but `build`, and `build`'s inference is the build's own. `models` reads the
  selected provider's model catalogue; `configure` reads that catalogue before it writes the model
  map, refusing a category `--model-map` leaves unassigned and naming the catalogue as the ids to
  choose from — except under `--create` with no `--provider`, where it dials nothing and says the ids
  are unchecked. Neither asks a model anything. It writes nothing but JSON-RPC frames to stdout;
  diagnostics go to stderr and `--log-file`. It reads no harness configuration and names no harness.
- **Protocol.** One MCP revision, the one the server implements, pinned as one constant
  (ARCHITECTURE §12); an initialise naming another revision is answered with the server's for the
  client to decide; calls are answered one at a time, in order; EOF on stdin ends the server.
- **Instructions.** `initialize` carries `instructions`: what the server serves, where it is, and
  what carries a session on from there. Over a repository whose `kb-root/` stands it names the KB's
  path; over one whose `kb-root/` is yet to be made it names the repository's and says the KB can be
  built; where no repository holds the served directory the field is absent. Either path is
  absolute, the `kbase` skill is named as the workflow, and `show-config` is the one tool it names.
- **Registration** is the harness's: one server entry naming the command `kbase mcp --kb-root
  <absolute path>`. README gives the entry for two harnesses as examples.
