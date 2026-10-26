# Proposed revision: strength scoring

**Status.** A capture of work in progress, not a contract. It records what the system now does, what
is proposed and not yet built, and what is still open. Where it disagrees with
[STRENGTH_SCORING_SCHEME.md](../STRENGTH_SCORING_SCHEME.md) or
[RIGOR_AND_BEARING_SCORING_SCHEME.md](../RIGOR_AND_BEARING_SCORING_SCHEME.md), those pages describe the
code today and this page describes where it is heading.

Sections marked **landed** are in `main`. Sections marked **proposed** are not in the code.

## 1. What a claim is — landed

**A claim is, by definition, a statement shorn of supporting argument.** Its `rigor` is the quality of
the assertion its text makes — fixed, consistent, single-sensed — never the quality of an argument,
which a claim does not display. Supporting argument is separated from the claim and stands in an
analysis or an experiment, reaching the claim through an edge. A claim whose text carries a derivation
is mis-typed: the derivation belongs in the analysis it should have been.

## 2. Support — landed

Support comes through edges and nowhere else. In the walk, `r(n)` is the cap on what the node's
premises establish, not a substitute for having any:

```
P(n) = { contribution of each contributing supports/founds edge }
D(n) = 0                                  where P(n) is empty and n is a claim
D(n) = r(n)                               where P(n) is empty and n is an analysis
D(n) = min(r(n), min(P(n)))               otherwise
σ(n) = min(max(D(n), L(n)), C(n))
```

- A claim with no incoming scoring edge is `strength 0`, however good its statement.
- An analysis with no incoming scoring edge is `strength` at its own `rigor`: a method's inputs are
  the rules of inference, not a KB node. An analysis that does cite premises is `min(r, P)` as a
  claim is, so a zero still propagates through a real dependency.
- A pending rigor over a non-empty premise set is `⊥`; over an empty one it is `0` for a claim and
  `⊥` for an analysis.
- An edge that does not contribute — pending bearing, pending source, excluded — is no supporter.
- A lift-only claim takes `L` and a refutation-only claim takes `0`; an analysis composes the same
  two around its own rigor, taking `max(r, L)` and `min(r, C)`.

The claim's statement quality survives in `0.95 / 0.00`: the rigor is what the reader sees, and the
strength says nothing carries it.

## 3. Two rigor vocabularies — landed

A node's class decides which vocabulary judges it and which table prices it. One ask per item; the
class only gates which template is used.

### Statement (claims), max `0.95`

| id | weight |
|---|---|
| `flt-contradiction` | 0.45 |
| `flt-overreach` | 0.35 |
| `flt-equivocation` | 0.25 |
| `flt-underspecified` | 0.05 |

The worked case: *"Parisian bleu cheese emits 2.8×10⁹ scent molecules per gram per degree above
270 K"* — specific, disprovable, and underspecified, because it fixes no producer or brand, no vintage
or year, no humidity and no pressure. Four free parameters, four charges of `flt-underspecified` →
`0.75`.

### Method (analyses, experiments), max `0.95`

| id | weight |
|---|---|
| `flt-unaddressed` | 0.95 |
| `flt-contradiction` | 0.45 |
| `flt-overreach` | 0.35 |
| `flt-claimed-step` | 0.30 |
| `flt-unstated-premise` | 0.30 |
| `flt-bare-measurement` | 0.30 |

`flt-contradiction` and `flt-overreach` are shared ids with the same weight in both tables; the item's
class supplies the subject. `flt-pending` belongs to both.

**The ceiling.** `kb.RigorMaximum = 0.95`, defined once and read everywhere. `1.00` belongs to an axiom
by rule and no authored node may carry it: a write above the maximum is refused, and a leaf already
holding `1.0` is read as `0.95` while its bytes stay as written. Bearing reserves nothing and takes
`1.00`.

## 4. Bearing — landed, unchanged

Six ids: `flt-no-engagement` 1.00; `flt-referent`, `flt-partial`, `flt-narrower` 0.50; `flt-transfer`,
`flt-conditional` 0.35. Max `1.00`. Engagement is judged blind to its sign: support and refutation are
both engagement, and the charges measure how much of the target the source touches.

## 5. The combination — proposed

