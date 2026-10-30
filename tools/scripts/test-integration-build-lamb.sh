#!/usr/bin/env bash
# The lamb fixture's four properties over the delivered binary, each leg a
# ./bin/kbase build --no-inference in a fresh repository. The bib leg builds
# the fixture as committed — lamb.tex with lamb.bib and stray.bib beside it —
# and checks: the colon title reaching the volume heading, its directory slug
# and the entry point's link text unquoted, verify done (1); an empty claim
# graph from a done build (2); the citation resolved through the .bib offered
# beside the paper into a references leaf the volume lists, stray.bib's
# uncited entry reaching no document (3). The bbl leg stages a copy of the
# paper that inputs lamb.bbl, no .bib beside it, and checks what kbase does
# with bibtex's output (4): a done build and verify, no claim, no references
# leaf, the citation left as its key, and the bibliography's entry text in a
# leaf of the volume. Needs pandoc.
#
# Inputs (environment): BIN_DIR; LAMB_FIXTURE_DIR, the fixture;
# BUILD_LAMB_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${LAMB_FIXTURE_DIR:?LAMB_FIXTURE_DIR is not set: the lamb fixture}"
: "${BUILD_LAMB_OUT_DIR:?BUILD_LAMB_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
out="${root}/${BUILD_LAMB_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
fixture="${root}/${LAMB_FIXTURE_DIR}"
rm -rf "${out}/bib" "${out}/bbl" "${out}/home" "${out}/config" "${out}/xdg" "${out}/legs.tsv"
mkdir -p "${out}/bib/repo" "${out}/bbl/repo" "${out}/home" "${out}/config"
printf 'pandoc: %s\n' "$(pandoc --version | head -n 1)"
# Git and kbase read only what this run stages: a quarantined global git
# configuration, a fixed identity for the build's commits, an empty kbase
# configuration directory and a state home under the evidence directory.
: > "${out}/home/.gitconfig"
export HOME="${out}/home" XDG_STATE_HOME="${out}/xdg" KBASE_CONFIG_DIR="${out}/config" \
    GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${out}/home/.gitconfig" \
    GIT_AUTHOR_NAME=lamb GIT_AUTHOR_EMAIL=lamb@invalid GIT_COMMITTER_NAME=lamb GIT_COMMITTER_EMAIL=lamb@invalid
# capture <leg> <name> <command...>: run from the leg's repository, stdout to
# <leg>/<name>.out, stderr to .log, exit code to .exit.
capture() {
    local leg="$1" name="$2"
    shift 2
    set +e
    ( cd "${out}/${leg}/repo" && "$@" ) > "${out}/${leg}/${name}.out" 2> "${out}/${leg}/${name}.log"
    printf '%s\n' "$?" > "${out}/${leg}/${name}.exit"
    set -e
}
status=0
# check <leg> <name> <command...>: one check, recorded in legs.tsv.
check() {
    local leg="$1" name="$2" verdict=pass
    shift 2
    "$@" || { verdict=fail; status=1; }
    printf '%s\t%s\t%s\n' "${leg}" "${name}" "${verdict}" >> "${out}/legs.tsv"
    printf '  %s %s: %s\n' "${verdict}" "${leg}" "${name}"
}
# absent <pattern> <dir>: no file under dir holds the fixed string.
absent() { ! grep -rqF "$1" "$2"; }
# build <leg>: commit the staged sources, build lamb.tex, then verify and stats.
build() {
    local leg="$1"
    git -C "${out}/${leg}/repo" init -q
    git -C "${out}/${leg}/repo" add -A
    git -C "${out}/${leg}/repo" commit -qm "the lamb fixture, ${leg} leg"
    capture "${leg}" build "${kbase}" build --no-inference --state-dir "${out}/${leg}/state" lamb.tex
    capture "${leg}" verify "${kbase}" verify
    capture "${leg}" stats "${kbase}" stats
    check "${leg}" 'build done' grep -qx 'outcome: "done"' "${out}/${leg}/build.out"
    check "${leg}" 'verify done' grep -qx 'outcome: "done"' "${out}/${leg}/verify.out"
    check "${leg}" 'no claim' grep -qx '    claims: 0' "${out}/${leg}/stats.out"
    check "${leg}" 'no edge' grep -qx '    edges: 0' "${out}/${leg}/stats.out"
}

title='Mary and Her Lamb: An Account'
slug='mary-and-her-lamb-an-account'

# The bib leg: the fixture as committed.
cp "${fixture}/lamb.tex" "${fixture}/lamb.bib" "${fixture}/stray.bib" "${out}/bib/repo/"
build bib
kb="${out}/bib/repo/kb-root"
check bib 'the entry point links the volume by its title' grep -qxF -e "- [${title}](${slug}/index.md)" "${kb}/entry-point.md"
check bib 'the volume heading is the title' grep -qxF "# ${title}" "${kb}/${slug}/index.md"
check bib 'the volume lists a references leaf' grep -qxF -e '- [References](references.md)' "${kb}/${slug}/index.md"
check bib 'the references leaf holds the cited entry' grep -qF 'id="ref-nobody2026"' "${kb}/${slug}/references.md"
check bib 'the citation renders through the bibliography' grep -rqF 'data-cites="nobody2026">(Nobody 2026)</span>' "${kb}/${slug}"
check bib 'the uncited stray entry reaches no document' absent 'strayentry1999' "${kb}"
check bib 'the uncited stray title reaches no document' absent 'A Work This Paper Does Not Cite' "${kb}"

# The bbl leg: a copy of the paper inputting lamb.bbl before its end, and no
# .bib beside it, so no bibliography is offered.
cp "${fixture}/lamb.bbl" "${out}/bbl/repo/"
awk '/^\\end\{document\}$/ { print "\\input{lamb.bbl}" } { print }' "${fixture}/lamb.tex" > "${out}/bbl/repo/lamb.tex"
build bbl
kb="${out}/bbl/repo/kb-root"
check bbl 'no references leaf' test ! -e "${kb}/${slug}/references.md"
check bbl 'the citation stays its key' grep -rqF 'data-cites="nobody2026">(nobody2026)</span>' "${kb}/${slug}"
check bbl 'the bibliography entry text reaches a leaf' grep -rqF 'Nobody. *Nothing*. 2026.' "${kb}/${slug}"

OUT_DIR="${out}" RECIPE="test-integration-build-lamb" INVOCATION="per leg, <leg>/repo/ with its own .git and one commit (identity fixed in the environment, global configuration quarantined to home/.gitconfig, KBASE_CONFIG_DIR the empty config/)\nbib: cp ${LAMB_FIXTURE_DIR}/{lamb.tex,lamb.bib,stray.bib} bib/repo/\nbbl: cp ${LAMB_FIXTURE_DIR}/lamb.bbl bbl/repo/; bbl/repo/lamb.tex is ${LAMB_FIXTURE_DIR}/lamb.tex with \\\\input{lamb.bbl} before \\\\end{document}\nper leg, <leg>/repo/: ${kbase} build --no-inference --state-dir <leg>/state lamb.tex > <leg>/build.out; ${kbase} verify > <leg>/verify.out; ${kbase} stats > <leg>/stats.out\nbib: the title unquoted in kb-root/entry-point.md and the volume heading under its slug, a references leaf the volume lists, the citation rendered, stray.bib's entry under no file of kb-root/\nbbl: no references leaf, the citation left as its key, the entry text in a leaf\nlegs.tsv: per check the leg, the check, pass or fail" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-build-lamb): a check failed — see legs.tsv\n'
    exit 1
fi
printf 'integration(test-integration-build-lamb) ok\n'
