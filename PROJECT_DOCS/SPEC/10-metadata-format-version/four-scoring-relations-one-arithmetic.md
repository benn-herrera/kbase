### Four scoring relations, one arithmetic

Every scoring edge is stored push: `source` is the supporter, `target` the supported node, and
`target_kind` the target's kind. An edge reads "source *relation* target": "A supports B" means A's
strength bears on B.

| Relation | Source kinds | Target kinds | Contribution |
|---|---|---|---|
| `supports` | claim, experiment | claim, analysis | `bearing × source strength` into the minimum |
| `founds` | work, axiom | claim, analysis | `bearing × the source's rigor` into the minimum; an axiom's rigor is `1.0` by rule |
| `lifts` | analysis, experiment | claim, analysis | `bearing × the lifter's strength`, maxed above the minimum |
| `refutes` | claim, analysis, experiment | claim, analysis | a cap, `1 − bearing × refuter strength`; the smallest cap wins |
| `references` | any kind | any kind | none: it has `context` only, saying why a node names another without bearing on its strength |
| `demoted` | ([The `demoted` relation](the-demoted-relation.md)) | | none |

The four *scoring relations* are `supports`, `founds`, `lifts` and `refutes`. A node's *dependents*
are the distinct targets of its outgoing scoring edges.

`supports` carries a premise from the argument itself; `founds` carries support from outside it — a
cited work, or an axiom the argument leans on. An axiom's declaration in `axioms.md` is bookkeeping,
not origin: what it states is an external fact.

`references` and `demoted` are the two unscored relations: `references` has no bearing and no
value, and `demoted` keeps the bearing of the edge it replaced, which strength propagation does not
read. A bearing is required on every scoring edge and illegal on these two. A bearing weights the
contribution and never modifies its source's value. The scoring edges form no cycle, and an edge
always names nodes that exist (The load).

```
premises   = every supports and founds contribution that participates
derivation = min(rigor, every contribution in premises), and where premises is empty the node's
             own rigor for an analysis, 0 for a claim
final      = min(max(derivation, every lift), every cap)
```

`rigor` caps what the premises establish and is no premise itself. A node no participating premise
carries has an empty premise set, and its derivation is what its kind is worth bare: `0` for a claim
— whatever its `rigor` says, judged, pending or absent — so a bare claim reads `0.00` and falls in
`unsubstantiated`, however good its statement; an analysis's own `rigor`, since a method's inputs are
the rules of inference and not a KB node, so a premise-less analysis reads its method's value and the
claim it lifts reads it too. An edge that does not contribute is no supporter, so a claim whose only
edge is excluded reads `0`. Where a premise does participate and `rigor` is pending, the derivation
is unknown and propagates nowhere: a lift still counts — `max(pending, lift)` is the lift — while a
refutation alone leaves it unknown. `lifts` and `refutes` compose around the derivation unchanged: a
claim whose only scoring edge is a lift takes the lift's product, and one whose only scoring edge is
a refutation takes `0`; an analysis composes the same two around its own rigor, so a lift takes
`max(rigor, lift)` and a refutation `min(rigor, cap)`.

Strength propagation uses full precision: nothing rounds a value on its way into a derivation, so a
dependent multiplies its source's exact strength. A summary page — its `- strength:` line and a
premise's `(strength …)` — shows the value in the fewest decimals that parse back as it: four in the
ordinary case, more only where four would lose it, trailing zeros trimmed. A query result gives it as
a number at two decimals, the trace on a `- strength:` line writes it at two, and a `verify` message
quotes the record's form, being about the record. A pending bearing, or a pending source strength,
excludes its edge — no contribution, no lift, no cap — while `0.0` is a judgement and participates,
taking a minimum to zero and a cap to `1.0`. A non-`run` experiment is inert as a lifter or refuter,
and its `supports` edge is excluded like any unjudged premise.

**A value is derived from a charge list, not composed from the interior of the scale.** Each of the
two scoring passes makes a model call for the faults the text commits — the ids
[RIGOR_AND_BEARING_SCORING_SCHEME.md](../scoring/rigor-and-bearing/entry-point.md) defines, each with the
line that shows it — and kbase computes the value from the list: the pass's maximum less its ids' weights per
occurrence, floored at zero and exact at two decimals ([§8](../8-maintenance-subcommands/index.md)). A written `rigor` is a value in
`[0, 1]` or `*pending*`, and a bearing's the same. `1.00` is the rigor tables' maximum — the largest
a `rigor` pass derives, which reads *no charge*, never *the text displays its reasoning*. An axiom's
`1.00` by rule is that same number and no separate case on the read path: an authored `rigor: 1.0`
is in domain and keeps that value everywhere the system shows it — the summary page's strength line,
the derivation, a query's display and strength propagation. An authored `rigor` above the maximum is
outside the domain, as an infinity (`.inf`) is, and the load refuses it (`check: metadata`), naming
the record file, the record and the field (The load). A bearing's `1.0` is in domain, its table
reserving nothing either. Strength propagation reads the value it is handed, whatever wrote the
number.

