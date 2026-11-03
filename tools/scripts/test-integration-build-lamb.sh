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
# leaf of the volume. Each leg also checks that no title stands twice within a
# kind, and that every kb-build: commit holding a KB holds the derived files
# its records derive (check-build-trail.sh). The rewind leg then runs, in the
# bib leg's repository, the commands build-trail gives for a mid-trail
# boundary and for started-at, each followed by the build it sets up. Needs
# pandoc.
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
rm -rf "${out}/bib" "${out}/bbl" "${out}/rewind" "${out}/home" "${out}/config" "${out}/xdg" "${out}/legs.tsv"
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
    git -C "${out}/${leg}/repo" ls-tree -r --name-only HEAD > "${out}/${leg}/committed.txt"
    check "${leg}" 'the build commits its four records under kb-root/.build/' \
        test "$(grep -cE '^kb-root/\.build/kb-build-(node-pass|classification|unmarked|cull)\.yaml$' "${out}/${leg}/committed.txt")" -eq 4
    check "${leg}" 'no build record at the repository root' absent 'kb-build-' <(grep -v '^kb-root/' "${out}/${leg}/committed.txt")
    # Titles are unique within each kind: no kind-and-title pair twice among
    # find's rows.
    capture "${leg}" find "${kbase}" find
    check "${leg}" 'find done' grep -qx 'outcome: "done"' "${out}/${leg}/find.out"
    check "${leg}" 'no title twice within a kind' \
        test -z "$(awk '/^ +node_type: /{k=$2} /^ +title: /{print k, $0}' "${out}/${leg}/find.out" | sort | uniq -d)"
    check "${leg}" 'every kb-build: commit holding a KB derives unchanged' \
        tools/scripts/check-build-trail.sh "${out}/${leg}/repo" "${out}/${leg}" "${kbase}"
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

# trail_commands <document> <index|started-at>: the commands build-trail's
# document gives the boundary at that 0-based index, oldest first, or
# started-at, one per line as the document's double-quoted strings hold them.
trail_commands() {
    awk -v want="$2" '
      /^[^ ]/ { top = $0; sub(/:.*/, "", top); n = -1; listing = 0; next }
      top == "boundaries" && /^    - / { n++; listing = 0 }
      /^ +command:$/ { listing = (top == "boundaries" && n == want + 0 && want != "started-at") || (top == "started-at" && want == "started-at"); next }
      listing && /^ +- "/ { s = $0; sub(/^ +- "/, "", s); sub(/"$/, "", s); gsub(/\\"/, "\"", s); gsub(/\\\\/, "\\", s); print s; next }
      /^ +[^ -]/ { listing = 0 }
    ' "$1"
}
# rewind_run <name> <command...>: run from the bib leg's repository with the
# delivered binary first on PATH, so kbase in a command is that binary;
# outputs under rewind/ as capture's.
rewind_run() {
    local name="$1"
    shift
    set +e
    ( cd "${out}/bib/repo" && PATH="${root}/${BIN_DIR}:${PATH}" "$@" ) > "${out}/rewind/${name}.out" 2> "${out}/rewind/${name}.log"
    printf '%s\n' "$?" > "${out}/rewind/${name}.exit"
    set -e
}

# The rewind leg, over the bib leg's repository and state store as its build
# left them: build-trail's commands for the boundary at index n/2 (rounded
# down) of its n, oldest first, run as the document gives them — the reset,
# then the resume, done, re-recording the bib leg's kb-build: subjects from
# that boundary on, every commit holding a KB deriving unchanged — then
# started-at's command and a fresh build, done.
mkdir -p "${out}/rewind"
git -C "${out}/bib/repo" log --reverse --format='%s' --grep='^kb-build: ' > "${out}/rewind/subjects-before.txt"
rewind_run build-trail kbase build-trail --state-dir "${out}/bib/state"
check rewind 'build-trail done' grep -qx 'outcome: "done"' "${out}/rewind/build-trail.out"
boundaries="$(grep -c '^    - stage: ' "${out}/rewind/build-trail.out" || true)"
middle=$(( boundaries / 2 ))
trail_commands "${out}/rewind/build-trail.out" "${middle}" > "${out}/rewind/boundary-commands.txt"
trail_commands "${out}/rewind/build-trail.out" started-at > "${out}/rewind/started-at-commands.txt"
check rewind 'the boundary gives the reset, then the resume' test "$(grep -c . "${out}/rewind/boundary-commands.txt")" -eq 2
check rewind "started-at gives the reset alone" test "$(grep -c . "${out}/rewind/started-at-commands.txt")" -eq 1
step=0
while IFS= read -r command; do
    step=$(( step + 1 ))
    rewind_run "boundary-${step}" bash -c "${command}"
