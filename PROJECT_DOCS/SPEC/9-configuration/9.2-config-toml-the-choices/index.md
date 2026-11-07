### 9.2 `config.toml` — the choices

```toml
[models]
decision = "<entry>/<id>"      # the claim graph's multichoice questions: a providers.toml
                               # entry name, then the model that entry's
                               # decision API serves
text = "<entry>/<id>"          # the overview passage
scoring = "<entry>/<id>"       # kbase score's model calls; required for score

[asks]
concurrency = 4                # optional; model calls one build or score has in flight at once, across every role

[sampling.text]                # optional; the overview passage's sampling
temperature = 0.0              # optional; a number
thinking = "off"               # optional; off | low | medium | high
[sampling.scoring]             # optional; kbase score's sampling, the same two keys
```

**A `[models]` value is a reference**, `<provider>/<model>`, split at the **first** `/`: a provider name is the name of a `providers.toml` table and contains none, while a model id may (`openrouter/anthropic/…`). Either part empty, or no `/` at all, is not a reference and is refused at resolution, naming `models.<role>` and the form. Nothing is normalised but surrounding whitespace and the provider part, case-folded to lowercase ([§9.1](../9.1-providers-toml-the-endpoint-pool/index.md)): a model id keeps the provider's formatting, not kbase's.

A role left unset resolves to "not configured": no defaulting, no guessing, and no role falls back to another.

`concurrency` is how many model calls one invocation of `build` or `score` has in flight at once, across every role. Absent, it is 4. `--ask-concurrency` overrides it for that invocation, and a flag value under 1 is a usage refusal keyed by the flag. A `concurrency` under 1 fails the load, naming `asks.concurrency`. Calls start in the order the invocation submits them.

Contents:

- [§9.2 Sampling is per chat role](sampling-is-per-chat-role.md)
