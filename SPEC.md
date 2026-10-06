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
harness that speaks MCP.

- A **volume root** is the top `.tex` file of a paper: the file `00README.json`'s `toplevel` entry
  names, else the sole file containing `\documentclass`.
- A **KB** is a `kb-root/` directory beside a repository's `.git`: the Markdown document tree, its
  metadata layer, and `.index/`.
- The **metadata layer** is what kb_tools' write API renders into a KB: the YAML frontmatter,
  registers, Tier-2 markers, `.index/*.yaml` (the format of §10).
- The **load-bearing forms** are the Markdown shapes KB readers key on: the up-link line (kb_tools
  `UPLINK_MARKER`), the cross-reference anchor form, the `` ``` math `` fence, the citation span,
  and the labelled blockquote (kb_tools SPEC, "The Document-Tree Contract", points 3, 7, 9, 10, 12).

kbase builds exactly what kb_tools builds. Input is LaTeX only; no other input format is in scope.
Two consumers use the binary: a person at a shell, and personant (a sibling Go project) whose model
calls kbase subcommands as builtin tools. kbase is personant's peer; personant's rules govern
personant.

---

## 2. Given interfaces

| Interface | Observed version | Contract |
|---|---|---|
| kb_tools (reference implementation) | adjagent `dev` at `35fa6cb326f811910dc027249482e47b22d50c3e`, the clone at `.claude/adjagent/` from which this repository's agent set is installed | Every kb_tools citation in this repository's documents names this commit and is read from `.claude/adjagent/kb_tools/`. Sections cited below are kb_tools' SPEC unless noted. |
| pandoc | 3.12 (`pandoc-api-version` 1.23.1.2) | Required on the host, not shipped. |
| Graphviz `dot` | Any version | Optional on the host, found on `PATH`, not shipped. Draws the claim-graph sheets (§3); absent, `refresh` writes the placeholder at the root only and `render-claim-graph` says so (§3, §7). |
| git | 2.56.0 | Required on the host. The recoverable record of a build (§6). |

**pandoc.** The accepted range's lower bound is 3.12. Its upper bound is the next major
`pandoc-api-version`, exclusive, where major means the first two components under Haskell's
versioning policy: 1.23.x is accepted, 1.24 is not. Only the owner widens the range. The range and
the `pandoc-api-version` are enforced at preflight; a missing pandoc is refused with an error naming
pandoc's install page. kbase runs pandoc LaTeX to JSON with no filters, and with `--citeproc`
exactly when a bibliography is offered, as kb_tools does.

**kb_tools' KB contract**, cited by section at the pinned commit:

- "What a KB Is"
- "The Document-Tree Contract"
- "Claim-Graph Nodes and Edges"
- "Derived Metadata, Defined"
- "The Claim-Graph Sheet"
- "Citation Grammar"
- "Project Scoping"
- "The Write API's Contract"

kbase restates none of them. A KB kbase produces satisfies them except where §4 names a divergence.

**Reference roles.** kb_tools' KB contract, in the sections cited above, is what both toolchains
build to. For everything else the two may differ on — the inference layer's prompts and algorithms,
the tool interface, maintenance-op behaviour — kbase is the canonical design and kb_tools tracks it:
a §4 divergence is a difference kb_tools has not yet adopted, or one named as permanent there. The
one change kb_tools leads on is a defect fix for a fault found in use; kbase reproduces the fault by
test and adopts the fix.

---

## 3. Compatibility contract

- A KB kbase builds is navigable by kb_tools and the kb-docent agent (Claude Code or opencode) and
  maintainable by kb-maintainer.
- A KB kb_tools builds is fully queryable and modifiable by kbase.
- kb_tools' build-pipeline intermediates are out of scope.
- Exact: the metadata layer, the load-bearing forms, and the KB's placement (`kb-root/` beside
  `.git`; the docent starts at `kb-root/entry-point.md` and descends through `index.md` files).
- Free and untested: presentation — wrap width, bullet characters and the like.
- Byte equality is asserted only where a named reader needs it:

| Byte-equal | Reader that needs it |
|---|---|
| Metadata rendering (register entries, frontmatter, markers, derived-field placeholders) | kb_tools' parser; refresh's placeholder recognition; `kb_write`'s locate/splice |
| `.index/*.yaml` and derived fields | Each toolchain's freshness gate (dry-run refresh diffed against disk) |
| `kb-root/CLAUDE.md` exactly `@AGENTS.md` | kb_tools' `kb_index_lib.unmigrated_agents_file` (refresh and verify exit 2 otherwise) |
| The `` ``` math `` fence opener | kb_tools' fence scanner |
| The up-link line | `UPLINK_MARKER` |
| The claim-graph sheets' DOT, against kb_tools' `claim_sheet.py` goldens | The sheet writer's write-only-where-bytes-change rule: under the same `dot`, `refresh` and `render-claim-graph` leave a sheet kb_tools drew unchanged |

**Acceptance bar.** Each toolchain's own checks run green over the other's output.

**Formats.** A new KB is written in the current metadata format, `1.0.0` (§10). A KB in the
superseded `0.9.0` form, which kb_tools wrote before it adopted `1.0.0`, is read through migration
(§10).

**Theorem numbering** is whatever pandoc prints, as in kb_tools.

### `claim-graph.svg`

A picture of the claim graph, drawn through Graphviz `dot` (§2). `refresh` and `render-claim-graph`
render every sheet on each run and write one only where its bytes change. The SVG's bytes are
Graphviz's; no check reads them for their content — the compat harness only compares two renders
of one DOT by one `dot` for identity — and no other command's behaviour depends on them (kb_tools
SPEC, "The Claim-Graph Sheet"; ARCHITECTURE §6).

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
rows (kb_tools-built, before kb_tools adopted §10) draws no cuts. A claim with no drawn edge is not
drawn as a node; it is listed in a table titled "unattached (N)", one row per claim with a link and
a tooltip. Of the drawn `depends` edges, only the transitive reduction appears; it checks an edge
only against the edges still kept. Of `depends` edges sharing a source and target, one is drawn
whatever their contexts.

- *Edges*: `depends` edges are **cited**, black solid, where the paper's text marks the dependency,
  or **inferred**, blue dashed, where the build found it with no mark (the unmarked-reference stage,
  ARCHITECTURE §4). Of the build records beside `kb-root/` the sheet reads only the unmarked record,
  for that distinction. `strengthens`, `supports` and `rests-on` edges are each drawn in a style of
  their own, beside them. A **cut**, read from the index's `demoted` rows, is drawn dotted red,
  heavier where its origin is `cited` (or absent) than where it is `inferred`; its tooltip names the
  origin (`cut, cited`, `cut, inferred`), and the digest counts cuts by origin (`1 cited cut`) and
  draws a bundle of cuts at the cited weight. Both origins share one legend row, "cut from depends:
  part of a circle (cited / inferred)".
- *Nodes by kind*, eight, each with its own mark: a labelled block (bold frame), an equation, prose,
  a support (dashed rounded), an experiment (component shape), an invariant and an axiom (grey, bold
  frame), a work (note). An edge end that no record carries is drawn as a ghost box, dashed red,
  labelled with the id. Claims and supports are filled by solidity band, `*pending*` in grey. A
  node's tooltip reads `<id> [<kind>, <band>] <title>`.
- *Links*: every node links to its register entry; a digest box links to the volume's index and
  sheet.
- A legend. The SVG root is `width="100%"` with a `viewBox`, so it scales to the viewer.

**Without `dot` on `PATH`**, no sheet is rendered: an existing sheet is left as it is, and where
`kb-root/claim-graph.svg` does not exist `refresh` writes a placeholder there — an SVG naming the
missing tool and, beneath it, `index sha256:` followed by the first 12 hex digits of the SHA-256 of
the `.index/*.yaml` files concatenated in sorted path order, deterministic. `refresh` says so in one
line on stderr; `render-claim-graph`'s result names the lack (§7).

---

## 4. Named divergences

A named divergence is a difference from kb_tools that this list names. This is the whole list of
differences in KB contents, maintenance-op behaviour and the tool interface; any unlisted difference
there is a defect. Build-orchestration differences are the "Not ported" list in ARCHITECTURE §4.

| Divergence | kbase | kb_tools |
|---|---|---|
| Values transport | YAML (JSON accepted) on stdin or `--values` (§8) | A TOML file |
| Exit codes | The table in §7 | kb_tools' own codes (e.g. write ops: 7 refused, 8 retry) |
| Build-state location | Outside the worktree (§6) | kb_tools' own run directory |
| Stamped KB documents (`AGENTS.md`, `CONVENTIONS.md`, `README.md`) | Give the maintenance commands for both toolchains: kbase's subcommands and kb_tools' `kb_util` ops | Give kb_tools' only |
| Tool results | YAML documents (§7) | `[kb-write] STATUS` report lines |
| Inference endpoint variables | `KBASE_API_BASE_URL`, `KBASE_MODEL`, `KBASE_API_KEY_FILE` (§9), beside the configuration files; a cleartext key is a one-line warning | `API_BASE_URL`, `MODEL`, `API_KEY_FILE`, the environment alone, with `ALLOW_HTTP` gating plaintext (kb_tools SPEC, "The Driver's Contract") |
| Re-issued insert | Adopts the existing entry of the same register and title, reports `unchanged` and names any field whose value differs; a work whose key is already keyed likewise `unchanged` (§8, idempotence) | A claim or support insert mints a second entry under the same title; a work re-insert is refused (7) |
| Query paging | A list query returns at most `--limit` results (default in ARCHITECTURE §12; `0` means all) from `--offset`, reporting `count` and `truncated` (§7) | Every result, always |
| Tool surface | The subcommands, and the same operations over MCP (§11) | The `kb_util` ops at a shell |
| Build inputs on the trail | Every boundary's body ends with the build's inputs (§6); a resume given other inputs is refused (§5) | Its bodies carry none, until it adopts, which closes this row |

---

## 5. Build behaviour

```
kbase build <volume-root>... [--bibliography FILE]... [--charter FILE] [--no-inference]
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
before boundaries carried them, is not compared.

**Determinism.** The same inputs and pandoc version give a byte-identical document graph: the tree
`document-graph` writes and the records. The claim-graph stages mint fresh node ids on every build —
unique within the KB, now and going forward, as kb_tools' are — so a build is reproducible modulo
ids from `claims-declared` on, and no consumer may read anything into an id's value.

**`--no-inference`** is first-class. It drops the rows that spend inference, still walks and records
every stage, and states the rows it dropped. The run is a real KB built without those rows, not a
stopped walk. Which rows drop follows kb_tools' classification (kb_tools ARCHITECTURE, "The Driver":
`--no-inference` "drops rows and bounds nothing", `spends_inference`; SPEC, "The Driver's
Contract").

**`--through <stage>`** bounds the run at the named stage. A bounded run is not a failed one:
outcome `bounded`, exit 0 (§7).

**Inference preflight.** A build with a stage left to walk that calls a model refuses (§7,
`refused`) before the first stage it walks when no provider is configured (§9), naming the
configuration it lacks. A build spending no inference needs none. Every model call a build makes is
one tool-less chat request answered from its prompt alone, as kb_tools (SPEC, "The Driver's
Contract").

**Refusals** (outcome `refused`):

- Unparseable source: names the paper and the reader error.
- Unloadable include: names each file and the line that named it.
- An unclassified metadata key: a key the rendering's metadata block carries that is outside
  kb_tools' content and apparatus lists (`kb_docgraph/outline.py`). A key pandoc's writer drops — an
  empty entry — reaches no block and is not refused, as in kb_tools.

**Bibliographies.** With no `--bibliography`, the build passes every `.bib` in any volume root's
directory, in sorted path order, one set offered to every volume, as kb_tools' driver gathers them;
`--bibliography` given replaces that set. **Degradation, not refusal.** An unreadable bibliography:
the build continues without it and reports the file.

**Under `--no-inference`** no `README.md` is written: the overview call is the dropped row, and the
README is assembled around its passage, as in kb_tools.

**Monitoring.** `build` runs in the foreground; personant backgrounds it. `kbase status` and `kbase
cancel` (§6) observe and stop it.

**What a build produces** is kb_tools' output, as listed in the cited sections of §2: Markdown
leaves; the index, up-link and cross-reference structure; load-bearing forms; claim-graph registers
and nodes; the derived index. The build's stages and what each does are in ARCHITECTURE, "Stage
table"; how each computes its product is ARCHITECTURE's.

**Records and inference.** The reader and placement facts the claim graph needs travel as records in
the build state store, not in `kb-root/` (ARCHITECTURE, "Records"). Each inference pass is one
closed-set decision per call under a template whose provenance the build's manifest states —
imported from kb_tools or authored here and adopted by kb_tools (§2, reference roles) (ARCHITECTURE,
"Inference seams"); no stage exits on a model's opinion.

---

## 6. Build state, ledger and monitoring

**Git holds the recoverable record.** Each stage boundary is a commit in the user's repository, with
subject `kb-build: <stage-id> | <display name>` and a structured body. Every boundary's body ends
with the build's inputs: a `volume-root: <path>` line per volume root, then a `bibliography: <path>`
line per `--bibliography`, each in the order given, the path repository-relative with forward
slashes. A commit carries `kb-root/` and the tracked build records, at the repository root:
`kb-build-charter.md`, the node-pass record `kb-build-node-pass.yaml`, the unmarked record
`kb-build-unmarked.yaml` and the classification record `kb-build-classification.yaml`, YAML at
`1.0.0` (§10). Resume position is read from the commit trail. A user may reset to a stage commit and
resume from there. Per-leaf work inside an inference stage stays uncommitted until the stage
boundary; a resume over an interrupted stage restores the kbase-owned paths to the last stage commit
and re-runs the stage. **A build in flight owns `kb-root/`:** a resume restores every path under it
to the last boundary, whatever was done to it since, and nothing vets what it finds there. An edit
made while a build is interrupted is the editor's to carry in git, by waiting for the build or by
reverting the commit that reset it; a write op, `refresh` or `render-claim-graph` attempted while a
build runs is refused (§8). Commits are scoped by pathspec to kbase-owned paths.

**The state store holds what does not belong in the repository** — the pid and the holder lock
`status` and `cancel` read, `progress.jsonl`, per-call evidence, the docgraph records (§5), a
per-unit answer cache keyed by input hash, and the captured output of a build the MCP server starts
(§11). Nothing in it is authoritative for position. The store lives at
`$XDG_STATE_HOME/kbase/<key>/`, where `<key>` is the first 16 hex digits of the SHA-256 of the
absolute, symlink-resolved `kb-root/` path, overridable with `--state-dir`; it holds paid-for
inference and survives `git clean`. **The run lock is in the repository's git directory**,
`kbase-build.lock`, recording the state-dir: one build runs on a repository at a time, whatever
`--state-dir` each names. A second build is `refused` with `check: lock`, `path` the running build's
state-dir, and no remedy; where the run lock is held but names no state-dir yet (the build is
starting), the second build gets `retry`. Every lock is advisory and exists on Unix only; on Windows
`status` cannot see a running build and no lock-based refusal or `retry` is given.

**The state store is versioned.** `<state-dir>/format` holds one integer, currently `1`, written
when a build first holds the store; a store with no `format` file is version 1. `build`, `status`,
`cancel` and the MCP `build` tool refuse a store newer than this kbase: `check: state-dir`, `key:
--state-dir`, `path` the store, remedy "update kbase, or use another --state-dir". A build converts
an older store in place before it uses it; the conversion is stepwise and resumes if interrupted.
The store's version is independent of the KB format version (§10).

**The state store is pruned only by a build.** At a build's start, the captured output of earlier
builds (`reports/build-*`) and the call captures (`scratch/captures/`) last modified before the
start of the fifth-latest build that ended are removed; a build that ended `refused` or `retry` does
not count. The answer cache and the stage reports are kept. `status` and `cancel` remove nothing.
Deleting the store is always safe, but it forfeits the answer cache, whose entries were paid for in
inference; `status` reports its size.

**Maintenance operations never require git.**

### `kbase status`

Returns a YAML document with: state (`none`, `running`, `cancelled`, `failed`, `bounded`,
`finished`); pid; timestamps; each stage recorded or not, with its commit; the current stage's units
done and total; recent refusals and fallbacks; the `--no-inference` flag; and the resume command.
Progress is persisted to a file so a late or reconnecting monitor catches up.

### `kbase cancel`

Signals the lock holder. The in-flight call is abandoned with nothing written for it; the event is
logged, the lock is released, and the build process exits `cancelled`. Cancel costs only the
in-flight unit; the build is resumable.

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
   them except `models` and `configure`.
3. The subcommand's own keys, in the order its table gives.
4. Exactly one of these, chosen by `outcome`: `refusals` for `refused` and `retry`; `failures` for
   `failed`; `cancelled` (a mapping of `stage` and `unit`) for `cancelled`.

Keys are kebab-case. The one exception is query data under `results`, which keeps `kb_cmd --json`'s
names verbatim. The same fact always uses the same key: `written` (files this call wrote;
kb-root-relative inside `kb-root/`, absolute outside it; `[]` when nothing was written), `ids` (node
ids, in entry order), `count` (a total before any limit), `resume` (a shell command that continues
the work), `state-dir` (the state store's absolute path).

**Item.** Every refusal, failure and verify finding is one mapping with these keys in this order, a
key omitted when it does not apply; `detail` is always present, with at least one of `check`, `path`
or `key`:

| Key | Holds |
|---|---|
| `check` | The named check or refusal class: a verify check name, `usage`, `dirty-paths`, `include`, `metadata-key`, `lock`, `state-dir`, `inputs`, `pandoc`, `unknown-id`, `kb-format`, `kb-root`, `demoted`, `dependency-cycle` |
| `path` | The file the item is about |
| `entry` | 1-based index into the values' `entry` list |
| `key` | The values key, flag or positional argument, spelled as `--help` spells it |
| `line`, `column` | 1-based position in the values document or in `path` |
| `allowed` | The closed set the value must come from |
| `remedy` | The kbase command that clears the item, where one exists (e.g. `kbase refresh`) |
| `detail` | One sentence |

A refusal lists every offending item. A usage error is one item with `check: usage`. A
provider-selection refusal names `key: --provider` and lists every entry name as `allowed`; a tier
`configure` cannot detect names `key: models.heavy` (or `models.light`) and lists the detected
candidates as `allowed`.

**Per-subcommand keys**, following `kb-root`:

| Subcommand | Keys |
|---|---|
| `build` | `state-dir`; `through`; `no-inference`; `resumed` (bool); `restored` (paths reset to the last boundary); `stages` (list of `stage`, `commit` — null if unrecorded — `dropped` (row ids), `report` (path to the stage's full findings in the state store)); `resume` (absent on `done`/`unchanged`) |
| `status` | `state-dir`; `state`; `pid`; `started`; `updated`; `ended`; `no-inference`; `stages` (list of `stage`, `commit`); `current` (`stage`, `units-done`, `units-total`); `recent-refusals`, `recent-fallbacks` (items, the newest 10 of each); `cache-entries`, `cache-bytes` (integers: the answer cache's entry count and total size in bytes, §6); `resume` |
| `cancel` | `state-dir`; `pid`; `resume` |
| `refresh` | `written` (includes every sheet written); `removed` (files the KB no longer calls for — a sheet the volume-count rule drops, a file a format migration made obsolete (§10) — kb-root-relative inside `kb-root/`, repository-relative beside it; `[]` when none) |
| `verify` | `findings` (items the KB's structure carries that are not faults — every `demoted` edge (§10), as items with `check: demoted`, `path` the register, `detail` the edge and its origin; `[]` when none); its faults are `refusals` items, a refresh-fixable item carrying `remedy: kbase refresh` |
| every write op | `ids` (inserts only; minted or adopted, in entry order); `minted` (the subset this call drew, as kb_tools' `minted`); `adopted` (list of `entry`, `id`, `differs` — values keys whose supplied value the existing entry does not carry); `written` (as kb_tools' `written`); `refreshed` (paths; null under `--no-refresh`); `removed` (the trailing refresh's, as `refresh`'s; `[]` under `--no-refresh`) |
| `resolve-demoted` | as every write op; `resolved` (list of `source`, `target`, `action`: `removed` or `restored`) |
| `render-citation` | `citations` (one per entry, in entry order) |
| `deps`, `gated-on`, `cited-by`, `find`, `referenced-by`, `solidity-below`, `subtree`, `weak-points` | `count`; `offset`; `truncated` (bool); `results` (the `kb_cmd --json` list, sliced; `deps`' edge records, not `-i`'s ids, add `context` after `applicability`: the edge's context string, null where it has none) |
| `show`, `stats` | `results` (the `kb_cmd --json` mapping; `show`'s claim record adds `strengthen_by` after `strengthen_by_count`: the claim's strengthen-by items in `item_idx` order, each `item_idx`, `text`, `mentioned_ids` (`[]` when none)) |
| `render-claim-graph` | `written` (the sheets whose bytes changed); `removed` (as `refresh`'s); `sheet` (`drawn` where the sheets were rendered this call, else `placeholder`: nothing was drawn, whether or not a sheet stands); `dot` (`"not found on PATH"`, present only when `sheet` is `placeholder`) |
| `models` | `provider`; `models` (ids, sorted ascending) |
| `configure` | `provider`; `models` (`heavy`, `light`, as `config.toml`'s `[models]` spells them); `written` (`config.toml`'s path, or `[]` with outcome `unchanged`) |

**Bounded results.** List queries take `--limit N` (default in ARCHITECTURE §12; `0` means all) and
`--offset N`; where `truncated` is true the caller re-issues the query with `--offset` set to
`offset + len(results)`. `stats`' ranked sections stay at kb_tools' 10. No other subcommand's result
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

Every subcommand below is a builtin tool personant can call. kb_tools' semantics govern each, cited
from the sections named; kbase's differences are only those in §4 and the rules in this section.

**Values input.** YAML (JSON accepted) on stdin or `--values`. The per-op key vocabulary is closed.
It is kb_tools' `kb_write` `values.OP_FIELDS`, and kbase keeps no prose copy of it. A refusal names
the key, its position, and the allowed set. A prose value containing U+0008 or U+000C is refused.

**Idempotence.** Every mutating op is idempotent under re-issue: the same values issued twice leave
`kb-root/` byte-identical to issuing them once, and the second call reports `unchanged`. An insert
adopts an existing entry of the same register and title (an insert's values name no host; the host
is assigned later through `set-frontmatter` and the marker); a work is adopted by its key.

**Writes leave nothing stale.** A mutating op ends with a refresh unless `--no-refresh` is given.

**One write lock.** A write op, `refresh` and `render-claim-graph` hold the KB write lock for the
whole run: from loading the KB through the trailing refresh and the sheets to the result. A writer
that cannot take it within 30 s gets `retry` with `check: lock`, `path` the repository root, and the
remedy. A write verb probes for a running build (§6) before it waits on the write lock and again
after taking it: where a build runs it is `refused` with the `lock` item at once, taking no wait. A
build waits out a write in progress, up to the same 30 s, before it writes (`retry` if it cannot);
its own writes take no lock. `render-citation`, `verify` and the queries take no lock. On Windows no
advisory lock exists (§6): neither the `retry` nor the build-running refusal is available there, and
one writer at a time is the operator's to keep.

**Batches** are all-or-nothing, as in kb_tools SPEC, "The Write API's Contract".

### Write ops

Each is named after kb_tools' `kb_util` op and follows "The Write API's Contract". Their result
keys are §7's.

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

The register-creating inserts take `--create` where kb_tools' op takes it.

### `render-citation`

Reads the cited document and writes nothing; returns the sanctioned authority-citation string
(kb_tools SPEC, "The Write API's Contract"; "Citation Grammar").

### Queries

Each is named after a kb_tools `kb_cmd` query and has its semantics (kb_tools ARCHITECTURE, "Query
Surface"; `kb_cmd/cli.py`). Over a kb_tools-built KB, each returns what `kb_cmd --json` returns, as
data.

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
change; kb_tools semantics: "The Claim-Graph Sheet". Without `dot`, §3's rule, and the result names
the lack. Result keys: §7.

### `refresh`

Derives the metadata layer's derived fields and `.index/*.yaml` and writes every claim-graph sheet
(§3) on each run, per kb_tools SPEC, "Derived Metadata, Defined". Without `dot`, §3's rule. Result
keys: §7.

### `verify`

The standard check a running KB owes (kb_tools SPEC, "Project Scoping"): the dead-link and
unknown-id gate over the documents under `kb-root/`, link targets resolved against the repository,
then the metadata gate. Faults are outcome `refused`; a `demoted` edge (§10) is a finding, listed
under `findings` with outcome `done`, and `findings` is present on both `done` and `refused`. A
`demoted` row whose origin is missing or not `cited` or `inferred` is a fault, refused under
`referential integrity`. The citation grammar (kb_tools SPEC, "Citation Grammar") is the build-time
check: the build's gate stage runs it after the standard check, and `verify` does not. Result keys:
§7.

### `status`, `cancel`

See §6.

---

## 9. Configuration

Two TOML files in the resolved configuration directory, read under the strict-load contract (§9.3).

**Global flags.** Every subcommand accepts `--config-dir DIR`, `--log-level debug|info|warn|error`
(default `warn`), `--log-file PATH` (tees diagnostics to a file; console output is never redirected
away) and `-h`/`--help`; the root command also accepts `-v`/`--version`, which prints exactly the
version string and a newline. The directory is `--config-dir`, else `$KBASE_CONFIG_DIR`, else
`~/.config/kbase`.

**Provider commands.** `kbase models [--provider NAME] [--timeout DURATION]` lists the selected
provider's model identifiers, sorted ascending, in its result document (§7). `kbase configure
[--provider NAME] [--model-map TIER=ID]... [--timeout DURATION]` assigns a model id to each tier — a
tier not given is detected from the provider's model list, and one it cannot detect is refused
naming `models.<tier>` with the candidates as `allowed` (§7) — and writes `config.toml` under §9.5.
Each writes its result under §7. Provider selection for both is `--provider`, else `config.toml`'s
`provider`, else the sole `providers.toml` entry; zero usable providers refuses naming
`providers.toml`, and two or more with none selected refuses listing every entry name.

**The environment.** A subcommand that spends inference also reads `KBASE_API_BASE_URL`,
`KBASE_MODEL` and `KBASE_API_KEY_FILE` — kb_tools' variables under kbase's prefix, with kb_tools'
meanings — so an integrator configures a build from the invoking environment with no configuration
directory at all. Precedence is flags, then the environment, then the configuration files; a
variable given overrides the corresponding field and nothing else, and `KBASE_MODEL` names both
tiers. The inference preflight (§5) names the variable or file it lacks. There is no
plaintext-transport switch: when a key is configured and the base URL is `http://` to a host other
than loopback, one line on stderr says the key travels in cleartext, and the build proceeds (as
§9.4's permission warning does).

### 9.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses:

```toml
[<name>]
baseUrl = "..."        # required
apiKeyFile = "..."     # path to a key file, resolved relative to this file's
                        # own directory if relative; preferred over apiKeyUnsafe
apiKeyUnsafe = "..."   # inline key; discouraged
type = "inference"     # optional; the only accepted value, also the default
api = "openai"         # optional; the only accepted value, also the default
```

No other key is recognized inside an entry. An entry that fails validation (empty `baseUrl`, or
`type`/`api` set to anything but its one accepted value) is **dropped** with a warning naming the
entry and the reason (never key material); the rest of the pool still loads. A key-file read failure
faults just that entry the same way. An absent `providers.toml` is an empty pool, not a load error.

### 9.2 `config.toml` — the choices

```toml
provider = "..."             # active providers.toml entry name

[models]
heavy = "..."                 # model id for the heavy tier
light = "..."                  # model id for the light tier

[asks]
readerConcurrency = 4          # optional; asks of one group in flight once its first has returned
```

A tier left unset resolves to "not configured" — no defaulting, no guessing. `readerConcurrency` is
how many of one group's letter asks a build has in flight once the group's first ask has returned
(kb_tools reads it from `KB_READER_CONCURRENCY`); absent, it is 4. A value under 1 fails the load,
naming `asks.readerConcurrency`. There is no `[dev]` table; an old `[dev]` key fails strict load
(§9.3).

### 9.3 Strict-load contract

Both files are decoded strictly: any key or table not modeled by the schemas above fails the **whole
file's** load. The message names the file path and every offending key by its full dotted path (e.g.
`models.typo_key`), pluralizing "unknown key"/"unknown keys" when more than one. A malformed-TOML
file fails with a parser-detail message naming the file and line. Only key **names** ever appear in
either message, never values, since `providers.toml` may carry credential material. `kbase
configure` cannot repair a config file it cannot load: every subcommand loads configuration before
doing anything else.

### 9.4 Key-file permission warning

When a `providers.toml` entry sets `apiKeyFile` and the host is not Windows, kbase stats the
resolved key-file path. If its permission bits grant group-read or other-read, it prints a warning
naming the path and the offending mode and recommending `chmod 600`. The entry stays usable. This is
the only file-permission behaviour kbase applies to a file it did not create.

### 9.5 `configure`'s targeted update (byte-preservation contract)

`kbase configure` never re-serializes `config.toml`; it edits the existing bytes in place.

- An absent or blank file gets a full commented template, with `provider`, `models.heavy` and
  `models.light` filled in.
- Otherwise **only** the top-level `provider` key and the `[models]` table's `heavy`/`light` keys
  are rewritten. Every other byte — comments, unrelated keys and tables, spacing, key order — is
  preserved exactly. A rewritten line's own trailing `# comment` is kept; its column position is not
  guaranteed.
- A missing `provider` key is inserted just after the leading comment block; missing `heavy`/`light`
  keys are appended inside (or as a new) `[models]` table.
- Before writing, kbase decodes both the original and the proposed file as TOML and refuses the
  write, leaving the file untouched, unless the only difference between the two decoded structures
  is exactly the values it intended to change.
- The write is atomic (temp file in the same directory, then rename). A `config.toml` that is a
  symlink has its target rewritten; the symlink stays.
- Any failure along this path leaves the file completely unwritten.

---

## 10. Metadata format version

**The stamp.** A KB declares its metadata format as `kb-format: <major>.<minor>.<patch>` in the
entry point's frontmatter, semver. The formats kb_tools wrote before it adopted `1.0.0` — the
comment-block frontmatter, the JSON build records, the JSONL index — are `0.9.0`, and a KB with no
stamp is `0.9.0`. Major: a reader built for the old major misreads the KB (a renamed field, changed
semantics, a changed anchor rule). Minor: additive; an old reader ignores the addition safely.
Patch: the bytes a writer produces changed, their meaning did not.

**Reading the stamp.** Before any other read, kbase reads `kb-format` alone from
`kb-root/entry-point.md`, recognising either frontmatter form; a KB with no entry point is refused
(`check: kb-root`) as it is today. Every producer of a KB stamps it: the build's spine, and every
save (below). A stamp at the current major and minor with another patch is read as current and
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
turning it into `references`. Both the classify stage's cuts and, under `--no-inference`, the
containment-ring stage's cuts are demoted. The origin is `inferred` where the cut pair, in its
written direction, is a yes in the unmarked record, else `cited`. It stands outside the premise walk
and the acyclicity check exactly as `references` does and carries no solidity. In the register an
entry carries a `- demoted:` list after `references:`, its items `- <target> — <title> (origin
cited)` or `(origin inferred)`, with an optional context, in the annotation style of the other
lists. In the index the row has `relation: "demoted"`, every field a `references` row carries, then
`origin` (`cited`, `inferred`, or null where the bullet has none). The build writes cuts through a
build-only write op, `add-build-edges`, which is no subcommand and not among the public ops, so the
public write surface cannot create one; `verify` refuses a `demoted` row carrying no valid origin,
so a KB's demoted edges are the build's and a person's resolutions only remove them. One write op,
`resolve-demoted` (§8), removes one or restores it to `depends`, refusing a restore that would close
a cycle (`check: dependency-cycle`, the entry number and the path) and a restore where neither a
`demoted` nor a `depends` edge exists. `verify` lists every `demoted` edge under `findings` (§7)
with outcome `done`; the queries report it under its relation wherever they report `references`
(`show`, `deps`, `referenced-by`); the sheet draws it as a cut, its origin in the cut's weight and
tooltip (§3); `stats` counts them as `demoted_edges`, after `depends_on_edges`, which still counts
every row.

---

## 11. The MCP server

`kbase mcp --kb-root <path>` serves one KB to any agent harness that speaks the Model Context
Protocol, over stdio JSON-RPC. It is the subcommands, bound: every tool is one subcommand, with the
same arguments and the same result, so a harness without MCP loses nothing but convenience.

- **Tools.** One per subcommand, named as the subcommand, less `completion`, `help`, `models`,
  `configure` and `mcp` itself. A tool's input schema is derived from its command's arguments and
  flags (the global flags excluded; the server carries its own). A write op's schema is one entry:
  the op's closed vocabulary (§8) as properties, no others allowed, plus `create` where the CLI
  offers `--create`; a call writes that one entry, and `--no-refresh` is not offered. A refusal over
  MCP carries `entry` as `1` and no `line` or `column`; its `key` keeps the CLI spelling (§7).
  Arguments that fail the tool's schema (a missing required key, a wrong type, an unknown key) are
  JSON-RPC `-32602` errors, not refusal documents. A command declares itself read-only in its
  definition, and the tool carries that as its read-only annotation; the queries, `verify`,
  `status`, `render-citation` and `show` are the read-only set.
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
  - *entered a stage* — outcome `done` with `kb-root`, `state-dir`, `pid` and `resume`; `status` and
    `cancel` follow it;
  - *exited* — the build's own document, `isError` set by its outcome (§7);
  - *still starting at the bound* — outcome `failed`, with an item naming the state directory and
    saying the build is still starting; the build keeps running, and `status` reports it once it has
    begun.

  A relative `state-dir` argument resolves against the repository root; the server's own
  `--config-dir` and `--log-file` stay relative to the server's working directory. Where `status`
  would refuse (no git worktree, or a `--state-dir` inside `kb-root/`), the tool returns that
  refusal and starts no process. The capture files are not removed.
- **Resources.** The claim-graph sheets (§3) present on disk, the placeholder included, are listed
  by `resources/list` as `image/svg+xml` resources with URI `kbase://sheet/<kb-root-relative path>`;
  `resources/read` returns the SVG as text. An unknown URI is JSON-RPC `-32002` (resource not
  found); a missing `uri` is `-32602`. `initialize` advertises the `resources` capability. Nothing
  else is served: the KB's documents are the harness's own files.
- **What the server never does.** It sends no sampling request and offers no prompts; no model is
  reachable from any tool but `build`, and `build`'s inference is the build's own. It writes nothing
  but JSON-RPC frames to stdout; diagnostics go to stderr and `--log-file`. It reads no harness
  configuration and names no harness.
- **Protocol.** One MCP revision, the one the server implements, pinned as one constant
  (ARCHITECTURE §12); an initialise naming another revision is answered with the server's for the
  client to decide; calls are answered one at a time, in order; EOF on stdin ends the server.
- **Registration** is the harness's: one server entry naming the command `kbase mcp --kb-root
  <absolute path>`. README gives the entry for two harnesses as examples.
