#!/usr/bin/env bash
# The show-config verb over the delivered binary, on three staged configuration
# directories — absent, providers.toml only, both files — each run through
# KBASE_CONFIG_DIR and through --config-dir: the outcome, the one layer each
# reports and the layer's source, the diagnostic count and the absence of the
# key-file value from stdout. A pool entry that drops and one whose key file any
# local account can read are what the diagnostics count. A layered leg stages a repository's own .kbase over a
# staged $HOME's ~/.config/kbase — the repository adding a pool entry and
# overriding one by name — and checks the merged pool, each entry's layer,
# whole-entry replacement, both layers in precedence order with what each holds,
# and the layer each resolved field came from. Five further legs cover a leading
# ~: the configuration directory a flag names and the one $KBASE_CONFIG_DIR
# names, ~ alone, an entry's apiKeyFile (which must expand rather than join to
# the configuration directory), ~user and a/~/b staying literal, and a home the
# lookup cannot give or one that is not absolute, which leaves the spelling
# alone. Three sampling legs stage a configuration directory each: settings on an
# openai entry (done, the text role reporting temperature and thinking with the
# layer that supplied each, the scoring role null), a [sampling.decision] table
# holding a key (refused by strict load, naming sampling.decision.temperature),
# and temperature 1.5 on an anthropic text entry (done, one diagnostic naming
# sampling.text.temperature). The last leg calls the verb over MCP from a directory holding no KB,
# which is what a verb resolving no KB must answer.
# It needs git only to stage the layered leg's repository; no pandoc, no
# provider and no corpus.
#
# Inputs (environment): BIN_DIR; CONFIG_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${CONFIG_OUT_DIR:?CONFIG_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
out="${root}/${CONFIG_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
dirs="${out}/configs"
docs="${out}/docs"
work="${out}/work"
rm -rf "${dirs}" "${docs}" "${work}" "${out}/home" "${out}/layered" "${out}/literal" "${out}/configure" "${out}/sampling" "${out}/mcp.requests.jsonl" "${out}/mcp.actual.jsonl" "${out}/mcp.log" "${out}/mcp.exit" "${out}/config.tsv"
mkdir -p "${dirs}/pool" "${dirs}/both" "${docs}" "${work}"
file_key='sk-file-MUST-NOT-APPEAR'
# The home every invocation runs under, so the global layer is this staged
# ~/.config/kbase and no leg reads or writes the operator's own. It is left
# empty: a leg that names a directory reads that one layer only.
home="${out}/home"
mkdir -p "${home}"
# One providers.toml, staged into the two states that hold it: an entry that
# drops for want of a baseUrl and one supplying its key from a file any local
# account can read — so the permission warning fires. configs/absent is never
# created: that is the absent state.
printf '%s\n' \
    '[broken]' \
    '' '[file]' 'baseUrl = "http://file.example/v1"' 'apiKeyFile = "key.txt"' \
    > "${dirs}/pool/providers.toml"
printf '%s\n' "${file_key}" > "${dirs}/pool/key.txt"
chmod 0644 "${dirs}/pool/key.txt"
cp -R "${dirs}/pool/." "${dirs}/both/"
printf '%s\n' '[models]' 'decision = "file/fixture-decision"' 'text = "file/fixture-text"' 'scoring = "file/fixture-scoring"' \
    > "${dirs}/both/config.toml"
