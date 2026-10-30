#!/usr/bin/env bash
# The result contract over the delivered binary: every maintenance subcommand,
# status and cancel run by ./bin/kbase over a copy of a committed fixture KB,
# each result parsed against its subcommand's documented keys and outcome, and
# probe's decision selection run against a Go stub that answers the decision
# surface and refuses the chat path. The copy is staged through the write ops
# with an edge of every relation a public op writes; one freehand edit then
# closes a cycle, which refresh cuts into a demoted record that verify, the
# edge queries, stats and resolve-demoted are checked over.
# It needs git and the Go toolchain, which the build already uses — no pandoc,
# no provider, no adjagent clone — so it joins the omnibus.
#
# Inputs (environment): BIN_DIR; RESULTS_FIXTURE_DIR, the committed fixture;
# RESULTS_FIXTURE_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${RESULTS_FIXTURE_DIR:?RESULTS_FIXTURE_DIR is not set: the committed results fixture}"
: "${RESULTS_FIXTURE_OUT_DIR:?RESULTS_FIXTURE_OUT_DIR is not set: the evidence directory}"

root="$(pwd)"
out="${RESULTS_FIXTURE_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
fixture="${root}/${RESULTS_FIXTURE_DIR}"
# The configuration-directory variable is scrubbed for the whole script: every
# leg below reads the staged tree and no directory the host exports.
unset KBASE_CONFIG_DIR
# The state home every kbase call runs under, so the credentials store each
# reads is one under the evidence directory, never the operator's own.
export XDG_STATE_HOME="${root}/${out}/xdg"
repo="${out}/repo"
docs="${out}/docs"
rm -rf "${repo}" "${docs}" "${out}/values" "${out}/checks.tsv" "${out}/score-config" \
    "${out}/score-state" "${out}/probe-config" "${out}/probestub" \
    "${out}/probestub.url" "${out}/probestub.log" "${XDG_STATE_HOME}"
