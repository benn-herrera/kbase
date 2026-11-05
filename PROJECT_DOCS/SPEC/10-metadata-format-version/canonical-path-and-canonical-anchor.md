### `canonical_path` and `canonical_anchor`

Each node has a canonical path and anchor, derived at load, which the claim-graph sheet links to;
no result contains them, `show` and `find` naming `host`, `host_anchor` and `summary` instead
([§7](../7-tool-result-contract-and-exit-codes/item.md)).

| Node | Canonical path | Canonical anchor |
|---|---|---|
| claim | its summary page | its entry's anchor there |
| work | the root summary page | its entry's anchor there |
| analysis, experiment | the leaf hosting it | the anchor of the heading of the section holding its marker |
| axiom | `axioms.md` | the anchor of its marked heading |

A node with no host has neither.

