### The files are the one source of inference

The endpoint and the model each role uses come from `providers.toml` and `config.toml` alone, and so do the settings a role's calls are made with: `[asks]` and `[sampling]` ([§9.2](9.2-config-toml-the-choices/index.md)). An entry's key is the one the store contains under its name, whatever layer supplies the entry; else the one in the file its `apiKeyFile` names, which an entry with a stored key never reads; else none. Every verb that reads the configuration reads the store.

- A stored key file that does not open is treated as no stored key (the entry's `apiKeyFile`, else none) and is reported by its path and what is wrong with it, never its content: as a diagnostic in `show-config` ([§9.6](9.6-kbase-show-config/index.md)), as an item of `warnings` and a line on stderr in `models`, `configure-models`, `configure-providers`, `build`, `score` and `probe`. A stored key file that no entry's name matches is read by nothing and is not reported.
- A store directory that exists and does not list yields no stored key for any entry, and is reported the same way, by its path and what went wrong. It does not stop the configuration loading.
- No environment variable supplies an endpoint, a model or a key. `$XDG_STATE_HOME` chooses where the store is, never a key. A run isolated from the user's layers names its directory with `--config-dir` or `$KBASE_CONFIG_DIR`; that does not isolate it from the user's store, but `$XDG_STATE_HOME` does.
- The inference preflight ([§5](../5-build-behaviour/inference-preflight.md)) names the lacking setting as the refusal's `key`: `models.decision`, `models.text` or `models.scoring`. `show-config` reports that refusal as a diagnostic rather than stopping ([§9.6](9.6-kbase-show-config/index.md)).

There is no plaintext-transport switch. When a key is configured and the base URL is `http://` to a host off the local network — any host but `localhost`, a `*.local` name, or a loopback, private or link-local IP address — `build`, `score` and `probe` write one line on stderr saying the key travels in cleartext, and proceed (as [§9.4](9.4-key-file-permission-warning.md)'s permission warning does).

