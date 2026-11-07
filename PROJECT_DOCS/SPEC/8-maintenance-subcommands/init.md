### `init`

```
kbase init [--title TEXT]
```

Creates a KB directly — no LaTeX, no build: the equivalent of `git init` for a KB. Run at any depth in a git worktree; it resolves the repository root and works at `kb-root/` in the git worktree root (placement: [§3](../3-compatibility-contract/index.md)).

**Refusals come before any write** ([§7](../7-tool-result-contract-and-exit-codes/item.md)):
- a `--title` containing a line break (CR or LF): `check: usage`, `key` the `title` argument as [§7](../7-tool-result-contract-and-exit-codes/item.md)'s `key` states it; the title is the entry point's one heading line;
- outside a git worktree: `check: worktree`, `path` the start directory, remedy `git init`, which the tool never runs itself;
- a `kb-root/` that exists in any state, an empty one included: `check: kb-root`, no remedy. `init` creates once, so a repeat where a tree exists is `refused`, never `unchanged`. A fresh `build` over an existing `kb-root/` gives the same refusal ([§5](../5-build-behaviour/starting-over.md)): one item, `path` the `kb-root/`, the same detail. The result of a refusal that found no `kb-root/` has no `kb-root` key; the refusal items name the paths.

Under the KB write lock, held from before the directory exists ([One write lock](write-batches/one-write-lock.md)), `init` writes:
- `entry-point.md`, with the `--title` text, or the repository's directory name, as its only heading, no children and no frontmatter ([§10](../10-metadata-format-version/documents-of-a-kb.md));
- the format version file, `.index/format.yaml` ([§10](../10-metadata-format-version/the-record-files.md)), and the two record files, `.index/nodes.yaml` and `.index/edges.yaml`, containing no record;
- `AGENTS.md`, `CONVENTIONS.md` and the `CLAUDE.md` redirect, each only where absent. `AGENTS.md` states that this KB is authored, not distilled: its author writes what it contains, a claim at a time.

It then rebuilds the derived views, so they are present when the KB is created: the full sheet is always drawn, the placeholder where `dot` is absent ([§3](../3-compatibility-contract/index.md)); the volume overview diagram and the per-volume sheets are not drawn, since no volume contains a node. `verify` ([§8](verify.md)) over a freshly created KB is green.

Finally it commits `kb-root/` with the plain subject `kb-init: knowledge base created`, never a `kb-build:` subject. So the build's checkpoint commits ([§5](../5-build-behaviour/index.md)) are absent, a `build --resume` finds nothing to resume, and a fresh `build` is refused over an initialized KB as `init` is: content joins through [§8](index.md)'s ops, never a later build. The commit uses the host's git identity and configuration. Git's own refusal (no identity configured) is `failed`, naming git's words, with the written tree left uncommitted for the operator to commit or remove. Result keys: [§7](../7-tool-result-contract-and-exit-codes/item.md).

