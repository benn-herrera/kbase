# formats testdata — converter and loader fixtures

A converter's test is a golden pair: old bytes in, new bytes out. `from200to300`'s pairs are these
(`internal/formats/from200to300/goldens_test.go`):

| Input | Output | What it pins |
|---|---|---|
| `register-2.0.0/` | `register-3.0.0/`, `register-3.0.0.conversion.yaml` | the register fixture's records, every document's frontmatter stripped — the entry point's stamp with it, the next save writing the stamp file — its markers from the `claims:` listings and the hosts, and its warnings: an edge authored twice with different bearings, register text dropped, an analysis's register heading replaced by its host's, markers appended to a first heading |
| `adjagent-sheet-2.0.1/` | `adjagent-sheet-3.0.0/`, `adjagent-sheet-3.0.0.conversion.yaml` | the claim-graph sheet fixture's 2.0.1 form, every relation, both demoted records, a joined context and the 2.0.x index files made obsolete |
| `bad-token-2.0.0/` | `bad-token-2.0.0.faults.yaml` | a KB the 2.0.x load refuses is refused with that load's own faults; the golden is the one `format200`'s own test pins |

The 2.0.x inputs are shared and frozen: `register-2.0.0/` and `bad-token-2.0.0/` (with
`bad-token-2.0.0.faults.yaml`) are also the 1.0.0 → 2.0.0 converter's outputs and the 2.0.x load's
inputs, read by `from100to200`, `format200` and `kbload` tests. Never edit them. A new 2.0.x case is a new
input tree beside them. `adjagent-sheet-2.0.1/` is read here and by the 2.0.1 parity comparison
(`internal/kbload/parity_test.go`, run by `just test-integration-parity-adjagent-sheet`), whose
frozen goldens `internal/kbload/testdata/parity-2.0.1/` the 2.0.1 binary recorded over these bytes.

A `.conversion.yaml` is the obsolete paths and the warnings the conversion returns beside its
files. A converter writes no stamp, so each output's entry point keeps its input's: the save
stamps it. Each output tree converts again to itself, with no obsolete path and no warning
(`TestAConvertedKBConvertsToItself`): a save that stops before the stamp leaves 3.0.0 files under a 2.0.x
stamp, and the next load converts them. `TestAStoppedSaveIsTakenAsItStands` builds such a half-saved tree
from `register-2.0.0/` and `register-3.0.0/`.

`adjagent-sheet-2.0.1/` is `internal/sheet/testdata/adjagent-sheet/`'s `kb-root/` and
`kb-build-unmarked.yaml` at commit `60adcb3`, copied with one `git show 60adcb3:<path>` per file. It is a
copy of its own: the sheet fixture changes with the sheet's tests, and this converter's golden must not
move with them.

## Decoded values

`adjagent-sheet-3.0.0.values.jsonl` and `register-3.0.0.values.jsonl` are what the record files of
`adjagent-sheet-3.0.0/` and `register-3.0.0/` decode to: one JSON object per record, nodes by id then
edges by identity as the files keep them, each record's keys in its type's field order, a pending
value as `"*pending*"` (`internal/formats/format300/values_test.go`). A reader of the record files in
any language checks itself against them. A decode that differs is written under
`test-data/transient/unit_tests/formats/got/` with the golden's name, and a golden is replaced only
by copying that file over it.

## Producing an output

Run `just test-pkg internal/formats/from200to300`. A conversion that differs from its golden is written
under `test-data/transient/unit_tests/formats/got/` with the golden's name (a `.conversion.yaml` directly
there). An output is replaced only by copying that file over it. The commit that does so names each
golden file it changes and the converter change that caused it. After a kbase release ships 3.0.0, the
converter is frozen (CONVENTIONS House Rules): a differing conversion is a converter defect, never a
golden to replace.

`register-1.0.0/` and `bad-token-1.0.0/` are the 1.0.0 → 2.0.0 converter's inputs, and
`register-2.0.0.state.yaml` the 2.0.x load's golden over `register-2.0.0/`. `1.0.0/` is the older
KB `cmd/format_test.go` opens.
