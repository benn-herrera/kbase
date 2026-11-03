#### `init` (tool + CLI)

This creates an empty KB at `kb-root/`: no LaTeX, no build. It writes:
- `entry-point.md`, with the title as its only heading;
- `.index/format.yaml` and the two empty record files;
- `AGENTS.md`, `CONVENTIONS.md` (each with its maintained block) and the `CLAUDE.md` redirect;
- the derived skeleton (the sheet, or its placeholder where `dot` is absent).

It then commits `kb-root/` as `kb-init: knowledge base created`.

- **Arguments**: `title` (default: the repository's directory name).
- **Result**: `written`, `removed`.
- **Refusals**:
  - `kb-root/` exists, in any state (`check: kb-root`, no remedy): you cannot create what already
    exists;
  - outside a git worktree: remedy `git init`;
  - a title holding a line break ([§2.2](../../2-conventions/2.2-write-batches.md), Values);
  - git refusing the commit (`failed`, git's words, the tree left uncommitted).
