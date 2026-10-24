# KB format 3.0.0 — Phase C work packages

The dispatch plan for `ACTIVE_PLAN.md`'s Phase C, one package per dispatch. The plan's frame — C1,
C2, then C3 in the worktree in the order a, b, d, c, e, its gates and its release gate — governs.
The design is `KB_3_DATA_MODEL.md`; a package cites its sections. Each dispatch prompt quotes this
preamble, the package's section, the table rows naming the package and the design sections it cites,
and names THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md for the coder to read. The coder
reads neither this file nor the design whole: it follows a cross-reference only where the quoted
text does not settle the point, reads only that section, and names it in its report. Where design
and plan differ the plan governs, and a coder that finds the design unworkable stops and reports
rather than redesigning. Packages run in dependency order; packages whose predecessors are
all committed and that share no file may run in parallel, each audited and committed on its own (the
package table's "Parallel with" column).

**Seats.** Expert: go-coder at the coordinator's model. Mechanical: go-coder at sonnet.

**Trees.** C1 and C2 run in the shared tree, `/Users/agent-user/projects/kbase`. C3 runs in the
worktree `/Users/agent-user/projects/kbase/.claude-temp/kb3` (branch `kb-3.0.0`); every C3 command
runs as `cd /Users/agent-user/projects/kbase/.claude-temp/kb3 && just …`. In C3 every path a package
names resolves under `/Users/agent-user/projects/kbase/.claude-temp/kb3/`. Read and edit only there; a
change elsewhere under `/Users/agent-user/projects/kbase/` is outside every C3 scope. Scratch goes in
`/Users/agent-user/projects/kbase/.claude-temp/kb3/.claude-temp/`.

**Facts.** Recorded at `e95e735`. The symbol is the fact; a line number moves with earlier packages.

**Done targets** are only `just fmt-check`, `just test-pkg <pkg>`, `just edit-gate`, `just test`,
`just checkpoint`, `just test-integration` and, once C1.4 lands, `just bench-load`. `test-pkg` runs a
whole package; no Done names a subset of one. The plan's gates: C1, C2 and C3e end on
`just checkpoint` and `just test-integration`; between C3 dispatches, `just fmt-check` and
`just test-pkg <pkg>` for each package the dispatch names green. In C3 every Done also runs
`VERBOSE=1 just test-pkg <pkg>` for each package the "Allowed red" table lists as red at that point;
the report lists every `--- FAIL` and build error, each with the package that owns it. A red test is
reported against its owner, never adapted by a package that does not own it. Green means green but
for the rows owned by a later package; the VERBOSE run shows only those failing. While its package
is red, a package's own Tests pass by name in the VERBOSE run. Every C3 report also carries the output of
`git grep -n 'kb\.\(ReadIndex\|IndexRowType\|IndexFiles\)\b' -- internal cmd ':!*_test.go'`: through
C3a.6 only `internal/index`'s `kb.IndexFiles` lines, from C3a.7 nothing. These symbols stay in `kb`
until C3d.5c, after `.index/` stops holding the files they name, so any production caller reads
nothing.

---

## All packages

The one home of each package's rating, predecessors, tree and parallelism. "Parallel with" names the
packages it may run beside once both sets of predecessors are committed: they share no file,
neither package's tests import the other's package, and neither's Done runs a target that builds
or tests the other's package (`just edit-gate`, `just checkpoint`, `just test`, or a `test-pkg`
over a package importing it) while the other is uncommitted.

| ID | Title | Rating | Predecessors | Tree | Parallel with |
|---|---|---|---|---|---|
| C1.1 | `index.Derive`: the index rows in memory | expert | — | shared | — |
| C1.2 | `query.Load` from `Derive` | expert | C1.1 | shared | — |
| C1.3 | the sheet and the overview take the rows | expert | C1.1 | shared | — |
| C1.4 | `just bench-load` and `BenchmarkLoadDerive` | expert | C1.2, C1.3 | shared | — |
| C2.0a | 0.9.0 removed | expert | — | shared | — |
| C2.0b | the migrate tree | expert | C2.0a | shared | — |
| C2.1 | text primitives, shared types and the 2.0.x register reader | expert | C2.0b | shared | — |
| C2.2 | the 2.0.x leaf, node and axiom readers | expert | C2.1 | shared | — |
| C2.3 | parse goldens and the agreement test | mechanical | C2.2 | shared | — |
| C2.4 | the 2.0.1 parity query goldens | expert | C2.3, C1.4 | shared | — (shares `justfile` with C1.4) |
| C3a.1 | record types, encode/decode, the strict load, `3.0.0` | expert | C1, C2, worktree | kb3 | — |
| C3a.2 | markers, document kind, hosts | expert | C3a.1 | kb3 | — |
| C3a.3 | `index.Derive` over records and markers | expert | C3a.2 | kb3 | — |
| C3a.4 | the degraded load and cycle breaking | expert | C3a.3 | kb3 | — |
| C3a.5 | register pages, freshness, dead-link fragments | expert | C3a.4 | kb3 | — |
| C3a.6 | `index`'s refresh, non-migration path | expert | C3a.5 | kb3 | — |
| C3a.7 | `index`'s verify | expert | C3a.6 | kb3 | — |
| C3b.1 | the record store, provenance, field ops | expert | C3a.7 | kb3 | — |
| C3b.2 | the inserts | expert | C3b.1 | kb3 | — |
| C3b.3 | edge ops and markers | expert | C3b.2 | kb3 | — |
| C3b.4 | op help, flags and MCP schema | expert | C3b.3 | kb3 | — |
| C3d.1 | the queries | expert | C3b.4 | kb3 | — |
| C3d.2 | `score`'s skip | expert | C3d.1 | kb3 | C3d.5a |
| C3d.3 | the sheet | expert | C3b.4 | kb3 | C3d.5b |
| C3d.4 | `claimgraph` and `buildrecords` | expert | C3b.4 | kb3 | C3d.5b |
| C3d.5a | `build` and `export` | expert | C3d.4 | kb3 | C3d.2, C3d.5b |
| C3d.5b | the `cmd`-side fixtures | mechanical | C3b.4 | kb3 | C3d.3, C3d.4, C3d.5a |
| C3d.5c | the `cmd` verbs | expert | C3d.1, C3d.2, C3d.3, C3d.5a, C3d.5b | kb3 | — |
| C3d.6 | the model-facing texts | mechanical | C3d.5c | kb3 | — |
| C3c.1 | the 2→3 mapping, emitter and chain step | expert | C3d.6 | kb3 | — |
| C3c.2 | markers from `claims:` listings | expert | C3c.1 | kb3 | — |
| C3c.3 | the stamped documents' restamp | expert | C3c.2 | kb3 | — |
| C3c.4 | the migration save and the golden pairs | expert | C3c.3 | kb3 | — |
| C3e.1 | the contract documents | expert | C3c.4 | kb3 | — |
| C3e.2 | the remaining fixtures | expert | C3e.1 | kb3 | — |
| C3e.3 | the harnesses and the closing gates | expert | C3e.2 | kb3 | — |

"Parallel with" also excludes, from C3a.1 until C3d.5c, any two packages that edit `cmd/` or a
fixture `cmd`'s tests read: `cmd` does not build until C3d.5c, so no Done run would catch a conflict
between them.

## Allowed red in C3

From C3a.1 `kb.FormatVersion` is `3.0.0`, so every 2.0.x fixture a test opens through `kbload`
needs a 2→3 converter (`migrate.Chain` refuses "no converter leads from 2.0.0") until C3c; and
`index`'s types move under its importers. A package not yet green is red until its owner below.

| Package | Red from | Green at (owner) |
|---|---|---|
| `internal/kb` | — | C3a.1 |
| `internal/index` | C3a.1 | C3a.7 |
| `internal/sheet` (its tests open a 2.0.1 fixture through `kbload.Open`, `sheet_test.go:27, 376`) | C3a.1 | C3d.3 |
| `internal/write` | C3a.1 | C3b.3 |
| `internal/query` | C3a.1 | C3d.1 |
| `internal/score` | C3a.1 | C3d.2 |
| `internal/claimgraph`, `internal/buildrecords` | C3a.1 | C3d.4 |
| `internal/build`, `internal/export` | C3a.1 | C3d.5a |
| `cmd` | C3a.1 | C3d.5c |
| `internal/kbdocs` | C3a.1 | C3d.6 |
| `internal/migrate`, `internal/kbload` | C3a.1 | C3c.4 |
| tests opening a pre-2.0.1 KB through `kbload`: `internal/index/migration_test.go` (`TestRefreshFinishesAStoppedSave`), `cmd/format_test.go` (`kb100Dir`), `cmd/score_test.go`'s `score-kb-1.0.0` cases (:860, :1121) | C3a.1 | C3c.4 |

## The import graph

Recorded with
`for p in kb kbload migrate index query sheet score write export kbdocs claimgraph build buildrecords; do echo "$p: $(go list -f '{{join .Imports " "}}' ./internal/$p | tr ' ' '\n' | grep '^kbase/' | sed 's#kbase/internal/##' | tr '\n' ' ')"; done`:

