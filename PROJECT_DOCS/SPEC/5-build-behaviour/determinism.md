### Determinism

The same inputs and pandoc version give a byte-identical document graph: the tree
`document-graph` writes and the records. The claim-graph stages mint fresh node ids on every build —
unique within the KB, now and going forward — so a build is reproducible modulo
ids from `claims-declared` on, and no consumer may read anything into an id's value. The order in
which volume roots and `--bibliography` files are given does not affect the result: the same set
gives the same document graph, and the same KB modulo ids.

**`--no-inference`** is first-class. It drops the steps that spend inference, still walks and records
every stage, and states the steps it dropped. The run is a real KB built without those steps, not a
stopped walk. A trail a `--no-inference` build finished is a finished trail: `kbase build --resume`
over it is `unchanged`, and its dropped steps never run on it. A build that wants them starts over
([§5 Starting over](starting-over.md)) and is a fresh build without `--no-inference`. Which steps drop follows the classification on the step (ARCHITECTURE §4), never a stage
id. A step that spends inference in part is not dropped: its asks are skipped and the rest of the step
still runs, as for `depends-attributed`.

**`--through <stage>`** bounds the run at the named stage. A bounded run is not a failed one:
outcome `bounded`, exit 0 ([§7](../7-tool-result-contract-and-exit-codes/index.md)).

