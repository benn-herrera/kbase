## 3. Operations

The surfaces:
- **write op**: an entry of a write batch. Over MCP, an op of the `write` tool. At the CLI, its own
  subcommand, or an entry of a `kbase write` batch ([§2.2](../2-conventions/2.2-write-batches.md)).
- **query op**: over MCP, an op of the `query` tool, taking `operation` and `arguments`. At the
  CLI, its own subcommand.
- **tool + CLI**: an MCP tool of its own and a CLI subcommand, with the same arguments.
- **CLI only**.

Every MCP tool also takes `project-dir` ([§4.3](../4-served-text/4.3-server-modes.md)). Every CLI command takes the global flags
`--project-dir`, `--config-dir`, `--log-file` and `--log-level` (debug|info|warn|error, default
warn). Each op's help is described in [§4.2](../4-served-text/4.2-tool-descriptions-and-help.md).

| Family | Ops | Count |
|---|---|---|
| Node writes | `add-claim`, `add-analysis`, `add-experiment`, `add-work`, `add-axiom`, `edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work`, `edit-axiom`, `remove-node`, `move-node` | 12 |
| Edge writes | `add-edge`, `edit-edge`, `remove-edge`, `restore-demoted` | 4 |
| Citations | `add-citation`, `citations-of` | 2 |
| Queries | `find`, `show`, `edges`, `trace`, `stats` | 5 |
| Runs and run control | `build`, `score`, `status`, `cancel`, `wait`, `build-trail` | 6 |
| Checks and output | `verify`, `update-format`, `export-latex` | 3 |
| Setup and configuration | `init`, `configure-providers`, `configure-models`, `models`, `show-config`, `probe`, `auth add`, `auth list`, `auth remove` | 9 |
| Server | `mcp` | 1 |
| **Total** | | **42** |

MCP tools (16): `write`, `query`, `build`, `score`, `status`, `cancel`, `build-trail`, `verify`,
`update-format`, `export-latex`, `init`, `configure-providers`, `configure-models`, `models`,
`show-config`, `probe`.

The read-only tools are `query`, `status`, `build-trail`, `verify`, `models`, `show-config` and
`probe`. Each carries the read-only annotation.

Groups:

- [§3.1 Node writes](3.1-node-writes/index.md)
- [§3.2 Edge writes](3.2-edge-writes/index.md)
- [§3.3 Citations](3.3-citations/index.md)
- [§3.4 Queries](3.4-queries/index.md)
- [§3.5 Runs and run control](3.5-runs-and-run-control/index.md)
- [§3.6 Checks and output](3.6-checks-and-output/index.md)
- [§3.7 Setup and configuration](3.7-setup-and-configuration/index.md)
- [§3.8 The server](3.8-server/index.md)
