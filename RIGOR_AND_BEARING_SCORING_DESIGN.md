# Rigor and bearing: a closed fault set, charged per occurrence

The two scoring passes ask a model about one item: `rigor` on a node, `bearing` on a scoring edge.
Two earlier answers are retired. The first was a decimal in `[0,1]`: the ends anchored, the middle left
to the reader as "the share of what the statement claims that the text shown establishes". A share
nobody can compute is answered from a prior, so two runs agreed at the ends and disagreed in the middle.
The second was a set of letters, one per fault found — closed and decidable, but one charge per fault
*class*, so a text with one hole in its chain and a text with three were the same answer.

The answer is now a **list of charged occurrences**. Each line carries a fault id and a citation:

```
flt-id: citation
```

The citation is a verbatim span of the text being judged, and kbase checks that it appears. A fault may
be charged as many times as the text commits it — one line per occurrence, no cap per class — and kbase
sums the weights and floors the sum at zero. `flt-pending` is the answer where there is nothing to
judge, and renders the rest moot. `none` is the empty answer, and it is the only way to reach the
maximum.

This page is the statement of the fault ids, the criteria that produce them, the parse and the table. It
is a design document — no prompt text, no code. [SPEC.md](SPEC.md) §8 carries the pass and its answer
format, §10 the register, and [STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md) states the
quantities and the walk; that page is the authority for the arithmetic and is not restated here.

**The normative document is [RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md);
this page is the proposal behind it.** Where the two disagree — on the parse, the answer's floor, the
run's identity or the ladder's words — the scheme governs. What follows is the record of the proposal:
where it stated what the code does today and the scheme settled otherwise, it now reads in the past
tense, and the reasoning stands as it was written.

Two readers: the prompt-engineer who will turn the criteria into text a model applies, and the owner
deciding whether the numbers in the register mean what the ledger says they mean.

## 1. What this page supplies, and what it leaves alone

SPEC §10 closes its account of the arithmetic with a sentence that was the whole problem: *"The
interior of the scale is the judge's: kbase validates the range and composes."* Everything outside that
sentence is settled — the domain, the relations, the stations, the pending rule. The interior was
`[0,1]` minus two endpoints, and any point in it was admissible.

The interior is no longer the judge's. Three things change, and only these three:

1. The reply is a list of lines, each a fault id and a citation from the call's closed fault set, or
   `flt-pending`, or `none` (§3.8, §3.9).
2. The value is the table's output for the charges — their weights summed per occurrence and floored —
   computed in kbase (§3.11, §4.4, §5.4).
3. The ledger records the charges beside the reply the model gave: the id, the passage the citation was
   found in, the citation as written and its offset (§7.4, §8.8).

What does not change: the relations, the minimum, the lift, the cap, the pending rule, the register's
domain. `rigor` and `bearing` in a register are still numbers in `[0,1]` or `*pending*`, and
`set-rigor` and `set-bearing` are unchanged — no fault id and no citation ever reaches a register, and
the arithmetic that consumes the numbers is untouched. A value the pass writes is a projection of a
charge list; the register holds the projection and the ledger holds both.

## 2. Why deductive, and where the analogy stops

Score a routine from the top and deduct for clear faults: each element is an enumerated thing a judge
names, and the code prices it. That structure is what a claim graph's two values need, for a reason that
is about decidability rather than about taste.

The top of the scale is already a universal statement: *nothing in the text fails to carry the
assertion*. A universal over an unenumerated domain cannot be checked; a universal over a closed list
can, by exhausting the list. So the top is decidable exactly when the fault set is closed, and the
bottom is decidable already — one property, *no sentence of the text bears on the assertion*. With
both ends decidable, the interior has a definition: two texts are the same judgement exactly when the
same charges come back — the same ids at the same counts — and one carries less than another when its
charges sum to a greater weight. That is comparability — the property the share lacked, because "the
share established" requires a partition of the assertion into parts and an apportionment across them,
and neither is given anywhere, so the reader supplied both from a prior.

Moving the arithmetic into kbase shortens the chain rather than lengthening it: the model supplies the
classification and the evidence, kbase supplies the scale, and the scale can be revised without asking
any model anything (§8.8).

**Where the analogy stops, and how far the ids carry it.** Diving and gymnastics have a code of points:
correct execution is defined independently of any one routine, and an element is a *deviation from* it.
Derivations have no code of points — there is no canonical execution to compare a text against, so a
fault cannot mean "wrong form", and an element cannot be read off a template. What the sport has that
can be transplanted is the *separation*: the judge names the elements, the code prices them. The ids are
the naming; the table is the price. What cannot be transplanted is the comparison — so an element here
must be a **property of the text whose effect on carrying the assertion is statable in one sentence,
whose presence is decidable by inspection, and whose occurrences are separable**. A text either states
the premise its step needs or it does not. An element that fails any of the three is not in the set,
however often it is noticed; that is what "closed" means here. §3.10 is the third requirement made
explicit: an id whose occurrences cannot be told apart has no count, and the table prices counts.

The price is per occurrence, and that is the one place the code of points must be taken literally. A
routine with three bobbled landings loses three times; a routine where the same landing is bobbled once
in a repeated passage still loses once, because the landing is one. A set of classes could not do this:
a second hole in the same chain was the same answer as the first, so the only signal a rotten text gave
was the number of *kinds* of fault in it — which understates exactly the texts a reader most needs
marked. The charge list is what makes the count expressible, and the citation is what keeps the count
honest: a second occurrence needs a second span.

Two smaller breaks. Sport reports one total; kbase keeps two values and multiplies them up a graph, so
one table per value, never a combined mark. And sport's panel manages disagreement by dropping the high
and low marks; kbase has no panel, one call answers once, so the charges must carry the agreement a
panel would supply. That is the standard the scheme is held to: not accuracy, but reproducibility (§9).

## 3. The form both rubrics take

Same procedure, same parse, same arithmetic; different ids, different tables. The design is a sequence
of decisions a reader makes in order, each with a written outcome, ending in a charge list.

**3.1 The procedure.**

1. Apply the pending test (§6). No proposition to judge: answer `flt-pending` alone and stop. No other
   id applies.
2. Fix the assertion. For `rigor` it is the node's title, read together with the statement its text
   displays; for `bearing` it is the target's assertion by the same rule.
3. Walk the criteria **in the order they are written**. A criterion is met at each site in the text
   whose test it satisfies; each such occurrence is charged with the criterion's id and the span that
   shows it. *A defect is charged once*: where one site satisfies two criteria, the earlier one's id is
   the charge and the later one is not (§3.6).
4. Answer with one line per occurrence — `flt-id: citation` — in any order, and `none` alone where
   nothing is charged.
5. kbase looks up every citation, sums the charges' weights and floors the sum (§3.11). The reader never
   sees a number, a weight or a scale.

**3.2 The assertion is fixed by the title.** The title states what the node asserts; the text is
judged against it. Where the text says less than the title, the shortfall is a fault, not a
redefinition of the assertion. Without this rule a reader can always define the assertion downwards
from whatever the text happens to show and find nothing wrong.

**3.3 Truth is out of scope.** A charge is returned on what the text displays, never on whether the
assertion is true in the world. A text that reasons impeccably toward a false conclusion, with no
internal contradiction, returns `none`. The graph's other values and the world's own record are where
that is answered, not this answer.

**3.4 The quotation rule, made a rule.** A charge is not returned unless the reader can point at the
sentence that earns it, and the pointing is now part of the answer: every line carries a citation,
kbase looks the citation up in the closed text the call carried, and a charge whose citation is not
found is refused (§3.9). The instruction became a rule and the rule has teeth — no charge can be
invented from a prior, paraphrased from the title, carried over from a previous item, or quoted from
the register the guard keeps out. That closes the gap the letter set left open: a fault could then be
charged without a word of justification and nothing in a landed value would show it.

