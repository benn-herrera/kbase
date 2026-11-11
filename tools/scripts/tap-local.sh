#!/usr/bin/env bash
# Installs the working tree through a scratch formula: tars every tracked file
# at its working-tree content (uncommitted edits included, untracked files left
# out) under bin/tap/, copies the tap's formula to bin/tap/${TAP_PRODUCT}.rb, points that
# copy at the tarball, reinstalls ${TAP_FORMULA} from it. The tap clone and Homebrew's
# own tap checkout are read or touched by nothing here.
#
# Inputs (environment): TAP_DIR, the tap clone's location relative to this
# repository's root.
# Needs macOS tar (bsdtar, for -s), git, cargo, shasum and brew.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
while [[ ! -d .git && "$(pwd)" != "/" ]]; do
  cd "${script_dir}/.."
done
[[ -d .git ]] || { printf '%s: failed finding project root.\n' "$(basename "${0}")" 1>&2; exit 1; }

: "${TAP_DIR:?TAP_DIR is not set: the tap clone location relative to this repository}"
: "${TAP_FORMULA:?TAP_FORMULA is not set: the name of the formula}"

src="${TAP_DIR}/Formula/${TAP_FORMULA}.rb"
if [[ ! -f "${src}" ]]; then
  printf 'tap-local: no formula at %s; run `just tap`\n' "${src}" >&2
  exit 1
fi
for tool in brew cargo shasum; do
  command -v "${tool}" >/dev/null || { printf 'tap-local: %s not on PATH\n' "${tool}" >&2; exit 1; }
done

lang=$(awk -F\" '/^[ \t]*depends_on/ { print $2; exit(0); }' "${src}")
case "${lang}" in
  rust) ver=v"$(cargo run -- --version)";;
  go) ver=v"$(go run ./cmd --version)";;
  *) printf '%s not supported. only go and rust.\n' "${lang}" 1>&2; exit 1;;
esac
tarball="${PWD}/bin/tap/${ver}.tar.gz"
formula="${PWD}/bin/tap/${TAP_FORMULA}.rb"

mkdir -p "${PWD}/bin/tap"
git ls-files -z | tar -czf "${tarball}" --null -T - -s ",^,${ver}/,"
sha="$(shasum -a 256 "${tarball}" | cut -d' ' -f1)"
cp "${src}" "${formula}"
"${script_dir}"/tap-formula-set.sh "${formula}" "file://${tarball}" "${sha}"

# `brew list --versions` prints nothing and fails only when ${TAP_FORMULA} is not installed.
if [[ -n "$(brew list --versions ${TAP_FORMULA} || true)" ]]; then
  brew uninstall ${TAP_FORMULA}
fi
# Homebrew refuses a formula installed from a path unless HOMEBREW_DEVELOPER is set.
HOMEBREW_DEVELOPER=1 brew install --formula "${formula}"
