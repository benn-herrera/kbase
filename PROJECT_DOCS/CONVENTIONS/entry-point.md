# CONVENTIONS – KBase

Project-specific process and house rules

Must-see:

- No user-facing behaviors or outcomes are designed here, that belongs to SPEC
- No project mechanisms or code structure are put forth, that belongs to ARCHITECTURE
- Edit PROJECT_DOCS/ leaves, never the root pointer files ([Project Documents](project-documents/index.md))
- Coding
  - When accepting dispatched-agent work, follow [CONVENTIONS, Coordinator Policy](coordinator-policy/index.md)
  - Audit the diff, not the report, before commit
- Briefing an agent role
  - A brief adds the task's facts to the agent role's definition; it never widens or replaces the role's question
  - Before briefing an agent role whose definition you have not read this session, read it

Contents:

- [Project Overview](project-overview.md)
- [Build and Run](build-and-run/index.md)
  - [No built binaries live in the repo](build-and-run/no-built-binaries-live-in-the-repo.md)
  - [Test data layout](build-and-run/test-data-layout.md)
- [Module Structure](module-structure.md)
- [Key Architecture Constraints](key-architecture-constraints.md)
- [House Rules](house-rules/index.md)
  - [The agent set is adjagent's, never edited here](house-rules/the-agent-set-is-adjagent-s-never-edited-here.md)
  - [Serialization formats](house-rules/serialization-formats.md)
  - [Superseded format parsers](house-rules/superseded-format-parsers.md)
  - [`kb-format` patch bumps](house-rules/kb-format-patch-bumps.md)
  - [Reads through `kb.Source`](house-rules/reads-through-kb-source.md)
  - [A result field's integer is `int`, never `int64`](house-rules/a-result-field-s-integer-is-int-never-int64.md)
  - [`just fmt` is the sanctioned formatter](house-rules/just-fmt-is-the-sanctioned-formatter.md)
  - [Never spell the node-kind list](house-rules/never-spell-the-node-kind-list.md)
  - [Never spell a relation](house-rules/never-spell-a-relation.md)
  - [A file kbase generates is regenerated whole](house-rules/a-file-kbase-generates-is-regenerated-whole.md)
  - [No stage exits on a model's opinion](house-rules/no-stage-exits-on-a-model-s-opinion.md)
  - [No test asserts a prompt's prose](house-rules/no-test-asserts-a-prompt-s-prose.md)
  - [A scoring-model fact lives once, in `internal/kb`](house-rules/a-scoring-model-fact-lives-once-in-internal-kb.md)
  - [A scoring value is derived in integer hundredths](house-rules/a-scoring-value-is-derived-in-integer-hundredths.md)
  - [Name the producer before you write the refusal](house-rules/name-the-producer-before-you-write-the-refusal.md)
  - [Compare a label in the page's spelling](house-rules/compare-a-label-in-the-page-s-spelling.md)
  - [Name a tool argument through the marker](house-rules/name-a-tool-argument-through-the-marker.md)
- [Plan & Execute Process](plan-execute-process/index.md)
  - [Surface audit, every third retired plan](plan-execute-process/surface-audit-every-third-retired-plan.md)
  - [Plan work rating and dispatch](plan-execute-process/plan-work-rating-and-dispatch.md)
  - [Plan dispatchability](plan-execute-process/plan-dispatchability.md)
- [Coordinator Policy](coordinator-policy/index.md)
  - [Audit the diff, not the report](coordinator-policy/audit-the-diff-not-the-report.md)
  - [Design fixes](coordinator-policy/design-fixes.md)
- [Testing](testing/index.md)
  - [No verification is a one-off](testing/no-verification-is-a-one-off.md)
  - [Integration tests invoke the app binary](testing/integration-tests-invoke-the-app-binary.md)
  - [Results are part of the test](testing/results-are-part-of-the-test.md)
- [Dependency Policy](dependency-policy.md)
- [Logging](logging.md)
- [Project Documents](project-documents/index.md)
  - [Section and rule heading brevity](project-documents/section-and-rule-heading-brevity.md)
