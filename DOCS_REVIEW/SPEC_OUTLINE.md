# SPEC rewrite outline

The structure of the rewritten SPEC, and what each section takes in. Inputs:
- `SPEC.md` (S:)
- `reference/NEW_MCP_CLI_DESIGN.md` (D:), the retired design
- `SURVEY-rehoming.md`, which maps every cited design section to its home
- `GLOSSARY.md`, the wording rules
- `THESIS.md`, which states which pieces are standard

## Rules for the whole rewrite

- **SPEC states every rule itself.** No section delegates to the retired design ("is page §X's",
  "What this release adds") or to §12. Each rule from either source is stated in the section that
  owns it.
- **Standard behaviour is named in standard terms**, per the glossary. For example: writes are
  idempotent; a batch is all-or-nothing; formats migrate on read and are saved by an explicit
  command.
- **SPEC states the operations in full**, keys included. "The per-op key vocabulary … is not copied
  into prose" (S:695-696) is removed. How the code keeps the key list, and the test that checks it
  against SPEC, belong to ARCHITECTURE.
- **Mechanism moves out.** Mechanism found in SPEC or the design goes to ARCHITECTURE. That covers
  in-memory application, temp-then-rename, stored help parts and the like (rehoming survey, "Class"
  column).
- **Section numbers stay as they are**, §4's gap included. ARCHITECTURE and CONVENTIONS cite them.
- **§12 is removed** once its rules are lifted.

## Outline

Each row: section → what changes. "Lift" names the content moving in, and its source lines.

### §1 What kbase is
- Plain statement of what kbase does, in standard terms consistent with THESIS's building blocks.

### §2 Given interfaces
- No structural change.

### §3 Compatibility contract
- **Stamped documents**: lift the maintained-block rules (D:240-257; §12 S:2866-2870). Define the term
  "maintained block" here once (glossary C).
- **`claim-graph.svg`**: define "sheet" once (glossary C).

### §5 Build behaviour
- **Starting over**: replace "are page §3.5's" (S:202-204) with the full list of `build`'s refusals,
  collected from S:179-187, S:220-260, S:441-446 and S:2007. Lift title uniqueness (§12 S:3045).
- **Co-linearity cull**: define cull once (glossary C: merge, union-find root).

### §6 Build state, ledger and monitoring
- **`kbase status`**: lift the build `state` vocabulary (D:1145). Lift the refusals (D:1138-1160;
  S:2661).
- **`kbase wait`**: lift "`--pid` is required and positive", and its refusal (D:1175-1176).
- **New: `kbase build-trail`**, after `wait`. Result keys and refusals (D:1177-1192). Move the trail
  rules here from S:205-207 and S:2687-2693.
- §8's "`status`, `cancel`" stub (S:1526) merges here.

### §7 Tool-result contract and exit codes
- **Item**:
  - define every check value (D:498-517)
  - add `agents-redirect` to S:600's list
  - keep the values the design lacks (`include`, `metadata-key`, `inputs`, `pandoc`, `export-dir`,
    `write`)
- **Result-key table**:
  - add a `build-trail` row
  - add `warnings` to the `configure-providers`, `configure-models` and `models` rows
- **Bounded results**: lift the sorting rules (D:527-531: ascending by default, every key both ways,
  pending below every number, null last) and negative `limit` as a slice (§12 S:2963).

### §8 Maintenance subcommands
- **Opening**: remove "`probe` excepted" (S:686-688). `probe` is a tool.
- **New: Write batches**, first subsection. It absorbs Values input, Idempotence, Writes leave
  nothing stale, One write lock and Write ops (S:691-786) as its own subsections. Contents, from the
  rehoming survey §4 item 1:
  - batch shape and the two interfaces
  - order, with each entry checked against the state the earlier ones left
  - no reference to an id minted in the same batch; minted ids returned per entry
  - every shape error reported at once, then apply, stopping at the first refusal
  - all-or-nothing, rebuild of derived views included, with the stated limit that a crash between
    two files' writes is recovered through git
  - idempotence and existence: create fails if the item exists; delete or edit fails if it is
    absent; an edit that changes nothing is `unchanged`; lookups are exact
  - value domains
  - dry run equals a real run
  - the `commit` flow
  - locks

  Sources: D:325-415 and D:553-571; §12 S:2812-2829 and S:2878-2879.
- **`add-<kind>`**:
  - lift each kind's key table and refusals (D:635-692)
  - lift the citation-key grammar for a work's label (D:57-58)
  - lift "a label is never an id" (D:60-61)
- **`edit-<kind>`**:
  - lift the editable-field table per kind (D:712-718)
  - lift the full re-pointing and label-change rules (D:696-729)
- **Edge ops**:
  - lift the keys of `add-edge`, `edit-edge` and `remove-edge` (D:789-835)
  - lift "`remove-edge` leaves every other record byte for byte" (D:829-830)
  - lift the refusals (§12 S:2912-2921)
- **`restore-demoted`**:
  - lift "relation must be `demoted`" (D:841, D:845)
  - lift exact restoration of relation, context, bearing and flag (§12 S:2803-2805)
- **`remove-node`**: lift its core effect (§12 S:2927-2930).
- **`move-node`**: lift its keys (D:766-780).
- **`add-citation`**:
  - lift its keys and the output form of each of the three citation forms (D:860-874)
  - lift "snippet occurs exactly once on the line" (§12 S:2946)
