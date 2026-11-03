#### Reader

`gfm+attributes-gfm_auto_identifiers`. The KB's maths is gfm's: pandoc 3.12.1's
`markdown` reader has `native_spans` but not `tex_math_gfm`, `gfm` the reverse. `markdown`
would not read the KB's maths, and `gfm` leaves the KB's contract forms (label spans,
reference anchors, citation spans) as raw-html open and close pairs, which the filter pairs
itself. `attributes` supplies the `{#id}` the document boundaries use; with `gfm_auto_identifiers`
off, a heading without one carries no identifier.

