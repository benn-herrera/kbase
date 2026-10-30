#!/usr/bin/env bash
# Sets up the sibling Homebrew tap clone, homebrew-kbase/, that this project's
# formula lives in, so the tap and the main tree are maintained in parallel.
#
# The clone is idempotent by STATE, not by the directory being absent: `git
# clone` refuses a non-empty destination, and a present directory may already
# hold the repository. A directory whose own `.git` makes it a repository, with
# a remote named origin, is already set up and is left untouched; one with no
# origin gets the remote and nothing else; one with another origin is refused,
# since pointing it at the tap would move whatever it clones. A directory with
# no `.git` holds something of the user's, and is refused rather than written
# into.
#
# Inputs (environment): TAP_DIR, the clone's location relative to this
# repository's root; TAP_URL, the tap repository to clone from or to set as
# origin.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${TAP_DIR:?TAP_DIR is not set: the tap clone location relative to this repository}"
: "${TAP_URL:?TAP_URL is not set: the tap repository to clone from or to set as origin}"

if [[ ! -e "${TAP_DIR}" ]]; then
    git clone "${TAP_URL}" "${TAP_DIR}" || exit 1
    printf 'tap: cloned %s into %s\n' "${TAP_URL}" "${TAP_DIR}"
    exit 0
fi

if [[ ! -d "${TAP_DIR}/.git" ]]; then
    kind="a directory that is not a git repository"
    [[ -d "${TAP_DIR}" ]] || kind="a file"
    printf 'tap: %s holds no .git (it is %s) — refused, nothing was changed; remove it by hand, or clone the tap somewhere else and set TAP_DIR to that.\n' \
        "${TAP_DIR}" "${kind}" >&2
    exit 1
fi

# A `.git` that is empty, or otherwise not a repository, does not make the
# directory one, and git run inside it answers from the enclosing repository —
# this one — instead. Refuse that before asking git anything.
if [[ "$(git -C "${TAP_DIR}" rev-parse --show-toplevel 2> /dev/null)" != "$(cd "${TAP_DIR}" && pwd -P)" ]]; then
    printf 'tap: %s holds a .git that is not a repository — refused, nothing was changed; remove it by hand, or clone the tap somewhere else and set TAP_DIR to that.\n' \
        "${TAP_DIR}" >&2
    exit 1
fi

if ! origin="$(git -C "${TAP_DIR}" remote get-url origin 2> /dev/null)"; then
    git -C "${TAP_DIR}" remote add origin "${TAP_URL}" || exit 1
    printf 'tap: added origin %s to the existing %s clone\n' "${TAP_URL}" "${TAP_DIR}"
    exit 0
fi

if [[ "${origin}" != "${TAP_URL}" ]]; then
    printf 'tap: %s already has origin %s, not %s — refused, nothing was changed.\n' \
        "${TAP_DIR}" "${origin}" "${TAP_URL}" >&2
    exit 1
fi

printf 'tap: %s is already set up (origin %s) — nothing was changed\n' "${TAP_DIR}" "${origin}"
