# Plan, stage 1 of 4: KB format 4.0.0 and its foundations (packages A–F)

**Status:** retired. Packages A–F landed (f5a33f9 … 5d39a96); the review burndown BD-1 … BD-11
landed (253eaec … 4e3af5a).

## Stage

- This is the first of four sequential stages of the active plan that together land [page §1–§4](../../NEW_MCP_CLI_DESIGN/entry-point.md). The
  others are [`stage-2/index.md`](../stage-2/index.md) (G–N), [`stage-3/index.md`](../stage-3/index.md) (O–R) and
  [`stage-4/index.md`](../stage-4/index.md) (S–X), under `NEW_MCP_CLI_PLAN/`.
- Entry condition: none. This stage runs first.
- No stage releases alone. Format `4.0.0` ships after stage 4. No commit of this stage is a
  release, and the `version` bump is not this stage's.
- Package letters run A–X across the four plans and are never re-lettered, so a later stage names
  a package of this one by its letter.

## Outcome

When this stage retires:
- KB format `4.0.0` is the live format, as [page §1.8](../../NEW_MCP_CLI_DESIGN/1-model/1.8-format-version.md) defines it, in its records, markers, anchors,
  summary pages, citations and build-record location.
  - kbase reads 1.0.0, 2.0.x and 3.0.0 KBs through in-memory conversion.
  - It writes nothing to an older KB but through `update-format`, which replaces `refresh`.
  - The pre-3.0.0 converters and format packages stay as they are.
- Duplicate edge identities, dangling edges and scoring cycles are load errors. No degraded state
  remains.
- `gated-on` is gone.
- Every other op stands as it does at `68021ba`, less the fields 4.0.0 drops. Stages 2–4 replace
  them.

## Decisions the record settles

- **The contract cites the page and does not restate it.** [CONVENTIONS "Plan & Execute Process"](../../PROJECT_DOCS/CONVENTIONS/plan-execute-process/index.md): a
  page that a plan names normative "outranks the plan"; "the contract documents cite it rather than
  restate it."
  - So each package replaces the [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md) text that its change invalidates with a citation of the
    page section.
  - SPEC keeps only what the page does not state and still holds.
  - [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md) states the mechanism, citing SPEC.
