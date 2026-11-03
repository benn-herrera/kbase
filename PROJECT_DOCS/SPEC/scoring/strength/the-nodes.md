## The nodes

| node | id | local value | derived |
|---|---|---|---|
| `claim` | `clm-xxxxxx` | `rigor` (its statement's own quality) | `strength` derived |
| `analysis` | `ana-xxxxxx` | `rigor` (its method's own quality) | `strength` derived |
| `work` | `wrk-xxxxxx` | `rigor` (the paper's soundness) | none — `rigor` is the source strength |
| `experiment` | `exp-xxxxxx` | `rigor` (the method's soundness) | none — `rigor` is the source strength |
| `axiom` | `axm-xxxxxx` | `rigor` `1.00` by rule — no authored file carries it | none — `rigor` is the source strength |

**A claim is, by definition, a statement shorn of supporting argument.** Its `rigor` is therefore the
quality of the assertion its text makes — fixed, consistent, single-sensed — never the quality of an
argument, which a claim does not display. Supporting argument is separated from the claim and stands
in an analysis or an experiment, reaching the claim through an edge; an analysis's `rigor` is the
quality of the method it displays against what it sets out to establish. A claim whose text carries a
derivation is mis-typed: the derivation belongs in the analysis it should have been, and until it
moves, the claim's strength says what actually carries it — nothing. Whether anything carries a node
is `strength`'s business, and the walk below says how.

