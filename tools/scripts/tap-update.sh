#!/usr/bin/env bash
# After a release is pushed: points the tap's formula at the v<version> source
# tarball and its sha256, commits that change in the tap where the formula changed (pushing nothing), then
# keeps go.mod's `go` line inside Go's supported window.
#
# The window is the two newest minors https://go.dev/dl/?mode=json lists. A
# go.mod line older than the older of them is raised to 1.<older>.0 and left
# uncommitted in this repository; this script makes no commit here.
#
# Inputs (environment): TAP_DIR, the tap clone's location relative to this
# repository's root.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${TAP_DIR:?TAP_DIR is not set: the tap clone location relative to this repository}"

releases_url="https://go.dev/dl/?mode=json"
if ! releases="$(curl -fsSL "${releases_url}")"; then
    printf 'tap-update: could not read %s\n' "${releases_url}" >&2
    exit 1
fi
# The two newest minors, newest first.
minors="$(printf '%s\n' "${releases}" | sed -n 's/.*"version": *"go1\.\([0-9][0-9]*\)\..*/\1/p' | sort -rnu | head -2)"
if [[ -z "${minors}" ]]; then
    printf 'tap-update: %s lists no Go version\n' "${releases_url}" >&2
    exit 1
fi
older="$(printf '%s\n' "${minors}" | tail -1)"

ver="v$(go run ./cmd --version)"
url="https://github.com/benn-herrera/kbase/archive/refs/tags/${ver}.tar.gz"
sha="$(curl -fsSL "${url}" | shasum -a 256 | cut -d' ' -f1)"
f="${TAP_DIR}/Formula/kbase.rb"
tools/scripts/tap-formula-set.sh "${f}" "${url}" "${sha}"
if ! git -C "${TAP_DIR}" diff --quiet HEAD -- Formula/kbase.rb; then
    git -C "${TAP_DIR}" commit -S -m "kbase ${ver}" -- Formula/kbase.rb
fi

current="$(sed -n 's/^go \([0-9.][0-9.]*\)$/\1/p' go.mod)"
minor="${current#1.}"
minor="${minor%%.*}"
if ((minor < older)); then
    go mod edit -go="1.${older}.0"
    printf 'go.mod: go line raised from %s to 1.%s.0, left uncommitted\n' "${current}" "${older}"
else
    printf 'go.mod: go %s is within Go'\''s supported window (1.%s, 1.%s)\n' "${current}" "${older}" "$((older + 1))"
fi
