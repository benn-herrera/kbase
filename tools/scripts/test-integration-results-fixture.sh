#!/usr/bin/env bash
# The result contract over the delivered binary: every maintenance subcommand,
# status and cancel run by ./bin/kbase over a copy of a committed fixture KB,
# each result parsed against its subcommand's documented keys and outcome, and
# probe's decision selection run against a Go stub that answers the decision
# surface and refuses the chat path. The copy is committed and converted by
# update-format, the README's write-batch example run dry over it, then staged
# through the write ops with an edge of every
# relation a public op writes; one freehand edit then closes a cycle, which
# every loading command refuses, and another adds the demoted record that
# verify, the edge queries, stats, restore-demoted and remove-edge are checked
# over.
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
rm -rf "${repo}" "${docs}" "${out}/values" "${out}/readme-batch.yaml" "${out}/checks.tsv" "${out}/score-config" \
    "${out}/score-state" "${out}/probe-config" "${out}/probestub" \
    "${out}/probestub.url" "${out}/probestub.log" "${XDG_STATE_HOME}"
mkdir -p "${repo}/.git/objects" "${repo}/.git/refs/heads" "${docs}"
printf 'ref: refs/heads/main\n' > "${repo}/.git/HEAD"
cp -R "${fixture}/kb-root" "${repo}/"
# git reads no configuration of the host's: update-format's clean check and
# the commits below run under none.
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
# commit <message>: the copy's kb-root/ committed as it stands, so
# update-format finds it clean.
commit() {
    git -C "${repo}" add -A kb-root
    git -C "${repo}" -c user.name=kb-testing -c user.email=kb-testing@invalid commit -qm "$1"
}
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
    go test -count=1 -run '^TestResult$' ./cmd -args -result.file="${root}/${stem}.out" -result.verb="${verb}" \
        -result.outcome="${want}" > "${stem}.keys.log" 2>&1
    local rc=$?
    set -e
    printf '%s\t%s\t%s\t%s\n' "$*" "${want}" "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "${stem}.out" >> "${out}/checks.tsv"
    printf '  %s %s: want %s, exit %s\n' "$([[ "${rc}" -eq 0 ]] && echo pass || echo fail)" "$*" "${want}" "$(cat "${stem}.exit")"
    [[ "${rc}" -eq 0 ]] || status=1
    LAST="${stem}.out"
}
# The fixture is a conversion input at an older format: verify reports it
# unmigrated, a write is refused while the conversion is pending, update-format
# refuses the uncommitted copy and, once it is committed, saves the conversion;
# a second run over the committed result changes nothing.
run verify refused verify
grep -qx '    - check: "index-freshness"' "${LAST}" || { printf '  fail: verify does not report the unmigrated index\n'; status=1; }
run add-claim refused add-claim --values "${fixture}/values/add-claim.yaml"
grep -qx '      remedy: "kbase update-format"' "${LAST}" || { printf '  fail: the write over the unmigrated copy does not name update-format\n'; status=1; }
run update-format refused update-format
grep -qx '    - check: "dirty-paths"' "${LAST}" || { printf '  fail: update-format over the uncommitted copy is not refused as dirty\n'; status=1; }
commit "the results fixture as staged"
run update-format done update-format
commit "the results fixture converted"
run update-format unchanged update-format
# update-format renders every sheet on each call: drawn wherever the host has dot.
sheet=not-drawn
command -v dot > /dev/null && sheet=drawn
grep -qx "sheet: \"${sheet}\"" "${LAST}" || { printf '  fail: update-format does not report sheet %s\n' "${sheet}"; status=1; }
run verify done verify
# The lines the claims and the analysis are placed on; an identical re-issued
# add is unchanged, reporting the ids the add minted.
cat "${fixture}/values/statements.md" >> "${repo}/kb-root/a.md"
# The docs' write-batch example, the first fenced yaml block under
# "# Maintaining a KB" (docs/maintaining.md) with its fence's indent removed,
# placed on a.md's line 6 and run dry: the batch the docs show is one kbase takes.
readme_batch="${root}/${out}/readme-batch.yaml"
awk '/^# Maintaining a KB$/ { f = 1; next } f && /^## / { exit } f && !y && /^ *```yaml$/ { y = 1; ind = index($0, "`"); next }
    y && /^ *```$/ { exit } y { print substr($0, ind) }' "${root}/docs/maintaining.md" \
    | sed -E -e 's/^( *)document: .*/\1document: a.md/' -e 's/^( *)line: .*/\1line: 6/' -e 's/^( *)snippet: .*/\1snippet: first result/' \
    > "${readme_batch}"
grep -qx '    document: a.md' "${readme_batch}" || { printf '  fail: no README batch example placed on a.md — see %s\n' "${readme_batch}"; status=1; }
run write done write --dry-run --values "${readme_batch}"
run add-claim done add-claim --values "${fixture}/values/add-claim.yaml"
added="${LAST}"
run add-claim unchanged add-claim --values "${fixture}/values/add-claim.yaml"
[[ "$(tools/scripts/entry-ids.sh "${LAST}" "")" == "$(tools/scripts/entry-ids.sh "${added}" "")" ]] \
    || { printf '  fail: the identical re-issued add does not report the ids the add minted\n'; status=1; }
# The nodes and edges every later leg reads: a work, an analysis, then one
# edge of each relation a public op writes, pending and numeric bearings both,
# from the fixture's values files with the minted ids substituted — ${ID1}
# and ${ID2} the claims in id order, ${WORK1} the work and ${ID3} the
# analysis, each read from its add's result.
ids=()
work=""
while IFS= read -r i; do ids+=("${i}"); done < <(tools/scripts/entry-ids.sh "${added}" clm- | sort)
[[ "${#ids[@]}" -eq 2 ]] || { printf 'integration(results-fixture): the add reported no two claim ids\n'; exit 1; }
mkdir -p "${out}/values"
# staged <file>: the fixture's values file, its ids substituted, under values/;
# prints the copy's path.
staged() {
    sed -e "s/\${ID1}/${ids[0]}/g" -e "s/\${ID2}/${ids[1]}/g" -e "s/\${ID3}/${ids[2]-}/g" -e "s/\${WORK1}/${work}/g" \
        "${fixture}/values/${1}" > "${out}/values/${1}"
    printf '%s' "${root}/${out}/values/${1}"
}
# holds <what> <command...>: a check on the last result; a failure names what.
holds() {
    local what="$1"
    shift
    "$@" || { printf '  fail: %s\n' "${what}"; status=1; }
}
run add-work done add-work --values "${fixture}/values/add-work.yaml"
work="$(tools/scripts/entry-ids.sh "${LAST}" wrk-)"
run add-analysis done add-analysis --values "${fixture}/values/add-analysis.yaml"
ids+=("$(tools/scripts/entry-ids.sh "${LAST}" ana-)")
[[ -n "${work}" && -n "${ids[2]}" ]] || { printf 'integration(results-fixture): the work or analysis add reported no id\n'; exit 1; }
run add-edge done add-edge --values "$(staged add-edge.yaml)"
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
# run reaches the pool's entry, --node bounding it to the one item it asks,
# which the endpoint nothing listens on fails, and the roles record says the
# flag named provider and model.
run score failed score --node "${ids[0]}" --config-dir "${score_config}" --scoring-model unreachable/judge --state-dir "${score_state}"
grep -qx '      provider-rule: "flag"' "${LAST}" || { printf '  fail: the roles record does not say the flag supplied the role\n'; status=1; }
grep -qx '      model: "judge"' "${LAST}" || { printf '  fail: the roles record does not name the flag\x27s model\n'; status=1; }
# The role configured, the flag naming a provider the pool lacks: the
# flag's refusal, before the first item, with the pool's entries listed.
printf '%s\n' '[models]' 'decision = "unreachable/d"' 'text = "unreachable/t"' 'scoring = "unreachable/s"' > "${score_config}/config.toml"
run score refused score --pass rigor --config-dir "${score_config}" --scoring-model elsewhere/judge --state-dir "${score_state}"
grep -qF 'key: "--scoring-model"' "${LAST}" || { printf '  fail: a missing provider is not named as the flag\x27s\n'; status=1; }
grep -qF '"unreachable"' "${LAST}" || { printf '  fail: the pool is not listed\n'; status=1; }
# A flag that parses and names the pool's entry reaches it: --node bounds the
# run to one item, which the endpoint nothing listens on fails, so the run is
# failed; the roles record says the flag named provider and model.
run score failed score --node "${ids[0]}" --config-dir "${score_config}" --scoring-model unreachable/judge --state-dir "${score_state}"
grep -qx '      provider-rule: "flag"' "${LAST}" || { printf '  fail: the roles record does not say the flag named the provider\n'; status=1; }
grep -qx '      model: "judge"' "${LAST}" || { printf '  fail: the roles record does not name the flag\x27s model\n'; status=1; }
# The files alone, no flag naming a model: text and scoring name different
# models on the unreachable entry, --node bounds the run to one item and that
# item fails. The roles record and the ledger's lines name the scoring model; text's
# appears nowhere on stdout, resume included, nor in the ledger. The
# directory is typed relative to the repository, and resume writes it
# absolute.
printf '%s\n' '[models]' 'decision = "unreachable/d"' 'text = "unreachable/file-text"' 'scoring = "unreachable/file-scoring"' \
    > "${score_config}/config.toml"
run score failed score --node "${ids[0]}" --config-dir ../score-config --state-dir "${score_state}"
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
# chat path is answered 400 and its leg fails; the result is parsed
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
run probe done probe --role decision --config-dir "${probe_config}"
grep -qx 'probed: "decision"' "${LAST}" || { printf '  fail: the probe did not ask the decision selection\n'; status=1; }
grep -qx 'first-token-ms: 0' "${LAST}" || { printf '  fail: a decision surface reports no first token\n'; status=1; }
grep -qx 'finish-reason: null' "${LAST}" || { printf '  fail: a decision surface reports no finish reason\n'; status=1; }
kill "${stub_pid}" 2>/dev/null || true
wait "${stub_pid}" 2>/dev/null || true
trap - EXIT

run edit-claim refused edit-claim --values "${fixture}/values/edit-claim-unknown-key.yaml"
id="${ids[0]}"
run find done find
run find refused find --limit 1
holds 'find --limit without --sort is not refused at --limit' grep -qx '      key: "--limit"' "${LAST}"
run find refused find --sort strength --limit 0
holds 'find --limit 0 is not refused at --limit' grep -qx '      key: "--limit"' "${LAST}"
run find done find --sort strength --limit 1
holds 'find --sort strength --limit 1 does not keep one row' grep -qx 'count: 1' "${LAST}"
run find done find --sort strength --limit -1
holds 'find --sort strength --limit -1 does not keep one row' grep -qx 'count: 1' "${LAST}"
run show done show "${id}"
run show refused show clm-zzzzzz
run stats done stats
run edges done edges
# The cycle load error. One freehand edit of the staged copy's edge records
# adds supports ${ID1} -> ${ID3}, closing a cycle with the analysis's lifts of
# ${ID1}: the state a merge leaves. The load fails, every loading command is
# refused naming the edge, and nothing resolves it; the edit is then undone.
cut="${ids[0]} → ${ids[2]}"
command cp "${repo}/kb-root/.index/edges.yaml" "${root}/${out}/edges.yaml.before-cycle"
printf '%s\n' '- relation: "supports"' "  source: \"${ids[0]}\"" "  target: \"${ids[2]}\"" '  context: ""' \
    '  bearing: 0.5' '  manually-scored: true' "  judged-hash: \"$(printf '0%.0s' {1..64})\"" >> "${repo}/kb-root/.index/edges.yaml"
command cp "${repo}/kb-root/.index/edges.yaml" "${root}/${out}/edges.yaml.cycle"
run verify refused verify
holds 'verify does not refuse the cycle as a load error' grep -qx '    - check: "metadata"' "${LAST}"
holds 'the load error does not name the edge closing the cycle' \
    grep -qxF "      detail: \"the edge supports ${cut} closes a cycle among the scoring edges\"" "${LAST}"
commit "the cycle edit"
run update-format refused update-format
holds 'update-format does not refuse the cycle as a load error' grep -qx '    - check: "metadata"' "${LAST}"
holds 'update-format rewrote the edge records over the cycle' \
    cmp -s "${root}/${out}/edges.yaml.cycle" "${repo}/kb-root/.index/edges.yaml"
command cp "${root}/${out}/edges.yaml.before-cycle" "${repo}/kb-root/.index/edges.yaml"
# The demoted relation. A second freehand edit adds the record the build's ring
# row leaves for that cycle: supports ${ID1} -> ${ID3}, demoted.
printf '%s\n' '- relation: "demoted"' "  source: \"${ids[0]}\"" "  target: \"${ids[2]}\"" '  context: ""' \
    '  replaced-relation: "supports"' '  bearing: 0.5' '  manually-scored: true' >> "${repo}/kb-root/.index/edges.yaml"
commit "the demoted edit"
run update-format done update-format
run verify done verify
holds 'verify lists no demoted finding' grep -qx '    - check: "demoted"' "${LAST}"
holds 'the demoted finding does not name the cut edge' \
    grep -qxF "      detail: \"${cut}, demoted from supports: cut by cycle breaking\"" "${LAST}"
run edges done edges --relation demoted
holds 'edges lists not one demoted row' grep -qx 'count: 1' "${LAST}"
holds 'the demoted row is not the cut edge' grep -qxF "    - source: \"${ids[0]}\"" "${LAST}"
holds 'the demoted row is not the cut edge' grep -qxF "      target: \"${ids[2]}\"" "${LAST}"
run stats done stats
holds 'stats counts not one demoted edge' grep -qx '    demoted-edges: 1' "${LAST}"
holds 'stats relations counts not one demoted edge' test "$(sed -n '/^        demoted:$/{n;p;}' "${LAST}")" = '            edges: 1'
run restore-demoted refused restore-demoted --values "$(staged demoted-edge.yaml)"
holds 'restore is not refused as closing the cycle' grep -qx '    - check: "dependency-cycle"' "${LAST}"
run remove-edge done remove-edge --values "$(staged demoted-edge.yaml)"
run verify done verify
holds 'verify still lists a finding after the remove' grep -qx 'findings: \[\]' "${LAST}"
run status done status --state-dir "${root}/${out}/state"
run cancel refused cancel --state-dir "${root}/${out}/state"
OUT_DIR="${out}" RECIPE="test-integration-results-fixture" INVOCATION="cp -R ${RESULTS_FIXTURE_DIR}/kb-root repo/ (with a bare .git); kb-root committed (git add -A kb-root; git commit) before each update-format that must find it clean\n${RESULTS_FIXTURE_DIR}/values/statements.md appended to repo/kb-root/a.md after the conversion, before add-claim places the claims on its lines\nreadme-batch.yaml: README.md's first fenced yaml block under \"## Maintaining a KB\", its fence's indent removed, document, line and snippet set to a.md, 6 and first result; ${kbase} write --dry-run --values readme-batch.yaml, wanting done\nper invocation NN-<verb>, from repo/: ${kbase} <args> > docs/NN-<verb>.out, parsed by go test -run '^TestResult\$' ./cmd -args -result.file=docs/NN-<verb>.out -result.verb=<verb> -result.outcome=<outcome> > docs/NN-<verb>.keys.log\nvalues/: the fixture's values files add-edge, restore-demoted and remove-edge read, \${ID1} and \${ID2} substituted by the added claims' ids in id order, \${WORK1} by the work's and \${ID3} by the analysis's, each read from its add's result\nthe cycle leg: a supports record from \${ID1} into \${ID3} appended to repo/kb-root/.index/edges.yaml, closing a cycle with the analysis's lifts; verify and update-format each refused, check metadata naming the edge, the edge records unchanged (edges.yaml.cycle), then the edit undone from edges.yaml.before-cycle\nthe demoted leg: a demoted record of that supports edge appended instead; update-format done, verify done with a demoted finding, edges --relation demoted and stats (demoted-edges and its relations) showing it, restore-demoted refused dependency-cycle and remove-edge, relation demoted, done, verify done with no finding\nscore-config/: a pool entry whose endpoint is a closed loopback port, and a config.toml naming decision and text but no scoring; score legs over it: refused naming models.scoring without a flag, the flag supplying the unset role and reaching the pool's entry, refused naming --scoring-model for a provider the pool lacks with the pool listed, and, --node \${ID1} bounding each run to that one item, a failed run whose roles record states provider-rule flag and the flag's model; then, with config.toml naming different text and scoring models on that entry, a run naming no model by flag and its directory as ../score-config from repo/: failed on the one item --node names, the roles record naming the entry, provider-rule config and the scoring model, the ledger's line naming it, the text model nowhere on stdout (resume included) or in the ledger, and resume writing --config-dir absolute\nprobe-config/ with probestub (built from tools/probestub): a pool entry the stub listens on, named as the decision role's model; ${kbase} probe --role decision asks it and its result's probed names the decision role, no first token and no finish reason, and the stub's chat path refuses any probe that took it\nchecks.tsv: per invocation its arguments, the outcome wanted, pass or fail, and its result" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-results-fixture): a result did not parse as documented — see checks.tsv\n'
    exit 1
fi
printf 'integration(test-integration-results-fixture) ok\n'
