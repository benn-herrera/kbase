# Sourced by the arxiv integration scripts, not run. Relies on the caller's
# check function (check <name> <0 or 1> <detail> <evidence>), its root variable
# (the repository root, absolute) and its working directory being the root.

# keys <stem> <verb> <outcome>: the result <stem>.out parsed against its
# subcommand's documented keys, its outcome the one named; the go test output
# lands in <stem>.keys.log and one check row records the verdict.
keys() {
    set +e
    go test -count=1 -run '^TestResult$' ./cmd -args -result.file="${root}/$1.out" -result.verb="$2" -result.outcome="$3" > "$1.keys.log" 2>&1
    local rc=$?
    set -e
    check "$(basename "$1")-keys" "${rc}" "the $2 result parses against its documented keys, outcome $3" "$1.keys.log"
}
