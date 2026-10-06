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
RESULTS_FIXTURE_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-results-fixture"
MONITOR_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-monitor-arxiv"
# The committed KB the hermetic result-contract recipe works over, with the
# values its write ops take.
RESULTS_FIXTURE_DIR := TEST_DATA_FIXTURES_DIR / "results"
MCP_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-mcp"
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
ARXIV_IDS := ARXIV_CS_GR + " " + ARXIV_CS_LG + " " + ARXIV_ECON_TH + " " + ARXIV_EESS_AS + " " + ARXIV_MATH_DG + " " + ARXIV_MATH_ST + " " + ARXIV_MATH_AG + " " + ARXIV_MATH_NT + " " + ARXIV_MATH_AP + " " + ARXIV_MATH_PR + " " + ARXIV_PHYSICS_OPTICS
# The e-prints as downloaded, and one unpacked source tree per id beside them.
ARXIV_TGZ_DIR := TEST_DATA_TRANSIENT_DIR / "arxiv-tgz"
ARXIV_DIR := TEST_DATA_TRANSIENT_DIR / "arxiv"
# arXiv's published courtesy interval for automated access, paid only on a
# download that actually happens.
ARXIV_DELAY := "3"

# The slice: kb_tools' reference trees, the fixture repositories they were
# built in, and each slice stage's paper list.
KBTOOLS_REF_DIR := TEST_DATA_TRANSIENT_DIR / "kbtools-ref"
SLICE_REF_DIR := TEST_DATA_TRANSIENT_DIR / "slice-ref"
SLICE_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-slice-arxiv"
DOCGRAPH_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-docgraph-arxiv"
# The file in each reference fixture naming the adjagent commit it was staged at.
KBTOOLS_REF_PIN_FILE := "adjagent-commit.txt"
# The file in a reference directory recording the driver's stop — its exit code
# and console log — where it stopped short of a document-graph commit; it stands
# in for kb-root/.
KBTOOLS_REF_STOP_FILE := "driver-stop.yaml"
SLICE_1_IDS := "2609.10318v1 2609.09855v1"
SLICE_2_IDS := "2609.10525v1 2609.10291v1 2609.10111v1 2609.10385v1"
# Stage 2's negative controls: the first must refuse naming the paper and
# pandoc's error, the second naming each unloaded file and the line that named it.
SLICE_2_UNPARSEABLE_ID := "2609.10029v1"
SLICE_2_UNLOADABLE_ID := "2609.09821v1"
# kb_tools-built KBs kbase's refresh and verify are measured against: the
# slice's six papers, built complete by prep-test-integration-kbtools-full.
VERIFY_ARXIV_IDS := SLICE_1_IDS + " " + SLICE_2_IDS
VERIFY_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-verify-arxiv"
WRITE_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-write-arxiv"
QUERY_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-query-arxiv"
CLAIMGRAPH_ARXIV_OUT_DIR := TEST_DATA_TRANSIENT_DIR / "test-integration-claimgraph-arxiv"
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
COMPAT_OP_SCRIPT := TEST_DATA_DIR / "fixtures" / "compat" / "ops.yaml"

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

AGENTS_REPO := "https://github.com/benn-herrera/adjagent.git"
AGENTS_DIR := ".claude" / file_stem(AGENTS_REPO)
agents:
  @mkdir -p "{{parent_directory(AGENTS_DIR)}}"
  @[[ -d "{{AGENTS_DIR}}" ]] && git -C "{{AGENTS_DIR}}" pull || git -C {{parent_directory(AGENTS_DIR)}} clone "{{AGENTS_REPO}}"
  just --justfile "{{AGENTS_DIR}}/justfile" install "$(pwd)"

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
      compgen -G "{{out_dir}}/*/run-1.result.yaml" > /dev/null && echo "- <id>/: per paper, the fixture repositories run-<n>/ (kb-root/ inside), state-<n>/records/, and each build's run-<n>.result.yaml, run-<n>.log, run-<n>.exit"; \
      compgen -G "{{out_dir}}/*/dump.json" > /dev/null       && echo "- <id>/dump.json: kb_tools' readers over run-1's tree and the reference tree"; \
      compgen -G "{{out_dir}}/*/comparison.yaml" > /dev/null && echo "- <id>/comparison.yaml: every comparison check, pass or fail, its evidence paths and what differs"; \
      compgen -G "{{out_dir}}/*/records.yaml" > /dev/null    && echo "- <id>/records.yaml: the Records check and the record join, each with its evidence paths and what differs"; \
      compgen -G "{{out_dir}}/*/census.yaml" > /dev/null     && echo "- <id>/census.yaml: the paper's census"; \
      [[ -f "{{out_dir}}/citation-states.yaml" ]]           && echo "- citation-states.yaml: citation states pooled over slice stages 1 and 2"; \
      [[ -f "{{out_dir}}/rollup.yaml" ]]                    && echo "- rollup.yaml: per comparison check, the papers passing and failing; per build check, the papers reporting each status"; \
      [[ -f "{{out_dir}}/comparison.yaml" && ! -f "{{out_dir}}/compare.log" ]] && echo "- comparison.yaml: per kb_tools-built fixture, kbase verify's and refresh's exits and every finding: a red verify, a fixture verify changed, a path refresh changed"; \
      [[ -f "{{out_dir}}/comparison.yaml" && -f "{{out_dir}}/compare.log" ]] && echo "- comparison.yaml, compare.log: per query its cases, how many equal and how many answered non-empty; every case whose kbase and kb_cmd answers differ as data, its arguments and both values"; \
      [[ -f "{{out_dir}}/findings.yaml" ]]          && echo "- findings.yaml: per fixture, every finding beside the comparison: the fixture left changed, a scoring op or a render-claim-graph step not as expected"; \
      [[ -f "{{out_dir}}/limit.log" ]]              && echo "- limit.log: every list query at its default limit over each fixture, each result within the tool-result cap, and the largest per query"; \
      [[ -f "{{out_dir}}/checks.tsv" ]]             && echo "- checks.tsv, docs/: per invocation its arguments, the outcome wanted and pass or fail; docs/NN-<verb>.out, .log, .exit and .keys.log (the documented-keys parse); repo/ as the run left it"; \
      [[ -f "{{out_dir}}/comparison.tsv" ]]         && echo "- comparison.tsv, transcripts/<name>/: per MCP transcript pass or the response line that differs; requests.jsonl, expected.jsonl (placeholders substituted), actual.jsonl, mcp.log, mcp.exit; insert.*, refresh.* (.out, .log) and repo/ as the staging left them"; \
      [[ -f "{{out_dir}}/monitor.tsv" ]]           && echo "- monitor.tsv, polls/: every status poll the monitor made while the build ran (polls/NNN.out); build.*, cancel.*, cancel-again.*, status-*.*, resume.* (.out, .log, .exit, .keys.log); repo/ and state/ as the run left them"; \
      compgen -G "{{out_dir}}/*/fixture/*.args" > /dev/null && echo "- <id>/: fixture/ (per case NNN.args — the query, then one argument per line — and NNN.kbase.* and NNN.kbcmd.* .out, .log, .exit); scored-clone/, scores/ (the values and each op's run) and scored/ (the cases on the scored clone); sheet/ (clone/, drawn.svg, each render-claim-graph and verify run); stage.log"; \
      compgen -G "{{out_dir}}/*/verify.result.yaml" > /dev/null && echo "- <id>/: verify.result.yaml and verify.log on the fixture; clone/ refreshed by kbase, refresh.result.yaml, refresh.log and refresh.diff"; \
      [[ -d "{{out_dir}}/kbtools-agreement" ]]      && echo "- kbtools-agreement/: refresh-agreement.yaml and verify-agreement.yaml, each toolchain run over one input, with every input's trees and logs under refresh/ and verify/"; \
      [[ -f "{{out_dir}}/kbtools-fixtures.log" ]]   && echo "- kbtools-fixtures.log: kb_tools' render goldens and write-API regression replay, run over the write package"; \
      [[ -f "{{out_dir}}/summary.yaml" ]]           && echo "- summary.yaml: per fixture, every finding of the op-script run"; \
      compgen -G "{{out_dir}}/*/ref/commit.txt" > /dev/null && echo "- <id>/: repo/ (the fixture repository kbase built: kb-root/, kb-build-node-pass.yaml, kb-build-classification.yaml, kb-build-unmarked.yaml), repo-2/ and state-2/ (the second build, compared modulo node ids), state/ (records/, reports/), ledger.txt (kbase's boundary commits) and status.* (kbase status over the build), ref/ (kb_tools' kb-root/ and build records at commit.txt, its ledger to that commit in ledger.txt, its stage vocabulary in stage-ids.txt, its depends-attributed report in depends-report.txt), kbtools-verify/ (kb_tools' fixture with kbase's kb-root/), checks/ (kbase-build, kbase-refresh, kbase-verify, kbtools-verify: .out, .log, .exit), comparison.yaml, compare.log"; \
      compgen -G "{{out_dir}}/*/checks/kbase-status.out" > /dev/null && echo "- comparison.yaml: per paper every Direction 1 check, pass or fail, its detail and evidence path"; \
      compgen -G "{{out_dir}}/*/checks/kbase-status.out" > /dev/null && echo "- <id>/: repo/ (the fixture repository kbase built, kb_tools installed, the op script applied), built/ (kb-root/ and the build records as kbase's build left them), state/ (progress.jsonl, records/), config/ (empty), opscript/ and kbtools-steps/ (the op script rendered, and each step run through kb_util), excerpts.kbtools.txt, checks/ (per command .out, .log, .exit); on the special paper also kill/, guard/ and stamp/ (each repo/ and state/), kill/kill.txt, kill/comparison.yaml and kill/compare.log (the resumed build against built/ modulo node ids)"; \
      compgen -G "{{out_dir}}/*/nodepass.json" > /dev/null && echo "- comparison.yaml: the paper's checks, its state-dir, the scratch/ census and the counts (paragraphs asked, claims minted, defaults by cause; the unmarked pairs planned and asked, their letters and outcomes, the source groups and calls; candidates by harvest, by letter and outcome, own-equation pairs dropped, edges a ring demoted); <id>/: repo/ (the fixture repository the build wrote, kb_tools installed), state/ (progress.jsonl, records/, scratch/captures/, scratch/answers/, scratch/asks/), counts.yaml, nodepass.json, kb-build-node-pass.yaml and unmarked-plan.json (the node-pass record in kb_tools' spelling and the planned pairs), at-references-found/ (kb-root/ at the references-found commit), checks/ (per command .out, .log, .exit); models.*: the provider's catalogue, read before the build"; \
      compgen -G "{{out_dir}}/*/steps/steps.tsv" > /dev/null && echo "- <id>/: clones kbase/ and kbtools/; steps/ (the rendered script, choice.yaml); kbase-steps/, kbtools-steps/, rerun-steps/ (per step its values, .out, .log, .exit); ids.txt; kbtools-refresh.*; kbase-verify.<clone>.* and kbtools-verify.<clone>.*; raw.diff; comparison.yaml; kb-root.before-rerun/ and rerun.diff"; \
      true; \
    } > "{{out_dir}}/EVIDENCE.md"

# Fetch and unpack happen in one recipe because the corpus a test reads is the
# unpacked tree. Both steps land atomically (a .part file, a .unpack dir, then
# a rename), so an interrupted run leaves nothing a later run mistakes for done.
# An e-print is usually a tarball but may be a single gzipped .tex — a one-file
# submission has nothing to tar — and that lands as paper.tex.
[doc("fetch and unpack the pinned arXiv sample, one source tree per id under test_data/transient/arxiv/ (no-op per id when present); pass ids= to take a subset")]
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

# kb_tools' own document-graph tree for each paper: the reference the slice
# compares kbase's tree against. Each paper is staged as a fixture repository
# the way kb-testing's stage-arxiv-paper stages one, gets the installed agent
# set (kb_tools included) from this repository's own adjagent clone, and is
# built by kb_tools' driver with --no-inference --through document-graph. The
# tree is read out of the document-graph stage commit, so what lands in
# slice-ref/ is exactly what kb_tools committed. The fixture records the
# adjagent commit its tooling was installed from; an id is re-staged unless its
# reference tree exists and that commit is the clone's HEAD. A driver that stops
# short of the document-graph commit leaves its exit code and log in place of the tree,
# and the next run stages that id again.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): build kb_tools' reference document-graph tree for each id into test_data/transient/slice-ref/<id>/kb-root, or record the driver's stop in slice-ref/<id>/driver-stop.yaml (no-op per id when the tree is present and staged at the adjagent clone's HEAD)")]
prep-test-integration-kbtools-ref ids:
    #!/usr/bin/env bash
    set -euo pipefail
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    for id in {{ids}}; do
        name="$(printf '%s' "${id}" | tr '/' '_')"
        ref="{{SLICE_REF_DIR}}/${name}"
        fixture="{{KBTOOLS_REF_DIR}}/${name}"
        pin="${fixture}/{{KBTOOLS_REF_PIN_FILE}}"
        if [[ -d "${ref}/kb-root" && -f "${pin}" && "$(cat "${pin}")" == "${adjagent}" ]]; then
            printf 'kbtools-ref %s: present at %s/kb-root, staged at adjagent %s\n' "${id}" "${ref}" "${adjagent}"
            continue
        fi
        log_dir="{{KBTOOLS_REF_DIR}}/logs"
        log="${log_dir}/${name}.console.log"
        doc_graph() { git -C "${fixture}" log -n1 --format=%H --grep='^kb-build: document-graph '; }
        if [[ ! -d "${fixture}/.git" || ! -f "${pin}" || "$(cat "${pin}")" != "${adjagent}" ]]; then
            {{just_executable()}} _kbtools-ref-stage "${id}"
        fi
        # A fixture pinned at the clone's HEAD is never restaged: with the
        # document-graph commit already in its ledger (whatever stage the build
        # has reached since) the tree is read out of it; otherwise the driver
        # resumes from the ledger.
        if [[ -n "$(doc_graph)" ]]; then
            driver_exit=0
        else
            {{just_executable()}} _kbtools-ref-drive "${id}" document-graph
            driver_exit="$(cat "${log_dir}/${name}.exit")"
        fi
        commit="$(doc_graph)"
        rm -rf "${ref}"
        mkdir -p "${ref}"
        if [[ "${driver_exit}" -ne 0 && "${driver_exit}" -ne 18 ]] || [[ -z "${commit}" ]]; then
            # The driver's own stop is a reference result too: recorded, never
            # cached, so the next run asks the driver again.
            printf 'exit: %s\nlog: "%s"\n' "${driver_exit}" "${log}" > "${ref}/{{KBTOOLS_REF_STOP_FILE}}"
            printf 'kbtools-ref %s: the driver stopped (exit %s, document-graph commit %s) — log: %s\n' \
                "${id}" "${driver_exit}" "${commit:-none}" "${log}"
            continue
        fi
        git -C "${fixture}" archive "${commit}" kb-root | tar -x -C "${ref}"
        printf 'kbtools-ref %s: %s/kb-root from %s\n' "${id}" "${ref}" "${commit}"
    done

