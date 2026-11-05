### Merging

The two files merge by two rules.

- `providers.toml` merges by provider name, and a provider's table is replaced whole: a repository table of the same name supersedes the global one entirely, never field by field, so a repository cannot inherit the global table's `apiKeyFile` for an endpoint it chose. A stored key belongs to the provider name, whatever layer supplies the table. Provider names compare case-insensitively ([§9.1](9.1-providers-toml-the-endpoint-pool/index.md)), so a repository table overrides the global table of the same name in any case. A table that a fault drops ([§9.1](9.1-providers-toml-the-endpoint-pool/index.md)) supersedes nothing: the lower layer's table of that name remains in the pool, and the fault is reported. A fault naming a provider that a higher layer supersedes is not reported: the pool's provider is the higher one, and a diagnostic about a value nobody reads is not a fact about this run.
- `config.toml` merges field by field: a value a higher layer states wins, and one it leaves out keeps the lower layer's value.

A layer that fails the strict load refuses the whole load, naming its file ([§9.3](9.3-strict-load-contract.md)): no lower layer is read, so a broken repository file never silently falls through to the global one.

