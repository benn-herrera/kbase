#!/usr/bin/env bash
# Fetches and unpacks the pinned arXiv sample, one source tree per id.
#
# Fetch and unpack happen in one script because the corpus a test reads is the
# unpacked tree. Both steps land atomically (a .part file, a .unpack dir, then
# a rename), so an interrupted run leaves nothing a later run mistakes for done.
# An e-print is usually a tarball but may be a single gzipped .tex — a one-file
# submission has nothing to tar — and that lands as paper.tex.
#
# Inputs (environment): ARXIV_IDS, the space-separated versioned ids;
# ARXIV_TGZ_DIR, the e-prints as downloaded; ARXIV_DIR, one unpacked tree per
# id; ARXIV_DELAY, the courtesy interval in seconds paid per download.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${ARXIV_IDS:?ARXIV_IDS is not set: the ids to fetch}"
: "${ARXIV_TGZ_DIR:?ARXIV_TGZ_DIR is not set: where e-prints are downloaded}"
: "${ARXIV_DIR:?ARXIV_DIR is not set: where e-prints are unpacked}"
: "${ARXIV_DELAY:?ARXIV_DELAY is not set: seconds between downloads}"

mkdir -p "${ARXIV_TGZ_DIR}" "${ARXIV_DIR}"
fetched=0; unpacked=0; present=0
# Unquoted: the ids are a space-separated list, split into words here.
for id in ${ARXIV_IDS}; do
    # A pre-2007 id carries a `/` (math/0211159v1); `_` keeps one paper to
    # one file and one directory.
    name="$(printf '%s' "${id}" | tr '/' '_')"
    tgz="${ARXIV_TGZ_DIR}/${name}.tar.gz"
    dir="${ARXIV_DIR}/${name}"
    if [[ -d "${dir}" ]]; then
        present=$((present + 1))
        continue
    fi
    if [[ ! -s "${tgz}" ]]; then
        printf 'fetching %s\n' "${id}"
        curl -fsSL -o "${tgz}.part" "https://arxiv.org/e-print/${id}" || {
            rm -f "${tgz}.part"
            printf 'error: %s could not be fetched\n' "${id}" >&2
            exit 1
        }
        mv "${tgz}.part" "${tgz}"
        fetched=$((fetched + 1))
        sleep "${ARXIV_DELAY}"
    fi
    rm -rf "${dir}.unpack"
    mkdir -p "${dir}.unpack"
    if ! tar xzf "${tgz}" -C "${dir}.unpack" 2>/dev/null; then
        rm -rf "${dir}.unpack"
        mkdir -p "${dir}.unpack"
        gunzip -c "${tgz}" > "${dir}.unpack/paper.tex"
    fi
    mv "${dir}.unpack" "${dir}"
    unpacked=$((unpacked + 1))
done
printf 'arxiv: %s fetched, %s unpacked, %s already present → %s\n' "${fetched}" "${unpacked}" "${present}" "${ARXIV_DIR}"
