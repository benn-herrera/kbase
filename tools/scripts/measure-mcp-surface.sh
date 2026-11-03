#!/usr/bin/env bash
# The served MCP surface's size over the release build, the delivered shape
# (`just build-release`): `./bin/release/kbase mcp` started in a system temporary directory under no repository, so it locates
# no project and no count varies with a path, and sent initialize, tools/list
# and resources/list. TestMeasureMCPSurface measures the frames it answered
# into surface.yaml. A measurement, not a gate: it fails only where the server
# exits non-zero, a request goes unanswered or a count is not positive.
#
# run/served/ holds the served text in full, for the surface audit, compared
# with nothing: every listed resource read back (resources.jsonl), tools/list
# where every call names its project (required.jsonl), the instructions served
# in a repository with no kb-root/ yet (building.jsonl), the located mode over a
# KB this binary's init made, with every kbase://sheet/ resource it lists read
# back (located.jsonl), and every command's --help, the root's first, walked
# through each Commands list (cli-help/kbase[.<command>...].txt); cobra's
# completion and help are not walked. It also fails where a capture's server, or
# the located KB's init, exits non-zero or a --help render fails.
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
served="${run}/served"
mkdir -p "${served}/cli-help" "${served}/building/.git/objects" "${served}/building/.git/refs/heads"
printf 'ref: refs/heads/main\n' > "${served}/building/.git/HEAD"
{
    sed -n '1,2p' "${run}/requests.jsonl"
    id=1
    while IFS= read -r uri; do
        id=$((id + 1))
        printf '{"jsonrpc":"2.0","id":%d,"method":"resources/read","params":{"uri":"%s"}}\n' "${id}" "${uri}"
    done < <(grep -o '"uri":"kbase://[^"]*"' "${run}/frames.jsonl" | cut -d'"' -f4)
} > "${served}/resources.requests.jsonl"
sed -n '1,3p' "${run}/requests.jsonl" > "${served}/required.requests.jsonl"
sed -n '1,2p' "${run}/requests.jsonl" > "${served}/building.requests.jsonl"
# The located-with-KB capture: a fresh repository under the run tree holding
# the KB this binary's init made; the identity and global git config are fixed
# so init's birth commit depends on neither.
mkdir -p "${served}/located"
sed -n '1,2p' "${run}/requests.jsonl" > "${served}/located.list.requests.jsonl"
printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"resources/list"}' >> "${served}/located.list.requests.jsonl"
set +e
(
    export GIT_AUTHOR_NAME=measure-mcp-surface GIT_AUTHOR_EMAIL=measure@invalid GIT_COMMITTER_NAME=measure-mcp-surface GIT_COMMITTER_EMAIL=measure@invalid GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${run}/gitconfig"
    cd "${served}/located" && git init --quiet && "${kbase}" init > "${served}/located.init.out" 2> "${served}/located.init.log"
)
printf '%s\n' "$?" > "${served}/located.init.exit"
( cd "${served}/located" && "${kbase}" mcp ) < "${served}/located.list.requests.jsonl" > "${served}/located.list.jsonl" 2> "${served}/located.list.log"
printf '%s\n' "$?" > "${served}/located.list.exit"
set -e
{
    cat "${served}/located.list.requests.jsonl"
    id=2
    while IFS= read -r uri; do
        id=$((id + 1))
        printf '{"jsonrpc":"2.0","id":%d,"method":"resources/read","params":{"uri":"%s"}}\n' "${id}" "${uri}"
    done < <(grep -o '"uri":"kbase://sheet/[^"]*"' "${served}/located.list.jsonl" | cut -d'"' -f4)
} > "${served}/located.requests.jsonl"
# help_walk [<command>...]: `kbase [<command>...] --help` into
# cli-help/kbase[.<command>...].txt, then the same for each entry of its
# Commands list; a failed render is named in cli-help/failed.txt.
function help_walk() {
    local name="kbase" w sub
    for w in ${@+"$@"}; do name="${name}.${w}"; done
    ( cd "${run}" && "${kbase}" ${@+"$@"} --help ) < /dev/null > "${served}/cli-help/${name}.txt" 2> "${served}/cli-help/${name}.log" \
        || printf '%s\n' "${name}" >> "${served}/cli-help/failed.txt"
    while IFS= read -r sub; do
        help_walk ${@+"$@"} "${sub}"
    done < <(awk '/^Commands:/ { f = 1; next } f && /^$/ { exit } f && $1 != "completion" && $1 != "help" { print $1 }' "${served}/cli-help/${name}.txt")
}
set +e
( cd "${run}" && "${kbase}" mcp ) < "${served}/resources.requests.jsonl" > "${served}/resources.jsonl" 2> "${served}/resources.log"
printf '%s\n' "$?" > "${served}/resources.exit"
( cd "${run}" && "${kbase}" mcp --project-dir-required ) < "${served}/required.requests.jsonl" > "${served}/required.jsonl" 2> "${served}/required.log"
printf '%s\n' "$?" > "${served}/required.exit"
( cd "${served}/building" && "${kbase}" mcp ) < "${served}/building.requests.jsonl" > "${served}/building.jsonl" 2> "${served}/building.log"
printf '%s\n' "$?" > "${served}/building.exit"
( cd "${served}/located" && "${kbase}" mcp ) < "${served}/located.requests.jsonl" > "${served}/located.jsonl" 2> "${served}/located.log"
printf '%s\n' "$?" > "${served}/located.exit"
set -e
help_walk
mkdir -p "${out}/run"
cp -R "${run}/." "${out}/run/"
rm -rf "${run}"
status=0
if [[ "$(cat "${out}/run/mcp.exit")" -ne 0 ]]; then
    printf 'measure(measure-mcp-surface): kbase mcp exited %s — see run/mcp.log\n' "$(cat "${out}/run/mcp.exit")"
    status=1
