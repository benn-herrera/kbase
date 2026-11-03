### A — `format400` records, the `kb` switch, `gated-on` gone, `regenerate-fixtures` (expert)

**Design:**
- [§1.1 Nodes](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md) — the node kinds, ids and record fields 4.0.0 writes
- [§1.2 Edges and relations](../../NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md) — the edge records and their fields
- [§1.8 Format version](../../NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md) — what 4.0.0 defines, the compatibility break
- [§2.4 Load errors](../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md) — a record missing `judged-hash` is a load error
- [§2.9 Manually scored, and the judged hash](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md) — the `judged-hash` field
- [§3.6 `update-format`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) — this package's conversion steps and the result lists its
  warnings feed

**Scope:**
- `internal/formats/format400/` (new), started from `format300`'s files;
- `internal/formats/from300to400/` (new);
- `internal/formats/formats.go` (`converters`; the imports);
- `internal/formats/common/common.go` (`Warning`);
- `internal/formats/testdata/`: the new goldens `register-4.0.0/`, `register-4.0.0.conversion.yaml`,
  `register-4.0.0.values.jsonl`, `adjagent-sheet-4.0.0/`, `adjagent-sheet-4.0.0.conversion.yaml`,
  `adjagent-sheet-4.0.0.values.jsonl`, and `README.md`'s table row and "Decoded values" paragraph
  for them (Rule 5 governs how they are produced);
- `internal/kb/` (`format.go`, `graph.go`, `schema.go`, `indexstream.go`, `kb.go`, `leaf.go`,
  `pytext.go`, `render.go`), re-exporting `format400`;
- the `gated-on` query: `cmd/gated-on.go`, `GatedOn`, `GatedOnPayload`, its `cmd/surface.go` line;
- the 14 parity cases that run `gated-on`: each `internal/kbload/testdata/parity-2.0.1/<n>.args`
  whose first line is `gated-on`, with its `<n>.exit` and `<n>.out`;
- `internal/kbload/parity_test.go` (the golden-side rules for the keys 4.0.0 drops);
- `justfile` (a new `regenerate-fixtures` recipe), `tools/scripts/regenerate-fixtures.sh` (new),
  `test-data/fixtures/init/skeleton/`;
- the policy tests that name the live format: `cmd/idprefixpolicy_test.go`,
  `cmd/importpolicy_test.go`, `cmd/recordpolicy_test.go`, `cmd/relationpolicy_test.go`;
- `internal/kb/source.go` (`MigrationWarning`, `MigrationNote` and its comment);
- `internal/kbload/kbload.go` (`conversionRefusal`, `migrationWarnings`);
- the tests that pin `3.0.0` or `status: pending` outside `internal/formats`:
  `git grep -ln '3\.0\.0' -- '*.go' ':!internal/formats'` and
  `git grep -n 'status: pending\|Status: "pending"' -- '*.go'` → at `178e814`,
  `cmd/maintenance_test.go`, `cmd/format_test.go`, `internal/kbload/kbload_test.go`,
  `internal/write/migrated_test.go`, `internal/kb/format_test.go`, `internal/index/derive_test.go`,
  `internal/index/index_test.go`, `internal/write/inserts_test.go`, `internal/write/records_test.go`.

**Facts acted on:**
- What 4.0.0's records are: [page §1.1](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md), the record-fields table, and [page §1.2](../../NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md).
  - `judged-hash` is the last field of a claim, an analysis and an experiment record, and of a
    `supports`, `lifts` and `refutes` record. It is required, and all zeros where never judged
    ([page §2.9](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)).
  - `judged-hash` is a SHA-256 digest written as 64 lowercase hex digits, as the code's
    `prompt_sha256` and `ask_sha256` are. Held as a `[32]byte`, its Go zero value is the all-zeros
    hash, so a producer that sets no hash writes the unjudged value. A `demoted` record carries no
    hash.
  - An experiment's `status` is `run` | `planned`.
  - No `no-edge` and no `strengthen-by`.
  - `references` joins any kind to any kind. The decoder already sets no pairing on `references`
    (`internal/formats/format300/graphdecode.go:368–369`), so the decoder has nothing to relax.
