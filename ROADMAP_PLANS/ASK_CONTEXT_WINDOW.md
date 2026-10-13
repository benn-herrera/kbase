# Positional context for the claim-graph asks

Parked: picked up after the pipeline rework (`ACTIVE_PLAN.md`) has run live once, because the
rework takes the larger part of this idea — proximity in the shortlist's ranking, adjacency within
a leaf as a link by rule, and the pair rather than the node as the unit the culling ask judges —
and what remains is only worth measuring on that pipeline's record.

## What remains

- **The ask's window.** The co-linearity and direction asks show the two claims of a link. A
  four-window in dependency order — one neighbour beyond each end — gives the classifier the
  adjacent context an incremental step is recognisable by. Dependency order, not document
  order: the two coincide only for the interstitial case, and a document neighbour that is not an
  argument neighbour is shown as context it is not.
- **Position as a prior in the direction station.** Within one document the later claim depends
  on the earlier one nearly always, and a marked reference settles it outright. A mechanical
  prior there skips asks; it is trusted only after being read against the three-way ask's
  answers on the same links.

## The reading it waits on

The rework's first live run leaves a co-linearity record and a direction record over every link.
Where the misses against the curated reference cluster positionally — isolated steps whose
neighbours were out of view — the window has its case with numbers in hand. Where they do not,
it is a solution without a demonstrated problem and is dropped. The sweep tooling in
`tools/measure` scores rankings against the curated edges only; the chain links proximity
recovers have no reference there and are read from the records.
