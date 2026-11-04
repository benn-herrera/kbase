# ARCHITECTURE rewrite outline

The structure of the rewritten ARCHITECTURE, what each section does, and how six writers produce it
in parallel.

Inputs:
- `ARCHITECTURE.md` (A:), the current text; its line numbers are what A: refers to.
- `SURVEY-classification.md`, which names the standard pattern of every mechanism and marks the novel
  ones (its §3 tables).
- `GLOSSARY.md`, the wording rules.
- `THESIS.md`, the building-block table and the novel core.
- `SPEC.md`, the rewritten contract ARCHITECTURE implements and cites.
- The code under `cmd/` and `internal/`, which is what ARCHITECTURE describes.

## Rules for the whole rewrite

1. **ARCHITECTURE describes the code as it is.** Every mechanism stated is landed. Where SPEC is ahead
   of the code (a SPEC rule the code does not yet meet), ARCHITECTURE describes the code and does not
   claim the SPEC rule; list the gap in your report. Where ARCHITECTURE and the code disagree, the code
   settles it; report each case with both lines.
2. **Name the pattern first.** Each section, and each mechanism within a section, opens by naming the
   standard pattern it implements, in a working engineer's terms (the classification survey's
   "Pattern" column). Then state only what kbase does differently from that pattern, or what the
   pattern leaves open, and why. Do not explain the pattern itself: the reader knows what a
   transactional batch, a counting semaphore or a schema migration is.
3. **Novel mechanisms are marked.** The survey marks four areas novel or mixed: the claim-graph stages
   (A:76-82), the node kinds and strength arithmetic (A:535-586), the scoring prompts, input guard and
   reply parse (A:697-768), and the relation predicates (A:801-807). Each such section opens with the
   line `**Novel.**` followed by one sentence saying what is new and which SPEC section defines it.
   Within it, the standard machinery that runs it is still named by its pattern.
4. **Departures carry their reason.** Where the survey records a departure from the standard pattern
   with a written reason (R), keep the reason, stated once. Where it records none (N), do not invent
   one: state the departure plainly and list it in your report as "departure, no stated reason".
5. **Mechanism only.** Behaviour lives in SPEC. Where ARCHITECTURE restates a SPEC rule, replace the
   restatement with a citation, `SPEC §N <section title>`, unless the mechanism needs the rule to make
   sense; then cite and keep one clause.
6. **Each mechanism is stated once**, in the section this outline assigns it. Other sections link to
   it with `[§N](#slug)` (link text only `§N`, never `[§N Title](…)`).
7. **No history.** Present tense. Versions appear only in the format-migration constants and §13.
8. **Wording per the glossary.** Identifiers stay (rule 1 of the glossary): package, type, function
   and file names are interface here and are kept exactly.
9. **Long table cells become prose or lists.** A table cell over about 60 words (invariant I9 at A:32,
   the `kb` and Write API rows at A:431-432) is rewritten as a subsection or a list under the table;
   the table keeps a one-line summary and a link.
10. **Section numbers and headings stay**, except where this outline says otherwise. List any renamed
    heading's old and new slug in your report.

## Outline

### Head (A:1-10) and §1 Purpose (A:11-17)
- Keep the Must-see. Rewrite Purpose to state plainly: kbase is assembled from standard patterns at
  small scale, the novel parts are THESIS's three, and this document names each mechanism's pattern.
- **New: §1 "Pattern map"**, after Purpose: one table, one row per section of this document — the
  section, the standard pattern(s) it implements, and `Novel` where rule 3 applies. Built from the
  classification survey. This is the reader's index.

### §2 Invariants (A:18-38)
- Keep I1-I12. Each row names its pattern (survey §3a). I9's cell becomes a short row and a list
  under the table (rule 9). Fix I6's dead link: cite SPEC §8 Write batches (Idempotence).

### §3 Principles (A:40-66)
- Keep. Name each principle's pattern (survey §3a: generate-then-validate, constrained output,
  graceful degradation, hermetic build, reuse an existing parser, self-describing API).

### §4 Stage table (A:67-104)
- Checkpoint/restart pipeline (survey §3b). Mark `claims-declared`, `claims-discovered`,
  `references-found`, `claims-culled`, `depends-attributed`, `analyses-split` rows per rule 3 where
  mixed; name the standard technique of each (blocking plus pairwise classification, near-duplicate
  clustering, relation classification, segmentation).

### §5 Reader and transforms (A:105-426)
- Pandoc seam, pre-pass registry, AST filter, sidecar records, conformance checks: name patterns
  (survey §3c). §5.7 LaTeX export: "pandoc multi-file conversion with a Lua filter and a custom
  template; manifest-owned output directory" — keep the pandoc-bug workarounds with their reasons;
  trim restatement of SPEC §8 `export-latex`.

### §6 KB model and maintenance (A:427-654)
- **Write API**: transactional unit of work with dry run (survey §3d, §3h). Rewrite the A:432 cell
  as prose. State the per-file atomic rename and the cross-file limit once. Fix the dead links at
  A:443, A:471, A:475: cite SPEC §8 Write batches.
