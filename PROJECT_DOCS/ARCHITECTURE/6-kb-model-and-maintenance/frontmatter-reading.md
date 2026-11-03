### Frontmatter reading

`internal/kb` takes a leading `---` block as frontmatter only when its
first non-empty line is key-shaped (`name:`, kebab-case); a block that then fails to parse as a YAML
mapping is the malformed-frontmatter refusal, and a block whose first line is not key-shaped is
prose, the document having no frontmatter (SPEC §10). The one frontmatter value kbase reads is the
superseded stamp in the entry point (`kb.FormatStamp`), read only where `kb.FormatFile` does not
stand ([§10](../10-compatibility-harness.md)); `verify`'s `frontmatter` check uses the same recogniser
(`kb.FindFrontmatter`) to report a block on any document, the entry point included, and the build's
conformance check (`claimgraph`) refuses any document that carries frontmatter at all.

