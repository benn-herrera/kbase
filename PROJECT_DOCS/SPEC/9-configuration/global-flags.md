### Global flags

Every subcommand accepts `--config-dir DIR`, `--log-level debug|info|warn|error`
(default `warn`), `--log-file PATH` (tees diagnostics to a file; console output is never redirected
away) and `-h`/`--help`; the root command also accepts `-v`/`--version`, which prints exactly the
version string and a newline. A directory `--config-dir` or `$KBASE_CONFIG_DIR` names is the only
one read, the flag before the variable; otherwise configuration is read from the two layers above.
`--project-dir PATH` (the project root: the git worktree root that holds `kb-root/` or will; the
subcommand's search for the repository starts there instead of the working directory; a path that
is no git worktree root is refused, `check: worktree`, `key` the `project-dir` argument, spelled as
[§7](../7-tool-result-contract-and-exit-codes/item.md)'s `key` states — an MCP call's `project-dir` refused so too — `path` the absolute
path)

A relative path a subcommand takes — a volume root, `--bibliography`, `--state-dir`, `export-latex
--out`, `--values` — resolves against the project root; `--config-dir` and `--log-file` resolve
against the working directory, whatever `--project-dir` names, and a printed `resume` writes
`--config-dir` absolute.

