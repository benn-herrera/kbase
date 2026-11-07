#### The run's entry

**Pattern:** Command orchestration.

`score.Invoke` runs one invocation over a KB its caller has opened, rebuilt and locked. It runs
`query.Load`, builds the run's `graph`, and determines the set: the scope (the scoring set), or a resume's
own `items.jsonl` under the pass and `rescore_manual` its `run.yaml` records. Then either the dry run's
records and the listing of what it would ask and skip (`Report.Ask`, `Report.Skip`), or the loop over the
`Launch` the verb built (`Invocation.Launch`): the ledger created or resumed, the prune, the pid,
`items.jsonl`, and the loop, whose values are written by the run's one batch when the loop ends.

It returns a `score.Report`:

- the outcome, and `Asked`: whether the invocation reached the route its calls take (not a dry run, so
  `Launch` was handed to the loop);
- the run and its ledger directory once the ledger exists with the pid and `items.jsonl` beside it, and
  on a stop only where the loop stopped it;
- the counts and the items scored;
- on `failed`, one `check: error` item per item the ledger records failed, its detail the item's name
  and its line's reason; on `cancelled`, the loop's first item with no attempt, `{}` where every one has
  one;
- `Unfinished`: whether an item failed or was not reached.

`score.RunID` creates the runs directory and mints the id (`run-<id>`, or `dry-<id>` under `--dry-run`)
or takes the one `score.ResumeTarget` found, before the locks, because the run lock's record names the
run.

