#!/usr/bin/env bash
# The compatibility op script over the delivered binary: TestStageOpScript
# renders test-data/fixtures/compat/ops.yaml against a scratch copy of the
# results fixture, every step's batch runs through ./bin/kbase write to done,
# verify is green, and every batch re-issued reports the outcome its step
# states, refused or unchanged, a refused one's first refusal detail starting
# with the prefix the step states, with kb-root/ byte-identical.
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
# The fixture's one leaf holds no line the staging can choose, so the
# scratch copy's gains one; the script hosts its claims there.
printf '\n%s\n' 'The probe rests on a line the snippet names once.' >> "${repo}/kb-root/a.md"
# The 3.0.0 copy committed and converted before the first write: every other
# write is refused while its conversion is pending.
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
KBASE="${kbase}" REPO="${repo}" OUT_DIR="${out}" RECIPE="test-integration-opscript-results" COMMIT_MESSAGE="the results fixture as staged" tools/scripts/stage-converted-copy.sh
go test -count=1 -run '^TestStageOpScript$' ./internal/write -args -opscript.script="${root}/${COMPAT_OP_SCRIPT}" \
    -opscript.kbroot="${repo}/kb-root" -opscript.out="${out}/steps" > "${out}/stage.log" 2>&1 \
    || { printf 'integration(test-integration-opscript-results): staging failed — see stage.log\n'; exit 1; }
[[ -s "${out}/steps/steps.tsv" ]] || { printf 'integration(test-integration-opscript-results): staging wrote no steps — see stage.log\n'; exit 1; }
status=0
fail() { printf '  fail: %s\n' "$*"; status=1; }
# minted holds name=id for each step that names its id; a later step's
# @MINT-<name>@ tokens are replaced by it before the step runs.
minted=()
# steps <pass>: every staged batch in order through the delivered binary's
# kbase write from the repository, the apply pass wanting done and the
# reissue pass the outcome steps.tsv states, a refused one's first refusal
# detail starting with the prefix steps.tsv states; the apply pass records the
# id each named step's one entry minted.
steps() {
    local pass="$1" num name reissue prefix want stem outcome detail verdict ids subst pair
    while IFS=$'\t' read -r num name reissue prefix; do
        want=done
        [[ "${pass}" == reissue ]] && want="${reissue}"
        stem="${out}/docs/${pass}-${num}"
        subst=()
        for pair in "${minted[@]+"${minted[@]}"}"; do subst+=(-e "s/@MINT-${pair%%=*}@/${pair#*=}/g"); done
        if [[ "${#subst[@]}" -gt 0 ]]; then
            sed "${subst[@]}" "${out}/steps/${num}.yaml" > "${stem}.values.yaml"
        else
            cp "${out}/steps/${num}.yaml" "${stem}.values.yaml"
        fi
        set +e
        ( cd "${repo}" && "${kbase}" write --values "${stem}.values.yaml" ) < /dev/null > "${stem}.out" 2> "${stem}.log"
        set -e
        outcome="$(sed -n 's/^outcome: "\(.*\)"$/\1/p' "${stem}.out")"
        verdict=pass
        if [[ "${outcome}" != "${want}" ]]; then
            verdict=fail
            fail "${pass} ${num}: want ${want}, got ${outcome:-no outcome} — see ${stem}.out"
        elif [[ "${pass}" == reissue && "${want}" == refused ]]; then
            detail="$(awk '/^refusals:/ { r = 1 } r && sub(/^      detail: "/, "") { sub(/"$/, ""); print; exit }' "${stem}.out")"
            if [[ "${detail}" != "${prefix}"* ]]; then
                verdict=fail
                fail "${pass} ${num}: want a first refusal detail starting \"${prefix}\", got \"${detail}\" — see ${stem}.out"
            fi
        fi
        printf '%s\t%s\t%s\t%s\t%s\n' "${pass}" "${num}" "${want}" "${verdict}" "${stem}.out" >> "${out}/opscript.tsv"
        [[ "${verdict}" == fail ]] || printf '  pass %s %s: %s\n' "${pass}" "${num}" "${want}"
        if [[ "${pass}" == apply && "${name}" != "-" ]]; then
            ids="$(tools/scripts/entry-ids.sh "${stem}.out" "")"
            [[ -n "${ids}" && "${ids}" != *$'\n'* ]] || fail "${num} minted not one id for ${name}: ${ids:-none}"
            minted+=("${name}=${ids}")
            printf '%s\t%s\n' "${name}" "${ids}" >> "${out}/minted.txt"
        fi
    done < "${out}/steps/steps.tsv"
}
steps apply
set +e
( cd "${repo}" && "${kbase}" verify ) > "${out}/docs/verify.out" 2> "${out}/docs/verify.log"
set -e
grep -qx 'outcome: "done"' "${out}/docs/verify.out" && grep -qx 'findings: \[\]' "${out}/docs/verify.out" \
    || fail "verify is not green after the script — see docs/verify.out"
cp -R "${repo}/kb-root" "${out}/kb-root.before-reissue"
steps reissue
diff -r "${out}/kb-root.before-reissue" "${repo}/kb-root" > "${out}/reissue.diff" \
    || fail "the re-issue moved kb-root/ — see reissue.diff"
OUT_DIR="${out}" RECIPE="test-integration-opscript-results" INVOCATION="cp -R ${RESULTS_FIXTURE_DIR}/kb-root repo/ (with a bare .git); one prose line appended to repo/kb-root/a.md; git add kb-root and commit; from repo/: ${kbase} update-format > update-format.out\ngo test -count=1 -run '^TestStageOpScript\$' ./internal/write -args -opscript.script=${COMPAT_OP_SCRIPT} -opscript.kbroot=repo/kb-root -opscript.out=steps > stage.log\nper step NN of steps/steps.tsv, from repo/: ${kbase} write --values docs/apply-NN.values.yaml (each @MINT-<name>@ the id minted.txt records) > docs/apply-NN.out, wanting done\n${kbase} verify > docs/verify.out, wanting done with no findings; kb-root/ copied to kb-root.before-reissue/\nper step again: docs/reissue-NN.*, wanting the outcome steps.tsv states, refused or unchanged, and a refused one's first refusal detail starting with the prefix steps.tsv states; diff -r kb-root.before-reissue repo/kb-root > reissue.diff, wanting no difference\nopscript.tsv: per step and pass the outcome wanted, pass or fail, and its result" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-opscript-results): a step did not land as wanted — see opscript.tsv\n'
    exit 1
fi
printf 'integration(test-integration-opscript-results) ok\n'
