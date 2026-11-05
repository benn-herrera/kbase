### A leading `~` expands; nothing else does

In the `--config-dir` and `$KBASE_CONFIG_DIR` values, and in an entry's `apiKeyFile`, `~` alone is the home directory and `~` followed by a path separator is a path beneath it. `~user` is not supported, a `~` anywhere else is literal, and where the home lookup fails or its answer is not an absolute path, the string remains exactly as written.

