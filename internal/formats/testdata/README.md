# formats testdata — converter and loader fixtures

A converter's test is a golden pair: old bytes in, new bytes out. `from20to30`'s pairs are these
(`internal/formats/from20to30/goldens_test.go`):

| Input | Output | What it pins |
|---|---|---|
| `register-2.0.0/` | `register-3.0.0/`, `register-3.0.0.conversion.yaml` | the register fixture's records, every document's frontmatter stripped — the entry point's stamp with it, the next save writing the stamp file — its markers from the `claims:` listings and the hosts, and its warnings: an edge authored twice with different bearings, register text dropped, an analysis's register heading replaced by its host's, markers appended to a first heading |
| `devpro-sheet-2.0.1/` | `devpro-sheet-3.0.0/`, `devpro-sheet-3.0.0.conversion.yaml` | the claim-graph sheet fixture's 2.0.1 form, every relation, both demoted records, a joined context and the 2.0.x index files made obsolete |
| `bad-token-2.0.0/` | `bad-token-2.0.0.faults.yaml` | a KB the 2.0.x load refuses is refused with that load's own faults; the golden is the one `format20`'s own test pins |

`from30to40`'s pairs (`internal/formats/from30to40/goldens_test.go`) read `from20to30`'s frozen
3.0.0 outputs as their inputs:

| Input | Output | What it pins |
|---|---|---|
| `register-3.0.0/`, `devpro-sheet-3.0.0/` | `register-4.0.0/`, `devpro-sheet-4.0.0/`, each with its `.conversion.yaml` | the records in the 4.0.0 form: no `no-edge` or `strengthen-by` field, an experiment's `status: pending` written `planned`, and `judged-hash` all zeros on every scored record; every work's and axiom's id minted from its 3.0.0 id alone, its key or label its `label`, every edge end naming it renamed, each axiom heading ending in its marker, and each mint listed under `ids`; every statement marker in the `kbase-id` form, attached with no space; each `claim-rigor.md` register page an obsolete path; a build record at the repository root, `devpro-sheet-3.0.0/`'s `kb-build-unmarked.yaml`, moved with its bytes into `kb-root/.build/`, its root path an obsolete path; every other file carried through as it stands. Neither input holds a `no-edge` or `strengthen-by` text, so neither conversion warns of one; the converter's own test pins those warnings. `register-3.0.0/`'s analysis markers stand below an experiment's, not directly below the claims they lift, so none moves |
| `markers-3.0.0/` | `markers-4.0.0/`, `markers-4.0.0.conversion.yaml` | the markers' one move: an analysis marker alone on the line after a heading ending in the marker of the claim it lifts moves onto that heading, abutting it; the near misses stay on their lines — an analysis below a claim it does not lift, one a blank line away, one quoted on a blockquote's blank line, an experiment below the claim it lifts; the analysis below a claim it does not lift stands alone on a list item's continuation line and keeps its indentation; a marker naming two ids becomes two abutting markers, and a marker-shaped comment naming no id stands. Written by hand as a 3.0.0 KB |
| `citations-3.0.0/` | `citations-4.0.0/`, `citations-4.0.0.conversion.yaml` | the citations: a link into a heading, its anchor the 3.0.0 slug with the marker's words, re-anchored to the 4.0.0 slug and titled with the id of the one node whose statement the section holds; an ordinary link into a re-slugged heading re-anchored, untitled; a link into a 3.0.0 register entry pointed at its node's host section, and a work's at its entry on the root `kb-summary.md`, titled with the work's minted id; an axiom's heading titled with its minted id, the heading ending in its marker; each bare id of a node, and a link whose text is one, made its naming citation, listed under `bare-ids`, a heading's included, the links into that heading anchored at the heading as it reads cited; a link into a heading carrying inline code, by the 3.0.0 slug of its code's words, re-anchored; reported under `unresolved-citations`: a register anchor two entries stated in one section share, pointed at that section, untitled, and a section holding two nodes' statements (re-anchored, untitled); and reported and left: an anchor naming one heading under 3.0.0's slugging and another under 4.0.0's, a link with a title of its own, a missing heading, an unhosted node's bare id, an id inside a link's text. An unknown id and ids in code stand. Written by hand as a 3.0.0 KB, its stamp file with it. A link into a register page whose anchor names no one place, and one into a page no summary page replaces, are pinned by `TestALinkIntoARemovedRegisterPageNeverStaysThere`, with a bare id in the text of a link that goes; the minting's determinism and collision order by `ids_test.go`'s tests |

