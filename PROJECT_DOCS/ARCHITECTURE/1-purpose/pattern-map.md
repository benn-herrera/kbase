### Pattern map

One row per `##` and `###` heading of this document, in document order. `Novel:` marks the novel
parts above; every other row is a standard pattern. The last column names the row of THESIS's
building-blocks table that the section implements. A `####` heading carries its own pattern in its
opening line and has no row here.

| Section | Link | Pattern | THESIS building block |
|---|---|---|---|
| 1. Purpose | [§1](index.md) | — | — |
| Pattern map | [§1](pattern-map.md) | — | — |
| 2. Invariants | [§2](../2-invariants.md) | — | — |
| 3. Principles | [§3](../3-principles.md) | — | — |
| 4. Stage table | [§4](../4-stage-table/index.md) | Checkpoint/restart pipeline | Builds and scoring runs |
| Finding claims | [§4](../4-stage-table/finding-claims/index.md) | Novel: what counts as a claim. Machinery: AST extraction; LLM classification of prose spans | The claim graph; Asking a model |
| 5. Reader and transforms | [§5](../5-reader-and-transforms/index.md) | Pandoc as the parser; Go transforms over its JSON AST | Reading LaTeX |
| 5.1 pandoc seam | [§5.1](../5-reader-and-transforms/5.1-pandoc-seam.md) | Wrapped external tool with a version gate | Reading LaTeX |
| 5.2 Pre-pass registry | [§5.2](../5-reader-and-transforms/5.2-pre-pass-registry.md) | Preprocessor registry with instrumentation | Reading LaTeX |
| 5.3 JSON transforms | [§5.3](../5-reader-and-transforms/5.3-json-transforms.md) | Pandoc AST filter | Reading LaTeX |
| 5.4 Writer route | [§5.4](../5-reader-and-transforms/5.4-writer-route.md) | Off-the-shelf writer | Reading LaTeX |
| 5.5 Records | [§5.5](../5-reader-and-transforms/5.5-records.md) | Sidecar metadata stream | The knowledge base |
| 5.6 Tree derivation, partition checks and `validate_build` | [§5.6](../5-reader-and-transforms/5.6-tree-derivation-partition-checks-and-validate-build.md) | Round-trip conformance checks and a link checker | Checks |
| 5.7 LaTeX export | [§5.7](../5-reader-and-transforms/5.7-latex-export/index.md) | Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked output directory | — |
| 6. KB model and maintenance | [§6](../6-kb-model-and-maintenance/index.md) | — | The knowledge base |
| The `kb` package | [§6](../6-kb-model-and-maintenance/the-kb-package.md) | Domain model and schema package | The knowledge base |
| Write API | [§6](../6-kb-model-and-maintenance/write-api/index.md) | All-or-none validated batch with dry run and per-file atomic writes | Writes |
| Provenance | [§6](../6-kb-model-and-maintenance/provenance.md) | Manual-override flag | — |
| Build-only ops | [§6](../6-kb-model-and-maintenance/build-only-ops.md) | Internal privileged ops | Writes |
| The node vocabulary | [§6](../6-kb-model-and-maintenance/the-node-vocabulary.md) | Novel: the node kinds. Machinery: prefixed random ids | The claim graph |
| The derived layer | [§6](../6-kb-model-and-maintenance/the-derived-layer/index.md) | Novel: the strength formula. Machinery: topological fold over a DAG (Kahn's order); materialized views rebuilt after each write | Derived pages and sheets; The claim graph |
| Cycle breaking | [§6](../6-kb-model-and-maintenance/cycle-breaking.md) | Greedy feedback-arc removal by DFS back edges | The claim graph |
| The query layer | [§6](../6-kb-model-and-maintenance/the-query-layer.md) | Read model | Queries |
| Frontmatter reading | [§6](../6-kb-model-and-maintenance/frontmatter-reading.md) | Heuristic sniffing before parse | The knowledge base |
| Sheet | [§6](../6-kb-model-and-maintenance/sheet.md) | Graphviz rendering with a bounded process pool; a materialized view | Derived pages and sheets |
| Scratch creation | [§6](../6-kb-model-and-maintenance/scratch-creation.md) | Scaffold command | — |
| Scoring | [§6](../6-kb-model-and-maintenance/scoring/index.md) | Job runner with an append-only event log, resume from the last status per item, optimistic-concurrency commit, projection rebuild | Builds and scoring runs; Model calls; Asking a model |
| 7. Inference seams | [§7](../7-inference-seams/index.md) | Role-based model routing behind provider adapters | Model calls; Configuration and credentials |
| Probe | [§7](../7-inference-seams/probe.md) | Health-check / smoke call | Model calls |
| Wire clients | [§7](../7-inference-seams/wire-clients.md) | Provider adapters behind one interface; connection pooling | Model calls |
| Retry and attempt outcomes | [§7](../7-inference-seams/retry-and-attempt-outcomes.md) | Retry with backoff on retryable status | Model calls |
| Refused keys | [§7](../7-inference-seams/refused-keys.md) | Typed error classification at the boundary | Configuration and credentials |
| Layered configuration | [§7](../7-inference-seams/layered-configuration.md) | Layered configuration | Configuration and credentials |
| Resolution and the client registry | [§7](../7-inference-seams/resolution-and-the-client-registry.md) | Factory registry with lazy, shared clients | Model calls; Configuration and credentials |
| The decision API | [§7](../7-inference-seams/the-decision-api.md) | Constrained classification endpoint | Asking a model |
| The multichoice-ask interface | [§7](../7-inference-seams/the-multichoice-ask-interface.md) | Dependency injection with an architecture fitness test | Asking a model |
| Credentials | [§7](../7-inference-seams/credentials.md) | Local secrets file with a redacting secret type | Configuration and credentials |
| 8. Ledger, state store and monitoring | [§8](../8-ledger-state-store-and-monitoring/index.md) | Checkpoint commits, a content-addressed answer cache, and a background job with a status file | Builds and scoring runs |
| Checkpoint commits and resume | [§8](../8-ledger-state-store-and-monitoring/checkpoint-commits-and-resume.md) | Checkpoint/restart pipeline; stage = idempotent unit | Builds and scoring runs |
| Preconditions | [§8](../8-ledger-state-store-and-monitoring/preconditions.md) | Precondition checks | Builds and scoring runs |
| Regenerating missing records | [§8](../8-ledger-state-store-and-monitoring/regenerating-missing-records.md) | Reproducible-build verification | Checks |
| The answer cache | [§8](../8-ledger-state-store-and-monitoring/the-answer-cache.md) | Content-addressed memoization cache | Builds and scoring runs; Asking a model |
| State store layout and format | [§8](../8-ledger-state-store-and-monitoring/state-store-layout-and-format.md) | Versioned on-disk schema with sequential migrations | Builds and scoring runs |
| State store retention | [§8](../8-ledger-state-store-and-monitoring/state-store-retention.md) | Keep-last-N retention | Builds and scoring runs |
| Locks | [§8](../8-ledger-state-store-and-monitoring/locks.md) | Repo-scoped mutex lockfile plus a pidfile-style holder record | Builds and scoring runs |
| Status, progress, cancel and wait | [§8](../8-ledger-state-store-and-monitoring/status-progress-cancel-and-wait.md) | Background job with a status file, signal cancel and a polling wait | Builds and scoring runs |
| The build's development replay | [§8](../8-ledger-state-store-and-monitoring/the-build-s-development-replay.md) | Offline eval / record-replay regression harness | Builds and scoring runs; Checks |
| 9. Result emitter | [§9](../9-result-emitter/index.md) | Single structured-output writer | The CLI and MCP server |
| Per-interface rendering | [§9](../9-result-emitter/per-interface-rendering.md) | Per-surface presentation layer | The CLI and MCP server |
| 10. Compatibility harness | [§10](../10-compatibility-harness.md) | Black-box integration tests with golden fixtures | Checks |
| 11. Package map | [§11](../11-package-map/index.md) | Package responsibility listing | — |
| Loading | [§11](../11-package-map/loading.md) | Schema validation at load, fail-closed | The knowledge base |
| The loader's file set and its other refusals | [§11](../11-package-map/the-loader-s-file-set-and-its-other-refusals.md) | Schema validation at load, fail-closed | The knowledge base |
| The zero-value hazard | [§11](../11-package-map/the-zero-value-hazard.md) | Go zero-value pitfall; required-field decoding | — |
| Dependency rules | [§11](../11-package-map/dependency-rules.md) | Architecture fitness functions | Checks |
| 12. Constants | [§12](../12-constants/index.md) | Named constants registry | — |
| Format migration (SPEC §10) | [§12](../12-constants/format-migration-spec-10/index.md) | Chained schema migrations (upcasters) with golden tests | Format versions |
| MCP server (SPEC §11) | [§12](../12-constants/mcp-server-spec-11/index.md) | MCP server generated from the command tree; JSON-RPC over stdio; detached child plus status polling | The CLI and MCP server |
| 13. Slice verdicts | [§13](../13-slice-verdicts/index.md) | Spike/prototype decision record | — |
| Slice-1 checks | [§13](../13-slice-verdicts/slice-1-checks.md) | — | — |
| 14. Limits | [§14](../14-limits/index.md) | Known-limitations section | — |
| A conversion records no history | [§14](../14-limits/a-conversion-records-no-history.md) | — | — |
| Merged metadata is refused, not merged | [§14](../14-limits/merged-metadata-is-refused-not-merged.md) | — | — |
| No format sits between two majors | [§14](../14-limits/no-format-sits-between-two-majors.md) | — | — |
| No refresh moves a judgement | [§14](../14-limits/no-refresh-moves-a-judgement.md) | — | — |

