#!/usr/bin/env bash
# usage: harness.sh <harness-name> [options...]
# e.g. harness opencode
# e.g. harness claude
# adds local ./.kbase to path first for finding fixture kbase.
export PATH="$(pwd)/.kbase:${PATH}"
printf './.kbase at head of PATH. active binary is v%s at "%s"\n<enter> to continue' \
  "$(command -v kbase > /dev/null && kbase --version || echo NONE)" \
  "$(command -v kbase)"
read _
"${@}"