# The complete kb_tools-built KB for each id, in the same fixture repository
# the reference recipe stages: the driver --no-inference with no --through
# walks every stage to overview-drafted, so the fixture holds kb-root/ as
# kb_tools finishes it and the kb-build: commit trail that built it. A fixture
# staged at the adjagent clone's HEAD resumes from its ledger (the document-graph
# commit the reference recipe left, or wherever an earlier run stopped);
# any other is staged afresh, which also drops its slice-ref/ tree. An id whose
# ledger already holds the overview-drafted commit at that HEAD is a no-op. The
# driver's exit code is kept in kbtools-ref/logs/<id>.exit and its stop, where
# it did not finish, in kbtools-ref/logs/<id>.console.log.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): build kb_tools' complete KB (driver --no-inference through overview-drafted) for each id in its fixture repository test_data/transient/kbtools-ref/<id>/, exit code in kbtools-ref/logs/<id>.exit (no-op per id when the fixture's ledger holds overview-drafted at the adjagent clone's HEAD)")]
prep-test-integration-kbtools-full ids:
    #!/usr/bin/env bash
    set -euo pipefail
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    for id in {{ids}}; do
        name="$(printf '%s' "${id}" | tr '/' '_')"
        fixture="{{KBTOOLS_REF_DIR}}/${name}"
        pin="${fixture}/{{KBTOOLS_REF_PIN_FILE}}"
        exit_file="{{KBTOOLS_REF_DIR}}/logs/${name}.exit"
        if [[ -f "${pin}" && "$(cat "${pin}")" == "${adjagent}" ]]; then
            if [[ -n "$(git -C "${fixture}" log -n1 --format=%H --grep='^kb-build: overview-drafted ')" ]]; then
                printf 'kbtools-full %s: overview-drafted at adjagent %s\n' "${id}" "${adjagent}"
                continue
            fi
        else
            {{just_executable()}} _kbtools-ref-stage "${id}"
        fi
        {{just_executable()}} _kbtools-ref-drive "${id}" ""
        driver_exit="$(cat "${exit_file}")"
        stage="$(git -C "${fixture}" log -n1 --format=%s --grep='^kb-build: ' | sed -e 's/^kb-build: //' -e 's/ |.*//')"
        printf 'kbtools-full %s: driver exit %s, ledger at %s — log: %s\n' \
            "${id}" "${driver_exit}" "${stage:-none}" "{{KBTOOLS_REF_DIR}}/logs/${name}.console.log"
    done

# One fixture repository per id the way kb-testing's stage-arxiv-paper stages
# one: the sources as fetched, a stub Makefile, a .gitignore for the scratch
# directory, the reference agent set installed from the adjagent clone, the
# clone's commit pinned. Everything an earlier staging of the id left — its
# fixture, run directory, console log and slice-ref/ tree — is removed first.
[private]
_kbtools-ref-stage id:
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    id="{{id}}"
    name="$(printf '%s' "${id}" | tr '/' '_')"
    src="{{ARXIV_DIR}}/${name}"
    fixture="{{KBTOOLS_REF_DIR}}/${name}"
    if [[ ! -d "${src}" ]]; then
        printf 'error: %s is not fetched — run `just prep-test-integration-arxiv "%s"` first\n' "${src}" "${id}" >&2
        exit 1
    fi
    log="{{KBTOOLS_REF_DIR}}/logs/${name}.console.log"
    notice=""
    mkdir -p "{{KBTOOLS_REF_DIR}}/logs"
    if [[ -e "${fixture}" ]]; then
        notice="restaging ${fixture}: pin is not the adjagent clone's HEAD, or the fixture is not a repository — deleting it, its run directory, logs and slice-ref tree"
        printf '%s\n' "${notice}" | tee -a "${log}" >&2
    fi
    rm -rf "${fixture}" "{{KBTOOLS_REF_DIR}}/runs/${name}" "{{SLICE_REF_DIR}}/${name}" "{{KBTOOLS_REF_DIR}}/logs/${name}".*
    mkdir -p "${fixture}" "{{KBTOOLS_REF_DIR}}/runs/${name}" "{{KBTOOLS_REF_DIR}}/logs"
    if [[ -n "${notice}" ]]; then
        printf '%s\n' "${notice}" > "${log}"
    fi
    cp -R "${src}/." "${fixture}/"
    printf 'default:\n\t@echo %s\n\ntest:\n\t@echo tests passed.\n' "${id}" > "${fixture}/Makefile"
    printf '%s\n' '.claude-temp/' '__pycache__/' > "${fixture}/.gitignore"
    git -C "${fixture}" init -q
    git -C "${fixture}" add -A
    git -C "${fixture}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "sources as fetched"
    just --justfile .claude/adjagent/justfile install "${root}/${fixture}"
    git -C "${root}/.claude/adjagent" rev-parse HEAD > "${fixture}/{{KBTOOLS_REF_PIN_FILE}}"
    git -C "${fixture}" add -A
    git -C "${fixture}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "agent defs installed"

# kb_tools' driver --no-inference over a staged fixture, from its ledger
# position, to `through` (a stage id; empty for no bound). The console log
# gains the run's header and output; the driver's exit code lands in
# logs/<id>.exit, and a nonzero one does not fail this recipe — the caller reads
# it. A --through run that stops at its bound exits 18, the driver's
# EXIT_BOUNDED (kb_tools/kb_driver/baton.py); 0 is the unbounded success.
[private]
_kbtools-ref-drive id through:
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    name="$(printf '%s' "{{id}}" | tr '/' '_')"
    fixture="{{KBTOOLS_REF_DIR}}/${name}"
    log="{{KBTOOLS_REF_DIR}}/logs/${name}.console.log"
    volume_root="$({{just_executable()}} _arxiv-volume-root "${fixture}")"
    bound=()
    if [[ -n "{{through}}" ]]; then
        bound=(--through "{{through}}")
    fi
    {
        printf 'adjagent: %s\n' "$(cat "${fixture}/{{KBTOOLS_REF_PIN_FILE}}")"
        pandoc --version | head -n 1
        printf 'volume root: %s\n' "${volume_root}"
    } >> "${log}"
    set +e
    ( cd "${fixture}" && \
      PYTHONPATH=.claude/agents python3 -m kb_tools.kb_driver run \
        --source "${volume_root}" \
        --no-inference \
        ${bound[@]+"${bound[@]}"} \
        --run-dir "${root}/{{KBTOOLS_REF_DIR}}/runs/${name}" ) >> "${log}" 2>&1
    driver_exit=$?
    set -e
    printf '%s\n' "${driver_exit}" > "{{KBTOOLS_REF_DIR}}/logs/${name}.exit"

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

# A tools/measure script over kb-root directories, with kb_tools' readers on
# PYTHONPATH. The scripts accept --out only under .claude-temp/, so the output
# directory is fixed here, one per run, and printed.
[positional-arguments]
[doc("NEEDS python3 and the installed agent set under .claude/agents (excluded from test-integration): run tools/measure/<script>.py over the kb-roots its arguments name, --out under .claude-temp/measure/<script-stem>/<timestamp>/ (printed); read-only over the kb-roots, which are relative to the repository root or absolute. e.g. just measure-kb-roots tools/measure/compare_to_pristine.py --ours <kb-root> --reference <kb-root>; then measure_unmarked_shortlist.py --ours <kb-root> --edges <out dir>/edges.tsv")]
measure-kb-roots script +dirs:
    #!/usr/bin/env bash
    set -euo pipefail
    script="${1}"
    shift
    if [[ ! -f "${script}" ]]; then
        printf 'error: script %s does not exist.\n' "${script}" >&2
        exit 1
    fi
    stem="$(basename "${script}" .py)"
    out="$(pwd -P)/.claude-temp/measure/${stem}/$(date +%Y%m%dT%H%M%S)"
    PYTHONPATH=.claude/agents python3 "${script}" "$@" --out "${out}"
    printf 'measure-kb-roots: output in %s\n' "${out}"

# The slice's evidence for one stage (1, 2 or 3), under its own directory
# stage-<n>/: stage 3's corpus includes every stage-1 and stage-2 paper, so a
# shared directory would rebuild their fixtures and delete their comparisons.
# Stages 1 and 2 build each paper twice and compare it against kb_tools'
# reference; stage 2 adds its two negative controls and pools citation states
# over its own papers and stage 1's, which must have run. Stage 3 builds every
# corpus paper once and takes its census, with no pass threshold. The body is
# _docgraph-arxiv's.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): slice stage 1, 2 or 3 — build the stage's papers with ./bin/kbase under test_data/transient/test-integration-slice-arxiv/stage-<n>/<id>/, compare them against kb_tools' reference (stages 1, 2) or take their census (stage 3)")]
test-integration-slice-arxiv stage:
    @mkdir -p "{{SLICE_ARXIV_OUT_DIR}}/stage-{{stage}}"
    @{{just_executable()}} _test-integration-slice-arxiv "{{stage}}" 2>&1 | tee "{{SLICE_ARXIV_OUT_DIR}}/stage-{{stage}}/log.txt"

[private]
_test-integration-slice-arxiv stage:
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{stage}}" in
        1) set -- "{{SLICE_1_IDS}}" "1 2" "" "compare" ;;
        2) set -- "{{SLICE_2_IDS}}" "1 2" "{{SLICE_2_UNPARSEABLE_ID}} {{SLICE_2_UNLOADABLE_ID}}" "compare controls states" ;;
        3) set -- "{{ARXIV_IDS}}" "1" "" "census" ;;
        *) printf 'error: stage is 1, 2 or 3, not %s\n' "{{stage}}" >&2; exit 1 ;;
    esac
    {{just_executable()}} _docgraph-arxiv "{{SLICE_ARXIV_OUT_DIR}}/stage-{{stage}}" "$1" "$2" "$3" "$4" "test-integration-slice-arxiv {{stage}}"

# The document graph over the whole corpus: every paper built twice and
# compared against kb_tools' reference by the slice's checks, its census taken,
# both negative controls built once, and the roll-up of every comparison and of
# each build's own checks written to rollup.yaml and EVIDENCE.md. A paper that
# refuses passes only where kb_tools' driver stopped on it too.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): build every corpus paper twice with ./bin/kbase --through document-graph under test_data/transient/test-integration-docgraph-arxiv/<id>/, compare each against kb_tools' reference, take its census, check both negative controls refuse, and roll the results up")]
test-integration-docgraph-arxiv:
    @mkdir -p "{{DOCGRAPH_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _docgraph-arxiv "{{DOCGRAPH_ARXIV_OUT_DIR}}" "{{ARXIV_IDS}}" "1 2" "{{SLICE_2_UNPARSEABLE_ID}} {{SLICE_2_UNLOADABLE_ID}}" "compare controls census rollup" "test-integration-docgraph-arxiv" 2>&1 | tee "{{DOCGRAPH_ARXIV_OUT_DIR}}/log.txt"

