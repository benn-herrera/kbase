### E — `update-format` replaces `refresh`; a pending conversion refuses every other write; maintained blocks (expert)

**Design:**
- [§1.7 Maintained-block documents](../../NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md) — the fences and how kbase rewrites them
- [§2.3 The derivation after every write](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md) — `update-format` re-derives every run
- [§2.5 The domain rule](../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md) — what kbase writes, and where
- [§2.6 Format conversion](../../NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md) — reads convert, writes refuse while a conversion is pending
- [§2.8 Results and exit codes](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md) — "Check values": `index`, `dirty-paths`, `conversion`
- [§3.6 `verify`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md) — the remedies that name `update-format`
- [§3.6 `update-format`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) — the op in full

**Scope:**
- `cmd/refresh.go`, which becomes `cmd/update-format.go`;
- `cmd/restamp.go`, `cmd/score.go`, `cmd/write_op.go`, `cmd/surface.go`;
- `internal/index/refresh.go` (`saveConverted`, `RefreshReporting` — package D folded
  `RefreshWithWarnings` into it —,
  `Refresh`, `Precheck`);
- `internal/index/verify.go` (`RefreshRemedy`);
- `internal/write/ops.go`, `internal/write/records.go` (the migration save on write);
- `internal/kbdocs/kbdocs.go` (`Restamp`, `Stamp`) and `internal/kbdocs/templates/`;
- `internal/kb/kb.go` (`StampedFiles`);
- the tests and recipes that write over a committed 3.0.0 fixture KB or a 3.0.0 stamp (Facts).

**Facts acted on:**
- `update-format` is [page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md): run alone, the clean-`kb-root/` precondition, always re-derives and
  rewrites the maintained blocks, its result keys, its refusals.
- The pending-conversion refusal is [page §2.6](../../NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md): `check: index`, remedy `kbase update-format`, on
  every write but `update-format`. Reads keep working through the in-memory conversion.
- The maintained blocks are [page §1.7](../../NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md): the fence lines, recognised by their ends; unfenced documents
  reported; `README.md` written only by a build.
- Today a migration rides along on any write:
  - `internal/write/records.go:193–198` ("On a migrated KB … both are written");
  - `internal/index/refresh.go:113` runs `saveConverted`;
  - `Restamp` rewrites the stamped documents whole (`internal/kbdocs/kbdocs.go:161–178`).
- `git grep -l 'RefreshRemedy' -- '*.go'` →
  - `cmd/{maintenance_test,query,refresh,verify}.go`;
  - `internal/build/build_test.go`;
  - `internal/index/{derive,refresh_test,verify}.go`;
  - `internal/query/query_test.go`;
  - `internal/score/{invoke,invoke_test}.go`;
  - `internal/write/ops.go`.
- `git grep -l 'Restamp' -- '*.go'` →
  - `cmd/{format_test,restamp,write_op}.go`;
  - `internal/build/build.go`;
  - `internal/kbdocs/kbdocs.go`;
  - `internal/write/ops.go`.
- `git grep -l 'RefreshReporting' -- '*.go'` (re-run: package D changed its callers) →
  - `cmd/{init,refresh,score}.go`;
  - `internal/index/{migration_test,refresh,refresh_test}.go`;
  - `internal/write/ops.go`.
- The recipes and transcripts that run `refresh`: `git grep -l -- 'refresh' -- tools/scripts test-data/fixtures/mcp` →
  - `tools/scripts/test-integration-build-live-fixture.sh`, `test-integration-init.sh`,
    `test-integration-mcp.sh`, `test-integration-refused-write-adjagent-sheet.sh`,
    `test-integration-results-fixture.sh`, `write-evidence.sh`;
  - `test-data/fixtures/mcp/refusal.jsonl`, `resources.jsonl`, `write-op.jsonl`.
- The writers over a 3.0.0 KB. Once this package lands they are refused unless they convert first
  (Rule 5).
  - Go: `git grep -l 'score-kb\|fixtures/results\|"results", "kb-root"\|fixtures", "results"\|adjagent-sheet\|kb-format: \\"3\|"3\.0\.0"' -- '*.go' ':!internal/formats/'` →
    - `cmd/{format_test,mcp_test,score_test}.go`;
    - `internal/build/build_test.go`;
    - `internal/export/{assemble_test,strip_test}.go`;
    - `internal/index/derive_bench_test.go`;
    - `internal/kb/format_test.go`;
    - `internal/kbload/{kbload_test,parity_test}.go`;
    - `internal/query/query_test.go`;
    - `internal/score/invoke_test.go`;
    - `internal/sheet/sheet_external_test.go`;
    - `internal/write/migrated_test.go`.

    The files that only read keep reading, since reads answer as converted ([page §2.6](../../NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)). Only those
    that write move.
  - Recipes: `git grep -l 'RESULTS_FIXTURE_DIR\|adjagent-sheet' -- tools/scripts` →
    `test-integration-mcp.sh`, `test-integration-opscript-results.sh`,
    `test-integration-parity-adjagent-sheet.sh`, `test-integration-refused-write-adjagent-sheet.sh`,
    `test-integration-results-fixture.sh`. The parity script only reads.
  - The MCP runner inserts over its copy (`tools/scripts/test-integration-mcp.sh:37`) before it
    refreshes (`:83`). The conversion comes before the first write.
  - Each of these scripts stages its copy beside a bare `.git` (for example `:33–34`). `update-format`
    needs the copy committed (Rule 5), so a converting script commits its copy first.
  - The refused-write leg tests the `agents-redirect` refusal over a 2.0.1 copy
    (`tools/scripts/test-integration-refused-write-adjagent-sheet.sh:2–4`). The pending-conversion
    refusal now meets every write over that copy first. The leg must still reach the
    `agents-redirect` refusal ([page §1.7](../../NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)). How its copy gets there is the coder's choice.