status=0
fail() { printf '  fail: %s\n' "$*"; status=1; }
# role_field <document> <role> <key>: one scalar of the roles entry the named
# role is; the entry's first key sits on its "- role:" line, the rest below it.
role_field() { awk -v r="$2" -v k="$3" '/^roles:/ { on = 1; next } on && /^    - role: / { cur = $0; sub(/^[^:]*: /, "", cur); gsub(/^"|"$/, "", cur); next } on && /^    / { if (cur == r && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "$1"; }
# role_source <document> <role> <key>: the layer the document names for one
# field of that role's sources record.
role_source() { awk -v r="$2" -v k="$3" '/^roles:/ { on = 1; next } on && /^    - role: / { cur = $0; sub(/^[^:]*: /, "", cur); gsub(/^"|"$/, "", cur); next } on && /^    / { if (cur == r && index($0, k ": ") == 9) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "$1"; }
# selection_field <document> <key>: one scalar of the selection record.
selection_field() { awk -v k="$2" '/^selection:/ { on = 1; next } on && /^    / { if (index($0, k ": ") == 5) { sub(/^[^:]*: /, ""); gsub(/^"|"$/, ""); print; exit }; next } on { exit }' "$1"; }
# item_field <document> <list> <key>: one scalar of an item under the named
# list (refusals, failures); the item's first key sits on its "- " line, the
# rest below it.
item_field() { awk -v list="$2" -v k="$3" '$0 == list ":" { on = 1; next } on && /^    - / { line = "      " substr($0, 7); if (index(line, k ": ") == 7) { sub(/^[^:]*: /, "", line); gsub(/^"|"$/, "", line); print line; exit } next } on && /^      / { if (index($0, k ": ") == 7) { sub(/^[^:]*: /, ""); gsub(/^"|"$/, ""); print; exit }; next } on { exit }' "$1"; }
# diagnostics <stem>: how many items the document's diagnostics list holds.
diagnostics() { awk '/^diagnostics:/ { on = 1; next } on && /^    - / { n++ } END { print n + 0 }' "$1.out"; }
# layers <stem>: how many layer entries the document reports.
layers() { awk '/^layers:/ { on = 1; next } on && /^    - / { n++; next } on && /^    / { next } on { exit } END { print n + 0 }' "$1.out"; }
# layer_field <stem> <n> <key>: one scalar of the nth layer entry (1-based);
# the entry's first key sits on its "- path:" line, the rest below it.
layer_field() { awk -v n="$2" -v k="$3" '/^layers:/ { on = 1; next } on && /^    - / { i++; if (i == n && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit } next } on && /^    / { if (i == n && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "$1.out"; }
# provider_names <stem>: the pool's entry names, in the order the document lists them.
provider_names() { awk '/^providers:/ { on = 1; next } on && /^    - name: / { n = $0; sub(/^[^:]*: /, "", n); gsub(/^"|"$/, "", n); printf "%s%s", sep, n; sep = ","; next } on && /^    / { next } on { exit }' "$1.out"; }
# provider_field <stem> <name> <key>: one scalar of the providers entry called name.
provider_field() { awk -v want="$2" -v k="$3" '/^providers:/ { on = 1; next } on && /^    - name: / { cur = $0; sub(/^[^:]*: /, "", cur); gsub(/^"|"$/, "", cur); next } on && /^    / { if (cur == want && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "$1.out"; }
# report <stem> <ok> <label> [detail]: one leg that checks its document
# itself — its config.tsv line, its console line, and the run's status. The
# console line states the layer and diagnostic counts a show-config document
# carries, or detail where the leg's document is not one.
report() {
    local verdict=pass
    [[ "$2" -eq 0 ]] || { verdict=fail; status=1; }
    printf '%s\t-\t%s\t%s\n' "$3" "${verdict}" "$1.out" >> "${out}/config.tsv"
    printf '  %s %s: %s, exit %s\n' \
        "${verdict}" "$3" "${4:-$(layers "$1") layers, $(diagnostics "$1") diagnostics}" "$(cat "$1.exit")"
}

# run <state> <route> <diagnostics> <files> [args...]: one invocation
# against the staged configuration named by state, its document checked for
# the outcome, the one layer it reads, that layer's source (the environment
# for a route through KBASE_CONFIG_DIR, explicit for --config-dir), which of
# its files stand (files: none, pool or both), the count of diagnostics, and
# the absence of the key-file value. HOME points at the staged home, so a
# leg never reads the operator's own.
n=0
run() {
    local state="$1" route="$2" want_count="$3" files="$4" dir stem want_source
    shift 4
    n=$((n + 1))
    dir="${dirs}/${state}"
    stem="${docs}/$(printf '%02d' "${n}")-${state}-${route}"
    local -a envargs=("HOME=${home}")
    local -a args=(show-config)
    if [[ "${route}" == environment ]]; then
        want_source=environment
        envargs+=("KBASE_CONFIG_DIR=${dir}")
    else
        want_source=explicit
        args+=(--config-dir "${dir}")
    fi
    set +e
    ( cd "${work}" && env "${envargs[@]}" "${kbase}" "${args[@]}" "$@" ) > "${stem}.out" 2> "${stem}.log"
    printf '%s\n' "$?" > "${stem}.exit"
    set -e
    local ok=0 got_source got_count invocation="${args[*]}"
    local want_exists=false want_providers=false want_config=false
    case "${files}" in
        none) ;;
        pool) want_exists=true; want_providers=true ;;
        both) want_exists=true; want_providers=true; want_config=true ;;
        *) ok=1 ;;
    esac
    [[ "${route}" == environment ]] && invocation="KBASE_CONFIG_DIR=${dir} ${invocation}"
    got_source="$(layer_field "${stem}" 1 source)"
    got_count="$(diagnostics "${stem}")"
    [[ "$(cat "${stem}.exit")" -eq 0 ]] || ok=1
    grep -qx 'outcome: "done"' "${stem}.out" || ok=1
    [[ "$(layers "${stem}")" == 1 ]] || ok=1
    [[ "${got_source}" == "${want_source}" ]] || ok=1
    [[ "${got_count}" == "${want_count}" ]] || ok=1
    [[ "$(layer_field "${stem}" 1 path)" == "${dir}" ]] || ok=1
    [[ "$(layer_field "${stem}" 1 exists)" == "${want_exists}" ]] || ok=1
    [[ "$(layer_field "${stem}" 1 providersExists)" == "${want_providers}" ]] || ok=1
    [[ "$(layer_field "${stem}" 1 configExists)" == "${want_config}" ]] || ok=1
    if grep -qF "${file_key}" "${stem}.out"; then ok=1; fi
    printf '%s\t%s\t%s\t%s\n' "${invocation}" "${want_source}" "$([[ "${ok}" -eq 0 ]] && echo pass || echo fail)" "${stem}.out" >> "${out}/config.tsv"
    printf '  %s %s %s: source %s, files %s, %s diagnostics, exit %s\n' \
        "$([[ "${ok}" -eq 0 ]] && echo pass || echo fail)" "${state}" "${route}" "${got_source:-none}" "${files}" "${got_count}" "$(cat "${stem}.exit")"
    [[ "${ok}" -eq 0 ]] || status=1
    LAST="${stem}.out"
}
# 1. absent: no configuration directory at all — nothing selected, and one
#    diagnostic per role, each naming the models.<role> key the missing
#    config.toml would carry.
run absent environment 3 none
run absent flag 3 none
# 2. providers.toml only: the drop and the key file's warning, and one
#    diagnostic per role: a pool with no config.toml names no model.
run pool environment 5 pool
run pool flag 5 pool
# 3. both files: config.toml names the key-file entry in each role's
#    reference, so every role resolves and the drop and the warning are all
#    that is diagnosed.
run both environment 2 both
run both flag 2 both
[[ "$(role_field "${LAST}" decision provider)" == file ]] || fail "the decision role does not name the pool entry its reference names in ${LAST}"
[[ "$(role_field "${LAST}" decision model)" == fixture-decision ]] || fail "the decision role does not name its reference's model part in ${LAST}"
[[ "$(role_source "${LAST}" text model)" == explicit ]] || fail "the text role's model does not name the layer that supplied it in ${LAST}"
[[ "$(role_source "${LAST}" text key)" == explicit ]] || fail "the text role's key does not name the layer that supplied it in ${LAST}"
[[ "$(grep -c 'usable: false' "${LAST}")" -eq 1 ]] || fail "the dropped entry is not reported unusable in ${LAST}"
# 5. layered: <repository>/.kbase over the staged home's ~/.config/kbase,
#    run from a directory below the repository's root. The repository pool
#    adds one entry and overrides one by name; its config.toml overrides the
#    decision role's reference and leaves text and scoring to the global file.
layered="${out}/layered"
repo="${layered}/repo"
mkdir -p "${layered}/home/.config/kbase" "${repo}/.kbase" "${repo}/nested"
printf '%s\n' \
    '[shared]' 'baseUrl = "http://shared.example/v1"' 'apiKeyFile = "shared.key"' \
    '' '[inherited]' 'baseUrl = "http://inherited.example/v1"' \
    > "${layered}/home/.config/kbase/providers.toml"
printf '%s\n' "${file_key}-global" > "${layered}/home/.config/kbase/shared.key"
chmod 0600 "${layered}/home/.config/kbase/shared.key"
printf '%s\n' '[models]' 'decision = "inherited/global-decision"' 'text = "inherited/global-text"' 'scoring = "inherited/global-scoring"' \
    > "${layered}/home/.config/kbase/config.toml"
printf '%s\n' \
    '[shared]' 'baseUrl = "http://shared.example/v2"' 'decisionsFlavor = "decisions"' \
    '' '[local]' 'baseUrl = "http://local.example/v1"' 'apiKeyFile = "local.key"' \
    > "${repo}/.kbase/providers.toml"
