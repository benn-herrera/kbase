### Records go through the loader and the encoder

Read a record file only through the record loader, and write one only through its encoder:
`kb.LoadGraph` and `kb.EncodeNodes` / `kb.EncodeEdges`. Pitfall: a second record struct or a
`yaml.Unmarshal` beside them
([ARCHITECTURE §11, The zero-value hazard](../../ARCHITECTURE/11-package-map/the-zero-value-hazard.md)).

`cmd/recordpolicy_test.go` is an AST scan outside `internal/kb` and `internal/formats` that fails
on a struct field tagged `bearing` or `manually-scored` and on a function that both names a record
file's path and calls the YAML package; a path handed to a YAML call in another function is
beyond it.
