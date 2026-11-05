### The record files

Each is one YAML document: a block list of block mappings, `[]` when it holds none. A node record's
type is its `id`'s prefix and an edge record's its `relation`; no record has a `kind` field. Every
field its type has is present and any other field is refused; an empty value is written as `""` or
`[]`, never omitted, so no reader takes an absent bearing for the judgement `0`. Strings are
double-quoted, escaping `\`, `"`, every control character below U+0020 but tab, and the characters a
YAML reader would take as a line break or refuse as unprintable, as `\uXXXX`; a string holds no line
break, and a stored prose value has each whitespace run collapsed to one space, its ends trimmed —
an op's `unchanged` test compares the collapsed value. Numbers are written in the shortest form that
parses back as the value (Python's float repr): a plain decimal with at least one fractional digit
(`1.0`, `0.85`), except below 1e-4, where it is exponent form (`5e-05`); booleans `true` and
`false`; keys in the type's field order. Nodes are written in `id` byte order and edges by identity —
`relation` in the order `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted`, a
`demoted` record's `replaced-relation` in the same order, then `source`, then `target`. Every
writer re-renders a whole file from the records, and writes it only where its bytes change.

| Node record | Id | Fields, in order |
|---|---|---|
| claim | `clm-<minted>` | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `judged-hash` |
| analysis | `ana-<minted>` | `id`, `title`, `rigor`, `manually-scored`, `rationale`, `judged-hash` |
| experiment | `exp-<minted>` | `id`, `title`, `status`, `rigor`, `manually-scored`, `judged-hash` |
| work | `wrk-<minted>` | `id`, `label`, `title`, `rigor`, `rationale` |
| axiom | `axm-<minted>` | `id`, `label`, `title` |

`title` and `label` follow Five node types. `rigor` is a number in `[0, 1]` or `"*pending*"`.
`manually-scored` is a boolean and `judged-hash` a digest (Provenance and the judged hash).
`rationale` is a string, `""` for none. `status` is `"run"` or `"planned"`: whether the experiment
has been run. No record names a summary page: the page that renders a node is derived from its
host.

| Edge record | Relations | Fields, in order |
|---|---|---|
| unscored | `references` | `relation`, `source`, `target`, `context` |
| demoted | `demoted` | `relation`, `source`, `target`, `context`, `replaced-relation`, `bearing`, `manually-scored` |
| bearing | `founds` | `relation`, `source`, `target`, `context`, `bearing` |
| scorable | `supports`, `lifts`, `refutes` | `relation`, `source`, `target`, `context`, `bearing`, `manually-scored`, `judged-hash` |

`context` is a one-line string, `""` for none, and a query reports it null. `bearing` is a number in
`[0, 1]` or `"*pending*"`. A scoring edge runs push, from the supporter to the supported; a
`references` edge runs from the node that names to the node named. An edge record's identity is
(`relation`, `source`, `target`), but a `demoted` record's is (`demoted`, `replaced-relation`,
`source`, `target`), so an analysis's `lifts` and `refutes` of one claim, both demoted, are two
records. Only one record holds an identity.

