#!/usr/bin/env bash
# Commits a scratch fixture copy's kb-root/ and converts it with update-format,
# the staging three integration scripts share: update-format runs over a clean
# kb-root/, and every other write is refused until the conversion has run. The
# conversion's stdout and stderr land in update-format.out and update-format.log
# under the evidence directory.
#
# Inputs (environment): KBASE, the kbase binary's absolute path; REPO, the
# scratch repository holding the copy's kb-root/; OUT_DIR, the evidence
# directory; RECIPE, the calling recipe's name; COMMIT_MESSAGE, the commit's.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${KBASE:?KBASE is not set: the absolute path of the kbase binary}"
: "${REPO:?REPO is not set: the scratch repository holding the copy}"
: "${OUT_DIR:?OUT_DIR is not set: the evidence directory}"
: "${RECIPE:?RECIPE is not set: the name of the calling recipe}"
: "${COMMIT_MESSAGE:?COMMIT_MESSAGE is not set: the message of the staging commit}"

export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
git -C "${REPO}" add kb-root
git -C "${REPO}" -c user.name=kb-testing -c user.email=kb-testing@invalid commit -qm "${COMMIT_MESSAGE}"
( cd "${REPO}" && "${KBASE}" update-format ) > "${OUT_DIR}/update-format.out" 2> "${OUT_DIR}/update-format.log" || {
  printf 'integration(%s): update-format over the staged copy failed — see update-format.out\n' "${RECIPE}" >&2
  exit 1
}
