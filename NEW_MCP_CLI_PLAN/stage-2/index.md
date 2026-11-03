# Plan, stage 2 of 4: the write surface (packages G–N)

**Status:** retired. Packages G–N landed (49ad800 … bc868f7); the review burndown BD2-1 … BD2-10
landed (04a2c8e … 9ab0ebb).

## Stage

- This is the second of four sequential stages of the active plan that together land [page §1–§4](../../NEW_MCP_CLI_DESIGN/entry-point.md). Stage 1 (A–F) is
  [`stage-1/index.md`](../stage-1/index.md). Stage 3 (O–R) is [`stage-3/index.md`](../stage-3/index.md),
  and stage 4 (S–X) is [`stage-4/index.md`](../stage-4/index.md).
- Entry condition: stage 1 is retired. Its packages A–F have landed and its post-sprint burndown
  has landed.
- What this stage relies on from stage 1, and where each one is:
  - KB format `4.0.0`, defined in `internal/formats/format400` and re-exported by `internal/kb`;
  - the `kbase-id` marker and its no-space attach (`git grep -n 'kbase-id' -- internal/formats/format400`);
  - the citation-link reader: link text, target and title id, defined in `format400` and
    re-exported by `internal/kb` (`git grep -n 'func .*[Cc]itation' -- internal/formats/format400 internal/kb`);
  - build records under `kb-root/.build/` (`internal/buildrecords/buildrecords.go`);
  - load errors in place of the degraded state;
  - `update-format` in place of `refresh` (`cmd/update-format.go`), and every write over an older
    KB refused;
  - `just regenerate-fixtures` for tool-produced fixtures.
- No stage releases alone. Format `4.0.0` ships after stage 4. No commit of this stage is a
  release, and the `version` bump is not this stage's.
- Package letters run A–X across the four plans and are never re-lettered.

## Outcome

When this stage retires:
- Every write goes through a write batch ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)).
  - A batch is ordered and mixed. It is checked for shape first, applied in memory, and stopped at
    the first refusal.
  - Nothing is written unless all of it is, the derivation included.
  - A batch can be a dry run, and can commit `kb-root/`.
- `kbase write` and the MCP `write` tool take a batch. Each op's subcommand takes a batch of that
  op alone.
- The write ops of [page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md)–[§3.3](../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/index.md) stand:
  - node writes: `add-<kind>`, `edit-<kind>`, `remove-node`, `move-node`;
  - edge writes: `add-edge`, `edit-edge`, `remove-edge`, `restore-demoted`;
  - `add-citation`, and the query `citations-of`.
- The ops these replace still stand, and stage 4 removes them.
- Each command's help has four homes: the summary, the shared body, the MCP note and the CLI note
  ([page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)).

## Decisions the record settles

- **The contract cites the page and does not restate it.** [CONVENTIONS "Plan & Execute Process"](../../PROJECT_DOCS/CONVENTIONS/plan-execute-process/index.md): a
  page that a plan names normative "outranks the plan"; "the contract documents cite it rather than
  restate it."
  - So each package replaces the [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md) text that its change invalidates with a citation of the
    page section.
  - SPEC keeps only what the page does not state and still holds.
  - [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md) states the mechanism, citing SPEC.
- **A replaced op is removed only after its replacement has done the job.**
  - The old write ops, and `render-citation`, keep working through this stage.
  - Package S (stage 4) removes them once the replacements have landed with their tests.
  - The build's and the scoring run's own writes move onto the new ops in stage 3 (packages Q and
    R).
- **ARCHITECTURE I6 changes.** Today: "Every mutating op is idempotent under re-issue". [Page §2.10](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md):
  - an add of something that stands is refused;
  - a removal of something that does not stand is refused;
  - an edit to a value that already stands is `unchanged`.

  So a re-issue never changes `kb-root/`, but an add or a removal re-issued is `refused`. The old
  write ops stay idempotent under re-issue until package S removes them. Package J restates I6 to
  that effect, citing [page §2.10](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md).
- **ARCHITECTURE I9 changes for the `write` tool.** It takes a batch ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)) rather than
  `operation` and `arguments`. `query` stays a family tool. Package H lands it. Stages 3 and 4 make
  I9's other changes.
- **`kbase write` is a new command.** The MCP `write` tool binds to it ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md), [§4.4](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md): "Every
  tool other than `query` is a CLI command"). Each write op keeps its own subcommand, which takes a
  batch of that op alone.
- **More of the code execs git.** [ARCHITECTURE §11](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/index.md)'s bullet naming `gitledger`'s importers names
  `build`, `buildstore`, `cmd/init.go` and, from stage 1, `cmd/update-format.go`. A committing batch
  ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)) needs git. Package I adds the batch runner's file to that bullet and to
  `cmdFileImporters` in `cmd/importpolicy_test.go` (`git grep -n '"internal/gitledger":' -- cmd/importpolicy_test.go`).
  I1 is unchanged: `gitledger` stays the one package that execs git.

## Rules every package keeps

