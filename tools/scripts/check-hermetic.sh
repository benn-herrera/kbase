#!/usr/bin/env bash
# Fails each tools/scripts/test-integration-*.sh that runs the binary with a
# configuration-loading verb and sets XDG_STATE_HOME by neither `export` nor
# `env`, unless its justfile recipe's [doc] begins LIVE. Such a script reads
# the credentials store under the user's real $XDG_STATE_HOME.
#
# Textual, over non-comment lines. A verb run is `kbase <verb>` (any quoting of
# the binary path ending in kbase) or a `run <verb> ...` helper call; an
# XDG_STATE_HOME set is an `export` or `env` line naming it. A verb inside a
# quoted string, an invocation through a helper other than `run`, and an export
# continued onto a second line are not seen.
#
# Inputs: none; runs from the repository root.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

verbs='(build|score|probe|show-config|configure|models|auth)'
runs_verb="(kbase\"?[[:space:]]+${verbs}([[:space:]]|\$)|^[[:space:]]*run[[:space:]]+${verbs}[[:space:]])"
sets_xdg='^[[:space:]]*(export|env)[[:space:]].*XDG_STATE_HOME=|[[:space:]]env[[:space:]].*XDG_STATE_HOME='

# is_live <recipe>: the recipe's [doc] begins LIVE.
function is_live() {
  awk -v n="${1}" '
    /^\[doc\("/ { d = $0; next }
    /^\[/ || /^#/ { next }
    $0 ~ "^" n "[ :]" { found = 1; exit (d ~ /^\[doc\("LIVE/) ? 0 : 1 }
    { d = "" }
    END { if (!found) exit 2 }
  ' justfile
}

status=0
for f in tools/scripts/test-integration-*.sh; do
  body="$(grep -v '^[[:space:]]*#' "${f}")"
  grep -Eq "${runs_verb}" <<< "${body}" || continue
  ! grep -Eq "${sets_xdg}" <<< "${body}" || continue
  rc=0
  is_live "$(basename "${f}" .sh)" || rc=$?
  [[ "${rc}" -ne 0 ]] || continue
  [[ "${rc}" -ne 2 ]] || { printf 'check-hermetic: %s: no justfile recipe %s\n' "${f}" "$(basename "${f}" .sh)" >&2; status=1; continue; }
  printf 'check-hermetic: %s runs a configuration-loading verb and sets XDG_STATE_HOME by neither export nor env\n' "${f}" >&2
  status=1
done
exit "${status}"
