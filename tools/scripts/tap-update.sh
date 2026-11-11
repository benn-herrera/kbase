#!/usr/bin/env bash
# After a release is pushed: points the tap's formula at the v<version> source
# tarball and its sha256, commits that change in the tap where the formula changed (pushing nothing), then
#
# Inputs (environment): TAP_DIR, the tap clone's location relative to this
# repository's root.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
while [[ ! -d .git && "$(pwd)" != "/" ]]; do
  cd "${script_dir}/.."
done
[[ -d .git ]] || { printf '%s: failed finding project root.\n' "$(basename "${0}")" 1>&2; exit 1; }

: "${TAP_DIR:?TAP_DIR is not set: the tap clone location relative to this repository}"
: "${TAP_FORMULA:?TAP_FORMULA is not set: the name of the formula}"

f="${TAP_DIR}/Formula/${TAP_FORMULA}.rb"
lang=$(awk -F\" '/^[ \t]*depends_on/ { print $2; exit(0); }' "${f}")
case "${lang}" in
  rust) ver=v"$(cargo run -- --version)";;
  go) ver=v"$(go run ./cmd --version)";;
  *) printf '%s not supported. only go and rust.\n' "${lang}" 1>&2; exit 1;;
esac

url="https://github.com/benn-herrera/${TAP_FORMULA}/archive/refs/tags/${ver}.tar.gz"
sha="$(curl -fsSL "${url}" | shasum -a 256 | cut -d' ' -f1)" || \
  { printf 'ensure %s tag %s was created and pushed to github\n' "${TAP_FORMULA}" "${ver}"; exit 1; }
"${script_dir}"/tap-formula-set.sh "${f}" "${url}" "${sha}"
git -C "${TAP_DIR}" pull
if ! git -C "${TAP_DIR}" diff --quiet HEAD -- Formula/${TAP_FORMULA}.rb; then
    git -C "${TAP_DIR}" commit -S -m "${TAP_FORMULA} ${ver}" -- Formula/${TAP_FORMULA}.rb
    git show --show-signature HEAD
fi
