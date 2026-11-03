## 2. Given interfaces

| Interface | Observed version | Contract |
|---|---|---|
| pandoc | 3.12 (`pandoc-api-version` 1.23.1.2) | Required on the host, not shipped. |
| Graphviz `dot` | Any version | Optional on the host, found on `PATH`, not shipped. Draws the claim-graph sheets ([§3](../3-compatibility-contract/index.md)); absent, the command that renders the sheets writes the placeholder at the root only and its result says so ([§3](../3-compatibility-contract/index.md), [§7](../7-tool-result-contract-and-exit-codes/index.md)). |
| git | 2.56.0 | Required on the host. The recoverable record of a build ([§6](../6-build-state-ledger-and-monitoring/index.md)). |

Contents:

- [§2 pandoc](pandoc.md)
