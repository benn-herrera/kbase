### Writes leave nothing stale

Every write batch is followed by the derivation, inside its all-or-nothing ([page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)); no
flag skips it. A batch refuses, before it writes anything, a `kb-root/CLAUDE.md` that is not the
redirect (`check: agents-redirect`, [§3](../3-compatibility-contract/index.md)), which the derivation would refuse.
