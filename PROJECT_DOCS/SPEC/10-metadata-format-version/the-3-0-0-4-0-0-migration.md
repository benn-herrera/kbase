### The `3.0.0 → 4.0.0` migration

One converter reads a `3.0.0` KB's record files through the
`3.0.0` load and writes them in the `4.0.0` form, doing the records' steps of
[page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)'s list: an
experiment's `status: pending` written `planned`; each `no-edge` and `strengthen-by` field dropped,
one warning per field naming the node's id and quoting its text (a claim's `strengthen-by` items
joined with `; `); `judged-hash` written all zeros on every scored record, `manually-scored`
kept; and each work's and axiom's id minted from its `3.0.0` id alone, the same id on every load
and in `update-format`, its key or label written as its `label` and every `3.0.0` work or axiom id
in an edge's end or a prose field renamed to the minted id, one warning per id listed `ids` with
the `3.0.0` id and the minted one. A bare id is read in the `3.0.0` grammar and cited under its
minted id, and each `### Axiom <label>:` heading of `axioms.md` naming an axiom record ends in that
axiom's marker, one naming none reported and left as written. It does the markers' steps of that list: every statement marker in a kb-root document
rewritten in the `kbase-id` form, attached with no space, a marker naming two ids written as two
abutting markers, and an analysis marker standing alone on the line directly after a line ending in
the marker of a claim it lifts moved to directly after that claim's marker, its emptied line removed.
The `3.0.0` register pages, `claim-rigor.md` at kb-root and directly inside each of its
directories, are obsolete and reported under `removed`; the save renders the summary pages. It does
the citations' steps of that list, resolving every anchor under `3.0.0` slugging: every link into a
heading, a citation or an ordinary link, rewritten to the heading's `4.0.0` anchor; the node id
added as the title of each link whose anchor resolves to one node's statement — a heading's section
holding that node's statement marker, an axiom's heading, or a `3.0.0` register entry, whose link
is pointed at the node's place — and a work's citation pointed at its entry on the root summary
page; and each bare node id in prose, and each link whose text is one, made the node's citation in
the naming form, one warning per conversion listed `bare-ids` with its path, line and id. Each link
or bare id it cannot resolve to one node without guessing — a section holding several nodes'
statements, a register anchor several entries share, a heading the anchor names under neither
slugging or names one heading under `3.0.0`'s slugging and another under `4.0.0`'s, a node with no
place to point at — takes no id, one warning listed `unresolved-citations`. A heading's `4.0.0`
anchor is read after its own bare ids become citations. A link into a removed register page never
stays aimed at it: where every entry its anchor names is stated in one section it points at that
section; otherwise at the summary page replacing the register page, without the anchor; and where
no summary page replaces it, the link is removed and its text stands, a bare id in that text
converted as any bare id is. Every other file is carried
as it stands. A `3.0.0` KB whose record files the `3.0.0` load refuses is
refused with that load's own items, `check: metadata`, one per fault.

