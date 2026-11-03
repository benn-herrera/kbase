### `canonical_path` and `canonical_anchor`

Each node's derived row carries a canonical path and anchor, which the claim-graph sheet links to
and the scoring run reads a node's text through; no result carries them, `show` and `find` naming
`host`, `host_anchor` and `summary` instead ([§7](../7-tool-result-contract-and-exit-codes/item.md)). A claim's are its register page and its entry's anchor
there; a work's the root register page and its entry's anchor; an analysis's and an experiment's
the leaf hosting it and the anchor of the heading of the section holding its marker; an axiom's
`axioms.md` and the slug of `Axiom <label>: <title>`. Anchors are
[page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md)'s. A node with no host has neither.
