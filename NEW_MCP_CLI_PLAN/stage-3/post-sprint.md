## Post-sprint review

This section runs after package R. Two reviewers: an architect and a coder.

**What each reviewer receives:**
- Pre-task reading: [THESIS.md](../../PROJECT_DOCS/entry-point.md), [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md), [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md) and [CONVENTIONS.md](../../PROJECT_DOCS/CONVENTIONS/entry-point.md), and
  the [`NEW_MCP_CLI_DESIGN/`](../../NEW_MCP_CLI_DESIGN/entry-point.md) tree, which SPEC cites.
- Not this plan. The review judges the changes against the standards and against fitness for
  purpose. It does not retrace the implementation.
- The range from this stage's first package commit to its last.

**What each reviewer reviews:** every changed file, and the tests of every touched package
(`cmd`, `internal/query`, `internal/index`, `internal/kb`, `internal/kbload`, `internal/score`,
`internal/write`, `internal/build`, `internal/buildstore`, `internal/claimgraph`, `internal/mcp`,
and the recipes' scripts).

**Questions to answer, among others:**
- Is the judged hash computed in one place, and does a manual set record it?
- Does `score` ask only what is dirty, and land only through the edit ops?
- Does any list query cut, page or reorder its answer outside its stated order?
- Is `trace`'s `contribution` the scoring code's value, never a restated formula?
- Does anything in kbase still issue an op that stage 4 removes?
- Can a session score only what changed, and rewind a build, from the served help alone?
- Does any key reach argv, an MCP argument, a log line, an error string or a result?

**After the review:**
1. Merge the findings into a burndown at `ACTIVE_PLAN_REVIEW_BURNDOWN.md`.
2. Send the burndown to prompt-engineer review, and revise it from the findings.
3. Dispatch it as its own packages, each meeting "Plan dispatchability".
4. When the burndown has landed, this stage is retired, and [stage 4 begins](../stage-4/index.md).
