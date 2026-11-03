#### The two passes

The passes, what neither reaches and the scope a run walks are [page §3.5 `score`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s. `rigor` walks
claims, analyses and experiments by id; `bearing` walks `supports`, `lifts` and `refutes` edges by
relation in that order, then source, then target. A run over `all`, the default, walks the node set
and then the edge set: a text edit moves a node's judged hash and the hash of every edge that
touches it ([page §2.9](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)), so a run scoped to one pass leaves the other dirty against that edit. An
all-pass run is one run: one lock, one `run.yaml`, one `items.jsonl`, one `ledger.jsonl` and one
`resume` call, and its `run.yaml` names the set it walks (`all`); a run naming a node records
`rigor`, one naming an edge `bearing`. The set is read from the index once, at the run's start, and
does not move as values land. A work's rigor is judged with the document in hand and `edit-work`
writes it; an axiom's is `1.0` by rule; a `founds` edge's bearing is the reader's, made with the
work in hand, and `edit-edge` writes it. The `founds` edge carries a bearing like any other — the
values table prices it `bearing × the source's rigor` — and no pass reaches it.
