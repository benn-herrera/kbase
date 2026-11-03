#!/usr/bin/env bash
# Prints the id of each entry of a write result document, entries[].id, one per
# line in entry order: those opening with the given prefix, every id where the
# prefix is empty. The result's layout — entries at the top level, each entry a
# list item whose keys stand one level under it — is assumed here and in no
# other script.
#
# Usage: entry-ids.sh <result file> <id prefix, or "">
set -euo pipefail

file="${1:?usage: entry-ids.sh <result file> <id prefix, or \"\">}"
prefix="${2?usage: entry-ids.sh <result file> <id prefix, or \"\">}"
awk -v prefix="${prefix}" '
  function flush() {
    if (id != "" && (prefix == "" || index(id, prefix) == 1)) print id
    id = ""
  }
  /^[^ ]/ { if (inside) flush(); inside = ($0 == "entries:"); next }
  !inside { next }
  /^    - / { flush(); key = substr($0, 7) }
  /^      [^ -]/ { key = substr($0, 7) }
  /^    - / || /^      [^ -]/ {
    if (key ~ /^id: "/) { id = key; sub(/^id: "/, "", id); sub(/"$/, "", id) }
  }
  END { if (inside) flush() }
' "${file}"
