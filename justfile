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

# Per-test output roots. CONVENTIONS.md's "results are part of the test" rule:
# every test run leaves its log — and, where the gathered stats ARE the
# result, its stats and artifacts — somewhere they can be read afterwards.
# One directory per test name, all under the gitignored transient tree.
UNIT_TEST_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "unit_tests"
SURVEY_ROJO_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-survey-rojo"
BUILD_MECHANICAL_ROJO_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-mechanical-rojo"
BUILD_LIVE_ROJO_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-live-rojo"
SURVEY_OMLX_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-survey-omlx"
BUILD_MECHANICAL_OMLX_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-mechanical-omlx"
BUILD_LIVE_OMLX_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-live-omlx"
WRITE_AGENTS_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-write-agents"
BUILD_REFUSAL_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-refusal"
BUILD_LIVE_TEMPORAL_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-live-temporal"

# The configuration the live build dials: the checked-in fixture pool, whose
# provider entry names a key FILE the built binary reads for itself. No recipe
# ever reads a key.
LIVE_CONFIG_DIR := TEST_DATA_FIXTURES_DIR / "config"

# The configuration the HERMETIC builds run under. `kbase build` is a
# model-driven appliance, so every run reads a configuration directory; this
# one carries the two [dev] switches that make an offline run possible —
# tree_plan = "mechanical" (no provider dialed at all) and a pinned build_date
# (so two runs produce a byte-identical tree, which a clock would break once a
# day). Both live in the fixture rather than on the command line: they are
# development switches, and `kbase build`'s flag set is the user's.
MECHANICAL_CONFIG_DIR := TEST_DATA_FIXTURES_DIR / "config-mechanical"

DIST_DIR := "dist"

# Pinned integration corpus: the Rojo v7 docs at the commit the survey
# expectations below were validated against. Bumping the pin means
# re-validating the expectations in test-integration.
ROJO_DOCS_REPO := "https://github.com/rojo-rbx/rojo.space"
ROJO_DOCS_COMMIT := "8dbffe025b928e2ff62969b386813a88b5eb593f"
ROJO_DOCS_DIR := TEST_DATA_TRANSIENT_DIR / "rojo.space"

# Pinned integration corpus: the oMLX docs at the commit the survey
# expectations below were validated against. Same bumping rule as Rojo's.
#
# jundot/omlx is a monorepo; docs/ is one subdirectory of an ML runtime. prep
# clones the WHOLE repo shallow, same pattern as Rojo's: simplicity over the
# ~150M of one-time transient disk a sparse fetch would have saved (ruled
# 2026-08-18). The corpus is {{OMLX_DOCS_DIR}}/docs, not {{OMLX_DOCS_DIR}}.
OMLX_DOCS_REPO := "https://github.com/jundot/omlx"
OMLX_DOCS_COMMIT := "aef5a0cf5a1c119ea55bd82f3476fc677230af99"
OMLX_DOCS_DIR := TEST_DATA_TRANSIENT_DIR / "omlx"

# Pinned integration corpus: the MVP shakedown corpus, temporal.io's
# documentation — hundreds of real docs, the first big-corpus live run.
# Ruled by Benn (2026-08-18): pin the shallow clone to this commit, closing
# the gap the prior no-pin comment here left. Same bumping rule as Rojo's.
#
# The repo holds far more than docs/ (a whole documentation site's source),
# but prep-test-integration-temporal clones it WHOLE and shallow, same
# pattern as Rojo's: simplicity over sparse machinery (ruled 2026-08-18) —
# the build only ever reads {{TEMPORAL_DOCS_DIR}}/docs.
TEMPORAL_DOCS_REPO := "https://github.com/temporalio/documentation.git"
TEMPORAL_DOCS_COMMIT := "133948056cc38ab06db42a685388bd96792b5e8a"
TEMPORAL_DOCS_DIR := TEST_DATA_TRANSIENT_DIR / "temporal-io-documentation"

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
# test_data/transient/<test-name>/ overwrites an EVIDENCE.md there: which
# recipe produced the directory, its exact invocation, and one line per
# artifact kind actually present. "Results are part of the test" (CONVENTIONS.md)
# extends to the directory itself — a folder of logs that does not say what
# produced it is not evidence.
#
# One shared writer, not five copies: every integration recipe draws
# artifacts from the same small vocabulary (a log, a survey pair, or a
# delivered kb/ tree), so a fixed checklist covers all five. It is called as
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
      [[ -f "{{out_dir}}/survey.json" ]]  && echo "- survey.json, survey-rerun.json: survey artifact, first run + determinism rerun"; \
      [[ -f "{{out_dir}}/summary.txt" ]]  && echo "- summary.txt: human-readable survey summary"; \
      [[ -d "{{out_dir}}/kb" ]]                    && echo "- kb/: delivered knowledge base tree"; \
      [[ -d "{{out_dir}}/kb/temp-work" ]]           && echo "- kb/temp-work/: kept pipeline intermediates (--keep-temp-work)"; \
      [[ -f "{{out_dir}}/kb/temp-work/run.json" ]]  && echo "- kb/temp-work/run.json: run record (stats, ten verify gates)"; \
      [[ -d "{{out_dir}}/out" ]]                    && echo "- out/: write-agents output (left as the partial-conflict proof left it: docent.md deleted, the other two present)"; \
      true; \
    } > "{{out_dir}}/EVIDENCE.md"

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
test-integration-survey-rojo:
    @mkdir -p "{{SURVEY_ROJO_OUT_DIR}}"
    @{{just_executable()}} _test-integration-survey-rojo 2>&1 | tee "{{SURVEY_ROJO_OUT_DIR}}/log.txt"

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
_test-integration-survey-rojo: build prep-test-integration-rojo
    go test -v -run 'TestSplitOverRealCorpusSections/rojo' -count=1 ./internal/dissect
    go test -v -run 'TestTreePlanOverRealCorpus/rojo' -count=1 ./internal/treeplan
    go test -v -run 'TestLinkResolutionOverRealCorpus/rojo' -count=1 ./internal/survey/markdown
    @a="{{SURVEY_ROJO_OUT_DIR}}/survey.json"; \
    b="{{SURVEY_ROJO_OUT_DIR}}/survey-rerun.json"; \
    s="{{SURVEY_ROJO_OUT_DIR}}/summary.txt"; \
    ./{{BIN_DIR}}/kbase survey "{{ROJO_DOCS_DIR}}/docs" --json "$a" && \
    ./{{BIN_DIR}}/kbase survey "{{ROJO_DOCS_DIR}}/docs" --json "$b" || \
      { echo "integration(survey-rojo): survey run failed"; exit 1; }; \
    cmp -s "$a" "$b" || { echo "integration(survey-rojo): artifact is not deterministic"; exit 1; }; \
    ./{{BIN_DIR}}/kbase survey "{{ROJO_DOCS_DIR}}/docs" > "$s" || \
      { echo "integration(survey-rojo): survey run failed"; exit 1; }; \
    summary="$(cat "$s")"; \
    echo "$summary"; \
    for want in "files=11 " "tokens=12855 " "sections=93 " "internal=6 " "unresolved=9 "; do \
      echo "$summary" | grep -qF "$want" || \
        { echo "integration(survey-rojo): expected '$want' in summary:"; echo "$summary"; exit 1; }; \
    done; \
    echo "integration(survey-rojo) ok: surveyed, deterministic, summary matches"; \
    {{just_executable()}} _write-evidence "{{SURVEY_ROJO_OUT_DIR}}" "test-integration-survey-rojo" \
      "./{{BIN_DIR}}/kbase survey {{ROJO_DOCS_DIR}}/docs --json {{SURVEY_ROJO_OUT_DIR}}/survey.json\n./{{BIN_DIR}}/kbase survey {{ROJO_DOCS_DIR}}/docs --json {{SURVEY_ROJO_OUT_DIR}}/survey-rerun.json  # determinism rerun\n./{{BIN_DIR}}/kbase survey {{ROJO_DOCS_DIR}}/docs > {{SURVEY_ROJO_OUT_DIR}}/summary.txt"

