#### The decision surface

`decisionsFlavor` names the wire shape the decision call uses, chosen per
server: it changes nothing about chat, which `api` alone decides, and it binds
no model id — the decision role's model id stays in `config.toml` ([§9.2](../9.2-config-toml-the-choices/index.md)). An entry silent on it speaks
`systemone`, the shape kbase's multichoice asks were written against. `systemone`: `POST
<baseUrl>/systemone`, body `model`, `state` and `questions` — an object keyed by each question's own
key — answered by an `answers` object keyed the same way. `decisions`: `POST <baseUrl>/decisions`,
body `model`, `input` (the same composed state) and `questions` — an array of `type`, `name`,
`instructions` and `choices`, where a `noul` question is asked as a `predicate` and a `choice`
question's criteria are its `choices`, each option a `value` with its guidance as the `description`,
sorted by value — answered by an `answers` array, each entry carrying the `name` it answers: a
`predicate` (a probability, read as P(true)), a `choice` (its choice, confidence and per-value
probabilities), a `score`, or a `refusal` (a declined answer, carrying no value, which leaves the
question unanswered). An answer naming a question the request did not ask is refused under either
shape; a question left unanswered is the seam's own re-ask-then-default path.

