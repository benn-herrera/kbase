### A file kbase generates is regenerated whole

The register pages and the sheets are kbase's output, each rewritten from its source whatever it
holds. The stamped `AGENTS.md` and `CONVENTIONS.md`, and a build's `README.md`, each hold one
maintained block between two fence lines, which kbase rewrites whole from its template on every
`update-format` and wherever `init` or the build writes the document
([page §1.7](../../../NEW_MCP_CLI_DESIGN/1-model/1.7-maintained-blocks.md)). A document without
the pair is reported `unfenced` and left as it stands; its owner adds the fences. The trap:
merging new text into a block that stands — a matcher for the old text, both forms of every
passage and a warning for each one not found, for text no user edits.
