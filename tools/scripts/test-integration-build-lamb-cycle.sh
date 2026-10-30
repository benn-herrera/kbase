#!/usr/bin/env bash
# A cycle a build must cut, over the delivered binary: the lamb-cycle fixture,
# two lemmas whose proofs cite each other, built by ./bin/kbase build
# --no-inference in a fresh repository. Checked: the build is done; the
# depends-attributed report warns of the supports edge it cut; the KB holds one
# demoted record, from that edge's source into an analysis (analyses-split
# moved it off the lemma it cut); and verify is done, listing it as a demoted
# finding. Ids are minted per build, so every check selects by kind and
# relation. Needs pandoc.
#
# Inputs (environment): BIN_DIR; LAMB_CYCLE_FIXTURE_DIR, the fixture;
# BUILD_LAMB_CYCLE_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${LAMB_CYCLE_FIXTURE_DIR:?LAMB_CYCLE_FIXTURE_DIR is not set: the lamb-cycle fixture}"
: "${BUILD_LAMB_CYCLE_OUT_DIR:?BUILD_LAMB_CYCLE_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
out="${root}/${BUILD_LAMB_CYCLE_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
leg="${out}/cycle"
rm -rf "${leg}" "${out}/home" "${out}/config" "${out}/xdg" "${out}/legs.tsv"
mkdir -p "${leg}/repo" "${out}/home" "${out}/config"
printf 'pandoc: %s\n' "$(pandoc --version | head -n 1)"
# Git and kbase read only what this run stages: a quarantined global git
# configuration, a fixed identity for the build's commits, an empty kbase
# configuration directory and a state home under the evidence directory.
: > "${out}/home/.gitconfig"
export HOME="${out}/home" XDG_STATE_HOME="${out}/xdg" KBASE_CONFIG_DIR="${out}/config" \
    GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${out}/home/.gitconfig" \
    GIT_AUTHOR_NAME=lamb GIT_AUTHOR_EMAIL=lamb@invalid GIT_COMMITTER_NAME=lamb GIT_COMMITTER_EMAIL=lamb@invalid
# capture <name> <command...>: run from the repository, stdout to <name>.out,
# stderr to <name>.log, exit code to <name>.exit.
capture() {
    local name="$1"
    shift
    set +e
    ( cd "${leg}/repo" && "$@" ) > "${leg}/${name}.out" 2> "${leg}/${name}.log"
    printf '%s\n' "$?" > "${leg}/${name}.exit"
    set -e
}
status=0
# check <name> <command...>: one check, recorded in legs.tsv.
check() {
    local name="$1" verdict=pass
    shift
    "$@" || { verdict=fail; status=1; }
    printf 'cycle\t%s\t%s\n' "${name}" "${verdict}" >> "${out}/legs.tsv"
    printf '  %s %s\n' "${verdict}" "${name}"
}

cp "${LAMB_CYCLE_FIXTURE_DIR}/lamb-cycle.tex" "${leg}/repo/"
git -C "${leg}/repo" init -q
git -C "${leg}/repo" add -A
git -C "${leg}/repo" commit -qm "the lamb-cycle fixture"
capture build "${kbase}" build --no-inference --state-dir "${leg}/state" lamb-cycle.tex
check 'build done' grep -qx 'outcome: "done"' "${leg}/build.out"

# The cut, as depends-attributed's cycle walk warns of it: a supports edge
# between the two lemmas, both still claims at that stage.
report="${leg}/state/reports/depends-attributed.yaml"
warning="$(grep -Eo 'supports clm-[a-z0-9]+ → clm-[a-z0-9]+ is demoted, breaking the cycle [^"]*' "${report}" || true)"
check 'depends-attributed reports one cut' test "$(printf '%s' "${warning}" | grep -c .)" -eq 1
cut_source="$(printf '%s' "${warning}" | sed -n 's/^supports \(clm-[a-z0-9]*\) → .*/\1/p')"

# The record the cut left, after analyses-split moved it onto the analysis
# split from the lemma it went into.
capture edges "${kbase}" edges --relation demoted
check 'edges done' grep -qx 'outcome: "done"' "${leg}/edges.out"
check 'one demoted record' grep -qx 'count: 1' "${leg}/edges.out"
check 'the demoted record leaves the cut edge source' grep -qxF "    - source: \"${cut_source}\"" "${leg}/edges.out"
target="$(sed -n 's/^      target: "\(ana-[a-z0-9]*\)"$/\1/p' "${leg}/edges.out")"
check 'the demoted record goes into an analysis' test -n "${target}"

capture verify "${kbase}" verify
check 'verify done' grep -qx 'outcome: "done"' "${leg}/verify.out"
check 'verify lists the demoted finding' grep -qx '    - check: "demoted"' "${leg}/verify.out"
check 'the finding names the record and the relation it replaced' \
    grep -qxF "      detail: \"${cut_source} → ${target}, demoted from supports: cut by cycle breaking\"" "${leg}/verify.out"

OUT_DIR="${out}" RECIPE="test-integration-build-lamb-cycle" INVOCATION="cp ${LAMB_CYCLE_FIXTURE_DIR}/lamb-cycle.tex cycle/repo/; git init and one commit (identity fixed in the environment, global configuration quarantined to home/.gitconfig, KBASE_CONFIG_DIR the empty config/)\ncycle/repo/: ${kbase} build --no-inference --state-dir cycle/state lamb-cycle.tex > cycle/build.out\ncycle/state/reports/depends-attributed.yaml: one warning naming the supports edge the cycle walk cut\ncycle/repo/: ${kbase} edges --relation demoted > cycle/edges.out: one record, from the cut edge's source into an analysis\ncycle/repo/: ${kbase} verify > cycle/verify.out: done, with the demoted finding naming that record and supports\nlegs.tsv: per check the leg, the check, pass or fail" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-build-lamb-cycle): a check failed — see legs.tsv\n'
    exit 1
fi
printf 'integration(test-integration-build-lamb-cycle) ok\n'
