#!/usr/bin/env bash
# LIVE: a build over a caller-named fixture repository, never a path this file
# knows. The build reads the fixture's own .kbase/*.toml; no configuration
# directory is passed. Before the build, the fixture's own kbase
# (.kbase/kbase) probe --decision and probe --text run
# from the fixture, and either failing stops the run naming its role. The
# arguments go to the fixture's own kbase build as given, run from the
# fixture: MODE staged takes at least one, the volume roots relative to the
# fixture and, where wanted, a build flag such as --through; MODE resume
# takes build flags only, after --resume. Everything the run writes but the fixture's own
# tree lands in one timestamped scratch directory: the state store, each
# command's capture, the counts and the log. A build bounded by --through skips
# the checks that read a finished KB, each recorded as skipped.
#
# MODE staged: the fixture stands at the `staged` tag
# prep-test-integration-fixture moved, so the build opens rather than resumes;
# kb-root/ is removed before it, and the state store is the scratch directory's.
# MODE resume: the fixture's commit trail's newest stage is equations-minted (a
# trail that reaches further is refused), with the prior run's state directory
# STATE_DIR so its answer cache and records are found. The build resumes at the
# trail's next stage, references-found, by build --resume; kb-root/ is left as
# the trail has it.
# The checks and the output directory are the staged form's.
#
# Inputs (environment): MODE, staged or resume; FIXTURE_DIR, the
# fixture repository; INVOCATION_DIR, the directory the recipe was invoked
# from, which FIXTURE_DIR and STATE_DIR are relative to; STATE_DIR, the prior
# run's state directory (MODE resume only).
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${MODE:?MODE is not set: staged or resume}"
: "${FIXTURE_DIR:?FIXTURE_DIR is not set: the fixture repository}"
: "${INVOCATION_DIR:?INVOCATION_DIR is not set: the directory the recipe was invoked from}"

dir="${FIXTURE_DIR}"
[[ "${dir}" == /* ]] || dir="${INVOCATION_DIR}/${dir}"
state=""
case "${MODE}" in
    staged) ;;
    resume)
        : "${STATE_DIR:?STATE_DIR is not set: the state directory of the prior run}"
        state="${STATE_DIR}"
        [[ "${state}" == /* ]] || state="${INVOCATION_DIR}/${state}"
        if [[ ! -d "${state}" ]]; then
            printf 'error: %s is not a directory — name the prior run'"'"'s state directory.\n' "${state}" >&2
            exit 1
        fi
        state="$(cd "${state}" && pwd -P)"
        ;;
    *)
        printf 'error: MODE is %s; it is staged or resume.\n' "${MODE}" >&2
        exit 1
        ;;
esac
out="$(pwd -P)/.claude-temp/live-fixture/$(date +%Y%m%dT%H%M%S)"
mkdir -p "${out}"
[[ "${MODE}" != staged ]] || state="${out}/state"

# The body both modes run; its stdout and stderr are the run's log.
function run_build() {
    mode="${MODE}"
    label=test-integration-build-live-fixture
    [[ "${mode}" == staged ]] || label="${label}-${mode}"
    root="$(pwd -P)"
    if [[ "${mode}" == staged && "$#" -eq 0 ]]; then
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
    kbase="${dir}/.kbase/kbase"
    if [[ ! -x "${kbase}" ]]; then
        printf 'error: %s is not executable — stage the fixture with just prep-test-integration-fixture first.\n' "${kbase}" >&2
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
    printf 'fixture: %s at %s\nbuild arguments: %s\nout: %s\n%s\n' \
        "${dir}" "${where}" "$*" "${out}" "${pandoc_version}"
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

    # The configuration the build will read is the fixture's own .kbase/*.toml.
    capture "${out}/show-config" "${dir}" "${kbase}" show-config
    printf 'config: the fixture'"'"'s own .kbase/*.toml, as %s.out reports\n' "${out}/show-config"

    # The endpoint first: an unreachable role fails here, by name, and no build
    # is started against it. The roles probed are the ones the build dials.
    for role in decision text; do
        capture "${out}/probe-${role}" "${dir}" "${kbase}" probe "--${role}"
        if [[ "$(rc "${out}/probe-${role}")" -ne 0 ]]; then
            printf 'integration(%s): the %s role is unreachable — see %s\n' \
                "${label}" "${role}" "${out}/probe-${role}.out"
            exit 1
        fi
    done

    # the build, asking: from a fresh kb-root/, or resuming over the trail's
    resume_flag=()
    if [[ "${mode}" == staged ]]; then
        command rm -rf "${dir}/kb-root"
    else
        resume_flag=(--resume)
    fi
    capture "${ck}/kbase-build" "${dir}" "${kbase}" build ${resume_flag[@]+"${resume_flag[@]}"} "$@" --state-dir "${state}" --log-level info
    outcome="$(sed -n 's/^outcome: "\(.*\)"$/\1/p' "${ck}/kbase-build.out" | head -n 1)"
    check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 && ( "${outcome}" == done || "${outcome}" == bounded ) ]] && echo 0 || echo 1)" \
        "exit $(rc "${ck}/kbase-build"), outcome ${outcome:-none}" "${ck}/kbase-build.out"
    volumes="$(grep '^- \[' "${dir}/kb-root/entry-point.md" 2>/dev/null || true)"
    check entry-point "$(ok test -n "${volumes}")" "$(grep -c . <<< "${volumes}" || true) volume(s) listed: $(tr '\n' ' ' <<< "${volumes}")" "${dir}/kb-root/entry-point.md"
    captures="$(count "${state}/scratch/trails" '*.capture.jsonl')"
    letter_answers="$(count "${state}/scratch/trails" '*.json')"
    overview_answers="$(count "${state}/scratch/trails" '*.txt')"
    ask_records="$(count "${state}/scratch/trails" '*.yaml')"

    if [[ "${outcome}" != done ]]; then
        for c in no-row-dropped captures kbase-refresh kbase-verify records; do
            record "${c}" skipped "the build did not finish (outcome ${outcome:-none}); this check reads a finished KB" "${ck}/kbase-build.out"
        done
    else
        check no-row-dropped "$(ok test -z "$(git -C "${dir}" log --format=%b --grep='^kb-build: ' "${staged}..HEAD" | grep -- '--no-inference')")" \
            "no boundary names a dropped row" "${dir}/.git"
        check captures "$(ok test "${captures}" -gt 0 -a "${letter_answers}" -gt 0 -a "${ask_records}" -gt 0)" \
            "${captures} captures, ${letter_answers} cached letter-ask answers, ${overview_answers} cached overview answers, ${ask_records} group-record files under the state store's scratch/" "${state}/scratch"

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
        printf '  outcome: %s\n  state-dir: %s\n  scratch:\n    captures: %s\n    letter-answers: %s\n    overview-answers: %s\n    ask-records: %s\n  checks:\n' \
            "$(yaml_str "${outcome}")" "$(yaml_str "${state}")" "${captures}" "${letter_answers}" "${overview_answers}" "${ask_records}"
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
}

run_build "$@" 2>&1 | tee "${out}/log.txt"