# Build a whole knowledge base out of the pinned Rojo corpus and walk it.
#
# This is the mechanical spine end to end — survey, tree plan, cuts, pages,
# assembly, the ten verify gates, delivery — with no model in the loop, so it
# runs offline in a second and any drift in the deterministic half of the
# pipeline fails it.
#
# The KB *is* the evidence, so unlike the other recipes this one keeps its
# whole output: the delivered tree under {{BUILD_MECHANICAL_ROJO_OUT_DIR}}/kb,
# the run record beside it, the job's intermediates in its temp-work/, the
# measured stats in stats.json, and the full log. A build that left nothing to
# walk would be a green tick nobody can check.
[doc("build a KB from the pinned Rojo corpus with no model in the loop; assert the ten verify gates and walk the tree")]
test-integration-build-mechanical-rojo:
    @mkdir -p "{{BUILD_MECHANICAL_ROJO_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-mechanical-rojo 2>&1 | tee "{{BUILD_MECHANICAL_ROJO_OUT_DIR}}/log.txt"

# The body, split out so the wrapper above can tee ONE stream — same reason as
# _test-integration-survey-rojo.
#
# Front-door per CONVENTIONS.md's "integration tests invoke the app binary" ruling:
# the binary is the thing under test, invoked exactly as a user would, with
# every behavior (--out, --config-dir, --keep-temp-work) a flag in this
# invocation rather than a Go test's struct literal. `build` is the delivered
# verb, not a development one: there is no second code path here for tests to
# exercise instead.
#
# The pinned counts below are read straight off run.json — the same record
# test-integration-build-live-rojo asserts the ten gates from — rather than
# recomputed against each other in process, because a shell recipe never has
# the tree-plan struct to recompute them from. That is not a loss: gate 5
# ("the delivered set is the tree plan's nodes plus the fixture manifest") and
# gate 3 ("the entry-point exists and its domains exist") already prove the
# relations the old in-process test derived by hand, and 34 nodes + 3 fixture
# manifest entries = 37 delivered is the same arithmetic, just pre-computed
# into a literal the way test-integration-survey-rojo pins tokens/sections.
# Bumping the corpus pin or a budget/manifest constant re-derives these with
# `./bin/kbase build ... --keep-temp-work` and updates them here.
#
# Gate 7 ("leaf fidelity: every page re-derives byte-for-byte from source")
# proves EVERY leaf on every run, inside the job — stronger than the deleted
# Go test's single largest-page spot check outside it. Do not re-add a spot
# check here; it would be strictly redundant with a gate this recipe already
# asserts green.
[private]
_test-integration-build-mechanical-rojo: build prep-test-integration-rojo
    @kb="{{BUILD_MECHANICAL_ROJO_OUT_DIR}}/kb"; \
    rm -rf "$kb"; \
    ./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" \
      --config-dir "{{MECHANICAL_CONFIG_DIR}}" \
      --out "$kb" \
      --keep-temp-work || \
      { echo "integration(build-mechanical-rojo): the build failed"; exit 1; }; \
    rec="$kb/temp-work/run.json"; \
    [[ -f "$rec" ]] || { echo "integration(build-mechanical-rojo): no run record at $rec"; exit 1; }; \
    green="$(grep -c '"ok": true' "$rec" || true)"; \
    red="$(grep -c '"ok": false' "$rec" || true)"; \
    [[ "$red" == 0 ]] || { echo "integration(build-mechanical-rojo): $red of the ten gates refused; see $rec"; exit 1; }; \
    [[ "$green" == 10 ]] || { echo "integration(build-mechanical-rojo): $green gates reported, want ten; see $rec"; exit 1; }; \
    for want in '"sourceFiles": 11' '"sourceSections": 93' '"nodes": 34' '"pages": 22' '"sections": 12' '"groups": 22' '"splitGroups": 0' '"deliveredFiles": 37'; do \
      grep -qF "$want" "$rec" || { echo "integration(build-mechanical-rojo): expected $want in $rec"; exit 1; }; \
    done; \
    for want in entry-point.md CONVENTIONS.md README.md CLAUDE.md; do \
      [[ -f "$kb/$want" ]] || { echo "integration(build-mechanical-rojo): $want was not delivered"; exit 1; }; \
    done; \
    kb2="{{BUILD_MECHANICAL_ROJO_OUT_DIR}}/kb-rerun"; \
    rm -rf "$kb2"; \
    ./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" \
      --config-dir "{{MECHANICAL_CONFIG_DIR}}" \
      --out "$kb2" || \
      { echo "integration(build-mechanical-rojo): the determinism rerun failed"; exit 1; }; \
    diff -rq -x temp-work "$kb" "$kb2" || \
      { echo "integration(build-mechanical-rojo): delivered tree is not byte-deterministic across runs"; exit 1; }; \
    rm -rf "$kb2"; \
    echo "integration(build-mechanical-rojo): delivered tree at $kb"; \
    find "$kb" -name '*.md' -not -path '*/temp-work/*' | wc -l | xargs echo "  markdown pages:"; \
    echo "  tree to depth 2:"; \
    find "$kb" -maxdepth 2 -not -path '*/temp-work*' | sort | sed "s|$kb|  .|"; \
    echo "integration(build-mechanical-rojo) ok: ten gates green, counts pinned, tree deterministic and delivered"; \
    {{just_executable()}} _write-evidence "{{BUILD_MECHANICAL_ROJO_OUT_DIR}}" "test-integration-build-mechanical-rojo" \
      "./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_MECHANICAL_ROJO_OUT_DIR}}/kb --keep-temp-work\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_MECHANICAL_ROJO_OUT_DIR}}/kb-rerun  # determinism rerun, diffed then discarded"

# The same build with the model IN the loop: stages 3, 4 and 6 — the container
# descent that designs the tree, the boundary fold that adjudicates page cuts,
# and the four level stages that summarise it — run against the provider
# configured in {{LIVE_CONFIG_DIR}}. Everything else is the same code on the
# same artifacts as the mechanical build, which is what makes the pair readable
# side by side.
#
# Stage 4 makes ZERO calls over this corpus, and that is the correct result
# rather than a stage that failed to run: no Rojo group exceeds the leaf budget
# (the mechanical recipe pins "splitGroups": 0), so there is no boundary for a
# model to choose. The recipe asserts that explicitly below, so the day a pin
# bump gives Rojo a split group the number changes loudly instead of silently
# starting to spend light-tier calls.
#
# It is NOT composed into `test-integration`, deliberately: that omnibus is
# hermetic and offline, and this recipe needs a reachable provider, a key file
# and minutes of model time. Run it by name when the live seams are what you
# are checking.
#
# The recipe invokes the built binary and nothing else — the API key stays a
# path in providers.toml that the binary reads for itself, and no recipe, log
# or evidence file ever holds it.
[doc("LIVE (excluded from test-integration: needs a provider + network): build a KB from the pinned Rojo corpus with the model in the loop; assert the ten verify gates")]
test-integration-build-live-rojo:
    @mkdir -p "{{BUILD_LIVE_ROJO_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-live-rojo 2>&1 | tee "{{BUILD_LIVE_ROJO_OUT_DIR}}/log.txt"

