#!/usr/bin/env bash
# The compatibility op script over the delivered binary: TestStageOpScript
# renders test-data/fixtures/compat/ops.yaml against a scratch copy of the
# results fixture, every step runs through ./bin/kbase to done, verify is
# green, and every step re-issued is unchanged with kb-root/ byte-identical.
# It needs the Go toolchain the build already uses, so it joins the omnibus.
#
# Inputs (environment): BIN_DIR; RESULTS_FIXTURE_DIR, the committed fixture;
# COMPAT_OP_SCRIPT, the op script; OPSCRIPT_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${RESULTS_FIXTURE_DIR:?RESULTS_FIXTURE_DIR is not set: the committed results fixture}"
: "${COMPAT_OP_SCRIPT:?COMPAT_OP_SCRIPT is not set: the compatibility op script}"
: "${OPSCRIPT_OUT_DIR:?OPSCRIPT_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
out="${root}/${OPSCRIPT_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
repo="${out}/repo"
rm -rf "${repo}" "${out}/steps" "${out}/docs" "${out}/kb-root.before-reissue" "${out}/reissue.diff" \
    "${out}/opscript.tsv" "${out}/minted.txt" "${out}/stage.log"
mkdir -p "${repo}/.git/objects" "${repo}/.git/refs/heads" "${out}/docs"
printf 'ref: refs/heads/main\n' > "${repo}/.git/HEAD"
cp -R "${root}/${RESULTS_FIXTURE_DIR}/kb-root" "${repo}/"
# The fixture's one leaf holds no line the locator search can choose, so
# the scratch copy's gains one; the script hosts its claims there.
printf '\n%s\n' 'The probe rests on a line the locator finds once.' >> "${repo}/kb-root/a.md"
go test -count=1 -run '^TestStageOpScript$' ./internal/write -args -opscript.script="${root}/${COMPAT_OP_SCRIPT}" \
    -opscript.kbroot="${repo}/kb-root" -opscript.out="${out}/steps" > "${out}/stage.log" 2>&1 \
    || { printf 'integration(test-integration-opscript-results): staging failed — see stage.log\n'; exit 1; }
[[ -s "${out}/steps/steps.tsv" ]] || { printf 'integration(test-integration-opscript-results): staging wrote no steps — see stage.log\n'; exit 1; }
status=0
fail() { printf '  fail: %s\n' "$*"; status=1; }
# minted holds name=id for each step that names its id; a later step's
# @MINT-<name>@ tokens are replaced by it before the step runs.
minted=()
# steps <pass> <outcome>: every staged step in order through the delivered
# binary from the repository, each wanting the outcome; the apply pass
# records the ids the named steps minted.
steps() {
    local pass="$1" want="$2" num op name stem outcome id subst pair
    while IFS=$'\t' read -r num op name; do
        stem="${out}/docs/${pass}-${num}-${op}"
        subst=()
        for pair in "${minted[@]+"${minted[@]}"}"; do subst+=(-e "s/@MINT-${pair%%=*}@/${pair#*=}/g"); done
        if [[ "${#subst[@]}" -gt 0 ]]; then
            sed "${subst[@]}" "${out}/steps/${num}-${op}.yaml" > "${stem}.values.yaml"
        else
            cp "${out}/steps/${num}-${op}.yaml" "${stem}.values.yaml"
        fi
        set +e
        ( cd "${repo}" && "${kbase}" "${op}" --values "${stem}.values.yaml" ) < /dev/null > "${stem}.out" 2> "${stem}.log"
        set -e
        outcome="$(sed -n 's/^outcome: "\(.*\)"$/\1/p' "${stem}.out")"
        if [[ "${outcome}" == "${want}" ]]; then
            printf '%s\t%s\t%s\t%s\tpass\t%s\n' "${pass}" "${num}" "${op}" "${want}" "${stem}.out" >> "${out}/opscript.tsv"
            printf '  pass %s %s %s: %s\n' "${pass}" "${num}" "${op}" "${want}"
        else
            printf '%s\t%s\t%s\t%s\tfail\t%s\n' "${pass}" "${num}" "${op}" "${want}" "${stem}.out" >> "${out}/opscript.tsv"
            fail "${pass} ${num} ${op}: want ${want}, got ${outcome:-no outcome} — see ${stem}.out"
        fi
        if [[ "${pass}" == apply && "${name}" != "-" ]]; then
            id="$(sed -n '/^minted:/,/^adopted:/s/^    - "\(clm-[a-z0-9]*\)"$/\1/p' "${stem}.out" | head -n 1)"
            [[ -n "${id}" ]] || fail "${num} ${op} minted no id for ${name}"
            minted+=("${name}=${id}")
            printf '%s\t%s\n' "${name}" "${id}" >> "${out}/minted.txt"
        fi
    done < "${out}/steps/steps.tsv"
}
steps apply done
set +e
( cd "${repo}" && "${kbase}" verify ) > "${out}/docs/verify.out" 2> "${out}/docs/verify.log"
set -e
grep -qx 'outcome: "done"' "${out}/docs/verify.out" && grep -qx 'findings: \[\]' "${out}/docs/verify.out" \
    || fail "verify is not green after the script — see docs/verify.out"
cp -R "${repo}/kb-root" "${out}/kb-root.before-reissue"
steps reissue unchanged
diff -r "${out}/kb-root.before-reissue" "${repo}/kb-root" > "${out}/reissue.diff" \
    || fail "the re-issue moved kb-root/ — see reissue.diff"
OUT_DIR="${out}" RECIPE="test-integration-opscript-results" INVOCATION="cp -R ${RESULTS_FIXTURE_DIR}/kb-root repo/ (with a bare .git); one prose line appended to repo/kb-root/a.md\ngo test -count=1 -run '^TestStageOpScript\$' ./internal/write -args -opscript.script=${COMPAT_OP_SCRIPT} -opscript.kbroot=repo/kb-root -opscript.out=steps > stage.log\nper step NN-op of steps/steps.tsv, from repo/: ${kbase} <op> --values docs/apply-NN-<op>.values.yaml (each @MINT-<name>@ the id minted.txt records) > docs/apply-NN-<op>.out, wanting done\n${kbase} verify > docs/verify.out, wanting done with no findings; kb-root/ copied to kb-root.before-reissue/\nper step again: docs/reissue-NN-<op>.*, wanting unchanged; diff -r kb-root.before-reissue repo/kb-root > reissue.diff, wanting no difference\nopscript.tsv: per step and pass the outcome wanted, pass or fail, and its document" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-opscript-results): a step did not land as wanted — see opscript.tsv\n'
    exit 1
fi
printf 'integration(test-integration-opscript-results) ok\n'