What the rule cannot do is as important as what it can, and §9 states it as a loss rather than papering
over it. It checks that the span **exists**, never that the span **earns the id**: an irrelevant
sentence is a legal citation, and nothing in the parse reads the sentence against the criterion. And
half these faults are absences — no sentence bears on the assertion, no displayed step carries the
conclusion, no method yields the number — where no span can be evidence of the fault. For those the
citation anchors the **obligation** (the title that asserts, the marker that claims the step, the value
reported) and the absence itself is checked by nobody. What carries the audit beyond the anchor: the
model's raw reply kept beside the parsed charges, the offset that locates the span for a later reader,
the value's derivability from the ids (§8.8), and agreement between runs measured on counts per id
(§9).

**3.5 A fault is not a defect of style.** Length, register, formatting, spelling, the absence of
citations, the absence of a proof where none is promised, and the presence of a weaker argument than
the reader would have written are not faults. Each criterion's *not met when* clause names the near-miss
it must not swallow; that clause is as much of the rubric as the test is, because a reader left without
one invents faults.

**3.6 One defect, one charge — a rule that now costs.** A set could not hold a letter twice, so a defect
could not be charged twice by repetition, and where two criteria described one defect their weights were
equal so the choice between them was free. A list can hold a line twice, so repetition is a deduction:
two lines of `flt-unstated-premise` are `0.60`, not `0.30`. Three things hold the rule in place, in
place of the set's impossibility:

- **Walk order decides the id** where one site satisfies two criteria; the earlier criterion's id is the
  charge (§3.1).
- **The parser refuses an overlapping span for one id** (§3.9). Two lines of one id whose spans overlap
  without coinciding name one site twice, and the reply does not distinguish the occurrences the count is
  supposed to carry; two lines with the same span are one charge written twice and collapse to one.
- **Two ids on one span are accepted**, and the shared offset is what a reader sees in the ledger. The
  parser cannot rule on whether both criteria were met — that is a judgement — so the walk-order mistake
  is instruction plus an audit signal, not enforcement.

The cost of getting the choice wrong is stated in the table: two charges cost the sum of their weights,
and where the two are confusable the weights are equal, so the *choice* costs nothing while the
*duplication* costs a whole weight (§3.7).

**3.7 Confusable faults cost nothing; distinct faults cost differently, per occurrence.** Where two
criteria describe the same defect seen from two sides, their ids carry the same weight per occurrence,
so a reader's choice between them moves no value — only the recorded id. Where two criteria describe
defects of different magnitude, their weights differ and the value carries the distinction. The
alternative — folding confusable criteria into one id — is worse now than it was under a set: it would
make one occurrence indistinguishable from three, and the count is the whole of the multiplicity that
the per-occurrence rule exists to express. So the ids stay separate, the weights stay equal, and the
count stays the reader's.

**3.8 The fault ids: one namespace, disjoint names.** The two passes judge different classes of defect,
so an id must not mean one thing in one pass and another in the other. The ids are therefore *disjoint*
across the two passes, in one namespace: an id names exactly one fault wherever it appears, and a reader
of a ledger line needs no context to read it. `flt-pending` is the one id both passes use, because it
means the same thing in both. `none` is a word rather than an id, because it is the absence of a charge
and not a member of the set.

| fault id | pass | criterion |
|---|---|---|
| `flt-unaddressed` | `rigor` | unaddressed |
| `flt-contradiction` | `rigor` | contradiction |
| `flt-overreach` | `rigor` | overreach |
| `flt-claimed-step` | `rigor` | claimed-step |
| `flt-unstated-premise` | `rigor` | unstated-premise |
| `flt-bare-measurement` | `rigor` | bare-measurement |
| `flt-no-engagement` | `bearing` | no-engagement |
| `flt-referent` | `bearing` | referent |
| `flt-partial` | `bearing` | partial |
| `flt-narrower` | `bearing` | narrower |
| `flt-transfer` | `bearing` | transfer |
| `flt-conditional` | `bearing` | conditional |
| `flt-pending` | both | pending |

The ids are the criteria's own names under the repository's `flt-[a-z0-9-]+` grammar, so the id spells
the fault and no table has to be memorised to read one line. The retired letter alphabet was justified
by three properties: disjointness, walk order encoded into the letter's rank, and brevity. The first is
kept here and the price of the other two is small and worth stating. Walk order is no longer visible in
the id, so precedence (§3.6) is a rule the ask states and the ledger can check after the fact rather
than a rank a reader computes; and brevity is gone, which is the one operational cost — a name costs a
few tokens where a letter cost one (§9). What the name buys is that a fault's identity survives
outside this document: a ledger line, a review note and a template's criterion all use the same string,
and nothing depends on a letter table's version.

The ids carry no severity and no order. A table of the weights they carry is the pass's table and lives
only there (§4.4, §5.4).

**3.9 The parse contract.** The parser implements exactly this, and the ask states it so the model knows
what will be read.

- **The line.** `flt-id: citation` — an id, a colon, and the span. Leading and trailing whitespace is
  trimmed. The brackets of the format sketch are delimiters, not characters: a line may carry `[…]:`
  and `[…]` around the two fields or not, and a bracket pair around a whole field is stripped before
  the field is read. A span that itself begins with `[` or ends with `]` is written unbracketed.
- **The reply is a list of lines.** One line per occurrence. Blank lines are ignored; any other line
  that is not a charge line — an explanatory sentence, a heading, a preamble — is `malformed`. Nothing
  is read out of prose.
- **The id.** Case-folded and matched against the call's closed set: `rigor` accepts the six `rigor` ids
  and `flt-pending`, `bearing` the six `bearing` ids and `flt-pending`. Recorded in the id's own
  spelling, lower case.
- **The citation.** A verbatim span of the closed text. The lookup collapses runs of whitespace to one
  space and folds case, so a re-wrapped or re-capitalised quotation is found; the citation is recorded
  as the model wrote it, with the offset of its first match. The span must lie inside **one passage** of
  the closed text — for `rigor`, the title or the statement; for `bearing`, the source's or the target's
  title or statement. A span in the edge's `## Context` is not a citation for a charge: the context
  states bridges and excuses, and a charge is about what the two texts do and do not do.
- **Which end a `bearing` citation comes from.** Either end, because a bearing fault's evidence can lie
  at either: `flt-referent` needs the term as one text uses it, `flt-partial` the part the source leaves
  alone, `flt-transfer` the two subjects. As proposed here, a span found at *both* ends was refused,
  since the answer then did not say which text it is evidence of, and the model cited a longer span that
  was unique to its end; the recorded passage said which end, so a reader never had to tell — the
  charge's own record made the attribution. The scheme records `both` at the first match instead. For
  `rigor` the two passages are one node's own text, so a span found in both is recorded at its first
  match and costs nothing.
- **The floor on a span.** As proposed here, at least three whitespace-separated words after that
  normalisation; the scheme's floor is one character of text. The floor was to stop an anchor from being
  a single token (`the`) that would be found in any text; it could not stop an anchor from being three
  words of boilerplate (§9).
- **Order.** Free. Charges are recorded in the order written, and nothing in the arithmetic depends on
  it.
- **The empty answer.** `none`, alone. An empty reply and a reply of whitespace only are not answers:
  `malformed`, nothing written.
- **A duplicate charge.** The same id with the same span, twice, is one occurrence written twice: the
  identical pair collapses, because the answer's own evidence cannot tell the two lines apart.