- Warnings and refusals across the conversion:
  - `internal/kbload/kbload.go:109` converts each `formats.Warning` (`common.Warning`) to
    `kb.MigrationWarning` (`internal/kb/source.go:25–29`) by struct conversion, so the fields
    `common.Warning` gains are added to `MigrationWarning` identically. Each new field carries a
    `yaml:"…,omitempty"` tag, so `from200to300`'s frozen `*-3.0.0.conversion.yaml` goldens, which
    marshal `common.Warning` (`conversionRecord`, `internal/formats/from200to300/goldens_test.go`),
    are unchanged.
  - `from300to400` decodes the 3.0.0 records through `format300`, whose fault is `GraphLoadError`.
    `conversionRefusal` (`internal/kbload/kbload.go:84–102`) maps it, re-exported from the
    `formats` root, to one `check: metadata` item per fault, as it maps `AnnotationError`, so a
    malformed 3.0.0 KB is refused as page §2.4 says, not as one `kb-format` item.
  - `kb.MigrationNote` (`internal/kb/source.go:34`) says "every rigor and bearing that score judges
    is now marked model-scored", true only of the chain to 3.0.0. The 3.0.0 → 4.0.0 conversion keeps
    `manually-scored` and writes every `judged-hash` as all zeros, so the note a 4.0.0 conversion
    reports says that: no scored value is claimed current, and the next `score` asks every one. The
    note's comment and [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md)'s quotation of it ([`:2120–2122`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-migration-announces-itself.md)) move with it.
- The 3.0.0 shapes in `internal/formats/format300/graph.go`: `nodeFields` (`:24–31`), `edgeFields`
  (from `:33`), `ExperimentRun`, `ExperimentPending` and `ExperimentStatuses` (`:50–57`), and the
  record types from `ClaimNode` (`:67`) to `EdgeRecord` (`:148`).
- `format400` takes `format300`'s import rule (`cmd/importpolicy_test.go:125–128`: no kbase
  import). The policy tests name `format400` as the live format.
- Who imports `format300`: `git grep -l 'formats/format300"' -- '*.go'` →
  - `cmd/importpolicy_test.go`, `cmd/recordpolicy_test.go`;
  - `internal/formats/formats.go`, `internal/formats/formats_test.go`;
  - `internal/formats/from100to200/split_test.go`;
  - `internal/formats/from200to300/from_2_0_0.go`, `from_2_0_0_test.go`, `markers.go`,
    `markers_test.go`;
  - `internal/kb/format.go`, `graph.go`, `indexstream.go`, `kb.go`, `leaf.go`, `pytext.go`,
    `render.go`, `schema.go`.

  The `from200to300` and `from100to200` files keep importing `format300`, and `kb` switches to
  `format400`. `git grep -l 'format300' -- 'cmd/*_test.go'` → `cmd/idprefixpolicy_test.go`,
  `cmd/importpolicy_test.go`, `cmd/recordpolicy_test.go`, `cmd/relationpolicy_test.go`.
- The fields that go, by their consumers:
  - `git grep -l 'NoEdge' -- '*.go'` →
    - `internal/formats/format300/{graph,graphdecode,graphencode,graphencode_test,values_test}.go`;
    - `internal/formats/from200to300/{from_2_0_0,from_2_0_0_test}.go`;
    - `internal/index/{citations,index_test,pages}.go`;
    - `internal/kb/graph_test.go`;
    - `internal/write/{inserts,marks,split_test}.go`.
  - `git grep -l 'StrengthenBy' -- '*.go'` → the same `format300` and `from200to300` files, plus
    - `internal/formats/format200/register.go`;
    - `internal/index/{citations,derive,pages}.go`;
    - `internal/kb/{graph_test,indexstream}.go`;
    - `internal/query/{load,payload,query_test}.go`;
    - `internal/write/{drop_edges_test,inserts,marks}.go`.
  - `git grep -l 'StrengthenByRow\|MentionedIDs\|mentioned_ids\|strengthen_by' -- '*.go'` →
    - `cmd/show.go`;
    - `internal/formats/format200/register.go`;
    - `internal/index/derive.go`;
    - `internal/kb/indexstream.go`;
    - `internal/query/{load,payload,query,query_test}.go`.
  - The write keys: `git grep -l '"no-edge"\|"strengthen-by"' -- '*.go'` →
    - `cmd/write_op.go` (`noEdgeHelp`, `writeOpLong`);
    - `internal/write/{inserts,marks,values}.go`;
    - `internal/kb/graph_test.go`, `internal/write/drop_edges_test.go`;
    - `internal/formats/format200/register.go`, `format300/graph.go`, `from100to200/from_1_0_0.go`,
      `from200to300/{from_2_0_0,from_2_0_0_test}.go`.
  - The foreign-domain lint reads `NoEdge`: `foreignEdgeFindings`
    (`internal/index/citations.go:223–266`). It goes here. The rest of the citation gate goes in
    package C.
  - `gated-on`: `git grep -l 'GatedOn' -- '*.go'` → `cmd/gated-on.go`,
    `internal/query/payload.go`, `internal/query/query.go`. Its registration is
    `queryOperation(gatedOnCommand()),` at `cmd/surface.go:71`.
