#!/usr/bin/env bash
# The credentials store over the delivered binary, with HOME, XDG_STATE_HOME and
# the configuration directory all under the evidence directory: auth add by
# pipe and auth list naming the provider; show-config reporting key: store; an
# empty pipe refused with the store unchanged; an entry with both a stored key
# and an apiKeyFile reporting key: store and the unread-apiKeyFile warning; an
# entry with an apiKeyFile alone reporting its upgrade, and that command run as
# printed making it report key: store; a store file this script truncates
# reported as a diagnostic by its path, with show-config still done; and auth
# remove, after which each entry reports key: file or none as it states. Every
# stream and file is then searched for the stored key's text, which none may
# hold, and for the key files' text, which only the key files may.
# No pandoc, no provider and no corpus.
#
# Inputs (environment): BIN_DIR; AUTH_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${AUTH_OUT_DIR:?AUTH_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
out="${root}/${AUTH_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
config="${out}/config"
docs="${out}/docs"
work="${out}/work"
# Every kbase call runs under these: the store is xdg/kbase/auth, the global
# layer an empty home, and no leg reads or writes the operator's own.
export HOME="${out}/home" XDG_STATE_HOME="${out}/xdg"
unset KBASE_CONFIG_DIR
store="${XDG_STATE_HOME}/kbase/auth"
rm -rf "${config}" "${docs}" "${work}" "${HOME}" "${XDG_STATE_HOME}" "${out}/auth.tsv"
mkdir -p "${config}/my keys" "${docs}" "${work}" "${HOME}"

# The key piped into the store is never written to a file by this script; the
# key files' key is, in both.key and my keys/filed.key alone.
store_key='sk-store-MUST-NOT-APPEAR'
file_key='sk-file-MUST-NOT-APPEAR'
printf '%s\n' \
    '[stored]' 'baseUrl = "http://stored.example/v1"' \
    '' '[both]' 'baseUrl = "http://both.example/v1"' 'apiKeyFile = "both.key"' \
    '' '[filed]' 'baseUrl = "http://filed.example/v1"' 'apiKeyFile = "my keys/filed.key"' \
    > "${config}/providers.toml"
# Every role resolves, so a document's diagnostics are the store's and the key
# files' alone.
printf '%s\n' '[models]' 'decision = "stored/d"' 'text = "both/t"' 'scoring = "filed/s"' > "${config}/config.toml"
for f in "${config}/both.key" "${config}/my keys/filed.key"; do
    printf '%s\n' "${file_key}" > "${f}"
    chmod 0600 "${f}"
done

status=0
n=0
# run <name> <stdin-text> <args...>: one kbase call from work/ with
# --config-dir on the staged directory, stdin-text piped in, stdout, stderr and
# exit code to docs/NN-<name>.{out,log,exit}; DOC names the stem.
run() {
    local name="$1" input="$2"
    shift 2
    n=$((n + 1))
    DOC="${docs}/$(printf '%02d' "${n}")-${name}"
    set +e
    ( cd "${work}" && printf '%s' "${input}" | "${kbase}" "$@" --config-dir "${config}" ) > "${DOC}.out" 2> "${DOC}.log"
    printf '%s\n' "$?" > "${DOC}.exit"
    set -e
}
# check <ok> <label>: one leg's verdict — its auth.tsv line, its console line
# and the run's status.
check() {
    local verdict=pass
    [[ "$1" -eq 0 ]] || { verdict=fail; status=1; }
    printf '%s\t%s\t%s\n' "$2" "${verdict}" "${DOC}.out" >> "${out}/auth.tsv"
    printf '  %s %s: exit %s\n' "${verdict}" "$2" "$(cat "${DOC}.exit")"
}
# done_doc: the last call exited 0 with outcome done.
done_doc() { [[ "$(cat "${DOC}.exit")" -eq 0 ]] && grep -qx 'outcome: "done"' "${DOC}.out"; }
# provider_field <name> <key>: one scalar of the last document's providers
# entry called name.
provider_field() { awk -v want="$1" -v k="$2" '/^providers:/ { on = 1; next } on && /^    - name: / { cur = $0; sub(/^[^:]*: /, "", cur); gsub(/^"|"$/, "", cur); next } on && /^    / { if (cur == want && index($0, k ": ") == 7) { sub(/^[^:]*: /, "", $0); gsub(/^"|"$/, "", $0); print; exit }; next } on { exit }' "${DOC}.out"; }
# items <list>: each item of the last document's named list on one line, its
# fields joined by tabs.
items() { awk -v list="$1" '$0 == list ":" { on = 1; next } on && /^    - / { if (rec != "") print rec; rec = substr($0, 7); next } on && /^      / { rec = rec "\t" substr($0, 7); next } on { exit } END { if (rec != "") print rec }' "${DOC}.out"; }
# store_sums: the store's files with their checksums, in name order.
store_sums() { ( cd "${store}" && cksum -- * ); }