The 2.0.x inputs are shared and frozen: `register-2.0.0/` and `bad-token-2.0.0/` (with
`bad-token-2.0.0.faults.yaml`) are also the 1.0.0 → 2.0.0 converter's outputs and the 2.0.x load's
inputs, read by `from10to20`, `format20` and `kbload` tests. Never edit them. A new 2.0.x case is a new
input tree beside them. `devpro-sheet-2.0.1/` is read here and by the 2.0.1 parity comparison
(`internal/kbload/parity_test.go`, run by `just test-integration-parity-devpro-sheet`), whose
frozen goldens `internal/kbload/testdata/parity-2.0.1/` the 2.0.1 binary recorded over these bytes.

A `.conversion.yaml` is the obsolete paths and the warnings the conversion returns beside its
files. A converter writes no stamp, so each output's entry point keeps its input's: the save
stamps it. Each output tree converts again to itself, with no obsolete path and no warning but, for a
4.0.0 output, each citation it still cannot resolve (`TestAConvertedKBConvertsToItself`): a save that stops before the stamp leaves 3.0.0 files under a 2.0.x
stamp, and the next load converts them. `TestAStoppedSaveIsTakenAsItStands` builds such a half-saved tree
from `register-2.0.0/` and `register-3.0.0/`.

`devpro-sheet-2.0.1/` is `internal/sheet/testdata/devpro-sheet/`'s `kb-root/` and
`kb-build-unmarked.yaml` at commit `60adcb3`, copied with one `git show 60adcb3:<path>` per file. It is a
copy of its own: the sheet fixture changes with the sheet's tests, and this converter's golden must not
move with them.

## Decoded values

`devpro-sheet-3.0.0.values.jsonl`, `citations-3.0.0.values.jsonl`, `markers-3.0.0.values.jsonl` and
`register-3.0.0.values.jsonl` are what the record files of the 3.0.0 trees beside them decode to: one JSON object per record, nodes by id then
edges by identity as the files keep them, each record's keys in its type's field order, a pending
value as `"*pending*"` (`internal/formats/format30/values_test.go`). A reader of the record files in
any language checks itself against them. The `*-4.0.0.values.jsonl` files are the same for the
4.0.0 trees, each `judged-hash` as its 64 hex
digits (`internal/formats/format40/values_test.go`). A decode that differs is written under
`test-data/transient/unit_tests/formats/got/` with the golden's name, and a golden is replaced only
by copying that file over it.

## Producing an output

Run `just test-pkg internal/formats/from30to40`, or `just test-pkg internal/formats/from20to30`
for the `2.0.x → 3.0.0` pairs. A conversion that differs from its golden is written under
`test-data/transient/unit_tests/formats/got/` with the golden's name (a `.conversion.yaml`
directly there). An output is replaced only by copying that file over it. The commit that does
so names each golden file it changes and the converter change that caused it. A superseded
format's packages are frozen at the version they shipped; the live format's packages change, patch
bumps included (CONVENTIONS House Rules): `from10to20` and `from20to30` are frozen; `from30to40` is
not, since it converts into the live format and changes with it. A differing
conversion from a frozen converter is a converter defect, never a golden to replace.

`register-1.0.0/` and `bad-token-1.0.0/` are the 1.0.0 → 2.0.0 converter's inputs, and
`register-2.0.0.state.yaml` the 2.0.x load's golden over `register-2.0.0/`. `1.0.0/` is the older
KB `cmd/format_test.go` opens.
