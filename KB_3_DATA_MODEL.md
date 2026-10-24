# KB format 3.0.0 — the data model

The design `ACTIVE_PLAN.md` calls for: the inventory, the two record schemas, and what every verb,
the converter and the contract documents do under them. It implements Settled 1–12 and Rulings 13–21
of `ACTIVE_PLAN.md`; where this document and the plan differ, the plan governs.

Every code fact names its package and function; the counts rest on the commands in §17. `$MC` is the
ModernCorp KB path `ACTIVE_PLAN.md`'s Terms give; nothing in this repository depends on it.

A **register page** is a generated `claim-rigor.md` (§5). The **sheets** are the claim-graph SVGs and
their `.dot` sources; they stay derived output no session reads. A **warning** is a conversion or
repair item (§8.5).

---

## 1. Settled items and rulings → sections

| Item | Section |
|---|---|
| Settled 1 — `.index/` kbase-owned, two authored files, everything else derived | §3.1, §3.6, §9.8, §13 |
| Settled 2 — edge record types | §3.3 |
| Settled 3 — node record types | §3.2, §3.4 |
| Settled 4 — `manually-scored` writes | §6 |
| Settled 5 — `score --rescore-manual` | §7 |
| Settled 6 — visibility | §6.3 |
| Settled 7 — leaves are prose and markers; the entry point keeps its stamp | §2.1, §4, §8.1 |
| Settled 8 — registers are generated pages | §5 |
| Settled 9 — `refresh` regenerates, `verify` checks | §9.3, §9.4 |
| Settled 10 — 3.0.0, converter from 2.0.x | §8 |
| Settled 11 — the session rule | §11 |
| Settled 12 — the benchmark | §12 |
| Ruling 13 — an upgrade never refuses | §8, §8.1–§8.5, §13 |
| Ruling 14 — host from the one statement marker | §4, §8.2 |
| Ruling 15 — `rigor` and `bearing` optional | §6.1, §9.2 |
| Ruling 16 — degraded load | §3.7, §9 |
| Ruling 17 — cycle breaking cuts as the walk finds cycles | §3.3, §9.2, §9.5, §10 |
| Ruling 18 — stale register pages; sessions read them as prose | §5, §9.4, §9.8 |
| Ruling 19 — no origin | §3.3, §9.4, §9.7 |
| Ruling 20 — 3.0.0, two files; ROADMAP | §13 |
| Ruling 21 — register page placement is derived | §3.2, §3.5, §5, §8.1, §9.2, §9.3, §9.4 |

---

## 2. The 2.0.x inventory: what converts from

Every site a 2.0.x load reads as metadata. The readers and writers listed are 2.0.x code as it
stands; §9 says which retire and which change, and the converter copies the 2.0.x readers it needs
(§8). The last column is where each piece of data lives in 3.0.0: a *field of a node record* or a
*field of an edge record* (the field named), *an edge record* (the relation named), *derived at
load*, *rendered on the register page*, or *removed* (with the reason). Two homes stand outside
those six: the `kb-format` stamp, *kept in the entry point's frontmatter*, and the statement
marker, *kept in leaf prose*. In 3.0.0 a register page is only ever rendered from records and is
never read.

### 2.1 2.0.x frontmatter keys

