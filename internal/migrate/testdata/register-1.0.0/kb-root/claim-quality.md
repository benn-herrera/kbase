# Part 1 Claim Quality Register

---

## Regime Conservation Laws
<!-- id: clm-aaaaaa -->

> **Leaf references:** *(none)*

### Quality
- confidence: 0.85
- depends-on:
  - clm-bbbbbb — Foundation Claim B (solidity 0.90) [builds directly on the anchor claim]
  - work-smith2020 — Smith, A. 2020 (applicability 0.8) [Theorem 3]
  - INVARIANT-S3 (a labelling convention)
- rationale: CES asymptotics applied with an explicit exponent formula.

### Solidity
- solidity: 0.62 (use as input only) [= min(0.85, 0.62)]

---

## Foundation Claim B
<!-- id: clm-bbbbbb -->

> **Leaf references:** *(none)*

### Quality
- confidence: 0.90
- depends-on:
  - clm-aaaaaa and clm-cccccc — A joint premise (solidity 0.85)
  - clm-cccccc and work-smith2020 — A second joint premise (applicability 0.5)
  - INVARIANT-S3 (applicability 0.5)
- rationale: One bullet names two claims; the next a claim and a work.

### Solidity
- solidity: 0.90

---

## A Second Claim
<!-- id: clm-cccccc -->

> **Leaf references:** *(none)*

### Quality
  - confidence: 0.60
- quality: 0.4
- depends-on:
  - work-smith2020 — Smith, A. 2020
  - Axiom 1 (the substrate assumption)
- rationale: An indented score line and a key of another kind.

### Solidity
- solidity: 0.60

---

## Smith, A. 2020. *Markets*, on bearing 0.5
<!-- id: work-smith2020 -->

### Quality
- strength: 0.9
- rationale: Fixture Press.

---

## A Supporting Lemma
<!-- id: sup-aaaaaa -->

### Quality
- quality: 0.8
- depends-on:
  - clm-aaaaaa — Regime Conservation Laws (solidity *pending*)
- supports:
  - clm-bbbbbb: 0.5
  - clm-cccccc: *pending*
- rationale: Derives the beneficiary result in full.

### Solidity
- solidity: *pending*
