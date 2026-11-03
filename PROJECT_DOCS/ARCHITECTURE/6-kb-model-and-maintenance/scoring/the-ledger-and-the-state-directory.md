#### The ledger and the state directory

`score.Ledger` opens `<state-dir>/score/<run>/`, writes
`run.yaml` through `atomicfile.Write`, appends the walk's lines in one write, with `O_APPEND` and
`Sync`, when the walk ends, and reads back the last entry per item — its status and the evidence behind it — so `--resume`
skips what landed and a failed run names each failed item's reason. A line is appended after the item's
write and validation, never before: a crash between the two re-asks a write that is idempotent, where
the reverse would skip a value that never landed. `Create` never reuses a run directory: the run id
is minted past any that stands, and a directory that appears under it fails the call. `Resume`
refuses a `run.yaml` naming another KB root, format or pass, and drops a tail torn after its last
newline so the item it names is asked again, then folds the result files a stopped invocation left —
landed through the run's landing, recorded, removed; one whose attempt the ledger already records is
removed alone — before it re-prices. `items.jsonl` holds the set the run enumerated — one
pending line per item, written when the run opens its ledger, so status and a waiter count the whole
run rather than the attempts made so far; it is all a dry run writes beside `run.yaml`.

A line carries `faults` — the parsed charges, `id`, `passage`, `lines`, `citation`, `offset`, in the
order written — `moot`, the charge lines a reply recorded beside a retired `flt-pending`, parsed the
same way and priced nowhere, and `before`, the value its item carried in the index the run read
before the attempt wrote: a before/after pair on the line itself, absent where the record held no
value. `[]` is written for one reply alone, the word `none`, so an empty list always carries the
model's positive statement that it found no fault. A failed attempt carries neither charge list, and no text:
its `reply` is the reason, and its `before` stands all the same, and its `remedy` is the one the
call's failure named — a provider refusing the call's key ([§7](../../7-inference-seams.md)) — absent otherwise, and its optional `note` is the refused-key note
that goes with it, which a failure item appends after the bounded reason so the bound never cuts it;
the fields are optional, so a ledger written before them reads under the same format, and the run's
failure items and `status`'s carry them from the line. `backend` sits on every
line, because a resumed or mixed run's lines must say what answered each item. `pass` sits on every
line beside it, naming its own item's class: the header states the set the run walks, and an `all`
run's lines say which class each judgement came from. The header also records `rescore_manual`
where the run re-judges manually scored values, which a resume continues with. `Create` also stamps
`ask_sha256`, from the header's `Pass`, as it stamps the format — one digest over every instruction
the run can send, each rendered for an empty subject and concatenated in a fixed order, so a rigor
run's two variants are covered by the one field and an update to either moves it, and an `all` run's
carries the statement, the method and the bearing renders in that order. It is stamped at
one site, so no caller can open a run without the ask its judgements answer, and a render that fails
is a defect of the build rather than a run written with no digest. `Resume` reads the header's digest
and compares nothing against it: the digest is a record of which instruction answered which judgement,
and an update applies its behaviour. A header written before the rigor pair carries one render's
digest, and is read as the one ask that run began under. `reprice` (below) is `Resume`'s other half.

