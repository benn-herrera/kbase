#### The captures

**Pattern:** Request/response capture for replay.

The loop's end appends every capture to `captures.jsonl` in one write, beside the ledger, through
`appendLines`, with the `O_APPEND` and `Sync` the ledger's own lines take. A capture is written for each
`call` an attempt returns, so a re-asked item has two, in call order. `attempt` returns none where the class cannot be derived or the input
guard refuses the text, and a call the context cancelled returns nothing, so none of the three leaves a
capture. Captures are written before the lines, and a capture that cannot be written ends the run
`failed`, as a ledger line that cannot be written does.

A capture's keys, in order: `run`, `pass`, the item's identity as the ledger line formats it (`identity`:
`kind` and `id`, or `source`, `target` and `relation`), `class` (`statement`, `method` or `bearing`),
`text` and `lines` (the item text whole, each line `{no, end, from, to}` as `closedText` holds it),
`correction` (on a re-ask alone: `case`, `values` and `quoted`, the inputs its correction was rendered
from — the case, its slot values by bare slot name, `{}` where it has none, and the quoted lines, `[]`
where it quotes none), `prompt` (the bytes sent), `prompt_sha256`, `reply` (what the `Ask` returned; `""` where the call
errored or the reply was truncated), `error` (the call's, the parse's or the write's error text, absent
on a successful write), `model`, `backend`, `started` and `ended`. The ledger's failed line carries the
reason and no reply text; the capture is where the reply is kept (SPEC §8 The ledger and resume).

`Resume` removes a torn last line from `captures.jsonl`, as from `ledger.jsonl`, through `trimTorn`, the
single torn-tail trim, before anything appends, so a crash mid-capture never joins two captures into one malformed line.

A capture is self-sufficient for a replay. `readLastCaptures` (in the replay's tagged `replay.go`; the
raw read is `readComplete` in the untagged `ledger.go`) returns, per item, its last capture with the item
text rebuilt. It reads the file to its last complete line, so a run still appending is readable, and
refuses a line that names no class or whose line ranges fall outside its text. The capture it keeps per
item is the last with no `correction`, a first ask, while every line is still read and checked.
**The fidelity rule:** `class.render` is the single site that renders an item text into a prompt, shared
by `compose`, the re-ask and the replay, so rendering a capture's item text, with its `correction` where
it has one, through an unchanged template reproduces its `prompt_sha256` byte for byte.
`TestCapturesReRenderToTheirDigest` asserts it over captures the runner wrote, a re-ask's included.