- **`find`**: lift the filter semantics and sort keys (D:917-935; §12 S:2957-2965).
- **`show`**: lift the `summary` form `<page>#<slug>` (§12 S:2967).
- **`edges`**: lift the arguments and the base order (D:962-981; §12 S:2973-2979).
- **`trace`**:
  - lift the `contribution` sort, where a `refutes` edge's contribution is its cap (D:990-994)
  - lift `*pending*` versus null (D:1000-1001)
  - lift "never follows `references` or `demoted`" (D:985-986)
  - the "reuses scoring code" line goes to ARCHITECTURE
- **`score`**:
  - lift the default scope (D:1092-1096)
  - lift what `--rescore-manual` reaches (§12 S:3001-3003)
  - lift the base refusal list (D:1122-1135)
  - lift "the scope is the budget" (D:1136-1137)
  - "Landing" becomes "Writing the results"; heading rename per glossary rule 2, anchors reported
- **`update-format`**: collect its refusals into one list (D:1276-1282; S:1475-1476, S:1485, S:1997,
  S:720).
- **`verify`**: lift the base fault list with check names, and the remedies (D:1204-1223; §12
  S:3049-3057).

### §9 Configuration
- Remove the links that point at §2.7 and the configuration commands; their content is already
  stated.

### §10 Metadata format version
- **New: Documents of a KB**, before The record files. Content:
  - the document classes (D:7-31): entry point, index, `axioms.md`, maintained-block documents, the
    `CLAUDE.md` redirect, summary pages, and leaves
  - markers read from leaves only
  - volume, citing S:115
  - the summary-page rules (D:213-239; §12 S:2839)
- **The record files**:
  - lift the record-field table (D:37-43)
  - in prose, "the index" means the record files (glossary B)
- **New: Statements and anchors**, after The record files. It replaces the statement-marker paragraph
  (S:2126-2136). Contents (D:127-179):
  - marker form and placement
  - how `document`, `line` and `snippet` name a line
  - how each kind's statement is read
  - what a missing or extra marker means for host, anchor and summary entry
  - the slug algorithm (GitHub's)
  - the section anchor

  Define "host" once (glossary C).
- **Citations**:
  - lift the link target per kind (§12 S:2942-2943)
  - lift "checked when placed" (§12 S:2843)
  - lift "a citation is not an edge" (D:212)
  - state citation target and `canonical_path` side by side
- **Axioms**: lift the heading form and `verify`'s three checks (D:64-69).
- **The load: every fault refused**: lift the nine-item fault list (D:420-428).
- **Five node types**:
  - lift the title and label rules (D:32-76)
  - lift title uniqueness within a kind (§12 S:2883)
  - define "mint" once (glossary C)
- **Four scoring relations**:
  - add a `references` row
  - define *dependents* (D:103)
- **The band ladder**: define band ladder and rung once (glossary C).
- **New: Provenance and the judged hash**, after Required bearings. Contents, from D:532-552 and the
  rehoming survey §4 item 4:
  - which records carry `manually-scored` and `judged-hash`
  - when a write op writes `true` or `false`, and re-assertion
  - that no argument sets the flag
  - what the hash covers
  - the all-zeros value
  - what makes an item stale

  Define "judged hash" once (glossary C).
- **The `demoted` relation**: its own heading, taken out of The migration announces itself
  (S:2521-2538). Lift the naming rule (D:115-116). Define `demoted` once (glossary C).
- **Cycles: cut as the walk finds them**: wording per glossary (cycle breaking, DFS back edge).
- **The `3.0.0 → 4.0.0` migration**: lift `.build/` placement and minted work and axiom ids (§12 S:2871,
  S:2890).

### §11 The MCP server
- **Remove** "no inference runs from any tool but `build` and `score`" (S:2724-2725). `probe` makes
  model calls too.
- **New: Served text**. It takes over the Instructions bullet (S:2736-2739). Contents:
  - the eight instruction items, with the wording SPEC requires, stated once (D:1451-1482)
  - which items `AGENTS.md` and `kbase --help` repeat byte for byte
  - the help renderings
  - the required content of the MCP and CLI notes (D:1514-1531): the `write` batch contract, the
    `probe` sentence, and the detached `build`/`score` note
  - the obligations on shared help bodies (D:1540-1547)
  - the advice on copying a row into an edge op (D:311-314)

  How the texts are stored, and the rendering test, go to ARCHITECTURE.
- **Two modes**: lift "startup never fails for want of a project" (D:1562).
- **Tools**: lift "root and `mcp` help name the three CLI-only commands" (§12 S:3088).
- Define "family tool" once (glossary C).

### §12 Interim releases
- Removed, after every rule above is lifted. Nothing in §12 is kept as text.

### Scoring schemes (separate files)
- RIGOR_AND_BEARING and STRENGTH: glossary pass only. STRENGTH defines "station" and `*pending*` once
  (glossary C).

## Follow-ups outside the SPEC rewrite

- ARCHITECTURE:
  - state where the code keeps each op's key list, and add a test that checks it against SPEC's
    tables (code work)
  - take over the mechanism lines moved out of SPEC
  - update the links at A:29, A:443, A:471, A:475, A:541, A:1906 and A:2147 to their new SPEC
    sections
- CONVENTIONS: update the links at C:211, C:272 and C:284.

## Dispatch plan

Six writer agents, each writing its sections to its own file under `DOCS_REVIEW/spec-parts/`:
1. §1–§7
2. §8 Write batches through `add-citation`
3. §8 queries through `verify`
4. §9
5. §10
6. §11 and the two scoring schemes

I merge the parts into `DOCS_REVIEW/SPEC.md`. Then two checks:
- **Coverage check**: every rule in the old SPEC, its §12 and the cited design sections appears in the
  new SPEC.
- **Drift probe**: scenario tasks run against the new SPEC.
