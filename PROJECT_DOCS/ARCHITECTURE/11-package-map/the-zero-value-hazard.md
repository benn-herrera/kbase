### The zero-value hazard

A record file decoded outside `kb.LoadGraph` — by a test, by a tool, or by
a command that read bytes of its own into a struct — reads a missing `bearing` as the zero
`kb.Bearing`, which is the value `0.0` and a judgement: a premise contributing zero to the
derivation. The strict decoder refuses a missing field for that reason, and the loader monopoly
therefore carries a type's meaning and not only a version's: every decode of a record file goes
through `kb.LoadGraph`, over `Source` bytes.

