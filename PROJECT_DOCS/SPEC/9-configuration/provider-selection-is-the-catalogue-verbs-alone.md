### Provider selection is the catalogue verbs' alone

`models` selects a provider to send a request to, not a role: a role is never chosen this way, and `config.toml` names no provider. The rule is `--provider`, else the sole provider. `models` refuses (`check: provider`) where it can select none: zero usable providers, two or more with none selected, or a `--provider` the pool does not hold.

- An empty pool, and a name the pool does not hold (in `--provider` or in a role's reference), point at the `providers.toml` that `configure-providers` writes into with no location flag ([The configure commands' write location](the-configure-commands-write-location.md)), the file a provider's table belongs in. `configure-models`' value points instead at the `providers.toml` of the layer it writes, checked against that layer's stack ([Provider commands](provider-commands.md)).
- A name the pool does not hold also has `key` the argument or `models.<role>` that named it, `allowed` the pool's provider names, a `detail` naming the table to add there (`configure-models`' says instead that its flag names a provider no `providers.toml` this call reads has), and a `remedy` naming `kbase configure-providers <name> --base-url <url>` (`configure-models`' including its location flag), then `kbase show-config`.
- Where that provider was dropped from the pool by a fault, the fault's own reason is the `detail`, with `kbase show-config` alone as the remedy.
- Two or more providers with none chosen list every provider name as `allowed`, with `key` the `provider` argument.

The argument, and each command a remedy names, are formatted as the `key` and `remedy` rules in [§7](../7-tool-result-contract-and-exit-codes/item.md) state. `show-config` reports any of these in its document and still exits 0.

