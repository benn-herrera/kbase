#!/usr/bin/env bash
# Staging legs over scratch fixture repositories this script creates, each run
# through prep-test-integration-fixture.sh:
#   01 refused-dirty: .gitignore `kb-root/`, a commit above `cleared`, the
#      `staged` tag on it and an uncommitted edit to a tracked file; refused
#      naming the .gitignore, with HEAD, the edit and the tag all kept and no
#      .kbase/kbase.
#   02 refused-narrow: .gitignore `.kbase/kbase` alone; refused naming the
#      .gitignore, with no .kbase/kbase and no `staged` tag.
#   03 refused-host-excludes: .gitignore `kb-root/`, with .kbase/ ignored only by
#      the fixture's core.excludesFile and its .git/info/exclude; refused, with
#      no .kbase/kbase and no `staged` tag.
#   04 refused-status-failed: .gitignore `.kbase/`, no .kbase/kbase, and the
#      operation lock a running build leaves, naming a store whose holder record
#      no kbase writes, so this tree's kbase status fails over it; refused naming
#      the failed status, with no .kbase/kbase and no `staged` tag.
#   05 staged: .gitignore `.kbase/`; staged, leaving .kbase/kbase a copy of the
#      build (not a link), untracked, .kbase/harness.sh a copy of tools/harness.sh
#      (executable, not a link), and the `staged` tag at HEAD.
#   06 restaged-over-link: the same fixture with .kbase/kbase replaced by a link
#      to a sentinel file; staged again, leaving .kbase/kbase a copy of the
#      build, .kbase/harness.sh a copy of tools/harness.sh, and the sentinel
#      untouched.
# Leg 04 stands in for a running build, which needs pandoc and a long paper.
# It needs git and the adjagent clone, so it stays out of test-integration.
#
# The staging script's `status` takes no --state-dir: it is issued by the
# staging tool, not by this test. XDG_STATE_HOME is exported under the output
# directory instead, so nothing reaches the user's real state store.
#
# Inputs (environment): BIN_DIR; AGENTS_DIR, the sibling adjagent clone; JUST,
# the just executable; STAGE_FIXTURE_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${AGENTS_DIR:?AGENTS_DIR is not set: the sibling adjagent clone}"
: "${JUST:?JUST is not set: the just executable}"
: "${STAGE_FIXTURE_OUT_DIR:?STAGE_FIXTURE_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
mkdir -p "${STAGE_FIXTURE_OUT_DIR}"
out="$(cd "${STAGE_FIXTURE_OUT_DIR}" && pwd -P)"
built="${root}/${BIN_DIR}/kbase"
prep="${root}/tools/scripts/prep-test-integration-fixture.sh"
legs="${out}/legs"
tsv="${out}/stage.tsv"

rm -rf "${out}/fixtures" "${legs}" "${tsv}" "${out}/gitconfig" "${out}/excludes" "${out}/excludes-kbase" "${out}/plant-state" "${out}/sentinel" "${out}/state"
mkdir -p "${out}/fixtures" "${legs}"
: > "${out}/excludes"
: > "${tsv}"
export GIT_AUTHOR_NAME=stage-integration GIT_AUTHOR_EMAIL=stage@invalid GIT_COMMITTER_NAME=stage-integration GIT_COMMITTER_EMAIL=stage@invalid GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${out}/gitconfig"
export XDG_STATE_HOME="${out}/state"

status=0
# check <leg> <check> <evidence> <command...>: one stage.tsv row, pass when the command succeeds.
function check() {
  local leg="${1}" name="${2}" evidence="${3}" verdict=pass
  shift 3
  "$@" > "${legs}/${leg}.${name}.check" 2>&1 || { verdict=fail; status=1; }
  printf '%s\t%s\t%s\t%s\n' "${leg}" "${name}" "${verdict}" "${evidence}" >> "${tsv}"
  printf '  %s %s: %s\n' "${verdict}" "${leg}" "${name}"
}

