# kbase task recipes — two-gate discipline:
# `edit-gate` after every change (cheap), `checkpoint` once at checkpoints (full).

# Recipe bodies are bash, not just's default `sh`. Stating it makes the
# contract explicit rather than inherited from whatever /bin/sh happens to be
# on the host — bashisms like fmt-check's `[[ ]]` are then legal everywhere,
# not portable by accident. `-u` (just's own default, kept) makes an unset
# variable a failure instead of an empty string. `-o pipefail` makes a
# pipeline fail when ANY stage does: the test recipes below pipe through
# `tee` to preserve their logs, and without it a failing `go test` would be
# masked by a successful `tee` and the gate would go green on a red run.
set shell := ["bash", "-cuo", "pipefail"]

BIN_DIR := "bin"
TEST_DATA_DIR  := "test-data"
# owned/version controlled test data
TEST_DATA_FIXTURES_DIR := TEST_DATA_DIR / "fixtures"
# test output, generated test data, cloud-sourced test data
TEST_DATA_TRANSIENT_DIR := TEST_DATA_DIR / "transient"

# Per-test output roots. CONVENTIONS.md's "results are part of the test" rule:
# every test run leaves its log — and, where the gathered stats ARE the
# result, its stats and artifacts — somewhere they can be read afterwards.
# One directory per test name, all under the gitignored transient tree.
UNIT_TEST_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "unit_tests"
BENCH_LOAD_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "bench-load"
RESULTS_FIXTURE_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-results-fixture"
MONITOR_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-monitor-arxiv"
# The committed KB the hermetic result-contract recipe works over, with the
# values its write ops take.
RESULTS_FIXTURE_DIR := TEST_DATA_FIXTURES_DIR / "results"
MCP_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-mcp"
MCP_BUILD_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-mcp-build"
MEASURE_MCP_SURFACE_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "measure-mcp-surface"
INIT_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-init"
CONFIG_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-config"
MCP_TRANSCRIPT_DIR := TEST_DATA_FIXTURES_DIR / "mcp"
OPSCRIPT_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-opscript-results"
COMPAT_OP_SCRIPT := TEST_DATA_FIXTURES_DIR / "compat" / "ops.yaml"
PARITY_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-parity-adjagent-sheet"
REFUSED_WRITE_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-refused-write-adjagent-sheet"
STAGE_FIXTURE_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-stage-fixture"
# The two small papers the lamb build recipes build, and their evidence.
LAMB_FIXTURE_DIR := TEST_DATA_FIXTURES_DIR / "lamb"
LAMB_CYCLE_FIXTURE_DIR := TEST_DATA_FIXTURES_DIR / "lamb-cycle"
BUILD_LAMB_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-lamb"
BUILD_LAMB_CYCLE_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-lamb-cycle"

# Pinned integration corpus: the arXiv survey sample, taken from adjagent's
# kb-testing/justfile — papers sampled by construction across categories whose
# LaTeX differs in kind. Ids are VERSIONED: an e-print version is append-only,
# so each id is its own pin and the list reproduces the corpus while
# redistributing none of it.
#
# One variable per category: the grouping is the sample's structure, and a
# category can be fetched alone (`just prep-test-integration-arxiv "$ids"`).
#
# Upstream selection bias: upstream dropped every drawn paper that pandoc could
# not parse (2609.10029v1, 2609.10235v1, 2609.10392v1, 2609.10474v1) and one it
# parsed while silently dropping included files (2609.09821v1), so this sample
# is pandoc-clean by construction and says nothing about where pandoc itself
# fails.
ARXIV_CS_AI := "2603.15381v1"
ARXIV_CS_GR := "2609.09828v1 2609.09834v1 2609.10363v1 2609.10385v1"
ARXIV_CS_LG := "2609.10505v1 2609.10514v1 2609.10525v1 2609.10529v1 2609.10534v1"
ARXIV_ECON_TH := "2609.09693v1 2609.09888v1 2609.10074v1 2609.10499v1"
ARXIV_EESS_AS := "2609.10351v1 2609.10366v1 2609.10394v1 2609.10466v1"
ARXIV_MATH_DG := "2609.10318v1 2609.10323v1 2609.10442v1 2609.10523v1 2609.10527v1 2609.00183v1"
ARXIV_MATH_ST := "2609.09855v1 2609.10038v1 2609.10291v1 2609.10428v1 2609.10467v1"
ARXIV_MATH_AG := "2609.10401v1 2609.10420v1 2609.10454v1 2609.10481v1 2609.10488v1 2609.00268v1 2609.00269v1"
ARXIV_MATH_NT := "2609.10423v1 2609.10446v1 2609.10483v1 2609.10526v1"
ARXIV_MATH_AP := "2609.10427v1 2609.10478v1 2609.10480v1 2609.10517v1 2609.00108v1 2609.00209v1"
ARXIV_MATH_PR := "2609.10450v1 2609.10528v1 2609.10530v1"
ARXIV_PHYSICS_OPTICS := "2609.10111v1 2609.10252v1"
ARXIV_IDS := ARXIV_CS_AI + " " + ARXIV_CS_GR + " " + ARXIV_CS_LG + " " + ARXIV_ECON_TH + " " + ARXIV_EESS_AS + " " + ARXIV_MATH_DG + " " + ARXIV_MATH_ST + " " + ARXIV_MATH_AG + " " + ARXIV_MATH_NT + " " + ARXIV_MATH_AP + " " + ARXIV_MATH_PR + " " + ARXIV_PHYSICS_OPTICS
# The e-prints as downloaded, and one unpacked source tree per id beside them.
ARXIV_TGZ_DIR := TEST_DATA_TRANSIENT_DIR / "arxiv-tgz"
ARXIV_DIR := TEST_DATA_TRANSIENT_DIR / "arxiv"
# arXiv's published courtesy interval for automated access, paid only on a
# download that actually happens.
ARXIV_DELAY := "3"

