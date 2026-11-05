### Layered configuration

**Pattern:** Layered configuration.

`config.Layers` resolves the directories and `config.Load` reads and merges the two files of each,
once, before a verb's own work. It returns one `Loaded`: the pool, its faults and the choices. The
layers are `<repository>/.kbase/` over `~/.config/kbase`, or the one directory `--config-dir` or
`$KBASE_CONFIG_DIR` names (`explicit` and `environment` in the source vocabulary), so an explicit choice
isolates a run from a repository's own `.kbase/`, which every test and pinned deployment needs. The
caller resolves the repository root with `kb.RepositoryRoot(workDir)`, which answers `""` outside a
worktree, so `internal/config` takes no git dependency and reads the global layer alone there.

- **Merge.** `takeProviders` replaces a provider's table whole by a higher layer's of the same name, so a
  repository entry cannot inherit the global entry's `apiKeyFile` for an endpoint it chose;
  `takeConfig` overlays `config.toml` one field at a time (SPEC §9 Merging). `parseProviders` folds each
  table name to lowercase as it reads it, after refusing a file with two names that differ only by case
  (SPEC §9.1 `providers.toml` — the endpoint pool). `config.ParseReference` and `config.Select` fold the
  names they are given the same way.
- **Faults.** `config.Load` drops a fault naming a provider a higher layer already supplies: the pool's
  provider is the higher one, so the fault describes a value nothing reads. A higher layer's own fault
  is kept, since its provider never entered the pool. A layer that fails the strict load refuses the
  whole load and no lower layer is read (SPEC §9.3 Strict-load contract; `rejectUnknownKeys`).
- **Provenance.** `Provider.Layer` and `Config.ModelLayer` record the layer each provider and each
  role's value came from. `ProviderFault.Path` names the `providers.toml` a fault came from, and a
  `Refusal`'s path names the file the lacking setting belongs in, so a layered configuration says which
  file to edit. `config.Layer` has a directory, its source (`explicit`, `environment`, `repository`,
  `global`) and, once `config.Report` has probed it, whether the directory and each file exist
  (`ConfigExists` beside `ProvidersExists`).
- **Write target.** `config.WriteTarget` is the directory `--config-dir` or `$KBASE_CONFIG_DIR` names,
  else the global one. It is where the configure commands write with no location flag
  (`configureLocation` in `cmd/provider.go`), so a refusal of a name the pool lacks, whose remedy is the
  `configure-providers` call that adds it, names the file that call writes. `configure-models` checks
  against the stack it writes into, `config.WrittenStack`, loaded by `loadWrittenStack`, so its refusal
  names the written layer's `providers.toml` and its remedy has the location flag the call had.
  `configure-providers --remove` loads the same stack to name what still refers to the provider. Both
  commands resolve the layers (`resolveVerbLayers`) without loading them, so no layer outside the stack
  written is read (SPEC §9 The configure commands' write location).
- **Writes.** `UpdateConfig`, `SetProvider` and `RemoveProvider` edit the existing file line by line.
  `verifyEdit` decodes the proposed file and requires it to equal the decoded original plus the
  intended change, else the write is refused. `installConfig` then calls `atomicfile.Write`: a temp file,
  synced, renamed into place (SPEC §9.5 The configure commands' targeted update (byte-preservation
  contract)).
- **Report.** `config.Report` produces the facts `show-config` reports: the pool's providers and faults,
  each provider's key kind and upgrade, the diagnostics, the files' presence and `config.toml`'s values.
  It reads no file of its own; the caller passes the existence probe. `cmd/show-config.go` opens no
  client (SPEC §9.6 `kbase show-config`).

