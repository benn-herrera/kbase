### A file kbase generates is regenerated whole

The summary pages and the sheets are kbase's output, each rewritten from its source whatever it
contains. The stamped `AGENTS.md` and `CONVENTIONS.md`, and a build's `README.md`, each contain one
maintained block between two fence lines, which kbase rewrites whole from its template on every
`update-format` and wherever `init` or the build writes the document
(SPEC §3 Stamped documents). A document without
the pair is reported `unfenced` and left as it is; its owner adds the fences. Pitfall:
merging new text into an existing block — a matcher for the old text, both forms of every
passage and a warning for each one not found, for text no user edits.