# The papers test-integration-build-arxiv builds: six of the pinned corpus,
# taken across categories whose LaTeX differs in kind, listed as the VERSIONED
# e-prints the corpus pins them by.
BUILD_ARXIV_IDS := "2609.10318v1 2609.09855v1 2609.10525v1 2609.10291v1 2609.10111v1 2609.10385v1"
BUILD_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-arxiv"
# The paper test-integration-build-arxiv also kills and resumes, refuses a
# second fresh build over, and stamps over an authored document.
BUILD_ARXIV_SPECIAL_ID := "2609.10318v1"
# The paper test-integration-monitor-arxiv builds and cancels: the corpus's
# largest source, so a --no-inference build runs long enough for a polling
# monitor to see it running.
MONITOR_ARXIV_ID := "2609.10074v1"
# The live build: one Slice-1 paper built asking the provider the resolved
# configuration names.
BUILD_LIVE_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-live-arxiv"
# The export contract checker's Python, which test-export-forms runs the unit
# tests of.
EXPORT_CHECK_DIR := "tools/export"

GOPKGS := "./..."
GOSRCDIRS := "cmd internal tools"

default:
    @just --list

# builds for current host platform only. product is .gitignored. The build is
# tagged kbasedev, the development shape, so it carries the development
# replays; an untagged build is the release shape and carries none of them.
build:
    @mkdir -p {{BIN_DIR}}
    go build -tags kbasedev -o {{BIN_DIR}}/kbase ./cmd

# Both unit recipes tee their output to {{UNIT_TEST_OUT_DIR}}. `go test ./...`
# is ONE process emitting ONE combined stream, so the preserved log is
# per-recipe (test_log.txt, test-race_log.txt) rather than per-test — splitting
# it per test would mean parsing `go test -json`, which is machinery this rule
# does not need. Each run overwrites the previous log: the interesting one is
# always the last one, and an append would bury it.
#
# Both carry -tags kbasedev: the development replays and their tests live in
# files tagged kbasedev, so an untagged `go test ./...` would run none of them
# and pass, silently.

# run the unit tests. VERBOSE=1 for per-test output.
test:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    go test -tags kbasedev ${VERBOSE:+-v} ./... 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test_log.txt"

# run the unit tests under the race detector (slower; catches real data races)
test-race:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    go test -race -tags kbasedev ${VERBOSE:+-v} ./... 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test-race_log.txt"

[doc("run one package's unit tests and its subpackages: just test-pkg ./internal/kb, or internal/kb, or internal/kb/... — one package alone, for testing it while OTHER packages still fail to build; log under the package's name in the unit-test output directory; VERBOSE=1 for per-test output; a package that is not named, or that names the whole repository, is refused")]
test-pkg pkg:
    @PKG="{{pkg}}" UNIT_TEST_OUT_DIR="{{UNIT_TEST_OUT_DIR}}" tools/scripts/test-pkg.sh

[doc("benchmark the index derivation: BenchmarkLoadDerive in internal/index times index.Derive over the claim-graph sheet fixture's KB as committed (x1) and replicated x40 in memory, reporting ns/op and allocs/op at each size; log under test-data/transient/bench-load/")]
bench-load:
    @mkdir -p "{{BENCH_LOAD_OUT_DIR}}"
    go test -run '^$' -bench '^BenchmarkLoadDerive$' -benchmem ./internal/index 2>&1 | tee "{{BENCH_LOAD_OUT_DIR}}/log.txt"

[doc("NEEDS python3 (excluded from test-integration): run the tools/export unit tests, the export contract checker's own; log in the unit-test output directory")]
test-export-forms:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    python3 -B -m unittest discover -s {{EXPORT_CHECK_DIR}} 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test-export-forms_log.txt"

