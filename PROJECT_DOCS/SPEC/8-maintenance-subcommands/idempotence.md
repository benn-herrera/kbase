### Idempotence

A batch's outcome, and each entry's, is [page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)'s. [Page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)'s
existence rule — an add of what stands refused, a removal of what does not refused, an edit to a
value that stands `unchanged`, and nothing adopted — is the rule of every write op: `add-<kind>`
([§8](add-kind.md)), `edit-<kind>` ([§8](edit-kind.md)), the edge writes ([§8](edge-ops.md)),
`restore-demoted` ([§8](restore-demoted.md)), `remove-node` ([§8](remove-node.md)), `move-node` ([§8](move-node.md)) and `add-citation` ([§8](add-citation.md)). A
re-issued add, removal, restore or citation is refused and a re-issued edit or move is `unchanged`, each leaving `kb-root/` byte-identical. There are
two exceptions. `init`'s creation is birth-once, and a re-issue where a `kb-root/` stands
is `refused`, never `unchanged`. `update-format` requires a clean `kb-root/`, so a re-issue
before its output is committed — by its own `commit` or by hand — is refused `dirty-paths`; over
a clean tree it already converted, it reports `unchanged`.
