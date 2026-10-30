# ACTIVE_PLAN

Built from [ACTIVE_DESIGN](ACTIVE_DESIGN.md)

## Outcome

When this plan is done:

1. Inference is configured by `providers.toml` and `config.toml` alone. kbase reads none of
   `KBASE_API_BASE_URL`, `KBASE_API_KEY_FILE`, `KBASE_DECISION`, `KBASE_TEXT`, `KBASE_SCORING`. A run
   isolated from the user's layers names its directory with `--config-dir` or `$KBASE_CONFIG_DIR`,
   both of which stay.
2. `config.toml` models no top-level `provider` key and no `[asks] readerConcurrency`. A file stating
   either fails strict load (SPEC §9.3), naming the key.
3. Every test and integration leg that used the environment as configuration reaches the same
   endpoint through a configuration directory. Each property it pinned is still pinned.
4. What an agent needs to drive kbase is served by kbase (ARCHITECTURE §3, "The served surface is the
   operating contract"):
   - every command's help is readable on demand as an MCP resource;
   - the queries are one MCP tool and the write ops one MCP tool, each operation still a subcommand;
   - one `strength` query replaces `weak-points` and `strength-below`;
   - `stats` absorbs `edge-census`;
   - `drop-claim-node` deletes a claim in one call;
   - `refresh` absorbs `render-claim-graph`;
   - `USER_CONVENTIONS.md` is gone, its needed content placed on the served surface.
5. The served surface's size is measured by one recipe before the first change and after each
   surface package. The numbers are recorded; nothing gates on them.
6. SPEC, ARCHITECTURE, CONVENTIONS, README.md, the stamped kb-root templates, the MCP instructions and
   the cobra help describe all of the above, and SPEC §12 carries one release entry for it.

## Settled consequences

These are decided. Packages implement them as stated.

**Configuration.**

- **What a user with an old `config.toml` sees.** Every verb that reads configuration refuses before
  doing anything else.
  - **The refusal.** Outcome `refused`, exit 1, one item with `check: usage`. Its detail is the
    strict-load message: the path, then `unknown key "provider"` (or `"asks.readerConcurrency"`, or
    `unknown keys` naming both), then the standing hint. That is the shape
    `TestShowConfigRefusesAConfigurationThatDoesNotLoad` (`cmd/show-config_test.go:423`) pins for
    `[models] heavy`.
  - **The remedy is a hand edit.** SPEC settles this already. §9.2: a `heavy`/`light` table "fails
    strict load as unmodeled keys (§9.3); the remedy is a hand edit". §9.3: "`kbase configure`
    cannot repair a config file it cannot load".
  - **No environment-variable diagnostic.** No consumer outside this repository sets the variables
    (P3 "Facts"). A check for them would read nothing anyone produces (CONVENTIONS "Name the producer
    before you write the refusal").
- **No "at the next major" wording remains.** SPEC, help and comments state the removal itself.
- **SPEC §12 carries one entry for this release, with no chosen version.**
  - Its heading is
    `**<release version> — the files configure inference; the served surface is smaller.**`.
    `<release version>` is a literal placeholder the release commit replaces.
  - The first package that adds a bullet creates the entry at the top of §12, and each later package
    adds its bullet. Past entries are not rewritten.
- **`configure` loses its one deletion.** The editor in `internal/config/write.go` stops knowing the
  `provider` key. A `provider` line that reaches it passes through as any other byte (SPEC §9.5), and
  the next load refuses it by name.
- **The role record loses `back-end`; `show-config` loses `mode` and `environment`.**
- **The score ledger's `backend` stays.** It stays in `run.yaml`, `ledger.jsonl`, `captures.jsonl`
  and the replay header.
  - New runs write `provider`, whose one home is `config.BackEndProvider`.
  - Runs started earlier may record `environment`. They stay readable and resumable.
  - A resumed environment-started run continues on whatever the files now resolve, and its new lines
    record that.
- **`KBASE_CONFIG_DIR` stays.** So do `config.LayerSourceEnvironment` and the `environment` value of
  `layers[].source`, `provider-layer` and `sources`. They name the directory `$KBASE_CONFIG_DIR`
  selects, not the inference mode.
- **`KBASE_API_KEY_FILE` goes, its file-mode key override included.** A pool entry's `apiKeyFile` is
  the one key path, and it may name a mounted secret.
- **SPEC §11's "configured as the server is (the environment, `--config-dir`, …)" is unchanged.**
  That environment is the process environment.

**Served surface.**

- **Help on demand.** S1 serves each bound command's help as the MCP resource
  `kbase://help/<command>` (`text/plain`).
  - **Body.** Exactly what `kbase <command> --help` prints, from the same cobra rendering.
  - **Discovery.** The MCP instructions name the resource in one sentence.
- **Families.** S5 binds two family tools, split by read versus write.
  - **`query`** is read-only. Its operations are the Queries group plus `render-citation`:
    `cited-by`, `deps`, `edges`, `find`, `gated-on`, `referenced-by`, `render-citation`, `show`,
    `stats`, `strength` and `subtree`.
  - **`write`** is not read-only. Its operations are `add-pairs`, `add-rests-on`, `drop-claim-edges`,
    `drop-claim-node`, `drop-inbound-edges`, the five `insert-*-entry`, `mark-claim-in-leaf`,
    `resolve-demoted` and the four `set-*`.
  - **Schema.** `operation` is required and is a string enum of the members. `arguments` is an
    object holding what the operation's tool takes today:
    - for a query, its positionals and flags by name;
    - for `render-citation` and every write op, its one values entry's keys.

    `project-dir` is present as on every tool. `query` therefore takes two argument shapes, chosen by
    the operation.
  - **Argument check.** kbase checks `arguments` against the operation's own derived schema:
    `mcp.CommandSchema` for a flag-and-positional query, `write.OpVocabulary` for `render-citation`
    and every write op.
    - An unknown key, a missing required key or a wrong type is a refusal document: `check: usage`,
      `key` the argument's name, `allowed` the operation's argument names.
    - An `operation` outside the enum stays a schema failure (`-32602`).
  - **Description.** A family's tool description is one sentence plus two pointers: to
    `kbase://help/query` (or `kbase://help/write`) for the family, and to `kbase://help/<command>`
    for each operation.
  - **Everything else is unchanged.** Every other bound command keeps its own tool. No command is
    gated on `kbasedev`, and none is made CLI-only.
- **Shared help is stated once.** `queryHelp` and `listQueryHelp` (`cmd/query.go:113`, `:115`) and
  `valuesHelp` (`cmd/write_op.go:139`) stop being appended to each member's `Long`.
  - **Where they are stated.** Over MCP:
    - `kbase://help/query` states `queryHelp` and `listQueryHelp`, and `valuesHelp` for
      `render-citation`, its one values operation;
    - `kbase://help/write` states `valuesHelp`.

    On the CLI: once each, in `kbase --help`.
  - **One source.** Those three constants are the single source that both family resources and the
    root help draw from. No second copy of their text exists.
- **The `strength` query.**
  - **Selection.** It lists claims with `--min` (inclusive), `--max` (strictly below) and
    `--min-dependents` (default 0).
  - **Order.** `--sort strength` is the default: ascending strength, then id. `--sort dependents`
    orders by most dependents, then lowest strength, then id.
  - **`--pending`.** It lists the claims with no strength yet. It refuses beside `--min` or `--max`
    (usage, naming `--pending`). `--min-dependents` filters it as usual. Under `--pending`,
    `--sort strength` orders by id, and `--sort dependents` by most dependents, then id.
  - **Rows.** One row shape: the full claim record, as `strength-below`'s rows carry it, plus
    `dependents`. A pending row's `strength` is null.
  - **No default threshold and no band flag.** With no flags it lists every scored claim. It is a
    list query with `--limit`/`--offset`.
  - **Constants.** `query.DefaultMaxStrength` and `query.DefaultMinDependents` stay only where
    `stats`' `highest_leverage` uses them.
- **`stats` absorbs `edge-census`.** `stats` gains `relations` after `bearing_model` and before
  `strength_bands`: one record per relation in `kb.Relations` order.
  - **Why there.** The parity rule that checks `stats`' four flag counts "in its place after
    `subtree_aggregates`" keeps its place (`internal/kbload/parity_test.go:607`).
  - **Each record.** `edges` and `walked`. On a scoring relation, also `bearings`: one record per
    distinct bearing (`bearing`, `edges`), ordered as `edge-census` orders them today.
  - **What is dropped.** The per-bearing `manual`/`model` split is not carried. `stats` already counts
    `bearing_manual` and `bearing_model`.
- **`drop-claim-node` deletes a claim in one call.** It removes:
  - the claim's own edges of both classes;
  - every edge by which another node names it (what `drop-claim-edges` and `drop-inbound-edges` did);
  - every `demoted` record naming it;
  - its markers and its record.

  The prose refusal is unchanged from SPEC §8: a line under `kb-root/` outside the register pages, or
  a prose field of another record, that names the claim still refuses the drop.
  `drop-claim-edges` and `drop-inbound-edges` stay unchanged, as edge operations in their own right.
- **`refresh` absorbs `render-claim-graph`.** `refresh` already draws every sheet.
  - **Result keys.** On `done` or `unchanged`, `refresh`'s result gains `sheet` and `dot` after
    `removed`. `sheet` is `drawn` where `index.WriteSheet` drew, else `placeholder`. `dot` is
    `"not found on PATH"`, present only with `placeholder`.
  - **On failure.** On `refused` or `failed` neither key is present, as `render-claim-graph`'s result
    carried neither on an error.
  - **Older formats.** Drawing on a KB in an older format now saves the conversion, as `refresh`
    always has.
- **`models`, `rewind-build` and `edges` are not removed, merged or gated.** `models` and
  `rewind-build` keep their own tools. `edges`, unchanged on the CLI, is an operation of `query`.
- **The parity suite and the migration tree are permanent.** Only parity cases 107–111 go (S2), and
  with them their commands. No case is retargeted.

## Open questions

- **Stale stamped documents in existing KBs (awaiting the owner).** S2 moved the stamped templates
  off `edge-census`, S4 moves them off `render-claim-graph`, but `kbdocs.Restamp` rewrites a KB's
  `AGENTS.md` and `CONVENTIONS.md` only at a format migration, so a KB already at `3.0.0` keeps
  naming retired commands. CONVENTIONS ("A file kbase generates is regenerated whole") says a new
  template rewrites them. Readings: (A) a package R — `refresh` restamps both wherever their bytes
  differ from the current templates' render, SPEC §3/§10 moving with it; (B) leave them until the
  next format migration and narrow CONVENTIONS to "a new format". The plan runs without R; it is
  added, or CONVENTIONS narrowed, on the owner's ruling.
- **`sheet: placeholder` (awaiting the owner).** SPEC §8 defines it as "nothing was drawn, whether or
  not a sheet stands", so a model reading it beside an older full drawing may report the graph as a
  placeholder. Readings: (A) rename the value to one that says what happened (`not-drawn`), a SPEC
  §7/§8 change; (B) keep it, refresh's help now stating what it means.

## Skill candidates

Items U finds that only the kbase skill can carry. Each entry is a candidate, not an edit: the quoted
text, the adjagent file it would go in, and the owner's ruling. Empty until U reports.

## Sequencing and status

| Package | Rating | Depends on | Status |
|---|---|---|---|
| S1: measure the served surface; serve each command's help as a resource | expert | Gate | landed |
| U: retire `USER_CONVENTIONS.md` onto the served surface | expert | S1 | landed |
| P1: move environment-configured tests onto configuration directories | expert | U | landed |
| P2: remove `provider` and `[asks] readerConcurrency` | mechanical | P1 | landed |
| P3: remove the environment inference mode | expert | P2; P1's evidence green | landed |
| S2: `strength` replaces `weak-points`/`strength-below`; `stats` absorbs `edge-census` | expert | P3 | landed |
| S3: `drop-claim-node` deletes a claim in one call | expert | S2 | landed |
| S4: `refresh` absorbs `render-claim-graph` | expert | S3 | landed |
| S5: family binding (`query` reads, `write` writes); shared help stated once | expert | S4 | pending |
| Review | — | S5 | pending |

- **Gate before S1.** The working-tree edit to `ARCHITECTURE.md` §3 is committed:
  `git status --short -- ARCHITECTURE.md` prints nothing.
- **Re-run before acting.** The scoping answers below were taken at HEAD `08ac874`. Each package
  re-runs its commands when it starts. An answer naming a file outside the package's Scope goes back
  to the coordinator as a scope change; the package does not absorb it.
- **U comes first among editors.** U precedes every package that would edit `USER_CONVENTIONS.md`:
  P2, P3 and S2–S4 each name content it holds.
- **Packages run one at a time.** S2–S4 have no dependency on one another, but each edits
  `cmd/surface.go`, SPEC §7/§8, `cmd/resultkeys_test.go` and
  `tools/scripts/test-integration-results-fixture.sh`. All packages edit `justfile`.
- **Plan gate.** The prompt-engineer review is applied, and so are the owner's rulings on the open
  questions.

## Rules every package keeps

- **One landing.** Each package lands as one commit, which the coordinator makes after auditing the
  diff (CONVENTIONS "Coordinator Policy").
- **The report's "Commit body" section.** The executor's report carries a section headed "Commit
  body", which the coordinator copies into the commit. It holds:
  - the measurements;
  - U's audit table;
  - each moved fixture and the new truth it states;
  - the adjagent follow-ups.
- **The report's "Model-facing text changed" section.** The executor lists every model-facing text
  it changed, by file and symbol, under that heading, and the coordinator dispatches prompt-engineer
  review of those items before the landing. Model-facing here means:
  - cobra `Short`, `Long` and flag usages;
  - `internal/mcp/instructions.txt`;
  - any MCP resource text;
  - the stamped templates under `internal/kbdocs/templates/`;
  - the `config.toml` template text in `internal/config/template.go`.
- **Tools.** Edits use the edit and write tools. A package deletes with `command rm` and lists each
  deletion in its report; the coordinator stages it with `git rm` at landing, because another
  committer shares the tree and a staged change rides into their commit. Formatting uses
  `just fmt`.
- **Instructions fixtures.** An instructions change moves `test-data/fixtures/mcp/initialize.jsonl`
  and `gui-project-dir.jsonl`, which pin the instructions' text.
- **Measurement.** S1, U and S2–S5 each run `just measure-mcp-surface` after their change and report
  the four numbers beside the previous package's.
- **adjagent.** Lines in `../adjagent` a package makes stale are reported as adjagent follow-ups and
  are never edited here (CONVENTIONS House Rules).
- **Role-record assertions.** New or moved assertions on a role record check `provider`,
  `provider-rule` and `model`, never `back-end`.
- **Stubs.** A `providers.toml` entry standing in for a stub that streams OpenAI chat states
  `api = "openai"`. An entry silent on `api` speaks Messages (SPEC §9.1). A test pinning one dial
  for several roles references an `openai` entry: the decision role always speaks OpenAI, so a
  Messages entry is two endpoints.

---

## S1: measure the served surface; serve each command's help as a resource (expert)

**Facts**

- **What the tool list carries today.** A tool's description is `cmd.Short`
  (`internal/mcp/mcp.go:200`), and its properties' descriptions are the flag usages
  (`mcp.CommandSchema`). `Long` reaches no MCP surface.
- **Resources today.** `kbase://sheet/…`, `kbase://model-hints`, `kbase://build-trail` and
  `kbase://build` (`internal/mcp/resources.go`; SPEC §11 "Resources").
- **No measuring recipe exists** (`just --list`). The design's baseline (42 tools, `tools/list` about
  35,000 characters, schemas about 28,000, instructions 1,443) has no recorded command.
- **Bound commands.** 41 bound commands in `cmd/surface.go`, plus the listed `rewind-build`.

**Outcome**

1. **The recipe.** Add `measure-mcp-surface`, a hermetic recipe that depends on `build`. Its script,
   `tools/scripts/measure-mcp-surface.sh`, follows the recipe-script rule.
   - **Where it runs.** `./bin/kbase mcp`, from a system temp directory under no repository, so no
     project is located and no count varies with a path. The run tree is copied under
     `test-data/transient/measure-mcp-surface/`.
   - **What it sends.** `initialize`, `tools/list` and `resources/list`.
   - **What it writes.** `surface.yaml`, holding:
     - `tools`, the count;
     - `tools-list-chars`, the `tools/list` result's JSON length;
     - `input-schema-chars`, every `inputSchema`'s JSON length, summed;
     - `instructions-chars`.

     The raw frames sit beside it.
   - **Tools.** No tool outside the hermetic set. A Go test run with `-args`, as `TestResultDocument`
     is, may compute the counts.
   - **Not a gate.** It asserts only that the server answered and the counts are positive, and it
     stays out of `test-integration`.
2. **Record the baseline.** Run the recipe on the tree before any surface edit. Report its four
   numbers as the baseline.
3. **Serve the help resource.** Serve `kbase://help/<command>` for every bound command, through the
   help resource's `mcp.ResourceFamily`.
   - **Body.** The text `kbase <command> --help` prints, from the same cobra rendering, with no second
     copy.
   - **Listing.** Listed by `resources/list`, `text/plain`, KB-independent like
     `kbase://model-hints`.
4. **Point to it.** `internal/mcp/instructions.txt` gains one sentence naming
   `kbase://help/<command>` as each tool's full help.
5. **Contract.**
   - SPEC §11 "Resources" states the resource.
   - ARCHITECTURE §11's `mcp` entry names it.
   - CONVENTIONS' MCP house rule names it.
   - SPEC §12 bullet: `- **Every command's help is an MCP resource**, `kbase://help/<command>`, the
     text its `--help` prints (§11).`
6. **Tests.**
   - A transcript reads `kbase://help/status` and pins its bytes; `status`'s help changes in no
     later package.
   - `resources.jsonl` moves for the new listing.
   - The two instructions fixtures move.

**Scope**

- **New files.** `tools/scripts/measure-mcp-surface.sh` and any measuring Go test.
- **Edited files.**
  - `justfile`.
  - `internal/mcp/resources.go`, or a new file in `internal/mcp`.
  - `cmd/surface.go` (`servedServer`'s `Resources`).
  - `internal/mcp/instructions.txt`.
  - `test-data/fixtures/mcp/resources.jsonl`, `initialize.jsonl`, `gui-project-dir.jsonl`, and the
    new transcript.
  - SPEC §11/§12, ARCHITECTURE §11, CONVENTIONS ("An MCP tool is a subcommand's binding").
- **Added in execution.** `cmd/mcp_test.go` (three tests pin the resource count and the
  instructions' tokens S1 moves); `internal/mcp/mcp.go` (the bound-command filter, shared with the
  help resource); ARCHITECTURE's "MCP server (SPEC §11)" paragraph; `tools/scripts/write-evidence.sh`.
- **Not touched.** No help text, and no tool.

**Done**

- `just edit-gate`, `just test`, `just test-integration` and `just measure-mcp-surface`.
- The baseline and after numbers are in the report's "Commit body".

---

## U: retire `USER_CONVENTIONS.md` onto the served surface (expert)

**Facts**

- **The file's sections.** `grep -n '^#' USER_CONVENTIONS.md` answers `Every invocation`,
  `Correcting a refusal`, `Building`, `Monitoring, cancelling and resuming a build`,
  `Maintaining a KB`, `Providers`.
- **Who names it.** `git grep -n USER_CONVENTIONS` answers `README.md:560` and
  `ROADMAP_PLANS/CREDENTIALS.md:55`. `git -C ../adjagent grep -n USER_CONVENTIONS` prints nothing.
- **What is served today.**
  - Each command's help: `kbase://help/<command>` (S1), and `--help` on the CLI.
  - Each tool's `Short` and flag usages.
  - `internal/mcp/instructions.txt` and `kbdocs.SessionRule()`.
  - Refusal `detail` and `remedy`, and §7's result keys.
  - `kbase://model-hints`.
  - The stamped `kb-root/AGENTS.md`, `CONVENTIONS.md` and `README.md`
    (`internal/kbdocs/templates/`).

**Outcome**

1. **Audit every bullet and paragraph** of each section against the served surface. Each item gets
   one row: what it says, where an agent already gets it, or where it now goes. Placement follows
   ARCHITECTURE §3's tiers:
   - a command's own fact goes into that command's `Long` or flag usage;
   - a short rule spanning commands goes into `instructions.txt`;
   - longer guidance goes into a new MCP resource read on demand, `kbase://guide/<topic>`: embedded
     text under `internal/mcp`, listed like `kbase://model-hints`.
2. **What is not placed.**
   - Content describing the `provider` key, `readerConcurrency` or the environment mode is dropped;
     P2 and P3 remove those.
   - Content a later package rewrites (`weak-points`, `strength-below`, `edge-census`, the drop ops,
     `render-claim-graph`) is placed as it stands, in that command's help; S2–S4 carry it forward.
   - Content stale against the current SPEC is dropped, and its row says why.
3. **Skill candidates.** An item only the kbase skill can carry, chiefly when to engage kbase, is not
   placed and not edited. It is reported as a skill candidate: the quoted text and the adjagent file
   it would go in. The coordinator enters it under "Skill candidates" for the owner's ruling.
4. **Remove the file.** `git rm USER_CONVENTIONS.md`.
   - Rewrite `README.md:560` to point at the served surface: `kbase --help`, the MCP tools and their
     help resources.
   - Drop `USER_CONVENTIONS` from the row at `ROADMAP_PLANS/CREDENTIALS.md:55`.
5. **Contract.** SPEC §11 lists any new `kbase://guide/…` resource, and ARCHITECTURE §11's `mcp`
   entry names it.
6. **Report.** The audit table goes under "Commit body". Its rows are section, item, and one of: the
   destination, "already served by …", "dropped: why", or "skill candidate".

**Scope**

- **Removed.** `USER_CONVENTIONS.md`.
- **Edited.** `README.md`, `ROADMAP_PLANS/CREDENTIALS.md`, `internal/mcp/instructions.txt` and its
  two fixtures.
- **Where content lands.** The `Long` and flag usages of the commands the audit places content in.
  Any new guide resource lands in `internal/mcp`, with its test and the `resources.jsonl` listing.
- **Contract.** SPEC §11 and ARCHITECTURE §11.
- **Not touched.** Nothing under `.claude/`, and nothing in `../adjagent`.

**Done**

- `just edit-gate`, `just test`, `just test-integration` and `just measure-mcp-surface`.
- `git grep -n USER_CONVENTIONS -- ':!ACTIVE_PLAN.md' ':!ACTIVE_DESIGN.md'` prints nothing.

---

## P1: move environment-configured tests onto configuration directories (expert)

The environment mode stays in place until this package's evidence is green. P1 changes no production
code.

**Facts** (HEAD `08ac874`)

- **Tests that set an inference variable.**
  `git grep -l -E 'Setenv\(config\.Env(APIBaseURL|Decision|Text|Scoring|APIKeyFile)' -- '*.go'`
  answers `cmd/build_env_test.go cmd/mcp_test.go cmd/score_scoring_test.go cmd/score_tool_test.go
  cmd/show-config_test.go`.
- **Scripts that set one.**
  `git grep -l -E 'KBASE_(API_BASE_URL|API_KEY_FILE|DECISION|TEXT|SCORING)=' -- '*.sh'` answers
  `tools/scripts/test-integration-build-live-arxiv.sh tools/scripts/test-integration-config.sh
  tools/scripts/test-integration-results-fixture.sh`.
- **The child directory already exists.** In `cmd/mcp_test.go`, `isolateChildBuild` sets
  `"KBASE_CONFIG_DIR": filepath.Join(base, "config")`.
  `git grep -l -E 'stalledBuildRepos|isolateChildBuild' -- '*.go'` answers
  `cmd/mcp_test.go cmd/score_tool_test.go`.
- **The results-fixture leg to replace.** The environment-mode score leg in
  `tools/scripts/test-integration-results-fixture.sh` is the block opening
  `# Environment mode with no configuration directory at all`.
- **`just test-integration` runs it.** `test-integration-results-fixture` is in `test-integration`'s
  dependencies (`justfile:294`).

**Each use and what happens to it**

| Use | What it configures | P1 | P3 |
|---|---|---|---|
| `cmd/mcp_test.go` `stalledBuildRepos` | the detached build's endpoint | writes `providers.toml` (one entry on the stub's URL, `api = "openai"`) and `config.toml` (decision and text references) into `filepath.Join(base, "config")`; sets no inference variable | — |
| `cmd/score_tool_test.go` `scoreToolProvider` | the detached score run's endpoint | the same with a scoring reference, into the directory the child reads | drops its `unsetInferenceEnv` call |
| `cmd/build_env_test.go` | tests the mode, and pins three file-mode facts: two roles naming one endpoint dial it once; the entry's key file is read; a key bound for non-loopback `http://` is one stderr line naming no key | a test in `cmd/build_test.go` pins those three through one entry both roles reference, with `apiKeyFile` and a non-loopback `http://` `baseUrl`, each role record naming the provider | `git rm` |
| `cmd/score_scoring_test.go` `TestScoreConfiguredByEnvironmentAlone` | tests the mode, and pins that the run names the scoring model and never the text model, in the document and the ledger | pins that in file mode, with no `--scoring-model` and distinct text and scoring models, checking stdout and `ledger.jsonl`; extending an existing file-mode test is acceptable | deletes the test |
| `cmd/show-config_test.go` environment tests | tests the mode | — | deletes them |
| results-fixture environment-mode score leg | tests the mode, and pins the text-model property and the resume line | adds the file-mode leg below | deletes the leg |
| `test-integration-config.sh` environment legs | tests the mode | — | deletes them |
| live-arxiv `kbase-build-env` leg | tests the mode | — | deletes it |
| live-arxiv `kbase-score-env` leg (LIVE) | the score run's endpoint | moves to `--config-dir`: a scratch directory under the leg's output holding the resolved layer's `providers.toml` (and `api_keys/` where it stands) and a `config.toml` with three references | — |

**The new results-fixture leg.**

- **Configuration.** Under the existing `score_config` pattern, a `config.toml` names distinct `text`
  and `scoring` models on the unreachable entry.
- **The run.** `score --pass rigor --limit 1 --config-dir <dir> --state-dir <dir>`, outcome
  `bounded`.
- **Checks.**
  - The `scoring` role record names the entry and the scoring model.
  - The ledger's lines name the scoring model.
  - The text model's id appears nowhere on stdout, `resume` included, and nowhere in
    `ledger.jsonl`.
  - `resume` carries `--config-dir` written absolute. SPEC §9: "a printed `resume` writes
    `--config-dir` absolute".
  - The document is parsed by `TestResultDocument`.
- **Evidence.** It writes `checks.tsv` and `docs/` entries in the script's existing pattern.

**The moved LIVE leg.**

- **What replaces `back-end: "environment"`.** The leg checks `provider: "${provider}"`,
  `provider-rule: "config"` and `model: "${scoring}"` in the scoring role record.
- **What it keeps.** Outcome `done`, one item asked, `${scoring}` in the ledger, and `${text}`
  nowhere, `resume` included.

**Scope**

- **Tests.** `cmd/mcp_test.go`, `cmd/score_tool_test.go`, `cmd/build_test.go`,
  `cmd/score_scoring_test.go`.
- **Scripts.** `tools/scripts/test-integration-results-fixture.sh`,
  `tools/scripts/test-integration-build-live-arxiv.sh`.
- **`justfile` `[doc]` strings.** `test-integration-results-fixture` describes the new leg.
  `test-integration-build-live-arxiv` says the score run is configured by `--config-dir`.
- **Not touched.** No production file and no contract document.

**Done**

- `just edit-gate`, `just test` and `just test-integration`. The MCP tests skip without pandoc; run
  them where it is usable.
- **LIVE leg.** `bash -n tools/scripts/test-integration-build-live-arxiv.sh` and
  `just --dry-run test-integration-build-live-arxiv`. The rendered line sets every variable the script
  requires.
- `git grep -n -E 'Setenv\(config\.Env' -- cmd/mcp_test.go cmd/score_tool_test.go` prints nothing.

---

## P2: remove `provider` and `[asks] readerConcurrency` (mechanical)

This is a pure removal: the keys do nothing today.

**Facts** (HEAD `08ac874`)

- **No reader-facing example states the key.**
  `git grep -n -E '^\s*provider\s*=' -- '*.md' '*.toml'` prints nothing.
  `git -C ../adjagent grep -n -E 'readerConcurrency|^provider *='` prints nothing.
- **"Next major" wording.** `git grep -n -i 'next major'` answers `SPEC.md:43`, `:1381`, `:1460`,
  `:1471`, `:1493`, `internal/config/config.go:39` and `ROADMAP.md:44`. Line 43 (pandoc's range) and
  `ROADMAP.md:44` (the metadata format) are not this removal's and stay.
- **Field users.** `git grep -l -E 'ReaderConcurrency' -- '*.go'` answers `cmd/askconcurrency_test.go
  cmd/build.go cmd/provider.go cmd/score.go internal/config/config.go internal/config/config_test.go
  internal/config/layer.go internal/config/layer_test.go internal/config/template_test.go
  internal/config/write_test.go`.
- **`Provider` users.** `git grep -l -E 'Config\{Provider|cfg\.Provider' -- '*.go'` answers
  `cmd/configure_test.go cmd/models_test.go cmd/provider.go internal/config/config_test.go
  internal/config/configure_test.go internal/config/inference_test.go internal/config/layer.go
  internal/config/layer_test.go internal/config/report_test.go internal/config/template_test.go
  internal/config/write.go internal/config/write_test.go`.
- **Superseded-line helpers.** `git grep -l -E 'supersededProvider|supersededReaderConcurrency' -- '*.go'`
  answers `cmd/build.go cmd/configure.go cmd/models.go cmd/probe.go cmd/provider.go cmd/score.go
  cmd/show-config.go`.
- **`keyProvider`.** `git grep -l -E 'keyProvider' -- '*.go'` answers
  `internal/config/template.go internal/config/write.go`.
- **The template-test exception.** `git grep -l -E 'modelledNotWritten' -- '*.go'` answers
  `internal/config/template_test.go`.
- **TOML that states the keys.** `git grep -l -E 'provider *= *\\?"|readerConcurrency' -- '*.go' '*.sh'`
  adds `cmd/show-config_test.go`, `tools/scripts/test-integration-config.sh` and
  `tools/scripts/test-integration-build-live-arxiv.sh`. The last matches only the shell variable
  `provider="$(…)"`.
- **Messages naming the superseded keys.**
  `git grep -l -E 'superseded provider|superseded \[asks\]|superseded .provider' -- '*.go' '*.sh' justfile`
  answers `cmd/askconcurrency_test.go cmd/configure_test.go cmd/models_test.go cmd/provider.go
  cmd/show-config_test.go internal/config/config_test.go internal/config/configure.go
  internal/config/configure_test.go internal/config/inference_test.go internal/config/write_test.go
  justfile tools/scripts/test-integration-config.sh`.
- **`just test-integration` covers the config recipe.** `test-integration-config` is in its
  dependencies (`justfile:294`).

**Scope**

- **Production.** `internal/config/config.go`, `layer.go`, `write.go`, `template.go`,
  `configure.go`; `cmd/provider.go`, `build.go`, `configure.go`, `models.go`, `probe.go`, `score.go`,
  `show-config.go`.
- **Tests.** `internal/config/config_test.go`, `configure_test.go`, `layer_test.go`,
  `inference_test.go`, `report_test.go`, `template_test.go`, `write_test.go`;
  `cmd/askconcurrency_test.go`, `configure_test.go`, `models_test.go`, `show-config_test.go`.
- **Other.** `tools/scripts/test-integration-config.sh`, `justfile`, `SPEC.md`, `README.md`.

**Edits: SPEC.md**

1. **`:1312`–`1313`.** Before: `` The rule is `--provider`, else the sole `providers.toml` entry;
   `config.toml`'s `provider` key is superseded and takes no part in it. `` After:
   `` The rule is `--provider`, else the sole `providers.toml` entry. ``
2. **`:1378`–`1382`.** Delete the paragraph opening `**The `provider` key is superseded.**`.
3. **`:1459`–`1461`.** Delete the `provider = "..."` line, its continuation line and the blank line
   after them.
4. **`:1471`.** Delete the `readerConcurrency …` line.
5. **`:1482`–`1483`.** End the sentence at `naming `models.<role>` and the form`. Delete `— never
   resolved through the superseded `provider` key, even where that key names the entry that would
   work`.
6. **`:1491`–`1493`.** Delete the sentence opening `` `readerConcurrency` is superseded ``. Replace
   `` There is no `[dev]` table; an old `[dev]` key fails strict load (§9.3). `` with `` There is no
   `[dev]` table, top-level `provider` key or `[asks] readerConcurrency`: a file stating any of them
   fails strict load (§9.3) naming the key, and the remedy is a hand edit. ``
7. **`:1534`.** Delete `, and no `provider` key`.
8. **`:1535`–`1536`.** Before: `` Otherwise **only** the four keys it writes are touched: the
   `[models]` table's `decision`, `text` and `scoring`, and — its one deletion — a top-level
   `provider` line, which is removed. `` After: `` Otherwise **only** the three keys it writes are
   touched: the `[models]` table's `decision`, `text` and `scoring`. ``
9. **`:1545`.** Delete ` — the deletion included`.
10. **§12.** Add: `` - **`provider` and `[asks] readerConcurrency` leave `config.toml`.** A file
    still stating either fails strict load naming the key (§9.3); delete the line by hand —
    `kbase configure` cannot load the file to repair it. ``

**Edits: README.md `:489`–`491`**

- Before: `` Its top-level `provider` key is **superseded**: it still loads and nothing chooses by it,
  and a file-mode run says so once on stderr; `kbase configure` removes it. ``
- After: `` A `config.toml` stating a top-level `provider` key or `[asks] readerConcurrency` fails to
  load, naming the key; delete the line. ``

**Edits: help and template text**

- **`cmd/configure.go:213`.** Delete `a superseded top-level provider key is removed, `.
- **`cmd/models.go:88`–`89`.** Before: `config.toml names no entry: its provider key is superseded,
  and each role names its own entry in its own model reference.` After: `config.toml names no entry:
  each role names its own entry in its own model reference.`
- **`internal/config/template.go:16`.** This is the `config.toml` template's text. Before:
  ``# [` + tableModels + `] values and removes a superseded ` + "`" + keyProvider + "`" + ` key, and leaves``
  After: ``# [` + tableModels + `] values and leaves``

**Edits: production code**

- **`internal/config/config.go`.** Delete `Config.Provider` and `AsksConfig.ReaderConcurrency`, with
  their comments. The comment at `:39` is one of them.
- **`internal/config/layer.go` `takeConfig`.** Delete the two merges and the readerConcurrency
  sentence of its comment.
- **`internal/config/write.go`.**
  - Delete `keyProvider`, the `providerAt` tracking and its line removal, and
    `delete(want, keyProvider)`.
  - The verification error becomes `the update would change more than the [models] values — this
    layout is beyond the line editor; set them by hand`.
  - Drop the provider sentences from `UpdateConfig`'s comment and the constant block's comments.
- **`internal/config/configure.go`.** Drop the superseded-provider clauses at the comments that
  `git grep -n 'superseded provider' -- internal/config/configure.go` names.
- **`cmd/provider.go`.** Delete `supersededProvider` and `supersededReaderConcurrency`.
- **The call sites.** Delete every call to those two in `cmd/build.go`, `cmd/configure.go`,
  `cmd/models.go`, `cmd/probe.go`, `cmd/score.go` and `cmd/show-config.go`, with comments that exist
  only to explain the calls.

**Edits: tests**

- **Delete what claims only the removed keys.**
  - In `internal/config/config_test.go`: `TestLoadConfigReaderConcurrency`, and the `cfg.Provider`
    assertion of the first load test.
  - In `cmd/askconcurrency_test.go`: the `readerConcurrency` warning test.
  - In `internal/config/configure_test.go`: the configure-removes-`provider` assertions.
  - In `internal/config/write_test.go`: the provider-deletion cases.
  - In `internal/config/layer_test.go`: the `provider`/`readerConcurrency` merge assertions.
  - In `internal/config/inference_test.go`: the cases whose only point is that `Config.Provider`
    selects nothing.
  - In `cmd/models_test.go`: the `Config{Provider: …}` parts. The flag and pool cases stay.
  - In `cmd/configure_test.go`, `cmd/models_test.go` and `cmd/show-config_test.go`: the
    superseded-line stderr assertions.
- **TOML text that must still load.** Delete filler `provider = …` and `readerConcurrency = …` lines.
  Where the text would be empty, use `[models]`.
- **`internal/config/write_test.go`.** The preservation of an `[asks]` key the editor does not own
  stays, with `concurrency = 1` in place of `readerConcurrency = 1`.
- **`internal/config/template_test.go`.** Delete `modelledNotWritten` and its two uses. Drop
  `Provider` and `Asks.ReaderConcurrency` from the zero-value assertion.
- **`internal/config/config_test.go` `TestLoadConfigUnknownKey`.** Add two rows: `provider = "x"`
  wanting `"provider"`, and `[asks]\nreaderConcurrency = 2` wanting `"asks.readerConcurrency"`.
- **`cmd/show-config_test.go` `configFixture` (`:20`–`39`).** Drop `provider = \"file\"\n\n` from its
  `config.toml`, and "the superseded provider key beside them" from its comment.
- **`cmd/show-config_test.go` `TestShowConfigRefusesAConfigurationThatDoesNotLoad` (`:423`).** It
  keeps its body and also asserts that the item's detail names `"provider"` and `"models.heavy"`.

**Edits: scripts and justfile**

- **`tools/scripts/test-integration-config.sh`.**
  - Drop the `'provider = "…"' ''` items from every `printf` staging a `config.toml`. The leg near
    `kept note` keeps its bare ids.
  - Delete the two `superseded provider key` checks and their comments.
  - Delete the phrase from `INVOCATION`.
- **`justfile` `test-integration-config` `[doc]`.** Remove `the superseded provider key said once, `.

**Done**

- `just edit-gate`, `just test` and `just test-integration`.
- `git grep -n -E 'readerConcurrency|ReaderConcurrency|supersededProvider|keyProvider'` prints only
  SPEC.md §9.2's strict-load sentence and §12's bullet, README.md's load sentence, and the new
  `TestLoadConfigUnknownKey` row.
- `git grep -n -i 'next major'` prints only `SPEC.md:43` and `ROADMAP.md:44`.

---

## P3: remove the environment inference mode (expert)

**Precondition.** P1 has landed, and `just test-integration` passed on the tree P3 starts from.

**Facts** (HEAD `08ac874`)

- **No outside consumer.**
  - `git -C ../adjagent grep -n -E 'KBASE_(API_BASE_URL|API_KEY_FILE|DECISION|TEXT|SCORING)'` prints
    nothing.
  - `git -C ../personant grep -n -E 'KBASE_(API_BASE_URL|API_KEY_FILE|DECISION|TEXT|SCORING)|readerConcurrency'`
    prints nothing.
- **Mode symbols.**
  `git grep -l -E 'ModeOf|ModeEnvironment|ModeFile|config\.Mode[^a-zA-Z]|Mode Mode' -- '*.go'`
  answers `cmd/askconcurrency_test.go cmd/build.go cmd/probe.go cmd/provider.go cmd/score.go
  cmd/score_record_test.go cmd/show-config.go cmd/show-config_test.go internal/config/expand_test.go
  internal/config/inference_test.go internal/config/resolve.go internal/modelclient/modelclient_test.go`.
- **Environment symbols.**
  `git grep -l -E 'Overrides|EnvAPIBaseURL|EnvDecision|EnvText|EnvScoring|EnvAPIKeyFile' -- '*.go'`
  answers `cmd/askconcurrency_test.go cmd/build.go cmd/build_env_test.go cmd/build_replay_test.go
  cmd/build_test.go cmd/configure.go cmd/mcp_test.go cmd/models.go cmd/probe.go cmd/probe_test.go
  cmd/provider.go cmd/score.go cmd/score_record_test.go cmd/score_replay.go cmd/score_scoring_test.go
  cmd/score_tool_test.go cmd/show-config.go cmd/show-config_test.go internal/config/expand_test.go
  internal/config/inference.go internal/config/inference_test.go internal/config/report.go
  internal/config/report_test.go internal/config/resolve.go internal/modelclient/modelclient.go
  internal/modelclient/modelclient_test.go`.
  - `cmd/probe_test.go` matches only through `TestProbeModelOverridesTheRole`.
  - `cmd/mcp_test.go` and `cmd/score_tool_test.go` drop out after P1.
- **`Resolve`/`ResolveFlag` callers.**
  `git grep -l -E 'ResolveFlag\(|config\.Resolve\(|[^a-zA-Z.]Resolve\(in' -- '*.go'` answers
  `cmd/build.go cmd/probe.go cmd/score.go cmd/show-config.go internal/config/expand_test.go
  internal/config/inference_test.go internal/config/resolve.go`.
- **`Endpoint.Mode` users.** `git grep -l -E 'ep\.Mode|\.Mode ==|Mode: +(config\.)?Mode' -- '*.go'`
  answers `cmd/build.go cmd/probe.go cmd/provider.go cmd/score.go cmd/score_replay.go
  cmd/show-config.go internal/config/inference_test.go internal/config/resolve.go
  internal/modelclient/modelclient_test.go`.
- **`APIKeyFile` users.** `git grep -l -E 'APIKeyFile' -- '*.go'` answers `cmd/build_env_test.go
  cmd/build_test.go cmd/score.go cmd/score_record_test.go cmd/show-config.go cmd/show-config_test.go
  internal/config/expand_test.go internal/config/inference.go internal/config/inference_test.go
  internal/config/layer_test.go internal/config/providers.go internal/config/report.go
  internal/config/report_test.go internal/config/resolve.go internal/modelclient/modelclient.go
  internal/modelclient/modelclient_test.go`. `Endpoint.APIKeyFile` goes; `Provider.APIKeyFile`, the
  entry's `apiKeyFile`, stays. The coder sorts each hit.
- **`Inputs.Env` users.**
  `git grep -l -E 'in\.Env|\.Env\.|Env: +(env|config|Overrides)|\.inputs\(' -- '*.go'` answers
  `cmd/build.go cmd/configure.go cmd/models.go cmd/probe.go cmd/provider.go cmd/score.go
  cmd/show-config.go internal/config/expand_test.go internal/config/inference_test.go
  internal/config/resolve.go`.
- **Registry methods.**
  - `git grep -l -E 'reg\.Preflight|Registry\) Preflight' -- '*.go'` answers `cmd/show-config.go
    internal/modelclient/modelclient.go internal/modelclient/modelclient_test.go`.
  - `git grep -l -E 'Registry\) Key\(|reg\.Key\(' -- '*.go'` answers `cmd/provider.go
    internal/modelclient/modelclient.go`.
- **`config.Report` users.**
  - `git grep -l -E 'config\.Report\(|func Report\(|[^a-zA-Z.]Report\(cfg|[^a-zA-Z.]Report\(tc' -- '*.go'`
    answers `cmd/show-config.go internal/config/layer_test.go internal/config/report.go
    internal/config/report_test.go`.
  - `git grep -l -E 'facts\.Environment|f\.Environment' -- '*.go'` answers `cmd/show-config.go
    internal/config/report.go`.
- **cmd helpers.**
  - `git grep -l -E 'scoreResume|environmentAssignments' -- '*.go'` answers
    `cmd/askconcurrency_test.go cmd/score.go cmd/score_record_test.go`.
  - `git grep -l -E 'buildProvider\(' -- '*.go'` answers `cmd/askconcurrency_test.go cmd/build.go
    cmd/build_env_test.go cmd/build_replay_test.go cmd/build_test.go`.
  - `git grep -l -E 'scoringRole\(' -- '*.go'` answers `cmd/score.go cmd/score_replay.go`; the second
    is `//go:build kbasedev`.
  - `git grep -l -E 'modeRecord|resolvedRoles' -- '*.go'` answers `cmd/show-config.go`.
  - `git grep -l -E 'backEndName|BackEndProvider|config\.BackEnd' -- '*.go'` answers
    `cmd/provider.go cmd/score.go cmd/score_replay.go internal/config/resolve.go`.
- **`unsetInferenceEnv`.** `git grep -l -E 'unsetInferenceEnv' -- '*.go'` answers `cmd/probe_test.go
  cmd/score_scoring_test.go cmd/score_test.go cmd/score_tool_test.go cmd/show-config_test.go`. After
  P1 it also answers `cmd/build_test.go` and `cmd/mcp_test.go` (both already in P3's scope).
  `TestScoreDocumentsARun` (`cmd/score_test.go`) does not blank the inference variables though
  `runScore` reads them; P3's removal ends that exposure.
- **`FileReference`.** `git grep -l -E 'FileReference' -- '*.go'` answers
  `internal/config/configure.go internal/config/inference_test.go internal/config/resolve.go`. Its doc
  reads "so the mode a value came from is stated where it is read".
- **`back-end` users.** `git grep -l -E '"back-end"|back-end' -- '*.go' '*.sh'` answers
  `cmd/build_env_test.go cmd/build_test.go cmd/provider.go cmd/resultkeys_test.go
  cmd/score_record_test.go cmd/score_scoring_test.go cmd/show-config.go cmd/show-config_test.go
  internal/build/inference_test.go tools/scripts/test-integration-build-live-arxiv.sh
  tools/scripts/test-integration-config.sh tools/scripts/test-integration-results-fixture.sh`.
- **Two stray sites.** `cmd/readpolicy_test.go:52`:
  `"internal/modelclient/modelclient.go": "the key file $KBASE_API_KEY_FILE names, read at the dial"`.
  `internal/build/build_test.go:887`: `Detail: "KBASE_API_BASE_URL is unset"`.
- **Docs.** `git grep -l -E 'KBASE_(API_BASE_URL|API_KEY_FILE|DECISION|TEXT|SCORING)' -- '*.md' justfile`
  answers `ARCHITECTURE.md CONVENTIONS.md README.md ROADMAP_PLANS/CREDENTIALS.md SPEC.md justfile`
  after U.

**Scope**

- **Production.**
  - `internal/config/inference.go`, `resolve.go`, `report.go`, `configure.go`.
  - `internal/modelclient/modelclient.go`.
  - `cmd/provider.go`, `build.go`, `score.go`, `score_replay.go`, `probe.go`, `show-config.go`,
    `configure.go`, `models.go`.
  - `internal/score/ledger.go`, `invoke.go`, `replay.go` (comments only).
- **Tests.**
  - `internal/config/inference_test.go`, `expand_test.go`, `report_test.go`, `layer_test.go`.
  - `internal/modelclient/modelclient_test.go`.
  - `cmd/build_env_test.go` (removed), `build_test.go`, `build_replay_test.go`,
    `askconcurrency_test.go`, `probe_test.go`, `score_test.go`, `score_record_test.go`,
    `score_scoring_test.go`, `score_tool_test.go`, `show-config_test.go`, `resultkeys_test.go`,
    `readpolicy_test.go`.
  - `internal/build/build_test.go`, `internal/build/inference_test.go`.
- **Scripts.** `tools/scripts/test-integration-config.sh`,
  `tools/scripts/test-integration-results-fixture.sh`,
  `tools/scripts/test-integration-build-live-arxiv.sh`.
- **Docs.** `justfile`, `SPEC.md`, `ARCHITECTURE.md`, `CONVENTIONS.md`, `README.md`,
  `ROADMAP_PLANS/CREDENTIALS.md`.

**Contract changes**

*SPEC.md:*

- **§7, tool-results table.**
  - `probe`: `(as `models`'s; absent where the environment supplies the endpoint)` becomes
    `(as `models`'s)`.
  - `show-config`: delete `environment` and `mode`, and delete "or the mode is undecided" from
    `diagnostics`.
- **§7, role record.** Delete the `back-end` row. The paragraph after the table says that `provider`,
  `provider-rule` and `provider-layer` stand together on every role that resolved. `provider-layer`
  keeps `environment`, the `$KBASE_CONFIG_DIR` layer.
- **§8 `score`.**
  - "The scoring role": `models.scoring` alone. Delete the environment-mode sentences and the
    environment use of `--scoring-model`.
  - Ledger: `backend` is `provider`, and a header or line written under an earlier release may read
    `environment`.
  - Refusals: "no scoring model in the mode's source, a mode whose sources are mixed or whose
    environment is partial" becomes "no scoring model configured".
  - `resume`: delete the environment-mode assignments clause.
  - `--dry-run`: "resolves no role and runs no mode decision" becomes "resolves no role".
- **§8 `probe`.** Delete every environment alternative. A role flag asks `models.<role>`, and a
  refusal names `models.<role>` or the flag.
- **§9.**
  - Intro: delete "The environment is not a layer: …".
  - The `~` paragraph: drop `$KBASE_API_KEY_FILE`.
  - Replace "The two modes" (table included), "`KBASE_API_KEY_FILE` is not a mode" and "The
    preflight" with one short statement:
    - inference comes from `providers.toml` and `config.toml` alone, `[asks]` and `[sampling]`
      included;
    - no environment variable supplies an endpoint, a model or a key;
    - an isolated run names its directory with `--config-dir` or `$KBASE_CONFIG_DIR`;
    - the preflight (§5) names a lacking setting as `baseUrl` or `models.<role>`;
    - `show-config` reports that refusal as a diagnostic (§9.6).
- **§9.2.** The `scoring` comment's "required for score, in every mode" becomes "required for score".
  Delete "In environment mode a model is a bare id … never read as the other."
- **§9.6.** Delete:
  - "a mixed or partial inference mode";
  - the key-file preflight sentence (a key file that cannot be read faults its entry at load, §9.1);
  - "the inference variable names this process sets …; the mode, or null where the sources are
    mixed";
  - "`environment` for one a `KBASE_*` variable supplied";
  - the per-side-of-a-mixed-mode diagnostics;
  - "an inference variable by its name, never its value".
- **§12.** Add: `` - **The environment inference mode is gone.** `KBASE_API_BASE_URL`,
  `KBASE_API_KEY_FILE`, `KBASE_DECISION`, `KBASE_TEXT` and `KBASE_SCORING` are not read;
  `providers.toml` and `config.toml` are the one source, and a run isolated from your layers names
  its directory with `--config-dir` or `$KBASE_CONFIG_DIR` (§9). The role record carries no
  `back-end`, and `show-config` no `mode` or `environment` (§7). ``

*ARCHITECTURE.md:*

- **§7, inference text.**
  - Delete "in both inference modes" and "or in the environment by `KBASE_<ROLE>`".
  - The probe item resolves "through `config.ResolveFlag`". Delete `config.ModeOf` and "an explicit
    reference resolved in file mode because a reference is a pool reference".
  - Delete "(environment mode is `openai`)".
- **§7, resolution item.** It becomes "Resolution is pure, and connection is the registry's":
  - `config.Resolve` / `config.ResolveFlag` and `config.Select`;
  - no `ModeOf` and no key-file sentence;
  - the registry keyed "by the entry's name, by the protocol and by the resolved sampling";
  - `config.Report` taking the existence probe alone.
- **§11 `internal/config`.** "the three roles, the two modes, `ModeOf`, `Resolve`, `Select`" becomes
  "the three roles, `Resolve`, `Select`".
- **§12 "Model roles".** Delete the `KBASE_DECISION`/… clause.

*CONVENTIONS.md:*

- **"Module Structure", `internal/config`.**
  - "`ModeOf`, `Resolve`, `Select`" becomes "`Resolve`, `Select`".
  - Delete "the key file `KBASE_API_KEY_FILE` names is read by whoever dials (…)".
  - Delete "an environment-supplied id is a bare id of the environment's endpoint, which nothing
    parses."
- **`internal/model`.** Delete "in file mode" from its last sentence.

**Code outcome**

- **`internal/config`.**
  - Delete `Mode`, `ModeFile`, `ModeEnvironment`, `ModeOf`, `Overrides`, `EnvOverrides` and the five
    `Env*` constants. `EnvConfigDir` stays.
  - Delete `Inputs.Env`, `Endpoint.Mode`, `Endpoint.APIKeyFile`, `Role.variable`, `Role.fromEnv`,
    `resolveEnvironment` and the key-file override in `resolveFile`.
  - Delete `FileReference`; its callers use `ParseReference`.
  - The signatures become `Resolve(in, role, flagModel)` and
    `ResolveFlag(in, role, flagModel, flagKey, defaulted)`.
  - `KeySource` is the entry's layer or "".
  - `Report` takes no `getenv`, and `Facts.Environment` goes.
  - `CleartextWarning` and `BackEndProvider` stay; the `BackEnd` type may go.
  - Rewrite the comments that describe two modes.
- **`internal/modelclient`.** Delete `Key` and `Preflight`. `Get` dials with `ep.APIKey`, keyed by
  `ep.Provider`, with no `base:` fallback.
- **`cmd`.**
  - `providerOptions.inputs()` takes no environment.
  - `buildProvider(p, stderr)`.
  - `scoringRole(opts)` returns `(config.Endpoint, error)`.
  - `scoreResume(opts, run, dir)`; `environmentAssignments` goes.
  - `cmd/score_replay.go` follows the new signatures.
  - `probe` resolves through `ResolveFlag`.
  - `show-config` loses `Env`, its preflight call, `modeRecord`, and the `environment` and `mode`
    keys.
  - `warnCleartextKey` judges `ep.APIKey`.
  - `roleRecord` writes no `back-end`, and `backEndName` goes.
  - `cmd/score.go` sets `inv.BackEnd` from `config.BackEndProvider`, and so does
    `cmd/score_replay.go`'s `ReplayHeader.BackEnd`.
  - Help: delete the environment sentences of `cmd/build.go` and `cmd/score.go` `Long`, and the
    `, or $KBASE_*` parts of the three `probe` role-flag usages.
- **`cmd/readpolicy_test.go:52`.** Delete the allowlist entry.
- **`internal/build/build_test.go:887`.** The stub's detail names the file-mode setting.
- **`internal/score` comments.** Comments describing `BackEnd` as "a pool entry or the environment's
  endpoint" say what new runs write and that older runs may read `environment`.

**Tests and legs**

- **Delete the tests of the mode.**
  - `cmd/build_env_test.go` (`git rm`).
  - `TestScoreConfiguredByEnvironmentAlone`, and the environment sub-case of
    `TestScoreRefusesWithoutAScoringRole`.
  - `TestShowConfigReportsAnEndpointTheEnvironmentSupplies`, and the mixed-source and env-key-file
    tests in `cmd/show-config_test.go`.
  - The env case in `cmd/score_record_test.go`.
  - `TestEnvOverrides`, `TestModeOf`, `TestResolveEnvironmentMode`,
    `TestResolveSamplingInEnvironmentMode`, and the env rows and key-file override case of
    `internal/config/inference_test.go`.
  - `TestExpandHomeEnvKeyFile`. `TestExpandHomeConfigDir` stays.
  - The env rows of `internal/config/report_test.go`.
  - The env-endpoint and `KBASE_API_KEY_FILE` cases of `internal/modelclient/modelclient_test.go`.
- **`unsetInferenceEnv`.** Delete its definition in `cmd/show-config_test.go` and every call in the
  five files the grep above answers.
- **Adjust the rest without loosening an assertion.**
  - The new signatures throughout.
  - Remove `cmd/build_test.go`'s `Setenv` loop.
  - In `cmd/resultkeys_test.go`: remove `"environment"` and `"mode"` from `show-config`, and
    `"back-end"` from the three `.roles` lists.
  - Remove the `back-end` and `mode` assertions in `cmd/build_test.go`, `cmd/score_record_test.go`,
    `cmd/show-config_test.go` and `internal/build/inference_test.go`.
- **Scripts.**
  - `test-integration-config.sh`:
    - delete leg 4 (`KBASE_TEXT` over both) and leg `07b`;
    - trim every `env -u KBASE_API_BASE_URL … -u KBASE_API_KEY_FILE` scrub and the `scrub=` array to
      `KBASE_CONFIG_DIR` where present;
    - delete the `mode:` and `back-end` checks;
    - rewrite the header comment and `INVOCATION`.
  - `test-integration-results-fixture.sh`: delete the environment-mode leg, and trim its opening
    `unset` to `KBASE_CONFIG_DIR`.
  - `test-integration-build-live-arxiv.sh`:
    - delete the `kbase-build-env` leg;
    - trim `layered_env` to `-u KBASE_CONFIG_DIR "HOME=…"`;
    - delete the `environment: []` check.
- **`justfile` `[doc]` strings.**
  - `test-integration-results-fixture`: delete the environment-alone clause.
  - `test-integration-config`: delete the `KBASE_TEXT` mixed-mode clause, the environment-alone
    clause and "its mode".
  - `test-integration-build-live-arxiv`: delete the environment-build clause.

**User docs**

- **README.md.** Remove the environment mentions, or rewrite them file-only:
  - Requirements;
  - Scoring, including `backend`'s "or `environment`";
  - the `resume` sentence;
  - the `~` sentence;
  - "Two modes, never merged";
  - the `probe` paragraph;
  - `show-config`'s "the inference variables this process sets; the mode;".
- **`ROADMAP_PLANS/CREDENTIALS.md`.** "The problem" and decision 3 keep `apiKeyFile` alone.

**Done**

- `just edit-gate` (both build polarities), `just test`, `just test-integration`, and
  `just test-integration-mcp-build` where pandoc is usable.
- **LIVE leg.** `bash -n tools/scripts/test-integration-build-live-arxiv.sh` and
  `just --dry-run test-integration-build-live-arxiv`.
- **Residue.**
  - `git grep -n -E 'KBASE_(API_BASE_URL|API_KEY_FILE|DECISION|TEXT|SCORING)'` prints only SPEC.md
    §12's bullet.
  - `git grep -n -E 'ModeOf|ModeEnvironment|EnvOverrides|resolveEnvironment' -- '*.go'` prints
    nothing.
- `just checkpoint`.

---

## S2: `strength` replaces `weak-points` and `strength-below`; `stats` absorbs `edge-census` (expert)

**Facts** (HEAD `08ac874`)

- **Files naming the commands** (`USER_CONVENTIONS.md` is gone after U):
  - `git grep -l -e weak-points` answers `README.md SPEC.md cmd/format_test.go cmd/resultdocs_test.go
    cmd/resultkeys_test.go cmd/weak-points.go internal/kbload/testdata/parity-2.0.1/110.args
    internal/kbload/testdata/parity-2.0.1/111.args internal/query/query.go
    tools/scripts/test-integration-results-fixture.sh`.
  - `git grep -l -e strength-below` answers `README.md SPEC.md cmd/format_test.go
    cmd/resultdocs_test.go cmd/resultkeys_test.go cmd/strength-below.go
    internal/kbload/testdata/parity-2.0.1/107.args internal/kbload/testdata/parity-2.0.1/108.args
    internal/kbload/testdata/parity-2.0.1/109.args internal/query/query_test.go
    tools/scripts/test-integration-results-fixture.sh`.
  - `git grep -l -e edge-census` answers `README.md ROADMAP_PLANS/NEXT_SCORING_WORK_SESSION.md
    SPEC.md cmd/edge-census.go cmd/format_test.go cmd/resultdocs_test.go cmd/resultkeys_test.go
    internal/kbdocs/templates/AGENTS.tmpl.md internal/kbdocs/templates/CONVENTIONS.tmpl.md
    internal/kbdocs/templates/README.tmpl.md internal/query/payload.go internal/query/query_test.go
    justfile test-data/fixtures/mcp/edge-census.jsonl
    tools/scripts/test-integration-results-fixture.sh`.
- **Line sites.**
  - SPEC.md: `:555`, `:556`, `:592`, `:809`, `:812`, `:814`, `:817`, `:824`, `:826`, `:1943`.
  - README.md: `:223`–`224`.
  - `git grep -n -E 'weak-points|strength-below|edge-census' -- ARCHITECTURE.md CONVENTIONS.md`
    prints nothing.
- **Symbols.**
  `git grep -l -E 'DefaultMaxStrength|DefaultMinDependents|WeakPoints|StrengthBelow|EdgeCensus' -- '*.go'`
  answers `cmd/edge-census.go cmd/strength-below.go cmd/weak-points.go internal/query/payload.go
  internal/query/query.go internal/query/query_test.go`. `StatsPayload` calls
  `ix.WeakPoints(DefaultMaxStrength, DefaultMinDependents)` (`internal/query/payload.go:347`), and
  that method stays.
- **Binary-level legs.** `tools/scripts/test-integration-results-fixture.sh` runs
  `strength-below --max-strength 0.5`, `weak-points` and `edge-census` twice; the second run checks
  the demoted count. `test-data/fixtures/mcp/edge-census.jsonl` is a transcript.
- **adjagent follow-ups.** `git -C ../adjagent grep -n -E 'weak-points|strength-below|edge-census'`
  answers `adjagent/definitions/kbase.py:179`, `:214` and `:215`.
- **Parity cases by command.**
  `for f in internal/kbload/testdata/parity-2.0.1/*.args; do head -1 "$f"; done | sort | uniq -c`
  answers: 14 `cited-by`, 28 `deps`, 10 `find`, 14 `gated-on`, 14 `referenced-by`, 15 `show`,
  1 `stats`, 3 `strength-below`, 11 `subtree`, 2 `weak-points`. The `strength-below` cases are
  `107`–`109` and the `weak-points` cases `110`–`111`.
- **Nothing names, counts or reads the retired cases.**
  - The harness finds its cases by glob: `internal/kbload/parity_test.go:646`,
    `filepath.Glob(filepath.Join(parityGoldenDir, "*.args"))`. It refuses only an empty set (`:647`).
  - The recipe's script loops `for f in "${golden}"/*.args`
    (`tools/scripts/test-integration-parity-adjagent-sheet.sh:30`).
  - `git grep -n -E 'parity-2\.0\.1/(10[7-9]|11[01])'` prints nothing.
  - `grep -n -E 'weak-points|strength-below|strength' internal/kbload/parity_test.go` prints nothing.
    No rule or fact reads those cases; `gatherFacts` reads `deps` and `cited-by` cases.
- **`stats` is a parity case.** `112.args` is `stats`, and its 2.0.1 golden has no `relations`.
  - `answerRules` (`internal/kbload/parity_test.go:118`–`124`) is the existing home of rules that
    remove "from the converted KB's answer a key 3.0.0 adds, after checking where it stands and what
    it holds".
  - Its `manuallyScored` rule checks `stats`' four flag counts "in its place after
    `subtree_aggregates`" (`:607`).

**Scope**

- **New.** `cmd/strength.go`.
- **Removed.** `cmd/weak-points.go`, `cmd/strength-below.go`, `cmd/edge-census.go` and
  `test-data/fixtures/mcp/edge-census.jsonl` (`git rm`).
- **Edited.**
  - `cmd/surface.go`.
  - `internal/query/payload.go`, `query.go`, `query_test.go`.
  - `cmd/format_test.go`, `resultdocs_test.go`, `resultkeys_test.go`.
  - `test-data/fixtures/mcp/stats.jsonl`.
  - `tools/scripts/test-integration-results-fixture.sh`.
  - `justfile` (the results-fixture `[doc]`).
  - `internal/kbdocs/templates/AGENTS.tmpl.md`, `CONVENTIONS.tmpl.md`, `README.tmpl.md`.
  - `ROADMAP_PLANS/NEXT_SCORING_WORK_SESSION.md`.
  - `SPEC.md`, `README.md`.
- **Parity.**
  - `git rm` the 15 files `internal/kbload/testdata/parity-2.0.1/{107,108,109,110,111}.{args,exit,out}`.
  - Add one answer-side rule to `answerRules` in `internal/kbload/parity_test.go`, the only other
    parity edit.
  - Every other golden, the harness's other rules, the recipe and its script, and the migration
    tree under `internal/formats` stay as they are.

**Outcome**

- **`strength`.** Built as "Settled consequences" states. `query.StrengthPayload` replaces
  `StrengthBelowPayload` and `WeakPointsPayload`.
- **`stats` gains `relations`.** `EdgeCensusPayload`'s logic moves into `StatsPayload`, without the
  per-bearing manual/model split.
- **SPEC.**
  - The sites listed under Facts: `strength` in the list-query row, `stats`' `relations`,
    `edge-census` removed.
  - The §8 Queries list, with the paragraphs at `:817` and `:824`–`826` restated for `strength` and
    `stats`.
  - §12 bullet: `strength` replaces `weak-points` and `strength-below`; `edge-census` is gone, its
    counts in `stats`' `relations`.
- **README.md** `:223`–`224`.
- **Stamped templates.** The report states when an existing KB's stamped documents pick up the new
  text, with the answer of `git grep -n 'func ' -- cmd/restamp.go`.
- **`ROADMAP_PLANS/NEXT_SCORING_WORK_SESSION.md`**: its one mention.
- **Tests.**
  - `internal/query/query_test.go` covers the bounds, both sorts, `--pending` with each sort and with
    `--min-dependents`, `--pending`'s usage refusal, and `relations` with the demoted count.
  - The results-fixture legs `strength --max 0.5`,
    `strength --sort dependents --min-dependents 1 --max 0.65` and `strength --pending` replace the
    old ones. `stats` is checked for `relations` with the demoted count, in place of the
    `edge-census` checks.
  - `edge-census.jsonl` folds into `stats.jsonl`.
  - **Parity.**
    - Delete cases 107–111.
    - Add one `answerRules` entry, "stats' relations after bearing_model, as 3.0.0 adds it". It
      checks that `relations` stands directly after `bearing_model` and holds one record per relation
      in `kb.Relations` order, then removes it, as the `manuallyScored` rule removes its keys.
    - Insert it before the `manuallyScored` entry (`internal/kbload/parity_test.go:123`), because
      answerRules run in slice order (`:674`) and `manuallyScored` removes `bearing_model`
      (`:602`–`606`). Appended after it, the check fails on case 112.
    - Case 112's golden is not edited, and no golden-side rule is added.

**Done**

- `just edit-gate`, `just test`, `just test-integration` and `just measure-mcp-surface`.
  `test-integration` includes `test-integration-mcp` and `test-integration-parity-adjagent-sheet`.
- **Residue.**
  `git grep -n -E 'weak-points|strength-below|edge-census|WeakPointsPayload|StrengthBelowPayload|EdgeCensusPayload'`
  prints only SPEC §12's bullet.
- **Parity set.** `command ls internal/kbload/testdata/parity-2.0.1 | wc -l` prints `321`
  (336 before, less 15).

---

## S3: `drop-claim-node` deletes a claim in one call (expert)

`drop-claim-edges` and `drop-inbound-edges` stay, unchanged, as edge operations in their own right.
Only the deletion sequence merges into `drop-claim-node`.

**Facts** (HEAD `08ac874`)

- **The op.** `internal/write/ops.go:81` registers
  `"drop-claim-node": {edit: &recordEdit{apply: refuseStrandedClaim, finish: dropClaimNodes}}`.
- **The edge refusal.** `refuseStrandedClaim` (`internal/write/marks.go:99`) refuses every inbound
  edge, with the remedy at `:105`–`107`:
  `"run kbase drop-claim-edges and kbase drop-inbound-edges first, then re-run"`.
- **The build's culls issue the three ops.** `internal/claimgraph/elision.go:11`–`25`, `IssueCulls`,
  issues `drop-claim-edges`, `drop-inbound-edges` and `drop-claim-node`, each as one batch, from the
  `claims-culled` stage (`internal/claimgraph/cull.go:342`, results discarded).
- **Tests that pin today's behaviour.**
  - `internal/write/drop_edges_test.go`: `TestDropClaimNode` (`:143`, refusal asserted at `:166`)
    and `TestDropClaimNodeTakesItsRefutations` (`:183`, refusal asserted at `:188`) assert the edge
    refusal.
  - `internal/claimgraph/elision_fixture_test.go`: `TestIssueCullsStopsOnRefusal` (`:166`) expects
    the batches `drop-claim-edges y,x` and `drop-inbound-edges y,x`, and
    `TestIssueCullsDropsAClaimNamedOnlyByAnotherCull` (`:135`) pins the per-batch prose rule.
- **Doc sites.**
  - SPEC.md: `:761`, `:769`, `:776`, and `:783` (`run `drop-claim-edges` and `drop-inbound-edges`
    first`).
  - README.md: `:241`–`242`.
- **adjagent follow-ups.** `adjagent/definitions/kbase.py:593`–`618`, with the three-step sequence at
  `:602`.

**Scope**

- **Production.** `internal/write/marks.go`, `ops.go`, `edges.go`; `internal/claimgraph/elision.go`;
  `cmd/drop-claim-node.go`.
- **Tests.** `internal/write/drop_edges_test.go`, `internal/claimgraph/elision_fixture_test.go`.
- **Docs.** `SPEC.md`, `README.md`.

**Outcome**

- **The op.** `drop-claim-node` removes the claim's own and inbound edges in the same record edit as
  the node, all-or-nothing under one lock. A re-issue is `unchanged`.
- **Each site.**
  - Delete `refuseStrandedClaim` (`internal/write/marks.go:93`–`113`), which is wholly the edge
    check. `recordEdit.apply` is called on every entry without a nil check
    (`internal/write/records.go:166`), so the field stays: `apply` is the per-entry drop, and
    `finish` is `refuseRemainingMentions`.
  - The prose checks are `refuseRemainingMentions` (`internal/write/marks.go:130`). They stay
    unchanged, run once after every entry's drop, and a refusal still writes nothing.
  - `proseRemedy` (`internal/write/marks.go:178`) is unchanged; its remedy still names ops
    that exist.
  - `IssueCulls` issues `drop-claim-node` alone, one batch over the ids. Its doc comment says so.
    The culled claim graph is unchanged: the composed op removes what the three batches removed.
- **SPEC.** §8 `:776`–`783`: `drop-claim-node` removes every edge naming the claim and needs no prior
  call. §12 bullet to match.
- **Help.** `cmd/drop-claim-node.go` `Long` matches.
- **README** `:241`–`242`.
- **Tests.**
  - Invert `TestDropClaimNode`'s refusal at `:166`: dropping the premise another claim rests on is
    `done`, and the edge goes with it.
  - Invert `TestDropClaimNodeTakesItsRefutations` at `:188`: dropping the refuted claim is `done`,
    and the refutation goes.
  - Add that the claim's demoted records go too, and that a re-issue is `unchanged`.
  - `TestIssueCullsStopsOnRefusal` expects one `drop-claim-node y,x` batch and its refusal.
  - `TestIssueCullsDropsAClaimNamedOnlyByAnotherCull` keeps its claim through the one batch.
  - The prose-mention tests (`:241`, `:307`) stand unchanged.

**Done**

- `just edit-gate`, `just test`, `just test-integration` and `just measure-mcp-surface`.
- **Culling evidence.** The cull's evidence is `internal/claimgraph/elision_fixture_test.go`, which
  drives the real write ops. The culling stage asks a model, so no hermetic `--no-inference` build
  runs it.

---

## S4: `refresh` absorbs `render-claim-graph` (expert)

**Facts** (HEAD `08ac874`)

- **Refresh already draws.** `internal/index/refresh.go:259`:
  `written, removed, drawn, err := WriteSheet(r.src)`. `:262`: `r.NoDot = err == nil && !drawn`.
  `cmd/refresh.go:203`–`205` writes the no-dot line to stderr.
- **Files.** `git grep -l -e render-claim-graph` answers `ARCHITECTURE.md README.md ROADMAP.md
  SPEC.md cmd/format_test.go cmd/maintenance_test.go cmd/refresh.go cmd/render-claim-graph.go
  cmd/resultdocs_test.go cmd/resultkeys_test.go cmd/writelock_test.go
  internal/kbdocs/templates/AGENTS.tmpl.md internal/kbdocs/templates/README.tmpl.md
  internal/mcp/instructions.txt test-data/fixtures/mcp/gui-project-dir.jsonl
  test-data/fixtures/mcp/initialize.jsonl tools/scripts/test-integration-results-fixture.sh`.
  `git grep -l -e RenderClaimGraph -e renderClaimGraph -- '*.go'` adds `cmd/surface.go`.
- **Line sites.**
  - SPEC.md: `:40` (§2), `:72`, `:90`, `:146` (§3), `:372` (§6), `:506`, `:557` (§7), `:637`, `:830`
    (§8), `:1617`, `:1627`, `:1808` (§10), `:2329` (§11).
  - ARCHITECTURE.md: `:404`, `:540`, `:1144`, `:1664`, `:1699`.
  - README.md: `:219`, `:306`, `:335`, `:342`, `:355`, `:396`.
  - `ROADMAP.md:41`.
  - `internal/kbdocs/templates/AGENTS.tmpl.md:22`, `README.tmpl.md:47`.
- **The keys that move.** SPEC §7 `:557`: `sheet` and `dot`. Their constants `sheetPlaceholder`,
  `sheetDrawn` and `dotNotFound` are in `cmd/render-claim-graph.go`.

**Scope**

- **Code.** `cmd/refresh.go`, `internal/index/refresh.go`, `cmd/render-claim-graph.go` (`git rm`),
  `cmd/surface.go`.
- **Tests.** `cmd/format_test.go`, `maintenance_test.go`, `resultdocs_test.go`,
  `resultkeys_test.go`, `writelock_test.go`.
- **MCP.** `internal/mcp/instructions.txt`, `test-data/fixtures/mcp/initialize.jsonl`,
  `gui-project-dir.jsonl`, `resources.jsonl` (S1's help listing drops
  `kbase://help/render-claim-graph`).
- **Templates.** `internal/kbdocs/templates/AGENTS.tmpl.md`, `README.tmpl.md`.
- **Other.** `tools/scripts/test-integration-results-fixture.sh`, `justfile`.
- **Docs.** `SPEC.md`, `ARCHITECTURE.md`, `README.md`, `ROADMAP.md`.

**Outcome**

- **`cmd/refresh.go`.** On `done` or `unchanged`, append `sheet` and `dot` after `removed`, per
  "Settled consequences". `WriteSheet`'s `drawn` reaches the verb as a new field of the refresher's
  result, beside `NoDot`, which stays.
  - `git grep -l -E 'NoDot' -- '*.go'` answers `cmd/init.go cmd/refresh.go cmd/score.go
    internal/index/refresh.go internal/index/refresh_test.go internal/sheet/sheet.go
    internal/sheet/sheet_external_test.go`. Only `internal/index/refresh.go` and `cmd/refresh.go`
    change. Move the three constants into `cmd/refresh.go`.
- **Remove the command.** `git rm cmd/render-claim-graph.go`, and delete its `AddCommand` in
  `cmd/surface.go`.
- **SPEC.**
  - `:557`: delete the row, and add `sheet` and `dot` to `refresh`'s row at `:550`.
  - `:830`: delete the section.
  - `:637`: drop it from "One write lock".
  - Every other listed site names `refresh` alone, and `:146` says `refresh`'s result names the lack.
  - §12 bullet: `` - **`render-claim-graph` is gone**: `refresh` draws the sheets and reports `sheet`
    and `dot` (§7, §8). ``
- **ARCHITECTURE.**
  - `:404`, `:540`, `:1144`: drop `render-claim-graph`.
  - `:1664`: drop it from the list.
  - `:1699`: delete the clause about `render-claim-graph` on an older KB.
- **README.** Every listed site says `refresh`, and `:355`'s `sheet: placeholder` is `refresh`'s.
- **`ROADMAP.md:41`.** `render-claim-graph` output becomes `refresh`'s sheets.
- **`internal/mcp/instructions.txt`.** "refresh and render-claim-graph write the sheets" becomes
  "refresh writes the sheets". The two fixtures move.
- **Templates.**
  - `AGENTS.tmpl.md:22`: "`refresh` and `render-claim-graph` write the sheets" becomes "`refresh`
    writes the sheets".
  - `README.tmpl.md:47`: "drawn by `refresh` and `render-claim-graph`" becomes "drawn by `refresh`".
- **Tests.**
  - In `cmd/resultkeys_test.go`: delete `render-claim-graph`, and add `sheet` and `dot` to
    `refresh`'s keys.
  - Every `render-claim-graph` case in `cmd/format_test.go`, `maintenance_test.go`,
    `resultdocs_test.go` and `writelock_test.go`: move it to `refresh` where it asserts
    `sheet`/`dot` or the write lock, and delete it where a `refresh` case in the same file already
    asserts the same. The report names each case and which was done.
  - Results fixture: delete the `run render-claim-graph …` leg, and check `sheet:` on a `refresh`
    leg.

**Done**

- `just edit-gate`, `just test`, `just test-integration` and `just measure-mcp-surface`.
- `git grep -n -e render-claim-graph -e renderClaimGraph -e RenderClaimGraph` prints only SPEC §12's
  bullet.

---

## S5: family binding for the queries and the write ops; shared help stated once (expert)

**Facts** (HEAD `08ac874`; re-run after S2–S4)

- **How the table is built today.** `internal/mcp/mcp.go` `Tools` builds one tool per command
  annotated `kbase.mcp` = `bound`. A command `write.OpVocabulary` knows gets `writeOpTool`; the rest
  get `commandTool`. The description is `cmd.Short`.
- **The two groups.** `cmd/surface.go` has the `// Queries.` and `// Write ops.` `AddCommand`
  groups. `render-citation` sits in the write-ops group as `readOnlyTool(renderCitationCommand())`,
  built by `writeOpCommand` (`cmd/render-citation.go:6`), so its values vocabulary is
  `write.OpVocabulary`'s.
- **The read-only set today.** SPEC §11 "Tools": "the queries, `verify`, `status`,
  `render-citation`, `show`, `show-config` and `models` are the read-only set, and `rewind-build`
  completes it".
- **Shared help.** `git grep -n 'Long +=' -- cmd` answers `cmd/query.go:113` (`queryHelp`) and
  `:115` (`listQueryHelp`). The write ops' shared text is `valuesHelp`, appended at
  `cmd/write_op.go:139`.
- **Tests calling a member by name.**
  `git grep -h -o -E 'callTool\([a-zA-Z]+, [a-zA-Z]+, "[a-z-]+"' -- cmd internal` counts
  `insert-claim-entry` 1, `show` 1 and `stats` 1, in `cmd/mcp_test.go`.
- **Transcripts calling a member.** `find-paged.jsonl`, `frame-errors.jsonl`,
  `gui-project-dir.jsonl`, `refusal.jsonl`, `stats.jsonl` and `edges-relation.jsonl`.
- **Types and functions whose signatures may change.**
  - `git grep -l -E 'mcp\.Tools\(|func Tools\(' -- '*.go'` answers `internal/mcp/mcp.go`.
  - `git grep -l -E 'writeOpTool|commandTool' -- '*.go'` answers `internal/mcp/mcp.go`.
  - `git grep -l -E 'mcp\.Calls|Calls\{|type Calls' -- '*.go'` answers `internal/mcp/mcp.go
    internal/mcp/server.go`.
  - `git grep -l -E 'mcp\.Bind\(|mcp\.ReadOnly\(|readOnlyTool\(|readWriteTool\(' -- '*.go'` answers
    `cmd/surface.go cmd/surface_test.go`.
- **Tests reading the table.** In `cmd/mcp_test.go`: `TestMCPToolsAreTheCommandTree`,
  `TestMCPRequiredFlags`, `TestMCPWriteOpSchemas`, `TestMCPProjectDirSchemaPerMode`,
  `TestNoSurfaceNamesTheProvenanceOrCreate` and `TestMCPInitializeNamesWhatItServes`.

**Scope**

- **Code.** `internal/mcp/mcp.go`, `server.go`, `resources.go`, `instructions.txt`;
  `cmd/surface.go`, `cmd/query.go`, `cmd/write_op.go`, and the root command's `Long`
  (`cmd/main.go:54`, `rootCmd`).
- **Tests.** `cmd/mcp_test.go`, `cmd/surface_test.go`; `internal/mcp/mcp_test.go`'s
  `TestCommandSchema`, where the argument check reuses `CommandSchema`.
- **Transcripts.** The listed `test-data/fixtures/mcp/*.jsonl`, plus `initialize.jsonl` and
  `resources.jsonl`.
- **Docs.** `SPEC.md` §8, §11, §12; `ARCHITECTURE.md` I9 and §11; `CONVENTIONS.md`; `README.md`.

**Outcome**

- **Family binding**, per "Settled consequences".
  - **Declaring membership.** A command declares its family through an annotation set by a
    `cmd/surface.go` wrapper beside `readOnlyTool`/`readWriteTool`. The `query` members are the
    Queries group plus `render-citation`, and each keeps its read-only declaration. The `write`
    members are the rest of the write-ops group.
  - **A test pins the split.** Every `query` member is declared read-only, and no `write` member is.
  - **Deriving the table.** `mcp.Tools` derives one tool per family and one per other bound command.
    A family's read-only hint is its members'. Each operation stays a cobra subcommand, unchanged on
    the CLI.
  - **Dispatch.** Within a family, an operation `write.OpVocabulary` knows runs as `writeOpTool`
    runs, its `arguments` being the one entry. Every other operation runs as `commandTool` runs, its
    `arguments` being positionals and flags. `query` thus serves both shapes, `render-citation`
    through the first. The argument check and its refusal live in `internal/mcp`. They reuse
    `CommandSchema` and `write.OpVocabulary`, with no second statement of any argument list.
- **Shared help.**
  - Members' `Long` stop appending `queryHelp`, `listQueryHelp` and `valuesHelp`.
  - `kbase://help/query` serves `queryHelp`, `listQueryHelp` and, for `render-citation`,
    `valuesHelp`. `kbase://help/write` serves `valuesHelp`. Each lists its family's operations,
    rendered from the same annotations as the tool's `operation` enum, never written by hand.
  - The root command's `Long` draws on the same three constants for the CLI.
  - `kbase://help/<command>` serves each member's own help (S1).
  - **The two family resources' names** are not built from `CommandPath`: no `kbase query` or
    `kbase write` command exists, and S1 names each help resource `kbase <command> --help`.
- **Instructions.** They name the two families in their existing sentence about query and write
  tools. S1's sentence ("A tool's full help is the resource kbase://help/<tool>.") stops being true
  for a family's operations, whose help is `kbase://help/<operation>`; reword it to cover both.
- **Contract.**
  - **ARCHITECTURE I9.** A command declares its binding and, for a family member, its family. The
    table is one tool per family plus one per other bound command, plus `rewind-build`. The test
    derives its expectation from the same annotations. The rest of I9 stands.
  - **ARCHITECTURE §11 `mcp`.** The family dispatch and argument check.
  - **SPEC §8 opening.** "Every subcommand below is a builtin tool" becomes: reachable as its own tool
    or as an operation of the `query` or `write` tool.
  - **SPEC §11 "Tools".**
    - the families, their schema, and the refusal for a bad argument, replacing "-32602 … an unknown
      key" for family arguments;
    - the read-only set becomes "`query` — every query and `render-citation` among its operations — `verify`, `status`, `show-config` and `models` are the read-only set, and
      `rewind-build` completes it";
    - the equality test;
    - a write op's one-entry rule, now under `arguments`.
  - **SPEC §12 bullet.** The queries and the write ops are one tool each, an operation named from an
    enum; every subcommand stands.
  - **CONVENTIONS** "An MCP tool is a subcommand's binding (I9)": the family annotation, and how the
    tool-table test derives families.
  - **README.md**, where it describes the tool table.
- **Tests.**
  - `TestMCPToolsAreTheCommandTree` asserts the families and the single tools by annotation.
  - `TestMCPWriteOpSchemas` is restated over the argument check.
  - A new test covers the refusals and the read-only split:
    - for a flag-and-positional query, a values-shaped `render-citation` call and a write op, each
      refuses an unknown argument, a missing required one and a wrong type, naming the key and the
      allowed set;
    - the read-only split test above.
  - `TestNoSurfaceNamesTheProvenanceOrCreate` checks every operation's derived arguments.
  - The listed transcripts are rewritten to the family call shape.
  - `TestMCPInitializeNamesWhatItServes` and the instructions fixtures move with the instructions.

**Done**

- `just edit-gate`, `just test`, `just test-integration`, and `just test-integration-mcp-build` where
  pandoc is usable.
- `just measure-mcp-surface`, with S1's baseline and this final measurement side by side in "Commit
  body".
- `just checkpoint`.

---

## Post-sprint review

After S5 lands:

1. **Dispatch the reviewers.** An architect and a coder, in parallel.
   - **Pre-task reading:** THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md.
   - **Withheld:** neither receives this plan.
   - **What they review:** the sprint's changes (`git diff <commit before S1>..HEAD`) against those
     standards and for fitness to purpose, with ARCHITECTURE §3's served-surface principle and I9
     named in the brief.
   - **Tests:** the tests of every touched file, by CONVENTIONS "Testing". They name duplicate or
     performative tests to cut, and any stated outcome no test or leg exercises.
2. **Burndown.** The architect merges both reports into `ACTIVE_PLAN_REVIEW_BURNDOWN.md`, each item
   with severity, finding, location and guidance.
3. **Prompt-engineer pass.** The burndown file gets prompt-engineer review, and the architect revises
   it on the findings before dispatch.
4. **adjagent change request.** The coordinator writes `../adjagent/KBASE_SKILL_TEXT_CR.md`: every
   adjagent follow-up the packages and the review reported, each with the adjagent file and line,
   the kbase change that made it stale, and the served surface that now carries the content.
5. **Land and close.** Items land under CONVENTIONS "Coordinator Policy". The outcomes are confirmed
   in the contract documents, and this file is replaced by the next plan.