2.0.x block readers: `kb.ParseFrontmatter` callers; `index.metaCheck.run`
(`CheckFrontmatterPresence`); `index.inScopeText` (the citation gate's leaf scope, keyed on `kind:`);
`write.existingFrontmatter`, `observedFrontmatter`, `replaceBlock`, `write.FrontmatterValues`
(`set-frontmatter`, and `claimgraph.leafOf` through it); `claimgraph.checkCleanliness`;
`build.seedSpine` (filters `CheckFrontmatterPresence`).

| Key | 2.0.x writers | 2.0.x readers | 3.0.0 home |
|---|---|---|---|
| `kind` | `cmd.skeletonEntryPoint`; `write.planSetFrontmatter`; `claimgraph.IssueCulls`, `writer.landLeaf`, `writer.writePlan` (pass 2, via `set-frontmatter`) | `kb.Frontmatter.Kind` → `kb.ParseLeaf`, `parseIndex`, `declarations`; `index.inScopeText`; `index.metaCheck.run`; `claimgraph.passTwoGate`, `readModuloIDsView` | *derived at load*: `claimgraph.documentKind`'s rule (path and child list), moved into `kb` |
| `claims` | `write.planSetFrontmatter`, `write.dropLeafClaim`; `claimgraph.writer.landLeaf`, `writePlan`, `IssueCulls` | `kb.ParseLeaf` → `index.buildCites`, `SubtreeAggregates`, `LeafReferences`, `score.declaringLeaf`, `sheet`; `kb.AuthoredIDs`; `index.metaCheck.run`; `write.claimNodeProver`, `insertMarker`, `observedFrontmatter`; `claimgraph.readGraph`, `passTwoGate`, `readModuloIDsView`, `writer.landLeaf` | *derived at load* from the statement marker (§4); the converter turns a listing into a marker or a mention (§8.2) |
| `no-claim` | `write.planSetFrontmatter`; `claimgraph.writePlan`, `IssueCulls` | `kb.ParseLeaf` (unused further); `index.metaCheck.run`; `claimgraph.passTwoGate`, `readModuloIDsView` | *removed*: a leaf with no marker hosts nothing; a build's reason stays in the node-pass record (`LeafEntry.Outcome`) and the cull record |
| `experiments` | `write.planSetFrontmatter` | `kb.ParseLeaf` (`ExperimentRefs`) → `index.LeafReferences` (no rendered footer reads it); `index.metaCheck.experimentRefs`; `kb.parseExperimentLeaf` | *removed*: no reader but its own check |
| `path-stable` | `write.planSetFrontmatter` | `write.observedFrontmatter`, `frontmatterProver` | *removed*: no kbase reader (§16) |
| `experiment-nodes`: `exp-id`, `status`, `rigor`, `supports`, `lifts`, `refutes` | `write.planExperiments`, `planSetFrontmatter`, `replaceDeclarationRigor`, `replaceDeclarationPair`, `dropDeclarationPairs` | `kb.parseExperimentLeaf`, `kb.AuthoredIDs`, `index.metaCheck.run`, write provers | *fields of a node record* (`ExperimentNode`: `id`, `status`, `rigor`); the pairs, *edge records* (`ScorableEdge`); the host, *derived at load* from the marker |
| `analysis-nodes`: `ana-id`, `lifts`, `refutes` | `write.planSetFrontmatter`, `replaceDeclarationPair`, `dropDeclarationPairs` | `kb.parseAnalysisLeaf` (joined in `kb.Discover`), `kb.AuthoredIDs`, `kb.WalkRegisters` | the host, *derived at load* from the marker; the pairs, *edge records* (`ScorableEdge`) |
| `subtree-claims`, `subtree-experiments` | `index.refresher.aggregates` | `kb.parseIndex` → `index.metaCheck.aggregateDrift` | *derived at load* (`index.SubtreeAggregates`) |
| `kb-format` | `kb.StampFormat` from `cmd.skeletonEntryPoint`, `index.refresher.save` | `kb.FormatStamp` from `kbload.Open` | *kept in the entry point's frontmatter*, the only frontmatter left |

### 2.2 2.0.x register lines (`claim-rigor.md`)

2.0.x readers: `kb.Registers` with the readers of `internal/kb/register.go`, consumed by
`kb.Discover`, `WalkRegisters`, `AuthoredIDs`, `KnownClaimIDs`, `WorkCitations` (→
`index.coverageAt`); `index.metaCheck` (`run`, `rigorBlocks`, `scoreValues`, `footersFresh`,
`registerWalk`); `index.CitationFindings` (`foreignEdgeFindings`, channel checks);
`write.readRegister`, `takeCensus`; `claimgraph.registerTitles`, `claimEntries`, `readModuloIDsView`;
`export.bibliography`.

| Line | 2.0.x writers | 3.0.0 home |
|---|---|---|
| `## <title>` | `write.renderEntry`, `renderWorkEntry` | *field of a node record*: `title` (claim, work); an analysis's title is its 2.0.x host's heading (§3.2) |
| `<!-- id: <id> -->` | inserts; `write.dropEntryBlock` | *field of a node record*: `id` |
| which 2.0.x register holds the entry | the insert's `register` value | *removed*: placement is *derived at load* from the host (§5); 15 of `$MC`'s 51 claims move (§17 E7) |
| `> **Leaf references:** …` | `index.refresher.leafReferences` | *derived at load*, *rendered on the register page* |
| `### Rigor`, `### Strength` | `write.renderEntry`; refresh | *rendered on the register page* |
| `- rigor:` | inserts, `write.replaceFieldLine` | *field of a node record*: `rigor` |
| `- rationale:` | inserts, `replaceFieldLine`; `claimgraph.writeWorks` | *field of a node record*: `rationale` |
| `- no-edge:` | inserts (`renderNoEdgeLine`) | *field of a node record*: `no-edge` |
| `- strengthen-by:` | `insert-claim-entry` (`renderStrengthenByBullet`) | *field of a node record*: `strengthen-by`; its rows *derived at load* |
| `- rests-on:` bullets | inserts, `write.addBullets`, `replacePremiseBearing`, `dropListBullets`, `demoteClaimEdge`, `planResolveDemoted` | *edge records*: `supports` (source `clm-`/`exp-`) or `founds` (source `wrk-`/`axm-`), target the entry; *field of an edge record* `bearing`, `context`; the source's title and `(strength …)` *derived at load*, *rendered on the register page* |
| claim `- refutes:` pairs | `insert-claim-entry`, `set-bearing`, drop ops | *edge records*: `refutes`, source the entry; *field of an edge record* `bearing` |
| analysis `- lifts:`, `- refutes:` pairs | `insert-analysis-entry`, `write.editStagedPair` | *edge records*, source the analysis; a second 2.0.x site beside the leaf's pairs (§8.3) |
| `- references:` | inserts, `add-rests-on`, `add-build-edges`, drop ops | *edge records*: `references`, source the entry |
| `- demoted:` with `(origin …)` | `write.demoteClaimEdge`, `planResolveDemoted` | *edge records*: `DemotedEdge`, in the replaced `supports` edge's orientation (§3.3); the origin *removed* (Ruling 19) |
| `- strength:` line and trace; `(strength …)` | `index.refresher.rewriteStrength` | *derived at load*, *rendered on the register page* |
| text outside every entry | hand; `--create`'s H1 | *removed* (§8.4) |

### 2.3 2.0.x `.index/` files (all derived)

2.0.x writer: `index.refresher.emit` from `index.BuildRecords`; `build.seedSpine` and `cmd/init.go`
create the directory.

| File | 2.0.x readers | 3.0.0 home |
|---|---|---|
| `claims.yaml` | `query.Load`; `sheet.readIndex`; `kb.SheetDigestDrawnOf` (→ `build`'s overview row, `internal/build/rows.go`); `index.knownNodeIDs`; `index.indexMaps`; `metaCheck.indexWellFormed`, `indexFresh`, `referentialIntegrity`, `strengthFresh`; compat harnesses | *derived at load* (§3.6); its authored columns are *fields of a node record* |
| `edges.yaml` | `query.Load` (`Index.Edge`, `ScoringSet`, deps, census); `sheet.readIndex`; `index.indexMaps`; `metaCheck.referentialIntegrity` | *edge records*, in the authored 3.0.0 `edges.yaml` at this path; `target_kind` *derived at load*; `source_strength_recorded` *removed* (its one reader, `metaCheck.annotationDrift`, compared an annotation 3.0.0 does not author) |
| `strengthen-by.yaml` | `query.Load` | *derived at load* |
| `cites.yaml` | `query.Load`; `score.declaringLeaf` | *derived at load* from markers |
| `subtree-aggregates.yaml` | `query.Load` | *derived at load* |

### 2.4 Other 2.0.x parsed sites

| Site | 2.0.x readers and writers | 3.0.0 home |
|---|---|---|
| marker `<!-- claim-rigor: <id> -->` | `kb.ParseLeaf`, `kb.MarkerLines` (`score.claimPassages`, `sheet.kindReader.claimKind`), `metaCheck.tier2`, `index.inScopeText`; `write.insertMarker` | *kept in leaf prose* as the statement marker (§4) |
| `axioms.md` `### Axiom <label>: Title` | `kb.ParseAxioms`, `KnownAxiomIDs`, `AxiomSource`; `index.declarationDocuments` | *fields of a node record* (`AxiomNode`); `axioms.md` stays prose and a sanctioned citation channel |
| legacy `invariants.md`, `AGENTS.md` declarations | `kb.parseLegacyAxioms`, `LegacyAxiomDeclarations` | *fields of a node record* (`AxiomNode`), through the converter |
| build records' unmarked and cull links (the sheet's cited/inferred split) | `sheet` via `buildrecords.InferredLinks`; `claimgraph.inferredAt` → `demotedOrigin` | *removed* (Ruling 19); `buildrecords.ReadUnmarked`/`ReadCull` stay for the build's own stages (`claimgraph.orientLinks`) |

---

## 3. The schemas and the load

### 3.1 Rules for both files

- One YAML document each: a block sequence of block mappings, under SPEC §10's dialect. Empty: `[]`.
- A node record's type is its `id` prefix (`clm-`, `ana-`, `exp-`, `wrk-`, `axm-`); an edge record's
  type is its `relation`. No `kind` field.
- Every field a type has is present; any other field is refused. An empty value is spelled `""` or
  `[]`, never omitted, so no decode reads an absent bearing as the zero `kb.Bearing`.
- Strings are double-quoted, escaping `\`, `"`, every control character below U+0020 other than
  tab, and every character `index.streamUnsafe` lists, as `\uXXXX`. A value holding a blank line is
  refused (`checkProse`); in any other value every whitespace run becomes one space and the ends are
  trimmed (`write.collapse`). Adoption, the `done`/`unchanged` re-assertion test and read-back compare
  the collapsed value. The load refuses a string holding a line break (`check: metadata`). The
  converter joins a 2.0.x continuation line with one space. Numbers plain, in the spelling `write`
  gives a rigor today. Booleans `true`/`false`. Keys in the type's field order.
- Nodes are written in `id` byte order; edges by identity (§3.3): relation in `kb.Relations` order,
  for a demoted record then `replaced-relation` in the same order, then `source`, then `target`.
- Every writer re-renders a whole file from the records through `atomicfile.Write`, only where its
  bytes change.

### 3.2 Node records

| Type | Fields, in order |
|---|---|
| `ClaimNode` (`clm-`) | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `no-edge`, `strengthen-by` |
| `AnalysisNode` (`ana-`) | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `no-edge` |
| `ExperimentNode` (`exp-`) | `id`, `title`, `status`, `rigor`, `manually-scored` |
| `WorkNode` (`wrk-`) | `id`, `title`, `rigor`, `rationale` |
| `AxiomNode` (`axm-`) | `id`, `title` |

No record carries a register: which register page renders a node is derived from its host (§5).

| Field | Type |
|---|---|
| `title` | non-empty one-line string. A claim's and a work's is its 2.0.x register heading; an analysis's and an experiment's is its 2.0.x host leaf's first heading, the title the 2.0.x load gave it (`kb.parseAnalysisLeaf`, `parseExperimentLeaf`) |
| `rigor` | number in `[0, 1]`, or `"*pending*"`; above the maximum is read as the maximum (`kb.State.readAuthoredRigor`'s clamp) |
| `manually-scored` | boolean (§6) |
| `rationale`, `no-edge` | string, may be `""` |
| `strengthen-by` | list of strings, may be `[]`, block form when non-empty |
| `status` | `"run"` or `"pending"` |

Titles may repeat. Two entries of one register page with one title share the slug, exactly as two
2.0.x register entries did; `canonical_anchor` reports the bare slug for both.

```yaml
- id: "axm-S1"
  title: "Agents act on what they observe"
- id: "ana-aaa007"
  title: "Sensitivity of the flow bound"
  rigor: 0.7
  manually-scored: true
  rationale: "Fixture."
  no-edge: ""
- id: "clm-aaa001"
  title: "Theorem 1"
  rigor: 0.9
  manually-scored: false
  rationale: "Fixture."
  no-edge: ""
  strengthen-by:
    - "State the noise distributions."
- id: "exp-aaa008"
  title: "Flow measurement"
  status: "run"
  rigor: "*pending*"
  manually-scored: false
- id: "wrk-smith2020"
  title: "Smith, A. 2020. *Markets*. Fixture Press."
  rigor: 0.9
  rationale: "Fixture Press."
```

### 3.3 Edge records

| Type | Relations | Fields, in order |
|---|---|---|
| `UnscoredEdge` | `references` | `relation`, `source`, `target`, `context` |
| `DemotedEdge` | `demoted` | `relation`, `source`, `target`, `context`, `replaced-relation`, `bearing`, `manually-scored` |
| `BearingEdge` | `founds` | `relation`, `source`, `target`, `context`, `bearing` |
| `ScorableEdge` | `supports`, `lifts`, `refutes` | `relation`, `source`, `target`, `context`, `bearing`, `manually-scored` |

`context` is `""` for none; queries render `""` as `null`, so SPEC §7's `deps` and `edges` shapes
hold. `bearing` is a number in `[0, 1]` or `"*pending*"`.

**Orientation.** Scoring edges push (supporter → supported), as the 2.0.x index row. `references`
runs from the entry listing it to the node named, as 2.0.x. A `demoted` record keeps the replaced
edge whole: its `source`, `target`, `context`, and in `replaced-relation`, `bearing` and
`manually-scored` that edge's relation and values. `replaced-relation` is `supports`, `lifts` or
`refutes`: only those can close a cycle (a `founds` source, a work or axiom, has no incoming scoring
edge), and each is a `ScorableEdge`, so a `DemotedEdge` always carries both values. Restoring it
(`resolve-demoted`, §9.2) rebuilds the original record exactly, with no re-orientation. This
reverses 2.0.x's demoted row, which ran from the supported entry to its premise; the converter flips
it (§8.1).

**Identity.** An edge record's identity is (`relation`, `source`, `target`), except a `demoted`
record's, which is (`demoted`, `replaced-relation`, `source`, `target`): an analysis's `lifts` and
`refutes` to one claim, both demoted, are two records, not a duplicate. Wherever this document says
"edge identity" of a demoted record, it means this four-part identity.

```yaml
- relation: "supports"
  source: "clm-aaa002"
  target: "clm-aaa001"
  context: "first use; second use"
  bearing: 1.0
  manually-scored: false
- relation: "founds"
  source: "wrk-smith2020"
  target: "clm-bbb002"
  context: ""
  bearing: 0.75
- relation: "lifts"
  source: "exp-aaa008"
  target: "clm-aaa003"
  context: ""
  bearing: 0.8
  manually-scored: true
- relation: "references"
  source: "clm-aaa004"
  target: "clm-bbb003"
  context: ""
- relation: "demoted"
  source: "clm-aaa004"
  target: "clm-bbb001"
  context: "the circle's other half"
  replaced-relation: "supports"
  bearing: "*pending*"
  manually-scored: false
```

### 3.4 Axioms

`nodes.yaml` declares an axiom; `axioms.md` is prose stating it, one `### Axiom <label>: <title>`
heading each, and is not read for declarations. New write op `insert-axiom-entry` (`label`,
`title`). An axiom's `canonical_anchor` is `kb.Slugify("Axiom <label>: <title>")`. `verify` checks
the two agree (`axiom heading`, not refresh-fixable): each axiom record has its heading in
`axioms.md`, and each such heading names an axiom record with that label and title. So the anchor
never dangles, and the stamped CONVENTIONS' statement of the heading form (§13.1) holds.

### 3.5 `canonical_path` and `canonical_anchor`

| Kind | `canonical_path` | `canonical_anchor` |
|---|---|---|
| claim | its register page (§5); `null` with no marker | `kb.Slugify(title)`; `null` with no marker |
| work | the root register page, `claim-rigor.md` | `kb.Slugify(title)` |
| analysis, experiment | the host leaf; `null` with no marker | the slug of the heading of the section holding the marker |
| axiom | `axioms.md` | §3.4 |

A claim's value equals 2.0.x's where its 2.0.x register is the page §5 derives for it (36 of `$MC`'s
51, §17 E7); a work's equals 2.0.x's where its 2.0.x register was the root one; an analysis's and an
experiment's equal 2.0.x's wherever the converter placed the marker (§8.2).

### 3.6 Derived at load

One function, `index.Derive` (today's `index.BuildRecords` without serialisation, coverage check or
cycle refusal), returns `kb.NodeRow`, `EdgeRow`, `StrengthenByRow`, `CiteRow` and `AggregateRow` in
memory: strength, derivation strength, build status and band, `premise_count`,
`strengthen_by_count`, `citation_count`, `target_kind`, strengthen-by rows and `mentioned_ids`,
cites, subtree aggregates, document kinds, hosts, canonical paths and anchors, the axiom rigor, and
the degraded state (§3.7). Edge rows keep `buildEdges`' order (source, target, context). `query.Load`,
`sheet`, `score`, `verify`, `export` and the page renderer read this one value. `kb.ReadIndex`,
`kb.IndexFiles`, `index.Serialize`, `index.checkCoverage`, `index.coverageAt` and `kb.WorkCitations`
retire: an edge naming no node is a dangling edge (§3.7).

### 3.7 The load: strict on syntax, lenient on integrity

**Refused**, naming file and line (`check: metadata`; `check: index` for an absent `nodes.yaml` or
`edges.yaml` in a stamped 3.0.0 KB, remedy "restore it from git"): a file that does not parse;
unknown or missing field; wrong type or domain; an id outside its kind's grammar; duplicate node `id`;
an illegal pairing (`kb.BearingLegal`, SPEC §10's kind table). These replace 2.0.x's bullet faults
R1–R5.

**Degraded** — the load completes, resolves the fault for the walk, and marks itself degraded; the
state is computed on every load and never stored:

| Fault | Resolution for the walk, and `refresh`'s repair |
|---|---|
| Duplicate edge identity (§3.3; for a demoted record, the four-part identity) | the record with the higher bearing (a number above `*pending*`; the first in file order on a tie) — for a scoring edge its `bearing`, for a demoted record the replaced edge's `bearing`. For `references`, the first in file order |
| Dangling edge (an end no record carries) | dropped |
| Cycle among scoring edges | broken by §10's rule; nothing is dropped |

While degraded, `verify` runs and reports the degraded state and every fault with remedy
`kbase refresh`; every query, `score`, `export-latex`, `render-claim-graph`, a `build` resuming a
trail and every write op is refused, `check: degraded`, naming the faults and remedy `kbase refresh`.
`refresh` writes the resolved records as clean files and a warning per loss or cut (§8.5). Merging
metadata across branches is unsupported; `kbase refresh` repairs what a merge leaves.

---

## 4. Statement markers

- **Form** `<!-- claim-rigor: <id> -->`, for any `clm-`, `ana-` or `exp-` id (the reader widens from
  `kb.ClaimIDs` to `kb.NodeIDs`).
- **Mandatory and single.** Every claim, analysis and experiment has exactly one marker; its host is
  the leaf holding it, derived at load. No record stores hosting. A node with no marker has no host,
  and a node with several takes the first in path order for the walk; both are `verify` faults
  (`statement marker`), not refresh-fixable, and neither degrades the load.
- **A claim's statement** is the block its marker sits in (`score.blockExtent`).
- **An analysis's or an experiment's statement** is the section the marker sits in: from the nearest
  heading above it to the next heading as deep or shallower (`kb.AnchorSection` over that heading's
  slug). The inserts and the converter place an analysis's or an experiment's marker on a new line
  directly after the host's first heading, so its statement equals 2.0.x's first-heading section;
  `mark-claim-in-leaf` may move it.
- **Placing one.** `mark-claim-in-leaf` (`document`, `id`, `locator`) places or moves a node's one
  marker, any of the three kinds, appending it to the line the locator names. `insert-analysis-entry`
  (optional `document`) and `insert-experiment-entry` (required `document`) place it as above;
  `insert-claim-entry` places it given `document` and `locator` together (a claim's marker needs a
  locator to its block); given one without the other it refuses, naming the missing key.
- **Adoption** by `insert-claim-entry`, `insert-analysis-entry` or `insert-experiment-entry` is by
  title and host: given a `document`, it adopts a node of that title hosted there, else one of that
  title with no marker, which it then marks there; given no `document`, it adopts only a node of that
  title with no marker. Otherwise it mints. No insert adopts by title alone. An adoption that writes
  nothing is `unchanged`; one that marks an unhosted node is `done`.
- **The build** writes a marker for every claim, single-claim leaves and equations included
  (`claimgraph.writer.landLeaf` drops its `prose[id] || counted > 1` condition; an equation's marker
  goes on the closing line of the display-maths fence carrying its label, a block claim's on its
  locator's line as now).

---

## 5. Register pages

- **Placement is derived** (no record carries it). A claim, analysis or experiment is rendered on the
  register page of the volume its host leaf lies in: `<volume>/claim-rigor.md`, a volume being a
  top-level directory of `kb-root/` holding an `index.md` (SPEC §3); a host outside every volume puts
  it on the root page, `claim-rigor.md`. Works are rendered on the root page. An axiom is rendered on
  none (`axioms.md` states it). A node with no marker has no host, is rendered on no page, and is
  `verify`'s `statement marker` fault (§4).
- **Which pages exist** follows: a page for each volume hosting a node, and the root page where a
  node is hosted outside every volume or a work exists. A page no longer called for is removed by
  `refresh` and reported under `removed`.
- **Anchors.** Each entry is headed `## <title>`, so its anchor is `kb.Slugify(title)`. Links the page
  renders — a premise's title to its entry — are computed from each node's derived page and slug, so
  they hold within a page and across pages. A prose link `<page>#<slug>` holds while its node stays
  on that page with that title.
- **Dead links.** The dead-link gate checks the fragment of every link into a register page against
  the slugs of the entries derived for that page — from records, without reading the page — beside
  its file check. So a prose link to a node that moved to another page, or to a removed page, is
  reported in the linking document.
- **Tracked by git**, with the rest of `kb-root/`.
- **Never read back.** `claim-rigor.md` stays in `kb.ExcludeNames`; `verify`'s freshness check renders
  in memory and compares bytes, and a difference is a fault `refresh` fixes (`register freshness`,
  remedy `kbase refresh`).
- **Sessions** may read a register page as prose, take no value from it, and never edit it.
- **Content** (presentation; untested beyond the anchor rule and the freshness compare): a
  `# Claim register` heading and one sentence that `refresh` generates the file; per entry
  `## <title>`, `<!-- id: <id> -->`, the leaf-references line, `### Rigor` with `rigor` (marked manual
  where `manually-scored` is true), `rests-on` (every `supports` and `founds` edge into it, with the
  source's title, derived strength and bearing), `lifted-by`, `refuted-by`, `references`, `demoted`
  (each demoted edge into it, with its replaced relation and bearing), `rationale`, `strengthen-by`,
  `no-edge`; `### Strength` with the derived line and trace on a claim or analysis. An experiment:
  `status`, `rigor`. A work: `rigor`, `rationale`. Each scoring and demoted edge appears on its
  target's entry.
- **Order.** Hosted nodes in reading order of their marker (spine walk, then marker line), then works
  by id.

---

## 6. `manually-scored`

### 6.1 Writes

| Writer | `manually-scored` |
|---|---|
| A public write op (CLI or MCP), on a rigor or bearing its caller supplied, `*pending*` included | `true` |
| A public write op, on a rigor or bearing the caller left out (written `*pending*`) | `false` |
| `build`, and `score`'s landings and re-prices | `false` |
| Any op, on a value it does not write | unchanged |
| `set-rigor`/`set-bearing` re-asserting the current value on a `false` record | `true`, outcome `done` |
| An insert that adopts an existing node | unchanged (it writes no value) |

`rigor` and `bearing` are optional on every insert (`insert-claim-entry`, `insert-analysis-entry`,
`insert-experiment-entry`, `insert-work-entry`, and their pair bearings), `add-rests-on` and
`add-pairs`; an omitted value is written `*pending*`, with `manually-scored: false` where the type
carries the flag. A demoted edge's `bearing` and `manually-scored` are the replaced edge's, carried
unchanged by the demotion and by its restoration.

### 6.2 Provenance

`write.Options.Provenance`, never a values key, flag or schema property: `ProvenanceCaller`, the zero
value, which marks supplied values manual, so an internal caller that forgets it never lets a run
overwrite a person's value; and `ProvenanceMachine`, set by `claimgraph`'s write runner, `landScore`
and the re-price `Write` seam. A test asserts no command flag and no tool property names it.

### 6.3 Visibility

| Surface | Change |
|---|---|
| `show` | `manually_scored` after `rigor` on a claim, analysis, experiment |
| `deps` edge record | `manually_scored` after `bearing`; `null` where the type has none |
| `edges` record | the same |
| `stats` | after `subtree_aggregates`: `rigor_manual`, `rigor_model`, `bearing_manual`, `bearing_model`, over records whose type carries the flag, `*pending*` included |
| `edge-census` | each `bearings` record on `supports`, `lifts`, `refutes` gains `manual` and `model` after `edges` |

---

## 7. `score`'s skip

- `--rescore-manual none|rigor|bearing|both`, default `none`; help: "re-judge values marked manually
  scored: none, rigor, bearing or both" (pflag prints the default). A value outside the four is a
  usage refusal.
- At the walk, per item, from the KB the run loaded at its start: a `true` value is skipped unless the
  flag names the item's pass. The ledger records status `skipped`, reason `manually-scored`, with no
  value, charges or call, hence no capture.
- `counts` gains `skipped` after `failed` in the run document, `status`'s score shape and
  `Runner.counts`; `scored` and `failures` exclude it.
- The result's `resume` key is present when any item is `failed` or not reached; a `skipped` item
  does not make it present. A run started with `--resume <id>` asks a skipped item only where its own
  `--rescore-manual` names that item's pass. A resumed run appends no second `skipped` line for an
  item. The command in `resume` repeats any `--rescore-manual` the run was started with.
- `--rescore-manual` is per invocation and not in `run.yaml`: `Ledger.refusalFor` does not compare it.
- `score.Resume` takes, beside its `Write` seam, the predicate saying which items are now skipped;
  `Ledger.reprice` leaves such an item's landed line unrepriced.
- `--dry-run` evaluates nothing; `skipped` is `0`.
- A degraded KB is refused before the start refresh (§3.7). The start refresh is the migration save;
  a KB still unmigrated after it is refused before the first item (`check: index`), since a landing
  runs with `--no-refresh` and `write.Run` refuses that on an unmigrated KB.

---

## 8. The converter, 2.0.x → 3.0.0

`internal/migrate`: bytes in, bytes out, obsolete paths, no import of kbase outside
`internal/migrate` (`cmd/importpolicy_test.go`);
a tree of packages, one per superseded format and one per converter (`KB_3_WORK_PACKAGES.md`, "The
migrate tree"). `format_2_0_0` copies, as superseded parsers, the 2.0.x register reader (`internal/kb/register.go`'s bullet
reader with its `known`/`axioms`/`authored` filters), `kb.ParseLeaf`, `parseExperimentLeaf`,
`parseAnalysisLeaf`, `ParseAxioms` and the legacy readers, `kb.Slugify`, the id and token patterns
they use; `from_2_0_0_to_3_0_0` holds the transformation and a records emitter. Its output need only
load: the save re-renders both files through the record writer.

Its input is a KB the 2.0.x load accepts; a KB the 2.0.x load refuses is refused with the 2.0.x
load's own item, and the owner repairs it as a 2.0.x KB. Otherwise an upgrade never refuses.

### 8.1 Mapping

| 2.0.x source | 3.0.0 output |
|---|---|
| claim, work entry of a 2.0.x register | node record: `title` the heading, `rigor`, `rationale`, `no-edge`, `strengthen-by`, `manually-scored: false` |
| which 2.0.x register held the entry | dropped silently: the register page is derived from the host (§5). A node printed in another volume's 2.0.x register moves to its host's page; a prose link to its old anchor is reported by the dead-link gate (§5) |
| analysis: leaf `analysis-nodes` entry joined with its 2.0.x register entry | `AnalysisNode`: `title` the host leaf's first heading; where the 2.0.x register heading differs from that title, a warning naming both (that heading's text is lost) |
| analysis entry of a 2.0.x register with no leaf `analysis-nodes` entry | dropped with a warning — not a node in 2.0.x (`kb.Discover` joins only hosted analyses) |
| `experiment-nodes` entry | `ExperimentNode`: `title` the host's first heading, `status`, `rigor` (`"*pending*"` where absent), `manually-scored: false` |
| `axioms.md` heading, else a legacy declaration | `AxiomNode` |
| `rests-on`, `refutes`, `references` bullets; analysis pairs of a 2.0.x register; leaf pair lines | edge records oriented as the 2.0.x index row; `context` `""` where none |
| `demoted` bullet on entry X naming Y | `DemotedEdge` `source` Y, `target` X (the replaced `supports` edge's orientation), `replaced-relation: supports`, `bearing: "*pending*"`, `manually-scored: false` — 2.0.x's cut kept no bearing, so none is lost here; `(origin …)` dropped silently |
| a bullet the 2.0.x `known` filter drops (a token naming no node) | dropped with a warning quoting the bullet |
| `experiment-nodes` / `analysis-nodes` host | a marker on a new line directly after the host's first heading (§4) |
| `claims:` listings and markers | §8.2 |
| leaf and index frontmatter | removed; the entry point keeps `kb-format` alone |
| `kind`, `claims`, `no-claim` (string), `subtree-*` | dropped silently: markers and the derivation carry their facts |
| `experiments`, `path-stable`, a key no 2.0.x reader knows (hand-written `depends-on:`), a value of the wrong type (`no-claim: true`) | dropped with a warning naming file, line and key |
| 2.0.x register files | left as bytes; the save renders the register pages §5 calls for, overwriting a 2.0.x register at the same path, and removes a 2.0.x register at a path no page takes (reported under `removed`) |
| 2.0.x `.index/` files | `claims`, `cites`, `strengthen-by`, `subtree-aggregates` obsolete; `edges.yaml` replaced by the authored file |
| stamped `AGENTS.md`, `CONVENTIONS.md` | each 2.0.x stamped paragraph found verbatim replaced by its 3.0.0 counterpart (§13.1), the session rule inserted per §11; a document lacking an expected paragraph is left and warned. `migrate` holds both forms; a test in `internal/kbdocs` asserts its 3.0.0 paragraphs equal the templates' |

Every `manually-scored` is written `false`. The `migration` record's `note`: "every rigor and bearing
that score judges is now marked model-scored, so a default score run re-judges it; to keep one, re-assert
its current value with set-rigor or set-bearing, only where the user asks."

### 8.2 Markers from `claims:` listings

| 2.0.x state of a claim | 3.0.0 |
|---|---|
| listed in a leaf that marks it | that marker stands; nothing else |
| listed in one or more leaves, marked in none | a marker inserted in the first listing leaf in path order; the other listing leaves become mentions, each with a warning (their `referenced-by` row is lost) |
| marked in a listing leaf and in other leaves | the listing leaf's marker kept, the others removed, a warning each |
| marked only in leaves that do not list it (2.0.x ignored those markers) | listed elsewhere: a marker inserted in the first listing leaf in path order, as above; the ignored markers removed, a warning each. Listed nowhere: the first in path order kept as its marker, the others removed, a warning each |
| listed and marked nowhere | a node with no marker: `verify`'s `statement marker` fault (§4) |

A listing leaf that does not hold the kept marker is a mention: nothing records it. `$MC`'s
`clm-uu11c9`, listed in two leaves and marked in none, gets one marker and one mention.

**Insertion site**, mechanical, silent: an equation claim (`kb.EquationLabel(title)`), appended to the
closing line of the display-maths fence whose body carries `\label{<label>}`; a block claim in a leaf
listing one non-equation claim and holding one labelled blockquote (`kb.LabelLine`), appended to that
label line. Otherwise appended to the leaf's first heading line, with a warning: the statement is then
that line until `mark-claim-in-leaf` moves the marker.

### 8.3 Edge identities authored more than once

Equal bearings make one record, contexts joined with `; ` in authored order, silently (example:
`supports clm-aaa002 → clm-aaa001`, "first use" / "second use", `internal/sheet/testdata/adjagent-sheet`,
§17 E9). Different bearings keep the higher (a number above `*pending*`), with a warning naming both
sites. Cases: a `rests-on` target repeated; an analysis pair in both the 2.0.x register and the leaf; an
experiment's `supports` pair and a `rests-on` bullet naming the same experiment. SPEC §3's "of
`supports` edges sharing a source and target, one is drawn whatever their contexts" retires.

### 8.4 2.0.x register text

In a 2.0.x register, text outside every entry (H1, intros, legend tables, non-entry `##` sections, rules) is dropped
silently. Text inside an entry that is neither a recognised field, the derived footer nor a derived
line is dropped with a warning quoting it.

### 8.5 Warnings

A warning is an item (`path`, `line`, `detail`). The `migration` record (SPEC §7) gains `warnings`,
the list, on every call that carries it; each verb also writes them to stderr. `refresh`'s repair of
a degraded load (§3.7) reports its losses and its cycle cuts (§10) under a `warnings` key of its own
result; the build's cuts go to the `depends-attributed` stage report.

### 8.6 The save

Order: `.index/edges.yaml`, `.index/nodes.yaml`, leaves, stamped documents, then register pages and
sheets, then the obsolete paths, then the stamp. In a migration save `refresh` renders the pages after
the authored files (2.0.x's `index.RefreshReporting` writes the 2.0.x registers before `save`). A write op on an
unmigrated KB that touches either authored file writes `edges.yaml` before `nodes.yaml`.

Where `.index/nodes.yaml` stands, both authored files are 3.0.0's — 2.0.x never writes it and it is
written second — so the converter takes them as they stand, strips any remaining frontmatter without
reading it, and derives nothing from 2.0.x registers or 2.0.x frontmatter. Where it does not stand, `edges.yaml`
is 2.0.x's derived file and is ignored.

---

## 9. Every verb's change

### 9.1 Queries

Read `index.Derive`'s value instead of `.index/`; `query.Load` loses `requiredFiles`. `referenced-by`
and `citation_count` count markers where 2.0.x counted `claims:` listings; §6.3's keys. A converted,
unsaved KB answers queries, carrying `migration`. Refused while degraded.

### 9.2 Write ops

Every op reads and writes records and markers; the 2.0.x register splices, read-back provers and census
(`write.readRegister`, `takeCensus`, `spliceLines`, `replaceFieldLine`, `replacePremiseBearing`,
`editStagedPair`, `replaceDeclaration*`) retire. Read-back reloads the records and compares. Every
public op that adds a scoring edge refuses one closing a cycle (`check: dependency-cycle`, the path
through `index.DependsGraph.Path`), as `add-rests-on` and `resolve-demoted`'s restore do today;
`add-build-edges` does not, and the stage's walk (§10) cuts.

`--create` retires from every op, with its code: `write.CreatesRegister` and the `creates` field of
`write.opSpec`, the flag and its help in `cmd/write_op.go` (`cmd.Flags().BoolVar(&create, "create", …)`),
and the `create` schema property in `cmd/mcp.go` (`schema.Properties[createArgument]`).

| Op | 3.0.0 |
|---|---|
| `insert-claim-entry` | node record and its edges; the 2.0.x `register` value and `--create` retire (placement is derived, §5); optional `document` and `locator`, given together, place the marker in the same call, and one without the other is refused naming the missing key; adopts by title and host (§4); `rigor`, bearings optional |
| `insert-analysis-entry` | the same, `document` optional and no `locator` (§4) |
| `insert-experiment-entry` | node record and edges; required `document` and `title`; adopts by title and host (§4) where status, rigor and pairs also equal the supplied ones (2.0.x's full-equality rule, `write.adoptExperiment`, with the title added) |
| `insert-work-entry` | node record; the 2.0.x `register` value and `--create` retire (works render on the root page); adopts by key; `rigor` optional |
| `insert-axiom-entry` | **new**, help "record an axiom by label and title"; adopts by label, reporting a differing title |
| `set-rigor`, `set-rationale` | the record's field; §6.1 |
| `set-bearing` | the edge record's `bearing`; never creates an edge; refuses a `demoted` record |
| `set-status` | **new**, help "set an experiment's status: run or pending" |
| `add-rests-on` | unchanged vocabulary, bearing optional; `supports`/`founds`/`references` records |
| `add-pairs` | **new**, help "add scoring edges from an existing claim, analysis or experiment": `id` the source; lists of `{id, bearing, context}`, bearing optional — `refutes` from a claim (re-authoring a claim's refutation, which 2.0.x wrote only at insert), `lifts` and `refutes` from an analysis, `supports`, `lifts` and `refutes` from an experiment |
| `set-frontmatter` | **retired**: its keys are derived or removed (§2.1); node declarations move to the inserts, `set-status`, `add-pairs`, `set-rigor` |
| `mark-claim-in-leaf` | places or moves a node's one marker (§4); no `claims:` list to update |
| `drop-claim-edges`, `drop-inbound-edges` | the same edge sets and refusals, over records |
| `resolve-demoted` | values unchanged in meaning: `id` the node the replaced edge went into (the record's `target`), `target` the other end (the record's `source`); both may be `clm-`, `ana-` or `exp-`. `remove` deletes the record; `restore` writes the replaced edge exactly — `replaced-relation`, `source`, `target`, `context`, `bearing`, `manually-scored` — refusing one that would close a cycle (`check: dependency-cycle`). `resolved` reports `restored` with the relation rebuilt |
| `drop-claim-node` | removes the record, its marker and demoted records naming it; the "would empty a leaf's `claims`" refusal goes |
| `add-build-edges` (build-only) | records, `ProvenanceMachine`; no `origin`, no `demoted` list |
| `demote-claim-edge` (build-only) | replaces one standing scoring edge with its `DemotedEdge` (§3.3), carrying its relation and values; no `origin` |
| `render-citation` | unchanged |

### 9.3 `refresh`

Renders the register pages and sheets from `index.Derive`; writes no authored file except the
migration save (§8.6) and the repair of a degraded load (§3.7). `index.refresher.aggregates`,
`strength`, `leafReferences`, `emit` and the frontmatter splice retire. `removed` reports the
obsolete 2.0.x index files once and any register page §5 no longer calls for.

### 9.4 `verify`

| 2.0.x check | 3.0.0 |
|---|---|
| dead link, unknown id | kept; ids from `nodes.yaml`; a link into a register page also has its fragment checked against the derived slugs (§5) |
| frontmatter presence | inverted: a frontmatter block on any document but the entry point is a fault, remedy "remove the block: kbase reads no frontmatter here" |
| tier-1, tier-2 coverage, orphan reference, id uniqueness, bidirectional coverage | replaced by `statement marker` (§4) and the load's duplicate-id refusal |
| subtree drift, strength freshness, leaf-references footer, index freshness, index well-formed | replaced by `register freshness` (§5) |
| register binding, register census, rigor block, score value, analysis fan-out, experiment reference (all over 2.0.x registers and frontmatter) | retired |
| referential integrity, acyclicity | the degraded state: each duplicate identity, dangling edge and cycle an item, remedy `kbase refresh` (§3.7) |
| `demoted` findings | kept, `detail` naming the edge and its replaced relation, `path` the register page of the record's `target`, or `null` where that target has no host (no marker) — the missing marker is reported separately as the `statement marker` fault; the origin check retires |
| `axiom-source` findings | retired |
| — | **new** `axiom heading`: an axiom record without its `### Axiom <label>: <title>` heading in `axioms.md`, or such a heading naming no record with that label and title (§3.4); not refresh-fixable |
| unmigrated KB | kept, refresh-fixable |
| citation gate (`index.BuildVerify`) | `index.inScopeText` takes a document's kind from `kb`'s derived kind, not `kind:`; channel checks skip register pages; `foreignEdgeFindings` reads node records' `rationale`, `strengthen-by`, `no-edge` |

### 9.5 `build`

| Stage | Change |
|---|---|
| `start`, `document-graph`, `references-found`, `overview-drafted` | none; `overview-drafted`'s `kb.SheetDigestDrawnOf` reads the derived node rows |
| `spine-seed` | writes empty `nodes.yaml` and `edges.yaml`; `build.seedSpine`'s `CheckFrontmatterPresence` filter goes |
| `claims-declared` | `claimgraph.checkCleanliness` also refuses a non-empty `.index/nodes.yaml`; inserts and markers through `ProvenanceMachine`; `writer.writePlan`'s pass-2 `set-frontmatter` goes; `writer.landLeaf` marks every claim (§4), inserting each with `document` and `locator` so it is hosted in the call that mints it; `claimgraph.registerFor` (the 2.0.x register a document's claims went to) retires; `claimgraph.passTwoGate` takes hosting from markers and "determined" leaves from the node-pass and cull records |
| `claims-discovered`, `equations-minted` | as above; `claimgraph.readGraph` joins markers to records; `writer.writeWorks` writes work records |
| `claims-culled` | `IssueCulls` drops the node and its marker; `claimgraph.leafOf` reads markers, not `write.FrontmatterValues` |
| `depends-attributed` | edge records; the ring row (`claimgraph.ringRow`, today every `supports` edge on a cycle) becomes §10's walk, each cut issued through `demote-claim-edge` and reported as a warning in the stage report; `claimgraph.cycleEdges`, `demotedOrigin`, `inferredAt` and `buildrecords.InferredLinks` retire |
| `phase-3a` | refresh generates the pages; verify per §9.4 |
| a resume | refused while degraded |
| reproducibility | `claimgraph.readModuloIDsView` (and `internal/claimgraph/compat_test.go`) compares records and markers |

### 9.6 `score`

§7; the set comes from `index.Derive`; `score.declaringLeaf` reads markers; `rigorText` reads an
analysis's or experiment's section through its marker; landings and re-prices use
`ProvenanceMachine`.

### 9.7 `render-claim-graph`, `export-latex`, `init`, `status`

- `render-claim-graph`: `index.WriteSheet` hands `sheet` the derived rows (`sheet` never imports
  `index`, §17 E11); `sheet` stops reading the build records and drops its `buildrecords` and `asks`
  imports. One style for `supports`, one for `demoted`; no cited or inferred variant in strokes,
  tooltips, legend or digest. On an unmigrated KB it draws from the in-memory conversion, writes
  sheets only and carries `migration`. Refused while degraded. The placeholder digest (`sheet.Placeholder`:
  `.index/` files in sorted path order) needs no change.
- `export-latex`: works from the node records (`export.bibliography`). Refused while degraded.
- `init`: entry point stamped with `kb-format` alone; empty `nodes.yaml`, `edges.yaml`; refresh.
- `status`: the score shape's `counts` gains `skipped`.

### 9.8 MCP

Tools follow their commands. Resources unchanged; register pages are not resources. Instructions
(`cmd/mcp_instructions.txt`), plain text, then a blank line and the session rule (§11):

"The kbase skill carries the workflow; show-config reports how provider and model configuration
resolves. kb-root/.index/ holds kbase's own records, and the claim-graph sheets (claim-graph.svg,
claim-graph-digest.svg, a volume's claim-graph.svg) and every .dot file are kbase's derived output:
never read or write any of them. The query tools, show, deps, find, stats and the rest, report what
the records hold; the write tools change them; refresh and render-claim-graph write the sheets.
Testing that one of these paths exists (test -d, a stat) is allowed; listing, globbing or opening
anything under kb-root/.index/ is reading it. A claim-rigor.md page is the exception: refresh
generates it for reading as prose; take values from the query tools, never from it, and never edit
it."

---

## 10. Cycles: cut as the walk finds them

One function beside `index.DependsGraph`, `index.BreakCycles`, serves the build's ring row, the
degraded load's resolution and `refresh`'s repair, so all three cut the same edges.

- **The walk** is depth-first over the scoring edges (`supports`, `founds`, `lifts`, `refutes`),
  starting nodes in id order and following edges in identity order (§3.3: relation in `kb.Relations`
  order, then source, then target). Demoted records are not walked.
- **Each time the walk closes a cycle** — an edge reaching a node on the current path — it demotes one
  edge of that cycle: the first, in identity order, of the cycle's `supports` edges going into an
  analysis, where one stands; else the edge the walk is on.
- **It continues until no cycle remains.** After a cut it restarts from the beginning, so the outcome
  depends on the edge set alone. A merge may leave many cycles; each is cut.
- **Each cut** replaces the edge with its `DemotedEdge` (§3.3) and warns, naming the edge and the
  cycle it broke (the nodes in path order). Nothing is dropped.
- **Repair** of one cycle is at most two edits: remove or redirect another edge of the cycle, then
  restore the demoted edge with `resolve-demoted`, which refuses a restore that still closes a cycle.

Every public op that adds a scoring edge refuses one closing a cycle (§9.2), so outside the build only
a merge reaches this walk. SPEC §10 "The demoted relation" moves from cutting every `supports` edge on
a cycle to this rule.

---

## 11. The session rule

One paragraph, plain text with no Markdown characters, the last paragraph of
`cmd/mcp_instructions.txt` (its source) and a paragraph of `internal/kbdocs/templates/AGENTS.tmpl.md`
after the opening bullet list, before `## What this tree holds`:

"Rigor and bearing values are judgements, and a session does not make them. Set a rigor or a bearing,
its current value included, only to a value the user has given for that node or edge, in their own
words or quoted in your task. When you insert a node or an edge and the user gave no value, leave the
value out: kbase records it as pending for score to judge. A value you write is recorded as the
user's, and score skips it unless run with the rescore-manual option naming rigor or bearing. To have values
judged or re-judged, run score."

A test in `internal/kbdocs` reads `../../cmd/mcp_instructions.txt`, takes its last paragraph,
collapses whitespace runs in both texts, and asserts it is a paragraph of the template and holds none
of `` ` * _ [ # ``. It holds no copy of the text. A shared embedded file is not possible:
`cmd/mcp.go` may not import `kbdocs` (ARCHITECTURE §11).

---

## 12. The benchmark

`just bench-load` runs `BenchmarkLoadDerive` in `internal/index`, its log under
`test-data/transient/bench-load/`. Fixture: `internal/sheet/testdata/adjagent-sheet/kb-root`, converted
to 3.0.0 in Phase C (every node kind and relation; 14 nodes, 18 edges), measured as committed and
replicated ×40 in memory (about 560 nodes) through an in-memory `kb.Source` (`kb.MigratedSource`).
It times record decode, the marker scan and `index.Derive`; the landing reports ns/op and allocs at
both sizes beside 2.0.1's `verify` at ~0.10 s on `$MC`.

---

## 13. The contract

| Item | Ruling | New wording (outcome) |
|---|---|---|
| SPEC §3 byte-equal row "Metadata rendering …" | amended | "Statement markers and the entry point's stamp — the marker reader; `mark-claim-in-leaf`'s locate and splice." |
| SPEC §3 byte-equal row "`.index/*.yaml` and derived fields" | amended | "Register pages — `verify`'s freshness check. `.index/nodes.yaml`, `.index/edges.yaml` — the write ops' idempotence: whole files rendered deterministically." |
| SPEC §3 placeholder `index sha256` | kept | the `.index/` files in sorted path order, now the two authored files |
| SPEC §3 stamped documents; §11 instructions | amended | §9.8's text; `AGENTS.md` per §11 and §13.1; register pages are prose for people and sessions |
| SPEC §10 YAML dialect, "the JSON object of an index line" | amended | the clause retires, as does CONVENTIONS' index-stream departure; the dialect admits a block list of scalars as a mapping value |
| The one loader, `internal/kbload` | kept | reads `.index/` and documents; `migrate.Chain` gains 2→3 |
| Migration at load, landing on the next write | kept, amended | §8.6; "a converted but unsaved KB answers no query" retires |
| The converter's refusal and "never makes invalid bytes valid" | amended | the converter refuses only what the 2.0.x load refuses; otherwise it picks §8's outcomes and warns where data is lost or invented |
| `set-bearing` never inserts a token | amended | "replaces an existing edge record's bearing and never creates an edge" |
| `refresh` never rewrites a rigor it found | amended | "refresh writes no authored file but a migration save and a degraded repair"; the read-time clamp stays |

Other moving sections:

- **SPEC**: §1 metadata layer; §3 formats (`3.0.0`; `2.0.x` superseded), the sheet's edge styles and
  links (Ruling 19), the multi-context drawing rule; §5 a build records no judgement (+ `false`) and
  marks every claim; §7 keys (`score`, `status`, `show`, `deps`, `edges`, `stats`, `edge-census`,
  `refresh`'s `warnings`, `migration.warnings`, `check: degraded`); §8 idempotence and adoption, the
  op list, optional `rigor`/`bearing`, queries' source, `score`, `refresh`, `verify`; §10 nearly
  whole (the 3.0.0 description, edge authoring, R1–R5, the index row, the 2.0.x register vocabulary (→ register pages placed by host, §5), the
  legacy `AGENTS.md` paragraph, the `demoted` relation without origin, carrying the edge it replaced and cut by §10's walk, a new
  2.0.x → 3.0.0 migration section, the degraded load); §12 a 3.0.0 entry.
- **ARCHITECTURE**: I2, I8; §6 rows for `kb`, `index`, Write API, Sheet, Scratch creation; Frontmatter
  reading; Scoring's closed text, set and landing; §11 package map (`kb` gains the record loader and
  document kind; `migrate` the 2.0.x readers; `sheet` loses `buildrecords` and `asks`), the
  requiredness gate, the zero-value hazard; §12 format version; §14's limits on unattributed
  judgements, list bullets, edge direction in the bytes, and the reference comparison.
- **CONVENTIONS**: the index-stream departure; "read an index line only through `kb.ReadIndex`" → the
  record loader; the frontmatter-writer rule retires; a new house rule: provenance is an options
  field, never a flag or values key.
- **Stamped and user docs**: `AGENTS.tmpl.md` and `CONVENTIONS.tmpl.md` per §13.1;
  `README.tmpl.md` (~14–29, 58: its 2.0.x register wording — "a domain's register holds that domain's nodes" becomes derived placement — and axioms); `USER_CONVENTIONS.md` (~122, 130, 133, 146, 159,
  199: `set-frontmatter`, `claims`, `no-claim`).
- **Adoption by title and host; `--create` retired** (§4, §9.2): SPEC §8 "An insert creates a missing
  register only under `--create`" and its adoption sentence ("an insert adopts an existing entry of
  the same register and title"); SPEC §11 "plus `create` where the CLI offers `--create`";
  ARCHITECTURE §11 "(`create` lifted to `--create`)"; CONVENTIONS `## Testing`'s two
  `insert-claim-entry --create` mentions; `USER_CONVENTIONS.md` ~129–134 (adoption by register and
  title, and inserts creating a missing register only under `--create`).
- **ROADMAP**: "3.0.0: the derived index as one file" retires; jointly established units move to the
  next major after 3.0.0.

### 13.1 The stamped templates' new text

These are the 3.0.0 paragraphs the converter's restamp writes (§8.1) and its test pins.

`AGENTS.tmpl.md`, the bullet replacing "**Do not read kbase's derived files.**":

> - **Do not read kbase's records or its sheets.** `.index/` holds kbase's own records; the
>   `claim-graph.svg` and `claim-graph-digest.svg` sheets, a volume's `claim-graph.svg` and any `.dot`
>   file are its derived output. The query tools — `show`, `deps`, `find`, `stats` and the rest —
>   report what the records hold, the write tools change them, and `refresh` and
>   `render-claim-graph` write the sheets. Testing that such a path exists (`test -d`, a stat) is
>   allowed; listing, globbing or opening anything under `.index/` is reading it. A `claim-rigor.md`
>   page is the exception: `refresh` generates it for reading as prose; take values from the query
>   tools, never from it, and never edit it.

`AGENTS.tmpl.md`, the axioms paragraph:

> Corpus axioms are stated in `axioms.md` beside this file. Read it before reasoning about anything
> here. Where there is no such file none is stated, and nothing here is waiting on it.

`CONVENTIONS.tmpl.md`, "What this tree is", the first paragraph from its third sentence ("The
**claim graph** …") to the paragraph's end:

> The **claim graph** is a graph over results, of node kinds {node-kinds}, held as kbase's records in
> `.index/` and shown for reading on generated `claim-rigor.md` pages. Its four scoring relations —
> `supports`, `founds`, `lifts` and `refutes` — close no cycle: the derived value is undefined on
> one, so the build and `refresh` demote one edge of any cycle they find, and a write op refuses an
> edge that would close one. `references` and `demoted` edges are unscored; a `demoted` edge is a
> scoring edge demoted to break a cycle, kept whole so `resolve-demoted` can restore it. A leaf is a
> container — its position label says where it sits in the hierarchy — and the claims, analyses and
> experiments it hosts are marked in its prose.

`CONVENTIONS.tmpl.md`, its axioms paragraph:

> Corpus axioms — notation and cross-cutting definitions — are stated in `axioms.md`, one
> `### Axiom <label>: <title>` heading each, its label and title those `insert-axiom-entry` recorded.
> Prose may call an axiom an invariant or a framework premise; the graph has one word for it.

`CONVENTIONS.tmpl.md`, "Authored versus derived":

> Authored: leaf content and the statement markers in it, and kbase's records — nodes, their local
> `rigor`, rationale and the rest, and edges with their `bearing`. Derived and regenerated:
> `strength`, build status, subtree aggregates, the `claim-rigor.md` pages, and the
> `claim-graph.svg` and `claim-graph-digest.svg` sheets. **A hand-edited `claim-rigor.md` page is a
> verifier failure**, not a shortcut; `refresh` overwrites every derived file.
>
> Records and the statement markers are written only by a write op (`mark-claim-in-leaf` places or
> moves a marker), never by hand: you supply values, the op composes the record, proves what it wrote
> by reading it back, and refuses rather than writing what it cannot prove. It mints an entry's id
> itself and reports it, so no id is ever chosen. A values file carrying a key its op does not take is
> refused with the offending key, its position, and the complete set the op does take, so the call
> you would have made anyway is what answers the question.

`CONVENTIONS.tmpl.md`, "The gate", its first paragraph's last two sentences and two added:

> `verify` also lists every `demoted` edge under `findings`, which are not failures;
> `resolve-demoted` removes one or restores the edge it replaced, refusing a restore that would close
> a cycle. After a merge the load may be degraded; `refresh` repairs it, and its result's `warnings`
> name every edge it dropped or demoted. Report them: nothing else records a drop.

`CONVENTIONS.tmpl.md`, "Citations", the first sentence's opening:

> In authored files — indexes, summaries, `axioms.md`, and meta-docs — a reference to a node is a
> structured field, a marker, or a link, never an id or a rule name sitting loose in prose.

`CONVENTIONS.tmpl.md`, "Use the agents", the `kb-maintainer` bullet:

> - `kb-maintainer` — every change: new leaves, corrections, records, claim-graph wiring, and the
>   loop back to green.

---

## 14. The compat harnesses

- `internal/query/compat_test.go`: kept as the conversion parity harness — the reference run over a
  2.0.x KB against `./bin/kbase` over the same unsaved KB (§9.1). Expected differences: §9.1's
  counts, §6.3's keys, §8.3's merges. Flag-gated as now.
- `internal/index/compat_test.go`: retired; it compares outputs 3.0.0 no longer produces.
- `internal/kbload/parity_test.go`: a committed 2.0.1 case whose query goldens, recorded once under
  2.0.1, equal its converted form's modulo the same differences.

---

## 15. Tests and fixtures affected

- `internal/write/testdata/writeapi-render-golden`, `writeapi-regression` (2.0.x register splices → records).
- `test-data/fixtures/compat/ops.yaml` and `README.md`.
- `test-data/fixtures/mcp/initialize.jsonl` (instructions text).
- `test-data/fixtures/init/skeleton` (entry point, two empty files).
- `test-data/fixtures/results/kb-root`, and `justfile`'s results-fixture recipe body around its
  verify/refresh/render/insert legs (~1213–1216).
- `cmd/testdata/score-kb`, `score-kb-1.0.0`.
- `internal/sheet/testdata/adjagent-sheet` and its DOT goldens (one style per relation, §8.3 merge).
- `internal/migrate/testdata/register-2.0.0`, `bad-token-2.0.0`: become 2→3 golden pairs.
- About 40 `*_test.go` files carrying 2.0.x shapes (`git grep -ln 'claim-rigor\|kind: leaf\|set-frontmatter' -- '*_test.go'`).
- The `Short` of eleven existing ops, each op's `Long` moving with it (help is MCP tool description text):

| Op | `Short` |
|---|---|
| `add-rests-on` | add supports or founds edges into a node, or references edges from it |
| `drop-claim-edges` | remove a claim's supports, founds or references edges |
| `drop-claim-node` | delete a claim's node record and marker |
| `drop-inbound-edges` | remove every other node's edges naming one claim |
| `insert-analysis-entry` | mint an analysis id, write its node record and edges and, given a document, mark it there |
| `insert-claim-entry` | mint a claim id, write its node record and edges and, given a document and locator, mark it there |
| `insert-experiment-entry` | mint an experiment id, write its node record and mark it in its hosting document |
| `insert-work-entry` | record an external work by its citation key |
| `mark-claim-in-leaf` | place or move a claim's, analysis's or experiment's marker at a located line of a document |
| `resolve-demoted` | remove a demoted edge, or restore the edge it replaced |
| `set-rationale` | rewrite a node's rationale |

- The `Long` of the three inserts whose placement and adoption change:

`insert-claim-entry`:

> insert-claim-entry mints a clm- id, writes the claim's node record and edges in one act, and
> reports the id. Given document and locator together, it places the claim's statement marker at the
> located line; a claim with no marker is a verify fault until mark-claim-in-leaf places one. A claim
> with the same title is adopted rather than written twice: one hosted in the given document, else
> one with no marker, which is then marked there; with no document, only an unmarked claim is
> adopted. A refresh follows unless --no-refresh.

`insert-analysis-entry`:

> insert-analysis-entry mints an ana- id, writes the analysis's node record and edges in one act, and
> reports the id. Given a document, it places the analysis's statement marker on a new line directly
> after that document's first heading; an analysis with no marker is a verify fault until one is
> placed. An analysis with the same title is adopted rather than written twice: one hosted in the
> given document, else one with no marker, which is then marked there; with no document, only an
> unmarked analysis is adopted. A refresh follows unless --no-refresh.

`insert-work-entry`:

> insert-work-entry records an external work by its citation key: it writes the work's wrk- node
> record and reports the id. Works are rendered on the root register page. A work with the same key
> is adopted rather than written twice. A refresh follows unless --no-refresh.

Sequencing: `index.Derive` in memory and the `migrate` copy of the 2.0.x readers can land first.

---

## 16. Hand-offs outside this repository

These must land in the same release as 3.0.0:

- **adjagent** `definitions/kbase.py` (and kb-maintainer) use `set-frontmatter` and `claims`/`no-claim`;
  they move to `mark-claim-in-leaf`, the inserts' `document`, `set-status` and `add-pairs`.
- **adjagent**: whether any agent reads `path-stable`, which 3.0.0 drops.
- **Owner/adjagent**: kb-maintainer's CLI delivery of the new ops and the session rule.

---

## 17. Evidence

Run from the repository root; `$MC` as above.

| # | Command | Answer |
|---|---|---|
| E1 | `git grep -n 'FormatVersion *=' internal/kb/format.go` | `2.0.1` |
| E2 | `git grep -n -E '"(kind\|path-stable\|claims\|no-claim\|experiments\|subtree-claims\|subtree-experiments)"' -- internal cmd ':!*_test.go' ':!internal/migrate'` | the plan's keys plus `path-stable` and the `claimgraph`/`index` sites of §2.1 |
| E3 | `grep -c '"tier2_marked": false' $MC/.index/cites.yaml`; per id `grep -rl "claim-rigor: <id>" $MC` | 14 listings over 13 claims, each marked nowhere |
| E4 | `sed -n 340,345p internal/claimgraph/write.go` | the build marks only `if prose[id] \|\| counted > 1` |
| E5 | the 2.0.x register classifier below over `$MC/**/claim-rigor.md` | text outside entries in all 7 2.0.x registers; none inside an entry |
| E6 | `grep -rl '^experiments:' $MC --include='*.md' \| wc -l` | 0 |
| E7 | claims whose 2.0.x register directory is not their host's volume, from `$MC/.index/claims.yaml` and `cites.yaml` | 15 of 51 — the claims whose register page moves on conversion (§5) |
| E8 | duplicate `(relation, source, target)` in `$MC/.index/edges.yaml` | 0 of 97 |
| E9 | `grep -n 'clm-aaa002.*clm-aaa001' internal/sheet/testdata/adjagent-sheet/kb-root/.index/edges.yaml` | two `supports` rows, contexts "first use"/"second use", bearing `1.0` both |
| E10 | `git grep -n 'inferredAt(\|InferredLinks(' -- internal ':!*_test.go'` | callers: `claimgraph` ring row (origin) and `sheet.graph` only |
| E11 | `for p in query sheet index; do go list -f '{{join .Imports " "}}' ./internal/$p; done` | `index` imports `sheet`, not the reverse |
| E12 | `git grep -n -A10 'type DependsGraph' -- 'internal/index/*.go'` | every scoring relation enters the acyclicity graph |

E5's classifier: lines before a 2.0.x register's first `## ` are preamble; inside a `## ` section a line is
recognised if blank, `---`, an id marker, the footer, `### Rigor`/`### Strength`, a `- <field>:` of
`kb.RigorFieldKeys` or `lifts`/`no-edge`, or an indented continuation; a section with no id marker is
outside every entry.

**Incumbents used:** `write.parseValues`' node checks (record refusals), `atomicfile.Write`,
`claimgraph.documentKind`, `kb.AnchorSection`, `score.blockExtent`, `write.renderEntry`,
`renderWorkEntry`, `index.RenderLeafReferences`, `index.DependsGraph`, `kb.MigratedSource`,
`index.streamUnsafe`. No benchmark recipe or `func Benchmark` exists (`grep -n bench justfile`;
`git grep -n 'func Benchmark' -- '*_test.go'`).
