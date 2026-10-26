#!/usr/bin/env bash
# The 2.0.1 parity goldens were recorded once, by the 2.0.1 binary, and are
# frozen: this script reads them and never writes them. Each case runs from a
# scratch copy of the 2.0.1 fixture with an empty `.git` directory beside its
# `kb-root/` (kbase finds its repository by the nearest `.git` upward, and
# without one the copy would resolve to this repository's own kb-root), so the
# delivered binary answers from the KB as converted at load. Every occurrence
# of the copy's `kb-root/` path in an answer is written as `<kb-root>`, as the
# goldens were; a path of the copy surviving that replacement fails the run.
# `show` and `stats` take no paging; every other query runs with `--limit 0`.
#
# Inputs (environment): BIN_DIR; PARITY_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${PARITY_OUT_DIR:?PARITY_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
kbase="${root}/${BIN_DIR}/kbase"
out="${root}/${PARITY_OUT_DIR}"
repo="${out}/repo"
answers="${out}/answers"
golden="${root}/internal/kbload/testdata/parity-2.0.1"
rm -rf "${repo}" "${answers}" "${out}/comparison.yaml" "${out}/compare.log"
mkdir -p "${repo}/.git" "${answers}"
cp -R "${root}/internal/formats/testdata/adjagent-sheet-2.0.1/kb-root" "${repo}/"
for f in "${golden}"/*.args; do
    stem="${answers}/$(basename "${f}" .args)"
    args=()
    while IFS= read -r line; do
        args+=("${line}")
    done < "${f}"
    paging=(--limit 0)
    case "${args[0]}" in show|stats) paging=() ;; esac
    rc=0
    ( cd "${repo}" && "${kbase}" "${args[@]}" ${paging[@]+"${paging[@]}"} ) > "${stem}.raw" 2> "${stem}.log" || rc=$?
    printf '%s\n' "${rc}" > "${stem}.exit"
    NEEDLE="${repo}/kb-root" awk 'BEGIN { needle = ENVIRON["NEEDLE"]; n = length(needle) }
        { out = ""; s = $0
          while ((i = index(s, needle)) > 0) { out = out substr(s, 1, i - 1) "<kb-root>"; s = substr(s, i + n) }
          print out s }' "${stem}.raw" > "${stem}.out"
    rm "${stem}.raw"
done
if grep -lF -- "${repo}" "${answers}"/*.out; then
    printf 'integration(test-integration-parity-adjagent-sheet): a path of the scratch copy survives in the answers above\n'
    exit 1
fi
status=0
go test -count=1 -v -run '^TestConvertedAnswersMatchTheGoldens$' ./internal/kbload -args -parity.answers="${answers}" \
    -parity.comparison="${out}/comparison.yaml" > "${out}/compare.log" 2>&1 || status=1
OUT_DIR="${out}" RECIPE="test-integration-parity-adjagent-sheet" INVOCATION="cp -R internal/formats/testdata/adjagent-sheet-2.0.1/kb-root repo/ (with an empty .git)\nper case NNN of internal/kbload/testdata/parity-2.0.1, from repo/: ${kbase} <NNN.args> [--limit 0] > answers/NNN.out (kb-root path as <kb-root>), 2> answers/NNN.log, exit in answers/NNN.exit\ngo test -count=1 -v -run '^TestConvertedAnswersMatchTheGoldens\$' ./internal/kbload -args -parity.answers=answers -parity.comparison=comparison.yaml > compare.log" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-parity-adjagent-sheet): an answer differs from its golden under the rules — see comparison.yaml and compare.log\n'
    exit 1
fi
printf 'integration(test-integration-parity-adjagent-sheet) ok\n'