```
kb: log
kbload: buildrecords kb migrate result
migrate: (none)
index: atomicfile kb log result sheet
query: index kb result
sheet: asks buildrecords kb
score: asks atomicfile kb log query result
write: atomicfile filelock index kb kbload log result
export: atomicfile kb
kbdocs: atomicfile kb result
claimgraph: asks atomicfile buildrecords index kb log records result write
build: asks asks/call atomicfile claimgraph docgraph filelock index kb kbdocs kbload latex/pandoc ledger log model records result statedir write
buildrecords: kb
```

`cmd` imports every one a verb reaches, so `cmd` builds only once `query`, `score`, `claimgraph` and
`build` do (C3d.5c). `index` imports `sheet`, never the reverse.

## Retirements of 2.0.x `kb` readers

A 2.0.x reader in `kb` retires with its last caller — production, test, or another `kb` reader.
Production callers: `for s in <symbol>…; do git grep -l "kb\.$s\b" -- internal cmd ':!*_test.go' ':!internal/kb'; done`.
Test callers: `git grep -n 'kb\.<sym>\b' -- '*_test.go' ':!internal/kb'`. Inside `kb`, `Discover`
calls `AuthoredIDs`, `ParseLeaf`, `LocateRegisterEntries`, `ParseAnalysisEntries` and
`ParseClaimEntries` (`internal/kb/discover.go:146, 186, 294/484, 492, 489`), so those retire with it.

| Symbol | Production callers (porting package) | Test callers (owner ports) | Retires in |
|---|---|---|---|
| `WorkCitations`, `WalkRegisters`, `LocateLeafReferenceFooters`, `LegacyAxiomDeclarations` | `index` (C3a.7) | — | C3a.7 |
| `ReplaceOrInsertFrontmatterField`, `LocateEntries`, `ParseExperimentLeaf`, `ParseAnalysisLeaf`, `FrontmatterKeySpan` | `index` (C3a.6), `write` (C3b.3) | — | C3b.3 |
| `ReadIndex`, `IndexRowType` | `query` (C1.2), `sheet` (C1.3), `kb.SheetDigestDrawnOf` (C1.3) | `internal/index/rows_test.go:42, 46`, `refresh_test.go:97` (C3a.3); `internal/write/bearing_test.go:523, 562` (C3b.3); `internal/score/text_test.go:140` (C3d.2); `cmd/demoted_test.go:24`, `cmd/resultdocs_test.go:259` (C3d.5c) | C3d.5c |
| `IndexFiles` | `index` (C3a.7) | `internal/index/rows_test.go:71` (C3a.7); `internal/score/run_test.go:132` (C3d.2); `cmd/format_test.go:164`, `cmd/init_test.go:185` (C3d.5c) | C3d.5c |
| `Discover`, the 2.0.x `kb.State`, `AuthoredIDs`, `ParseLeaf`, `LocateRegisterEntries`, `ParseAnalysisEntries`, `ParseClaimEntries`, `Registers`, `ParseWorkEntries`, `KnownClaimIDs`, `ParseAxioms`, `CanonicalClaimIDs` | `index` (C3a.6, C3a.7), `write` (C3b.3), `claimgraph` (C3d.4), `export` (C3d.5a) | `internal/index/index_test.go:334, 515`, `migration_test.go:60` (C3a.7); `internal/write/bearing_test.go:227, 490`, `experiment_test.go:28`, `opscript_test.go:67`, `regression_test.go:63, 66, 156` (C3b.3); `internal/claimgraph/depends_stage_test.go:100` (C3d.4); `internal/build/inference_test.go:404` (C3d.5a); C2.3's agreement test (C3d.5a removes it); the `discover` test helper, `from_1_0_0_test.go:275` at `e95e735` and in `internal/migrate/from_1_0_0_to_2_0_0` after C2.0b (C3c.1) | C3d.5a |

**Rule.** A package going green leaves none of its tests calling a symbol this table retires later;
the retiring package's report runs the test-caller grep and finds only packages still red.
`internal/kb`'s own tests of a retiring symbol (`discover_test.go`, `indexstream_test.go`,
`metadata_test.go`) go with the symbol, in the retiring package.

Kept: `BearingFromValue`, `Bearing`, `Edge`, `NodeIDs`, `ClaimIDs`, `ContainsWord`, `MarkerLines`,
`AxiomSource`, `UnmigratedAgentsFile` (`kbdocs.Stamp`, `internal/index/refresh.go`'s precheck),
`ParseFrontmatter` and `FormatStamp` (the entry point's stamp, `verify`'s frontmatter fault).

---

## C1 — in-memory derivation (shared tree)

Behaviour-neutral under 2.0.1, except as the plan says: a query over a KB whose sources `refresh`
refuses (a cycle, a coverage fault) now carries that refusal. `refresh` keeps writing `.index/`; C1
retires nothing §3.6 lists. C1's Done (the plan's): gates green; `git diff --stat` touches no
`testdata/` or `test-data/fixtures/` file; `git grep -n 'ReadIndex(' -- internal cmd ':!*_test.go'`
returns no `query` line; the report gives `just bench-load`'s ns/op and allocs at both sizes.

### C1.1 — `index.Derive`: the index rows in memory

- **Design:** §3.6 (the `.index/` rows only).
- **Scope:** `internal/index/records.go`: `Derive(src)` is `kb.Discover` then `deriveRows(st)`,
  returning the five typed row sets; `BuildRecords` keeps its signature as `fieldsOf` over
  `deriveRows(st)`. `deriveRows` keeps `checkCoverage` and `CycleError` as `BuildRecords` raises them;
  `Derive` maps a `kb.Discover` error as `refresh` does. Not touched: `refresh.go`, `verify.go`,
  callers.
- **Facts:**
  - `git grep -n 'func BuildRecords' -- internal` → `internal/index/records.go:291`.
  - `git grep -n 'kb\.Discover(' -- internal/index ':!*_test.go'` → `refresh.go:166, 219, 336, 389`;
    `verify.go:101, 275`; `refresh.go:37` is the unexported `refusal(src, err)`.
  - `sed -n 136,139p internal/index/records.go` → edge rows sorted by source, target, context.
  - `sed -n 63,64p internal/index/rows_test.go` → `BuildRecords(roundTripState())`: a caller passing
    a `kb.State`, hence the kept signature.
- **Tests:** `Serialize` of each `BuildRecords` file over `internal/sheet/testdata/adjagent-sheet`
  equals its committed `kb-root/.index/*.yaml` bytes.
- **Done:** `just fmt-check`; `just test-pkg ./internal/index` green.

### C1.2 — `query.Load` from `Derive`

- **Design:** §9.1's first sentence.
- **Scope:** `internal/query/load.go`: `Load(src)` is `index.Derive` then an unexported in-package
  constructor `fromRows(src, rows)` that applies the node-kind check, `checkEdges`, the analysis
  `BuildBand` fill and the sort; `loadRows`, `requiredFiles` and the missing-file refusal go; the
  `src.Migrated()` refusal stays. `score`'s set reaches the rows through `query.Load`
  (`cmd/score.go:213`).
