## 11. Package map

**Pattern:** Package responsibility listing.

The table lists every package by role. The dependency rules below it are the whole rule, and the
import graph they allow is acyclic. A row states what the package is for; the Mechanism column links
to the section that owns how it works.

| Package | Responsibility | Pattern | Mechanism |
|---|---|---|---|
| `cmd` | cobra root; one file per subcommand; `surface.go` is the single registration file | Generated API surface from a command registry | [§12](../12-constants/mcp-server-spec-11/index.md) |
| `internal/argmark` | The marker by which text names a tool argument (`Flag`, `Positional`, `FlagValue`) or contains a kbase call (`Call`, `Recorded`), and `Spelling.Spell`, each interface's formatting of it. Imports no kbase package | Per-surface presentation layer | [§9](../9-result-emitter/per-interface-rendering.md) |
| `internal/result` | The result and its emitter; the outcome-to-exit table; `Spelled`, `Verbatim` and `Reemit` | Single structured-output writer | [§9](../9-result-emitter/index.md), [§9](../9-result-emitter/per-interface-rendering.md) |
| `internal/latex/pandoc` | Reader, writer and defaults-run interface to pandoc (I1) | Wrapped external tool with a version gate | [§5](../5-reader-and-transforms/index.md) |
| `internal/latex/prepass` | The two source scans (I3) | Preprocessor registry with instrumentation | [§5](../5-reader-and-transforms/index.md) |
| `internal/docgraph` | JSON transforms, whole-volume render and cut, outline, records, partition checks, `validate_build` | Pandoc AST filter | [§5](../5-reader-and-transforms/index.md) |
| `internal/records` | The record schema `docgraph` writes and `claimgraph` reads | Sidecar metadata stream | [§5](../5-reader-and-transforms/index.md) |
| `internal/export` | The LaTeX export: `export.Run`, one sequence over unexported steps (the traversal, the inputs of `gen-tex/`, the embedded filter and template, the starter `preamble.tex`, the report) | Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked output directory | [§5](../5-reader-and-transforms/index.md) |
| `internal/kb` | The kb-root model: names, exclusions, links, the format stamp, the record loader `LoadGraph`, the scoring weights, `Source` (the read path), and re-exports of the live format `format40` | Domain model and schema package | [§6](../6-kb-model-and-maintenance/the-kb-package.md) |
| `internal/kbload` | `Open(kbRoot)`: reads the stamp, builds the `kb.Source`. Imports `buildrecords` for the root paths an older KB keeps its build records at | Schema validation at load, fail-closed | [§11](loading.md) |
| `internal/formats` | A tree of packages, one per format and one per converter; no file I/O. The root holds `Chain`, the converter list and the version arithmetic (`ParseVersion`, `CompareMinor`) | Chained schema migrations (upcasters) with golden tests | [§12](../12-constants/format-migration-spec-10/index.md) |
| `internal/formats/common` | What two or more of the superseded formats and converters use and no format defines (`Files`, `Warning`) | Chained schema migrations (upcasters) with golden tests | [§12](../12-constants/format-migration-spec-10/index.md) |
| `internal/index` | `Derive`, cycle breaking (`BreakCycles`), strength, summary pages, `RefreshReporting`, `UpdateFormat`, `PendingConversion`, the link checks, `SectionNodes`, `DependsGraph.Path` | Topological fold over a DAG (Kahn's order); materialized views rebuilt after each write | [§6](../6-kb-model-and-maintenance/the-derived-layer/index.md), [§6](../6-kb-model-and-maintenance/cycle-breaking.md) |
| `internal/write` | The write batch (`Run`) and the ops, values, store and render code behind it; provenance; the build-only ops | All-or-none validated batch with dry run and per-file atomic writes | [§6](../6-kb-model-and-maintenance/write-api/index.md), [§6](../6-kb-model-and-maintenance/build-only-ops.md) |
| `internal/atomicfile` | The atomic file writer; `WriteMode` sets a file's permission exactly, umask and a replaced file's mode aside. The temporary file is `.<name>.<random>.tmp`, beside the target for `Write` and `WriteMode`, in the temp directory it is given for `WriteIn`, which creates that directory holding a `.gitignore` of `*` where missing and rewrites a `.gitignore` that differs; removing the directory is its owner's (I13) | Write to a temporary file, then rename | [§6](../6-kb-model-and-maintenance/write-api/index.md) |
| `internal/kbwrite` | The write of a file under `kb-root/` (`Inside`: atomic, its temporary in `kb-root/.index/kb-temp/`, a symbolic link or a path leaving `kb-root/` refused) and the removal of that scratch (`RemoveTemps`) (I13). Imports no kbase package but `atomicfile`, `kb` and `result` | Write to a temporary file, then rename | [§2](../2-invariants.md), [§8](../8-ledger-state-store-and-monitoring/locks.md) |
| `internal/query` | The queries over `Derive`'s rows: `find`, `show`, `stats`, `edges`, `trace` | Read model | [§6](../6-kb-model-and-maintenance/the-query-layer.md) |
| `internal/sheet` | DOT emitter and `dot` invocation: rows, leaves and headings in, SVGs out | Graphviz rendering with a bounded process pool; a materialized view | [§6](../6-kb-model-and-maintenance/sheet.md) |
| `internal/buildrecords` | Readers for the four build records and their paths under `kb-root/.build/` (`Dir`, `RecordFiles`) and at the repository root (`RootRecordFile`), where a KB before `4.0.0` keeps them. Imported by `claimgraph`, by `kbdocs` (whether a build made the KB), and by `kbload` and `cmd/update-format.go` (the root paths the conversion moves) | Sidecar metadata stream | [§4](../4-stage-table/index.md) |
| `internal/kbdocs` | Readiness stamps, the maintained blocks' fences and rewrite (`Maintain`), README assembly, `SessionRule` and `IndexRule` | Scaffold command | [§6](../6-kb-model-and-maintenance/scratch-creation.md) |
| `internal/claimgraph` | The claim-graph stages: conform, inventory, identify, attribute, shortlist, unmarked, order, cull, split, elision (the cull's issuer), equation sites, classify, equations, endcap, hand-named mentions, assemble, write, the build-time check (`gate.go`), the four build records | Novel: the claim-graph stages `claims-declared`, `claims-discovered` and `analyses-split` (THESIS "What is new" item 1). Machinery: candidate generation then pairwise classification | [§4](../4-stage-table/index.md) |
| `internal/asks` | Embedded templates and fragments, composer, the multichoice-ask interface; `JudgedHash`, which `score`'s stale check and `write`'s value sets both call | Prompt templates with a lint test | [§6](../6-kb-model-and-maintenance/scoring/the-prompt-shelf.md) |
| `internal/asks/call` | The caller: multichoice model calls through a `Slot`'s `Decision`, the overview call through its `Chat`; captures; answer cache | Content-addressed memoization cache | [§8](../8-ledger-state-store-and-monitoring/index.md) |
| `internal/gitledger` | Git checkpoint commits, by exec (I1) | One adapter per external process | [§8](../8-ledger-state-store-and-monitoring/index.md) |
| `internal/filelock` | The advisory file lock behind the KB write lock, the run lock and the state store's holder lock | Advisory file lock with timeout | [§8](../8-ledger-state-store-and-monitoring/locks.md) |
| `internal/build` | The stage walk and each stage's behaviour, the state store's writers, resume, `--no-inference`, `--through`, progress and cancel, the development replay (`kbasedev`) | Checkpoint/restart pipeline | [§4](../4-stage-table/index.md), [§8](../8-ledger-state-store-and-monitoring/locks.md) |
| `internal/buildstore` | The build's state, read: store layout, holder and progress records, stage vocabulary, `status`, `WaitState`, the checkpoint-commit reading (`CurrentTrail`, `BuildTrail`), `KBRootExists`, `OpenRepo`. Reaches no `model` and no `asks/call` | Versioned on-disk schema with sequential migrations | [§8](../8-ledger-state-store-and-monitoring/index.md) |
| `internal/score` | The score passes: item text and its input guard, a run's scope and stale check, the parse, the runner, the run's entry (`Invoke`), its ledger, captures, recomputing values and run retention, the development replay. Imports not `model`; imported by `cmd` and `internal/mcp` only | Bounded worker pool over a job list, results written in one batch at the end | [§6](../6-kb-model-and-maintenance/scoring/the-runner.md) |
| `internal/statedir` | The state base `Base` (`$XDG_STATE_HOME/kbase`, `$XDG_STATE_HOME` defaulting to `~/.local/state` where unset or not an absolute path) and one KB's state directory: the override, else `<key>` under the base, never inside `kb-root/` | XDG state directory keyed by a path hash | [§8](../8-ledger-state-store-and-monitoring/index.md) |
| `internal/auth` | The credentials store | Local secrets file with a redacting secret type | [§7](../7-inference-seams/credentials.md) |
| `internal/secret` | `Key`, the type every key travels in; imports no kbase package | Local secrets file with a redacting secret type | [§7](../7-inference-seams/credentials.md) |
| `internal/config` | The configuration grammar, the three roles, `Resolve`, `Select`, the loaded files and templates, the configure commands' rules | Layered configuration | [§7](../7-inference-seams/index.md) |
| `internal/model` | `Queue` and its `Slot`, the only exported entry to inference; the two wire clients | Counting semaphore with a capability token | [§7](../7-inference-seams/index.md) |
| `internal/modelclient` | The client registry (`Registry`, one `model.Client` per endpoint, protocol and sampling) and the mapping of configuration's `api`, decision API and sampling onto `model`'s types (`chatProtocol`, `decisionsSurface`, `samplingOf`) | Factory registry with lazy, shared clients | [§7](../7-inference-seams/index.md) |
| `internal/mcp` | The MCP exposure of the command tree: tool table, help renderings, resources, instructions, the detached build and score launches. Imported by `cmd` alone | MCP server generated from the command tree | [§12](../12-constants/mcp-server-spec-11/index.md) |
| `internal/mcp/protocol` | JSON-RPC framing, dispatch, the `Tool`, `Schema` and `Property` types, the frame checks. Declares no subscribe capability | MCP server generated from the command tree | [§12](../12-constants/mcp-server-spec-11/index.md) |
| `internal/log` | The logging interface; formats each marked argument for the command line | Per-surface presentation layer | [§9](../9-result-emitter/per-interface-rendering.md) |
| `internal/version` | `Current`, the version string | Named constants registry | [§12](../12-constants/index.md) |

Contents:

- [§11 Loading](loading.md)
- [§11 The loader's file set and its other refusals](the-loader-s-file-set-and-its-other-refusals.md)
- [§11 The zero-value hazard](the-zero-value-hazard.md)
- [§11 Dependency rules](dependency-rules.md)
