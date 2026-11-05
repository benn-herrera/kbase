### Determinism

The same inputs and pandoc version give a byte-identical document graph: the tree
`document-graph` writes and the records. The claim-graph stages mint fresh node ids on every build —
unique within the KB, now and going forward — so a build is reproducible modulo
ids from `claims-declared` on, and no consumer may read anything into an id's value. The order in
which top `.tex` files and `--bibliography` files are given does not affect the result: the same
set gives the same document graph, and the same KB modulo ids.

**`--no-inference`** is first-class. It drops the steps that make model calls, still runs and
records every stage, and states the steps it dropped. The run is a real KB built without those
steps, not a stopped run. A build that `--no-inference` finished is a finished build:
`kbase build --resume` over it is `unchanged`, and its dropped steps never run on it. A build that
wants them starts over ([§5](starting-over.md)) and is a fresh build without
`--no-inference`. A step that makes model calls in part is not dropped: its model calls are skipped
and the rest of the step still runs, as for `depends-attributed`.

**`--through <stage>`** stops the run at the named stage, given by id or display name. A bounded
run is not a failed one: outcome `bounded`, exit 0 ([§7](../7-tool-result-contract-and-exit-codes/index.md)).

