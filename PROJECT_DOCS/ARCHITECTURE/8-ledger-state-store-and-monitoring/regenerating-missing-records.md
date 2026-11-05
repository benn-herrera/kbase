### Regenerating missing records

**Pattern:** Reproducible-build verification.

The document graph's records live in the state store. A resume that finds them absent re-runs the
document-graph stage into scratch (`walk.ensureRecords`) over the recorded inputs and requires its tree
byte-identical to the committed `kb-root/` (`firstDifference`); a difference is `failed`, naming the
first differing path (SPEC §5 Determinism).

