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
TEST_DATA_DIR  := "test_data"
# owned/version controlled test data
TEST_DATA_FIXTURES_DIR := TEST_DATA_DIR / "fixtures"
# test output, generated test data, cloud-sourced test data
TEST_DATA_TRANSIENT_DIR := TEST_DATA_DIR / "transient"

# Per-test output roots. AGENTS.md's "results are part of the test" rule:
# every test run leaves its log — and, where the gathered stats ARE the
# result, its stats and artifacts — somewhere they can be read afterwards.
# One directory per test name, all under the gitignored transient tree.
UNIT_TEST_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "unit_tests"
ROJO_TEST_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-rojo"

DIST_DIR := "dist"

# Pinned integration corpus: the Rojo v7 docs at the commit the survey
# expectations below were validated against. Bumping the pin means
# re-validating the expectations in test-integration.
ROJO_DOCS_REPO := "https://github.com/rojo-rbx/rojo.space"
ROJO_DOCS_COMMIT := "8dbffe025b928e2ff62969b386813a88b5eb593f"
ROJO_DOCS_DIR := TEST_DATA_TRANSIENT_DIR / "rojo.space"

# dist-only build trim: -trimpath drops local filesystem paths from the
# binary; -s -w strips symbol table + DWARF (panic traces keep function
# names). Local `just build` stays fat for debugging.
RELEASE_FLAGS := "-trimpath -ldflags=-s\\ -w"
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

# Cheap gate: run after every change.
edit-gate: fmt-check
    go vet ./...

# Integration tests are per-corpus pairs — prep-test-integration-<corpus>
# (pinned shallow fetch, no-op when present) + test-integration-<corpus>
# (expectations beside their runner) — composed by the omnibus
# `test-integration`. Each test pulls only its own data, so validating one
# corpus never fetches another. Adding a corpus = one pin block above, one
# pair here, one name on the omnibus.

[doc("fetch the pinned Rojo docs corpus (no-op when present)")]
prep-test-integration-rojo:
    @if [[ -f "{{ROJO_DOCS_DIR}}/.git/HEAD" ]]; then \
      echo "rojo docs present: {{ROJO_DOCS_DIR}}"; \
    else \
      mkdir -p "{{ROJO_DOCS_DIR}}" && \
      git -C "{{ROJO_DOCS_DIR}}" init -q && \
      git -C "{{ROJO_DOCS_DIR}}" fetch -q --depth 1 "{{ROJO_DOCS_REPO}}" "{{ROJO_DOCS_COMMIT}}" && \
      git -C "{{ROJO_DOCS_DIR}}" checkout -q FETCH_HEAD && \
      echo "rojo docs cloned at {{ROJO_DOCS_COMMIT}}"; \
    fi

# Survey the pinned Rojo corpus end-to-end. Asserts the summary matches the
# values validated at the pinned commit, and that the artifact is
# byte-deterministic across runs. Expectation changes are loud by design: a
# constants change (e.g. chars-per-token) or a pin bump re-validates here.
#
# It also runs the corpus properties — the dissection one over every section,
# and the tree-plan one over a whole composed tree. Those tests skip when the
# corpus is absent — correct for a unit test, which must not reach the network
# — so `just test` on a clean machine gives no signal on the half of
# ARCHITECTURE §5.1's claim that says "every section of the pinned corpus".
# Here the corpus is guaranteed present, which makes this the gate where the
# claim is true.
[doc("survey the pinned Rojo corpus; assert summary values + determinism")]
test-integration-rojo:
    @mkdir -p "{{ROJO_TEST_OUT_DIR}}"
    @{{just_executable()}} _test-integration-rojo 2>&1 | tee "{{ROJO_TEST_OUT_DIR}}/log.txt"