- **4.0.0 is a new package, `internal/formats/format400`, and `format300` is frozen.**
  - [ARCHITECTURE §11](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/index.md), under `internal/formats` ([ARCHITECTURE.md:1527–1529](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/index.md)): "A format package may
    change until a kbase release ships its version, and is frozen after".
  - 3.0.0 shipped. `git show -s --format=%s c835e23` → `kbase 0.8.0`, whose body reads "Metadata
    format 3.0.0".
  - `format400` restates whatever it shares with `format300` ([ARCHITECTURE.md:1644](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/dependency-rules.md): it "restates
    any declaration it shares with an earlier format").
  - `kb` re-exports `format400` in the place where it re-exports `format300` today.
  - The converter is `internal/formats/from300to400`, the next entry of `converters`
    (`internal/formats/formats.go:44–47`).
- **The formats tree imports nothing outside itself** (ARCHITECTURE I8, [ARCHITECTURE.md:31](../../PROJECT_DOCS/ARCHITECTURE/2-invariants.md): "a tree
  of packages that import each other and no kbase package outside it"). The test that enforces it:
  - `cmd/importpolicy_test.go:116–118` binds every package under `internal/formats`, the new
    `format400` and `from300to400` included, to import only packages of that tree;
  - `:125–128` lets `format300` import no kbase package at all. `format400` takes the same rule;
  - `formatsTreeViolations` (`:330`) lets a converter import `common` and every format up to its
    target.

  So whatever the conversion needs, it finds in the tree. Anything that both `kb` and the converter
  use is defined in `format400` and re-exported by `kb`: 4.0.0 slugging (package B) and the
  citation-link grammar (package C).
- **`format400` and `from300to400` grow across packages A–F.** No release ships between them, so
  each package extends the same two packages and the same converter goldens.
- **The pre-4.0.0 chain stays.** `internal/formats/format100`, `format200`, `format300`,
  `from100to200` and `from200to300` are not edited, and none is deleted (Rule 8). They may be
  imported where the tree's order allows.
- **A replaced op is removed only after its replacement has done the job.** Every op and query
  keeps working through this stage. The exceptions:
  - `gated-on`, whose field 4.0.0 drops (package A);
  - `refresh`, whose jobs `update-format` takes now and the empty write batch takes in stage 2
    (packages E and H).
- **The degraded state goes** ([page §2.4](../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).
  - Duplicate edge identities, dangling edges and scoring cycles become load errors.
  - `Rows.Degraded`, `DegradedItems`, `RefuseDegraded` and `check: degraded` go with them.
  - So do `refresh`'s repair and the build's write over a degraded load (`overDegraded`).
  - The build's own cycles are still cut by its ring row (`index.BreakCycles`), within a stage.
- **More of the code execs git.** [ARCHITECTURE.md:1679](../../PROJECT_DOCS/ARCHITECTURE/11-package-map/dependency-rules.md): "Only `build`, `buildstore` and `cmd`'s
  `init` verb import `gitledger`". `update-format`'s `git mv` ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)) needs git. Package F
  widens that rule to `cmd/update-format.go`, and `cmd/importpolicy_test.go`'s `cmdFileImporters`
  (`:482`, `"internal/gitledger": {"cmd/init.go"}`) widens with it. I1 is unchanged: `gitledger`
  stays the one package that execs git.

## Rules every package keeps

Rules 1, 2, 3, 4, 7, 9, 10 and 11 are in [`rules.md`](../rules.md). The rules below are this stage's own.

5. **Fixture KBs.**
   - The committed fixture KBs stamped `3.0.0` are conversion inputs and are never hand-edited.
     `git ls-files | grep 'format.yaml$'` → four files, each holding `kb-format: "3.0.0"` at
     `68021ba`:
     - `cmd/testdata/score-kb/kb-root/.index/format.yaml`;
     - `internal/sheet/testdata/adjagent-sheet/kb-root/.index/format.yaml`;
     - `test-data/fixtures/init/skeleton/.index/format.yaml`, which package A turns into a
       tool-produced 4.0.0 fixture;
     - `test-data/fixtures/results/kb-root/.index/format.yaml`.
   - A test or recipe that needs a 4.0.0 KB converts a copy.
     - Before package E lands, a recipe runs `./bin/kbase refresh` over its scratch copy, and a Go
       test saves its temp copy through `refresh`'s save, `index.RefreshReporting`
       (`internal/index/refresh.go`; `RefreshWithWarnings` folded into it in package D).
     - From package E on, a recipe runs `./bin/kbase update-format` over its scratch copy, and a Go
       test converts its temp copy through the `update-format` verb function in
       `cmd/update-format.go`.
   - `update-format` refuses a `kb-root/` with uncommitted changes ([page §3.6](../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)). A copy it converts
     sits in a git repository with the copy committed.
   - A committed 4.0.0 fixture, `init`'s skeleton included, is produced by running the tool through
     `just regenerate-fixtures` (package A adds it), never written by hand. A package that changes
     what `init` writes re-runs that recipe.
   - A converter golden under `internal/formats/testdata/` is produced as that directory's
     `README.md` "Producing an output" states: the test writes the differing conversion under
     `test-data/transient/unit_tests/formats/got/`, and the golden is replaced only by copying that
     file over it. The 3.0.0 trees there (`register-3.0.0/`, `adjagent-sheet-3.0.0/`) and their
     `.conversion.yaml` and `.values.jsonl` files are `from200to300`'s frozen outputs. They are read
     as `from300to400`'s inputs and never edited.
6. **The contract moves with the code.**
   - [SPEC.md](../../PROJECT_DOCS/SPEC/entry-point.md) first, then [ARCHITECTURE.md](../../PROJECT_DOCS/ARCHITECTURE/entry-point.md), then [CONVENTIONS.md](../../PROJECT_DOCS/CONVENTIONS/entry-point.md), in the same package as the code.
   - In each, replace the text the change invalidates, citing the page section.
   - Each user-visible change adds one bullet to [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md)'s `<release version>` list ([SPEC.md:2402](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md)).
   - A SPEC grep a package carries is partitioned by line: [SPEC.md:2402–2437](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md) is the unreleased
     `<release version>` list, whose bullets the package corrects where it makes them false;
     [SPEC.md:2438](../../PROJECT_DOCS/SPEC/12-interim-releases/0.9.0-messages-is-the-default-chat-protocol.md) on is released history and stands.
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
12. **Help text.** Before package G (stage 2), a command's help is its cobra `Short` and `Long`,
    edited where they stand.
13. **Done-lines.** Every done-line runs `just edit-gate` and `just test` first, then the recipes it
    names.
    - `just test-integration` is the hermetic omnibus: `justfile:302` runs
      `test-integration-results-fixture`, `-init`, `-opscript-results`, `-mcp`, `-config`, `-auth`,
      `-parity-adjagent-sheet`, `-refused-write-adjagent-sheet` and `test-dist-no-replay`. A
      done-line names only recipes outside it.
    - A recipe whose `[doc]` opens "NEEDS pandoc" or "NEEDS pandoc and python3" runs on this host:
      `command -v pandoc dot python3` → three paths under `/opt/homebrew/bin` and `~/.pyenv/shims`.
      Such a recipe in a done-line is run, never skipped. The arXiv recipes read the pinned corpus
      under `test-data/transient/arxiv/`, which stands on this host: each id of `BUILD_ARXIV_IDS`
      (`justfile:87`) has its directory there.

## Packages

| id | rating | one line | depends on | status |
|---|---|---|---|---|
| A | expert | [`format400` records, `kb` switched to it, `from300to400` record conversion, `gated-on` gone, `regenerate-fixtures`](A.md) | none | open |
| B | expert | [`kbase-id` markers, no-space attach, github-slugger anchors, `kb-summary.md`](B.md) | A | open |
| C | expert | [citation link titles, every heading link re-slugged, the `bare-id` fault, citation-grammar gate gone](C.md) | B | open |
| D | expert | [degraded state gone: three faults become load errors](D.md) | A | open |
| E | expert | [`update-format` replaces `refresh`; a pending conversion refuses every other write; maintained blocks](E.md) | A–D | open |
| F | expert | [build records under `kb-root/.build/`](F.md) | E | open |

**Order:** A, B, C, D, E, F. One coder edits the tree at a time. "Depends on" names the packages
whose output a package reads.

Also in this stage:

- [Post-sprint review](post-sprint.md)