- What each consumer of a dropped field leaves true:
  - an insert op given `no-edge` or `strengthen-by` is refused as an unknown key, as any key
    outside its closed vocabulary is; `noEdgeHelp` and its paragraph in `writeOpLong` go;
  - `show` drops `strengthen_by`, `strengthen_by_count` and their `mentioned_ids`
    (`internal/query/payload.go:253–278`);
  - `stats` drops `strengthen_by_items` (`internal/query/payload.go:322`);
  - a summary-page entry shows no strengthen-by row;
  - `drop-claim-node`'s mention check reads no `no-edge` or `strengthen-by` text.
- The names in fixtures and scripts: `git grep -l -- 'gated-on\|no-edge\|strengthen-by' -- test-data/fixtures tools/scripts justfile`
  → `test-data/fixtures/compat/ops.yaml`, `test-data/fixtures/mcp/frame-errors.jsonl`,
  `test-data/fixtures/mcp/resources.jsonl`, `tools/scripts/test-integration-results-fixture.sh`
  (`:190` runs `gated-on`). `frame-errors.jsonl` pins the `query` tool's operation list, which
  names `gated-on`.
- The pins of the dropped keys: `git grep -l 'strengthen_by\|strengthen-by\|no-edge\|gated-on' -- 'test-data/fixtures/mcp/*.jsonl' 'internal/kbload/testdata/parity-2.0.1/*.out'`
  → `mcp/{frame-errors,gui-project-dir,resources,stats}.jsonl` and the parity goldens `021`, `027`,
  `033`, `039`, `045`, `051`, `057`, `063`, `069` and `112` `.out`. The parity goldens are the
  2.0.1 binary's frozen recordings. A key 4.0.0 drops leaves them through a golden-side rule of
  `TestConvertedAnswersMatchTheGoldens` (`internal/kbload/parity_test.go`, the rules from `:100`),
  never by an edit to a golden.
- The parity cases: `for f in internal/kbload/testdata/parity-2.0.1/*.args; do head -1 "$f"; done | sort | uniq -c`
  → 28 `deps`, 14 `cited-by`, 14 `gated-on`, 14 `referenced-by`, 11 `subtree`, 10 `find`,
  15 `show`, 1 `stats`. The `gated-on` cases are retired with the verb: their three files are
  deleted. `TestConvertedAnswersMatchTheGoldens` reads whatever `.args` files stand
  (`internal/kbload/parity_test.go:671`).
- The status vocabulary: `git grep -l 'ExperimentPending\|ExperimentStatuses' -- '*.go'` →
  - `internal/formats/format300/{graph,graphdecode}.go`;
  - `internal/kb/graph.go`;
  - `internal/write/ops_test.go`, `internal/write/values.go`.
- The conversion's interface:
  - `converter{From, To, Convert func(Files) (Files, []string, []Warning, error)}`
    (`internal/formats/formats.go:39–42`).
  - `Files` is "a KB's metadata files by repository-relative slash path"
    (`internal/formats/common/common.go:5–7`).
  - `Warning{Path, Line, Detail}` (`:9–16`) holds no id and no text field.
- The converter's reports are `common.Warning` values. This package adds to `Warning` the fields an
  `update-format` result list needs: which list the warning belongs to (`no-edge`,
  `strengthen-by`, and the two package C adds, `bare-ids` and unresolved citations), the node `id`,
  and the `text`. Every existing literal is keyed
  (`git grep -n '[^a-zA-Z]Warning{' -- '*.go' | grep -v 'Warning{Path:'` → nothing), so each still
  compiles unchanged, and the frozen packages' output does not move. Package E reads these fields.
