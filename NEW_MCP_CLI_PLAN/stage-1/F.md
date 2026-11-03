### F — build records under `kb-root/.build/` (expert)

**Design:**
- [§2.5 The domain rule](../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md) — the build records live in `kb-root/.build/`
- [§3.5 `build`](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md) — stage boundaries are commits scoped to the build's paths
- [§3.6 `update-format`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) — the `git mv` of root records and the clean check over them

**Scope:**
- `internal/buildrecords/buildrecords.go` (`NodePassFile`, `ClassificationFile`, `UnmarkedFile`,
  `CullFile`, `RecordFiles`);
- `internal/kbload/kbload.go` (`coveredFiles`);
- `internal/kbdocs/kbdocs.go` (`bornByInit`);
- `internal/buildstore/` (`OwnedPaths`);
- `internal/formats/from300to400/`;
- `internal/formats/testdata/` (the 4.0.0 goldens);
- `internal/gitledger/ledger.go` (a `git mv` helper);
- `cmd/update-format.go` (`git mv`; the clean check over the records it moves);
- `cmd/importpolicy_test.go`;
- `tools/scripts/test-integration-build-arxiv.sh`.

**Facts acted on:**
- The records live in `kb-root/.build/` and are committed at every stage boundary ([page §2.5](../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md)). The
  4.0.0 conversion moves each root record there with `git mv` ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)).
- `update-format`'s clean check, which package E made cover `kb-root/`, now covers each root build
  record the conversion would move too ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) Refusals).
- Today: "The record files, at the repository root" (`internal/buildrecords/buildrecords.go:15–21`);
  `RecordFiles` at `:24`.
- Who names the records: `git grep -l 'buildrecords\.' -- '*.go'` →
  - `cmd/format_test.go`;
  - `internal/build/{build_test,concurrentasks_test,inference_test}.go`;
  - `internal/buildstore/{ownedpaths_test,stages}.go`;
  - `internal/claimgraph/{attribute,buildrecords,buildrecords_test,classify,commonfiles_test,cull,cull_stage_test,cull_test,depends_stage_test,launch_test,livecounts_test,modulo_ids,modulo_ids_test,nodepass_test,prose,split_stage_test,stages,unmarked,unmarked_test}.go`;
  - `internal/kbdocs/kbdocs.go`, `internal/kbload/kbload.go`;
  - `internal/sheet/sheet_external_test.go`.
- `git grep -l 'kb-build-' -- tools/scripts justfile test-data/fixtures '*.go'` →
  `internal/buildrecords/{buildrecords,buildrecords_test}.go`, `internal/claimgraph/livecounts_test.go`,
  `internal/kb/source_test.go`, `tools/scripts/test-integration-build-arxiv.sh` (`:97`, `:98`,
  `:128`, `:135` name the root records), `tools/scripts/write-evidence.sh`.
- The build's owned paths are `kb.KBDir` plus `RecordFiles`
  (`internal/buildstore/stages.go:51`). With the records under `kb-root/`, the build's stage commits
  stay within `kb-root/` ([page §2.5](../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md)).
- `.build/` is read neither as a leaf nor as a document. `coveredFiles` (`internal/kbload/kbload.go`,
  from `:189`) walks `kb-root/`, skipping `kb.ExcludeDirs` but `.index`. The coder states where
  `.build/` sits in that walk, and in leaf discovery, and pins it.
- `git mv` is git, so `update-format` reaches `gitledger`. Its methods today:
  `grep -n 'func (r \*Repo) [A-Z]' internal/gitledger/ledger.go` → `Trail`, `Head`, `Parent`,
  `Record`, `Commit`, `Dirty`, `DirtyTree`, `Restore`, `Files`, `IndexLock`. None moves a file.
  `cmdFileImporters` (`cmd/importpolicy_test.go:482`) gains `cmd/update-format.go` under
  `internal/gitledger` (see "Decisions the record settles").

**Contract updates:**
- [SPEC §5](../../PROJECT_DOCS/SPEC/5-build-behaviour/index.md) and [§6](../../PROJECT_DOCS/SPEC/6-build-state-ledger-and-monitoring/index.md): where they place the build records, citing [page §2.5](../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): one bullet.
- ARCHITECTURE:
  - §4's stage table, wherever it places the records;
  - the §11 `internal/buildrecords` entry;
  - the §11 bullet naming `gitledger`'s importers ([ARCHITECTURE.md:1679](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/dependency-rules.md)).

**Tests:**
- The converter golden: a root record moves to `.build/`.
- `update-format` refuses while a root record that it would move is dirty, naming it.
- Leaf discovery ignores `.build/`.
- A lamb build commits the records under `.build/`.

**Done-line** (after Rule 13's two): `just test-integration`, `just test-integration-build-lamb`,
`just test-integration-build-arxiv`.
