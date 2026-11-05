### Wire clients

**Pattern:** Provider adapters behind one interface; connection pooling.

`internal/model` contains two implementations of the chat request. `model.OpenAIClient`
(`openai_client.go`) speaks `chat/completions`, and requests the usage chunk
(`stream_options.include_usage`): without it a streamed response carries no `usage` and every token
count reads zero. `model.AnthropicClient` (`anthropic_client.go`) speaks
the Messages API: `POST <baseUrl>/messages`, the `x-api-key` and `anthropic-version` headers, its own
event sequence and its own catalogue shape. A provider silent on `api` gets the Messages client
(SPEC §9.1 The chat protocol). `model.NewClient` dispatches on `model.Endpoint.Protocol`, a value
`modelclient` resolves from `config.Endpoint.Protocol` (`chatProtocol`); `Endpoint.Decisions` is
resolved from `decisionsFlavor` the same way (`decisionsSurface`). `internal/model` reads no
configuration string, imports no kbase package but `secret`, and touches no file (`TestModelImportsNoOS`).
The body each client sends is its protocol's, never shaped by host or provider name.

Each client's HTTP client has no total timeout, which would cap a streamed generation. The
bounds are the call's context deadline, `responseHeaderTimeout` for the handshake and
`streamIdleTimeout` for the gap between reads ([§12](../12-constants/index.md)). A non-2xx answer is a
`model.StatusError` from both, its body truncated at `errorBodyLimit` and scrubbed of any key the server
echoed (`statusError`).

**Pattern:** Vendor parameter mapping.

`model.Sampling` is the sampling type for both protocols. Each client resolves `Endpoint.Sampling` once, at
construction, with `Sampling.Resolved()`: `DefaultTemperature` and thinking off where the role states
none. `chat()` copies it onto every request through `Client.sampling()` and shapes no field of either
protocol; each protocol's wire rules live only in its body encoder (`encodeRequest`,
`encodeAnthropicRequest`). `model.SamplingOf` returns a copy, so no caller can alter what a client
sends. The fields each protocol sends are SPEC §9.1 The chat protocol. The Messages encoder sets the
required `max_tokens` to the thinking budget (`thinkingBudgetLow`, `thinkingBudgetMedium`,
`thinkingBudgetHigh`) plus an answer allowance: the larger of `maxTokensFloor` and a quarter of the
request's characters, rounded up ([§12](../12-constants/index.md)). The Messages protocol requires `max_tokens` and kbase has no
figure of its own, so the cap is estimated from the request; four characters per token is the usual
English-prose estimate, and the floor keeps a one-word request from being cut off. The estimate
assumes no dense notation reaches a call, which a statement holding TeX maths does not meet; and why
the request's size, not the expected answer's, sets the allowance is not recorded.

**Pattern:** Provider prompt-cache affinity.

`modelclient.New` takes a prompt-cache key. A scoring run and a scoring replay pass their id, and the
registry sets it on every endpoint it builds. The OpenAI client sends it as `prompt_cache_key` on every
request, so one run's calls reach one upstream and share a cache entry. A verb that names no run passes
none, and the field is omitted rather than sent empty. The Messages client has no such field and
sends none.

