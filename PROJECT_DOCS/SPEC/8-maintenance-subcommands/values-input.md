### Values input

Every write is a batch, read and checked as [page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) states: YAML (JSON accepted) on stdin or in the
file `--values` names, `kbase write`'s entries each naming its op under `operation` and an op's own
subcommand's naming none. The per-op key vocabulary is closed, and stated once, in kbase's write package; it is not
copied into prose. A refusal names the key and its position, and lists as `allowed` the closed set
the key or value must come from, where there is one; `kbase <op> --help` describes the op. One edge
has one name across the ops: an op naming one edge takes it as the edge record's `source`, `target`
and `relation`, an `edges` row's keys, and an op over one existing node takes its `id`. A prose
value containing U+0008 or U+000C is refused.
