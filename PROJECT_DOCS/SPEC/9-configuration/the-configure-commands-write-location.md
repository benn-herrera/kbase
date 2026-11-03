### The configure commands' write location

`configure-providers` and `configure-models` write where [page §3.7](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-providers.md) states: `--global`, the default,
or `--local`, each refused beside `--config-dir`. What the page leaves open:

- With neither flag, a command writes the directory `--config-dir` or `$KBASE_CONFIG_DIR` names,
  whether or not it exists, the flag before the variable; else `~/.config/kbase`, even where a
  repository's `.kbase/` stands. An explicit `--global` or `--local` outranks the variable.
- `--global` and `--local` together are a usage refusal keyed `--local`; either beside
  `--config-dir` is a usage refusal keyed by that flag. `--local` outside a worktree is refused with
  `check: worktree`, `path` the directory the search started from, and `git init` as the remedy.
- `configure-providers`' `layer` names the layer written: `global`, `repository` (`--local`),
  `explicit` or `environment`.
- `configure-providers` creates `providers.toml` where absent holding its comment header, an example
  entry commented out, and the entry written, and no other entry. `configure-models` creates
  `config.toml` where absent from its commented template. Each otherwise edits the file under
  [§9.5](9.5-the-configure-commands-targeted-update-byte-preservation-contract.md). A refusal writes nothing, and so does a `configure-models` catalogue failure.
