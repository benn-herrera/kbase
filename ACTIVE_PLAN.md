# ACTIVE PLAN — KB format 3.0.0: authored data in two files, everything else derived

The KB's metadata moves out of the prose and into two authored YAML files under `kb-root/.index/`:
`nodes.yaml` and `edges.yaml`. They are the only metadata kbase parses. Every derived value is
computed at load. The registers (`claim-rigor.md`) become generated pages for people, as the
claim-graph sheets are. Leaves become prose and statement markers. Scores gain `manually-scored`,
and `score` leaves manually scored values alone by default.

## Terms

- **Authored:** a value only a write op or `build` sets, held in `nodes.yaml` or `edges.yaml`.
- **Derived:** computed at load from authored data and the prose, and never stored as data.
- **Generated page:** a file kbase writes for reading and never reads back — the claim-graph sheets
  today, the registers under this plan.
- **Edge identity:** (`relation`, `source`, `target`).
- **Statement marker:** today's Tier-2 marker `<!-- claim-rigor: <id> -->` in a leaf's prose (SPEC
  §10, "The register's vocabulary"), which anchors the node's statement.
- **The ModernCorp KB:** `/Users/agent-user/projects/adjagent/.claude-temp/mc-fixture/kb-root`, a
  2.0.1 KB of 157 documents. It is another project's scratch copy: facts below were read from it,
  and nothing in this repo may depend on it.

## Settled shape

