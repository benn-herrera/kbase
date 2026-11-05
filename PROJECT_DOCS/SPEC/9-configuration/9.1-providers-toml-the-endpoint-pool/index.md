### 9.1 `providers.toml` — the endpoint pool

Top-level tables are provider names, an open set the user chooses. A provider name is one or more ASCII letters, digits, `_` or `-`, not beginning with `-` (`^[A-Za-z0-9_][A-Za-z0-9_-]*$`). A table whose name breaks the rule fails the file's load, naming the file and the entry.

**A provider name is case-insensitive everywhere.** Every input naming a provider is case-folded to lowercase without complaint: each table name read from `providers.toml`, `--provider`, the provider part of a model reference, the provider that `configure-providers` or `configure-models` is given, and the provider that `auth add` or `auth remove` is given. Every output names a provider in lowercase. Two tables of one file whose names differ only by case refuse the load, naming both names as written and the file.

An entry:

```toml
[<name>]
baseUrl = "..."        # required
apiKeyFile = "..."     # path to a key file, resolved relative to this file's
                        # own directory if relative; an entry's only key field
type = "inference"     # optional; the only accepted value, also the default
api = "anthropic"      # optional; "anthropic" (also the default) or "openai" —
                       # the chat protocol this server speaks
decisionApi = "systemone"  # optional; "systemone" (also the default) or
                               # "decisions" — the decision API this
                               # server speaks
```

A key inside an entry that this list does not name fails the file's load ([§9.3](../9.3-strict-load-contract.md)). An entry that fails validation is **dropped** with a warning naming the entry and the reason (never key material), and the rest of the pool still loads. An entry fails validation when:

- its `baseUrl` is empty;
- its `baseUrl` contains credentials (`user:key@`), or is no absolute `http` or `https` URL with a host (the reason names no part of the URL);
- `type`, `api` or `decisionApi` is set to anything but its accepted values.

A failure to read the key file faults just that entry the same way. An absent `providers.toml` is an empty pool, not a load error.

Contents:

- [§9.1 The chat protocol](the-chat-protocol.md)
- [§9.1 The decision API](the-decision-api.md)
