#### The ledger and the state directory

**Pattern:** Append-only job log with resume.

`score.Ledger` opens `<state-dir>/score/<run>/`, writes `run.yaml` through `atomicfile.Write`, and
appends the loop's lines in one write, with `O_APPEND` and `Sync`, when the loop ends. Before then, an
answered item writes its own answer file, `results/<item index>.json` in the run's directory, the
index being the item's place in the set (`writeAnswerFile`); the loop's end folds these into the ledger
and removes them. An answer file holds the final call's `line` and `capture`, and, for an item re-asked
after a refused reply, the refused call's own `line` and `capture` under `prior`. The loop's end and
`fold` append the prior call's capture and line ahead of the final call's, so a re-asked item has two
lines in call order, the first `failed` with its own bounds and prompt digest; only the final line's
value lands. A file without `prior` holds one call, and `fold`'s check that the ledger already records
an attempt reads the final line alone. It reads back
the last entry per item, with its status and the evidence behind it, so `--resume` skips what was written
and a failed run names each failed item's reason. A line is appended after the item's write and
validation, never before: after a crash between the two the item gets a new model call and an
idempotent write, where the reverse order would skip a value that was never written. `Create` never reuses a run directory: the run id is minted
past any that exists, and a directory that appears under it fails the call. The run's directory also
holds `pid`, the process id of the invocation walking the run (`WritePID`), by which `wait --pid` finds
the run; a resumed invocation overwrites it with its own.

`Resume` refuses a `run.yaml` naming another KB root or ledger format. It takes the run's pass from the
header. It drops a tail torn after its last newline, so the item the line names gets a new model call, then
merges the answer files a stopped invocation left (`fold`): each value is written through the run's
`Write` interface, recorded, and the file removed; one whose attempt the ledger already records is
removed alone. Then it recomputes values ([the re-price](the-re-price.md)).

`items.jsonl` holds the set the run enumerated, one pending line per item, written when the run opens
its ledger, so status and a waiter count the whole run rather than the attempts made so far; it is all a
dry run writes beside `run.yaml`. The files, the line's keys and the optional `remedy`, `note` and
`judged` under one ledger format are SPEC §8 The ledger and resume. Two mechanisms behind them:

- A line contains `faults` (the parsed charges, in the order written), `moot` and `before`, the value
  its item had in the in-memory index the run read before the attempt wrote. `faults` is `[]` for the
  reply `none` alone, so an empty list always holds the model's positive statement that it found no
  fault. A failed attempt contains neither charge list and no reply text: its `reply` is the reason, and
  its line still carries `before`. `remedy` is the one the call's failure named, and `note` goes with it; a failure item appends the note
  after the bounded reason, so the bound never cuts it. Reply, remedy and note pass through
  `argmark.Spelling` ([§9](../../9-result-emitter/per-interface-rendering.md)), so no line carries a marker.
- `backend` and `pass` sit on every line, because a resumed or mixed run's lines must say what answered
  each item and which class each judgement came from. `Create` records `ask_sha256` from the header's
  `Pass` (`askDigest`), as it records the format: one hash over every instruction the run can send, each
  rendered for an empty subject, concatenated in the order statement, method, bearing. Each class's
  part is `asks.ScoreInstructionSet` ([the prompt shelf](the-prompt-shelf.md)): the first ask, then each
  refusal case unquoted and quoted, every `dyn.` value `""`, so the correction's fixed text is certified
  too. It is recorded at
  one site, so no caller can open a run without the prompts its judgements answer, and a render that fails
  is a defect of the build, not a run written with no hash. `Resume` reads the header's hash and compares
  nothing against it (SPEC §8 `ask_sha256` records the ask set and gates nothing).

The state directory is `internal/statedir.Dir`'s: the override, else `$XDG_STATE_HOME/kbase/<key>`, and
never inside `kb-root/`. It is shared with the build so a run and a build agree on the state directory
for a KB; the build store's own format record is `internal/buildstore`'s ([§8](../../8-ledger-state-store-and-monitoring/index.md)).
The run takes the run lock through `build.Hold` ([§8](../../8-ledger-state-store-and-monitoring/locks.md)); the lock's record names `kind: score` and
the run id.

