#### The chat protocol

`api` names the wire protocol chat uses, chosen per server; it is a two-value
field. `anthropic` is the Anthropic Messages surface, `POST <baseUrl>/messages` — what an entry
silent on `api` gets — and `openai` is the OpenAI-compatible `chat/completions` surface. The two
differ in more than a path. Over the OpenAI-compatible protocol a thinking level is an effort hint
that the server may exceed without bound; a Messages request carries `budget_tokens` and a
`max_tokens` ceiling, and the ceiling bounds thinking even where a server overruns the budget.
Messages requires a `max_tokens` cap: kbase's allowance for the answer is
`max(256, ceil(characters / 4))` over the ask's messages and system text ([§8](../../8-maintenance-subcommands/index.md)), and a role
whose `thinking` is not `off` ([§9.2](../9.2-config-toml-the-choices/index.md)) adds its thinking budget — 1,024 tokens for `low`, 4,096
for `medium`, 16,384 for `high` — sent as `thinking: {type: enabled, budget_tokens: <budget>}`;
thinking `off` is sent as `thinking: {type: disabled}`, the field's absence leaving it at the
server's default. Messages takes the system prompt as a top-level `system` string rather
than a turn, so a system message is lifted out of `messages`. `temperature` is the role's, 0
where unset, and is sent whatever the thinking level. Anthropic's own endpoint rejects a
temperature beside thinking. It has no
`prompt_cache_key`, `chat_template_kwargs`, `reasoning` or `reasoning_effort` field, and
none is sent. This mapping targets servers that implement the Messages protocol — oMLX,
LiteLLM-style proxies, OpenRouter; Anthropic's own newer models may reject `budget_tokens`,
`temperature` or `thinking: {type: disabled}`. An `openai` request carries the role's
`temperature`, 0 where unset; thinking `off` sends
`chat_template_kwargs: {enable_thinking: false}` and `reasoning: {enabled: false}`, and a
level sends it three ways at once — `reasoning_effort: <level>`,
`reasoning: {effort: <level>}` and `chat_template_kwargs: {enable_thinking: true}` — since a
server reads whichever of them it speaks. A Messages entry's
catalogue is `GET <baseUrl>/models`, whose `created_at` is RFC 3339. `decisionsFlavor` is unaffected
by `api`, and the decision surfaces are OpenAI-shaped: the `decision` role's multichoice asks
always use the OpenAI-shaped decision surface on its entry's base URL, authenticated as an `openai`
entry is, whatever the entry's `api`. A server that speaks only Messages has no such surface, and the
decision call fails with that server's HTTP status.

