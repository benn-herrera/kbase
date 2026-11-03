### Write ops

`kbase write` runs one batch whose entries each name their op, and each op below runs as a batch of
that op alone ([page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)). Each result's keys are [§7](../7-tool-result-contract-and-exit-codes/index.md)'s.

A batch's `commit` option (`--commit` at the command line) is [page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s "`commit`": the
pre-flight commits and their labels, the batch commit labelled `kb-write: <ops> <ids>`, each commit
covering `kb-root/` only, and `check: worktree`, remedy `git init`, outside a git worktree. What
this release adds to the page and holds:

- `<ops>` is the batch's ops, each once, in entry order, joined by `,`; for an empty batch, the
  command's (`write` for `kbase write`); `<ids>` is the ids its entries entered or name in their `id`, `source` and
  `target`, each once, in entry order, joined by `,`.
- A pre-flight commit is made only where its part of `kb-root/` has uncommitted changes, and a
  batch that writes and removes nothing (`unchanged`) makes no batch commit.
- Each commit's `paths` are the repository-relative paths it holds, a rename as both sides.
