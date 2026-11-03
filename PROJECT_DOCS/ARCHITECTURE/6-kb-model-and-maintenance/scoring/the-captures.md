#### The captures

The walk's end appends every capture to `captures.jsonl` in one write, beside the
ledger, through `appendLine`, which takes the `O_APPEND` and `Sync` the ledger's own lines take. A capture is written
for each attempt that returns a `call`: `attempt` returns none where the class cannot be derived or
the guard refuses the text, and a call the context cancelled returns nothing, so none of the three
leaves a capture. the captures are written before the lines, and a capture that cannot
be written ends the run `failed` as a ledger line that cannot be written does. A line's keys, in
order: `run`, `pass`, the item's identity as the ledger line spells it (`identity`: `kind` and `id`,
or `source`, `target` and `relation`, the other's omitted), `class` (`statement`, `method` or
`bearing`, `class.name`), `text` and `lines` (the closed text whole, each line `{no, end, from, to}` as
`closedText` holds it), `prompt` (the bytes sent), `prompt_sha256`, `reply` (what the `Ask` seam
returned, `""` where the call errored or the reply was truncated), `error` (the call's, the parse's or the write's error text,
absent on a landing), `model`, `backend`, `started` and `ended`. The ledger is unchanged: its failed
line carries the reason and no reply text, and the capture is where the reply stands. `Resume` cuts
a torn last line from `captures.jsonl`, as from `ledger.jsonl`, through the one `trimTorn`, before
anything appends, so a crash mid-capture never joins two captures into one malformed line. A capture
is self-sufficient for a replay: `readLastCaptures` (in the replay's tagged `replay.go`; the raw read
is `ledger.go`'s untagged `readComplete`) returns, per item, its last capture with the closed text
rebuilt, reads the file to its last complete line so a run still appending is readable, and refuses
a line that names no class or whose line ranges fall outside its text. **The fidelity rule:**
`class.render` is the one site that renders a closed text into a prompt, shared by `compose` and the
replay, so rendering a capture's closed text through an unchanged template reproduces its
`prompt_sha256` byte for byte; `TestCapturesReRenderToTheirDigest` asserts it over captures the runner
wrote.

The state directory is `internal/statedir.Dir`'s — the override, else `$XDG_STATE_HOME/kbase/<key>`,
and never inside `kb-root/` — extracted from `internal/build/store.go` so that a run and a build
agree on what the state directory for a KB is; the build store's own format stamp is
`internal/buildstore`'s ([§8](../../8-ledger-state-store-and-monitoring.md)). The run's locks are `build.Hold`'s ([§8](../../8-ledger-state-store-and-monitoring.md)): the operation lock in the
repository's git directory, then the state store's holder lock at `<state-dir>`, its record naming
`kind: score` and the run id.

