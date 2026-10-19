# A corpus survey of what an ask must see

Parked: the next inference-improvement session. The pass before it chose candidate traits by
intuition and elaborated variants of them; nothing in its record could say the line was exhausted,
so it ran long. This one starts from the corpus: read it whole, and let per-paper prevalence say
which commonalities are regularities and which are one author's habit.

## Inputs

Fifty organic KBs, supplied by the operator at run time. No path is cited here: the corpus lives
outside the repository and is not tracked.

## The two questions, kept apart

- **Structural.** What the KB encodes that a machine can compute: claim granularity, link density,
  which records are load-bearing, where prose carries what records do not.
- **Reader's.** What a competent reader must have in hand to act on a passage. This is the target;
  the structural question only supplies the handles the pipeline can pull.

Pooling them is what produces narrative categories instead of features, so the survey states each
row under one heading or the other.

## The split

Before any agent reads a paper, partition the fifty: two thirds derivation, one third holdout,
chosen by a recorded rule (a seeded shuffle is enough) and written down. The partition does not
change afterwards — a re-split in light of the results makes the holdout worthless.

## The survey

One agent, sequentially (local inference allows no fan-out), over the derivation papers only. It
reads each KB's records and prose — not the current selection code, which would anchor it to what
already exists — and returns one table, a row per candidate feature:

| Column | What it holds |
|---|---|
| Feature | A name and one sentence of claim |
| Definition | The field, key, link or prose property it is computed from, precise enough for another agent to implement |
| Prevalence | Per paper: count and the paper's identifier |
| Sites | Two quoted examples per paper |
| Ask | What an ask would have to see for the feature to matter |

A row without a computable definition is dropped, not graded. "The claims cluster around their
methods" is not a feature; "the claim's own record names a method that no dependency links to" is.

## The holdout pass

An agent that has not seen the survey scores every row against the holdout papers: does it occur,
at what prevalence, and how many papers carry it. A row present in fewer than half the holdout
papers is that corpus's habit and is struck. Prevalence is what this pass can supply; whether a
feature would change an answer is a judgment it cannot make without recorded answers, and where the
corpus holds none the surviving rows go forward marked `impact: unmeasured`.

## Selection

Rank what survives by holdout prevalence and by distance from what today's pipeline already does
(`internal/asks/prompt-templates`, `internal/claimgraph/shortlist.go`). The top two or three become
one measured change, each with the observation that would falsify it. That change is the next
session's work, not this one's.

## The stop condition

Written before the session starts, and read at its end:

- **No row survives the holdout in at least half the papers.** The survey reports that and the line
  closes: the corpus offers no cross-paper regularity in what asks need to see, and the next
  session starts elsewhere.
- **Rows survive.** The line continues only through the measured change. Another survey is not a
  result.

## Artifacts

The feature table and the partition record are committed beside this plan; they are the evidence
the next step rests on, and scratch is not. The session's dispatch brief gets prompt-engineer review
before an agent reads it.
