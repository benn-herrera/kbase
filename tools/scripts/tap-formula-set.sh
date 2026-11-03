#!/usr/bin/env bash
# Points a tap formula's url and sha256 lines at the given values; the one
# home of that rewrite, shared by tap-update.sh and tap-local.sh.
#
# Usage: tap-formula-set.sh <formula-file> <url> <sha256>
set -euo pipefail

f="${1}"
url="${2}"
sha="${3}"
if [[ ! -f "${f}" ]]; then
    printf 'tap-formula-set: no formula file %s\n' "${f}" >&2
    exit 1
fi

sed -i '' \
    -e "s|^  url \".*\"|  url \"${url}\"|" \
    -e "s|^  sha256 \".*\"|  sha256 \"${sha}\"|" \
    "${f}"
