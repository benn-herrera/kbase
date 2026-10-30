#!/usr/bin/env bash
# The served MCP surface's size over the delivered binary: `./bin/kbase mcp`
# started in a system temporary directory under no repository, so it locates
# no project and no count varies with a path, and sent initialize, tools/list
# and resources/list. TestMeasureMCPSurface measures the frames it answered
# into surface.yaml. A measurement, not a gate: it fails only where the server
# exits non-zero, a request goes unanswered or a count is not positive.
#
# Inputs (environment): BIN_DIR; MEASURE_MCP_SURFACE_OUT_DIR, the evidence
# directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${MEASURE_MCP_SURFACE_OUT_DIR:?MEASURE_MCP_SURFACE_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
out="${root}/${MEASURE_MCP_SURFACE_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
rm -rf "${out}/run" "${out}/surface.yaml" "${out}/measure.log"
# The run tree lives under no repository while the server runs, and is copied
# to run/ before the temporary directory goes.
run="$(mktemp -d)"
printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"measure-mcp-surface","version":"1"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
    '{"jsonrpc":"2.0","id":3,"method":"resources/list"}' > "${run}/requests.jsonl"
set +e
( cd "${run}" && "${kbase}" mcp ) < "${run}/requests.jsonl" > "${run}/frames.jsonl" 2> "${run}/mcp.log"
printf '%s\n' "$?" > "${run}/mcp.exit"
set -e
mkdir -p "${out}/run"
cp -R "${run}/." "${out}/run/"
rm -rf "${run}"
status=0
if [[ "$(cat "${out}/run/mcp.exit")" -ne 0 ]]; then
    printf 'measure(measure-mcp-surface): kbase mcp exited %s — see run/mcp.log\n' "$(cat "${out}/run/mcp.exit")"
    status=1
fi
go test -count=1 -run '^TestMeasureMCPSurface$' ./internal/mcp/protocol -args -surface.frames="${out}/run/frames.jsonl" \
    -surface.out="${out}/surface.yaml" > "${out}/measure.log" 2>&1 || {
    printf 'measure(measure-mcp-surface): the frames did not measure — see measure.log\n'
    status=1
}
OUT_DIR="${out}" RECIPE="measure-mcp-surface" INVOCATION="run/: a mktemp directory under no repository while the server ran, copied here after\nrun/: ${kbase} mcp < run/requests.jsonl (initialize, notifications/initialized, tools/list, resources/list) > run/frames.jsonl 2> run/mcp.log; its exit in run/mcp.exit\ngo test -run '^TestMeasureMCPSurface\$' ./internal/mcp/protocol -args -surface.frames=run/frames.jsonl -surface.out=surface.yaml > measure.log\nsurface.yaml: tools, tools-list-chars (the tools/list result's JSON length), input-schema-chars (every inputSchema's JSON length, summed), instructions-chars" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    exit 1
fi
cat "${out}/surface.yaml"
printf 'measure(measure-mcp-surface) ok\n'
