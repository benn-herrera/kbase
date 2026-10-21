# Rigor and bearing: the fault set, the table and the answer

The two scoring passes ask a model about one item each: `rigor` on one node, `bearing` on one
scoring edge. The answer is a list of **charged occurrences** — one fault id and one citation per
line — and kbase prices the list into the one value the register holds.
[SPEC.md](SPEC.md) §8 carries the `score` pass, its answer and its ledger, §10 the register's
spelling, and [STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md) the three quantities and the
walk's arithmetic; that page is the authority for the arithmetic and nothing here restates it.

## The two judgements

**`rigor` judges one node's own worth**: what the node's text displays, against what the node
asserts. It is asked of a claim, an analysis and an experiment — never of a work, whose standing is
judged with the document in hand and written by `set-rigor`, and never of an axiom, whose `rigor` is
`1.00` by rule. The assertion is the node's title, read together with the statement its text
displays: where the text says less than the title, the shortfall is a fault and not a redefinition
of the assertion. Truth is out of scope — a charge is returned on what the text displays,
never on whether the assertion holds in the world, so a text that reasons impeccably to a false
conclusion and contradicts itself nowhere returns `none`. A premise the node's edges carry is out of
scope too: whether it applies to the assertion is that edge's `bearing`.

**`bearing` judges one source's relevance to the target it scores**: how far the source's content
speaks to the target's assertion. The source is taken as given — true as it states itself — and
neither its soundness nor the target's wisdom in citing it is judged; bearing weights the source's
contribution and never modifies the source's value. A part of the target's assertion is *engaged*
when, taking the source as given, believing it would change what one should think about that part, in
either direction. Support and refutation are both engagement, and the sign of the engagement is never
judged: the charges measure how much of the target the source touches, and the walk turns that into a
limit. A source and a target may share subject matter and vocabulary while nothing the source states
reaches what the target asserts.

The two judgements are orthogonal: one text may be exquisite and bear on nothing, or shoddy and
exactly on point. Each has its own fault set, its own maximum and its own table, and no mark combines
them.

## The fault vocabulary

Twelve ids name the faults, six to each judgement, and `flt-pending` belongs to both. An id is
written `flt-[a-z0-9-]+` and spells the criterion it names, so a ledger line needs no table to read.
The two sets are disjoint: an id names exactly one fault wherever it appears, and a ledger line
carries its judgement with it. The ids carry no severity and no order — the weight is the table's,
below — and `none` is a word, not an id: it names the absence of a charge and is not a member of the
set.

Rows are in walk order. A criterion is met at each site of the text whose test it satisfies, and
where one site satisfies two rows the earlier row's id is the charge and the later is not.

