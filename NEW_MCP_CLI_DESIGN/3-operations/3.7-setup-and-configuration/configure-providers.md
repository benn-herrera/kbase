#### `configure-providers` (tool + CLI)

This adds or removes a provider in `providers.toml`, creating the file where it is absent.
- Keys never pass through it.
- Where the provider's key is stored, it dials the provider and reports its catalogue, which vets
  the connection.
- Where no key is stored, it writes the provider and names the next steps: `kbase auth add <provider>`,
  then `kbase models`.

- **Arguments**: `configure-providers <provider> --base-url <url>`, with `--api` and
  `--decisions-flavor` optional, writes the provider's table, creating it or setting its fields;
  fields already in place report `unchanged`. `configure-providers <provider> --remove` removes it,
  and a provider with no table in the file is refused ([§2.10](../../2-conventions/2.10-existence.md)). `--global` (default) | `--local`, mutually
  exclusive, each refused beside `--config-dir`.
- A dial that fails leaves the provider written, outcome `done`, with a diagnostic naming the
  failure and the next step. The provider is what was asked for; the dial only vets it.
- **Result**: `provider`, `layer`, `written`, `models` (where dialled), `next` (the steps that
  follow, where any do: no key stored, a failed dial or reload, or a removal that leaves a role or a
  stored key naming the provider), `warnings` (one item per configuration fault in the files the
  call writes into or reads beneath, as `models` reports them: `path` the file or directory,
  `detail` what is wrong; empty where none; on every outcome from the write on — a refusal or a
  write that fails carries none; `[]` also where the configuration does not reload after the
  write, which `next` reports). The written provider's own fault is both a `warnings` item and,
  with its remedy, a `next` step.
- **Refusals**: an invalid provider name; `--local` outside a git worktree; `--global` or `--local`
  beside `--config-dir`.
