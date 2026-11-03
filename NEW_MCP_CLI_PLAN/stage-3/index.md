# Plan, stage 3 of 4: queries, score and runs (packages O–R)

**Status:** retired. Packages O–R landed (aa6a64a … 09b1c5c); the review burndown BD3-1 … BD3-11
landed (391ab27 … ca35c94).

## Stage

- This is the third of four sequential stages of the active plan that together land [page §1–§4](../../NEW_MCP_CLI_DESIGN/entry-point.md). Stages 1 (A–F) and 2
  (G–N) precede it. Stage 4 (S–X) is [`stage-4/index.md`](../stage-4/index.md).
- Entry condition: stage 2 is retired. Its packages G–N have landed and its post-sprint burndown
  has landed.
- What this stage relies on from earlier stages, and where each one is:
  - KB format `4.0.0` in `internal/formats/format400`, re-exported by `internal/kb`, with
    `judged-hash` on every scored record, all zeros where never judged
    (`git grep -n 'judged-hash' -- internal/formats/format400`);
  - the 4.0.0 anchors and the citation-link reader, in `format400` and re-exported by `internal/kb`
    (`git grep -n 'func Slugify\|func .*[Cc]itation' -- internal/formats/format400 internal/kb`);
  - build records under `kb-root/.build/` (`internal/buildrecords/buildrecords.go`) and
    `update-format` (`cmd/update-format.go`);
  - write batches with the derivation inside them, `kbase write` (`cmd/write.go`), and the batch
    runner every write surface calls (`git grep -n 'func runWriteOp' -- cmd`);
  - `add-<kind>`, `edit-<kind>`, `remove-node`, `move-node` and the edge ops, each in its own
    `cmd/<op>.go`;
  - each command's four help homes (Rule 12).
- No stage releases alone. Format `4.0.0` ships after stage 4. No commit of this stage is a
  release, and the `version` bump is not this stage's.
- Package letters run A–X across the four plans and are never re-lettered.

## Outcome

When this stage retires:
- The queries `find`, `show`, `edges`, `trace` and `stats` are [page §3.4](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/index.md)'s. Each list query
  returns its complete answer ([page §2.8](../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)).
- `score` asks only what is dirty by the judged hash ([page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md), [§2.9](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)), lands through the edit ops,
  and takes no `--limit`.
- `build`, `init`, `status`, `cancel`, `wait` and `build-trail` are [page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/index.md) and [§3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md)'s.
  `build-trail` is a CLI command and a tool. No subscription remains.
- The build's and the scoring run's own writes go through the new write ops. Nothing in kbase
  issues an op that stage 4 removes.
- The old list queries (`deps`, `cited-by`, `referenced-by`, `strength`, `subtree`) and the old
  write ops still stand, and stage 4 removes them.

## Decisions the record settles

- **The contract cites the page and does not restate it.** [CONVENTIONS "Plan & Execute Process"](../../PROJECT_DOCS/CONVENTIONS/plan-execute-process/index.md): a
  page that a plan names normative "outranks the plan"; "the contract documents cite it rather than
  restate it."
  - So each package replaces the [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md) text that its change invalidates with a citation of the
    page section.
  - SPEC keeps only what the page does not state and still holds.
  - [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md) states the mechanism, citing SPEC.
- **A replaced op is removed only after its replacement has done the job.** The build's and the
  scoring run's own writes move onto the new ops here (packages Q and R), so that package S (stage
  4) can remove the old ops.
- **The judged hash has one computing function.** `score`'s dirty check and the write ops' manual
  sets both call it ([page §2.9](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)). `score` imports `write` ([ARCHITECTURE §11](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/index.md)), so the function cannot
  live in `score`. Its home is the coder's choice in package Q. It must not import `model`. Record
  the home in [ARCHITECTURE §11](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/index.md).
