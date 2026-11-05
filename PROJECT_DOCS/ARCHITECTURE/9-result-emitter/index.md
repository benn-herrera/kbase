## 9. Result emitter

**Pattern:** Single structured-output writer.

`internal/result` owns the YAML result document and its emitter ([§2](../2-invariants.md), I5). `result.Write`
builds the document as a `yaml.Node` mapping from the ordered `Field` list it is given: the key order
is the caller's, every string is double-quoted, and a float is written without an exponent. A refusal is
a list of `Item`s and the emitter writes every item it contains; collecting every offender is each
producer's work, and nothing in `internal/result` checks it. `Emit` puts `outcome` first. The result
tests parse each covered verb's stdout as one document and compare its keys with the documented order
(`documentedKeys`, `cmd/resultkeys_test.go`). Production code names `os.Stdout` once, in `main`, and no
test scans for another stdout writer. SPEC §7 Tool results is the contract.

`result.ExitCode` maps `outcome` to the exit code and `result.IsError` says whether the outcome is an
error: `cmd` exits with the first, and an MCP tool result's `isError` is the second. `Emit` ends the
`detail` of each item that ends a `retry` or `failed` document with the outcome's closing sentence. `RetryAsk` and
`FailedAsk` are the single source of both sentences, and a caller passes `Emit` its items without
them. The YAML library is the one named in CONVENTIONS "Dependency Policy".

Contents:

- [§9 Per-interface rendering](per-interface-rendering.md)
