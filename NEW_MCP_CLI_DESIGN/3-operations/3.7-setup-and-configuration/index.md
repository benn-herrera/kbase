### 3.7 Setup and configuration

The configuration lives in two places:
- `~/.config/kbase`, **global** (the default);
- `<repository>/.kbase`, **local**.

Each holds `providers.toml` (providers: `baseUrl`, `apiKeyFile`, `type`, `api`, `decisionsFlavor`)
and `config.toml` (`[models]` `decision`, `text`, `scoring`; `[asks]`; `[sampling.text]`,
`[sampling.scoring]`). `--config-dir` or `$KBASE_CONFIG_DIR` names the one directory read instead.

No op, CLI or MCP, takes a key file's path. A provider's `apiKeyFile` in `providers.toml` is read,
and no op writes it.

Ops:

- [`init` (tool + CLI)](init.md)
- [`configure-providers` (tool + CLI)](configure-providers.md)
- [`configure-models` (tool + CLI)](configure-models.md)
- [`models` (tool + CLI)](models.md)
- [`show-config` (tool + CLI)](show-config.md)
- [`probe` (tool + CLI)](probe.md)
- [`auth add`, `auth list`, `auth remove` (CLI only)](auth-add-auth-list-auth-remove.md)
