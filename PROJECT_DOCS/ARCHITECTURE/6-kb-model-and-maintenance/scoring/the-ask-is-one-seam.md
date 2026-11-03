#### The ask is one seam

Each item's `Ask` takes the rendered prompt whole and returns a
`score.Reply` — the reply's text and whether the endpoint cut it off at the completion cap — so
the runner has no view of a provider: `internal/score` imports `asks`, `kb`, `query`, `index`,
`write`, `result`, `atomicfile`, `buildstore` and `log`, and never `model` or `build`. A truncated
reply is the runner's to judge, not the seam's: `put` fails its item as it fails a call that
errored, whatever the text it reached would parse to. `scoringLaunch` in
`cmd/score.go` fills the `Launch` seam: it opens the scoring role's client through the registry — so a key-file refusal comes after the start refresh and before `score.Invoke`'s index load — builds the invocation's one `model.Queue`, and hands each item an `Ask` that makes one `Slot.Chat` call on the role's endpoint with an empty system prompt, the item's `--timeout` applied from the call's start, reporting `model.FinishLength` as the truncation; the runner learns nothing of what answered. The development replay takes its launch from the same function, so a replayed call carries the run's endpoint, timeout and the truncation the runner judges.
Each score template splices the `score-system` fragment into its own text, so the instruction is
carried once, in the user turn.

