# Burndown: credentials post-sprint review

Findings from the review of `2b87b28..fcb1738`. Each item is one package, dispatched alone, in the
order of the table. Every package keeps ACTIVE_PLAN.md "Rules every package keeps" 1–7; each
dispatch quotes them. Line numbers are as of `fcb1738`; locate cited text where it has moved.

| id | rating | item | done-line | status |
|---|---|---|---|---|
| R1 | expert | hermetic recipes never reach the user's store, held by a check | `just edit-gate`, `just test-integration-init` | landed |
| R12 | mechanical (tech-writer) | SPEC: "No result names a key" reads as the stored key | — | landed |
| R7 | expert | an error body is scrubbed before it is cut | `just test` | landed |
| R6 | expert | a key is a redacting type; a scan holds where it is revealed | `just checkpoint`, `just test-integration` | landed |
| R3 | expert | a store fault has one representation | `just checkpoint`, `just test-integration-config`, `just test-integration-auth` | landed |
| R4 | expert | a refused key is classified once on the `cmd` side | `just checkpoint` | landed |
| R5 | expert | a scan holds "no production text names apiKeyFile" | `just test` | landed |
| R8 | expert | store hardening: directory `0700`, symlinked key files, relative `XDG_STATE_HOME` | `just checkpoint`, `just test-integration-auth` | in progress |
| R2 | expert | the live layered leg keeps the store | `just edit-gate` (the live run is the operator's) | pending |
| R9 | expert | `auth add` bounds its input | `just checkpoint`, `just test-integration-auth` | pending |
| R10 | mechanical | one home: `kbase auth remove`'s spelling; the providers template built without a format | `just checkpoint` | pending |
| R11 | mechanical | tests: cuts, failure messages, `auth remove`'s refusals | `just test` | pending |

Out of scope: withholding the argument from refusals; binding a stored key to an entry's
`baseUrl`; one shared package for the provider-name rule.

---

## R1 — hermetic recipes never reach the user's store

**Finding.** `tools/scripts/test-integration-init.sh` runs `build` (`:82`), which loads
configuration and so reads and decrypts the credentials store, with no `XDG_STATE_HOME`, `HOME` or
`KBASE_CONFIG_DIR` set: `grep -n -E 'XDG|HOME|KBASE_CONFIG_DIR' tools/scripts/test-integration-init.sh`
→ `80:# reaches the real $XDG_STATE_HOME.` alone. CONVENTIONS "Testing" requires a hermetic recipe
that runs a configuration-loading verb to set `XDG_STATE_HOME` under its evidence directory.

Which scripts run a configuration-loading verb (`build`, `score`, `probe`, `show-config`,
`configure`, `models`, `auth`), and how each sets `XDG_STATE_HOME`:
`grep -n XDG_STATE_HOME tools/scripts/test-integration-*.sh` → `auth`, `build-arxiv`,
`build-lamb`, `build-lamb-cycle`, `config`, `monitor-arxiv`, `results-fixture` by `export`;
`mcp-build` by `env` (`:41`); `init` not at all; `build-live-arxiv` and `build-live-fixture` not at
all, and both are LIVE. `opscript-results`, `parity-adjagent-sheet` and
`refused-write-adjagent-sheet` run no configuration-loading verb. So the check below fails
`test-integration-init.sh` alone today.

**Work.**
- `test-integration-init.sh` exports `HOME="${out}/home"`, `XDG_STATE_HOME="${out}/xdg"`,
  `KBASE_CONFIG_DIR="${out}/config"` near its top, as `test-integration-build-lamb.sh:37` does, and
  clears and creates them with its other evidence.
- A script `tools/scripts/check-hermetic.sh`, run by `edit-gate`, fails each
  `tools/scripts/test-integration-*.sh` that runs the binary with a configuration-loading verb and
  sets `XDG_STATE_HOME` by neither `export` nor `env`, unless its justfile recipe's `[doc]` begins
  `LIVE`. CONVENTIONS "Testing" names the check beside the rule.

## R12 — SPEC: the stored key (tech-writer)

**Finding.** SPEC §9's auth paragraph ends "No result names a key." beside "`key` the name as
given": a key typed as the provider name stands on the command line already and is named back.

**Work.** "No result names a stored key." Nothing else moves.

## R7 — an error body is scrubbed before it is cut

**Finding.** `internal/model/transport.go:73` reads the body through
`io.LimitReader(resp.Body, errorBodyLimit)` and `:77` scrubs after (`scrubAuthorization`,
`internal/model/openai_client.go:190`): a key straddling the limit keeps an unscrubbed prefix.

**Work.** Read up to `errorBodyLimit + len(apiKey)` bytes and scrub. If the read stopped short of
EOF, drop the scrubbed text's last `len(apiKey)-1` bytes, then cut to `errorBodyLimit`.
Tests: a key straddling the limit leaves no prefix of it; the same with a whole key earlier in the
body. Run before R6, which changes `statusError`'s `apiKey` parameter's type.

## R6 — a key is a redacting type

**Finding.** `%+v` of a struct holding an `APIKey string` prints the key, and a syntax-only scan
cannot see it; "no test or log prints a key" is held by nothing but the edit of test failure
messages in commit `fadc532`. The fields: `git grep -n -E 'APIKey +string' -- '*.go' ':!*_test.go'`
→ `internal/config/providers.go:103` (``toml:"-"``, so never decoded), `internal/config/resolve.go:117`,
`internal/model/client.go:66`. `git grep -l APIKey -- '*.go'` → 23 files: the scope.

**Work.**
- A leaf package `internal/secret` defines `Key`. Its `String`, `GoString`, `Format` and
  `MarshalText` render `<redacted>`; `Reveal() string` returns the key. No `UnmarshalText`: no key
  is decoded.
- `config.Provider.APIKey`, `config.Endpoint.APIKey`, `config.StoredKeys`' values,
  `model.Endpoint.APIKey` and `internal/auth`'s `Store.Add`/`Read` take `secret.Key`.
- `Reveal` is called only in `internal/model/openai_client.go`, `internal/model/anthropic_client.go`
  (setting the header; `scrubAuthorization`), `internal/model/transport.go` (`statusError`) and
  `internal/auth/auth.go` (sealing). A scan on the module walk (`cmd/modulescan_test.go`) fails a
  `Reveal` call outside those files, tests excluded.
- The import rules (`cmd/importpolicy_test.go` `dependencyRules`) admit `internal/secret` to
  `config`, `auth` and `model`; `internal/secret` imports no kbase package. ARCHITECTURE moves with
  them: `:1627` ("`internal/model` imports no kbase package and no `os`"), `:1636` (`internal/config`'s list),
  `:1672` (`internal/auth`'s), and the §11 package map gains the entry.

## R3 — a store fault has one representation

**Finding.** `internal/config/layer.go:197-212` (`storeFaults`) folds a `StoreFault` into a
`ProviderFault` whose `Reason` is `sf.Path + ": " + sf.Reason`, shaped for `warnFaults`
(`cmd/provider.go`, which special-cases the store-directory fault as `Name == ""`). `config.Report`
therefore takes the raw `[]StoreFault` too and joins them back by path to recover the unprefixed
reason and `DoesNotOpen`; `providerOptions.StoreFaults` exists for that join alone.

**Work.**
- `ProviderFault` gains `Source FaultSource` (`SourceEntry`, `SourceStoreFile`, `SourceStoreDir`)
  and `DoesNotOpen bool`; `Reason` is unprefixed. `StoreFault` stays the type `cmd` builds and hands
  to `config` (`cmd/provider.go`, `readStoredKeys`).
- `warnFaults` composes the path from `Source`. `Report` loses its `stored` parameter;
  `providerOptions.StoreFaults` and the join go.
- ARCHITECTURE §7's "Keys" bullet (`:1029`) follows.
- Names: `git grep -l ProviderFault -- '*.go'` → `cmd/{configure_test,models_test,provider,show-config}.go`,
  `internal/config/{inference_test,layer,providers,providers_test,report,report_test,resolve}.go`;
  `git grep -l StoreFaults -- '*.go'` → `cmd/provider.go`, `cmd/show-config.go`.
- No output changes: `test-integration-auth` and `test-integration-config`'s documents, and
  `TestUnlistableStoreIsADiagnostic`'s stderr line, stay byte-identical.

## R4 — a refused key is classified once on the `cmd` side

**Finding.** `cmd/score.go:326-327` re-derives what `modelclient.KeyRefused`
(`internal/modelclient/modelclient.go:132-137`) decides. ARCHITECTURE: "`cmd` alone imports
[`modelclient`]"; `internal/asks/call/call.go:73-74` keeps its own, the import rules barring it from
`modelclient`.

**Work.**
- `cmd/score.go` calls `modelclient.KeyRefused` and converts a `KeyRefusedError` to
  `score.RemedyError{Err, Remedy, Note: config.RefusedKeyNote}`.
- `config.WithRefusedKeyNote(detail string) string` joins a detail and the note; `call.go:85` and
  `cmd/build.go:133` use it. `internal/score/invoke.go` is unchanged (it takes the note as data).
- If `cmd/score.go` no longer names `model` (`grep -n 'model\.' cmd/score.go`), it leaves
  `cmdFileImporters` in `cmd/importpolicy_test.go` and ARCHITECTURE's list of `cmd` files that
  import `model` (`:1629-1630`).

## R5 — a scan holds "no production text names apiKeyFile"

**Finding.** `cmd/servedtext_test.go` checks an enumerated list of served texts.
`git grep -n apiKeyFile -- '*.go' ':!*_test.go' ':!internal/config/report.go'` → comments, and the
struct tag at `internal/config/providers.go:99`.

**Work.** A scan on the module walk (`cmd/modulescan_test.go`) fails on a string literal containing
`apiKeyFile` in a non-`_test.go` Go file (`kbasedev`-tagged files included) outside
`internal/config/report.go`, struct tags and comments excluded; and on `internal/mcp/instructions.txt`
and `internal/kbdocs/templates/*`. `internal/asks` prompt templates are out of scope (no key text
reaches a prompt). `servedtext_test.go` stays for texts composed at run time.

## R8 — store hardening

One package, three changes.
- `internal/auth/auth.go:116` `Store.Add` creates the directory at `0o700`, and chmods an existing
  directory to `0o700`.
- `List` (`auth.go:182`, `e.Type().IsRegular()`) stats a symlinked entry and lists it where its
  target is a regular file, so a key `Add` wrote through a symlink is read.
- `statedir.Base` (`internal/statedir/statedir.go:56`) ignores a relative `XDG_STATE_HOME`, as unset
  (the XDG base directory spec). This moves every default state directory, not only the store;
  `cmd/auth.go`'s help (`:206`) and SPEC's state-base sentence move with it.

## R2 — the live layered leg keeps the store

**Finding.** `tools/scripts/test-integration-build-live-arxiv.sh`'s layered leg sets
`HOME=${layered_home}` and copies `providers.toml` and `api_keys/`, its comments (`:159-161`,
`:175`) saying its keys travel with it. With `XDG_STATE_HOME` unset, the store moves with `HOME`,
and a provider whose key is in the store alone answers 401.

**Work.** Before `layered_env` is built (`:193`), capture
`state_home="${XDG_STATE_HOME:-${HOME}/.local/state}"` and add `"XDG_STATE_HOME=${state_home}"` to
`layered_env`. The comments say `providers.toml` and `api_keys/` travel and the store stays; the
recipe's justfile `[doc]` follows.

## R9 — `auth add` bounds its input

**Finding.** `cmd/auth.go` `authAdd` reads stdin unbounded (`io.ReadAll`) and, after
`strings.TrimSpace` (`:67`), accepts whitespace and control characters inside the key: a whole
credentials file piped in is stored as one key.

**Work.** SPEC §9 first, then code and tests. Input of more than 65536 bytes is refused (65536 is
accepted); a key that, after `strings.TrimSpace`, holds a rune for which `unicode.IsSpace` or
`unicode.IsControl` is true is refused. Each refusal is `check: usage`, writes nothing, names no
part of the input, and carries the remedy `config.AuthAddRemedy(<provider>)`. Details:
`stdin held more than 64 KiB: a key is one line` and
`the piped key holds whitespace or a control character: pipe the key alone`.

## R10 — one home

- `kbase auth remove <provider>` is spelled at `cmd/auth.go:102` (`authRemoveForm`) and
  `internal/config/report.go:134`: `config.AuthRemoveCommand(name string) string` beside
  `AuthAddRemedy`, used at both.
- `providersTemplate` (`internal/config/template.go:46`, a `const`) is a `Sprintf` format in
  `internal/config/write.go` `freshProviders`, which a `%` added to the template would break. It
  becomes a function building the text by concatenation, `AuthAddRemedy("<name>")` included; the
  comment at `template.go:42` follows. (`configTemplate` keeps its `%d`/`%s` format; it is not
  touched.) `git grep -l providersTemplate -- '*.go'` → `template.go`, `write.go`.
- `cmd/auth.go` `configuredProviderNames` appends every fault's `Name`, so the store directory's
  fault (`config.SourceStoreDir`, no name) adds `""`: it takes the names of entry and store-file
  faults only.

## R11 — tests

- Cut, naming the survivor:
  - `internal/auth` `TestStoreErrorsCarryNoKey`'s "unusable name" subcase
    (`TestUnusableNamesRefused` asserts `Error() == NameRule`);
  - `TestFileModeIsOwnerOnly`'s replaced-0644 leg (`atomicfile.TestWriteModeSetsTheModeExactly`'s
    "replaced" case);
  - `TestRenamedFileOpensUnderItsNewName` (the `sealer` doc, `auth.go:200`, states it);
  - `internal/config` report_test's "spaced" case's quoted path: use a path with a plain space
    (`TestShellWord` covers the quote);
  - `internal/claimgraph` `TestAnInferenceStopCarriesItsRemedy`
    (`internal/build` `TestARefusedKeyFailsTheBuildWithTheRemedy`).
  - The 401/403/500 matrix is tested once, on `model.StatusError.RefusedKey`; each other site keeps
    one 401 and one 500: `git grep -l -E 'StatusUnauthorized|StatusForbidden' -- '*_test.go'` →
    `cmd/refusedkey_test.go`, `internal/asks/call/call_test.go`,
    `internal/model/{anthropic_client,chat,openai_client}_test.go`.
- Rule 5, quoted: "When a test fails, its message says whether a key appeared or matched. It never
  formats a value that holds a key, or would hold one if the assertion failed. Show the length, or
  show the struct with its key field left out." Apply it to `cmd/show-config_test.go`'s failure
  calls that format the document, an entry or a diagnostic: `grep -n -E 't\.(Errorf|Fatalf).*(stdout|%v)' cmd/show-config_test.go`.
- Add, in `cmd/auth_test.go`: `auth remove`'s name-rule refusal (remedy `kbase auth list`) and its
  argument-count refusal (remedy `kbase auth remove <provider>`).