fi
if ! grep -q 'kbase://sheet/' "${out}/run/served/located.list.jsonl"; then
    printf 'measure(measure-mcp-surface): the located server listed no kbase://sheet/ resource — see run/served/located.list.jsonl\n'
    status=1
fi
for s in resources required building located.init located.list located; do
    if [[ "$(cat "${out}/run/served/${s}.exit")" -ne 0 ]]; then
        printf 'measure(measure-mcp-surface): the %s capture exited %s — see run/served/%s.log\n' "${s}" "$(cat "${out}/run/served/${s}.exit")" "${s}"
        status=1
    fi
done
if [[ -e "${out}/run/served/cli-help/failed.txt" ]]; then
    printf 'measure(measure-mcp-surface): --help failed for each command in run/served/cli-help/failed.txt\n'
    status=1
fi
go test -count=1 -run '^TestMeasureMCPSurface$' ./internal/mcp/protocol -args -surface.frames="${out}/run/frames.jsonl" \
    -surface.out="${out}/surface.yaml" > "${out}/measure.log" 2>&1 || {
    printf 'measure(measure-mcp-surface): the frames did not measure — see measure.log\n'
    status=1
}
OUT_DIR="${out}" RECIPE="measure-mcp-surface" INVOCATION="run/: a mktemp directory under no repository while the server ran, copied here after\nrun/: ${kbase} mcp < run/requests.jsonl (initialize, notifications/initialized, tools/list, resources/list) > run/frames.jsonl 2> run/mcp.log; its exit in run/mcp.exit\ngo test -run '^TestMeasureMCPSurface\$' ./internal/mcp/protocol -args -surface.frames=run/frames.jsonl -surface.out=surface.yaml > measure.log\nrun/served/: resources.jsonl, every resources/list URI read back; required.jsonl, ${kbase} mcp --project-dir-required sent initialize and tools/list; building.jsonl, initialize from building/, a bare .git with no kb-root/; located.jsonl, ${kbase} mcp from located/, a fresh git repository holding the KB ${kbase} init made, sent initialize, resources/list and a resources/read of each kbase://sheet/ URI the list names (located.list.jsonl); each capture's exit in <capture>.exit, and init's in located.init.exit and the located list's in located.list.exit; cli-help/kbase[.<command>...].txt, ${kbase} [<command>...] --help for the root and each Commands entry, recursively\nsurface.yaml: tools, tools-list-chars (the tools/list result's JSON length), input-schema-chars (every inputSchema's JSON length, summed), instructions-chars" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    exit 1
fi
cat "${out}/surface.yaml"
printf 'measure(measure-mcp-surface) ok\n'
printf 'full run data: %s\n' "${out}"
