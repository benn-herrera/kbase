### Co-linearity cull (`claims-culled`)

To **cull** a claim is to merge it into another claim; the survivor of a chain of merges is the
claim's root. The stage runs between `references-found` and `depends-attributed`, and culls prose
claims that state no step a linked claim does not. Its candidates are the links — every marked
pair, every pair the unmarked-reference model call answered yes in either direction, and every
adjacency link, each unordered — with at least one end a prose claim. Each candidate gets one
co-linearity model call, which shows both claims in reading order and offers three answers: the
second adds nothing to the first, the first adds nothing to the second, or each is a step of its
own. An answer naming a claim as adding nothing — answered, or answered after a retry — culls it
into the other, except that a block claim or an equation node is never culled: where the answer
names one, the prose claim of the pair is culled into it instead. A candidate whose call yields no
readable answer after the retry is recorded `defaulted` and both claims are kept. A claim several
answers cull goes into the earliest of their partners in reading order; following those merges to
a claim culled into nothing gives its root. A claim whose root is not the claim it was culled into
gets one more model call against its root, and is kept unless that answer names it as adding
nothing; no claim gets a third call. A culled claim is removed — its outgoing and inbound edges,
then its node record and its statement marker — and no edge is added. The stage decides no direction:
every inferred link that survives, one with a culled end updated to refer to that end's root, goes
undirected to the cull record ([§6](../6-build-state-ledger-and-monitoring/index.md)) for dependency attribution. Under `--no-inference` the stage
still runs with its model-calling steps dropped: no call is made and the cull record is as the
declared pass wrote it.

