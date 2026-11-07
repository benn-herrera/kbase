### Inputs

A relative top `.tex` file path or `--bibliography` path resolves against the project root. A build
orders its top `.tex` files, and its `--bibliography` files where given, by full path as text,
whatever order they were given in; every stage reads them in that order and the checkpoint commits
record it ([§6](../6-build-state-ledger-and-monitoring/index.md)). `--resume` takes no inputs: it continues with the top `.tex` files and
`--bibliography` files the newest boundary records, in the order recorded. Before it resolves a
state store, `build` refuses, `check: usage`, a `--resume` given a top `.tex` file (`key`
`<volume-root>`) or a `--bibliography` (`key` `--bibliography`), one item per argument kind, and a
build given neither top `.tex` files nor `--resume` (`key` `<volume-root>`).

