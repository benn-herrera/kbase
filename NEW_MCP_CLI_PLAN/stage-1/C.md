### C — citations in the format (expert)

**Design:**
- [§1.4 Statement markers](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) — "Anchors": the 4.0.0 slugging every link is rewritten to
- [§1.5 Citations](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md) — the citation forms, the link title, the `bare-id` fault
- [§2.8 Results and exit codes](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) — "Check values": the check names
- [§3.6 `verify`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md) — the `bare-id` fault and the dead-link gate
- [§3.6 `update-format`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) — the citation conversion steps and the `bare-ids` result list

**Scope:**
- `internal/formats/format400/citation.go` (new: the citation-link grammar: link text, target and
  title id);
- `internal/kb/links.go` (re-exporting it) and `internal/kb/deadlinks.go` (`rawTargets`);
- `internal/index/citations.go` (the citation-grammar gate goes);
- `internal/index/verify.go` (the `bare-id` fault);
- `internal/index/links.go`;
- `internal/formats/from300to400/`;
- `internal/formats/testdata/` (the 4.0.0 goldens);
- `internal/write/excerpt.go`, `internal/write/render.go` (`renderCitation` writes the link title).

**Facts acted on:**
- A citation is [page §1.5](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md):
  - quote, paraphrase and naming forms, each with the node id as its link title;
  - the link target per kind;
  - the `bare-id` fault outside code blocks, markers and link titles.
- The reader has one home. The dead-link gate already reads link targets from a line
  (`rawTargets`, `internal/kb/deadlinks.go:134`). The citation grammar in `format400` and
  `rawTargets` read links through one link pattern, and no second link parser is written. The
  converter reads links through the same `format400` grammar.
- The gate that goes: `citationLinkFindings` and `CitationFindings`
  (`internal/index/citations.go:147`, `:275`), called from `BuildVerify`.
  `git grep -l 'CitationFindings' -- '*.go'` → `internal/index/{citations,index_test,refresh_test,verify}.go`.
- The `bare-id` fault's check name is `bare-id` ([page §2.8](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md), [§3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)).
- `renderCitation(excerpt, kbPath, anchor)` (`internal/write/render.go:46`).
  `git grep -l 'renderCitation(' -- '*.go'` → `internal/write/{excerpt,render,render_test}.go`.
  It gains the id so that the link it writes carries the title until package N (stage 2) replaces
  the op.
- 3.0.0 anchors carry marker words: `Slugify` drops no HTML comment (`internal/kb/pytext.go:154–170`),
  so `## Result <!-- claim-rigor: clm-a -->` slugs to `result-claim-rigor-clm-a`, and an analysis's
  or an experiment's anchor is slugged from its section's heading (`sectionSlug`,
  `internal/index/derive.go:373–381`). Under 4.0.0 slugging (package B) such an anchor names no
  heading.
- The converter resolves a 3.0.0 anchor with `format200.Slugify`
  (`internal/formats/format200/pytext.go:169`), which is the 3.0.0 slugging verbatim:
  `diff <(git show 68021ba:internal/formats/format200/pytext.go | sed -n '/^func Slugify/,/^}/p') <(git show 68021ba:internal/kb/pytext.go | sed -n '/^func Slugify/,/^}/p')`
  → no difference, and `IsSpace`, `Strip` and `isWord`, which it calls, read the same in both
  files. A converter may import any format up to its target (`formatsTreeViolations`,
  `cmd/importpolicy_test.go:330`). `format200` is imported, not edited.
- This package's part of the conversion ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)), resolving every anchor under 3.0.0 slugging:
  - adds the id as the link title of each citation whose anchor resolves to a node's statement;
  - rewrites every link into a heading, citations and ordinary links alike, to the heading's anchor
    under [page §1.4](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)'s slugging;
  - points a work's citation at its entry on the root summary page ([page §1.5](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md));
  - reports each citation it cannot resolve without guessing, as a `common.Warning` of the
    unresolved-citations list (package A's fields);
  - converts each bare id into the naming form, reporting `path`, `line` and `id` as a
    `common.Warning` of the `bare-ids` list. Only this conversion does so ([page §1.5](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)).

  So a converted KB passes `verify`'s dead-link gate.

**Contract updates:**
- [SPEC §10](../../PROJECT_DOCS/SPEC/10-metadata-format-version/index.md) and [SPEC §8 `verify`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/verify.md): citations and `bare-id`, citing [page §1.5](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md) and [§3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md).
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md) `### refresh` ([SPEC.md:1242](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/refresh.md), until package E replaces it): the link rewrite and the
  title addition, citing [page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): one bullet.
- [ARCHITECTURE §6](../../PROJECT_DOCS/ARCHITECTURE/6-kb-model-and-maintenance/index.md) and its `internal/index` row ([ARCHITECTURE.md:405](../../PROJECT_DOCS/ARCHITECTURE/6-kb-model-and-maintenance/index.md): "the citation gate and the
  claim-id link check").

**Tests:**
- The reader over each of the three forms, a link with no title, and a title naming no node.
- `bare-id` at each exclusion: a fence, a marker, a link title.
- Converter goldens for title addition, the rewrite of a citation's target and of an ordinary link
  into a re-slugged heading, a work's citation pointed at its summary entry, unresolved reporting
  and bare-id conversion.
- A KB converted by `refresh`'s save (Rule 5), with citations into marked headings, an ordinary link
  into a re-slugged heading, and a citation of a work, passes `verify`'s dead-link gate.

**Done-line** (after Rule 13's two): `just test-integration`.
