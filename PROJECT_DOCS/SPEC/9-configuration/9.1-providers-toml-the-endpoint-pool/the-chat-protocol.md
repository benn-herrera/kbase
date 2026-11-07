#### The chat protocol

`api` names the wire protocol chat uses, chosen per server; it has two values. `anthropic`, what an entry silent on `api` gets, is the Anthropic Messages API, `POST <baseUrl>/messages`. `openai` is the OpenAI-compatible `chat/completions` API. The two differ in more than a path. Over the OpenAI-compatible protocol a thinking level is an effort hint that the server may exceed without bound. A Messages request has `budget_tokens` and a `max_tokens` ceiling, and the ceiling bounds thinking even where a server overruns the budget.

Every chat call and catalogue listing (`models`, `configure-providers` and `configure-models`, `probe`'s chat selections, a build's overview call, a scoring run's calls) uses its entry's `api`. The `decision` role's calls never do (below).

A Messages request:

- Requires a `max_tokens` cap. The allowance for the answer is `max(256, ceil(characters / 4))` over the characters of the request's messages and system text. A role whose `thinking` is not `off` ([§9.2](../9.2-config-toml-the-choices/index.md)) adds its thinking budget: 1,024 tokens for `low`, 4,096 for `medium`, 16,384 for `high`, sent as `thinking: {type: enabled, budget_tokens: <budget>}`. Thinking `off` is sent as `thinking: {type: disabled}`; the field's absence would leave it at the server's default.
- Takes the system prompt as a top-level `system` string rather than a turn, so a system message is lifted out of `messages`.
- Sends `temperature`, the role's, 0 where unset, whatever the thinking level.
- Sends no `prompt_cache_key`, `chat_template_kwargs`, `reasoning` or `reasoning_effort` field.

This mapping targets servers that implement the Messages protocol: oMLX, LiteLLM-style proxies, OpenRouter. Anthropic's own endpoint rejects a temperature beside thinking, and its newer models may reject `budget_tokens`, `temperature` or `thinking: {type: disabled}`. A Messages entry's catalogue is `GET <baseUrl>/models`, whose `created_at` is RFC 3339.

An `openai` request contains the role's `temperature`, 0 where unset. Thinking `off` sends `chat_template_kwargs: {enable_thinking: false}` and `reasoning: {enabled: false}`. A level sends it three ways at once, since a server reads whichever it speaks: `reasoning_effort: <level>`, `reasoning: {effort: <level>}` and `chat_template_kwargs: {enable_thinking: true}`.

`decisionApi` is unaffected by `api`, and both decision APIs are OpenAI-shaped: the `decision` role's multichoice questions always go to the decision API's endpoint on its entry's base URL, authenticated as an `openai` entry is, whatever the entry's `api`. A server that speaks only Messages has no such endpoint, and the decision call fails with that server's HTTP status.

