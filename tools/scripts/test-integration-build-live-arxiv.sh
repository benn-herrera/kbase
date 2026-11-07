#!/usr/bin/env bash
# LIVE: ./bin/kbase build without --no-inference over the Slice-1 paper
# BUILD_ARXIV_SPECIAL_ID names, in a fresh fixture repository, asking the
# provider the resolved configuration names; then a layered-configuration
# proof naming no configuration directory (configure-models --layer repository, then
# show-config), and a full score of the KB the build produced, configured by
# --config-dir.
# Counts land in comparison.yaml.
#
# Inputs (environment): BIN_DIR; ARXIV_DIR, the unpacked corpus;
# BUILD_ARXIV_SPECIAL_ID, the paper built; BUILD_LIVE_ARXIV_OUT_DIR, the
# evidence directory; CONFIG, a configuration directory (absolute or relative
# to the repository root) every kbase call is then given as --config-dir, or
# empty: no call passes --config-dir and kbase resolves configuration as
# always. The legs' values come from the resolved configuration's show-config.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${ARXIV_DIR:?ARXIV_DIR is not set: the unpacked corpus}"
: "${BUILD_ARXIV_SPECIAL_ID:?BUILD_ARXIV_SPECIAL_ID is not set: the paper to build}"
: "${BUILD_LIVE_ARXIV_OUT_DIR:?BUILD_LIVE_ARXIV_OUT_DIR is not set: the evidence directory}"
: "${CONFIG?CONFIG is not set: a configuration directory, or empty for the resolved configuration}"

