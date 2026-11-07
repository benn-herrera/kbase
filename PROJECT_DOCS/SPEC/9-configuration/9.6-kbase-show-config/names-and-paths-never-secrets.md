#### Names and paths, never secrets

A provider's `key-source` is the kind of key it supplies (`store`, `file` or `none`), and a key file is reported by the path its entry states, never by its content. A `file` key's entry has `upgrade`, the command that relocates the key into the store, with its values filled in: `cat <key-path> | kbase auth add <provider>`. The stored key then outranks the file. An agent runs the command as printed ([§11](../../11-the-mcp-server/served-text.md)). `kbase auth add` runs from a shell with the key piped on stdin; no MCP tool accepts a key ([§11](../../11-the-mcp-server/index.md)).

