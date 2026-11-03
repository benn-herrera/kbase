### The load: every fault refused

The load refuses a KB whose records hold any fault
[page §2.4](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md) lists, `check: metadata`,
naming each with its file and its line or record; the pairings a relation joins are the relations
table below. An absent record file in a KB stamped at the current version is refused
`check: index`, remedy "restore it from git". The three faults no write op produces — one edge
identity on two records, an edge naming a node no record carries, a cycle among the scoring edges —
carry the remedy "return the record files to a consistent commit in git, such as the one before a
merge, and reapply the other side's changes through write ops". No op resolves a load error.
