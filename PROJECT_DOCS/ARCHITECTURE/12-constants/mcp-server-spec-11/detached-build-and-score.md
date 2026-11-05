#### Detached build and score

**Pattern:** Detached child process plus status polling.

`internal/mcp/launch.go` is the single self-exec owner under I1, file-level, enforced by the exec
policy. The `build` and `score` tools re-execute the kbase binary as `kbase build …` and `kbase
score …` by the same detachment:

- The child runs from the root of the call's project, with the server's environment plus
  `KBASE_RESULT_SURFACE` (`mcp.SurfaceEnv`), which tells the run its document is read over MCP. It
  has its own process group (detach flags per platform in `internal/mcp/detach_unix.go` and
  `detach_windows.go`) and no stdin.
- The server's `--config-dir`, `--log-level` and `--log-file` are passed through, and `--project-dir`
  where the call named one, so the child's recorded `resume` carries it.
- Every path the child is given is absolute, because its working directory is not the server's. A
  relative `state-dir` argument is resolved against the repository root; the server's `--config-dir`
  and `--log-file` are resolved against the server's working directory.
- Its stdout and stderr are files in the state store's `reports/`, `build-<UTC timestamp>-<random>`
  or `score-<UTC timestamp>-<random>` with `.stdout.yaml` and `.stderr.log`. Pruning keeps the
  newest `keptBuilds`+1 build captures (`build`'s prune) and the newest `KeptRuns` score captures
  (`score.PruneRuns`) ([§12](../index.md)).
- Before starting, the tool runs `status`'s preconditions: no worktree, or a `--state-dir` inside
  `kb-root/`, returns the refusal `status` gives and starts no process.
- The `edge` flag of `score` is declared a mapping (`mcp.MappingFlag`), so its schema property is an
  object of the required strings `source`, `target` and `relation`, closed to other keys, and
  `CommandLine` joins the mapping into the child's `--edge <source>,<target>,<relation>`, as
  `mcp.Spelling` does in a call it renders. A value holding a comma is refused at `edge`
  (`commaInMapping`) and starts nothing.

The tool polls the state store every `launchPoll` and returns when the run has started or the child
has exited, bounded by `launchStarting` ([§12](../index.md)). The child takes the run lock and the
holder lock and emits its first progress event before it waits out a write ([§8](../../8-ledger-state-store-and-monitoring/locks.md)). What
"started" means differs by tool:

| Tool | Started when | `done` document |
|---|---|---|
| `build` | the progress record shows a stage entered (`buildstore.Launched`) | `kb-root`, `state-dir`, `pid`, `resume` and `wait`. `resume` appears only where the record's command parses as a `kbase build` invocation |
| `score` | the run's ledger and pid exist (`score.RunByPID`: the run's directory written, its `pid` file reading back) | `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait` |

`wait` is the run's background command: `buildstore.ShellJoin` of the executable with `wait
[--project-dir <root>] --state-dir <store> --pid <pid>`, `--project-dir` where the call named one.
Otherwise the result is the child's own document where it exited first (`isError` from its outcome),
or `done` with `state: starting`, `state-dir`, `pid` and `wait` where neither happened within the
bound; `status` follows the run.

A score run's result document is written beside its ledger before the run lets go of the state
store's holder lock, so `wait` reads the same document the run reported rather than racing the
lock's release. `wait` locates the run by the pid the launch named (`score.RunByPID` reads the run's
`pid` file) and reports the run's outcome; a run whose process ended without writing a result
reports the failure its ledger supports. `status` and `cancel` read the holder record (the pid, the
run, the start stamp and the kind `score`), so a run in flight is visible under the pid the tool
returned.

Nothing announces a run's end: no resource is subscribable and the server writes no frame a request
does not owe, so a session follows a launched run by calling `status`, or by running the `wait`
command the launch returned. `test-integration-mcp-build` (`TestMCPBuildFollowedByStatus`) drives a
real server to a build's end by polling `status`.

---

