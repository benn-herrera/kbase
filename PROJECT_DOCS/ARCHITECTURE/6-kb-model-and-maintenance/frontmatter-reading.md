### Frontmatter reading

**Pattern:** Heuristic sniffing before parse.

`kb.FindFrontmatter` takes a leading `---` block as frontmatter only when its first non-empty line is
key-shaped (`name:`, kebab-case). A block that then fails to parse as a YAML mapping is a
`MalformedError`, the malformed-frontmatter refusal; a block whose first line is not key-shaped is
prose, the document having no frontmatter (SPEC §10 Recognising frontmatter).

The one frontmatter value kbase reads is the superseded stamp in the entry point (`kb.FormatStamp`),
which `internal/kbload` reads only where `kb.FormatFile` does not exist ([§11](../11-package-map/loading.md)). `verify`'s
`frontmatter` check uses the same recogniser to report a block on any document, the entry point
included. The build's conformance check (`claimgraph`) refuses any document whose frontmatter has a
field.

