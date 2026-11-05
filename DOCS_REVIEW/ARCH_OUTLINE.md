# ARCHITECTURE rewrite outline

The structure of the rewritten ARCHITECTURE, what each section does, and how six writers produce it
in parallel.

Inputs:
- `ARCHITECTURE.md` (A:): the current text. A: line numbers refer to it.
- `SURVEY-classification.md`: names the standard pattern of every mechanism (its §3 tables).
- `GLOSSARY.md`: the wording rules.
- `THESIS.md`: the building-block table and "What is new".
- `SPEC.md`: the rewritten contract ARCHITECTURE implements and cites.
- The code under `cmd/` and `internal/`: what ARCHITECTURE describes.

## Rules for the whole rewrite

1. **ARCHITECTURE describes the code as it is.**
   - Every mechanism stated is landed. Where ARCHITECTURE and the code disagree, the code settles it;
     report each case with both lines.
   - Cite a SPEC section only for a rule the code meets. Where the code does not yet meet a SPEC rule,
     describe the code without the citation and report the gap.
   - Where SPEC's name differs from the code's identifier, write the code's identifier. For example,
     the code's decision-API key is `decisionsFlavor`, while SPEC says `decisionApi`. SPEC's name
     appears only inside a citation.
2. **Name the pattern first, in a fixed form.**
   - A prose section's first line is `**Pattern:** <string>.` followed by a blank line. The string is
     the one this outline gives for that section, copied verbatim; where the outline gives none, it is
     the survey's Pattern cell.
   - A mechanism within a section opens its paragraph with the same label.
   - Tables (§2 Invariants, §4 Stage table) gain a `Pattern` column instead.
   - After the pattern, state only what kbase does differently from it, or what the pattern leaves
     open, and why. Do not explain the pattern itself.
3. **Novel mechanisms are marked.** The novel set is THESIS's "What is new", realized in:
   - the claim-graph rows `claims-declared`, `claims-discovered` and `analyses-split` (THESIS item 1);
   - the node kinds and strength arithmetic, A:535-586 (items 1 and 3);
   - the fault criteria in the prompt templates, A:697-715 (item 2). The templates, the input guard
     and the reply parse are standard machinery.
   - the bearing predicates, A:801-807 (item 1).

   Such a section opens with `**Novel.** <one sentence: the THESIS item realized and the SPEC section
   that defines it>`, then `**Pattern:** <the machinery that runs it>`. In a table, the Pattern cell
   starts `Novel:`. Nothing else is marked novel.
4. **Departures carry their reason.** Where the survey records a departure with a written reason (R),
   keep the reason, stated once. Where it records none (N), do not invent one: state the departure
   plainly and list it in your report (item 5).
5. **Mechanism only.** Behaviour lives in SPEC. Where ARCHITECTURE restates a SPEC rule the code
   meets, replace the restatement with a citation in the form `SPEC §N <exact SPEC heading>`. Keep one
   clause where the mechanism needs the rule to make sense.
6. **Each mechanism is stated once**, in its owner section: the "Owners" table below, else the section
   the "Outline" names.
   - Other sections link to it with `[§N](#slug)`. The link text is only `§N`, never `[§N Title](…)`.
   - Link across slices only to a heading that exists in A: or is listed in the "Headings" table. A
     heading you add that is not in that table is a link target for your own slice only.
   - Never add a heading whose text already exists in A:.
7. **No history.** No "was", "previously", "since vN", "no longer". Version numbers that are current
   facts stay: pandoc 3.12.1, the MCP protocol revision, the store format.
8. **Wording per the glossary.** Identifiers stay (glossary rule 1). Package, type, function and file
   names are interface here and stay exactly. No heading is renamed except as the "Headings" table
   says; glossary rows apply to prose only.
9. **Long table cells become prose or lists.** A cell over about 60 words gets a one-line summary and
   a link in the table, with the rest written as a subsection or list under it. This covers invariant
   I9 (A:32) and the `kb` and Write API rows (A:431-432).
10. **Before returning, grep your part** for every glossary section A house term and every section B
    term. Report each hit with its sense and disposition.

## Owners: mechanisms stated in more than one place