- **Node vocabulary and derived layer**: novel per rule 3 (A:535-586); the strength walk is a
  topological fold over a DAG (Kahn's order). Fix A:541: cite SPEC §10 Five node types.
- Query layer (read model), frontmatter sniffing, sheet (Graphviz, bounded process pool, materialized
  view), scratch creation (scaffold): name patterns.

### §6 Scoring (A:655-1081)
- Runner: bounded worker pool over a job list, batch commit at the end. Ledger: append-only job log
  with resume. Landing ("the one-op landing", heading becomes "Writing the results"): optimistic
  concurrency plus partial-batch retry. Re-price: projection rebuild from stored evidence. Captures:
  request/response capture for replay. Retention: keep-last-N.
- Prompt shelf, closed text and guard, the parse: novel per rule 3 (A:697-768); name the standard
  machinery (templates plus lint; delimiter-based injection guard with line-numbered grounding;
  structured-output validation).
- Scoring set (A:770-807): input-hash staleness like make; relation predicates novel per rule 3.

### §7 Inference seams (A:1082-1320)
- Role-based model routing, provider adapters, retry with backoff, typed error classification,
  layered configuration, local secrets file with a redacting type, client registry, constrained
  classification endpoint (the decision API), dependency injection with a fitness test. The code
  still names the decision setting `decisionsFlavor`; describe the code (rule 1).

### §8 Ledger, state store and monitoring (A:1321-1581)
- Checkpoint commits, content-addressed answer cache, retention, versioned state store, background
  job with status file and polling wait, pidfile lock ("the run lock"; glossary). One lock section,
  stated once.

### §9 Result emitter (A:1582-1591), §10 Compatibility harness (A:1592-1609)
- Single structured-output writer; black-box integration tests with golden fixtures.

### §11 Package map (A:1610-2058)
- Each package entry: its single responsibility, its pattern, and a link to the section that states
  its mechanism. Entries that restate mechanism (the `kb` entry, A:1648-1678) are cut to that.
- Loading: schema validation at load, fail-closed. Fix A:1906: cite SPEC §10 The load.
- Dependency rules: architecture fitness functions (name the pattern; keep the rules).

### §12 Constants (A:2059-2351)
- Keep the tables. Format migration: chained schema migrations (upcasters) with golden tests; fix
  A:2147: cite SPEC §8 `update-format`. MCP server: generated from the command tree; JSON-RPC over
  stdio; detached child plus status polling.

### §13 Slice verdicts (A:2352-2382), §14 Limits (A:2383-2422)
- Glossary pass only.

## Mechanism moved in from SPEC

The SPEC rewrite removed these sentences as mechanism. Each must be stated in ARCHITECTURE, in the
section named, unless it already is (then report "already stated at ⟨section⟩"):

| Mechanism | Section |
|---|---|
| Every KB file is read through one loader, `internal/kbload`, which serves migrated bytes; nothing is cached between loads | §11 Loading |
| A conversion step is bytes in, bytes out, plus obsolete paths | §12 Format migration |
| The per-op key vocabulary lives in the write package | §6 Write API |
| A batch reads the whole KB into memory, applies every entry in memory, rebuilds the derived views, then writes each changed file once, temp file then rename | §6 Write API |
| An op is split by node kind exactly when its fields differ by kind | §6 Write API |
| `init` runs no `verify` of its own, every skeleton input being kbase's own | §6 Scratch creation |
| The build's writes go through `add-claim`, `add-work` and `remove-node`; its writes take no KB write lock | §4 / §6 Write API |
| The build-only ops `add-build-edges`, `demote-claim-edge`, `split-claim` | §6 Write API (build-only ops) |
| A scoring run reads a node's title passage through its canonical path | §6 Scoring |
| The rigor and bearing tables are `internal/kb`'s | §6 Scoring |
| `contribution` reuses the scoring code, never a restated formula | §6 The query layer |
| The transport attempt count is a constant | §12 Constants |
| A config write is temp file then rename; `show-config` opens no client | §7 |
| An unanswered decision question goes through the multichoice seam's retry-then-default path | §7 |
| The sheet writer's test writes one DOT twice and asserts identical bytes | §6 Sheet |
| Which stages `--no-inference` drops follows each step's classification, never a stage id | §4 |
| The MCP tool table: one tool per family and per other exposed command, derived from command declarations, asserted by test; a command's read-only annotation comes from its definition; a tool's input schema derives from its command's flags; the protocol revision is one constant | §12 MCP server |
| Each command stores four help texts; a test asserts each rendering is exactly its parts; `measure-mcp-surface` tracks the always-read size | §12 MCP server |
| Golden-tree conformance fixtures under `internal/formats/testdata/` and `internal/sheet/testdata/` | §10 |

## Dispatch

Six writer agents. Each writes `DOCS_REVIEW/arch-parts/<n>-<first-section-slug>.md`: the complete
final text of its slice, current heading levels, no title of its own, no notes to the coordinator.

| Slice | Sections | Source |
|---|---|---|
| 1 | Head, §1 (with the Pattern map), §2, §3, §4 | A:1-104 |
| 2 | §5 | A:105-426 |
| 3 | §6 through Scratch creation | A:427-654 |
| 4 | §6 Scoring | A:655-1081 |
| 5 | §7, §8, §9, §10 | A:1082-1609 |
| 6 | §11, §12, §13, §14 | A:1610-2422 |

Slice 1 builds the Pattern map from the outline and the survey, not from the other slices' parts.

**Report.** Each writer returns:
1. Every A: range in its slice, with its disposition (kept, rewritten, moved to ⟨section⟩, cited to
   SPEC, removed as behaviour).
2. Mechanism from the table above: where stated.
3. Code conflicts (rule 1), with both lines.
4. SPEC rules the code does not yet meet, found while checking.
5. Departures with no stated reason (rule 4).
6. Headings added or renamed, with slugs.
7. Glossary sites under glossary rules 6 and 7.

**Merge.** The coordinator concatenates the parts into `DOCS_REVIEW/ARCHITECTURE.new.md`, checks the
reports against each other, runs a glossary pass, dices it into scratch to check links, then a
coverage check (every mechanism in the old text and the table above is in the new) and a drift probe,
before it replaces `ARCHITECTURE.md`.
