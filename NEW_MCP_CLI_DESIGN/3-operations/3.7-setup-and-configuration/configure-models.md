#### `configure-models` (tool + CLI)

This assigns a model reference to each role in `config.toml`, creating the file where it is absent.
It checks each reference against its provider's catalogue. It rewrites the `[models]` keys and keeps
every other byte. A model missing from its catalogue is a warning carried in `warnings`, not a
failure, since catalogues rotate.

- **Arguments**: `--decision`, `--text`, `--scoring`, each taking a `<provider>/<model>`, at least
  one given. The roles given are checked and written, and the rest stand as they are.
  `--global` | `--local` as above, each refused beside `--config-dir`; `--timeout`.
- **Result**: `models` (`decision`, `text`, `scoring`), `written`, `warnings` (one item per
  configuration fault in the layers read, as `models` reports them, `path` the file or directory,
  `detail` what is wrong; then one per catalogue miss, `key` the role's argument, `detail` the
  reference; `[]` where none; on every outcome once the configuration loads — a location refusal
  or a load failure carries none).
- **Refusals**: a value that is not a model reference; a provider with no entry; the location flags
  as above.
