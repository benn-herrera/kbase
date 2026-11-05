### `kbase build-trail`

`build-trail` lists the build's history, read-only. It runs none of the commands it lists. Its one
argument is `state-dir`; it takes no target, since every boundary and `started-at` has its own
command. The checkpoint commits it reads are the newest build's ([§5](../5-build-behaviour/starting-over.md)).

Its result has these keys after `kb-root`, in this order:

- `state-dir`;
- `head`: the commit `HEAD` names, null on a branch with no commit;
- `boundaries`, oldest first, each with `stage`, `commit`, `subject` and `command`;
- `started-at`: `commit` and `command`, null where the checkpoint commits record none;
- `dirty`: `false`, or the worktree's changed paths, the first 20 named and the rest counted as
  `+N more`.

A `command` is a list of the command lines to run in order. A boundary's is `git reset --keep
<commit>` then the build resumed with the flags its last run was launched with, `--through`
excepted, each written as the interface takes it: `kbase build --resume` and those flags at the
command line, the `build` tool's call over MCP. `started-at`'s is the reset alone. `git reset
--keep` refuses rather than discarding uncommitted changes in the files the reset touches.

`build-trail` is refused outside a git worktree (`check: worktree`), over a state store `status`
would refuse (`check: state-dir`, or `check: usage` for a `--state-dir` inside `kb-root/`), and over
checkpoint commits that record a stage over an unrecorded earlier one (`check: trail`, as
[Starting over](../5-build-behaviour/starting-over.md) states). `failed` (3) is a git failure.

---

