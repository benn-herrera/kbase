### The loader's file set and its other refusals

**Pattern:** Schema validation at load, fail-closed.

Leaf discovery (`kb.Documents`) descends into every directory but `.index/` and skips only the
non-leaf documents a name decides (`kb.ExcludeDirs`, `kb.ExcludeNames`; SPEC
[§10](../../SPEC/10-metadata-format-version/documents-of-a-kb.md)). A conversion's file set
(`kbload`'s `coveredFiles`) is the older formats' instead: it skips the directories those formats
excluded, `session/` and `tools/` among them, through `formats.ExcludeDirs`, the frozen `format20`
list, so what an older KB converts to does not move with the reader.

A KB document that is a symbolic link is refused, not followed: `check: symlink` at the link's path,
raised by the document walk and by the conversion's own read of the KB's files alike, so no link out
of the tree is read or written through. A conversion from `1.0.0` holding bytes it cannot carry
refuses before any state is built: `check: conversion`, `path` the file, `line` and `detail` the
content. A `2.0.x` KB the `2.0.x` load refuses, and a `3.0.0` KB whose record files the `3.0.0` load
(`formats.GraphLoadError`) refuses, are refused with that load's own items, `check: metadata`
(`kbload`'s `conversionRefusal`).

The stamp is read from `kb.FormatFile` and, where that file does not exist, from the entry point's
superseded frontmatter (`kbload`'s `formatStamp`). A KB stamped in neither place is a `check:
kb-format` refusal naming both. Where the file exists its content alone is read, the frontmatter
beside it unconsulted.

