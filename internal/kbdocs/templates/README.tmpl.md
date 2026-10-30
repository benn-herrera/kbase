# {project-name} Knowledge Base

{overview-passage}

The passage above is a generated summary of this knowledge base's documents — written from excerpts
of them, not verified against them; the rest of this file is the build's own text.

## What Is Here

A `leaf` is translated source — the corpus's own text carried across, not a summary of it. The rest
is navigation the build wrote: the `entry-point` at the root, and an `index` in each directory that
holds documents beneath it. An index carries no translated source of its own — a section's own prose
is a leaf beneath it. Those three kinds are the documents, and nothing else is one: no
`claim-rigor.md` register page, no `axioms.md`, not this file, `AGENTS.md` or `CONVENTIONS.md`, and
none of the images under `assets/` that leaves embed. Reading order is what the indexes themselves
carry.

## The Claim Graph Over It

This graph is over results rather than documents: what the corpus establishes, what each result
stands on, and which outside works it leans on.

A node is one of five kinds: `claim`, `analysis`, `experiment`, `work` or `axiom`. Each node is a
record kbase keeps. A claim, an analysis and a cited work carry a title, a local `rigor`, and a
rationale for the judgement; an experiment carries its title, its `status` and its `rigor`; and a
claim and an analysis also carry the `strength` derived from their premises. A claim, an analysis or
an experiment of this corpus carries a statement marker in the document that states it, and that
document's place decides where it is shown: the `claim-rigor.md` register page of the volume
holding the document, or the register page at this root for a result stated outside every volume.
The root page also shows the outside works the corpus cites. `refresh` generates every register
page from the records, for reading as prose; take values from the query tools, never from a page.
An axiom is recorded by its label and title and stated in `axioms.md` beside the root; it carries no value of its own — its
strength is `1.0` by definition — and prose may call one an invariant or a framework premise.

Each edge between the nodes is of one class, and every scoring edge runs from the node that bears
on the argument to the claim or analysis it bears on. `supports` runs from a claim or an
experiment. `founds` runs from a cited work — one this KB names and does not contain — or an
axiom. `lifts` runs from an analysis or an experiment and raises its target above what the
target's premises alone would give. `refutes` runs from a claim, an analysis or an experiment and
caps its target. Each of the four carries a
`bearing`: how much of the supporter's value bears on the target, a number from 0 to 1 or
{pending-literal}. A `references` edge records that one result's own text names another, and nothing
rests on it. A `demoted` edge is a scoring edge cut because it closed a cycle — by the build, or by
`refresh` repairing what a merge left; it keeps the edge it replaced, relation and bearing, so
`resolve-demoted` can restore it, and nothing rests on it either.

[`claim-graph.svg`](claim-graph.svg) beside this file is drawn by `refresh` and `render-claim-graph`
and authored by nobody — a picture for a person to view; a session neither opens nor edits it, and
takes the graph from the query tools. It draws every volume as a cluster.
{digest-chunk}`<volume>/claim-graph.svg` beside each volume's `index.md` shows that volume and every
node one edge from it.
A sheet draws the four scoring relations and no `references` edge, and every `demoted` one as a cut;
every arrow points from the node that bears on the argument to the node it bears on. Of the drawn
`supports` edges only the transitive reduction appears, so a missing stroke is not a missing edge — `deps` reports every edge
at a node. A `supports` edge is black solid; `founds`, `lifts` and `refutes` each have a stroke of
their own; and a cut is red dotted. A node's shape gives its kind — a labelled block with a bold
frame, an equation, prose, an analysis, an experiment, an axiom, a work — and a claim's or an
analysis's fill is its strength band. Nodes with no drawn edge are listed as text, volume by volume.
Every node links to where it is stated or shown — a claim's or a work's entry on its register page,
an analysis's or an experiment's section in the document that states it, an axiom's heading in
`axioms.md` — and a legend keys the rest.

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

Under `relations`, `kbase stats` splits the edges by relation and, on a scoring relation, counts the
edges at each bearing; `kbase edges --relation <relation>` lists them. `deps` reports each edge at a node with its
`relation` and its `bearing`.

No command counts the documents: `stats` counts the claim graph and `verify` reports faults.

## Reading It

Don't walk the tree by hand — it is built to be navigated, and a breadth-first read spends context
the answer never needed. Reach it through the `kbase` skill, which finds this KB and routes to the
reading reference.

[`AGENTS.md`](AGENTS.md) beside this file says how this KB expects to be read and changed.
[`CONVENTIONS.md`](CONVENTIONS.md) is the full operating contract: what is authored versus derived,
how the gate works, and how citations are formed.