# The body, split out so the wrapper above can tee ONE stream — same reason as
# _test-integration-survey-rojo.
#
# The gates are read from run.json rather than from the summary: the record is
# the artifact that outlives the terminal, so asserting over it is asserting
# over the evidence that gets kept.
[private]
_test-integration-build-live-rojo: build prep-test-integration-rojo
    @kb="{{BUILD_LIVE_ROJO_OUT_DIR}}/kb"; \
    rm -rf "$kb"; \
    ./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" \
      --config-dir "{{LIVE_CONFIG_DIR}}" \
      --out "$kb" \
      --keep-temp-work || \
      { echo "integration(build-live-rojo): the live build failed"; exit 1; }; \
    rec="$kb/temp-work/run.json"; \
    [[ -f "$rec" ]] || { echo "integration(build-live-rojo): no run record at $rec"; exit 1; }; \
    green="$(grep -c '"ok": true' "$rec" || true)"; \
    red="$(grep -c '"ok": false' "$rec" || true)"; \
    [[ "$red" == 0 ]] || { echo "integration(build-live-rojo): $red of the ten gates refused; see $rec"; exit 1; }; \
    [[ "$green" == 10 ]] || { echo "integration(build-live-rojo): $green gates reported, want ten; see $rec"; exit 1; }; \
    for want in entry-point.md CONVENTIONS.md README.md CLAUDE.md; do \
      [[ -f "$kb/$want" ]] || { echo "integration(build-live-rojo): $want was not delivered"; exit 1; }; \
    done; \
    grep -q '"lightModel": "..*"' "$rec" || \
      { echo "integration(build-live-rojo): the run record names no light-tier model; see $rec"; exit 1; }; \
    for want in '"splitGroups": 0' '"boundariesAdjudicated": 0' '"boundariesFellBack": 0'; do \
      grep -qF "$want" "$rec" || \
        { echo "integration(build-live-rojo): expected $want in $rec — Rojo has no oversized group, so stage 4 has nothing to adjudicate"; exit 1; }; \
    done; \
    echo "integration(build-live-rojo): stage 4 adjudicated 0 boundaries, which is correct for this corpus"; \
    echo "integration(build-live-rojo): delivered tree at $kb"; \
    find "$kb" -name '*.md' -not -path '*/temp-work/*' | wc -l | xargs echo "  markdown pages:"; \
    echo "  tree to depth 2:"; \
    find "$kb" -maxdepth 2 -not -path '*/temp-work*' | sort | sed "s|$kb|  .|"; \
    echo "integration(build-live-rojo) ok: ten gates green, tree delivered, temp work kept"; \
    {{just_executable()}} _write-evidence "{{BUILD_LIVE_ROJO_OUT_DIR}}" "test-integration-build-live-rojo" \
      "./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{LIVE_CONFIG_DIR}} --out {{BUILD_LIVE_ROJO_OUT_DIR}}/kb --keep-temp-work"

# The oMLX corpus, the same two hermetic processes over it plus one LIVE
# build (test-integration-build-live-omlx, below the mechanical pair) — a
# second live seam check over a corpus of a different shape than Rojo's,
# where the model (not the corpus) decides how many split groups and
# boundaries there are.
#
# The fetch is the plain shallow clone Rojo's prep does — see the pin block
# for why (ruled 2026-08-18: simplicity over sparse machinery). The
# repository's own top-level files come along; they are harmless, since
# ingest reads only the .md under the root it is given, and that root is
# {{OMLX_DOCS_DIR}}/docs.
[doc("fetch the pinned oMLX docs corpus (no-op when present)")]
prep-test-integration-omlx:
    @if [[ -f "{{OMLX_DOCS_DIR}}/.git/HEAD" ]]; then \
      echo "omlx docs present: {{OMLX_DOCS_DIR}}"; \
    else \
      mkdir -p "{{OMLX_DOCS_DIR}}" && \
      git -C "{{OMLX_DOCS_DIR}}" init -q && \
      git -C "{{OMLX_DOCS_DIR}}" fetch -q --depth 1 "{{OMLX_DOCS_REPO}}" "{{OMLX_DOCS_COMMIT}}" && \
      git -C "{{OMLX_DOCS_DIR}}" checkout -q FETCH_HEAD && \
      echo "omlx docs cloned at {{OMLX_DOCS_COMMIT}}"; \
    fi

# Survey the pinned oMLX corpus end-to-end — the same two claims
# test-integration-survey-rojo makes, over a corpus with a different shape:
# fewer, much larger documents. Expectation changes are loud by design here
# too.
#
# It also runs the dissect and treeplan corpus properties, same as
# test-integration-survey-rojo — both are now parameterized per-corpus
# subtests, so this recipe runs the /omlx subtest here, qualified exactly as
# the rojo recipe qualifies its own /rojo subtest, over the corpus this
# recipe (not that one) guarantees present.
[doc("survey the pinned oMLX corpus; assert summary values + determinism")]
test-integration-survey-omlx:
    @mkdir -p "{{SURVEY_OMLX_OUT_DIR}}"
    @{{just_executable()}} _test-integration-survey-omlx 2>&1 | tee "{{SURVEY_OMLX_OUT_DIR}}/log.txt"

# The body, split out so the wrapper above can tee ONE stream — same reason as
# _test-integration-survey-rojo.
#
# The two go test runs are -v here and nowhere else, same reason as
# _test-integration-survey-rojo: a preserved integration log wants the
# t.Logf lines that say what the corpus actually measured.
[private]
_test-integration-survey-omlx: build prep-test-integration-omlx
    go test -v -run 'TestSplitOverRealCorpusSections/omlx' -count=1 ./internal/dissect
    go test -v -run 'TestTreePlanOverRealCorpus/omlx' -count=1 ./internal/treeplan
    go test -v -run 'TestLinkResolutionOverRealCorpus/omlx' -count=1 ./internal/survey/markdown
    @a="{{SURVEY_OMLX_OUT_DIR}}/survey.json"; \
    b="{{SURVEY_OMLX_OUT_DIR}}/survey-rerun.json"; \
    s="{{SURVEY_OMLX_OUT_DIR}}/summary.txt"; \
    ./{{BIN_DIR}}/kbase survey "{{OMLX_DOCS_DIR}}/docs" --json "$a" && \
    ./{{BIN_DIR}}/kbase survey "{{OMLX_DOCS_DIR}}/docs" --json "$b" || \
      { echo "integration(survey-omlx): survey run failed"; exit 1; }; \
    cmp -s "$a" "$b" || { echo "integration(survey-omlx): artifact is not deterministic"; exit 1; }; \
    ./{{BIN_DIR}}/kbase survey "{{OMLX_DOCS_DIR}}/docs" > "$s" || \
      { echo "integration(survey-omlx): survey run failed"; exit 1; }; \
    summary="$(cat "$s")"; \
    echo "$summary"; \
    for want in "files=5 " "tokens=18264 " "sections=110 " "unresolved=2 "; do \
      echo "$summary" | grep -qF "$want" || \
        { echo "integration(survey-omlx): expected '$want' in summary:"; echo "$summary"; exit 1; }; \
    done; \
    echo "integration(survey-omlx) ok: surveyed, deterministic, summary matches"; \
    {{just_executable()}} _write-evidence "{{SURVEY_OMLX_OUT_DIR}}" "test-integration-survey-omlx" \
      "./{{BIN_DIR}}/kbase survey {{OMLX_DOCS_DIR}}/docs --json {{SURVEY_OMLX_OUT_DIR}}/survey.json\n./{{BIN_DIR}}/kbase survey {{OMLX_DOCS_DIR}}/docs --json {{SURVEY_OMLX_OUT_DIR}}/survey-rerun.json  # determinism rerun\n./{{BIN_DIR}}/kbase survey {{OMLX_DOCS_DIR}}/docs > {{SURVEY_OMLX_OUT_DIR}}/summary.txt"

