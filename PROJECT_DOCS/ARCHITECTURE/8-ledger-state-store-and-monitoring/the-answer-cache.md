### The answer cache

**Pattern:** Content-addressed memoization cache.

`internal/asks/call` keeps one answer per call under
`scratch/trails/<start-commit>/answers/`, written through `atomicfile`. A chat answer is keyed by the
digest of the model, the system prompt, the prompt, the attempt ordinal and the client's
sampling, which contributes key parts only where it is not kbase's default (`samplingKey`). A decision
answer is keyed by the model, the question whole, the state and the attempt ordinal, and by the
correction text where a refusal's re-ask carries one; a key with no correction is the key it was before
corrections existed. The attempt ordinal is in the key so a retry reaches the model instead of finding
the first answer.
`AskTotals.WarmCacheReadTokens` and `CallStats.CachedTokens` read 0 on every decision call: neither
decision API's usage block has a cached-token field, so the zero is the API's.