- **An overlapping span for one id.** Two lines of one id whose spans overlap without coinciding are
  `malformed`, naming both lines. The reply asserts two occurrences and supplies one site, so the count
  cannot be read from it; a retry settles it, where reading it as one would write a confidently high
  value — the asymmetry the `flt-pending` rule below rests on.
- **One span under two ids.** Accepted, both charged, and the shared offset stands in the ledger. Two
  distinct faults may share a sentence, and the one-defect rule is the walk's, not the parser's (§3.6).
- **An unknown id.** An id outside the call's set — the other pass's id, a letter from the retired
  alphabet, a misspelling — is `malformed`, and the refusal names the id and the pass's ids as
  `allowed`.
- **A missing citation.** An id with no separator, or with nothing after the colon, is `malformed`. The
  one exception is `flt-pending`, which takes no citation because there is no proposition to point at;
  anything after it is surplus (§6.3).
- **A citation not found.** A citation that does not appear in the closed text — including one that
  crosses a passage boundary, one in the context, and one cut short by a truncated reply — is
  `malformed`, naming the id and the span. The span is recorded with it, so the failure is legible.
- **A number.** A decimal, a fraction, `0`/`1`, or a `*pending*` literal written as an answer is
  `malformed`: a bare number has no id and no citation, and a reply that offers one is answering a
  question that is no longer asked.
- **`flt-pending` beside charges.** A reply carrying `flt-pending` is recorded as `[flt-pending]`, and
  every other line in it is read as surplus. Reason: `flt-pending` claims there is no proposition to
  judge, which entails both that no fault is findable and that no clean judgement was made, so the other
  lines cannot be right on its own terms; and the asymmetry of the error decides the direction — a
  needless `*pending*` leaves a value unknown and visible, while a charge on a text that holds no
  proposition writes a confident number. Making `flt-pending` costly would push a hedging model toward
  charging faults instead, which is the worse mistake. The reply is kept verbatim in the ledger, so a
  run that hedges this way is attributable after the fact.
- **`none` beside charges.** `malformed`. Here both halves are claims about faults and they contradict
  each other outright; the parse has no ground to prefer either, and choosing one would be inventing the
  judgement rather than reading it.
- **No count is enforced.** The parser bounds no id's occurrences. §3.10 is the model's instruction and
  the reader's check, and the citation rule is the only mechanical pressure on a count: one occurrence,
  one span.

A `malformed` reply is the pass's existing outcome: nothing written, the reply recorded, the item
re-asked on `--resume`. Two hazards live at the edges of this contract. A reply cut off at the attempt's
bound loses the lines it did not reach, and where the cut falls between lines the remainder parses
cleanly — a silent under-charge, which the parser cannot see and the ledger's attempt record can
(§10.4). And a citation that is legal but irrelevant is accepted, so a charge can be anchored to words
that do not earn it; that is the residual §9 states.

**3.10 The occurrence unit, per fault.** The count is the judgement's, so each id needs a unit: what
one occurrence is, and how many a text can carry. The second column is what the ask states and what a
reader checks; the parser enforces no count (§3.9), and a charge's site is its span, so two occurrences
of one id need two disjoint spans.

| fault id | one occurrence is | a text carries |
|---|---|---|
| `flt-unaddressed` | the text, whose test is universal over it | at most one |
| `flt-contradiction` | one minimal set of statements that cannot all hold | as many sets |
| `flt-overreach` | one respect in which the narrowest established proposition fails to entail the assertion — scope, modality, or a value for a case the text does not cover | at most three, the criterion naming three respects |
| `flt-claimed-step` | one marked inference with no displayed statement the conclusion follows from | as many markers |
| `flt-unstated-premise` | one **distinct** proposition a displayed step needs and the text does not state | as many propositions |
| `flt-bare-measurement` | one reported value with no displayed method | as many values |
| `flt-no-engagement` | the link, whose test is universal over the target | at most one |
| `flt-referent` | one shared term the two texts use differently and that carries the link | as many terms |
| `flt-partial` | one part of a compound assertion left unengaged | as many parts |
| `flt-narrower` | one dimension in which the source's scope is narrower than the assertion's | as many dimensions |
| `flt-transfer` | one unstated bridge the link runs through | as many bridges |
| `flt-conditional` | one unstated condition the bearing depends on | as many conditions |

Two of these units are load-bearing and worth naming. `flt-unstated-premise` counts *distinct
propositions*, not steps: one missing premise deployed at three steps is one hole and one charge, where
a step-by-step count would penalise a text for reusing a gap it does not have. `flt-contradiction`
counts a *minimal* inconsistent set: three mutually incompatible statements are one contradiction, not
three pairs, and a pairwise count would floor a text that made one mistake three times over.

**3.11 The arithmetic, stated once.** For a charge list `C` in a pass whose maximum is `M` and whose
ids carry weights `w`:

```
value(C) = max(0, M − Σ w(id(c)) for c ∈ C)
value([flt-pending]) = *pending*
value(none) = M
```

The floor is the domain's, not a rule of its own: a list whose weights reach `M` is a text with nothing
left to carry the assertion, and `0` is what the register says for that. Every weight is a multiple of
`0.05`, so every value is a two-decimal number and nothing rounds (§8.5).

## 4. The rigor rubric

`rigor` is one node's own worth: what the node's text displays, against what the node asserts. It is
asked of a claim, an analysis and an experiment — never of a work, whose standing is judged with the
document in hand, and never of an axiom, whose `rigor` is `1.00` by rule.

**4.1 Maximum: `0.95`, reached by `none`.** No authored node may carry `1.00` (§8.1), so the top of the
authored table is the largest multiple of `0.05` below it. `none` is the only answer that reaches it,
and the table has no weight producing `1.00` from a charge, so the reserve is structural here and not
only a convention of the ask. `0.95` also clears the register's own top band with room to spare, and the
arithmetic of a chain still lands in that band at ordinary bearings (§8.2, §8.3).

**4.2 Pending first.** Where the title-and-text pair holds no proposition, the answer is `flt-pending`
and no other id applies (§6).

**4.3 The criteria.** Walked in this order; the id is the criterion's.

---

**`flt-unaddressed` — `unaddressed`: the text does not address the assertion.**

*Met when* no sentence of the text, the assertion itself aside, bears on what the assertion claims: the
passage is about another subject, or is boilerplate that could sit under any node, or defers to material
the text does not show. One occurrence at most: the test is universal over the text, so a second charge
would need a second text.

*Not met when* the text addresses the assertion and argues badly — that is one of the criteria below;
or when the text displays no proposition at all, which is `flt-pending` and not a fault.

---

**`flt-contradiction` — `contradiction`: the text cannot all be true.**

*Met when* two statements the text makes cannot both hold and the assertion rests on at least one of
them, or the assertion is incompatible with a statement the same text makes. One occurrence per minimal
set of statements that cannot all hold (§3.10).

*Not met when* the text states a limitation, a counter-case or a range the assertion already names — a
stated scope is not a contradiction; or the two statements are in tension that a condition the text
states resolves.

---

**`flt-overreach` — `overreach`: the assertion claims more than the text shows.**

*Met when* the narrowest proposition the text establishes does not entail the assertion: a wider scope
(more objects, regimes, parameters or populations), a stronger modality (certainty for a possibility, a
cause for an association), or a value stated for a case the text does not cover. One occurrence per
respect in which the entailment fails, so a title that widens the scope *and* strengthens the modality
carries two (§3.10). Where the title is wider than the body, this id carries that shortfall and no other
does.

