#!/usr/bin/env bash
# The MCP transcripts over the delivered binary: the results fixture staged as
# cmd/mcp_test.go stages it, each test-data/fixtures/mcp/*.jsonl fed to
# `./bin/kbase mcp` over real pipes and every response line compared to the
# expected line after placeholder substitution. A transcript named gui-* is fed
# to `./bin/kbase mcp --project-dir-required` from the evidence directory, not
# the staged repository. Every transcript runs: none needs a stub child or a
# provider (build-refused refuses before it starts one).
#
# Inputs (environment): BIN_DIR; RESULTS_FIXTURE_DIR, the committed fixture;
# MCP_TRANSCRIPT_DIR, the transcripts; MCP_OUT_DIR, the evidence directory.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${BIN_DIR:?BIN_DIR is not set: the build output directory}"
: "${RESULTS_FIXTURE_DIR:?RESULTS_FIXTURE_DIR is not set: the committed results fixture}"
: "${MCP_TRANSCRIPT_DIR:?MCP_TRANSCRIPT_DIR is not set: the transcript fixtures}"
: "${MCP_OUT_DIR:?MCP_OUT_DIR is not set: the evidence directory}"

root="$(pwd -P)"
out="${root}/${MCP_OUT_DIR}"
kbase="${root}/${BIN_DIR}/kbase"
fixture="${root}/${RESULTS_FIXTURE_DIR}"
transcripts="${root}/${MCP_TRANSCRIPT_DIR}"
repo="${out}/repo"
rm -rf "${repo}" "${out}/state" "${out}/transcripts" "${out}/staging" "${out}/comparison.tsv"
mkdir -p "${repo}/.git/objects" "${repo}/.git/refs/heads" "${out}/transcripts" "${out}/staging"
printf 'ref: refs/heads/main\n' > "${repo}/.git/HEAD"
cp -R "${fixture}/kb-root" "${repo}/"
mkdir "${repo}/kb-root/b"
( cd "${repo}" && "${kbase}" insert-claim-entry --values "${transcripts}/seed.yaml" ) > "${out}/insert.out" 2> "${out}/insert.log"
kb_root="$(sed -n 's/^kb-root: "\(.*\)"$/\1/p' "${out}/insert.out")"
ids=()
while IFS= read -r id; do ids+=("${id}"); done < <(sed -n '/^ids:/,/^minted:/s/^    - "\(clm-[a-z0-9]*\)"$/\1/p' "${out}/insert.out" | sort)
if [[ -z "${kb_root}" || "${#ids[@]}" -ne 2 ]]; then
    printf 'integration(test-integration-mcp): the staging insert reported no kb-root and two ids — see insert.out\n' >&2
    exit 1
fi
version="$("${kbase}" --version)"
version="${version##* }"
# sub <line>: the transcript placeholders, as cmd/mcp_test.go's mcpServer
# derives them (paths here carry no JSON-escaped characters). ${ID3}, the
# staged analysis, is empty until its insert reports it.
sub() {
    local s="${1}"
    s="${s//'${KB_ROOT}'/${kb_root}}"
    s="${s//'${REPO}'/${repo}}"
    s="${s//'${STATE_DIR}'/${out}/state}"
    s="${s//'${VERSION}'/${version}}"
    s="${s//'${ID1}'/${ids[0]}}"
    s="${s//'${ID2}'/${ids[1]}}"
    s="${s//'${ID3}'/${ids[2]-}}"
    printf '%s' "${s}"
}
# stage <op> <values>: one staging write op from the repository over the
# results fixture's values file, its placeholders substituted; the values as
# given and the result are kept under staging/.
stage() {
    local values="${out}/staging/$(basename "${2}")"
    printf '%s\n' "$(sub "$(cat "${fixture}/values/${2}")")" > "${values}"
    ( cd "${repo}" && "${kbase}" "${1}" --values "${values}" ) > "${out}/staging/${1}.out" 2> "${out}/staging/${1}.log" || {
        printf 'integration(test-integration-mcp): the staging %s failed — see staging/%s.out\n' "${1}" "${1}" >&2
        exit 1
    }
}
# The edges: one of each relation a public op writes, pending and numeric
# bearings both.
stage insert-work-entry insert-work.yaml
stage add-rests-on add-rests-on.yaml
stage insert-analysis-entry insert-analysis.yaml
ids+=("$(sed -n '/^ids:/,/^minted:/s/^    - "\(ana-[a-z0-9]*\)"$/\1/p' "${out}/staging/insert-analysis-entry.out")")
if [[ -z "${ids[2]}" ]]; then
    printf 'integration(test-integration-mcp): the staging analysis insert reported no id — see staging/insert-analysis-entry.out\n' >&2
    exit 1
