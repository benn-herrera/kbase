## 1. Purpose

kbase makes a knowledge base auditable for an academic lab running local inference. The KB carries a
claim graph whose nodes and edges are drafted deterministically and refined by model calls, and every
refinement is checkable against the artifact it refined. kbase is a Go program that builds that KB. It
manages its own model providers and makes its own inference calls.

kbase is assembled from standard patterns at small scale. Each section below names its pattern in its
first line and states only where kbase departs from it, or what the pattern leaves open. Three parts
are novel, the three of THESIS's "What is new":

1. **What a claim graph is made of**: five node kinds and four scoring relations (SPEC §10 Five node
   types; SPEC §10 Four scoring relations, one arithmetic). Realized in the `claims-declared`,
   `claims-discovered` and `analyses-split` stages ([§4](../4-stage-table/index.md)) and in
   [§6](../6-kb-model-and-maintenance/the-node-vocabulary.md).
2. **The local judgments**: the fault criteria in the scoring prompt templates, and the bearing
   predicates that decide which edges are judged ([§6](../6-kb-model-and-maintenance/scoring/the-prompt-shelf.md),
   [§6](../6-kb-model-and-maintenance/scoring/bearing-predicates.md)). The templates, the input guard and the reply parse are standard
   machinery.
3. **How the judgments combine**: the strength arithmetic ([§6](../6-kb-model-and-maintenance/the-derived-layer/index.md)).

Contents:

- [§1 Pattern map](pattern-map.md)
