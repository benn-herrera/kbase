### Inputs

A relative volume root or `--bibliography` resolves against the project root. A build
orders its volume roots, and its `--bibliography` files where given, by full path as text, whatever
order they were given in; every stage reads them in that order and the trail records it ([§6](../6-build-state-ledger-and-monitoring/index.md)).
`--resume` takes no inputs: it continues with the volume roots and `--bibliography` files the
trail's newest boundary records, in the order recorded. Before it resolves a state store, `build`
refuses, `check: usage`, a `--resume` given a volume root (`key` `<volume-root>`) or a
`--bibliography` (`key` `--bibliography`), one item per argument kind, and a build given neither
volume roots nor `--resume` (`key` `<volume-root>`).

