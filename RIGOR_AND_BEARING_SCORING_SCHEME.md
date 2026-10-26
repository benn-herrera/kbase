# Rigor and bearing: the two vocabularies, the table and the answer

The two scoring passes ask a model about one item each: `rigor` on one node, `bearing` on one
scoring edge. The answer is a list of **charged occurrences** — one fault id and one citation per
line — and kbase prices the list into the one value the register holds.
[SPEC.md](SPEC.md) §8 carries the `score` pass, its answer and its ledger, §10 the register's
spelling, and [STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md) the three quantities and the
walk's arithmetic; that page is the authority for the arithmetic, and this page says of it only what a
`rigor` value needs, under
[The strength is the walk's, not the rigor's](#the-strength-is-the-walks-not-the-rigors).

## The two judgements

**`rigor` judges one node's own worth**: what the node's text displays, against what the node
asserts. It is asked of a claim, an analysis and an experiment — never of a work, whose standing is
judged with the document in hand and written by `set-rigor`, and never of an axiom, whose `rigor` is
`1.00` by rule. Truth is out of scope — a charge is returned on what the text displays, never on
whether the assertion holds in the world, so a text that reasons impeccably to a false conclusion and
contradicts itself nowhere returns `none`. A premise the node's edges carry is out of scope too:
whether it applies to the assertion is that edge's `bearing`.

**The node's class decides what `rigor` asks.** A claim is an assertion: its own text is a
**statement**, and the **statement vocabulary** asks whether the assertion is fixed, consistent and
single-sensed. An analysis or an experiment is a method: its text is a derivation or a procedure, and
the **method vocabulary** asks whether it establishes what it sets out to establish. A work is judged
as a method when it is judged at all, though no pass asks it. A claim displays no derivation, and that
is not a defect: a derivation belongs in an analysis, and support reaches a claim through edges. A
claim's missing method is therefore not a statement fault, and a claim's missing support is not a
rigor fault at all — it is the walk's `0`, under
[The strength is the walk's, not the rigor's](#the-strength-is-the-walks-not-the-rigors).

**`bearing` judges how much of one source's argument does work for the target it scores**: which of
its parts supply what the link claims the target takes from it — the edge's context states that claim
— and which would change what one should think of some part of the target's assertion. A premise need
not share the conclusion's content: what it supplies is judged, not how far the two statements
overlap. The context is the claim under test, never evidence that the claim holds. The source is
taken as given — true as it states itself — and neither its soundness nor the target's wisdom in
citing it is judged; bearing weights the source's contribution and never modifies the source's value.
A part of the source does *work* for the target when, taking the source as given, believing it would
change what one should think about some part of the target's assertion, in either direction. Support
and refutation are both work done, and the sign is never judged: the charges count the parts of the
source that do none, and the walk turns what remains into a limit. A source and a target may share
subject matter and vocabulary while nothing any part of the source states reaches what the target
asserts.

How much of the *target* the source covers is a different question, and no charge prices it: see the
coverage note under [The `bearing` faults](#the-bearing-faults).

The judgements are orthogonal: one text may be an exquisite statement and bear on nothing, a shoddy
method and exactly on point, or a claim whose statement is weak and whose support is strong. `rigor`
and `bearing` have their own sets, and each rigor vocabulary its own table; no mark combines them.
Which rigor vocabulary applies is fixed by the item's class before the call.

## The fault vocabulary

Fifteen id rows name the faults: four in the statement vocabulary, six in the method vocabulary and
five in `bearing`, with `flt-contradiction` and `flt-overreach` standing in both rigor vocabularies.
Thirteen ids are distinct, and a ledger may also carry the retired `flt-pending` from a line written
before it went. An id is written `flt-[a-z0-9-]+` and spells the criterion it names, so a
ledger line needs no table to read.

**An id names one fault wherever it appears.** The `bearing` set is disjoint from both rigor sets. The
two rigor vocabularies share two ids because `flt-contradiction` and `flt-overreach` name the same
fault at different subjects, and each shared id carries the same weight in both tables, so a ledger
line's price never depends on which rigor vocabulary read it; only the definition's subject changes.
The ids carry no severity and no order — the weight is the table's, below — and `none` is a word, not
an id: it names the absence of a charge and is not a member of any set.

Rows are in walk order within a vocabulary. A criterion is met at each site of the text whose test it
satisfies, and where one site satisfies two rows of the same vocabulary the earlier row's id is the
charge and the later is not.

**That precedence binds the judgement, and the parse records what the reply wrote.** It is a rule on
how the model reads the text, and no check enforces it. The reason is the parse's whole remit: it
cannot rule on whether two criteria were met at one site, which is the same question as whether a
line earns an id, and it never asks that ([What the check does, and does not
do](#what-the-check-does-and-does-not-do)). A shared line is also not the same fact as a shared
site: two rows whose tests are met at different occurrence units — a minimal set, a respect, a
distinct proposition — may anchor at one line, so a parse that refused the second id on a line set
would refuse defensible answers. Two ids on one line set are therefore accepted and both charged, and
the shared quote and offset beside the verbatim reply are what let a later reader hold the line
against the precedence and see whether the walk's order was missed. The cost is stated rather than
hidden: a departure from the precedence is priced, not refused. The alternative — the parse refusing a
second id on a line set — is not the choice, because it turns a judgement it cannot check into
`malformed`, discarding both the answer and the evidence for it, and a retry is a fresh guess at the
same unverifiable question.

### The `statement` faults

Four faults, maximum `1.00`. The weight is per occurrence. The subject is a claim's title read
together with the statement under it: the title announces, the statement fixes, and both are the
claim's own text.

| fault id | the fault | one occurrence is | weight per occurrence |
|---|---|---|---|
| `flt-contradiction` | The claim cannot all be true: two parts of its title and statement cannot both hold — a value and a range that excludes it, two quantities that cannot coexist, a clause that negates another — while a limitation, counter-case or range the statement names, and a tension a condition it states resolves, are not contradictions. | One minimal set of statements that cannot all hold: as many sets. | `0.15` |
| `flt-overreach` | The title claims more than the statement fixes: the narrowest proposition the statement settles does not entail the title — a wider scope, a stronger modality, a value stated for a case the statement does not cover — while a title that is a summary entailing nothing more is not overreach. | One respect in which the entailment fails: as many as fail, and at most three, because the respects are a closed list — scope, modality, an uncovered value; a fourth charge repeats one of them. | `0.15` |
| `flt-equivocation` | A term the claim turns on carries two senses: the title and the statement, or two clauses of either, use one term for different referents or under different definitions, so that substituting one sense for the other leaves the claim saying two things or nothing. | One such term: as many terms. | `0.15` |
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
a node with no supporter is the walk's `0`, never a charge on the text.

**`flt-overreach` survives in the statement vocabulary, and is not the method's overreach.** A claim
has no derivation, so it cannot draw a conclusion beyond one; but it has two passages, and the title
can announce more than the statement fixes. The fault is measured there, between the announcement and
the statement, and it is a statement fault because neither passage is an argument. The old rigor's
rule stands: where the statement says less than the title, the shortfall is a fault and not a
redefinition of the assertion. What the fault cannot be is the method's form — a claim is never
charged for failing to derive its title, because deriving is not what a claim does.

**The owner's worked case.** *"Parisian bleu cheese emits 2.8×10⁹ scent molecules per gram per degree
above 270 K"* is specific, disprovable and underspecified: it does not fix the producer or brand, the
vintage or year, the humidity or the pressure, and the emission rate can turn on each. Four free
parameters charge `0.20` and land `0.80`; naming all four fires nothing and lands `1.00`. The
statement is not charged for the method it does not display, because that method is its supporter's.

**A bare assertion fires none of the four, and that is the vocabulary working, not a gap.** A claim
whose title and statement say the same thing, whose parameters are fixed, whose terms carry one sense
and whose parts are consistent, answers `none` and keeps `1.00`. The value reads *the statement is
fully specified, consistent and single-sensed*; it never reads *the claim is supported*. Support is
`strength`, and a claim whose statement is impeccable and which nothing supports earns `1.00` rigor
and `0.00` strength. The old worry — that a bare assertion and a text that argues impeccably were the
same value — does not arise for a claim: a claim is not supposed to argue, and the walk states what
supports it. This is settled: neither the four nor the six names bareness — a bare claim fires none
of the four, its rigor is the maximum and its strength is the walk's `0` because support is the
edges' — and no seventh criterion is added over either vocabulary
([STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md), "One premise, and none").

**Where the boundary runs.** A condition the assertion leaves free is `flt-underspecified`. A
condition a displayed step needs and the text does not state is the method's `flt-unstated-premise`. A
condition a source's bearing depends on is the edge's `flt-conditional`. A term used two ways within
one claim is `flt-equivocation`; a term two texts use differently, carrying a link, is `bearing`'s
`flt-referent`.

### The `method` faults

Six faults, maximum `1.00`. The weight is per occurrence. The subject is an analysis's or an
experiment's text: the derivation or procedure it displays, against what the node sets out to
establish. A work, judged by `set-rigor` with its document in hand, is judged by this vocabulary; no
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

### The `bearing` faults

Five faults, maximum `1.00`. The weight is per occurrence, and every charge costs the same ten
hundredths whatever its reason: a link's bearing is the proportion of the source's parts that do work
for the target, so one part that does not is `0.90`, two are `0.80`, and nothing dies of one
judgement. It takes ten charges to price a link at nothing, which is the work a worthless link should
cost.

Bearing is how much of the source's own argument does work for the target: the parts that supply what
the edge's context says the target takes, and the parts that would change what one should think of
some part of the target's assertion, in either direction. A part that does neither is what the table
charges. **How much of the target's surface the source reaches is not this measure.** `flt-partial`
priced one part of a compound assertion left unengaged, and it is retired with the coverage question
it was answering: a link that reaches one clause of a five-part theorem and nothing else is priced on
what the source's own argument does with that clause. Target coverage is a known weakness of the
scheme, with its own R&D pending, and no charge prices it until that lands.

| fault id | the fault | one occurrence is | weight per occurrence |
|---|---|---|---|
| `flt-referent` | The link rests on a term the two statements use differently: source and target share a term that carries the link and use it for different referents or under different definitions; where nothing survives substituting the target's sense for the source's, the fault is `flt-inapplicable`. | One shared term the two texts use differently and that carries the link: as many terms. | `0.10` |
| `flt-narrower` | The source reaches the assertion only within a narrower scope: the assertion quantifies over a class, a range, a population or a set of conditions, and the source's content holds for part of it — one member, one regime, one value of a parameter; an assertion scoped to that part, and a source general where the assertion is the case, are not narrower. | One dimension in which the source's scope is narrower: as many dimensions. | `0.10` |
| `flt-transfer` | The link runs through a step neither text states: the source's subject differs from the target's, and what makes the source bear on the target is an unstated bridge — an analogy, a scaling, *the same holds for*, a change of unit or of definition; a bridge stated in either text or in the edge's context is not this. | One unstated bridge the link runs through: as many bridges. | `0.10` |
| `flt-conditional` | The source applies only under a condition it does not state: its content is stated under conditions, and its bearing on the target depends on a condition the source does not give — a regime, a temperature, an assumption — that the target's assertion needs. | One unstated condition the bearing depends on: as many conditions. | `0.10` |
| `flt-inapplicable` | One part of the source's argument does no work for the target: nothing that part states is what the edge's context says the target takes, and nothing else it states would change what one should think of any part of the target's assertion, in either direction; a shared vocabulary is not work done. | One part of the source that reaches nothing: as many parts. | `0.10` |

**The ask lists them specificity first.** A part that satisfies two of these tests is charged as the
earlier one — a term used two ways, a scope narrower than the assertion, an unstated bridge, an
unstated condition — and `flt-inapplicable` comes last so it takes only a part that fits none of them.
The table follows the ask's order.

**A part's weight is absolute, and the bias that follows is known.** Fifty parts that do work and two
that do not price as two and two do: `0.80` either way. Pricing the fraction the dings are of the whole
would take a second ask that quantifies the argument's total volume, and asking a model what fraction
of an argument one statement is answers unreliably; the scheme carries the bias until a reliable form
exists. The `0.10` is the same for every charge, tuned against a guessed average chunk size, and is
provisional.

### `flt-pending` (retired)

The id named a text that held no proposition to judge, in all three vocabularies, and an answer of it
carried `*pending*`. It is retired as an answer: it was a one-line end to an item that took no work,
and in a bearing pass it became the hedge of first resort. A text that asserts nothing is now charged
— `flt-underspecified` on an empty claim, `flt-unaddressed` on a method that displays nothing, and on
a bearing `flt-inapplicable` for the part that asserts nothing. No table holds the id, so a reply
naming it is an id outside the set; `Price` still reads one recorded before the retirement, so an old
line keeps the value it was written with.

## The value

The maximum is per vocabulary: `M = 1.00` for the statement faults, `M = 1.00` for the method faults,
and `M = 1.00` for `bearing`. For a charge list `C` under the table the item's class selects:

```
value(C)             = max(0, M − Σ w(id(c)) for c ∈ C)
value(none)          = M
value([flt-pending]) = *pending*   (a recorded line only; the id is no answer)
```

The weight is charged **per occurrence**, each occurrence separately, and the table caps no count: a
text that commits a fault three times loses three weights. The parse enforces no count at all, and
the only mechanical pressure on one is that two occurrences of one id need two line sets that do not
coincide. A shared id has the same weight under either rigor vocabulary, so the price of a line that
carries only shared ids is the same whichever table read it; a line carrying `flt-underspecified`,
`flt-unaddressed`, `flt-equivocation` or any method-only id is priced by the table that holds it, and
the item's class fixes which that is.

A count is bounded only where the row's own occurrence unit is an enumeration of a closed set, and
there the bound is what *one occurrence* means rather than a ceiling the table adds.
`flt-unaddressed`'s respects are the closed list of three — the steps, the result, the link to the
conclusion — so it has three. `flt-overreach`'s
respects are the closed list of three, so it has three. `flt-inapplicable`'s parts are the source
argument's own — its stretches of reasoning — which the text fixes in outline and no field counts. A
charge beyond such a bound is not a further
occurrence: it is one of them written a second time. The parse cannot see that, because no respect and
no part is a ledger field; the reader can, holding the reply's line sets against the text.

The floor at zero is the domain's, not a rule of its own: a list whose weights reach `M` is a text
with nothing left to carry the assertion, and `0` is what the register says for that. Every weight is
a multiple of `0.05`, so every sum and every value is exact at two decimals and nothing rounds.

**`1.00` is reserved by rule, and the reserve is not a model's answer.** `kb.AxiomRigor` is `1.0`; an
axiom premise contributes `bearing × 1.0`, and no authored file carries `1.00`. An axiom is asked by
neither pass — its `rigor` is kbase's rule, not a judgement — so the authored maximum under either
rigor vocabulary is the largest multiple of `0.05` below the reserve, `0.95`, and `none` is the only
answer that reaches it. A charge list under either rigor table cannot reach `1.00` at all. `0.95` is
the largest value a rigor pass derives and the largest a write carries: `set-rigor` and every other op
that takes a node's own `rigor` refuse a value above it, naming the key and the range, so `1.00` is
never an authored value. A file already holding `1.0` — a converted experiment — is read as the
maximum: an authored rigor above `kb.RigorMaximum` reads as `kb.RigorMaximum`, so the `1.0` a
converted experiment carries is `0.95` everywhere the system shows it — the strength line a refresh
derives from it, the index's derivation, a query's display and the walk — while the file keeps the
text it wrote, a refresh rewriting no rigor it found. The ceiling is a rule on reading as well as
writing, and `kb.RigorMaximum` is its one statement; `1.00` belongs to an axiom by rule, is no read
value, and is never clamped. `bearing` has no such reserve, and `none` reaches `1.00`.

> **Superseded in 0.8.0.** The reserve above is withdrawn: `kb.RigorMaximum` is `1.00`, a rigor pass's
> maximum is the same number as an axiom's rule value, and a text charged with nothing prices at
> `1.00` — so an authored `1.0` is in domain and reads as itself, and only a value above `1.00` is
> read at the ceiling. The reasoning stays as the ruling that produced it; the figures in this
> paragraph are the earlier ones. The three tables below are priced at the ceiling stated here, and a
> value one shows is its own arithmetic under that ceiling.

## The strength is the walk's, not the rigor's

[STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md) owns the walk — the sets, the `⊥` table, the
empty-`P(n)` rule and its consequences — and this page changes none of it. One consequence bears on a
reader of a `rigor` value: an empty `P(n)` means no premise carries the node, so a claim's strength is
`0` however good its statement.

**What a bare assertion earns.** "Bare" means a claim whose statement fires nothing: its rigor is the
maximum the statement vocabulary reaches, `1.00`, and its strength is decided by its edges. A claim
whose statement is impeccable and which nothing supports is `1.00` rigor and `0.00` strength; with one
supporting edge it keeps its `1.00` and takes the edge's contribution as its strength, unless that
product exceeds `1.00`, in which case its own statement quality is the cap. An analysis reads the same,
with its method rigor in place of `1.00`. The statement quality is kept in every case; what varies is
whether anything carries it. The ladder reads `strength` and not `rigor`, so this pair is not a band:
its strength is `0.00`, and [the band outcomes](#the-band-outcomes) bottom out there unless a lift
raises it.

## The band outcomes

The ladder is `kb.BuildBandLadder` (`internal/kb/schema.go`): cuts at `0.85`, `0.65`, `0.45`, `0.20`
and `0.00`, each an inclusive lower bound. The ladder reads `strength`, not `rigor` — a node's band
can sit below its rigor because of a premise, and `rigor` is not an upper bound on `strength` because
a lift station stands above the derivation ([STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md),
"The walk"). The columns below are what a rigor value would read when rigor is what binds — that is,
when `P(n)` is non-empty and its weakest contribution is at least the rigor. Where `P(n)` is empty no
row in either rigor table reaches a band: strength is `0`, and the bottom rung is the answer.

### The statement table

Both rigor tables are uniform at fifteen hundredths of their maximum, so each reaches eight values —
`1.00`, `0.85`, `0.70`, `0.55`, `0.40`, `0.25`, `0.10`, `0.00` — and nothing else. A rigor value off
them is a defect of a landing, not a judgement.

| charges | statement rigor | band it would read, when rigor is what binds |
|---|---|---|
| `none` | `1.00` | ok to build on |
| one finding, whatever its id | `0.85` | ok to build on — exactly at the cut |
| two findings | `0.70` | ok to build on, see caveats |
| three findings | `0.55` | use as input only |
| four findings | `0.40` | do not build on, rework needed |
| five findings | `0.25` | do not build on, rework needed |
| six findings | `0.10` | unsubstantiated |
| seven findings, or any sum of `1.00` or more | `0.00` | unsubstantiated |
| a recorded `flt-pending` (a person's, or an old line's) | `*pending*` | no band |

**The bottom rung is now a count, not a kind.** Seven findings of any kind reach it, so a claim with
seven off-target parameters reads as a claim with a contradiction, an overreach, an equivocation and
four further defects. That is the price of no single charge being able to price a text at nothing: zero
is work.

### The method table

The method table reaches the same eight values.

| charges | method rigor | band it would read, when rigor is what binds |
|---|---|---|
| `none` | `1.00` | ok to build on |
| one finding — a gap, an overreach, a contradiction, a respect unaddressed | `0.85` | ok to build on — exactly at the cut |
| two findings | `0.70` | ok to build on, see caveats |
| three findings | `0.55` | use as input only |
| four findings | `0.40` | do not build on, rework needed |
| five findings | `0.25` | do not build on, rework needed |
| six findings | `0.10` | unsubstantiated |
| seven findings, or any sum of `1.00` or more | `0.00` | unsubstantiated |
| a recorded `flt-pending` (a person's, or an old line's) | `*pending*` | no band |

`flt-unaddressed` is one finding per respect the text leaves empty — the steps, the result, the link to
the conclusion — so a text that displays nothing reaches `0.55` on those alone, and the bottom rung by
being charged on each respect it misses and anything else the text commits.

### The bearing values

`bearing` reaches eleven values, every tenth, and no band of its own: it reaches the ladder only
through a product in the walk.

| charges | sum | bearing |
|---|---|---|
| `none` | — | `1.00` |
| one | `0.10` | `0.90` |
| two | `0.20` | `0.80` |
| three | `0.30` | `0.70` |
| five | `0.50` | `0.50` |
| eight | `0.80` | `0.20` |
| nine | `0.90` | `0.10` |
| ten, or any sum of `1.00` or more | `≥ 1.00` | `0.00` |
| a recorded `flt-pending` (a person's, or an old line's) | — | `*pending*` |

### The bottom rung

**`unsubstantiated` states an outcome, not a cause.** Every strength below `0.20` lands there from
either direction: by a cap, where a refuter's `bearing × strength` exceeds `0.80`; or by the minimum,
where four rigor findings, two bearing findings past their tail, a statement's stacked charges, or an
empty `P(n)` — no contributing supporter — leaves nothing to carry the assertion. What the rung says
is that nothing a claim built on this would inherit substantiates the claim, and how that state
arrived is not part of it. The no-supporter case is the one the two-vocabulary split makes ordinary: a
bare claim is a statement with no premise, and the rung is its exact outcome. A reader who needs the
cause has it where it belongs: the fault record names the charges, and the derivation trace prints the
cap, the empty premise set or the station that produced the strength. The rung's cut is an inclusive
lower bound of `0.00`, and its height is not settled ([Not settled](#not-settled)).

## `flt-pending` (retired)

`flt-pending` used to answer *the text does not support judging at all*: no proposition identifiable,
so nothing to judge. It is retired as an answer. One line ended an item that took no work, and in a
bearing pass it became the hedge of first resort — 22 of 89 links in one capture, on texts whose ends
both stated a proposition, which the old ask itself called impossible. No table holds the id, so a
reply naming it is refused as an id outside the set.

It remains a **value**, not an answer: `set-bearing` and `set-rigor` write `*pending*` as a person's
decision, and `Price` still reads a ledger line recorded before the retirement, so an old run keeps
the values it was written with.

**What a text that asserts nothing is charged with.** A title that names a topic and carries no
statement, an empty section, a stub, a placeholder, a pointer at material the call does not show: on a
claim `flt-underspecified` (citing the title or the pointer), on an analysis or an experiment
`flt-unaddressed`, and on a bearing `flt-inapplicable` for each part of the source that asserts
nothing. A work renders its title alone, so a link whose source is a bare work title charges its part
the same way.

**Against `0.00`.** The two are different states where a pending is recorded, and the walk treats
them in opposite directions: a pending `bearing` excludes its edge and a `0.00` bearing is a judgement
that participates, while a pending `rigor` does not by itself make its node's strength pending, since
a claim with no premise reads `0` on its own account ([STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md),
"Pending: an edge either participates or does not"). An analysis differs there, its method standing
under it where a claim's statement does not: one with no premise reads its own rigor, and one whose
rigor is pending reads no number. `0.00` is the judgement that the text carries nothing;
`*pending*` is the absence of a judgement, which only a person now writes.

## The answer

The ask states this contract, so that a reply is not lost to a spelling the ask never named. kbase
reads the reply and nothing else.

### The closed text

Each call carries one item's own text. For `rigor`, under either vocabulary: the node's title and the
statement its own document gives it. For `bearing`: both ends' title and statement in that same form,
then the edge's authored context. The text is carried wrapped at word boundaries to at most 100
columns, its own line breaks kept and nothing rewritten, and the lines lying in a passage under
judgement are numbered from 1 over the whole text, `N| ` before each. The call's own labels carry no
number, and neither does the edge's context: an unnumbered line is no site a charge can cite. A work
renders its title alone — its document lies outside the corpus — so a link from a work is judged on
the title's proposition. SPEC §8 states the closure; the register's own scoring tokens are refused
before the call, and that item is `failed` with no call made.

### The form

One line per occurrence:

```
flt-id: 12
```

An id, a colon, and the number of the line where the fault first shows. The ask asks for that one
line and never a range; the parse still reads a range of lines within one passage, `flt-id: 12-14`.
Leading and trailing whitespace is trimmed, and the field after the colon is trimmed. The number before the bar is the frame's, not the text's: `N| ` is a gutter the call writes,
and the words behind it are the text's own. A line is a charge line or it is not: blank lines are
ignored, and any other line — an explanatory sentence, a heading, a preamble — is `malformed`. Nothing
is read out of prose. The brackets of the format sketch are delimiters, not characters: a line may
carry `[…]:` and `[…]` around the two fields or not, and a bracket pair around a whole field is
stripped before the field is read. Order is free; charges are recorded in the order written, and no
arithmetic depends on it.

### The closed set

The item's class fixes the accepted set before the call:

- `rigor` on a claim (`clm-`) accepts the four statement ids;
- `rigor` on an analysis or an experiment (`ana-`, `exp-`) accepts the six method ids;
- `bearing` accepts the five `bearing` ids, on any scoring edge.

No set holds `flt-pending`: a reply naming it is an id outside the set, and the `allowed` list the
refusal carries is the class's own.

`flt-contradiction` and `flt-overreach` are accepted by both rigor vocabularies. An id from the other
rigor vocabulary — `flt-underspecified` on an analysis, `flt-unaddressed` on a claim — is
`malformed`, and the refusal names the id and the call's ids as `allowed`. So is the other pass's id,
a letter from the retired alphabet, and a misspelling. An id is case-folded before matching and
recorded in its own lower-case spelling.

### `none`

`none`, alone, is the empty answer: no charge, value `M`. It is the only way to reach a maximum. It
is a word and not an id. `none` beside charges is `malformed`: both halves are claims about faults
and they contradict each other outright, and the parse has no ground to prefer either, so preferring
one would invent the judgement rather than read it. An empty reply and a reply of whitespace only are
not answers either: `malformed`, nothing written.

### The citation

A citation is a numbered line of the closed text: `flt-inapplicable: 12`, the line where the fault first
shows. A fault running over several lines is one charge, at its first line. The ask asks for one line
and never a range, and the parse still reads a contiguous range of lines, `flt-transfer: 12-14`. Its
floor is **one valid line number** — a number the call shows, lying in a passage a charge may cite —
and a charge naming none is `malformed`. The dash of a range may be a hyphen, an en dash, an em dash
or a minus sign, with or without spaces around it. Every number of a range lies in the same passage, and the range is
contiguous: lines sitting together with nothing between them, since a blank line, a label or a heading
ends it.

| judgement | a citation lies in |
|---|---|
| `rigor` | the node's title, or its statement |
| `bearing` | the source's title or statement, or the target's |

kbase resolves the field against the rendering the call carried and quotes the cited lines into the
ledger as it carried them, without the gutter, joined by newlines; `lines` records the numbers the
answer named, `citation` the quote, and `offset` the quote's byte offset in the closed text.

A line in the edge's `## Context` carries no number and is no citation for a charge: the context states
what the link claims and the bridges it runs through, and a charge is about what the two texts do and
do not do. A `bearing` citation
may come from either end, because the evidence for a bearing fault can lie at either — `flt-referent`
needs the term as one text uses it, `flt-inapplicable` the source's own part that does no work,
`flt-transfer` the
two subjects. No line names its end: the recorded `passage` carries the attribution. A numbered line
lies in one passage, so no charge is recorded `both`; a ledger line written before line citation may
carry it, and keeps the meaning it was written with.

A statement fault's anchor is the line that carries it: the title phrase that outruns the statement,
the term, the unfixed quantity, one of the incompatible statements. A method fault's anchor is the
step, the value, the marker or the line that carries the gap. The citation proves where the line is; it
never proves the id.

### What the check does, and does not do

kbase checks that the line **exists**; it never checks that the line **earns** the id. An irrelevant
line is a legal citation, and nothing in the parse reads the line against the criterion. Half these
faults are absences — no line bears on the assertion, no displayed step carries the conclusion, no
method yields the number — where no line could be evidence of the fault at all; for those the citation
anchors the **obligation** (the title line that asserts, the line that claims the step, the line
reporting the value), and the absence itself is checked by nobody. A line is an anchor, not proof.
What carries the audit beyond the anchor is the reply kept verbatim beside the parsed charges, the
quote and its offset that locate the line for a later reader, the value's derivability from the ids,
and agreement between runs measured on counts per id.

### Duplicates, overlaps and shared lines

| case | outcome |
|---|---|
| one id, one line set, written twice | one occurrence written twice: the pair collapses to one charge |
| one id, two line sets that overlap without coinciding | `malformed`, naming both lines — the reply asserts two occurrences and supplies one site |
| two ids, one line set | accepted, both charged, the quote and its offset shared in the ledger |

The duplicate collapses because the answer's own evidence cannot tell the two lines apart. The overlap
is refused because the count cannot be read from it, and a retry settles it, where reading it as one
would write a confidently high value. Two distinct faults may be cited at one line: the one-defect
rule is the walk's, not the parser's, and the shared offset is the signal that the precedence was
missed — not proof that it was ([the fault vocabulary](#the-fault-vocabulary)).

### Malformed replies

| case | outcome |
|---|---|
| a line that is not a charge line | `malformed` |
| an id outside the call's set — the other rigor vocabulary's, the other pass's, a retired letter, a misspelling | `malformed`, naming the id and the call's ids as `allowed` |
| an id with no separator, or nothing after the colon | `malformed`, naming the id |
| `flt-pending`, alone or with any field | `malformed`, naming the id: no table holds it ([`flt-pending`](#flt-pending-retired)) |
| a field that names no line — words, a quotation, a line of the context or of a label | `malformed`, naming the id and the field |
| a line number the call does not show, a reply cut short before it wrote one included | `malformed`, naming the id and the number |
| a range written backwards | `malformed`, naming the id and the range |
| a range crossing from one passage to another, or over a gap — a blank line, a label or a heading between its lines | `malformed`, naming the id and the range |
| a number — a decimal, a fraction, `0`/`1`, or a `*pending*` literal | `malformed`: a number standing alone names no id, and a number in a charge's field is a line of the text, so a reply offering the first answers a question no longer asked |
| `none` beside charges | `malformed` |
| an empty reply, or whitespace only | `malformed`, nothing written |

A `malformed` reply is the pass's outcome: no charges and no judgement, the item re-asked on
`--resume` (SPEC §8). The attempt's record carries `status: failed` and the reason — the case from the
table and, where there is one, the id and the lines it named — and **not the reply's text**: a
malformed reply is blank, a truncated fragment or think-lock, and the data point that matters is that
the attempt failed, not the bytes it failed with.

## One ask per item

**The class gates the instruction, and nothing else changes.** A claim's rigor call renders the
**statement** variant; an analysis's or an experiment's renders the **method** variant; an edge's
bearing call renders the bearing ask as it stands. One item is one call and one instruction: no item
is asked twice, no call carries both vocabularies, and a run's call count and model cost are what they
were. The two rigor variants replace the single rigor template; the bearing instruction is untouched,
and its header entry is the one-element mapping.

**The run's identity is one digest over the set.** A rigor run now sends two different instructions, so
the header's `ask_sha256` is the digest of every instruction the pass can send, rendered for an empty
subject, concatenated in a fixed order — statement, then method for a rigor run; bearing alone for a
bearing run — and hashed once. One field, and it moves if any instruction moves: separate digests
would agree or differ together, so a set of them carries no more information than the one. The digest
covers the set the pass can send, fixed at the run's creation — not the subset a `--limit` reached.
Each attempt's `prompt_sha256` in the ledger is the instruction that item actually received, and it is
the line that answers "which variant judged this item". The header is written once and never
rewritten; a resumed run keeps the digest it began under.

## The ledger

Each attempt is one record in the run's `ledger.jsonl`, carrying the item, its status, `faults` as
parsed, the `reply` verbatim where the reply parsed, the `value` landed, `before` — the value the item
carried before the attempt, so the line is its own before/after pair — the model, the prompt's
digest and the attempt's bounds (SPEC §8), or the reason it failed and no text where it did not.
`faults` is the effective answer — one record per charge, in the order written — and `moot` holds any
charge lines a reply once recorded beside a retired `flt-pending`, parsed the same way and priced
nowhere:

| field | holds |
|---|---|
| `id` | the fault id, in its lower-case spelling |
| `passage` | where the cited lines lie: `title` or `statement` for `rigor`; `source` or `target` for `bearing` |
| `lines` | the answer's own numbers: `12`, or `12-14` |
| `citation` | the cited lines as the call carried them, without the gutter, joined by newlines |
| `offset` | the quote's byte offset in the closed text |

A rigor line's item is a `clm-`, `ana-` or `exp-` id; its prefix selects the statement or method table
a value or a re-price is computed under, so the rigor vocabulary is not a new field. A bearing line's
table is the pass's own. `[]` is written for one reply only —
the word `none` — so a record's empty list always carries the model's positive statement that it found
no fault; an empty or whitespace-only reply is `malformed`, so the attempt records `failed` and no
charges, and silence is never read as a clean pass. A line recorded before the pending id was retired
may carry `id: flt-pending` and no lines and no citation; no reply produces one now. The reply stands
beside the parsed charges, so the parse's reading
of a line can be held against the line. A `--dry-run` writes no charge record: nothing is asked, so
nothing is returned.

**The value is a projection of the charge list, and in fact of its ids alone** — `value = table(counts
by id)`, the table chosen by the item's class. The citations are not the input, and that is why they
are the audit rather than the measure: they can be read, checked against the text and argued about,
and they cannot perturb the arithmetic.

- **A landing can be checked without a model.** A value that does not equal the table's output for
  its recorded ids is a defect of the landing or of the table, not a judgement to argue with. Counts
  are integers, so the check is exact.
- **A run can be re-scored without a model.** Change a weight and re-derive every value in the ledger
  by counting ids under the line's pass and, for rigor, its item's class; no call is made and no
  judgement re-asked. The durable record is the count per id.
- **Two runs are comparable on their fault sets, not their numbers.** Two runs agree on the judgement
  exactly when their counts per id match; their values are comparable only within one table. A value
  agreement is the weaker test: `0.35` is reached by two `flt-bare-measurement` and equally by one
  `flt-claimed-step` and one `flt-unstated-premise`, so a sum hides both the split and the identity.
  Reproducibility is measured on counts. A shared id crossing the two rigor vocabularies is one id in
  the count and one weight in the price, which is what keeps the comparison honest.

**The run records the ask set, not the table.** What a charge set means is fixed by the instructions
the model was given — the fault set and the text describing it — and never by the weights, which the
model is never shown. So the run's header carries one `ask_sha256` over the pass's rendered
instructions, each rendered for an empty subject: rendering substitutes and never inspects the text,
so the empty input covers every fixed byte of an instruction and nothing that varies, and no
size-dependent element may enter a rendered prompt. A resume reads the digest it began under and its
lines carry their own digests as before.

- **An update applies its behaviour.** A run resumed under a changed table re-prices every landed
  value from the recorded ids under its item's class, lands the values that moved and continues; one
  resumed under a changed ask continues under the new ask. Nothing is refused, and nothing asks the
  user to choose between versions: they updated, which is the choice.
- **The digests are a record, not a gate.** `ask_sha256` says which instructions answered which
  class of item, so a corpus spanning an update can be read honestly. It decides nothing at run time.

**A truncated reply.** A reply cut off at the attempt's bound loses the lines it did not reach, and
where the cut falls between lines the remainder would parse cleanly — a silent under-charge the
parser cannot see. The attempt record carries the fact, not the text: a reply that reached the
bound is recorded as failed for that reason, so an under-charge from truncation never lands as a
judgement.

**A failed attempt keeps no text.** A reply that failed is blank, cut short or think-lock, and the
record's data point is that it failed; a reply that parsed keeps its text, where the shape of a good
answer — order, spelling, what a model chose to charge — is behaviour worth comparing.

**The record format is a contract.** One record per attempt, in the run's own file, with the fields
this section names and no fields that summarize them: no prose description of the judgement, no
count rolled up at write time, no value stored in place of the charges it came from. A parsed reply
stands beside its charges; the value stands beside both, derived and checkable; the run's header
carries the ask set once, for every attempt in the run. This is what makes a run re-scorable, a
landing checkable and two runs comparable, so the format changes only with the scheme's version and
never as an implementation convenience: a field dropped at write time cannot be recovered by a later
reader, and a summary written by the writer is a judgement the corpus can no longer take apart.

## Frozen rules

These parse rules are settled here and not open:

- **The floor of one valid line number.** A citation names at least one line the call shows, lying in
  a passage a charge may cite; a charge naming none is `malformed`.
- **`none` beside charges.** Refused as `malformed`; the two halves contradict each other and the
  parse has no preference for one.
- **No reply is a refusal.** A table's charges or `none` are the whole vocabulary; a claim that
  engages nothing, an empty text or a text that asserts nothing is charged, not declined. The retired
  `flt-pending` is an id outside the set.
- **Overlapping line sets for one id.** Two line sets of one id that overlap without coinciding are
  `malformed`, naming both lines; one id on one line set written twice collapses to one charge.

The selection of a line in the closed text, the retirement of the `both` value and
the meaning of a line already written with it, and the malformed classes and the ledger's charge records
are settled in the same way.

## Not settled

- The bottom cut's height: the last rung takes every strength under `0.20`, and where the cut belongs
  is open; it stays at `0.20` for now, reviewable if the bands crowd.
- Whether the table's numbers are right. The statement table was built to keep `unsubstantiated`
  rare — its weights leave the bottom rung to contradictions, stacked defects and the walk's empty
  `P(n)` — and whether those numbers are right is a corpus question.
- **Target coverage has no measure.** Bearing prices how much of the source's argument does work for
  the target; how much of the target the source reaches is not priced, so a link that touches one
  clause of a five-part theorem reads high. `flt-partial` priced the second quantity and is retired.
  Whether coverage belongs inside the bearing value, beside it as its own record, or nowhere, is open.
- **`flt-inapplicable`'s weight is provisional.** `0.10` is tuned against a guessed average chunk
  size, and the weight is absolute: fifty on-target parts with two off read as two and two. Pricing
  the fraction would take a second ask quantifying the argument's total volume, which no reliable
  answer gives today.
