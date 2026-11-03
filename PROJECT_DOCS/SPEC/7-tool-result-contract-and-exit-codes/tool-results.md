### Tool results

Every subcommand, `models` and the configure commands included, writes one YAML document to
stdout and nothing else. stderr is for humans. Keys are in fixed order; strings are quoted on emit;
a refusal enumerates every offending item. The `outcome` key is the contract. `--help` and
`--version` are human-facing and outside it; `mcp`'s stdout carries JSON-RPC frames ([§11](../11-the-mcp-server.md)), though a
usage error on its own command line, before it serves, still prints a YAML refusal.