*Not met when* the text's own statement of the assertion is the same proposition in other words; the
test is entailment, not resemblance, and a shortened title that entails nothing more does not meet it.

---

**`flt-claimed-step` — `claimed-step`: a step is claimed and none is shown.**

*Met when* the text marks an inference — *hence*, *it follows*, *therefore*, *clearly*, *one can show*
— and displays no statement from which the asserted conclusion follows. One occurrence per marker so
placed; the marker is the citation, and the absence is checked against the whole text.

*Not met when* the text states a conclusion and claims no step for it (a bare assertion, or a result
attributed to a source); or a step is displayed and `flt-unstated-premise` applies instead.

---

**`flt-unstated-premise` — `unstated-premise`: a displayed step rests on something the text does not
state.**

*Met when* the text displays a step — an inference, or a computation from stated quantities — and that
step does not follow unless a proposition the text does not state is granted. One occurrence per
distinct proposition, not per step that needs it (§3.10).

*Not met when* the missing proposition is a node of the graph the text names, or a premise the node's
edges carry: a premise's applicability to the assertion is the edge's `bearing`, judged in the other
pass, and is never a `rigor` fault. Nor is it met where the step follows from what is displayed, however
much more a reader would want.

---

**`flt-bare-measurement` — `bare-measurement`: a number is reported and no method is.**

*Met when* the text reports a measured or computed value and displays no procedure, sample, instrument,
dataset or calculation that yields it. One occurrence per value; a table of six numbers with no method
is six charges.

*Not met when* the value's provenance is displayed in the same text; or the value is a stated constant,
a stipulated parameter or a definition; or the number is a reference — a table, a clause, a year —
rather than a result.

---

The three gap ids — `flt-claimed-step`, `flt-unstated-premise`, `flt-bare-measurement` — carry one
weight by §3.7: each names the same defect at a different site — a gap between what the text
displays and what it asks the reader to take from it — and moving a case from one to another must not
move the value. `flt-overreach` is a defect of a different site, the assertion's reach rather than the
text's chain, and `flt-contradiction` a different one again. `flt-unaddressed` is not on that scale at
all: it is the judgement that no sentence bears on the assertion, which leaves the others nothing to be
about.

**4.4 The table.** `M = 0.95`. The weight is per occurrence, and `value = max(0, 0.95 − Σ w)` over the
charge list.

| fault id | weight per occurrence |
|---|---|
| `flt-unaddressed` | `0.95` |
| `flt-contradiction` | `0.45` |
| `flt-overreach` | `0.35` |
| `flt-claimed-step` | `0.30` |
| `flt-unstated-premise` | `0.30` |
| `flt-bare-measurement` | `0.30` |

Summed, the reachable values are these; the first column names one charge list that reaches each,
"gap" standing for any of the three gap ids.

| charges | sum | value |
|---|---|---|
| none | — | `0.95` |
| one gap | `0.30` | `0.65` |
| one `flt-overreach` | `0.35` | `0.60` |
| one `flt-contradiction` | `0.45` | `0.50` |
| two gaps | `0.60` | `0.35` |
| a gap and an `flt-overreach` | `0.65` | `0.30` |
| two `flt-overreach` | `0.70` | `0.25` |
| a gap and an `flt-contradiction` | `0.75` | `0.20` |
| an `flt-overreach` and an `flt-contradiction` | `0.80` | `0.15` |
| three gaps, or two `flt-contradiction` | `0.90` | `0.05` |
| an `flt-unaddressed`, or any sum of `0.95` or more | `≥ 0.95` | `0.00` |
| `flt-pending` | — | `*pending*` |

Eleven values over `[0, 0.95]`, and nine of the twenty multiples of `0.05` in that range are
unreachable — `0.10`, `0.40`, `0.45`, `0.55`, `0.70`, `0.75`, `0.80`, `0.85`, `0.90`. Nothing lands
there, so a value off this list is a defect of a landing, not a judgement.

**4.5 The values, and where they land.** Against `kb.BuildBandLadder` (`internal/kb/schema.go:32-38`),
whose cuts are `0.85`, `0.65`, `0.45`, `0.20` and `0.00`, inclusive at the bottom of each band:

| charges | rigor | band it would read, when rigor is what binds |
|---|---|---|
| none | `0.95` | ok to build on |
| one gap | `0.65` | ok to build on, see caveats — exactly at the cut |
| one `flt-overreach` | `0.60` | use as input only |
| one `flt-contradiction` | `0.50` | use as input only |
| two gaps | `0.35` | do not build on, rework needed |
| a gap and an `flt-overreach` | `0.30` | do not build on, rework needed |
| two `flt-overreach` | `0.25` | do not build on, rework needed |
| a gap and an `flt-contradiction` | `0.20` | do not build on, rework needed — exactly at the cut |
| an `flt-overreach` and an `flt-contradiction` | `0.15` | unsubstantiated — §8.6 |
| three gaps, or two `flt-contradiction` | `0.05` | unsubstantiated — §8.6 |
| an `flt-unaddressed`, or a saturated sum | `0.00` | unsubstantiated — §8.6 |

The bottom rung's wording is §8.6's; the code then still read `refuted, do not use` until the ladder
changed, and the band's slug stayed `refuted` until then.

Three cautions on reading that column. The band reads **strength**, not `rigor`; `rigor` is one operand
of the minimum, so a node's band can sit below its rigor because of a premise. `rigor` is **not an
upper bound on strength**, because a lift station stands above the derivation, so a shoddy node can
publish a high strength on a strong lifter. And the placements are deliberate: one gap is meant to be
"caveats", one defect in the assertion "input only", two independent defects "do not build on" — while
everything below `0.20` reads "unsubstantiated", which is an outcome and not a cause (§8.6).
Counts now move a text across a cut on their own: three gaps sit a whole rung below two, and two
contradictions sit at `0.05` where one sits at `0.50`.

**4.6 Cases.**

*Clean.* Title: "The deposition rate rose with substrate temperature in four runs between 300 °C and
400 °C". Body: "In four runs at constant pressure, the rate was 12, 14, 17 and 19 nm/min at 300, 335,
370 and 400 °C. No run departed from monotone order." No criterion is met: the assertion is exactly
what the body establishes, the body gives the method and the values, and the step from them to the
windowed statement needs nothing unstated. Answer `none`, value `0.95`. With one premise at bearing
`0.90` and source strength `0.95`, the contribution is `0.855`, and `min(0.95, 0.855) = 0.855` puts the
node in the register's top band.

*Borderline — the same body under a wider title.* Same body, title: "Substrate temperature raises the
deposition rate", with no window. The assertion is the title's, so the narrowest proposition the body
establishes does not entail it: `flt-overreach` is met, one charge, value `0.60`. Suppose the body's
step also needs something it does not state — that four points bracket the range and are not four
favourable samples. `flt-unstated-premise` is met as well, and the answer is two charges, value `0.30`.
The rule that separates the two is the site: `flt-overreach` is a shortfall between the assertion's
reach and the text's, `flt-unstated-premise` a hole inside the text's own step. Both are charged when
both are present, because they are two defects at two sites.

This is also where the scheme's residual variance lives, and it is worth stating rather than hiding. A
reader who holds that the missing bracketing premise *is* the general title charges `flt-overreach`
alone and `0.60`; a reader who holds they are two charges both and `0.30`. The quotation rule is the
whole defence: each charge must be anchored at the sentence it is about, and where one sentence
satisfies two criteria there is one defect and the earlier criterion's id is the charge. Across sites
the weights cannot make the disagreement free; within a site they can, which is why §3.7 exists.

