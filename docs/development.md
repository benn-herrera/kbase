# Development

How to build, test and release kbase itself. Back to the [README](../README.md).

```sh
just             # list recipes
just edit-gate   # cheap gate: run after every change
just checkpoint  # full suite: run at checkpoints
just test        # unit tests (VERBOSE=1 for per-test output)
just tag         # signed tag v<version> of a vetted tree; local only
just tap-update  # after a pushed release: the tap formula's url and sha256, committed in the tap; go.mod's go line raised if outside Go's supported window, uncommitted
just tap-local   # local homebrew install of release candidate
```

`just tag` refuses while tracked files have uncommitted changes, then creates a signed annotated
tag `v<version>` locally, the version being what the binary reports, and verifies its signature; it
runs no gates, so vet the tree first, and it pushes nothing. No built binaries are committed;
`bin/` is gitignored. An agent driving kbase reads its operating contract from kbase itself:
`kbase --help` and `kbase <command> --help` on the command line; over MCP, the server's
instructions, the tools, and each tool's or operation's help at `kbase://help/<name>`.

Integration recipes run `./bin/kbase` itself, except the live-fixture recipes, which run the
fixture's own copy (below), and keep their evidence under `test-data/transient/`.
A recipe excluded from `just test-integration` says so, and what it needs, in `just --list`.

`just prep-test-integration-fixture <dir>` stages a fixture repository. It resets the repository to
its `cleared` tag, installs the agent set for both harnesses, and copies `bin/kbase` to
`<dir>/.kbase/kbase`; it also copies `tools/harness.sh` to `<dir>/.kbase/harness.sh`. The fixture's root `.gitignore` at `cleared` must ignore `.kbase/`, which also
keeps its test configuration out of its history; staging refuses one that does not, before it
changes anything. The live
fixture recipes run that copy. A harness session over a staged fixture finds kbase on `PATH`:
start it from the fixture as `PATH=$(pwd)/.kbase:${PATH} claude` (or `opencode`), or
as `.kbase/harness.sh <harness>`, which does that and shows the `kbase` it resolves. No recipe
starts one.
