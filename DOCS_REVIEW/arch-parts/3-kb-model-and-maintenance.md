## 6. KB model and maintenance

Each package and the subsection that states its mechanism.

| Package | Responsibility | Section |
|---|---|---|
| `internal/kb` | kb-root model, schema, record loader, link and marker readers | [§6](#the-kb-package) |
| `internal/write` | every write, as a batch of ops | [§6](#write-api) |
| `internal/index` | the rows `Derive` yields, strength, summary pages, `verify` checks | [§6](#the-derived-layer) |
| `internal/query` | read queries over the rows `Derive` yields | [§6](#the-query-layer) |
| `internal/sheet` | claim-graph sheets | [§6](#sheet) |
| `internal/kbdocs` | readiness documents and README assembly | below |
| `cmd/init.go` | KB creation | [§6](#scratch-creation) |
| `internal/score` | `kbase score` | [§6](#scoring) |

**Readiness documents (`internal/kbdocs`).** `Stamp` writes `AGENTS.md`, `CONVENTIONS.md` and the
`CLAUDE.md` redirect from `internal/kbdocs/templates/`, each only where absent, and refuses a
`CLAUDE.md` that is not the redirect before it writes. `Maintain` rewrites each such document's
maintained block to its template's: a document without exactly one ordered pair of fence lines is left
untouched and listed in `unfenced`, an absent `AGENTS.md` or `CONVENTIONS.md` is written whole, and
`README.md` is rewritten only where it exists. `AGENTS.md`'s scope pin is `AuthoredKBPin` where no build
record exists. `ComposeExcerpts` and `Overview` assemble the README's overview passage, and `NotProse`
checks the structure of a returned passage.

### The `kb` package

**Pattern:** Domain model and schema package.

`internal/kb` is the model of `kb-root/` that every other package reads through. It contains:

- **Names and paths.** The names readers key on (`KBDir`, `EntryPointFile`, `AxiomsFile`, the sheet
  file names) and the paths leaf discovery skips.
- **`Source`.** The read path for every KB file: the disk at the current format version, or the bytes a
  migration produced for an older one, with obsolete paths reading as absent. `Put` and `Removed`
  record a writer's change so later reads in the same command see it. `internal/kbload` builds it
  (§2, I8; [§11](#loading)).
- **Records.** The schema types, `LoadGraph` (the two record files read through `Source`, decoded by
  `format400`, a rigor above `RigorMaximum` refused) and `EncodeNodes` and `EncodeEdges`.
- **Schema.** The node kinds, relation names, id prefixes, `BearingLegal`, `MintID`, the band ladder
  (`StrengthBandLadder`, `BandFor`), `AxiomRigor` and `DocumentKind`, which reads a document's kind
  from its child list (`ChildList`, `ChildListLines`).
- **Scoring faults.** The fault ids, the three weight tables and `FaultTable.Price` (`faults.go`).
- **Row types.** `NodeRow`, `EdgeRow` and `CiteRow`, which `index.Derive` yields
  (`indexstream.go`).
- **Link and text readers.** The link grammar (`LinkRE`, `Links`, `Citation`, `BareIDs`), the links in
  a document's text (`TextLinks`, `TextLinkDefinitions`, `TextCitations`), `CitingDocuments`,
  `CitationKeyRefs`, `BibliographyEntries`, and the dead-link check (`DeadLinks`, `DeadLinksIn`).
  `HeadingAnchors` computes every heading's anchor through the slugger (`Slugify`, `Slugger`).
- **Statements.** The marker reader and writer (`MarkerLines`, `StatementMarker`), the hosts it derives
  (`Hosts`), a hosted node's statement as the layout around its marker gives it (`Statement`),
  `FirstHeading`, `AxiomHeadings` and `LabelLine`.
- **Summary pages' rule.** The file name, title and entry headings (`RegisterFile`, `SummaryTitle`,
  `SummaryEntryHeadings`), the volumes among a KB's documents (`Volumes`), the page a host's entry is
  on (`SummaryPage`) and every path a page can be at (`SummaryPagePositions`), which `internal/index`
  renders by, the build rebases titles by and the converter predicts. The renderers of the derived lines
  (`RenderStrengthLine`, `RenderBearingAnnotation`) are in `render.go`.
- **Format stamp.** `FormatFile`, `EncodeFormatStamp`, and `FormatStamp`, the superseded stamp in the
  entry point's frontmatter.
- **The "no node" refusal.** `NoNode`, `NodeLister` and `CheckUnknownID`, the single source of its
  text and the query its remedy points at.

`format400` defines the live format's declarations (node kinds, relations, id prefixes, the link
grammar, the marker's format, the slugger, the summary pages' rule) and `kb` re-exports them under
the same names (SPEC §10). Outside the formats tree only `kb` and `internal/kbload` import
`internal/formats`, and `kb` imports only `log`, `argmark` and `format400`; the dependency rules
enforce both ([§11](#dependency-rules)).

### Write API

**Pattern:** Transactional unit of work with dry run.

Every write is a batch (SPEC §8 Write batches), run by `write.Run`: `write.Batch` for `kbase write`'s
mixed batch, an op's name for a batch of that op alone. `Run` takes no lock; its caller holds the KB
write lock across it ([§8](#locks)). A batch reads the KB, applies every entry in memory, rebuilds the
derived views in memory, then writes each changed file once. The phases:

1. *Shape.* `parseValues` checks every entry against its op's fixed key vocabulary, `opFields` in
   `values.go`, the single source of per-op keys; `OpVocabulary` hands it to the help text and the MCP
   tool schema. A mixed entry's op is read from its `operation` key and must be in `Ops()`. Each op's
   own checks over its entries follow (`batchChecks`; an edit's `refuseFieldless`). Every fault is
   returned at once.
2. *Apply.* `Run` opens the KB through `kbload.Open`, reads it (`index.Derive` reads every document and
   both record files) and refuses what `Derive`, `index.PendingConversion` and `index.Precheck`
   refuse. `applyEntries` then
   loads both record files through `kb.LoadGraph`, keeping the bytes the disk held, and applies each
   entry in order to the batch's `kb.Graph` and to the leaves it marks (`leafEdit`), each judged
   against what earlier entries left. The first refusal stops the batch and names its entry. An
   entry's outcome is `done` or `unchanged` by whether the rendered record files or leaves changed under
   it. Facts only known while an op applies (a minted id, `Restored`, `RemovedEdges`, a citation's
   document, line and link) travel on `entryReport`, which `applyEntries` copies into `EntryResult`;
   the names an entry's values give are read from the values afterward. The record files are rendered
   whole through `kb.EncodeEdges` and `kb.EncodeNodes`, and a file whose bytes did not move is no
   change. That the decoder reads back what the encoder wrote is `format400`'s round-trip unit test,
   not a runtime check. `readBackMarkers` refuses a leaf whose composed text does not contain each marker
   the batch placed in it.
3. *Rebuild the derived views.* The changed record files and leaves go into the `kb.Source` overlay,
   unwritten, and `index.PlanDerivation` computes the summary pages and sheets from it, writing
   nothing. Every batch does this, the build's and the scoring run's own writes included (SPEC §8
   Writes leave nothing stale); no option skips it. The one exception is a batch of a build-only op
   that leaves only cycles among the scoring edges ([§6](#build-only-ops)).
4. *Write.* `writeChanges` writes `edges.yaml`, `nodes.yaml`, the leaves, then the derived files, each
   temp file then rename (`index.WriteKBFile`, `atomicfile`), and removes what the rebuild drops.
   A dry run (SPEC §8 Write batches) gives the same result with nothing written.

A failure in any phase before the write writes nothing. The write is atomic per file and not as a set:
a failure part-way is `failed`, naming what was already written (`interruptedError`), and recovery is git
(SPEC §6 Git holds the recoverable record).

**Pattern:** Optimistic concurrency check (compare before write) layered on a lock. Before the write,
`stillMatches` compares each file with the bytes the batch read; a difference is `retry`
(`RetryRemedy`) with nothing written. For a committing batch the comparison runs again once
`Preflight` returns, so a hook that edited a file meanwhile turns the batch into `retry`.

A committing batch (the document's `commit`, or `write.Options.Commit`; SPEC §8 Write ops) calls the
caller's `write.Options.Preflight` before its first write, and its error stops the batch with nothing
written. A committing dry run calls it too, which opens the repository and commits nothing.
`cmd/write_op.go` supplies it: `kbCommits.preflight` makes the snapshot commits of `kb-root/`'s
uncommitted changes outside `.index/` (owned `kb-root` less `gitledger.Exclude("kb-root/.index")`),
then those inside it, each only where `Repo.Dirty` finds any. Once `write.Run` returns `done`,
`kbCommits.final` commits `kb-root/` under the label `batchLabel` composes from the entries' ops,
their ids (`EntryResult.ID`) and the ids their values name (`EntryResult.Names`). Each commit's
`paths` are what `Repo.Paths` reads back from it.

The remaining mechanisms are per op family. No op accepts a preformatted record or marker: every value
is a values key, and the marker is `kb.StatementMarker`'s. `kb.Bearing` keeps `Value` and `Pending`
apart, so a written `0` reads back as `0` and a written `*pending*` as pending. An op is one per node
kind for `add-<kind>` and `edit-<kind>`, whose keys differ by kind in `opFields` (an experiment's
`status`, a work's and an axiom's `label`); `remove-node`, `move-node` and `add-citation` take any
kind by `id`.

#### Adds

An add fails if the item exists (SPEC §8 Idempotence) and upserts nothing. Against the records as the
batch's earlier entries left them, it refuses a title its kind already has (`titleHolder`) and, for a
work or an axiom, a label its kind already has (`labelHolder`); then it mints the id (`kb.MintID`) for
every kind. A
hosted node's marker is placed through `markAtLine` once `checkNamedLine` has refused a line past the
end, a fence's opening or closing line, and a line without the snippet.

A hosted add and `add-work` run through `keepSummaryLinks`, which updates references to every summary
entry whose repeat suffix the new entry shifts, comparing `anchored` before and after. `summaryMayShift`
first rules out, without a rebuild, an entry whose slug (repeat suffixes stripped) no other title and
no fixed rigor or strength heading shares, so a build's many adds rebuild nothing extra.

`add-axiom` marks the one unmarked heading of its text in `axioms.md`, or appends its marked heading,
and refuses two unmarked headings of its text. `axioms.md` travels with the leaves as a `leafEdit`.

#### Edits and renames

An edit (`edit-<kind>`) replaces the fields its entry gives, through `nodesOf`, which presents each
kind's records as their editable fields. Title uniqueness is judged by `titleHolder` and label
uniqueness by `labelHolder`, as in an add.

**Pattern:** Referential-integrity maintenance (rename refactoring). A title or label change
(`rename`) keeps every link into the node valid in the same batch. It reads each work's and axiom's
link target and every node's summary entry before and after the change, from `index.Derive` over a
source holding the batch's records and documents as earlier entries left them (`anchored`, one rebuild
each side). It then rewrites every document but the summary pages (`eachDocument`: `kb.CitingDocuments`
and each document an earlier entry created), each document it changes a `leafEdit`.

- `edit-axiom` rewrites its marked heading in `axioms.md` (`rewriteAxiomHeading`).
- A citation in the naming form takes the new title through `renameCitations`, which refuses at
  `title`, naming the citing `path:line`, a rewritten link that `readsAsOneLink` does not see whole.
- A work's new label replaces every use of the old key as a key (`rekey`, over
  `kb.CitationKeyRefs`), after `refuseKeyInUse` refuses a label that a citation span or a
  reference-list entry already uses.
- These rewrites change lines in place, so `movedHeadings` finds each heading whose anchor changed by
  comparing `kb.HeadingAnchors` line by line, before and after, and `movedAnchors.summaries` adds each
  summary entry whose anchor changed. `repoint` then gives every link outside code, citation or not,
  inline link or reference definition, one destination computed from the one it had before: the
  node's new target where a citation of it pointed at the old, otherwise the changed heading's or
  entry's new anchor, the fragment alone changing. The map is read once per link, so no link is
  updated twice and a shifted repeat suffix (`on-x-1` to `on-x`) is never chained.

#### Edge ops

`add-edge`, `edit-edge`, `remove-edge` and `restore-demoted` look an edge's ends up in the batch's
records (`edgeEnds`, refusing through `noNode`, the "no node" refusal the edits share) and its
identity through `holds` and `noEdge`. `Joins` decides whether a relation may join two kinds:
`references` any, a scoring relation what `kb.BearingLegal` allows, `demoted` none.

**Pattern:** DAG insert with cycle check (path finder). `index.GraphDependsGraph` builds the premise
graph over the scoring edges, and `index.DependsGraph.Path` is a shortest-path search over it.
`closesCycle` runs the search for every public op that adds a scoring edge and for `restore-demoted`,
and refuses an edge it finds closing a cycle, naming the path in edge direction (it reverses the
premise graph's path). The premise graph is built once per batch and cleared by every op that takes a
scoring edge out of the records or changes one (`dropEdges`, `demoteEdge`, `splitClaim`). `addEdge` deletes the `demoted` record (`kb.DemotedMatch`)
of a scoring edge it writes, so an add over a removed edge writes the edge with the add's values.

A scoring edge's judgement is written by `add-edge` and `edit-edge` through `bearing`, and a node's
local value by `add-<kind>` and `edit-<kind>` through `rigor` (SPEC §8 Values input).

#### Removal and move

`remove-node` reads the rows of the edges it removes from `derived` (`index.Derive` over the batch's
records and documents, which `targetOf` shares), drops them through `dropEdges`, and takes the record
and the marker out through `dropNode` inside `keepSummaryLinks`, or an axiom's headings out of
`axioms.md` through `dropAxiomHeadings`, which updates references to every later heading whose repeat
suffix the removal lowered. It then refuses, through `refuseMentions`, a mention that outlives the
node:

- a citation in any document but the summary pages (`eachDocument`, shared with `renameCitations`,
  `rekey` and `repoint`);
- for a work, a citation span with its label as the key where no reference-list entry has that key;
- a surviving record's prose field (`graphProse`, `Mentions`).

`move-node` checks the named line through `checkNamedLine`, places the marker with `appendMarker`,
takes every other occurrence out (`unmarkEverywhere`, keeping the named line), and updates every
citation, through `repoint`, to `targetOf`: a hosted node's host and its marker's section anchor
(`index.SectionSlug`, the slug the rebuild of the derived views uses).

A document the batch edits is a leaf as `index.Derive` reads it, by its child list (`leafAt`), so a
citation added to a leaf leaves it a leaf.

#### Citations

`add-citation` opens any document of the KB as a `leafEdit` (`citingDocument`), finds the one span of
the named line its snippet names outside code, links and markers (`citedSpan`; a missing span is
refused through `snippetAbsent`, which `checkNamedLine` shares), and links it to `targetOf`. It refuses
a link the reader would not see whole (`readsAsOneLink`) and checks a quote against `kb.Statement`,
the cited node's statement read from its host leaf by the layout around its marker, the `lifts` edges
the batch contains deciding the split layouts. A link spliced into a heading changes its anchor, so
`add-citation` runs `movedHeadings` and `repoint` after the splice, the new link among those updated.

### Provenance

**Pattern:** Manual-override flag.

`write.Options.Provenance` says who supplied the values a call writes and decides the `manually-scored`
flag each rigor and bearing is written with (SPEC §10 Provenance and the judged hash). `ProvenanceCaller`,
the zero value, sets the flag on every value its caller supplied, so an internal caller that names
none never lets a run overwrite a person's value. `ProvenanceMachine` writes `false`. The writers that use
it are `claimgraph`'s write runner, `internal/score`'s writing of results (`landValues`, a score run
and a recompute alike; [§6](#writing-the-results)) and the build-only `add-build-edges` and
`split-claim`, each a `recordEdit` marked `machine`.

Provenance is an options field and never a values key, a flag or a schema property:
`TestNoSurfaceNamesTheProvenanceOrCreate` (`cmd/mcp_test.go`) asserts that no command flag and no
tool property names it.

A rigor or bearing that an add or an edit is given records the judged hash of the text as the entry
leaves it (`valueSet`, `hashValuesSet`, over `asks.NodeJudgedHash` and `asks.EdgeJudgedHash`). A
pending value set by a machine records none.

No build path writes a judgement (SPEC §5 A build records no judgement): `add-build-edges` writes
`*pending*` on every edge it adds and no number anywhere, and `split-claim` writes `*pending*` on the
analysis's rigor and on its lift.

### Build-only ops

**Pattern:** Internal privileged ops.

Three write ops are the build's alone: `add-build-edges` (`write.AddBuildEdges`), `demote-claim-edge`
(`write.DemoteClaimEdge`) and `split-claim` (`write.SplitClaim`). They are absent from `write.Ops()`,
have no subcommand and are refused in a mixed batch, so the public write operations cannot create or
retarget a `demoted` record. `restore-demoted` is public, and `remove-edge` removes a `demoted` record.

- `add-build-edges` writes the build's edges as `add-edge` writes one, without its cycle refusal. A
  `rests-on` premise `founds` the node where `kb.BearingLegal` allows `founds` from its source and
  `supports` it otherwise; `references` are accepted on a claim only. An edge the records already
  contain is left as it is, and an edge with a `demoted` record of its relation is written with these
  values and the record deleted.
- `demote-claim-edge` replaces one standing scoring edge, named by its `source`, `target` and
  `relation`, with the `kb.DemotedEdge` that keeps it whole. Where only that record exists the edge
  is left as it is; an edge with neither is refused.
- `split-claim` handles one claim. Where an analysis already lifts the claim it uses that one;
  otherwise it mints an analysis and places its marker at a given line of a given leaf, and, where
  given a `statement-end`, relocates the claim's marker to that byte offset of line `statement-line`,
  or of the claim's own line where none is given. It retargets every `supports` and `founds` record
  into the claim, and every `demoted` record that replaced a `supports` edge into it, onto the
  analysis, and adds the analysis's `lifts` of the claim at `*pending*`. It runs no cycle check: a
  path through the analysis is the path that ran through the claim.

`add-build-edges` and `demote-claim-edge` run over records holding a cycle among the scoring edges
(`index.DeriveOverCycles`). The build's edge batch closes such a cycle and cycle breaking
([§6](#cycle-breaking)) removes it within the same stage: its removals reach the records through
`demote-claim-edge`, at the tail of `depends-attributed` (§4). `split-claim` is
applied by `analyses-split`. A batch of `add-build-edges` or `demote-claim-edge` that leaves records
the rebuild refuses for a cycle among the scoring edges alone (`holdsOnlyCycles` in `ops.go`) is
written without its rebuild of the derived views, because the rebuild cannot read such records; the
cycle-breaking batch after it rebuilds them before the stage's boundary.

The build's other writes are `add-claim`, `add-work`, and `remove-node` and `edit-claim` for culls.
None of the build's own writes takes the KB write lock; the build takes it once, at start, to wait out
a write in progress ([§8](#locks)).

- **Culls.** A cull is one mixed batch of `remove-node` entries, ordered so that a culled claim whose
  rationale mentions another is removed first (`write.Mentions`, the test `remove-node` refuses by).
  Where culled claims mention each other round a loop, an `edit-claim` entry clears a rationale first
  (`cullEntries` in `internal/claimgraph/elision.go`).
- **Titles.** A title its kind already has takes a counter, ` (2)` on (`uniqueTitle` in
  `internal/claimgraph/write.go`). An `add-claim` entry whose title a claim in its document already
  has takes that claim's id and writes nothing.
- **Block claims.** A block claim given a counter does not meet its block by title in `readGraph`. It
  joins the block of its leaf whose display line its marker is on, where no claim of that leaf has
  the block's own title. That line is the one `add-claim` was given, found by the same
  `write.ExcerptLines` search within the block's extent: an exact, positional match, never the
  counter read back.

### The node vocabulary

**Novel.** What a claim graph is made of, THESIS "What is new" item 1: five node kinds and four
scoring relations, defined in SPEC §10 Five node types.

**Pattern:** Prefixed random ids.

`kb.NodeKinds` is five words, `claim`, `analysis`, `experiment`, `axiom` and `work`, and nothing else is
a `node_type` or a `target_kind`. `kb.BearingLegal(relation, sourceKind, targetKind)` is the single
source of whether a pairing may have a bearing: every scoring pairing may, `references` and `demoted`
may not. It answers that and never whether an edge may exist.

Every node id is minted: its kind's three-letter prefix over a body `kb.MintID` draws, `kb.HashLen`
characters uniform over `kb.HashAlphabet` and redrawn while taken. One grammar serves every kind
(`kb.IDBody`, over `kb.IDKinds`). Each prefix is a `format400` constant re-exported through `kb`
(`kb.ClaimPrefix`, `kb.AnalysisPrefix`, `kb.ExperimentPrefix`, `kb.WorkPrefix`, `kb.AxiomPrefix`);
`kb.NodeIDKind` reads an id's kind off them, and `kb.NodeIDs` finds an id of any kind in text. A KB
converted from `3.0.0` has its work and axiom ids minted by the migration ([§12](#format-migration-spec-10)).

A work's citation key and an axiom's label live in the record's `label` field (`WorkNode.Label`,
`AxiomNode.Label`), never in the id; the strict load requires a work's label to match `kb.WorkLabelPattern`.
An axiom's prose is `axioms.md`, its heading paired with its record by the axiom's marker.
`kb.AxiomHeadings` reads each heading containing an axiom's marker, and each unmarked `### Axiom `
heading, for `add-axiom`, `edit-axiom`, `remove-node` and `verify`'s `axiom-heading` check.
`kb.AxiomAnchor` gives `index.Derive` an axiom's anchor, that of its marked heading. A statement marker
hosts only a claim, an analysis or an experiment (`kb.HostedIDKinds`); an axiom's or a work's id in a
marker hosts nothing.

The legacy declaration forms are the superseded readers' alone (`internal/formats`). SPEC §10 is the
single source of the prose legend the authors' words map through.

### The derived layer

**Novel.** The formula that combines local judgments into a strength for every node, THESIS "What is
new" item 3, defined in SPEC §10 Four scoring relations, one arithmetic.

**Pattern:** Topological fold over a DAG (Kahn's order); materialized views rebuilt after each write.

#### Strength

`internal/index/strength.go` is the arithmetic's authority: the four relations, their three stations,
the pending rule and the value's two formats live there. [STRENGTH_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/strength/entry-point.md)
states the relations, the formula and its worked examples and is cited, not reproduced.

| Station | Relations | Function |
|---|---|---|
| minimum | `supports`, `founds` | `MinDependencyStrength` |
| maximum | `lifts` | `liftStrength` |
| cap | `refutes` | `capStrength` |

`EdgeContribution` is one edge's value, nil where its bearing or its source's strength is pending. Each
station function reduces it over its edges, omitting an excluded one; `ExperimentTakesPart` excludes
an experiment that is not `run`. `derive` composes one node's result. `graphStrength` fills a
`strengthWalk` from the records and `run` computes each claim into `Strength.Results` and each
analysis into `Strength.AnalysisResults`, in `derivationOrder`, which is Kahn's order: a node after
every node its premises name, from sources to sinks. Nodes a cycle keeps out of that order, and nodes
resting on one, derive nothing; the load refuses such records ([§6](#cycle-breaking)).
`RenderStrengthTrace` composes the station text for a trace. The `trace` query (SPEC §8 `trace`)
reports `EdgeContribution` per edge, `internal/query` reading each source's strength off `Derive`'s
node rows and leaving an experiment that is not `run` out as the propagation does.

The propagation computes at full precision, so a dependent multiplies its source's exact strength, and
nothing else rounds a strength. `FormatStrengthRecord` writes the value in the fewest decimals that
read back as it (a summary page's strength line and premise annotation), and a query result contains
the value `FormatStrengthDisplay` formats, where a trace shows it.

#### Derived views

`index.Derive` yields the rows in memory (`Rows`) from the records and the statement markers in the
leaves. The summary pages and sheets are views regenerated from them after every write and checked for
staleness.

- **Rebuild.** `renderRegisterPages` renders every summary page from the records and markers, never
  from a page already on disk. `Refresh` (load, pages, sheet) writes only the files whose bytes change
  and removes a page the KB does not call for. `PlanDerivation` is the same computation without writing,
  for a write batch. `UpdateFormat` is the save of a converted KB ([§12](#format-migration-spec-10)).
- **Anchors.** A summary page's entry anchors are computed at derive (`pageAnchors`): the page's title
  and every entry's headings (`kb.SummaryEntryHeadings`) through one `kb.Slugger` in page order. A
  claim's and a work's canonical anchor, a page's links and the dead-link check's page slugs read
  that one map, and a test holds the rendered page's headings to it.
- **Staleness.** `Verify` is the standard check: `LinkFindings`, `BareIDFindings`, then
  `MetadataFindings` (a frontmatter block, a KB loaded at an older format as `index-freshness`,
  statement-marker faults, disagreements between axiom records and `axioms.md`, and
  `RegisterFreshness`, a finding per summary page that is missing, differs, or has no node rendered on
  it). The build's `phase-3a` runs `Refresh` and `Verify` (`internal/claimgraph/gate.go`).
- **Remedies.** Each `Finding` has its own: `update-format` for `index-freshness`; the empty write
  batch (`EmptyBatchRemedy`) for `register-freshness`, but `update-format` over a KB whose conversion is
  pending, where the batch is refused; a write batch of an `add-citation` entry for `bare-id`.
- **Links.** `LinkFindings` checks a citation's title id among the ids it checks (`kb.CheckUnknownID`).
  It hands `kb.DeadLinksIn` the summary pages' derived slugs, which turns the fragment check on: a
  fragment into a summary page is judged against those slugs, one into any other Markdown document
  under `kb-root/` against its heading anchors and element ids (`documentAnchors`), read once per
  document. `kb.DeadLinks`, which the build's document-graph and conformance checks call, checks files
  alone.

### Cycle breaking

**Pattern:** Greedy feedback-arc removal by DFS back edges.

`index.BreakCycles(g, hosted)` returns the graph with no cycle among its scoring edges, the edges it
removed in order, and a warning per removal. Each removed edge becomes its `kb.DemotedEdge`. SPEC §10
Cycles: cut as the walk finds them defines the rule; the mechanism is:

- The node order is `hosted` (`index.HostedOrder`, the order the summary pages render nodes) and then
  every other node by id; an edge sorts by its relation's place in `kb.Relations`, then its source's,
  then its target's.
- The search is depth-first over the `supports`, `founds`, `lifts` and `refutes` records, from every
  node in turn, following each node's edges in that order (`closeCycle`); `demoted` records are not
  followed.
- An edge reaching a node on the current path closes a cycle. The edge removed is the first, in that
  order, of the cycle's `supports` edges into an analysis, else the closing edge.
- After each removal the search starts over, so the removals depend on the edge set and the node
  order alone.
- The removal set is not a minimum feedback arc set.

Three callers use it:

- `Derive` runs it over every load and only detects: a cycle in loaded records is a load error, one
  item per edge the search would remove (`deriveGraph`, `loadDerivation`).
- `DeriveOverCycles` accepts such records and computes strength over `Rows.Resolved`, the graph with
  those edges removed.
- The build's cycle breaking (`ringRow` in `internal/claimgraph/stages.go`) takes the removals over
  `index.HostedOrder` and applies them in one `demote-claim-edge` batch, in removal order, because the
  search computes every removal over one graph before any is applied.

Public ops refuse an edge that would close a cycle ([§6](#edge-ops)), so only the build breaks cycles.

### The query layer

**Pattern:** Read model.

`query.Load` is the rows `index.Derive` yields, as an `Index` sorted by id; `Derive`'s refusals are
`Load`'s, and a KB at an older format answers as converted. Queries read the `Index` and nothing else.
`query.ScoringSet` builds the scoring set ([§6](#the-scoring-set)).

`query.EdgesPayload` is the single incidence adapter. It selects the `Index`'s edge rows by relation and,
where `EdgesSelection` names a node, by `Direction` (`in` the rows whose target it is, `out` those whose
source it is, else both), and presents each as `query.EdgeRowRecord`: `source`, `target`, `relation`,
`bearing` (a number or the pending literal, null only where the pairing has none), `context`,
`manually_scored` and `replaced_relation`. The order is fixed: relation, then source and target
(SPEC §8 `edges`).

An answer is the whole selection: no list query slices it, and `find` and `trace` truncate only where
`--limit` is given beside `--sort` (`rankUsage` refuses a limit without a sort). No query verb returns
statement text: the rows contain none.

`contribution` in a `trace` row is `index.EdgeContribution`, never a restated formula (`tracedEdge`).

### Frontmatter reading

**Pattern:** Heuristic sniffing before parse.

`kb.FindFrontmatter` takes a leading `---` block as frontmatter only when its first non-empty line is
key-shaped (`name:`, kebab-case). A block that then fails to parse as a YAML mapping is a
`MalformedError`, the malformed-frontmatter refusal; a block whose first line is not key-shaped is
prose, the document having no frontmatter (SPEC §10 Recognising frontmatter).

The one frontmatter value kbase reads is the superseded stamp in the entry point (`kb.FormatStamp`),
which `internal/kbload` reads only where `kb.FormatFile` does not exist ([§11](#loading)). `verify`'s
`frontmatter` check uses the same recogniser to report a block on any document, the entry point
included. The build's conformance check (`claimgraph`) refuses any document whose frontmatter has a
field.

### Sheet

**Pattern:** Graphviz rendering with a bounded process pool; a materialized view.

`internal/sheet` emits Graphviz DOT and renders it by running `dot -Tsvg` on `PATH`. The SVG's bytes are
Graphviz's, so the goldens pin the DOT; a test that renders through `dot` skips without it and asserts
only that the root is fitted to the viewer. The goldens are a fixture and its three DOT files under
`internal/sheet/testdata/adjagent-sheet/`: a `3.0.0` KB and the DOT the sheet composes for it, with a
build record beside it that the sheet does not read. `TestAdjagentSheetGoldens` compares the emitted
DOT with them byte for byte.

The inputs are the rows `index.Derive` yields (nodes, edges, strength bands, cites), which `index`'s `planSheet` hands
it from `index.Derive`, never a read of `.index/`:

- the `demoted` rows, for the removed edges, and one style per relation;
- each node's `CanonicalPath` and `CanonicalAnchor`, for its link;
- the leaf, for a node's kind, with the title deciding where no marker is readable;
- the entry point's first heading, for the title and the cluster of nodes no volume hosts;
- each volume's `index.md` first heading, falling back to the directory name, for the volume's
  cluster title.

It reads no build record. Its non-test code imports `kb` alone today; the dependency rules only keep it
below the build ([§11](#dependency-rules)).

It produces the full sheet always, and the overview diagram and one sheet per volume where two or more
volumes contain nodes (SPEC §3 `claim-graph.svg`). It renders through a pool of `dot` processes bounded by
`sheet.renderConcurrency` (§12), keeping output order. `dot`'s spline router can fail
on a dense sheet, and straight lines bypass it, so the emitter then composes the same DOT with
straight-line splines and renders it once more; where both fail, the first error is reported. Without
`dot` it renders nothing, leaves existing sheets as they are, and yields the placeholder only for a root
that has no sheet.

Every rebuild of the derived views (`index.UpdateFormat`, every caller of `index.Refresh`: `init`,
`score`'s start rebuild, the build's `phase-3a`; and a write batch's `index.PlanDerivation`) reaches the
sheet through one interface, `index`'s `planSheet`, which renders every sheet on each call and yields
one only where its bytes change, with no ownership test on an existing sheet. A rebuild writes what it
yields; a batch writes it with its records. `verify` does not reach it.

### Scratch creation

**Pattern:** Scaffold command.

`cmd/init.go` is the only producer of a KB that is not the build. It composes existing mechanisms and
owns none of its own:

- the format stamp through `kb.EncodeFormatStamp`, the plain entry point, and the two empty record
  files through `kb.EncodeNodes` and `kb.EncodeEdges`, every file through `atomicfile.Write`;
- the loader's `Open` (§2, I8);
- the readiness documents through `kbdocs.Stamp` with `kbdocs.AuthoredKBPin` in its scope-pin slot, its
  `written` list being the paths `Stamp` returns;
- the derived views through `index.RefreshReporting`;
- the KB write lock through `holdKB`, which takes `write.LockKB` from before `kb-root/` exists
  ([§8](#locks));
- the create commit through `gitledger.Repo.Commit`, the plain-subject half of `gitledger` beside
  `Record`, its pathspec scoped the same so nothing the user staged rides along.

It refuses a multi-line `--title`, a directory outside a worktree and any existing `kb-root/` before
any write, and decides create-once again under the lock (SPEC §8 `init`). Its order is load-bearing:
the entry point and the stamp file exist before `Open`, the readiness documents before the rebuild,
and the rebuild before the commit. The commit's subject, `kb-init: knowledge base created`, is outside
`gitledger.Prefix`, so the KB has no checkpoint commits and a resume finds nothing to resume.

`init` runs no `verify` of its own: every input to the scaffold is kbase's, so a failing `verify` over
it is a kbase defect, which `test-integration-init` catches by running `verify` right after creation.
