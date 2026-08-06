# kbase task recipes — two-gate discipline:
# `edit-gate` after every change (cheap), `checkpoint` once at checkpoints (full).

BIN_DIR := "bin"
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

# Cheap gate: run after every change.
edit-gate: fmt-check
    go vet ./...

# Full suite: run once at checkpoints.
checkpoint: edit-gate test build

clean:
    @echo "cleaning local kbase build"
    @rm -f {{BIN_DIR}}/kbase

# cross-platform dist builds: apple-silicon mac, x86_64 windows, x86_64 linux
dist: test
    @mkdir -p {{BIN_DIR}}
    GOOS=darwin  GOARCH=arm64 go build -o {{BIN_DIR}}/kbase-darwin-arm64 ./cmd
    GOOS=windows GOARCH=amd64 go build -o {{BIN_DIR}}/kbase-windows-amd64.exe ./cmd
    GOOS=linux   GOARCH=amd64 go build -o {{BIN_DIR}}/kbase-linux-amd64 ./cmd

nuke:
    @echo "cleaning all kbase builds"
    rm -rf {{BIN_DIR}}

# cover reports aggregate test coverage across all packages. -coverpkg
# instruments every package for every test binary, so the number
# reflects how much of the codebase the WHOLE suite exercises — not
# just each package's own tests (cross-package scenario tests count).
# The final `total:` line is the headline percentage; for a
# line-by-line view run `go tool cover -html=cover.out`. cover.out is
# a .gitignore'd derived artifact. Build-tagged tests are not included.
[doc("aggregate whole-suite coverage; headline total on last line")]
cover: build
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

update-dependencies:
    go mod tidy
    go get -u ./...
    go mod tidy