printf '%s\n' "${file_key}-repo" > "${repo}/.kbase/local.key"
chmod 0600 "${repo}/.kbase/local.key"
printf '%s\n' '[models]' 'decision = "local/repo-decision"' \
    > "${repo}/.kbase/config.toml"
git init -q "${repo}"
layered_stem="${docs}/08-layered"
set +e
( cd "${repo}/nested" && env HOME="${layered}/home" "${kbase}" show-config ) > "${layered_stem}.out" 2> "${layered_stem}.log"
printf '%s\n' "$?" > "${layered_stem}.exit"
set -e
layered_ok=0
[[ "$(cat "${layered_stem}.exit")" -eq 0 ]] || layered_ok=1
grep -qx 'outcome: "done"' "${layered_stem}.out" || layered_ok=1
[[ "$(diagnostics "${layered_stem}")" == 0 ]] || layered_ok=1
if grep -qF "${file_key}" "${layered_stem}.out"; then layered_ok=1; fi
# (a) the pool is the union of both layers, each entry naming its own.
[[ "$(provider_names "${layered_stem}")" == "inherited,local,shared" ]] || layered_ok=1
[[ "$(provider_field "${layered_stem}" inherited layer)" == "global" ]] || layered_ok=1
[[ "$(provider_field "${layered_stem}" local layer)" == "repository" ]] || layered_ok=1
[[ "$(provider_field "${layered_stem}" shared layer)" == "repository" ]] || layered_ok=1
# (b) the repository entry replaced the global one whole: its URL stands, and
#     neither the apiKeyFile nor the key the global entry supplied came with
#     it.
[[ "$(provider_field "${layered_stem}" shared base-url)" == "http://shared.example/v2" ]] || layered_ok=1
[[ "$(provider_field "${layered_stem}" shared key)" == "none" ]] || layered_ok=1
[[ -z "$(provider_field "${layered_stem}" shared key-file)" ]] || layered_ok=1
# (b) Each entry states the decision surface it speaks: the repository's
#     shared entry names one, and the local entry, silent, resolves to the
#     systemone default.
[[ "$(provider_field "${layered_stem}" shared decisions-flavor)" == "decisions" ]] || layered_ok=1
[[ "$(provider_field "${layered_stem}" local decisions-flavor)" == "systemone" ]] || layered_ok=1
# (c) both layers in precedence order, each with what it holds.
[[ "$(layers "${layered_stem}")" == 2 ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 1 path)" == "${repo}/.kbase" ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 1 source)" == "repository" ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 1 exists)" == true ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 1 providersExists)" == true ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 1 configExists)" == true ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 2 path)" == "${layered}/home/.config/kbase" ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 2 source)" == "global" ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 2 exists)" == true ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 2 providersExists)" == true ]] || layered_ok=1
[[ "$(layer_field "${layered_stem}" 2 configExists)" == true ]] || layered_ok=1
# (d) each role names the layer that supplied it: the repository's pool entry
#     and its config.toml for the decision role, the global file and the
#     entry it names for text and scoring, which the repository left alone.
[[ "$(role_field "${layered_stem}.out" decision provider)" == "local" ]] || layered_ok=1
[[ "$(role_field "${layered_stem}.out" decision provider-layer)" == "repository" ]] || layered_ok=1
[[ "$(role_field "${layered_stem}.out" decision model)" == "repo-decision" ]] || layered_ok=1
[[ "$(role_field "${layered_stem}.out" text provider)" == "inherited" ]] || layered_ok=1
[[ "$(role_field "${layered_stem}.out" text model)" == "global-text" ]] || layered_ok=1
[[ "$(role_field "${layered_stem}.out" scoring model)" == "global-scoring" ]] || layered_ok=1
[[ "$(role_field "${layered_stem}.out" decision base-url)" == "http://local.example/v1" ]] || layered_ok=1
[[ "$(role_field "${layered_stem}.out" text base-url)" == "http://inherited.example/v1" ]] || layered_ok=1
[[ "$(role_source "${layered_stem}.out" decision base-url)" == "repository" ]] || layered_ok=1
[[ "$(role_source "${layered_stem}.out" decision key)" == "repository" ]] || layered_ok=1
[[ "$(role_source "${layered_stem}.out" decision model)" == "repository" ]] || layered_ok=1
[[ "$(role_source "${layered_stem}.out" text base-url)" == "global" ]] || layered_ok=1
[[ "$(role_source "${layered_stem}.out" text model)" == "global" ]] || layered_ok=1
# The global entry names no key, so the role it supplies names no key source:
# null, not a layer.
[[ "$(role_source "${layered_stem}.out" text key)" == null ]] || layered_ok=1
report "${layered_stem}" "${layered_ok}" "layered show-config"
# 6. ~ expands. A leading ~ in the directory a flag names and in an entry's
#    apiKeyFile resolves from the staged home. The key file stands under the
#    home alone, so an apiKeyFile joined to the configuration directory
#    instead would drop its entry: the diagnostic count and the entry's
#    usability are what say it did not. The report keeps the entry's own
#    spelling of the path.
home_cfg="${home}/cfg"
mkdir -p "${home_cfg}" "${home}/keys"
printf '%s\n' "${file_key}" > "${home}/keys/home.txt"
chmod 0600 "${home}/keys/home.txt"
printf '%s\n' '[homekey]' 'baseUrl = "http://home.example/v1"' 'apiKeyFile = "~/keys/home.txt"' \
    > "${home_cfg}/providers.toml"
printf '%s\n' '[models]' 'decision = "homekey/home-decision"' 'text = "homekey/home-text"' 'scoring = "homekey/home-scoring"' \
    > "${home_cfg}/config.toml"
