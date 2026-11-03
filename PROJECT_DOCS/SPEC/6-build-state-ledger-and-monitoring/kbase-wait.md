### `kbase wait`

`wait`'s arguments and refusal are [page §3.5 `wait`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/wait.md)'s: it returns when the run the pid names,
of either kind, is no longer progressing — the store stands at another run, or at that run with a
state it stopped at — then prints that store's `status` document, so a command run in the
background carries the run's terminal state when it exits. `--state-dir` defaults as `status`'s.
It takes no lock, and reads no commit trail until that final document.
