#!/usr/bin/env bash
# Init legs over the delivered binary in real git repositories, each result
# checked against its documented keys: refused outside a worktree with nothing
# created, birth, birth-once, build refused over a created KB, the
# cross-process maintenance chain closed green, the no-dot placeholder. The
# born bytes, the birth commit's subject and its trail belong to
# cmd/init_test.go and ledger_test.go; the script never inspects them.
# It needs only git — no pandoc, no provider, no adjagent clone.
#
# Inputs (environment): BIN_DIR; INIT_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${INIT_OUT_DIR:?INIT_OUT_DIR is not set: the evidence directory}"

root="$(pwd)"
out="${root}/${INIT_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
repo="${out}/repo"
docs="${out}/docs"
nodot="${out}/repo-nodot"
rm -rf "${repo}" "${docs}" "${nodot}" "${out}/outside" "${out}/gitonly" "${out}/gitconfig" "${out}/seed.yaml" "${out}/leaf.md" "${out}/mc.json" "${out}/mc-run.json" "${out}/init.tsv" "${out}/state"
mkdir -p "${docs}"
export GIT_AUTHOR_NAME=init-integration GIT_AUTHOR_EMAIL=init@invalid GIT_COMMITTER_NAME=init-integration GIT_COMMITTER_EMAIL=init@invalid GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${out}/gitconfig"
status=0
fail() { printf '  fail: %s\n' "$*"; status=1; }
# run <verb-for-keys> <outcome> <args...>: one invocation of the
# delivered binary from the repository, its result parsed against the
# verb's documented keys; LAST names the result document it captured.
# Arguments reach the repository absolute: the tool runs in a subshell.
n=0
run() {
    local verb="$1" want="$2" stem
    shift 2
    n=$((n + 1))
    stem="${docs}/$(printf '%02d' "${n}")-$1"
    set +e
    ( cd "${repo}" && "${kbase}" "$@" ) > "${stem}.out" 2> "${stem}.log"
    printf '%s\n' "$?" > "${stem}.exit"
    go test -count=1 -run '^TestResultDocument$' ./cmd -args -result.doc="${stem}.out" -result.verb="${verb}" \
        -result.outcome="${want}" > "${stem}.keys.log" 2>&1
    local rc=$?
    set -e
    printf '%s\t%s\t%s\t%s\n' "$*" "${want}" "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "${stem}.out" >> "${out}/init.tsv"
    printf '  %s %s: want %s, exit %s\n' "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "$*" "${want}" "$(cat "${stem}.exit")"
    [[ "${rc}" -eq 0 ]] || status=1
    LAST="${stem}.out"
}
# 1. outside a worktree: refused, nothing created. run() executes from
# ${repo}, so ${repo} points at mktemp's own tree — somewhere under no
# repository at all, not the project's transient dir, which a repository
# does hold. The whole run tree is copied to ${out}/outside before the
# temporary directory goes.
outside="$(mktemp -d)"
repo="${outside}"
run init refused init
[[ ! -e "${outside}/kb-root" ]] || fail "a kb-root stands outside a worktree"
mkdir -p "${out}/outside"
cp -R "${outside}/." "${out}/outside/"
rm -rf "${outside}"
repo="${out}/repo"
# 2. birth, default title: the repository's directory name. init commits
# its own birth; the commit's subject and trail are ledger's, pinned in
# ledger_test.go — the script never reads git.
git init --quiet "${repo}"
printf '%s\n' 'entry:' '- title: An Authored Result' '  rigor: 0.5' '  rationale: Written at birth.' > "${out}/seed.yaml"
printf '%s\n' 'entry:' '- document: a.md' '  id: "@ID@"' '  locator: authored text stands' > "${out}/mc.json"
printf '%s\n' '# A Leaf' '' 'The authored text stands. It cites nothing it cannot show.' > "${out}/leaf.md"
run init done init
# verify green over the born tree as shipped, before any maintenance
# step reaches it.
run verify done verify
# 3. re-issue: birth-once.
run init refused init
# 4. build stays refused over a created KB. The store is quarantined to
# state/: build opens and creates it before its first refusal, so nothing
# reaches the real $XDG_STATE_HOME.
printf '%s\n' '\documentclass{article}' '\begin{document}' 'Nothing distilled.' '\end{document}' > "${repo}/paper.tex"
run build refused build "${repo}/paper.tex" --no-inference --state-dir "${out}/state"
# 5. the cross-process maintenance chain: the id the insert minted
# reaches the later invocations through its document, the claim is
# hosted by the marker mark-claim-in-leaf places, and the loop closes
# green on what init wrote.
run insert-claim-entry done insert-claim-entry --values "${out}/seed.yaml"
id="$(sed -n 's/^    - "\(clm-[a-z0-9]*\)"$/\1/p' "${LAST}" | head -n 1)"
[[ -n "${id}" ]] || fail "the insert minted no id"
cp "${out}/leaf.md" "${repo}/kb-root/a.md"
sed "s/@ID@/${id}/g" "${out}/mc.json" > "${out}/mc-run.json"
run mark-claim-in-leaf done mark-claim-in-leaf --values "${out}/mc-run.json"
run refresh unchanged refresh
run verify done verify
# 6. no dot: the placeholder stands, in its own repository, PATH holding
# git alone through a symlink of its own.
git init --quiet "${nodot}"
mkdir -p "${out}/gitonly"
ln -s "$(command -v git)" "${out}/gitonly/git"
( cd "${nodot}" && env PATH="${out}/gitonly" "${kbase}" init ) > "${docs}/init-nodot.out" 2> "${docs}/init-nodot.log"
grep -q 'outcome: "done"' "${docs}/init-nodot.out" || fail "the no-dot birth did not reach done: $(cat "${docs}/init-nodot.out")"
grep -q 'no Graphviz dot on PATH' "${docs}/init-nodot.log" || fail "the no-dot lack is not said on stderr"
grep -q 'no Graphviz dot on PATH' "${nodot}/kb-root/claim-graph.svg" || fail "claim-graph.svg is not the placeholder"
OUT_DIR="${out}" RECIPE="test-integration-init" INVOCATION="git init repo/ (identity fixed in the environment, global config quarantined to gitconfig)\nper invocation NN-<verb>, from repo/: ${kbase} <args> > docs/NN-<verb>.out, parsed by go test -run '^TestResultDocument\$' ./cmd -args -result.doc=docs/NN-<verb>.out -result.verb=<verb> -result.outcome=<outcome> > docs/NN-<verb>.keys.log\noutside a worktree: invocation 01-init run from a mktemp dir, whose run tree is copied to outside/; state/: the refused build's quarantined store\ndocs/init-nodot.{out,log}: init run from repo-nodot/ with PATH holding git alone\ninit.tsv: per invocation its arguments, the outcome wanted, pass or fail, and its document" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-init): a birth check failed — see init.tsv\n'
    exit 1
fi
printf 'integration(test-integration-init) ok\n'
