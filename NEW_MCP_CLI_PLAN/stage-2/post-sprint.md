## Post-sprint review

This section runs after package N. Two reviewers: an architect and a coder.

**What each reviewer receives:**
- Pre-task reading: [THESIS.md](../../PROJECT_DOCS/entry-point.md), [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md), [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md) and [CONVENTIONS.md](../../PROJECT_DOCS/CONVENTIONS/entry-point.md), and
  the [`NEW_MCP_CLI_DESIGN/`](../../NEW_MCP_CLI_DESIGN/entry-point.md) tree, which SPEC cites.
- Not this plan. The review judges the changes against the standards and against fitness for
  purpose. It does not retrace the implementation.
- The range from this stage's first package commit to its last.

**What each reviewer reviews:** every changed file, and the tests of every touched package
(`cmd`, `internal/write`, `internal/index`, `internal/sheet`, `internal/query`, `internal/mcp`,
`internal/gitledger`, `internal/claimgraph`, `internal/score`, and the recipes' scripts).

**Questions to answer, among others:**
- Does any path write records while their derived files stand stale, or write part of a refused
  batch?
- Does a committing batch commit anything outside `kb-root/`, or mix the batch into earlier
  uncommitted changes?
- Can a session do each of these from the served help alone, at the first call: add nodes then
  edges; remove a node; resolve a demoted record; cite a node?
- Does each help rendering equal its parts?
- Does any key reach argv, an MCP argument, a log line, an error string or a result?

**After the review:**
1. Merge the findings into a burndown at `ACTIVE_PLAN_REVIEW_BURNDOWN.md`.
2. Send the burndown to prompt-engineer review, and revise it from the findings.
3. Dispatch it as its own packages, each meeting "Plan dispatchability".
4. When the burndown has landed, this stage is retired, and [stage 3 begins](../stage-3/index.md).
