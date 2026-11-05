### Global flags

Every subcommand accepts:

- `--config-dir DIR`: the one configuration directory read (the introduction above). The flag outranks `$KBASE_CONFIG_DIR`, and a blank value of either counts as unset.
- `--log-level debug|info|warn|error` (default `warn`).
- `--log-file PATH`: copies diagnostics to a file; console output is never redirected away.
- `--project-dir PATH`: the project root, the git worktree root that contains `kb-root/` or will. The subcommand's search for the repository starts there instead of the working directory. A path that is not a git worktree root is refused: `check: worktree`, `key` the `project-dir` argument formatted as the `key` rule in [§7](../7-tool-result-contract-and-exit-codes/item.md) states (an MCP call's `project-dir` is refused the same way), `path` the absolute path.
- `-h`/`--help`.

The root command also accepts `-v`/`--version`, which prints exactly the version string and a newline.

A relative path a subcommand takes (a volume root, `--bibliography`, `--state-dir`, `export-latex --out`, `--values`) resolves against the project root. `--config-dir` and `--log-file` resolve against the working directory, whatever `--project-dir` names, and a printed `resume` writes `--config-dir` absolute.

