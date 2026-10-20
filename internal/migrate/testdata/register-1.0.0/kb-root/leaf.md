---
kind: leaf
claims: [clm-aaaaaa, clm-bbbbbb, clm-cccccc]
experiment-nodes:
  - exp-id: exp-aaaaaa
    status: run
    strengthens:
      - clm-aaaaaa: 0.8
      - clm-bbbbbb: "*pending*"
  - exp-id: exp-bbbbbb
    status: pending
support-nodes:
  - sup-id: sup-aaaaaa
    supports:
      - clm-bbbbbb: 0.5
      - clm-cccccc: "*pending*"
  - sup-id: sup-bbbbbb
    supports:
      - clm-cccccc: 0.7
---
[↑ KB](entry-point.md)


# A Leaf

> **Theorem 2.** Every flow conserves stock. <!-- claim-quality: clm-aaaaaa -->
