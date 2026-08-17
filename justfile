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
SURVEY_ROJO_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-survey-rojo"
BUILD_MECHANICAL_ROJO_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-mechanical-rojo"
BUILD_LIVE_ROJO_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-live-rojo"
SURVEY_OMLX_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-survey-omlx"
BUILD_MECHANICAL_OMLX_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-mechanical-omlx"
BUILD_LIVE_OMLX_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-build-live-omlx"
WRITE_AGENTS_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-write-agents"

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
# Unlike rojo.space — a small dedicated docs site, cloned whole — jundot/omlx
# is a monorepo whose docs/ is one subdirectory of an ML runtime, so a full
# shallow clone is 176M for the 73K we survey. prep fetches it sparse
# (docs/ only) and partial (blobs on demand), which is 31M. The corpus is
# therefore {{OMLX_DOCS_DIR}}/docs, not {{OMLX_DOCS_DIR}}.
OMLX_DOCS_REPO := "https://github.com/jundot/omlx"
OMLX_DOCS_COMMIT := "aef5a0cf5a1c119ea55bd82f3476fc677230af99"
OMLX_DOCS_DIR := TEST_DATA_TRANSIENT_DIR / "omlx"

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
# artifact kind actually present. "Results are part of the test" (AGENTS.md)
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
    for want in "files=8 " "tokens=10553 " "sections=83 " "unresolved=5 "; do \
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
# Front-door per AGENTS.md's "integration tests invoke the app binary" ruling:
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
    for want in '"sourceFiles": 8' '"sourceSections": 83' '"nodes": 34' '"pages": 25' '"sections": 9' '"groups": 25' '"splitGroups": 0' '"deliveredFiles": 37'; do \
      grep -qF "$want" "$rec" || { echo "integration(build-mechanical-rojo): expected $want in $rec"; exit 1; }; \
    done; \
    for want in entry-point.md AGENTS.md README.md CLAUDE.md; do \
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
    for want in entry-point.md AGENTS.md README.md CLAUDE.md; do \
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
# The fetch is sparse + partial rather than the plain shallow clone Rojo's
# prep does — see the pin block for why. `sparse-checkout set docs` in cone
# mode also keeps the repository's top-level files; they are harmless, since
# ingest reads only the .md under the root it is given, and that root is
# {{OMLX_DOCS_DIR}}/docs.
[doc("fetch the pinned oMLX docs corpus, sparse + partial (no-op when present)")]
prep-test-integration-omlx:
    @if [[ -f "{{OMLX_DOCS_DIR}}/.git/HEAD" ]]; then \
      echo "omlx docs present: {{OMLX_DOCS_DIR}}"; \
    else \
      mkdir -p "{{OMLX_DOCS_DIR}}" && \
      git -C "{{OMLX_DOCS_DIR}}" init -q && \
      git -C "{{OMLX_DOCS_DIR}}" remote add origin "{{OMLX_DOCS_REPO}}" && \
      git -C "{{OMLX_DOCS_DIR}}" sparse-checkout init --cone && \
      git -C "{{OMLX_DOCS_DIR}}" sparse-checkout set docs && \
      git -C "{{OMLX_DOCS_DIR}}" fetch -q --depth 1 --filter=blob:none origin "{{OMLX_DOCS_COMMIT}}" && \
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
    for want in '"sourceFiles": 5' '"sourceSections": 110' '"nodes": 12' '"pages": 6' '"sections": 6' '"groups": 5' '"splitGroups": 1' '"deliveredFiles": 15'; do \
      grep -qF "$want" "$rec" || { echo "integration(build-mechanical-omlx): expected $want in $rec"; exit 1; }; \
    done; \
    for want in entry-point.md AGENTS.md README.md CLAUDE.md; do \
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
    for want in entry-point.md AGENTS.md README.md CLAUDE.md; do \
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

# The omnibus composes the HERMETIC per-corpus recipes, plus
# test-integration-write-agents (hermetic but corpus-free), and writes
# no log of its own: each of them already preserves its full output under its
# own name, and a second copy of the same bytes under a second name is a file
# that can go stale against the one anybody reads. test-integration-build-live-rojo
# and test-integration-build-live-omlx are excluded on purpose — see their doc
# strings.
[doc("run every hermetic integration test (the live ones are excluded; run those by name)")]
test-integration: test-integration-survey-rojo test-integration-build-mechanical-rojo test-integration-survey-omlx test-integration-build-mechanical-omlx test-integration-write-agents

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