mkdir -p "${repo}/.git/objects" "${repo}/.git/refs/heads" "${docs}"
printf 'ref: refs/heads/main\n' > "${repo}/.git/HEAD"
cp -R "${fixture}/kb-root" "${repo}/"
status=0
n=0
LAST=""
# run <verb-for-keys> <outcome> <args...>: one invocation from the
# repository, its result parsed against the verb's documented keys.
run() {
    local verb="$1" want="$2" stem
    shift 2
    n=$((n + 1))
    stem="${docs}/$(printf '%02d' "${n}")-${1}"
    set +e
    ( cd "${repo}" && "${kbase}" "$@" ) > "${stem}.out" 2> "${stem}.log"
    printf '%s\n' "$?" > "${stem}.exit"
    go test -count=1 -run '^TestResultDocument$' ./cmd -args -result.doc="${root}/${stem}.out" -result.verb="${verb}" \
        -result.outcome="${want}" > "${stem}.keys.log" 2>&1
    local rc=$?
    set -e
    printf '%s\t%s\t%s\t%s\n' "$*" "${want}" "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "${stem}.out" >> "${out}/checks.tsv"
    printf '  %s %s: want %s, exit %s\n' "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "$*" "${want}" "$(cat "${stem}.exit")"
    [[ "${rc}" -eq 0 ]] || status=1
    LAST="${stem}.out"
}
run verify done verify
run refresh done refresh
run refresh unchanged refresh
# refresh renders every sheet on each call: drawn wherever the host has dot.
sheet=placeholder
command -v dot > /dev/null && sheet=drawn
grep -qx "sheet: \"${sheet}\"" "${LAST}" || { printf '  fail: refresh does not report sheet %s\n' "${sheet}"; status=1; }
run verify done verify
run insert-claim-entry done insert-claim-entry --values "${fixture}/values/insert.yaml"
run insert-claim-entry unchanged insert-claim-entry --no-refresh --values "${fixture}/values/insert.yaml"
# The edges every later leg reads: a work, an analysis and one edge of each
# relation a public op writes, pending and numeric bearings both, from the
# fixture's values files with the minted ids substituted — ${ID1} and ${ID2}
# the claims in id order, ${ID3} the analysis once its insert reports it —
# then both claims marked in a.md, so verify reads the staged KB whole.
ids=()
while IFS= read -r i; do ids+=("${i}"); done < <(sed -n '/^ids:/,/^minted:/s/^    - "\(clm-[a-z0-9]*\)"$/\1/p' "${docs}/05-insert-claim-entry.out" | sort)
[[ "${#ids[@]}" -eq 2 ]] || { printf 'integration(results-fixture): the insert reported no two claim ids\n'; exit 1; }
mkdir -p "${out}/values"
# staged <file>: the fixture's values file, its ids substituted, under values/;
# prints the copy's path.
staged() {
    sed -e "s/\${ID1}/${ids[0]}/g" -e "s/\${ID2}/${ids[1]}/g" -e "s/\${ID3}/${ids[2]-}/g" \
        "${fixture}/values/${1}" > "${out}/values/${1}"
    printf '%s' "${root}/${out}/values/${1}"
}
# holds <what> <command...>: a check on the last document; a failure names what.
holds() {
    local what="$1"
    shift
    "$@" || { printf '  fail: %s\n' "${what}"; status=1; }
}
run insert-work-entry done insert-work-entry --values "$(staged insert-work.yaml)"
run add-rests-on done add-rests-on --values "$(staged add-rests-on.yaml)"
run insert-analysis-entry done insert-analysis-entry --values "$(staged insert-analysis.yaml)"
ids+=("$(sed -n '/^ids:/,/^minted:/s/^    - "\(ana-[a-z0-9]*\)"$/\1/p' "${LAST}")")
[[ -n "${ids[2]}" ]] || { printf 'integration(results-fixture): the analysis insert reported no id\n'; exit 1; }
run add-pairs done add-pairs --values "$(staged add-pairs.yaml)"
run mark-claim-in-leaf done mark-claim-in-leaf --values "$(staged mark-claim-in-leaf.yaml)"
run verify done verify
run score done score --dry-run --pass rigor --state-dir "${root}/${out}/state"
# Score's role is the KB's own declaration. A configuration that names
# decision and text but no scoring refuses before the first item,
# naming models.scoring — unless --scoring-model names a model, which
# supplies the role the files leave unset.
score_config="${root}/${out}/score-config"
mkdir -p "${score_config}"
printf '%s\n' '[unreachable]' 'baseUrl = "http://127.0.0.1:1/v1"' > "${score_config}/providers.toml"
printf '%s\n' '[models]' 'decision = "unreachable/d"' 'text = "unreachable/t"' > "${score_config}/config.toml"
score_state="${root}/${out}/score-state"
run score refused score --pass rigor --config-dir "${score_config}" --state-dir "${score_state}"
grep -qF 'key: "models.scoring"' "${LAST}" || { printf '  fail: the absent scoring role is not named\n'; status=1; }
# The flag naming a model stands in for the role the files leave unset: the
# run reaches the pool's entry, few enough items for --limit to bound it,
# and the roles record says the flag named provider and model.
run score bounded score --pass rigor --limit 1 --config-dir "${score_config}" --scoring-model unreachable/judge --state-dir "${score_state}"
grep -qx '      provider-rule: "flag"' "${LAST}" || { printf '  fail: the roles record does not say the flag supplied the role\n'; status=1; }
grep -qx '      model: "judge"' "${LAST}" || { printf '  fail: the roles record does not name the flag\x27s model\n'; status=1; }
# The role configured, the flag naming a provider the pool lacks: the
# flag's refusal, before the first item, with the pool's entries listed.
printf '%s\n' '[models]' 'decision = "unreachable/d"' 'text = "unreachable/t"' 'scoring = "unreachable/s"' > "${score_config}/config.toml"
run score refused score --pass rigor --config-dir "${score_config}" --scoring-model elsewhere/judge --state-dir "${score_state}"
grep -qF 'key: "--scoring-model"' "${LAST}" || { printf '  fail: a missing provider is not named as the flag\x27s\n'; status=1; }
grep -qF '"unreachable"' "${LAST}" || { printf '  fail: the pool is not listed\n'; status=1; }
# A flag that parses and names the pool's entry reaches it: the endpoint
# nothing listens on fails the one item and --limit stops the walk, so the
# run is bounded; the roles record says the flag named provider and model.
run score bounded score --pass rigor --limit 1 --config-dir "${score_config}" --scoring-model unreachable/judge --state-dir "${score_state}"
grep -qx '      provider-rule: "flag"' "${LAST}" || { printf '  fail: the roles record does not say the flag named the provider\n'; status=1; }
grep -qx '      model: "judge"' "${LAST}" || { printf '  fail: the roles record does not name the flag\x27s model\n'; status=1; }
# The files alone, no flag naming a model: text and scoring name different
# models on the unreachable entry, the one item fails and --limit bounds the
# run. The roles record and the ledger's lines name the scoring model; text's
# appears nowhere on stdout, resume included, nor in the ledger. The
# directory is typed relative to the repository, and resume writes it
# absolute.
printf '%s\n' '[models]' 'decision = "unreachable/d"' 'text = "unreachable/file-text"' 'scoring = "unreachable/file-scoring"' \
    > "${score_config}/config.toml"