Today a chain compounds: `P(n)` uses `b(e)·σ(source)`, so a five-link chain at bearing `0.9` reads
`0.9⁵ · 0.95 ≈ 0.56`. Depth is penalised, and nothing in the model says a long argument is weaker than
a short one of the same quality.

**Proposed:** each edge attenuates its own link once, and the chain is as strong as its weakest link.

```
T(e)  = min( b(e) · r(source), T(source) )      the support an edge passes upward
σ(n)  = min( r(n), min over incoming edges of T(e) )
```

Equivalently, the value of a path is `min` over its links of `r_i · b_i`. Properties:

- **Depth does not compound.** Five links of `0.95` texts with `0.9` bearings read `0.855`, the same as
  one link.
- **Transitivity survives.** A weak link anywhere in the chain carries up through the `min`.
- **A bearing costs its own share, once.** `0.9` on a `0.95` text gives `0.855`, not a discount that
  repeats at every level.

**Consequences to settle before it lands.**
- Every number rises, so `unsubstantiated` becomes rare and the ladder may stop separating anything.
  The band cuts and the weights have to be re-read together.
- `lifts` and `refutes` must use the same transmission `T`, or a lift over a long chain reintroduces the
  compounding through the back door.
- `min` over supporters is strict: a claim with one `0.9` and one `0.3` premise reads `0.3`. Correct if
  every `supports` edge is a necessary premise; worth stating in the walk.

## 6. Coverage — proposed, not designed in detail

A quantity the system does not have: **how much of a claim is supported**. An airtight derivation of
one of four necessary pillars currently reads `0.95` and lands "ok to build", when the claim is a
quarter explained.

**Proposed: claims declare their pillars, and edges land on sockets.**

- A claim's metadata enumerates its pillars — the things that must hold for the claim to hold.
  Enumerating them is the author's job, part of making the claim rather than an optional extra. The
  layout (top-down, bottom-up, sections) is immaterial; what matters is that the pillars are named.
- A `supports` edge names the socket — the pillar — it lands on.
- Coverage is `filled pillars / declared pillars`, computed from the graph with no model involved and
  no double counting: two sources on one pillar fill one pillar.
- Strength is `min(weakest link) × coverage`, so the four-pillar claim with one `0.9` pillar reads
  `0.225`: `unsubstantiated` — "call us when you're done" — with three unfilled sockets saying why.

**Open.**
- What an edge with no socket does. Refusing it at write time is the cleaner statement — enumerate or
  do not claim support — and makes the socket required for `supports`.
- What a claim with no pillars declared means. Every claim in every existing KB is in that state.
  *Legacy, coverage 1* leaves the corpus unchanged; *coverage 0* is honest and zeroes most of it until
  authors enumerate. A wipe-the-board call, not a technical one.
- Whether coverage multiplies the weakest link or caps it as another term in the `min`. Multiplication
  expresses "one of four" directly; a `min` would let a strong pillar hide three empty ones.
- Whether the sockets are declared per claim only, or whether an analysis may itself have pillars the
  claim's sockets map onto.

## 7. Still open from earlier work

- **The bottom cut's height.** `[0, 0.20)` currently holds three of eleven method values, and the
  ladder's last rung is `unsubstantiated` — an outcome, not a cause. Held at `0.20`; reviewable if the
  bands crowd.
- **Whether the statement table's numbers are right.** `flt-underspecified` at `0.05` was chosen so
  ordinary statement deduction never reaches the bottom rung. A corpus question.
- **The twelve-id count** was a pre-split figure: there are now fourteen ids across the three
  vocabularies plus `flt-pending`, and the id set may grow while a named fault keeps its id for life.

## 8. What is landed, and what a reader should expect

| thing | state |
|---|---|
| claim as statement; support through edges; empty `P` → `0` | landed in `STRENGTH_SCORING_SCHEME.md` and the walk (`internal/index/strength.go`) |
| two rigor vocabularies, their tables, the class-gated ask and parse | landed |
| `RigorMaximum` enforced on write, `1.0` read as `0.95` | landed |
| one `ask_sha256` over every instruction a run can send | landed |
| the six worked examples at uniform inputs | landed in `STRENGTH_SCORING_SCHEME.md` |
| no-compounding transmission `T` | **proposed** — changes every value and the bands |
| coverage by declared pillars and sockets | **proposed** — a data-model change to claims and `supports` edges |
