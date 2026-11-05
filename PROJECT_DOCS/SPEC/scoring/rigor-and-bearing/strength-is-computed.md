## Strength is computed

[STRENGTH_SCORING_SCHEME.md](../strength/entry-point.md) owns strength propagation — the sets, the `⊥` table, the
empty-`P(n)` rule and its consequences — and this page changes none of it. One consequence bears on a
reader of a `rigor` value: an empty `P(n)` means no premise carries the node, so a claim's strength is
`0` however good its statement.

**What a bare assertion earns.** "Bare" means a claim whose statement fires nothing: its rigor is the
maximum the statement vocabulary reaches, `1.00`, and its strength is decided by its edges. A claim
whose statement is impeccable and which nothing supports is `1.00` rigor and `0.00` strength; with one
supporting edge it keeps its `1.00` and takes the edge's contribution as its strength, unless that
product exceeds `1.00`, in which case its own statement quality is the cap. An analysis reads the same,
with its method rigor in place of `1.00`. The statement quality is kept in every case; what varies is
whether anything carries it. The ladder reads `strength` and not `rigor`, so this pair is not a band:
its strength is `0.00`, and [the band outcomes](the-band-outcomes/index.md) bottom out there unless a lift
raises it.

