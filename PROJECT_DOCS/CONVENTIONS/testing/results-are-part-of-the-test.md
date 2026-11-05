### Results are part of the test

Results that cannot be examined are not results.

**A test helper an external test package needs is defined in that package.** An unexported name does
not cross between `package p` and `package p_test`, and a test-support package that imports `p` cannot
be imported by `p`'s in-package tests — the cycle is refused. Where the two need the same fixture,
each package keeps one copy and the pair states why.

**Where a directory contains both an in-package test file and an external `p_test` one for the same
production file, the external file is `<file>_external_test.go`.** The two cannot share a file name,
and the in-package file keeps the name that matches the production file —
`internal/model/modeltest/` contains `mockllm_test.go` and `mockllm_external_test.go` for `mockllm.go`.

**A test harness selects its inputs by content the KB contract fixes, never by presentation.** A
choice keyed on line wrapping or bullet style stays valid on one KB's output and silently finds nothing on
another's.

**A fixture whose answer shape a change invalidates changes in the same commit, and the commit body
names it.** When a change alters what an answer must look like — the parse now reads charge lines, so
a stub answering `0.5` is no answer — the fixture that answers in the old form changes with it, not in
a later package: a stub left behind makes every test case over it red at the boundary, and reads as a
failure of the change that came after. The commit body names the fixture and what it now answers, the
way it names any other truth it changed, so a reader can see which change made it answer differently.

- Golden-file tests: MCP protocol tests are transcript fixtures, `test-data/fixtures/mcp/*.jsonl`: each request line is
  followed by the expected response line, or `null` where none is sent. Placeholders: `${KB_ROOT}`,
  `${VERSION}`, `${ID1}` and `${ID2}` (the seeded claims, in id order), `${TITLE1}` and `${TITLE2}`
  (the seeded claims' titles, in id order), `${ID3}` (the staged analysis), `${WORK1}` (the staged
  work's id), `${REPO}` (the repository root) and `${STATE_DIR}`, applied to requests as well as
  responses. The two seeded claims are hosted alike and differ in title, so a transcript names a
  seeded claim's title through `${TITLE1}` or `${TITLE2}` alone, and the staged work's id through
  `${WORK1}`. A transcript named `gui-*.jsonl` runs the without-a-default-project mode: the recipe serves it
  with `kbase mcp --project-dir-required` from outside the staged repository, and
  `TestMCPTranscripts` with a server that has no default project, so a call reaches the fixture only by naming
  it, `"project-dir":"${REPO}"`. The detached-build test needs a kbase binary to re-execute:
  `TestMain` runs the test binary as kbase when `KBASE_TEST_RUN_AS_KBASE` is set. The runner stages
  the results fixture, appends `values/statements.md` to the copy's `a.md`, adds `seed.yaml`'s
  claims through `add-claim`, then a work and an analysis through their add ops, then an edge of
  each relation a public op writes through `add-edge`, each batch naming only ids an earlier
  batch's result gave, from the results fixture's `values/` files with their placeholders
  substituted, and requires exactly the expected lines on stdout.
- A transcript response a change alters is replaced by the `got:` line the failing
  `TestMCPTranscripts` run prints, with each placeholder restored as the replaced line writes it.
- Integration recipes: `test-integration-<process>-<corpus>`; shared corpus fetch
  `prep-test-integration-<corpus>`; anything needing network or a live provider is excluded from the
  hermetic aggregate target `test-integration` and says so in its `[doc]`, and a recipe that gains a tool
  dependency — pandoc, python3, the adjagent clone — leaves the aggregate target the same way, what needs no
  tool staying in it as a recipe of its own.
- A script is checked where it stands: after changing one, `bash -n tools/scripts/<name>.sh`, and
  `just edit-gate` runs `bash -n` over every `tools/scripts/*.sh`, so a syntax error fails the check
  whether or not the script's recipe is run. That is a syntax check only and does not replace running
  the recipe; a recipe that is LIVE or needs an absent tool is checked by `bash -n` on its script and
  `just --dry-run <recipe> <args>`, whose rendered line must set every variable the script requires.
  `shellcheck` is not a project prerequisite and `edit-gate` does not call it.
- The results fixture ships no claims: `test-data/fixtures/results/kb-root` contains its papers and nothing
  else, so a test case needing a claim to score cannot run against it as it is — it looks ready and is
  not, for that purpose. Copy the fixture's repo to a scratch directory and add what the test case needs
  through the add ops: `add-claim` places each claim on a leaf's line by `document`, `line` and
  `snippet`, as the transcript staging above does on `values/statements.md`'s lines. A claim given
  a `rigor` is manually scored, and a scoring run skips it.
- A Go test that must drive `kbase mcp` over pipes the recipe provides starts no process: the recipe
  starts the server on two named FIFOs and passes their paths with `-args`, and the test opens the
  write end first, so neither side waits on the other's open.
- Every integration test preserves its full log, its stats, and its run artifacts under
  `test-data/transient/<test-name>/` — nothing is written to an ephemeral location and tossed. A test case
  that must run outside any repository runs in a system temporary directory and copies its whole
  run tree under `test-data/transient/<test-name>/` before removing it. Every
  integration test passes `--state-dir` under that directory to each command that uses the state
  store, so nothing reaches the user's real `$XDG_STATE_HOME`; a command that never touches the
  store (`update-format`, `verify`, the queries) takes no such flag. Every verb that reads the
  configuration also reads the credentials store under `$XDG_STATE_HOME`, which neither
  `--state-dir` nor `--config-dir` moves, so a hermetic recipe that runs one sets `XDG_STATE_HOME`
  under that directory too, and `cmd`'s `TestMain` points it at scratch. `edit-gate` runs
  `tools/scripts/check-hermetic.sh`, which fails each `tools/scripts/test-integration-*.sh` that
  runs the binary with such a verb (`build`, `score`, `probe`, `show-config`,
  `configure-providers`, `configure-models`, `models`, `auth`) and sets `XDG_STATE_HOME` by neither `export` nor `env`, unless its recipe's
  `[doc]` begins `LIVE`. `edit-gate` also runs `tools/scripts/check-just-docs.sh`, which fails a
  public `justfile` recipe with no `[doc]`, a doc over 200 characters, `LIVE` anywhere but a doc's
  first word, a doc's first word that is a marker (`LIVE`, `NEEDS`, `DESTRUCTIVE`, `hermetic`) written in another case, a doc containing a non-ASCII byte, "(excluded
  from test-integration)" on a recipe that is not `test-*`, and an aggregate-target recipe whose doc does
  not open `hermetic`. `build` refuses an unknown `--through`,
  `--resume` given inputs, no inputs without `--resume`, a `--state-dir` inside `kb-root/` and a
  `--ask-concurrency` under 1 before it resolves and creates its state store, and those refusals contain no `state-dir`; its later refusals follow the store. Every
  recipe invocation of `build` takes `--state-dir`, refused outcomes included.
