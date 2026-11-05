### 9.6 `kbase show-config`

`kbase show-config [--provider NAME]` reports how this invocation's configuration resolves, and changes nothing: it reads no KB, writes no file and makes no network request. `--provider` names the provider the selection reports. Its result keys are the `show-config` row of [§7](../../7-tool-result-contract-and-exit-codes/item.md). A configuration that fails the strict load is refused before the verb runs ([§9.3](../9.3-strict-load-contract.md)). Otherwise the outcome is `done` whatever the report finds: a fault in the pool and an unresolved role are diagnostics, not refusals, so one call shows every gap at once.

The report states:

- the layers this invocation read, in precedence order, each with its path, the source that named it (`explicit` for a flag, `environment` for `$KBASE_CONFIG_DIR`, `repository` or `global`), and whether the directory and each of its two files exist;
- `config.toml`'s merged `[models]` values, null where a role is unset, and its `[sampling]` values, null where unset;
- every provider in name order, with the layer that supplied it and the chat protocol and decision API it speaks, each default filled in (so a provider silent on `decisionApi` is reported as `systemone`), including a provider that a dropping fault removed (unusable, with the fault's reason);
- the provider the selection names and the rule that named it, or why none was named;
- one record per role: the endpoint it resolves to, the layer that supplied each field, a chat role's sampling settings, or why it does not resolve.

The sources vocabulary is the layers': `repository` or `global` for a value a file supplies, `explicit` or `environment` for one read from the one directory a flag or `$KBASE_CONFIG_DIR` named.

Its diagnostics are, in order:

- one `provider` item per fault, with `path` the `providers.toml` that has the fault (which need not be the file a selection or `configure-providers` names). For a stored key file that does not open, `path` is that file, `detail` is what is wrong with it and, where it is truncated or does not decrypt, `remedy` is the refused-key remedy ([A refused key](../a-refused-key.md)) with the provider's name filled in. For a store directory that exists and does not list, `path` is that directory and `detail` is what went wrong, with no `remedy`;
- one `provider` item per provider whose stored key leaves the `apiKeyFile` it states unread, `path` the `providers.toml` that states it and `remedy` removing that line from the file, or `kbase auth remove <provider>` to use the file;
- one item per role that does not resolve, its `path` naming the file the setting belongs in: the write target's `providers.toml` or `config.toml`.

The upgrade (below) and the unread-`apiKeyFile` warning appear in this report only: no other verb's output has them.

Contents:

- [§9.6 Names and paths, never secrets](names-and-paths-never-secrets.md)
- [§9.6 It describes the process that ran it](it-describes-the-process-that-ran-it.md)
