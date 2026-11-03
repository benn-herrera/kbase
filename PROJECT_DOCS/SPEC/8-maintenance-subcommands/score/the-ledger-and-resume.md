#### The ledger and resume

A run's records stand under `<state-dir>/score/<run>/`: `run.yaml`,
`items.jsonl` (the set the run enumerated, one pending line per item, so a reader counts the whole
run), `ledger.jsonl`, `captures.jsonl` and, while a walk runs, one result file per answered item, which
the walk's end folds into the ledger. A real run's id is `run-<id>`; the runs kept are [§6](../../6-build-state-ledger-and-monitoring/index.md)'s.
`run.yaml` holds the run, its KB root, the ledger's format, the set it walks
(one pass, or `all`), `rescore_manual` (`true` where the run re-judges manually scored values,
absent otherwise), `backend` (`provider`, which a header written under an earlier release may
read `environment`; absent under `--dry-run`, which resolves no role because it asks nothing), the
provider, the model, `ask_sha256` and the start. **It is
written once, at the run's creation, and no resume rewrites it**: a resumed or mixed run's header
states the first invocation's back end and model, and each ledger line states its own item's pass and
what answered it.

What `--resume` continues and when it is refused are [page §3.5 `score`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s. The run it continues
is the store's newest real run by its `run.yaml` start — a dry run is never one; that run
finished where no item of it is `failed` or unreached; and a build has started since it where the
store's progress record names a build invocation that started later. A resume walks the run's own
`items.jsonl`, the set it enumerated, and asks again each item that is not `done`, under the pass
and `rescore_manual` its `run.yaml` records.

`ledger.jsonl` is one JSON object per attempt — the item, its status, the value, the value the KB
carried before the attempt (`before`, absent where the record held none), the reply, `faults`
(the parsed charges: `id`, `passage`, `lines`, `citation`, `offset`, in the order written), `moot` (the charge
lines a reply recorded beside a retired `flt-pending`), `backend` (as the header's; a line written
under an earlier release may read `environment`), the model, the prompt's digest, `remedy` (on a
failed attempt whose call the provider refused for its key, the command that stores one, [§9](../../9-configuration/index.md);
absent otherwise, and absent from every line written before the field, which reads under the same
ledger format), `note` (beside a `remedy`, the note on how the key reaches the store, [§9](../../9-configuration/index.md); absent
otherwise), `judged` (the judged hash of the item's text as the call was composed, [page §2.9](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md);
absent from every line written before the field) and the attempt's bounds —
appended after the item's write and its validation, never before. A resume re-prices only a line
that records `judged`; one written before the field stands as it is. A run writes its captures and then
its ledger lines once, in the set's order, when its walk ends — on completion, on cancellation and
on a failure that stops it. `faults` is `[]` for one reply
alone, the word `none`; a failed attempt carries neither `faults` nor text. The state directory is
`build`'s:
`$XDG_STATE_HOME/kbase/<key>/`, or `--state-dir`, and one inside `kb-root/` is refused.

`captures.jsonl` is one JSON object per call made — the item, its class (`statement`, `method` or
`bearing`), the closed text whole with the line each number names, the prompt sent and its digest,
the reply the call returned (`""` where the call failed), the error that failed the attempt where
one did — the call's, the parse's or the write's — the model, the back end and the call's bounds.
Every attempt that makes a call has one, whatever its outcome: a landing, a refused reply, a failed
call or a refused write. A failed attempt's reply therefore stands whole there, where its ledger
line keeps no text of it. An item the guard refuses makes no call and has no capture, and a call
cancelled in flight leaves no capture and no ledger line. A run's captures are appended before its
ledger lines, at the walk's end, and a resume appends to the same file, and a capture that cannot be written fails the run
as a ledger line that cannot be does.