run score bounded score --pass rigor --limit 1 --config-dir ../score-config --state-dir "${score_state}"
holds 'the roles record names no scoring role' grep -qx '    - role: "scoring"' "${LAST}"
holds 'the scoring role does not name the entry' grep -qx '      provider: "unreachable"' "${LAST}"
holds 'the scoring role does not say config named it' grep -qx '      provider-rule: "config"' "${LAST}"
holds 'the scoring role does not name the scoring model' grep -qx '      model: "file-scoring"' "${LAST}"
if grep -qF file-text "${LAST}"; then printf '  fail: the text model is named on stdout\n'; status=1; fi
file_resume="$(grep '^resume: ' "${LAST}" || true)"
[[ "${file_resume}" == *"--config-dir ${score_config}"* ]] || { printf '  fail: resume does not write --config-dir absolute\n'; status=1; }
file_ledger="$(sed -n 's/^ledger: "\(.*\)"$/\1/p' "${LAST}" | head -n 1)"
holds 'the run left no ledger' test -n "${file_ledger}" -a -f "${file_ledger}/ledger.jsonl"
holds 'the ledger does not name the scoring model' grep -qF '"model":"file-scoring"' "${file_ledger}/ledger.jsonl"
if grep -qF file-text "${file_ledger}/ledger.jsonl"; then printf '  fail: the ledger names the text model\n'; status=1; fi

# Probe's decision selection asks the surface its entry declares. The stub
# answers /systemone and refuses every other path, so a probe that took the
# chat path is answered 400 and its leg fails; the document is parsed
# against the probe keys, and a surface that streams nothing reports no
# first token and no finish reason.
go build -o "${out}/probestub" ./tools/probestub
"${out}/probestub" > "${out}/probestub.url" 2> "${out}/probestub.log" &
stub_pid=$!
trap 'kill "${stub_pid}" 2>/dev/null || true' EXIT
for _ in $(seq 1 100); do
    [[ -s "${out}/probestub.url" ]] && break
    sleep 0.05
done
probe_url="$(head -n 1 "${out}/probestub.url")"
[[ -n "${probe_url}" ]] || { printf '  fail: the probe stub printed no base URL\n'; status=1; }
probe_config="${root}/${out}/probe-config"
mkdir -p "${probe_config}"
printf '%s\n' '[stub]' "baseUrl = \"${probe_url}\"" > "${probe_config}/providers.toml"
printf '%s\n' '[models]' 'decision = "stub/d"' > "${probe_config}/config.toml"
run probe done probe --config-dir "${probe_config}" --decision
grep -qx 'selection: "decision"' "${LAST}" || { printf '  fail: the probe did not ask the decision selection\n'; status=1; }
grep -qx 'first-token-ms: 0' "${LAST}" || { printf '  fail: a decision surface reports no first token\n'; status=1; }
grep -qx 'finish-reason: null' "${LAST}" || { printf '  fail: a decision surface reports no finish reason\n'; status=1; }
kill "${stub_pid}" 2>/dev/null || true
wait "${stub_pid}" 2>/dev/null || true
trap - EXIT

run set-rigor refused set-rigor --values "${fixture}/values/bad-rigor.yaml"
run render-citation refused render-citation --values "${fixture}/values/cite-nowhere.yaml"
id="${ids[0]}"
run find done find ""
run find done find "" --limit 1
run find refused find "" --limit -1
run deps done deps "${id}"
run deps done deps "${id}" -i
run gated-on done gated-on "${id}"
run cited-by done cited-by "${id}"
run referenced-by done referenced-by "${id}"
run strength done strength --max 0.5
run strength done strength --sort dependents --min-dependents 1 --max 0.65
run strength done strength --pending
run subtree done subtree ""
run show done show "${id}"
run show refused show clm-zzzzzz
run stats done stats
run edges done edges
# The demoted relation. One freehand edit of the staged copy's edge records
# adds supports ${ID1} -> ${ID3}, closing a cycle with the analysis's lifts of
# ${ID1}: the state a merge leaves. The load is degraded until refresh cuts the
# cycle, and the cut is the cycle's one supports edge into an analysis — the
# added edge, whichever claim id is minted lower.
cut="${ids[0]} → ${ids[2]}"
printf '%s\n' '- relation: "supports"' "  source: \"${ids[0]}\"" "  target: \"${ids[2]}\"" '  context: ""' \
    '  bearing: 0.5' '  manually-scored: true' >> "${repo}/kb-root/.index/edges.yaml"