# One body for every document-graph comparison over arXiv papers, writing under
# out/<id>/. Each paper is copied into fresh fixture repositories run-<n>/ and
# built there with ./bin/kbase build --through document-graph, passing every
# .bib beside the volume root in sorted path order; each build keeps its stderr
# log, YAML result and exit code beside its fixture. kb_tools' readers then dump
# run-1's tree and, where there is one, the reference tree. controls is the
# unparseable control then the unloadable one, each built once. checks names
# what the comparator does: compare (against kb_tools' reference, staged
# first), controls, states (pooled with slice stage 1's papers), census, rollup.
[private]
_docgraph-arxiv out ids runs controls checks recipe: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{out}}"
    ids="{{ids}}"
    checks=" {{checks}} "
    read -r unparseable unloadable <<< "{{controls}} " || true
    {{just_executable()}} prep-test-integration-arxiv "${ids} {{controls}}"
    if [[ "${checks}" == *" compare "* ]]; then
        {{just_executable()}} prep-test-integration-kbtools-ref "${ids}"
    fi
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    pandoc_version="$(pandoc --version | head -n 1)"
    printf 'adjagent: %s\n%s\n' "${adjagent}" "${pandoc_version}"

    # One fresh fixture repository holding the paper's sources, and one build
    # in it. The build's inputs, YAML result, stderr log and exit code land
    # beside the fixture.
    build_once() {
        local id="$1" n="$2" dir="${out}/$1"
        local fixture="${dir}/run-${n}"
        rm -rf "${fixture}" "${dir}/state-${n}"
        mkdir -p "${fixture}"
        cp -R "{{ARXIV_DIR}}/${id}/." "${fixture}/"
        git -C "${fixture}" init -q
        local volume_root
        if ! volume_root="$({{just_executable()}} _arxiv-volume-root "${fixture}")"; then
            printf 'no volume root\n' > "${dir}/run-${n}.log"
            printf '2\n' > "${dir}/run-${n}.exit"
            return
        fi
        printf '%s\n' "${volume_root}" > "${dir}/volume-root.txt"
        (cd "${fixture}" && find "$(dirname "${volume_root}")" -maxdepth 1 -name '*.bib' | sed 's|^\./||' | LC_ALL=C sort) > "${dir}/bibliographies.txt"
        local bibs=()
        while IFS= read -r bib; do
            bibs+=(--bibliography "${bib}")
        done < "${dir}/bibliographies.txt"
        set +e
        ( cd "${fixture}" && "${root}/{{BIN_DIR}}/kbase" build "${volume_root}" \
            --through document-graph \
            --state-dir "${root}/${dir}/state-${n}" \
            ${bibs[@]+"${bibs[@]}"} ) > "${dir}/run-${n}.result.yaml" 2> "${dir}/run-${n}.log"
        printf '%s\n' "$?" > "${dir}/run-${n}.exit"
        set -e
    }

    # kb_tools' readers over run-1's tree and, where there is one, the
    # reference tree. kb_tools comes from the reference fixture's installed
    # agent set where there is one, else from this repository's own; both are
    # the adjagent clone at ${adjagent}.
    dump() {
        local id="$1" dir="${out}/$1" agents=".claude/agents" ref=()
        if [[ -d "{{SLICE_REF_DIR}}/${id}/kb-root" ]]; then
            agents="{{KBTOOLS_REF_DIR}}/${id}/.claude/agents"
            ref=(--ref "${id}={{SLICE_REF_DIR}}/${id}/kb-root")
        fi
        PYTHONPATH="${agents}" python3 tools/slice/kbtools_dump.py \
            ${ref[@]+"${ref[@]}"} --kbase "${id}=${dir}/run-1/kb-root" --out "${dir}/dump.json"
    }

    for id in ${ids}; do
        mkdir -p "${out}/${id}"
        rm -f "${out}/${id}/dump.json" "${out}/${id}/comparison.yaml" "${out}/${id}/census.yaml"
        for n in {{runs}}; do
            build_once "${id}" "${n}"
            printf 'built %s run-%s: exit %s\n' "${id}" "${n}" "$(cat "${out}/${id}/run-${n}.exit")"
        done
        if [[ -f "${out}/${id}/run-1/kb-root/entry-point.md" ]]; then
            dump "${id}" || printf 'dump failed for %s\n' "${id}"
        fi
    done
    for id in {{controls}}; do
        mkdir -p "${out}/${id}"
        build_once "${id}" 1
        printf 'built %s (negative control): exit %s\n' "${id}" "$(cat "${out}/${id}/run-1.exit")"
    done

    args=(-slice.out="${root}/${out}" -slice.refs="${root}/{{SLICE_REF_DIR}}")
    for check in ${checks}; do
        case "${check}" in
            compare) args+=(-slice.compare="${ids}") ;;
            controls) args+=(-slice.unparseable="${unparseable}" -slice.unloadable="${unloadable}") ;;
            states) args+=(-slice.states="$(for id in {{SLICE_1_IDS}}; do printf '%s ' "${root}/{{SLICE_ARXIV_OUT_DIR}}/stage-1/${id}"; done; for id in ${ids}; do printf '%s ' "${root}/${out}/${id}"; done)") ;;
            census) args+=(-slice.census="${ids}") ;;
            rollup) ;;
            *) printf 'error: unknown check %s\n' "${check}" >&2; exit 1 ;;
        esac
    done
    set +e
    go test -count=1 -v -run '^TestSlice' ./internal/docgraph -args "${args[@]}"
    verdict=$?
    set -e
    invocation="go test -count=1 -v -run '^TestSlice' ./internal/docgraph -args ${args[*]}"
    # The Records check — records against kb_tools' inventory over kbase's
    # tree — is the claim graph's, read through its own inventory; it writes
    # <id>/records.yaml.
    if [[ "${checks}" == *" compare "* ]]; then
        set +e
        go test -count=1 -v -run '^TestSliceRecords$' ./internal/claimgraph -args -slice.out="${root}/${out}" -slice.compare="${ids}"
        [[ $? -eq 0 ]] || verdict=1
        set -e
        invocation="${invocation}\ngo test -count=1 -v -run '^TestSliceRecords\$' ./internal/claimgraph -args -slice.out=${root}/${out} -slice.compare='${ids}'"
    fi
    if [[ "${checks}" == *" rollup "* ]]; then
        rollup=(-slice.out="${root}/${out}" -slice.rollup="${ids}")
        go test -count=1 -v -run '^TestRollup$' ./internal/docgraph -args "${rollup[@]}"
        invocation="${invocation}\ngo test -count=1 -v -run '^TestRollup\$' ./internal/docgraph -args ${rollup[*]}"
    fi

    {{just_executable()}} _write-evidence "${out}" "{{recipe}}" \
      "per paper, in fresh fixture repositories ${out}/<id>/run-<n>:\n(cd <fixture> && ${root}/{{BIN_DIR}}/kbase build <volume-root> --through document-graph --state-dir ${root}/${out}/<id>/state-<n> [--bibliography <each .bib beside the volume root, sorted>]) > <id>/run-<n>.result.yaml 2> <id>/run-<n>.log\nPYTHONPATH=<kb_tools agents> python3 tools/slice/kbtools_dump.py [--ref <id>={{SLICE_REF_DIR}}/<id>/kb-root] --kbase <id>=<id>/run-1/kb-root --out <id>/dump.json\n${invocation}"
    printf '\nReference: adjagent %s; %s\n' "${adjagent}" "${pandoc_version}" >> "${out}/EVIDENCE.md"
    if [[ -f "${out}/rollup.yaml" ]]; then
        printf '\n## Roll-up (rollup.yaml)\n\n```yaml\n' >> "${out}/EVIDENCE.md"
        cat "${out}/rollup.yaml" >> "${out}/EVIDENCE.md"
        printf '```\n' >> "${out}/EVIDENCE.md"
    fi
    if [[ "${verdict}" -ne 0 ]]; then
        printf 'integration(%s): a check failed — see each paper'"'"'s comparison.yaml\n' "{{recipe}}"
        exit 1
    fi
    printf 'integration(%s) ok\n' "{{recipe}}"

# kbase's refresh and verify against kb_tools-built KBs. Per fixture:
# ./bin/kbase verify runs on the fixture itself, which it must leave untouched
# and pass; then a git clone of the fixture is refreshed with ./bin/kbase
# refresh and `git diff --exit-code` over its kb-root/ measures byte equality
# with kb_tools' refresh. Every difference is a finding in comparison.yaml.
# Then TestRefreshMatchesKbTools and TestVerifyAgreesWithKbTools run each
# toolchain over the same inputs — the fixtures as built, with their derived
# fields stale and with their pending scores valued, and kb_tools' mini-kb
# with the mutations of kb_tools' own verifier tests — into
# kbtools-agreement/.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): ./bin/kbase verify on each kb_tools-built KB of prep-test-integration-kbtools-full, ./bin/kbase refresh in a clone of it diffed against kb_tools' bytes, and kbase's refresh and verify gates run beside kb_tools' on the same inputs (mini-kb and its mutations included); evidence under test_data/transient/test-integration-verify-arxiv/")]
test-integration-verify-arxiv: (prep-test-integration-kbtools-full VERIFY_ARXIV_IDS)
    @mkdir -p "{{VERIFY_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _test-integration-verify-arxiv 2>&1 | tee "{{VERIFY_ARXIV_OUT_DIR}}/log.txt"

[private]
_test-integration-verify-arxiv: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{VERIFY_ARXIV_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    printf 'adjagent: %s\n' "${adjagent}"
    status=0
    comparison="${out}/comparison.yaml"
    : > "${comparison}"
    for id in {{VERIFY_ARXIV_IDS}}; do
        name="$(printf '%s' "${id}" | tr '/' '_')"
        fixture="{{KBTOOLS_REF_DIR}}/${name}"
        dir="${out}/${name}"
        rm -rf "${dir}"
        mkdir -p "${dir}"
        findings=()
        verify_exit=none
        refresh_exit=none
        if [[ -z "$(git -C "${fixture}" log -n1 --format=%H --grep='^kb-build: overview-drafted ' 2>/dev/null)" ]]; then
            findings+=("${fixture} holds no kb-build: overview-drafted commit")
        else
            set +e
            ( cd "${fixture}" && "${kbase}" verify ) > "${dir}/verify.result.yaml" 2> "${dir}/verify.log"
            verify_exit=$?
            set -e
            if [[ "${verify_exit}" -ne 0 ]]; then
                findings+=("kbase verify exited ${verify_exit} on the kb_tools-built KB")
            fi
            while IFS= read -r line; do
                findings+=("kbase verify left the fixture changed: ${line}")
            done < <(git -C "${fixture}" status --porcelain)
            git clone -q "${fixture}" "${dir}/clone"
            set +e
            ( cd "${dir}/clone" && "${kbase}" refresh ) > "${dir}/refresh.result.yaml" 2> "${dir}/refresh.log"
            refresh_exit=$?
            git -C "${dir}/clone" diff --exit-code -- kb-root > "${dir}/refresh.diff"
            set -e
            if [[ "${refresh_exit}" -ne 0 ]]; then
                findings+=("kbase refresh exited ${refresh_exit} in the clone")
            fi
            while IFS= read -r path; do
                findings+=("kbase refresh changed ${path}")
            done < <(git -C "${dir}/clone" diff --name-only -- kb-root ':(exclude,glob)**/claim-graph*.svg'; git -C "${dir}/clone" ls-files --others --exclude-standard -- kb-root ':(exclude,glob)**/claim-graph*.svg')
        fi
        printf -- '- id: "%s"\n  verify-exit: %s\n  refresh-exit: %s\n  findings:' "${id}" "${verify_exit}" "${refresh_exit}" >> "${comparison}"
        if [[ "${#findings[@]}" -eq 0 ]]; then
            printf ' []\n' >> "${comparison}"
        else
            printf '\n' >> "${comparison}"
            for f in "${findings[@]}"; do
                printf '    - "%s"\n' "${f//\"/\\\"}" >> "${comparison}"
            done
            status=1
        fi
        printf '%s: verify exit %s, refresh exit %s, %s finding(s)\n' "${id}" "${verify_exit}" "${refresh_exit}" "${#findings[@]}"
    done

    # Each toolchain over the same inputs, staged by TestStageKbToolsComparisons
    # and compared by TestKbToolsComparisons. kb_tools runs from the adjagent
    # clone, mini-kb materialized by its own refresh as its tests do.
    agree="${out}/kbtools-agreement"
    rm -rf "${agree}"
    mkdir -p "${agree}/mini-kb"
    cp -R .claude/adjagent/kb_tools/tests/fixtures/mini-kb "${agree}/mini-kb/kb-root"
    kbtools() { PYTHONPATH="${root}/.claude/adjagent" PYTHONDONTWRITEBYTECODE=1 python3 -m "$@"; }
    kbtools kb_tools.refresh_kb_metadata --kb-root "${agree}/mini-kb/kb-root" > "${agree}/mini-kb/materialize.log" 2>&1
    go test -count=1 -v -run '^TestStageKbToolsComparisons$' ./internal/index -args -kbtools.stage \
        -kbtools.out="${root}/${agree}" -kbtools.minikb="${root}/.claude/adjagent/kb_tools/tests/fixtures/mini-kb" \
        -kbtools.materialized="${root}/${agree}/mini-kb/kb-root" -kbtools.fixtures="${root}/{{KBTOOLS_REF_DIR}}" \
        -kbtools.ids="{{VERIFY_ARXIV_IDS}}"
    # step <dir> <name> <command...>: run a command in dir, its output in
    # <dir>/<name>.log and its exit code in <dir>/<name>.exit.
    step() {
        local dir="$1" name="$2"
        shift 2
        set +e
        ( cd "${dir}" && "$@" ) > "${dir}/${name}.log" 2>&1
        printf '%s\n' "$?" > "${dir}/${name}.exit"
        set -e
    }
    while read -r case; do
        d="${agree}/refresh/${case}"
        step "${d}" kbtools-refresh kbtools kb_tools.refresh_kb_metadata --kb-root kbtools/kb-root
        git init -q "${d}/kbase"
        step "${d}/kbase" ../kbase-refresh "${kbase}" refresh
    done < "${agree}/refresh/cases.txt"
    while read -r variant after; do
        d="${agree}/verify/${variant}"
        git init -q "${d}"
        case "${after}" in
            kbtools-refresh) step "${d}" kbtools-after kbtools kb_tools.refresh_kb_metadata --kb-root kb-root ;;
            kbase-refresh) step "${d}" kbase-after "${kbase}" refresh ;;
        esac
        step "${d}" kbtools-links kbtools kb_tools.verify_md_links --root .
        step "${d}" kbtools-metadata kbtools kb_tools.verify_kb_metadata --kb-root kb-root
        set +e
        ( cd "${d}" && "${kbase}" verify ) > "${d}/kbase-verify.yaml" 2> "${d}/kbase-verify.log"
        set -e
    done < "${agree}/verify/variants.txt"
    set +e
    go test -count=1 -v -run '^TestKbToolsComparisons$' ./internal/index -args -kbtools.out="${root}/${agree}"
    [[ $? -eq 0 ]] || status=1
    set -e

    {{just_executable()}} _write-evidence "${out}" "test-integration-verify-arxiv" \
      "per fixture <id> of {{KBTOOLS_REF_DIR}}:\n(cd {{KBTOOLS_REF_DIR}}/<id> && ${kbase} verify) > <id>/verify.result.yaml 2> <id>/verify.log\ngit clone {{KBTOOLS_REF_DIR}}/<id> <id>/clone\n(cd <id>/clone && ${kbase} refresh) > <id>/refresh.result.yaml 2> <id>/refresh.log\ngit -C <id>/clone diff --exit-code -- kb-root > <id>/refresh.diff\nkbtools-agreement/: go test -run '^TestStageKbToolsComparisons\$' ./internal/index -args -kbtools.stage ...; per refresh case, kb_tools' refresh on kbtools/kb-root and ${kbase} refresh in kbase/; per verify variant, kb_tools' verify_md_links and verify_kb_metadata and ${kbase} verify; go test -run '^TestKbToolsComparisons\$' ./internal/index -args -kbtools.out=${agree}"
    printf '\nReference: adjagent %s\n' "${adjagent}" >> "${out}/EVIDENCE.md"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-verify-arxiv): a finding — see comparison.yaml and kbtools-agreement/\n'
        exit 1
    fi
    printf 'integration(test-integration-verify-arxiv) ok\n'

# The compatibility op script (COMPAT_OP_SCRIPT, rendered per fixture by
# TestStageOpScript) applied to two clones of each kb_tools-built fixture:
# through ./bin/kbase on kbase/, through kb_tools' kb_util on kbtools/, each
# step's minted ids substituted from that toolchain's own output. kb_tools'
# ops do not refresh, so kbtools/ is refreshed once after the script. Both
# toolchains' verify then run on both clones, each of which must exit 0.
# TestCompareOpScript compares the two kb-roots with kb_tools' ids read
# as kbase's, and the script is re-run on kbase/, which must report
# unchanged at every step and leave kb-root/ byte-identical.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): per kb_tools-built fixture of prep-test-integration-kbtools-full, the compatibility op script through ./bin/kbase on one clone and through kb_tools' kb_util on another, both toolchains' verify on both clones, the two kb-root/ compared, and the script re-run through ./bin/kbase to unchanged; kb_tools' render goldens and write-API regression replay run first; evidence under test_data/transient/test-integration-write-arxiv/")]
test-integration-write-arxiv: (prep-test-integration-kbtools-full VERIFY_ARXIV_IDS)
    @mkdir -p "{{WRITE_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _test-integration-write-arxiv 2>&1 | tee "{{WRITE_ARXIV_OUT_DIR}}/log.txt"

