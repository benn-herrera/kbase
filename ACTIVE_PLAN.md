# ACTIVE PLAN

**Scope.** A read-only `kbase config` subcommand and its MCP tool; the build preflight's provider
item; the contract text both change; an upstream hand-off for the agent definitions that must name
them.

## Outcomes

1. `kbase config [--provider NAME]` reports configuration resolution: which directory was read and
   what chose it, every pool entry and why any faulted, both files' presence, the tier ids, the
   `KBASE_*` names set, which endpoint resolves, and the diagnostics. It writes nothing, reads no
   KB, and makes no network call. It returns `done` whenever configuration loads, whatever the
   resolution state; a file that fails §9.3's strict load refuses exactly as every other verb does.
2. The MCP server binds it as a tool — one per subcommand (I9), read-only annotation — so an agent
   can diagnose a configuration gap before starting a build, and the same verb serves a human at a
   shell.
3. A build that needs a model and cannot resolve one reports an item whose `detail` names
   `kbase config` and which carries no `remedy`.
4. The contract states all of it: SPEC §5, §7, §9, §11; ARCHITECTURE §7; README; the MCP integration
   guide. The superseded proposal at `MCP_CONFIG_GUIDANCE_RECC.md` is retired.
5. An upstream hand-off for adjagent's kb-* definitions, evidenced by a fixture transcript.

## Decisions at kickoff

- **The verb reports; it never acts.** No file written, no directory created, no model chosen.
  `configure` stays the only writer of `config.toml`, which keeps §9.5's byte-preservation contract
  as one testable path.
- **One resolution function.** `cmd/build.go`'s `buildProvider` and `runConfig` both call one
  `resolveProvider` returning the resolved inference, the selection outcome and the error;
  `internal/build`'s launch consumes the error. Configuration facts come from `internal/config`;
  §7 items are built in `cmd`, where `result` already lives.
- **A diagnostic is not a refusal.** `config`'s items sit under `diagnostics`, because §7 gives a
  `done` result no `refusals` key. Same item shape, same `check: provider`.
- **No `kbase://config` resource, no `initialize.instructions` sentence.** Discovery is
  `tools/list` plus the refusal's `detail`.
