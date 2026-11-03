### N — `add-citation` and `citations-of` (expert)

**Design:**
- [§1.4 Statement markers](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) — locating a line by `line` and `snippet`
- [§1.5 Citations](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md) — the citation forms and link targets
- [§2.8 Results and exit codes](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) — "Lists": `citations-of` returns its complete answer
- [§2.10 Existence](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) — the cited node must exist
- [§3.3 Citations](../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/index.md), [`add-citation`](../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/add-citation.md), [`citations-of`](../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/citations-of.md) — `add-citation` and `citations-of`
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — MCP and CLI notes: the write ops' notes

Relies on stage 1: the citation-link reader and the link title, in `format400` and re-exported by
`internal/kb` (`git grep -n 'func .*[Cc]itation' -- internal/formats/format400 internal/kb`).
`renderCitation` writes the link title since stage 1 (`internal/write/render.go`).

**Scope:**
- `internal/write/excerpt.go`, `internal/write/render.go` (the new op; `render-citation` stands
  until package S);
- `internal/query/`;
- `cmd/add-citation.go`, `cmd/citations-of.go` (new);
- `cmd/surface.go`;
- the write-op plumbing every new op touches: `internal/write/ops.go` (op entry, `EntryResult`
  fields), `values.go` (`opFields`), `records.go` (the per-entry report `applyEntries` copies),
  `edits.go` (`targetOf`, the citation target, reused);
- `cmd/write_op.go` (`batchFields`: an `add-citation` entry carries `document`, `line`, `id`,
  `link`; `batchHelp`'s result sentence) and `cmd/write_op_test.go`.

**Facts acted on:**
- [Page §3.3](../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/index.md):
  - `snippet` is the span replaced and occurs exactly once on the line;
  - the three forms;
  - a work or an axiom takes the naming form only;
  - the result keys and refusals;
  - `citations-of` rows `path`, `line`, `text`, ordered by path then line.
- A citation's link target is [page §1.5](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)'s: a hosted node's host leaf and its marker's section anchor.
  `index`'s `CanonicalPath` for a claim is its summary-page entry, not that target. Package L's
  `targetOf` (`internal/write/edits.go`) gives the hosted target; `add-citation` uses it.
- `RenderCitation` is today "the one op that reads instead of writing" (`internal/write/ops.go:17–18`).
  It stays until package S (stage 4).

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): sections citing [page §3.3](../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/index.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.

**Tests:**
- Each form, over a hosted node and over an axiom.
- A snippet that occurs twice on the line is refused, naming each column.
- A quote absent from the statement is refused.
- `citations-of` finds a citation after `move-node`.
- A hosted node's citation never targets a `kb-summary.md` page. Each summary entry heading carries
  its node's `kbase-id` marker (`internal/index/pages.go`: `"## " + n.title + kb.StatementMarker(id)`),
  so a target found by reading markers can land on the summary page. The link's path is the node's
  host leaf, and its anchor is the node's section anchor ([page §1.5](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)). A quote that stands only
  in the node's summary entry, such as its `- rationale:` line, is refused as absent from the
  statement. A work's citation is the exception: it targets its root `kb-summary.md` entry, as
  page §1.5 states.
- The anchor is the heading nearest above the cited node's marker, never an enclosing heading's. The
  quote check reads that node's statement, never the whole parent section. Over the leaf
  `# A\n\nClaim A holds.<!-- kbase-id: clm-aaaaaa -->\n\n## B\n\nClaim B is the bound.<!-- kbase-id: clm-bbbbbb -->\n`:
  - citing `clm-bbbbbb` with `snippet: "Claim B is the bound"` writes a link anchored `#b`;
  - citing `clm-aaaaaa` with the same snippet is refused as a quote absent from the statement, even
    though `# A`'s section, read to the next heading as deep or shallower, contains the text.

**Done-line** (after Rule 13's two): `just test-integration`.
