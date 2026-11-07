## 12. Constants

**Pattern:** Named constants registry.

Every numeric value of the implementation is defined here and nowhere else; other sections name the
constant and link to this one. Exit codes and the placeholder's hash form are defined in SPEC §7 and
[§3](../3-principles.md), the pandoc range in SPEC §2 and the checkpoint commit subject in SPEC §6.

| Constant | Value | Defined in |
|---|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6 Git holds the recoverable record) | `statedir.KeyHexDigits` |
| State-store format | 1, the integer in `<state-dir>/format` (SPEC §6 The state store is versioned; [§8](../8-ledger-state-store-and-monitoring/index.md)) | `StoreFormat`, `internal/buildstore` |
| Run lock path | `<git dir>/kbase-operation.lock` (SPEC §6 Git holds the recoverable record; [§8](../8-ledger-state-store-and-monitoring/locks.md)) | `buildstore.OperationLockFile` |
| KB write-lock wait | 30 s: how long a writer, and the build's wait-out, try to take the KB write lock before `retry` (SPEC §8 One write lock) | `write.LockWait` |
| Monitor lock wait | 2 s: the wait for the run lock and the holder lock, so a monitor probing one does not turn a starting run away | `buildstore.LockWait` |
| Lock poll interval | 5 ms between attempts while waiting for a holder | `poll`, `internal/filelock` |
| Cancel wait | 20 s: how long `cancel` waits, after signalling the holder, for it to let the lock go before it reports the holder still running | `cancelWait`, `internal/build/monitor.go` |
| Builds kept | 5: the build trails, beside the current one, whose directories (answers, group records, call captures, the development replays) survive pruning; the captured builds under `reports/`, beside the newest, are kept to the same count ([§8](../8-ledger-state-store-and-monitoring/index.md)) | `keptBuilds`, `internal/build` |
| Progress rotation | 500 lines a fresh build's `progress.jsonl` holds before it is rotated, about five builds; 15 rotated files kept ([§8](../8-ledger-state-store-and-monitoring/index.md)) | `progressRotateLines`, `keptProgressFiles`, `internal/build` |
| Runs kept | 16: the scoring runs of each kind (`run-`, `dry-`), the replays of a run's `replays/`, the development replays of a build trail's `replays/`, and the captured score runs under `reports/` that survive pruning ([§6](../6-kb-model-and-maintenance/scoring/run-retention.md); [§8](../8-ledger-state-store-and-monitoring/index.md)) | `KeptRuns`, `internal/buildstore` |
| Replay progress interval | 10 s: the most often a development replay's walk logs its `replay progress` line ([§6](../6-kb-model-and-maintenance/scoring/the-development-replay.md)) | `ReplayProgressInterval`, `internal/buildstore` (`kbasedev`) |
| Item-text wrap width | 100 columns: the width each section of the item text is wrapped to before its lines are numbered ([§6](../6-kb-model-and-maintenance/scoring/the-closed-text-and-its-guard.md)) | `wrapWidth`, `internal/score/text.go` |
| Sheet render concurrency | 4: the most `dot` processes at once; output order is kept ([§6](../6-kb-model-and-maintenance/sheet.md)) | `sheet.renderConcurrency` |
| Unmarked shortlist, proximity slots | 3: the nearest targets in its own paper that `references-found` makes model calls about per source ([§4](../4-stage-table/index.md)) | `shortlistProximityK`, `internal/claimgraph` |
| Unmarked shortlist, wildcard slots | 2: the best-ranked targets by cosine asked about per source besides the proximity picks ([§4](../4-stage-table/index.md)) | `shortlistWildcardK`, `internal/claimgraph` |
| Claim opening, minimum words | 4: the fewest canonical words a sentence has to open a claim in `claims-discovered`; measured, every folded collision at three words or fewer was a structural label ([§4](../4-stage-table/index.md)) | `minOpeningWords`, `internal/claimgraph/prose.go` |
| Prose claim title length | 120 characters: the longest a discovered claim's derived title runs before it is cut at a word boundary ([§4](../4-stage-table/index.md)) | `titleMaxChars`, `internal/claimgraph/prose.go` |
| Claim-graph write retries | 3: the identical re-issues of a claim-graph write op that answers `retry` before the stage stops ([§4](../4-stage-table/index.md)) | `retryLimit`, `internal/claimgraph/write.go` |
| `stats` leverage bar | 0.65, the strength bar of `highest-leverage`. It is its own constant, not derived from the band ladder, so a change to a band's lower bound does not move `stats` | `query.DefaultMaxStrength` |
| `stats` dependents floor | 1, the dependents floor of `highest-leverage` | `query.DefaultMinDependents` |
| `stats` rank limit | 10: the rows each of `highest-leverage` and `weakest` carries | `query.StatsRankLimit` |
| Ask concurrency | Default 4: the model calls one invocation has in flight, and the idle connections per host its clients keep; `probe` runs a queue of one. Set by `[asks] concurrency` in `config.toml` (SPEC §9.2 `config.toml` — the choices) or `--ask-concurrency` on `build` and `score` | `config.DefaultAskConcurrency` |
| Asks per item | 2: the most model calls one item's ask makes — the first ask and one corrective re-ask — read by a score item's attempt ([§6](../6-kb-model-and-maintenance/scoring/the-runner.md)), and by the build's overview loop and a decision item's `asks.resolve` ([§7](../7-inference-seams/the-multichoice-ask-interface.md)) | `asks.MaxAsks`, `internal/asks/compose.go` |
| Probe ask concurrency | 1: the `probe` queue's one permit, for its one call | `probeAskConcurrency`, `cmd/probe.go` |
| Transport attempts | 4 per model call, chat and decision alike ([§7](../7-inference-seams/index.md) classifies what is retried) | `Attempts` of `model.DefaultRetry()`, `internal/model/retry.go` |
| Transport pauses | Before attempt n, a uniform draw from [0, 1 s × 2ⁿ⁻²] — bounds 1 s, 2 s and 4 s — or the delay a 429 or 503 asks for in `Retry-After`. A pause the call's deadline cannot cover is not waited out: the call ends at once, naming the attempts it made | `Base` and `Draw` (`math/rand/v2`'s `Int64N`) of `model.DefaultRetry()`, `internal/model/retry.go` |
| Response-header timeout | 2 min: the handshake alone, from the request written to the response headers read; a stream's body is bounded by the stream idle timeout | `responseHeaderTimeout`, `internal/model/transport.go` |
| Stream idle timeout | 2 min: the longest gap between reads on an open stream before it surfaces as idle | `streamIdleTimeout`, `internal/model/sse.go` |
| Error-body limit | 8 KiB (8192 bytes) of a non-2xx response body echoed into the error | `errorBodyLimit`, `internal/model/transport.go` |
| Messages completion allowance | `max_tokens` is the thinking budget plus `max(256, ceil(characters / 4))` over the request's messages and system text; 256 is the floor | `maxTokensFloor`, `maxTokensFor`, `internal/model/anthropic_client.go` |
| Thinking budgets (Messages) | `low` 1,024, `medium` 4,096, `high` 16,384 tokens (SPEC §9.1 The chat protocol) | `thinkingBudgetLow`, `thinkingBudgetMedium`, `thinkingBudgetHigh` |
| Sampling | `[sampling.text]` and `[sampling.scoring]` in `config.toml`, each `temperature` and `thinking`; unset is temperature 0 and thinking off; `thinking` takes the values of `config.ThinkingLevels`; the temperature ranges are `openai` [0, 2] and `anthropic` [0, 1] | `config.ThinkingLevels` |
| Model roles | `[models] decision` (the multichoice asks, on whichever decision API its entry speaks), `[models] text` (the overview call) and `[models] scoring` (`kbase score`'s calls), each a `<provider>/<model>` reference (SPEC §9.2 `config.toml` — the choices) | `config.toml` |
| Cached-token totals | `AskTotals.WarmCacheReadTokens` (and `CachedTokens`) report 0 on every decision call: neither decision API reports cached tokens, their `usage` being the one shared `wireDecisionUsage`, so the zero is the API's | `AskTotals` |
| `auth add` input bound | 65536 bytes, the most stdin `kbase auth add` accepts; it reads one byte past the bound to tell more from exactly that (SPEC §9 Provider commands) | `authAddInputLimit`, `cmd/auth.go` |
| Credentials store directory | `auth`, under the state base (`statedir.Base`), beside the per-KB state directories, whose 16-hex-digit keys it cannot collide with; mode `0700`, set on every add | `auth.DirName`, `auth.DirMode` |
| Credentials store file | `<provider>-key.enc`, the name lowercased; mode `0600` on creation and on every replace ([§7](../7-inference-seams/credentials.md)) | `auth.FileSuffix` (`-key.enc`), `auth.FileMode` |
| Detached-run start wait | 5 s: how long the `build` and `score` tools wait for the child to start, or to exit, before returning the run still starting (SPEC §11 Tools) | `launchStarting`, `internal/mcp/launch.go` |
| Detached-run poll interval | 50 ms between reads of the state store while a launch waits | `launchPoll`, `internal/mcp/launch.go` |
| `wait` poll interval | 1 s: how often `kbase wait` reads the state store while the run it names progresses (SPEC §6 `kbase wait`) | `waitPoll`, `cmd/wait.go` |
| MCP protocol revision | `2025-11-25`, the one revision the server implements (SPEC §11 Tools) | `protocol.ProtocolVersion` |
| Metadata format version | `4.0.1`, the one version read and written (SPEC §10 One version read and written); the stamp key is `kb-format` | `kb.FormatVersion` (`format40.Version`) |
| Format stamp file | `.index/format.yaml`, the one-key block mapping that holds the stamp. `kb.EncodeFormatStamp` writes it and no package but `kbload` reads it, which falls back to the superseded entry-point frontmatter where the file does not exist (SPEC §10 The stamp) | `kb.FormatFile` (`format40.FormatFile`) |
| Pending literal | `*pending*`, the format's one no-number literal, written for a value not yet judged and accepted wherever a value is absent | `kb.PendingLiteral` |
| Conversion record | `migration`: a result's conditional record after `kb-root`, holding `from`, `to`, `note` (`kb.MigrationNote`, one sentence for every migration) and `warnings`; the same call writes the same lines to stderr (SPEC §10 The migration announces itself) | `Source.Migration()` |

Contents:

- [§12 Format migration (SPEC §10)](format-migration-spec-10/index.md)
- [§12 MCP server (SPEC §11)](mcp-server-spec-11/index.md)
