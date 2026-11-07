#### `claims-declared`

**Pattern:** AST extraction of labelled environments.

`claimgraph.Declared` reads the blocks the document graph's records name ([§5.5](../../5-reader-and-transforms/5.5-records.md)) and decides which
are claims. It is deterministic and runs under `--no-inference`.

- **Blocks.** The reader records every author-declared environment as a `block` whose name is the
  Div's first class that is not one of `docgraph`'s `structural` classes. The pre-pass replaces a
  `\newtheorem` internal name by its display name ([§5.2](../../5-reader-and-transforms/5.2-pre-pass-registry.md); `theoremDeclaration` reads the starred,
  shared-counter and subordinate-counter forms, and a babel `\theoremname` macro gives its stem). A
  declaration whose display name is other markup gives no entry, and its blocks keep the internal
  name. A block is recorded only where its label line is read: the name matches `kb.LabelName` and the
  block sits at the page margin. An unread one is no block, and its facts host to the enclosing block.
- **The set.** `inventory.go` classifies the case-folded name.
  Claim-bearing: `theorem`, `lemma`, `proposition`, `corollary`, `conjecture`, `claim`, `result`.
  Not claim-bearing: `proof`, `remark`, `definition`, `example`, `assumption`, `notation`,
  `problem`. A name in neither is admitted as not claim-bearing and counted (`stage-B-unclassified`),
  never refused. The set is fixed in the code. A claim-bearing block is a claim only if its display
  line yields a title and a locator (`Block.ClaimBearing`).
- **Title and locator.** The display line is the first line of the block's content. Its title is the
  optional argument, `(Title)`, read with balanced parentheses, else the printed name and number
  with emphasis and stop removed. The span the title is read from is the locator. A span that another
  display line of the leaf contains grows a word at a time until none does (`distinct`); a block whose
  span cannot be made distinct is unreadable, counted (`stage-B-unreadable`) and yields no claim. The
  title is made unique among the KB's claims by a ` (N)` suffix. A title the leaf already hosts is the
  same claim: it takes that claim's id and writes nothing.
- **Statement and marker.** The claim record holds a title and a rationale, no statement; the host is
  where the marker sits (SPEC §10 Statements and anchors). The block is the statement. The marker goes
  on the one line of the leaf the locator begins on (`write.ExcerptLines`); a locator that names none
  or several stops the stage.
- **Equations.** Not minted here. `equations-minted` (`claimgraph.Equations`) mints a node for each
  labelled display-maths equation that a counting cross-reference names and that no claim-bearing
  block and no proof holds. A reference counts unless the discovery judged its paragraph not a claim.
  The title is ``Equation (`label`) — <the leaf's H1>``, and a leaf with no H1 stops the stage. The
  marker goes inside the fence (SPEC §5 A build marks every claim).
- **Skipped.** Every environment outside the claim-bearing set. A proof is bound to the claims it
  establishes (`bindProofs`), and a work cited inside a claim's block or its proof becomes a work
  node, joined to the claim by a `rests-on` premise written pending (`add-work`, `add-build-edges`;
  SPEC §5 A build records no judgement). A definition, remark or example is no claim and has no
  node.
- **Refusal.** A tree that already holds node records, frontmatter or a statement marker is
  refused (`checkCleanliness`): a second run would mint a second set of ids.
- **Records.** The stage writes the four build records fresh. The node-pass record lists every
  `leaf` document `unread`.