# make_fixture <name> <gitignore-line>: a committed repository tagged `cleared`; prints its pwd -P path.
function make_fixture() {
  local dir="${out}/fixtures/${1}"
  mkdir -p "${dir}"
  git -C "${dir}" init -q
  git -C "${dir}" config core.excludesFile "${out}/excludes"
  printf '%s\n' "${2}" > "${dir}/.gitignore"
  printf 'fixture %s\n' "${1}" > "${dir}/README.md"
  git -C "${dir}" add -A
  git -C "${dir}" commit -qm "fixture ${1}"
  git -C "${dir}" tag cleared
  (cd "${dir}" && pwd -P)
}

# run_leg <NN-leg> <fixture>: one run of the staging script; leaves legs/<NN-leg>.{out,log,exit}.
function run_leg() {
  local stem="${legs}/${1}" rc=0
  BIN_DIR="${BIN_DIR}" JUST="${JUST}" AGENTS_DIR="${AGENTS_DIR}" FIXTURE_DIR="${2}" INVOCATION_DIR="${out}" \
    "${prep}" > "${stem}.out" 2> "${stem}.log" || rc=$?
  printf '%s\n' "${rc}" > "${stem}.exit"
}

function exit_nonzero() { [[ "$(cat "${1}")" != 0 ]]; }
function exit_zero() { [[ "$(cat "${1}")" == 0 ]]; }
function absent() { [[ ! -e "${1}" ]]; }
function copy_of_build() { [[ -f "${1}" && ! -L "${1}" && ! "${1}" -ef "${built}" ]] && cmp -s "${1}" "${built}"; }
function harness_is_copy() { [[ -f "${1}" && -x "${1}" && ! -L "${1}" ]] && cmp -s "${1}" "${root}/tools/harness.sh"; }
function untracked() { ! git -C "${1}" ls-files --error-unmatch .kbase/kbase; }
function staged_tag_at_head() { [[ "$(git -C "${1}" rev-parse 'staged^{commit}')" == "$(git -C "${1}" rev-parse HEAD)" ]]; }
function no_staged_tag() { ! git -C "${1}" rev-parse -q --verify refs/tags/staged; }
function head_is() { [[ "$(git -C "${1}" rev-parse HEAD)" == "${2}" ]]; }
function file_is() { [[ "$(cat "${1}")" == "${2}" ]]; }

leg=refused-dirty
stem="${legs}/01-${leg}"
fx="$(make_fixture unignored 'kb-root/')"
printf 'extra\n' > "${fx}/extra.txt"
git -C "${fx}" add -A
git -C "${fx}" commit -qm "above cleared"
above="$(git -C "${fx}" rev-parse HEAD)"
git -C "${fx}" tag staged
printf 'edited\n' > "${fx}/README.md"
run_leg "01-${leg}" "${fx}"
check "${leg}" exit-nonzero "${stem}.exit" exit_nonzero "${stem}.exit"
check "${leg}" log-names-gitignore "${stem}.log" grep -qF "${fx}/.gitignore" "${stem}.log"
check "${leg}" no-kbase-copy "${fx}/.kbase" absent "${fx}/.kbase/kbase"
check "${leg}" head-kept "${fx}" head_is "${fx}" "${above}"
check "${leg}" edit-kept "${fx}/README.md" file_is "${fx}/README.md" edited
check "${leg}" staged-tag-at-head "${fx}" staged_tag_at_head "${fx}"

leg=refused-narrow
stem="${legs}/02-${leg}"
fx="$(make_fixture narrow '.kbase/kbase')"
run_leg "02-${leg}" "${fx}"
check "${leg}" exit-nonzero "${stem}.exit" exit_nonzero "${stem}.exit"
check "${leg}" log-names-gitignore "${stem}.log" grep -qF "${fx}/.gitignore" "${stem}.log"
check "${leg}" no-kbase-copy "${fx}/.kbase" absent "${fx}/.kbase/kbase"
check "${leg}" no-staged-tag "${fx}" no_staged_tag "${fx}"

