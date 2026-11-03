#### `show-config` (tool + CLI)

This reports how the configuration resolves, and changes nothing: the layers read, every provider
with the layer it came from, its protocol, the kind of key it has (`store`, `file`, `none`) and any
fault, the merged choices, the selection, each role's resolution, and diagnostics.
- Keys never appear.
- For a provider whose key comes from a file, `upgrade` is the command that moves the key into the
  store, and it is run as printed ([§4.1](../../4-served-text/4.1-instructions.md)).
- It reads no KB and makes no network request.
- An unresolved role is a diagnostic, not a refusal.

- **Arguments**: `provider`.
- **Result**: `layers`, `providers` (each provider whose key comes from a file carries `upgrade`),
  `config`, `selection`, `roles`, `diagnostics`.
