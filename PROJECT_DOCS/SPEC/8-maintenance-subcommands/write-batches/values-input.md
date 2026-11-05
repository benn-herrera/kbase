#### Values input

A batch is YAML (JSON accepted), on stdin or in the file `--values` names. At the command line, `kbase write`'s entries each name their op under `operation`; an op's own subcommand's entries name none. `kbase <op> --help` describes the op.

Each op's key set is closed and stated with the op below. A refusal names the key and its position in the document. It lists as `allowed` the fixed set the key or value must come from, where there is one; an unknown key lists the op's keys, and a required key left out is refused by name.

**Naming.**
- `id` names a node wherever the node is the subject: `edit-<kind>`, `remove-node`, `move-node`, `add-citation`'s cited node, and the queries `show`, `citations-of`, `edges` and `trace`. `score` names its item `node` or `edge`, since it takes either.
- An edge is named by `source`, `target` and `relation`, everywhere; no edge end is keyed `id`. Every edge row a query returns has the op keys `source`, `target`, `relation`, `bearing` and `context`, so a row's values name the edge for any edge op.
- A node is named by its id only; the id's prefix tells its kind. A `label` or a `title` is a field, and `find` locates a node by either ([§8](../find.md)).

**Values.**
- A `rigor` or a `bearing` is a number in `[0, 1]` or the string `"*pending*"`, quoted in YAML, where a bare `*` begins an alias. A value outside that domain, a rigor above `1.0` included, is refused at its key, the `detail` naming the domain.
- A prose value (`title`, `rationale`, `context`) containing a line break (CR or LF), U+0008 or U+000C is refused. Any other whitespace run collapses to one space, and the ends are trimmed. The `unchanged` test compares the collapsed value.