# The body, split out only so the wrapper above can tee ONE stream. just runs
# each recipe line in its own shell, so a per-line redirect would truncate the
# log four times over; a wrapper around the whole recipe — dependencies
# included — is the one invocation that captures the build, the fetch, both go
# test binaries and every line the surveyed binary writes.
#
# The two go test runs are -v here and nowhere else: `just test` wants a
# pass/fail list, but a preserved integration log wants the t.Logf lines that
# say what the corpus actually measured. The numbers those lines summarise are
# written as JSON beside this log by the tests themselves.
[private]
_test-integration-rojo: build prep-test-integration-rojo
    go test -v -run TestSplitOverRealCorpusSections -count=1 ./internal/dissect
    go test -v -run TestTreePlanOverRealCorpus -count=1 ./internal/treeplan
    @a="{{ROJO_TEST_OUT_DIR}}/survey.json"; \
    b="{{ROJO_TEST_OUT_DIR}}/survey-rerun.json"; \
    s="{{ROJO_TEST_OUT_DIR}}/summary.txt"; \
    ./{{BIN_DIR}}/kbase survey "{{ROJO_DOCS_DIR}}/docs" --json "$a" && \
    ./{{BIN_DIR}}/kbase survey "{{ROJO_DOCS_DIR}}/docs" --json "$b" || \
      { echo "integration(rojo): survey run failed"; exit 1; }; \
    cmp -s "$a" "$b" || { echo "integration(rojo): artifact is not deterministic"; exit 1; }; \
    ./{{BIN_DIR}}/kbase survey "{{ROJO_DOCS_DIR}}/docs" > "$s" || \
      { echo "integration(rojo): survey run failed"; exit 1; }; \
    summary="$(cat "$s")"; \
    echo "$summary"; \
    for want in "files=8 " "tokens=10553 " "sections=83 " "unresolved=5 "; do \
      echo "$summary" | grep -qF "$want" || \
        { echo "integration(rojo): expected '$want' in summary:"; echo "$summary"; exit 1; }; \
    done; \
    echo "integration(rojo) ok: surveyed, deterministic, summary matches"

# The omnibus composes the per-corpus recipes and writes no log of its own:
# each of them already preserves its full output under its own name, and a
# second copy of the same bytes under a second name is a file that can go
# stale against the one anybody reads.
[doc("run every per-corpus integration test")]
test-integration: test-integration-rojo

# Full suite: run once at checkpoints.
checkpoint: edit-gate test-race build

[doc("remove the host build only (bin/kbase, bin/kbase.exe)")]
clean:
    @echo "cleaning local kbase build"
    @rm -f "{{BIN_DIR}}/kbase" "{{BIN_DIR}}/kbase.exe"

# cross-platform dist builds (apple-silicon mac, x86_64 windows, x86_64 linux),
# staged with the user-facing README + LICENSE and tarballed as the user
# distro artifact: {{DIST_DIR}}/kbase-dist-<version>.tar.gz, unpacking to
# kbase-<version>/. Version comes from the binary itself (internal/version
# stays the single source). It runs the full checkpoint first: a cross-build
# is what users receive, so it ships only from a tree that passes the gate
# every commit passes.
[doc("checkpoint, cross-build every target, stage the user distro tarball")]
dist: checkpoint
    @mkdir -p {{BIN_DIR}}
    GOOS=darwin  GOARCH=arm64 go build {{RELEASE_FLAGS}} -o {{BIN_DIR}}/kbase-darwin-arm64 ./cmd
    GOOS=windows GOARCH=amd64 go build {{RELEASE_FLAGS}} -o {{BIN_DIR}}/kbase-windows-amd64.exe ./cmd
    GOOS=linux   GOARCH=amd64 go build {{RELEASE_FLAGS}} -o {{BIN_DIR}}/kbase-linux-amd64 ./cmd
    @ver="$(go run ./cmd --version)"; \
    stage="{{DIST_DIR}}/kbase-$ver"; \
    rm -rf "$stage" && mkdir -p "$stage" && \
    cp {{BIN_DIR}}/kbase-* "$stage/" && \
    cp USER_README.md "$stage/README.md" && \
    cp USER_AGENTS.md "$stage/AGENTS.md" && \
    cp LICENSE "$stage/" && \
    tar -czf "{{DIST_DIR}}/kbase-dist-$ver.tar.gz" -C {{DIST_DIR}} "kbase-$ver" && \
    echo "dist: {{DIST_DIR}}/kbase-dist-$ver.tar.gz"

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

# add-dependency pins ONE vetted module into go.mod/go.sum. It exists so
# adding a dependency has a recipe like every other toolchain operation —
# `update-dependencies` is the wrong tool (its `go get -u ./...` upgrades
# the whole graph, which is unrelated churn on a commit whose subject is
# one new import).
#
#   just add-dependency example.com/mod/v2@v2.1.2
#
# The AGENTS.md vetting checklist (release date, importers, deprecation
# status, transitive dep count, license) is a PRECONDITION of running this,
# not something it can check.
[doc("pin ONE vetted module: just add-dependency <module>@<version>")]
add-dependency mod:
    go get {{mod}}
    go mod tidy

[doc("upgrade the WHOLE module graph; use add-dependency for one module")]
update-dependencies:
    go mod tidy
    go get -u ./...
    go mod tidy
