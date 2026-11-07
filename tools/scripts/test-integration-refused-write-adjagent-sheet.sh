#!/usr/bin/env bash
# A write op whose trailing refresh would refuse refuses before it writes:
# edit-claim over a scratch copy of the 2.0.1 fixture whose kb-root/CLAUDE.md
# is not the redirect comes back refused, check agents-redirect, and leaves
# kb-root/ byte-identical to a snapshot taken before the run. The copy sits
# beside a minimal `.git` (kbase finds its repository by the
# nearest `.git` upward, and without one the copy would resolve to this
# repository's own kb-root); it is committed and converted by update-format
# first, since every write over a KB whose conversion is pending is refused
# before the redirect is read. The result is parsed against
# edit-claim's keys and the refused outcome by TestResult.
#
# Inputs (environment): BIN_DIR; REFUSED_WRITE_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${REFUSED_WRITE_OUT_DIR:?REFUSED_WRITE_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
kbase="${root}/${BIN_DIR}/kbase"
out="${root}/${REFUSED_WRITE_OUT_DIR}"
repo="${out}/repo"
docs="${out}/docs"
rm -rf "${repo}" "${docs}" "${out}/snapshot" "${out}/tree.diff" "${out}/values.yaml" "${out}/checks.tsv" \
  "${out}/update-format.out" "${out}/update-format.log"
mkdir -p "${repo}/.git/objects" "${repo}/.git/refs/heads" "${docs}" "${out}/snapshot"
printf 'ref: refs/heads/main\n' > "${repo}/.git/HEAD"
cp -R "${root}/internal/formats/testdata/adjagent-sheet-2.0.1/kb-root" "${repo}/"
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
KBASE="${kbase}" REPO="${repo}" OUT_DIR="${out}" RECIPE="test-integration-refused-write-adjagent-sheet" COMMIT_MESSAGE="the 2.0.1 fixture as staged" tools/scripts/stage-converted-copy.sh
printf 'not the redirect\n' > "${repo}/kb-root/CLAUDE.md"
cp -R "${repo}/kb-root/." "${out}/snapshot/"
printf 'entry:\n- id: clm-aaa002\n  rigor: 0.5\n' > "${out}/values.yaml"

status=0
# run <verb> <outcome> <args...>: one invocation from the repository, its
# result parsed against the verb's documented keys and the outcome wanted.
function run() {
  local verb="${1}" want="${2}" stem="${docs}/01-${1}" rc
  shift 2
  set +e
  ( cd "${repo}" && "${kbase}" "${verb}" "$@" ) > "${stem}.out" 2> "${stem}.log"
  printf '%s\n' "${?}" > "${stem}.exit"
  go test -count=1 -run '^TestResult$' ./cmd -args -result.file="${stem}.out" -result.verb="${verb}" \
    -result.outcome="${want}" > "${stem}.keys.log" 2>&1
  rc=${?}
  set -e
  printf '%s\t%s\t%s\t%s\n' "${verb} $*" "${want}" "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "${stem}.out" >> "${out}/checks.tsv"
  printf '  %s %s: want %s, exit %s\n' "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "${verb}" "${want}" "$(cat "${stem}.exit")"
  [[ "${rc}" -eq 0 ]] || status=1
}

run edit-claim refused --values "${out}/values.yaml"
if ! grep -qx '    - check: "agents-redirect"' "${docs}/01-edit-claim.out"; then
  printf '  fail: edit-claim is not refused as agents-redirect — see docs/01-edit-claim.out\n'
  status=1
fi
if ! diff -r "${out}/snapshot" "${repo}/kb-root" > "${out}/tree.diff" 2>&1; then
  printf '  fail: kb-root differs from its snapshot — see tree.diff\n'
  status=1
fi

OUT_DIR="${out}" RECIPE="test-integration-refused-write-adjagent-sheet" INVOCATION="cp -R internal/formats/testdata/adjagent-sheet-2.0.1/kb-root repo/ (with a bare .git), git add kb-root and commit, from repo/: ${kbase} update-format > update-format.out, then kb-root/CLAUDE.md rewritten as a line other than the redirect, kb-root/ copied to snapshot/\nfrom repo/: ${kbase} edit-claim --values values.yaml > docs/01-edit-claim.out, 2> docs/01-edit-claim.log, exit in docs/01-edit-claim.exit\ngo test -count=1 -run '^TestResult\$' ./cmd -args -result.file=docs/01-edit-claim.out -result.verb=edit-claim -result.outcome=refused > docs/01-edit-claim.keys.log; docs/01-edit-claim.out holds check agents-redirect\ndiff -r snapshot repo/kb-root > tree.diff (empty when kb-root is unchanged)" \
  tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
  printf 'integration(test-integration-refused-write-adjagent-sheet): the write was not refused with kb-root unchanged — see checks.tsv and tree.diff\n'
  exit 1
fi
printf 'integration(test-integration-refused-write-adjagent-sheet) ok\n'
