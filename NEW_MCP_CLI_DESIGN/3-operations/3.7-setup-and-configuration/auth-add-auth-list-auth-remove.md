#### `auth add`, `auth list`, `auth remove` (CLI only)

These manage stored API keys, one encrypted file per provider under `$XDG_STATE_HOME/kbase/auth/`.
The encryption is obfuscation, not a boundary. A key arrives on stdin alone, piped; it is never an
argument and never typed at a prompt.

- `auth add <provider>`: stores the piped key. Refused: a provider no `providers.toml` table names (any case); stdin
  that is a terminal; empty input; input over 64 KiB; whitespace or a control character inside the
  key. Result: `provider`, never the key.
- `auth list`: `providers`, lowercase, in name order.
- `auth remove <provider>`: `provider`. A provider with no stored key is refused ([§2.10](../../2-conventions/2.10-existence.md)).
