### I — `commit` (expert)

**Design:**
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — "`commit`": the option, its pre-flight commits and labels
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — "Result": the `commits` key
- [§3.6 `update-format`](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) — its `commit` option and label

Relies on stage 1: `cmd/update-format.go` and its `gitledger` import.

**Scope:**
- `cmd/write.go`, `cmd/write_op.go`, `cmd/update-format.go`;
- `internal/gitledger/ledger.go` (any status or commit helper it lacks for a `kb-root/` scope);
- `cmd/importpolicy_test.go`;
- `internal/write/values.go` (the `commit` key beside `entry` and `dry-run`) and
  `internal/write/ops.go` (the point between apply and write where the pre-flight commits run);
- `internal/mcp/mcp.go` (`batchTool`'s schema gains `commit`);
- the tests beside those files, and `test-data/fixtures/mcp/*.jsonl` where the `write` tool's or
  `update-format`'s schema moves (Rule 4).

**Facts acted on:**
- [Page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) "`commit`":
  - the two pre-flight commits and their labels;
  - a failed pre-flight stops the batch with nothing written;
  - the batch commit's label `kb-write: <ops> <ids>`;
  - a failed batch commit leaves the batch written, outcome `done`, `uncommitted`;
  - `check: worktree` outside a git worktree;
  - every commit covers `kb-root/` only.
- [Page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md): `update-format --commit` makes one commit, label `kb-write: update-format <from> → <to>`.
- `commit` is a batch key beside `entry` and `dry-run`, and a `--commit` flag, as `dry-run` is
  both. `parseValues` (`internal/write/values.go`) accepts only `dry-run` and `entry` at the top
  level today, and `batchTool` (`internal/mcp/mcp.go`) lists only those two.
- The pre-flight commits run after apply and before write. That point is inside `internal/write`:
  `runOverSource` (`internal/write/ops.go`) applies (`applyEntries`), derives
  (`index.PlanDerivation`), then writes (`writeChanges`) unless the batch is a dry run. Git stays
  out of `internal/write`: `write` exposes a hook at that point that its caller supplies, and a
  dry run never reaches it. The batch commit runs after `write.Run` returns.
- The commits run where every write surface runs its batch: `runWriteOp` in `cmd/write_op.go`
  (`git grep -n 'func runWriteOp' -- cmd`), which `kbase write`, each op's subcommand and the MCP
  `write` tool call. That file is the new `gitledger` importer.
- A batch that writes and removes nothing makes no batch commit ("Commit the batch's changes");
  `gitledger`'s commit passes `--allow-empty`, so the caller skips it.
- `check: worktree` carries remedy `git init` (page §2.2); `gitledger.NotWorktreeError` holds it
  only in its error text today. `cmd/update-format.go` imports it
  since stage 1. `cmdFileImporters` in `cmd/importpolicy_test.go` gains `cmd/write_op.go` under
  `"internal/gitledger"`.
- `gitledger` has `Commit(subject, body)` (`internal/gitledger/ledger.go:131`), `Dirty()` (`:158`)
  and `DirtyTree()` (`:165`). `Open(dir, owned, lg)` (`:47`) takes the owned paths.
  `git grep -l 'gitledger\.Open' -- '*.go'` →
  - `cmd/init.go`, `cmd/mcp_test.go`;
  - `internal/build/{build,build_test}.go`;
  - `internal/buildstore/{status,status_test}.go`.
- The help this package writes (Rule 12): `--commit` joins every write op's CLI note and
  `update-format`'s; the write help states that a committing batch commits the whole of `kb-root/`,
  and that a batch without it has no safety net ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)).

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): `commit`, citing [page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): one bullet.
- [ARCHITECTURE §11](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/dependency-rules.md): the bullet naming `gitledger`'s importers names `cmd/write_op.go`.

**Tests:**
- In a git repository the test creates:
  - a dirty leaf, and a dirty `.index/` produced by a kbase write left uncommitted, each get their
    own pre-flight commit;
  - the batch commit holds exactly the batch;
  - a revert of it restores the prior `kb-root/`;
  - a hook that refuses the batch commit leaves the batch written with `uncommitted`;
  - a hook that refuses a pre-flight commit leaves nothing written;
  - a dry run commits nothing.

**Done-line** (after Rule 13's two): `just test-integration`.
