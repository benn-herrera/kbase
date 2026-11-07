### The migration announces itself

A command that reads a KB in an older format writes one sentence to stderr, and each warning on a
line of its own, and includes one `migration` record — `from`, `to`, `note`, `warnings` — after
`kb-root` ([§7](../7-tool-result-contract-and-exit-codes/index.md)), on every such call until the `update-format` whose save stamps the current version
([§10 The stamp](the-stamp.md)). `note` is,
for every migration, "every scored record's judged hash is now the unjudged one: no rigor or bearing
is claimed current, and the next score run finds every one dirty". [§7](../7-tool-result-contract-and-exit-codes/index.md) names the commands. Over MCP
the result is the channel and stderr is the server's log; `status` and `cancel` load no KB and
never include it.

