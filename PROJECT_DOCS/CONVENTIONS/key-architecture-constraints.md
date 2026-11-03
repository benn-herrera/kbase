## Key Architecture Constraints

The invariants and principles in ARCHITECTURE "Invariants" and "Principles" bind every code change;
violating any is a blocking defect. They are deliberately not restated here — ARCHITECTURE.md is the
single source. Working habits they impose:

- Before writing code, have those two sections fresh in context; check your change against them
  before considering it done.
- When a requested change collides with an invariant or principle, stop and surface the collision —
  do not quietly pick a side.
- Constants live in ARCHITECTURE "Constants" and as named constants in code; never introduce a magic
  number alongside them.

---

