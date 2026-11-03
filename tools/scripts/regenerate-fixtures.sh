#!/usr/bin/env bash
# Regenerates the committed fixtures that pin kbase's own output, each from the
# delivered binary, so none is written by hand. Today that is init's skeleton:
# ./bin/kbase init in a fresh git repository named `repo` (the entry point's
# heading is the repository's directory name), whose entry point, format stamp
# and two record files are copied into the skeleton fixture that
# TestInitBirthWritesTheBornTreeAndCommitsIt compares a born KB against. The
# scratch repository stays under the evidence directory.
#
# Inputs (environment): BIN_DIR; REGENERATE_OUT_DIR, the evidence directory;
# INIT_SKELETON_DIR, the skeleton fixture.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${REGENERATE_OUT_DIR:?REGENERATE_OUT_DIR is not set: the evidence directory}"
: "${INIT_SKELETON_DIR:?INIT_SKELETON_DIR is not set: the init skeleton fixture}"

root="$(pwd)"
out="${root}/${REGENERATE_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
repo="${out}/repo"
skeleton="${root}/${INIT_SKELETON_DIR}"
rm -rf "${repo}" "${out}/xdg" "${out}/config" "${out}/gitconfig" "${out}/init.out" "${out}/init.log"
mkdir -p "${repo}" "${out}/config"
export XDG_STATE_HOME="${out}/xdg" KBASE_CONFIG_DIR="${out}/config"
export GIT_AUTHOR_NAME=regenerate-fixtures GIT_AUTHOR_EMAIL=regenerate@invalid GIT_COMMITTER_NAME=regenerate-fixtures \
    GIT_COMMITTER_EMAIL=regenerate@invalid GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${out}/gitconfig"
git -C "${repo}" init -q
if ! ( cd "${repo}" && "${kbase}" init ) > "${out}/init.out" 2> "${out}/init.log"; then
    printf 'regenerate-fixtures: kbase init failed — see %s\n' "${out}/init.out"
    exit 1
fi
for rel in entry-point.md .index/format.yaml .index/nodes.yaml .index/edges.yaml; do
    mkdir -p "$(dirname "${skeleton}/${rel}")"
    cp "${repo}/kb-root/${rel}" "${skeleton}/${rel}"
    printf 'regenerate-fixtures: %s/%s\n' "${INIT_SKELETON_DIR}" "${rel}"
done
