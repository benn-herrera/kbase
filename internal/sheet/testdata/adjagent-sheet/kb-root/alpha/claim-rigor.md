# Claim register

`kbase refresh` generates this file from the KB's records; it is never edited by hand.

## A supporting lemma
<!-- id: ana-aaa007 -->

> **Leaf references:** [analysis](./analysis.md).

### Rigor
- rigor: 0.8
- rests-on:
  - axm-S1 — Conservation of stock (bearing 0.8)
- rationale: Fixture.

### Strength
- strength: 0.8 (ok to build on, see caveats) [= min(0.80, 0.80)]

## Flow simulation
<!-- id: exp-aaa008 -->

> **Leaf references:** [flow](./experiments/flow.md).

### Rigor
- status: run
- rigor: 0.8

## Equation (`eq:flow`) — Flow Dynamics
<!-- id: clm-aaa002 -->

> **Leaf references:** [results](./results.md).

### Rigor
- rigor: 0.95
- rests-on:
  - axm-S1 — Conservation of stock (bearing 1.0)
- rationale: Fixture.

### Strength
- strength: 0.95 (ok to build on) [= min(0.95, 1.00)]

## Theorem 1
<!-- id: clm-aaa001 -->

> **Leaf references:** [results](./results.md).

### Rigor
- rigor: 0.9
- rests-on:
  - clm-aaa002 — [Equation (`eq:flow`) — Flow Dynamics](claim-rigor.md#equation-eqflow-flow-dynamics) (strength 0.95) (bearing 1.0) [first use; second use]
  - axm-S1 — Conservation of stock (bearing 1.0)
- demoted:
  - clm-aaa004 — [A deliberately long prose claim title that runs well past the three wrapped lines a sheet label allows before it is cut](claim-rigor.md#a-deliberately-long-prose-claim-title-that-runs-well-past-the-three-wrapped-lines-a-sheet-label-allows-before-it-is-cut) (demoted from supports) (bearing *pending*)
- rationale: Fixture.

### Strength
- strength: 0.9 (ok to build on) [= min(0.90, 0.95)]

## The gradient $`\nabla f`$ is "bounded" when a < b & c
<!-- id: clm-aaa003 -->

> **Leaf references:** [results](./results.md).

### Rigor
- rigor: 0.5
- rests-on:
  - clm-aaa001 — [Theorem 1](claim-rigor.md#theorem-1) (strength 0.9) (bearing 1.0)
  - clm-aaa002 — [Equation (`eq:flow`) — Flow Dynamics](claim-rigor.md#equation-eqflow-flow-dynamics) (strength 0.95) (bearing 1.0)
- lifted-by:
  - exp-aaa008 — [Flow simulation](claim-rigor.md#flow-simulation) (bearing 0.8)
- refuted-by:
  - ana-aaa007 — [A supporting lemma](claim-rigor.md#a-supporting-lemma) (strength 0.8) (bearing 0.5)
- rationale: Fixture.

### Strength
- strength: 0.6 (use as input only, don't build deeper) [= min(max(min(0.50, 0.90), 0.64), 0.60)]

## A deliberately long prose claim title that runs well past the three wrapped lines a sheet label allows before it is cut
<!-- id: clm-aaa004 -->

> **Leaf references:** [results](./results.md).

### Rigor
- rigor: *pending*
- lifted-by:
  - ana-aaa007 — [A supporting lemma](claim-rigor.md#a-supporting-lemma) (strength 0.8) (bearing 0.5)
- references:
  - clm-bbb003 — A prose claim in beta
- rationale: Fixture.

### Strength
- strength: 0.4 (do not build on, rework needed) [= lift 0.40]
