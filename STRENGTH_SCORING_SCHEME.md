# Strength, edge by edge

The claim graph: five node types, three quantities, four scoring relations, and the
arithmetic that turns them into a number. This page is the scoring model's statement; [SPEC.md](SPEC.md)
§10 carries the format, and [ARCHITECTURE.md](ARCHITECTURE.md) the implementation.

## The three values

Each value is a number in `[0,1]` or `*pending*`.

| value | meaning |
|---|---|
| `rigor` | one node's own worth – the quality of a claim's statement, an analysis's method, or a work's soundness |
| `bearing` | one scoring edge's relevance (relevance of the source node to the target node) |
| `strength` | computed from rigor and effects of incoming edges – what the sheet draws and bands use |

Rigor and bearing are orthogonal values. A source may be exquisite, but have no bearing on the node
it is supposed to support. It may also be shoddy, but exactly on point.

A low bearing is not a verdict on the work. It says the claim does not stand on this work — a fact
about the link's construction, not the work's soundness — and the work's `rigor` is unchanged.
The remedy is to remove the entry if the work is irrelevant to the claim, or re-judge the bearing if
it is not.

## The nodes

| node | id | local value | derived |
|---|---|---|---|
| `claim` | `clm-xxxxxx` | `rigor` (its statement's own quality) | `strength` derived |
| `analysis` | `ana-xxxxxx` | `rigor` (its method's own quality) | `strength` derived |
| `work` | `wrk-<key>` | `rigor` (the paper's soundness) | none — `rigor` is the source strength |
| `experiment` | `exp-xxxxxx` | `rigor` (the method's soundness) | none — `rigor` is the source strength |
| `axiom` | `axm-<label>` | `rigor` `1.00` by rule — no authored file carries it | none — `rigor` is the source strength |

**A claim is, by definition, a statement shorn of supporting argument.** Its `rigor` is therefore the
quality of the assertion its text makes — fixed, consistent, single-sensed — never the quality of an
argument, which a claim does not display. Supporting argument is separated from the claim and stands
in an analysis or an experiment, reaching the claim through an edge; an analysis's `rigor` is the
quality of the method it displays against what it sets out to establish. A claim whose text carries a
derivation is mis-typed: the derivation belongs in the analysis it should have been, and until it
moves, the claim's strength says what actually carries it — nothing. Whether anything carries a node
is `strength`'s business, and the walk below says how.

## The four scoring relations — all point at what they support

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

An axiom premise is a `founds` edge from an axiom; it carries its bearing like every scoring edge,
and the axiom's `rigor` of `1.00` by rule leaves its contribution the bearing alone.

## The graph

![The strength walk](readme-assets/strength-walk.svg)

Every arrow runs from the supporter to what it supports, the way the verb reads: C2 *supports* C1,
S1 *founds* C1, W *founds* C1, E *lifts* C1, R *refutes* C1. Every box lists `rigor` first and
`strength` last, and each line holds a value or says why there is none. A claim or an analysis has
both: its own `rigor` and the `strength` the walk derives. A work, an experiment or an axiom has no
derived value of its own — its `rigor` *is* its strength, which is what its edges multiply — so its
box names both and marks the second `strength=rigor`. An axiom's `rigor` is `1.00` by rule, and no
authored file carries it. Each edge is labelled with its relation and its `bearing`.

Two boxes carry a strength whose premises this fragment does not draw: R's `0.75` and C2's `0.65`. A
claim with no incoming scoring edge reads `0`, so neither value could hold without a premise set
outside the picture — R is a refuter whose derivation lies outside the fragment, and C2 stands on
premises not shown. An analysis is constituted by the nodes it cites, so one carrying no premise is
its own method's case rather than a claim's: the walk reads it at its `rigor` (§ The walk), and the
fragment holds none.

C1's box carries `strength 0.40`, the value the sheet draws and the bands read. The `0.26` the min
derives is not a value any node holds: a lift stands above it and a refutation caps it, and those
compositions are the worked example below.

`readme-assets/strength-walk.dot` is the image's source and the file to review;
`readme-assets/strength-walk.svg` is its rendering, generated by `just strength-walk` and never
edited by hand.

## The walk

```
Let n be a claim or an analysis. ⊥ means "no number yet"; an empty lift set contributes −∞ and an
empty cap set +∞, so neither changes anything. D is 0 where a claim's premise set is empty, and n's
own rigor where an analysis's is: a claim proves nothing bare, an analysis's method is its evidence.

r(n)  = n's authored rigor, or ⊥
σ(m)  = the published strength of a claim or an analysis m;
        the authored rigor of a work or an experiment; 1.0 for an axiom,
        its rigor by rule
b(e)  = the bearing authored on edge e, or ⊥
·     = an operand that is absent wherever either factor is ⊥

P(n)  = { b(e)·σ(e) : e ∈ supports(n) ∪ founds(n), e contributes }
D(n)  = 0                      where P(n) = ∅ and n is a claim
D(n)  = r(n)                   where P(n) = ∅ and n is an analysis
D(n)  = min({ r(n) } ∪ P(n))   otherwise
L(n)  = max{ b(e)·σ(e) : e ∈ lifts(n), e contributes }
C(n)  = min{ 1 − b(e)·σ(e) : e ∈ refutes(n), e contributes }

max(⊥, −∞) = ⊥     max(⊥, x) = x     max(x, −∞) = x
min(⊥, c)  = ⊥      min(x, +∞) = x

σ(n)  = min( max(D(n), L(n)), C(n) )
```

**`r(n)` caps what the premises establish, and is not a premise.** Where `P(n)` is empty no premise
carries the node, and `D(n)` is what the node's kind is worth bare: `0` for a claim, whatever `r(n)`
says — judged, pending or absent — and `r(n)` itself for an analysis, whose method is the evidence
its premises would otherwise supply. Support is
structural: the walk reads that nothing supports the claim without knowing how good the node's own
text is, so it does not wait for a judgement. Where `P(n)` is non-empty and `r(n)` is pending, `D(n)`
stays `⊥`, because `min(⊥, c) = ⊥`. The asymmetry is the rule's content: an absent cap is unknown, an
absent premise is nothing.

`⊥` is "no number", not a small one: a lift over an unjudged node publishes the lift, and a cap over
one publishes nothing. So an empty `P(n)` with a pending `r(n)` publishes `0` for a claim — or the
lift standing over it — and not `⊥`, while an analysis publishes `⊥`, having no method's number to
stand on; and a non-empty `P(n)` with a pending `r(n)` stays `⊥` for either kind, and a lift standing
over it publishes the lift's product. The walk carries full precision, so a dependent multiplies its
source's exact strength. The record writes a value to four decimals — trailing zeros trimmed, and
more only where four would lose it, so it reads back as the value; a query result carries the value
at two decimals and the trace prints it, and a `verify` message quotes the record. The trace on a
`- strength:` line prints the innermost station first, so the formula reads back: the min, the lift
above it, then the cap.

## One premise, and none

`lifts` and `refutes` compose around `D` as the formula states. A claim whose only incoming scoring
edge is a lift takes `max(0, L) = L`; an analysis takes `max(r, L)`, its own method standing under
the lift. Either way the product is a contribution through an edge, and an empty premise set does
not suppress it. A claim whose only incoming edge is a refutation takes `min(0, C) = 0`: a cap is a
ceiling and never a floor, so an attack with nothing under it establishes nothing. An analysis takes
`min(r, C)` there — its method's number where the cap sits above it. A claim with no incoming
scoring edge at all has an empty `L` and an empty `C`, and lands `0`; an analysis with none lands
its `r`.

One premise proves the weaker of the two things in play — the claim's own statement and the premise's
contribution — and no premise proves nothing. Let `c` be a supporting edge's contribution,
`b(e)·σ(e)`, and `r` the node's rigor:

| a claim | rigor | strength |
|---|---|---|
| one supporting edge with contribution `c` | `r` | `min(r, c)` |
| no supporting edge | `r` | `0.00` |

| an analysis | rigor | strength |
|---|---|---|
| one supporting edge with contribution `c` | `r` | `min(r, c)` |
| no supporting edge | `r` | `r` |

The first row is the claim standing on one thing: the premise carries it and the statement caps it,
so a premise worth more than the statement cannot make the claim worth more than its statement, and a
thin premise gives it less. The second row is the bare claim, where `r` does not enter at all. A bare
claim whose statement fires nothing is the case `r = 1.00`, the statement vocabulary's maximum
([RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md), "The statement table"):
`1.00` rigor and `0.00` strength — a statement fully specified, consistent and single-sensed, with
nothing carrying it. What it proves is what it says, and nothing about whether anything supports it.

The analysis's second row is the other asymmetry. A claim is a statement, so nothing outside it can
stand under its assertion; an analysis is a method, and its inputs are the rules of inference rather
than a KB node — so one citing nothing reads its own `rigor`, and the claim it lifts off that
analysis reads it too. A method that does cite premises is unchanged: `min(r, c)` as for a claim, so
an analysis resting on an unsubstantiated claim still reads `0`, and a zero still propagates through
real dependencies.

## Worked example

```
  supports   C2 -> C1   bearing 0.40 x strength 0.65 = 0.26
  founds     W  -> C1   bearing 0.70 x strength 0.72 = 0.50
  founds     S1 -> C1   bearing 0.90 x strength 1.00 = 0.90
  D(C1)                 min(rigor 0.84, C2 0.26, W 0.50, S1 0.90) = 0.26
  lifts      E  -> C1   bearing 0.75 x strength 0.64 = 0.48
  max(D, L)             max(0.26, 0.48) = 0.48
  refutes    R  -> C1   cap = 1 - (bearing 0.80 x strength 0.75) = 0.40
  σ(C1)                 min(0.48, 0.40) = 0.40
```

W's product is `0.504`: the min is the same at full precision and at the two decimals the page prints.
S1 is the axiom shape: its `rigor` is `1.00` by rule, so the contribution is the bearing alone. Its
`0.90` sits in the min beside the others and does not bind — a premise stronger than the weakest link
changes nothing.

Change one bearing and the graph answers differently:

- `supports` bearing `1.0` (C2) → contribution `0.65`, derivation `min(0.84, 0.65, 0.50, 0.90) = 0.50`.
- `founds` bearing `0.0` (W) → contribution `0`, derivation `0`: the work is not what the claim stands
  on.
- `lifts` bearing `1.0` → lift `0.64`, so `max(D, L) = 0.64` before any cap.
- every premise excluded (C2, W and S1 all pending) → `P(C1)` is empty and `D(C1) = 0`, so
  `max(D, L) = 0.48` is the lift's product alone; the cap still holds C1 at `0.40`.

## Worked examples: one input set, six shapes

The example above is the diagram's own fragment, so its figures are the diagram's and move with it. This
set holds the inputs still instead: the same values are authored on every node in every case, and the only
thing that varies is which edges exist. Nothing requires a KB to author the same values everywhere — the repetition
is the exercise's, not the format's — and it is what lets each relation be read against the same operands.

| authored input | value | the table it comes from |
|---|---|---|
| a work's `rigor` | `0.70` | method vocabulary, two findings |
| a claim's `rigor` | `1.00` | statement vocabulary, no charge — the table's maximum |
| an analysis's `rigor` | `1.00` | method vocabulary, no charge — the table's maximum |
| an experiment's `rigor` | `1.00` | method vocabulary, no charge — the table's maximum |
| `founds` `bearing` | `1.00` | bearing vocabulary, no charge |
| `supports` `bearing` | `0.20` | eight bearing charges |
| `lifts` `bearing` | `1.00` | bearing vocabulary, no charge |
| `refutes` `bearing` | `1.00` | bearing vocabulary, no charge |

Every one of these is a value its table reaches, not merely a multiple of `0.05`: the method vocabulary
reaches eight — every fifteen hundredths — and `0.70` is one of them, and the bearing vocabulary
reaches eleven — every tenth
([RIGOR_AND_BEARING_SCORING_SCHEME.md](RIGOR_AND_BEARING_SCORING_SCHEME.md), "The method table", "The
bearing values"). Three of the four bearings are `1.00`, a link with no bearing charge, and those
contributions are their sources' own values: the sources are what this set holds still. The exception is
the supporting edge's `0.20` — eight bearing charges, a claim
speaking to part of what it is cited for — and that is the set's one edge where a bearing visibly moves a
number.

The four products, one per relation, are therefore:

```
founds      1.00 x a work's rigor 0.70         = 0.70
supports    0.20 x a claim's strength 0.70     = 0.14
lifts       1.00 x an experiment's rigor 1.00  = 1.00
refutes     cap = 1 - (1.00 x a claim's strength 0.70) = 0.30
```

The refuter there is a claim like any other: its own strength is `0.70` once the work founds it, and the
cap multiplies that `0.70` — not the work's rigor.

Bands are `kb.BuildBandLadder`'s, each threshold an inclusive lower bound:

| band | at or above |
|---|---|
| `ok-to-build` | `0.85` |
| `ok-with-caveats` | `0.65` |
| `input-only` | `0.45` |
| `do-not-build` | `0.20` |
| `unsubstantiated` | `0.00` |

Every figure below is exact at two decimals. The record trims a trailing zero — `0.20` is written
`- strength: 0.2`, `0.30` is `0.3` — while a query and the trace print the two decimals written here.

### 1. A chain: a work founds a claim, the claim supports another

```
  W  --founds 1.00--> C2  --supports 0.20--> C1
 0.70                 1.00                   1.00
```

```
founds     W  -> C2   bearing 1.00 x strength 0.70 = 0.70
D(C2)                 min(rigor 1.00, W 0.70) = 0.70
σ(C2)                 min(max(0.70, −∞), +∞) = 0.70
supports   C2 -> C1   bearing 0.20 x strength 0.70 = 0.14
D(C1)                 min(rigor 1.00, C2 0.14) = 0.14
σ(C1)                 min(max(0.14, −∞), +∞) = 0.14
```

`σ(C1) = 0.14`, band `unsubstantiated`. At neither node does the `1.00` bind — a rigor caps what the premises
establish, and at both nodes the premise is the lower operand — while the second hop costs exactly the
bearing: `0.70` in, `0.14` out. The work is cited at full relevance and `σ(C2)` carries its `0.70`
unchanged; what puts C1 under the `0.85` cut is the link it stands on, not the work.

### 2. A lift over a weak base

```
  W  --founds 1.00--> C2  --supports 0.20--> C1
 0.70                 1.00                   1.00
  E  --lifts 1.00-------------------------> C1
 1.00   (run)
```

```
founds     W  -> C2   bearing 1.00 x strength 0.70 = 0.70
supports   C2 -> C1   bearing 0.20 x strength 0.70 = 0.14
D(C1)                 min(rigor 1.00, C2 0.14) = 0.14
lifts      E  -> C1   bearing 1.00 x strength 1.00 = 1.00
L(C1)                 max{ E 1.00 } = 1.00
C(C1)                 none — no refute edge, so +∞
σ(C1)                 min(max(0.14, 1.00), +∞) = 1.00
```

`σ(C1) = 1.00`, band `ok-to-build`. `D(C1)` is case 1's `0.14`, untouched: a lift is not a premise and
does not enter the min, so the derivation still says the node stands on `0.14`. What `max` does is stand
the lift's `1.00` beside that derivation and keep the larger — the only station in the walk that can raise
a value, and here it carries the node from `unsubstantiated` to `ok-to-build`, four rungs. Take the edge away
and this is case 1 exactly; that is the case the station exists for.

### 3. A refutation caps — and sometimes does not

One refuter serves both readings: a claim `R` at `rigor 1.00`, founded by the same work, so
`σ(R) = min(1.00, 1.00 × 0.70) = 0.70`, refuting at `bearing 1.00`. The cap is `0.30` in both.

Binding:

```
founds     W  -> R    bearing 1.00 x strength 0.70 = 0.70
D(R)                  min(rigor 1.00, W 0.70) = 0.70
σ(R)                  min(max(0.70, −∞), +∞) = 0.70
founds     W  -> C1   bearing 1.00 x strength 0.70 = 0.70
D(C1)                 min(rigor 1.00, W 0.70) = 0.70
lifts                 none, so −∞
refutes    R  -> C1   cap = 1 - (bearing 1.00 x strength 0.70) = 0.30
C(C1)                 min{ R 0.30 } = 0.30
σ(C1)                 min(max(0.70, −∞), 0.30) = 0.30
```

`σ(C1) = 0.30`, band `do-not-build`. Its base alone was `0.70`, `ok-with-caveats`: the cap costs two
bands as
well as the number, and it is the only station that can put a node below both its derivation and its lift.

Not binding:

```
founds     W  -> R    bearing 1.00 x strength 0.70 = 0.70; σ(R) = 0.70
founds     W  -> C2   bearing 1.00 x strength 0.70 = 0.70
D(C2)                 min(rigor 1.00, W 0.70) = 0.70
supports   C2 -> C1   bearing 0.20 x strength 0.70 = 0.14
D(C1)                 min(rigor 1.00, C2 0.14) = 0.14
refutes    R  -> C1   cap = 1 - (bearing 1.00 x strength 0.70) = 0.30
C(C1)                 min{ R 0.30 } = 0.30
σ(C1)                 min(max(0.14, −∞), 0.30) = 0.14
```

`σ(C1) = 0.14`, band `unsubstantiated` — case 1's node with an attack on it. The cap participates, is at
full bearing and full strength, and is not the binding term: `min(0.14, 0.30)` is the premise's number. A
refutation can be worth a great deal and change nothing, because a cap is a ceiling and this node was
already under it.

### 4. A claim nothing supports

```
  C1   rigor 1.00, strength 0.00   — no incoming scoring edge
```

```
P(C1)                 empty — no supports edge and no founds edge
D(C1)                 0
L(C1)                 −∞      (no lift edge)
C(C1)                 +∞      (no refute edge)
σ(C1)                 min(max(0, −∞), +∞) = 0.00
```

`σ(C1) = 0.00`, band `unsubstantiated`; the record writes `- strength: 0`. Its statement is at the
vocabulary's maximum — `1.00`, no fault charged: fixed, consistent, single-sensed and fully specified —
and that number is nowhere in the arithmetic. `r(n)` caps what a premise set establishes and is not a
premise, and `P(C1)` is empty, so there is nothing for the cap to cap. What the node proves is what its
text says, and nothing about whether anything carries it: no improvement to the statement could move this
`0`, and only an edge can.

### 5. Pending: a pending node, and a pending edge

The three states below are the Pending section's, in arithmetic. Two of them are the pair a reader
conflates — a node whose own judgement is missing, and a node one of whose edges is.

Pending `rigor` over a premise set:

```
founds     W  -> C1   bearing 1.00 x strength 0.70 = 0.70
D(C1)                 min(rigor *pending*, W 0.70) = *pending*
L(C1)                 −∞      (no lift edge)
σ(C1)                 *pending*
```

Strength `*pending*`, band `unknown` — the ladder has no rung for a value that is not a number. The premise
set is non-empty and judged; the node's own rigor is not, and `min(⊥, c) = ⊥` carries that through.

Pending `bearing` as the node's only edge:

```
supports   C2 -> C1   bearing *pending* — excluded, adds no operand
P(C1)                 empty
D(C1)                 0
L(C1), C(C1)          −∞, +∞   (no lift edge, no refute edge)
σ(C1)                 min(max(0, −∞), +∞) = 0.00
```

Strength `0.00`, band `unsubstantiated`. An unjudged bearing is not support yet, so the edge is no
supporter; the node's `1.00` does not enter, for case 4's reason.

The same pending bearing beside a premise that participates:

```
founds     W  -> C1   bearing 1.00 x strength 0.70 = 0.70
supports   C2 -> C1   bearing *pending* — excluded, adds no operand
D(C1)                 min(rigor 1.00, W 0.70) = 0.70
L(C1), C(C1)          −∞, +∞   (no lift edge, no refute edge)
σ(C1)                 0.70
```

Strength `0.70`, band `ok-with-caveats`. An edge excluded for a pending bearing does not make its target
`*pending*`: it removes its own operand, and the node reads what the rest of its premise set establishes.
`*pending*` reaches a node from the node's own `rigor`, and only where the premise set is otherwise
complete and nothing lifts — a lift would publish its own product instead. It does not reach a node from
an edge.

### 6. A claim and an analysis on the same number

```
  W  --founds 1.00--> C1   claim,    rigor 1.00
  W  --founds 1.00--> A1   analysis, rigor 1.00
```

```
founds     W  -> C1   bearing 1.00 x strength 0.70 = 0.70
D(C1)                 min(rigor 1.00, W 0.70) = 0.70
σ(C1)                 min(max(0.70, −∞), +∞) = 0.70
founds     W  -> A1   bearing 1.00 x strength 0.70 = 0.70
D(A1)                 min(rigor 1.00, W 0.70) = 0.70
σ(A1)                 min(max(0.70, −∞), +∞) = 0.70
```

Both `0.70`, both band `ok-with-caveats`, and the arithmetic is identical term for term. The `1.00` inside each
min is a different judgement: on C1 it is the statement vocabulary charged nothing — the assertion's text
is fixed, consistent and single-sensed, and says nothing about an argument, which a claim does not display;
on A1 it is the method vocabulary charged nothing — the derivation engages what it sets out to establish
and states its steps. A claim has no method and an analysis no statement quality, so one number stands for
two different absences of fault; the node's class decides which criteria priced it, and the walk reads only
the number.

### The six at a glance

| case | the edges | `D` | `L` | `C` | `σ` | band |
|---|---|---|---|---|---|---|
| 1 chain | `W→C2` founds, `C2→C1` supports | `0.14` | — | — | `0.14` | `unsubstantiated` |
| 2 lift | as 1, plus `E→C1` lifts | `0.14` | `1.00` | — | `1.00` | `ok-to-build` |
| 3a cap binds | `W→C1` founds, `R→C1` refutes | `0.70` | — | `0.30` | `0.30` | `do-not-build` |
| 3b cap idle | `W→C2`, `C2→C1`, `R→C1` | `0.14` | — | `0.30` | `0.14` | `unsubstantiated` |
| 4 bare | none | `0` | — | — | `0.00` | `unsubstantiated` |
| 5a pending rigor | `W→C1` founds | `⊥` | — | — | `*pending*` | `unknown` |
| 5b pending bearing only | `C2→C1`, bearing `*pending*` | `0` | — | — | `0.00` | `unsubstantiated` |
| 5c pending bearing beside a premise | `W→C1`, `C2→C1` pending | `0.70` | — | — | `0.70` | `ok-with-caveats` |
| 6 same rigor | `W→C1` founds, `W→A1` founds | `0.70` | — | — | `0.70` | `ok-with-caveats` |

## Pending: an edge either participates or does not

| state | effect |
|---|---|
| pending `rigor` | where `P(n)` is non-empty the derivation has no number; where it is empty a claim reads `0` and an analysis reads no number, having no method's value to stand on. A lift over either still publishes the lift's product |
| pending `bearing` | the edge does not participate: no link in the min, no lift, no cap. As a claim's only edge it leaves `P(n)` empty and the claim reads `0`; as an analysis's only edge the analysis reads its `r` |
| no source strength | the edge does not participate either: a premise whose own strength is pending, and any edge from a non-run experiment, add no operand. As a claim's only edge it likewise reads `0`, and as an analysis's only edge the analysis reads its `r` |
| `bearing 0.0` | participates with zero: the claim's min goes to zero |
| pending refute | no cap; an unfinished attack establishes nothing, and a cap alone publishes no number where the derivation is `⊥` |

**An edge that does not contribute is no supporter.** A pending bearing, a pending source strength,
an edge from a non-`run` experiment and a premise the walk no longer holds all add no operand, so
where such an edge is a claim's only one `P(n)` is empty and the claim reads `0`: an unjudged bearing
is not support yet, and a claim behind one reads `unsubstantiated` until that bearing lands, never
`*pending*`. An analysis behind one is a method with no premise it can cite, and reads its own
rigor. `bearing 0.0` reaches the same `0` for a claim by the other road — it participates, and the
min takes the zero — and the two stay different states in the record whatever the strength reads.

`bearing *pending*` is the absence of a judgement, so the edge is excluded. `bearing 0.0` is the
judgement that the supporting thing does not apply, so the edge participates and drags the min to
zero. Never judged and judged-irrelevant are different states, and their scores coincide only where
the min reaches `0` either way — most plainly where the edge is the node's only one: `0.0` empties
the min by taking it to zero, an excluded edge empties it by leaving no operand, and both read `0`.
The difference is the record's and the re-judge's, not the strength's. A source with no strength is
the first case, not the second: there is nothing to multiply, so there is no product.

## Authoring and storage

The register is claim-centric: a claim lists what it rests on.

| register (claim C1) | stored edge (push) |
|---|---|
| `- rests-on:` | |
| `  - clm-c20000 — the lemma (strength 0.65) (bearing 0.40)` | `clm-c20000 --supports--> clm-c10000` |
| `  - wrk-w00000 — A Cited Work (bearing 0.70)` | `wrk-w00000 --founds--> clm-c10000` |
| `  - Axiom S1 — a labelling rule (bearing 0.90)` | `axm-S1 --founds--> clm-c10000` |

A bullet's target decides the relation: a `clm-` or `exp-` token is a `supports` edge into the entry,
and a `wrk-` or `Axiom …` token a `founds` edge into it. An axiom is cited by its label, so `Axiom S1`
is the node `axm-S1`. An `ana-` token is refused — an analysis lifts or refutes and never supplies a
premise — and so is a bullet naming more than one scoring target.

`deps <id>` reads the stored edges from the reader's side: every incident scoring edge, plus the
node's own outgoing `references` and `demoted` edges, each with `direction` (`in` = the other end
supports this node, `out` = this node supports the other), the other end, and its `bearing` — null on
a row that cannot carry one.
