### `kb-format` patch bumps

Schema versioning: a patch bump changes the format version and every fixture that pins it. Two halves, both load-bearing.

- The live format's `Version` (`format40.Version`, which `kb.FormatVersion` aliases) changes, and every `.index/format.yaml` containing the format version — or a fixture whose
  bytes pin it, or a derived line the change rewrote — is rewritten in the same commit. The loader
  parses any patch of the current minor as current, so no chain runs and no write is refused; only
  the next `update-format` rewrites the format version. So what breaks under a fixture left behind is a test case
  that pins bytes after an `update-format`, not one that only reads or writes records.
- Pitfall on the chain side: teaching `formats.Chain` to reach the patch, which is a converter
  whose output differs from its input only by the format version
  (SPEC §10 Reading the stamp).
