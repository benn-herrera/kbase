### `kbase wait`

`wait` returns when the run the pid names, of either kind, is no longer progressing — the store is
at another run, or at that run with a state it stopped at — then prints the run's document: the
store's `status` document for a build, and the result document the run wrote for a scoring run, so
a command run in the background has the run's terminal state when it exits. Its arguments are
`--pid`, required and positive, and `--state-dir`, which defaults as `status`'s. A missing or
non-positive `--pid` is refused: `check: usage`, `key` `--pid`. `wait` is a command-line command
only ([§11](../11-the-mcp-server/index.md)). It takes no lock, and reads no checkpoint commits until that final document.

