#### Edge ops

`add-edge`, `edit-edge`, `remove-edge` and `restore-demoted` look an edge's ends up in the batch's
records (`edgeEnds`, refusing through `noNode`, the "no node" refusal the edits share) and its
identity through `holds` and `noEdge`. `Joins` decides whether a relation may join two kinds:
`references` any, a scoring relation what `kb.BearingLegal` allows, `demoted` none. An `add-edge`
over a standing edge writes that edge's record in place, refused "edge exists" unless the entry moves
no byte ([§6 Adds](adds.md)). A `restore-demoted` entry no demoted record answers is refused "no
edge"; where a scoring edge from its `source` to its `target` stands, its remedy names that edge and
the `edges` call that reads it, in place of `noEdgeRemedy`'s.

**Pattern:** DAG insert with cycle check (path finder). `index.GraphDependsGraph` builds the premise
graph over the scoring edges, and `index.DependsGraph.Path` is a shortest-path search over it.
`closesCycle` runs the search for every public op that adds a scoring edge and for `restore-demoted`,
and refuses an edge it finds closing a cycle, naming the path in edge direction (it reverses the
premise graph's path). The premise graph is built once per batch and cleared by every op that takes a
scoring edge out of the records or changes one (`dropEdges`, `demoteEdge`, `splitClaim`). `addEdge` deletes the `demoted` record (`kb.DemotedMatch`)
of a scoring edge it writes, so an add over a removed edge writes the edge with the add's values.

A scoring edge's judgement is written by `add-edge` and `edit-edge` through `bearing`, and a node's
local value by `add-<kind>` and `edit-<kind>` through `rigor` (SPEC §8 Values input).

