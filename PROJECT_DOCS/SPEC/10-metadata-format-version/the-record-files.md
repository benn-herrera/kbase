### The record files

Each is one YAML document: a block list of block mappings, `[]` when it holds
none. A node record's type is its `id`'s prefix and an edge record's its `relation`; no record
carries a `kind`. Every field its type has is present and any other field is refused; an empty
value is spelled `""` or `[]`, never omitted, so no reader takes an absent bearing for the judgement
`0`. Strings are double-quoted, escaping `\`, `"`, every control character below U+0020 but tab, and
the characters a YAML reader would take as a line break or refuse as unprintable, as `\uXXXX`; a
string holds no line break, and a prose value collapses each whitespace run to one space, its ends
trimmed — adoption and an op's re-assertion test compare the collapsed value.
Numbers are plain, in the spelling a rigor is written in; booleans `true` and `false`; keys in the
type's field order. Nodes are written in `id` byte order and edges by identity — relation in the
order of the relations table below, a `demoted` record's `replaced-relation` in the same order, then
`source`, then `target`. Every writer re-renders a whole file from the records, and writes it only
where its bytes change.

A node record's fields, in order, are its kind's in
[page §1.1](../../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)'s record-fields table.

A `title` is a non-empty line ([page §1.1](../../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)); two entries
of one register page sharing a title take the anchors GitHub's slugger gives repeated headings
([page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) "Anchors"). `rigor` is a number in `[0, 1]` or `"*pending*"`. `manually-scored` is a boolean
([§8](../8-maintenance-subcommands/index.md)). `rationale` is a string, `""` for none; `status` `"run"` or `"planned"`. `judged-hash` is a
SHA-256 digest as 64 lowercase hex digits, all zeros on a value never judged
([page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)). No record
names a register: the page that renders a node is derived from its host.

| Edge record | Relations | Fields, in order |
|---|---|---|
| unscored | `references` | `relation`, `source`, `target`, `context` |
| demoted | `demoted` | `relation`, `source`, `target`, `context`, `replaced-relation`, `bearing`, `manually-scored` |
| bearing | `founds` | `relation`, `source`, `target`, `context`, `bearing` |
| scorable | `supports`, `lifts`, `refutes` | `relation`, `source`, `target`, `context`, `bearing`, `manually-scored`, `judged-hash` |

`context` is `""` for none, and a query reports it null. `bearing` is a number in `[0, 1]` or
`"*pending*"`. A scoring edge runs push, from the supporter to the supported; a `references` edge
runs from the node that names to the node named. An edge record's identity is (`relation`,
`source`, `target`), but a `demoted` record's is (`demoted`, `replaced-relation`, `source`,
`target`), so an analysis's `lifts` and `refutes` of one claim, both demoted, are two records.

**Statement markers** are [page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)'s: the marker
`<!-- kbase-id: <id> -->`, attached with no space after its line's last character, a paired
analysis's abutting its claim's; the reading of each node's statement, the two layouts the build's
analysis split writes among them; the `statement-marker` fault; and the anchors. A host is derived
at load, and no record stores hosting. The build's analysis split ([§5](../5-build-behaviour/index.md)) places both of the split's
layouts, through its build-only `split-claim`. No public write op places the blockquote layout;
the add ops and `move-node`, which place a marker at the end of the line `document`, `line` and
`snippet` name, can place the paragraph layout, and a marker placed into either layout reads by
it ([§8](../8-maintenance-subcommands/add-kind.md), [§8](../8-maintenance-subcommands/move-node.md)). A `line` naming a
fenced block's opening or closing line (a display-maths fence included) is refused, naming the
line.

**Citations** are [page §1.5](../../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)'s: the quote,
paraphrase and naming forms, each a Markdown link whose title is the cited node's id; the link
target per kind; and a bare node id in prose, `verify`'s `bare-id` fault ([§8](../8-maintenance-subcommands/index.md)). A link whose title is
absent or is no node id is an ordinary link. kbase reads every link, a citation's title included,
through one link grammar.

**Register pages** are [page §1.6](../../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md)'s summary
pages, `kb-summary.md` at kb-root and in each volume ([§3](../3-compatibility-contract/index.md)): which page a node's entry stands on, the
title, the one-line preamble, each entry's `## <title><!-- kbase-id: <id> -->` heading, what an entry
shows, the entry order and a page's removal. An entry's anchor is its heading's among every heading
its page renders ([page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) "Anchors"), and every link
a page renders is computed from each node's derived page and anchor. A page no longer called for is
reported under `removed`. A prose link `<page>#<anchor>`
holds while its node stays on that page with that anchor, and the dead-link gate reports one that no
longer does ([§8](../8-maintenance-subcommands/index.md)). A page is never read back: `verify` renders it in memory and compares bytes, and a
difference is `register-freshness`, which an empty write batch fixes ([page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)). Beyond the anchors and the freshness
compare, what an entry shows is presentation and untested: today an entry carries the leaves citing
it, a `### Rigor` section and, on a claim or an analysis, a `### Strength` section.

