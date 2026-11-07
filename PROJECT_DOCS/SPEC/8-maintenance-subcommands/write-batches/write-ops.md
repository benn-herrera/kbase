#### Write ops

The write ops are `add-claim`, `add-analysis`, `add-experiment`, `add-work`, `add-axiom`, `edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work`, `edit-axiom`, `remove-node`, `move-node`, `add-edge`, `edit-edge`, `remove-edge`, `restore-demoted` and `add-citation`. Each is an entry of a batch, as above; each result's keys are defined in [§7](../../7-tool-result-contract-and-exit-codes/item.md).

**`commit`.** `commit: true` (`--commit`) makes git the batch's undo. It is never on by default, and no configuration sets it. Every commit it makes covers `kb-root/` only. A committing batch:
1. applies every entry in memory; on any error it returns the error with nothing written and nothing committed;
2. makes the snapshot commits, before writing, each only where its part of `kb-root/` has uncommitted changes and each alone: first the changes outside `.index/`, labelled `kb-write: kb-root/ changes outside .index/ made before this batch`, then the changes inside it, labelled `kb-write: record changes made before this batch`;
3. writes the batch;
4. commits the batch's changes, inside and outside `.index/`, labelled `kb-write: <ops> <ids>`.

A commit's label is its subject line. A label says where the changes are, not who made them: changes left by an earlier batch run without `commit` go into the commit for wherever they lie.
- `<ops>` is the batch's ops, each once, in entry order, joined by `,`; for an empty batch, the command's (`write` for `kbase write`). `<ids>` is the ids its entries entered or name in their `id`, `source` and `target`, each once, in entry order, joined by `,`.
- A snapshot commit that fails, for example on a hook, stops the batch before it writes: outcome `failed`, nothing written, since writing then would mix the batch into uncommitted changes.
- A batch that writes and removes nothing (`unchanged`) makes no batch commit. A batch commit that fails, for example on a hook, leaves the batch written; the result says so under `uncommitted`, with outcome `done`.
- Reverting the batch commit undoes exactly that batch and never the author's work. `commits` lists each commit made: `commit`, `label` and `paths`, the repository-relative paths it contains, a rename as both sides. A batch without `commit` has no safety net.

**`check: worktree`.** kbase runs git over a KB only where git's worktree root is the repository root containing the `.git` beside `kb-root/`. Outside a git worktree (remedy `git init`), where `GIT_DIR` or `GIT_WORK_TREE` redirects git, or where that `.git` is not one git accepts, so that git reads an enclosing repository, every command that runs git over the KB is refused `check: worktree` before it writes: a committing batch, a dry run included, and `update-format`, `init`, `build`, `status`, `cancel`, `wait` and `build-trail`.

