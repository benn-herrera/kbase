### 3.5 Runs and run control

Over MCP, `build` and `score` start a detached child process and return once the run has started.
The run carries on without the server. At the CLI they run in the foreground, and SIGINT or SIGTERM
stops them resumably.

One repository carries one run at a time, a build or a score run, under the operation lock in the
repository's git directory. A second run is refused (`check: lock`), naming the holder's verb and
pid.

The state store defaults to `$XDG_STATE_HOME/kbase/<key>` (`~/.local/state/kbase/<key>` where the
variable is unset). `<key>` is the first 16 hex digits of the SHA-256 of the absolute,
symlink-resolved `kb-root/` path. `build` and `score` use that default unless `--state-dir` names
another. `status`, `cancel`, `wait` and `build-trail` read the store that the operation lock names
while a run holds it, and otherwise the default.

Ops:

- [`build` (tool + CLI)](build.md)
- [`score` (tool + CLI)](score.md)
- [`status` (tool + CLI)](status.md)
- [`cancel` (tool + CLI)](cancel.md)
- [`wait` (CLI only)](wait.md)
- [`build-trail` (tool + CLI)](build-trail.md)
