### `edit-<kind>`

`edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work` and `edit-axiom` are [page §3.1](../../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/edit-claim-edit-analysis-edit-experiment-edit-work-edit-axiom.md)'s
node edits: the fields each kind's edit may change, at least one given, a title change or an
axiom's label change keeping every link to the node live in the same write ([page §1.5](../../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)), a work's
label change rewriting every reference that carries the old key as a key, the result's `id` and
that label change's `rewritten`, and the refusals; `label` is checked as `add-work` and `add-axiom` check it ([§8](add-kind.md)). An edit
whose every value already stands is `unchanged` ([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)); the mark a `rigor` writes is
[page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)'s, and `title` and `rationale` take [page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s "Values" check. What this release adds to the
page and holds:

- An entry giving no field, and a field the kind does not carry, are each a shape refusal whose
  `allowed` is the fields the kind's edit may change, neither `id` nor `operation` among them.
- An `id` of another kind is refused at `id`, its remedy naming the edit op for that kind; an `id`
  no node carries is refused `check: unknown-id` at `id`, `detail` `no node <id>`; a `title`
  or a `label` another node of the kind carries is refused at that key, its `detail` naming that
  node; a work `label` a citation span or a reference-list entry already uses as a key is refused
  at `label`, one item per such line, each naming it by `path` and `line`.
- An edit giving a `rigor` — `*pending*` included — on a claim, an analysis or an experiment
  records the judged hash of the node's statement as the edit leaves it ([page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)); a scoring run's
  landing, which sets a number, records it the same way. An edit giving no `rigor` keeps the
  record's `judged-hash` as it stands.
- A link re-pointed is an inline link or a reference definition outside code, in any document but
  the summary pages, which the edit does not rewrite: the derivation renders them. Where the edit
  moves a summary entry's anchor — the node's own, or another entry's repeat suffix shifting with
  it — every link into that entry is re-pointed too. A citation of the node whose destination resolves to the node's
  link target before the change is pointed at the target after it, written relative to the citing
  document. Any other link stands.
- A work's label change reads citation spans, reference-list entries and links into them outside
  code, a blockquote's prefixes read past, in every document but the summary pages.
- A title under which a naming-form citation of the node would not read, whole, as one link — an
  unbalanced `[` or `]` — is refused at `title`, naming the citing `path:line`.
- `edit-axiom` rewrites every `axioms.md` heading carrying its marker, and appends its heading
  where none stands, creating `axioms.md` where the file is absent.
