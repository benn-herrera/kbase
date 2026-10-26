#!/usr/bin/env bash
# Runs one package's unit tests and its subpackages, logging under the package's
# name in the unit-test output directory; VERBOSE=1 for per-test output.
#
# Inputs (environment): PKG, the package as the caller spelled it;
# UNIT_TEST_OUT_DIR, where the log goes.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${PKG:?PKG is not set: the package to test}"
: "${UNIT_TEST_OUT_DIR:?UNIT_TEST_OUT_DIR is not set: the unit-test output directory}"

pkg="${PKG}"
# Reduce the spellings to the path alone; `./<pkg>/...` is built below, so a
# bare pattern is never handed to go test.
pkg="${pkg#./}"
pkg="${pkg%/...}"
pkg="${pkg%/}"
if [[ -z "${pkg}" || "${pkg}" == "." || "${pkg}" == "..." ]]; then
    printf 'error: name the package to test, e.g. just test-pkg ./internal/kb — the whole repository is `just test`.\n' >&2
    exit 1
fi
mkdir -p "${UNIT_TEST_OUT_DIR}"
go test ${VERBOSE:+-v} "./${pkg}/..." 2>&1 | tee "${UNIT_TEST_OUT_DIR}/test-pkg_${pkg//\//-}_log.txt"
