### V — `models`, `probe`, `show-config`, `auth` (expert)

**Design:**
- [§2.1 Naming](../../NEW_MCP_CLI_DESIGN/2-conventions/2.1-naming.md) — role, provider and model reference
- [§2.7 Provider names](../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)
- [§3.7 Setup and configuration](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md) — the configuration files, no key-file path
- [§3.7 `models`](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/models.md), [`show-config`](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/show-config.md), [`probe`](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/probe.md) and [`auth`](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/auth-add-auth-list-auth-remove.md)
- [§4.1 The instructions](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md) — item 8, `show-config`'s `upgrade` run as printed
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — MCP notes: `probe`'s note
- [§4.4 CLI-only commands](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md) — `auth`

**Scope:**
- `cmd/probe.go`, `cmd/models.go`, `cmd/show-config.go`, `cmd/auth.go`, `cmd/provider.go`,
  `cmd/surface.go`;
- `internal/config/`, including `providers.go`, `layer.go`, `hints.go`, `resolve.go`, `config.go`,
  `write.go` and `template.go`;
- `internal/auth/auth.go`;
- `internal/mcp/resources.go` (`kbase://model-hints`).

**Facts acted on:**
- Once table names read from `providers.toml` fold to lowercase, `config.Load` no longer refuses an
  override differing only by case (`rejectCaseOverrides`), and `template.go`'s providers template
  reads "replaces the global entry of the same name (any case) whole".
- [Page §2.1](../../NEW_MCP_CLI_DESIGN/2-conventions/2.1-naming.md): the vocabulary is role, provider and model reference.
- [Page §2.7](../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md): provider names fold to lowercase on every input, every output is lowercase, and two
  tables that differ only in case are a load error.
- [Page §3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/probe.md): `probe` takes `timeout`, and its MCP note's sentence is quoted in [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md). This
  package writes that note (Rule 12).
- Today:
  - `auth`'s text says the name is given "exactly as the entry spells it" (`cmd/auth.go:63`,
    `:237`);
  - `probe` is CLI-only (`cmd/surface.go:119`);
  - "category" and "`<entry>/<id>`" appear across the command files and config:
    `git grep -n 'categor' -- 'cmd/*.go' 'internal/config/*.go' 'internal/mcp/*.go' | grep -v _test | wc -l`
    → 74.
- `git grep -l 'CategoryDecision\|CategoryText\|CategoryScoring' -- '*.go'` →
  - `cmd/{configure,configure_test,show-config,show-config_test}.go`;
  - `internal/config/{config,config_test,configure,configure_test,hints,resolve,write}.go`.

  Package U removed `cmd/configure.go` and its test, so the re-run answer differs.
- `git grep -l 'probeCmd\|NameRule' -- '*.go'` →
  - `cmd/{auth,auth_test,probe,probe_test,surface}.go`;
  - `internal/auth/{auth,auth_test}.go`;
  - `internal/config/{providers,providers_test,template}.go`.
- Outside Go: `git grep -l -i 'categor' -- tools/scripts justfile test-data/fixtures` → `justfile`,
  `test-data/fixtures/eval/kb-quality-eval-brief.md`, `tools/scripts/test-integration-build-live-arxiv.sh`.
  Only the script's `:165` comment uses the configuration's vocabulary. The `justfile` and the eval
  brief name arXiv and evaluation categories, which this package does not touch.
- `show-config`'s `upgrade` is run as printed ([page §3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/show-config.md)). The rule lives once, in the instructions
  ([page §4.1](../../NEW_MCP_CLI_DESIGN/4-served-text/4.1-instructions.md) item 8, package W). This package adds no run-as-printed sentence to `show-config`'s
  help.

**Contract updates:**
- [SPEC §9](../../PROJECT_DOCS/SPEC/9-configuration/index.md) (§9.1, §9.6), [§8 `probe`](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/probe.md) and [§11](../../PROJECT_DOCS/SPEC/11-the-mcp-server.md) (the tool table), citing [page §2.1](../../NEW_MCP_CLI_DESIGN/2-conventions/2.1-naming.md), [§2.7](../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md), [§3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.
- ARCHITECTURE I9 (`probe` bound).

**Tests:**
- Mixed-case inputs fold, on each input surface.
- A case-only collision in `providers.toml` is a load error.
- `probe` is in `tools/list` with the read-only annotation.
- `TestServedTextNamesNoKeyFile` stays green.

**Done-line** (after Rule 13's two): `just test-integration`,
`just --dry-run test-integration-build-live-arxiv`.
