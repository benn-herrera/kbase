#!/usr/bin/env bash
# Checks that the derived files are current at every stage commit of a build:
# each kb-build: commit reachable from the repository's HEAD whose kb-root/ is
# a KB — from the spine seed on — exported by git archive into
# <out-dir>/commits/<commit>/ beside an empty .git, answers an empty
# write --dry-run batch unchanged. Writes <out-dir>/trail.txt, the kb-build:
# commits newest first, and <out-dir>/commits.tsv, each commit checked and its
# outcome. Exits non-zero where no commit holds a KB or one does not derive
# unchanged.
#
# Usage: check-build-trail.sh <repo> <out-dir> <kbase>
set -euo pipefail

usage='usage: check-build-trail.sh <repo> <out-dir> <kbase>'
repo="${1:?${usage}}"
out="${2:?${usage}}"
kbase="${3:?${usage}}"

mkdir -p "${out}"
printf 'entry: []\n' > "${out}/empty-batch.yaml"
: > "${out}/commits.tsv"
git -C "${repo}" log --format='%H' --grep='^kb-build: ' > "${out}/trail.txt"
while read -r commit; do
    git -C "${repo}" cat-file -e "${commit}:kb-root/.index/format.yaml" 2> /dev/null || continue
    dir="${out}/commits/${commit}"
    mkdir -p "${dir}/.git"
    git -C "${repo}" archive --format=tar "${commit}" kb-root | tar -x -C "${dir}"
    set +e
    ( cd "${dir}" && "${kbase}" write --dry-run --values "${out}/empty-batch.yaml" ) > "${dir}.out" 2> "${dir}.log"
    set -e
    printf '%s\t%s\n' "${commit}" "$(sed -n 's/^outcome: "\(.*\)"$/\1/p' "${dir}.out")" >> "${out}/commits.tsv"
done < "${out}/trail.txt"
if [[ ! -s "${out}/commits.tsv" ]]; then
    printf 'check-build-trail: no kb-build: commit in %s holds a KB\n' "${repo}" >&2
    exit 1
fi
if [[ -n "$(awk -F '\t' '$2 != "unchanged"' "${out}/commits.tsv")" ]]; then
    printf 'check-build-trail: a kb-build: commit in %s does not derive unchanged — see %s\n' "${repo}" "${out}/commits.tsv" >&2
    exit 1
fi
