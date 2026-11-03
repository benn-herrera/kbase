## Post-sprint review

This section runs after the surface comparison. Two reviewers: an architect and a coder.

**What each reviewer receives:**
- Pre-task reading: [THESIS.md](../../PROJECT_DOCS/entry-point.md), [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md), [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md) and [CONVENTIONS.md](../../PROJECT_DOCS/CONVENTIONS/entry-point.md), and
  the [`NEW_MCP_CLI_DESIGN/`](../../NEW_MCP_CLI_DESIGN/entry-point.md) tree, which SPEC cites.
- Not this plan. The review judges the changes against the standards and against fitness for
  purpose. It does not retrace the implementation.
- The range from this stage's first package commit to its last.

**What each reviewer reviews:** every changed file, and the tests of every touched package
(`cmd`, `internal/write`, `internal/query`, `internal/index`, `internal/export`, `internal/config`,
`internal/auth`, `internal/mcp`, `internal/kbdocs`, `internal/kbload`, the recipes' scripts and the
`justfile`).

**Questions to answer, among others:**
- Does any removed op, query or flag survive anywhere a session can reach: help, tools, resources,
  instructions, templates or `README.md`?
- Does the op script still prove that a re-issue leaves `kb-root/` unchanged?
- Can a session do each of these from the served text alone, at the first call: add nodes then
  edges; remove a node; resolve a demoted record; score only what changed; convert after an upgrade;
  configure a provider and its models?
- Does each help rendering equal its parts?
- Does any key reach argv, an MCP argument, a log line, an error string or a result?

**After the review:**
1. Merge the findings into a burndown at `ACTIVE_PLAN_REVIEW_BURNDOWN.md`. No burndown item adds a
   test, recipe or gate that compares the served surface with the page.
2. Send the burndown to prompt-engineer review, and revise it from the findings.
3. Dispatch it as its own packages, each meeting "Plan dispatchability".
4. When the burndown has landed, this stage is retired, and format `4.0.0` is ready to ship.