**That precedence binds the judgement, and the parse records what the reply wrote.** It is a rule on
how the model reads the text, and no check enforces it. The reason is the parse's whole remit: it
cannot rule on whether two criteria were met at one site, which is the same question as whether a
span earns an id, and it never asks that ([What the check does, and does not
do](#what-the-check-does-and-does-not-do)). A shared span is also not the same fact as a shared
site: two rows whose tests are met at different occurrence units — a minimal set, a respect, a
distinct proposition — may anchor at one sentence, so a parse that refused the second id on a span
would refuse defensible answers. Two ids on one span are therefore accepted and both charged, and
the shared offset beside the verbatim reply is what lets a later reader hold the line against the
precedence and see whether the walk's order was missed. The cost is stated rather than hidden: a
departure from the precedence is priced, not refused. The alternative — the parse refusing a second
id on a span — is not the choice, because it turns a judgement it cannot check into `malformed`,
discarding both the answer and the evidence for it, and a retry is a fresh guess at the same
unverifiable question.

### The `rigor` faults

Six faults, maximum `0.95`. The weight is per occurrence.

| fault id | the fault | one occurrence is | weight per occurrence |
|---|---|---|---|
| `flt-unaddressed` | The text does not address the assertion: no sentence of the text, the assertion itself aside, bears on what the assertion claims. | The text, whose test is universal over it: at most one. | `0.95` |
| `flt-contradiction` | The text cannot all be true: two statements the text makes cannot both hold and the assertion rests on at least one of them, or the assertion is incompatible with a statement the text makes — while a limitation, counter-case or range the assertion names, and a tension a condition the text states resolves, are not contradictions. | One minimal set of statements that cannot all hold: as many sets. | `0.45` |
| `flt-overreach` | The assertion claims more than the text shows: the narrowest proposition the text establishes does not entail the assertion — a wider scope, a stronger modality, a value stated for a case the text does not cover — while a restatement that entails nothing more is not overreach. | One respect in which the entailment fails: as many as fail, and at most three, because the respects are a closed list — scope, modality, an uncovered value; a fourth charge repeats one of them. | `0.35` |
| `flt-claimed-step` | A step is claimed and none is shown: the text marks an inference — *hence*, *it follows*, *therefore*, *clearly*, *one can show* — and displays no statement from which the asserted conclusion follows; a conclusion claimed with no marker is not this. | One marked inference with no displayed statement the conclusion follows from: as many as the markers so placed. | `0.30` |
| `flt-unstated-premise` | A displayed step — an inference, or a computation from stated quantities — rests on something the text does not state: it does not follow unless a proposition the text does not state is granted; a premise the text names as a graph node, or one an edge of the node carries, is that edge's `bearing` and never this. | One **distinct** proposition a displayed step needs and the text does not state: as many propositions, not as many steps. | `0.30` |
| `flt-bare-measurement` | A number is reported and no method is: the text reports a measured or computed value and displays no procedure, sample, instrument, dataset or calculation that yields it; a stated constant, a stipulated parameter, a definition and a reference are not results. | One reported value with no displayed method: as many values — a table of six numbers with no method is six charges. | `0.30` |

**A bare assertion fires none of the six, and that is a known consequence, not a gap in the table.** A
text that only restates its title displays no step, reports no value, states no premise and widens
nothing, so it satisfies none of the tests above: `flt-overreach` needs the text to establish less than
the assertion, and a restatement establishes exactly it. Such a text answers `none` and keeps `0.95`.
The consequence belongs in the vocabulary because it is what the value can be read to mean: `0.95`
reads *no charge among the six*, and never *the text displays its reasoning*. A bare assertion and a
text that argues impeccably are the same value, and only the reply beside the value and the node's own
text tell them apart. Whether the absence of displayed reasoning becomes a seventh criterion is open
([Not settled](#not-settled)).

### The `bearing` faults

Six faults, maximum `1.00`. The weight is per occurrence.

| fault id | the fault | one occurrence is | weight per occurrence |
|---|---|---|---|
| `flt-no-engagement` | Nothing in the source speaks to the assertion: for every part of the target's assertion the source states nothing that would change what one should think of that part, in either direction; a shared vocabulary is not engagement. | The link, whose test is universal over the target: at most one. | `1.00` |
| `flt-referent` | The link rests on a term the two statements use differently: source and target share a term that carries the link and use it for different referents or under different definitions; where nothing survives substituting the target's sense for the source's, the fault is `flt-no-engagement`. | One shared term the two texts use differently and that carries the link: as many terms. | `0.50` |
| `flt-partial` | The source engages part of a compound assertion: the target's assertion names more than one part — a conjunction, a list of conditions, two quantities asserted together — and the source engages at least one part and not all. | One part of a compound assertion left unengaged: as many parts. | `0.50` |
| `flt-narrower` | The source reaches the assertion only within a narrower scope: the assertion quantifies over a class, a range, a population or a set of conditions, and the source's content holds for part of it — one member, one regime, one value of a parameter; an assertion scoped to that part, and a source general where the assertion is the case, are not narrower. | One dimension in which the source's scope is narrower: as many dimensions. | `0.50` |
| `flt-transfer` | The link runs through a step neither text states: the source's subject differs from the target's, and what makes the source bear on the target is an unstated bridge — an analogy, a scaling, *the same holds for*, a change of unit or of definition; a bridge stated in either text or in the edge's context is not this. | One unstated bridge the link runs through: as many bridges. | `0.35` |
| `flt-conditional` | The source applies only under a condition it does not state: its content is stated under conditions, and its bearing on the target depends on a condition the source does not give — a regime, a temperature, an assumption — that the target's assertion needs. | One unstated condition the bearing depends on: as many conditions. | `0.35` |

### `flt-pending`

The thirteenth id belongs to both judgements and names one fault: the text holds no proposition to
judge. It takes no citation, because there is no sentence to point at, and it carries no weight: an
answer of `flt-pending` has the value `*pending*` whatever the table. The
[`flt-pending`](#flt-pending) section below states when each pass answers it.

## The value

The maximum is per judgement: `M = 0.95` for `rigor` and `M = 1.00` for `bearing`. For a charge list
`C` under a pass's table:

```
value(C)             = max(0, M − Σ w(id(c)) for c ∈ C)
value(none)          = M
value([flt-pending]) = *pending*
```

The weight is charged **per occurrence**, each occurrence separately, and the table caps no count: a
text that commits a fault three times loses three weights. The parse enforces no count at all, and
the only mechanical pressure on one is that two occurrences of one id need two spans that do not
coincide.

A count is bounded only where the row's own occurrence unit is an enumeration of a closed set, and
there the bound is what *one occurrence* means rather than a ceiling the table adds.
`flt-unaddressed`'s and `flt-no-engagement`'s tests are universal over the text they judge, so each
has one occurrence. `flt-overreach`'s respects are the closed list of three, so it has three.
`flt-partial`'s parts are the assertion's own, and a compound assertion fixes how many there are. A
charge beyond such a bound is not a further occurrence: it is one of them written a second time. The
parse cannot see that, because no respect and no part is a ledger field; the reader can, holding the
reply's spans against the text.

The floor at zero is the domain's, not a rule of its own: a list whose weights reach `M` is a text
with nothing left to carry the assertion, and `0` is what the register says for that. Every weight is
a multiple of `0.05`, so every sum and every value is exact at two decimals and nothing rounds.

**`1.00` is reserved by rule, and the reserve is not a model's answer.** `kb.AxiomRigor` is `1.0`; an
axiom premise contributes `bearing × 1.0`, and no authored file carries `1.00`. An axiom is asked by
neither pass — its `rigor` is kbase's rule, not a judgement — so `rigor`'s authored maximum is the
largest multiple of `0.05` below the reserve, `0.95`, and `none` is the only answer that reaches it.
What `0.95` certifies is the absence of a charge among the six, not the presence of reasoning: a bare
assertion reaches it too ([The `rigor` faults](#the-rigor-faults)).
A charge list under the `rigor` table cannot reach `1.00` at all. `0.95` is the largest value a rigor
pass derives and not the largest the register holds: `set-rigor` accepts any value in `[0,1]`, `1.0`
included. `bearing` has no such reserve, and `none` reaches `1.00`.

## The band outcomes

The ladder is `kb.BuildBandLadder` (`internal/kb/schema.go`): cuts at `0.85`, `0.65`, `0.45`, `0.20`
and `0.00`, each an inclusive lower bound. The ladder reads `strength`, not `rigor` — a node's band
can sit below its rigor because of a premise, and `rigor` is not an upper bound on `strength` because
a lift station stands above the derivation ([STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md),
"The walk"). The column below is what a rigor value would read when rigor is what binds.

| charges | rigor | band it would read, when rigor is what binds |
|---|---|---|
| `none` | `0.95` | ok to build on |
| one gap id | `0.65` | ok to build on, see caveats — exactly at the cut |
| one `flt-overreach` | `0.60` | use as input only |
| one `flt-contradiction` | `0.50` | use as input only |
| two gap ids | `0.35` | do not build on, rework needed |
| a gap id and an `flt-overreach` | `0.30` | do not build on, rework needed |
| two `flt-overreach` | `0.25` | do not build on, rework needed |
| a gap id and an `flt-contradiction` | `0.20` | do not build on, rework needed — exactly at the cut |
| an `flt-overreach` and an `flt-contradiction` | `0.15` | unsubstantiated |
| three gap ids, or two `flt-contradiction` | `0.05` | unsubstantiated |
| an `flt-unaddressed`, or any sum of `0.95` or more | `0.00` | unsubstantiated |
| `flt-pending` | `*pending*` | no band |

"Gap id" stands for any of `flt-claimed-step`, `flt-unstated-premise` and `flt-bare-measurement`,
which carry one weight because each names the same defect at a different site. The eleven values over
`[0, 0.95]` leave nine of the range's twenty multiples of `0.05` unreachable — `0.10`, `0.40`,
`0.45`, `0.55`, `0.70`, `0.75`, `0.80`, `0.85`, `0.90` — so a rigor value off this list is a defect
of a landing, not a judgement.

`bearing` reaches six values and no band of its own: it reaches the ladder only through a product in
the walk.

| charges | sum | bearing |
|---|---|---|
| `none` | — | `1.00` |
| one `flt-transfer` or `flt-conditional` | `0.35` | `0.65` |
| one of `flt-referent`, `flt-partial`, `flt-narrower` | `0.50` | `0.50` |
| two of `flt-transfer`, `flt-conditional` | `0.70` | `0.30` |
| one half-weight and one step-weight charge | `0.85` | `0.15` |
| two of `flt-referent`, `flt-partial`, `flt-narrower` | `1.00` | `0.00` |
| an `flt-no-engagement`, or any sum of `1.00` or more | `≥ 1.00` | `0.00` |
| `flt-pending` | — | `*pending*` |

**The bottom rung is `unsubstantiated`, and it states an outcome, not a cause.** Every strength below
`0.20` lands there from either direction: by a cap, where a refuter's `bearing × strength` exceeds
`0.80`; or by the minimum, where `flt-unaddressed`, three gap charges, two contradictions, or a
premise contribution low enough leaves nothing to carry the assertion. What the rung says is
that nothing a claim built on this would inherit substantiates the claim, and how that state arrived
is not part of it. A reader who needs the cause has it where it belongs: the fault record names the
charges, and the derivation trace prints the cap or the station that produced the strength. The rung's
cut is an inclusive lower bound of `0.00`, and its height is not settled ([Not settled](#not-settled)).

## `flt-pending`

`flt-pending` is not a hedge and not "unsure". It is **the text does not support judging at all**: no
proposition is identifiable, so there is nothing to judge and no other id applies. Its test is applied
first, before any criterion: where the pair holds no proposition, the answer is `flt-pending` and the
walk stops there. It occupies the answer, it has a table outcome, and its value is `*pending*`. A
`flt-pending` where a judgement was possible leaves a hole in the graph that no query can see.

**When `rigor` is `flt-pending`.** The title-and-text pair holds no proposition:

- a title that names a topic rather than asserting, with no statement under it;
- a section that is empty, a stub or a placeholder ("TBD", "—", "to be written");
- a statement that asks a question, lists keywords, or points at material the call does not show,
  where the title names a topic rather than asserting.

Nothing else. A text that holds a proposition and is about another subject is not pending: it is one
`flt-unaddressed` and answers `0.00`. A text that holds a proposition and argues badly is not pending
either: it is a charge list against it.

**When `bearing` is `flt-pending`.** Either end holds no proposition by the test above. A work
renders its title alone, so a link from a work whose title names no subject is pending; where the
title states a subject, the criteria apply to what the title states, that being the text shown. A
link whose two ends both hold propositions is never pending: an irrelevant link charges
`flt-no-engagement` and answers `0.00`, and a hedge on a link that can be judged removes the edge from
the graph silently.

**Its form.** `flt-pending` takes no citation, and it has exactly two accepted spellings, each the
whole of its line: the bare id, and the bare id inside one bracket pair (`[flt-pending]`), which the
form's bracket rule strips as it strips any field's. The id is case-folded like any other. Everything
else is `malformed`, and three cases are named because they are the ones a reply will produce:
`flt-pending:` with nothing after the colon or with whitespace only; `flt-pending: <span>` with any
span, since it takes none; and the id carrying trailing letters, which is an id outside the set. The
empty-field form is refused rather than read as the bare id because an empty citation field is
exactly the shape of a charge line cut off after its colon, and a truncated reply must fail rather
than land as a judgement.

It occupies the answer: a reply carrying it beside charge lines is read with `flt-pending` as the
judgement and the charges as **moot** — parsed and recorded, priced nowhere. There is no proposition
to point at, so nothing else can become the judgement; but a model that hedged and a model that
guessed are told apart by the record, not by the value, and a smaller model's straggling charges are
kept for what they say about the model rather than dropped.

**Against `0.00`.** The two are different states and the walk treats them in opposite directions: a
pending `rigor` leaves its derivation with no number while a lift over it still publishes the lift's
product, a pending `bearing` excludes its edge entirely, and a `0.00` is a judgement that
participates ([STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md), "Pending: an edge either
participates or does not"). `0.00` is the judgement that the text carries nothing; `*pending*` is the
absence of a judgement.

## The answer

The ask states this contract, so that a reply is not lost to a spelling the ask never named. kbase
reads the reply and nothing else.

### The closed text

Each call carries one item's own text. For `rigor`: the node's title and the statement its own
document gives it. For `bearing`: both ends' title and statement in that same form, then the edge's
authored context. A work renders its title alone — its document lies outside the corpus — so a link
from a work is judged on the title's proposition. SPEC §8 states the closure; the register's own
scoring tokens are refused before the call, and that item is `failed` with no call made.

### The form

One line per occurrence:

```
flt-id: citation
```

Leading and trailing whitespace is trimmed. A line is a charge line or it is not: blank lines are
ignored, and any other line — an explanatory sentence, a heading, a preamble — is `malformed`.
Nothing is read out of prose. The brackets of the format sketch are delimiters, not characters: a
line may carry `[…]:` and `[…]` around the two fields or not, a bracket pair around a whole field is
stripped before the field is read, and a span that itself begins with `[` or ends with `]` is written
unbracketed. Order is free; charges are recorded in the order written, and no arithmetic depends on
it.

### The closed set

`rigor` accepts the six `rigor` ids and `flt-pending`; `bearing` accepts the six `bearing` ids and
`flt-pending`; neither accepts anything else. An id is case-folded before matching and recorded in
its own lower-case spelling. An id outside the call's set — the other pass's id, a letter from the
retired alphabet, a misspelling — is `malformed`, and the refusal names the id and the pass's ids as
`allowed`.

### `none`

`none`, alone, is the empty answer: no charge, value `M`. It is the only way to reach a maximum. It
is a word and not an id. `none` beside charges is `malformed`: both halves are claims about faults
and they contradict each other outright, and the parse has no ground to prefer either, so preferring
one would invent the judgement rather than read it. An empty reply and a reply of whitespace only are
not answers either: `malformed`, nothing written.

### The citation

A citation is a verbatim span of the closed text. Its floor is **one character of text**: a span
that carries none — empty, or whitespace only — is `malformed`, and a single character is enough,
because the citations most worth having are often tiny (`0.42` for a bare measurement, a symbol, a
term the link turns on). The lookup collapses runs of whitespace to one space and folds case, so a
re-wrapped or re-capitalised quotation is found; the citation is recorded as the model wrote it, with
the offset of its first match. The span must lie whole inside a passage and never cross from one to
another:

| judgement | a citation lies in |
|---|---|
| `rigor` | the node's title, or its statement |
| `bearing` | the source's title or statement, or the target's |

A span in the edge's `## Context` is not a citation for a charge: the context states bridges and
excuses, and a charge is about what the two texts do and do not do. A `bearing` citation may come
from either end, because the evidence for a bearing fault can lie at either — `flt-referent` needs
the term as one text uses it, `flt-partial` the part the source leaves alone, `flt-transfer` the two
subjects — and the ask directs the model to quote a span unique to its end wherever the words allow
one. **Where the ends carry the same words and no longer span separates them, the citation is
recorded as `both`**, at the first match in the closed text's order, and the charge stands: `passage`
then states exactly what the lookup found, that the two texts carry this span alike. For `rigor` the
two passages are one node's own text, and a span found in both takes `both` on the same rule. No
line names its end: the recorded passage carries the attribution wherever the texts make one.

The two alternatives were weighed and are not the choice. **Refusing a shared span** leaves a fault
with no chargeable citation wherever both ends state the same words — a judgement the criteria make
available, made unaskable, and a reply right in substance failed and re-asked until the model finds
words that were never there. **Attributing the span to one end by rule** writes a falsehood into the
field a reader audits: `passage` states where the citation was found, so recording `source` for words
the target carries too would assert a fact the lookup contradicts, and per-id rules for which end a
fault "belongs to" would make one field mean two things. `both` records the finding and lets the
reader see that this charge's anchor does not distinguish the ends — which is true, and is what a
reader needs to know to weigh the charge.

### What the check does, and does not do

kbase checks that the span **exists**; it never checks that the span **earns** the id. An irrelevant
sentence is a legal citation, and nothing in the parse reads the sentence against the criterion. Half
these faults are absences — no sentence bears on the assertion, no displayed step carries the
conclusion, no method yields the number — where no span could be evidence of the fault at all; for
those the citation anchors the **obligation** (the title that asserts, the marker that claims the
step, the value reported), and the absence itself is checked by nobody. A span is an anchor, not
proof. What carries the audit beyond the anchor is the reply kept verbatim beside the parsed charges,
the offset that locates the span for a later reader, the value's derivability from the ids, and agreement between runs measured on counts
per id.

### Duplicates, overlaps and shared spans

| case | outcome |
|---|---|
| one id, one span, written twice | one occurrence written twice: the pair collapses to one charge |
| one id, two spans that overlap without coinciding | `malformed`, naming both lines — the reply asserts two occurrences and supplies one site |
| two ids, one span | accepted, both charged, the offset shared in the ledger |

The duplicate collapses because the answer's own evidence cannot tell the two lines apart. The overlap
is refused because the count cannot be read from it, and a retry settles it, where reading it as one
would write a confidently high value. Two distinct faults may share a sentence: the one-defect rule
is the walk's, not the parser's, and the shared offset is the signal that the precedence was missed —
not proof that it was ([the fault vocabulary](#the-fault-vocabulary)).

### Malformed replies

| case | outcome |
|---|---|
| a line that is not a charge line | `malformed` |
| an id outside the call's set | `malformed`, naming the id and the pass's ids as `allowed` |
| an id with no separator, or nothing after the colon | `malformed` — except the bare `flt-pending`, which takes no citation and no colon ([`flt-pending`](#flt-pending), "Its form") |
| `flt-pending` with a colon and an empty field | `malformed`, naming the id — the shape of a charge line cut short after its colon |
| `flt-pending` with a citation field | `malformed`, naming the id and the span, and recording the span with it — it takes no citation |
| a citation not found in the closed text, including one that crosses a passage boundary, one in the context, and one cut short by a truncated reply | `malformed`, naming the id and the span, and recording the span with it |
| a number — a decimal, a fraction, `0`/`1`, or a `*pending*` literal | `malformed`: a bare number has no id and no citation, and a reply offering one answers a question no longer asked |
| `none` beside charges | `malformed` |
| an empty reply, or whitespace only | `malformed`, nothing written |

A `malformed` reply is the pass's outcome: no charges and no judgement, the item re-asked on
`--resume` (SPEC §8). The attempt's record carries `status: failed` and the reason — the case from
the table and, where there is one, the id and span it named — and **not the reply's text**: a
malformed reply is blank, a truncated fragment or think-lock, and the data point that matters is that
the attempt failed, not the bytes it failed with.

## The ledger

Each attempt is one record in the run's `ledger.jsonl`, carrying the item, its status, `faults` as
parsed, the `reply` verbatim where the reply parsed, the `value` landed, the model, the prompt's
digest and the attempt's bounds (SPEC §8), or the reason it failed and no text where it did not. `faults` is the effective answer — one record per charge, in the order written
— and `moot` holds any charge lines a `flt-pending` made moot, parsed the same way and priced
nowhere:

| field | holds |
|---|---|
| `id` | the fault id, in its lower-case spelling |
| `passage` | where the citation was found: `title`, `statement` or `both` for `rigor`; `source`, `target` or `both` for `bearing` |
| `citation` | the span as the model wrote it |
| `offset` | its offset in the closed text, at its first match — for `both`, the earlier of the two passages |

`[]` is written for one reply only — the word `none` — so a record's empty list always carries the
model's positive statement that it found no fault; an empty or whitespace-only reply is `malformed`,
so the attempt records `failed` and no charges, and silence is never read as a clean pass. One record carries `id: flt-pending`
and no citation for a pending item. The reply stands beside the parsed charges, so the parse's reading
of a line can be held against the line. A `--dry-run` writes no charge record: nothing is asked, so
nothing is returned.

**The value is a projection of the charge list, and in fact of its ids alone** — `value = table(counts
by id)`. The spans are not the input, and that is why they are the audit rather than the measure:
they can be read, checked against the text and argued about, and they cannot perturb the arithmetic.

- **A landing can be checked without a model.** A value that does not equal the table's output for
  its recorded ids is a defect of the landing or of the table, not a judgement to argue with. Counts
  are integers, so the check is exact.
- **A run can be re-scored without a model.** Change a weight and re-derive every value in the ledger
  by counting ids; no call is made and no judgement re-asked. The durable record is the count per id.
- **Two runs are comparable on their fault sets, not their numbers.** Two runs agree on the judgement
  exactly when their counts per id match; their values are comparable only within one table. A value
  agreement is the weaker test: `0.35` is reached by two `flt-bare-measurement` and equally by one
  `flt-claimed-step` and one `flt-unstated-premise`, so a sum hides both the split and the identity.
  Reproducibility is measured on counts.

**The run records the ask, not the table.** What a charge set means is fixed by the instructions the
model was given — the fault set and the text describing it — and never by the weights, which the
model is never shown. So the run's header carries `ask_sha256`, the digest of the prompt rendered
for an empty node: rendering substitutes and never inspects the text, so the empty input covers every
fixed byte of instruction and nothing that varies, and no size-dependent element may enter a
rendered prompt.

- **An update applies its behaviour.** A run resumed under a changed table re-prices every landed
  value from the recorded ids, lands the values that moved and continues; one resumed under a changed
  ask continues under the new ask. Nothing is refused, and nothing asks the user to choose between
  versions: they updated, which is the choice.
- **The digest is a record, not a gate.** `ask_sha256` says which instruction answered which
  judgement, so a corpus spanning an update can be read honestly. It decides nothing at run time.

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
carries the ask's digest once, for every attempt in the run. This is what makes a run
re-scorable, a landing checkable and two runs comparable, so the format changes only with the
scheme's version and never as an implementation convenience: a field dropped at write time cannot be
recovered by a later reader, and a summary written by the writer is a judgement the corpus can no
longer take apart.

## Not settled

- What `counts` reports for a run whose values have been re-priced mid-run.
- The bottom cut's height: the last rung takes every strength under `0.20`, and where the cut belongs
  is open.
- The occurrence units: the parse enforces no count, and which proposition, which minimal set, which
  part a second site counts as is the reader's judgement rather than a frozen rule. What a row's unit
  settles — the universal tests' one, `flt-overreach`'s three — is stated where the row is.
- Whether the twelve ids are the frozen vocabulary: a rename after a run moves the ledger's record
  with it.
- Whether a text that displays no reasoning charges nothing and keeps `0.95`, or the bare assertion
  becomes a criterion of its own. What is settled is the table's answer today: such a text fires none
  of the six and keeps `0.95`, stated in [The `rigor` faults](#the-rigor-faults) so that no reader
  takes `0.95` for evidence of reasoning; what is open is whether a seventh criterion is added and at
  what weight.
- Whether `0.95` is a discipline of the table alone, or `set-rigor` refuses `1.0` for a claim, an
  analysis and an experiment — and what becomes of the `rigor: 1.0` a conversion writes on every
  experiment.
