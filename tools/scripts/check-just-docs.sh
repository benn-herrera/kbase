#!/usr/bin/env bash
# Fails, naming the recipe, each public recipe in justfile (no leading `_`)
# whose doc breaks the machine-checkable half of the doc standard:
#   - it has no [doc("...")] attribute;
#   - its doc is over 200 characters between the quotes;
#   - its doc carries LIVE as any word but the first;
#   - a doc's first word that is a marker (`LIVE`, `NEEDS`, `DESTRUCTIVE`,
#     `hermetic`) spelled in another case, a doc holding a non-ASCII byte;
#   - a recipe that is not test-* carries "(excluded from test-integration)";
#   - a recipe the omnibus (`test-integration:`) names has a doc not opening
#     with `hermetic`.
# A first word ends at a space, colon, semicolon or comma. Textual, over
# justfile alone: only justfile's own recipes are read, not imported ones.
# The length is counted in bytes, which a non-ASCII doc cannot reach.
#
# Inputs: none; runs from the repository root.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

LC_ALL=C awk '
  BEGIN {
    marker["live"] = "LIVE"; marker["needs"] = "NEEDS"
    marker["destructive"] = "DESTRUCTIVE"; marker["hermetic"] = "hermetic"
  }
  function fail(name, why) {
    printf "check-just-docs: recipe %s: %s\n", name, why > "/dev/stderr"
    bad = 1
  }
  function check(name, doc,    n, words, i, first, lower) {
    if (doc ~ /[\200-\377]/) fail(name, "doc holds a non-ASCII byte")
    n = split(doc, words, /[^A-Za-z0-9_]+/)
    if (words[1] == "") { first = words[2]; i = 2 } else { first = words[1]; i = 1 }
    if (length(doc) > 200) fail(name, "doc is " length(doc) " characters, over 200")
    for (i = i + 1; i <= n; i++)
      if (words[i] == "LIVE") fail(name, "LIVE is not the first word")
    lower = tolower(first)
    if (lower in marker && first != marker[lower])
      fail(name, "marker \"" first "\" is not spelled " marker[lower])
    if (name !~ /^test-/ && index(doc, "(excluded from test-integration)"))
      fail(name, "doc carries \"(excluded from test-integration)\" on a recipe that is not test-*")
    docfirst[name] = first
  }
  # Pass 1 collects recipes in order; the omnibus line is read in the same pass.
  /^\[doc\("/ {
    doc = $0
    sub(/^\[doc\("/, "", doc)
    sub(/"\)\]$/, "", doc)
    have = 1
    next
  }
  /^\[/ || /^#/ || /^[[:space:]]/ || /^$/ { next }
  /^[A-Za-z_][A-Za-z0-9_-]*[[:space:]]*:=/ { next }
  /^(set|alias|export|import\??|mod)[[:space:]]/ { next }
  /^[A-Za-z_][A-Za-z0-9_-]*[ :]/ && /:/ {
    name = $0
    sub(/[ :].*/, "", name)
    if (name == "test-integration") {
      line = $0
      sub(/^[^:]*:/, "", line)
      m = split(line, omni, /[[:space:]]+/)
    }
    if (name !~ /^_/) {
      names[++count] = name
      if (have) { docs[name] = doc; hasdoc[name] = 1 }
    }
    have = 0
    next
  }
  END {
    for (i = 1; i <= count; i++) {
      name = names[i]
      if (!hasdoc[name]) { fail(name, "has no [doc(\"...\")]"); continue }
      check(name, docs[name])
    }
    for (j = 1; j <= m; j++)
      if (omni[j] != "" && hasdoc[omni[j]] && docfirst[omni[j]] != "hermetic")
        fail(omni[j], "is run by the omnibus test-integration, so its doc must open with hermetic")
    exit bad
  }
' justfile
