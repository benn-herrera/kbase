# {project-name} knowledge base

This tree is a knowledge base with a verified claim graph over it. It has agents. Use them.

- **To read it** — load the `kbase` skill, which is how this KB is reached: it locates the tree and
  routes to reading, maintenance or building. Its reading route navigates the hierarchy, pulling
  only what the question needs. Do not walk the tree yourself: it is built to be navigated, and
  reading it breadth-first spends context the answer never needed.
- **To change anything in it** — dispatch `kb-maintainer`. It calls kbase's metadata write ops,
  wires the claim graph, and runs the refresh-then-verify loop that has to end green. Where the
  harness has the `kbase` MCP server, its tools are kbase's subcommands — same names, same result
  documents — so what this tree's documents say of a subcommand holds for its tool. `rewind-build`
  is the one exception: no subcommand stands behind it; read-only, it reports the build's trail and
  changes nothing.
- **Do not edit files here directly.** Every structured field was written by a tool from supplied
  values, and much of what looks editable is derived and regenerated. An edit made outside the
  maintainer is either overwritten or caught as drift.

## What this tree holds

{scope-pin}

A leaf built from a corpus is a verbatim, mechanical rendering of it — nothing here rewrites
source text in place, so there is never a second copy for a corpus and the KB to disagree over.
Text a maintainer adds directly — a summary, a rationale, an analysis — lives in its own place in
the tree; it was never a distillation of a corpus, so it is the KB's own word, not a claim about
what any corpus says. For live node and edge counts — what is in the graph right now — run
`kbase stats`.

Corpus invariants — notation and cross-cutting definitions — live in `invariants.md` beside this
file. Read it before reasoning about anything here. Where there is no such file none is declared,
and nothing here is waiting on it.

`CONVENTIONS.md` beside this file is the full contract: what is authored versus derived, how
metadata gets written, how citations must be formed, and what to do when the gate goes red. Read it
before any work on this KB beyond reading.
