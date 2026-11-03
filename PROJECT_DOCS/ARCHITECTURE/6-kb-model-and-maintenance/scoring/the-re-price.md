#### The re-price

`Resume` takes the same `Write` seam a landing is opened with, and `reprice`
re-derives every landed line's value from the ids its line records, under the table the line's own
class holds — a run landed under one rigor table re-prices under the class table, which is intended.
Only a line that records `judged`, the hash its call was composed under, is re-priced: one written
before the key says nothing of what text its value judged, and stands as it is.
A value the running table gives differently is landed first through the landing's write batches,
under the KB lock as a landing holds it, and the line records it after, so no line claims a value
the KB did not take. A value the landing discards — its item's text was edited after it was asked
(`errTextChanged`), or its node or edge is no longer in the KB (`errItemGone`) — rewrites its line with
that reason through `attempt.fail`, as a walk's discarded landing does: `failed` for a changed item,
which the walk that follows asks again, and `skipped` for a removed one, which nothing asks again. Any other refused
write fails the resume with the ledger as it stood, and the next resume re-derives the same move,
the write being idempotent. Each moved line is
listed in `repriced.yaml` in the run's directory, written only where a value landed — the run, the time, and per line the item, the
value it held and the value it took — which is where a re-price is recorded, `run.yaml` being written
once. `Ledger.Repriced()` says how many values moved, so the verb's closing refresh can follow a
re-price as it follows a landing. No model is asked: the ids are the whole input of the price, and
the model never saw a weight.

