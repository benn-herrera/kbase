### 9.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses within the provider-name rule:
each matches `^[A-Za-z0-9_][A-Za-z0-9_-]*$`, and an entry whose name does not fails the file's
load, naming the file and the entry. A name is case-insensitive and read in lowercase, and two names
of one file that differ only by case are refused ([page §2.7](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md); [§9](../index.md), "Merging"). An
entry:

```toml
[<name>]
baseUrl = "..."        # required
apiKeyFile = "..."     # path to a key file, resolved relative to this file's
                        # own directory if relative; an entry's only key field
type = "inference"     # optional; the only accepted value, also the default
api = "anthropic"      # optional; "anthropic" (also the default) or "openai" —
                       # the chat protocol this server speaks
decisionsFlavor = "systemone"  # optional; "systemone" (also the default) or
                               # "decisions" — the decision surface this
                               # server speaks
```

No other key is recognized inside an entry. An entry that fails validation (empty `baseUrl`; a
`baseUrl` carrying credentials (`user:key@`), or one that is no absolute `http` or `https` URL
with a host, its reason naming no part of the URL; or `type`/`api`/`decisionsFlavor` set to
anything but its accepted values) is **dropped** with a
warning naming the entry and the reason (never key material); the rest of the pool still loads. A
key-file read failure faults just that entry the same way. An absent `providers.toml` is an empty
pool, not a load error.

Contents:

- [§9.1 The chat protocol](the-chat-protocol.md)
- [§9.1 The decision surface](the-decision-surface.md)
