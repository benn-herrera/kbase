### Reading the stamp

Before any other read, kbase reads `kb-format` from
`kb-root/.index/format.yaml`; where that file does not stand, from the entry point's frontmatter,
the superseded place — and where both stand, the file's stamp is the KB's. A KB with neither stamp
file nor entry point is refused (`check: kb-root`) as it is today; a KB stamped in neither place,
and an unreadable stamp in either, are refused (`check: kb-format`), the refusal naming where it
looked. Every producer of a KB stamps the file: the build's spine, `init` at birth, and every save
(below). A stamp at the current major and minor with another patch is read as current and
rewritten on the next save.

The current version is `4.0.0` ([page §1.8](../../../NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md)).
No converter leads to a patch, so a migration chain stops at the
form its last converter produces and the next save supplies the stamp: a `2.0.0` and a `2.0.1` KB
both convert through the one `2.0.x → 3.0.0` converter (below), `2.0.1`'s patch having changed only
the words of the band ladder's bottom rung on a derived line.

