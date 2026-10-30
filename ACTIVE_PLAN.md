# Plan: the credentials store

Built from [ACTIVE_DESIGN](ACTIVE_DESIGN.md)

**Status: dispatching,** one package at a time; the table below tracks each. Line numbers below are
as of commit `be1adb5`.

**The store location** is `$XDG_STATE_HOME/kbase/auth/`, defaulting to
`~/.local/state/kbase/auth/` where the variable is unset, as kbase's state base already resolves
(`internal/statedir/statedir.go:35–41`; "The store's location" below). Tests isolate the store
through `XDG_STATE_HOME`, and SPEC §9's "No environment variable supplies an endpoint, a model or a
key" narrows: the variable chooses where the store is, never a key.

## Outcome

- `kbase auth add <provider>` reads a key piped on stdin and stores it encrypted, one file per
  provider. `kbase auth remove <provider>` clears it, and `kbase auth list` names the providers
  that have one. All three are CLI-only.
- A pool entry's key is the store's, looked up by the entry's name, else its `apiKeyFile`, else
  none.
- A provider that answers 401 or 403 fails the call as it does today, and the failure's item
  carries the remedy `kbase auth add <provider>`.
- `show-config` reports where each key came from and the upgrade from a key file. No other served
  text names `apiKeyFile`.

The design's "Decisions taken" are the requirement. Its "Acceptance" list is this plan's exit
condition, met by packages A–F together.

## Principles that steer open choices

These are quoted from the contract. Each one binds a choice a package makes.

- **Configuration stays pure.** ARCHITECTURE §11: "`internal/config` imports no kbase package but
  `atomicfile` and `result`". CONVENTIONS "internal/config": "Resolution is pure and lives here
  alone … reads no file". Consequence: `config` cannot import the store. `cmd` reads the store and
  hands its keys to `config` as data. The precedence rule (store, else file, else none) is a pure
  function inside `config`, its one home.
- **One writer per mechanism.** ARCHITECTURE §11 calls `internal/atomicfile` "the one atomic file
  writer". The store writes through it, and no second implementation is written.
- **One emitter owns stdout** (ARCHITECTURE I5). A verb's acknowledgement is its result document.
- **The served surface is the operating contract** (ARCHITECTURE §3). An agent learns about
  `kbase auth add` from kbase itself: the failure's remedy, `show-config`'s report, or a command's
  help. The MCP instructions cannot carry it. `TestMCPInitializeNamesWhatItServes`
  (`cmd/mcp_test.go:1609`) refuses any command name in the instructions that the tool table does
  not serve, and `auth` is not served.
