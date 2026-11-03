### The `method` faults

Six faults, maximum `1.00`. The weight is per occurrence. The subject is an analysis's or an
experiment's text: the derivation or procedure it displays, against what the node sets out to
establish. A work, judged by `edit-work` with its document in hand, is judged by this vocabulary; no
pass asks it, because the document is not in the corpus.

| fault id | the fault | one occurrence is | weight per occurrence |
|---|---|---|---|
| `flt-unaddressed` | The method does not address what it sets out to establish: no step, procedure, result or citation the text displays bears on the conclusion the node asserts. | One respect the text leaves unaddressed — the steps, the result, the link from what it shows to the conclusion — at most three, the respects being a closed list. | `0.15` |
| `flt-contradiction` | The method's text cannot all be true: two statements it makes cannot both hold and the conclusion rests on at least one of them, or the conclusion is incompatible with a statement the text makes — while a limitation, counter-case or range the conclusion names, and a tension a condition the text states resolves, are not contradictions. | One minimal set of statements that cannot all hold: as many sets. | `0.15` |
| `flt-overreach` | The conclusion the method draws claims more than its text establishes: the narrowest proposition the text establishes does not entail the conclusion — a wider scope, a stronger modality, a value stated for a case the text does not cover — while a restatement that entails nothing more is not overreach. | One respect in which the entailment fails: as many as fail, and at most three, because the respects are a closed list — scope, modality, an uncovered value; a fourth charge repeats one of them. | `0.15` |
| `flt-claimed-step` | A step is claimed and none is shown: the text marks an inference — *hence*, *it follows*, *therefore*, *clearly*, *one can show* — and displays no statement from which the asserted conclusion follows; a conclusion claimed with no marker is not this. | One marked inference with no displayed statement the conclusion follows from: as many as the markers so placed. | `0.15` |
| `flt-unstated-premise` | A displayed step — an inference, or a computation from stated quantities — rests on something the text does not state: it does not follow unless a proposition the text does not state is granted; a premise the text names as a graph node, or one an edge of the node carries, is that edge's `bearing` and never this. | One **distinct** proposition a displayed step needs and the text does not state: as many propositions, not as many steps. | `0.15` |
| `flt-bare-measurement` | A number is reported and no method is: the text reports a measured or computed value and displays no procedure, sample, instrument, dataset or calculation that yields it; a stated constant, a stipulated parameter, a definition and a reference are not results. | One reported value with no displayed method: as many values — a table of six numbers with no method is six charges. | `0.15` |

**A method that displays nothing has not made a claim's mistake.**
A text that asserts a conclusion and displays nothing bearing on it is `flt-unaddressed` on each
respect it leaves empty — the steps, the result, the link to the conclusion, at most three — and those
alone leave it at `0.55`, never at nothing; a text that displays a procedure and no reasoning for its
conclusion is charged where the gap is. `1.00` on a method reads *no charge among the six*, and for an
analysis or an experiment that is a statement about its method, not about whether anyone supports it.

