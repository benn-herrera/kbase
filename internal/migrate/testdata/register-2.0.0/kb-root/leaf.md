---
kind: leaf
claims: [clm-aaaaaa, clm-bbbbbb, clm-cccccc]
experiment-nodes:
  - exp-id: exp-aaaaaa
    status: run
    rigor: 1.0
    lifts:
      - clm-aaaaaa: 0.8
      - clm-bbbbbb: "*pending*"
  - exp-id: exp-bbbbbb
    status: pending
    rigor: 1.0
analysis-nodes:
  - ana-id: ana-aaaaaa
    lifts:
      - clm-bbbbbb: 0.5
      - clm-cccccc: "*pending*"
  - ana-id: ana-bbbbbb
    lifts:
      - clm-cccccc: 0.7
---
[↑ KB](entry-point.md)


# A Leaf

> **Theorem 2.** Every flow conserves stock. <!-- claim-rigor: clm-aaaaaa -->