fi
stage add-pairs add-pairs.yaml
( cd "${repo}" && "${kbase}" refresh ) > "${out}/refresh.out" 2> "${out}/refresh.log"
status=0
for file in "${transcripts}"/*.jsonl; do
    name="$(basename "${file}" .jsonl)"
    tdir="${out}/transcripts/${name}"
    mkdir -p "${tdir}"
    : > "${tdir}/requests.jsonl"
    : > "${tdir}/expected.jsonl"
    lines=()
    while IFS= read -r line || [[ -n "${line}" ]]; do lines+=("${line}"); done < "${file}"
    if (( ${#lines[@]} % 2 != 0 )); then
        printf '  fail %s: %d lines — a transcript pairs each request with its response\n' "${name}" "${#lines[@]}"
        printf '%s\t0\todd line count\n' "${name}" >> "${out}/comparison.tsv"
        status=1
        continue
    fi
    for ((i = 0; i < ${#lines[@]}; i += 2)); do
        printf '%s\n' "$(sub "${lines[${i}]}")" >> "${tdir}/requests.jsonl"
        if [[ "${lines[$((i + 1))]}" != "null" ]]; then
            printf '%s\n' "$(sub "${lines[$((i + 1))]}")" >> "${tdir}/expected.jsonl"
        fi
    done
    # A gui- transcript runs GUI mode: no located project, so it is served from
    # outside the staged repository and each call names its own.
    serve_dir="${repo}"
    mode=()
    if [[ "${name}" == gui-* ]]; then
        serve_dir="${out}"
        mode=(--project-dir-required)
    fi
    set +e
    ( cd "${serve_dir}" && "${kbase}" mcp ${mode[@]+"${mode[@]}"} ) < "${tdir}/requests.jsonl" > "${tdir}/actual.jsonl" 2> "${tdir}/mcp.log"
    printf '%s\n' "$?" > "${tdir}/mcp.exit"
    set -e
    bad=0
    wanted="$(wc -l < "${tdir}/expected.jsonl" | tr -d ' ')"
    # The line numbers where actual and expected differ, one per line; a
    # line present on one side only differs.
    while IFS= read -r j; do
        bad=1
        printf '  fail %s response %s:\n    got:  %s\n    want: %s\n' "${name}" "${j}" \
            "$(sed -n "${j}p" "${tdir}/actual.jsonl")" "$(sed -n "${j}p" "${tdir}/expected.jsonl")"
        printf '%s\t%s\tdiffers\n' "${name}" "${j}" >> "${out}/comparison.tsv"
    done < <(awk 'NR == FNR { w[FNR] = $0; n = FNR; next } { g[FNR] = $0; m = FNR }
        END { for (i = 1; i <= (n > m ? n : m); i++) if (g[i] != w[i]) print i }' "${tdir}/expected.jsonl" "${tdir}/actual.jsonl")
    [[ "$(cat "${tdir}/mcp.exit")" -eq 0 ]] || { bad=1; printf '  fail %s: kbase mcp exited %s — see %s\n' "${name}" "$(cat "${tdir}/mcp.exit")" "${tdir}/mcp.log"; printf '%s\t0\texit %s\n' "${name}" "$(cat "${tdir}/mcp.exit")" >> "${out}/comparison.tsv"; }
    if [[ "${bad}" -eq 0 ]]; then
        printf '  pass %s: %d response lines\n' "${name}" "${wanted}"
        printf '%s\t-\tpass\n' "${name}" >> "${out}/comparison.tsv"
    else
        status=1
    fi
done
OUT_DIR="${out}" RECIPE="test-integration-mcp" INVOCATION="cp -R ${RESULTS_FIXTURE_DIR}/kb-root repo/ (with a bare .git); mkdir repo/kb-root/b\nrepo/: ${kbase} insert-claim-entry --values ${MCP_TRANSCRIPT_DIR}/seed.yaml\nrepo/: per staging op — insert-work-entry, add-rests-on, insert-analysis-entry, add-pairs — ${kbase} <op> --values staging/<file>, <file> from ${RESULTS_FIXTURE_DIR}/values/ with \${ID1} and \${ID2} the claims' ids in id order and \${ID3} the analysis's; result in staging/<op>.out\nrepo/: ${kbase} refresh\nper transcript but gui-*, repo/: ${kbase} mcp < transcripts/<name>/requests.jsonl > transcripts/<name>/actual.jsonl, compared line by line to transcripts/<name>/expected.jsonl\nper gui-* transcript, ${MCP_OUT_DIR}/: ${kbase} mcp --project-dir-required, the same comparison\ncomparison.tsv: per transcript, the response line that differs, or pass" \
    tools/scripts/write-evidence.sh
if [[ "${status}" -ne 0 ]]; then
    printf 'integration(test-integration-mcp): a response differs from its transcript — see comparison.tsv\n'
    exit 1
fi
printf 'integration(test-integration-mcp) ok\n'
