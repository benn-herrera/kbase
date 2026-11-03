### Loading

Every KB file is read through `kb.Source`, built by `kbload.Open` (I8), so an older KB
arrives already in the current form and no verb sees the on-disk bytes of an unmigrated one. The
loader carries the conversion record: `Source.Migration()` holds the chain's `From`, `To`, the one
sentence `Note` (`kb.MigrationNote`) and the conversion's `Warnings`, set where the source was
migrated and kept across the save that stamps the current format, so every call on an older KB
until that save carries the `migration` record and writes its stderr lines — the sentence, then one
per warning (`cmd`'s `migrationNotice`; SPEC §7, [§10](../10-compatibility-harness.md)). `update-format` alone saves a migrated
source (`index.UpdateFormat`); every other writer — `write.Run`, `score`'s verb before its start
refresh, the build's resume (`loadFormat`) — refuses it through `index.PendingConversion` before it
writes, so no write ever lands on an unconverted tree (SPEC §10). `update-format`'s result lists the
warnings whose `List` is `ids`, `bare-ids`, `no-edge` or `strengthen-by` under keys of those names and
the rest under `migration`. `build` is the one loader that carries it
nowhere: a mid-build KB is not a finished state, and its stage commits own that trail.
`kb.MigratedSource` carries the paths the conversion made obsolete, and `update-format`'s save
removes them.

**The strict load** is `kb.LoadGraph`'s, and it sits at the load of every command that builds a
state: `format400.DecodeNodes` and `DecodeEdges` read both record files and return every
`format400.GraphFault{Check, Path, Line, Key, Allowed, Remedy, Detail}` — a file that does not
parse, an unknown or missing field (a scored record's `judged-hash` among them), a wrong type or a
value outside its domain, an id outside its
kind's grammar, a node id twice, a pairing `BearingLegal` refuses — before any state exists, so no
page is rendered and no op runs; an absent record file is a fault of its own (`check: index`, remedy
"restore it from git"). The faults arrive as one `kb.GraphLoadError`, which `index`'s refusal path
(`graphItems`, `internal/index/derive.go`) maps to `result` items — `check: metadata`, the field as
`key`, the file and line — and every reader of a
state, `write` included, refuses with them.

**The integrity faults refuse too** (SPEC §10, citing
[page §2.4](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)); there is no degraded
state. Where both files decode clean, `kb.LoadGraph`'s `edgeIntegrity` adds a fault per edge record
naming a node no record carries and per second record of one edge identity, each `check: metadata`
with `kb.ConsistentRecordsRemedy`. A cycle among the scoring edges is `index`'s: `deriveGraph` runs
`BreakCycles` on every load, and `Derive` refuses each edge the walk cut as a `check: metadata` item
naming it. The cycle stays out of `kb.LoadGraph` because the build's ring row reads, through it, the
records its own edge batch left, cycle included. `index.DeriveOverCycles` is the one read that takes
the cut records instead of refusing: `write` uses it for the two build-only ops that run inside that
window, `add-build-edges` and `demote-claim-edge` (`opSpec.overCycles`), and the records at every
stage commit hold no cycle.

**The rigor ceiling is checked at the same gate.** `kb.LoadGraph` refuses a node record whose
`rigor` stands above `kb.RigorMaximum`, a value the `format400` decoder takes because its rigor
domain has no top, as a `check: metadata` fault naming the record and the field (SPEC §10). A value
at the maximum is no special case: an authored `rigor: 1.0` is in domain, and the ceiling is the
whole of the scale; an axiom's `1.00` is `kb.AxiomRigor`'s by rule, the same number and not a
distinct read value.

