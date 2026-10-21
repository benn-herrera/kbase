#!/usr/bin/env bash
# A harness stub: the command the integration fixture's profiles name. Every
# mode answers the item with the one answer no closed text can refuse — `none` —
# and the mode decides what else it does: where the prompt reached it, what it
# leaves in the working directory, or how it fails.
#
# It writes the sha256 of the text it was handed, never the text: the recipes
# compare that digest with the ledger's prompt_sha256, which is how they say the
# command received exactly the prompt the run composed. The over-limit modes
# write a megabyte — past the bound whatever the bound is, since the reason the
# item fails names the bound itself.
set -euo pipefail

mode="${1:-value}"
shift || true

digest() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum | cut -d' ' -f1
    else
        shasum -a 256 | cut -d' ' -f1
    fi
}

case "${mode}" in
argv)
    # The prompt is an argv element; stdin is closed empty.
    printf '%s' "${1-}" | digest > argv.digest
    printf 'none\n'
    ;;
stdin)
    # The prompt is the whole of stdin, and the argv carries none.
    digest > stdin.digest
    printf 'none\n'
    ;;
model)
    # The argv carries both tokens: the model kbase named and the prompt.
    printf '%s\n' "${1-}" > model.txt
    printf '%s' "${2-}" | digest > argv.digest
    printf 'none\n'
    ;;
isolate)
    # A file left by the previous item fails the next one: the run empties the
    # directory it hands each command, so a second item must see it empty.
    if [ -e seen ]; then
        printf 'the previous item is still visible\n' >&2
        exit 4
    fi
    : > seen
    printf 'none\n'
    ;;
fail)
    printf 'the stub refuses\n' >&2
    exit 3
    ;;

big-reply)
    head -c 1048576 /dev/zero | tr '\0' 'x'
    printf '\nnone\n'
    ;;
big-stderr)
    head -c 1048576 /dev/zero | tr '\0' 'x' >&2
    exit 1
    ;;
*)
    printf 'none\n'
    ;;
esac
