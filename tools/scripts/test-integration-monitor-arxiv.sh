#!/usr/bin/env bash
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
#
# Inputs (environment): BIN_DIR; ARXIV_DIR, the unpacked corpus;
# MONITOR_ARXIV_ID, the paper built; MONITOR_ARXIV_OUT_DIR, the evidence
# directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${ARXIV_DIR:?ARXIV_DIR is not set: the unpacked corpus}"
: "${MONITOR_ARXIV_ID:?MONITOR_ARXIV_ID is not set: the paper to build and cancel}"
: "${MONITOR_ARXIV_OUT_DIR:?MONITOR_ARXIV_OUT_DIR is not set: the evidence directory}"

root="$(pwd)"
out="${MONITOR_ARXIV_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
name="$(printf '%s' "${MONITOR_ARXIV_ID}" | tr '/' '_')"
repo="${out}/repo"
state="${root}/${out}/state"
# The state home every kbase call runs under, so the credentials store each
# reads is one under the evidence directory, never the operator's own.
export XDG_STATE_HOME="${root}/${out}/xdg"
rm -rf "${repo}" "${out}/state" "${out}/polls" "${out}/config" "${out}"/*.out "${out}"/*.log "${out}"/*.exit "${out}/monitor.tsv" "${out}/checks.tsv" "${XDG_STATE_HOME}"
mkdir -p "${repo}" "${out}/polls" "${out}/config"
cp -R "${ARXIV_DIR}/${name}/." "${repo}/"
git -C "${repo}" init -q
git -C "${repo}" add -A
git -C "${repo}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "sources as fetched"
volume_root="$(ARXIV_VOLUME_DIR="${repo}" tools/scripts/arxiv-volume-root.sh)"
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
check lock-released "$([[ "$(cat "${out}/cancel-again.exit")" -eq 1 ]] && grep -q 'no run holds the state store' "${out}/cancel-again.out" && echo 0 || echo 1)" \
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

OUT_DIR="${out}" RECIPE="test-integration-monitor-arxiv" INVOCATION="${MONITOR_ARXIV_ID} staged into repo/ (sources, git init, one commit)\n(cd repo && ${kbase} build <volume-root> --no-inference --config-dir config --state-dir state) > build.out &\nper poll NNNN until one reports running: (cd repo && ${kbase} status --state-dir state) > polls/NNNN.out; then ${kbase} cancel --state-dir state > cancel.out\nwait for the build; ${kbase} cancel again > cancel-again.out; ${kbase} status > status-cancelled.out\nthe resume command status-cancelled.out gives (resume.command), run from repo > resume.out; ${kbase} status > status-finished.out\neach result: go test -run '^TestResultDocument\$' ./cmd -args -result.doc=<stem>.out -result.verb=<verb> -result.outcome=<outcome> > <stem>.keys.log" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-monitor-arxiv): a check failed — see checks.tsv\n'
    exit 1
fi
printf 'integration(test-integration-monitor-arxiv) ok\n'
