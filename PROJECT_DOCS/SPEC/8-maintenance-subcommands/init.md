### `init`

```
kbase init [--title TEXT]
```

Creates a KB directly — no LaTeX, no build: the equivalent of `git init` for a KB. Run at any depth
in a git worktree; it resolves the repository root and works at `kb-root/` in the git worktree root ([§1](../1-what-kbase-is.md)'s
placement). **Refusals come before any write** ([§7](../7-tool-result-contract-and-exit-codes/index.md) item shape): a `--title` holding a line break
(CR or LF) — `check: usage`, `key` the `title` argument, spelled as [§7](../7-tool-result-contract-and-exit-codes/item.md)'s `key` states: the title is the entry point's one heading line;
outside a git worktree — `check: worktree`, `path` the start directory, remedy `git init`, which
the tool never runs itself; and
against a `kb-root/` that exists in any state, an empty one included — `check: kb-root`, no remedy:
creation is birth-once, a re-issue where a tree stands is `refused`, never `unchanged`. That refusal
is the one a fresh `build` gives over an existing `kb-root/` ([page §3.7 `init`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/init.md), [page §3.5
`build`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)): one item, `path` the `kb-root/`, the same detail. The result of
a refusal that found no `kb-root/` carries no `kb-root` key; the refusal items name the paths.

Then, under the KB write lock from before the directory exists ([§8](index.md)): it writes `entry-point.md` —
with the `--title` text, or the repository's directory name, as the only heading, no children and
no frontmatter ([§10](../10-metadata-format-version/index.md)) — the format stamp file, `.index/format.yaml` ([§10](../10-metadata-format-version/index.md)), and the two record files,
`.index/nodes.yaml` and `.index/edges.yaml`, holding no record; stamps the readiness documents, `AGENTS.md`,
`CONVENTIONS.md` and the `CLAUDE.md` redirect, each only where absent, `AGENTS.md` pinned that this
KB is authored, not distilled — its author writes what it holds, a claim at a time; then
refreshes, so the derived metadata layer is present at birth
([§3](../3-compatibility-contract/index.md)'s full sheet always, the placeholder where `dot` is absent, the digest and volume sheets
not drawn: no volume holds a node). Every input to the skeleton is kbase's own, so `init` runs no
`verify` of its own; `verify` ([§8](index.md)) over a freshly born KB is green. Finally the birth commit: `kb-root/` committed with the plain subject
`kb-init: knowledge base created` — never a `kb-build:` subject, so the trail of [§5](../5-build-behaviour/index.md) stays empty,
a `build --resume` finds nothing to resume, and a fresh `build` is refused over an initialized KB as
`init` is: content joins
[§8](index.md)'s ops, never a later build. The commit rides the host's git identity and configuration; git's own
refusal (no identity configured) is a `failed` naming git's words, the written tree standing
uncommitted for the operator to commit or remove. Result keys: [§7](../7-tool-result-contract-and-exit-codes/index.md).

