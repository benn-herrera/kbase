### Citations

A citation is a Markdown link in prose that refers to a node and links to where the node is placed.
It takes no stance: the citing text may agree with the node, dispute it or mock it. Its link title
is the cited node's id. Three forms:

- quote: `["the bound is tight"](leaf.md#bounds "ana-012xyz")`;
- paraphrase: `[the bound holds](leaf.md#bounds "ana-012xyz")`, with no quotation marks;
- naming: `[The Bound Is Tight](leaf.md#bounds "ana-012xyz")`, whose link text is the node's title,
  for naming a node without quoting it.

A quote or a paraphrase cites a statement, so only a hosted node takes those forms. A work or an
axiom is cited in the naming form only. A citation is the only way prose refers to a node: a bare
node id — one standing in prose outside code, markers and every link, or a link's whole text — is
`verify`'s `bare-id` fault, whose remedy is `kbase update-format` ([§8](../8-maintenance-subcommands/verify.md)). An id that is only part of a
link's text is not bare: a link cannot nest inside a link, and that link already names a target. A link whose title is absent or is no node id
is an ordinary link. kbase reads every link, a citation's title included, through one link grammar.

The link path is relative to the citing document. The link target is, for a hosted node, its host
leaf and section anchor; for an axiom, the anchor of its marked heading in `axioms.md`; for a work,
its entry on the root summary page. The id lets kbase tell which node a citation means even after a
heading rename or a move. A citation names a node and nothing else.

`add-citation` checks a quote or paraphrase once, when it places it ([§8](../8-maintenance-subcommands/add-citation.md)). After
that, a citation is checked like every other link: `verify`'s `dead-link` check confirms that the
document exists and the anchor is in it. A quote is never checked again against later text; keeping
the words faithful as the cited text changes is the author's job.

A citation is not an edge. It does not appear in `edges`; `citations-of` lists citations.

A citation's target is not the node's `canonical_path` (below). For a claim the target is its host
leaf and section anchor, while its `canonical_path` is its summary entry.

