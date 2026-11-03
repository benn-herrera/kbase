### Recognising frontmatter

A document's leading `---` block is frontmatter only when its first
non-empty line is key-shaped: `name:` with a kebab-case name. Such a block that does not parse as a
YAML mapping is the malformed-frontmatter refusal. A leading `---` block whose first line is not
key-shaped (a prose paragraph, a list, the content after a thematic break) is prose, and the
document has no frontmatter. kbase reads frontmatter on the entry point alone, and there for the
superseded stamp alone ([§10](index.md)); `verify` reports a
frontmatter block on every document, the entry point included ([§8](../8-maintenance-subcommands/index.md)).

