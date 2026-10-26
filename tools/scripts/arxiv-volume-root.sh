#!/usr/bin/env bash
# The one statement of how a paper's volume root is found, read by every
# integration script that builds a paper: 00README.json's single "toplevel"
# entry, else the sole top-level .tex file containing \documentclass. Prints
# the root relative to ARXIV_VOLUME_DIR; anything else is an error naming what
# was found.
#
# Inputs (environment): ARXIV_VOLUME_DIR, the paper's source tree.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${ARXIV_VOLUME_DIR:?ARXIV_VOLUME_DIR is not set: the source tree of the paper}"

dir="${ARXIV_VOLUME_DIR}"
readme="${dir}/00README.json"
if [[ -f "${readme}" ]]; then
    toplevel="$(python3 -c 'import json, sys; data = json.load(open(sys.argv[1])); names = [s.get("filename", "") for s in data.get("sources", []) if s.get("usage") == "toplevel"]; print(names[0] if len(names) == 1 and names[0] else "")' "${readme}" 2>/dev/null || true)"
    if [[ -n "${toplevel}" && -f "${dir}/${toplevel}" ]]; then
        printf '%s\n' "${toplevel}"
        exit 0
    fi
fi
candidates=()
shopt -s nullglob
for f in "${dir}"/*.tex; do
    if grep -qF '\documentclass' "${f}"; then
        candidates+=("$(basename "${f}")")
    fi
done
if [[ "${#candidates[@]}" -ne 1 ]]; then
    printf 'error: %s: 00README.json names no single toplevel source, and %s top-level .tex file(s) contain \\documentclass\n' "${dir}" "${#candidates[@]}" >&2
    exit 2
fi
printf '%s\n' "${candidates[0]}"
