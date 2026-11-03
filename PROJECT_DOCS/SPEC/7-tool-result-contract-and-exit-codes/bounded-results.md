### Bounded results

`find`, `edges`, `trace` and `citations-of` return their complete answers ([page §2.8](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)), `find`'s and
`trace`'s `--limit N` allowed only beside `--sort`. No list query pages. `stats`' ranked sections stay at 10, and the `bearings` of its `relations` hold
one record per distinct bearing value. No other subcommand's result grows with the KB; `build`'s
per-stage detail goes to the `report` file, not stdout.

**Exit codes** derive from `outcome`:

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success, including a `--through`-bounded run |
| `refused` (including verify faults and usage errors) | 1 | Wrong input or KB state; nothing written past the last checkpoint |
| `retry` | 2 | Another writer holds the KB write lock past the wait (`check: lock`, `path` the repository root), or another run holds the state store (its holder lock, [§6](../6-build-state-ledger-and-monitoring/index.md)); re-issue identically later. A run holding the operation lock is not `retry`: it is `refused` with the `lock` item ([§6](../6-build-state-ledger-and-monitoring/index.md)) |
| `failed` | 3 | Defect in the tool, its input or the environment; re-issuing will not fix it |
| `cancelled` | 4 | Stopped on request; resumable |

---