- **The derivation inside a run.** [Page §2.3](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md): "`score`'s landings and the build's writes are
  followed by the derivation the same way", and the records are never written while their derived
  files are stale. How often a run's writes run the derivation is the coder's choice, in package Q
  for the scoring run and package R for the build. The derived files are current at every build
  stage commit, and whenever a run exits, a cancel included ([page §1.6](../../NEW_MCP_CLI_DESIGN/1-model/1.6-derived-files.md): derived files are never
  read back).
- **No subscription** ([page §4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md)). The build watcher (`internal/mcp/watch.go`) and the
  `kbase://build` and `kbase://build-trail` resources go.
  - `test-integration-mcp-build` subscribes today. Its doc (`justfile:252`): "subscribed to
    kbase://build, sending nothing until the end notification". It is rewritten to poll `status`
    (package R).
- **ARCHITECTURE I9 changes for the build trail.** Its listed exception, `rewind-build`, goes:
  `build-trail` is a subcommand ([page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md), [§4.4](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md)). Package R lands it.

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
     - the files holding the four help homes (Rule 12) of every command the package adds or
       changes;
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
12. **Help text.** A command's help has four homes on its own `cobra.Command`: the summary is
    `Short`, the shared body `Long`, and the MCP note and the CLI note the annotations
    `kbase.help.mcp` and `kbase.help.cli` ([page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md)).
    - `git grep -n '"kbase.help.mcp"\|"kbase.help.cli"' -- internal/mcp` finds the constants that
      name the annotations. A write op's homes are set through `writeOpCommand`
      (`git grep -n 'func writeOpCommand' -- cmd`).
    - `kbase <command> --help` prints the shared body, the CLI note, then cobra's generated usage
      block. `kbase://help/<command>` is the shared body and the MCP note. A test asserts each
      rendering is exactly its parts.
    - A package that adds or changes a command puts its help text in those homes only, and writes
      that command's [page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) body and notes.
13. **Done-lines.** Every done-line runs `just edit-gate` and `just test` first, then the recipes it
    names.
    - `just test-integration` is the hermetic omnibus: `justfile:302` runs
      `test-integration-results-fixture`, `-init`, `-opscript-results`, `-mcp`, `-config`, `-auth`,
      `-parity-adjagent-sheet`, `-refused-write-adjagent-sheet` and `test-dist-no-replay`. A
      done-line names only recipes outside it.
    - A recipe whose `[doc]` opens "NEEDS pandoc" or "NEEDS pandoc and python3" runs on this host:
      `command -v pandoc dot python3` → three paths under `/opt/homebrew/bin` and `~/.pyenv/shims`.
      Such a recipe in a done-line is run, never skipped.
    - The arXiv recipes read the pinned corpus under `test-data/transient/arxiv/`, which stands on
      this host: each id of `BUILD_ARXIV_IDS` (`justfile:87`) and `MONITOR_ARXIV_ID` (`:95`) has its
      directory there.

## Packages

| id | rating | one line | depends on | status |
|---|---|---|---|---|
| O | expert | [`find`, `show`, `stats`](O.md) | B, C (stage 1) | open |
| P | expert | [`edges`, `trace`, one per-edge contribution function](P.md) | O | open |
| P2 | expert | [each surface names an argument as it takes it: one argument marker, spelled per surface in help, schema descriptions and result documents](P2.md) | O, P | open |
| Q | expert | [`score`: judged hash, dirty scope, `node`/`edge`/`pass`, `resume` as a bool, no `--limit`](Q.md) | A (stage 1), K, M (stage 2) | open |
| R | expert | [`build`, `init`, `status`, `cancel`, `wait`, `build-trail`; no subscription; the build on the new ops](R.md) | F (stage 1), J, L (stage 2) | open |

**Order:** O, P, P2, Q, R. One coder edits the tree at a time. "Depends on" names the packages whose
output a package reads.

Also in this stage:

- [Post-sprint review](post-sprint.md)
