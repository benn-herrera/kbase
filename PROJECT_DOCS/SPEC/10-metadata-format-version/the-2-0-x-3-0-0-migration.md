### The `2.0.x → 3.0.0` migration

The `3.0.0` form moves the graph into records. Every node and every edge is a record in one of two
authored files, `.index/nodes.yaml` and `.index/edges.yaml`; a statement marker in a leaf hosts each
claim, analysis and experiment; the register pages are rendered from the records and never read
back; and no document has frontmatter at all, the stamp being in `.index/format.yaml`. Everything
else — what the queries, the sheets and `score` read — is derived at load and never stored:
strength, derivation strength, build status and band, the premise and citation counts, a scoring
edge's `target_kind`, the citations, subtree aggregates, each document's kind (Documents of a KB),
each node's host, canonical path and anchor. The scoring vocabulary of `2.0.0` — five node types,
three quantities, four scoring relations, one arithmetic — is unchanged.

One converter reads a `2.0.x` KB's files as bytes and writes the `3.0.0` form's: the two record
files from what the `2.0.x` load reads, every document stripped of its frontmatter — the entry
point's `kb-format` with it, the next save writing the stamp file — and a statement marker for each
node. It refuses only what the `2.0.x` load refuses, with that load's own items, and the owner
repairs such a KB as a `2.0.x` KB; otherwise it never refuses, but takes the outcomes below and
warns — one item each, `path`, `line` and `detail` — wherever it drops data or invents it.

- A claim's and a work's register entry becomes its node record — `title` its heading, `rigor`,
  `rationale`, `no-edge`, `strengthen-by` — and which `2.0.x` register held it is dropped silently:
  its page is determined by its host (Documents of a KB), so a node printed in another volume's register moves,
  and a prose link to its old anchor is reported by `verify`'s `dead-link` check. A second register
  entry for one id is dropped with a warning.
- An analysis is its leaf's `analysis-nodes` entry joined with its register entry, titled with the
  heading the register entry is written under — the author's title for it — and where the register
  has no entry for it, with its host leaf's first heading. An analysis entry no leaf hosts, which
  `2.0.x` never read as a node, is dropped with a warning. Its canonical anchor is the section its
  statement marker is in, wherever the marker is (below), so a title and an anchor differing is no
  loss: the anchor names the place the reader lands on. An experiment's `experiment-nodes` entry
  becomes its record, titled with its host's first heading and its `rigor` `*pending*` where it had
  none. A host with no heading titles its node with its file name, with a warning. An axiom is
  read from its heading in `axioms.md`, else from a legacy declaration, as the `2.0.x` load read it.
- Every `rests-on`, `refutes`, `references` bullet, register analysis pair and leaf pair line
  becomes an edge record oriented as the `2.0.x` index row, `context` `""` where it had none. A
  bullet whose token named no node, which `2.0.x` dropped too, is dropped with a warning quoting it.
  An edge authored more than once — a `rests-on` target repeated, an analysis pair in both the
  register and the leaf, an experiment's `supports` pair beside a `rests-on` bullet naming it — is
  one record: equal bearings silently, their contexts joined with `; ` in authored order; different
  bearings keep the higher, a number above `*pending*`, with a warning naming both sites.
- A `demoted` bullet on an entry X naming Y becomes a `demoted` record from Y into X, replacing
  `supports`, its bearing `*pending*` — `2.0.x` kept no bearing when it removed the edge, so none is lost — and its
  origin dropped silently.
- Every `manually-scored` is written `false`.
- Each node's statement marker is placed from the `2.0.x` `claims:` listings and markers. A claim
  marked in a leaf that lists it keeps that marker. A claim listed but marked nowhere it is listed
  gets a marker in the first listing leaf in path order — an equation's on the line inside its
  display-maths fence holding its `\label{…}`, and a claim whose leaf lists no other non-equation
  claim and holds one labelled blockquote on that blockquote's first content line, so its label line
  still parses as one, silently; any other on the leaf's first heading line, with a warning, the
  section that heading opens being its statement. A leaf with no heading gets none of the markers
  its heading would hold, each with a warning. Every other listing leaf becomes a mention, with a
  warning, its `referenced-by` row lost, and every other marker of the claim is removed, with a
  warning. A claim listed nowhere keeps its first marker in path order and loses the others, each
  with a warning; one listed and marked nowhere has no host, `verify`'s `statement-marker` fault. An
  analysis's or an experiment's marker that a `2.0.x` document already contains remains where it is —
  the section it sits in is the node's statement, so one the `1.0.0` migration placed beside the
  claim it argues for keeps that place — and a second marker for the same node is dropped with a
  warning naming the one that remains; a marker in a leaf other than the one declaring the node is
  warned of, naming both paths, and the statement is read from its marker's leaf. Where none exists, one
  is written on a new line directly after the host's first heading.
- Frontmatter is removed from every document, the entry point's with it. `kb-format`, `kind`,
  `claims`, a string `no-claim` and the `subtree-*` keys are dropped silently, markers and the
  values computed at load recording their facts; `experiments`, `path-stable`, a key no `2.0.x` reader knew, an
  `experiment-nodes` or `analysis-nodes` list on a document that is not `kind: leaf` — which
  declared no node to the `2.0.x` load — and a value of the wrong type are dropped with a warning
  naming file, line and key.
- In a `2.0.x` register, text outside every entry is dropped silently, and text inside an entry that
  is neither a field it read nor a derived line is dropped with a warning quoting it. The registers
  are left as bytes: the save renders the pages the KB calls for (Documents of a KB) over them, and
  removes a `2.0.x` register at a page position no page takes, reported under `removed`.
- The derived `.index/edges.yaml` is replaced by the record file, and every other file under
  `.index/` but the two record files and the stamp file — `2.0.x`'s `claims.yaml`, `cites.yaml`,
  `strengthen-by.yaml` and `subtree-aggregates.yaml` — is obsolete, reported under `removed`. The
  stamp file is kept, holding the version the tree is still at, until the save's last
  write replaces it. The stamped `AGENTS.md` and `CONVENTIONS.md` are not read by the conversion:
  `update-format` rewrites the block each contains between its marker lines, and leaves one with no
  such pair untouched, listing it under `unfenced` ([§3](../3-compatibility-contract/stamped-documents.md)).

