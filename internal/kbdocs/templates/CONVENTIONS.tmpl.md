# CONVENTIONS — {project-name} knowledge base

<!-- vvv-kbase the block below is kbase's and kbase rewrites it; text outside it is this KB's own kbase-vvv -->
How to behave when operating on this knowledge base.

## Authored versus derived

Authored, never regenerated: leaf content (a corpus leaf as the build rendered it, never rewritten
in place) and the statement markers in it, and kbase's records — nodes, their local `rigor`,
rationale and the rest, and edges with their `bearing`. Derived and regenerated: each claim's and
analysis's strength, build status, subtree aggregates, the `kb-summary.md` pages and the
claim-graph sheets. The records and the statement markers are written only by a write op, never by
hand.

In an authored file, a reference to a node is a structured field, a marker or a link, never an id
loose in prose.

## The maintained documents

`AGENTS.md`, `CONVENTIONS.md` and `README.md` in this tree each hold one block that kbase
maintains, between a `vvv-kbase` marker line and a `^^^-kbase` one: no agent edits inside it, and
kbase rewrites it whole. Text outside the block is this KB's own. `CLAUDE.md` is kbase's
redirect to `AGENTS.md`. A corpus's axioms go in `axioms.md`.
<!-- ^^^-kbase the block above is kbase's kbase-^^^ -->
