### The role record

A verb whose endpoints are per role states them in `roles`, one record per role
it resolved, in role order — `decision`, `text`, `scoring` — with these keys in this order, each
omitted where it does not apply:

| Key | Holds |
|---|---|
| `role` | `decision`, `text` or `scoring` |
| `provider` | the provider's name |
| `provider-rule` | the rule that named it: `flag` (a flag overrode the reference) or `config` (the reference named it) |
| `provider-layer` | the layer that supplied the provider: `explicit`, `environment`, `repository` or `global` |
| `model` | the model id this invocation asked for; null where no source named one |
| `base-url`, `sources` | `show-config` alone: the endpoint, and the layer that supplied each field |
| `temperature`, `thinking` | `show-config` alone, chat roles: the role's sampling settings, null where unset ([§9.2](../9-configuration/9.2-config-toml-the-choices/index.md)) |
| `detail` | `show-config` alone: why the role does not resolve, where it does not |

`provider`, `provider-rule` and `provider-layer` stand together on every role that resolved: a pool
entry supplies every endpoint. `provider-layer`'s `environment` is the directory `$KBASE_CONFIG_DIR`
names ([§9](../9-configuration/index.md)).

**The model is the one requested, never one observed.** `model` is the id this invocation asked for,
and no result document or ledger line states one a server ran.

