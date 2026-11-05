### Reading the stamp

Before any other read, kbase reads `kb-format` from `kb-root/.index/format.yaml`; where that file
does not exist, from the entry point's frontmatter, the stamp's superseded place, which every KB
older than `3.0.0` uses; where both exist, the file's stamp governs. A `kb-root/` with neither
stamp file nor entry point is refused (`check: kb-root`). A KB stamped in neither place, an
unreadable stamp in either, and a stamp that is not a `<major>.<minor>.<patch>` version are refused
(`check: kb-format`), the refusal naming where it looked. Every producer of a KB writes the stamp
file: the build, `init` and every save (below). A stamp at the current major and minor with another
patch is treated as current and rewritten on the next save.

No conversion leads to a patch, so a migration chain stops at the form its last conversion produces
and the next save supplies the stamp: a `2.0.0` and a `2.0.1` KB both convert through the one
`2.0.x → 3.0.0` conversion (below).

