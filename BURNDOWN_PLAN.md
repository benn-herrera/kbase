# Burndown plan — the probe, the score status, and drift

Status: landed in `6e84c13`; post-sprint review and remediation in `8dc9488`; the burndown closed in
the landing commit on `release-0.8.0`.

## Post-sprint review

Once the landing is merged, an architect and a coder review the merged result against THESIS.md,
SPEC.md, ARCHITECTURE.md and CONVENTIONS.md, without this plan. Their subject is the changes against
those standards and whether the result is fit for its purpose — not how it was implemented — and their
brief includes reviewing the tests.

- **Review** — findings against the four documents, and tests that assert a document's claim rather
  than an implementation detail.
- **Remediation** — each finding lands in a follow-up commit or is declined with the reason recorded.
- **Burndown** — what remains, one fact per item, below.

### Remediation

Declined:

- **The charge-line prose test.** `TestScoreSystemStatesTheChargeLineForm` and its `chargeLineRE` are
  removed. "No test asserts a prompt's prose" governs; the fragment's form is carried by the parser's
  refusal and the prompt-engineer review. Residual: no test fails when the form sentence is dropped.
- **The four fixtures' second home.** `testAPIKey`, `newTestEndpoint`, `chatRequest` and `statusServer`
  stay one copy per package. A shared home needs a test-support package importing `model`, which
  `model`'s in-package tests cannot import without an import cycle; the Testing rule states it.
- **`usage.reported` as `PromptTokens > 0`.** The convention's read for a surface that sends no usage
  block; an explicit field would touch five sites for one return. Note only.

### Burndown

None outstanding.