| Mechanism | Owner (slice) | Slug | Other A: ranges: become a link |
|---|---|---|---|
| The KB write lock and the run lock | §8 Locks (5) | `#locks` | A:432 tail, A:1375-1407 (moves into Locks), A:2078, scoring's landing |
| Every numeric value (attempt counts, waits, retention counts, concurrency, slot counts) | §12 Constants (6) | `#12-constants` | A:1149, A:2067, A:2080-2082, §6 Run retention, §8, and every other inline value: name the constant and link |
| Cycle breaking | §6 Cycle breaking (3) | `#cycle-breaking` | A:81, A:433 `BreakCycles`, A:1908-1915 |
| Format migration | §12 Format migration (6) | `#format-migration-spec-10` | I8, A:98 `phase-3a`, A:433 `UpdateFormat`, §11 Loading |
| The MCP tool table and the detached build/score child | §12 MCP server (6) | `#mcp-server-spec-11` | I9 (one line plus a link, no list), §6 The verb (A:966-1005) |
| Per-interface rendering (`argmark`, `Spelled`, `Verbatim`) | §9 Per-interface rendering (5) | `#per-interface-rendering` | A:1569-1580, A:1618-1642 |
| Credentials store | §7 Credentials (5) | `#credentials` | A:1215-1240, A:1773-1785 |
| The `kb` package's responsibilities | §6 The `kb` package (3) | `#the-kb-package` | A:431 row, A:1648-1678 |
| The build's writes and the build-only ops | §6 Build-only ops (3) | `#build-only-ops` | A:512-533 (moves here), A:98 |

## Headings

New or moved headings that other slices may link to. Use exactly this text and level.

| Heading | Level and position | Slug | Slice |
|---|---|---|---|
| `### Pattern map` | §1, after the Purpose text | `#pattern-map` | 1 |
| `` ### The `kb` package `` | §6, first subsection | `#the-kb-package` | 3 |
| `### Build-only ops` | §6, after Provenance | `#build-only-ops` | 3 |
| `### Cycle breaking` | §6, after The derived layer | `#cycle-breaking` | 3 |
| `#### Bearing predicates` | §6 Scoring, after The scoring set; holds A:801-807 | `#bearing-predicates` | 4 |
| `#### Writing the results` | §6 Scoring; replaces `#### The one-op landing` | `#writing-the-results` | 4 |
| `### Credentials` | §7 | `#credentials` | 5 |
| `### Locks` | §8 | `#locks` | 5 |
| `### Per-interface rendering` | §9 | `#per-interface-rendering` | 5 |

Slice 5 may add further `###` headings to §7 and §8, which currently have none. They are link targets
for its own slice only.

## Outline

Each entry gives the pattern string to use verbatim (rule 2), then what changes.

### Head (A:1-10) and §1 Purpose (A:11-17) — slice 1
- Keep the Must-see. Rewrite Purpose to say that kbase is assembled from standard patterns at small
  scale, that the novel parts are THESIS's three, and that each section names its pattern.
- **`### Pattern map`**: one row per `##` and `###` heading of this document, with the section, a link
  and its pattern string. A novel section's row starts `Novel:`. Copy the strings verbatim from this
  outline (rule 2). Where a THESIS building-blocks row covers a section, the row cites that THESIS row
  by name. Drop the old Status column. The coordinator diffs the map against each section's opening at
  merge.

### §2 Invariants (A:18-38) — slice 1
- Add a Pattern column using the survey §3a strings. I1: "One adapter per external process, enforced
  by a fitness test". I12: "Counting semaphore with a capability token". The rest are as the survey
  gives them.
- I9 becomes one line plus `[§12](#mcp-server-spec-11)`.
- I6: cite `SPEC §8 Idempotence`.

### §3 Principles (A:40-66) — slice 1
- Open each principle with its pattern:
  - "Generate-then-validate"
  - "Constrained output"
  - "Graceful degradation to a deterministic baseline"
  - "Hermetic, deterministic build"
  - "Reuse an existing parser"
  - "Self-describing API"

### §4 Stage table (A:67-104) — slice 1
- **Pattern:** "Checkpoint/restart pipeline".
- Add a Pattern column:
  - `references-found`: "Candidate generation then pairwise classification"
  - `claims-culled`: "Near-duplicate clustering with a canonical representative"
  - `depends-attributed`: "Pairwise relation classification, then cycle removal"
  - `claims-declared`, `claims-discovered` and `analyses-split`: `Novel:` (rule 3)
- Cycle removal links `[§6](#cycle-breaking)`. The build's writes link `[§6](#build-only-ops)`.

### §5 Reader and transforms (A:105-426) — slice 2
- **Pattern:** "Pandoc as the parser; Go transforms over its JSON AST".
- §5.1: "Wrapped external tool with a version gate".
- §5.2: "Preprocessor registry with instrumentation".
- §5.3: "Pandoc AST filter".
- §5.5: "Sidecar metadata stream".
- §5.6: "Round-trip conformance checks and a link checker".
- §5.7: "Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked
  output directory". Keep the pandoc 3.12.1 workarounds with their reasons. Cite SPEC §8
  `export-latex` in place of restated behaviour.

