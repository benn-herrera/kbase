#### Bearing predicates

**Novel.** The two predicates separate which edges carry a bearing from which a score run asks about,
which realizes THESIS item 1 (the four scoring relations) as defined in SPEC §10 Four scoring relations,
one arithmetic.

**Pattern:** Filtered work set.

Two predicates in `internal/kb` answer whether an edge carries a bearing, and they are not the same
question. `kb.Scoring`, over the four relations in `kb.ScoringRelations`, is whether an edge carries a
bearing at all. The readers of a stored bearing use it: the write package's relation checks, the
derived views and strength propagation in `internal/index`, `internal/query`'s trace and payloads, and
`bearingText`. `kb.BearingWalk`, over the three in `kb.BearingWalkRelations`, is whether a score run
asks for that bearing. Its readers are `query.ScoringSet`'s bearing pass, `namedEdge`, the
`manually-scored` flag on edge rows (`internal/index/derive.go`) and its description in the write
package, and `stats`' `walked` key. A `founds` edge carries a bearing and is never an item, its source
lying outside the corpus, so `edit-edge` alone writes it.

