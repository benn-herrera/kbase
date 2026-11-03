## 9. Result emitter

`internal/result` owns the YAML result document and its emitter, and nothing else writes stdout
(I5). Fixed key order, strings quoted on emit, a refusal enumerating every offending item.
`internal/result` maps `outcome` to exit code per SPEC §7 (`ExitCode`) and to whether the outcome is
an error (`IsError`): `cmd` exits with the first, and an MCP tool result's `isError` is the second.
`Emit` closes a `retry` or `failed` document's items with the outcome's ask, ending each `detail`
(SPEC §7); `RetryAsk` and `FailedAsk` are the one home of both sentences, and a caller hands `Emit`
its items without them. YAML uses the package named in CONVENTIONS "Dependency Policy".

