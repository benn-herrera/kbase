#### Idempotence

Every op that names something existing first looks it up, and is refused where the lookup fails, with one error everywhere: `no node <id>` or `no edge <relation> <source> → <target>`. The rule covers every op naming an existing node, edge, demoted record, provider entry or stored key: the edits, `move-node`, the removals, `restore-demoted` (which looks up a demoted record), `add-citation`'s cited node, `show`, `edges`, `trace`, `citations-of`, `score`'s `node` and `edge`, `configure-providers --remove` and `auth remove` ([§8](../show.md), [§8](../edges.md), [§8](../trace.md), [§8](../citations-of.md), [§8](../score/index.md), [§9](../../9-configuration/provider-commands.md)).
- A lookup matches exactly; there is no near-miss matching.
- A removal is a lookup that can fail. Nothing records what was removed, so a repeated removal and a removal of something that never existed cannot be told apart; both are refused. A restore is refused on repeat for the same reason: it consumes the demoted record, and the edge it leaves is indistinguishable from one added directly.
- The remedy of a refusal points at the query that lists what exists: `find` for nodes, `edges` for edges.

An add looks up first too. Where the item is not found, it is created. Where it is found:
- an identical add — one whose application would change no byte of `kb-root/`: the same values, the same flag and judged hash, and a node's statement marker already where the add places it — is `unchanged`, and reports the existing item as an add would have;
- any other add is refused (`node exists: <id>` or `edge exists: <relation> <source> → <target>`). Nothing is upserted: `edit-<kind>` and `edit-edge` change an existing item's values.

An edit that changes nothing, because every value given is already the current one, is `unchanged`: the item exists, and the caller asked for a state the KB is already in. Each batch entry reports its own outcome, so an `unchanged` entry is never hidden in a `done` batch.

So for `add-<kind>` ([§8](../add-kind.md)), `edit-<kind>` ([§8](../edit-kind.md)), the edge writes ([§8](../edge-ops.md)), `restore-demoted` ([§8](../restore-demoted.md)), `remove-node` ([§8](../remove-node.md)), `move-node` ([§8](../move-node.md)) and `add-citation` ([§8](../add-citation.md)): a repeated removal or restore is refused, and a repeated add, edit, move or citation is `unchanged`, each leaving `kb-root/` byte-identical. There are two exceptions. `init` creates once: a repeat where a `kb-root/` exists is `refused`, never `unchanged`. `update-format` requires a clean `kb-root/`, so a repeat before its output is committed — by its own `commit` or by hand — is refused `dirty-paths`; over a clean tree it already converted, it reports `unchanged`.

