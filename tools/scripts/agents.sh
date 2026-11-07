#!/usr/bin/env bash
# Installs the agent set, developed in tandem with adjagent, into this
# repository for both harnesses.
#
# Inputs (environment): AGENTS_DIR, the sibling adjagent clone.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${AGENTS_DIR:?AGENTS_DIR is not set: the sibling adjagent clone}"

if [[ ! -f "${AGENTS_DIR}/adjagent/__init__.py" ]]; then
    printf 'agents: no python module at %s/adjagent — kbase is developed in tandem with adjagent, so AGENTS_DIR must name a sibling clone\n' "${AGENTS_DIR}" >&2
    exit 1
fi
PYTHONPATH="${AGENTS_DIR}" python -m adjagent install . --harness=claude
PYTHONPATH="${AGENTS_DIR}" python -m adjagent install . --harness=opencode
