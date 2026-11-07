### Never spell a relation

Single source of truth: the six relation names (`RelationSupports`, `RelationFounds`,
`RelationLifts`, `RelationRefutes`, `RelationReferences` and `RelationDemoted`) and `Relations`,
the set of them, live in `internal/formats/format40`, and `ScoringRelations` in
`internal/kb/schema.go`; every other site names them through `kb` (`kb.RelationSupports`,
`kb.Relations`, `kb.ScoringRelations`), naming the constant. `cmd/relationpolicy_test.go` is an
AST scan that fails on a literal elsewhere, with a short allowlist of files where the same word
means something else.
