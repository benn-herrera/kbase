### Resources

| URI | Type | Listed | Contents |
|---|---|---|---|
| `kbase://sheet/<kb-root-relative path>` | `image/svg+xml` | with a default project whose KB opens | each claim-graph sheet ([§3](../3-compatibility-contract/claim-graph-svg/index.md)) present on disk, the placeholder included |
| `kbase://model-hints` | `text/markdown` | always | guidance on choosing a model for each role, in the role vocabulary ([§9.2](../9-configuration/9.2-config-toml-the-choices/index.md)), the same rows a created `config.toml` has |
| `kbase://help/<command>` | `text/plain` | always | one per command exposed as an MCP tool, each write op and query operation included: shared body, argument list and MCP note ([Served text](served-text.md)) |
| `kbase://help/query` | `text/plain` | always | the `query` tool's operations, each listed from the commands it serves with its one-line summary, then what they share |
| `kbase://help/write` | `text/plain` | always | `kbase write`'s own help, which has the batch contract in full |

- `resources/read` of a sheet returns the SVG as text. A KB that exists but will not open lists no sheet: the list is answered without one, and that KB's unopenability is reported by `status` and `verify` when they are asked, not as a list fault.
- A help resource is named `<command> help` (`query help` for the `query` tool's). The `query` tool's shared help states, as MCP's arguments name them, how a query reads the KB and answers; `kbase --help` states it as the command line names the arguments, and no operation's own help states it.
- A help resource's argument list is generated from the arguments the command's tool takes: a write op's is an entry's keys, `kbase write`'s the batch's. The command's `--help` has the same list, formatted as the command line formats it.
- Model guidance and help are KB-independent text: listed whether or not a KB is found or opens, and the same in every project.
- No resource is subscribable. `initialize` advertises the `resources` capability with no `subscribe` among the ones it has. Nothing announces a run's end, so a session polls `status`, or runs in the background the `wait` command `build` and `score` return. `resources/subscribe` and `resources/unsubscribe` are methods the server does not answer (`-32601`).
- An unknown URI is JSON-RPC `-32002` (resource not found); a missing `uri` is `-32602`.
- Nothing else is served, a summary page included: the KB's documents are the external agent harness's own files.