# 1. auth add by pipe, and auth list naming the provider.
run add-stored "${store_key}" auth add stored
ok=0; done_doc || ok=1
check "${ok}" "auth add stored, the key piped"
run list "" auth list
ok=0; done_doc || ok=1
grep -qx '    - "stored"' "${DOC}.out" || ok=1
check "${ok}" "auth list names stored"

# 2. show-config reports the stored key by its kind.
run show-stored "" show-config
ok=0; done_doc || ok=1
[[ "$(provider_field stored key)" == store ]] || ok=1
check "${ok}" "show-config reports stored's key: store"

# 3. an empty pipe is refused, and the store is unchanged.
before="$(store_sums)"
run add-empty "" auth add stored
ok=0
[[ "$(cat "${DOC}.exit")" -ne 0 ]] && grep -qx 'outcome: "refused"' "${DOC}.out" || ok=1
[[ "$(store_sums)" == "${before}" ]] || ok=1
check "${ok}" "auth add stored over an empty pipe refused, the store unchanged by cksum"

# 4. a stored key over a stated apiKeyFile: key store, and the warning that the
#    file is not read, its path the providers.toml stating it; and 5. a key
#    file alone: key file and its upgrade.
run add-both "${store_key}" auth add both
ok=0; done_doc || ok=1
check "${ok}" "auth add both, the key piped"
run show-both "" show-config
ok=0; done_doc || ok=1
[[ "$(provider_field both key)" == store ]] || ok=1
items diagnostics | grep -F "path: \"${config}/providers.toml\"" | grep -F 'provider \"both\"' | grep -qF 'kbase auth remove both' || ok=1
check "${ok}" "show-config reports both's key: store and its unread apiKeyFile"
upgrade="$(provider_field filed upgrade)"
ok=0
[[ "$(provider_field filed key)" == file ]] || ok=1
[[ "${upgrade}" == "cat '${config}/my keys/filed.key' | kbase auth add filed" ]] || ok=1
check "${ok}" "show-config reports filed's key: file and its upgrade, the path quoted"

# 5. the upgrade run as printed, kbase on PATH and the configuration directory
#    named by the variable the printed command does not override.
n=$((n + 1))
DOC="${docs}/$(printf '%02d' "${n}")-upgrade"
printf '%s\n' "${upgrade}" > "${DOC}.command"
set +e
( cd "${work}" && PATH="${root}/${BIN_DIR}:${PATH}" KBASE_CONFIG_DIR="${config}" bash -c "${upgrade}" ) > "${DOC}.out" 2> "${DOC}.log"
printf '%s\n' "$?" > "${DOC}.exit"
set -e
ok=0; done_doc || ok=1
check "${ok}" "the upgrade command, run as printed"
run show-upgraded "" show-config
ok=0; done_doc || ok=1
[[ "$(provider_field filed key)" == store ]] || ok=1
check "${ok}" "show-config reports filed's key: store after the upgrade"

