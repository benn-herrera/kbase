### U — `configure-providers` and `configure-models` (expert)

**Design:**
- [§2.7 Provider names](../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md) — lowercase folding on every input
- [§2.10 Existence](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) — removing a provider with no entry is refused
- [§3.7 Setup and configuration](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md) — the two locations, the files, no key-file path
- [§3.7 `configure-providers`](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-providers.md) and [`configure-models`](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-models.md)
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings

Relies on stage 2: the four help homes (Rule 12).

**Scope:**
- `cmd/configure.go`, which is removed;
- `cmd/configure-providers.go`, `cmd/configure-models.go` (new);
- `internal/config/configure.go` (`Configure`, `ParseModelMap`), `internal/config/write.go`;
- `cmd/surface.go`;
- `tools/scripts/test-integration-config.sh`, `tools/scripts/check-hermetic.sh`;
- `tools/scripts/test-integration-build-live-arxiv.sh` (LIVE; the layered leg);
- what names or serves `configure` and goes with it: `internal/config/resolve.go` (the two
  remedies naming configure), `internal/config/template.go` (the config.toml header; the
  providers.toml template), `internal/config/layer.go` (`WriteTarget`), `cmd/build.go` (the
  `config.CheckDetect` guard, which goes with its producer), `cmd/main.go` (the `rootLong` clause
  naming configure only), `cmd/provider.go`, `internal/config/config.go` and
  `internal/config/strict.go` (comments).

**Readings settled:**
- With no location flag, a configure command writes into the directory `--config-dir` or
  `KBASE_CONFIG_DIR` names, else the global directory. An explicit `--global` or `--local`
  overrides the environment variable; beside `--config-dir` it is refused, as the page says. Every
  refusal naming "the providers.toml configure writes into" names the file `WriteTarget` gives.
- `configure-providers` creates `providers.toml` holding the entry it writes and no other: the
  template's example entry is commented out, never a live pool entry.
- Until V folds the table names read from `providers.toml`, a provider name matches an entry
  case-insensitively.

**Facts acted on:**
- [Page §3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md):
  - `configure-providers <provider> --base-url <url>`, with optional `--api` and
    `--decisions-flavor`, or `--remove`;
  - it dials where a key is stored, and otherwise names the next steps;
  - a failed dial leaves the entry written;
  - `configure-models --decision/--text/--scoring`, at least one given, keeping every other byte;
    its result `models` (`decision`, `text`, `scoring`) and `written`;
  - `--global` (the default) | `--local`, each refused beside `--config-dir`.
- Keys never pass through either command (Rule 9).
- Today:
  - `--create`, `--project` and `--model-map` (`cmd/configure.go:253–258`);
  - `git grep -l 'ParseModelMap\|config\.Configure\|configureCmd' -- '*.go'` → `cmd/configure.go`,
    `cmd/surface.go`, `internal/config/configure.go`, `internal/config/configure_test.go`.
- Who runs or names `configure` outside Go: `git grep -lw -- configure -- tools/scripts test-data/fixtures justfile` →
  - `justfile` (the `test-integration-config` doc, `:272`);
  - `test-data/fixtures/mcp/resources.jsonl`;
  - `tools/scripts/check-hermetic.sh` (the verb list, `:19`);
  - `tools/scripts/test-integration-build-live-arxiv.sh` (LIVE): `:200–213` runs
    `configure --create --project --provider … --model-map …`, and `:163` and the evidence line
    `:315` name it;
  - `tools/scripts/test-integration-config.sh`.
- The live leg after this package. Its provider stands in the global pool the leg stages, so the
  leg writes no provider entry. It runs `configure-models --local --decision <provider>/<decision>
  --text <provider>/<text> --scoring <provider>/<scoring>` in the layered repository, in place of
  `configure --create --project …`. Its checks pin `configure-models`' result: outcome `done`,
  `written` naming the repository layer's `.kbase/config.toml`, and `models` holding the three
  references. Its check that `config.toml` holds no `provider =` line stays. The comment at `:163`
  and the evidence line `:315` name the new command.
- `check-hermetic.sh` names `configure` among the verbs that read the credentials store (CONVENTIONS
  "Testing"). The two new names replace it there.

**Contract updates:**
- [SPEC §9.5](../../PROJECT_DOCS/SPEC/9-configuration/9.5-the-configure-commands-targeted-update-byte-preservation-contract.md) and [§9](../../PROJECT_DOCS/SPEC/9-configuration/index.md), wherever they name `configure`, citing [page §3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.
- ARCHITECTURE I9's clause "`models` and `configure` reaching only the provider's catalogue —
  `models` reading it, `configure` dialling it to check a model map": `configure-providers` dials
  to report the catalogue, and `configure-models` checks references against it.
- [ARCHITECTURE §11](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/index.md) `internal/config` ("configure's rules too, in `Configure`").
- [CONVENTIONS "Module Structure"](../../PROJECT_DOCS/CONVENTIONS/module-structure.md) (`kbase configure` as a remedy) and ["Testing"](../../PROJECT_DOCS/CONVENTIONS/testing/index.md) (the hermetic verb
  list).

**Tests:**
- A provider written with no key names the next steps.
- `--remove` of an absent provider is refused.
- `configure-models` keeps every other byte of `config.toml`.
- A model missing from its catalogue is a warning.
- `--local` outside a worktree is refused.

**Done-line** (after Rule 13's two): `just test-integration`,
`just --dry-run test-integration-build-live-arxiv`.
