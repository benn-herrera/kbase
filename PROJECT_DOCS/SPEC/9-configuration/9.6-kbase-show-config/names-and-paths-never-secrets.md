#### Names and paths, never secrets

A key is reported by the kind it is — `store`, `file` or
`none` — and a key file by the path its entry states, never by its content. A `file` key's entry
carries its `upgrade`, the command that moves the key into the store with its values filled in,
`cat <key-path> | kbase auth add <provider>`; the stored key then outranks the file.
`kbase auth add` runs from a shell with the key piped on stdin; no MCP tool accepts a key ([§11](../../11-the-mcp-server.md)).

