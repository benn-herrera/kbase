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
RESULTS_FIXTURE_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-results-fixture"
MONITOR_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-monitor-arxiv"
# The committed KB the hermetic result-contract recipe works over, with the
# values its write ops take, and the harness stub the back end override names:
# one command, one mode per behaviour the process contract has.
RESULTS_FIXTURE_DIR := TEST_DATA_FIXTURES_DIR / "results"
HARNESS_STUB := TEST_DATA_FIXTURES_DIR / "harness" / "stub.sh"
MCP_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-mcp"
MCP_BUILD_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-mcp-build"
INIT_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-init"
CONFIG_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-config"
MCP_TRANSCRIPT_DIR := TEST_DATA_FIXTURES_DIR / "mcp"

DIST_DIR := "dist"

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
# The live build: one Slice-1 paper built asking the provider the live
# configuration fixture names.
BUILD_LIVE_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-live-arxiv"
LIVE_CONFIG_DIR := TEST_DATA_FIXTURES_DIR / "config"
# The export contract checker's Python, which test-export-forms runs the unit
# tests of.
EXPORT_CHECK_DIR := "tools/export"

GOPKGS := "./..."
GOSRCDIRS := "cmd internal"

default:
    @just --list

# builds for current host platform only. product is .gitignored.
build:
    @mkdir -p {{BIN_DIR}}
    go build -o {{BIN_DIR}}/kbase ./cmd

# Both unit recipes tee their output to {{UNIT_TEST_OUT_DIR}}. `go test ./...`
# is ONE process emitting ONE combined stream, so the preserved log is
# per-recipe (test_log.txt, test-race_log.txt) rather than per-test — splitting
# it per test would mean parsing `go test -json`, which is machinery this rule
# does not need. Each run overwrites the previous log: the interesting one is
# always the last one, and an append would bury it.

# run the unit tests. VERBOSE=1 for per-test output.
test:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    go test ${VERBOSE:+-v} ./... 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test_log.txt"

# run the unit tests under the race detector (slower; catches real data races)
test-race:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    go test -race ${VERBOSE:+-v} ./... 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test-race_log.txt"

