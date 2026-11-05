## Module Structure

The package map and its dependency rules are ARCHITECTURE "Package map"; this file does not restate
them. Working rules for packages that carry over:

- **cmd/**: verb logic lives behind a plain options-struct function; the cobra `RunE` is a thin
  loader shell, so verbs unit-test without process/network.
- **internal/config**: loaders take explicit paths (tests never touch a real home). Per-entry
  provider faults never abort the pool; fault reasons contain no key material. Resolution is pure and
  lives here alone — `Resolve`, `Select` — and reads no file, opens no client and writes no
  stderr (a refusal's remedy may name the command that fixes it, such as `kbase configure-models`). The package imports no kbase package but
  `atomicfile`, `result` (it refuses in `result.Refusal`, each item with its check),
  `argmark` (a refusal names an argument or a remedy's call in its marked form) and `secret` (a
  key is a `secret.Key`), never
  `model` or `log`: it returns its faults as data, so each caller decides whether one is a warning
  or a result field. A `[models]` value is parsed by one function,
  `ParseReference` (`<provider>/<model>`, split at the first `/`), and `Reference.String` writes such a
  reference back. No site splits an id or concatenates a reference by hand.
- **internal/model**: `Endpoint{Name, BaseURL, APIKey, Protocol, Decisions, PromptCacheKey, IdleConns, Sampling}` is the
  transport-level slice of a provider — `Protocol` saying which client the registry builds
  (the chat protocol the role's endpoint speaks, and always `openai` for the `decision` role), `Decisions` whether it speaks the Decisions shape (`systemone` unless the entry says `decisions`), and `PromptCacheKey` the run that every OpenAI-API model
  call to it sends, and `IdleConns` the idle connections per host its client keeps (the invocation's
  concurrency; zero leaves the defaults) — and the single source of the mapping from configuration's vocabulary onto `Protocol`,
  `Decisions` and `Sampling` (`chatProtocol`, `decisionsSurface`, `samplingOf`) is `internal/modelclient`. The chat calls — the overview and each scoring call — go through
  a `Slot`'s `Chat`: one tool-less request, retried identically on an incomplete stream up to the
  attempt count (ARCHITECTURE §12); an attempt cut off at the completion cap with no answer text is
  `ChatTruncatedEmpty` and ends the call on that attempt. An attempt's outcome is the classification of what ended it, and
  the two failures are not interchangeable: an endpoint that answered with a non-2xx status refused
  the request and is `ChatProviderRefused` (its `StatusError` matched with `errors.As`, its status and
  body in the reason), where `ChatTransportFailure` is a connection, a name, a certificate or a
  stream that failed with no answer — reading one as the other sends a diagnosis to the path to a
  provider that is answering. The multichoice calls go through a `Slot`'s `Decision`, one `POST` to whichever
  API flavour the endpoint names (ARCHITECTURE §7; SPEC §9.1): `systemone` asks `<base-url>/systemone` one
  fixed-set question (`choice` or `noul`), answered by probabilities over the offered letters;
  `decisions` speaks the OpenAI Decisions shape at `<base-url>/decisions` — the state as `input`,
  questions a named array with a `noul` asked as a `predicate` with no choices, its criteria
  folded into `instructions`, and an `answers` array where a `refusal` is unreadable and takes the
  retry-once-then-default path. Either API flavour is retried identically on a transport error or
  retry-class 5xx. API keys never appear in logs or error strings. The `[models]` keys are
  `decision`, `text` and `scoring`, and each is a `<provider>/<model>` reference.

---

