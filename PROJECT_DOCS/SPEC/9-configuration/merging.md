### Merging

The two files merge by two rules. `providers.toml` merges by provider
name and a provider's table is replaced whole: a repository table of the same name supersedes the global one
entirely, never field by field, so a repository cannot inherit the global table's `apiKeyFile` for an
endpoint it chose; a stored key belongs to the provider name, whatever layer supplies the table.
A provider's name is folded to lowercase as it is read ([page §2.7](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)), so a repository table
overrides the global table of the same name in any case; two tables of one file whose names differ
only by case refuse the load, naming both spellings and the file. A fault naming a provider a higher layer superseded is
not reported: the pool's provider is the higher one, and a diagnostic about a value nobody reads is not a
fact about this run. `config.toml` merges field by field: a value a higher layer states wins, and one
it leaves out keeps the lower layer's value. A layer that fails the strict load refuses the whole
load, naming its file: no lower layer is read, so a broken repository file never silently falls
through to the global one.