# 6. a store file truncated: a diagnostic by its path whose remedy adds the key
#    again, show-config still done, and stored falling through to no key.
printf 'x' > "${store}/stored-key.enc"
run show-truncated "" show-config
ok=0; done_doc || ok=1
items diagnostics | grep -F "path: \"${store}/stored-key.enc\"" | grep -qF 'remedy: "<command-that-prints-the-key> | kbase auth add stored"' || ok=1
[[ "$(provider_field stored key)" == none ]] || ok=1
check "${ok}" "show-config over a truncated store file: done, the file a diagnostic with the piped kbase auth add stored"

# 7. auth remove: each entry then reports the key its entry states.
run remove-both "" auth remove both
ok=0; done_doc || ok=1
check "${ok}" "auth remove both"
run remove-stored "" auth remove stored
ok=0; done_doc || ok=1
check "${ok}" "auth remove stored"
run show-removed "" show-config
ok=0; done_doc || ok=1
[[ "$(provider_field both key)" == file ]] || ok=1
[[ "$(provider_field stored key)" == none ]] || ok=1
# What remains is filed's: its key is stored, and its apiKeyFile goes unread.
[[ "$(items diagnostics | wc -l | tr -d ' ')" == 1 ]] || ok=1
items diagnostics | grep -qF 'kbase auth remove filed' || ok=1
check "${ok}" "show-config after remove: both key: file, stored key: none, filed's unread apiKeyFile the one diagnostic"

# No stream and no file holds the stored key's text — the store's files hold
# it sealed — and only the two key files hold theirs.
DOC="${out}/key-search"
set +e
grep -rlF -- "${store_key}" "${out}" > "${DOC}.store.txt"
grep -rlF -- "${file_key}" "${out}" | sort > "${DOC}.file.txt"
set -e
printf '0\n' > "${DOC}.exit"
: > "${DOC}.out"
ok=0
[[ ! -s "${DOC}.store.txt" ]] || ok=1
[[ "$(cat "${DOC}.file.txt")" == "$(printf '%s\n' "${config}/both.key" "${config}/my keys/filed.key" | sort)" ]] || ok=1
check "${ok}" "the stored key in no stream or file, the key files' key in those files alone"

OUT_DIR="${out}" RECIPE="test-integration-auth" INVOCATION="config/: providers.toml with [stored] (no key field), [both] (apiKeyFile both.key) and [filed] (apiKeyFile my keys/filed.key), each key file holding the file key, mode 0600, and config.toml naming stored, both and filed for the three roles; HOME=home/ (empty) and XDG_STATE_HOME=xdg/ for every call, so the store is xdg/kbase/auth/; KBASE_CONFIG_DIR unset\nper invocation NN-<name>, from work/: ${kbase} <args> --config-dir config, a key piped where the leg adds one (never written to a file), stdout, stderr and exit to docs/NN-<name>.{out,log,exit}: auth add stored, auth list, show-config (stored key: store), auth add stored over an empty pipe (refused, the store's cksum unchanged), auth add both, show-config (both key: store with the unread-apiKeyFile diagnostic against config/providers.toml; filed key: file with its upgrade), the upgrade as printed (docs/NN-upgrade.command) run by bash -c with bin/ on PATH and KBASE_CONFIG_DIR=config, show-config (filed key: store), show-config after xdg/kbase/auth/stored-key.enc is overwritten with one byte (done, a diagnostic at that path with remedy <command-that-prints-the-key> | kbase auth add stored, stored key: none), auth remove both, auth remove stored, show-config (both key: file, stored key: none, filed's unread apiKeyFile the one diagnostic)\nkey-search.store.txt and key-search.file.txt: every file under this directory holding the stored key's text (none) and the file key's text (the two key files alone)\nauth.tsv: per check its label, pass or fail, and its document" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-auth): an auth check failed — see auth.tsv\n'
    exit 1
fi
printf 'integration(test-integration-auth) ok\n'