# Build a whole knowledge base out of the pinned oMLX corpus and walk it — the
# mechanical spine end to end, exactly as test-integration-build-mechanical-rojo
# does it, over the second corpus. Running the deterministic half over two
# corpora of different shape is what stops it being tuned to one of them.
#
# It keeps its whole output for the same reason that one does: the KB is the
# evidence.
[doc("build a KB from the pinned oMLX corpus with no model in the loop; assert the ten verify gates and walk the tree")]
test-integration-build-mechanical-omlx:
    @mkdir -p "{{BUILD_MECHANICAL_OMLX_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-mechanical-omlx 2>&1 | tee "{{BUILD_MECHANICAL_OMLX_OUT_DIR}}/log.txt"

# The body, split out so the wrapper above can tee ONE stream — same reason as
# _test-integration-survey-rojo. Same front-door shape and same reasoning as
# _test-integration-build-mechanical-rojo — see its comment for why the
# pinned counts are read off run.json and why no leaf spot check is re-added.
[private]
_test-integration-build-mechanical-omlx: build prep-test-integration-omlx
    @kb="{{BUILD_MECHANICAL_OMLX_OUT_DIR}}/kb"; \
    rm -rf "$kb"; \
    ./{{BIN_DIR}}/kbase build "{{OMLX_DOCS_DIR}}/docs" \
      --config-dir "{{MECHANICAL_CONFIG_DIR}}" \
      --out "$kb" \
      --keep-temp-work || \
      { echo "integration(build-mechanical-omlx): the build failed"; exit 1; }; \
    rec="$kb/temp-work/run.json"; \
    [[ -f "$rec" ]] || { echo "integration(build-mechanical-omlx): no run record at $rec"; exit 1; }; \
    green="$(grep -c '"ok": true' "$rec" || true)"; \
    red="$(grep -c '"ok": false' "$rec" || true)"; \
    [[ "$red" == 0 ]] || { echo "integration(build-mechanical-omlx): $red of the ten gates refused; see $rec"; exit 1; }; \
    [[ "$green" == 10 ]] || { echo "integration(build-mechanical-omlx): $green gates reported, want ten; see $rec"; exit 1; }; \
    for want in '"sourceFiles": 5' '"sourceSections": 110' '"nodes": 31' '"pages": 22' '"sections": 9' '"groups": 22' '"splitGroups": 0' '"deliveredFiles": 34'; do \
      grep -qF "$want" "$rec" || { echo "integration(build-mechanical-omlx): expected $want in $rec"; exit 1; }; \
    done; \
    for want in entry-point.md CONVENTIONS.md README.md CLAUDE.md; do \
      [[ -f "$kb/$want" ]] || { echo "integration(build-mechanical-omlx): $want was not delivered"; exit 1; }; \
    done; \
    kb2="{{BUILD_MECHANICAL_OMLX_OUT_DIR}}/kb-rerun"; \
    rm -rf "$kb2"; \
    ./{{BIN_DIR}}/kbase build "{{OMLX_DOCS_DIR}}/docs" \
      --config-dir "{{MECHANICAL_CONFIG_DIR}}" \
      --out "$kb2" || \
      { echo "integration(build-mechanical-omlx): the determinism rerun failed"; exit 1; }; \
    diff -rq -x temp-work "$kb" "$kb2" || \
      { echo "integration(build-mechanical-omlx): delivered tree is not byte-deterministic across runs"; exit 1; }; \
    rm -rf "$kb2"; \
    echo "integration(build-mechanical-omlx): delivered tree at $kb"; \
    find "$kb" -name '*.md' -not -path '*/temp-work/*' | wc -l | xargs echo "  markdown pages:"; \
    echo "  tree to depth 2:"; \
    find "$kb" -maxdepth 2 -not -path '*/temp-work*' | sort | sed "s|$kb|  .|"; \
    echo "integration(build-mechanical-omlx) ok: ten gates green, counts pinned, tree deterministic and delivered"; \
    {{just_executable()}} _write-evidence "{{BUILD_MECHANICAL_OMLX_OUT_DIR}}" "test-integration-build-mechanical-omlx" \
      "./{{BIN_DIR}}/kbase build {{OMLX_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_MECHANICAL_OMLX_OUT_DIR}}/kb --keep-temp-work\n./{{BIN_DIR}}/kbase build {{OMLX_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_MECHANICAL_OMLX_OUT_DIR}}/kb-rerun  # determinism rerun, diffed then discarded"

# The same build with the model IN the loop, over the oMLX corpus — the same
# pairing test-integration-build-live-rojo makes with its own mechanical
# sibling, over a corpus of a different shape. See that recipe's comment for
# the stage-3/4/6-in-the-loop shape and the front-door/no-key-in-recipe
# reasoning, both unchanged here.
#
# It is NOT composed into `test-integration`, deliberately, for the same
# reason test-integration-build-live-rojo is not: that omnibus is hermetic
# and offline, and this recipe needs a reachable provider, a key file and
# minutes of model time.
[doc("LIVE (excluded from test-integration: needs a provider + network): build a KB from the pinned oMLX corpus with the model in the loop; assert the ten verify gates")]
test-integration-build-live-omlx:
    @mkdir -p "{{BUILD_LIVE_OMLX_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-live-omlx 2>&1 | tee "{{BUILD_LIVE_OMLX_OUT_DIR}}/log.txt"

