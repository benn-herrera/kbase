### 3.1 Node writes

Every add, edit and remove is a write op, and every one follows [§2.2](../../2-conventions/2.2-write-batches.md) and [§2.3](../../2-conventions/2.3-derivation.md).

**Existing nodes.** An add of a node that already stands is refused, naming its id ([§2.10](../../2-conventions/2.10-existence.md)). The
identity an add checks is as follows.
- A node exists where a node of that kind carries that title anywhere in the KB ([§1.1](../../1-model/1.1-nodes.md)).
- A work also exists where another work carries that `label`, and an axiom where another axiom
  carries that `label`.
- Entries apply in order ([§2.2](../../2-conventions/2.2-write-batches.md)), so a batch adding one node twice is refused at the repeat, "node
  exists" ([§2.10](../../2-conventions/2.10-existence.md)).

Ops:

- [`add-claim`, `add-analysis`, `add-experiment` (write ops)](add-claim-add-analysis-add-experiment.md)
- [`add-work` (write op)](add-work.md)
- [`add-axiom` (write op)](add-axiom.md)
- [`edit-claim`, `edit-analysis`, `edit-experiment`, `edit-work`, `edit-axiom` (write ops)](edit-claim-edit-analysis-edit-experiment-edit-work-edit-axiom.md)
- [`remove-node` (write op)](remove-node.md)
- [`move-node` (write op)](move-node.md)
