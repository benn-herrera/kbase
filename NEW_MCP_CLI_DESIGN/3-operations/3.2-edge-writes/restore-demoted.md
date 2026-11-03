#### `restore-demoted` (write op)

This puts back exactly the edge each demoted record from source to target replaced, with its
relation, context, bearing and mark, and deletes the record.

- **Keys**: `source`, `target`, `relation` (must be `demoted`; all required).
- **Result**: `source`, `target`, `relation`, `restored` (the relations put back). No demoted
  record from source to target is refused ([§2.10](../../2-conventions/2.10-existence.md)).
- **Refusals**:
  - any relation other than `demoted`;
  - a restore that would close a cycle (`check: dependency-cycle`, naming the cycle): remedy,
    remove another edge of the cycle first.
