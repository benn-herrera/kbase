#### Citations

`add-citation` opens any document of the KB as a `leafEdit` (`citingDocument`), finds the one span of
the named line its snippet names outside code, links and markers (`citedSpan`; a missing span is
refused through `snippetAbsent`, which `checkNamedLine` shares), and links it to `targetOf`. It refuses
a link the reader would not see whole (`readsAsOneLink`) and checks a quote against `kb.Statement`,
the cited node's statement read from its host leaf by the layout around its marker, the `lifts` edges
the batch contains deciding the split layouts; `citationOf` composes the link. Where `citedSpan`
finds no span but the named line already holds exactly the link the entry composes (`lineHolds`),
the entry is an identical repeat: it writes nothing and reports that link, so the batch reads it
`unchanged`. A link spliced into a heading changes its anchor, so
`add-citation` runs `movedHeadings` and `repoint` after the splice, the new link among those updated.

