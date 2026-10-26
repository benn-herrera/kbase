#!/usr/bin/env bash
# Renders readme-assets/strength-walk.dot to SVG_OUT with Graphviz dot.
#
# readme-assets/strength-walk.svg is the rendering of STRENGTH_SCORING_SCHEME.md's
# graph, and readme-assets/strength-walk.dot is its source of record: the file a
# reader reviews and edits. dot lays the graph out itself, so the image is a
# function of the .dot AND the Graphviz version that rendered it — an upgrade
# re-renders different bytes from unchanged source. The SVG 1.1 DOCTYPE's two
# lines are dropped, so the checked-in image names no resource a viewer would
# fetch.
#
# Inputs (environment): SVG_OUT, the file to write.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${SVG_OUT:?SVG_OUT is not set: the file to write}"

command -v dot > /dev/null || { echo "error: no Graphviz dot on PATH — install graphviz, then re-run." >&2; exit 1; }
dot -Tsvg readme-assets/strength-walk.dot | sed '/^<!DOCTYPE svg/,/svg11\.dtd">$/d' > "${SVG_OUT}"
