### The files are the one source of inference

The endpoint and the model each role asks come from
`providers.toml` and `config.toml` alone, and so does how a role is asked — `[asks]` and `[sampling]`
([§9.2](9.2-config-toml-the-choices/index.md)). An entry's key is the one the store (`kbase auth add`, above) holds under its name, whatever
layer supplies the entry; else the one in the file its `apiKeyFile` names, which an entry with a
stored key never reads; else none. Every verb that reads the configuration reads the store. A stored
key file that does not open is treated as no stored key — the entry's `apiKeyFile`, else none — and
is reported by its path and what is wrong with it, never its content: a diagnostic in `show-config`
([§9.6](9.6-kbase-show-config/index.md)), an item of `warnings` and a line on stderr in `models`, `configure-models` and `configure-providers`, and a warning line
on stderr for every other verb. A store directory that stands and does
not list holds no stored key for any entry, and is reported the same way by its path and what went
wrong; it does not stop the configuration loading. No environment variable supplies an
endpoint, a model or a key; `$XDG_STATE_HOME` chooses where the store is, never a key. A run isolated
from the user's layers names its directory with `--config-dir` or `$KBASE_CONFIG_DIR`, which does
not isolate it from the user's store: `$XDG_STATE_HOME` does. The inference
preflight ([§5](../5-build-behaviour/index.md)) names each lacking setting as `baseUrl` or `models.<role>` — `models.decision`,
`models.text`, `models.scoring` — and `show-config` reports that refusal as a diagnostic rather than
stopping ([§9.6](9.6-kbase-show-config/index.md)).

There is no plaintext-transport switch: when a key is configured and the base URL is `http://` to a
host other than loopback, one line on stderr says the key travels in cleartext, and the build
proceeds (as [§9.4](9.4-key-file-permission-warning.md)'s permission warning does).

