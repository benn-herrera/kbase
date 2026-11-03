## 12. Constants

| Constant | Value |
|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6) |
| Credentials store | `auth.DirName`: `auth`, the store's directory under the state base (`statedir.Base`), beside the per-KB state directories, whose 16-hex-digit keys it cannot collide with; `auth.FileSuffix`: `-key.enc`, ending each provider's file, `<provider>-key.enc` with the name lowercased; `auth.FileMode`: `0600`, each file's permission on creation and on every replace; `auth.DirMode`: `0700`, the store directory's permission, set on every add |
| `auth add` input bound | `authAddInputLimit` in `cmd/auth.go`: 65536 bytes, the most stdin `kbase auth add` accepts; it reads one byte past it to tell more from exactly (SPEC §9) |
| `stats` leverage bar and floor | `query.DefaultMaxStrength`: 0.65, the strength bar, and `query.DefaultMinDependents`: 1, the dependents floor, of `stats`' `highest_leverage`; the bar is its own constant, not derived from the band ladder, so a change to a band's lower bound does not move `stats` |
| Transport attempts | 3 per call; a provider's non-2xx status ends the call on its first attempt unless it is a 429 or a 5xx, an answer cut off at the completion cap before any answer text ends the call on its first attempt, and every transport failure and incomplete stream is re-issued; the build's multichoice asks re-issue at once, its overview call and `kbase score`'s each ask pause 5 s then 30 s between attempts, and a pause the call's deadline cannot cover is not waited out — the call gives up at once, naming the attempts it made |
| Cached-token totals | `AskTotals.WarmCacheReadTokens` (and `CachedTokens`) report 0 on every decision call: the SystemOne surface's `usage` has no cached-token field, so the zero is the API's, not a regression |
| Ask concurrency | `[asks] concurrency` in `config.toml` (SPEC §9.2), `--ask-concurrency` on `build` and `score`, default `config.DefaultAskConcurrency`, 4; the model calls one invocation has in flight, and the idle connections per host its clients keep; `probe` runs a queue of one |
| Sampling | `[sampling.text]` and `[sampling.scoring]` in `config.toml` (SPEC §9.2), each `temperature` and `thinking`; the `thinking` vocabulary `config.ThinkingLevels`; unset is temperature 0 and thinking off; the temperature ranges `openai` [0, 2] and `anthropic` [0, 1] |
| Thinking budgets (Messages) | `low` 1,024, `medium` 4,096, `high` 16,384 tokens, as named constants in `anthropic_client.go` (`thinkingBudgetLow`, `thinkingBudgetMedium`, `thinkingBudgetHigh`); `max_tokens` is the budget plus the answer's allowance (SPEC §9.1) |
| Model roles | `[models] decision` (the multichoice asks, on whichever surface its entry speaks), `[models] text` (the overview call) and `[models] scoring` (`kbase score`'s asks) in `config.toml`, each a `<provider>/<model>` reference (SPEC §9) |
| Metadata format version | The one version read and written (SPEC §10), `kb.FormatVersion` (`format400.Version400`); the stamp key is `kb-format`. `4.0.0` |
| Format stamp file | `kb.FormatFile` (`format400.FormatFile`): `.index/format.yaml`, the one-key block mapping that holds the stamp, written by `kb.EncodeFormatStamp` and read by no package but `kbload`, which falls back to the superseded entry-point frontmatter where the file does not stand (SPEC §10) |
| Pending literal | `kb.PendingLiteral`: `*pending*`, the format's one no-number literal, written for a value not yet judged and accepted wherever a value is absent (SPEC §10) |
| Conversion record | `migration`: a result's conditional record after `kb-root`, holding `from`, `to`, `note` (`kb.MigrationNote`, one sentence for every migration) and `warnings`, and the stderr lines the same call writes — the sentence, then one per warning (SPEC §7, [§10](../10-compatibility-harness.md)) |
| MCP protocol version | `protocol.ProtocolVersion`: the MCP revision the server implements (SPEC §11): `2025-11-25`, confirmed, not chosen, by the owner's live check |
| KB write-lock wait | `write.LockWait`: 30 s, how long a writer, and the build's wait-out, try to take the KB write lock before `retry` (SPEC §8) |
| State-store format | `StoreFormat` in `internal/buildstore`: 1, the integer in `<state-dir>/format` (SPEC §6, [§8](../8-ledger-state-store-and-monitoring.md)) |
| Builds kept | `keptBuilds` in `internal/build`: 5, the build trails, beside the current one, whose directories — answers, group records, call captures and the trail's development replays — survive pruning, and are pruned with them, and the captured builds, beside the newest, that survive it ([§8](../8-ledger-state-store-and-monitoring.md)) |
| Progress rotation | `progressRotateLines` in `internal/build`: 500, the lines a fresh build's `progress.jsonl` may hold before it is rotated, about five builds; `keptProgressFiles`: 15, the rotated files kept ([§8](../8-ledger-state-store-and-monitoring.md)) |
| Runs kept | `KeptRuns` in `internal/buildstore`: 16, the scoring runs of each kind (`run-`, `dry-`) that survive pruning, the replays of a run's `replays/`, and the development replays of a build trail's `replays/` ([§6](../6-kb-model-and-maintenance/index.md), Scoring; [§8](../8-ledger-state-store-and-monitoring.md)) |
| Replay progress interval | `ReplayProgressInterval` in `internal/buildstore` (`kbasedev`): 10 s, the most often a development replay's walk logs its `replay progress` line ([§6](../6-kb-model-and-maintenance/index.md), Scoring; [§8](../8-ledger-state-store-and-monitoring.md)) |
| Scoring wrap width | `wrapWidth` in `internal/score/text.go`: 100 columns, the width each passage of the closed text is wrapped to before its lines are numbered ([§6](../6-kb-model-and-maintenance/index.md), Scoring) |
| Operation lock path | `<git dir>/kbase-operation.lock` (SPEC §6, ARCHITECTURE §8) |
| Detached-run start wait | `launchStarting` in `internal/mcp/launch.go`: 5 s, how long the `build` and `score` tools wait for the child to start — a build's progress record to show a stage entered, a score run's ledger and pid to stand — or to exit, before returning the run still starting (SPEC §11) |
| `wait` poll interval | `waitPoll` in `cmd/wait.go`: 1 s, how often `kbase wait` reads the state store while the run it names progresses (SPEC §6) |
| Sheet render concurrency | `sheet.renderConcurrency`: 4, the most `dot` processes at once; output order is kept ([§6](../6-kb-model-and-maintenance/index.md)) |
| Unmarked shortlist, proximity slots | `shortlistProximityK` in `internal/claimgraph`: 3, the nearest targets in its own paper `references-found` asks per source ([§4](../4-stage-table.md)) |
| Unmarked shortlist, wildcard slots | `shortlistWildcardK` in `internal/claimgraph`: 2, the best-ranked targets by cosine it asks per source besides the proximity picks ([§4](../4-stage-table.md)) |

Exit codes and the placeholder's digest form are in SPEC §7 and [§3](../3-principles.md); the pandoc range is SPEC §2; the
stage-commit subject is SPEC §6.

Contents:

- [§12 Format migration (SPEC §10)](format-migration-spec-10.md)
- [§12 MCP server (SPEC §11)](mcp-server-spec-11.md)
