### The `statement` faults

Four faults, maximum `1.00`. The weight is per occurrence. The subject is a claim's title read
together with the statement under it: the title announces, the statement fixes, and both are the
claim's own text.

| fault id | the fault | one occurrence is | weight per occurrence |
|---|---|---|---|
| `flt-contradiction` | The claim cannot all be true: two parts of its title and statement cannot both hold — a value and a range that excludes it, two quantities that cannot coexist, a clause that negates another — while a limitation, counter-case or range the statement names, and a tension a condition it states resolves, are not contradictions. | One minimal set of statements that cannot all hold: as many sets. | `0.15` |
| `flt-overreach` | The title claims more than the statement fixes: the narrowest proposition the statement settles does not entail the title — a wider scope, a stronger modality, a value stated for a case the statement does not cover — while a title that is a summary entailing nothing more is not overreach. | One respect in which the entailment fails: as many as fail, and at most three, because the respects are a fixed list — scope, modality, an uncovered value; a fourth charge repeats one of them. | `0.15` |
| `flt-equivocation` | A term the claim turns on has two senses: the title and the statement, or two clauses of either, use one term for different referents or under different definitions, so that substituting one sense for the other leaves the claim saying two things or nothing. | One such term: as many terms. | `0.15` |
| `flt-underspecified` | The statement leaves free a parameter its truth depends on: a quantity, a condition, a population, a basis or unit, or the criterion by which a predicate is applied, that the assertion does not fix and whose value could change whether it holds. A statement that fixes it, or whose truth does not turn on it, is specified. | One distinct free parameter the assertion's truth depends on: as many parameters. | `0.15` |

**A claim is an assertion, not an argument, and the statement vocabulary is short because of it.** The
method's `flt-claimed-step`, `flt-unstated-premise` and `flt-bare-measurement` name failures of a
displayed derivation: a step claimed with none shown, a step resting on an unstated proposition, a
number reported with no method. A claim displays no derivation, so it can commit none of them. A
number in a claim is part of the proposition — a value the claim asserts, with the conditions under
which it holds — not a result whose method is missing; the method is the supporter's, and is judged
there. This is the whole reason a claim needs no `flt-bare-measurement`: `2.8×10⁹ molecules per gram
per degree above 270 K` is a statement to be tested, and the experiment that measured it is a
different node. Neither vocabulary has an `flt-unsupported`: support is the edges' business in both, so
a node with no supporter has strength `0` from propagation, never a charge on the text.

**`flt-overreach` survives in the statement vocabulary, and is not the method's overreach.** A claim
has no derivation, so it cannot draw a conclusion beyond one; but it has two sections, and the title
can announce more than the statement fixes. The fault is measured there, between the announcement and
the statement, and it is a statement fault because neither section is an argument. The old rigor's
rule remains: where the statement says less than the title, the shortfall is a fault and not a
redefinition of the assertion. What the fault cannot be is the method's form — a claim is never
charged for failing to derive its title, because deriving is not what a claim does.

**The owner's worked case.** *"Parisian bleu cheese emits 2.8×10⁹ scent molecules per gram per degree
above 270 K"* is specific, disprovable and underspecified: it does not fix the producer or brand, the
vintage or year, the humidity or the pressure, and the emission rate can turn on each. Four free
parameters charge `0.20` and the value comes to `0.80`; naming all four fires nothing and it comes to `1.00`. The
statement is not charged for the method it does not display, because that method is its supporter's.

**A bare assertion fires none of the four, and that is the vocabulary working, not a gap.** A claim
whose title and statement say the same thing, whose parameters are fixed, whose terms have one sense
and whose parts are consistent, answers `none` and keeps `1.00`. The value reads *the statement is
fully specified, consistent and single-sensed*; it never reads *the claim is supported*. Support is
`strength`, and a claim whose statement is impeccable and which nothing supports earns `1.00` rigor
and `0.00` strength. The old worry — that a bare assertion and a text that argues impeccably were the
same value — does not arise for a claim: a claim is not supposed to argue, and strength propagation accounts for what
supports it. This is settled: neither the four nor the six names bareness — a bare claim fires none
of the four, its rigor is the maximum and its strength is `0` from strength propagation because support is the
edges' — and no seventh criterion is added over either vocabulary
([STRENGTH_SCORING_SCHEME.md](../../strength/one-premise-and-none.md), "One premise, and none").

**Where the boundary runs.** A condition the assertion leaves free is `flt-underspecified`. A
condition a displayed step needs and the text does not state is the method's `flt-unstated-premise`. A
condition a source's bearing depends on is the edge's `flt-conditional`. A term used two ways within
one claim is `flt-equivocation`; a term two texts use differently, carrying a link, is `bearing`'s
`flt-referent`.

