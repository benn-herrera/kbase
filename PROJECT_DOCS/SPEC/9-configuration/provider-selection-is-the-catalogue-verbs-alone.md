### Provider selection is the catalogue verbs' alone

`models` selects a provider to
dial — not a role: a role is never chosen this way, and `config.toml` names no provider. The rule is
`--provider`, else the sole provider. Where no provider is named — zero usable providers, two or more with none
selected, or a `--provider` the pool does not hold — `models` refuses. An empty pool,
and a name the pool does not hold — here, or in a role's reference — point at the
`providers.toml` `configure-providers` writes into with no location flag ([§9](the-configure-commands-write-location.md), "The
configure commands' write location"), the file a provider's table belongs in; `configure-models`' value
points instead at the `providers.toml` of the layer it writes, checked against that layer's stack
([Provider commands](provider-commands.md)). The name the pool does not hold
also carries `key` the argument or `models.<role>` that named it, a detail naming the table to add
there — `configure-models`' says instead that its flag names a provider no `providers.toml`
this call reads holds — and a remedy naming `kbase configure-providers <name> --base-url <url>`, `configure-models`'
carrying its location flag, then `kbase show-config`. Where that provider faulted out of the pool, the fault's own reason stands
instead, with `kbase show-config` alone as the remedy. Two or more providers with none chosen list every provider name as `allowed` with
`key` the `provider` argument. The argument, and each command a remedy names, are spelled as
[§7](../7-tool-result-contract-and-exit-codes/item.md)'s `key` and `remedy` state. `show-config` reports any of these in its document and still exits 0.

