#### `remove-edge` (write op)

This removes the edge record that the entry names. With `relation: demoted`, it removes every
demoted record from source to target, leaving the edge demoted away. Every other record stays byte
for byte as it was.

- **Keys**: `source`, `target`, `relation` (any of the six; required).
- **Result**: `source`, `target`, `relation`.
- **Refusals**: no such record ([§2.10](../../2-conventions/2.10-existence.md)); a `source` or `target` no node carries; a scoring relation
  that does not join the two nodes' kinds.
