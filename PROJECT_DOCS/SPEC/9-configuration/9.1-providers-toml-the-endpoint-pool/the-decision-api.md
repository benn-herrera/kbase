#### The decision API

`decisionApi` names which of two decision APIs the server speaks; each has its own endpoint. It changes nothing about chat, which `api` alone decides, and it names no model id: the decision role's model id stays in `config.toml` ([§9.2](../9.2-config-toml-the-choices/index.md)). An entry silent on it speaks `systemone`.

- `systemone`: `POST <baseUrl>/systemone`, body `model`, `state` and `questions`, an object keyed by each question's own key. The response is an `answers` object keyed the same way.
- `decisions`: `POST <baseUrl>/decisions`, body `model`, `input` (the same composed state) and `questions`, an array of `type`, `name`, `instructions` and `choices`. A `noul` question is asked as a `predicate`. A `choice` question's criteria are its `choices`, each option a `value` with its guidance as the `description`, sorted by value. The response is an `answers` array, each entry having the `name` it answers: a `predicate` (a probability, parsed as P(true)), a `choice` (its choice, confidence and per-value probabilities), a `score`, or a `refusal` (a declined answer, with no value, which leaves the question unanswered).

An answer naming a question the request did not ask is refused under either shape. A question answered with a `refusal` is asked once more with a correction appended: to the question's `instructions` under `decisions`, to the end of `state` under `systemone`. A question with no answer, or with any other answer that names no offered letter, is asked once more identically. If the second answer decides nothing either, the question takes its deterministic default ([§5](../../5-build-behaviour/co-linearity-cull-claims-culled.md)).