*Three numbers, three charges.* A text reporting three computed rates and no method displays three
occurrences of `flt-bare-measurement` — one per value, each citing the clause that reports it — summing
`0.90`, value `0.05`, bottom rung. The retired letter set answered the same text with one letter and
`0.65`: it could not tell one bare number from three, and this is the case the per-occurrence rule
exists for. A parameter and a reference in the same sentence are not charges (§4.3's *not met when*),
so the count is the number of *reported results*, not of numerals.

*Pending against `flt-unaddressed`, which the walk does not confuse.* A claim titled "Deposition
behaviour" whose body is "See `wrk-w00000`." — the pair holds no proposition, so `flt-pending`, value
`*pending*`. A claim titled "The deposition rate is pressure-limited" whose body is "Runs were performed
at 5 Torr; the chamber was cleaned between runs." — an identifiable assertion, and no sentence bears on
it: one `flt-unaddressed`, value `0.00`. The two differ in the walk exactly as §6 states: `0.00` is a
judgement that participates, and `*pending*` is the absence of one.

## 5. The bearing rubric

`bearing` is one scoring edge's scalar: how far the source's content speaks to the target's assertion.
The source is taken as given — true as it states itself — and neither its soundness nor the target's
wisdom in citing it is judged here. `bearing` weights the source's contribution; it never modifies the
source's value.

Read against the anchor words: **fully on point** is every part of the target's assertion engaged, so
`none` (`1.00`); **off point** is no part engaged (`flt-no-engagement`, `0.00`); **partly on point** is
one or more charges, and it is the id that says in what way the reach falls short.

**5.1 Maximum: `1.00`, reached by `none`.** The `1.00` that is reserved by rule is `rigor`'s alone: an
axiom's `rigor` is `1.00`, and no authored node carries it. `bearing` has no such reserve, and the field
already spells `(bearing 1.0)` for a premise bullet that applies wholly — a conversion writes it where a
bullet carried no token (§8.4). A bearing maximum below `1.0` would make "wholly on point" unreachable
while that spelling stood in the same field for the same sense — one field, two meanings. Hence the
maximum, and `none` is the answer that reaches it, and every charge is a deliberate judgement that part
of the target's assertion is left unengaged.

**5.1a What the call shows, and what a citation is evidence of.** The bearing call carries both ends'
title-and-statement as the closed-text rule renders them, plus the edge's authored context. A work
renders its title alone — its document lies outside the corpus — so a link from a work is judged on the
title's proposition, and a title naming no subject is `flt-pending` (§6.2). That limit belongs to the
closed text, not to the ids; it is stated here so the charges are not read as having failed to notice
it.

A span is evidence of what **one text states**. A bearing fault is a claim about a relation, so a single
span can only show one end of it: the term as one text uses it, the part the target asserts that the
source does not reach, the subject the source is about. A charge therefore cites either end (§3.9), the
parse records which end the span was found in, and the reader holding the charge has the other end in
the same closed text, under its own `## Source:` or `## Target:` label. No line names its end, because
the record does: the attribution is mechanical, and a model that has to choose a span unique to its end
cannot leave the question open.

**5.2 The engagement test, and its blindness to sign.** A part of the target's assertion is *engaged*
when, taking the source as given, believing it would change what one should think about that part — in
either direction. Support and refutation are both engagement: a refuter's bearing is how much of the
target it actually touches, and the cap the walk computes from it (`1 − bearing × strength`) is
meaningless if bearing were read as agreement. This test is the whole of §5; every criterion narrows it.

**5.3 The criteria.** Walked in this order.

---

**`flt-no-engagement` — `no-engagement`: nothing in the source speaks to the assertion.**

*Met when* for every part of the target's assertion the source states nothing that would change what
one should think of that part, in either direction. The common causes: the source is about a different
subject, or it shares the subject matter and states nothing that reaches the assertion — an existence
result where the target asserts a magnitude, a method where the target asserts a value. One occurrence
at most: the test is universal over the target.

*Not met when* the source engages one part only (`flt-partial`), or reaches the assertion only through a
scope or a condition that a criterion below names. Sharing a vocabulary with the target is not
engagement.

---

**`flt-referent` — `referent`: the link rests on a term the two statements use differently.**

*Met when* source and target share a term, that term is what appears to carry the link, and the two
statements use it for different referents or under different definitions. One occurrence per term.

*Not met when* the two use the term identically; or the link survives without the shared word, being in
the content rather than the vocabulary; or, the target's sense substituted for the source's, nothing at
all is engaged — that is `flt-no-engagement`.

---

**`flt-partial` — `partial`: the source engages part of a compound assertion.**

*Met when* the target's assertion names more than one part — a conjunction, a list of conditions, two
quantities asserted together — and the source engages at least one part and not all. One occurrence per
part left unengaged.

*Not met when* the assertion names one thing to establish, however qualified: a scope narrower than
that one thing is `flt-narrower`, not this; or the source engages every part.

---

**`flt-narrower` — `narrower`: the source reaches the assertion only within a narrower scope.**

*Met when* the target's assertion quantifies over a class, a range, a population or a set of conditions,
and the source's content holds for part of it — one member, one regime, one value of a parameter the
assertion ranges over. One occurrence per dimension in which the source's scope is narrower.

*Not met when* the assertion itself is scoped to that part; or the source's content is general and the
assertion the special case, which is full engagement.

---

**`flt-transfer` — `transfer`: the link runs through a step neither text states.**

*Met when* the source's subject differs from the target's, and what makes the source bear on the target
is an unstated bridge — an analogy, a scaling, *the same holds for*, a change of unit or of definition.
One occurrence per bridge.

*Not met when* the two statements share their subject and their terms; or the bridge is stated in the
source, in the target, or in the edge's own context.

---

**`flt-conditional` — `conditional`: the source applies only under a condition it does not state.**

*Met when* the source's content is stated under conditions, and its bearing on the target depends on a
condition the source does not give — a regime, a temperature, an assumption — that the target's
assertion needs. One occurrence per condition.

*Not met when* the condition is stated; or the target's assertion needs no such condition.

---

`flt-referent`, `flt-partial` and `flt-narrower` carry one weight because each leaves about half of the
target's assertion unengaged; a reader who reads a case as one of the three records a different id and
the same value. `flt-transfer` and `flt-conditional` carry one weight because both say "the source
reaches the assertion, but only by a step the text does not supply", and differ only in what the step
supplies.

**5.4 The table.** `M = 1.00`. The weight is per occurrence, and `value = max(0, 1.00 − Σ w)` over the
charge list.

| fault id | weight per occurrence |
|---|---|
| `flt-no-engagement` | `1.00` |
| `flt-referent` | `0.50` |
| `flt-partial` | `0.50` |
| `flt-narrower` | `0.50` |
| `flt-transfer` | `0.35` |
| `flt-conditional` | `0.35` |

Summed:

| charges | sum | value |
|---|---|---|
| none | — | `1.00` |
| one `flt-transfer` or `flt-conditional` | `0.35` | `0.65` |
| one of `flt-referent`, `flt-partial`, `flt-narrower` | `0.50` | `0.50` |
| two of `flt-transfer`, `flt-conditional` | `0.70` | `0.30` |
| one half-weight and one step-weight charge | `0.85` | `0.15` |
| two of `flt-referent`, `flt-partial`, `flt-narrower` | `1.00` | `0.00` |
| an `flt-no-engagement`, or any sum of `1.00` or more | `≥ 1.00` | `0.00` |
| `flt-pending` | — | `*pending*` |

**5.5 The values.** Six over `[0, 1.00]`, and the same six the letter set produced — bearing's weights
are coarse enough that a second half-weight charge, or a half plus a step, reaches the floor before any
new value appears. What the per-occurrence rule changes here is how a text gets to `0.00`: two unengaged
parts of a compound assertion and an unrelated source are the same value and no longer the same answer,
and the record says which. Bearing needs fewer values than `rigor` because it multiplies rather than
caps: `0.65` against `0.50` is a difference of `0.15 × strength` in the contribution, which the walk
carries rather than rounds away. There is no band column here: bands read strength, and bearing reaches
one only through a product.

**5.6 Cases.**

*Narrower.* Source: "For copper at 300 K the coefficient is 0.4." Target: "The coefficient exceeds 0.3
for metals at 300 K." `flt-narrower`: the assertion ranges over a class, the source reaches one member.
Value `0.50`, and against a source strength of `0.95` the contribution is `0.475` — the model's own
statement that one measurement of one metal is half a reason for the general claim. A target asserting
the coefficient for three metals, of which the source reaches one, charges `flt-partial` twice — one
occurrence per part left unengaged — for `1.00` and `0.00`, where the retired set charged the same
defect once and read `0.50`. That is the count doing work a set could not do, and it is also why §3.10
pins the unit to parts rather than to assertions.

*Borderline — `flt-referent` against `flt-no-engagement`.* Source: "The stress in the sample rose with
strain." Target: "The stress reported by participants rose with workload." The word *stress* carries the
apparent link and the referents differ. If some part of the target's assertion survives the substitution
— say the target also asserts that the rise was monotone, which both statements are about —
`flt-referent` is met and the value is `0.50`. If nothing survives, `flt-no-engagement` is met and the
value is `0.00`. The disqualifier decides it, and the disqualifier is a test on the substituted reading,
not an impression of the pair.

*A refuter, where sign-blindness has to hold.* A refuter of strength `0.60` on an edge whose bearing the
table puts at `0.50` caps the target at `1 − 0.30 = 0.70`; the same refuter at `1.00` (`none`) caps it
at `0.40`. Nothing in either judgement says the source is right or the target wrong: the charges measure
how much of the target the refutation touches, and the walk turns that into a limit.

## 6. `flt-pending`, and what it now means

`flt-pending` is not "I am unsure". It is **the text does not support judging at all**: no proposition
is identifiable, so there is nothing to judge, and no other id applies. It is an id like the rest — it
occupies the answer, it has a table row, and its value is `*pending*` (§3.11). What it is not is a hedge
chosen in place of a judgement that could have been made. It is the one id that takes no citation,
because there is no sentence to point at: the fault it names is that the text holds nothing.

The distinction is load-bearing in the walk and in opposite directions. A pending `rigor` leaves the
node's derivation with no number, so a lift over it still publishes the lift's product and a cap over it
publishes nothing. A pending `bearing` excludes its edge — no contribution, no lift, no cap. A `0.0`, by
contrast, is a judgement and participates: a `0.0` bearing takes a minimum to zero and a cap to `1.0`,
and a `0.00` rigor takes a minimum to zero. Never judged and judged-worthless are different states, and
an answer of `flt-pending` where a judgement was possible leaves a hole in the graph that no query can
see.

**6.1 When `rigor` is `flt-pending`.** The title-and-text pair holds no proposition:

- a title that names a topic rather than asserting, with no statement under it;
- a section that is empty, a stub, or a placeholder ("TBD", "—", "to be written");
- a statement that asks a question, lists keywords, or points at material the call does not show, where
  the title names a topic rather than asserting.

Nothing else. A text that holds a proposition and is about another subject is not pending: it is one
`flt-unaddressed` and answers `0.00` (§4.6). A text that holds a proposition and argues badly is not
pending either: it is a list of charges against it.

**6.2 When `bearing` is `flt-pending`.** Either end holds no proposition by §6.1's test. A source whose
text is its title alone is pending where the title names no subject ("A Cited Work"); where the title
states a subject, the criteria apply to what the title states, since that is the text shown. A link
whose source and target both hold propositions is never pending: a judged-irrelevant link charges
`flt-no-engagement` and answers `0.00`, and a hedged `flt-pending` on a link that can be judged removes
the edge from the graph silently.

**6.3 `flt-pending` beside charges, and `none` beside charges.** The first is accepted with the other
lines read as surplus; the second is `malformed`. §3.9 gives the reasons and the record each leaves
behind.

## 7. The ask: what it must carry, and what stays out

The answer is a list of lines, each an id and a citation. The reply's shape is fixed by
`score.ParseAnswer` and the text's closure by `scoreTokenRE`, the four-pattern guard in
`internal/score/text.go` that keeps the KB's own scoring tokens (`- rigor:`, `(bearing`, `- strength:`,
`(strength `) out of the text a call reads. The first changes when this lands — SPEC §8's "A reply is
one line holding a decimal in `[0, 1]` or `*pending*`" becomes "a reply is a list of lines, each a fault
id and a citation from the call's set, or `none`, or `flt-pending`" — and the second does not: no id and
no citation ever enters a register, so the guard's closed text is exactly what it is today (§1). The
guard is also what makes the citation rule sound: the span a charge quotes is a span of the same closed
text, so no citation can reach the register's own score fields.

