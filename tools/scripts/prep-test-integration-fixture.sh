#!/usr/bin/env bash
# kb-testing's stage-fixture, for a fixture repository the owner staged by hand:
# a caller-named directory, never a path this file knows. The scratch directory
# it removes is the installed harness's project-temp-dir, read by adjagent's
# _harness-value for the harness `install` defaults to (claude); the run-lock
# guard asks `kbase status`, which reads the lock in the state store the
# fixture's kb-root/ keys to. The reset runs on the fixture, and the fixture is
# refused if it is this repository.
#
# Inputs (environment): FIXTURE_DIR, the fixture repository, absolute or
# relative to INVOCATION_DIR; INVOCATION_DIR, the directory the recipe was
# invoked from; JUST, the just executable.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${FIXTURE_DIR:?FIXTURE_DIR is not set: the fixture repository to reset}"
: "${INVOCATION_DIR:?INVOCATION_DIR is not set: the directory the recipe was invoked from}"
: "${JUST:?JUST is not set: the just executable}"

root="$(pwd -P)"
dir="${FIXTURE_DIR}"
[[ "${dir}" == /* ]] || dir="${INVOCATION_DIR}/${dir}"
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
scratch="$("${JUST}" --justfile .claude/adjagent/justfile _harness-value claude project-temp-dir)"
printf 'staging %s: resetting to `cleared` — discards every commit above it (the installed agent set, the build ledger), uncommitted changes, kb-root/ and %s/ — then reinstalling the current agent set.\n' "${dir}" "${scratch}" >&2
git -C "${dir}" reset --hard cleared > /dev/null
rm -rf "${dir}/${scratch}" "${dir}/kb-root"
just --justfile .claude/adjagent/justfile install "${dir}"
git -C "${dir}" add -A
git -C "${dir}" -c user.name=kb-testing -c user.email=kb-testing@invalid commit -qm "agent defs installed"
git -C "${dir}" tag -f staged > /dev/null
printf 'staged %s\n' "${dir}"