- **Facts:**
  - `git grep -n 'ReadIndex(' -- internal cmd ':!*_test.go'` → `internal/query/load.go:142` only.
  - `git grep -n 'query.Load' -- internal cmd ':!*_test.go'` → `cmd/query.go:69`, `cmd/score.go:213`.
  - `git grep -c 'writeIndex(' -- internal/query` → `query_test.go:6`, `scoring_test.go:2`;
    `git grep -c 'load(t,' -- internal/query` → `query_test.go:17`, `scoring_test.go:2`.
  - `sed -n 646,693p internal/query/query_test.go` → `TestLoadRefusals` (an empty-`kb-root` case,
    a bad-lines case over malformed index lines, subtests "unknown relation" and "bearing as a
    number") and `TestLoadRefusesAnOrphanScoringRow`.
- **Tests:** the `writeIndex` and `load(t,…)` seeds build rows and call `fromRows`, so they keep
  their rows and assertions. In `TestLoadRefusals`, the empty-`kb-root` (missing files) and bad-lines
  cases go — `Load` reads no index file; "unknown relation", "bearing as a number" and
  `TestLoadRefusesAnOrphanScoringRow` keep their assertions over `fromRows`. One new test: `Load`
  answers with no `.index/` files on disk.
- **Risk (intended, plan C1):** `Load` raises `Derive`'s coverage and cycle refusals.
- **Done:** `just fmt-check`; `just test-pkg ./internal/query`, `./internal/score`,
  `./internal/write`, `./cmd` green; the plan's `ReadIndex(` grep returns no `query` line.

### C1.3 — the sheet and the overview take the rows

- **Design:** §9.7's first clause; §2.3's `kb.SheetDigestDrawnOf` row.
- **Scope:** `internal/sheet/graph.go`: `readIndex` goes; `Render` takes node, edge and cite rows.
  `internal/index/refresh.go` `WriteSheet` calls `Derive` and passes them. `internal/build/rows.go`
  calls `kb.SheetDigestDrawn` over `Derive`'s node rows; `kb.SheetDigestDrawnOf` goes. Sheet tests
  keep reading the committed `.index/` (until C3a.3). Not touched: the sheet's build-record reads,
  `sheet.Placeholder`.
- **Facts:**
  - `git grep -n 'ReadIndex\[' -- internal cmd ':!*_test.go'` → `internal/kb/indexstream.go:194`
    (definition), `internal/kb/schema.go:222` (`SheetDigestDrawnOf`), `internal/sheet/graph.go:80`.
  - `git grep -n 'sheet.Render(' -- internal cmd` → `internal/index/refresh.go:513` only.
  - `git grep -n 'SheetDigestDrawnOf(' -- internal cmd ':!*_test.go'` → `internal/build/rows.go:401`,
    definition `internal/kb/schema.go:221`.
  - `git grep -n 'index.WriteSheet' -- cmd ':!*_test.go'` → `cmd/render-claim-graph.go:55`.
- **Tests:** the DOT goldens under `internal/sheet/testdata/` match unchanged.
- **Risk (intended, as C1's query rule):** `render-claim-graph` over a KB whose sources `refresh`
  refuses now carries that refusal (it reaches `Derive` through `index.WriteSheet`).
- **Done:** `just fmt-check`; `just test-pkg ./internal/sheet`, `./internal/index`,
  `./internal/build`, `./internal/kb`, `./cmd` green.

### C1.4 — `just bench-load` and `BenchmarkLoadDerive`

- **Design:** §12.
- **Scope:** `justfile` (`bench-load` with a `[doc]`, log under `test-data/transient/bench-load/`);
  `BenchmarkLoadDerive` in an `internal/index` test file: `adjagent-sheet` as committed and
  replicated ×40 in memory, ids renamed within the id grammar, through `kb.MigratedSource`; it times
  `Derive` over the source.
- **Facts:**
  - `grep -n 'bench' justfile` → nothing; `git grep -n 'func Benchmark' -- '*_test.go'` → nothing.
  - `sed -n 87,93p internal/kb/schema.go` → `<kind>-[a-z0-9]{6}`;
    `git grep -n 'AxiomLabel *=\|WorkIDPattern *=' internal/kb` → `schema.go:171`, `schema.go:154`.
  - `sed -n 54,61p internal/kb/source.go` → `MigratedSource` serves `files` and marks the source
    migrated.
  - 14 node rows and 18 edge rows: `wc -l internal/sheet/testdata/adjagent-sheet/kb-root/.index/claims.yaml internal/sheet/testdata/adjagent-sheet/kb-root/.index/edges.yaml`.
- **Done:** `just fmt-check`; `just bench-load` prints ns/op and allocs at ×1 and ×40; C1's gates
  `just checkpoint`, `just test-integration` green.

---

## C2 — the migrate tree and the 2.0.x readers (shared tree)

C2's Done (the plan's): 0.9.0 gone and `internal/migrate` a tree of packages (C2.0a, C2.0b); a
committed golden of the copied readers' parse per `internal/migrate/testdata/*-2.0.0` fixture (a
refusal golden where the 2.0.x load refuses it); the 2.0.1 parity goldens; `just checkpoint`,
`just test-integration`.

**The migrate tree.** Each package of the tree is its own namespace, so a name in one never
collides with a name in another. A declaration another package of the tree uses is exported by
capitalising its first letter; a name copied from `kb` keeps its `kb` spelling.

- `internal/migrate` — `Chain`, the `converters` list and the version arithmetic. It is the only
  package of the tree imported from outside it, and it keeps every exported name it has after
  C2.0a, so no importer outside the tree changes: a type moved elsewhere is kept as an alias of its
  new home (`type Files = common.Files`, `type ConversionError =
  from_1_0_0_to_2_0_0.ConversionError`).
- `internal/migrate/common` — a declaration with no format content that two or more packages of the
  tree use: `Files`, and helpers such as line splitting. This is the one test for what belongs
  here. Nothing in it changes
  behaviour once a package uses it: a helper that would have to change for one format was that
  format's all along, and the new behaviour is declared in that format's package under its own
  name, the `common` one left as it is.
- `internal/migrate/format_<major>_<minor>_<patch>` (`format_1_0_0`, `format_2_0_0`) — one
  superseded format's frozen definition: its stamp, field names, token patterns, vocabulary
  (`PendingLiteral`), record shapes, and its reader where one exists. Each is defined once, there,
  and the converters on either side import it. A format that carries a predecessor's declaration
  unchanged imports it from the predecessor's package rather than restating it — except the
  frozen copy of `kb`'s reader (C2.1, C2.2), which carries its own. A format's reader is a function
  another package calls to read that format; a parse only a conversion performs is the converter's.
  `format_2_0_0` covers every 2.0 patch.
- `internal/migrate/from_<from>_to_<to>` (`from_1_0_0_to_2_0_0`; C3c adds `from_2_0_0_to_3_0_0`) —
  one converter's transformation, importing `common` and the format packages of its two sides.
- Imports run one way: `common` imports nothing of the tree; a format package imports `common` and
  earlier format packages; a converter imports `common` and format packages; the root imports the
  converters, `common`, and the format packages for their stamps (`converters` names each step's
  versions). No package imports a converter but the root, and none imports a later format.
- The tree imports nothing of kbase outside `internal/migrate` (I8), and only `kbload` imports into
  it from outside (tests aside).
- `internal/migrate/testdata` stays where it is, read by relative path. A test moves with the
  declarations it tests.

**Copy rules (C2.1, C2.2).** The copy lands in `internal/migrate/format_2_0_0`, and every identifier
keeps its `kb` name, exported or not. A copied body takes exactly three kinds of edit, and no
other:

1. Each `*kb.Source` parameter becomes `common.Files`.
2. Each `log.Logger` becomes a drop sink, `func(path string, line int, detail string)`, called where
   the original logged: `path` the reader's `path` field (`""` where it has none), `line` its `at`
   field (`0` where it has none), `detail` the message followed by each key and value in order,
   space-separated.
3. A call that reads through `kb.Source` or touches the disk (`walk`, `IsFile`, `Root`, `KBPath`,
   `ReadText`, `os.Stat`, `isSymlink`) becomes the narrowest stand-in over the `common.Files` in
   reach that yields the same paths in the same order and the same text — `ReadText`'s UTF-8 check
   and newline translation included. A declaration that touches the disk with no `common.Files` in
   reach is not copied, and the report names it. The report lists every stand-in; C2.3's agreement
   test checks them.

A file the scope names is copied whole: every declaration in it, reachable or not, its helpers
included, except one edit 3 excludes. Every `kb` identifier the copy references is copied too,
transitively, until the package builds. The copy is frozen and carries everything it runs on: it
shares no helper with `common` or an earlier format package, even one with an identical body, and
takes from `common` only `Files`. Where `format_2_0_0` already holds a declaration whose value or
body is identical, token for token, to one the copy brings in, under the same name or another
(`Register200Name` beside `kb.RegisterFile`), the
copy's survives, the other is deleted, and its users in `format_2_0_0` and `from_1_0_0_to_2_0_0`
switch to the copy's; a pair that differs in any token (a pattern spelled with `\s` beside one
built through `PyRE`) is not identical, and both stay. The copy imports only the standard library,
the modules CONVENTIONS.md "Dependency Policy" sanctions, and `common`. Nothing calls the copy yet.

Shared facts: the tree's tests may import `kb` (`internal/migrate/from_1_0_0_test.go:14` at
`e95e735`, in `from_1_0_0_to_2_0_0` after C2.0b).

### C2.0a — 0.9.0 removed

0.9.0 does not exist: no KB in that form is read, converted, tested or described. This is a
removal, not a migration: nothing replaces what goes, and no text says 0.9.0 is gone.

