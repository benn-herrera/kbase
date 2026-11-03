#### `cancel` (tool + CLI)

This signals the run that holds the state store, of any kind, and waits for it to let go. The item
in flight is abandoned with nothing written for it, and the run exits `cancelled`. It continues only
when asked to: `kbase build --resume` or `kbase score --resume`.

- **Arguments**: `state-dir`.
- **Result**: `state-dir`, `pid`, `resume`.
- **Refusals**: no run holds the store (`check: lock`).