run verify refused verify
holds 'verify does not report the cycle as degraded' grep -qx '    - check: "degraded"' "${LAST}"
run refresh done refresh
holds 'refresh does not warn of the cut' \
    grep -qxF "      detail: \"supports ${cut} is demoted, breaking the cycle ${cut} → ${ids[0]}\"" "${LAST}"
run verify done verify
holds 'verify lists no demoted finding' grep -qx '    - check: "demoted"' "${LAST}"
holds 'the demoted finding does not name the cut edge' \
    grep -qxF "      detail: \"${cut}, demoted from supports: cut by cycle breaking\"" "${LAST}"
run edges done edges --relation demoted
holds 'edges lists not one demoted row' grep -qx 'count: 1' "${LAST}"
holds 'the demoted row is not the cut edge' grep -qxF "    - source: \"${ids[0]}\"" "${LAST}"
holds 'the demoted row is not the cut edge' grep -qxF "      target: \"${ids[2]}\"" "${LAST}"
run deps done deps "${ids[0]}"
holds 'deps lists no demoted row out of the source' \
    test "$(grep -A2 -x '    - relation: "demoted"' "${LAST}" | tail -n 2 | tr -d '\n')" = "      direction: \"out\"      target: \"${ids[2]}\""
run stats done stats
holds 'stats counts not one demoted edge' grep -qx '    demoted_edges: 1' "${LAST}"
holds 'stats relations counts not one demoted edge' test "$(sed -n '/^        demoted:$/{n;p;}' "${LAST}")" = '            edges: 1'
run resolve-demoted refused resolve-demoted --values "$(staged resolve-demoted-restore.yaml)"
holds 'restore is not refused as closing the cycle' grep -qx '    - check: "dependency-cycle"' "${LAST}"
run resolve-demoted done resolve-demoted --values "$(staged resolve-demoted-remove.yaml)"
holds 'remove does not report the record removed' grep -qx '      action: "removed"' "${LAST}"
run verify done verify
holds 'verify still lists a finding after the remove' grep -qx 'findings: \[\]' "${LAST}"
run status done status --state-dir "${root}/${out}/state"
run cancel refused cancel --state-dir "${root}/${out}/state"
OUT_DIR="${out}" RECIPE="test-integration-results-fixture" INVOCATION="cp -R ${RESULTS_FIXTURE_DIR}/kb-root repo/ (with a bare .git)\nper invocation NN-<verb>, from repo/: ${kbase} <args> > docs/NN-<verb>.out, parsed by go test -run '^TestResultDocument\$' ./cmd -args -result.doc=docs/NN-<verb>.out -result.verb=<verb> -result.outcome=<outcome> > docs/NN-<verb>.keys.log\nvalues/: the fixture's values files the staging ops and resolve-demoted read, \${ID1} and \${ID2} substituted by the inserted claims' ids in id order and \${ID3} by the analysis's\nthe demoted leg: a supports record from \${ID1} into \${ID3} appended to repo/kb-root/.index/edges.yaml, closing a cycle with the analysis's lifts; verify refused degraded, refresh warning of the cut, verify done with a demoted finding, edges --relation demoted, deps \${ID1} and stats (demoted_edges and its relations) showing it, resolve-demoted restore refused dependency-cycle and remove done, verify done with no finding\nscore-config/: a pool entry whose endpoint is a closed loopback port, and a config.toml naming decision and text but no scoring; score legs over it: refused naming models.scoring without a flag, the flag supplying the unset role and reaching the pool's entry, refused naming --scoring-model for a provider the pool lacks with the pool listed, and one failed item whose roles record states provider-rule flag and the flag's model; then, with config.toml naming different text and scoring models on that entry, a run naming no model by flag and its directory as ../score-config from repo/: bounded on one failed item, the roles record naming the entry, provider-rule config and the scoring model, the ledger's line naming it, the text model nowhere on stdout (resume included) or in the ledger, and resume writing --config-dir absolute\nprobe-config/ with probestub (built from tools/probestub): a pool entry the stub listens on, named as the decision role's model; ${kbase} probe --decision asks it and its document carries the decision selection, no first token and no finish reason, and the stub's chat path refuses any probe that took it\nchecks.tsv: per invocation its arguments, the outcome wanted, pass or fail, and its document" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-results-fixture): a result did not parse as documented — see checks.tsv\n'
    exit 1
fi
printf 'integration(test-integration-results-fixture) ok\n'
