# kbase task recipes — two-gate discipline:
# `edit-gate` after every change (cheap), `checkpoint` once at checkpoints (full).

# Recipe bodies are bash, not just's default `sh`. Stating it makes the
# contract explicit rather than inherited from whatever /bin/sh happens to be
# on the host — bashisms like fmt-check's `[[ ]]` are then legal everywhere,
# not portable by accident. `-u` (just's own default, kept) makes an unset
# variable a failure instead of an empty string.
set shell := ["bash", "-cu"]

BIN_DIR := "bin"
DIST_DIR := "dist"

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

# run the unit tests. VERBOSE=1 for per-test output.
test:
    go test ${VERBOSE:+-v} ./...

# run the unit tests under the race detector (slower; catches real data races)
test-race:
    go test -race ${VERBOSE:+-v} ./...

# Cheap gate: run after every change.
edit-gate: fmt-check
    go vet ./...

# Full suite: run once at checkpoints.
checkpoint: edit-gate test-race build

[doc("remove the host build only (bin/kbase)")]
clean:
    @echo "cleaning local kbase build"
    @rm -f {{BIN_DIR}}/kbase

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

[doc("remove every build output: bin/ and dist/ entirely")]
nuke:
    @echo "cleaning all kbase builds"
    rm -rf {{BIN_DIR}} {{DIST_DIR}}

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
