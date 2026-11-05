### The loader's file set and its other refusals

**Pattern:** Schema validation at load, fail-closed.

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