Rules 1, 2, 3, 4, 7, 9, 10 and 11 are in [`rules.md`](../rules.md). The rules below are this stage's own.

5. **Fixture KBs.**
   - The committed fixture KBs stamped `3.0.0` are conversion inputs and are never hand-edited.
     `git ls-files | grep 'format.yaml$'` → four files at `68021ba`. Three are 3.0.0 KBs:
     - `cmd/testdata/score-kb/kb-root/.index/format.yaml`;
     - `internal/sheet/testdata/adjagent-sheet/kb-root/.index/format.yaml`;
     - `test-data/fixtures/results/kb-root/.index/format.yaml`.

     The fourth, `test-data/fixtures/init/skeleton/.index/format.yaml`, is a tool-produced 4.0.0
     fixture since stage 1.
   - A test or recipe that needs a 4.0.0 KB converts a copy. A recipe runs `./bin/kbase
     update-format` over its scratch copy. A Go test converts its temp copy through the
     `update-format` verb function (`git grep -n '^func ' -- cmd/update-format.go`).
   - `update-format` refuses a `kb-root/` with uncommitted changes ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)). A copy it converts
     sits in a git repository with the copy committed.
   - A committed 4.0.0 fixture, `init`'s skeleton included, is produced by running the tool through
     `just regenerate-fixtures`, never written by hand. A package that changes what `init` writes
     re-runs that recipe.
6. **The contract moves with the code.**
   - [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md) first, then [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md), then [CONVENTIONS.md](../../PROJECT_DOCS/CONVENTIONS/entry-point.md), in the same package as the code.
   - In each, replace the text the change invalidates, citing the page section.
   - Each user-visible change adds one bullet to [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md)'s `<release version>` list ([SPEC.md:2402](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md)).
     A bullet in that list that the package makes false is corrected. The released sections below
     it are history and stand.
8. **Scope.**
   - The scope is:
     - the files the package names;
     - every file its `git grep` commands return, re-run at dispatch;
     - `test-data/fixtures/mcp/*.jsonl`, wherever the package adds, renames or removes a command,
       tool or resource, or moves a response a transcript pins;
     - the tests beside them.
   - The transcripts are exact. `test-integration-mcp.sh` compares each response line with the
     expected line by string equality (`tools/scripts/test-integration-mcp.sh:126–127`), and so does
     `TestMCPTranscripts` (`cmd/mcp_test.go:258–267`). `resources.jsonl`'s `resources/list` response
     names every `kbase://help/<command>` (42 at `68021ba`), and its `kbase://help/write` read lists
     every write op. So every package that adds, renames or removes a command moves it.
   - Files in `internal/formats/format100`, `format200`, `format300`, `from100to200` and
     `from200to300` are never edited, whatever a grep returns.
   - Where the work needs any other file, the executor reports it and stops. It does not widen the
     scope on its own reading.
12. **Help text.** From package G on, a command's help has four homes: the summary, the shared
    body, the MCP note and the CLI note ([page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)), placed as package G states. A package that adds
    or changes a command puts its help text in those four homes only, and writes that command's
    [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) body and notes.
13. **Done-lines.** Every done-line runs `just edit-gate` and `just test` first, then the recipes it
    names.
    - `just test-integration` is the hermetic omnibus: `justfile:302` runs
      `test-integration-results-fixture`, `-init`, `-opscript-results`, `-mcp`, `-config`, `-auth`,
      `-parity-adjagent-sheet`, `-refused-write-adjagent-sheet` and `test-dist-no-replay`. A
      done-line names only recipes outside it.
    - A recipe whose `[doc]` opens "NEEDS pandoc" runs on this host: `command -v pandoc dot python3`
      → three paths under `/opt/homebrew/bin` and `~/.pyenv/shims`. Such a recipe in a done-line is
      run, never skipped.

## Packages

| id | rating | one line | depends on | status |
|---|---|---|---|---|
| G | expert | [help: one shared body plus MCP and CLI notes per command](G.md) | none | open |
| H | expert | [write batches: mixed, two-phase, all-or-nothing with the derivation, dry run; `kbase write`; the `write` tool](H.md) | D, E (stage 1), G | open |
| I | expert | [`commit` on write batches and `update-format`](I.md) | H, F (stage 1) | open |
| J | expert | [`add-claim`, `add-analysis`, `add-experiment`, `add-work`, `add-axiom`](J.md) | H, B (stage 1) | open |
| K | expert | [`edit-<kind>` for each kind](K.md) | J, C (stage 1) | open |
| M | expert | [`add-edge`, `edit-edge`, `remove-edge`, `restore-demoted`](M.md) | H | open |
| L | expert | [`remove-node`, `move-node`](L.md) | J, K, M, C (stage 1) | open |
| N | expert | [`add-citation`, `citations-of`](N.md) | H, L, C (stage 1) | open |

**Order:** G, H, I, J, K, M, L, N. One coder edits the tree at a time. "Depends on" names the
packages whose output a package reads. M runs before L because L's mention refusal is cleared by an
`edit-edge` entry.

Also in this stage:

- [Post-sprint review](post-sprint.md)
