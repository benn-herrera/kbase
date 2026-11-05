### The decision API

**Pattern:** Constrained classification endpoint.

A multichoice question is one `POST` to the provider's decision API (SPEC §9.1 The decision API): a
`Slot.Decision` call with one `noul` or `choice` question, whose answer is read by the key it echoes, so
no reply text is parsed. `decisionsFlavor` selects between two request shapes, and a provider silent on
it gets `systemone`.

- `systemone`: `model`, `state`, and `questions` as a JSON object keyed by the question key, each
  question's `criteria` having the per-letter meanings. The `answers` object returns keyed the same way.
- `decisions`: the state as `input`, and `questions` as an array of named questions. A `noul` question
  is sent as a `predicate`, which has no choices, so its true and false guidance goes into
  `instructions` (`predicateInstructions`); a `choice` question's criteria are `choices`, sorted by
  value. The `answers` array names the question each entry answers.

Both decoders map into `model.DecisionAnswer`, so a caller cannot tell the shapes apart. Under either,
`validateDecisionAnswers` refuses an answer naming a question kbase did not send. A `refusal`, a missing
answer and an answer with no value decode to an answer with no confidence, so `asks.Decide` finds no
letter and the question takes the multichoice-ask interface's retry-once-then-default path
([next](the-multichoice-ask-interface.md)).

Both shapes are OpenAI-shaped, so the registry gives the `decision` role an `OpenAIClient` whatever its
provider's `api`. A provider whose `api` is `anthropic` keeps its Messages client for chat and gains an
OpenAI client, on the same base URL and key, for decisions. A Messages-only server has no decision API
and the call fails with that server's status. `AnthropicClient.decision` refuses, and nothing in kbase
reaches it. A decision call has no system message.