# The body, split out so the wrapper above can tee ONE stream — same reason as
# _test-integration-survey-rojo.
#
# Unlike test-integration-build-live-rojo, this recipe does NOT pin
# splitGroups/boundariesAdjudicated/boundariesFellBack/callsSkipped: Rojo's
# zero split groups is a fact about the CORPUS (no group exceeds the leaf
# budget, mechanically), but oMLX's mechanical sibling already pins
# "splitGroups": 1 — and here stage 3 runs WITH the model, which designs the
# tree itself, so how many groups it produces and how many boundaries stage 4
# ends up adjudicating are model-dependent too, not knowable in advance. Those
# counts are echoed into the log below instead of asserted, so the numbers
# land in the preserved evidence; once enough live runs show them stable they
# can graduate into pins the way the mechanical recipe's already are.
[private]
_test-integration-build-live-omlx: build prep-test-integration-omlx
    @kb="{{BUILD_LIVE_OMLX_OUT_DIR}}/kb"; \
    rm -rf "$kb"; \
    ./{{BIN_DIR}}/kbase build "{{OMLX_DOCS_DIR}}/docs" \
      --config-dir "{{LIVE_CONFIG_DIR}}" \
      --out "$kb" \
      --keep-temp-work || \
      { echo "integration(build-live-omlx): the live build failed"; exit 1; }; \
    rec="$kb/temp-work/run.json"; \
    [[ -f "$rec" ]] || { echo "integration(build-live-omlx): no run record at $rec"; exit 1; }; \
    green="$(grep -c '"ok": true' "$rec" || true)"; \
    red="$(grep -c '"ok": false' "$rec" || true)"; \
    [[ "$red" == 0 ]] || { echo "integration(build-live-omlx): $red of the ten gates refused; see $rec"; exit 1; }; \
    [[ "$green" == 10 ]] || { echo "integration(build-live-omlx): $green gates reported, want ten; see $rec"; exit 1; }; \
    for want in entry-point.md CONVENTIONS.md README.md CLAUDE.md; do \
      [[ -f "$kb/$want" ]] || { echo "integration(build-live-omlx): $want was not delivered"; exit 1; }; \
    done; \
    grep -q '"lightModel": "..*"' "$rec" || \
      { echo "integration(build-live-omlx): the run record names no light-tier model; see $rec"; exit 1; }; \
    echo "integration(build-live-omlx): adjudication stats (corpus- and model-dependent, not pinned):"; \
    grep -E '"splitGroups"|"boundariesAdjudicated"|"boundariesFellBack"|"callsSkipped"' "$rec" | sed 's/^/  /'; \
    echo "integration(build-live-omlx): delivered tree at $kb"; \
    find "$kb" -name '*.md' -not -path '*/temp-work/*' | wc -l | xargs echo "  markdown pages:"; \
    echo "  tree to depth 2:"; \
    find "$kb" -maxdepth 2 -not -path '*/temp-work*' | sort | sed "s|$kb|  .|"; \
    echo "integration(build-live-omlx) ok: ten gates green, tree delivered, temp work kept"; \
    {{just_executable()}} _write-evidence "{{BUILD_LIVE_OMLX_OUT_DIR}}" "test-integration-build-live-omlx" \
      "./{{BIN_DIR}}/kbase build {{OMLX_DOCS_DIR}}/docs --config-dir {{LIVE_CONFIG_DIR}} --out {{BUILD_LIVE_OMLX_OUT_DIR}}/kb --keep-temp-work"

# Same no-op-when-present shape as Rojo's and oMLX's own preps: presence of
# .git/HEAD is the whole check. It does NOT re-verify a present checkout sits
# at {{TEMPORAL_DOCS_COMMIT}}, and does not touch it either way. That is the
# established precedent, not a gap specific to this corpus: a
# present-but-unpinned or present-but-mismatched checkout is trusted and left
# alone here exactly as it would be for Rojo or oMLX — re-validating a corpus
# already on disk is what bumping the pin (and clearing the directory) is
# for, not what prep does on every run.
#
# The fetch is the plain shallow clone Rojo's (and now oMLX's) prep does —
# ruled 2026-08-18: simplicity over sparse machinery. The repo holds far more
# than docs/ (a whole documentation site's source) but the build only ever
# reads {{TEMPORAL_DOCS_DIR}}/docs, so the rest is harmless, unread bulk.
[doc("fetch the pinned temporal.io docs corpus (no-op when present)")]
prep-test-integration-temporal:
    @if [[ -f "{{TEMPORAL_DOCS_DIR}}/.git/HEAD" ]]; then \
      echo "temporal docs present: {{TEMPORAL_DOCS_DIR}}"; \
    else \
      mkdir -p "{{TEMPORAL_DOCS_DIR}}" && \
      git -C "{{TEMPORAL_DOCS_DIR}}" init -q && \
      git -C "{{TEMPORAL_DOCS_DIR}}" fetch -q --depth 1 "{{TEMPORAL_DOCS_REPO}}" "{{TEMPORAL_DOCS_COMMIT}}" && \
      git -C "{{TEMPORAL_DOCS_DIR}}" checkout -q FETCH_HEAD && \
      echo "temporal docs cloned at {{TEMPORAL_DOCS_COMMIT}}"; \
    fi

# The MVP shakedown: temporal.io's documentation, hundreds of real docs —
# the first live run over a corpus this suite did not pick for its shape.
# Unlike the Rojo/oMLX live pair, this is first contact: no gate count, tree
# shape, or call count has been observed yet, so nothing beyond "it ran and
# the ten gates passed" is asserted. Everything else the run measured is
# ECHOED from run.json into the preserved log instead — first-run evidence,
# not a pin — the same restraint test-integration-build-live-omlx already
# takes with its own adjudication stats, extended here to the whole record
# because this corpus has no history to pin against yet. Once repeated runs
# show a number stable, it graduates into a pin the way the mechanical
# recipes' own did.
#
# prep-test-integration-temporal is a dependency, same as the other live
# recipes depend on their own preps — but the corpus-absent check in the body
# below is KEPT as a fallback message, not removed as redundant: prep can be
# skipped (`just _test-integration-build-live-temporal` direct, bypassing the
# dependency chain a stale build tool might take) or itself refuse without
# this recipe ever finding out, and a build that walked into a missing corpus
# with no message of its own would fail on kbase's own generic error instead
# of naming the path this recipe expects.
#
# No timeout is imposed here or asked of the shell: hundreds of documents
# through stages 3/4/6 with a model in the loop is the longest run in this
# suite, and buildTimeout (cmd/build.go) already bounds the job itself.
#
# NOT composed into `test-integration`, for the same reason the other live
# recipes are not: hermetic and offline is what that omnibus means, and this
# needs a reachable provider, a key file, and a long stretch of model time.
[doc("LIVE (excluded from test-integration: needs a provider + network): the MVP shakedown — build a KB from the temporal.io docs with the model in the loop; assert only that it ran and the ten gates passed")]
test-integration-build-live-temporal:
    @mkdir -p "{{BUILD_LIVE_TEMPORAL_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-live-temporal 2>&1 | tee "{{BUILD_LIVE_TEMPORAL_OUT_DIR}}/log.txt"

# The body, split out so the wrapper above can tee ONE stream — same reason as
# _test-integration-survey-rojo.
[private]
_test-integration-build-live-temporal: build prep-test-integration-temporal
    @corpus="{{TEMPORAL_DOCS_DIR}}/docs"; \
    [[ -d "$corpus" ]] || \
      { echo "integration(build-live-temporal): corpus not found at $corpus — this recipe does not fetch it; fetch the temporal.io docs there first"; exit 1; }; \
    kb="{{BUILD_LIVE_TEMPORAL_OUT_DIR}}/kb"; \
    rm -rf "$kb"; \
    ./{{BIN_DIR}}/kbase build "$corpus" \
      --config-dir "{{LIVE_CONFIG_DIR}}" \
      --out "$kb" \
      --title "Temporal Documentation" \
      --keep-temp-work || \
      { echo "integration(build-live-temporal): the live build failed"; exit 1; }; \
    rec="$kb/temp-work/run.json"; \
    [[ -f "$rec" ]] || { echo "integration(build-live-temporal): no run record at $rec"; exit 1; }; \
    green="$(grep -c '"ok": true' "$rec" || true)"; \
    red="$(grep -c '"ok": false' "$rec" || true)"; \
    [[ "$red" == 0 ]] || { echo "integration(build-live-temporal): $red of the ten gates refused; see $rec"; exit 1; }; \
    [[ "$green" == 10 ]] || { echo "integration(build-live-temporal): $green gates reported, want ten; see $rec"; exit 1; }; \
    echo "integration(build-live-temporal): ten gates green"; \
    echo "integration(build-live-temporal): first-run evidence from $rec (echoed, not pinned):"; \
    grep -E '"sourceFiles"|"sourceExcluded"|"nodes"|"pages"|"splitGroups"|"maxDepth"|"elapsedMs"' "$rec" | sed 's/^/  /'; \
    grep -A4 '"links"' "$rec" | sed 's/^/  /'; \
    grep -E '"taxonomyCalls"|"leafGroupCards"|"boundariesAdjudicated"' "$rec" | sed 's/^/  /'; \
    echo "integration(build-live-temporal): delivered tree at $kb"; \
    find "$kb" -name '*.md' -not -path '*/temp-work/*' | wc -l | xargs echo "  markdown pages:"; \
    echo "  tree to depth 2:"; \
    find "$kb" -maxdepth 2 -not -path '*/temp-work*' | sort | sed "s|$kb|  .|"; \
    echo "integration(build-live-temporal) ok: ten gates green, tree delivered, temp work kept"; \
    {{just_executable()}} _write-evidence "{{BUILD_LIVE_TEMPORAL_OUT_DIR}}" "test-integration-build-live-temporal" \
      "./{{BIN_DIR}}/kbase build {{TEMPORAL_DOCS_DIR}}/docs --config-dir {{LIVE_CONFIG_DIR}} --out {{BUILD_LIVE_TEMPORAL_OUT_DIR}}/kb --title \"Temporal Documentation\" --keep-temp-work"