- **Design:** none.
- **Scope:**
  - `internal/migrate`: `from_0_9_0.go`, its `converters` entry, `testdata/0.9.0`,
    `SupersededStamp`, `SupersededRecordPaths`. Every declaration of `from_0_9_0.go` that a surviving
    file uses moves unchanged to the file that uses it (`pendingLiteral`, `strip`, `isSpace`,
    `indexDir` to `from_1_0_0.go`; `version100` to `migrate.go`; `entryPointPath` to
    `from_1_0_0_test.go`); the rest goes. `scalars_test.go` and `export_test.go` pin the 0.9.0
    converter's writers (`yamlFrontmatter`, `stringNode`, `numberTag`, `encodeYAML`) and go with
    them. `migrate_test.go`'s 0.9.0 cases go; `TestChainRefusesAGap` keeps its subject over versions
    that still exist.
  - `internal/kb/format.go`: `UnstampedFormatVersion` and the no-stamp branch of `FormatStamp`. An
    entry point with no `kb-format` stamp then takes the path an unreadable stamp takes, refused
    `check: kb-format`; `FormatStamp`'s signature changes only as far as that needs.
  - `internal/kbload`: the unstamped and comment-block branches (`kbload.go:110–111`), the
    `SupersededRecordPaths` term in `RecordFiles()` (`:36`, which also shrinks `build.ownedPaths`,
    `internal/build/build.go:60`), the `.jsonl` term of `inIndex` (`:154`), `testdata/0.9.0`,
    `testdata/loose-0.9.0`, and `parity_test.go` (every case it holds is 0.9.0).
  - The flag-gated reference harnesses, which nothing has run since `bedc61b` removed the
    comparison toolchain: `internal/claimgraph/compat_test.go`, `internal/query/compat_test.go`'s
    `TestStageQueryScores`, `cmd/resultdocs_test.go`'s `-query.fixtures` cases.
    `internal/query/compat_test.go`'s `TestStageQueryCases` and `comparedQueries` stay (C2.4 uses
    them).
  - Every other test naming 0.9.0, `UnstampedFormatVersion` or the comment block: one asserting
    only 0.9.0 behaviour is deleted (`index_test.go`'s `TestSerializeAsTheMigrationWrites`,
    `claimgraph/buildrecords_test.go`'s `TestEncodeRecordAsTheMigrationWrites`); one using 0.9.0 as
    any older version is re-pointed at `1.0.0` (`kb.FormatStamp`'s and `kb.MigratedSource`'s tests,
    `refresh_test.go:348`); one whose fixture is a 0.9.0 KB for another subject gets a stamped
    fixture (`cmd/maintenance_test.go`'s `TestRefreshAndVerifyVerbs`); a case whose subject is a
    shape keeps the case and drops 0.9.0 from its name (`export/assemble_test.go:199`,
    `write/ops_test.go:286`).
  - `cmd/format_test.go`'s `kb090Dir` cases, each by name: `TestQueriesReadAnOlderKBAsItsConversion`
    and `TestRefreshMigratesAnOlderKB` are deleted (each compares 0.9.0 against 1.0.0;
    `TestA100KBRefreshesTo200` covers the 1.0.0 refresh); `TestNewerFormatRefusedByEveryReader`'s
    "a newer patch" subtest and `TestNoRefreshRefusedOnAnOlderKB` stage `kb100Dir` instead, their
    expected `from` version `1.0.0`.
  - The contract, each passage losing what it says of 0.9.0 and keeping the rest: SPEC.md §3
    (`:78`); §10 "Reading the stamp" ("either frontmatter form", `:1374`), the `1.0.0` paragraph's
    "in place of the comment block" and "in place of `.jsonl`" (`:1410–1417`), and `:1367–1369`,
    `:1428–1431`, `:1643–1644`; ARCHITECTURE.md "**Format migration (SPEC §10).**" ("either
    frontmatter form", `:1337`; the parity-test sentences, `:1351–1353`) and "**The reference
    comparison is flag-gated and partial.**"; CONVENTIONS.md's superseded-parser rule, its
    parity-test exception sentence.
- **Facts:**
  - `git grep -n -e '0\.9\.0' -e 'UnstampedFormatVersion' -e 'SupersededStamp' -e 'SupersededRecordPaths' -e 'kb-frontmatter' -e 'old090\|is090\|kb090' -- internal cmd justfile SPEC.md ARCHITECTURE.md CONVENTIONS.md ':!internal/migrate'`
    → ARCHITECTURE.md 5, SPEC.md 7, `cmd/format_test.go` 23, `cmd/maintenance_test.go` 3,
    `cmd/resultdocs_test.go` 1, `internal/build/ownedpaths_test.go` 1,
    `internal/claimgraph/buildrecords_test.go` 2, `internal/claimgraph/compat_test.go` 16,
    `internal/export/assemble_test.go` 1, `internal/index/index_test.go` 2,
    `internal/index/refresh_test.go` 1, `internal/kb/format.go` 4, `internal/kb/format_test.go` 3,
    `internal/kb/source_test.go` 2, `internal/kbload/kbload.go` 3, `internal/kbload/kbload_test.go` 7,
    `internal/kbload/parity_test.go` 91, `internal/query/compat_test.go` 1,
    `internal/write/ops_test.go` 1, and five files under `internal/kbload/testdata/*0.9.0`.
  - `git grep -n 'migrate\.SupersededRecordPaths\|migrate\.SupersededStamp' -- internal cmd` →
    `internal/kbload/kbload.go:36, 111`, `internal/build/ownedpaths_test.go:15`.
  - `grep -n -e 'claimgraph.reference' -e 'query.fixtures' -e 'TestStageQueryScores' justfile` →
    nothing;
    `git log -1 --format=%s bedc61b` → "… and the end of the comparison toolchain".
- **Done:** `just fmt-check`; `just test` green; the Facts' first grep, run with
  `-e 'comment-block frontmatter' -e 'either frontmatter'` added and `internal/migrate` included, returns
  nothing; the scope's passages that grep cannot see (the `.jsonl` term of `inIndex`, SPEC's
  "in place of `.jsonl`", ARCHITECTURE's "either frontmatter" where a line break splits it) are
  checked by reading them.

### C2.0b — the migrate tree

- **Design:** none; "The migrate tree" above.
- **Scope:** `internal/migrate` split into `common`, `format_1_0_0`, `format_2_0_0`,
  `from_1_0_0_to_2_0_0` and the root, each declaration placed by one rule: a declaration that says
  what a format's bytes are goes to that format's package; one that transforms goes to the
  converter; one with no format content that two or more packages of the tree use goes to
  `common`; `Chain` and its arithmetic stay in the root. Behaviour-neutral: no converter's output changes. Also
  `cmd/importpolicy_test.go`'s two `migrate` rules, worded so a reader cannot take them to forbid
  the tree's packages importing each other: the tree may import within itself and nothing else of
  kbase; outside the tree only `kbload` imports it. ARCHITECTURE.md I8, the package map's
  `internal/migrate` entry, the dependency-rules bullet and the migration section describe the
  tree as above, and state the reason for the import rule: so maintenance of the live packages can
  never change what an old KB converts to. The policy rules' `why` strings carry the same reason.
- **Facts:**
  - `git ls-files internal/migrate | grep -v testdata/` → `from_1_0_0.go`, `from_1_0_0_test.go`,
    `migrate.go`, `migrate_test.go` (after C2.0a).
  - `git grep -n -o 'migrate\.[A-Z][A-Za-z]*' -- internal cmd ':!internal/migrate'` → `Chain`,
    `Files`, `ConversionError` (`kbload.go`; `ConversionError` is declared at `from_1_0_0.go:499`)
    after C2.0a.
  - `sed -n 94,102p cmd/importpolicy_test.go` → the rules `from: internal/migrate, forbidden: cmd,
    internal` and `except: internal/kbload, forbidden: internal/migrate, direct`; `under`
    (`:144`) matches a prefix and every path beneath it.
- **Tests:** the existing converter tests and goldens pass unchanged; one `cmd/importpolicy_test.go`
  case over a synthetic graph shows a tree package importing another passes and a tree package
  importing `internal/kb` fails.
- **Done:** `just fmt-check`; `just test-pkg ./internal/migrate`, `./internal/kbload`, `./cmd`
  green; `git diff --stat` touches no `testdata/` file.

### C2.1 — text primitives, shared types and the 2.0.x register reader

- **Design:** §8 (the copy list); §2.2.
- **Scope:** new files in `internal/migrate/format_2_0_0`, and the edits the copy rules' identical-
  declaration clause makes in `format_2_0_0` and `from_1_0_0_to_2_0_0`: `internal/kb/pytext.go` whole (`PyRE :24`,
  `NormalizeSpace :115`, `Slugify :176` among it), `Strip`, `StripCodeFences`, `SplitLines`,
  `IDBody`, `WorkIDPattern`, `AxiomLabel`; the shared types `AnnotationFault`, `AnnotationError`,
  `IDRecord`, `MalformedError`, `Bearing`, `Edge`; `internal/kb/register.go` whole (the bullet reader
  with its `known`/`axioms`/`authored` filters, `LocateRegisterEntries`, `ParseClaimEntries`,
  `ParseAnalysisEntries`, `ParseWorkEntries`, `ParsePair`, `ParseStrengthLine`, strengthen-by).
- **Facts:** `wc -l internal/kb/register.go internal/kb/pytext.go` → 1034, 192.
- **Done:** `just fmt-check`; `just test-pkg ./internal/migrate` green; `just edit-gate`.

### C2.2 — the 2.0.x leaf, node and axiom readers

- **Design:** §8; §2.1, §2.4.
- **Scope:** `internal/migrate/format_2_0_0`, and the edits the copy rules' identical-declaration
  clause makes in `format_2_0_0` and `from_1_0_0_to_2_0_0`: `kb.ParseFrontmatter`, `ParseLeaf`,
  `parseExperimentLeaf`, `parseAnalysisLeaf`, `scorePairs`, `declarations`, `MarkerLines`,
  `ParseAxioms`, `parseAxiomsDocument`, `parseLegacyAxioms`, `AuthoredIDs`, and `kb.Discover`'s join
  (the 2.0.x `State`, `known` filter, `unionPairs`, hosted-analysis join), under the copy rules'
  stand-in edit for every `kb.Source` and disk call.
- **Facts:** `wc -l internal/kb/leaf.go internal/kb/discover.go internal/kb/frontmatter.go` → 526,
  551, 314; `sed -n 213,221p internal/kb/discover.go` → only hosted analyses join.
- **Done:** `just fmt-check`; `just test-pkg ./internal/migrate` green; `just edit-gate`.

### C2.3 — parse goldens and the agreement test

- **Design:** C2's Done.
- **Scope:** a `format_2_0_0` test parsing each `*-2.0.0` fixture with the copy: `register-2.0.0`
  against a golden of the copied `State` marshalled by `yaml.v3` (fields as copied), and
  `bad-token-2.0.0` against a refusal golden (the copied `AnnotationError`'s faults, marshalled the
  same way). The goldens are committed files beside their fixtures,
  `internal/migrate/testdata/register-2.0.0.state.yaml` and `bad-token-2.0.0.faults.yaml`. On a
  mismatch the test writes what it got, each file under its golden's name, to
  `test-data/transient/unit_tests/migrate/got/` (from `format_2_0_0`'s directory,
  `../../../test-data/transient/unit_tests/migrate/got/`), and fails; a golden is written by copying
  that output in. An agreement test in `format_2_0_0`'s tests compares, over each fixture, `kb.Discover`
  with the copy: for `register-2.0.0` their states' `yaml.v3` marshals, for `bad-token-2.0.0` their
  faults' marshals, byte-equal. It takes its own `discover` and `fixtureTree` helpers, copies of
  `from_1_0_0_to_2_0_0`'s with the stamp `"2.0.0"`; each of the two `discover` helpers carries a
  one-line comment that a test helper does not cross packages. C3d.5a removes the agreement test
  (no `kb.Discover` left); C3c.1 ports `from_1_0_0_to_2_0_0`'s helper to C2's copy. If the agreement
  test fails, stop and report the differing fields; diagnosing the difference is not this package's.
- **Facts:** `command ls internal/migrate/testdata` → `1.0.0`, `bad-token-1.0.0`, `bad-token-2.0.0`,
  `register-1.0.0`, `register-2.0.0` (after C2.0a).
- **Done:** `just fmt-check`; `just test-pkg ./internal/migrate` green.

### C2.4 — the 2.0.1 parity query goldens

- **Design:** §14's parity case; plan C2 ("records, in the shared tree with the 2.0.1 binary").
- **Scope:** a `just` recipe `parity-goldens` (`[doc]` stating it records 2.0.1 answers) that runs
  `just build`, copies `internal/sheet/testdata/adjagent-sheet` into `.claude-temp/`, and stages the
  cases with the existing flag-gated `TestStageQueryCases` (`internal/query/compat_test.go:87`; flags
  `query.kbroot`, `query.cases`): one `NNN.args` per case, the query then one argument per line. For
  each case it runs, from the copy's repository root, `{{BIN_DIR}}/kbase <args>` (`--limit 0` on a
  list query), writes stdout to `NNN.out` with the absolute `kb-root:` path replaced by a fixed
  placeholder, and the exit status to `NNN.exit`; all committed under
  `internal/kbload/testdata/parity-2.0.1/`.
- **Facts:** `git grep -n 'var comparedQueries' internal/query/compat_test.go` → `:45` (`deps`,
  `gated-on`, `cited-by`, `find`, `referenced-by`, `strength-below`, `subtree`, `show`,
  `weak-points`, `stats`); `git show bedc61b^:justfile | sed -n 1325,1340p` → `run_cases`, the model:
  reads each `NNN.args` line by line, `--limit 0` but for `show` and `stats`.
- **Done:** `just fmt-check`; on a second run, `just parity-goldens && git status --porcelain internal/kbload/testdata/parity-2.0.1/`
  prints nothing; C2's gates `just checkpoint`, `just test-integration` green.

---

## C3 — the format flip (worktree)

Each package converts the fixtures and goldens its own tests read (design §15), by hand to the
design's 3.0.0 form, stamped `3.0.0`, until C3c. Migration-input fixtures older than 2.0.1
(`cmd/testdata/score-kb-1.0.0`, `internal/migrate/testdata/*`) are never converted.

### C3a — records, the load and `index`'s own consumers

#### C3a.1 — record types, encode/decode, the strict load, `3.0.0`

- **Design:** §3.1, §3.2, §3.3, §3.4 (the record), §3.7's refusals.
- **Scope:** `internal/kb`: a record-built state (`kb.Graph`) beside the 2.0.x `kb.State`, which
  retires with its last caller (C3d.5a); one type per §3.2/§3.3 record type; the decoder with the
  required/refused rule, quoting and escaping (`index.streamUnsafe`'s list moves into `kb`); the
  bytes encoder; the absent-file refusal (`check: index`); `kb.FormatVersion = "3.0.0"`.
  `internal/index`: the file write of the encoded bytes through `index.WriteKBFile`.
- **Facts:** `go list -f '{{join .Imports " "}}' ./internal/kb | tr ' ' '\n' | grep '^kbase/'` →
  `kbase/internal/log`; `git grep -n 'streamUnsafe' internal/index/records.go` → `:337`;
  `git grep -n 'func WriteKBFile' internal/index` → `refresh.go:142`, also called from
  `internal/write/store.go:692`; `git grep -n 'func BearingFromValue\|PendingLiteral *=' internal/kb`
  → `register.go:99`, `schema.go:11`; `git grep -n 'FormatVersion *=' internal/kb/format.go` →
  `2.0.1`.
- **Tests:** a round-trip test per §3.2/§3.3 type; one test per §3.7 refusal class (parse failure,
  unknown field, missing field, wrong type, domain, id grammar, duplicate id, illegal pairing, absent
  file, a string holding a line break).
- **Done:** `just fmt-check`; `just test-pkg ./internal/kb` green; the Allowed-red runs.

#### C3a.2 — markers, document kind, hosts

- **Design:** §4 (form, single-marker rule, statements); §2.1's `kind` row.
- **Scope:** `internal/kb`: the marker reader widened to `clm-`/`ana-`/`exp-` (`kb.NodeIDs`); the
  document-kind rule moved from `internal/claimgraph/tree.go`'s `documentKind` into `kb`, and
  `tree.go` calling it; hosts (first marker in path order) with the no-marker and several-marker
  faults returned to the caller.
- **Facts:** `git grep -n 'func documentKind' internal` → `internal/claimgraph/tree.go:125`;
  `sed -n 21,22p internal/kb/leaf.go` → `tier2InlineRE`; `MarkerLines` (`leaf.go:88`) keys on
  `ClaimIDs`.
- **Tests:** hosts for one, two and no markers; kind by path and child list.
- **Done:** `just fmt-check`; `just test-pkg ./internal/kb` green; the Allowed-red runs.

#### C3a.3 — `index.Derive` over records and markers

- **Design:** §3.5, §3.6, §5's placement and "which pages exist" bullets.
- **Scope:** `internal/index`: `Derive` over `kb.Graph` and markers, canonical paths and anchors, each
  node's register page; `checkCoverage` and the cycle refusal leave `Derive`. Hand-convert
  `internal/sheet/testdata/adjagent-sheet/kb-root` to 3.0.0; move the sheet's tests to an external
  `sheet_test` package calling `index.Derive`; adapt `BenchmarkLoadDerive`. `internal/index/rows_test.go`
  and `refresh_test.go:97` stop calling `kb.ReadIndex` and `IndexRowType` (table above). Keeps
  `internal/query/load.go` and `internal/build/rows.go` building against `Derive` (`write`'s tests
  import `query`: `internal/write/bearing_test.go:14, 194, 505`).
- **Facts:** `git grep -n 'func checkCoverage\|func coverageAt' internal/index` → `records.go:236`,
  `:270`; SPEC.md §3: "`kb-root/<volume>/claim-graph.svg`, one per volume, a top-level directory
  holding an `index.md`".
- **Tests:** placement (volume host, host outside every volume, work, unhosted); canonical path and
  anchor per kind; `Derive` over the converted fixture.
- **Done:** `just fmt-check`; `just test-pkg ./internal/kb` green; `just bench-load` runs; the
  Allowed-red runs.

#### C3a.4 — the degraded load and cycle breaking

- **Design:** §3.7 (degraded), §10.
- **Scope:** `internal/index`: the degraded state computed in `Derive`, resolved for the walk;
  `index.BreakCycles` beside `DependsGraph`, with its warnings; `ComputeStrength`'s `CycleError` no
  longer refuses a load. Keeps `internal/query/load.go` and `internal/build/rows.go` building against
  `Derive`.
- **Facts:** `git grep -n 'func ComputeStrength\|type CycleError\|func AuthoredDependsGraph\|func (g DependsGraph) Path' internal/index`
  → `strength.go:334`, `strength.go:54`, `dependspath.go:17`, `dependspath.go:48`;
  `git grep -n -A10 'type DependsGraph' -- 'internal/index/*.go'` → every scoring relation enters it.
- **Tests:** one per degraded row; §10's walk over a fixed multi-cycle graph cutting a stated edge set
  (one cycle holding a `supports` edge into an analysis); no cut drops an edge.
- **Done:** `just fmt-check`; the Allowed-red runs (`internal/index`'s new tests pass).

#### C3a.5 — register pages, freshness, dead-link fragments

- **Design:** §5 (all bullets); §2.2's *rendered on the register page* rows.
- **Scope:** `internal/index`: the page renderer, its order and content; the freshness comparison;
  `LinkFindings` passing each page's derived slugs to the dead-link gate. `internal/kb`:
  `DeadLinksIn` gains the slug input; `kb.DeadLinks` keeps its signature for its callers.
- **Facts:** `git grep -n 'func DeadLinksIn\|func LinkFindings' internal` → `kb/deadlinks.go:52`,
  `index/links.go:13`; `git grep -n 'kb\.DeadLinks(' -- internal cmd ':!*_test.go'` →
  `internal/claimgraph/conform.go:190`, `internal/docgraph/build.go:180`;
  `git grep -n 'func renderEntry\|func renderWorkEntry' internal/write` → `render.go:201`, `:243`
  (incumbents; `index` may not import `write`).
- **Tests:** page bytes for the converted fixture; freshness fault and fix; a moved node's old
  fragment reported.
- **Done:** `just fmt-check`; `just test-pkg ./internal/kb` green; the Allowed-red runs.

#### C3a.6 — `index`'s refresh, non-migration path

- **Design:** §9.3; §3.7's repair; §8.5's `refresh` `warnings`.
- **Scope:** `internal/index/refresh.go`: phases become pages, sheets and, when degraded, the repair
  write; `aggregates`, `strength`, `leafReferences`, `emit` and the frontmatter splice retire;
  `removed` reports pages no longer called for. Nothing `verify.go` (C3a.7's file) still calls
  retires here: `BuildRecords`, `fieldsOf` and `Serialize` stay for C3a.7. The migration save stays
  as it is (C3c.4).
- **Facts:** `sed -n 98,110p internal/index/refresh.go` → phases `aggregates, strength,
  leafReferences, emit, sheet, save`; `git grep -n 'BuildRecords(\|Serialize(' internal/index/verify.go`
  → `:576`, `:589`.
- **Tests:** refresh over the converted `adjagent-sheet` writes pages and sheets and no derived
  `.index/` file; a degraded fixture's repair write.
- **Done:** `just fmt-check`; the Allowed-red runs.

#### C3a.7 — `index`'s verify

- **Design:** §9.4; §3.4's `axiom heading`; §4's `statement marker`.
- **Scope:** `internal/index/verify.go`, `citations.go`, `links.go`; retires `coverageAt`,
  `BuildRecords`, `fieldsOf`, `Serialize`, `kb.WorkCitations`, `kb.WalkRegisters`,
  `kb.LocateLeafReferenceFooters`, `kb.LegacyAxiomDeclarations`, and `index`'s own production call of
  `kb.IndexFiles` (table above). Ports `internal/index`'s tests off `kb.Discover` (`index_test.go:334,
  515`, `migration_test.go:60`) and `kb.IndexFiles` (`rows_test.go:71`).
- **Facts:** the 2.0.x check names: `git grep -n -o -E 'add(At)?\("[a-z0-9 -]+"' internal/index/verify.go | sed 's/.*("//' | sort -u`,
  plus `"acyclicity"` (`verify.go:353`), `CheckIndexFreshness = "index freshness"` (`:209`),
  `CheckFrontmatterPresence` `"frontmatter"` (`:55`), `CheckAxiomSource` (`:117`), `"dead link"` and
  `"unknown id"` (`links.go`); `sed -n 71,96p internal/index/citations.go` → `inScopeText` keys on
  `kind:`.
- **Done:** `just fmt-check`; `just test-pkg ./internal/kb`, `./internal/index` green (C3a's Done);
  the Allowed-red runs.

### C3b — write ops

#### C3b.1 — the record store, provenance, field ops

- **Design:** §6.1, §6.2; §9.2's intro and the `set-rigor`, `set-rationale`, `set-bearing`,
  `set-status` rows; §3.7's write-op refusal.
- **Scope:** `internal/write`: read, modify and whole-file write of records (bytes from `kb`, file
  write through `index.WriteKBFile`); `Options.Provenance` (`ProvenanceCaller` zero,
  `ProvenanceMachine`); read-back over records; the four ops; `check: degraded`. The splices retire
  as their callers go.
- **Facts:** `sed -n 57,74p internal/write/ops.go` → `writeOps`; `sed -n 686,692p internal/write/store.go`
  → only touched files written, a migrated KB's entry point held in memory (`src.Put`, `:691`),
  others through `index.WriteKBFile` (`:692`); `sed -n 233,239p internal/write/ops.go` →
  `--no-refresh` refused on a migrated KB.
- **Tests:** one per §6.1 row.
- **Done:** `just fmt-check`; the Allowed-red runs.

#### C3b.2 — the inserts

- **Design:** §4 ("Placing one", "Adoption"); §9.2's insert rows.
- **Scope:** `internal/write`: the five inserts over records; marker placement (`document`;
  `locator` for claims; one without the other refused); adoption by title and host; optional
  `rigor` and bearings; the `register` value goes. `CreatesRegister` and `opSpec.creates` stay (their
  callers are in `cmd`, C3b.4).
- **Facts:** `git grep -n -B2 -A8 '^var insertCommon' internal/write/values.go` → `register`,
  `title`, `rigor` (required), `rationale`, `rests-on`, `no-edge`; `sed -n 723,737p internal/write/ops.go`
  → `adoptExperiment`'s full equality.
- **Tests:** adoption (hosted in `document`, unmarked, none → mint; `unchanged` vs `done`); the
  one-key refusal; `internal/write/testdata/writeapi-render-golden` and `writeapi-regression`
  converted as these tests need.
- **Done:** `just fmt-check`; the Allowed-red runs.

#### C3b.3 — edge ops and markers

- **Design:** §9.2's remaining rows; §3.3's demoted record; §10's last paragraph.
- **Scope:** `internal/write`: `add-rests-on`, `add-pairs`, `drop-claim-edges`, `drop-inbound-edges`,
  `drop-claim-node`, `resolve-demoted`, `mark-claim-in-leaf`, `add-build-edges`, `demote-claim-edge`;
  `set-frontmatter` retired; `origin` gone from values. Retires the `kb` readers whose last caller is
  `write` (table above), and ports `internal/write`'s tests off `kb.Discover`,
  `kb.ParseClaimEntries`, `kb.ParseAnalysisEntries` and `kb.ReadIndex` (`bearing_test.go:227, 490,
  523, 562`, `experiment_test.go:28`, `opscript_test.go:67`, `regression_test.go:63, 66, 156`).
- **Facts:** `git grep -n -A4 'ResolveDemoted: {' internal/write/values.go` → `id`, `target`
  (`checkClaimID`), `action`; `git grep -n 'func planDemoteClaimEdge\|func planResolveDemoted\|func planMarkClaim\|func planSetFrontmatter' internal/write/ops.go`
  → `:1382`, `:1987`, `:2472`, `:2142`.
- **Tests:** restore rebuilds relation, bearing and `manually-scored`; cycle refusal per adding op;
  marker move.
- **Done:** `just fmt-check`; `just test-pkg ./internal/write`, `./internal/kb` green (C3b's Done);
  the Allowed-red runs.

#### C3b.4 — op help, flags and MCP schema

- **Design:** §15's `Short` and `Long` tables; §9.2's quoted help; §9.2's `--create` paragraph.
- **Scope:** `CreatesRegister` and `opSpec.creates` removed (`internal/write/ops.go:88–89`) with their
  callers: `OpVocabulary`'s `create` result (`internal/write/ops.go:99–108`; called at
  `cmd/mcp.go:152`, `cmd/mcp_test.go:402`), `ops.go:244` (`ctx.create`), `Options.Create`
  (`cmd/write_op.go:59, 145`, `cmd/mcp.go:201`), `cmd/write_op.go:159` (the flag), `cmd/mcp.go:171–194`
  (`createArgument` and its schema property), `cmd/mcp_test.go:415–416`. The op command files: `git grep -ln 'writeOpCommand(' cmd` →
  `add-rests-on.go`, `drop-claim-edges.go`, `drop-claim-node.go`, `drop-inbound-edges.go`,
  `insert-analysis-entry.go`, `insert-claim-entry.go`, `insert-experiment-entry.go`,
  `insert-work-entry.go`, `mark-claim-in-leaf.go`, `render-citation.go`, `resolve-demoted.go`,
  `set-bearing.go`, `set-frontmatter.go`, `set-rationale.go`, `set-rigor.go`, `write_op.go`; new
  `set-status.go`, `add-pairs.go`, `insert-axiom-entry.go`; `cmd/set-frontmatter.go` deleted.
- **Prompt-engineer gate:** the report quotes every `Long` the design does not quote, verbatim, under
  "For prompt-engineer review"; the coordinator commits after that review.
- **Done:** `just fmt-check`; `just test-pkg ./internal/write` green (edited after going green); the
  Allowed-red runs (`cmd` first builds in C3d.5c).

### C3d — the remaining consumers

#### C3d.1 — the queries

- **Design:** §9.1, §6.3.
- **Scope:** `internal/query` (`load.go`: the migrated refusal goes, `check: degraded` added;
  `payload.go`: §6.3's keys); `cmd/query.go`; the MCP transcripts carrying query output,
  `test-data/fixtures/mcp/stats.jsonl` and `edges-relation.jsonl`.
- **Facts:** `git grep -n 'func ShowPayload\|func DepsPayload\|func StatsPayload\|func EdgeCensus\|func EdgesPayload' internal/query`
  → `payload.go:224`, `:128`, `:274`, `:324`, `:372`.
- **Done:** `just fmt-check`; `just test-pkg ./internal/query` green; the Allowed-red runs.

#### C3d.2 — `score`'s skip

- **Design:** §7 (but its last bullet, C3c.4), §9.6.
- **Scope:** `internal/score` (`ledger.go`: status `skipped`, `Resume`'s skip predicate, `reprice`;
  `run.go`: the walk's skip, `Counts`; `text.go`: statements through markers); `cmd/score.go`
  (`--rescore-manual`, the `resume` command, `check: degraded`, `landScore` with `ProvenanceMachine`),
  `cmd/score_status.go` (counts), `cmd/score_replay.go:163–165` (counts gain `skipped`). Ports
  `internal/score`'s tests off `kb.ReadIndex` (`text_test.go:140`) and `kb.IndexFiles`
  (`run_test.go:132`).
- **Facts:** `git grep -n 'StatusDone = \|StatusFailed = \|StatusPending = ' internal/score/ledger.go`
  → `:28`, `:30`, `:33`; `git grep -n 'func Resume\|func (l \*Ledger) reprice\|func (l \*Ledger) refusalFor' internal/score/ledger.go`
  → `:234`, `:942`, `:268`; `git grep -n 'func landScore' cmd/score.go` → `:570`.
- **Done:** `just fmt-check`; `just test-pkg ./internal/score` green; the Allowed-red runs.

#### C3d.3 — the sheet

- **Design:** §9.7's `render-claim-graph` bullet; Ruling 19.
- **Scope:** `internal/sheet`: one style per relation, no cited or inferred variant; no build-record
  reads; the `buildrecords` and `asks` imports go; DOT goldens re-pinned, the new truth named in the
  commit body.
- **Facts:** `git grep -n 'asks\.\|buildrecords\.' internal/sheet/*.go | grep -v _test` →
  `graph.go:104, 108, 150`; `git grep -n -i 'origin' internal/sheet/*.go | grep -v _test | wc -l` → 16.
- **Done:** `just fmt-check`; `just test-pkg ./internal/sheet` green; the Allowed-red runs.

#### C3d.4 — `claimgraph` and `buildrecords`

- **Design:** §9.5's `claims-declared` … `depends-attributed` and reproducibility rows; §4's build
  bullet; §10.
- **Scope:** `internal/claimgraph`: `writer.landLeaf` (marks every claim; inserts with `document` and
  `locator`), `writer.writePlan` (pass-2 `set-frontmatter` goes), `writer.writeWorks`, `readGraph`,
  `passTwoGate`, `checkCleanliness` (refuses a non-empty `nodes.yaml`), `IssueCulls`, `leafOf`,
  `ringRow` → `index.BreakCycles`, `readModuloIDsView`; `cycleEdges`, `demotedOrigin`, `inferredAt`,
  `registerFor` retire; `internal/buildrecords`: `InferredLinks` retires. Ports `internal/claimgraph`'s
  tests off `kb.Discover` (`depends_stage_test.go:100`).
- **Facts:** `git grep -n 'inferredAt(\|InferredLinks(' -- internal ':!*_test.go'` →
  `buildrecords.go:173`, `claimgraph/buildrecords.go:197, 207`, `claimgraph/stages.go:596`
  (`sheet/graph.go:150` gone in C3d.3); `sed -n 340,345p internal/claimgraph/write.go` → the
  `prose[id] || counted > 1` condition; `sed -n 221,272p internal/claimgraph/conform.go` →
  `checkCleanliness`.
- **Done:** `just fmt-check`; `just test-pkg ./internal/claimgraph`, `./internal/buildrecords`,
  `./internal/kb` green; the Allowed-red runs.

#### C3d.5a — `build` and `export`

- **Design:** §9.5's `start`, `spine-seed`, `phase-3a` and resume rows; §9.7's `export-latex`
  bullet.
- **Scope:** `internal/build/seed.go` (empty `nodes.yaml`, `edges.yaml`; the
  `CheckFrontmatterPresence` filter goes), `rows.go` (resume refused while degraded);
  `internal/export/assemble.go` (`bibliography` from records). With `export`'s
  `kb.ParseWorkEntries`/`kb.Registers` calls gone, no production caller of the last 2.0.x `kb`
  readers remains: they retire here with `internal/kb`'s own tests of them (table above). Ports
  `internal/build/inference_test.go:404` off `kb.Discover` and removes C2.3's agreement test; the
  test-caller grep then finds only `internal/migrate` and `internal/kbload` (red until C3c).
- **Facts:** `sed -n 40,50p internal/build/seed.go` → the filter; `git grep -n 'ParseWorkEntries' internal/export`
  → `assemble.go:403`; `git grep -l 'kb\.Discover\b\|kb\.Registers\b' -- internal cmd ':!*_test.go' ':!internal/kb'`
  → `claimgraph` (ported in C3d.4), `export`, `index`, `write` (ported in C3a, C3b).
- **Done:** `just fmt-check`; `just test-pkg ./internal/build`, `./internal/export`, `./internal/kb`
  green; the Allowed-red runs.

#### C3d.5b — the `cmd`-side fixtures

- **Design:** §3.1–§3.3 (the record forms), §4 (markers), §8 (the mapping), §9.2 (the inserts'
  values without `register`), §9.7 (`init`), §15.
- **Scope:** hand-convert to 3.0.0, stamped `3.0.0`: `test-data/fixtures/results/kb-root`,
  `test-data/fixtures/results/values/insert.yaml` (drop `register`), `test-data/fixtures/mcp/seed.yaml`
  (drop `register`), `cmd/testdata/score-kb`; `test-data/fixtures/init/skeleton` re-pinned to the new
  `init`'s output (entry point, two empty files). Every conversion follows the design's record forms and
  §8's mapping; a case they do not settle is reported, not decided. A generated page is not written:
  `cmd/testdata/score-kb/kb-root/claim-rigor.md` is removed (`git rm`), and C3d.5c generates it.
- **Facts:** `git ls-files test-data/fixtures | grep -E 'seed.yaml|insert.yaml'` →
  `test-data/fixtures/mcp/seed.yaml`, `test-data/fixtures/results/values/insert.yaml`;
  `git ls-files cmd/testdata/score-kb` lists the score fixture's files.
- **Done:** `just fmt-check`; the Allowed-red runs (`cmd` builds in C3d.5c).

#### C3d.5c — the `cmd` verbs

- **Design:** §9.7 (`export-latex`, `init`, `status`); the `refresh` and `verify` verbs over C3a's
  `index`; §6.2's test.
- **Scope:** `cmd/export-latex.go` (`check: degraded`; `export` may import only `kb` and
  `atomicfile`, so the verb refuses); `cmd/init.go`; `cmd/refresh.go` (`warnings`), `cmd/verify.go`,
  `cmd/render-claim-graph.go` (degraded refusal; its `.index/` check folds into the load). Ports
  `cmd`'s tests off `kb.ReadIndex` (`demoted_test.go:24`, `resultdocs_test.go:259`) and `kb.IndexFiles`
  (`format_test.go:164`, `init_test.go:185`), which then retire. Observes in `cmd` §6.2's test that no
  flag or tool property names the provenance, and that no tool carries `create`. Generates
  `cmd/testdata/score-kb/kb-root/claim-rigor.md` with the built binary's `refresh` over the fixture
  and commits it; the page is never written by hand.
- **Facts:** `sed -n 160,171p cmd/init.go` → `skeletonEntryPoint` writes `kind` and the stamp;
  `sed -n 50,54p cmd/render-claim-graph.go` → the `.index/` refusal.
- **Done:** `just fmt-check`; `just test-pkg ./cmd` green; the Allowed-red runs.

#### C3d.6 — the model-facing texts

- **Design:** §9.8, §11, §13.1.
- **Scope:** `cmd/mcp_instructions.txt`; `internal/kbdocs/templates/AGENTS.tmpl.md`,
  `CONVENTIONS.tmpl.md`; §11's test in `internal/kbdocs`; `test-data/fixtures/mcp/initialize.jsonl`.
  Not touched: `README.tmpl.md` (C3e.1).
- **Facts:** `git grep -n 'go:embed mcp_instructions\|func servedInstructions' cmd` → `cmd/mcp.go:432`,
  `:445`; `git grep -n 'go:embed' internal/kbdocs/kbdocs.go` → `:21`.
- **Done:** `just fmt-check`; `just test-pkg ./internal/kbdocs`, `./cmd` green (C3d's Done:
  `cmd`, `query`, `score`, `sheet`, `claimgraph`, `build`, `kbdocs` green, §11's test included).

### C3c — the converter

#### C3c.1 — the 2→3 mapping, emitter and chain step

- **Design:** §8's intro; §8.1 but its marker and stamped-document rows; §8.3, §8.4, §8.5.
- **Scope:** `internal/migrate/from_2_0_0_to_3_0_0` (new): `convert200To300` over `format_2_0_0`'s
  readers, the records emitter, the frontmatter strip, the warnings; the root's `converters` entry;
  `internal/kb/source.go` (`Migration` gains `Warnings`; the new `MigrationNote`);
  `internal/kbload/kbload.go` (the 2→3 step through `migrate.Chain`, its only caller, `:78`;
  `MigratedSource`, `:87`); `cmd/refresh.go` `migrationNotice` prints the warnings. Ports
  `from_1_0_0_to_2_0_0`'s `discover` test helper (calling `kb.Discover`) to `format_2_0_0`'s
  reader.
- **Blocked on Open question 1** (`ACTIVE_PLAN.md`): where the 3.0.0 record shape the emitter
  writes is defined. Not dispatchable until it is ruled.
- **Facts:** `git grep -n 'var converters\|func Chain' internal/migrate/migrate.go` → `:27`, `:38`;
  `git grep -n 'MigrationNote =' internal/kb/source.go` → `:23`; `git grep -n 'func migrationNotice' cmd`
  → `cmd/refresh.go:123`.
- **Done:** `just fmt-check`; `just test-pkg ./internal/kb` green (edited after going green); the
  Allowed-red runs (`internal/migrate`'s new tests pass by name; `internal/kbload` stays red until
  C3c.4, its owner in the Allowed-red table).

#### C3c.2 — markers from `claims:` listings

- **Design:** §8.2; §8.1's analysis and experiment host row.
- **Scope:** `internal/migrate` only.
- **Facts:** `git show release-0.8.0:internal/index/verify.go | sed -n 465,490p` → 2.0.x requires a
  marker only for two or more non-equation claims.
- **Tests:** one per §8.2 row and insertion site.
- **Done:** `just fmt-check`; the Allowed-red runs.

#### C3c.3 — the stamped documents' restamp

- **Design:** §8.1's stamped-documents row; §13.1.
- **Scope:** `internal/migrate` (both forms of each paragraph); a test in `internal/kbdocs` that the
  3.0.0 paragraphs equal the templates'.
- **Facts:** `sed -n 96,104p internal/kbdocs/kbdocs.go` → `Stamp` writes only where absent.
- **Done:** `just fmt-check`; `just test-pkg ./internal/kbdocs` green; the Allowed-red runs.

#### C3c.4 — the migration save and the golden pairs

- **Design:** §8.6; §9.3's migration path; §7's last bullet; §9.7's unmigrated `render-claim-graph`.
- **Scope:** `internal/index/refresh.go` (save order: authored files, leaves, stamped documents,
  pages, sheets, removals, stamp); `internal/kbload`; `cmd/render-claim-graph.go` (draws from the
  in-memory conversion, writes sheets only); `cmd/score.go` (a KB still unmigrated after the start
  refresh is refused); `from_2_0_0_to_3_0_0` golden pairs: `register-2.0.0` and `bad-token-2.0.0` as
  2→3 pairs, and `adjagent-sheet` at `release-0.8.0` (`git show release-0.8.0:<path>` per file)
  against C3a.3's hand conversion.
- **Facts:** `sed -n 140,160p internal/kbload/kbload.go` → `.index/*.yaml` files enter the
  conversion's file set (`inIndex`, `:154`).
- **Tests:** every §8.5 warning class; the §8.6 unmigrated write order (`edges.yaml` before
  `nodes.yaml`); the partial-save rule; `score`'s still-unmigrated refusal.
- **Done:** `just fmt-check`; `just test-pkg ./internal/migrate`, `./internal/kbload`,
  `./internal/index`, `./cmd` green (C3c's Done).

### C3e — the contract and the remaining fixtures

#### C3e.1 — the contract documents

- **Design:** §13 (all but §13.1).
- **Scope:** SPEC.md, ARCHITECTURE.md, CONVENTIONS.md, `USER_CONVENTIONS.md`, ROADMAP.md, README.md
  (~255, 338, 345, 368), `internal/kbdocs/templates/README.tmpl.md` (including `origin`, line 42).
- **Prompt-engineer gate:** `README.tmpl.md`'s new wording is not quoted in the design; the report
  quotes it verbatim under "For prompt-engineer review", and the coordinator commits after that
  review.
- **Facts:** `grep -n 'register\|frontmatter\|origin' README.md` → `:255`, `:338`, `:345`, `:368`;
  `grep -n 'origin' internal/kbdocs/templates/README.tmpl.md` → `:42`;
  `grep -n 'set-frontmatter\|claims\|no-claim\|register\|create' USER_CONVENTIONS.md` gives §13's lines.
- **Done:** `just fmt-check`; `just edit-gate`; for each §13 row, `grep -n` finds the new wording in
  the named document and finds the retired clause nowhere; the report lists both.

#### C3e.2 — the remaining fixtures

- **Design:** §15.
- **Scope:** `test-data/fixtures/compat/ops.yaml` and `README.md`; the `justfile` legs that insert
  with `--create`, seed `register:`, or call `set-frontmatter`; any `*_test.go` shape no earlier
  package took. No 2.0.x KB fixture remains to convert: every one was hand-converted by its owner.
- **Facts:** `grep -n -e 'entry --create' -e 'register: claim-rigor' -e 'claims: \[' -e 'set-frontmatter --values' justfile`
  → 7 lines at `e95e735` (the `--create` inserts, the `register: claim-rigor.md` seed, the
  `kind: leaf`/`claims:` `set-frontmatter` values, its call, and the MCP staging line).
- **Done:** `just fmt-check`; `just test` green.

#### C3e.3 — the harnesses and the closing gates

- **Design:** §14; §12's rerun.
- **Scope:** `internal/query/compat_test.go` (kept, §14's expected differences);
  `internal/index/compat_test.go` (retired); a new `internal/kbload/parity_test.go` holding the 2.0.1 case, comparing
  against C2.4's goldens with the same `kb-root:` placeholder applied to 3.0.0's answers.
- **Facts:** `git ls-files | grep compat_test` → `internal/claimgraph/`, `internal/index/`,
  `internal/query/compat_test.go`.
- **Done:** `just checkpoint`, `just test-integration` green; `just bench-load` rerun on the converted
  fixture, numbers in the report (C3e's Done).

**Release gate** (the plan's): before `just dist`, the owner confirms each design §16 item landed in
adjagent or was settled.

---

## ⚠ Frame departures

1. **Dispatch granularity.** Each of the plan's C1, C2 and C3a–e is split into several dispatches;
   order, trees and gates are unchanged. C3d's internal order — queries, `score`, sheet,
   `claimgraph`, `build` and `export`, the `cmd`-side fixtures, the `cmd` verbs, texts — follows the
   import graph so `cmd` builds in C3d.5c.
2. **§6.2's test is observed in C3d.5c, not in C3b.** The plan puts "§6.2's test that no flag or tool
   property names the provenance" in C3b's Done; flags and tool properties live in `cmd`, which
   first builds in C3d.5c, so C3b.1 writes the provenance and C3d.5c observes the test.

## Design coverage

| Design section | Package(s) |
|---|---|
| §2 | C2.1, C2.2 (copy source); retirements per the table above |
| §3.1–§3.3, §3.7 strict | C3a.1 |
| §3.4 | C3a.1, C3b.2 (`insert-axiom-entry`), C3a.7 (`axiom heading`) |
| §3.5 | C3a.3 |
| §3.6 | C1.1 (2.0.1 rows), C3a.3 |
| §3.7 degraded | C3a.4; refusals C3b.1, C3d.1, C3d.2, C3d.5a, C3d.5c; repair C3a.6 |
| §4 | C3a.2; placement and adoption C3b.2, C3b.3; build C3d.4; statements C3d.2 |
| §5 | C3a.3 (placement), C3a.5 (render, freshness, fragments), C3a.6 (removal) |
| §6.1, §6.2 | C3b.1; `cmd` observation C3d.5c |
| §6.3 | C3d.1 |
| §7 | C3d.2; last bullet C3c.4 |
| §8 copy list | C2.1, C2.2 |
| §8.1 | C3c.1, C3c.2 (host row), C3c.3 (stamped row) |
| §8.2 | C3c.2 |
| §8.3–§8.5 | C3c.1 |
| §8.6 | C3c.4 |
| §9.1 | C1.2, C3d.1 |
| §9.2 | C3b.1–C3b.4 |
| §9.3 | C3a.6, C3c.4 (migration path), C3d.5c (verb) |
| §9.4 | C3a.7, C3a.5 (fragments), C3d.5c (verb) |
| §9.5 | C3d.4, C3d.5a |
| §9.6 | C3d.2 |
| §9.7 | C1.3, C3d.3, C3d.5a, C3d.5c, C3c.4 |
| §9.8 | C3d.6 |
| §10 | C3a.4; build C3d.4; op refusal C3b.3 |
| §11 | C3d.6 |
| §12 | C1.4, C3a.3, C3e.3 |
| §13 | C3e.1 |
| §13.1 | C3d.6, C3c.3 |
| §14 | C2.4 (goldens), C3e.3 |
| §15 | each package's fixtures; C3b.4 (help); C3e.2 |
| §16 | release gate |
