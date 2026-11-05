### `claim-graph.svg`

A picture of the claim graph, drawn through Graphviz `dot` ([§2](../../2-given-interfaces/index.md)). Each such diagram is a **sheet**.
`update-format`, `init` and a write batch's rebuild of the derived views render every sheet on each
run and write one only where its bytes change. The SVG's bytes are Graphviz's; no check reads them
for their content, and no other command's behaviour depends on them.

Contents:

- [§3 Three kinds of sheet](three-kinds-of-sheet.md)
- [§3 What is drawn](what-is-drawn.md)
