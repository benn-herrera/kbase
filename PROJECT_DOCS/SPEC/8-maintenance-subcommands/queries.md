### Queries

Every query answers from the KB's records and statement markers, writes nothing and takes no lock. A KB in an older format answers as converted, with `migration` in its result ([§10](../10-metadata-format-version/one-version-read-and-written.md)); a KB whose load fails is refused ([§10](../10-metadata-format-version/the-load-every-fault-refused.md)). A query's data sits under `results`: a list with `count` for `citations-of`, `find`, `edges` and `trace`, a mapping for `show` and `stats` ([§7](../7-tool-result-contract-and-exit-codes/item.md)). A list query returns its complete answer ([§7](../7-tool-result-contract-and-exit-codes/bounded-results.md)). `kbase <query> --help` states its arguments.

