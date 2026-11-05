### Five node types

`claim`, `analysis`, `experiment`, `work` and `axiom`. `node_type`, `target_kind`, the derived
views, the sheet and the result documents use these five words and no other.

Every id is minted: its kind's three-letter prefix (`clm`, `ana`, `exp`, `wrk`, `axm`), a hyphen
and a minted body of six lowercase letters and digits, so a token names its kind without
translation. To *mint* is to generate a new unique id; every add mints its node's id, for every
kind. No other field is part of an id.

- `title` is one non-empty line. A title is unique within its kind across the KB; nodes of
  different kinds may share a title, as a claim and the analysis split from it often do.
- `label` is one non-empty line. A work's label is its citation key,
  `[A-Za-z0-9](?:[A-Za-z0-9_.:+/-]*[A-Za-z0-9])?`, which opens and closes alphanumeric. An axiom's
  label is any text, LaTeX included. A label is unique within its kind across the KB: a work's among
  works, an axiom's among axioms.
- A label is never an id, and no op takes one where it takes an id.
- The load refuses an empty title or label and a work's label outside the citation-key grammar. The
  write ops refuse a title or a label that another node of the kind holds (`node exists` on an add
  that is not identical to that node, naming it); the load does not check uniqueness. The build gives a repeated title the
  counter ` (2)`, ` (3)` and so on, so the KB it writes holds every title unique within its kind.
- A claim, an analysis and an experiment are hosted (Statements and anchors). An axiom is stated in
  `axioms.md` (Axioms). A work is shown by its entry on the root summary page.

Authoring prose may say *invariant*, *framework premise*, *framework*, *proposition*, *evidence*,
*trial*, *cited work* or *reference*: *invariant* and *framework premise* name an axiom,
*proposition* a claim, *evidence* and *trial* an experiment, *cited work* and *reference* a work;
*framework* names the document that declares axioms and is never a node type. The graph has one word
per role; the other words are the author's.

**A claim stands alone; an analysis is made of its citations.** A claim is a proposition about the
subject matter: its premises support it, and it means what it means without them. An analysis is
constituted by the nodes it cites — strip the ids from its sentence and no proposition remains — so
it has at least one premise and concludes in a claim. An analysis citing none is a method with no
inputs but its own, and reads its `rigor` rather than a claim's `0` (Four scoring relations). The
authoring test is that removal: if a proposition survives it is a claim, and if nothing survives it
is an analysis. A claim is the model's atom and needs neither: it may exist with no premises and
nothing resting on it. These are rules for authors, not faults: the loader enforces neither, and a
KB that breaks one is still read.

