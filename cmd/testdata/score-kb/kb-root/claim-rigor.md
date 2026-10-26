# Claim register

`kbase refresh` generates this file from the KB's records; it is never edited by hand.

## The First Result
<!-- id: clm-aaaaaa -->

> **Leaf references:** [a](./a.md).

### Rigor
- rigor: 0.6
- refuted-by:
  - ana-aaaaaa — [A Supporting Argument](claim-rigor.md#a-supporting-argument) (strength 0) (bearing *pending*)
- rationale: Read off the leaf's own statement.

### Strength
- strength: 0 (unsubstantiated)

## The Second Result
<!-- id: clm-bbbbbb -->

> **Leaf references:** [a](./a.md).

### Rigor
- rigor: 0.55
- rests-on:
  - clm-aaaaaa — [The First Result](claim-rigor.md#the-first-result) (strength 0) (bearing *pending*)
  - exp-aaaaaa — [A Trial Run](claim-rigor.md#a-trial-run) (bearing *pending*)
  - wrk-aaaaaa — [Smith, A. 2020. *Markets and Clearing*](claim-rigor.md#smith-a-2020-markets-and-clearing) (bearing *pending*) [Theorem 3]
- lifted-by:
  - ana-aaaaaa — [A Supporting Argument](claim-rigor.md#a-supporting-argument) (strength 0) (bearing *pending*)
- rationale: Clears where the first result holds and the cited work applies.

### Strength
- strength: 0 (unsubstantiated)

## A Trial Run
<!-- id: exp-aaaaaa -->

> **Leaf references:** [exp](./exp.md).

### Rigor
- status: run
- rigor: 0.8

## A Supporting Argument
<!-- id: ana-aaaaaa -->

> **Leaf references:** [ana](./ana.md).

### Rigor
- rigor: 0.65
- rests-on:
  - axm-S1 — Conservation Of Stock (bearing *pending*) [a conservation assumption]
- rationale: The argument carries the trial's result to the second result.

### Strength
- strength: 0 (unsubstantiated)

## Smith, A. 2020. *Markets and Clearing*
<!-- id: wrk-aaaaaa -->

### Rigor
- rigor: 0.9
- rationale: Fixture Press.
