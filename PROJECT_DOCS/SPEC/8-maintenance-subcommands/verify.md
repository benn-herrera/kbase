### `verify`

`verify` checks the KB and writes nothing. It takes no arguments and no lock. Its result is `findings`, then `refusals` where the outcome is `refused` ([§7](../7-tool-result-contract-and-exit-codes/item.md)).

**Faults.** Each fault is an item under `refusals`, with the check that names it:
- `dead-link`: a link under `kb-root/`, anchors and citations included, whose target names no file, or whose fragment names no anchor of the document it reaches.
  - A target resolves against the repository, taken as written; a target leaving the repository is no fault. A URL, a `~` path and a `.tex` target are not file links, and a link inside code is no link.
  - A fragment is live where it names a heading's anchor, or an HTML element's `id` outside code, in the document it reaches. A bare `#fragment` reaches its own document, and an empty fragment names the document's top. A summary page's live anchors are the entries derived for it, never the page as it is now. A fragment into a file outside `kb-root/`, or into one that is not Markdown, is not checked.
- `unknown-id`: an id in a citation's link title, or a node id in prose, that names no node. Prose ids are read outside code, of any kind. A placeholder id — a kind's prefix over `xxxxxx`, such as `clm-xxxxxx` — is never unknown.
- `bare-id`: a node id of any kind a record has, in every document but the summary pages, outside code and statement markers, standing in prose outside any link's text, title and destination, or as a link's whole text ([§10](../10-metadata-format-version/citations.md)).
- `frontmatter`: a frontmatter block on any document, the entry point included.
- `index-freshness`: a KB in an older format.
- `statement-marker`: a claim, an analysis or an experiment with no statement marker or more than one ([§10](../10-metadata-format-version/statements-and-anchors.md)).
- `axiom-heading`: an axiom whose record and `axioms.md` disagree by one of the three checks of [§10](../10-metadata-format-version/axioms.md). A marked heading is one, at any depth, whose line contains an axiom's marker.
- `register-freshness`: a summary page that does not match what the rebuild of the derived views renders ([§10](../10-metadata-format-version/documents-of-a-kb.md)).

**Remedies.**
- `index-freshness`: `kbase update-format`.
- `register-freshness`: the empty write batch ([§8](write-batches/index.md)), which rebuilds every page; over a KB in an older format, where that batch is refused until the conversion is saved, `kbase update-format`, which rebuilds every page too.
- `bare-id`: `kbase update-format`, which cites every bare id except that of a node with no statement marker; that node's `statement-marker` fault is cleared first ([§8](update-format.md)).
- `frontmatter`: remove the block.
- Any other fault: the user's git work or prose edit, guided by the fault's detail.

**Findings** are not faults: every `demoted` record is one (`check: demoted`, [§10](../10-metadata-format-version/the-demoted-relation.md)). `findings` is present on `done` and on a `refused` outcome that lists faults. A KB whose load fails is refused with the load error and has none.

The build's gate stage runs this same check.

Result keys: [§7](../7-tool-result-contract-and-exit-codes/index.md).

