# Strength, edge by edge

The claim graph after `2.0.0`: three quantities, four scoring relations, and the arithmetic that
turns them into a number.

## The three names

```
  rigor      one node's own worth                 claim, support, work, experiment
  bearing    one scoring edge's relevance         a fraction in [0,1], or *pending*
  strength   the derived value                    what the sheet draws and bands use
```

`solidity`, `confidence`, `quality`, `applicability`, `fraction` and the edge-level `strength` are all
retired names for these three.

## The nodes

```
  claim       clm-xxx    rigor  (its own derivation)        strength derived
  support     sup-xxx    rigor                             strength derived
  work        wrk-xxx    rigor  (the paper's soundness)    strength = rigor
  experiment  exp-xxx    rigor  (the method's soundness)   strength = rigor
  framework   inv-/axiom-  no rigor; strength 1.0 by rule
```

## The four scoring relations — all point at what they support

```
  relation   source → target                  bearing feeds        arithmetic
  ────────────────────────────────────────────────────────────────────────────────
  supports   claim / experiment → claim        bearer's strength    multiply, into the min
             (target may also be a support)
  founds     work → claim                      work's rigor         multiply, into the min
             (target may also be a support)
  lifts      support / experiment → claim      lifter's strength    multiply, max above the min
  refutes    support / experiment → claim      refuter's strength   cap: 1 - (bearing * strength)

  references claim → claim                    —                    not scored
  demoted    claim → claim   (build only)     —                    not scored
```

A framework premise is a `supports` edge from the invariant or axiom with no bearing; it contributes
`1.0`, the identity in the min.

## The graph

```
   ┌──────────────┐
   │ claim C2     │─── supports 0.40 ───┐
   └──────────────┘                      │
   ┌──────────────┐                      │
   │ work W       │─── founds   0.80 ────┤
   └──────────────┘                      ▼
   ┌──────────────┐           ┌──────────────────┐
   │ experiment E │── lifts ─▶│ claim C1         │
   └──────────────┘   0.70    └──────────────────┘
   ┌──────────────┐                      ▲
   │ support R    │─── refutes 0.50 ─────┘
   └──────────────┘            (caps C1)
```

Every arrow runs from the supporter to what it supports, the way the verb reads: C2 *supports* C1,
W *founds* C1, E *lifts* C1, R *refutes* C1.

## The walk

```
  strength(C1) = min( rigor,
                      every supports contribution,     bearing x supporter strength
                      every founds contribution )     bearing x work rigor

  final(C1)    = max( strength, every lift )          bearing x lifter strength

  final(C1)    = min( final, every refute cap )       cap = 1 - bearing x refuter strength
```

A support or an experiment lifts the **final**, above the min: a fabulous piece of later work can
establish what a shoddy first attempt only hinted at. A refutation caps the final from above: a
rigorous, on-point refutation drives the claim to zero, and several refutations take the *minimum*
cap — they do not stack.

## Worked example

```
  supports  C2 -> C1   0.40 x 0.90 = 0.36
  founds    W  -> C1   0.80 x 0.90 = 0.72
  strength(C1)         min(0.85, 0.36, 0.72) = 0.36
  lifts     E  -> C1   0.70 x 0.90 = 0.63
  final after lifts    max(0.36, 0.63) = 0.63
  refutes   R  -> C1   cap = 1 - (0.50 x 0.90) = 0.55
  final(C1)            min(0.63, 0.55) = 0.55
```

Change one bearing and the graph answers differently:

- `supports` bearing `1.0` → contribution `0.90`, strength `min(0.85, 0.90, 0.72) = 0.72`.
- `founds` bearing `0.0` → contribution `0`, strength `0`: the work is not what the claim stands on.
- `lifts` bearing `1.0` → lift `0.90`, final `max(0.36, 0.90) = 0.90` before any cap.

## Pending: an edge either participates or does not

```
  pending rigor        the node has no strength
  pending bearing      the edge does not participate: no link in the min, no lift, no cap
  bearing 0.0          participates with zero: the claim's min goes to zero
  pending refute       cap 1.0; an unfinished attack establishes nothing
```

`bearing *pending*` is the absence of a judgement, so the edge is excluded. `bearing 0.0` is the
judgement that the supporting thing does not apply, so the edge participates and drags the min to
zero. Never judged and judged-irrelevant are different states with different scores.

## Authoring and storage

The register is claim-centric: a claim lists what it rests on.

```
  register (claim C1)                     stored edge (push)
  ────────────────────────────────────────────────────────────────
  - rests-on:
      - clm-c2 — the lemma (strength 0.90) (bearing 0.40)     clm-c2 --supports--> clm-c1
      - wrk-w  (bearing 0.80)                                 wrk-w  --founds-->   clm-c1
```

`deps <claim>` reads the stored edges from the reader's side: every incident scoring edge, with
`direction` (`in` = the other end supports this claim, `out` = this node supports the other), the
other end, and its `bearing`.

## Required from 2.0.0

```
  every supports / founds / lifts / refutes edge   (bearing …)   required; number or *pending*
  every claim, support, work                       - rigor:      required
  every experiment                                 rigor:        in its leaf's node mapping
  every premise bullet's annotation                (strength …)  the source's derived strength
  framework premises, references, demoted          no bearing
```
