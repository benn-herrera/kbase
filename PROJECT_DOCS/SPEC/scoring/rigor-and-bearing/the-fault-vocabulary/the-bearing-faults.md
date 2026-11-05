### The `bearing` faults

Five faults, maximum `1.00`. The weight is per occurrence, and every charge costs the same ten
hundredths whatever its reason: a link's bearing is the proportion of the source's parts that do work
for the target, so one part that does not is `0.90`, two are `0.80`, and nothing dies of one
judgement. It takes ten charges to price a link at nothing, which is the work a worthless link should
cost.

Bearing is how much of the source's own argument does work for the target: the parts that supply what
the edge's context says the target takes, and the parts that would change what one should think of
some part of the target's assertion, in either direction. A part that does neither is what the table
charges. **How much of the target's content the source reaches is not this measure.** `flt-partial`
priced one part of a compound assertion left unengaged, and it is retired with the coverage question
it was answering: a link that reaches one clause of a five-part theorem and nothing else is priced on
what the source's own argument does with that clause. Target coverage is a known weakness of the
scheme, with its own R&D pending, and no charge prices it until that is delivered.

| fault id | the fault | one occurrence is | weight per occurrence |
|---|---|---|---|
| `flt-referent` | The link rests on a term the two statements use differently: source and target share a term that carries the link and use it for different referents or under different definitions; where nothing survives substituting the target's sense for the source's, the fault is `flt-inapplicable`. | One shared term the two texts use differently and that carries the link: as many terms. | `0.10` |
| `flt-narrower` | The source reaches the assertion only within a narrower scope: the assertion quantifies over a class, a range, a population or a set of conditions, and the source's content holds for part of it — one member, one regime, one value of a parameter; an assertion scoped to that part, and a source general where the assertion is the case, are not narrower. | One dimension in which the source's scope is narrower: as many dimensions. | `0.10` |
| `flt-transfer` | The link runs through a step neither text states: the source's subject differs from the target's, and what makes the source bear on the target is an unstated bridge — an analogy, a scaling, *the same holds for*, a change of unit or of definition; a bridge stated in either text or in the edge's context is not this. | One unstated bridge the link runs through: as many bridges. | `0.10` |
| `flt-conditional` | The source applies only under a condition it does not state: its content is stated under conditions, and its bearing on the target depends on a condition the source does not give — a regime, a temperature, an assumption — that the target's assertion needs. | One unstated condition the bearing depends on: as many conditions. | `0.10` |
| `flt-inapplicable` | One part of the source's argument does no work for the target: nothing that part states is what the edge's context says the target takes, and nothing else it states would change what one should think of any part of the target's assertion, in either direction; a shared vocabulary is not work done. | One part of the source that reaches nothing: as many parts. | `0.10` |

**The prompt lists them specificity first.** A part that satisfies two of these tests is charged as the
earlier one — a term used two ways, a scope narrower than the assertion, an unstated bridge, an
unstated condition — and `flt-inapplicable` comes last so it takes only a part that fits none of them.
The table follows the prompt's order.

**A part's weight is absolute, and the bias that follows is known.** Fifty parts that do work and two
that do not price as two and two do: `0.80` either way. Pricing the fraction the dings are of the whole
would take a second model call that quantifies the argument's total volume, and asking a model what fraction
of an argument one statement is answers unreliably; the scheme has the bias until a reliable form
exists. The `0.10` is the same for every charge, tuned against a guessed average chunk size, and is
provisional.

