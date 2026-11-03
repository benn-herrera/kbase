## Post-sprint review

This section runs after package F. Two reviewers: an architect and a coder.

**What each reviewer receives:**
- Pre-task reading: [THESIS.md](../../PROJECT_DOCS/entry-point.md), [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md), [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md) and [CONVENTIONS.md](../../PROJECT_DOCS/CONVENTIONS/entry-point.md), and
  the [`NEW_MCP_CLI_DESIGN/`](../../NEW_MCP_CLI_DESIGN/entry-point.md) tree, which SPEC cites.
- Not this plan. The review judges the changes against the standards and against fitness for
  purpose. It does not retrace the implementation.
- The range from this stage's first package commit to its last.

**What each reviewer reviews:** every changed file, and the tests of every touched package
(`cmd`, `internal/formats`, `internal/kb`, `internal/index`, `internal/write`, `internal/query`,
`internal/kbdocs`, `internal/kbload`, `internal/buildrecords`, `internal/buildstore`,
`internal/gitledger`, `internal/claimgraph`, `internal/build`, and the recipes' scripts).

**Questions to answer, among others:**
- Does the 4.0.0 conversion do exactly what [page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) lists?
- Do the conversion steps leave a 4.0.0 KB's records and prose unchanged?
- Is every reported item, bare ids and dropped texts included, in the `update-format` result?
- Does a converted KB pass `verify`'s dead-link gate?
- Does any path but `update-format` write to a KB in an older format?
- Is any committed fixture written by hand where the tool should produce it?
- Does any key reach argv, an MCP argument, a log line, an error string or a result?

**After the review:**
1. Merge the findings into a burndown at `ACTIVE_PLAN_REVIEW_BURNDOWN.md`.
2. Send the burndown to prompt-engineer review, and revise it from the findings.
3. Dispatch it as its own packages, each meeting "Plan dispatchability".
4. When the burndown has landed, this stage is retired, and [stage 2 begins](../stage-2/index.md).
