#!/usr/bin/env bash
# The result contract over the delivered binary: every maintenance subcommand,
# status and cancel run by ./bin/kbase over a copy of a committed fixture KB,
# each result parsed against its subcommand's documented keys and outcome, and
# probe's decision selection run against a Go stub that answers the decision
# surface and refuses the chat path.
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
# The inference variables are scrubbed for the whole script: every leg below
# reads the staged tree and nothing the host exports, except the one that
# sets them to prove the environment supplies a run.
unset KBASE_API_BASE_URL KBASE_DECISION KBASE_TEXT KBASE_SCORING KBASE_API_KEY_FILE KBASE_CONFIG_DIR
repo="${out}/repo"
docs="${out}/docs"
rm -rf "${repo}" "${docs}" "${out}/checks.tsv" "${out}/score-config" "${out}/score-home" \
    "${out}/score-state" "${out}/score-env-state" "${out}/probe-config" "${out}/probestub" \
    "${out}/probestub.url" "${out}/probestub.log"
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
run verify done verify
run render-claim-graph unchanged render-claim-graph
run insert-claim-entry done insert-claim-entry --values "${fixture}/values/insert.yaml"
run insert-claim-entry unchanged insert-claim-entry --no-refresh --values "${fixture}/values/insert.yaml"
run score done score --dry-run --pass rigor --state-dir "${root}/${out}/state"
# Score's role is the KB's own declaration. A file-mode configuration that
# names decision and text but no scoring refuses before the first item,
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
# Environment mode with no configuration directory at all: the base URL and
# the role variables configure the run, its one item fails on the same
# unreachable endpoint, and the document and the ledger name the scoring
# model — a different model from text's, which appears nowhere.
mkdir -p "${out}/score-home"
env_score="${docs}/$(printf '%02d' "$((n + 1))")-score-env-mode"
set +e
( cd "${repo}" && env -u KBASE_CONFIG_DIR -u KBASE_API_KEY_FILE "HOME=${out}/score-home" \
    KBASE_API_BASE_URL=http://127.0.0.1:1/v1 KBASE_DECISION=env-decision KBASE_TEXT=env-text KBASE_SCORING=env-scoring \
    "${kbase}" score --pass rigor --limit 1 --state-dir "${root}/${out}/score-env-state" ) > "${env_score}.out" 2> "${env_score}.log"
printf '%s\n' "$?" > "${env_score}.exit"
go test -count=1 -run '^TestResultDocument$' ./cmd -args -result.doc="${root}/${env_score}.out" -result.verb=score \
    -result.outcome=bounded > "${env_score}.keys.log" 2>&1
env_score_keys=$?
set -e
env_score_ok=0
[[ "$(cat "${env_score}.exit")" -eq 0 ]] || env_score_ok=1
[[ "${env_score_keys}" -eq 0 ]] || env_score_ok=1
n=$((n + 1))
grep -qx 'outcome: "bounded"' "${env_score}.out" || env_score_ok=1
grep -qx '    - role: "scoring"' "${env_score}.out" || env_score_ok=1
grep -qx '      back-end: "environment"' "${env_score}.out" || env_score_ok=1
grep -qx '      model: "env-scoring"' "${env_score}.out" || env_score_ok=1
score_ledger="$(sed -n 's/^ledger: "\(.*\)"$/\1/p' "${env_score}.out" | head -n 1)"
[[ -n "${score_ledger}" && -f "${score_ledger}/ledger.jsonl" ]] || env_score_ok=1
grep -qF '"model":"env-scoring"' "${score_ledger}/ledger.jsonl" || env_score_ok=1
# The text model appears nowhere the run asked: the roles record and the
# ledger's lines name the scoring model alone. The resume record is the one
# place it stands, because an environment-mode command re-issues the whole
# configuration it read, text's variable included.
if grep -v '^resume:' "${env_score}.out" | grep -qF 'env-text'; then env_score_ok=1; fi
if grep -qF 'env-text' "${score_ledger}/ledger.jsonl"; then env_score_ok=1; fi
# 28: an environment-mode run's command re-issues the mode as assignments
# before the program, so a shell with none of them set asks the same
# endpoint with the same model.
grep -q '^resume: "KBASE_API_BASE_URL=http://127.0.0.1:1/v1 KBASE_DECISION=env-decision KBASE_TEXT=env-text KBASE_SCORING=env-scoring kbase score ' \
    "${env_score}.out" || env_score_ok=1
printf 'score --pass rigor --limit 1 (environment mode)\tbounded\t%s\t%s\n' \
    "$([[ "${env_score_ok}" -eq 0 ]] && echo pass || echo fail)" "${env_score}.out" >> "${out}/checks.tsv"
printf '  %s score in environment mode: want bounded, exit %s\n' \
    "$([[ "${env_score_ok}" -eq 0 ]] && echo pass || echo fail)" "$(cat "${env_score}.exit")"
[[ "${env_score_ok}" -eq 0 ]] || status=1

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
id="$(sed -n 's/^    - "\(clm-[a-z0-9]*\)"$/\1/p' "${docs}/06-insert-claim-entry.out" | head -n 1)"
[[ -n "${id}" ]] || { printf 'integration(results-fixture): the insert reported no id\n'; status=1; }
run find done find ""
run find done find "" --limit 1
run find refused find "" --limit -1
run deps done deps "${id}"
run deps done deps "${id}" -i
run gated-on done gated-on "${id}"
run cited-by done cited-by "${id}"
run referenced-by done referenced-by "${id}"
run strength-below done strength-below --max-strength 0.5
run subtree done subtree ""
run weak-points done weak-points
run show done show "${id}"
run show refused show clm-zzzzzz
run stats done stats
run edge-census done edge-census
run edges done edges
run status done status --state-dir "${root}/${out}/state"
run cancel refused cancel --state-dir "${root}/${out}/state"
OUT_DIR="${out}" RECIPE="test-integration-results-fixture" INVOCATION="cp -R ${RESULTS_FIXTURE_DIR}/kb-root repo/ (with a bare .git)\nper invocation NN-<verb>, from repo/: ${kbase} <args> > docs/NN-<verb>.out, parsed by go test -run '^TestResultDocument\$' ./cmd -args -result.doc=docs/NN-<verb>.out -result.verb=<verb> -result.outcome=<outcome> > docs/NN-<verb>.keys.log\nscore-config/: a pool entry whose endpoint is a closed loopback port, and a config.toml naming decision and text but no scoring; score legs over it: refused naming models.scoring without a flag, the flag supplying the unset role and reaching the pool's entry, refused naming --scoring-model for a provider the pool lacks with the pool listed, and one failed item whose roles record states provider-rule flag and the flag's model\nscore-env-mode: the same KB with HOME on score-home, KBASE_API_BASE_URL and KBASE_DECISION, KBASE_TEXT, KBASE_SCORING set and no configuration directory; its one item fails on the closed endpoint, the roles record names the environment back end and KBASE_SCORING's id, the ledger's line names it, and the text model appears nowhere the run asked (the resume record re-issues the whole configuration, so it names the mode's variables)\nprobe-config/ with probestub (built from tools/probestub): a pool entry the stub listens on, named as the decision role's model; ${kbase} probe --decision asks it and its document carries the decision selection, no first token and no finish reason, and the stub's chat path refuses any probe that took it\nchecks.tsv: per invocation its arguments, the outcome wanted, pass or fail, and its document" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-results-fixture): a result did not parse as documented — see checks.tsv\n'
    exit 1
fi
printf 'integration(test-integration-results-fixture) ok\n'
