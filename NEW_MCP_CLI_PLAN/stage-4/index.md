# Plan, stage 4 of 4: removals, checks, configuration and served text (packages S–X)

**Status:** retired. Packages S–X landed (93c6c47 … 5423d55, README c8c946c); the surface check
and the post-sprint review ran, and the review burndown BD4-1 … BD4-9 landed (224eb3b … fb6d9e4).
Format `4.0.0` is ready to ship; the adjagent served-text map follows (surface-check.md).

## Stage

- This is the last of four sequential stages of the active plan that together land [page §1–§4](../../NEW_MCP_CLI_DESIGN/entry-point.md). Stages 1 (A–F), 2
  (G–N) and 3 (O–R) precede it.
- Entry condition: stage 3 is retired. Its packages O–R have landed and its post-sprint burndown
  has landed.
- What this stage relies on from earlier stages, and where each one is:
  - KB format `4.0.0` in `internal/formats/format400`, re-exported by `internal/kb`;
    `update-format` (`cmd/update-format.go`); load errors; the citation-link reader and the
    `bare-id` fault (`git grep -n '"bare-id"' -- internal/index`) (stage 1);
  - write batches, `kbase write` (`cmd/write.go`), the empty batch that re-derives, each command's
    four help homes (Rule 12), and every replacement write op and `citations-of`, each in its own
    `cmd/<op>.go` (stage 2);
  - the new queries, `score` on the judged hash, `build-trail` (`cmd/build-trail.go`), no
    subscription, and the build's and the scoring run's writes on the new ops (stage 3).
- No stage releases alone. Format `4.0.0` ships after this stage. The release and the `version`
  bump are not this plan's.
- Package letters run A–X across the four plans and are never re-lettered.

## Outcome

When this stage retires, the four stages together have landed:
- KB format `4.0.0`, as [page §1.8](../../NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md) defines it.
  - kbase reads 1.0.0, 2.0.x and 3.0.0 KBs through in-memory conversion.
  - It writes nothing to an older KB but through `update-format`.
  - The pre-3.0.0 converters and format packages stay as they are.
- Every write through a write batch ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)).
- The write ops of [page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md)–[§3.3](../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/index.md), and every op they replace gone.
- The queries `find`, `show`, `edges`, `trace`, `stats` and `citations-of` ([page §3.3](../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/index.md)–[§3.4](../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/index.md)), each
  list query returning its complete answer.
- The runs, checks and setup commands of [page §3.5](../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/index.md)–[§3.8](../../NEW_MCP_CLI_DESIGN/3-operations/3.8-server/index.md).
  - `score` asks only what is dirty.
  - `build-trail` is a CLI command and a tool.
  - `update-format` replaces `refresh`.
  - `configure-providers` and `configure-models` replace `configure`.
  - `probe` is a tool.
- The served text of [page §4](../../NEW_MCP_CLI_DESIGN/4-served-text/index.md):
  - the eight instruction items;
  - one shared help body per command, plus an MCP note and a CLI note;
  - the two server modes;
  - the 16 tools, with no subscription.
- `just measure-mcp-surface` captures the release build, in every server mode [page §4.3](../../NEW_MCP_CLI_DESIGN/4-served-text/4.3-server-modes.md) and [§4.5](../../NEW_MCP_CLI_DESIGN/4-served-text/4.5-served-resources.md)
  distinguish.

## Decisions the record settles

- **The contract cites the page and does not restate it.** [CONVENTIONS "Plan & Execute Process"](../../PROJECT_DOCS/CONVENTIONS/plan-execute-process/index.md): a
  page that a plan names normative "outranks the plan"; "the contract documents cite it rather than
  restate it."
  - So each package replaces the [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md) text that its change invalidates with a citation of the
    page section.
  - SPEC keeps only what the page does not state and still holds.
  - [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md) states the mechanism, citing SPEC.
- **A replaced op is removed only after its replacement has done the job.** Stages 2 and 3 landed
  every replacement with its tests, and moved the build's and the scoring run's own writes onto
  them. Package S removes the old ops.
- **ARCHITECTURE I9's last changes.**
  - `probe` becomes a bound tool ([page §3.7](../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/probe.md), [§4.4](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md)). Package V.
  - The CLI-only commands are `auth`, `wait` and `mcp`, plus cobra's `help` and `completion`
    ([page §4.4](../../NEW_MCP_CLI_DESIGN/4-served-text/4.4-cli-only-commands.md)). Package W names them in the root help and `mcp`'s help.
- **The release build.**
  - `justfile` `build` (`justfile:110–113`): "build bin/kbase for the host platform with -tags
    kbasedev".
  - `measure-mcp-surface` (`justfile:258`) depends on `build` and runs `${BIN_DIR}/kbase`. Its
    script does so at
    `tools/scripts/measure-mcp-surface.sh:30`, where `kbase="${root}/${BIN_DIR}/kbase"`.
  - So it captures the development build today. No recipe builds the release shape into `bin/`:
    `git grep -n 'go build' -- justfile` → only the `build` recipe's line, `:113`.
  - Package X adds that recipe.

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
13. **Done-lines.** Every coder done-line runs `just edit-gate` and `just test` first, then the
    recipes it names. The tech-writer dispatch's done-line is its own (package W).
    - `just test-integration` is the hermetic omnibus: `justfile:302` runs
      `test-integration-results-fixture`, `-init`, `-opscript-results`, `-mcp`, `-config`, `-auth`,
      `-parity-adjagent-sheet`, `-refused-write-adjagent-sheet` and `test-dist-no-replay`. A
      done-line names only recipes outside it.
    - A recipe whose `[doc]` opens "NEEDS pandoc" or "NEEDS pandoc and python3" runs on this host:
      `command -v pandoc dot python3` → three paths under `/opt/homebrew/bin` and `~/.pyenv/shims`.
      Such a recipe in a done-line is run, never skipped.

## Packages

| id | rating | one line | depends on | status |
|---|---|---|---|---|
| S | expert | [the superseded ops removed; op script, transcripts, fixture scripts and parity cases rewritten](S.md) | J–N (stage 2), O–R (stage 3) | open |
| T | expert | [`verify` and `export-latex`](T.md) | C, D, E (stage 1), H (stage 2) | open |
| U | expert | [`configure-providers`, `configure-models`](U.md) | G (stage 2) | open |
| V | expert | [`models`, `probe`, `show-config`, `auth`: the role vocabulary, case-insensitive providers, `probe` a tool](V.md) | U | open |
| W | expert | [served text: instructions, server modes, tool descriptions, resources, stamped templates; README by tech-writer](W.md) | S, T, V, G (stage 2) | open |
| X | expert | [`measure-mcp-surface` captures the release build, located mode with a KB included](X.md) | none | open |

**Order:** S, T, U, V, W (its coder dispatch, then its tech-writer dispatch), X, then the surface
check. One coder edits the tree at a time. "Depends on" names the packages whose output a package
reads. U reads no output of this stage's, so it may move earlier in the order. X stays last of the
packages, and the surface check runs after X. None may run beside another.

Also in this stage:

- [Surface check](surface-check.md)
- [Post-sprint review](post-sprint.md)
