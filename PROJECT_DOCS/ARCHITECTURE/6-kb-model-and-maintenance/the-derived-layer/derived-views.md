#### Derived views

`index.Derive` yields the rows in memory (`Rows`) from the records and the statement markers in the
leaves. The summary pages and sheets are views regenerated from them after every write and checked for
staleness.

- **Rebuild.** `renderRegisterPages` renders every summary page from the records and markers, never
  from a page already on disk. `Refresh` (load, pages, sheet) writes only the files whose bytes change
  and removes a page the KB does not call for. `PlanDerivation` is the same computation without writing,
  for a write batch. `UpdateFormat` is the save of a converted KB ([§12](../../12-constants/format-migration-spec-10/index.md)).
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

