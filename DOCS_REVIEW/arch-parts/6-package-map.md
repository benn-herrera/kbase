## 11. Package map

**Pattern:** Package responsibility listing.

The table lists every package by role. The dependency rules below it are the whole rule, and the
import graph they allow is acyclic. A row states what the package is for; the Mechanism column links
to the section that owns how it works.

| Package | Responsibility | Pattern | Mechanism |
|---|---|---|---|
| `cmd` | cobra root; one file per subcommand; `surface.go` is the single registration file | Generated API surface from a command registry | [§12](#mcp-server-spec-11) |
| `internal/argmark` | The marker by which text names a tool argument (`Flag`, `Positional`, `FlagValue`) or contains a kbase call (`Call`, `Recorded`), and `Spelling.Spell`, each interface's formatting of it. Imports no kbase package | Per-surface presentation layer | [§9](#per-interface-rendering) |
| `internal/result` | The result document and its emitter; the outcome-to-exit table; `Spelled`, `Verbatim` and `Reemit` | Single structured-output writer | §9, [§9](#per-interface-rendering) |
| `internal/latex/pandoc` | Reader, writer and defaults-run interface to pandoc (I1) | Wrapped external tool with a version gate | §5 |
| `internal/latex/prepass` | The two source scans (I3) | Preprocessor registry with instrumentation | §5 |
| `internal/docgraph` | JSON transforms, whole-volume render and cut, outline, records, partition checks, `validate_build` | Pandoc AST filter | §5 |
| `internal/records` | The record schema `docgraph` writes and `claimgraph` reads | Sidecar metadata stream | §5 |
| `internal/export` | The LaTeX export: `export.Run`, one sequence over unexported steps (the traversal, the inputs of `gen-tex/`, the embedded filter and template, the starter `preamble.tex`, the report) | Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked output directory | §5 |
| `internal/kb` | The kb-root model: names, exclusions, links, the format stamp, the record loader `LoadGraph`, the scoring weights, `Source` (the read path), and re-exports of the live format `format400` | Domain model and schema package | [§6](#the-kb-package) |
| `internal/kbload` | `Open(kbRoot)`: reads the stamp, builds the `kb.Source`. Imports `buildrecords` for the root paths an older KB keeps its build records at | Schema validation at load, fail-closed | [§11](#loading) |
| `internal/formats` | A tree of packages, one per format and one per converter; no file I/O | Chained schema migrations (upcasters) with golden tests | [§12](#format-migration-spec-10) |
| `internal/index` | `Derive`, cycle breaking (`BreakCycles`), strength, summary pages, `RefreshReporting`, `UpdateFormat`, `PendingConversion`, the link checks, `SectionNodes`, `DependsGraph.Path` | Topological fold over a DAG (Kahn's order); materialized views rebuilt after each write | [§6](#the-derived-layer), [§6](#cycle-breaking) |
| `internal/write` | The write batch (`Run`) and the ops, values, store and render code behind it; provenance; the build-only ops | Transactional unit of work with dry run | [§6](#write-api), [§6](#build-only-ops) |
| `internal/atomicfile` | The atomic file writer; `WriteMode` sets a file's permission exactly, umask and a replaced file's mode aside | Write to a temporary file, then rename | [§6](#write-api) |
| `internal/query` | The queries over `Derive`'s rows: `find`, `show`, `stats`, `edges`, `trace` | Read model | [§6](#the-query-layer) |
| `internal/sheet` | DOT emitter and `dot` invocation: rows, leaves and headings in, SVGs out | Graphviz rendering with a bounded process pool; a materialized view | [§6](#sheet) |
| `internal/buildrecords` | Readers for the four build records and their paths under `kb-root/.build/` (`Dir`, `RecordFiles`) and at the repository root (`RootRecordFile`), where a KB before `4.0.0` keeps them | Sidecar metadata stream | §4 |
| `internal/kbdocs` | Readiness stamps, the maintained blocks' fences and rewrite (`Maintain`), README assembly, `SessionRule` and `IndexRule` | Scaffold command | [§6](#scratch-creation) |
| `internal/claimgraph` | The claim-graph stages: conform, inventory, identify, attribute, shortlist, unmarked, order, cull, split, classify, equations, endcap, assemble, write, check, the four build records | Candidate generation then pairwise classification; `Novel:` for the claim-graph rows | §4 |
| `internal/asks` | Embedded templates and fragments, composer, the multichoice-ask interface; `JudgedHash`, which `score`'s stale check and `write`'s value sets both call | Prompt templates with a lint test | [§6](#the-prompt-shelf) |
| `internal/asks/call` | The caller: multichoice model calls through a `Slot`'s `Decision`, the overview call through its `Chat`; captures; answer cache | Content-addressed memoization cache | §8 |
| `internal/gitledger` | Git checkpoint commits, by exec (I1) | One adapter per external process | §8 |
| `internal/filelock` | The advisory file lock behind the KB write lock, the run lock and the state store's holder lock | Advisory file lock with timeout | [§8](#locks) |
| `internal/build` | The stage walk and each stage's behaviour, the state store's writers, resume, `--no-inference`, `--through`, progress and cancel, the development replay (`kbasedev`) | Checkpoint/restart pipeline | §4, [§8](#locks) |
| `internal/buildstore` | The build's state, read: store layout, holder and progress records, stage vocabulary, `status`, `WaitState`, the checkpoint-commit reading (`CurrentTrail`, `BuildTrail`), `KBRootExists`, `OpenRepo`. Reaches no `model` and no `asks/call` | Versioned on-disk schema with sequential migrations | §8 |
| `internal/score` | The score passes: item text and its input guard, a run's scope and stale check, the parse, the runner, the run's entry (`Invoke`), its ledger, captures, recomputing values and run retention, the development replay. Imports not `model`; imported by `cmd` and `internal/mcp` only | Bounded worker pool over a job list, results written in one batch at the end | [§6](#the-runner) |
| `internal/statedir` | The state base `Base` (`$XDG_STATE_HOME/kbase`, `$XDG_STATE_HOME` defaulting to `~/.local/state` where unset or not an absolute path) and one KB's state directory: the override, else `<key>` under the base, never inside `kb-root/` | XDG state directory keyed by a path hash | §8 |
| `internal/auth` | The credentials store | Local secrets file with a redacting secret type | [§7](#credentials) |
| `internal/secret` | `Key`, the type every key travels in; imports no kbase package | Local secrets file with a redacting secret type | [§7](#credentials) |
| `internal/config` | The configuration grammar, the three roles, `Resolve`, `Select`, the loaded files and templates, the configure commands' rules | Layered configuration | §7 |
| `internal/model` | `Queue` and its `Slot`, the only exported entry to inference; the two wire clients | Counting semaphore with a capability token | §7 |
| `internal/modelclient` | The client registry (`Registry`, one `model.Client` per endpoint, protocol and sampling) and the mapping of configuration's `api`, decision API and sampling onto `model`'s types (`chatProtocol`, `decisionsSurface`, `samplingOf`) | Factory registry with lazy, shared clients | §7 |
| `internal/mcp` | The MCP exposure of the command tree: tool table, help renderings, resources, instructions, the detached build and score launches. Imported by `cmd` alone | MCP server generated from the command tree | [§12](#mcp-server-spec-11) |
| `internal/mcp/protocol` | JSON-RPC framing, dispatch, the `Tool`, `Schema` and `Property` types, the frame checks. Declares no subscribe capability | MCP server generated from the command tree | [§12](#mcp-server-spec-11) |
| `internal/log` | The logging interface; formats each marked argument for the command line | Per-surface presentation layer | [§9](#per-interface-rendering) |
| `internal/version` | `Current`, the version string | Named constants registry | §12 |

### Loading

**Pattern:** Schema validation at load, fail-closed.

Every KB file is read through `kb.Source`, built by `kbload.Open` (I8). `Open` constructs a new
`Source` on each call and no package-level state passes bytes from one load to the next. An older KB
arrives already in the current form, and no verb sees the on-disk bytes of an unmigrated one.
`cmd/readpolicy_test.go` enforces the single read path ([§11](#dependency-rules)).

The loader keeps the conversion record. `Source.Migration()` contains the chain's `From`, `To`, the
one-sentence `Note` (`kb.MigrationNote`) and the conversion's `Warnings`. The record is set where the
source was migrated and kept across the save that stamps the current format. Every call on an older
KB until that save therefore returns the `migration` record and writes its stderr lines: the
sentence, then one per warning (`migrationNotice` in `cmd/update-format.go`;
SPEC §10 The migration announces itself). `build` is the one loader that returns it nowhere: a
mid-build KB is not a finished state, and its checkpoint commits own that history.

`update-format` alone saves a migrated source (`index.UpdateFormat`). Every other writer refuses it
through `index.PendingConversion` before it writes: `write.Run`, `score`'s verb before its start
rebuild of the derived views, and the build's resume (`loadFormat`). No write lands on an unconverted
tree. The conversion's result is formatted in [§12](#format-migration-spec-10).
`kb.MigratedSource` carries the paths the conversion made obsolete, and the save removes them.

**The strict load** is `kb.LoadGraph`'s, at the load of every command that builds a state.
`format400.DecodeNodes` and `DecodeEdges` read both record files and return every
`format400.GraphFault{Check, Path, Line, Key, Allowed, Remedy, Detail}`, before any state exists:

- a file that does not parse;
- an unknown or missing field, a scored record's `judged-hash` among them;
- a wrong type or a value outside its domain;
- an id outside its kind's grammar, or a node id twice;
- a pairing `BearingLegal` refuses.

No summary page is rendered and no op runs. An absent record file is a fault of its own
(`check: index`, remedy "restore it from git"). The faults arrive as one `kb.GraphLoadError`, which
`index`'s refusal path (`graphItems`, `internal/index/derive.go`) maps to `result` items: `check:
metadata`, the field as `key`, the file and line. Every reader of a state, `write` included, refuses
with them.

**The integrity faults refuse too** (SPEC §10 The load: every fault refused); there is no degraded
state. Where both files decode clean, `kb.LoadGraph`'s `edgeIntegrity` adds a fault per edge record
naming a node no record has and per second record of one edge identity, each `check: metadata` with
`kb.ConsistentRecordsRemedy`. A cycle among the scoring edges is `index`'s: `deriveGraph` runs
`BreakCycles` on every load ([§6](#cycle-breaking)), and `Derive` refuses each edge the cycle
breaking removed as a `check: metadata` item naming it. The cycle stays out of `kb.LoadGraph`
because the build's cycle-breaking stage reads, through it, the records its own edge batch left,
cycle included. `index.DeriveOverCycles` is the one read that takes the removed records instead of
refusing. `write` uses it for the two build-only ops that run inside that window, `add-build-edges`
and `demote-claim-edge` (`opSpec.overCycles`), and the records at every checkpoint commit hold no
cycle.

**The rigor ceiling is checked at the same load.** `kb.LoadGraph` refuses a node record whose
`rigor` is above `kb.RigorMaximum` as a `check: metadata` fault naming the record and the field. The
`format400` decoder takes such a value because its rigor domain has no top. A value at the maximum
is no special case: an authored `rigor: 1.0` is in domain, and an axiom's `1.00` is `kb.AxiomRigor`'s
by rule, the same number and not a distinct read value.

### The loader's file set and its other refusals

**Pattern:** Schema validation at load, fail-closed.

A KB document that is a symbolic link is refused, not followed: `check: symlink` at the link's path,
raised by the document walk and by the conversion's own read of the KB's files alike, so no link out
of the tree is read or written through. A conversion from `1.0.0` holding bytes it cannot carry
refuses before any state is built: `check: conversion`, `path` the file, `line` and `detail` the
content. A `2.0.x` KB the `2.0.x` load refuses, and a `3.0.0` KB whose record files the `3.0.0` load
(`formats.GraphLoadError`) refuses, are refused with that load's own items, `check: metadata`
(`kbload`'s `conversionRefusal`).

The stamp is read from `kb.FormatFile` and, where that file does not exist, from the entry point's
superseded frontmatter (`kbload`'s `formatStamp`). A KB stamped in neither place is a `check:
kb-format` refusal naming both. Where the file exists its content alone is read, the frontmatter
beside it unconsulted.

### The zero-value hazard

**Pattern:** Go zero-value pitfall; required-field decoding.

A record file decoded outside `kb.LoadGraph` (by a test, by a tool, or by a command that read bytes
of its own into a struct) reads a missing `bearing` as the zero `kb.Bearing`. That is the value
`0.0` and a judgement: a premise contributing zero to the strength formula. The strict decoder
refuses a missing field for that reason, and the loader monopoly therefore carries a type's meaning
and not only a version's: every decode of a record file goes through `kb.LoadGraph`, over `Source`
bytes.

### Dependency rules

**Pattern:** Architecture fitness functions.

`cmd/importpolicy_test.go` checks the rules below over the transitive import graph of the module's
production files, with `cmd/readpolicy_test.go`, `cmd/revealpolicy_test.go` and the exec policy
beside it. A rule that names a `cmd` file is checked at file level (`TestCmdFileImporters`). A
package outside a rule's list that reaches the named package fails the test.

- `docgraph` knows nothing of claims. The record schema it writes is the one `claimgraph` reads,
  defined once in `internal/records`, which imports nothing of kbase but `atomicfile`, to write its
  files.
- `claimgraph` never imports `latex/*`, and reaches neither `model` nor `asks/call`, transitively.
- `buildrecords` imports no kbase package but `kb`; `claimgraph` reads the build records through it.
  `sheet` reads no build record and imports no kbase package but `kb`.
- `argmark` and `secret` import no kbase package. `argmark` is therefore reached by every producer of
  text an MCP reader can receive.
- `log` imports `argmark` and nothing else of kbase: every line it writes formats a marked argument
  as the command line takes it, so no log line contains a marker. A package that imports only `log`
  reaches `argmark` through it.
- `kb` imports no other kbase package but `log`, `formats/format400` (the live format it re-exports)
  and `argmark`. `docgraph`, `index`, `write`, `query`, `sheet`, `kbdocs` and `claimgraph` read the
  kb-root vocabulary, the link relation (in its two readings, one resolver), the label-name grammar
  and id minting from it. `export` reads the link relation a third way, `kb.ChildList`: an index
  page's child-list lines only, so a body link is never a navigation-tree edge.
- `write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`, `claimgraph` or `build`.
- The `formats` tree imports nothing of kbase outside itself (I8), so maintenance of a package
  outside it can never change what an old KB converts to. It takes and returns files as bytes, and
  owns the parsers of every superseded format and the live format's definition. Outside the tree,
  only `kbload` imports the root and only `kb` imports `format400`; `kbload` reaches `format400`
  only through `kb` and the root. The write side drains the conversion's delete set; the tree
  touches no file. Within the tree imports run one way: `common` imports nothing of the tree; a
  superseded format package imports `common` and earlier format packages; a converter imports
  `common` and format packages up to its target; the root imports the converters, `common` and the
  format packages. `format400` imports no kbase package, `common` included, and restates any
  declaration it shares with an earlier format. No package but the root imports a converter, and none
  imports a later format (`formatsTreeViolations`).
- A file is opened by path only in the files `fileOpenOwners` in `cmd/readpolicy_test.go` lists, each
  for the purpose it names. A KB's files (the documents, the index and the build records beside
  them) are read through `kb.Source` alone, which `kb/source.go` implements and `kbload` constructs.
  Only `kbload`, `docgraph/validate.go` and `claimgraph/tree.go` construct a `Source` with
  `kb.OnDisk`, and only `write/store.go` and `index/refresh.go` call `Source.DiskFile`.
- `kbload` is imported by `cmd`, `build` and `write` only; `index` does not import it, since a
  refusal is a `result.Refusal`, one shape. `kbload` imports `buildrecords`, `kb`, `result` and the
  `formats` root; `buildrecords` stays free of `kbload`.
- `mcp` is imported by `cmd` alone and reaches no `model`, no `asks/call` and no `build` (I9);
  `mcp/protocol` imports `log` only (and `argmark` through it).
- `internal/model` imports no kbase package but `secret`, the key's type, and no `os`: it touches no
  file and knows no role or consumer (`TestModelImportsNoOS`).
- Only `asks/call`, `build`, `modelclient` and three `cmd` files import `model`: `cmd/build.go`
  (the build's role closure), `cmd/score.go` (the score verb's model call) and `cmd/probe.go` (the probe's
  one call). `internal/model/modeltest` imports it as test support, which nothing shipped imports.
  `modelclient` is the only constructor of a `model.Client`, and a verb opens one only where it is
  about to make a model call. `modelclient` imports no kbase package but `model`, `config`, `result`
  and `log`, and `cmd` alone imports it. Only `build` and `asks/call` import `asks/call`.
- `internal/config` imports no kbase package but `atomicfile`, `result`, `argmark` and `secret`. It
  holds the grammar, the three roles, `Resolve`, `Select`, the loaded files and the templates, and
  refuses in `result.Refusal`. It holds the configure commands' rules too
  (SPEC §9 Provider commands): `ConfigureModels` (the references given, the match of each provider
  part against the pool of the written layer's stack, each reference against its provider's
  catalogue, and the targeted write); `ProviderEntryRefusals` (an entry's arguments); and the
  targeted editors in `write.go` (`UpdateConfig`, `SetProvider`, `RemoveProvider`), with one line
  editor and one decode-compare check for both files
  (SPEC §9.5 The configure commands' targeted update (byte-preservation contract)). It reads a
  catalogue through a function value its caller supplies (`cmd/configure-models.go` binds it over
  the registry's `Catalogue`, under the `--timeout` deadline), and returns the catalogue misses and
  what the run wrote as data, from which the verb writes its stderr lines. It never imports `model`
  or `log`, opens no client and writes no output, and returns its faults as data, so each caller
  decides whether one is a warning or a result field. A refusal's remedy may name the command that
  fixes it (`kbase configure-providers`, `kbase configure-models`).
- `internal/score` imports `model` nowhere: the endpoint is below the verb's model-call interface,
  so the runner, the input guard, the parse and the ledger learn nothing of what answered. It imports
  `write` for a writing-the-results batch's ops and its lock, and never `build`, `kbload`,
  `statedir` or `kbdocs`: the verb takes the run's locks, opens the KB, resolves the state directory
  and runs the start rebuild of the derived views. A batch reopens the KB, to compare each value's
  judged hash with its item's text as the text is now, only through the function the verb supplies
  (`score.Invocation.Open`, `kbload.Open` in `cmd/score.go`). Only `cmd` and `internal/mcp` import
  `score`.
- The judged hash is defined in `internal/asks`, which owns the templates its hash covers: `score`
  imports `write`, so the function cannot be defined in `score`, and `write` imports `asks` for it
  alone. Neither reaches `model` through it.
- Only `build`, `buildstore` and, within `cmd`, `cmd/update-format.go` (its clean check, and the
  `git mv` of each root build record into `kb-root/.build/`, `Repo.Move`) and `cmd/write_op.go` (a
  committing batch's commits, made where every write interface runs its batch, and
  `update-format`'s, through the same `kbCommits`) import `gitledger`; `gitledger` imports `log` and
  nothing else of kbase. Every opener of a KB's repository goes through `buildstore.OpenRepo`, which
  refuses `worktree` where git's worktree root is not the directory `kb.RepositoryRoot` finds;
  `cmd/init.go`'s create commit opens through it and imports no `gitledger`. `write` runs no git: a
  committing batch's snapshot commits are the caller's `write.Options.Preflight`.
- `buildstore` reaches no `model` and no `asks/call`: the build's state, its stage vocabulary and
  its checkpoint commits are read without the runner, and `build` imports them.
- Only `build`, `cmd` and `mcp` import `kbdocs`: `build` and `cmd` stamp readiness documents (within
  `cmd`, file-level, `init` at a KB's creation and `update-format` rewriting the maintained blocks
  with `kbdocs.Maintain`). `mcp` imports it for the `.index/` rule and the session rule alone
  (`kbdocs.IndexRule`, `kbdocs.SessionRule`), which the instructions carry; it stamps nothing.
- Only `build`, `buildstore` and `cmd` import `statedir`.
- `build` imports `write` (the write-lock wait).
- Only `internal/latex/pandoc` execs pandoc; only `internal/gitledger` execs git; only
  `internal/sheet` execs `dot`; and only `internal/mcp/launch.go` (file-level) execs kbase itself.
  Those four are the exec policy's whole list (`cmd/execpolicy_test.go`): a process started anywhere
  else in non-test code, or a listed owner starting a binary other than its own, fails the test;
  test files are out of its scope.
- `internal/export` imports `kb` and `atomicfile`, and nothing else of kbase; the verb opens the KB
  and supplies every pandoc call (the preflight and the one `pandoc.RunDefaults` call) as a function
  value `export.Run` calls.
- `internal/auth` imports nothing of kbase but `atomicfile`, to write its files at their mode, and
  `secret`; only `cmd` imports it, and `config`'s own rule keeps it from the store.
- A key's text leaves a `secret.Key` through `Reveal` in four files alone:
  `internal/model/openai_client.go` and `anthropic_client.go` (setting the auth header and scrubbing
  an echoed key), `internal/model/transport.go` (cutting a status error's body around it) and
  `internal/auth/auth.go` (sealing it). `cmd/revealpolicy_test.go`, on the module walk, fails a
  production file outside them that names `Reveal`.

---

## 12. Constants

**Pattern:** Named constants registry.

Every numeric value of the implementation is defined here and nowhere else; other sections name the
constant and link to this one. Exit codes and the placeholder's hash form are defined in SPEC §7 and
§3, the pandoc range in SPEC §2 and the checkpoint commit subject in SPEC §6.

| Constant | Value | Defined in |
|---|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6 Git holds the recoverable record) | `statedir.KeyHexDigits` |
| State-store format | 1, the integer in `<state-dir>/format` (SPEC §6 The state store is versioned; §8) | `StoreFormat`, `internal/buildstore` |
| Run lock path | `<git dir>/kbase-operation.lock` (SPEC §6 Git holds the recoverable record; [§8](#locks)) | `buildstore.OperationLockFile` |
| KB write-lock wait | 30 s: how long a writer, and the build's wait-out, try to take the KB write lock before `retry` (SPEC §8 One write lock) | `write.LockWait` |
| Monitor lock wait | 2 s: the wait for the run lock and the holder lock, so a monitor probing one does not turn a starting run away | `buildstore.LockWait` |
| Lock poll interval | 5 ms between attempts while waiting for a holder | `poll`, `internal/filelock` |
| Cancel wait | 20 s: how long `cancel` waits, after signalling the holder, for it to let the lock go before it reports the holder still running | `cancelWait`, `internal/build/monitor.go` |
| Builds kept | 5: the build trails, beside the current one, whose directories (answers, group records, call captures, the development replays) survive pruning; the captured builds under `reports/`, beside the newest, are kept to the same count (§8) | `keptBuilds`, `internal/build` |
| Progress rotation | 500 lines a fresh build's `progress.jsonl` holds before it is rotated, about five builds; 15 rotated files kept (§8) | `progressRotateLines`, `keptProgressFiles`, `internal/build` |
| Runs kept | 16: the scoring runs of each kind (`run-`, `dry-`), the replays of a run's `replays/`, the development replays of a build trail's `replays/`, and the captured score runs under `reports/` that survive pruning ([§6](#run-retention); §8) | `KeptRuns`, `internal/buildstore` |
| Replay progress interval | 10 s: the most often a development replay's walk logs its `replay progress` line ([§6](#the-development-replay)) | `ReplayProgressInterval`, `internal/buildstore` (`kbasedev`) |
| Item-text wrap width | 100 columns: the width each section of the item text is wrapped to before its lines are numbered ([§6](#the-closed-text-and-its-guard)) | `wrapWidth`, `internal/score/text.go` |
| Sheet render concurrency | 4: the most `dot` processes at once; output order is kept ([§6](#sheet)) | `sheet.renderConcurrency` |
| Unmarked shortlist, proximity slots | 3: the nearest targets in its own paper that `references-found` makes model calls about per source (§4) | `shortlistProximityK`, `internal/claimgraph` |
| Unmarked shortlist, wildcard slots | 2: the best-ranked targets by cosine asked about per source besides the proximity picks (§4) | `shortlistWildcardK`, `internal/claimgraph` |
| `stats` leverage bar | 0.65, the strength bar of `highest_leverage`. It is its own constant, not derived from the band ladder, so a change to a band's lower bound does not move `stats` | `query.DefaultMaxStrength` |
| `stats` dependents floor | 1, the dependents floor of `highest_leverage` | `query.DefaultMinDependents` |
| Ask concurrency | Default 4: the model calls one invocation has in flight, and the idle connections per host its clients keep; `probe` runs a queue of one. Set by `[asks] concurrency` in `config.toml` (SPEC §9.2 `config.toml` — the choices) or `--ask-concurrency` on `build` and `score` | `config.DefaultAskConcurrency` |
| Probe ask concurrency | 1: the `probe` queue's one token, for its one call | `probeAskConcurrency`, `cmd/probe.go` |
| Transport attempts | 3 per model call, chat and decision alike (§7 classifies what is retried) | `model.ChatAttempts` |
| Transport pauses | 5 s, then 30 s, between attempts: the build's overview call and each of `kbase score`'s model calls. The build's multichoice calls repeat at once. A pause the call's deadline cannot cover is not waited out: the call ends at once, naming the attempts it made | `overviewBackoff`, `internal/asks/call`; `scoreBackoff`, `cmd/score.go` |
| Response-header timeout | 2 min: the handshake alone, from the request written to the response headers read; a stream's body is bounded by the stream idle timeout | `responseHeaderTimeout`, `internal/model/transport.go` |
| Stream idle timeout | 2 min: the longest gap between reads on an open stream before it surfaces as idle | `streamIdleTimeout`, `internal/model/sse.go` |
| Error-body limit | 8 KiB (8192 bytes) of a non-2xx response body echoed into the error | `errorBodyLimit`, `internal/model/transport.go` |
| Messages completion allowance | `max_tokens` is the thinking budget plus `max(256, ceil(characters / 4))` over the request's messages and system text; 256 is the floor | `maxTokensFloor`, `maxTokensFor`, `internal/model/anthropic_client.go` |
| Thinking budgets (Messages) | `low` 1,024, `medium` 4,096, `high` 16,384 tokens (SPEC §9.1 The chat protocol) | `thinkingBudgetLow`, `thinkingBudgetMedium`, `thinkingBudgetHigh` |
| Sampling | `[sampling.text]` and `[sampling.scoring]` in `config.toml`, each `temperature` and `thinking`; unset is temperature 0 and thinking off; `thinking` takes the values of `config.ThinkingLevels`; the temperature ranges are `openai` [0, 2] and `anthropic` [0, 1] | `config.ThinkingLevels` |
| Model roles | `[models] decision` (the multichoice asks, on whichever decision API its entry speaks), `[models] text` (the overview call) and `[models] scoring` (`kbase score`'s calls), each a `<provider>/<model>` reference (SPEC §9.2 `config.toml` — the choices) | `config.toml` |
| Cached-token totals | `AskTotals.WarmCacheReadTokens` (and `CachedTokens`) report 0 on every decision call: the `systemone` decision API's `usage` has no cached-token field, so the zero is the API's | `AskTotals` |
| `auth add` input bound | 65536 bytes, the most stdin `kbase auth add` accepts; it reads one byte past the bound to tell more from exactly that (SPEC §9 Provider commands) | `authAddInputLimit`, `cmd/auth.go` |
| Credentials store directory | `auth`, under the state base (`statedir.Base`), beside the per-KB state directories, whose 16-hex-digit keys it cannot collide with; mode `0700`, set on every add | `auth.DirName`, `auth.DirMode` |
| Credentials store file | `<provider>-key.enc`, the name lowercased; mode `0600` on creation and on every replace ([§7](#credentials)) | `auth.FileSuffix` (`-key.enc`), `auth.FileMode` |
| Detached-run start wait | 5 s: how long the `build` and `score` tools wait for the child to start, or to exit, before returning the run still starting (SPEC §11 Tools) | `launchStarting`, `internal/mcp/launch.go` |
| Detached-run poll interval | 50 ms between reads of the state store while a launch waits | `launchPoll`, `internal/mcp/launch.go` |
| `wait` poll interval | 1 s: how often `kbase wait` reads the state store while the run it names progresses (SPEC §6 `kbase wait`) | `waitPoll`, `cmd/wait.go` |
| MCP protocol revision | `2025-11-25`, the one revision the server implements (SPEC §11 Tools) | `protocol.ProtocolVersion` |
| Metadata format version | `4.0.0`, the one version read and written (SPEC §10 One version read and written); the stamp key is `kb-format` | `kb.FormatVersion` (`format400.Version400`) |
| Format stamp file | `.index/format.yaml`, the one-key block mapping that holds the stamp. `kb.EncodeFormatStamp` writes it and no package but `kbload` reads it, which falls back to the superseded entry-point frontmatter where the file does not exist (SPEC §10 The stamp) | `kb.FormatFile` (`format400.FormatFile`) |
| Pending literal | `*pending*`, the format's one no-number literal, written for a value not yet judged and accepted wherever a value is absent | `kb.PendingLiteral` |
| Conversion record | `migration`: a result's conditional record after `kb-root`, holding `from`, `to`, `note` (`kb.MigrationNote`, one sentence for every migration) and `warnings`; the same call writes the same lines to stderr (SPEC §10 The migration announces itself) | `Source.Migration()` |

### Format migration (SPEC §10)

**Pattern:** Chained schema migrations (upcasters) with golden tests.

A conversion step is bytes in, bytes out, plus the paths it made obsolete: a converter is a pure
function over `formats.Files` (`internal/formats/from<from>to<to>`) that parses its own superseded
form, reads each side's definition from its format package (`internal/formats/format<version>`) and
emits the next form. The chain's order is the version order and a gap in it is a test failure. The
pending obsolete set lives in the `Source`.

**Pattern:** Read-time upcasting plus an explicit migrate command.

Every verb that reads a KB (`update-format`, `verify`, the queries, the write ops, `export-latex`,
and the build's `spine-seed`, `phase-3a`, readiness stamp, overview and resume) calls
`kbload.Open(kbRoot)`; `status` and `cancel` do not load the KB. `Open` reads the stamp alone, from
`.index/format.yaml` (`kb.DecodeFormatStamp`), else from the superseded entry-point frontmatter
(`kb.FormatStamp`), before anything else.

- A stamp at the current major and minor builds a `kb.Source` that reads disk.
- An older stamp reads every file the format covers (`kb-root/` and the build records beside it) into
  a map of repository-relative paths to bytes and hands it to `formats.Chain(from, to)`. `Chain`
  applies each converter in turn and returns the current form's files, the union of the obsolete
  paths and every step's warnings. The `Source` serves the returned bytes and reads an obsolete path
  as absent. One converter covers every patch of its minor.
- A newer or unreadable stamp is refused `check: kb-format`; no entry point is refused `check:
  kb-root`.

The `formats` tree is the only home of a superseded parser and of the live format's definition
(`format400`, which `kb` re-exports). It imports nothing of kbase outside itself, and `kbload` is the
only importer of its root from outside it (import-policy test, [§11](#dependency-rules)). The five
warning list names (`common.ListIDs`, `ListBareIDs`, `ListUnresolvedCitations`, `ListNoEdge`,
`ListStrengthenBy`) are defined in `common` alone; the `formats` root and `kbload` re-export them,
and `update-format`'s result lists (`formatResultLists` in `cmd/update-format.go`) match on
`kbload`'s: warnings listed `ids`, `bare-ids`, `no-edge` or `strengthen-by` appear under keys of
those names and the rest under `migration`. The save reports the removals as `removed`
(SPEC §8 `update-format`).

**Pattern:** Crash-resumable ordered save.

`index.UpdateFormat` is the save, and only `cmd/update-format.go` calls it. Every other writer
refuses a migrated `Source` first (`index.PendingConversion`; [§11](#loading)). After the load
(`Precheck`, then `Derive`) the phases run in this order, each file atomically:

1. The caller's moves of the build records at the repository root (below).
2. `saveConverted`: `.index/edges.yaml`, then `.index/nodes.yaml`, then every other migrated file but
   the entry point.
3. The summary pages and the sheets.
4. The stamp file, written at the version the tree is still at, so a stamp always exists and always
   names the tree's version.
5. The obsolete paths, less what this save covered.
6. The entry point, its frontmatter (which stamped the superseded form) stripped by the conversion.
7. The stamp file again, at the current version, so the stamp marks a completed migration.

The verb then rebuilds the maintained blocks (`kbdocs.Maintain`, [§6](#scratch-creation)). A save
that stops part-way leaves the KB stamped at the version it converts from, its next load resuming the
conversion. `update-format` refuses that tree as not clean, and the recovery is to restore `kb-root/`
and any moved build record from `HEAD` (SPEC §8 `update-format`). A converter that finds
`.index/nodes.yaml` in place takes both record files as they are (`from200to300`'s `resumed`), since
only an interrupted save writes it, and passes the stamp file through unchanged. `from300to400`
likewise takes as they are the record files that already decode as `4.0.0`, which only an
interrupted save writes over a `3.0.0` stamp.

Build records move with `git mv` (`gitledger.Repo.Move`), the only mover. The verb checks the moved
records clean and hands `index.UpdateFormat` the moves (`index.RecordMove`) and `Move`, which it
calls as the phase after the load and before the save's first write. A refused conversion therefore
moves nothing, and the save finds each record in place and its root path gone. The save itself
writes and removes nothing outside `kb-root/` (`index.WriteKBFile` refuses such a target). Each
moved record is reported in `removed` at its root path, repository-relative, and in `written` at its
new path, kb-root-relative.

#### The `from300to400` converter

- **Records.** It decodes the `3.0.0` records through `format300` and writes them through
  `format400`, every `judged-hash` the zero `format400.JudgedHash`, the unjudged value. It reports
  each dropped `no-edge` and `strengthen-by` field as a `formats.Warning` carrying its list
  (`common.ListNoEdge`, `common.ListStrengthenBy`), the node's id and the text, fields
  `kb.MigrationWarning` carries identically.
- **Ids.** It mints each work's and axiom's id from its `3.0.0` id alone (`mintIDs`: the SHA-256 of
  the old id, then of the old id, a NUL and a counter, recomputed in ascending `3.0.0`-id order while
  the result equals an earlier minted id or any `3.0.0` id), its key or label the record's `label`. It renames
  every `3.0.0` work or axiom id in an edge's end and a prose field, each a warning listed `ids`
  (`common.ListIDs`), the `3.0.0` id under `ID` and the minted one under `Text`. The mapping is read
  off a `4.0.0` node file where an interrupted save left one, and an edge file decoding as `4.0.0`
  still has its `3.0.0` ends renamed.
- **Markers.** `convertMarkers` reads each kb-root document's `3.0.0` markers through
  `format300.StatementMarkerRE` and writes `format400.StatementMarker`s, the `lifts` edges of
  whichever record file form exists deciding the one move. It returns each `3.0.0` summary page,
  `claim-rigor.md` at kb-root and in a top-level directory, as an obsolete path, and the save renders
  the new summary pages.
- **Build records.** It moves each build record at the repository root into `kb-root/.build/`, bytes
  unchanged, its root path obsolete.
- **Citations.** `convertCitations` reads every link of each kb-root document but a summary page
  through `format400`'s link grammar. It resolves each anchor under `3.0.0` slugging
  (`format200.Slugify` over the heading as written) and writes it under `4.0.0`'s (`format400.Slugger`
  over the heading as converted, its bare ids already citations, which render as their link text); a
  document's nth heading is the same heading in both forms. An anchor naming one heading under
  `3.0.0` slugging and another under `4.0.0`'s is left as written.
  - A node's place is read from the converted markers, a `3.0.0` register entry from the records (its
    node's title slugged on the page its host's volume rendered), and a work's anchor on the root
    summary page from the page's heading texts (`format400.SummaryTitle` and the entry headings),
    taken only where no other heading there can take it.
  - No link stays aimed at a removed summary page. A link whose anchor names one entry with a place
    points at that place; one whose entries are all stated in one section points at that section,
    untitled; any other points at the summary page replacing the page (`format400.SummaryPage` over
    the page's own path), without its anchor. Where no node renders on that page, so the save writes
    none, the link goes and its text remains, a bare id in that text converted as any is.
  - A bare id is read in the `3.0.0` id grammar and cited under its minted id. Each `### Axiom
    <label>:` heading of `axioms.md` naming an axiom record ends in its axiom's marker; one naming
    none is a warning.
  - Each conversion of a bare id is a warning listed `bare-ids` (`common.ListBareIDs`). Each link or
    id it cannot resolve to one node, and each link it removes, is one listed `unresolved-citations`
    (`common.ListUnresolvedCitations`), each carrying the id where it has one. Only this converter
    makes them.

#### Golden tests

Each converter has a golden pair as its test. The record files of each `3.0.0` and `4.0.0` golden
tree, `internal/formats/testdata/<tree>-<version>/`, have a decoded-values golden beside them,
`<tree>-<version>.values.jsonl`: one JSON object per record in the files' order, its keys in its
type's field order, a pending value as the literal. That version's decoder, `format300`'s or
`format400`'s, reads the record files to those values (`TestRecordFilesDecodeToTheirValues`).

### MCP server (SPEC §11)

**Pattern:** MCP server generated from the command tree; JSON-RPC over stdio; detached child plus
status polling.

`internal/mcp` builds the server from the command tree its caller hands it; `internal/mcp/protocol`
holds the wire. Nothing is listed by hand.

#### Tool table

**Pattern:** Generated API surface from a command registry.

`mcp.Tools` builds one tool per command declaring `kbase.mcp` = `bound` (`mcp.Bind`); `mcp.Exclude`
declares `excluded` with a reason. `mcp`, `wait`, `auth` and cobra's own `completion` and `help`
declare `excluded`. `cmd/surface.go` adds every command to the tree, one `AddCommand` per group, its
declaration on that line (`readOnlyTool`, `readWriteTool`, `cliOnly`; for a family tool's operation
`queryOperation` or `writeOperation`, which add `kbase.mcp.family`). It also declares the MCP-only
interfaces: the resource families and the detached launches. It names no project for them:
`mcp.Server.Serve` binds the resources to the server's default project.

- **Read-only.** A command's read-only standing is the cobra annotation `kbase.readOnly`
  (`mcp.ReadOnly`), read when the table is built. A family tool is read-only where every member
  declares it.
- **Families.** Commands declaring one family (`kbase.mcp.family`, set by `mcp.InFamily`) are one
  tool together, in the place of the first. A family is an `mcp.Family` its caller hands it (name,
  description sentence, shared help): `query` (`toolFamilies`, `cmd/surface.go`). A family a `bound`
  command is named for is that command's, its members having no tool of their own: the write ops
  declare the family `write`, and the `write` tool is `kbase write`'s (`mcp.batchTool`), its schema
  the batch (`entry`, a required array of objects; `dry-run` and `commit`, booleans), its arguments
  handed whole to `Calls.WriteOp` as `write.Batch`'s values document. `runWriteOp` runs the batch
  with `ArgumentEntry` set: each refusal keeps its `entry`, and one naming no `path` drops `line` and
  `column`, which would point into a document the caller never wrote, while one naming a document
  keeps its line there. A refusal's `key` is the values key, which no interface formats differently.
- **Command tool schema.** `mcp.CommandSchema` derives it from the `Use` line's positionals and the
  command's flags, skipping a flag annotated `kbase.mcp` = `excluded` (`mcp.ExcludeFlag`), which is no
  argument of the tool: `score`'s `--replay` and `--failed` are such flags, and `internal/mcp/protocol`
  refuses an argument naming one as outside the schema. `required` is the positionals, less an
  optional variadic one that the `Use` line writes `[<name>...]`, plus each flag declared with
  `mcp.RequireFlag`, which is `trace`'s `direction` alone. `wait`'s `--pid`, the one flag a verb
  requires without that declaration, is outside the derivation: `wait` declares `excluded`. No other
  bound command refuses for want of a flag, `score`'s `--pass` among them, a run walking both passes
  unless it is told otherwise.
- **Family tool schema.** `operation` is a string enum of the members' names in the tree's order, and
  `arguments` an object. Each member is built as its own command tool would be, and the family keeps
  it by name, unlisted. A call takes the member `operation` names and checks `arguments` against that
  member's schema with `protocol.Schema.Faults`, the same check `internal/mcp/protocol` runs on a
  tool's arguments, so no argument list is stated twice. A fault is a usage refusal document, one item
  per fault naming the argument, the member's arguments as `allowed`, and a remedy naming `allowed`
  for the argument names and the member's help resource for what it does. Arguments that fit run the
  member as its own tool would. A values op's argument list reads `write.OpVocabulary` (the entry's
  keys in vocabulary order, each with the type, required mark and description its definition in
  `internal/write/values.go` carries), naming no flag, so it is true on both interfaces.
- **Results.** Every tool but `write` runs the command's `RunE` with its flags reset and its output
  captured to a buffer. The buffer is the result text; decoded once more, the YAML document is the
  `structuredContent`; the document's outcome gives `isError` (`result.IsError`). A `RunE` starts from
  `invocationDir(cmd)`, never `os.Getwd()`. Each tool's result is formatted for MCP and checked for an
  item keyed `--name` or `<name>` of its own arguments (`warnCLIKeys`, a warning only).
- **The tool set is asserted.** `TestMCPToolsAreTheCommandTree` derives its expectation from the
  annotations alone (a family's name for its members, a command's own otherwise) and asserts the tool
  set for equality, so a tool listed by hand fails. `build-trail`, the build's checkpoint-commit
  history, is `cmd/build-trail.go`, a command tool like `status` (`buildstore.BuildTrail`), so its
  schema, key order and refusal keys are derived as any command's are;
  `TestBuildTrailIsOneDocumentOnBothSurfaces` runs it at the command line and through the server and
  requires the two documents to agree. Every tool has a command behind it (I9).
- **Protocol revision.** `protocol.ProtocolVersion` is the one constant; an `initialize` naming
  another revision is answered with the server's, for the client to decide.

#### Help texts

**Pattern:** Self-describing API.

Each command stores four help texts: `Short`, `Long`, an MCP note (`kbase.help.mcp`) and a CLI note
(`kbase.help.cli`). `internal/mcp/help.go` composes three renderings from them, each formatting its
markers for its interface through `mcp.Spelling` ([§9](#per-interface-rendering)):

- the tool description (`Short`, the MCP note, where the full help is), read on `tools/list`;
- the help resource `kbase://help/<command>` (`Long`, the argument list, the MCP note);
- `CLIHelp`, the `--help` text the root's help function prints (the usage line, `Long`, the argument
  list, the CLI note `CLINote`, which is closed by the flags excluded from MCP and, below the root, a
  line naming the global flags).

`ArgumentList` builds the argument list from the definitions a tool's schema reads: a command's
positionals and flags (`commandArguments`), a values op's `write.OpVocabulary`, the batch's
`batchSchema`, and the root's persistent and own flags, each interface typing a list and a mapping
its own way. `TestHelpRenderingsAreTheirParts` (`cmd/helpresources_test.go`) asserts, for every
`bound` command and with and without notes, that each rendering is exactly its parts. The
`measure-mcp-surface` target (`tools/scripts/measure-mcp-surface.sh`, with `TestMeasureMCPSurface`)
measures the always-read size (the `tools/list` result, the input schemas and the instructions) over
the release build. It is a measurement and gates nothing.

The root command's `Long` states the command line's form once each: `queryHelp`, `listQueryHelp` and
`valuesHelp` in `cmd`, the last closing on `batchHelp`, the write batch's contract. The `query`
family's `Help` is the MCP form for `kbase://help/query`, from the same text where the two agree
(`queryReads`, `queryAnswerHelp` and `listQueryHelp`, one marked text that each rendering formats for
its interface); the resource lists the members from the annotations the enum is built from.
`batchHelp` is also `kbase write`'s `Long`, so `kbase://help/write` has the contract in full, and
every write op shares one MCP note and one CLI note (`writeOpCommand`). The members' shared help is
appended to no member's `Long`.

`mcp.ServedInstructions` is what `initialize` reports (`instructions.go`): its opening, which
differs by mode (the default project's state where there is one, the `project-dir` lines where
there is none) and the sentence naming the skill; then `kbdocs.IndexRule` and `kbdocs.SessionRule`,
which the stamped `AGENTS.md` contains in its `index-rule` and `session-rule` slots; then
`mcp.EdgeDirection`, which the root's `Long` contains too; then the paragraphs only the instructions
have.

#### A call's project

`mcp.Tools` wraps every tool in `inProject`. The wrapper adds the `project-dir` property to the
schema, and on each call `callProject` resolves the project: the `project-dir` the call names, which
`buildstore.ProjectRoot` requires to be a git worktree root (the same check `--project-dir` gets on
the command line), else the server's default project. It removes `project-dir` from the arguments
and hands the inner tool the project. No tool closes over a directory: a command tool sets it as the
call's invocation directory, a write op passes it to `Calls.WriteOp`, and a launch starts its child
there.

The default project is `mcp.Located`, whose zero value is the explicit none: `cmd/mcp.go` sets it
from `kb.RepositoryRoot` of the invocation directory only where that finds a repository, so no
reader ever passes `""` to a path function that would read it as the working directory. The server
runs in one of two modes, chosen in `cmd/mcp.go` before serving (SPEC §11 Two modes). With a
default project, `projectDirDescription` describes `project-dir`, that text followed by the
sentences on the server's own project. Where `cmd/mcp.go` finds none, or under
`--project-dir-required`, the project is `mcp.ProjectRequired()`, the zero `mcp.Located`: `inProject`
lists `project-dir` in each schema's `required` with `projectDirMeaning` alone as its description.
The `callProject` refusal, a usage refusal keyed `project-dir`, stays behind that schema for a call
whose `project-dir` is empty. The global `--project-dir` given to `mcp` is a usage refusal keyed
`--project-dir` before serving; startup never fails for want of a project.

#### Wire and resources

`internal/mcp/protocol` holds newline-delimited JSON-RPC over stdio, `initialize`, `ping`,
`tools/list` and `tools/call`, the `Tool`, `Schema` and `Property` types, and the frame checks
(`jsonrpc`, `id`, `method`, known tool, object arguments). Arguments failing the schema (missing,
wrong type, a string outside its `enum`, unknown key) are JSON-RPC `-32602` errors, not refusal
documents; `Schema.Faults` is that check as data, which a family tool runs on its `arguments`.
`Serve(in, out, version, instructions, tools, resources, lg)` takes the server version, the
instructions, the tool table and the resources from its caller, and imports `log` only.

`resources/list` and `resources/read` serve the resources `internal/mcp` supplies: the sheets, under
`kbase://sheet/<kb-root-relative path>`; the model guidance at `kbase://model-hints`, which is
`internal/config`'s rows rendered as markdown and needs no KB; and the help of each family
(`kbase://help/<family>`) and of each command exposed as a tool (`kbase://help/<command>`,
`mcp.HelpResources`), composed in `internal/mcp/help.go`, which needs no KB. An unknown URI is
`-32002` and a missing `uri` `-32602`. `initialize` advertises `resources` with no `subscribe`, and
the protocol answers no subscribe method (`TestNoSubscription`).

#### Detached build and score

**Pattern:** Detached child process plus status polling.

`internal/mcp/launch.go` is the single self-exec owner under I1, file-level, enforced by the exec
policy. The `build` and `score` tools re-execute the kbase binary as `kbase build …` and `kbase
score …` by the same detachment:

- The child runs from the root of the call's project, with the server's environment plus
  `KBASE_RESULT_SURFACE` (`mcp.SurfaceEnv`), which tells the run its document is read over MCP. It
  has its own process group (detach flags per platform in `internal/mcp/detach_unix.go` and
  `detach_windows.go`) and no stdin.
- The server's `--config-dir`, `--log-level` and `--log-file` are passed through, and `--project-dir`
  where the call named one, so the child's recorded `resume` carries it.
- Every path the child is given is absolute, because its working directory is not the server's. A
  relative `state-dir` argument is resolved against the repository root; the server's `--config-dir`
  and `--log-file` are resolved against the server's working directory.
- Its stdout and stderr are files in the state store's `reports/`, `build-<UTC timestamp>-<random>`
  or `score-<UTC timestamp>-<random>` with `.stdout.yaml` and `.stderr.log`. Pruning keeps the
  newest `keptBuilds`+1 build captures (`build`'s prune) and the newest `KeptRuns` score captures
  (`score.PruneRuns`) (§12).
- Before starting, the tool runs `status`'s preconditions: no worktree, or a `--state-dir` inside
  `kb-root/`, returns the refusal `status` gives and starts no process.
- The `edge` flag of `score` is declared a mapping (`mcp.MappingFlag`), so its schema property is an
  object of the required strings `source`, `target` and `relation`, closed to other keys, and
  `CommandLine` joins the mapping into the child's `--edge <source>,<target>,<relation>`, as
  `mcp.Spelling` does in a call it renders. A value holding a comma is refused at `edge`
  (`commaInMapping`) and starts nothing.

The tool polls the state store every `launchPoll` and returns when the run has started or the child
has exited, bounded by `launchStarting` (§12). The child takes the run lock and the
holder lock and emits its first progress event before it waits out a write ([§8](#locks)). What
"started" means differs by tool:

| Tool | Started when | `done` document |
|---|---|---|
| `build` | the progress record shows a stage entered (`buildstore.Launched`) | `kb-root`, `state-dir`, `pid`, `resume` and `wait`. `resume` appears only where the record's command parses as a `kbase build` invocation |
| `score` | the run's ledger and pid exist (`score.RunByPID`: the run's directory written, its `pid` file reading back) | `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait` |

`wait` is the run's background command: `buildstore.ShellJoin` of the executable with `wait
[--project-dir <root>] --state-dir <store> --pid <pid>`, `--project-dir` where the call named one.
Otherwise the result is the child's own document where it exited first (`isError` from its outcome),
or `done` with `state: starting`, `state-dir`, `pid` and `wait` where neither happened within the
bound; `status` follows the run.

A score run's result document is written beside its ledger before the run lets go of the state
store's holder lock, so `wait` reads the same document the run reported rather than racing the
lock's release. `wait` locates the run by the pid the launch named (`score.RunByPID` reads the run's
`pid` file) and reports the run's outcome; a run whose process ended without writing a result
reports the failure its ledger supports. `status` and `cancel` read the holder record (the pid, the
run, the start stamp and the kind `score`), so a run in flight is visible under the pid the tool
returned.

Nothing announces a run's end: no resource is subscribable and the server writes no frame a request
does not owe, so a session follows a launched run by calling `status`, or by running the `wait`
command the launch returned. `test-integration-mcp-build` (`TestMCPBuildFollowedByStatus`) drives a
real server to a build's end by polling `status`.

---

## 13. Slice verdicts

**Pattern:** Spike/prototype decision record.

The reader's design was settled by a slice: `kbase build --through document-graph` over six corpus
papers and a count report of all fifty, judged by comparing kbase's tree against the reference tree
for the same paper, with kbase's own readers. The hypotheses and their verdicts:

| Hypothesis | Confirmed by | Refuted by | Verdict |
|---|---|---|---|
| No filter layer is needed | The conformance and inventory checks pass on kbase's tree; per-leaf word streams match; every citation state the reference trees exhibit appears in kbase's | A load-bearing form or word no JSON transform can produce | **Confirmed** on the six Slice-1 and Slice-2 papers; 49 of 50 corpus papers build |
| Records suffice for the inventory | Every reader and placement fact of the inventory (§5.5) is carried by kbase's records, item by item in scan order; every record joins exactly one document | A reader or placement fact that exists only in rendered bytes | **Confirmed**, with the visibility rule of §5.5: a block record exists only where the page has a label line the reader reads |
| The preceding text run suffices | Withdrawn: the word before a reference is defined over the written page, so it is a page-defined reading (§5.5), not a record | — | Withdrawn |
| pandoc's writer suffices | Every Slice-1 check passes under it | A load-bearing form it cannot emit | **Confirmed**; the writer route is pandoc's gfm writer with `--wrap=none` (§5.4) |

### Slice-1 checks

Identical tree path set and child order; equal per-leaf word streams under the markdown tokeniser;
the Document-Tree conformance passes with matching counts of up-link lines, three-attribute
anchors, math fences, `data-cites` citation spans and labelled blockquotes with identifiers; equal
inventory; records that carry every reader and placement fact of that inventory; byte-identical
`kb-root/` and records across two runs. All pass: the writer route is pandoc's. Any fails: execution
stops and the owner rules (fix, amend the check, or a Go writer).

**Slice-2** applies the same checks to four more papers, and requires two negative controls to
refuse as SPEC §5 Inference preflight specifies (naming the paper and pandoc's error; naming each unloaded file
and the line that named it). **Slice-3** records a count report, with no pass threshold.

A check that fails for a reason whose fix needs a third pre-pass, a filter, or a change to any of I1
to I7 is escalated to the owner.

---

## 14. Limits

**Pattern:** Known-limitations section.

Where the format and this implementation stop. A limit whose mechanism already states it is not
repeated here: the zero-value default is defined in [§11](#the-zero-value-hazard), and the in-memory
index's lack of statement text in [§6](#the-derived-layer).

**A value records whose call wrote it and what it was judged against, and nothing more.**
`manually-scored` says whether a person's call or a machine's wrote a rigor or a bearing, and
`judged-hash` what it was last judged against (SPEC §10 Provenance and the judged hash): no author,
time or reason. A work's rigor and a `founds` bearing, which no run writes, have neither. A
`*pending*` the caller wrote and one written because it left the value out differ only in that flag.
A value converted from `2.0.x` is marked model-scored whoever judged it, and nothing records that it
was converted.

**A `founds` bearing on a work names its finding in prose.** The `context` on the record says which
theorem, table or result the bearing judges, and no id, range or hash joins the two, so no check can
prove the judgement was about that finding. An axiom's `founds` edge needs no such prose: its source
is the axiom's own id.

### A conversion records no history

The value changes a conversion makes are recomputed from the converted records, and nothing but its
warnings says which of them rose or fell.

### Merged metadata is refused, not merged

Two branches' record files merge by text: a duplicate edge, a dangling edge or a cycle the merge
leaves is a load error, `check: metadata`, which every command that loads the KB refuses. No op
resolves it: the recovery is in git, returning the record files to a consistent commit, such as the
one before the merge, and reapplying the other side's changes through write ops
(SPEC §10 The load: every fault refused).

### No format sits between two majors

`formats.Chain` walks converters version by version and refuses a step no converter leads, so a KB
stamped `1.1.0` is refused at load rather than read; no `1.1.0` converter exists and none is
reachable.

### No refresh moves a judgement

A rebuild of the derived views, `update-format`'s included, makes no model call, so a derived value
changes only when the records do: a write op, or a scoring run writing its results. `verify`'s
freshness check is between the records and a summary page, never between two judgements.
