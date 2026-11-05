### A build records no judgement

Every scoring edge a build writes has `*pending*` as its
bearing, and no stage writes a `rigor` or any other number into a score; every value a build writes
is `manually-scored: false` ([§10](../10-metadata-format-version/provenance-and-the-judged-hash.md)). The judgement is a person's,
through `edit-<kind>` with `rigor` and `edit-edge` with `bearing`, or a scoring run's ([§8](../8-maintenance-subcommands/index.md)). A claim
a build attaches an edge to reads unchanged until that edge is judged.

