### Never spell a relation

The six relation names (`RelationSupports`, `RelationFounds`,
`RelationLifts`, `RelationRefutes`, `RelationReferences` and `RelationDemoted`) and `Relations`,
the set of them, live in `internal/formats/format400`, and `ScoringRelations` in
`internal/kb/schema.go`; every other site names them through `kb` (`kb.RelationSupports`,
`kb.Relations`, `kb.ScoringRelations`), naming the constant. `cmd/relationpolicy_test.go` is an
AST scan that fails on a spelling elsewhere, with a short allowlist of files where the same word
means something else.
- **Read a record file only through the record loader, and write one only through its encoder.**
  `kb.LoadGraph` decodes both files strictly through `format400`; `kb.EncodeNodes` and
  `kb.EncodeEdges` render them whole. The trap: a second record struct or a `yaml.Unmarshal` beside
  them, which reads a missing `bearing` as the zero `kb.Bearing` — the judgement `0.0` — where the
  strict decoder refuses the record (ARCHITECTURE §11, "The zero-value hazard").
  `cmd/recordpolicy_test.go` is an AST scan outside `internal/kb` and `internal/formats` that fails
  on a struct field tagged `bearing` or `manually-scored` and on a function that both names a record
  file's path and calls the YAML package; a path handed to a YAML call in another function is
  beyond it.
- **Keep no prose copy of an op's key vocabulary.** The vocabulary is closed, total and per-op,
  stated once in `internal/write`. A second statement — in a doc, a docstring or a comment — drifts;
  point at the definition or the refusal message it produces.
- **Who supplied a value is an options field, never a flag or a values key.**
  `write.Options.Provenance` decides the `manually-scored` flag on the rigor and bearing values a
  write gives (the record types [page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md) names); its zero value,
  `ProvenanceCaller`, marks them a person's, and only kbase's own machine writers — the build's
  write runner, a scoring run's landing and re-price — pass `ProvenanceMachine`. The trap: a `--provenance` flag, a values key or an MCP property, through
  which a session could mark its own value model-scored and have `score` overwrite it.
  `TestNoSurfaceNamesTheProvenanceOrCreate` (`cmd/mcp_test.go`) fails on a flag or a tool property
  that names it.
