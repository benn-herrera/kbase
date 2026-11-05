### The state store is versioned

`<state-dir>/format` contains one integer, currently `1`, written
when a build first takes the run lock; a store with no `format` file is version 1. `build`,
`status`, `wait`, `cancel`, the MCP `build` tool and `build-trail` refuse a store newer than this
kbase: `check: state-dir`, `key` the `state-dir` argument, `path` the store, remedy "update kbase,
or use another `state-dir`", the argument in both written as [§7](../7-tool-result-contract-and-exit-codes/item.md)'s `key` states. A build
converts an older store in place before it uses it; the conversion is stepwise and resumes if
interrupted. The store's version is independent of the KB format version ([§10](../10-metadata-format-version/index.md)).

**The state store is pruned only by a build or a scoring run.** At a build's start, the captured
output of builds the MCP server started (`reports/build-*`, [§11](../11-the-mcp-server/index.md)) is pruned to the newest six by the
start each one's name records. At a scoring run's start, once its own run directory exists,
the run directories under `score/` are pruned by kind: a real run's id is `run-<id>` and a dry run's
`dry-<id>` ([§8](../8-maintenance-subcommands/index.md)), a directory's kind is its name's prefix, and the invocation keeps, of its own kind,
16 runs — the run it names, created or resumed, and the newest 15 others by the start their
`run.yaml` records, equal starts ordered by name with the later name newer — and removes the rest of
that kind whole, captures included. The captured output of scoring runs the MCP server started
(`reports/score-*`, [§11](../11-the-mcp-server/index.md)) is pruned at the same point to the newest 16 by the start each one's name
records. A dry run never removes a real run, nor a real run a dry one,
and a dry run an earlier kbase minted as `run-…` counts as real. A directory whose `run.yaml` is
absent or unreadable has no start to order by and is left. An invocation refused before its run
directory exists — a `--resume` finding no run to continue among them — removes nothing. A
resumed run's `captures.jsonl` loses a torn last line before anything appends to it, as its ledger
does. A build's answer cache, multichoice call records and call captures are kept in one
directory per build, named by the build's `start` boundary commit. At a build's start, before its
first stage, the current build's directory and the newest five others are kept, and the rest are
removed whole, along with the undivided cache, records and call captures an earlier kbase kept. At
the same point, a fresh build whose progress record exceeds 500 lines moves the earlier runs' lines
to a file under a timestamped name, keeping its own in the record; the 15 newest such files are
kept. The stage reports are kept. `status` and `cancel`
remove nothing.
Deleting the store is always safe, but it forfeits the answer cache, whose entries are cached model
answers; `status` reports its size.

