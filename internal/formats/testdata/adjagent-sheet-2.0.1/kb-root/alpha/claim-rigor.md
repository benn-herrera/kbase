# Claim Rigor

---

## Theorem 1
<!-- id: clm-aaa001 -->

> **Leaf references:** [results](./results.md).

### Rigor
- rigor: 0.9
- rests-on:
  - clm-aaa002 — Flow dynamics (strength 0.95) (bearing 1.0) [first use]
  - clm-aaa002 — Flow dynamics (strength 0.95) (bearing 1.0) [second use]
  - Axiom S1 (bearing 1.0)
- demoted:
  - clm-aaa004 — The long claim (origin inferred)
- rationale: Fixture.

### Strength
- strength: 0.9 (ok to build on) [= min(0.90, 0.95)]

---

## Equation (`eq:flow`) — Flow Dynamics
<!-- id: clm-aaa002 -->

> **Leaf references:** [results](./results.md).

### Rigor
- rigor: 0.95
- rests-on:
  - Axiom S1 (bearing 1.0)
- rationale: Fixture.

### Strength
- strength: 0.95 (ok to build on) [= min(0.95, 1.00)]

---

## The gradient $`\nabla f`$ is "bounded" when a < b & c
<!-- id: clm-aaa003 -->

> **Leaf references:** [results](./results.md).

### Rigor
- rigor: 0.5
- rests-on:
  - clm-aaa002 — Flow dynamics (strength 0.95) (bearing 1.0)
  - clm-aaa001 — Theorem 1 (strength 0.9) (bearing 1.0)
- rationale: Fixture.

### Strength
- strength: 0.6 (use as input only, don't build deeper) [= min(max(min(0.50, 0.90), 0.64), 0.60)]

---

## A deliberately long prose claim title that runs well past the three wrapped lines a sheet label allows before it is cut
<!-- id: clm-aaa004 -->

> **Leaf references:** [results](./results.md).

### Rigor
- references:
  - clm-bbb003 — A prose claim in beta
- rationale: Fixture.

### Strength
- strength: 0.4 (do not build on, rework needed) [= lift 0.40]

---

## An isolated $`\Gamma`$-observation nothing cites
<!-- id: clm-aaa005 -->

> **Leaf references:** *(none — entry has no citing leaf; bidirectional-coverage check will fail)*

### Rigor
- rigor: 0.3
- rationale: Fixture.

### Strength
- strength: 0 (unsubstantiated)

---

## See [the appendix](appendix.md) for a *stated* bound that the rest of the volume never reaches at all
<!-- id: clm-aaa006 -->

> **Leaf references:** *(none — entry has no citing leaf; bidirectional-coverage check will fail)*

### Rigor
- rigor: 0.1
- rationale: Fixture.

### Strength
- strength: 0 (unsubstantiated)

---

## A supporting lemma
<!-- id: ana-aaa007 -->

> **Leaf references:** [analysis](./analysis.md).

### Rigor
- rigor: 0.8
- rests-on:
  - Axiom S1 (bearing 0.8)
- lifts:
  - clm-aaa004: 0.5
- refutes:
  - clm-aaa003: 0.5
- rationale: Fixture.

### Strength
- strength: 0.8 (ok to build on, see caveats) [= min(0.80, 0.80)]
