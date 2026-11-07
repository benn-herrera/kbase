#### The ask is one seam

**Pattern:** Dependency injection.

`Ask` takes the rendered prompt whole and returns a `score.Reply`: the reply's text and whether the
endpoint truncated it at the completion cap. The runner has no view of a provider. `cmd/importpolicy_test.go`
enforces that `internal/score` never imports `model` or `build`: `internal/score` imports `argmark`,
`asks`, `atomicfile`, `buildstore`, `kb`, `log`, `query`, `result` and `write`. A truncated reply is the
runner's to judge, not the `Ask`'s: `put` fails its item as it fails a call that returned an error,
whatever the text would parse to. A corrective re-ask ([the runner](the-runner.md)) is a second call
through the same `Ask`, made from inside the item's own queue item, so it holds the item's slot and adds
no queue and no goroutine. The re-ask is one whole prompt — the first ask's with the correction after
it — not a follow-up turn in a chat: the seam stays one string in and one reply out, so the runner, the
captures and the replay handle a re-ask exactly as a first ask, and nothing of a conversation's state
crosses it.

`scoringLaunch` in `cmd/score.go` fills `Launch`. It builds the invocation's one `model.Queue` and
returns a `Launch` that starts each item on a slot of it. The item's `Ask` makes one `Slot.Chat` call on
the scoring role's endpoint with an empty system prompt, applies `--timeout` from the call's start
(never over the wait for a permit), and reports `model.FinishLength` as the truncation. The `modelclient`
registry opens the client at the first call, so a run refused at its scope or holding no item connects
to nothing. A provider's refusal of the call's key is a `modelclient.KeyRefusedError`; `scoringLaunch`
turns it into a `score.RemedyError`, which the failed attempt's ledger line records. Retry and its pauses
are the model client's alone ([§7](../../7-inference-seams/index.md)), and the attempt count and pauses are in
[§12](../../12-constants/index.md). The development replay takes its `Launch`
from the same function, so a replayed call carries the run's endpoint, timeout and truncation rule.

Each score template splices the `score-system` fragment into its own text, so the instruction is carried
once, in the user turn.

