### Output and placement

The KB is written only at `kb-root/` in the git worktree root. The build records live in `kb-root/.build/`
([page §2.5](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md)), so every path the build commits is under `kb-root/`. The state store ([§6](../6-build-state-ledger-and-monitoring/index.md)) is
never written into `kb-root/`.

**Fresh versus resume** is derived from the `kb-build:` commit trail ([§6](../6-build-state-ledger-and-monitoring/index.md)), never configured.
`kbase build <volume-root>...` starts a fresh build where no `kb-root/` stands; `kbase build
--resume` continues the trail. A fresh build where `kb-root/` exists, in any state, is refused
([§5 Starting over](starting-over.md)). The KB a build writes holds every title unique within its kind ([page §3.5
`build`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)), and its writes go through the write ops, each followed by the derivation ([page §2.3](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)):
the derived files are current at every stage commit from the spine seed on. A link pandoc wraps
around a cross-reference, its destination the bare LaTeX label, is unwrapped to the
cross-reference inside it, which links the target document's heading.
`build` refuses outside a git worktree, naming `git init`. It refuses if the kbase-owned paths it
commits are dirty and the dirt is not an interrupted stage's own uncommitted work ([§6](../6-build-state-ledger-and-monitoring/index.md)), and only
then.

