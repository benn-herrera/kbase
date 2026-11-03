### `claim-graph.svg`

A picture of the claim graph, drawn through Graphviz `dot` ([§2](../../2-given-interfaces/index.md)). `update-format`, `init` and a write batch's
derivation render every sheet on each run and write one only where its bytes change. The SVG's bytes are
Graphviz's; no check reads them for their content — the sheet writer's test writes one DOT twice through
the same `dot` and asserts identical bytes, the write-only-where-bytes-change rule's basis — and no
other command's behaviour depends on them (ARCHITECTURE §6).

Contents:

- [§3 Three kinds of sheet](three-kinds-of-sheet.md)
- [§3 What is drawn](what-is-drawn.md)