done < "${out}/rewind/boundary-commands.txt"
check rewind 'the reset to the boundary exits 0' test "$(cat "${out}/rewind/boundary-1.exit")" -eq 0
check rewind 'the resume is done' grep -qx 'outcome: "done"' "${out}/rewind/boundary-2.out"
git -C "${out}/bib/repo" log --reverse --format='%s' --grep='^kb-build: ' > "${out}/rewind/subjects-after.txt"
check rewind 'the kb-build: subjects from the boundary on are the bib leg'"'"'s' \
    cmp -s <(tail -n "+$(( middle + 1 ))" "${out}/rewind/subjects-before.txt") <(tail -n "+$(( middle + 1 ))" "${out}/rewind/subjects-after.txt")
check rewind 'every kb-build: commit holding a KB derives unchanged' \
    tools/scripts/check-build-trail.sh "${out}/bib/repo" "${out}/rewind" "${kbase}"
rewind_run started-at bash -c "$(cat "${out}/rewind/started-at-commands.txt")"
check rewind "started-at's reset exits 0" test "$(cat "${out}/rewind/started-at.exit")" -eq 0
rewind_run fresh-build kbase build --no-inference --state-dir "${out}/bib/state" lamb.tex
check rewind 'a fresh build after it is done' grep -qx 'outcome: "done"' "${out}/rewind/fresh-build.out"

# The bbl leg: a copy of the paper inputting lamb.bbl before its end, and no
# .bib beside it, so no bibliography is offered.
cp "${fixture}/lamb.bbl" "${out}/bbl/repo/"
awk '/^\\end\{document\}$/ { print "\\input{lamb.bbl}" } { print }' "${fixture}/lamb.tex" > "${out}/bbl/repo/lamb.tex"
build bbl
kb="${out}/bbl/repo/kb-root"
check bbl 'no references leaf' test ! -e "${kb}/${slug}/references.md"
check bbl 'the citation stays its key' grep -rqF 'data-cites="nobody2026">(nobody2026)</span>' "${kb}/${slug}"
check bbl 'the bibliography entry text reaches a leaf' grep -rqF 'Nobody. *Nothing*. 2026.' "${kb}/${slug}"

OUT_DIR="${out}" RECIPE="test-integration-build-lamb" INVOCATION="per leg, <leg>/repo/ with its own .git and one commit (identity fixed in the environment, global configuration quarantined to home/.gitconfig, KBASE_CONFIG_DIR the empty config/)\nbib: cp ${LAMB_FIXTURE_DIR}/{lamb.tex,lamb.bib,stray.bib} bib/repo/\nbbl: cp ${LAMB_FIXTURE_DIR}/lamb.bbl bbl/repo/; bbl/repo/lamb.tex is ${LAMB_FIXTURE_DIR}/lamb.tex with \\\\input{lamb.bbl} before \\\\end{document}\nper leg, <leg>/repo/: ${kbase} build --no-inference --state-dir <leg>/state lamb.tex > <leg>/build.out; ${kbase} verify > <leg>/verify.out; ${kbase} stats > <leg>/stats.out; ${kbase} find > <leg>/find.out, no kind-and-title pair twice\nper leg, tools/scripts/check-build-trail.sh <leg>/repo <leg> ${kbase}: per kb-build: commit holding kb-root/.index/format.yaml, git archive <commit> kb-root into <leg>/commits/<commit>/ beside an empty .git; there ${kbase} write --dry-run --values <leg>/empty-batch.yaml (entry: []) > <leg>/commits/<commit>.out; <leg>/commits.tsv: each commit and its outcome, every one unchanged\nbib: the title unquoted in kb-root/entry-point.md and the volume heading under its slug, a references leaf the volume lists, the citation rendered, stray.bib's entry under no file of kb-root/\nrewind, in bib/repo/ with ${BIN_DIR} first on PATH: kbase build-trail --state-dir bib/state > rewind/build-trail.out; the two commands of the boundary at index n/2 (rounded down) of n, oldest first, each run by bash -c as the document gives it > rewind/boundary-<1|2>.out, the reset exiting 0 and the resume done; rewind/subjects-after.txt equal to rewind/subjects-before.txt (git log --reverse --format=%s --grep='^kb-build: ') from that boundary on; tools/scripts/check-build-trail.sh bib/repo rewind ${kbase}; started-at's command by bash -c > rewind/started-at.out, exiting 0; kbase build --no-inference --state-dir bib/state lamb.tex > rewind/fresh-build.out, done\nbbl: no references leaf, the citation left as its key, the entry text in a leaf\nlegs.tsv: per check the leg, the check, pass or fail" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-build-lamb): a check failed — see legs.tsv\n'
    exit 1
fi
printf 'integration(test-integration-build-lamb) ok\n'