leg=refused-host-excludes
stem="${legs}/03-${leg}"
fx="$(make_fixture hostexcl 'kb-root/')"
printf '.kbase/\n' > "${out}/excludes-kbase"
git -C "${fx}" config core.excludesFile "${out}/excludes-kbase"
mkdir -p "${fx}/.git/info"
printf '.kbase/\n' >> "${fx}/.git/info/exclude"
run_leg "03-${leg}" "${fx}"
check "${leg}" exit-nonzero "${stem}.exit" exit_nonzero "${stem}.exit"
check "${leg}" no-kbase-copy "${fx}/.kbase" absent "${fx}/.kbase/kbase"
check "${leg}" no-staged-tag "${fx}" no_staged_tag "${fx}"

leg=refused-status-failed
stem="${legs}/04-${leg}"
fx="$(make_fixture opheld '.kbase/')"
mkdir -p "${out}/plant-state"
printf '{"pid":1,"started":"2026-01-01T00:00:00Z","kind":"other"}\n' > "${out}/plant-state/lock"
printf '%s' "${out}/plant-state" > "${fx}/.git/kbase-operation.lock"
run_leg "04-${leg}" "${fx}"
check "${leg}" exit-nonzero "${stem}.exit" exit_nonzero "${stem}.exit"
check "${leg}" log-names-status-failed "${stem}.log" grep -qF "kbase status failed over" "${stem}.log"
check "${leg}" no-kbase-copy "${fx}/.kbase" absent "${fx}/.kbase/kbase"
check "${leg}" no-staged-tag "${fx}" no_staged_tag "${fx}"

leg=staged
stem="${legs}/05-${leg}"
ignoring="$(make_fixture ignoring '.kbase/')"
run_leg "05-${leg}" "${ignoring}"
check "${leg}" exit-zero "${stem}.exit" exit_zero "${stem}.exit"
check "${leg}" kbase-is-copy-of-build "${ignoring}/.kbase/kbase" copy_of_build "${ignoring}/.kbase/kbase"
check "${leg}" harness-is-copy "${ignoring}/.kbase/harness.sh" harness_is_copy "${ignoring}/.kbase/harness.sh"
check "${leg}" kbase-untracked "${ignoring}" untracked "${ignoring}"
check "${leg}" staged-tag-at-head "${ignoring}" staged_tag_at_head "${ignoring}"

leg=restaged-over-link
stem="${legs}/06-${leg}"
printf 'sentinel\n' > "${out}/sentinel"
command rm -f "${ignoring}/.kbase/kbase"
ln -s "${out}/sentinel" "${ignoring}/.kbase/kbase"
run_leg "06-${leg}" "${ignoring}"
check "${leg}" exit-zero "${stem}.exit" exit_zero "${stem}.exit"
check "${leg}" kbase-is-copy-of-build "${ignoring}/.kbase/kbase" copy_of_build "${ignoring}/.kbase/kbase"
check "${leg}" harness-is-copy "${ignoring}/.kbase/harness.sh" harness_is_copy "${ignoring}/.kbase/harness.sh"
check "${leg}" sentinel-untouched "${out}/sentinel" file_is "${out}/sentinel" sentinel

OUT_DIR="${out}" RECIPE=test-integration-stage-fixture \
  INVOCATION="scratch fixtures under ${out}/fixtures/ (git init, core.excludesFile an empty file, one commit, tag cleared)\nper leg: BIN_DIR=${BIN_DIR} JUST=${JUST} AGENTS_DIR=${AGENTS_DIR} FIXTURE_DIR=<fixture> INVOCATION_DIR=${out} tools/scripts/prep-test-integration-fixture.sh" \
  tools/scripts/write-evidence.sh

if [[ "${status}" -ne 0 ]]; then
  printf 'integration(test-integration-stage-fixture) FAILED: see %s\n' "${tsv}" >&2
  exit 1
fi
printf 'integration(test-integration-stage-fixture) ok\n'
