#### One item, one call

The ask is one tool-less chat call on the `scoring` role's endpoint, over the
chat protocol that endpoint's entry names ([§9.1](../../9-configuration/9.1-providers-toml-the-endpoint-pool/index.md)): `openai` speaks `chat/completions` and `anthropic`
speaks the Messages protocol. Every ask a run sends to an `openai` endpoint carries the run's id as
its `prompt_cache_key`, so a run's asks reach one provider and share one cache entry; the Messages
protocol has no such field, so a run whose endpoint speaks it sends no key. Messages requires a
completion cap, and no other request is capped: the answer's allowance is `max(256, ceil(characters / 4))` over the
characters of the ask's messages and system text — four characters per token because the ask is plain
prose and no LaTeX is ever sent. The call carries the scoring role's sampling settings ([§9.2](../../9-configuration/9.2-config-toml-the-choices/index.md)) in its
protocol's fields ([§9.1](../../9-configuration/9.1-providers-toml-the-endpoint-pool/index.md)), and thinking text is never part of a reply on either protocol. A call
that does not complete is re-issued identically up to the transport attempt count, pausing 5 s then
30 s between attempts (ARCHITECTURE §12). What fails the item at once, on the one attempt that met
it, is a provider's answer carrying a status that is not retryable — any non-2xx other than 429 and
the 5xx class — an answer that reached the completion cap before any answer text, which an
identical re-send would cut off the same way, and a reply the parse refuses. `--resume` is how a user asks a failed item again.

