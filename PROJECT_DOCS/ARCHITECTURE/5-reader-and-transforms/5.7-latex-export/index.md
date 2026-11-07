### 5.7 LaTeX export

**Pattern:** Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked
output directory.

The mechanism of `export-latex` (SPEC §8 `export-latex`), which defines the files of `gen-tex/`, the
content rules, simplification and idempotence. The export emits no records, so pandoc's own filter
mechanism suffices where [§5.3](../5.3-json-transforms.md) uses Go, and the author can re-run the
conversion with plain pandoc.

Contents:

- [§5.7 One run, three owners](one-run-three-owners.md)
- [§5.7 Reader](reader.md)
- [§5.7 Streams and file scope](streams-and-file-scope.md)
- [§5.7 Document boundaries](document-boundaries.md)
- [§5.7 Tables](tables.md)
- [§5.7 The filter](the-filter.md)
- [§5.7 Labels](labels.md)
- [§5.7 Theorem blocks](theorem-blocks.md)
- [§5.7 Template](template.md)
- [§5.7 `gen-tex/` and the preamble](gen-tex-and-the-preamble.md)
- [§5.7 Idempotence](idempotence.md)
- [§5.7 The report](the-report.md)