- **Tests never touch a real home** (CONVENTIONS "internal/config": "tests never touch a real
  home"; CONVENTIONS "Testing": "nothing reaches the user's real `$XDG_STATE_HOME`"). The store
  lives under `$XDG_STATE_HOME`, so every test and hermetic recipe that runs a verb which reads the
  store sets `XDG_STATE_HOME` to scratch. An isolating `--config-dir` does not isolate the store.

## The store's location

The store is at `$XDG_STATE_HOME/kbase/auth/<provider>-key.enc`, with `$XDG_STATE_HOME`
defaulting to `~/.local/state`. That is the design's `~/.local/state/kbase/auth/<provider>-key.enc`
whenever the variable is unset. kbase already resolves its state base this way
(`internal/statedir/statedir.go:21-43`, SPEC §6 line 385), so both share one function. The
consequence is that tests isolate the store through `XDG_STATE_HOME`, as they already isolate the
state store. The `auth/` directory sits beside the per-KB state stores, whose keys are 16 hex
digits (`statedir.KeyHexDigits`), so `auth` never collides with a store's name.

## Rules every package keeps

1. **Commit body.** The executor's report ends with a section headed `Commit body`, which the
   coordinator copies into the commit. It names every truth the package moved, including each
   fixture and what it now answers (CONVENTIONS "Testing"). It also lists every model-facing text
   the package changed:
   - cobra `Short` and `Long` text and flag usages
   - `internal/mcp/instructions.txt`
   - any MCP resource text
   - the stamped templates under `internal/kbdocs/templates/`
   - the `config.toml` and `providers.toml` template text in `internal/config/template.go`
   - a refusal's or failure's detail and remedy text
   - a verb's acknowledgement text

   The coordinator sends that list to prompt-engineer review before the commit lands.
2. **A pin moves with its fact.** A test or fixture that pins a fact the package moves is updated in
   the same package (CONVENTIONS "Testing").
3. **The contract moves with the code.** Each package updates the SPEC, ARCHITECTURE, CONVENTIONS
   and README text its change makes stale. It moves nothing else in them.
4. **Keys stay out of logs and errors.** CONVENTIONS "internal/model": "API keys never appear in
   logs or error strings". This binds every new path that reads, writes, decrypts or resolves a
   key. Each package names the test that pins this for its own paths.
5. **Failure messages say whether, never what.** When a test fails, its message says whether a key
   appeared or matched. It never formats a value that holds a key, or would hold one if the
   assertion failed. Show the length, or show the struct with its key field left out. The
   incumbent form is `cmd/models_test.go:132`, which prints
   `model.Endpoint{Name: e.Name, BaseURL: e.BaseURL}`.
6. **Every new path goes through a runner.** A new path is verified by a runner recipe or a unit
   test, never by a one-off command. Done-lines are `just` recipes, from the `just` listing.
7. **The executor builds and tests through `just` only.** It never runs `go test` or `go build`
   directly.

## Packages

| id | rating | one line | depends on | status |
|---|---|---|---|---|
| A | mechanical | test failure messages never print a key | none | landed |
| B | expert | `internal/auth`: the encrypted per-provider files; an atomic write that takes a mode; one state-base function | A | landed |
| C | expert | `kbase auth add`, `auth remove` and `auth list`, CLI-only; `cmd` tests isolate the store | B | landed |
| D | expert | resolution reads the store first; `show-config` reports `store`, the upgrade and the unread-file warning; hermetic recipes isolate the store | C | pending |
| E | expert | a 401 or 403 from the provider fails with the remedy `kbase auth add <provider>` | D | pending |
| F | expert | the served surface stops naming `apiKeyFile`, except in `show-config`'s report | E | pending |

**Order:** one package at a time, A through F, with one agent editing the tree at once.

---

### A — test failure messages never print a key (mechanical)

**Scope:** the failure calls below, found by the first command. The rule is Rule 5. Change only
the message's arguments, never the assertion.

```
git grep -n -A3 -E 'testAPIKey|fakeKey|sentinel|fileKey|APIKey|apiKey|secret-key|gotAuth|x-api-key|Authorization' -- '*_test.go' \
  | grep -E '[-:][0-9]+[-:].*t\.(Errorf|Fatalf|Logf|Error|Fatal)\(' | grep -v -E 't\.(Error|Fatal)\("'
```

From that output, these lines format a value that holds a fixture key, or would hold one if the
assertion failed:

- `cmd/build_test.go:170`, `:183`
- `cmd/probe_test.go:626`
- `cmd/score_test.go:491`, `:494`
- `cmd/show-config_test.go:159`
- `internal/config/expand_test.go:81`, `:88`
- `internal/config/inference_test.go:108`, `:237`
- `internal/config/layer_test.go:162`
- `internal/config/providers_test.go:155`, `:158`, `:192`, `:384`, `:388`, `:467`
- `internal/model/anthropic_client_test.go:156`, `:430`, `:499`
- `internal/model/decision_test.go:165`, `:306`
- `internal/model/openai_client_test.go:144`, `:227`, `:303`, `:306`, `:392`, `:639`

The other lines the command lists format no key, and they stay as they are. If the executor finds a
site that is not in this list, it reports the site and does not change it.

**Signatures and types changed:** none.

**Done-line:** `just edit-gate`, then `just test`.

---

### B — the store (expert)

**Scope:**
- `internal/auth/` (new)
- `internal/atomicfile/atomicfile.go`
- `internal/statedir/statedir.go`, and its test file
- `cmd/importpolicy_test.go` (`dependencyRules`)
- ARCHITECTURE §11 (package map and dependency rules) and §12 (Constants)

**Facts acted on:**
- `atomicfile` takes no mode argument. `internal/atomicfile/atomicfile.go:19`:
  `const CreateMode = 0o666`, "the umask decides what lands on disk". At `:32`: "An existing file
  keeps its mode". Each store file needs `0600` when it is created and on every replace. Add a
  mode-taking write beside `Write`, and leave `Write`'s signature alone:
  `git grep -l 'atomicfile\.Write(' -- '*.go' | wc -l` → `15` callers.
- The state base is computed inline in `statedir.Dir` (`internal/statedir/statedir.go:35-43`):
  `$XDG_STATE_HOME`, else `~/.local/state`, joined with `kbase`. Extract it into an exported
  function that `Dir` calls, so the store's directory and the state stores share one rule. `Dir`
  keeps its signature: `git grep -l 'statedir.Dir(' -- '*.go'` → `cmd/replay.go cmd/score.go
  internal/build/build.go internal/buildstore/status.go internal/score/status_test.go`.
  ARCHITECTURE §11: `statedir` is "imported by build, buildstore and cmd", and `cmd` is the caller
  that joins the base with `auth/`.
- `cmd/importpolicy_test.go` places no requirement on a new package. Rules bind only the packages
  their `from` or `except` lists name (`dependencyRule.binds`). A new `internal/auth` that no rule
  mentions is unconstrained, so this package adds its rules.
- `crypto/hkdf` is in the standard library from Go 1.24. `go.mod`: `go 1.26.5`.
- Provider names are TOML table headers, an open set. `config.ParseReference`
  (`internal/config/resolve.go:63-67`) already holds that "a provider name may not" contain `/`.

**Work:**
- **The blob.** Use AES-GCM under a per-user key derived by HKDF from a plain 32-byte constant in
  the source. HKDF's `info` is the username alone. Each write uses a fresh nonce. The blob has no
  format version. Nothing binds a ciphertext to its provider's name: the name is in neither the GCM
  additional data nor the HKDF `info`. A file renamed by hand therefore decrypts under its new
  name.
- **One file per provider,** `<provider>-key.enc` in a directory the caller passes. The package
  owns the file-name rule:
  - a name to its file;
  - a file back to its name, for `auth list`;
  - refusal of a name that is empty, is `.` or `..`, or contains `/`, `\` or a NUL. A name that
    cannot be a file name is a user-input boundary, and the check runs before any path is built.
- **Operations over an explicit directory:** add (create or replace), remove, read one, and list
  names. Each write goes through the mode-taking atomic write at `0600`. Writes to separate files
  need no lock. Two concurrent adds of one name end with the last one written.
- **Errors carry no key.** Every error is returned as data, with the path but no key and no
  plaintext. Covered failures: a tampered or truncated file, a file written for another user, a
  username the OS cannot supply, and an unusable name. Callers can tell each failure apart with
  `errors.As` or `errors.Is`, so package D can report it.
- **Import rules.** Add to `dependencyRules`:
  - `internal/auth` imports nothing of kbase but `internal/atomicfile`;
  - only `cmd` imports `internal/auth`.

  `config` already cannot import it (`allowed: atomicfile, result`).

**Contract:**
- **ARCHITECTURE §11.**
  - Add a package-map entry for `internal/auth`.
  - Add the two dependency rules.
  - Update `atomicfile`'s description: it now has a mode-taking write.
  - Update `statedir`'s description: it now owns the state base.
- **ARCHITECTURE §12.** Add the store's directory name, file suffix and permission.
- **SPEC.** No change in this package; C states the verbs.

**Tests (`just test-pkg internal/auth`):**
- a round trip;
- two writes of one key produce different bytes;
- a file is `0600` after creation, and after a replace over a `0644` file;
- the file's bytes do not contain the plaintext;
- a file copied to another name decrypts under that name;
- each unusable name is refused;
- `TestStoreErrorsCarryNoKey`: every failure above returns an error whose text does not contain
  the sentinel key.

**Done-line:** `just edit-gate`, `just test-pkg internal/auth`, `just test-pkg internal/atomicfile`,
`just test-pkg internal/statedir`, then `just test`.

---

### C — `kbase auth add`, `auth remove` and `auth list` (expert)

**Scope:**
- `cmd/auth.go` (new)
- `cmd/surface.go`
- `cmd/mcp_test.go` (`TestMain`, line 52)
- `cmd/resultkeys_test.go`: `documentedKeys` (line 19), and the `kb-root` exemption in
  `checkDocument` (line 165), which names `models`, `configure`, `show-config` and `probe` and
  gains the auth verbs
- SPEC §7 (the result table, and the `kb-root` exemption at line 500), §9 ("Provider commands"),
  §11 ("Tools", lines 2097–2099)
- ARCHITECTURE I9 (line 32), §11 "MCP server"
- README "Configuration" and the MCP not-offered list
- `cmd/main.go` (root `Long`), `cmd/auth_test.go` (new), CONVENTIONS (subcommands are added in
  `cmd/surface.go`)

**Facts acted on:**
- `auth` is the first command with subcommands. The tool table reads `root.Commands()` alone
  (`internal/mcp/mcp.go:252`, `boundCommands`). Declaring the parent `cliOnly` therefore keeps
  `add`, `remove` and `list` out of the table.
- I9's CLI-only list is closed. ARCHITECTURE line 32: "only process control (`mcp`, `wait`),
  environment diagnostics (`probe`) and cobra's own `help` and `completion` are CLI-only". SPEC
  §11 line 2097: "Every subcommand is served, less `completion`, `help`, `mcp` itself and `wait` …
  and less `probe`". Both lists gain `auth`, with its reason: no tool call has stdin, and no tool
  accepts a key.
- Configuration is loaded per verb by `loadVerbContext`; the root's `PersistentPreRunE`
  (`cmd/main.go:85-95`) builds only the logger. `auth add` loads it, to refuse a name no
  configured entry carries (ACTIVE_DESIGN decision 2); `remove` and `list` load none.
- No new module is needed to detect a tty. `os.Stdin.Stat()` reports `os.ModeCharDevice` for a
  terminal.
- `cmd`'s `TestMain` (`cmd/mcp_test.go:52`) isolates nothing. The command
  `git grep -n -E 'Setenv\("(HOME|XDG_[A-Z_]+|KBASE_CONFIG_DIR)"' -- 'cmd/*_test.go'` returns only
  `cmd/probe_test.go:851` and `cmd/show-config_test.go:371`.

**Work:**
- **Verb shape.** Each verb's logic sits behind an options-struct function, and `RunE` is a thin
  shell around it (CONVENTIONS "cmd/"). `RunE` resolves the store directory (B's state base, then
  `auth/`) and passes it in the options struct.
- **`auth add <provider>`.** It takes exactly one positional argument, a `providers.toml` entry
  name, and checks it with B's name rule.
  - **Tty.** Stdin that is a terminal is refused, writing nothing. The refusal names the piped
    form. Whether stdin is a terminal is a seam on the options struct.
  - **Empty input.** Piped stdin that is empty after trimming surrounding whitespace is refused,
    writing nothing. The key is trimmed as `resolveAPIKey` trims a key file
    (`internal/config/providers.go:276`).
  - **Output.** The acknowledgement is the result document, and it says nothing about the key.
  - **Help.** The `Long` text shows `<key-manager> --print-key | kbase auth add <provider>` and
    `<your-system-clipboard-paster> | kbase auth add <provider>`, and names no platform's command.
- **`auth remove <provider>`** clears the named key, and reports `unchanged` where there was none.
- **`auth list`** names the providers with a stored file, by name only, in name order.
- **Usage refusals.** A second positional argument is a usage refusal, and the refusal does not
  repeat that argument.
- **Test isolation.** `TestMain` sets `XDG_STATE_HOME` to a directory from `os.MkdirTemp`,
  removed after `m.Run`. From this package on, every `cmd` test that resolves the store's default
  location resolves it under scratch. C's own verb tests also pass an explicit directory under
  `t.TempDir()`.

**Tests:**
- a tty on stdin refuses, names the piped form, and leaves the directory empty;
- empty piped stdin refuses and writes nothing;
- add, list and remove round trip;
- `TestAuthKeyAppearsNowhere`: a sentinel key piped to the add function with a debug-level logger
  teed to a file under `t.TempDir()`, and the sentinel absent from stdout, stderr, the log file and
  every file in the store directory;
- a second argument is refused without being echoed;
- `TestNoSurfaceNamesACredential`: walk every command recursively and every tool schema, and fail
  any flag or property named for a key or credential. Mirror
  `TestNoSurfaceNamesTheProvenanceOrCreate` (`cmd/mcp_test.go`);
- `TestStoreLocationIsScratch`: the store directory `RunE` resolves lies under `TestMain`'s
  scratch directory.

**Done-line:** `just edit-gate`, `just test`, `just test-integration-mcp`.

---

### D — resolution and reporting (expert)

**Scope:**
- `internal/config/providers.go`, `layer.go`, `report.go`, `resolve.go`, and their tests
  (`providers_test.go`, `layer_test.go`, `report_test.go`, `inference_test.go`,
  `template_test.go`, `expand_test.go`, `configure_test.go`)
- `cmd/provider.go` (`loadConfigContext`, `warnFaults`)
- `cmd/show-config.go`, `cmd/show-config_test.go`, `cmd/resultkeys_test.go` (line 50)
- the hermetic recipe scripts that run a verb which reads the store, each gaining
  `XDG_STATE_HOME` under its evidence directory:
  - `tools/scripts/test-integration-config.sh`
  - `tools/scripts/test-integration-results-fixture.sh`
  - `tools/scripts/test-integration-build-arxiv.sh`
  - `tools/scripts/test-integration-monitor-arxiv.sh`
  - `tools/scripts/test-integration-mcp.sh`
- a new recipe `test-integration-auth` in `justfile`, its script
  `tools/scripts/test-integration-auth.sh`, and the `test-integration` omnibus (`justfile:300`)
- SPEC §7 (line 558), §9 (lines 1255–1257, 1330–1336), §9.3 (lines 1464–1467), §9.6
  (lines 1523–1524)
- ARCHITECTURE §7's configuration bullets (lines 993–1030)
- README "Configuration" (lines 466–472)

**Facts acted on:**
- The key file is read at load. `LoadProviders` calls `resolveAPIKey` per entry
  (`internal/config/providers.go:250`). A read failure drops the entry (`:251-254`), and
  `keyFileExposure` warns (`:255-257`). The design says an entry with a stored key does not read
  its `apiKeyFile`, so the load must know which names the store holds before it reads any key
  file. `config.Load` therefore takes the stored keys.
- `config.Load` callers: `git grep -n -E '(^|[^a-zA-Z.])Load\(|config\.Load\(' -- 'internal/config/*.go' 'cmd/*.go' | cut -d: -f1 | sort -u`
  → `cmd/provider.go internal/config/layer.go internal/config/layer_test.go internal/config/template_test.go`.
  Every file listed is in scope for the signature change.
- `git grep -l 'LoadProviders' -- '*.go'` → `cmd/show-config.go internal/config/config.go
  internal/config/configure_test.go internal/config/expand_test.go internal/config/layer.go
  internal/config/providers.go internal/config/providers_test.go`.
- `git grep -l 'KeyKind' -- '*.go'` → `cmd/show-config_test.go internal/config/report.go
  internal/config/report_test.go internal/config/resolve.go`.
- `git grep -l 'KeySource' -- '*.go'` → `cmd/show-config.go internal/config/resolve.go`.
- `git grep -l -w 'KeyFile' -- '*.go'` → `cmd/show-config.go internal/config/report.go
  internal/config/report_test.go`.
- `ProviderFault` keeps its shape, `{Name, Reason, Path, Warning}`
  (`internal/config/providers.go:105-120`). Its referencers are not in scope:
  `git grep -l 'ProviderFault' -- '*.go'` → `cmd/configure_test.go cmd/models_test.go
  cmd/provider.go cmd/show-config.go internal/config/inference_test.go internal/config/layer.go
  internal/config/providers.go internal/config/providers_test.go internal/config/report.go
  internal/config/report_test.go internal/config/resolve.go`.
- `warnFaults` prints every `ProviderFault` on the stderr of every configuration-loading verb:
  `git grep -l 'warnFaults' -- '*.go'` → `cmd/build.go cmd/configure.go cmd/models.go
  cmd/probe.go cmd/provider.go cmd/score.go`. The design (outline item 3) makes the upgrade and the
  unread-`apiKeyFile` warning `show-config`'s alone ("no other verb's output carries them"). So
  neither is a `ProviderFault`. Both are computed in `config.Report` (`internal/config/report.go`)
  from the loaded pool and the stored names, and `show-config` alone emits them.
- The comment at `internal/config/layer.go:145-147` says whole-entry replacement means "a
  repository entry cannot borrow the global entry's credential". After this package that holds for
  `apiKeyFile` only, and the comment is corrected.
- Only the verbs that load configuration read the store. Which recipe scripts run one:
  - `grep -l -E 'kbase[}"]? +(build|score|probe|models|configure|show-config)|\$\{kbase\} (build|score|probe|models|configure|show-config)' tools/scripts/*.sh`
    → `prep-test-integration-fixture.sh test-integration-build-arxiv.sh
    test-integration-build-lamb.sh test-integration-build-lamb-cycle.sh
    test-integration-build-live-arxiv.sh test-integration-build-live-fixture.sh
    test-integration-config.sh test-integration-monitor-arxiv.sh
    test-integration-results-fixture.sh`
  - `grep -l XDG_STATE_HOME tools/scripts/*.sh` → `test-integration-build-lamb.sh
    test-integration-build-lamb-cycle.sh test-integration-init.sh test-integration-mcp-build.sh
    test-integration-stage-fixture.sh`
  - `grep -l -E '"name":"(show-config|models|configure|build|score)"' test-data/fixtures/mcp/*.jsonl`
    → `test-data/fixtures/mcp/build-refused.jsonl`, a `build` call that
    `test-integration-mcp.sh` serves, so that script is in scope too.

  Excluded from the first list, so not in scope:
  - `test-integration-build-lamb.sh` and `test-integration-build-lamb-cycle.sh` already set
    `XDG_STATE_HOME`;
  - the LIVE recipes' scripts, `test-integration-build-live-arxiv.sh` and
    `test-integration-build-live-fixture.sh`, run against the user's own configuration by design
    and read the user's store as a user's run does;
  - `prep-test-integration-fixture.sh` matches only a message string (`:79`, "a kbase build is
    running over …") and runs no configuration-loading verb.

**Work:**
- **Reading the store.** `loadConfigContext` reads every stored file through `internal/auth`. It
  hands `config.Load` the keys by name, plus one fault per file it could not read. Each such fault
  is a `ProviderFault` of the existing shape: `Name` the provider, `Path` the store file,
  `Warning` true.
- **The precedence.** Write it as a pure `config` function: the store's key for the entry's name,
  whatever layer supplied the entry; else the key from `apiKeyFile`; else none. An entry with a
  stored key does not read its `apiKeyFile`, and so raises neither a read fault nor an exposure
  warning for it.
- **The kind.** `KeyKind` gains `store`, and `Endpoint.KeySource` names the store as its source.
- **An unreadable store file** is treated as no stored key, and resolution falls through to the
  entry's `apiKeyFile`, else none. It is reported by path and failure kind, never content:
  - `show-config` lists it as a diagnostic, and its outcome stays `done`;
  - every other configuration-loading verb writes it as a `warnFaults` line on stderr.

  When resolution falls through and reads the key file, that read behaves as it does today. A file
  that cannot be read raises its own read fault and drops the entry (`providers.go:251-254`), and
  an exposed file raises its permission warning (`:255-257`). The entry then carries two faults,
  the store file's and the key file's. A call that goes out keyless and is refused gets E's
  remedy, and `auth add` replaces the store file.
- **`show-config`'s report:**
  - **Key from a file.** It keeps `key: file` and `key-file`, and adds an `upgrade` field with the
    command, values filled in: `cat <key-path> | kbase auth add <provider>`. `<key-path>` is the
    resolved path (`keyFilePath`, `internal/config/providers.go:282`), single-quoted where it holds
    a character the shell splits on or expands.
  - **Both a stored key and `apiKeyFile`.** It reports `key: store` and a warning diagnostic that
    the entry's `apiKeyFile` is not read. The warning's `path` is the `providers.toml` that states
    it. Its remedy is: remove the `apiKeyFile` line from that file, or
    `kbase auth remove <provider>` to use the file. Like the `upgrade` field, it is computed in
    `config.Report` and appears in `show-config`'s document alone, never through `warnFaults`.
  - **`Long`.** It states the three kinds and that a key is added by the user running
    `kbase auth add`, never through a tool.
- **Recipes.** Each hermetic script listed in Scope sets `XDG_STATE_HOME` under its evidence
  directory for every kbase call.
- **`test-integration-auth`.** It is hermetic, carries a `[doc]` naming its legs and its evidence
  directory, and joins the `test-integration` omnibus. Its legs, all with `XDG_STATE_HOME` and
  `--config-dir` on scratch:
  1. `./bin/kbase auth add` by pipe; `auth list` names the provider;
  2. `show-config` reports `key: store`, and the key's text appears on no stream and in no file
     except the store file;
  3. `auth add` with empty piped stdin is refused, and the store is unchanged by `cksum`;
  4. an entry with both a stored key and `apiKeyFile` reports `key: store` and the
     unread-`apiKeyFile` warning;
  5. an entry with `apiKeyFile` alone reports its `upgrade`, and running that command as printed
     makes the entry report `key: store`;
  6. a store file truncated by the script is reported as a diagnostic, with `show-config` still
     `done`;
  7. `auth remove`, after which `show-config` reports `key: file` or `none`, as the entry states;
  8. evidence goes under `test-data/transient/test-integration-auth/`.

  A tty on stdin cannot be staged hermetically, so C's unit test covers that refusal.

**Contract:**
- SPEC §7's `show-config` row: `key` gains `store`; the providers record gains `upgrade`;
  diagnostics gain the unread-file warning and the store fault.
- SPEC §9, lines 1255–1257: the "a repository cannot inherit a credential" sentence narrows to
  `apiKeyFile`.
- SPEC §9, lines 1330–1336: the store becomes a source of a key beside the files.
  `$XDG_STATE_HOME` locates it, and `--config-dir` does not isolate it.
- SPEC §9.3, lines 1465–1467: the trust sentence names the store beside key files.
- SPEC §9.6, lines 1523–1524: the "Names and paths, never secrets" sentence names the store kind
  and the upgrade command.
- ARCHITECTURE §7: the store's reading in `cmd`, and the precedence in `config`.
- README "Configuration": `kbase auth add` is the setup path.

**Tests:**
- the precedence, and that a stored name keeps its `apiKeyFile` unread (no fault for a missing
  file), in `config` unit tests;
- an unreadable store file over a missing key file: both faults raised, the entry dropped;
- the upgrade and the unread-file warning on `show-config`'s document, and on no other verb's
  stderr;
- `TestStoredKeyNeverReported` (`cmd`): `show-config` over a store holding a sentinel key; the
  sentinel absent from stdout and stderr;
- the `upgrade` value's quoting, over a path holding a space.

**Done-line:** `just test-integration`, then `just checkpoint`.

---

### E — a refused key fails with the remedy (expert)

**Requirement** (design, outline item 2): "Nothing is checked before dialling: a provider that
answers 401 or 403 fails the call as `provider-refused` does today, and that failure's item
carries the remedy `kbase auth add <provider>`, the provider's name filled in — for a missing key
and a wrong one alike."

**Scope:**
- `internal/model/client.go` (`StatusError`)
- `internal/config/resolve.go`, the remedy text's one home, beside `configureRemedy` (line 215)
- `internal/asks/multichoice.go` (`IncompleteError`) and `internal/asks/call/call.go`
- `internal/claimgraph/report.go` (`stop`) and `internal/claimgraph/stages.go` (`askStop`)
- `internal/build/rows.go` (overview failure, lines 444–447)
- `cmd/score.go`, `internal/score/invoke.go` (`Failures`, line 87; `failures`, line 184),
  `internal/score/run.go` (the failed entry, line 287), `internal/score/ledger.go` (`Entry`, line
  134; `attempt`, line 154), and their tests: `internal/score/ledger_test.go`,
  `internal/score/status_test.go`, `cmd/score_status_test.go`
- SPEC §8, the `ledger.jsonl` line text (lines 1063–1071)
- `cmd/probe.go`
- `internal/modelclient/modelclient.go` (`Catalogue`), `cmd/models.go`, `cmd/configure.go`
- the tests of each, including every file listed below for a type that changes
- SPEC §6 and §8 text for the failure items that gain the remedy; SPEC §7's `remedy` use

**Facts acted on:**
- Today the status is lost on the build path. `asks/call` turns a failed call into
  `asks.IncompleteError{Subject, Detail}` with `fmt.Sprintf("%v; capture: %s", err, …)`
  (`internal/asks/call/call.go:232`, `:510`). `claimgraph`'s `askStop` turns that into
  `stopf("inference-failed", "%s", …)` (`internal/claimgraph/stages.go:87-93`). Neither carries a
  remedy, and `errors.As` cannot find a `model.StatusError` after the `%v`.
- Who may classify a status: `asks` imports no `model` (its imports are `atomicfile` and `kb`).
  `claimgraph` "reaches neither `model` nor `asks/call`" (ARCHITECTURE §11). Within `cmd`, only
  `cmd/build.go`, `cmd/score.go` and `cmd/probe.go` may import `model` (`cmdFileImporters`,
  `cmd/importpolicy_test.go`). The 401/403 test therefore happens in `asks/call`, `cmd/score.go`,
  `cmd/probe.go` and `modelclient`. It travels as data to `claimgraph`, `build`, `score`,
  `cmd/models.go` and `cmd/configure.go`.
- `score` reports failed calls as items: `rep.Failures` under `toolresult.FailuresKey`
  (`cmd/score.go:279-280`), built from the ledger by `failures(res.Walked, l)`
  (`internal/score/invoke.go:184`). `internal/score` imports no `model`.
- Types and functions that change, with every file that names them:
  - `git grep -l 'IncompleteError' -- '*.go'` → `internal/asks/call/call.go
    internal/asks/call/call_test.go internal/asks/multichoice.go internal/asks/multichoice_test.go
    internal/asks/replay_test.go internal/build/replay.go internal/build/rows.go
    internal/claimgraph/commonfiles_test.go internal/claimgraph/launch_test.go
    internal/claimgraph/stages.go internal/claimgraph/stop_test.go
    internal/claimgraph/unmarked_test.go`
  - `git grep -l 'StatusError' -- '*.go'` → `cmd/score_scoring_test.go
    internal/model/anthropic_client.go internal/model/anthropic_client_test.go
    internal/model/chat.go internal/model/client.go internal/model/decision.go
    internal/model/decision_test.go internal/model/openai_client.go internal/model/transport.go`
  - `git grep -l 'stopf' -- '*.go'` → `internal/claimgraph/assemble.go
    internal/claimgraph/conform.go internal/claimgraph/cull.go internal/claimgraph/graph.go
    internal/claimgraph/inventory.go internal/claimgraph/report.go internal/claimgraph/stages.go
    internal/claimgraph/unmarked.go internal/claimgraph/write.go`. Leave `stopf`'s signature alone
    and give the inference stop its own constructor, so these callers do not move.

**Work:**
- **One predicate.** It says a `StatusError` refused the credential: 401 or 403, from `net/http`'s
  named constants. It lives on `model.StatusError`.
- **One home for the remedy text,** `kbase auth add <provider>`, as a `config` function beside its
  other remedies (`configureRemedy`, `internal/config/resolve.go:215`).
- **Threading the remedy.** At each classifying site, a 401/403 attaches the remedy, with the name
  of the provider that was dialled, to what travels outward: `IncompleteError` gains a remedy field;
  the inference stop carries it into the build's failure item; and `score`'s failure item,
  `probe`'s item, and `models`' and `configure`'s catalogue failure carry it. The provider name is
  the dialled endpoint's `config.Endpoint.Provider`.
- **No reason is parsed back.** No site recovers the status by matching a reason string.
- **`score` records the remedy in its ledger.** `score`'s failure items are built from the ledger
  (`internal/score/invoke.go:184`), so a remedy held only in memory would be lost on `--resume`.
  - A failed attempt's ledger line gains an optional `remedy` field, set where the call was
    refused for its key.
  - `cmd/score.go`'s ask closure, which may import `model`, hands the remedy to `internal/score`
    in an error type `score` defines. `score` still imports no `model`.
  - `failures` copies the line's `remedy` onto the item.
  - The field is optional, so a ledger written before it still reads, and the ledger's format
    stamp (`internal/score/ledger.go:100`) does not move. Moving it would refuse the resume of
    every earlier run.
  - SPEC §8's ledger line text names the field.
- **The outcome does not change.** The call fails as it does today. Only the item gains a remedy.

**Tests:**
- per path, a stub answering 401 and one answering 403: `build` (a decision ask and the overview),
  `score`, `probe`, `models` and `configure --provider`, each failure item carrying
  `remedy: kbase auth add <provider>` with the stub's entry name;
- `score` failed with a 401, then resumed: the resumed run's failure item still carries the
  remedy from the ledger;
- a stub answering 500 carries no such remedy;
- `TestRefusedKeyFailureCarriesNoKey`: a sentinel key sent to a 401 stub, absent from every item
  and stream.

**Done-line:** `just test-integration`, then `just checkpoint`.

---

### F — the served surface stops naming `apiKeyFile` (expert)

**Precondition:** D's `just test-integration-auth` is green. The store route does the job before
the served text about `apiKeyFile` is withdrawn. `apiKeyFile` stays supported, SPEC §9.1 keeps
documenting it, and `show-config`'s report keeps naming it (design decision 4).

**Scope:**
- `internal/config/resolve.go:291`
- `internal/config/providers.go:274`, `:318`, and the test that pins the first
  (`internal/config/providers_test.go:154`)
- `internal/config/template.go:40-43`, `:51`, `:61`
- `internal/config/template_test.go` (`TestTemplatesSpellEveryModeledKey`, line 54)
- a new test, in `cmd`
- SPEC §9.4 (key-file warning wording)
- README "Configuration" (lines 466–472): `apiKeyFile` is for CI and mounted secrets

**Facts acted on:**
- Served text outside `show-config`'s report that names the field today:
  - the `unresolvedProvider` detail, `resolve.go:291`: `"… a pool entry there supplies baseUrl and
    apiKeyFile"`;
  - the `providersTemplate` lines `template.go:51`, `# apiKeyFile = "~/.api_keys/lab-server.txt"`,
    and `:61`, `# apiKeyFile = "~/.api_keys/cloud.txt"`;
  - the fault reasons `"read apiKeyFile: %w"` (`providers.go:274`) and
    `"apiKeyFile %s is readable beyond its owner (mode %#o) — chmod 600 it"` (`:318`). These reach
    every configuration-loading verb's stderr through `warnFaults`, as well as `show-config`. The
    acceptance line "No served text an agent reads names `apiKeyFile`" covers that stderr, so both
    are reworded to "key file", still naming the path.
- `show-config`'s help does not name the field, and design outline item 4 no longer lists it.
  Its `Long` (`cmd/show-config.go`) says "a key file by the path its entry states". D rewrites
  it, and F does not touch it.
- `internal/mcp/instructions.txt` and `internal/kbdocs/templates/*` name no key.
  `grep -n -i key internal/mcp/instructions.txt` returns nothing. The template matches are
  unrelated: result `outcome` key, values key, legend.
- `TestTemplatesSpellEveryModeledKey` requires `providersTemplate` to spell every modeled
  `Provider` key, `apiKeyFile` included (`template_test.go:50-54`: "Every key the loaders model is
  spelled in the template it belongs to"). The test's rule narrows: it names the one key the
  template deliberately omits, and says why.
- `internal/config/template.go:40-43`, the `providersTemplate` doc comment, says: "apiKeyFile is
  commented because a live one naming a file that is not there drops its entry — a fault where an
  endpoint was meant." Once the template no longer carries the key, that sentence describes a line
  that is gone.
- The MCP fixtures do not pin this text. `git grep -l -E 'apiKeyFile|key file' --
  test-data/fixtures` returns nothing.

**Work:**
- Remove the field name from `resolve.go:291` and from both fault reasons.
- The template's commented key lines (`:51`, `:61`) become one comment naming
  `kbase auth add <name>`.
- In the doc comment at `template.go:40-43`, replace the `apiKeyFile` sentence with one saying the
  template names no key field because a key is added with `kbase auth add`, while `apiKeyFile`
  stays loadable. Name no project document in the comment.
- Add `TestServedTextNamesNoKeyFile`, which fails on `apiKeyFile` in any of:
  - every command's help, walked recursively;
  - every MCP resource;
  - the MCP instructions;
  - both templates;
  - the unresolved-provider refusal;
  - the two fault reasons.

  `show-config`'s result document is exempt, by decision 4.

**Done-line:** `just test-integration`, then `just checkpoint`.

---

## Facts for the adjagent change request

The coordinator writes this request after the plan lands; it is not part of any package here.

- `git -C ../adjagent grep -n -E 'apiKeyFile|auth add|KBASE_API_KEY'` returns nothing at adjagent
  `d092cd2`. No adjagent line names `apiKeyFile`, `auth add` or `KBASE_API_KEY`.
- The nearest guidance is `../adjagent/adjagent/definitions/kbase.py:306-312`. On a `check:
  provider` refusal it tells the agent to "call the `show-config` tool and relay its document
  rather than reading the file yourself", then to offer `configure` with `create` and `models`. It
  has no key step. That passage is where the skill learns of `kbase auth add`: the agent tells the
  user to run it, or runs the piped form against the user's key manager, and never asks for the
  key.

## Post-sprint review

This section runs after package F lands. Two reviewers: an architect and a coder.

**What each reviewer receives:**
- Pre-task reading: THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md.
- Not this plan. The review judges the changes against the standards and against fitness for
  purpose. It does not retrace the implementation.
- The range from this plan's first package commit to its last.

**What each reviewer reviews:**
- every changed file;
- the tests of every touched package: `internal/auth`, `internal/atomicfile`, `internal/statedir`,
  `internal/config`, `internal/model`, `internal/modelclient`, `internal/asks`,
  `internal/asks/call`, `internal/claimgraph`, `internal/build`, `internal/score` and `cmd`;
- the new recipe and the recipe scripts that changed.

**Questions to answer, among others:**
- Does any path print, log or return a key?
- Does a test or hermetic recipe reach the user's store?
- Does the precedence have exactly one home, and the remedy text one home?
- Is any served text outside `show-config`'s report still naming `apiKeyFile`?
- Is "no test prints a key" held by anything but package A's one-time sweep? An AST policy scan
  is the tool that would hold it.

**After the review:**
1. Merge the findings into a burndown at `ACTIVE_PLAN_REVIEW_BURNDOWN.md`.
2. Send the burndown to prompt-engineer review, and revise it from the findings.
3. Dispatch it as its own packages, each meeting "Plan dispatchability".