# Drives `kbase write-agents` directly rather than through a build:
# the verb dials nothing and reads nothing (see write_agents.go), so
# it needs no corpus and no config-dir fixture — hermetic in the strongest
# sense, which is why it joins the omnibus below despite not being a
# <process>-<corpus> recipe.
#
# Three claims, in order, over the SAME target directory: a fresh write
# succeeds with all three files present and non-empty; an identical second
# run refuses (exit 1) and names all three conflicts, having written nothing
# new; and deleting one file and rerunning still refuses — proving the
# all-or-nothing contract holds even when only one of three files conflicts,
# not just when all three do. That last case is the one a partial, per-file
# overwrite policy would pass and this one must not.
[doc("hermetic: kbase write-agents fresh write, full-conflict refusal, partial-conflict refusal; joins the omnibus")]
test-integration-write-agents:
    @mkdir -p "{{WRITE_AGENTS_OUT_DIR}}"
    @{{just_executable()}} _test-integration-write-agents 2>&1 | tee "{{WRITE_AGENTS_OUT_DIR}}/log.txt"

# The body, split out so the wrapper above can tee ONE stream — same reason as
# _test-integration-survey-rojo.
[private]
_test-integration-write-agents: build
    @out="{{WRITE_AGENTS_OUT_DIR}}/out"; \
    rm -rf "$out"; \
    ./{{BIN_DIR}}/kbase write-agents --out "$out" || \
      { echo "integration(write-agents): fresh write into an empty directory failed"; exit 1; }; \
    for f in docent.md maintainer.md README-ADAPTATION.md; do \
      [[ -s "$out/$f" ]] || \
        { echo "integration(write-agents): $f missing or empty after the fresh write"; exit 1; }; \
    done; \
    echo "integration(write-agents): fresh write ok — three files present and non-empty"; \
    full_err="$(./{{BIN_DIR}}/kbase write-agents --out "$out" 2>&1)"; \
    full_status=$?; \
    [[ "$full_status" == 1 ]] || \
      { echo "integration(write-agents): second run over the same directory exited $full_status, want 1"; exit 1; }; \
    for f in "$out/docent.md" "$out/maintainer.md" "$out/README-ADAPTATION.md"; do \
      echo "$full_err" | grep -qF "$f" || \
        { echo "integration(write-agents): full-conflict refusal did not name $f:"; echo "$full_err"; exit 1; }; \
    done; \
    echo "integration(write-agents): full-conflict refusal ok — exit 1, all three conflicts named"; \
    rm -f "$out/docent.md"; \
    partial_err="$(./{{BIN_DIR}}/kbase write-agents --out "$out" 2>&1)"; \
    partial_status=$?; \
    [[ "$partial_status" == 1 ]] || \
      { echo "integration(write-agents): partial-conflict run exited $partial_status, want 1"; exit 1; }; \
    [[ ! -e "$out/docent.md" ]] || \
      { echo "integration(write-agents): partial-conflict refusal wrote docent.md — all-or-nothing contract broken"; exit 1; }; \
    echo "integration(write-agents): partial-conflict refusal ok — exit 1, nothing written back"; \
    echo "integration(write-agents) ok: fresh write, full-conflict refusal, and partial-conflict refusal all correct"; \
    {{just_executable()}} _write-evidence "{{WRITE_AGENTS_OUT_DIR}}" "test-integration-write-agents" \
      "./{{BIN_DIR}}/kbase write-agents --out {{WRITE_AGENTS_OUT_DIR}}/out\n./{{BIN_DIR}}/kbase write-agents --out {{WRITE_AGENTS_OUT_DIR}}/out  # rerun: full-conflict refusal\nrm {{WRITE_AGENTS_OUT_DIR}}/out/docent.md && ./{{BIN_DIR}}/kbase write-agents --out {{WRITE_AGENTS_OUT_DIR}}/out  # partial-conflict refusal"

# `kbase build`'s delivery precondition [MAD2: B-7]: an --out that already
# holds something refuses (exit 1, every entry named, nothing written) UNLESS
# the directory is an interrupted job's own remains — temp-work/ present, its
# run.json absent (checkOutIsClear / interruptedJob in cmd/build.go). Proven
# here rather than only in Go unit tests because CONVENTIONS.md's "results are part
# of the test" applies to a refusal exactly as it does to a success: the front
# door is `kbase build`, invoked twice into the same directory, not an
# in-process call to the check function.
#
# Uses the mechanical config fixture over the Rojo corpus — same offline pair
# test-integration-build-mechanical-rojo uses — so this recipe needs no
# provider and joins the omnibus. It is corpus-bound (unlike
# test-integration-write-agents) but does not duplicate that recipe's ten-gate
# assertions: this one is about the precondition at job setup, before the
# corpus is even read.
[doc("hermetic: kbase build into a populated --out refuses (full-conflict) or resumes (interrupted-job remains); joins the omnibus")]
test-integration-build-refusal:
    @mkdir -p "{{BUILD_REFUSAL_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-refusal 2>&1 | tee "{{BUILD_REFUSAL_OUT_DIR}}/log.txt"

