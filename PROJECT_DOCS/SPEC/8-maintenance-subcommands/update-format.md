### `update-format`

[Page §3.6 `update-format`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md) is the op: run alone over a clean `kb-root/` and clean build records the
conversion moves (`check: dirty-paths`, naming each path), it converts a KB in an older format ([§10](../10-metadata-format-version/index.md)) and, whether or not the
format changed, re-derives every register page and claim-graph sheet ([§3](../3-compatibility-contract/index.md)) and rewrites the
maintained blocks ([page §1.7](../../../NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)). While a conversion is pending every other write is refused
([page §2.6](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)).

What this release adds to the page and holds:

- It writes each file only where its bytes change, and removes a page or a sheet the KB no longer
  calls for. Without `dot`, [§3](../3-compatibility-contract/index.md)'s rule.
- A KB whose load fails is refused before anything is written, and `update-format` repairs no load
  error ([page §2.4](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).
- Without `commit` (`--commit`) `commits` is `[]` and the `git mv` of each build record into
  `kb-root/.build/` stands staged. With it, a run that writes or removes anything makes [page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)'s
  one commit, labelled `kb-write: update-format <from> → <to>` (the format before and after, the
  same version twice where it did not change), holding `kb-root/` and each moved record's root
  path; an `unchanged` run commits nothing. A commit that fails leaves the conversion written,
  outcome `done`, with `uncommitted` saying why, as a write batch's does ([page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)).
- The move is made once the KB loads, before the first write, so a KB whose load fails moves no
  record. Each moved record is reported twice: its root path, repository-relative, in `removed`,
  and its new path, kb-root-relative, in `written`.
- `ids`, `bare-ids`, `no-edge` and `strengthen-by` are listed only on `done` and `unchanged`. On a
  refusal or a failure, their warnings stay in `migration.warnings` and on stderr.

Result keys: [§7](../7-tool-result-contract-and-exit-codes/index.md).
