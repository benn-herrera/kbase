#!/usr/bin/env bash
# LIVE: ./bin/kbase build without --no-inference over the Slice-1 paper
# BUILD_ARXIV_SPECIAL_ID names, in a fresh fixture repository, asking the
# provider CONFIG names; then a second build through claims-discovered
# configured by the environment alone, a third naming no configuration
# directory, and a score run configured by the environment alone. Counts land
# in comparison.yaml.
#
# Inputs (environment): BIN_DIR; ARXIV_DIR, the unpacked corpus;
# BUILD_ARXIV_SPECIAL_ID, the paper built; BUILD_LIVE_ARXIV_OUT_DIR, the
# evidence directory; CONFIG, the live configuration directory (absolute or
# relative to the repository root).
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${ARXIV_DIR:?ARXIV_DIR is not set: the unpacked corpus}"
: "${BUILD_ARXIV_SPECIAL_ID:?BUILD_ARXIV_SPECIAL_ID is not set: the paper to build}"
: "${BUILD_LIVE_ARXIV_OUT_DIR:?BUILD_LIVE_ARXIV_OUT_DIR is not set: the evidence directory}"
: "${CONFIG:?CONFIG is not set: the live configuration directory}"

root="$(pwd)"
out="${BUILD_LIVE_ARXIV_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
config="${CONFIG}"
[[ "${config}" == /* ]] || config="${root}/${config}"
id="${BUILD_ARXIV_SPECIAL_ID}"
pandoc_version="$(pandoc --version | head -n 1)"
printf 'pandoc: %s\n' "${pandoc_version}"
# capture <stem> <dir> <command...>: run the command in dir, stdout to
# <stem>.out, stderr to <stem>.log, exit code to <stem>.exit.
capture() {
    local stem="$1" dir="$2"
    shift 2
    set +e
    ( cd "${dir}" && "$@" ) > "${stem}.out" 2> "${stem}.log"
    printf '%s\n' "$?" > "${stem}.exit"
    set -e
}
rc() { cat "$1.exit"; }
checks=()
status=0
check() {
    local verdict=pass
    [[ "$2" -eq 0 ]] || { verdict=fail; status=1; }
    checks+=("$1"$'\t'"${verdict}"$'\t'"$3"$'\t'"$4")
    printf '  %s %s: %s\n' "${verdict}" "$1" "$3"
}
yaml_str() { local s="${1//\\/\\\\}"; printf '"%s"' "${s//\"/\\\"}"; }
ok() { "$@" > /dev/null 2>&1 && echo 0 || echo 1; }
count() { find "$1" -type f -name "$2" 2>/dev/null | wc -l | tr -d ' '; }

# The endpoint first: an unreachable provider fails here, by name, and no
# build is started against it. The fixture's pool holds two entries and its
# config.toml's provider key is superseded, so the entry is named.
provider="$(sed -n 's/^provider *= *"\(.*\)".*/\1/p' "${config}/config.toml" | head -n 1)"
capture "${out}/models" "${root}" "${kbase}" models --config-dir "${config}" --provider "${provider}"
if [[ "$(rc "${out}/models")" -ne 0 ]]; then
    printf 'integration(test-integration-build-live-arxiv): provider %s, named in %s/config.toml, is unreachable — see %s\n' \
        "${provider:-(none named)}" "${CONFIG}" "${out}/models.out"
    exit 1
fi

name="$(printf '%s' "${id}" | tr '/' '_')"
dir="${out}/${name}"
repo="${dir}/repo"
ck="${dir}/checks"
state="${root}/${dir}/state"
rm -rf "${dir}"
mkdir -p "${ck}"
mkdir -p "${repo}"
cp -R "${ARXIV_DIR}/${name}/." "${repo}/"
printf 'default:\n\t@echo %s\n\ntest:\n\t@echo tests passed.\n' "${name}" > "${repo}/Makefile"
printf '%s\n' '.claude-temp/' '__pycache__/' > "${repo}/.gitignore"
git -C "${repo}" init -q
git -C "${repo}" add -A
git -C "${repo}" -c user.name=arxiv -c user.email=arxiv@invalid commit -qm "sources as fetched"
volume_root="$(ARXIV_VOLUME_DIR="${repo}" tools/scripts/arxiv-volume-root.sh)"
printf '%s\n' "${id}"

# The fixture's config.toml is in the old grammar — bare model ids beside a
# superseded provider key — and file-mode resolution refuses a bare id. The
# file-mode legs read a reference-shaped copy in this run's directory: the
# same pool and keys, and three references naming that pool's entry. The
# environment leg below still reads the fixture's bare ids, which is what
# the environment names.
toml_get() { awk -v t="[$2]" -v k="$3" '/^\[/ { in_t = ($0 == t) } in_t && $1 == k { sub(/^[^=]*= *"/, ""); sub(/".*/, ""); print; exit }' "$1"; }
base_url="$(toml_get "${config}/providers.toml" "${provider}" baseUrl)"
key_file="$(toml_get "${config}/providers.toml" "${provider}" apiKeyFile)"
[[ -z "${key_file}" || "${key_file}" == /* ]] || key_file="${config}/${key_file}"
decision="$(toml_get "${config}/config.toml" models decision)"
text="$(toml_get "${config}/config.toml" models text)"
# The fixture declares no scoring model; the decision model judges the KB.
scoring="${decision}"
file_config="${dir}/file-config"
mkdir -p "${file_config}"
cp "${config}/providers.toml" "${file_config}/providers.toml"
cp -R "${config}/api_keys" "${file_config}/api_keys"
printf '%s\n' '[models]' "decision = \"${provider}/${decision}\"" "text = \"${provider}/${text}\"" "scoring = \"${provider}/${scoring}\"" \
    > "${file_config}/config.toml"

# the build, asking, to completion
capture "${ck}/kbase-build" "${repo}" "${kbase}" build "${volume_root}" --config-dir "${file_config}" --state-dir "${state}" --log-level info
check build "$([[ "$(rc "${ck}/kbase-build")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-build.out" && echo 0 || echo 1)" \
    "exit $(rc "${ck}/kbase-build"), $(grep -m1 '^outcome:' "${ck}/kbase-build.out" || echo 'no outcome')" "${ck}/kbase-build.out"
check no-row-dropped "$(ok test -z "$(git -C "${repo}" log --format=%b --grep='^kb-build: ' | grep -- '--no-inference')")" \
    "no boundary names a dropped row" "${repo}/.git"
check overview "$(ok grep -q '^# .* Knowledge Base$' "${repo}/kb-root/README.md")" "README.md written around its passage" "${repo}/kb-root/README.md"
captures="$(count "${state}/scratch/trails" '*.capture.jsonl')"
letter_answers="$(count "${state}/scratch/trails" '*.json')"
overview_answers="$(count "${state}/scratch/trails" '*.txt')"
ask_records="$(count "${state}/scratch/trails" '*.yaml')"
check captures "$(ok test "${captures}" -gt 0 -a "${letter_answers}" -gt 0 -a "${overview_answers}" -gt 0 -a "${ask_records}" -gt 0)" \
    "${captures} captures, ${letter_answers} cached letter-ask answers, ${overview_answers} cached overview answers, ${ask_records} group-record files under the state store's scratch/" "${state}/scratch"

# the counts, off the build records and the result
set +e
go test -count=1 -run '^TestLiveBuildCounts$' ./internal/claimgraph -args -claimgraph.repo="${root}/${repo}" \
    -claimgraph.result="${root}/${ck}/kbase-build.out" -claimgraph.out="${root}/${dir}" > "${ck}/counts.log" 2>&1
check records "$?" "every leaf landed, every planned pair answered, no candidate drafted; counts.yaml and nodepass.json written" "${ck}/counts.log"
set -e

# The same server and model configured by the environment alone: no
# --config-dir, the configuration directory absent, through the node pass
# so the build asks. The environment names bare ids, as it does by
# definition; the fixture's own values are what it carries.
env_repo="${dir}/env-repo"
no_config="${root}/${dir}/no-config"
mkdir -p "${env_repo}"
cp -R "${ARXIV_DIR}/${name}/." "${env_repo}/"
git -C "${env_repo}" init -q
capture "${ck}/kbase-build-env" "${env_repo}" env KBASE_CONFIG_DIR="${no_config}" KBASE_API_BASE_URL="${base_url}" \
    KBASE_DECISION="${decision}" KBASE_TEXT="${text}" KBASE_API_KEY_FILE="${key_file}" "${kbase}" build "${volume_root}" \
    --through claims-discovered --state-dir "${root}/${dir}/env-state" --log-level info
warned="$(grep -c 'travels in cleartext' "${ck}/kbase-build-env.log" || true)"
expect_warning=0
[[ -n "${key_file}" && "${base_url}" == http://* && ! "${base_url}" =~ ^http://(localhost|127\.|\[::1\]) ]] && expect_warning=1
check build-env "$([[ "$(rc "${ck}/kbase-build-env")" -eq 0 ]] && grep -qx 'outcome: "bounded"' "${ck}/kbase-build-env.out" \
    && [[ "$(count "${root}/${dir}/env-state/scratch/trails" '*.capture.jsonl')" -gt 0 && ! -e "${no_config}" && "${warned}" -eq "${expect_warning}" ]] && echo 0 || echo 1)" \
    "configured by KBASE_API_BASE_URL, KBASE_DECISION=${decision}, KBASE_TEXT=${text} and KBASE_API_KEY_FILE alone: exit $(rc "${ck}/kbase-build-env"), $(grep -m1 '^outcome:' "${ck}/kbase-build-env.out" || echo 'no outcome'), $(count "${root}/${dir}/env-state/scratch/trails" '*.capture.jsonl') captures, ${warned} cleartext warning(s) (${expect_warning} expected), no configuration directory" \
    "${ck}/kbase-build-env.out"

# The layering proof, which names no configuration directory at all: every
# leg above passes --config-dir, which is single-layer mode. The global
# layer is the fixture's pool copied into this leg's own home — its
# apiKeyFile paths resolve beside the file, so its keys travel with it —
# and the repository layer is what configure --create --project writes,
# dialing reaper out of that pool and writing a reference for each category
# — decision and text from the fixture, scoring from its decision model.
# The build then takes its endpoint from the global file and its three
# references from the repository's, with the operator's own
# ~/.config/kbase read by neither: HOME is this leg's directory, and the
# variable that would name a directory, and the five that would supply a
# field over the files, are unset.
layered="${root}/${dir}/layered"
layered_home="${layered}/home"
layered_repo="${layered}/repo"
mkdir -p "${layered_home}/.config/kbase" "${layered_repo}"
cp "${config}/providers.toml" "${layered_home}/.config/kbase/providers.toml"
# A relative apiKeyFile resolves beside its providers.toml, so the keys it
# names travel with it or both entries drop; owner-only, because a key file
# any other account can read is reported as a warning fault.
cp -R "${config}/api_keys" "${layered_home}/.config/kbase/api_keys"
chmod 0600 "${layered_home}/.config/kbase/api_keys/reaper.txt" "${layered_home}/.config/kbase/api_keys/imp.txt"
# The build commits its boundaries with the ambient git identity, which this
# home supplies in place of the operator's.
printf '[user]\n\tname = kbase integration\n\temail = kbase@invalid\n' > "${layered_home}/.gitconfig"
cp -R "${ARXIV_DIR}/${name}/." "${layered_repo}/"
git -C "${layered_repo}" init -q
layered_volume_root="$(ARXIV_VOLUME_DIR="${layered_repo}" tools/scripts/arxiv-volume-root.sh)"
layered_env=(-u KBASE_CONFIG_DIR -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE "HOME=${layered_home}")
capture "${ck}/kbase-configure-layered" "${layered_repo}" env "${layered_env[@]}" "${kbase}" configure \
    --create --project --provider reaper \
    --model-map "decision=reaper/${decision},text=reaper/${text},scoring=reaper/${scoring}"
layered_refs=0
[[ "$(rc "${ck}/kbase-configure-layered")" -eq 0 ]] || layered_refs=1
grep -qx 'outcome: "done"' "${ck}/kbase-configure-layered.out" || layered_refs=1
grep -q '^    - ".*/\.kbase/config\.toml"$' "${ck}/kbase-configure-layered.out" || layered_refs=1
grep -qxF "    decision: \"reaper/${decision}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
grep -qxF "    text: \"reaper/${text}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
grep -qxF "    scoring: \"reaper/${scoring}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
if grep -q '^provider *=' "${layered_repo}/.kbase/config.toml"; then layered_refs=1; fi
check configure-layered "${layered_refs}" \
    "reaper dialed from the global pool; three references written into the repository layer, and no provider key: exit $(rc "${ck}/kbase-configure-layered")" \
    "${ck}/kbase-configure-layered.out"
capture "${ck}/kbase-build-layered" "${layered_repo}" env "${layered_env[@]}" "${kbase}" build "${layered_volume_root}" \
    --state-dir "${layered}/state" --log-level info
layered_captures="$(count "${layered}/state/scratch/trails" '*.capture.jsonl')"
check layered-build "$([[ "$(rc "${ck}/kbase-build-layered")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-build-layered.out" \
    && [[ "${layered_captures}" -gt 0 ]] && echo 0 || echo 1)" \
    "no --config-dir: exit $(rc "${ck}/kbase-build-layered"), $(grep -m1 '^outcome:' "${ck}/kbase-build-layered.out" || echo 'no outcome'), ${layered_captures} captures against the endpoint the global layer names" \
    "${ck}/kbase-build-layered.out"
capture "${ck}/kbase-show-config-layered" "${layered_repo}" env "${layered_env[@]}" "${kbase}" show-config
layered_show="${ck}/kbase-show-config-layered"
layered_pool=0
[[ "$(rc "${layered_show}")" -eq 0 ]] || layered_pool=1
grep -qx 'outcome: "done"' "${layered_show}.out" || layered_pool=1
# the two layers, the global one this leg's home — so no other directory
# was read
[[ "$(grep -cxF '      source: "global"' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
[[ "$(grep -cxF '      source: "repository"' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
grep -qxF "    - path: \"${layered_home}/.config/kbase\"" "${layered_show}.out" || layered_pool=1
# the merged pool: the fixture's two entries out of the global file, the
# template's [local] out of the repository one, every entry usable
[[ "$(grep -c '^    - name: ' "${layered_show}.out")" -eq 3 ]] || layered_pool=1
[[ "$(grep -cxF '      usable: true' "${layered_show}.out")" -eq 3 ]] || layered_pool=1
[[ "$(grep -cxF '      layer: "global"' "${layered_show}.out")" -eq 2 ]] || layered_pool=1
[[ "$(grep -cxF '      layer: "repository"' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
for entry in imp local reaper; do
    grep -qxF "    - name: \"${entry}\"" "${layered_show}.out" || layered_pool=1
done
[[ "$(grep -cxF 'diagnostics: []' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
check layered-pool "${layered_pool}" \
    "3 pool entries, imp and reaper global and local repository, all usable, no diagnostics; the global layer is ${layered_home}/.config/kbase" \
    "${layered_show}.out"
# The repository's config.toml names three references, and the document
# reports them verbatim; each role's record names the entry it asked, the
# rule, the layer, and the layer that supplied each field: the endpoint and
# the key from the global pool entry, the model from the repository's file.
layered_sources=0
grep -qxF "    decision: \"reaper/${decision}\"" "${layered_show}.out" || layered_sources=1
grep -qxF "    text: \"reaper/${text}\"" "${layered_show}.out" || layered_sources=1
grep -qxF "    scoring: \"reaper/${scoring}\"" "${layered_show}.out" || layered_sources=1
for role in decision text scoring; do
    grep -qxF "    - role: \"${role}\"" "${layered_show}.out" || layered_sources=1
done
[[ "$(grep -cxF '      provider: "reaper"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF '      provider-rule: "config"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF '      provider-layer: "global"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF "      base-url: \"${base_url}\"" "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF '        base-url: "global"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF '        model: "repository"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF '        key: "global"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
grep -qxF "      model: \"Qwen3.5-9B\"" "${layered_show}.out" || layered_sources=1
[[ "$(grep -cxF "      model: \"${decision}\"" "${layered_show}.out")" -eq 2 ]] || layered_sources=1
grep -qxF 'environment: []' "${layered_show}.out" || layered_sources=1
check layered-sources "${layered_sources}" \
    "three role records, each reaper out of the global pool with its model from the repository's file: base-url and key global, model repository; decision and scoring ${decision}, text ${text}" \
    "${layered_show}.out"

# The environment mode configures a score run with no configuration
# directory: the base URL and the role variables alone, against the KB the
# first build finished. One item is asked; the document and the ledger name
# the scoring model, which differs from the text model, and the text model
# appears nowhere in the run.
score_home="${dir}/score-home"
mkdir -p "${score_home}"
score_env=(-u KBASE_CONFIG_DIR "HOME=${score_home}" "KBASE_API_BASE_URL=${base_url}"
    "KBASE_DECISION=${decision}" "KBASE_TEXT=${text}" "KBASE_SCORING=${scoring}")
[[ -z "${key_file}" ]] || score_env+=("KBASE_API_KEY_FILE=${key_file}")
capture "${ck}/kbase-score-env" "${repo}" env "${score_env[@]}" "${kbase}" score \
    --pass rigor --limit 1 --state-dir "${root}/${dir}/score-state"
score_ok=0
[[ "$(rc "${ck}/kbase-score-env")" -eq 0 ]] || score_ok=1
grep -qx 'outcome: "done"' "${ck}/kbase-score-env.out" || score_ok=1
grep -qx '    - role: "scoring"' "${ck}/kbase-score-env.out" || score_ok=1
grep -qx '      back-end: "environment"' "${ck}/kbase-score-env.out" || score_ok=1
grep -qxF "      model: \"${scoring}\"" "${ck}/kbase-score-env.out" || score_ok=1
[[ "$(sed -n 's/^    done: //p' "${ck}/kbase-score-env.out")" -eq 1 ]] || score_ok=1
score_ledger="$(sed -n 's/^ledger: "\(.*\)"$/\1/p' "${ck}/kbase-score-env.out" | head -n 1)"
[[ -n "${score_ledger}" && -f "${score_ledger}/ledger.jsonl" ]] || score_ok=1
grep -qF "\"model\":\"${scoring}\"" "${score_ledger}/ledger.jsonl" || score_ok=1
# The text model appears nowhere the run asked. The resume record is the one
# place it stands: an environment-mode command re-issues the whole
# configuration it read, text's variable included.
if grep -v '^resume:' "${ck}/kbase-score-env.out" | grep -qF "${text}"; then score_ok=1; fi
if grep -qF "${text}" "${score_ledger}/ledger.jsonl"; then score_ok=1; fi
check score-env "${score_ok}" \
    "environment mode configured a score run with no configuration directory: one item asked on ${scoring} (the text model ${text} appears nowhere the run asked), against ${base_url}" \
    "${ck}/kbase-score-env.out"

{
    printf -- '- id: %s\n  state-dir: %s\n  scratch:\n    captures: %s\n    letter-answers: %s\n    overview-answers: %s\n    ask-records: %s\n  checks:\n' \
        "$(yaml_str "${id}")" "$(yaml_str "${state}")" "${captures}" "${letter_answers}" "${overview_answers}" "${ask_records}"
    for c in "${checks[@]}"; do
        IFS=$'\t' read -r c_name c_verdict c_detail c_evidence <<< "${c}"
        printf '    - check: %s\n      result: %s\n      detail: %s\n      evidence: %s\n' \
            "$(yaml_str "${c_name}")" "$(yaml_str "${c_verdict}")" "$(yaml_str "${c_detail}")" "$(yaml_str "${c_evidence}")"
    done
    [[ -f "${dir}/counts.yaml" ]] && sed 's/^/  /' "${dir}/counts.yaml"
} > "${out}/comparison.yaml"

OUT_DIR="${out}" RECIPE="test-integration-build-live-arxiv" INVOCATION="${kbase} models --config-dir <config> --provider <the fixture's provider> into models.*\n<id>/file-config: the fixture's providers.toml and api_keys with a config.toml naming three references (decision and text from the fixture, scoring from its decision model), because the fixture's own [models] values are bare ids the file mode refuses\non ${BUILD_ARXIV_SPECIAL_ID}, staged into <id>/repo (sources, stub Makefile, .gitignore):\n(cd <id>/repo && ${kbase} build <volume-root> --config-dir <id>/file-config --state-dir <id>/state --log-level info) into <id>/checks/kbase-build.*\ngo test -run '^TestLiveBuildCounts\$' ./internal/claimgraph -args -claimgraph.repo=<id>/repo -claimgraph.result=<id>/checks/kbase-build.out -claimgraph.out=<id>\n(cd <id>/env-repo && env KBASE_CONFIG_DIR=<id>/no-config KBASE_API_BASE_URL=<the provider's baseUrl> KBASE_DECISION=<config's models.decision> KBASE_TEXT=<config's models.text> KBASE_API_KEY_FILE=<its apiKeyFile> ${kbase} build <volume-root> --through claims-discovered --state-dir <id>/env-state --log-level info) into <id>/checks/kbase-build-env.*\n(cd <id>/layered/repo && env -u KBASE_CONFIG_DIR -u KBASE_API_BASE_URL -u KBASE_DECISION -u KBASE_TEXT -u KBASE_SCORING -u KBASE_API_KEY_FILE HOME=<id>/layered/home ${kbase} configure --create --project --provider reaper --model-map decision=reaper/<config's models.decision>,text=reaper/<config's models.text>,scoring=reaper/<its decision model>; ${kbase} build <volume-root> --state-dir <id>/layered/state --log-level info; ${kbase} show-config) into <id>/checks/kbase-configure-layered.*, kbase-build-layered.* and kbase-show-config-layered.*, in <id>/layered/repo below <id>/layered/home/.config/kbase (the fixture's providers.toml and its api_keys, the global layer, over .kbase/ that configure wrote, the repository layer)\n(cd <id>/repo && env -u KBASE_CONFIG_DIR -u KBASE_API_KEY_FILE HOME=<id>/score-home KBASE_API_BASE_URL=<the provider's baseUrl> KBASE_DECISION=<config's models.decision> KBASE_TEXT=<config's models.text> KBASE_SCORING=<its decision model> [KBASE_API_KEY_FILE=<its apiKeyFile>] ${kbase} score --pass rigor --limit 1 --state-dir <id>/score-state) into <id>/checks/kbase-score-env.*: environment mode with no configuration directory, one item asked on the scoring model, the ledger naming it and no line naming the text model" \
    tools/scripts/write-evidence.sh
printf '\nReference: %s\n' "${pandoc_version}" >> "${out}/EVIDENCE.md"
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-build-live-arxiv): a check failed — see comparison.yaml\n'
    exit 1
fi
printf 'integration(test-integration-build-live-arxiv) ok\n'
