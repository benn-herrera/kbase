#### `score` (tool + CLI)

This asks the scoring model for rigor and bearing values, one item per call. It prices the faults
each reply names and lands each value through the edit ops, with the mark `false`.

- **Scope**:
  - by default, every dirty item that is not manually scored, in both passes;
  - the rigor pass covers claims, analyses and experiments;
  - the bearing pass covers `supports`, `lifts` and `refutes` edges;
  - no pass reaches a work's rigor (judged by hand), an axiom (`1.0` by rule) or a `founds` bearing.
- **Arguments**:
  - `pass` (`rigor` | `bearing` | `all`, default `all`);
  - `node` (an id: score that node's rigor, dirty or not);
  - `edge`: score that edge's bearing, dirty or not. Over MCP a mapping `{source, target,
    relation}`; at the CLI `--edge <source>,<target>,<relation>` (ids and relations hold no
    comma);
  - `rescore-manual` (bool). In a full run it also asks every manually scored item in the pass;
    clean unmarked items are still skipped. It is required with `node` or `edge` on a manually
    scored item.
  - `dry-run` (bool);
  - `resume` (bool: continue the repository's most recent score run, with the scope it recorded,
    keeping every value it landed; it takes none of `pass`, `node`, `edge`, `rescore-manual`
    beside it);
  - `scoring-model` (a model reference for this run);
  - `ask-concurrency`;
  - `timeout` (duration, default `30s`);
  - `state-dir`.
- **Dry run**: lists every node and edge a run would ask, manually scored ones included where
  `rescore-manual` reaches them, with the total ask count. It also lists the dirty manually scored
  values, which the run skips. It writes no KB and cannot be resumed.
- **Result**: `state-dir`, `pass`, `roles`, `run`, `ledger`, `counts` (`total`, `done`, `failed`,
  `skipped`, `remaining`), `scored`, `resume` (where an item failed or was not reached).
- **MCP result**: once the ledger and pid stand, `done` with `kb-root`, `state-dir`, `pid`, `run`,
  `ledger` and `wait`. Where the run exits first, its own document. Where it is still starting
  after 5 s, `done` with `state: starting`, `state-dir`, `pid` and `wait`; `status` follows it.
- **Refusals**:
  - no scoring role in the configuration or the arguments (`key: models.scoring`);
  - `node` on a work or an axiom;
  - `resume` where the most recent score run finished, or where a newer run of any kind has
    started since it: remedy, a fresh `score`, which asks only what is dirty;
  - `resume` beside `pass`, `node`, `edge` or `rescore-manual` (usage);
  - `edge` on a `founds`, `references` or `demoted` edge;
  - `pass` with `node` or `edge`: a named item's pass follows from its kind;
  - `node` or `edge` on a manually scored item without `rescore-manual`;
  - an id or edge that does not stand;
  - a `resume` naming no standing run;
  - a run holding the operation lock;
  - a pending conversion;
  - a load error.
- There is no score batch and no item limit. The scope the caller names is the budget. `cancel`
  stops a run, `resume` continues it, and what landed stays.
