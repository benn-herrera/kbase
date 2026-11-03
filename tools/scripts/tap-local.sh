#!/usr/bin/env bash
# Installs the working tree through a scratch formula: tars every tracked file
# at its working-tree content (uncommitted edits included, untracked files left
# out) under bin/tap/, copies the tap's formula to bin/tap/kbase.rb, points that
# copy at the tarball, reinstalls kbase from it. The tap clone and Homebrew's
# own tap checkout are read or touched by nothing here.
#
# Inputs (environment): TAP_DIR, the tap clone's location relative to this
# repository's root.
# Needs macOS tar (bsdtar, for -s), git, go, shasum and brew.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${TAP_DIR:?TAP_DIR is not set: the tap clone location relative to this repository}"
src="${TAP_DIR}/Formula/kbase.rb"
if [[ ! -f "${src}" ]]; then
  printf 'tap-local: no formula at %s; run `just tap`\n' "${src}" >&2
  exit 1
fi
for tool in brew go shasum; do
  command -v "${tool}" >/dev/null || { printf 'tap-local: %s not on PATH\n' "${tool}" >&2; exit 1; }
done

version="$(go run ./cmd --version)"
top="kbase-${version}"
tarball="${PWD}/bin/tap/${top}.tar.gz"
formula="${PWD}/bin/tap/kbase.rb"
mkdir -p "${PWD}/bin/tap"
git ls-files -z | tar -czf "${tarball}" --null -T - -s ",^,${top}/,"
sha="$(shasum -a 256 "${tarball}" | cut -d' ' -f1)"
cp "${src}" "${formula}"
tools/scripts/tap-formula-set.sh "${formula}" "file://${tarball}" "${sha}"

# `brew list --versions` prints nothing and fails only when kbase is not installed.
if [[ -n "$(brew list --versions kbase || true)" ]]; then
  brew uninstall kbase
fi
# Homebrew refuses a formula installed from a path unless HOMEBREW_DEVELOPER is set.
HOMEBREW_DEVELOPER=1 brew install --formula "${formula}"
