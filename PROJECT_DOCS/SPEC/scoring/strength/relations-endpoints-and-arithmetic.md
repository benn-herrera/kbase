## Relations: endpoints and arithmetic

The two minimum relations split by where the premise comes from: `supports` carries the node's own
premises, and `founds` carries support from outside its text — a cited work, or an axiom the node
leans on. An axiom's being declared in `axioms.md` is bookkeeping; what it states is an external
fact, and that is what `founds` records.

| relation | source → target | bearing feeds | arithmetic |
|---|---|---|---|
| `supports` | claim / experiment → claim or analysis | supporter's strength | multiply, into the min |
| `founds` | work / axiom → claim or analysis | the source's rigor (an axiom's is `1.00` by rule) | multiply, into the min |
| `lifts` | analysis / experiment → claim or analysis | lifter's strength | multiply, max above the min |
| `refutes` | claim / analysis / experiment → claim or analysis | refuter's strength | cap: `1 - (bearing * strength)` |

Unscored relations:

| relation | source → target | bearing | arithmetic |
|---|---|---|---|
| `references` | claim → claim | — | not scored |
| `demoted` | claim → claim (at build time only, for breaking cycles) | — | not scored |

An axiom premise is a `founds` edge from an axiom; it has its bearing like every scoring edge,
and the axiom's `rigor` of `1.00` by rule leaves its contribution the bearing alone.

