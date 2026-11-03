### Scoring

`kbase score` is SPEC §8's; this states the mechanism. Six packages and the verb carry it:
`internal/score` (the run's entry, its scope and the dirty check, the runner, the closed text and
its passages, the parse, the run's ledger and captures, run retention, the landing, the
development replay), `internal/kb` (the fault vocabulary, the three weight tables and the price,
and the statement reader `kb.Statement`), `internal/asks` (the three templates, the check that they
name their class's ids, and the judged hash, `asks.JudgedHash`), `internal/query` (the set),
`internal/write` (the edit ops a landing calls, the KB write lock it takes, and the hash every
value set records), `internal/statedir` (the state directory) and `cmd/score.go`.

Contents:

- [§6 The runner](the-runner.md)
- [§6 The ask is one seam](the-ask-is-one-seam.md)
- [§6 The prompt shelf](the-prompt-shelf.md)
- [§6 The closed text and its guard](the-closed-text-and-its-guard.md)
- [§6 The parse resolves the numbered lines](the-parse-resolves-the-numbered-lines.md)
- [§6 The scoring set](the-scoring-set.md)
- [§6 The ledger and the state directory](the-ledger-and-the-state-directory.md)
- [§6 The captures](the-captures.md)
- [§6 Run retention](run-retention.md)
- [§6 The one-op landing](the-one-op-landing.md)
- [§6 The re-price](the-re-price.md)
- [§6 The run's entry](the-run-s-entry.md)
- [§6 The verb](the-verb.md)
- [§6 The development replay](the-development-replay.md)
