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
# The committed fixtures that pin kbase's own output, which only
# regenerate-fixtures writes, and its evidence.
INIT_SKELETON_DIR := TEST_DATA_FIXTURES_DIR / "init" / "skeleton"
REGENERATE_FIXTURES_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "regenerate-fixtures"
CONFIG_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-config"
AUTH_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-auth"
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

[doc("list the public recipes with their docs")]
default:
    @just --list

[doc("build bin/kbase for the host platform with -tags kbasedev, the development shape that carries the development replays; an untagged build is the release shape and carries none")]
build:
    @mkdir -p {{BIN_DIR}}
    go build -tags kbasedev -o {{BIN_DIR}}/kbase ./cmd

[doc("build bin/release/kbase for the host platform with no build tag: the release shape the tap ships, carrying no development replay")]
build-release:
    @mkdir -p {{BIN_DIR}}/release
    go build -o {{BIN_DIR}}/release/kbase ./cmd

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

[doc("run the unit tests under -tags kbasedev; VERBOSE=1 for per-test output; log in test-data/transient/unit_tests/test_log.txt")]
test:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    go test -tags kbasedev ${VERBOSE:+-v} ./... 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test_log.txt"

[doc("run the unit tests under the race detector, slower; VERBOSE=1 for per-test output; log in test-data/transient/unit_tests/test-race_log.txt")]
test-race:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    go test -race -tags kbasedev ${VERBOSE:+-v} ./... 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test-race_log.txt"

[doc("run one package's unit tests and its subpackages, for testing it while other packages fail to build: just test-pkg ./internal/kb (or internal/kb, internal/kb/...); VERBOSE=1 for per-test output")]
test-pkg pkg:
    @PKG="{{pkg}}" UNIT_TEST_OUT_DIR="{{UNIT_TEST_OUT_DIR}}" tools/scripts/test-pkg.sh

[doc("benchmark index.Derive (BenchmarkLoadDerive) over the claim-graph sheet fixture, converted first, and its x40 in-memory replica; ns/op and allocs/op")]
bench-load:
    @mkdir -p "{{BENCH_LOAD_OUT_DIR}}"
    go test -run '^$' -bench '^BenchmarkLoadDerive$' -benchmem ./internal/index 2>&1 | tee "{{BENCH_LOAD_OUT_DIR}}/log.txt"

[doc("NEEDS python3 (excluded from test-integration): run the tools/export unit tests, the export contract checker's own; log in test-data/transient/unit_tests/")]
test-export-forms:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    python3 -B -m unittest discover -s {{EXPORT_CHECK_DIR}} 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test-export-forms_log.txt"