tilde_stem="${docs}/09-tilde-flag"
set +e
( cd "${work}" && env HOME="${home}" "${kbase}" show-config --config-dir '~/cfg' ) > "${tilde_stem}.out" 2> "${tilde_stem}.log"
printf '%s\n' "$?" > "${tilde_stem}.exit"
set -e
tilde_ok=0
[[ "$(cat "${tilde_stem}.exit")" -eq 0 ]] || tilde_ok=1
grep -qx 'outcome: "done"' "${tilde_stem}.out" || tilde_ok=1
[[ "$(layers "${tilde_stem}")" == 1 ]] || tilde_ok=1
[[ "$(layer_field "${tilde_stem}" 1 path)" == "${home_cfg}" ]] || tilde_ok=1
[[ "$(layer_field "${tilde_stem}" 1 source)" == "explicit" ]] || tilde_ok=1
[[ "$(diagnostics "${tilde_stem}")" == 0 ]] || tilde_ok=1
[[ "$(provider_field "${tilde_stem}" homekey usable)" == true ]] || tilde_ok=1
[[ "$(provider_field "${tilde_stem}" homekey key)" == file ]] || tilde_ok=1
[[ "$(provider_field "${tilde_stem}" homekey key-file)" == '~/keys/home.txt' ]] || tilde_ok=1
if grep -qF "${file_key}" "${tilde_stem}.out"; then tilde_ok=1; fi
report "${tilde_stem}" "${tilde_ok}" "show-config --config-dir '~/cfg'"
# 7. $KBASE_CONFIG_DIR expands the same way.
tilde_env_stem="${docs}/10-tilde-environment"
set +e
( cd "${work}" && env HOME="${home}" KBASE_CONFIG_DIR='~/cfg' "${kbase}" show-config ) > "${tilde_env_stem}.out" 2> "${tilde_env_stem}.log"
printf '%s\n' "$?" > "${tilde_env_stem}.exit"
set -e
tilde_env_ok=0
[[ "$(cat "${tilde_env_stem}.exit")" -eq 0 ]] || tilde_env_ok=1
grep -qx 'outcome: "done"' "${tilde_env_stem}.out" || tilde_env_ok=1
[[ "$(layers "${tilde_env_stem}")" == 1 ]] || tilde_env_ok=1
[[ "$(layer_field "${tilde_env_stem}" 1 path)" == "${home_cfg}" ]] || tilde_env_ok=1
[[ "$(layer_field "${tilde_env_stem}" 1 source)" == "environment" ]] || tilde_env_ok=1
[[ "$(diagnostics "${tilde_env_stem}")" == 0 ]] || tilde_env_ok=1
report "${tilde_env_stem}" "${tilde_env_ok}" "KBASE_CONFIG_DIR='~/cfg' show-config"
# 8. ~ alone is the home directory: the layer is the staged home, holding
#    neither configuration file.
tilde_home_stem="${docs}/11-tilde-home"
set +e
( cd "${work}" && env HOME="${home}" "${kbase}" show-config --config-dir '~' ) > "${tilde_home_stem}.out" 2> "${tilde_home_stem}.log"
printf '%s\n' "$?" > "${tilde_home_stem}.exit"
set -e
tilde_home_ok=0
[[ "$(cat "${tilde_home_stem}.exit")" -eq 0 ]] || tilde_home_ok=1
grep -qx 'outcome: "done"' "${tilde_home_stem}.out" || tilde_home_ok=1
[[ "$(layers "${tilde_home_stem}")" == 1 ]] || tilde_home_ok=1
[[ "$(layer_field "${tilde_home_stem}" 1 path)" == "${home}" ]] || tilde_home_ok=1
[[ "$(layer_field "${tilde_home_stem}" 1 exists)" == true ]] || tilde_home_ok=1
[[ "$(layer_field "${tilde_home_stem}" 1 providersExists)" == false ]] || tilde_home_ok=1
[[ "$(layer_field "${tilde_home_stem}" 1 configExists)" == false ]] || tilde_home_ok=1
report "${tilde_home_stem}" "${tilde_home_ok}" "show-config --config-dir '~'"
# 9. A ~ anywhere but the start is literal: the two key files below stand
#    under names that only a join to the configuration directory reaches, so
#    an expansion of ~user or of a/~/b would drop the entry naming it.
literal="${out}/literal"
mkdir -p "${literal}/~user" "${literal}/a/~/b"
printf '%s\n' "${file_key}" > "${literal}/~user/key.txt"
printf '%s\n' "${file_key}" > "${literal}/a/~/b/key.txt"
chmod 0600 "${literal}/~user/key.txt" "${literal}/a/~/b/key.txt"
printf '%s\n' \
    '[userdir]' 'baseUrl = "http://user.example/v1"' 'apiKeyFile = "~user/key.txt"' \
    '' '[inside]' 'baseUrl = "http://inside.example/v1"' 'apiKeyFile = "a/~/b/key.txt"' \
    > "${literal}/providers.toml"
printf '%s\n' '[models]' 'decision = "userdir/literal-decision"' 'text = "userdir/literal-text"' 'scoring = "userdir/literal-scoring"' \
    > "${literal}/config.toml"
literal_stem="${docs}/12-tilde-literal"
set +e
( cd "${work}" && env HOME="${home}" "${kbase}" show-config --config-dir "${literal}" ) > "${literal_stem}.out" 2> "${literal_stem}.log"
printf '%s\n' "$?" > "${literal_stem}.exit"
set -e
literal_ok=0
[[ "$(cat "${literal_stem}.exit")" -eq 0 ]] || literal_ok=1
grep -qx 'outcome: "done"' "${literal_stem}.out" || literal_ok=1
[[ "$(diagnostics "${literal_stem}")" == 0 ]] || literal_ok=1
[[ "$(provider_names "${literal_stem}")" == "inside,userdir" ]] || literal_ok=1
[[ "$(provider_field "${literal_stem}" userdir usable)" == true ]] || literal_ok=1
[[ "$(provider_field "${literal_stem}" inside usable)" == true ]] || literal_ok=1
[[ "$(provider_field "${literal_stem}" userdir key-file)" == '~user/key.txt' ]] || literal_ok=1
[[ "$(provider_field "${literal_stem}" inside key-file)" == 'a/~/b/key.txt' ]] || literal_ok=1
report "${literal_stem}" "${literal_ok}" "show-config --config-dir with ~ not leading"
# 10. A home the lookup cannot give, or one that is not an absolute path,
#     leaves the spelling as written: the layer is the literal string, and
#     nothing stands there.
leg=13
for home_value in "" "relative/home"; do
    label=no-home
    [[ -z "${home_value}" ]] || label=relative-home
    stem="${docs}/$(printf '%02d' "${leg}")-tilde-${label}"
    leg=$((leg + 1))
    set +e
    ( cd "${work}" && env HOME="${home_value}" "${kbase}" show-config --config-dir '~/cfg' ) > "${stem}.out" 2> "${stem}.log"
    printf '%s\n' "$?" > "${stem}.exit"
    set -e
    ok=0
    [[ "$(cat "${stem}.exit")" -eq 0 ]] || ok=1
    grep -qx 'outcome: "done"' "${stem}.out" || ok=1
    [[ "$(layers "${stem}")" == 1 ]] || ok=1
    [[ "$(layer_field "${stem}" 1 path)" == '~/cfg' ]] || ok=1
    [[ "$(layer_field "${stem}" 1 source)" == "explicit" ]] || ok=1
    [[ "$(layer_field "${stem}" 1 exists)" == false ]] || ok=1
    report "${stem}" "${ok}" "show-config --config-dir '~/cfg' with HOME='${home_value}'"