- What this package's part of the conversion does ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)):
  - rewrites `status: pending` as `status: planned`;
  - drops `no-edge` and `strengthen-by`, reporting each node's `id` and its text;
  - writes `judged-hash` as all zeros on every scored record;
  - stamps `4.0.0`.

  Packages B, C and F add the rest.
- `init`'s skeleton fixture:
  - `TestInitBirthWritesTheBornTreeAndCommitsIt` (`cmd/init_test.go:156`) compares the born
    `kb.EntryPointFile`, `kb.FormatFile`, `kb.GraphNodesFile` and `kb.GraphEdgesFile` byte for byte
    with `test-data/fixtures/init/skeleton/` (`:185–199`). The fixture's `.index/format.yaml` holds
    `3.0.0`, and `init` stamps `4.0.0` once `kb` is switched.
  - The test keeps comparing with the fixture. The fixture becomes the tool's own output.
  - No recipe regenerates a fixture today: `grep -n 'generate' justfile` → one comment line, `:18`.
  - `regenerate-fixtures` depends on `build`. Its script runs `./bin/kbase init` in a fresh git
    repository under `test-data/transient/regenerate-fixtures/`, and copies the four files the test
    compares into `test-data/fixtures/init/skeleton/`. Its `[doc]` is at most 200 ASCII characters
    (`tools/scripts/check-just-docs.sh`, run by `edit-gate`).
  - [CONVENTIONS.md:39–41](../../PROJECT_DOCS/CONVENTIONS/build-and-run/test-data-layout.md) ("Test data layout") says: "Never write generated or downloaded data into
    `fixtures/`." The skeleton is generated, so that line gains the exception (Contract updates).

**Contract updates:**
- SPEC, by `git show 68021ba:SPEC.md | grep -n 'gated-on\|strengthen\|no-edge\|no_edge'`. At `68021ba`:
  - this package's: [`:516`](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/shared-vocabulary.md), [`:555`](../../PROJECT_DOCS/SPEC/7-tool-result-contract-and-exit-codes/item.md) (§7), [`:803–804`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/drop-claim-node.md) (§8 `drop-claim-node`), `:823`
    (`### gated-on`), [`:844`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/edges.md) (§8 `stats`), [`:1698–1699`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/one-version-read-and-written.md), [`:1738–1739`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-record-files.md), [`:1746`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-record-files.md) and `:1808` (§10
    record fields), each citing [page §1.1](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md), [§1.2](../../NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md), [§1.8](../../NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md) or [§2.9](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md);
  - not this package's: [`:1680`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/one-version-read-and-written.md), [`:1989`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-1-0-0-2-0-0-migration.md), [`:2057`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-2-0-x-3-0-0-migration.md) and [`:2111`](../../PROJECT_DOCS/SPEC/10-metadata-format-version/the-2-0-x-3-0-0-migration.md) state what the 1.0.0, 2.0.x and
    3.0.0 conversions do, which still holds.
- [SPEC §3 "Formats"](../../PROJECT_DOCS/SPEC/3-compatibility-contract/formats.md): the current format is `4.0.0`.
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): one bullet.
- ARCHITECTURE:
  - I8: `format400` is the live format's one definition;
  - §11, the `internal/formats` and `internal/kb` entries;
  - §6: wherever it names `no-edge`, `strengthen-by` or the foreign-domain lint.
- [CONVENTIONS "Build and Run"](../../PROJECT_DOCS/CONVENTIONS/build-and-run/index.md), ["Test data layout"](../../PROJECT_DOCS/CONVENTIONS/build-and-run/test-data-layout.md): a committed fixture that pins kbase's own output
  is written only by `just regenerate-fixtures`.

**Tests:**
- Encoder and decoder round-trip tests in `format400`, carried over from `format300`'s and changed
  for the new fields. A record without `judged-hash` is a load fault naming the record and the
  field.
- `from300to400` golden tests over `internal/formats/testdata/register-3.0.0` and
  `adjagent-sheet-3.0.0` as inputs, into the 4.0.0 goldens in Scope. The golden pins the records and
  the reported `no-edge` and `strengthen-by` warnings with their ids and texts.
- An insert op given `no-edge` is refused as an unknown key.
- The policy tests name `format400` as the live format.
- `TestInitBirthWritesTheBornTreeAndCommitsIt` passes against the regenerated skeleton, whose
  `.index/format.yaml` holds `kb-format: "4.0.0"`.

**Done-line** (after Rule 13's two): `just regenerate-fixtures`, `just test-integration`.
