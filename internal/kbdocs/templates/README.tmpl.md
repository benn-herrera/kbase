# {project-name} Knowledge Base

{overview-passage}

The passage above is a generated summary of this knowledge base's documents — written from excerpts
of them, not verified against them; the rest of this file is the build's own text.

## What Is Here

A `leaf` is translated source — the corpus's own text carried across, not a summary of it. The rest
is navigation the build wrote: the `entry-point` at the root, and an `index` in each directory that
holds documents beneath it. An index carries no translated source of its own — a section's own prose
is a leaf beneath it. Those three kinds are the documents, and nothing else is one: no
`claim-rigor.md` register, no `axioms.md`, not this file, `AGENTS.md` or `CONVENTIONS.md`, and
none of the images under `assets/` that leaves embed. Reading order is what the indexes themselves
carry.

## The Claim Graph Over It

This graph is over results rather than documents: what the corpus establishes, what each result
stands on, and which outside works it leans on.

A node is one of five kinds: `claim`, `analysis`, `experiment`, `work` or `axiom`. A claim, an
analysis and a cited work have an entry in a `claim-rigor.md` register — its title, its local
`rigor`, and a rationale for the judgement — and a claim and an analysis also carry the `strength`
derived from their premises. An entry for a result of this corpus links the document that states it.
A domain's register holds that domain's nodes; the register at this root holds the outside works the
corpus cites. An experiment is declared in the document that reports it, with its `status` and its own
pairs, and has no register entry. An axiom is declared in `axioms.md` beside the root and carries no
value of its own — its strength is `1.0` by definition — and prose may call one an invariant or a
framework premise.

Each edge between the nodes is of one class, and every scoring edge runs from what bears on the
argument to what it supports. `supports` runs from a claim, an experiment or an axiom to the claim or
analysis it supports. `founds` runs from a cited work — one this KB names and does not contain — to
the claim or analysis that uses it. `lifts` runs from an analysis or an experiment to the claim it
corroborates, raising that claim above what its premises alone would give. `refutes` runs from a
claim, an analysis or an experiment and caps the claim it attacks. Each of the four carries a
`bearing`: how much of the supporter's value bears on the target, a number from 0 to 1 or
{pending-literal}. A `references` edge records that one result's own text names another, and nothing
rests on it. A `demoted` edge is a `supports` edge the build's cycle breaking cut because it closed a
circle; it keeps its origin, cited or inferred, and nothing rests on it either.

[`claim-graph.svg`](claim-graph.svg) beside this file is drawn by `refresh` and `render-claim-graph`
and authored by nobody — a picture for a person to view; a session neither opens nor edits it, and
takes the graph from the query tools. It draws every volume as a cluster.
{digest-chunk}`<volume>/claim-graph.svg` beside each volume's `index.md` shows that volume and every
node one edge from it.
A sheet draws the four scoring relations and no `references` edge, and every `demoted` one as a cut;
every arrow points from the supporter to what it supports. Of the drawn `supports` edges only the
transitive reduction appears, so a missing stroke is not a missing edge — `deps` reports every edge
at a node. A `supports` edge is black solid where cited (the paper's text marks the dependency) and blue
dashed where inferred (the build found it with no mark in the text); `founds`, `lifts` and `refutes`
each have a stroke of their own; and a cut is red dotted, heavier where the cut edge was cited,
lighter where inferred. A node's shape gives its kind — a labelled block with a bold frame, an
equation, prose, an analysis, an experiment, an axiom, a work — and a claim's or an analysis's fill
is its strength band. Nodes with no drawn edge are listed as text, volume by volume. Every node links
to its register entry, and a legend keys the rest.

A claim, analysis, work or experiment with no `rigor` reads {pending-literal} wherever it appears,
and so does the `strength` of a claim or an analysis whose value cannot be derived yet: that says
*unscored*, not *low* and not *doubted*. A {pending-literal} bearing takes its edge out of the
derivation until it is judged, and a bearing of `0` is a judgement that participates.

## Current Figures

This file states no counts, because the KB changes after it is written. The current ones come from
`kbase stats`, run from the project root that holds this KB.

`kbase stats` counts the claim graph: one count per node kind — `claims`, `analyses`, `experiments`,
`axioms`, `works` — then `edges`, every edge of every class, and `demoted_edges` beside it. The claims
per band are under `strength_bands`: the {pending-literal} band holds the unscored claims, and every
other band holds scored ones.

It does not split the edges by class. `deps` reports each edge at a node with its `relation`, its
class, and its `bearing`.

No command counts the documents: `stats` counts the claim graph and `verify` reports faults.

## Reading It

Don't walk the tree by hand — it is built to be navigated, and a breadth-first read spends context
the answer never needed. Reach it through the `kbase` skill, which finds this KB and routes to the
reading reference.

[`AGENTS.md`](AGENTS.md) beside this file says how this KB expects to be read and changed.
[`CONVENTIONS.md`](CONVENTIONS.md) is the full operating contract: what is authored versus derived,
how the gate works, and how citations are formed.
