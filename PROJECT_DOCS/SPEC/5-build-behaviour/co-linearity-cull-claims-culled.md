### Co-linearity cull (`claims-culled`)

The stage walks between `references-found` and
`depends-attributed`, and removes prose claims that state no step a linked claim does not. Its
candidates are the links — every marked pair, every pair the unmarked-reference ask answered yes in
either direction, and every adjacency link, each unordered — with at least one end a prose claim.
Each candidate gets one co-linearity ask, shown both claims in reading order and offered three
answers: the second adds nothing to the first, the first adds nothing to the second, or each is a
step of its own. An answer naming a claim as adding nothing — answered, or
answered after a re-ask — culls it into the other, except that a block claim or an equation node is
never culled: where the answer names one, the prose claim of the pair is culled into it instead.
One whose ask yields no readable answer after the re-ask is recorded `defaulted` and both claims
keep. A claim several answers cull goes into the earliest of their partners in reading order;
following those to a claim culled into nothing gives its root. A claim whose root is not the claim
it was culled into is asked once more against its root and kept unless that answer names it as
adding nothing; no claim is asked a third time. A culled claim is removed — its outgoing and
inbound edges, then its node record and its statement marker — and no edge is added. The stage asks no
direction: every inferred link that survives, one with a culled end re-pointed onto that end's root,
goes undirected to the cull record ([§6](../6-build-state-ledger-and-monitoring/index.md)) for dependency attribution. Under `--no-inference` the stage
is still walked with its row dropped: no ask is made and the cull record stands as the declared
pass wrote it.