[private]
_test-integration-write-arxiv: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{WRITE_ARXIV_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    script="${root}/{{COMPAT_OP_SCRIPT}}"
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    printf 'adjagent: %s\n' "${adjagent}"
    status=0

    # kb_tools' render goldens and write-API regression fixtures: the clone is
    # present here, so a skip is a failure.
    set +e
    go test -count=1 -v -run '^(TestRenderGoldens|TestRegression.*)$' ./internal/write > "${out}/kbtools-fixtures.log" 2>&1
    rc=$?
    set -e
    if [[ "${rc}" -ne 0 ]] || grep -q -- '--- SKIP' "${out}/kbtools-fixtures.log"; then
        printf 'render goldens or regression replay failed or skipped: see kbtools-fixtures.log\n'
        status=1
    fi

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
    kbtools() { PYTHONPATH=.claude/agents PYTHONDONTWRITEBYTECODE=1 "$@"; }

    summary="${out}/summary.yaml"
    : > "${summary}"
    for id in {{VERIFY_ARXIV_IDS}}; do
        name="$(printf '%s' "${id}" | tr '/' '_')"
        fixture="{{KBTOOLS_REF_DIR}}/${name}"
        dir="${out}/${name}"
        rm -rf "${dir}"
        mkdir -p "${dir}/kbase-steps" "${dir}/kbtools-steps" "${dir}/rerun-steps"
        findings=()
        git clone -q "${fixture}" "${dir}/kbase"
        git clone -q "${fixture}" "${dir}/kbtools"
        if ! go test -count=1 -run '^TestStageOpScript$' ./internal/write -args -opscript.script="${script}" \
            -opscript.kbroot="${root}/${dir}/kbase/kb-root" -opscript.out="${root}/${dir}/steps" > "${dir}/stage.log" 2>&1; then
            findings+=("the op script could not be staged: see stage.log")
        else
            kb_subs=(-e 's/^//')
            tl_subs=(-e 's/^//')
            : > "${dir}/ids.txt"
            while IFS=$'\t' read -r nn op create minted; do
                flags=()
                [[ "${create}" == true ]] && flags+=(--create)
                step="${nn}-${op}"
                sed "${kb_subs[@]}" "${dir}/steps/${step}.yaml" > "${dir}/kbase-steps/${step}.yaml"
                capture "${dir}/kbase-steps/${step}" "${dir}/kbase" "${kbase}" "${op}" --values "${root}/${dir}/kbase-steps/${step}.yaml" ${flags[@]+"${flags[@]}"}
                [[ "$(cat "${dir}/kbase-steps/${step}.exit")" -eq 0 ]] || findings+=("kbase ${op} (step ${nn}) exited $(cat "${dir}/kbase-steps/${step}.exit")")
                sed "${tl_subs[@]}" "${dir}/steps/${step}.toml" > "${dir}/kbtools-steps/${step}.toml"
                capture "${dir}/kbtools-steps/${step}" "${dir}/kbtools" kbtools python3 -m kb_tools.kb_util "${op}" --values "${root}/${dir}/kbtools-steps/${step}.toml" ${flags[@]+"${flags[@]}"}
                [[ "$(cat "${dir}/kbtools-steps/${step}.exit")" -eq 0 ]] || findings+=("kb_tools ${op} (step ${nn}) exited $(cat "${dir}/kbtools-steps/${step}.exit")")
                if [[ "${minted}" != - ]]; then
                    kid="$(sed -n '/^ids:/{n;s/^ *- "\(.*\)"$/\1/p;}' "${dir}/kbase-steps/${step}.out")"
                    tid="$(sed -n 's/.*FACT minted  *\([^ ]*\) (.*/\1/p' "${dir}/kbtools-steps/${step}.out" | head -n1)"
                    [[ -n "${kid}" && -n "${tid}" ]] || findings+=("step ${nn} reported no minted id on one side (kbase '${kid}', kb_tools '${tid}')")
                    kb_subs+=(-e "s/@MINT-${minted}@/${kid}/g")
                    tl_subs+=(-e "s/@MINT-${minted}@/${tid}/g")
                    printf '%s %s %s\n' "${minted}" "${kid}" "${tid}" >> "${dir}/ids.txt"
                fi
            done < "${dir}/steps/steps.tsv"

            capture "${dir}/kbtools-refresh" "${dir}/kbtools" kbtools make kb-refresh
            [[ "$(cat "${dir}/kbtools-refresh.exit")" -eq 0 ]] || findings+=("kb_tools' refresh exited $(cat "${dir}/kbtools-refresh.exit") on kbtools/")
            for clone in kbase kbtools; do
                capture "${dir}/kbase-verify.${clone}" "${dir}/${clone}" "${kbase}" verify
                [[ "$(cat "${dir}/kbase-verify.${clone}.exit")" -eq 0 ]] || findings+=("kbase verify exited $(cat "${dir}/kbase-verify.${clone}.exit") on ${clone}/")
                capture "${dir}/kbtools-verify.${clone}" "${dir}/${clone}" kbtools make kb-verify
                rc="$(cat "${dir}/kbtools-verify.${clone}.exit")"
                [[ "${rc}" -eq 0 ]] || findings+=("kb_tools' verify exited ${rc} on ${clone}/")
            done

            set +e
            diff -r "${dir}/kbtools/kb-root" "${dir}/kbase/kb-root" > "${dir}/raw.diff"
            go test -count=1 -v -run '^TestCompareOpScript$' ./internal/write -args -opscript.kbase="${root}/${dir}/kbase/kb-root" \
                -opscript.kbtools="${root}/${dir}/kbtools/kb-root" -opscript.ids="${root}/${dir}/ids.txt" \
                -opscript.comparison="${root}/${dir}/comparison.yaml" > "${dir}/compare.log" 2>&1
            rc=$?
            set -e
            [[ "${rc}" -eq 0 ]] || findings+=("the two kb-roots differ beyond the sheet: see comparison.yaml")

            cp -R "${dir}/kbase/kb-root" "${dir}/kb-root.before-rerun"
            while IFS=$'\t' read -r nn op create minted; do
                flags=()
                [[ "${create}" == true ]] && flags+=(--create)
                step="${nn}-${op}"
                capture "${dir}/rerun-steps/${step}" "${dir}/kbase" "${kbase}" "${op}" --values "${root}/${dir}/kbase-steps/${step}.yaml" ${flags[@]+"${flags[@]}"}
                grep -qx 'outcome: "unchanged"' "${dir}/rerun-steps/${step}.out" || findings+=("re-run ${op} (step ${nn}) did not report unchanged")
            done < "${dir}/steps/steps.tsv"
            diff -r "${dir}/kb-root.before-rerun" "${dir}/kbase/kb-root" > "${dir}/rerun.diff" || findings+=("the re-run changed kb-root/: see rerun.diff")
        fi

        printf -- '- id: "%s"\n  findings:' "${id}" >> "${summary}"
        if [[ "${#findings[@]}" -eq 0 ]]; then
            printf ' []\n' >> "${summary}"
        else
            printf '\n' >> "${summary}"
            for f in "${findings[@]}"; do
                printf '    - "%s"\n' "${f//\"/\\\"}" >> "${summary}"
            done
            status=1
        fi
        printf '%s: %s finding(s)\n' "${id}" "${#findings[@]}"
    done

    {{just_executable()}} _write-evidence "${out}" "test-integration-write-arxiv" \
      "go test -run '^(TestRenderGoldens|TestRegression.*)\$' ./internal/write > kbtools-fixtures.log\nper fixture <id> of {{KBTOOLS_REF_DIR}}, cloned to <id>/kbase and <id>/kbtools:\ngo test -run '^TestStageOpScript\$' ./internal/write -args -opscript.script={{COMPAT_OP_SCRIPT}} -opscript.kbroot=<id>/kbase/kb-root -opscript.out=<id>/steps\nper step NN-op: (cd <id>/kbase && ${kbase} op --values <id>/kbase-steps/NN-op.yaml); (cd <id>/kbtools && PYTHONPATH=.claude/agents python3 -m kb_tools.kb_util op --values <id>/kbtools-steps/NN-op.toml)\n(cd <id>/kbtools && make kb-refresh)\nper clone: ${kbase} verify; make kb-verify\ngo test -run '^TestCompareOpScript\$' ./internal/write -args ... -opscript.comparison=<id>/comparison.yaml\nper step again on <id>/kbase into <id>/rerun-steps/, then diff -r <id>/kb-root.before-rerun <id>/kbase/kb-root > <id>/rerun.diff"
    printf '\nReference: adjagent %s\n' "${adjagent}" >> "${out}/EVIDENCE.md"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-write-arxiv): a finding — see summary.yaml\n'
        exit 1
    fi
    printf 'integration(test-integration-write-arxiv) ok\n'

# Every query through ./bin/kbase and through kb_tools' kb_cmd --json, the
# fixture's own installed copy, run from the same repository with the same
# arguments: on each kb_tools-built fixture, read-only, every case
# TestStageQueryCases draws from its index; and, because those fixtures score
# nothing and link no leaf from another's body, again on a clone given one
# such link and scored by kbase (TestStageQueryScores' values through
# set-rigor and insert-claim-entry), where referenced-by, solidity-below,
# weak-points and gated-on have something to answer. TestCompareQueries compares every
# case as data into comparison.yaml. render-claim-graph is exercised on a
# third clone: kb_tools' drawn sheet left byte-identical (outcome unchanged), the
# re-run unchanged, a removed root sheet drawn again (a Graphviz SVG, not the
# placeholder), and kbase verify green after.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): every query through ./bin/kbase and through kb_tools' kb_cmd --json on each kb_tools-built fixture of prep-test-integration-kbtools-full (read-only) and on a clone kbase scored, compared as data; render-claim-graph's sheet rule on a clone (kb_tools' sheet left byte-identical with outcome unchanged, unchanged on re-run, a removed root sheet drawn again); evidence under test_data/transient/test-integration-query-arxiv/")]
test-integration-query-arxiv: (prep-test-integration-kbtools-full VERIFY_ARXIV_IDS)
    @mkdir -p "{{QUERY_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _test-integration-query-arxiv 2>&1 | tee "{{QUERY_ARXIV_OUT_DIR}}/log.txt"

