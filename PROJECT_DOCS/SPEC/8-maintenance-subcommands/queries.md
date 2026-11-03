### Queries

Each query derives its answer from the KB's records and statement markers and writes nothing;
`kbase <query> --help` states its arguments, and [§7](../7-tool-result-contract-and-exit-codes/index.md) gives its result keys. A KB converted at load
and not yet saved answers as converted, its result carrying `migration`; a KB whose load fails is
refused ([page §2.4](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).
