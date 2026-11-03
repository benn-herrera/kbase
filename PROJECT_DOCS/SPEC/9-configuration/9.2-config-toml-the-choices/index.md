### 9.2 `config.toml` — the choices

```toml
[models]
decision = "<entry>/<id>"      # the claim graph's multichoice asks: a providers.toml
                               # entry name, then the model that entry's
                               # decision surface serves
text = "<entry>/<id>"          # the overview passage
scoring = "<entry>/<id>"       # kbase score's asks; required for score

[asks]
concurrency = 4                # optional; model calls one build or score has in flight at once, across every role

[sampling.text]                # optional; the overview passage's sampling
temperature = 0.0              # optional; a number
thinking = "off"               # optional; off | low | medium | high
[sampling.scoring]             # optional; kbase score's asks' sampling, the same two keys
```

**A `[models]` value is a reference**, `<provider>/<model>`, split at the **first** `/`: a provider
name is a `providers.toml` table's and carries none, while a model id may (`openrouter/anthropic/…`).
Either part empty, or no `/` at all, is not a reference and is refused at resolution, naming
`models.<role>` and the form. Nothing is normalised but whitespace and the provider part, folded to
lowercase ([page §2.7](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)): a model id is the provider's spelling, not kbase's.

A role left unset resolves to "not configured" — no defaulting, no guessing, and no role falls
back to another. `concurrency` is how many model calls one invocation of `build` or `score` has in
flight at once, across every role; absent, it is 4, and `--ask-concurrency` overrides it for that
invocation. A value under 1 fails the load, naming `asks.concurrency`. Calls start in the order the
invocation submits them. There is no `[dev]` table, top-level `provider` key or `[asks]
readerConcurrency`: a file stating any of them fails strict load ([§9.3](../9.3-strict-load-contract.md)) naming the key, and the
remedy is a hand edit. A
`[models]` table spelling `heavy`/`light` — the superseded role names — fails strict load as
unmodeled keys ([§9.3](../9.3-strict-load-contract.md)); the remedy is a hand edit.

Contents:

- [§9.2 Sampling is per chat role](sampling-is-per-chat-role.md)
