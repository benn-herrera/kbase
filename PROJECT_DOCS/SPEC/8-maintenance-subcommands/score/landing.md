#### Landing

A value lands through the edit ops with the mark `false` ([page §3.5 `score`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)): a rigor answer as an
entry of its node kind's op — `edit-claim`, `edit-analysis` or `edit-experiment` — with the node's
`id`, a bearing answer as an `edit-edge` entry with the edge's `source`, `target` and `relation`,
and each records the judged hash of the text it was judged against ([page §2.9](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)). A value whose
item's text was edited after its call was composed lands nothing: its line is `failed`, reason
"its text was edited after it was asked; the value is discarded and a resume asks it again", and a
resume asks it again. A value whose node or edge is no longer in the KB lands nothing either, and
fails nothing: its line is `skipped`, reason "the node or edge is no longer in the KB; its value is
discarded and a resume does not ask it again", so it counts under `counts.skipped`, never under `failures`, and does not make the run
resumable. A resume that finds an item of its run removed records it the same way, once, and asks
nothing of it. The run lands
every answered value at once when its walk ends: one mixed write batch, followed by the
derivation as every batch is ([page §2.3](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)), under one hold of the KB write lock. An entry the
batch refuses is that item's `failed`, and the batch is re-issued without it; a landing that
cannot take the lock fails every item it carried: where another writer held the lock past the
wait, the item's line carries the lock item's `detail`; any other lock failure records its own
error. The run goes on.
