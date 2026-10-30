#!/usr/bin/env bash
# §8.3 Direction 1: a kbase-built KB. Per slice paper, a fresh fixture
# repository (the sources as fetched, a stub Makefile, a .gitignore) is built
# by ./bin/kbase --no-inference to completion; the build's ledger, its owned
# paths and the readiness documents are checked as built, and ./bin/kbase
# status and the build result are parsed against their documented keys. On
# BUILD_ARXIV_SPECIAL_ID: a build killed with SIGKILL while
# claims-declared's writes are in flight, its status read, then resumed; a fresh
# build over the built tree in a repository with no trail refused; and
# CONVENTIONS.md authored before phase-3a and left as it is. Every
# result lands in comparison.yaml.
#
# Inputs (environment): BIN_DIR; ARXIV_DIR, the unpacked corpus;
# BUILD_ARXIV_IDS, the papers built; BUILD_ARXIV_SPECIAL_ID, the paper that is
# also killed, guarded and stamped; BUILD_ARXIV_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${ARXIV_DIR:?ARXIV_DIR is not set: the unpacked corpus}"
: "${BUILD_ARXIV_IDS:?BUILD_ARXIV_IDS is not set: the papers to build}"
: "${BUILD_ARXIV_SPECIAL_ID:?BUILD_ARXIV_SPECIAL_ID is not set: the killed and stamped paper}"
: "${BUILD_ARXIV_OUT_DIR:?BUILD_ARXIV_OUT_DIR is not set: the evidence directory}"

root="$(pwd)"
out="${BUILD_ARXIV_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
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
    cp -R "${ARXIV_DIR}/${name}/." "${repo}/"
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
# Unquoted: the ids are a space-separated list, split into words here.
for id in ${BUILD_ARXIV_IDS}; do
    name="$(printf '%s' "${id}" | tr '/' '_')"
    dir="${out}/${name}"
    repo="${dir}/repo"
    ck="${dir}/checks"
    rm -rf "${dir}" "${dir}".*
    mkdir -p "${ck}" "${dir}/config" "${dir}/built"
    checks=()
    printf '%s\n' "${id}"
    stage "${repo}" "${name}"
    volume_root="$(ARXIV_VOLUME_DIR="${repo}" tools/scripts/arxiv-volume-root.sh)"
    bibs=()
    while IFS= read -r bib; do
        bibs+=(--bibliography "${bib}")
    done < <(cd "${repo}" && find "$(dirname "${volume_root}")" -maxdepth 1 -name '*.bib' | sed 's|^\./||' | LC_ALL=C sort)
    build_args=(build "${volume_root}" --no-inference --config-dir "${root}/${dir}/config" ${bibs[@]+"${bibs[@]}"})
    resume_args=(build --resume --no-inference --config-dir "${root}/${dir}/config")

    # the build, to completion
    capture "${ck}/kbase-build" "${repo}" "${kbase}" "${build_args[@]}" --state-dir "${root}/${dir}/state"
    check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-build.out" && echo 0 || echo 1)" \
        "exit $(rc "${ck}/kbase-build"), $(grep -m1 '^outcome:' "${ck}/kbase-build.out" || echo 'no outcome')" "${ck}/kbase-build.out"
    cp -R "${repo}/kb-root" "${dir}/built/"
    cp "${repo}"/kb-build-*.yaml "${dir}/built/" 2>/dev/null || true
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

    if [[ "${id}" == "${BUILD_ARXIV_SPECIAL_ID}" ]]; then
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
        capture "${kill_dir}/plain-over-trail" "${kill_dir}/repo" "${kbase}" "${build_args[@]}" --state-dir "${root}/${kill_dir}/state"
        check plain-over-trail "$([[ "$(rc "${kill_dir}/plain-over-trail")" -eq 1 ]] && grep -q 'check: "inputs"' "${kill_dir}/plain-over-trail.out" \
            && grep 'remedy:' "${kill_dir}/plain-over-trail.out" | grep 'kbase build --resume' | grep 'git reset --hard' | grep -q 'started-at' && echo 0 || echo 1)" \
            "exit $(rc "${kill_dir}/plain-over-trail"), a plain build over the interrupted trail refused, its remedy naming --resume and the start-over commit" "${kill_dir}/plain-over-trail.out"
        capture "${kill_dir}/resume" "${kill_dir}/repo" "${kbase}" "${resume_args[@]}" --state-dir "${root}/${kill_dir}/state"
        check kill-resume "$([[ "$(rc "${kill_dir}/resume")" -eq 0 ]] && grep -qx 'resumed: true' "${kill_dir}/resume.out" \
            && grep -A1 '^restored:' "${kill_dir}/resume.out" | grep -q '^    - ' && echo 0 || echo 1)" \
            "resume exit $(rc "${kill_dir}/resume"), restoring the interrupted stage's paths" "${kill_dir}/resume.out"

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
        capture "${stamp_dir}/resume" "${stamp_dir}/repo" "${kbase}" "${resume_args[@]}" --state-dir "${root}/${stamp_dir}/state"
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

OUT_DIR="${out}" RECIPE="test-integration-build-arxiv" INVOCATION="per paper <id> of ${BUILD_ARXIV_IDS}, staged into <id>/repo (sources as fetched, stub Makefile, .gitignore, one commit):\n(cd <id>/repo && ${kbase} build <volume-root> --no-inference --config-dir <id>/config --state-dir <id>/state [--bibliography <each .bib beside the volume root, sorted>]) into <id>/checks/kbase-build.*\n<id>/built/kb-root and the three kb-build-*.yaml files as the build left them; the ledger read from <id>/repo/.git\n<id>/built/kb-root's AGENTS.md, CONVENTIONS.md and CLAUDE.md as built, checked against the readiness rules\n${kbase} status --state-dir <id>/state, parsed by go test -run '^TestStatusDocument\$' ./internal/build; the build and status results parsed by go test -run '^TestResultDocument\$' ./cmd\non ${BUILD_ARXIV_SPECIAL_ID}: kill -9 inside claims-declared, status, a plain build over the interrupted trail refused (<id>/kill/plain-over-trail.*), then ${kbase} build --resume --no-inference --config-dir <id>/config --state-dir <id>/state (<id>/kill/); a fresh build over the built tree with no trail (<id>/guard/); --through depends-attributed, an authored CONVENTIONS.md committed, then the same build --resume (<id>/stamp/)" \
    tools/scripts/write-evidence.sh
printf '\nReference: %s\n' "${pandoc_version}" >> "${out}/EVIDENCE.md"
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-build-arxiv): a check failed — see comparison.yaml\n'
    exit 1
fi
printf 'integration(test-integration-build-arxiv) ok\n'