[private]
_test-integration-query-arxiv: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{QUERY_ARXIV_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    printf 'adjagent: %s\n' "${adjagent}"
    status=0

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
    # run_cases <cases-dir> <repo>: each NNN.args (the query, then one argument
    # per line) through both toolchains from the repository; a list query
    # through kbase with --limit 0, so its whole answer is compared.
    run_cases() {
        local cases="$1" repo="$2" f stem line
        local -a args paging
        for f in "${cases}"/*.args; do
            stem="${f%.args}"
            args=()
            while IFS= read -r line; do
                args+=("${line}")
            done < "${f}"
            paging=(--limit 0)
            case "${args[0]}" in show|stats) paging=() ;; esac
            capture "${stem}.kbase" "${repo}" "${kbase}" "${args[@]}" ${paging[@]+"${paging[@]}"}
            capture "${stem}.kbcmd" "${repo}" env PYTHONPATH=.claude/agents PYTHONDONTWRITEBYTECODE=1 \
                python3 -m kb_tools.kb_cmd "${args[@]}" --json
        done
    }
    # expect <stem> <exit> <outcome>: a finding unless the captured run exited
    # so and reported that outcome.
    expect() {
        local stem="$1" want_exit="$2" want_outcome="$3"
        local got
        got="$(cat "${stem}.exit")"
        if [[ "${got}" -ne "${want_exit}" ]] || ! grep -qx "outcome: \"${want_outcome}\"" "${stem}.out"; then
            findings+=("$(basename "${stem}"): exit ${got}, want ${want_exit} with outcome ${want_outcome}")
        fi
    }

    dirs=()
    findings_file="${out}/findings.yaml"
    : > "${findings_file}"
    for id in {{VERIFY_ARXIV_IDS}}; do
        name="$(printf '%s' "${id}" | tr '/' '_')"
        fixture="{{KBTOOLS_REF_DIR}}/${name}"
        dir="${out}/${name}"
        rm -rf "${dir}"
        mkdir -p "${dir}"
        findings=()
        if [[ -z "$(git -C "${fixture}" log -n1 --format=%H --grep='^kb-build: overview-drafted ' 2>/dev/null)" ]]; then
            findings+=("${fixture} holds no kb-build: overview-drafted commit")
        else
            # The fixture itself, read-only.
            if go test -count=1 -run '^TestStageQueryCases$' ./internal/query -args -query.kbroot="${root}/${fixture}/kb-root" \
                -query.cases="${root}/${dir}/fixture" > "${dir}/stage.log" 2>&1; then
                run_cases "${dir}/fixture" "${fixture}"
                dirs+=("${root}/${dir}/fixture")
            else
                findings+=("the fixture's cases could not be staged: see stage.log")
            fi
            while IFS= read -r line; do
                findings+=("the queries left the fixture changed: ${line}")
            done < <(git -C "${fixture}" status --porcelain)

            # A clone kbase scores, where the scored queries have answers.
            git clone -q "${fixture}" "${dir}/scored-clone"
            if go test -count=1 -run '^TestStageQueryScores$' ./internal/query -args -query.kbroot="${root}/${dir}/scored-clone/kb-root" \
                -query.values="${root}/${dir}/scores" >> "${dir}/stage.log" 2>&1; then
                if [[ -f "${dir}/scores/set-rigor.yaml" ]]; then
                    for op in set-rigor insert-claim-entry; do
                        capture "${dir}/scores/${op}" "${dir}/scored-clone" "${kbase}" "${op}" --values "${root}/${dir}/scores/${op}.yaml"
                        expect "${dir}/scores/${op}" 0 done
                    done
                    if go test -count=1 -run '^TestStageQueryCases$' ./internal/query -args -query.kbroot="${root}/${dir}/scored-clone/kb-root" \
                        -query.cases="${root}/${dir}/scored" >> "${dir}/stage.log" 2>&1; then
                        run_cases "${dir}/scored" "${dir}/scored-clone"
                        dirs+=("${root}/${dir}/scored")
                    else
                        findings+=("the scored clone's cases could not be staged: see stage.log")
                    fi
                fi
            else
                findings+=("the scores could not be staged: see stage.log")
            fi

            # render-claim-graph's sheet rule on a clone.
            sheet="${dir}/sheet"
            mkdir -p "${sheet}"
            git clone -q "${fixture}" "${sheet}/clone"
            svg="${sheet}/clone/kb-root/claim-graph.svg"
            if [[ -f "${svg}" ]]; then
                cp "${svg}" "${sheet}/drawn.svg"
                capture "${sheet}/1-drawn" "${sheet}/clone" "${kbase}" render-claim-graph
                expect "${sheet}/1-drawn" 0 unchanged
                cmp -s "${sheet}/drawn.svg" "${svg}" || findings+=("render-claim-graph changed the bytes of kb_tools' drawn sheet, want it byte-identical")
                capture "${sheet}/2-redrawn" "${sheet}/clone" "${kbase}" render-claim-graph
                expect "${sheet}/2-redrawn" 0 unchanged
                rm "${svg}"
            fi
            capture "${sheet}/3-absent" "${sheet}/clone" "${kbase}" render-claim-graph
            expect "${sheet}/3-absent" 0 done
            if [[ ! -f "${svg}" ]]; then
                findings+=("render-claim-graph wrote no root sheet where none was")
            elif grep -q '>NYI</text>' "${svg}" || ! grep -q '<svg' "${svg}"; then
                findings+=("render-claim-graph wrote a placeholder, not a Graphviz sheet, with dot on PATH")
            fi
            capture "${sheet}/4-verify" "${sheet}/clone" "${kbase}" verify
            expect "${sheet}/4-verify" 0 done
        fi

        printf -- '- id: "%s"\n  findings:' "${id}" >> "${findings_file}"
        if [[ "${#findings[@]}" -eq 0 ]]; then
            printf ' []\n' >> "${findings_file}"
        else
            printf '\n' >> "${findings_file}"
            for f in "${findings[@]}"; do
                printf '    - "%s"\n' "${f//\"/\\\"}" >> "${findings_file}"
            done
            status=1
        fi
        printf '%s: %s finding(s)\n' "${id}" "${#findings[@]}"
    done

    set +e
    go test -count=1 -v -run '^TestCompareQueries$' ./internal/query -args -query.dirs="${dirs[*]+"${dirs[*]}"}" \
        -query.comparison="${root}/${out}/comparison.yaml" > "${out}/compare.log" 2>&1
    rc=$?
    set -e
    if [[ "${rc}" -ne 0 ]] || grep -q -- '--- SKIP' "${out}/compare.log"; then
        status=1
    fi

    # Every list query at its default limit, over each fixture, within the
    # tool-result cap.
    fixtures=()
    for id in {{VERIFY_ARXIV_IDS}}; do
        fixtures+=("${root}/{{KBTOOLS_REF_DIR}}/$(printf '%s' "${id}" | tr '/' '_')")
    done
    set +e
    go test -count=1 -v -run '^TestDefaultLimitFitsTheToolResultCap$' ./cmd -args -query.fixtures="${fixtures[*]}" > "${out}/limit.log" 2>&1
    rc=$?
    set -e
    if [[ "${rc}" -ne 0 ]] || grep -q -- '--- SKIP' "${out}/limit.log"; then
        status=1
    fi

    {{just_executable()}} _write-evidence "${out}" "test-integration-query-arxiv" \
      "per fixture <id> of {{KBTOOLS_REF_DIR}}:\ngo test -run '^TestStageQueryCases\$' ./internal/query -args -query.kbroot={{KBTOOLS_REF_DIR}}/<id>/kb-root -query.cases=<id>/fixture\nper case NNN.args, from {{KBTOOLS_REF_DIR}}/<id>: ${kbase} <args> [--limit 0 on a list query] and PYTHONPATH=.claude/agents python3 -m kb_tools.kb_cmd <args> --json\ngit clone {{KBTOOLS_REF_DIR}}/<id> <id>/scored-clone; go test -run '^TestStageQueryScores\$' ./internal/query -args -query.kbroot=<id>/scored-clone/kb-root -query.values=<id>/scores\n(cd <id>/scored-clone && ${kbase} set-rigor --values <id>/scores/set-rigor.yaml && ${kbase} insert-claim-entry --values <id>/scores/insert-claim-entry.yaml), then the cases again into <id>/scored\ngit clone {{KBTOOLS_REF_DIR}}/<id> <id>/sheet/clone; ${kbase} render-claim-graph over the drawn sheet (bytes change), again (unchanged), and with the root sheet removed (drawn again); ${kbase} verify\ngo test -run '^TestCompareQueries\$' ./internal/query -args -query.dirs='<every case dir>' -query.comparison=comparison.yaml > compare.log\ngo test -run '^TestDefaultLimitFitsTheToolResultCap\$' ./cmd -args -query.fixtures='<every fixture repository>' > limit.log"
    printf '\nReference: adjagent %s\n' "${adjagent}" >> "${out}/EVIDENCE.md"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-query-arxiv): a finding — see findings.yaml and comparison.yaml\n'
        exit 1
    fi
    printf 'integration(test-integration-query-arxiv) ok\n'

# kbase's claim graph against kb_tools' over Slice-2's papers. Per paper: a
# fresh fixture repository from the arXiv sources, built by ./bin/kbase
# --through depends-attributed --no-inference with every .bib beside the
# volume root, sorted, as kb_tools' driver passes them; kb_tools' tree and
# build records read out of the kb_tools-built fixture's depends-attributed
# commit with git archive; ./bin/kbase refresh and verify on kbase's tree; and
# kb_tools' own make kb-verify on a clone of that fixture whose kb-root/ is
# replaced by kbase's, which must exit 0. A second build in a second fresh repository must equal the first
# modulo node ids. TestCompareKbTools compares the two toolchains, and the two
# kbase builds, as data by title and host — the three build records included,
# the unmarked one as the declared pass left it — and the two ledgers to
# depends-attributed (the stages recorded and the rows each dropped), kbase
# status's stages against kb_tools' vocabulary and ledger, and
# depends-attributed's candidate counts against kb_tools' report, and writes
# every difference, and every run's exit, to comparison.yaml.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): per Slice-2 paper, ./bin/kbase build --through depends-attributed --no-inference twice in fresh fixture repositories, the two equal modulo node ids, compared by title and host — the build records byte for byte, kbase's ids renamed to kb_tools' — against the kb_tools-built fixture's depends-attributed commit (prep-test-integration-kbtools-full) with its ledger, stage vocabulary and candidate counts, with kbase's refresh, verify and status and kb_tools' make kb-verify over kbase's tree; evidence under test_data/transient/test-integration-claimgraph-arxiv/")]
test-integration-claimgraph-arxiv: (prep-test-integration-arxiv SLICE_2_IDS) (prep-test-integration-kbtools-full SLICE_2_IDS)
    @mkdir -p "{{CLAIMGRAPH_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _test-integration-claimgraph-arxiv 2>&1 | tee "{{CLAIMGRAPH_ARXIV_OUT_DIR}}/log.txt"

[private]
_test-integration-claimgraph-arxiv: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{CLAIMGRAPH_ARXIV_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    pandoc_version="$(pandoc --version | head -n 1)"
    printf 'adjagent: %s\n%s\n' "${adjagent}" "${pandoc_version}"
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
    kbtools() { PYTHONPATH=.claude/agents PYTHONDONTWRITEBYTECODE=1 "$@"; }
    status=0
    for id in {{SLICE_2_IDS}}; do
        name="$(printf '%s' "${id}" | tr '/' '_')"
        fixture="{{KBTOOLS_REF_DIR}}/${name}"
        dir="${out}/${name}"
        rm -rf "${dir}"
        mkdir -p "${dir}/repo" "${dir}/repo-2" "${dir}/ref" "${dir}/checks"
        cp -R "{{ARXIV_DIR}}/${name}/." "${dir}/repo/"
        cp -R "{{ARXIV_DIR}}/${name}/." "${dir}/repo-2/"
        git -C "${dir}/repo" init -q
        git -C "${dir}/repo-2" init -q
        volume_root="$({{just_executable()}} _arxiv-volume-root "${dir}/repo")"
        (cd "${dir}/repo" && find "$(dirname "${volume_root}")" -maxdepth 1 -name '*.bib' | sed 's|^\./||' | LC_ALL=C sort) > "${dir}/bibliographies.txt"
        bibs=()
        while IFS= read -r bib; do
            bibs+=(--bibliography "${bib}")
        done < "${dir}/bibliographies.txt"
        capture "${dir}/checks/kbase-build" "${dir}/repo" "${kbase}" build "${volume_root}" \
            --through depends-attributed --no-inference --state-dir "${root}/${dir}/state" ${bibs[@]+"${bibs[@]}"}
        capture "${dir}/checks/kbase-build-2" "${dir}/repo-2" "${kbase}" build "${volume_root}" \
            --through depends-attributed --no-inference --state-dir "${root}/${dir}/state-2" ${bibs[@]+"${bibs[@]}"}

        commit="$(git -C "${fixture}" log -n1 --format=%H --grep='^kb-build: depends-attributed ')"
        printf '%s\n' "${commit}" > "${dir}/ref/commit.txt"
        git -C "${fixture}" archive "${commit}" kb-root kb-build-node-pass.yaml kb-build-classification.yaml kb-build-unmarked.yaml | tar -x -C "${dir}/ref"
        # Each side's boundary commits to depends-attributed, oldest first,
        # NUL-separated; kb_tools' stage vocabulary; and the report kb_tools'
        # depends-attributed invocation gave its driver, from the newest run
        # log holding one.
        git -C "${fixture}" log --reverse -z --format='%s%n%b' --grep='^kb-build: ' "${commit}" > "${dir}/ref/ledger.txt"
        git -C "${dir}/repo" log --reverse -z --format='%s%n%b' --grep='^kb-build: ' > "${dir}/ledger.txt"
        (cd "${fixture}" && kbtools python3 -c 'from kb_tools import kb_pipeline; print("\n".join(kb_pipeline.STAGE_IDS))') > "${dir}/ref/stage-ids.txt"
        python3 -c '
    import json, pathlib, sys
    found = ""
    for log in sorted(pathlib.Path(sys.argv[1]).glob("*/run.log")):
        for line in log.read_text(encoding="utf-8").splitlines():
            try:
                entry = json.loads(line)
            except ValueError:
                continue
            context = entry.get("context") or {}
            if entry.get("message") == "front-end report" and context.get("op") == "kb_claimgraph --pass 2 --no-inference":
                found = context.get("report", "")
    print(found, end="")
    ' "{{KBTOOLS_REF_DIR}}/runs/${name}" > "${dir}/ref/depends-report.txt"
        capture "${dir}/status" "${dir}/repo" "${kbase}" status --state-dir "${root}/${dir}/state"

        capture "${dir}/checks/kbase-refresh" "${dir}/repo" "${kbase}" refresh
        capture "${dir}/checks/kbase-verify" "${dir}/repo" "${kbase}" verify
        git clone -q "${fixture}" "${dir}/kbtools-verify"
        rm -rf "${dir}/kbtools-verify/kb-root"
        cp -R "${dir}/repo/kb-root" "${dir}/kbtools-verify/kb-root"
        capture "${dir}/checks/kbtools-verify" "${dir}/kbtools-verify" kbtools make kb-verify

        set +e
        go test -count=1 -v -run '^TestCompareKbTools$' ./internal/claimgraph -args \
            -claimgraph.kbase="${root}/${dir}/repo" -claimgraph.rerun="${root}/${dir}/repo-2" -claimgraph.kbtools="${root}/${dir}/ref" \
            -claimgraph.checks="${root}/${dir}/checks" -claimgraph.comparison="${root}/${dir}/comparison.yaml" \
            -claimgraph.ledger="${root}/${dir}/ledger.txt" -claimgraph.status="${root}/${dir}/status.out" -claimgraph.state="${root}/${dir}/state" \
            > "${dir}/compare.log" 2>&1
        rc=$?
        set -e
        [[ "${rc}" -eq 0 ]] || status=1
        printf '%s: build exit %s, comparison %s — see %s\n' "${id}" "$(cat "${dir}/checks/kbase-build.exit")" \
            "$([[ "${rc}" -eq 0 ]] && echo passed || echo FAILED)" "${dir}/comparison.yaml"
    done

    {{just_executable()}} _write-evidence "${out}" "test-integration-claimgraph-arxiv" \
      "per paper <id> of {{SLICE_2_IDS}}, in a fresh fixture repository <id>/repo from {{ARXIV_DIR}}/<id>:\n(cd <id>/repo && ${kbase} build <volume-root> --through depends-attributed --no-inference --state-dir <id>/state [--bibliography <each .bib beside the volume root, sorted>]) > <id>/checks/kbase-build.out 2> <id>/checks/kbase-build.log\nthe same in a second fixture repository <id>/repo-2 with --state-dir <id>/state-2, into <id>/checks/kbase-build-2.*\ngit -C {{KBTOOLS_REF_DIR}}/<id> archive <depends-attributed commit> kb-root kb-build-node-pass.yaml kb-build-classification.yaml kb-build-unmarked.yaml | tar -x -C <id>/ref\ngit -C {{KBTOOLS_REF_DIR}}/<id> log --reverse -z --format='%s%n%b' --grep='^kb-build: ' <depends-attributed commit> > <id>/ref/ledger.txt; the same over <id>/repo > <id>/ledger.txt\n(cd {{KBTOOLS_REF_DIR}}/<id> && PYTHONPATH=.claude/agents python3 -c 'print kb_pipeline.STAGE_IDS') > <id>/ref/stage-ids.txt\nthe newest 'kb_claimgraph --pass 2 --no-inference' front-end report in {{KBTOOLS_REF_DIR}}/runs/<id>/*/run.log > <id>/ref/depends-report.txt\n(cd <id>/repo && ${kbase} status --state-dir <id>/state) into <id>/status.*\n(cd <id>/repo && ${kbase} refresh; ${kbase} verify) into <id>/checks/kbase-{refresh,verify}.*\ngit clone {{KBTOOLS_REF_DIR}}/<id> <id>/kbtools-verify, its kb-root/ replaced by <id>/repo/kb-root; (cd <id>/kbtools-verify && PYTHONPATH=.claude/agents make kb-verify) into <id>/checks/kbtools-verify.*\ngo test -run '^TestCompareKbTools\$' ./internal/claimgraph -args -claimgraph.kbase=<id>/repo -claimgraph.rerun=<id>/repo-2 -claimgraph.kbtools=<id>/ref -claimgraph.checks=<id>/checks -claimgraph.comparison=<id>/comparison.yaml -claimgraph.ledger=<id>/ledger.txt -claimgraph.status=<id>/status.out -claimgraph.state=<id>/state > <id>/compare.log"
    printf '\nReference: adjagent %s; %s\n' "${adjagent}" "${pandoc_version}" >> "${out}/EVIDENCE.md"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-claimgraph-arxiv): a difference — see each paper'"'"'s comparison.yaml\n'
        exit 1
    fi
    printf 'integration(test-integration-claimgraph-arxiv) ok\n'

# §8.3 Direction 1: a kbase-built KB judged by kb_tools. Per slice paper, a
# fresh fixture repository staged as the reference recipe stages one (sources,
# stub Makefile, .gitignore, the reference agent set installed from the
# adjagent clone) is built by ./bin/kbase --no-inference to completion; then
# kb_tools' install-targets, its make kb-verify (green before any refresh),
# make kb-refresh and kb-verify green, make kb-stats and the
# docent's kb_cmd deps/show/subtree, the compatibility op script through
# kb_util followed by refresh and verify green and ./bin/kbase verify green,
# and ./bin/kbase status parsed at the end state. The readiness documents are
# checked as built, and kbase's excerpts of the built tree against kb_tools'
# compose_excerpts. On BUILD_ARXIV_SPECIAL_ID: a build killed with SIGKILL
# while claims-declared's writes are in flight, its status read, then resumed
# to a tree and records equal to the uninterrupted build's modulo node ids
# (TestCompareKbTools with only a rerun named); a fresh
# build over that tree in a repository with no trail refused; and a charter
# given, CONVENTIONS.md authored before phase-3a and left as it is. Every
# result lands in comparison.yaml.
[doc("NEEDS pandoc + python3 + the adjagent clone (excluded from test-integration): §8.3 Direction 1 — per slice paper ./bin/kbase build --no-inference in a fresh fixture repository with kb_tools installed, judged by kb_tools' verify, refresh, stats, kb_cmd and the op script through kb_util, then ./bin/kbase verify and status; kill-and-resume, the double-run guard and stamping over an authored document on one paper; evidence under test_data/transient/test-integration-build-arxiv/")]
test-integration-build-arxiv: (prep-test-integration-arxiv VERIFY_ARXIV_IDS)
    @mkdir -p "{{BUILD_ARXIV_OUT_DIR}}"
    @{{just_executable()}} _test-integration-build-arxiv 2>&1 | tee "{{BUILD_ARXIV_OUT_DIR}}/log.txt"

[private]
_test-integration-build-arxiv: build
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(pwd)"
    out="{{BUILD_ARXIV_OUT_DIR}}"
    kbase="${root}/{{BIN_DIR}}/kbase"
    script="${root}/{{COMPAT_OP_SCRIPT}}"
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    pandoc_version="$(pandoc --version | head -n 1)"
    printf 'adjagent: %s\n%s\n' "${adjagent}" "${pandoc_version}"
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
    kbtools() { PYTHONPATH=.claude/agents PYTHONDONTWRITEBYTECODE=1 "$@"; }
    # stage <repo> <name> [agents]: a fresh fixture repository holding the
    # paper's sources, committed, with the reference agent set where asked.
    stage() {
        local repo="$1" name="$2" agents="${3:-}"
        mkdir -p "${repo}"
        cp -R "{{ARXIV_DIR}}/${name}/." "${repo}/"
        printf 'default:\n\t@echo %s\n\ntest:\n\t@echo tests passed.\n' "${name}" > "${repo}/Makefile"
        printf '%s\n' '.claude-temp/' '__pycache__/' > "${repo}/.gitignore"
        git -C "${repo}" init -q
        git -C "${repo}" add -A
        git -C "${repo}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "sources as fetched"
        if [[ -n "${agents}" ]]; then
            just --justfile .claude/adjagent/justfile install "${root}/${repo}" > "${repo}.install.log" 2>&1
            git -C "${repo}" add -A
            git -C "${repo}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "agent defs installed"
        fi
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
    for id in {{VERIFY_ARXIV_IDS}}; do
        name="$(printf '%s' "${id}" | tr '/' '_')"
        dir="${out}/${name}"
        repo="${dir}/repo"
        ck="${dir}/checks"
        rm -rf "${dir}" "${dir}".*
        mkdir -p "${ck}" "${dir}/config" "${dir}/built" "${dir}/kbtools-steps"
        checks=()
        printf '%s\n' "${id}"
        stage "${repo}" "${name}" agents
        volume_root="$({{just_executable()}} _arxiv-volume-root "${repo}")"
        bibs=()
        while IFS= read -r bib; do
            bibs+=(--bibliography "${bib}")
        done < <(cd "${repo}" && find "$(dirname "${volume_root}")" -maxdepth 1 -name '*.bib' | sed 's|^\./||' | LC_ALL=C sort)
        build_args=(build "${volume_root}" --no-inference --config-dir "${root}/${dir}/config" ${bibs[@]+"${bibs[@]}"})

        # 3. the build, to completion
        capture "${ck}/kbase-build" "${repo}" "${kbase}" "${build_args[@]}" --state-dir "${root}/${dir}/state"
        check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-build.out" && echo 0 || echo 1)" \
            "exit $(rc "${ck}/kbase-build"), $(grep -m1 '^outcome:' "${ck}/kbase-build.out" || echo 'no outcome')" "${ck}/kbase-build.out"
        cp -R "${repo}/kb-root" "${dir}/built/"
        cp "${repo}"/kb-build-*.yaml "${dir}/built/" 2>/dev/null || true
        check ledger "$(ok test "$(git -C "${repo}" log --format=%s --grep='^kb-build: ' | wc -l | tr -d ' ')" -eq 10)" \
            "$(git -C "${repo}" log --format=%s --grep='^kb-build: ' | wc -l | tr -d ' ') kb-build: commits, one per stage" "${repo}/.git"
        check ledger-scope "$(ok test -z "$(git -C "${repo}" status --porcelain -- kb-root kb-build-node-pass.yaml kb-build-classification.yaml kb-build-unmarked.yaml kb-build-charter.md)")" \
            "the owned paths are clean after the build" "${repo}"

        # readiness documents, as built
        check claude-redirect "$(ok cmp -s <(printf '@AGENTS.md\n') "${dir}/built/kb-root/CLAUDE.md")" "kb-root/CLAUDE.md is exactly @AGENTS.md" "${dir}/built/kb-root/CLAUDE.md"
        check scope-pin "$(ok grep -qF 'This build was given no charter' "${dir}/built/kb-root/AGENTS.md")" "AGENTS.md carries the scope pin (no charter given)" "${dir}/built/kb-root/AGENTS.md"
        check no-invariant-heading "$(ok test -z "$(grep -l '^### INVARIANT-' "${dir}"/built/kb-root/{AGENTS,CONVENTIONS,CLAUDE}.md 2>/dev/null)")" \
            "no stamped document holds a ### INVARIANT- heading" "${dir}/built/kb-root"
        check overview-absent "$(ok test ! -e "${dir}/built/kb-root/README.md")" "no README.md, as kb_tools' --no-inference build writes none" "${dir}/built/kb-root"

        # 4. kb_tools' runner targets
        capture "${ck}/install-targets" "${repo}" kbtools python3 -m kb_tools.kb_util install-targets
        check install-targets "$(rc "${ck}/install-targets")" "exit $(rc "${ck}/install-targets")" "${ck}/install-targets.out"

        # 5. kb_tools' verify before any refresh: green
        capture "${ck}/kbtools-verify-unrefreshed" "${repo}" kbtools make kb-verify
        check kbtools-verify-unrefreshed "$(rc "${ck}/kbtools-verify-unrefreshed")" "green before any refresh: exit $(rc "${ck}/kbtools-verify-unrefreshed")" "${ck}/kbtools-verify-unrefreshed.out"

        # 6. kb_tools' refresh, then verify green
        capture "${ck}/kbtools-refresh" "${repo}" kbtools make kb-refresh
        capture "${ck}/kbtools-verify" "${repo}" kbtools make kb-verify
        check kbtools-refresh-verify "$(( $(rc "${ck}/kbtools-refresh") | $(rc "${ck}/kbtools-verify") ))" \
            "refresh exit $(rc "${ck}/kbtools-refresh"), verify exit $(rc "${ck}/kbtools-verify")" "${ck}/kbtools-verify.out"

        # 7. kb-stats and the docent's queries
        capture "${ck}/kbtools-stats" "${repo}" kbtools make kb-stats
        check kbtools-stats "$(rc "${ck}/kbtools-stats")" "exit $(rc "${ck}/kbtools-stats")" "${ck}/kbtools-stats.out"
        # docent_queries <after>: kb_cmd deps and show on the index's first
        # node, and subtree from the entry point. A KB with no node yet has
        # deps and show asked after the op script has inserted some.
        first_node() { python3 -c 'import json, sys; lines = open(sys.argv[1]).read().splitlines(); print(json.loads(lines[0].removeprefix("--- "))["id"] if lines else "")' "${repo}/kb-root/.index/claims.yaml"; }
        docent_queries() {
            local node query arg
            node="$(first_node)"
            for query in deps show subtree; do
                arg="${node}"
                [[ "${query}" == subtree ]] && arg=.
                [[ -n "${arg}" || "$1" == op-script ]] || continue
                [[ "$1" == op-script && "${query}" == subtree ]] && continue
                capture "${ck}/kbcmd-${query}" "${repo}" kbtools python3 -m kb_tools.kb_cmd "${query}" "${arg}"
                check "kbcmd-${query}" "$( [[ -n "${arg}" ]] && rc "${ck}/kbcmd-${query}" || echo 1)" \
                    "kb_cmd ${query} '${arg}' after the ${1}: exit $(rc "${ck}/kbcmd-${query}")" "${ck}/kbcmd-${query}.out"
            done
        }
        docent_queries build
        deferred="$([[ -z "$(first_node)" ]] && echo yes || true)"

        # the overview's excerpts, against kb_tools' composition of the same tree
        ( cd "${repo}" && kbtools python3 -c 'import sys; from pathlib import Path; from kb_tools.kb_readme import compose_excerpts; sys.stdout.write(compose_excerpts(Path(sys.argv[1])).text)' \
            "${root}/${dir}/built/kb-root" ) > "${dir}/excerpts.kbtools.txt" 2> "${ck}/excerpts.log" || true
        set +e
        go test -count=1 -run '^TestExcerptsEqualKbTools$' ./internal/kbdocs -args -kbdocs.kbroot="${root}/${dir}/built/kb-root" \
            -kbdocs.want="${root}/${dir}/excerpts.kbtools.txt" >> "${ck}/excerpts.log" 2>&1
        check excerpts "$?" "kbase's excerpts of the built tree equal kb_tools' compose_excerpts" "${ck}/excerpts.log"
        set -e

        # 8. the op script through kb_util, refresh, verify; then kbase verify
        set +e
        go test -count=1 -run '^TestStageOpScript$' ./internal/write -args -opscript.script="${script}" \
            -opscript.kbroot="${root}/${repo}/kb-root" -opscript.out="${root}/${dir}/opscript" > "${ck}/opscript-stage.log" 2>&1
        staged=$?
        set -e
        check opscript-staged "${staged}" "the op script rendered against the KB" "${ck}/opscript-stage.log"
        if [[ "${staged}" -eq 0 ]]; then
            subs=(-e 's/^//')
            failed_steps=()
            while IFS=$'\t' read -r nn op create minted; do
                flags=()
                [[ "${create}" == true ]] && flags+=(--create)
                step="${nn}-${op}"
                sed "${subs[@]}" "${dir}/opscript/${step}.toml" > "${dir}/kbtools-steps/${step}.toml"
                capture "${dir}/kbtools-steps/${step}" "${repo}" kbtools python3 -m kb_tools.kb_util "${op}" --values "${root}/${dir}/kbtools-steps/${step}.toml" ${flags[@]+"${flags[@]}"}
                [[ "$(rc "${dir}/kbtools-steps/${step}")" -eq 0 ]] || failed_steps+=("${step}")
                if [[ "${minted}" != - ]]; then
                    minted_id="$(sed -n 's/.*FACT minted  *\([^ ]*\) (.*/\1/p' "${dir}/kbtools-steps/${step}.out" | head -n1)"
                    subs+=(-e "s/@MINT-${minted}@/${minted_id}/g")
                fi
            done < "${dir}/opscript/steps.tsv"
            check opscript-kbutil "$(ok test "${#failed_steps[@]}" -eq 0)" "steps exiting nonzero: ${failed_steps[*]:-none}" "${dir}/kbtools-steps"
            capture "${ck}/kbtools-refresh-ops" "${repo}" kbtools make kb-refresh
            capture "${ck}/kbtools-verify-ops" "${repo}" kbtools make kb-verify
            check opscript-kbtools-verify "$(( $(rc "${ck}/kbtools-refresh-ops") | $(rc "${ck}/kbtools-verify-ops") ))" \
                "refresh exit $(rc "${ck}/kbtools-refresh-ops"), verify exit $(rc "${ck}/kbtools-verify-ops")" "${ck}/kbtools-verify-ops.out"
            capture "${ck}/kbase-verify-ops" "${repo}" "${kbase}" verify
            check opscript-kbase-verify "$(rc "${ck}/kbase-verify-ops")" "exit $(rc "${ck}/kbase-verify-ops")" "${ck}/kbase-verify-ops.out"
        fi
        if [[ -n "${deferred}" ]]; then
            docent_queries op-script
        fi

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
            go test -count=1 -run '^TestStatusDocument$' ./internal/build -args -build.status="${root}/${kill_dir}/status.out" -build.state=failed > "${kill_dir}/status.check.log" 2>&1
            check status-killed "$?" "status after the kill parses and reports failed in claims-declared with a resume command" "${kill_dir}/status.check.log"
            set -e
            capture "${kill_dir}/resume" "${kill_dir}/repo" "${kbase}" "${build_args[@]}" --state-dir "${root}/${kill_dir}/state"
            check kill-resume "$([[ "$(rc "${kill_dir}/resume")" -eq 0 ]] && grep -qx 'resumed: true' "${kill_dir}/resume.out" \
                && grep -A1 '^restored:' "${kill_dir}/resume.out" | grep -q '^    - ' && echo 0 || echo 1)" \
                "resume exit $(rc "${kill_dir}/resume"), restoring the interrupted stage's paths" "${kill_dir}/resume.out"
            set +e
            go test -count=1 -run '^TestCompareKbTools$' ./internal/claimgraph -args -claimgraph.kbase="${root}/${dir}/built" \
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

            # stamping only if absent, and a charter's scope pin
            stamp_dir="${dir}/stamp"
            stage "${stamp_dir}/repo" "${name}"
            printf 'This KB distills arXiv %s and nothing else.\n' "${id}" > "${stamp_dir}/charter.md"
            capture "${stamp_dir}/bounded" "${stamp_dir}/repo" "${kbase}" "${build_args[@]}" --state-dir "${root}/${stamp_dir}/state" \
                --charter "${root}/${stamp_dir}/charter.md" --through depends-attributed
            printf '# Conventions this project wrote for itself\n' > "${stamp_dir}/repo/kb-root/CONVENTIONS.md"
            git -C "${stamp_dir}/repo" add kb-root/CONVENTIONS.md
            git -C "${stamp_dir}/repo" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "the project's own conventions"
            capture "${stamp_dir}/resume" "${stamp_dir}/repo" "${kbase}" "${build_args[@]}" --state-dir "${root}/${stamp_dir}/state"
            check stamp-only-if-absent "$([[ "$(rc "${stamp_dir}/bounded")" -eq 0 && "$(rc "${stamp_dir}/resume")" -eq 0 ]] \
                && cmp -s <(printf '# Conventions this project wrote for itself\n') "${stamp_dir}/repo/kb-root/CONVENTIONS.md" \
                && grep -q 'CONVENTIONS.md: present, left as authored' "${stamp_dir}/state/reports/phase-3a.yaml" && echo 0 || echo 1)" \
                "an authored CONVENTIONS.md left as it is" "${stamp_dir}/state/reports/phase-3a.yaml"
            check charter-in-resume "$(ok grep -qF -- "--charter ${root}/${stamp_dir}/charter.md" "${stamp_dir}/bounded.out")" \
                "the bounded build's resume command carries its --charter" "${stamp_dir}/bounded.out"
            check charter-scope-pin "$(ok grep -qF "This KB distills arXiv ${id} and nothing else." "${stamp_dir}/repo/kb-root/AGENTS.md")" \
                "AGENTS.md carries the charter as its scope pin" "${stamp_dir}/repo/kb-root/AGENTS.md"
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
      "per paper <id> of {{VERIFY_ARXIV_IDS}}, staged into <id>/repo (sources, stub Makefile, .gitignore, just --justfile .claude/adjagent/justfile install):\n(cd <id>/repo && ${kbase} build <volume-root> --no-inference --config-dir <id>/config --state-dir <id>/state [--bibliography <each .bib beside the volume root, sorted>]) into <id>/checks/kbase-build.*\n(cd <id>/repo && PYTHONPATH=.claude/agents python3 -m kb_tools.kb_util install-targets; make kb-verify; make kb-refresh; make kb-verify; make kb-stats; python3 -m kb_tools.kb_cmd deps|show <first claims.yaml id>; subtree .)\nkb_tools' kb_readme.compose_excerpts over <id>/built/kb-root against go test -run '^TestExcerptsEqualKbTools\$' ./internal/kbdocs\ngo test -run '^TestStageOpScript\$' ./internal/write -args -opscript.kbroot=<id>/repo/kb-root -opscript.out=<id>/opscript; each step through kb_util into <id>/kbtools-steps/; make kb-refresh; make kb-verify; ${kbase} verify\n${kbase} status --state-dir <id>/state, parsed by go test -run '^TestStatusDocument\$' ./internal/build\non {{BUILD_ARXIV_SPECIAL_ID}}: kill -9 inside claims-declared, status, resume (<id>/kill/), then go test -run '^TestCompareKbTools\$' ./internal/claimgraph -args -claimgraph.kbase=<id>/built -claimgraph.rerun=<id>/kill/repo -claimgraph.comparison=<id>/kill/comparison.yaml; a fresh build over the built tree with no trail (<id>/guard/); --charter and --through depends-attributed, an authored CONVENTIONS.md committed, then the resume (<id>/stamp/)"
    printf '\nReference: adjagent %s; %s\n' "${adjagent}" "${pandoc_version}" >> "${out}/EVIDENCE.md"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-build-arxiv): a check failed — see comparison.yaml\n'
        exit 1
    fi
    printf 'integration(test-integration-build-arxiv) ok\n'

[doc("LIVE — NEEDS the local inference endpoint test_data/fixtures/config/ names, pandoc, python3 and the adjagent clone (excluded from test-integration): ./bin/kbase build without --no-inference over the Slice-1 paper BUILD_ARXIV_SPECIAL_ID names, in a fresh fixture repository with kb_tools installed; kb_tools' verify green before any refresh, then refresh and verify green; README.md with its passage; the asks' captures, cached answers and group records under the state store's scratch/; a second build through claims-discovered configured by KBASE_API_BASE_URL, KBASE_MODEL and KBASE_API_KEY_FILE alone, no configuration directory; config= names another configuration directory; kb_tools' identify over the built tree agreeing with the node-pass record; kb_tools' unmarked-reference shortlist over the tree references-found read planning the pairs the unmarked record plans; counts in comparison.yaml; evidence under test_data/transient/test-integration-build-live-arxiv/")]
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
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    pandoc_version="$(pandoc --version | head -n 1)"
    printf 'adjagent: %s\n%s\n' "${adjagent}" "${pandoc_version}"
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
    kbtools() { PYTHONPATH=.claude/agents PYTHONDONTWRITEBYTECODE=1 "$@"; }
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
    # build is started against it.
    provider="$(sed -n 's/^provider *= *"\(.*\)".*/\1/p' "${config}/config.toml" | head -n 1)"
    capture "${out}/models" "${root}" "${kbase}" models --config-dir "${config}"
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
    just --justfile .claude/adjagent/justfile install "${root}/${repo}" > "${dir}/repo.install.log" 2>&1
    git -C "${repo}" add -A
    git -C "${repo}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "agent defs installed"
    volume_root="$({{just_executable()}} _arxiv-volume-root "${repo}")"
    printf '%s\n' "${id}"

    # the build, asking, to completion
    capture "${ck}/kbase-build" "${repo}" "${kbase}" build "${volume_root}" --config-dir "${config}" --state-dir "${state}" --log-level info
    check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-build.out" && echo 0 || echo 1)" \
        "exit $(rc "${ck}/kbase-build"), $(grep -m1 '^outcome:' "${ck}/kbase-build.out" || echo 'no outcome')" "${ck}/kbase-build.out"
    check ledger "$(ok test "$(git -C "${repo}" log --format=%s --grep='^kb-build: ' | wc -l | tr -d ' ')" -eq 10)" \
        "$(git -C "${repo}" log --format=%s --grep='^kb-build: ' | wc -l | tr -d ' ') kb-build: commits, one per stage" "${repo}/.git"
    check no-row-dropped "$(ok test -z "$(git -C "${repo}" log --format=%b --grep='^kb-build: ' | grep -- '--no-inference')")" \
        "no boundary names a dropped row" "${repo}/.git"
    check overview "$(ok grep -q '^# .* Knowledge Base$' "${repo}/kb-root/README.md")" "README.md written around its passage" "${repo}/kb-root/README.md"
    captures="$(count "${state}/scratch/captures" '*.capture.jsonl')"
    answers="$(count "${state}/scratch/answers" '*.txt')"
    ask_records="$(count "${state}/scratch/asks" '*.yaml')"
    check captures "$(ok test "${captures}" -gt 0 -a "${answers}" -gt 0 -a "${ask_records}" -gt 0)" \
        "${captures} captures, ${answers} cached answers, ${ask_records} group records under the state store's scratch/" "${state}/scratch"

    # kb_tools' runner targets: verify green before any refresh, then refresh and verify green
    capture "${ck}/install-targets" "${repo}" kbtools python3 -m kb_tools.kb_util install-targets
    check install-targets "$(rc "${ck}/install-targets")" "exit $(rc "${ck}/install-targets")" "${ck}/install-targets.out"
    capture "${ck}/kbtools-verify-unrefreshed" "${repo}" kbtools make kb-verify
    check kbtools-verify-unrefreshed "$(rc "${ck}/kbtools-verify-unrefreshed")" "green before any refresh: exit $(rc "${ck}/kbtools-verify-unrefreshed")" "${ck}/kbtools-verify-unrefreshed.out"
    capture "${ck}/kbtools-refresh" "${repo}" kbtools make kb-refresh
    capture "${ck}/kbtools-verify" "${repo}" kbtools make kb-verify
    check kbtools-refresh-verify "$(( $(rc "${ck}/kbtools-refresh") | $(rc "${ck}/kbtools-verify") ))" \
        "refresh exit $(rc "${ck}/kbtools-refresh"), verify exit $(rc "${ck}/kbtools-verify")" "${ck}/kbtools-verify.out"

    # the counts, off the build records and the result
    set +e
    go test -count=1 -run '^TestLiveBuildCounts$' ./internal/claimgraph -args -claimgraph.repo="${root}/${repo}" \
        -claimgraph.result="${root}/${ck}/kbase-build.out" -claimgraph.out="${root}/${dir}" > "${ck}/counts.log" 2>&1
    check records "$?" "every leaf landed, every planned pair answered, no candidate drafted; counts.yaml and nodepass.json written" "${ck}/counts.log"
    set -e

    # kb_tools' shortlist over the tree references-found planned over — the
    # stage writes nothing under kb-root/, so its commit's tree is the one it
    # read — with kbase's node-pass record in kb_tools' spelling: the same
    # pairs, in the same order, as the unmarked record plans.
    if [[ -f "${dir}/unmarked-plan.json" ]]; then
        at="${dir}/at-references-found"
        rm -rf "${at}"
        mkdir -p "${at}"
        git -C "${repo}" archive "$(git -C "${repo}" log -n1 --format=%H --grep='^kb-build: references-found ')" kb-root | tar -x -C "${at}"
        cp "${dir}/kb-build-node-pass.yaml" "${at}/"
        capture "${ck}/kbtools-shortlist" "${repo}" kbtools python3 -c '
    import json, sys
    from pathlib import Path
    from kb_tools import kb_pipeline
    from kb_tools.kb_claimgraph import attribute, classify, equation_sites, graph, inventory, tree, unmarked
    root = Path(sys.argv[1])
    documents = tree.read(root / "kb-root")
    sites = inventory.scan(documents)
    authored = graph.read(documents, sites)
    statement_of = classify.statements(documents, authored, sites)
    statement = {node_id: statement_of(node) for node_id, node in authored.nodes.items()}
    narrowed = attribute.narrow(documents, authored, sites, kb_pipeline.read_node_pass(root))
    planned = unmarked.plan(authored.nodes, statement, candidate_pairs=[c.pair for c in narrowed.candidates],
                            own=equation_sites.own_equations(documents, authored, sites))
    theirs = [tuple(pair) for pair in planned.pairs]
    ours = [tuple(pair) for pair in json.load(open(sys.argv[2]))]
    print(f"kb_tools plans {len(theirs)} pairs, kbase {len(ours)}; the same pairs in the same order: {theirs == ours}")
    for pair in sorted(set(theirs) - set(ours)):
        print("only kb_tools:", " -> ".join(pair))
    for pair in sorted(set(ours) - set(theirs)):
        print("only kbase:", " -> ".join(pair))
    sys.exit(0 if theirs == ours else 1)
    ' "${root}/${at}" "${root}/${dir}/unmarked-plan.json"
        check kbtools-shortlist "$(rc "${ck}/kbtools-shortlist")" "$(head -n 1 "${ck}/kbtools-shortlist.out")" "${ck}/kbtools-shortlist.out"
    fi

    # kb_tools' own reading of the built tree, judged with the answers kbase recorded
    if [[ -f "${dir}/nodepass.json" ]]; then
        capture "${ck}/kbtools-identify" "${repo}" kbtools python3 -c '
    import json, sys
    from pathlib import Path
    from kb_tools.kb_claimgraph import identify, inventory, tree
    documents = tree.read(Path("kb-root"))
    sites = inventory.scan(documents)
    differences = []
    for path, leaf in sorted(json.load(open(sys.argv[1])).items()):
        reading = identify.reading_of(documents.documents[path], sites)
        asked = identify.asked_paragraphs(reading)
        recorded = {v["line"]: v for v in leaf["verdicts"] or []}
        if [a.paragraph.start for a in asked] != sorted(recorded):
            differences.append(f"{path}: kb_tools asks paragraphs at {[a.paragraph.start for a in asked]}, the record judges {sorted(recorded)}")
            continue
        def answer(v):
            if v["verdict"] == "claim" or v.get("cause") == "unplaceable":
                return True
            return False if v["verdict"] == "not-a-claim" else None
        found = identify.judge(reading, [(a, answer(recorded[a.paragraph.start])) for a in asked])
        got = [(v.line, str(v.judgement), None if v.cause is None else str(v.cause)) for v in found.verdicts]
        want = [(v["line"], v["verdict"], v.get("cause")) for _, v in sorted(recorded.items())]
        if got != want:
            differences.append(f"{path}: kb_tools judges {got}, the record holds {want}")
        planned = leaf["claims"] or []
        if [c.title for c in found.claims] != planned:
            differences.append(f"{path}: kb_tools titles {[c.title for c in found.claims]}, the record plans {planned}")
    print("\n".join(differences) or "agree")
    sys.exit(1 if differences else 0)
    ' "${root}/${dir}/nodepass.json"
        check kbtools-identify "$(rc "${ck}/kbtools-identify")" \
            "kb_tools' asked paragraphs, verdicts and titles over the built tree $(head -n 1 "${ck}/kbtools-identify.out")" "${ck}/kbtools-identify.out"
    fi

    # The same server and model configured by the environment alone: no
    # --config-dir, the configuration directory absent, through the node pass
    # so the build asks.
    toml_get() { awk -v t="[$2]" -v k="$3" '/^\[/ { in_t = ($0 == t) } in_t && $1 == k { sub(/^[^=]*= *"/, ""); sub(/".*/, ""); print; exit }' "$1"; }
    base_url="$(toml_get "${config}/providers.toml" "${provider}" baseUrl)"
    key_file="$(toml_get "${config}/providers.toml" "${provider}" apiKeyFile)"
    [[ -z "${key_file}" || "${key_file}" == /* ]] || key_file="${config}/${key_file}"
    model="$(toml_get "${config}/config.toml" models light)"
    env_repo="${dir}/env-repo"
    no_config="${root}/${dir}/no-config"
    mkdir -p "${env_repo}"
    cp -R "{{ARXIV_DIR}}/${name}/." "${env_repo}/"
    git -C "${env_repo}" init -q
    capture "${ck}/kbase-build-env" "${env_repo}" env KBASE_CONFIG_DIR="${no_config}" KBASE_API_BASE_URL="${base_url}" \
        KBASE_MODEL="${model}" KBASE_API_KEY_FILE="${key_file}" "${kbase}" build "${volume_root}" \
        --through claims-discovered --state-dir "${root}/${dir}/env-state" --log-level info
    warned="$(grep -c 'travels in cleartext' "${ck}/kbase-build-env.log" || true)"
    expect_warning=0
    [[ -n "${key_file}" && "${base_url}" == http://* && ! "${base_url}" =~ ^http://(localhost|127\.|\[::1\]) ]] && expect_warning=1
    check build-env "$([[ "$(rc "${ck}/kbase-build-env")" -eq 0 ]] && grep -qx 'outcome: "bounded"' "${ck}/kbase-build-env.out" \
        && [[ "$(count "${root}/${dir}/env-state/scratch/captures" '*.capture.jsonl')" -gt 0 && ! -e "${no_config}" && "${warned}" -eq "${expect_warning}" ]] && echo 0 || echo 1)" \
        "configured by KBASE_API_BASE_URL, KBASE_MODEL=${model} and KBASE_API_KEY_FILE alone: exit $(rc "${ck}/kbase-build-env"), $(grep -m1 '^outcome:' "${ck}/kbase-build-env.out" || echo 'no outcome'), $(count "${root}/${dir}/env-state/scratch/captures" '*.capture.jsonl') captures, ${warned} cleartext warning(s) (${expect_warning} expected), no configuration directory" \
        "${ck}/kbase-build-env.out"

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
      "${kbase} models --config-dir <config> into models.*\non {{BUILD_ARXIV_SPECIAL_ID}}, staged into <id>/repo (sources, stub Makefile, .gitignore, just --justfile .claude/adjagent/justfile install):\n(cd <id>/repo && ${kbase} build <volume-root> --config-dir <config> --state-dir <id>/state --log-level info) into <id>/checks/kbase-build.*\n(cd <id>/repo && PYTHONPATH=.claude/agents python3 -m kb_tools.kb_util install-targets; make kb-verify; make kb-refresh; make kb-verify)\ngo test -run '^TestLiveBuildCounts\$' ./internal/claimgraph -args -claimgraph.repo=<id>/repo -claimgraph.result=<id>/checks/kbase-build.out -claimgraph.out=<id>\nkb_tools' identify.asked_paragraphs and identify.judge over <id>/repo/kb-root with <id>/nodepass.json into <id>/checks/kbtools-identify.*\ngit -C <id>/repo archive <references-found commit> kb-root | tar -x -C <id>/at-references-found; kb_tools' unmarked.plan over it with <id>/kb-build-node-pass.yaml, against <id>/unmarked-plan.json, into <id>/checks/kbtools-shortlist.*\n(cd <id>/env-repo && env KBASE_CONFIG_DIR=<id>/no-config KBASE_API_BASE_URL=<the provider's baseUrl> KBASE_MODEL=<config's models.light> KBASE_API_KEY_FILE=<its apiKeyFile> ${kbase} build <volume-root> --through claims-discovered --state-dir <id>/env-state --log-level info) into <id>/checks/kbase-build-env.*"
    printf '\nReference: adjagent %s; %s\n' "${adjagent}" "${pandoc_version}" >> "${out}/EVIDENCE.md"
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
[doc("LIVE — NEEDS the inference endpoint config names, pandoc, python3 and the adjagent clone (excluded from test-integration); DESTRUCTIVE to dir's kb-root/: over a fixture repository prep-test-integration-fixture has staged (HEAD at its `staged` tag), ./bin/kbase build <roots...> from dir without --no-inference, configured by config (default test_data/fixtures/config/, relative to the repository root); then kb_tools' verify through the fixture's installed runner green before any refresh, ./bin/kbase refresh and ./bin/kbase verify green; the counts the arXiv live recipe records in comparison.yaml, the log, the state store and each command's capture under .claude-temp/live-fixture/<timestamp>/; dir is absolute or relative to the invocation directory, roots relative to dir, and a build flag among them (e.g. --through document-graph) is passed as given")]
test-integration-build-live-fixture dir config=LIVE_CONFIG_DIR *roots:
    #!/usr/bin/env bash
    set -euo pipefail
    dir="${1}"
    shift
    [[ "${dir}" == /* ]] || dir="{{invocation_directory()}}/${dir}"
    out="$(pwd -P)/.claude-temp/live-fixture/$(date +%Y%m%dT%H%M%S)"
    mkdir -p "${out}"
    "{{just_executable()}}" _test-integration-build-live-fixture "${out}" "${dir}" "$@" 2>&1 | tee "${out}/log.txt"

[private]
[positional-arguments]
_test-integration-build-live-fixture out dir config *roots: build
    #!/usr/bin/env bash
    set -euo pipefail
    out="${1}"
    dir="${2}"
    config="${3}"
    shift 3
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
    if [[ "$(git -C "${dir}" rev-parse HEAD)" != "${staged}" ]]; then
        printf 'error: %s is not at its `staged` tag (a build has committed over it) — re-stage it with just prep-test-integration-fixture %s.\n' "${dir}" "${dir}" >&2
        exit 1
    fi
    adjagent="$(git -C .claude/adjagent rev-parse HEAD)"
    pandoc_version="$(pandoc --version | head -n 1)"
    printf 'fixture: %s at staged %s\nbuild arguments: %s\nconfig: %s\nout: %s\nadjagent: %s\n%s\n' \
        "${dir}" "${staged}" "$*" "${config}" "${out}" "${adjagent}" "${pandoc_version}"
    ck="${out}/checks"
    state="${out}/state"
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
    kbtools() { PYTHONPATH=.claude/agents PYTHONDONTWRITEBYTECODE=1 "$@"; }
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
    # build is started against it.
    provider="$(sed -n 's/^provider *= *"\(.*\)".*/\1/p' "${config}/config.toml" | head -n 1)"
    capture "${out}/models" "${root}" "${kbase}" models --config-dir "${config}"
    if [[ "$(rc "${out}/models")" -ne 0 ]]; then
        printf 'integration(test-integration-build-live-fixture): provider %s, named in %s/config.toml, is unreachable — see %s\n' \
            "${provider:-(none named)}" "${config}" "${out}/models.out"
        exit 1
    fi

    # the build, asking, from a fresh kb-root/
    rm -rf "${dir}/kb-root"
    capture "${ck}/kbase-build" "${dir}" "${kbase}" build "$@" --config-dir "${config}" --state-dir "${state}" --log-level info
    outcome="$(sed -n 's/^outcome: "\(.*\)"$/\1/p' "${ck}/kbase-build.out" | head -n 1)"
    check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 && ( "${outcome}" == done || "${outcome}" == bounded ) ]] && echo 0 || echo 1)" \
        "exit $(rc "${ck}/kbase-build"), outcome ${outcome:-none}" "${ck}/kbase-build.out"
    volumes="$(grep '^- \[' "${dir}/kb-root/entry-point.md" 2>/dev/null || true)"
    check entry-point "$(ok test -n "${volumes}")" "$(grep -c . <<< "${volumes}" || true) volume(s) listed: $(tr '\n' ' ' <<< "${volumes}")" "${dir}/kb-root/entry-point.md"
    captures="$(count "${state}/scratch/captures" '*.capture.jsonl')"
    answers="$(count "${state}/scratch/answers" '*.txt')"
    ask_records="$(count "${state}/scratch/asks" '*.yaml')"

    if [[ "${outcome}" != done ]]; then
        for c in ledger no-row-dropped captures install-targets kbtools-verify-unrefreshed kbase-refresh kbase-verify records; do
            record "${c}" skipped "the build did not finish (outcome ${outcome:-none}); this check reads a finished KB" "${ck}/kbase-build.out"
        done
    else
        boundaries="$(git -C "${dir}" log --format=%s --grep='^kb-build: ' "${staged}..HEAD" | wc -l | tr -d ' ')"
        check ledger "$(ok test "${boundaries}" -eq 10)" "${boundaries} kb-build: commits above staged, one per stage" "${dir}/.git"
        check no-row-dropped "$(ok test -z "$(git -C "${dir}" log --format=%b --grep='^kb-build: ' "${staged}..HEAD" | grep -- '--no-inference')")" \
            "no boundary names a dropped row" "${dir}/.git"
        check captures "$(ok test "${captures}" -gt 0 -a "${answers}" -gt 0 -a "${ask_records}" -gt 0)" \
            "${captures} captures, ${answers} cached answers, ${ask_records} group records under the state store's scratch/" "${state}/scratch"

        # kb_tools' verify through the runner the fixture carries, green before
        # any refresh; then kbase's refresh and verify green
        capture "${ck}/install-targets" "${dir}" kbtools python3 -m kb_tools.kb_util install-targets
        check install-targets "$(rc "${ck}/install-targets")" "exit $(rc "${ck}/install-targets")" "${ck}/install-targets.out"
        read -r -a verify_argv <<< "$(cd "${dir}" && kbtools python3 -c 'from kb_tools import kb_util; print(kb_util.verify_cmd())')"
        capture "${ck}/kbtools-verify-unrefreshed" "${dir}" kbtools "${verify_argv[@]}"
        check kbtools-verify-unrefreshed "$(rc "${ck}/kbtools-verify-unrefreshed")" "${verify_argv[*]}: green before any refresh: exit $(rc "${ck}/kbtools-verify-unrefreshed")" "${ck}/kbtools-verify-unrefreshed.out"
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
        printf 'integration(test-integration-build-live-fixture): a check failed — see %s\n' "${out}/comparison.yaml"
        exit 1
    fi
    printf 'integration(test-integration-build-live-fixture) ok — see %s\n' "${out}/comparison.yaml"

# An out-of-process monitor over a running build, as personant runs one: the
# build is started in the background in a fresh fixture repository and a
# loop polls ./bin/kbase status, each poll kept; at the first poll reporting
# the build running in a stage past start — so the start boundary is
# recorded and what follows is a resume — ./bin/kbase cancel stops it. Then:
# the build exited
# cancelled (exit 4) naming its stage and unit, that stage unrecorded; the
# lock released (a second cancel is refused, finding no holder); status
# reports cancelled with a resume command; and that command, run as status
# gives it, resumes the build to done, status then reporting it finished.
# Every result is parsed against its subcommand's documented keys.
[doc("NEEDS pandoc (excluded from test-integration): ./bin/kbase build --no-inference over MONITOR_ARXIV_ID in the background, an out-of-process loop polling ./bin/kbase status and cancelling it with ./bin/kbase cancel; cancelled (exit 4), the lock released, status cancelled with its resume, the resume to done; evidence under test_data/transient/test-integration-monitor-arxiv/")]
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
[doc("hermetic: every maintenance subcommand, status and cancel through ./bin/kbase over the committed fixture KB, each result parsed against its documented keys; evidence under test_data/transient/test-integration-results-fixture/")]
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
    repo="${out}/repo"
    docs="${out}/docs"
    rm -rf "${repo}" "${docs}" "${out}/checks.tsv"
    mkdir -p "${repo}/.git/objects" "${repo}/.git/refs/heads" "${docs}"
    printf 'ref: refs/heads/main\n' > "${repo}/.git/HEAD"
    cp -R "${fixture}/kb-root" "${repo}/"
    status=0
    n=0
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
    }
    run verify refused verify
    run refresh done refresh
    run refresh unchanged refresh
    run verify done verify
    run render-claim-graph unchanged render-claim-graph
    run insert-claim-entry done insert-claim-entry --create --values "${fixture}/values/insert.yaml"
    run insert-claim-entry unchanged insert-claim-entry --no-refresh --values "${fixture}/values/insert.yaml"
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
    run solidity-below done solidity-below 0.5
    run subtree done subtree ""
    run weak-points done weak-points
    run show done show "${id}"
    run show refused show clm-zzzzzz
    run stats done stats
    run status done status --state-dir "${root}/${out}/state"
    run cancel refused cancel --state-dir "${root}/${out}/state"
    {{just_executable()}} _write-evidence "${out}" "test-integration-results-fixture" \
      "cp -R {{RESULTS_FIXTURE_DIR}}/kb-root repo/ (with a bare .git)\nper invocation NN-<verb>, from repo/: ${kbase} <args> > docs/NN-<verb>.out, parsed by go test -run '^TestResultDocument\$' ./cmd -args -result.doc=docs/NN-<verb>.out -result.verb=<verb> -result.outcome=<outcome> > docs/NN-<verb>.keys.log\nchecks.tsv: per invocation its arguments, the outcome wanted, pass or fail, and its document"
    if [[ "${status}" -ne 0 ]]; then
        printf 'integration(test-integration-results-fixture): a result did not parse as documented — see checks.tsv\n'
        exit 1
    fi
    printf 'integration(test-integration-results-fixture) ok\n'