**7.1 What the ask must contain.**

- The pending test first, before any criterion, as a closing of the question rather than a hedge (§6).
- The assertion rule: the title fixes what the node asserts, and a shortfall in the text is a fault
  rather than a redefinition (§3.2).
- The scope statement: truth is out of scope; premises carried by edges are out of scope; the other
  pass's quantity is out of scope (§3.3, §4.3's `flt-unstated-premise`).
- The criteria: id, name, test, near-miss clause and occurrence unit for each, in the order they are
  walked (§3.10, §4.3, §5.3).
- The instruction to charge **every** occurrence it can point at and nothing else — the answer is all
  the occurrences found, not the worst one, and not one per kind (§3.1, §2).
- The citation rule as a rule, not an instruction: one line per occurrence, the span verbatim from the
  text, the span that shows the fault (or, for a fault that is an absence, the span that carries the
  obligation), and what the parse will refuse (§3.4, §3.9).
- The one-defect-one-charge rule with its precedence (§3.6), and that two occurrences of one id need two
  spans that do not overlap (§3.9).
- The ids, `none`, and `flt-pending`; the answer's shape, and what the parse will refuse (§3.9) — so
  that a reply is not lost to a spelling the ask never named.
- For `bearing`, that a citation may come from either end and must name its end uniquely, and that the
  edge's context is not citable (§3.9, §5.1a).
- The explicit list of non-faults: style, length, formatting, spelling, absent citations, absent proofs
  where none are promised (§3.5).

**7.2 What stays out of the ask.** Now this list is shorter by everything numeric, which is the point of
moving the arithmetic into kbase:

- **Any number at all**: no maximum, no weight, no fraction, no percentage, no `0` or `1`, no count to
  aim at, no arithmetic the reader is asked to do. The table is kbase's (§4.4, §5.4).
- The band ladder and its thresholds. With no number in the ask there is nothing to aim at, which
  removes the hazard rather than instructing against it.
- The walk's arithmetic — the minimum, the lift, the cap, the products — and what a value will do
  downstream.
- The old middle sentence, "the share of what the statement claims that the text shown establishes". It
  is gone with the decimal: two procedures in one prompt produce a blend, and the blend is the prior
  this scheme replaces.
- Worked examples drawn from the corpus. They anchor the answer and grow the prompt for no gain; the
  criteria are the teaching, and an example invites copying its charges.
- Any instruction to reward or punish for length, citation count, or the presence of equations. The
  count of charges is the judgement; the count of citations is a consequence of it and must never be
  read the other way.

