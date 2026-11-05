### Superseded format parsers

Numbered schema migrations: a superseded format's parser lives in `internal/formats` and nowhere else (ARCHITECTURE I8;
`cmd/importpolicy_test.go` enforces the import side). Pitfall: keeping a reader for the old form
"for compatibility" in `kb` or `index`, which is the multi-version reader the design forbids. A
converter's test is a golden pair, old bytes in and new bytes out. A format package may change
until a kbase release ships its version, and is frozen after. Frozen is what it accepts and what a
conversion produces; its refusal and fault text may change.
