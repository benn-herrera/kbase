### `kb-format` patch bumps

A patch bump moves the stamp and every fixture that pins it. Two halves, both load-bearing.

- `kb.FormatVersion` moves, and every `.index/format.yaml` carrying the stamp — or a fixture whose
  bytes pin it, or a derived line the change rewrote — is rewritten in the same commit. The loader
  reads any patch of the current minor as current, so no chain runs and no write is refused; only
  the next `update-format` rewrites the stamp. So what moves under a fixture left behind is a leg
  that pins bytes after an `update-format`, not one that only reads or writes records.
- `formats.Chain` stops at the form its last converter produces, because no converter leads to a
  patch: the chain breaks out where the major and minor already match the target. The trap on this
  side is teaching the chain to reach the patch, which is a converter whose output differs from its
  input only by the stamp.
