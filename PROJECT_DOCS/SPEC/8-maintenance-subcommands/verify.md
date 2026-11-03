### `verify`

[Page §3.6 `verify`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md) is the contract: its
faults and their check names, their remedies, the `demoted` findings and the result. What this
release adds to the page and holds:

- The dead-link gate resolves each link target against the repository; a target leaving the
  repository is no fault. Which fragments are live is the page's rule; a bare `#fragment` reaches
  its own document, an element's `id` counts outside code only, a summary page's entry anchors are
  those derived for it, never the page as it stands, and an empty fragment names the document's
  top. A fragment into a file outside `kb-root/`, or into one that is not Markdown, is not checked.
- A summary page's `register-freshness` remedy is the page's empty write batch, except over a KB in an
  older format, where that batch is refused until the conversion is saved, it is
  `kbase update-format`, which re-derives every page too.
- `unknown-id`'s prose ids are read outside code, of any kind.
- `bare-id` reads every document but the register pages, outside code, statement markers and link
  titles and destinations. Its remedy is a write batch of one `add-citation` entry: the `id`, the
  `document`, the `line` and the id as `snippet`.
- `frontmatter`'s remedy is to remove the block; the entry point is checked as every document is.
- `statement-marker` covers a claim, an analysis or an experiment. An `axiom-heading` marked heading
  is one at any depth whose line carries an axiom's marker.
- A KB whose load fails is refused with the load error ([page §2.4](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).
- `findings` is present on `done` and on `refused`.
- The build's gate stage runs this same check.

Result keys: [§7](../7-tool-result-contract-and-exit-codes/index.md).
