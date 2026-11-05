### Scoring

**Pattern:** Job runner with an append-only event log, resume from the last status per item, optimistic-concurrency commit, projection rebuild.

`kbase score` is defined in SPEC §8 `score`; this section states the mechanism. The packages that run it:

- `internal/score`: the run's entry, scope and stale check, the runner, the item text and its sections,
  the parse, the run's ledger and captures, run retention, the writing of results, the run's status
  reader and the development replay.
- `internal/kb`: the fault vocabulary (`kb.FaultTable`), the three weight tables with their value
  computation (`Price`), and the statement reader `kb.Statement`.
- `internal/asks`: the three templates, the check that each names its class's ids, and the judged hash
  (`asks.JudgedHash`).
- `internal/query`: the scoring set.
- `internal/write`: the edit ops a score run applies, the KB write lock, and the hash every value set
  records.
- `internal/statedir`: the state directory.
- `cmd/score.go`: the verb.

Contents:

- [§6 The runner](the-runner.md)
- [§6 The ask is one seam](the-ask-is-one-seam.md)
- [§6 The prompt shelf](the-prompt-shelf.md)
- [§6 The closed text and its guard](the-closed-text-and-its-guard.md)
- [§6 The parse resolves the numbered lines](the-parse-resolves-the-numbered-lines.md)
- [§6 The scoring set](the-scoring-set.md)
- [§6 Bearing predicates](bearing-predicates.md)
- [§6 The ledger and the state directory](the-ledger-and-the-state-directory.md)
- [§6 The captures](the-captures.md)
- [§6 Run retention](run-retention.md)
- [§6 Writing the results](writing-the-results.md)
- [§6 The re-price](the-re-price.md)
- [§6 The run's entry](the-run-s-entry.md)
- [§6 The verb](the-verb.md)
- [§6 The development replay](the-development-replay.md)
