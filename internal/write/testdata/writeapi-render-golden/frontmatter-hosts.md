---
kind: leaf
no-claim: "hosts an experiment and two analysis nodes only"
experiment-nodes:
  - exp-id: exp-gg7777
    status: run
    rigor: 1.0
    lifts:
      - clm-aa1111: 0.8
analysis-nodes:
  - ana-id: ana-hh8888
    lifts:
      - clm-bb2222: 1.0
  - ana-id: ana-ii9999
    refutes:
      - clm-bb2222: "*pending*"
      - clm-cc3333: 0.5
---
