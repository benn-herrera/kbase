### `stats`

`stats` takes no arguments and gives a complete snapshot in one call, its keys as [§7](../7-tool-result-contract-and-exit-codes/item.md) lists them: one count per node kind; `edges` and `demoted-edges`; `subtree-aggregates`; `rigor-manual`, `rigor-model`, `bearing-manual` and `bearing-model`; `relations`; `strength-bands`, the claims counted by band, keyed by band slug, the pending claims under `unknown` ([§10](../10-metadata-format-version/the-band-ladder.md)); and the ranked sections `highest-leverage` and `weakest`.

- `subtree-aggregates` is one record per volume ([§10](../10-metadata-format-version/documents-of-a-kb.md)), keyed by the volume's directory in path order, each with `claims` and `experiments`, the counts of those whose statement marker lies in a leaf under it. A KB with no volume has `{}`.
- Each ranked section holds ten rows and previews a `find` question, which its help names: `highest-leverage` is the first ten of `find --kind claim --max 0.65 --min-dependents 1 --sort dependents --order descending`, and `weakest` of `find --kind claim --min 0 --sort strength`.
- `stats` reads derived strengths in `strength-bands`, `highest-leverage` and `weakest`, and is structural in its per-kind counts, `edges`, `demoted-edges` and the edge counts of `relations`.
- `relations` and the `edges` query read the edge records and no derived value: the bearings they report are the values the records hold, which `edit-edge` changes. `relations` reports `walked` per relation: whether a score run's bearing pass makes model calls for that relation's edges — `supports`, `lifts` and `refutes` — where a `founds` bearing is the user's, written by `add-edge` or `edit-edge`.