done
# 11. configure --create in a repository: --project names <repo>/.kbase
#     whether or not it stands, so one call creates both files there from
#     their templates and names them as written. No --provider is given, so
#     nothing is dialed and stderr says the ids went unchecked; show-config
#     then reports that layer holding both, and models resolves the one live
#     pool entry — [local] — by the sole-entry rule. A second identical call
#     finds every file holding what it would write.
cdir="${out}/configure"
crepo="${cdir}/repo"
chome="${cdir}/home"
mkdir -p "${crepo}/nested" "${chome}"
git init --quiet "${crepo}"
# scrub: what every configure leg runs under — the configuration-directory
# variable unset, so the leg reads and writes the staged tree alone, and HOME
# on the staged home.
scrub=(-u KBASE_CONFIG_DIR "HOME=${chome}")
create_stem="${docs}/15-configure-create-project"
set +e
( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project ) > "${create_stem}.out" 2> "${create_stem}.log"
printf '%s\n' "$?" > "${create_stem}.exit"
set -e
create_ok=0
[[ "$(cat "${create_stem}.exit")" -eq 0 ]] || create_ok=1
grep -qx 'outcome: "done"' "${create_stem}.out" || create_ok=1
grep -qx 'provider: null' "${create_stem}.out" || create_ok=1
grep -qx "    - \"${crepo}/.kbase/providers.toml\"" "${create_stem}.out" || create_ok=1
grep -qx "    - \"${crepo}/.kbase/config.toml\"" "${create_stem}.out" || create_ok=1
# The two files in the order --create writes them: the pool, then the
# choices.
[[ "$(sed -n 's/^    - "\(.*\)"$/\1/p' "${create_stem}.out" | tr '\n' ' ')" == \
    "${crepo}/.kbase/providers.toml ${crepo}/.kbase/config.toml " ]] || create_ok=1