root="$(pwd)"
out="${BUILD_LIVE_ARXIV_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
# cfg is the flag every kbase call carries; empty, no call names a directory.
config="${CONFIG}"
cfg=()
if [[ -n "${config}" ]]; then
    [[ "${config}" == /* ]] || config="${root}/${config}"
    cfg=(--config-dir "${config}")
fi
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
# shellcheck source=tools/scripts/lib-result-keys.sh
source "${script_dir}/lib-result-keys.sh"
yaml_str() { local s="${1//\\/\\\\}"; printf '"%s"' "${s//\"/\\\"}"; }
ok() { "$@" > /dev/null 2>&1 && echo 0 || echo 1; }
count() { find "$1" -type f -name "$2" 2>/dev/null | wc -l | tr -d ' '; }

# doc_get <result> <section> <key> <value> <field>: <field> of each record in
# the top-level <section> list whose <key> is <value>, one per line, unquoted
# (a null prints empty). A record opens on its "    - " line; its fields stand
# at six spaces.
doc_get() {
    awk -v sec="$2" -v mk="$3" -v mv="$4" -v f="$5" '
        function flush() { if (hit && (f in rec)) print rec[f]; delete rec; hit = 0 }
        /^[^ ]/ { flush(); in_sec = ($0 == sec ":"); next }
        !in_sec { next }
        {
            line = $0
            if (line ~ /^    - /) { flush(); line = "      " substr(line, 7) }
            if (match(line, /^      [A-Za-z_-]+: /)) {
                k = substr(line, 7, RLENGTH - 8)
                v = substr(line, RLENGTH + 1)
                if (v == "null") v = ""
                gsub(/^"|"$/, "", v)
                rec[k] = v
                if (k == mk && v == mv) hit = 1
            }
        }
        END { flush() }' "$1"
}

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

# The configuration the build will read, as the staged repository resolves it:
# the legs' values come from this result. The decision role names the
# provider, the endpoint and the layer that supplied the provider; that
# layer's directory is the source the layered leg copies. The text role names
# the text model, the scoring role the scoring model (a decision model may be
# decision-only, so scoring is read, not assumed). The provider's key kind
# (store, file or none) says where the layered leg's key source comes from.
sc="${out}/show-config-source"
capture "${sc}" "${repo}" "${kbase}" show-config ${cfg[@]+"${cfg[@]}"}
refuse() { printf 'integration(test-integration-build-live-arxiv): %s — see %s.out\n' "$1" "${sc}" >&2; exit 1; }
[[ "$(rc "${sc}")" -eq 0 ]] || refuse "show-config failed"
provider="$(doc_get "${sc}.out" roles role decision provider)"
decision="$(doc_get "${sc}.out" roles role decision model)"
text="$(doc_get "${sc}.out" roles role text model)"
base_url="$(doc_get "${sc}.out" roles role decision base-url)"
provider_layer="$(doc_get "${sc}.out" roles role decision provider-layer)"
[[ -n "${provider}" && -n "${decision}" && -n "${text}" && -n "${base_url}" && -n "${provider_layer}" ]] \
    || refuse "the decision and text roles do not both resolve to a provider and a model"
source_dir="$(doc_get "${sc}.out" layers layer "${provider_layer}" path)"
[[ -d "${source_dir}" ]] || refuse "the layer that supplied ${provider} (${provider_layer}) has no directory ('${source_dir}')"
key_kind="$(doc_get "${sc}.out" providers name "${provider}" key-source)"
scoring="$(doc_get "${sc}.out" roles role scoring model)"
[[ -n "${scoring}" ]] || refuse "the scoring role does not resolve to a model"
source_names=()
while IFS= read -r source_name; do source_names+=("${source_name}"); done \
    < <(doc_get "${sc}.out" providers layer "${provider_layer}" name)
printf 'config: %s from %s (layer %s); decision %s, text %s\n' "${provider}" "${source_dir}" "${provider_layer}" "${decision}" "${text}"

# The endpoint next: an unreachable role fails here, by name, and no build is
# started against it.
capture "${out}/probe-decision" "${repo}" "${kbase}" probe --role decision ${cfg[@]+"${cfg[@]}"}
if [[ "$(rc "${out}/probe-decision")" -ne 0 ]]; then
    printf 'integration(test-integration-build-live-arxiv): the decision role (provider %s) is unreachable — see %s/probe-decision.out\n' \
        "${provider}" "${out}" >&2
    exit 1
fi

# the build, asking, to completion
capture "${ck}/kbase-build" "${repo}" "${kbase}" build "${volume_root}" ${cfg[@]+"${cfg[@]}"} --state-dir "${state}" --log-level info
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

# The layering proof, which names no configuration directory at all: a
# --config-dir, where given, is single-layer mode. The global layer is the
# providers.toml and api_keys/ of the layer that supplied the provider, copied
# into this leg's own home — a relative apiKeyFile resolves beside the file,
# so those keys travel with it; keys held in the operator's key store stay
# there, the store's state base being passed through —
# and the repository layer is what configure-models --layer repository writes,
# dialing the resolved provider out of that pool and writing a reference for
# each role — decision, text and scoring from the resolved roles. The provider
# stands in that global pool, so the leg writes no provider entry.
# show-config then reports the endpoint from the global file and the three
# references from the repository's, with the operator's own
# ~/.config/kbase read by neither: HOME is this leg's directory, and the
# variable that would name a directory is unset. No second build runs: the
# first build proves a build uses what it resolves, and show-config and build
# resolve through the same config.Resolve.
layered="${root}/${dir}/layered"
layered_home="${layered}/home"
layered_repo="${layered}/repo"
# copy_pool <dir>: the providers.toml of the layer that supplied the provider,
# copied into dir. A relative apiKeyFile resolves beside its providers.toml,
# so the keys it names travel with it or its entries drop (keys in the
# operator's key store do not travel); owner-only, because
# a key file any other account can read is reported as a warning fault.
copy_pool() {
    mkdir -p "$1"
    cp "${source_dir}/providers.toml" "$1/providers.toml"
    if [[ -d "${source_dir}/api_keys" ]]; then
        cp -R "${source_dir}/api_keys" "$1/api_keys"
        find "$1/api_keys" -type f -exec chmod 0600 {} +
    fi
}
mkdir -p "${layered_repo}"
copy_pool "${layered_home}/.config/kbase"
git -C "${layered_repo}" init -q
# The operator's state base, captured before HOME moves: kbase reads a relative
# XDG_STATE_HOME as unset, so only an absolute one is kept.
state_home="${HOME}/.local/state"
[[ "${XDG_STATE_HOME:-}" != /* ]] || state_home="${XDG_STATE_HOME}"
layered_env=(-u KBASE_CONFIG_DIR "HOME=${layered_home}" "XDG_STATE_HOME=${state_home}")
capture "${ck}/kbase-configure-layered" "${layered_repo}" env "${layered_env[@]}" "${kbase}" configure-models \
    --layer repository --decision "${provider}/${decision}" --text "${provider}/${text}" --scoring "${provider}/${scoring}"
layered_refs=0
[[ "$(rc "${ck}/kbase-configure-layered")" -eq 0 ]] || layered_refs=1
grep -qx 'outcome: "done"' "${ck}/kbase-configure-layered.out" || layered_refs=1
grep -q '^    - ".*/\.kbase/config\.toml"$' "${ck}/kbase-configure-layered.out" || layered_refs=1
grep -qxF "    decision: \"${provider}/${decision}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
grep -qxF "    text: \"${provider}/${text}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
grep -qxF "    scoring: \"${provider}/${scoring}\"" "${ck}/kbase-configure-layered.out" || layered_refs=1
if grep -q '^provider *=' "${layered_repo}/.kbase/config.toml"; then layered_refs=1; fi
check configure-layered "${layered_refs}" \
    "${provider} dialed from the global pool; three references written into the repository layer, and no provider key: exit $(rc "${ck}/kbase-configure-layered")" \
    "${ck}/kbase-configure-layered.out"
capture "${ck}/kbase-show-config-layered" "${layered_repo}" env "${layered_env[@]}" "${kbase}" show-config
layered_show="${ck}/kbase-show-config-layered"
layered_pool=0
[[ "$(rc "${layered_show}")" -eq 0 ]] || layered_pool=1
grep -qx 'outcome: "done"' "${layered_show}.out" || layered_pool=1
# the two layers, the global one this leg's home — so no other directory
# was read
[[ "$(doc_get "${layered_show}.out" layers layer global path | wc -l | tr -d ' ')" -eq 1 ]] || layered_pool=1
[[ "$(doc_get "${layered_show}.out" layers layer repository path | wc -l | tr -d ' ')" -eq 1 ]] || layered_pool=1
grep -qxF "    - path: \"${layered_home}/.config/kbase\"" "${layered_show}.out" || layered_pool=1
# the merged pool: the source layer's entries out of the global file and no
# others (the repository layer holds no providers.toml), every entry usable
pool_size="${#source_names[@]}"
[[ "$(grep -c '^    - name: ' "${layered_show}.out")" -eq "${pool_size}" ]] || layered_pool=1
[[ "$(grep -cxF '      usable: true' "${layered_show}.out")" -eq "${pool_size}" ]] || layered_pool=1
[[ "$(doc_get "${layered_show}.out" providers layer global name | wc -l | tr -d ' ')" -eq "${pool_size}" ]] || layered_pool=1
[[ "$(doc_get "${layered_show}.out" providers layer repository name | wc -l | tr -d ' ')" -eq 0 ]] || layered_pool=1
for entry in "${source_names[@]}"; do
    grep -qxF "    - name: \"${entry}\"" "${layered_show}.out" || layered_pool=1
done
[[ "$(grep -cxF 'diagnostics: []' "${layered_show}.out")" -eq 1 ]] || layered_pool=1
check layered-pool "${layered_pool}" \
    "${pool_size} pool entries, ${source_names[*]}, all global and usable, none repository, no diagnostics; the global layer is ${layered_home}/.config/kbase" \
    "${layered_show}.out"
# The repository's config.toml names three references, and the result
# reports them verbatim; each role's record names the entry it asked, the
# rule, the layer, and the layer that supplied each field: the endpoint and
# the key from the key store or the global pool entry's key file, the model
# from the repository's file.
layered_sources=0
grep -qxF "        decision: \"${provider}/${decision}\"" "${layered_show}.out" || layered_sources=1
grep -qxF "        text: \"${provider}/${text}\"" "${layered_show}.out" || layered_sources=1
grep -qxF "        scoring: \"${provider}/${scoring}\"" "${layered_show}.out" || layered_sources=1
for role in decision text scoring; do
    grep -qxF "    - role: \"${role}\"" "${layered_show}.out" || layered_sources=1
done
[[ "$(grep -cxF "      provider: \"${provider}\"" "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF '      provider-rule: "config"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF '      provider-layer: "global"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
for role in decision text scoring; do
    [[ "$(doc_get "${layered_show}.out" roles role "${role}" base-url)" == "${base_url}" ]] || layered_sources=1
done
[[ "$(grep -cxF '        base-url: "global"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
[[ "$(grep -cxF '        model: "repository"' "${layered_show}.out")" -eq 3 ]] || layered_sources=1
# the key's source follows the provider's key kind: a stored key reports
# "store"; a key file travels with the copied pool and reports the global
# layer; no key reports neither
case "${key_kind}" in
    store) key_source=store ;;
    file) key_source=global ;;
    *) key_source= ;;
esac
key_expect=3
[[ -n "${key_source}" ]] || key_expect=0
[[ "$(grep -cxF "        key: \"${key_source}\"" "${layered_show}.out")" -eq "${key_expect}" ]] || layered_sources=1
# each distinct model across the three role records appears once per role
# naming it
for model in "${decision}" "${text}" "${scoring}"; do
    model_expect=0
    for named in "${decision}" "${text}" "${scoring}"; do
        [[ "${named}" != "${model}" ]] || model_expect=$(( model_expect + 1 ))
    done
    [[ "$(grep -cxF "      model: \"${model}\"" "${layered_show}.out")" -eq "${model_expect}" ]] || layered_sources=1
done
check layered-sources "${layered_sources}" \
    "three role records, each ${provider} out of the global pool with its model from the repository's file: base-url global, model repository, key ${key_source:-absent}; decision ${decision}, text ${text}, scoring ${scoring}" \
    "${layered_show}.out"

# A full score configured by --config-dir, against the KB the first build
# finished: a directory of this leg's own holding the pool of the layer that
# supplied the provider and a config.toml with the three references, so the
# operator's layers are not read. A dry run counts the items first (N); the
# run then asks all N, finishes done with none failed, its scoring role record
# naming the provider, the config rule and the scoring model, and its ledger
# naming that model; afterwards every claim's strength is derived, verify finds
# no fault, and a second run asks nothing. Each result is parsed against its
# documented keys.
score_config="${root}/${dir}/score-config"
copy_pool "${score_config}"
printf '%s\n' '[models]' "decision = \"${provider}/${decision}\"" "text = \"${provider}/${text}\"" \
    "scoring = \"${provider}/${scoring}\"" > "${score_config}/config.toml"
score_args=(--config-dir "${score_config}" --state-dir "${root}/${dir}/score-state")
# map_get <result> <section> <key>: <key>'s value in the mapping <section>, at
# whatever indent the section stands (top level, or under results:).
map_get() {
    awk -v sec="$2" -v key="$3" '
        { match($0, /^ */); ind = RLENGTH }
        in_sec && $0 != "" && ind <= base { exit }
        in_sec { line = substr($0, ind + 1); if (index(line, key ": ") == 1) { print substr(line, length(key) + 3); exit } }
        !in_sec && substr($0, ind + 1) == sec ":" { in_sec = 1; base = ind }' "$1"
}

capture "${ck}/kbase-score-dry" "${repo}" "${kbase}" score --dry-run "${score_args[@]}"
asked="$(map_get "${ck}/kbase-score-dry.out" counts remaining)"
check score-dry-run "$([[ "$(rc "${ck}/kbase-score-dry")" -eq 0 && "${asked}" =~ ^[0-9]+$ && "${asked}" -gt 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-score-dry.out" && echo 0 || echo 1)" \
    "exit $(rc "${ck}/kbase-score-dry"), ${asked:-no count} items would be asked" "${ck}/kbase-score-dry.out"
keys "${ck}/kbase-score-dry" score done

capture "${ck}/kbase-score" "${repo}" "${kbase}" score "${score_args[@]}"
check score-done "$([[ "$(rc "${ck}/kbase-score")" -eq 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-score.out" && echo 0 || echo 1)" \
    "exit $(rc "${ck}/kbase-score"), $(grep -m1 '^outcome:' "${ck}/kbase-score.out" || echo 'no outcome')" "${ck}/kbase-score.out"
score_role=0
[[ "$(doc_get "${ck}/kbase-score.out" roles role scoring provider)" == "${provider}" ]] || score_role=1
[[ "$(doc_get "${ck}/kbase-score.out" roles role scoring provider-rule)" == config ]] || score_role=1
[[ "$(doc_get "${ck}/kbase-score.out" roles role scoring model)" == "${scoring}" ]] || score_role=1
score_ledger="$(sed -n 's/^ledger: "\(.*\)"$/\1/p' "${ck}/kbase-score.out" | head -n 1)"
[[ -n "${score_ledger}" && -f "${score_ledger}/ledger.jsonl" ]] || score_role=1
grep -qF "\"model\":\"${scoring}\"" "${score_ledger}/ledger.jsonl" || score_role=1
check score-role "${score_role}" \
    "the scoring role record names ${provider}/${scoring} under the config rule, against ${base_url}, and the ledger names the model" "${ck}/kbase-score.out"
score_done="$(map_get "${ck}/kbase-score.out" counts done)"
score_failed="$(map_get "${ck}/kbase-score.out" counts failed)"
check score-all-asked "$([[ "${score_done}" == "${asked}" && "${score_failed}" == 0 ]] && echo 0 || echo 1)" \
    "${score_done:-no} items done of the ${asked} the dry run counted, ${score_failed:-no} failed" "${ck}/kbase-score.out"
keys "${ck}/kbase-score" score done

capture "${ck}/kbase-stats" "${repo}" "${kbase}" stats
unknown_claims="$(map_get "${ck}/kbase-stats.out" strength-bands unknown)"
check strength-derived "$([[ "$(rc "${ck}/kbase-stats")" -eq 0 && "${unknown_claims}" == 0 ]] && echo 0 || echo 1)" \
    "strength-bands.unknown is ${unknown_claims:-absent}" "${ck}/kbase-stats.out"
keys "${ck}/kbase-stats" stats done

capture "${ck}/kbase-verify" "${repo}" "${kbase}" verify
check verify-clean "$([[ "$(rc "${ck}/kbase-verify")" -eq 0 ]] && echo 0 || echo 1)" \
    "exit $(rc "${ck}/kbase-verify")" "${ck}/kbase-verify.out"
keys "${ck}/kbase-verify" verify done

capture "${ck}/kbase-score-again" "${repo}" "${kbase}" score "${score_args[@]}"
again_done="$(map_get "${ck}/kbase-score-again.out" counts done)"
again_remaining="$(map_get "${ck}/kbase-score-again.out" counts remaining)"
check score-again-asks-nothing "$([[ "$(rc "${ck}/kbase-score-again")" -eq 0 && "${again_done}" == 0 && "${again_remaining}" == 0 ]] && grep -qx 'outcome: "done"' "${ck}/kbase-score-again.out" && grep -qx 'scored: \[\]' "${ck}/kbase-score-again.out" && echo 0 || echo 1)" \
    "exit $(rc "${ck}/kbase-score-again"), ${again_done:-no} done and ${again_remaining:-no} remaining, nothing scored" "${ck}/kbase-score-again.out"
keys "${ck}/kbase-score-again" score done

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

OUT_DIR="${out}" RECIPE="test-integration-build-live-arxiv" INVOCATION="on ${BUILD_ARXIV_SPECIAL_ID}, staged into <id>/repo (sources, stub Makefile, .gitignore); [--config-dir <config>] is passed to every call below that is not in the layered leg, and only where config is given:\n(cd <id>/repo && ${kbase} show-config [--config-dir <config>]) into show-config-source.*, whose roles, providers and layers give the provider, the three models, base URL, key kind and source directory the legs use\n(cd <id>/repo && ${kbase} probe --role decision [--config-dir <config>]) into probe-decision.*\n(cd <id>/repo && ${kbase} build <volume-root> [--config-dir <config>] --state-dir <id>/state --log-level info) into <id>/checks/kbase-build.*\ngo test -run '^TestLiveBuildCounts\$' ./internal/claimgraph -args -claimgraph.repo=<id>/repo -claimgraph.result=<id>/checks/kbase-build.out -claimgraph.out=<id>\n(cd <id>/layered/repo && env -u KBASE_CONFIG_DIR HOME=<id>/layered/home XDG_STATE_HOME=<the operator's absolute XDG_STATE_HOME, else \$HOME/.local/state> ${kbase} configure-models --layer repository --decision <the decision role's provider>/<the decision role's model> --text <provider>/<the text role's model> --scoring <provider>/<the scoring role's model>; ${kbase} show-config) into <id>/checks/kbase-configure-layered.* and kbase-show-config-layered.*, in <id>/layered/repo below <id>/layered/home/.config/kbase (the providers.toml and api_keys of the layer that supplied the provider, the global layer, over .kbase/ that configure-models wrote, the repository layer)\n(cd <id>/repo && ${kbase} score --dry-run --config-dir <id>/score-config --state-dir <id>/score-state) into <id>/checks/kbase-score-dry.*, <id>/score-config holding the providers.toml and api_keys of the layer that supplied the provider and a config.toml with decision=<provider>/<the decision role's model>, text=<provider>/<the text role's model>, scoring=<provider>/<the scoring role's model>: counts.remaining is N, the items a full run would ask\n(cd <id>/repo && ${kbase} score --config-dir <id>/score-config --state-dir <id>/score-state) into <id>/checks/kbase-score.*: outcome done, counts.done N and counts.failed 0, the scoring role record naming the provider, provider-rule config and the scoring model, and the ledger naming it\n(cd <id>/repo && ${kbase} stats) into <id>/checks/kbase-stats.*: strength-bands.unknown 0\n(cd <id>/repo && ${kbase} verify) into <id>/checks/kbase-verify.*: exit 0, no refusals\n(cd <id>/repo && ${kbase} score --config-dir <id>/score-config --state-dir <id>/score-state) again into <id>/checks/kbase-score-again.*: outcome done, counts.done and counts.remaining 0, scored []\ngo test -count=1 -run '^TestResult\$' ./cmd -args -result.file=<id>/checks/<stem>.out -result.verb=<verb> -result.outcome=done > <id>/checks/<stem>.keys.log, for each of the five results above" \
    tools/scripts/write-evidence.sh
printf '\nReference: %s\n' "${pandoc_version}" >> "${out}/EVIDENCE.md"
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-build-live-arxiv): a check failed — see comparison.yaml\n'
    exit 1
fi
printf 'integration(test-integration-build-live-arxiv) ok\n'
