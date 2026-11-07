## 5. Reader and transforms

**Pattern:** Pandoc as the parser; Go transforms over its JSON AST.

pandoc parses and kbase never does. `docgraph.Build` converts each top `.tex` file (one volume) in
this order, then writes the trees:

1. `prepass.Apply` scans the source text ([§5.2](5.2-pre-pass-registry.md)).
2. `pandoc.ReadLaTeX` parses it to the JSON AST ([§5.1](5.1-pandoc-seam.md)).
3. `readVolume` walks the AST once and derives the outline and the records ([§5.5](5.5-records.md)).
4. `transform` rewrites the AST ([§5.3](5.3-json-transforms.md)).
5. `pandoc.WriteGFM` renders it ([§5.4](5.4-writer-route.md)).
6. `buildTree` splits the rendering into documents and `place` places the records in them.
7. Five checks compare the products ([§5.6](5.6-tree-derivation-partition-checks-and-validate-build.md)).

After the last volume, `writeTrees` writes the tree and the entry point, `validateBuild` and the
dead-link check read it back, and `internal/build` rewrites each link wrapped around a
cross-reference to its text (`unwrapTree`). Every step is deterministic and runs no model.

Contents:

- [§5.1 pandoc seam](5.1-pandoc-seam.md)
- [§5.2 Pre-pass registry](5.2-pre-pass-registry.md)
- [§5.3 JSON transforms](5.3-json-transforms.md)
- [§5.4 Writer route](5.4-writer-route.md)
- [§5.5 Records](5.5-records.md)
- [§5.6 Tree derivation, partition checks and `validate_build`](5.6-tree-derivation-partition-checks-and-validate-build.md)
- [§5.7 LaTeX export](5.7-latex-export/index.md)
