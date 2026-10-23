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
   - `DemotedEdge` — `demoted`: those, plus `origin` (`cited` or `inferred`);
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

## Open questions for ratification

Phase A recommends an answer to each; the owner rules in Phase B.

1. **An edge identity authored at more than one site in a 2.0.x KB.** Today the load accepts it and
   the last in load order answers (`Index.Edge`, `internal/query/scoring.go`). The converter
   refuses, merges where bearing and context agree, or keeps the last.
2. **Register text that fits no field** — intro paragraphs, legend tables, `> **Leaf references:**`
   lines (ModernCorp: `part2/claim-rigor.md`). The converter drops it, refuses with `check:
   conversion` (SPEC §10), or carries it somewhere.
3. **A stale generated page:** `verify` reports it as a fault, as a finding, or not at all.
4. **Session access to generated pages:** whether a session may read a generated `claim-rigor.md`.
5. **The version number.** ROADMAP assigns `3.0.0` to "the derived index as one file" (which this
   plan supersedes) and to jointly established units; units move to the next major.

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
| A — inventory and design | expert | architect | pending |
| B — design review and ratification | expert | go-coder, prompt-engineer; then the owner | pending |
| C — implementation | planned from the ratified design | — | not yet planned |

### Phase A — inventory and design

The architect writes `ROADMAP_PLANS/KB_3_DATA_MODEL.md`, its own design artifact, holding:

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
the brief Phase C's coders will execute. The architect folds the findings in. The owner rules on the
open questions and ratifies; Phase C's work packages are then planned here from the ratified design.

## Gates

Phase C, per package: `just edit-gate`, `just test`; before each landing `just test-integration`
and `just checkpoint`.

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
