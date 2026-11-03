### Provenance

`write.Options.Provenance` says who supplied the values a call writes, and decides
the `manually-scored` flag each rigor and bearing is written with (SPEC §8): `ProvenanceCaller`, the
zero value, marks every value its caller supplied manual, so an internal caller that names none
never lets a run overwrite a person's value; `ProvenanceMachine` writes `false`. `claimgraph`'s
write runner, `internal/score`'s landing (`landValues`, which writes every value as `ProvenanceMachine`, a landing and a re-price alike) and the build-only
`add-build-edges` and `split-claim` (each a `recordEdit` marked `machine`) write as the machine. It
is an options field and never a values key, a flag or a schema property:
`TestNoSurfaceNamesTheProvenanceOrCreate` (`cmd/mcp_test.go`) asserts no command flag and no tool
property names it. No build path writes a judgement: `add-build-edges` writes `*pending*` on every
edge it adds and no number anywhere, and `split-claim` writes `*pending*` on the analysis's rigor
and its lift, so a claim a build attaches an edge to reads unchanged until that edge is scored.

Input, idempotence (I6) and the derivation after a batch are SPEC §8; byte equality is
SPEC §3. Three ops are build-only:
`add-build-edges` (`write.AddBuildEdges`), which writes the build's edges as `add-edge` writes one, without its cycle
refusal; `demote-claim-edge` (`write.DemoteClaimEdge`), which replaces one standing scoring edge,
named by its `source`, `target` and `relation`, with the `kb.DemotedEdge` that keeps it whole; and
`split-claim` (`write.SplitClaim`). Per claim, `split-claim` mints an analysis and appends its
marker to a given line of a given leaf, and where given a `statement-end` moves the claim's marker
to that byte offset of line `statement-line`, or of its own line where none is given. It adopts
instead the analysis already lifting the claim. It moves every `supports` and `founds` record into
the claim, and every `demoted` record that replaced a `supports` edge into it, onto that analysis,
and adds the analysis's `lifts` of the claim at `*pending*`. All three are absent from
`write.Ops()` and have no subcommand, so the public write surface cannot create or move a `demoted`
record. The first two run over records holding a cycle among the scoring edges, which the build's
edge batch closes and its ring row demotes within one stage (`index.DeriveOverCycles`). The build's cuts reach the records through
`demote-claim-edge`, from the ring row at the tail of the dependency-attribution stage. The
analysis split reaches them through `split-claim`, from the `analyses-split` stage.
`restore-demoted` is a public op, and `remove-edge` removes a `demoted` record.
`index.DependsGraph.Path` is the path finder over the scoring edges (`index.GraphDependsGraph`);
every public op that adds a scoring edge, through the batch's `closesCycle`, and `restore-demoted`
refuse an edge it finds closing a cycle, naming the path in edge direction: `closesCycle` reverses
the premise graph's path. `addEdge` deletes the `demoted` record (`kb.DemotedMatch`)
of a scoring edge it writes, so an add over a cut writes the edge with the add's values.

