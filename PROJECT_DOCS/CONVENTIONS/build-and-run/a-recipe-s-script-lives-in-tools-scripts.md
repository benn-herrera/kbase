### A recipe's script lives in `tools/scripts/`

A `justfile` recipe whose body is a shebang script is a script under `tools/scripts/`, named for
its recipe, and the recipe passes the script its inputs as environment variables. The recipe line
is `NAME="{{NAME}}" tools/scripts/<recipe>.sh`, so a command-line variable override reaches the script
and no recipe re-invokes `just` for a private body. The script runs under its own
`#!/usr/bin/env bash` with at least `set -u`, reads only what the recipe sets (the recipe's variables
and parameters, and `just` built-ins such as `invocation_directory()` as variables of its own), and
opens by refusing an unset one — `: "${NAME:?…}"`, with no apostrophe in the message, which the macOS
`/bin/bash` 3.2 reads as an open quote. Scripts that need one another call each other by path, not
through `just`. The one exception to environment inputs is a variadic list a script passes on
untouched (the top `.tex` files and build flags of `test-integration-build-live-fixture`), which arrives
as positional arguments.

A helper a script sources rather than runs lives beside it in `tools/scripts/`, named `lib-<what>.sh`,
and is not executable. Its header states the caller's functions and variables it relies on.