- `refresh`'s re-derivation of a stale page passes to `update-format` now, and to the empty write
  batch in package H (stage 2). Until H lands, `verify`'s register-freshness remedy names
  `update-format`. Package T (stage 4) sets the final remedies.
- This package's clean check covers `kb-root/`. Package F extends it to the root build records it
  moves ([page §2.5](../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md), [§3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)). `update-format`'s `commit` option lands in package I (stage 2).
- The `update-format` result lists come from the converter's reports, the `common.Warning` fields
  package A added and package C reused: `git grep -n -A16 'type Warning struct' -- internal/formats/common/common.go`.
  - `bare-ids`, `no-edge` and `strengthen-by` are result keys of their own ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) Result);
  - the unresolved citations go among `migration`'s `warnings`.

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): `### refresh` is replaced by `### update-format`, citing [page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md).
- SPEC, by `git show 68021ba:SPEC.md | grep -n '`refresh`\|kbase refresh'`, the lines before [`:2438`](../../PROJECT_DOCS/SPEC/12-interim-releases/0.9.0-messages-is-the-default-chat-protocol.md).
  At `68021ba`: [`:26`](../../PROJECT_DOCS/SPEC/1-what-kbase-is.md), [`:40`](../../PROJECT_DOCS/SPEC/2-given-interfaces/index.md), [`:72`](../../PROJECT_DOCS/SPEC/3-compatibility-contract/index.md), [`:77`](../../PROJECT_DOCS/SPEC/3-compatibility-contract/stamped-documents.md), [`:90`](../../PROJECT_DOCS/SPEC/3-compatibility-contract/claim-graph-svg/index.md), [`:143`](../../PROJECT_DOCS/SPEC/3-compatibility-contract/claim-graph-svg/what-is-drawn.md), [`:146`](../../PROJECT_DOCS/SPEC/3-compatibility-contract/claim-graph-svg/what-is-drawn.md), [`:372`](../../PROJECT_DOCS/SPEC/6-build-state-ledger-and-monitoring/git-holds-the-recoverable-record.md), [`:507`](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/shared-vocabulary.md), [`:535`](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/item.md),
  [`:550–553`](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/item.md), [`:640`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/one-write-lock.md), [`:1143`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/score/refresh-and-format.md), [`:1146`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/score/refresh-and-format.md), [`:1148`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/score/refresh-and-format.md), [`:1201`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/score/the-closing-record.md), [`:1242`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/refresh.md), [`:1246`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/refresh.md), [`:1261`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/verify.md), [`:1647`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/one-version-read-and-written.md),
  [`:1658`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/one-version-read-and-written.md), [`:1670`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/one-version-read-and-written.md), [`:1676`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/one-version-read-and-written.md), [`:1795`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-record-files.md), [`:1801`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-record-files.md), [`:1831`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-load-strict-on-syntax-lenient-on-integrity.md), [`:1837–1841`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-load-strict-on-syntax-lenient-on-integrity.md), [`:2127`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-migration-announces-itself.md), [`:2150`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/cycles-cut-as-the-walk-finds-them.md), [`:2429`](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md).
  - This package replaces each mention of the `refresh` command, of `kbase refresh` as a remedy,
    and of a migration riding on a natural write ([SPEC §3 "Stamped documents"](../../PROJECT_DOCS/SPEC/3-compatibility-contract/stamped-documents.md), [§10](../../PROJECT_DOCS/SPEC/10-metadata-format-version/index.md)'s migration
    text, the §2 `dot` row).
  - A mention of a write op's own trailing refresh, or of a run's start or end refresh, names a
    mechanism that stands until package H (write ops), Q (`score`) or R (the build) replaces it. It
    is renamed where it names the command, and otherwise stands.
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.
- ARCHITECTURE:
  - I8's last clause ("the result is written only by the next natural write…");
  - §11 "Loading";
  - the `internal/kbdocs` entry and the `cmd` file-level `kbdocs` rule ("`cmd/restamp.go` …
    serving `refresh`, `score`'s start refresh and the write ops' trailing refresh").
- [CONVENTIONS "Testing"](../../PROJECT_DOCS/CONVENTIONS/testing/index.md):
  - wherever it names `refresh`;
  - [CONVENTIONS.md:549–552](../../PROJECT_DOCS/CONVENTIONS/testing/results-are-part-of-the-test.md) ("an empty directory serves") gains the case of a copy that
    `update-format` converts, which needs the copy committed.

**Tests:**
- `update-format` over a 3.0.0 KB: converts it, re-derives, rewrites the fences, and lists the
  converter's reports under `bare-ids`, `no-edge`, `strengthen-by` and `migration.warnings`.
- `update-format` over a 4.0.0 KB: re-derives, rewrites the fences, leaves a bare id.
- A dirty `kb-root/` is refused, naming the paths.
- Every write op, `score` and `build` over an existing 3.0.0 KB is refused with the remedy, and a
  query answers with `migration`.
- The fences: one pair rewritten; an absent `AGENTS.md` written whole; a document without the pair
  reported and untouched; an empty pair filled.
- The recipes and transcripts that ran `refresh` run `update-format`.

**Done-line** (after Rule 13's two): `just test-integration`, `just test-integration-build-lamb`.