[doc("the cheap gate, run after every change: fmt-check, go vet plain and tagged kbasedev, bash -n of every tools/scripts script, check-hermetic.sh and check-just-docs.sh")]
edit-gate: fmt-check
    go vet ./...
    go vet -tags kbasedev ./...
    @for f in tools/scripts/*.sh; do bash -n "${f}" || exit 1; done
    @tools/scripts/check-hermetic.sh
    @tools/scripts/check-just-docs.sh

# this project is developed in tandem with adjagent
AGENTS_DIR := "../adjagent"
[doc("NEEDS the adjagent clone (AGENTS_DIR): install its agent set into this repository for both harnesses, claude and opencode")]
agents:
    @AGENTS_DIR="{{AGENTS_DIR}}" tools/scripts/agents.sh

# The tap clone's one source: its origin URL, which the clone and the remote
# set-up both take, and the location its .gitignore entry names.
TAP_DIR := "homebrew-kbase"
TAP_URL := "git@github.com:benn-herrera/homebrew-kbase.git"
[doc("set up the sibling Homebrew tap clone at homebrew-kbase/: clone it, or add the missing origin to a clone; leave a correct clone alone, refuse any other; one line says what was done")]
tap:
    @TAP_DIR="{{TAP_DIR}}" TAP_URL="{{TAP_URL}}" tools/scripts/tap.sh

[doc("LIVE; NEEDS the tap clone, a pushed release: set the tap formula's v<version> tarball and sha256 and commit it there, pushing nothing; raise go.mod's go line if Go no longer supports it, uncommitted")]
tap-update:
    @TAP_DIR="{{TAP_DIR}}" tools/scripts/tap-update.sh

[doc("LIVE; NEEDS brew and the tap clone; DESTRUCTIVE to this machine's brew install of kbase: reinstall it from a scratch formula over a tarball of the working tree's tracked files; the tap is untouched")]
tap-local:
    @TAP_DIR="{{TAP_DIR}}" tools/scripts/tap-local.sh

# Integration recipes are named test-integration-<process>-<corpus>: the
# process first, the input corpus last, because a corpus is served by several
# processes and more corpora are coming. Each is paired with a
# prep-test-integration-<corpus> (pinned shallow fetch, no-op when present)
# that is shared by every process over that corpus, and the hermetic ones are
# composed by the omnibus `test-integration`. Each test pulls only its own
# data, so validating one corpus never fetches another. Adding a corpus = one
# pin block above, one prep recipe, one recipe per process, and the hermetic
# names on the omnibus.

[doc("LIVE: fetch and unpack the pinned arXiv sample into one source tree per id under test-data/transient/arxiv/, skipping ids present; ids= takes a subset")]
prep-test-integration-arxiv ids=ARXIV_IDS:
    @ARXIV_IDS="{{ids}}" ARXIV_TGZ_DIR="{{ARXIV_TGZ_DIR}}" ARXIV_DIR="{{ARXIV_DIR}}" ARXIV_DELAY="{{ARXIV_DELAY}}" tools/scripts/prep-test-integration-arxiv.sh

[doc("NEEDS git and the adjagent clone; DESTRUCTIVE to dir: reset fixture repository dir to its `cleared` tag, install the agent set, copy bin/kbase and tools/harness.sh into .kbase/, move the `staged` tag")]
prep-test-integration-fixture dir: build
    @BIN_DIR="{{BIN_DIR}}" FIXTURE_DIR="{{dir}}" INVOCATION_DIR="{{invocation_directory()}}" JUST="{{just_executable()}}" AGENTS_DIR="{{AGENTS_DIR}}" tools/scripts/prep-test-integration-fixture.sh

[doc("NEEDS git and the adjagent clone (excluded from test-integration): prep-test-integration-fixture over scratch fixture repositories this recipe creates, checking that it stages, re-stages and refuses")]
test-integration-stage-fixture: build
    @mkdir -p "{{STAGE_FIXTURE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" AGENTS_DIR="{{AGENTS_DIR}}" JUST="{{just_executable()}}" STAGE_FIXTURE_OUT_DIR="{{STAGE_FIXTURE_OUT_DIR}}" tools/scripts/test-integration-stage-fixture.sh 2>&1 | tee "{{STAGE_FIXTURE_OUT_DIR}}/log.txt"

[doc("NEEDS pandoc and python3 (excluded from test-integration): ./bin/kbase build --no-inference over each BUILD_ARXIV_IDS paper in a fresh fixture repository; fetches the papers first if absent")]
test-integration-build-arxiv: (prep-test-integration-arxiv BUILD_ARXIV_IDS) build
    @mkdir -p "{{BUILD_ARXIV_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" ARXIV_DIR="{{ARXIV_DIR}}" BUILD_ARXIV_IDS="{{BUILD_ARXIV_IDS}}" BUILD_ARXIV_SPECIAL_ID="{{BUILD_ARXIV_SPECIAL_ID}}" BUILD_ARXIV_OUT_DIR="{{BUILD_ARXIV_OUT_DIR}}" tools/scripts/test-integration-build-arxiv.sh 2>&1 | tee "{{BUILD_ARXIV_OUT_DIR}}/log.txt"

[doc("LIVE; NEEDS the configured endpoint, pandoc and python3 (excluded from test-integration): ./bin/kbase build and score over the BUILD_ARXIV_SPECIAL_ID paper; config: a --config-dir")]
test-integration-build-live-arxiv config="": (prep-test-integration-arxiv BUILD_ARXIV_SPECIAL_ID) build
    @mkdir -p "{{BUILD_LIVE_ARXIV_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" ARXIV_DIR="{{ARXIV_DIR}}" BUILD_ARXIV_SPECIAL_ID="{{BUILD_ARXIV_SPECIAL_ID}}" BUILD_LIVE_ARXIV_OUT_DIR="{{BUILD_LIVE_ARXIV_OUT_DIR}}" CONFIG="{{config}}" tools/scripts/test-integration-build-live-arxiv.sh 2>&1 | tee "{{BUILD_LIVE_ARXIV_OUT_DIR}}/log.txt"

# The live fixture builds read the fixture's own .kbase/*.toml and take no
# configuration argument. The staged form takes its volume roots and build flags
# as positional arguments, passed on as given; the resume form takes build
# flags only.
[positional-arguments]
[doc("LIVE; NEEDS pandoc and dir's .kbase/*.toml (excluded from test-integration); DESTRUCTIVE to dir's kb-root/: dir's own kbase builds dir-relative roots; evidence: .claude-temp/live-fixture/<timestamp>/")]
test-integration-build-live-fixture dir +roots:
    @MODE=staged FIXTURE_DIR="${1}" INVOCATION_DIR="{{invocation_directory()}}" tools/scripts/test-integration-build-live-fixture.sh "${@:2}"

[positional-arguments]
[doc("LIVE; NEEDS pandoc and dir's .kbase/*.toml (excluded from test-integration); DESTRUCTIVE to dir's kb-root/ and state: dir's kbase resumes from state; evidence: .claude-temp/live-fixture/<timestamp>/")]
test-integration-build-live-fixture-resume dir state *flags:
    @MODE=resume FIXTURE_DIR="${1}" STATE_DIR="${2}" INVOCATION_DIR="{{invocation_directory()}}" tools/scripts/test-integration-build-live-fixture.sh "${@:3}"

[doc("NEEDS pandoc (excluded from test-integration): ./bin/kbase build --no-inference over MONITOR_ARXIV_ID, fetched if absent, polled and cancelled with ./bin/kbase status and cancel, then resumed to done")]
test-integration-monitor-arxiv: (prep-test-integration-arxiv MONITOR_ARXIV_ID) build
    @mkdir -p "{{MONITOR_ARXIV_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" ARXIV_DIR="{{ARXIV_DIR}}" MONITOR_ARXIV_ID="{{MONITOR_ARXIV_ID}}" MONITOR_ARXIV_OUT_DIR="{{MONITOR_ARXIV_OUT_DIR}}" tools/scripts/test-integration-monitor-arxiv.sh 2>&1 | tee "{{MONITOR_ARXIV_OUT_DIR}}/log.txt"

[doc("hermetic: every maintenance subcommand, status, cancel, probe --decision and score's scoring role through ./bin/kbase over the results fixture KB, each result parsed against its documented keys")]
test-integration-results-fixture: build
    @mkdir -p "{{RESULTS_FIXTURE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" RESULTS_FIXTURE_DIR="{{RESULTS_FIXTURE_DIR}}" RESULTS_FIXTURE_OUT_DIR="{{RESULTS_FIXTURE_OUT_DIR}}" tools/scripts/test-integration-results-fixture.sh 2>&1 | tee "{{RESULTS_FIXTURE_OUT_DIR}}/log.txt"

[doc("hermetic: ./bin/kbase init in scratch repositories, then the maintenance chain over the created KB, each result checked against its documented keys")]
test-integration-init: build
    @mkdir -p "{{INIT_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" INIT_OUT_DIR="{{INIT_OUT_DIR}}" tools/scripts/test-integration-init.sh 2>&1 | tee "{{INIT_OUT_DIR}}/log.txt"

[doc("regenerate each committed fixture that pins kbase's own output from ./bin/kbase: init's skeleton, test-data/fixtures/init/skeleton/, from init in a fresh repository")]
regenerate-fixtures: build
    @mkdir -p "{{REGENERATE_FIXTURES_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" REGENERATE_OUT_DIR="{{REGENERATE_FIXTURES_OUT_DIR}}" INIT_SKELETON_DIR="{{INIT_SKELETON_DIR}}" tools/scripts/regenerate-fixtures.sh 2>&1 | tee "{{REGENERATE_FIXTURES_OUT_DIR}}/log.txt"

[doc("split a markdown monolith by heading level into linked leaves under out (optional link map TSV), then check the leaves reassemble it and every relative link resolves")]
dice-doc monolith out map="": _docdicer
    @PYTHONPATH="{{justfile_dir()}}/tools/docdicer" python3 -m docdicer dice-doc "{{monolith}}" "{{out}}" "{{map}}"

[doc("dice the documents listed in a manifest TSV, read from src-dir, into linked volumes, with intermediates in work-dir, then verify all trees (see tools/docdicer/README.md)")]
dice-docs manifest src work: _docdicer
    @PYTHONPATH="{{justfile_dir()}}/tools/docdicer" python3 -m docdicer dice-docs "{{manifest}}" "{{src}}" "{{work}}"

[doc("write a diced tree (the directory holding its entry-point.md) back out as one markdown document at monolith, which must not exist, then check it dices back to the same tree")]
undice tree monolith: _docdicer
    @PYTHONPATH="{{justfile_dir()}}/tools/docdicer" python3 -m docdicer undice "{{tree}}" "{{monolith}}"

[doc("check every relative link and #anchor under PROJECT_DOCS, docs and the top-level *.md, and that each diced tree's documents are linked from their parent and reachable (tools/docdicer, cloned when absent)")]
check-links: _docdicer
    @PYTHONPATH="{{justfile_dir()}}/tools/docdicer" python3 -m docdicer check-links PROJECT_DOCS docs *.md
    @PYTHONPATH="{{justfile_dir()}}/tools/docdicer" python3 -m docdicer check-tree PROJECT_DOCS

[private]
_docdicer:
    @[[ -d tools/docdicer ]] || git -C tools clone git@github.com:benn-herrera/docdicer

[doc("hermetic: stage the compatibility op script test-data/fixtures/compat/ops.yaml through ./bin/kbase over a scratch copy of the results fixture, then re-issue it and check nothing changed")]
test-integration-opscript-results: build
    @mkdir -p "{{OPSCRIPT_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" RESULTS_FIXTURE_DIR="{{RESULTS_FIXTURE_DIR}}" COMPAT_OP_SCRIPT="{{COMPAT_OP_SCRIPT}}" OPSCRIPT_OUT_DIR="{{OPSCRIPT_OUT_DIR}}" tools/scripts/test-integration-opscript-results.sh 2>&1 | tee "{{OPSCRIPT_OUT_DIR}}/log.txt"

[doc("hermetic: feed each test-data/fixtures/mcp/*.jsonl transcript to ./bin/kbase mcp over pipes and compare every response with its expected line; compare each kbase://help/<command> resource with --help")]
test-integration-mcp: build
    @mkdir -p "{{MCP_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" RESULTS_FIXTURE_DIR="{{RESULTS_FIXTURE_DIR}}" MCP_TRANSCRIPT_DIR="{{MCP_TRANSCRIPT_DIR}}" MCP_OUT_DIR="{{MCP_OUT_DIR}}" tools/scripts/test-integration-mcp.sh 2>&1 | tee "{{MCP_OUT_DIR}}/log.txt"

[doc("NEEDS pandoc (excluded from test-integration): a no-inference build of test-data/fixtures/mcp/paper.tex through ./bin/kbase mcp, followed to its end by polling status, then a resume likewise")]
test-integration-mcp-build: build
    @mkdir -p "{{MCP_BUILD_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" MCP_TRANSCRIPT_DIR="{{MCP_TRANSCRIPT_DIR}}" MCP_BUILD_OUT_DIR="{{MCP_BUILD_OUT_DIR}}" tools/scripts/test-integration-mcp-build.sh 2>&1 | tee "{{MCP_BUILD_OUT_DIR}}/log.txt"

[doc("hermetic, a measurement and no gate: ./bin/release/kbase mcp, the release build, under no repository; surface.yaml holds its sizes, run/served/ every resource read, the other modes and every --help")]
measure-mcp-surface: build-release
    @mkdir -p "{{MEASURE_MCP_SURFACE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}/release" MEASURE_MCP_SURFACE_OUT_DIR="{{MEASURE_MCP_SURFACE_OUT_DIR}}" tools/scripts/measure-mcp-surface.sh 2>&1 | tee "{{MEASURE_MCP_SURFACE_OUT_DIR}}/log.txt"

[doc("NEEDS pandoc (excluded from test-integration): ./bin/kbase build --no-inference over test-data/fixtures/lamb/ as committed and inputting lamb.bbl, then a rewind run by build-trail's commands")]
test-integration-build-lamb: build
    @mkdir -p "{{BUILD_LAMB_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" LAMB_FIXTURE_DIR="{{LAMB_FIXTURE_DIR}}" BUILD_LAMB_OUT_DIR="{{BUILD_LAMB_OUT_DIR}}" tools/scripts/test-integration-build-lamb.sh 2>&1 | tee "{{BUILD_LAMB_OUT_DIR}}/log.txt"

[doc("NEEDS pandoc (excluded from test-integration): ./bin/kbase build --no-inference over test-data/fixtures/lamb-cycle/lamb-cycle.tex in a fresh repository, checking its cycle cut")]
test-integration-build-lamb-cycle: build
    @mkdir -p "{{BUILD_LAMB_CYCLE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" LAMB_CYCLE_FIXTURE_DIR="{{LAMB_CYCLE_FIXTURE_DIR}}" BUILD_LAMB_CYCLE_OUT_DIR="{{BUILD_LAMB_CYCLE_OUT_DIR}}" tools/scripts/test-integration-build-lamb-cycle.sh 2>&1 | tee "{{BUILD_LAMB_CYCLE_OUT_DIR}}/log.txt"

[doc("hermetic: ./bin/kbase show-config and the configure commands over staged configuration directories, repositories and a staged home, checking layers, roles and diagnostics, and show-config over MCP")]
test-integration-config: build
    @mkdir -p "{{CONFIG_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" CONFIG_OUT_DIR="{{CONFIG_OUT_DIR}}" tools/scripts/test-integration-config.sh 2>&1 | tee "{{CONFIG_OUT_DIR}}/log.txt"

[doc("hermetic: ./bin/kbase auth and show-config over a credentials store staged under the evidence directory, checking each key's reported source and that no stream or file holds a stored key")]
test-integration-auth: build
    @mkdir -p "{{AUTH_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" AUTH_OUT_DIR="{{AUTH_OUT_DIR}}" tools/scripts/test-integration-auth.sh 2>&1 | tee "{{AUTH_OUT_DIR}}/log.txt"

[doc("hermetic: TestDistBuildHasNoReplay, run in the untagged build, which carries no development replays (`score --replay`, `build --replay`)")]
test-dist-no-replay:
    @mkdir -p "{{TEST_DATA_TRANSIENT_DIR}}/test-dist-no-replay"
    @go test -count=1 -run '^TestDistBuildHasNoReplay$' ./cmd 2>&1 | tee "{{TEST_DATA_TRANSIENT_DIR}}/test-dist-no-replay/log.txt"

[doc("hermetic: every query case of the frozen goldens in internal/kbload/testdata/parity-2.0.1/ through ./bin/kbase over a scratch copy of the adjagent-sheet-2.0.1 fixture, compared with its golden")]
test-integration-parity-adjagent-sheet: build
    @mkdir -p "{{PARITY_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" PARITY_OUT_DIR="{{PARITY_OUT_DIR}}" tools/scripts/test-integration-parity-adjagent-sheet.sh 2>&1 | tee "{{PARITY_OUT_DIR}}/log.txt"

[doc("hermetic: edit-claim through ./bin/kbase over a scratch copy of internal/formats/testdata/adjagent-sheet-2.0.1 with an altered kb-root/CLAUDE.md, checking it is refused and kb-root unchanged")]
test-integration-refused-write-adjagent-sheet: build
    @mkdir -p "{{REFUSED_WRITE_OUT_DIR}}"
    @BIN_DIR="{{BIN_DIR}}" REFUSED_WRITE_OUT_DIR="{{REFUSED_WRITE_OUT_DIR}}" tools/scripts/test-integration-refused-write-adjagent-sheet.sh 2>&1 | tee "{{REFUSED_WRITE_OUT_DIR}}/log.txt"

# The omnibus composes the HERMETIC recipes and writes no log of its own: each
# already preserves its full output under its own name, and a second copy of
# the same bytes under a second name is a file that can go stale against the
# one anybody reads.
[doc("run every hermetic integration test; the ones needing network, a provider, pandoc, python3 or the adjagent clone are excluded and run by name")]
test-integration: test-integration-results-fixture test-integration-init test-integration-opscript-results test-integration-mcp test-integration-config test-integration-auth test-integration-parity-adjagent-sheet test-integration-refused-write-adjagent-sheet test-dist-no-replay

[doc("the full gate, run once at checkpoints: edit-gate, test-race and build")]
checkpoint: edit-gate test-race build

[doc("remove the host builds only (bin/kbase, bin/release/kbase, each with .exe)")]
clean:
    @echo "cleaning local kbase build"
    @rm -f "{{BIN_DIR}}/kbase" "{{BIN_DIR}}/kbase.exe" "{{BIN_DIR}}/release/kbase" "{{BIN_DIR}}/release/kbase.exe"

_tag-clean:
    @git diff --quiet HEAD || { echo "tag: tracked files have uncommitted changes — commit them, then tag" >&2; exit 1; }

[doc("NEEDS a signing key git tag -s can use: create and verify a signed annotated tag v<version>, <version> being go run ./cmd --version; runs no gates, pushes nothing")]
tag: _tag-clean
    @ver="v$(go run ./cmd --version)"; [[ "${ver}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "tag: version ${ver} is not v<major>.<minor>.<patch>" >&2; exit 1; }; GPG_TTY=$(tty) git tag -s "${ver}" -m "release ${ver}"; git tag -v "${ver}"

[doc("remove every build output: bin/ and build/ entirely")]
nuke:
    @echo "cleaning all kbase builds"
    rm -rf {{BIN_DIR}}

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

[doc("NEEDS dot (Graphviz): render readme-assets/strength-walk.svg from readme-assets/strength-walk.dot")]
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
