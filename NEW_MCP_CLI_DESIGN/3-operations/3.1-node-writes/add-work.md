#### `add-work` (write op)

This records an external work and mints its id, `wrk-<minted>`. Its citation key is its `label`
([§1.1](../../1-model/1.1-nodes.md)). The work is shown on the root summary page.

- **Keys**: `label` (the citation key, required), `title` (required), `rationale` (required),
  `rigor` (optional; default `*pending*`; no manual mark).
- **Result**: `id`, minted.
- **Refusals**: a `label` that is not a citation key (it must open and close alphanumeric,
  [§1.1](../../1-model/1.1-nodes.md)); a work of that label or that title that already stands ([§2.10](../../2-conventions/2.10-existence.md)); `rigor` out of
  domain.