- **`remedy` only where one command clears the item** (§7's rule). The preflight item clears by no
  single command, so it carries none.

## Work packages

### WP-A — configuration facts (`internal/config`)

**Scope:** `internal/config/report.go` (new), `internal/config/report_test.go` (new),
`internal/config/paths.go`.

**Build.**

- `DirFrom(flag string) (dir, source string, err error)` owning the whole precedence: `flag`, else
  `EnvConfigDir`, else the default. `source` is `flag`, `environment` or `default`. `Dir()` keeps
  its behaviour and calls it with `""`.
- `Report(cfg Config, providers Providers, faults []ProviderFault, dir string, exists func(string) bool, getenv func(string) string) Facts` —
  pure: no file I/O, no `os.Getenv` (both seams are injected). It returns:
  - the two files' paths and `exists` for each;
  - `Entries`, a list sorted by name: one element per usable pool entry (`Name`, `BaseURL`, key
    kind `file`/`inline`/`none`, the key-file path when `file`, `Usable: true`, and the fault reason
    where a warning fault names it), then one per drop fault, which carries only `Name`,
    `Usable: false` and the reason — the pool has no entry for it;
  - `config.toml`'s `provider` and its two tier ids;
  - the four variable names §9 lists that are set, sorted — the closed list in `inference.go`,
    never a prefix scan, and never `EnvConfigDir`, which the directory source already reports.

**Facts the step rests on** — re-run each before editing:

- `internal/config/providers.go:37-62` — `Provider{Name, BaseURL, Type, API, APIKeyFile, APIKey}`.
- `internal/config/providers.go:71-82` — `ProviderFault{Name, Reason, Warning}`; a dropped entry
  exists only here, carrying no URL or key field.
- `internal/config/inference.go:14-19` — the four `KBASE_*` names, a closed set.
- `internal/config/paths.go:41-42` — `Dir()` reads `EnvConfigDir`, else the default.
- `cmd/main.go:104-109` — `resolveConfigDir`, the only place the flag precedence is expressed.

**Acceptance.** `go test ./internal/config` green. No new dependency. No `os.Getenv` or file read in
`report.go`.

**Out of scope.** Any §7 item or field shape — that is `cmd`'s.

### WP-B — the verb and its binding

**Scope:** `cmd/config.go` (new), `cmd/config_test.go` (new), `cmd/provider.go`, `cmd/build.go`,
`cmd/main.go`, `cmd/mcp_test.go`, `justfile`.

**Build.**

- `resolveProvider(p providerOptions, env config.Overrides) resolution` in `cmd/provider.go`, where
  `resolution` carries the `config.Inference`, the selection (the pool entry name and its rule, or
  none) and the error. It holds what `buildProvider` does today except the client construction and
  the cleartext line; `buildProvider` calls it, prints the cleartext line, builds the client, and
  returns `build.Provider` unchanged. `runConfig` calls it and reports.
- The selection rule is `flag`, `config` or `sole-entry` — `selectProviderName`'s three. The
  endpoint name may still be `environment`, which is `ResolveInference`'s name for a base URL
  `KBASE_API_BASE_URL` supplied with no pool entry selected; that is a fact of the resolved
  endpoint, not a fourth rule.
- `resolveConfigDir` returns `config.DirFrom(flagConfigDir)`.
- `kbase config [--provider NAME]`: verb logic in a `runConfig`-style function behind an options
  struct (CONVENTIONS "Module Structure"), `requireStreams`, `emitResult("config", …)`. No
  `--timeout`: it makes no request.
- Result keys, in §7's order after `outcome` (`kb-root` is absent — `config` resolves no KB):
  `config-dir`, `config-dir-source`, `providers`, `config`, `environment`, `selection`,
  `inference`, `diagnostics`.
  - `config-dir-source`: `flag`, `environment` or `default`.
  - `providers`: a list of mappings — `name`, `base-url`, `key` (`file` / `inline` / `none`),
    `key-file` (when `key` is `file`), `usable`, `fault` (when one exists). `base-url` and `key`
    are omitted for a drop-faulted entry.
  - `config`: `path`, `exists`, `provider`, `models` (a mapping of `decision` and `text`, each null
    when unset).
  - `environment`: the four `KBASE_*` names set in the invoking process, sorted — names only, never
    values. For an MCP call this is the server's environment, so the document describes the process
    that ran it.
  - `selection`: `provider` and `rule`, or `detail` when nothing resolves.
  - `inference`: `name`, `base-url`, `decision`, `text` — never a key — or null when resolution
    fails.
  - `diagnostics`: one item per drop fault, one per warning fault, and the resolution failure's own
    item when there is one — each `check: provider`, `path` the file it concerns, `key` and `detail`
    from the error that produced it, `[]` when none. No detail string is composed here.
- Never emit `APIKey`, a key file's contents, or an environment value.
- Bind it: `mcpBinding(configCmd, mcpBound)` and `readOnly(configCmd)` in `init`.
- `cmd/mcp_test.go:216-218` holds the read-only set as a literal, which this binding breaks. Derive
  that set from `readOnlyAnnotation` over the command tree, as the bound set is already derived
  (lines 219-228), so the test has no second home for the fact.
- Recipe `test-integration-config`: hermetic, no corpus. Stage a config directory under
  `test_data/transient/test-integration-config/` in three states — absent, `providers.toml` only,
  both files — one entry carrying a key file and one dropped for want of a `baseUrl`. Run `./bin/kbase config`
  against each, through `KBASE_CONFIG_DIR` and through `--config-dir`, and assert the outcome, the
  `config-dir-source` each invocation reports, the diagnostic count, and that neither key value
  appears on stdout. Add it to the `test-integration` omnibus.

**Facts the step rests on** — re-run each before editing:

- `cmd/build.go:89-113` — `buildProvider`: `selectProviderName`, `ResolveInference`, the cleartext
  line, the client. (`cmd/provider.go:89-113` is `dial`'s tail, a different function; do not
  extract from it.)
- `cmd/provider.go:140-165` — `selectProviderName`'s three rules and its two refusals, and
  `checkProvider`.
- `internal/config/inference.go:47-120` — `LackError{Key, Path, Detail}` and `Named()`, then
  `ResolveInference`.
- `cmd/build.go:161` — the build always passes `Provider: buildProvider(…)`, so production never
  reaches `launch`'s bare "no provider is configured" branch.
- `cmd/mcp.go:120-143` — the tool table binds only `mcpBound` commands and reads
  `readOnlyAnnotation`.
- `cmd/models.go:27-50` — the options-struct, `requireStreams` and `emitResult` shape a
  provider-facing verb takes.

**Acceptance.** `just edit-gate`; `just test`; `just test-integration-config`; `just test-integration`
green. `./bin/kbase config` against an empty `KBASE_CONFIG_DIR` prints `outcome: done` with a
non-empty `diagnostics`. One `tools/call {"name":"config","arguments":{}}` through the server returns
the same document, proving a bound command that resolves no KB works over MCP.

**Out of scope.** The preflight item (WP-C). Any resource. `initialize.instructions`.

### WP-C — the preflight item

**Scope:** `internal/build/build.go`, `internal/build/build_test.go`.

**Build.** In `launch`'s provider refusal, keep `check: provider` and the `key`/`path` a
`namedSetting` supplies; drop `Remedy: "kbase configure"`; make `detail` one sentence:

```
<row-id> (<stage>) calls a model, and <gap>; run kbase config to see the resolved configuration, or run with --no-inference
```

`build_test.go:728-782` asserts detail substrings only; extend it to assert `remedy` is absent. No
new `namedSetting` producer anywhere: the resolution errors already implement it.

**Facts the step rests on** — re-run each before editing:

- `internal/build/build.go:527-540` — the item as it stands: `Remedy: "kbase configure"`, detail
  naming the row and its stage, `key`/`path` from `errors.As(err, &lack)`.
- `internal/build/build_test.go:765-776` — a `config.LackError` already fills both `key` and
  `path`, so the "unnamed cases" the superseded proposal describes are not the production path.
- `cmd/build.go:89-113` — the closure whose error `launch` reads.

**Acceptance.** `go test ./internal/build`; `just edit-gate`.

### WP-D — the contract text

**Scope:** `SPEC.md`, `ARCHITECTURE.md`, `README.md`, `INTEGRATING_KBASE_VIA_MCP.md`,
`MCP_CONFIG_GUIDANCE_RECC.md`.

**Edits.**

- SPEC §5, "Inference preflight": the item names `kbase config` and carries no `remedy` where no
  command clears the gap.
- SPEC §7: `config` joins the `kb-root` exception list; `provider` joins the `check` vocabulary (the
  build already emits it and the list omits it — a defect this change closes); the per-subcommand
  table gains `config`'s row in the order WP-B builds.
- SPEC §9: a subsection for `kbase config` — what it reports, that it takes no action, that it
  returns names and paths but never key material or environment values, that it describes the
  process that ran it, and that a strict-load failure refuses. In "Provider commands", "both"
  becomes the three commands.
- SPEC §11: `config` joins the read-only set.
- ARCHITECTURE §7: one paragraph naming `resolveProvider` as the one resolution function behind
  `config`, `buildProvider` and the launch refusal, and `internal/config`'s facts as the report's
  source.
- README: the Configuration section and the MCP tool list gain `config`.
- `INTEGRATING_KBASE_VIA_MCP.md`: the tool bullet in "What the harness sees" and the
  provider-configuration paragraph in "Setting up" gain `config`; the fixed tool count in "Checking
  the integration" becomes no count — that check is `tools/list` naming every tool and `stats`
  returning `done`.
- `MCP_CONFIG_GUIDANCE_RECC.md` is removed: its resource, its `initialize.instructions` sentence
  and its `namedSetting` layer are superseded, and its surviving content is SPEC §9.

**Acceptance.** Every edited sentence holds against `./bin/kbase --help`, `./bin/kbase config
--help` and one run of `./bin/kbase config`. `just edit-gate`.

**Out of scope.** Any claim about `models`, `configure` or the provider pool that this work does not
change.

### WP-E — the upstream hand-off

**Scope:** the fixture under `test_data/transient/kb-agents-inspect` (untracked, its own
repository). The hand-off document is delivered upstream; this repository tracks no copy of it.

**Build.** Install with `just --justfile .claude/adjagent/justfile install-with-kb <fixture>
--harness=opencode`; replace `<fixture>/agents/kb-tools/kbase` with this tree's `bin/kbase`; in the
fixture's own repository `git init`, commit the install, then edit the kb-* definitions in place —
`kb-build` first, since the build seat is what meets the refusal, and any other definition that
names configuration — commit, and tag `staged`. The evidence is a transcript: the refusal document,
the `config` tool call, and the result document. The hand-off names the adjagent template files, the
rendered definitions, the diff and that transcript.

**Acceptance.** The transcript shows an agent reaching `kbase config`'s document from the refusal
with no file read of its own. The hand-off has had prompt-engineer review before it is considered
done.

**Out of scope.** Any edit to `.claude/adjagent`'s templates; any upstream submission.

### WP-F — the setting the item names

**Scope:** `internal/config/inference.go`, `internal/config/inference_test.go`,
`internal/build/build_test.go`, `cmd/config_test.go`.

**Build.** `ResolveInference`'s no-base-URL branch returns a `LackError` keyed by
`EnvAPI_BASE_URL`. The key becomes `baseUrl` — the pool entry's setting that supplies it — with
`path` still `providers.toml` and the detail still naming `KBASE_API_BASE_URL` as the alternative it
is. §7's `key` holds a values key, a flag or a positional argument; an environment variable is none
of the three, and the environment is a layer above the files, not the configuration. The three tests
carrying the old expectation move with it.

**Facts the step rests on** — re-run each before editing:

- `internal/config/inference.go:84-91` — the branch and its `LackError`.
- `internal/config/providers.go:44` — `BaseURL`'s `toml:"baseUrl"` tag is the one spelling of the key.
- `internal/config/inference_test.go:86`, `internal/build/build_test.go:772,776`,
  `cmd/config_test.go:206` — the expectations this changes.

**Acceptance.** `go test ./internal/config ./internal/build ./cmd`; `just edit-gate`;
`just test-integration-config`.

**Out of scope.** Naming an environment variable in the key-file branch, where the error can only
arise from a variable the user set.

### WP-G — the inline key is gone

**Scope:** `internal/config/providers.go`, `internal/config/providers_test.go`,
`internal/config/report.go`, `internal/config/report_test.go`, `cmd/config.go`,
`cmd/config_test.go`, `justfile`, `SPEC.md`, `README.md`, `USER_CONVENTIONS.md`.

**Build.** `apiKeyUnsafe` is removed as a supported field. A key reaches kbase one way: a file whose
path the entry's `apiKeyFile` or the environment's `KBASE_API_KEY_FILE` names. The environment is a
layer above the files and never carries a secret; a config file that carried one is the same
mistake in a slower form.

- `Provider.APIKeyUnsafe` and its tag go. Strict load (§9.3) then refuses a `providers.toml` that
  still spells it, naming the key — the removal needs no separate refusal of its own.
- Key resolution returns the file's content only; a fault reason still carries no key material.
- `KeyKindInline` goes with it: `KeyKind` is `file` or `none`.
- The tests that built an inline key into a fixture keep their subject and change their input: the
  sentinel-in-a-fault-reason test uses a key file containing the sentinel, and the resolution
  precedence test loses the inline arm rather than the precedence.
- SPEC §9.1's block and its "preferred over" comment lose the field; §7's `config` row and §9.6
  reduce the key kinds to `file` and `none`; README's pool sentence does the same;
  `USER_CONVENTIONS.md`'s advice drops the comparison and keeps the rule.
- `test-integration-config` stages no inline key. Its absence assertion keeps the file key, which is
  the only kind left to leak.

**Facts the step rests on** — re-run each before editing:

- `internal/config/providers.go:53-56, 65, 137, 173, 215-220` — the field, its comment, the
  resolution order and the fault paths that mention it.
- `internal/config/report.go:14-15, 98-99` — `KeyKindInline` and its arm.
- `cmd/config_test.go:30, 38, 138` and `internal/config/report_test.go:72, 102` — fixtures built on
  the inline key.
- `internal/config/providers_test.go:48, 113, 212, 219, 338, 411` — six fixtures, of which 411 is
  the sentinel test that must keep its subject.
- `justfile:2778-2780` — the recipe's inline entries.

**Acceptance.** `just test`; `just edit-gate`; `just test-integration-config`. A `providers.toml`
spelling `apiKeyUnsafe` fails strict load naming that key, and the recipe's pool state shows a
`key: file` entry and a dropped one.

### WP-H — per-field provenance for the resolved inference

**Scope:** `internal/config/inference.go`, `internal/config/inference_test.go`,
`internal/config/report.go`, `internal/config/report_test.go`, `cmd/config.go`,
`cmd/config_test.go`, `SPEC.md` §7's `config` row and §9.6, `justfile`.

**Build.** `config` says where each resolved inference field came from. The layers are the
environment, `providers.toml` and `config.toml`; today only the endpoint's is visible and only
indirectly (`inference.name` becomes `environment` when the endpoint came from a variable), while a
`KBASE_DECISION` or `KBASE_TEXT` in the caller's shell is indistinguishable from a `config.toml`
tier holding the same value.

- `Inference` carries the source of each field it resolves: `base-url`, `decision`, `text` and the
  key. The values are the layer names the report already uses — `environment`, `providers.toml`,
  `config.toml` — so they sit beside `config-dir-source` and `selection.rule` rather than inventing a
  vocabulary. A field the environment supplied names `environment`, not the variable: the mapping
  from field to variable is one-to-one (§9, "The environment") and the `environment` section above
  it lists which names are set, so the document's own lines join.
- `ResolveInference` records the source where it picks the value; nothing re-derives it afterwards.
  `pick` walks layers in precedence order, so the source is the layer that answered.
- Additive in the document: `inference` keeps its four scalars and gains `sources`, a record with
  those keys, each null where its field is unset. The document is the rendering — `config` writes one
  YAML document on stdout (§7), so the source sits in the record beside the field it explains, not in
  a second human form.
- SPEC §7's `config` row gains `sources`; §9.6's prose gains the sentence that states it.
- The recipe gains a leg in which one inference variable is set over the `both` pool and asserts that
  the document names `environment` for that field and `config.toml` for the other, so the assertion
  can fail. Check first whether any existing leg sets an inference variable at all — the current
  `environment`/`flag` axis is the configuration *directory's* source, not this one.

**Facts the step rests on** — re-run each before editing:

- `internal/config/inference.go:60-99` — `pick`, the precedence walk, and where each field is set.
- `internal/config/report.go` — the record `Config` builds, and how `Inference` reaches it.
- `cmd/config.go` — the `inference` rendering, JSON and human.
- `justfile`'s `test-integration-config` — the `run` function and its arguments, and which legs set
  inference variables (`grep -n 'KBASE_DECISION\|KBASE_TEXT' justfile`).
- `SPEC.md:447` (the `config` row) and `SPEC.md:813-837` (§9.6).

**Acceptance.** `just test`; `just edit-gate`; `just test-integration-config`. One document shows a
field resolved from each of the three layers, and the recipe's own assertion fails if the source is
dropped or flattened to a constant.

## Sequencing and status

| WP | Agent | Depends on | Commit | State |
|---|---|---|---|---|
| A | `go-coder` | — | one | done |
| B | `go-coder` | A | one | done |
| C | `go-coder` | B | one | done |
| D | `tech-writer` | A, B, C | one | done |
| E | coordinator + `prompt-engineer` | B, D | one | done |
| F | `go-coder` | B | one | done |
| G | `go-coder` | B | one | done |
| H | `go-coder` | F | one | done |

One commit per work package, made by the coordinator after auditing the diff against CONVENTIONS
"Coordinator Policy" — never by the dispatched agent. The branch is `mcp-config-help`; nothing is
merged to `main`.
