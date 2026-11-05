### Loading

**Pattern:** Schema validation at load, fail-closed.

Every KB file is read through `kb.Source`, built by `kbload.Open` (I8). `Open` constructs a new
`Source` on each call and no package-level state passes bytes from one load to the next. An older KB
arrives already in the current form, and no verb sees the on-disk bytes of an unmigrated one.
`cmd/readpolicy_test.go` enforces the single read path ([§11](dependency-rules.md)).

The loader keeps the conversion record. `Source.Migration()` contains the chain's `From`, `To`, the
one-sentence `Note` (`kb.MigrationNote`) and the conversion's `Warnings`. The record is set where the
source was migrated and kept across the save that stamps the current format. Every call on an older
KB until that save therefore returns the `migration` record and writes its stderr lines: the
sentence, then one per warning (`migrationNotice` in `cmd/update-format.go`;
SPEC §10 The migration announces itself). `build` is the one loader that returns it nowhere: a
mid-build KB is not a finished state, and its checkpoint commits own that history.

`update-format` alone saves a migrated source (`index.UpdateFormat`). Every other writer refuses it
through `index.PendingConversion` before it writes: `write.Run`, `score`'s verb before its start
rebuild of the derived views, and the build's resume (`loadFormat`). No write lands on an unconverted
tree. The conversion's result is formatted in [§12](../12-constants/format-migration-spec-10/index.md).
`kb.MigratedSource` carries the paths the conversion made obsolete, and the save removes them.

**The strict load** is `kb.LoadGraph`'s, at the load of every command that builds a state.
`format400.DecodeNodes` and `DecodeEdges` read both record files and return every
`format400.GraphFault{Check, Path, Line, Key, Allowed, Remedy, Detail}`, before any state exists:

- a file that does not parse;
- an unknown or missing field, a scored record's `judged-hash` among them;
- a wrong type or a value outside its domain;
- an id outside its kind's grammar, or a node id twice;
- a pairing `BearingLegal` refuses.

No summary page is rendered and no op runs. An absent record file is a fault of its own
(`check: index`, remedy "restore it from git"). The faults arrive as one `kb.GraphLoadError`, which
`index`'s refusal path (`graphItems`, `internal/index/derive.go`) maps to `result` items: `check:
metadata`, the field as `key`, the file and line. Every reader of a state, `write` included, refuses
with them.

**The integrity faults refuse too** (SPEC §10 The load: every fault refused); there is no degraded
state. Where both files decode clean, `kb.LoadGraph`'s `edgeIntegrity` adds a fault per edge record
naming a node no record has and per second record of one edge identity, each `check: metadata` with
`kb.ConsistentRecordsRemedy`. A cycle among the scoring edges is `index`'s: `deriveGraph` runs
`BreakCycles` on every load ([§6](../6-kb-model-and-maintenance/cycle-breaking.md)), and `Derive` refuses each edge the cycle
breaking removed as a `check: metadata` item naming it. The cycle stays out of `kb.LoadGraph`
because the build's cycle-breaking stage reads, through it, the records its own edge batch left,
cycle included. `index.DeriveOverCycles` is the one read that takes the removed records instead of
refusing. `write` uses it for the two build-only ops that run inside that window, `add-build-edges`
and `demote-claim-edge` (`opSpec.overCycles`), and the records at every checkpoint commit hold no
cycle.

**The rigor ceiling is checked at the same load.** `kb.LoadGraph` refuses a node record whose
`rigor` is above `kb.RigorMaximum` as a `check: metadata` fault naming the record and the field. The
`format400` decoder takes such a value because its rigor domain has no top. A value at the maximum
is no special case: an authored `rigor: 1.0` is in domain, and an axiom's `1.00` is `kb.AxiomRigor`'s
by rule, the same number and not a distinct read value.

