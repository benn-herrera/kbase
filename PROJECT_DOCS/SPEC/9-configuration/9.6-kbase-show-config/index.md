### 9.6 `kbase show-config`

`kbase show-config [--provider NAME]` reports how this invocation's configuration resolves, and
changes nothing: it reads no KB, writes no file and makes no network request. A configuration that
fails strict load is refused before the verb runs, as for every verb ([§9.3](../9.3-strict-load-contract.md)). Otherwise the outcome is
`done` whatever the report finds: a fault in the pool and an unresolved role are diagnostics, not
refusals, so one call shows every gap at once. It makes no request and opens no client.

The report states the layers this invocation read, in precedence order, each with its path, the
source that named it — `explicit` (a flag), `environment` (`$KBASE_CONFIG_DIR`), `repository` or
`global` — and whether the directory and each of its two files stand; `config.toml`'s merged
`[models]` values, null where a role is unset, and its `[sampling]` values, null where unset; every provider in name order with the layer
that supplied it and the decision surface it speaks — its default filled in, so a provider silent on
the flavour reads `systemone` — including a provider a dropping fault removed (unusable, with the
fault's reason); the provider the selection names and the rule that named it, or
why none was named; one record per role — the endpoint it resolves to, the layer that supplied
each field, a chat role's sampling settings, or why it does not resolve. Its arguments, result and
the `upgrade` a provider with a file key carries are [page §3.7](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/show-config.md)'s; every provider it names is named
in lowercase ([page §2.7](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)).

The sources vocabulary is `layers`': `repository` or `global` for a value a file holds, `explicit` or
`environment` for one read from the one directory a flag or `$KBASE_CONFIG_DIR` named. Its
diagnostics are one `provider` item per fault, each carrying `path` the `providers.toml` that carries
the fault — which need not be the file a selection or `configure-providers` names — or, for a stored key file
that does not open, that file, its `detail` what is wrong with it and, where it is truncated or
does not decrypt, `remedy` the refused-key remedy ([§9](../index.md), "A refused key"), the provider's name filled
in — or, for a store directory that stands and does not list, that directory and its `detail` what
went wrong, with no `remedy`; then one `provider` item per provider whose stored key leaves the
`apiKeyFile` it states unread, `path` the `providers.toml` that states it and `remedy` removing
that line from the file, or `kbase auth remove <provider>` to use the file; then one item per role
that does not resolve, its `path` naming the file the setting belongs in: the
write target's `providers.toml` or `config.toml`. The upgrade and the unread-`apiKeyFile` warning are
this report's alone: no other verb's output carries them.

Contents:

- [§9.6 Names and paths, never secrets](names-and-paths-never-secrets.md)
- [§9.6 It describes the process that ran it](it-describes-the-process-that-ran-it.md)