**7.3 What the answer's form costs the ask.** The criteria are the bulk of it. The two templates run to
about 300 words each today; a table of six criteria with a test, a near-miss clause and an occurrence
unit is roughly 500 more, and the answer's shape and its refusals another 200. The shared `score-system`
fragment carries the answer discipline and not the criteria — the form is shared, the ids are not, and
one splice into both calls cannot hold two tables. One thing the ids buy back: the criteria need no
letter table, because the id is the criterion's name, so the ask names each fault once and the answer
uses the name it was given. One thing they cost: an answer is now as long as the judgement — a text with
twelve occurrences needs twelve lines and twelve spans — so an attempt's bound is a live constraint on a
rotten text in a way a one-token decimal never was (§3.9, §10.4).

**7.4 What the ledger records.** Per attempt: the item, its status, `faults` as parsed, the model's
`reply` verbatim, the `value` landed, the model, the prompt's digest and the attempt's bounds — the same
record SPEC §8 states, with `faults` carrying more than a set. `faults` is one record per charge, in the
order written: the `id`, the passage the citation was found in (the node's `title` or `statement` for
`rigor`, `source` or `target` for `bearing`), the `citation` as the model wrote it, and its `offset` in
the closed text. `[]` for `none`; one record carrying `id: flt-pending` and no citation for pending. The
reply is kept beside it, so the parse's own reading of a line can be held against the line. `--dry-run`'s
`items.jsonl` gains nothing: nothing is asked, so nothing is returned.

Two consequences worth stating here rather than discovering later. The value is a function of the
**ids alone** — their counts, not their spans (§8.8) — so re-scoring reads counts and ignores spans. And
a reply that hit the attempt's bound is not distinguishable from a complete one by its content alone; the
ledger's attempt record is the only place the difference can be kept (§10.4).

## 8. The arithmetic the rubric has to live with

Each item below is settled in [STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md); what follows is
what the table must not contradict, and what the table makes visible.

**8.1 The axiom's `1.00`, and the ceiling.** `kb.AxiomRigor` is `1.0`, an axiom premise contributes
`bearing × 1.0`, and no authored file carries `1.00` (`STRENGTH_SCORING_SCHEME.md`, "The nodes"). The
rigor maximum of `0.95` is the largest value below that reserve, and the table has no weight that
produces `1.00` from a charge: a maximally aimed axiom premise contributes `1.00`, which no charge list
can equal. It does not make axiom premises binding — an axiom at bearing `0.90` contributes `0.90` and
sits below a clean node's `0.95` like any other premise.

**8.2 The ceiling compounds along every chain, and that is not cosmetic.** A `supports` or `founds`
contribution is a product, so a ceiling `m` costs `m^k` over a chain of `k` clean links at bearing
`1.0`. At `0.95`: `0.9025`, `0.85738`, `0.81451`, `0.77378` for two to five links. At `0.90`: `0.81`,
`0.729`, `0.6561`, `0.59049`. With the band ladder's top cut at `0.85` (§8.3), a two-link chain of clean
nodes at bearing `0.90` reads `0.855` under `0.95` and `0.81` under `0.90` — the difference between "ok
to build on" and "ok to build on, see caveats" for a perfectly clean argument, decided by the ceiling
rather than by the argument.