# The body, split out so the wrapper above can tee ONE stream — same reason as
# _test-integration-survey-rojo.
#
# Seven builds into the SAME --out, in order: (1) a normal --keep-temp-work
# build, which must succeed and leaves temp-work/run.json behind: exactly the
# "finished" state checkOutIsClear's rerun clause detects. (2) an identical
# rerun with nothing touched: must refuse, both because the directory holds
# files and because interruptedJob reads run.json and calls this a rerun — the
# refusal message is asserted for both clauses' wording, and the delivered
# tree is diffed against a snapshot taken right after (1) to prove the refused
# attempt wrote nothing. (3) the interrupted-job clause itself: the delivered
# files are removed and run.json is deleted, leaving exactly "temp-work/
# present, run.json absent" — the one state checkOutIsClear licenses an
# overwrite for — and the same build must now succeed and re-deliver the
# identical tree (reusing the temp-work store rather than recomputing it,
# which is the resume path's own point, not just this recipe's). (4) the
# kill-window clause: only run.json is deleted this time, so the delivered
# files AND temp-work/delivery.json both survive — the one state
# deliveryFinished reads as "already complete, only the run record was lost".
# The rebuild must succeed, restore run.json, and touch nothing outside
# temp-work (a marker file plus `find -newer` proves no delivered byte was
# recopied) — and a further rerun must refuse again with the rerun-diagnosis
# marker, proving the refusal is re-armed once the record is back. (5) the
# same interrupted-job state again (delivered files removed, run.json
# deleted), but rebuilt with --fresh: must succeed AND report "0 reused"
# (proving the store was actually discarded, not silently resumed) AND
# re-deliver the identical tree — --fresh changes how the work is done, never
# what gets delivered. (6) --fresh into the FINISHED directory left by (5):
# still refuses, identically to (2) — [MAD2: B-7 x B-12] --fresh discards
# kbase's own scratch tree, never the populated-out refusal, and this recipe
# is that composition's one proof. (7) the changed-plan clause: run.json is
# deleted and the build repeats with --annex getting-started added, so the new
# plan no longer delivers the three domains that territory held —
# porting-an-existing-game/, creating-a-new-game/, and installation/ —
# sweepPriorDelivery must remove exactly the prior delivery's orphaned paths
# (a before/after file-list comm proves nothing else moved) and the annex
# must land. Then the
# postcondition's own restraint is proven the other way: run.json is deleted
# once more, a foreign notes.md is planted beside the delivered tree, and the
# rebuild must refuse on checkOutHoldsExactly's "no delivered path names"
# clause, naming notes.md, without touching its content — the file this
# appliance never wrote is a file it never removes either.
[private]
_test-integration-build-refusal: build prep-test-integration-rojo
    @kb="{{BUILD_REFUSAL_OUT_DIR}}/kb"; \
    snapshot="{{BUILD_REFUSAL_OUT_DIR}}/kb-snapshot"; \
    rm -rf "$kb" "$snapshot"; \
    ./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" \
      --config-dir "{{MECHANICAL_CONFIG_DIR}}" \
      --out "$kb" \
      --keep-temp-work || \
      { echo "integration(build-refusal): the first build into an empty directory failed"; exit 1; }; \
    [[ -f "$kb/temp-work/run.json" ]] || \
      { echo "integration(build-refusal): no run.json after the first build; see $kb/temp-work"; exit 1; }; \
    cp -R "$kb" "$snapshot"; \
    rerun_err="$(./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" --config-dir "{{MECHANICAL_CONFIG_DIR}}" --out "$kb" --keep-temp-work 2>&1)"; \
    rerun_status=$?; \
    [[ "$rerun_status" == 1 ]] || \
      { echo "integration(build-refusal): rerun into the finished directory exited $rerun_status, want 1"; exit 1; }; \
    echo "$rerun_err" | grep -qF "refusing to deliver into a directory that is not empty" || \
      { echo "integration(build-refusal): refusal is missing the not-empty marker:"; echo "$rerun_err"; exit 1; }; \
    echo "$rerun_err" | grep -qF "this is a rerun, not the resume of an interrupted one" || \
      { echo "integration(build-refusal): refusal is missing the rerun-diagnosis marker:"; echo "$rerun_err"; exit 1; }; \
    diff -rq -x temp-work "$snapshot" "$kb" || \
      { echo "integration(build-refusal): the refused rerun changed the delivered tree"; exit 1; }; \
    echo "integration(build-refusal): full-conflict refusal ok — exit 1, both markers present, tree byte-unchanged"; \
    find "$kb" -mindepth 1 -maxdepth 1 -not -name temp-work -exec rm -rf {} +; \
    rm -f "$kb/temp-work/run.json"; \
    ./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" \
      --config-dir "{{MECHANICAL_CONFIG_DIR}}" \
      --out "$kb" \
      --keep-temp-work || \
      { echo "integration(build-refusal): the interrupted-job rerun (temp-work present, run.json absent) failed"; exit 1; }; \
    diff -rq -x temp-work "$snapshot" "$kb" || \
      { echo "integration(build-refusal): the resumed build re-delivered a different tree than the original"; exit 1; }; \
    echo "integration(build-refusal): interrupted-job resume ok — temp-work present/run.json absent rebuilt and re-delivered the same tree"; \
    rm -f "$kb/temp-work/run.json"; \
    marker="{{BUILD_REFUSAL_OUT_DIR}}/marker"; \
    touch "$marker"; \
    sleep 1; \
    ./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" \
      --config-dir "{{MECHANICAL_CONFIG_DIR}}" \
      --out "$kb" \
      --keep-temp-work || \
      { echo "integration(build-refusal): the kill-window rerun (run.json alone lost) failed"; exit 1; }; \
    [[ -f "$kb/temp-work/run.json" ]] || \
      { echo "integration(build-refusal): the kill-window rerun did not restore run.json"; exit 1; }; \
    recopied="$(find "$kb" -path "$kb/temp-work" -prune -o -newer "$marker" -print)"; \
    [[ -z "$recopied" ]] || \
      { echo "integration(build-refusal): the kill-window rerun touched files outside temp-work, so it recopied a delivery that was already complete:"; echo "$recopied"; exit 1; }; \
    rm -f "$marker"; \
    diff -rq -x temp-work "$snapshot" "$kb" || \
      { echo "integration(build-refusal): the kill-window rerun re-delivered a different tree than the original"; exit 1; }; \
    echo "integration(build-refusal): kill-window completion ok — run.json restored, nothing outside temp-work recopied, tree byte-unchanged"; \
    rearm_err="$(./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" --config-dir "{{MECHANICAL_CONFIG_DIR}}" --out "$kb" --keep-temp-work 2>&1)"; \
    rearm_status=$?; \
    [[ "$rearm_status" == 1 ]] || \
      { echo "integration(build-refusal): the rerun after kill-window completion exited $rearm_status, want 1"; exit 1; }; \
    echo "$rearm_err" | grep -qF "this is a rerun, not the resume of an interrupted one" || \
      { echo "integration(build-refusal): the rerun after kill-window completion is missing the rerun-diagnosis marker:"; echo "$rearm_err"; exit 1; }; \
    echo "integration(build-refusal): kill-window completion re-arms the refusal ok — the further rerun exits 1 with the rerun marker"; \
    find "$kb" -mindepth 1 -maxdepth 1 -not -name temp-work -exec rm -rf {} +; \
    rm -f "$kb/temp-work/run.json"; \
    fresh_out="$(./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" --config-dir "{{MECHANICAL_CONFIG_DIR}}" --out "$kb" --keep-temp-work --fresh)"; \
    fresh_status=$?; \
    [[ "$fresh_status" == 0 ]] || \
      { echo "integration(build-refusal): the --fresh rebuild over interrupted-job remains failed (exit $fresh_status)"; exit 1; }; \
    echo "$fresh_out" | grep -qE 'units: [0-9]+ produced, 0 reused, 0 failed' || \
      { echo "integration(build-refusal): --fresh rebuild did not report 0 reused — the store was not discarded:"; echo "$fresh_out"; exit 1; }; \
    diff -rq -x temp-work "$snapshot" "$kb" || \
      { echo "integration(build-refusal): the --fresh rebuild re-delivered a different tree than the original"; exit 1; }; \
    echo "integration(build-refusal): --fresh rebuild ok — 0 reused (store discarded), tree matches the resume output"; \
    freshrefuse_err="$(./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" --config-dir "{{MECHANICAL_CONFIG_DIR}}" --out "$kb" --keep-temp-work --fresh 2>&1)"; \
    freshrefuse_status=$?; \
    [[ "$freshrefuse_status" == 1 ]] || \
      { echo "integration(build-refusal): --fresh into the finished directory exited $freshrefuse_status, want 1"; exit 1; }; \
    echo "$freshrefuse_err" | grep -qF "refusing to deliver into a directory that is not empty" || \
      { echo "integration(build-refusal): --fresh refusal is missing the not-empty marker:"; echo "$freshrefuse_err"; exit 1; }; \
    echo "$freshrefuse_err" | grep -qF "this is a rerun, not the resume of an interrupted one" || \
      { echo "integration(build-refusal): --fresh refusal is missing the rerun-diagnosis marker:"; echo "$freshrefuse_err"; exit 1; }; \
    diff -rq -x temp-work "$snapshot" "$kb" || \
      { echo "integration(build-refusal): the refused --fresh attempt changed the delivered tree"; exit 1; }; \
    echo "integration(build-refusal): --fresh does not override the populated-out refusal ok — exit 1, both markers present, tree byte-unchanged"; \
    rm -rf "$snapshot"; \
    before_list="{{BUILD_REFUSAL_OUT_DIR}}/before-annex.txt"; \
    after_list="{{BUILD_REFUSAL_OUT_DIR}}/after-annex.txt"; \
    rm -f "$kb/temp-work/run.json"; \
    find "$kb" -path "$kb/temp-work" -prune -o -type f -print | sed "s|^$kb/||" | sort > "$before_list"; \
    ./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" \
      --config-dir "{{MECHANICAL_CONFIG_DIR}}" \
      --out "$kb" \
      --keep-temp-work \
      --annex getting-started || \
      { echo "integration(build-refusal): the changed-plan rerun (--annex getting-started added) failed"; exit 1; }; \
    for gone in porting-an-existing-game creating-a-new-game installation; do \
      [[ ! -e "$kb/$gone" ]] || \
        { echo "integration(build-refusal): $gone/ survived the getting-started annex"; exit 1; }; \
    done; \
    find "$kb" -path "$kb/temp-work" -prune -o -type f -print | sed "s|^$kb/||" | sort > "$after_list"; \
    new_paths="$(comm -13 "$before_list" "$after_list")"; \
    [[ -z "$new_paths" ]] || \
      { echo "integration(build-refusal): the changed-plan rerun added paths outside the expected set:"; echo "$new_paths"; exit 1; }; \
    vanished="$(comm -23 "$before_list" "$after_list")"; \
    stray_vanished="$(printf '%s\n' "$vanished" | grep -vE '^(porting-an-existing-game|creating-a-new-game|installation)/' || true)"; \
    [[ -z "$stray_vanished" ]] || \
      { echo "integration(build-refusal): the changed-plan sweep removed paths beyond the three orphaned domains' set:"; echo "$stray_vanished"; exit 1; }; \
    [[ -n "$vanished" ]] || \
      { echo "integration(build-refusal): the changed-plan sweep reported no vanished paths at all"; exit 1; }; \
    rm -f "$before_list" "$after_list"; \
    echo "integration(build-refusal): changed-plan sweep ok — porting-an-existing-game/, creating-a-new-game/, and installation/ gone entirely, only their orphaned paths vanished, nothing new appeared"; \
    rm -f "$kb/temp-work/run.json"; \
    notes="$kb/notes.md"; \
    notes_content="a user file the sweep must never touch"; \
    printf '%s\n' "$notes_content" > "$notes"; \
    restraint_err="$(./{{BIN_DIR}}/kbase build "{{ROJO_DOCS_DIR}}/docs" --config-dir "{{MECHANICAL_CONFIG_DIR}}" --out "$kb" --keep-temp-work --annex getting-started 2>&1)"; \
    restraint_status=$?; \
    [[ "$restraint_status" == 1 ]] || \
      { echo "integration(build-refusal): the rerun with a stray notes.md exited $restraint_status, want 1"; exit 1; }; \
    echo "$restraint_err" | grep -qF "no delivered path names" || \
      { echo "integration(build-refusal): the stray-file postcondition refusal is missing the 'no delivered path names' marker:"; echo "$restraint_err"; exit 1; }; \
    echo "$restraint_err" | grep -qF "notes.md" || \
      { echo "integration(build-refusal): the stray-file postcondition refusal does not name notes.md:"; echo "$restraint_err"; exit 1; }; \
    [[ "$(cat "$notes")" == "$notes_content" ]] || \
      { echo "integration(build-refusal): notes.md's content changed even though the postcondition refused delivery"; exit 1; }; \
    rm -f "$notes"; \
    echo "integration(build-refusal): changed-plan sweep restraint ok — a stray notes.md triggers the postcondition refusal by name, exit 1, content untouched"; \
    echo "integration(build-refusal) ok: full-conflict refusal, both message clauses, unchanged tree, interrupted-job resume, kill-window completion (re-armed), --fresh rebuild, --fresh-vs-refusal, changed-plan sweep, and postcondition restraint all correct"; \
    {{just_executable()}} _write-evidence "{{BUILD_REFUSAL_OUT_DIR}}" "test-integration-build-refusal" \
      "./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work  # rerun: full-conflict refusal, exit 1\nrm -rf {{BUILD_REFUSAL_OUT_DIR}}/kb/* (except temp-work) && rm {{BUILD_REFUSAL_OUT_DIR}}/kb/temp-work/run.json  # simulate interrupted-job remains\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work  # resumes and succeeds\nrm {{BUILD_REFUSAL_OUT_DIR}}/kb/temp-work/run.json  # simulate the kill-window: delivered files and delivery.json survive\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work  # kill-window completion: restores run.json, recopies nothing, exit 0\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work  # rerun after completion: refusal re-armed, exit 1\nrm -rf {{BUILD_REFUSAL_OUT_DIR}}/kb/* (except temp-work) && rm {{BUILD_REFUSAL_OUT_DIR}}/kb/temp-work/run.json  # simulate interrupted-job remains again\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work --fresh  # discards the store, 0 reused, rebuilds and re-delivers\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work --fresh  # --fresh into the finished directory still refuses, exit 1\nrm {{BUILD_REFUSAL_OUT_DIR}}/kb/temp-work/run.json  # simulate interrupted-job remains ahead of a changed plan\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work --annex getting-started  # changed-plan sweep: porting-an-existing-game/, creating-a-new-game/, installation/ removed, annex lands, exit 0\nrm {{BUILD_REFUSAL_OUT_DIR}}/kb/temp-work/run.json && touch {{BUILD_REFUSAL_OUT_DIR}}/kb/notes.md  # plant a foreign file ahead of a resumed delivery\n./{{BIN_DIR}}/kbase build {{ROJO_DOCS_DIR}}/docs --config-dir {{MECHANICAL_CONFIG_DIR}} --out {{BUILD_REFUSAL_OUT_DIR}}/kb --keep-temp-work --annex getting-started  # postcondition restraint: refuses on notes.md by name, exit 1, content untouched"

# The omnibus composes the HERMETIC per-corpus recipes, plus
# test-integration-write-agents (hermetic but corpus-free) and
# test-integration-build-refusal (hermetic, Rojo-bound), and writes no log of
# its own: each of them already preserves its full output under its own name,
# and a second copy of the same bytes under a second name is a file that can
# go stale against the one anybody reads. test-integration-build-live-rojo and
# test-integration-build-live-omlx are excluded on purpose — see their doc
# strings.
[doc("run every hermetic integration test (the live ones are excluded; run those by name)")]
test-integration: test-integration-survey-rojo test-integration-build-mechanical-rojo test-integration-survey-omlx test-integration-build-mechanical-omlx test-integration-write-agents test-integration-build-refusal

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
    cp USER_CONVENTIONS.md "$stage/CONVENTIONS.md" && \
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
# The CONVENTIONS.md vetting checklist (release date, importers, deprecation
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
