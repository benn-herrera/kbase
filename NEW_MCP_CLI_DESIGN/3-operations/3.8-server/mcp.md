#### `mcp` (CLI only)

This serves kbase over the Model Context Protocol: newline-delimited JSON-RPC on stdin and stdout.
stdout carries protocol frames only; diagnostics go to stderr and `--log-file`. EOF ends the server.

- **Arguments**: `--project-dir-required`. The global `--project-dir` is refused.
- The modes are in [§4.3](../../4-served-text/4.3-server-modes.md). The server sends no sampling request and offers no prompts. No inference
  runs from any tool but `build`, `score` and `probe`.
