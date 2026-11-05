# SPEC rewrite outline

The structure of the rewritten SPEC, what each section takes in, and how six writers produce it in
parallel.

Inputs:
- `SPEC.md` (S:), read as committed. Its line numbers are what S: refers to.
- `reference/NEW_MCP_CLI_DESIGN.md` (D:), the retired design.
- `SURVEY-rehoming.md`, which maps every cited design section to its home.
- `GLOSSARY.md`, the wording rules.
- `THESIS.md`, which states which pieces are standard.
- The code, under `cmd/` and `internal/`, which settles conflicts (rule 9).

## Rules for the whole rewrite

1. **SPEC states every rule itself.** Each rule from SPEC.md (§12 included) or the design is stated in
   the section that owns it. No section delegates to the design ("is page §X's", "What this release
   adds") or to §12.
2. **Each rule is stated once.** It goes in the section this outline assigns it ("Assignments" below,
   otherwise the section listed under "Outline"). Every other section links to it, and adds only what
   is specific to that section. Where your source text restates a rule another section owns, replace
   it with a link.
3. **Standard behaviour is named in standard terms**, per the glossary. For example: writes are
   idempotent; a batch is all-or-nothing; formats migrate on read and are saved by an explicit
   command.
4. **SPEC states the operations in full**, keys included. "The per-op key vocabulary … is not copied
   into prose" (S:695-696) is removed.
5. **Behaviour stays, mechanism goes.** A sentence is mechanism if a compliant implementation could do
   otherwise and no consumer could tell. Keep what a consumer can observe; for example, "no public
   write op creates a `demoted` record" stays. Drop mechanism from your part and list it in your report
   (item 3) for ARCHITECTURE.
6. **Present tense, no history.** Drop "is gone", "replaces", "no longer", "now", "in place of",
   release versions and removed names, but keep any current refusal they imply. Versions appear only
   in the §10 migration sections.
7. **Text lifted from the design brings no design references.** Replace every § reference in it with
   the SPEC section that now holds the content, using the anchor table. Rewrite "this document" to
   name what is meant.
8. **The survey's gaps are part of the job.** Each "Missing" item that `SURVEY-rehoming.md` §2 names
   for your sections is part of your assignment, unless this outline says otherwise.
9. **Where sources conflict, the code settles it.** Check what `cmd/` and `internal/` do and write
   that. Where the code doesn't settle it, use this order: this outline, then SPEC.md's normative
   sections, then §12, then the design. Report every conflict with both source lines and how you
   resolved it. Example: `find` rows carry `node_type` (S:624, `internal/query/payload.go:165`), not
   `kind` (S:2961).
10. **This outline governs.** Where it disagrees with `GLOSSARY.md` or the survey, follow the outline.
11. **Section numbers stay as they are**, §4's gap included. ARCHITECTURE and CONVENTIONS cite them.
12. **§12 is removed.** Its rules are lifted by the §12 sweep (below), and nothing of it stays as text.

## Assignments for rules that span sections

| Rule | Stated in | Linked from |
|---|---|---|
| Work label grammar (D:57-58); a label is never an id (D:60-61); title uniqueness within a kind (S:2883, S:3045) | §10 Five node types | §8 `add-<kind>`, §8 `edit-<kind>`, §5 Starting over |
| Axiom heading form and its three checks (D:64-69) | §10 Axioms | §8 `verify` |
| How `document`, `line` and `snippet` name a line; snippet matching; the refusal that names nearby lines | §10 Statements and anchors | §8 add, `move-node`, `add-citation` (keep only the refusals specific to each op) |
| Which records carry `manually-scored` and `judged-hash`, and when each op writes them | §10 Provenance and the judged hash | §8 `add-<kind>`, `edit-<kind>`, Edge ops, `score` |
| `build-trail`'s result, refusals and commands (S:205-207, S:2687-2693, D:1177-1192) | §6 `kbase build-trail` | §11 (one bullet: it is a tool) |
| The `check: worktree` list (S:2826-2829) | §8 Write batches (commit flow) | §6 |
| An MCP refusal keeps `line` and `column` (S:2955) | §7 Item | — |
| Volume | §3 Three kinds of sheet (as now) | §10 Documents of a KB |

## Anchor table

New and renamed headings. Link with `[§N](#slug)`, and only to slugs in this table or headings
already in SPEC.md. Never cite by line number.

| Heading | Level and position | Slug |
|---|---|---|
| `` ### `kbase build-trail` `` | §6, after `kbase wait` | `#kbase-build-trail` |
| `### Write batches` | §8, first subsection. Values input, Idempotence, Writes leave nothing stale, One write lock and Write ops become `####` under it, with their slugs unchanged. `### init` follows it | `#write-batches` |
| `#### Writing the results` | §8 `score`, where `#### Landing` is now | `#writing-the-results` |
| `### Documents of a KB` | §10, before The record files | `#documents-of-a-kb` |
| `### Statements and anchors` | §10, after The record files | `#statements-and-anchors` |
| `### Citations` | §10, after Statements and anchors; it is a bold paragraph now (S:2138) | `#citations` |
| `### Provenance and the judged hash` | §10, after Required bearings | `#provenance-and-the-judged-hash` |
| `` ### The `demoted` relation `` | §10, before Cycles | `#the-demoted-relation` |
| `### Tools`, `### Two modes`, `### Resources`, `### Served text` | §11, in that order; the other §11 bullets stay bullets under the nearest heading | `#tools`, `#two-modes`, `#resources`, `#served-text` |

## Outline

Each entry gives a section and what changes. "Lift" names content moving in, with its source lines.

### §1 What kbase is
- Rewrite S:13-18 as a plain statement of what kbase does, in standard terms consistent with
  THESIS's building blocks.
- Keep the definitions at S:20-29 after the glossary pass ("volume root" becomes "the top `.tex`
  file").

### §2 Given interfaces
- Glossary pass only.

### §3 Compatibility contract
- **Stamped documents**: lift the maintained-block rules (D:240-257; §12 S:2866-2870). Define
  "maintained block" here (glossary C).
- **`claim-graph.svg`**: define "sheet" here (glossary C).

### §5 Build behaviour
- **Starting over**: replace "are page §3.5's" (S:202-204) with the full list of `build`'s refusals.
  The list is currently spread across S:179-187, S:220-260, S:441-446 and S:2007. Title uniqueness
  is linked to §10.
- **Co-linearity cull**: define cull here (glossary C).

### §6 Build state, ledger and monitoring
- **`kbase status`**:
  - lift the build `state` vocabulary (D:1145)
  - lift the refusals (D:1138-1160; S:2661)
- **`kbase wait`**: lift "`--pid` is required and positive", and its refusal (D:1175-1176).
- **`kbase build-trail`**, new: see Assignments.

### §7 Tool-result contract and exit codes
- **Item**:
  - define every check value (D:498-517)
  - add `agents-redirect` to S:600's list
  - keep the values the design lacks (`include`, `metadata-key`, `inputs`, `pandoc`, `export-dir`, `write`)
  - an MCP refusal keeps `line` and `column` (S:2955)
- **Result-key table**:
  - add a `build-trail` row
  - add `warnings` to the `configure-providers`, `configure-models` and `models` rows
- **Bounded results**: lift the sorting rules (D:527-531) and negative `limit` as a slice (§12
  S:2963).

### §8 Maintenance subcommands
- **Opening (S:683-689)**: remove "`probe` excepted". `probe` is a tool.
- **Write batches**, new. Contents (rehoming survey §4 item 1; D:325-415, D:553-571; §12
  S:2812-2829, S:2878-2879):
  - batch shape and the two interfaces
  - order, with each entry checked against the state the earlier ones left
  - no reference to an id minted in the same batch; minted ids returned per entry
  - every shape error reported at once, then apply, stopping at the first refusal
  - all-or-nothing, rebuild of derived views included; a crash between two files' writes is
    recovered through git
  - the rebuild of derived views does not repair or convert (D:414)
  - idempotence and existence:
    - create fails if the item exists; delete or edit fails if it is absent
    - an edit that changes nothing is `unchanged`
    - lookups are exact
    - a refusal's remedy points at the query that lists what exists (D:560-563)
  - value domains and the prose-value rules
  - dry run equals a real run
  - the `commit` flow, including the `check: worktree` list
  - locks
- **`add-<kind>`**: lift each kind's key table and refusals (D:635-692).
- **`edit-<kind>`**:
  - lift the editable-field table per kind (D:712-718)
  - lift the full re-pointing and label-change rules (D:696-729)
- **Edge ops**:
  - lift the keys of `add-edge`, `edit-edge` and `remove-edge` (D:789-835)
  - lift "`remove-edge` leaves every other record byte for byte" (D:829-830)
  - lift the refusals (§12 S:2912-2921)
- **`restore-demoted`**:
  - lift "`relation` must be `demoted`" (D:841, D:845)
  - lift exact restoration of relation, context, bearing and flag (§12 S:2803-2805)
- **`remove-node`**: lift its core effect (§12 S:2927-2930).
- **`move-node`**: lift its keys (D:766-780).
- **`add-citation`**:
  - lift its keys and the output form of each citation form (D:860-874)
  - lift "snippet occurs exactly once on the line" (§12 S:2946)
- **`find`**: lift the filter semantics and sort keys (D:917-935; §12 S:2957-2965).
- **`show`**: lift the `summary` form `<page>#<slug>` (§12 S:2967).
- **`edges`**: lift the arguments and the base order (D:962-981; §12 S:2973-2979).
- **`trace`**:
  - lift the `contribution` sort, where a `refutes` edge's contribution is its cap (D:990-994)
  - lift `*pending*` versus null (D:1000-1001)
  - lift "never follows `references` or `demoted`" (D:985-986)
- **`score`**:
  - lift the default scope (D:1092-1096)
  - lift what `--rescore-manual` reaches (§12 S:3001-3003)
  - lift the base refusal list (D:1122-1135)
  - lift "the scope is the budget" (D:1136-1137)
  - `#### Landing` becomes `#### Writing the results`
- **`update-format`**:
  - collect its refusals into one list (D:1276-1282; S:1475-1476, S:1485, S:1997, S:720)
  - state the domain-rule exception: it checks the build records at the repository root that it
    would move (D:438-439 against D:1277)
- **`verify`**:
  - lift the base fault list with check names, and the remedies (D:1204-1223; §12 S:3049-3057)
  - the axiom checks are linked to §10 Axioms
- **`status`, `cancel` stub (S:1526-1531)**: removed. §6 holds both.

### §9 Configuration
- Remove the links to the design's provider names and configuration commands, and the delegations
  they carry. Their content is already stated (rehoming survey rows §2.7 and §3.7).

### §10 Metadata format version
- **Documents of a KB**, new:
  - the document classes (D:7-31): entry point, index, `axioms.md`, maintained-block documents, the
    `CLAUDE.md` redirect, summary pages, and leaves
  - markers are read from leaves only
  - volume, linked to §3 Three kinds of sheet
  - the summary-page rules (D:213-239; §12 S:2839), absorbing the Register pages paragraph (S:2144)
- **The record files**: lift the record-field table (D:37-43).
- **Statements and anchors**, new. It replaces the statement-marker paragraph (S:2126-2136):
  - marker form and placement
  - how a line is named
  - how each kind's statement is read
  - what a missing or extra marker means
  - the slug algorithm (GitHub's)
  - the section anchor

  Sources: D:127-179. Define "host" here (glossary C).
- **Citations**, now a heading:
  - lift the link target per kind (§12 S:2942-2943)
  - lift "checked when placed" (§12 S:2843)
  - lift "a citation is not an edge" (D:212)
  - state citation target and `canonical_path` side by side
- **Axioms**: lift the heading form and its three checks (D:64-69).
- **The load: every fault refused**: lift the nine-item fault list (D:420-428).
- **Five node types**:
  - lift the title and label rules (D:32-76), the label grammar, label-never-id and title
    uniqueness (see Assignments)
  - define "mint" here (glossary C)
- **Four scoring relations**:
  - add a `references` row
  - define *dependents* (D:103)
- **The band ladder**: define band ladder and rung here (glossary C).
- **Provenance and the judged hash**, new. Contents (D:532-552; rehoming survey §4 item 4):
  - which records carry the flag and the hash
  - when a write op writes `true` or `false`, and re-assertion
  - that no argument sets the flag
  - what the hash covers
  - the all-zeros value
  - what makes an item stale

  Define "judged hash" here (glossary C).
- **The `demoted` relation**, new. It is taken out of The migration announces itself (S:2521-2538).
  Lift the naming rule (D:115-116). Define `demoted` here (glossary C).
- **Cycles**: wording per glossary (cycle breaking, the DFS back edge).
- **The `3.0.0 → 4.0.0` migration**: lift `.build/` placement and minted work and axiom ids (§12
  S:2871, S:2890).

### §11 The MCP server
- Restructure into the four headings in the anchor table.
- **"No inference runs from any tool but `build` and `score`" (S:2724-2725)** becomes "but `build`,
  `score` and `probe`".
- **Served text**, new. It takes over the Instructions bullet (S:2736-2739). Contents:
  - the eight instruction items, with the wording SPEC requires stated once (D:1451-1482)
  - which items `AGENTS.md` and `kbase --help` repeat byte for byte
  - the help renderings
  - the required content of the MCP and CLI notes (D:1514-1531)
  - the obligations on shared help bodies (D:1540-1547)
  - the advice on copying a row into an edge op (D:311-314)
- **Two modes**: lift "startup never fails for want of a project" (D:1562).
- **Tools**:
  - lift "root and `mcp` help name the three CLI-only commands" (§12 S:3088)
  - `build-trail` gets one bullet, linked to §6
- Define "family tool" here (glossary C).

### Scoring schemes (separate files)
- RIGOR_AND_BEARING and STRENGTH: glossary pass only. STRENGTH defines "station" and `*pending*`
  (glossary C).

## The §12 sweep

Every writer sweeps all of §12 (S:2747-3167, the 0.9.0 and 0.8.0 blocks included). For each bullet
whose subject is a command or concept in your sections, record exactly one disposition:
- already stated at ⟨heading⟩;
- lifted to ⟨heading⟩;
- mechanism (quote it);
- history only (quote it).

Write a lift only into your own sections. A rule that belongs to another slice is reported, not
written.

## Dispatch

Six writer agents. Each writes `DOCS_REVIEW/spec-parts/<n>-<first-section-slug>.md`.

**Content of each part:**
- The complete final text of every section in its slice, unchanged sections included after the
  glossary pass.
- At current heading levels.
- No title of its own, and no notes to the coordinator in the file.

**Slices:**

| Slice | Sections | Source | Notes |
|---|---|---|---|
| 1 | `## 1.` through the end of `## 7.` | S:11-682 | Includes §6 `build-trail`. |
| 2 | `## 8.` heading and opening through `add-citation` | S:683-948 | Owns the opening; `init` included. |
| 3 | `### Queries` through `verify` | S:949-1525 | Omits the `status`, `cancel` stub. |
| 4 | `## 9.` | S:1532-1958 | |
| 5 | `## 10.` | S:1959-2567 | |
| 6 | `## 11.` | S:2568-2746 | Omits `## 12.` Also writes the two schemes as whole files: `spec-parts/RIGOR_AND_BEARING_SCORING_SCHEME.md` and `spec-parts/STRENGTH_SCORING_SCHEME.md`. |

**Report.** Each writer returns:
1. **Dispositions.** Every S: and D: range this outline assigns it, and every §12 bullet in its sweep,
   each with its disposition.
2. **Rules belonging to another slice**: quote, source line, target section.
3. **Mechanism moved out**, quoted with its S:/D: line, for ARCHITECTURE.
4. **Source conflicts**: both lines, and how rule 9 resolved them.
5. **Headings added or renamed**, with old and new slugs.
6. **Glossary sites**: those listed under glossary rules 2, 6 and 7.

**Merge.** The coordinator concatenates the parts into `DOCS_REVIEW/SPEC.new.md`. `SPEC.md` stays
untouched until two checks pass:
- **Coverage check**: every rule in the old `SPEC.md`, its §12 and the cited design sections appears
  in the new text.
- **Drift probe**: scenario tasks run against the new SPEC.

Only then does `SPEC.new.md` replace `SPEC.md`.

## Follow-ups outside the SPEC rewrite

- ARCHITECTURE:
  - state where the code keeps each op's key list, and add a test that checks it against SPEC's
    tables (code work)
  - take over the mechanism lines from the writers' reports
  - update the links at A:29, A:443, A:471, A:475, A:541, A:1906 and A:2147 to their new SPEC
    sections
- CONVENTIONS: update the links at C:211, C:272 and C:284.
- Code: leaf discovery excludes names SPEC does not state (`internal/kb/kb.go` `ExcludeDirs`,
  `ExcludeNames`: `session/`, `tools/`, `invariants.md`, `claim-quality-closure-roadmap.md`). Align
  the code with SPEC's leaf definition.
- Code: rename the decision-API setting on every interface — `providers.toml` key `decisionApi`,
  `configure-providers` flag `--decision-api`, `show-config` field `decision-api` — with no alias for
  the old names.
- Code: an identical add (one that would change no byte of `kb-root/`) is `unchanged`, reporting the
  existing item; a differing add stays refused (SPEC §8 Idempotence, `add-<kind>`, Edge ops). The
  same change updates ARCHITECTURE invariant I6 and its detail.
- Code: one retry rule for every model call. A call that did not get through (transport error, 429,
  5xx, an undecodable response body) waits 5 s, then 30 s, before its next attempt. A retry that
  changes the ask because the answer came back flawed (refused, malformed, failing its check) goes out
  at once. Decision calls currently retry comms failures at once (`internal/model/decision.go`); bring
  them in line. ARCHITECTURE §7 Retry and §12 Constants change with it.
- ARCHITECTURE: the `claims-declared` and `claims-discovered` stages have no mechanism section — which
  environments become claims, how prose is chunked for the model, what it is asked, and how answers
  become nodes (`internal/claimgraph`).
- SPEC: §7's long table cells (the `remedy` rule; the `write`, `show` and `stats` result rows) need
  restructuring into lists or sub-tables.
- Code: the `trace` help carries the hint that `trace` sorted by contribution shows what would
  strengthen a node, and the edge-op help carries the bearing-copy advice (SPEC §11 Served text).
- Code: `update-format`'s help says to run it after upgrading kbase (SPEC §11 Served text).
- Code: the help of each op that takes a `snippet` recommends quoting a few distinctive words
  (SPEC §11 Served text).