grep -q 'the references were not checked' "${create_stem}.log" || create_ok=1
grep -qx '\[local\]' "${crepo}/.kbase/providers.toml" || create_ok=1
# Nothing else is written: the harness profiles a run may execute are
# kbase's, so there is no file a user stocks with commands.
[[ ! -e "${crepo}/.kbase/harnesses.toml" ]] || create_ok=1
report "${create_stem}" "${create_ok}" "configure --create --project" "created ${crepo}/.kbase from the two templates, ids unchecked"
created_show="${docs}/16-configure-created-layer"
set +e
( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" show-config ) > "${created_show}.out" 2> "${created_show}.log"
printf '%s\n' "$?" > "${created_show}.exit"
set -e
created_show_ok=0
[[ "$(cat "${created_show}.exit")" -eq 0 ]] || created_show_ok=1
[[ "$(layers "${created_show}")" == 2 ]] || created_show_ok=1
[[ "$(layer_field "${created_show}" 1 path)" == "${crepo}/.kbase" ]] || created_show_ok=1
[[ "$(layer_field "${created_show}" 1 source)" == repository ]] || created_show_ok=1
[[ "$(layer_field "${created_show}" 1 exists)" == true ]] || created_show_ok=1
[[ "$(layer_field "${created_show}" 1 providersExists)" == true ]] || created_show_ok=1
[[ "$(layer_field "${created_show}" 1 configExists)" == true ]] || created_show_ok=1
[[ "$(selection_field "${created_show}.out" provider)" == local ]] || created_show_ok=1
[[ "$(selection_field "${created_show}.out" rule)" == sole-entry ]] || created_show_ok=1
# The template writes the model keys blank, which is exactly the
# fresh-install diagnostic: one item per role, naming models.<role>.
[[ "$(diagnostics "${created_show}")" == 3 ]] || created_show_ok=1
for role in decision text scoring; do
    grep -qF "key: \"models.${role}\"" "${created_show}.out" || created_show_ok=1
done
report "${created_show}" "${created_show_ok}" "show-config over the created repository layer"
created_models="${docs}/17-configure-models"
set +e
( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" models --timeout 300ms ) > "${created_models}.out" 2> "${created_models}.log"
printf '%s\n' "$?" > "${created_models}.exit"
set -e
created_models_ok=0
grep -qx 'provider: "local"' "${created_models}.out" || created_models_ok=1
# The template's live endpoint is a host that does not answer here, so the
# request may fail; what this leg checks is that the sole entry was chosen,
# which a refusal would contradict.
if grep -qx 'outcome: "refused"' "${created_models}.out"; then created_models_ok=1; fi
report "${created_models}" "${created_models_ok}" "models over the created pool" "the sole entry local selected"
again_stem="${docs}/18-configure-unchanged"
set +e
( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project ) > "${again_stem}.out" 2> "${again_stem}.log"
printf '%s\n' "$?" > "${again_stem}.exit"
set -e
again_ok=0
[[ "$(cat "${again_stem}.exit")" -eq 0 ]] || again_ok=1
grep -qx 'outcome: "unchanged"' "${again_stem}.out" || again_ok=1
grep -qx 'written: \[\]' "${again_stem}.out" || again_ok=1
report "${again_stem}" "${again_ok}" "configure --create --project, second call" "both files already hold what it would write"
# 32. A file a user left in the layer is theirs: --create adds nothing to it
#     — the templates kbase writes today included — and reports every file
#     unwritten, so a file a user deleted stays deleted. The file here is
#     the harnesses.toml an older kbase seeded: it is read by nothing, and
#     the run neither rewrites nor removes it.
user_harnesses="${crepo}/.kbase/harnesses.toml"
printf '%s\n' '# mine' '' '[only-mine]' 'command = ["mine", "{prompt}"]' > "${user_harnesses}"
cp "${user_harnesses}" "${out}/harnesses-expected.toml"
hseed_stem="${docs}/18b-configure-existing-file"
set +e
( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project ) > "${hseed_stem}.out" 2> "${hseed_stem}.log"
printf '%s\n' "$?" > "${hseed_stem}.exit"
set -e
hseed_ok=0
[[ "$(cat "${hseed_stem}.exit")" -eq 0 ]] || hseed_ok=1
grep -qx 'outcome: "unchanged"' "${hseed_stem}.out" || hseed_ok=1
grep -qx 'written: \[\]' "${hseed_stem}.out" || hseed_ok=1
cmp -s "${user_harnesses}" "${out}/harnesses-expected.toml" || hseed_ok=1
report "${hseed_stem}" "${hseed_ok}" "configure --create --project over a leftover harnesses.toml" "every file unwritten and the leftover untouched"
hshow_stem="${docs}/18c-harnesses-leftover"
set +e
( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" show-config ) > "${hshow_stem}.out" 2> "${hshow_stem}.log"
printf '%s\n' "$?" > "${hshow_stem}.exit"
set -e
hshow_ok=0
[[ "$(cat "${hshow_stem}.exit")" -eq 0 ]] || hshow_ok=1
# The report lists the layer's two files and no third: a leftover
# harnesses.toml is a file nothing reads, and the report says nothing of it.
[[ "$(layer_field "${hshow_stem}" 1 providersExists)" == true ]] || hshow_ok=1
[[ "$(layer_field "${hshow_stem}" 1 configExists)" == true ]] || hshow_ok=1
if grep -q '^harnesses' "${hshow_stem}.out"; then hshow_ok=1; fi
report "${hshow_stem}" "${hshow_ok}" "show-config over a layer holding a leftover harnesses.toml" "two files reported, the leftover unread"
# 12. A --provider the pool does not hold refuses before anything is
#     written: the item names the target layer's providers.toml by its
#     absolute path, and offers the creation remedy. Both files still hold
#     exactly what the previous call left.
pool_before="$(cksum "${crepo}/.kbase/providers.toml" "${crepo}/.kbase/config.toml")"
unknown_stem="${docs}/19-configure-unknown-provider"
set +e
( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project \
    --provider nosuch --model-map decision=nosuch/d,text=nosuch/t,scoring=nosuch/s ) > "${unknown_stem}.out" 2> "${unknown_stem}.log"
printf '%s\n' "$?" > "${unknown_stem}.exit"
set -e
unknown_ok=0
[[ "$(cat "${unknown_stem}.exit")" -eq 1 ]] || unknown_ok=1
grep -qx 'outcome: "refused"' "${unknown_stem}.out" || unknown_ok=1
[[ "$(grep -c '^    - check:' "${unknown_stem}.out")" == 1 ]] || unknown_ok=1
[[ "$(item_field "${unknown_stem}.out" refusals check)" == provider ]] || unknown_ok=1
[[ "$(item_field "${unknown_stem}.out" refusals path)" == "${crepo}/.kbase/providers.toml" ]] || unknown_ok=1
[[ "$(item_field "${unknown_stem}.out" refusals key)" == --provider ]] || unknown_ok=1
[[ "$(item_field "${unknown_stem}.out" refusals remedy)" == "kbase configure --create" ]] || unknown_ok=1
grep -q 'baseUrl' "${unknown_stem}.out" || unknown_ok=1
[[ "$(cksum "${crepo}/.kbase/providers.toml" "${crepo}/.kbase/config.toml")" == "${pool_before}" ]] || unknown_ok=1
report "${unknown_stem}" "${unknown_ok}" "configure --create --project --provider nosuch" "refused against the absolute providers.toml, nothing written"
# 13. --config-dir and --project name different directories, so together
#     they are refused as usage, naming --project, and nothing is created.
conflict="${cdir}/conflict"
conflict_stem="${docs}/20-configure-conflict"
set +e
( cd "${crepo}/nested" && env "${scrub[@]}" "${kbase}" configure --create --project \
    --config-dir "${conflict}" ) > "${conflict_stem}.out" 2> "${conflict_stem}.log"
printf '%s\n' "$?" > "${conflict_stem}.exit"
set -e
conflict_ok=0
[[ "$(cat "${conflict_stem}.exit")" -eq 1 ]] || conflict_ok=1
grep -qx 'outcome: "refused"' "${conflict_stem}.out" || conflict_ok=1
[[ "$(item_field "${conflict_stem}.out" refusals check)" == usage ]] || conflict_ok=1
[[ "$(item_field "${conflict_stem}.out" refusals key)" == --project ]] || conflict_ok=1
[[ ! -e "${conflict}" ]] || conflict_ok=1
report "${conflict_stem}" "${conflict_ok}" "configure --create --project --config-dir" "refused naming --project, no directory created"
# 14. --config-dir into an empty directory creates both files there: the
#     directory read is the directory written.
flagdir="${cdir}/flagdir"
mkdir -p "${flagdir}"
flag_stem="${docs}/21-configure-config-dir"
set +e
( cd "${work}" && env "${scrub[@]}" "${kbase}" configure --create --config-dir "${flagdir}" ) > "${flag_stem}.out" 2> "${flag_stem}.log"
printf '%s\n' "$?" > "${flag_stem}.exit"
set -e
flag_ok=0
[[ "$(cat "${flag_stem}.exit")" -eq 0 ]] || flag_ok=1
grep -qx 'outcome: "done"' "${flag_stem}.out" || flag_ok=1
grep -qx "    - \"${flagdir}/providers.toml\"" "${flag_stem}.out" || flag_ok=1
grep -qx "    - \"${flagdir}/config.toml\"" "${flag_stem}.out" || flag_ok=1
[[ -f "${flagdir}/providers.toml" && -f "${flagdir}/config.toml" ]] || flag_ok=1
report "${flag_stem}" "${flag_ok}" "configure --create --config-dir into an empty directory" "both files created there"
# 15. --project outside a worktree refuses, naming the start directory and
#     the git init the tool does not run, and creates nothing. The run
#     directory is a mktemp tree, somewhere under no repository at all — the
#     project's own transient directory sits in one.
outside="$(mktemp -d)"
outside_stem="${docs}/22-configure-outside-worktree"
set +e
( cd "${outside}" && env "${scrub[@]}" "${kbase}" configure --create --project ) > "${outside_stem}.out" 2> "${outside_stem}.log"
printf '%s\n' "$?" > "${outside_stem}.exit"
set -e
outside_ok=0
[[ "$(cat "${outside_stem}.exit")" -eq 1 ]] || outside_ok=1
grep -qx 'outcome: "refused"' "${outside_stem}.out" || outside_ok=1
[[ "$(item_field "${outside_stem}.out" refusals check)" == worktree ]] || outside_ok=1
[[ "$(item_field "${outside_stem}.out" refusals path)" == "${outside}" ]] || outside_ok=1
[[ "$(item_field "${outside_stem}.out" refusals remedy)" == "git init" ]] || outside_ok=1
[[ ! -e "${outside}/.kbase" ]] || outside_ok=1
rm -rf "${outside}"
report "${outside_stem}" "${outside_ok}" "configure --create --project outside a worktree" "refused on worktree, nothing created"
# 16. A configure that writes references the resolver resolves, and its edit
#     contract over an old-grammar file: a config.toml naming bare ids is
#     rewritten in one call — the three references written, every other byte
#     identical — and show-config over the result reports every role resolved
#     with no diagnostic. The resolver refuses a bare id, so this is what says
#     the writer and the resolver agree.
refsdir="${cdir}/refsdir"
mkdir -p "${refsdir}"
printf '%s\n' '# kept note' '' '[models]' 'decision = "d3"   # the letter asks' 'text = "qwen"' \
    '' '[asks]' 'concurrency = 4' > "${refsdir}/config.toml"
printf '%s\n' '# kept note' '' '[models]' 'decision = "local/d3"  # the letter asks' 'text = "local/qwen"' \
    'scoring = "local/clef"' '' '[asks]' 'concurrency = 4' > "${out}/refs-expected.toml"
refs_stem="${docs}/23-configure-references"
set +e
( cd "${work}" && env "${scrub[@]}" "${kbase}" configure --create --config-dir "${refsdir}" \
    --model-map decision=local/d3,text=local/qwen,scoring=local/clef ) > "${refs_stem}.out" 2> "${refs_stem}.log"
printf '%s\n' "$?" > "${refs_stem}.exit"
set -e
refs_ok=0
[[ "$(cat "${refs_stem}.exit")" -eq 0 ]] || refs_ok=1
grep -qx 'outcome: "done"' "${refs_stem}.out" || refs_ok=1
grep -qxF '    decision: "local/d3"' "${refs_stem}.out" || refs_ok=1
grep -qxF '    text: "local/qwen"' "${refs_stem}.out" || refs_ok=1
grep -qxF '    scoring: "local/clef"' "${refs_stem}.out" || refs_ok=1
cmp -s "${refsdir}/config.toml" "${out}/refs-expected.toml" || refs_ok=1
report "${refs_stem}" "${refs_ok}" "configure --create rewriting an old-grammar file" "three references, every other byte identical"
refs_show="${docs}/24-configure-references-show"
set +e
( cd "${work}" && env "${scrub[@]}" "${kbase}" show-config --config-dir "${refsdir}" ) > "${refs_show}.out" 2> "${refs_show}.log"
printf '%s\n' "$?" > "${refs_show}.exit"
set -e
refs_show_ok=0
[[ "$(cat "${refs_show}.exit")" -eq 0 ]] || refs_show_ok=1
[[ "$(diagnostics "${refs_show}")" == 0 ]] || refs_show_ok=1
for role in decision text scoring; do
    [[ "$(role_field "${refs_show}.out" "${role}" provider)" == local ]] || refs_show_ok=1
done
[[ "$(role_field "${refs_show}.out" scoring model)" == clef ]] || refs_show_ok=1
report "${refs_show}" "${refs_show_ok}" "show-config over the references configure wrote" "every role resolved, no diagnostics"
# 17. sampling. Settings on an openai entry come back done with no diagnostic,
#     and the text role reports both values and the layer that supplied each;
#     a [sampling.decision] table holding a key fails the strict load, naming
#     the key; a temperature outside the anthropic range is a diagnostic, not
#     a stop.
sampdir="${out}/sampling"
mkdir -p "${sampdir}/openai" "${sampdir}/decision" "${sampdir}/anthropic"
printf '%s\n' '[oa]' 'baseUrl = "http://oa.example/v1"' 'api = "openai"' > "${sampdir}/openai/providers.toml"
printf '%s\n' '[models]' 'decision = "oa/d"' 'text = "oa/t"' 'scoring = "oa/s"' '' \
    '[sampling.text]' 'temperature = 0.3' 'thinking = "low"' > "${sampdir}/openai/config.toml"
cp "${sampdir}/openai/providers.toml" "${sampdir}/decision/providers.toml"
printf '%s\n' '[models]' 'decision = "oa/d"' 'text = "oa/t"' 'scoring = "oa/s"' '' \
    '[sampling.decision]' 'temperature = 0.5' > "${sampdir}/decision/config.toml"
printf '%s\n' '[msg]' 'baseUrl = "http://msg.example/v1"' 'api = "anthropic"' > "${sampdir}/anthropic/providers.toml"
printf '%s\n' '[models]' 'decision = "msg/d"' 'text = "msg/t"' 'scoring = "msg/s"' '' \
    '[sampling.text]' 'temperature = 1.5' > "${sampdir}/anthropic/config.toml"
samp_run() {
    local name="$1" stem="${docs}/$2"
    set +e
    ( cd "${work}" && env HOME="${home}" "${kbase}" show-config --config-dir "${sampdir}/${name}" ) > "${stem}.out" 2> "${stem}.log"
    printf '%s\n' "$?" > "${stem}.exit"
    set -e
}
samp_stem="${docs}/25-sampling-openai"
samp_run openai 25-sampling-openai
samp_ok=0
[[ "$(cat "${samp_stem}.exit")" -eq 0 ]] || samp_ok=1
grep -qx 'outcome: "done"' "${samp_stem}.out" || samp_ok=1
[[ "$(diagnostics "${samp_stem}")" == 0 ]] || samp_ok=1
[[ "$(role_field "${samp_stem}.out" text temperature)" == 0.3 ]] || samp_ok=1
[[ "$(role_field "${samp_stem}.out" text thinking)" == low ]] || samp_ok=1
[[ "$(role_source "${samp_stem}.out" text temperature)" == explicit ]] || samp_ok=1
[[ "$(role_source "${samp_stem}.out" text thinking)" == explicit ]] || samp_ok=1
[[ "$(role_field "${samp_stem}.out" scoring temperature)" == null ]] || samp_ok=1
report "${samp_stem}" "${samp_ok}" "show-config with sampling on an openai entry" "done, the text role reports both values and their layer"
samp_stem="${docs}/26-sampling-decision"
samp_run decision 26-sampling-decision
samp_ok=0
[[ "$(cat "${samp_stem}.exit")" -eq 1 ]] || samp_ok=1
grep -qx 'outcome: "refused"' "${samp_stem}.out" || samp_ok=1
[[ "$(item_field "${samp_stem}.out" refusals detail)" == *'unknown key \"sampling.decision.temperature\"'* ]] || samp_ok=1
report "${samp_stem}" "${samp_ok}" "show-config over a [sampling.decision] table" "refused by strict load, naming sampling.decision.temperature"
samp_stem="${docs}/27-sampling-anthropic-range"
samp_run anthropic 27-sampling-anthropic-range
samp_ok=0
[[ "$(cat "${samp_stem}.exit")" -eq 0 ]] || samp_ok=1
grep -qx 'outcome: "done"' "${samp_stem}.out" || samp_ok=1
[[ "$(diagnostics "${samp_stem}")" == 1 ]] || samp_ok=1
[[ "$(item_field "${samp_stem}.out" diagnostics key)" == sampling.text.temperature ]] || samp_ok=1
report "${samp_stem}" "${samp_ok}" "show-config with temperature 1.5 on an anthropic entry" "done, one diagnostic naming sampling.text.temperature"
# The same report over MCP, from a directory holding no KB: the tool binds
# the verb (I9), and a verb that resolves no KB answers there.
printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"config-recipe","version":"1"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"show-config","arguments":{}}}' \
    > "${out}/mcp.requests.jsonl"
set +e
( cd "${work}" && env HOME="${home}" KBASE_CONFIG_DIR="${dirs}/both" "${kbase}" mcp < "${out}/mcp.requests.jsonl" ) > "${out}/mcp.actual.jsonl" 2> "${out}/mcp.log"
printf '%s\n' "$?" > "${out}/mcp.exit"
set -e
call="$(awk 'index($0, "\"id\":2") { print; exit }' "${out}/mcp.actual.jsonl")"
mcp_ok=0
[[ -n "${call}" ]] || mcp_ok=1
[[ "${call}" == *'"isError":false'* ]] || mcp_ok=1
[[ "${call}" == *"\"path\":\"${dirs}/both\""* ]] || mcp_ok=1
[[ "${call}" == *'"source":"environment"'* ]] || mcp_ok=1
[[ "${call}" == *'"providersExists":true'* ]] || mcp_ok=1
[[ "${call}" == *'"configExists":true'* ]] || mcp_ok=1
[[ "$(grep -o '"check":"provider"' <<< "${call}" | wc -l | tr -d ' ')" == 2 ]] || mcp_ok=1
printf 'tools/call show-config\t-\t%s\t%s\n' "$([[ "${mcp_ok}" -eq 0 ]] && echo pass || echo fail)" "${out}/mcp.actual.jsonl" >> "${out}/config.tsv"
printf '  %s tools/call show-config over MCP: exit %s\n' "$([[ "${mcp_ok}" -eq 0 ]] && echo pass || echo fail)" "$(cat "${out}/mcp.exit")"
[[ "${mcp_ok}" -eq 0 ]] || status=1
OUT_DIR="${out}" RECIPE="test-integration-config" INVOCATION="three staged configurations under configs/: absent (no directory), pool (providers.toml only) and both (the same providers.toml plus config.toml); the pool holds an entry that drops for want of a baseUrl, and one supplying its key from key.txt seeded mode 0644. home/ is the staged home directory every invocation runs under, left empty so a leg naming a directory reads that one layer only\nper invocation NN-<state>-<route>: from work/, a directory holding no KB, env HOME=${CONFIG_OUT_DIR}/home KBASE_CONFIG_DIR=<dir> ${kbase} show-config (route environment) or ${kbase} show-config --config-dir <dir> (route flag), stdout to docs/NN-<state>-<route>.out\nconfig.tsv: per invocation the arguments, the source wanted, pass or fail, and its document; each checked for outcome done, exactly one layer naming the directory with its source and whether it and each of its two files stand, its diagnostic count, and that the key-file value appears nowhere on stdout; the both legs are checked further for the roles record: provider, model and the layer that supplied the model and the key\nlayered/: a staged git repository whose .kbase/providers.toml adds [local] and overrides the global [shared], and whose .kbase/config.toml overrides models.decision; layered/home/.config/kbase holds the global pool and config.toml. docs/08-layered.out is show-config run from layered/repo/nested with HOME=layered/home, checked for the merged pool in name order with each entry's layer, whole-entry replacement (shared's URL from the repository, key none, no key-file), both layers in precedence order with path, source, exists, providersExists and configExists, and each role's own provider, model, endpoint and the layer that supplied each field: repository for the decision role's entry and id, global for the text and scoring roles the repository left alone, whose entry names no key\nconfigure/: a staged git repository with an empty Home, and the configuration legs run under it — KBASE_CONFIG_DIR unset, HOME on configure/home. docs/15-configure-create-project.out is configure --create --project from configure/repo/nested, checked for outcome done, null provider, both repository paths listed written, the unchecked-ids line on stderr and a live [local] in the created providers.toml; docs/16-configure-created-layer.out is show-config over it, checked for two layers with the repository one first, source repository, exists/providersExists/configExists true, selection local by sole-entry, and the three diagnostics a blank [models] table leaves (models.decision, models.text, models.scoring); docs/17-configure-models.out is models --timeout 300ms over the created pool, checked for provider local and no refusal, so the sole entry is what selected; docs/18-configure-unchanged.out is the same configure again, checked for unchanged with an empty written list; docs/18b-configure-existing-file.out and docs/18c-harnesses-leftover.out put back the harnesses.toml an older kbase seeded and check that configure writes nothing over it and show-config reports the layer's two files and nothing of that one, a file nothing reads; docs/19-configure-unknown-provider.out is configure --create --project --provider nosuch, checked for one provider refusal naming the absolute configure/repo/.kbase/providers.toml, key --provider, remedy kbase configure --create, and both files unchanged by cksum; docs/20-configure-conflict.out is the same flags plus --config-dir, checked for one usage refusal keyed --project with no directory created; docs/21-configure-config-dir.out is configure --create --config-dir into an empty directory, checked for both files written there; docs/22-configure-outside-worktree.out is configure --create --project from a mktemp directory under no repository (removed after the run), checked for one worktree refusal naming that directory with remedy git init and no .kbase created\ndocs/23-configure-references.out is configure --create over an empty directory with --model-map decision=local/d3,text=local/qwen,scoring=local/clef, checked for three references written and no provider key; docs/24-configure-references-show.out is show-config over that directory, checked for no diagnostics and every role on the reference's entry and model\nsampling/: three configuration directories, each read by one show-config --config-dir <dir> from work/ with HOME on home/: docs/25-sampling-openai.out over an openai entry whose config.toml states [sampling.text] temperature 0.3 and thinking low, checked for exit 0, done, no diagnostics, the text role's temperature 0.3 and thinking low each sourced explicit, and the scoring role's temperature null; docs/26-sampling-decision.out over a config.toml holding [sampling.decision] temperature 0.5, checked for exit 1, refused, and a refusal naming unknown key sampling.decision.temperature; docs/27-sampling-anthropic-range.out over an anthropic entry whose [sampling.text] states temperature 1.5, checked for exit 0, done, and exactly one diagnostic keyed sampling.text.temperature\nmcp.requests.jsonl, mcp.actual.jsonl: a tools/call of show-config with no arguments to ${kbase} mcp from work/, checked for isError false, the both directory's path and source, both files present, and the same diagnostic count" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-config): a config check failed — see config.tsv\n'
    exit 1
fi
printf 'integration(test-integration-config) ok\n'
