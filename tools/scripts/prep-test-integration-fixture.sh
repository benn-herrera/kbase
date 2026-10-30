#!/usr/bin/env bash
# kb-testing's stage-fixture, for a fixture repository the owner staged by hand:
# a caller-named directory, never a path this file knows. The scratch directory
# it removes is each harness's project-temp-dir, read by the _harness-value
# recipe of AGENTS_DIR's justfile (adjagent), whose install-with-kb recipe
# installs the agent set into the fixture for both harnesses, claude and
# opencode. The fixture's own kbase is .kbase/kbase, a copy of this tree's
# build, and beside it .kbase/harness.sh, a copy of this tree's tools/harness.sh.
# The run-lock guard asks .kbase/kbase for `status`, or this tree's
# build where the fixture has none; status follows the operation lock to a
# non-default store. Staging is refused before anything changes unless the
# `cleared` commit's root .gitignore ignores .kbase/, read with no host or local
# excludes. The reset runs on the fixture, and the fixture is refused if it is
# this repository.
#
# Inputs (environment): FIXTURE_DIR, the fixture repository, absolute or
# relative to INVOCATION_DIR; INVOCATION_DIR, the directory the recipe was
# invoked from; JUST, the just executable; AGENTS_DIR, the sibling adjagent
# clone; BIN_DIR, the build output directory, relative to the repository root.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${FIXTURE_DIR:?FIXTURE_DIR is not set: the fixture repository to reset}"
: "${INVOCATION_DIR:?INVOCATION_DIR is not set: the directory the recipe was invoked from}"
: "${JUST:?JUST is not set: the just executable}"
: "${AGENTS_DIR:?AGENTS_DIR is not set: the sibling adjagent clone}"
: "${BIN_DIR:?BIN_DIR is not set: the build output directory holding kbase}"

if [[ ! -f "${AGENTS_DIR}/justfile" ]]; then
    printf 'error: no justfile at %s/justfile — kbase is developed in tandem with adjagent, so AGENTS_DIR must name a sibling clone.\n' "${AGENTS_DIR}" >&2
    exit 1
fi

root="$(pwd -P)"
built="${root}/${BIN_DIR}/kbase"
if [[ ! -f "${built}" ]]; then
    printf 'error: no built kbase at %s — run `just build`.\n' "${built}" >&2
    exit 1
fi
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
probe="$(mktemp -d)"
trap 'command rm -rf "${probe}"' EXIT
git init -q "${probe}"
git -C "${dir}" show cleared:.gitignore > "${probe}/.gitignore" 2> /dev/null || true
for p in .kbase/kbase .kbase/config.toml; do
    if ! git -C "${probe}" -c core.excludesFile=/dev/null check-ignore -q "${p}"; then
        printf 'error: %s/.gitignore at `cleared` does not ignore .kbase/ — commit an entry ignoring .kbase/ and move the `cleared` tag to that commit; nothing was changed.\n' "${dir}" >&2
        exit 1
    fi
done
fixture_kbase="${dir}/.kbase/kbase"
# The fixture's own copy answers where it has one, else this tree's build. Given no
# --state-dir, status reads the store the operation lock names (a live-fixture
# build's own), else the default store. A store another kbase version wrote fails
# status, which refuses.
asker="${built}"
[[ -x "${fixture_kbase}" ]] && asker="${fixture_kbase}"
if ! status="$(cd "${dir}" && "${asker}" status)"; then
    printf 'error: kbase status failed over %s — cannot tell whether a build is running.\n' "${dir}" >&2
    exit 1
fi
if grep -q '^state: "running"' <<< "${status}"; then
    printf 'error: a kbase build is running over %s — cancel it (kbase cancel) or wait.\n' "${dir}" >&2
    exit 1
fi
scratch_claude="$("${JUST}" --justfile "${AGENTS_DIR}/justfile" _harness-value claude project-temp-dir)"
scratch_opencode="$("${JUST}" --justfile "${AGENTS_DIR}/justfile" _harness-value opencode project-temp-dir)"
for pair in "claude:${scratch_claude}" "opencode:${scratch_opencode}"; do
    if [[ -z "${pair#*:}" || "${pair#*:}" == . || "${pair#*:}" == *..* ]]; then
        printf 'error: _harness-value %s project-temp-dir returned "%s" — refusing, the reset would remove it from the fixture.\n' "${pair%%:*}" "${pair#*:}" >&2
        exit 1
    fi
done
printf 'staging %s: resetting to `cleared` — discards every commit above it (the installed agent set, the build ledger), uncommitted changes, kb-root/, %s/ and %s/ — then reinstalling the current agent set for both harnesses and copying the build of this tree to .kbase/kbase and tools/harness.sh to .kbase/harness.sh.\n' "${dir}" "${scratch_claude}" "${scratch_opencode}" >&2
git -C "${dir}" reset --hard cleared > /dev/null
rm -rf "${dir}/${scratch_claude}" "${dir}/${scratch_opencode}" "${dir}/kb-root"
"${JUST}" --justfile "${AGENTS_DIR}/justfile" install-with-kb "${dir}" --harness=claude
"${JUST}" --justfile "${AGENTS_DIR}/justfile" install-with-kb "${dir}" --harness=opencode --model-tier-map=default
# Removing before copying keeps a symlinked copy's target untouched; the copy
# is the fixture's own, so a rebuild of bin/kbase leaves a running build alone.
mkdir -p "$(dirname "${fixture_kbase}")"
command rm -f "${fixture_kbase}"
command cp "${built}" "${fixture_kbase}"
command cp -f tools/harness.sh "${dir}/.kbase/harness.sh"
git -C "${dir}" add -A
# The installs are gitignored output, so there may be nothing to commit.
git -C "${dir}" diff --cached --quiet || git -C "${dir}" -c user.name=kb-testing -c user.email=kb-testing@invalid commit -qm "agent defs installed"
git -C "${dir}" tag -f staged > /dev/null
printf 'staged %s\n' "${dir}"