1. **`.index/` is kbase-owned.** Sessions never read or write it (the MCP instructions' prohibition),
   git tracks it, and the tools are the only access. It holds exactly two files, `nodes.yaml` and
   `edges.yaml`, both authored. Strength, bands, roll-ups, citation edges, target kinds and every
   other derived value are computed at load and stored nowhere.
2. **Edge records, one type per relation group, one record per edge identity in `edges.yaml`:**
   - `UnscoredEdge` — `references`: `relation`, `source`, `target`, `context`;
   - `DemotedEdge` — `demoted`: those, plus the edge it replaced — its former `relation`, and
     that relation's `bearing` and `manually-scored` where its type carries them (Ruling 17);
   - `BearingEdge` — `founds`: those of `UnscoredEdge`, plus `bearing`;
   - `ScorableEdge` — `supports`, `lifts`, `refutes` (`kb.BearingWalkRelations`): those of
     `UnscoredEdge`, plus `bearing` and `manually-scored`.
   A field is required where its type has it and refused where it does not. `context` is required
   and may be `""`.
3. **Node records, one type per kind, one record per node in `nodes.yaml`,** under the same
   required/refused rule. Phase A names each type's fields, title and an experiment's status among
   them. A claim, an analysis and an experiment (the kinds `rigorItems` walks) carry `rigor` and
   `manually-scored`; a work carries `rigor` and no `manually-scored`; an axiom carries no `rigor`,
   its `1.0` being by rule. The analysis and experiment records leaf frontmatter holds today move
   here. A node's host document is the leaf holding its statement marker.
4. **`manually-scored`:**
   - a public write op, called through the CLI or MCP, writes `true` beside each rigor or bearing
     its caller supplied, `*pending*` included, on a record whose type carries the field;
   - a value a public write op creates without the caller supplying it (a defaulted `*pending*`),
     and every value `build` writes, is written `false`;
   - a value the call does not write keeps its flag;
   - a `score` landing writes `false`, through an internal provenance parameter that is never a
     flag and never in a tool schema;
   - re-asserting the current value on a record whose `manually-scored` is `false` sets it `true`
     and is `done`, not `unchanged`.
5. **`score --rescore-manual none|rigor|bearing|both`**, default `none`: a value whose
   `manually-scored` is `true` is skipped unless the flag names its pass's kind, and the ledger
   records the skip.
6. **Visibility.** `show`, `deps` and `edges` report `manually-scored` beside each value that has
   one; `stats` counts manual against model per score kind; `edge-census` splits its bearing counts
   by it.
7. **Leaves are prose and statement markers.** Leaf frontmatter is removed once each key it carries
   has a home; the entry point keeps its `kb-format` stamp.
8. **Registers are generated pages**, written by kbase for reading and never read back.
9. **`refresh` regenerates the generated pages; `verify` checks the authored data and the prose.**
10. **KB format 3.0.0**, a major: a converter step from 2.0.x lifts register entries, `rests-on`,
    `references:` and `demoted` lists, pair lines and frontmatter node records into the two files,
    every `manually-scored` written `false`, and the registers re-render as generated pages.
11. **The session rule**, one source text placed in `cmd/mcp_instructions.txt` (plain text) and
    `internal/kbdocs/templates/AGENTS.tmpl.md` (Markdown), a test holding both to it: a rigor or
    bearing is changed only when the user directs that change; a session never sets one on its own
    judgement, and `score` is the only way to re-judge.
12. **A benchmark `just` target** times load plus full derivation on an in-repo fixture; its number is
    reported with the landing.

## Rulings

13. **An upgrade never refuses.** Every converter step that loses or invents data picks a defined
    outcome and warns, naming file and line; a step with no data consequence does not warn. SPEC
    §10's `check: conversion` refusal and "never makes invalid bytes valid" are amended to this.
    - An edge identity authored at more than one site: equal bearings make one record with the
      contexts joined, silently; different bearings keep the higher, with a warning.
    - Register text outside every entry is dropped silently; text inside an entry that fits no field
      is dropped with a warning quoting it; an unrecognised frontmatter key is dropped with a
      warning.
    - A 2.0.x KB the 2.0.x load refuses is not the converter's input.
14. **No host field.** A hosted node's host is the leaf holding its statement marker, derived at
    load; nothing about hosting is stored. Every hosted node has exactly one statement marker: the
    build always writes it, single-claim leaves included; the converter inserts a marker where a
    2.0.x `claims:` listing implies one and none stands, and where a claim carries markers in more
    than one leaf it keeps the one in the leaf that lists the claim, removes the others and warns.
    A leaf that lists a claim without its marker is a mention. A hosted node with no statement
    marker is an ordinary `verify` fault and does not make a load degraded.
15. **`rigor` and `bearing` are optional on inserts and `add-rests-on`;** an omitted value is
    written `*pending*` with `manually-scored: false`.
16. **Degraded load.** The load is strict about syntax — a file that does not parse refuses, naming
    file and line — and lenient about integrity: a duplicate edge identity, a dangling edge and a
    cycle load, resolved for the walk, and mark the load degraded. Degraded is computed on every
    load and never stored. While degraded: `verify` works and reports the degraded state, each fault
    and its remedy; every query, `score`, a `build` resuming a trail and every write op is refused,
    naming the state and `refresh`; `refresh` repairs — a duplicate identity keeps the higher bearing,
    a dangling edge is dropped, a cycle is broken — warns of each loss, and writes clean files.
    Merging metadata across branches is unsupported; this is how a merge's damage is dug out of.
17. **Cycle breaking cuts as the walk finds cycles, never the whole ring.** In `build` and in
    `refresh` alike: walk the graph depth-first in edge-identity order; each time the walk closes a
    cycle, demote one edge of it — a `supports` edge on the cycle going into an analysis where one
    stands, the first in identity order, else the edge the walk is on — and continue until no cycle
    remains. A merge may leave many cycles; each cut warns, naming the edge and the cycle it broke.
    A demoted edge keeps the edge it replaced (Settled 2), so `resolve-demoted` restores it exactly
    — its relation, bearing and `manually-scored` — and the repair of one cycle is at most two
    edits: restore the demoted edge, re-author another edge of the cycle.
18. **Stale generated pages** are a fault `refresh` fixes. **Sessions** may read a generated
    `claim-rigor.md` as prose, take no value from it, and never edit it.
19. **No origin.** Whether the build found an edge unmarked has no effect on the KB, so no record
    carries it and no surface reports it: the sheet draws one style for `supports` and one for a
    demoted edge, with no cited or inferred variant in its strokes, tooltips, legend or digest, and
    no longer reads the build records; `verify`'s `demoted` finding names the edge alone. The
    converter drops 2.0.x's `(origin …)` annotations without a warning. The build records keep what
    they hold for the build's own stages; a later report could reconstruct the fact from them.
20. **KB format 3.0.0, two authored files.** ROADMAP's "the derived index as one file" retires;
    jointly established units move to the next major.
21. **Register page placement is derived.** A node is rendered on the register page of the volume its
    host leaf lies in; no record carries a register. The converter drops 2.0.x placement silently;
    a node printed in another volume's 2.0.x register moves to its host's page, and a prose link to
    its old anchor is reported by the dead-link check.

## Established facts

| Fact | Command | Answer |
|---|---|---|
| Current format version | `git grep -n 'FormatVersion *=' internal/kb/format.go` | `2.0.1` |
| Format version rules | SPEC.md §10 "The stamp", "One version read and written" | major: an old reader misreads; a newer major or minor is refused by every KB-reading verb; migration at load, landing on the next write |
| Today's `.index/` files | `ls /Users/agent-user/projects/adjagent/.claude-temp/mc-fixture/kb-root/.index/` | `cites.yaml`, `claims.yaml`, `edges.yaml`, `strengthen-by.yaml`, `subtree-aggregates.yaml` — all derived |
| `.index/` is git-tracked | `git ls-files kb-root/.index` in `/Users/agent-user/projects/adjagent/.claude-temp/mc-fixture` | yes |
| Registers in the ModernCorp KB | `find /Users/agent-user/projects/adjagent/.claude-temp/mc-fixture/kb-root -name claim-rigor.md` | 7: the root and per-volume sidecars |
| Edge authoring sites today | SPEC.md "Edge direction and authoring" (~1536) | `rests-on` bullets in a register (pull); a register's `references:` and `- demoted:` lists (`demoted` written by the build only); an analysis's and an experiment's pair lists in leaf frontmatter (push); a claim's `refutes:` list (push) |
| Frontmatter keys read by name | `git grep -n -E '(Get\|ListOrEmpty\|Str)\("[a-z-]+"' -- internal cmd ':!*_test.go'` | `claims`, `experiments`, `kind`, `no-claim`, `subtree-claims`, `subtree-experiments`; plus `analysis-nodes`, `experiment-nodes` (`*NodesKey`, `internal/kb/frontmatter.go`) and `kb-format` (SPEC §10) |
| Which kinds and relations `score` walks | `rigorItems`, `bearingItems` in `internal/query/scoring.go` | rigor: every kind but work and axiom; bearing: `supports`, `lifts`, `refutes` |
| A score landing goes through the write ops | SPEC.md §8 "Landing" | a rigor answer becomes `set-rigor`, a bearing answer `set-bearing` |
| The ledger's statuses | `internal/score/ledger.go` status constants | `done`, `failed`, `pending` |
| `mcp_instructions.txt` is plain text | `cmd/mcp.go` near `servedInstructions` | no Markdown reaches the wire |
| Full verification time today | `/usr/bin/time -p kbase verify`, ten runs, from `/Users/agent-user/projects/adjagent/.claude-temp/mc-fixture` | ~0.10 s each |

## Phases

| Phase | Rating | Seat | Status |
|---|---|---|---|
| A — inventory and design | expert | architect | landed |
| B — design review and ratification | expert | go-coder, prompt-engineer; then the owner | reviewed, ruled and folded; awaits ratification |
| C — implementation | per package | per package (below) | planned below; awaits ratification |

### Phase A — inventory and design

The architect writes `KB_3_DATA_MODEL.md` (project root, beside this plan), its own design artifact, holding:

- **The inventory:** every frontmatter key, register line and `.index/` file; for each, every reader
  and writer in the code (package and function — `internal/claimgraph` and `internal/index/verify.go`
  included), and its home: a field of a node or edge record, derived at load, a generated page, kept
  in the entry point's frontmatter (the `kb-format` stamp only), or removed with the reason.
  `claims`, `no-claim`, `experiments`, `kind`, `subtree-claims` and `subtree-experiments` each get an
  explicit home.
- **The two schemas:** each record type's field names, types and order under Settled 2–3's
  required/refused rule, the YAML form, and a worked example of each type; whether `axioms.md` or
  `nodes.yaml` declares an axiom; what `show`'s `canonical_path` becomes.
- **The statement markers:** whether the marker keeps its register-derived name; how an analysis or
  an experiment finds its statement once `CanonicalPath` names no register (ARCHITECTURE, "The closed
  text and its guard"); what a node with no marker reads as.
- **The generated register page:** its path (one per volume, as today), its anchor rule (each
  entry's anchor unchanged from 2.0.x, so prose links and the dead-link check keep working), whether
  git tracks it, what it shows and in what order.
- **The converter:** each 2.0.x source form to its 3.0.0 record, with a recommendation on open
  questions 1 and 2.
- **Every verb's change:** each query, write op, `build` stage, `refresh`, `verify`, `score`,
  `render-claim-graph` and `export-latex`, and the MCP resources.
- **`score`'s skip:** the ledger's new status, whether `--resume` retries a skipped item, and how a
  run's counts report it.
- **The session rule's single source:** where the text lives and how the plain-text and Markdown
  surfaces both take it.
- **The benchmark:** its in-repo fixture and its target's name.
- **The contract:** for each of the following, whether it is kept, amended or retired, with the new
  wording — SPEC §3's byte-equal rows (metadata rendering, the `.index/*.yaml` freshness gate);
  SPEC §3's placeholder `index sha256`; SPEC §3's stamped-documents rule and §11's prohibition,
  both of which call `.index/` derived (`cmd/mcp_instructions.txt`, `AGENTS.tmpl.md`); SPEC §10's
  YAML dialect (one JSON object per index line); the one loader, `internal/kbload`; migration at
  load, landing on the next write; the converter's refusal to make invalid bytes valid, and
  `check: conversion`; `set-bearing` never inserting a token; `refresh` never rewriting a rigor it
  found. Then each other SPEC, ARCHITECTURE and CONVENTIONS section that moves, named.
- **The compat harness** in `internal/query/compat_test.go`, which compares the derived index with
  the reference run: what becomes of it.
- **Recommendations** on open questions 3–5.

Done when every Settled item maps to a named section of the design and every open question carries a
recommendation.

### Phase B — review and ratification

A go-coder reviews the design for feasibility and missing consumers; a prompt-engineer reviews it as
the brief Phase C's coders will execute. The architect folds the findings and Rulings 13–21 into the
design, the body stating each ruling as a rule and the design's open-question and "does not hold"
sections retired into it. The owner ratifies; Phase C's work packages are then planned here from the
ratified design.

### Phase C — implementation

`KB_3_DATA_MODEL.md` is the design and `KB_3_WORK_PACKAGES.md` the dispatch plan: one package per
dispatch, refining C1–C3 below. Each dispatch prompt quotes verbatim, from those two reviewed files:
the work-packages preamble, the package's own section, the rows of the package table, Allowed-red
and retirement tables that name the package, and the design sections the package cites. It names
the four contract documents by path for the coder to read. The coder reads neither file whole; it
follows a cross-reference only where the quoted text does not settle the point, reads only that
section, and names it in its report. A section followed repeatedly is quoted from then on. Where the design
and this plan differ, this plan governs; a coder that finds the design unworkable stops and reports
rather than redesigning. Packages run in dependency order; packages whose predecessors are all
committed and that share no file may run in parallel, each audited and committed on its own.

| WP | Rating | Seat | Lands | Design | Status |
|---|---|---|---|---|---|
| C1 | expert | go-coder, coordinator's model | In-memory derivation under 2.0.1, behaviour-neutral | §3.6 (the `.index/` rows only), §9.1's first sentence, §12 | pending |
| C2 | mechanical | go-coder, sonnet | The converter's 2.0.x readers, golden-tested and not yet wired | §8's copy list; §2 as the source inventory | pending |
| C3a–e | expert, except a sub-package `KB_3_WORK_PACKAGES.md` rates mechanical (settled by quoted text) | go-coder, coordinator's model; a mechanical sub-package go-coder, sonnet | The format flip, one landing | below | pending |

**C1 — in-memory derivation.** `index.Derive` computes from the 2.0.1 sources what `.index/` holds.
`query.Load`, `score`'s scoring set and `kb.SheetDigestDrawnOf` (`internal/build/rows.go`) read it
instead of the files; the sheet receives the rows through `index.WriteSheet` (§9.7), since `index`
imports `sheet`. `refresh` still writes the 2.0.1 index files, and C1 retires nothing §3.6 lists:
`refresh` and `verify` keep the coverage check and the cycle refusal, whose retirement is C3a's.
`just bench-load`, a recipe C1 creates, and `BenchmarkLoadDerive` land here; on 2.0.1 they time the
source parse plus `index.Derive`. The load's leniency (§3.7) is not part of C1. A query over a KB
whose sources `refresh` refuses (a cycle, a coverage fault) now carries that refusal instead of
answering from the last `.index/` written; that is intended, not a regression.
Done: the gates green; `git diff --stat` touches no `testdata/` or `test-data/fixtures/` file;
`git grep -n 'ReadIndex(' -- internal cmd ':!*_test.go'` returns no `query` line; the report gives
`just bench-load`'s ns/op and allocs at both sizes.

**C2 — the 2.0.x readers.** `internal/migrate` gains §8's copy list except the records emitter, which
is C3c's: the 2.0.x register reader, `ParseLeaf`, `parseExperimentLeaf`, `parseAnalysisLeaf`,
`ParseAxioms`, the legacy readers, the slug and id rules and the unknown-id filter. `migrate`
imports nothing of kbase (`cmd/importpolicy_test.go`). Nothing calls the copy yet.
Done: for each `internal/migrate/testdata/*-2.0.0` fixture, a committed golden of the copied readers'
parse (a refusal golden where the 2.0.x load refuses it); the gates green. C2 also records, in the
shared tree with the 2.0.1 binary, the 2.0.1 query goldens §14's parity case compares against.

**C3 — the format flip**, one landing, built in a git worktree. Once C1 and C2 are committed, and the
plan, the design and `KB_3_WORK_PACKAGES.md` are committed, the coordinator runs
`git worktree add .claude-temp/kb3 -b kb-3.0.0 release-0.8.0` and creates
`.claude-temp/kb3/.claude-temp/`. Every C3 dispatch names the absolute worktree path and runs each
command from it (`cd <path> && just …`); the coordinator audits and commits each dispatch on
`kb-3.0.0`. Nothing else lands on `release-0.8.0` while C3 is open. The landing,
`git merge --ff-only kb-3.0.0` in the shared tree, is the owner's act, or the coordinator's under the
owner's go, with no agent dispatched and `git status` clean.

The dispatches run in this order; each converts the fixtures and goldens its packages' tests read
(§15's row for that package), and its report names every package still failing to build.

- **C3a — records, the load and `index`'s own consumers:** `kb.FormatVersion` set to `3.0.0`, and
  every fixture a package converts by hand stamped `3.0.0`; the two schemas, their bytes encoder in
  `kb` and their file write in `index` (§3.1–§3.5); the strict and lenient load and the degraded
  state (§3.7); `index.Derive` over records and markers, and §3.6's retirements (§3.6, §4); register
  page placement and rendering (§5); `index.BreakCycles` (§10); `index`'s `refresh` (non-migration
  path, §9.3) and `verify` (§9.4), since `write` refreshes after every op. A 2.0.x reader in `kb`
  retires with its last caller.
  Done: `internal/kb` and `internal/index` green, with a round-trip test for each §3.2/§3.3 type, one
  test per §3.7 refusal class and degraded row, and §10's walk cutting a fixed multi-cycle graph to
  a stated edge set.
- **C3b — write ops:** every op over records and markers (§9.2); `manually-scored` and its provenance
  (§6.1–§6.2); the new ops; `set-frontmatter` retired; `resolve-demoted`'s exact restore; the ops'
  `Short` and `Long` help (§15). Help text the design does not quote lands after prompt-engineer
  review.
  Done: `internal/write` green, with one test per §6.1 row and §6.2's test that no flag or tool
  property names the provenance.
- **C3d — the remaining consumers:** the `refresh` and `verify` verbs in `cmd` over C3a's `index`;
  `build`'s stages (§9.5); `score`'s skip and `--rescore-manual` (§7, §9.6); the
  queries' 3.0.0 behaviour (§9.1) and §6.3's keys; the sheet, `export-latex`, `init`, `status`
  (§9.7); the MCP instructions (§9.8); the session rule and its test (§11); the stamped templates'
  text (§13.1).
  Done: `cmd`, `query`, `score`, `sheet`, `claimgraph`, `build` and `kbdocs` green, with §11's test.
- **C3c — the converter:** C2 wired in; the mapping, marker placement, merges and warnings (§8); the
  migration save, `refresh`'s migration path included (§8.6, §9.3); the 2→3 step in `migrate.Chain`
  and `kbload`.
  Done: `internal/migrate` and `internal/kbload` green, `register-2.0.0` and `bad-token-2.0.0` as
  2→3 golden pairs, every §8.5 warning class asserted.
- **C3e — the contract and the remaining fixtures:** SPEC, ARCHITECTURE, CONVENTIONS,
  `USER_CONVENTIONS.md`, README and ROADMAP per §13 except §13.1; `test-data/fixtures/*`, §14's
  harnesses and any `cmd/testdata` no earlier dispatch took.
  Done: the gates green; `just bench-load` rerun on the converted fixture, its numbers in the report.

**Release gate:** before a 3.0.0 binary ships (`just dist`), the owner confirms each §16 item landed
in adjagent, or was settled for the `path-stable` question.

## Gates

C1, C2 and C3e: `just checkpoint` and `just test-integration`. Between C3 dispatches: `just fmt-check`
and `just test-pkg <pkg>` for each package the dispatch names green.

## Not in this plan

- The build gap that leaves claims with no statement marker (ModernCorp: 10 claims, among them
  `clm-uu11c9` and `clm-fsig2a`).
- The ModernCorp KB's dead links to `../part2/claim-quality.md`.
- The bearing ask's lone `flt-no-engagement` answers on edges whose source supplies what the
  context claims.

## Post-sprint review

Once the sprint's last landing is committed, one pass reviews the changes since the plan was
activated. The reviewers are an architect and a coder; before they start they read THESIS.md,
SPEC.md, ARCHITECTURE.md and CONVENTIONS.md, and they do not read this plan. Their subject is the
changes against those standards and whether the result is fit for its purpose, and their brief
includes the tests.

**Review**: findings against the four documents. **Remediation**: each finding landed in a follow-up
commit or declined with the reason recorded. **Burndown**: what remains, one fact to an item, below.
