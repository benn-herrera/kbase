#### `wait` (CLI only)

This returns when the run with the given pid, of any kind, is no longer progressing, then prints
that store's `status` document. It takes no lock.

- **Arguments**: `--pid` (required, positive), `--state-dir`.
- **Refusals**: a missing or non-positive `--pid` (usage).