# The MCP transcripts over the delivered binary: the results fixture staged as
# cmd/mcp_test.go stages it, each test_data/fixtures/mcp/*.jsonl fed to
# `./bin/kbase mcp` over real pipes and every response line compared to the
# expected line after placeholder substitution. Every transcript runs: none
# needs a stub child or a provider (build-refused refuses before it starts one).
[doc("hermetic: stage the results fixture, feed each test_data/fixtures/mcp/*.jsonl transcript to ./bin/kbase mcp over pipes and compare every response line to the expected line; evidence under test_data/transient/test-integration-mcp/")]
test-integration-mcp:
    @mkdir -p "{{MCP_OUT_DIR}}"
    @{{just_executable()}} _test-integration-mcp 2>&1 | tee "{{MCP_OUT_DIR}}/log.txt"

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

# The omnibus composes the HERMETIC recipes and writes no log of its own: each
# already preserves its full output under its own name, and a second copy of
# the same bytes under a second name is a file that can go stale against the
# one anybody reads.
[doc("run every hermetic integration test (the ones needing pandoc, python3, the adjagent clone or a provider are excluded; run those by name)")]
test-integration: test-integration-results-fixture test-integration-mcp

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
    cp README.md "$stage/README.md" && \
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

[doc("drop every module nothing imports from go.mod and go.sum, upgrading nothing")]
tidy:
    go mod tidy

[doc("upgrade the WHOLE module graph; use add-dependency for one module")]
update-dependencies:
    go mod tidy
    go get -u ./...
    go mod tidy
