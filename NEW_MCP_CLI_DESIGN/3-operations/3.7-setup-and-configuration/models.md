#### `models` (tool + CLI)

This lists the model ids a provider's catalogue offers, sorted.

- **Arguments**: `provider` (default: the only provider), `timeout`.
- **Result**: `provider`, `provider-rule`, `provider-layer`, `models`, `warnings` (one item per
  configuration fault, `path` the file or directory it is against, `[]` where none, on every
  outcome).
- **Refusals**: several providers and none chosen (`key: --provider`, `allowed` the names).
