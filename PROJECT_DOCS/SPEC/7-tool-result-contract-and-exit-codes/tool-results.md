### Tool results

Every subcommand, `models` and the configure commands included, writes one YAML document to
stdout and nothing else, but for `kbase auth get-piped`'s success, which writes the recovered key
alone (and a failure to write it, part of the key before the result) ([Provider commands](../9-configuration/provider-commands.md)).

- That YAML is the subcommand's **result**; `results` is one key inside some results (a query's rows).
- A score run's result, which kbase stores beside its ledger and `status` and `wait` print again, is a
  **stored result** ([The ledger and resume](../8-maintenance-subcommands/score/the-ledger-and-resume.md)).
- A write's input is a **batch** ([Write batches](../8-maintenance-subcommands/write-batches/index.md)).

stderr is for humans. Keys are in fixed order; strings are quoted on emit; a refusal enumerates
every offending item. The `outcome` key is the contract. `--help` and `--version` are human-facing
and outside it; `mcp`'s stdout transports JSON-RPC frames ([§11](../11-the-mcp-server/index.md)),
though a usage error on its own command line, before it serves, still prints a YAML refusal.