# Cheap gate: run after every change.
edit-gate: fmt-check
    go vet ./...
    go vet -tags kbasedev ./...
    @for f in tools/scripts/*.sh; do bash -n "${f}" || exit 1; done

# this project is developed in tandem with adjagent
AGENTS_DIR := "../adjagent"
agents:
    @AGENTS_DIR="{{AGENTS_DIR}}" tools/scripts/agents.sh

# The tap clone's one source: its origin URL, which the clone and the remote
# set-up both take, and the location its .gitignore entry names.
TAP_DIR := "homebrew-kbase"
TAP_URL := "git@github.com:benn-herrera/homebrew-kbase.git"
[doc("set up the sibling Homebrew tap clone at homebrew-kbase/, idempotent by state: absent clones it, a clone whose origin is the tap is left alone, one with no origin gets the origin, one with another origin or no .git is refused naming what it holds; one line says what was done")]
tap:
    @TAP_DIR="{{TAP_DIR}}" TAP_URL="{{TAP_URL}}" tools/scripts/tap.sh

[doc("after a release is pushed: point the tap's formula at the v<version> source tarball and its sha256; refuses until GitHub serves it; commits the formula change in the tap; pushes nothing")]
tap-update:
    #!/usr/bin/env bash
    set -euo pipefail
    ver="v$(go run ./cmd --version)"
    url="https://github.com/benn-herrera/kbase/archive/refs/tags/${ver}.tar.gz"
    sha="$(curl -fsSL "${url}" | shasum -a 256 | cut -d' ' -f1)"
    f="{{TAP_DIR}}/Formula/kbase.rb"
    sed -i '' \
        -e "s|^  url \".*\"|  url \"${url}\"|" \
        -e "s|^  sha256 \".*\"|  sha256 \"${sha}\"|" \
        "${f}"
    git -C "{{TAP_DIR}}" commit -m "kbase ${ver}" -- Formula/kbase.rb

# Integration recipes are named test-integration-<process>-<corpus>: the
# process first, the input corpus last, because a corpus is served by several
# processes and more corpora are coming. Each is paired with a
# prep-test-integration-<corpus> (pinned shallow fetch, no-op when present)
# that is shared by every process over that corpus, and the hermetic ones are
# composed by the omnibus `test-integration`. Each test pulls only its own
# data, so validating one corpus never fetches another. Adding a corpus = one
# pin block above, one prep recipe, one recipe per process, and the hermetic
# names on the omnibus.

[doc("fetch and unpack the pinned arXiv sample, one source tree per id under test-data/transient/arxiv/ (no-op per id when present); pass ids= to take a subset")]
prep-test-integration-arxiv ids=ARXIV_IDS:
    @ARXIV_IDS="{{ids}}" ARXIV_TGZ_DIR="{{ARXIV_TGZ_DIR}}" ARXIV_DIR="{{ARXIV_DIR}}" ARXIV_DELAY="{{ARXIV_DELAY}}" tools/scripts/prep-test-integration-arxiv.sh

[doc("NEEDS git and the adjagent clone (excluded from test-integration); DESTRUCTIVE to dir: reset a hand-staged fixture repository to its `cleared` tag, install the current agent set for both harnesses (claude and opencode), copy this tree's freshly built bin/kbase to the fixture's own .kbase/kbase and tools/harness.sh to .kbase/harness.sh (refused when tools/harness.sh is missing) and move the `staged` tag; dir is absolute or relative to the invocation directory; refuses a dir that is not a git repository, has no `cleared` tag (tag the commit before any agent-set install or build commit by hand), has a kbase build running (asked of its .kbase/kbase, else of this tree's build, which follow its operation lock to the run's store), or whose `cleared` commit's root .gitignore does not ignore .kbase/ — the copy and its configuration both — read with no host or local excludes; every refusal comes before anything changes")]
prep-test-integration-fixture dir: build
    @BIN_DIR="{{BIN_DIR}}" FIXTURE_DIR="{{dir}}" INVOCATION_DIR="{{invocation_directory()}}" JUST="{{just_executable()}}" AGENTS_DIR="{{AGENTS_DIR}}" tools/scripts/prep-test-integration-fixture.sh

[doc("NEEDS git and the adjagent clone (excluded from test-integration): prep-test-integration-fixture over scratch fixture repositories this recipe creates, six legs — refused-dirty: .gitignore not ignoring .kbase/, with a commit above `cleared`, the `staged` tag and an uncommitted edit, refused naming that .gitignore with HEAD, the edit and the tag all kept; refused-narrow: .gitignore ignoring only .kbase/kbase, refused; refused-host-excludes: .kbase/ ignored only by the fixture's core.excludesFile and .git/info/exclude, refused; refused-status-failed: an operation lock naming a store whose holder record no kbase writes, so status fails over it, refused naming the failed status; staged: a fixture ignoring .kbase/ staged, leaving .kbase/kbase a copy of bin/kbase (not a link), untracked, .kbase/harness.sh a copy of tools/harness.sh (executable, not a link), and the `staged` tag at HEAD; restaged-over-link: the same fixture re-staged over a .kbase/kbase that is a link to a sentinel file, leaving a copy of bin/kbase, .kbase/harness.sh a copy of tools/harness.sh and the sentinel untouched; every refusal leaves no .kbase/kbase copy; evidence under test-data/transient/test-integration-stage-fixture/")]
test-integration-stage-fixture: build
    @mkdir -p "{{STAGE_FIXTURE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" AGENTS_DIR="{{AGENTS_DIR}}" JUST="{{just_executable()}}" STAGE_FIXTURE_OUT_DIR="{{STAGE_FIXTURE_OUT_DIR}}" tools/scripts/test-integration-stage-fixture.sh 2>&1 | tee "{{STAGE_FIXTURE_OUT_DIR}}/log.txt"

[doc("NEEDS pandoc + python3 (excluded from test-integration): §8.3 Direction 1 — per slice paper ./bin/kbase build --no-inference in a fresh fixture repository, then the build's ledger and owned paths, the readiness documents, ./bin/kbase status and its result document checked; kill-and-resume, the double-run guard and stamping over an authored document on one paper; evidence under test-data/transient/test-integration-build-arxiv/")]
test-integration-build-arxiv: (prep-test-integration-arxiv BUILD_ARXIV_IDS) build
    @mkdir -p "{{BUILD_ARXIV_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" ARXIV_DIR="{{ARXIV_DIR}}" BUILD_ARXIV_IDS="{{BUILD_ARXIV_IDS}}" BUILD_ARXIV_SPECIAL_ID="{{BUILD_ARXIV_SPECIAL_ID}}" BUILD_ARXIV_OUT_DIR="{{BUILD_ARXIV_OUT_DIR}}" tools/scripts/test-integration-build-arxiv.sh 2>&1 | tee "{{BUILD_ARXIV_OUT_DIR}}/log.txt"

[doc("LIVE — NEEDS the inference endpoint the resolved configuration names (given no config argument, kbase resolves configuration as always: the global ~/.config/kbase plus any project .kbase/), pandoc and python3 (excluded from test-integration): ./bin/kbase build without --no-inference over the Slice-1 paper BUILD_ARXIV_SPECIAL_ID names, in a fresh fixture repository; README.md with its passage; the asks' captures, cached answers and group records under the state store's scratch/; a second build through claims-discovered configured by KBASE_API_BASE_URL, KBASE_DECISION, KBASE_TEXT and KBASE_API_KEY_FILE alone, no configuration directory; an optional config argument, given positionally (`just test-integration-build-live-arxiv <dir>`), names one configuration directory every kbase call then passes as --config-dir; a third build naming no configuration directory at all, with HOME on a scratch home holding the providers.toml and api_keys/ of the layer that supplied the resolved provider as the global layer and configure --create --project dialing that provider out of it and writing three references into the repository layer, green with its base URL sourced global and its provider and each model sourced repository, its show-config the evidence; and a score run configured by the environment alone — KBASE_API_BASE_URL and the three role variables, no configuration directory — one item on the scoring model, which differs from the text model and is the only model the ledger names; counts in comparison.yaml; evidence under test-data/transient/test-integration-build-live-arxiv/")]
test-integration-build-live-arxiv config="": (prep-test-integration-arxiv BUILD_ARXIV_SPECIAL_ID) build
    @mkdir -p "{{BUILD_LIVE_ARXIV_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" ARXIV_DIR="{{ARXIV_DIR}}" BUILD_ARXIV_SPECIAL_ID="{{BUILD_ARXIV_SPECIAL_ID}}" BUILD_LIVE_ARXIV_OUT_DIR="{{BUILD_LIVE_ARXIV_OUT_DIR}}" CONFIG="{{config}}" tools/scripts/test-integration-build-live-arxiv.sh 2>&1 | tee "{{BUILD_LIVE_ARXIV_OUT_DIR}}/log.txt"

# The live fixture builds read the fixture's own .kbase/*.toml and take no
# configuration argument. The staged form takes its volume roots and build flags
# as positional arguments, passed on as given; the resume form takes build
# flags only.
[positional-arguments]
[doc("LIVE — NEEDS the fixture's own .kbase/*.toml to name a reachable endpoint, and pandoc (excluded from test-integration); DESTRUCTIVE to dir's kb-root/: over a fixture repository prep-test-integration-fixture has staged (HEAD at its `staged` tag), the fixture's own kbase (.kbase/kbase) show-config captured, then probe --decision and probe --text from dir (either failing stops the run naming its role), then the fixture's own kbase build <roots...> from dir without --no-inference, configured by the fixture's own .kbase/*.toml; then the fixture's own kbase refresh and verify green; the counts the arXiv live recipe records in comparison.yaml, the log, the state store and each command's capture under .claude-temp/live-fixture/<timestamp>/; dir is absolute or relative to the invocation directory, at least one root is required, roots are relative to dir, and a build flag among them (e.g. --through document-graph) is passed as given")]
test-integration-build-live-fixture dir +roots:
    @MODE=staged FIXTURE_DIR="${1}" INVOCATION_DIR="{{invocation_directory()}}" tools/scripts/test-integration-build-live-fixture.sh "${@:2}"

[positional-arguments]
[doc("LIVE — NEEDS the fixture's own .kbase/*.toml to name a reachable endpoint, and pandoc (excluded from test-integration); DESTRUCTIVE to dir's kb-root/ and state: over a fixture repository whose commit trail (its `kb-build: <stage> |` commits) has equations-minted as its newest stage — refused, naming the newest stage the trail holds, otherwise — using state, the prior run's state directory (answer cache and records), the fixture's own kbase (.kbase/kbase) show-config captured, probe --decision and probe --text from dir, then build --resume with the given flags from dir with --state-dir state, configured by the fixture's own .kbase/*.toml, which resumes at the trail's next stage (references-found); then the post-build checks of test-integration-build-live-fixture (the fixture's own kbase refresh and verify, comparison.yaml, the log and each command's capture) under a new .claude-temp/live-fixture/<timestamp>/; dir and state are absolute or relative to the invocation directory, and the flags are build flags only, passed as given")]
test-integration-build-live-fixture-resume dir state *flags:
    @MODE=resume FIXTURE_DIR="${1}" STATE_DIR="${2}" INVOCATION_DIR="{{invocation_directory()}}" tools/scripts/test-integration-build-live-fixture.sh "${@:3}"

[doc("NEEDS pandoc (excluded from test-integration): ./bin/kbase build --no-inference over MONITOR_ARXIV_ID in the background, an out-of-process loop polling ./bin/kbase status and cancelling it with ./bin/kbase cancel; cancelled (exit 4), the lock released and its file removed, status cancelled with its resume, the resume to done; evidence under test-data/transient/test-integration-monitor-arxiv/")]
test-integration-monitor-arxiv: (prep-test-integration-arxiv MONITOR_ARXIV_ID) build
    @mkdir -p "{{MONITOR_ARXIV_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" ARXIV_DIR="{{ARXIV_DIR}}" MONITOR_ARXIV_ID="{{MONITOR_ARXIV_ID}}" MONITOR_ARXIV_OUT_DIR="{{MONITOR_ARXIV_OUT_DIR}}" tools/scripts/test-integration-monitor-arxiv.sh 2>&1 | tee "{{MONITOR_ARXIV_OUT_DIR}}/log.txt"

[doc("hermetic: every maintenance subcommand, status and cancel through ./bin/kbase over the committed fixture KB, staged through the write ops with a work, an analysis and an edge of every relation a public op writes, each result parsed against its documented keys — plus a demoted leg: one freehand edge record closing a cycle, verify refused degraded, refresh cutting it into a demoted record, verify done listing it, edges, edge-census, deps and stats showing it, resolve-demoted restore refused dependency-cycle and remove done — plus score's scoring role: a file-mode configuration naming no scoring refuses as models.scoring, --scoring-model supplies that role and reaches the pool's entry, a flag naming a provider the pool lacks refuses as --scoring-model with the pool listed, a flag naming the pool's entry reaches it and records provider-rule flag, and a run configured by the environment alone reaches the scoring model, names it in the ledger and never names text's; plus probe --decision against a Go stub answering only the SystemOne decision surface, its document parsed against the probe keys with no first token and no finish reason; evidence under test-data/transient/test-integration-results-fixture/")]
test-integration-results-fixture: build
    @mkdir -p "{{RESULTS_FIXTURE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" RESULTS_FIXTURE_DIR="{{RESULTS_FIXTURE_DIR}}" RESULTS_FIXTURE_OUT_DIR="{{RESULTS_FIXTURE_OUT_DIR}}" tools/scripts/test-integration-results-fixture.sh 2>&1 | tee "{{RESULTS_FIXTURE_OUT_DIR}}/log.txt"

[doc("hermetic, git-only: init legs over ./bin/kbase, each result checked against its documented keys — refused outside a worktree with nothing created, birth, birth-once, build refused over a created KB, the cross-process maintenance chain closed green, the no-dot placeholder; evidence under test-data/transient/test-integration-init/")]
test-integration-init: build
    @mkdir -p "{{INIT_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" INIT_OUT_DIR="{{INIT_OUT_DIR}}" tools/scripts/test-integration-init.sh 2>&1 | tee "{{INIT_OUT_DIR}}/log.txt"

[doc("hermetic: the compatibility op script test-data/fixtures/compat/ops.yaml staged by TestStageOpScript against a scratch copy of the results fixture whose leaf gains one prose line, every step through ./bin/kbase expecting done, verify green, every step re-issued expecting unchanged, and kb-root/ byte-identical after the re-issue; evidence under test-data/transient/test-integration-opscript-results/")]
test-integration-opscript-results: build
    @mkdir -p "{{OPSCRIPT_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" RESULTS_FIXTURE_DIR="{{RESULTS_FIXTURE_DIR}}" COMPAT_OP_SCRIPT="{{COMPAT_OP_SCRIPT}}" OPSCRIPT_OUT_DIR="{{OPSCRIPT_OUT_DIR}}" tools/scripts/test-integration-opscript-results.sh 2>&1 | tee "{{OPSCRIPT_OUT_DIR}}/log.txt"

[doc("hermetic: stage the results fixture, feed each test-data/fixtures/mcp/*.jsonl transcript to ./bin/kbase mcp over pipes and compare every response line to the expected line, a gui-*.jsonl transcript fed to ./bin/kbase mcp --project-dir-required (GUI mode) from outside the staged repository so each call names its project; evidence under test-data/transient/test-integration-mcp/")]
test-integration-mcp: build
    @mkdir -p "{{MCP_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" RESULTS_FIXTURE_DIR="{{RESULTS_FIXTURE_DIR}}" MCP_TRANSCRIPT_DIR="{{MCP_TRANSCRIPT_DIR}}" MCP_OUT_DIR="{{MCP_OUT_DIR}}" tools/scripts/test-integration-mcp.sh 2>&1 | tee "{{MCP_OUT_DIR}}/log.txt"

[doc("NEEDS pandoc (excluded from test-integration): a build with no inference over test-data/fixtures/mcp/paper.tex through ./bin/kbase mcp, subscribed to kbase://build, sending nothing until the notification of its end arrives; evidence under test-data/transient/test-integration-mcp-build/")]
test-integration-mcp-build: build
    @mkdir -p "{{MCP_BUILD_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" MCP_TRANSCRIPT_DIR="{{MCP_TRANSCRIPT_DIR}}" MCP_BUILD_OUT_DIR="{{MCP_BUILD_OUT_DIR}}" tools/scripts/test-integration-mcp-build.sh 2>&1 | tee "{{MCP_BUILD_OUT_DIR}}/log.txt"

[doc("hermetic, a measurement and no gate (excluded from test-integration): ./bin/kbase mcp from a system temporary directory under no repository, sent initialize, tools/list and resources/list; surface.yaml holds tools (the count), tools-list-chars (the tools/list result's JSON length), input-schema-chars (every inputSchema's JSON length, summed) and instructions-chars, measured by TestMeasureMCPSurface, which fails only on an unanswered request or a count that is not positive; evidence under test-data/transient/measure-mcp-surface/")]
measure-mcp-surface: build
    @mkdir -p "{{MEASURE_MCP_SURFACE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" MEASURE_MCP_SURFACE_OUT_DIR="{{MEASURE_MCP_SURFACE_OUT_DIR}}" tools/scripts/measure-mcp-surface.sh 2>&1 | tee "{{MEASURE_MCP_SURFACE_OUT_DIR}}/log.txt"

[doc("NEEDS pandoc (excluded from test-integration): ./bin/kbase build --no-inference over test-data/fixtures/lamb/ in fresh repositories — the fixture as committed, checked for its colon title unquoted in the volume heading, its slug and the entry point's link text, verify done, no claim, the citation resolved through lamb.bib into a references leaf, and stray.bib's uncited entry in no document; and a copy of the paper inputting lamb.bbl with no .bib beside it, checked for a done build and verify, no claim, no references leaf, the citation left as its key and the entry text in a leaf; evidence under test-data/transient/test-integration-build-lamb/")]
test-integration-build-lamb: build
    @mkdir -p "{{BUILD_LAMB_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" LAMB_FIXTURE_DIR="{{LAMB_FIXTURE_DIR}}" BUILD_LAMB_OUT_DIR="{{BUILD_LAMB_OUT_DIR}}" tools/scripts/test-integration-build-lamb.sh 2>&1 | tee "{{BUILD_LAMB_OUT_DIR}}/log.txt"

[doc("NEEDS pandoc (excluded from test-integration): ./bin/kbase build --no-inference over test-data/fixtures/lamb-cycle/lamb-cycle.tex in a fresh repository, checked for a done build, the depends-attributed report's warning of the supports edge its cycle walk cut, one demoted record from that edge's source into an analysis, and verify done listing it as a demoted finding; evidence under test-data/transient/test-integration-build-lamb-cycle/")]
test-integration-build-lamb-cycle: build
    @mkdir -p "{{BUILD_LAMB_CYCLE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" LAMB_CYCLE_FIXTURE_DIR="{{LAMB_CYCLE_FIXTURE_DIR}}" BUILD_LAMB_CYCLE_OUT_DIR="{{BUILD_LAMB_CYCLE_OUT_DIR}}" tools/scripts/test-integration-build-lamb-cycle.sh 2>&1 | tee "{{BUILD_LAMB_CYCLE_OUT_DIR}}/log.txt"

[doc("hermetic: ./bin/kbase show-config over staged configurations with HOME pointed at a staged home — absent, providers.toml alone and both files, each through KBASE_CONFIG_DIR and --config-dir, checked for its outcome, its mode, its one layer, that layer's existence flags, its diagnostic count, its roles and the absence of key material; a run setting KBASE_TEXT over the both files checked for the mixed-mode refusal it is, and one run supplying the environment alone checked for every role resolving on its own variable; a layered leg in a staged git repository whose .kbase adds a pool entry and overrides one by name, checked for the merged pool and each entry's layer, whole-entry replacement, each entry's decision surface (the value an entry names, systemone where it is silent), both layers in precedence order with exists/providersExists/configExists, the superseded provider key said once, and each role's entry, model, endpoint and the layer that supplied each field; legs for a leading ~ in the configuration directory a flag or the variable names, alone, and in an apiKeyFile (never joined to the configuration directory), with ~user and a/~/b staying literal, and with a home missing or not absolute; configure --create --project in a staged repository, checked for creating both files unchecked and without a request, for show-config reporting that layer holding both with models resolving the sole entry [local], for a second call coming back unchanged, for a --provider the pool lacks refusing against the absolute providers.toml with the creation remedy and writing nothing, for --config-dir beside --project refusing as usage, for --config-dir into an empty directory creating both files, and for --project outside a worktree refusing on the worktree; and a dial-free configure writing three references that show-config resolves for every role with no diagnostic, the writer and the resolver agreeing on the reference form; sampling legs: settings on an openai entry coming back done with the text role reporting both values and their layer, a [sampling.decision] table refused by strict load naming sampling.decision.temperature, and temperature 1.5 on an anthropic text entry coming back done with a diagnostic naming sampling.text.temperature; and one show-config tools/call over MCP from a directory holding no KB; evidence under test-data/transient/test-integration-config/")]
test-integration-config: build
    @mkdir -p "{{CONFIG_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" CONFIG_OUT_DIR="{{CONFIG_OUT_DIR}}" tools/scripts/test-integration-config.sh 2>&1 | tee "{{CONFIG_OUT_DIR}}/log.txt"

[doc("hermetic: the default, release-shaped build carries no development replays (`score --replay`, `build --replay`) — TestDistBuildHasNoReplay in the untagged build; log under test-data/transient/test-dist-no-replay/")]
test-dist-no-replay:
    @mkdir -p "{{TEST_DATA_TRANSIENT_DIR}}/test-dist-no-replay"
    @go test -count=1 -run '^TestDistBuildHasNoReplay$' ./cmd 2>&1 | tee "{{TEST_DATA_TRANSIENT_DIR}}/test-dist-no-replay/log.txt"

[doc("hermetic: every query case of the frozen 2.0.1 goldens internal/kbload/testdata/parity-2.0.1/NNN.args through ./bin/kbase over a scratch copy of internal/formats/testdata/adjagent-sheet-2.0.1 (converted at load, unsaved), each answer compared with its 2.0.1 golden by TestConvertedAnswersMatchTheGoldens under the conversion's stated rules; evidence under test-data/transient/test-integration-parity-adjagent-sheet/")]
test-integration-parity-adjagent-sheet: build
    @mkdir -p "{{PARITY_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" PARITY_OUT_DIR="{{PARITY_OUT_DIR}}" tools/scripts/test-integration-parity-adjagent-sheet.sh 2>&1 | tee "{{PARITY_OUT_DIR}}/log.txt"

[doc("hermetic: set-rigor through ./bin/kbase over a scratch copy of internal/formats/testdata/adjagent-sheet-2.0.1 whose CLAUDE.md is not the redirect, refused agents-redirect with kb-root unchanged; evidence under test-data/transient/test-integration-refused-write-adjagent-sheet/")]
test-integration-refused-write-adjagent-sheet: build
    @mkdir -p "{{REFUSED_WRITE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" REFUSED_WRITE_OUT_DIR="{{REFUSED_WRITE_OUT_DIR}}" tools/scripts/test-integration-refused-write-adjagent-sheet.sh 2>&1 | tee "{{REFUSED_WRITE_OUT_DIR}}/log.txt"

# The omnibus composes the HERMETIC recipes and writes no log of its own: each
# already preserves its full output under its own name, and a second copy of
# the same bytes under a second name is a file that can go stale against the
# one anybody reads.
[doc("run every hermetic integration test (the ones needing network, a provider, pandoc, python3 or the adjagent clone are excluded; run those by name)")]
test-integration: test-integration-results-fixture test-integration-init test-integration-opscript-results test-integration-mcp test-integration-config test-integration-parity-adjagent-sheet test-integration-refused-write-adjagent-sheet test-dist-no-replay

# Full suite: run once at checkpoints.
checkpoint: edit-gate test-race build

[doc("remove the host build only (bin/kbase, bin/kbase.exe)")]
clean:
    @echo "cleaning local kbase build"
    @rm -f "{{BIN_DIR}}/kbase" "{{BIN_DIR}}/kbase.exe"

_tag-clean:
    @git diff --quiet HEAD || { echo "tag: tracked files have uncommitted changes — commit them, then tag" >&2; exit 1; }

[doc("NEEDS a signing key git tag -s can use (gpg, or ssh where gpg.format says so): refused while tracked files have uncommitted changes, then creates a signed annotated tag v<version> locally and verifies its signature, <version> being what go run ./cmd --version prints, refused unless it is <major>.<minor>.<patch>; runs no gates, so vet the tree first; pushes nothing")]
tag: _tag-clean
    @ver="v$(go run ./cmd --version)"; [[ "${ver}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "tag: version ${ver} is not v<major>.<minor>.<patch>" >&2; exit 1; }; GPG_TTY=$(tty) git tag -s "${ver}" -m "release ${ver}"; git tag -v "${ver}"

[doc("remove every build output: bin/ and build/ entirely")]
nuke:
    @echo "cleaning all kbase builds"
    rm -rf {{BIN_DIR}} build

# cover reports aggregate test coverage across all packages. -coverpkg
# instruments every package for every test binary, so the number
# reflects how much of the codebase the WHOLE suite exercises — not
# just each package's own tests (cross-package scenario tests count).
# The final `total:` line is the headline percentage; for a
# line-by-line view run `go tool cover -html=cover.out`. cover.out is
# a .gitignore'd derived artifact. -tags kbasedev brings the
# kbasedev-tagged replay code and its tests into the measured set; an
# untagged run omits both.
[doc("aggregate whole-suite coverage; headline total on last line")]
cover:
    go test -coverpkg={{GOPKGS}} -coverprofile=cover.out -tags kbasedev {{GOPKGS}} --count=1
    @go tool cover -func=cover.out | tail -1

# fmt canonically formats every Go source root in place. fmt-check is its
# read-only gate counterpart: gofmt -l lists files that are NOT canonically
# formatted, and a non-empty list fails. fmt-check fronts `edit-gate` (and
# through it `checkpoint`) so formatting drift fails the gate — it runs in
# milliseconds, so it fronts the slow steps (fail fast).
[doc("canonically format all Go source roots in place")]
fmt:
    gofmt -w {{GOSRCDIRS}}

[doc("fail if any Go source is not canonically formatted")]
fmt-check:
    @drift="$(gofmt -l {{GOSRCDIRS}})"; \
    if [[ -n "$drift" ]]; then \
      echo "gofmt drift — these files are not canonically formatted:"; \
      echo "$drift" | sed 's/^/  /'; \
      echo "run 'just fmt' to fix."; \
      exit 1; \
    fi

[doc("render readme-assets/strength-walk.svg from readme-assets/strength-walk.dot with Graphviz dot")]
strength-walk:
    @SVG_OUT="readme-assets/strength-walk.svg" tools/scripts/strength-walk.sh

# add-dependency pins ONE vetted module into go.mod/go.sum. It exists so
# adding a dependency has a recipe like every other toolchain operation —
# `update-dependencies` is the wrong tool (its `go get -u ./...` upgrades
# the whole graph, which is unrelated churn on a commit whose subject is
# one new import).
#
#   just add-dependency example.com/mod/v2@v2.1.2
#
# The CONVENTIONS.md vetting checklist (release date, importers, deprecation
# status, transitive dep count, license) is a PRECONDITION of running this,
# not something it can check.
[doc("pin ONE vetted module: just add-dependency <module>@<version>")]
add-dependency mod:
    go get {{mod}}
    go mod tidy

[doc("drop every module nothing imports from go.mod and go.sum, upgrading nothing")]
tidy:
    go mod tidy

[doc("upgrade the WHOLE module graph; use add-dependency for one module")]
update-dependencies:
    go mod tidy
    go get -u ./...
    go mod tidy

import? '.claude/skills/kbase/kb-tools/runner-snippets/kb.just'
