### `find`

`find` lists the nodes that match its filters, across every kind; with no filter it lists every node. `find` locates nodes; `show` reads one.

**Arguments**, all optional; a node must match every filter given:
- `--kind`: `claim`, `analysis`, `experiment`, `work` or `axiom`; repeatable, keeping the nodes of any kind named.
- `--title`: text, matched as a case-insensitive substring of the title.
- `--label`: text, matched as a case-insensitive substring of the label, as `--title` is. Only works and axioms have a label.
- `--path`: a kb-root-relative directory, matching every node hosted under it, or a leaf, matching the nodes it hosts. `""` and `.` name `kb-root/`, matching every hosted node. A work, an axiom and a node with no host never match it.
- `--min`: strength at or above. `--max`: strength strictly below.
- `--pending`: the nodes with no strength yet.
- `--min-dependents`: an integer; the nodes with at least that many dependents ([§10](../10-metadata-format-version/four-scoring-relations-one-arithmetic.md)).
- `--sort`: `id` (the default), `strength` or `dependents`. `--order`: `ascending` (the default) or `descending`. `--limit`: allowed only beside an explicit `--sort` ([§7](../7-tool-result-contract-and-exit-codes/bounded-results.md)).

A node's strength here is its derived strength for a claim or an analysis, and its rigor for a work, an experiment or an axiom. `--min`, `--max` and `--pending` apply to every kind. `--min` and `--max` compare scored strengths only, so a pending node matches neither, and `--min 0` keeps exactly the scored nodes.

**Rows**: `id`, `kind`, `title`, `label` (null for a claim, an analysis or an experiment), `host` (the host leaf's path; null for a work, an axiom and a node with no statement marker), `strength` (`*pending*` where pending) and `dependents`. The answer is complete, sorted and limited as [§7](../7-tool-result-contract-and-exit-codes/bounded-results.md) states: `count` and `results`.

- A row's `strength` is the derived strength, so a bearing that changes a claim's strength changes the list. Its `dependents` and `show`'s `premises` are structural — counts of edges, which no bearing changes. `strength` is a number at two decimals; the filters and the sort read the full value.
- `--order` turns the named key only: ties under `--sort strength` or `--sort dependents` are broken by ascending strength, then ascending id, in either order.
- A refusal is `check: usage` at the flag: `--kind`, `--sort` and `--order` outside their vocabularies, each with it as `allowed`; `--pending` beside `--min` or `--max`; `--limit` beside no `--sort`, and `--limit 0`.

