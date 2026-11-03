#### `build-trail` (tool + CLI)

This lists the build's history, read-only. Each stage boundary comes with its commit and the exact
command that rewinds to it. `started-at` comes with the command that starts over. It runs none of
them. A boundary's command is the reset followed by `kbase build --resume` with the flags the
build's last run was launched with; the start-over command
is the reset alone. The reset is `git reset --keep <commit>`, which refuses rather than discarding
uncommitted changes in files the reset touches. It takes no target argument: every boundary and
`started-at` carries its own command.

- **Arguments**: `state-dir`.
- **Result**: `state-dir`, `head`, `boundaries` (oldest first: `stage`, `commit`, `subject`,
  `command`), `started-at` (`commit`, `command`; null where unrecorded), `dirty` (false, or the
  changed paths, the first 20 named and the rest counted).
- **Refusals**: outside a worktree; a state store `status` would refuse; a trail that records a
  stage over an unrecorded earlier one (`check: trail`).