- A scratch copy of a fixture repository under this repository needs its own `.git` (an empty
  directory serves) before kbase runs over it. An empty `.git` serves queries only; a command that
  runs git over the KB — a committing batch, `update-format`, `init`, `build`, `status`, `cancel`,
  `wait` — is refused `worktree` there, so such a copy needs a `.git` git reads. Pitfall: kbase resolves the repository root at the
  nearest `.git` upward, so a copy without one answers from this repository's own `kb-root/`,
  silently. A copy that `update-format` converts — every committed fixture KB whose format version is below the
  current one, before any write over it — needs more: a `.git` git reads (`HEAD`, `objects/`,
  `refs/heads/`) with the copy's `kb-root/` committed, since `update-format` refuses uncommitted
  changes there (`check: dirty-paths`). A recipe commits with `git -C <repo> -c user.name=… -c
  user.email=…` under `GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null`; a `cmd` test calls
  `commitKB` or `updateFormatKB` (`cmd/maintenance_test.go`); a test outside `cmd` converts through
  `index.UpdateFormat`, the save the verb runs, which takes no clean check. A tree holding a build
  record at the repository root converts only through `cmd`'s verb, whose `git mv` is the one
  mover: a test outside `cmd` passes `index.UpdateFormat` no moves and no mover of its own.
- Hermetic tests: unit tests pass hermetically (no network, no corpus) beside their packages; their `go test` log is
  preserved under `test-data/transient/unit_tests/`. Corpus-driven property tests skip when the
  corpus is absent and are wired into an integration recipe where it is guaranteed present.
- `just test-pkg` takes `GOFLAGS=-run=<regexp>` to run one test; red-first evidence uses it.
- Observational evidence (counts, measurements, A/B numbers) is emitted to an inspectable location,
  never left in agent reports or scrollback.

---

