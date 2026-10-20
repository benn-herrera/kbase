# Part 1 Claim Quality Register

---

## Regime Conservation Laws
<!-- id: clm-aaaaaa -->

> **Leaf references:** *(none)*

### Rigor
- rigor: 0.85
- rests-on:
  - clm-bbbbbb — Foundation Claim B (strength 0.90) (bearing 1.0) [builds directly on the anchor claim]
  - wrk-smith2020 — Smith, A. 2020 (bearing 0.8) [Theorem 3]
  - Axiom S3 (bearing 1.0) [a labelling convention]
- rationale: CES asymptotics applied with an explicit exponent formula.

### Strength
- strength: 0.62 (use as input only) [= min(0.85, 0.62)]

---

## Foundation Claim B
<!-- id: clm-bbbbbb -->

> **Leaf references:** *(none)*

### Rigor
- rigor: 0.90
- rests-on:
  - clm-aaaaaa — A joint premise (strength 0.85) (bearing 1.0)
  - clm-cccccc — A joint premise (strength *pending*) (bearing 1.0)
  - clm-cccccc — A second joint premise (strength *pending*) (bearing 0.5)
  - wrk-smith2020 — A second joint premise (bearing 0.5)
  - Axiom S3 (bearing 0.5)
- rationale: One bullet names two claims; the next a claim and a work.

### Strength
- strength: 0.90

---

## A Second Claim
<!-- id: clm-cccccc -->

> **Leaf references:** *(none)*

### Rigor
  - rigor: 0.60
- quality: 0.4
- rests-on:
  - wrk-smith2020 — Smith, A. 2020 (bearing 1.0)
  - Axiom 1 (bearing 1.0) [the substrate assumption]
- rationale: An indented score line and a key of another kind.

### Strength
- strength: 0.60

---

## Smith, A. 2020. *Markets*, on bearing 0.5
<!-- id: wrk-smith2020 -->

### Rigor
- rigor: 0.9
- rationale: Fixture Press.

---

## A Supporting Lemma
<!-- id: ana-aaaaaa -->

### Rigor
- rigor: 0.8
- rests-on:
  - clm-aaaaaa — Regime Conservation Laws (strength *pending*) (bearing 1.0)
- lifts:
  - clm-bbbbbb: 0.5
  - clm-cccccc: *pending*
- rationale: Derives the beneficiary result in full.

### Strength
- strength: *pending*