[doc("run one package's unit tests and its subpackages: just test-pkg ./internal/kb, or internal/kb, or internal/kb/... — one package alone, for testing it while OTHER packages still fail to build; log under the package's name in the unit-test output directory; VERBOSE=1 for per-test output; a package that is not named, or that names the whole repository, is refused")]
test-pkg pkg:
    #!/usr/bin/env bash
    set -euo pipefail
    pkg="{{pkg}}"
    # Reduce the spellings to the path alone; `./<pkg>/...` is built below, so a
    # bare pattern is never handed to go test.
    pkg="${pkg#./}"
    pkg="${pkg%/...}"
    pkg="${pkg%/}"
    if [[ -z "${pkg}" || "${pkg}" == "." || "${pkg}" == "..." ]]; then
        printf 'error: name the package to test, e.g. just test-pkg ./internal/kb — the whole repository is `just test`.\n' >&2
        exit 1
    fi
    mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    go test ${VERBOSE:+-v} "./${pkg}/..." 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test-pkg_${pkg//\//-}_log.txt"

[doc("NEEDS python3 (excluded from test-integration): run the tools/export unit tests, the export contract checker's own; log in the unit-test output directory")]
test-export-forms:
    @mkdir -p "{{UNIT_TEST_OUT_DIR}}"
    python3 -B -m unittest discover -s {{EXPORT_CHECK_DIR}} 2>&1 | tee "{{UNIT_TEST_OUT_DIR}}/test-export-forms_log.txt"

# Cheap gate: run after every change.
edit-gate: fmt-check
    go vet ./...

AGENTS_REPO := "https://github.com/benn-herrera/adjagent.git"
AGENTS_DIR := ".claude" / file_stem(AGENTS_REPO)
agents:
  @mkdir -p "{{parent_directory(AGENTS_DIR)}}"
  @[[ -d "{{AGENTS_DIR}}" ]] && git -C "{{AGENTS_DIR}}" pull || git -C {{parent_directory(AGENTS_DIR)}} clone "{{AGENTS_REPO}}"
  just --justfile "{{AGENTS_DIR}}/justfile" install "$(pwd)" --harness=claude
  just --justfile "{{AGENTS_DIR}}/justfile" install "$(pwd)" --harness=opencode --model-tier-map=all=inherit

# Integration recipes are named test-integration-<process>-<corpus>: the
# process first, the input corpus last, because a corpus is served by several
# processes and more corpora are coming. Each is paired with a
# prep-test-integration-<corpus> (pinned shallow fetch, no-op when present)
# that is shared by every process over that corpus, and the hermetic ones are
# composed by the omnibus `test-integration`. Each test pulls only its own
# data, so validating one corpus never fetches another. Adding a corpus = one
# pin block above, one prep recipe, one recipe per process, and the hermetic
# names on the omnibus.

# Every integration recipe that preserves output under
# test-data/transient/<test-name>/ overwrites an EVIDENCE.md there: which
# recipe produced the directory, its exact invocation, and one line per
# artifact kind actually present. "Results are part of the test" (CONVENTIONS.md)
# extends to the directory itself — a folder of logs that does not say what
# produced it is not evidence.
#
# One shared writer, not a copy per recipe: every integration recipe draws
# artifacts from the same small vocabulary (a log, per-paper or per-fixture
# directories, comparison records), so a fixed checklist covers them all. It
# is called as
# the LAST step of each private recipe body, via a recursive `just`
# invocation — the same subprocess idiom the tee wrappers below use — because
# a `just` dependency only runs BEFORE a recipe's own body, and this needs to
# run after the artifacts it inspects already exist.
[private]
_write-evidence out_dir recipe invocation:
    @{ \
      echo "# EVIDENCE"; \
      echo; \
      echo "Producing recipe: just {{recipe}}"; \
      echo; \
      echo "Invocation:"; \
      echo; \
      printf '%b\n' "{{invocation}}" | sed 's/^/    /'; \
      echo; \
      echo "Artifacts present:"; \
      [[ -f "{{out_dir}}/log.txt" ]]      && echo "- log.txt: full recipe log (tee'd stdout+stderr)"; \
      [[ -f "{{out_dir}}/checks.tsv" ]]             && echo "- checks.tsv, docs/: per invocation its arguments, the outcome wanted and pass or fail; docs/NN-<verb>.out, .log, .exit and .keys.log (the documented-keys parse); repo/ as the run left it"; \
      [[ -f "{{out_dir}}/init.tsv" ]]               && echo "- init.tsv, docs/: per invocation its arguments, the outcome wanted and pass or fail; docs/NN-<verb>.out, .log, .exit and .keys.log (the documented-keys parse), docs/init-nodot.{out,log}; repo/, repo-nodot/, gitonly/ and state/ as the runs left them"; \
      [[ -f "{{out_dir}}/config.tsv" ]]             && echo "- config.tsv, docs/: per invocation the configuration it read, the source wanted and pass or fail; docs/NN-<state>-<route>.out, .log and .exit; configs/ as staged (absent, pool, both); work/ (the directory every invocation ran in); mcp.requests.jsonl and mcp.actual.jsonl, the config tools/call and the frames the server answered"; \
      [[ -d "{{out_dir}}/layered" ]]                && echo "- home/, layered/, literal/: home/ the staged home directory every invocation ran under, holding the ~ fixtures (cfg/, keys/); layered/repo the staged git repository the layered leg read, with its own .kbase/ over layered/home/.config/kbase; literal/ the configuration directory whose pool entries spell ~user/key.txt and a/~/b/key.txt — paths only a join to that directory reaches"; \
      [[ -f "{{out_dir}}/comparison.tsv" ]]         && echo "- comparison.tsv, transcripts/<name>/: per MCP transcript pass or the response line that differs; requests.jsonl, expected.jsonl (placeholders substituted), actual.jsonl, mcp.log, mcp.exit; insert.*, refresh.* (.out, .log) and repo/ as the staging left them"; \
      [[ -f "{{out_dir}}/monitor.tsv" ]]           && echo "- monitor.tsv, polls/: every status poll the monitor made while the build ran (polls/NNN.out); build.*, cancel.*, cancel-again.*, status-*.*, resume.* (.out, .log, .exit, .keys.log); repo/ and state/ as the run left them"; \
      [[ -f "{{out_dir}}/test.log" ]]               && echo "- repo/ (the fixture paper, git init and one commit), requests.fifo and frames.fifo (the pipes the server read and wrote), repo/kb-root and state/ as the probe's two builds left them, mcp.log (the server's stderr) and test.log (the probe's go test output)"; \
      [[ -f "{{out_dir}}/docs/04-score.out" ]]      && echo "- docs/: per step its .out, .log and .exit; repo/ (the staged KB), config/ (the three seeded files) and state/ as the run left them; comparison.yaml: the fixture, the KB, the configuration and the run's outcome"; \
      compgen -G "{{out_dir}}/*/checks/kbase-status.out" > /dev/null && echo "- comparison.yaml: per paper every Direction 1 check, pass or fail, its detail and evidence path"; \
      compgen -G "{{out_dir}}/*/checks/kbase-status.out" > /dev/null && echo "- <id>/: repo/ (the fixture repository kbase built), built/ (kb-root/ and the build records as kbase's build left them), state/ (progress.jsonl, records/), config/ (empty), checks/ (per command .out, .log, .exit); on the special paper also kill/, guard/ and stamp/ (each repo/ and state/), kill/kill.txt, kill/comparison.yaml and kill/compare.log (the resumed build against built/ modulo node ids)"; \
      compgen -G "{{out_dir}}/*/nodepass.json" > /dev/null && echo "- comparison.yaml: the paper's checks, its state-dir, the scratch/ census and the counts (paragraphs asked, claims minted, defaults by cause; the unmarked pairs planned and asked, their letters and outcomes, the source groups and calls; candidates by harvest, by letter and outcome, own-equation pairs dropped, edges a ring demoted); <id>/: repo/ (the fixture repository the build wrote), state/ (progress.jsonl, records/, scratch/captures/, scratch/answers/, scratch/asks/), counts.yaml, nodepass.json, kb-build-node-pass.yaml and unmarked-plan.json (the node-pass record and the planned pairs), checks/ (per command .out, .log, .exit); models.*: the provider's catalogue, read before the build"; \
      true; \
    } > "{{out_dir}}/EVIDENCE.md"

# Fetch and unpack happen in one recipe because the corpus a test reads is the
# unpacked tree. Both steps land atomically (a .part file, a .unpack dir, then
# a rename), so an interrupted run leaves nothing a later run mistakes for done.
# An e-print is usually a tarball but may be a single gzipped .tex — a one-file
# submission has nothing to tar — and that lands as paper.tex.
[doc("fetch and unpack the pinned arXiv sample, one source tree per id under test-data/transient/arxiv/ (no-op per id when present); pass ids= to take a subset")]
prep-test-integration-arxiv ids=ARXIV_IDS:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p "{{ARXIV_TGZ_DIR}}" "{{ARXIV_DIR}}"
    fetched=0; unpacked=0; present=0
    for id in {{ids}}; do
        # A pre-2007 id carries a `/` (math/0211159v1); `_` keeps one paper to
        # one file and one directory.
        name="$(printf '%s' "${id}" | tr '/' '_')"
        tgz="{{ARXIV_TGZ_DIR}}/${name}.tar.gz"
        dir="{{ARXIV_DIR}}/${name}"
        if [[ -d "${dir}" ]]; then
            present=$((present + 1))
            continue
        fi
        if [[ ! -s "${tgz}" ]]; then
            printf 'fetching %s\n' "${id}"
            curl -fsSL -o "${tgz}.part" "https://arxiv.org/e-print/${id}" || {
                rm -f "${tgz}.part"
                printf 'error: %s could not be fetched\n' "${id}" >&2
                exit 1
            }
            mv "${tgz}.part" "${tgz}"
            fetched=$((fetched + 1))
            sleep "{{ARXIV_DELAY}}"
        fi
        rm -rf "${dir}.unpack"
        mkdir -p "${dir}.unpack"
        if ! tar xzf "${tgz}" -C "${dir}.unpack" 2>/dev/null; then
            rm -rf "${dir}.unpack"
            mkdir -p "${dir}.unpack"
            gunzip -c "${tgz}" > "${dir}.unpack/paper.tex"
        fi
        mv "${dir}.unpack" "${dir}"
        unpacked=$((unpacked + 1))
    done
    printf 'arxiv: %s fetched, %s unpacked, %s already present → %s\n' "${fetched}" "${unpacked}" "${present}" "{{ARXIV_DIR}}"

# The one statement of how a paper's volume root is found, read by every recipe
# that builds a paper: 00README.json's single "toplevel" entry, else the sole
# top-level .tex file containing \documentclass. Prints the root relative to
# dir; anything else is an error naming what was found.
[private]
_arxiv-volume-root dir:
    #!/usr/bin/env bash
    set -euo pipefail
    dir="{{dir}}"
    readme="${dir}/00README.json"
    if [[ -f "${readme}" ]]; then
        toplevel="$(python3 -c 'import json, sys; data = json.load(open(sys.argv[1])); names = [s.get("filename", "") for s in data.get("sources", []) if s.get("usage") == "toplevel"]; print(names[0] if len(names) == 1 and names[0] else "")' "${readme}" 2>/dev/null || true)"
        if [[ -n "${toplevel}" && -f "${dir}/${toplevel}" ]]; then
            printf '%s\n' "${toplevel}"
            exit 0
        fi
    fi
    candidates=()
    shopt -s nullglob
    for f in "${dir}"/*.tex; do
        if grep -qF '\documentclass' "${f}"; then
            candidates+=("$(basename "${f}")")
        fi
    done
    if [[ "${#candidates[@]}" -ne 1 ]]; then
        printf 'error: %s: 00README.json names no single toplevel source, and %s top-level .tex file(s) contain \\documentclass\n' "${dir}" "${#candidates[@]}" >&2
        exit 2
    fi
    printf '%s\n' "${candidates[0]}"

# kb-testing's stage-fixture, for a fixture repository the owner staged by hand:
# a caller-named directory, never a path this file knows. The scratch directory
# it removes is the installed harness's project-temp-dir, read by adjagent's
# _harness-value for the harness `install` defaults to (claude); the run-lock
# guard asks `kbase status`, which reads the lock in the state store the
# fixture's kb-root/ keys to. The reset runs on the fixture, and the fixture is
# refused if it is this repository.
[doc("NEEDS git and the adjagent clone (excluded from test-integration); DESTRUCTIVE to dir: reset a hand-staged fixture repository to its `cleared` tag, reinstall the current agent set and move the `staged` tag; dir is absolute or relative to the invocation directory; refuses a dir that is not a git repository, has no `cleared` tag (tag the commit before any agent-set install or build commit by hand) or has a kbase build running")]
prep-test-integration-fixture dir: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd -P)"
    dir="{{dir}}"
    [[ "${dir}" == /* ]] || dir="{{invocation_directory()}}/${dir}"
    if [[ ! -e "${dir}/.git" ]]; then
        printf 'error: %s is not a git repository — this recipe re-anchors an already-committed fixture, it does not create one.\n' "${dir}" >&2
        exit 1
    fi
    dir="$(cd "${dir}" && pwd -P)"
    if [[ "${dir}" == "${root}" ]]; then
        printf 'error: %s is this repository — the reset is for a fixture copy only.\n' "${dir}" >&2
        exit 1
    fi
    if ! git -C "${dir}" rev-parse -q --verify refs/tags/cleared > /dev/null; then
        printf 'error: %s carries no `cleared` tag (the commit before any agent-set install or build commit) — the owner tags it by hand before staging.\n' "${dir}" >&2
        exit 1
    fi
    if ! status="$(cd "${dir}" && "${root}/bin/kbase" status)"; then
        printf 'error: kbase status failed over %s — cannot tell whether a build is running.\n' "${dir}" >&2
        exit 1
    fi
    if grep -q '^state: "running"' <<< "${status}"; then
        printf 'error: a kbase build is running over %s — cancel it (kbase cancel) or wait.\n' "${dir}" >&2
        exit 1
    fi
    scratch="$("{{just_executable()}}" --justfile .claude/adjagent/justfile _harness-value claude project-temp-dir)"
    printf 'staging %s: resetting to `cleared` — discards every commit above it (the installed agent set, the build ledger), uncommitted changes, kb-root/ and %s/ — then reinstalling the current agent set.\n' "${dir}" "${scratch}" >&2
    git -C "${dir}" reset --hard cleared > /dev/null
    rm -rf "${dir}/${scratch}" "${dir}/kb-root"
    just --justfile .claude/adjagent/justfile install "${dir}"
    git -C "${dir}" add -A
    git -C "${dir}" -c user.name=kb-testing -c user.email=kb-testing@invalid commit -qm "agent defs installed"
    git -C "${dir}" tag -f staged > /dev/null
    printf 'staged %s\n' "${dir}"

# §8.3 Direction 1: a kbase-built KB. Per slice paper, a fresh fixture
# repository (the sources as fetched, a stub Makefile, a .gitignore) is built
# by ./bin/kbase --no-inference to completion; the build's ledger, its owned
# paths and the readiness documents are checked as built, and ./bin/kbase
# status and the build result are parsed against their documented keys. On
# BUILD_ARXIV_SPECIAL_ID: a build killed with SIGKILL while
# claims-declared's writes are in flight, its status read, then resumed
# to a tree and records equal to the uninterrupted build's modulo node ids
# (TestCompareReference with only a rerun named); a fresh
# build over that tree in a repository with no trail refused; and
# CONVENTIONS.md authored before phase-3a and left as it is. Every
# result lands in comparison.yaml.
[doc("NEEDS pandoc + python3 (excluded from test-integration): §8.3 Direction 1 — per slice paper ./bin/kbase build --no-inference in a fresh fixture repository, then the build's ledger and owned paths, the readiness documents, ./bin/kbase status and its result document checked; kill-and-resume, the double-run guard and stamping over an authored document on one paper; evidence under test-data/transient/test-integration-build-arxiv/")]
test-integration-build-arxiv: (prep-test-integration-arxiv BUILD_ARXIV_IDS)
    @mkdir -p "{{BUILD_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-arxiv 2>&1 | tee "{{BUILD_ARXIV_OUT_DIR}}/log.txt"

[private]
_test-integration-build-arxiv: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{BUILD_ARXIV_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    pandoc_version="$(pandoc --version | head -n 1)"
    printf 'pandoc: %s\n' "${pandoc_version}"
    # capture <stem> <dir> <command...>: run the command in dir, stdout to
    # <stem>.out, stderr to <stem>.log, exit code to <stem>.exit.
    capture() {
        local stem="$1" dir="$2"
        shift 2
        set +e
        ( cd "${dir}" && "$@" ) > "${stem}.out" 2> "${stem}.log"
        printf '%s\n' "$?" > "${stem}.exit"
        set -e
    }
    rc() { cat "$1.exit"; }
    # stage <repo> <name>: a fresh fixture repository holding the paper's
    # sources, committed.
    stage() {
        local repo="$1" name="$2"
        mkdir -p "${repo}"
        cp -R "{{ARXIV_DIR}}/${name}/." "${repo}/"
        printf 'default:\n\t@echo %s\n\ntest:\n\t@echo tests passed.\n' "${name}" > "${repo}/Makefile"
        printf '%s\n' '.claude-temp/' '__pycache__/' > "${repo}/.gitignore"
        git -C "${repo}" init -q
        git -C "${repo}" add -A
        git -C "${repo}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "sources as fetched"
    }
    # check <name> <passed: 0 or 1> <detail> <evidence>: one result.
    checks=()
    check() {
        local verdict=pass
        [[ "$2" -eq 0 ]] || { verdict=fail; status=1; }
        checks+=("$1"$'\t'"${verdict}"$'\t'"$3"$'\t'"$4")
        printf '  %s %s: %s\n' "${verdict}" "$1" "$3"
    }
    yaml_str() { local s="${1//\\/\\\\}"; printf '"%s"' "${s//\"/\\\"}"; }
    ok() { "$@" > /dev/null 2>&1 && echo 0 || echo 1; }

    status=0
    comparison="${out}/comparison.yaml"
    : > "${comparison}"
    for id in {{BUILD_ARXIV_IDS}}; do
        name="$(printf '%s' "${id}" | tr '/' '_')"
        dir="${out}/${name}"
        repo="${dir}/repo"
        ck="${dir}/checks"
        rm -rf "${dir}" "${dir}".*
        mkdir -p "${ck}" "${dir}/config" "${dir}/built"
        checks=()
        printf '%s\n' "${id}"
        stage "${repo}" "${name}"
        volume_root="$({{just_executable()}} _arxiv-volume-root "${repo}")"
        bibs=()
        while IFS= read -r bib; do
            bibs+=(--bibliography "${bib}")
        done < <(cd "${repo}" && find "$(dirname "${volume_root}")" -maxdepth 1 -name '*.bib' | sed 's|^\./||' | LC_ALL=C sort)
        build_args=(build "${volume_root}" --no-inference --config-dir "${root}/${dir}/config" ${bibs[@]+"${bibs[@]}"})

        # the build, to completion
        capture "${ck}/kbase-build" "${repo}" "${kbase}" "${build_args[@]}" --state-dir "${root}/${dir}/state"
        check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-build.out" && echo 0 || echo 1)" \
            "exit $(rc "${ck}/kbase-build"), $(grep -m1 '^outcome:' "${ck}/kbase-build.out" || echo 'no outcome')" "${ck}/kbase-build.out"
        cp -R "${repo}/kb-root" "${dir}/built/"
        cp "${repo}"/kb-build-*.yaml "${dir}/built/" 2>/dev/null || true
        check ledger "$(ok test "$(git -C "${repo}" log --format=%s --grep='^kb-build: ' | wc -l | tr -d ' ')" -eq 10)" \
            "$(git -C "${repo}" log --format=%s --grep='^kb-build: ' | wc -l | tr -d ' ') kb-build: commits, one per stage" "${repo}/.git"
        check ledger-scope "$(ok test -z "$(git -C "${repo}" status --porcelain -- kb-root kb-build-node-pass.yaml kb-build-classification.yaml kb-build-unmarked.yaml)")" \
            "the owned paths are clean after the build" "${repo}"

        # readiness documents, as built
        check claude-redirect "$(ok cmp -s <(printf '@AGENTS.md\n') "${dir}/built/kb-root/CLAUDE.md")" "kb-root/CLAUDE.md is exactly @AGENTS.md" "${dir}/built/kb-root/CLAUDE.md"
        check no-invariant-heading "$(ok test -z "$(grep -l '^### INVARIANT-' "${dir}"/built/kb-root/{AGENTS,CONVENTIONS,CLAUDE}.md 2>/dev/null)")" \
            "no stamped document holds a ### INVARIANT- heading" "${dir}/built/kb-root"
        check overview-absent "$(ok test ! -e "${dir}/built/kb-root/README.md")" "no README.md, as a --no-inference build writes none" "${dir}/built/kb-root"

        # status at the end state
        capture "${ck}/kbase-status" "${repo}" "${kbase}" status --state-dir "${root}/${dir}/state"
        set +e
        go test -count=1 -run '^TestStatusDocument$' ./internal/build -args -build.status="${root}/${ck}/kbase-status.out" -build.state=finished > "${ck}/kbase-status.check.log" 2>&1
        check status-finished "$?" "status parses and reports the finished build" "${ck}/kbase-status.check.log"
        for verb in build status; do
            go test -count=1 -run '^TestResultDocument$' ./cmd -args -result.doc="${root}/${ck}/kbase-${verb}.out" -result.verb="${verb}" > "${ck}/kbase-${verb}.keys.log" 2>&1
            check "${verb}-keys" "$?" "the ${verb} result parses against its documented keys" "${ck}/kbase-${verb}.keys.log"
        done
        set -e

        if [[ "${id}" == "{{BUILD_ARXIV_SPECIAL_ID}}" ]]; then
            # kill-and-resume: SIGKILL while claims-declared's writes are in flight
            kill_dir="${dir}/kill"
            stage "${kill_dir}/repo" "${name}"
            ( cd "${kill_dir}/repo" && exec "${kbase}" "${build_args[@]}" --state-dir "${root}/${kill_dir}/state" ) \
                > "${kill_dir}/first.out" 2> "${kill_dir}/first.log" &
            pid=$!
            landed=missed
            for _ in $(seq 1 20000); do
                if grep -q '"stage":"claims-declared"' "${kill_dir}/state/progress.jsonl" 2>/dev/null \
                    && [[ -n "$(git -C "${kill_dir}/repo" status --porcelain -- kb-root kb-build-node-pass.yaml kb-build-classification.yaml kb-build-unmarked.yaml 2>/dev/null)" ]]; then
                    kill -9 "${pid}" 2>/dev/null && landed=mid-stage
                    break
                fi
                kill -0 "${pid}" 2>/dev/null || break
            done
            wait "${pid}" 2>/dev/null || true
            dirt="$(git -C "${kill_dir}/repo" status --porcelain -- kb-root kb-build-node-pass.yaml kb-build-classification.yaml kb-build-unmarked.yaml)"
            recorded="$(git -C "${kill_dir}/repo" log --format=%s --grep='^kb-build: claims-declared ')"
            printf 'landed: %s\ndirt:\n%s\nclaims-declared recorded: %s\n' "${landed}" "${dirt}" "${recorded:-no}" > "${kill_dir}/kill.txt"
            check kill-mid-stage "$(ok test "${landed}" = mid-stage -a -n "${dirt}" -a -z "${recorded}")" \
                "killed inside claims-declared with its writes uncommitted" "${kill_dir}/kill.txt"
            capture "${kill_dir}/status" "${kill_dir}/repo" "${kbase}" status --state-dir "${root}/${kill_dir}/state"
            set +e
            go test -count=1 -run '^TestStatusDocument$' ./internal/build -args -build.status="${root}/${kill_dir}/status.out" -build.state=interrupted > "${kill_dir}/status.check.log" 2>&1
            check status-killed "$?" "status after the kill parses and reports interrupted in claims-declared with a resume command" "${kill_dir}/status.check.log"
            set -e
            capture "${kill_dir}/resume" "${kill_dir}/repo" "${kbase}" "${build_args[@]}" --state-dir "${root}/${kill_dir}/state"
            check kill-resume "$([[ "$(rc "${kill_dir}/resume")" -eq 0 ]] && grep -qx 'resumed: true' "${kill_dir}/resume.out" \
                && grep -A1 '^restored:' "${kill_dir}/resume.out" | grep -q '^    - ' && echo 0 || echo 1)" \
                "resume exit $(rc "${kill_dir}/resume"), restoring the interrupted stage's paths" "${kill_dir}/resume.out"
            set +e
            go test -count=1 -run '^TestCompareReference$' ./internal/claimgraph -args -claimgraph.kbase="${root}/${dir}/built" \
                -claimgraph.rerun="${root}/${kill_dir}/repo" -claimgraph.comparison="${root}/${kill_dir}/comparison.yaml" > "${kill_dir}/compare.log" 2>&1
            check kill-resume-modulo-ids "$?" "kb-root/ and the build records equal to the uninterrupted build's modulo node ids" "${kill_dir}/comparison.yaml"
            set -e

            # the double-run guard: a fresh build over a written tree
            guard_dir="${dir}/guard"
            stage "${guard_dir}/repo" "${name}"
            cp -R "${dir}/built/kb-root" "${guard_dir}/repo/"
            capture "${guard_dir}/build" "${guard_dir}/repo" "${kbase}" "${build_args[@]}" --state-dir "${root}/${guard_dir}/state"
            check double-run-guard "$([[ "$(rc "${guard_dir}/build")" -eq 1 ]] && grep -q 'is populated' "${guard_dir}/build.out" && echo 0 || echo 1)" \
                "exit $(rc "${guard_dir}/build"), refused over a populated kb-root/ with no trail" "${guard_dir}/build.out"

            # stamping only if absent
            stamp_dir="${dir}/stamp"
            stage "${stamp_dir}/repo" "${name}"
            capture "${stamp_dir}/bounded" "${stamp_dir}/repo" "${kbase}" "${build_args[@]}" --state-dir "${root}/${stamp_dir}/state" \
                --through depends-attributed
            printf '# Conventions this project wrote for itself\n' > "${stamp_dir}/repo/kb-root/CONVENTIONS.md"
            git -C "${stamp_dir}/repo" add kb-root/CONVENTIONS.md
            git -C "${stamp_dir}/repo" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "the project's own conventions"
            capture "${stamp_dir}/resume" "${stamp_dir}/repo" "${kbase}" "${build_args[@]}" --state-dir "${root}/${stamp_dir}/state"
            check stamp-only-if-absent "$([[ "$(rc "${stamp_dir}/bounded")" -eq 0 && "$(rc "${stamp_dir}/resume")" -eq 0 ]] \
                && cmp -s <(printf '# Conventions this project wrote for itself\n') "${stamp_dir}/repo/kb-root/CONVENTIONS.md" \
                && grep -q 'CONVENTIONS.md: present, left as authored' "${stamp_dir}/state/reports/phase-3a.yaml" && echo 0 || echo 1)" \
                "an authored CONVENTIONS.md left as it is" "${stamp_dir}/state/reports/phase-3a.yaml"
            check bounded-no-resume "$(ok test -z "$(grep '^resume:' "${stamp_dir}/bounded.out")")" \
                "the bounded result carries no resume command" "${stamp_dir}/bounded.out"
        fi

        {
            printf -- '- id: %s\n  checks:\n' "$(yaml_str "${id}")"
            for c in "${checks[@]}"; do
                IFS=$'\t' read -r c_name c_verdict c_detail c_evidence <<< "${c}"
                printf '    - check: %s\n      result: %s\n      detail: %s\n      evidence: %s\n' \
                    "$(yaml_str "${c_name}")" "$(yaml_str "${c_verdict}")" "$(yaml_str "${c_detail}")" "$(yaml_str "${c_evidence}")"
            done
        } >> "${comparison}"
    done

    {{just_executable()}} _write-evidence "${out}" "test-integration-build-arxiv" \
      "per paper <id> of {{BUILD_ARXIV_IDS}}, staged into <id>/repo (sources as fetched, stub Makefile, .gitignore, one commit):\n(cd <id>/repo && ${kbase} build <volume-root> --no-inference --config-dir <id>/config --state-dir <id>/state [--bibliography <each .bib beside the volume root, sorted>]) into <id>/checks/kbase-build.*\n<id>/built/kb-root and the three kb-build-*.yaml files as the build left them; the ledger read from <id>/repo/.git\n<id>/built/kb-root's AGENTS.md, CONVENTIONS.md and CLAUDE.md as built, checked against the readiness rules\n${kbase} status --state-dir <id>/state, parsed by go test -run '^TestStatusDocument\$' ./internal/build; the build and status results parsed by go test -run '^TestResultDocument\$' ./cmd\non {{BUILD_ARXIV_SPECIAL_ID}}: kill -9 inside claims-declared, status, resume (<id>/kill/), then go test -run '^TestCompareReference\$' ./internal/claimgraph -args -claimgraph.kbase=<id>/built -claimgraph.rerun=<id>/kill/repo -claimgraph.comparison=<id>/kill/comparison.yaml; a fresh build over the built tree with no trail (<id>/guard/); --through depends-attributed, an authored CONVENTIONS.md committed, then the resume (<id>/stamp/)"
    printf '\nReference: %s\n' "${pandoc_version}" >> "${out}/EVIDENCE.md"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-build-arxiv): a check failed — see comparison.yaml\n'
        exit 1
    fi
    printf 'integration(test-integration-build-arxiv) ok\n'

[doc("LIVE — NEEDS the local inference endpoint test-data/fixtures/config/ names, pandoc and python3 (excluded from test-integration): ./bin/kbase build without --no-inference over the Slice-1 paper BUILD_ARXIV_SPECIAL_ID names, in a fresh fixture repository; README.md with its passage; the asks' captures, cached answers and group records under the state store's scratch/; a second build through claims-discovered configured by KBASE_API_BASE_URL, KBASE_DECISION, KBASE_TEXT and KBASE_API_KEY_FILE alone, no configuration directory; config= names another configuration directory; a third build naming no configuration directory at all, with HOME on a scratch home holding the fixture's providers.toml as the global layer and configure --create --project dialing reaper out of it and writing three references into the repository layer, green with its base URL sourced global and its provider and each model sourced repository, its show-config the evidence; and a score run configured by the environment alone — KBASE_API_BASE_URL and the three role variables, no configuration directory — one item on the scoring model, which differs from the text model and is the only model the ledger names; counts in comparison.yaml; evidence under test-data/transient/test-integration-build-live-arxiv/")]
test-integration-build-live-arxiv config=LIVE_CONFIG_DIR: (prep-test-integration-arxiv BUILD_ARXIV_SPECIAL_ID)
    @mkdir -p "{{BUILD_LIVE_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-live-arxiv "{{config}}" 2>&1 | tee "{{BUILD_LIVE_ARXIV_OUT_DIR}}/log.txt"

[private]
_test-integration-build-live-arxiv config: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{BUILD_LIVE_ARXIV_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    config="{{config}}"
    [[ "${config}" == /* ]] || config="${root}/${config}"
    id="{{BUILD_ARXIV_SPECIAL_ID}}"
    pandoc_version="$(pandoc --version | head -n 1)"
    printf 'pandoc: %s\n' "${pandoc_version}"
    # capture <stem> <dir> <command...>: run the command in dir, stdout to
    # <stem>.out, stderr to <stem>.log, exit code to <stem>.exit.
    capture() {
        local stem="$1" dir="$2"
        shift 2
        set +e
        ( cd "${dir}" && "$@" ) > "${stem}.out" 2> "${stem}.log"
        printf '%s\n' "$?" > "${stem}.exit"
        set -e
    }
    rc() { cat "$1.exit"; }
    checks=()
    status=0
    check() {
        local verdict=pass
        [[ "$2" -eq 0 ]] || { verdict=fail; status=1; }
        checks+=("$1"$'\t'"${verdict}"$'\t'"$3"$'\t'"$4")
        printf '  %s %s: %s\n' "${verdict}" "$1" "$3"
    }
    yaml_str() { local s="${1//\\/\\\\}"; printf '"%s"' "${s//\"/\\\"}"; }
    ok() { "$@" > /dev/null 2>&1 && echo 0 || echo 1; }
    count() { find "$1" -type f -name "$2" 2>/dev/null | wc -l | tr -d ' '; }

    # The endpoint first: an unreachable provider fails here, by name, and no
    # build is started against it. The fixture's pool holds two entries and its
    # config.toml's provider key is superseded, so the entry is named.
    provider="$(sed -n 's/^provider *= *"\(.*\)".*/\1/p' "${config}/config.toml" | head -n 1)"
    capture "${out}/models" "${root}" "${kbase}" models --config-dir "${config}" --provider "${provider}"
    if [[ "$(rc "${out}/models")" -ne 0 ]]; then
        printf 'integration(test-integration-build-live-arxiv): provider %s, named in %s/config.toml, is unreachable — see %s\n' \
            "${provider:-(none named)}" "{{config}}" "${out}/models.out"
        exit 1
    fi

    name="$(printf '%s' "${id}" | tr '/' '_')"
    dir="${out}/${name}"
    repo="${dir}/repo"
    ck="${dir}/checks"
    state="${root}/${dir}/state"
    rm -rf "${dir}"
    mkdir -p "${ck}"
    mkdir -p "${repo}"
    cp -R "{{ARXIV_DIR}}/${name}/." "${repo}/"
    printf 'default:\n\t@echo %s\n\ntest:\n\t@echo tests passed.\n' "${name}" > "${repo}/Makefile"
    printf '%s\n' '.claude-temp/' '__pycache__/' > "${repo}/.gitignore"
    git -C "${repo}" init -q
    git -C "${repo}" add -A
    git -C "${repo}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "sources as fetched"
    volume_root="$({{just_executable()}} _arxiv-volume-root "${repo}")"
    printf '%s\n' "${id}"

    # The fixture's config.toml is in the old grammar — bare model ids beside a
    # superseded provider key — and file-mode resolution refuses a bare id. The
    # file-mode legs read a reference-shaped copy in this run's directory: the
    # same pool and keys, and three references naming that pool's entry. The
    # environment leg below still reads the fixture's bare ids, which is what
    # the environment names.
    toml_get() { awk -v t="[$2]" -v k="$3" '/^\[/ { in_t = ($0 == t) } in_t && $1 == k { sub(/^[^=]*= *"/, ""); sub(/".*/, ""); print; exit }' "$1"; }
    base_url="$(toml_get "${config}/providers.toml" "${provider}" baseUrl)"
    key_file="$(toml_get "${config}/providers.toml" "${provider}" apiKeyFile)"
    [[ -z "${key_file}" || "${key_file}" == /* ]] || key_file="${config}/${key_file}"
    decision="$(toml_get "${config}/config.toml" models decision)"
    text="$(toml_get "${config}/config.toml" models text)"
    # The fixture declares no scoring model; the decision model judges the KB.
    scoring="${decision}"
    file_config="${dir}/file-config"
    mkdir -p "${file_config}"
    cp "${config}/providers.toml" "${file_config}/providers.toml"
    cp -R "${config}/api_keys" "${file_config}/api_keys"
    printf '%s\n' '[models]' "decision = \"${provider}/${decision}\"" "text = \"${provider}/${text}\"" "scoring = \"${provider}/${scoring}\"" \
        > "${file_config}/config.toml"

    # the build, asking, to completion
    capture "${ck}/kbase-build" "${repo}" "${kbase}" build "${volume_root}" --config-dir "${file_config}" --state-dir "${state}" --log-level info
    check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-build.out" && echo 0 || echo 1)" \
        "exit $(rc "${ck}/kbase-build"), $(grep -m1 '^outcome:' "${ck}/kbase-build.out" || echo 'no outcome')" "${ck}/kbase-build.out"
    check ledger "$(ok test "$(git -C "${repo}" log --format=%s --grep='^kb-build: ' | wc -l | tr -d ' ')" -eq 11)" \
        "$(git -C "${repo}" log --format=%s --grep='^kb-build: ' | wc -l | tr -d ' ') kb-build: commits, one per stage" "${repo}/.git"
    check no-row-dropped "$(ok test -z "$(git -C "${repo}" log --format=%b --grep='^kb-build: ' | grep -- '--no-inference')")" \
        "no boundary names a dropped row" "${repo}/.git"
    check overview "$(ok grep -q '^# .* Knowledge Base$' "${repo}/kb-root/README.md")" "README.md written around its passage" "${repo}/kb-root/README.md"
    captures="$(count "${state}/scratch/captures" '*.capture.jsonl')"
    answers="$(count "${state}/scratch/answers" '*.txt')"
    ask_records="$(count "${state}/scratch/asks" '*.yaml')"
    check captures "$(ok test "${captures}" -gt 0 -a "${answers}" -gt 0 -a "${ask_records}" -gt 0)" \
        "${captures} captures, ${answers} cached answers, ${ask_records} group records under the state store's scratch/" "${state}/scratch"

    # the counts, off the build records and the result
    set +e
    go test -count=1 -run '^TestLiveBuildCounts$' ./internal/claimgraph -args -claimgraph.repo="${root}/${repo}" \
        -claimgraph.result="${root}/${ck}/kbase-build.out" -claimgraph.out="${root}/${dir}" > "${ck}/counts.log" 2>&1
    check records "$?" "every leaf landed, every planned pair answered, no candidate drafted; counts.yaml and nodepass.json written" "${ck}/counts.log"
    set -e

    # The same server and model configured by the environment alone: no
    # --config-dir, the configuration directory absent, through the node pass
    # so the build asks. The environment names bare ids, as it does by
    # definition; the fixture's own values are what it carries.
    env_repo="${dir}/env-repo"
    no_config="${root}/${dir}/no-config"
    mkdir -p "${env_repo}"
    cp -R "{{ARXIV_DIR}}/${name}/." "${env_repo}/"
    git -C "${env_repo}" init -q
    capture "${ck}/kbase-build-env" "${env_repo}" env KBASE_CONFIG_DIR="${no_config}" KBASE_API_BASE_URL="${base_url}" \
        KBASE_DECISION="${decision}" KBASE_TEXT="${text}" KBASE_API_KEY_FILE="${key_file}" "${kbase}" build "${volume_root}" \
        --through claims-discovered --state-dir "${root}/${dir}/env-state" --log-level info
    warned="$(grep -c 'travels in cleartext' "${ck}/kbase-build-env.log" || true)"
    expect_warning=0
    [[ -n "${key_file}" && "${base_url}" == http://* && ! "${base_url}" =~ ^http://(localhost|127\.|\[::1\]) ]] && expect_warning=1
    check build-env "$([[ "$(rc "${ck}/kbase-build-env")" -eq 0 ]] && grep -qx 'outcome: "bounded"' "${ck}/kbase-build-env.out" \
        && [[ "$(count "${root}/${dir}/env-state/scratch/captures" '*.capture.jsonl')" -gt 0 && ! -e "${no_config}" && "${warned}" -eq "${expect_warning}" ]] && echo 0 || echo 1)" \
        "configured by KBASE_API_BASE_URL, KBASE_DECISION=${decision}, KBASE_TEXT=${text} and KBASE_API_KEY_FILE alone: exit $(rc "${ck}/kbase-build-env"), $(grep -m1 '^outcome:' "${ck}/kbase-build-env.out" || echo 'no outcome'), $(count "${root}/${dir}/env-state/scratch/captures" '*.capture.jsonl') captures, ${warned} cleartext warning(s) (${expect_warning} expected), no configuration directory" \
        "${ck}/kbase-build-env.out"

    # The layering proof, which names no configuration directory at all: every
    # leg above passes --config-dir, which is single-layer mode. The global
    # layer is the fixture's pool copied into this leg's own home — its
    # apiKeyFile paths resolve beside the file, so its keys travel with it —
    # and the repository layer is what configure --create --project writes,
    # dialing reaper out of that pool and writing a reference for each category
    # — decision and text from the fixture, scoring from its decision model.
    # The build then takes its endpoint from the global file and its three
    # references from the repository's, with the operator's own
    # ~/.config/kbase read by neither: HOME is this leg's directory, and the
    # variable that would name a directory, and the five that would supply a
    # field over the files, are unset.
    layered="${root}/${dir}/layered"
    layered_home="${layered}/home"
    layered_repo="${layered}/repo"
    mkdir -p "${layered_home}/.config/kbase" "${layered_repo}"
    cp "${config}/providers.toml" "${layered_home}/.config/kbase/providers.toml"
    # A relative apiKeyFile resolves beside its providers.toml, so the keys it
    # names travel with it or both entries drop; owner-only, because a key file
    # any other account can read is reported as a warning fault.
    cp -R "${config}/api_keys" "${layered_home}/.config/kbase/api_keys"
    chmod 0600 "${layered_home}/.config/kbase/api_keys/reaper.txt" "${layered_home}/.config/kbase/api_keys/imp.txt"
    # The build commits its boundaries with the ambient git identity, which this
    # home supplies in place of the operator's.
    printf '[user]\n\tname = kbase integration\n\temail = kbase@invalid\n' > "${layered_home}/.gitconfig"
    cp -R "{{ARXIV_DIR}}/${name}/." "${layered_repo}/"
    git -C "${layered_repo}" init -q
    layered_volume_root="$({{just_executable()}} _arxiv-volume-root "${layered_repo}")"
    layered_env=(-u KBASE_CONFIG_DIR -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE "HOME=${layered_home}")
    capture "${ck}/kbase-configure-layered" "${layered_repo}" env "${layered_env[@]}" "${kbase}" configure \
        --create --project --provider reaper \
        --model-map "decision=reaper/${decision},text=reaper/${text},scoring=reaper/${scoring}"
    layered_refs=0
    [[ "$(rc "${ck}/kbase-configure-layered")" -eq 0 ]] || layered_refs=1
    grep -qx 'outcome: "done"' "${ck}/kbase-configure-layered.out" || layered_refs=1
    grep -q '^    - ".*/\.kbase/config\.toml"$' "${ck}/kbase-configure-layered.out" || layered_refs=1
    grep -qxF "    decision: \"reaper/${decision}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
    grep -qxF "    text: \"reaper/${text}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
    grep -qxF "    scoring: \"reaper/${scoring}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
    if grep -q '^provider *=' "${layered_repo}/.kbase/config.toml"; then layered_refs=1; fi
    check configure-layered "${layered_refs}" \
        "reaper dialed from the global pool; three references written into the repository layer, and no provider key: exit $(rc "${ck}/kbase-configure-layered")" \
        "${ck}/kbase-configure-layered.out"
    capture "${ck}/kbase-build-layered" "${layered_repo}" env "${layered_env[@]}" "${kbase}" build "${layered_volume_root}" \
        --state-dir "${layered}/state" --log-level info
    layered_captures="$(count "${layered}/state/scratch/captures" '*.capture.jsonl')"
    check layered-build "$([[ "$(rc "${ck}/kbase-build-layered")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-build-layered.out" \
        && [[ "${layered_captures}" -gt 0 ]] && echo 0 || echo 1)" \
        "no --config-dir: exit $(rc "${ck}/kbase-build-layered"), $(grep -m1 '^outcome:' "${ck}/kbase-build-layered.out" || echo 'no outcome'), ${layered_captures} captures against the endpoint the global layer names" \
        "${ck}/kbase-build-layered.out"
    capture "${ck}/kbase-show-config-layered" "${layered_repo}" env "${layered_env[@]}" "${kbase}" show-config
    layered_show="${ck}/kbase-show-config-layered"
    layered_pool=0
    [[ "$(rc "${layered_show}")" -eq 0 ]] || layered_pool=1
    grep -qx 'outcome: "done"' "${layered_show}.out" || layered_pool=1
    # the two layers, the global one this leg's home — so no other directory
    # was read
    [[ "$(grep -cxF '      source: "global"' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
    [[ "$(grep -cxF '      source: "repository"' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
    grep -qxF "    - path: \"${layered_home}/.config/kbase\"" "${layered_show}.out" || layered_pool=1
    # the merged pool: the fixture's two entries out of the global file, the
    # template's [local] out of the repository one, every entry usable
    [[ "$(grep -c '^    - name: ' "${layered_show}.out")" -eq 3 ]] || layered_pool=1
    [[ "$(grep -cxF '      usable: true' "${layered_show}.out")" -eq 3 ]] || layered_pool=1
    [[ "$(grep -cxF '      layer: "global"' "${layered_show}.out")" -eq 2 ]] || layered_pool=1
    [[ "$(grep -cxF '      layer: "repository"' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
    for entry in imp local reaper; do
        grep -qxF "    - name: \"${entry}\"" "${layered_show}.out" || layered_pool=1
    done
    [[ "$(grep -cxF 'diagnostics: []' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
    check layered-pool "${layered_pool}" \
        "3 pool entries, imp and reaper global and local repository, all usable, no diagnostics; the global layer is ${layered_home}/.config/kbase" \
        "${layered_show}.out"
    # The repository's config.toml names three references, and the document
    # reports them verbatim; each role's record names the entry it asked, the
    # rule, the layer, and the layer that supplied each field: the endpoint and
    # the key from the global pool entry, the model from the repository's file.
    layered_sources=0
    grep -qxF "    decision: \"reaper/${decision}\"" "${layered_show}.out" || layered_sources=1
    grep -qxF "    text: \"reaper/${text}\"" "${layered_show}.out" || layered_sources=1
    grep -qxF "    scoring: \"reaper/${scoring}\"" "${layered_show}.out" || layered_sources=1
    for role in decision text scoring; do
        grep -qxF "    - role: \"${role}\"" "${layered_show}.out" || layered_sources=1
    done
    [[ "$(grep -cxF '      provider: "reaper"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
    [[ "$(grep -cxF '      provider-rule: "config"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
    [[ "$(grep -cxF '      provider-layer: "global"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
    [[ "$(grep -cxF "      base-url: \"${base_url}\"" "${layered_show}.out")" -eq 3 ]] || layered_sources=1
    [[ "$(grep -cxF '        base-url: "global"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
    [[ "$(grep -cxF '        model: "repository"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
    [[ "$(grep -cxF '        key: "global"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
    grep -qxF "      model: \"Qwen3.5-9B\"" "${layered_show}.out" || layered_sources=1
    [[ "$(grep -cxF "      model: \"${decision}\"" "${layered_show}.out")" -eq 2 ]] || layered_sources=1
    grep -qxF 'environment: []' "${layered_show}.out" || layered_sources=1
    check layered-sources "${layered_sources}" \
        "three role records, each reaper out of the global pool with its model from the repository's file: base-url and key global, model repository; decision and scoring ${decision}, text ${text}" \
        "${layered_show}.out"

    # The environment mode configures a score run with no configuration
    # directory: the base URL and the role variables alone, against the KB the
    # first build finished. One item is asked; the document and the ledger name
    # the scoring model, which differs from the text model, and the text model
    # appears nowhere in the run.
    score_home="${dir}/score-home"
    mkdir -p "${score_home}"
    score_env=(-u KBASE_CONFIG_DIR "HOME=${score_home}" "KBASE_API_BASE_URL=${base_url}"
        "KBASE_DECISION=${decision}" "KBASE_TEXT=${text}" "KBASE_SCORING=${scoring}")
    [[ -z "${key_file}" ]] || score_env+=("KBASE_API_KEY_FILE=${key_file}")
    capture "${ck}/kbase-score-env" "${repo}" env "${score_env[@]}" "${kbase}" score \
        --pass rigor --limit 1 --state-dir "${root}/${dir}/score-state"
    score_ok=0
    [[ "$(rc "${ck}/kbase-score-env")" -eq 0 ]] || score_ok=1
    grep -qx 'outcome: "done"' "${ck}/kbase-score-env.out" || score_ok=1
    grep -qx '    - role: "scoring"' "${ck}/kbase-score-env.out" || score_ok=1
    grep -qx '      back-end: "environment"' "${ck}/kbase-score-env.out" || score_ok=1
    grep -qxF "      model: \"${scoring}\"" "${ck}/kbase-score-env.out" || score_ok=1
    [[ "$(sed -n 's/^    done: //p' "${ck}/kbase-score-env.out")" -eq 1 ]] || score_ok=1
    score_ledger="$(sed -n 's/^ledger: "\(.*\)"$/\1/p' "${ck}/kbase-score-env.out" | head -n 1)"
    [[ -n "${score_ledger}" && -f "${score_ledger}/ledger.jsonl" ]] || score_ok=1
    grep -qF "\"model\":\"${scoring}\"" "${score_ledger}/ledger.jsonl" || score_ok=1
    # The text model appears nowhere the run asked. The resume record is the one
    # place it stands: an environment-mode command re-issues the whole
    # configuration it read, text's variable included.
    if grep -v '^resume:' "${ck}/kbase-score-env.out" | grep -qF "${text}"; then score_ok=1; fi
    if grep -qF "${text}" "${score_ledger}/ledger.jsonl"; then score_ok=1; fi
    check score-env "${score_ok}" \
        "environment mode configured a score run with no configuration directory: one item asked on ${scoring} (the text model ${text} appears nowhere the run asked), against ${base_url}" \
        "${ck}/kbase-score-env.out"

    {
        printf -- '- id: %s\n  state-dir: %s\n  scratch:\n    captures: %s\n    answers: %s\n    ask-records: %s\n  checks:\n' \
            "$(yaml_str "${id}")" "$(yaml_str "${state}")" "${captures}" "${answers}" "${ask_records}"
        for c in "${checks[@]}"; do
            IFS=$'\t' read -r c_name c_verdict c_detail c_evidence <<< "${c}"
            printf '    - check: %s\n      result: %s\n      detail: %s\n      evidence: %s\n' \
                "$(yaml_str "${c_name}")" "$(yaml_str "${c_verdict}")" "$(yaml_str "${c_detail}")" "$(yaml_str "${c_evidence}")"
        done
        [[ -f "${dir}/counts.yaml" ]] && sed 's/^/  /' "${dir}/counts.yaml"
    } > "${out}/comparison.yaml"

    {{just_executable()}} _write-evidence "${out}" "test-integration-build-live-arxiv" \
      "${kbase} models --config-dir <config> --provider <the fixture's provider> into models.*\n<id>/file-config: the fixture's providers.toml and api_keys with a config.toml naming three references (decision and text from the fixture, scoring from its decision model), because the fixture's own [models] values are bare ids the file mode refuses\non {{BUILD_ARXIV_SPECIAL_ID}}, staged into <id>/repo (sources, stub Makefile, .gitignore):\n(cd <id>/repo && ${kbase} build <volume-root> --config-dir <id>/file-config --state-dir <id>/state --log-level info) into <id>/checks/kbase-build.*\ngo test -run '^TestLiveBuildCounts\$' ./internal/claimgraph -args -claimgraph.repo=<id>/repo -claimgraph.result=<id>/checks/kbase-build.out -claimgraph.out=<id>\n(cd <id>/env-repo && env KBASE_CONFIG_DIR=<id>/no-config KBASE_API_BASE_URL=<the provider's baseUrl> KBASE_DECISION=<config's models.decision> KBASE_TEXT=<config's models.text> KBASE_API_KEY_FILE=<its apiKeyFile> ${kbase} build <volume-root> --through claims-discovered --state-dir <id>/env-state --log-level info) into <id>/checks/kbase-build-env.*\n(cd <id>/layered/repo && env -u KBASE_CONFIG_DIR -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE HOME=<id>/layered/home ${kbase} configure --create --project --provider reaper --model-map decision=reaper/<config's models.decision>,text=reaper/<config's models.text>,scoring=reaper/<its decision model>; ${kbase} build <volume-root> --state-dir <id>/layered/state --log-level info; ${kbase} show-config) into <id>/checks/kbase-configure-layered.*, kbase-build-layered.* and kbase-show-config-layered.*, in <id>/layered/repo below <id>/layered/home/.config/kbase (the fixture's providers.toml and its api_keys, the global layer, over .kbase/ that configure wrote, the repository layer)\n(cd <id>/repo && env -u KBASE_CONFIG_DIR -u KBASE_API_KEY_FILE HOME=<id>/score-home KBASE_API_BASE_URL=<the provider's baseUrl> KBASE_DECISION=<config's models.decision> KBASE_TEXT=<config's models.text> KBASE_SCORING=<its decision model> [KBASE_API_KEY_FILE=<its apiKeyFile>] ${kbase} score --pass rigor --limit 1 --state-dir <id>/score-state) into <id>/checks/kbase-score-env.*: environment mode with no configuration directory, one item asked on the scoring model, the ledger naming it and no line naming the text model"
    printf '\nReference: %s\n' "${pandoc_version}" >> "${out}/EVIDENCE.md"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-build-live-arxiv): a check failed — see comparison.yaml\n'
        exit 1
    fi
    printf 'integration(test-integration-build-live-arxiv) ok\n'

# A live build over a caller-named fixture repository, never a path this file
# knows. The fixture must stand at the `staged` tag prep-test-integration-fixture
# moved, so the build opens rather than resumes; kb-root/ is removed before it.
# The arguments after config go to ./bin/kbase build as given, run from dir:
# the volume roots relative to dir and, where wanted, a build flag such as
# --through. Everything the run writes but the fixture's own tree lands in one
# timestamped scratch directory: the state store, each command's capture, the
# counts and the log. A build bounded by --through skips the checks that read
# a finished KB, each recorded as skipped. (`*roots` rather than `+roots`:
# just admits no required variadic after a defaulted parameter; none given is
# refused below.)
[positional-arguments]
[doc("LIVE — NEEDS the inference endpoint config names and pandoc (excluded from test-integration); DESTRUCTIVE to dir's kb-root/: over a fixture repository prep-test-integration-fixture has staged (HEAD at its `staged` tag), ./bin/kbase models --provider <the entry config.toml's provider key names>, then ./bin/kbase build <roots...> from dir without --no-inference, configured by a reference-shaped copy of config (default test-data/fixtures/config/, relative to the repository root) staged in the run's own directory — the fixture's own file mode is the old grammar, which a build refuses; then ./bin/kbase refresh and ./bin/kbase verify green; the counts the arXiv live recipe records in comparison.yaml, the log, the state store and each command's capture under .claude-temp/live-fixture/<timestamp>/; dir is absolute or relative to the invocation directory, roots relative to dir, and a build flag among them (e.g. --through document-graph) is passed as given")]
test-integration-build-live-fixture dir config=LIVE_CONFIG_DIR *roots:
    #!/usr/bin/env bash
    set -euo pipefail
    dir="${1}"
    config="${2}"
    shift 2
    [[ "${dir}" == /* ]] || dir="{{invocation_directory()}}/${dir}"
    out="$(pwd -P)/.claude-temp/live-fixture/$(date +%Y%m%dT%H%M%S)"
    mkdir -p "${out}"
    "{{just_executable()}}" _test-integration-build-live-fixture "${out}" "${dir}" "${out}/state" "${config}" staged "$@" 2>&1 | tee "${out}/log.txt"

# The resume form: the same build invocation and post-build checks over a
# fixture whose commit trail's newest stage is equations-minted (a trail that
# reaches further is refused), with the prior run's state directory so its
# answer cache and records are found. The build resumes at the trail's next
# stage, references-found; kb-root/ is left as the trail has it. The checks
# and the output directory are the staged form's.
[positional-arguments]
[doc("LIVE — NEEDS the inference endpoint config names and pandoc (excluded from test-integration); DESTRUCTIVE to dir's kb-root/ and state: over a fixture repository whose commit trail (its `kb-build: <stage> |` commits) has equations-minted as its newest stage — refused, naming the newest stage the trail holds, otherwise — using state, the prior run's state directory (answer cache and records), ./bin/kbase build <roots...> from dir with --state-dir state and a reference-shaped copy of config (default test-data/fixtures/config/, relative to the repository root) staged in the run's own directory, which resumes at the trail's next stage (references-found); then the post-build checks of test-integration-build-live-fixture (./bin/kbase refresh and verify, comparison.yaml, the log and each command's capture) under a new .claude-temp/live-fixture/<timestamp>/; dir and state are absolute or relative to the invocation directory, roots relative to dir, and a build flag among them is passed as given")]
test-integration-build-live-fixture-resume dir state config=LIVE_CONFIG_DIR *roots:
    #!/usr/bin/env bash
    set -euo pipefail
    dir="${1}"
    state="${2}"
    config="${3}"
    shift 3
    [[ "${dir}" == /* ]] || dir="{{invocation_directory()}}/${dir}"
    [[ "${state}" == /* ]] || state="{{invocation_directory()}}/${state}"
    if [[ ! -d "${state}" ]]; then
        printf 'error: %s is not a directory — name the prior run'"'"'s state directory.\n' "${state}" >&2
        exit 1
    fi
    state="$(cd "${state}" && pwd -P)"
    out="$(pwd -P)/.claude-temp/live-fixture/$(date +%Y%m%dT%H%M%S)"
    mkdir -p "${out}"
    "{{just_executable()}}" _test-integration-build-live-fixture "${out}" "${dir}" "${state}" "${config}" resume "$@" 2>&1 | tee "${out}/log.txt"

# The body both forms run. mode is staged (kb-root/ removed, the fixture at its
# `staged` tag) or resume (the trail's newest stage is equations-minted, kb-root/ kept);
# state is the build's --state-dir.
[private]
[positional-arguments]
_test-integration-build-live-fixture out dir state config mode *roots: build
    #!/usr/bin/env bash
    set -euo pipefail
    out="${1}"
    dir="${2}"
    state="${3}"
    config="${4}"
    mode="${5}"
    shift 5
    label=test-integration-build-live-fixture
    [[ "${mode}" == staged ]] || label="${label}-${mode}"
    root="$(pwd -P)"
    kbase="${root}/{{BIN_DIR}}/kbase"
    [[ "${config}" == /* ]] || config="${root}/${config}"
    if [[ "$#" -eq 0 ]]; then
        printf 'error: name at least one volume root, relative to %s.\n' "${dir}" >&2
        exit 1
    fi
    if [[ ! -e "${dir}/.git" ]]; then
        printf 'error: %s is not a git repository — stage a fixture with just prep-test-integration-fixture first.\n' "${dir}" >&2
        exit 1
    fi
    dir="$(cd "${dir}" && pwd -P)"
    if [[ "${dir}" == "${root}" ]]; then
        printf 'error: %s is this repository — the build is for a fixture copy only.\n' "${dir}" >&2
        exit 1
    fi
    if ! staged="$(git -C "${dir}" rev-parse -q --verify 'refs/tags/staged^{commit}')"; then
        printf 'error: %s carries no `staged` tag — run just prep-test-integration-fixture %s first.\n' "${dir}" "${dir}" >&2
        exit 1
    fi
    head_rev="$(git -C "${dir}" rev-parse HEAD)"
    if [[ "${mode}" == staged ]]; then
        if [[ "${head_rev}" != "${staged}" ]]; then
            printf 'error: %s is not at its `staged` tag (a build has committed over it) — re-stage it with just prep-test-integration-fixture %s.\n' "${dir}" "${dir}" >&2
            exit 1
        fi
        where="staged ${staged}"
    else
        trail="$(git -C "${dir}" log --format=%s --grep='^kb-build: ' "${staged}..HEAD")"
        latest="$(sed -n '1s/^kb-build: \([^ ]*\) |.*/\1/p' <<< "${trail}")"
        if [[ "${latest}" != equations-minted ]]; then
            printf 'error: the newest stage in the commit trail of %s must be equations-minted; it is %s — reset the fixture to its equations-minted boundary commit.\n' \
                "${dir}" "${latest:-(none: no kb-build: commits above staged)}" >&2
            exit 1
        fi
        where="HEAD ${head_rev} (staged ${staged}), state ${state}"
    fi
    pandoc_version="$(pandoc --version | head -n 1)"
    printf 'fixture: %s at %s\nbuild arguments: %s\nconfig: %s\nout: %s\n%s\n' \
        "${dir}" "${where}" "$*" "${config}" "${out}" "${pandoc_version}"
    ck="${out}/checks"
    mkdir -p "${ck}"
    # capture <stem> <dir> <command...>: run the command in dir, stdout to
    # <stem>.out, stderr to <stem>.log, exit code to <stem>.exit.
    capture() {
        local stem="$1" in="$2"
        shift 2
        set +e
        ( cd "${in}" && "$@" ) > "${stem}.out" 2> "${stem}.log"
        printf '%s\n' "$?" > "${stem}.exit"
        set -e
    }
    rc() { cat "$1.exit"; }
    checks=()
    status=0
    record() {
        checks+=("$1"$'\t'"$2"$'\t'"$3"$'\t'"$4")
        printf '  %s %s: %s\n' "$2" "$1" "$3"
    }
    check() {
        local verdict=pass
        [[ "$2" -eq 0 ]] || { verdict=fail; status=1; }
        record "$1" "${verdict}" "$3" "$4"
    }
    yaml_str() { local s="${1//\\/\\\\}"; printf '"%s"' "${s//\"/\\\"}"; }
    ok() { "$@" > /dev/null 2>&1 && echo 0 || echo 1; }
    # count: a bounded build leaves no scratch/ to search.
    count() { { find "$1" -type f -name "$2" 2>/dev/null || true; } | wc -l | tr -d ' '; }

    # The endpoint first: an unreachable provider fails here, by name, and no
    # build is started against it. The fixture's pool holds two entries, so the
    # catalogue verb is told which one the fixture's key names.
    provider="$(sed -n 's/^provider *= *"\(.*\)".*/\1/p' "${config}/config.toml" | head -n 1)"
    if [[ -z "${provider}" ]]; then
        printf 'error: %s/config.toml names no provider, so the endpoint to check cannot be chosen.\n' "${config}" >&2
        exit 1
    fi
    capture "${out}/models" "${root}" "${kbase}" models --config-dir "${config}" --provider "${provider}"
    if [[ "$(rc "${out}/models")" -ne 0 ]]; then
        printf 'integration(%s): provider %s, named in %s/config.toml, is unreachable — see %s\n' \
            "${label}" "${provider}" "${config}" "${out}/models.out"
        exit 1
    fi

    # The fixture's config.toml is in the old grammar — bare model ids beside a
    # superseded provider key — and file-mode resolution refuses a bare id. The
    # build reads a reference-shaped copy in this run's directory: the same pool
    # and keys, and three references naming the entry that key names.
    toml_get() { awk -v t="[$2]" -v k="$3" '/^\[/ { in_t = ($0 == t) } in_t && $1 == k { sub(/^[^=]*= *"/, ""); sub(/".*/, ""); print; exit }' "$1"; }
    decision="$(toml_get "${config}/config.toml" models decision)"
    text="$(toml_get "${config}/config.toml" models text)"
    file_config="${out}/file-config"
    mkdir -p "${file_config}"
    cp "${config}/providers.toml" "${file_config}/providers.toml"
    [[ ! -d "${config}/api_keys" ]] || cp -R "${config}/api_keys" "${file_config}/api_keys"
    # The fixture declares no scoring model; the decision model judges the KB.
    printf '%s\n' '[models]' "decision = \"${provider}/${decision}\"" "text = \"${provider}/${text}\"" "scoring = \"${provider}/${decision}\"" \
        > "${file_config}/config.toml"

    # the build, asking: from a fresh kb-root/, or resuming over the trail's
    if [[ "${mode}" == staged ]]; then
        command rm -rf "${dir}/kb-root"
    fi
    capture "${ck}/kbase-build" "${dir}" "${kbase}" build "$@" --config-dir "${file_config}" --state-dir "${state}" --log-level info
    outcome="$(sed -n 's/^outcome: "\(.*\)"$/\1/p' "${ck}/kbase-build.out" | head -n 1)"
    check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 && ( "${outcome}" == done || "${outcome}" == bounded ) ]] && echo 0 || echo 1)" \
        "exit $(rc "${ck}/kbase-build"), outcome ${outcome:-none}" "${ck}/kbase-build.out"
    volumes="$(grep '^- \[' "${dir}/kb-root/entry-point.md" 2>/dev/null || true)"
    check entry-point "$(ok test -n "${volumes}")" "$(grep -c . <<< "${volumes}" || true) volume(s) listed: $(tr '\n' ' ' <<< "${volumes}")" "${dir}/kb-root/entry-point.md"
    captures="$(count "${state}/scratch/captures" '*.capture.jsonl')"
    answers="$(count "${state}/scratch/answers" '*.txt')"
    ask_records="$(count "${state}/scratch/asks" '*.yaml')"

    if [[ "${outcome}" != done ]]; then
        for c in ledger no-row-dropped captures kbase-refresh kbase-verify records; do
            record "${c}" skipped "the build did not finish (outcome ${outcome:-none}); this check reads a finished KB" "${ck}/kbase-build.out"
        done
    else
        boundaries="$(git -C "${dir}" log --format=%s --grep='^kb-build: ' "${staged}..HEAD" | wc -l | tr -d ' ')"
        check ledger "$(ok test "${boundaries}" -eq 11)" "${boundaries} kb-build: commits above staged, one per stage" "${dir}/.git"
        check no-row-dropped "$(ok test -z "$(git -C "${dir}" log --format=%b --grep='^kb-build: ' "${staged}..HEAD" | grep -- '--no-inference')")" \
            "no boundary names a dropped row" "${dir}/.git"
        check captures "$(ok test "${captures}" -gt 0 -a "${answers}" -gt 0 -a "${ask_records}" -gt 0)" \
            "${captures} captures, ${answers} cached answers, ${ask_records} group records under the state store's scratch/" "${state}/scratch"

        capture "${ck}/kbase-refresh" "${dir}" "${kbase}" refresh
        check kbase-refresh "$(rc "${ck}/kbase-refresh")" "exit $(rc "${ck}/kbase-refresh")" "${ck}/kbase-refresh.out"
        capture "${ck}/kbase-verify" "${dir}" "${kbase}" verify
        check kbase-verify "$(rc "${ck}/kbase-verify")" "exit $(rc "${ck}/kbase-verify")" "${ck}/kbase-verify.out"

        # the counts, off the build records and the result
        set +e
        go test -count=1 -run '^TestLiveBuildCounts$' ./internal/claimgraph -args -claimgraph.repo="${dir}" \
            -claimgraph.result="${ck}/kbase-build.out" -claimgraph.out="${out}" > "${ck}/counts.log" 2>&1
        check records "$?" "every leaf landed, every planned pair answered, no candidate drafted; counts.yaml written" "${ck}/counts.log"
        set -e
    fi

    {
        printf -- '- fixture: %s\n  staged: %s\n  build-arguments:\n' "$(yaml_str "${dir}")" "$(yaml_str "${staged}")"
        for a in "$@"; do
            printf '    - %s\n' "$(yaml_str "${a}")"
        done
        printf '  outcome: %s\n  state-dir: %s\n  scratch:\n    captures: %s\n    answers: %s\n    ask-records: %s\n  checks:\n' \
            "$(yaml_str "${outcome}")" "$(yaml_str "${state}")" "${captures}" "${answers}" "${ask_records}"
        for c in "${checks[@]}"; do
            IFS=$'\t' read -r c_name c_verdict c_detail c_evidence <<< "${c}"
            printf '    - check: %s\n      result: %s\n      detail: %s\n      evidence: %s\n' \
                "$(yaml_str "${c_name}")" "$(yaml_str "${c_verdict}")" "$(yaml_str "${c_detail}")" "$(yaml_str "${c_evidence}")"
        done
        if [[ -f "${out}/counts.yaml" ]]; then
            sed 's/^/  /' "${out}/counts.yaml"
        fi
    } > "${out}/comparison.yaml"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(%s): a check failed — see %s\n' "${label}" "${out}/comparison.yaml"
        exit 1
    fi
    printf 'integration(%s) ok — see %s\n' "${label}" "${out}/comparison.yaml"

# An out-of-process monitor over a running build, as personant runs one: the
# build is started in the background in a fresh fixture repository and a
# loop polls ./bin/kbase status, each poll kept; at the first poll reporting
# the build running in a stage past start — so the start boundary is
# recorded and what follows is a resume — ./bin/kbase cancel stops it. Then:
# the build exited
# cancelled (exit 4) naming its stage and unit, that stage unrecorded; the
# lock released and its file removed (a second cancel is refused, finding no
# holder); status reports cancelled with a resume command; and that command,
# run as status gives it, resumes the build to done, status then reporting it
# finished.
# Every result is parsed against its subcommand's documented keys.
[doc("NEEDS pandoc (excluded from test-integration): ./bin/kbase build --no-inference over MONITOR_ARXIV_ID in the background, an out-of-process loop polling ./bin/kbase status and cancelling it with ./bin/kbase cancel; cancelled (exit 4), the lock released and its file removed, status cancelled with its resume, the resume to done; evidence under test-data/transient/test-integration-monitor-arxiv/")]
test-integration-monitor-arxiv: (prep-test-integration-arxiv MONITOR_ARXIV_ID)
    @mkdir -p "{{MONITOR_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _test-integration-monitor-arxiv 2>&1 | tee "{{MONITOR_ARXIV_OUT_DIR}}/log.txt"

[private]
_test-integration-monitor-arxiv: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{MONITOR_ARXIV_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    name="$(printf '%s' "{{MONITOR_ARXIV_ID}}" | tr '/' '_')"
    repo="${out}/repo"
    state="${root}/${out}/state"
    rm -rf "${repo}" "${out}/state" "${out}/polls" "${out}/config" "${out}"/*.out "${out}"/*.log "${out}"/*.exit "${out}/monitor.tsv" "${out}/checks.tsv"
    mkdir -p "${repo}" "${out}/polls" "${out}/config"
    cp -R "{{ARXIV_DIR}}/${name}/." "${repo}/"
    git -C "${repo}" init -q
    git -C "${repo}" add -A
    git -C "${repo}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "sources as fetched"
    volume_root="$({{just_executable()}} _arxiv-volume-root "${repo}")"
    status=0
    # capture <stem> <command...>: run the command in the repository, stdout
    # to <stem>.out, stderr to <stem>.log, exit code to <stem>.exit.
    capture() {
        local stem="$1"
        shift
        set +e
        ( cd "${repo}" && "$@" ) > "${stem}.out" 2> "${stem}.log"
        printf '%s\n' "$?" > "${stem}.exit"
        set -e
    }
    # check <name> <passed: 0 or 1> <detail> <evidence>: one result.
    check() {
        local verdict=pass
        [[ "$2" -eq 0 ]] || { verdict=fail; status=1; }
        printf '%s\t%s\t%s\t%s\n' "$1" "${verdict}" "$3" "$4" >> "${out}/checks.tsv"
        printf '  %s %s: %s\n' "${verdict}" "$1" "$3"
    }
    # keys <stem> <verb> <outcome>: the result parsed against its documented
    # keys, its outcome the one named.
    keys() {
        set +e
        go test -count=1 -run '^TestResultDocument$' ./cmd -args -result.doc="${root}/$1.out" -result.verb="$2" -result.outcome="$3" > "$1.keys.log" 2>&1
        local rc=$?
        set -e
        check "$(basename "$1")-keys" "${rc}" "the $2 result parses against its documented keys, outcome $3" "$1.keys.log"
    }
    field() { sed -n "s/^$2: \"\{0,1\}\([^\"]*\)\"\{0,1\}$/\1/p" "$1.out" | head -n 1; }

    # The build, in the background; the monitor, out of its process.
    ( cd "${repo}" && exec "${kbase}" build "${volume_root}" --no-inference --config-dir "${root}/${out}/config" --state-dir "${state}" ) \
        > "${out}/build.out" 2> "${out}/build.log" &
    pid=$!
    printf 'poll\tstate\tcurrent\n' > "${out}/monitor.tsv"
    cancelled_at=""
    for n in $(seq 1 2000); do
        stem="${out}/polls/$(printf '%04d' "${n}")"
        capture "${stem}" "${kbase}" status --state-dir "${state}"
        state_now="$(field "${stem}" state)"
        current="$(sed -n '/^current:$/,/^[a-z]/s/^    stage: "\(.*\)"$/\1/p' "${stem}.out")"
        printf '%s\t%s\t%s\n' "${n}" "${state_now}" "${current}" >> "${out}/monitor.tsv"
        if [[ "${state_now}" == running && -n "${current}" && "${current}" != start ]]; then
            capture "${out}/cancel" "${kbase}" cancel --state-dir "${state}"
            cancelled_at="${n}"
            break
        fi
        kill -0 "${pid}" 2>/dev/null || break
    done
    set +e
    wait "${pid}"
    printf '%s\n' "$?" > "${out}/build.exit"
    set -e

    check observed-running "$([[ -n "${cancelled_at}" ]] && echo 0 || echo 1)" \
        "a status poll saw the build running past start (poll ${cancelled_at:-none}) and cancel was issued" "${out}/monitor.tsv"
    if [[ -z "${cancelled_at}" ]]; then
        printf 'integration(test-integration-monitor-arxiv): the build finished before any poll saw it running — see monitor.tsv\n'
        exit 1
    fi
    check cancel-done "$([[ "$(cat "${out}/cancel.exit")" -eq 0 ]] && echo 0 || echo 1)" "cancel exit $(cat "${out}/cancel.exit")" "${out}/cancel.out"
    keys "${out}/cancel" cancel done
    stage="$(sed -n '/^cancelled:$/,$s/^    stage: "\(.*\)"$/\1/p' "${out}/build.out")"
    check build-cancelled "$([[ "$(cat "${out}/build.exit")" -eq 4 && -n "${stage}" ]] && echo 0 || echo 1)" \
        "build exit $(cat "${out}/build.exit"), cancelled in ${stage:-no stage}" "${out}/build.out"
    keys "${out}/build" build cancelled
    check stage-unrecorded "$([[ -n "${stage}" && -z "$(git -C "${repo}" log --format=%s --grep="^kb-build: ${stage} ")" ]] && echo 0 || echo 1)" \
        "the stage in flight, ${stage:-none}, has no boundary commit" "${repo}/.git"
    capture "${out}/cancel-again" "${kbase}" cancel --state-dir "${state}"
    check lock-released "$([[ "$(cat "${out}/cancel-again.exit")" -eq 1 ]] && grep -q 'no build holds the state store' "${out}/cancel-again.out" && echo 0 || echo 1)" \
        "a second cancel exit $(cat "${out}/cancel-again.exit"), finding no holder" "${out}/cancel-again.out"
    keys "${out}/cancel-again" cancel refused
    check lock-absent "$([[ ! -e "${state}/lock" ]] && echo 0 || echo 1)" \
        "the state store's lock file is gone after the cancel, not merely unheld" "${out}/state"
    capture "${out}/status-cancelled" "${kbase}" status --state-dir "${state}"
    resume="$(field "${out}/status-cancelled" resume)"
    check status-cancelled "$([[ "$(field "${out}/status-cancelled" state)" == cancelled && "${resume}" == "kbase build "* ]] && echo 0 || echo 1)" \
        "status reports $(field "${out}/status-cancelled" state), resume: ${resume:-none}" "${out}/status-cancelled.out"
    keys "${out}/status-cancelled" status done
    printf '%s\n' "${resume}" > "${out}/resume.command"
    set +e
    ( cd "${repo}" && eval "\"${kbase}\" ${resume#kbase }" ) > "${out}/resume.out" 2> "${out}/resume.log"
    printf '%s\n' "$?" > "${out}/resume.exit"
    set -e
    check resume-done "$([[ "$(cat "${out}/resume.exit")" -eq 0 ]] && grep -qx 'resumed: true' "${out}/resume.out" && echo 0 || echo 1)" \
        "the resume command exit $(cat "${out}/resume.exit"), outcome $(field "${out}/resume" outcome)" "${out}/resume.out"
    keys "${out}/resume" build done
    capture "${out}/status-finished" "${kbase}" status --state-dir "${state}"
    check status-finished "$([[ "$(field "${out}/status-finished" state)" == finished ]] && echo 0 || echo 1)" \
        "status after the resume reports $(field "${out}/status-finished" state)" "${out}/status-finished.out"
    keys "${out}/status-finished" status done

    {{just_executable()}} _write-evidence "${out}" "test-integration-monitor-arxiv" \
      "{{MONITOR_ARXIV_ID}} staged into repo/ (sources, git init, one commit)\n(cd repo && ${kbase} build <volume-root> --no-inference --config-dir config --state-dir state) > build.out &\nper poll NNNN until one reports running: (cd repo && ${kbase} status --state-dir state) > polls/NNNN.out; then ${kbase} cancel --state-dir state > cancel.out\nwait for the build; ${kbase} cancel again > cancel-again.out; ${kbase} status > status-cancelled.out\nthe resume command status-cancelled.out gives (resume.command), run from repo > resume.out; ${kbase} status > status-finished.out\neach result: go test -run '^TestResultDocument\$' ./cmd -args -result.doc=<stem>.out -result.verb=<verb> -result.outcome=<outcome> > <stem>.keys.log"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-monitor-arxiv): a check failed — see checks.tsv\n'
        exit 1
    fi
    printf 'integration(test-integration-monitor-arxiv) ok\n'

# The result contract over the delivered binary: every maintenance subcommand,
# status and cancel run by ./bin/kbase over a copy of a committed fixture KB,
# each result parsed against its subcommand's documented keys and outcome.
# It needs only git, which SPEC requires on the host — no pandoc, no
# provider, no adjagent clone — so it joins the omnibus.
[doc("hermetic: every maintenance subcommand, status and cancel through ./bin/kbase over the committed fixture KB, each result parsed against its documented keys — plus score's scoring role: a file-mode configuration naming no scoring refuses as models.scoring with and without --scoring-model, a flag naming a provider the pool lacks refuses as --scoring-model with the pool listed, a flag naming the pool's entry reaches it and records provider-rule flag, and a run configured by the environment alone reaches the scoring model, names it in the ledger and never names text's — and the harness back end over the internal command override: the prompt on argv versus stdin compared with the ledger's digest, one process per item in a directory emptied per item, a non-zero exit and each of the two bounds failing that item alone, the override announced once and recorded in run.yaml, a command that cannot be started and a --harness name kbase ships no back end for refused before the walk, a dry run touching no command, and the environment set beside --harness; evidence under test-data/transient/test-integration-results-fixture/")]
test-integration-results-fixture:
    @mkdir -p "{{RESULTS_FIXTURE_OUT_DIR}}"
    @{{just_executable()}} _test-integration-results-fixture 2>&1 | tee "{{RESULTS_FIXTURE_OUT_DIR}}/log.txt"

[private]
_test-integration-results-fixture: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{RESULTS_FIXTURE_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    fixture="${root}/{{RESULTS_FIXTURE_DIR}}"
    # The inference variables are scrubbed for the whole recipe: every leg below
    # reads the staged tree and nothing the host exports, except the one that
    # sets them to prove the environment supplies a run, and the back end
    # override, which the harness legs set for themselves.
    unset KBASE_API_BASE_URL KBASE_DECISION KBASE_TEXT KBASE_SCORING KBASE_API_KEY_FILE KBASE_CONFIG_DIR KBASE_SCORE_BACK_END_COMMAND
    repo="${out}/repo"
    docs="${out}/docs"
    rm -rf "${repo}" "${docs}" "${out}/checks.tsv" "${out}/score-config" "${out}/score-home" \
        "${out}/score-state" "${out}/score-env-state"
    mkdir -p "${repo}/.git/objects" "${repo}/.git/refs/heads" "${docs}"
    printf 'ref: refs/heads/main\n' > "${repo}/.git/HEAD"
    cp -R "${fixture}/kb-root" "${repo}/"
    status=0
    n=0
    LAST=""
    # run <verb-for-keys> <outcome> <args...>: one invocation from the
    # repository, its result parsed against the verb's documented keys.
    run() {
        local verb="$1" want="$2" stem
        shift 2
        n=$((n + 1))
        stem="${docs}/$(printf '%02d' "${n}")-${1}"
        set +e
        ( cd "${repo}" && "${kbase}" "$@" ) > "${stem}.out" 2> "${stem}.log"
        printf '%s\n' "$?" > "${stem}.exit"
        go test -count=1 -run '^TestResultDocument$' ./cmd -args -result.doc="${root}/${stem}.out" -result.verb="${verb}" \
            -result.outcome="${want}" > "${stem}.keys.log" 2>&1
        local rc=$?
        set -e
        printf '%s\t%s\t%s\t%s\n' "$*" "${want}" "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "${stem}.out" >> "${out}/checks.tsv"
        printf '  %s %s: want %s, exit %s\n' "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "$*" "${want}" "$(cat "${stem}.exit")"
        [[ "${rc}" -eq 0 ]] || status=1
        LAST="${stem}.out"
    }
    run verify refused verify
    run refresh done refresh
    run refresh unchanged refresh
    run verify done verify
    run render-claim-graph unchanged render-claim-graph
    run insert-claim-entry done insert-claim-entry --create --values "${fixture}/values/insert.yaml"
    run insert-claim-entry unchanged insert-claim-entry --no-refresh --values "${fixture}/values/insert.yaml"
    run score done score --dry-run --pass rigor --state-dir "${root}/${out}/state"
    # Score's role is the KB's own declaration. A file-mode configuration that
    # names decision and text but no scoring refuses before the first item,
    # naming models.scoring — and refuses even where --scoring-model names a
    # model, because the flag overrides a role and never substitutes for one.
    score_config="${root}/${out}/score-config"
    mkdir -p "${score_config}"
    printf '%s\n' '[unreachable]' 'baseUrl = "http://127.0.0.1:1/v1"' > "${score_config}/providers.toml"
    printf '%s\n' '[models]' 'decision = "unreachable/d"' 'text = "unreachable/t"' > "${score_config}/config.toml"
    score_state="${root}/${out}/score-state"
    run score refused score --pass rigor --config-dir "${score_config}" --state-dir "${score_state}"
    grep -qF 'key: "models.scoring"' "${LAST}" || { printf '  fail: the absent scoring role is not named\n'; status=1; }
    run score refused score --pass rigor --config-dir "${score_config}" --scoring-model unreachable/judge --state-dir "${score_state}"
    grep -qF 'key: "models.scoring"' "${LAST}" || { printf '  fail: --scoring-model substituted for the absent role\n'; status=1; }
    # The role configured, the flag naming a provider the pool lacks: the
    # flag's refusal, before the first item, with the pool's entries listed.
    printf '%s\n' '[models]' 'decision = "unreachable/d"' 'text = "unreachable/t"' 'scoring = "unreachable/s"' > "${score_config}/config.toml"
    run score refused score --pass rigor --config-dir "${score_config}" --scoring-model elsewhere/judge --state-dir "${score_state}"
    grep -qF 'key: "--scoring-model"' "${LAST}" || { printf '  fail: a missing provider is not named as the flag\x27s\n'; status=1; }
    grep -qF '"unreachable"' "${LAST}" || { printf '  fail: the pool is not listed\n'; status=1; }
    # A flag that parses and names the pool's entry reaches it: the endpoint
    # nothing listens on fails the one item and --limit stops the walk, so the
    # run is bounded; the roles record says the flag named provider and model.
    run score bounded score --pass rigor --limit 1 --config-dir "${score_config}" --scoring-model unreachable/judge --state-dir "${score_state}"
    grep -qx '      provider-rule: "flag"' "${LAST}" || { printf '  fail: the roles record does not say the flag named the provider\n'; status=1; }
    grep -qx '      model: "judge"' "${LAST}" || { printf '  fail: the roles record does not name the flag\x27s model\n'; status=1; }
    # Environment mode with no configuration directory at all: the base URL and
    # the role variables configure the run, its one item fails on the same
    # unreachable endpoint, and the document and the ledger name the scoring
    # model — a different model from text's, which appears nowhere.
    mkdir -p "${out}/score-home"
    env_score="${docs}/$(printf '%02d' "$((n + 1))")-score-env-mode"
    set +e
    ( cd "${repo}" && env -u KBASE_CONFIG_DIR -u KBASE_API_KEY_FILE "HOME=${out}/score-home" \
        KBASE_API_BASE_URL=http://127.0.0.1:1/v1 KBASE_DECISION=env-decision KBASE_TEXT=env-text KBASE_SCORING=env-scoring \
        "${kbase}" score --pass rigor --limit 1 --state-dir "${root}/${out}/score-env-state" ) > "${env_score}.out" 2> "${env_score}.log"
    printf '%s\n' "$?" > "${env_score}.exit"
    go test -count=1 -run '^TestResultDocument$' ./cmd -args -result.doc="${root}/${env_score}.out" -result.verb=score \
        -result.outcome=bounded > "${env_score}.keys.log" 2>&1
    env_score_keys=$?
    set -e
    env_score_ok=0
    [[ "$(cat "${env_score}.exit")" -eq 0 ]] || env_score_ok=1
    [[ "${env_score_keys}" -eq 0 ]] || env_score_ok=1
    n=$((n + 1))
    grep -qx 'outcome: "bounded"' "${env_score}.out" || env_score_ok=1
    grep -qx '    - role: "scoring"' "${env_score}.out" || env_score_ok=1
    grep -qx '      back-end: "environment"' "${env_score}.out" || env_score_ok=1
    grep -qx '      model: "env-scoring"' "${env_score}.out" || env_score_ok=1
    score_ledger="$(sed -n 's/^ledger: "\(.*\)"$/\1/p' "${env_score}.out" | head -n 1)"
    [[ -n "${score_ledger}" && -f "${score_ledger}/ledger.jsonl" ]] || env_score_ok=1
    grep -qF '"model":"env-scoring"' "${score_ledger}/ledger.jsonl" || env_score_ok=1
    # The text model appears nowhere the run asked: the roles record and the
    # ledger's lines name the scoring model alone. The resume record is the one
    # place it stands, because an environment-mode command re-issues the whole
    # configuration it read, text's variable included.
    if grep -v '^resume:' "${env_score}.out" | grep -qF 'env-text'; then env_score_ok=1; fi
    if grep -qF 'env-text' "${score_ledger}/ledger.jsonl"; then env_score_ok=1; fi
    # 28: an environment-mode run's command re-issues the mode as assignments
    # before the program, so a shell with none of them set asks the same
    # endpoint with the same model.
    grep -q '^resume: "KBASE_API_BASE_URL=http://127.0.0.1:1/v1 KBASE_DECISION=env-decision KBASE_TEXT=env-text KBASE_SCORING=env-scoring kbase score ' \
        "${env_score}.out" || env_score_ok=1
    printf 'score --pass rigor --limit 1 (environment mode)\tbounded\t%s\t%s\n' \
        "$([[ "${env_score_ok}" -eq 0 ]] && echo pass || echo fail)" "${env_score}.out" >> "${out}/checks.tsv"
    printf '  %s score in environment mode: want bounded, exit %s\n' \
        "$([[ "${env_score_ok}" -eq 0 ]] && echo pass || echo fail)" "$(cat "${env_score}.exit")"
    [[ "${env_score_ok}" -eq 0 ]] || status=1

    # The harness back end: one process per item, its command named by the
    # back end override — the one way a run's command is chosen from outside
    # kbase, and the way the suite fakes a back end now that no profile can.
    # The stub answers each item with one value and leaves, in the run's own
    # working directory, the digest of the text it was handed — so these legs
    # say where the prompt reached it, and that it was exactly the prompt the
    # ledger digested. It never writes a prompt. The variable is set per leg
    # and unset again, and the announcement every run that saw it writes is
    # checked with the first.
    stub="${root}/{{HARNESS_STUB}}"
    hstate="${root}/${out}/harness-state"
    # ledger_of <document>: the run directory its ledger field names.
    ledger_of() { sed -n 's/^ledger: "\(.*\)"$/\1/p' "$1" | head -n 1; }
    # line_digest <line>: the prompt_sha256 the ledger's line records.
    line_digest() { sed -n 's/.*"prompt_sha256":"\([0-9a-f]*\)".*/\1/p' <<< "$1"; }

    # 29 (argv): the prompt reaches the one element that names it, and stdin is
    # closed empty. The stub's digest is the ledger's, so the text the command
    # was handed is the text the run composed.
    export KBASE_SCORE_BACK_END_COMMAND="${stub} argv {prompt}"
    run score bounded score --pass rigor --limit 1 --harness opencode --config-dir "${score_config}" --state-dir "${hstate}"
    hledger="$(ledger_of "${LAST}")"
    hline="$(head -n 1 "${hledger}/ledger.jsonl" 2>/dev/null || true)"
    [[ -f "${hledger}/harness/argv.digest" ]] || { printf '  fail: the argv command left no digest\n'; status=1; }
    [[ ! -e "${hledger}/harness/stdin.digest" ]] || { printf '  fail: the argv command also received the prompt on stdin\n'; status=1; }
    [[ -n "${hline}" && "$(cat "${hledger}/harness/argv.digest")" == "$(line_digest "${hline}")" ]] \
        || { printf '  fail: the argv prompt is not the prompt the ledger digested\n'; status=1; }
    grep -qx '    - role: "scoring"' "${LAST}" || { printf '  fail: the harness document names no scoring role\n'; status=1; }
    grep -qx '      back-end: "harness"' "${LAST}" || { printf '  fail: the roles record does not name the harness back end\n'; status=1; }
    grep -qx '      model: "unreachable/s"' "${LAST}" || { printf '  fail: the roles record does not name the scoring role\x27s model\n'; status=1; }
    for key in provider provider-rule provider-layer harness harness-layer; do
        if grep -q "^      ${key}:" "${LAST}"; then printf '  fail: a harness role names %s\n' "${key}"; status=1; fi
    done
    grep -qx 'backend: harness' "${hledger}/run.yaml" || { printf '  fail: run.yaml does not name the back end\n'; status=1; }
    grep -qx 'command_override: KBASE_SCORE_BACK_END_COMMAND' "${hledger}/run.yaml" \
        || { printf '  fail: run.yaml does not name the variable that supplied the command\n'; status=1; }
    grep -qx "    - '{prompt}'" "${hledger}/run.yaml" || { printf '  fail: run.yaml does not record the argv with the prompt left as the token\n'; status=1; }
    if grep -qE '^harness(_layer)?:' "${hledger}/run.yaml"; then printf '  fail: run.yaml names a profile\n'; status=1; fi
    grep -qF '"backend":"harness"' "${hledger}/ledger.jsonl" || { printf '  fail: the ledger line does not name the back end\n'; status=1; }
    if grep -qF '"harness":' "${hledger}/ledger.jsonl"; then printf '  fail: the ledger line names a profile\n'; status=1; fi
    # The announcement: once per run, naming the variable and the argv, so a
    # command nobody chose is never executed silently.
    [[ "$(grep -c 'names the command' "${LAST%.out}.log")" -eq 1 ]] \
        || { printf '  fail: the back end override did not announce itself exactly once\n'; status=1; }
    grep -qF 'KBASE_SCORE_BACK_END_COMMAND' "${LAST%.out}.log" \
        || { printf '  fail: the announcement does not name the variable\n'; status=1; }
    # 43: the header records the run's ask once — the digest of its pass's
    # prompt rendered for an empty subject — and a landed line records the
    # charges it was priced from. This run's reply is `none`, so its empty list
    # is the model's own statement that it found no fault.
    grep -qE '^ask_sha256: [0-9a-f]{64}$' "${hledger}/run.yaml" || { printf '  fail: run.yaml records no ask digest\n'; status=1; }
    grep -qF '"faults":[]' "${hledger}/ledger.jsonl" || { printf '  fail: a landed line records no empty charge list for `none`\n'; status=1; }

    # 20/29 (stdin): a command naming no {prompt} receives the prompt there.
    export KBASE_SCORE_BACK_END_COMMAND="${stub} stdin"
    run score bounded score --pass rigor --limit 1 --harness opencode --config-dir "${score_config}" --state-dir "${hstate}"
    hledger="$(ledger_of "${LAST}")"
    hline="$(head -n 1 "${hledger}/ledger.jsonl" 2>/dev/null || true)"
    [[ -f "${hledger}/harness/stdin.digest" ]] || { printf '  fail: the stdin command left no digest\n'; status=1; }
    [[ ! -e "${hledger}/harness/argv.digest" ]] || { printf '  fail: the stdin command also received the prompt in an element\n'; status=1; }
    [[ -n "${hline}" && "$(cat "${hledger}/harness/stdin.digest")" == "$(line_digest "${hline}")" ]] \
        || { printf '  fail: the stdin prompt is not the prompt the ledger digested\n'; status=1; }

    # 23: two items, two processes, and nothing one item wrote visible to the
    # next — the stub fails an item that finds the previous one's file.
    export KBASE_SCORE_BACK_END_COMMAND="${stub} isolate {prompt}"
    run score done score --pass rigor --harness opencode --config-dir "${score_config}" --state-dir "${hstate}"
    hledger="$(ledger_of "${LAST}")"
    [[ "$(sed -n 's/^    done: //p' "${LAST}")" -eq 2 ]] || { printf '  fail: the isolation command did not answer both items\n'; status=1; }
    [[ -f "${hledger}/harness/seen" ]] || { printf '  fail: the isolation stub never ran\n'; status=1; }

    # 24: a non-zero exit is that item's failure — the line carries the exit code
    # and the stderr tail, the next item is asked, and the run closes on failures.
    export KBASE_SCORE_BACK_END_COMMAND="${stub} fail {prompt}"
    run score failed score --pass rigor --harness opencode --config-dir "${score_config}" --state-dir "${hstate}"
    hledger="$(ledger_of "${LAST}")"
    [[ "$(grep -cF '"status":"failed"' "${hledger}/ledger.jsonl")" -eq 2 ]] \
        || { printf '  fail: the walk did not carry on past the first failed item\n'; status=1; }
    grep -qF 'the stub refuses' "${hledger}/ledger.jsonl" || { printf '  fail: the line does not carry the stderr tail\n'; status=1; }
    grep -qF 'exit status 3' "${LAST}" || { printf '  fail: the failure does not carry the exit code\n'; status=1; }
    [[ "$(grep -c '^    - check: "error"' "${LAST}")" -eq 2 ]] || { printf '  fail: failures does not name both items\n'; status=1; }
    # 28: the command that continues the run continues it on the same back end.
    grep -q '^resume: ".* --harness opencode"$' "${LAST}" || { printf '  fail: the resume command does not carry --harness\n'; status=1; }

    # 25: a command that cannot be started refuses before the walk.
    export KBASE_SCORE_BACK_END_COMMAND="${stub}-missing value"
    run score refused score --pass rigor --harness opencode --config-dir "${score_config}" --state-dir "${hstate}"
    grep -qF 'check: "harness"' "${LAST}" || { printf '  fail: a command that cannot start is not a harness refusal\n'; status=1; }
    grep -qF 'key: "--harness"' "${LAST}" || { printf '  fail: the refusal does not name --harness\n'; status=1; }
    unset KBASE_SCORE_BACK_END_COMMAND

    # 27: a dry run asks nothing, so --harness changes nothing.
    run score done score --dry-run --pass rigor --harness opencode --config-dir "${score_config}" --state-dir "${hstate}"
    hledger="$(ledger_of "${LAST}")"
    if grep -q '^roles:' "${LAST}"; then printf '  fail: a dry run carries a roles record\n'; status=1; fi
    [[ ! -e "${hledger}/harness" ]] || { printf '  fail: a dry run left a working directory for a command it never ran\n'; status=1; }

    # 33: both bounds fail the item with the reason that bound produces.
    export KBASE_SCORE_BACK_END_COMMAND="${stub} big-reply {prompt}"
    run score bounded score --pass rigor --limit 1 --harness opencode --config-dir "${score_config}" --state-dir "${hstate}"
    hledger="$(ledger_of "${LAST}")"
    grep -q 'the reply is over the' "${hledger}/ledger.jsonl" || { printf '  fail: an over-limit reply is not the item\x27s failure\n'; status=1; }
    export KBASE_SCORE_BACK_END_COMMAND="${stub} big-stderr {prompt}"
    run score bounded score --pass rigor --limit 1 --harness opencode --config-dir "${score_config}" --state-dir "${hstate}"
    hledger="$(ledger_of "${LAST}")"
    grep -q 'stderr bound' "${hledger}/ledger.jsonl" || { printf '  fail: over-limit stderr is not the item\x27s failure\n'; status=1; }
    unset KBASE_SCORE_BACK_END_COMMAND

    # 21: a harness consults no mode, so a base URL and the role variables set
    # beside it are neither a mixed mode nor a refusal.
    harness_env="${root}/${out}/harness-env-state"
    set +e
    ( cd "${repo}" && env -u KBASE_CONFIG_DIR -u KBASE_API_KEY_FILE "HOME=${out}/score-home" \
        KBASE_API_BASE_URL=http://127.0.0.1:1/v1 KBASE_DECISION=env-decision KBASE_TEXT=env-text KBASE_SCORING=env-scoring \
        KBASE_SCORE_BACK_END_COMMAND="${stub} argv {prompt}" \
        "${kbase}" score --pass rigor --limit 1 --harness opencode --config-dir "${score_config}" --state-dir "${harness_env}" ) \
        > "${out}/harness-env.out" 2> "${out}/harness-env.log"
    printf '%s\n' "$?" > "${out}/harness-env.exit"
    set -e
    harness_env_ok=0
    [[ "$(cat "${out}/harness-env.exit")" -eq 0 ]] || harness_env_ok=1
    grep -qx 'outcome: "bounded"' "${out}/harness-env.out" || harness_env_ok=1
    grep -qx '      back-end: "harness"' "${out}/harness-env.out" || harness_env_ok=1
    # The environment names the model where it names one.
    grep -qx '      model: "env-scoring"' "${out}/harness-env.out" || harness_env_ok=1
    [[ "$(grep -c 'names the command' "${out}/harness-env.log")" -eq 1 ]] || harness_env_ok=1
    printf 'score --pass rigor --limit 1 --harness opencode (environment set)\tbounded\t%s\t%s\n' \
        "$([[ "${harness_env_ok}" -eq 0 ]] && echo pass || echo fail)" "${out}/harness-env.out" >> "${out}/checks.tsv"
    printf '  %s score --harness with the environment set: want bounded, exit %s\n' \
        "$([[ "${harness_env_ok}" -eq 0 ]] && echo pass || echo fail)" "$(cat "${out}/harness-env.exit")"
    [[ "${harness_env_ok}" -eq 0 ]] || status=1

    # 30: the flag names the back ends kbase ships and no profile, so a name
    # nothing ships refuses before the KB is opened, listing the one it takes.
    run score refused score --pass rigor --harness claude --config-dir "${score_config}" --state-dir "${hstate}"
    grep -qF 'check: "harness"' "${LAST}" || { printf '  fail: a name kbase ships no back end for is not a harness refusal\n'; status=1; }
    grep -qF 'key: "--harness"' "${LAST}" || { printf '  fail: the refusal does not name --harness\n'; status=1; }
    [[ "$(sed -n '/^      allowed:/,/^      [a-z]/p' "${LAST}" | sed -n 's/^        - "\(.*\)"$/\1/p' | tr '\n' ' ')" == "opencode " ]] \
        || { printf '  fail: the refusal does not list the one back end kbase ships\n'; status=1; }

    run set-rigor refused set-rigor --values "${fixture}/values/bad-rigor.yaml"
    run render-citation refused render-citation --values "${fixture}/values/cite-nowhere.yaml"
    id="$(sed -n 's/^    - "\(clm-[a-z0-9]*\)"$/\1/p' "${docs}/06-insert-claim-entry.out" | head -n 1)"
    [[ -n "${id}" ]] || { printf 'integration(results-fixture): the insert reported no id\n'; status=1; }
    run find done find ""
    run find done find "" --limit 1
    run find refused find "" --limit -1
    run deps done deps "${id}"
    run deps done deps "${id}" -i
    run gated-on done gated-on "${id}"
    run cited-by done cited-by "${id}"
    run referenced-by done referenced-by "${id}"
    run strength-below done strength-below --max-strength 0.5
    run subtree done subtree ""
    run weak-points done weak-points
    run show done show "${id}"
    run show refused show clm-zzzzzz
    run stats done stats
    run status done status --state-dir "${root}/${out}/state"
    run cancel refused cancel --state-dir "${root}/${out}/state"
    {{just_executable()}} _write-evidence "${out}" "test-integration-results-fixture" \
      "cp -R {{RESULTS_FIXTURE_DIR}}/kb-root repo/ (with a bare .git)\nper invocation NN-<verb>, from repo/: ${kbase} <args> > docs/NN-<verb>.out, parsed by go test -run '^TestResultDocument\$' ./cmd -args -result.doc=docs/NN-<verb>.out -result.verb=<verb> -result.outcome=<outcome> > docs/NN-<verb>.keys.log\nscore-config/: a pool entry whose endpoint is a closed loopback port, and a config.toml naming decision and text but no scoring; score legs over it: refused naming models.scoring with and without --scoring-model, refused naming --scoring-model for a provider the pool lacks with the pool listed, and one failed item whose roles record states provider-rule flag and the flag's model\nscore-env-mode: the same KB with HOME on score-home, KBASE_API_BASE_URL and KBASE_DECISION, KBASE_TEXT, KBASE_SCORING set and no configuration directory; its one item fails on the closed endpoint, the roles record names the environment back end and KBASE_SCORING's id, the ledger's line names it, and the text model appears nowhere the run asked (the resume record re-issues the whole configuration, so it names the mode's variables)\nharness: the harness legs run --harness opencode with KBASE_SCORE_BACK_END_COMMAND naming {{HARNESS_STUB}}, one mode per behaviour: argv and stdin (the prompt's two routes, each leaving the digest of what it was handed, compared with the ledger's prompt_sha256), isolate (a second item fails if it can see the first's file), fail (exit 3 with stderr: the item's failure, the walk carrying on), big-reply and big-stderr (each past a bound), and a missing program (refused before the walk); the first leg also checks the override announced once on stderr and recorded as command_override in run.yaml, and that run.yaml and the ledger's lines name the back end and no profile; a --harness name kbase ships no back end for refuses listing opencode; a dry run touches no command, and a run with the environment set beside --harness is bounded and reads its model from KBASE_SCORING\nchecks.tsv: per invocation its arguments, the outcome wanted, pass or fail, and its document"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-results-fixture): a result did not parse as documented — see checks.tsv\n'
        exit 1
    fi
    printf 'integration(test-integration-results-fixture) ok\n'

# Init legs over the delivered binary in real git repositories, each result
# checked against its documented keys: refused outside a worktree with nothing
# created, birth, birth-once, build refused over a created KB, the
# cross-process maintenance chain closed green, the no-dot placeholder. The
# born bytes, the birth commit's subject and its trail belong to
# cmd/init_test.go and ledger_test.go; the recipe never inspects them.
# It needs only git — no pandoc, no provider, no adjagent clone.
[doc("hermetic, git-only: init legs over ./bin/kbase, each result checked against its documented keys — refused outside a worktree with nothing created, birth, birth-once, build refused over a created KB, the cross-process maintenance chain closed green, the no-dot placeholder; evidence under test-data/transient/test-integration-init/")]
test-integration-init:
    @mkdir -p "{{INIT_OUT_DIR}}"
    @{{just_executable()}} _test-integration-init 2>&1 | tee "{{INIT_OUT_DIR}}/log.txt"

[private]
_test-integration-init: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="${root}/{{INIT_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    repo="${out}/repo"
    docs="${out}/docs"
    nodot="${out}/repo-nodot"
    rm -rf "${repo}" "${docs}" "${nodot}" "${out}/outside" "${out}/gitonly" "${out}/gitconfig" "${out}/seed.yaml" "${out}/leaf.md" "${out}/sf.json" "${out}/mc.json" "${out}/sf-run.json" "${out}/mc-run.json" "${out}/init.tsv" "${out}/state"
    mkdir -p "${docs}"
    export GIT_AUTHOR_NAME=init-integration GIT_AUTHOR_EMAIL=init@invalid GIT_COMMITTER_NAME=init-integration GIT_COMMITTER_EMAIL=init@invalid GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${out}/gitconfig"
    status=0
    fail() { printf '  fail: %s\n' "$*"; status=1; }
    # run <verb-for-keys> <outcome> <args...>: one invocation of the
    # delivered binary from the repository, its result parsed against the
    # verb's documented keys; LAST names the result document it captured.
    # Arguments reach the repository absolute: the tool runs in a subshell.
    n=0
    run() {
        local verb="$1" want="$2" stem
        shift 2
        n=$((n + 1))
        stem="${docs}/$(printf '%02d' "${n}")-$1"
        set +e
        ( cd "${repo}" && "${kbase}" "$@" ) > "${stem}.out" 2> "${stem}.log"
        printf '%s\n' "$?" > "${stem}.exit"
        go test -count=1 -run '^TestResultDocument$' ./cmd -args -result.doc="${stem}.out" -result.verb="${verb}" \
            -result.outcome="${want}" > "${stem}.keys.log" 2>&1
        local rc=$?
        set -e
        printf '%s\t%s\t%s\t%s\n' "$*" "${want}" "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "${stem}.out" >> "${out}/init.tsv"
        printf '  %s %s: want %s, exit %s\n' "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "$*" "${want}" "$(cat "${stem}.exit")"
        [[ "${rc}" -eq 0 ]] || status=1
        LAST="${stem}.out"
    }
    # 1. outside a worktree: refused, nothing created. run() executes from
    # ${repo}, so ${repo} points at mktemp's own tree — somewhere under no
    # repository at all, not the project's transient dir, which a repository
    # does hold. The whole run tree is copied to ${out}/outside before the
    # temporary directory goes.
    outside="$(mktemp -d)"
    repo="${outside}"
    run init refused init
    [[ ! -e "${outside}/kb-root" ]] || fail "a kb-root stands outside a worktree"
    mkdir -p "${out}/outside"
    cp -R "${outside}/." "${out}/outside/"
    rm -rf "${outside}"
    repo="${out}/repo"
    # 2. birth, default title: the repository's directory name. init commits
    # its own birth; the commit's subject and trail are ledger's, pinned in
    # ledger_test.go — the recipe never reads git.
    git init --quiet "${repo}"
    printf '%s\n' 'entry:' '- register: claim-rigor.md' '  title: An Authored Result' '  rigor: 0.5' '  rationale: Written at birth.' > "${out}/seed.yaml"
    printf '%s\n' 'entry:' '- document: a.md' '  kind: leaf' '  claims: ["@ID@"]' > "${out}/sf.json"
    printf '%s\n' 'entry:' '- document: a.md' '  id: "@ID@"' '  locator: authored text stands' > "${out}/mc.json"
    printf '%s\n' '# A Leaf' '' 'The authored text stands. It cites nothing it cannot show.' > "${out}/leaf.md"
    run init done init
    # verify green over the born tree as shipped, before any maintenance
    # step reaches it.
    run verify done verify
    # 3. re-issue: birth-once.
    run init refused init
    # 4. build stays refused over a created KB. The store is quarantined to
    # state/: build opens and creates it before its first refusal, so nothing
    # reaches the real $XDG_STATE_HOME.
    printf '%s\n' '\documentclass{article}' '\begin{document}' 'Nothing distilled.' '\end{document}' > "${repo}/paper.tex"
    run build refused build "${repo}/paper.tex" --no-inference --state-dir "${out}/state"
    # 5. the cross-process maintenance chain: the id the insert minted
    # reaches the later invocations through its document, and the loop
    # closes green on what init wrote.
    run insert-claim-entry done insert-claim-entry --create --values "${out}/seed.yaml"
    id="$(sed -n 's/^    - "\(clm-[a-z0-9]*\)"$/\1/p' "${LAST}" | head -n 1)"
    [[ -n "${id}" ]] || fail "the insert minted no id"
    cp "${out}/leaf.md" "${repo}/kb-root/a.md"
    sed "s/@ID@/${id}/g" "${out}/sf.json" > "${out}/sf-run.json"
    sed "s/@ID@/${id}/g" "${out}/mc.json" > "${out}/mc-run.json"
    run set-frontmatter done set-frontmatter --values "${out}/sf-run.json"
    run mark-claim-in-leaf done mark-claim-in-leaf --values "${out}/mc-run.json"
    run refresh unchanged refresh
    run verify done verify
    # 6. no dot: the placeholder stands, in its own repository, PATH holding
    # git alone through a symlink of its own.
    git init --quiet "${nodot}"
    mkdir -p "${out}/gitonly"
    ln -s "$(command -v git)" "${out}/gitonly/git"
    ( cd "${nodot}" && env PATH="${out}/gitonly" "${kbase}" init ) > "${docs}/init-nodot.out" 2> "${docs}/init-nodot.log"
    grep -q 'outcome: "done"' "${docs}/init-nodot.out" || fail "the no-dot birth did not reach done: $(cat "${docs}/init-nodot.out")"
    grep -q 'no Graphviz dot on PATH' "${docs}/init-nodot.log" || fail "the no-dot lack is not said on stderr"
    grep -q 'no Graphviz dot on PATH' "${nodot}/kb-root/claim-graph.svg" || fail "claim-graph.svg is not the placeholder"
    {{just_executable()}} _write-evidence "${out}" "test-integration-init" \
      "git init repo/ (identity fixed in the environment, global config quarantined to gitconfig)\nper invocation NN-<verb>, from repo/: ${kbase} <args> > docs/NN-<verb>.out, parsed by go test -run '^TestResultDocument\$' ./cmd -args -result.doc=docs/NN-<verb>.out -result.verb=<verb> -result.outcome=<outcome> > docs/NN-<verb>.keys.log\noutside a worktree: invocation 01-init run from a mktemp dir, whose run tree is copied to outside/; state/: the refused build's quarantined store\ndocs/init-nodot.{out,log}: init run from repo-nodot/ with PATH holding git alone\ninit.tsv: per invocation its arguments, the outcome wanted, pass or fail, and its document"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-init): a birth check failed — see init.tsv\n'
        exit 1
    fi
    printf 'integration(test-integration-init) ok\n'

# The MCP transcripts over the delivered binary: the results fixture staged as
# cmd/mcp_test.go stages it, each test-data/fixtures/mcp/*.jsonl fed to
# `./bin/kbase mcp` over real pipes and every response line compared to the
# expected line after placeholder substitution. Every transcript runs: none
# needs a stub child or a provider (build-refused refuses before it starts one).
[doc("hermetic: stage the results fixture, feed each test-data/fixtures/mcp/*.jsonl transcript to ./bin/kbase mcp over pipes and compare every response line to the expected line; evidence under test-data/transient/test-integration-mcp/")]
test-integration-mcp:
    @mkdir -p "{{MCP_OUT_DIR}}"
    @{{just_executable()}} _test-integration-mcp 2>&1 | tee "{{MCP_OUT_DIR}}/log.txt"

# The build-end leg over the delivered binary: a notification owes no request,
# which a transcript cannot express, so a Go test drives a real server — the
# recipe's child, because the exec monopoly (I1) leaves starting it to
# cmd/mcp.go inside the module and to a recipe outside it — over two named
# pipes the recipe holds, waits for the frame, and starts no process of its
# own. That leg builds a paper with no inference, so it needs pandoc.
[doc("NEEDS pandoc (excluded from test-integration): a build with no inference over test-data/fixtures/mcp/paper.tex through ./bin/kbase mcp, subscribed to kbase://build, sending nothing until the notification of its end arrives; evidence under test-data/transient/test-integration-mcp-build/")]
test-integration-mcp-build:
    @mkdir -p "{{MCP_BUILD_OUT_DIR}}"
    @{{just_executable()}} _test-integration-mcp-build 2>&1 | tee "{{MCP_BUILD_OUT_DIR}}/log.txt"

[private]
_test-integration-mcp: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd -P)"
    out="${root}/{{MCP_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    fixture="${root}/{{RESULTS_FIXTURE_DIR}}"
    transcripts="${root}/{{MCP_TRANSCRIPT_DIR}}"
    repo="${out}/repo"
    rm -rf "${repo}" "${out}/state" "${out}/transcripts" "${out}/comparison.tsv"
    mkdir -p "${repo}/.git/objects" "${repo}/.git/refs/heads" "${out}/transcripts"
    printf 'ref: refs/heads/main\n' > "${repo}/.git/HEAD"
    cp -R "${fixture}/kb-root" "${repo}/"
    mkdir "${repo}/kb-root/b"
    ( cd "${repo}" && "${kbase}" insert-claim-entry --create --values "${transcripts}/seed.yaml" ) > "${out}/insert.out" 2> "${out}/insert.log"
    ( cd "${repo}" && "${kbase}" refresh ) > "${out}/refresh.out" 2> "${out}/refresh.log"
    kb_root="$(sed -n 's/^kb-root: "\(.*\)"$/\1/p' "${out}/insert.out")"
    ids=()
    while IFS= read -r id; do ids+=("${id}"); done < <(sed -n '/^ids:/,/^minted:/s/^    - "\(clm-[a-z0-9]*\)"$/\1/p' "${out}/insert.out" | sort)
    if [[ -z "${kb_root}" || "${#ids[@]}" -ne 2 ]]; then
        printf 'integration(test-integration-mcp): the staging insert reported no kb-root and two ids — see insert.out\n' >&2
        exit 1
    fi
    version="$("${kbase}" --version)"
    version="${version##* }"
    # sub <line>: the transcript placeholders, as cmd/mcp_test.go's mcpServer
    # derives them (paths here carry no JSON-escaped characters).
    sub() {
        local s="${1}"
        s="${s//'${KB_ROOT}'/${kb_root}}"
        s="${s//'${REPO}'/${repo}}"
        s="${s//'${STATE_DIR}'/${out}/state}"
        s="${s//'${VERSION}'/${version}}"
        s="${s//'${ID1}'/${ids[0]}}"
        s="${s//'${ID2}'/${ids[1]}}"
        printf '%s' "${s}"
    }
    status=0
    for file in "${transcripts}"/*.jsonl; do
        name="$(basename "${file}" .jsonl)"
        tdir="${out}/transcripts/${name}"
        mkdir -p "${tdir}"
        : > "${tdir}/requests.jsonl"
        : > "${tdir}/expected.jsonl"
        lines=()
        while IFS= read -r line || [[ -n "${line}" ]]; do lines+=("${line}"); done < "${file}"
        if (( ${#lines[@]} % 2 != 0 )); then
            printf '  fail %s: %d lines — a transcript pairs each request with its response\n' "${name}" "${#lines[@]}"
            printf '%s\t0\todd line count\n' "${name}" >> "${out}/comparison.tsv"
            status=1
            continue
        fi
        for ((i = 0; i < ${#lines[@]}; i += 2)); do
            printf '%s\n' "$(sub "${lines[${i}]}")" >> "${tdir}/requests.jsonl"
            if [[ "${lines[$((i + 1))]}" != "null" ]]; then
                printf '%s\n' "$(sub "${lines[$((i + 1))]}")" >> "${tdir}/expected.jsonl"
            fi
        done
        set +e
        "${kbase}" mcp --kb-root "${kb_root}" < "${tdir}/requests.jsonl" > "${tdir}/actual.jsonl" 2> "${tdir}/mcp.log"
        printf '%s\n' "$?" > "${tdir}/mcp.exit"
        set -e
        bad=0
        wanted="$(wc -l < "${tdir}/expected.jsonl" | tr -d ' ')"
        # The line numbers where actual and expected differ, one per line; a
        # line present on one side only differs.
        while IFS= read -r j; do
            bad=1
            printf '  fail %s response %s:\n    got:  %s\n    want: %s\n' "${name}" "${j}" \
                "$(sed -n "${j}p" "${tdir}/actual.jsonl")" "$(sed -n "${j}p" "${tdir}/expected.jsonl")"
            printf '%s\t%s\tdiffers\n' "${name}" "${j}" >> "${out}/comparison.tsv"
        done < <(awk 'NR == FNR { w[FNR] = $0; n = FNR; next } { g[FNR] = $0; m = FNR }
            END { for (i = 1; i <= (n > m ? n : m); i++) if (g[i] != w[i]) print i }' "${tdir}/expected.jsonl" "${tdir}/actual.jsonl")
        [[ "$(cat "${tdir}/mcp.exit")" -eq 0 ]] || { bad=1; printf '  fail %s: kbase mcp exited %s — see %s\n' "${name}" "$(cat "${tdir}/mcp.exit")" "${tdir}/mcp.log"; printf '%s\t0\texit %s\n' "${name}" "$(cat "${tdir}/mcp.exit")" >> "${out}/comparison.tsv"; }
        if [[ "${bad}" -eq 0 ]]; then
            printf '  pass %s: %d response lines\n' "${name}" "${wanted}"
            printf '%s\t-\tpass\n' "${name}" >> "${out}/comparison.tsv"
        else
            status=1
        fi
    done
    {{just_executable()}} _write-evidence "${out}" "test-integration-mcp" \
      "cp -R {{RESULTS_FIXTURE_DIR}}/kb-root repo/ (with a bare .git); mkdir repo/kb-root/b\nrepo/: ${kbase} insert-claim-entry --create --values {{MCP_TRANSCRIPT_DIR}}/seed.yaml; ${kbase} refresh\nper transcript: ${kbase} mcp --kb-root ${kb_root} < transcripts/<name>/requests.jsonl > transcripts/<name>/actual.jsonl, compared line by line to transcripts/<name>/expected.jsonl\ncomparison.tsv: per transcript, the response line that differs, or pass"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-mcp): a response differs from its transcript — see comparison.tsv\n'
        exit 1
    fi
    printf 'integration(test-integration-mcp) ok\n'

[private]
_test-integration-mcp-build: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd -P)"
    out="${root}/{{MCP_BUILD_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    # The test drives the protocol and must start no process, so the server is
    # this recipe's child, its requests and its frames two named pipes the test
    # opens: it sends nothing after subscribing and waits for a frame no
    # request owes. The pipes are opened in the order that cannot deadlock —
    # the test's write end unblocks the server's `< requests`, which then opens
    # `> frames` for the test's read — and a build that never ends is caught by
    # the test runner's -timeout.
    rm -rf "${out}/repo" "${out}/state" "${out}/xdg" "${out}/config" "${out}/home" \
        "${out}/requests.fifo" "${out}/frames.fifo" "${out}/mcp.log" "${out}/test.log"
    mkdir -p "${out}/repo" "${out}/config" "${out}/home"
    cp "{{MCP_TRANSCRIPT_DIR}}/paper.tex" "${out}/repo/"
    : > "${out}/home/.gitconfig"
    git -C "${out}/repo" init -q
    git -C "${out}/repo" add -A
    git -C "${out}/repo" -c user.name=mcp -c user.email=mcp@invalid commit -qm "sources as fetched"
    mkfifo "${out}/requests.fifo" "${out}/frames.fifo"
    (
        cd "${out}/repo"
        env HOME="${out}/home" XDG_STATE_HOME="${out}/xdg" KBASE_CONFIG_DIR="${out}/config" \
            GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${out}/home/.gitconfig" \
            GIT_AUTHOR_NAME=mcp GIT_AUTHOR_EMAIL=mcp@invalid GIT_COMMITTER_NAME=mcp GIT_COMMITTER_EMAIL=mcp@invalid \
            "${kbase}" mcp --kb-root "${out}/repo" < "${out}/requests.fifo" > "${out}/frames.fifo" 2> "${out}/mcp.log"
    ) &
    server=$!
    set +e
    go test -count=1 -timeout 300s -run '^TestMCPBuildEndNotification$' ./cmd -args \
        -notify.repo="${out}/repo" -notify.state-dir="${out}/state" \
        -notify.requests="${out}/requests.fifo" -notify.frames="${out}/frames.fifo" \
        > "${out}/test.log" 2>&1
    code=$?
    set -e
    wait "${server}" || true
    {{just_executable()}} _write-evidence "${out}" "test-integration-mcp-build" \
      "cp {{MCP_TRANSCRIPT_DIR}}/paper.tex repo/; git init repo/ and one commit (identity fixed in the environment, global config quarantined to home/.gitconfig); mkfifo requests.fifo frames.fifo\nrepo/: ${kbase} mcp --kb-root repo/ < requests.fifo > frames.fifo 2> mcp.log, whose build tool launched two builds with no inference into state/\ngo test -count=1 -timeout 300s -run '^TestMCPBuildEndNotification$' ./cmd -args -notify.repo=repo -notify.state-dir=state -notify.requests=requests.fifo -notify.frames=frames.fifo > test.log: initialize, build, subscribe, send nothing until the end's notification arrives, then a second build and a subscription taken again\npass is test.log's exit 0; a frame that is not the one owing, an end announced twice, or a build that never ends fails it"
    if [[ "${code}" -ne 0 ]]; then
        printf 'integration(test-integration-mcp-build): the notification never arrived as expected — see %s\n' "${out}/test.log"
        exit 1
    fi
    printf 'integration(test-integration-mcp-build) ok\n'

# The show-config verb over the delivered binary, on three staged configuration
# directories — absent, providers.toml only, both files — each run through
# KBASE_CONFIG_DIR and through --config-dir: the outcome, the one layer each
# reports and the layer's source, the diagnostic count and the absence of the
# key-file value from stdout. A pool entry that drops and one whose key file any
# local account can read are what the diagnostics count. One further run sets
# KBASE_TEXT over the both pool, whose config.toml names both categories: the
# document must name the environment as the text category's source, and the one
# layer read for the rest. A layered leg stages a repository's own .kbase over a
# staged $HOME's ~/.config/kbase — the repository adding a pool entry and
# overriding one by name — and checks the merged pool, each entry's layer,
# whole-entry replacement, both layers in precedence order with what each holds,
# and the layer each resolved field came from. Five further legs cover a leading
# ~: the configuration directory a flag names and the one $KBASE_CONFIG_DIR
# names, ~ alone, an entry's apiKeyFile (which must expand rather than join to
# the configuration directory), ~user and a/~/b staying literal, and a home the
# lookup cannot give or one that is not absolute, which leaves the spelling
# alone. The last leg calls the verb over MCP from a directory holding no KB,
# which is what a verb resolving no KB must answer.
# It needs git only to stage the layered leg's repository; no pandoc, no
# provider and no corpus.
[doc("hermetic: ./bin/kbase show-config over staged configurations with HOME pointed at a staged home — absent, providers.toml alone and both files, each through KBASE_CONFIG_DIR and --config-dir, checked for its outcome, its mode, its one layer, that layer's existence flags, its diagnostic count, its roles and the absence of key material; a run setting KBASE_TEXT over the both files checked for the mixed-mode refusal it is, and one run supplying the environment alone checked for every role resolving on its own variable; a layered leg in a staged git repository whose .kbase adds a pool entry and overrides one by name, checked for the merged pool and each entry's layer, whole-entry replacement, each entry's decision surface (the value an entry names, systemone where it is silent), both layers in precedence order with exists/providersExists/configExists, the superseded provider key said once, and each role's entry, model, endpoint and the layer that supplied each field; legs for a leading ~ in the configuration directory a flag or the variable names, alone, and in an apiKeyFile (never joined to the configuration directory), with ~user and a/~/b staying literal, and with a home missing or not absolute; configure --create --project in a staged repository, checked for creating both files unchecked and without a request, for show-config reporting that layer holding both with models resolving the sole entry [local], for a second call coming back unchanged, for a --provider the pool lacks refusing against the absolute providers.toml with the creation remedy and writing nothing, for --config-dir beside --project refusing as usage, for --config-dir into an empty directory creating both files, and for --project outside a worktree refusing on the worktree; and a dial-free configure writing three references that show-config resolves for every role with no diagnostic, the writer and the resolver agreeing on the reference form; and one show-config tools/call over MCP from a directory holding no KB; evidence under test-data/transient/test-integration-config/")]
test-integration-config:
    @mkdir -p "{{CONFIG_OUT_DIR}}"
    @{{just_executable()}} _test-integration-config 2>&1 | tee "{{CONFIG_OUT_DIR}}/log.txt"

[private]
_test-integration-config: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd -P)"
    out="${root}/{{CONFIG_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    dirs="${out}/configs"
    docs="${out}/docs"
    work="${out}/work"
    rm -rf "${dirs}" "${docs}" "${work}" "${out}/home" "${out}/layered" "${out}/literal" "${out}/configure" "${out}/mcp.requests.jsonl" "${out}/mcp.actual.jsonl" "${out}/mcp.log" "${out}/mcp.exit" "${out}/config.tsv"
    mkdir -p "${dirs}/pool" "${dirs}/both" "${docs}" "${work}"
    file_key='sk-file-MUST-NOT-APPEAR'
    # The home every invocation runs under, so the global layer is this staged
    # ~/.config/kbase and no leg reads or writes the operator's own. It is left
    # empty: a leg that names a directory reads that one layer only.
    home="${out}/home"
    mkdir -p "${home}"
    # One providers.toml, staged into the two states that hold it: an entry that
    # drops for want of a baseUrl and one supplying its key from a file any local
    # account can read — so the permission warning fires. configs/absent is never
    # created: that is the absent state.
    printf '%s\n' \
        '[broken]' \
        '' '[file]' 'baseUrl = "http://file.example/v1"' 'apiKeyFile = "key.txt"' \
        > "${dirs}/pool/providers.toml"
    printf '%s\n' "${file_key}" > "${dirs}/pool/key.txt"
    chmod 0644 "${dirs}/pool/key.txt"
    cp -R "${dirs}/pool/." "${dirs}/both/"
    printf '%s\n' 'provider = "file"' '' '[models]' 'decision = "file/fixture-decision"' 'text = "file/fixture-text"' 'scoring = "file/fixture-scoring"' \
        > "${dirs}/both/config.toml"
    status=0
    fail() { printf '  fail: %s\n' "$*"; status=1; }
    # role_field <document> <role> <key>: one scalar of the roles entry the named
    # role is; the entry's first key sits on its "- role:" line, the rest below it.
    role_field() { awk -v r="$2" -v k="$3" '/^roles:/ { on = 1; next } on && /^    - role: / { cur = $0; sub(/^[^:]*: /, "", cur); gsub(/^"|"$/, "", cur); next } on && /^    / { if (cur == r && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "$1"; }
    # role_source <document> <role> <key>: the layer the document names for one
    # field of that role's sources record.
    role_source() { awk -v r="$2" -v k="$3" '/^roles:/ { on = 1; next } on && /^    - role: / { cur = $0; sub(/^[^:]*: /, "", cur); gsub(/^"|"$/, "", cur); next } on && /^    / { if (cur == r && index($0, k ": ") == 9) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "$1"; }
    # selection_field <document> <key>: one scalar of the selection record.
    selection_field() { awk -v k="$2" '/^selection:/ { on = 1; next } on && /^    / { if (index($0, k ": ") == 5) { sub(/^[^:]*: /, ""); gsub(/^"|"$/, ""); print; exit }; next } on { exit }' "$1"; }
    # item_field <document> <list> <key>: one scalar of an item under the named
    # list (refusals, failures); the item's first key sits on its "- " line, the
    # rest below it.
    item_field() { awk -v list="$2" -v k="$3" '$0 == list ":" { on = 1; next } on && /^    - / { line = "      " substr($0, 7); if (index(line, k ": ") == 7) { sub(/^[^:]*: /, "", line); gsub(/^"|"$/, "", line); print line; exit } next } on && /^      / { if (index($0, k ": ") == 7) { sub(/^[^:]*: /, ""); gsub(/^"|"$/, ""); print; exit }; next } on { exit }' "$1"; }
    # diagnostics <stem>: how many items the document's diagnostics list holds.
    diagnostics() { awk '/^diagnostics:/ { on = 1; next } on && /^    - / { n++ } END { print n + 0 }' "$1.out"; }
    # layers <stem>: how many layer entries the document reports.
    layers() { awk '/^layers:/ { on = 1; next } on && /^    - / { n++; next } on && /^    / { next } on { exit } END { print n + 0 }' "$1.out"; }
    # layer_field <stem> <n> <key>: one scalar of the nth layer entry (1-based);
    # the entry's first key sits on its "- path:" line, the rest below it.
    layer_field() { awk -v n="$2" -v k="$3" '/^layers:/ { on = 1; next } on && /^    - / { i++; if (i == n && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit } next } on && /^    / { if (i == n && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "$1.out"; }
    # provider_names <stem>: the pool's entry names, in the order the document lists them.
    provider_names() { awk '/^providers:/ { on = 1; next } on && /^    - name: / { n = $0; sub(/^[^:]*: /, "", n); gsub(/^"|"$/, "", n); printf "%s%s", sep, n; sep = ","; next } on && /^    / { next } on { exit }' "$1.out"; }
    # provider_field <stem> <name> <key>: one scalar of the providers entry called name.
    provider_field() { awk -v want="$2" -v k="$3" '/^providers:/ { on = 1; next } on && /^    - name: / { cur = $0; sub(/^[^:]*: /, "", cur); gsub(/^"|"$/, "", cur); next } on && /^    / { if (cur == want && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "$1.out"; }
    # report <stem> <ok> <label> [detail]: one leg that checks its document
    # itself — its config.tsv line, its console line, and the run's status. The
    # console line states the layer and diagnostic counts a show-config document
    # carries, or detail where the leg's document is not one.
    report() {
        local verdict=pass
        [[ "$2" -eq 0 ]] || { verdict=fail; status=1; }
        printf '%s\t-\t%s\t%s\n' "$3" "${verdict}" "$1.out" >> "${out}/config.tsv"
        printf '  %s %s: %s, exit %s\n' \
            "${verdict}" "$3" "${4:-$(layers "$1") layers, $(diagnostics "$1") diagnostics}" "$(cat "$1.exit")"
    }

    # run <state> <route> <diagnostics> <files> [args...]: one invocation
    # against the staged configuration named by state, its document checked for
    # the outcome, the one layer it reads, that layer's source (the environment
    # for a route through KBASE_CONFIG_DIR, explicit for --config-dir), which of
    # its files stand (files: none, pool or both), the count of diagnostics, and
    # the absence of the key-file value. HOME points at the staged home, so a
    # leg never reads the operator's own, and the inference variables are
    # scrubbed, so the document describes the staged configuration and nothing
    # the host exports; extra adds assignments after the scrubs, for a leg that
    # supplies one of them.
    extra=()
    n=0
    run() {
        local state="$1" route="$2" want_count="$3" files="$4" dir stem want_source
        shift 4
        n=$((n + 1))
        dir="${dirs}/${state}"
        stem="${docs}/$(printf '%02d' "${n}")-${state}-${route}"
        local -a envargs=(-u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE "HOME=${home}")
        envargs+=(${extra[@]+"${extra[@]}"})
        local -a args=(show-config)
        if [[ "${route}" == environment ]]; then
            want_source=environment
            envargs+=("KBASE_CONFIG_DIR=${dir}")
        else
            want_source=explicit
            args+=(--config-dir "${dir}")
        fi
        set +e
        ( cd "${work}" && env "${envargs[@]}" "${kbase}" "${args[@]}" "$@" ) > "${stem}.out" 2> "${stem}.log"
        printf '%s\n' "$?" > "${stem}.exit"
        set -e
        local ok=0 got_source got_count invocation="${args[*]}"
        local want_exists=false want_providers=false want_config=false
        case "${files}" in
            none) ;;
            pool) want_exists=true; want_providers=true ;;
            both) want_exists=true; want_providers=true; want_config=true ;;
            *) ok=1 ;;
        esac
        [[ "${route}" == environment ]] && invocation="KBASE_CONFIG_DIR=${dir} ${invocation}"
        [[ ${#extra[@]} -eq 0 ]] || invocation="${extra[*]} ${invocation}"
        got_source="$(layer_field "${stem}" 1 source)"
        got_count="$(diagnostics "${stem}")"
        [[ "$(cat "${stem}.exit")" -eq 0 ]] || ok=1
        grep -qx 'outcome: "done"' "${stem}.out" || ok=1
        [[ "$(layers "${stem}")" == 1 ]] || ok=1
        [[ "${got_source}" == "${want_source}" ]] || ok=1
        [[ "${got_count}" == "${want_count}" ]] || ok=1
        [[ "$(layer_field "${stem}" 1 path)" == "${dir}" ]] || ok=1
        [[ "$(layer_field "${stem}" 1 exists)" == "${want_exists}" ]] || ok=1
        [[ "$(layer_field "${stem}" 1 providersExists)" == "${want_providers}" ]] || ok=1
        [[ "$(layer_field "${stem}" 1 configExists)" == "${want_config}" ]] || ok=1
        # With no inference variable set, the files supply inference and the
        # document names that mode; a leg that sets one is checked for the
        # refusal that sets the mode nowhere.
        if [[ ${#extra[@]} -eq 0 ]]; then grep -qx 'mode: "file"' "${stem}.out" || ok=1; fi
        if grep -qF "${file_key}" "${stem}.out"; then ok=1; fi
        printf '%s\t%s\t%s\t%s\n' "${invocation}" "${want_source}" "$([[ "${ok}" -eq 0 ]] && echo pass || echo fail)" "${stem}.out" >> "${out}/config.tsv"
        printf '  %s %s %s: source %s, files %s, %s diagnostics, exit %s\n' \
            "$([[ "${ok}" -eq 0 ]] && echo pass || echo fail)" "${state}" "${route}" "${got_source:-none}" "${files}" "${got_count}" "$(cat "${stem}.exit")"
        [[ "${ok}" -eq 0 ]] || status=1
        LAST="${stem}.out"
    }
    # 1. absent: no configuration directory at all — nothing selected, and one
    #    diagnostic per role, each naming the models.<role> key the missing
    #    config.toml would carry.
    run absent environment 3 none
    run absent flag 3 none
    # 2. providers.toml only: the drop and the key file's warning, and one
    #    diagnostic per role: a pool with no config.toml names no model.
    run pool environment 5 pool
    run pool flag 5 pool
    # 3. both files: config.toml names the key-file entry in each role's
    #    reference, so every role resolves and the drop and the warning are all
    #    that is diagnosed. Its superseded provider key is said once on stderr.
    run both environment 2 both
    run both flag 2 both
    [[ "$(grep -c 'superseded provider key' "${LAST%.out}.log")" -eq 1 ]] || fail "the superseded provider key is not said once in ${LAST%.out}.log"
    [[ "$(role_field "${LAST}" decision provider)" == file ]] || fail "the decision role does not name the pool entry its reference names in ${LAST}"
    [[ "$(role_field "${LAST}" decision model)" == fixture-decision ]] || fail "the decision role does not name its reference's model part in ${LAST}"
    [[ "$(role_field "${LAST}" decision back-end)" == provider ]] || fail "the decision role does not name its back end in ${LAST}"
    [[ "$(role_source "${LAST}" text model)" == explicit ]] || fail "the text role's model does not name the layer that supplied it in ${LAST}"
    [[ "$(role_source "${LAST}" text key)" == explicit ]] || fail "the text role's key does not name the layer that supplied it in ${LAST}"
    [[ "$(grep -c 'usable: false' "${LAST}")" -eq 1 ]] || fail "the dropped entry is not reported unusable in ${LAST}"
    # 4. KBASE_TEXT over the both files: the two sources are exclusive, so the
    #    mode is undecided — `mode` null, no role resolved, and one item per
    #    side: the variable, and each [models] key that stands. Nothing is
    #    overridden: the refusal is the whole of what the run says.
    extra=(KBASE_TEXT=fixture-env-text)
    run both flag 6 both
    extra=()
    grep -qx 'mode: null' "${LAST}" || fail "a mixed mode reports a mode in ${LAST}"
    [[ "$(role_field "${LAST}" text back-end)" == "" ]] || fail "a role resolved while the mode is undecided in ${LAST}"
    for item in KBASE_TEXT models.decision models.text models.scoring; do
        grep -qF "key: \"${item}\"" "${LAST}" || fail "the mix does not name ${item} in ${LAST}"
    done
    # 4b. the environment alone, with no configuration directory at all: the
    #     base URL and the three role variables supply every endpoint and model,
    #     so the mode is environment and every role names that back end.
    env_stem="${docs}/07b-environment-mode"
    set +e
    ( cd "${work}" && env -u KBASE_CONFIG_DIR -u KBASE_API_KEY_FILE "HOME=${home}" \
        KBASE_API_BASE_URL=http://env.example/v1 KBASE_DECISION=env-decision KBASE_TEXT=env-text KBASE_SCORING=env-scoring \
        "${kbase}" show-config --config-dir "${out}/environment-mode-cfg" ) > "${env_stem}.out" 2> "${env_stem}.log"
    printf '%s\n' "$?" > "${env_stem}.exit"
    set -e
    env_ok=0
    [[ "$(cat "${env_stem}.exit")" -eq 0 ]] || env_ok=1
    grep -qx 'outcome: "done"' "${env_stem}.out" || env_ok=1
    grep -qx 'mode: "environment"' "${env_stem}.out" || env_ok=1
    [[ "$(diagnostics "${env_stem}")" == 0 ]] || env_ok=1
    for role in decision text scoring; do
        [[ "$(role_field "${env_stem}.out" "${role}" back-end)" == environment ]] || env_ok=1
        [[ "$(role_field "${env_stem}.out" "${role}" base-url)" == http://env.example/v1 ]] || env_ok=1
    done
    [[ "$(role_field "${env_stem}.out" scoring model)" == env-scoring ]] || env_ok=1
    report "${env_stem}" "${env_ok}" "show-config with the environment alone" "mode environment, every role on its own variable"
    # 5. layered: <repository>/.kbase over the staged home's ~/.config/kbase,
    #    run from a directory below the repository's root. The repository pool
    #    adds one entry and overrides one by name; its config.toml overrides the
    #    decision role's reference and leaves text and scoring to the global file.
    layered="${out}/layered"
    repo="${layered}/repo"
    mkdir -p "${layered}/home/.config/kbase" "${repo}/.kbase" "${repo}/nested"
    printf '%s\n' \
        '[shared]' 'baseUrl = "http://shared.example/v1"' 'apiKeyFile = "shared.key"' \
        '' '[inherited]' 'baseUrl = "http://inherited.example/v1"' \
        > "${layered}/home/.config/kbase/providers.toml"
    printf '%s\n' "${file_key}-global" > "${layered}/home/.config/kbase/shared.key"
    chmod 0600 "${layered}/home/.config/kbase/shared.key"
    printf '%s\n' 'provider = "inherited"' '' '[models]' 'decision = "inherited/global-decision"' 'text = "inherited/global-text"' 'scoring = "inherited/global-scoring"' \
        > "${layered}/home/.config/kbase/config.toml"
    printf '%s\n' \
        '[shared]' 'baseUrl = "http://shared.example/v2"' 'decisionsFlavor = "decisions"' \
        '' '[local]' 'baseUrl = "http://local.example/v1"' 'apiKeyFile = "local.key"' \
        > "${repo}/.kbase/providers.toml"
    printf '%s\n' "${file_key}-repo" > "${repo}/.kbase/local.key"
    chmod 0600 "${repo}/.kbase/local.key"
    printf '%s\n' 'provider = "local"' '' '[models]' 'decision = "local/repo-decision"' \
        > "${repo}/.kbase/config.toml"
    git init -q "${repo}"
    layered_stem="${docs}/08-layered"
    set +e
    ( cd "${repo}/nested" && env -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE \
        HOME="${layered}/home" "${kbase}" show-config ) > "${layered_stem}.out" 2> "${layered_stem}.log"
    printf '%s\n' "$?" > "${layered_stem}.exit"
    set -e
    layered_ok=0
    [[ "$(cat "${layered_stem}.exit")" -eq 0 ]] || layered_ok=1
    grep -qx 'outcome: "done"' "${layered_stem}.out" || layered_ok=1
    [[ "$(diagnostics "${layered_stem}")" == 0 ]] || layered_ok=1
    if grep -qF "${file_key}" "${layered_stem}.out"; then layered_ok=1; fi
    # (a) the pool is the union of both layers, each entry naming its own.
    [[ "$(provider_names "${layered_stem}")" == "inherited,local,shared" ]] || layered_ok=1
    [[ "$(provider_field "${layered_stem}" inherited layer)" == "global" ]] || layered_ok=1
    [[ "$(provider_field "${layered_stem}" local layer)" == "repository" ]] || layered_ok=1
    [[ "$(provider_field "${layered_stem}" shared layer)" == "repository" ]] || layered_ok=1
    # (b) the repository entry replaced the global one whole: its URL stands, and
    #     neither the apiKeyFile nor the key the global entry supplied came with
    #     it.
    [[ "$(provider_field "${layered_stem}" shared base-url)" == "http://shared.example/v2" ]] || layered_ok=1
    [[ "$(provider_field "${layered_stem}" shared key)" == "none" ]] || layered_ok=1
    [[ -z "$(provider_field "${layered_stem}" shared key-file)" ]] || layered_ok=1
    # (b) Each entry states the decision surface it speaks: the repository's
    #     shared entry names one, and the local entry, silent, resolves to the
    #     systemone default.
    [[ "$(provider_field "${layered_stem}" shared decisions-flavor)" == "decisions" ]] || layered_ok=1
    [[ "$(provider_field "${layered_stem}" local decisions-flavor)" == "systemone" ]] || layered_ok=1
    # (c) both layers in precedence order, each with what it holds.
    [[ "$(layers "${layered_stem}")" == 2 ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 1 path)" == "${repo}/.kbase" ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 1 source)" == "repository" ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 1 exists)" == true ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 1 providersExists)" == true ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 1 configExists)" == true ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 2 path)" == "${layered}/home/.config/kbase" ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 2 source)" == "global" ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 2 exists)" == true ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 2 providersExists)" == true ]] || layered_ok=1
    [[ "$(layer_field "${layered_stem}" 2 configExists)" == true ]] || layered_ok=1
    # (d) each role names the layer that supplied it: the repository's pool entry
    #     and its config.toml for the decision role, the global file and the
    #     entry it names for text and scoring, which the repository left alone.
    [[ "$(role_field "${layered_stem}.out" decision provider)" == "local" ]] || layered_ok=1
    [[ "$(role_field "${layered_stem}.out" decision provider-layer)" == "repository" ]] || layered_ok=1
    [[ "$(role_field "${layered_stem}.out" decision model)" == "repo-decision" ]] || layered_ok=1
    [[ "$(role_field "${layered_stem}.out" text provider)" == "inherited" ]] || layered_ok=1
    [[ "$(role_field "${layered_stem}.out" text model)" == "global-text" ]] || layered_ok=1
    [[ "$(role_field "${layered_stem}.out" scoring model)" == "global-scoring" ]] || layered_ok=1
    [[ "$(role_field "${layered_stem}.out" decision base-url)" == "http://local.example/v1" ]] || layered_ok=1
    [[ "$(role_field "${layered_stem}.out" text base-url)" == "http://inherited.example/v1" ]] || layered_ok=1
    [[ "$(role_source "${layered_stem}.out" decision base-url)" == "repository" ]] || layered_ok=1
    [[ "$(role_source "${layered_stem}.out" decision key)" == "repository" ]] || layered_ok=1
    [[ "$(role_source "${layered_stem}.out" decision model)" == "repository" ]] || layered_ok=1
    [[ "$(role_source "${layered_stem}.out" text base-url)" == "global" ]] || layered_ok=1
    [[ "$(role_source "${layered_stem}.out" text model)" == "global" ]] || layered_ok=1
    # The global entry names no key, so the role it supplies names no key source:
    # null, not a layer.
    [[ "$(role_source "${layered_stem}.out" text key)" == null ]] || layered_ok=1
    grep -qx 'mode: "file"' "${layered_stem}.out" || layered_ok=1
    [[ "$(grep -c 'superseded provider key' "${layered_stem}.log")" -eq 1 ]] || layered_ok=1
    report "${layered_stem}" "${layered_ok}" "layered show-config"
    # 6. ~ expands. A leading ~ in the directory a flag names and in an entry's
    #    apiKeyFile resolves from the staged home. The key file stands under the
    #    home alone, so an apiKeyFile joined to the configuration directory
    #    instead would drop its entry: the diagnostic count and the entry's
    #    usability are what say it did not. The report keeps the entry's own
    #    spelling of the path.
    home_cfg="${home}/cfg"
    mkdir -p "${home_cfg}" "${home}/keys"
    printf '%s\n' "${file_key}" > "${home}/keys/home.txt"
    chmod 0600 "${home}/keys/home.txt"
    printf '%s\n' '[homekey]' 'baseUrl = "http://home.example/v1"' 'apiKeyFile = "~/keys/home.txt"' \
        > "${home_cfg}/providers.toml"
    printf '%s\n' 'provider = "homekey"' '' '[models]' 'decision = "homekey/home-decision"' 'text = "homekey/home-text"' 'scoring = "homekey/home-scoring"' \
        > "${home_cfg}/config.toml"
    tilde_stem="${docs}/09-tilde-flag"
    set +e
    ( cd "${work}" && env -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE \
        HOME="${home}" "${kbase}" show-config --config-dir '~/cfg' ) > "${tilde_stem}.out" 2> "${tilde_stem}.log"
    printf '%s\n' "$?" > "${tilde_stem}.exit"
    set -e
    tilde_ok=0
    [[ "$(cat "${tilde_stem}.exit")" -eq 0 ]] || tilde_ok=1
    grep -qx 'outcome: "done"' "${tilde_stem}.out" || tilde_ok=1
    [[ "$(layers "${tilde_stem}")" == 1 ]] || tilde_ok=1
    [[ "$(layer_field "${tilde_stem}" 1 path)" == "${home_cfg}" ]] || tilde_ok=1
    [[ "$(layer_field "${tilde_stem}" 1 source)" == "explicit" ]] || tilde_ok=1
    [[ "$(diagnostics "${tilde_stem}")" == 0 ]] || tilde_ok=1
    [[ "$(provider_field "${tilde_stem}" homekey usable)" == true ]] || tilde_ok=1
    [[ "$(provider_field "${tilde_stem}" homekey key)" == file ]] || tilde_ok=1
    [[ "$(provider_field "${tilde_stem}" homekey key-file)" == '~/keys/home.txt' ]] || tilde_ok=1
    if grep -qF "${file_key}" "${tilde_stem}.out"; then tilde_ok=1; fi
    report "${tilde_stem}" "${tilde_ok}" "show-config --config-dir '~/cfg'"
    # 7. $KBASE_CONFIG_DIR expands the same way.
    tilde_env_stem="${docs}/10-tilde-environment"
    set +e
    ( cd "${work}" && env -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE \
        HOME="${home}" KBASE_CONFIG_DIR='~/cfg' "${kbase}" show-config ) > "${tilde_env_stem}.out" 2> "${tilde_env_stem}.log"
    printf '%s\n' "$?" > "${tilde_env_stem}.exit"
    set -e
    tilde_env_ok=0
    [[ "$(cat "${tilde_env_stem}.exit")" -eq 0 ]] || tilde_env_ok=1
    grep -qx 'outcome: "done"' "${tilde_env_stem}.out" || tilde_env_ok=1
    [[ "$(layers "${tilde_env_stem}")" == 1 ]] || tilde_env_ok=1
    [[ "$(layer_field "${tilde_env_stem}" 1 path)" == "${home_cfg}" ]] || tilde_env_ok=1
    [[ "$(layer_field "${tilde_env_stem}" 1 source)" == "environment" ]] || tilde_env_ok=1
    [[ "$(diagnostics "${tilde_env_stem}")" == 0 ]] || tilde_env_ok=1
    report "${tilde_env_stem}" "${tilde_env_ok}" "KBASE_CONFIG_DIR='~/cfg' show-config"
    # 8. ~ alone is the home directory: the layer is the staged home, holding
    #    neither configuration file.
    tilde_home_stem="${docs}/11-tilde-home"
    set +e
    ( cd "${work}" && env -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE \
        HOME="${home}" "${kbase}" show-config --config-dir '~' ) > "${tilde_home_stem}.out" 2> "${tilde_home_stem}.log"
    printf '%s\n' "$?" > "${tilde_home_stem}.exit"
    set -e
    tilde_home_ok=0
    [[ "$(cat "${tilde_home_stem}.exit")" -eq 0 ]] || tilde_home_ok=1
    grep -qx 'outcome: "done"' "${tilde_home_stem}.out" || tilde_home_ok=1
    [[ "$(layers "${tilde_home_stem}")" == 1 ]] || tilde_home_ok=1
    [[ "$(layer_field "${tilde_home_stem}" 1 path)" == "${home}" ]] || tilde_home_ok=1
    [[ "$(layer_field "${tilde_home_stem}" 1 exists)" == true ]] || tilde_home_ok=1
    [[ "$(layer_field "${tilde_home_stem}" 1 providersExists)" == false ]] || tilde_home_ok=1
    [[ "$(layer_field "${tilde_home_stem}" 1 configExists)" == false ]] || tilde_home_ok=1
    report "${tilde_home_stem}" "${tilde_home_ok}" "show-config --config-dir '~'"
    # 9. A ~ anywhere but the start is literal: the two key files below stand
    #    under names that only a join to the configuration directory reaches, so
    #    an expansion of ~user or of a/~/b would drop the entry naming it.
    literal="${out}/literal"
    mkdir -p "${literal}/~user" "${literal}/a/~/b"
    printf '%s\n' "${file_key}" > "${literal}/~user/key.txt"
    printf '%s\n' "${file_key}" > "${literal}/a/~/b/key.txt"
    chmod 0600 "${literal}/~user/key.txt" "${literal}/a/~/b/key.txt"
    printf '%s\n' \
        '[userdir]' 'baseUrl = "http://user.example/v1"' 'apiKeyFile = "~user/key.txt"' \
        '' '[inside]' 'baseUrl = "http://inside.example/v1"' 'apiKeyFile = "a/~/b/key.txt"' \
        > "${literal}/providers.toml"
    printf '%s\n' 'provider = "userdir"' '' '[models]' 'decision = "userdir/literal-decision"' 'text = "userdir/literal-text"' 'scoring = "userdir/literal-scoring"' \
        > "${literal}/config.toml"
    literal_stem="${docs}/12-tilde-literal"
    set +e
    ( cd "${work}" && env -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE \
        HOME="${home}" "${kbase}" show-config --config-dir "${literal}" ) > "${literal_stem}.out" 2> "${literal_stem}.log"
    printf '%s\n' "$?" > "${literal_stem}.exit"
    set -e
    literal_ok=0
    [[ "$(cat "${literal_stem}.exit")" -eq 0 ]] || literal_ok=1
    grep -qx 'outcome: "done"' "${literal_stem}.out" || literal_ok=1
    [[ "$(diagnostics "${literal_stem}")" == 0 ]] || literal_ok=1
    [[ "$(provider_names "${literal_stem}")" == "inside,userdir" ]] || literal_ok=1
    [[ "$(provider_field "${literal_stem}" userdir usable)" == true ]] || literal_ok=1
    [[ "$(provider_field "${literal_stem}" inside usable)" == true ]] || literal_ok=1
    [[ "$(provider_field "${literal_stem}" userdir key-file)" == '~user/key.txt' ]] || literal_ok=1
    [[ "$(provider_field "${literal_stem}" inside key-file)" == 'a/~/b/key.txt' ]] || literal_ok=1
    report "${literal_stem}" "${literal_ok}" "show-config --config-dir with ~ not leading"
    # 10. A home the lookup cannot give, or one that is not an absolute path,
    #     leaves the spelling as written: the layer is the literal string, and
    #     nothing stands there.
    leg=13
    for home_value in "" "relative/home"; do
        label=no-home
        [[ -z "${home_value}" ]] || label=relative-home
        stem="${docs}/$(printf '%02d' "${leg}")-tilde-${label}"
        leg=$((leg + 1))
        set +e
        ( cd "${work}" && env -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE \
            HOME="${home_value}" "${kbase}" show-config --config-dir '~/cfg' ) > "${stem}.out" 2> "${stem}.log"
        printf '%s\n' "$?" > "${stem}.exit"
        set -e
        ok=0
        [[ "$(cat "${stem}.exit")" -eq 0 ]] || ok=1
        grep -qx 'outcome: "done"' "${stem}.out" || ok=1
        [[ "$(layers "${stem}")" == 1 ]] || ok=1
        [[ "$(layer_field "${stem}" 1 path)" == '~/cfg' ]] || ok=1
        [[ "$(layer_field "${stem}" 1 source)" == "explicit" ]] || ok=1
        [[ "$(layer_field "${stem}" 1 exists)" == false ]] || ok=1
        report "${stem}" "${ok}" "show-config --config-dir '~/cfg' with HOME='${home_value}'"
    done
    # 11. configure --create in a repository: --project names <repo>/.kbase
    #     whether or not it stands, so one call creates both files there from
    #     their templates and names them as written. No --provider is given, so
    #     nothing is dialed and stderr says the ids went unchecked; show-config
    #     then reports that layer holding both, and models resolves the one live
    #     pool entry — [local] — by the sole-entry rule. A second identical call
    #     finds every file holding what it would write.
    cdir="${out}/configure"
    crepo="${cdir}/repo"
    chome="${cdir}/home"
    mkdir -p "${crepo}/nested" "${chome}"
    git init --quiet "${crepo}"
    # scrub: what every configure leg runs under — the five inference variables
    # and the configuration-directory variable unset, so the leg reads and
    # writes the staged tree alone, and HOME on the staged home.
    scrub=(-u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE -u KBASE_CONFIG_DIR "HOME=${chome}")
    create_stem="${docs}/15-configure-create-project"
    set +e
    ( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project ) > "${create_stem}.out" 2> "${create_stem}.log"
    printf '%s\n' "$?" > "${create_stem}.exit"
    set -e
    create_ok=0
    [[ "$(cat "${create_stem}.exit")" -eq 0 ]] || create_ok=1
    grep -qx 'outcome: "done"' "${create_stem}.out" || create_ok=1
    grep -qx 'provider: null' "${create_stem}.out" || create_ok=1
    grep -qx "    - \"${crepo}/.kbase/providers.toml\"" "${create_stem}.out" || create_ok=1
    grep -qx "    - \"${crepo}/.kbase/config.toml\"" "${create_stem}.out" || create_ok=1
    # The two files in the order --create writes them: the pool, then the
    # choices.
    [[ "$(sed -n 's/^    - "\(.*\)"$/\1/p' "${create_stem}.out" | tr '\n' ' ')" == \
        "${crepo}/.kbase/providers.toml ${crepo}/.kbase/config.toml " ]] || create_ok=1
    grep -q 'the references were not checked' "${create_stem}.log" || create_ok=1
    grep -qx '\[local\]' "${crepo}/.kbase/providers.toml" || create_ok=1
    # Nothing else is written: the harness profiles a run may execute are
    # kbase's, so there is no file a user stocks with commands.
    [[ ! -e "${crepo}/.kbase/harnesses.toml" ]] || create_ok=1
    report "${create_stem}" "${create_ok}" "configure --create --project" "created ${crepo}/.kbase from the two templates, ids unchecked"
    created_show="${docs}/16-configure-created-layer"
    set +e
    ( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" show-config ) > "${created_show}.out" 2> "${created_show}.log"
    printf '%s\n' "$?" > "${created_show}.exit"
    set -e
    created_show_ok=0
    [[ "$(cat "${created_show}.exit")" -eq 0 ]] || created_show_ok=1
    [[ "$(layers "${created_show}")" == 2 ]] || created_show_ok=1
    [[ "$(layer_field "${created_show}" 1 path)" == "${crepo}/.kbase" ]] || created_show_ok=1
    [[ "$(layer_field "${created_show}" 1 source)" == repository ]] || created_show_ok=1
    [[ "$(layer_field "${created_show}" 1 exists)" == true ]] || created_show_ok=1
    [[ "$(layer_field "${created_show}" 1 providersExists)" == true ]] || created_show_ok=1
    [[ "$(layer_field "${created_show}" 1 configExists)" == true ]] || created_show_ok=1
    [[ "$(selection_field "${created_show}.out" provider)" == local ]] || created_show_ok=1
    [[ "$(selection_field "${created_show}.out" rule)" == sole-entry ]] || created_show_ok=1
    # The template writes the model keys blank, which is exactly the
    # fresh-install diagnostic: one item per role, naming models.<role>.
    [[ "$(diagnostics "${created_show}")" == 3 ]] || created_show_ok=1
    for role in decision text scoring; do
        grep -qF "key: \"models.${role}\"" "${created_show}.out" || created_show_ok=1
    done
    report "${created_show}" "${created_show_ok}" "show-config over the created repository layer"
    created_models="${docs}/17-configure-models"
    set +e
    ( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" models --timeout 300ms ) > "${created_models}.out" 2> "${created_models}.log"
    printf '%s\n' "$?" > "${created_models}.exit"
    set -e
    created_models_ok=0
    grep -qx 'provider: "local"' "${created_models}.out" || created_models_ok=1
    # The template's live endpoint is a host that does not answer here, so the
    # request may fail; what this leg checks is that the sole entry was chosen,
    # which a refusal would contradict.
    if grep -qx 'outcome: "refused"' "${created_models}.out"; then created_models_ok=1; fi
    report "${created_models}" "${created_models_ok}" "models over the created pool" "the sole entry local selected"
    again_stem="${docs}/18-configure-unchanged"
    set +e
    ( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project ) > "${again_stem}.out" 2> "${again_stem}.log"
    printf '%s\n' "$?" > "${again_stem}.exit"
    set -e
    again_ok=0
    [[ "$(cat "${again_stem}.exit")" -eq 0 ]] || again_ok=1
    grep -qx 'outcome: "unchanged"' "${again_stem}.out" || again_ok=1
    grep -qx 'written: \[\]' "${again_stem}.out" || again_ok=1
    report "${again_stem}" "${again_ok}" "configure --create --project, second call" "both files already hold what it would write"
    # 32. A file a user left in the layer is theirs: --create adds nothing to it
    #     — the templates kbase writes today included — and reports every file
    #     unwritten, so a file a user deleted stays deleted. The file here is
    #     the harnesses.toml an older kbase seeded: it is read by nothing, and
    #     the run neither rewrites nor removes it.
    user_harnesses="${crepo}/.kbase/harnesses.toml"
    printf '%s\n' '# mine' '' '[only-mine]' 'command = ["mine", "{prompt}"]' > "${user_harnesses}"
    cp "${user_harnesses}" "${out}/harnesses-expected.toml"
    hseed_stem="${docs}/18b-configure-existing-file"
    set +e
    ( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project ) > "${hseed_stem}.out" 2> "${hseed_stem}.log"
    printf '%s\n' "$?" > "${hseed_stem}.exit"
    set -e
    hseed_ok=0
    [[ "$(cat "${hseed_stem}.exit")" -eq 0 ]] || hseed_ok=1
    grep -qx 'outcome: "unchanged"' "${hseed_stem}.out" || hseed_ok=1
    grep -qx 'written: \[\]' "${hseed_stem}.out" || hseed_ok=1
    cmp -s "${user_harnesses}" "${out}/harnesses-expected.toml" || hseed_ok=1
    report "${hseed_stem}" "${hseed_ok}" "configure --create --project over a leftover harnesses.toml" "every file unwritten and the leftover untouched"
    hshow_stem="${docs}/18c-harnesses-leftover"
    set +e
    ( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" show-config ) > "${hshow_stem}.out" 2> "${hshow_stem}.log"
    printf '%s\n' "$?" > "${hshow_stem}.exit"
    set -e
    hshow_ok=0
    [[ "$(cat "${hshow_stem}.exit")" -eq 0 ]] || hshow_ok=1
    # The report lists the layer's two files and no third: a leftover
    # harnesses.toml is a file nothing reads, and the report says nothing of it.
    [[ "$(layer_field "${hshow_stem}" 1 providersExists)" == true ]] || hshow_ok=1
    [[ "$(layer_field "${hshow_stem}" 1 configExists)" == true ]] || hshow_ok=1
    if grep -q '^harnesses' "${hshow_stem}.out"; then hshow_ok=1; fi
    report "${hshow_stem}" "${hshow_ok}" "show-config over a layer holding a leftover harnesses.toml" "two files reported, the leftover unread"
    # 12. A --provider the pool does not hold refuses before anything is
    #     written: the item names the target layer's providers.toml by its
    #     absolute path, and offers the creation remedy. Both files still hold
    #     exactly what the previous call left.
    pool_before="$(cksum "${crepo}/.kbase/providers.toml" "${crepo}/.kbase/config.toml")"
    unknown_stem="${docs}/19-configure-unknown-provider"
    set +e
    ( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project \
        --provider nosuch --model-map decision=nosuch/d,text=nosuch/t,scoring=nosuch/s ) > "${unknown_stem}.out" 2> "${unknown_stem}.log"
    printf '%s\n' "$?" > "${unknown_stem}.exit"
    set -e
    unknown_ok=0
    [[ "$(cat "${unknown_stem}.exit")" -eq 1 ]] || unknown_ok=1
    grep -qx 'outcome: "refused"' "${unknown_stem}.out" || unknown_ok=1
    [[ "$(grep -c '^    - check:' "${unknown_stem}.out")" == 1 ]] || unknown_ok=1
    [[ "$(item_field "${unknown_stem}.out" refusals check)" == provider ]] || unknown_ok=1
    [[ "$(item_field "${unknown_stem}.out" refusals path)" == "${crepo}/.kbase/providers.toml" ]] || unknown_ok=1
    [[ "$(item_field "${unknown_stem}.out" refusals key)" == --provider ]] || unknown_ok=1
    [[ "$(item_field "${unknown_stem}.out" refusals remedy)" == "kbase configure --create" ]] || unknown_ok=1
    grep -q 'baseUrl' "${unknown_stem}.out" || unknown_ok=1
    [[ "$(cksum "${crepo}/.kbase/providers.toml" "${crepo}/.kbase/config.toml")" == "${pool_before}" ]] || unknown_ok=1
    report "${unknown_stem}" "${unknown_ok}" "configure --create --project --provider nosuch" "refused against the absolute providers.toml, nothing written"
    # 13. --config-dir and --project name different directories, so together
    #     they are refused as usage, naming --project, and nothing is created.
    conflict="${cdir}/conflict"
    conflict_stem="${docs}/20-configure-conflict"
    set +e
    ( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project \
        --config-dir "${conflict}" ) > "${conflict_stem}.out" 2> "${conflict_stem}.log"
    printf '%s\n' "$?" > "${conflict_stem}.exit"
    set -e
    conflict_ok=0
    [[ "$(cat "${conflict_stem}.exit")" -eq 1 ]] || conflict_ok=1
    grep -qx 'outcome: "refused"' "${conflict_stem}.out" || conflict_ok=1
    [[ "$(item_field "${conflict_stem}.out" refusals check)" == usage ]] || conflict_ok=1
    [[ "$(item_field "${conflict_stem}.out" refusals key)" == --project ]] || conflict_ok=1
    [[ ! -e "${conflict}" ]] || conflict_ok=1
    report "${conflict_stem}" "${conflict_ok}" "configure --create --project --config-dir" "refused naming --project, no directory created"
    # 14. --config-dir into an empty directory creates both files there: the
    #     directory read is the directory written.
    flagdir="${cdir}/flagdir"
    mkdir -p "${flagdir}"
    flag_stem="${docs}/21-configure-config-dir"
    set +e
    ( cd "${work}" && env "${scrub[@]}" "${kbase}" configure --create --config-dir "${flagdir}" ) > "${flag_stem}.out" 2> "${flag_stem}.log"
    printf '%s\n' "$?" > "${flag_stem}.exit"
    set -e
    flag_ok=0
    [[ "$(cat "${flag_stem}.exit")" -eq 0 ]] || flag_ok=1
    grep -qx 'outcome: "done"' "${flag_stem}.out" || flag_ok=1
    grep -qx "    - \"${flagdir}/providers.toml\"" "${flag_stem}.out" || flag_ok=1
    grep -qx "    - \"${flagdir}/config.toml\"" "${flag_stem}.out" || flag_ok=1
    [[ -f "${flagdir}/providers.toml" && -f "${flagdir}/config.toml" ]] || flag_ok=1
    report "${flag_stem}" "${flag_ok}" "configure --create --config-dir into an empty directory" "both files created there"
    # 15. --project outside a worktree refuses, naming the start directory and
    #     the git init the tool does not run, and creates nothing. The run
    #     directory is a mktemp tree, somewhere under no repository at all — the
    #     project's own transient directory sits in one.
    outside="$(mktemp -d)"
    outside_stem="${docs}/22-configure-outside-worktree"
    set +e
    ( cd "${outside}" && env "${scrub[@]}" "${kbase}" configure --create --project ) > "${outside_stem}.out" 2> "${outside_stem}.log"
    printf '%s\n' "$?" > "${outside_stem}.exit"
    set -e
    outside_ok=0
    [[ "$(cat "${outside_stem}.exit")" -eq 1 ]] || outside_ok=1
    grep -qx 'outcome: "refused"' "${outside_stem}.out" || outside_ok=1
    [[ "$(item_field "${outside_stem}.out" refusals check)" == worktree ]] || outside_ok=1
    [[ "$(item_field "${outside_stem}.out" refusals path)" == "${outside}" ]] || outside_ok=1
    [[ "$(item_field "${outside_stem}.out" refusals remedy)" == "git init" ]] || outside_ok=1
    [[ ! -e "${outside}/.kbase" ]] || outside_ok=1
    rm -rf "${outside}"
    report "${outside_stem}" "${outside_ok}" "configure --create --project outside a worktree" "refused on worktree, nothing created"
    # 16. A configure that writes references this package's file mode resolves,
    #     and its edit contract over an old-grammar file: a config.toml naming
    #     bare ids beside a superseded provider key is rewritten in one call —
    #     the three references written, the provider line removed, every other
    #     byte identical — and show-config over the result reports every role
    #     resolved with no diagnostic. A file mode refuses a bare id, so this is
    #     what says the writer and the resolver agree.
    refsdir="${cdir}/refsdir"
    mkdir -p "${refsdir}"
    printf '%s\n' '# kept note' 'provider = "local"' '' '[models]' 'decision = "d3"   # the letter asks' 'text = "qwen"' \
        '' '[asks]' 'readerConcurrency = 2' > "${refsdir}/config.toml"
    printf '%s\n' '# kept note' '' '[models]' 'decision = "local/d3"  # the letter asks' 'text = "local/qwen"' \
        'scoring = "local/clef"' '' '[asks]' 'readerConcurrency = 2' > "${out}/refs-expected.toml"
    refs_stem="${docs}/23-configure-references"
    set +e
    ( cd "${work}" && env "${scrub[@]}" "${kbase}" configure --create --config-dir "${refsdir}" \
        --model-map decision=local/d3,text=local/qwen,scoring=local/clef ) > "${refs_stem}.out" 2> "${refs_stem}.log"
    printf '%s\n' "$?" > "${refs_stem}.exit"
    set -e
    refs_ok=0
    [[ "$(cat "${refs_stem}.exit")" -eq 0 ]] || refs_ok=1
    grep -qx 'outcome: "done"' "${refs_stem}.out" || refs_ok=1
    grep -qxF '    decision: "local/d3"' "${refs_stem}.out" || refs_ok=1
    grep -qxF '    text: "local/qwen"' "${refs_stem}.out" || refs_ok=1
    grep -qxF '    scoring: "local/clef"' "${refs_stem}.out" || refs_ok=1
    cmp -s "${refsdir}/config.toml" "${out}/refs-expected.toml" || refs_ok=1
    report "${refs_stem}" "${refs_ok}" "configure --create rewriting an old-grammar file" "three references, provider gone, every other byte identical"
    refs_show="${docs}/24-configure-references-show"
    set +e
    ( cd "${work}" && env "${scrub[@]}" "${kbase}" show-config --config-dir "${refsdir}" ) > "${refs_show}.out" 2> "${refs_show}.log"
    printf '%s\n' "$?" > "${refs_show}.exit"
    set -e
    refs_show_ok=0
    [[ "$(cat "${refs_show}.exit")" -eq 0 ]] || refs_show_ok=1
    [[ "$(diagnostics "${refs_show}")" == 0 ]] || refs_show_ok=1
    for role in decision text scoring; do
        [[ "$(role_field "${refs_show}.out" "${role}" provider)" == local ]] || refs_show_ok=1
        [[ "$(role_field "${refs_show}.out" "${role}" back-end)" == provider ]] || refs_show_ok=1
    done
    [[ "$(role_field "${refs_show}.out" scoring model)" == clef ]] || refs_show_ok=1
    report "${refs_show}" "${refs_show_ok}" "show-config over the references configure wrote" "every role resolved, no diagnostics"
    # The same report over MCP, from a directory holding no KB: the tool binds
    # the verb (I9), and a verb that resolves no KB answers there.
    printf '%s\n' \
        '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"config-recipe","version":"1"}}}' \
        '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
        '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"show-config","arguments":{}}}' \
        > "${out}/mcp.requests.jsonl"
    set +e
    ( cd "${work}" && env -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE \
        HOME="${home}" KBASE_CONFIG_DIR="${dirs}/both" "${kbase}" mcp < "${out}/mcp.requests.jsonl" ) > "${out}/mcp.actual.jsonl" 2> "${out}/mcp.log"
    printf '%s\n' "$?" > "${out}/mcp.exit"
    set -e
    call="$(awk 'index($0, "\"id\":2") { print; exit }' "${out}/mcp.actual.jsonl")"
    mcp_ok=0
    [[ -n "${call}" ]] || mcp_ok=1
    [[ "${call}" == *'"isError":false'* ]] || mcp_ok=1
    [[ "${call}" == *"\"path\":\"${dirs}/both\""* ]] || mcp_ok=1
    [[ "${call}" == *'"source":"environment"'* ]] || mcp_ok=1
    [[ "${call}" == *'"providersExists":true'* ]] || mcp_ok=1
    [[ "${call}" == *'"configExists":true'* ]] || mcp_ok=1
    [[ "$(grep -o '"check":"provider"' <<< "${call}" | wc -l | tr -d ' ')" == 2 ]] || mcp_ok=1
    printf 'tools/call show-config\t-\t%s\t%s\n' "$([[ "${mcp_ok}" -eq 0 ]] && echo pass || echo fail)" "${out}/mcp.actual.jsonl" >> "${out}/config.tsv"
    printf '  %s tools/call show-config over MCP: exit %s\n' "$([[ "${mcp_ok}" -eq 0 ]] && echo pass || echo fail)" "$(cat "${out}/mcp.exit")"
    [[ "${mcp_ok}" -eq 0 ]] || status=1
    {{just_executable()}} _write-evidence "${out}" "test-integration-config" \
      "three staged configurations under configs/: absent (no directory), pool (providers.toml only) and both (the same providers.toml plus config.toml); the pool holds an entry that drops for want of a baseUrl, and one supplying its key from key.txt seeded mode 0644. home/ is the staged home directory every invocation runs under, left empty so a leg naming a directory reads that one layer only\nper invocation NN-<state>-<route>: from work/, a directory holding no KB, env -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE HOME={{CONFIG_OUT_DIR}}/home KBASE_CONFIG_DIR=<dir> ${kbase} show-config (route environment) or ${kbase} show-config --config-dir <dir> (route flag), stdout to docs/NN-<state>-<route>.out\nconfig.tsv: per invocation the arguments, the source wanted, pass or fail, and its document; each checked for outcome done, mode file, exactly one layer naming the directory with its source and whether it and each of its two files stand, its diagnostic count, and that the key-file value appears nowhere on stdout; the both legs are checked further for the superseded provider key said once on stderr, and for the roles record: provider, model, back-end and the layer that supplied the model and the key\none run over both with the flag route sets KBASE_TEXT=fixture-env-text (NN-07-both-flag.out): the two sources are exclusive, so the document reports mode null, no role back-end, and one item per side — the variable and each standing [models] key (models.decision, models.text, models.scoring)\none run with KBASE_API_BASE_URL and the three role variables and no configuration directory (07b-environment-mode.out): checked for mode environment, no diagnostics, and every role naming the environment back end, the environment's base URL and its own variable's id\nlayered/: a staged git repository whose .kbase/providers.toml adds [local] and overrides the global [shared], and whose .kbase/config.toml overrides models.decision; layered/home/.config/kbase holds the global pool and config.toml. docs/08-layered.out is show-config run from layered/repo/nested with HOME=layered/home, checked for the merged pool in name order with each entry's layer, whole-entry replacement (shared's URL from the repository, key none, no key-file), both layers in precedence order with path, source, exists, providersExists and configExists, the superseded provider key said once on stderr, and each role's own provider, model, endpoint and the layer that supplied each field: repository for the decision role's entry and id, global for the text and scoring roles the repository left alone, whose entry names no key\nconfigure/: a staged git repository with an empty Home, and the configuration legs run under it — the five inference variables and KBASE_CONFIG_DIR unset, HOME on configure/home. docs/15-configure-create-project.out is configure --create --project from configure/repo/nested, checked for outcome done, null provider, both repository paths listed written, the unchecked-ids line on stderr and a live [local] in the created providers.toml; docs/16-configure-created-layer.out is show-config over it, checked for two layers with the repository one first, source repository, exists/providersExists/configExists true, selection local by sole-entry, and the three diagnostics a blank [models] table leaves (models.decision, models.text, models.scoring); docs/17-configure-models.out is models --timeout 300ms over the created pool, checked for provider local and no refusal, so the sole entry is what selected; docs/18-configure-unchanged.out is the same configure again, checked for unchanged with an empty written list; docs/18b-configure-existing-file.out and docs/18c-harnesses-leftover.out put back the harnesses.toml an older kbase seeded and check that configure writes nothing over it and show-config reports the layer's two files and nothing of that one, a file nothing reads; docs/19-configure-unknown-provider.out is configure --create --project --provider nosuch, checked for one provider refusal naming the absolute configure/repo/.kbase/providers.toml, key --provider, remedy kbase configure --create, and both files unchanged by cksum; docs/20-configure-conflict.out is the same flags plus --config-dir, checked for one usage refusal keyed --project with no directory created; docs/21-configure-config-dir.out is configure --create --config-dir into an empty directory, checked for both files written there; docs/22-configure-outside-worktree.out is configure --create --project from a mktemp directory under no repository (removed after the run), checked for one worktree refusal naming that directory with remedy git init and no .kbase created\ndocs/23-configure-references.out is configure --create over an empty directory with --model-map decision=local/d3,text=local/qwen,scoring=local/clef, checked for three references written and no provider key; docs/24-configure-references-show.out is show-config over that directory, checked for no diagnostics and every role on the reference's entry and model\nmcp.requests.jsonl, mcp.actual.jsonl: a tools/call of show-config with no arguments to ${kbase} mcp from work/, checked for isError false, the both directory's path and source, both files present, and the same diagnostic count"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-config): a config check failed — see config.tsv\n'
        exit 1
    fi
    printf 'integration(test-integration-config) ok\n'

[doc("NEEDS expect (the trust prompt is answered over a pty): hermetic run of tools/scripts/build-dist.sh against stub go, gpg, curl and uname — the release layout it stages, the installer checked byte for byte against its golden, the platform matrix, what the installer verifies before it installs anything (a tampered tarball, another file's signature, a 404 in place of a signature), a second run at one version, the version-string refusal, and what it does about the signing key; scratch under .opencode-temp/test-dist/, log in the transient tree")]
test-dist:
    @mkdir -p "{{TEST_DATA_TRANSIENT_DIR}}/test-dist"
    @tools/scripts/test-build-dist.sh 2>&1 | tee "{{TEST_DATA_TRANSIENT_DIR}}/test-dist/log.txt"

# The omnibus composes the HERMETIC recipes and writes no log of its own: each
# already preserves its full output under its own name, and a second copy of
# the same bytes under a second name is a file that can go stale against the
# one anybody reads.
[doc("run every hermetic integration test (the ones needing pandoc, python3, the adjagent clone or a provider are excluded; run those by name)")]
test-integration: test-integration-results-fixture test-integration-init test-integration-mcp test-integration-config test-dist

# Full suite: run once at checkpoints.
checkpoint: edit-gate test-race build

[doc("remove the host build only (bin/kbase, bin/kbase.exe)")]
clean:
    @echo "cleaning local kbase build"
    @rm -f "{{BIN_DIR}}/kbase" "{{BIN_DIR}}/kbase.exe"

# cross-platform dist builds (apple-silicon mac, x86_64 windows, x86_64 linux),
# staged and signed. staging dir also gets user-facing project docs.
# distro artifacts in {{DIST_DIR}}: kbase-<version>.tgz, kbase-<version>.asc, install-kbase-<version>.sh, install-kbase-<version>.sh.asc
# tarball unpacks to kbase-<version>/.
# Signing key, by fingerprint: 5E19B09B9B4CD771F938734C8418EF9F1BE16E1E
# (Benn Herrera <benn@bennherrera.dev>; tools/scripts/build-dist.sh holds the
# same constant).
# Version comes from the binary itself (internal/version stays the single
# source). Run it on a tree that has passed the checkpoint: a cross-build is
# what users receive, so the gate is a precondition of the release, not a step
# this recipe runs.
[doc("cross-build every target and stage the signed user distro tarball")]
dist:
    @BIN_DIR="{{BIN_DIR}}" DIST_DIR="{{DIST_DIR}}" tools/scripts/build-dist.sh

[doc("remove every build output: bin/, dist/, and build/ entirely")]
nuke:
    @echo "cleaning all kbase builds"
    rm -rf {{BIN_DIR}} {{DIST_DIR}} build

# cover reports aggregate test coverage across all packages. -coverpkg
# instruments every package for every test binary, so the number
# reflects how much of the codebase the WHOLE suite exercises — not
# just each package's own tests (cross-package scenario tests count).
# The final `total:` line is the headline percentage; for a
# line-by-line view run `go tool cover -html=cover.out`. cover.out is
# a .gitignore'd derived artifact. Build-tagged tests are not included.
[doc("aggregate whole-suite coverage; headline total on last line")]
cover:
    go test -coverpkg={{GOPKGS}} -coverprofile=cover.out {{GOPKGS}} --count=1
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

# readme-assets/strength-walk.svg is the rendering of STRENGTH_SCORING_SCHEME.md's
# graph, and readme-assets/strength-walk.dot is its source of record: the file a
# reader reviews and edits. dot lays the graph out itself, so the image is a
# function of the .dot AND the Graphviz version that rendered it — an upgrade
# re-renders different bytes from unchanged source. The SVG 1.1 DOCTYPE's two
# lines are dropped, so the checked-in image names no resource a viewer would
# fetch.
[private]
_strength-walk-svg out:
    #!/usr/bin/env bash
    set -euo pipefail
    command -v dot > /dev/null || { echo "error: no Graphviz dot on PATH — install graphviz, then re-run." >&2; exit 1; }
    dot -Tsvg readme-assets/strength-walk.dot | sed '/^<!DOCTYPE svg/,/svg11\.dtd">$/d' > "{{out}}"

[doc("render readme-assets/strength-walk.svg from readme-assets/strength-walk.dot with Graphviz dot")]
strength-walk:
    @{{just_executable()}} _strength-walk-svg readme-assets/strength-walk.svg

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