**8.3 The band ladder is a second staircase, and the weights were read against it.** `kb.BuildBandLadder`
(`internal/kb/schema.go:32-38`) cuts at `0.85` ("ok to build on"), `0.65` ("ok to build on, see
caveats"), `0.45` ("use as input only, don't build deeper"), `0.20` ("do not build on, rework needed")
and `0.00` (then "refuted, do not use"; "unsubstantiated" by §8.6), each an inclusive lower
bound. §4.5 gives the charge lists against it. Two cautions, repeated there because they are the
reader's commonest error: the band reads **strength**, not `rigor`, so a node's band can sit below its
rigor because of a premise; and
`rigor` is **not an upper bound on strength**, because a lift station stands above the derivation, so a
shoddy node can publish a high strength on a strong lifter.

**8.4 Bearing `1.00` is a value the register already carries.** A premise bullet that carries no token
takes `(bearing 1.0)` where a register is converted to `2.0.0` (SPEC §10, the conversion rules), and
`set-bearing` accepts `1.0` like any value in the domain. A bearing maximum below `1.0` would make
"wholly on point" unreachable while that spelling stood in the same field for the same sense — one
field, two meanings. Hence §5.1, and `none` is the answer that reaches it.

**8.5 The record's spelling.** Every table weight is a multiple of `0.05`, so every sum and every value
is too: a record's four decimals never lose a digit and a lifted or capped value derived from it is
exact; the value sets of §4.4 and §5.4 are the audit a value can be held to.

**8.6 The bottom rung states an outcome, not its cause.** `kb.BandFor` (`internal/kb/schema.go:45-55`)
sends every strength below `0.20` to the ladder's last rung, whose entry then read
`{"refuted", 0.00, "refuted, do not use", "refuted, do not use"}` (`internal/kb/schema.go:32-38`). That
wording is wrong for the rung it labels. The rung is reached from both directions. By a cap: `bearing
0.0` takes a minimum to zero, and a refuter whose `bearing × strength` exceeds `0.80` caps the node below
`0.20`. By a min: `flt-unaddressed`, three gap occurrences, two contradictions, or any premise
contribution low enough to drag the minimum under `0.20` publishes a strength that is low because
nothing in the register substantiates the claim. A refutation and a text that supports nothing land on
the same rung for one reason — nothing a claim built on it would inherit substantiates the claim — and
how that state arrived is immaterial to a reader deciding whether to build. So the label must state the
outcome and not the cause, and the two must not be told apart by it.

The wording the register should carry is **`unsubstantiated`** — status phrase, label and slug alike.
The rung's cut stays `0.00` and the ladder's
thresholds do not move: this is a change of words, not of arithmetic, and it leaves every strength where
it was. A reader who needs the cause still has it, in the two places it belongs — the fault record, where
`flt-unaddressed` is a judgement about reach and a saturation is a sum of charges, and the derivation
trace, which prints the cap that produced the strength. The label's job is whether to build on the
claim; the trace's job is why it stands where it stands.

**8.7 The ceiling is a discipline of the table, not an invariant of the register.** `set-rigor` accepts
any value in `[0,1]` or the pending literal, `1.0` included, and a converted KB carries `rigor: 1.0` on
every experiment. So `0.95` is the largest value a pass derives and not the largest the register can
hold; a hand-authored `1.0` remains writable and would read as an axiom's value in any query.

**8.8 The value is derivable, and that is the audit.** A landed value is a function of the recorded
charges — in fact of their ids alone:

```
value = table(counts by id)
```

Three things follow, and they are the gain the arithmetic's move buys.

- **A landing can be checked without a model.** A value that does not equal the table's output for its
  recorded ids is a defect of the landing or of the table, not a judgement to argue with. Counts are
  integers, so the check is exact.
- **A run can be re-scored without a model.** Change a weight and re-derive every value in a ledger by
  counting ids; no call is made and no judgement is re-asked. The durable record is the count per id;
  the value is a projection of it under one table (`run.yaml`'s). The spans are not needed for this, and
  that is why they are the audit rather than the input: they can be read, checked against the text and
  argued about, and they cannot perturb the arithmetic.
- **Comparisons belong on counts.** Two runs under different tables agree or disagree on the judgement
  exactly when their counts per id match; their values are comparable only within one table. A value
  agreement is weaker than a count agreement: `0.35` is reached by two `flt-bare-measurement` and equally
  by one `flt-claimed-step` and one `flt-unstated-premise`, so a sum hides both the split and the
  identity, and the reproducibility §9 asks for is measured on counts, not values.

**8.9 Resume, and the table's identity.** `--resume` skips every `done` item, so a run resumed across a
table change would otherwise mix two scales. It no longer has to: a run's values can be re-derived from
its charges, so the landing at the end of a resumed run is a re-derivation under the table in force, and
the mixing is repairable rather than fatal. What that needed was the table's identity in `run.yaml`, so
that "which table were these derived under" is answerable and re-derivation is a decision rather than a
guess. The per-occurrence table makes this sharper than the letter table did: a count multiplied by a
changed weight moves a value further than a membership test did, so a resume across a change is a larger
error and a cheaper repair. Whether `run.yaml` recorded it, and whether a resume across a change refused
or re-derived, was open here; the scheme settled it the other way — the run records the ask, not the
table, and a resumed run re-prices.

## 9. What the scheme claims, and what it does not

It claims **reproducibility**, not accuracy. The ids do not make `rigor` and `bearing` measure the things
they name; they make two careful readers, and two runs of the same model, return the same charges for the
same text, and they make the reason for a difference nameable — an id, a count, and a span to look at.

**What the form refines, and what it coarsens.** The value is coarse and the record is not. `rigor` holds
eleven values and `bearing` six, and each is the projection of unboundedly many charge lists: two texts
that share a value may differ in the *kind* of fault, in the *number* of occurrences, and in the sites —
and all of that survives in the ledger. Discrimination therefore lives where the division of labour puts
it, in the charge list, and the value is the single number the walk consumes. The one thing the retired
letter set could not represent at all, and this form can, is multiplicity: three holes in one chain are
three deductions, and a text with one hole in three places now scores below a text with one.

**The two places the charge list is weaker than the decimal was.**

1. **Nothing unnamed can be expressed, and nothing unnamed carries a magnitude.** The vocabulary is
   closed: twelve ids and the pending id. A reader who can see that a text is worse than any id names, in
   a way the ids do not capture, has only the nearest id, `none`, or `flt-pending` — and `none` reads as
   the maximum, which is the wrong direction. Severity exists only as a count, and only where the fault
   is occurrence-countable: three bare numbers cost `0.90`, while one wild overreach and one trivial
   overreach both cost `0.35`. A decimal could land at `0.42` for a defect nobody has named yet. The
   price is paid knowingly: the alternative is an open vocabulary whose terms two runs would spell
   differently, and the whole scheme exists to stop that.
2. **The citation reads as evidence and is only an anchor.** A verbatim span in a landed record looks
   like a checked quotation, and the check is only that the words are there. It cannot tell whether the
   span earns the id, and for the absence faults — `flt-unaddressed`, `flt-claimed-step`,
   `flt-bare-measurement`, `flt-unstated-premise`, and every bearing fault that names what the source
   leaves unsaid — no span could be evidence of the fault at all, so the citation anchors the obligation
   and the absence is nobody's check. A decimal made no such promise: it was a bare verdict, and a reader
   knew to hold it against the text. The new form invites the opposite reading, and a reader who takes
   the span for proof is worse off than one who took the decimal for a verdict. §3.4 states the limits;
   §8.8 states what is genuinely machine-checkable, which is the value and not the judgement.

**The acceptance test.** Two runs of the same model over the same set, with the same template, return the
same **count per id** for every item. Counts, not spans: two careful readers may quote different
sentences for one occurrence, and a boundary choice is not a disagreement about the text. Counts, not
values: a value agreement can hide a disagreement between ids, and a count disagreement is exactly the
disagreement this scheme is answerable for. Where the counts differ, the spans say where — one reader
saw two occurrences where the other saw one, and the two spans show whether the second is real. That
segmentation question is the residual the per-occurrence rule creates: the old set had no count to
disagree about, and the old decimal had no structure to disagree about, so the new form moves variance
from "which letter" to "how many" and makes it locatable rather than removing it.

## 10. Open questions

These are settled before a template changes.

1. **The ids.** Are these twelve names the frozen vocabulary — `flt-unaddressed`, `flt-contradiction`,
   `flt-overreach`, `flt-claimed-step`, `flt-unstated-premise`, `flt-bare-measurement`,
   `flt-no-engagement`, `flt-referent`, `flt-partial`, `flt-narrower`, `flt-transfer`,
   `flt-conditional` — with `flt-pending` shared? The argument for them is that the id spells the
   criterion, that the two passes stay disjoint by name, and that a ledger line needs no table; the cost
   is tokens (§3.8). If an id changes after a run, the ledger's record moves with it.
2. **The parse's strictness.** Four recommendations: the three-word floor on a span; `flt-pending`
   accepted with surplus lines, which costs no retry; `none` beside charges refused; and an overlapping
   span for one id refused rather than collapsed to one charge. The three refusals each cost a retry. Is
   that the split to freeze? The floor is the weakest of the four — it stops a one-token anchor and
   nothing else — and the overlap refusal is the one that decides a value by refusing it.
3. **The table's numbers.** The six `rigor` weights (`0.95`, `0.45`, `0.35`, three at `0.30`) and the
   six `bearing` weights (`1.00`, three at `0.50`, two at `0.35`) are a scale read against the band cuts
   (§4.5). Changing one now costs one re-derivation and no calls (§8.8) — but with a count multiplying
   each weight, a change to a weight moves values by the count and not by one step, so a weight is a
   bigger decision than it was under a set: who may change one, and what marks the table's identity in a
   run?
4. **The ledger's charge record, and truncation.** `faults` carries the id, passage, citation and offset
   per charge (§7.4). Confirm the record is kept for the life of the ledger, that a run's record is not
   trimmed to the ids on the way to a report, and — the new hazard — that a reply which stopped at the
   attempt's bound is recorded as such, since a reply cut between lines parses cleanly and under-charges
   silently (§3.9). Is the attempt's finish reason enough to refuse such an item, or is it an operator's
   finding?
5. **The table's identity and resume.** This page asked whether `run.yaml` should carry the table's
   version, and whether a resume across a change should refuse or re-derive at the end (§8.9), and what
   `counts` reported while the two were mixed. The scheme settled it: the run records the ask, not the
   table, and a resumed run re-prices.
6. **The bare assertion.** Does an authored node whose text displays no reasoning keep `none` and
   `0.95`, with its support read from its edges, or does the absence of a displayed argument become a
   criterion? The rubric as written takes the first; the second caps every chain that passes through a
   bare-text claim at the bare value, because `rigor` is each node's own cap in the minimum.
7. **The bottom rung's wording, and its cut.** The wording is decided — `unsubstantiated`,
   status phrase, label and slug (§8.6) — and the code change was the contract's: `internal/kb/schema.go`
   and the `build_status` and `build_band` a query reports then still said `refuted, do not use` and
   `refuted`.
   What remains open is whether the cut moves: `[0, 0.20)` is now three of eleven rigor values wide, and a
   rung that names an outcome is a rung a reader may want at a different height. Changing the cut moves
   published strengths; changing the words does not.
8. **The ceiling's enforcement.** Is `0.95` a discipline of the table alone, or does `set-rigor` refuse
   `1.0` for a claim, an analysis and an experiment, or does `verify` report it? And what becomes of the
   `rigor: 1.0` a conversion writes on every experiment: left as read, lowered by a scoring run, or
   corrected by a migration that revises a register already at `2.0.0`?
9. **The count is the residual.** Segmentation is the model's, the parser enforces none of it (§3.9), and
   the count is what the acceptance test compares (§9). Is that acceptable for a landing, or does the
   first calibration run need to measure how far two runs' counts diverge on the same text before the
   per-occurrence arithmetic is trusted to move a value across a band cut?