### §6 KB model and maintenance (A:427-654) — slice 3
- **`### The kb package`**: "Domain model and schema package". Absorbs the A:431 row.
- **Write API**: "All-or-none validated batch with dry run and per-file atomic writes". The A:432 cell becomes prose.
  - Per-file atomic rename, and the cross-file limit, stated once.
  - Cite `SPEC §8 Write batches` at A:443 and A:471, and `SPEC §8 Writes leave nothing stale` at
    A:475.
  - Locks link `[§8](#locks)`.
- **Provenance**: "Manual-override flag".
- **`### Build-only ops`**: "Internal privileged ops". Holds A:512-533 and the build's writes.
- **The node vocabulary**: `Novel:` (rule 3), then "Prefixed random ids". Cite `SPEC §10 Five node
  types` at A:541.
- **The derived layer**: `Novel:` for the strength arithmetic, then "Topological fold over a DAG
  (Kahn's order); materialized views rebuilt after each write".
- **`### Cycle breaking`**: "Greedy feedback-arc removal by DFS back edges".
- **The query layer**: "Read model".
- **Frontmatter reading**: "Heuristic sniffing before parse".
- **Sheet**: "Graphviz rendering with a bounded process pool; a materialized view".
- **Scratch creation**: "Scaffold command".

### §6 Scoring (A:655-1081) — slice 4
- **The runner**: "Bounded worker pool over a job list, results written in one batch at the end".
- **The ask is one seam**: "Dependency injection".
- **The prompt shelf**: `Novel:` for the fault criteria only, then "Prompt templates with a lint test".
- **The closed text and its guard**: "Line-numbered context for grounded citation; delimiter-based
  prompt-injection guard".
- **The parse resolves the numbered lines**: "Structured-output validation against a fixed grammar".
- **The scoring set**: "Input-hash staleness, as in make".
- **`#### Bearing predicates`**: `Novel:` (rule 3). Holds A:801-807.
- **The ledger and the state directory**: "Append-only job log with resume".
- **The captures**: "Request/response capture for replay".
- **Run retention**: "Keep-last-N retention". Values link to §12.
- **`#### Writing the results`**: "Optimistic concurrency with partial-batch retry". Locks link
  `[§8](#locks)`.
- **The re-price**: "Projection rebuild from stored evidence".
- **The run's entry**: "Command orchestration".
- **The verb**: link the detached child to `[§12](#mcp-server-spec-11)`.
- **The development replay**: "Offline prompt-regression harness".

### §7 Inference seams (A:1082-1320) — slice 5
- **Pattern:** "Role-based model routing behind provider adapters".
- Within §7:
  - "Retry with backoff on retryable status"
  - "Typed error classification at the boundary"
  - "Layered configuration"
  - "Factory registry with lazy, shared clients"
  - "Constrained classification endpoint" (the decision API, code identifier `decisionsFlavor`)
  - "Dependency injection with an architecture fitness test"
- **`### Credentials`**: "Local secrets file with a redacting secret type".

### §8 Ledger, state store and monitoring (A:1321-1581) — slice 5
- **Pattern:** "Checkpoint commits, a content-addressed answer cache, and a background job with a
  status file".
- Within §8:
  - "Keep-last-N retention"
  - "Versioned on-disk schema with sequential migrations" (the state store format)
  - "Background job with a status file, signal cancel and a polling wait"
- **`### Locks`**: "Repo-scoped mutex lockfile plus a pidfile-style holder record". It is the run lock,
  and the KB write lock is an "Advisory file lock with timeout". This is the owner for both.

### §9 Result emitter (A:1582-1591) — slice 5
- **Pattern:** "Single structured-output writer".
- **`### Per-interface rendering`**: "Per-surface presentation layer". Owner of `argmark`, `Spelled`
  and `Verbatim`.

### §10 Compatibility harness (A:1592-1609) — slice 5
- **Pattern:** "Black-box integration tests with golden fixtures".

### §11 Package map (A:1610-2058) — slice 6
- Rewrite the fenced listing (A:1615-1873) as a table: `Package | Responsibility | Pattern |
  Mechanism`. The Mechanism column holds a `[§N](#slug)` link. Mechanism prose in the listing moves to
  its owner, or is cut where the owner already states it.
- **Loading**: "Schema validation at load, fail-closed". Cite `SPEC §10 The load: every fault
  refused` at A:1906.
- **Dependency rules**: "Architecture fitness functions". Keep the rules.

### §12 Constants (A:2059-2351) — slice 6
- **Pattern:** "Named constants registry". Owner of every numeric value.
- **Format migration**: "Chained schema migrations (upcasters) with golden tests". Owner of migration.
  Cite `SPEC §8 update-format` at A:2147.
- **MCP server**: "MCP server generated from the command tree; JSON-RPC over stdio; detached child plus
  status polling". Owner of the tool table and the detached child.

### §13 Slice verdicts (A:2352-2382) and §14 Limits (A:2383-2422) — slice 6
- Glossary pass only.

## Mechanism moved in from SPEC

The SPEC rewrite removed these as mechanism. Each must be stated in ARCHITECTURE, in the section named,
where the code does it. Where it is already stated, report "already stated at ⟨section⟩". Where the
code does not do it, report the gap under item 4.

| Mechanism | Section |
|---|---|
| Every KB file is read through one loader, `internal/kbload`, which serves migrated bytes. Nothing is cached between loads | §11 Loading |
| A conversion step is bytes in, bytes out, plus obsolete paths | §12 Format migration |
| The per-op key vocabulary lives in the write package | §6 Write API |
| A batch reads the whole KB into memory, applies every entry in memory, rebuilds the derived views, then writes each changed file once, temp file then rename | §6 Write API |
| An op is split by node kind exactly when its fields differ by kind | §6 Write API |
| `init` runs no `verify` of its own, every skeleton input being kbase's own | §6 Scratch creation |
| The build's writes go through `add-claim`, `add-work` and `remove-node`, and take no KB write lock | §6 Build-only ops |
| The build-only ops `add-build-edges`, `demote-claim-edge` and `split-claim` | §6 Build-only ops |
| A scoring run reads a node's title passage through its canonical path | §6 The closed text and its guard |
| The rigor and bearing tables are `internal/kb`'s | §6 The parse resolves the numbered lines |
| `contribution` reuses the scoring code, never a restated formula | §6 The query layer |
| The transport attempt count is a constant | §12 Constants |
| A config write is temp file then rename; `show-config` opens no client | §7 |
| An unanswered decision question takes the multichoice seam's retry-then-default path | §7 |
| The sheet writer's test writes one DOT twice and asserts identical bytes | §6 Sheet |
| Which stages `--no-inference` drops follows each step's classification, never a stage id | §4 |
| The MCP tool table: one tool per family and per other exposed command, derived from command declarations and asserted by test; a command's read-only annotation comes from its definition; a tool's input schema derives from its command's flags; the protocol revision is one constant | §12 MCP server |
| Each command stores four help texts; a test asserts each rendering is exactly its parts; `measure-mcp-surface` tracks the always-read size | §12 MCP server |
| Golden-tree conformance fixtures under `internal/formats/testdata/` and `internal/sheet/testdata/` | §10 |

## Dispatch

Six writer agents. Each writes its part file under `DOCS_REVIEW/arch-parts/`: the complete final text
of its slice, at current heading levels, with no title of its own and no notes to the coordinator.

| Slice | File | Sections | Source |
|---|---|---|---|
| 1 | `1-purpose.md` | Head, §1 (with the Pattern map), §2, §3, §4 | A:1-104 |
| 2 | `2-reader-and-transforms.md` | §5 | A:105-426 |
| 3 | `3-kb-model-and-maintenance.md` | §6 through Scratch creation | A:427-654 |
| 4 | `4-scoring.md` | §6 Scoring | A:655-1081 |
| 5 | `5-inference-seams.md` | §7, §8, §9, §10 | A:1082-1609 |
| 6 | `6-package-map.md` | §11, §12, §13, §14 | A:1610-2422 |

**Report.** Each writer returns:
1. Every A: range in its slice, with its disposition: kept, rewritten, moved to ⟨section⟩, cited to
   SPEC, or removed as behaviour.
2. Each row of the moved-in table, and each Owners row, in its slice: where stated.
3. Code conflicts (rule 1), with both lines.
4. SPEC rules the code does not yet meet, found while checking.
5. Departures with no stated reason (rule 4).
6. Headings added, with slugs.
7. The rule 10 grep results, plus glossary sites under glossary rules 6 and 7.
8. A: ranges whose claims you did not check against the code.

**Merge.** The coordinator:
1. Concatenates the parts into `DOCS_REVIEW/ARCHITECTURE.new.md`.
2. Checks the reports against each other and against the Owners table.
3. Diffs the Pattern map against the section openings.
4. Greps each part's backticked identifiers in `cmd/` and `internal/`.
5. Runs a glossary pass.
6. Dices the file into `.claude-temp/` to check links.
7. Runs a coverage check: every mechanism in the old text and in the moved-in table is in the new.
8. Runs a drift probe.

Only then does `ARCHITECTURE.new.md` replace `ARCHITECTURE.md`.
