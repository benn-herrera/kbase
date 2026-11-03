#!/usr/bin/env bash
# The build leg over the delivered binary: a build launched through the tool,
# followed to its end by polling status, the server announcing nothing. A Go
# test drives a real server — this script's child, because only
# internal/mcp/launch.go starts a process inside the module, and a script
# starts the server outside it — over two named pipes the script holds, and starts no
# process of its own. That leg builds a paper with no inference, so it needs
# pandoc.
#
# Inputs (environment): BIN_DIR; MCP_TRANSCRIPT_DIR, the transcripts and the
# paper; MCP_BUILD_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${MCP_TRANSCRIPT_DIR:?MCP_TRANSCRIPT_DIR is not set: the transcript fixtures}"
: "${MCP_BUILD_OUT_DIR:?MCP_BUILD_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
out="${root}/${MCP_BUILD_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
# The test drives the protocol and must start no process, so the server is
# this script's child, its requests and its frames two named pipes the test
# opens. The pipes are opened in the order that cannot deadlock — the test's
# write end unblocks the server's `< requests`, which then opens `> frames`
# for the test's read — and a build that never ends is caught by the test's
# own deadline and the test runner's -timeout.
rm -rf "${out}/repo" "${out}/state" "${out}/xdg" "${out}/config" "${out}/home" \
    "${out}/requests.fifo" "${out}/frames.fifo" "${out}/mcp.log" "${out}/test.log"
mkdir -p "${out}/repo" "${out}/config" "${out}/home"
cp "${MCP_TRANSCRIPT_DIR}/paper.tex" "${out}/repo/"
: > "${out}/home/.gitconfig"
git -C "${out}/repo" init -q
git -C "${out}/repo" add -A
git -C "${out}/repo" -c user.name=mcp -c user.email=mcp@invalid commit -qm "sources as fetched"
mkfifo "${out}/requests.fifo" "${out}/frames.fifo"
# A pipe left in test-data/transient/ blocks any recursive read of the tree on open.
trap 'command rm -f "${out}/requests.fifo" "${out}/frames.fifo"' EXIT
(
    cd "${out}/repo"
    env HOME="${out}/home" XDG_STATE_HOME="${out}/xdg" KBASE_CONFIG_DIR="${out}/config" \
        GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="${out}/home/.gitconfig" \
        GIT_AUTHOR_NAME=mcp GIT_AUTHOR_EMAIL=mcp@invalid GIT_COMMITTER_NAME=mcp GIT_COMMITTER_EMAIL=mcp@invalid \
        "${kbase}" mcp < "${out}/requests.fifo" > "${out}/frames.fifo" 2> "${out}/mcp.log"
) &
server=$!
set +e
go test -count=1 -timeout 300s -run '^TestMCPBuildFollowedByStatus$' ./cmd -args \
    -probe.repo="${out}/repo" -probe.state-dir="${out}/state" \
    -probe.requests="${out}/requests.fifo" -probe.frames="${out}/frames.fifo" \
    > "${out}/test.log" 2>&1
code=$?
set -e
wait "${server}" || true
OUT_DIR="${out}" RECIPE="test-integration-mcp-build" INVOCATION="cp ${MCP_TRANSCRIPT_DIR}/paper.tex repo/; git init repo/ and one commit (identity fixed in the environment, global config quarantined to home/.gitconfig); mkfifo requests.fifo frames.fifo\nrepo/: ${kbase} mcp < requests.fifo > frames.fifo 2> mcp.log, whose build tool launched two builds with no inference into state/\ngo test -count=1 -timeout 300s -run '^TestMCPBuildFollowedByStatus\$' ./cmd -args -probe.repo=repo -probe.state-dir=state -probe.requests=requests.fifo -probe.frames=frames.fifo > test.log: initialize, build, status polled until the build is no longer running, then a resume followed the same way\npass is test.log's exit 0; a subscribe capability declared, a frame no request owes, a build that does not end finished, or one that never ends fails it" \
    tools/scripts/write-evidence.sh
if [[ "${code}" -ne 0 ]]; then
    printf 'integration(test-integration-mcp-build): the build was not followed to its end as expected — see %s\n' "${out}/test.log"
    exit 1
fi
printf 'integration(test-integration-mcp-build) ok\n'
