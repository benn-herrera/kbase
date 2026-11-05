### The `kb` package

**Pattern:** Domain model and schema package.

`internal/kb` is the model of `kb-root/` that every other package reads through. It contains:

- **Names and paths.** The names readers key on (`KBDir`, `EntryPointFile`, `AxiomsFile`, the sheet
  file names) and the paths leaf discovery skips.
- **`Source`.** The read path for every KB file: the disk at the current format version, or the bytes a
  migration produced for an older one, with obsolete paths reading as absent. `Put` and `Removed`
  record a writer's change so later reads in the same command see it. `internal/kbload` builds it
  ([§2](../2-invariants.md), I8; [§11](../11-package-map/loading.md)).
- **Records.** The schema types, `LoadGraph` (the two record files read through `Source`, decoded by
  `format400`, a rigor above `RigorMaximum` refused) and `EncodeNodes` and `EncodeEdges`.
- **Schema.** The node kinds, relation names, id prefixes, `BearingLegal`, `MintID`, the band ladder
  (`StrengthBandLadder`, `BandFor`), `AxiomRigor` and `DocumentKind`, which reads a document's kind
  from its child list (`ChildList`, `ChildListLines`).
- **Scoring faults.** The fault ids, the three weight tables and `FaultTable.Price` (`faults.go`).
- **Row types.** `NodeRow`, `EdgeRow` and `CiteRow`, which `index.Derive` yields
  (`indexstream.go`).
- **Link and text readers.** The link grammar (`LinkRE`, `Links`, `Citation`, `BareIDs`) is the one
  pattern the dead-link check, the tree's link readers and the converter read links through. The
  readers over a document's text:
  - `TextLinks`: every inline link outside code, with its line and its offsets on the raw line;
  - `TextLinkDefinitions`: every reference definition outside code, with its destination's offsets;
  - `TextCitations`: the `TextLinks` whose title is a node id;
  - `CitingDocuments`: every Markdown document but those at a summary page position. `citations-of`,
    `remove-node`'s refusal, the edits' renames and every op's re-pointing share this one reading
    (SPEC §8);
  - `CitationKeyRefs`: each use of a citation key as a key, outside code, with a blockquote's
    prefixes read past: a citation span's `data-cites` token or key-only rendering, a reference-list
    entry's id, a link's `#ref-` fragment. A work's label edit and a work's removal read it.

  `BibliographyEntries` returns a document's rendered reference-list entries. The dead-link check
  (`DeadLinks`, `DeadLinksIn`) reads links through the same grammar. `HeadingAnchors` computes every heading's anchor through the slugger (`Slugify`, `Slugger`).
- **Statements.** The marker reader and writer (`MarkerLines`, `StatementMarker`), the hosts it derives
  (`Hosts`), a hosted node's statement as the layout around its marker gives it (`Statement`),
  `FirstHeading`, `AxiomHeadings` and `LabelLine`.
- **Summary pages' rule.** The file name, title and entry headings (`RegisterFile`, `SummaryTitle`,
  `SummaryEntryHeadings`), the volumes among a KB's documents (`Volumes`), the page a host's entry is
  on (`SummaryPage`) and every path a page can be at (`SummaryPagePositions`: kb-root's and each volume's), which `internal/index`
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
enforce both ([§11](../11-package-map/dependency-rules.md)).

