# THESIS - kbase

*Why the KB build pipeline has the shape it has.*

---

## The thesis

**A well-structured argument inscribed in LaTeX is not raw text to be mined. It is a tree that has
already been flattened, and distillation is deserialization.**

Dump a program's tree data as JSON and you get a flat file; parse it back and you get the tree. A
LaTeX document set is that flat file. The author built a hierarchy and the format serialized it, so
the hierarchy is present in the bytes rather than latent in the prose.

Two hierarchies are encoded this way, and they are independent.

## The navigation hierarchy

Titles, parts, chapters, sections, subsections. These are hierarchy encodings and nothing else: each
one names a node and its position, and the nesting is the edge set. Breaking them into documents
with children linked under their parents is a transcription, not a judgment.

Mechanical in the testable sense: a second implementation produces the same tree.

## The argument hierarchy

The spine is also mechanically encoded, by two different constructs.

**Nodes** LaTeX environments for theorems, lemmas, definitions, propositions and displayed equations
carry auto-assigned identifiers. A claim that got an environment got an identity, and that identity
is in the file.

**Edges** A reference to one of those identifiers is written in `\ref`-family syntax, which names
the reference target explicitly. A ref can not fully determine an edge mechanically as it may be a
mention, supported by, or in support of the citation. It does indicate an edge candidate, however.

**For an ideally transcribed paper, that captures all nodes and edge candidates.**

## Why the ideal does not hold

We don't get to choose our inputs, and authors frequently do not transcribe arguments into LaTeX
exhaustively. Under-marking is the issue refinement via inference resolves.

## What inference is for

**Inference refines a mechanical draft. It never produces one.** Both hierarchies come out of the
mechanical pass first.

**Claim-graph refinement is the half that needs bounding**, and it closes exactly three gaps, all
nameable:

1. A claim constructed in prose without a formal delimiter — real, argued, and carrying no
   environment. (nodes)
2. A reference made in prose without `\ref` syntax — inside either a prose-defined claim or a
   properly delimited one. (additional edge candidates)
3. Edge existence and direction - mention, supported by, in support of (exactly 3 options)

In the final claim graph construction, base claims sit at the bottom, asserted de novo, or resting
on a citation to a work outside this document set. The rest of the edges connect internally,
resulting in a directed graph whose nodes and edges were drafted mechanically and refined by
inference.

## Where we came from and where we are going

The first prototypes of KB building took a full day of combined human/frontier AI model work and resulted
in something that had a considerable settling-in period. Scoring the claim graph was on the order of hours.

The mechanical process to get the spine now takes seconds, and the current state of R&D has the inference
portion of a KB build down to a couple hours of local inference with a result that still
requires settling in, but is more structurally sound than the prototype. Scoring a full KB on high-end local
inference (a 128 GB machine) takes minutes (again, WIP).

The goal is to get to a ~30 minute KB build time for an average arXiv math paper using models that can be
hosted on a 36GB M3 Max MacBook Pro. Scoring via local inference on the same machine should only take 
five minutes. This would make kbase's utility available on a standard middleweight notebook commonly found
in university labs and on researcher's desks.

If we could make it work even moderately well on a 16GB M3 MacBook Air, we've made it available to students.
