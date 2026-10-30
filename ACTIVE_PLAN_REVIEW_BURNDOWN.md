# ACTIVE_PLAN review burndown

Post-sprint review of `02d4d33..b57f50e`, architect and coder findings merged. Items that touch the
same files are grouped together. Each item gives a rating (CONVENTIONS "Plan work rating and
dispatch"). An item's **Location** is its scope: change no file it does not name.

Rules for every item:

- **Gates.** Every item ends on `just edit-gate` and `just test`. An item's **Confirm** line names
  only the targets it needs beyond those two.
- **Commit body.** The executor's report carries a section headed `Commit body`, which the
  coordinator copies into the commit. Where an item says "name … in the commit body", that is the
  section it means. It names every fixture whose bytes moved and the truth each now states.
- **Model-facing text.** The report lists every model-facing text the item changed, by file and
  symbol: a command's `Short` or `Long`, a flag usage, an MCP description, a family's `Help`, a
  refusal's `detail` or `remedy`, `internal/mcp/instructions.txt`. The coordinator dispatches
  prompt-engineer review of that list before the commit.

---

## Served help and the family tools

### B1 — [Critical] expert — A values operation's keys are on no served surface

- **Finding**: Over MCP, a write op's or `render-citation`'s keys appear on no served surface.
  - The family tool's `arguments` is an open object.
  - `kbase://help/<op>` is the command's `--help` text (SPEC §11: "exactly the text
    `kbase <command> --help` prints"), and no write op's `Long` names its keys.
  - That help lists `--values` and `--no-refresh`, which MCP does not offer, and nothing served
    says so.
  - The argument-check refusal's remedy, "its arguments are in its full help, kbase://help/<op>",
    is false for every values operation.
  - This breaks ARCHITECTURE §3, "The served surface is the operating contract": a command's own
    facts belong in its help.
- **Resolution of the `--help` / MCP mismatch**: `kbase://help/<op>` stays byte-equal to `--help`.
  The operation's help names its keys (step 1), which is true on both surfaces. What differs over
  MCP is stated once, in `kbase://help/write`, the help every write op's tool description points
  at (step 3).
- **Location**:
  - `cmd/write_op.go` (`writeOpCommand`)
  - `internal/mcp/mcp.go`: `argumentRefusal` (~345) and `familyTool`'s `argumentsProperty`
    description (~298)
  - `cmd/surface.go` (`toolFamilies`, the `write` family's `Help`)
  - `cmd/mcp_test.go`
  - `test-data/fixtures/mcp/resources.jsonl`, `help.jsonl`, `refusal.jsonl`
  - `SPEC.md` §11
  - `ARCHITECTURE.md` §12, the "MCP server (SPEC §11)" paragraph (ARCHITECTURE.md:1705 onward)
- **Guidance**:
  1. In `writeOpCommand`, generate a closing paragraph of `Long` from `write.OpVocabulary(op)`: one
     line naming each key in vocabulary order and marking which are required.
     - Generate it from the vocabulary and never type it by hand. It is then not a prose copy under
       CONVENTIONS "Keep no prose copy of an op's key vocabulary".
     - It names keys only, never a flag, so it is true on the CLI and over MCP alike.
  2. Change `argumentRefusal`'s remedy to name both sources:
     - for the argument names, the item's `allowed`;
     - for what the operation does and its keys, its full help, `kbase://help/<op>`.
     Keep one wording for every operation.
  3. Extend the `write` family's `Help` (the text of `kbase://help/write`) with one sentence: over
     MCP, `arguments` is the entry, so an operation's values and no-refresh flags do not apply, and
     a refresh always follows. Write no `--`: `TestSharedHelpIsStatedOnce` fails family help
     carrying it.
  4. In `familyTool`, change the `arguments` description from "for a write op, its entry's keys" to
     "for a values op, its entry's keys". On `query`, the values op is `render-citation`.
  5. Add a test in `cmd/mcp_test.go`: for each command `write.OpVocabulary` knows, the text
     `kbase://help/<command>` reads contains every key name. Derive the names from
     `OpVocabulary`, never a literal list.
  6. Move `resources.jsonl`, `help.jsonl` and `refusal.jsonl` wherever their bytes change, and name
     each in the commit body.
  7. Update SPEC §11:
     - the refusal's `remedy` (today "`remedy` naming the operation's help resource") becomes
       `allowed` for names and the help resource for the rest;
     - the `write` help's content ("for `write`, how a write op takes its arguments, one entry")
       gains that the values and no-refresh flags do not apply over MCP and a refresh always
       follows.
  8. Update ARCHITECTURE §12's "MCP server (SPEC §11)" paragraph:
     - a write op's `Long` closes with the key line generated from `write.OpVocabulary`;
     - the remedy names `allowed` and the help resource.
- **Confirm**: `just test-integration-mcp`, `just measure-mcp-surface` (record the counts in the
  commit body).

### B2 — [Warning] mechanical — No test checks a help resource against the binary's `--help`

- **Finding**: SPEC §11 says `kbase://help/<command>` is exactly what `kbase <command> --help`
  prints. Only `help.jsonl` pins one command (`status`), as a hand-kept string, and nothing
  compares the resource with the binary's output.
- **Design**: The house pattern for decoding frames is a Go test fed files with `-args`, as
  `measure-mcp-surface` feeds `TestMeasureMCPSurface`. The shell extracts only URIs, which are
  plain tokens, and never decodes JSON. Python is not used: it would take the recipe out of the
  hermetic omnibus.
- **Location**:
  - `tools/scripts/test-integration-mcp.sh`
  - `tools/scripts/write-evidence.sh`
  - `justfile` (the `test-integration-mcp` recipe's `[doc]`)
  - `cmd/helpresources_test.go` (new)
- **Guidance**:
  1. After the transcript loop in the script, run `./bin/kbase mcp` from `${repo}` with
     `initialize`, `notifications/initialized` and `resources/list`. Write its frames to
     `${out}/help/list.jsonl`.
  2. Take the command names with
     `grep -o 'kbase://help/[a-z-]*' "${out}/help/list.jsonl" | sed 's|kbase://help/||'`. Drop
     `query` and `write`, which are families with no command.
  3. Run `./bin/kbase mcp` once more, sending one `resources/read` per name. Write its frames to
     `${out}/help/read.jsonl`.
  4. For each name, write `( cd "${repo}" && "${kbase}" <name> --help )` to
     `${out}/help/<name>.txt`.
  5. Run `go test -count=1 -run '^TestHelpResourcesAreTheBinarysHelp$' ./cmd -args
     -help.frames="${out}/help/read.jsonl" -help.dir="${out}/help" > "${out}/help/compare.log"
     2>&1`. A failure sets `status=1`, as a transcript failure does.
  6. In `cmd/helpresources_test.go`, add `TestHelpResourcesAreTheBinarysHelp`:
     - it skips unless both flags are set;
     - it decodes each frame's `result.contents[0]` (`uri`, `text`);
     - it requires `text` to equal `<dir>/<name>.txt` byte for byte, reporting each name that
       differs;
     - it fails where a name has no frame, or a frame no file.
  7. Add the `help/` files to `write-evidence.sh`'s section for `test-integration-mcp`, and add
     the leg to the `INVOCATION` text and the recipe's `[doc]`.
  8. Run `bash -n` on the script.
- **Confirm**: `just test-integration-mcp`.

### B3 — [Note] mechanical — `cliOnly(help)`'s reason is no longer true

- **Finding**: The exclusion reason for cobra's `help` is "a tool's schema and description already
  carry what help prints for a terminal". Help is now served as `kbase://help/<command>`, and the
  family tools' schemas carry no flag list.
- **Location**: `cmd/surface.go`, the `case "help":` line (~130).
- **Guidance**: Change the reason to state that each bound command's help is the resource
  `kbase://help/<command>`, so a tool that prints it would duplicate a resource.
- **Confirm**: the preamble's gates.

### B4 — [Note] mechanical — The query answer sentence is written twice

- **Finding**: Two constants spell "Its result is one YAML document … the answer under results",
  differing only by "on stdout": `queryHelp` (`cmd/query.go`, ~107) and the `query` family's
  `Help` (`cmd/surface.go`, ~55). ARCHITECTURE §12's "MCP server (SPEC §11)" paragraph
  (ARCHITECTURE.md:1732) says the two forms come "from the same text where the two agree".
  `TestSharedHelpIsStatedOnce` compares whole constants, so it misses this.
- **Location**: `cmd/query.go`, `cmd/surface.go`, `cmd/mcp_test.go` (`TestSharedHelpIsStatedOnce`).
- **Guidance**:
  1. Add `queryAnswerHelp(where string) string`, shaped like `listQueryHelp`. It returns
     `"Its result is one YAML document" + where + ", the answer under \"" + query.ResultsKey + "\"."`.
  2. Build `queryHelp` from `queryReads + " " + queryAnswerHelp(" on stdout")`.
  3. Build the family `Help` from `queryAnswerHelp("")`.
  4. In `TestSharedHelpIsStatedOnce`:
     - add `queryAnswerHelp(" on stdout")` to `every` (the root help's CLI list already covers it
       through `queryHelp`);
     - add `queryAnswerHelp("")` to `every` and to the `query` family's `want`.
- **Confirm**: `just test-integration-mcp`.

### B5 — [Note] mechanical — Two sources say which families exist

- **Finding**: The tool table builds families from `kbase.mcp.family` annotations, but help
  resources list `Server.Families` (`internal/mcp/help.go`, `helpResources.List`). `Tools` fails
  a member that names no `Family`. A `Family` that no bound command names still serves
  `kbase://help/<family>` for a tool that does not exist.
- **Location**: `internal/mcp/mcp.go` (`Tools`); `internal/mcp/mcp_test.go`.
- **Guidance**:
  1. In `Tools`, after building the table, return an error for each `calls.Families` entry with no
     member among `boundCommands(root)`: `"family %q: no bound command declares it"`.
  2. In `internal/mcp/mcp_test.go`, add one table test over a small cobra tree with two rows: a
     member whose family is absent from `Families`, and a `Family` with no member. Each row
     requires `Tools` to return an error.
- **Confirm**: the preamble's gates.

---

## MCP tests in `cmd/mcp_test.go` and the transcripts

### B6 — [Warning] mechanical — No run of the binary covers a successful `write` call

- **Finding**: Only `refusal.jsonl` calls `"name":"write"`, and that call is refused. The path where
  arguments fit and the member runs is exercised only in-process (`TestMCPCallRunsInItsProject`).
- **How the transcripts are staged**:
  - `TestMCPTranscripts` builds a fresh `mcpServer(t)` over newly staged state for each transcript.
  - `tools/scripts/test-integration-mcp.sh` stages one repository and feeds every `*.jsonl` to it in
    glob order, so a write in one transcript persists into every transcript after it.
  - `${ID1}` appears in `edges-relation`, `find-paged`, `gui-project-dir` and `stats`.
  - `seed.yaml` gives both claims `rigor: 0.5`, and `stats` pins `rigor_manual: 2` and
    `rigor_model: 1`.
  - So the new transcript must leave the KB as it found it, and must change a value: re-setting
    0.5 answers `unchanged`.
- **Location**: `test-data/fixtures/mcp/write-op.jsonl` (new).
- **Guidance**:
  1. Add `write-op.jsonl` with four exchanges, in order:
     - `write`, `operation: set-rigor`, `arguments: {id: ${ID1}, rigor: 0.25}`, answered `done`;
     - `query`, `operation: show`, `arguments: {id: ${ID1}}`, its `rigor` reading `0.25`;
     - `write`, `operation: set-rigor`, `arguments: {id: ${ID1}, rigor: 0.5}`, answered `done`;
     - `query`, `operation: show`, `arguments: {id: ${ID1}}`, its `rigor` reading `0.5`.
     The last write restores the seeded value and its manual flag, so no transcript run after this
     one sees a change.
  2. Take each expected response line from a run of `TestMCPTranscripts`, and check its fields
     against SPEC §7 before committing it.
  3. Name the new fixture in the commit body.
- **Confirm**: `just test-integration-mcp`.

### B7 — [Warning] mechanical — `TestMCPFamilyArgumentCheck` repeats coverage that exists elsewhere

- **Finding**: Most of this test's rows repeat coverage elsewhere.
  - The three `find` rows repeat `refusal.jsonl`'s exchange 4.
  - The `set-rigor` wrong-type row repeats `refusal.jsonl`'s exchange 1.
  - The unknown and missing rows for `set-rigor` and `render-citation` repeat
    `TestMCPWriteOpSchemas`, which checks every values operation through `operationArguments`.
  - The `set-rigor` rows hard-code `{"id","rigor"}`, a test-side copy of the vocabulary.
- **Location**: `cmd/mcp_test.go`, `TestMCPFamilyArgumentCheck` (~655–700).
- **Guidance**:
  1. Delete every row but `render-citation's wrong type`.
  2. Build that row's `allowed` from `renderCitationVocabulary(t)`, as it does now.
  3. Rename the test `TestMCPRenderCitationWrongTypeIsTheOpsRefusal`, and narrow its comment to
     that one case.
  4. Remove the `usage` field from the row struct. Where the test compared
     `(item["check"] == checkUsage) != tc.usage`, require `item["check"] != checkUsage` instead.
     Also drop `tc.usage &&` from the `len(items) != 1` condition. A wrong-typed values key is the
     op's own refusal, not the argument check.
- **Confirm**: the preamble's gates.

### B8 — [Note] mechanical — `TestMCPFamiliesSplitReadFromWrite` cannot fail

- **Finding**: `queryOperation` always sets the read-only annotation and `writeOperation` never
  does. `TestCommandsRegisterOnlyInTheSurface` confines family membership to those wrappers in
  `cmd/surface.go`, and `TestMCPToolsAreTheCommandTree` checks each tool's read-only hint.
- **Location**: `cmd/mcp_test.go`, `TestMCPFamiliesSplitReadFromWrite` (~633).
- **Guidance**: Delete the test.
- **Confirm**: the preamble's gates.

### B9 — [Note] mechanical — Three tests check the help-resource list

- **Finding**: `requireHelpResources` rebuilds `helpResources.List`'s naming and is called from
  three tests. `resources.jsonl` already pins the list's bytes.
- **Location**: `cmd/mcp_test.go`: `TestMCPModelHintsResource` (two call sites, one inside
  `listedBy`) and `TestMCPRewindBuildIsTheBuildTrail` (~1354).
- **Guidance**:
  1. Keep the first call in `TestMCPModelHintsResource`. It is the one check that the list
     follows the command tree rather than a hand-kept transcript.
  2. In `listedBy`, return `listed.Resources[:len(want)]` without calling `requireHelpResources`.
  3. In `TestMCPRewindBuildIsTheBuildTrail`, delete the call. Keep its `< 3` length guard.
- **Confirm**: the preamble's gates.

### B10 — [Note] mechanical — Binding error message omits the family wrappers

- **Finding**: `TestMCPToolsAreTheCommandTree`'s message for a command with no binding names only
  `readOnlyTool`, `readWriteTool` or `cliOnly`.
- **Location**: `cmd/mcp_test.go` (~390).
- **Guidance**: Name all five wrappers, matching `TestCommandsRegisterOnlyInTheSurface`'s message:
  `readOnlyTool`, `readWriteTool`, `queryOperation`, `writeOperation` or `cliOnly`.
- **Confirm**: the preamble's gates.

### B11 — [Note] expert — An instructions test pins prose wording

- **Finding**: `TestMCPInitializeNamesWhatItServes` requires the phrase "full help" exactly once,
  which only stops the word "help" from reading as the CLI-only `help` command. Rewording the
  instructions fails the test although no logic error would.
- **Location**: `cmd/mcp_test.go`, the `helpPhrase` block (~1739).
- **Guidance**:
  1. Delete the `helpPhrase` count and the `strings.Replace`.
  2. In the token scan, skip a token that names one of cobra's own commands. Take their names from
     the tree, from the commands `cmd/surface.go`'s final loop declares `cliOnly` (`help`,
     `completion`); never from a literal.
  3. Keep the stripping of `kbase://` words.
- **Confirm**: the preamble's gates.

---

## The usage check constant and the query package

### B12 — [Warning] mechanical — The `"usage"` check has five homes, and `query` imports `config` for one

- **Finding**: The refusal check `"usage"` is defined in five places:
  - `internal/buildstore/stages.go:55` (`CheckUsage`)
  - `internal/config/resolve.go:320` (`CheckUsage`)
  - `cmd/query.go:96` (`checkUsage`)
  - `internal/score/ledger.go:79` (`checkUsage`)
  - `internal/statedir/statedir.go:18` (`checkUsage`)

  `internal/query/payload.go` (~229, ~233) newly imports `internal/config` only to read it. This is
  CONVENTIONS Coordinator Policy's "a second home for a value".
- **Before-state**: `git grep -c 'CheckUsage\b\|checkUsage\b' -- '*.go'` answers 89 lines in 43
  files:
  - `cmd`: askconcurrency.go, askconcurrency_test.go, build_replay.go, build_replay_test.go,
    configure.go, configure_test.go, edges.go, export-latex.go, export-latex_test.go, init.go,
    main.go, mcp.go, mcp_test.go, probe.go, probe_test.go, query.go, replay.go, resultdocs_test.go,
    score.go, score_replay.go, score_replay_test.go, score_test.go, show-config_test.go, wait.go,
    wait_test.go
  - `internal/build`: build.go, build_test.go, replay.go, replay_test.go
  - `internal/buildstore`: rewind.go, rewind_test.go, stages.go
  - `internal/config`: configure.go, inference_test.go, resolve.go
  - `internal/mcp`: mcp.go
  - `internal/query`: payload.go
  - `internal/score`: invoke_test.go, ledger.go, ledger_test.go, replay.go
  - `internal/statedir`: statedir.go, statedir_test.go
- **Location**: the 43 files above, plus `internal/result` for the new definition.
- **Guidance**:
  1. Define `CheckUsage = "usage"` once in `internal/result`, beside `Refusal`. Every package
     above already imports `result`.
  2. Delete the five definitions and point every reference at `result.CheckUsage`, or
     `toolresult.CheckUsage` in `cmd`.
  3. Remove the `internal/config` import from `internal/query/payload.go`.
  4. Re-run the grep above. Every remaining line must be the `result` definition or a qualified
     reference to it: `git grep -n '\bcheckUsage\b' -- '*.go'` answers nothing, and
     `git grep -n 'CheckUsage *=' -- '*.go'` answers one line, in `internal/result`.
- **Confirm**: `just test-integration`.

### B13 — [Note] mechanical — `WeakPoints` is an exported wrapper with one caller

- **Finding**: `Index.WeakPoints` (`internal/query/query.go:216`) only forwards to `Strengths`.
  `git grep -n 'WeakPoints(' -- '*.go'` answers two lines: that definition and its one caller,
  `internal/query/payload.go:350` in `StatsPayload`, which passes constants.
- **Location**: `internal/query/query.go`, `internal/query/payload.go`.
- **Guidance**:
  1. Replace the call with `ix.Strengths(StrengthSelection{Max: &max, MinDependents:
     DefaultMinDependents, Sort: SortDependents})`, where `max := DefaultMaxStrength` is declared
     just before it.
  2. Delete `WeakPoints`.
- **Confirm**: the preamble's gates.

### B14 — [Note] mechanical — The leverage bar's value is in no contract constant list

- **Finding**: `query.DefaultMaxStrength = 0.65`, the bar of `stats`' `highest_leverage`, has the
  value of an unnamed band-ladder rung. `internal/kb/schema.go:36`:
  `{"ok-with-caveats", 0.65, "ok to build on, see caveats", "ok to build on, see caveats"},`.
  No contract document ties the leverage bar to a band, and binding the two would let a
  band-ladder change move `stats` silently. So the bar stays its own constant. It is listed in
  neither ARCHITECTURE §12 "Constants" nor elsewhere, and `stats`' help now prints it.
- **Location**: `ARCHITECTURE.md` §12 "Constants".
- **Guidance**: Add `query.DefaultMaxStrength` (0.65) and `query.DefaultMinDependents` (1) to
  ARCHITECTURE §12 "Constants", as the strength bar and the dependents floor of `stats`'
  `highest_leverage`. State that the bar is not derived from the band ladder.
- **Confirm**: the preamble's gates.

---

## Write-op tests

### B15 — [Warning] mechanical — `drop-claim-node` tests repeat the survivor test

- **Finding**: Two tests repeat `TestDropClaimNodeTakesEveryRecordNamingIt` through the same path,
  the `drop-claim-node` op's `apply`, `dropClaimNode` (`internal/write/marks.go:96`):
  - the second half of `TestDropClaimNode`, a premise another claim rests on (from ~163);
  - all of `TestDropClaimNodeTakesItsRefutations`.

  Through `dropGraph`'s `inboundEdges` and `unrelatedEdges`, the survivor already covers an
  inbound `supports`, a `refutes` into the claim, and the claim's own `refutes`.
- **Location**: `internal/write/drop_edges_test.go`.
- **Guidance**:
  1. Delete `TestDropClaimNode` from the `root, b = seedSharedLeaf(t, "  rests-on:…` line to the
     end of the function, and the last sentence of its doc comment.
  2. Delete `TestDropClaimNodeTakesItsRefutations` whole.
  3. Remove any imports or helpers left unused (`just edit-gate` reports them).
- **Confirm**: the preamble's gates.
