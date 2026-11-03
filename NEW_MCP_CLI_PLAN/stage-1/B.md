### B — markers, anchors and summary pages (expert)

**Design:**
- [§1.1 Nodes](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md) — leaves and hosting, which a left-over `claim-rigor.md` would break
- [§1.4 Statement markers](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) — the marker, placement, reading rules and anchors
- [§1.6 Derived files: summary pages and sheets](../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md) — `kb-summary.md` pages and sheets
- [§3.6 `update-format`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) — the 4.0.0 conversion steps this package implements

**Scope:**
- `internal/formats/format400/marker.go`;
- `internal/formats/format400/pytext.go` (4.0.0 slugging, which `kb.Slugify` re-exports);
- `internal/formats/from300to400/`;
- `internal/formats/testdata/` (the 4.0.0 goldens package A added);
- `internal/kb/` (`leaf.go` `MarkerLines`, `Hosts`; `pytext.go` `Slugify`; `render.go`;
  `register.go` `RegisterFile`);
- `internal/index/` (`derive.go` `sectionSlug`; `pages.go` `registerPageHead`, the page renderer;
  `footers.go`);
- `internal/write/` (`inserts.go` `markAtLine`, `moveMarker`; `marks.go`);
- `internal/claimgraph/` (`conform.go`, `graph.go`, `prose.go`, `split.go`);
- `internal/sheet/graph.go`, `internal/score/text.go`.

**Facts acted on:**
- The marker, its placement and the reading rules are [page §1.4](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md).
  - The marker is `<!-- kbase-id: <id> -->`.
  - It is attached directly after the line's last character, with no space.
  - It never goes on a fence's opening or closing line.
  - A paired analysis's marker abuts the claim's marker.
- Anchors are [page §1.4](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) "Anchors": github-slugger's algorithm after HTML comments are stripped and
  the heading trimmed. Lowercase; drop every character that is not a letter, mark, number,
  connector, space or hyphen; each space one hyphen, runs not collapsed (`A — B` → `a--b`); a
  heading repeated in one document gets the suffix `-1`, `-2`.
- The 4.0.0 slugging lives in `format400` and `kb.Slugify` re-exports it, because the converter
  cannot import `kb` (see "Decisions the record settles"). `format300` defines no `Slugify`:
  `git grep -n 'func Slugify' -- '*.go'` → `internal/formats/format200/pytext.go:169` and
  `internal/kb/pytext.go:154`.
- Summary pages are [page §1.6](../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md): `kb-summary.md`, titled `# KB summary`, a one-line preamble, and
  each entry headed `## <title><!-- kbase-id: <id> -->`.
- Today:
  - `StatementMarkerOpener = "<!-- claim-rigor:"` (`internal/formats/format300/marker.go:10`);
  - `AppendMarker` puts one space before the marker (`:22–28`);
  - `RegisterFile = "claim-rigor.md"` (`internal/kb/register.go:10`);
  - `registerPageHead` names `kbase refresh` (`internal/index/pages.go:19`);
  - `Slugify` collapses runs and keeps comment words (`internal/kb/pytext.go:154–170`);
  - `sectionSlug` slugs an analysis's or an experiment's section heading
    (`internal/index/derive.go:373–381`).
- Who spells the marker: `git grep -l 'StatementMarker(\|StatementMarkerOpener\|StatementMarkerRE' -- '*.go'` →
  - `internal/build/build_test.go`;
  - `internal/claimgraph/{conform,graph,prose,split_stage_test}.go`;
  - `internal/formats/format300/marker.go`;
  - `internal/formats/from200to300/markers.go`;
  - `internal/index/migration_test.go`;
  - `internal/kb/{leaf,render}.go`;
  - `internal/query/query_test.go`;
  - `internal/write/{bearing_test,drop_edges_test,inserts,inserts_test,marks,marks_test,ops_test,render_test,split_test}.go`.
- `git grep -l 'RegisterFile' -- '*.go'` →
  - `cmd/maintenance_test.go`, `cmd/refresh.go`;
  - `internal/index/{derive,index_test,links_test,pages,pages_test,refresh_test}.go`;
  - `internal/kb/{kb,register}.go`;
  - `internal/kbload/migration_strength_test.go`;
  - `internal/query/query_test.go`;
  - `internal/write/drop_edges_test.go`;
  - frozen `format200`, `from100to200` and `from200to300` files.
- `git grep -l 'Slugify' -- '*.go'` →
  - `internal/index/{derive,pages,pages_test}.go`;
  - `internal/kb/{leaf,metadata_test,pytext}.go`;
  - `internal/write/inserts_test.go`;
  - frozen `format200` files.
- The 3.0.0 heading pattern comes from the old analysis and experiment inserts' own placement rule,
  `markAfterHeading` (`internal/write/inserts.go:390–404`). It is not touched here: it stands with
  those inserts until package S (stage 4) removes them. The conversion rewrites the layout it
  produced ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md), second bullet).
- `split-claim` places the analysis through `markAtLine` (`internal/write/split.go:25`). Its layouts
  stand ([page §1.4](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) "Reading"). Only the marker's spelling and the no-space attach change for it.
- This package's part of the conversion ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)):
  - rewrites every marker to `kbase-id` form, with no space;
  - moves the analysis marker of the exact heading pattern.
- The summary pages are written by the derivation that follows the save, not by the converter.
  `RefreshWithWarnings` runs `saveConverted` and then `pages` (`internal/index/refresh.go:113`;
  `pages` at `:160`), and `update-format` re-derives every derived file on every run ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)).
  The converter writes no page. It returns each `claim-rigor.md` among its obsolete paths, the
  second return of `Convert` (`internal/formats/formats.go:41`), and the save deletes every obsolete
  path and lists it in `removed` (`internal/index/refresh.go:326–337`). [Page §1.6](../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md): "A page no node
  calls for is removed". A `claim-rigor.md` left standing would be a leaf ([page §1.1](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)).

**Contract updates:**
- [SPEC §3](../../PROJECT_DOCS/SPEC/3-compatibility-contract/index.md): the byte-equal table's "Statement markers" and "Register pages" rows.
- [SPEC §10](../../PROJECT_DOCS/SPEC/10-metadata-format-version/index.md): the marker, anchors and register pages, citing [page §1.4](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) and [§1.6](../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): one bullet.
- [ARCHITECTURE §6](../../PROJECT_DOCS/ARCHITECTURE/6-kb-model-and-maintenance/index.md) and I2: wherever they name the marker, the slugging or the register page.

**Tests:**
- Slug tests over [page §1.4](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) "Anchors": a heading with an HTML comment, which is stripped; a run of
  spaces around a dash, kept (`A — B` → `a--b`); a heading repeated in one document, suffixed `-1`
  and `-2`; a heading ending in a no-space marker, which slugs as the bare heading.
- The marker-reading tests for the abutting layout and the split's two layouts ([page §1.4](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)).
- `from300to400` goldens cover the marker rewrite and the abutting move. A near-miss pattern must
  not be moved: an analysis marker after a claim it does not lift, or a non-adjacent line.
- The converter golden lists each `claim-rigor.md` among the obsolete paths.
- Summary-page rendering pins the title, the preamble and the entry heading.

**Done-line** (after Rule 13's two): `just test-integration`, `just test-integration-build-lamb`,
`just test-integration-build-lamb-cycle`.
