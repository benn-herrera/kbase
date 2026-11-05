#### Golden tests

Each converter has a golden pair as its test. The record files of each `3.0.0` and `4.0.0` golden
tree, `internal/formats/testdata/<tree>-<version>/`, have a decoded-values golden beside them,
`<tree>-<version>.values.jsonl`: one JSON object per record in the files' order, its keys in its
type's field order, a pending value as the literal. That version's decoder, `format300`'s or
`format400`'s, reads the record files to those values (`TestRecordFilesDecodeToTheirValues`).

