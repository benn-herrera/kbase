## 9. Configuration

Two TOML files, `providers.toml` ([§9.1](9.1-providers-toml-the-endpoint-pool/index.md), the endpoint pool) and `config.toml` ([§9.2](9.2-config-toml-the-choices/index.md), the choices), are read under the strict-load contract ([§9.3](9.3-strict-load-contract.md)) from one or two layers. `<repository>/.kbase/` is read over `~/.config/kbase`. A directory `--config-dir` names, else `$KBASE_CONFIG_DIR`, is instead the one directory read, so an explicit choice isolates a run from a repository's own `.kbase/`. An invocation outside a worktree has no repository layer. `show-config`, `models`, `configure-providers` and `configure-models` run in any directory. `configure-providers` and `configure-models` write where [The configure commands' write location](the-configure-commands-write-location.md) states.

API keys are in neither file. The credentials store (`kbase auth add`) contains a key under a provider's name, and an entry may name a key file instead ([§9.1](9.1-providers-toml-the-endpoint-pool/index.md)).

Contents:

- [§9 Merging](merging.md)
- [§9 A leading `~` expands; nothing else does](a-leading-expands-nothing-else-does.md)
- [§9 Global flags](global-flags.md)
- [§9 Provider commands](provider-commands.md)
- [§9 A refused key](a-refused-key.md)
- [§9 Provider selection is the catalogue verbs' alone](provider-selection-is-the-catalogue-verbs-alone.md)
- [§9 The configure commands' write location](the-configure-commands-write-location.md)
- [§9 The files are the one source of inference](the-files-are-the-one-source-of-inference.md)
- [§9.1 `providers.toml` — the endpoint pool](9.1-providers-toml-the-endpoint-pool/index.md)
- [§9.2 `config.toml` — the choices](9.2-config-toml-the-choices/index.md)
- [§9.3 Strict-load contract](9.3-strict-load-contract.md)
- [§9.4 Key-file permission warning](9.4-key-file-permission-warning.md)
- [§9.5 The configure commands' targeted update (byte-preservation contract)](9.5-the-configure-commands-targeted-update-byte-preservation-contract.md)
- [§9.6 `kbase show-config`](9.6-kbase-show-config/index.md)
