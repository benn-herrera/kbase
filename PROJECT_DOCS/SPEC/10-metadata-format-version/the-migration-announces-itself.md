### The migration announces itself

A verb that reads a KB through the loader on an older version
writes one sentence to stderr, and each warning on a line of its own, and carries one `migration`
record — `from`, `to`, `note`, `warnings` — after `kb-root` ([§7](../7-tool-result-contract-and-exit-codes/index.md)), on every such call until the
`update-format` whose save stamps `4.0.0`. `note` is, for every migration, "every scored record's judged hash is now
the unjudged one: no rigor or bearing is claimed current, and the next score run finds every one
dirty". [§7](../7-tool-result-contract-and-exit-codes/index.md) names the verbs. Over MCP the
document is the channel and stderr is the server's log; `status` and `cancel` load no KB and never
carry it. A `4.1.0` and a `5.0.0` KB are refused `check: kb-format`.

**The `demoted` relation**, added at `1.0.0`. An edge class only the build's cycle walk
writes, and only the build's analysis split ([§5](../5-build-behaviour/index.md)) moves: a
scoring edge the walk cuts to break a cycle is replaced by [page §1.3](../../../NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)'s `demoted` record, which
keeps it whole in its direction and stands outside the strength walk and the cycle check.
`replaced-relation` is `supports`, `lifts` or `refutes`: a `founds` source, a work or an axiom, has
no incoming scoring edge, so it closes no cycle. The build writes cuts through two
build-only write ops, `add-build-edges` and `demote-claim-edge`, and moves one onto the analysis it
splits from a claim through a third, `split-claim`. None is a subcommand or among the public ops,
so the public write surface cannot create one. `demote-claim-edge` replaces one standing scoring
edge, named by its `source`, `target` and `relation`, with its `demoted` record; where only that
record stands it leaves it as it stands, and an edge with neither is refused. Each cut is one
record, so more than one `demoted` record from one `source` to one `target` stands only where
separate cycles were cut at different relations between them. A record is resolved in one of
[page §1.3](../../../NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)'s three ways — `restore-demoted`, `remove-edge` with `relation: demoted`, or `add-edge`
of the replaced edge ([§8](../8-maintenance-subcommands/edge-ops.md)); the second leaves the edge cut. `verify` lists every `demoted` record under `findings`
([§7](../7-tool-result-contract-and-exit-codes/index.md)) with outcome `done`; `edges` reports it as a row of its own relation, out of its source;
`stats` counts them as `demoted_edges`, after `edges`, which counts every row; the sheet draws each
as a cut ([§3](../3-compatibility-contract/index.md)); and its target's register entry lists it.

