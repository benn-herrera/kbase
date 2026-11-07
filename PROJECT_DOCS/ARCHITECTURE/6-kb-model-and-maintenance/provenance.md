### Provenance

**Pattern:** Manual-override flag.

`write.Options.Provenance` says who supplied the values a call writes and decides the `manually-scored`
flag each rigor and bearing is written with (SPEC §10 Provenance and the judged hash). `ProvenanceCaller`,
the zero value, sets the flag on every value its caller supplied, so an internal caller that names
none never lets a run overwrite a person's value. `ProvenanceMachine` writes `false`. The writers that use
it are `claimgraph`'s write runner, `internal/score`'s writing of results (`landValues`, a score run
and a recompute alike; [§6](scoring/writing-the-results.md)) and the build-only `add-build-edges` and
`split-claim`, each a `recordEdit` marked `machine`.

Provenance is an options field and never a values key, a flag or a schema property:
`TestNoSurfaceNamesTheProvenanceOrCreate` (`cmd/mcp_test.go`) asserts that no command flag and no
tool property names it.

A rigor or bearing that an add or an edit is given records the judged hash of the text as the entry
leaves it (`valueSet`, `hashValuesSet`, over `asks.NodeJudgedHash` and `asks.EdgeJudgedHash`). A
pending value set by a machine records none.

No build path writes a judgement (SPEC §5 A build records no judgement): `add-build-edges` writes
`*pending*` on every edge it adds and no number anywhere, and `split-claim` writes `*pending*` on the
analysis's rigor and on its lift.

