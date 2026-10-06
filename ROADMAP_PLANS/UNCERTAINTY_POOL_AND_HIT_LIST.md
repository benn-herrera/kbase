# The uncertainty pool and the hit list

**Status:** design notes, not a plan; taken up after the inference-passes plan (`ACTIVE_PLAN.md`)
has its first measured experiment, because the pool's size depends on how well the asks decide.
Nothing here changes the KB's metadata formats.

## The goal, stated so it does not violate information theory

A build is a one-time bootstrap. Every dependency it fails to recover and every one it asserts
wrongly becomes work for the person who takes the KB over — finding a missed edge by reading,
noticing and removing a false one — and the goal is to drive that work toward zero.

Two parts of it are different in kind:

- **Irreducible:** information absent from the source. Authors under-mark (THESIS); a dependency
  that exists only in the author's head is in no text, and no reader recovers it. This is the floor,
  a property of the paper.
- **Eliminable:** the *search*. The build already computes its own uncertainty — pairs it considered
  and did not assert, items it could not decide, edges whose direction it could not settle — and
  today discards most of it. Kept and surfaced, that uncertainty turns an undirected read of the
  corpus into adjudication of a list.

So the target is **zero human search; human adjudication only over what the build declares
uncertain**, with the floor measured rather than assumed.

## The pool is declared uncertainty, not every negative

A confident `B` is a decision and never enters the pool; a build that put every negative on the list
would hand a 400-node work some 2,000 pairs and be worse than reading the paper. A member earns its
place by having evidence on both sides:

- a shortlisted candidate the build **defaulted** — no offered letter after the re-ask;
- a **`references`** edge — a relationship asserted, direction undecided — and every ring demotion;
- a **`B` with strong shortlist evidence** (top rank, high cosine): the near miss;
- a claim with **no incoming support and a `*pending*` score**, where the text gave the build
  nothing.

Everything else the build decided; its decisions are its output, not its agenda.

## Two measures, carried as build quality

- **Pool size per node.** Its target scales with the floor — the author's edges that have no textual
  evidence — not with the corpus. A pool half the node count means the build is deferring decisions
  it should make, which is the recall-at-any-cost ask's failure in another form.
- **Pool yield.** Of the author's edges the build missed, how many a reader finds in the pool rather
  than in the corpus. A small pool with low yield is a failure of a different kind.

Both need the comparison against an author's own graph to split misses into *evidence present,
unused* (ours, reducible) and *no evidence in the text* (the floor). Until that split exists,
"recall" conflates the build's shortfall with the paper's.

## What exists already

Every fact the pool needs is state the build keeps:

| Fact | Where it lives today |
|---|---|
| Shortlisted pairs, their rank and letter, defaults with cause | `kb-build-unmarked.json`, tracked at the repository root (SPEC §6) |
| Unjudged paragraphs, defaulted items | `kb-build-node-pass.json`, `kb-build-classification.json` |
| `references` edges, ring demotions | the registers; the classification record |
| Claims without support; `*pending*` scores | `.index/*.jsonl` |

Missing is one reading of them together. **The hit list** is a query — a subcommand over the three
build records and the index — emitting the agenda in SPEC §7's item shape: each member with its
evidence (rank, cosine, letter, cause) and no grade, which is the Dream's "map, not verdict" held by
construction. The same list serves the author's recovery after a build and a reviewer's attention
over a finished KB.

## What the build must keep for it

The unmarked record is written for resume. Before the hit list is built, check that it also carries
what the agenda needs for each planned pair: its shortlist rank and score, the letter or the
default's cause, and enough to show the reader the evidence the ask saw (the claim and candidate
lines). A record that lacks any of these is a records-channel finding, fixed in the stage that
writes it, never recovered from the page.

## Levers on pool size

The same ones the inference experiments pull: an ask that decides more pairs correctly leaves fewer
defaults and fewer high-ranked `B`s; a shortlist that ranks true pointers higher turns near misses
into asked pairs. Pool size is therefore measured after each tactic change, beside the recall and
precision numbers, and a change that improves recall by enlarging the pool has not improved the
build.

## Open questions

1. The near-miss threshold — which `B`s enter the pool — by rank, by score or by a margin against
   the `A`s of the same source. Set from the measured distribution, not ahead of it.
2. Whether the hit list carries a reader's adjudication back into the KB (a write op that promotes a
   pool member to an edge or dismisses it), or stays read-only with the existing write ops doing the
   landing.
3. Where a reviewer's list differs from an author's: the reviewer wants unsupported claims first,
   the author wants undecided edges first. One query with an ordering flag, or two.
